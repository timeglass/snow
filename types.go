package watch

import (
	"time"
)

type DirEvent interface {
	Dir() string
	Time() time.Time
}

type M interface {
	Start() (chan DirEvent, error)
	Events() chan DirEvent
	Errors() chan error
	Dir() string
	Stop() error
}
