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
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

const runtimeUpdateRepairAttemptLimit = 3

// StartRuntimeUpdateRepair diagnoses and repairs only driver-owned known
// failure states: either an exact rollback_failed transaction or a closed
// legacy-config repair plan. It then reruns the full dry-run and starts a fresh
// apply transaction. It never accepts an image, path, apply ID, or strategy
// from the caller.
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
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return RuntimeUpdateApplyStatus{}, &RuntimeUpdateValidationError{Code: "recovery_state_uncertain", Message: "现有升级状态文件损坏或无法读取；已拒绝覆盖现场。"}
	}
	if err != nil || status.Phase != RuntimeUpdateApplyRollbackFailed {
		inspection := InspectManagedRuntimeStack(instance.DataDir, instance.State)
		if !inspection.Repairable {
			return RuntimeUpdateApplyStatus{}, &RuntimeUpdateValidationError{Code: "runtime_repair_not_needed", Message: "当前没有已知且可证明安全的 Junimo 升级修复方案。"}
		}
		return d.startKnownRuntimeConfigRepair(ctx, runtimeDocker, instance, createdBy, status, inspection)
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
	status.Checks = append(status.Checks,
		RuntimeUpdateDryRunCheck{Name: "repair_failure_state", Status: "ok", Message: "已锁定同一实例的 rollback_failed 事务、首次失败步骤与回滚失败步骤。"},
		RuntimeUpdateDryRunCheck{Name: "repair_manifest", Status: "ok", Message: "恢复清单的实例、apply ID、版本对、Compose project 与事务私有资源名称一致。"},
		RuntimeUpdateDryRunCheck{Name: "repair_materials", Status: "ok", Message: "原 .env、Compose 与 Control 备份均为普通文件且 SHA-256 与清单一致。"},
	)
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
	job, err := d.jobs.Start(ctx, jobs.Spec{Type: RuntimeUpdateApplyJobType, DisplayName: "检测、修复并继续 Junimo 升级", TargetType: "instance", TargetID: instance.ID, CreatedBy: createdBy, Timeout: 2 * time.Hour, Run: func(runCtx context.Context, jobCtx *jobs.Context) error {
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

func (d *Driver) startKnownRuntimeConfigRepair(ctx context.Context, docker RuntimeUpdateApplyDockerService, instance registry.Instance, createdBy int64, previous RuntimeUpdateApplyStatus, inspection sjconfig.RuntimeStackInspection) (RuntimeUpdateApplyStatus, error) {
	if previous.RepairAttempts >= runtimeUpdateRepairAttemptLimit {
		return RuntimeUpdateApplyStatus{}, &RuntimeUpdateValidationError{Code: "runtime_repair_exhausted", Message: "同一实例的自动修复已连续尝试 3 次；已停止自动操作。"}
	}
	active, err := d.jobs.Active(ctx, storage.ListActiveJobsFilter{TargetType: "instance", TargetID: instance.ID, Types: []string{"stardew_install", "stardew_lifecycle", RuntimeUpdateDryRunJobType, RuntimeUpdateApplyJobType, SMAPIUpdateDryRunJobType, SMAPIUpdateApplyJobType}})
	if err != nil {
		return RuntimeUpdateApplyStatus{}, fmt.Errorf("list conflicting jobs: %w", err)
	}
	if len(active) > 0 {
		return RuntimeUpdateApplyStatus{}, &RuntimeUpdateValidationError{Code: "runtime_update_busy", Message: "实例存在安装、生命周期或组件升级任务，请等待任务结束。"}
	}
	now := time.Now().UTC().Format(time.RFC3339)
	sourceID := newRuntimeApplyID()
	status := RuntimeUpdateApplyStatus{
		CreatedBy: createdBy, ApplyID: sourceID, Phase: RuntimeUpdateApplyResumingUpgrade, Progress: 5,
		Current: inspection.Current, Target: inspection.Recommended,
		Checks: []RuntimeUpdateDryRunCheck{
			{Name: "known_issue_detection", Status: "ok", Message: "已将当前错误匹配到 Panel 内置的闭集诊断规则。"},
			{Name: "known_legacy_config", Status: "warning", Message: inspection.RepairReason},
		},
		Warnings: []string{}, Logs: []RuntimeUpdateDryRunLog{{At: now, Level: "warning", Message: "检测到可信旧版候选配置；将先私有备份并原子修复，复检通过后执行完整预检与升级。"}},
		ServerWasRunning: instance.State == storage.InstanceStateRunning || instance.State == storage.InstanceStateStarting,
		ServerRunning:    instance.State == storage.InstanceStateRunning || instance.State == storage.InstanceStateStarting,
		RepairAttempts:   previous.RepairAttempts + 1, RepairSourceApplyID: sourceID, ResumeAfterRepair: true,
		StartedAt: now, UpdatedAt: now,
	}
	gate := make(chan struct{})
	var initialWriteErr error
	job, err := d.jobs.Start(ctx, jobs.Spec{Type: RuntimeUpdateApplyJobType, DisplayName: "检测、修复并继续 Junimo 升级", TargetType: "instance", TargetID: instance.ID, CreatedBy: createdBy, Timeout: 2 * time.Hour, Run: func(runCtx context.Context, jobCtx *jobs.Context) error {
		<-gate
		if initialWriteErr != nil {
			return errors.New("修复状态初始化失败")
		}
		return d.retryRuntimeUpdateAfterRepair(runCtx, jobCtx, docker, instance, status)
	}})
	if err != nil {
		return RuntimeUpdateApplyStatus{}, fmt.Errorf("start known runtime repair job: %w", err)
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
	_, _ = job.Warn(ctx, "已完成只读故障识别与恢复材料校验；正在按私有清单幂等恢复原版本。")
	repairErr := d.performRuntimeUpdateRollback(ctx, job, docker, instance, manifest)
	now := time.Now().UTC().Format(time.RFC3339)
	if repairErr != nil {
		status.Progress, status.UpdatedAt, status.FinishedAt = 100, now, now
		status.Phase, status.ErrorCode = RuntimeUpdateApplyRollbackFailed, "rollback_failed"
		status.Error = "检测已完成，但已知恢复流程未能通过验收；私有恢复材料继续保留。"
		status.RollbackCode, status.RollbackError = runtimeUpdateRollbackFailure(repairErr)
		if status.RepairAttempts >= runtimeUpdateRepairAttemptLimit {
			status.ManualAction = "同一恢复事务已连续失败 3 次；请保留恢复材料和支持包，停止继续重试。"
		} else {
			status.ManualAction = "可在排除 Docker、磁盘或文件锁故障后再次点击“检测、修复并升级”；系统仍只使用同一份已校验材料。"
		}
		status.Logs = append(status.Logs, RuntimeUpdateDryRunLog{At: now, Level: "error", Message: status.Error})
		_ = writeRuntimeUpdateApplyStatus(instance.DataDir, status)
		d.auditRuntimeUpdateTerminal(ctx, instance.ID, status)
		return errors.New("Junimo 升级安全恢复未完成")
	}
	status.Phase, status.Progress = RuntimeUpdateApplyResumingUpgrade, 8
	status.ErrorCode, status.Error, status.RollbackCode, status.RollbackError, status.ManualAction, status.FinishedAt = "", "", "", "", "", ""
	status.ServerRunning = manifest.ServerWasRunning
	status.ResumeAfterRepair = true
	if status.RepairSourceApplyID == "" {
		status.RepairSourceApplyID = manifest.ApplyID
	}
	status.UpdatedAt = now
	status.Checks = append(status.Checks, RuntimeUpdateDryRunCheck{Name: "repair_original_runtime", Status: "ok", Message: "原 server/auth image ID、认证卷、Mod、配置、控制契约与升级前运行状态已恢复并通过验收。"})
	status.Logs = append(status.Logs, RuntimeUpdateDryRunLog{At: now, Level: "warning", Message: "原版本恢复已通过验收；开始重新检测已知旧配置并执行完整升级预检。"})
	if err := writeRuntimeUpdateApplyStatus(instance.DataDir, status); err != nil {
		return err
	}
	if runtimeUpdateAuthSnapshotVolumeCreated(manifest) {
		if err := docker.RuntimeRemoveSnapshotVolume(ctx, instance.DataDir, manifest.Project, manifest.SnapshotVolume); err != nil {
			return d.failRetryableRuntimeUpdateRepair(ctx, instance, status, "repair_cleanup_failed", "原版本已恢复，但无法清理本事务的认证快照；已停止重新升级并保留恢复目录。")
		}
	}
	if err := os.RemoveAll(runtimeUpdateRecoveryDir(instance.DataDir, manifest.ApplyID)); err != nil {
		return d.failRetryableRuntimeUpdateRepair(ctx, instance, status, "repair_cleanup_failed", "原版本已恢复，但无法清理本事务的恢复目录；已停止重新升级。")
	}
	return d.retryRuntimeUpdateAfterRepair(ctx, job, docker, instance, status)
}

func (d *Driver) retryRuntimeUpdateAfterRepair(ctx context.Context, job *jobs.Context, docker RuntimeUpdateApplyDockerService, instance registry.Instance, status RuntimeUpdateApplyStatus) error {
	stored, err := d.store.GetInstance(ctx, instance.ID)
	if err != nil {
		return d.failRuntimeUpdateRepair(ctx, instance, status, "repair_instance_reload_failed", "无法读取修复后的实例状态；旧运行组件保持可用，未继续升级。")
	}
	instance.State, instance.DriverPhase, instance.DriverPayload = stored.State, stored.DriverPhase, stored.DriverPayload
	inspection := InspectManagedRuntimeStack(instance.DataDir, instance.State)
	if inspection.Repairable {
		status.Checks = append(status.Checks, RuntimeUpdateDryRunCheck{Name: "known_legacy_config", Status: "warning", Message: inspection.RepairReason})
		status.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		_ = writeRuntimeUpdateApplyStatus(instance.DataDir, status)
		result, err := d.repairKnownRuntimeStackConfig(instance)
		if err != nil {
			return d.failRuntimeUpdateRepair(ctx, instance, status, runtimeUpdateErrorCode(err), "检测到已知旧版配置，但原子备份、规范化或复检未能完成；旧运行组件保持可用。")
		}
		inspection = result.RuntimeStackInspection
		status.Checks = append(status.Checks, RuntimeUpdateDryRunCheck{Name: "known_legacy_config_repaired", Status: "ok", Message: "可信旧候选配置已私有备份、原子规范化并复检通过；backupId=" + result.BackupID + "。"})
	}
	if inspection.Status == sjconfig.RuntimeStackStatusUpToDate {
		now := time.Now().UTC().Format(time.RFC3339)
		status.Phase, status.Progress = RuntimeUpdateApplySucceeded, 100
		status.ErrorCode, status.Error, status.ManualAction, status.FinishedAt = "", "", "", now
		status.ResumeAfterRepair, status.ServerRunning, status.UpdatedAt = false, status.ServerWasRunning, now
		status.Checks = append(status.Checks, RuntimeUpdateDryRunCheck{Name: "repair_upgrade_result", Status: "ok", Message: "修复后实时检查确认运行组件已经是当前推荐版本，无需重复变更。"})
		if err := writeRuntimeUpdateApplyStatus(instance.DataDir, status); err != nil {
			return err
		}
		d.auditRuntimeUpdateTerminal(ctx, instance.ID, status)
		d.reconcileRequiredRuntimeRepair(ctx, instance, status)
		return nil
	}
	if inspection.Status != sjconfig.RuntimeStackStatusUpdateAvailable {
		return d.failRuntimeUpdateRepair(ctx, instance, status, inspection.Code, "恢复后检测到的运行组件状态不属于已知可安全修复范围；旧运行组件保持可用，未猜测性修改。")
	}

	now := time.Now().UTC().Format(time.RFC3339)
	dryRun := RuntimeUpdateDryRunStatus{DryRunID: newRuntimeDryRunID(), JobID: job.ID, Phase: RuntimeUpdatePhaseStarting, Current: inspection.Current, Target: inspection.Recommended, Checks: []RuntimeUpdateDryRunCheck{}, Warnings: []string{}, Logs: []RuntimeUpdateDryRunLog{}, ServerRunning: status.ServerWasRunning, StartedAt: now, UpdatedAt: now}
	if err := writeRuntimeUpdateDryRunStatus(instance.DataDir, dryRun); err != nil {
		return d.failRuntimeUpdateRepair(ctx, instance, status, "repair_preflight_status_failed", "无法持久化修复后的升级预检状态；旧运行组件保持可用。")
	}
	_, _ = job.Info(ctx, "正在执行与普通升级相同的完整预检：实例、Docker/Compose、当前 digest、认证卷、目标镜像与 Compose 覆盖配置。")
	if err := d.runRuntimeUpdateDryRun(ctx, job, docker, instance, dryRun); err != nil {
		finalDryRun, readErr := readRuntimeUpdateDryRunStatus(instance.DataDir)
		code, message := "repair_preflight_failed", "修复后的完整升级预检失败；旧运行组件保持可用，未开始新的升级事务。"
		if readErr == nil {
			if finalDryRun.ErrorCode != "" {
				code = finalDryRun.ErrorCode
			}
			if finalDryRun.Error != "" {
				message = finalDryRun.Error
			}
		}
		return d.failRuntimeUpdateRepair(ctx, instance, status, code, message)
	}
	finalDryRun, err := readRuntimeUpdateDryRunStatus(instance.DataDir)
	if err != nil || finalDryRun.Phase != RuntimeUpdatePhaseSucceeded || finalDryRun.DryRunID != dryRun.DryRunID {
		return d.failRuntimeUpdateRepair(ctx, instance, status, "repair_preflight_result_invalid", "修复后的升级预检终态无法确认；旧运行组件保持可用。")
	}
	for _, check := range finalDryRun.Checks {
		check.Name = "retry_" + check.Name
		status.Checks = append(status.Checks, check)
	}
	status.Checks = append(status.Checks, RuntimeUpdateDryRunCheck{Name: "repair_upgrade_preflight", Status: "ok", Message: "修复后已重新执行完整升级预检，所有硬门槛通过；现在才开始新的升级事务。"})
	status.Warnings = append(status.Warnings, finalDryRun.Warnings...)
	maintenance := RequiredRuntimeUpdateStatus{ServerWasRunning: finalDryRun.ServerRunning}
	setMaintenancePhase := func(phase, code, message string, terminal bool) error {
		status.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		if message != "" {
			level := "info"
			if code != "" {
				level = "warning"
			}
			status.Logs = append(status.Logs, RuntimeUpdateDryRunLog{At: status.UpdatedAt, Level: level, Message: paneldocker.RedactString(message)})
		}
		return writeRuntimeUpdateApplyStatus(instance.DataDir, status)
	}
	if err := d.prepareRequiredRuntimeMaintenance(ctx, instance, &maintenance, setMaintenancePhase); err != nil {
		return d.failRuntimeUpdateRepair(ctx, instance, status, runtimeUpdateErrorCode(err), "修复后的升级前保存或整档保护备份失败；旧运行组件保持可用，未开始新的升级事务。")
	}
	if maintenance.BackupName != "" {
		status.Checks = append(status.Checks, RuntimeUpdateDryRunCheck{Name: "repair_retry_save_backup", Status: "ok", Message: "重新升级前已完成存档保存确认和整档保护备份。"})
	}

	now = time.Now().UTC().Format(time.RFC3339)
	retry := RuntimeUpdateApplyStatus{
		CreatedBy:           status.CreatedBy,
		ApplyID:             newRuntimeApplyID(),
		JobID:               job.ID,
		Phase:               RuntimeUpdateApplyChecking,
		Current:             finalDryRun.Current,
		Target:              finalDryRun.Target,
		Checks:              status.Checks,
		Warnings:            status.Warnings,
		Logs:                append(status.Logs, RuntimeUpdateDryRunLog{At: now, Level: "info", Message: "已知故障已修复且完整预检通过；开始新的 Junimo 运行组件升级事务。"}),
		ServerWasRunning:    finalDryRun.ServerRunning,
		ServerRunning:       finalDryRun.ServerRunning,
		CauseCode:           status.CauseCode,
		CauseError:          status.CauseError,
		RepairAttempts:      status.RepairAttempts,
		RepairSourceApplyID: status.RepairSourceApplyID,
		ResumeAfterRepair:   true,
		StartedAt:           now,
		UpdatedAt:           now,
	}
	if err := writeRuntimeUpdateApplyStatus(instance.DataDir, retry); err != nil {
		return err
	}
	err = d.runRuntimeUpdateApply(ctx, job, docker, instance, retry, nil)
	if final, readErr := readRuntimeUpdateApplyStatus(instance.DataDir); readErr == nil {
		d.reconcileRequiredRuntimeRepair(ctx, instance, final)
	}
	return err
}

func (d *Driver) failRuntimeUpdateRepair(ctx context.Context, instance registry.Instance, status RuntimeUpdateApplyStatus, code, message string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	status.Phase, status.Progress = RuntimeUpdateApplyFailedRolledBack, 100
	status.ErrorCode, status.Error = code, paneldocker.RedactString(message)
	status.RollbackCode, status.RollbackError, status.FinishedAt = "", "", now
	status.ResumeAfterRepair, status.ServerRunning, status.UpdatedAt = false, status.ServerWasRunning, now
	status.ManualAction = "自动检测未匹配到可证明安全的下一步，或修复后复检未通过；旧运行组件已恢复，请导出支持包。"
	status.Logs = append(status.Logs, RuntimeUpdateDryRunLog{At: now, Level: "error", Message: status.Error})
	if err := writeRuntimeUpdateApplyStatus(instance.DataDir, status); err != nil {
		return err
	}
	d.auditRuntimeUpdateTerminal(ctx, instance.ID, status)
	d.reconcileRequiredRuntimeRepair(ctx, instance, status)
	return errors.New(message)
}

func (d *Driver) failRetryableRuntimeUpdateRepair(ctx context.Context, instance registry.Instance, status RuntimeUpdateApplyStatus, code, message string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	status.Phase, status.Progress = RuntimeUpdateApplyRollbackFailed, 100
	status.ErrorCode, status.Error = "rollback_failed", paneldocker.RedactString(message)
	status.RollbackCode, status.RollbackError = code, paneldocker.RedactString(message)
	status.ResumeAfterRepair, status.FinishedAt, status.UpdatedAt = false, now, now
	if status.RepairAttempts >= runtimeUpdateRepairAttemptLimit {
		status.ManualAction = "同一恢复事务已连续失败 3 次；请保留恢复材料和支持包，停止继续重试。"
	} else {
		status.ManualAction = "实例本身已恢复；排除 Docker 资源清理或文件锁故障后，可再次执行“检测、修复并升级”。"
	}
	status.Logs = append(status.Logs, RuntimeUpdateDryRunLog{At: now, Level: "error", Message: status.Error})
	if err := writeRuntimeUpdateApplyStatus(instance.DataDir, status); err != nil {
		return err
	}
	d.auditRuntimeUpdateTerminal(ctx, instance.ID, status)
	d.reconcileRequiredRuntimeRepair(ctx, instance, status)
	return errors.New(message)
}

func (d *Driver) recoverRuntimeUpdateAfterRepair(ctx context.Context, docker RuntimeUpdateApplyDockerService, instance registry.Instance, status RuntimeUpdateApplyStatus, rebuildRetry bool) error {
	gate := make(chan struct{})
	var initialWriteErr error
	job, err := d.jobs.Start(ctx, jobs.Spec{Type: RuntimeUpdateApplyJobType, DisplayName: "继续修复后的 Junimo 升级", TargetType: "instance", TargetID: instance.ID, CreatedBy: status.CreatedBy, Timeout: 2 * time.Hour, Run: func(runCtx context.Context, jobCtx *jobs.Context) error {
		<-gate
		if initialWriteErr != nil {
			return errors.New("修复后续跑状态初始化失败")
		}
		if rebuildRetry {
			return d.retryRuntimeUpdateAfterRepair(runCtx, jobCtx, docker, instance, status)
		}
		return d.runRuntimeUpdateApply(runCtx, jobCtx, docker, instance, status, nil)
	}})
	if err != nil {
		return err
	}
	status.JobID = job.ID
	status.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	initialWriteErr = writeRuntimeUpdateApplyStatus(instance.DataDir, status)
	close(gate)
	return initialWriteErr
}

func (d *Driver) reconcileRequiredRuntimeRepair(ctx context.Context, instance registry.Instance, apply RuntimeUpdateApplyStatus) {
	manifest, err := sjconfig.BuiltInRuntimeStackManifest()
	if err != nil || manifest.RuntimeUpdatePolicy != sjconfig.RuntimeUpdatePolicyRequired {
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	status, _ := readRequiredRuntimeUpdateStatus(instance.DataDir)
	status.SchemaVersion, status.PanelVersion, status.StackVersion = 1, d.panelVersion, manifest.StackVersion
	status.UpdatedAt, status.FinishedAt = now, now
	switch apply.Phase {
	case RuntimeUpdateApplySucceeded:
		status.Phase, status.ErrorCode, status.Error = requiredRuntimePhaseSucceeded, "", ""
	case RuntimeUpdateApplyRollbackFailed:
		status.Phase, status.ErrorCode, status.Error = requiredRuntimePhaseManual, apply.ErrorCode, apply.Error
	default:
		status.Phase, status.ErrorCode, status.Error = requiredRuntimePhaseFailed, apply.ErrorCode, apply.Error
	}
	_ = writeRequiredRuntimeUpdateStatus(instance.DataDir, status)
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
