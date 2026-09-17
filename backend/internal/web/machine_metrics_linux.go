//go:build linux

package web

import (
	"os"
	"strconv"
	"strings"
)

func readMachineCounters() (machineCounters, error) {
	stat, err := os.ReadFile("/proc/stat")
	if err != nil {
		return machineCounters{}, err
	}
	mem, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return machineCounters{}, err
	}
	return parseMachineCounters(string(stat), string(mem))
}

// procfs system counters describe the Linux host, not the Panel's cgroup limit.
// Guest ticks are already included in user/nice; iowait belongs to idle time.
func parseMachineCounters(stat, mem string) (machineCounters, error) {
	var c machineCounters
	for _, line := range strings.Split(stat, "\n") {
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		if f[0] == "cpu" {
			if len(f) < 5 {
				return c, errMachineCounters
			}
			for i := 1; i < len(f) && i <= 8; i++ {
				n, err := strconv.ParseUint(f[i], 10, 64)
				if err != nil {
					return c, errMachineCounters
				}
				c.total += n
				if i == 4 || i == 5 {
					c.idle += n
				}
			}
		} else if strings.HasPrefix(f[0], "cpu") {
			if _, err := strconv.Atoi(strings.TrimPrefix(f[0], "cpu")); err == nil {
				c.cpus++
			}
		}
	}
	availableFound := false
	for _, line := range strings.Split(mem, "\n") {
		f := strings.Fields(line)
		if len(f) < 2 || (f[0] != "MemTotal:" && f[0] != "MemAvailable:") {
			continue
		}
		n, err := strconv.ParseInt(f[1], 10, 64)
		if err != nil || n < 0 {
			return c, errMachineCounters
		}
		if f[0] == "MemTotal:" {
			c.memoryTotal = n * 1024
		} else {
			c.memoryAvailable = n * 1024
			availableFound = true
		}
	}
	if c.cpus < 1 || c.total == 0 || c.memoryTotal <= 0 || !availableFound || c.memoryAvailable > c.memoryTotal {
		return c, errMachineCounters
	}
	return c, nil
}
