package stardew_junimo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	appconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func TestNewGameTransactionStandardAndMeadowlandsValidation(t *testing.T) {
	for _, tc := range []struct {
		farmType  string
		whichFarm string
	}{
		{farmType: "standard", whichFarm: "0"},
		{farmType: "meadowlands", whichFarm: "MeadowlandsFarm"},
	} {
		t.Run(tc.farmType, func(t *testing.T) {
			dataDir := t.TempDir()
			cfg := newGameTestConfig(tc.farmType)
			tx, err := beginNewGameTransaction(dataDir, cfg)
			if err != nil {
				t.Fatal(err)
			}
			if err := tx.prepareConfigAndMarker(); err != nil {
				t.Fatal(err)
			}
			writeNewGameTestSave(t, dataDir, "Farm_123", tc.whichFarm)
			state := &newGameFileStability{}
			if stable, err := validateStableNewGameSave(dataDir, "Farm_123", tc.farmType, state); err != nil || stable {
				t.Fatalf("first stability check = %v, %v", stable, err)
			}
			if stable, err := validateStableNewGameSave(dataDir, "Farm_123", tc.farmType, state); err != nil || !stable {
				t.Fatalf("second stability check = %v, %v", stable, err)
			}
			recordCompleteNewGameDurabilityEvidence(t, tx, "Farm_123")
			if err := tx.complete("Farm_123"); err != nil {
				t.Fatal(err)
			}
			record, err := LoadNewGameTransaction(dataDir, tx.record.TransactionID)
			if err != nil {
				t.Fatal(err)
			}
			if record.Stage != newGameStateSuccess || record.CreatedSave != "Farm_123" {
				t.Fatalf("record = %#v", record)
			}
		})
	}
}

func TestNewGameTransactionWritesPrivateStateAndStructuredMarker(t *testing.T) {
	dataDir := t.TempDir()
	tx, err := beginNewGameTransaction(dataDir, newGameTestConfig("standard"))
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.prepareConfigAndMarker(); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		if info, err := os.Stat(tx.dir); err != nil || info.Mode().Perm() != 0o700 {
			t.Fatalf("transaction dir mode = %v, err=%v", info.Mode().Perm(), err)
		}
	}
	statePath := filepath.Join(tx.dir, "transaction.json")
	if runtime.GOOS != "windows" {
		if info, err := os.Stat(statePath); err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("transaction file mode = %v, err=%v", info.Mode().Perm(), err)
		}
	}
	var marker newGamePendingMarker
	readJSONFileForTest(t, newGamePendingPath(dataDir), &marker)
	if marker.SchemaVersion != 1 || marker.TransactionID != tx.record.TransactionID || marker.RequestedFarmType != "standard" || marker.State != "pending" {
		t.Fatalf("marker = %#v", marker)
	}
}

func TestNewGameTransactionCanBeReadAfterRestartAtEveryIrreversibleStage(t *testing.T) {
	dataDir := t.TempDir()
	tx := prepareNewGameTestTransaction(t, dataDir, "standard")
	for _, stage := range []NewGameTransactionState{
		newGameStateConfigured,
		newGameStateMarkerWritten,
		newGameStateModsPrepared,
		newGameStateComposeUp,
		newGameStateCatalogAsked,
		newGameStateCatalogReady,
		newGameStateObserving,
		newGameStateProfilePending,
	} {
		if err := tx.mark(stage); err != nil {
			t.Fatal(err)
		}
		loaded, err := LoadNewGameTransaction(dataDir, tx.record.TransactionID)
		if err != nil || loaded.Stage != stage || loaded.Config.FarmType != "standard" {
			t.Fatalf("load stage %s: %#v, %v", stage, loaded, err)
		}
	}
	if err := tx.markCommandCalled(); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadNewGameTransaction(dataDir, tx.record.TransactionID)
	if err != nil || !loaded.CommandCalled || loaded.CommandCalledAt == nil {
		t.Fatalf("command-called recovery record = %#v, %v", loaded, err)
	}
	if err := tx.mark(newGameStateObserving); err != nil {
		t.Fatal(err)
	}
	loaded, err = LoadNewGameTransaction(dataDir, tx.record.TransactionID)
	if err != nil || loaded.Stage != newGameStateObserving || !loaded.CommandCalled {
		t.Fatalf("post-command recovery record = %#v, %v", loaded, err)
	}
}

func TestNewGameConfigAndMarkerWriteFailuresRollback(t *testing.T) {
	for _, failBase := range []string{"server-init.json", "new-game-pending"} {
		t.Run(failBase, func(t *testing.T) {
			dataDir := t.TempDir()
			oldSettings := []byte(`{"Server":{"MaxPlayers":3}}`)
			if err := os.MkdirAll(filepath.Dir(serverSettingsPath(dataDir)), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(serverSettingsPath(dataDir), oldSettings, 0o644); err != nil {
				t.Fatal(err)
			}
			tx, err := beginNewGameTransaction(dataDir, newGameTestConfig("standard"))
			if err != nil {
				t.Fatal(err)
			}
			tx.writeJSON = func(path string, data []byte, mode os.FileMode) error {
				if filepath.Base(path) == failBase {
					return errors.New("injected write failure")
				}
				return atomicWriteValidatedJSON(path, data, mode)
			}
			err = tx.prepareConfigAndMarker()
			if err == nil {
				t.Fatal("expected failure")
			}
			code := "new_game_config_write_failed"
			if failBase == "new-game-pending" {
				code = "new_game_marker_write_failed"
			}
			if rollbackErr := tx.rollback(err, code, newGameStateFailed); rollbackErr != nil {
				t.Fatal(rollbackErr)
			}
			if got, _ := os.ReadFile(serverSettingsPath(dataDir)); string(got) != string(oldSettings) {
				t.Fatalf("settings not restored: %s", got)
			}
		})
	}
}

func TestNewGameComposeStartFailureRollsBackPreparedConfiguration(t *testing.T) {
	dataDir := t.TempDir()
	oldSettings := []byte(`{"Server":{"MaxPlayers":4}}`)
	writeNewGameSnapshotFixture(t, serverSettingsPath(dataDir), oldSettings)
	tx := prepareNewGameTestTransaction(t, dataDir, "standard")
	cause := &NewGameTransactionError{Code: "new_game_compose_start_failed", Message: "compose start failed", Cause: errors.New("injected compose failure")}
	if err := tx.rollback(cause, cause.Code, newGameStateFailed); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(serverSettingsPath(dataDir)); string(got) != string(oldSettings) {
		t.Fatalf("settings not restored after compose failure: %s", got)
	}
	if tx.record.ErrorCode != "new_game_compose_start_failed" || !tx.record.RollbackCompleted {
		t.Fatalf("record = %#v", tx.record)
	}
}

func TestNewGameHTTPWriterPostsOnceAndUsesOneDurableSaveCommandID(t *testing.T) {
	dataDir := t.TempDir()
	tx := prepareHTTPNewGameTestTransaction(t, dataDir, "standard", "Existing_100")
	if tx.record.CreationWriter != newGameCreationWriterHTTP {
		t.Fatalf("creation writer = %q, want http", tx.record.CreationWriter)
	}
	simulator := startNewGameControlSimulator(t, tx, "Fresh_101", "0", nil)
	var posts atomic.Int32
	fake := &fakeConsoleDocker{execFunc: func(ctx context.Context, _, _, _ string, args ...string) (paneldocker.CommandResult, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "/status") {
			return paneldocker.CommandResult{ExitCode: 0}, nil
		}
		if strings.Contains(joined, "/newgame") {
			posts.Add(1)
			if err := stageNewGameCandidate(tx, "Fresh_101"); err != nil {
				return paneldocker.CommandResult{}, err
			}
			return paneldocker.CommandResult{ExitCode: 0, Stdout: "ok"}, nil
		}
		return paneldocker.CommandResult{ExitCode: 0}, nil
	}}
	runner := newGameTestRunner(dataDir, fake)
	if err := runNewGameCommandJob(t, runner, tx); err != nil {
		t.Fatal(err)
	}
	if err := runNewGameCommandJob(t, runner, tx); err != nil {
		t.Fatal(err)
	}
	if posts.Load() != 1 {
		t.Fatalf("POST count = %d", posts.Load())
	}
	result := simulator.wait(t)
	assertSingleNewGameSaveCommand(t, dataDir, tx.record.TransactionID, "Fresh_101", result.CommandID)
}

func TestNewGameStartupWriterNeverPostsAndStillCompletesDurabilityGates(t *testing.T) {
	dataDir := t.TempDir()
	tx := prepareNewGameTestTransaction(t, dataDir, "standard")
	if tx.record.CreationWriter != newGameCreationWriterStartup {
		t.Fatalf("creation writer = %q, want startup", tx.record.CreationWriter)
	}
	simulator := startNewGameControlSimulator(t, tx, "Startup_102", "0", nil)
	if err := stageNewGameCandidate(tx, "Startup_102"); err != nil {
		t.Fatal(err)
	}
	var posts atomic.Int32
	fake := &fakeConsoleDocker{execFunc: func(_ context.Context, _, _, _ string, args ...string) (paneldocker.CommandResult, error) {
		if strings.Contains(strings.Join(args, " "), "/newgame") {
			posts.Add(1)
		}
		return paneldocker.CommandResult{ExitCode: 0}, nil
	}}
	if err := runNewGameCommandJob(t, newGameTestRunner(dataDir, fake), tx); err != nil {
		t.Fatal(err)
	}
	if posts.Load() != 0 || tx.record.CommandCalled {
		t.Fatalf("POST count=%d commandCalled=%v", posts.Load(), tx.record.CommandCalled)
	}
	if tx.record.Stage != newGameStateSuccess || tx.record.CreatedSave != "Startup_102" {
		t.Fatalf("record = %#v", tx.record)
	}
	result := simulator.wait(t)
	assertSingleNewGameSaveCommand(t, dataDir, tx.record.TransactionID, "Startup_102", result.CommandID)
}

func TestNewGameCommandTimeoutCanStillSucceed(t *testing.T) {
	dataDir := t.TempDir()
	tx := prepareHTTPNewGameTestTransaction(t, dataDir, "standard", "Existing_200")
	simulator := startNewGameControlSimulator(t, tx, "Late_202", "0", nil)
	var posts atomic.Int32
	fake := &fakeConsoleDocker{execFunc: func(ctx context.Context, _, _, _ string, args ...string) (paneldocker.CommandResult, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "/status") {
			return paneldocker.CommandResult{ExitCode: 0}, nil
		}
		if strings.Contains(joined, "/newgame") {
			posts.Add(1)
			<-ctx.Done()
			if err := stageNewGameCandidate(tx, "Late_202"); err != nil {
				return paneldocker.CommandResult{}, err
			}
			return paneldocker.CommandResult{}, ctx.Err()
		}
		return paneldocker.CommandResult{}, nil
	}}
	runner := newGameTestRunner(dataDir, fake)
	runner.newGameCommandTimeout = 10 * time.Millisecond
	if err := runNewGameCommandJob(t, runner, tx); err != nil {
		t.Fatal(err)
	}
	if tx.record.Stage != newGameStateSuccess {
		t.Fatalf("stage = %s", tx.record.Stage)
	}
	if posts.Load() != 1 {
		t.Fatalf("POST count = %d, want 1", posts.Load())
	}
	result := simulator.wait(t)
	assertSingleNewGameSaveCommand(t, dataDir, tx.record.TransactionID, "Late_202", result.CommandID)
}

func TestNewGameCommandTimeoutWithoutSaveIsUnknown(t *testing.T) {
	dataDir := t.TempDir()
	tx := prepareHTTPNewGameTestTransaction(t, dataDir, "standard", "Existing_300")
	var posts atomic.Int32
	fake := &fakeConsoleDocker{execFunc: func(ctx context.Context, _, _, _ string, args ...string) (paneldocker.CommandResult, error) {
		if strings.Contains(strings.Join(args, " "), "/status") {
			return paneldocker.CommandResult{ExitCode: 0}, nil
		}
		posts.Add(1)
		<-ctx.Done()
		return paneldocker.CommandResult{}, ctx.Err()
	}}
	runner := newGameTestRunner(dataDir, fake)
	runner.newGameCommandTimeout = 10 * time.Millisecond
	err := runNewGameCommandJob(t, runner, tx)
	assertNewGameErrorCode(t, err, "new_game_outcome_unknown")
	if tx.record.Stage != newGameStateUnknown {
		t.Fatalf("stage = %s", tx.record.Stage)
	}
	if posts.Load() != 1 || !tx.record.CommandCalled {
		t.Fatalf("POST count=%d commandCalled=%v", posts.Load(), tx.record.CommandCalled)
	}
}

func TestNewGameCommandExplicitFailureAfterIntentIsOutcomeUnknown(t *testing.T) {
	dataDir := t.TempDir()
	tx := prepareHTTPNewGameTestTransaction(t, dataDir, "standard", "Existing_400")
	var posts atomic.Int32
	fake := &fakeConsoleDocker{execFunc: func(_ context.Context, _, _, _ string, args ...string) (paneldocker.CommandResult, error) {
		if strings.Contains(strings.Join(args, " "), "/status") {
			return paneldocker.CommandResult{ExitCode: 0}, nil
		}
		posts.Add(1)
		return paneldocker.CommandResult{ExitCode: 22}, errors.New("explicit HTTP failure")
	}}
	err := runNewGameCommandJob(t, newGameTestRunner(dataDir, fake), tx)
	assertNewGameErrorCode(t, err, "new_game_outcome_unknown")
	if posts.Load() != 1 || tx.record.Stage != newGameStateUnknown || !tx.record.CommandCalled {
		t.Fatalf("POST count=%d record=%#v", posts.Load(), tx.record)
	}
}

func TestNewGameDetectionRejectsAmbiguousBrokenAndMismatchedSaves(t *testing.T) {
	tests := []struct {
		name      string
		make      func(*testing.T, *newGameTransaction)
		code      string
		simulate  bool
		whichFarm string
		disk      func(string, string, registry.NewGameConfig, string) error
	}{
		{name: "multiple", make: func(t *testing.T, tx *newGameTransaction) {
			if err := stageNewGameCandidate(tx, "One_1"); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Join(savesDir(tx.dataDir), "Saves", "Two_2"), 0o755); err != nil {
				t.Fatal(err)
			}
		}, code: "new_game_ambiguous"},
		{name: "broken XML remains pending", simulate: true, whichFarm: "0", code: "new_game_disk_durability_timeout", disk: writeBrokenNewGameDiskFixture},
		{name: "farm mismatch", simulate: true, whichFarm: "7", code: "new_game_disk_farm_type_mismatch"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dataDir := t.TempDir()
			tx := prepareNewGameTestTransaction(t, dataDir, "standard")
			if tx.record.CreationWriter != newGameCreationWriterStartup {
				t.Fatalf("creation writer = %q, want startup", tx.record.CreationWriter)
			}
			var simulator *newGameControlSimulator
			if tc.simulate {
				simulator = startNewGameControlSimulator(t, tx, "Candidate_3", tc.whichFarm, tc.disk)
				if err := stageNewGameCandidate(tx, "Candidate_3"); err != nil {
					t.Fatal(err)
				}
			} else {
				tc.make(t, tx)
			}
			var posts atomic.Int32
			fake := &fakeConsoleDocker{execFunc: func(_ context.Context, _, _, _ string, args ...string) (paneldocker.CommandResult, error) {
				if strings.Contains(strings.Join(args, " "), "/newgame") {
					posts.Add(1)
				}
				return paneldocker.CommandResult{ExitCode: 0}, nil
			}}
			err := runNewGameCommandJob(t, newGameTestRunner(dataDir, fake), tx)
			assertNewGameErrorCode(t, err, tc.code)
			if posts.Load() != 0 {
				t.Fatalf("startup writer POST count = %d, want 0", posts.Load())
			}
			if simulator != nil {
				result := simulator.wait(t)
				assertSingleNewGameSaveCommand(t, dataDir, tx.record.TransactionID, "Candidate_3", result.CommandID)
			}
		})
	}
}

func TestNewGameHTTPWriterLoaderProgressWithoutDirectoryIsOutcomeUnknown(t *testing.T) {
	dataDir := t.TempDir()
	tx := prepareHTTPNewGameTestTransaction(t, dataDir, "standard", "Existing_500")
	var posts atomic.Int32
	fake := &fakeConsoleDocker{execFunc: func(_ context.Context, _, _, _ string, args ...string) (paneldocker.CommandResult, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "/status") {
			return paneldocker.CommandResult{ExitCode: 0}, nil
		}
		if strings.Contains(joined, "/newgame") {
			posts.Add(1)
			if err := writeGameloaderPointer(dataDir, "Missing_99"); err != nil {
				return paneldocker.CommandResult{}, err
			}
		}
		return paneldocker.CommandResult{ExitCode: 0}, nil
	}}
	err := runNewGameCommandJob(t, newGameTestRunner(dataDir, fake), tx)
	assertNewGameErrorCode(t, err, "new_game_outcome_unknown")
	if posts.Load() != 1 || !tx.record.CommandCalled || !tx.record.ProgressObserved || tx.record.Stage != newGameStateUnknown {
		t.Fatalf("POST count=%d record=%#v", posts.Load(), tx.record)
	}
}

func TestNewGameRollbackRestoresFilesModsAndQuarantinesGeneratedSave(t *testing.T) {
	dataDir := t.TempDir()
	oldSettings := []byte(`{"Server":{"MaxPlayers":2}}`)
	oldInit := []byte(`{"mode":"old"}`)
	oldPointer := []byte(`{"SaveNameToLoad":"Old_1"}`)
	oldMarker := []byte("legacy-pending\n")
	writeNewGameSnapshotFixture(t, serverSettingsPath(dataDir), oldSettings)
	writeNewGameSnapshotFixture(t, serverInitPath(dataDir), oldInit)
	writeNewGameSnapshotFixture(t, gameloaderPath(dataDir), oldPointer)
	writeNewGameSnapshotFixture(t, newGamePendingPath(dataDir), oldMarker)
	writeNewGameTestMod(t, dataDir, true, "Third Party", "Example.Mod")

	tx := prepareNewGameTestTransaction(t, dataDir, "standard")
	if err := ApplyNewSaveDefaultModState(dataDir); err != nil {
		t.Fatal(err)
	}
	writeNewGameTestRawSave(t, dataDir, "Invalid_55", `<SaveGame>`)
	cause := errors.New("validation failed")
	if err := tx.rollback(cause, "new_game_xml_invalid", newGameStateFailed); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string][]byte{
		serverSettingsPath(dataDir): oldSettings, serverInitPath(dataDir): oldInit,
		gameloaderPath(dataDir): oldPointer, newGamePendingPath(dataDir): oldMarker,
	} {
		if got, _ := os.ReadFile(path); string(got) != string(want) {
			t.Fatalf("%s not restored: %q", filepath.Base(path), got)
		}
	}
	if _, err := os.Stat(filepath.Join(dataDir, ".local-container", "mods", "Third Party")); err != nil {
		t.Fatalf("mod not restored enabled: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, ".local-container", "saves-quarantine", "new-game", tx.record.TransactionID, "Invalid_55")); err != nil {
		t.Fatalf("invalid save not quarantined: %v", err)
	}
}

func TestNewGameRollbackFailureHasIndependentState(t *testing.T) {
	dataDir := t.TempDir()
	tx := prepareNewGameTestTransaction(t, dataDir, "standard")
	tx.restoreFile = func(string, newGameFileSnapshot) error { return errors.New("injected restore failure") }
	err := tx.rollback(errors.New("original failure"), "new_game_command_failed", newGameStateFailed)
	if err == nil || tx.record.Stage != newGameStateRollbackFail || !strings.Contains(tx.record.ErrorMessage, "original failure") {
		t.Fatalf("rollback err=%v record=%#v", err, tx.record)
	}
}

func TestNewGameRollbackJournalPersistFailureLeavesTargetsUntouched(t *testing.T) {
	dataDir := t.TempDir()
	oldSettings := []byte(`{"Server":{"MaxPlayers":2}}`)
	newSettings := []byte(`{"Server":{"MaxPlayers":8}}`)
	writeNewGameSnapshotFixture(t, serverSettingsPath(dataDir), oldSettings)
	tx := prepareNewGameTestTransaction(t, dataDir, "standard")
	writeNewGameSnapshotFixture(t, serverSettingsPath(dataDir), newSettings)
	writeNewGameTestRawSave(t, dataDir, "Uncommitted_88", `<SaveGame>`)
	tx.writeState = func(string, []byte, os.FileMode) error {
		return errors.New("injected transaction persist failure")
	}

	err := tx.rollback(errors.New("original failure"), "new_game_failed", newGameStateFailed)
	if err == nil || !strings.Contains(err.Error(), "injected transaction persist failure") {
		t.Fatalf("rollback error = %v", err)
	}
	if got, readErr := os.ReadFile(serverSettingsPath(dataDir)); readErr != nil || string(got) != string(newSettings) {
		t.Fatalf("settings changed after journal persist failure: %q, %v", got, readErr)
	}
	if _, statErr := os.Stat(filepath.Join(savesDir(dataDir), "Saves", "Uncommitted_88")); statErr != nil {
		t.Fatalf("new save moved after journal persist failure: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(dataDir, ".local-container", "saves-quarantine", "new-game", tx.record.TransactionID, "Uncommitted_88")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("quarantine changed after journal persist failure: %v", statErr)
	}
	disk, loadErr := LoadNewGameTransaction(dataDir, tx.record.TransactionID)
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if disk.Stage == newGameStateRollingBack || disk.RollbackStartedAt != nil {
		t.Fatalf("failed write-ahead journal became durable: %#v", disk)
	}
}

func TestNewGameRollbackResumesAfterQuarantineBeforeCheckpoint(t *testing.T) {
	dataDir := t.TempDir()
	oldSettings := []byte(`{"Server":{"MaxPlayers":2}}`)
	newSettings := []byte(`{"Server":{"MaxPlayers":8}}`)
	writeNewGameSnapshotFixture(t, serverSettingsPath(dataDir), oldSettings)
	tx := prepareNewGameTestTransaction(t, dataDir, "standard")
	writeNewGameSnapshotFixture(t, serverSettingsPath(dataDir), newSettings)
	writeNewGameTestRawSave(t, dataDir, "Interrupted_41", `<SaveGame>`)
	if err := tx.beginRollback(errors.New("original failure"), "new_game_failed", newGameStateFailed); err != nil {
		t.Fatal(err)
	}
	if err := tx.prepareRollbackStep(newGameRollbackStepQuarantine); err != nil {
		t.Fatal(err)
	}
	if err := tx.quarantineRollbackSaveDirs(); err != nil {
		t.Fatal(err)
	}
	// Simulate a process kill after rename and before the completed-step
	// checkpoint. Recovery must detect the destination and replay idempotently.
	disk, err := LoadNewGameTransaction(dataDir, tx.record.TransactionID)
	if err != nil {
		t.Fatal(err)
	}
	if disk.Stage != newGameStateRollingBack || disk.RollbackCurrentStep != newGameRollbackStepQuarantine || len(disk.RollbackCompletedSteps) != 0 {
		t.Fatalf("pre-kill journal = %#v", disk)
	}
	recovered := rehydrateNewGameTransaction(dataDir, disk, "")
	if err := recovered.continueRollback(); err != nil {
		t.Fatal(err)
	}
	if recovered.record.Stage != newGameStateRolledBack || !recovered.record.RollbackCompleted {
		t.Fatalf("recovered transaction = %#v", recovered.record)
	}
	if got, readErr := os.ReadFile(serverSettingsPath(dataDir)); readErr != nil || string(got) != string(oldSettings) {
		t.Fatalf("settings not restored: %q, %v", got, readErr)
	}
	if _, statErr := os.Stat(filepath.Join(dataDir, ".local-container", "saves-quarantine", "new-game", tx.record.TransactionID, "Interrupted_41")); statErr != nil {
		t.Fatalf("quarantined save missing after recovery: %v", statErr)
	}
}

func TestNewGameRollbackResumesInterruptedFileRestoreWithoutRepeatingCompletedStep(t *testing.T) {
	dataDir := t.TempDir()
	oldSettings := []byte(`{"Server":{"MaxPlayers":2}}`)
	oldInit := []byte(`{"mode":"old"}`)
	writeNewGameSnapshotFixture(t, serverSettingsPath(dataDir), oldSettings)
	writeNewGameSnapshotFixture(t, serverInitPath(dataDir), oldInit)
	tx := prepareNewGameTestTransaction(t, dataDir, "standard")
	writeNewGameSnapshotFixture(t, serverSettingsPath(dataDir), []byte(`{"Server":{"MaxPlayers":8}}`))
	writeNewGameSnapshotFixture(t, serverInitPath(dataDir), []byte(`{"mode":"new"}`))
	if err := tx.beginRollback(errors.New("original failure"), "new_game_failed", newGameStateFailed); err != nil {
		t.Fatal(err)
	}
	if err := tx.runRollbackStep(newGameRollbackStepQuarantine, tx.quarantineRollbackSaveDirs); err != nil {
		t.Fatal(err)
	}
	steps := tx.rollbackFileSteps()
	if err := tx.runRollbackStep(steps[0].id, func() error { return tx.restoreFile(steps[0].path, steps[0].snapshot) }); err != nil {
		t.Fatal(err)
	}
	if steps[1].id != newGameRollbackStepServerInit {
		t.Fatalf("second restore step = %s", steps[1].id)
	}
	if err := tx.prepareRollbackStep(steps[1].id); err != nil {
		t.Fatal(err)
	}
	if err := tx.restoreFile(steps[1].path, steps[1].snapshot); err != nil {
		t.Fatal(err)
	}
	// Simulate a kill after server-init restore but before its completion
	// checkpoint. The already checkpointed server-settings step must be skipped.
	disk, err := LoadNewGameTransaction(dataDir, tx.record.TransactionID)
	if err != nil {
		t.Fatal(err)
	}
	recovered := rehydrateNewGameTransaction(dataDir, disk, "")
	var restoredSettings, restoredInit int
	recovered.restoreFile = func(path string, snapshot newGameFileSnapshot) error {
		switch path {
		case serverSettingsPath(dataDir):
			restoredSettings++
		case serverInitPath(dataDir):
			restoredInit++
		}
		return restoreNewGameFile(path, snapshot)
	}
	if err := recovered.continueRollback(); err != nil {
		t.Fatal(err)
	}
	if restoredSettings != 0 || restoredInit != 1 {
		t.Fatalf("restore calls settings=%d init=%d, want 0/1", restoredSettings, restoredInit)
	}
	if got, readErr := os.ReadFile(serverInitPath(dataDir)); readErr != nil || string(got) != string(oldInit) {
		t.Fatalf("server-init not restored: %q, %v", got, readErr)
	}
}

func TestNewGameRollbackOwnerReleasedOnlyAfterTerminalCheckpoint(t *testing.T) {
	dataDir := t.TempDir()
	cfg := newGameTestConfig("standard")
	tx, _, err := beginOrResumeNewGameTransactionWithJobStatus(
		dataDir, cfg, "request-owner-release-after-rollback", "rollback-job",
		func(string) (bool, error) { return false, nil },
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.beginRollback(errors.New("original failure"), "new_game_failed", newGameStateFailed); err != nil {
		t.Fatal(err)
	}
	if err := tx.releaseOwner(); err == nil {
		t.Fatal("owner released while rollback journal was nonterminal")
	}
	if _, err := LoadNewGameOwner(dataDir); err != nil {
		t.Fatalf("owner missing during rolling_back: %v", err)
	}
	if err := tx.failRollback(errors.New("injected restore interruption")); err == nil {
		t.Fatal("expected rollback failure")
	}
	if err := tx.releaseOwner(); err == nil {
		t.Fatal("owner released from rollback_failed")
	}
	if _, err := LoadNewGameOwner(dataDir); err != nil {
		t.Fatalf("owner missing during rollback_failed: %v", err)
	}
	if err := tx.continueRollback(); err != nil {
		t.Fatal(err)
	}
	if err := tx.releaseOwner(); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadNewGameOwner(dataDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("owner after terminal rollback = %v", err)
	}
}

func TestNewGameNumericSuffixRepairRequiresUniqueNumericSuffix(t *testing.T) {
	if got := uniqueNumericSuffixCandidate("Wrong_123", []string{"Right_123"}); got != "Right_123" {
		t.Fatalf("got %q", got)
	}
	if got := uniqueNumericSuffixCandidate("Wrong_abc", []string{"Right_abc"}); got != "" {
		t.Fatalf("non-numeric suffix repaired to %q", got)
	}
	if got := uniqueNumericSuffixCandidate("Wrong_123", []string{"One_123", "Two_123"}); got != "" {
		t.Fatalf("ambiguous suffix repaired to %q", got)
	}
}

func newGameTestConfig(farmType string) registry.NewGameConfig {
	cfg, err := NormalizeNewGameConfig(registry.NewGameConfig{FarmName: "Test Farm", FarmType: farmType})
	if err != nil {
		panic(err)
	}
	return cfg
}

func recordCompleteNewGameDurabilityEvidence(t *testing.T, tx *newGameTransaction, saveID string) {
	t.Helper()
	loadedAt := time.Now().UTC()
	verifiedAt := loadedAt.Add(time.Millisecond)
	publishedAt := verifiedAt.Add(time.Millisecond)
	savedAt := publishedAt.Add(time.Millisecond)
	diskAt := savedAt.Add(time.Millisecond)
	tx.record.CandidateSave = saveID
	tx.record.SaveLoadedAt = &loadedAt
	tx.record.CustomizationVerifiedAt = &verifiedAt
	tx.record.DurableSaveCommandID = strings.Repeat("a", 32)
	tx.record.DurableSaveCommandPublishedAt = &publishedAt
	tx.record.DurableGameLoopSavedAt = &savedAt
	tx.record.MainSaveSHA256 = strings.Repeat("b", 64)
	tx.record.SaveGameInfoSHA256 = strings.Repeat("c", 64)
	tx.record.DiskVerifiedAt = &diskAt
	if err := tx.persist(); err != nil {
		t.Fatal(err)
	}
}

func prepareHTTPNewGameTestTransaction(t *testing.T, dataDir, farmType, activeSave string) *newGameTransaction {
	t.Helper()
	cfg := newGameTestConfig(farmType)
	whichFarm := "0"
	if normalized, err := NormalizeNewGameFarmType(farmType); err == nil {
		whichFarm = normalized.ID
	}
	if err := writeNewGameDurableDiskFixture(dataDir, activeSave, cfg, whichFarm); err != nil {
		t.Fatal(err)
	}
	if err := writeGameloaderPointer(dataDir, activeSave); err != nil {
		t.Fatal(err)
	}
	tx := prepareNewGameTestTransaction(t, dataDir, farmType)
	updatedAt := time.Now().UTC()
	status := NewGameRuntimeStatusSnapshot{
		Present: true, State: "save-loaded", SaveID: activeSave,
		TransactionID: tx.record.TransactionID, CreationObserved: false, UpdatedAt: &updatedAt,
	}
	if err := writeJSONAtomic(filepath.Join(controlDir(dataDir), "status.json"), status); err != nil {
		t.Fatal(err)
	}
	return tx
}

type newGameControlSimulationResult struct {
	CommandID string
}

type newGameControlSimulator struct {
	cancel context.CancelFunc
	done   chan struct{}
	mu     sync.Mutex
	result newGameControlSimulationResult
	err    error
}

type newGameDiskFixtureWriter func(string, string, registry.NewGameConfig, string) error

var newGameCandidateSignalsForTest sync.Map

func startNewGameControlSimulator(
	t *testing.T,
	tx *newGameTransaction,
	saveID string,
	whichFarm string,
	diskWriter newGameDiskFixtureWriter,
) *newGameControlSimulator {
	t.Helper()
	if diskWriter == nil {
		diskWriter = writeNewGameDurableDiskFixture
	}
	initialMarker, err := os.Stat(newGamePendingPath(tx.dataDir))
	if err != nil {
		t.Fatal(err)
	}
	candidateSignal := make(chan string, 1)
	newGameCandidateSignalsForTest.Store(tx.dataDir, candidateSignal)
	ctx, cancel := context.WithCancel(context.Background())
	simulator := &newGameControlSimulator{cancel: cancel, done: make(chan struct{})}
	go func() {
		result, err := runNewGameControlSimulator(ctx, tx, saveID, whichFarm, initialMarker.Size(), candidateSignal, diskWriter)
		simulator.mu.Lock()
		simulator.result = result
		simulator.err = err
		simulator.mu.Unlock()
		close(simulator.done)
	}()
	t.Cleanup(func() {
		newGameCandidateSignalsForTest.Delete(tx.dataDir)
		simulator.cancel()
		select {
		case <-simulator.done:
			simulator.mu.Lock()
			err := simulator.err
			simulator.mu.Unlock()
			if err != nil && !errors.Is(err, context.Canceled) {
				t.Errorf("Control simulator: %v", err)
			}
		case <-time.After(time.Second):
			t.Errorf("Control simulator did not stop")
		}
	})
	return simulator
}

func (s *newGameControlSimulator) wait(t *testing.T) newGameControlSimulationResult {
	t.Helper()
	select {
	case <-s.done:
	case <-time.After(2 * time.Second):
		t.Fatal("Control simulator did not finish")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		t.Fatal(s.err)
	}
	return s.result
}

func runNewGameControlSimulator(
	ctx context.Context,
	tx *newGameTransaction,
	saveID string,
	whichFarm string,
	initialMarkerSize int64,
	candidateSignal <-chan string,
	diskWriter newGameDiskFixtureWriter,
) (newGameControlSimulationResult, error) {
	if err := waitForNewGameTargetMarkerForTest(ctx, tx, saveID, initialMarkerSize, candidateSignal); err != nil {
		return newGameControlSimulationResult{}, err
	}
	statusAt := time.Now().UTC()
	status := exactNewGameControlStatus(tx.record.TransactionID, saveID, tx.record.Config, statusAt)
	if err := writeNewGameJSONAtomicForTest(ctx, filepath.Join(controlDir(tx.dataDir), "status.json"), status); err != nil {
		return newGameControlSimulationResult{}, fmt.Errorf("write exact Control status: %w", err)
	}
	players := map[string]any{
		"updatedAt": status.CustomizationVerifiedAt.Add(time.Millisecond),
		"saveId":    saveID,
		"players": []map[string]any{{
			"name": tx.record.Config.FarmerName, "isHost": true,
		}},
	}
	if err := writeNewGameJSONAtomicForTest(ctx, filepath.Join(controlDir(tx.dataDir), "players.json"), players); err != nil {
		return newGameControlSimulationResult{}, fmt.Errorf("write exact Control players: %w", err)
	}
	command, err := waitForNewGameSaveCommandForTest(ctx, tx.dataDir, tx.record.TransactionID, saveID)
	if err != nil {
		return newGameControlSimulationResult{}, err
	}
	if err := diskWriter(tx.dataDir, saveID, tx.record.Config, whichFarm); err != nil {
		return newGameControlSimulationResult{}, fmt.Errorf("write durable save fixture: %w", err)
	}
	createdAt := time.Now().UTC()
	outcome := CommandOutcome{
		CommandID: command.ID, Status: CommandStatusSucceeded, ErrorCode: "ok",
		CreatedAt: createdAt, UpdatedAt: createdAt.Add(time.Millisecond),
		Details: map[string]string{
			"event": "GameLoop.Saved", "transactionId": tx.record.TransactionID,
			"saveId": saveID, "expectedSaveId": saveID,
		},
	}
	if err := writeNewGameJSONAtomicForTest(ctx, filepath.Join(commandResultsDir(tx.dataDir), command.ID+".json"), outcome); err != nil {
		return newGameControlSimulationResult{}, fmt.Errorf("write exact GameLoop.Saved result: %w", err)
	}
	return newGameControlSimulationResult{CommandID: command.ID}, nil
}

func waitForNewGameTargetMarkerForTest(ctx context.Context, tx *newGameTransaction, saveID string, initialMarkerSize int64, candidateSignal <-chan string) error {
	select {
	case got := <-candidateSignal:
		if got != saveID {
			return fmt.Errorf("candidate signal = %q, want %q", got, saveID)
		}
	case <-ctx.Done():
		return ctx.Err()
	}
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		info, err := os.Stat(newGamePendingPath(tx.dataDir))
		if err == nil && info.Size() != initialMarkerSize {
			raw, err := os.ReadFile(newGamePendingPath(tx.dataDir))
			if err == nil {
				var marker newGamePendingMarker
				if json.Unmarshal(raw, &marker) == nil && marker.TransactionID == tx.record.TransactionID && marker.TargetSaveID == saveID {
					return nil
				}
			} else if !errors.Is(err, os.ErrNotExist) {
				return err
			}
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

type newGameSaveCommandForTest struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Payload map[string]string `json:"payload"`
}

func waitForNewGameSaveCommandForTest(ctx context.Context, dataDir, transactionID, saveID string) (newGameSaveCommandForTest, error) {
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		commands, err := readNewGameSaveCommandsForTest(dataDir)
		if err != nil {
			return newGameSaveCommandForTest{}, err
		}
		if len(commands) > 1 {
			return newGameSaveCommandForTest{}, fmt.Errorf("save-now command count = %d, want 1", len(commands))
		}
		if len(commands) == 1 {
			command := commands[0]
			if command.Payload["transactionId"] != transactionID || command.Payload["saveId"] != saveID {
				return newGameSaveCommandForTest{}, fmt.Errorf("save-now identity = %+v", command)
			}
			return command, nil
		}
		select {
		case <-ctx.Done():
			return newGameSaveCommandForTest{}, ctx.Err()
		case <-ticker.C:
		}
	}
}

func readNewGameSaveCommandsForTest(dataDir string) ([]newGameSaveCommandForTest, error) {
	dir := filepath.Join(controlDir(dataDir), "commands")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	commands := make([]newGameSaveCommandForTest, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		var command newGameSaveCommandForTest
		if err := json.Unmarshal(raw, &command); err != nil {
			return nil, err
		}
		if command.Name == "save-now" {
			commands = append(commands, command)
		}
	}
	return commands, nil
}

func assertSingleNewGameSaveCommand(t *testing.T, dataDir, transactionID, saveID, commandID string) {
	t.Helper()
	commands, err := readNewGameSaveCommandsForTest(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(commands) != 1 {
		t.Fatalf("save-now command count = %d, want 1", len(commands))
	}
	command := commands[0]
	if command.ID != commandID || command.Payload["transactionId"] != transactionID || command.Payload["saveId"] != saveID {
		t.Fatalf("save-now command = %+v, commandID=%q", command, commandID)
	}
}

func stageNewGameCandidate(tx *newGameTransaction, saveID string) error {
	if err := os.MkdirAll(filepath.Join(savesDir(tx.dataDir), "Saves", saveID), 0o755); err != nil {
		return err
	}
	if err := writeGameloaderPointer(tx.dataDir, saveID); err != nil {
		return err
	}
	status := newGameControlDurabilityStatus{
		State: "save-creating", SaveID: saveID, UpdatedAt: time.Now().UTC(),
		NewGameTransactionID: tx.record.TransactionID, NewGameCreationObserved: true,
	}
	if err := writeNewGameJSONAtomicForTest(context.Background(), filepath.Join(controlDir(tx.dataDir), "status.json"), status); err != nil {
		return err
	}
	if signal, ok := newGameCandidateSignalsForTest.Load(tx.dataDir); ok {
		select {
		case signal.(chan string) <- saveID:
		default:
		}
	}
	return nil
}

func writeNewGameJSONAtomicForTest(ctx context.Context, path string, value any) error {
	ticker := time.NewTicker(2 * time.Millisecond)
	defer ticker.Stop()
	var lastErr error
	for attempts := 0; attempts < 100; attempts++ {
		if err := writeJSONAtomic(path, value); err == nil {
			return nil
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
	return lastErr
}

func writeNewGameDurableDiskFixture(dataDir, saveID string, cfg registry.NewGameConfig, whichFarm string) error {
	mainData := []byte(validNewGameMainXML(cfg, whichFarm))
	infoData := []byte(validNewGameInfoXML(cfg))
	root := filepath.Join(savesDir(dataDir), "Saves", saveID)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(root, saveID), mainData, 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, "SaveGameInfo"), infoData, 0o644)
}

func writeBrokenNewGameDiskFixture(dataDir, saveID string, cfg registry.NewGameConfig, _ string) error {
	root := filepath.Join(savesDir(dataDir), "Saves", saveID)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(root, saveID), []byte(`<SaveGame><player>`), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, "SaveGameInfo"), []byte(validNewGameInfoXML(cfg)), 0o644)
}

func prepareNewGameTestTransaction(t *testing.T, dataDir, farmType string) *newGameTransaction {
	t.Helper()
	tx, err := beginNewGameTransaction(dataDir, newGameTestConfig(farmType))
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.prepareConfigAndMarker(); err != nil {
		t.Fatal(err)
	}
	return tx
}

func newGameTestRunner(dataDir string, lifecycle LifecycleDockerService) *lifecycleRunner {
	return &lifecycleRunner{
		lifecycle: lifecycle, instance: storage.Instance{ID: "stardew", DataDir: dataDir},
		newGameAPIReadyTimeout: 150 * time.Millisecond, newGameObservationTimeout: 300 * time.Millisecond,
		newGameCatalogTimeout: 20 * time.Millisecond, newGamePollInterval: 10 * time.Millisecond, newGameCommandTimeout: 50 * time.Millisecond,
		newGameControlGateTimeout: time.Second, newGameSaveGateTimeout: time.Second,
		newGameDiskGateTimeout: 300 * time.Millisecond,
	}
}

func runNewGameCommandJob(t *testing.T, runner *lifecycleRunner, tx *newGameTransaction) error {
	t.Helper()
	storeDir := t.TempDir()
	store, err := storage.Open(context.Background(), appconfig.Config{DataDir: storeDir, DBPath: filepath.Join(storeDir, "panel.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	manager := jobs.NewManager(store, slog.Default())
	job, err := manager.Start(context.Background(), jobs.Spec{
		Type: "new-game-test", TargetType: "instance", TargetID: "stardew", Timeout: 3 * time.Second,
		Run: func(ctx context.Context, jobCtx *jobs.Context) error {
			return runner.sendNewGameCommand(ctx, jobCtx, tx)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		stored, getErr := store.GetJob(context.Background(), job.ID)
		if getErr == nil && (stored.Status == storage.JobStatusSucceeded || stored.Status == storage.JobStatusFailed) {
			if stored.Status == storage.JobStatusSucceeded {
				return nil
			}
			return errors.New(stored.ErrorMessage.String)
		}
		time.Sleep(5 * time.Millisecond)
	}
	return fmt.Errorf("job did not finish")
}

func assertNewGameErrorCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), code) {
		// jobs persist Error(), which contains the localized message but not the
		// code. Fall back to the expected semantic fragment for this test helper.
		semantic := map[string]string{
			"new_game_outcome_unknown":         "结果未知",
			"new_game_ambiguous":               "多个新存档",
			"new_game_xml_invalid":             "XML",
			"new_game_farm_type_mismatch":      "farm type mismatch",
			"new_game_pointer_without_save":    "gameloader",
			"new_game_disk_durability_timeout": "主存档与 SaveGameInfo",
			"new_game_disk_farm_type_mismatch": "whichFarm",
		}[code]
		if err == nil || semantic == "" || !strings.Contains(err.Error(), semantic) {
			t.Fatalf("error = %v, want code %s", err, code)
		}
	}
}

func writeNewGameTestSave(t *testing.T, dataDir, name, whichFarm string) {
	t.Helper()
	writeNewGameTestRawSave(t, dataDir, name, fmt.Sprintf(`<SaveGame><player><name>Farmer</name><farmName>Farm</farmName></player><whichFarm>%s</whichFarm></SaveGame>`, whichFarm))
}

func writeNewGameTestRawSave(t *testing.T, dataDir, name, content string) {
	t.Helper()
	dir := filepath.Join(savesDir(dataDir), "Saves", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeNewGameSnapshotFixture(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeNewGameTestMod(t *testing.T, dataDir string, enabled bool, folder, uniqueID string) {
	t.Helper()
	base := "mods-disabled"
	if enabled {
		base = "mods"
	}
	dir := filepath.Join(dataDir, ".local-container", base, folder)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := fmt.Sprintf(`{"Name":%q,"UniqueID":%q,"Version":"1.0.0"}`, folder, uniqueID)
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readJSONFileForTest(t *testing.T, path string, target any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		t.Fatal(err)
	}
}
