package watch

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

var NrOfGoroutines = 0
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

	err = os.MkdirAll(filepath.Join(tdir, "workspace", "existing_dir", "existing_sub_dir"), 0744)
	if err != nil {
		t.Fatalf("Failed to create existing test dirs: '%s'", err)
	}

	f1, err := os.Create(filepath.Join(tdir, "workspace", "existing_file_1.md"))
	if err != nil {
		t.Fatalf("Failed to create test directory existing file: '%s'", err)
	}

	defer f1.Close()

	doSettle()
	return filepath.Join(tdir, "workspace")
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
	<-time.After(Latency + SettleTime)
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
		t.Fatalf("Failed to rename from '%s' to '%s': '%s'", filepath.Join(from...), filepath.Join(to...), err)
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

func waitForNEvents(t *testing.T, m M, min, max int) chan *results {
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
				if len(ress.evs) >= max {
					break L
				}

			case err := <-m.Errors():
				ress.errs = append(ress.errs, err)
				break L
			case <-time.After(Timeout):
				if len(ress.evs) < min {
					ress.errs = append(ress.errs, errEventTimeout)
				}
				break L
			}
		}

		done <- ress
	}()

	return done
}

func assertAtLeast(t *testing.T, evs []DirEvent, n int, dir string) {
	fi1, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Couldn't stat '%s' for comparision", dir)
	}

	count := 0
	for _, ev := range evs {
		fi2, err := os.Stat(ev.Dir())
		if err != nil {
			t.Fatalf("Couldn't stat from event '%s' for comparison", ev.Dir())
		}

		if os.SameFile(fi1, fi2) {
			count++
		}
	}

	if count != n {
		t.Fatalf("Expected %d events for '%s', received: %d", n, dir, count)
	}
}

func assertNthDirEventNoLongerExists(t *testing.T, evs []DirEvent, n int, dir string) {
	if len(evs) < n {
		t.Fatalf("Expected at least %d event(s), received only: %d", n, len(evs))
	}

	ev := evs[n-1]
	if !strings.HasPrefix(ev.Dir(), dir) {
		t.Fatalf("Asserting path, '%s' doesn't has prefix '%s'", ev.Dir(), dir)
	}

	_, err := os.Stat(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			t.Fatalf("Expected dir '%s' to no longer exists, but received other err: %s", dir, err)
		}
	} else {
		t.Fatalf("Expected dir '%s' to no longer exists, but it did", dir)
	}

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
		t.Fatalf("Couldn't stat from event '%s' for comparison", ev.Dir())
	}

	if !os.SameFile(fi1, fi2) {
		t.Fatalf("Expected something to have happend in '%s', instead event nr %d was about %s", dir, n, ev.Dir())
	}

}

func assertCanEmit(t *testing.T, m M, path string, expected bool) {
	res := m.CanEmit(path)
	if res != expected {
		t.Fatalf("Expected path '%s' CanEmit() to return %t, got: %t", path, expected, res)
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

func assertShutdown(t *testing.T, m M) {
	err := m.Stop()
	if err != nil {
		t.Fatalf("Failed to stop: %s", err)
	}

	//wait for the garbage collector
	<-time.After(time.Millisecond * 5)

	//check that goroutines dont leak
	nr := runtime.NumGoroutine()
	if NrOfGoroutines == 0 {
		NrOfGoroutines = nr
	} else {
		if nr != NrOfGoroutines {
			panic(fmt.Sprintf("Dont expect the number of goroutines to increase above %d, got: %d", NrOfGoroutines, nr))
		}
	}
}
