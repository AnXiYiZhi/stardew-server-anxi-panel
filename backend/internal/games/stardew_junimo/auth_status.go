package stardew_junimo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

const authStatusRequestTimeout = 10 * time.Second

// AuthStatusResult mirrors JunimoServer's GET /auth response (see
// docs/10-junimo-rest-api.md 3.5). SERVER_PASSWORD itself only takes effect
// when the server container starts, so this is a read-only status view.
type AuthStatusResult struct {
	Enabled            bool `json:"enabled"`
	AuthenticatedCount int  `json:"authenticatedCount"`
	PendingCount       int  `json:"pendingCount"`
	TimeoutSeconds     int  `json:"timeoutSeconds"`
	MaxAttempts        int  `json:"maxAttempts"`

	// PasswordBridgeAvailable/Detail come from the embedded control mod's
	// startup reflection self-check (see player_auth.go readPasswordBridgeStatus),
	// not from JunimoServer's REST API. The frontend uses this to disable the
	// "approve pending player" action up front when the reflection bridge into
	// JunimoServer's PasswordProtectionService failed to resolve.
	PasswordBridgeAvailable bool   `json:"passwordBridgeAvailable"`
	PasswordBridgeDetail    string `json:"passwordBridgeDetail,omitempty"`
}

// GetAuthStatus proxies JunimoServer's GET /auth endpoint from inside the
// server container, so the browser never sees the Junimo API key.
func (d *Driver) GetAuthStatus(ctx context.Context, instance registry.Instance) (*AuthStatusResult, error) {
	if instance.State != storage.InstanceStateRunning {
		return nil, &CommandError{Code: "server_not_running", Message: "服务器未运行，无法读取密码保护状态"}
	}

	ld, ok := d.docker.(LifecycleDockerService)
	if !ok {
		return nil, &CommandError{Code: "not_supported", Message: "Docker 服务不支持 Junimo API 调用"}
	}

	apiPort, apiKey, err := readJunimoAPIConfig(instance.DataDir)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("http://localhost:%s/auth", apiPort)
	args := []string{"curl", "-sf", "-X", "GET"}
	if apiKey != "" {
		args = append(args, "-H", "Authorization: Bearer "+apiKey)
	}
	args = append(args, url)

	reqCtx, cancel := context.WithTimeout(ctx, authStatusRequestTimeout)
	defer cancel()
	result, err := ld.ComposeExecPipe(reqCtx, instance.DataDir, "server", "", args...)
	if err != nil {
		if looksLikeJunimoAPIUnavailable(result.Stdout + "\n" + result.Stderr + "\n" + err.Error()) {
			return nil, &CommandError{
				Code:    "junimo_api_unavailable",
				Message: "JunimoServer API 未就绪，无法读取密码保护状态；请等待服务器完全启动。",
			}
		}
		return nil, err
	}
	if result.ExitCode != 0 {
		combined := result.Stdout + "\n" + result.Stderr
		if looksLikeJunimoAPIUnavailable(combined) {
			return nil, &CommandError{
				Code:    "junimo_api_unavailable",
				Message: "JunimoServer API 未就绪，无法读取密码保护状态；请等待服务器完全启动。",
			}
		}
		return nil, fmt.Errorf("GET /auth failed: %s", strings.TrimSpace(result.Stderr))
	}

	var status AuthStatusResult
	if err := json.Unmarshal([]byte(result.Stdout), &status); err != nil {
		return nil, fmt.Errorf("parse GET /auth response: %w", err)
	}
	bridge := readPasswordBridgeStatus(instance.DataDir)
	status.PasswordBridgeAvailable = bridge.Available
	status.PasswordBridgeDetail = bridge.Detail
	return &status, nil
}
