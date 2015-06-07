// +build !windows

package monitor

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"testing"
)

func nrOfOpenResources(t *testing.T) int {
	fnr := 0
	out, err := exec.Command("/bin/sh", "-c", fmt.Sprintf("lsof -p %v", os.Getpid())).Output()
	if err != nil {
		t.Fatalf("Failed to exec lsof -p: %s", err)
	}

	fnr = bytes.Count(out, []byte("\n"))
	return fnr
}
