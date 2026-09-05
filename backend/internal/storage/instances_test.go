package storage

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sort"
	"sync"
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

func TestCreateInstanceReservesUniqueIDAndDataDirectory(t *testing.T) {
	store, closeStore := newStorageTestStore(t)
	defer closeStore()
	root := filepath.Join(t.TempDir(), "instances")
	created, err := store.CreateInstance(context.Background(), CreateInstanceParams{
		ID: "river-farm", DriverID: DefaultDriverID, Name: "河湾农场",
		DataDir: filepath.Join(root, "river-farm"), State: InstanceStateAdminCreated,
		StateMessage: "creating", DriverPhase: "instance_provisioning",
	})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}
	if created.ID != "river-farm" || created.DriverPhase != "instance_provisioning" {
		t.Fatalf("unexpected created instance: %#v", created)
	}
	if _, err := store.CreateInstance(context.Background(), CreateInstanceParams{
		ID: "river-farm", DriverID: DefaultDriverID, Name: "duplicate",
		DataDir: filepath.Join(root, "other"), State: InstanceStateAdminCreated,
	}); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate id error = %v, want conflict", err)
	}
	if _, err := store.CreateInstance(context.Background(), CreateInstanceParams{
		ID: "forest-farm", DriverID: DefaultDriverID, Name: "duplicate dir",
		DataDir: created.DataDir, State: InstanceStateAdminCreated,
	}); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate data dir error = %v, want conflict", err)
	}
	if deleted, err := store.DeleteInstanceIfPhase(context.Background(), created.ID, "wrong_phase"); err != nil || deleted {
		t.Fatalf("wrong phase delete = %v, %v", deleted, err)
	}
	if deleted, err := store.DeleteInstanceIfPhase(context.Background(), created.ID, "instance_provisioning"); err != nil || !deleted {
		t.Fatalf("owned delete = %v, %v", deleted, err)
	}
	if _, err := store.GetInstance(context.Background(), created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted instance lookup = %v, want not found", err)
	}
}

func TestAllocateInstanceOrdinalSeedsLegacyIDsAndNeverReusesDeletedValue(t *testing.T) {
	store, closeStore := newStorageTestStore(t)
	defer closeStore()
	root := filepath.Join(t.TempDir(), "instances")
	if _, err := store.EnsureDefaultInstance(context.Background(), EnsureDefaultInstanceParams{
		ID: DefaultInstanceID, DriverID: DefaultDriverID, Name: "主世界",
		DataDir: filepath.Join(root, DefaultInstanceID),
	}); err != nil {
		t.Fatalf("ensure default instance: %v", err)
	}
	for _, id := range []string{"river-farm", "stardew-7"} {
		if _, err := store.CreateInstance(context.Background(), CreateInstanceParams{
			ID: id, DriverID: DefaultDriverID, Name: id,
			DataDir: filepath.Join(root, id), State: InstanceStateAdminCreated,
			DriverPhase: "instance_provisioning",
		}); err != nil {
			t.Fatalf("seed legacy instance %s: %v", id, err)
		}
	}

	ordinal, err := store.AllocateInstanceOrdinal(context.Background(), "stardew", DefaultDriverID, "stardew")
	if err != nil {
		t.Fatalf("allocate first ordinal: %v", err)
	}
	if ordinal != 8 {
		t.Fatalf("first ordinal = %d, want 8", ordinal)
	}
	created, err := store.CreateInstance(context.Background(), CreateInstanceParams{
		ID: "stardew-8", DriverID: DefaultDriverID, Name: "第八个世界",
		DataDir: filepath.Join(root, "stardew-8"), State: InstanceStateAdminCreated,
		DriverPhase: "instance_provisioning",
	})
	if err != nil {
		t.Fatalf("reserve allocated instance: %v", err)
	}
	if deleted, err := store.DeleteInstanceIfPhase(context.Background(), created.ID, "instance_provisioning"); err != nil || !deleted {
		t.Fatalf("delete allocated instance = %v, %v", deleted, err)
	}

	ordinal, err = store.AllocateInstanceOrdinal(context.Background(), "stardew", DefaultDriverID, "stardew")
	if err != nil {
		t.Fatalf("allocate ordinal after deletion: %v", err)
	}
	if ordinal != 9 {
		t.Fatalf("ordinal after deletion = %d, want 9", ordinal)
	}
}

func TestAllocateInstanceOrdinalSerializesConcurrentRequests(t *testing.T) {
	store, closeStore := newStorageTestStore(t)
	defer closeStore()
	if _, err := store.EnsureDefaultInstance(context.Background(), EnsureDefaultInstanceParams{
		ID: DefaultInstanceID, DriverID: DefaultDriverID, Name: "主世界",
		DataDir: filepath.Join(t.TempDir(), "instances", DefaultInstanceID),
	}); err != nil {
		t.Fatalf("ensure default instance: %v", err)
	}

	const requestCount = 8
	start := make(chan struct{})
	results := make(chan int, requestCount)
	errorsCh := make(chan error, requestCount)
	var wait sync.WaitGroup
	for range requestCount {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			ordinal, err := store.AllocateInstanceOrdinal(context.Background(), "stardew", DefaultDriverID, "stardew")
			if err != nil {
				errorsCh <- err
				return
			}
			results <- ordinal
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	close(errorsCh)
	for err := range errorsCh {
		t.Fatalf("concurrent allocation: %v", err)
	}
	ordinals := make([]int, 0, requestCount)
	for ordinal := range results {
		ordinals = append(ordinals, ordinal)
	}
	sort.Ints(ordinals)
	for index, ordinal := range ordinals {
		if want := index + 2; ordinal != want {
			t.Fatalf("ordinal[%d] = %d, want %d; all = %v", index, ordinal, want, ordinals)
		}
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
