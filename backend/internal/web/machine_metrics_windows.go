//go:build windows

package web

import (
	"golang.org/x/sys/windows"
	"runtime"
	"unsafe"
)

var machineKernel = windows.NewLazySystemDLL("kernel32.dll")
var machineSystemTimes = machineKernel.NewProc("GetSystemTimes")
var machineMemoryStatus = machineKernel.NewProc("GlobalMemoryStatusEx")

type machineMemory struct {
	Length, Load                                                                                         uint32
	TotalPhys, AvailPhys, TotalPageFile, AvailPageFile, TotalVirtual, AvailVirtual, AvailExtendedVirtual uint64
}

func readMachineCounters() (machineCounters, error) {
	var idle, kernel, user windows.Filetime
	ok, _, err := machineSystemTimes.Call(uintptr(unsafe.Pointer(&idle)), uintptr(unsafe.Pointer(&kernel)), uintptr(unsafe.Pointer(&user)))
	if ok == 0 {
		return machineCounters{}, err
	}
	m := machineMemory{}
	m.Length = uint32(unsafe.Sizeof(m))
	ok, _, err = machineMemoryStatus.Call(uintptr(unsafe.Pointer(&m)))
	if ok == 0 {
		return machineCounters{}, err
	}
	ticks := func(v windows.Filetime) uint64 { return uint64(v.HighDateTime)<<32 | uint64(v.LowDateTime) }
	return machineCounters{total: ticks(kernel) + ticks(user), idle: ticks(idle), memoryTotal: int64(m.TotalPhys), memoryAvailable: int64(m.AvailPhys), cpus: runtime.NumCPU()}, nil
}
