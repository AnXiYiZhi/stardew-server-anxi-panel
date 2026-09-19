package web

import (
	"net/http"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
)

// handleInstanceDirectConnect reads the instance's driver-owned connection
// settings without resolving an external address or requiring a running game.
func (s *server) handleInstanceDirectConnect(w http.ResponseWriter, r *http.Request, instanceID string) {
	if _, ok := s.requireAuth(w, r); !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}
	driver, err := s.registry.Get(instance.DriverID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "driver_not_found", "无法读取游戏驱动")
		return
	}
	provider, ok := driver.(registry.DirectConnectConfigProvider)
	if !ok {
		writeError(w, http.StatusBadRequest, "direct_connect_unsupported", "当前游戏不支持直连配置")
		return
	}
	config, err := provider.DirectConnectConfig(r.Context(), makeRegistryInstance(instance))
	if err != nil {
		s.logger.Warn("direct-connect config read failed", "instance", instanceID, "driver", instance.DriverID, "error", err)
		writeError(w, http.StatusInternalServerError, "direct_connect_config_failed", "无法读取当前世界的游戏端口，请检查世界配置")
		return
	}
	writeJSON(w, http.StatusOK, config)
}
