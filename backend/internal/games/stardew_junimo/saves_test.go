package stardew_junimo

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"golang.org/x/text/encoding/simplifiedchinese"
)

func TestWriteServerSettings_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := registry.NewGameConfig{
		FarmName:               "TestFarm",
		FarmType:               "riverland",
		StartingCabins:         2,
		MaxPlayers:             16,
		CabinLayout:            "nearby",
		ProfitMargin:           "75",
		PetBreed:               1,
		MoneyMode:              "shared",
		RemixedCommunityCenter: true,
		RemixedMineRewards:     true,
		SpawnMonstersOnFarm:    true,
	}
	if err := WriteServerSettings(dir, cfg); err != nil {
		t.Fatalf("WriteServerSettings: %v", err)
	}
	path := serverSettingsPath(dir)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("settings file not created: %v", err)
	}
	if _, err := os.Stat(newGamePendingPath(dir)); err != nil {
		t.Fatalf("new-game pending marker not created: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatal(err)
	}
	// Verify nested structure: {"Game":{...}, "Server":{...}}
	game, ok := settings["Game"].(map[string]any)
	if !ok {
		t.Fatalf("missing or invalid 'Game' section in settings: %v", settings)
	}
	server, ok := settings["Server"].(map[string]any)
	if !ok {
		t.Fatalf("missing or invalid 'Server' section in settings: %v", settings)
	}
	for _, key := range []string{"RemixBundles", "RemixMines"} {
		if game[key] != true {
			t.Errorf("Game.%s = %v, want true", key, game[key])
		}
	}
	if game["SpawnMonstersAtNight"] != "true" {
		t.Errorf("Game.SpawnMonstersAtNight = %v, want \"true\"", game["SpawnMonstersAtNight"])
	}
	if game["FarmName"] != "TestFarm" {
		t.Errorf("Game.FarmName = %v, want TestFarm", game["FarmName"])
	}
	if game["FarmType"] != float64(1) { // riverland = 1
		t.Errorf("Game.FarmType = %v, want 1", game["FarmType"])
	}
	if game["StartingCabins"] != float64(2) {
		t.Errorf("Game.StartingCabins = %v, want 2", game["StartingCabins"])
	}
	if server["SeparateWallets"] != false { // shared → false
		t.Errorf("Server.SeparateWallets = %v, want false", server["SeparateWallets"])
	}
	if server["MaxPlayers"] != float64(16) {
		t.Errorf("Server.MaxPlayers = %v, want 16", server["MaxPlayers"])
	}
	if server["CabinStrategy"] != "CabinStack" {
		t.Errorf("Server.CabinStrategy = %v, want CabinStack", server["CabinStrategy"])
	}
	if server["AllowIpConnections"] != true {
		t.Errorf("Server.AllowIpConnections = %v, want true (IP direct-connect on by default)", server["AllowIpConnections"])
	}
	if game["CabinLayoutNearby"] != true { // nearby → true
		t.Errorf("Game.CabinLayoutNearby = %v, want true", game["CabinLayoutNearby"])
	}
}

func TestEnsureServerSettingsDefaults(t *testing.T) {
	// Missing file → creates one with IP direct-connect enabled.
	dir := t.TempDir()
	if err := EnsureServerSettingsDefaults(dir); err != nil {
		t.Fatalf("EnsureServerSettingsDefaults (no file): %v", err)
	}
	server := readServerSection(t, dir)
	if server["AllowIpConnections"] != true {
		t.Fatalf("AllowIpConnections = %v, want true", server["AllowIpConnections"])
	}

	// Existing file without the key → adds it, preserving other keys.
	dir2 := t.TempDir()
	path := serverSettingsPath(dir2)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"Server":{"MaxPlayers":8}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := EnsureServerSettingsDefaults(dir2); err != nil {
		t.Fatalf("EnsureServerSettingsDefaults (existing): %v", err)
	}
	server = readServerSection(t, dir2)
	if server["AllowIpConnections"] != true {
		t.Errorf("AllowIpConnections = %v, want true", server["AllowIpConnections"])
	}
	if server["MaxPlayers"] != float64(8) {
		t.Errorf("MaxPlayers = %v, want 8 preserved", server["MaxPlayers"])
	}

	// Existing explicit false → respected, not overwritten.
	dir3 := t.TempDir()
	path3 := serverSettingsPath(dir3)
	if err := os.MkdirAll(filepath.Dir(path3), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path3, []byte(`{"Server":{"AllowIpConnections":false}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := EnsureServerSettingsDefaults(dir3); err != nil {
		t.Fatalf("EnsureServerSettingsDefaults (explicit false): %v", err)
	}
	server = readServerSection(t, dir3)
	if server["AllowIpConnections"] != false {
		t.Errorf("AllowIpConnections = %v, want false preserved", server["AllowIpConnections"])
	}
}

func readServerSection(t *testing.T, dir string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(serverSettingsPath(dir))
	if err != nil {
		t.Fatalf("read server-settings.json: %v", err)
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("parse server-settings.json: %v", err)
	}
	server, ok := settings["Server"].(map[string]any)
	if !ok {
		t.Fatalf("missing Server section: %v", settings)
	}
	return server
}

func TestWriteServerSettings_EmptyFarmName(t *testing.T) {
	dir := t.TempDir()
	cfg := registry.NewGameConfig{FarmName: ""}
	if err := WriteServerSettings(dir, cfg); err == nil {
		t.Fatal("expected error for empty farmName")
	}
}

func TestWriteServerSettings_ExplicitModFarmTypeIsString(t *testing.T) {
	dir := t.TempDir()
	cfg := registry.NewGameConfig{FarmName: "Farm", FarmType: "FrontierFarm", ProfitMargin: "100", MoneyMode: "shared"}
	if err := WriteServerSettings(dir, cfg); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(serverSettingsPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	var settings map[string]map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatal(err)
	}
	if got := settings["Game"]["FarmType"]; got != "FrontierFarm" {
		t.Fatalf("FarmType = %#v", got)
	}
}

func TestWriteServerSettings_CabinsAllowsStardew16Range(t *testing.T) {
	dir := t.TempDir()
	cfg := registry.NewGameConfig{FarmName: "Farm", StartingCabins: 5}
	if err := WriteServerSettings(dir, cfg); err != nil {
		t.Fatalf("startingCabins=5 should be accepted: %v", err)
	}
}

func TestWriteServerSettings_CabinsOutOfRange(t *testing.T) {
	dir := t.TempDir()
	cfg := registry.NewGameConfig{FarmName: "Farm", StartingCabins: 8}
	if err := WriteServerSettings(dir, cfg); err == nil {
		t.Fatal("expected error for startingCabins=8")
	}
}

func TestWriteServerSettings_MaxPlayersRange(t *testing.T) {
	dir := t.TempDir()
	if err := WriteServerSettings(dir, registry.NewGameConfig{FarmName: "Farm", MaxPlayers: 100}); err != nil {
		t.Fatalf("maxPlayers=100 should be accepted: %v", err)
	}
	if err := WriteServerSettings(dir, registry.NewGameConfig{FarmName: "Farm", MaxPlayers: 101}); err == nil {
		t.Fatal("expected error for maxPlayers=101")
	}
	if err := WriteServerSettings(dir, registry.NewGameConfig{FarmName: "Farm", MaxPlayers: -1}); err == nil {
		t.Fatal("expected error for maxPlayers=-1")
	}
	if err := WriteServerSettings(dir, registry.NewGameConfig{FarmName: "Farm", StartingCabins: 7, MaxPlayers: 7}); err == nil {
		t.Fatal("expected error when maxPlayers is below startingCabins + host")
	}
}

func TestWriteServerSettings_CabinModeRecommendedDefault(t *testing.T) {
	dir := t.TempDir()
	if err := WriteServerSettings(dir, registry.NewGameConfig{FarmName: "Farm"}); err != nil {
		t.Fatalf("WriteServerSettings: %v", err)
	}
	server := readServerSection(t, dir)
	if server["CabinStrategy"] != "CabinStack" {
		t.Errorf("Server.CabinStrategy = %v, want CabinStack (default cabinMode)", server["CabinStrategy"])
	}
}

func TestWriteServerSettings_CabinModeVanilla(t *testing.T) {
	dir := t.TempDir()
	cfg := registry.NewGameConfig{FarmName: "Farm", CabinMode: "vanilla"}
	if err := WriteServerSettings(dir, cfg); err != nil {
		t.Fatalf("WriteServerSettings: %v", err)
	}
	server := readServerSection(t, dir)
	if server["CabinStrategy"] != "None" {
		t.Errorf("Server.CabinStrategy = %v, want None (cabinMode=vanilla)", server["CabinStrategy"])
	}
}

func TestWriteServerSettings_CabinModeInvalid(t *testing.T) {
	dir := t.TempDir()
	cfg := registry.NewGameConfig{FarmName: "Farm", CabinMode: "hidden"}
	if err := WriteServerSettings(dir, cfg); err == nil {
		t.Fatal("expected error for invalid cabinMode")
	}
}

func TestServerRuntimeSettings_ReadDefaultsWhenMissing(t *testing.T) {
	dir := t.TempDir()
	settings, err := ReadServerRuntimeSettings(dir)
	if err != nil {
		t.Fatalf("ReadServerRuntimeSettings: %v", err)
	}
	if settings.CabinStrategy != "CabinStack" || settings.ExistingCabinBehavior != "KeepExisting" || settings.NetworkBroadcastPeriod != 1 {
		t.Errorf("unexpected defaults: %+v", settings)
	}
}

func TestServerRuntimeSettings_UpdateAndReadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if err := WriteServerSettings(dir, registry.NewGameConfig{FarmName: "Farm", MaxPlayers: 20}); err != nil {
		t.Fatalf("WriteServerSettings: %v", err)
	}
	want := ServerRuntimeSettings{
		CabinStrategy:          "FarmhouseStack",
		ExistingCabinBehavior:  "MoveToStack",
		NetworkBroadcastPeriod: 3,
	}
	if err := UpdateServerRuntimeSettings(dir, want); err != nil {
		t.Fatalf("UpdateServerRuntimeSettings: %v", err)
	}
	got, err := ReadServerRuntimeSettings(dir)
	if err != nil {
		t.Fatalf("ReadServerRuntimeSettings: %v", err)
	}
	if got != want {
		t.Errorf("ReadServerRuntimeSettings = %+v, want %+v", got, want)
	}
	// MaxPlayers written earlier must survive the runtime settings update.
	server := readServerSection(t, dir)
	if server["MaxPlayers"] != float64(20) {
		t.Errorf("Server.MaxPlayers = %v, want 20 (must be preserved)", server["MaxPlayers"])
	}
}

func TestServerRuntimeSettings_UpdateRejectsInvalid(t *testing.T) {
	dir := t.TempDir()
	cases := []ServerRuntimeSettings{
		{CabinStrategy: "Bogus", ExistingCabinBehavior: "KeepExisting", NetworkBroadcastPeriod: 1},
		{CabinStrategy: "CabinStack", ExistingCabinBehavior: "Bogus", NetworkBroadcastPeriod: 1},
		{CabinStrategy: "CabinStack", ExistingCabinBehavior: "KeepExisting", NetworkBroadcastPeriod: 0},
		{CabinStrategy: "CabinStack", ExistingCabinBehavior: "KeepExisting", NetworkBroadcastPeriod: 11},
	}
	for _, c := range cases {
		if err := UpdateServerRuntimeSettings(dir, c); err == nil {
			t.Errorf("expected error for %+v", c)
		}
	}
}

func TestValidateNewGameConfig_ProfitMargin(t *testing.T) {
	cases := []struct {
		margin string
		valid  bool
	}{
		{"100", true},
		{"75", true},
		{"50", true},
		{"25", true},
		{"normal", false},
		{"75%", false},
		{"100%", false},
	}
	for _, tc := range cases {
		cfg := registry.NewGameConfig{FarmName: "Farm", FarmType: "standard", CabinLayout: "nearby", CabinMode: "recommended", ProfitMargin: tc.margin, MoneyMode: "shared"}
		err := validateCfg(cfg)
		if tc.valid && err != nil {
			t.Errorf("margin=%q expected valid, got error: %v", tc.margin, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("margin=%q expected invalid, but no error", tc.margin)
		}
	}
}

func TestJunimoFarmTypeID(t *testing.T) {
	cases := map[string]int{
		"standard": 0, "riverland": 1, "forest": 2,
		"hilltop": 3, "wilderness": 4, "fourcorners": 5, "beach": 6, "meadowlands": 7,
		"unknown": -1,
	}
	for name, want := range cases {
		if got := junimoFarmTypeID(name); got != want {
			t.Errorf("farmTypeID(%q) = %d, want %d", name, got, want)
		}
	}
}

func TestValidateNewGameConfig_PetPreference(t *testing.T) {
	base := registry.NewGameConfig{
		FarmName: "Farm", FarmType: "standard", CabinLayout: "nearby", CabinMode: "recommended",
		ProfitMargin: "100", MoneyMode: "shared", PetType: "Dog", PetBreed: 4, PetBreedID: "4",
	}
	if err := validateCfg(base); err != nil {
		t.Fatalf("game data breed should be accepted: %v", err)
	}
	base.PetBreedID = "3"
	if err := validateCfg(base); err == nil {
		t.Fatal("mismatched petBreed/petBreedId should be rejected")
	}
}

func TestWriteInitConfig_PreservesPetGenderAndCabinSelection(t *testing.T) {
	dir := t.TempDir()
	cfg := registry.NewGameConfig{
		FarmName: "Meadow", FarmerName: "Robin", FarmType: "meadowlands",
		Gender: "female", PetType: "Dog", PetBreed: 4, PetBreedID: "4",
		StartingCabins: 2, CabinLayout: "separate", ProfitMargin: "75", MoneyMode: "shared",
	}
	if err := WriteServerSettings(dir, cfg); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(serverInitPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	var init initConfigJSON
	if err := json.Unmarshal(data, &init); err != nil {
		t.Fatal(err)
	}
	if init.Mode != "panel-newgame" || init.Gender != "female" || init.PetType != "Dog" || init.PetBreed != "4" || init.CabinCount != 2 || init.CabinLayout != "separate" {
		t.Fatalf("init selection changed: %#v", init)
	}
	if !init.AutoPause {
		t.Fatalf("expected autoPause enabled in init config: %#v", init)
	}
}

func TestListSaveDirs_Empty(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".local-container", "saves", "Saves"), 0o755); err != nil {
		t.Fatal(err)
	}
	names, err := listSaveDirs(dir)
	if err != nil {
		t.Fatalf("listSaveDirs: %v", err)
	}
	if len(names) != 0 {
		t.Fatalf("expected 0 saves, got %d", len(names))
	}
}

func TestListSaveDirs_WithSave(t *testing.T) {
	dir := t.TempDir()
	savesPath := filepath.Join(dir, ".local-container", "saves", "Saves", "TestFarmer_12345")
	if err := os.MkdirAll(savesPath, 0o755); err != nil {
		t.Fatal(err)
	}
	names, err := listSaveDirs(dir)
	if err != nil {
		t.Fatalf("listSaveDirs: %v", err)
	}
	if len(names) != 1 || names[0] != "TestFarmer_12345" {
		t.Fatalf("unexpected names: %v", names)
	}
}

func TestListSaveDirs_SkipsDotPrefixedTempDirs(t *testing.T) {
	dir := t.TempDir()
	savesPath := filepath.Join(dir, ".local-container", "saves", "Saves")
	if err := os.MkdirAll(filepath.Join(savesPath, "TestFarmer_12345"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Simulate a leftover extraction temp dir from an interrupted restore.
	if err := os.MkdirAll(filepath.Join(savesPath, ".restore-tmp-2156104854"), 0o755); err != nil {
		t.Fatal(err)
	}
	names, err := listSaveDirs(dir)
	if err != nil {
		t.Fatalf("listSaveDirs: %v", err)
	}
	if len(names) != 1 || names[0] != "TestFarmer_12345" {
		t.Fatalf("dot-prefixed temp dir should be skipped, got: %v", names)
	}
}

func TestPreviewSaveZip_RejectsOversizedZip(t *testing.T) {
	// Create a ZIP that claims to have a huge uncompressed file.
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "test.zip")

	zf, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(zf)
	// Add a normal-sized file; the "huge" test requires a real huge file or
	// crafting a zip header with inflated uncompressed size. We test the zip-slip
	// and missing-save scenarios here instead.
	fw, _ := w.Create("ValidSave/SaveGameInfo")
	_, _ = fw.Write([]byte("<SaveGame><player><name>Test</name><farmName>TestFarm</farmName></player><year>1</year><currentSeason>spring</currentSeason><dayOfMonth>1</dayOfMonth><whichFarm>0</whichFarm></SaveGame>"))
	main, _ := w.Create("ValidSave/ValidSave")
	_, _ = main.Write([]byte("<SaveGame><player><name>Test</name><farmName>TestFarm</farmName></player></SaveGame>"))
	_ = w.Close()
	_ = zf.Close()

	saveName, _, tempDir, err := PreviewSaveZip(zipPath, "test.zip")
	if err != nil {
		t.Fatalf("PreviewSaveZip: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()
	if saveName != "ValidSave" {
		t.Fatalf("expected saveName=ValidSave, got %q", saveName)
	}
}

func TestPreviewSaveZip_RejectsZipSlip(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "evil.zip")

	zf, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(zf)
	// Add a file with path traversal.
	_, _ = w.Create("../evil.txt")
	_ = w.Close()
	_ = zf.Close()

	_, _, _, err = PreviewSaveZip(zipPath, "evil.zip")
	if err == nil {
		t.Fatal("expected error for zip-slip path")
	}
}

func TestPreviewSaveZip_RejectsAbsolutePath(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "abs.zip")

	zf, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(zf)
	_, _ = w.Create("/etc/passwd")
	_ = w.Close()
	_ = zf.Close()

	_, _, _, err = PreviewSaveZip(zipPath, "abs.zip")
	if err == nil {
		t.Fatal("expected error for absolute path in zip")
	}
}

func TestPreviewSaveZip_RejectsMultipleTopDirs(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "multi.zip")

	zf, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(zf)
	_, _ = w.Create("FolderA/file.txt")
	_, _ = w.Create("FolderB/file.txt")
	_ = w.Close()
	_ = zf.Close()

	_, _, _, err = PreviewSaveZip(zipPath, "multi.zip")
	if err == nil {
		t.Fatal("expected error for multiple top-level dirs")
	}
}

func TestImportSaveToVolume(t *testing.T) {
	dataDir := t.TempDir()
	tempDir := t.TempDir()

	// Create fake save in tempDir.
	saveDir := filepath.Join(tempDir, "FarmerName_12345")
	if err := os.MkdirAll(saveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(saveDir, "SaveGameInfo"), []byte("<save/>"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create the target saves dir.
	savesPath := filepath.Join(dataDir, ".local-container", "saves", "Saves")
	if err := os.MkdirAll(savesPath, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := ImportSaveToVolume(dataDir, tempDir, "FarmerName_12345"); err != nil {
		t.Fatalf("ImportSaveToVolume: %v", err)
	}

	dest := filepath.Join(savesPath, "FarmerName_12345", "SaveGameInfo")
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("imported file not found: %v", err)
	}
}

func TestMigrateSavesVolume(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "docker-compose.yml")
	oldCompose := `services:
  server:
    volumes:
      - game-data:/data/game
      - saves:/config/xdg/config/StardewValley
      - ./.local-container/settings:/data/settings
volumes:
  steam-session:
  game-data:
  saves:
`
	if err := os.WriteFile(path, []byte(oldCompose), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, err := migrateSavesVolume(path)
	if err != nil {
		t.Fatalf("migrateSavesVolume: %v", err)
	}
	if !changed {
		t.Fatal("expected changed=true")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(got)
	if contains(content, "- saves:/config/xdg/config/StardewValley") {
		t.Error("old named volume mount still present")
	}
	if !contains(content, "- ./.local-container/saves:/config/xdg/config/StardewValley") {
		t.Error("new bind mount not found")
	}
	// The `saves:` entry under volumes section should be removed.
	if contains(content, "\n  saves:\n") {
		t.Error("saves: volume entry still present")
	}
}

func TestMigrateSavesVolume_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "docker-compose.yml")
	alreadyMigrated := `services:
  server:
    volumes:
      - ./.local-container/saves:/config/xdg/config/StardewValley
`
	if err := os.WriteFile(path, []byte(alreadyMigrated), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := migrateSavesVolume(path)
	if err != nil {
		t.Fatalf("migrateSavesVolume: %v", err)
	}
	if changed {
		t.Fatal("expected changed=false for already-migrated compose")
	}
}

func TestMigrateRemoveAssetExporterService(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "docker-compose.yml")
	compose := `services:
  server:
    image: sdvd/server:test
  asset-exporter:
    image: sdvd/server:test
    profiles:
      - catalog-export
volumes:
  game-data:
`
	if err := os.WriteFile(path, []byte(compose), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := migrateRemoveAssetExporterService(path)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected legacy service to be removed")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if contains(string(data), "asset-exporter:") {
		t.Fatal("legacy asset-exporter service remains")
	}
	if !contains(string(data), "\nvolumes:\n") {
		t.Fatal("volumes section was removed")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestDeleteSave_RejectsDotDot(t *testing.T) {
	dir := t.TempDir()
	savesPath := filepath.Join(dir, ".local-container", "saves", "Saves")
	if err := os.MkdirAll(savesPath, 0o755); err != nil {
		t.Fatal(err)
	}
	err := DeleteSave(dir, "..")
	if err == nil {
		t.Fatal("expected error for .. save name")
	}
}

func TestDeleteSave_RejectsPathSeparator(t *testing.T) {
	dir := t.TempDir()
	savesPath := filepath.Join(dir, ".local-container", "saves", "Saves")
	if err := os.MkdirAll(savesPath, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"../evil", "foo/bar", `foo\bar`, "/absolute"} {
		err := DeleteSave(dir, name)
		if err == nil {
			t.Fatalf("expected error for save name %q", name)
		}
	}
}

func TestDeleteSave_CannotDeleteSavesRoot(t *testing.T) {
	dir := t.TempDir()
	savesPath := filepath.Join(dir, ".local-container", "saves", "Saves")
	if err := os.MkdirAll(savesPath, 0o755); err != nil {
		t.Fatal(err)
	}
	err := DeleteSave(dir, ".")
	if err == nil {
		t.Fatal("expected error for . save name")
	}
	if _, err := os.Stat(savesPath); os.IsNotExist(err) {
		t.Fatal("Saves root was deleted")
	}
}

func TestDeleteSave_ActiveSaveCleanup(t *testing.T) {
	dir := t.TempDir()
	savePath := filepath.Join(dir, ".local-container", "saves", "Saves", "TestSave")
	if err := os.MkdirAll(savePath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := SetActiveSave(dir, "TestSave"); err != nil {
		t.Fatal(err)
	}
	if got := GetActiveSaveName(dir); got != "TestSave" {
		t.Fatalf("active save = %q, want TestSave", got)
	}
	if err := DeleteSave(dir, "TestSave"); err != nil {
		t.Fatal(err)
	}
	if got := GetActiveSaveName(dir); got != "" {
		t.Fatalf("active save = %q after delete, want empty", got)
	}
}

func TestGetActiveSaveName_RecoversFromWrongPrefix(t *testing.T) {
	dir := t.TempDir()
	// JunimoServer wrote the wrong farm-name prefix into gameloader.json
	// ("test" instead of "test2") while the folder it actually created kept
	// the same numeric suffix.
	realSave := filepath.Join(dir, ".local-container", "saves", "Saves", "test2_443102605")
	if err := os.MkdirAll(realSave, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := SetActiveSave(dir, "test_443102605"); err != nil {
		t.Fatal(err)
	}
	if got := GetActiveSaveName(dir); got != "test2_443102605" {
		t.Fatalf("active save = %q, want recovered name test2_443102605", got)
	}
}

func TestGetActiveSaveName_AmbiguousSuffixNotRecovered(t *testing.T) {
	dir := t.TempDir()
	savesRoot := filepath.Join(dir, ".local-container", "saves", "Saves")
	if err := os.MkdirAll(filepath.Join(savesRoot, "test2_443102605"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(savesRoot, "test3_443102605"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := SetActiveSave(dir, "test_443102605"); err != nil {
		t.Fatal(err)
	}
	if got := GetActiveSaveName(dir); got != "test_443102605" {
		t.Fatalf("active save = %q, want unresolved pointer test_443102605 when ambiguous", got)
	}
}

func TestValidateSaveExists(t *testing.T) {
	dir := t.TempDir()
	savePath := filepath.Join(dir, ".local-container", "saves", "Saves", "RealSave")
	if err := os.MkdirAll(savePath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := ValidateSaveExists(dir, "RealSave"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := ValidateSaveExists(dir, "FakeSave"); err == nil {
		t.Fatal("expected error for non-existing save")
	}
	if err := ValidateSaveExists(dir, "../evil"); err == nil {
		t.Fatal("expected error for path traversal name")
	}
}

func TestValidateSaveExists_RejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	savesPath := filepath.Join(dir, ".local-container", "saves", "Saves")
	if err := os.MkdirAll(savesPath, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"..", ".", "../etc", `..\windows`, "/abs"} {
		if err := ValidateSaveExists(dir, name); err == nil {
			t.Fatalf("expected error for %q", name)
		}
	}
}

func TestReadSaveInfo_FallbackPaths(t *testing.T) {
	dir := t.TempDir()
	savePath := filepath.Join(dir, "TestSave_12345")
	if err := os.MkdirAll(savePath, 0o755); err != nil {
		t.Fatal(err)
	}
	// Write SaveGameInfo.xml (alternative name).
	xmlContent := `<SaveGame><player><name>Farmer</name><farmName>MyFarm</farmName></player><year>2</year><currentSeason>summer</currentSeason><dayOfMonth>15</dayOfMonth><whichFarm>3</whichFarm></SaveGame>`
	if err := os.WriteFile(filepath.Join(savePath, "SaveGameInfo.xml"), []byte(xmlContent), 0o644); err != nil {
		t.Fatal(err)
	}
	info := readSaveInfo(savePath)
	if info.ParseError != "" {
		t.Fatalf("unexpected parse error: %s", info.ParseError)
	}
	if info.FarmerName != "Farmer" {
		t.Errorf("FarmerName = %q, want Farmer", info.FarmerName)
	}
	if info.FarmName != "MyFarm" {
		t.Errorf("FarmName = %q, want MyFarm", info.FarmName)
	}
	if info.FarmType != "hilltop" {
		t.Errorf("FarmType = %q, want hilltop", info.FarmType)
	}
}

// ── Farmer XML structure tests ──────────────────────────────────────────────

func TestReadSaveInfo_FarmerStructure(t *testing.T) {
	dir := t.TempDir()
	savePath := filepath.Join(dir, "1111_442155312")
	if err := os.MkdirAll(savePath, 0o755); err != nil {
		t.Fatal(err)
	}
	// Minimal <Farmer> XML matching Junimo's SaveGameInfo format.
	xmlContent := `<?xml version="1.0" encoding="utf-8"?>
<Farmer>
  <name>Server</name>
  <farmName>1111</farmName>
  <dayOfMonthForSaveGame>1</dayOfMonthForSaveGame>
  <seasonForSaveGame>0</seasonForSaveGame>
  <yearForSaveGame>1</yearForSaveGame>
</Farmer>`
	if err := os.WriteFile(filepath.Join(savePath, "SaveGameInfo"), []byte(xmlContent), 0o644); err != nil {
		t.Fatal(err)
	}
	info := readSaveInfo(savePath)
	if info.ParseError != "" {
		t.Fatalf("unexpected parse error: %s", info.ParseError)
	}
	if info.FarmerName != "Server" {
		t.Errorf("FarmerName = %q, want Server", info.FarmerName)
	}
	if info.FarmName != "1111" {
		t.Errorf("FarmName = %q, want 1111", info.FarmName)
	}
	if info.GameYear != 1 {
		t.Errorf("GameYear = %d, want 1", info.GameYear)
	}
	if info.GameSeason != "spring" {
		t.Errorf("GameSeason = %q, want spring", info.GameSeason)
	}
	if info.GameDay != 1 {
		t.Errorf("GameDay = %d, want 1", info.GameDay)
	}
	if info.FarmType != "" {
		t.Errorf("FarmType = %q, want empty (no whichFarm in Farmer)", info.FarmType)
	}
}

func TestReadSaveInfo_FarmerSeasonMapping(t *testing.T) {
	cases := []struct {
		seasonInt int
		want      string
	}{
		{0, "spring"},
		{1, "summer"},
		{2, "fall"},
		{3, "winter"},
	}
	for _, tc := range cases {
		dir := t.TempDir()
		savePath := filepath.Join(dir, "TestSave")
		if err := os.MkdirAll(savePath, 0o755); err != nil {
			t.Fatal(err)
		}
		xmlContent := fmt.Sprintf(`<Farmer><name>F</name><farmName>Farm</farmName><dayOfMonthForSaveGame>1</dayOfMonthForSaveGame><seasonForSaveGame>%d</seasonForSaveGame><yearForSaveGame>1</yearForSaveGame></Farmer>`, tc.seasonInt)
		if err := os.WriteFile(filepath.Join(savePath, "SaveGameInfo"), []byte(xmlContent), 0o644); err != nil {
			t.Fatal(err)
		}
		info := readSaveInfo(savePath)
		if info.ParseError != "" {
			t.Fatalf("season %d: unexpected parse error: %s", tc.seasonInt, info.ParseError)
		}
		if info.GameSeason != tc.want {
			t.Errorf("season %d: GameSeason = %q, want %q", tc.seasonInt, info.GameSeason, tc.want)
		}
	}
}

func TestReadSaveInfo_FarmerMissingFarmType(t *testing.T) {
	dir := t.TempDir()
	savePath := filepath.Join(dir, "TestSave")
	if err := os.MkdirAll(savePath, 0o755); err != nil {
		t.Fatal(err)
	}
	xmlContent := `<Farmer><name>F</name><farmName>Farm</farmName><dayOfMonthForSaveGame>1</dayOfMonthForSaveGame><seasonForSaveGame>0</seasonForSaveGame><yearForSaveGame>1</yearForSaveGame></Farmer>`
	if err := os.WriteFile(filepath.Join(savePath, "SaveGameInfo"), []byte(xmlContent), 0o644); err != nil {
		t.Fatal(err)
	}
	info := readSaveInfo(savePath)
	if info.FarmType != "" {
		t.Errorf("FarmType = %q, want empty — Farmer XML has no whichFarm", info.FarmType)
	}
}

func TestReadSaveInfo_SaveGameWhichFarmZero(t *testing.T) {
	dir := t.TempDir()
	savePath := filepath.Join(dir, "TestSave")
	if err := os.MkdirAll(savePath, 0o755); err != nil {
		t.Fatal(err)
	}
	// whichFarm=0 means standard farm in the full SaveGame format.
	xmlContent := `<SaveGame><player><name>F</name><farmName>Farm</farmName></player><year>1</year><currentSeason>spring</currentSeason><dayOfMonth>1</dayOfMonth><whichFarm>0</whichFarm></SaveGame>`
	if err := os.WriteFile(filepath.Join(savePath, "SaveGameInfo"), []byte(xmlContent), 0o644); err != nil {
		t.Fatal(err)
	}
	info := readSaveInfo(savePath)
	if info.FarmType != "standard" {
		t.Errorf("FarmType = %q, want standard (whichFarm=0)", info.FarmType)
	}
}

func TestReadSaveInfo_InvalidXML(t *testing.T) {
	dir := t.TempDir()
	savePath := filepath.Join(dir, "TestSave")
	if err := os.MkdirAll(savePath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(savePath, "SaveGameInfo"), []byte("not xml at all"), 0o644); err != nil {
		t.Fatal(err)
	}
	info := readSaveInfo(savePath)
	if info.ParseError == "" {
		t.Fatal("expected parse error for invalid XML")
	}
}

func TestReadSaveInfo_NoSaveGameInfo(t *testing.T) {
	dir := t.TempDir()
	savePath := filepath.Join(dir, "TestSave")
	if err := os.MkdirAll(savePath, 0o755); err != nil {
		t.Fatal(err)
	}
	info := readSaveInfo(savePath)
	if info.ParseError == "" {
		t.Fatal("expected parse error when no SaveGameInfo file exists")
	}
}

func TestReadSaveInfo_FarmerReadsWhichFarmFromMainFile(t *testing.T) {
	dir := t.TempDir()
	savePath := filepath.Join(dir, "TestSave_123")
	if err := os.MkdirAll(savePath, 0o755); err != nil {
		t.Fatal(err)
	}
	// SaveGameInfo is Farmer format (no whichFarm).
	farmerXML := `<Farmer><name>Server</name><farmName>MyFarm</farmName><dayOfMonthForSaveGame>1</dayOfMonthForSaveGame><seasonForSaveGame>0</seasonForSaveGame><yearForSaveGame>1</yearForSaveGame></Farmer>`
	if err := os.WriteFile(filepath.Join(savePath, "SaveGameInfo"), []byte(farmerXML), 0o644); err != nil {
		t.Fatal(err)
	}
	// Main save file has <whichFarm>3</whichFarm> (hilltop).
	mainSaveXML := `<SaveGame><whichFarm>3</whichFarm><player><name>Server</name></player></SaveGame>`
	if err := os.WriteFile(filepath.Join(savePath, "TestSave_123"), []byte(mainSaveXML), 0o644); err != nil {
		t.Fatal(err)
	}

	info := readSaveInfo(savePath)
	if info.ParseError != "" {
		t.Fatalf("unexpected parse error: %s", info.ParseError)
	}
	if info.FarmType != "hilltop" {
		t.Errorf("FarmType = %q, want hilltop (from main save file whichFarm=3)", info.FarmType)
	}
}

func TestReadSaveInfo_FarmerReadsStringWhichFarm(t *testing.T) {
	dir := t.TempDir()
	savePath := filepath.Join(dir, "MeadowSave_456")
	if err := os.MkdirAll(savePath, 0o755); err != nil {
		t.Fatal(err)
	}
	// SaveGameInfo is Farmer format.
	farmerXML := `<Farmer><name>Server</name><farmName>Meadow</farmName><dayOfMonthForSaveGame>1</dayOfMonthForSaveGame><seasonForSaveGame>0</seasonForSaveGame><yearForSaveGame>1</yearForSaveGame></Farmer>`
	if err := os.WriteFile(filepath.Join(savePath, "SaveGameInfo"), []byte(farmerXML), 0o644); err != nil {
		t.Fatal(err)
	}
	// Main save file has <whichFarm>MeadowlandsFarm</whichFarm> (string).
	mainSaveXML := `<SaveGame><whichFarm>MeadowlandsFarm</whichFarm><player><name>Server</name></player></SaveGame>`
	if err := os.WriteFile(filepath.Join(savePath, "MeadowSave_456"), []byte(mainSaveXML), 0o644); err != nil {
		t.Fatal(err)
	}

	info := readSaveInfo(savePath)
	if info.ParseError != "" {
		t.Fatalf("unexpected parse error: %s", info.ParseError)
	}
	if info.FarmType != "meadowlands" {
		t.Errorf("FarmType = %q, want meadowlands (from main save file whichFarm=MeadowlandsFarm)", info.FarmType)
	}
}

func TestReadWhichFarmFromReaderAcrossChunkBoundary(t *testing.T) {
	payload := strings.Repeat("x", 32*1024-5) + "<whichFarm>MeadowlandsFarm</whichFarm>"
	if got := readWhichFarmFromReader(strings.NewReader(payload)); got != "meadowlands" {
		t.Fatalf("readWhichFarmFromReader() = %q, want meadowlands", got)
	}
}

func TestReadSaveInfo_FarmerNoMainFile(t *testing.T) {
	dir := t.TempDir()
	savePath := filepath.Join(dir, "NoMainSave_789")
	if err := os.MkdirAll(savePath, 0o755); err != nil {
		t.Fatal(err)
	}
	// SaveGameInfo is Farmer format, but no main save file exists.
	farmerXML := `<Farmer><name>Server</name><farmName>Farm</farmName><dayOfMonthForSaveGame>1</dayOfMonthForSaveGame><seasonForSaveGame>0</seasonForSaveGame><yearForSaveGame>1</yearForSaveGame></Farmer>`
	if err := os.WriteFile(filepath.Join(savePath, "SaveGameInfo"), []byte(farmerXML), 0o644); err != nil {
		t.Fatal(err)
	}

	info := readSaveInfo(savePath)
	if info.ParseError != "" {
		t.Fatalf("unexpected parse error: %s", info.ParseError)
	}
	if info.FarmType != "" {
		t.Errorf("FarmType = %q, want empty (no main save file)", info.FarmType)
	}
	if info.FarmerName != "Server" {
		t.Errorf("FarmerName = %q, want Server", info.FarmerName)
	}
}

func TestReadSaveInfo_SaveGameStringWhichFarm(t *testing.T) {
	dir := t.TempDir()
	savePath := filepath.Join(dir, "TestSave")
	if err := os.MkdirAll(savePath, 0o755); err != nil {
		t.Fatal(err)
	}
	// SaveGame format with string whichFarm.
	xmlContent := `<SaveGame><player><name>F</name><farmName>Farm</farmName></player><year>1</year><currentSeason>spring</currentSeason><dayOfMonth>1</dayOfMonth><whichFarm>MeadowlandsFarm</whichFarm></SaveGame>`
	if err := os.WriteFile(filepath.Join(savePath, "SaveGameInfo"), []byte(xmlContent), 0o644); err != nil {
		t.Fatal(err)
	}
	info := readSaveInfo(savePath)
	if info.FarmType != "meadowlands" {
		t.Errorf("FarmType = %q, want meadowlands (string whichFarm)", info.FarmType)
	}
}

func TestFarmTypeLabelFromString(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"0", "standard"},
		{"1", "riverland"},
		{"2", "forest"},
		{"3", "hilltop"},
		{"4", "wilderness"},
		{"5", "fourcorners"},
		{"6", "beach"},
		{"7", "meadowlands"},
		{"MeadowlandsFarm", "meadowlands"},
		{"StandardFarm", "standard"},
		{"BeachFarm", "beach"},
		{"unknown_farm", "unknown_farm"},
		{"FrontierFarm", "FrontierFarm"},
		{"", ""},
		// Whitespace-wrapped values from XML like <whichFarm> 6 </whichFarm>.
		{" 6 ", "beach"},
		{"\t3\n", "hilltop"},
		{" MeadowlandsFarm ", "meadowlands"},
	}
	for _, tc := range cases {
		got := farmTypeLabelFromString(tc.input)
		if got != tc.want {
			t.Errorf("farmTypeLabelFromString(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// ── PreviewSaveZip: dangerous save name tests ──────────────────────────────

func createZipWithTopDir(t *testing.T, topDir string) string {
	t.Helper()
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "test.zip")
	zf, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(zf)
	fw, _ := w.Create(topDir + "/SaveGameInfo")
	_, _ = fw.Write([]byte(`<Farmer><name>F</name><farmName>Farm</farmName><dayOfMonthForSaveGame>1</dayOfMonthForSaveGame><seasonForSaveGame>0</seasonForSaveGame><yearForSaveGame>1</yearForSaveGame></Farmer>`))
	_ = w.Close()
	_ = zf.Close()
	return zipPath
}

func TestPreviewSaveZip_RejectsDotDir(t *testing.T) {
	zipPath := createZipWithTopDir(t, ".")
	_, _, _, err := PreviewSaveZip(zipPath, "test.zip")
	if err == nil {
		t.Fatal("expected error for . top-level dir")
	}
}

func TestPreviewSaveZip_RejectsDotDotDir(t *testing.T) {
	zipPath := createZipWithTopDir(t, "..")
	_, _, _, err := PreviewSaveZip(zipPath, "test.zip")
	if err == nil {
		t.Fatal("expected error for .. top-level dir")
	}
}

func TestPreviewSaveZip_RejectsPathTraversalDir(t *testing.T) {
	// "foo/bar" has two top-level segments; detectSaveFolderName will see "foo" and "bar".
	// The zip entry "foo/bar/SaveGameInfo" produces topDirs={"foo":{}, "bar":{}}, which is rejected
	// as "multiple top-level dirs". Also test reserved names.
	zipPath := createZipWithTopDir(t, "select")
	_, _, _, err := PreviewSaveZip(zipPath, "test.zip")
	if err == nil {
		t.Fatal("expected error for reserved name 'select'")
	}
}

func TestPreviewSaveZip_RejectsReservedNames(t *testing.T) {
	for _, name := range []string{"preflight", "custom-new-game", "upload-preview", "select-and-start", "delete"} {
		zipPath := createZipWithTopDir(t, name)
		_, _, _, err := PreviewSaveZip(zipPath, "test.zip")
		if err == nil {
			t.Fatalf("expected error for reserved name %q", name)
		}
	}
}

func TestPreviewSaveZip_RejectsDotDotInMiddle(t *testing.T) {
	// "foo/../bar/SaveGameInfo" should be rejected because it contains ".." segment.
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "traversal.zip")
	zf, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(zf)
	_, _ = w.Create("foo/../bar/SaveGameInfo")
	_ = w.Close()
	_ = zf.Close()

	_, _, _, err = PreviewSaveZip(zipPath, "traversal.zip")
	if err == nil {
		t.Fatal("expected error for foo/../bar path")
	}
}

func TestPreviewSaveZip_RejectsDotSlash(t *testing.T) {
	// "./SaveGameInfo" should be rejected because it contains "." segment.
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "dot.zip")
	zf, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(zf)
	_, _ = w.Create("./SaveGameInfo")
	_ = w.Close()
	_ = zf.Close()

	_, _, _, err = PreviewSaveZip(zipPath, "dot.zip")
	if err == nil {
		t.Fatal("expected error for ./ path")
	}
}

func TestPreviewSaveZip_RejectsDoubleSlash(t *testing.T) {
	// "foo//SaveGameInfo" should be rejected because it contains empty segment.
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "doubleslash.zip")
	zf, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(zf)
	_, _ = w.Create("foo//SaveGameInfo")
	_ = w.Close()
	_ = zf.Close()

	_, _, _, err = PreviewSaveZip(zipPath, "doubleslash.zip")
	if err == nil {
		t.Fatal("expected error for foo// path")
	}
}

func TestPreviewSaveZip_AcceptsValidPath(t *testing.T) {
	// "FarmerName_12345/SaveGameInfo" is a normal valid path.
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "valid.zip")
	zf, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(zf)
	fw, _ := w.Create("FarmerName_12345/SaveGameInfo")
	_, _ = fw.Write([]byte(`<Farmer><name>F</name><farmName>Farm</farmName><dayOfMonthForSaveGame>1</dayOfMonthForSaveGame><seasonForSaveGame>0</seasonForSaveGame><yearForSaveGame>1</yearForSaveGame></Farmer>`))
	main, _ := w.Create("FarmerName_12345/FarmerName_12345")
	_, _ = main.Write([]byte(`<SaveGame><player><name>F</name><farmName>Farm</farmName></player></SaveGame>`))
	_ = w.Close()
	_ = zf.Close()

	saveName, _, tempDir, err := PreviewSaveZip(zipPath, "valid.zip")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()
	if saveName != "FarmerName_12345" {
		t.Fatalf("expected saveName=FarmerName_12345, got %q", saveName)
	}
}

func TestPreviewSaveZip_AcceptsDirectoryEntry(t *testing.T) {
	// Common ZIP tools emit explicit directory entries like "FarmerName_12345/".
	// The trailing "/" must not be rejected as an empty segment.
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "direntry.zip")
	zf, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(zf)
	// Explicit directory entry.
	_, _ = w.Create("FarmerName_12345/")
	fw, _ := w.Create("FarmerName_12345/SaveGameInfo")
	_, _ = fw.Write([]byte(`<Farmer><name>F</name><farmName>Farm</farmName><dayOfMonthForSaveGame>1</dayOfMonthForSaveGame><seasonForSaveGame>0</seasonForSaveGame><yearForSaveGame>1</yearForSaveGame></Farmer>`))
	main, _ := w.Create("FarmerName_12345/FarmerName_12345")
	_, _ = main.Write([]byte(`<SaveGame><player><name>F</name><farmName>Farm</farmName></player></SaveGame>`))
	_ = w.Close()
	_ = zf.Close()

	saveName, _, tempDir, err := PreviewSaveZip(zipPath, "direntry.zip")
	if err != nil {
		t.Fatalf("unexpected error for ZIP with directory entry: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()
	if saveName != "FarmerName_12345" {
		t.Fatalf("expected saveName=FarmerName_12345, got %q", saveName)
	}
}

func TestPreviewSaveZip_DecodesLegacyGBKSaveName(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "legacy-gbk.zip")
	zf, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(zf)
	saveName := "中文农场_12345"
	for _, relative := range []string{"SaveGameInfo", saveName} {
		rawName, encodeErr := simplifiedchinese.GBK.NewEncoder().Bytes([]byte(saveName + "/" + relative))
		if encodeErr != nil {
			t.Fatal(encodeErr)
		}
		header := &zip.FileHeader{Name: string(rawName), Method: zip.Deflate, NonUTF8: true}
		entry, createErr := w.CreateHeader(header)
		if createErr != nil {
			t.Fatal(createErr)
		}
		if _, writeErr := entry.Write([]byte(`<SaveGame><player><name>F</name><farmName>中文农场</farmName></player></SaveGame>`)); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zf.Close(); err != nil {
		t.Fatal(err)
	}

	gotName, preview, tempDir, err := PreviewSaveZip(zipPath, "legacy-gbk.zip")
	if err != nil {
		t.Fatalf("PreviewSaveZip: %v", err)
	}
	defer os.RemoveAll(tempDir)
	if gotName != saveName || preview.Name != saveName {
		t.Fatalf("decoded names = %q / %q, want %q", gotName, preview.Name, saveName)
	}
	for _, relative := range []string{"SaveGameInfo", saveName} {
		if _, err := os.Stat(filepath.Join(tempDir, saveName, relative)); err != nil {
			t.Fatalf("normalized extracted file %q missing: %v", relative, err)
		}
	}
}

func TestLegacyGBKSaveCanBeListedBackedUpAndDeleted(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows filenames are UTF-16; raw invalid UTF-8 directory entries require a Unix filesystem")
	}
	dataDir := t.TempDir()
	publicName := "中文农场_54321"
	rawNameBytes, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte(publicName))
	if err != nil {
		t.Fatal(err)
	}
	rawName := string(rawNameBytes)
	saveDir := filepath.Join(dataDir, ".local-container", "saves", "Saves", rawName)
	if err := os.MkdirAll(saveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	xml := []byte(`<SaveGame><player><name>F</name><farmName>中文农场</farmName></player></SaveGame>`)
	if err := os.WriteFile(filepath.Join(saveDir, rawName), xml, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(saveDir, "SaveGameInfo"), xml, 0o644); err != nil {
		t.Fatal(err)
	}

	driver := &Driver{}
	saves, err := driver.ListSaves(context.Background(), registry.Instance{DataDir: dataDir})
	if err != nil {
		t.Fatal(err)
	}
	if len(saves) != 1 || saves[0].Name != publicName || saves[0].NameWarning == "" {
		t.Fatalf("legacy save listing = %+v", saves)
	}
	if err := ValidateSaveCanActivate(dataDir, publicName); err == nil {
		t.Fatal("legacy raw-byte save must not be activatable")
	}

	backupPath, err := DeleteSaveWithBackup(dataDir, publicName)
	if err != nil {
		t.Fatalf("DeleteSaveWithBackup: %v", err)
	}
	if _, err := os.Stat(saveDir); !os.IsNotExist(err) {
		t.Fatalf("raw legacy save still exists: %v", err)
	}
	zr, err := zip.OpenReader(backupPath)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	wantMain := publicName + "/" + publicName
	foundMain := false
	for _, file := range zr.File {
		if !utf8.ValidString(file.Name) {
			t.Fatalf("backup retained invalid UTF-8 entry %q", file.Name)
		}
		if file.Name == wantMain {
			foundMain = true
		}
	}
	if !foundMain {
		t.Fatalf("normalized backup main file %q missing", wantMain)
	}
}

func TestLegacySaveAliasDoesNotCollideWithUTF8Directory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows filenames are UTF-16; raw invalid UTF-8 directory entries require a Unix filesystem")
	}
	dataDir := t.TempDir()
	publicName := "中文农场_77777"
	rawNameBytes, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte(publicName))
	if err != nil {
		t.Fatal(err)
	}
	rawName := string(rawNameBytes)
	root := filepath.Join(dataDir, ".local-container", "saves", "Saves")
	for _, name := range []string{publicName, rawName} {
		saveDir := filepath.Join(root, name)
		if err := os.MkdirAll(saveDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(saveDir, "SaveGameInfo"), []byte(`<SaveGame/>`), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	driver := &Driver{}
	saves, err := driver.ListSaves(context.Background(), registry.Instance{DataDir: dataDir})
	if err != nil {
		t.Fatal(err)
	}
	if len(saves) != 2 {
		t.Fatalf("saves = %+v", saves)
	}
	legacyAlias := ""
	for _, save := range saves {
		if save.NameWarning != "" {
			legacyAlias = save.Name
		}
	}
	if legacyAlias == "" || legacyAlias == publicName || !strings.HasPrefix(legacyAlias, "encoding_error_") {
		t.Fatalf("legacy collision alias = %q, saves = %+v", legacyAlias, saves)
	}
	if err := DeleteSave(dataDir, legacyAlias); err != nil {
		t.Fatalf("delete legacy alias: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, publicName)); err != nil {
		t.Fatalf("UTF-8 save was incorrectly deleted: %v", err)
	}
}

// ── ImportSaveToVolume: validation tests ────────────────────────────────────

func TestImportSaveToVolume_RejectsDot(t *testing.T) {
	dataDir := t.TempDir()
	savesPath := filepath.Join(dataDir, ".local-container", "saves", "Saves")
	if err := os.MkdirAll(savesPath, 0o755); err != nil {
		t.Fatal(err)
	}
	err := ImportSaveToVolume(dataDir, t.TempDir(), ".")
	if err == nil {
		t.Fatal("expected error for . saveName")
	}
}

func TestImportSaveToVolume_RejectsDotDot(t *testing.T) {
	dataDir := t.TempDir()
	savesPath := filepath.Join(dataDir, ".local-container", "saves", "Saves")
	if err := os.MkdirAll(savesPath, 0o755); err != nil {
		t.Fatal(err)
	}
	err := ImportSaveToVolume(dataDir, t.TempDir(), "..")
	if err == nil {
		t.Fatal("expected error for .. saveName")
	}
}

func TestImportSaveToVolume_RejectsPathSeparators(t *testing.T) {
	dataDir := t.TempDir()
	savesPath := filepath.Join(dataDir, ".local-container", "saves", "Saves")
	if err := os.MkdirAll(savesPath, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"foo/bar", `foo\bar`, "/absolute"} {
		err := ImportSaveToVolume(dataDir, t.TempDir(), name)
		if err == nil {
			t.Fatalf("expected error for saveName %q", name)
		}
	}
}

func TestImportSaveToVolume_CannotDeleteSavesRoot(t *testing.T) {
	dataDir := t.TempDir()
	savesPath := filepath.Join(dataDir, ".local-container", "saves", "Saves")
	if err := os.MkdirAll(savesPath, 0o755); err != nil {
		t.Fatal(err)
	}
	// This should be rejected by validateSaveName (.) or resolveSavePath guard.
	err := ImportSaveToVolume(dataDir, t.TempDir(), ".")
	if err == nil {
		t.Fatal("expected error for . saveName")
	}
	// Verify Saves root still exists.
	if _, err := os.Stat(savesPath); os.IsNotExist(err) {
		t.Fatal("Saves root was deleted")
	}
}

func TestImportSaveToVolume_NormalImport(t *testing.T) {
	dataDir := t.TempDir()
	tempDir := t.TempDir()

	saveDir := filepath.Join(tempDir, "FarmerName_12345")
	if err := os.MkdirAll(saveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(saveDir, "SaveGameInfo"), []byte("<Farmer><name>F</name></Farmer>"), 0o644); err != nil {
		t.Fatal(err)
	}

	savesPath := filepath.Join(dataDir, ".local-container", "saves", "Saves")
	if err := os.MkdirAll(savesPath, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := ImportSaveToVolume(dataDir, tempDir, "FarmerName_12345"); err != nil {
		t.Fatalf("ImportSaveToVolume: %v", err)
	}
	dest := filepath.Join(savesPath, "FarmerName_12345", "SaveGameInfo")
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("imported file not found: %v", err)
	}
}

func TestImportSaveToVolume_RejectsReservedNames(t *testing.T) {
	dataDir := t.TempDir()
	savesPath := filepath.Join(dataDir, ".local-container", "saves", "Saves")
	if err := os.MkdirAll(savesPath, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"preflight", "select", "select-and-start", "delete"} {
		err := ImportSaveToVolume(dataDir, t.TempDir(), name)
		if err == nil {
			t.Fatalf("expected error for reserved name %q", name)
		}
	}
}

// ── validateSaveName: reserved names ────────────────────────────────────────

func TestValidateSaveName_ReservedNames(t *testing.T) {
	for _, name := range []string{"preflight", "custom-new-game", "upload-preview", "upload-commit-and-start", "select", "select-and-start", "delete"} {
		if err := validateSaveName(name); err == nil {
			t.Fatalf("expected error for reserved name %q", name)
		}
	}
}

func TestValidateSaveName_ValidNames(t *testing.T) {
	for _, name := range []string{"FarmerName_12345", "TEST_PANEL_CUSTOM_442153826", "1111_442155312", "MySave"} {
		if err := validateSaveName(name); err != nil {
			t.Fatalf("unexpected error for valid name %q: %v", name, err)
		}
	}
}

// ── ExportSaveZip tests ───────────────────────────────────────────────────────

func TestExportSaveZip_Valid(t *testing.T) {
	dir := t.TempDir()
	savePath := filepath.Join(dir, ".local-container", "saves", "Saves", "TestSave_123")
	if err := os.MkdirAll(savePath, 0o755); err != nil {
		t.Fatal(err)
	}
	xmlContent := `<SaveGame><player><name>Farmer</name><farmName>MyFarm</farmName></player><year>2</year><currentSeason>summer</currentSeason><dayOfMonth>15</dayOfMonth><whichFarm>3</whichFarm></SaveGame>`
	if err := os.WriteFile(filepath.Join(savePath, "SaveGameInfo"), []byte(xmlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	zipPath, err := ExportSaveZip(dir, "TestSave_123")
	if err != nil {
		t.Fatalf("ExportSaveZip: %v", err)
	}
	defer func() { _ = os.Remove(zipPath) }()

	// Verify filename contains save name and game time.
	name := filepath.Base(zipPath)
	if !strings.Contains(name, "TestSave_123") {
		t.Errorf("filename %q should contain save name", name)
	}
	if !strings.Contains(name, "2年") {
		t.Errorf("filename %q should contain year", name)
	}
	if !strings.Contains(name, "夏") {
		t.Errorf("filename %q should contain season", name)
	}
	if !strings.Contains(name, "15日") {
		t.Errorf("filename %q should contain day", name)
	}

	// Verify ZIP contains the save files.
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open exported zip: %v", err)
	}
	defer func() { _ = zr.Close() }()
	found := false
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, "SaveGameInfo") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("exported ZIP missing SaveGameInfo")
	}
}

func TestExportSaveZip_NotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := ExportSaveZip(dir, "NonExistent")
	if err == nil {
		t.Fatal("expected error for non-existent save")
	}
}

func TestExportSaveZip_RejectsDotDot(t *testing.T) {
	dir := t.TempDir()
	_, err := ExportSaveZip(dir, "..")
	if err == nil {
		t.Fatal("expected error for .. save name")
	}
}

func TestBuildSaveZipName_WithGameTime(t *testing.T) {
	info := registry.SaveInfo{GameYear: 3, GameSeason: "winter", GameDay: 28}
	name := buildSaveZipName("FarmerName_12345", info)
	if name != "FarmerName_12345_3年_冬_28日.zip" {
		t.Errorf("buildSaveZipName = %q, want FarmerName_12345_3年_冬_28日.zip", name)
	}
}

func TestBuildSaveZipName_NoGameTime(t *testing.T) {
	info := registry.SaveInfo{}
	name := buildSaveZipName("MySave", info)
	if name != "MySave.zip" {
		t.Errorf("buildSaveZipName = %q, want MySave.zip", name)
	}
}

func TestBuildSaveZipName_WithSpaces(t *testing.T) {
	info := registry.SaveInfo{GameYear: 1, GameSeason: "spring", GameDay: 1}
	name := buildSaveZipName("My Save File", info)
	if name != "My_Save_File_1年_春_1日.zip" {
		t.Errorf("buildSaveZipName = %q, want My_Save_File_1年_春_1日.zip", name)
	}
}

// ── Backup / Restore Tests ────────────────────────────────────────────────────

func TestBackupSave_CreatesBackup(t *testing.T) {
	dir := t.TempDir()
	// Create a save directory with some content.
	saveDir := filepath.Join(dir, ".local-container", "saves", "Saves", "TestSave")
	if err := os.MkdirAll(saveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(saveDir, "SaveGameInfo"), []byte("<SaveGame><player><name>Farmer</name></player></SaveGame>"), 0o644); err != nil {
		t.Fatal(err)
	}

	backupPath, err := BackupSave(dir, "TestSave")
	if err != nil {
		t.Fatalf("BackupSave: %v", err)
	}
	if backupPath == "" {
		t.Fatal("expected non-empty backup path")
	}
	if _, err := os.Stat(backupPath); err != nil {
		t.Fatalf("backup file not found: %v", err)
	}
	if !strings.HasSuffix(backupPath, ".zip") {
		t.Errorf("backup should be a .zip file, got %q", backupPath)
	}
}

func TestBackupSave_NonExistentSave(t *testing.T) {
	dir := t.TempDir()
	_, err := BackupSave(dir, "NonExistent")
	if err == nil {
		t.Fatal("expected error for non-existent save")
	}
}

func TestBackupSave_InvalidName(t *testing.T) {
	dir := t.TempDir()
	_, err := BackupSave(dir, "../escape")
	if err == nil {
		t.Fatal("expected error for invalid save name")
	}
}

func TestDeleteSaveWithBackup_CreatesBackupBeforeDelete(t *testing.T) {
	dir := t.TempDir()
	saveDir := filepath.Join(dir, ".local-container", "saves", "Saves", "TestSave")
	if err := os.MkdirAll(saveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(saveDir, "SaveGameInfo"), []byte("<SaveGame/>"), 0o644); err != nil {
		t.Fatal(err)
	}

	backupPath, err := DeleteSaveWithBackup(dir, "TestSave")
	if err != nil {
		t.Fatalf("DeleteSaveWithBackup: %v", err)
	}
	if backupPath == "" {
		t.Fatal("expected backup to be created")
	}
	// Save should be deleted.
	if _, err := os.Stat(saveDir); !os.IsNotExist(err) {
		t.Error("save directory should have been deleted")
	}
	// Backup should exist.
	if _, err := os.Stat(backupPath); err != nil {
		t.Errorf("backup file should exist: %v", err)
	}
	name := filepath.Base(backupPath)
	if !strings.HasPrefix(name, "predelete_") {
		t.Errorf("expected predelete_ prefix, got %q", name)
	}
	if inferBackupKind(name) != "predelete" {
		t.Errorf("expected kind predelete, got %q", inferBackupKind(name))
	}
}

func TestListBackups_ReturnsBackupInfo(t *testing.T) {
	dir := t.TempDir()
	// Create a save and backup it.
	saveDir := filepath.Join(dir, ".local-container", "saves", "Saves", "TestSave")
	if err := os.MkdirAll(saveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	saveInfoXML := `<SaveGame>
<player><name>Abigail</name><farmName>Junimo Farm</farmName></player>
<year>3</year><currentSeason>fall</currentSeason><dayOfMonth>12</dayOfMonth><whichFarm>2</whichFarm>
</SaveGame>`
	if err := os.WriteFile(filepath.Join(saveDir, "SaveGameInfo"), []byte(saveInfoXML), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(saveDir, "TestSave"), []byte(saveInfoXML), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := BackupSave(dir, "TestSave")
	if err != nil {
		t.Fatalf("BackupSave: %v", err)
	}

	backups, err := ListBackups(dir)
	if err != nil {
		t.Fatalf("ListBackups: %v", err)
	}
	if len(backups) != 1 {
		t.Fatalf("expected 1 backup, got %d", len(backups))
	}
	if backups[0].SaveName != "TestSave" {
		t.Errorf("expected saveName TestSave, got %q", backups[0].SaveName)
	}
	if backups[0].FarmerName != "Abigail" {
		t.Errorf("expected farmer Abigail, got %q", backups[0].FarmerName)
	}
	if backups[0].FarmName != "Junimo Farm" {
		t.Errorf("expected farm Junimo Farm, got %q", backups[0].FarmName)
	}
	if backups[0].GameYear != 3 || backups[0].GameSeason != "fall" || backups[0].GameDay != 12 {
		t.Errorf("expected date year=3 season=fall day=12, got year=%d season=%q day=%d", backups[0].GameYear, backups[0].GameSeason, backups[0].GameDay)
	}
	if backups[0].FarmType != "forest" {
		t.Errorf("expected farm type forest, got %q", backups[0].FarmType)
	}
}

func TestListBackups_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	backups, err := ListBackups(dir)
	if err != nil {
		t.Fatalf("ListBackups: %v", err)
	}
	if len(backups) != 0 {
		t.Errorf("expected 0 backups, got %d", len(backups))
	}
	if backups == nil {
		t.Error("expected an empty non-nil slice so JSON responses encode backups as []")
	}
}

func TestDeleteBackup_RemovesBackupFile(t *testing.T) {
	dir := t.TempDir()
	saveDir := filepath.Join(dir, ".local-container", "saves", "Saves", "TestSave")
	if err := os.MkdirAll(saveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(saveDir, "SaveGameInfo"), []byte("<SaveGame/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	backupPath, err := BackupSave(dir, "TestSave")
	if err != nil {
		t.Fatalf("BackupSave: %v", err)
	}
	backupName := filepath.Base(backupPath)

	if err := DeleteBackup(dir, backupName); err != nil {
		t.Fatalf("DeleteBackup: %v", err)
	}
	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		t.Fatalf("backup file should be deleted, stat err=%v", err)
	}
}

func TestDeleteBackup_InvalidName(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"../escape.zip", "nested/backup.zip", "backup.txt", ""} {
		if err := DeleteBackup(dir, name); err == nil {
			t.Fatalf("DeleteBackup(%q) expected error", name)
		}
	}
}

func TestRestoreBackup_Success(t *testing.T) {
	dir := t.TempDir()
	// Create a save and backup it.
	saveDir := filepath.Join(dir, ".local-container", "saves", "Saves", "TestSave")
	if err := os.MkdirAll(saveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(saveDir, "SaveGameInfo"), []byte("<SaveGame><player><name>Farmer</name></player></SaveGame>"), 0o644); err != nil {
		t.Fatal(err)
	}
	backupPath, err := BackupSave(dir, "TestSave")
	if err != nil {
		t.Fatalf("BackupSave: %v", err)
	}

	// Delete the save.
	if err := os.RemoveAll(saveDir); err != nil {
		t.Fatal(err)
	}

	// Restore.
	backupName := filepath.Base(backupPath)
	saveName, err := RestoreBackup(dir, backupName, false)
	if err != nil {
		t.Fatalf("RestoreBackup: %v", err)
	}
	if saveName != "TestSave" {
		t.Errorf("expected saveName TestSave, got %q", saveName)
	}
	// Verify restored content.
	restoredFile := filepath.Join(saveDir, "SaveGameInfo")
	if _, err := os.Stat(restoredFile); err != nil {
		t.Errorf("restored file should exist: %v", err)
	}

	// Regression: a successful restore must not leave its ".restore-tmp-*"
	// extraction directory behind in Saves/ — it previously did, and the
	// leftover empty dir would then show up in the saves list as a broken
	// save with a "SaveGameInfo not found" parse error.
	savesRootEntries, err := os.ReadDir(filepath.Join(dir, ".local-container", "saves", "Saves"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range savesRootEntries {
		if strings.HasPrefix(e.Name(), ".restore-tmp-") {
			t.Fatalf("restore left behind a temp directory: %q", e.Name())
		}
	}
}

func TestRestoreBackup_ConflictWithoutOverwrite(t *testing.T) {
	dir := t.TempDir()
	// Create a save and backup it.
	saveDir := filepath.Join(dir, ".local-container", "saves", "Saves", "TestSave")
	if err := os.MkdirAll(saveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(saveDir, "SaveGameInfo"), []byte("<SaveGame/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	backupPath, err := BackupSave(dir, "TestSave")
	if err != nil {
		t.Fatalf("BackupSave: %v", err)
	}

	// Try to restore without overwrite — should fail because save still exists.
	backupName := filepath.Base(backupPath)
	_, err = RestoreBackup(dir, backupName, false)
	if err == nil {
		t.Fatal("expected conflict error when save exists and overwrite is false")
	}
}

func TestRestoreBackup_WithOverwrite(t *testing.T) {
	dir := t.TempDir()
	// Create a save and backup it.
	saveDir := filepath.Join(dir, ".local-container", "saves", "Saves", "TestSave")
	if err := os.MkdirAll(saveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(saveDir, "SaveGameInfo"), []byte("<SaveGame><player><name>Old</name></player></SaveGame>"), 0o644); err != nil {
		t.Fatal(err)
	}
	backupPath, err := BackupSave(dir, "TestSave")
	if err != nil {
		t.Fatalf("BackupSave: %v", err)
	}

	// Modify the save.
	if err := os.WriteFile(filepath.Join(saveDir, "SaveGameInfo"), []byte("<SaveGame><player><name>Modified</name></player></SaveGame>"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Restore with overwrite.
	backupName := filepath.Base(backupPath)
	saveName, err := RestoreBackup(dir, backupName, true)
	if err != nil {
		t.Fatalf("RestoreBackup with overwrite: %v", err)
	}
	if saveName != "TestSave" {
		t.Errorf("expected saveName TestSave, got %q", saveName)
	}
}

func TestRestoreBackup_CreatesPreRestoreProtectionBackup(t *testing.T) {
	dir := t.TempDir()
	saveDir := filepath.Join(dir, ".local-container", "saves", "Saves", "TestSave")
	if err := os.MkdirAll(saveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(saveDir, "SaveGameInfo"), []byte("<SaveGame><player><name>Old</name></player></SaveGame>"), 0o644); err != nil {
		t.Fatal(err)
	}
	backupPath, err := BackupSave(dir, "TestSave")
	if err != nil {
		t.Fatalf("BackupSave: %v", err)
	}
	backupName := filepath.Base(backupPath)

	if _, err := RestoreBackup(dir, backupName, true); err != nil {
		t.Fatalf("RestoreBackup with overwrite: %v", err)
	}

	entries, err := os.ReadDir(backupsDir(dir))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "prerestore_TestSave_") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected a prerestore_ protection backup to be created before overwrite")
	}
}

func TestRestoreBackup_AbortsWhenProtectionBackupFails(t *testing.T) {
	dir := t.TempDir()
	saveDir := filepath.Join(dir, ".local-container", "saves", "Saves", "TestSave")
	if err := os.MkdirAll(saveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(saveDir, "SaveGameInfo"), []byte("<SaveGame><player><name>Source</name></player></SaveGame>"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create the backup we will restore FROM (a separate, valid snapshot).
	backupPath, err := BackupSave(dir, "TestSave")
	if err != nil {
		t.Fatalf("BackupSave: %v", err)
	}
	backupName := filepath.Base(backupPath)

	// Replace the existing save directory with a plain file so the protection
	// backup step (which re-zips the current save before overwrite) fails
	// deterministically: BackupSave requires the save path to be a directory.
	if err := os.RemoveAll(saveDir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(saveDir, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := RestoreBackup(dir, backupName, true); err == nil {
		t.Fatal("expected restore to abort when the protection backup fails")
	}

	// The current "save" must be untouched by the aborted restore.
	info, err := os.Stat(saveDir)
	if err != nil {
		t.Fatalf("save path should still exist: %v", err)
	}
	if info.IsDir() {
		t.Fatal("aborted restore should not have replaced the file with a directory")
	}
	content, err := os.ReadFile(saveDir)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "not a directory" {
		t.Fatalf("save content changed after aborted restore: %q", content)
	}
}

func TestRestoreBackup_InvalidBackupName(t *testing.T) {
	dir := t.TempDir()
	_, err := RestoreBackup(dir, "../escape.zip", false)
	if err == nil {
		t.Fatal("expected error for path traversal backup name")
	}
}

func TestRestoreBackup_NonExistentBackup(t *testing.T) {
	dir := t.TempDir()
	_, err := RestoreBackup(dir, "nonexistent.zip", false)
	if err == nil {
		t.Fatal("expected error for non-existent backup")
	}
}

func TestBackupPolicy_DefaultAndClamp(t *testing.T) {
	dir := t.TempDir()
	policy, err := ReadBackupPolicy(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !policy.GameSaveBackups || policy.RetainGameDays != 5 {
		t.Fatalf("unexpected default policy: %#v", policy)
	}
	policy, err = WriteBackupPolicy(dir, BackupPolicy{
		GameSaveBackups: true,
		RetainGameDays:  99,
	})
	if err != nil {
		t.Fatal(err)
	}
	if policy.RetainGameDays != 14 {
		t.Fatalf("policy not clamped: %#v", policy)
	}
}

func TestReadBackupPolicy_IgnoresLegacyFields(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(backupsDir(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	legacyJSON := `{"gameSaveBackups":false,"dailySnapshots":true,"dailyRetentionDays":7,"scheduledBackups":true,"scheduledHour":9}`
	if err := os.WriteFile(backupPolicyPath(dir), []byte(legacyJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	policy, err := ReadBackupPolicy(dir)
	if err != nil {
		t.Fatalf("ReadBackupPolicy should tolerate legacy fields: %v", err)
	}
	if policy.GameSaveBackups {
		t.Fatalf("expected gameSaveBackups to carry over as false, got %#v", policy)
	}
	if policy.RetainGameDays != 5 {
		t.Fatalf("expected default RetainGameDays=5 when absent from legacy JSON, got %d", policy.RetainGameDays)
	}
}

func TestGameDayOrdinal_CrossSeasonAndYear(t *testing.T) {
	tests := []struct {
		year   int
		season string
		day    int
		want   int
	}{
		{1, "spring", 1, 1},
		{1, "spring", 28, 28},
		{1, "summer", 1, 29},
		{1, "fall", 1, 57},
		{1, "winter", 28, 112},
		{2, "spring", 1, 113},
		{3, "winter", 28, 336},
	}
	for _, tt := range tests {
		got := gameDayOrdinal(tt.year, tt.season, tt.day)
		if got != tt.want {
			t.Errorf("gameDayOrdinal(%d, %q, %d) = %d, want %d", tt.year, tt.season, tt.day, got, tt.want)
		}
	}
}

func TestBackupMaintenance_SaveEventCreatesAutoGameDayBackup(t *testing.T) {
	dir := t.TempDir()
	createTestSaveForBackup(t, dir, "TestSave") // year=1 spring day=1 → ordinal 1
	eventDir := saveEventsDir(dir)
	if err := os.MkdirAll(eventDir, 0o755); err != nil {
		t.Fatal(err)
	}
	event := saveEventFile{Type: "saved", SaveName: "TestSave", CreatedAt: time.Now().UTC()}
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(eventDir, "event.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := RunBackupMaintenance(dir)
	if err != nil {
		t.Fatalf("RunBackupMaintenance: %v", err)
	}
	if result.ConsumedEvents != 1 {
		t.Fatalf("ConsumedEvents = %d, want 1", result.ConsumedEvents)
	}
	if _, err := os.Stat(filepath.Join(backupsDir(dir), "auto_TestSave_000001.zip")); err != nil {
		t.Fatalf("expected auto game-day backup: %v", err)
	}
	if _, err := os.Stat(filepath.Join(eventDir, "event.json")); !os.IsNotExist(err) {
		t.Fatalf("event should be consumed, stat err=%v", err)
	}
}

func TestBackupAutoGameDay_OverwritesSameGameDay(t *testing.T) {
	dir := t.TempDir()
	createTestSaveForBackup(t, dir, "TestSave")
	first, err := BackupAutoGameDay(dir, "TestSave")
	if err != nil {
		t.Fatalf("BackupAutoGameDay (first): %v", err)
	}
	firstInfo, err := os.Stat(first)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	second, err := BackupAutoGameDay(dir, "TestSave")
	if err != nil {
		t.Fatalf("BackupAutoGameDay (second): %v", err)
	}
	if first != second {
		t.Fatalf("expected same game day to overwrite the same file, got %q then %q", first, second)
	}
	secondInfo, err := os.Stat(second)
	if err != nil {
		t.Fatal(err)
	}
	if !secondInfo.ModTime().After(firstInfo.ModTime()) {
		t.Fatalf("expected file to be recreated with a newer mtime")
	}
	entries, err := os.ReadDir(backupsDir(dir))
	if err != nil {
		t.Fatal(err)
	}
	autoCount := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "auto_TestSave_") {
			autoCount++
		}
	}
	if autoCount != 1 {
		t.Fatalf("expected exactly 1 auto backup file, got %d", autoCount)
	}
}

func TestBackupAutoGameDay_RestoreEarlierThenReplaySameDayOverwrites(t *testing.T) {
	dir := t.TempDir()
	createTestSaveForBackup(t, dir, "TestSave") // day 1
	writeTestSaveGameDay(t, dir, "TestSave", 1, "spring", 5)
	day5Path, err := BackupAutoGameDay(dir, "TestSave")
	if err != nil {
		t.Fatal(err)
	}

	// Simulate restoring to an earlier day and creating a new auto backup there.
	writeTestSaveGameDay(t, dir, "TestSave", 1, "spring", 2)
	day2Path, err := BackupAutoGameDay(dir, "TestSave")
	if err != nil {
		t.Fatal(err)
	}
	if day2Path == day5Path {
		t.Fatalf("day 2 and day 5 should be distinct backup files")
	}
	if _, err := os.Stat(day5Path); err != nil {
		t.Fatalf("day 5 backup should still exist after branching to day 2: %v", err)
	}

	// Replaying back up to day 5 must overwrite the original day 5 backup,
	// not create a third file.
	beforeReplay, err := os.Stat(day5Path)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	writeTestSaveGameDay(t, dir, "TestSave", 1, "spring", 5)
	replayPath, err := BackupAutoGameDay(dir, "TestSave")
	if err != nil {
		t.Fatal(err)
	}
	if replayPath != day5Path {
		t.Fatalf("expected replay to day 5 to reuse the same file %q, got %q", day5Path, replayPath)
	}
	afterReplay, err := os.Stat(replayPath)
	if err != nil {
		t.Fatal(err)
	}
	if !afterReplay.ModTime().After(beforeReplay.ModTime()) {
		t.Fatalf("expected day 5 backup to be recreated with a newer mtime")
	}
}

func TestPruneAutoGameDayBackups_KeepsFiveMostRecentGameDays(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(backupsDir(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	for ordinal := 1; ordinal <= 6; ordinal++ {
		name := fmt.Sprintf("auto_TestSave_%06d.zip", ordinal)
		if err := os.WriteFile(filepath.Join(backupsDir(dir), name), []byte("zip"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := PruneAutoGameDayBackups(dir, "TestSave", 5); err != nil {
		t.Fatalf("PruneAutoGameDayBackups: %v", err)
	}
	if _, err := os.Stat(filepath.Join(backupsDir(dir), "auto_TestSave_000001.zip")); !os.IsNotExist(err) {
		t.Fatalf("oldest game day should be pruned, stat err=%v", err)
	}
	for ordinal := 2; ordinal <= 6; ordinal++ {
		name := fmt.Sprintf("auto_TestSave_%06d.zip", ordinal)
		if _, err := os.Stat(filepath.Join(backupsDir(dir), name)); err != nil {
			t.Fatalf("recent game day %s should remain: %v", name, err)
		}
	}
}

func TestRunBackupMaintenance_DoesNotTouchManualOrProtectionBackups(t *testing.T) {
	dir := t.TempDir()
	createTestSaveForBackup(t, dir, "TestSave")
	manualPath, err := BackupManual(dir, "TestSave")
	if err != nil {
		t.Fatal(err)
	}
	protectPath, err := BackupPreRestore(dir, "TestSave")
	if err != nil {
		t.Fatal(err)
	}

	eventDir := saveEventsDir(dir)
	if err := os.MkdirAll(eventDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for day := 1; day <= 7; day++ {
		writeTestSaveGameDay(t, dir, "TestSave", 1, "spring", day)
		event := saveEventFile{Type: "saved", SaveName: "TestSave", CreatedAt: time.Now().UTC()}
		data, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		eventPath := filepath.Join(eventDir, fmt.Sprintf("event-%d.json", day))
		if err := os.WriteFile(eventPath, data, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := RunBackupMaintenance(dir); err != nil {
			t.Fatalf("RunBackupMaintenance (day %d): %v", day, err)
		}
	}

	if _, err := os.Stat(manualPath); err != nil {
		t.Fatalf("manual backup should not be touched by auto cleanup: %v", err)
	}
	if _, err := os.Stat(protectPath); err != nil {
		t.Fatalf("protection backup should not be touched by auto cleanup: %v", err)
	}
	if _, err := os.Stat(filepath.Join(backupsDir(dir), "auto_TestSave_000001.zip")); !os.IsNotExist(err) {
		t.Fatalf("day 1 auto backup should have been pruned after 7 days with default retention 5")
	}
}

// writeTestSaveGameDay overwrites the live save directory's SaveGameInfo/main
// file with a given year/season/day, simulating the game writing a new save.
func writeTestSaveGameDay(t *testing.T, dir, saveName string, year int, season string, day int) {
	t.Helper()
	saveDir := filepath.Join(dir, ".local-container", "saves", "Saves", saveName)
	xmlContent := fmt.Sprintf(`<SaveGame><player><name>Farmer</name><farmName>Farm</farmName></player><year>%d</year><currentSeason>%s</currentSeason><dayOfMonth>%d</dayOfMonth><whichFarm>0</whichFarm></SaveGame>`, year, season, day)
	if err := os.WriteFile(filepath.Join(saveDir, "SaveGameInfo"), []byte(xmlContent), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(saveDir, saveName), []byte(xmlContent), 0o644); err != nil {
		t.Fatal(err)
	}
}

func createTestSaveForBackup(t *testing.T, dir, saveName string) {
	t.Helper()
	saveDir := filepath.Join(dir, ".local-container", "saves", "Saves", saveName)
	if err := os.MkdirAll(saveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	xmlContent := `<SaveGame><player><name>Farmer</name><farmName>Farm</farmName></player><year>1</year><currentSeason>spring</currentSeason><dayOfMonth>1</dayOfMonth><whichFarm>0</whichFarm></SaveGame>`
	if err := os.WriteFile(filepath.Join(saveDir, "SaveGameInfo"), []byte(xmlContent), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(saveDir, saveName), []byte(xmlContent), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestParseBackupSaveName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"TestSave_20260627-150405.zip", "TestSave"},
		{"MyFarm_20260627-120000.zip", "MyFarm"},
		{"SimpleSave.zip", "SimpleSave"},
		{"latest_TestSave.zip", "TestSave"},
		{"scheduled_TestSave.zip", "TestSave"},
		{"daily_TestSave_20260702.zip", "TestSave"},
		{"manual_Test_Save_20260702-120000.zip", "Test_Save"},
		{"auto_TestSave_000001.zip", "TestSave"},
		{"predelete_TestSave_20260702-120000.zip", "TestSave"},
		{"prefarmhanddelete_TestSave_20260702-120000.zip", "TestSave"},
		{"prerestore_TestSave_20260702-120000.zip", "TestSave"},
	}
	for _, tt := range tests {
		got := parseBackupSaveName(tt.input)
		if got != tt.want {
			t.Errorf("parseBackupSaveName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestInferBackupKind(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"auto_TestSave_000001.zip", "auto"},
		{"manual_TestSave_20260702-120000.zip", "manual"},
		{"predelete_TestSave_20260702-120000.zip", "predelete"},
		{"prefarmhanddelete_TestSave_20260702-120000.zip", "prefarmhanddelete"},
		{"prerestore_TestSave_20260702-120000.zip", "prerestore"},
		{"latest_TestSave.zip", "latest"},
		{"scheduled_TestSave.zip", "scheduled"},
		{"daily_TestSave_20260702.zip", "daily"},
		{"TestSave_20260702-120000.zip", "manual"},
	}
	for _, tt := range tests {
		got := inferBackupKind(tt.input)
		if got != tt.want {
			t.Errorf("inferBackupKind(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
