package web

import (
	"context"
	"net/http"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
)

type playerLister interface {
	ListPlayers(ctx context.Context, instance registry.Instance) (*sj.PlayersResult, error)
}

// handlePlayersList handles GET /api/instances/:id/players.
func (s *server) handlePlayersList(w http.ResponseWriter, r *http.Request, instanceID string) {
	if _, ok := s.requireAuth(w, r); !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}
	instance, ok = s.reconcileInstanceState(w, r, instance)
	if !ok {
		return
	}
	driver, ok := s.loadDriver(w, instance.DriverID)
	if !ok {
		return
	}
	lister, supported := driver.(playerLister)
	if !supported {
		writeError(w, http.StatusNotImplemented, "not_supported", "该 driver 不支持玩家状态读取")
		return
	}
	result, err := lister.ListPlayers(r.Context(), makeRegistryInstance(instance))
	if err != nil {
		if ce, ok := err.(*sj.CommandError); ok {
			status := http.StatusBadRequest
			if ce.Code == "server_not_running" {
				status = http.StatusConflict
			}
			writeError(w, status, ce.Code, ce.Message)
			return
		}
		writeError(w, http.StatusInternalServerError, "players_failed", sanitizeErrorMsg(err, "读取在线玩家失败"))
		return
	}
	writeJSON(w, http.StatusOK, result)
}
