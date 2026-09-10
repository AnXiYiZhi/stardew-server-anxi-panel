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

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
)

const (
	farmhandDeleteCommandTimeout    = 20 * time.Second
	farmhandDeleteDisconnectTimeout = 30 * time.Second
	farmhandDeleteCountdownSeconds  = 60
)

type farmhandDeleteMaintenanceMarker struct {
	SchemaVersion       int        `json:"schemaVersion"`
	OperationID         string     `json:"operationId"`
	ExpectedSaveID      string     `json:"expectedSaveId"`
	TargetPlayerID      string     `json:"targetPlayerId"`
	Phase               string     `json:"phase"`
	CancellationCode    string     `json:"cancellationCode"`
	CancellationMessage string     `json:"cancellationMessage"`
	CreatedAt           time.Time  `json:"createdAt"`
	ExpiresAt           *time.Time `json:"expiresAt,omitempty"`
}

func pendingSaveCommandPath(dataDir string) string {
	return filepath.Join(controlDir(dataDir), "pending-save-command.json")
}

func ensureNoPendingSaveCommand(dataDir string) error {
	_, err := os.Stat(pendingSaveCommandPath(dataDir))
	if err == nil {
		return &CommandError{Code: "save_recovery_pending", Message: "上一条游戏保存仍在等待或恢复中，请等待完成；如长时间没有进展，请重启服务器后再试"}
	}
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("检查待恢复保存命令失败: %w", err)
	}
	return nil
}

func readFarmhandDeleteMaintenanceMarker(dataDir string) (farmhandDeleteMaintenanceMarker, error) {
	var marker farmhandDeleteMaintenanceMarker
	raw, err := os.ReadFile(filepath.Join(controlDir(dataDir), "farmhand-delete-maintenance.json"))
	if err != nil {
		return marker, err
	}
	if err := json.Unmarshal(raw, &marker); err != nil {
		return marker, err
	}
	if marker.SchemaVersion != 1 {
		return marker, fmt.Errorf("unsupported farmhand deletion maintenance marker schema %d", marker.SchemaVersion)
	}
	return marker, nil
}

func (r *farmhandDeleteRunner) runCountdown(ctx context.Context, job *jobs.Context) error {
	startedAt := time.Now()
	for index, remaining := range farmhandDeleteCountdownValues() {
		_, _ = job.Info(ctx, fmt.Sprintf("维护删除将在 %d 秒后开始；正在确认无人睡觉或进入日结算。", remaining))
		if err := r.runMaintenanceCommand(ctx, "farmhand-delete-countdown", map[string]string{
			"operationId": r.operationID, "expectedSaveId": r.expectedSave,
			"uniqueMultiplayerId": r.playerID, "seconds": strconv.Itoa(remaining),
		}); err != nil {
			return err
		}
		if err := r.waitCountdownInterval(ctx, time.Until(startedAt.Add(time.Duration(index+1)*10*time.Second))); err != nil {
			return err
		}
	}
	return nil
}

func (r *farmhandDeleteRunner) waitCountdownInterval(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return nil
		case <-ticker.C:
			if err := r.checkLocalMaintenanceCancellation(); err != nil {
				return err
			}
		}
	}
}

func farmhandDeleteCountdownValues() []int {
	values := make([]int, 0, farmhandDeleteCountdownSeconds/10)
	for remaining := farmhandDeleteCountdownSeconds; remaining >= 10; remaining -= 10 {
		values = append(values, remaining)
	}
	return values
}

func (r *farmhandDeleteRunner) beginMaintenance(ctx context.Context) error {
	return r.runMaintenanceCommand(ctx, "farmhand-delete-maintenance-begin", map[string]string{
		"operationId": r.operationID, "expectedSaveId": r.expectedSave, "uniqueMultiplayerId": r.playerID, "mode": r.mode,
	})
}

func (r *farmhandDeleteRunner) sealMaintenance(ctx context.Context) error {
	return r.runMaintenanceCommand(ctx, "farmhand-delete-maintenance-seal", map[string]string{"operationId": r.operationID})
}

func (r *farmhandDeleteRunner) checkMaintenance(ctx context.Context) error {
	return r.runMaintenanceCommand(ctx, "farmhand-delete-maintenance-check", map[string]string{
		"operationId": r.operationID, "expectedSaveId": r.expectedSave,
	})
}

func (r *farmhandDeleteRunner) endMaintenance(ctx context.Context) error {
	return r.runMaintenanceCommand(ctx, "farmhand-delete-maintenance-end", map[string]string{"operationId": r.operationID})
}

func (r *farmhandDeleteRunner) runMaintenanceCommand(ctx context.Context, name string, payload map[string]string) error {
	commandID, err := r.writeExpiringCommand(name, payload, farmhandDeleteCommandTimeout)
	if err != nil {
		return err
	}
	outcome, err := r.waitCommandOutcome(ctx, commandID, farmhandDeleteCommandTimeout)
	if err != nil {
		paths, globErr := filepath.Glob(filepath.Join(controlDir(r.instance.DataDir), "commands", "*-"+commandID+".json"))
		if globErr != nil {
			return errors.Join(err, globErr)
		}
		for _, path := range paths {
			if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				return errors.Join(err, removeErr)
			}
		}
		return err
	}
	if outcome.Status == CommandStatusSucceeded {
		return nil
	}
	message := strings.TrimSpace(outcome.Message)
	if message == "" {
		message = "Control 未确认人物删除维护命令"
	}
	return &CommandError{Code: outcome.ErrorCode, Message: maintenanceCommandMessage(outcome.ErrorCode, message)}
}

func (r *farmhandDeleteRunner) writeExpiringCommand(name string, payload map[string]string, lifetime time.Duration) (string, error) {
	payload["operationId"] = r.operationID
	payload["expiresAt"] = time.Now().UTC().Add(lifetime).Format(time.RFC3339Nano)
	return writePanelCommand(r.instance.DataDir, name, payload)
}

func maintenanceCommandMessage(code, fallback string) string {
	switch code {
	case "sleep_in_progress", "day_transition_in_progress":
		return "检测到玩家正在睡觉或游戏正在日结算，本次人物删除已取消"
	case "active_save_changed":
		return "当前活动存档已变化，本次人物删除已取消"
	case "save_recovery_pending", "save_already_pending":
		return "上一条游戏保存仍在等待或恢复中，本次人物删除未开始"
	case "world_not_ready":
		return "游戏世界尚未准备完成，本次人物删除未开始"
	default:
		return fallback
	}
}

func (r *farmhandDeleteRunner) waitCommandOutcome(ctx context.Context, commandID string, timeout time.Duration) (CommandOutcome, error) {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		outcome, err := r.driver.importCommandOutcome(ctx, r.instance.ID, r.instance.DataDir, commandID)
		if err != nil {
			return outcome, err
		}
		switch outcome.Status {
		case CommandStatusSucceeded, CommandStatusDispatched, CommandStatusFailed, CommandStatusExpired:
			return outcome, nil
		}
		select {
		case <-ctx.Done():
			return outcome, ctx.Err()
		case <-deadline.C:
			return outcome, errors.New("未在期限内收到 Control 命令执行确认")
		case <-ticker.C:
		}
	}
}

func (r *farmhandDeleteRunner) waitForNoConnectedHumans(ctx context.Context) error {
	deadline := time.NewTimer(farmhandDeleteDisconnectTimeout)
	defer deadline.Stop()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := r.checkLocalMaintenanceCancellation(); err != nil {
			return err
		}
		players, err := r.driver.ListPlayers(ctx, r.instance)
		if err == nil {
			connected, countErr := r.connectedHumansFromSnapshot(players)
			if countErr != nil {
				return countErr
			}
			if len(connected) == 0 {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return &CommandError{Code: "players_disconnect_timeout", Message: "在线玩家未能在维护期限内全部断开，人物删除未开始"}
		case <-ticker.C:
		}
	}
}

func (r *farmhandDeleteRunner) checkLocalMaintenanceCancellation() error {
	marker, err := readFarmhandDeleteMaintenanceMarker(r.instance.DataDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("读取人物删除维护状态失败: %w", err)
	}
	if marker.OperationID != r.operationID {
		return &CommandError{Code: "farmhand_delete_maintenance_conflict", Message: "人物删除维护状态已由另一操作持有"}
	}
	if marker.Phase != "canceled" {
		return nil
	}
	code := strings.TrimSpace(marker.CancellationCode)
	if code == "" {
		code = "farmhand_delete_canceled"
	}
	message := strings.TrimSpace(marker.CancellationMessage)
	if message == "" {
		message = "人物删除已在破坏性操作前取消"
	}
	return &CommandError{Code: code, Message: maintenanceCommandMessage(code, message)}
}

func connectedHumans(players []PlayerInfo, targetPlayerID string) ([]PlayerInfo, error) {
	connected := make([]PlayerInfo, 0)
	for _, player := range players {
		if player.IsHost || !isConnectedPlayerStatus(player.Status) {
			continue
		}
		if player.UniqueMultiplayerID == targetPlayerID {
			return nil, &CommandError{Code: "farmhand_online", Message: "被删除的人物当前在线，人物删除已取消"}
		}
		connected = append(connected, player)
	}
	return connected, nil
}

func isConnectedPlayerStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "online", "waiting", "pending", "joining":
		return true
	default:
		return false
	}
}

func (r *farmhandDeleteRunner) connectedHumansFromSnapshot(players *PlayersResult) ([]PlayerInfo, error) {
	if players == nil || players.Source != "smapi_control" || players.ParseStatus != "exact" || players.SaveID != r.expectedSave || players.OnlineCount == nil {
		return nil, &CommandError{Code: "world_not_ready", Message: "无法确认目标存档的完整在线快照，人物删除未开始"}
	}
	observedAt, err := time.Parse(time.RFC3339Nano, players.UpdatedAt)
	age := time.Since(observedAt)
	if err != nil || age < -5*time.Second || age > 10*time.Second {
		return nil, &CommandError{Code: "world_not_ready", Message: "在线玩家快照已过期，人物删除未开始"}
	}
	listed := 0
	for _, player := range players.Players {
		if isConnectedPlayerStatus(player.Status) {
			if strings.TrimSpace(player.UniqueMultiplayerID) == "" {
				return nil, &CommandError{Code: "world_not_ready", Message: "在线玩家身份不完整，人物删除未开始"}
			}
			listed++
		}
	}
	if listed != *players.OnlineCount {
		return nil, &CommandError{Code: "world_not_ready", Message: "在线人数与玩家列表不一致，人物删除未开始"}
	}
	return connectedHumans(players.Players, r.playerID)
}
