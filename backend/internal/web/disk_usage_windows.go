//go:build windows

package web

import (
	"path/filepath"

	"golang.org/x/sys/windows"
)

type diskUsage struct {
	TotalBytes int64
	UsedBytes  int64
}

func diskUsageForPath(path string) (diskUsage, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return diskUsage{}, err
	}
	root := filepath.VolumeName(abs) + `\`
	ptr, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return diskUsage{}, err
	}

	var freeAvailable, total, totalFree uint64
	err = windows.GetDiskFreeSpaceEx(ptr, &freeAvailable, &total, &totalFree)
	if err != nil {
		return diskUsage{}, err
	}
	return diskUsage{TotalBytes: int64(total), UsedBytes: int64(total - totalFree)}, nil
}
