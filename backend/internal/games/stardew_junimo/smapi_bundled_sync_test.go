package stardew_junimo

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestEnsureManagedSMAPIBundledModsPublishesFirstLaunchRuntimeSet(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "stardew")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	docker := &fakeDocker{bundledSyncMods: []runtimeLoadedMod{
		{UniqueID: consoleCommandsID, Version: "4.5.2"},
		{UniqueID: saveBackupID, Version: "4.5.2"},
		{UniqueID: "SMAPI.FutureBundledSupport", Version: "1.0.0"},
	}}
	driver := New(docker, nil, nil, nil)

	changed, err := driver.EnsureManagedSMAPIBundledMods(context.Background(), dataDir, "example/server:test", nil)
	if err != nil {
		t.Fatalf("first bundled sync: %v", err)
	}
	if !changed {
		t.Fatal("first bundled sync should publish the managed directory")
	}
	loaded, err := bundledSMAPIRuntimeLoadedMods(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 3 {
		t.Fatalf("managed runtime mods = %#v, want all three source manifests", loaded)
	}
	if docker.bundledSyncRuns != 1 {
		t.Fatalf("sync runs = %d, want 1", docker.bundledSyncRuns)
	}

	changed, err = driver.EnsureManagedSMAPIBundledMods(context.Background(), dataDir, "example/server:test", nil)
	if err != nil {
		t.Fatalf("idempotent bundled sync: %v", err)
	}
	if changed {
		t.Fatal("identical bundled runtime tree should not be replaced")
	}
}

func TestEnsureManagedSMAPIBundledModsRejectsInvalidStageWithoutReplacingCurrent(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "stardew")
	current := filepath.Join(modsDir(dataDir), "smapi", "Current")
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, "sentinel.txt"), []byte("preserve"), 0o644); err != nil {
		t.Fatal(err)
	}
	docker := &fakeDocker{bundledSyncMods: []runtimeLoadedMod{
		{UniqueID: "SMAPI.UnexpectedOnly", Version: "1.0.0"},
	}}
	driver := New(docker, nil, nil, nil)

	if _, err := driver.EnsureManagedSMAPIBundledMods(context.Background(), dataDir, "example/server:test", nil); err == nil {
		t.Fatal("sync without required SMAPI support manifests should fail")
	}
	data, err := os.ReadFile(filepath.Join(current, "sentinel.txt"))
	if err != nil {
		t.Fatalf("read preserved current tree: %v", err)
	}
	if string(data) != "preserve" {
		t.Fatalf("current managed tree changed to %q", data)
	}
	staging, err := filepath.Glob(filepath.Join(modsDir(dataDir), ".smapi-sync-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(staging) != 0 {
		t.Fatalf("staging directories were not cleaned: %v", staging)
	}
}

func TestRecoverManagedSMAPIBundledPublicationRestoresInterruptedBackup(t *testing.T) {
	dataDir := t.TempDir()
	root := modsDir(dataDir)
	backup := filepath.Join(root, ".smapi-backup-interrupted")
	createTestMod(t, backup, "ConsoleCommands", consoleCommandsID, "Console Commands")
	createTestMod(t, backup, "SaveBackup", saveBackupID, "Save Backup")
	orphanStage := filepath.Join(root, ".smapi-sync-interrupted")
	if err := os.MkdirAll(orphanStage, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(orphanStage, "partial"), []byte("partial"), 0o644); err != nil {
		t.Fatal(err)
	}

	destination := filepath.Join(root, "smapi")
	if err := recoverManagedSMAPIBundledPublication(root, destination); err != nil {
		t.Fatalf("recover interrupted publication: %v", err)
	}
	loaded, err := bundledSMAPIRuntimeLoadedMods(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 2 {
		t.Fatalf("recovered runtime mods = %#v", loaded)
	}
	for _, path := range []string{backup, orphanStage} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("owned crash artifact still exists at %s: %v", path, err)
		}
	}
}

func TestValidateManagedSMAPIBundledStageRejectsDuplicateAndEmptyVersion(t *testing.T) {
	t.Run("duplicate unique id", func(t *testing.T) {
		root := t.TempDir()
		createTestMod(t, root, "ConsoleCommands", consoleCommandsID, "Console Commands")
		createTestMod(t, root, "ConsoleCommandsDuplicate", consoleCommandsID, "Console Commands Duplicate")
		createTestMod(t, root, "SaveBackup", saveBackupID, "Save Backup")
		if err := validateManagedSMAPIBundledStage(root); err == nil || !strings.Contains(err.Error(), "duplicate UniqueID") {
			t.Fatalf("duplicate validation error = %v", err)
		}
	})

	t.Run("empty version", func(t *testing.T) {
		root := t.TempDir()
		createTestMod(t, root, "SaveBackup", saveBackupID, "Save Backup")
		dir := filepath.Join(root, "ConsoleCommands")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		manifest := `{"Name":"Console Commands","UniqueID":"SMAPI.ConsoleCommands","Version":""}`
		if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := validateManagedSMAPIBundledStage(root); err == nil || !strings.Contains(err.Error(), "missing UniqueID or Version") {
			t.Fatalf("empty version validation error = %v", err)
		}
	})
}

func TestManagedSMAPIBundledTreeDigestRejectsUnsafeEntriesAndSize(t *testing.T) {
	t.Run("symbolic link", func(t *testing.T) {
		root := t.TempDir()
		target := filepath.Join(root, "target")
		if err := os.WriteFile(target, []byte("target"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(root, "link")); err != nil {
			if runtime.GOOS == "windows" {
				t.Skipf("symbolic link privilege is unavailable: %v", err)
			}
			t.Fatal(err)
		}
		if _, err := managedSMAPIBundledTreeDigest(root); err == nil || !strings.Contains(err.Error(), "symbolic link") {
			t.Fatalf("symbolic link digest error = %v", err)
		}
	})

	t.Run("size limit", func(t *testing.T) {
		root := t.TempDir()
		oversized := filepath.Join(root, "oversized")
		file, err := os.Create(oversized)
		if err != nil {
			t.Fatal(err)
		}
		if err := file.Truncate(maxSMAPIBundledRuntimeSize + 1); err != nil {
			_ = file.Close()
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		if _, err := managedSMAPIBundledTreeDigest(root); err == nil || !strings.Contains(err.Error(), "exceeds") {
			t.Fatalf("oversized digest error = %v", err)
		}
	})
}

func TestExpectedRuntimeFingerprintIncludesEveryManagedSMAPIBundledManifest(t *testing.T) {
	dataDir := t.TempDir()
	createTestMod(t, modsDir(dataDir), "Control", "AnXiYiZhi.StardewAnxiPanel.Control", "Control")
	createTestMod(t, filepath.Join(modsDir(dataDir), "smapi"), "ConsoleCommands", consoleCommandsID, "Console Commands")
	createTestMod(t, filepath.Join(modsDir(dataDir), "smapi"), "SaveBackup", saveBackupID, "Save Backup")
	createTestMod(t, filepath.Join(modsDir(dataDir), "smapi"), "Future", "SMAPI.FutureBundledSupport", "Future Support")

	got, err := expectedRuntimeModFingerprint(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	want := runtimeModFingerprint([]runtimeLoadedMod{
		{UniqueID: "AnXiYiZhi.StardewAnxiPanel.Control", Version: "1.0.0"},
		{UniqueID: consoleCommandsID, Version: "1.0.0"},
		{UniqueID: saveBackupID, Version: "1.0.0"},
		{UniqueID: "SMAPI.FutureBundledSupport", Version: "1.0.0"},
	})
	if got != want {
		t.Fatalf("fingerprint = %s, want %s", got, want)
	}
}
