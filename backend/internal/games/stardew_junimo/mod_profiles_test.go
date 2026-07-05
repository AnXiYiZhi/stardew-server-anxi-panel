package stardew_junimo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
)

func TestApplyNewSaveDefaultModStateDisablesNonBuiltInMods(t *testing.T) {
	dir := t.TempDir()
	root := modsDir(dir)
	createTestMod(t, root, controlModFolderName, controlModUniqueID, "Panel Control")
	createTestMod(t, root, junimoServerModFolderName, junimoServerModUniqueID, "JunimoServer")
	createTestMod(t, root, "ContentPatcher", "Pathoschild.ContentPatcher", "Content Patcher")

	if err := ApplyNewSaveDefaultModState(dir); err != nil {
		t.Fatalf("ApplyNewSaveDefaultModState: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, controlModFolderName)); err != nil {
		t.Fatalf("control mod should stay enabled: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, junimoServerModFolderName)); err != nil {
		t.Fatalf("JunimoServer mod should stay enabled: %v", err)
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

func TestApplyModProfileKeepsJunimoServerEnabledFromHistoricalProfile(t *testing.T) {
	dir := t.TempDir()
	root := modsDir(dir)
	saveName := "Farmer_12345"
	if err := os.MkdirAll(filepath.Join(savesDir(dir), "Saves", saveName), 0o755); err != nil {
		t.Fatalf("create save: %v", err)
	}
	createTestMod(t, root, junimoServerModFolderName, junimoServerModUniqueID, "JunimoServer")
	if err := EnsureDisabledModProfileForSave(dir, saveName); err != nil {
		t.Fatalf("EnsureDisabledModProfileForSave: %v", err)
	}
	if err := ApplyModProfile(dir, saveName); err != nil {
		t.Fatalf("ApplyModProfile: %v", err)
	}

	mods, err := ListModsWithState(dir, saveName)
	if err != nil {
		t.Fatalf("ListModsWithState: %v", err)
	}
	if len(mods) != 1 {
		t.Fatalf("expected JunimoServer only, got %+v", mods)
	}
	if !mods[0].Enabled || !mods[0].BuiltIn || mods[0].CanToggle {
		t.Fatalf("JunimoServer should stay enabled and built-in: %+v", mods[0])
	}
	if _, err := os.Stat(filepath.Join(root, junimoServerModFolderName)); err != nil {
		t.Fatalf("JunimoServer should remain in active mods dir: %v", err)
	}
}

func TestMarkImportedModsEnabledForSaveOverridesDisabledDefault(t *testing.T) {
	dir := t.TempDir()
	root := modsDir(dir)
	saveName := "Farmer_12345"
	if err := os.MkdirAll(filepath.Join(savesDir(dir), "Saves", saveName), 0o755); err != nil {
		t.Fatalf("create save: %v", err)
	}
	createTestMod(t, root, "OldMod", "author.old", "Old Mod")
	if err := EnsureDisabledModProfileForSave(dir, saveName); err != nil {
		t.Fatalf("EnsureDisabledModProfileForSave: %v", err)
	}
	if err := ApplyModProfile(dir, saveName); err != nil {
		t.Fatalf("ApplyModProfile: %v", err)
	}
	if _, err := os.Stat(filepath.Join(disabledModsDir(dir), "OldMod")); err != nil {
		t.Fatalf("old mod should be disabled: %v", err)
	}

	createTestMod(t, root, "SpaceCore", "spacechase0.SpaceCore", "SpaceCore")
	imported := []registry.ModInfo{readModInfo(filepath.Join(root, "SpaceCore"), "SpaceCore")}
	if err := MarkImportedModsEnabledForSave(dir, saveName, imported); err != nil {
		t.Fatalf("MarkImportedModsEnabledForSave: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "SpaceCore")); err != nil {
		t.Fatalf("newly imported mod should stay enabled: %v", err)
	}
	mods, err := ListModsWithState(dir, saveName)
	if err != nil {
		t.Fatalf("ListModsWithState: %v", err)
	}
	state := map[string]bool{}
	for _, mod := range mods {
		state[mod.FolderName] = mod.Enabled
	}
	if !state["SpaceCore"] || state["OldMod"] {
		t.Fatalf("unexpected mod states: %#v", state)
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

func TestSetModEnabledForSaveCascadeBundlesPackageAndKeepsSharedDependency(t *testing.T) {
	dir := t.TempDir()
	root := modsDir(dir)
	saveName := "Farmer_12345"
	if err := os.MkdirAll(filepath.Join(savesDir(dir), "Saves", saveName), 0o755); err != nil {
		t.Fatalf("create save: %v", err)
	}
	createTestMod(t, root, "ContentPatcher", "Pathoschild.ContentPatcher", "Content Patcher")
	createTestModWithManifest(t, root, "MultipleConstructionOrders", modManifest{
		Name:       "Multiple Construction Orders",
		UniqueID:   "moonslime.MultipleConstructionOrders",
		Version:    "1.0.0",
		Author:     "Test",
		UpdateKeys: []string{"Nexus:47289"},
	})
	createTestModWithManifest(t, root, "[CP] Multiple Construction Orders", modManifest{
		Name:     "[CP] Multiple Construction Orders",
		UniqueID: "moonslime.MultipleConstructionOrders.CP",
		Version:  "1.0.0",
		Author:   "Test",
		ContentPackFor: &modManifestDependency{
			UniqueID: "Pathoschild.ContentPatcher",
		},
		Dependencies: []modManifestDependency{
			{UniqueID: "moonslime.MultipleConstructionOrders"},
		},
	})
	imported, err := ListMods(dir)
	if err != nil {
		t.Fatalf("ListMods: %v", err)
	}
	nexusImported := make([]registry.ModInfo, 0, 2)
	for _, mod := range imported {
		if mod.FolderName == "MultipleConstructionOrders" || mod.FolderName == "[CP] Multiple Construction Orders" {
			nexusImported = append(nexusImported, mod)
		}
	}
	if err := SaveInstalledNexusMetadata(dir, nexusImported, NexusModSearchResult{ModID: 47289, Name: "Multiple Construction Orders"}); err != nil {
		t.Fatalf("SaveInstalledNexusMetadata: %v", err)
	}
	if err := EnsureDisabledModProfileForSave(dir, saveName); err != nil {
		t.Fatalf("EnsureDisabledModProfileForSave: %v", err)
	}
	if err := ApplyModProfile(dir, saveName); err != nil {
		t.Fatalf("ApplyModProfile: %v", err)
	}

	affected, err := SetModEnabledForSaveCascade(dir, saveName, "moonslime.MultipleConstructionOrders.CP", true)
	if err != nil {
		t.Fatalf("enable cascade: %v", err)
	}
	if got := modFoldersForTest(affected); strings.Join(got, ",") != "ContentPatcher,MultipleConstructionOrders,[CP] Multiple Construction Orders" {
		t.Fatalf("enable affected %v, want ContentPatcher/MCO/[CP]", got)
	}

	affected, err = SetModEnabledForSaveCascade(dir, saveName, "MultipleConstructionOrders", false)
	if err != nil {
		t.Fatalf("disable cascade: %v", err)
	}
	if got := modFoldersForTest(affected); strings.Join(got, ",") != "MultipleConstructionOrders,[CP] Multiple Construction Orders" {
		t.Fatalf("disable affected %v, want only MCO/[CP]", got)
	}
	mods, err := ListModsWithState(dir, saveName)
	if err != nil {
		t.Fatalf("ListModsWithState: %v", err)
	}
	state := map[string]bool{}
	for _, mod := range mods {
		state[mod.FolderName] = mod.Enabled
	}
	if !state["ContentPatcher"] {
		t.Fatalf("ContentPatcher should remain enabled as a shared dependency: %#v", state)
	}
	if state["MultipleConstructionOrders"] || state["[CP] Multiple Construction Orders"] {
		t.Fatalf("MCO package should be disabled together: %#v", state)
	}
}
