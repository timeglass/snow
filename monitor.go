package watch

import (
	"fmt"
	"path/filepath"
)

var Recursive Selector = func(root, path string) (bool, error) { return true, nil }
var NonRecursive Selector = func(root, path string) (bool, error) {
	if root == path {
		return true, nil
	}

	return false, nil
}

//a monitor event
type mevent struct {
	dir string
}

func (m *mevent) Dir() string { return m.dir }

//abstract monitor
func newMonitor(dir string, sel Selector) (*monitor, error) {
	rdir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, fmt.Errorf("Failed to eval symlink for '%s': %s", dir, rdir)
	}

	return &monitor{
		sel:    sel,
		dir:    rdir,
		events: make(chan DirEvent),
		errors: make(chan error),
	}, nil
}

type monitor struct {
	sel    Selector
	dir    string
	events chan DirEvent
	errors chan error
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

func (m *monitor) Dir() string {
	return m.dir
}
