package stardew_junimo

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
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

const (
	SMAPIUpdateDryRunJobType = "stardew_smapi_update_dry_run"
	SMAPIUpdateApplyJobType  = "stardew_smapi_update_apply"

	SMAPIApplyIdle             = "idle"
	SMAPIApplyChecking         = "checking"
	SMAPIApplyDownloading      = "downloading"
	SMAPIApplyValidating       = "validating_archive"
	SMAPIApplyCreatingStaging  = "creating_staging"
	SMAPIApplyCloning          = "cloning"
	SMAPIApplyInstalling       = "installing"
	SMAPIApplyVerifyingStaging = "verifying_staging"
	SMAPIApplyStopping         = "stopping"
	SMAPIApplySwitching        = "switching"
	SMAPIApplyStarting         = "starting"
	SMAPIApplyVerifying        = "verifying_stack"
	SMAPIApplyRestoringState   = "restoring_state"
	SMAPIApplySucceeded        = "succeeded"
	SMAPIApplyRollingBack      = "rolling_back"
	SMAPIApplyFailedRolledBack = "failed_rolled_back"
	SMAPIApplyRollbackFailed   = "rollback_failed"
)

var (
	ensureSMAPIArchiveForUpdate   = ensureRecommendedSMAPIArchive
	validateSMAPIArchiveForUpdate = validateRecommendedSMAPIArchive
)

type SMAPIUpdateWorkflowDocker interface {
	LifecycleDockerService
	SMAPIDetectionDockerService
	RuntimeCreateSMAPIStagingVolume(context.Context, string, string, string) error
	RuntimeCloneGameData(context.Context, string, string, string, string) error
	RuntimeInstallSMAPIArchive(context.Context, string, string, string, string) error
	RuntimeRemoveSMAPIStagingVolume(context.Context, string, string, string) error
	RuntimeServerHealth(context.Context, string, string) error
	RuntimeSteamAuthReady(context.Context, string, string) (paneldocker.RuntimeSteamReady, error)
}

type SMAPIUpdateStatus struct {
	UpdateID         string                     `json:"updateId,omitempty"`
	JobID            string                     `json:"jobId,omitempty"`
	Phase            string                     `json:"phase"`
	Progress         int                        `json:"progress"`
	Current          SMAPICurrent               `json:"current"`
	Target           SMAPIRecommendation        `json:"target"`
	Checks           []RuntimeUpdateDryRunCheck `json:"checks"`
	Warnings         []string                   `json:"warnings"`
	Logs             []RuntimeUpdateDryRunLog   `json:"logs"`
	ServerWasRunning bool                       `json:"serverWasRunning"`
	RequiredBytes    int64                      `json:"requiredBytes,omitempty"`
	FreeBytes        int64                      `json:"freeBytes,omitempty"`
	ErrorCode        string                     `json:"errorCode,omitempty"`
	Error            string                     `json:"error,omitempty"`
	ManualAction     string                     `json:"manualAction,omitempty"`
	StartedAt        string                     `json:"startedAt,omitempty"`
	UpdatedAt        string                     `json:"updatedAt,omitempty"`
	FinishedAt       string                     `json:"finishedAt,omitempty"`
	CreatedBy        int64                      `json:"-"`
}

type smapiRecoveryManifest struct {
	SchemaVersion          int    `json:"schemaVersion"`
	UpdateID               string `json:"updateId"`
	ActorID                int64  `json:"actorId"`
	Project                string `json:"project"`
	OriginalVolume         string `json:"originalVolume"`
	StagingVolume          string `json:"stagingVolume"`
	ServerWasRunning       bool   `json:"serverWasRunning"`
	ConfigSwitched         bool   `json:"configSwitched"`
	ControlManifestPresent bool   `json:"controlManifestPresent"`
	ControlDLLPresent      bool   `json:"controlDllPresent"`
}

func (d *Driver) RunSMAPIUpdateDryRun(ctx context.Context, instance registry.Instance) (SMAPIUpdateStatus, error) {
	inspection, err := d.InspectSMAPIUpdate(ctx, instance)
	if err != nil {
		return SMAPIUpdateStatus{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	status := SMAPIUpdateStatus{UpdateID: newSMAPIUpdateID("dryrun"), Phase: SMAPIApplyChecking, Progress: 10, Current: inspection.Current, Target: inspection.Recommended, Checks: []RuntimeUpdateDryRunCheck{}, Warnings: []string{}, Logs: []RuntimeUpdateDryRunLog{}, StartedAt: now, UpdatedAt: now}
	add := func(name, state, message string) {
		status.Checks = append(status.Checks, RuntimeUpdateDryRunCheck{Name: name, Status: state, Message: paneldocker.RedactString(message)})
	}
	fail := func(code, message string) (SMAPIUpdateStatus, error) {
		status.Phase, status.Progress, status.ErrorCode, status.Error, status.FinishedAt = RuntimeUpdatePhaseFailed, 100, code, message, time.Now().UTC().Format(time.RFC3339)
		status.UpdatedAt = status.FinishedAt
		_ = writeSMAPIUpdateStatus(instance.DataDir, "dry-run-status.json", status)
		return status, nil
	}
	if inspection.Status != SMAPIStatusUpdateAvailable && inspection.Status != SMAPIStatusMissing {
		add("smapi_status", "error", inspection.Reason)
		return fail(inspection.Code, inspection.Reason)
	}
	add("smapi_status", "ok", "SMAPI 实际安装产物已读取，目标只来自 Panel 内置推荐清单。")
	manifest, err := sjconfig.BuiltInRuntimeStackManifest()
	if err != nil || !manifest.Installable() || !sjconfig.PanelVersionSatisfies(d.panelVersion, manifest.MinimumPanelVersion) || validateSMAPIManifestURL(manifest.SMAPI.DownloadURL, manifest) != nil {
		return fail("invalid_manifest", "SMAPI 推荐清单无效。")
	}
	add("manifest", "ok", "官方 Release URL、精确版本、SHA256 与大小上限已固定。")
	components, err := d.InspectRuntimeComponents(ctx, instance)
	if err != nil || components.Status != RuntimeComponentsStatusUpToDate {
		return fail("incompatible_game", "Stardew 或 Steamworks SDK 不匹配推荐前置矩阵。")
	}
	add("game_sdk", "ok", "Stardew 与 Steamworks SDK buildid 精确匹配推荐矩阵。")
	stack := InspectRuntimeStack(instance.DataDir, instance.State)
	if stack.Status != sjconfig.RuntimeStackStatusUpToDate {
		return fail("incompatible_junimo", "Junimo server 或 steam-auth-cn 不匹配推荐前置矩阵。")
	}
	add("junimo_auth", "ok", "Junimo server 与 steam-auth-cn 精确匹配推荐版本对。")
	if err := verifyEmbeddedControlManifest(manifest); err != nil {
		return fail("control_incompatible", "内置 Control Mod 与推荐矩阵不一致。")
	}
	add("control", "ok", "Control Mod 版本、DLL SHA256 与 commandResultVersion 匹配推荐矩阵。")
	preflight, err := d.RunRuntimeComponentsPreflight(ctx, instance)
	if err != nil || preflight.Phase != RuntimeUpdatePhaseSucceeded {
		return fail(firstNonEmptyString(preflight.ErrorCode, "staging_preflight_failed"), firstNonEmptyString(preflight.Error, "staging 与磁盘空间预检失败。"))
	}
	status.RequiredBytes = preflight.GameDataBytes + manifest.SMAPI.ArchiveBytes + manifest.SMAPI.MaxExtractBytes
	status.FreeBytes = preflight.FreeBytes
	if status.FreeBytes > 0 && status.FreeBytes < status.RequiredBytes {
		add("staging_space", "error", "game-data 可用空间不足以容纳当前卷克隆与 SMAPI 安装缓冲。")
		return fail("staging_space_insufficient", "可用空间不足以安全创建 SMAPI staging 副本。")
	}
	status.Warnings = append(status.Warnings, preflight.Warnings...)
	add("staging", "ok", "显式 GAME_DATA_VOLUME、当前卷只读容量估算、磁盘空间和受控 staging 命名能力通过。")
	status.Phase, status.Progress, status.UpdatedAt, status.FinishedAt = RuntimeUpdatePhaseSucceeded, 100, time.Now().UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339)
	if err := writeSMAPIUpdateStatus(instance.DataDir, "dry-run-status.json", status); err != nil {
		return SMAPIUpdateStatus{}, err
	}
	return status, nil
}

func (d *Driver) SMAPIUpdateDryRunStatus(instance registry.Instance) (SMAPIUpdateStatus, error) {
	status, err := readSMAPIUpdateStatus(instance.DataDir, "dry-run-status.json")
	if errors.Is(err, os.ErrNotExist) {
		manifest, manifestErr := sjconfig.BuiltInRuntimeStackManifest()
		if manifestErr != nil {
			return SMAPIUpdateStatus{}, manifestErr
		}
		return SMAPIUpdateStatus{Phase: SMAPIApplyIdle, Target: smapiRecommendation(manifest), Checks: []RuntimeUpdateDryRunCheck{}, Warnings: []string{}, Logs: []RuntimeUpdateDryRunLog{}}, nil
	}
	return status, err
}

func (d *Driver) StartSMAPIUpdateApply(ctx context.Context, instance registry.Instance, createdBy int64) (SMAPIUpdateStatus, error) {
	manifest, manifestErr := sjconfig.BuiltInRuntimeStackManifest()
	if manifestErr != nil || !manifest.Installable() || !sjconfig.PanelVersionSatisfies(d.panelVersion, manifest.MinimumPanelVersion) {
		return SMAPIUpdateStatus{}, &RuntimeUpdateValidationError{Code: "matrix_not_recommended", Message: "内置兼容矩阵不是 recommended，禁止 SMAPI 升级。"}
	}
	if d.jobs == nil || d.store == nil {
		return SMAPIUpdateStatus{}, errors.New("SMAPI update service is not configured")
	}
	if previous, err := readSMAPIUpdateStatus(instance.DataDir, "apply-status.json"); err == nil && previous.Phase == SMAPIApplyRollbackFailed {
		return SMAPIUpdateStatus{}, &RuntimeUpdateValidationError{Code: "manual_recovery_required", Message: "上次 SMAPI 升级回滚失败，必须保留材料并人工处理。"}
	}
	dryRun, err := d.SMAPIUpdateDryRunStatus(instance)
	if err != nil || dryRun.Phase != RuntimeUpdatePhaseSucceeded {
		return SMAPIUpdateStatus{}, &RuntimeUpdateValidationError{Code: "dry_run_required", Message: "请先完成当前推荐版本的 SMAPI dry-run。"}
	}
	inspection, err := d.InspectSMAPIUpdate(ctx, instance)
	if err != nil {
		return SMAPIUpdateStatus{}, err
	}
	if inspection.Status != SMAPIStatusUpdateAvailable && inspection.Status != SMAPIStatusMissing {
		return SMAPIUpdateStatus{}, &RuntimeUpdateValidationError{Code: inspection.Code, Message: inspection.Reason}
	}
	dockerWorkflow, ok := d.docker.(SMAPIUpdateWorkflowDocker)
	if !ok {
		return SMAPIUpdateStatus{}, &RuntimeUpdateValidationError{Code: "unsupported/docker_contract", Message: "当前 Docker driver 不支持 SMAPI staging 升级。"}
	}
	d.runtimeUpdateMu.Lock()
	defer d.runtimeUpdateMu.Unlock()
	active, err := d.jobs.Active(ctx, storage.ListActiveJobsFilter{TargetType: "instance", TargetID: instance.ID, Types: []string{"stardew_install", "stardew_lifecycle", RuntimeUpdateDryRunJobType, RuntimeUpdateApplyJobType, SMAPIUpdateDryRunJobType, SMAPIUpdateApplyJobType}})
	if err != nil {
		return SMAPIUpdateStatus{}, err
	}
	if len(active) > 0 {
		return SMAPIUpdateStatus{}, &RuntimeUpdateValidationError{Code: "runtime_update_busy", Message: "实例存在安装、生命周期或组件升级任务。"}
	}
	ps, err := dockerWorkflow.ComposePs(ctx, instance.DataDir)
	if err != nil {
		return SMAPIUpdateStatus{}, &RuntimeUpdateValidationError{Code: "runtime_state_unavailable", Message: "无法读取真实 Docker 运行状态，禁止开始 SMAPI 升级。"}
	}
	now := time.Now().UTC().Format(time.RFC3339)
	status := SMAPIUpdateStatus{UpdateID: newSMAPIUpdateID("apply"), Phase: SMAPIApplyChecking, Progress: 5, Current: inspection.Current, Target: inspection.Recommended, Checks: []RuntimeUpdateDryRunCheck{}, Warnings: []string{}, Logs: []RuntimeUpdateDryRunLog{}, ServerWasRunning: composeServiceRunning(ps.Services, "server"), StartedAt: now, UpdatedAt: now, CreatedBy: createdBy}
	gate := make(chan struct{})
	var initialErr error
	job, err := d.jobs.Start(ctx, jobs.Spec{Type: SMAPIUpdateApplyJobType, DisplayName: "SMAPI 安全升级", TargetType: "instance", TargetID: instance.ID, CreatedBy: createdBy, Timeout: 2 * time.Hour, Run: func(runCtx context.Context, jobCtx *jobs.Context) error {
		<-gate
		if initialErr != nil {
			return initialErr
		}
		return d.runSMAPIUpdateApply(runCtx, jobCtx, dockerWorkflow, instance, status)
	}})
	if err != nil {
		return SMAPIUpdateStatus{}, err
	}
	status.JobID = job.ID
	initialErr = writeSMAPIUpdateStatus(instance.DataDir, "apply-status.json", status)
	close(gate)
	return status, initialErr
}

func (d *Driver) SMAPIUpdateApplyStatus(instance registry.Instance) (SMAPIUpdateStatus, error) {
	status, err := readSMAPIUpdateStatus(instance.DataDir, "apply-status.json")
	if errors.Is(err, os.ErrNotExist) {
		manifest, manifestErr := sjconfig.BuiltInRuntimeStackManifest()
		if manifestErr != nil {
			return SMAPIUpdateStatus{}, manifestErr
		}
		return SMAPIUpdateStatus{Phase: SMAPIApplyIdle, Target: smapiRecommendation(manifest), Checks: []RuntimeUpdateDryRunCheck{}, Warnings: []string{}, Logs: []RuntimeUpdateDryRunLog{}}, nil
	}
	return status, err
}

func (d *Driver) SMAPIUpdateApplyInProgress(instance registry.Instance) bool {
	status, err := readSMAPIUpdateStatus(instance.DataDir, "apply-status.json")
	return err == nil && status.Phase != SMAPIApplyIdle && !smapiApplyTerminal(status.Phase)
}

func smapiApplyTerminal(phase string) bool {
	return phase == SMAPIApplySucceeded || phase == SMAPIApplyFailedRolledBack || phase == SMAPIApplyRollbackFailed
}

// RecoverSMAPIUpdateApply never resumes an installer after a Panel restart.
// Before the volume switch it discards only the controlled staging volume;
// after the switch it runs the same old-volume rollback path as an apply error.
func (d *Driver) RecoverSMAPIUpdateApply(ctx context.Context, instance registry.Instance) error {
	status, err := readSMAPIUpdateStatus(instance.DataDir, "apply-status.json")
	if errors.Is(err, os.ErrNotExist) || err == nil && smapiApplyTerminal(status.Phase) {
		return nil
	}
	if err != nil {
		return err
	}
	workflow, ok := d.docker.(SMAPIUpdateWorkflowDocker)
	if !ok {
		return d.markSMAPIRollbackFailed(instance.DataDir, &status, errors.New("Docker recovery contract unavailable"))
	}
	recovery, err := readSMAPIRecoveryManifest(instance.DataDir, status.UpdateID)
	if err != nil {
		if status.Phase == SMAPIApplyChecking || status.Phase == SMAPIApplyDownloading || status.Phase == SMAPIApplyValidating {
			status.Phase, status.Progress, status.ErrorCode, status.Error = SMAPIApplyFailedRolledBack, 100, "panel_restart_before_change", "Panel 重启发生在任何实例修改前；实例保持原状。"
			status.UpdatedAt, status.FinishedAt = time.Now().UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339)
			return writeSMAPIUpdateStatus(instance.DataDir, "apply-status.json", status)
		}
		return d.markSMAPIRollbackFailed(instance.DataDir, &status, errors.New("SMAPI recovery manifest missing"))
	}
	project := strings.ToLower(filepath.Base(filepath.Clean(instance.DataDir)))
	if recovery.SchemaVersion != 1 || recovery.UpdateID != status.UpdateID || recovery.Project != project || !gameDataVolumeNamePattern.MatchString(recovery.OriginalVolume) || recovery.StagingVolume != project+"_anxi-smapi-update-"+strings.TrimPrefix(status.UpdateID, "apply_") {
		return d.markSMAPIRollbackFailed(instance.DataDir, &status, errors.New("SMAPI recovery manifest is inconsistent"))
	}
	stored, err := d.store.GetInstance(ctx, instance.ID)
	if err != nil {
		return err
	}
	job, err := d.jobs.Start(ctx, jobs.Spec{Type: SMAPIUpdateApplyJobType, DisplayName: "恢复 SMAPI 升级", TargetType: "instance", TargetID: instance.ID, CreatedBy: recovery.ActorID, Timeout: time.Hour, Run: func(runCtx context.Context, jobCtx *jobs.Context) error {
		if !recovery.ConfigSwitched {
			if err := restoreControlMod(instance.DataDir, recovery); err != nil {
				return d.markSMAPIRollbackFailed(instance.DataDir, &status, err)
			}
			if recovery.ServerWasRunning {
				lifecycle := &lifecycleRunner{driver: d, lifecycle: workflow, instance: stored, actorID: recovery.ActorID, preserveControlMod: true}
				stored.State = storage.InstanceStateStopped
				lifecycle.instance = stored
				if err := lifecycle.doStart(runCtx, jobCtx); err != nil {
					return d.markSMAPIRollbackFailed(instance.DataDir, &status, err)
				}
				if err := d.verifySMAPIRollbackStack(runCtx, workflow, instance, recovery.OriginalVolume, status.Current.Version); err != nil {
					return d.markSMAPIRollbackFailed(instance.DataDir, &status, err)
				}
			}
			if err := workflow.RuntimeRemoveSMAPIStagingVolume(runCtx, instance.DataDir, recovery.Project, recovery.StagingVolume); err != nil {
				status.Warnings = append(status.Warnings, "Panel 重启后 staging volume 需人工检查。")
			}
			status.Phase, status.Progress, status.ErrorCode, status.Error = SMAPIApplyFailedRolledBack, 100, "panel_restart_before_switch", "Panel 重启发生在切换前；旧 GAME_DATA_VOLUME 未改变。"
			status.UpdatedAt, status.FinishedAt = time.Now().UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339)
			_ = os.RemoveAll(smapiRecoveryDir(instance.DataDir, recovery.UpdateID))
			return writeSMAPIUpdateStatus(instance.DataDir, "apply-status.json", status)
		}
		return d.rollbackSMAPIUpdate(runCtx, jobCtx, workflow, stored, &status, recovery, true)
	}})
	if err != nil {
		return err
	}
	status.JobID, status.UpdatedAt = job.ID, time.Now().UTC().Format(time.RFC3339)
	return writeSMAPIUpdateStatus(instance.DataDir, "apply-status.json", status)
}

func (d *Driver) runSMAPIUpdateApply(ctx context.Context, jobCtx *jobs.Context, dockerWorkflow SMAPIUpdateWorkflowDocker, instance registry.Instance, status SMAPIUpdateStatus) (runErr error) {
	manifest, err := sjconfig.BuiltInRuntimeStackManifest()
	if err != nil {
		return err
	}
	storageInstance, err := d.store.GetInstance(ctx, instance.ID)
	if err != nil {
		return err
	}
	project := strings.ToLower(filepath.Base(filepath.Clean(instance.DataDir)))
	originalVolume, err := GameDataVolumeName(instance.DataDir)
	if err != nil {
		return err
	}
	stagingVolume := project + "_anxi-smapi-update-" + strings.TrimPrefix(status.UpdateID, "apply_")
	recovery := smapiRecoveryManifest{SchemaVersion: 1, UpdateID: status.UpdateID, ActorID: status.CreatedBy, Project: project, OriginalVolume: originalVolume, StagingVolume: stagingVolume, ServerWasRunning: status.ServerWasRunning}
	mutated := false
	defer func() {
		if runErr == nil {
			return
		}
		if rollbackErr := d.rollbackSMAPIUpdate(context.Background(), jobCtx, dockerWorkflow, storageInstance, &status, recovery, mutated); rollbackErr != nil {
			runErr = fmt.Errorf("%w; rollback failed", runErr)
		}
	}()
	set := func(phase string, progress int, message string) error {
		status.Phase, status.Progress, status.UpdatedAt = phase, progress, time.Now().UTC().Format(time.RFC3339)
		status.Logs = append(status.Logs, RuntimeUpdateDryRunLog{At: status.UpdatedAt, Level: "info", Message: paneldocker.RedactString(message)})
		if len(status.Logs) > 40 {
			status.Logs = status.Logs[len(status.Logs)-40:]
		}
		_, _ = jobCtx.Info(ctx, message)
		return writeSMAPIUpdateStatus(instance.DataDir, "apply-status.json", status)
	}
	if err := set(SMAPIApplyChecking, 8, "重新验证 SMAPI 推荐矩阵和所有前置组件。"); err != nil {
		return err
	}
	dry, err := d.RunSMAPIUpdateDryRun(ctx, instance)
	if err != nil || dry.Phase != RuntimeUpdatePhaseSucceeded {
		return errors.New("SMAPI apply preflight no longer passes")
	}
	if err := set(SMAPIApplyDownloading, 15, "下载清单中的精确 SMAPI 官方安装包。"); err != nil {
		return err
	}
	archivePath, err := ensureSMAPIArchiveForUpdate(ctx, instance.DataDir, manifest)
	if err != nil {
		return err
	}
	if err := set(SMAPIApplyValidating, 25, "校验 SHA256、ZIP 结构、路径和解压上限。"); err != nil {
		return err
	}
	if err := validateSMAPIArchiveForUpdate(archivePath, manifest); err != nil {
		return err
	}
	controlManifestPresent, controlDLLPresent, err := backupControlMod(instance.DataDir, status.UpdateID)
	if err != nil {
		return err
	}
	recovery.ControlManifestPresent = controlManifestPresent
	recovery.ControlDLLPresent = controlDLLPresent
	if err := writeSMAPIRecoveryManifest(instance.DataDir, recovery); err != nil {
		return err
	}
	if err := set(SMAPIApplyCreatingStaging, 32, "创建受控 staging game-data volume。"); err != nil {
		return err
	}
	if err := dockerWorkflow.RuntimeCreateSMAPIStagingVolume(ctx, instance.DataDir, project, stagingVolume); err != nil {
		return err
	}
	mutated = true
	image := gameInstallImage(instance.DataDir)
	if err := set(SMAPIApplyCloning, 42, "只读克隆当前 GAME_DATA_VOLUME 到 staging。"); err != nil {
		return err
	}
	if err := dockerWorkflow.RuntimeCloneGameData(ctx, instance.DataDir, originalVolume, stagingVolume, image); err != nil {
		return err
	}
	if err := set(SMAPIApplyInstalling, 55, "在 staging volume 上运行 SMAPI 官方 Linux installer。"); err != nil {
		return err
	}
	if err := dockerWorkflow.RuntimeInstallSMAPIArchive(ctx, instance.DataDir, stagingVolume, archivePath, image); err != nil {
		return err
	}
	if err := set(SMAPIApplyVerifyingStaging, 63, "验证 staging SMAPI 精确版本。"); err != nil {
		return err
	}
	meta, err := dockerWorkflow.RuntimeReadSMAPIMetadata(ctx, instance.DataDir, stagingVolume, image)
	if err != nil || normalizedSMAPIVersion(meta.Version) != manifest.SMAPI.Version || !meta.RequiredFiles {
		return errors.New("staging SMAPI version verification failed")
	}
	lifecycle := &lifecycleRunner{driver: d, lifecycle: dockerWorkflow, instance: storageInstance, actorID: status.CreatedBy}
	ps, err := dockerWorkflow.ComposePs(ctx, instance.DataDir)
	if err != nil {
		return errors.New("runtime state unavailable before SMAPI switch")
	}
	if composeServiceRunning(ps.Services, "server") || composeServiceRunning(ps.Services, "steam-auth") {
		if err := set(SMAPIApplyStopping, 70, "停止服务器后切换 GAME_DATA_VOLUME。"); err != nil {
			return err
		}
		if err := lifecycle.doStop(ctx, jobCtx); err != nil {
			return err
		}
	}
	// Control is a fixed host bind shared by both game-data volumes. Update it
	// only after the old runtime has stopped; the recovery manifest records
	// whether each old file existed so rollback can also restore an absent Mod.
	if err := installSMAPIMod(instance.DataDir); err != nil {
		return err
	}
	if err := set(SMAPIApplySwitching, 76, "原子切换到已验证的 staging GAME_DATA_VOLUME。"); err != nil {
		return err
	}
	if err := switchGameDataVolumeAtomic(instance.DataDir, stagingVolume); err != nil {
		return err
	}
	recovery.ConfigSwitched = true
	if err := writeSMAPIRecoveryManifest(instance.DataDir, recovery); err != nil {
		return err
	}
	if err := set(SMAPIApplyStarting, 82, "启动完整推荐 stack 进行验收。"); err != nil {
		return err
	}
	storageInstance.State = storage.InstanceStateStopped
	lifecycle.instance = storageInstance
	if err := lifecycle.doStart(ctx, jobCtx); err != nil {
		return err
	}
	if err := set(SMAPIApplyVerifying, 90, "验证 SMAPI、Junimo、Control、状态文件与 auth 服务接口。"); err != nil {
		return err
	}
	if err := d.verifySMAPIUpdatedStack(ctx, dockerWorkflow, instance, stagingVolume, manifest); err != nil {
		return err
	}
	if !status.ServerWasRunning {
		if err := set(SMAPIApplyRestoringState, 96, "恢复升级前的停止状态。"); err != nil {
			return err
		}
		if err := lifecycle.doStop(ctx, jobCtx); err != nil {
			return err
		}
	}
	status.Phase, status.Progress, status.UpdatedAt, status.FinishedAt = SMAPIApplySucceeded, 100, time.Now().UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339)
	status.Checks = append(status.Checks, RuntimeUpdateDryRunCheck{Name: "full_stack", Status: "ok", Message: "SMAPI、JunimoServer、Control、commandResultVersion、status/players、health 与 auth 服务接口均通过；Steam 登录和邀请码不属于升级硬门槛。"})
	if err := writeSMAPIUpdateStatus(instance.DataDir, "apply-status.json", status); err != nil {
		return err
	}
	_ = os.RemoveAll(smapiRecoveryDir(instance.DataDir, status.UpdateID))
	return nil
}

func (d *Driver) verifySMAPIUpdatedStack(ctx context.Context, dockerWorkflow SMAPIUpdateWorkflowDocker, instance registry.Instance, volume string, manifest sjconfig.RuntimeStackManifest) error {
	return d.verifySMAPIStack(ctx, dockerWorkflow, instance, volume, manifest.SMAPI.Version, manifest.Control.CommandResultVersion)
}

func (d *Driver) verifySMAPIRollbackStack(ctx context.Context, dockerWorkflow SMAPIUpdateWorkflowDocker, instance registry.Instance, volume, expectedVersion string) error {
	manifest, err := sjconfig.BuiltInRuntimeStackManifest()
	if err != nil {
		return err
	}
	return d.verifySMAPIStack(ctx, dockerWorkflow, instance, volume, expectedVersion, manifest.Control.CommandResultVersion)
}

func (d *Driver) verifySMAPIStack(ctx context.Context, dockerWorkflow SMAPIUpdateWorkflowDocker, instance registry.Instance, volume, expectedVersion string, commandResultVersion int) error {
	deadline := time.Now().Add(d.runtimeUpdateServerTimeout)
	for time.Now().Before(deadline) {
		meta, metaErr := dockerWorkflow.RuntimeReadSMAPIMetadata(ctx, instance.DataDir, volume, gameInstallImage(instance.DataDir))
		healthErr := dockerWorkflow.RuntimeServerHealth(ctx, instance.DataDir, strings.ToLower(filepath.Base(instance.DataDir)))
		_, authErr := dockerWorkflow.RuntimeSteamAuthReady(ctx, instance.DataDir, strings.ToLower(filepath.Base(instance.DataDir)))
		statusOK, playersOK := verifyControlRuntimeFiles(instance.DataDir, commandResultVersion)
		logs, logsErr := dockerWorkflow.ComposeLogs(ctx, instance.DataDir, paneldocker.LogsOptions{Service: "server", Tail: 800})
		junimoLoaded, controlLoaded := requiredSMAPIModsLoaded(logs.Stdout + "\n" + logs.Stderr)
		if metaErr == nil && meta.RequiredFiles && normalizedSMAPIVersion(meta.Version) == normalizedSMAPIVersion(expectedVersion) && healthErr == nil && authErr == nil && logsErr == nil && junimoLoaded && controlLoaded && statusOK && playersOK {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(d.runtimeUpdatePollInterval):
		}
	}
	return errors.New("full SMAPI stack verification timed out")
}

func requiredSMAPIModsLoaded(logs string) (bool, bool) {
	var junimo, control bool
	for _, line := range strings.Split(strings.ToLower(logs), "\n") {
		if !strings.Contains(line, "[smapi]") || strings.Contains(line, "failed") || strings.Contains(line, "error") || strings.Contains(line, "exception") {
			continue
		}
		junimo = junimo || strings.Contains(line, "junimoserver")
		control = control || strings.Contains(line, "stardewanxipanel.control")
	}
	return junimo, control
}

func verifyControlRuntimeFiles(dataDir string, commandVersion int) (bool, bool) {
	var status struct {
		CommandResultVersion int    `json:"commandResultVersion"`
		UpdatedAt            string `json:"updatedAt"`
	}
	var players struct {
		UpdatedAt string            `json:"updatedAt"`
		Players   []json.RawMessage `json:"players"`
	}
	statusData, statusErr := os.ReadFile(filepath.Join(dataDir, ".local-container", "control", "status.json"))
	playersData, playersErr := os.ReadFile(filepath.Join(dataDir, ".local-container", "control", "players.json"))
	statusOK := statusErr == nil && json.Unmarshal(statusData, &status) == nil && status.CommandResultVersion >= commandVersion && status.UpdatedAt != ""
	playersOK := playersErr == nil && json.Unmarshal(playersData, &players) == nil && players.UpdatedAt != "" && players.Players != nil
	return statusOK, playersOK
}

func normalizedSMAPIVersion(raw string) string {
	raw = strings.TrimSpace(raw)
	if smapiExactVersionPattern.MatchString(raw) {
		return raw
	}
	match := smapiInformationalVersionPattern.FindStringSubmatch(raw)
	if len(match) == 2 {
		return match[1]
	}
	return ""
}

func (d *Driver) rollbackSMAPIUpdate(ctx context.Context, jobCtx *jobs.Context, dockerWorkflow SMAPIUpdateWorkflowDocker, instance storage.Instance, status *SMAPIUpdateStatus, recovery smapiRecoveryManifest, mutated bool) error {
	status.Phase, status.Progress, status.ErrorCode, status.Error, status.UpdatedAt = SMAPIApplyRollingBack, 95, "apply_failed", "SMAPI 升级失败，正在恢复旧 GAME_DATA_VOLUME。", time.Now().UTC().Format(time.RFC3339)
	_ = writeSMAPIUpdateStatus(instance.DataDir, "apply-status.json", *status)
	if !mutated {
		status.Phase, status.Progress, status.FinishedAt = SMAPIApplyFailedRolledBack, 100, time.Now().UTC().Format(time.RFC3339)
		status.UpdatedAt = status.FinishedAt
		return writeSMAPIUpdateStatus(instance.DataDir, "apply-status.json", *status)
	}
	lifecycle := &lifecycleRunner{driver: d, lifecycle: dockerWorkflow, instance: instance, actorID: recovery.ActorID, preserveControlMod: true}
	if recovery.ConfigSwitched {
		if err := lifecycle.doStop(ctx, jobCtx); err != nil {
			return d.markSMAPIRollbackFailed(instance.DataDir, status, err)
		}
		if err := switchGameDataVolumeAtomic(instance.DataDir, recovery.OriginalVolume); err != nil {
			return d.markSMAPIRollbackFailed(instance.DataDir, status, err)
		}
	}
	if err := restoreControlMod(instance.DataDir, recovery); err != nil {
		return d.markSMAPIRollbackFailed(instance.DataDir, status, err)
	}
	instance.State = storage.InstanceStateStopped
	lifecycle.instance = instance
	if err := lifecycle.doStart(ctx, jobCtx); err != nil {
		return d.markSMAPIRollbackFailed(instance.DataDir, status, err)
	}
	if err := d.verifySMAPIRollbackStack(ctx, dockerWorkflow, registry.Instance{ID: instance.ID, DriverID: instance.DriverID, Name: instance.Name, DataDir: instance.DataDir, State: instance.State, DriverPhase: instance.DriverPhase, DriverPayload: instance.DriverPayload}, recovery.OriginalVolume, status.Current.Version); err != nil {
		return d.markSMAPIRollbackFailed(instance.DataDir, status, err)
	}
	if !recovery.ServerWasRunning {
		if err := lifecycle.doStop(ctx, jobCtx); err != nil {
			return d.markSMAPIRollbackFailed(instance.DataDir, status, err)
		}
	}
	if err := dockerWorkflow.RuntimeRemoveSMAPIStagingVolume(ctx, instance.DataDir, recovery.Project, recovery.StagingVolume); err != nil {
		status.Warnings = append(status.Warnings, "已恢复旧卷，但失败的 staging volume 需人工清理。")
	}
	status.Phase, status.Progress, status.UpdatedAt, status.FinishedAt = SMAPIApplyFailedRolledBack, 100, time.Now().UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339)
	if err := writeSMAPIUpdateStatus(instance.DataDir, "apply-status.json", *status); err != nil {
		return err
	}
	_ = os.RemoveAll(smapiRecoveryDir(instance.DataDir, recovery.UpdateID))
	return nil
}

func (d *Driver) markSMAPIRollbackFailed(dataDir string, status *SMAPIUpdateStatus, cause error) error {
	status.Phase, status.Progress, status.ErrorCode, status.Error = SMAPIApplyRollbackFailed, 100, "rollback_failed", "自动回滚失败；恢复材料和旧/新 volume 已保留。"
	status.ManualAction = "保留实例目录与两份 Docker volume，人工核对 GAME_DATA_VOLUME 后恢复；禁止自动重试或 volume prune。"
	status.UpdatedAt, status.FinishedAt = time.Now().UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339)
	_ = writeSMAPIUpdateStatus(dataDir, "apply-status.json", *status)
	return cause
}

func backupControlMod(dataDir, updateID string) (bool, bool, error) {
	dir := smapiRecoveryDir(dataDir, updateID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return false, false, err
	}
	present := map[string]bool{}
	for _, name := range []string{"manifest.json", "StardewAnxiPanel.Control.dll"} {
		data, err := os.ReadFile(filepath.Join(smapiModDir(dataDir), name))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return false, false, err
		}
		present[name] = true
		if err := os.WriteFile(filepath.Join(dir, "control-"+name), data, 0o600); err != nil {
			return false, false, err
		}
	}
	return present["manifest.json"], present["StardewAnxiPanel.Control.dll"], nil
}

func restoreControlMod(dataDir string, recovery smapiRecoveryManifest) error {
	dir := smapiRecoveryDir(dataDir, recovery.UpdateID)
	existed := map[string]bool{
		"manifest.json":                recovery.ControlManifestPresent,
		"StardewAnxiPanel.Control.dll": recovery.ControlDLLPresent,
	}
	for _, name := range []string{"manifest.json", "StardewAnxiPanel.Control.dll"} {
		if !existed[name] {
			if err := os.Remove(filepath.Join(smapiModDir(dataDir), name)); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			continue
		}
		backup := filepath.Join(dir, "control-"+name)
		data, err := os.ReadFile(backup)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(smapiModDir(dataDir), name), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func smapiRecoveryDir(dataDir, updateID string) string {
	return filepath.Join(dataDir, ".local-container", "smapi-update", "recovery", updateID)
}
func writeSMAPIRecoveryManifest(dataDir string, manifest smapiRecoveryManifest) error {
	dir := smapiRecoveryDir(dataDir, manifest.UpdateID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".recovery-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return replaceRuntimeUpdateStatusFile(name, filepath.Join(dir, "manifest.json"))
}

func readSMAPIRecoveryManifest(dataDir, updateID string) (smapiRecoveryManifest, error) {
	data, err := os.ReadFile(filepath.Join(smapiRecoveryDir(dataDir, updateID), "manifest.json"))
	if err != nil {
		return smapiRecoveryManifest{}, err
	}
	var manifest smapiRecoveryManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return manifest, err
	}
	return manifest, nil
}

func newSMAPIUpdateID(prefix string) string {
	var value [12]byte
	if _, err := rand.Read(value[:]); err != nil {
		return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
	}
	return prefix + "_" + hex.EncodeToString(value[:])
}
func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
func smapiStatusPath(dataDir, filename string) string {
	return filepath.Join(dataDir, ".local-container", "smapi-update", filename)
}
func readSMAPIUpdateStatus(dataDir, filename string) (SMAPIUpdateStatus, error) {
	data, err := readRuntimeStatusFile(smapiStatusPath(dataDir, filename))
	if err != nil {
		return SMAPIUpdateStatus{}, err
	}
	var status SMAPIUpdateStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return status, err
	}
	if status.Checks == nil {
		status.Checks = []RuntimeUpdateDryRunCheck{}
	}
	if status.Warnings == nil {
		status.Warnings = []string{}
	}
	if status.Logs == nil {
		status.Logs = []RuntimeUpdateDryRunLog{}
	}
	return status, nil
}
func writeSMAPIUpdateStatus(dataDir, filename string, status SMAPIUpdateStatus) error {
	path := smapiStatusPath(dataDir, filename)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".smapi-status-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return replaceRuntimeUpdateStatusFile(name, path)
}
