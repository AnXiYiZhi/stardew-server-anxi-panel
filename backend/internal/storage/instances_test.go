package storage

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

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
