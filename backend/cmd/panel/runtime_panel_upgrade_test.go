package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/updater"
)

type runtimeBootstrapStoreFake struct{ reads int }

func (s *runtimeBootstrapStoreFake) ListInstances(context.Context) ([]storage.Instance, error) {
	s.reads++
	return []storage.Instance{{ID: "existing-world", DriverID: "stardew_junimo", State: "running"}}, nil
}

func TestAutomaticRuntimeBootstrapWaitsForPanelHealthOutcome(t *testing.T) {
	for _, scenario := range []struct {
		name, current, terminal string
		readErr                 error
		cancel                  bool
		wantStart               bool
	}{
		{name: "accepted new Panel", current: "0.7.3", terminal: updater.PhaseSucceeded, wantStart: true},
		{name: "old Panel restored", current: "0.7.2", terminal: updater.PhaseFailedRolledBack, wantStart: true},
		{name: "rejected candidate still running", current: "0.7.3", terminal: updater.PhaseFailedRolledBack},
		{name: "rollback failed", current: "0.7.2", terminal: updater.PhaseRollbackFailed},
		{name: "unreadable receipt", current: "0.7.3", readErr: errors.New("invalid persisted JSON")},
		{name: "unknown outcome", current: "0.7.3", terminal: "unknown"},
		{name: "shutdown during health wait", current: "0.7.3", cancel: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			driver := &bootstrapDriverFake{}
			store := &runtimeBootstrapStoreFake{}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			reads := 0
			read := func() (updater.ApplyStatus, error) {
				reads++
				if len(driver.prepared) != 0 || len(driver.started) != 0 || store.reads != 0 {
					t.Fatal("game bootstrap ran before the helper terminal outcome")
				}
				status := updater.ApplyStatus{FromVersion: "0.7.2", ToVersion: "0.7.3", Phase: updater.PhaseWaitingHealth}
				if reads == 1 {
					return status, nil
				}
				if scenario.cancel {
					cancel()
					return status, nil
				}
				status.Phase = scenario.terminal
				return status, scenario.readErr
			}
			startRequiredRuntimeUpdatesAfterPanelUpgrade(ctx, slog.New(slog.NewTextHandler(io.Discard, nil)), driver, store, read, scenario.current, time.Millisecond)
			if started := len(driver.started) == 1; started != scenario.wantStart {
				t.Fatalf("runtime started=%v want=%v", started, scenario.wantStart)
			}
			if scenario.wantStart && (driver.prepared[0] != "existing-world" || store.reads != 1) {
				t.Fatal("accepted bootstrap must reload and prepare the current instance")
			}
		})
	}
}

func TestAutomaticRuntimeBootstrapWithoutPanelUpgrade(t *testing.T) {
	driver := &bootstrapDriverFake{}
	store := &runtimeBootstrapStoreFake{}
	read := func() (updater.ApplyStatus, error) { return updater.ApplyStatus{}, updater.ErrNoApplyStatus }
	startRequiredRuntimeUpdatesAfterPanelUpgrade(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)), driver, store, read, "0.7.3", time.Millisecond)
	if len(driver.started) != 1 || store.reads != 1 {
		t.Fatal("normal installation/restart did not start required runtime bootstrap")
	}
}
