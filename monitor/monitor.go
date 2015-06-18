package monitor

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

//a monitor event
type mevent struct {
	dir string
}

func (m *mevent) Dir() string { return m.dir }

//abstract monitor
type monitor struct {
	stopped     bool
	latency     time.Duration
	sel         Selector
	dir         string
	unthrottled chan DirEvent
	events      chan DirEvent
	errors      chan error
}

func newMonitor(dir string, sel Selector, latency time.Duration) (*monitor, error) {
	rdir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, fmt.Errorf("Failed to eval symlink for '%s': %s", dir, err)
	}

	return &monitor{
		latency:     latency,
		sel:         sel,
		dir:         rdir,
		stopped:     true,
		unthrottled: make(chan DirEvent),
		events:      make(chan DirEvent),
		errors:      make(chan error),
	}, nil
}

func (m *monitor) throttle() {
	throttles := map[string]time.Time{}
	for ev := range m.unthrottled {
		if until, ok := throttles[ev.Dir()]; ok {
			diff := until.Sub(time.Now())
			if diff > 0 {
				continue
			}
		}

		m.events <- ev
		throttles[ev.Dir()] = time.Now().Add(m.latency)
	}
}

func (m *monitor) CanEmit(path string) bool {
	if m.stopped == true {
		return false
	}

	if res, err := m.IsSelected(path); !res || err != nil {
		return false
	}

	if _, err := os.Stat(path); err != nil {
		return false
	}

	return true
}

func (m *monitor) IsSelected(path string) (bool, error) {
	res, err := m.sel(m.dir, filepath.Clean(path))
	if err != nil {
		return false, err
	}

	return res, nil
}

func (m *monitor) Events() chan DirEvent {
	return m.events
}

func (m *monitor) Errors() chan error {
	return m.errors
}

func (m *monitor) Start() error {
	if m.stopped == false {
		return ErrAlreadyStarted
	}

	m.stopped = false
	m.unthrottled = make(chan DirEvent)

	go m.throttle()
	return nil
}

func (m *monitor) Stop() error {
	if m.stopped == true {
		return ErrAlreadyStopped
	}

	m.stopped = true
	close(m.unthrottled)
	return nil
}

func (m *monitor) Dir() string {
	return m.dir
}
