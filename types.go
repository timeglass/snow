package watch

type DirEvent interface {
	Dir() string
}

type Selector func(root, path string) (bool, error)

type M interface {
	Start() (chan DirEvent, error)
	Events() chan DirEvent
	Errors() chan error
	Dir() string
}
