package stardew_junimo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func TestPlayerReadAcceptsConcurrentControlRefresh(t *testing.T) {
	for _, changeSave := range []bool{false, true} {
		t.Run(fmt.Sprintf("change-save-%t", changeSave), func(t *testing.T) {
			dir := t.TempDir()
			path := playerControlFilePath(dir)
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			raw := `{"updatedAt":"2026-09-17T01:00:00Z","saveId":"farm_123","players":[{"name":"host","uniqueMultiplayerId":"1","isHost":true}]}`
			if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
				t.Fatal(err)
			}
			d := newTestDriver(&fakeConsoleDocker{execFunc: func(_ context.Context, _, _, stdin string, _ ...string) (paneldocker.CommandResult, error) {
				if stdin == "info\n" {
					updated := strings.Replace(raw, "01:00:00Z", "01:00:02.100Z", 1)
					if changeSave {
						updated = strings.Replace(updated, "farm_123", "other_456", 1)
					}
					if err := os.WriteFile(path, []byte(updated), 0600); err != nil {
						return paneldocker.CommandResult{}, err
					}
				}
				return paneldocker.CommandResult{Stdout: "Players: 1/4\nOnline players: host\n"}, nil
			}})
			instance := makeRunningInstance()
			instance.DataDir = dir
			result, err := d.ListPlayers(context.Background(), instance)
			if changeSave {
				if err == nil {
					t.Fatal("cross-save observation was persisted")
				}
			} else if err != nil || result.Source != "smapi_control" || len(result.Players) != 1 {
				t.Fatalf("normal Control refresh failed: result=%+v err=%v", result, err)
			}
		})
	}
}

func TestPlayerLimitCacheLeavesLiveSnapshotsFresh(t *testing.T) {
	dir := t.TempDir()
	path := playerControlFilePath(dir)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	calls := 0
	d := newTestDriver(&fakeConsoleDocker{execFunc: func(_ context.Context, _, _, _ string, _ ...string) (paneldocker.CommandResult, error) {
		calls++
		return paneldocker.CommandResult{Stdout: "Players: 1/4\nOnline players: host\n"}, nil
	}})
	instance := makeRunningInstance()
	instance.DataDir = dir
	for i := 0; i < 3; i++ {
		raw := fmt.Sprintf(`{"updatedAt":"2026-09-17T01:00:%02dZ","saveId":"farm_123","players":[{"name":"host","uniqueMultiplayerId":"1","isHost":true,"money":%d}]}`, i*5, i)
		if err := os.WriteFile(path, []byte(raw+strings.Repeat(" ", i)), 0600); err != nil {
			t.Fatal(err)
		}
		result, err := d.ListPlayers(context.Background(), instance)
		if err != nil || result.MaxPlayers == nil || *result.MaxPlayers != 4 || result.Players[0].Money == nil || *result.Players[0].Money != int64(i) {
			t.Fatalf("stale snapshot: %+v %v", result, err)
		}
	}
	if calls != 6 {
		t.Fatalf("3 fresh snapshots executed %d commands; want 4+1+1", calls)
	}
	instance.UpdatedAt = "new-runtime-generation"
	if d.cachedLiveServerMaxPlayers(context.Background(), instance) == nil || calls != 9 {
		t.Fatal("runtime generation did not invalidate limit")
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("MAX_PLAYERS=8\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if d.cachedLiveServerMaxPlayers(context.Background(), instance) == nil || calls != 12 {
		t.Fatal("config change did not invalidate limit")
	}
}

func TestPlayerRosterThrottlesHeartbeatButPersistsChanges(t *testing.T) {
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
	for i, second := range []int{0, 5, 10, 31, 32, 33} {
		observed := fmt.Sprintf("2026-09-17T00:00:%02dZ", second)
		money := int64(100)
		if i >= 4 {
			money = 200
		}
		status := "online"
		if i == 5 {
			status = "offline"
		}
		result := &PlayersResult{SaveID: "farm_123", UpdatedAt: observed, Players: []PlayerInfo{{Name: "host", UniqueMultiplayerID: "1", IsHost: true, Role: "host", Status: status, Source: "smapi_control", LastSeen: observed, Money: &money}}}
		d.persistPlayerRoster(ctx, instance, result, save, map[string]bool{})
		want := []int{1, 1, 1, 2, 3, 4}[i]
		if counted.writes != want {
			t.Fatalf("observation %d: %d writes, want %d", i, counted.writes, want)
		}
		if status == "online" && result.Players[0].LastSeen != observed {
			t.Fatal("live heartbeat display was throttled")
		}
	}
	rows, err := store.ListPlayerRoster(ctx, instance.ID, "farm_123")
	if err != nil || len(rows) != 1 || rows[0].CurrentStatus != "offline" || *rows[0].Money != 200 {
		t.Fatalf("lost final state: %+v %v", rows, err)
	}
}
