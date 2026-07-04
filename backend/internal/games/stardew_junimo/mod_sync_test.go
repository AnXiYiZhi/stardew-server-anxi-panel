package stardew_junimo

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
)

// ── Classification file read/write ──────────────────────────────────────────

func TestGetModSyncClassification_DefaultClientRequired(t *testing.T) {
	dir := t.TempDir()
	kind, note := GetModSyncClassification(dir, "SomeMod")
	if kind != registry.ModSyncKindClientRequired {
		t.Errorf("kind = %q, want %q", kind, registry.ModSyncKindClientRequired)
	}
	if note != "" {
		t.Errorf("note = %q, want empty", note)
	}
}

func TestGetModSyncClassification_ControlModDefaultsServerOnly(t *testing.T) {
	dir := t.TempDir()
	kind, _ := GetModSyncClassification(dir, controlModFolderName)
	if kind != registry.ModSyncKindServerOnly {
		t.Errorf("kind = %q, want %q", kind, registry.ModSyncKindServerOnly)
	}
}

func TestSetModSyncClassification_PersistsAndReads(t *testing.T) {
	dir := t.TempDir()
	if err := SetModSyncClassification(dir, "MyMod", registry.ModSyncKindClientRequired, "需要客户端安装"); err != nil {
		t.Fatalf("SetModSyncClassification: %v", err)
	}
	kind, note := GetModSyncClassification(dir, "MyMod")
	if kind != registry.ModSyncKindClientRequired {
		t.Errorf("kind = %q, want %q", kind, registry.ModSyncKindClientRequired)
	}
	if note != "需要客户端安装" {
		t.Errorf("note = %q, want 需要客户端安装", note)
	}

	// Verify the file was written under .local-container/control/, not in the mod folder.
	path := filepath.Join(dir, ".local-container", "control", "mod-sync.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected mod-sync.json at %s: %v", path, err)
	}
}

func TestSetModSyncClassification_RejectsInvalidKind(t *testing.T) {
	dir := t.TempDir()
	if err := SetModSyncClassification(dir, "MyMod", "not_a_real_kind", ""); err == nil {
		t.Fatal("expected error for invalid sync kind")
	}
}

func TestSetModSyncClassification_DoesNotTouchManifest(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".local-container", "mods")
	createTestMod(t, root, "MyMod", "author.mymod", "My Mod")
	manifestPath := filepath.Join(root, "MyMod", "manifest.json")
	before, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}

	if err := SetModSyncClassification(dir, "MyMod", registry.ModSyncKindClientRequired, ""); err != nil {
		t.Fatalf("SetModSyncClassification: %v", err)
	}

	after, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("manifest.json should not be modified by sync classification")
	}
}

func TestGetModSyncClassification_IgnoresInvalidStoredKind(t *testing.T) {
	dir := t.TempDir()
	path := modSyncFilePath(dir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data := `{"mods":{"MyMod":{"syncKind":"bogus"}}}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	kind, _ := GetModSyncClassification(dir, "MyMod")
	if kind != registry.ModSyncKindClientRequired {
		t.Errorf("kind = %q, want fallback to %q", kind, registry.ModSyncKindClientRequired)
	}
}

// ── ApplyModSyncClassification / BuildModSyncPlan ───────────────────────────

func TestApplyModSyncClassification_DefaultsClientRequiredForUnclassifiedMods(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".local-container", "mods")
	createTestMod(t, root, "ModA", "author.a", "Mod A")

	mods, err := ListMods(dir)
	if err != nil {
		t.Fatal(err)
	}
	mods = ApplyModSyncClassification(dir, mods)
	if len(mods) != 1 {
		t.Fatalf("expected 1 mod, got %d", len(mods))
	}
	if mods[0].SyncKind != registry.ModSyncKindClientRequired {
		t.Errorf("SyncKind = %q, want %q", mods[0].SyncKind, registry.ModSyncKindClientRequired)
	}
	if mods[0].SyncNote == "" {
		t.Fatal("expected automatic sync note")
	}
}

func TestApplyModSyncClassification_ContentPackDefaultsClientRequired(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".local-container", "mods")
	modPath := filepath.Join(root, "[CP] Example")
	if err := os.MkdirAll(modPath, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{
		"Name":"[CP] Example",
		"UniqueID":"author.cp",
		"Version":"1.0.0",
		"ContentPackFor":{"UniqueID":"Pathoschild.ContentPatcher"}
	}`
	if err := os.WriteFile(filepath.Join(modPath, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	mods, err := ListMods(dir)
	if err != nil {
		t.Fatal(err)
	}
	mods = ApplyModSyncClassification(dir, mods)
	if len(mods) != 1 {
		t.Fatalf("expected 1 mod, got %d", len(mods))
	}
	if mods[0].SyncKind != registry.ModSyncKindClientRequired {
		t.Errorf("SyncKind = %q, want %q", mods[0].SyncKind, registry.ModSyncKindClientRequired)
	}
	if mods[0].ContentPackFor != "Pathoschild.ContentPatcher" || !mods[0].IsContentPack {
		t.Fatalf("content pack fields not populated: %+v", mods[0])
	}
	if mods[0].SyncNote != "自动识别：内容包需要玩家同步" {
		t.Errorf("SyncNote = %q, want content-pack auto note", mods[0].SyncNote)
	}
}

func TestBuildModSyncPlan_Summary(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".local-container", "mods")
	createTestMod(t, root, "ModA", "author.a", "Mod A")
	createTestMod(t, root, "ModB", "author.b", "Mod B")
	createTestMod(t, root, controlModFolderName, "anxi.control", "Control Mod")

	if err := SetModSyncClassification(dir, "ModA", registry.ModSyncKindClientRequired, ""); err != nil {
		t.Fatal(err)
	}

	plan, err := BuildModSyncPlan(dir)
	if err != nil {
		t.Fatalf("BuildModSyncPlan: %v", err)
	}
	if plan.Summary.Total != 3 {
		t.Errorf("Total = %d, want 3", plan.Summary.Total)
	}
	if plan.Summary.ClientRequired != 3 {
		t.Errorf("ClientRequired = %d, want 3", plan.Summary.ClientRequired)
	}
	if plan.Summary.ServerOnly != 0 {
		t.Errorf("ServerOnly = %d, want 0 (control mod is built-in and excluded)", plan.Summary.ServerOnly)
	}
	if plan.Summary.Unknown != 0 {
		t.Errorf("Unknown = %d, want 0", plan.Summary.Unknown)
	}
}

func TestSetModSyncClassificationCascadeUpdatesDependencyComponent(t *testing.T) {
	dir := t.TempDir()
	root := modsDir(dir)
	createTestModWithManifest(t, root, "Framework", modManifest{
		Name:     "Framework",
		UniqueID: "author.framework",
		Version:  "1.0.0",
		Author:   "Test",
	})
	createTestModWithManifest(t, root, "Core", modManifest{
		Name:     "Core",
		UniqueID: "author.core",
		Version:  "1.0.0",
		Author:   "Test",
		Dependencies: []modManifestDependency{
			{UniqueID: "author.framework"},
		},
	})
	createTestModWithManifest(t, root, "Content", modManifest{
		Name:     "Content",
		UniqueID: "author.content",
		Version:  "1.0.0",
		Author:   "Test",
		Dependencies: []modManifestDependency{
			{UniqueID: "author.core"},
		},
	})

	affected, err := SetModSyncClassificationCascade(dir, "author.core", registry.ModSyncKindUnknown, "")
	if err != nil {
		t.Fatalf("unknown cascade: %v", err)
	}
	if got := modFoldersForTest(affected); strings.Join(got, ",") != "Content,Core,Framework" {
		t.Fatalf("unknown cascade affected %v, want Content/Core/Framework", got)
	}
	for _, folder := range []string{"Content", "Core", "Framework"} {
		kind, _ := GetModSyncClassification(dir, folder)
		if kind != registry.ModSyncKindUnknown {
			t.Fatalf("%s kind = %q, want unknown", folder, kind)
		}
	}

	affected, err = SetModSyncClassificationCascade(dir, "author.core", registry.ModSyncKindClientRequired, "")
	if err != nil {
		t.Fatalf("client cascade: %v", err)
	}
	if got := modFoldersForTest(affected); strings.Join(got, ",") != "Content,Core,Framework" {
		t.Fatalf("client cascade affected %v, want Content/Core/Framework", got)
	}
	for _, folder := range []string{"Content", "Core", "Framework"} {
		kind, _ := GetModSyncClassification(dir, folder)
		if kind != registry.ModSyncKindClientRequired {
			t.Fatalf("%s kind = %q, want client_required", folder, kind)
		}
	}
}

// ── ResolveModFolder ─────────────────────────────────────────────────────────

func TestResolveModFolder_ByFolderName(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".local-container", "mods")
	createTestMod(t, root, "TestMod", "author.test", "Test")

	folder, err := ResolveModFolder(dir, "TestMod")
	if err != nil {
		t.Fatalf("ResolveModFolder: %v", err)
	}
	if folder != "TestMod" {
		t.Errorf("folder = %q, want TestMod", folder)
	}
}

func TestResolveModFolder_ByUniqueID(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".local-container", "mods")
	createTestMod(t, root, "TestMod", "author.test", "Test")

	folder, err := ResolveModFolder(dir, "author.test")
	if err != nil {
		t.Fatalf("ResolveModFolder: %v", err)
	}
	if folder != "TestMod" {
		t.Errorf("folder = %q, want TestMod", folder)
	}
}

func TestResolveModFolder_NotFound(t *testing.T) {
	dir := t.TempDir()
	if _, err := ResolveModFolder(dir, "NonExistent"); err == nil {
		t.Fatal("expected error for non-existent mod")
	}
}

func TestResolveModFolder_RejectsPathTraversal(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"../evil", "foo/bar", `foo\bar`, "/absolute", ".."} {
		if _, err := ResolveModFolder(dir, name); err == nil {
			t.Fatalf("expected error for mod id %q", name)
		}
	}
}

// ── ExportModSyncPackZip ─────────────────────────────────────────────────────

func TestExportModSyncPackZip_OnlyClientRequired(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".local-container", "mods")
	createTestMod(t, root, "ClientMod", "author.client", "Client Mod")
	createTestMod(t, root, "ServerMod", "author.server", "Server Mod")
	createTestMod(t, root, "UnknownMod", "author.unknown", "Unknown Mod")

	if err := SetModSyncClassification(dir, "ClientMod", registry.ModSyncKindClientRequired, ""); err != nil {
		t.Fatal(err)
	}
	if err := SetModSyncClassification(dir, "ServerMod", registry.ModSyncKindServerOnly, ""); err != nil {
		t.Fatal(err)
	}
	if err := SetModSyncClassification(dir, "UnknownMod", registry.ModSyncKindUnknown, ""); err != nil {
		t.Fatal(err)
	}

	zipPath, err := ExportModSyncPackZip(dir)
	if err != nil {
		t.Fatalf("ExportModSyncPackZip: %v", err)
	}
	defer func() { _ = os.Remove(zipPath) }()

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open exported zip: %v", err)
	}
	defer func() { _ = zr.Close() }()

	seenFolders := map[string]bool{}
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "payload/mods/") {
			parts := strings.Split(strings.TrimPrefix(f.Name, "payload/mods/"), "/")
			if len(parts) > 0 && parts[0] != "" {
				seenFolders[parts[0]] = true
			}
		}
	}

	if !seenFolders["ClientMod"] {
		t.Error("expected ClientMod in export")
	}
	if seenFolders["ServerMod"] {
		t.Error("ServerMod should not be in export")
	}
	if seenFolders["UnknownMod"] {
		t.Error("UnknownMod should not be in export")
	}
}

func TestExportModSyncPackZip_ExcludesControlMod(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".local-container", "mods")
	createTestMod(t, root, "ClientMod", "author.client", "Client Mod")
	createTestMod(t, root, controlModFolderName, "anxi.control", "Control Mod")

	if err := SetModSyncClassification(dir, "ClientMod", registry.ModSyncKindClientRequired, ""); err != nil {
		t.Fatal(err)
	}
	// Force-classify the control mod as client_required too — export must still exclude it.
	if err := SetModSyncClassification(dir, controlModFolderName, registry.ModSyncKindClientRequired, ""); err != nil {
		t.Fatal(err)
	}

	zipPath, err := ExportModSyncPackZip(dir)
	if err != nil {
		t.Fatalf("ExportModSyncPackZip: %v", err)
	}
	defer func() { _ = os.Remove(zipPath) }()

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open exported zip: %v", err)
	}
	defer func() { _ = zr.Close() }()

	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "payload/mods/"+controlModFolderName+"/") {
			t.Fatalf("export must not contain %s, found entry %q", controlModFolderName, f.Name)
		}
	}
}

func TestExportModSyncPackZip_NoMods(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".local-container", "mods")
	createTestMod(t, root, "ServerMod", "author.server", "Server Mod")
	if err := SetModSyncClassification(dir, "ServerMod", registry.ModSyncKindServerOnly, ""); err != nil {
		t.Fatal(err)
	}

	_, err := ExportModSyncPackZip(dir)
	if err == nil {
		t.Fatal("expected error when no client_required mods exist")
	}
}

func TestExportModSyncPackZip_IncludesBuiltInSMAPIInManifestOnly(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".local-container", "mods")
	createTestMod(t, root, controlModFolderName, "StardewAnxiPanel.Control", "Panel Control")

	mods, err := ListMods(dir)
	if err != nil {
		t.Fatalf("ListMods: %v", err)
	}
	if len(mods) == 0 || !mods[0].BuiltIn {
		t.Fatalf("expected built-in SMAPI runtime in list: %+v", mods)
	}

	zipPath, err := ExportModSyncPackZip(dir)
	if err != nil {
		t.Fatalf("ExportModSyncPackZip: %v", err)
	}
	defer func() { _ = os.Remove(zipPath) }()

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open exported zip: %v", err)
	}
	defer func() { _ = zr.Close() }()

	var manifestFile *zip.File
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "payload/mods/"+controlModFolderName+"/") {
			t.Fatalf("export must not contain %s, found entry %q", controlModFolderName, f.Name)
		}
		if f.Name == "pack-manifest.json" {
			manifestFile = f
		}
	}
	if manifestFile == nil {
		t.Fatal("expected pack-manifest.json in export")
	}

	rc, err := manifestFile.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rc.Close() }()

	var manifest playerSyncPackManifest
	if err := json.NewDecoder(rc).Decode(&manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	if len(manifest.Mods) != 1 {
		t.Fatalf("expected only SMAPI in manifest, got %+v", manifest.Mods)
	}
	m := manifest.Mods[0]
	if m.UniqueID != "Pathoschild.SMAPI" || !m.BuiltIn || m.Packaged {
		t.Fatalf("unexpected SMAPI manifest entry: %+v", m)
	}
}

func TestExportModSyncPackZip_IncludesManifest(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".local-container", "mods")
	createTestMod(t, root, "ClientMod", "author.client", "Client Mod")
	if err := SetModSyncClassification(dir, "ClientMod", registry.ModSyncKindClientRequired, ""); err != nil {
		t.Fatal(err)
	}

	zipPath, err := ExportModSyncPackZip(dir)
	if err != nil {
		t.Fatalf("ExportModSyncPackZip: %v", err)
	}
	defer func() { _ = os.Remove(zipPath) }()

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open exported zip: %v", err)
	}
	defer func() { _ = zr.Close() }()

	var manifestFile *zip.File
	for _, f := range zr.File {
		if f.Name == "pack-manifest.json" {
			manifestFile = f
		}
	}
	if manifestFile == nil {
		t.Fatal("expected pack-manifest.json in export")
	}

	rc, err := manifestFile.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rc.Close() }()

	var manifest playerSyncPackManifest
	if err := json.NewDecoder(rc).Decode(&manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	if manifest.ExportedAt == "" {
		t.Error("expected non-empty exportedAt")
	}
	if len(manifest.Mods) != 1 {
		t.Fatalf("expected 1 mod in manifest, got %d", len(manifest.Mods))
	}
	m := manifest.Mods[0]
	if m.UniqueID != "author.client" || m.Name != "Client Mod" || m.FolderName != "ClientMod" {
		t.Errorf("unexpected manifest mod entry: %+v", m)
	}
	if m.SyncKind != registry.ModSyncKindClientRequired {
		t.Errorf("SyncKind = %q, want %q", m.SyncKind, registry.ModSyncKindClientRequired)
	}
	if !m.Packaged {
		t.Error("ClientMod should be marked as packaged")
	}
	if manifest.PackVersion != playerSyncPackVersion || manifest.PackType != playerSyncPackTypeFull || manifest.ChecksumFile != "checksums.sha256" {
		t.Fatalf("unexpected pack metadata: %+v", manifest)
	}
	if !manifest.SMAPI.Required || manifest.SMAPI.UniqueID != "Pathoschild.SMAPI" {
		t.Fatalf("unexpected SMAPI metadata: %+v", manifest.SMAPI)
	}
}

func TestExportModSyncPackZip_IncludesInstallerStructureAndValidChecksums(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".local-container", "mods")
	createTestMod(t, root, "ClientMod", "author.client", "Client Mod")

	zipPath, err := ExportModSyncPackZip(dir)
	if err != nil {
		t.Fatalf("ExportModSyncPackZip: %v", err)
	}
	defer func() { _ = os.Remove(zipPath) }()

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open exported zip: %v", err)
	}
	defer func() { _ = zr.Close() }()

	required := []string{
		"安装玩家同步包.bat",
		"卸载本同步包.bat",
		"README.txt",
		"pack-manifest.json",
		"checksums.sha256",
		"tools/install.ps1",
		"tools/uninstall.ps1",
		"tools/steam-launch-options.ps1",
		"tools/vdf.ps1",
		"payload/smapi/smapi.json",
		"payload/mods/ClientMod/manifest.json",
	}
	for _, name := range required {
		if findZipFile(zr.File, name) == nil {
			t.Fatalf("expected zip entry %q", name)
		}
	}

	checksumFile := findZipFile(zr.File, "checksums.sha256")
	checksums := readZipFileString(t, checksumFile)
	if !strings.Contains(checksums, "payload/mods/ClientMod/manifest.json") {
		t.Fatalf("checksums.sha256 missing mod manifest entry:\n%s", checksums)
	}
	for _, line := range strings.Split(checksums, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) != 2 {
			t.Fatalf("invalid checksum line %q", line)
		}
		file := findZipFile(zr.File, parts[1])
		if file == nil {
			t.Fatalf("checksum references missing file %q", parts[1])
		}
		data := readZipFileBytes(t, file)
		actual := fmt.Sprintf("%x", sha256.Sum256(data))
		if actual != parts[0] {
			t.Fatalf("checksum for %s = %s, want %s", parts[1], actual, parts[0])
		}
	}
}

func TestExportModSyncUpdatePackZip_ExcludesSMAPIBundle(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".local-container", "mods")
	createTestMod(t, root, "ClientMod", "author.client", "Client Mod")
	smapiDir := filepath.Join(dir, ".local-container", "smapi")
	if err := os.MkdirAll(smapiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(smapiDir, "SMAPI-4.5.2-installer.zip"), []byte("dummy smapi zip"), 0o644); err != nil {
		t.Fatal(err)
	}

	zipPath, err := ExportModSyncUpdatePackZip(dir)
	if err != nil {
		t.Fatalf("ExportModSyncUpdatePackZip: %v", err)
	}
	defer func() { _ = os.Remove(zipPath) }()

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open exported zip: %v", err)
	}
	defer func() { _ = zr.Close() }()

	required := []string{
		"安装模组更新.bat",
		"卸载本次模组更新.bat",
		"README.txt",
		"pack-manifest.json",
		"checksums.sha256",
		"tools/install.ps1",
		"payload/mods/ClientMod/manifest.json",
	}
	for _, name := range required {
		if findZipFile(zr.File, name) == nil {
			t.Fatalf("expected update pack entry %q", name)
		}
	}
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "payload/smapi/") {
			t.Fatalf("update pack must not contain SMAPI payload entry %q", f.Name)
		}
		if f.Name == "tools/steam-launch-options.ps1" {
			t.Fatalf("update pack must not contain Steam launch options helper")
		}
	}

	var manifest playerSyncPackManifest
	if err := json.Unmarshal(readZipFileBytes(t, findZipFile(zr.File, "pack-manifest.json")), &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	if manifest.PackType != playerSyncPackTypeModsUpdate {
		t.Fatalf("PackType = %q, want %q", manifest.PackType, playerSyncPackTypeModsUpdate)
	}
	if !manifest.SMAPI.Required || manifest.SMAPI.Bundled || manifest.SMAPI.InstallerFile != "" || manifest.SMAPI.SHA256 != "" {
		t.Fatalf("unexpected update-pack SMAPI metadata: %+v", manifest.SMAPI)
	}

	checksums := readZipFileString(t, findZipFile(zr.File, "checksums.sha256"))
	if strings.Contains(checksums, "payload/smapi/") {
		t.Fatalf("update pack checksums must not reference SMAPI payload:\n%s", checksums)
	}
	installScript := readZipFileString(t, findZipFile(zr.File, "tools/install.ps1"))
	for _, snippet := range []string{
		"function Assert-SMAPIAlreadyInstalled",
		"$packType -eq \"mods_update\"",
		"reason = \"mods_update_pack\"",
		"模组更新包已跳过 Steam 启动项配置。",
		"if ($steamReason -ne \"mods_update_pack\")",
		"请先运行完整版玩家同步包",
	} {
		if !strings.Contains(installScript, snippet) {
			t.Fatalf("update installer should require existing SMAPI and skip Steam launch options; missing %q", snippet)
		}
	}
}

func TestExportNexusInstallerExtensionZip_IncludesManifestAtRoot(t *testing.T) {
	dir := t.TempDir()
	zipPath, err := ExportNexusInstallerExtensionZip(dir)
	if err != nil {
		t.Fatalf("ExportNexusInstallerExtensionZip: %v", err)
	}
	if want := filepath.Join(dir, ".local-container", "browser-extensions", NexusInstallerExtensionFileName); zipPath != want {
		t.Fatalf("zipPath = %q, want %q", zipPath, want)
	}

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open exported zip: %v", err)
	}
	defer func() { _ = zr.Close() }()

	if findZipFile(zr.File, "manifest.json") == nil {
		t.Fatal("extension zip should contain manifest.json at root")
	}
	if findZipFile(zr.File, "background.js") == nil {
		t.Fatal("extension zip should contain background.js at root")
	}
	if findZipFile(zr.File, "安装说明.txt") == nil {
		t.Fatal("extension zip should contain install instructions")
	}
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "nexus-slow-installer/") {
			t.Fatalf("extension zip should not require selecting an inner folder, found %q", f.Name)
		}
	}
}

func TestEnsureNexusInstallerExtensionZip_ReusesValidExistingPackage(t *testing.T) {
	dir := t.TempDir()
	outPath := nexusInstallerExtensionZipPath(dir)
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		t.Fatal(err)
	}
	zf, err := os.Create(outPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(zf)
	if err := addZipTextFile(zw, "manifest.json", `{"manifest_version":3,"name":"prebuilt","version":"1.0.0"}`); err != nil {
		t.Fatal(err)
	}
	if err := addZipTextFile(zw, "background.js", "const prebuilt = true;\n"); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zf.Close(); err != nil {
		t.Fatal(err)
	}

	zipPath, err := EnsureNexusInstallerExtensionZip(dir)
	if err != nil {
		t.Fatalf("EnsureNexusInstallerExtensionZip: %v", err)
	}
	if zipPath != outPath {
		t.Fatalf("zipPath = %q, want %q", zipPath, outPath)
	}

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open exported zip: %v", err)
	}
	defer func() { _ = zr.Close() }()
	if got := readZipFileString(t, findZipFile(zr.File, "background.js")); got != "const prebuilt = true;\n" {
		t.Fatalf("existing package should be reused, background.js = %q", got)
	}
}

func TestEnsureNexusInstallerExtensionZip_RebuildsInvalidExistingPackage(t *testing.T) {
	dir := t.TempDir()
	outPath := nexusInstallerExtensionZipPath(dir)
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outPath, []byte("not a zip"), 0o644); err != nil {
		t.Fatal(err)
	}

	zipPath, err := EnsureNexusInstallerExtensionZip(dir)
	if err != nil {
		t.Fatalf("EnsureNexusInstallerExtensionZip: %v", err)
	}
	if zipPath != outPath {
		t.Fatalf("zipPath = %q, want %q", zipPath, outPath)
	}

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open exported zip: %v", err)
	}
	defer func() { _ = zr.Close() }()
	if findZipFile(zr.File, "manifest.json") == nil {
		t.Fatal("rebuilt extension zip should contain manifest.json at root")
	}
	if findZipFile(zr.File, "background.js") == nil {
		t.Fatal("rebuilt extension zip should contain background.js at root")
	}
}

func TestPlayerSyncPowerShellScriptsParse(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("PowerShell parser smoke test only runs on Windows")
	}
	scripts := map[string]string{
		"install.ps1":              installPowerShellScript,
		"uninstall.ps1":            uninstallPowerShellScript,
		"steam-launch-options.ps1": steamLaunchOptionsPowerShellScript,
		"vdf.ps1":                  vdfPowerShellScript,
	}
	dir := t.TempDir()
	for name, script := range scripts {
		path := filepath.Join(dir, name)
		data := append([]byte{0xEF, 0xBB, 0xBF}, []byte(script)...)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		quotedPath := strings.ReplaceAll(path, "'", "''")
		command := fmt.Sprintf(`$p='%s'; $tokens=$null; $errors=$null; [System.Management.Automation.Language.Parser]::ParseFile($p, [ref]$tokens, [ref]$errors) | Out-Null; if ($errors.Count -gt 0) { $errors | ForEach-Object { Write-Error $_.Message }; exit 1 }`, quotedPath)
		cmd := exec.Command("powershell", "-NoProfile", "-Command", command)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s failed PowerShell parse: %v\n%s", name, err, out)
		}
	}
}

func TestPlayerSyncInstallScriptUsesLiteralPathsForModFolders(t *testing.T) {
	required := []string{
		"Test-Path -LiteralPath $file",
		"Get-FileHash -Algorithm SHA256 -LiteralPath $file",
		"Test-Path -LiteralPath $source",
		"Test-Path -LiteralPath $target",
		"Test-Path -LiteralPath $backup",
	}
	for _, snippet := range required {
		if !strings.Contains(installPowerShellScript+"\n"+uninstallPowerShellScript, snippet) {
			t.Fatalf("installer scripts must use literal paths for bracketed mod folders; missing %q", snippet)
		}
	}
}

func TestPlayerSyncInstallScriptShowsProgress(t *testing.T) {
	required := []string{
		"function Write-LogLine",
		"AppendAllText($LogPath",
		"function Show-InstallProgress",
		"function Render-InstallProgressLine",
		"function Clear-InstallProgressLine",
		"function Redraw-InstallProgressLine",
		"function Finish-InstallProgressLine",
		"$ProgressPreference = \"SilentlyContinue\"",
		"Get-InstallProgressStage",
		"START",
		"MODS",
		"progress {1,3}%",
		"校验文件 {0}/{1}",
		"安装 Mod {0}/{1}",
		"Complete-InstallProgress \"安装完成\"",
	}
	for _, snippet := range required {
		if !strings.Contains(installPowerShellScript, snippet) {
			t.Fatalf("install script should show progress; missing %q", snippet)
		}
	}
	start := strings.Index(installPowerShellScript, "function Show-InstallProgress")
	end := strings.Index(installPowerShellScript, "function Complete-InstallProgress")
	if start < 0 || end < start {
		t.Fatal("could not isolate Show-InstallProgress function")
	}
	showProgress := installPowerShellScript[start:end]
	if strings.Contains(showProgress, "Write-Host $line") {
		t.Fatal("install progress should not print every progress tick to the console")
	}
	renderStart := strings.Index(installPowerShellScript, "function Render-InstallProgressLine")
	renderEnd := strings.Index(installPowerShellScript, "function Clear-InstallProgressLine")
	if renderStart < 0 || renderEnd < renderStart {
		t.Fatal("could not isolate Render-InstallProgressLine function")
	}
	renderProgress := installPowerShellScript[renderStart:renderEnd]
	forbiddenInRender := []string{"$Status", "安装", "校验", "解压", "配置", "完成"}
	for _, snippet := range forbiddenInRender {
		if strings.Contains(renderProgress, snippet) {
			t.Fatalf("console progress line must stay ASCII-only and not include Chinese status text; found %q", snippet)
		}
	}
	forbidden := []string{
		"Add-Content -Path $LogPath",
		"Write-Progress",
		"[char]13",
	}
	for _, snippet := range forbidden {
		if strings.Contains(installPowerShellScript, snippet) {
			t.Fatalf("install script should avoid carriage-return progress rendering; found %q", snippet)
		}
	}
}

func TestPlayerSyncInstallScriptPrintsSteamLaunchOptionsInSummary(t *testing.T) {
	required := []string{
		"Steam 启动项：{0}",
		"Write-Host \"Steam 启动项文本：\" -ForegroundColor Yellow",
		"Write-Host \"请复制到 Steam 的游戏启动项中。\" -ForegroundColor Yellow",
		"Write-Host $launchOptionsText -ForegroundColor Cyan",
		"$launchOptionsText",
		"$steamResult.ContainsKey(\"reason\")",
		"if ($steamReason -ne \"mods_update_pack\")",
		"Write-Host \"建议 Steam 启动项：\" -ForegroundColor Yellow",
		"Write-Host $launchOptions -ForegroundColor Cyan",
	}
	for _, snippet := range required {
		if !strings.Contains(installPowerShellScript+"\n"+steamLaunchOptionsPowerShellScript, snippet) {
			t.Fatalf("install script should print launch options clearly; missing %q", snippet)
		}
	}
	if strings.Contains(installPowerShellScript, "未自动设置，请查看上方提示") {
		t.Fatal("install summary should include the concrete launch options instead of referring to earlier output")
	}
	if strings.Contains(installPowerShellScript, "SMAPI 路径：") {
		t.Fatal("install summary should only show the copyable Steam launch options, not a separate SMAPI path")
	}
}

func TestPlayerSyncInstallScriptUsesOfficialSMAPIInstaller(t *testing.T) {
	required := []string{
		"-Filter \"SMAPI.Installer.exe\"",
		"--install --game-path",
		"--no-prompt",
		"official_installer",
		"Invoke-OfficialSMAPIInstaller",
	}
	for _, snippet := range required {
		if !strings.Contains(installPowerShellScript, snippet) {
			t.Fatalf("install script should install SMAPI through the official installer; missing %q", snippet)
		}
	}
	forbidden := []string{
		"-Filter \"install.dat\"",
		"smapi-install-payload.zip",
		"official_install_payload",
	}
	for _, snippet := range forbidden {
		if strings.Contains(installPowerShellScript, snippet) {
			t.Fatalf("install script should not unpack SMAPI's install.dat payload directly; found %q", snippet)
		}
	}
}

func TestPlayerSyncInstallScriptSkipsIdenticalMods(t *testing.T) {
	required := []string{
		"function Get-DirectoryFingerprint",
		"$sourceFingerprint = Get-DirectoryFingerprint $source",
		"$targetFingerprint = Get-DirectoryFingerprint $target",
		"$skippedIdentical = $true",
		"skippedIdentical = $skippedIdentical",
		"$backupCreated = $true",
		"if ($backupCreated) { $resultBackupId = $backupId }",
		"已跳过相同 Mod",
	}
	for _, snippet := range required {
		if !strings.Contains(installPowerShellScript, snippet) {
			t.Fatalf("install script should skip identical mods; missing %q", snippet)
		}
	}
	requiredFlow := []string{
		"if (-not $skippedIdentical) {\n        New-Item -ItemType Directory -Force -Path $backupRoot",
		"if (-not $skippedIdentical) {\n      Copy-Item -LiteralPath $source -Destination $target -Recurse -Force",
	}
	for _, snippet := range requiredFlow {
		if !strings.Contains(installPowerShellScript, snippet) {
			t.Fatalf("identical mod skip should avoid backup/copy; missing flow %q", snippet)
		}
	}
}

func TestExportModSyncPackZip_RejectsPathEscape(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".local-container", "mods")
	createTestMod(t, root, "ClientMod", "author.client", "Client Mod")
	if err := SetModSyncClassification(dir, "ClientMod", registry.ModSyncKindClientRequired, ""); err != nil {
		t.Fatal(err)
	}

	zipPath, err := ExportModSyncPackZip(dir)
	if err != nil {
		t.Fatalf("ExportModSyncPackZip: %v", err)
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

// splitFirst returns the first path segment of a slash-separated zip entry name.
func splitFirst(name string) string {
	for i, c := range name {
		if c == '/' {
			return name[:i]
		}
	}
	return name
}

func findZipFile(files []*zip.File, name string) *zip.File {
	for _, f := range files {
		if f.Name == name {
			return f
		}
	}
	return nil
}

func readZipFileString(t *testing.T, f *zip.File) string {
	t.Helper()
	return string(readZipFileBytes(t, f))
}

func readZipFileBytes(t *testing.T, f *zip.File) []byte {
	t.Helper()
	rc, err := f.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rc.Close() }()
	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
