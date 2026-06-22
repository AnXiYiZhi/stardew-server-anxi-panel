package web

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
)

type dockerStatusResponse struct {
	Docker         dockerAvailability   `json:"docker"`
	Compose        dockerAvailability   `json:"compose"`
	ComposeProject composeProjectStatus `json:"composeProject"`
}

type dockerAvailability struct {
	Available bool                       `json:"available"`
	Result    *paneldocker.CommandResult `json:"result,omitempty"`
}

type composeProjectStatus struct {
	WorkDir           string `json:"workDir"`
	WorkDirExists     bool   `json:"workDirExists"`
	ComposeFileExists bool   `json:"composeFileExists"`
	Ready             bool   `json:"ready"`
}

type composePsResponse struct {
	WorkDir  string                       `json:"workDir"`
	Result   paneldocker.CommandResult    `json:"result"`
	Services []paneldocker.ComposeService `json:"services"`
}

type composeLogsResponse struct {
	WorkDir string                    `json:"workDir"`
	Service string                    `json:"service"`
	Tail    int                       `json:"tail"`
	Result  paneldocker.CommandResult `json:"result"`
}

func (s *server) handleDockerStatus(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	workDir := s.config.DataDir
	project := s.composeProjectStatus()

	dockerResult, dockerErr := s.docker.DockerVersion(r.Context(), workDir)
	composeResult, composeErr := s.docker.ComposeVersion(r.Context(), workDir)

	response := dockerStatusResponse{
		Docker: dockerAvailability{
			Available: dockerErr == nil,
			Result:    &dockerResult,
		},
		Compose: dockerAvailability{
			Available: composeErr == nil,
			Result:    &composeResult,
		},
		ComposeProject: project,
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *server) handleDockerPs(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	project := s.composeProjectStatus()
	if !project.Ready {
		writeError(w, http.StatusConflict, "compose_project_not_ready", "Compose 工作目录尚未准备，请等待后续 Junimo 安装里程碑或手动准备 compose 文件")
		return
	}

	result, err := s.docker.ComposePs(r.Context(), project.WorkDir)
	if err != nil {
		s.writeDockerCommandError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, composePsResponse{
		WorkDir:  project.WorkDir,
		Result:   result.Result,
		Services: result.Services,
	})
}

func (s *server) handleDockerLogs(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	project := s.composeProjectStatus()
	if !project.Ready {
		writeError(w, http.StatusConflict, "compose_project_not_ready", "Compose 工作目录尚未准备，请等待后续 Junimo 安装里程碑或手动准备 compose 文件")
		return
	}

	service := r.URL.Query().Get("service")
	tail, err := parseTail(r.URL.Query().Get("tail"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_tail", "tail 必须是 1 到 1000 之间的整数")
		return
	}

	result, err := s.docker.ComposeLogs(r.Context(), project.WorkDir, paneldocker.LogsOptions{Service: service, Tail: tail})
	if err != nil {
		if errors.Is(err, paneldocker.ErrInvalidService) {
			writeError(w, http.StatusBadRequest, "invalid_service", "service 参数只能包含字母、数字、点、下划线或短横线")
			return
		}
		if errors.Is(err, paneldocker.ErrInvalidTail) {
			writeError(w, http.StatusBadRequest, "invalid_tail", "tail 必须是 1 到 1000 之间的整数")
			return
		}
		s.writeDockerCommandError(w, err)
		return
	}

	if tail == 0 {
		tail = paneldocker.DefaultLogTail
	}
	writeJSON(w, http.StatusOK, composeLogsResponse{
		WorkDir: project.WorkDir,
		Service: service,
		Tail:    tail,
		Result:  result,
	})
}

func (s *server) writeDockerCommandError(w http.ResponseWriter, err error) {
	var commandErr paneldocker.CommandError
	if errors.As(err, &commandErr) {
		status := http.StatusBadGateway
		code := "docker_command_failed"
		message := "Docker 命令执行失败"
		if commandErr.Result.TimedOut || errors.Is(commandErr.Err, paneldocker.ErrCommandTimeout) {
			status = http.StatusGatewayTimeout
			code = "docker_command_timeout"
			message = "Docker 命令执行超时"
		}
		writeErrorWithDetails(w, status, code, message, commandErr.Result)
		return
	}
	writeError(w, http.StatusInternalServerError, "internal_error", "服务器内部错误")
}

func (s *server) composeProjectStatus() composeProjectStatus {
	workDir := filepath.Join(s.config.DataDir, "instances", "stardew")
	status := composeProjectStatus{WorkDir: workDir}
	if stat, err := os.Stat(workDir); err == nil && stat.IsDir() {
		status.WorkDirExists = true
	}
	status.ComposeFileExists = composeFileExists(workDir)
	status.Ready = status.WorkDirExists && status.ComposeFileExists
	return status
}

func composeFileExists(workDir string) bool {
	for _, name := range []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"} {
		if stat, err := os.Stat(filepath.Join(workDir, name)); err == nil && !stat.IsDir() {
			return true
		}
	}
	return false
}

func parseTail(value string) (int, error) {
	if value == "" {
		return paneldocker.DefaultLogTail, nil
	}
	tail, err := strconv.Atoi(value)
	if err != nil || tail < 1 || tail > paneldocker.MaxLogTail {
		return 0, paneldocker.ErrInvalidTail
	}
	return tail, nil
}
