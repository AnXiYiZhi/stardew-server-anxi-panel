package main

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/updater"
)

type runtimeBootstrapStore interface {
	ListInstances(context.Context) ([]storage.Instance, error)
}

// The new process must serve health probes before it changes game components.
// Otherwise an unhealthy Panel can restart a live world before its helper rolls
// the Panel back. A pending or unreadable helper result never grants permission
// to start automatic runtime work.
func startRequiredRuntimeUpdatesAfterPanelUpgrade(ctx context.Context, logger *slog.Logger, driver requiredRuntimeBootstrapDriver, store runtimeBootstrapStore, readStatus func() (updater.ApplyStatus, error), currentVersion string, interval time.Duration, beforeRuntime ...func() error) {
	if logger == nil {
		logger = slog.Default()
	}
	if interval <= 0 {
		interval = time.Second
	}
	waited := false
	for {
		if ctx.Err() != nil {
			return
		}
		status, err := readStatus()
		if err != nil && !errors.Is(err, updater.ErrNoApplyStatus) {
			logger.Error("skip automatic runtime bootstrap because Panel upgrade status is unreadable", "error", err)
			return
		}
		if err == nil && updater.IsActiveApplyPhase(status.Phase) {
			if !waited {
				logger.Info("waiting for Panel upgrade health acceptance before automatic runtime bootstrap")
				waited = true
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(interval):
				continue
			}
		}
		if err == nil {
			current, currentErr := updater.NormalizeTargetVersion(currentVersion)
			target, targetErr := updater.NormalizeTargetVersion(status.ToVersion)
			if status.Phase == updater.PhaseRollbackFailed || (status.Phase != updater.PhaseSucceeded && status.Phase != updater.PhaseFailedRolledBack) ||
				(currentErr == nil && targetErr == nil && current == target && status.Phase != updater.PhaseSucceeded) {
				logger.Warn("skip automatic runtime bootstrap because Panel upgrade did not accept this process", "phase", status.Phase)
				return
			}
		}
		for _, prepare := range beforeRuntime {
			if err := prepare(); err != nil {
				logger.Error("upgrade preparation required before runtime bootstrap", "error", err)
				return
			}
		}
		instances, err := store.ListInstances(ctx)
		if err != nil {
			logger.Error("skip automatic runtime bootstrap because instances cannot be reloaded", "error", err)
			return
		}
		startPreparedRequiredRuntimeUpdates(ctx, logger, driver, instances)
		return
	}
}
