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

type RuntimeUpdateRepairPlan struct {
	ActionAvailable bool     `json:"actionAvailable"`
	Action          string   `json:"action"`
	Code            string   `json:"code"`
	Title           string   `json:"title"`
	Detection       string   `json:"detection"`
	Method          string   `json:"method"`
	ButtonLabel     string   `json:"buttonLabel"`
	Steps           []string `json:"steps"`
	Attempts        int      `json:"attempts"`
	MaxAttempts     int      `json:"maxAttempts"`
}

// DetectRuntimeUpdateRepairPlan is the single read-only catalog used by both
// the maintenance UI and StartRuntimeUpdateRepair. Returning a plan does not
// imply that it is safe to mutate the instance: ActionAvailable is true only
// for a closed, driver-owned repair path whose inputs can be revalidated.
func DetectRuntimeUpdateRepairPlan(instance registry.Instance) *RuntimeUpdateRepairPlan {
	status, statusErr := readRuntimeUpdateApplyStatus(instance.DataDir)
	if statusErr != nil && !errors.Is(statusErr, os.ErrNotExist) {
		return manualRuntimeUpdateRepairPlan(
			"recovery_state_uncertain",
			"升级状态文件无法安全读取",
			"检测到持久化升级状态缺失字段、JSON 损坏或文件不可读，无法证明实例处于哪个事务阶段。",
			"保留实例目录和恢复材料，不覆盖状态文件；导出支持包后人工核对。",
		)
	}

	inspection := InspectManagedRuntimeStack(instance.DataDir, instance.State)
	if statusErr == nil && status.Phase == RuntimeUpdateApplyRollbackFailed {
		if status.RepairAttempts >= runtimeUpdateRepairAttemptLimit {
			return exhaustedRuntimeUpdateRepairPlan(status.RepairAttempts)
		}
		manifest, err := readRuntimeUpdateRecoveryManifest(instance.DataDir, status.ApplyID)
		if err != nil || !validRuntimeUpdateRecoveryManifest(instance, status, manifest) {
			return manualRuntimeUpdateRepairPlan(
				"recovery_state_uncertain",
				"恢复事务无法精确匹配",
				"rollback_failed 状态与恢复清单的实例、apply ID、版本对、Compose project 或事务资源名不一致。",
				"保留现场并导出支持包；人工核对事务归属，禁止猜测旧镜像或恢复路径。",
			)
		}
		if err := validateRuntimeUpdateRecoveryMaterials(instance.DataDir, manifest); err != nil {
			return manualRuntimeUpdateRepairPlan(
				"recovery_material_invalid",
				"私有恢复材料校验失败",
				"原 .env、Compose 或 Control 备份缺失、不是普通文件，或 SHA-256 与事务清单不一致。",
				"停止自动修改并保留材料；导出支持包后从可信备份人工恢复。",
			)
		}
		return &RuntimeUpdateRepairPlan{
			ActionAvailable: true,
			Action:          "repair",
			Code:            "repair/rollback_failed",
			Title:           "自动回滚未完成，但原事务材料可验证",
			Detection:       "已精确匹配 rollback_failed 状态、恢复清单、原版本镜像标识和全部私有备份摘要。",
			Method:          "按原事务清单幂等恢复旧版并验收；成功后重新检测配置、执行完整预检，再创建新的升级事务。",
			ButtonLabel:     "修复：恢复旧版后升级",
			Steps: []string{
				"校验事务归属与恢复材料 SHA-256",
				"幂等恢复旧版配置、Mod、认证卷和容器",
				"验收旧版镜像、认证、Control 与运行状态",
				"重新检测配置并执行完整 dry-run",
				"保存与备份通过后创建新的升级事务",
			},
			Attempts:    status.RepairAttempts,
			MaxAttempts: runtimeUpdateRepairAttemptLimit,
		}
	}

	if !runtimeUpdateApplyTerminal(status.Phase) && status.Phase != "" && status.Phase != "idle" {
		return waitRuntimeUpdateRepairPlan()
	}

	attempts := runtimeUpdateRepairAttempts(status, inspection)
	if inspection.Repairable {
		if attempts >= runtimeUpdateRepairAttemptLimit {
			return exhaustedRuntimeUpdateRepairPlan(attempts)
		}
		return &RuntimeUpdateRepairPlan{
			ActionAvailable: true,
			Action:          "repair",
			Code:            inspection.RepairCode,
			Title:           "检测到可证明来源的历史候选配置",
			Detection:       inspection.RepairReason,
			Method:          "私有备份原 .env，仅规范化可信候选镜像列表；复检通过后执行完整预检并继续升级。",
			ButtonLabel:     "修复：规范配置并升级",
			Steps: []string{
				"确认主镜像、主 tag 与 IMAGE_VERSION 一致",
				"确认候选项只来自当前或固定历史可信仓库",
				"私有备份原 .env 并原子写入规范候选列表",
				"重新检查运行栈并执行完整 dry-run",
				"保存与备份通过后创建新的升级事务",
			},
			Attempts:    attempts,
			MaxAttempts: runtimeUpdateRepairAttemptLimit,
		}
	}

	if statusErr == nil && status.Phase == RuntimeUpdateApplyFailedRolledBack &&
		status.Target.StackVersion != "" && status.Target.StackVersion == inspection.Recommended.StackVersion &&
		status.Current.Server.Tag == inspection.Current.Server.Tag && status.Current.SteamAuth.Tag == inspection.Current.SteamAuth.Tag &&
		inspection.Status == sjconfig.RuntimeStackStatusUpdateAvailable {
		if attempts >= runtimeUpdateRepairAttemptLimit {
			return exhaustedRuntimeUpdateRepairPlan(attempts)
		}
		detection, method := safeRetryRuntimeUpdateGuidance(status.ErrorCode)
		return &RuntimeUpdateRepairPlan{
			ActionAvailable: true,
			Action:          "repair",
			Code:            "repair/safe_retry",
			Title:           "上次失败已安全恢复旧版，可重新检测后重试",
			Detection:       detection,
			Method:          method,
			ButtonLabel:     "修复：重新预检并升级",
			Steps: []string{
				"确认上次终态为 failed_rolled_back 且当前仍是同一旧版本对",
				"重新检查 Docker、Compose、磁盘、当前镜像 ID 与目标 digest",
				"重新拉取或复用已验签目标并验证覆盖配置",
				"完成游戏保存与整档保护备份",
				"以新 apply ID 重试；失败时再次自动回滚",
			},
			Attempts:    attempts,
			MaxAttempts: runtimeUpdateRepairAttemptLimit,
		}
	}

	switch inspection.Status {
	case sjconfig.RuntimeStackStatusUpToDate, sjconfig.RuntimeStackStatusUpdateAvailable, sjconfig.RuntimeStackStatusNotInstalled:
		return nil
	case sjconfig.RuntimeStackStatusWithdrawn, sjconfig.RuntimeStackStatusNotRecommended:
		return &RuntimeUpdateRepairPlan{
			Action:      "wait",
			Code:        inspection.Code,
			Title:       "当前推荐矩阵不允许继续升级",
			Detection:   inspection.Reason,
			Method:      "保持当前实例不变，等待 Panel 提供新的已测试兼容矩阵。",
			ButtonLabel: "等待安全版本",
			Steps:       []string{"不修改实例", "等待新的已测试兼容矩阵", "再次检查版本维护页"},
			MaxAttempts: runtimeUpdateRepairAttemptLimit,
		}
	default:
		return manualRuntimeUpdateRepairPlan(
			inspection.Code,
			"当前运行栈无法自动修复",
			inspection.Reason,
			manualRuntimeStackMethod(inspection.Code),
		)
	}
}

func runtimeUpdateRepairAttempts(status RuntimeUpdateApplyStatus, inspection sjconfig.RuntimeStackInspection) int {
	if status.Target.StackVersion == "" || status.Target.StackVersion != inspection.Recommended.StackVersion {
		return 0
	}
	return status.RepairAttempts
}

func manualRuntimeUpdateRepairPlan(code, title, detection, method string) *RuntimeUpdateRepairPlan {
	return &RuntimeUpdateRepairPlan{
		Action:      "export",
		Code:        code,
		Title:       title,
		Detection:   detection,
		Method:      method,
		ButtonLabel: "保留现场并导出支持包",
		Steps:       []string{"停止自动修改", "保留实例与私有恢复材料", "导出脱敏支持包供人工核对"},
		MaxAttempts: runtimeUpdateRepairAttemptLimit,
	}
}

func exhaustedRuntimeUpdateRepairPlan(attempts int) *RuntimeUpdateRepairPlan {
	plan := manualRuntimeUpdateRepairPlan(
		"runtime_repair_exhausted",
		"同一目标的自动修复已停止",
		"同一恢复目标已经连续执行 3 次受限修复，仍未得到可验收终态。",
		"停止继续重试并保留全部材料；导出支持包后排查 Docker、磁盘、文件锁或目标镜像健康问题。",
	)
	plan.Attempts = attempts
	return plan
}

func waitRuntimeUpdateRepairPlan() *RuntimeUpdateRepairPlan {
	return &RuntimeUpdateRepairPlan{
		Action:      "wait",
		Code:        "runtime_update_in_progress",
		Title:       "升级或启动恢复仍在进行",
		Detection:   "持久状态仍处于非终态；Panel 重启后会按 write-ahead 恢复规则自动续跑或回滚。",
		Method:      "等待当前任务进入终态；不要并发创建第二个修复事务。",
		ButtonLabel: "等待自动恢复",
		Steps:       []string{"等待当前任务", "由 Panel 自动续跑或回滚", "进入终态后重新检测"},
		MaxAttempts: runtimeUpdateRepairAttemptLimit,
	}
}

func safeRetryRuntimeUpdateGuidance(code string) (string, string) {
	switch code {
	case "panel_restart_before_change", "panel_restart_recovery":
		return "上次升级被 Panel 或容器重启中断，但旧版本已经恢复并验收，当前版本对仍与事务起点一致。", "重新读取实时状态和镜像标识，执行完整预检后以新事务重试；不会重放旧事务中的未知步骤。"
	case "backup_failed", "control_backup_failed", "recovery_manifest_failed", "env_write_failed", "instance_reload_failed":
		return "上次升级因磁盘、文件写入或恢复清单持久化失败而结束，实例未修改或已经成功恢复旧版。", "重新校验磁盘空间、目录权限、普通文件与原子写入条件；全部通过后重新备份并以新事务升级。"
	case "stop_failed", "auth_recreate_failed", "auth_start_failed", "server_recreate_failed", "restore_state_failed":
		return "上次 Docker/Compose 生命周期操作失败，但自动回滚已经恢复并验收旧版运行栈。", "重新检查 Docker daemon、Compose project、容器和资源状态，完整预检通过后以新事务重试。"
	default:
		return "上次目标下载、摘要或健康验收失败，但 failed_rolled_back 终态已确认旧版本对和原运行状态恢复。", "重新检查网络、目标 digest、实际容器 image ID、认证和 Junimo/Control 健康；完整预检通过后以新事务重试。"
	}
}

func manualRuntimeStackMethod(code string) string {
	switch code {
	case "unsupported/custom_images":
		return "保留自定义镜像与候选配置；导出支持包后由管理员确认镜像来源和兼容关系，Panel 不自动覆盖。"
	case "invalid_config/server_version_mismatch":
		return "人工确认 IMAGE_VERSION 与 SERVER_IMAGE tag 哪一个才是真实来源；消除歧义后重新检测，Panel 不替用户选择。"
	case "invalid_config/image_reference", "invalid_config/image_candidates":
		return "人工核对镜像引用和候选列表；未知仓库或混合 tag 不会被自动改写，先导出支持包保留证据。"
	case "invalid_config/read_env", "invalid_config/missing_env":
		return "从可信实例备份恢复可读的 .env，并核对文件权限与编码；在来源不明时不要生成替代配置。"
	default:
		return "保留当前配置和运行现场，导出支持包后按检测代码人工核对；Panel 不执行猜测性修改。"
	}
}

// StartRuntimeUpdateRepair diagnoses and repairs only driver-owned known
// failure states from DetectRuntimeUpdateRepairPlan. It then reruns the full
// dry-run and starts a fresh apply transaction. It never accepts an image,
// path, apply ID, or strategy from the caller.
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
	plan := DetectRuntimeUpdateRepairPlan(instance)
	if plan == nil {
		return RuntimeUpdateApplyStatus{}, &RuntimeUpdateValidationError{Code: "runtime_repair_not_needed", Message: "当前没有需要执行的一键修复方案。"}
	}
	if !plan.ActionAvailable {
		return RuntimeUpdateApplyStatus{}, &RuntimeUpdateValidationError{Code: plan.Code, Message: plan.Method}
	}
	status, err := readRuntimeUpdateApplyStatus(instance.DataDir)
	if plan.Code != "repair/rollback_failed" {
		inspection := InspectManagedRuntimeStack(instance.DataDir, instance.State)
		return d.startKnownRuntimeRepairPlan(ctx, runtimeDocker, instance, createdBy, status, inspection, *plan)
	}
	if err != nil {
		return RuntimeUpdateApplyStatus{}, &RuntimeUpdateValidationError{Code: "recovery_state_uncertain", Message: "升级状态在检测后发生变化；请刷新后重试。"}
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

func (d *Driver) startKnownRuntimeRepairPlan(ctx context.Context, docker RuntimeUpdateApplyDockerService, instance registry.Instance, createdBy int64, previous RuntimeUpdateApplyStatus, inspection sjconfig.RuntimeStackInspection, plan RuntimeUpdateRepairPlan) (RuntimeUpdateApplyStatus, error) {
	if plan.Attempts >= runtimeUpdateRepairAttemptLimit {
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
	repairSourceID := previous.ApplyID
	if !runtimeUpdateApplyIDPattern.MatchString(repairSourceID) {
		repairSourceID = sourceID
	}
	status := RuntimeUpdateApplyStatus{
		CreatedBy: createdBy, ApplyID: sourceID, Phase: RuntimeUpdateApplyResumingUpgrade, Progress: 5,
		Current: inspection.Current, Target: inspection.Recommended,
		Checks: []RuntimeUpdateDryRunCheck{
			{Name: "known_issue_detection", Status: "ok", Message: plan.Detection},
			{Name: "repair_plan", Status: "warning", Message: plan.Method},
		},
		Warnings: []string{}, Logs: []RuntimeUpdateDryRunLog{{At: now, Level: "warning", Message: "已命中修复方案“" + plan.Title + "”；将按后端固定步骤执行，复检通过后再升级。"}},
		ServerWasRunning: instance.State == storage.InstanceStateRunning || instance.State == storage.InstanceStateStarting,
		ServerRunning:    instance.State == storage.InstanceStateRunning || instance.State == storage.InstanceStateStarting,
		RepairAttempts:   plan.Attempts + 1, RepairSourceApplyID: repairSourceID, ResumeAfterRepair: true,
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
