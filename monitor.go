package watch

import (
	"time"
)

//a monitor event
type mevent struct {
	dir  string
	time time.Time
}

func (m *mevent) Dir() string     { return m.dir }
func (m *mevent) Time() time.Time { return m.time }

//abstract monitor
func newMonitor(dir string) *monitor {
	return &monitor{
		dir:    dir,
		events: make(chan DirEvent),
		errors: make(chan error),
	}
}

type monitor struct {
	dir    string
	events chan DirEvent
	errors chan error
}

func (m *monitor) Events() chan DirEvent {
	return m.events
}

func (m *monitor) Errors() chan error {
	return m.errors
}

func (m *monitor) Dir() string {
	return m.dir
}
