package stardew_junimo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplyNewSaveDefaultModStateDisablesNonBuiltInMods(t *testing.T) {
	dir := t.TempDir()
	root := modsDir(dir)
	createTestMod(t, root, controlModFolderName, controlModUniqueID, "Panel Control")
	createTestMod(t, root, "ContentPatcher", "Pathoschild.ContentPatcher", "Content Patcher")

	if err := ApplyNewSaveDefaultModState(dir); err != nil {
		t.Fatalf("ApplyNewSaveDefaultModState: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, controlModFolderName)); err != nil {
		t.Fatalf("control mod should stay enabled: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "ContentPatcher")); !os.IsNotExist(err) {
		t.Fatalf("third-party mod should be removed from active dir, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(disabledModsDir(dir), "ContentPatcher")); err != nil {
		t.Fatalf("third-party mod should move to disabled dir: %v", err)
	}
}

func TestSetModEnabledForSavePersistsAndAppliesProfile(t *testing.T) {
	dir := t.TempDir()
	root := modsDir(dir)
	saveName := "Farmer_12345"
	savePath := filepath.Join(savesDir(dir), "Saves", saveName)
	if err := os.MkdirAll(savePath, 0o755); err != nil {
		t.Fatalf("create save: %v", err)
	}
	createTestMod(t, root, "ContentPatcher", "Pathoschild.ContentPatcher", "Content Patcher")

	if _, err := SetModEnabledForSave(dir, saveName, "ContentPatcher", false); err != nil {
		t.Fatalf("disable mod: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "ContentPatcher")); !os.IsNotExist(err) {
		t.Fatalf("mod should leave active dir after disable, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(disabledModsDir(dir), "ContentPatcher")); err != nil {
		t.Fatalf("mod should be in disabled dir: %v", err)
	}

	mods, err := ListModsWithState(dir, saveName)
	if err != nil {
		t.Fatalf("ListModsWithState: %v", err)
	}
	if len(mods) != 1 || mods[0].Enabled {
		t.Fatalf("expected disabled mod in list, got %#v", mods)
	}

	if _, err := SetModEnabledForSave(dir, saveName, "Pathoschild.ContentPatcher", true); err != nil {
		t.Fatalf("enable mod by unique id: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "ContentPatcher")); err != nil {
		t.Fatalf("mod should return to active dir: %v", err)
	}
}

func TestApplyModProfileSwitchesPhysicalStateBetweenSaves(t *testing.T) {
	dir := t.TempDir()
	root := modsDir(dir)
	saveA := "FarmerA_111"
	saveB := "FarmerB_222"
	for _, saveName := range []string{saveA, saveB} {
		if err := os.MkdirAll(filepath.Join(savesDir(dir), "Saves", saveName), 0o755); err != nil {
			t.Fatalf("create save %s: %v", saveName, err)
		}
	}
	createTestMod(t, root, "ModA", "author.a", "Mod A")
	createTestMod(t, root, "ModB", "author.b", "Mod B")

	if _, err := SetModEnabledForSave(dir, saveA, "ModB", false); err != nil {
		t.Fatalf("disable ModB for saveA: %v", err)
	}
	if _, err := SetModEnabledForSave(dir, saveB, "ModA", false); err != nil {
		t.Fatalf("disable ModA for saveB: %v", err)
	}

	if err := ApplyModProfile(dir, saveA); err != nil {
		t.Fatalf("apply saveA profile: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "ModA")); err != nil {
		t.Fatalf("ModA should be enabled for saveA: %v", err)
	}
	if _, err := os.Stat(filepath.Join(disabledModsDir(dir), "ModB")); err != nil {
		t.Fatalf("ModB should be disabled for saveA: %v", err)
	}

	if err := ApplyModProfile(dir, saveB); err != nil {
		t.Fatalf("apply saveB profile: %v", err)
	}
	if _, err := os.Stat(filepath.Join(disabledModsDir(dir), "ModA")); err != nil {
		t.Fatalf("ModA should be disabled for saveB: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "ModB")); err != nil {
		t.Fatalf("ModB should be enabled for saveB: %v", err)
	}
}
