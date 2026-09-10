package stardew_junimo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func (d *Driver) RetryFarmhandDeletePersistence(ctx context.Context, instance registry.Instance, operationID string, actorID int64) (*FarmhandDeleteSubmission, error) {
	store, ok := d.store.(farmhandDeleteIntentStore)
	if !ok || d.jobs == nil {
		return nil, &CommandError{Code: "not_supported", Message: "人物删除恢复服务未配置"}
	}
	operationID = strings.TrimSpace(operationID)
	intent, err := store.GetFarmhandDeleteIntent(ctx, instance.ID)
	if err != nil {
		return nil, err
	}
	if intent.OperationID != operationID || intent.Status != storage.FarmhandDeleteIntentRecoveryRequired {
		return nil, &CommandError{Code: "farmhand_delete_recovery_not_available", Message: "当前没有可重试的人物删除恢复操作"}
	}
	if instance.State != storage.InstanceStateRunning {
		return nil, &CommandError{Code: "server_not_running", Message: "服务器运行后才能重试人物删除的最终保存"}
	}
	if err := ensureNoPendingSaveCommand(instance.DataDir); err != nil {
		return nil, err
	}

	d.runtimeUpdateMu.Lock()
	defer d.runtimeUpdateMu.Unlock()
	active, err := d.jobs.Active(ctx, storage.ListActiveJobsFilter{TargetType: "instance", TargetID: instance.ID})
	if err != nil {
		return nil, err
	}
	if len(active) > 0 {
		return nil, &CommandError{Code: "operation_in_progress", Message: "当前有其他服务器任务正在执行，请等待完成后重试"}
	}
	intent, err = store.BeginFarmhandDeleteRecovery(ctx, instance.ID, operationID)
	if err != nil {
		return nil, &CommandError{Code: "farmhand_delete_recovery_not_available", Message: "人物删除恢复状态已变化，请刷新后重试"}
	}
	runner, err := newFarmhandDeleteRunner(d, instance, intent)
	if err != nil {
		_ = store.UpdateFarmhandDeleteIntent(context.Background(), instance.ID, operationID, storage.FarmhandDeleteIntentRecoveryRequired, intent.BackupName, err.Error())
		return nil, err
	}
	job, err := d.jobs.Start(ctx, jobs.Spec{
		Type: FarmhandDeleteJobType, DisplayName: "恢复人物删除最终保存", TargetType: "instance", TargetID: instance.ID,
		CreatedBy: actorID, Payload: fmt.Sprintf(`{"operationId":%q,"mode":"recovery_retry"}`, operationID),
		Timeout: farmhandDeleteTimeout, Exclusive: true,
		BeforeRun: func(prepareCtx context.Context, job storage.Job) error {
			return store.BindFarmhandDeleteRecoveryJob(prepareCtx, instance.ID, operationID, job.ID)
		},
		Run: func(runCtx context.Context, jobContext *jobs.Context) error {
			return runner.runRecovery(runCtx, jobContext, intent.BackupName)
		},
	})
	if err != nil {
		_ = store.UpdateFarmhandDeleteIntent(context.Background(), instance.ID, operationID, storage.FarmhandDeleteIntentRecoveryRequired, intent.BackupName, err.Error())
		return nil, err
	}
	resultIntent := makeFarmhandDeleteIntentResult(intent)
	resultIntent.Status = storage.FarmhandDeleteIntentActive
	resultIntent.JobID = job.ID
	return &FarmhandDeleteSubmission{Status: storage.FarmhandDeleteIntentActive, JobID: job.ID, Intent: &resultIntent}, nil
}

func newFarmhandDeleteRunner(d *Driver, instance registry.Instance, intent storage.FarmhandDeleteIntent) (*farmhandDeleteRunner, error) {
	lifecycle, ok := d.docker.(LifecycleDockerService)
	if !ok {
		return nil, &CommandError{Code: "not_supported", Message: "Docker 服务不支持 Junimo API 调用"}
	}
	playerID, err := parseFarmhandDeletePlayerID(intent.PlayerID)
	if err != nil {
		return nil, err
	}
	return &farmhandDeleteRunner{
		driver: d, lifecycle: lifecycle, instance: instance, playerID: intent.PlayerID, playerIDInt: playerID,
		expectedName: intent.ExpectedName, expectedSave: intent.ExpectedSaveID, operationID: intent.OperationID, mode: intent.Mode,
	}, nil
}

func (r *farmhandDeleteRunner) runRecovery(ctx context.Context, job *jobs.Context, backupName string) (retErr error) {
	store := r.driver.store.(farmhandDeleteIntentStore)
	defer func() {
		status := storage.FarmhandDeleteIntentCompleted
		lastError := ""
		if retErr != nil {
			status = storage.FarmhandDeleteIntentRecoveryRequired
			lastError = retErr.Error()
		}
		updateCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		updateErr := store.UpdateFarmhandDeleteIntent(updateCtx, r.instance.ID, r.operationID, status, backupName, lastError)
		cancel()
		if updateErr != nil && retErr == nil {
			retErr = fmt.Errorf("更新人物删除恢复状态失败: %w", updateErr)
		}
	}()

	stored, err := r.driver.store.GetInstance(ctx, r.instance.ID)
	if err != nil {
		return err
	}
	r.instance = makeRegistryInstanceFromStorage(stored)
	if stored.State != storage.InstanceStateRunning || GetActiveSaveName(stored.DataDir) != r.expectedSave {
		return &CommandError{Code: "farmhand_delete_recovery_not_ready", Message: "目标世界或活动存档尚未准备好，联机入口继续保持关闭"}
	}
	marker, err := readFarmhandDeleteMaintenanceMarker(stored.DataDir)
	markerMissing := errors.Is(err, os.ErrNotExist)
	if !markerMissing && (err != nil || marker.OperationID != r.operationID || marker.ExpectedSaveID != r.expectedSave || marker.Phase != "destructive") {
		return &CommandError{Code: "farmhand_delete_maintenance_invalid", Message: "无法确认该删除操作仍持有维护联机门禁"}
	}
	if err := r.verifyRuntimeAbsent(ctx); err != nil {
		return &CommandError{Code: "farmhand_delete_runtime_restored", Message: "运行世界中已重新出现目标人物，请恢复保护备份或重新发起删除"}
	}
	if markerMissing {
		if err := verifyFarmhandAbsentOnDisk(stored.DataDir, r.expectedSave, r.playerID); err != nil {
			return err
		}
		_, _ = job.Info(ctx, "维护已解除，运行世界与磁盘均确认人物已删除，正在修正任务结果。")
		return r.markRosterDeleted(ctx)
	}
	_, _ = job.Info(ctx, "正在重试已删除人物后的最终游戏保存。")
	if err := r.saveAndWait(ctx); err != nil {
		return &CommandError{Code: "farmhand_delete_persistence_unconfirmed", Message: "最终保存仍未确认；联机入口继续保持关闭：" + err.Error()}
	}
	if err := verifyFarmhandAbsentOnDisk(stored.DataDir, r.expectedSave, r.playerID); err != nil {
		return &CommandError{Code: "farmhand_delete_verification_failed", Message: "最终存档验证仍未通过；联机入口继续保持关闭：" + err.Error()}
	}
	if err := r.markRosterDeleted(ctx); err != nil {
		return err
	}
	if err := r.endMaintenance(ctx); err != nil {
		return &CommandError{Code: "farmhand_delete_maintenance_release_failed", Message: "人物删除已持久化，但恢复世界联机入口失败：" + err.Error()}
	}
	_, _ = job.Info(ctx, "最终保存与磁盘验证成功，世界联机入口已恢复。")
	return nil
}

func (r *farmhandDeleteRunner) markRosterDeleted(ctx context.Context) error {
	stableID, _, _ := resolveStableSaveIdentity(r.instance.DataDir, r.expectedSave)
	store, ok := r.driver.store.(interface {
		MarkPlayerCharacterDeleted(context.Context, string, string, string, string, string) error
	})
	if !ok {
		return nil
	}
	if err := store.MarkPlayerCharacterDeleted(ctx, r.instance.ID, stableID, r.playerID, r.operationID, time.Now().UTC().Format(time.RFC3339Nano)); err != nil && !errors.Is(err, storage.ErrNotFound) {
		return fmt.Errorf("更新面板人物名册失败: %w", err)
	}
	return nil
}

func (d *Driver) ValidateFarmhandDeleteBackupRestore(ctx context.Context, instance registry.Instance, backupName string) error {
	if _, ok := d.store.(farmhandDeleteIntentStore); !ok {
		return nil
	}
	intent, err := d.GetFarmhandDeleteIntent(ctx, instance)
	if err != nil {
		return err
	}
	if intent != nil && intent.Status == storage.FarmhandDeleteIntentRecoveryRequired && (intent.BackupName == "" || intent.BackupName != backupName) {
		return &CommandError{Code: "farmhand_delete_recovery_backup_mismatch", Message: "备份与当前删除操作不匹配，请选择本次删除生成的保护备份"}
	}
	return nil
}

// Only an explicit, completed restore of this operation's protection backup
// can clear a destructive marker while the game is stopped.
func (d *Driver) CompleteFarmhandDeleteBackupRestore(ctx context.Context, instance registry.Instance, backupName, saveName string) error {
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
	if intent.Status != storage.FarmhandDeleteIntentRecoveryRequired {
		return nil
	}
	if intent.BackupName != backupName || intent.ExpectedSaveID != saveName {
		return &CommandError{Code: "farmhand_delete_recovery_backup_mismatch", Message: "人物删除维护尚未解除，请恢复该操作对应的保护备份"}
	}
	present, err := farmhandPresentOnDisk(instance.DataDir, saveName, intent.PlayerID)
	if err != nil {
		return err
	}
	if !present {
		return &CommandError{Code: "farmhand_delete_restore_unconfirmed", Message: "保护备份中尚未确认目标人物已恢复"}
	}
	marker, err := readFarmhandDeleteMaintenanceMarker(instance.DataDir)
	if !errors.Is(err, os.ErrNotExist) {
		if err != nil {
			return err
		}
		if marker.OperationID != intent.OperationID || marker.ExpectedSaveID != saveName || marker.TargetPlayerID != intent.PlayerID {
			return &CommandError{Code: "farmhand_delete_maintenance_invalid", Message: "人物删除维护标记与保护备份不匹配"}
		}
		if err := os.Remove(filepath.Join(controlDir(instance.DataDir), "farmhand-delete-maintenance.json")); err != nil {
			return err
		}
	}
	err = store.ReconcileFarmhandDeleteIntent(ctx, intent, storage.FarmhandDeleteIntentCanceled, "管理员已恢复本次删除的保护备份，人物删除维护已解除。")
	if errors.Is(err, storage.ErrConflict) {
		current, readErr := store.GetFarmhandDeleteIntent(ctx, instance.ID)
		if readErr == nil && current.OperationID == intent.OperationID && current.Status == storage.FarmhandDeleteIntentCanceled {
			return nil
		}
	}
	return err
}
