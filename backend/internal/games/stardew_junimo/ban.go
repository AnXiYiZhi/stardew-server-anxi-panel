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
// The vanilla Game1.bannedUsers list is process-local in this deployment;
// real-instance verification confirmed that it is lost after a server
// container restart. This stage intentionally does not add panel-side
// persistence or an unban/list management surface.
func (d *Driver) BanPlayer(ctx context.Context, instance registry.Instance, name, uniqueMultiplayerID string) (*CommandRunResult, error) {
	return banPlayer(ctx, d, instance, name, uniqueMultiplayerID)
}

// banPlayer is the testable core of Driver.BanPlayer.
func banPlayer(ctx context.Context, d *Driver, instance registry.Instance, name, uniqueMultiplayerID string) (*CommandRunResult, error) {
	name = strings.TrimSpace(name)
	uniqueMultiplayerID = strings.TrimSpace(uniqueMultiplayerID)
	if name == "" {
		return nil, &CommandError{Code: "invalid_player", Message: "缺少玩家名字"}
	}
	if uniqueMultiplayerID == "" {
		return nil, &CommandError{Code: "invalid_player", Message: "缺少玩家联机 ID"}
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
			return nil, &CommandError{Code: "admin_promotion_failed", Message: "提升 JunimoServer 管理员权限失败：" + ce.Message}
		}
		return nil, &CommandError{Code: "admin_promotion_failed", Message: "提升 JunimoServer 管理员权限失败"}
	}

	commandID, err := writePanelCommand(instance.DataDir, "ban", map[string]string{
		"name":                name,
		"uniqueMultiplayerId": uniqueMultiplayerID,
		"adminPromoted":       "true",
	})
	if err != nil {
		return nil, fmt.Errorf("写入封禁玩家命令失败: %w", err)
	}

	return &CommandRunResult{
		Command:    "ban",
		CommandID:  commandID,
		Status:     submissionStatus(instance.DataDir),
		Output:     "封禁操作已提交，控制模组会优先按玩家联机 ID 调用游戏服务器封禁能力。",
		ExitCode:   0,
		DurationMS: time.Since(start).Milliseconds(),
	}, nil
}
