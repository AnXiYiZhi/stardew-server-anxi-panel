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
	"regexp"
	"strings"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

const (
	RuntimeUpdateApplyJobType = "stardew_junimo_update_apply"

	RuntimeUpdateApplyChecking         = "checking"
	RuntimeUpdateApplyPulling          = "pulling"
	RuntimeUpdateApplyBackingUp        = "backing_up"
	RuntimeUpdateApplyStopping         = "stopping"
	RuntimeUpdateApplyWritingConfig    = "writing_config"
	RuntimeUpdateApplyRecreatingAuth   = "recreating_auth"
	RuntimeUpdateApplyVerifyingAuth    = "verifying_auth"
	RuntimeUpdateApplyRecreatingServer = "recreating_server"
	RuntimeUpdateApplyVerifyingServer  = "verifying_server"
	RuntimeUpdateApplyRestoringState   = "restoring_state"
	RuntimeUpdateApplySucceeded        = "succeeded"
	RuntimeUpdateApplyRollingBack      = "rolling_back"
	RuntimeUpdateApplyResumingUpgrade  = "resuming_upgrade"
	RuntimeUpdateApplyFailedRolledBack = "failed_rolled_back"
	RuntimeUpdateApplyRollbackFailed   = "rollback_failed"
)

type RuntimeUpdateApplyDockerService interface {
	RuntimeUpdateDockerService
	LifecycleDockerService
	RuntimeComposeStopServices(context.Context, string, string, ...string) error
	RuntimeComposeUpService(context.Context, string, string, string) error
	RuntimeComposeUpServicePreserve(context.Context, string, string, string) error
	RuntimeUpdateServiceCPUShares(context.Context, string, string, string, int64) error
	RuntimeServiceInspect(context.Context, string, string, string) (paneldocker.RuntimeServiceMetadata, error)
	RuntimeSteamAuthReady(context.Context, string, string) (paneldocker.RuntimeSteamReady, error)
	RuntimeServerHealth(context.Context, string, string) error
	RuntimeCreateSnapshotVolume(context.Context, string, string, string) error
	RuntimeCloneVolume(context.Context, string, string, string, string) error
	RuntimeRestoreVolume(context.Context, string, string, string, string) error
	RuntimeRemoveSnapshotVolume(context.Context, string, string, string) error
	RuntimeRemoveImage(context.Context, string, string, string) error
}

type runtimeUpdateAuditStore interface {
	CreateAuditLog(context.Context, storage.AuditLogParams) error
}

func (d *Driver) auditRuntimeUpdateTerminal(ctx context.Context, instanceID string, status RuntimeUpdateApplyStatus) {
	store, ok := d.store.(runtimeUpdateAuditStore)
	if !ok {
		return
	}
	fromStackVersion := "junimo-" + status.Current.Server.Tag + "_auth-" + status.Current.SteamAuth.Tag
	metadata, _ := json.Marshal(map[string]string{"applyId": status.ApplyID, "fromStackVersion": fromStackVersion, "toStackVersion": status.Target.StackVersion, "terminalPhase": status.Phase})
	params := storage.AuditLogParams{Action: "junimo_runtime_update_apply_terminal", TargetType: "instance", TargetID: instanceID, Metadata: string(metadata)}
	if status.CreatedBy > 0 {
		actor := status.CreatedBy
		params.ActorUserID = &actor
	}
	if err := store.CreateAuditLog(ctx, params); err != nil {
		d.logger.Warn("failed to audit Junimo runtime update terminal state", "instance", instanceID, "phase", status.Phase)
	}
}

type RuntimeUpdateApplyStatus struct {
	CreatedBy           int64                               `json:"-"`
	ApplyID             string                              `json:"applyId,omitempty"`
	JobID               string                              `json:"jobId,omitempty"`
	Phase               string                              `json:"phase"`
	Progress            int                                 `json:"progress"`
	Download            *RuntimeUpdateDownloadProgress      `json:"download,omitempty"`
	Current             sjconfig.RuntimeStackCurrent        `json:"current"`
	Target              sjconfig.RuntimeStackRecommendation `json:"target"`
	Selected            RuntimeUpdateSelectedPair           `json:"selected"`
	Checks              []RuntimeUpdateDryRunCheck          `json:"checks"`
	Warnings            []string                            `json:"warnings"`
	Logs                []RuntimeUpdateDryRunLog            `json:"logs"`
	ServerWasRunning    bool                                `json:"serverWasRunning"`
	ServerRunning       bool                                `json:"serverRunning"`
	ErrorCode           string                              `json:"errorCode,omitempty"`
	Error               string                              `json:"error,omitempty"`
	CauseCode           string                              `json:"causeCode,omitempty"`
	CauseError          string                              `json:"causeError,omitempty"`
	RollbackCode        string                              `json:"rollbackCode,omitempty"`
	RollbackError       string                              `json:"rollbackError,omitempty"`
	RepairAttempts      int                                 `json:"repairAttempts,omitempty"`
	RepairSourceApplyID string                              `json:"repairSourceApplyId,omitempty"`
	ResumeAfterRepair   bool                                `json:"resumeAfterRepair,omitempty"`
	ManualAction        string                              `json:"manualAction,omitempty"`
	StartedAt           string                              `json:"startedAt,omitempty"`
	UpdatedAt           string                              `json:"updatedAt,omitempty"`
	FinishedAt          string                              `json:"finishedAt,omitempty"`
}

type runtimeUpdateRecoveryManifest struct {
	SchemaVersion            int                        `json:"schemaVersion"`
	ApplyID                  string                     `json:"applyId"`
	ActorID                  int64                      `json:"actorId"`
	Project                  string                     `json:"project"`
	SteamSessionVolume       string                     `json:"steamSessionVolume"`
	SnapshotVolume           string                     `json:"snapshotVolume"`
	ServerWasRunning         bool                       `json:"serverWasRunning"`
	AuthWasRunning           bool                       `json:"authWasRunning"`
	ServerImageChanged       bool                       `json:"serverImageChanged,omitempty"`
	AuthImageChanged         bool                       `json:"authImageChanged,omitempty"`
	AuthSnapshotCreated      bool                       `json:"authSnapshotCreated,omitempty"`
	OriginalState            string                     `json:"originalState"`
	OriginalServer           RuntimeUpdateSelectedImage `json:"originalServer"`
	OriginalAuth             RuntimeUpdateSelectedImage `json:"originalAuth"`
	Target                   RuntimeUpdateSelectedPair  `json:"target"`
	OriginalServerVersion    string                     `json:"originalServerVersion"`
	TargetServerVersion      string                     `json:"targetServerVersion"`
	JunimoModOriginalPresent bool                       `json:"junimoModOriginalPresent"`
	JunimoModPrepared        bool                       `json:"junimoModPrepared"`
	JunimoModReplaced        bool                       `json:"junimoModReplaced"`
	ConfigWritten            bool                       `json:"configWritten"`
	AuthRecreated            bool                       `json:"authRecreated"`
	ServerRecreated          bool                       `json:"serverRecreated"`
	ControlManifestPresent   bool                       `json:"controlManifestPresent"`
	ControlDLLPresent        bool                       `json:"controlDllPresent"`
	ControlUpdated           bool                       `json:"controlUpdated"`
	MutationStarted          bool                       `json:"mutationStarted,omitempty"`
	StopIntent               bool                       `json:"stopIntent,omitempty"`
	ControlUpdateIntent      bool                       `json:"controlUpdateIntent,omitempty"`
	AuthSnapshotCreateIntent bool                       `json:"authSnapshotCreateIntent,omitempty"`
	AuthSnapshotVolumeMade   bool                       `json:"authSnapshotVolumeMade,omitempty"`
	JunimoModReplaceIntent   bool                       `json:"junimoModReplaceIntent,omitempty"`
	ConfigWriteIntent        bool                       `json:"configWriteIntent,omitempty"`
	AuthRecreateIntent       bool                       `json:"authRecreateIntent,omitempty"`
	AuthServiceStartIntent   bool                       `json:"authServiceStartIntent,omitempty"`
	ServerRecreateIntent     bool                       `json:"serverRecreateIntent,omitempty"`
	LastIntent               string                     `json:"lastIntent,omitempty"`
	OriginalEnvSHA256        string                     `json:"originalEnvSha256,omitempty"`
	OriginalComposeSHA256    string                     `json:"originalComposeSha256,omitempty"`
	OriginalControlJSONSHA   string                     `json:"originalControlManifestSha256,omitempty"`
	OriginalControlDLLSHA    string                     `json:"originalControlDllSha256,omitempty"`
}

var runtimeUpdateApplyIDPattern = regexp.MustCompile(`^apply_[a-f0-9]{24}$`)

func runtimeUpdateServerChanged(manifest runtimeUpdateRecoveryManifest) bool {
	return manifest.SchemaVersion < 2 || manifest.ServerImageChanged
}

func runtimeUpdateAuthChanged(manifest runtimeUpdateRecoveryManifest) bool {
	return manifest.SchemaVersion < 2 || manifest.AuthImageChanged
}

func runtimeUpdateAuthSnapshotCreated(manifest runtimeUpdateRecoveryManifest) bool {
	return manifest.SchemaVersion < 2 || manifest.AuthSnapshotCreated
}

func runtimeUpdateAuthSnapshotVolumeCreated(manifest runtimeUpdateRecoveryManifest) bool {
	if manifest.SchemaVersion < 3 {
		return runtimeUpdateAuthSnapshotCreated(manifest)
	}
	// The Docker volume may already exist when the process is interrupted after
	// the create call reached the daemon but before its completion flag was
	// persisted. Removing this transaction-owned exact name is idempotent.
	return manifest.AuthSnapshotCreateIntent || manifest.AuthSnapshotVolumeMade || manifest.AuthSnapshotCreated
}

func runtimeUpdateMutationStarted(manifest runtimeUpdateRecoveryManifest) bool {
	// Schema 1/2 persisted completion flags after several mutations. Once their
	// recovery manifest exists, conservatively roll back instead of assuming an
	// absent completion flag proves that nothing happened.
	return manifest.SchemaVersion < 3 || manifest.MutationStarted
}

func runtimeUpdateControlMayHaveChanged(manifest runtimeUpdateRecoveryManifest) bool {
	return manifest.ControlUpdated || manifest.SchemaVersion >= 3 && manifest.ControlUpdateIntent
}

func runtimeUpdateJunimoModMayHaveChanged(manifest runtimeUpdateRecoveryManifest) bool {
	return manifest.JunimoModReplaced || manifest.SchemaVersion >= 3 && manifest.JunimoModReplaceIntent
}

func runtimeUpdateAuthMayHaveBeenRecreated(manifest runtimeUpdateRecoveryManifest) bool {
	return manifest.AuthRecreated || manifest.SchemaVersion >= 3 && manifest.AuthRecreateIntent
}

func runtimeUpdateAuthServiceMayHaveStarted(manifest runtimeUpdateRecoveryManifest) bool {
	return runtimeUpdateAuthMayHaveBeenRecreated(manifest) || manifest.SchemaVersion >= 3 && manifest.AuthServiceStartIntent
}

func runtimeUpdateServerMayHaveBeenRecreated(manifest runtimeUpdateRecoveryManifest) bool {
	return manifest.ServerRecreated || manifest.SchemaVersion >= 3 && manifest.ServerRecreateIntent
}

func (d *Driver) StartRuntimeUpdateApply(ctx context.Context, instance registry.Instance, createdBy int64) (RuntimeUpdateApplyStatus, error) {
	if d.jobs == nil || d.store == nil {
		return RuntimeUpdateApplyStatus{}, errors.New("runtime update apply service is not configured")
	}
	if instance.DriverID != DriverID {
		return RuntimeUpdateApplyStatus{}, &RuntimeUpdateValidationError{Code: "unsupported/driver", Message: "实例不是 stardew_junimo driver。"}
	}
	if previous, err := readRuntimeUpdateApplyStatus(instance.DataDir); err == nil && previous.Phase == RuntimeUpdateApplyRollbackFailed {
		return RuntimeUpdateApplyStatus{}, &RuntimeUpdateValidationError{Code: "manual_recovery_required", Message: "上次升级自动回滚失败；必须先人工处理并保全恢复材料，禁止自动重试。"}
	}
	inspection := InspectManagedRuntimeStack(instance.DataDir, instance.State)
	if inspection.Status == sjconfig.RuntimeStackStatusUpToDate {
		return RuntimeUpdateApplyStatus{}, &RuntimeUpdateValidationError{Code: "already_up_to_date", Message: "当前运行组件版本对已经是推荐版本。"}
	}
	if inspection.Status != sjconfig.RuntimeStackStatusUpdateAvailable {
		return RuntimeUpdateApplyStatus{}, &RuntimeUpdateValidationError{Code: inspection.Code, Message: inspection.Reason}
	}
	dryRun, err := readRuntimeUpdateDryRunStatus(instance.DataDir)
	if err != nil || dryRun.Phase != RuntimeUpdatePhaseSucceeded || dryRun.Target.StackVersion != inspection.Recommended.StackVersion {
		return RuntimeUpdateApplyStatus{}, &RuntimeUpdateValidationError{Code: "dry_run_required", Message: "请先完成当前推荐版本对的升级预检。"}
	}
	runtimeDocker, ok := d.docker.(RuntimeUpdateApplyDockerService)
	if !ok {
		return RuntimeUpdateApplyStatus{}, &RuntimeUpdateValidationError{Code: "unsupported/docker_contract", Message: "当前 Docker driver 不支持 Junimo 成对升级。"}
	}

	d.runtimeUpdateMu.Lock()
	defer d.runtimeUpdateMu.Unlock()
	active, err := d.jobs.Active(ctx, storage.ListActiveJobsFilter{TargetType: "instance", TargetID: instance.ID, Types: []string{"stardew_install", "stardew_lifecycle", RuntimeUpdateDryRunJobType, RuntimeUpdateApplyJobType, SMAPIUpdateDryRunJobType, SMAPIUpdateApplyJobType}})
	if err != nil {
		return RuntimeUpdateApplyStatus{}, fmt.Errorf("list conflicting jobs: %w", err)
	}
	if len(active) > 0 {
		return RuntimeUpdateApplyStatus{}, &RuntimeUpdateValidationError{Code: "runtime_update_busy", Message: "实例存在安装、生命周期或运行组件更新任务。"}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	status := RuntimeUpdateApplyStatus{CreatedBy: createdBy, ApplyID: newRuntimeApplyID(), Phase: RuntimeUpdateApplyChecking, Current: inspection.Current, Target: inspection.Recommended, Checks: []RuntimeUpdateDryRunCheck{}, Warnings: []string{}, Logs: []RuntimeUpdateDryRunLog{}, ServerWasRunning: instance.State == storage.InstanceStateRunning || instance.State == storage.InstanceStateStarting, StartedAt: now, UpdatedAt: now}
	gate := make(chan struct{})
	var initialWriteErr error
	job, err := d.jobs.Start(ctx, jobs.Spec{Type: RuntimeUpdateApplyJobType, DisplayName: "Junimo 运行组件成对升级", TargetType: "instance", TargetID: instance.ID, CreatedBy: createdBy, Timeout: 2 * time.Hour, Run: func(runCtx context.Context, jobCtx *jobs.Context) error {
		<-gate
		if initialWriteErr != nil {
			return errors.New("升级状态初始化失败")
		}
		return d.runRuntimeUpdateApply(runCtx, jobCtx, runtimeDocker, instance, status, nil)
	}})
	if err != nil {
		return RuntimeUpdateApplyStatus{}, fmt.Errorf("start runtime update apply job: %w", err)
	}
	status.JobID = job.ID
	initialWriteErr = writeRuntimeUpdateApplyStatus(instance.DataDir, status)
	close(gate)
	if initialWriteErr != nil {
		return RuntimeUpdateApplyStatus{}, initialWriteErr
	}
	return status, nil
}

func (d *Driver) RuntimeUpdateApplyStatus(instance registry.Instance) (RuntimeUpdateApplyStatus, error) {
	status, err := readRuntimeUpdateApplyStatus(instance.DataDir)
	if errors.Is(err, os.ErrNotExist) {
		inspection := InspectManagedRuntimeStack(instance.DataDir, instance.State)
		return RuntimeUpdateApplyStatus{Phase: "idle", Current: inspection.Current, Target: inspection.Recommended, Checks: []RuntimeUpdateDryRunCheck{}, Warnings: []string{}, Logs: []RuntimeUpdateDryRunLog{}}, nil
	}
	return status, err
}

// RecoverRuntimeUpdateApply resumes a persisted non-terminal apply after the
// generic jobs recovery has released its database lock. A missing manifest is
// treated as safe only in phases that precede every instance mutation.
func (d *Driver) RecoverRuntimeUpdateApply(ctx context.Context, instance registry.Instance) error {
	status, err := readRuntimeUpdateApplyStatus(instance.DataDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if status.Phase == RuntimeUpdateApplySucceeded || status.Phase == RuntimeUpdateApplyFailedRolledBack {
		manifest, manifestErr := readRuntimeUpdateRecoveryManifest(instance.DataDir, status.ApplyID)
		if manifestErr != nil || !validRuntimeUpdateRecoveryManifest(instance, status, manifest) {
			return nil
		}
		if runtimeDocker, ok := d.docker.(RuntimeUpdateApplyDockerService); ok && runtimeUpdateAuthSnapshotVolumeCreated(manifest) {
			_ = runtimeDocker.RuntimeRemoveSnapshotVolume(ctx, instance.DataDir, manifest.Project, manifest.SnapshotVolume)
		}
		_ = os.RemoveAll(runtimeUpdateRecoveryDir(instance.DataDir, manifest.ApplyID))
		return nil
	}
	if status.Phase == RuntimeUpdateApplyRollbackFailed {
		return nil
	}
	runtimeDocker, dockerOK := d.docker.(RuntimeUpdateApplyDockerService)
	manifest, err := readRuntimeUpdateRecoveryManifest(instance.DataDir, status.ApplyID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && status.ResumeAfterRepair && runtimeUpdateApplyPreMutationPhase(status.Phase) {
			if !dockerOK {
				return d.markRuntimeUpdateRecoveryUncertain(instance, status, "Docker 恢复能力不可用，无法继续修复后的升级复检。")
			}
			return d.recoverRuntimeUpdateAfterRepair(ctx, runtimeDocker, instance, status, status.Phase == RuntimeUpdateApplyResumingUpgrade)
		}
		if errors.Is(err, os.ErrNotExist) && runtimeUpdateApplyPreMutationPhase(status.Phase) {
			return d.finishRuntimeUpdateRecoveryBeforeChange(ctx, instance, status)
		}
		return d.markRuntimeUpdateRecoveryUncertain(instance, status, "私有恢复材料缺失，已禁止猜测性恢复。")
	}
	if !validRuntimeUpdateRecoveryManifest(instance, status, manifest) {
		return d.markRuntimeUpdateRecoveryUncertain(instance, status, "私有恢复清单与实例或该事务记录的版本对不一致。")
	}
	if !runtimeUpdateMutationStarted(manifest) {
		if status.ResumeAfterRepair {
			if !dockerOK {
				return d.markRuntimeUpdateRecoveryUncertain(instance, status, "Docker 恢复能力不可用，无法继续修复后的升级复检。")
			}
			if err := os.RemoveAll(runtimeUpdateRecoveryDir(instance.DataDir, manifest.ApplyID)); err != nil {
				return d.markRuntimeUpdateRecoveryUncertain(instance, status, "无法清理尚未发生实例修改的重试恢复目录。")
			}
			return d.recoverRuntimeUpdateAfterRepair(ctx, runtimeDocker, instance, status, status.Phase == RuntimeUpdateApplyResumingUpgrade)
		}
		return d.finishRuntimeUpdateRecoveryBeforeChange(ctx, instance, status)
	}
	if !dockerOK {
		return d.markRuntimeUpdateRecoveryUncertain(instance, status, "Docker 恢复能力不可用。")
	}
	if status.ResumeAfterRepair {
		// A repaired retry deliberately keeps this marker while its recovery
		// manifest still has no mutation intent, so a restart can continue the
		// same apply. Once an intent is durable, the runner clears its in-memory
		// marker; if the prior status write still has true, normal transaction
		// rollback is safer than starting another upgrade over a partial change.
		status.ResumeAfterRepair = false
		if err := writeRuntimeUpdateApplyStatus(instance.DataDir, status); err != nil {
			return err
		}
	}
	if err := validateRuntimeUpdateRecoveryMaterials(instance.DataDir, manifest); err != nil {
		return d.markRuntimeUpdateRecoveryUncertain(instance, status, "私有恢复材料不完整或校验失败，已拒绝自动修改实例。")
	}
	status.CreatedBy = manifest.ActorID
	job, err := d.jobs.Start(ctx, jobs.Spec{Type: RuntimeUpdateApplyJobType, DisplayName: "恢复 Junimo 运行组件升级", TargetType: "instance", TargetID: instance.ID, CreatedBy: manifest.ActorID, Timeout: 2 * time.Hour, Run: func(runCtx context.Context, jobCtx *jobs.Context) error {
		return d.runRuntimeUpdateApply(runCtx, jobCtx, runtimeDocker, instance, status, &manifest)
	}})
	if err != nil {
		return err
	}
	status.JobID = job.ID
	status.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return writeRuntimeUpdateApplyStatus(instance.DataDir, status)
}

func validRuntimeUpdateRecoveryManifest(instance registry.Instance, status RuntimeUpdateApplyStatus, manifest runtimeUpdateRecoveryManifest) bool {
	if manifest.SchemaVersion < 1 || manifest.SchemaVersion > 3 || !runtimeUpdateApplyIDPattern.MatchString(manifest.ApplyID) || manifest.ApplyID != status.ApplyID || manifest.Project != strings.ToLower(filepath.Base(filepath.Clean(instance.DataDir))) {
		return false
	}
	expectedSnapshot := manifest.Project + "_anxi-junimo-update-" + strings.TrimPrefix(manifest.ApplyID, "apply_") + "-steam-session"
	if manifest.SchemaVersion >= 3 && (manifest.OriginalServerVersion != status.Current.Server.Tag || manifest.TargetServerVersion != status.Target.Server.Tag) {
		return false
	}
	return manifest.SnapshotVolume == expectedSnapshot && manifest.Target == status.Selected &&
		containsString(status.Target.Server.TrustedCandidates, manifest.Target.Server.Image) &&
		containsString(status.Target.SteamAuth.TrustedCandidates, manifest.Target.SteamAuth.Image) &&
		runtimeImageDigestPattern.MatchString(manifest.Target.Server.Digest) && runtimeImageDigestPattern.MatchString(manifest.Target.SteamAuth.Digest) &&
		runtimeImageDigestPattern.MatchString(manifest.Target.Server.ImageID) && runtimeImageDigestPattern.MatchString(manifest.Target.SteamAuth.ImageID) &&
		runtimeImageDigestPattern.MatchString(manifest.OriginalServer.Digest) && runtimeImageDigestPattern.MatchString(manifest.OriginalAuth.Digest) &&
		runtimeImageDigestPattern.MatchString(manifest.OriginalServer.ImageID) && runtimeImageDigestPattern.MatchString(manifest.OriginalAuth.ImageID)
}

func runtimeUpdateApplyPreMutationPhase(phase string) bool {
	return phase == RuntimeUpdateApplyChecking || phase == RuntimeUpdateApplyPulling || phase == RuntimeUpdateApplyBackingUp || phase == RuntimeUpdateApplyResumingUpgrade
}

func (d *Driver) finishRuntimeUpdateRecoveryBeforeChange(ctx context.Context, instance registry.Instance, status RuntimeUpdateApplyStatus) error {
	now := time.Now().UTC().Format(time.RFC3339)
	status.Phase, status.Progress = RuntimeUpdateApplyFailedRolledBack, 100
	status.ErrorCode, status.Error = "panel_restart_before_change", "Panel 重启发生在实例修改前；实例保持原状。"
	status.ServerRunning, status.ManualAction = status.ServerWasRunning, ""
	status.UpdatedAt, status.FinishedAt = now, now
	status.Logs = append(status.Logs, RuntimeUpdateDryRunLog{At: now, Level: "warning", Message: status.Error})
	if err := writeRuntimeUpdateApplyStatus(instance.DataDir, status); err != nil {
		return err
	}
	d.auditRuntimeUpdateTerminal(ctx, instance.ID, status)
	if runtimeUpdateApplyIDPattern.MatchString(status.ApplyID) {
		_ = os.RemoveAll(runtimeUpdateRecoveryDir(instance.DataDir, status.ApplyID))
	}
	return nil
}

func containsString(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func runtimeUpdateApplyTerminal(phase string) bool {
	return phase == RuntimeUpdateApplySucceeded || phase == RuntimeUpdateApplyFailedRolledBack || phase == RuntimeUpdateApplyRollbackFailed
}

func (d *Driver) RuntimeUpdateApplyInProgress(instance registry.Instance) bool {
	status, err := readRuntimeUpdateApplyStatus(instance.DataDir)
	return err == nil && status.Phase != "idle" && (!runtimeUpdateApplyTerminal(status.Phase) || status.Phase == RuntimeUpdateApplyRollbackFailed)
}

func (d *Driver) markRuntimeUpdateRecoveryUncertain(instance registry.Instance, status RuntimeUpdateApplyStatus, message string) error {
	status.Phase, status.Progress, status.ErrorCode, status.Error = RuntimeUpdateApplyRollbackFailed, 100, "recovery_state_uncertain", message
	status.ManualAction = "请保留实例目录并由管理员人工核对 .env、容器镜像 digest 与私有恢复材料；不要自动重试。"
	status.UpdatedAt, status.FinishedAt = time.Now().UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339)
	return writeRuntimeUpdateApplyStatus(instance.DataDir, status)
}

func newRuntimeApplyID() string {
	var value [12]byte
	if _, err := rand.Read(value[:]); err != nil {
		return fmt.Sprintf("apply_%024x", uint64(time.Now().UnixNano()))
	}
	return "apply_" + hex.EncodeToString(value[:])
}
func runtimeUpdateApplyStatusPath(dataDir string) string {
	return filepath.Join(dataDir, ".local-container", "junimo-update", "apply-status.json")
}
func runtimeUpdateRecoveryDir(dataDir, applyID string) string {
	return filepath.Join(dataDir, ".local-container", "junimo-update", "recovery", applyID)
}

func readRuntimeUpdateApplyStatus(dataDir string) (RuntimeUpdateApplyStatus, error) {
	data, err := readRuntimeStatusFile(runtimeUpdateApplyStatusPath(dataDir))
	if err != nil {
		return RuntimeUpdateApplyStatus{}, err
	}
	var status RuntimeUpdateApplyStatus
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
func writeRuntimeUpdateApplyStatus(dataDir string, status RuntimeUpdateApplyStatus) error {
	return writePrivateRuntimeUpdateJSON(runtimeUpdateApplyStatusPath(dataDir), status)
}
func writeRuntimeUpdateRecoveryManifest(dataDir string, manifest runtimeUpdateRecoveryManifest) error {
	return writePrivateRuntimeUpdateJSON(filepath.Join(runtimeUpdateRecoveryDir(dataDir, manifest.ApplyID), "manifest.json"), manifest)
}
func readRuntimeUpdateRecoveryManifest(dataDir, applyID string) (runtimeUpdateRecoveryManifest, error) {
	data, err := os.ReadFile(filepath.Join(runtimeUpdateRecoveryDir(dataDir, applyID), "manifest.json"))
	if err != nil {
		return runtimeUpdateRecoveryManifest{}, err
	}
	var m runtimeUpdateRecoveryManifest
	err = json.Unmarshal(data, &m)
	return m, err
}
func writePrivateRuntimeUpdateJSON(path string, value any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	_ = os.Chmod(dir, 0o700)
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".runtime-update-*.tmp")
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
