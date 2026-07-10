package stardew_junimo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

// BanPlayer bans a player (online or offline) from rejoining the server.
// Upstream JunimoServer requires the triggering player to already hold the
// admin role (RoleService.IsPlayerAdmin), and the dedicated server's host
// player is not an admin by default, so this first promotes the host via
// JunimoServer's own POST /roles/admin before simulating the "!ban <name>"
// chat command through the embedded control mod (same pattern as EnableJojaRoute).
//
// Whether this ban survives a server container restart is unconfirmed:
// upstream JunimoServer writes it into vanilla Stardew Valley's
// Game1.bannedUsers, and neither JunimoServer nor this driver persists that
// list anywhere else. Callers must not present this as a guaranteed
// permanent ban until that is verified against the game assembly.
func (d *Driver) BanPlayer(ctx context.Context, instance registry.Instance, name, uniqueMultiplayerID string) (*CommandRunResult, error) {
	return banPlayer(ctx, d, instance, name)
}

// banPlayer is the testable core of Driver.BanPlayer.
func banPlayer(ctx context.Context, d *Driver, instance registry.Instance, name string) (*CommandRunResult, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, &CommandError{Code: "invalid_player", Message: "缺少玩家名字"}
	}
	if instance.State != storage.InstanceStateRunning {
		return nil, &CommandError{Code: "server_not_running", Message: "服务器未运行，无法封禁玩家"}
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

	if err := writePanelCommand(instance.DataDir, "ban", map[string]string{"name": name}); err != nil {
		return nil, fmt.Errorf("写入封禁玩家命令失败: %w", err)
	}

	return &CommandRunResult{
		Command:    "ban",
		Output:     "已将主机提升为管理员并提交 !ban 指令，控制模组会在游戏 tick 中执行；如果服务器容器重启，此封禁可能失效，需要重新操作。",
		ExitCode:   0,
		DurationMS: time.Since(start).Milliseconds(),
	}, nil
}
