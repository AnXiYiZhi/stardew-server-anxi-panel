package web

import (
	"context"
	"errors"
	"net/http"

	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

var errStardewMutationResponseWritten = errors.New("stardew mutation response already written")

// writeStardewMutationGuardConflict maps the driver's persistent transaction
// guard to one stable HTTP conflict without teaching Web how owner files work.
// Stardew remains the sole authority for whether an owner is valid/unfinished.
func writeStardewMutationGuardConflict(w http.ResponseWriter, err error) bool {
	if errors.Is(err, sj.ErrSteamInviteCleanupPending) {
		writeError(w, http.StatusConflict, "steam_invite_cleanup_pending", "Steam 授权 holder 尚未安全收敛，请检查 Docker 状态后重试。")
		return true
	}
	var ownerErr *sj.NewGameOwnerError
	if !errors.As(err, &ownerErr) {
		return false
	}
	writeError(w, http.StatusConflict, ownerErr.Code, ownerErr.Message)
	return true
}

// withStardewOfflineMutation performs the final stopped/owner check and the
// filesystem mutation under the driver's runtimeUpdateMu. Handler-level
// preflight remains useful for early HTTP feedback, but it cannot be the
// linearization boundary because Start may reserve an owner immediately after
// that check returns.
func (s *server) withStardewOfflineMutation(ctx context.Context, instance storage.Instance, mutate func() error) error {
	driver, err := s.registry.Get(instance.DriverID)
	if err != nil {
		// Legacy/test registries may omit the optional driver capability. Preserve
		// the prior handler behavior there; production Stardew registration always
		// supplies InstanceMutationExecutor.
		return mutate()
	}
	if executor, ok := driver.(sj.InstanceMutationExecutor); ok {
		return executor.WithOfflineMutation(ctx, makeRegistryInstance(instance), mutate)
	}
	return mutate()
}

// withStardewMutationOwnership serializes writes which are safe while the
// normal game is online (backup metadata, inactive-save maintenance, panel-only
// classifications) with new-game owner publication and rollback.
func (s *server) withStardewMutationOwnership(ctx context.Context, instance storage.Instance, mutate func() error) error {
	driver, err := s.registry.Get(instance.DriverID)
	if err != nil {
		return mutate()
	}
	if executor, ok := driver.(sj.InstanceMutationExecutor); ok {
		return executor.WithMutationOwnership(ctx, makeRegistryInstance(instance), mutate)
	}
	return mutate()
}
