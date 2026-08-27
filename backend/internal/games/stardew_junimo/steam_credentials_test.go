package stardew_junimo

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func seedSteamCredentialEnv(t *testing.T, dataDir string) {
	t.Helper()
	if err := sjconfig.UpdateEnvFile(filepath.Join(dataDir, ".env"), map[string]string{
		"STEAM_USERNAME": "old-user",
		"STEAM_PASSWORD": "old-pass",
	}); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateSteamCredentialsPreservesAuthorizationState(t *testing.T) {
	store, instance, dataDir := newInstalledAuthOnlyFixture(t)
	if err := sjconfig.UpdateEnvFile(filepath.Join(dataDir, ".env"), map[string]string{
		"STEAMCMD_AUTH_COMPLETED": "true",
		"STEAM_AUTH_COMPLETED":    "true",
		"STEAM_INVITE_ENABLED":    "true",
		"STEAM_INVITE_AUTH_STATE": sjconfig.SteamInviteAuthStateReady,
		"STEAM_REFRESH_TOKEN":     "preserve-session",
	}); err != nil {
		t.Fatal(err)
	}
	driver := New(&fakeDocker{}, slog.Default(), jobs.NewManager(store, slog.Default()), store)

	if err := driver.UpdateSteamCredentials(context.Background(), registry.Instance{ID: instance.ID, DataDir: dataDir}, "new-user", "new-pass"); err != nil {
		t.Fatalf("update credentials: %v", err)
	}
	fields, err := sjconfig.ReadEnvFile(filepath.Join(dataDir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if fields["STEAM_USERNAME"] != "new-user" || fields["STEAM_PASSWORD"] != "new-pass" {
		t.Fatalf("credential fields = %q/%q", fields["STEAM_USERNAME"], fields["STEAM_PASSWORD"])
	}
	if fields["STEAMCMD_AUTH_COMPLETED"] != "true" || fields["STEAM_AUTH_COMPLETED"] != "true" ||
		fields["STEAM_INVITE_AUTH_STATE"] != sjconfig.SteamInviteAuthStateReady || fields["STEAM_REFRESH_TOKEN"] != "preserve-session" {
		t.Fatalf("authorization state changed: %#v", fields)
	}
}

func TestUpdateSteamCredentialsRechecksActiveConsumersInsideDriver(t *testing.T) {
	store, instance, dataDir := newInstalledAuthOnlyFixture(t)
	seedSteamCredentialEnv(t, dataDir)
	manager := jobs.NewManager(store, slog.Default())
	release := make(chan struct{})
	job, err := manager.Start(context.Background(), jobs.Spec{
		Type: "stardew_steam_auth", TargetType: "instance", TargetID: instance.ID,
		Run: func(ctx context.Context, _ *jobs.Context) error {
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		close(release)
		waitForDriverTestJobStatus(t, store, job.ID, storage.JobStatusSucceeded)
	})
	driver := New(&fakeDocker{}, slog.Default(), manager, store)

	err = driver.UpdateSteamCredentials(context.Background(), registry.Instance{ID: instance.ID, DataDir: dataDir}, "new-user", "new-pass")
	if !errors.Is(err, ErrSteamCredentialsInUse) {
		t.Fatalf("active consumer error = %v, want credentials-in-use", err)
	}
	fields, readErr := sjconfig.ReadEnvFile(filepath.Join(dataDir, ".env"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if fields["STEAM_USERNAME"] == "new-user" || fields["STEAM_PASSWORD"] == "new-pass" {
		t.Fatal("credentials changed while an Auth job was active")
	}
}

func TestUpdateSteamCredentialsRechecksInstallationStateInsideDriverLock(t *testing.T) {
	store, instance, dataDir := newInstalledAuthOnlyFixture(t)
	seedSteamCredentialEnv(t, dataDir)
	driver := New(&fakeDocker{}, slog.Default(), jobs.NewManager(store, slog.Default()), store)
	driver.runtimeUpdateMu.Lock()
	locked := true
	defer func() {
		if locked {
			driver.runtimeUpdateMu.Unlock()
		}
	}()

	attempted := make(chan struct{})
	updated := make(chan error, 1)
	go func() {
		close(attempted)
		updated <- driver.UpdateSteamCredentials(context.Background(), registry.Instance{ID: instance.ID, DataDir: dataDir}, "new-user", "new-pass")
	}()
	<-attempted
	if _, err := store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID:            instance.ID,
		State:         storage.InstanceStateError,
		StateMessage:  "base installation became unavailable",
		DriverPhase:   "install_failed",
		DriverPayload: instance.DriverPayload,
	}); err != nil {
		t.Fatal(err)
	}
	driver.runtimeUpdateMu.Unlock()
	locked = false

	if err := <-updated; !errors.Is(err, ErrSteamCredentialsInstallationRequired) {
		t.Fatalf("credential update after installation state changed = %v, want installation-required", err)
	}
	fields, err := sjconfig.ReadEnvFile(filepath.Join(dataDir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if fields["STEAM_USERNAME"] == "new-user" || fields["STEAM_PASSWORD"] == "new-pass" {
		t.Fatal("credentials changed after the base installation state became unavailable")
	}
}

func TestUpdateSteamCredentialsHoldsLifecycleLinearizationLockThroughWrite(t *testing.T) {
	store, instance, dataDir := newInstalledAuthOnlyFixture(t)
	seedSteamCredentialEnv(t, dataDir)
	fake := &fakeDocker{composePsStarted: make(chan struct{}), composePsRelease: make(chan struct{})}
	driver := New(fake, slog.Default(), jobs.NewManager(store, slog.Default()), store)
	updated := make(chan error, 1)
	go func() {
		updated <- driver.UpdateSteamCredentials(context.Background(), registry.Instance{ID: instance.ID, DataDir: dataDir}, "new-user", "new-pass")
	}()
	select {
	case <-fake.composePsStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("credential mutation did not reach the stopped-state check")
	}

	lifecycleEntered := make(chan struct{})
	go func() {
		_ = driver.WithMutationOwnership(context.Background(), registry.Instance{ID: instance.ID, DataDir: dataDir}, func() error {
			close(lifecycleEntered)
			return nil
		})
	}()
	select {
	case <-lifecycleEntered:
		t.Fatal("lifecycle mutation entered before credential env write released its lock")
	case <-time.After(100 * time.Millisecond):
	}
	close(fake.composePsRelease)
	if err := <-updated; err != nil {
		t.Fatalf("credential mutation: %v", err)
	}
	select {
	case <-lifecycleEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("lifecycle mutation did not resume after credential write")
	}
}
