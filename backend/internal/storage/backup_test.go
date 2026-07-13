package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
)

func TestBackupToCreatesConsistentSQLiteSnapshot(t *testing.T) {
	dataDir := t.TempDir()
	store, err := Open(context.Background(), config.Config{DataDir: dataDir, DBPath: filepath.Join(dataDir, "panel.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := store.SetPanelSetting(context.Background(), "backup-test", "present"); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dataDir, "updater", "backups", "test", "panel.db")
	if err := store.BackupTo(context.Background(), target); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(target)
	if err != nil || info.Size() == 0 {
		t.Fatalf("backup info=%v err=%v", info, err)
	}
	backup, err := Open(context.Background(), config.Config{DataDir: filepath.Dir(target), DBPath: target})
	if err != nil {
		t.Fatal(err)
	}
	defer backup.Close()
	value, err := backup.GetPanelSetting(context.Background(), "backup-test")
	if err != nil || value != "present" {
		t.Fatalf("backup value=%q err=%v", value, err)
	}
}
