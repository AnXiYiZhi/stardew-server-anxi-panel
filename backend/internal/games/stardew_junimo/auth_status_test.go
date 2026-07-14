package stardew_junimo

import (
	"context"
	"testing"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
)

func TestGetAuthStatus_MergesPasswordBridgeStatus(t *testing.T) {
	d := newTestDriver(&fakeConsoleDocker{
		execFunc: func(_ context.Context, _, _, _ string, _ ...string) (paneldocker.CommandResult, error) {
			return paneldocker.CommandResult{
				Stdout:   `{"enabled":true,"authenticatedCount":1,"pendingCount":2,"timeoutSeconds":120,"maxAttempts":5}`,
				ExitCode: 0,
			}, nil
		},
	})
	instance := makeRunningInstance()
	instance.DataDir = t.TempDir()
	writeStatusJSON(t, instance.DataDir, true, "OK")

	status, err := d.GetAuthStatus(context.Background(), instance)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.Enabled || status.PendingCount != 2 {
		t.Fatalf("unexpected REST fields: %+v", status)
	}
	if !status.PasswordBridgeAvailable {
		t.Fatalf("expected PasswordBridgeAvailable=true, got %+v", status)
	}
	if status.PasswordBridgeDetail != "OK" {
		t.Fatalf("detail = %q, want OK", status.PasswordBridgeDetail)
	}
}

func TestGetAuthStatus_PasswordBridgeUnavailableWhenStatusMissing(t *testing.T) {
	d := newTestDriver(&fakeConsoleDocker{
		execFunc: func(_ context.Context, _, _, _ string, _ ...string) (paneldocker.CommandResult, error) {
			return paneldocker.CommandResult{
				Stdout:   `{"enabled":false,"authenticatedCount":0,"pendingCount":0,"timeoutSeconds":0,"maxAttempts":0}`,
				ExitCode: 0,
			}, nil
		},
	})
	instance := makeRunningInstance()
	instance.DataDir = t.TempDir()
	// No status.json written: reflection bridge status defaults to unavailable.

	status, err := d.GetAuthStatus(context.Background(), instance)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.PasswordBridgeAvailable {
		t.Fatalf("expected PasswordBridgeAvailable=false, got %+v", status)
	}
}
