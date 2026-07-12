package web

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func TestControlCommandHistoryEndpointIncludesLegacyCompatibility(t *testing.T) {
	handler, store, cleanup := newTestHandlerWithStore(t)
	defer cleanup()
	_, cookie := doJSON(t, handler, http.MethodPost, "/api/setup/admin", map[string]string{"username": "admin", "password": "admin-password", "confirmPassword": "admin-password"}, nil)
	instance := ensureCommandSyncInstance(t, store)
	now := time.Now().UTC()
	if err := store.CreateControlCommand(context.Background(), storage.CreateControlCommandParams{CommandID: "legacy-command", InstanceID: instance.ID, CommandType: "broadcast", TargetType: "audience", TargetLabel: "全服玩家", Status: "dispatched", ResultSupported: false, SubmittedAt: now}); err != nil {
		t.Fatal(err)
	}
	response, _ := doJSON(t, handler, http.MethodGet, "/api/instances/stardew/control-commands", nil, cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("history status=%d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Commands []storage.ControlCommand `json:"commands"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Commands) != 1 || payload.Commands[0].ResultSupported || payload.Commands[0].Status != "dispatched" {
		t.Fatalf("unexpected legacy history: %#v", payload.Commands)
	}
}

func TestSyncControlCommandResultsImportsIdempotentlyAndDeletesTerminalFile(t *testing.T) {
	_, store, cleanup := newTestHandlerWithStore(t)
	defer cleanup()
	instance := ensureCommandSyncInstance(t, store)
	now := time.Now().UTC()
	if err := store.CreateControlCommand(context.Background(), storage.CreateControlCommandParams{CommandID: "0123456789abcdef0123456789abcdef", InstanceID: instance.ID, CommandType: "kick", Status: "queued", ResultSupported: true, SubmittedAt: now}); err != nil {
		t.Fatal(err)
	}
	path := writeCommandResultFixture(t, instance.DataDir, sj.CommandOutcome{CommandID: "0123456789abcdef0123456789abcdef", Status: sj.CommandStatusSucceeded, ErrorCode: "ok", Message: "done", CreatedAt: now, UpdatedAt: now.Add(time.Second)})
	if err := syncControlCommandResults(context.Background(), store, instance, slog.Default()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("terminal result should be removed after import: %v", err)
	}
	if err := syncControlCommandResults(context.Background(), store, instance, slog.Default()); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetControlCommand(context.Background(), "0123456789abcdef0123456789abcdef")
	if err != nil || got.Status != "succeeded" {
		t.Fatalf("persisted result=%#v err=%v", got, err)
	}
}

func TestSyncKeepsRunningResultWhileCommandFileExists(t *testing.T) {
	_, store, cleanup := newTestHandlerWithStore(t)
	defer cleanup()
	instance := ensureCommandSyncInstance(t, store)
	id := "fedcba9876543210fedcba9876543210"
	now := time.Now().UTC()
	resultPath := writeCommandResultFixture(t, instance.DataDir, sj.CommandOutcome{CommandID: id, Status: sj.CommandStatusRunning, CreatedAt: now, UpdatedAt: now})
	commandsDir := filepath.Join(instance.DataDir, ".local-container", "control", "commands")
	if err := os.MkdirAll(commandsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	commandPath := filepath.Join(commandsDir, "20260712-"+id+".json")
	if err := os.WriteFile(commandPath, []byte(`{"id":"`+id+`","command":"save-now"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := syncControlCommandResults(context.Background(), store, instance, slog.Default()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(resultPath); err != nil {
		t.Fatalf("running result gate must remain: %v", err)
	}
	if err := os.Remove(commandPath); err != nil {
		t.Fatal(err)
	}
	if err := syncControlCommandResults(context.Background(), store, instance, slog.Default()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(resultPath); !os.IsNotExist(err) {
		t.Fatalf("orphaned imported running result should be removable: %v", err)
	}
}

func ensureCommandSyncInstance(t *testing.T, store *storage.Store) storage.Instance {
	t.Helper()
	instance, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{ID: storage.DefaultInstanceID, DriverID: sj.DriverID, Name: "test", DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	return instance
}

func writeCommandResultFixture(t *testing.T, dataDir string, outcome sj.CommandOutcome) string {
	t.Helper()
	dir := filepath.Join(dataDir, ".local-container", "control", "command-results")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(outcome)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, outcome.CommandID+".json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
