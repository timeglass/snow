package watch

type DirEvent interface {
	Dir() string
}

type Selector func(root, path string) (bool, error)

type M interface {
	CanEmit(path string) bool
	Start() (chan DirEvent, error)
	Events() chan DirEvent
	Errors() chan error
	Dir() string
}
