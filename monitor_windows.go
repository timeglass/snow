// +build windows

package watch

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"
)

type Monitor struct {
	*monitor
}

func NewMonitor(dir string, sel Selector, latency time.Duration) (*Monitor, error) {
	mon, err := newMonitor(dir, sel)
	if err != nil {
		return nil, err
	}

	return &Monitor{
		monitor: mon,
	}, nil
}

func (m *Monitor) Start() (chan DirEvent, error) {
	overlapped := &syscall.Overlapped{}
	var buffer [4096]byte

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
					m.errors <- errors.New("ERROR_MORE_DATA has unexpectedly null lpOverlapped buffer")
				} else {
					// The i/o succeeded but the buffer is full.
					// In theory we should be building up a full packet.
					// In practice we can get away with just carrying on.
					n = uint32(unsafe.Sizeof(buffer))
				}
			case syscall.ERROR_ACCESS_DENIED:
				// Watched directory was probably removed
				// w.sendEvent(watch.path, watch.mask&sys_FS_DELETE_SELF)
				// w.deleteWatch(watch)
				// w.startRead(watch)
				continue
			case syscall.ERROR_OPERATION_ABORTED:
				// CancelIo was called on this handle
				continue
			default:
				m.errors <- os.NewSyscallError("GetQueuedCompletionPort", err)
				continue
			case nil:
			}

			var offset uint32
			for {
				if n == 0 {
					m.errors <- errors.New("short read in readEvents()")
					break
				}

				raw := (*syscall.FileNotifyInformation)(unsafe.Pointer(&buffer[offset]))
				buf := (*[syscall.MAX_PATH]uint16)(unsafe.Pointer(&raw.FileName))
				name := syscall.UTF16ToString(buf[:raw.FileNameLength/2])
				fullname := m.Dir() + "\\" + name
				dirName := filepath.Dir(fullname)

				clean := filepath.Clean(dirName)
				fmt.Println(fullname, clean)

				///////

				res, err := m.IsSelected(clean)
				if err != nil {
					m.errors <- err
				} else if res {
					m.events <- &mevent{clean}
				}

				/////

				if raw.NextEntryOffset == 0 {
					break
				}

				offset += raw.NextEntryOffset
				if offset >= n {
					m.errors <- errors.New("Windows system assumed buffer larger than it is, events have likely been missed.")
					break
				}
			}

		}
	}()

	err = syscall.ReadDirectoryChanges(
		h,
		&buffer[0],
		uint32(unsafe.Sizeof(buffer)),
		true,
		syscall.FILE_NOTIFY_CHANGE_LAST_ACCESS|syscall.FILE_NOTIFY_CHANGE_SIZE|syscall.FILE_NOTIFY_CHANGE_ATTRIBUTES|syscall.FILE_NOTIFY_CHANGE_LAST_WRITE|syscall.FILE_NOTIFY_CHANGE_CREATION|syscall.FILE_NOTIFY_CHANGE_FILE_NAME|syscall.FILE_NOTIFY_CHANGE_DIR_NAME,
		nil,
		(*syscall.Overlapped)(unsafe.Pointer(overlapped)),
		0,
	)

	if err != nil {
		return nil, os.NewSyscallError("ReadDirectoryChanges", err)
	}

	return m.Events(), nil
}
