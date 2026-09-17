package web

import (
	"context"
	"math"
	"net/http"
	"strings"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
)

const resourceMetricsCacheTTL = 5 * time.Second

type resourceMetricsCacheEntry struct {
	response  resourceMetricsResponse
	expiresAt time.Time
}

type resourceMetricsFlight struct {
	done     chan struct{}
	response resourceMetricsResponse
}

type resourceMetricsResponse struct {
	InstanceID     string                            `json:"instanceId"`
	Service        string                            `json:"service"`
	Sample         resourceMetricSample              `json:"sample"`
	Machine        *resourceMetricSample             `json:"machine,omitempty"`
	Services       []paneldocker.ComposeServiceStats `json:"-"`
	StatsAvailable bool                              `json:"-"`
}

type resourceMetricSample struct {
	ContainerState   string   `json:"containerState,omitempty"`
	Scope            string   `json:"scope,omitempty"`
	CPUCount         int      `json:"cpuCount,omitempty"`
	CPUCores         *float64 `json:"cpuCores,omitempty"`
	MemoryTotalBytes int64    `json:"memoryTotalBytes,omitempty"`
	StorageUsedBytes *int64   `json:"storageUsedBytes"`
	StorageTimestamp string   `json:"storageTimestamp,omitempty"`
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

	response := s.resourceMetrics(instance.ID, instance.DataDir)
	storage := s.resourceStorage(r.Context())
	response.Sample.StorageUsedBytes = storage.worlds[instance.ID]
	response.Sample.StorageTimestamp = storage.timestamp
	machine := s.machineMetrics()
	response.Machine = &machine
	writeJSON(w, http.StatusOK, response)
}

func (s *server) resourceMetrics(instanceID, dataDir string) resourceMetricsResponse {
	return s.cachedResourceMetrics("instance:"+instanceID+":"+dataDir, func(ctx context.Context) resourceMetricsResponse {
		return s.collectResourceMetrics(ctx, instanceID, dataDir)
	})
}

func (s *server) machineMetrics() resourceMetricSample {
	return s.cachedResourceMetrics("machine", func(ctx context.Context) resourceMetricsResponse {
		return resourceMetricsResponse{Sample: collectMachineMetrics(ctx, s.config.DataDir)}
	}).Sample
}

func (s *server) cachedResourceMetrics(cacheKey string, collect func(context.Context) resourceMetricsResponse) resourceMetricsResponse {
	now := time.Now()
	s.metricsMu.Lock()
	if s.metricsCache == nil {
		s.metricsCache = make(map[string]resourceMetricsCacheEntry)
	}
	if s.metricsFlights == nil {
		s.metricsFlights = make(map[string]*resourceMetricsFlight)
	}
	if cached, ok := s.metricsCache[cacheKey]; ok && now.Before(cached.expiresAt) {
		s.metricsMu.Unlock()
		return cached.response
	}
	if flight, ok := s.metricsFlights[cacheKey]; ok {
		s.metricsMu.Unlock()
		<-flight.done
		return flight.response
	}
	flight := &resourceMetricsFlight{done: make(chan struct{})}
	s.metricsFlights[cacheKey] = flight
	s.metricsMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	flight.response = collect(ctx)
	cancel()

	s.metricsMu.Lock()
	s.metricsCache[cacheKey] = resourceMetricsCacheEntry{
		response:  flight.response,
		expiresAt: time.Now().Add(resourceMetricsCacheTTL),
	}
	delete(s.metricsFlights, cacheKey)
	close(flight.done)
	s.metricsMu.Unlock()
	return flight.response
}

func (s *server) collectResourceMetrics(ctx context.Context, instanceID, dataDir string) resourceMetricsResponse {
	machine := s.machineMetrics()
	sample := resourceMetricSample{Scope: "world", Timestamp: time.Now().UTC().Format(time.RFC3339), CPUCount: machine.CPUCount, MemoryTotalBytes: machine.MemoryTotalBytes}

	project := composeProjectStatusForWorkDir(dataDir)
	if !project.Ready {
		sample.ContainerState = "unprepared"
		sample.Message = "世界尚未准备"
		return resourceMetricsResponse{
			InstanceID:     instanceID,
			Service:        "server",
			Sample:         sample,
			StatsAvailable: true,
		}
	}

	stats, err := s.docker.ComposeStats(ctx, dataDir)
	if err != nil {
		sample.ContainerState = "unavailable"
		sample.Message = "容器指标暂不可用，请确认服务器容器已启动"
		return resourceMetricsResponse{
			InstanceID: instanceID,
			Service:    "server",
			Sample:     sample,
		}
	}

	if service, ok := selectServerStats(stats.Services); ok {
		sample.ContainerState = "running"
		sample.ContainerRunning = true
		setContainerUsage(&sample, service.CPUPerc, service.MemUsedBytes)
		sample.MemoryUsedBytes = service.MemUsedBytes
		sample.MemoryLimitBytes = service.MemLimitBytes
		return resourceMetricsResponse{
			InstanceID: instanceID,
			Service:    serviceNameForStats(service),
			Sample:     sample,
			Services:   stats.Services, StatsAvailable: true,
		}
	}

	sample.ContainerState = "stopped"
	sample.Message = "服务器容器未运行，启动后会显示 CPU 和内存趋势"
	return resourceMetricsResponse{
		InstanceID: instanceID,
		Service:    "server",
		Sample:     sample,
		Services:   stats.Services, StatsAvailable: true,
	}
}

func setContainerUsage(sample *resourceMetricSample, cpu float64, memory int64) {
	if cpu >= 0 && !math.IsNaN(cpu) && !math.IsInf(cpu, 0) {
		sample.CPUCores = floatPtr(cpu / 100)
		if sample.CPUCount > 0 {
			sample.CPUPercent = floatPtr(occupancyPercent(cpu / float64(sample.CPUCount)))
		}
	}
	sample.MemoryUsedBytes = memory
	if sample.MemoryTotalBytes > 0 && memory >= 0 {
		sample.MemoryPercent = floatPtr(occupancyPercent(float64(memory) / float64(sample.MemoryTotalBytes) * 100))
	}
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
