package watch

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"
)

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

	return tdir
}

func setupTestDirMonitor(t *testing.T) M {
	tdir := setupTestDir(t)

	m, err := NewMonitor(tdir)
	if err != nil {
		t.Fatalf("Failed to create monitor: %s", err)
	}

	_, err = m.Start()
	if err != nil {
		t.Fatalf("Failed to start monitor: %s", err)
	}

	return m
}

func doCreateFile(m M, name string, t *testing.T) {
	path := filepath.Join(m.Dir(), name)
	_, err := os.Create(path)
	if err != nil {
		t.Fatalf("Failed to creat test file: '%s': '%s'", path, err)
	}
}

func waitForNEvents(m M, n int, to time.Duration) chan *results {
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
			case <-time.After(to):
				ress.errs = append(ress.errs, errEventTimeout)
				break L
			}
		}

		done <- ress
	}()

	return done
}

func assertNthDirEvent(t *testing.T, evs []DirEvent, idx int, dir string) {
	n := idx + 1
	if len(evs) < n {
		t.Fatalf("Expected at least %d event(s), received only: %d", n, len(evs))
	}

	ev := evs[idx]

	fi1, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Couldn't stat '%s' for comparision", dir)
	}

	fi2, err := os.Stat(ev.Dir())
	if err != nil {
		t.Fatalf("Couldn't stat from event '%s' for comparision", ev.Dir())
	}

	if !os.SameFile(fi1, fi2) {
		t.Fatalf("Expected something to have happend in '%s', instead event %d was about %s", dir, idx, ev.Dir())
	}

}

func assertNoErrors(t *testing.T, errs []error) {
	if len(errs) == 0 {
		return
	}

	t.Fatalf("Expected no errors, got %d: %s", len(errs), errs)
}
