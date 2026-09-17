//go:build integration

package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func TestResourceScopesRealDocker(t *testing.T) {
	handler, store, root, cleanup := newDockerTestHandlerWithStore(t, fakeDockerService{})
	defer cleanup()
	cookie := setupDockerAdmin(t, handler)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	prefix := fmt.Sprintf("anxi-resource-%d", time.Now().UnixNano())
	client := paneldocker.NewClient(paneldocker.Options{})
	volumes := []string{prefix + "-a", prefix + "-b", prefix + "-shared"}
	// Every Docker resource is synthetic, uniquely named and created by this test.
	t.Cleanup(func() {
		for _, name := range volumes {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 20*time.Second)
			out, err := exec.CommandContext(cleanupCtx, "docker", "volume", "rm", name).CombinedOutput()
			cleanupCancel()
			if err != nil {
				t.Errorf("cleanup fixture volume: %v %s", err, out)
			}
		}
	})
	for _, name := range volumes {
		if out, err := exec.CommandContext(ctx, "docker", "volume", "create", "--label", "anxi.resource-test="+prefix, name).CombinedOutput(); err != nil {
			t.Fatalf("create fixture volume: %v %s", err, out)
		}
	}
	worldA := filepath.Join(root, "instances", "stardew")
	worldB := filepath.Join(root, "instances", "world-b")
	if _, err := store.CreateInstance(ctx, storage.CreateInstanceParams{ID: "world-b", DriverID: storage.DefaultDriverID, Name: "B", DataDir: worldB, State: "stopped"}); err != nil {
		t.Fatal(err)
	}
	plans := map[string]registry.ResourceStorage{}
	for i, dir := range []string{worldA, worldB} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		compose := fmt.Sprintf("name: %s-%d\nservices:\n  server:\n    image: alpine:3.20\n    pull_policy: never\n    command: [sh, -c, 'dd if=/dev/zero of=/owned/data bs=4096 count=1 && exec sleep 600']\n    volumes: [owned:/owned, shared:/shared]\nvolumes:\n  owned:\n    external: true\n    name: %s\n  shared:\n    external: true\n    name: %s\n", prefix, i, volumes[i], volumes[2])
		if err := os.WriteFile(filepath.Join(dir, "compose.yml"), []byte(compose), 0600); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cleanupCancel()
			if _, err := client.ComposeDown(cleanupCtx, dir); err != nil {
				t.Errorf("cleanup fixture compose: %v", err)
			}
		})
		if _, err := client.ComposeUp(ctx, dir); err != nil {
			t.Fatal(err)
		}
		plans[filepath.Base(dir)] = registry.ResourceStorage{Directories: []string{dir}, Volumes: []string{volumes[i]}, SharedVolumes: []string{volumes[2]}, ProbeImage: "alpine:3.20"}
	}
	reg := registry.New()
	if err := reg.Register(resourceTestDriver{GameDriver: sj.New(client, nil, nil, store), plans: plans}); err != nil {
		t.Fatal(err)
	}
	s := &server{store: store, registry: reg, docker: client, config: config.Config{DataDir: root, Secret: "test-secret"}}
	response, _ := doJSON(t, http.HandlerFunc(s.handleResourceOverview), http.MethodGet, "/api/resources", nil, cookie)
	var overview resourceOverview
	if err := json.Unmarshal(response.Body.Bytes(), &overview); err != nil {
		t.Fatal(err)
	}
	if response.Code != 200 || overview.Machine.CPUPercent == nil || overview.Machine.MemoryPercent == nil || len(overview.Games) != 1 {
		t.Fatalf("overview unavailable: %s", response.Body.String())
	}
	// Storage is a separately refreshed sample; a cold HTTP read may return
	// CPU/memory before Docker's disk inventory completes.
	deadline := time.Now().Add(20 * time.Second)
	for overview.Games[0].Sample.StorageUsedBytes == nil && time.Now().Before(deadline) {
		time.Sleep(25 * time.Millisecond)
		response, _ = doJSON(t, http.HandlerFunc(s.handleResourceOverview), http.MethodGet, "/api/resources", nil, cookie)
		if err := json.Unmarshal(response.Body.Bytes(), &overview); err != nil {
			t.Fatal(err)
		}
	}
	readWorld := func(id string) resourceMetricsResponse {
		r, _ := doJSON(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { s.handleInstanceMetrics(w, r, id) }), http.MethodGet, "/api/instances/"+id+"/metrics", nil, cookie)
		var result resourceMetricsResponse
		if r.Code != 200 {
			t.Fatalf("world status %d", r.Code)
		}
		if err := json.Unmarshal(r.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	a, b := readWorld("stardew"), readWorld("world-b")
	game := overview.Games[0].Sample
	if !a.Sample.ContainerRunning || !b.Sample.ContainerRunning || game.MemoryUsedBytes != a.Sample.MemoryUsedBytes+b.Sample.MemoryUsedBytes {
		t.Fatalf("world CPU/memory aggregation failed: game=%+v a=%+v b=%+v", game, a.Sample, b.Sample)
	}
	if game.StorageUsedBytes == nil || a.Sample.StorageUsedBytes == nil || b.Sample.StorageUsedBytes == nil || *a.Sample.StorageUsedBytes < 4096 || *game.StorageUsedBytes != *a.Sample.StorageUsedBytes+*b.Sample.StorageUsedBytes {
		t.Fatal("real volume storage or shared deduplication incorrect")
	}
	if _, err := client.ComposeDown(ctx, worldA); err != nil {
		t.Fatal(err)
	}
	s.metricsCache = nil
	afterA, afterB := readWorld("stardew"), readWorld("world-b")
	if afterA.Sample.ContainerRunning || afterA.Sample.CPUPercent != nil || !afterB.Sample.ContainerRunning || afterA.Sample.StorageUsedBytes == nil || *afterA.Sample.StorageUsedBytes != *a.Sample.StorageUsedBytes {
		t.Fatal("stopping a world altered its storage or the other world's running state")
	}
	if afterA.Sample.DiskPercent != nil || afterB.Sample.DiskPercent != nil {
		t.Fatal("world inherited machine disk usage")
	}
}
