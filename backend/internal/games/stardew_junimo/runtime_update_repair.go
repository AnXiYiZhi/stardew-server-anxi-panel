package stardew_junimo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

const runtimeUpdateRepairAttemptLimit = 3

// StartRuntimeUpdateRepair retries only the exact persisted rollback for a
// rollback_failed transaction. It never accepts an image, path, apply ID, or
// recovery strategy from the caller.
func (d *Driver) StartRuntimeUpdateRepair(ctx context.Context, instance registry.Instance, createdBy int64) (RuntimeUpdateApplyStatus, error) {
	if d.jobs == nil || d.store == nil {
		return RuntimeUpdateApplyStatus{}, errors.New("runtime update repair service is not configured")
	}
	if instance.DriverID != DriverID {
		return RuntimeUpdateApplyStatus{}, &RuntimeUpdateValidationError{Code: "unsupported/driver", Message: "实例不是 stardew_junimo driver。"}
	}
	runtimeDocker, ok := d.docker.(RuntimeUpdateApplyDockerService)
	if !ok {
		return RuntimeUpdateApplyStatus{}, &RuntimeUpdateValidationError{Code: "unsupported/docker_contract", Message: "当前 Docker driver 不支持 Junimo 恢复。"}
	}

	d.runtimeUpdateMu.Lock()
	defer d.runtimeUpdateMu.Unlock()
	status, err := readRuntimeUpdateApplyStatus(instance.DataDir)
	if err != nil || status.Phase != RuntimeUpdateApplyRollbackFailed {
		return RuntimeUpdateApplyStatus{}, &RuntimeUpdateValidationError{Code: "runtime_repair_not_needed", Message: "当前没有可安全重试的 Junimo 升级回滚。"}
	}
	if status.RepairAttempts >= runtimeUpdateRepairAttemptLimit {
		return RuntimeUpdateApplyStatus{}, &RuntimeUpdateValidationError{Code: "runtime_repair_exhausted", Message: "同一恢复事务已连续失败 3 次；已停止自动操作并保留全部恢复材料。"}
	}
	manifest, err := readRuntimeUpdateRecoveryManifest(instance.DataDir, status.ApplyID)
	if err != nil || !validRuntimeUpdateRecoveryManifest(instance, status, manifest) {
		return RuntimeUpdateApplyStatus{}, &RuntimeUpdateValidationError{Code: "recovery_state_uncertain", Message: "恢复清单缺失、损坏或与当前实例不匹配；已拒绝猜测性修复。"}
	}
	if err := validateRuntimeUpdateRecoveryMaterials(instance.DataDir, manifest); err != nil {
		d.logger.Warn("runtime update repair materials rejected", "instance", instance.ID, "apply", status.ApplyID, "error", paneldocker.RedactString(err.Error()))
		return RuntimeUpdateApplyStatus{}, &RuntimeUpdateValidationError{Code: "recovery_material_invalid", Message: "私有恢复材料不完整或校验失败；已拒绝修改实例。"}
	}
	// The status file reaches rollback_failed immediately before the jobs table
	// records the prior runner's terminal state. Wait briefly for that exact job
	// only, so a user's first click cannot lose a harmless completion race.
	slotDeadline := time.Now().Add(3 * time.Second)
	for {
		active, activeErr := d.jobs.Active(ctx, storage.ListActiveJobsFilter{TargetType: "instance", TargetID: instance.ID})
		if activeErr != nil {
			return RuntimeUpdateApplyStatus{}, fmt.Errorf("list conflicting jobs: %w", activeErr)
		}
		if len(active) == 0 {
			break
		}
		if len(active) != 1 || active[0].ID != status.JobID || time.Now().After(slotDeadline) {
			return RuntimeUpdateApplyStatus{}, &RuntimeUpdateValidationError{Code: "runtime_update_busy", Message: "实例存在进行中的任务，暂不能修复升级。"}
		}
		select {
		case <-ctx.Done():
			return RuntimeUpdateApplyStatus{}, ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}

	status.CreatedBy = createdBy
	status.RepairAttempts++
	status.Phase, status.Progress = RuntimeUpdateApplyRollingBack, 90
	status.RollbackCode, status.RollbackError, status.ManualAction, status.FinishedAt = "", "", "", ""
	status.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	status.Logs = append(status.Logs, RuntimeUpdateDryRunLog{At: status.UpdatedAt, Level: "warning", Message: fmt.Sprintf("正在执行第 %d/%d 次幂等安全恢复；只使用已校验的原版本材料。", status.RepairAttempts, runtimeUpdateRepairAttemptLimit)})
	gate := make(chan struct{})
	var initialWriteErr error
	job, err := d.jobs.Start(ctx, jobs.Spec{Type: RuntimeUpdateApplyJobType, DisplayName: "一键修复 Junimo 升级", TargetType: "instance", TargetID: instance.ID, CreatedBy: createdBy, Timeout: 2 * time.Hour, Run: func(runCtx context.Context, jobCtx *jobs.Context) error {
		<-gate
		if initialWriteErr != nil {
			return errors.New("修复状态初始化失败")
		}
		return d.runRuntimeUpdateRepair(runCtx, jobCtx, runtimeDocker, instance, status, manifest)
	}})
	if err != nil {
		return RuntimeUpdateApplyStatus{}, fmt.Errorf("start runtime update repair job: %w", err)
	}
	status.JobID = job.ID
	initialWriteErr = writeRuntimeUpdateApplyStatus(instance.DataDir, status)
	close(gate)
	if initialWriteErr != nil {
		return RuntimeUpdateApplyStatus{}, initialWriteErr
	}
	return status, nil
}

func (d *Driver) runRuntimeUpdateRepair(ctx context.Context, job *jobs.Context, docker RuntimeUpdateApplyDockerService, instance registry.Instance, status RuntimeUpdateApplyStatus, manifest runtimeUpdateRecoveryManifest) error {
	_, _ = job.Warn(ctx, "正在按私有恢复清单重试原版本回滚；不会选择或下载新的目标版本。")
	repairErr := d.performRuntimeUpdateRollback(ctx, job, docker, instance, manifest)
	now := time.Now().UTC().Format(time.RFC3339)
	status.Progress, status.UpdatedAt, status.FinishedAt = 100, now, now
	if repairErr != nil {
		status.Phase, status.ErrorCode = RuntimeUpdateApplyRollbackFailed, "rollback_failed"
		status.Error = "一键安全恢复未能完成；私有恢复材料继续保留。"
		status.RollbackCode, status.RollbackError = runtimeUpdateRollbackFailure(repairErr)
		if status.RepairAttempts >= runtimeUpdateRepairAttemptLimit {
			status.ManualAction = "同一恢复事务已连续失败 3 次；请保留恢复材料和支持包，停止继续重试。"
		} else {
			status.ManualAction = "可在排除 Docker、磁盘或文件锁故障后再次点击“一键安全恢复”；修复只会重试同一份原版本回滚。"
		}
		status.Logs = append(status.Logs, RuntimeUpdateDryRunLog{At: now, Level: "error", Message: status.Error})
		_ = writeRuntimeUpdateApplyStatus(instance.DataDir, status)
		d.auditRuntimeUpdateTerminal(ctx, instance.ID, status)
		return errors.New("Junimo 升级安全恢复未完成")
	}
	status.Phase = RuntimeUpdateApplyFailedRolledBack
	if status.CauseCode != "" {
		status.ErrorCode, status.Error = status.CauseCode, status.CauseError
	} else {
		status.ErrorCode, status.Error = "repaired_rollback", "上次升级失败，原运行组件和运行状态已恢复。"
	}
	status.RollbackCode, status.RollbackError, status.ManualAction = "", "", ""
	status.ServerRunning = manifest.ServerWasRunning
	status.Logs = append(status.Logs, RuntimeUpdateDryRunLog{At: now, Level: "warning", Message: "一键安全恢复已验证原 server/auth、认证卷、Mod、配置与运行状态。"})
	if err := writeRuntimeUpdateApplyStatus(instance.DataDir, status); err != nil {
		return err
	}
	d.auditRuntimeUpdateTerminal(ctx, instance.ID, status)
	if runtimeUpdateAuthSnapshotVolumeCreated(manifest) {
		_ = docker.RuntimeRemoveSnapshotVolume(ctx, instance.DataDir, manifest.Project, manifest.SnapshotVolume)
	}
	_ = os.RemoveAll(runtimeUpdateRecoveryDir(instance.DataDir, manifest.ApplyID))
	return nil
}

func validateRuntimeUpdateRecoveryMaterials(dataDir string, manifest runtimeUpdateRecoveryManifest) error {
	recoveryDir := runtimeUpdateRecoveryDir(dataDir, manifest.ApplyID)
	info, err := os.Lstat(recoveryDir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("recovery directory is missing or unsafe")
	}
	checks := []struct {
		name string
		hash string
	}{
		{name: "original.env", hash: manifest.OriginalEnvSHA256},
		{name: "original-compose.yml", hash: manifest.OriginalComposeSHA256},
	}
	if manifest.ControlManifestPresent {
		checks = append(checks, struct {
			name string
			hash string
		}{name: "original-control-manifest.json", hash: manifest.OriginalControlJSONSHA})
	}
	if manifest.ControlDLLPresent {
		checks = append(checks, struct {
			name string
			hash string
		}{name: "original-control-StardewAnxiPanel.Control.dll", hash: manifest.OriginalControlDLLSHA})
	}
	for _, check := range checks {
		actual, hashErr := runtimeRecoveryFileSHA256(filepath.Join(recoveryDir, check.name))
		if hashErr != nil {
			return fmt.Errorf("validate %s: %w", check.name, hashErr)
		}
		if manifest.SchemaVersion >= 3 && (len(check.hash) != 64 || !strings.EqualFold(actual, check.hash)) {
			return fmt.Errorf("validate %s: sha256 mismatch", check.name)
		}
	}
	return nil
}
