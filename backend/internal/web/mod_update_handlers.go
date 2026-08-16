package web

import (
	"context"
	"net/http"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
)

type modUpdateChecker interface {
	CheckModUpdates(context.Context, registry.Instance, bool) (registry.ModUpdateCheckResult, error)
}

func (s *server) handleModUpdatesGet(w http.ResponseWriter, r *http.Request, instanceID string) {
	if _, ok := s.requireAuth(w, r); !ok {
		return
	}
	s.handleModUpdateCheck(w, r, instanceID, false)
}

func (s *server) handleModUpdatesCheck(w http.ResponseWriter, r *http.Request, instanceID string) {
	actor, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	if s.handleModUpdateCheck(w, r, instanceID, true) {
		s.auditLog(r, &actor, "mod_updates_check", "instance", instanceID, "{}")
	}
}

func (s *server) handleModUpdateCheck(w http.ResponseWriter, r *http.Request, instanceID string, force bool) bool {
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return false
	}
	driver, err := s.registry.Get(instance.DriverID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "driver_not_found", "游戏驱动不可用")
		return false
	}
	checker, ok := driver.(modUpdateChecker)
	if !ok {
		writeError(w, http.StatusNotImplemented, "mod_update_check_unsupported", "当前游戏暂不支持 Mod 更新检查")
		return false
	}
	result, err := checker.CheckModUpdates(r.Context(), makeRegistryInstance(instance), force)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "mod_update_check_failed", sanitizeErrorMsg(err, "检查 Mod 更新失败"))
		return false
	}
	writeJSON(w, http.StatusOK, result)
	return true
}
