package stardew_junimo

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
)

type cleanupPendingLifecycleDocker struct {
	*fakeConsoleDocker
	removeErr      error
	removedHolders []string
	removedVolumes []string
}

func (f *cleanupPendingLifecycleDocker) RemoveSteamInviteAuthSessionHolders(_ context.Context, _, _, volume string) (paneldocker.CommandResult, error) {
	f.removedHolders = append(f.removedHolders, volume)
	return paneldocker.CommandResult{ExitCode: 0}, f.removeErr
}

func (f *cleanupPendingLifecycleDocker) RemoveVolumes(_ context.Context, _ string, volumes []string) (paneldocker.CommandResult, error) {
	f.removedVolumes = append(f.removedVolumes, volumes...)
	return paneldocker.CommandResult{ExitCode: 0}, nil
}

func writeSteamInviteRecoveryEnv(t *testing.T, dataDir, enabled, completed, state string) {
	t.Helper()
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("mkdir data dir: %v", err)
	}
	contents := "STEAM_INVITE_ENABLED=" + enabled + "\n" +
		"STEAM_AUTH_COMPLETED=" + completed + "\n" +
		"STEAM_INVITE_AUTH_STATE=" + state + "\n"
	if err := os.WriteFile(filepath.Join(dataDir, ".env"), []byte(contents), 0o600); err != nil {
		t.Fatalf("write env: %v", err)
	}
}

func TestRecoverInterruptedSteamInviteAuthorizationDisabledDoesNotProbeDocker(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "disabled-instance")
	writeSteamInviteRecoveryEnv(t, dataDir, "false", "", "disabled")
	fake := &fakeDocker{psErr: errors.New("must not probe")}
	driver := New(fake, nil, nil, nil)

	if err := driver.RecoverInterruptedSteamInviteAuthorization(context.Background(), registry.Instance{DataDir: dataDir}); err != nil {
		t.Fatalf("recover disabled instance: %v", err)
	}
	if fake.workDir != "" || len(fake.removedByVolumes) != 0 || len(fake.removedVolumes) != 0 {
		t.Fatalf("disabled recovery touched Docker: workDir=%q holders=%v volumes=%v", fake.workDir, fake.removedByVolumes, fake.removedVolumes)
	}
}

func TestRecoverInterruptedSteamInviteAuthorizationCompletedCrashDoesNotProbeOrDeleteSession(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "completed-crash")
	writeSteamInviteRecoveryEnv(t, dataDir, "true", "true", "authorizing")
	fake := &fakeDocker{psErr: errors.New("must not probe")}
	driver := New(fake, nil, nil, nil)

	if err := driver.RecoverInterruptedSteamInviteAuthorization(context.Background(), registry.Instance{DataDir: dataDir}); err != nil {
		t.Fatalf("recover completed authorization crash: %v", err)
	}
	if fake.workDir != "" || len(fake.removedByVolumes) != 0 || len(fake.removedVolumes) != 0 {
		t.Fatalf("completed authorization recovery touched Docker: workDir=%q holders=%v volumes=%v", fake.workDir, fake.removedByVolumes, fake.removedVolumes)
	}
}

func TestRecoverInterruptedSteamInviteAuthorizationConvergesSuccessfulHolderOnly(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "cleanup-pending")
	writeSteamInviteRecoveryEnv(t, dataDir, "true", "true", sjconfig.SteamInviteAuthStateCleanupPending)
	fake := &fakeDocker{}
	driver := New(fake, nil, nil, nil)

	if err := driver.RecoverInterruptedSteamInviteAuthorization(context.Background(), registry.Instance{DataDir: dataDir}); err != nil {
		t.Fatalf("recover successful holder cleanup: %v", err)
	}
	if want := []string{"cleanup-pending_steam-session"}; !reflect.DeepEqual(fake.removedByVolumes, want) {
		t.Fatalf("removed holders = %v, want %v", fake.removedByVolumes, want)
	}
	if len(fake.removedVolumes) != 0 {
		t.Fatalf("successful session volume was deleted: %v", fake.removedVolumes)
	}
	if got := sjconfig.SteamInviteAuthState(dataDir); got != sjconfig.SteamInviteAuthStateReady {
		t.Fatalf("recovered auth state = %q, want ready", got)
	}
}

func TestRecoverInterruptedSteamInviteAuthorizationCleanupPendingFailsClosed(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "unknown-holder")
	writeSteamInviteRecoveryEnv(t, dataDir, "true", "true", sjconfig.SteamInviteAuthStateCleanupPending)
	fake := &fakeDocker{removeContainersErr: errors.New("unknown holder")}
	driver := New(fake, nil, nil, nil)

	if err := driver.RecoverInterruptedSteamInviteAuthorization(context.Background(), registry.Instance{DataDir: dataDir}); !errors.Is(err, ErrSteamInviteCleanupPending) {
		t.Fatalf("recovery error = %v, want cleanup-pending sentinel", err)
	}
	if len(fake.removedVolumes) != 0 {
		t.Fatalf("failed holder classification deleted session volume: %v", fake.removedVolumes)
	}
	if got := sjconfig.SteamInviteAuthState(dataDir); got != sjconfig.SteamInviteAuthStateCleanupPending {
		t.Fatalf("failed convergence state = %q, want cleanup_pending", got)
	}
}

func TestRecoverInterruptedSteamInviteAuthorizationCleanupPendingRequiresSuccessfulSessionEvidence(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "cleanup-pending-without-session")
	writeSteamInviteRecoveryEnv(t, dataDir, "true", "", sjconfig.SteamInviteAuthStateCleanupPending)
	fake := &fakeDocker{psErr: errors.New("must not probe without completed-session evidence")}
	driver := New(fake, nil, nil, nil)

	if err := driver.RecoverInterruptedSteamInviteAuthorization(context.Background(), registry.Instance{DataDir: dataDir}); !errors.Is(err, ErrSteamInviteCleanupPending) {
		t.Fatalf("recovery error = %v, want cleanup-pending sentinel", err)
	}
	if fake.workDir != "" || len(fake.removedByVolumes) != 0 || len(fake.removedVolumes) != 0 {
		t.Fatalf("invalid cleanup-pending state touched Docker: workDir=%q holders=%v volumes=%v", fake.workDir, fake.removedByVolumes, fake.removedVolumes)
	}
}

func TestStartBlocksWhenSuccessfulAuthHolderCannotBeClassified(t *testing.T) {
	store, instance, dataDir := newInstalledAuthOnlyFixture(t)
	if err := sjconfig.SetSteamAuthLoggedIn(dataDir, true); err != nil {
		t.Fatal(err)
	}
	if err := sjconfig.SetSteamInviteAuthState(dataDir, sjconfig.SteamInviteAuthStateCleanupPending); err != nil {
		t.Fatal(err)
	}
	fake := &cleanupPendingLifecycleDocker{
		fakeConsoleDocker: &fakeConsoleDocker{},
		removeErr:         errors.New("unknown holder"),
	}
	driver := New(fake, slog.Default(), jobs.NewManager(store, slog.Default()), store)

	job, err := driver.Start(context.Background(), registry.StartRequest{Instance: registry.Instance{ID: instance.ID}})
	if job != nil || !errors.Is(err, ErrSteamInviteCleanupPending) {
		t.Fatalf("start during unresolved holder cleanup = job %#v, err %v", job, err)
	}
	if len(fake.removedHolders) != 1 || len(fake.removedVolumes) != 0 {
		t.Fatalf("start cleanup holders=%v volumes=%v", fake.removedHolders, fake.removedVolumes)
	}
	if got := sjconfig.SteamInviteAuthState(dataDir); got != sjconfig.SteamInviteAuthStateCleanupPending {
		t.Fatalf("blocked start state = %q, want cleanup_pending", got)
	}
}

func TestRecoverInterruptedSteamInviteAuthorizationRemovesExactFailedSession(t *testing.T) {
	for _, state := range []string{"failed", "authorizing"} {
		t.Run(state, func(t *testing.T) {
			dataDir := filepath.Join(t.TempDir(), "invite-recovery")
			writeSteamInviteRecoveryEnv(t, dataDir, "true", "", state)
			fake := &fakeDocker{}
			driver := New(fake, nil, nil, nil)

			if err := driver.RecoverInterruptedSteamInviteAuthorization(context.Background(), registry.Instance{DataDir: dataDir}); err != nil {
				t.Fatalf("recover %s authorization: %v", state, err)
			}
			want := []string{"invite-recovery_steam-session"}
			if !reflect.DeepEqual(fake.removedByVolumes, want) || !reflect.DeepEqual(fake.removedVolumes, want) {
				t.Fatalf("recovered resources holders=%v volumes=%v want=%v", fake.removedByVolumes, fake.removedVolumes, want)
			}
		})
	}
}

func TestRecoverInterruptedSteamInviteAuthorizationLeavesRunningServerUntouched(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "running-invite")
	writeSteamInviteRecoveryEnv(t, dataDir, "true", "", "failed")
	fake := &fakeDocker{psResult: paneldocker.ComposePsResult{Services: []paneldocker.ComposeService{{
		Service: "server", State: "running", Status: "Up 1 minute",
	}}}}
	driver := New(fake, nil, nil, nil)

	if err := driver.RecoverInterruptedSteamInviteAuthorization(context.Background(), registry.Instance{DataDir: dataDir}); err != nil {
		t.Fatalf("recover running instance: %v", err)
	}
	if len(fake.removedByVolumes) != 0 || len(fake.removedVolumes) != 0 {
		t.Fatalf("running server session was removed: holders=%v volumes=%v", fake.removedByVolumes, fake.removedVolumes)
	}
}

func TestRecoverInterruptedSteamInviteAuthorizationFailsClosed(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "failed-cleanup")
	writeSteamInviteRecoveryEnv(t, dataDir, "true", "", "failed")
	if err := sjconfig.MarkSteamInviteRuntimeScopeCurrent(dataDir); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(dataDir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	fake := &fakeDocker{removeContainersErr: errors.New("holder cleanup failed")}
	driver := New(fake, nil, nil, nil)

	if err := driver.RecoverInterruptedSteamInviteAuthorization(context.Background(), registry.Instance{DataDir: dataDir}); err == nil {
		t.Fatal("expected holder cleanup failure")
	}
	if len(fake.removedVolumes) != 0 {
		t.Fatalf("session volume removal continued after holder failure: %v", fake.removedVolumes)
	}
	after, err := os.ReadFile(filepath.Join(dataDir, ".env"))
	if err != nil || string(after) != string(before) {
		t.Fatalf("unknown holder rejection changed authorization state: before=%q after=%q err=%v", before, after, err)
	}
	if !sjconfig.SteamInviteRuntimeScopeCurrent(dataDir) {
		t.Fatal("unknown holder rejection removed the existing runtime scope marker")
	}
}
