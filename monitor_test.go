package watch

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func init() {
	Latency = time.Millisecond * 10    //how long to wait after an event occurs before forwarding it
	SettleTime = time.Millisecond * 30 //when the fs is asked to stettle, settle by the much on top of the latency
	Timeout = time.Millisecond * 100   //how long to wait for the expected nr of events to come in
}

//do simple stuff in root

func TestRootFileCreation(t *testing.T) {
	m := setupTestDirMonitor(t, Recursive)
	done := waitForNEvents(t, m, 1, 1)
	m.Start()

	doWriteFile(t, m, "#foobar", "file_1.md")

	res := <-done
	assertNoErrors(t, res.errs)
	assertNthDirEvent(t, res.evs, 1, m.Dir())
	assertShutdown(t, m)
}

func TestRootFileCreationTwice(t *testing.T) {
	m := setupTestDirMonitor(t, Recursive)
	done := waitForNEvents(t, m, 2, 2)
	m.Start()

	doWriteFile(t, m, "#foobar", "file_1.md")
	doWriteFile(t, m, "#foobar", "file_2.md")

	res := <-done
	assertTimeout(t, res.errs)
	assertNthDirEvent(t, res.evs, 1, m.Dir())
	assertShutdown(t, m)
}

func TestRootFileCreationTwiceWithSettle(t *testing.T) {
	m := setupTestDirMonitor(t, Recursive)
	done := waitForNEvents(t, m, 2, 2)
	m.Start()

	doWriteFile(t, m, "#foobar", "file_1.md")
	doSettle()
	doWriteFile(t, m, "#foobar", "file_2.md")

	res := <-done
	assertNoErrors(t, res.errs)
	assertNthDirEvent(t, res.evs, 1, m.Dir())
	assertNthDirEvent(t, res.evs, 2, m.Dir())
	assertShutdown(t, m)
}

func TestRootFileRemoval(t *testing.T) {
	m := setupTestDirMonitor(t, Recursive)
	done := waitForNEvents(t, m, 1, 1)
	m.Start()

	doRemove(t, m, "existing_file_1.md")

	res := <-done
	assertNoErrors(t, res.errs)
	assertNthDirEvent(t, res.evs, 1, m.Dir())
	assertShutdown(t, m)
}

func TestRootFolderRemoval(t *testing.T) {
	m := setupTestDirMonitor(t, Recursive)
	done := waitForNEvents(t, m, 2, 2)
	m.Start()

	doRemove(t, m, "existing_dir")

	res := <-done
	assertNoErrors(t, res.errs)
	assertNthDirEventNoLongerExists(t, res.evs, 1, filepath.Join(m.Dir(), "existing_dir"))
	assertNthDirEvent(t, res.evs, 2, m.Dir())
	assertShutdown(t, m)
}

func TestRootFolderMoveOutOfWatchedDir(t *testing.T) {
	m := setupTestDirMonitor(t, Recursive)
	done := waitForNEvents(t, m, 2, 2)
	m.Start()

	opath := filepath.Join(m.Dir(), "../outside_dir")
	path := filepath.Join(m.Dir(), "existing_dir")
	assertCanEmit(t, m, path, true)
	assertCanEmit(t, m, opath, false)

	doMove(t, m, "existing_dir", "->", "../outside_dir")

	res := <-done
	assertTimeout(t, res.errs)
	assertNthDirEvent(t, res.evs, 1, m.Dir())
	assertCanEmit(t, m, path, false)
	assertCanEmit(t, m, opath, false)
	assertShutdown(t, m)
}

func TestRootFileEdit(t *testing.T) {
	m := setupTestDirMonitor(t, Recursive)
	done := waitForNEvents(t, m, 1, 1)
	m.Start()

	doWriteFile(t, m, "#foobar", "existing_file_1.md")

	res := <-done
	assertNoErrors(t, res.errs)
	assertNthDirEvent(t, res.evs, 1, m.Dir())
	assertShutdown(t, m)
}

func TestRootFileEditTwiceWithSameContent(t *testing.T) {
	m := setupTestDirMonitor(t, Recursive)
	done := waitForNEvents(t, m, 2, 2)
	m.Start()

	doWriteFile(t, m, "#foobar", "existing_file_1.md")
	fiA, _ := os.Stat(filepath.Join(m.Dir(), "existing_file_1.md"))
	doSettle()

	doWriteFile(t, m, "#foobar", "existing_file_1.md")
	fiB, _ := os.Stat(filepath.Join(m.Dir(), "existing_file_1.md"))

	if fiA.Size() != fiB.Size() {
		t.Fatalf("Expected sizes of test file to be the same")
	}

	res := <-done
	assertNoErrors(t, res.errs)
	assertNthDirEvent(t, res.evs, 1, m.Dir())
	assertNthDirEvent(t, res.evs, 2, m.Dir())
	assertShutdown(t, m)
}

func TestRootFileMove(t *testing.T) {
	m := setupTestDirMonitor(t, Recursive)
	done := waitForNEvents(t, m, 1, 1)
	m.Start()

	doMove(t, m, "existing_file_1.md", "->", "existing_file_2.md")

	res := <-done
	assertNoErrors(t, res.errs)
	assertNthDirEvent(t, res.evs, 1, m.Dir())
	assertShutdown(t, m)
}

func TestRootFolderCreation(t *testing.T) {
	m := setupTestDirMonitor(t, Recursive)
	done := waitForNEvents(t, m, 1, 1)
	m.Start()

	doCreateFolders(t, m, "folder_1")

	res := <-done
	assertNoErrors(t, res.errs)
	assertNthDirEvent(t, res.evs, 1, m.Dir())
	assertShutdown(t, m)
}

func TestRootFolderMoveExistingFolderSettle(t *testing.T) {
	m := setupTestDirMonitor(t, Recursive)
	done := waitForNEvents(t, m, 2, 2)
	m.Start()

	doMove(t, m, "existing_dir", "->", "existing_folder_2")
	doSettle()
	doWriteFile(t, m, "#foobar", "existing_folder_2", "file_1.md")

	res := <-done
	assertNoErrors(t, res.errs)
	assertNthDirEvent(t, res.evs, 1, m.Dir())
	assertNthDirEvent(t, res.evs, 2, filepath.Join(m.Dir(), "existing_folder_2"))
	assertShutdown(t, m)
}

func TestRootFolderMoveExistingFolderNoSettle(t *testing.T) {
	m := setupTestDirMonitor(t, Recursive)
	done := waitForNEvents(t, m, 2, 2)
	m.Start()

	doMove(t, m, "existing_dir", "->", "existing_folder_2")
	doWriteFile(t, m, "#foobar", "existing_folder_2", "file_1.md")

	res := <-done
	assertNoErrors(t, res.errs)
	assertNthDirEvent(t, res.evs, 1, m.Dir())
	assertNthDirEvent(t, res.evs, 2, filepath.Join(m.Dir(), "existing_folder_2"))
	assertShutdown(t, m)
}

// do stuff in sub folders

func TestSubFolderMoveToExistingSettleBefore(t *testing.T) {
	m := setupTestDirMonitor(t, Recursive)
	done := waitForNEvents(t, m, 3, 3)
	m.Start()

	dir := doCreateFolders(t, m, "folder_1")
	doSettle()
	doWriteFile(t, m, "#foobar", "folder_1", "file_1.md")
	doMove(t, m, "folder_1", "file_1.md", "->", "existing_dir", "file_2.md")

	res := <-done
	assertNoErrors(t, res.errs)
	assertNthDirEvent(t, res.evs, 1, m.Dir())
	assertNthDirEvent(t, res.evs, 2, dir)
	assertNthDirEvent(t, res.evs, 3, filepath.Join(m.Dir(), "existing_dir"))
	assertShutdown(t, m)
}

func TestSubFolderMoveToExistingSettleAfter(t *testing.T) {
	m := setupTestDirMonitor(t, Recursive)
	done := waitForNEvents(t, m, 4, 4)
	m.Start()

	dir := doCreateFolders(t, m, "folder_1")
	doWriteFile(t, m, "#foobar", "folder_1", "file_1.md")
	doSettle()
	doMove(t, m, "folder_1", "file_1.md", "->", "existing_dir", "file_2.md")

	res := <-done
	assertNoErrors(t, res.errs)
	assertNthDirEvent(t, res.evs, 1, m.Dir())
	assertNthDirEvent(t, res.evs, 2, dir)
	assertNthDirEvent(t, res.evs, 3, filepath.Join(m.Dir(), "folder_1"))
	assertNthDirEvent(t, res.evs, 4, filepath.Join(m.Dir(), "existing_dir"))
	assertShutdown(t, m)
}

//unfortunately we cannot guarantee that inotify will an event
//for the newly created folder itself unless something is put into it
func TestSubFolderMoveToExistingNoSettle(t *testing.T) {
	m := setupTestDirMonitor(t, Recursive)
	done := waitForNEvents(t, m, 2, 3)
	m.Start()

	doCreateFolders(t, m, "folder_1")
	doWriteFile(t, m, "#foobar", "folder_1", "file_1.md")
	doMove(t, m, "folder_1", "file_1.md", "->", "existing_dir", "file_2.md")

	res := <-done
	assertNoErrors(t, res.errs)
	assertAtLeast(t, res.evs, 1, m.Dir())
	assertAtLeast(t, res.evs, 1, filepath.Join(m.Dir(), "existing_dir"))
	assertNthDirEvent(t, res.evs, 1, m.Dir())
	assertShutdown(t, m)
}

func TestSubFolderMoveFromToNewNoSettle(t *testing.T) {
	m := setupTestDirMonitor(t, Recursive)
	done := waitForNEvents(t, m, 3, 4)
	m.Start()

	doCreateFolders(t, m, "folder_2")
	doCreateFolders(t, m, "folder_1", "sub_folder_1")
	doWriteFile(t, m, "#foobar", "folder_1", "file_1.md")
	doMove(t, m, "folder_1", "file_1.md", "->", "folder_2", "file_2.md")

	res := <-done
	assertNoErrors(t, res.errs)
	assertAtLeast(t, res.evs, 1, m.Dir())
	assertAtLeast(t, res.evs, 1, filepath.Join(m.Dir(), "folder_1"))
	assertAtLeast(t, res.evs, 1, filepath.Join(m.Dir(), "folder_2"))
	assertNthDirEvent(t, res.evs, 1, m.Dir())
	assertShutdown(t, m)
}

func TestSubFolderCreateFileInExistingMaxEvents(t *testing.T) {
	m := setupTestDirMonitor(t, Recursive)
	done := waitForNEvents(t, m, 2, 2)
	m.Start()

	doWriteFile(t, m, "#foobar", "existing_dir", "new_file_1.md")

	res := <-done
	assertTimeout(t, res.errs)
	assertNthDirEvent(t, res.evs, 1, filepath.Join(m.Dir(), "existing_dir"))
	assertShutdown(t, m)
}

func TestSubFolderCreateFileInNew(t *testing.T) {
	m := setupTestDirMonitor(t, Recursive)
	done := waitForNEvents(t, m, 2, 2)
	m.Start()

	dir := doCreateFolders(t, m, "folder_1")
	doWriteFile(t, m, "#foobar", "folder_1", "file_1.md")

	res := <-done
	assertNoErrors(t, res.errs)
	assertNthDirEvent(t, res.evs, 1, m.Dir())
	assertNthDirEvent(t, res.evs, 2, dir)
	assertShutdown(t, m)
}

func TestSubFolderCreateMoveEditRemove(t *testing.T) {
	m := setupTestDirMonitor(t, Recursive)
	done := waitForNEvents(t, m, 5, 5)
	m.Start()

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
	assertShutdown(t, m)
}

func TestSubFolderCreationNonRecursive(t *testing.T) {
	m := setupTestDirMonitor(t, NonRecursive)
	done := waitForNEvents(t, m, 2, 2)
	m.Start()

	doCreateFolders(t, m, "folder_1")
	doWriteFile(t, m, "#foobar", "folder_1", "file_1.md")

	res := <-done
	assertTimeout(t, res.errs)
	assertNthDirEvent(t, res.evs, 1, m.Dir())
	assertShutdown(t, m)
}

func TestSubFolderCreationRecursive(t *testing.T) {
	m := setupTestDirMonitor(t, Recursive)
	done := waitForNEvents(t, m, 3, 3)
	m.Start()

	dir := doCreateFolders(t, m, "folder_1", "sub_folder_1")
	doWriteFile(t, m, "#foobar", "folder_1", "sub_folder_1", "file_1.md")

	res := <-done
	assertNoErrors(t, res.errs)
	assertNthDirEvent(t, res.evs, 1, m.Dir())
	assertNthDirEvent(t, res.evs, 2, filepath.Join(m.Dir(), "folder_1"))
	assertNthDirEvent(t, res.evs, 3, dir)
	assertShutdown(t, m)
}

func TestWatchedFolderRemoval(t *testing.T) {
	m := setupTestDirMonitor(t, Recursive)
	done := waitForNEvents(t, m, 2, 2)
	m.Start()

	doRemove(t, m, "..", "workspace")

	res := <-done
	assertNoErrors(t, res.errs)
	assertNthDirEventNoLongerExists(t, res.evs, 1, m.Dir())
	assertNthDirEventNoLongerExists(t, res.evs, 2, m.Dir())
}

func TestSubFolderCreationStartStop(t *testing.T) {
	m := setupTestDirMonitor(t, Recursive)

	//initial start with some events
	done := waitForNEvents(t, m, 3, 3)
	m.Start()

	dir := doCreateFolders(t, m, "folder_1", "sub_folder_1")
	doWriteFile(t, m, "#foobar", "folder_1", "sub_folder_1", "file_1.md")

	res := <-done
	assertNoErrors(t, res.errs)
	assertNthDirEvent(t, res.evs, 1, m.Dir())
	assertNthDirEvent(t, res.evs, 2, filepath.Join(m.Dir(), "folder_1"))
	assertNthDirEvent(t, res.evs, 3, dir)

	//stop and cause some events
	done = waitForNEvents(t, m, 1, 1)
	m.Stop()
	assertCanEmit(t, m, m.Dir(), false)

	doSettle()
	doWriteFile(t, m, "#foobar", "folder_1", "sub_folder_1", "file_2.md")

	res = <-done
	assertTimeout(t, res.errs)
	doSettle()

	//start again, should recapture events as before
	done = waitForNEvents(t, m, 1, 1)
	m.Start()

	doWriteFile(t, m, "#foobar", "folder_1", "sub_folder_1", "file_3.md")

	res = <-done
	assertNoErrors(t, res.errs)
	assertNthDirEvent(t, res.evs, 1, dir)
	assertShutdown(t, m)
}

//@todo test removal of watched directory itself
