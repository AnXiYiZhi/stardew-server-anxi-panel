package stardew_junimo

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
)

func writeHostPlayersJSON(t *testing.T, dataDir string) {
	t.Helper()
	control := filepath.Join(dataDir, ".local-container", "control")
	if err := os.MkdirAll(control, 0o755); err != nil {
		t.Fatalf("mkdir control: %v", err)
	}
	raw := `{
  "updatedAt": "2026-07-10T10:00:00Z",
  "saveId": "test",
  "players": [
    {"name": "host", "uniqueMultiplayerId": "1", "isHost": true}
  ]
}`
	if err := os.WriteFile(filepath.Join(control, "players.json"), []byte(raw), 0o644); err != nil {
		t.Fatalf("write players.json: %v", err)
	}
}

func TestBanPlayer_RequiresName(t *testing.T) {
	d := newTestDriver(&fakeConsoleDocker{})
	instance := makeRunningInstance()
	instance.DataDir = t.TempDir()

	_, err := banPlayer(context.Background(), d, instance, "  ")
	if err == nil {
		t.Fatal("expected error for empty name")
	}
	ce := err.(*CommandError)
	if ce.Code != "invalid_player" {
		t.Errorf("expected code 'invalid_player', got %q", ce.Code)
	}
}

func TestBanPlayer_ServerNotRunning(t *testing.T) {
	d := newTestDriver(&fakeConsoleDocker{})
	instance := makeStoppedInstance()

	_, err := banPlayer(context.Background(), d, instance, "griefer")
	if err == nil {
		t.Fatal("expected error for stopped server")
	}
	ce := err.(*CommandError)
	if ce.Code != "server_not_running" {
		t.Errorf("expected code 'server_not_running', got %q", ce.Code)
	}
}

func TestBanPlayer_HostUnknown(t *testing.T) {
	d := newTestDriver(&fakeConsoleDocker{})
	instance := makeRunningInstance()
	instance.DataDir = t.TempDir()
	// No players.json written: findHostPlayerID cannot resolve the host.

	_, err := banPlayer(context.Background(), d, instance, "griefer")
	if err == nil {
		t.Fatal("expected error when host is unknown")
	}
	ce := err.(*CommandError)
	if ce.Code != "host_unknown" {
		t.Errorf("expected code 'host_unknown', got %q", ce.Code)
	}
}

func TestBanPlayer_PromotesHostThenWritesCommandFile(t *testing.T) {
	d := newTestDriver(&fakeConsoleDocker{
		execFunc: func(_ context.Context, _, _, _ string, args ...string) (paneldocker.CommandResult, error) {
			joined := strings.Join(args, " ")
			if !strings.Contains(joined, "/roles/admin") {
				t.Fatalf("expected roles/admin request, got args: %v", args)
			}
			return paneldocker.CommandResult{ExitCode: 0}, nil
		},
	})
	instance := makeRunningInstance()
	instance.DataDir = t.TempDir()
	writeHostPlayersJSON(t, instance.DataDir)

	result, err := banPlayer(context.Background(), d, instance, "griefer")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Command != "ban" || result.ExitCode != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}

	files, err := os.ReadDir(filepath.Join(instance.DataDir, ".local-container", "control", "commands"))
	if err != nil {
		t.Fatalf("read commands dir: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 command file, got %d", len(files))
	}

	raw, err := os.ReadFile(filepath.Join(instance.DataDir, ".local-container", "control", "commands", files[0].Name()))
	if err != nil {
		t.Fatalf("read command file: %v", err)
	}
	var command struct {
		Name    string            `json:"name"`
		Payload map[string]string `json:"payload"`
	}
	if err := json.Unmarshal(raw, &command); err != nil {
		t.Fatalf("unmarshal command file: %v", err)
	}
	if command.Name != "ban" {
		t.Fatalf("command name = %q, want ban", command.Name)
	}
	if command.Payload["name"] != "griefer" {
		t.Fatalf("payload name = %q, want griefer", command.Payload["name"])
	}
}
