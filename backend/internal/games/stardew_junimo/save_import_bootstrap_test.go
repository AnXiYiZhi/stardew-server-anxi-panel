package stardew_junimo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCompletedImportBootstrapCleanupRequiresTargetPointer(t *testing.T) {
	dataDir := t.TempDir()
	op := "43112233445566778899aabbccddeeff"
	bootstrapName := importBootstrapSaveName(op)
	bootstrapDir := writeImportSourceFixture(t, filepath.Join(savesDir(dataDir), "Saves"), bootstrapName, "bootstrap")
	bootstrapSource := importBootstrapSourceRoot(dataDir, op)
	if err := os.MkdirAll(bootstrapSource, 0o700); err != nil {
		t.Fatal(err)
	}
	j := ImportJournal{OperationID: op, SaveName: "Imported_123", BootstrapSaveName: bootstrapName, BootstrapSaveCreated: true}
	if err := writeGameloaderPointer(dataDir, bootstrapName); err != nil {
		t.Fatal(err)
	}
	err := cleanupCompletedImportBootstrap(dataDir, &j)
	typed, ok := AsImportTransactionError(err)
	if !ok || typed.Code != ImportErrorRecoveryRequired {
		t.Fatalf("cleanup error=%v", err)
	}
	if _, err := os.Stat(bootstrapDir); err != nil {
		t.Fatalf("bootstrap removed while still active: %v", err)
	}

	if err := writeGameloaderPointer(dataDir, j.SaveName); err != nil {
		t.Fatal(err)
	}
	if err := cleanupCompletedImportBootstrap(dataDir, &j); err != nil {
		t.Fatal(err)
	}
	if !j.BootstrapCleanupCompleted {
		t.Fatalf("cleanup state=%+v", j)
	}
	if _, err := os.Stat(bootstrapDir); !os.IsNotExist(err) {
		t.Fatalf("completed bootstrap survived: %v", err)
	}
	if _, err := os.Stat(bootstrapSource); !os.IsNotExist(err) {
		t.Fatalf("completed bootstrap source survived: %v", err)
	}
}
