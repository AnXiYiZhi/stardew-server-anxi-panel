package stardew_junimo

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
)

func TestDetectModCompatibilityWarningsFlagsPreSVEIntroductions(t *testing.T) {
	dir := t.TempDir()
	saveName := "OldFarm_123"
	writeCompatibilitySave(t, dir, saveName, 28)
	mods := []registry.ModInfo{{UniqueID: sveContentPackUniqueID, Enabled: true}}

	warnings := DetectModCompatibilityWarnings(dir, saveName, mods)
	if len(warnings) != 1 || warnings[0].Code != "existing_save_world_overhaul_not_rebuilt" || warnings[0].SaveName != saveName {
		t.Fatalf("warnings = %+v", warnings)
	}
}

func TestDetectModCompatibilityWarningsAcceptsFreshSVEIntroductions(t *testing.T) {
	dir := t.TempDir()
	saveName := "FreshFarm_456"
	writeCompatibilitySave(t, dir, saveName, 32)
	mods := []registry.ModInfo{{UniqueID: sveContentPackUniqueID, Enabled: true}}

	if warnings := DetectModCompatibilityWarnings(dir, saveName, mods); len(warnings) != 0 {
		t.Fatalf("fresh SVE save warnings = %+v", warnings)
	}
}

func TestDetectModCompatibilityWarningsRequiresEnabledSVE(t *testing.T) {
	dir := t.TempDir()
	saveName := "VanillaFarm_789"
	writeCompatibilitySave(t, dir, saveName, 28)
	mods := []registry.ModInfo{{UniqueID: sveContentPackUniqueID, Enabled: false}}

	if warnings := DetectModCompatibilityWarnings(dir, saveName, mods); len(warnings) != 0 {
		t.Fatalf("disabled SVE warnings = %+v", warnings)
	}
}

func writeCompatibilitySave(t *testing.T, dataDir, saveName string, total int) {
	t.Helper()
	dir := filepath.Join(savesDir(dataDir), "Saves", saveName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	xml := `<?xml version="1.0"?><SaveGame><player><questLog><Quest xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="SocializeQuest"><questTitle>Introductions</questTitle><total>` +
		strconv.Itoa(total) + `</total></Quest></questLog></player></SaveGame>`
	if err := os.WriteFile(filepath.Join(dir, saveName), []byte(xml), 0o644); err != nil {
		t.Fatal(err)
	}
}
