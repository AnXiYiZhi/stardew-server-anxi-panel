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
	Mode         string
	ActorID      int64
}

type farmhandDeletePayload struct {
	OperationID  string `json:"operationId"`
	PlayerID     string `json:"uniqueMultiplayerId"`
	ExpectedName string `json:"expectedName,omitempty"`
	ExpectedSave string `json:"expectedSaveId"`
	Mode         string `json:"mode"`
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
	mode         string
}

// DeleteFarmhand records or starts a guarded deletion of one offline farmhand
// from the currently loaded save. The destructive step never runs while a
// human player is connected.
func (d *Driver) DeleteFarmhand(ctx context.Context, req FarmhandDeleteRequest) (*FarmhandDeleteSubmission, error) {
	if err := rejectUnfinishedNewGameOwner(req.Instance.DataDir); err != nil {
		return nil, err
	}
	if d.jobs == nil || d.store == nil {
		return nil, &CommandError{Code: "not_supported", Message: "人物删除任务服务未配置"}
	}
	if req.Instance.State != storage.InstanceStateRunning {
		return nil, &CommandError{Code: "server_not_running", Message: "服务器未运行，无法删除存档人物"}
	}
	playerID := strings.TrimSpace(req.PlayerID)
	_, err := parseFarmhandDeletePlayerID(playerID)
	if err != nil {
		return nil, err
	}
	mode := strings.TrimSpace(req.Mode)
	if mode == "" {
		mode = storage.FarmhandDeleteModeWait
	}
	if mode != storage.FarmhandDeleteModeWait && mode != storage.FarmhandDeleteModeMaintenanceNow {
		return nil, &CommandError{Code: "invalid_delete_mode", Message: "人物删除方式无效"}
	}
	expectedSave := strings.TrimSpace(req.ExpectedSave)
	activeSave := strings.TrimSpace(GetActiveSaveName(req.Instance.DataDir))
	if expectedSave == "" || activeSave == "" || expectedSave != activeSave {
		return nil, &CommandError{Code: "active_save_changed", Message: "当前激活存档已变化，请刷新后重试"}
	}
	if err := ensureNoPendingSaveCommand(req.Instance.DataDir); err != nil {
		return nil, err
	}
	_, ok := d.docker.(LifecycleDockerService)
	if !ok {
		return nil, &CommandError{Code: "not_supported", Message: "Docker 服务不支持 Junimo API 调用"}
	}
	// Serialize owner validation, the active-job check, and job creation with
	// new-game preclaim. Without this shared guard, both operations could observe
	// an empty owner/job set and then mutate the same save concurrently.
	d.runtimeUpdateMu.Lock()
	defer d.runtimeUpdateMu.Unlock()
	if err := rejectUnfinishedNewGameOwner(req.Instance.DataDir); err != nil {
		return nil, err
	}
	active, err := d.jobs.Active(ctx, storage.ListActiveJobsFilter{TargetType: "instance", TargetID: req.Instance.ID})
	if err != nil {
		return nil, fmt.Errorf("读取活动任务失败: %w", err)
	}
	if len(active) > 0 {
		return nil, &CommandError{Code: "operation_in_progress", Message: "当前有其他服务器任务正在执行，请等待完成后重试"}
	}
	intentStore, ok := d.store.(farmhandDeleteIntentStore)
	if !ok {
		return nil, &CommandError{Code: "not_supported", Message: "人物删除等待服务未配置"}
	}
	operationID, err := randomHex(16)
	if err != nil {
		return nil, err
	}
	status := storage.FarmhandDeleteIntentLaunching
	if mode == storage.FarmhandDeleteModeWait {
		status = storage.FarmhandDeleteIntentWaiting
	}
	intent, err := intentStore.CreateFarmhandDeleteIntent(ctx, storage.CreateFarmhandDeleteIntentParams{
		InstanceID: req.Instance.ID, OperationID: operationID, Mode: mode, Status: status,
		PlayerID: playerID, ExpectedName: strings.TrimSpace(req.ExpectedName), ExpectedSaveID: expectedSave,
		CreatedBy: req.ActorID, ExpiresAt: time.Now().UTC().Add(farmhandDeleteIntentLifetime).Format(time.RFC3339Nano),
	})
	if err != nil {
		if errors.Is(err, storage.ErrConflict) {
			return nil, &CommandError{Code: "farmhand_delete_in_progress", Message: "该世界已有等待中、执行中或待恢复的人物删除操作"}
		}
		return nil, fmt.Errorf("保存人物删除请求失败: %w", err)
	}
	resultIntent := makeFarmhandDeleteIntentResult(intent)
	if mode == storage.FarmhandDeleteModeWait {
		return &FarmhandDeleteSubmission{Status: storage.FarmhandDeleteIntentWaiting, Intent: &resultIntent}, nil
	}
	job, err := d.startFarmhandDeleteJob(ctx, req.Instance, intent)
	if err != nil {
		_ = intentStore.UpdateFarmhandDeleteIntent(context.Background(), req.Instance.ID, operationID, storage.FarmhandDeleteIntentFailed, "", err.Error())
		return nil, fmt.Errorf("创建人物删除任务失败: %w", err)
	}
	resultIntent.Status = storage.FarmhandDeleteIntentActive
	if mode == storage.FarmhandDeleteModeMaintenanceNow {
		resultIntent.Status = storage.FarmhandDeleteIntentCountdown
	}
	resultIntent.JobID = job.ID
	return &FarmhandDeleteSubmission{Status: resultIntent.Status, JobID: job.ID, Intent: &resultIntent}, nil
}

func (d *Driver) startFarmhandDeleteJob(ctx context.Context, instance registry.Instance, intent storage.FarmhandDeleteIntent) (*registry.Job, error) {
	lifecycle, ok := d.docker.(LifecycleDockerService)
	if !ok {
		return nil, &CommandError{Code: "not_supported", Message: "Docker 服务不支持 Junimo API 调用"}
	}
	playerIDInt, err := parseFarmhandDeletePlayerID(intent.PlayerID)
	if err != nil {
		return nil, err
	}
	payload := farmhandDeletePayload{
		OperationID: intent.OperationID, PlayerID: intent.PlayerID, ExpectedName: intent.ExpectedName,
		ExpectedSave: intent.ExpectedSaveID, Mode: intent.Mode,
	}
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	runner := &farmhandDeleteRunner{
		driver: d, lifecycle: lifecycle, instance: instance, playerID: intent.PlayerID, playerIDInt: playerIDInt,
		expectedName: intent.ExpectedName, expectedSave: intent.ExpectedSaveID, operationID: intent.OperationID, mode: intent.Mode,
	}
	intentStore := d.store.(farmhandDeleteIntentStore)
	job, err := d.jobs.Start(ctx, jobs.Spec{
		Type: FarmhandDeleteJobType, DisplayName: "删除离线存档人物", TargetType: "instance",
		TargetID: instance.ID, CreatedBy: intent.CreatedBy.Int64, Payload: string(rawPayload), Timeout: farmhandDeleteTimeout,
		Exclusive: true,
		BeforeRun: func(prepareCtx context.Context, job storage.Job) error {
			return intentStore.BindFarmhandDeleteIntentJob(prepareCtx, instance.ID, intent.OperationID, job.ID)
		},
		Run: runner.run,
	})
	if err != nil {
		return nil, err
	}
	return &registry.Job{ID: job.ID}, nil
}

func (r *farmhandDeleteRunner) run(ctx context.Context, job *jobs.Context) (retErr error) {
	intentStore := r.driver.store.(farmhandDeleteIntentStore)
	backupName := ""
	maintenanceOwned := false
	destructiveBoundaryCrossed := false
	maintenanceReleaseFailed := false
	defer func() {
		if maintenanceOwned && (!destructiveBoundaryCrossed || retErr == nil) {
			releaseCtx, cancel := context.WithTimeout(context.Background(), farmhandDeleteCommandTimeout)
			releaseErr := r.endMaintenance(releaseCtx)
			cancel()
			if releaseErr != nil {
				maintenanceReleaseFailed = true
				retErr = errors.Join(retErr, fmt.Errorf("恢复世界联机入口失败: %w", releaseErr))
			}
		}
		status := storage.FarmhandDeleteIntentCompleted
		lastError := ""
		if retErr != nil {
			lastError = retErr.Error()
			status = storage.FarmhandDeleteIntentFailed
			if destructiveBoundaryCrossed {
				status = storage.FarmhandDeleteIntentRecoveryRequired
			} else if maintenanceReleaseFailed {
				// Seal may have persisted before its acknowledgement was lost.
				// Let the scheduler reconcile the marker before allowing another request.
				status = storage.FarmhandDeleteIntentActive
			} else {
				currentCtx, currentCancel := context.WithTimeout(context.Background(), 5*time.Second)
				currentIntent, currentErr := intentStore.GetFarmhandDeleteIntent(currentCtx, r.instance.ID)
				currentCancel()
				if currentErr == nil && currentIntent.OperationID == r.operationID && currentIntent.Status == storage.FarmhandDeleteIntentCanceled {
					status = storage.FarmhandDeleteIntentCanceled
					lastError = currentIntent.LastError
				} else if isFarmhandDeleteCancellation(retErr) {
					status = storage.FarmhandDeleteIntentCanceled
				} else {
					var commandErr *CommandError
					if r.mode == storage.FarmhandDeleteModeWait && errors.As(retErr, &commandErr) && commandErr.Code == "players_connected" {
						status = storage.FarmhandDeleteIntentWaiting
					}
				}
			}
		}
		updateCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		updateErr := intentStore.UpdateFarmhandDeleteIntent(updateCtx, r.instance.ID, r.operationID, status, backupName, lastError)
		cancel()
		if updateErr != nil && retErr == nil {
			retErr = fmt.Errorf("更新人物删除状态失败: %w", updateErr)
		}
	}()

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
	if err := ensureNoPendingSaveCommand(r.instance.DataDir); err != nil {
		return err
	}
	farmhand, connectedHumans, err := r.preflight(ctx)
	if err != nil {
		return err
	}
	if r.expectedName != "" && !strings.EqualFold(r.expectedName, farmhand.Name) {
		_, _ = job.Warn(ctx, "人物显示名已变化，将继续按联机 ID 精确删除。")
	}
	if r.mode == storage.FarmhandDeleteModeWait && connectedHumans != 0 {
		return &CommandError{Code: "players_connected", Message: "等待删除启动时又检测到玩家连接，人物删除未开始"}
	}
	if r.mode == storage.FarmhandDeleteModeMaintenanceNow {
		maintenanceOwned = true
		if err := r.runCountdown(ctx, job); err != nil {
			return err
		}
		if err := intentStore.ActivateFarmhandDeleteIntent(ctx, r.instance.ID, r.operationID); err != nil {
			if errors.Is(err, storage.ErrConflict) {
				return &CommandError{Code: "farmhand_delete_canceled", Message: "人物删除已在倒计时结束前取消"}
			}
			return fmt.Errorf("进入人物删除维护阶段失败: %w", err)
		}
	}
	maintenanceOwned = true
	if err := r.beginMaintenance(ctx); err != nil {
		return err
	}
	_, _ = job.Info(ctx, "世界已暂时关闭联机入口，正在确认真人连接已全部断开。")
	if err := r.waitForNoConnectedHumans(ctx); err != nil {
		return err
	}
	if err := ensureNoPendingSaveCommand(r.instance.DataDir); err != nil {
		return err
	}
	_, connectedHumans, err = r.preflight(ctx)
	if err != nil {
		return err
	}
	if connectedHumans != 0 {
		return &CommandError{Code: "players_connected", Message: "维护期间仍检测到真人玩家连接，人物删除未开始"}
	}
	if err := r.checkMaintenance(ctx); err != nil {
		return err
	}

	_, _ = job.Info(ctx, "正在保存删除前的最新游戏进度。")
	if err := r.saveAndWait(ctx); err != nil {
		return &CommandError{Code: "predelete_save_failed", Message: "删除前游戏保存未确认，未执行人物删除：" + err.Error()}
	}
	if err := r.checkMaintenance(ctx); err != nil {
		return err
	}
	backupPath, err := BackupPreFarmhandDelete(r.instance.DataDir, r.expectedSave)
	if err != nil {
		return &CommandError{Code: "predelete_backup_failed", Message: "创建人物删除保护备份失败，未执行删除：" + err.Error()}
	}
	backupName = filepath.Base(backupPath)
	if err := intentStore.UpdateFarmhandDeleteIntent(ctx, r.instance.ID, r.operationID, storage.FarmhandDeleteIntentActive, backupName, ""); err != nil {
		return fmt.Errorf("记录人物删除保护备份失败，未执行删除: %w", err)
	}
	_, _ = job.Info(ctx, "已创建整档保护备份："+backupName)

	// Recheck immediately before the destructive call. Junimo performs its own
	// game-thread online/save checks as the final authority.
	_, connectedHumans, err = r.preflight(ctx)
	if err != nil {
		return err
	}
	if connectedHumans != 0 {
		return &CommandError{Code: "players_connected", Message: "保护备份完成后检测到真人玩家连接，人物删除未执行"}
	}
	if err := r.sealMaintenance(ctx); err != nil {
		return err
	}
	destructiveBoundaryCrossed = true
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

	if err := r.markRosterDeleted(ctx); err != nil {
		return err
	}
	_, _ = job.Info(ctx, fmt.Sprintf("人物 %s 已删除并持久化；保护备份：%s。", farmhand.Name, backupName))
	return nil
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
	connected, err := r.connectedHumansFromSnapshot(players)
	if err != nil {
		return junimoFarmhand{}, 0, err
	}
	return *target, len(connected), nil
}

func (r *farmhandDeleteRunner) saveAndWait(ctx context.Context) error {
	commandID, err := r.requestTargetedSave()
	if err != nil {
		return err
	}
	_, err = waitForDurableSaveOutcome(ctx, r.instance.DataDir, commandID, importDurableSaveOptions{
		CommandTimeout: 3 * time.Minute,
		PollInterval:   250 * time.Millisecond,
		GetOutcome: func(dataDir, commandID string) (CommandOutcome, error) {
			return r.driver.importCommandOutcome(ctx, r.instance.ID, dataDir, commandID)
		},
	})
	return err
}

func (r *farmhandDeleteRunner) requestTargetedSave() (string, error) {
	if r.instance.State != storage.InstanceStateRunning {
		return "", &CommandError{Code: "server_not_running", Message: "服务器未运行，无法请求游戏内保存"}
	}
	commandID, err := r.writeExpiringCommand("save-now", map[string]string{
		"transactionId": r.operationID,
		"saveId":        r.expectedSave,
	}, 2*time.Minute)
	if err != nil {
		return "", fmt.Errorf("写入人物删除保存命令失败: %w", err)
	}
	return commandID, nil
}

func isFarmhandDeleteCancellation(err error) bool {
	var commandErr *CommandError
	if !errors.As(err, &commandErr) {
		return false
	}
	switch commandErr.Code {
	case "farmhand_delete_canceled", "sleep_in_progress", "day_transition_in_progress", "active_save_changed", "farmhand_online":
		return true
	default:
		return false
	}
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
	_, apiKey, err := readJunimoAPIConfig(r.instance.DataDir)
	if err != nil {
		return err
	}
	requestURL := "http://localhost:" + junimoContainerAPIPort + "/farmhands?playerId=" + url.QueryEscape(r.playerID)
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
	present, err := farmhandPresentOnDisk(dataDir, saveName, playerID)
	if err != nil {
		return err
	}
	if present {
		return fmt.Errorf("磁盘主存档仍包含被删除人物")
	}
	return nil
}

func farmhandPresentOnDisk(dataDir, saveName, playerID string) (bool, error) {
	saveDir := filepath.Join(savesDir(dataDir), "Saves", saveName)
	raw, err := os.ReadFile(filepath.Join(saveDir, saveName))
	if err != nil {
		return false, err
	}
	var parsed saveRosterXML
	if err := xml.Unmarshal(raw, &parsed); err != nil || parsed.XMLName.Local != "SaveGame" {
		return false, fmt.Errorf("主存档 XML 无法解析")
	}
	for _, farmer := range parsed.Farmhands {
		id := strings.TrimSpace(farmer.UniqueMultiplayerID)
		if id == "" {
			id = strings.TrimSpace(farmer.UniqueMultiplayerIDFallback)
		}
		if id == playerID {
			return true, nil
		}
	}
	return false, nil
}
