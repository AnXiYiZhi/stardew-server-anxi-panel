package stardew_junimo

import (
	"archive/zip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
)

func writeImportSourceFixture(t *testing.T, root, saveName, mainBytes string) string {
	t.Helper()
	dir := filepath.Join(root, saveName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, saveName), []byte(mainBytes), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SaveGameInfo"), []byte("info:"+mainBytes), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "extra.dat"), []byte("extra:"+mainBytes), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestImportStagingSameNameRejectedWithoutByteChanges(t *testing.T) {
	dataDir := t.TempDir()
	sourceRoot := filepath.Join(t.TempDir(), "source")
	writeImportSourceFixture(t, sourceRoot, "Imported_123", "uploaded")
	existing := filepath.Join(savesDir(dataDir), "Saves", "Imported_123")
	writeImportSourceFixture(t, filepath.Dir(existing), "Imported_123", "existing-exact")
	before, err := importDirectoryFingerprint(existing)
	if err != nil {
		t.Fatal(err)
	}
	_, err = StageImportedSaveNoReplace(dataDir, sourceRoot, "Imported_123")
	typed, ok := AsImportTransactionError(err)
	if !ok || typed.Code != ImportErrorSaveExists {
		t.Fatalf("error=%v", err)
	}
	after, err := importDirectoryFingerprint(existing)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatal("existing save changed")
	}
}

func TestImportStagingAtomicRename(t *testing.T) {
	dataDir := t.TempDir()
	sourceRoot := filepath.Join(t.TempDir(), "source")
	sourceSave := writeImportSourceFixture(t, sourceRoot, "Imported_123", "atomic")
	fingerprint, err := StageImportedSaveNoReplace(dataDir, sourceRoot, "Imported_123")
	if err != nil {
		t.Fatal(err)
	}
	if fingerprint == "" {
		t.Fatal("missing staged fingerprint")
	}
	if _, err := os.Stat(sourceSave); !os.IsNotExist(err) {
		t.Fatalf("source still exists after rename: %v", err)
	}
	if _, err := os.Stat(filepath.Join(savesDir(dataDir), "Saves", "Imported_123", "Imported_123")); err != nil {
		t.Fatal(err)
	}
}

func TestImportStagingCrossFilesystemCopy(t *testing.T) {
	dataDir := t.TempDir()
	sourceRoot := filepath.Join(t.TempDir(), "source")
	writeImportSourceFixture(t, sourceRoot, "Imported_123", "cross-device")
	renameCalls := 0
	ops := defaultImportStageOps()
	defaultRename := ops.renameNoReplace
	ops.renameNoReplace = func(source, target string) error {
		renameCalls++
		if renameCalls == 1 {
			return errImportCrossDevice
		}
		return defaultRename(source, target)
	}
	if _, err := stageImportedSaveNoReplace(dataDir, sourceRoot, "Imported_123", ops); err != nil {
		t.Fatal(err)
	}
	if renameCalls != 2 {
		t.Fatalf("rename calls=%d", renameCalls)
	}
	data, err := os.ReadFile(filepath.Join(savesDir(dataDir), "Saves", "Imported_123", "Imported_123"))
	if err != nil || string(data) != "cross-device" {
		t.Fatalf("staged data=%q err=%v", data, err)
	}
}

func TestImportStagingInterruptedCopyHasNoVisibleSave(t *testing.T) {
	dataDir := t.TempDir()
	sourceRoot := filepath.Join(t.TempDir(), "source")
	writeImportSourceFixture(t, sourceRoot, "Imported_123", "partial")
	ops := defaultImportStageOps()
	ops.renameNoReplace = func(_, _ string) error { return errImportCrossDevice }
	ops.copyTree = func(_, target string) error {
		if err := os.WriteFile(filepath.Join(target, "partial"), []byte("partial"), 0o600); err != nil {
			t.Fatal(err)
		}
		return errors.New("injected copy interruption")
	}
	if _, err := stageImportedSaveNoReplace(dataDir, sourceRoot, "Imported_123", ops); err == nil {
		t.Fatal("interrupted copy succeeded")
	}
	savesRoot := filepath.Join(savesDir(dataDir), "Saves")
	if _, err := os.Stat(filepath.Join(savesRoot, "Imported_123")); !os.IsNotExist(err) {
		t.Fatalf("partial save became visible: %v", err)
	}
	entries, err := os.ReadDir(savesRoot)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".import-stage-") {
			t.Fatalf("hidden partial directory leaked: %s", entry.Name())
		}
	}
}

func createOwnedImportJournalFixture(t *testing.T, dataDir, operationID, uploadedBytes string) ImportJournal {
	t.Helper()
	req := registry.SaveImportRequest{Instance: registry.Instance{ID: "instance-1", DataDir: dataDir}, OperationID: operationID, SaveName: "Imported_123", HostHandling: "server_owns_original"}
	j, err := CreateImportJournal(dataDir, req)
	if err != nil {
		t.Fatal(err)
	}
	writeImportSourceFixture(t, importTransactionSourceDir(dataDir, operationID), "Imported_123", uploadedBytes)
	j.SourceOwned = true
	if err := WriteImportJournal(dataDir, j); err != nil {
		t.Fatal(err)
	}
	return j
}

func TestImportStagingRestartRecoveryFindsOwnedSource(t *testing.T) {
	dataDir := t.TempDir()
	op := "90112233445566778899aabbccddeeff"
	createOwnedImportJournalFixture(t, dataDir, op, "restart-source")
	recoveries, err := RecoverImportTransactions(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(recoveries) != 1 || !recoveries[0].SourceAvailable || recoveries[0].State != "safe_to_resume_or_cleanup" {
		t.Fatalf("recoveries=%+v", recoveries)
	}
}

func TestImportStagingJournalAndPreimportBackupTarget(t *testing.T) {
	dataDir := t.TempDir()
	op := "40112233445566778899aabbccddeeff"
	active := writeImportSourceFixture(t, filepath.Join(savesDir(dataDir), "Saves"), "Active_999", "active-bytes")
	_ = active
	if err := SetActiveSave(dataDir, "Active_999"); err != nil {
		t.Fatal(err)
	}
	createOwnedImportJournalFixture(t, dataDir, op, "uploaded-target-bytes")
	if err := prepareImportStaging(dataDir, op); err != nil {
		t.Fatal(err)
	}
	j, err := LoadImportJournal(dataDir, op)
	if err != nil {
		t.Fatal(err)
	}
	if j.Stage != ImportStageBackupCreated || !j.StagedSaveCreated || j.StagedSaveFingerprint == "" || j.PreimportBackupName == "" || j.PreimportBackupSHA256 == "" {
		t.Fatalf("journal=%+v", j)
	}
	backupFile := filepath.Join(backupsDir(dataDir), j.PreimportBackupName)
	actualSHA, err := stableFileSHA256(backupFile)
	if err != nil || actualSHA != j.PreimportBackupSHA256 {
		t.Fatalf("backup sha actual=%q journal=%q err=%v", actualSHA, j.PreimportBackupSHA256, err)
	}
	if inferBackupKind(j.PreimportBackupName) != "preimport" || !strings.Contains(j.PreimportBackupName, importOperationDigest(op)) {
		t.Fatalf("invalid preimport name/kind: %q", j.PreimportBackupName)
	}
	if j.OriginalActiveSave != "Active_999" {
		t.Fatalf("original active=%q", j.OriginalActiveSave)
	}
	zr, err := zip.OpenReader(backupFile)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	var main []byte
	for _, file := range zr.File {
		if filepath.ToSlash(file.Name) == "Imported_123/Imported_123" {
			r, openErr := file.Open()
			if openErr != nil {
				t.Fatal(openErr)
			}
			main, err = io.ReadAll(r)
			_ = r.Close()
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	if string(main) != "uploaded-target-bytes" {
		t.Fatalf("preimport backed up wrong save: %q", main)
	}
}

func TestImportStagingPreimportRestoresAndSurvivesCleanup(t *testing.T) {
	dataDir := t.TempDir()
	op := "50112233445566778899aabbccddeeff"
	createOwnedImportJournalFixture(t, dataDir, op, "restore-me")
	if err := prepareImportStaging(dataDir, op); err != nil {
		t.Fatal(err)
	}
	j, _ := LoadImportJournal(dataDir, op)
	backupPath := filepath.Join(backupsDir(dataDir), j.PreimportBackupName)
	if err := CleanupUnsubmittedImport(dataDir, op); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(backupPath); err != nil {
		t.Fatalf("preimport backup was removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(savesDir(dataDir), "Saves", "Imported_123")); !os.IsNotExist(err) {
		t.Fatalf("staged target survived cancel cleanup: %v", err)
	}
	name, err := RestoreBackup(dataDir, j.PreimportBackupName, false)
	if err != nil || name != "Imported_123" {
		t.Fatalf("restore name=%q err=%v", name, err)
	}
	main, err := os.ReadFile(filepath.Join(savesDir(dataDir), "Saves", name, name))
	if err != nil || string(main) != "restore-me" {
		t.Fatalf("restored bytes=%q err=%v", main, err)
	}
	if err := PruneAutoGameDayBackups(dataDir, "Imported_123", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(backupPath); err != nil {
		t.Fatalf("auto cleanup removed preimport: %v", err)
	}
}

func TestImportStagingSubmittedCleanupRejected(t *testing.T) {
	dataDir := t.TempDir()
	op := "60112233445566778899aabbccddeeff"
	createOwnedImportJournalFixture(t, dataDir, op, "submitted")
	if err := prepareImportStaging(dataDir, op); err != nil {
		t.Fatal(err)
	}
	j, _ := LoadImportJournal(dataDir, op)
	j.Stage, j.UpstreamSubmitted = ImportStageSubmitted, true
	if err := WriteImportJournal(dataDir, j); err != nil {
		t.Fatal(err)
	}
	err := CleanupUnsubmittedImport(dataDir, op)
	typed, ok := AsImportTransactionError(err)
	if !ok || typed.Code != ImportErrorRecoveryRequired {
		t.Fatalf("cleanup err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(savesDir(dataDir), "Saves", "Imported_123", "Imported_123")); err != nil {
		t.Fatalf("submitted target removed: %v", err)
	}
}
