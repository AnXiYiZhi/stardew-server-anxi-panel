package web

import (
	"context"
	"net/http"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
)

type gameInstallationResponse struct {
	GameID                string           `json:"gameId"`
	DriverID              string           `json:"driverId"`
	InstallationTargetID  string           `json:"installationTargetId"`
	Installed             bool             `json:"installed"`
	RequiredFiles         string           `json:"requiredFiles"`
	CredentialsConfigured bool             `json:"credentialsConfigured"`
	AuthorizationCached   bool             `json:"authorizationCached"`
	Instance              instanceResponse `json:"instance"`
}

func (s *server) handleStardewGameInstallation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if _, ok := s.requireAuth(w, r); !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, s.config.DefaultInstanceID)
	if !ok {
		return
	}
	driver, ok := s.loadDriver(w, instance.DriverID)
	if !ok {
		return
	}
	diagnosticProvider, ok := driver.(interface {
		InstallationDiagnostic(context.Context, registry.Instance) sj.InstallationDiagnostic
	})
	if !ok {
		writeError(w, http.StatusInternalServerError, "installation_status_unsupported", "当前游戏驱动无法检查安装状态。")
		return
	}
	credentialProvider, ok := driver.(interface {
		SteamDownloadStatus(registry.Instance) (bool, bool, error)
	})
	if !ok {
		writeError(w, http.StatusInternalServerError, "steam_download_status_unsupported", "当前游戏驱动无法检查公用 Steam 下载状态。")
		return
	}
	registryInstance := makeRegistryInstance(instance)
	diagnostic := diagnosticProvider.InstallationDiagnostic(r.Context(), registryInstance)
	configured, authorizationCached, err := credentialProvider.SteamDownloadStatus(registryInstance)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "steam_download_status_failed", sanitizeErrorMsg(err, "读取公用 Steam 下载状态失败"))
		return
	}
	writeJSON(w, http.StatusOK, gameInstallationResponse{
		GameID: "stardew", DriverID: instance.DriverID, InstallationTargetID: instance.ID,
		Installed: diagnostic.RequiredFiles == "ok", RequiredFiles: diagnostic.RequiredFiles,
		CredentialsConfigured: configured, AuthorizationCached: authorizationCached,
		Instance: s.makeInstanceResponse(instance),
	})
}
