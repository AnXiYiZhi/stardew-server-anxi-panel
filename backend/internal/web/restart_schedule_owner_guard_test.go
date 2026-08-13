package web

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	appconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

type restartOwnerGuardDriver struct {
	registry.GameDriver
	ownerErr        error
	ownershipChecks int
	offlineChecks   int
	playerReads     int
	sayCalls        int
	stopCalls       int
	listSaveCalls   int
	startCalls      int
}

func (d *restartOwnerGuardDriver) EnsureMutationOwnershipAvailable(context.Context, registry.Instance) error {
	d.ownershipChecks++
	return d.ownerErr
}

func (d *restartOwnerGuardDriver) EnsureOfflineMutationAllowed(context.Context, registry.Instance) error {
	d.offlineChecks++
	return d.ownerErr
}

func (d *restartOwnerGuardDriver) ListPlayers(context.Context, registry.Instance) (*sj.PlayersResult, error) {
	d.playerReads++
	return &sj.PlayersResult{}, nil
}

func (d *restartOwnerGuardDriver) SendSay(context.Context, registry.Instance, string) (*sj.CommandRunResult, error) {
	d.sayCalls++
	return &sj.CommandRunResult{}, nil
}

func (d *restartOwnerGuardDriver) Stop(context.Context, registry.Instance) error {
	d.stopCalls++
	return nil
}

func (d *restartOwnerGuardDriver) ListSaves(context.Context, registry.Instance) ([]registry.SaveInfo, error) {
	d.listSaveCalls++
	return []registry.SaveInfo{{Name: "owned_save"}}, nil
}

func (d *restartOwnerGuardDriver) Start(context.Context, registry.StartRequest) (*registry.Job, error) {
	d.startCalls++
	return &registry.Job{ID: "unexpected"}, nil
}

func TestRestartScheduleSkipsAllAutomaticActionsUnderNewGameOwner(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	store, err := storage.Open(ctx, appconfig.Config{DataDir: root, DBPath: filepath.Join(root, "panel.db"), Secret: "test-secret"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	instance, err := store.EnsureDefaultInstance(ctx, storage.EnsureDefaultInstanceParams{ID: "stardew", DriverID: sj.DriverID, Name: "test", DataDir: filepath.Join(root, "instance")})
	if err != nil {
		t.Fatal(err)
	}
	instance, err = store.UpdateInstanceState(ctx, storage.UpdateInstanceStateParams{ID: instance.ID, State: storage.InstanceStateRunning, DriverPhase: "running", DriverPayload: "{}"})
	if err != nil {
		t.Fatal(err)
	}
	if err := sj.SetActiveSave(instance.DataDir, "owned_save"); err != nil {
		t.Fatal(err)
	}
	schedule, err := store.UpsertRestartSchedule(ctx, storage.UpsertRestartScheduleParams{
		InstanceID: instance.ID, Enabled: true, ShutdownTime: "03:00", StartupTime: "04:00", Timezone: "Asia/Shanghai",
		WarningMinutes: []int{}, BackupBeforeShutdown: true, SkipIfPlayersOnline: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	driver := &restartOwnerGuardDriver{ownerErr: &sj.NewGameOwnerError{Code: "new_game_in_progress", Message: "新建存档事务尚未结束"}}
	scheduler := NewRestartScheduler(RestartSchedulerDeps{Store: store})
	scheduledAt := time.Date(2026, 8, 13, 3, 0, 0, 0, time.FixedZone("CST", 8*60*60))

	if err := scheduler.runShutdown(ctx, driver, instance, schedule, scheduledAt); err != nil {
		t.Fatal(err)
	}
	storedSchedule, err := store.GetRestartSchedule(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !storedSchedule.LastStatus.Valid || storedSchedule.LastStatus.String != "skipped_new_game_recovery_required" {
		t.Fatalf("shutdown status = %#v", storedSchedule.LastStatus)
	}
	if driver.ownershipChecks != 1 || driver.playerReads != 0 || driver.sayCalls != 0 || driver.stopCalls != 0 {
		t.Fatalf("shutdown crossed owner boundary: %#v", driver)
	}

	instance.State = storage.InstanceStateStopped
	if err := scheduler.runStartup(ctx, driver, instance, schedule, scheduledAt.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	storedSchedule, err = store.GetRestartSchedule(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !storedSchedule.LastStatus.Valid || storedSchedule.LastStatus.String != "skipped_new_game_recovery_required" {
		t.Fatalf("startup status = %#v", storedSchedule.LastStatus)
	}
	if driver.offlineChecks != 1 || driver.listSaveCalls != 0 || driver.startCalls != 0 {
		t.Fatalf("startup crossed owner boundary: %#v", driver)
	}
}
