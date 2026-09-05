package stardew_junimo

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func TestReconcileRestoredGameFiles(t *testing.T) {
	for _, phase := range []string{"install_verification_failed", "steamcmd_failed"} {
		t.Run(phase, func(t *testing.T) {
			instance := storage.Instance{ID: "stardew", DataDir: t.TempDir(), State: storage.InstanceStateError, DriverPhase: phase, DriverPayload: `{"preserved":true}`}
			store := &fakeStore{instance: instance}
			fake := &fakeDocker{}
			driver := New(fake, slog.Default(), nil, store)
			// A stale successful cache must not substitute for a fresh probe.
			driver.rememberInstallationEvidence(instance.ID, "ok")
			updated, err := driver.ReconcileState(context.Background(), instance)
			if err != nil || updated.State != storage.InstanceStateStopped || updated.DriverPhase != "game_files_restored" || updated.DriverPayload != instance.DriverPayload || fake.verifyRuns != 1 {
				t.Fatalf("recovery: state=%+v probes=%d err=%v", updated, fake.verifyRuns, err)
			}
			_, err = driver.ReconcileState(context.Background(), updated)
			if err != nil || fake.verifyRuns != 1 || len(store.updated) != 1 {
				t.Fatalf("repeated reads must be idempotent: probes=%d writes=%d err=%v", fake.verifyRuns, len(store.updated), err)
			}
		})
	}
}

func TestReconcileRestoredGameFilesFailsClosed(t *testing.T) {
	for _, tc := range []struct {
		name  string
		phase string
		fake  *fakeDocker
	}{
		{"missing files", "install_verification_failed", &fakeDocker{verifyCode: installVerificationMissingExitCode}},
		{"probe error", "steamcmd_failed", &fakeDocker{verifyErr: errors.New("unavailable")}},
		{"image missing", "steamcmd_failed", &fakeDocker{inspectErr: errors.New("unavailable")}},
		{"docker unavailable", "steamcmd_failed", &fakeDocker{psErr: errors.New("unavailable")}},
		{"server running", "steamcmd_failed", &fakeDocker{psResult: paneldocker.ComposePsResult{Services: []paneldocker.ComposeService{{Service: "server", State: "running"}}}}},
		{"unrelated error", "control_runtime_failed", &fakeDocker{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			instance := storage.Instance{ID: "stardew", DataDir: t.TempDir(), State: storage.InstanceStateError, DriverPhase: tc.phase}
			store := &fakeStore{instance: instance}
			driver := New(tc.fake, slog.Default(), nil, store)
			for i := 0; i < 2; i++ {
				updated, err := driver.ReconcileState(context.Background(), instance)
				if err != nil || updated.State != instance.State || len(store.updated) != 0 {
					t.Fatalf("unsafe recovery: %+v %v", updated, err)
				}
			}
			if tc.name == "missing files" && tc.fake.verifyRuns != 1 {
				t.Fatal("missing-file probes must be cached")
			}
		})
	}
}

func TestReconcileRestoredGameFilesKeepsActiveOwner(t *testing.T) {
	store := newLifecycleTestStore(t)
	instance, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{ID: "stardew", DriverID: DriverID, Name: "World", DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	instance, err = store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{ID: instance.ID, State: storage.InstanceStateError, DriverPhase: "steamcmd_failed"})
	if err != nil {
		t.Fatal(err)
	}
	manager := jobs.NewManager(store, slog.Default())
	release := make(chan struct{})
	job, err := manager.Start(context.Background(), jobs.Spec{Type: "stardew_install", TargetType: "instance", TargetID: instance.ID, Run: func(ctx context.Context, _ *jobs.Context) error {
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { close(release); waitForDriverTestJobStatus(t, store, job.ID, storage.JobStatusSucceeded) })
	fake := &fakeDocker{}
	driver := New(fake, slog.Default(), manager, store)
	updated, err := driver.ReconcileState(context.Background(), instance)
	if err != nil || updated.State != storage.InstanceStateError || fake.verifyRuns != 0 {
		t.Fatalf("active owner overwritten: %+v %v", updated, err)
	}
}

func TestReconcileRestoredGameFilesKeepsMutationAndTransactionOwners(t *testing.T) {
	for _, owner := range []string{"mutation lock", "unreadable transaction"} {
		t.Run(owner, func(t *testing.T) {
			instance := storage.Instance{ID: "stardew", DataDir: t.TempDir(), State: storage.InstanceStateError, DriverPhase: "steamcmd_failed"}
			store := &fakeStore{instance: instance}
			fake := &fakeDocker{}
			driver := New(fake, slog.Default(), nil, store)
			if owner == "mutation lock" {
				driver.runtimeUpdateMu.Lock()
				defer driver.runtimeUpdateMu.Unlock()
			} else if err := os.MkdirAll(newGameOwnerDir(instance.DataDir), 0o700); err != nil {
				t.Fatal(err)
			}
			updated, err := driver.ReconcileState(context.Background(), instance)
			if err != nil || updated.State != instance.State || fake.verifyRuns != 0 || len(store.updated) != 0 {
				t.Fatalf("owner was not preserved: %+v %v", updated, err)
			}
		})
	}
}
