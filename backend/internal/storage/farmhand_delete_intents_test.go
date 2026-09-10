package storage

import (
	"context"
	"errors"
	"testing"
)

func TestFarmhandDeleteIntentLifecycle(t *testing.T) {
	store, closeStore := newStorageTestStore(t)
	defer closeStore()
	ctx := context.Background()
	if _, err := store.CreateInstance(ctx, CreateInstanceParams{ID: "world", DriverID: DefaultDriverID, Name: "World", DataDir: "/instances/world", State: InstanceStateRunning}); err != nil {
		t.Fatal(err)
	}
	created, err := store.CreateFarmhandDeleteIntent(ctx, CreateFarmhandDeleteIntentParams{
		InstanceID: "world", OperationID: "op-1", Mode: FarmhandDeleteModeWait,
		Status: FarmhandDeleteIntentWaiting, PlayerID: "42", ExpectedName: "Alice",
		ExpectedSaveID: "Farm_1", ExpiresAt: "2030-01-02T00:00:00Z",
	})
	if err != nil || created.Status != FarmhandDeleteIntentWaiting {
		t.Fatalf("create intent=%+v err=%v", created, err)
	}
	if _, err := store.CreateFarmhandDeleteIntent(ctx, CreateFarmhandDeleteIntentParams{
		InstanceID: "world", OperationID: "op-2", Mode: FarmhandDeleteModeWait,
		Status: FarmhandDeleteIntentWaiting, PlayerID: "43", ExpectedSaveID: "Farm_1",
		ExpiresAt: "2030-01-02T00:00:00Z",
	}); !errors.Is(err, ErrConflict) {
		t.Fatalf("active intent replacement err=%v", err)
	}
	claimed, err := store.ClaimFarmhandDeleteIntent(ctx, "world", "op-1")
	if err != nil || claimed.Status != FarmhandDeleteIntentLaunching {
		t.Fatalf("claim intent=%+v err=%v", claimed, err)
	}
	job, err := store.CreateJob(ctx, CreateJobParams{Type: "stardew_farmhand_delete", TargetType: "instance", TargetID: "world"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.BindFarmhandDeleteIntentJob(ctx, "world", "op-1", job.ID); err != nil {
		t.Fatal(err)
	}
	actionable, err := store.ListActionableFarmhandDeleteIntents(ctx)
	if err != nil || len(actionable) != 1 || actionable[0].Status != FarmhandDeleteIntentActive {
		t.Fatalf("active intent was not actionable for restart recovery: %+v err=%v", actionable, err)
	}
	if err := store.UpdateFarmhandDeleteIntent(ctx, "world", "op-1", FarmhandDeleteIntentRecoveryRequired, "backup.zip", "save failed"); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetFarmhandDeleteIntent(ctx, "world")
	if err != nil || got.Status != FarmhandDeleteIntentRecoveryRequired || got.BackupName != "backup.zip" || got.LastError != "save failed" || got.JobID.String != job.ID {
		t.Fatalf("stored intent=%+v err=%v", got, err)
	}
	if _, err := store.BeginFarmhandDeleteRecovery(ctx, "world", "op-1"); err != nil {
		t.Fatal(err)
	}
	recoveryJob, err := store.CreateJob(ctx, CreateJobParams{Type: "stardew_farmhand_delete", TargetType: "instance", TargetID: "world"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.BindFarmhandDeleteRecoveryJob(ctx, "world", "op-1", recoveryJob.ID); err != nil {
		t.Fatal(err)
	}
	got, err = store.GetFarmhandDeleteIntent(ctx, "world")
	if err != nil || got.Status != FarmhandDeleteIntentActive || got.JobID.String != recoveryJob.ID {
		t.Fatalf("recovery job binding=%+v err=%v", got, err)
	}
}

func TestFarmhandDeleteIntentCancellationAndTerminalReplacement(t *testing.T) {
	store, closeStore := newStorageTestStore(t)
	defer closeStore()
	ctx := context.Background()
	if _, err := store.CreateInstance(ctx, CreateInstanceParams{ID: "world", DriverID: DefaultDriverID, Name: "World", DataDir: "/instances/world", State: InstanceStateRunning}); err != nil {
		t.Fatal(err)
	}
	params := CreateFarmhandDeleteIntentParams{
		InstanceID: "world", OperationID: "op-1", Mode: FarmhandDeleteModeWait,
		Status: FarmhandDeleteIntentWaiting, PlayerID: "42", ExpectedSaveID: "Farm_1",
		ExpiresAt: "2030-01-02T00:00:00Z",
	}
	if _, err := store.CreateFarmhandDeleteIntent(ctx, params); err != nil {
		t.Fatal(err)
	}
	if err := store.CancelWaitingFarmhandDeleteIntent(ctx, "world", "op-1", "target returned"); err != nil {
		t.Fatal(err)
	}
	params.OperationID = "op-2"
	params.PlayerID = "43"
	created, err := store.CreateFarmhandDeleteIntent(ctx, params)
	if err != nil || created.OperationID != "op-2" || created.Status != FarmhandDeleteIntentWaiting {
		t.Fatalf("replacement=%+v err=%v", created, err)
	}
	actionable, err := store.ListActionableFarmhandDeleteIntents(ctx)
	if err != nil || len(actionable) != 1 || actionable[0].OperationID != "op-2" {
		t.Fatalf("actionable=%+v err=%v", actionable, err)
	}
}

func TestFarmhandDeleteCountdownCancelAndActivateAreAtomic(t *testing.T) {
	store, closeStore := newStorageTestStore(t)
	defer closeStore()
	ctx := context.Background()
	if _, err := store.CreateInstance(ctx, CreateInstanceParams{ID: "world", DriverID: DefaultDriverID, Name: "World", DataDir: "/instances/world", State: InstanceStateRunning}); err != nil {
		t.Fatal(err)
	}
	createCountdown := func(operationID string) {
		t.Helper()
		if _, err := store.CreateFarmhandDeleteIntent(ctx, CreateFarmhandDeleteIntentParams{
			InstanceID: "world", OperationID: operationID, Mode: FarmhandDeleteModeMaintenanceNow,
			Status: FarmhandDeleteIntentLaunching, PlayerID: "42", ExpectedSaveID: "Farm_1", ExpiresAt: "2030-01-02T00:00:00Z",
		}); err != nil {
			t.Fatal(err)
		}
		job, err := store.CreateJob(ctx, CreateJobParams{Type: "stardew_farmhand_delete", TargetType: "instance", TargetID: "world"})
		if err != nil {
			t.Fatal(err)
		}
		if err := store.BindFarmhandDeleteIntentJob(ctx, "world", operationID, job.ID); err != nil {
			t.Fatal(err)
		}
		intent, err := store.GetFarmhandDeleteIntent(ctx, "world")
		if err != nil || intent.Status != FarmhandDeleteIntentCountdown {
			t.Fatalf("bound immediate intent = %+v, err=%v", intent, err)
		}
	}

	createCountdown("op-cancel")
	if err := store.CancelFarmhandDeleteCountdown(ctx, "world", "op-cancel", "admin canceled"); err != nil {
		t.Fatal(err)
	}
	if err := store.ActivateFarmhandDeleteIntent(ctx, "world", "op-cancel"); !errors.Is(err, ErrConflict) {
		t.Fatalf("canceled countdown activated: %v", err)
	}

	createCountdown("op-activate")
	if err := store.ActivateFarmhandDeleteIntent(ctx, "world", "op-activate"); err != nil {
		t.Fatal(err)
	}
	if err := store.CancelFarmhandDeleteCountdown(ctx, "world", "op-activate", "too late"); !errors.Is(err, ErrConflict) {
		t.Fatalf("active maintenance was canceled: %v", err)
	}
}
