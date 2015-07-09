package index

import (
	"log"
	"os"
	"path/filepath"

	"github.com/timeglass/snow/monitor"
)

type Lazy struct {
	dirs    map[string]*Dir
	stop    chan struct{}
	dirEvs  <-chan monitor.DirEvent
	fileEvs chan<- struct{}
}

// the lazy index will not scan any directories recusively and will
// only scan a directory at all until it saw activity in it, this cuts
// down drastically on cpu cycles in large projects but makes but misses
// quite a few file events, especially after nested directories are removed
// or added.
func NewLazy(dirEvs <-chan monitor.DirEvent) (*Lazy, error) {
	idx := &Lazy{
		dirs:    map[string]*Dir{},
		stop:    make(chan struct{}),
		dirEvs:  dirEvs,
		fileEvs: make(chan<- struct{}),
	}
	return idx, nil
}

func (i *Lazy) Index(dir string) error {
	ndir, err := NewDir(dir)
	if err != nil {
		return err
	}

	i.dirs[dir] = ndir

	//indexing a dir could mean a rich set
	//of subdirectories was moved here. as such
	//it is expected we index recursively and send
	//diffs for files in those directories. But since
	//this is a lazy indexer we wont do this
	return nil
}

func (i *Lazy) Deindex(dir string) error {
	delete(i.dirs, dir)

	//deindexing a dir could mean it was moved.
	//in that case, deindex any subdirectories as well
	//furthermore, send diffs for all files in those
	//directories

	return nil
}

func (i *Lazy) Start() {
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
					if os.IsNotExist(err) {
						//it was not indexed before but
						//after an event it was not there,
						//nothing to do
						log.Println("New dir", ndir, "didn't exist on scanning it")
						continue
					} else {
						//@todo, handle errors correctly
						log.Fatal("Not  indexed", evdir, err)
					}
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
						err := i.Index(p)
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

func (i *Lazy) Stop() {
	i.stop <- struct{}{}
}
