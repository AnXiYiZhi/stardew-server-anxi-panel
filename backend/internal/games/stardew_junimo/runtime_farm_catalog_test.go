package stardew_junimo

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
)

func TestRuntimeFarmCatalogFresh(t *testing.T) {
	tx := prepareRuntimeCatalogTestTransaction(t, "FrontierFarm")
	writeRuntimeCatalogTestOptions(t, tx, runtimeFarmCatalog{
		SchemaVersion:  runtimeFarmCatalogSchemaVersion,
		Source:         "smapi-runtime",
		RequestID:      tx.record.TransactionID,
		TransactionID:  tx.record.TransactionID,
		GeneratedAt:    time.Now().UTC(),
		LoadedMods:     []runtimeLoadedMod{},
		ModFingerprint: tx.record.ExpectedFingerprint,
		FarmTypes:      []runtimeFarmType{{ID: "FrontierFarm", Kind: "modded", GeneratedAt: time.Now().UTC()}},
	})
	if err := tx.waitForRuntimeFarmCatalog(context.Background(), 50*time.Millisecond, time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if tx.record.Stage != newGameStateCatalogReady {
		t.Fatalf("stage = %s", tx.record.Stage)
	}
}

func TestRuntimeModFingerprintMatchesControlContract(t *testing.T) {
	mods := []runtimeLoadedMod{
		{UniqueID: "Pathoschild.ContentPatcher", Version: "2.0.0"},
		{UniqueID: "FlashShifter.SVECode", Version: "1.0.0"},
	}
	const want = "0bc44377624ec2e2b98cda195b9df9ba06d9feed38be9c83991566d42bc12e22"
	if got := runtimeModFingerprint(mods); got != want {
		t.Fatalf("fingerprint = %s, want %s", got, want)
	}
}

func TestExpectedRuntimeModFingerprintIncludesDisabledBundledSMAPIMods(t *testing.T) {
	dir := t.TempDir()
	createTestMod(t, modsDir(dir), "Control", "AnXiYiZhi.StardewAnxiPanel.Control", "Control")
	createTestMod(t, disabledModsDir(dir), "ConsoleCommands", "SMAPI.ConsoleCommands", "Console Commands")
	createTestMod(t, disabledModsDir(dir), "SaveBackup", "SMAPI.SaveBackup", "Save Backup")

	got, err := expectedRuntimeModFingerprint(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := runtimeModFingerprint([]runtimeLoadedMod{
		{UniqueID: "AnXiYiZhi.StardewAnxiPanel.Control", Version: "1.0.0"},
		{UniqueID: "SMAPI.ConsoleCommands", Version: "1.0.0"},
		{UniqueID: "SMAPI.SaveBackup", Version: "1.0.0"},
	})
	if got != want {
		t.Fatalf("fingerprint = %s, want %s", got, want)
	}
}

func TestRuntimeFarmCatalogValidationFailures(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*runtimeFarmCatalog)
		code   string
	}{
		{name: "stale request id", mutate: func(c *runtimeFarmCatalog) { c.RequestID = "old" }, code: "runtime_catalog_stale"},
		{name: "fingerprint mismatch", mutate: func(c *runtimeFarmCatalog) { c.ModFingerprint = strings.Repeat("0", 64) }, code: "mod_fingerprint_mismatch"},
		{name: "target absent", mutate: func(c *runtimeFarmCatalog) { c.FarmTypes = []runtimeFarmType{{ID: "OtherFarm"}} }, code: "farm_type_not_loaded"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tx := prepareRuntimeCatalogTestTransaction(t, "FrontierFarm")
			catalog := validRuntimeCatalogForTest(tx)
			tc.mutate(&catalog)
			writeRuntimeCatalogTestOptions(t, tx, catalog)
			err := tx.waitForRuntimeFarmCatalog(context.Background(), 50*time.Millisecond, time.Millisecond)
			assertRuntimeCatalogErrorCode(t, err, tc.code)
		})
	}
}

func TestRuntimeFarmCatalogRejectsBrokenAndOversizedOptions(t *testing.T) {
	t.Run("broken JSON", func(t *testing.T) {
		tx := prepareRuntimeCatalogTestTransaction(t, "FrontierFarm")
		if err := os.WriteFile(runtimeOptionsPath(tx.dataDir), []byte(`{"schemaVersion":`), 0o644); err != nil {
			t.Fatal(err)
		}
		err := tx.waitForRuntimeFarmCatalog(context.Background(), 50*time.Millisecond, time.Millisecond)
		assertRuntimeCatalogErrorCode(t, err, "runtime_catalog_invalid")
	})
	t.Run("too large", func(t *testing.T) {
		tx := prepareRuntimeCatalogTestTransaction(t, "FrontierFarm")
		if err := os.WriteFile(runtimeOptionsPath(tx.dataDir), make([]byte, maxRuntimeOptionsBytes+1), 0o644); err != nil {
			t.Fatal(err)
		}
		err := tx.waitForRuntimeFarmCatalog(context.Background(), 50*time.Millisecond, time.Millisecond)
		assertRuntimeCatalogErrorCode(t, err, "runtime_catalog_too_large")
	})
}

func TestRuntimeFarmCatalogOldSchemaCompatibility(t *testing.T) {
	t.Run("official remains compatible", func(t *testing.T) {
		tx := prepareRuntimeCatalogTestTransaction(t, "standard")
		writeRuntimeCatalogTestOptions(t, tx, runtimeFarmCatalog{SchemaVersion: 1})
		if err := tx.waitForRuntimeFarmCatalog(context.Background(), 50*time.Millisecond, time.Millisecond); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("modded requires upgraded control", func(t *testing.T) {
		tx := prepareRuntimeCatalogTestTransaction(t, "FrontierFarm")
		writeRuntimeCatalogTestOptions(t, tx, runtimeFarmCatalog{SchemaVersion: 1})
		err := tx.waitForRuntimeFarmCatalog(context.Background(), 50*time.Millisecond, time.Millisecond)
		assertRuntimeCatalogErrorCode(t, err, "control_mod_catalog_unsupported")
	})
}

func TestRuntimeFarmCatalogWaitTimeout(t *testing.T) {
	tx := prepareRuntimeCatalogTestTransaction(t, "FrontierFarm")
	err := tx.waitForRuntimeFarmCatalog(context.Background(), 10*time.Millisecond, time.Millisecond)
	assertRuntimeCatalogErrorCode(t, err, "runtime_catalog_timeout")
}

func TestRuntimeFarmCatalogFailureStopsBeforeNewGamePost(t *testing.T) {
	tx := prepareRuntimeCatalogTestTransaction(t, "FrontierFarm")
	catalog := validRuntimeCatalogForTest(tx)
	catalog.FarmTypes = nil
	writeRuntimeCatalogTestOptions(t, tx, catalog)
	var newGameCalls atomic.Int32
	fake := &fakeConsoleDocker{execFunc: func(_ context.Context, _, _, _ string, args ...string) (paneldocker.CommandResult, error) {
		if strings.Contains(strings.Join(args, " "), "/newgame") {
			newGameCalls.Add(1)
		}
		return paneldocker.CommandResult{ExitCode: 0}, nil
	}}
	err := runNewGameCommandJob(t, newGameTestRunner(tx.dataDir, fake), tx)
	if err == nil || !strings.Contains(err.Error(), "farm_type_not_loaded") {
		t.Fatalf("err = %v", err)
	}
	if got := newGameCalls.Load(); got != 0 {
		t.Fatalf("POST /newgame calls = %d", got)
	}
}

func TestModdedNewGameValidatesXMLAndCommitsExactProfile(t *testing.T) {
	tx := prepareRuntimeCatalogTestTransaction(t, "FrontierFarm")
	tx.record.ModSelection = &NewGameModSelection{FarmTypeID: "FrontierFarm"}
	tx.record.EnabledModKeys = []string{"unique:FlashShifter.FrontierFarm", "unique:Pathoschild.ContentPatcher"}
	writeRuntimeCatalogTestOptions(t, tx, validRuntimeCatalogForTest(tx))
	var committedSave string
	var committedKeys []string
	fake := &fakeConsoleDocker{execFunc: func(_ context.Context, _, _, _ string, args ...string) (paneldocker.CommandResult, error) {
		if strings.Contains(strings.Join(args, " "), "/newgame") {
			writeNewGameTestSave(t, tx.dataDir, "Frontier_101", "FrontierFarm")
		}
		return paneldocker.CommandResult{ExitCode: 0}, nil
	}}
	runner := newGameTestRunner(tx.dataDir, fake)
	runner.commitNewGameModProfile = func(_ string, save string, keys []string) error {
		committedSave = save
		committedKeys = append([]string{}, keys...)
		return nil
	}
	if err := runNewGameCommandJob(t, runner, tx); err != nil {
		t.Fatal(err)
	}
	if committedSave != "Frontier_101" || strings.Join(committedKeys, ",") != strings.Join(tx.record.EnabledModKeys, ",") {
		t.Fatalf("profile commit save=%q keys=%v", committedSave, committedKeys)
	}
	if tx.record.Stage != newGameStateSuccess || tx.record.ResolvedFarmType != "FrontierFarm" {
		t.Fatalf("record = %#v", tx.record)
	}
}

func TestModdedNewGameStandardXMLMismatchIsQuarantinedOnRollback(t *testing.T) {
	tx := prepareRuntimeCatalogTestTransaction(t, "FrontierFarm")
	writeRuntimeCatalogTestOptions(t, tx, validRuntimeCatalogForTest(tx))
	fake := &fakeConsoleDocker{execFunc: func(_ context.Context, _, _, _ string, args ...string) (paneldocker.CommandResult, error) {
		if strings.Contains(strings.Join(args, " "), "/newgame") {
			writeNewGameTestSave(t, tx.dataDir, "Wrong_202", "0")
		}
		return paneldocker.CommandResult{ExitCode: 0}, nil
	}}
	err := runNewGameCommandJob(t, newGameTestRunner(tx.dataDir, fake), tx)
	if err == nil || !strings.Contains(err.Error(), "farm type mismatch") {
		t.Fatalf("err = %v", err)
	}
	if rollbackErr := tx.rollback(err, "farm_type_mismatch", newGameStateFailed); rollbackErr != nil {
		t.Fatal(rollbackErr)
	}
	if _, statErr := os.Stat(filepath.Join(tx.dataDir, ".local-container", "saves-quarantine", "new-game", tx.record.TransactionID, "Wrong_202")); statErr != nil {
		t.Fatalf("mismatched save not quarantined: %v", statErr)
	}
}

func TestModdedNewGameProfileCommitFailurePreservesSaveAsRecoverable(t *testing.T) {
	tx := prepareRuntimeCatalogTestTransaction(t, "FrontierFarm")
	tx.record.ModSelection = &NewGameModSelection{FarmTypeID: "FrontierFarm"}
	writeRuntimeCatalogTestOptions(t, tx, validRuntimeCatalogForTest(tx))
	fake := &fakeConsoleDocker{execFunc: func(_ context.Context, _, _, _ string, args ...string) (paneldocker.CommandResult, error) {
		if strings.Contains(strings.Join(args, " "), "/newgame") {
			writeNewGameTestSave(t, tx.dataDir, "Keep_303", "FrontierFarm")
		}
		return paneldocker.CommandResult{ExitCode: 0}, nil
	}}
	runner := newGameTestRunner(tx.dataDir, fake)
	runner.commitNewGameModProfile = func(string, string, []string) error { return errors.New("injected profile failure") }
	err := runNewGameCommandJob(t, runner, tx)
	if err == nil || !strings.Contains(err.Error(), "profile") {
		t.Fatalf("err = %v", err)
	}
	if rollbackErr := tx.rollback(err, "mod_profile_commit_failed", newGameStateFailed); rollbackErr != nil {
		t.Fatal(rollbackErr)
	}
	if tx.record.Stage != newGameStateProfilePending || tx.record.Result != "recoverable" || tx.record.CreatedSave != "Keep_303" {
		t.Fatalf("record = %#v", tx.record)
	}
	if _, statErr := os.Stat(filepath.Join(savesDir(tx.dataDir), "Saves", "Keep_303", "Keep_303")); statErr != nil {
		t.Fatalf("correct save was not preserved: %v", statErr)
	}
}

func prepareRuntimeCatalogTestTransaction(t *testing.T, farmType string) *newGameTransaction {
	t.Helper()
	dataDir := t.TempDir()
	tx, err := beginNewGameTransaction(dataDir, newGameTestConfig("standard"))
	if err != nil {
		t.Fatal(err)
	}
	tx.record.RequestedFarmType = farmType
	tx.record.Config.FarmType = farmType
	if err := tx.prepareRuntimeCatalogRequest(); err != nil {
		t.Fatal(err)
	}
	return tx
}

func validRuntimeCatalogForTest(tx *newGameTransaction) runtimeFarmCatalog {
	return runtimeFarmCatalog{
		SchemaVersion:  runtimeFarmCatalogSchemaVersion,
		Source:         "smapi-runtime",
		RequestID:      tx.record.TransactionID,
		TransactionID:  tx.record.TransactionID,
		GeneratedAt:    time.Now().UTC(),
		LoadedMods:     []runtimeLoadedMod{},
		ModFingerprint: tx.record.ExpectedFingerprint,
		FarmTypes:      []runtimeFarmType{{ID: "FrontierFarm", Kind: "modded", GeneratedAt: time.Now().UTC()}},
	}
}

func writeRuntimeCatalogTestOptions(t *testing.T, tx *newGameTransaction, catalog runtimeFarmCatalog) {
	t.Helper()
	data, err := json.Marshal(catalog)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(controlDir(tx.dataDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(runtimeOptionsPath(tx.dataDir), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertRuntimeCatalogErrorCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %s", code)
	}
	var txErr *NewGameTransactionError
	if !errors.As(err, &txErr) || txErr.Code != code {
		t.Fatalf("error = %v, want %s", err, code)
	}
}
