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
	latency     time.Duration
	unthrottled chan DirEvent
	*monitor
}

func NewMonitor(dir string, sel Selector, latency time.Duration) (*Monitor, error) {
	mon, err := newMonitor(dir, sel)
	if err != nil {
		return nil, err
	}

	m := &Monitor{
		unthrottled: make(chan DirEvent),
		latency:     latency,
		monitor:     mon,
	}

	go m.throttle()
	return m, nil
}

func (m *Monitor) throttle() {
	throttles := map[string]time.Time{}
	for ev := range m.unthrottled {
		if until, ok := throttles[ev.Dir()]; ok {
			if until.Sub(time.Now()) > 0 {
				continue
			}
		}

		m.events <- ev
		throttles[ev.Dir()] = time.Now().Add(m.latency)
	}
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

func (m *Monitor) Start() (chan DirEvent, error) {
	overlapped := &syscall.Overlapped{}
	var buffer [bufferSize]byte

	pdir, err := syscall.UTF16PtrFromString(m.Dir())
	if err != nil {
		return nil, os.NewSyscallError("UTF16PtrFromString", err)
	}

	h, err := syscall.CreateFile(
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

	cph, err := syscall.CreateIoCompletionPort(h, 0, 0, 0)
	if err != nil {
		err2 := syscall.CloseHandle(h)
		if err2 != nil {
			return nil, os.NewSyscallError("CloseHandle", err2)
		}

		return nil, os.NewSyscallError("CreateIoCompletionPort", err)
	}

	go func() {

		var n, key uint32
		var ov *syscall.Overlapped

		for {
			err := syscall.GetQueuedCompletionStatus(cph, &n, &key, &ov, syscall.INFINITE)
			switch err {
			case syscall.ERROR_MORE_DATA:
				if ov == nil {
					m.errors <- fmt.Errorf("ERROR_MORE_DATA has unexpectedly null lpOverlapped buffer")
				} else {
					n = uint32(unsafe.Sizeof(buffer))
				}
			case syscall.ERROR_ACCESS_DENIED:
				// Watched directory was probably removed
				// w.sendEvent(watch.path, watch.mask&sys_FS_DELETE_SELF)
				// w.deleteWatch(watch)
				// w.startRead(watch)
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

			if n != 0 {
				err = m.readDirChanges(h, &buffer[0], overlapped)
				if err != nil {
					m.errors <- os.NewSyscallError("readDirChanges", err)
				}
			}
		}
	}()

	err = m.readDirChanges(h, &buffer[0], overlapped)
	if err != nil {
		return nil, os.NewSyscallError("ReadDirectoryChanges", err)
	}

	return m.Events(), nil
}
