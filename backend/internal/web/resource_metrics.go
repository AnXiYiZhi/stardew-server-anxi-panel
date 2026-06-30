package web

import (
	"math"
	"net/http"
	"strings"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
)

type resourceMetricsResponse struct {
	InstanceID string               `json:"instanceId"`
	Service    string               `json:"service"`
	Sample     resourceMetricSample `json:"sample"`
}

type resourceMetricSample struct {
	Timestamp        string   `json:"timestamp"`
	CPUPercent       *float64 `json:"cpuPercent"`
	MemoryPercent    *float64 `json:"memoryPercent"`
	MemoryUsedBytes  int64    `json:"memoryUsedBytes,omitempty"`
	MemoryLimitBytes int64    `json:"memoryLimitBytes,omitempty"`
	DiskPercent      *float64 `json:"diskPercent"`
	DiskUsedBytes    int64    `json:"diskUsedBytes,omitempty"`
	DiskTotalBytes   int64    `json:"diskTotalBytes,omitempty"`
	ContainerRunning bool     `json:"containerRunning"`
	Message          string   `json:"message,omitempty"`
}

func (s *server) handleInstanceMetrics(w http.ResponseWriter, r *http.Request, instanceID string) {
	if _, ok := s.requireAuth(w, r); !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}

	sample := resourceMetricSample{Timestamp: time.Now().Format(time.RFC3339)}
	if disk, err := diskUsageForPath(instance.DataDir); err == nil && disk.TotalBytes > 0 {
		sample.DiskUsedBytes = disk.UsedBytes
		sample.DiskTotalBytes = disk.TotalBytes
		sample.DiskPercent = floatPtr(occupancyPercent(float64(disk.UsedBytes) / float64(disk.TotalBytes) * 100))
	}

	project := composeProjectStatusForWorkDir(instance.DataDir)
	if !project.Ready {
		sample.Message = "Compose 项目尚未准备，暂时只能显示磁盘占用"
		writeJSON(w, http.StatusOK, resourceMetricsResponse{
			InstanceID: instance.ID,
			Service:    "server",
			Sample:     sample,
		})
		return
	}

	stats, err := s.docker.ComposeStats(r.Context(), instance.DataDir)
	if err != nil {
		sample.Message = "容器指标暂不可用，请确认服务器容器已启动"
		writeJSON(w, http.StatusOK, resourceMetricsResponse{
			InstanceID: instance.ID,
			Service:    "server",
			Sample:     sample,
		})
		return
	}

	if service, ok := selectServerStats(stats.Services); ok {
		sample.ContainerRunning = true
		sample.CPUPercent = floatPtr(metricPercent(service.CPUPerc))
		sample.MemoryPercent = floatPtr(occupancyPercent(service.MemPerc))
		sample.MemoryUsedBytes = service.MemUsedBytes
		sample.MemoryLimitBytes = service.MemLimitBytes
		writeJSON(w, http.StatusOK, resourceMetricsResponse{
			InstanceID: instance.ID,
			Service:    serviceNameForStats(service),
			Sample:     sample,
		})
		return
	}

	sample.Message = "服务器容器未运行，启动后会显示 CPU 和内存趋势"
	writeJSON(w, http.StatusOK, resourceMetricsResponse{
		InstanceID: instance.ID,
		Service:    "server",
		Sample:     sample,
	})
}

func selectServerStats(services []paneldocker.ComposeServiceStats) (paneldocker.ComposeServiceStats, bool) {
	if len(services) == 0 {
		return paneldocker.ComposeServiceStats{}, false
	}
	for _, service := range services {
		if strings.EqualFold(service.Service, "server") {
			return service, true
		}
	}
	for _, service := range services {
		name := strings.ToLower(serviceNameForStats(service))
		if strings.Contains(name, "server") {
			return service, true
		}
	}
	return paneldocker.ComposeServiceStats{}, false
}

func serviceNameForStats(service paneldocker.ComposeServiceStats) string {
	if service.Service != "" {
		return service.Service
	}
	if service.Name != "" {
		return service.Name
	}
	if service.Container != "" {
		return service.Container
	}
	return "server"
}

func metricPercent(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return 0
	}
	return math.Round(value*10) / 10
}

func occupancyPercent(value float64) float64 {
	value = metricPercent(value)
	if value > 100 {
		return 100
	}
	return value
}

func floatPtr(value float64) *float64 {
	return &value
}
