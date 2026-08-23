package docker

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestRuntimeApplyDockerContractRejectsInjectedArguments(t *testing.T) {
	client := NewClient(Options{DockerPath: "docker-that-must-not-run"})
	ctx := context.Background()
	if err := client.RuntimeComposeUpService(ctx, t.TempDir(), "safe_project", "server;docker compose down -v"); err == nil {
		t.Fatal("injected service accepted")
	}
	if err := client.RuntimeComposeUpServicePreserve(ctx, t.TempDir(), "safe_project", "steam-auth;docker compose down -v"); err == nil {
		t.Fatal("injected preserved service accepted")
	}
	if err := client.RuntimeUpdateServiceCPUShares(ctx, t.TempDir(), "safe_project", "steam-auth", 768); err == nil {
		t.Fatal("invalid auth cpu shares accepted")
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

func TestParseRuntimeHostCapacityUsesOnlyNumericFields(t *testing.T) {
	capacity, err := parseRuntimeHostCapacity("2|1728053248\n")
	if err != nil || capacity.CPUs != 2 || capacity.MemoryBytes != 1728053248 {
		t.Fatalf("capacity=%+v err=%v", capacity, err)
	}
	for _, invalid := range []string{"", "0|1024", "2|0", "two|1024", "2|1024|extra"} {
		if _, err := parseRuntimeHostCapacity(invalid); err == nil {
			t.Fatalf("invalid capacity accepted: %q", invalid)
		}
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

func TestParseRuntimeAuthHealthAcceptsStrictLoggedOutAndLoggedInContracts(t *testing.T) {
	loggedOut, err := parseRuntimeAuthHealthHTTPResponse("HTTP/1.1 200 OK\r\n" + `{"status":"ok","logged_in":false,"accounts":[]}`)
	if err != nil || loggedOut.LoggedIn || loggedOut.AccountCount != 0 {
		t.Fatalf("logged-out health=%+v err=%v", loggedOut, err)
	}
	loggedIn, err := parseRuntimeAuthHealthHTTPResponse("HTTP/1.0 200 OK\r\n" + `{"status":"ok","logged_in":true,"accounts":[{"index":0}]}`)
	if err != nil || !loggedIn.LoggedIn || loggedIn.AccountCount != 1 {
		t.Fatalf("logged-in health=%+v err=%v", loggedIn, err)
	}
}

func TestRuntimeAuthHealthCommandErrorRecognizesContainerProbeDeadline(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code string
	}{
		{name: "host command deadline", err: ErrCommandTimeout, code: "auth_health_timeout"},
		{name: "container watchdog", err: CommandError{Result: CommandResult{ExitCode: 124}, Err: ErrCommandFailed}, code: "auth_health_timeout"},
		{name: "unreachable", err: CommandError{Result: CommandResult{ExitCode: 1}, Err: ErrCommandFailed}, code: "auth_health_unreachable"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := runtimeAuthHealthCommandError(test.err)
			var healthErr *RuntimeAuthHealthError
			if !errors.As(err, &healthErr) || healthErr.Code != test.code {
				t.Fatalf("error=%v typed=%+v, want %s", err, healthErr, test.code)
			}
		})
	}
}

func TestParseRuntimeAuthHealthRejectsUnsupportedHTTPAndJSON(t *testing.T) {
	for _, test := range []struct {
		name     string
		response string
		code     string
	}{
		{name: "non 200", response: "HTTP/1.1 503 Service Unavailable\r\n" + `{"status":"ok","logged_in":false,"accounts":[]}`, code: "auth_health_http_status"},
		{name: "empty", response: "", code: "auth_health_invalid_response"},
		{name: "status line only", response: "HTTP/1.1 200 OK\r\n", code: "auth_health_invalid_response"},
		{name: "non json", response: "HTTP/1.1 200 OK\r\nnot-json", code: "auth_health_invalid_response"},
		{name: "missing status", response: "HTTP/1.1 200 OK\r\n" + `{"logged_in":false,"accounts":[]}`, code: "auth_health_invalid_response"},
		{name: "missing logged in", response: "HTTP/1.1 200 OK\r\n" + `{"status":"ok","accounts":[]}`, code: "auth_health_invalid_response"},
		{name: "missing accounts", response: "HTTP/1.1 200 OK\r\n" + `{"status":"ok","logged_in":false}`, code: "auth_health_invalid_response"},
		{name: "status null", response: "HTTP/1.1 200 OK\r\n" + `{"status":null,"logged_in":false,"accounts":[]}`, code: "auth_health_invalid_response"},
		{name: "status wrong", response: "HTTP/1.1 200 OK\r\n" + `{"status":"failed","logged_in":false,"accounts":[]}`, code: "auth_health_invalid_response"},
		{name: "status wrong case", response: "HTTP/1.1 200 OK\r\n" + `{"status":"OK","logged_in":false,"accounts":[]}`, code: "auth_health_invalid_response"},
		{name: "logged in null", response: "HTTP/1.1 200 OK\r\n" + `{"status":"ok","logged_in":null,"accounts":[]}`, code: "auth_health_invalid_response"},
		{name: "logged in string", response: "HTTP/1.1 200 OK\r\n" + `{"status":"ok","logged_in":"false","accounts":[]}`, code: "auth_health_invalid_response"},
		{name: "logged in number", response: "HTTP/1.1 200 OK\r\n" + `{"status":"ok","logged_in":0,"accounts":[]}`, code: "auth_health_invalid_response"},
		{name: "accounts null", response: "HTTP/1.1 200 OK\r\n" + `{"status":"ok","logged_in":false,"accounts":null}`, code: "auth_health_invalid_response"},
		{name: "accounts object", response: "HTTP/1.1 200 OK\r\n" + `{"status":"ok","logged_in":false,"accounts":{}}`, code: "auth_health_invalid_response"},
		{name: "accounts string", response: "HTTP/1.1 200 OK\r\n" + `{"status":"ok","logged_in":false,"accounts":"none"}`, code: "auth_health_invalid_response"},
		{name: "accounts number", response: "HTTP/1.1 200 OK\r\n" + `{"status":"ok","logged_in":false,"accounts":0}`, code: "auth_health_invalid_response"},
		{name: "trailing json", response: "HTTP/1.1 200 OK\r\n" + `{"status":"ok","logged_in":false,"accounts":[]} {}`, code: "auth_health_invalid_response"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseRuntimeAuthHealthHTTPResponse(test.response)
			var healthErr *RuntimeAuthHealthError
			if !errors.As(err, &healthErr) || healthErr.Code != test.code {
				t.Fatalf("error=%v typed=%+v, want %s", err, healthErr, test.code)
			}
		})
	}
}
