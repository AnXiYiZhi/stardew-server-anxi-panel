package stardew_junimo

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

const (
	// jojaConfirmText mirrors the exact argument upstream JunimoServer's "!joja"
	// chat command requires. The frontend must ask the admin to type this text
	// verbatim before submitting; the backend re-checks it here as a hard gate.
	jojaConfirmText          = "IRREVERSIBLY_ENABLE_JOJA_RUN"
	rolesAdminRequestTimeout = 10 * time.Second
)

// EnableJojaRoute permanently disables the standard Community Center run for
// the current save in favor of the Joja route. This is IRREVERSIBLE for the
// life of the save. Upstream JunimoServer requires the triggering player to
// already hold the admin role (RoleService.IsPlayerAdmin), and the dedicated
// server's host player is not an admin by default, so this first promotes the
// host via JunimoServer's own POST /roles/admin before simulating the "!joja"
// chat command through the embedded control mod.
func (d *Driver) EnableJojaRoute(ctx context.Context, instance registry.Instance, confirm string) (*CommandRunResult, error) {
	return enableJojaRoute(ctx, d, instance, confirm)
}

// enableJojaRoute is the testable core of Driver.EnableJojaRoute.
func enableJojaRoute(ctx context.Context, d *Driver, instance registry.Instance, confirm string) (*CommandRunResult, error) {
	if strings.TrimSpace(confirm) != jojaConfirmText {
		return nil, &CommandError{Code: "confirm_mismatch", Message: fmt.Sprintf("确认文本不匹配，请准确输入 %s", jojaConfirmText)}
	}
	if instance.State != storage.InstanceStateRunning {
		return nil, &CommandError{Code: "server_not_running", Message: "服务器未运行，无法启用 Joja 路线"}
	}

	ld, ok := d.docker.(LifecycleDockerService)
	if !ok {
		return nil, &CommandError{Code: "not_supported", Message: "Docker 服务不支持 Junimo API 调用"}
	}

	hostID, ok := findHostPlayerID(instance.DataDir)
	if !ok {
		return nil, &CommandError{Code: "host_unknown", Message: "无法确定当前主机玩家 ID，请确认服务器已完全启动后重试"}
	}

	start := time.Now()
	if _, err := callRolesAdminAPI(ctx, ld, instance, hostID); err != nil {
		var ce *CommandError
		if errors.As(err, &ce) {
			return nil, ce
		}
		return nil, fmt.Errorf("POST /roles/admin: %w", err)
	}

	if err := writePanelCommand(instance.DataDir, "enable-joja", nil); err != nil {
		return nil, fmt.Errorf("写入启用 Joja 路线命令失败: %w", err)
	}

	return &CommandRunResult{
		Command:    "enable-joja",
		Output:     "已将主机提升为管理员并提交 !joja 指令，控制模组会在游戏 tick 中执行；此操作不可逆，将永久禁用标准社区中心路线。",
		ExitCode:   0,
		DurationMS: time.Since(start).Milliseconds(),
	}, nil
}

// findHostPlayerID returns the current host's UniqueMultiplayerID from the
// embedded control mod's players.json snapshot, when available.
func findHostPlayerID(dataDir string) (string, bool) {
	snapshot, ok := readPlayersFromControl(dataDir)
	if !ok {
		return "", false
	}
	for _, player := range snapshot.Players {
		if player.IsHost && strings.TrimSpace(player.UniqueMultiplayerID) != "" {
			return strings.TrimSpace(player.UniqueMultiplayerID), true
		}
	}
	return "", false
}

// callRolesAdminAPI proxies to JunimoServer's POST /roles/admin endpoint from
// inside the server container, so the browser never sees the Junimo API key.
func callRolesAdminAPI(ctx context.Context, ld LifecycleDockerService, instance registry.Instance, playerID string) (paneldocker.CommandResult, error) {
	apiPort, apiKey, err := readJunimoAPIConfig(instance.DataDir)
	if err != nil {
		return paneldocker.CommandResult{}, err
	}
	requestURL := fmt.Sprintf("http://localhost:%s/roles/admin?playerId=%s", apiPort, url.QueryEscape(playerID))
	args := []string{"curl", "-sf", "-X", "POST", "-H", "Content-Length: 0"}
	if apiKey != "" {
		args = append(args, "-H", "Authorization: Bearer "+apiKey)
	}
	args = append(args, requestURL)

	reqCtx, cancel := context.WithTimeout(ctx, rolesAdminRequestTimeout)
	defer cancel()
	result, err := ld.ComposeExecPipe(reqCtx, instance.DataDir, "server", "", args...)
	if err != nil {
		if looksLikeJunimoAPIUnavailable(result.Stdout + "\n" + result.Stderr + "\n" + err.Error()) {
			return result, &CommandError{
				Code:    "junimo_api_unavailable",
				Message: "JunimoServer API 未就绪，无法提升管理员权限；请等待服务器完全启动。",
			}
		}
		return result, err
	}
	if result.ExitCode != 0 {
		combined := result.Stdout + "\n" + result.Stderr
		if looksLikeJunimoAPIUnavailable(combined) {
			return result, &CommandError{
				Code:    "junimo_api_unavailable",
				Message: "JunimoServer API 未就绪，无法提升管理员权限；请等待服务器完全启动。",
			}
		}
		return result, fmt.Errorf("POST /roles/admin failed: %s", strings.TrimSpace(result.Stderr))
	}
	return result, nil
}
