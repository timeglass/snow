package monitor

import (
	"errors"
	"strings"
	"time"
)

var ErrAlreadyStarted = errors.New("The monitor is already running")
var ErrAlreadyStopped = errors.New("The monitor is already not running")

//Selectors allows monitoring to occure on something else then
//the complete subtree
type Selector func(root, path string) (bool, error)

var Recursive Selector = func(root, path string) (bool, error) {
	if strings.HasPrefix(path, root) {
		return true, nil
	}

	return false, nil
}

var NonRecursive Selector = func(root, path string) (bool, error) {
	if root == path {
		return true, nil
	}

	return false, nil
}

//Is emitted when something has happend to or in a directory
type DirEvent interface {
	Dir() string
}

type M interface {
	CanEmit(path string) bool
	Start() (chan DirEvent, error)
	Stop() error
	Events() chan DirEvent
	Errors() chan error
	Dir() string
}

func New(dir string, sel Selector, latency time.Duration) (M, error) {
	if sel == nil {
		sel = Recursive
	}

	if latency == 0 {
		latency = time.Millisecond * 50
	}

	return new(dir, sel, latency)
}
