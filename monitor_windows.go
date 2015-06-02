// +build windows

package watch

import (
	"time"
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

	//@todo implement

	return m.Events(), nil
}
