package web

import (
	"context"
	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestResourceScopesNormalizeAndDeduplicateContainers(t *testing.T) {
	machine := resourceMetricSample{CPUCount: 8, MemoryTotalBytes: 16 * 1024}
	a := paneldocker.ComposeServiceStats{ID: "world-a", CPUPerc: 100, MemUsedBytes: 2048}
	b := paneldocker.ComposeServiceStats{ID: "world-b", CPUPerc: 50, MemUsedBytes: 1024}
	shared := paneldocker.ComposeServiceStats{ID: "shared", CPUPerc: 10, MemUsedBytes: 256}
	worlds := []resourceMetricsResponse{
		{StatsAvailable: true, Services: []paneldocker.ComposeServiceStats{a, shared}},
		{StatsAvailable: true, Services: []paneldocker.ComposeServiceStats{b, shared}},
	}
	sample := aggregateGameMetrics(worlds, machine)
	if sample.CPUPercent == nil || *sample.CPUPercent != 20 || sample.MemoryUsedBytes != 3328 || sample.MemoryPercent == nil || *sample.MemoryPercent != 20.3 {
		t.Fatalf("wrong game total: %+v", sample)
	}
	if sample.DiskPercent != nil || sample.DiskTotalBytes != 0 {
		t.Fatal("game inherited host filesystem usage")
	}
	worlds[1].StatsAvailable = false
	partial := aggregateGameMetrics(worlds, machine)
	if partial.CPUPercent != nil || partial.MemoryPercent != nil || partial.Message == "" {
		t.Fatal("partial result was presented as complete")
	}
	stopped := aggregateGameMetrics([]resourceMetricsResponse{{StatsAvailable: true}}, machine)
	if stopped.CPUPercent == nil || *stopped.CPUPercent != 0 || stopped.MemoryPercent == nil || *stopped.MemoryPercent != 0 || stopped.ContainerRunning {
		t.Fatal("known stopped game must be a true zero")
	}
}

func TestResourceDirectoryScopeAndCancellation(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "saves")
	if err := os.Mkdir(child, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(child, "save"), []byte("12345"), 0600); err != nil {
		t.Fatal(err)
	}
	roots := uniqueResourceRoots([]string{child, root, root})
	if len(roots) != 1 || roots[0] != root {
		t.Fatalf("nested roots double counted: %v", roots)
	}
	if size := measureResourceDirectory(context.Background(), root); size == nil || *size != 5 {
		t.Fatalf("wrong owned file size: %v", size)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if size := measureResourceDirectory(ctx, root); size != nil {
		t.Fatal("canceled scan produced an authoritative total")
	}
	if size := measureResourceDirectory(context.Background(), filepath.Join(root, "absent")); size == nil || *size != 0 {
		t.Fatal("missing optional data should be zero")
	}
}

func TestMachineCPUCounterDelta(t *testing.T) {
	before := machineCounters{total: 100, idle: 70}
	if p := machineCPUPercent(before, machineCounters{total: 200, idle: 145}); p == nil || *p != 25 {
		t.Fatalf("CPU delta %v", p)
	}
	for _, after := range []machineCounters{before, {total: 99, idle: 70}, {total: 200, idle: 200}} {
		if machineCPUPercent(before, after) != nil {
			t.Fatal("invalid counters displayed as zero")
		}
	}
}

func TestResourceOverviewRequiresAuthentication(t *testing.T) {
	handler, _, closeStore := newDockerTestHandler(t, fakeDockerService{})
	defer closeStore()
	setupDockerAdmin(t, handler)
	response, _ := doJSON(t, handler, http.MethodGet, "/api/resources", nil, nil)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("resources returned %d", response.Code)
	}
}
