package stardew_junimo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
)

func TestEnsureInstanceDockerHostBindingsMigratesLegacyCompose(t *testing.T) {
	containerRoot := filepath.Join(t.TempDir(), "container-data")
	hostRoot := filepath.Join(t.TempDir(), "host-data")
	instanceDir := filepath.Join(containerRoot, "instances", "stardew")
	if err := os.MkdirAll(instanceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	legacyCompose := strings.ReplaceAll(junimoComposeTemplate, hostManagedBindPrefix, legacyManagedBindPrefix)
	if err := os.WriteFile(filepath.Join(instanceDir, "docker-compose.yml"), []byte(legacyCompose), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := sjconfig.UpdateEnvFile(filepath.Join(instanceDir, ".env"), map[string]string{"IMAGE_VERSION": TestedImageTag}); err != nil {
		t.Fatal(err)
	}
	driver := NewWithOptions(nil, nil, nil, nil, DriverOptions{
		ContainerDataDir: containerRoot,
		HostDataDir:      hostRoot,
	})
	if err := driver.ensureInstanceDockerHostBindings(instanceDir); err != nil {
		t.Fatalf("ensureInstanceDockerHostBindings: %v", err)
	}

	values, err := sjconfig.ReadEnvFile(filepath.Join(instanceDir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	wantHostDir := filepath.Join(hostRoot, "instances", "stardew")
	if values["INSTANCE_HOST_DATA_DIR"] != wantHostDir {
		t.Fatalf("INSTANCE_HOST_DATA_DIR = %q, want %q", values["INSTANCE_HOST_DATA_DIR"], wantHostDir)
	}
	composePath := filepath.Join(instanceDir, "docker-compose.yml")
	migrated, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(migrated), legacyManagedBindPrefix) {
		t.Fatal("legacy Panel-relative managed bind remains after migration")
	}
	wantManagedBinds := strings.Count(junimoComposeTemplate, hostManagedBindPrefix)
	if got := strings.Count(string(migrated), hostManagedBindPrefix); got != wantManagedBinds || got == 0 {
		t.Fatalf("managed host binds = %d, want %d", got, wantManagedBinds)
	}
	info, err := os.Stat(composePath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("Compose mode = %o, want 640", info.Mode().Perm())
	}

	if err := driver.ensureInstanceDockerHostBindings(instanceDir); err != nil {
		t.Fatalf("idempotent ensureInstanceDockerHostBindings: %v", err)
	}
	afterRetry, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(afterRetry) != string(migrated) {
		t.Fatal("idempotent host bind migration changed Compose bytes")
	}
}

func TestEnsureInstanceDockerHostBindingsRejectsIncompletePreparedInstance(t *testing.T) {
	instanceDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(instanceDir, "docker-compose.yml"), []byte(junimoComposeTemplate), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := New(nil, nil, nil, nil).ensureInstanceDockerHostBindings(instanceDir); err == nil {
		t.Fatal("host binding migration should reject a prepared instance without .env")
	}
}

func TestJunimoComposeTemplateUsesDaemonVisibleManagedBindRoot(t *testing.T) {
	if strings.Contains(junimoComposeTemplate, legacyManagedBindPrefix) {
		t.Fatal("new instance template contains Panel-relative managed binds")
	}
	if count := strings.Count(junimoComposeTemplate, hostManagedBindPrefix); count < 20 {
		t.Fatalf("new instance template has only %d daemon-visible managed binds", count)
	}
}
