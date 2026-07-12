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

// WarpHomeBridgeStatus mirrors the embedded control mod's reflection self-check
// for JunimoServer.Util.FarmerExtensions.WarpHome(Farmer). The bridge uses a
// JunimoServer public helper but still resolves it by reflection, so it can
// break if JunimoServer moves or renames the extension class.
type WarpHomeBridgeStatus struct {
	Available bool
	Detail    string
}

func readWarpHomeBridgeStatus(dataDir string) WarpHomeBridgeStatus {
	raw, err := os.ReadFile(filepath.Join(controlDir(dataDir), "status.json"))
	if err != nil {
		return WarpHomeBridgeStatus{}
	}
	var s struct {
		WarpHomeBridgeAvailable bool   `json:"warpHomeBridgeAvailable"`
		WarpHomeBridgeDetail    string `json:"warpHomeBridgeDetail"`
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		return WarpHomeBridgeStatus{}
	}
	return WarpHomeBridgeStatus{Available: s.WarpHomeBridgeAvailable, Detail: s.WarpHomeBridgeDetail}
}

// WarpPlayerHome asks the embedded control mod to call JunimoServer's own
// FarmerExtensions.WarpHome helper for the target online farmhand.
func (d *Driver) WarpPlayerHome(ctx context.Context, instance registry.Instance, uniqueMultiplayerID, name string) (*CommandRunResult, error) {
	return warpPlayerHome(instance, uniqueMultiplayerID, name)
}

// warpPlayerHome is the testable core of Driver.WarpPlayerHome.
func warpPlayerHome(instance registry.Instance, uniqueMultiplayerID, name string) (*CommandRunResult, error) {
	uniqueMultiplayerID = strings.TrimSpace(uniqueMultiplayerID)
	if uniqueMultiplayerID == "" {
		return nil, &CommandError{Code: "invalid_player", Message: "缺少玩家联机 ID"}
	}
	if instance.State != storage.InstanceStateRunning {
		return nil, &CommandError{Code: "server_not_running", Message: "服务器未运行，无法传送玩家回家"}
	}
	bridge := readWarpHomeBridgeStatus(instance.DataDir)
	if !bridge.Available {
		message := "回家传送反射桥不可用，无法调用 JunimoServer 的 WarpHome；请升级控制模组后重启服务器"
		if strings.TrimSpace(bridge.Detail) != "" {
			message += "（" + bridge.Detail + "）"
		}
		return nil, &CommandError{Code: "warp_home_bridge_unavailable", Message: message}
	}

	start := time.Now()
	commandID, err := writePanelCommand(instance.DataDir, "warp-home", map[string]string{
		"uniqueMultiplayerId": uniqueMultiplayerID,
		"name":                strings.TrimSpace(name),
	})
	if err != nil {
		return nil, fmt.Errorf("写入回家传送命令失败: %w", err)
	}
	return &CommandRunResult{
		Command:    "warp-home",
		CommandID:  commandID,
		Status:     submissionStatus(instance.DataDir),
		Output:     "回家传送指令已提交，控制模组会在游戏 tick 中调用 JunimoServer 的 WarpHome 逻辑。",
		ExitCode:   0,
		DurationMS: time.Since(start).Milliseconds(),
	}, nil
}
