package stardew_junimo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

const renderingRequestTimeout = 10 * time.Second

// RenderingResult is returned after reading or updating JunimoServer's
// server-side rendering FPS.
type RenderingResult struct {
	FPS    int    `json:"fps"`
	Output string `json:"output,omitempty"`
}

// GetRenderingFPS returns JunimoServer's current server-side rendering FPS.
func (d *Driver) GetRenderingFPS(ctx context.Context, instance registry.Instance) (*RenderingResult, error) {
	if instance.State != storage.InstanceStateRunning {
		return nil, &CommandError{Code: "server_not_running", Message: "服务器未运行，无法读取 VNC 显示状态"}
	}

	result, err := d.callRenderingAPI(ctx, instance, "GET", "")
	if err != nil {
		var ce *CommandError
		if errors.As(err, &ce) {
			return nil, ce
		}
		return nil, fmt.Errorf("GET /rendering: %w", err)
	}
	fps, err := parseRenderingFPS(result.Stdout)
	if err != nil {
		return nil, fmt.Errorf("parse GET /rendering response: %w", err)
	}
	return &RenderingResult{FPS: fps, Output: strings.TrimSpace(result.Stdout)}, nil
}

// SetRenderingFPS proxies to JunimoServer's POST /rendering endpoint from inside
// the server container, so the browser never sees the Junimo API key.
func (d *Driver) SetRenderingFPS(ctx context.Context, instance registry.Instance, fps int) (*RenderingResult, error) {
	if fps < 0 || fps > 60 {
		return nil, &CommandError{Code: "invalid_rendering_fps", Message: "VNC 显示帧率必须是 0 到 60 之间的数字"}
	}
	if instance.State != storage.InstanceStateRunning {
		return nil, &CommandError{Code: "server_not_running", Message: "服务器未运行，无法打开 VNC 显示"}
	}

	result, err := d.callRenderingAPI(ctx, instance, "POST", fmt.Sprintf("fps=%d", fps))
	if err != nil {
		var ce *CommandError
		if errors.As(err, &ce) {
			return nil, ce
		}
		return nil, fmt.Errorf("POST /rendering: %w", err)
	}
	return &RenderingResult{FPS: fps, Output: strings.TrimSpace(result.Stdout)}, nil
}

func (d *Driver) callRenderingAPI(ctx context.Context, instance registry.Instance, method, query string) (paneldocker.CommandResult, error) {
	ld, ok := d.docker.(LifecycleDockerService)
	if !ok {
		return paneldocker.CommandResult{}, &CommandError{Code: "not_supported", Message: "Docker 服务不支持 Junimo API 调用"}
	}

	apiPort, apiKey, err := readJunimoAPIConfig(instance.DataDir)
	if err != nil {
		return paneldocker.CommandResult{}, err
	}
	url := fmt.Sprintf("http://localhost:%s/rendering", apiPort)
	if query != "" {
		url += "?" + query
	}
	args := []string{"curl", "-sf", "-X", method}
	if method == "POST" {
		args = append(args, "-H", "Content-Length: 0")
	}
	if apiKey != "" {
		args = append(args, "-H", "Authorization: Bearer "+apiKey)
	}
	args = append(args, url)

	reqCtx, cancel := context.WithTimeout(ctx, renderingRequestTimeout)
	defer cancel()
	result, err := ld.ComposeExecPipe(reqCtx, instance.DataDir, "server", "", args...)
	if err != nil {
		if looksLikeJunimoAPIUnavailable(result.Stdout + "\n" + result.Stderr + "\n" + err.Error()) {
			return result, &CommandError{
				Code:    "junimo_api_unavailable",
				Message: "JunimoServer API 未就绪，无法控制 VNC 显示；请等待服务器完全启动，或检查 JunimoServer 官方 Mod 是否已加载。",
			}
		}
		return result, err
	}
	if result.ExitCode != 0 {
		combined := result.Stdout + "\n" + result.Stderr
		if looksLikeJunimoAPIUnavailable(combined) {
			return result, &CommandError{
				Code:    "junimo_api_unavailable",
				Message: "JunimoServer API 未就绪，无法控制 VNC 显示；请等待服务器完全启动，或检查 JunimoServer 官方 Mod 是否已加载。",
			}
		}
		return result, fmt.Errorf("%s /rendering failed: %s", method, strings.TrimSpace(result.Stderr))
	}
	return result, nil
}

func looksLikeJunimoAPIUnavailable(output string) bool {
	lower := strings.ToLower(output)
	return strings.Contains(lower, "failed to connect") ||
		strings.Contains(lower, "connection refused") ||
		strings.Contains(lower, "could not connect") ||
		strings.Contains(lower, "empty reply from server") ||
		strings.Contains(lower, "curl: (7)")
}

func parseRenderingFPS(output string) (int, error) {
	var payload struct {
		FPS int `json:"fps"`
	}
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		return 0, err
	}
	return payload.FPS, nil
}

func readJunimoAPIConfig(dataDir string) (string, string, error) {
	values, err := sjconfig.ReadEnvFile(filepath.Join(dataDir, ".env"))
	if err != nil && !os.IsNotExist(err) {
		return "", "", fmt.Errorf("read Junimo .env: %w", err)
	}
	defaults := sjconfig.EmptyEnvTemplate()
	apiPort := strings.TrimSpace(values["API_PORT"])
	if apiPort == "" {
		apiPort = defaults["API_PORT"]
	}
	n, err := strconv.Atoi(apiPort)
	if err != nil || n < 1 || n > 65535 {
		return "", "", fmt.Errorf("invalid Junimo API_PORT %q", apiPort)
	}
	return strconv.Itoa(n), strings.TrimSpace(values["API_KEY"]), nil
}
