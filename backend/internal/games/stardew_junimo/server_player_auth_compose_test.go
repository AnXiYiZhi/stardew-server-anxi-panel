package stardew_junimo

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestEnsureServerPlayerAuthEnvironmentMigratesLegacyComposeIdempotently(t *testing.T) {
	dataDir := t.TempDir()
	path := filepath.Join(dataDir, "docker-compose.yml")
	legacy := `services:
  steam-auth:
    image: auth:test
  server:
    image: server:test
    environment:
      SERVER_PASSWORD: "${SERVER_PASSWORD:-}"
      SAP_CONTROL_DIR: /data/control
    volumes:
      - ./.local-container/control:/data/control
`
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}

	changed, err := EnsureServerPlayerAuthEnvironment(dataDir)
	if err != nil || !changed {
		t.Fatalf("first migration changed=%v err=%v", changed, err)
	}
	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(updated)
	for _, entry := range playerAuthComposeEnvironment {
		if strings.Count(text, entry.mappingLine) != 1 {
			t.Fatalf("compose should contain one %s mapping:\n%s", entry.key, text)
		}
	}
	if !strings.Contains(text, `      SERVER_PASSWORD: "${SERVER_PASSWORD:-}"`) ||
		!strings.Contains(text, `      - ./.local-container/control:/data/control`) {
		t.Fatalf("migration lost existing content:\n%s", text)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("compose mode=%v, want 0600", info.Mode().Perm())
	}

	changed, err = EnsureServerPlayerAuthEnvironment(dataDir)
	if err != nil || changed {
		t.Fatalf("second migration changed=%v err=%v", changed, err)
	}
}

func TestEnsureServerPlayerAuthEnvironmentPreservesCRLFAndListSyntax(t *testing.T) {
	dataDir := t.TempDir()
	path := filepath.Join(dataDir, "docker-compose.yml")
	legacy := "services:\r\n  server:\r\n    image: server:test\r\n    environment:\r\n      - EXISTING=value\r\n    volumes:\r\n      - data:/data\r\n"
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, err := EnsureServerPlayerAuthEnvironment(dataDir)
	if err != nil || !changed {
		t.Fatalf("migration changed=%v err=%v", changed, err)
	}
	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(updated)
	if strings.Contains(strings.ReplaceAll(text, "\r\n", ""), "\n") {
		t.Fatalf("migration introduced mixed line endings: %q", text)
	}
	for _, entry := range playerAuthComposeEnvironment {
		want := "      - " + entry.key + "=${" + entry.key + ":-}"
		if strings.Count(text, want) != 1 {
			t.Fatalf("compose should contain one %s list entry:\n%s", entry.key, text)
		}
	}
}

func TestEnsureServerPlayerAuthEnvironmentAddsMissingEnvironmentBlock(t *testing.T) {
	dataDir := t.TempDir()
	path := filepath.Join(dataDir, "docker-compose.yml")
	legacy := "services:\n  server:\n    image: server:test\n    volumes:\n      - data:/data\n"
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, err := EnsureServerPlayerAuthEnvironment(dataDir)
	if err != nil || !changed {
		t.Fatalf("migration changed=%v err=%v", changed, err)
	}
	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(updated)
	if strings.Index(text, "    environment:\n") > strings.Index(text, "    volumes:\n") {
		t.Fatalf("environment block must precede volumes:\n%s", text)
	}
	for _, entry := range playerAuthComposeEnvironment {
		if !strings.Contains(text, entry.mappingLine) {
			t.Fatalf("compose missing %s:\n%s", entry.key, text)
		}
	}
}

func TestEnsureServerPlayerAuthEnvironmentRejectsUnsupportedInlineEnvironmentWithoutChanges(t *testing.T) {
	dataDir := t.TempDir()
	path := filepath.Join(dataDir, "docker-compose.yml")
	original := "services:\n  server:\n    image: server:test\n    environment: { EXISTING: value }\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, err := EnsureServerPlayerAuthEnvironment(dataDir)
	if err == nil || changed {
		t.Fatalf("unsupported migration changed=%v err=%v", changed, err)
	}
	updated, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(updated) != original {
		t.Fatalf("unsupported compose was modified:\n%s", updated)
	}
}

func TestEnsureServerPlayerAuthEnvironmentLeavesCurrentTemplateUntouched(t *testing.T) {
	dataDir := t.TempDir()
	path := filepath.Join(dataDir, "docker-compose.yml")
	if err := os.WriteFile(path, []byte(junimoComposeTemplate), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, err := EnsureServerPlayerAuthEnvironment(dataDir)
	if err != nil || changed {
		t.Fatalf("current template changed=%v err=%v", changed, err)
	}
}
