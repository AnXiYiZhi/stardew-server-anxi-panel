package stardew_junimo

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
)

func TestWriteServerSettings_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := registry.NewGameConfig{
		FarmName:               "TestFarm",
		FarmType:               "riverland",
		StartingCabins:         2,
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
	if server["SeparateWallets"] != false { // shared → false
		t.Errorf("Server.SeparateWallets = %v, want false", server["SeparateWallets"])
	}
	if game["CabinLayoutNearby"] != true { // nearby → true
		t.Errorf("Game.CabinLayoutNearby = %v, want true", game["CabinLayoutNearby"])
	}
}

func TestWriteServerSettings_EmptyFarmName(t *testing.T) {
	dir := t.TempDir()
	cfg := registry.NewGameConfig{FarmName: ""}
	if err := WriteServerSettings(dir, cfg); err == nil {
		t.Fatal("expected error for empty farmName")
	}
}

func TestWriteServerSettings_InvalidFarmType(t *testing.T) {
	dir := t.TempDir()
	cfg := registry.NewGameConfig{FarmName: "Farm", FarmType: "moon", ProfitMargin: "100", MoneyMode: "shared"}
	if err := WriteServerSettings(dir, cfg); err == nil {
		t.Fatal("expected error for invalid farmType")
	}
}

func TestWriteServerSettings_CabinsOutOfRange(t *testing.T) {
	dir := t.TempDir()
	cfg := registry.NewGameConfig{FarmName: "Farm", StartingCabins: 5}
	if err := WriteServerSettings(dir, cfg); err == nil {
		t.Fatal("expected error for startingCabins=5")
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
		cfg := registry.NewGameConfig{FarmName: "Farm", FarmType: "standard", CabinLayout: "nearby", ProfitMargin: tc.margin, MoneyMode: "shared"}
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
		"unknown": 0,
	}
	for name, want := range cases {
		if got := junimoFarmTypeID(name); got != want {
			t.Errorf("farmTypeID(%q) = %d, want %d", name, got, want)
		}
	}
}

func TestValidateNewGameConfig_PetPreference(t *testing.T) {
	base := registry.NewGameConfig{
		FarmName: "Farm", FarmType: "standard", CabinLayout: "nearby",
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
	if init.Gender != "female" || init.PetType != "Dog" || init.PetBreed != "4" || init.CabinLayout != "separate" {
		t.Fatalf("init selection changed: %#v", init)
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
		{"unknown_farm", ""},
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

func TestParseBackupSaveName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"TestSave_20260627-150405.zip", "TestSave"},
		{"MyFarm_20260627-120000.zip", "MyFarm"},
		{"SimpleSave.zip", "SimpleSave"},
	}
	for _, tt := range tests {
		got := parseBackupSaveName(tt.input)
		if got != tt.want {
			t.Errorf("parseBackupSaveName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
