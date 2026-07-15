package stardew_junimo

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"golang.org/x/text/encoding/simplifiedchinese"
)

// ── Helper ────────────────────────────────────────────────────────────────────

func createTestMod(t *testing.T, modsRoot, folderName, uniqueID, name string) {
	t.Helper()
	createTestModWithManifest(t, modsRoot, folderName, modManifest{
		Name:     name,
		UniqueID: uniqueID,
		Version:  "1.0.0",
		Author:   "Test",
	})
}

func createTestModWithManifest(t *testing.T, modsRoot, folderName string, m modManifest) {
	t.Helper()
	modPath := filepath.Join(modsRoot, folderName)
	if err := os.MkdirAll(modPath, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modPath, "manifest.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func modFoldersForTest(mods []registry.ModInfo) []string {
	folders := make([]string, 0, len(mods))
	for _, mod := range mods {
		folders = append(folders, mod.FolderName)
	}
	sort.Strings(folders)
	return folders
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

func TestListMods_IncludesSMAPIRuntimeWhenControlModInstalled(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".local-container", "mods")
	createTestMod(t, root, controlModFolderName, "StardewAnxiPanel.Control", "Panel Control")

	mods, err := ListMods(dir)
	if err != nil {
		t.Fatalf("ListMods: %v", err)
	}
	if len(mods) != 2 {
		t.Fatalf("expected SMAPI runtime plus control mod, got %d mods: %+v", len(mods), mods)
	}
	smapi := mods[0]
	if !smapi.BuiltIn {
		t.Fatalf("first mod should be built-in SMAPI runtime: %+v", smapi)
	}
	if smapi.Name != "SMAPI" || smapi.UniqueID != "Pathoschild.SMAPI" {
		t.Fatalf("unexpected SMAPI runtime info: %+v", smapi)
	}
	if smapi.NexusModID != 2400 || len(smapi.UpdateKeys) != 1 || smapi.UpdateKeys[0] != "Nexus:2400" {
		t.Fatalf("SMAPI Nexus metadata = id %d keys %v, want Nexus:2400", smapi.NexusModID, smapi.UpdateKeys)
	}
	if smapi.SyncKind != "client_required" {
		t.Fatalf("SMAPI SyncKind = %q, want client_required", smapi.SyncKind)
	}
	control := mods[1]
	if !control.BuiltIn {
		t.Fatalf("control mod should be marked built-in: %+v", control)
	}
	if control.SyncKind != "server_only" {
		t.Fatalf("control SyncKind = %q, want server_only", control.SyncKind)
	}
}

func TestListMods_SkipsPhysicalSMAPIRuntimeFolder(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".local-container", "mods")
	createTestMod(t, root, controlModFolderName, controlModUniqueID, "Panel Control")
	if err := os.MkdirAll(filepath.Join(root, "smapi", "ConsoleCommands"), 0o755); err != nil {
		t.Fatal(err)
	}

	mods, err := ListMods(dir)
	if err != nil {
		t.Fatalf("ListMods: %v", err)
	}
	if got := modFoldersForTest(mods); strings.Join(got, ",") != "SMAPI,StardewAnxiPanel.Control" {
		t.Fatalf("mod folders = %v, want only virtual SMAPI and control mod", got)
	}
	for _, mod := range mods {
		if mod.FolderName == "smapi" {
			t.Fatalf("physical smapi folder should not be listed: %+v", mod)
		}
	}
}

func TestListMods_MarksJunimoServerBuiltIn(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".local-container", "mods")
	createTestMod(t, root, junimoServerModFolderName, junimoServerModUniqueID, "JunimoServer")

	mods, err := ListMods(dir)
	if err != nil {
		t.Fatalf("ListMods: %v", err)
	}
	if len(mods) != 1 {
		t.Fatalf("expected one mod, got %+v", mods)
	}
	mod := mods[0]
	if !mod.BuiltIn || mod.CanToggle || mod.SyncKind != registry.ModSyncKindServerOnly {
		t.Fatalf("JunimoServer should be built-in server_only: %+v", mod)
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

func TestListModsWithState_AnnotatesDependencyStatus(t *testing.T) {
	dir := t.TempDir()
	root := modsDir(dir)
	disabledRoot := disabledModsDir(dir)
	optional := false
	createTestModWithManifest(t, root, "Consumer", modManifest{
		Name:     "Consumer",
		UniqueID: "Author.Consumer",
		Version:  "1.0.0",
		Author:   "Test",
		Dependencies: []modManifestDependency{
			{UniqueID: "Pathoschild.ContentPatcher", MinimumVersion: "2.0.0"},
			{UniqueID: "Author.DisabledCore"},
			{UniqueID: "Author.MissingCore"},
			{UniqueID: "Author.OptionalMissing", IsRequired: &optional},
		},
	})
	createTestModWithManifest(t, root, "ContentPatcher", modManifest{
		Name:     "Content Patcher",
		UniqueID: "Pathoschild.ContentPatcher",
		Version:  "1.9.0",
		Author:   "Test",
	})
	createTestModWithManifest(t, disabledRoot, "DisabledCore", modManifest{
		Name:     "Disabled Core",
		UniqueID: "Author.DisabledCore",
		Version:  "1.0.0",
		Author:   "Test",
	})

	mods, err := ListModsWithState(dir, "")
	if err != nil {
		t.Fatalf("ListModsWithState: %v", err)
	}
	byUniqueID := map[string]registry.ModInfo{}
	for _, mod := range mods {
		byUniqueID[mod.UniqueID] = mod
	}
	consumer := byUniqueID["Author.Consumer"]
	if len(consumer.Dependencies) != 4 {
		t.Fatalf("dependencies = %+v, want 4", consumer.Dependencies)
	}
	deps := map[string]registry.ModDependency{}
	for _, dep := range consumer.Dependencies {
		deps[dep.UniqueID] = dep
	}
	if dep := deps["Pathoschild.ContentPatcher"]; dep.Status != modDependencyStatusVersionMismatch || dep.Satisfied || dep.InstalledVersion != "1.9.0" {
		t.Fatalf("Content Patcher status = %+v, want version_mismatch with current 1.9.0", dep)
	}
	if dep := deps["Author.DisabledCore"]; dep.Status != modDependencyStatusDisabled || dep.Satisfied || !dep.Installed || dep.Enabled {
		t.Fatalf("DisabledCore status = %+v, want installed disabled", dep)
	}
	if dep := deps["Author.MissingCore"]; dep.Status != modDependencyStatusMissing || dep.Satisfied || dep.Installed {
		t.Fatalf("MissingCore status = %+v, want missing", dep)
	}
	if dep := deps["Author.OptionalMissing"]; dep.Status != modDependencyStatusOptionalMissing || !dep.Satisfied || dep.Installed {
		t.Fatalf("OptionalMissing status = %+v, want optional missing satisfied", dep)
	}
}

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

func TestUploadModZipDetailedSkipsSMAPIBundledSupportMods(t *testing.T) {
	dir := t.TempDir()
	zipPath := createModZip(t, map[string]string{
		"Mods1/ConsoleCommands/manifest.json": `{"Name":"Console Commands","UniqueID":"SMAPI.ConsoleCommands","Version":"4.5.2","Author":"SMAPI"}`,
		"Mods1/SaveBackup/manifest.json":      `{"Name":"Save Backup","UniqueID":"SMAPI.SaveBackup","Version":"4.5.2","Author":"SMAPI"}`,
		"Mods1/SVE/manifest.json":             `{"Name":"SVE","UniqueID":"FlashShifter.SVECode","Version":"1.15.11","Author":"FlashShifter"}`,
	})

	result, err := UploadModZipDetailed(dir, zipPath)
	if err != nil {
		t.Fatalf("UploadModZipDetailed: %v", err)
	}
	if result.Stats.DiscoveredCount != 3 || len(result.Stats.SkippedBuiltInNames) != 2 {
		t.Fatalf("stats = %+v, want discovered 3 and skipped 2", result.Stats)
	}
	if len(result.Mods) != 1 || result.Mods[0].UniqueID != "FlashShifter.SVECode" {
		t.Fatalf("imported mods = %+v, want SVE only", result.Mods)
	}
	for _, folder := range []string{"ConsoleCommands", "SaveBackup"} {
		if _, err := os.Stat(filepath.Join(modsDir(dir), folder)); !os.IsNotExist(err) {
			t.Fatalf("SMAPI bundled folder %q should not be imported: %v", folder, err)
		}
	}
}

func TestQuarantineSMAPIBundledDuplicatesPreservesFiles(t *testing.T) {
	dir := t.TempDir()
	root := modsDir(dir)
	createTestMod(t, filepath.Join(root, "smapi"), "ConsoleCommands", consoleCommandsID, "Managed Console Commands")
	createTestMod(t, filepath.Join(root, "smapi"), "SaveBackup", saveBackupID, "Managed Save Backup")
	createTestMod(t, root, "ConsoleCommands", consoleCommandsID, "User Console Commands")
	createTestMod(t, root, "SaveBackup", saveBackupID, "User Save Backup")
	createTestMod(t, root, "KeepMe", "Author.KeepMe", "Keep Me")

	quarantined, err := QuarantineSMAPIBundledDuplicates(dir)
	if err != nil {
		t.Fatalf("QuarantineSMAPIBundledDuplicates: %v", err)
	}
	if strings.Join(quarantined, ",") != "ConsoleCommands,SaveBackup" {
		t.Fatalf("quarantined = %v", quarantined)
	}
	for _, folder := range quarantined {
		if _, err := os.Stat(filepath.Join(root, folder)); !os.IsNotExist(err) {
			t.Fatalf("duplicate %q remains mounted: %v", folder, err)
		}
		matches, err := filepath.Glob(filepath.Join(dir, ".local-container", "mod-quarantine", "smapi-bundled-duplicates", "*", folder, "manifest.json"))
		if err != nil || len(matches) != 1 {
			t.Fatalf("quarantined %q manifest matches = %v, err=%v", folder, matches, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "KeepMe", "manifest.json")); err != nil {
		t.Fatalf("unrelated mod moved: %v", err)
	}
	if again, err := QuarantineSMAPIBundledDuplicates(dir); err != nil || len(again) != 0 {
		t.Fatalf("second quarantine = %v, err=%v", again, err)
	}
}

func TestQuarantineSMAPIBundledDuplicatesKeepsOnlyAvailableCopy(t *testing.T) {
	dir := t.TempDir()
	createTestMod(t, modsDir(dir), "ConsoleCommands", consoleCommandsID, "Console Commands")

	quarantined, err := QuarantineSMAPIBundledDuplicates(dir)
	if err != nil || len(quarantined) != 0 {
		t.Fatalf("quarantined = %v, err=%v", quarantined, err)
	}
	if _, err := os.Stat(filepath.Join(modsDir(dir), "ConsoleCommands", "manifest.json")); err != nil {
		t.Fatalf("only available copy should remain: %v", err)
	}
}

func TestUploadModZip_RecursivelyImportsModsFolderBundle(t *testing.T) {
	dir := t.TempDir()
	zipPath := createModZip(t, map[string]string{
		"Mods1/DirectMod/manifest.json":                `{"Name":"Direct","UniqueID":"author.direct","Version":"1.0","Author":"A"}`,
		"Mods1/Category/MainMod/manifest.json":         `{"Name":"Main","UniqueID":"author.main","Version":"1.0","Author":"A"}`,
		"Mods1/Category/[CP] Main/manifest.json":       `{"Name":"Content","UniqueID":"author.main.cp","Version":"1.0","Author":"A","ContentPackFor":{"UniqueID":"Pathoschild.ContentPatcher"}}`,
		"Mods1/Deep/Bundle/[CP] Feature/Manifest.json": `{"Name":"Feature","UniqueID":"author.feature","Version":"1.0","Author":"A","ContentPackFor":{"UniqueID":"Pathoschild.ContentPatcher"}}`,
		"Mods1/Deep/Bundle/[CP] Feature/Content.json":  `{}`,
	})

	imported, err := UploadModZip(dir, zipPath)
	if err != nil {
		t.Fatalf("UploadModZip: %v", err)
	}
	if len(imported) != 4 {
		t.Fatalf("expected all 4 recursively discovered mods, got %d: %+v", len(imported), imported)
	}

	root := filepath.Join(dir, ".local-container", "mods")
	for _, folder := range []string{"DirectMod", "MainMod", "[CP] Main", "[CP] Feature"} {
		if _, err := os.Stat(filepath.Join(root, folder, "manifest.json")); err != nil {
			t.Fatalf("canonical manifest for %q not imported: %v", folder, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "[CP] Feature", "content.json")); err != nil {
		t.Fatalf("uppercase Content.json was not normalized: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "Mods1")); !os.IsNotExist(err) {
		t.Fatalf("wrapper directory should not be installed, stat err = %v", err)
	}
}

func TestUploadModZip_ModsCollectionKeepsPackagesAndNexusOriginsSeparate(t *testing.T) {
	dir := t.TempDir()
	zipPath := createModZip(t, map[string]string{
		"Mods1/PackA/MainA/manifest.json":   `{"Name":"Main A","UniqueID":"author.main-a","Version":"1.0","Author":"A","UpdateKeys":["Nexus:100"]}`,
		"Mods1/PackA/[CP] A/manifest.json":  `{"Name":"A Content","UniqueID":"author.content-a","Version":"1.0","Author":"A","ContentPackFor":{"UniqueID":"Pathoschild.ContentPatcher"}}`,
		"Mods1/PackA/ZAddon/manifest.json":  `{"Name":"Z Addon","UniqueID":"author.addon-a","Version":"1.0","Author":"A","UpdateKeys":["Nexus:150"]}`,
		"Mods1/PackB/MainB/manifest.json":   `{"Name":"Main B","UniqueID":"author.main-b","Version":"1.0","Author":"B","UpdateKeys":["Nexus:200"]}`,
		"Mods1/PackB/[FTM] B/manifest.json": `{"Name":"B Farm","UniqueID":"author.farm-b","Version":"1.0","Author":"B","ContentPackFor":{"UniqueID":"Esca.FarmTypeManager"}}`,
		"Mods1/Direct/manifest.json":        `{"Name":"Direct","UniqueID":"author.direct-only","Version":"1.0","Author":"C","UpdateKeys":["Nexus:300"]}`,
	})

	imported, err := UploadModZip(dir, zipPath)
	if err != nil {
		t.Fatalf("UploadModZip: %v", err)
	}
	if len(imported) != 6 {
		t.Fatalf("imported %d mods, want 6", len(imported))
	}
	byFolder := map[string]registry.ModInfo{}
	for _, mod := range imported {
		byFolder[mod.FolderName] = mod
	}
	mainA, contentA := byFolder["MainA"], byFolder["[CP] A"]
	mainB, farmB := byFolder["MainB"], byFolder["[FTM] B"]
	if mainA.PackageKey == "" || mainA.PackageKey != contentA.PackageKey {
		t.Fatalf("PackA package keys = %q and %q, want same non-empty key", mainA.PackageKey, contentA.PackageKey)
	}
	if mainB.PackageKey == "" || mainB.PackageKey != farmB.PackageKey || mainB.PackageKey == mainA.PackageKey {
		t.Fatalf("PackB package keys = %q and %q; PackA = %q", mainB.PackageKey, farmB.PackageKey, mainA.PackageKey)
	}
	if contentA.OriginNexusModID != 100 || farmB.OriginNexusModID != 200 {
		t.Fatalf("component origins = A:%d B:%d, want 100 and 200", contentA.OriginNexusModID, farmB.OriginNexusModID)
	}
	if direct := byFolder["Direct"]; direct.PackageKey != "" || direct.OriginNexusModID != 0 {
		t.Fatalf("direct singleton unexpectedly grouped: %+v", direct)
	}

	if err := DeleteMod(dir, contentA.UniqueID); err != nil {
		t.Fatalf("DeleteMod PackA component: %v", err)
	}
	root := filepath.Join(dir, ".local-container", "mods")
	for _, deleted := range []string{"MainA", "[CP] A", "ZAddon"} {
		if _, err := os.Stat(filepath.Join(root, deleted)); !os.IsNotExist(err) {
			t.Fatalf("PackA folder %q was not deleted: %v", deleted, err)
		}
	}
	for _, kept := range []string{"MainB", "[FTM] B", "Direct"} {
		if _, err := os.Stat(filepath.Join(root, kept)); err != nil {
			t.Fatalf("unrelated folder %q was deleted: %v", kept, err)
		}
	}
}

func TestUploadModZip_DecodesLegacyGBKFolderAndNumericUpdateKey(t *testing.T) {
	dir := t.TempDir()
	archive, err := os.CreateTemp(t.TempDir(), "legacy-gbk-*.zip")
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(archive)
	encodedName, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte("模组合包/配偶助手/manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	header := &zip.FileHeader{Name: string(encodedName), Method: zip.Deflate, NonUTF8: true}
	entry, err := zw.CreateHeader(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte(`{"Name":"配偶助手","UniqueID":"author.spouse","Version":"1.0","Author":"A","UpdateKeys":[48113]}`)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}

	imported, err := UploadModZip(dir, archive.Name())
	if err != nil {
		t.Fatalf("UploadModZip: %v", err)
	}
	if len(imported) != 1 || imported[0].FolderName != "配偶助手" {
		t.Fatalf("unexpected imported mods: %+v", imported)
	}
	if len(imported[0].UpdateKeys) != 1 || imported[0].UpdateKeys[0] != "48113" {
		t.Fatalf("numeric UpdateKeys not normalized: %+v", imported[0].UpdateKeys)
	}
	if _, err := os.Stat(filepath.Join(dir, ".local-container", "mods", "配偶助手", "manifest.json")); err != nil {
		t.Fatalf("decoded GBK folder missing: %v", err)
	}
}

func TestUploadModZip_InvalidDeepManifestRollsBackWholeBundle(t *testing.T) {
	dir := t.TempDir()
	zipPath := createModZip(t, map[string]string{
		"Mods1/Good/manifest.json":           `{"Name":"Good","UniqueID":"author.good","Version":"1.0","Author":"A"}`,
		"Mods1/Category/Bad/manifest.json":   `not json`,
		"Mods1/Category/Bad/some-asset.json": `{}`,
	})

	if _, err := UploadModZip(dir, zipPath); err == nil {
		t.Fatal("expected invalid deep manifest to reject the whole bundle")
	}
	root := filepath.Join(dir, ".local-container", "mods")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read mods root: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("bundle failure left partially imported directories: %+v", entries)
	}
}

func TestUploadModZip_AllowsSingleNexusWrapperWithMultipleMods(t *testing.T) {
	dir := t.TempDir()
	mainManifest := `{
		"Name":"Multiple Construction Orders",
		"UniqueID":"moonslime.MultipleConstructionOrders",
		"Version":"1.1.0",
		"Author":"moonslime",
		"EntryDll":"MultipleConstructionOrders.dll",
		"Dependencies":[{"UniqueID":"moonslime.MultipleConstructionOrders.CP"},{"UniqueID":"Pathoschild.ContentPatcher"}],
		"UpdateKeys":["Nexus:47289"]
	}`
	cpManifest := `{
		"Name":"[CP] Multiple Construction Orders",
		"UniqueID":"moonslime.MultipleConstructionOrders.CP",
		"Version":"1.0.0",
		"Author":"moonslime",
		"ContentPackFor":{"UniqueID":"Pathoschild.ContentPatcher"},
		"Dependencies":[{"UniqueID":"Pathoschild.ContentPatcher"}]
	}`
	zipPath := createModZip(t, map[string]string{
		"MultipleConstructionOrders/MultipleConstructionOrders/manifest.json":                      mainManifest,
		"MultipleConstructionOrders/MultipleConstructionOrders/MultipleConstructionOrders.dll":     "dll",
		"MultipleConstructionOrders/[CP] MultipleConstructionOrders/manifest.json":                 cpManifest,
		"MultipleConstructionOrders/[CP] MultipleConstructionOrders/content.json":                  "{}",
		"MultipleConstructionOrders/[CP] MultipleConstructionOrders/Assets/ConstructionWorker.png": "png",
	})

	imported, err := UploadModZip(dir, zipPath)
	if err != nil {
		t.Fatalf("UploadModZip: %v", err)
	}
	if len(imported) != 2 {
		t.Fatalf("expected 2 imported mods, got %d", len(imported))
	}
	byFolder := map[string]registry.ModInfo{}
	for _, mod := range imported {
		byFolder[mod.FolderName] = mod
	}
	if main := byFolder["MultipleConstructionOrders"]; main.NexusModID != 47289 {
		t.Fatalf("main NexusModID = %d, want 47289", main.NexusModID)
	}
	cp := byFolder["[CP] MultipleConstructionOrders"]
	if cp.NexusModID != 0 {
		t.Fatalf("content pack NexusModID = %d, want 0", cp.NexusModID)
	}
	if cp.OriginSource != "nexus" || cp.OriginNexusModID != 47289 || cp.OriginModName != "Multiple Construction Orders" {
		t.Fatalf("content pack origin = source %q id %d name %q, want Nexus package 47289",
			cp.OriginSource, cp.OriginNexusModID, cp.OriginModName)
	}

	root := filepath.Join(dir, ".local-container", "mods")
	for _, folder := range []string{"MultipleConstructionOrders", "[CP] MultipleConstructionOrders"} {
		if _, err := os.Stat(filepath.Join(root, folder, "manifest.json")); err != nil {
			t.Fatalf("manifest for %q not imported at mods root: %v", folder, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "MultipleConstructionOrders", "MultipleConstructionOrders", "manifest.json")); !os.IsNotExist(err) {
		t.Fatalf("wrapper directory should be stripped, nested manifest stat err = %v", err)
	}

	listed, err := ListMods(dir)
	if err != nil {
		t.Fatalf("ListMods: %v", err)
	}
	listed = ApplyNexusMetadataToMods(dir, listed)
	byFolder = map[string]registry.ModInfo{}
	for _, mod := range listed {
		byFolder[mod.FolderName] = mod
	}
	cp = byFolder["[CP] MultipleConstructionOrders"]
	if cp.NexusModID != 0 || cp.OriginNexusModID != 47289 {
		t.Fatalf("persisted content pack ids = own %d origin %d, want own 0 origin 47289",
			cp.NexusModID, cp.OriginNexusModID)
	}

	if err := SaveInstalledNexusMetadata(dir, []registry.ModInfo{byFolder["MultipleConstructionOrders"]}, NexusModSearchResult{
		ModID:      47289,
		Name:       "Multiple Construction Orders",
		PictureURL: "https://example.com/mco.png",
		NexusURL:   "https://www.nexusmods.com/stardewvalley/mods/47289",
	}); err != nil {
		t.Fatalf("SaveInstalledNexusMetadata: %v", err)
	}
	listed, err = ListMods(dir)
	if err != nil {
		t.Fatalf("ListMods after metadata: %v", err)
	}
	listed = ApplyNexusMetadataToMods(dir, listed)
	byFolder = map[string]registry.ModInfo{}
	for _, mod := range listed {
		byFolder[mod.FolderName] = mod
	}
	cp = byFolder["[CP] MultipleConstructionOrders"]
	if cp.PictureURL != "https://example.com/mco.png" {
		t.Fatalf("content pack PictureURL = %q, want package thumbnail", cp.PictureURL)
	}
}

func TestSaveInstalledNexusMetadata_CorrectsMismatchedBatchResultFromPackageUpdateKey(t *testing.T) {
	dir := t.TempDir()
	smapiManifest := `{
		"Name":"Ridgeside Village [SMAPI component]",
		"UniqueID":"Rafseazz.RidgesideVillage",
		"Version":"2.5.17",
		"Author":"Rafseazz",
		"UpdateKeys":["Nexus:-1"]
	}`
	ccManifest := `{
		"Name":"Ridgeside Village [Custom Companions component]",
		"UniqueID":"Rafseazz.RSVCC",
		"Version":"2.5.17",
		"Author":"Rafseazz",
		"UpdateKeys":["Nexus:-1"],
		"ContentPackFor":{"UniqueID":"PeacefulEnd.CustomCompanions"}
	}`
	ftmManifest := `{
		"Name":"Ridgeside Village [Farm Type Manager component]",
		"UniqueID":"Rafseazz.RSVFTM",
		"Version":"2.5.17",
		"Author":"Rafseazz",
		"UpdateKeys":["Nexus:-1"],
		"ContentPackFor":{"UniqueID":"Esca.FarmTypeManager"}
	}`
	cpManifest := `{
		"Name":"Ridgeside Village [Content Patcher component]",
		"UniqueID":"Rafseazz.RSVCP",
		"Version":"2.5.17",
		"Author":"Rafseazz",
		"UpdateKeys":["Nexus:7286"],
		"ContentPackFor":{"UniqueID":"Pathoschild.ContentPatcher"}
	}`
	zipPath := createModZip(t, map[string]string{
		"Ridgeside/RidgesideVillage/manifest.json":        smapiManifest,
		"Ridgeside/[CC] Ridgeside Village/manifest.json":  ccManifest,
		"Ridgeside/[FTM] Ridgeside Village/manifest.json": ftmManifest,
		"Ridgeside/[CP] Ridgeside Village/manifest.json":  cpManifest,
		"Ridgeside/[CP] Ridgeside Village/content.json":   "{}",
		"Ridgeside/[CC] Ridgeside Village/content.json":   "{}",
		"Ridgeside/[FTM] Ridgeside Village/content.json":  "{}",
		"Ridgeside/RidgesideVillage/RidgesideVillage.dll": "dll",
	})

	imported, err := uploadModZip(dir, zipPath, uploadModZipOptions{inferNexusPackageOrigin: false})
	if err != nil {
		t.Fatalf("uploadModZip: %v", err)
	}
	if err := SaveInstalledNexusMetadata(dir, imported, NexusModSearchResult{
		ModID:    1348,
		Name:     "SpaceCore",
		Author:   "spacechase0",
		Version:  "1.28.4",
		NexusURL: nexusModURL(1348),
	}); err != nil {
		t.Fatalf("SaveInstalledNexusMetadata: %v", err)
	}

	listed, err := ListMods(dir)
	if err != nil {
		t.Fatalf("ListMods: %v", err)
	}
	listed = ApplyNexusMetadataToMods(dir, listed)
	byFolder := map[string]registry.ModInfo{}
	for _, mod := range listed {
		byFolder[mod.FolderName] = mod
	}
	for _, folder := range []string{"RidgesideVillage", "[CC] Ridgeside Village", "[FTM] Ridgeside Village"} {
		mod := byFolder[folder]
		if mod.OriginNexusModID != 7286 || mod.OriginModName != "Ridgeside Village [Content Patcher component]" {
			t.Fatalf("%s origin = id %d name %q, want Ridgeside package 7286",
				folder, mod.OriginNexusModID, mod.OriginModName)
		}
	}
	if cp := byFolder["[CP] Ridgeside Village"]; cp.NexusModID != 7286 || cp.OriginNexusModID != 0 {
		t.Fatalf("CP component ids = own %d origin %d, want own 7286 only", cp.NexusModID, cp.OriginNexusModID)
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
		"TestMod/":              "",
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

func TestUploadModZip_RejectsXNBReplacementWithHelpfulError(t *testing.T) {
	dir := t.TempDir()
	zipPath := createModZip(t, map[string]string{
		"I was thinking about dyeing my hair again/Dye it black/Characters/Abigail.xnb": "xnb",
		"I was thinking about dyeing my hair again/Dye it black/Portraits/Abigail.xnb":  "xnb",
	})

	_, err := UploadModZip(dir, zipPath)
	if err == nil {
		t.Fatal("expected error for XNB replacement archive")
	}
	if !strings.Contains(err.Error(), "XNB") || !strings.Contains(err.Error(), "SMAPI") {
		t.Fatalf("expected helpful XNB/SMAPI error, got %v", err)
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

func TestUploadModZip_AllowsAlreadyInstalledWhenRemoteInstallIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	manifest := `{"Name":"Test Mod","UniqueID":"author.testmod","Version":"1.0.0","Author":"Tester"}`

	zipPath1 := createModZip(t, map[string]string{"Mod1/manifest.json": manifest})
	if _, err := UploadModZip(dir, zipPath1); err != nil {
		t.Fatalf("first upload: %v", err)
	}

	zipPath2 := createModZip(t, map[string]string{"Mod2/manifest.json": manifest})
	imported, err := uploadModZip(dir, zipPath2, uploadModZipOptions{allowAlreadyInstalled: true})
	if err != nil {
		t.Fatalf("idempotent remote upload should skip already installed mod: %v", err)
	}
	if len(imported) != 0 {
		t.Fatalf("imported duplicate mods = %d, want 0", len(imported))
	}
	if _, err := os.Stat(filepath.Join(dir, ".local-container", "mods", "Mod2")); !os.IsNotExist(err) {
		t.Fatalf("duplicate folder should not be imported, stat err = %v", err)
	}
}

func TestUploadModZip_ImportsMissingModsWhenRemotePackagePartlyExists(t *testing.T) {
	dir := t.TempDir()
	existingManifest := `{"Name":"Existing Mod","UniqueID":"author.existing","Version":"1.0.0","Author":"Tester"}`
	newManifest := `{"Name":"New Mod","UniqueID":"author.new","Version":"1.0.0","Author":"Tester"}`

	zipPath1 := createModZip(t, map[string]string{"ExistingMod/manifest.json": existingManifest})
	if _, err := UploadModZip(dir, zipPath1); err != nil {
		t.Fatalf("first upload: %v", err)
	}

	zipPath2 := createModZip(t, map[string]string{
		"ExistingModFromPackage/manifest.json": existingManifest,
		"NewMod/manifest.json":                 newManifest,
	})
	imported, err := uploadModZip(dir, zipPath2, uploadModZipOptions{allowAlreadyInstalled: true})
	if err != nil {
		t.Fatalf("partly installed remote package should import missing mods: %v", err)
	}
	if got := modFoldersForTest(imported); strings.Join(got, ",") != "NewMod" {
		t.Fatalf("imported folders = %v, want only NewMod", got)
	}
	if _, err := os.Stat(filepath.Join(dir, ".local-container", "mods", "NewMod", "manifest.json")); err != nil {
		t.Fatalf("new mod should be imported: %v", err)
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

func TestDeleteMod_RemovesNexusPackageBundle(t *testing.T) {
	dir := t.TempDir()
	mainManifest := `{
		"Name":"Multiple Construction Orders",
		"UniqueID":"moonslime.MultipleConstructionOrders",
		"Version":"1.1.0",
		"Author":"moonslime",
		"UpdateKeys":["Nexus:47289"]
	}`
	cpManifest := `{
		"Name":"[CP] Multiple Construction Orders",
		"UniqueID":"moonslime.MultipleConstructionOrders.CP",
		"Version":"1.0.0",
		"Author":"moonslime",
		"ContentPackFor":{"UniqueID":"Pathoschild.ContentPatcher"}
	}`
	zipPath := createModZip(t, map[string]string{
		"MultipleConstructionOrders/MultipleConstructionOrders/manifest.json":      mainManifest,
		"MultipleConstructionOrders/[CP] MultipleConstructionOrders/manifest.json": cpManifest,
	})
	if _, err := UploadModZip(dir, zipPath); err != nil {
		t.Fatalf("UploadModZip: %v", err)
	}

	if err := DeleteMod(dir, "[CP] MultipleConstructionOrders"); err != nil {
		t.Fatalf("DeleteMod bundle member: %v", err)
	}

	root := filepath.Join(dir, ".local-container", "mods")
	for _, folder := range []string{"MultipleConstructionOrders", "[CP] MultipleConstructionOrders"} {
		if _, err := os.Stat(filepath.Join(root, folder)); !os.IsNotExist(err) {
			t.Fatalf("folder %q should have been deleted with bundle, stat err = %v", folder, err)
		}
	}
	store, err := loadNexusMetadataStore(dir)
	if err != nil {
		t.Fatalf("loadNexusMetadataStore: %v", err)
	}
	if len(store.Mods) != 0 {
		t.Fatalf("nexus sidecar entries = %+v, want empty after bundle delete", store.Mods)
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

func TestDeleteMod_CannotDeleteControlMod(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".local-container", "mods")
	createTestMod(t, root, controlModFolderName, controlModUniqueID, "Panel Control")

	if err := DeleteMod(dir, controlModFolderName); err == nil {
		t.Fatal("expected error when deleting built-in control mod")
	}
	if _, err := os.Stat(filepath.Join(root, controlModFolderName)); err != nil {
		t.Fatalf("control mod folder should remain: %v", err)
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
		"Description": "A test mod",
		"Dependencies": [
			{"UniqueID": "Pathoschild.ContentPatcher", "MinimumVersion": "2.0.0"},
			{"UniqueID": "spacechase0.GenericModConfigMenu", "IsRequired": false}
		]
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
	if len(info.Dependencies) != 2 {
		t.Fatalf("len(Dependencies) = %d, want 2: %+v", len(info.Dependencies), info.Dependencies)
	}
	if info.Dependencies[0].UniqueID != "Pathoschild.ContentPatcher" ||
		info.Dependencies[0].MinimumVersion != "2.0.0" ||
		!info.Dependencies[0].Required {
		t.Errorf("Dependencies[0] = %+v, want required Content Patcher min 2.0.0", info.Dependencies[0])
	}
	if info.Dependencies[1].UniqueID != "spacechase0.GenericModConfigMenu" || info.Dependencies[1].Required {
		t.Errorf("Dependencies[1] = %+v, want optional GMCM", info.Dependencies[1])
	}
}

func TestReadModInfo_ContentPackForAsDependency(t *testing.T) {
	dir := t.TempDir()
	modPath := filepath.Join(dir, "CPMod")
	if err := os.MkdirAll(modPath, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{
		"Name": "[CP] Example",
		"UniqueID": "author.cp",
		"Version": "1.0.0",
		"ContentPackFor": {"UniqueID": "Pathoschild.ContentPatcher", "MinimumVersion": "2.6.0"},
		"Dependencies": [{"UniqueID": "Pathoschild.ContentPatcher"}]
	}`
	if err := os.WriteFile(filepath.Join(modPath, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	info := readModInfo(modPath, "CPMod")
	if len(info.Dependencies) != 1 {
		t.Fatalf("len(Dependencies) = %d, want deduped Content Patcher dependency: %+v", len(info.Dependencies), info.Dependencies)
	}
	dep := info.Dependencies[0]
	if dep.UniqueID != "Pathoschild.ContentPatcher" || dep.MinimumVersion != "2.6.0" || !dep.Required {
		t.Fatalf("dependency = %+v, want required Content Patcher min 2.6.0", dep)
	}
	if !info.IsContentPack || info.ContentPackFor != "Pathoschild.ContentPatcher" {
		t.Fatalf("content pack fields = isContentPack %v contentPackFor %q, want Content Patcher",
			info.IsContentPack, info.ContentPackFor)
	}
}

func TestReadModInfo_AllowsUTF8BOMManifest(t *testing.T) {
	dir := t.TempDir()
	modPath := filepath.Join(dir, "BOMMod")
	if err := os.MkdirAll(modPath, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := "\ufeff" + `{
		"Name": "BOM Mod",
		"UniqueID": "author.bom",
		"Version": "1.0.0"
	}`
	if err := os.WriteFile(filepath.Join(modPath, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	info := readModInfo(modPath, "BOMMod")
	if info.ParseError != "" {
		t.Fatalf("ParseError = %q, want empty", info.ParseError)
	}
	if info.UniqueID != "author.bom" || info.Name != "BOM Mod" {
		t.Fatalf("info = %+v, want BOM manifest parsed", info)
	}
}

func TestReadModInfo_AllowsJSONCManifest(t *testing.T) {
	dir := t.TempDir()
	modPath := filepath.Join(dir, "JSONCMod")
	if err := os.MkdirAll(modPath, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{
		// Some Nexus mods ship comments in manifest.json.
		"Name": "JSONC Mod",
		"UniqueID": "author.jsonc",
		"Version": "1.0.0",
		"Description": "URL should survive: https://example.com/mod",
		/* trailing commas should also be accepted */
		"UpdateKeys": [
			"Nexus:1348",
		],
	}`
	if err := os.WriteFile(filepath.Join(modPath, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	info := readModInfo(modPath, "JSONCMod")
	if info.ParseError != "" {
		t.Fatalf("ParseError = %q, want empty", info.ParseError)
	}
	if info.UniqueID != "author.jsonc" || info.Name != "JSONC Mod" || info.NexusModID != 1348 {
		t.Fatalf("info = %+v, want JSONC manifest parsed", info)
	}
	if !strings.Contains(info.Description, "https://example.com/mod") {
		t.Fatalf("Description = %q, want URL string preserved", info.Description)
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
