package watch

import (
	"testing"
	"time"
)

var Timeout = time.Millisecond * 300 //how long to wait for the expected nr of events

func TestFileCreation(t *testing.T) {
	m := setupTestDirMonitor(t)
	done := waitForNEvents(t, m, 1, Timeout)

	doCreateFile(t, m, "file_1.md")

	res := <-done
	assertNoErrors(t, res.errs)
	assertNthDirEvent(t, res.evs, 1, m.Dir())
}

func TestFolderCreation(t *testing.T) {
	m := setupTestDirMonitor(t)
	done := waitForNEvents(t, m, 2, Timeout)

	dir := doCreateFolders(t, m, "folder_1")
	doCreateFile(t, m, "folder_1", "file_1.md")

	res := <-done
	assertNoErrors(t, res.errs)
	assertNthDirEvent(t, res.evs, 2, dir)
}

func TestSubFolderCreation(t *testing.T) {
	m := setupTestDirMonitor(t)
	done := waitForNEvents(t, m, 3, Timeout)

	dir := doCreateFolders(t, m, "folder_1", "sub_folder_1")
	doCreateFile(t, m, "folder_1", "sub_folder_1", "file_1.md")

	res := <-done
	assertNoErrors(t, res.errs)
	assertNthDirEvent(t, res.evs, 3, dir)
}
