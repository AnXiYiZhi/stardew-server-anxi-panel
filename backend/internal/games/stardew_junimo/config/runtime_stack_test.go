package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuiltInRuntimeStackManifestIsValid(t *testing.T) {
	manifest, err := BuiltInRuntimeStackManifest()
	if err != nil {
		t.Fatalf("BuiltInRuntimeStackManifest: %v", err)
	}
	if manifest.Server.Tag != "1.5.0-preview.125" || manifest.SteamAuth.Tag != "1.5.0-anxi.2" {
		t.Fatalf("unexpected tested pair: server=%q auth=%q", manifest.Server.Tag, manifest.SteamAuth.Tag)
	}
	if manifest.Game.AppID != "413150" || manifest.Game.BuildID == "" || manifest.SDK.AppID != "1007" || manifest.SDK.BuildID == "" || !manifest.Tested {
		t.Fatalf("runtime content recommendation is incomplete: game=%#v sdk=%#v tested=%v", manifest.Game, manifest.SDK, manifest.Tested)
	}
	if manifest.Server.Image != DefaultServerImage {
		t.Fatalf("server matrix preferred image must match the canonical install default")
	}
	if len(manifest.Server.Images) != 1 || len(manifest.SteamAuth.Images) != 1 {
		t.Fatal("recommended matrix must contain only the currently verified canonical image refs")
	}
	for _, component := range []RuntimeStackManifestComponent{manifest.Server, manifest.SteamAuth} {
		if !validRuntimeDigest(component.Digests[component.Images[0]]) {
			t.Fatal("canonical image digest is missing")
		}
	}
}

func TestPanelVersionSatisfiesMatrixMinimum(t *testing.T) {
	for _, tc := range []struct {
		current string
		want    bool
	}{{"0.2.2", true}, {"v0.2.3", true}, {"0.2.1", false}, {"latest", false}, {"dev", true}} {
		if got := PanelVersionSatisfies(tc.current, "0.2.2"); got != tc.want {
			t.Fatalf("PanelVersionSatisfies(%q)=%v, want %v", tc.current, got, tc.want)
		}
	}
}

func TestOnlyRecommendedMatrixIsInstallable(t *testing.T) {
	manifest, err := BuiltInRuntimeStackManifest()
	if err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{RuntimeMatrixStatusWithdrawn, "candidate", "tested"} {
		manifest.Status = status
		if manifest.Installable() {
			t.Fatalf("status %q must not be installable", status)
		}
	}
	manifest.Status = RuntimeMatrixStatusRecommended
	if !manifest.Installable() {
		t.Fatal("recommended matrix must be installable")
	}
}

func TestRuntimeStackManifestRejectsDiscoveredStatus(t *testing.T) {
	manifest, err := BuiltInRuntimeStackManifest()
	if err != nil {
		t.Fatal(err)
	}
	manifest.Status = "discovered"
	if err := ValidateRuntimeStackManifest(manifest); err == nil {
		t.Fatal("discovered status must no longer be accepted")
	}
}

func TestRuntimeStackManifestRejectsUnsafeComponents(t *testing.T) {
	base, err := BuiltInRuntimeStackManifest()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func(*RuntimeStackManifest)
	}{
		{"server missing", func(m *RuntimeStackManifest) { m.Server = RuntimeStackManifestComponent{} }},
		{"auth missing", func(m *RuntimeStackManifest) { m.SteamAuth = RuntimeStackManifestComponent{} }},
		{"latest", func(m *RuntimeStackManifest) { m.Server.Tag = "latest" }},
		{"empty tag", func(m *RuntimeStackManifest) { m.SteamAuth.Tag = "" }},
		{"digest ref", func(m *RuntimeStackManifest) { m.Server.Image = "sdvd/server@sha256:" + strings.Repeat("a", 64) }},
		{"digest tag", func(m *RuntimeStackManifest) { m.Server.Tag = strings.Repeat("a", 64) }},
		{"untrusted repository", func(m *RuntimeStackManifest) { m.SteamAuth.Image = "evil.example/steam-auth:1.5.0-anxi.2" }},
		{"game app mismatch", func(m *RuntimeStackManifest) { m.Game.AppID = "1007" }},
		{"game build invalid", func(m *RuntimeStackManifest) { m.Game.BuildID = "latest" }},
		{"sdk build missing", func(m *RuntimeStackManifest) { m.SDK.BuildID = "" }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			manifest := base
			manifest.Server.TrustedCandidates = append([]string(nil), base.Server.TrustedCandidates...)
			manifest.SteamAuth.TrustedCandidates = append([]string(nil), base.SteamAuth.TrustedCandidates...)
			tc.mutate(&manifest)
			if err := ValidateRuntimeStackManifest(manifest); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestSteamDiscoveryToolCannotEditRecommendationMatrix(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..", "..")
	for _, name := range []string{filepath.Join(root, "scripts", "discover-steam-builds.ps1"), filepath.Join(root, ".github", "workflows", "discover-steam-builds.yml")} {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		lower := strings.ToLower(string(data))
		for _, forbidden := range []string{"runtime_stack_manifest.json", "git commit", "git push", "git tag"} {
			if strings.Contains(lower, forbidden) {
				t.Fatalf("%s contains forbidden mutation %q", name, forbidden)
			}
		}
	}
}

func TestInspectRuntimeStackVersionPairs(t *testing.T) {
	manifest, err := BuiltInRuntimeStackManifest()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name      string
		serverTag string
		authTag   string
		want      string
		available bool
	}{
		{"exact pair", manifest.Server.Tag, manifest.SteamAuth.Tag, RuntimeStackStatusUpToDate, false},
		{"supported preview.121 remains optional update", "1.5.0-preview.121", manifest.SteamAuth.Tag, RuntimeStackStatusUpdateAvailable, true},
		{"server new auth old", manifest.Server.Tag, "1.5.0-anxi.1", RuntimeStackStatusUpdateAvailable, true},
		{"both old", "1.5.0-preview.120", "1.5.0-anxi.1", RuntimeStackStatusUpdateAvailable, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeRuntimeEnv(t, dir, tc.serverTag, tc.authTag, false)
			got := InspectRuntimeStack(dir, true)
			if got.Status != tc.want || got.Available != tc.available || !got.Supported {
				t.Fatalf("status=%q available=%v supported=%v, want %q/%v/true (%s)", got.Status, got.Available, got.Supported, tc.want, tc.available, got.Reason)
			}
		})
	}
}

func TestInspectRuntimeStackCustomMissingAndNotInstalled(t *testing.T) {
	dir := t.TempDir()
	writeRuntimeEnv(t, dir, "1.5.0-preview.121", "1.5.0-anxi.2", true)
	custom := InspectRuntimeStack(dir, true)
	if custom.Status != RuntimeStackStatusCustomImages || custom.Supported || custom.Code != "unsupported/custom_images" {
		t.Fatalf("custom result = %#v", custom)
	}

	missing := InspectRuntimeStack(t.TempDir(), true)
	if missing.Status != RuntimeStackStatusInvalidConfig || missing.Code != "invalid_config/missing_env" {
		t.Fatalf("missing env result = %#v", missing)
	}

	notInstalled := InspectRuntimeStack(t.TempDir(), false)
	if notInstalled.Status != RuntimeStackStatusNotInstalled || notInstalled.Available || !notInstalled.Supported {
		t.Fatalf("not installed result = %#v", notInstalled)
	}
}

func writeRuntimeEnv(t *testing.T, dir, serverTag, authTag string, custom bool) {
	t.Helper()
	serverRepo := "sdvd/server"
	if custom {
		serverRepo = "registry.example/custom/server"
	}
	content := strings.Join([]string{
		"IMAGE_VERSION=" + serverTag,
		"SERVER_IMAGE=" + serverRepo + ":" + serverTag,
		"SERVER_IMAGE_CANDIDATES=" + serverRepo + ":" + serverTag,
		"STEAM_SERVICE_IMAGE=anxiyizhi/junimo-steam-service-cn:" + authTag,
		"STEAM_SERVICE_IMAGE_CANDIDATES=anxiyizhi/junimo-steam-service-cn:" + authTag,
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
