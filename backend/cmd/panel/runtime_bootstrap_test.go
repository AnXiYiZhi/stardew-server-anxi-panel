package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"testing"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

type bootstrapDriverFake struct {
	prepareErr map[string]error
	active     map[string]bool
	prepared   []string
	started    []string
}

func (f *bootstrapDriverFake) ID() string { return "stardew_junimo" }

func (f *bootstrapDriverFake) Prepare(_ context.Context, instance registry.Instance) error {
	f.prepared = append(f.prepared, instance.ID)
	return f.prepareErr[instance.ID]
}

func (f *bootstrapDriverFake) RuntimeUpdateApplyInProgress(instance registry.Instance) bool {
	return f.active[instance.ID]
}

func (f *bootstrapDriverFake) SMAPIUpdateApplyInProgress(registry.Instance) bool { return false }

func (f *bootstrapDriverFake) StartRequiredRuntimeUpdate(_ context.Context, instance registry.Instance) {
	f.started = append(f.started, instance.ID)
}

func TestStartPreparedRequiredRuntimeUpdatesPreparesEveryInstanceAndSkipsFailures(t *testing.T) {
	driver := &bootstrapDriverFake{
		prepareErr: map[string]error{"broken": errors.New("unsafe session holder")},
		active:     map[string]bool{"recovering": true},
	}
	instances := []storage.Instance{
		{ID: "default", DriverID: driver.ID()},
		{ID: "secondary", DriverID: driver.ID()},
		{ID: "broken", DriverID: driver.ID()},
		{ID: "recovering", DriverID: driver.ID()},
		{ID: "other", DriverID: "other_driver"},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	startPreparedRequiredRuntimeUpdates(context.Background(), logger, driver, instances)

	if want := []string{"default", "secondary", "broken"}; !reflect.DeepEqual(driver.prepared, want) {
		t.Fatalf("prepared=%v want=%v", driver.prepared, want)
	}
	if want := []string{"default", "secondary"}; !reflect.DeepEqual(driver.started, want) {
		t.Fatalf("started=%v want=%v", driver.started, want)
	}
}
