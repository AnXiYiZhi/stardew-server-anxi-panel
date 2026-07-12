package web

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
)

// globalWorkDir returns a directory suitable for docker commands that don't
// need a specific project directory (e.g. "docker version").
func globalWorkDir() string { return os.TempDir() }

// HealthCheck represents a single diagnostic check result.
type HealthCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // "ok", "warning", "error"
	Message string `json:"message"`
}

// handleHealthDiagnostics handles GET /api/health/diagnostics.
// Returns a list of diagnostic checks with human-readable Chinese messages.
// Requires authentication.
func (s *server) handleHealthDiagnostics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if _, ok := s.requireAuth(w, r); !ok {
		return
	}

	var checks []HealthCheck

	// 1. Docker daemon
	dockerOK := s.checkDockerDaemon(r)
	if dockerOK {
		checks = append(checks, HealthCheck{Name: "docker_daemon", Status: "ok", Message: "Docker 服务正常"})
	} else {
		checks = append(checks, HealthCheck{Name: "docker_daemon", Status: "error", Message: "Docker 服务不可用，请确认 Docker 已启动且面板有 socket 权限"})
	}

	// 2. Docker Compose
	composeOK := s.checkDockerCompose(r)
	if composeOK {
		checks = append(checks, HealthCheck{Name: "docker_compose", Status: "ok", Message: "Docker Compose 可用"})
	} else {
		checks = append(checks, HealthCheck{Name: "docker_compose", Status: "error", Message: "Docker Compose 不可用，请确认安装了 Compose V2 插件"})
	}

	// 3. Data directory
	dataDirOK := s.checkDataDir()
	if dataDirOK {
		checks = append(checks, HealthCheck{Name: "data_dir", Status: "ok", Message: "数据目录可写"})
	} else {
		checks = append(checks, HealthCheck{Name: "data_dir", Status: "error", Message: "数据目录不可写，请检查 /data 目录权限"})
	}

	// 4. Instance directory and compose file
	instanceChecks := s.checkInstance(r)
	checks = append(checks, instanceChecks...)

	// 5. Active save
	saveChecks := s.checkActiveSave(r)
	checks = append(checks, saveChecks...)

	// 6. Control command result protocol
	if instance, err := s.store.GetInstance(r.Context(), s.config.DefaultInstanceID); err == nil {
		diagnostic := s.commandProtocolDiagnostics(r.Context(), instance)
		warnings, _ := diagnostic["warnings"].([]string)
		if len(warnings) == 0 {
			checks = append(checks, HealthCheck{Name: "control_commands", Status: "ok", Message: "控制命令队列与回执目录正常"})
		} else {
			for i, message := range warnings {
				checks = append(checks, HealthCheck{Name: fmt.Sprintf("control_commands_%d", i+1), Status: "warning", Message: message})
			}
		}
	}

	// Determine overall status.
	overallStatus := "ok"
	for _, c := range checks {
		if c.Status == "error" {
			overallStatus = "error"
			break
		}
		if c.Status == "warning" {
			overallStatus = "warning"
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": overallStatus,
		"checks": checks,
	})
}

func (s *server) checkDockerDaemon(r *http.Request) bool {
	result, err := s.docker.DockerVersion(r.Context(), globalWorkDir())
	return err == nil && result.ExitCode == 0
}

func (s *server) checkDockerCompose(r *http.Request) bool {
	result, err := s.docker.ComposeVersion(r.Context(), globalWorkDir())
	return err == nil && result.ExitCode == 0
}

func (s *server) checkDataDir() bool {
	dir := s.config.DataDir
	if dir == "" {
		return false
	}
	// Check if directory exists and is writable.
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return false
	}
	// Try to create a temp file to verify write access.
	testFile := filepath.Join(dir, ".health-check-test")
	if err := os.WriteFile(testFile, []byte("test"), 0o644); err != nil {
		return false
	}
	_ = os.Remove(testFile)
	return true
}

func (s *server) checkInstance(r *http.Request) []HealthCheck {
	var checks []HealthCheck

	instance, err := s.store.GetInstance(r.Context(), s.config.DefaultInstanceID)
	if err != nil {
		checks = append(checks, HealthCheck{
			Name:    "instance_dir",
			Status:  "error",
			Message: "默认实例不存在，请重新初始化面板",
		})
		return checks
	}

	// Check instance directory.
	if _, err := os.Stat(instance.DataDir); err != nil {
		checks = append(checks, HealthCheck{
			Name:    "instance_dir",
			Status:  "error",
			Message: "实例目录不存在，请点击「准备实例」创建",
		})
	} else {
		checks = append(checks, HealthCheck{
			Name:    "instance_dir",
			Status:  "ok",
			Message: "实例目录已就绪",
		})
	}

	// Check compose file.
	composePath := filepath.Join(instance.DataDir, "docker-compose.yml")
	if _, err := os.Stat(composePath); err != nil {
		checks = append(checks, HealthCheck{
			Name:    "compose_file",
			Status:  "warning",
			Message: "docker-compose.yml 不存在，需要重新安装",
		})
	} else {
		checks = append(checks, HealthCheck{
			Name:    "compose_file",
			Status:  "ok",
			Message: "docker-compose.yml 已就绪",
		})
	}

	return checks
}

func (s *server) checkActiveSave(r *http.Request) []HealthCheck {
	var checks []HealthCheck

	instance, err := s.store.GetInstance(r.Context(), s.config.DefaultInstanceID)
	if err != nil {
		return checks
	}

	activeSave := sj.GetActiveSaveName(instance.DataDir)
	if activeSave == "" {
		checks = append(checks, HealthCheck{
			Name:    "active_save",
			Status:  "warning",
			Message: "没有已选择的启动存档，请创建或选择一个存档",
		})
		return checks
	}

	if err := sj.ValidateSaveExists(instance.DataDir, activeSave); err != nil {
		checks = append(checks, HealthCheck{
			Name:    "active_save",
			Status:  "error",
			Message: "已选择的存档已被删除或不存在，请重新选择存档",
		})
	} else {
		checks = append(checks, HealthCheck{
			Name:    "active_save",
			Status:  "ok",
			Message: "启动存档已就绪",
		})
	}

	return checks
}
