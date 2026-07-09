package web

import (
	"context"
	"net/http"
	"strings"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
)

type playerLister interface {
	ListPlayers(ctx context.Context, instance registry.Instance) (*sj.PlayersResult, error)
}

type playerKicker interface {
	KickPlayer(ctx context.Context, instance registry.Instance, uniqueMultiplayerID, name string) (*sj.CommandRunResult, error)
}

type kickPlayerRequest struct {
	UniqueMultiplayerID string `json:"uniqueMultiplayerId"`
	Name                string `json:"name"`
}

// handlePlayerKick handles POST /api/instances/:id/players/kick.
func (s *server) handlePlayerKick(w http.ResponseWriter, r *http.Request, instanceID string) {
	actor, ok := s.requireAdmin(w, r)
	if !ok {
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
	kicker, supported := driver.(playerKicker)
	if !supported {
		writeError(w, http.StatusNotImplemented, "not_supported", "该 driver 不支持踢出玩家")
		return
	}

	var body kickPlayerRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	uniqueMultiplayerID := strings.TrimSpace(body.UniqueMultiplayerID)
	if uniqueMultiplayerID == "" {
		writeError(w, http.StatusBadRequest, "invalid_player", "缺少玩家联机 ID")
		return
	}

	result, err := kicker.KickPlayer(r.Context(), makeRegistryInstance(instance), uniqueMultiplayerID, body.Name)
	if err != nil {
		if ce, ok := err.(*sj.CommandError); ok {
			status := http.StatusBadRequest
			switch ce.Code {
			case "server_not_running":
				status = http.StatusConflict
			case "not_supported":
				status = http.StatusNotImplemented
			}
			writeError(w, status, ce.Code, ce.Message)
			return
		}
		writeError(w, http.StatusInternalServerError, "kick_failed", sanitizeErrorMsg(err, "踢出玩家失败"))
		return
	}
	s.auditLog(r, &actor, "player_kick", "instance", instanceID, auditMetadata("uniqueMultiplayerId", uniqueMultiplayerID, "name", body.Name))
	writeJSON(w, http.StatusOK, result)
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
