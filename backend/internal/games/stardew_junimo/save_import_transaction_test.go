package stardew_junimo

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
)

func testImportRequest(dir, operationID, platformID string) registry.SaveImportRequest {
	return registry.SaveImportRequest{Instance: registry.Instance{ID: "instance-1", DataDir: dir}, OperationID: operationID, SaveName: "Imported_123", HostHandling: "swap_host_to", PlatformID: platformID, StagedDir: filepath.Join(dir, "staged")}
}

func TestImportJournalIdempotentAndSensitiveIDNotPersisted(t *testing.T) {
	dir := t.TempDir()
	op := "00112233445566778899aabbccddeeff"
	platformID := "76561198012345678"
	first, err := CreateImportJournal(dir, testImportRequest(dir, op, platformID))
	if err != nil {
		t.Fatal(err)
	}
	second, err := CreateImportJournal(dir, testImportRequest(dir, op, platformID))
	if err != nil {
		t.Fatal(err)
	}
	if first.OperationID != second.OperationID || first.Stage != ImportStageValidated {
		t.Fatalf("not idempotent: %#v %#v", first, second)
	}
	data, err := os.ReadFile(importJournalPath(dir, op))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), platformID) {
		t.Fatal("journal leaked full platform ID")
	}
	if first.PlatformIDFingerprint == "" {
		t.Fatal("missing platform fingerprint")
	}
	mismatch := testImportRequest(dir, op, "76561198000000000")
	if _, err := CreateImportJournal(dir, mismatch); err == nil {
		t.Fatal("same operation accepted a different platform fingerprint")
	} else if typed, ok := AsImportTransactionError(err); !ok || typed.Code != "operation_conflict" {
		t.Fatalf("mismatch error=%v", err)
	}
	if info, err := os.Stat(importJournalPath(dir, op)); err != nil {
		t.Fatal(err)
	} else if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("journal mode = %v", info.Mode().Perm())
	}
}

func TestImportJournalSameNameDoesNotModifyExistingSave(t *testing.T) {
	dir := t.TempDir()
	saveDir := filepath.Join(savesDir(dir), "Saves", "Imported_123")
	if err := os.MkdirAll(saveDir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(saveDir, "Imported_123")
	original := []byte("exact existing bytes")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := CreateImportJournal(dir, testImportRequest(dir, "10112233445566778899aabbccddeeff", ""))
	typed, ok := AsImportTransactionError(err)
	if !ok || typed.Code != ImportErrorSaveExists {
		t.Fatalf("error=%v typed=%#v", err, typed)
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(original) {
		t.Fatal("existing save bytes changed")
	}
}

func TestRecoverImportTransactionAtEveryStage(t *testing.T) {
	stages := []string{ImportStageValidated, ImportStageStaged, ImportStageBackupCreated, ImportStageRuntimeReady, ImportStageSubmitted, ImportStageConfirmed, ImportStageSaveActivating, ImportStageFinalizeConfirmed, ImportStageSavePersisting, ImportStageSaveVerified}
	for _, stage := range stages {
		t.Run(stage, func(t *testing.T) {
			dir := t.TempDir()
			op := "20112233445566778899aabbccddeeff"
			j, err := CreateImportJournal(dir, testImportRequest(dir, op, ""))
			if err != nil {
				t.Fatal(err)
			}
			j.Stage = stage
			j.UpstreamSubmitted = importStageAtLeast(stage, ImportStageSubmitted)
			j.UpstreamConfirmed = importStageAtLeast(stage, ImportStageConfirmed)
			if err := WriteImportJournal(dir, j); err != nil {
				t.Fatal(err)
			}
			recovered, err := RecoverImportTransactions(dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(recovered) != 1 {
				t.Fatalf("recoveries=%v", recovered)
			}
			durableResume := stage == ImportStageFinalizeConfirmed || stage == ImportStageSavePersisting || stage == ImportStageSaveVerified
			activationResume := stage == ImportStageConfirmed || stage == ImportStageSaveActivating
			if durableResume && recovered[0].State != "resume_save_verification" {
				t.Fatalf("durable stage was not resumable: %#v", recovered[0])
			}
			if activationResume && recovered[0].State != "resume_activation_verification" {
				t.Fatalf("activation stage was not safely resumable: %#v", recovered[0])
			}
			if importStageAtLeast(stage, ImportStageSubmitted) && !durableResume && !activationResume && recovered[0].State != "manual_required" {
				t.Fatalf("submitted stage was not manual: %#v", recovered[0])
			}
			if !importStageAtLeast(stage, ImportStageSubmitted) && recovered[0].State != "safe_to_resume_or_cleanup" {
				t.Fatalf("pre-submit stage not safe: %#v", recovered[0])
			}
		})
	}
}

func TestCleanupImportStopsAfterUpstreamSubmission(t *testing.T) {
	dir := t.TempDir()
	op := "30112233445566778899aabbccddeeff"
	j, err := CreateImportJournal(dir, testImportRequest(dir, op, ""))
	if err != nil {
		t.Fatal(err)
	}
	if err := CleanupUnsubmittedImport(dir, op); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadImportJournal(dir, op); !os.IsNotExist(err) {
		t.Fatalf("journal still exists: %v", err)
	}
	j, err = CreateImportJournal(dir, testImportRequest(dir, op, ""))
	if err != nil {
		t.Fatal(err)
	}
	j.Stage, j.UpstreamSubmitted = ImportStageSubmitted, true
	if err := WriteImportJournal(dir, j); err != nil {
		t.Fatal(err)
	}
	err = CleanupUnsubmittedImport(dir, op)
	typed, ok := AsImportTransactionError(err)
	if !ok || typed.Code != ImportErrorRecoveryRequired {
		t.Fatalf("cleanup error=%v", err)
	}
	if _, err := LoadImportJournal(dir, op); err != nil {
		t.Fatalf("submitted journal was removed: %v", err)
	}
}

func TestValidateImportCapabilityVersionDLLAndFIFO(t *testing.T) {
	writeRuntime := func(t *testing.T, version, modVersion string, fifo bool) error {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("IMAGE_VERSION="+version+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		modDir := junimoServerModDir(dir)
		if err := os.MkdirAll(modDir, 0o700); err != nil {
			t.Fatal(err)
		}
		manifest := `{"Name":"JunimoServer","Version":"` + modVersion + `","UniqueID":"JunimoHost.Server"}`
		if err := os.WriteFile(filepath.Join(modDir, "manifest.json"), []byte(manifest), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(modDir, "JunimoServer.dll"), []byte("dll"), 0o600); err != nil {
			t.Fatal(err)
		}
		return ValidateImportCapability(dir, fifo)
	}
	for _, tc := range []struct {
		name, image, mod string
		fifo, wantOK     bool
	}{
		{"121", "1.5.0-preview.121", "1.5.0-preview.121", true, false},
		{"125", "1.5.0-preview.125", "1.5.0-preview.125", true, true},
		{"126_not_yet_qualified", "1.5.0-preview.126", "1.5.0-preview.126", true, false},
		{"old_dll", "1.5.0-preview.125", "1.5.0-preview.121", true, false},
		{"fifo_missing", "1.5.0-preview.125", "1.5.0-preview.125", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := writeRuntime(t, tc.image, tc.mod, tc.fifo)
			if (err == nil) != tc.wantOK {
				t.Fatalf("err=%v wantOK=%v", err, tc.wantOK)
			}
		})
	}
}
