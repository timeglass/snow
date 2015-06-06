// +build linux

package watch

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

type Monitor struct {
	ifd    int
	pipefd []int
	paths  map[int]string
	*monitor
	sync.Mutex
}

func NewMonitor(dir string, sel Selector, latency time.Duration) (*Monitor, error) {
	mon, err := newMonitor(dir, sel, latency)
	if err != nil {
		return nil, err
	}

	ifd, err := syscall.InotifyInit()
	if err != nil {
		return nil, os.NewSyscallError("InotifyInit", err)
	}

	m := &Monitor{
		paths:   map[int]string{},
		ifd:     ifd,
		monitor: mon,
	}

	go m.throttle()
	return m, nil
}

func (m *Monitor) addWatch(dir string) error {
	res, err := m.IsSelected(dir)
	if err != nil {
		return err
	} else if !res {
		return nil
	}

	m.Lock()
	defer m.Unlock()

	// If the filesystem object was already being watched
	// (perhaps via a different link to the same object), then the
	// descriptor for the existing watch is returned
	// @see http://man7.org/linux/man-pages/man2/inotify_add_watch.2.html
	wfd, err := syscall.InotifyAddWatch(m.ifd, dir, syscall.IN_CREATE|syscall.IN_DELETE|syscall.IN_MODIFY|syscall.IN_MOVED_FROM|syscall.IN_MOVED_TO|syscall.IN_ONLYDIR)
	if err != nil {
		return os.NewSyscallError("InotifyAddWatch", err)
	}

	m.paths[wfd] = dir
	return nil
}

// directory creation under linux requires some fake events
// at the time of finotify read() some sub files or directories
// may already be created, as such we walk the new directory recursively
// and emit "fake" events for any created files or directories and for the latter
// also add watches
func (m *Monitor) handleDirCreation(dir string) error {
	res, err := m.IsSelected(dir)
	if err != nil {
		return err
	} else if !res {
		return nil
	}

	//walk subdirectories that could have been created
	err = filepath.Walk(dir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if fi.IsDir() {
			fis, err := ioutil.ReadDir(path)
			if err != nil {
				return fmt.Errorf("Failed read dir '%s': %s", path, err)
			}

			if len(fis) > 0 {
				m.unthrottled <- &mevent{path}
			}

			err = m.addWatch(path)
			if err != nil {
				return fmt.Errorf("Failed to add '%s': %s", path, err)
			}
		}

		return nil
	})

	if err != nil {
		return err
	}

	//fake event for newly created directory
	fis, err := ioutil.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("Failed read dir '%s': %s", dir, err)
	}

	if len(fis) > 0 {
		m.unthrottled <- &mevent{dir}
	}

	//add the newly created dir itself
	err = m.addWatch(dir)
	if err != nil {
		return fmt.Errorf("Failed to watch directory '%s' that was just created: %s", dir, err)
	}

	return nil
}

func (m *Monitor) Start() (chan DirEvent, error) {
	go func() {
		var buf [syscall.SizeofInotifyEvent * 4096]byte
		var move struct {
			ID   uint32
			Fd   int
			From string
			To   string
		}

		for {
			n, err := syscall.Read(m.ifd, buf[:])
			if err != nil {
				m.errors <- os.NewSyscallError("Read", err)
				continue
			}

			if n == 0 {
				err := syscall.Close(m.ifd)
				if err != nil {
					m.errors <- os.NewSyscallError("Close", err)
				}

				return
			} else if n < 0 {
				m.errors <- os.NewSyscallError("Read", err)
				continue
			} else if n < syscall.SizeofInotifyEvent {
				m.errors <- fmt.Errorf("inotify: short read")
				continue
			}

			var offset uint32
			for offset <= uint32(n-syscall.SizeofInotifyEvent) {
				raw := (*syscall.InotifyEvent)(unsafe.Pointer(&buf[offset]))
				mask := uint32(raw.Mask)
				nbytes := (*[syscall.PathMax]byte)(unsafe.Pointer(&buf[offset+syscall.SizeofInotifyEvent]))
				name := strings.TrimRight(string(nbytes[0:raw.Len]), "\000")

				m.Lock()
				path := m.paths[int(raw.Wd)]
				m.Unlock()
				clean := filepath.Clean(path)

				//send all but implicit/explicit watch removal
				if mask&syscall.IN_IGNORED != syscall.IN_IGNORED {
					m.unthrottled <- &mevent{clean}
				}

				//something happend to a dir (created, deleted, moved etc)
				//handle these cases consistently with other implementations
				//to mimic recursive behaviour
				if mask&syscall.IN_ISDIR == syscall.IN_ISDIR {
					subject := filepath.Clean(filepath.Join(path, name))
					if mask&syscall.IN_CREATE == syscall.IN_CREATE {
						m.handleDirCreation(subject)
					} else if mask&syscall.IN_MOVED_FROM == syscall.IN_MOVED_FROM {
						move.ID = uint32(raw.Cookie)
						move.From = subject

						//attempt to fetch fd for directory that is about to be moved
						m.Lock()
						for fd, path := range m.paths {
							if path == subject {
								move.Fd = fd
							}
						}
						m.Unlock()

					} else if mask&syscall.IN_MOVED_TO == syscall.IN_MOVED_TO {
						if move.ID != 0 {
							if move.ID == raw.Cookie {
								move.To = subject
								if move.Fd != 0 {
									//it is associated with a fd in our path index, modify it
									//to complete the move for further events
									m.Lock()
									m.paths[move.Fd] = subject
									m.Unlock()
								}
							} else {
								m.errors <- fmt.Errorf("move didn't have a matching Cookie on arrival of IN_MOVE_FROM event")
							}
						} else {
							m.errors <- fmt.Errorf("move has no Cookie on arrival of IN_MOVE_FROM event")
						}
					} else if mask&syscall.IN_DELETE == syscall.IN_DELETE {
						//dir was removed, remove from paths index
						//if its indexed
						m.Lock()
						for fd, path := range m.paths {
							if path == subject {
								delete(m.paths, fd)
							}
						}
						m.Unlock()
					}
				}

				offset += syscall.SizeofInotifyEvent + raw.Len
			}

		}
	}()

	//recursive watch
	err := filepath.Walk(m.dir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if fi.IsDir() {
			err = m.addWatch(path)
			if err != nil {
				return fmt.Errorf("Failed to add '%s': %s", path, err)
			}
		}

		return nil
	})

	if err != nil {
		return m.Events(), err
	}

	return m.Events(), m.addWatch(m.Dir())
}
