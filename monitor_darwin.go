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

func NewMonitor(dir string) (*Monitor, error) {
	mon := newMonitor(dir)
	es := &fsevents.EventStream{
		Latency: time.Millisecond * 80,
		Paths:   []string{dir},
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
				m.events <- &mevent{ev.Path, time.Now()}
			}
		}
	}()

	return m.Events(), nil
}

func (m *Monitor) Stop() error {
	m.es.Stop()
	return nil
}
