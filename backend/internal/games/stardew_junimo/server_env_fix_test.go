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

	scriptPath := filepath.Join(dataDir, filepath.FromSlash(serverAppNameContEnvFile))
	script, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("APP_NAME script not written: %v", err)
	}
	if string(script) != serverAppNameScript {
		t.Fatalf("unexpected APP_NAME script:\n%s", script)
	}
	updated, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(updated), serverAppNameComposeMount) {
		t.Fatalf("compose missing APP_NAME mount:\n%s", updated)
	}

	changed, err = EnsureServerContEnvFix(dataDir)
	if err != nil {
		t.Fatalf("EnsureServerContEnvFix second run: %v", err)
	}
	if changed {
		t.Fatal("expected second run to be idempotent")
	}
}
