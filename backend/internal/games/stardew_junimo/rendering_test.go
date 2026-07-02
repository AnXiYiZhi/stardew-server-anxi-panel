package stardew_junimo

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
)

func TestSetRenderingFPSCallsJunimoAPIInsideServerContainer(t *testing.T) {
	var capturedArgs []string
	instance := makeRunningInstance()
	instance.DataDir = t.TempDir()
	if err := os.WriteFile(filepath.Join(instance.DataDir, ".env"), []byte("API_PORT=18080\nAPI_KEY=secret\n"), 0o600); err != nil {
		t.Fatalf("write env: %v", err)
	}
	d := newTestDriver(&fakeConsoleDocker{
		execFunc: func(_ context.Context, dir, service, stdinData string, args ...string) (paneldocker.CommandResult, error) {
			if dir != instance.DataDir || service != "server" || stdinData != "" {
				t.Fatalf("unexpected exec target dir=%q service=%q stdin=%q", dir, service, stdinData)
			}
			capturedArgs = append([]string(nil), args...)
			return paneldocker.CommandResult{Stdout: `{"fps":15}`, ExitCode: 0}, nil
		},
	})

	result, err := d.SetRenderingFPS(context.Background(), instance, 15)
	if err != nil {
		t.Fatalf("SetRenderingFPS returned error: %v", err)
	}
	if result.FPS != 15 {
		t.Fatalf("fps = %d, want 15", result.FPS)
	}
	want := []string{
		"curl", "-sf", "-X", "POST", "-H", "Content-Length: 0",
		"-H", "Authorization: Bearer secret",
		"http://localhost:18080/rendering?fps=15",
	}
	if !reflect.DeepEqual(capturedArgs, want) {
		t.Fatalf("args = %#v, want %#v", capturedArgs, want)
	}
}

func TestGetRenderingFPSCallsJunimoAPIInsideServerContainer(t *testing.T) {
	var capturedArgs []string
	instance := makeRunningInstance()
	instance.DataDir = t.TempDir()
	if err := os.WriteFile(filepath.Join(instance.DataDir, ".env"), []byte("API_PORT=18080\nAPI_KEY=secret\n"), 0o600); err != nil {
		t.Fatalf("write env: %v", err)
	}
	d := newTestDriver(&fakeConsoleDocker{
		execFunc: func(_ context.Context, dir, service, stdinData string, args ...string) (paneldocker.CommandResult, error) {
			if dir != instance.DataDir || service != "server" || stdinData != "" {
				t.Fatalf("unexpected exec target dir=%q service=%q stdin=%q", dir, service, stdinData)
			}
			capturedArgs = append([]string(nil), args...)
			return paneldocker.CommandResult{Stdout: `{"fps":15}`, ExitCode: 0}, nil
		},
	})

	result, err := d.GetRenderingFPS(context.Background(), instance)
	if err != nil {
		t.Fatalf("GetRenderingFPS returned error: %v", err)
	}
	if result.FPS != 15 {
		t.Fatalf("fps = %d, want 15", result.FPS)
	}
	want := []string{
		"curl", "-sf", "-X", "GET",
		"-H", "Authorization: Bearer secret",
		"http://localhost:18080/rendering",
	}
	if !reflect.DeepEqual(capturedArgs, want) {
		t.Fatalf("args = %#v, want %#v", capturedArgs, want)
	}
}

func TestSetRenderingFPSRejectsStoppedServer(t *testing.T) {
	d := newTestDriver(&fakeConsoleDocker{})
	_, err := d.SetRenderingFPS(context.Background(), makeStoppedInstance(), 15)
	if err == nil {
		t.Fatal("expected stopped server error")
	}
	ce, ok := err.(*CommandError)
	if !ok || ce.Code != "server_not_running" {
		t.Fatalf("error = %#v, want server_not_running CommandError", err)
	}
}
