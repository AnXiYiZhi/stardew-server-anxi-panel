package storage

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func TestRestoreInstanceStateSnapshotPreservesNullableMessage(t *testing.T) {
	store, closeStore := newStorageTestStore(t)
	defer closeStore()
	instance, err := store.EnsureDefaultInstance(context.Background(), EnsureDefaultInstanceParams{
		ID: DefaultInstanceID, DriverID: DefaultDriverID, Name: "Stardew Valley",
		DataDir: filepath.Join(t.TempDir(), "instances", "stardew"),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		message sql.NullString
		phase   string
		payload string
	}{
		{name: "null", message: sql.NullString{String: "ignored", Valid: false}, phase: "", payload: ""},
		{name: "empty", message: sql.NullString{String: "", Valid: true}, phase: "exact-empty-message", payload: `{"exact":true}`},
		{name: "ordinary and raw bytes", message: sql.NullString{String: "ordinary", Valid: true}, phase: string([]byte{'p', 0, 'h'}), payload: string([]byte{'{', 0xff, 0, '}'})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			instance.State = InstanceStateGameInstalled
			instance.StateMessage = tc.message
			instance.DriverPhase = tc.phase
			instance.DriverPayload = tc.payload
			restored, err := store.RestoreInstanceStateSnapshot(context.Background(), instance)
			if err != nil {
				t.Fatal(err)
			}
			if restored.StateMessage.Valid != tc.message.Valid ||
				(tc.message.Valid && restored.StateMessage.String != tc.message.String) ||
				restored.DriverPhase != tc.phase || restored.DriverPayload != tc.payload {
				t.Fatalf("snapshot not preserved exactly: got=%+v", restored)
			}
		})
	}
}

func TestEnsureDefaultInstanceCreatesAndPreservesExisting(t *testing.T) {
	store, closeStore := newStorageTestStore(t)
	defer closeStore()

	dataDir := filepath.Join(t.TempDir(), "instances", "stardew")
	instance, err := store.EnsureDefaultInstance(context.Background(), EnsureDefaultInstanceParams{
		ID:       DefaultInstanceID,
		DriverID: DefaultDriverID,
		Name:     "Stardew Valley",
		DataDir:  dataDir,
	})
	if err != nil {
		t.Fatalf("ensure default instance: %v", err)
	}
	if instance.ID != DefaultInstanceID || instance.DriverID != DefaultDriverID || instance.DataDir != dataDir {
		t.Fatalf("unexpected instance: %#v", instance)
	}
	if instance.State != InstanceStateUninitialized {
		t.Fatalf("expected uninitialized, got %s", instance.State)
	}

	updated, err := store.UpdateInstanceState(context.Background(), UpdateInstanceStateParams{
		ID:            DefaultInstanceID,
		State:         InstanceStateAdminCreated,
		StateMessage:  "admin ready",
		DriverPhase:   DefaultDriverPhase,
		DriverPayload: "{}",
	})
	if err != nil {
		t.Fatalf("update instance state: %v", err)
	}

	preserved, err := store.EnsureDefaultInstance(context.Background(), EnsureDefaultInstanceParams{
		ID:       DefaultInstanceID,
		DriverID: DefaultDriverID,
		Name:     "Changed",
		DataDir:  filepath.Join(t.TempDir(), "other"),
	})
	if err != nil {
		t.Fatalf("ensure existing instance: %v", err)
	}
	if preserved.State != updated.State || preserved.StateMessage.String != "admin ready" || preserved.Name != "Stardew Valley" {
		t.Fatalf("existing instance was not preserved: %#v", preserved)
	}
}

func TestEnsureDefaultInstanceMigratesLegacyState(t *testing.T) {
	store, closeStore := newStorageTestStore(t)
	defer closeStore()

	_, err := store.SetInstanceState(context.Background(), SetInstanceStateParams{
		InstanceID:   DefaultInstanceID,
		DriverID:     DefaultDriverID,
		State:        InstanceStateError,
		StateMessage: "legacy error",
		SkipValidate: true,
	})
	if err != nil {
		t.Fatalf("set legacy state: %v", err)
	}
	instance, err := store.EnsureDefaultInstance(context.Background(), EnsureDefaultInstanceParams{
		ID:       DefaultInstanceID,
		DriverID: DefaultDriverID,
		Name:     "Stardew Valley",
		DataDir:  filepath.Join(t.TempDir(), "instances", "stardew"),
	})
	if err != nil {
		t.Fatalf("ensure migrated instance: %v", err)
	}
	if instance.State != InstanceStateError || instance.StateMessage.String != "legacy error" {
		t.Fatalf("legacy state not migrated: %#v", instance)
	}
}

func TestListAndGetInstances(t *testing.T) {
	store, closeStore := newStorageTestStore(t)
	defer closeStore()

	_, err := store.EnsureDefaultInstance(context.Background(), EnsureDefaultInstanceParams{
		ID:       DefaultInstanceID,
		DriverID: DefaultDriverID,
		Name:     "Stardew Valley",
		DataDir:  filepath.Join(t.TempDir(), "instances", "stardew"),
	})
	if err != nil {
		t.Fatalf("ensure default instance: %v", err)
	}
	instances, err := store.ListInstances(context.Background())
	if err != nil {
		t.Fatalf("list instances: %v", err)
	}
	if len(instances) != 1 {
		t.Fatalf("expected one instance, got %d", len(instances))
	}
	loaded, err := store.GetInstance(context.Background(), DefaultInstanceID)
	if err != nil {
		t.Fatalf("get instance: %v", err)
	}
	if loaded.ID != DefaultInstanceID {
		t.Fatalf("unexpected loaded instance: %#v", loaded)
	}
	if _, err := store.GetInstance(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestUpdateInstanceStateForActiveJobRejectsTerminalOwner(t *testing.T) {
	store, closeStore := newStorageTestStore(t)
	defer closeStore()

	_, err := store.EnsureDefaultInstance(context.Background(), EnsureDefaultInstanceParams{
		ID: DefaultInstanceID, DriverID: DefaultDriverID, Name: "Stardew Valley",
		DataDir: filepath.Join(t.TempDir(), "instances", "stardew"),
	})
	if err != nil {
		t.Fatalf("ensure instance: %v", err)
	}
	job, err := store.CreateExclusiveJob(context.Background(), CreateJobParams{
		Type: "stardew_install", TargetType: "instance", TargetID: DefaultInstanceID,
	})
	if err != nil {
		t.Fatalf("create install job: %v", err)
	}
	params := UpdateInstanceStateForActiveJobParams{
		JobID: job.ID,
		UpdateInstanceStateParams: UpdateInstanceStateParams{
			ID: DefaultInstanceID, State: InstanceStateSteamAuthRunning,
			StateMessage: "owned update", DriverPhase: "steam_auth_running",
		},
	}
	updated, err := store.UpdateInstanceStateForActiveJob(context.Background(), params)
	if err != nil {
		t.Fatalf("update from active owner: %v", err)
	}
	if updated.DriverPhase != "steam_auth_running" {
		t.Fatalf("active owner phase = %s", updated.DriverPhase)
	}

	if _, err := store.FinishJob(context.Background(), job.ID); err != nil {
		t.Fatalf("finish install job: %v", err)
	}
	params.State = InstanceStateGameInstalled
	params.DriverPhase = "game_installed"
	if _, err := store.UpdateInstanceStateForActiveJob(context.Background(), params); !errors.Is(err, ErrJobNotActive) {
		t.Fatalf("terminal owner update error = %v, want ErrJobNotActive", err)
	}
	preserved, err := store.GetInstance(context.Background(), DefaultInstanceID)
	if err != nil {
		t.Fatal(err)
	}
	if preserved.DriverPhase != "steam_auth_running" {
		t.Fatalf("terminal owner overwrote phase: %s", preserved.DriverPhase)
	}
}
