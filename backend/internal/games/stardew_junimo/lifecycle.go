package stardew_junimo

import (
	"context"
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
	lifecycleJobType       = "stardew_lifecycle"
	lifecycleJobTimeout    = 30 * time.Minute
	startServerWaitTimeout = 5 * time.Minute // Docker container reaches "running" within seconds; 5m is ample
	startCheckInterval     = 3 * time.Second
	startProgressInterval  = 30 * time.Second

	// readyStateTimeout covers the entire window from "container running" to "invite code obtained".
	// New-game world generation can take 15+ min, but the invite code may arrive earlier
	// (JunimoServer writes it as soon as the lobby is created, before save load completes).
	readyStateTimeout   = 20 * time.Minute
	readyInviteInterval = 15 * time.Second // how often to read the Junimo invite-code file
	readyLogInterval    = 60 * time.Second // how often to tail container logs
	readySMAPIInterval  = 5 * time.Second  // how often to poll status.json

	inviteCodeTimeout = 30 * time.Second

	backgroundInviteAttempts = 20
	backgroundInviteInterval = 15 * time.Second
	inviteCodeCacheTTL       = 5 * time.Second
)

const restartJobPayload = `{"operation":"restart"}`

var ErrRestartInProgress = errors.New("restart already in progress")

type inviteCodeCacheEntry struct {
	code      string
	expiresAt time.Time
}

type inviteCodeFlight struct {
	done chan struct{}
	code string
	err  error
}

// LifecycleDockerService extends DockerService with lifecycle operations.
type LifecycleDockerService interface {
	DockerService
	ComposeUp(ctx context.Context, dir string) (paneldocker.CommandResult, error)
	ComposeDown(ctx context.Context, dir string) (paneldocker.CommandResult, error)
	ComposeRestart(ctx context.Context, dir string) (paneldocker.CommandResult, error)
	ComposeRestartServices(ctx context.Context, dir string, services ...string) (paneldocker.CommandResult, error)
	ComposeExecPipe(ctx context.Context, dir, service, stdinData string, args ...string) (paneldocker.CommandResult, error)
	ComposeExecTTY(ctx context.Context, dir, service, stdinData string, args ...string) (paneldocker.ComposeExecTTYResult, error)
	ComposeLogs(ctx context.Context, dir string, opts paneldocker.LogsOptions) (paneldocker.CommandResult, error)
}

// lifecycleRunner handles start/stop/restart job execution.
type lifecycleRunner struct {
	driver                    *Driver
	lifecycle                 LifecycleDockerService
	instance                  storage.Instance
	operation                 string // "start", "stop", "restart", "restore_restart"
	actorID                   int64
	newGame                   bool // When true, send "settings newgame --confirm" after server starts.
	newGameConfig             *registry.NewGameConfig
	newGameCommandTimeout     time.Duration
	newGameObservationTimeout time.Duration
	newGamePollInterval       time.Duration
	newGameAPIReadyTimeout    time.Duration
	newGameCatalogTimeout     time.Duration
	commitNewGameModProfile   func(string, string, []string) error
	// rollback-only: the SMAPI updater has restored the exact prior Control Mod
	// and must not replace it before starting the old game-data volume.
	preserveControlMod bool

	// Set when operation == "restore_restart": which backup to restore before
	// (re)starting the server.
	restoreBackupName string
	restoreOverwrite  bool

	steamAuthRefreshAttempted bool
}

// Start implements registry.GameDriver.Start.
// Creates an async job that runs docker compose up and retrieves the invite code.
func (d *Driver) Start(ctx context.Context, req registry.StartRequest) (*registry.Job, error) {
	if d.jobs == nil {
		return nil, fmt.Errorf("driver: job manager not configured")
	}
	d.runtimeUpdateMu.Lock()
	defer d.runtimeUpdateMu.Unlock()
	if err := d.rejectActiveSaveImport(ctx, req.Instance.ID); err != nil {
		return nil, err
	}
	if err := d.rejectActiveFarmhandDelete(ctx, req.Instance.ID); err != nil {
		return nil, err
	}
	if err := d.rejectActiveRuntimeUpdate(ctx, req.Instance.ID); err != nil {
		return nil, err
	}
	if err := d.requireCurrentRuntimeStack(req.Instance); err != nil {
		return nil, err
	}
	ld, ok := d.docker.(LifecycleDockerService)
	if !ok {
		return nil, fmt.Errorf("docker 服务不支持生命周期操作")
	}
	instance, err := d.store.GetInstance(ctx, req.Instance.ID)
	if err != nil {
		return nil, fmt.Errorf("load instance: %w", err)
	}
	if err := d.cancelActiveLifecycleJobs(ctx, req.Instance.ID, "新的启动请求已提交，取消旧的生命周期任务。"); err != nil {
		return nil, err
	}
	jobPayload := ""
	if req.NewGame {
		if req.NewGameConfig == nil {
			return nil, &NewGameTransactionError{Code: "new_game_payload_missing", Message: "新建存档任务缺少规范化配置"}
		}
		payloadData, err := json.Marshal(req.NewGameConfig)
		if err != nil {
			return nil, &NewGameTransactionError{Code: "new_game_payload_invalid", Message: "序列化新建存档任务配置失败", Cause: err}
		}
		jobPayload = string(payloadData)
	}
	runner := &lifecycleRunner{
		driver:        d,
		lifecycle:     ld,
		instance:      instance,
		operation:     "start",
		actorID:       req.ActorID,
		newGame:       req.NewGame,
		newGameConfig: req.NewGameConfig,
	}
	job, err := d.jobs.Start(ctx, jobs.Spec{
		Type:       lifecycleJobType,
		TargetType: "instance",
		TargetID:   req.Instance.ID,
		CreatedBy:  req.ActorID,
		Payload:    jobPayload,
		Timeout:    lifecycleJobTimeout,
		Run:        runner.run,
	})
	if err != nil {
		return nil, fmt.Errorf("start lifecycle job: %w", err)
	}
	return &registry.Job{ID: job.ID}, nil
}

// Stop implements registry.GameDriver.Stop.
func (d *Driver) Stop(ctx context.Context, instance registry.Instance) error {
	if d.jobs == nil {
		return fmt.Errorf("driver: job manager not configured")
	}
	d.runtimeUpdateMu.Lock()
	defer d.runtimeUpdateMu.Unlock()
	if err := d.rejectActiveSaveImport(ctx, instance.ID); err != nil {
		return err
	}
	if err := d.rejectActiveFarmhandDelete(ctx, instance.ID); err != nil {
		return err
	}
	if err := d.rejectActiveRuntimeUpdate(ctx, instance.ID); err != nil {
		return err
	}
	ld, ok := d.docker.(LifecycleDockerService)
	if !ok {
		return fmt.Errorf("docker 服务不支持生命周期操作")
	}
	stored, err := d.store.GetInstance(ctx, instance.ID)
	if err != nil {
		return fmt.Errorf("load instance: %w", err)
	}
	if err := d.cancelActiveLifecycleJobs(ctx, instance.ID, "停止服务器请求已提交，取消旧的生命周期任务。"); err != nil {
		return err
	}
	runner := &lifecycleRunner{
		driver:    d,
		lifecycle: ld,
		instance:  stored,
		operation: "stop",
	}
	if _, err := d.jobs.Start(ctx, jobs.Spec{
		Type:       lifecycleJobType,
		TargetType: "instance",
		TargetID:   instance.ID,
		CreatedBy:  0,
		Timeout:    lifecycleJobTimeout,
		Run:        runner.run,
	}); err != nil {
		return fmt.Errorf("start stop job: %w", err)
	}
	return nil
}

// Restart implements registry.GameDriver.Restart.
func (d *Driver) Restart(ctx context.Context, instance registry.Instance) error {
	if d.jobs == nil {
		return fmt.Errorf("driver: job manager not configured")
	}
	d.runtimeUpdateMu.Lock()
	defer d.runtimeUpdateMu.Unlock()
	if err := d.rejectActiveSaveImport(ctx, instance.ID); err != nil {
		return err
	}
	if err := d.rejectActiveFarmhandDelete(ctx, instance.ID); err != nil {
		return err
	}
	if err := d.rejectActiveRuntimeUpdate(ctx, instance.ID); err != nil {
		return err
	}
	ld, ok := d.docker.(LifecycleDockerService)
	if !ok {
		return fmt.Errorf("docker 服务不支持生命周期操作")
	}
	stored, err := d.store.GetInstance(ctx, instance.ID)
	if err != nil {
		return fmt.Errorf("load instance: %w", err)
	}
	if err := d.rejectActiveRestart(ctx, instance.ID); err != nil {
		return err
	}
	if err := d.cancelActiveLifecycleJobs(ctx, instance.ID, "重启服务器请求已提交，取消旧的生命周期任务。"); err != nil {
		return err
	}
	runner := &lifecycleRunner{
		driver:    d,
		lifecycle: ld,
		instance:  stored,
		operation: "restart",
	}
	if _, err := d.jobs.Start(ctx, jobs.Spec{
		Type:       lifecycleJobType,
		TargetType: "instance",
		TargetID:   instance.ID,
		CreatedBy:  0,
		Payload:    restartJobPayload,
		Timeout:    lifecycleJobTimeout,
		Run:        runner.run,
	}); err != nil {
		return fmt.Errorf("start restart job: %w", err)
	}
	return nil
}

func (d *Driver) rejectActiveRestart(ctx context.Context, instanceID string) error {
	active, err := d.jobs.Active(ctx, storage.ListActiveJobsFilter{
		TargetType: "instance",
		TargetID:   instanceID,
		Types:      []string{lifecycleJobType},
	})
	if err != nil {
		return fmt.Errorf("list active lifecycle jobs: %w", err)
	}
	for _, job := range active {
		if job.Payload.Valid && job.Payload.String == restartJobPayload {
			return ErrRestartInProgress
		}
	}
	return nil
}

// RestoreBackupWithRestart runs stop -> restore -> start as a single async
// lifecycle job. It exists for the case where the admin wants to restore a
// backup while the server is currently running: rather than making them
// manually stop the server first, this submits one job that stops it,
// restores the backup on disk, and starts it again — tracked by the caller
// exactly like a plain Start/Stop/Restart job (same jobId, same job log /
// SSE polling UI).
func (d *Driver) RestoreBackupWithRestart(ctx context.Context, instance registry.Instance, backupName string, overwrite bool, actorID int64) (*registry.Job, error) {
	if d.jobs == nil {
		return nil, fmt.Errorf("driver: job manager not configured")
	}
	d.runtimeUpdateMu.Lock()
	defer d.runtimeUpdateMu.Unlock()
	if err := d.rejectActiveSaveImport(ctx, instance.ID); err != nil {
		return nil, err
	}
	if err := d.rejectActiveFarmhandDelete(ctx, instance.ID); err != nil {
		return nil, err
	}
	if err := d.rejectActiveRuntimeUpdate(ctx, instance.ID); err != nil {
		return nil, err
	}
	ld, ok := d.docker.(LifecycleDockerService)
	if !ok {
		return nil, fmt.Errorf("docker 服务不支持生命周期操作")
	}
	stored, err := d.store.GetInstance(ctx, instance.ID)
	if err != nil {
		return nil, fmt.Errorf("load instance: %w", err)
	}
	if err := d.cancelActiveLifecycleJobs(ctx, instance.ID, "回档请求已提交，取消旧的生命周期任务。"); err != nil {
		return nil, err
	}
	runner := &lifecycleRunner{
		driver:            d,
		lifecycle:         ld,
		instance:          stored,
		operation:         "restore_restart",
		actorID:           actorID,
		restoreBackupName: backupName,
		restoreOverwrite:  overwrite,
	}
	job, err := d.jobs.Start(ctx, jobs.Spec{
		Type:       lifecycleJobType,
		TargetType: "instance",
		TargetID:   instance.ID,
		CreatedBy:  actorID,
		Timeout:    lifecycleJobTimeout,
		Run:        runner.run,
	})
	if err != nil {
		return nil, fmt.Errorf("start restore-restart job: %w", err)
	}
	return &registry.Job{ID: job.ID}, nil
}

func (d *Driver) cancelActiveLifecycleJobs(ctx context.Context, instanceID, reason string) error {
	if d.jobs == nil {
		return nil
	}
	canceled, err := d.jobs.CancelActive(ctx, storage.ListActiveJobsFilter{
		TargetType: "instance",
		TargetID:   instanceID,
		Types:      []string{lifecycleJobType},
	}, "")
	if err != nil {
		return fmt.Errorf("cancel active lifecycle jobs: %w", err)
	}
	for _, job := range canceled {
		_, _ = d.jobs.AppendLog(context.Background(), job.ID, storage.JobLogLevelWarn, reason)
	}
	return nil
}

// run is the job.Runner function for lifecycle operations.
func (r *lifecycleRunner) run(ctx context.Context, jobCtx *jobs.Context) error {
	switch r.operation {
	case "start":
		return r.doStart(ctx, jobCtx)
	case "stop":
		return r.doStop(ctx, jobCtx)
	case "restart":
		return r.doRestart(ctx, jobCtx)
	case "restore_restart":
		return r.doRestoreAndRestart(ctx, jobCtx)
	default:
		return fmt.Errorf("未知的生命周期操作: %s", r.operation)
	}
}

func (r *lifecycleRunner) doStart(ctx context.Context, jobCtx *jobs.Context) (retErr error) {
	_, _ = jobCtx.Info(ctx, "正在启动 Stardew 服务器...")
	imageRef := gameInstallImage(r.instance.DataDir)
	ok, err := r.driver.verifyGameDataVolume(ctx, r.instance.DataDir, imageRef, func(line string) {
		_, _ = jobCtx.Info(ctx, "[verify] "+paneldocker.RedactString(line))
	})
	if err != nil || !ok {
		message := "游戏运行文件不完整，请重新安装或修复。"
		if err != nil {
			message = "验证游戏运行文件失败，请检查任务日志后重试。"
		}
		r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateError,
			message, "install_verification_failed", jobCtx.ID)
		if err != nil {
			return fmt.Errorf("verify game runtime files before start: %w", err)
		}
		return fmt.Errorf("game runtime files are incomplete")
	}

	var newGameTx *newGameTransaction
	var newGameSelection *NewGameModSelection
	composeStarted := false
	newGameCompleted := false
	if r.newGame {
		if r.newGameConfig == nil {
			return &NewGameTransactionError{Code: "new_game_payload_missing", Message: "新建存档任务缺少规范化配置"}
		}
		cfg, cfgErr := NormalizeNewGameConfigWithModded(*r.newGameConfig, true)
		if cfgErr != nil {
			return &NewGameTransactionError{Code: "new_game_payload_invalid", Message: "新建存档任务配置无效", Cause: cfgErr}
		}
		farmType, farmTypeErr := NormalizeNewGameFarmType(cfg.FarmType)
		if farmTypeErr != nil {
			return &NewGameTransactionError{Code: "new_game_payload_invalid", Message: "新建存档 FarmType 无效", Cause: farmTypeErr}
		}
		if !farmType.Builtin {
			selection, selectionErr := ResolveNewGameModSelection(r.instance.DataDir, farmType.ID)
			if selectionErr != nil {
				return selectionErr
			}
			cfg.FarmType = selection.FarmTypeID
			newGameSelection = &selection
		}
		newGameTx, retErr = beginNewGameTransaction(r.instance.DataDir, cfg)
		if retErr != nil {
			return &NewGameTransactionError{Code: "new_game_snapshot_failed", Message: "创建新存档事务快照失败", Cause: retErr}
		}
		defer func() {
			if newGameCompleted || retErr == nil {
				return
			}
			var composeStopErr error
			if composeStarted {
				stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				_, composeStopErr = r.lifecycle.ComposeDown(stopCtx, r.instance.DataDir)
				cancel()
			}
			code := "new_game_failed"
			stage := newGameStateFailed
			var txErr *NewGameTransactionError
			if errors.As(retErr, &txErr) {
				code = txErr.Code
			}
			if newGameTx.record.Stage == newGameStateUnknown || newGameTx.record.Stage == newGameStateAmbiguous {
				stage = newGameTx.record.Stage
			}
			if rollbackErr := newGameTx.rollback(retErr, code, stage); rollbackErr != nil {
				retErr = &NewGameTransactionError{Code: "new_game_rollback_failed", Message: "新建存档失败且回滚未完整完成", Cause: retErr, RollbackError: rollbackErr}
			}
			if composeStopErr != nil {
				newGameTx.record.Stage = newGameStateRollbackFail
				newGameTx.record.Result = "failed"
				newGameTx.record.RollbackCompleted = false
				newGameTx.record.RollbackError = "stop server during rollback: " + paneldocker.RedactString(composeStopErr.Error())
				_ = newGameTx.persist()
				retErr = &NewGameTransactionError{Code: "new_game_rollback_failed", Message: "新建存档失败且停止服务器失败", Cause: retErr, RollbackError: composeStopErr}
			}
			finalCode := code
			if newGameTx.record.Stage == newGameStateRollbackFail {
				finalCode = "new_game_rollback_failed"
			}
			r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateStopped,
				"创建新存档失败: "+paneldocker.RedactString(retErr.Error()), finalCode, jobCtx.ID)
		}()
		if retErr = newGameTx.prepareConfigAndMarker(); retErr != nil {
			return retErr
		}
		_, _ = jobCtx.Info(ctx, fmt.Sprintf("新建存档事务已准备：%s", newGameTx.record.TransactionID))
	}
	r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateStarting, "正在启动服务器...", "starting", jobCtx.ID)

	if err := r.ensureJunimoServerMod(ctx, jobCtx); err != nil {
		r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateStopped,
			"同步 JunimoServer 官方 Mod 失败: "+err.Error(), "junimo_server_mod_failed", jobCtx.ID)
		return err
	}

	// Ensure the latest SMAPI mod DLL is deployed (idempotent; needed for IP direct-connect).
	if !r.preserveControlMod {
		if err := installSMAPIMod(r.instance.DataDir); err != nil {
			_, _ = jobCtx.Info(ctx, fmt.Sprintf("警告：SMAPI mod 部署失败（不影响启动）：%v", err))
		}
	}

	// Ensure IP direct-connect is enabled by default, including for saves created
	// before this default existed. Invite codes (Steam SDR / Galaxy P2P) can stall
	// at "n/a", so IP direct-connect must be available as the reliable join path.
	if err := EnsureServerSettingsDefaults(r.instance.DataDir); err != nil {
		_, _ = jobCtx.Info(ctx, fmt.Sprintf("警告：确保 IP 直连默认设置失败（不影响启动）：%v", err))
	}
	if language, err := EnsureGameLanguagePreferences(r.instance.DataDir); err != nil {
		r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateStopped,
			"同步服务器游戏语言失败: "+err.Error(), "game_language_sync_failed", jobCtx.ID)
		return fmt.Errorf("sync game language before start: %w", err)
	} else {
		_, _ = jobCtx.Info(ctx, "服务器游戏语言已同步："+language.LanguageCode)
	}
	r.clearRuntimeControlSnapshots(ctx, jobCtx)

	if changed, err := EnsureServerContEnvFix(r.instance.DataDir); err != nil {
		_, _ = jobCtx.Info(ctx, fmt.Sprintf("warning: ensure JunimoServer static init compatibility mounts failed: %v", err))
	} else if changed {
		_, _ = jobCtx.Info(ctx, "JunimoServer static init compatibility mounts have been applied.")
	}

	if r.newGame {
		var modPrepareErr error
		if newGameSelection == nil {
			modPrepareErr = ApplyNewSaveDefaultModState(r.instance.DataDir)
		} else {
			prepared, err := ApplyNewGameModSelectionState(r.instance.DataDir, *newGameSelection)
			if err != nil {
				modPrepareErr = err
			} else {
				newGameSelection = &prepared
				newGameTx.record.ModSelection = &prepared
				newGameTx.record.EnabledModKeys = append([]string{}, prepared.EnabledModKeys...)
				newGameTx.record.RequestedFarmType = prepared.FarmTypeID
				newGameTx.record.Config.FarmType = prepared.FarmTypeID
				modPrepareErr = newGameTx.persist()
			}
		}
		if modPrepareErr != nil {
			r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateStopped,
				"prepare new-save Mod set failed: "+modPrepareErr.Error(), "farm_dependencies_missing", jobCtx.ID)
			return modPrepareErr
		}
		if err := newGameTx.mark(newGameStateModsPrepared); err != nil {
			return &NewGameTransactionError{Code: "new_game_state_write_failed", Message: "记录 Mod 准备状态失败", Cause: err}
		}
		if err := newGameTx.prepareRuntimeCatalogRequest(); err != nil {
			return err
		}
		if newGameSelection == nil {
			_, _ = jobCtx.Info(ctx, "New save mod defaults applied: third-party mods are disabled.")
		} else {
			_, _ = jobCtx.Info(ctx, fmt.Sprintf("已准备模组农场 %s 的必要 Mod 集合（%d 个组件）。", newGameSelection.FarmTypeID, len(newGameSelection.EnabledModKeys)))
		}
	} else if activeSaveName := GetActiveSaveName(r.instance.DataDir); activeSaveName != "" {
		if err := ApplyModProfile(r.instance.DataDir, activeSaveName); err != nil {
			r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateStopped,
				"apply save mod profile failed: "+err.Error(), "mod_profile_failed", jobCtx.ID)
			return err
		}
	}
	if quarantined, err := QuarantineSMAPIBundledDuplicates(r.instance.DataDir); err != nil {
		r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateStopped,
			"repair duplicate SMAPI support mods failed: "+err.Error(), "smapi_mod_dedup_failed", jobCtx.ID)
		return err
	} else if len(quarantined) > 0 {
		_, _ = jobCtx.Info(ctx, fmt.Sprintf("已隔离重复的 SMAPI 内置组件：%s。原文件保留在私有隔离目录。", strings.Join(quarantined, "、")))
	}

	result, err := r.lifecycle.ComposeUp(ctx, r.instance.DataDir)
	if err != nil {
		if friendly, ok := r.vncPortUnavailableMessage(result); ok {
			_, _ = jobCtx.Error(ctx, friendly)
			if stderr := strings.TrimSpace(result.Stderr); stderr != "" {
				_, _ = jobCtx.Debug(ctx, "Docker 原始错误："+stderr)
			}
			r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateStopped,
				friendly, "vnc_port_unavailable", jobCtx.ID)
			return errors.New(friendly)
		}
		r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateStopped,
			"启动失败: "+result.Stderr, "start_failed", jobCtx.ID)
		if r.newGame {
			return &NewGameTransactionError{Code: "new_game_compose_start_failed", Message: "新建存档服务器启动失败", Cause: err}
		}
		return fmt.Errorf("docker compose up: %w", err)
	}
	composeStarted = true
	if r.newGame {
		if err := newGameTx.mark(newGameStateComposeUp); err != nil {
			return &NewGameTransactionError{Code: "new_game_state_write_failed", Message: "记录服务器启动状态失败", Cause: err}
		}
	}
	_, _ = jobCtx.Info(ctx, "docker compose up 完成，等待服务器就绪...")

	if err := r.waitForServer(ctx, jobCtx); err != nil {
		r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateStopped,
			"服务器启动失败", "start_failed", jobCtx.ID)
		return err
	}
	r.clearStaleInviteCode(ctx, jobCtx)

	// Container is running; poll for invite code and SMAPI status concurrently.
	// JunimoServer writes the invite code as soon as the lobby is created (before save load),
	// so we must not gate invite-code polling on SMAPI save-loaded.

	// If this is a new-game request, send "settings newgame --confirm" once SMAPI is ready.
	// This creates a fresh save using the server-settings.json values without deleting old saves.
	if r.newGame {
		if err := r.sendNewGameCommand(ctx, jobCtx, newGameTx); err != nil {
			return err
		}
		newGameCompleted = true
	}

	r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateRunning,
		"服务器容器已启动，正在初始化游戏...", "server_initializing", jobCtx.ID)

	_, _ = jobCtx.Info(ctx, fmt.Sprintf("服务器已启动；邀请码将后台尝试获取，最多 %d 次，不影响 IP 直连。", backgroundInviteAttempts))
	r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateRunning,
		"服务器运行中（邀请码后台获取中）", "running", jobCtx.ID)
	r.startInviteCodePolling()

	// Clear the "restart required" flag now that the server is running with latest mods.
	_ = ClearModsRestartRequired(r.instance.DataDir)
	return nil
}

func (r *lifecycleRunner) vncPortUnavailableMessage(result paneldocker.CommandResult) (string, bool) {
	port := r.currentVNCPort()
	combined := strings.ToLower(result.Stderr + "\n" + result.Stdout)
	if !looksLikePortBindFailure(combined) {
		return "", false
	}
	if port != "" && !strings.Contains(combined, ":"+port) && !strings.Contains(combined, "0.0.0.0:"+port) {
		return "", false
	}
	if port == "" {
		port = "当前"
	}
	return fmt.Sprintf("VNC 端口 %s 被占用或被系统保留，请更换 VNC 端口后重试。", port), true
}

func (r *lifecycleRunner) currentVNCPort() string {
	values, err := sjconfig.ReadEnvFile(filepath.Join(r.instance.DataDir, ".env"))
	if err != nil {
		return ""
	}
	port := strings.TrimSpace(values["VNC_PORT"])
	if port == "" {
		port = sjconfig.EmptyEnvTemplate()["VNC_PORT"]
	}
	return port
}

func looksLikePortBindFailure(text string) bool {
	if text == "" {
		return false
	}
	hasPortContext := strings.Contains(text, "ports are not available") ||
		strings.Contains(text, "port is already allocated") ||
		strings.Contains(text, "bind for 0.0.0.0") ||
		strings.Contains(text, "listen tcp")
	hasBindFailure := strings.Contains(text, "bind") ||
		strings.Contains(text, "forbidden by its access permissions") ||
		strings.Contains(text, "address already in use") ||
		strings.Contains(text, "already allocated")
	return hasPortContext && hasBindFailure
}

func (r *lifecycleRunner) doStop(ctx context.Context, jobCtx *jobs.Context) error {
	_, _ = jobCtx.Info(ctx, "正在停止 Stardew 服务器...")
	r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateStopped, "正在停止...", "stopping", jobCtx.ID)

	_, err := r.lifecycle.ComposeDown(ctx, r.instance.DataDir)
	if err != nil {
		r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateError,
			"停止失败，请检查 Docker/Compose 状态。", "stop_failed", jobCtx.ID)
		return fmt.Errorf("docker compose down: %w", err)
	}
	r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateStopped, "服务器已停止", "stopped", jobCtx.ID)
	_ = ClearModsRestartRequired(r.instance.DataDir)
	_, _ = jobCtx.Info(ctx, "服务器已停止")
	return nil
}

// doRestoreAndRestart stops the server (if it was running), restores the
// requested backup onto disk, then starts the server again. Reuses doStop/
// doStart verbatim rather than duplicating their compose/mod-sync/invite-code
// logic, so this stays in lockstep with any future change to plain start/stop.
func (r *lifecycleRunner) doRestoreAndRestart(ctx context.Context, jobCtx *jobs.Context) error {
	wasRunning := r.instance.State == storage.InstanceStateRunning || r.instance.State == storage.InstanceStateStarting
	if wasRunning {
		if err := r.doStop(ctx, jobCtx); err != nil {
			return err
		}
	}

	_, _ = jobCtx.Info(ctx, fmt.Sprintf("正在回档到备份 %s...", r.restoreBackupName))
	saveName, err := RestoreBackup(r.instance.DataDir, r.restoreBackupName, r.restoreOverwrite)
	if err != nil {
		r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateStopped,
			"回档失败: "+err.Error(), "restore_failed", jobCtx.ID)
		return err
	}
	_, _ = jobCtx.Info(ctx, fmt.Sprintf("回档完成，当前存档已切换为 %s", saveName))

	if !wasRunning {
		r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateStopped, "回档完成", "restored", jobCtx.ID)
		return nil
	}

	_, _ = jobCtx.Info(ctx, "正在重新启动服务器...")
	return r.doStart(ctx, jobCtx)
}

func (r *lifecycleRunner) doRestart(ctx context.Context, jobCtx *jobs.Context) error {
	_, _ = jobCtx.Info(ctx, "正在重启 Stardew 服务器...")
	r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateStarting, "正在重启...", "restarting", jobCtx.ID)
	r.removeInviteCodeFile(ctx, jobCtx)
	r.clearRuntimeControlSnapshots(ctx, jobCtx)

	if err := r.ensureJunimoServerMod(ctx, jobCtx); err != nil {
		r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateError,
			"同步 JunimoServer 官方 Mod 失败: "+err.Error(), "junimo_server_mod_failed", jobCtx.ID)
		return err
	}

	composeConfigChanged := false
	if changed, err := EnsureServerContEnvFix(r.instance.DataDir); err != nil {
		_, _ = jobCtx.Info(ctx, fmt.Sprintf("warning: ensure JunimoServer static init compatibility mounts failed: %v", err))
	} else if changed {
		composeConfigChanged = true
		_, _ = jobCtx.Info(ctx, "JunimoServer static init compatibility mounts have been applied.")
	}
	if quarantined, err := QuarantineSMAPIBundledDuplicates(r.instance.DataDir); err != nil {
		r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateError,
			"repair duplicate SMAPI support mods failed: "+err.Error(), "smapi_mod_dedup_failed", jobCtx.ID)
		return err
	} else if len(quarantined) > 0 {
		_, _ = jobCtx.Info(ctx, fmt.Sprintf("已隔离重复的 SMAPI 内置组件：%s。原文件保留在私有隔离目录。", strings.Join(quarantined, "、")))
	}

	var result paneldocker.CommandResult
	var err error
	if composeConfigChanged {
		result, err = r.lifecycle.ComposeUp(ctx, r.instance.DataDir)
	} else {
		result, err = r.lifecycle.ComposeRestartServices(ctx, r.instance.DataDir, "server")
	}
	if err != nil {
		r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateError,
			"重启失败: "+result.Stderr, "restart_failed", jobCtx.ID)
		return fmt.Errorf("docker compose restart server: %w", err)
	}
	_, _ = jobCtx.Info(ctx, "重启完成，等待服务器就绪...")

	if err := r.waitForServer(ctx, jobCtx); err != nil {
		r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateError,
			"重启后服务器未就绪", "restart_timeout", jobCtx.ID)
		return err
	}
	r.clearStaleInviteCode(ctx, jobCtx)

	_, _ = jobCtx.Info(ctx, fmt.Sprintf("服务器已重启；邀请码将后台尝试获取，最多 %d 次，不影响 IP 直连。", backgroundInviteAttempts))
	r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateRunning,
		"服务器运行中（邀请码后台获取中）", "running", jobCtx.ID)
	r.startInviteCodePolling()

	// Clear the "restart required" flag now that the server is running with latest mods.
	_ = ClearModsRestartRequired(r.instance.DataDir)
	return nil
}

func (r *lifecycleRunner) ensureJunimoServerMod(ctx context.Context, jobCtx *jobs.Context) error {
	values, err := sjconfig.ReadEnvFile(filepath.Join(r.instance.DataDir, ".env"))
	if err != nil {
		return fmt.Errorf("read runtime config for JunimoServer sync: %w", err)
	}
	expectedVersion := strings.TrimSpace(values["IMAGE_VERSION"])
	if expectedVersion == "" {
		expectedVersion = TestedImageTag
	}
	installedDir := junimoServerModDir(r.instance.DataDir)
	installedVersion, versionErr := readJunimoServerModVersion(installedDir)
	if versionErr == nil {
		versionErr = validateExtractedJunimoServerMod(installedDir, expectedVersion)
	}
	if versionErr == nil {
		return nil
	}
	imageRef := serverImageRef(r.instance.DataDir)
	if jobCtx != nil {
		if versionErr == nil {
			_, _ = jobCtx.Info(ctx, fmt.Sprintf("正在把 JunimoServer 官方 Mod 从 %s 同步到 %s...", installedVersion, expectedVersion))
		} else {
			_, _ = jobCtx.Info(ctx, "正在同步 JunimoServer 官方 Mod...")
		}
	}
	root := filepath.Join(r.instance.DataDir, ".local-container", "junimo-mod-sync")
	if err := os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	workDir, err := os.MkdirTemp(root, "sync-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workDir)
	extractedDir, err := extractJunimoServerMod(ctx, r.lifecycle, imageRef, workDir, expectedVersion)
	if err != nil {
		return fmt.Errorf("sync JunimoServer mod from %s: %w", imageRef, err)
	}
	originalPresent, err := replaceJunimoServerMod(r.instance.DataDir, extractedDir, filepath.Join(workDir, runtimeOriginalJunimoDir))
	if err != nil {
		return err
	}
	if originalPresent && jobCtx != nil {
		_, _ = jobCtx.Info(ctx, fmt.Sprintf("JunimoServer 官方 Mod 已更新到 %s。", expectedVersion))
	}
	return nil
}

func serverImageRef(dataDir string) string {
	values, err := sjconfig.ReadEnvFile(filepath.Join(dataDir, ".env"))
	if err != nil {
		return "sdvd/server:" + TestedImageTag
	}
	if image := strings.TrimSpace(values["SERVER_IMAGE"]); image != "" {
		return image
	}
	tag := strings.TrimSpace(values["IMAGE_VERSION"])
	if tag == "" {
		tag = TestedImageTag
	}
	return "sdvd/server:" + tag
}

// waitForServer polls docker compose ps until the `server` container is in running state.
// Returns early if the container exits (non-recoverable) instead of waiting for full timeout.
func (r *lifecycleRunner) waitForServer(ctx context.Context, jobCtx *jobs.Context) error {
	startTime := time.Now()
	deadline := startTime.Add(startServerWaitTimeout)
	lastProgress := time.Time{} // zero value → log on first iteration

	for time.Now().Before(deadline) {
		ps, err := r.lifecycle.ComposePs(ctx, r.instance.DataDir)
		if err == nil {
			for _, svc := range ps.Services {
				if svc.Service != "server" {
					continue
				}
				state := strings.ToLower(svc.State)
				// Accept either State=="running" or Status starting with "Up" (Compose v5 compat).
				if state == "running" || strings.HasPrefix(strings.ToLower(svc.Status), "up") {
					_, _ = jobCtx.Info(ctx, fmt.Sprintf("server 容器已就绪（%s）", svc.Status))
					return nil
				}
				// Container exited — no point waiting further.
				if state == "exited" || state == "dead" {
					return fmt.Errorf("server 容器已退出（ExitCode=%d，Status=%s）；请检查 docker compose logs server", svc.ExitCode, svc.Status)
				}
			}
			if time.Since(lastProgress) >= startProgressInterval {
				elapsed := int(time.Since(startTime).Seconds())
				remaining := int(deadline.Sub(time.Now()).Seconds())
				if len(ps.Services) == 0 {
					_, _ = jobCtx.Info(ctx, fmt.Sprintf(
						"等待容器启动...（%ds 已过，最多剩 %ds）", elapsed, remaining))
				} else {
					for _, svc := range ps.Services {
						_, _ = jobCtx.Info(ctx, fmt.Sprintf(
							"[状态] %s: %s（%s）", svc.Service, svc.State, svc.Status))
					}
				}
				lastProgress = time.Now()
			}
		} else if time.Since(lastProgress) >= startProgressInterval {
			elapsed := int(time.Since(startTime).Seconds())
			_, _ = jobCtx.Info(ctx, fmt.Sprintf(
				"等待服务器（%ds），docker compose ps 出错：%v", elapsed, err))
			lastProgress = time.Now()
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(startCheckInterval):
		}
	}
	_, _ = jobCtx.Info(ctx, fmt.Sprintf("等待 server 容器超时（%v），请检查 docker compose logs server", startServerWaitTimeout))
	return fmt.Errorf("等待 server 容器超时（%v）", startServerWaitTimeout)
}

// readSMAPIStatus reads state from the SMAPI mod's status.json in the control directory.
// Returns "" when the file does not exist or cannot be parsed.
func readSMAPIStatus(dataDir string) string {
	raw, err := os.ReadFile(filepath.Join(controlDir(dataDir), "status.json"))
	if err != nil {
		return ""
	}
	var s struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return s.State
}

// waitForReadyState combines SMAPI status polling, invite-code polling, and container log
// tailing into a single loop.  It returns the invite code as soon as it is available.
//
// JunimoServer writes the invite code to /tmp/invite-code.txt the moment the Steam/Galaxy
// lobby is created — which happens *before* any save is loaded.  So invite-code polling
// must not be gated on SMAPI save-loaded; both are checked concurrently in the same loop.
func (r *lifecycleRunner) waitForReadyState(ctx context.Context, jobCtx *jobs.Context) string {
	deadline := time.Now().Add(readyStateTimeout)
	lastSMAPIState := ""
	lastInviteAttempt := time.Time{} // zero = try immediately on first iteration
	lastLogTail := time.Now()
	inviteAttempt := 0

	for time.Now().Before(deadline) {
		// ── SMAPI status (for progress logging only) ──────────────────────
		state := readSMAPIStatus(r.instance.DataDir)
		if state != lastSMAPIState {
			lastSMAPIState = state
			switch state {
			case "booting":
				_, _ = jobCtx.Info(ctx, "[SMAPI] 游戏进程启动中...")
			case "launched":
				_, _ = jobCtx.Info(ctx, "[SMAPI] 游戏已启动，正在创建或加载存档...")
			case "save-loaded":
				_, _ = jobCtx.Info(ctx, "[SMAPI] 存档已加载。")
			case "":
				// not yet written — silent
			default:
				_, _ = jobCtx.Info(ctx, fmt.Sprintf("[SMAPI] 状态：%s", state))
			}
		}

		// ── Invite code (try every readyInviteInterval) ───────────────────
		if time.Since(lastInviteAttempt) >= readyInviteInterval {
			inviteAttempt++
			lastInviteAttempt = time.Now()
			code, err := r.fetchInviteCode(ctx)
			if err == nil && code != "" {
				r.markSteamAuthUsableFromInviteCode(jobCtx)
				_, _ = jobCtx.Info(ctx, fmt.Sprintf("邀请码已就绪（第 %d 次）：%s", inviteAttempt, code))
				return code
			}
			// Only log every 4 attempts (~1 min) to avoid flooding.
			if inviteAttempt == 1 || inviteAttempt%4 == 0 {
				remaining := int(deadline.Sub(time.Now()).Seconds())
				_, _ = jobCtx.Info(ctx, fmt.Sprintf(
					"等待邀请码（第 %d 次，剩余 %ds，SMAPI=%s）",
					inviteAttempt, remaining, lastSMAPIState))
			}
		}

		// ── Container log tail (every readyLogInterval) ───────────────────
		if time.Since(lastLogTail) >= readyLogInterval {
			lastLogTail = time.Now()
			r.tailServerLogs(ctx, jobCtx, 30)
		}

		select {
		case <-ctx.Done():
			return ""
		case <-time.After(readySMAPIInterval):
		}
	}

	// Final diagnostics.
	r.tailServerLogs(ctx, jobCtx, 50)
	_, _ = jobCtx.Info(ctx, fmt.Sprintf(
		"服务器在 %v 内未就绪（SMAPI 最终状态：%q），尝试最后一次获取邀请码...", readyStateTimeout, lastSMAPIState))
	code, _ := r.fetchInviteCode(ctx)
	if code != "" {
		r.markSteamAuthUsableFromInviteCode(jobCtx)
	}
	return code
}

// tailServerLogs fetches recent server container logs and writes them to the job context.
func (r *lifecycleRunner) tailServerLogs(ctx context.Context, jobCtx *jobs.Context, tail int) {
	logCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	result, err := r.lifecycle.ComposeLogs(logCtx, r.instance.DataDir, paneldocker.LogsOptions{
		Service: "server",
		Tail:    tail,
	})
	if err != nil || strings.TrimSpace(result.Stdout) == "" {
		return
	}
	if serverLogShowsSteamAuthUnavailable(result.Stdout) {
		if err := sjconfig.SetSteamAuthLoggedIn(r.instance.DataDir, false); err != nil {
			_, _ = jobCtx.Warn(ctx, "检测到 steam-auth 授权不可用，但更新授权状态失败。")
		} else {
			_, _ = jobCtx.Warn(ctx, "检测到 steam-auth 容器当前没有可用授权账号，已标记为需要重新登录授权。")
		}
	} else if sjconfig.SteamAuthLoggedIn(r.instance.DataDir) && serverLogShowsSteamAuthServiceNotReady(result.Stdout) {
		r.refreshSteamAuthService(ctx, jobCtx)
	}
	_, _ = jobCtx.Info(ctx, fmt.Sprintf("[server 容器日志 —最后 %d 行]\n%s", tail, result.Stdout))
}

func serverLogShowsSteamAuthUnavailable(output string) bool {
	lower := strings.ToLower(output)
	return containsAny(lower,
		"steam-auth service has no logged-in accounts",
		"steam-auth service has no logged in accounts",
		"steam-auth has no logged-in accounts",
		"steam-auth has no logged in accounts",
		"no logged-in accounts",
		"no logged in accounts",
	)
}

func serverLogShowsSteamAuthServiceNotReady(output string) bool {
	lower := strings.ToLower(output)
	return containsAny(lower,
		"steam-auth service not ready",
		"steam auth service not ready",
		"could not reach steam-auth service",
		"could not reach steam auth service",
		"steam auth service request failed",
	)
}

func (r *lifecycleRunner) refreshSteamAuthService(ctx context.Context, jobCtx *jobs.Context) {
	if r.steamAuthRefreshAttempted {
		return
	}
	r.steamAuthRefreshAttempted = true
	restartCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	result, err := r.lifecycle.ComposeRestartServices(restartCtx, r.instance.DataDir, "steam-auth")
	if err != nil {
		detail := dockerResultDetail(result)
		if detail != "" {
			detail = "：" + detail
		}
		_, _ = jobCtx.Warn(ctx, "检测到 steam-auth 服务暂未就绪；已有授权标记，但自动刷新 steam-auth 服务失败"+detail)
		return
	}
	_, _ = jobCtx.Warn(ctx, "检测到 steam-auth 服务暂未就绪；已有授权标记，已自动刷新 steam-auth 服务。")
}

func (r *lifecycleRunner) markSteamAuthUsableFromInviteCode(jobCtx *jobs.Context) {
	if sjconfig.SteamAuthLoggedIn(r.instance.DataDir) {
		return
	}
	if err := sjconfig.SetSteamAuthLoggedIn(r.instance.DataDir, true); err != nil {
		if jobCtx != nil {
			_, _ = jobCtx.Warn(context.Background(), "邀请码已获取，但记录 Steam 授权状态失败。")
		}
	}
}

func (r *lifecycleRunner) startInviteCodePolling() {
	driver := r.driver
	if driver == nil {
		return
	}
	runner := &lifecycleRunner{
		driver:    driver,
		lifecycle: r.lifecycle,
		instance:  r.instance,
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(backgroundInviteAttempts)*(backgroundInviteInterval+inviteCodeTimeout))
		defer cancel()
		runner.pollInviteCodeAttempts(ctx, backgroundInviteAttempts, backgroundInviteInterval)
	}()
}

func (r *lifecycleRunner) pollInviteCodeAttempts(ctx context.Context, attempts int, interval time.Duration) string {
	if attempts <= 0 {
		return ""
	}
	if interval <= 0 {
		interval = backgroundInviteInterval
	}
	for attempt := 1; attempt <= attempts; attempt++ {
		if !r.instanceStillRunning(ctx) {
			if r.driver != nil && r.driver.logger != nil {
				r.driver.logger.Info("invite code background polling stopped because instance is no longer running", "instance", r.instance.ID, "attempt", attempt)
			}
			return ""
		}
		code, err := r.fetchInviteCode(ctx)
		if err == nil && code != "" {
			r.markSteamAuthUsableFromInviteCode(nil)
			if r.driver != nil {
				r.driver.updateDriverPayloadInviteCode(context.Background(), r.instance.ID, code)
				if r.driver.logger != nil {
					r.driver.logger.Info("invite code obtained in background", "instance", r.instance.ID, "attempt", attempt)
				}
			}
			return code
		}
		if r.driver != nil && r.driver.logger != nil && (attempt == 1 || attempt == attempts || attempt%5 == 0) {
			r.driver.logger.Info("invite code not ready in background", "instance", r.instance.ID, "attempt", attempt, "max_attempts", attempts)
		}
		if attempt == attempts {
			break
		}
		select {
		case <-ctx.Done():
			return ""
		case <-time.After(interval):
		}
	}
	if r.driver != nil && r.driver.logger != nil {
		r.driver.logger.Info("invite code background polling finished without code", "instance", r.instance.ID, "attempts", attempts)
	}
	return ""
}

func (r *lifecycleRunner) instanceStillRunning(ctx context.Context) bool {
	if r.driver == nil || r.driver.store == nil {
		return true
	}
	inst, err := r.driver.store.GetInstance(ctx, r.instance.ID)
	if err != nil {
		return true
	}
	switch inst.State {
	case storage.InstanceStateStopped,
		storage.InstanceStateError,
		storage.InstanceStateReadyToStart,
		storage.InstanceStateGameInstalled,
		storage.InstanceStateSaveRequired:
		return false
	default:
		return true
	}
}

func (r *lifecycleRunner) clearRuntimeControlSnapshots(ctx context.Context, jobCtx *jobs.Context) {
	paths := []string{
		filepath.Join(controlDir(r.instance.DataDir), "status.json"),
		filepath.Join(controlDir(r.instance.DataDir), "players.json"),
	}
	removed := false
	for _, path := range paths {
		if err := os.Remove(path); err == nil {
			removed = true
		} else if err != nil && !os.IsNotExist(err) {
			_, _ = jobCtx.Warn(ctx, fmt.Sprintf("清理旧运行状态文件失败：%s: %v", filepath.Base(path), err))
		}
	}
	if removed {
		_, _ = jobCtx.Info(ctx, "已清理上一轮 SMAPI 运行状态快照，等待本次启动写入新状态。")
	}
}

// clearStaleInviteCode removes /tmp/invite-code.txt only when it still contains
// the invite code recorded before this lifecycle operation. This prevents
// docker compose restart/up from reusing a stale /tmp file while avoiding
// deletion of a fresh code that Junimo may have already written.
func (r *lifecycleRunner) clearStaleInviteCode(ctx context.Context, jobCtx *jobs.Context) {
	oldCode := inviteCodeFromPayload(r.instance.DriverPayload)
	if oldCode == "" {
		return
	}
	execCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	result, err := r.lifecycle.ComposeExecPipe(execCtx, r.instance.DataDir, "server",
		"", "cat", "/tmp/invite-code.txt")
	if err != nil {
		return
	}
	current := strings.TrimSpace(result.Stdout)
	if current == "" || current != oldCode {
		return
	}
	r.removeInviteCodeFile(ctx, jobCtx)
}

func (r *lifecycleRunner) removeInviteCodeFile(ctx context.Context, jobCtx *jobs.Context) {
	execCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := r.lifecycle.ComposeExecPipe(execCtx, r.instance.DataDir, "server",
		"", "rm", "-f", "/tmp/invite-code.txt")
	if err == nil && r.driver != nil {
		r.driver.inviteCodeMu.Lock()
		delete(r.driver.inviteCodeCache, r.instance.ID)
		r.driver.inviteCodeMu.Unlock()
	}
	if err == nil && jobCtx != nil {
		_, _ = jobCtx.Info(ctx, "已清理旧邀请码，等待 Junimo 生成新的邀请码...")
	}
}

// fetchInviteCode only reads /tmp/invite-code.txt from the server container.
// Never fall back to attach-cli here: it owns an interactive tmux client and
// leaked processes when browser polling overlapped or disconnected.
func (r *lifecycleRunner) fetchInviteCode(ctx context.Context) (string, error) {
	execCtx, cancel := context.WithTimeout(ctx, inviteCodeTimeout)
	defer cancel()

	catResult, catErr := r.lifecycle.ComposeExecPipe(execCtx, r.instance.DataDir, "server",
		"", "cat", "/tmp/invite-code.txt")
	if catErr != nil {
		return "", fmt.Errorf("read /tmp/invite-code.txt: %w", catErr)
	}
	return strings.TrimSpace(catResult.Stdout), nil
}

// GetInviteCode fetches the invite code for a running instance (used by HTTP handler).
func (d *Driver) GetInviteCode(ctx context.Context, instance registry.Instance) (string, error) {
	ld, ok := d.docker.(LifecycleDockerService)
	if !ok {
		return "", fmt.Errorf("docker 服务不支持生命周期操作")
	}
	stored, err := d.store.GetInstance(ctx, instance.ID)
	if err != nil {
		return "", fmt.Errorf("load instance: %w", err)
	}
	if stored.DriverPhase == importMaintenancePhase {
		return "", &ImportTransactionError{Code: ImportErrorBusy, Message: "invite codes are unavailable during save import maintenance"}
	}
	runner := &lifecycleRunner{
		driver:    d,
		lifecycle: ld,
		instance:  stored,
	}
	code, err := d.getInviteCodeCached(ctx, runner)
	if err == nil && code != "" {
		_ = sjconfig.SetSteamAuthLoggedIn(stored.DataDir, true)
	}
	if err == nil && code == "" {
		return "n/a", nil
	}
	return code, err
}

func (d *Driver) getInviteCodeCached(ctx context.Context, runner *lifecycleRunner) (string, error) {
	now := time.Now()
	d.inviteCodeMu.Lock()
	if cached, ok := d.inviteCodeCache[runner.instance.ID]; ok && now.Before(cached.expiresAt) {
		d.inviteCodeMu.Unlock()
		return cached.code, nil
	}
	if flight, ok := d.inviteCodeFlights[runner.instance.ID]; ok {
		d.inviteCodeMu.Unlock()
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-flight.done:
			return flight.code, flight.err
		}
	}
	flight := &inviteCodeFlight{done: make(chan struct{})}
	d.inviteCodeFlights[runner.instance.ID] = flight
	d.inviteCodeMu.Unlock()

	flight.code, flight.err = runner.fetchInviteCode(ctx)

	d.inviteCodeMu.Lock()
	if flight.err == nil {
		d.inviteCodeCache[runner.instance.ID] = inviteCodeCacheEntry{
			code:      flight.code,
			expiresAt: time.Now().Add(inviteCodeCacheTTL),
		}
	}
	delete(d.inviteCodeFlights, runner.instance.ID)
	close(flight.done)
	d.inviteCodeMu.Unlock()
	return flight.code, flight.err
}

// updateDriverPayloadInviteCode stores the invite code in the instance driver payload.
func (d *Driver) updateDriverPayloadInviteCode(ctx context.Context, instanceID, inviteCode string) {
	if d.store == nil {
		return
	}
	// Get current instance to merge payload.
	inst, err := d.store.GetInstance(ctx, instanceID)
	if err != nil {
		d.logger.Warn("update invite code: load instance", "error", err)
		return
	}
	newPayload := mergeInviteCodeInPayload(inst.DriverPayload, inviteCode)
	_, err = d.store.UpdateInstanceState(ctx, storage.UpdateInstanceStateParams{
		ID:            instanceID,
		State:         inst.State,
		StateMessage:  inst.StateMessage.String,
		DriverPhase:   inst.DriverPhase,
		DriverPayload: newPayload,
	})
	if err != nil {
		d.logger.Warn("update invite code: update state", "error", err)
	}
}

// sendNewGameCommand waits for the JunimoServer HTTP API to be ready, then calls
// POST /newgame to create a fresh save using the current server-settings.json values.
// Existing saves are preserved; junimohost.gameloader.json is updated automatically.
func (r *lifecycleRunner) sendNewGameCommand(ctx context.Context, jobCtx *jobs.Context, tx *newGameTransaction) error {
	if tx == nil {
		return &NewGameTransactionError{Code: "new_game_transaction_missing", Message: "新建存档事务不存在"}
	}
	if err := tx.waitForRuntimeFarmCatalog(ctx, r.newGameCatalogTimeout, r.newGamePollInterval); err != nil {
		return err
	}
	_, _ = jobCtx.Info(ctx, "运行时农场目录已通过 transactionId、Mod 指纹和 FarmType 校验。")
	_, _ = jobCtx.Info(ctx, "等待服务器 API 就绪后创建新存档...")

	// Poll the HTTP API until /status responds (server is up and accepting requests).
	apiURL := "http://localhost:8080/status"
	readyTimeout := r.newGameAPIReadyTimeout
	if readyTimeout <= 0 {
		readyTimeout = 5 * time.Minute
	}
	pollInterval := r.newGamePollInterval
	if pollInterval <= 0 {
		pollInterval = 3 * time.Second
	}
	deadline := time.Now().Add(readyTimeout)
	apiReady := false
	for time.Now().Before(deadline) {
		reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		result, err := r.lifecycle.ComposeExecPipe(reqCtx, r.instance.DataDir, "server",
			"", "curl", "-sf", apiURL)
		cancel()
		if err == nil && result.ExitCode == 0 {
			apiReady = true
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollInterval):
		}
	}
	if !apiReady {
		return &NewGameTransactionError{Code: "new_game_api_not_ready", Message: "服务器 API 在期限内未就绪"}
	}

	_, _ = jobCtx.Info(ctx, "服务器 API 就绪，发送创建新存档请求...")

	// Remember which save (if any) the gameloader currently points at. A fresh install
	// keeps the persistent saves dir, so an old save can still be present; the poll below
	// uses this to tell a genuinely new save apart from a pre-existing one and never
	// report the old save as "created".
	gameloaderFile := gameloaderPath(r.instance.DataDir)
	prevSave := ""
	if data, err := os.ReadFile(gameloaderFile); err == nil {
		var gl struct {
			SaveNameToLoad string `json:"SaveNameToLoad"`
		}
		if json.Unmarshal(data, &gl) == nil {
			prevSave = gl.SaveNameToLoad
		}
	}

	// Call POST /newgame.  JunimoServer reads server-settings.json and creates a new save.
	// The gameloader config is updated automatically.
	//
	// /newgame is synchronous: it generates the whole world before responding, which on a
	// fresh first boot (cold cache, small VM) can take a couple of minutes. Give it a
	// generous timeout so curl is not killed mid-generation. If it still times out, do NOT
	// fail — the server keeps generating the save server-side, so fall through to the
	// save-detection poll below. Failing here instead makes the lifecycle fall back to a
	// pre-existing save (e.g. an old save left in the persistent saves dir), which is
	// exactly the surprising "loaded the wrong save" behaviour.
	commandTimeout := r.newGameCommandTimeout
	if commandTimeout <= 0 {
		commandTimeout = 4 * time.Minute
	}
	var commandErr error
	commandTimedOut := false
	startupSaveDetected := false
	if !tx.record.CommandCalled {
		// Junimo's legacy server-init flow can create the requested save during
		// startup before its HTTP API becomes ready. Never POST /newgame on top of
		// that already-observable result, otherwise one transaction can create two
		// directories. The directory set was snapshotted before startup, so this
		// check cannot confuse an old save with the requested result.
		newDirs, scanErr := tx.newSaveDirs()
		if scanErr != nil {
			return &NewGameTransactionError{Code: "new_game_save_scan_failed", Message: "扫描启动期间生成的存档目录失败", Cause: scanErr}
		}
		if len(newDirs) > 1 {
			err := fmt.Errorf("detected multiple new save directories before /newgame: %s", strings.Join(newDirs, ", "))
			tx.setFailure(newGameStateAmbiguous, "new_game_ambiguous", err)
			return &NewGameTransactionError{Code: "new_game_ambiguous", Message: "调用 /newgame 前已检测到多个新存档目录，结果不明确", Cause: err}
		}
		startupSaveDetected = len(newDirs) == 1
	}
	if startupSaveDetected {
		_, _ = jobCtx.Info(ctx, "Junimo 启动流程已生成新存档；跳过 /newgame POST，直接验证落盘结果。")
	} else if !tx.record.CommandCalled {
		// Persist the irreversible fact before POST. A process crash after this
		// point is ambiguous and must never cause an automatic second POST.
		if err := tx.markCommandCalled(); err != nil {
			return &NewGameTransactionError{Code: "new_game_state_write_failed", Message: "记录 /newgame 调用状态失败", Cause: err}
		}
		cmdCtx, cancel := context.WithTimeout(ctx, commandTimeout)
		result, err := r.lifecycle.ComposeExecPipe(cmdCtx, r.instance.DataDir, "server",
			"", "curl", "-sf", "-X", "POST", "-H", "Content-Type: application/json", "-d", "{}",
			"http://localhost:8080/newgame")
		commandTimedOut = errors.Is(cmdCtx.Err(), context.DeadlineExceeded)
		cancel()
		commandErr = err
		if err != nil {
			_, _ = jobCtx.Warn(ctx, fmt.Sprintf("创建请求未正常返回（%s）；不重试，继续按目录差异观察结果。", paneldocker.RedactString(err.Error())))
		} else if result.ExitCode != 0 {
			commandErr = fmt.Errorf("newgame request exited with code %d", result.ExitCode)
			_, _ = jobCtx.Warn(ctx, "创建请求返回失败；不重试，继续观察是否仍有存档落盘。")
		} else {
			_, _ = jobCtx.Info(ctx, "新存档创建请求已返回，正在验证落盘结果...")
		}
	} else {
		_, _ = jobCtx.Warn(ctx, "事务已记录 /newgame 调用；不会重复 POST，仅恢复结果观察。")
	}
	if err := tx.mark(newGameStateObserving); err != nil {
		return &NewGameTransactionError{Code: "new_game_state_write_failed", Message: "记录结果观察状态失败", Cause: err}
	}

	// Wait for the new save to appear in the Saves directory. Require the gameloader to
	// point at a save name different from the pre-existing one so a leftover old save is
	// never mistaken for the newly created one.
	observationTimeout := r.newGameObservationTimeout
	if observationTimeout <= 0 {
		observationTimeout = 5 * time.Minute
	}
	saveDeadline := time.Now().Add(observationTimeout)
	stability := map[string]*newGameFileStability{}
	for time.Now().Before(saveDeadline) {
		newDirs, err := tx.newSaveDirs()
		if err != nil {
			return &NewGameTransactionError{Code: "new_game_save_scan_failed", Message: "扫描新存档目录失败", Cause: err}
		}
		if len(newDirs) > 1 {
			err := fmt.Errorf("detected multiple new save directories: %s", strings.Join(newDirs, ", "))
			tx.setFailure(newGameStateAmbiguous, "new_game_ambiguous", err)
			return &NewGameTransactionError{Code: "new_game_ambiguous", Message: "检测到多个新存档目录，结果不明确", Cause: err}
		}
		if len(newDirs) == 1 {
			name := newDirs[0]
			state := stability[name]
			if state == nil {
				state = &newGameFileStability{}
				stability[name] = state
			}
			validationFarmType := tx.record.ResolvedFarmType
			if validationFarmType == "" {
				validationFarmType = tx.record.RequestedFarmType
			}
			stable, validateErr := validateStableNewGameSave(r.instance.DataDir, name, validationFarmType, state)
			if validateErr != nil {
				return &NewGameTransactionError{Code: classifyNewGameValidationError(validateErr, tx.record.RequestedFarmType), Message: "新存档验证失败", Cause: validateErr}
			}
			if stable {
				if data, readErr := os.ReadFile(gameloaderFile); readErr == nil {
					var gl struct {
						SaveNameToLoad string `json:"SaveNameToLoad"`
					}
					if json.Unmarshal(data, &gl) == nil && gl.SaveNameToLoad != "" && gl.SaveNameToLoad != name && uniqueNumericSuffixCandidate(gl.SaveNameToLoad, newDirs) == name {
						if writeErr := writeGameloaderPointer(r.instance.DataDir, name); writeErr != nil {
							return &NewGameTransactionError{Code: "new_game_gameloader_repair_failed", Message: "修复新存档指针失败", Cause: writeErr}
						}
						_, _ = jobCtx.Info(ctx, fmt.Sprintf("已将错误的 gameloader 前缀修正为：%s", name))
					}
				}
				commitProfile := r.commitNewGameModProfile
				if commitProfile == nil {
					commitProfile = EnsureNewSaveModProfile
				}
				profileKeys := []string{}
				if tx.record.ModSelection != nil {
					profileKeys = append(profileKeys, tx.record.EnabledModKeys...)
				}
				if err := commitProfile(r.instance.DataDir, name, profileKeys); err != nil {
					tx.record.CreatedSave = name
					tx.setFailure(newGameStateProfilePending, "mod_profile_commit_failed", err)
					return &NewGameTransactionError{Code: "mod_profile_commit_failed", Message: "存档已正确创建，但 Mod profile 提交失败；存档已保留且未激活，需要重试 profile commit", Cause: err}
				}
				if err := tx.complete(name); err != nil {
					return &NewGameTransactionError{Code: "new_game_state_write_failed", Message: "记录新存档成功状态失败", Cause: err}
				}
				_, _ = jobCtx.Info(ctx, fmt.Sprintf("新存档已验证创建：%s（%s）", name, tx.record.RequestedFarmType))
				return nil
			}
		}
		select {
		case <-ctx.Done():
			tx.setFailure(newGameStateUnknown, "new_game_outcome_unknown", ctx.Err())
			return &NewGameTransactionError{Code: "new_game_outcome_unknown", Message: "创建结果未知，禁止自动重试", Cause: ctx.Err()}
		case <-time.After(pollInterval):
		}
	}

	if commandTimedOut || commandErr != nil && errors.Is(commandErr, context.DeadlineExceeded) {
		err := fmt.Errorf("newgame request timed out and no new save became stable")
		tx.setFailure(newGameStateUnknown, "new_game_outcome_unknown", err)
		return &NewGameTransactionError{Code: "new_game_outcome_unknown", Message: "创建请求超时且未检测到可验证存档，结果未知，禁止自动重试", Cause: err}
	}
	if commandErr != nil {
		return &NewGameTransactionError{Code: "new_game_command_failed", Message: "/newgame 返回失败且未生成存档", Cause: commandErr}
	}
	if current := readGameloaderSaveName(gameloaderFile); current != "" && current != prevSave {
		return &NewGameTransactionError{Code: "new_game_pointer_without_save", Message: "gameloader 已变化但没有对应的新存档目录"}
	}
	return &NewGameTransactionError{Code: "new_game_save_not_found", Message: "/newgame 已调用但未检测到新存档目录"}
}

func readGameloaderSaveName(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var cfg struct {
		SaveNameToLoad string `json:"SaveNameToLoad"`
	}
	if json.Unmarshal(data, &cfg) != nil {
		return ""
	}
	return cfg.SaveNameToLoad
}

func classifyNewGameValidationError(err error, requestedFarmType string) string {
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "xml"):
		return "new_game_xml_invalid"
	case strings.Contains(message, "farm type mismatch"):
		if !isBuiltinFarmType(requestedFarmType) {
			return "farm_type_mismatch"
		}
		return "new_game_farm_type_mismatch"
	default:
		return "new_game_save_invalid"
	}
}

// mergeInviteCodeInPayload parses existing JSON payload and injects invite_code.
func mergeInviteCodeInPayload(existing, inviteCode string) string {
	payload := map[string]any{}
	if existing != "" {
		_ = jsonUnmarshal(existing, &payload)
	}
	payload["invite_code"] = inviteCode
	b, err := marshalJSON(payload)
	if err != nil {
		return existing
	}
	return strings.TrimSpace(string(b))
}

func inviteCodeFromPayload(existing string) string {
	if strings.TrimSpace(existing) == "" {
		return ""
	}
	payload := map[string]any{}
	if err := jsonUnmarshal(existing, &payload); err != nil {
		return ""
	}
	code, _ := payload["invite_code"].(string)
	return strings.TrimSpace(code)
}

func jsonUnmarshal(s string, v any) error {
	return json.Unmarshal([]byte(s), v)
}
