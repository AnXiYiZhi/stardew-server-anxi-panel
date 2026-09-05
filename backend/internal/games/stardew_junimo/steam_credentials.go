package stardew_junimo

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

var (
	ErrSteamCredentialsInUse                = errors.New("Steam credentials are in use")
	ErrSteamCredentialsInstallationRequired = errors.New("Steam credentials require a completed base installation")
)

// UpdateSteamCredentials linearizes the final stopped/busy checks and the env
// write with lifecycle and install reservations. The Panel-wide credential
// store is authoritative; the current instance .env is kept as a compatibility
// shadow for the optional instance-level Steam invite service.
func (d *Driver) UpdateSteamCredentials(ctx context.Context, instance registry.Instance, username, password string) error {
	d.runtimeUpdateMu.Lock()
	defer d.runtimeUpdateMu.Unlock()
	if err := d.RejectInstanceDeletion(ctx, instance.ID); err != nil {
		return err
	}

	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return errors.New("Steam username and password are required")
	}
	if d.store == nil {
		return errors.New("driver: state store not configured")
	}
	stored, err := d.store.GetInstance(ctx, instance.ID)
	if err != nil {
		return fmt.Errorf("load instance before Steam credential update: %w", err)
	}
	instance.DataDir = stored.DataDir
	instance.State = stored.State
	if !requiresInstalledFiles(stored.State) {
		return ErrSteamCredentialsInstallationRequired
	}
	if err := rejectUnfinishedNewGameOwner(instance.DataDir); err != nil {
		return err
	}
	if d.jobs == nil {
		return errors.New("driver: job manager not configured")
	}
	if active, operation, found, err := d.findActiveLifecycleJob(ctx, instance.ID); err != nil {
		return err
	} else if found {
		return lifecycleConflict(operation, active.ID)
	}
	active, err := d.jobs.Active(ctx, storage.ListActiveJobsFilter{
		TargetType: "instance",
		Types:      []string{"stardew_install", "stardew_steam_auth"},
	})
	if err != nil {
		return fmt.Errorf("list active Steam credential consumers: %w", err)
	}
	if len(active) > 0 {
		return fmt.Errorf("%w: job %s", ErrSteamCredentialsInUse, active[0].ID)
	}
	if d.docker == nil {
		return &NewGameOwnerError{Code: "server_state_unknown", Message: "无法确认服务器已停止，请检查 Docker 状态后重试"}
	}
	ps, err := d.docker.ComposePsStrict(ctx, instance.DataDir)
	if err != nil {
		return &NewGameOwnerError{Code: "server_state_unknown", Message: "无法确认服务器已停止，请检查 Docker 状态后重试", Cause: err}
	}
	if serverServiceUp(ps.Services) {
		return &NewGameOwnerError{Code: "server_running", Message: "服务器容器仍在运行，请先停止服务器再修改 Steam 凭据"}
	}
	if err := d.saveSharedSteamCredentials(instance, username, password); err != nil {
		return err
	}
	return sjconfig.UpdateEnvFile(filepath.Join(instance.DataDir, ".env"), map[string]string{
		"STEAM_USERNAME": username,
		"STEAM_PASSWORD": password,
	})
}
