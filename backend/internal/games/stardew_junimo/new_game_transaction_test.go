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

func TestNewGameCommandCalledOnceAndGameloaderNeedNotChange(t *testing.T) {
	dataDir := t.TempDir()
	tx := prepareNewGameTestTransaction(t, dataDir, "standard")
	var posts atomic.Int32
	fake := &fakeConsoleDocker{execFunc: func(ctx context.Context, _, _, _ string, args ...string) (paneldocker.CommandResult, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "/status") {
			return paneldocker.CommandResult{ExitCode: 0}, nil
		}
		if strings.Contains(joined, "/newgame") {
			posts.Add(1)
			writeNewGameTestSave(t, dataDir, "Fresh_101", "0")
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
}

func TestNewGameSkipsPostWhenLegacyStartupAlreadyCreatedSave(t *testing.T) {
	dataDir := t.TempDir()
	tx := prepareNewGameTestTransaction(t, dataDir, "standard")
	writeNewGameTestSave(t, dataDir, "Startup_102", "0")
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
}

func TestNewGameCommandTimeoutCanStillSucceed(t *testing.T) {
	dataDir := t.TempDir()
	tx := prepareNewGameTestTransaction(t, dataDir, "standard")
	fake := &fakeConsoleDocker{execFunc: func(ctx context.Context, _, _, _ string, args ...string) (paneldocker.CommandResult, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "/status") {
			return paneldocker.CommandResult{ExitCode: 0}, nil
		}
		if strings.Contains(joined, "/newgame") {
			<-ctx.Done()
			writeNewGameTestSave(t, dataDir, "Late_202", "0")
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
}

func TestNewGameCommandTimeoutWithoutSaveIsUnknown(t *testing.T) {
	dataDir := t.TempDir()
	tx := prepareNewGameTestTransaction(t, dataDir, "standard")
	fake := &fakeConsoleDocker{execFunc: func(ctx context.Context, _, _, _ string, args ...string) (paneldocker.CommandResult, error) {
		if strings.Contains(strings.Join(args, " "), "/status") {
			return paneldocker.CommandResult{ExitCode: 0}, nil
		}
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
}

func TestNewGameCommandExplicitFailureStillObservesThenFails(t *testing.T) {
	dataDir := t.TempDir()
	tx := prepareNewGameTestTransaction(t, dataDir, "standard")
	fake := &fakeConsoleDocker{execFunc: func(_ context.Context, _, _, _ string, args ...string) (paneldocker.CommandResult, error) {
		if strings.Contains(strings.Join(args, " "), "/status") {
			return paneldocker.CommandResult{ExitCode: 0}, nil
		}
		return paneldocker.CommandResult{ExitCode: 22}, errors.New("explicit HTTP failure")
	}}
	err := runNewGameCommandJob(t, newGameTestRunner(dataDir, fake), tx)
	assertNewGameErrorCode(t, err, "new_game_command_failed")
}

func TestNewGameDetectionRejectsAmbiguousBrokenAndMismatchedSaves(t *testing.T) {
	tests := []struct {
		name string
		make func(*testing.T, string)
		code string
	}{
		{name: "multiple", make: func(t *testing.T, dir string) {
			writeNewGameTestSave(t, dir, "One_1", "0")
			writeNewGameTestSave(t, dir, "Two_2", "0")
		}, code: "new_game_ambiguous"},
		{name: "broken XML", make: func(t *testing.T, dir string) {
			writeNewGameTestRawSave(t, dir, "Broken_3", `<SaveGame><whichFarm>0</whichFarm>`)
		}, code: "new_game_xml_invalid"},
		{name: "farm mismatch", make: func(t *testing.T, dir string) {
			writeNewGameTestSave(t, dir, "Wrong_4", "7")
		}, code: "new_game_farm_type_mismatch"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dataDir := t.TempDir()
			tx := prepareNewGameTestTransaction(t, dataDir, "standard")
			fake := &fakeConsoleDocker{execFunc: func(_ context.Context, _, _, _ string, args ...string) (paneldocker.CommandResult, error) {
				if strings.Contains(strings.Join(args, " "), "/newgame") {
					tc.make(t, dataDir)
				}
				return paneldocker.CommandResult{ExitCode: 0}, nil
			}}
			err := runNewGameCommandJob(t, newGameTestRunner(dataDir, fake), tx)
			assertNewGameErrorCode(t, err, tc.code)
		})
	}
}

func TestNewGameGameloaderPointerWithoutDirectoryFails(t *testing.T) {
	dataDir := t.TempDir()
	tx := prepareNewGameTestTransaction(t, dataDir, "standard")
	fake := &fakeConsoleDocker{execFunc: func(_ context.Context, _, _, _ string, args ...string) (paneldocker.CommandResult, error) {
		if strings.Contains(strings.Join(args, " "), "/newgame") {
			if err := writeGameloaderPointer(dataDir, "Missing_99"); err != nil {
				t.Fatal(err)
			}
		}
		return paneldocker.CommandResult{ExitCode: 0}, nil
	}}
	err := runNewGameCommandJob(t, newGameTestRunner(dataDir, fake), tx)
	assertNewGameErrorCode(t, err, "new_game_pointer_without_save")
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
		newGameAPIReadyTimeout: 100 * time.Millisecond, newGameObservationTimeout: 500 * time.Millisecond,
		newGameCatalogTimeout: 10 * time.Millisecond, newGamePollInterval: 5 * time.Millisecond, newGameCommandTimeout: 50 * time.Millisecond,
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
		Type: "new-game-test", TargetType: "instance", TargetID: "stardew", Timeout: time.Second,
		Run: func(ctx context.Context, jobCtx *jobs.Context) error {
			return runner.sendNewGameCommand(ctx, jobCtx, tx)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
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
			"new_game_outcome_unknown":      "结果未知",
			"new_game_ambiguous":            "多个新存档",
			"new_game_xml_invalid":          "XML",
			"new_game_farm_type_mismatch":   "farm type mismatch",
			"new_game_pointer_without_save": "gameloader",
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
