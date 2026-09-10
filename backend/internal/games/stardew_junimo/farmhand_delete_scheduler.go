package stardew_junimo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

const (
	farmhandDeleteIntentPollInterval = 2 * time.Second
	farmhandDeleteIntentLifetime     = 24 * time.Hour
	farmhandDeleteLaunchingStaleAge  = 30 * time.Second
)

type farmhandDeleteIntentStore interface {
	CreateFarmhandDeleteIntent(context.Context, storage.CreateFarmhandDeleteIntentParams) (storage.FarmhandDeleteIntent, error)
	GetFarmhandDeleteIntent(context.Context, string) (storage.FarmhandDeleteIntent, error)
	ListActionableFarmhandDeleteIntents(context.Context) ([]storage.FarmhandDeleteIntent, error)
	ClaimFarmhandDeleteIntent(context.Context, string, string) (storage.FarmhandDeleteIntent, error)
	BeginFarmhandDeleteRecovery(context.Context, string, string) (storage.FarmhandDeleteIntent, error)
	BindFarmhandDeleteIntentJob(context.Context, string, string, string) error
	BindFarmhandDeleteRecoveryJob(context.Context, string, string, string) error
	ActivateFarmhandDeleteIntent(context.Context, string, string) error
	UpdateFarmhandDeleteIntent(context.Context, string, string, string, string, string) error
	ReconcileFarmhandDeleteIntent(context.Context, storage.FarmhandDeleteIntent, string, string) error
	ResetLaunchingFarmhandDeleteIntent(context.Context, string, string) error
	CancelWaitingFarmhandDeleteIntent(context.Context, string, string, string) error
	CancelFarmhandDeleteCountdown(context.Context, string, string, string) error
}

type FarmhandDeleteIntentResult struct {
	OperationID    string `json:"operationId"`
	Mode           string `json:"mode"`
	Status         string `json:"status"`
	PlayerID       string `json:"uniqueMultiplayerId"`
	ExpectedName   string `json:"expectedName"`
	ExpectedSaveID string `json:"expectedSaveId"`
	JobID          string `json:"jobId,omitempty"`
	ExpiresAt      string `json:"expiresAt"`
	BackupName     string `json:"backupName,omitempty"`
	LastError      string `json:"lastError,omitempty"`
}

type FarmhandDeleteSubmission struct {
	Status string                      `json:"status"`
	JobID  string                      `json:"jobId,omitempty"`
	Intent *FarmhandDeleteIntentResult `json:"intent,omitempty"`
}

func makeFarmhandDeleteIntentResult(intent storage.FarmhandDeleteIntent) FarmhandDeleteIntentResult {
	return FarmhandDeleteIntentResult{
		OperationID: intent.OperationID, Mode: intent.Mode, Status: intent.Status,
		PlayerID: intent.PlayerID, ExpectedName: intent.ExpectedName, ExpectedSaveID: intent.ExpectedSaveID,
		JobID: intent.JobID.String, ExpiresAt: intent.ExpiresAt, BackupName: intent.BackupName, LastError: intent.LastError,
	}
}

func (d *Driver) GetFarmhandDeleteIntent(ctx context.Context, instance registry.Instance) (*FarmhandDeleteIntentResult, error) {
	store, ok := d.store.(farmhandDeleteIntentStore)
	if !ok {
		return nil, &CommandError{Code: "not_supported", Message: "人物删除等待服务未配置"}
	}
	intent, err := store.GetFarmhandDeleteIntent(ctx, instance.ID)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	result := makeFarmhandDeleteIntentResult(intent)
	return &result, nil
}

func (d *Driver) CancelFarmhandDeleteIntent(ctx context.Context, instance registry.Instance, operationID string) error {
	store, ok := d.store.(farmhandDeleteIntentStore)
	if !ok {
		return &CommandError{Code: "not_supported", Message: "人物删除等待服务未配置"}
	}
	if operationID == "" {
		return &CommandError{Code: "invalid_delete_operation", Message: "缺少人物删除操作 ID"}
	}
	intent, err := store.GetFarmhandDeleteIntent(ctx, instance.ID)
	if errors.Is(err, storage.ErrNotFound) || (err == nil && intent.OperationID != operationID) {
		return &CommandError{Code: "farmhand_delete_not_cancelable", Message: "人物删除状态已变化，不能再取消"}
	}
	if err != nil {
		return err
	}
	if intent.Status == storage.FarmhandDeleteIntentWaiting {
		err = store.CancelWaitingFarmhandDeleteIntent(ctx, instance.ID, operationID, "管理员已取消等待删除。请重新确认后再发起。")
	} else if intent.Status == storage.FarmhandDeleteIntentCountdown {
		err = store.CancelFarmhandDeleteCountdown(ctx, instance.ID, operationID, "管理员在倒计时结束前取消了立即维护删除。")
		if err == nil && intent.JobID.Valid && d.jobs != nil {
			if cancelErr := d.jobs.Cancel(ctx, intent.JobID.String); cancelErr != nil && !errors.Is(cancelErr, storage.ErrNotFound) {
				return fmt.Errorf("取消人物删除倒计时任务失败: %w", cancelErr)
			}
		}
	} else {
		return &CommandError{Code: "farmhand_delete_not_cancelable", Message: "人物删除已进入维护阶段，不能再取消"}
	}
	if err != nil {
		if errors.Is(err, storage.ErrConflict) {
			return &CommandError{Code: "farmhand_delete_not_cancelable", Message: "人物删除已进入维护阶段，不能再取消"}
		}
		return err
	}
	return nil
}

func (d *Driver) RunFarmhandDeleteScheduler(ctx context.Context) {
	if d == nil || d.jobs == nil {
		return
	}
	if _, ok := d.store.(farmhandDeleteIntentStore); !ok {
		return
	}
	ticker := time.NewTicker(farmhandDeleteIntentPollInterval)
	defer ticker.Stop()
	d.processFarmhandDeleteIntents(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.processFarmhandDeleteIntents(ctx)
		}
	}
}

func (d *Driver) processFarmhandDeleteIntents(ctx context.Context) {
	store := d.store.(farmhandDeleteIntentStore)
	intents, err := store.ListActionableFarmhandDeleteIntents(ctx)
	if err != nil {
		d.logger.Warn("list pending farmhand deletions failed", "error", err)
		return
	}
	for _, intent := range intents {
		if err := d.processFarmhandDeleteIntent(ctx, store, intent); err != nil && !errors.Is(err, storage.ErrConflict) {
			d.logger.Warn("process pending farmhand deletion failed", "instance", intent.InstanceID, "operation", intent.OperationID, "error", err)
		}
	}
}

func (d *Driver) processFarmhandDeleteIntent(ctx context.Context, store farmhandDeleteIntentStore, intent storage.FarmhandDeleteIntent) error {
	now := time.Now().UTC()
	if intent.Status == storage.FarmhandDeleteIntentRecoveryRequired {
		return d.reconcileFarmhandDeleteRecovery(ctx, store, intent)
	}
	if intent.Status == storage.FarmhandDeleteIntentCountdown || intent.Status == storage.FarmhandDeleteIntentActive {
		return d.reconcileInterruptedFarmhandDeleteIntent(ctx, store, intent)
	}
	if intent.Status == storage.FarmhandDeleteIntentLaunching {
		updatedAt, parseErr := time.Parse(time.RFC3339Nano, intent.UpdatedAt)
		if parseErr == nil && now.Sub(updatedAt) < farmhandDeleteLaunchingStaleAge {
			return nil
		}
		instance, err := d.store.GetInstance(ctx, intent.InstanceID)
		if err != nil {
			return err
		}
		_, markerErr := readFarmhandDeleteMaintenanceMarker(instance.DataDir)
		if intent.Mode == storage.FarmhandDeleteModeMaintenanceNow || intent.BackupName != "" || !errors.Is(markerErr, os.ErrNotExist) {
			return d.reconcileInterruptedFarmhandDeleteIntent(ctx, store, intent)
		}
		return store.ResetLaunchingFarmhandDeleteIntent(ctx, intent.InstanceID, intent.OperationID)
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, intent.ExpiresAt)
	if err != nil || !now.Before(expiresAt) {
		return store.ReconcileFarmhandDeleteIntent(ctx, intent, storage.FarmhandDeleteIntentExpired, "等待删除已超过 24 小时，请重新确认。")
	}
	instance, err := d.store.GetInstance(ctx, intent.InstanceID)
	if err != nil {
		return err
	}
	if GetActiveSaveName(instance.DataDir) != intent.ExpectedSaveID {
		return store.CancelWaitingFarmhandDeleteIntent(ctx, intent.InstanceID, intent.OperationID, "活动存档已变化，等待删除已取消。")
	}
	if instance.State != storage.InstanceStateRunning {
		return nil
	}
	registryInstance := makeRegistryInstanceFromStorage(instance)
	runner := &farmhandDeleteRunner{
		driver: d, instance: registryInstance, playerID: intent.PlayerID, expectedName: intent.ExpectedName,
		expectedSave: intent.ExpectedSaveID, operationID: intent.OperationID, mode: storage.FarmhandDeleteModeWait,
	}
	parsedID, err := parseFarmhandDeletePlayerID(intent.PlayerID)
	if err != nil {
		return store.CancelWaitingFarmhandDeleteIntent(ctx, intent.InstanceID, intent.OperationID, err.Error())
	}
	runner.playerIDInt = parsedID
	lifecycle, ok := d.docker.(LifecycleDockerService)
	if !ok {
		return nil
	}
	runner.lifecycle = lifecycle
	_, connected, err := runner.preflight(ctx)
	if err != nil {
		var commandErr *CommandError
		if errors.As(err, &commandErr) && (commandErr.Code == "farmhand_online" || commandErr.Code == "farmhand_not_found") {
			return store.CancelWaitingFarmhandDeleteIntent(ctx, intent.InstanceID, intent.OperationID, commandErr.Message+"，等待删除已取消。")
		}
		return nil
	}
	if connected != 0 {
		return nil
	}
	d.runtimeUpdateMu.Lock()
	defer d.runtimeUpdateMu.Unlock()
	active, err := d.jobs.Active(ctx, storage.ListActiveJobsFilter{TargetType: "instance", TargetID: intent.InstanceID})
	if err != nil || len(active) > 0 {
		return err
	}
	claimed, err := store.ClaimFarmhandDeleteIntent(ctx, intent.InstanceID, intent.OperationID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) || errors.Is(err, storage.ErrConflict) {
			return nil
		}
		return err
	}
	_, err = d.startFarmhandDeleteJob(ctx, registryInstance, claimed)
	if err != nil {
		if resetErr := store.ResetLaunchingFarmhandDeleteIntent(context.Background(), intent.InstanceID, intent.OperationID); resetErr != nil {
			return fmt.Errorf("start pending delete: %v; reset intent: %w", err, resetErr)
		}
	}
	return err
}

func (d *Driver) reconcileInterruptedFarmhandDeleteIntent(ctx context.Context, store farmhandDeleteIntentStore, intent storage.FarmhandDeleteIntent) error {
	d.runtimeUpdateMu.Lock()
	defer d.runtimeUpdateMu.Unlock()
	current, err := store.GetFarmhandDeleteIntent(ctx, intent.InstanceID)
	if err != nil {
		return err
	}
	if current.OperationID != intent.OperationID || current.Status != intent.Status || current.JobID != intent.JobID || current.UpdatedAt != intent.UpdatedAt {
		return nil
	}
	active, err := d.jobs.Active(ctx, storage.ListActiveJobsFilter{
		TargetType: "instance", TargetID: intent.InstanceID, Types: []string{FarmhandDeleteJobType},
	})
	if err != nil {
		return err
	}
	for _, job := range active {
		if intent.JobID.Valid && job.ID == intent.JobID.String {
			return nil
		}
	}
	if len(active) > 0 {
		return fmt.Errorf("a different farmhand deletion job is active for instance %s", intent.InstanceID)
	}

	instance, err := d.store.GetInstance(ctx, intent.InstanceID)
	if err != nil {
		return err
	}
	marker, markerErr := readFarmhandDeleteMaintenanceMarker(instance.DataDir)
	if errors.Is(markerErr, os.ErrNotExist) {
		present, presentErr := farmhandPresentOnDisk(instance.DataDir, intent.ExpectedSaveID, intent.PlayerID)
		if presentErr != nil {
			return presentErr
		}
		if present {
			return store.ReconcileFarmhandDeleteIntent(ctx, intent,
				storage.FarmhandDeleteIntentCanceled, "Panel 重启中断了人物删除；目标人物仍在存档中。")
		}
		return store.ReconcileFarmhandDeleteIntent(ctx, intent,
			storage.FarmhandDeleteIntentRecoveryRequired, "Panel 重启后目标人物已不在存档中，但维护标记缺失；需要人工核对保护备份。")
	}
	if markerErr != nil {
		return markerErr
	}
	if marker.OperationID != intent.OperationID || marker.ExpectedSaveID != intent.ExpectedSaveID || marker.TargetPlayerID != intent.PlayerID {
		return fmt.Errorf("farmhand deletion maintenance marker does not match operation %s", intent.OperationID)
	}
	if marker.Phase == "destructive" {
		return store.ReconcileFarmhandDeleteIntent(ctx, intent,
			storage.FarmhandDeleteIntentRecoveryRequired, "Panel 在人物删除后、最终保存验证前重启；联机入口继续保持关闭。")
	}
	if marker.Phase != "countdown" && marker.Phase != "guarded" && marker.Phase != "canceled" {
		return fmt.Errorf("unsupported farmhand deletion maintenance phase %q", marker.Phase)
	}
	if instance.State != storage.InstanceStateRunning {
		return nil
	}
	runner, err := newFarmhandDeleteRunner(d, makeRegistryInstanceFromStorage(instance), intent)
	if err != nil {
		return err
	}
	releaseCtx, cancel := context.WithTimeout(ctx, farmhandDeleteCommandTimeout)
	err = runner.endMaintenance(releaseCtx)
	cancel()
	if err != nil {
		return err
	}
	return store.ReconcileFarmhandDeleteIntent(ctx, intent,
		storage.FarmhandDeleteIntentCanceled, "Panel 重启中断了人物删除；维护状态已解除，目标人物未被继续删除。")
}

func (d *Driver) reconcileFarmhandDeleteRecovery(ctx context.Context, store farmhandDeleteIntentStore, intent storage.FarmhandDeleteIntent) error {
	instance, err := d.store.GetInstance(ctx, intent.InstanceID)
	if err != nil {
		return err
	}
	present, err := farmhandPresentOnDisk(instance.DataDir, intent.ExpectedSaveID, intent.PlayerID)
	if err != nil || !present {
		return err
	}
	_, markerErr := readFarmhandDeleteMaintenanceMarker(instance.DataDir)
	if markerErr == nil {
		return nil
	}
	if !errors.Is(markerErr, os.ErrNotExist) {
		return markerErr
	}
	return store.ReconcileFarmhandDeleteIntent(ctx, intent, storage.FarmhandDeleteIntentCanceled, "保护备份已恢复，目标人物重新出现；删除恢复状态已解除。")
}

func parseFarmhandDeletePlayerID(playerID string) (int64, error) {
	parsed, err := strconv.ParseInt(strings.TrimSpace(playerID), 10, 64)
	if err != nil || parsed == 0 {
		return 0, &CommandError{Code: "invalid_player", Message: "玩家联机 ID 无效"}
	}
	return parsed, nil
}

// Cancel on the selection event as well as on polling, so selecting another
// save and immediately selecting this one again cannot revive an old request.
func (d *Driver) CancelFarmhandDeleteForSaveChange(ctx context.Context, instance registry.Instance, saveName string) error {
	store, ok := d.store.(farmhandDeleteIntentStore)
	if !ok {
		return nil
	}
	intent, err := store.GetFarmhandDeleteIntent(ctx, instance.ID)
	if errors.Is(err, storage.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if intent.Status == storage.FarmhandDeleteIntentRecoveryRequired && intent.ExpectedSaveID != saveName {
		return &CommandError{Code: "farmhand_delete_recovery_required", Message: "请先处理人物删除恢复，再切换其他存档"}
	}
	if intent.Status != storage.FarmhandDeleteIntentWaiting || intent.ExpectedSaveID == saveName {
		return nil
	}
	return store.CancelWaitingFarmhandDeleteIntent(ctx, instance.ID, intent.OperationID, "管理员选择了其他存档，等待删除已取消。")
}
