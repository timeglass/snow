package watch

import (
	"path/filepath"
	"testing"
	"time"
)

func init() {
	Latency = time.Millisecond * 80    //how long to wait after an event occurs before forwarding it
	SettleTime = time.Millisecond * 10 //file system sometimes needs time to settle
	Timeout = time.Millisecond * 1000  //how long to wait for the expected nr of events to come in
}

//do simple stuff in root

func TestRootFileCreation(t *testing.T) {
	m := setupTestDirMonitor(t, Recursive)
	done := waitForNEvents(t, m, 1)

	doWriteFile(t, m, "#foobar", "file_1.md")

	res := <-done
	assertNoErrors(t, res.errs)
	assertNthDirEvent(t, res.evs, 1, m.Dir())
}

func TestRootFileRemoval(t *testing.T) {
	m := setupTestDirMonitor(t, Recursive)
	done := waitForNEvents(t, m, 1)

	doRemove(t, m, "existing_file_1.md")

	res := <-done
	assertNoErrors(t, res.errs)
	assertNthDirEvent(t, res.evs, 1, m.Dir())
}

func TestRootFileEdit(t *testing.T) {
	m := setupTestDirMonitor(t, Recursive)
	done := waitForNEvents(t, m, 1)

	doWriteFile(t, m, "#foobar", "existing_file_1.md")

	res := <-done
	assertNoErrors(t, res.errs)
	assertNthDirEvent(t, res.evs, 1, m.Dir())
}

func TestRootFileMove(t *testing.T) {
	m := setupTestDirMonitor(t, Recursive)
	done := waitForNEvents(t, m, 1)

	doMove(t, m, "existing_file_1.md", "->", "existing_file_2.md")

	res := <-done
	assertNoErrors(t, res.errs)
	assertNthDirEvent(t, res.evs, 1, m.Dir())
}

func TestRootFolderCreation(t *testing.T) {
	m := setupTestDirMonitor(t, Recursive)
	done := waitForNEvents(t, m, 1)

	doCreateFolders(t, m, "folder_1")

	res := <-done
	assertNoErrors(t, res.errs)
	assertNthDirEvent(t, res.evs, 1, m.Dir())
}

// do stuff in sub folders

func TestSubFolderCreateFileInExisting(t *testing.T) {
	m := setupTestDirMonitor(t, Recursive)
	done := waitForNEvents(t, m, 1)

	doWriteFile(t, m, "#foobar", "existing_dir", "new_file_1.md")

	res := <-done
	assertNoErrors(t, res.errs)
	assertNthDirEvent(t, res.evs, 1, filepath.Join(m.Dir(), "existing_dir"))
}

func TestSubFolderCreateFileInNew(t *testing.T) {
	m := setupTestDirMonitor(t, Recursive)
	done := waitForNEvents(t, m, 2)

	dir := doCreateFolders(t, m, "folder_1")
	doWriteFile(t, m, "#foobar", "folder_1", "file_1.md")

	res := <-done
	assertNoErrors(t, res.errs)
	assertNthDirEvent(t, res.evs, 1, m.Dir())
	assertNthDirEvent(t, res.evs, 2, dir)
}

func TestSubFolderCreateMoveEditRemove(t *testing.T) {
	m := setupTestDirMonitor(t, Recursive)
	done := waitForNEvents(t, m, 5)

	dir := doCreateFolders(t, m, "folder_1")
	doWriteFile(t, m, "#foobar", "folder_1", "file_1.md")
	doSettle()
	doMove(t, m, "folder_1", "file_1.md", "->", "folder_1", "file_2.md")
	doSettle()
	doWriteFile(t, m, "#foobar", "folder_1", "file_2.md")
	doSettle()
	doRemove(t, m, "folder_1", "file_2.md")

	res := <-done
	assertNoErrors(t, res.errs)
	assertNthDirEvent(t, res.evs, 1, m.Dir())
	assertNthDirEvent(t, res.evs, 2, dir)
	assertNthDirEvent(t, res.evs, 3, dir)
	assertNthDirEvent(t, res.evs, 4, dir)
	assertNthDirEvent(t, res.evs, 5, dir)
}

func TestSubFolderCreationNonRecursive(t *testing.T) {
	m := setupTestDirMonitor(t, NonRecursive)
	done := waitForNEvents(t, m, 2)

	doCreateFolders(t, m, "folder_1")
	doWriteFile(t, m, "#foobar", "folder_1", "file_1.md")

	res := <-done
	assertTimeout(t, res.errs)
	assertNthDirEvent(t, res.evs, 1, m.Dir())
}

func TestSubFolderCreationRecursive(t *testing.T) {
	m := setupTestDirMonitor(t, Recursive)
	done := waitForNEvents(t, m, 3)

	dir := doCreateFolders(t, m, "folder_1", "sub_folder_1")
	doWriteFile(t, m, "#foobar", "folder_1", "sub_folder_1", "file_1.md")

	res := <-done
	assertNoErrors(t, res.errs)
	assertNthDirEvent(t, res.evs, 1, m.Dir())
	assertNthDirEvent(t, res.evs, 2, filepath.Join(m.Dir(), "folder_1"))
	assertNthDirEvent(t, res.evs, 3, dir)
}

//@todo test move between folders
