// +build darwin

package watch

type Monitor struct {
	*monitor
}

func NewMonitor(dir string) (*Monitor, error) {
	return &Monitor{
		monitor: newMonitor(dir),
	}, nil
}

func (m *Monitor) Start() (chan DirEvent, error) {
	return m.Events(), nil
}

func (m *Monitor) Stop() error {
	return nil
}
