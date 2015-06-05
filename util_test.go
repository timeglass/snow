package watch

import (
	"errors"
	// "fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var Latency = time.Millisecond * 20
var Timeout = time.Second * 100
var SettleTime = time.Millisecond * 40

var errEventTimeout = errors.New("Timed out waiting for a monitor event or error")

type results struct {
	evs  []DirEvent
	errs []error
}

func setupTestDir(t *testing.T) string {
	tdir, err := ioutil.TempDir("", ".timeglass_watch")
	if err != nil {
		t.Fatalf("Failed to create test directory: %s", err)
	}

	f1, err := os.Create(filepath.Join(tdir, "existing_file_1.md"))
	if err != nil {
		t.Fatalf("Failed to create test directory existing file: '%s'", err)
	}

	defer f1.Close()

	err = os.MkdirAll(filepath.Join(tdir, "existing_dir", "existing_sub_dir"), 0744)
	if err != nil {
		t.Fatalf("Failed to create existing test dirs: '%s'", err)
	}

	<-time.After(SettleTime)

	return tdir
}

func setupTestDirMonitor(t *testing.T, sel Selector) M {
	tdir := setupTestDir(t)

	m, err := NewMonitor(tdir, sel, Latency)
	if err != nil {
		t.Fatalf("Failed to create monitor: %s", err)
	}

	return m
}

func doSettle() {
	<-time.After(Latency + (time.Millisecond * 80))
}

func doMove(t *testing.T, m M, parts ...string) {
	from := []string{m.Dir()}
	to := []string{m.Dir()}

	s := false
	for _, p := range parts {
		if p == "->" {
			s = true
			continue
		}

		if s {
			to = append(to, p)
		} else {
			from = append(from, p)
		}
	}

	err := os.Rename(filepath.Join(from...), filepath.Join(to...))
	if err != nil {
		t.Fatalf("Failed to rename from '%s' to '%s': '%s'", from, to, err)
	}
}

func doRemove(t *testing.T, m M, name ...string) {
	path := filepath.Join(name...)
	path = filepath.Join(m.Dir(), path)
	err := os.RemoveAll(path)
	if err != nil {
		t.Fatalf("Failed to remove '%s': '%s'", path, err)
	}
}

func doWriteFile(t *testing.T, m M, data string, name ...string) string {
	path := filepath.Join(name...)
	path = filepath.Join(m.Dir(), path)
	err := ioutil.WriteFile(path, []byte(data), 0644)
	if err != nil {
		t.Fatalf("Failed to write file '%s': '%s'", path, err)
	}

	return path
}

func doCreateFolders(t *testing.T, m M, name ...string) string {
	path := filepath.Join(name...)
	path = filepath.Join(m.Dir(), path)

	err := os.MkdirAll(path, 0744)
	if err != nil {
		t.Fatalf("Failed to create test directory: '%s': '%s'", path, err)
	}

	return path
}

func waitForNEvents(t *testing.T, m M, n int) chan *results {
	done := make(chan *results)
	ress := &results{
		errs: []error{},
		evs:  []DirEvent{},
	}

	go func() {
	L:
		for {
			select {
			case ev := <-m.Events():
				ress.evs = append(ress.evs, ev)
				if len(ress.evs) >= n {
					break L
				}

			case err := <-m.Errors():
				ress.errs = append(ress.errs, err)
				break L
			case <-time.After(Timeout):
				ress.errs = append(ress.errs, errEventTimeout)
				break L
			}
		}

		done <- ress
	}()

	_, err := m.Start()
	if err != nil {
		t.Fatalf("Failed to start monitor: %s", err)
	}

	return done
}

func assertNthDirEvent(t *testing.T, evs []DirEvent, n int, dir string) {
	if len(evs) < n {
		t.Fatalf("Expected at least %d event(s), received only: %d", n, len(evs))
	}

	ev := evs[n-1]

	fi1, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Couldn't stat '%s' for comparision", dir)
	}

	fi2, err := os.Stat(ev.Dir())
	if err != nil {
		t.Fatalf("Couldn't stat from event '%s' for comparision", ev.Dir())
	}

	if !os.SameFile(fi1, fi2) {
		t.Fatalf("Expected something to have happend in '%s', instead event nr %d was about %s", dir, n, ev.Dir())
	}

}

func assertTimeout(t *testing.T, errs []error) {
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error (timeout), received: %d", len(errs))
	}

	if errs[0] != errEventTimeout {
		t.Fatalf("Expected only a timeout error, instead got: %s", errs[0])
	}
}

func assertNoErrors(t *testing.T, errs []error) {
	if len(errs) == 0 {
		return
	}

	t.Fatalf("Expected no errors, got %d: %s", len(errs), errs)
}
