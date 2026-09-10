package stardew_junimo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func TestSplitCurlResponsePreservesJSONAndStatus(t *testing.T) {
	body, status, err := splitCurlResponse(paneldocker.CommandResult{Stdout: `{"success":false,"error":"currently online"}` + "\n200\n", ExitCode: 0})
	if err != nil || status != "200" || body != `{"success":false,"error":"currently online"}` {
		t.Fatalf("body=%q status=%q err=%v", body, status, err)
	}
}

func TestVerifyFarmhandAbsentOnDisk(t *testing.T) {
	dataDir := t.TempDir()
	saveName := "Farm_1"
	saveDir := filepath.Join(dataDir, ".local-container", "saves", "Saves", saveName)
	if err := os.MkdirAll(saveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	withFarmhand := `<SaveGame><player><name>Host</name><UniqueMultiplayerID>1</UniqueMultiplayerID></player><farmhands><Farmer><name>Alice</name><UniqueMultiplayerID>42</UniqueMultiplayerID></Farmer></farmhands></SaveGame>`
	path := filepath.Join(saveDir, saveName)
	if err := os.WriteFile(path, []byte(withFarmhand), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyFarmhandAbsentOnDisk(dataDir, saveName, "42"); err == nil {
		t.Fatal("expected present farmhand verification to fail")
	}
	withoutFarmhand := `<SaveGame><player><name>Host</name><UniqueMultiplayerID>1</UniqueMultiplayerID></player><farmhands></farmhands></SaveGame>`
	if err := os.WriteFile(path, []byte(withoutFarmhand), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyFarmhandAbsentOnDisk(dataDir, saveName, "42"); err != nil {
		t.Fatalf("absent farmhand verification failed: %v", err)
	}
}

func TestMarkSaveCharacterCapabilities(t *testing.T) {
	dataDir := t.TempDir()
	saveName := "Farm_1"
	saveDir := filepath.Join(dataDir, ".local-container", "saves", "Saves", saveName)
	if err := os.MkdirAll(saveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	raw := `<SaveGame><player><name>Host</name><UniqueMultiplayerID>1</UniqueMultiplayerID></player><farmhands><Farmer><name>Alice</name><UniqueMultiplayerID>42</UniqueMultiplayerID></Farmer></farmhands></SaveGame>`
	if err := os.WriteFile(filepath.Join(saveDir, saveName), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	players := []PlayerInfo{
		{Name: "Host", UniqueMultiplayerID: "1", IsHost: true, Status: "online"},
		{Name: "Alice", UniqueMultiplayerID: "42", Status: "offline"},
		{Name: "History", UniqueMultiplayerID: "99", Status: "offline", Source: "sqlite_roster"},
	}
	markSaveCharacterCapabilities(dataDir, saveName, players)
	if !players[0].SaveCharacterPresent || players[0].CanDeleteCharacter || players[0].DeleteCharacterBlockReason != "host_not_supported" {
		t.Fatalf("host capability = %+v", players[0])
	}
	if !players[1].SaveCharacterPresent || !players[1].CanDeleteCharacter {
		t.Fatalf("offline farmhand capability = %+v", players[1])
	}
	if players[2].SaveCharacterPresent || players[2].CanDeleteCharacter || players[2].DeleteCharacterBlockReason != "character_not_in_save" {
		t.Fatalf("history-only capability = %+v", players[2])
	}
}

func TestConnectedHumansCountsConnectionsAndRejectsTarget(t *testing.T) {
	players := []PlayerInfo{
		{UniqueMultiplayerID: "host", IsHost: true, Status: "online"},
		{UniqueMultiplayerID: "target", Status: "offline"},
		{UniqueMultiplayerID: "other-a", Status: "online"},
		{UniqueMultiplayerID: "other-b", Status: "online"},
	}
	connected, err := connectedHumans(players, "target")
	if err != nil || len(connected) != 2 {
		t.Fatalf("expected two online humans, connected=%v err=%v", connected, err)
	}
	players[1].Status = "online"
	_, err = connectedHumans(players, "target")
	var commandErr *CommandError
	if !errors.As(err, &commandErr) || commandErr.Code != "farmhand_online" {
		t.Fatalf("expected farmhand_online, got %v", err)
	}
}

func TestFarmhandDeleteCountdownNotifiesEveryTenSeconds(t *testing.T) {
	want := []int{60, 50, 40, 30, 20, 10}
	if got := farmhandDeleteCountdownValues(); !reflect.DeepEqual(got, want) {
		t.Fatalf("countdown values = %v, want %v", got, want)
	}
}

func TestFarmhandDeleteTargetedSaveBindsOperationAndWorld(t *testing.T) {
	dataDir := t.TempDir()
	runner := farmhandDeleteRunner{
		instance:     registry.Instance{ID: "world", DataDir: dataDir, State: storage.InstanceStateRunning},
		operationID:  "0123456789abcdef0123456789abcdef",
		expectedSave: "Farm_1",
	}
	commandID, err := runner.requestTargetedSave()
	if err != nil {
		t.Fatal(err)
	}
	paths, err := filepath.Glob(filepath.Join(controlDir(dataDir), "commands", "*.json"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("command paths = %v, err = %v", paths, err)
	}
	var command struct {
		ID      string            `json:"id"`
		Name    string            `json:"name"`
		Payload map[string]string `json:"payload"`
	}
	raw, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &command); err != nil {
		t.Fatal(err)
	}
	if command.ID != commandID || command.Name != "save-now" ||
		command.Payload["transactionId"] != runner.operationID || command.Payload["saveId"] != runner.expectedSave {
		t.Fatalf("targeted save command = %+v", command)
	}
}

func TestFarmhandDeleteCancellationClassification(t *testing.T) {
	for _, code := range []string{"sleep_in_progress", "day_transition_in_progress", "active_save_changed", "farmhand_online"} {
		if !isFarmhandDeleteCancellation(fmt.Errorf("wrapped: %w", &CommandError{Code: code})) {
			t.Fatalf("%s was not classified as cancellation", code)
		}
	}
	if isFarmhandDeleteCancellation(&CommandError{Code: "predelete_save_failed"}) {
		t.Fatal("save failure was classified as cancellation")
	}
}

func TestLocalMaintenanceCancellationStopsDisconnectLoop(t *testing.T) {
	dataDir := t.TempDir()
	operationID := "0123456789abcdef0123456789abcdef"
	marker := farmhandDeleteMaintenanceMarker{
		SchemaVersion: 1, OperationID: operationID, ExpectedSaveID: "Farm_1", TargetPlayerID: "42",
		Phase: "canceled", CancellationCode: "sleep_in_progress", CancellationMessage: "sleep started",
	}
	raw, err := json.Marshal(marker)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(controlDir(dataDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(controlDir(dataDir), "farmhand-delete-maintenance.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	runner := farmhandDeleteRunner{instance: registry.Instance{DataDir: dataDir}, operationID: operationID}
	err = runner.checkLocalMaintenanceCancellation()
	var commandErr *CommandError
	if !errors.As(err, &commandErr) || commandErr.Code != "sleep_in_progress" {
		t.Fatalf("local cancellation = %v", err)
	}
}

func TestInterruptedFarmhandDeleteReconciliation(t *testing.T) {
	dataDir := t.TempDir()
	store, err := storage.Open(context.Background(), config.Config{
		Addr: ":0", DataDir: dataDir, DBPath: filepath.Join(dataDir, "panel.db"), Secret: "test-secret", Version: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	instanceDir := filepath.Join(dataDir, "instances", "world")
	if _, err := store.CreateInstance(ctx, storage.CreateInstanceParams{
		ID: "world", DriverID: storage.DefaultDriverID, Name: "World", DataDir: instanceDir, State: storage.InstanceStateRunning,
	}); err != nil {
		t.Fatal(err)
	}
	if err := SetActiveSave(instanceDir, "Farm_1"); err != nil {
		t.Fatal(err)
	}
	saveDir := filepath.Join(instanceDir, ".local-container", "saves", "Saves", "Farm_1")
	if err := os.MkdirAll(saveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	withTarget := `<SaveGame><player><UniqueMultiplayerID>1</UniqueMultiplayerID></player><farmhands><Farmer><UniqueMultiplayerID>42</UniqueMultiplayerID></Farmer></farmhands></SaveGame>`
	if err := os.WriteFile(filepath.Join(saveDir, "Farm_1"), []byte(withTarget), 0o600); err != nil {
		t.Fatal(err)
	}
	manager := jobs.NewManager(store, nil)
	driver := New(&fakeConsoleDocker{}, nil, manager, store)
	cancelIntent, err := store.CreateFarmhandDeleteIntent(ctx, storage.CreateFarmhandDeleteIntentParams{
		InstanceID: "world", OperationID: "11111111111111111111111111111111", Mode: storage.FarmhandDeleteModeMaintenanceNow,
		Status: storage.FarmhandDeleteIntentLaunching, PlayerID: "42", ExpectedName: "Alice",
		ExpectedSaveID: "Farm_1", ExpiresAt: "2030-01-02T00:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	cancelJob, err := store.CreateJob(ctx, storage.CreateJobParams{Type: FarmhandDeleteJobType, TargetType: "instance", TargetID: "world"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.BindFarmhandDeleteIntentJob(ctx, "world", cancelIntent.OperationID, cancelJob.ID); err != nil {
		t.Fatal(err)
	}
	if err := driver.CancelFarmhandDeleteIntent(ctx, registry.Instance{ID: "world"}, cancelIntent.OperationID); err != nil {
		t.Fatal(err)
	}
	canceledIntent, err := store.GetFarmhandDeleteIntent(ctx, "world")
	if err != nil || canceledIntent.Status != storage.FarmhandDeleteIntentCanceled {
		t.Fatalf("canceled countdown intent = %+v, err=%v", canceledIntent, err)
	}
	canceledJob, err := store.GetJob(ctx, cancelJob.ID)
	if err != nil || canceledJob.Status != storage.JobStatusCanceled {
		t.Fatalf("canceled countdown job = %+v, err=%v", canceledJob, err)
	}

	createInterrupted := func(operationID string) storage.FarmhandDeleteIntent {
		t.Helper()
		intent, createErr := store.CreateFarmhandDeleteIntent(ctx, storage.CreateFarmhandDeleteIntentParams{
			InstanceID: "world", OperationID: operationID, Mode: storage.FarmhandDeleteModeMaintenanceNow,
			Status: storage.FarmhandDeleteIntentLaunching, PlayerID: "42", ExpectedName: "Alice",
			ExpectedSaveID: "Farm_1", ExpiresAt: "2030-01-02T00:00:00Z",
		})
		if createErr != nil {
			t.Fatal(createErr)
		}
		job, createErr := store.CreateJob(ctx, storage.CreateJobParams{Type: FarmhandDeleteJobType, TargetType: "instance", TargetID: "world"})
		if createErr != nil {
			t.Fatal(createErr)
		}
		if createErr = store.BindFarmhandDeleteIntentJob(ctx, "world", operationID, job.ID); createErr != nil {
			t.Fatal(createErr)
		}
		if _, createErr = store.FailJob(ctx, job.ID, "panel restarted"); createErr != nil {
			t.Fatal(createErr)
		}
		intent, createErr = store.GetFarmhandDeleteIntent(ctx, "world")
		if createErr != nil {
			t.Fatal(createErr)
		}
		return intent
	}

	interrupted := createInterrupted("0123456789abcdef0123456789abcdef")
	if err := driver.reconcileInterruptedFarmhandDeleteIntent(ctx, store, interrupted); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetFarmhandDeleteIntent(ctx, "world")
	if err != nil || got.Status != storage.FarmhandDeleteIntentCanceled {
		t.Fatalf("pre-delete interruption = %+v, err=%v", got, err)
	}

	destructive := createInterrupted("abcdef0123456789abcdef0123456789")
	marker := farmhandDeleteMaintenanceMarker{
		SchemaVersion: 1, OperationID: destructive.OperationID, ExpectedSaveID: "Farm_1", TargetPlayerID: "42", Phase: "destructive",
	}
	markerRaw, err := json.Marshal(marker)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(controlDir(instanceDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(controlDir(instanceDir), "farmhand-delete-maintenance.json"), markerRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := driver.reconcileInterruptedFarmhandDeleteIntent(ctx, store, destructive); err != nil {
		t.Fatal(err)
	}
	got, err = store.GetFarmhandDeleteIntent(ctx, "world")
	if err != nil || got.Status != storage.FarmhandDeleteIntentRecoveryRequired {
		t.Fatalf("post-delete interruption = %+v, err=%v", got, err)
	}
}
