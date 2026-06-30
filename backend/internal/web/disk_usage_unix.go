//go:build !windows

package web

import "golang.org/x/sys/unix"

type diskUsage struct {
	TotalBytes int64
	UsedBytes  int64
}

func diskUsageForPath(path string) (diskUsage, error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return diskUsage{}, err
	}
	total := int64(stat.Blocks) * int64(stat.Bsize)
	free := int64(stat.Bavail) * int64(stat.Bsize)
	return diskUsage{TotalBytes: total, UsedBytes: total - free}, nil
}
