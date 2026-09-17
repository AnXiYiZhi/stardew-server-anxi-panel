package stardew_junimo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func TestInstanceReadCacheCoalescesAndIsolatesCancellation(t *testing.T) {
	var cache instanceReadCache[int]
	var calls atomic.Int32
	entered, release := make(chan struct{}), make(chan struct{})
	read := func(context.Context) (int, error) { calls.Add(1); close(entered); <-release; return 42, nil }
	ctx, cancel := context.WithCancel(context.Background())
	first := make(chan error, 1)
	go func() { _, err := cache.get(ctx, "a", "revision", time.Minute, read); first <- err }()
	<-entered
	other, err := cache.get(context.Background(), "b", "revision", time.Minute, func(context.Context) (int, error) { return 7, nil })
	if err != nil || other != 7 {
		t.Fatal("another instance was blocked")
	}
	cancel()
	if <-first != context.Canceled {
		t.Fatal("waiter did not cancel")
	}
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if value, err := cache.get(context.Background(), "a", "revision", time.Minute, read); err != nil || value != 42 {
				t.Error(value, err)
			}
		}()
	}
	close(release)
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatal("duplicated shared work")
	}
	value, _ := cache.get(context.Background(), "a", "changed", time.Minute, func(context.Context) (int, error) { return 99, nil })
	if value != 99 {
		t.Fatal("changed input reused stale value")
	}
}

func TestInstanceReadCacheRetriesFailureAfterAllWaitersCancel(t *testing.T) {
	var cache instanceReadCache[int]
	entered, release := make(chan struct{}), make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		_, _ = cache.get(ctx, "a", "same", time.Minute, func(context.Context) (int, error) {
			close(entered)
			<-release
			return 0, errors.New("temporary failure")
		})
	}()
	<-entered
	cancel()
	<-finished
	cache.mu.Lock()
	done := cache.entries["a"].done
	cache.mu.Unlock()
	close(release)
	<-done
	value, err := cache.get(context.Background(), "a", "same", time.Minute, func(context.Context) (int, error) { return 42, nil })
	if err != nil || value != 42 {
		t.Fatalf("failure retained after canceled waiters: %d %v", value, err)
	}
}

type countedRuntimeDocker struct {
	*smapiDetectionFakeDocker
	versions, metadata atomic.Int32
}

func (f *countedRuntimeDocker) RuntimeReadContentVersions(ctx context.Context, dir, volume, image string) (paneldocker.RuntimeContentRead, error) {
	f.versions.Add(1)
	return f.smapiDetectionFakeDocker.RuntimeReadContentManifests(ctx, dir, volume, image)
}
func (f *countedRuntimeDocker) RuntimeReadSMAPIMetadata(ctx context.Context, dir, volume, image string) (paneldocker.RuntimeSMAPIMetadata, error) {
	f.metadata.Add(1)
	return f.smapiDetectionFakeDocker.RuntimeReadSMAPIMetadata(ctx, dir, volume, image)
}
func TestRuntimeDisplaySharesInspectionButPreflightReadsFresh(t *testing.T) {
	d, instance, fake := smapiDetectionFixture(t, "4.5.2")
	counted := &countedRuntimeDocker{smapiDetectionFakeDocker: fake}
	d.docker = counted
	for i := 0; i < 4; i++ {
		result, err := d.InspectRuntimeDisplay(context.Background(), instance)
		if err != nil || result.SMAPI == nil || result.SMAPI.Current.Version != "4.5.2" {
			t.Fatal(result, err)
		}
	}
	if counted.versions.Load() != 1 || counted.metadata.Load() != 1 {
		t.Fatal("repeated component/SMAPI display probes")
	}
	if _, err := d.InspectSMAPIUpdate(context.Background(), instance); err != nil {
		t.Fatal(err)
	}
	if counted.versions.Load() != 2 || counted.metadata.Load() != 2 {
		t.Fatal("mutation inspection used display cache")
	}
	path := filepath.Join(instance.DataDir, ".env")
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = file.WriteString("\n# updated configuration\n")
	_ = file.Close()
	_, _ = d.InspectRuntimeDisplay(context.Background(), instance)
	if counted.versions.Load() != 3 {
		t.Fatal("configuration change did not invalidate display")
	}
}

func rosterCacheFixture(t testing.TB, padding int) (*Driver, registry.Instance, string) {
	t.Helper()
	dir := t.TempDir()
	folder := filepath.Join(dir, ".local-container", "saves", "Saves", "farm_123")
	if err := os.MkdirAll(folder, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(folder, "farm_123")
	raw := `<SaveGame><player><name>host</name><UniqueMultiplayerID>1</UniqueMultiplayerID></player><farmhands><Farmer><name>guest</name><UniqueMultiplayerID>2</UniqueMultiplayerID><isCustomized>true</isCustomized></Farmer></farmhands><padding>` + strings.Repeat("x", padding) + `</padding></SaveGame>`
	if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	return New(nil, nil, nil, nil), registry.Instance{ID: "a", DataDir: dir}, path
}

func TestPlayerSaveRosterCacheInvalidation(t *testing.T) {
	d, instance, path := rosterCacheFixture(t, 1024)
	first := d.cachedSaveRoster(context.Background(), instance, "farm_123")
	second := d.cachedSaveRoster(context.Background(), instance, "farm_123")
	if first != second || len(first.items) != 2 {
		t.Fatal("unchanged save was reparsed")
	}
	if err := os.WriteFile(path, []byte(`<SaveGame><player><name>changed</name></player></SaveGame>`), 0600); err != nil {
		t.Fatal(err)
	}
	third := d.cachedSaveRoster(context.Background(), instance, "farm_123")
	if third == first || len(third.items) != 1 || third.items[0].Name != "changed" {
		t.Fatal("changed save was not reloaded")
	}
	if err := os.WriteFile(path, []byte(`<SaveGame>`), 0600); err != nil {
		t.Fatal(err)
	}
	if broken := d.cachedSaveRoster(context.Background(), instance, "farm_123"); len(broken.items) != 0 {
		t.Fatal("broken save retained old membership")
	}
}

func TestSlowPlayerReadDoesNotHoldMutationLock(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	d := newTestDriver(&fakeConsoleDocker{execFunc: func(ctx context.Context, _, _, _ string, _ ...string) (paneldocker.CommandResult, error) {
		once.Do(func() {
			close(entered)
			select {
			case <-release:
			case <-ctx.Done():
			}
		})
		return paneldocker.CommandResult{Stdout: "Players: 0/4\n"}, nil
	}})
	instance := makeRunningInstance()
	instance.DataDir = t.TempDir()
	done := make(chan struct{})
	go func() { defer close(done); _, _ = d.ListPlayers(context.Background(), instance) }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("player read did not start")
	}
	if !d.runtimeUpdateMu.TryLock() {
		close(release)
		<-done
		t.Fatal("slow player read holds driver mutation mutex")
	}
	d.runtimeUpdateMu.Unlock()
	close(release)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("player read did not finish")
	}
}

func BenchmarkCachedSaveRoster10MiB(b *testing.B) {
	d, instance, _ := rosterCacheFixture(b, 10<<20)
	_ = d.cachedSaveRoster(context.Background(), instance, "farm_123")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = d.cachedSaveRoster(context.Background(), instance, "farm_123")
	}
}

type countedRosterStore struct {
	*storage.Store
	writes int
}

func (s *countedRosterStore) UpsertPlayerRoster(ctx context.Context, params storage.UpsertPlayerRosterParams) error {
	s.writes++
	return s.Store.UpsertPlayerRoster(ctx, params)
}

func (s *countedRosterStore) UpsertPlayerRosterBatch(ctx context.Context, params []storage.UpsertPlayerRosterParams) error {
	s.writes += len(params)
	return s.Store.UpsertPlayerRosterBatch(ctx, params)
}

func TestPlayerRosterUnchangedSnapshotSkipsWrites(t *testing.T) {
	ctx := context.Background()
	d, instance, _ := rosterCacheFixture(t, 0)
	dbDir := t.TempDir()
	store, err := storage.Open(ctx, config.Config{DataDir: dbDir, DBPath: filepath.Join(dbDir, "panel.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := store.EnsureDefaultInstance(ctx, storage.EnsureDefaultInstanceParams{ID: instance.ID, DriverID: DriverID, Name: "test", DataDir: instance.DataDir}); err != nil {
		t.Fatal(err)
	}
	counted := &countedRosterStore{Store: store}
	d.store = counted
	save := d.cachedSaveRoster(ctx, instance, "farm_123")
	for i := 0; i < 3; i++ {
		result := &PlayersResult{SaveID: "farm_123", UpdatedAt: "2026-09-17T00:00:00Z", Players: []PlayerInfo{{Name: "host", UniqueMultiplayerID: "1", IsHost: true, Role: "host", Status: "online", Source: "smapi_control", LastSeen: "2026-09-17T00:00:00Z"}}}
		d.persistPlayerRoster(ctx, instance, result, save, map[string]bool{})
	}
	if counted.writes != 1 {
		t.Fatalf("unchanged snapshot wrote %d times, want 1", counted.writes)
	}
}
