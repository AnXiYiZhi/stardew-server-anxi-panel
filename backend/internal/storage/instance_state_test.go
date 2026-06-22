package storage

import (
	"context"
	"errors"
	"testing"
)

func TestInstanceStateDefaultsAndTransitions(t *testing.T) {
	store, closeStore := newStorageTestStore(t)
	defer closeStore()

	state, err := store.EnsureDefaultInstanceState(context.Background())
	if err != nil {
		t.Fatalf("ensure default state: %v", err)
	}
	if state.State != InstanceStateUninitialized {
		t.Fatalf("expected uninitialized, got %s", state.State)
	}

	updated, err := store.SetInstanceState(context.Background(), SetInstanceStateParams{
		State:        InstanceStateAdminCreated,
		StateMessage: "admin ready",
	})
	if err != nil {
		t.Fatalf("set admin_created: %v", err)
	}
	if updated.State != InstanceStateAdminCreated || updated.StateMessage.String != "admin ready" {
		t.Fatalf("unexpected updated state: %#v", updated)
	}

	_, err = store.SetInstanceState(context.Background(), SetInstanceStateParams{
		State: InstanceStateRunning,
	})
	if !errors.Is(err, ErrInvalidStateTransition) {
		t.Fatalf("expected invalid transition, got %v", err)
	}

	errored, err := store.SetInstanceState(context.Background(), SetInstanceStateParams{
		State: InstanceStateError,
	})
	if err != nil {
		t.Fatalf("set error state: %v", err)
	}
	if errored.State != InstanceStateError {
		t.Fatalf("expected error, got %s", errored.State)
	}
}
