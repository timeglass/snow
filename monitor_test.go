package watch

import (
	"io/ioutil"
	"testing"
)

func TestMonitorInterfaceCompliance(t *testing.T) {
	tdir, err := ioutil.TempDir("", ".timeglass_watch")
	if err != nil {
		t.Fatalf("Failed to create test directory: %s", err)
	}

	var m M

	m, err = NewMonitor(tdir)
	if err != nil {
		t.Fatalf("Failed to monitor: %s", err)
	}

	_ = m
}
