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
	epfd   int
	pipefd []int
	epes   []syscall.EpollEvent
	paths  map[int]string
	*monitor
	sync.Mutex
}

func NewMonitor(dir string, sel Selector, latency time.Duration) (*Monitor, error) {
	mon, err := newMonitor(dir, sel, latency)
	if err != nil {
		return nil, err
	}

	m := &Monitor{
		pipefd:  []int{-1, -1},
		paths:   map[int]string{},
		epes:    []syscall.EpollEvent{},
		monitor: mon,
	}

	return m, nil
}

func (m *Monitor) close() error {
	m.Lock()
	defer m.Unlock()

	err := syscall.Close(m.epfd)
	if err != nil {
		return os.NewSyscallError("Close", err)
	}

	err = syscall.Close(m.ifd)
	if err != nil {
		return os.NewSyscallError("Close", err)
	}

	err = syscall.Close(m.pipefd[0])
	if err != nil {
		return os.NewSyscallError("Close", err)
	}

	err = syscall.Close(m.pipefd[1])
	if err != nil {
		return os.NewSyscallError("Close", err)
	}

	return nil
}

func (m *Monitor) init() error {
	var err error
	m.ifd, err = syscall.InotifyInit()
	if err != nil {
		return os.NewSyscallError("InotifyInit", err)
	}

	m.epfd, err = syscall.EpollCreate(2)
	if err != nil {
		return os.NewSyscallError("EpollCreate", err)
	}

	err = syscall.Pipe(m.pipefd)
	if err != nil {
		return os.NewSyscallError("Pipe", err)
	}

	m.epes = []syscall.EpollEvent{
		{Events: syscall.EPOLLIN, Fd: int32(m.ifd)},
		{Events: syscall.EPOLLIN, Fd: int32(m.pipefd[0])},
	}

	err = syscall.EpollCtl(m.epfd, syscall.EPOLL_CTL_ADD, int(m.ifd), &m.epes[0])
	if err != nil {
		return os.NewSyscallError("EpollCtl", err)
	}

	err = syscall.EpollCtl(m.epfd, syscall.EPOLL_CTL_ADD, m.pipefd[0], &m.epes[1])
	if err != nil {
		return os.NewSyscallError("EpollCtl", err)
	}

	return nil
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
	wfd, err := syscall.InotifyAddWatch(m.ifd, dir, syscall.IN_DELETE_SELF|syscall.IN_CREATE|syscall.IN_DELETE|syscall.IN_MODIFY|syscall.IN_MOVED_FROM|syscall.IN_MOVED_TO|syscall.IN_ONLYDIR)
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

func (m *Monitor) CanEmit(path string) bool {
	if res, err := m.IsSelected(path); !res || err != nil {
		return false
	}

	m.Lock()
	defer m.Unlock()
	for _, p := range m.paths {
		if p == path {
			return true
		}
	}

	return false
}

func (m *Monitor) Stop() error {

	err := m.monitor.Stop()
	if err != nil {
		return err
	}

	_, err = syscall.Write(m.pipefd[1], []byte{0x00})
	if err != nil {
		return os.NewSyscallError("Write", err)
	}

	for fd, _ := range m.paths {
		delete(m.paths, fd)
	}

	return nil
}

func (m *Monitor) Start() (chan DirEvent, error) {
	err := m.monitor.Start()
	if err != nil {
		return m.Events(), nil
	}

	err = m.init()
	if err != nil {
		return nil, err
	}

	go func() {

		var buf [syscall.SizeofInotifyEvent * 4096]byte
		var move struct {
			ID   uint32
			Fd   int
			From string
			To   string
		}

		for {
			epes := make([]syscall.EpollEvent, 1)
			switch _, err := syscall.EpollWait(m.epfd, epes, -1); err {
			case nil:
				if epes[0].Fd == int32(m.ifd) {

					//from inotify
					n, err := syscall.Read(m.ifd, buf[:])
					if n == 0 || m.stopped {
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
						if mask&syscall.IN_IGNORED != syscall.IN_IGNORED && mask&syscall.IN_DELETE_SELF != syscall.IN_DELETE_SELF {
							m.unthrottled <- &mevent{clean}
						}

						//root directory removed? stop the monitor
						if mask&syscall.IN_DELETE_SELF == syscall.IN_DELETE_SELF && m.Dir() == clean {
							m.Stop()
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
										//we remove it here since it might be moved outside
										//the watchers view, if not the "IN_MOVE_TO" event will
										//re-insert it into the m.paths
										delete(m.paths, fd)
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

				} else if epes[0].Fd == int32(m.pipefd[0]) {

					//we are shutting down
					err := m.close()
					if err != nil {
						m.errors <- fmt.Errorf("Failed to close down: %s", err)
					}

					return
				} else {
					m.errors <- fmt.Errorf("epoll wait: unexpected event source: '%d'", epes[0].Fd)
				}
			case syscall.EINTR:
				continue
			default:
				m.errors <- fmt.Errorf("epoll wait: %s", err)
			}

		}
	}()

	//recursive watch
	err = filepath.Walk(m.dir, func(path string, fi os.FileInfo, err error) error {
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
