package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
)

func TestControlCommandMigrationAndIdempotentImport(t *testing.T) {
	store, closeStore := newStorageTestStore(t)
	defer closeStore()
	ctx := context.Background()
	ensureControlCommandTestInstance(t, store)
	now := time.Now().UTC().Truncate(time.Millisecond)
	p := CreateControlCommandParams{CommandID: "cmd-idempotent", InstanceID: DefaultInstanceID, CommandType: "kick", TargetType: "player", TargetID: "42", TargetLabel: "Leah", Status: "queued", ResultSupported: true, SubmittedAt: now}
	if err := store.CreateControlCommand(ctx, p); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateControlCommand(ctx, p); err != nil {
		t.Fatal(err)
	}
	result := ImportControlCommandResultParams{CommandID: p.CommandID, InstanceID: p.InstanceID, Status: "succeeded", ErrorCode: "ok", ResultMessage: "done", ResultDetails: map[string]string{"playerId": "42"}, CreatedAt: now, UpdatedAt: now.Add(time.Second), ImportedAt: now.Add(2 * time.Second)}
	if err := store.ImportControlCommandResult(ctx, result); err != nil {
		t.Fatal(err)
	}
	if err := store.ImportControlCommandResult(ctx, result); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetControlCommand(ctx, p.CommandID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "succeeded" || got.ErrorCode != "ok" || got.ResultDetails["playerId"] != "42" || got.CompletedAt == nil {
		t.Fatalf("unexpected persisted result: %#v", got)
	}
	var auditCount int
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_logs WHERE action = 'control_command_completed' AND target_id = ?`, p.CommandID).Scan(&auditCount); err != nil || auditCount != 1 {
		t.Fatalf("final audit count=%d err=%v", auditCount, err)
	}
}

func TestControlCommandSubmissionEnrichesResultImportedFirst(t *testing.T) {
	store, closeStore := newStorageTestStore(t)
	defer closeStore()
	ctx := context.Background()
	ensureControlCommandTestInstance(t, store)
	now := time.Now().UTC()
	id := "cmd-result-first"
	if err := store.ImportControlCommandResult(ctx, ImportControlCommandResultParams{CommandID: id, InstanceID: DefaultInstanceID, Status: "succeeded", ErrorCode: "ok", CreatedAt: now, UpdatedAt: now.Add(time.Second), ImportedAt: now.Add(2 * time.Second)}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateControlCommand(ctx, CreateControlCommandParams{CommandID: id, InstanceID: DefaultInstanceID, CommandType: "broadcast", TargetType: "audience", TargetLabel: "全服玩家", ActorUsername: "admin", Status: "queued", ResultSupported: true, SubmittedAt: now}); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetControlCommand(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "succeeded" || got.CommandType != "broadcast" || got.TargetLabel != "全服玩家" || got.ActorUsername != "admin" {
		t.Fatalf("submission did not enrich imported result: %#v", got)
	}
	var auditCount int
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_logs WHERE action = 'control_command_completed' AND target_id = ?`, id).Scan(&auditCount); err != nil || auditCount != 1 {
		t.Fatalf("result-first final audit count=%d err=%v", auditCount, err)
	}
}

func TestControlCommandPersistsAcrossStoreRestart(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	cfg := config.Config{DataDir: dataDir, DBPath: filepath.Join(dataDir, "panel.db")}
	store, err := Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	ensureControlCommandTestInstance(t, store)
	now := time.Now().UTC()
	if err := store.CreateControlCommand(ctx, CreateControlCommandParams{CommandID: "cmd-restart", InstanceID: DefaultInstanceID, CommandType: "save-now", Status: "queued", ResultSupported: true, SubmittedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if got, err := store.GetControlCommand(ctx, "cmd-restart"); err != nil || got.Status != "queued" {
		t.Fatalf("restart result=%#v err=%v", got, err)
	}
}

func TestCleanupControlCommandsPreservesActiveAndAppliesAgeOrCount(t *testing.T) {
	store, closeStore := newStorageTestStore(t)
	defer closeStore()
	ctx := context.Background()
	ensureControlCommandTestInstance(t, store)
	now := time.Now().UTC()
	for _, item := range []struct {
		id, status string
		at         time.Time
	}{
		{"queued-old", "queued", now.AddDate(0, 0, -60)},
		{"running-old", "running", now.AddDate(0, 0, -60)},
		{"terminal-old", "succeeded", now.AddDate(0, 0, -60)},
		{"terminal-new-a", "failed", now.Add(-2 * time.Minute)},
		{"terminal-new-b", "dispatched", now.Add(-time.Minute)},
	} {
		if err := store.CreateControlCommand(ctx, CreateControlCommandParams{CommandID: item.id, InstanceID: DefaultInstanceID, CommandType: "test", Status: item.status, ResultSupported: true, SubmittedAt: item.at}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.CleanupControlCommands(ctx, now.AddDate(0, 0, -30), 1); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"queued-old", "running-old", "terminal-new-b"} {
		if _, err := store.GetControlCommand(ctx, id); err != nil {
			t.Fatalf("%s should remain: %v", id, err)
		}
	}
	for _, id := range []string{"terminal-old", "terminal-new-a"} {
		if _, err := store.GetControlCommand(ctx, id); err != ErrNotFound {
			t.Fatalf("%s should be deleted: %v", id, err)
		}
	}
}

func ensureControlCommandTestInstance(t *testing.T, store *Store) {
	t.Helper()
	_, err := store.EnsureDefaultInstance(context.Background(), EnsureDefaultInstanceParams{ID: DefaultInstanceID, DriverID: DefaultDriverID, Name: "test", DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
}
