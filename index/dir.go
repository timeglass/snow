package index

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

type Diff struct {
	Additions     map[string]os.FileInfo
	Modifications map[string]os.FileInfo
	Deletions     map[string]os.FileInfo
}

func (d Diff) String() string {
	out := ""
	for p, _ := range d.Additions {
		out += fmt.Sprintf("\t+ %s\n", p)
	}

	for p, _ := range d.Modifications {
		out += fmt.Sprintf("\t= %s\n", p)
	}

	for p, _ := range d.Deletions {
		out += fmt.Sprintf("\t- %s\n", p)
	}

	return out
}

type Dir struct {
	files map[string]os.FileInfo
}

func NewDir(path string) (*Dir, error) {
	dir := &Dir{
		files: map[string]os.FileInfo{},
	}

	fis, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}

	for _, fi := range fis {
		dir.files[filepath.Join(path, fi.Name())] = fi
	}

	return dir, nil
}

func (d *Dir) OverwriteFiles(ndir *Dir) {
	d.files = ndir.files
}

func (d *Dir) CompareNew(ndir *Dir) (*Diff, error) {
	diff := &Diff{
		Additions:     map[string]os.FileInfo{},
		Modifications: map[string]os.FileInfo{},
		Deletions:     map[string]os.FileInfo{},
	}

	for p, exf := range d.files {
		if newf, ok := ndir.files[p]; ok {

			//it is in both, check if mod time was edited
			if newf.ModTime().Sub(exf.ModTime()) > 0 {
				diff.Modifications[p] = exf
			}

		} else {
			//file is not in the existing not in the new: deleted
			diff.Deletions[p] = exf
		}
	}

	for p, newf := range ndir.files {
		if _, ok := d.files[p]; !ok {
			//file is in the new one but not in the existing: addition
			diff.Additions[p] = newf
		}
	}

	return diff, nil
}
