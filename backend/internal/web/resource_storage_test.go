package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

type resourceTestDriver struct {
	registry.GameDriver
	plans map[string]registry.ResourceStorage
}

func (d resourceTestDriver) ResourceStorage(instance registry.Instance) (registry.ResourceStorage, error) {
	return d.plans[instance.ID], nil
}

type resourceTestDocker struct {
	fakeDockerService
	volumes map[string]int64
	err     error
}

func (d resourceTestDocker) ResourceVolumeSizes(context.Context, string, string, []string) (map[string]int64, error) {
	return d.volumes, d.err
}

func TestResourceStorageWorldOwnershipSharedDeduplicationAndFailure(t *testing.T) {
	handler, store, root, cleanup := newDockerTestHandlerWithStore(t, fakeDockerService{})
	defer cleanup()
	cookie := setupDockerAdmin(t, handler)
	ctx := context.Background()
	worldA := filepath.Join(root, "instances", "stardew")
	worldB := filepath.Join(root, "instances", "world-b")
	shared := filepath.Join(root, "shared")
	for _, dir := range []string{worldA, worldB, shared} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "data"), []byte("12345"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.CreateInstance(ctx, storage.CreateInstanceParams{ID: "world-b", DriverID: storage.DefaultDriverID, Name: "B", DataDir: worldB, State: "stopped"}); err != nil {
		t.Fatal(err)
	}
	driver := resourceTestDriver{GameDriver: sj.New(fakeDockerService{}, nil, nil, store), plans: map[string]registry.ResourceStorage{
		"stardew": {Directories: []string{worldA, worldA}, Volumes: []string{"a", "a"}, SharedDirectories: []string{shared}, SharedVolumes: []string{"shared"}},
		"world-b": {Directories: []string{worldB}, Volumes: []string{"b"}, SharedDirectories: []string{shared}, SharedVolumes: []string{"shared"}},
	}}
	reg := registry.New()
	if err := reg.Register(driver); err != nil {
		t.Fatal(err)
	}
	docker := resourceTestDocker{volumes: map[string]int64{"a": 100, "b": 200, "shared": 300}}
	s := &server{store: store, registry: reg, docker: docker, config: config.Config{DataDir: root, Secret: "test-secret"}}
	result := s.collectResourceStorage(ctx)
	s.resourceStorageCache = result
	if result.worlds["stardew"] == nil || *result.worlds["stardew"] != 105 || result.worlds["world-b"] == nil || *result.worlds["world-b"] != 205 || result.games[storage.DefaultDriverID] == nil || *result.games[storage.DefaultDriverID] != 615 {
		t.Fatalf("incorrect world/shared storage: %+v", result)
	}
	response, _ := doJSON(t, http.HandlerFunc(s.handleResourceOverview), http.MethodGet, "/api/resources", nil, cookie)
	var body resourceOverview
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || body.Machine.Scope != "machine" || len(body.Games) != 1 || body.Games[0].Sample.Scope != "game" {
		t.Fatalf("overview contract: %d %s", response.Code, response.Body.String())
	}
	response, _ = doJSON(t, http.HandlerFunc(s.handleResourceOverview), http.MethodPost, "/api/resources", nil, cookie)
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST returned %d", response.Code)
	}
	docker.err = errors.New("Docker inventory interrupted after partial output")
	s.docker = docker
	s.resourceStorageCache.expires = time.Time{}
	failed := s.collectResourceStorage(ctx)
	if failed.worlds["stardew"] != nil || failed.games[storage.DefaultDriverID] != nil {
		t.Fatal("failed volume inventory displayed a partial total")
	}
}
