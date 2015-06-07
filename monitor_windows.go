// +build windows

package watch

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"
)

const bufferSize = 4096

type Monitor struct {
	handle syscall.Handle
	cph    syscall.Handle
	*monitor
}

func NewMonitor(dir string, sel Selector, latency time.Duration) (*Monitor, error) {
	mon, err := newMonitor(dir, sel, latency)
	if err != nil {
		return nil, err
	}

	m := &Monitor{
		monitor: mon,
	}

	return m, nil
}

func (m *Monitor) readDirChanges(h syscall.Handle, pBuff *byte, ov *syscall.Overlapped) error {
	return syscall.ReadDirectoryChanges(
		h,
		pBuff,
		uint32(bufferSize),
		true,
		syscall.FILE_NOTIFY_CHANGE_SIZE|syscall.FILE_NOTIFY_CHANGE_FILE_NAME|syscall.FILE_NOTIFY_CHANGE_DIR_NAME,
		nil,
		(*syscall.Overlapped)(unsafe.Pointer(ov)),
		0,
	)
}

func (m *Monitor) Stop() error {
	err := m.monitor.Stop()
	if err != nil {
		return err
	}

	err = syscall.CloseHandle(m.handle)
	if err != nil {
		return os.NewSyscallError("CloseHandle", err)
	}

	err = syscall.CloseHandle(m.cph)
	if err != nil {
		return os.NewSyscallError("CloseHandle", err)
	}

	return nil
}

func (m *Monitor) Start() (chan DirEvent, error) {
	m.monitor.Start()
	overlapped := &syscall.Overlapped{}
	var buffer [bufferSize]byte

	pdir, err := syscall.UTF16PtrFromString(m.Dir())
	if err != nil {
		return nil, os.NewSyscallError("UTF16PtrFromString", err)
	}

	m.handle, err = syscall.CreateFile(
		pdir,
		syscall.FILE_LIST_DIRECTORY,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_FLAG_BACKUP_SEMANTICS|syscall.FILE_FLAG_OVERLAPPED,
		0,
	)

	if err != nil {
		return nil, os.NewSyscallError("CreateFile", err)
	}

	m.cph, err = syscall.CreateIoCompletionPort(m.handle, 0, 0, 0)
	if err != nil {
		err2 := syscall.CloseHandle(m.handle)
		if err2 != nil {
			return nil, os.NewSyscallError("CloseHandle", err2)
		}

		return nil, os.NewSyscallError("CreateIoCompletionPort", err)
	}

	go func() {

		var n, key uint32
		var ov *syscall.Overlapped

		for {
			if m.stopped {
				return
			}

			err := syscall.GetQueuedCompletionStatus(m.cph, &n, &key, &ov, syscall.INFINITE)
			if m.stopped {
				return
			}

			switch err {
			case syscall.ERROR_MORE_DATA:
				if ov == nil {
					m.errors <- fmt.Errorf("ERROR_MORE_DATA has unexpectedly null lpOverlapped buffer")
				} else {
					n = uint32(unsafe.Sizeof(buffer))
				}
			case syscall.ERROR_ACCESS_DENIED:
				// @todo, handle watched dir is removed
				continue
			case syscall.ERROR_OPERATION_ABORTED:
				continue
			default:
				m.errors <- os.NewSyscallError("GetQueuedCompletionPort", err)
				continue
			case nil:
			}

			var offset uint32
			for {
				if n == 0 {
					m.errors <- fmt.Errorf("short read in readEvents()")
					break
				}

				raw := (*syscall.FileNotifyInformation)(unsafe.Pointer(&buffer[offset]))
				buf := (*[syscall.MAX_PATH]uint16)(unsafe.Pointer(&raw.FileName))
				name := syscall.UTF16ToString(buf[:raw.FileNameLength/2])
				fullname := m.Dir() + "\\" + name
				dirName := filepath.Dir(fullname)
				clean := filepath.Clean(dirName)

				res, err := m.IsSelected(clean)
				if err != nil {
					m.errors <- err
				} else if res {
					m.unthrottled <- &mevent{clean}
				}

				if raw.NextEntryOffset == 0 {
					break
				}

				offset += raw.NextEntryOffset
				if offset >= n {
					m.errors <- fmt.Errorf("Windows system assumed buffer larger than it is, events have likely been missed.")
					break
				}
			}

			//schedule new read if we didn't stop in the meantime
			if n != 0 && !m.stopped {
				err = m.readDirChanges(m.handle, &buffer[0], overlapped)
				if err != nil {
					if err == syscall.ERROR_ACCESS_DENIED {
						m.Stop()
						continue
					}

					m.errors <- os.NewSyscallError("readDirChanges", err)
				}
			}
		}
	}()

	err = m.readDirChanges(m.handle, &buffer[0], overlapped)
	if err != nil {
		return nil, os.NewSyscallError("ReadDirectoryChanges", err)
	}

	return m.Events(), nil
}
