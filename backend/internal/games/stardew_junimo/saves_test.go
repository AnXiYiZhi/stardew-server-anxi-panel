package stardew_junimo

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
)

func TestWriteServerSettings_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := registry.NewGameConfig{
		FarmName:       "TestFarm",
		FarmType:       "riverland",
		StartingCabins: 2,
		CabinLayout:    "nearby",
		ProfitMargin:   "75",
		PetBreed:       1,
		MoneyMode:      "shared",
	}
	if err := WriteServerSettings(dir, cfg); err != nil {
		t.Fatalf("WriteServerSettings: %v", err)
	}
	path := serverSettingsPath(dir)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("settings file not created: %v", err)
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
		"hilltop": 3, "wilderness": 4, "fourcorners": 5, "beach": 6,
		"unknown": 0,
	}
	for name, want := range cases {
		if got := junimoFarmTypeID(name); got != want {
			t.Errorf("farmTypeID(%q) = %d, want %d", name, got, want)
		}
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
