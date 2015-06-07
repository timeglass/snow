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
	mon, err := newMonitor(dir, sel, latency)
	if err != nil {
		return nil, err
	}

	m := &Monitor{
		monitor: mon,
	}

	return m, nil
}

func (m *Monitor) Start() (chan DirEvent, error) {
	err := m.monitor.Start()
	if err != nil {
		return m.Events(), err
	}

	m.es = &fsevents.EventStream{
		Latency: m.latency,
		Paths:   []string{m.Dir()},
		Flags:   fsevents.WatchRoot | fsevents.NoDefer,
	}

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
					m.unthrottled <- &mevent{ev.Path}
				}

				//for now, just stop when the root changed (deleted/moved)
				if ev.Flags&fsevents.RootChanged == fsevents.RootChanged {
					m.Stop()
				}
			}
		}
	}()

	return m.Events(), nil
}

func (m *Monitor) Stop() error {
	err := m.monitor.Stop()
	if err != nil {
		return err
	}

	m.es.Stop()
	close(m.es.Events)
	return nil
}
