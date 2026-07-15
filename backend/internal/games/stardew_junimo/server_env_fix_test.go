package stardew_junimo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureServerContEnvFixWritesScriptAndMigratesCompose(t *testing.T) {
	dataDir := t.TempDir()
	composePath := filepath.Join(dataDir, "docker-compose.yml")
	compose := `services:
  server:
    image: ${SERVER_IMAGE:-sdvd/server:1.5.0-preview.121}
    volumes:
      - ./.local-container/settings:/data/settings
      - ./.local-container/control:/data/control
      - ./.local-container/mods:/data/Mods
`
	if err := os.WriteFile(composePath, []byte(compose), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, err := EnsureServerContEnvFix(dataDir)
	if err != nil {
		t.Fatalf("EnsureServerContEnvFix: %v", err)
	}
	if !changed {
		t.Fatal("expected first run to report changed")
	}

	updated, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatal(err)
	}
	updatedText := string(updated)
	for _, staticValue := range serverStaticInitValues {
		scriptPath := filepath.Join(dataDir, filepath.FromSlash(staticValue.localPath))
		script, err := os.ReadFile(scriptPath)
		if err != nil {
			t.Fatalf("%s script not written: %v", staticValue.localPath, err)
		}
		if string(script) != serverStaticInitScript(staticValue.value) {
			t.Fatalf("unexpected %s script:\n%s", staticValue.localPath, script)
		}
		if !strings.Contains(updatedText, serverStaticInitComposeMount(staticValue)) {
			t.Fatalf("compose missing %s mount:\n%s", staticValue.localPath, updatedText)
		}
	}
	for _, line := range serverHeadlessAudioEnvironment {
		if !strings.Contains(updatedText, line) {
			t.Fatalf("compose missing headless audio environment %q:\n%s", line, updatedText)
		}
	}

	changed, err = EnsureServerContEnvFix(dataDir)
	if err != nil {
		t.Fatalf("EnsureServerContEnvFix second run: %v", err)
	}
	if changed {
		t.Fatal("expected second run to be idempotent")
	}
}

func TestEnsureServerContEnvFixAddsMissingStaticMountsWhenAppNameExists(t *testing.T) {
	dataDir := t.TempDir()
	composePath := filepath.Join(dataDir, "docker-compose.yml")
	compose := `services:
  server:
    image: ${SERVER_IMAGE:-sdvd/server:1.5.0-preview.121}
    volumes:
      - ./.local-container/settings:/data/settings
      - ./.local-container/control:/data/control
      - ./.local-container/cont-env/APP_NAME:/etc/cont-env.d/APP_NAME:ro
      - ./.local-container/mods:/data/Mods
`
	if err := os.WriteFile(composePath, []byte(compose), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, err := EnsureServerContEnvFix(dataDir)
	if err != nil {
		t.Fatalf("EnsureServerContEnvFix: %v", err)
	}
	if !changed {
		t.Fatal("expected missing static mounts to report changed")
	}

	updated, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatal(err)
	}
	updatedText := string(updated)
	if !strings.Contains(updatedText, "/etc/cont-env.d/DBUS_SESSION_BUS_ADDRESS") {
		t.Fatalf("compose missing DBUS_SESSION_BUS_ADDRESS mount:\n%s", updatedText)
	}
	if !strings.Contains(updatedText, "/etc/cont-groups.d/cinit/id") {
		t.Fatalf("compose missing cont-groups mount:\n%s", updatedText)
	}
	if !strings.Contains(updatedText, "/etc/cont-users.d/root/home") {
		t.Fatalf("compose missing cont-users mount:\n%s", updatedText)
	}
}
