package stardew_junimo

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
)

func TestUpdateServerRuntimeSettingsLinearizesWithNewGameOwner(t *testing.T) {
	dataDir := t.TempDir()
	settingsPath := serverSettingsPath(dataDir)
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	original := []byte("{\"CabinStrategy\":\"separate\",\"ExistingCabinBehavior\":\"keep\"}\n")
	if err := os.WriteFile(settingsPath, original, 0o600); err != nil {
		t.Fatal(err)
	}

	driver := New(nil, nil, nil, nil)
	// Hold the exact mutex used by Start/preclaim. The settings writer must not
	// pass its owner check and then write outside this linearization boundary.
	driver.runtimeUpdateMu.Lock()
	result := make(chan error, 1)
	go func() {
		_, err := driver.UpdateServerRuntimeSettings(context.Background(), registry.Instance{
			ID: "stardew", DataDir: dataDir,
		}, ServerRuntimeSettings{})
		result <- err
	}()
	select {
	case err := <-result:
		driver.runtimeUpdateMu.Unlock()
		t.Fatalf("settings update returned outside mutation mutex: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	seedUnfinishedNewGameOwnerForSettingsTest(t, dataDir)
	driver.runtimeUpdateMu.Unlock()
	err := <-result
	var ownerErr *NewGameOwnerError
	if !errors.As(err, &ownerErr) || ownerErr.Code != "new_game_in_progress" {
		t.Fatalf("UpdateServerRuntimeSettings error = %v, want new_game_in_progress", err)
	}
	got, readErr := os.ReadFile(settingsPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != string(original) {
		t.Fatalf("server-settings changed under new-game owner: got %q want %q", got, original)
	}
}

func seedUnfinishedNewGameOwnerForSettingsTest(t *testing.T, dataDir string) {
	t.Helper()
	transactionID, err := newGameRandomHex(16)
	if err != nil {
		t.Fatal(err)
	}
	ownerToken, err := newGameRandomHex(32)
	if err != nil {
		t.Fatal(err)
	}
	configHash, err := newGameConfigSHA256(registry.NewGameConfig{})
	if err != nil {
		t.Fatal(err)
	}
	dataDirHash, err := newGameInstanceDataDirHash(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	executorID, err := currentNewGameExecutorID()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	owner := NewGameOwnerRecord{
		SchemaVersion:       newGameOwnerSchemaVersion,
		InstanceDataDirHash: dataDirHash,
		RequestID:           "settings-linearization-test",
		ConfigSHA256:        configHash,
		TransactionID:       transactionID,
		JobID:               "job_settings_linearization",
		ExecutorID:          executorID,
		OwnerToken:          ownerToken,
		State:               newGameOwnerStateActive,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	claimed, err := claimNewGameOwner(dataDir, owner)
	if err != nil || !claimed {
		t.Fatalf("claim owner = %v, %v", claimed, err)
	}
	record := NewGameTransactionRecord{
		SchemaVersion:       newGameTransactionSchemaVersion,
		TransactionID:       transactionID,
		InstanceDataDirHash: dataDirHash,
		RequestID:           owner.RequestID,
		JobID:               owner.JobID,
		ConfigSHA256:        configHash,
		OwnerToken:          ownerToken,
		Config:              registry.NewGameConfig{},
		CreatedAt:           now,
		UpdatedAt:           now,
		Stage:               newGameStatePreparing,
	}
	payload, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(newGameTransactionsDir(dataDir), transactionID, "transaction.json")
	if err := atomicWriteValidatedJSON(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
}
