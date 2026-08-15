package stardew_junimo

import (
	"context"
	"path/filepath"
	"testing"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
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

func TestGetAuthStatus_MergesPlayerAuthRuntimeStatus(t *testing.T) {
	d := newTestDriver(&fakeConsoleDocker{
		execFunc: func(_ context.Context, _, _, _ string, _ ...string) (paneldocker.CommandResult, error) {
			return paneldocker.CommandResult{
				Stdout:   `{"enabled":true,"authenticatedCount":2,"pendingCount":1,"timeoutSeconds":120,"maxAttempts":3}`,
				ExitCode: 0,
			}, nil
		},
	})
	instance := makeRunningInstance()
	instance.DataDir = t.TempDir()
	if err := sjconfig.UpdateEnvFile(filepath.Join(instance.DataDir, ".env"), map[string]string{
		"SAP_PLAYER_AUTH_MODE":     PlayerAuthModeGlobal,
		"SAP_PLAYER_AUTH_REVISION": "configured-v2",
		"SERVER_PASSWORD":          "shared-secret",
	}); err != nil {
		t.Fatalf("write player auth env: %v", err)
	}
	writeStatusJSONFields(t, instance.DataDir, map[string]any{
		"passwordBridgeAvailable":    true,
		"passwordBridgeDetail":       "OK",
		"playerAuthMode":             PlayerAuthModeGlobal,
		"playerAuthConfigRevision":   "runtime-v1",
		"rolePasswordPatchAvailable": true,
		"rolePasswordPatchDetail":    "OK",
	})

	status, err := d.GetAuthStatus(context.Background(), instance)
	if err != nil {
		t.Fatalf("GetAuthStatus: %v", err)
	}
	if status.ConfiguredMode != PlayerAuthModeGlobal || status.ConfiguredRevision != "configured-v2" {
		t.Fatalf("unexpected configured auth status: %+v", status)
	}
	if status.RuntimeMode != PlayerAuthModeGlobal || status.RuntimeRevision != "runtime-v1" {
		t.Fatalf("unexpected runtime auth status: %+v", status)
	}
	if !status.RestartRequired || !status.RolePasswordPatchReady || status.RolePasswordPatchDetail != "OK" {
		t.Fatalf("expected restart and patch status, got %+v", status)
	}
}
