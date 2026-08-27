package stardew_junimo

import (
	"context"
	"errors"
	"fmt"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
)

// RecoverInterruptedSteamInviteAuthorization removes only the optional
// steam-auth one-shot/session resources left behind by a failed or interrupted
// authorization. Disabled instances are intentionally not probed, and a
// running server is never disturbed.
func (d *Driver) RecoverInterruptedSteamInviteAuthorization(ctx context.Context, instance registry.Instance) error {
	if !sjconfig.SteamInviteEnabled(instance.DataDir) {
		return nil
	}
	authState := sjconfig.SteamInviteAuthState(instance.DataDir)
	if authState == sjconfig.SteamInviteAuthStateCleanupPending {
		return d.convergeSteamInviteCleanupPending(ctx, instance)
	}
	if authState != sjconfig.SteamInviteAuthStateFailed && authState != sjconfig.SteamInviteAuthStateAuthorizing {
		return nil
	}
	if d.docker == nil {
		return errors.New("docker service is not configured")
	}
	ps, err := d.docker.ComposePs(ctx, instance.DataDir)
	if err != nil {
		return fmt.Errorf("inspect server before Steam invite recovery: %w", err)
	}
	if serverServiceUp(ps.Services) {
		return nil
	}

	result, sessionVolume, err := d.removeSteamInviteAuthSessionHolders(ctx, instance.DataDir)
	if err != nil {
		return fmt.Errorf("remove interrupted Steam invite container: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("remove interrupted Steam invite container exited with code %d", result.ExitCode)
	}
	if _, err := d.docker.RemoveVolumes(ctx, instance.DataDir, []string{sessionVolume}); err != nil {
		return fmt.Errorf("remove interrupted Steam invite session: %w", err)
	}
	return nil
}
