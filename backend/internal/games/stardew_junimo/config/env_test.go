package config_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
)

func TestReadEnvFile_NotExist(t *testing.T) {
	fields, err := config.ReadEnvFile(filepath.Join(t.TempDir(), "missing.env"))
	if err != nil {
		t.Fatalf("ReadEnvFile on nonexistent file: %v", err)
	}
	if len(fields) != 0 {
		t.Errorf("expected empty map, got %v", fields)
	}
}

func TestUpdateEnvFile_NewFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")

	if err := config.UpdateEnvFile(path, map[string]string{
		"STEAM_USERNAME": "testuser",
		"STEAM_PASSWORD": "s3cr3t",
		"VNC_PASSWORD":   "vncp4ss",
	}); err != nil {
		t.Fatalf("UpdateEnvFile: %v", err)
	}

	fields, err := config.ReadEnvFile(path)
	if err != nil {
		t.Fatalf("ReadEnvFile: %v", err)
	}
	if fields["STEAM_USERNAME"] != "testuser" {
		t.Errorf("STEAM_USERNAME = %q, want %q", fields["STEAM_USERNAME"], "testuser")
	}
	if fields["STEAM_PASSWORD"] == "" {
		t.Error("STEAM_PASSWORD should not be empty after write")
	}
	if fields["VNC_PASSWORD"] == "" {
		t.Error("VNC_PASSWORD should not be empty after write")
	}

	// Verify 0600 permissions (Unix only; Windows does not map Unix mode bits).
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat .env: %v", err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf(".env permissions = %o, want 0600", perm)
		}
	}
}

func TestUpdateEnvFile_UpdatesExistingField(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")

	if err := config.UpdateEnvFile(path, map[string]string{
		"STEAM_USERNAME": "olduser",
		"VNC_PASSWORD":   "oldvnc",
	}); err != nil {
		t.Fatalf("initial write: %v", err)
	}

	if err := config.UpdateEnvFile(path, map[string]string{
		"STEAM_USERNAME": "newuser",
	}); err != nil {
		t.Fatalf("update write: %v", err)
	}

	fields, err := config.ReadEnvFile(path)
	if err != nil {
		t.Fatalf("ReadEnvFile: %v", err)
	}
	if fields["STEAM_USERNAME"] != "newuser" {
		t.Errorf("STEAM_USERNAME = %q, want %q", fields["STEAM_USERNAME"], "newuser")
	}
	if fields["VNC_PASSWORD"] != "oldvnc" {
		t.Errorf("VNC_PASSWORD should be preserved, got %q", fields["VNC_PASSWORD"])
	}
}

func TestUpdateEnvFile_PreservesUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")

	if err := os.WriteFile(path, []byte("CUSTOM_KEY=custom_value\nSTEAM_USERNAME=olduser\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := config.UpdateEnvFile(path, map[string]string{
		"STEAM_USERNAME": "newuser",
	}); err != nil {
		t.Fatalf("UpdateEnvFile: %v", err)
	}

	fields, err := config.ReadEnvFile(path)
	if err != nil {
		t.Fatalf("ReadEnvFile: %v", err)
	}
	if fields["CUSTOM_KEY"] != "custom_value" {
		t.Errorf("CUSTOM_KEY should be preserved, got %q", fields["CUSTOM_KEY"])
	}
	if fields["STEAM_USERNAME"] != "newuser" {
		t.Errorf("STEAM_USERNAME = %q, want %q", fields["STEAM_USERNAME"], "newuser")
	}
}

func TestEmptyEnvTemplate_UsesOfficialJunimoKeys(t *testing.T) {
	fields := config.EmptyEnvTemplate()
	for _, key := range []string{
		"IMAGE_VERSION",
		"STEAM_USERNAME",
		"STEAM_PASSWORD",
		"STEAM_REFRESH_TOKEN",
		"STEAM_KEEP_LANGUAGES",
		"VNC_PASSWORD",
		"GAME_PORT",
		"QUERY_PORT",
		"VNC_PORT",
		"API_PORT",
		"STEAM_AUTH_PORT",
		"SERVER_TPS",
		"SERVER_FPS",
		"SERVER_PASSWORD",
		"MAX_LOGIN_ATTEMPTS",
		"AUTH_TIMEOUT_SECONDS",
		"API_ENABLED",
		"API_KEY",
		"ALLOW_INSECURE_SETUP",
	} {
		if _, ok := fields[key]; !ok {
			t.Fatalf("official Junimo env key %s missing", key)
		}
	}
	if _, ok := fields["JUNIMO_IMAGE_TAG"]; ok {
		t.Fatal("JUNIMO_IMAGE_TAG should not be used; Junimo expects IMAGE_VERSION")
	}
	if fields["GAME_PORT"] != "24642" || fields["QUERY_PORT"] != "27015" || fields["API_ENABLED"] != "true" {
		t.Fatalf("unexpected defaults: %#v", fields)
	}
}
