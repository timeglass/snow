package watch

import (
	"errors"
)

var ErrAlreadyStarted = errors.New("The monitor is already running")
var ErrAlreadyStopped = errors.New("The monitor is already not running")

type DirEvent interface {
	Dir() string
}

type Selector func(root, path string) (bool, error)

type M interface {
	CanEmit(path string) bool
	Start() (chan DirEvent, error)
	Stop() error
	Events() chan DirEvent
	Errors() chan error
	Dir() string
}
