// +build windows

package monitor

import (
	"syscall"
	"testing"
	"unsafe"
)

func nrOfOpenResources(t *testing.T) int {
	fnr := 0

	//@see https://github.com/golang/go/blob/master/misc/cgo/test/issue8517_windows.go
	cp, err := syscall.GetCurrentProcess()
	if err != nil {
		t.Fatalf("GetCurrentProcess: %v\n", err)
	}

	kernel32 := syscall.MustLoadDLL("kernel32.dll")
	getProcessHandleCount := kernel32.MustFindProc("GetProcessHandleCount")
	r, _, err := getProcessHandleCount.Call(uintptr(cp), uintptr(unsafe.Pointer(&fnr)))
	if r == 0 {
		t.Fatalf("GetProcessHandleCount: %v\n", error(err))
	}

	return fnr
}
