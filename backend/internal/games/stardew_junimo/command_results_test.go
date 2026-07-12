package stardew_junimo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWritePanelCommandUniqueIDInAtomicJSON(t *testing.T) {
	dataDir := t.TempDir()
	first, err := writePanelCommand(dataDir, "kick", map[string]string{"name": "Leah"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := writePanelCommand(dataDir, "kick", nil)
	if err != nil {
		t.Fatal(err)
	}
	if first == second || !validCommandID(first) || !validCommandID(second) {
		t.Fatalf("IDs must be unique stable 128-bit hex values: %q %q", first, second)
	}
	dir := filepath.Join(controlDir(dataDir), "commands")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".json" {
			t.Fatalf("temporary command file leaked: %s", entry.Name())
		}
		raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		var command struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(raw, &command) != nil || !validCommandID(command.ID) {
			t.Fatalf("command ID missing from JSON: %s", raw)
		}
	}
}

func TestGetCommandOutcomeStatesAndCleanup(t *testing.T) {
	dataDir := t.TempDir()
	id, err := writePanelCommand(dataDir, "broadcast", nil)
	if err != nil {
		t.Fatal(err)
	}
	assertStatus := func(want CommandStatus) {
		t.Helper()
		got, err := GetCommandOutcome(dataDir, id)
		if err != nil || got.Status != want {
			t.Fatalf("status = %q, err=%v; want %q", got.Status, err, want)
		}
	}
	assertStatus(CommandStatusQueued)

	resultPath := filepath.Join(commandResultsDir(dataDir), id+".json")
	now := time.Now().UTC()
	for _, status := range []CommandStatus{CommandStatusSucceeded, CommandStatusFailed, CommandStatusRunning} {
		if err := writeJSONAtomic(resultPath, CommandOutcome{CommandID: id, Status: status, ErrorCode: "ok", UpdatedAt: now}); err != nil {
			t.Fatal(err)
		}
		assertStatus(status)
		outcome, err := GetCommandOutcome(dataDir, id)
		if err != nil || outcome.ErrorCode != "ok" {
			t.Fatalf("structured error code was not preserved: %+v, %v", outcome, err)
		}
	}
	if err := os.Remove(resultPath); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Join(controlDir(dataDir), "commands"))
	for _, entry := range entries {
		_ = os.Remove(filepath.Join(controlDir(dataDir), "commands", entry.Name()))
	}
	assertStatus(CommandStatusUnknown)

	old := now.Add(-commandResultTTL - time.Hour)
	if err := writeJSONAtomic(resultPath, CommandOutcome{CommandID: id, Status: CommandStatusSucceeded, UpdatedAt: old}); err != nil {
		t.Fatal(err)
	}
	// 查询是只读的；过期转换由清理任务负责，避免查询时抢先删除尚未入库的结果。
	if err := cleanupCommandResults(dataDir, now); err != nil {
		t.Fatal(err)
	}
	assertStatus(CommandStatusExpired)
}

func TestStaleRunningCommandBecomesUnknownWithoutRetry(t *testing.T) {
	dataDir := t.TempDir()
	id, err := writePanelCommand(dataDir, "kick", nil)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(commandResultsDir(dataDir), id+".json")
	if err := writeJSONAtomic(path, CommandOutcome{
		CommandID: id,
		Status:    CommandStatusRunning,
		UpdatedAt: time.Now().UTC().Add(-commandRunningStale - time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	if err := cleanupCommandResults(dataDir, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	outcome, err := GetCommandOutcome(dataDir, id)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Status != CommandStatusUnknown || outcome.ErrorCode != "execution_interrupted" {
		t.Fatalf("outcome = %+v", outcome)
	}
	exists, err := commandFileExists(dataDir, id)
	if err != nil || !exists {
		t.Fatalf("ambiguous command must remain non-retried for the mod result gate: exists=%v err=%v", exists, err)
	}
}

func TestCleanupDoesNotRemoveActiveCommandResult(t *testing.T) {
	dataDir := t.TempDir()
	id := "0123456789abcdef0123456789abcdef"
	path := filepath.Join(commandResultsDir(dataDir), id+".json")
	old := time.Now().UTC().Add(-commandResultTTL - time.Hour)
	if err := writeJSONAtomic(path, CommandOutcome{CommandID: id, Status: CommandStatusQueued, UpdatedAt: old}); err != nil {
		t.Fatal(err)
	}
	if err := cleanupCommandResults(dataDir, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("active result was removed: %v", err)
	}
}

func TestLegacyModSubmissionOmitsQueuedStatus(t *testing.T) {
	dataDir := t.TempDir()
	if got := submissionStatus(dataDir); got != "" {
		t.Fatalf("legacy instance status = %q", got)
	}
	if err := os.MkdirAll(controlDir(dataDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(controlDir(dataDir), "status.json"), []byte(`{"commandResultVersion":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := submissionStatus(dataDir); got != CommandStatusQueued {
		t.Fatalf("new instance status = %q", got)
	}
}
