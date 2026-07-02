package web

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

const (
	panelSettingNexusAPIKey = "nexus_api_key"
)

type nexusSettingsResponse struct {
	Configured bool   `json:"configured"`
	Last4      string `json:"last4,omitempty"`
}

type nexusAPIKeyRequest struct {
	APIKey string `json:"apiKey"`
}

func (s *server) handleNexusSettingsStatus(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	status, err := s.nexusSettingsStatus(r.Context())
	if err != nil {
		s.logger.Error("failed to load nexus settings", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *server) handleNexusAPIKey(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPut:
		s.handleNexusAPIKeySave(w, r)
	case http.MethodDelete:
		s.handleNexusAPIKeyDelete(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (s *server) handleNexusAPIKeySave(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	var req nexusAPIKeyRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	apiKey := strings.TrimSpace(req.APIKey)
	if apiKey == "" {
		writeError(w, http.StatusBadRequest, "missing_api_key", "Nexus API Key cannot be empty")
		return
	}
	if err := s.store.SetPanelSetting(r.Context(), panelSettingNexusAPIKey, apiKey); err != nil {
		s.logger.Error("failed to save nexus api key", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	s.auditLog(r, &actor, "nexus_api_key_update", "setting", "nexus", "{}")
	writeJSON(w, http.StatusOK, nexusSettingsResponse{Configured: true, Last4: last4(apiKey)})
}

func (s *server) handleNexusAPIKeyDelete(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	if err := s.store.DeletePanelSetting(r.Context(), panelSettingNexusAPIKey); err != nil {
		s.logger.Error("failed to delete nexus api key", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	s.auditLog(r, &actor, "nexus_api_key_delete", "setting", "nexus", "{}")
	writeJSON(w, http.StatusOK, nexusSettingsResponse{Configured: false})
}

func (s *server) nexusSettingsStatus(ctx context.Context) (nexusSettingsResponse, error) {
	apiKey, err := s.nexusAPIKey(ctx)
	if err != nil {
		return nexusSettingsResponse{}, err
	}
	if apiKey == "" {
		return nexusSettingsResponse{Configured: false}, nil
	}
	return nexusSettingsResponse{Configured: true, Last4: last4(apiKey)}, nil
}

func (s *server) nexusAPIKey(ctx context.Context) (string, error) {
	apiKey, err := s.store.GetPanelSetting(ctx, panelSettingNexusAPIKey)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(apiKey), nil
}

func last4(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 4 {
		return ""
	}
	return value[len(value)-4:]
}
