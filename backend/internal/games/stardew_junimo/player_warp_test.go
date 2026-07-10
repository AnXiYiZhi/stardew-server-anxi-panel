package stardew_junimo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeWarpHomeStatusJSON(t *testing.T, dataDir string, available bool, detail string) {
	t.Helper()
	control := filepath.Join(dataDir, ".local-container", "control")
	if err := os.MkdirAll(control, 0o755); err != nil {
		t.Fatalf("mkdir control: %v", err)
	}
	raw, err := json.Marshal(map[string]any{
		"state":                   "running",
		"warpHomeBridgeAvailable": available,
		"warpHomeBridgeDetail":    detail,
	})
	if err != nil {
		t.Fatalf("marshal status.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(control, "status.json"), raw, 0o644); err != nil {
		t.Fatalf("write status.json: %v", err)
	}
}

func TestReadWarpHomeBridgeStatus_MissingFile(t *testing.T) {
	status := readWarpHomeBridgeStatus(t.TempDir())
	if status.Available {
		t.Fatalf("expected Available=false when status.json is missing, got %+v", status)
	}
}

func TestReadWarpHomeBridgeStatus_ParsesFields(t *testing.T) {
	dir := t.TempDir()
	writeWarpHomeStatusJSON(t, dir, true, "OK")

	status := readWarpHomeBridgeStatus(dir)
	if !status.Available {
		t.Fatalf("expected Available=true, got %+v", status)
	}
	if status.Detail != "OK" {
		t.Fatalf("detail = %q, want OK", status.Detail)
	}
}

func TestWarpPlayerHome_RequiresPlayerID(t *testing.T) {
	instance := makeRunningInstance()
	instance.DataDir = t.TempDir()

	_, err := warpPlayerHome(instance, "  ", "Abigail")
	if err == nil {
		t.Fatal("expected error for empty player id")
	}
	ce := err.(*CommandError)
	if ce.Code != "invalid_player" {
		t.Errorf("expected code 'invalid_player', got %q", ce.Code)
	}
}

func TestWarpPlayerHome_ServerNotRunning(t *testing.T) {
	instance := makeStoppedInstance()

	_, err := warpPlayerHome(instance, "12345", "Abigail")
	if err == nil {
		t.Fatal("expected error for stopped server")
	}
	ce := err.(*CommandError)
	if ce.Code != "server_not_running" {
		t.Errorf("expected code 'server_not_running', got %q", ce.Code)
	}
}

func TestWarpPlayerHome_RequiresWarpHomeBridgeAvailable(t *testing.T) {
	instance := makeRunningInstance()
	instance.DataDir = t.TempDir()

	_, err := warpPlayerHome(instance, "12345", "Abigail")
	if err == nil {
		t.Fatal("expected error when warp-home bridge is unavailable")
	}
	ce := err.(*CommandError)
	if ce.Code != "warp_home_bridge_unavailable" {
		t.Errorf("expected code 'warp_home_bridge_unavailable', got %q", ce.Code)
	}
}

func TestWarpPlayerHome_WritesCommandFile(t *testing.T) {
	instance := makeRunningInstance()
	instance.DataDir = t.TempDir()
	writeWarpHomeStatusJSON(t, instance.DataDir, true, "OK")

	result, err := warpPlayerHome(instance, "620826087702429092", "Abigail")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Command != "warp-home" || result.ExitCode != 0 {
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
	if command.Name != "warp-home" {
		t.Fatalf("command name = %q, want warp-home", command.Name)
	}
	if command.Payload["uniqueMultiplayerId"] != "620826087702429092" {
		t.Fatalf("payload uniqueMultiplayerId = %q, want 620826087702429092", command.Payload["uniqueMultiplayerId"])
	}
	if command.Payload["name"] != "Abigail" {
		t.Fatalf("payload name = %q, want Abigail", command.Payload["name"])
	}
}
