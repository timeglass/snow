// +build linux

package watch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

type Monitor struct {
	ifd    int
	epfd   int
	epev   []syscall.EpollEvent
	pipefd []int
	*monitor
}

func NewMonitor(dir string, sel Selector, latency time.Duration) (*Monitor, error) {
	mon, err := newMonitor(dir, sel)
	if err != nil {
		return nil, err
	}

	ifd, err := syscall.InotifyInit()
	if err != nil {
		return nil, os.NewSyscallError("InotifyInit", err)
	}

	m := &Monitor{
		ifd:     ifd,
		monitor: mon,
	}

	return m, nil
}

func (m *Monitor) Start() (chan DirEvent, error) {

	go func() {
		var buf [syscall.SizeofInotifyEvent * 4096]byte

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
				nbytes := (*[syscall.PathMax]byte)(unsafe.Pointer(&buf[offset+syscall.SizeofInotifyEvent]))
				name := strings.TrimRight(string(nbytes[0:raw.Len]), "\000")

				///

				fullname := m.Dir() + "/" + name
				dirName := filepath.Dir(fullname)
				clean := filepath.Clean(dirName)

				var events string = ""
				mask := uint32(raw.Mask)
				for _, b := range eventBits {
					if mask&b.Value == b.Value {
						mask &^= b.Value
						events += "|" + b.Name
					}
				}

				if mask != 0 {
					events += fmt.Sprintf("|%#x", mask)
				}
				if len(events) > 0 {
					events = " == " + events[1:]
				}

				fmt.Println("EVs:", events, clean)
				m.events <- &mevent{clean}

				///

				offset += syscall.SizeofInotifyEvent + raw.Len
			}

		}

	}()

	_, err := syscall.InotifyAddWatch(m.ifd, m.Dir(), syscall.IN_ALL_EVENTS)
	if err != nil {
		return m.Events(), os.NewSyscallError("InotifyAddWatch", err)
	}

	return m.Events(), nil
}

const (
	// Options for inotify_init() are not exported
	// IN_CLOEXEC    uint32 = syscall.IN_CLOEXEC
	// IN_NONBLOCK   uint32 = syscall.IN_NONBLOCK

	// Options for AddWatch
	IN_DONT_FOLLOW uint32 = syscall.IN_DONT_FOLLOW
	IN_ONESHOT     uint32 = syscall.IN_ONESHOT
	IN_ONLYDIR     uint32 = syscall.IN_ONLYDIR

	// The "IN_MASK_ADD" option is not exported, as AddWatch
	// adds it automatically, if there is already a watch for the given path
	// IN_MASK_ADD      uint32 = syscall.IN_MASK_ADD

	// Events
	IN_ACCESS        uint32 = syscall.IN_ACCESS
	IN_ALL_EVENTS    uint32 = syscall.IN_ALL_EVENTS
	IN_ATTRIB        uint32 = syscall.IN_ATTRIB
	IN_CLOSE         uint32 = syscall.IN_CLOSE
	IN_CLOSE_NOWRITE uint32 = syscall.IN_CLOSE_NOWRITE
	IN_CLOSE_WRITE   uint32 = syscall.IN_CLOSE_WRITE
	IN_CREATE        uint32 = syscall.IN_CREATE
	IN_DELETE        uint32 = syscall.IN_DELETE
	IN_DELETE_SELF   uint32 = syscall.IN_DELETE_SELF
	IN_MODIFY        uint32 = syscall.IN_MODIFY
	IN_MOVE          uint32 = syscall.IN_MOVE
	IN_MOVED_FROM    uint32 = syscall.IN_MOVED_FROM
	IN_MOVED_TO      uint32 = syscall.IN_MOVED_TO
	IN_MOVE_SELF     uint32 = syscall.IN_MOVE_SELF
	IN_OPEN          uint32 = syscall.IN_OPEN

	// Special events
	IN_ISDIR      uint32 = syscall.IN_ISDIR
	IN_IGNORED    uint32 = syscall.IN_IGNORED
	IN_Q_OVERFLOW uint32 = syscall.IN_Q_OVERFLOW
	IN_UNMOUNT    uint32 = syscall.IN_UNMOUNT
)

var eventBits = []struct {
	Value uint32
	Name  string
}{
	{IN_ACCESS, "IN_ACCESS"},
	{IN_ATTRIB, "IN_ATTRIB"},
	{IN_CLOSE, "IN_CLOSE"},
	{IN_CLOSE_NOWRITE, "IN_CLOSE_NOWRITE"},
	{IN_CLOSE_WRITE, "IN_CLOSE_WRITE"},
	{IN_CREATE, "IN_CREATE"},
	{IN_DELETE, "IN_DELETE"},
	{IN_DELETE_SELF, "IN_DELETE_SELF"},
	{IN_MODIFY, "IN_MODIFY"},
	{IN_MOVE, "IN_MOVE"},
	{IN_MOVED_FROM, "IN_MOVED_FROM"},
	{IN_MOVED_TO, "IN_MOVED_TO"},
	{IN_MOVE_SELF, "IN_MOVE_SELF"},
	{IN_OPEN, "IN_OPEN"},
	{IN_ISDIR, "IN_ISDIR"},
	{IN_IGNORED, "IN_IGNORED"},
	{IN_Q_OVERFLOW, "IN_Q_OVERFLOW"},
	{IN_UNMOUNT, "IN_UNMOUNT"},
}
