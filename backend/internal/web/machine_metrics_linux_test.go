//go:build linux

package web

import "testing"

func TestMachineProcCounters(t *testing.T) {
	c, err := parseMachineCounters("cpu  10 20 30 40 5 6 7 8 100 200\ncpu0 1\ncpu1 1\n", "MemTotal: 1024 kB\nMemAvailable: 384 kB\nCached: 200 kB\n")
	if err != nil || c.total != 126 || c.idle != 45 || c.cpus != 2 || c.memoryTotal != 1048576 || c.memoryAvailable != 393216 {
		t.Fatalf("counter interpretation %+v %v", c, err)
	}
	if _, err := parseMachineCounters("cpu 1 1 1 1\ncpu0 1\n", "MemTotal: 1024 kB\n"); err == nil {
		t.Fatal("missing available memory must not be reported as 100% usage")
	}
}
