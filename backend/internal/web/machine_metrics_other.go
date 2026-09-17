//go:build !linux && !windows

package web

func readMachineCounters() (machineCounters, error) { return machineCounters{}, errMachineCounters }
