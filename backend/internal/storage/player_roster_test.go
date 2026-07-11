package storage

import (
	"context"
	"path/filepath"
	"testing"
)

func TestPlayerRosterUpsertPreservesIdentityHistoryAndLatestSnapshot(t *testing.T) {
	store, closeStore := newStorageTestStore(t)
	defer closeStore()
	ctx := context.Background()
	if _, err := store.EnsureDefaultInstance(ctx, EnsureDefaultInstanceParams{ID: "i1", DriverID: DefaultDriverID, Name: "test", DataDir: filepath.Join(t.TempDir(), "i1")}); err != nil {
		t.Fatal(err)
	}
	x1, x2 := 3, 9
	money1, money2 := int64(100), int64(250)
	base := UpsertPlayerRosterParams{BaseSaveID: "Farm", FullSaveID: "Farm_123", Entry: PlayerRosterEntry{
		InstanceID: "i1", StableSaveID: "Farm_123", PlayerID: "42", DisplayName: "Alice", Role: "player",
		Location: "Farm", TileX: &x1, Money: &money1, SnapshotSource: "smapi_control", SnapshotObservedAt: "2026-07-11T10:00:00Z",
	}, Online: true}
	if err := store.UpsertPlayerRoster(ctx, base); err != nil {
		t.Fatal(err)
	}
	base.Entry.DisplayName = "AliceNew"
	base.Entry.Location = "Town"
	base.Entry.TileX = &x2
	base.Entry.Money = &money2
	base.Entry.SnapshotObservedAt = "2026-07-11T11:00:00Z"
	base.Online = false
	if err := store.UpsertPlayerRoster(ctx, base); err != nil {
		t.Fatal(err)
	}
	rows, err := store.ListPlayerRoster(ctx, "i1", "Farm_123")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	got := rows[0]
	if got.DisplayName != "AliceNew" || got.FirstSeenAt != "2026-07-11T10:00:00Z" || got.LastSeenAt != "2026-07-11T11:00:00Z" || got.LastOnlineAt != "2026-07-11T10:00:00Z" {
		t.Fatalf("unexpected history: %+v", got)
	}
	if got.Location != "Town" || got.TileX == nil || *got.TileX != 9 || got.Money == nil || *got.Money != 250 {
		t.Fatalf("unexpected latest snapshot: %+v", got)
	}
	events, err := store.ListPlayerRosterEvents(ctx, "i1", "Farm_123", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].Type != "left" || events[1].Type != "seen" {
		t.Fatalf("events = %+v, want left then seen", events)
	}
	base.Entry.SnapshotObservedAt = "2026-07-11T12:00:00Z"
	base.Online = true
	if err := store.UpsertPlayerRoster(ctx, base); err != nil {
		t.Fatal(err)
	}
	events, err = store.ListPlayerRosterEvents(ctx, "i1", "Farm_123", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 || events[0].Type != "joined" {
		t.Fatalf("events after rejoin = %+v", events)
	}
}

func TestPlayerRosterPromotesTemporaryNameIdentity(t *testing.T) {
	store, closeStore := newStorageTestStore(t)
	defer closeStore()
	ctx := context.Background()
	if _, err := store.EnsureDefaultInstance(ctx, EnsureDefaultInstanceParams{ID: "i1", DriverID: DefaultDriverID, Name: "test", DataDir: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	makeParams := func(id string) UpsertPlayerRosterParams {
		return UpsertPlayerRosterParams{BaseSaveID: "Farm", FullSaveID: "Farm_1", Entry: PlayerRosterEntry{InstanceID: "i1", StableSaveID: "Farm_1", PlayerID: id, DisplayName: "Alice", SnapshotSource: "legacy_cache", SnapshotObservedAt: "2026-07-11T10:00:00Z"}}
	}
	if err := store.UpsertPlayerRoster(ctx, makeParams("name:alice")); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertPlayerRoster(ctx, makeParams("42")); err != nil {
		t.Fatal(err)
	}
	rows, err := store.ListPlayerRoster(ctx, "i1", "Farm_1")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].PlayerID != "42" {
		t.Fatalf("temporary identity was not promoted: %+v", rows)
	}
}
