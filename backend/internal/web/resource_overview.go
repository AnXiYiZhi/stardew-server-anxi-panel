package web

import (
	"net/http"
	"sort"
	"sync"
	"time"
)

type gameResourceMetrics struct {
	DriverID   string               `json:"driverId"`
	WorldCount int                  `json:"worldCount"`
	Sample     resourceMetricSample `json:"sample"`
}

type resourceOverview struct {
	Machine resourceMetricSample  `json:"machine"`
	Games   []gameResourceMetrics `json:"games"`
}

func (s *server) handleResourceOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if _, ok := s.requireAuth(w, r); !ok {
		return
	}
	instances, err := s.store.ListInstances(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "无法读取资源范围")
		return
	}
	response := resourceOverview{Machine: s.machineMetrics(), Games: []gameResourceMetrics{}}
	// Limit concurrent Compose invocations even on hosts with many worlds.
	samples := make([]resourceMetricsResponse, len(instances))
	queue := make(chan int)
	var workers sync.WaitGroup
	for range min(4, len(instances)) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for i := range queue {
				samples[i] = s.resourceMetrics(instances[i].ID, instances[i].DataDir)
			}
		}()
	}
	for i := range instances {
		select {
		case queue <- i:
		case <-r.Context().Done():
			close(queue)
			workers.Wait()
			return
		}
	}
	close(queue)
	workers.Wait()
	storage := s.resourceStorage(r.Context())
	groups := make(map[string][]resourceMetricsResponse)
	for i, instance := range instances {
		groups[instance.DriverID] = append(groups[instance.DriverID], samples[i])
	}
	for id, values := range groups {
		sample := aggregateGameMetrics(values, response.Machine)
		sample.StorageUsedBytes, sample.StorageTimestamp = storage.games[id], storage.timestamp
		response.Games = append(response.Games, gameResourceMetrics{DriverID: id, WorldCount: len(values), Sample: sample})
	}
	sort.Slice(response.Games, func(i, j int) bool { return response.Games[i].DriverID < response.Games[j].DriverID })
	writeJSON(w, http.StatusOK, response)
}

func aggregateGameMetrics(worlds []resourceMetricsResponse, machine resourceMetricSample) resourceMetricSample {
	sample := resourceMetricSample{Scope: "game", Timestamp: time.Now().UTC().Format(time.RFC3339), CPUCount: machine.CPUCount, MemoryTotalBytes: machine.MemoryTotalBytes}
	var cpu float64
	var memory int64
	seen := map[string]bool{}
	available := true
	for _, world := range worlds {
		if !world.StatsAvailable {
			available = false
			continue
		}
		for _, service := range world.Services {
			id := service.ID
			if id == "" {
				id = service.Name
			}
			if id == "" {
				id = service.Container
			}
			if id != "" && seen[id] {
				continue
			}
			if id != "" {
				seen[id] = true
			}
			cpu += service.CPUPerc
			memory += service.MemUsedBytes
			sample.ContainerRunning = true
		}
	}
	if available {
		setContainerUsage(&sample, cpu, memory)
	} else {
		sample.Message = "部分世界的资源暂不可用"
	}
	return sample
}
