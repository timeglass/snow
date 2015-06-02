// +build darwin

package watch

import (
	"time"

	"github.com/go-fsnotify/fsevents"
)

type Monitor struct {
	es *fsevents.EventStream
	*monitor
}

func NewMonitor(dir string, sel Selector, latency time.Duration) (*Monitor, error) {
	mon, err := newMonitor(dir, sel)
	if err != nil {
		return nil, err
	}

	es := &fsevents.EventStream{
		Latency: latency,
		Paths:   []string{mon.Dir()},
		Flags:   fsevents.WatchRoot,
	}

	return &Monitor{
		es:      es,
		monitor: mon,
	}, nil
}

func (m *Monitor) Start() (chan DirEvent, error) {
	m.es.Start()

	go func() {
		for msg := range m.es.Events {
			for _, ev := range msg {
				res, err := m.IsSelected(ev.Path)
				if err != nil {
					m.errors <- err
					continue
				}

				//for fsevent, only emit
				//events that match selector
				if res {
					m.events <- &mevent{ev.Path}
				}
			}
		}
	}()

	return m.Events(), nil
}
