package watch

import (
	"testing"
	"time"
)

var Timeout = time.Millisecond * 300 //how long to wait for the expected nr of events

func TestFileCreationEvent(t *testing.T) {
	m := setupTestDirMonitor(t)
	done := waitForNEvents(m, 1, Timeout)

	doCreateFile(m, "file_1.md", t)

	res := <-done
	assertNoErrors(t, res.errs)
	assertNthDirEvent(t, res.evs, 0, m.Dir())
}
