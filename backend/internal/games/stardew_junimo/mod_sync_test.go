package stardew_junimo

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
)

// ── Classification file read/write ──────────────────────────────────────────

func TestGetModSyncClassification_DefaultUnknown(t *testing.T) {
	dir := t.TempDir()
	kind, note := GetModSyncClassification(dir, "SomeMod")
	if kind != registry.ModSyncKindUnknown {
		t.Errorf("kind = %q, want %q", kind, registry.ModSyncKindUnknown)
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
	if kind != registry.ModSyncKindUnknown {
		t.Errorf("kind = %q, want fallback to %q", kind, registry.ModSyncKindUnknown)
	}
}

// ── ApplyModSyncClassification / BuildModSyncPlan ───────────────────────────

func TestApplyModSyncClassification_DefaultsUnknownForUnclassifiedMods(t *testing.T) {
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
	if mods[0].SyncKind != registry.ModSyncKindUnknown {
		t.Errorf("SyncKind = %q, want %q", mods[0].SyncKind, registry.ModSyncKindUnknown)
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
	if plan.Summary.ClientRequired != 1 {
		t.Errorf("ClientRequired = %d, want 1", plan.Summary.ClientRequired)
	}
	if plan.Summary.ServerOnly != 1 {
		t.Errorf("ServerOnly = %d, want 1 (control mod default)", plan.Summary.ServerOnly)
	}
	if plan.Summary.Unknown != 1 {
		t.Errorf("Unknown = %d, want 1", plan.Summary.Unknown)
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
	// UnknownMod stays unclassified (default unknown).

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
		parts := splitFirst(f.Name)
		seenFolders[parts] = true
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
		if splitFirst(f.Name) == controlModFolderName {
			t.Fatalf("export must not contain %s, found entry %q", controlModFolderName, f.Name)
		}
	}
}

func TestExportModSyncPackZip_NoMods(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".local-container", "mods")
	createTestMod(t, root, "ServerMod", "author.server", "Server Mod")
	// No mod classified as client_required.

	_, err := ExportModSyncPackZip(dir)
	if err == nil {
		t.Fatal("expected error when no client_required mods exist")
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
		if f.Name == "player-sync-manifest.json" {
			manifestFile = f
		}
	}
	if manifestFile == nil {
		t.Fatal("expected player-sync-manifest.json in export")
	}

	rc, err := manifestFile.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rc.Close() }()

	var manifest playerSyncManifest
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
