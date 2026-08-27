package main

import (
	"context"
	"log/slog"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

type requiredRuntimeBootstrapDriver interface {
	ID() string
	Prepare(context.Context, registry.Instance) error
	RuntimeUpdateApplyInProgress(registry.Instance) bool
	SMAPIUpdateApplyInProgress(registry.Instance) bool
	StartRequiredRuntimeUpdate(context.Context, registry.Instance)
}

// startPreparedRequiredRuntimeUpdates makes legacy instance migration the
// linear prerequisite for required runtime work. A failed Prepare may mean the
// optional Auth intent/session scope is unresolved, so that instance must not
// proceed into a server-only update using a partially migrated .env.
func startPreparedRequiredRuntimeUpdates(ctx context.Context, logger *slog.Logger, driver requiredRuntimeBootstrapDriver, instances []storage.Instance) {
	if logger == nil {
		logger = slog.Default()
	}
	for _, stored := range instances {
		if stored.DriverID != driver.ID() {
			continue
		}
		instance := registry.Instance{
			ID:            stored.ID,
			DriverID:      stored.DriverID,
			Name:          stored.Name,
			DataDir:       stored.DataDir,
			State:         stored.State,
			StateMessage:  stored.StateMessage.String,
			DriverPhase:   stored.DriverPhase,
			DriverPayload: stored.DriverPayload,
			CreatedAt:     stored.CreatedAt,
			UpdatedAt:     stored.UpdatedAt,
		}
		if driver.RuntimeUpdateApplyInProgress(instance) || driver.SMAPIUpdateApplyInProgress(instance) {
			logger.Warn("skip instance prepare and required runtime coordinator while a recovery transaction remains active", "instance", instance.ID)
			continue
		}
		if err := driver.Prepare(ctx, instance); err != nil {
			logger.Error("skip required runtime coordinator because instance prepare failed", "instance", instance.ID, "error", err)
			continue
		}
		driver.StartRequiredRuntimeUpdate(ctx, instance)
	}
}
