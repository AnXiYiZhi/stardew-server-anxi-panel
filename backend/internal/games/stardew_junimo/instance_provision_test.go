package stardew_junimo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
)

func TestAllocateProvisionPortsSkipsExistingInstanceBindings(t *testing.T) {
	root := t.TempDir()
	firstDir := filepath.Join(root, "stardew")
	secondDir := filepath.Join(root, "river-farm")
	for _, item := range []struct {
		dir   string
		ports map[string]string
	}{
		{dir: firstDir, ports: map[string]string{"GAME_PORT": "24642", "QUERY_PORT": "27015", "VNC_PORT": "5800", "API_PORT": "8080"}},
		{dir: secondDir, ports: map[string]string{"GAME_PORT": "24643", "QUERY_PORT": "27016", "VNC_PORT": "5801", "API_PORT": "8081"}},
	} {
		if err := os.MkdirAll(item.dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := sjconfig.UpdateEnvFile(filepath.Join(item.dir, ".env"), item.ports); err != nil {
			t.Fatal(err)
		}
	}
	ports, err := allocateProvisionPorts([]registry.Instance{
		{ID: "stardew", DriverID: DriverID, DataDir: firstDir},
		{ID: "river-farm", DriverID: DriverID, DataDir: secondDir},
	}, "forest-farm")
	if err != nil {
		t.Fatal(err)
	}
	if ports.game != 24644 || ports.query != 27017 || ports.vnc != 5802 || ports.api != 8082 {
		t.Fatalf("unexpected allocated ports: %#v", ports)
	}
}

func TestAllocateProvisionPortsRejectsInvalidExistingBinding(t *testing.T) {
	dataDir := t.TempDir()
	if err := sjconfig.UpdateEnvFile(filepath.Join(dataDir, ".env"), map[string]string{"GAME_PORT": "not-a-port"}); err != nil {
		t.Fatal(err)
	}
	if _, err := allocateProvisionPorts([]registry.Instance{{ID: "broken", DriverID: DriverID, DataDir: dataDir}}, "new-world"); err == nil {
		t.Fatal("expected invalid existing port to fail closed")
	}
}

func TestInstallationTemplateEnvPreservesLegacyInstalledRuntimeIdentity(t *testing.T) {
	dataDir := t.TempDir()
	if err := sjconfig.UpdateEnvFile(filepath.Join(dataDir, ".env"), map[string]string{
		"IMAGE_VERSION":           "1.5.0-preview.121",
		"SERVER_IMAGE":            "dockerproxy.net/sdvd/server:1.5.0-preview.125",
		"SERVER_IMAGE_CANDIDATES": "dockerproxy.net/sdvd/server:1.5.0-preview.125,sdvd/server:1.5.0-preview.125",
		"SMAPI_VERSION":           "4.1.10",
		"SMAPI_DOWNLOAD_URLS":     "https://example.invalid/smapi.zip",
		"STEAM_USERNAME":          "must-not-copy",
		"STEAM_PASSWORD":          "must-not-copy",
		"VNC_PASSWORD":            "must-not-copy",
	}); err != nil {
		t.Fatal(err)
	}

	updates, err := installationTemplateEnv(dataDir, "dockerproxy.net/sdvd/server:1.5.0-preview.125")
	if err != nil {
		t.Fatal(err)
	}
	if updates["SERVER_IMAGE"] != "dockerproxy.net/sdvd/server:1.5.0-preview.125" || updates["IMAGE_VERSION"] != "1.5.0-preview.125" {
		t.Fatalf("installed runtime identity was not preserved: %#v", updates)
	}
	if updates["SERVER_IMAGE_CANDIDATES"] != "dockerproxy.net/sdvd/server:1.5.0-preview.125,sdvd/server:1.5.0-preview.125" ||
		updates["SMAPI_VERSION"] != "4.1.10" || updates["SMAPI_DOWNLOAD_URLS"] != "https://example.invalid/smapi.zip" {
		t.Fatalf("installed runtime metadata was not preserved: %#v", updates)
	}
	for _, secret := range []string{"STEAM_USERNAME", "STEAM_PASSWORD", "VNC_PASSWORD"} {
		if _, exists := updates[secret]; exists {
			t.Fatalf("instance-owned secret %s must not be copied", secret)
		}
	}
}

func TestInstallationTemplateEnvFallsBackToVerifiedImageCandidate(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dataDir, ".env"), []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	updates, err := installationTemplateEnv(dataDir, "sdvd/server:1.5.0-preview.125")
	if err != nil {
		t.Fatal(err)
	}
	if updates["IMAGE_VERSION"] != "1.5.0-preview.125" || updates["SERVER_IMAGE_CANDIDATES"] != "sdvd/server:1.5.0-preview.125" {
		t.Fatalf("verified image fallback = %#v", updates)
	}
}

func TestConvergeProvisionedInstanceTemplateRepairsMissingDefaultImageWithoutCopyingSecrets(t *testing.T) {
	root := t.TempDir()
	templateDir := filepath.Join(root, "instances", "stardew")
	targetDir := filepath.Join(root, "instances", "stardew-2")
	for _, dir := range []string{templateDir, targetDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := sjconfig.UpdateEnvFile(filepath.Join(templateDir, ".env"), map[string]string{
		"GAME_DATA_VOLUME":        "stardew_game-data",
		"IMAGE_VERSION":           "1.5.0-preview.125",
		"SERVER_IMAGE":            "dockerproxy.net/sdvd/server:1.5.0-preview.125",
		"SERVER_IMAGE_CANDIDATES": "dockerproxy.net/sdvd/server:1.5.0-preview.125,sdvd/server:1.5.0-preview.125",
		"STEAM_USERNAME":          "template-secret",
	}); err != nil {
		t.Fatal(err)
	}
	if err := sjconfig.UpdateEnvFile(filepath.Join(targetDir, ".env"), map[string]string{
		"GAME_DATA_VOLUME": "stardew-2_game-data",
		"IMAGE_VERSION":    "1.5.0-preview.125",
		"SERVER_IMAGE":     "sdvd/server:1.5.0-preview.125",
		"STEAM_USERNAME":   "target-owned",
	}); err != nil {
		t.Fatal(err)
	}
	fake := &fakeDocker{inspectErrByImage: map[string]error{
		"sdvd/server:1.5.0-preview.125": errors.New("No such image: sdvd/server:1.5.0-preview.125"),
	}}
	driver := NewWithOptions(fake, nil, nil, nil, DriverOptions{ContainerDataDir: root})
	changed, err := driver.ConvergeProvisionedInstanceTemplate(context.Background(),
		registry.Instance{ID: "stardew", DriverID: DriverID, DataDir: templateDir, State: "stopped"},
		registry.Instance{ID: "stardew-2", DriverID: DriverID, DataDir: targetDir, State: "save_required", DriverPhase: "instance_ready"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected compatibility convergence")
	}
	values, err := sjconfig.ReadEnvFile(filepath.Join(targetDir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if values["SERVER_IMAGE"] != "dockerproxy.net/sdvd/server:1.5.0-preview.125" ||
		values["SERVER_IMAGE_CANDIDATES"] != "dockerproxy.net/sdvd/server:1.5.0-preview.125,sdvd/server:1.5.0-preview.125" {
		t.Fatalf("target runtime identity = %#v", values)
	}
	if values["STEAM_USERNAME"] != "target-owned" {
		t.Fatalf("target-owned credential changed: %q", values["STEAM_USERNAME"])
	}
	if fake.verifyOpts.ImageRef != "dockerproxy.net/sdvd/server:1.5.0-preview.125" ||
		len(fake.verifyOpts.Binds) != 1 || fake.verifyOpts.Binds[0] != "stardew-2_game-data:/data/game" {
		t.Fatalf("verification did not use template image with target volume: %#v", fake.verifyOpts)
	}
}

func TestConvergeProvisionedInstanceTemplateDoesNotOverrideAvailableOrCustomizedRuntime(t *testing.T) {
	root := t.TempDir()
	templateDir := filepath.Join(root, "instances", "stardew")
	targetDir := filepath.Join(root, "instances", "stardew-2")
	for _, item := range []struct {
		dir    string
		values map[string]string
	}{
		{dir: templateDir, values: map[string]string{"GAME_DATA_VOLUME": "stardew_game-data", "SERVER_IMAGE": "mirror.example/server:1.5.0-preview.125"}},
		{dir: targetDir, values: map[string]string{"GAME_DATA_VOLUME": "stardew-2_game-data", "SERVER_IMAGE": "custom.example/server:1.5.0-preview.124"}},
	} {
		if err := os.MkdirAll(item.dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := sjconfig.UpdateEnvFile(filepath.Join(item.dir, ".env"), item.values); err != nil {
			t.Fatal(err)
		}
	}
	fake := &fakeDocker{}
	driver := NewWithOptions(fake, nil, nil, nil, DriverOptions{ContainerDataDir: root})
	changed, err := driver.ConvergeProvisionedInstanceTemplate(context.Background(),
		registry.Instance{ID: "stardew", DriverID: DriverID, DataDir: templateDir, State: "stopped"},
		registry.Instance{ID: "stardew-2", DriverID: DriverID, DataDir: targetDir, State: "save_required", DriverPhase: "instance_ready"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if changed || len(fake.inspectedImages) != 0 {
		t.Fatalf("custom runtime should be left untouched: changed=%v inspected=%#v", changed, fake.inspectedImages)
	}
}

func TestCleanupProvisionedInstanceWithoutJournalPreservesResources(t *testing.T) {
	containerDataDir := t.TempDir()
	target := filepath.Join(containerDataDir, "instances", "river-farm")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := sjconfig.UpdateEnvFile(filepath.Join(target, ".env"), map[string]string{"GAME_DATA_VOLUME": "river-farm_game-data"}); err != nil {
		t.Fatal(err)
	}
	docker := &fakeDocker{}
	driver := &Driver{docker: docker, containerDataDir: containerDataDir}
	instance := registry.Instance{ID: "river-farm", DriverID: DriverID, DataDir: target}
	if err := driver.CleanupProvisionedInstance(context.Background(), instance); err != nil {
		t.Fatalf("cleanup owned target: %v", err)
	}
	if len(docker.removedVolumes) != 0 {
		t.Fatalf("removed volumes = %#v", docker.removedVolumes)
	}
	if _, err := os.Stat(filepath.Join(target, ".env")); err != nil {
		t.Fatalf("unowned target was removed: %v", err)
	}
	outside := registry.Instance{ID: "outside", DriverID: DriverID, DataDir: filepath.Join(containerDataDir, "outside")}
	if err := driver.CleanupProvisionedInstance(context.Background(), outside); err == nil {
		t.Fatal("expected cleanup outside instances root to be rejected")
	}
}
