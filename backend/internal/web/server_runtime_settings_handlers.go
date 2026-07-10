package web

import (
	"net/http"

	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
)

// handleInstanceServerRuntimeSettings handles GET/PUT
// /api/instances/:id/config/server-runtime-settings.
// These fields live in server-settings.json and only take effect the next
// time the Junimo `server` container starts, so writing here does not affect
// an already-running server.
func (s *server) handleInstanceServerRuntimeSettings(w http.ResponseWriter, r *http.Request, instanceID string) {
	if r.Method == http.MethodGet {
		if _, ok := s.requireAdmin(w, r); !ok {
			return
		}
		instance, ok := s.loadInstance(w, r, instanceID)
		if !ok {
			return
		}
		settings, err := sj.ReadServerRuntimeSettings(instance.DataDir)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "settings_read_failed", sanitizeErrorMsg(err, "读取小屋与联机设置失败"))
			return
		}
		writeJSON(w, http.StatusOK, settings)
		return
	}

	if r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	actor, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}

	var body sj.ServerRuntimeSettings
	if !decodeJSON(w, r, &body) {
		return
	}
	if err := sj.UpdateServerRuntimeSettings(instance.DataDir, body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_settings", sanitizeErrorMsg(err, "小屋与联机设置无效"))
		return
	}
	s.auditLog(r, &actor, "instance_server_runtime_settings_update", "instance", instanceID, auditMetadata(
		"cabinStrategy", body.CabinStrategy,
		"existingCabinBehavior", body.ExistingCabinBehavior,
	))
	writeJSON(w, http.StatusOK, body)
}
