package web

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
)

type blockedResourceDriver struct {
	registry.GameDriver
	entered chan struct{}
	release chan struct{}
	calls   atomic.Int32
}

func (d *blockedResourceDriver) ResourceStorage(registry.Instance) (registry.ResourceStorage, error) {
	if d.calls.Add(1) == 1 {
		close(d.entered)
	}
	<-d.release
	return registry.ResourceStorage{}, nil
}

func TestResourceStorageNeverBlocksReaders(t *testing.T) {
	_, store, root, cleanup := newDockerTestHandlerWithStore(t, fakeDockerService{})
	defer cleanup()
	s := &server{store: store, config: config.Config{DataDir: root}}
	original := sj.New(fakeDockerService{}, nil, nil, store)
	driver := &blockedResourceDriver{GameDriver: original, entered: make(chan struct{}), release: make(chan struct{})}
	reg := registry.New()
	if err := reg.Register(driver); err != nil {
		t.Fatal(err)
	}
	s.registry = reg
	old := int64(42)
	s.resourceStorageCache = resourceStorageSnapshot{worlds: map[string]*int64{"stardew": &old}, timestamp: "old"}
	defer func() {
		close(driver.release)
		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			s.resourceStorageMu.Lock()
			active := s.resourceStorageRefreshing
			s.resourceStorageMu.Unlock()
			if !active {
				return
			}
			time.Sleep(time.Millisecond)
		}
		t.Error("background scan did not complete")
	}()
	first := s.resourceStorage(context.Background())
	if first.timestamp != "old" {
		t.Fatal("lost last sample")
	}
	select {
	case <-driver.entered:
	case <-time.After(time.Second):
		t.Fatal("scan not started")
	}
	for i := 0; i < 20; i++ {
		if sample := s.resourceStorage(context.Background()); sample.timestamp != "old" || *sample.worlds["stardew"] != 42 {
			t.Fatal("stale sample changed")
		}
	}
	if driver.calls.Load() != 1 {
		t.Fatal("duplicate storage scans")
	}
}
