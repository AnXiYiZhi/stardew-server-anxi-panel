package web

import (
	"context"
	"net/http"
	"strconv"
	"unicode/utf8"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
)

const maxServerPasswordLength = 128

type instanceServerPasswordResponse struct {
	ServerPassword string `json:"serverPassword"`
}

type updateServerPasswordRequest struct {
	Password string `json:"password"`
}

type playerAuthConfigManager interface {
	GetPlayerAuthConfig(ctx context.Context, instance registry.Instance) (*sj.PlayerAuthConfigResult, error)
	UpdatePlayerAuthConfig(ctx context.Context, instance registry.Instance, request sj.UpdatePlayerAuthConfigRequest) (*sj.PlayerAuthConfigResult, error)
}

// handleInstanceServerPassword handles GET/PUT /api/instances/:id/config/server-password.
// SERVER_PASSWORD only takes effect when the Junimo `server` container starts,
// so writing it here does not affect an already-running server.
func (s *server) handleInstanceServerPassword(w http.ResponseWriter, r *http.Request, instanceID string) {
	if r.Method != http.MethodGet && r.Method != http.MethodPut {
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
	instance, ok = s.reconcileInstanceState(w, r, instance)
	if !ok {
		return
	}
	driver, ok := s.loadDriver(w, instance.DriverID)
	if !ok {
		return
	}
	manager, supported := driver.(playerAuthConfigManager)
	if !supported {
		writeError(w, http.StatusNotImplemented, "not_supported", "该 driver 不支持玩家加入保护配置")
		return
	}
	current, err := manager.GetPlayerAuthConfig(r.Context(), makeRegistryInstance(instance))
	if err != nil {
		writePlayerAuthConfigError(w, err, "读取服务器密码失败")
		return
	}
	if current.Mode == sj.PlayerAuthModeRole {
		writeError(w, http.StatusConflict, "role_auth_mode_active", "当前已启用角色独立密码，请使用玩家加入保护设置修改")
		return
	}
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, instanceServerPasswordResponse{ServerPassword: current.GlobalPassword})
		return
	}

	var body updateServerPasswordRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	if utf8.RuneCountInString(body.Password) > maxServerPasswordLength {
		writeError(w, http.StatusBadRequest, "invalid_server_password", "服务器密码不能超过 128 个字符")
		return
	}
	mode := sj.PlayerAuthModeNone
	var globalPassword *string
	if body.Password != "" {
		mode = sj.PlayerAuthModeGlobal
		globalPassword = &body.Password
	}
	if _, err := manager.UpdatePlayerAuthConfig(r.Context(), makeRegistryInstance(instance), sj.UpdatePlayerAuthConfigRequest{
		ExpectedRevision: current.Revision,
		Mode:             mode,
		GlobalPassword:   globalPassword,
	}); err != nil {
		if writeStardewMutationGuardConflict(w, err) {
			return
		}
		writePlayerAuthConfigError(w, err, "更新服务器密码失败")
		return
	}
	s.auditLog(r, &actor, "instance_server_password_update", "instance", instanceID, auditMetadata("passwordSet", boolLabel(body.Password != "")))
	writeJSON(w, http.StatusOK, instanceServerPasswordResponse{ServerPassword: body.Password})
}

func (s *server) handleInstancePlayerAuthConfig(w http.ResponseWriter, r *http.Request, instanceID string) {
	if r.Method != http.MethodGet && r.Method != http.MethodPut {
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
	instance, ok = s.reconcileInstanceState(w, r, instance)
	if !ok {
		return
	}
	driver, ok := s.loadDriver(w, instance.DriverID)
	if !ok {
		return
	}
	manager, supported := driver.(playerAuthConfigManager)
	if !supported {
		writeError(w, http.StatusNotImplemented, "not_supported", "该 driver 不支持玩家加入保护配置")
		return
	}
	if r.Method == http.MethodGet {
		result, err := manager.GetPlayerAuthConfig(r.Context(), makeRegistryInstance(instance))
		if err != nil {
			writePlayerAuthConfigError(w, err, "读取玩家加入保护配置失败")
			return
		}
		writeJSON(w, http.StatusOK, result)
		return
	}
	var body sj.UpdatePlayerAuthConfigRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	result, err := manager.UpdatePlayerAuthConfig(r.Context(), makeRegistryInstance(instance), body)
	if err != nil {
		if writeStardewMutationGuardConflict(w, err) {
			return
		}
		writePlayerAuthConfigError(w, err, "更新玩家加入保护配置失败")
		return
	}
	s.auditLog(r, &actor, "instance_player_auth_update", "instance", instanceID, auditMetadata(
		"mode", result.Mode,
		"configuredRoleCount", strconv.Itoa(result.ConfiguredRoleCount),
		"removedRoleCount", strconv.Itoa(len(body.RolePasswordRemovals)),
		"updatedRoleCount", strconv.Itoa(len(body.RolePasswordUpdates)),
	))
	writeJSON(w, http.StatusOK, result)
}

func writePlayerAuthConfigError(w http.ResponseWriter, err error, fallback string) {
	if ce, ok := err.(*sj.CommandError); ok {
		status := http.StatusBadRequest
		switch ce.Code {
		case "player_auth_revision_conflict", "role_auth_mode_active":
			status = http.StatusConflict
		case "not_supported":
			status = http.StatusNotImplemented
		case "role_auth_config_invalid", "role_auth_guard_mismatch":
			status = http.StatusConflict
		}
		writeError(w, status, ce.Code, ce.Message)
		return
	}
	writeError(w, http.StatusInternalServerError, "player_auth_config_failed", sanitizeErrorMsg(err, fallback))
}

func boolLabel(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

type authStatusGetter interface {
	GetAuthStatus(ctx context.Context, instance registry.Instance) (*sj.AuthStatusResult, error)
}

// handleInstancePasswordStatus handles GET /api/instances/:id/password-status.
// It proxies JunimoServer's GET /auth so the panel can show whether password
// protection is currently active and how many players are authenticated.
func (s *server) handleInstancePasswordStatus(w http.ResponseWriter, r *http.Request, instanceID string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if _, ok := s.requireAdmin(w, r); !ok {
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
	getter, supported := driver.(authStatusGetter)
	if !supported {
		writeError(w, http.StatusNotImplemented, "not_supported", "该 driver 不支持密码保护状态查询")
		return
	}
	result, err := getter.GetAuthStatus(r.Context(), makeRegistryInstance(instance))
	if err != nil {
		if ce, ok := err.(*sj.CommandError); ok {
			status := http.StatusBadRequest
			switch ce.Code {
			case "server_not_running":
				status = http.StatusConflict
			case "junimo_api_unavailable":
				status = http.StatusBadGateway
			case "not_supported":
				status = http.StatusNotImplemented
			}
			writeError(w, status, ce.Code, ce.Message)
			return
		}
		writeError(w, http.StatusBadGateway, "auth_status_failed", sanitizeErrorMsg(err, "读取密码保护状态失败"))
		return
	}
	writeJSON(w, http.StatusOK, result)
}
