package web

import (
	"context"
	"net/http"
	"os"
	"path/filepath"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
)

const maxServerPasswordLength = 128

type instanceServerPasswordResponse struct {
	ServerPassword string `json:"serverPassword"`
}

type updateServerPasswordRequest struct {
	Password string `json:"password"`
}

// handleInstanceServerPassword handles GET/PUT /api/instances/:id/config/server-password.
// SERVER_PASSWORD only takes effect when the Junimo `server` container starts,
// so writing it here does not affect an already-running server.
func (s *server) handleInstanceServerPassword(w http.ResponseWriter, r *http.Request, instanceID string) {
	if r.Method == http.MethodGet {
		if _, ok := s.requireAdmin(w, r); !ok {
			return
		}
		instance, ok := s.loadInstance(w, r, instanceID)
		if !ok {
			return
		}
		password, err := readInstanceServerPassword(instance.DataDir)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "env_read_failed", sanitizeErrorMsg(err, "读取服务器密码失败"))
			return
		}
		writeJSON(w, http.StatusOK, instanceServerPasswordResponse{ServerPassword: password})
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

	var body updateServerPasswordRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	if len(body.Password) > maxServerPasswordLength {
		writeError(w, http.StatusBadRequest, "invalid_server_password", "服务器密码不能超过 128 个字符")
		return
	}

	envPath := filepath.Join(instance.DataDir, ".env")
	updates := map[string]string{"SERVER_PASSWORD": body.Password}
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		updates = sjconfig.EmptyEnvTemplate()
		updates["SERVER_PASSWORD"] = body.Password
	}
	if err := os.MkdirAll(instance.DataDir, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, "env_write_failed", sanitizeErrorMsg(err, "更新服务器密码失败"))
		return
	}
	if err := sjconfig.UpdateEnvFile(envPath, updates); err != nil {
		writeError(w, http.StatusInternalServerError, "env_write_failed", sanitizeErrorMsg(err, "更新服务器密码失败"))
		return
	}
	s.auditLog(r, &actor, "instance_server_password_update", "instance", instanceID, auditMetadata("passwordSet", boolLabel(body.Password != "")))
	writeJSON(w, http.StatusOK, instanceServerPasswordResponse{ServerPassword: body.Password})
}

func readInstanceServerPassword(dataDir string) (string, error) {
	values, err := sjconfig.ReadEnvFile(filepath.Join(dataDir, ".env"))
	if err != nil {
		if os.IsNotExist(err) {
			return sjconfig.EmptyEnvTemplate()["SERVER_PASSWORD"], nil
		}
		return "", err
	}
	return values["SERVER_PASSWORD"], nil
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
