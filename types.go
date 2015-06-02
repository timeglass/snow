package watch

type DirEvent interface {
	Directory() string
	Time()
}

type M interface {
	Start() (chan DirEvent, error)
	Events() chan DirEvent
	Errors() chan error
	Stop() error
}
