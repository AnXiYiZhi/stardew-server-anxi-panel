package stardew_junimo

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

const (
	FarmhandDeleteJobType = "stardew_farmhand_delete"
	farmhandDeleteTimeout = 10 * time.Minute
)

func (d *Driver) rejectActiveFarmhandDelete(ctx context.Context, instanceID string) error {
	if d.jobs == nil {
		return nil
	}
	active, err := d.jobs.Active(ctx, storage.ListActiveJobsFilter{TargetType: "instance", TargetID: instanceID, Types: []string{FarmhandDeleteJobType}})
	if err != nil {
		return fmt.Errorf("读取人物删除任务失败: %w", err)
	}
	if len(active) > 0 {
		return &CommandError{Code: "farmhand_delete_in_progress", Message: "存档人物删除正在执行，请等待保存和验证完成"}
	}
	return nil
}

type FarmhandDeleteRequest struct {
	Instance     registry.Instance
	PlayerID     string
	ExpectedName string
	ExpectedSave string
	ActorID      int64
}

type farmhandDeletePayload struct {
	OperationID  string `json:"operationId"`
	PlayerID     string `json:"uniqueMultiplayerId"`
	ExpectedName string `json:"expectedName,omitempty"`
	ExpectedSave string `json:"expectedSaveId"`
}

type junimoFarmhand struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	IsCustomized bool   `json:"isCustomized"`
}

type junimoFarmhandsResponse struct {
	Farmhands []junimoFarmhand `json:"farmhands"`
	Version   int64            `json:"version"`
}

type junimoFarmhandDeleteResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Error   string `json:"error"`
}

type farmhandDeleteRunner struct {
	driver       *Driver
	lifecycle    LifecycleDockerService
	instance     registry.Instance
	playerID     string
	playerIDInt  int64
	expectedName string
	expectedSave string
	operationID  string
}

// DeleteFarmhand starts a guarded, asynchronous deletion of one offline
// farmhand from the currently loaded save. Other human players may remain
// online; they receive in-game notices because Junimo's cabin removal is not
// guaranteed to refresh an already-connected client's building snapshot.
func (d *Driver) DeleteFarmhand(ctx context.Context, req FarmhandDeleteRequest) (*registry.Job, error) {
	if d.jobs == nil || d.store == nil {
		return nil, &CommandError{Code: "not_supported", Message: "人物删除任务服务未配置"}
	}
	if req.Instance.State != storage.InstanceStateRunning {
		return nil, &CommandError{Code: "server_not_running", Message: "服务器未运行，无法删除存档人物"}
	}
	playerID := strings.TrimSpace(req.PlayerID)
	parsedID, err := strconv.ParseInt(playerID, 10, 64)
	if err != nil {
		return nil, &CommandError{Code: "invalid_player", Message: "玩家联机 ID 无效"}
	}
	expectedSave := strings.TrimSpace(req.ExpectedSave)
	activeSave := strings.TrimSpace(GetActiveSaveName(req.Instance.DataDir))
	if expectedSave == "" || activeSave == "" || expectedSave != activeSave {
		return nil, &CommandError{Code: "active_save_changed", Message: "当前激活存档已变化，请刷新后重试"}
	}
	ld, ok := d.docker.(LifecycleDockerService)
	if !ok {
		return nil, &CommandError{Code: "not_supported", Message: "Docker 服务不支持 Junimo API 调用"}
	}
	// Serialize the active-job check and job creation. Without this guard, two
	// admins clicking at the same instant could both observe an empty job list.
	d.mu.Lock()
	defer d.mu.Unlock()
	active, err := d.jobs.Active(ctx, storage.ListActiveJobsFilter{TargetType: "instance", TargetID: req.Instance.ID})
	if err != nil {
		return nil, fmt.Errorf("读取活动任务失败: %w", err)
	}
	if len(active) > 0 {
		return nil, &CommandError{Code: "operation_in_progress", Message: "当前有其他服务器任务正在执行，请等待完成后重试"}
	}
	operationID, err := randomHex(16)
	if err != nil {
		return nil, err
	}
	payload := farmhandDeletePayload{OperationID: operationID, PlayerID: playerID, ExpectedName: strings.TrimSpace(req.ExpectedName), ExpectedSave: expectedSave}
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	runner := &farmhandDeleteRunner{driver: d, lifecycle: ld, instance: req.Instance, playerID: playerID, playerIDInt: parsedID,
		expectedName: payload.ExpectedName, expectedSave: expectedSave, operationID: operationID}
	job, err := d.jobs.Start(ctx, jobs.Spec{Type: FarmhandDeleteJobType, DisplayName: "删除离线存档人物", TargetType: "instance",
		TargetID: req.Instance.ID, CreatedBy: req.ActorID, Payload: string(rawPayload), Timeout: farmhandDeleteTimeout, Run: runner.run})
	if err != nil {
		return nil, fmt.Errorf("创建人物删除任务失败: %w", err)
	}
	return &registry.Job{ID: job.ID}, nil
}

func (r *farmhandDeleteRunner) run(ctx context.Context, job *jobs.Context) error {
	stored, err := r.driver.store.GetInstance(ctx, r.instance.ID)
	if err != nil {
		return fmt.Errorf("读取实例失败: %w", err)
	}
	if stored.State != storage.InstanceStateRunning {
		return &CommandError{Code: "server_not_running", Message: "服务器已不再运行，人物删除未开始"}
	}
	r.instance = makeRegistryInstanceFromStorage(stored)
	if current := strings.TrimSpace(GetActiveSaveName(r.instance.DataDir)); current != r.expectedSave {
		return &CommandError{Code: "active_save_changed", Message: "当前激活存档已变化，人物删除未开始"}
	}

	_, _ = job.Info(ctx, "正在核对当前存档、人物身份与在线状态。")
	farmhand, onlineHumans, err := r.preflight(ctx)
	if err != nil {
		return err
	}
	if r.expectedName != "" && !strings.EqualFold(r.expectedName, farmhand.Name) {
		_, _ = job.Warn(ctx, "人物显示名已变化，将继续按联机 ID 精确删除。")
	}
	preDeleteNoticeSent := false
	if onlineHumans > 0 {
		if err := r.broadcastPreDelete(ctx); err != nil {
			return fmt.Errorf("发送删除前游戏内通告失败: %w", err)
		}
		preDeleteNoticeSent = true
		_, _ = job.Info(ctx, fmt.Sprintf("已向 %d 名在线真人玩家发送删除前通告。", onlineHumans))
	}

	_, _ = job.Info(ctx, "正在保存删除前的最新游戏进度。")
	if err := r.saveAndWait(ctx); err != nil {
		return &CommandError{Code: "predelete_save_failed", Message: "删除前游戏保存未确认，未执行人物删除：" + err.Error()}
	}
	backupPath, err := BackupPreFarmhandDelete(r.instance.DataDir, r.expectedSave)
	if err != nil {
		return &CommandError{Code: "predelete_backup_failed", Message: "创建人物删除保护备份失败，未执行删除：" + err.Error()}
	}
	backupName := filepath.Base(backupPath)
	_, _ = job.Info(ctx, "已创建整档保护备份："+backupName)

	// Recheck immediately before the destructive call. Junimo performs its own
	// game-thread online/save checks as the final authority.
	_, onlineHumans, err = r.preflight(ctx)
	if err != nil {
		return err
	}
	// A player can join while the pre-delete save and backup are running. Make
	// sure that case gets the warning too, without sending duplicates to players
	// who were already notified at the first preflight.
	if onlineHumans > 0 && !preDeleteNoticeSent {
		if err := r.broadcastPreDelete(ctx); err != nil {
			return fmt.Errorf("发送删除前游戏内通告失败: %w", err)
		}
		_, _ = job.Info(ctx, fmt.Sprintf("已向 %d 名刚上线的真人玩家发送删除前通告。", onlineHumans))
	}
	_, _ = job.Info(ctx, "正在通过 Junimo 删除离线人物及其小屋。")
	if err := r.deleteViaJunimo(ctx); err != nil {
		return err
	}
	if err := r.verifyRuntimeAbsent(ctx); err != nil {
		return &CommandError{Code: "farmhand_delete_result_unconfirmed", Message: err.Error()}
	}

	_, _ = job.Info(ctx, "人物已从运行世界移除，正在保存删除结果。")
	if err := r.saveAndWait(ctx); err != nil {
		return &CommandError{Code: "farmhand_delete_persistence_unconfirmed", Message: "人物已从运行世界删除，但最终保存未确认；保护备份为 " + backupName + "：" + err.Error()}
	}
	if err := verifyFarmhandAbsentOnDisk(r.instance.DataDir, r.expectedSave, r.playerID); err != nil {
		return &CommandError{Code: "farmhand_delete_verification_failed", Message: "最终存档验证失败；保护备份为 " + backupName + "：" + err.Error()}
	}

	stableID, _, _ := resolveStableSaveIdentity(r.instance.DataDir, r.expectedSave)
	if store, ok := r.driver.store.(interface {
		MarkPlayerCharacterDeleted(context.Context, string, string, string, string, string) error
	}); ok {
		if err := store.MarkPlayerCharacterDeleted(ctx, r.instance.ID, stableID, r.playerID, r.operationID, time.Now().UTC().Format(time.RFC3339Nano)); err != nil && !errors.Is(err, storage.ErrNotFound) {
			return fmt.Errorf("更新面板人物名册失败: %w", err)
		}
	}
	// Refresh the audience after the final save so players who joined during the
	// deletion also receive the reconnect notice.
	if players, err := r.driver.ListPlayers(ctx, r.instance); err == nil {
		if current, countErr := countOtherOnlineHumans(players.Players, r.playerID); countErr == nil {
			onlineHumans = current
		}
	}
	if onlineHumans > 0 {
		message := "离线存档人物已删除并保存完成。如仍看到旧小屋或位置异常，请重新连接服务器刷新世界状态。"
		if err := r.broadcastAndWait(ctx, message); err != nil {
			_, _ = job.Warn(ctx, "删除已完成，但删除后游戏内通告发送失败："+err.Error())
		} else {
			_, _ = job.Info(ctx, "已向在线玩家发送删除完成通告。")
		}
	}
	_, _ = job.Info(ctx, fmt.Sprintf("人物 %s 已删除并持久化；保护备份：%s。", farmhand.Name, backupName))
	return nil
}

func (r *farmhandDeleteRunner) broadcastPreDelete(ctx context.Context) error {
	return r.broadcastAndWait(ctx, "管理员即将删除一个离线存档人物及其小屋。操作完成后如仍看到旧小屋或位置异常，请重新连接服务器。")
}

func (r *farmhandDeleteRunner) broadcastAndWait(ctx context.Context, message string) error {
	commandID, err := writePanelBroadcastCommand(r.instance.DataDir, message)
	if err != nil {
		return err
	}
	// Older compatible control mods don't publish result files. In that case,
	// writing the command is the strongest available acknowledgement.
	if !commandResultSupported(r.instance.DataDir) {
		return nil
	}
	deadline := time.NewTimer(20 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		outcome, err := r.driver.importCommandOutcome(ctx, r.instance.ID, r.instance.DataDir, commandID)
		if err != nil {
			return err
		}
		switch outcome.Status {
		case CommandStatusSucceeded, CommandStatusDispatched:
			return nil
		case CommandStatusFailed, CommandStatusExpired:
			if strings.TrimSpace(outcome.Message) != "" {
				return errors.New(outcome.Message)
			}
			return fmt.Errorf("广播命令状态为 %s", outcome.Status)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return errors.New("等待游戏内广播确认超时")
		case <-ticker.C:
		}
	}
}

func makeRegistryInstanceFromStorage(instance storage.Instance) registry.Instance {
	return registry.Instance{ID: instance.ID, DriverID: instance.DriverID, DataDir: instance.DataDir, State: instance.State, DriverPayload: instance.DriverPayload}
}

func (r *farmhandDeleteRunner) preflight(ctx context.Context) (junimoFarmhand, int, error) {
	farmhands, err := readJunimoFarmhands(ctx, r.lifecycle, r.instance.DataDir)
	if err != nil {
		return junimoFarmhand{}, 0, err
	}
	var target *junimoFarmhand
	for i := range farmhands.Farmhands {
		if farmhands.Farmhands[i].ID == r.playerIDInt {
			target = &farmhands.Farmhands[i]
			break
		}
	}
	if target == nil || !target.IsCustomized {
		return junimoFarmhand{}, 0, &CommandError{Code: "farmhand_not_found", Message: "当前存档中未找到该已认领人物"}
	}
	players, err := r.driver.ListPlayers(ctx, r.instance)
	if err != nil {
		return junimoFarmhand{}, 0, &CommandError{Code: "world_not_ready", Message: "无法确认当前在线玩家状态：" + err.Error()}
	}
	onlineHumans, err := countOtherOnlineHumans(players.Players, r.playerID)
	if err != nil {
		return junimoFarmhand{}, 0, err
	}
	return *target, onlineHumans, nil
}

func countOtherOnlineHumans(players []PlayerInfo, targetPlayerID string) (int, error) {
	onlineHumans := 0
	for _, player := range players {
		if player.IsHost || player.Status != "online" {
			continue
		}
		if player.UniqueMultiplayerID == targetPlayerID {
			return 0, &CommandError{Code: "farmhand_online", Message: "被删除的人物当前在线，请先让该玩家退出服务器"}
		}
		onlineHumans++
	}
	return onlineHumans, nil
}

func (r *farmhandDeleteRunner) saveAndWait(ctx context.Context) error {
	result, err := requestSaveNow(r.instance)
	if err != nil {
		return err
	}
	_, err = waitForDurableSaveOutcome(ctx, r.instance.DataDir, result.CommandID, importDurableSaveOptions{
		CommandTimeout: 3 * time.Minute,
		PollInterval:   250 * time.Millisecond,
		GetOutcome: func(dataDir, commandID string) (CommandOutcome, error) {
			return r.driver.importCommandOutcome(ctx, r.instance.ID, dataDir, commandID)
		},
	})
	return err
}

func readJunimoFarmhands(ctx context.Context, exec commandExecutor, dataDir string) (junimoFarmhandsResponse, error) {
	raw, err := readJunimoAPI(ctx, exec, dataDir, "/farmhands")
	if err != nil {
		return junimoFarmhandsResponse{}, &CommandError{Code: "junimo_api_unavailable", Message: "Junimo 人物接口尚未就绪"}
	}
	var response junimoFarmhandsResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		return response, &CommandError{Code: "farmhand_delete_unsupported", Message: "Junimo 人物接口返回了无法识别的数据"}
	}
	return response, nil
}

func (r *farmhandDeleteRunner) deleteViaJunimo(ctx context.Context) error {
	apiPort, apiKey, err := readJunimoAPIConfig(r.instance.DataDir)
	if err != nil {
		return err
	}
	requestURL := "http://localhost:" + apiPort + "/farmhands?playerId=" + url.QueryEscape(r.playerID)
	args := []string{"curl", "-sS", "--max-time", "20", "-w", "\n%{http_code}", "-X", "DELETE"}
	if apiKey != "" {
		args = append(args, "-H", "Authorization: Bearer "+apiKey)
	}
	args = append(args, requestURL)
	reqCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	result, execErr := r.lifecycle.ComposeExecPipe(reqCtx, r.instance.DataDir, "server", "", args...)
	if execErr != nil || result.ExitCode != 0 {
		return &CommandError{Code: "junimo_api_unavailable", Message: "调用 Junimo 人物删除接口失败"}
	}
	body, status, err := splitCurlResponse(result)
	if err != nil || status != "200" {
		return &CommandError{Code: "farmhand_delete_failed", Message: "Junimo 人物删除接口返回异常状态"}
	}
	var response junimoFarmhandDeleteResponse
	if err := json.Unmarshal([]byte(body), &response); err != nil {
		return &CommandError{Code: "farmhand_delete_result_unconfirmed", Message: "无法解析 Junimo 人物删除结果"}
	}
	if response.Success {
		return nil
	}
	lower := strings.ToLower(response.Error)
	switch {
	case strings.Contains(lower, "currently online"):
		return &CommandError{Code: "farmhand_online", Message: "被删除的人物已重新上线，删除未执行"}
	case strings.Contains(lower, "save is in progress"):
		return &CommandError{Code: "save_in_progress", Message: "游戏正在保存，请稍后重试"}
	case strings.Contains(lower, "not found"):
		return &CommandError{Code: "farmhand_not_found", Message: "Junimo 未找到该存档人物"}
	case strings.Contains(lower, "server not ready"):
		return &CommandError{Code: "world_not_ready", Message: "游戏世界尚未准备完成"}
	default:
		return &CommandError{Code: "farmhand_delete_failed", Message: "Junimo 拒绝删除人物：" + strings.TrimSpace(response.Error)}
	}
}

func splitCurlResponse(result paneldocker.CommandResult) (string, string, error) {
	output := strings.TrimRight(result.Stdout, "\r\n")
	idx := strings.LastIndex(output, "\n")
	if idx < 0 {
		return "", "", fmt.Errorf("missing HTTP status")
	}
	return strings.TrimSpace(output[:idx]), strings.TrimSpace(output[idx+1:]), nil
}

func (r *farmhandDeleteRunner) verifyRuntimeAbsent(ctx context.Context) error {
	farmhands, err := readJunimoFarmhands(ctx, r.lifecycle, r.instance.DataDir)
	if err != nil {
		return err
	}
	for _, farmhand := range farmhands.Farmhands {
		if farmhand.ID == r.playerIDInt && farmhand.IsCustomized {
			return fmt.Errorf("Junimo 仍返回被删除的人物")
		}
	}
	return nil
}

func verifyFarmhandAbsentOnDisk(dataDir, saveName, playerID string) error {
	saveDir := filepath.Join(savesDir(dataDir), "Saves", saveName)
	raw, err := os.ReadFile(filepath.Join(saveDir, saveName))
	if err != nil {
		return err
	}
	var parsed saveRosterXML
	if err := xml.Unmarshal(raw, &parsed); err != nil || parsed.XMLName.Local != "SaveGame" {
		return fmt.Errorf("主存档 XML 无法解析")
	}
	for _, farmer := range parsed.Farmhands {
		id := strings.TrimSpace(farmer.UniqueMultiplayerID)
		if id == "" {
			id = strings.TrimSpace(farmer.UniqueMultiplayerIDFallback)
		}
		if id == playerID {
			return fmt.Errorf("磁盘主存档仍包含被删除人物")
		}
	}
	return nil
}
