package index

import (
	"log"
	"path/filepath"

	"github.com/timeglass/snow/monitor"
)

type Index struct {
	dirs    map[string]*Dir
	stop    chan struct{}
	dirEvs  <-chan monitor.DirEvent
	fileEvs chan<- struct{}
}

func NewIndex(dirEvs <-chan monitor.DirEvent) (*Index, error) {
	idx := &Index{
		dirs:    map[string]*Dir{},
		stop:    make(chan struct{}),
		dirEvs:  dirEvs,
		fileEvs: make(chan<- struct{}),
	}
	return idx, nil
}

func (i *Index) Index(dir string) error {
	ndir, err := NewDir(dir)
	if err != nil {
		return err
	}

	i.dirs[dir] = ndir
	return nil
}

func (i *Index) Deindex(dir string) error {
	delete(i.dirs, dir)

	return nil
}

func (i *Index) Start() {
	for {
		select {
		case <-i.stop:
			return
		case ev := <-i.dirEvs:
			evdir := filepath.Clean(ev.Dir())

			log.Println("Dir EV:", evdir)
			if dir, ok := i.dirs[evdir]; !ok {

				err := i.Index(evdir)
				if err != nil {
					log.Fatal(err)
				}

			} else {

				//index the new dir
				ndir, err := NewDir(evdir)
				if err != nil {
					//@todo, handle errors correctly
					log.Fatal("Not indexed", evdir, err)
				}

				//else compare with existing
				diff, err := dir.CompareNew(ndir)
				if err != nil {
					//@todo, handle errors correctly
					log.Fatal("Error comparison", err, diff)
				}

				//overwrite with new file
				dir.OverwriteFiles(ndir)

				//any any dir additions we can add to index immediately
				for p, add := range diff.Additions {
					if add.IsDir() {

						err := i.Index(evdir)
						if err != nil {
							log.Fatal(err)
						}

						delete(diff.Additions, p)
					}
				}

				//any any dir removals we can discard from index
				for p, del := range diff.Deletions {
					if del.IsDir() {
						err := i.Deindex(p)
						if err != nil {
							log.Fatal(err)
						}

						delete(diff.Deletions, p)
					}
				}

				//any dir modifications are ignored
				for p, mod := range diff.Modifications {
					if mod.IsDir() {
						delete(diff.Modifications, p)
					}
				}

				log.Printf("Diff:\n %s \n\n", diff)
			}

		}

	}
}

func (i *Index) Stop() {
	i.stop <- struct{}{}
}
