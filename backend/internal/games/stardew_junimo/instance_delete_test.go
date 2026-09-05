package stardew_junimo

import (
	"context"
	"errors"
	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
	"os"
	"path/filepath"
	"testing"
)

type deletionFakeDocker struct {
	*fakeDocker
	fail           bool
	plans, applies int
}

func (f *deletionFakeDocker) PlanInstanceDeletion(_ context.Context, _, project, hostDir, volume string) (paneldocker.DeletionPlan, error) {
	f.plans++
	return paneldocker.DeletionPlan{Project: project, HostDir: hostDir, Volumes: map[string]string{volume: "identity"}}, nil
}
func (f *deletionFakeDocker) ApplyInstanceDeletion(context.Context, string, paneldocker.DeletionPlan) error {
	f.applies++
	if f.fail {
		return errors.New("injected partial removal")
	}
	return nil
}
func TestDeleteInstanceRecoveryAndOwners(t *testing.T) {
	for _, scenario := range []string{"retry", "default", "running", "task", "owner", "runtime", "shared", "path", "symlink"} {
		t.Run(scenario, func(t *testing.T) {
			s, _, defaultDir := newInstalledAuthOnlyFixture(t)
			root := filepath.Dir(filepath.Dir(defaultDir))
			ctx := context.Background()
			target := filepath.Join(root, "instances", "delete-target")
			if err := os.MkdirAll(filepath.Join(target, ".local-container", "backups", "saves"), 0700); err != nil {
				t.Fatal(err)
			}
			backup := filepath.Join(target, ".local-container", "backups", "saves", "synthetic.zip")
			if err := os.WriteFile(backup, []byte("synthetic backup"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := sjconfig.UpdateEnvFile(filepath.Join(target, ".env"), map[string]string{"INSTANCE_HOST_DATA_DIR": target}); err != nil {
				t.Fatal(err)
			}
			state := storage.InstanceStateStopped
			if scenario == "running" {
				state = storage.InstanceStateRunning
			}
			_, err := s.CreateInstance(ctx, storage.CreateInstanceParams{ID: "delete-target", DriverID: DriverID, Name: "stardew", DataDir: target, State: state})
			if err != nil {
				t.Fatal(err)
			}
			instance := registry.Instance{ID: "delete-target", DriverID: DriverID, DataDir: target}
			engine := &deletionFakeDocker{fakeDocker: &fakeDocker{}, fail: scenario == "retry"}
			driver := NewWithOptions(engine, nil, jobs.NewManager(s, nil), s, DriverOptions{ContainerDataDir: root})
			defaultID := "stardew"
			switch scenario {
			case "default":
				defaultID = instance.ID
			case "task":
				_, err = s.CreateJob(ctx, storage.CreateJobParams{Type: "backup", TargetType: "instance", TargetID: instance.ID})
			case "owner":
				err = os.MkdirAll(newGameOwnerDir(target), 0700)
			case "runtime":
				err = writeRuntimeUpdateApplyStatus(target, RuntimeUpdateApplyStatus{Phase: RuntimeUpdateApplyRollbackFailed})
			case "shared":
				err = sjconfig.UpdateEnvFile(filepath.Join(defaultDir, ".env"), map[string]string{"GAME_DATA_VOLUME": "delete-target_game-data"})
			case "path":
				instance.DataDir = defaultDir
			case "symlink":
				if err := os.Symlink(t.TempDir(), filepath.Join(target, "external")); err != nil {
					t.Skip("symlink privilege unavailable")
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			if err = driver.DeleteInstance(ctx, instance, defaultID); err == nil {
				t.Fatal("expected initial refusal/failure")
			}
			if _, err = os.Stat(backup); err != nil {
				t.Fatal("backup lost before completion", err)
			}
			if scenario != "retry" {
				if engine.applies != 0 {
					t.Fatal("unsafe resource deletion")
				}
				return
			}
			record, err := s.GetInstanceDeletion(ctx, instance.ID)
			if err != nil || record.Completed {
				t.Fatal(record, err)
			}
			engine.fail = false
			// A crash after filesystem cleanup may have removed .env and backup
			// already; retry must use the durable plan without recreating either.
			if err := os.Remove(filepath.Join(target, ".env")); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(backup); err != nil {
				t.Fatal(err)
			}
			// Simulate process restart: fresh driver, same persistent database/plan.
			driver = NewWithOptions(engine, nil, jobs.NewManager(s, nil), s, DriverOptions{ContainerDataDir: root})
			if err = driver.Prepare(ctx, instance); err == nil {
				t.Fatal("bootstrap recreated deleting world")
			}
			if err = driver.Stop(ctx, instance); err == nil {
				t.Fatal("stop admitted")
			}
			if err = driver.Restart(ctx, instance); err == nil {
				t.Fatal("restart admitted")
			}
			if _, err = driver.RestoreBackupWithRestart(ctx, instance, "backup.zip", false, 1); err == nil {
				t.Fatal("restore admitted")
			}
			if _, err = driver.PrepareFarmMods(ctx, instance, "standard"); err == nil {
				t.Fatal("Mods mutation admitted")
			}
			if _, err = driver.UpdateServerRuntimeSettings(ctx, instance, ServerRuntimeSettings{}); err == nil {
				t.Fatal("settings mutation admitted")
			}
			if err = driver.WithOfflineMutation(ctx, instance, func() error { t.Fatal("mutation executed"); return nil }); err == nil {
				t.Fatal("mutation admitted")
			}
			if err = driver.DeleteInstance(ctx, instance, defaultID); err != nil {
				t.Fatal(err)
			}
			if engine.plans != 1 || engine.applies != 2 {
				t.Fatal("plan was rebuilt", engine)
			}
			if _, err = os.Stat(target); !os.IsNotExist(err) {
				t.Fatal("world directory/backup retained", err)
			}
			if _, err = s.GetInstance(ctx, instance.ID); !errors.Is(err, storage.ErrNotFound) {
				t.Fatal("instance retained", err)
			}
			if _, err = s.GetInstance(ctx, "stardew"); err != nil {
				t.Fatal("default lost", err)
			}
			if err = driver.DeleteInstance(ctx, instance, defaultID); err != nil {
				t.Fatal("repeat delete", err)
			}
		})
	}
}
