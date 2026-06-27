package stardew_junimo

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// ── Helper ────────────────────────────────────────────────────────────────────

func createTestMod(t *testing.T, modsRoot, folderName, uniqueID, name string) {
	t.Helper()
	modPath := filepath.Join(modsRoot, folderName)
	if err := os.MkdirAll(modPath, 0o755); err != nil {
		t.Fatal(err)
	}
	m := modManifest{
		Name:     name,
		UniqueID: uniqueID,
		Version:  "1.0.0",
		Author:   "Test",
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modPath, "manifest.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func createModZip(t *testing.T, entries map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "mod.zip")
	zf, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(zf)
	for name, content := range entries {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zf.Close(); err != nil {
		t.Fatal(err)
	}
	return zipPath
}

// ── ListMods ──────────────────────────────────────────────────────────────────

func TestListMods_Empty(t *testing.T) {
	dir := t.TempDir()
	mods, err := ListMods(dir)
	if err != nil {
		t.Fatalf("ListMods: %v", err)
	}
	if len(mods) != 0 {
		t.Fatalf("expected 0 mods, got %d", len(mods))
	}
}

func TestListMods_WithMods(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".local-container", "mods")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	createTestMod(t, root, "TestMod", "author.testmod", "Test Mod")
	createTestMod(t, root, "AnotherMod", "author.another", "Another Mod")

	mods, err := ListMods(dir)
	if err != nil {
		t.Fatalf("ListMods: %v", err)
	}
	if len(mods) != 2 {
		t.Fatalf("expected 2 mods, got %d", len(mods))
	}
}

func TestListMods_ParseError(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".local-container", "mods")
	modPath := filepath.Join(root, "BadMod")
	if err := os.MkdirAll(modPath, 0o755); err != nil {
		t.Fatal(err)
	}
	// Write invalid JSON.
	if err := os.WriteFile(filepath.Join(modPath, "manifest.json"), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	mods, err := ListMods(dir)
	if err != nil {
		t.Fatalf("ListMods: %v", err)
	}
	if len(mods) != 1 {
		t.Fatalf("expected 1 mod, got %d", len(mods))
	}
	if mods[0].ParseError == "" {
		t.Fatal("expected parse error for invalid manifest")
	}
}

func TestListMods_NoManifest(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".local-container", "mods")
	modPath := filepath.Join(root, "NoManifestMod")
	if err := os.MkdirAll(modPath, 0o755); err != nil {
		t.Fatal(err)
	}

	mods, err := ListMods(dir)
	if err != nil {
		t.Fatalf("ListMods: %v", err)
	}
	if len(mods) != 1 {
		t.Fatalf("expected 1 mod, got %d", len(mods))
	}
	if mods[0].ParseError == "" {
		t.Fatal("expected parse error for missing manifest")
	}
}

func TestListMods_SkipsFiles(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".local-container", "mods")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	// Create a file (not a directory) in mods root.
	if err := os.WriteFile(filepath.Join(root, "readme.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	createTestMod(t, root, "RealMod", "author.real", "Real Mod")

	mods, err := ListMods(dir)
	if err != nil {
		t.Fatalf("ListMods: %v", err)
	}
	if len(mods) != 1 {
		t.Fatalf("expected 1 mod, got %d", len(mods))
	}
	if mods[0].FolderName != "RealMod" {
		t.Fatalf("expected RealMod, got %s", mods[0].FolderName)
	}
}

// ── UploadModZip ──────────────────────────────────────────────────────────────

func TestUploadModZip_ValidSingleMod(t *testing.T) {
	dir := t.TempDir()
	manifest := `{"Name":"Test Mod","UniqueID":"author.testmod","Version":"1.0.0","Author":"Tester"}`
	zipPath := createModZip(t, map[string]string{
		"TestMod/manifest.json": manifest,
		"TestMod/data.json":     "{}",
	})

	imported, err := UploadModZip(dir, zipPath)
	if err != nil {
		t.Fatalf("UploadModZip: %v", err)
	}
	if len(imported) != 1 {
		t.Fatalf("expected 1 imported mod, got %d", len(imported))
	}
	if imported[0].UniqueID != "author.testmod" {
		t.Errorf("UniqueID = %q, want author.testmod", imported[0].UniqueID)
	}
	if imported[0].Name != "Test Mod" {
		t.Errorf("Name = %q, want Test Mod", imported[0].Name)
	}

	// Verify file exists on disk.
	manifestPath := filepath.Join(dir, ".local-container", "mods", "TestMod", "manifest.json")
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("manifest not found on disk: %v", err)
	}
}

func TestUploadModZip_ValidMultipleMods(t *testing.T) {
	dir := t.TempDir()
	manifest1 := `{"Name":"Mod A","UniqueID":"author.moda","Version":"1.0","Author":"A"}`
	manifest2 := `{"Name":"Mod B","UniqueID":"author.modb","Version":"2.0","Author":"B"}`
	zipPath := createModZip(t, map[string]string{
		"ModA/manifest.json": manifest1,
		"ModB/manifest.json": manifest2,
	})

	imported, err := UploadModZip(dir, zipPath)
	if err != nil {
		t.Fatalf("UploadModZip: %v", err)
	}
	if len(imported) != 2 {
		t.Fatalf("expected 2 imported mods, got %d", len(imported))
	}
}

func TestUploadModZip_RejectsZipSlip(t *testing.T) {
	dir := t.TempDir()
	zipPath := createModZip(t, map[string]string{
		"../evil/manifest.json": `{"Name":"Evil","UniqueID":"evil","Version":"1.0","Author":"E"}`,
	})

	_, err := UploadModZip(dir, zipPath)
	if err == nil {
		t.Fatal("expected error for zip-slip path")
	}
}

func TestUploadModZip_RejectsAbsolutePath(t *testing.T) {
	dir := t.TempDir()
	zipPath := createModZip(t, map[string]string{
		"/etc/manifest.json": `{"Name":"Evil","UniqueID":"evil","Version":"1.0","Author":"E"}`,
	})

	_, err := UploadModZip(dir, zipPath)
	if err == nil {
		t.Fatal("expected error for absolute path")
	}
}

func TestUploadModZip_RejectsDoubleDot(t *testing.T) {
	dir := t.TempDir()
	zipPath := createModZip(t, map[string]string{
		"foo/../bar/manifest.json": `{"Name":"Evil","UniqueID":"evil","Version":"1.0","Author":"E"}`,
	})

	_, err := UploadModZip(dir, zipPath)
	if err == nil {
		t.Fatal("expected error for foo/../bar path")
	}
}

func TestUploadModZip_RejectsDoubleSlash(t *testing.T) {
	dir := t.TempDir()
	zipPath := createModZip(t, map[string]string{
		"foo//manifest.json": `{"Name":"Evil","UniqueID":"evil","Version":"1.0","Author":"E"}`,
	})

	_, err := UploadModZip(dir, zipPath)
	if err == nil {
		t.Fatal("expected error for foo// path")
	}
}

func TestUploadModZip_AllowsDirectoryEntry(t *testing.T) {
	dir := t.TempDir()
	manifest := `{"Name":"Test Mod","UniqueID":"author.testmod","Version":"1.0.0","Author":"Tester"}`
	zipPath := createModZip(t, map[string]string{
		"TestMod/":             "",
		"TestMod/manifest.json": manifest,
	})

	imported, err := UploadModZip(dir, zipPath)
	if err != nil {
		t.Fatalf("UploadModZip with directory entry: %v", err)
	}
	if len(imported) != 1 {
		t.Fatalf("expected 1 imported mod, got %d", len(imported))
	}
}

func TestUploadModZip_RejectsNoManifest(t *testing.T) {
	dir := t.TempDir()
	zipPath := createModZip(t, map[string]string{
		"BadMod/data.json": "{}",
	})

	_, err := UploadModZip(dir, zipPath)
	if err == nil {
		t.Fatal("expected error for mod without manifest")
	}
}

func TestUploadModZip_RejectsDuplicateUniqueID(t *testing.T) {
	dir := t.TempDir()
	manifest := `{"Name":"Test Mod","UniqueID":"author.testmod","Version":"1.0.0","Author":"Tester"}`

	// First upload.
	zipPath1 := createModZip(t, map[string]string{"Mod1/manifest.json": manifest})
	if _, err := UploadModZip(dir, zipPath1); err != nil {
		t.Fatalf("first upload: %v", err)
	}

	// Second upload with same UniqueID but different folder.
	zipPath2 := createModZip(t, map[string]string{"Mod2/manifest.json": manifest})
	_, err := UploadModZip(dir, zipPath2)
	if err == nil {
		t.Fatal("expected error for duplicate UniqueID")
	}
}

func TestUploadModZip_AtomicOnSecondModFailure(t *testing.T) {
	dir := t.TempDir()
	manifest1 := `{"Name":"Good Mod","UniqueID":"author.good","Version":"1.0","Author":"A"}`
	// Second mod has no UniqueID — will fail validation.
	badManifest := `{"Name":"Bad Mod","Version":"1.0","Author":"B"}`

	zipPath := createModZip(t, map[string]string{
		"GoodMod/manifest.json": manifest1,
		"BadMod/manifest.json":  badManifest,
	})

	_, err := UploadModZip(dir, zipPath)
	if err == nil {
		t.Fatal("expected error for bad manifest in second mod")
	}

	// Verify GoodMod was NOT left behind (atomicity).
	modsRoot := filepath.Join(dir, ".local-container", "mods")
	if _, statErr := os.Stat(filepath.Join(modsRoot, "GoodMod")); statErr == nil {
		t.Fatal("GoodMod should NOT have been installed when BadMod failed")
	}
}

func TestUploadModZip_RejectsExistingFolder(t *testing.T) {
	dir := t.TempDir()
	manifest := `{"Name":"Test Mod","UniqueID":"author.testmod","Version":"1.0.0","Author":"Tester"}`

	// First upload.
	zipPath1 := createModZip(t, map[string]string{"TestMod/manifest.json": manifest})
	if _, err := UploadModZip(dir, zipPath1); err != nil {
		t.Fatalf("first upload: %v", err)
	}

	// Second upload with same folder name.
	zipPath2 := createModZip(t, map[string]string{"TestMod/manifest.json": manifest})
	_, err := UploadModZip(dir, zipPath2)
	if err == nil {
		t.Fatal("expected error for existing folder")
	}
}

// ── DeleteMod ─────────────────────────────────────────────────────────────────

func TestDeleteMod_ByFolderName(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".local-container", "mods")
	createTestMod(t, root, "TestMod", "author.test", "Test")

	if err := DeleteMod(dir, "TestMod"); err != nil {
		t.Fatalf("DeleteMod: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "TestMod")); !os.IsNotExist(err) {
		t.Fatal("mod folder should have been deleted")
	}
}

func TestDeleteMod_ByUniqueID(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".local-container", "mods")
	createTestMod(t, root, "TestMod", "author.test", "Test")

	if err := DeleteMod(dir, "author.test"); err != nil {
		t.Fatalf("DeleteMod by UniqueID: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "TestMod")); !os.IsNotExist(err) {
		t.Fatal("mod folder should have been deleted")
	}
}

func TestDeleteMod_RejectsDotDot(t *testing.T) {
	dir := t.TempDir()
	if err := DeleteMod(dir, ".."); err == nil {
		t.Fatal("expected error for .. mod name")
	}
}

func TestDeleteMod_RejectsPathSeparator(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"../evil", "foo/bar", `foo\bar`, "/absolute"} {
		if err := DeleteMod(dir, name); err == nil {
			t.Fatalf("expected error for mod name %q", name)
		}
	}
}

func TestDeleteMod_CannotDeleteModsRoot(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".local-container", "mods")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := DeleteMod(dir, "."); err == nil {
		t.Fatal("expected error for . mod name")
	}
	if _, err := os.Stat(root); os.IsNotExist(err) {
		t.Fatal("mods root was deleted")
	}
}

func TestDeleteMod_NotFound(t *testing.T) {
	dir := t.TempDir()
	if err := DeleteMod(dir, "NonExistent"); err == nil {
		t.Fatal("expected error for non-existent mod")
	}
}

// ── FindModByUniqueID ─────────────────────────────────────────────────────────

func TestFindModByUniqueID_Found(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".local-container", "mods")
	createTestMod(t, root, "TestMod", "author.test", "Test")

	folder, err := FindModByUniqueID(dir, "author.test")
	if err != nil {
		t.Fatalf("FindModByUniqueID: %v", err)
	}
	if folder != "TestMod" {
		t.Fatalf("expected TestMod, got %q", folder)
	}
}

func TestFindModByUniqueID_NotFound(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".local-container", "mods")
	createTestMod(t, root, "TestMod", "author.test", "Test")

	folder, err := FindModByUniqueID(dir, "nonexistent")
	if err != nil {
		t.Fatalf("FindModByUniqueID: %v", err)
	}
	if folder != "" {
		t.Fatalf("expected empty, got %q", folder)
	}
}

// ── ValidateModName ───────────────────────────────────────────────────────────

func TestValidateModName(t *testing.T) {
	cases := []struct {
		name  string
		valid bool
	}{
		{"TestMod", true},
		{"My Cool Mod", true},
		{"", false},
		{".", false},
		{"..", false},
		{"../evil", false},
		{"foo/bar", false},
		{`foo\bar`, false},
	}
	for _, tc := range cases {
		err := ValidateModName(tc.name)
		if tc.valid && err != nil {
			t.Errorf("ValidateModName(%q) expected valid, got: %v", tc.name, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("ValidateModName(%q) expected invalid, got nil", tc.name)
		}
	}
}

// ── ExportModsZip ─────────────────────────────────────────────────────────────

func TestExportModsZip_Valid(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".local-container", "mods")
	createTestMod(t, root, "TestMod", "author.test", "Test Mod")

	zipPath, err := ExportModsZip(dir)
	if err != nil {
		t.Fatalf("ExportModsZip: %v", err)
	}
	defer func() { _ = os.Remove(zipPath) }()

	// Verify the ZIP contains the mod.
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open exported zip: %v", err)
	}
	defer func() { _ = zr.Close() }()

	found := false
	for _, f := range zr.File {
		if f.Name == "TestMod/manifest.json" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("exported ZIP missing TestMod/manifest.json")
	}
}

func TestExportModsZip_NoMods(t *testing.T) {
	dir := t.TempDir()
	_, err := ExportModsZip(dir)
	if err == nil {
		t.Fatal("expected error when no mods exist")
	}
}

func TestExportModsZip_RelativePaths(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".local-container", "mods")
	createTestMod(t, root, "TestMod", "author.test", "Test Mod")

	zipPath, err := ExportModsZip(dir)
	if err != nil {
		t.Fatalf("ExportModsZip: %v", err)
	}
	defer func() { _ = os.Remove(zipPath) }()

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open exported zip: %v", err)
	}
	defer func() { _ = zr.Close() }()

	for _, f := range zr.File {
		if filepath.IsAbs(f.Name) {
			t.Errorf("ZIP entry %q is an absolute path", f.Name)
		}
	}
}

// ── Restart required flag ─────────────────────────────────────────────────────

func TestRestartRequiredFlag(t *testing.T) {
	dir := t.TempDir()
	if GetModsRestartRequired(dir) {
		t.Fatal("should not be restart required initially")
	}
	if err := SetModsRestartRequired(dir); err != nil {
		t.Fatalf("SetModsRestartRequired: %v", err)
	}
	if !GetModsRestartRequired(dir) {
		t.Fatal("should be restart required after set")
	}
	if err := ClearModsRestartRequired(dir); err != nil {
		t.Fatalf("ClearModsRestartRequired: %v", err)
	}
	if GetModsRestartRequired(dir) {
		t.Fatal("should not be restart required after clear")
	}
}

// ── migrateModsCompose ────────────────────────────────────────────────────────

func TestMigrateModsCompose_AddsMount(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "docker-compose.yml")
	compose := `services:
  server:
    volumes:
      - ./.local-container/settings:/data/settings
      - ./.local-container/mods/StardewAnxiPanel.Control:/data/Mods/StardewAnxiPanel.Control
`
	if err := os.WriteFile(path, []byte(compose), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, err := migrateModsCompose(path)
	if err != nil {
		t.Fatalf("migrateModsCompose: %v", err)
	}
	if !changed {
		t.Fatal("expected changed=true")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !contains(content, "- ./.local-container/mods:/data/Mods") {
		t.Error("mods bind mount not found")
	}
}

func TestMigrateModsCompose_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "docker-compose.yml")
	compose := `services:
  server:
    volumes:
      - ./.local-container/mods/StardewAnxiPanel.Control:/data/Mods/StardewAnxiPanel.Control
      - ./.local-container/mods:/data/Mods
`
	if err := os.WriteFile(path, []byte(compose), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, err := migrateModsCompose(path)
	if err != nil {
		t.Fatalf("migrateModsCompose: %v", err)
	}
	if changed {
		t.Fatal("expected changed=false for already-migrated compose")
	}
}

// ── readModInfo ───────────────────────────────────────────────────────────────

func TestReadModInfo_AllFields(t *testing.T) {
	dir := t.TempDir()
	modPath := filepath.Join(dir, "TestMod")
	if err := os.MkdirAll(modPath, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{
		"Name": "Test Mod",
		"UniqueID": "author.testmod",
		"Version": "1.2.3",
		"Author": "TestAuthor",
		"Description": "A test mod"
	}`
	if err := os.WriteFile(filepath.Join(modPath, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	info := readModInfo(modPath, "TestMod")
	if info.ParseError != "" {
		t.Fatalf("unexpected parse error: %s", info.ParseError)
	}
	if info.UniqueID != "author.testmod" {
		t.Errorf("UniqueID = %q, want author.testmod", info.UniqueID)
	}
	if info.Name != "Test Mod" {
		t.Errorf("Name = %q, want Test Mod", info.Name)
	}
	if info.Version != "1.2.3" {
		t.Errorf("Version = %q, want 1.2.3", info.Version)
	}
	if info.Author != "TestAuthor" {
		t.Errorf("Author = %q, want TestAuthor", info.Author)
	}
	if info.Description != "A test mod" {
		t.Errorf("Description = %q, want A test mod", info.Description)
	}
	if info.FolderName != "TestMod" {
		t.Errorf("FolderName = %q, want TestMod", info.FolderName)
	}
}

func TestReadModInfo_MissingUniqueID(t *testing.T) {
	dir := t.TempDir()
	modPath := filepath.Join(dir, "BadMod")
	if err := os.MkdirAll(modPath, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"Name": "Bad Mod", "Version": "1.0"}`
	if err := os.WriteFile(filepath.Join(modPath, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	info := readModInfo(modPath, "BadMod")
	if info.ParseError == "" {
		t.Fatal("expected parse error for missing UniqueID")
	}
}

func TestReadModInfo_MissingName(t *testing.T) {
	dir := t.TempDir()
	modPath := filepath.Join(dir, "BadMod")
	if err := os.MkdirAll(modPath, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"UniqueID": "author.bad", "Version": "1.0"}`
	if err := os.WriteFile(filepath.Join(modPath, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	info := readModInfo(modPath, "BadMod")
	if info.ParseError == "" {
		t.Fatal("expected parse error for missing Name")
	}
}

// ── buildModsZipName ──────────────────────────────────────────────────────────

func TestBuildModsZipName_SingleMod(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".local-container", "mods")
	createTestMod(t, root, "MyMod", "author.mymod", "My Cool Mod")

	entries, _ := os.ReadDir(root)
	name := buildModsZipName(root, entries)
	if name != "My_Cool_Mod_Test.zip" {
		t.Errorf("buildModsZipName = %q, want My_Cool_Mod_Test.zip", name)
	}
}

func TestBuildModsZipName_MultipleMods(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".local-container", "mods")
	createTestMod(t, root, "ModA", "author.a", "Mod A")
	createTestMod(t, root, "ModB", "author.b", "Mod B")

	entries, _ := os.ReadDir(root)
	name := buildModsZipName(root, entries)
	if name != "stardew-mods-2.zip" {
		t.Errorf("buildModsZipName = %q, want stardew-mods-2.zip", name)
	}
}

func TestBuildModsZipName_SingleModNoManifest(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".local-container", "mods")
	modPath := filepath.Join(root, "RawMod")
	if err := os.MkdirAll(modPath, 0o755); err != nil {
		t.Fatal(err)
	}

	entries, _ := os.ReadDir(root)
	name := buildModsZipName(root, entries)
	if name != "RawMod.zip" {
		t.Errorf("buildModsZipName = %q, want RawMod.zip", name)
	}
}
