package web

import (
	"context"
	"errors"
	"time"
)

type machineCounters struct {
	total, idle                  uint64
	memoryTotal, memoryAvailable int64
	cpus                         int
}

func machineCPUPercent(before, after machineCounters) *float64 {
	if after.total <= before.total || after.idle < before.idle {
		return nil
	}
	total, idle := after.total-before.total, after.idle-before.idle
	if idle > total {
		return nil
	}
	return floatPtr(occupancyPercent(float64(total-idle) / float64(total) * 100))
}

func collectMachineMetrics(ctx context.Context, dataDir string) resourceMetricSample {
	sample := resourceMetricSample{Scope: "machine", Timestamp: time.Now().UTC().Format(time.RFC3339), ContainerRunning: true}
	before, firstErr := readMachineCounters()
	timer := time.NewTimer(200 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		sample.Message = "机器资源采样超时"
		return sample
	case <-timer.C:
	}
	after, err := readMachineCounters()
	if err == nil {
		sample.CPUCount = after.cpus
		if firstErr == nil {
			sample.CPUPercent = machineCPUPercent(before, after)
		}
		if after.memoryTotal > 0 && after.memoryAvailable >= 0 && after.memoryAvailable <= after.memoryTotal {
			sample.MemoryTotalBytes = after.memoryTotal
			sample.MemoryUsedBytes = after.memoryTotal - after.memoryAvailable
			sample.MemoryPercent = floatPtr(occupancyPercent(float64(sample.MemoryUsedBytes) / float64(after.memoryTotal) * 100))
		}
	} else {
		sample.Message = "机器 CPU 和内存暂不可用"
	}
	if disk, err := diskUsageForPath(dataDir); err == nil && disk.TotalBytes > 0 {
		sample.DiskUsedBytes, sample.DiskTotalBytes = disk.UsedBytes, disk.TotalBytes
		sample.DiskPercent = floatPtr(occupancyPercent(float64(disk.UsedBytes) / float64(disk.TotalBytes) * 100))
	}
	return sample
}

var errMachineCounters = errors.New("machine counters unavailable")
