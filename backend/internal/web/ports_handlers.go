package web

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
)

type instanceVNCConfigResponse struct {
	VNCPort string `json:"vncPort"`
}

type updateVNCConfigRequest struct {
	Port string `json:"port"`
}

func (s *server) handleInstanceVNCConfig(w http.ResponseWriter, r *http.Request, instanceID string) {
	if r.Method == http.MethodGet {
		if _, ok := s.requireAdmin(w, r); !ok {
			return
		}
		instance, ok := s.loadInstance(w, r, instanceID)
		if !ok {
			return
		}
		port, err := readInstanceVNCPort(instance.DataDir)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "env_read_failed", sanitizeErrorMsg(err, "读取 VNC 端口失败"))
			return
		}
		writeJSON(w, http.StatusOK, instanceVNCConfigResponse{VNCPort: port})
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

	var body updateVNCConfigRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	port, ok := normalizePort(body.Port)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_vnc_port", "VNC 端口必须是 1 到 65535 之间的数字")
		return
	}

	envPath := filepath.Join(instance.DataDir, ".env")
	updates := map[string]string{"VNC_PORT": port}
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		updates = sjconfig.EmptyEnvTemplate()
		updates["VNC_PORT"] = port
	}
	if err := os.MkdirAll(instance.DataDir, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, "env_write_failed", sanitizeErrorMsg(err, "更新 VNC 端口失败"))
		return
	}
	if err := sjconfig.UpdateEnvFile(envPath, updates); err != nil {
		writeError(w, http.StatusInternalServerError, "env_write_failed", sanitizeErrorMsg(err, "更新 VNC 端口失败"))
		return
	}
	s.auditLog(r, &actor, "instance_vnc_port_update", "instance", instanceID, auditMetadata("vncPort", port))
	writeJSON(w, http.StatusOK, instanceVNCConfigResponse{VNCPort: port})
}

func readInstanceVNCPort(dataDir string) (string, error) {
	values, err := sjconfig.ReadEnvFile(filepath.Join(dataDir, ".env"))
	if err != nil {
		if os.IsNotExist(err) {
			return sjconfig.EmptyEnvTemplate()["VNC_PORT"], nil
		}
		return "", err
	}
	port := strings.TrimSpace(values["VNC_PORT"])
	if port == "" {
		port = sjconfig.EmptyEnvTemplate()["VNC_PORT"]
	}
	return port, nil
}

func normalizePort(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 1 || n > 65535 {
		return "", false
	}
	return strconv.Itoa(n), true
}
