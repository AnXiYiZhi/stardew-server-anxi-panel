package stardew_junimo

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

// PasswordBridgeStatus mirrors the embedded StardewAnxiPanel.Control mod's
// one-time reflection self-check for JunimoServer's PasswordProtectionService
// (see ModEntry.cs PasswordProtectionBridge.Initialize). The mod reflects into
// JunimoServer's private, non-contractual internals, so this flag lets the
// panel detect a broken reflection bridge (e.g. after a JunimoServer upgrade
// renames something) up front instead of only failing per approve click.
// A true value only means the reflection chain resolved at mod startup, not
// that every future call is guaranteed to succeed.
type PasswordBridgeStatus struct {
	Available bool
	Detail    string
}

// readPasswordBridgeStatus reads the mod-written control/status.json for the
// passwordBridgeAvailable/passwordBridgeDetail fields. Independent of
// readSMAPIStatus (lifecycle.go), which only cares about the "state" field
// for startup progress logging. Missing file, missing fields, or older mod
// builds without this self-check all resolve to Available=false.
func readPasswordBridgeStatus(dataDir string) PasswordBridgeStatus {
	raw, err := os.ReadFile(filepath.Join(controlDir(dataDir), "status.json"))
	if err != nil {
		return PasswordBridgeStatus{}
	}
	var s struct {
		PasswordBridgeAvailable bool   `json:"passwordBridgeAvailable"`
		PasswordBridgeDetail    string `json:"passwordBridgeDetail"`
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		return PasswordBridgeStatus{}
	}
	return PasswordBridgeStatus{Available: s.PasswordBridgeAvailable, Detail: s.PasswordBridgeDetail}
}

// ApproveAuth authenticates a pending player on the admin's behalf, using the
// instance's own SERVER_PASSWORD read by the control mod (never sent through
// this API). It is fire-and-forget like KickPlayer: the embedded control mod
// consumes the command on its next tick and reflects into JunimoServer's
// PasswordProtectionService to complete the authentication.
func (d *Driver) ApproveAuth(ctx context.Context, instance registry.Instance, uniqueMultiplayerID string) (*CommandRunResult, error) {
	return approveAuth(instance, uniqueMultiplayerID)
}

// approveAuth is the testable core of Driver.ApproveAuth.
func approveAuth(instance registry.Instance, uniqueMultiplayerID string) (*CommandRunResult, error) {
	uniqueMultiplayerID = strings.TrimSpace(uniqueMultiplayerID)
	if uniqueMultiplayerID == "" {
		return nil, &CommandError{Code: "invalid_player", Message: "缺少玩家联机 ID"}
	}
	if instance.State != storage.InstanceStateRunning {
		return nil, &CommandError{Code: "server_not_running", Message: "服务器未运行，无法批准认证"}
	}
	bridge := readPasswordBridgeStatus(instance.DataDir)
	if !bridge.Available {
		message := "密码认证反射桥不可用，无法批准；请让玩家在游戏内输入 !login 密码，或升级控制模组后重启服务器"
		if strings.TrimSpace(bridge.Detail) != "" {
			message += "（" + bridge.Detail + "）"
		}
		return nil, &CommandError{Code: "password_bridge_unavailable", Message: message}
	}

	start := time.Now()
	if err := writePanelCommand(instance.DataDir, "approve-auth", map[string]string{
		"uniqueMultiplayerId": uniqueMultiplayerID,
	}); err != nil {
		return nil, fmt.Errorf("写入批准认证命令失败: %w", err)
	}
	return &CommandRunResult{
		Command:    "approve-auth",
		Output:     "批准指令已提交，控制模组会在游戏 tick 中处理。",
		ExitCode:   0,
		DurationMS: time.Since(start).Milliseconds(),
	}, nil
}
