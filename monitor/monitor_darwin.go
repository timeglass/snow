// +build darwin

package monitor

import (
	"time"

	"github.com/timeglass/snow/_vendor/github.com/go-fsnotify/fsevents"
)

type Monitor struct {
	es *fsevents.EventStream
	*monitor
}

func new(dir string, sel Selector, latency time.Duration) (*Monitor, error) {
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

	//@todo sometimes fatal error: unexpected signal during runtime execution
	//but needs to be called to release open resources
	m.es.Stop()

	//@todo without closing, the program will leak goroutines
	close(m.es.Events)

	return nil
}
