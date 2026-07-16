package docker

import (
	"context"
	"strings"
	"testing"
)

func TestRuntimeApplyDockerContractRejectsInjectedArguments(t *testing.T) {
	client := NewClient(Options{DockerPath: "docker-that-must-not-run"})
	ctx := context.Background()
	if err := client.RuntimeComposeUpService(ctx, t.TempDir(), "safe_project", "server;docker compose down -v"); err == nil {
		t.Fatal("injected service accepted")
	}
	if err := client.RuntimeComposeStopServices(ctx, t.TempDir(), "safe_project", "server", "server"); err == nil {
		t.Fatal("duplicate service accepted")
	}
	if err := client.RuntimeCreateSnapshotVolume(ctx, t.TempDir(), "safe_project", "production_steam-session"); err == nil {
		t.Fatal("unscoped snapshot accepted")
	}
	if err := client.RuntimeCloneVolume(ctx, t.TempDir(), "safe_project_steam-session", "safe_project_anxi-junimo-update-0123456789abcdef01234567-steam-session", "sdvd/server:latest"); err == nil {
		t.Fatal("latest clone image accepted")
	}
	if err := client.RuntimeRestoreVolume(ctx, t.TempDir(), "safe_project_anxi-junimo-update-0123456789abcdef01234567-steam-session", "safe_project_steam-session;rm", "sdvd/server:1.5.0-preview.121"); err == nil {
		t.Fatal("injected target volume accepted")
	}
}

func TestParseRuntimeServiceInspectOutputUsesOnlySafeFields(t *testing.T) {
	containerID := strings.Repeat("c", 64)
	imageID := "sha256:" + strings.Repeat("d", 64)
	metadata, err := parseRuntimeServiceInspectOutput(`"`+imageID+`"|"registry.example/auth:1.0.0"|"running"|"healthy"`, containerID)
	if err != nil {
		t.Fatal(err)
	}
	if metadata.ContainerID != containerID || metadata.ImageID != imageID || metadata.Image != "registry.example/auth:1.0.0" || metadata.State != "running" || metadata.Health != "healthy" {
		t.Fatalf("metadata=%+v", metadata)
	}
	if _, err := parseRuntimeServiceInspectOutput(`"bad"|"image"|"running"|""`, containerID); err == nil {
		t.Fatal("invalid image ID accepted")
	}
}

func TestRuntimeApplyServiceAllowlistIsPairOnly(t *testing.T) {
	if !validRuntimeServices([]string{"steam-auth", "server"}) {
		t.Fatal("required pair rejected")
	}
	for _, services := range [][]string{{"panel"}, {"server", "panel"}, {}, {"server", "steam-auth", "panel"}} {
		if validRuntimeServices(services) {
			t.Fatalf("invalid services accepted: %v", services)
		}
	}
}

func TestParseRuntimeSteamReadyResponseTreatsLoginStateAsCapability(t *testing.T) {
	ready, err := parseRuntimeSteamReadyResponse(`{"ready":false,"has_ticket":false}`)
	if err != nil || ready.Ready || ready.HasTicket {
		t.Fatalf("logged-out response should remain a valid service contract: ready=%+v err=%v", ready, err)
	}
	withoutTicket, err := parseRuntimeSteamReadyResponse(`{"ready":false,"error":"Account 0 not configured"}`)
	if err != nil || withoutTicket.Ready || withoutTicket.HasTicket {
		t.Fatalf("real logged-out auth contract should be accepted without has_ticket: ready=%+v err=%v", withoutTicket, err)
	}
	for _, invalid := range []string{`{}`, `{"has_ticket":false}`, `not-json`} {
		if _, err := parseRuntimeSteamReadyResponse(invalid); err == nil {
			t.Fatalf("incomplete auth contract accepted: %s", invalid)
		}
	}
	current, err := parseRuntimeSteamReadyResponse(`{"status":"ok","logged_in":true,"accounts":[]}`)
	if err != nil || !current.Ready || current.HasTicket {
		t.Fatalf("current logged-out auth contract should be accepted: ready=%+v err=%v", current, err)
	}
	for _, invalid := range []string{
		`{"status":"ok","logged_in":true}`,
		`{"status":"ok","accounts":[]}`,
		`{"status":"failed","logged_in":false,"accounts":[]}`,
	} {
		if _, err := parseRuntimeSteamReadyResponse(invalid); err == nil {
			t.Fatalf("incomplete current auth contract accepted: %s", invalid)
		}
	}
}
