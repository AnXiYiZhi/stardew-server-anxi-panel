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
	newGameJobTimeout      = 90 * time.Minute
	controlCleanupTimeout  = 2 * time.Minute
	startServerWaitTimeout = 5 * time.Minute // Docker container reaches "running" within seconds; 5m is ample
	startCheckInterval     = 3 * time.Second
	startProgressInterval  = 30 * time.Second
	newGameReservationWait = 5 * time.Second
	newGameReservationPoll = 25 * time.Millisecond

	// readyStateTimeout covers the entire window from "container running" to "invite code obtained".
	// New-game world generation can take 15+ min, but the invite code may arrive earlier
	// (JunimoServer writes it as soon as the lobby is created, before save load completes).
	readyStateTimeout   = 20 * time.Minute
	readyInviteInterval = 15 * time.Second // how often to read the Junimo invite-code file
	readyLogInterval    = 60 * time.Second // how often to tail container logs
	readySMAPIInterval  = 5 * time.Second  // how often to poll status.json

	inviteCodeTimeout = 30 * time.Second

	backgroundInviteAttempts             = 20
	backgroundInviteInterval             = 15 * time.Second
	inviteCodeCacheTTL                   = 5 * time.Second
	inviteCodeReadScript                 = "if [ ! -e /tmp/invite-code.txt ]; then exit 0; fi\ncat /tmp/invite-code.txt"
	steamInviteWarmupStartedAtPayloadKey = "steam_invite_warmup_started_at"
)

const (
	startJobPayload          = `{"operation":"start"}`
	stopJobPayload           = `{"operation":"stop"}`
	restartJobPayload        = `{"operation":"restart"}`
	restoreRestartJobPayload = `{"operation":"restore_restart"}`
)

type lifecycleJobPayload struct {
	Operation string                  `json:"operation"`
	RequestID string                  `json:"requestId,omitempty"`
	Config    *registry.NewGameConfig `json:"config,omitempty"`
}

var (
	ErrLifecycleInProgress = errors.New("lifecycle operation already in progress")
	ErrRestartInProgress   = errors.New("restart already in progress")
	ErrSteamInviteDisabled = errors.New("Steam invite codes are not enabled")
)

type inviteCodeCacheEntry struct {
	code      string
	expiresAt time.Time
}

type inviteCodeFlight struct {
	done chan struct{}
	code string
	err  error
}

func runtimeServicesForSteamInvite(dataDir string) []string {
	return runtimeServicesForSteamInviteEnabled(sjconfig.SteamInviteEnabled(dataDir))
}

func runtimeServicesForSteamInviteEnabled(enabled bool) []string {
	if enabled {
		return []string{"steam-auth", "server"}
	}
	return []string{"server"}
}

func runtimeStopServicesForSteamInvite(dataDir string) []string {
	return runtimeStopServicesForSteamInviteEnabled(sjconfig.SteamInviteEnabled(dataDir))
}

func runtimeStopServicesForSteamInviteEnabled(enabled bool) []string {
	if enabled {
		return []string{"server", "steam-auth"}
	}
	return []string{"server"}
}

func stopRuntimeServices(ctx context.Context, lifecycle LifecycleDockerService, dataDir string) error {
	return stopRuntimeServicesSelected(ctx, lifecycle, dataDir, runtimeStopServicesForSteamInvite(dataDir))
}

func stopRuntimeServicesSelected(ctx context.Context, lifecycle LifecycleDockerService, dataDir string, services []string) error {
	project := strings.ToLower(filepath.Base(filepath.Clean(dataDir)))
	if !filepath.IsAbs(dataDir) || !runtimeComposeProjectPattern.MatchString(project) {
		return errors.New("runtime Compose project cannot be derived safely")
	}
	return lifecycle.RuntimeComposeStopServices(ctx, dataDir, project, services...)
}

// LifecycleDockerService extends DockerService with lifecycle operations.
type LifecycleDockerService interface {
	DockerService
	ComposeUp(ctx context.Context, dir string) (paneldocker.CommandResult, error)
	ComposeDown(ctx context.Context, dir string) (paneldocker.CommandResult, error)
	ComposeRestart(ctx context.Context, dir string) (paneldocker.CommandResult, error)
	ComposeRestartServices(ctx context.Context, dir string, services ...string) (paneldocker.CommandResult, error)
	ComposeRecreateServices(ctx context.Context, dir string, services ...string) (paneldocker.CommandResult, error)
	RuntimeComposeStopServices(ctx context.Context, dir, project string, services ...string) error
	ComposeExecPipe(ctx context.Context, dir, service, stdinData string, args ...string) (paneldocker.CommandResult, error)
	ComposeExecTTY(ctx context.Context, dir, service, stdinData string, args ...string) (paneldocker.ComposeExecTTYResult, error)
	ComposeLogs(ctx context.Context, dir string, opts paneldocker.LogsOptions) (paneldocker.CommandResult, error)
}

// lifecycleRunner handles start/stop/restart job execution.
type lifecycleRunner struct {
	driver                    *Driver
	lifecycle                 LifecycleDockerService
	instance                  storage.Instance
	operation                 string // "start", "stop", "restart", "restore_restart", "new_game_rollback"
	actorID                   int64
	newGame                   bool // When true, send "settings newgame --confirm" after server starts.
	newGameConfig             *registry.NewGameConfig
	newGameRequestID          string
	newGameCommandTimeout     time.Duration
	newGameObservationTimeout time.Duration
	newGamePollInterval       time.Duration
	newGameAPIReadyTimeout    time.Duration
	newGameCatalogTimeout     time.Duration
	newGameControlGateTimeout time.Duration
	newGameSaveGateTimeout    time.Duration
	newGameDiskGateTimeout    time.Duration
	commitNewGameModProfile   func(string, string, []string) error
	// A production new-game Start creates the exclusive job first to obtain its
	// durable jobId, then synchronously reserves the persistent owner/transaction
	// before returning to the HTTP caller. The runner waits on this handoff so it
	// can never touch runtime files before the reservation is fully durable.
	newGameReservationReady   <-chan struct{}
	newGameReservationTx      *newGameTransaction
	newGameReservationResumed bool
	newGameReservationErr     error
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
	ld, ok := d.docker.(LifecycleDockerService)
	if !ok {
		return nil, fmt.Errorf("docker 服务不支持生命周期操作")
	}
	instance, err := d.store.GetInstance(ctx, req.Instance.ID)
	if err != nil {
		return nil, fmt.Errorf("load instance: %w", err)
	}
	if err := d.convergeSteamInviteCleanupPending(ctx, registry.Instance{ID: instance.ID, DriverID: instance.DriverID, DataDir: instance.DataDir}); err != nil {
		return nil, err
	}
	if err := d.rejectActiveSteamInviteAuthorization(ctx, req.Instance.ID); err != nil {
		return nil, err
	}
	resumeFromManualStart := false
	rollbackRecovery := false
	if !req.NewGame {
		resume, resumeErr := pendingNewGameStartRequest(instance.DataDir)
		if resumeErr != nil {
			return nil, resumeErr
		}
		if resume != nil {
			resumeFromManualStart = true
			rollbackRecovery = isNewGameRollbackRecoveryStage(resume.Stage)
			req.NewGame = true
			req.NewGameConfig = &resume.Config
			req.RequestID = resume.RequestID
		}
	}
	jobPayload := ""
	if req.NewGame {
		if req.NewGameConfig == nil {
			return nil, &NewGameTransactionError{Code: "new_game_payload_missing", Message: "新建存档任务缺少规范化配置"}
		}
		if strings.TrimSpace(req.RequestID) == "" {
			compatID, randomErr := newGameRandomHex(16)
			if randomErr != nil {
				return nil, &NewGameOwnerError{Code: "new_game_request_invalid", Message: "无法生成兼容的新建存档请求 ID", Cause: randomErr}
			}
			req.RequestID = "compat-" + compatID
		}
		if err := validateNewGameRequestID(req.RequestID); err != nil {
			return nil, &NewGameOwnerError{Code: "new_game_request_invalid", Message: "新建存档请求 ID 无效", Cause: err}
		}
		normalized, normalizeErr := NormalizeNewGameConfigWithModded(*req.NewGameConfig, true)
		if normalizeErr != nil {
			return nil, &NewGameOwnerError{Code: "new_game_payload_invalid", Message: "新建存档配置无效", Cause: normalizeErr}
		}
		req.NewGameConfig = &normalized
		if !resumeFromManualStart {
			if persisted, found, err := d.findPersistedNewGameJob(ctx, instance.DataDir, req.RequestID, *req.NewGameConfig); err != nil {
				return nil, err
			} else if found {
				return &registry.Job{ID: persisted.ID}, nil
			}
		}
		// A new request (unlike an exact idempotent replay above) must prove the
		// instance is stopped before it reserves files that are also mutation
		// targets. This is inside runtimeUpdateMu, the same linearization mutex used
		// by lifecycle/runtime/import mutations.
		if !resumeFromManualStart {
			ps, psErr := d.docker.ComposePs(ctx, instance.DataDir)
			if psErr != nil {
				return nil, &NewGameOwnerError{Code: "server_state_unknown", Message: "无法确认服务器已停止，请检查 Docker 状态后重试", Cause: psErr}
			}
			if serverServiceUp(ps.Services) {
				return nil, &NewGameOwnerError{Code: "server_running", Message: "服务器容器仍在运行，请先停止服务器再创建存档"}
			}
		}
		if existing, found, err := d.findActiveNewGameJob(ctx, req.Instance.ID, req.RequestID, *req.NewGameConfig); err != nil {
			return nil, err
		} else if found {
			persisted, waitErr := d.waitForPersistedNewGameReservation(ctx, instance.DataDir, req.RequestID, *req.NewGameConfig, existing.ID)
			if waitErr != nil {
				return nil, waitErr
			}
			return &registry.Job{ID: persisted.ID}, nil
		}
		payloadOperation := "new_game"
		if rollbackRecovery {
			payloadOperation = "new_game_rollback"
		}
		payloadData, err := json.Marshal(lifecycleJobPayload{Operation: payloadOperation, RequestID: req.RequestID, Config: req.NewGameConfig})
		if err != nil {
			return nil, &NewGameTransactionError{Code: "new_game_payload_invalid", Message: "序列化新建存档任务配置失败", Cause: err}
		}
		jobPayload = string(payloadData)
	} else {
		if err := rejectUnfinishedNewGameOwner(instance.DataDir); err != nil {
			return nil, err
		}
		active, activeOperation, found, activeErr := d.findActiveLifecycleJob(ctx, req.Instance.ID)
		if activeErr != nil {
			return nil, activeErr
		}
		if found {
			if activeOperation == "start" {
				return &registry.Job{ID: active.ID}, nil
			}
			return nil, lifecycleConflict(activeOperation, active.ID)
		}
		jobPayload = startJobPayload
	}
	if err := d.rejectActiveSaveImport(ctx, req.Instance.ID, ""); err != nil {
		return nil, err
	}
	if err := d.rejectActiveFarmhandDelete(ctx, req.Instance.ID); err != nil {
		return nil, err
	}
	if err := d.rejectActiveRuntimeUpdate(ctx, req.Instance.ID); err != nil {
		return nil, err
	}
	// A rollback-only recovery must remain available even when the forward
	// runtime stack is outdated. It never starts game containers or consumes the
	// candidate runtime; it only confirms Compose is stopped and restores the
	// transaction snapshots.
	if !rollbackRecovery {
		if err := d.requireCurrentRuntimeStack(req.Instance); err != nil {
			return nil, err
		}
	}
	operation := "start"
	if rollbackRecovery {
		operation = "new_game_rollback"
	}
	runner := &lifecycleRunner{
		driver:           d,
		lifecycle:        ld,
		instance:         instance,
		operation:        operation,
		actorID:          req.ActorID,
		newGame:          req.NewGame,
		newGameConfig:    req.NewGameConfig,
		newGameRequestID: req.RequestID,
	}
	var reservationReady chan struct{}
	if req.NewGame {
		reservationReady = make(chan struct{})
		runner.newGameReservationReady = reservationReady
	}
	jobTimeout := lifecycleJobTimeout
	if req.NewGame {
		jobTimeout = newGameJobTimeout
	}
	job, err := d.jobs.Start(ctx, jobs.Spec{
		Type:       lifecycleJobType,
		TargetType: "instance",
		TargetID:   req.Instance.ID,
		Exclusive:  true,
		CreatedBy:  req.ActorID,
		Payload:    jobPayload,
		Timeout:    jobTimeout,
		Run:        runner.run,
	})
	if err != nil {
		if req.NewGame && !rollbackRecovery {
			if existing, found, lookupErr := d.findActiveNewGameJob(ctx, req.Instance.ID, req.RequestID, *req.NewGameConfig); lookupErr != nil {
				return nil, lookupErr
			} else if found {
				persisted, waitErr := d.waitForPersistedNewGameReservation(ctx, instance.DataDir, req.RequestID, *req.NewGameConfig, existing.ID)
				if waitErr != nil {
					return nil, waitErr
				}
				return &registry.Job{ID: persisted.ID}, nil
			}
		}
		if !req.NewGame {
			if existing, activeOperation, found, lookupErr := d.findActiveLifecycleJob(ctx, req.Instance.ID); lookupErr != nil {
				return nil, lookupErr
			} else if found {
				if activeOperation == "start" {
					return &registry.Job{ID: existing.ID}, nil
				}
				return nil, lifecycleConflict(activeOperation, existing.ID)
			}
		}
		return nil, fmt.Errorf("start lifecycle job: %w", err)
	}
	if req.NewGame {
		isJobActive := func(jobID string) (bool, error) {
			active, activeErr := d.jobs.Active(context.Background(), storage.ListActiveJobsFilter{
				TargetType: "instance",
				TargetID:   req.Instance.ID,
				Types:      []string{lifecycleJobType},
			})
			if activeErr != nil {
				return false, activeErr
			}
			for _, activeJob := range active {
				if activeJob.ID == jobID {
					return true, nil
				}
			}
			return false, nil
		}
		tx, resumed, reserveErr := beginOrResumeNewGameTransactionWithJobStatus(
			instance.DataDir, *req.NewGameConfig, req.RequestID, job.ID, isJobActive,
		)
		runner.newGameReservationTx = tx
		runner.newGameReservationResumed = resumed
		runner.newGameReservationErr = reserveErr
		close(reservationReady)
		if reserveErr != nil {
			return nil, reserveErr
		}
	}
	return &registry.Job{ID: job.ID}, nil
}

func (d *Driver) findActiveLifecycleJob(ctx context.Context, instanceID string) (storage.Job, string, bool, error) {
	active, err := d.jobs.Active(ctx, storage.ListActiveJobsFilter{
		TargetType: "instance",
		TargetID:   instanceID,
		Types:      []string{lifecycleJobType},
	})
	if err != nil {
		return storage.Job{}, "", false, fmt.Errorf("list active lifecycle jobs: %w", err)
	}
	if len(active) == 0 {
		return storage.Job{}, "", false, nil
	}
	job := active[0]
	if len(active) > 1 {
		return job, "multiple", true, nil
	}
	return job, lifecycleOperation(job), true, nil
}

func (d *Driver) rejectActiveSteamInviteAuthorization(ctx context.Context, instanceID string) error {
	active, err := d.jobs.Active(ctx, storage.ListActiveJobsFilter{
		TargetType: "instance",
		TargetID:   instanceID,
		Types:      []string{"stardew_steam_auth"},
	})
	if err != nil {
		return fmt.Errorf("list active Steam invite authorization jobs: %w", err)
	}
	if len(active) > 0 {
		return lifecycleConflict("steam_invite_authorization", active[0].ID)
	}
	return nil
}

func lifecycleOperation(job storage.Job) string {
	if !job.Payload.Valid || strings.TrimSpace(job.Payload.String) == "" {
		return "unknown"
	}
	var payload lifecycleJobPayload
	if err := json.Unmarshal([]byte(job.Payload.String), &payload); err != nil || strings.TrimSpace(payload.Operation) == "" {
		return "unknown"
	}
	return strings.TrimSpace(payload.Operation)
}

func lifecycleConflict(operation, jobID string) error {
	return fmt.Errorf("%w: active operation=%s job=%s", ErrLifecycleInProgress, operation, jobID)
}

// waitForPersistedNewGameReservation closes the cross-process window between
// SQLite's exclusive job commit and atomic owner publication. A second Panel
// may observe the job during that interval, but it must not acknowledge 202
// until the same jobId is durably bound by both transaction and owner.
func (d *Driver) waitForPersistedNewGameReservation(ctx context.Context, dataDir, requestID string, cfg registry.NewGameConfig, jobID string) (storage.Job, error) {
	deadline := time.NewTimer(newGameReservationWait)
	defer deadline.Stop()
	ticker := time.NewTicker(newGameReservationPoll)
	defer ticker.Stop()
	for {
		job, found, err := d.findPersistedNewGameJob(ctx, dataDir, requestID, cfg)
		if err != nil {
			return storage.Job{}, err
		}
		if found {
			if job.ID != jobID {
				return storage.Job{}, &NewGameOwnerError{Code: "new_game_recovery_required", Message: "活动任务与持久新建存档事务的 jobId 不一致"}
			}
			owner, ownerErr := LoadNewGameOwner(dataDir)
			if ownerErr != nil {
				return storage.Job{}, &NewGameOwnerError{Code: "new_game_recovery_required", Message: "持久新建存档事务已出现但 owner 无法验证", Cause: ownerErr}
			}
			record, recordErr := LoadNewGameTransaction(dataDir, owner.TransactionID)
			if recordErr != nil || owner.JobID != jobID || record.JobID != jobID || owner.RequestID != requestID || record.RequestID != requestID || owner.TransactionID != record.TransactionID || owner.ConfigSHA256 != record.ConfigSHA256 {
				return storage.Job{}, &NewGameOwnerError{Code: "new_game_recovery_required", Message: "活动任务尚未形成一致的持久 owner/transaction 绑定", Cause: recordErr}
			}
			return job, nil
		}
		select {
		case <-ctx.Done():
			return storage.Job{}, ctx.Err()
		case <-deadline.C:
			return storage.Job{}, &NewGameOwnerError{Code: "new_game_in_progress", Message: "新建存档任务正在持久化 owner；尚未安全接受本次重试"}
		case <-ticker.C:
		}
	}
}

func (d *Driver) findPersistedNewGameJob(ctx context.Context, dataDir, requestID string, cfg registry.NewGameConfig) (storage.Job, bool, error) {
	record, err := findNewGameTransactionByRequest(dataDir, requestID)
	if err != nil {
		return storage.Job{}, false, &NewGameOwnerError{Code: "new_game_recovery_required", Message: "读取新建存档幂等事务失败", Cause: err}
	}
	if record == nil {
		return storage.Job{}, false, nil
	}
	hash, err := newGameConfigSHA256(cfg)
	if err != nil {
		return storage.Job{}, false, err
	}
	if record.ConfigSHA256 != hash {
		return storage.Job{}, false, &NewGameOwnerError{Code: "new_game_request_conflict", Message: "同一请求 ID 已绑定到不同的新建存档配置"}
	}
	if strings.TrimSpace(record.JobID) == "" {
		return storage.Job{}, false, &NewGameOwnerError{Code: "new_game_recovery_required", Message: "新建存档事务缺少原始 jobId"}
	}
	job, err := d.jobs.Get(ctx, record.JobID)
	if err != nil {
		return storage.Job{}, false, &NewGameOwnerError{Code: "new_game_recovery_required", Message: "新建存档事务引用的原始任务不存在", Cause: err}
	}
	return job, true, nil
}

func (d *Driver) findActiveNewGameJob(ctx context.Context, instanceID, requestID string, cfg registry.NewGameConfig) (storage.Job, bool, error) {
	active, err := d.jobs.Active(ctx, storage.ListActiveJobsFilter{
		TargetType: "instance",
		TargetID:   instanceID,
		Types:      []string{lifecycleJobType},
	})
	if err != nil {
		return storage.Job{}, false, fmt.Errorf("list active lifecycle jobs: %w", err)
	}
	wantedHash, err := newGameConfigSHA256(cfg)
	if err != nil {
		return storage.Job{}, false, err
	}
	for _, job := range active {
		var payload lifecycleJobPayload
		if !job.Payload.Valid || json.Unmarshal([]byte(job.Payload.String), &payload) != nil || payload.Operation != "new_game" || payload.Config == nil {
			return storage.Job{}, false, &NewGameOwnerError{Code: "new_game_in_progress", Message: "实例存在尚未结束的生命周期任务，当前不会取消它以启动新建存档"}
		}
		if payload.RequestID != requestID {
			return storage.Job{}, false, &NewGameOwnerError{Code: "new_game_in_progress", Message: "另一个新建存档任务仍在执行"}
		}
		activeHash, hashErr := newGameConfigSHA256(*payload.Config)
		if hashErr != nil || activeHash != wantedHash {
			return storage.Job{}, false, &NewGameOwnerError{Code: "new_game_request_conflict", Message: "同一请求 ID 已绑定到不同的新建存档配置", Cause: hashErr}
		}
		return job, true, nil
	}
	return storage.Job{}, false, nil
}

func pendingNewGameStartRequest(dataDir string) (*NewGameTransactionRecord, error) {
	owner, err := LoadNewGameOwner(dataDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, &NewGameOwnerError{Code: "new_game_recovery_required", Message: "新建存档 owner 无法读取，需要先修复事务现场", Cause: err}
	}
	record, err := LoadNewGameTransaction(dataDir, owner.TransactionID)
	if err != nil {
		return nil, &NewGameOwnerError{Code: "new_game_recovery_required", Message: "新建存档事务记录缺失或损坏", Owner: &owner, Cause: err}
	}
	if record.TransactionID != owner.TransactionID || record.RequestID != owner.RequestID || record.ConfigSHA256 != owner.ConfigSHA256 {
		return nil, &NewGameOwnerError{Code: "new_game_recovery_required", Message: "新建存档 owner 与事务身份不一致", Owner: &owner}
	}
	if isTerminalNewGameOwnerStage(record.Stage) {
		if err := releaseTerminalNewGameOwner(dataDir, owner); err != nil {
			return nil, &NewGameOwnerError{Code: "new_game_recovery_required", Message: "清理已结束的新建存档 owner 失败", Owner: &owner, Cause: err}
		}
		return nil, nil
	}
	return &record, nil
}

func isNewGameRollbackRecoveryStage(stage NewGameTransactionState) bool {
	return stage == newGameStateRollingBack || stage == newGameStateRollbackFail
}

func rejectUnfinishedNewGameOwner(dataDir string) error {
	record, err := pendingNewGameStartRequest(dataDir)
	if err != nil {
		return err
	}
	if record != nil {
		return &NewGameOwnerError{Code: "new_game_in_progress", Message: "新建存档事务尚未结束，当前操作不会取消或覆盖它"}
	}
	return nil
}

// Stop implements registry.GameDriver.Stop.
func (d *Driver) Stop(ctx context.Context, instance registry.Instance) error {
	if d.jobs == nil {
		return fmt.Errorf("driver: job manager not configured")
	}
	d.runtimeUpdateMu.Lock()
	defer d.runtimeUpdateMu.Unlock()
	if err := d.rejectActiveSaveImport(ctx, instance.ID, ""); err != nil {
		return err
	}
	if err := d.rejectActiveFarmhandDelete(ctx, instance.ID); err != nil {
		return err
	}
	if err := d.rejectActiveSteamInviteAuthorization(ctx, instance.ID); err != nil {
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
	if err := rejectUnfinishedNewGameOwner(stored.DataDir); err != nil {
		return err
	}
	active, activeOperation, found, activeErr := d.findActiveLifecycleJob(ctx, instance.ID)
	if activeErr != nil {
		return activeErr
	}
	if found {
		if activeOperation == "stop" {
			return nil
		}
		return lifecycleConflict(activeOperation, active.ID)
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
		Exclusive:  true,
		CreatedBy:  0,
		Payload:    stopJobPayload,
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
	if err := d.rejectActiveSaveImport(ctx, instance.ID, ""); err != nil {
		return err
	}
	if err := d.rejectActiveFarmhandDelete(ctx, instance.ID); err != nil {
		return err
	}
	if err := d.rejectActiveSteamInviteAuthorization(ctx, instance.ID); err != nil {
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
	if err := rejectUnfinishedNewGameOwner(stored.DataDir); err != nil {
		return err
	}
	active, activeOperation, found, activeErr := d.findActiveLifecycleJob(ctx, instance.ID)
	if activeErr != nil {
		return activeErr
	}
	if found {
		if activeOperation == "restart" {
			return ErrRestartInProgress
		}
		return lifecycleConflict(activeOperation, active.ID)
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
		Exclusive:  true,
		CreatedBy:  0,
		Payload:    restartJobPayload,
		Timeout:    lifecycleJobTimeout,
		Run:        runner.run,
	}); err != nil {
		return fmt.Errorf("start restart job: %w", err)
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
	if err := d.rejectActiveSaveImport(ctx, instance.ID, ""); err != nil {
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
	if err := rejectUnfinishedNewGameOwner(stored.DataDir); err != nil {
		return nil, err
	}
	active, activeOperation, found, activeErr := d.findActiveLifecycleJob(ctx, instance.ID)
	if activeErr != nil {
		return nil, activeErr
	}
	if found {
		return nil, lifecycleConflict(activeOperation, active.ID)
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
		Exclusive:  true,
		CreatedBy:  actorID,
		Payload:    restoreRestartJobPayload,
		Timeout:    lifecycleJobTimeout,
		Run:        runner.run,
	})
	if err != nil {
		return nil, fmt.Errorf("start restore-restart job: %w", err)
	}
	return &registry.Job{ID: job.ID}, nil
}

// run is the job.Runner function for lifecycle operations.
func (r *lifecycleRunner) run(ctx context.Context, jobCtx *jobs.Context) error {
	if r.newGameReservationReady != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-r.newGameReservationReady:
		}
		if r.newGameReservationErr != nil {
			return r.newGameReservationErr
		}
		if r.newGameReservationTx == nil {
			return &NewGameOwnerError{Code: "new_game_recovery_required", Message: "新建存档任务缺少已持久化的 owner 预留"}
		}
	}
	switch r.operation {
	case "start":
		return r.doStart(ctx, jobCtx)
	case "stop":
		return r.doStop(ctx, jobCtx)
	case "restart":
		return r.doRestart(ctx, jobCtx)
	case "restore_restart":
		return r.doRestoreAndRestart(ctx, jobCtx)
	case "new_game_rollback":
		return r.doResumeNewGameRollback(ctx, jobCtx)
	default:
		return fmt.Errorf("未知的生命周期操作: %s", r.operation)
	}
}

// doResumeNewGameRollback is deliberately separate from doStart. A manual
// Start while a rollback journal is unfinished is a recovery trigger, not
// permission to resume the forward new-game path: it may only stop/confirm the
// Compose project, replay idempotent rollback steps, and leave the game off.
func (r *lifecycleRunner) doResumeNewGameRollback(ctx context.Context, jobCtx *jobs.Context) error {
	if r.newGameConfig == nil || strings.TrimSpace(r.newGameRequestID) == "" {
		return &NewGameOwnerError{Code: "new_game_recovery_required", Message: "回滚恢复缺少原事务身份"}
	}
	isJobActive := func(jobID string) (bool, error) {
		active, err := r.driver.jobs.Active(context.Background(), storage.ListActiveJobsFilter{
			TargetType: "instance",
			TargetID:   r.instance.ID,
			Types:      []string{lifecycleJobType},
		})
		if err != nil {
			return false, err
		}
		for _, activeJob := range active {
			if activeJob.ID == jobID {
				return true, nil
			}
		}
		return false, nil
	}
	tx := r.newGameReservationTx
	if tx == nil {
		var err error
		tx, _, err = beginOrResumeNewGameTransactionWithJobStatus(
			r.instance.DataDir, *r.newGameConfig, r.newGameRequestID, jobCtx.ID, isJobActive,
		)
		if err != nil {
			return &NewGameOwnerError{Code: "new_game_recovery_required", Message: "接管新建存档回滚事务失败", Cause: err}
		}
	}
	if !isNewGameRollbackRecoveryStage(tx.record.Stage) {
		return &NewGameOwnerError{Code: "new_game_recovery_required", Message: "事务不处于可恢复回滚阶段"}
	}
	originalCause := errors.New(tx.record.ErrorMessage)
	if tx.record.ErrorMessage == "" {
		originalCause = errors.New("new-game transaction failed before rollback")
	}
	if err := tx.beginRollback(originalCause, tx.record.ErrorCode, tx.record.RollbackOriginalStage); err != nil {
		r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateError,
			"无法持久化新建存档回滚恢复点，未修改事务文件。", "new_game_rollback_failed", jobCtx.ID)
		return &NewGameTransactionError{Code: "new_game_rollback_failed", Message: "持久化回滚恢复点失败", Cause: err}
	}
	_, _ = jobCtx.Info(ctx, fmt.Sprintf("正在恢复新建存档回滚事务：%s", tx.record.TransactionID))

	stopServices := runtimeStopServicesForSteamInvite(r.instance.DataDir)
	compose, inspectErr := r.lifecycle.ComposePs(ctx, r.instance.DataDir)
	if inspectErr != nil {
		rollbackErr := tx.failRollback(fmt.Errorf("inspect Compose before rollback: %w", inspectErr))
		r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateError,
			"无法确认游戏容器已停止；回滚现场已保留。", "new_game_rollback_failed", jobCtx.ID)
		return &NewGameTransactionError{Code: "new_game_rollback_failed", Message: "无法确认游戏容器已停止", Cause: inspectErr, RollbackError: rollbackErr}
	}
	if !runtimeServicesConfirmedStopped(compose, stopServices) {
		_, _ = jobCtx.Info(ctx, "检测到 Compose 服务仍在运行；仅执行停服后继续原回滚，不会启动游戏。")
		downErr := stopRuntimeServices(ctx, r.lifecycle, r.instance.DataDir)
		if downErr != nil {
			rollbackErr := tx.failRollback(fmt.Errorf("stop Compose before rollback: %w", downErr))
			r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateError,
				"游戏容器停止失败；未继续文件回滚，现场已保留。", "new_game_rollback_failed", jobCtx.ID)
			return &NewGameTransactionError{Code: "new_game_rollback_failed", Message: "游戏容器停止失败", Cause: downErr, RollbackError: rollbackErr}
		}
		compose, inspectErr = r.lifecycle.ComposePs(ctx, r.instance.DataDir)
		if inspectErr != nil || !runtimeServicesConfirmedStopped(compose, stopServices) {
			if inspectErr == nil {
				inspectErr = errors.New("selected runtime services are still active after stop")
			}
			rollbackErr := tx.failRollback(fmt.Errorf("confirm Compose stopped after rollback stop: %w", inspectErr))
			r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateError,
				"停服后仍无法确认容器终态；未继续文件回滚。", "new_game_rollback_failed", jobCtx.ID)
			return &NewGameTransactionError{Code: "new_game_rollback_failed", Message: "停服后无法确认容器终态", Cause: inspectErr, RollbackError: rollbackErr}
		}
	}

	if err := tx.continueRollback(); err != nil {
		r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateError,
			"新建存档回滚尚未完整完成；owner 和 journal 已保留。", "new_game_rollback_failed", jobCtx.ID)
		return &NewGameTransactionError{Code: "new_game_rollback_failed", Message: "继续新建存档回滚失败", Cause: err}
	}
	if tx.record.Stage != newGameStateRolledBack || !tx.record.RollbackCompleted {
		return &NewGameOwnerError{Code: "new_game_recovery_required", Message: "回滚未达到完整终态，owner 不会释放"}
	}
	if err := tx.releaseOwner(); err != nil {
		return &NewGameOwnerError{Code: "new_game_recovery_required", Message: "回滚已完成但 owner 清理失败", Cause: err}
	}
	r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateStopped,
		"新建存档事务已完整回滚；游戏服务器保持关闭，请手动再次启动。", "new_game_rolled_back", jobCtx.ID)
	_, _ = jobCtx.Info(ctx, "原新建存档事务已完整回滚，游戏服务器保持关闭。")
	return nil
}

func newGameComposeConfirmedStopped(result paneldocker.ComposePsResult) bool {
	for _, service := range result.Services {
		state := strings.ToLower(strings.TrimSpace(service.State))
		status := strings.ToLower(strings.TrimSpace(service.Status))
		if state == "exited" || state == "dead" || state == "stopped" || strings.HasPrefix(status, "exited") {
			continue
		}
		return false
	}
	return true
}

func runtimeServicesConfirmedStopped(result paneldocker.ComposePsResult, services []string) bool {
	selected := make(map[string]struct{}, len(services))
	for _, service := range services {
		selected[service] = struct{}{}
	}
	for _, service := range result.Services {
		if _, ok := selected[service.Service]; !ok {
			continue
		}
		state := strings.ToLower(strings.TrimSpace(service.State))
		status := strings.ToLower(strings.TrimSpace(service.Status))
		if state == "exited" || state == "dead" || state == "stopped" || strings.HasPrefix(status, "exited") {
			continue
		}
		return false
	}
	return true
}

func (r *lifecycleRunner) doStart(ctx context.Context, jobCtx *jobs.Context) (retErr error) {
	_, _ = jobCtx.Info(ctx, "正在启动 Stardew 服务器...")
	if r.driver != nil {
		r.driver.clearDriverPayloadInviteCode(context.Background(), r.instance.ID)
	}
	// The production Start path has already made this reservation durable before
	// returning the job to its caller. Arm a narrow early-failure settlement for
	// verification/config errors that occur before the main transaction defer is
	// installed below. No runtime mutation has happened at that point, so a clean
	// transaction can be rolled back and released; uncertain progress is retained.
	preStartReservationPending := r.newGame && r.newGameReservationTx != nil
	defer func() {
		if !preStartReservationPending || retErr == nil {
			return
		}
		tx := r.newGameReservationTx
		evidence, progressErr := tx.observeNewGameProgress()
		if progressErr != nil || evidence.Observed || evidence.Ambiguous {
			tx.record.Stage = newGameStateUnknown
			tx.record.Result = "unconfirmed"
			tx.record.ErrorCode = "new_game_recovery_required"
			tx.record.ErrorMessage = paneldocker.RedactString(retErr.Error())
			_ = tx.persist()
			return
		}
		if rollbackErr := tx.rollback(retErr, "new_game_start_preflight_failed", newGameStateFailed); rollbackErr != nil {
			retErr = &NewGameTransactionError{Code: "new_game_rollback_failed", Message: "新建存档启动前校验失败且事务回滚未完成", Cause: retErr, RollbackError: rollbackErr}
			return
		}
		if releaseErr := tx.releaseOwner(); releaseErr != nil {
			retErr = &NewGameTransactionError{Code: "new_game_owner_release_failed", Message: "新建存档启动前校验失败，事务已回滚但 owner 清理失败", Cause: retErr, RollbackError: releaseErr}
		}
	}()
	if changed, warning, err := ensureServerPlayerAuthEnvironmentForLifecycle(r.instance.DataDir); err != nil {
		r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateStopped,
			"迁移玩家加入保护运行环境失败，已阻止启动", "player_auth_compose_migration_failed", jobCtx.ID)
		return fmt.Errorf("migrate player authentication Compose environment before start: %w", err)
	} else if changed {
		_, _ = jobCtx.Info(ctx, "已为旧实例补齐玩家加入保护运行环境。")
	} else if warning != "" {
		_, _ = jobCtx.Info(ctx, "warning: "+warning)
	}
	imageRef := gameInstallImage(r.instance.DataDir)
	ok, err := r.driver.verifyGameDataVolume(ctx, r.instance.DataDir, imageRef, func(line string) {
		_, _ = jobCtx.Info(ctx, "[verify] "+paneldocker.RedactString(line))
	})
	if err != nil || !ok {
		if err == nil {
			r.driver.rememberInstallationEvidence(r.instance.ID, "missing")
		}
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
	r.driver.rememberInstallationEvidence(r.instance.ID, "ok")

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
		isJobActive := func(jobID string) (bool, error) {
			active, activeErr := r.driver.jobs.Active(context.Background(), storage.ListActiveJobsFilter{
				TargetType: "instance",
				TargetID:   r.instance.ID,
				Types:      []string{lifecycleJobType},
			})
			if activeErr != nil {
				return false, activeErr
			}
			for _, activeJob := range active {
				if activeJob.ID == jobID {
					return true, nil
				}
			}
			return false, nil
		}
		var resumed bool
		if strings.TrimSpace(r.newGameRequestID) == "" {
			compatID, randomErr := newGameRandomHex(16)
			if randomErr != nil {
				return &NewGameTransactionError{Code: "new_game_request_invalid", Message: "无法生成兼容的新建存档请求 ID", Cause: randomErr}
			}
			r.newGameRequestID = "compat-" + compatID
		}
		if r.newGameReservationTx != nil {
			newGameTx = r.newGameReservationTx
			resumed = r.newGameReservationResumed
			cfg = newGameTx.record.Config
		} else {
			newGameTx, resumed, retErr = beginOrResumeNewGameTransactionWithJobStatus(
				r.instance.DataDir, cfg, r.newGameRequestID, jobCtx.ID, isJobActive,
			)
			if retErr != nil {
				return &NewGameTransactionError{Code: "new_game_snapshot_failed", Message: "创建新存档事务快照失败", Cause: retErr}
			}
		}
		preStartReservationPending = false
		defer func() {
			if newGameCompleted || retErr == nil {
				return
			}
			code := "new_game_failed"
			var txErr *NewGameTransactionError
			if errors.As(retErr, &txErr) && txErr.Code != "" {
				code = txErr.Code
			}
			preserveForRecovery := newGameTx.record.CommandCalled || newGameTx.record.ProgressObserved ||
				newGameTx.record.Stage == newGameStateUnknown || newGameTx.record.Stage == newGameStateAmbiguous
			if !preserveForRecovery {
				// Progress can appear after the last normal observation (for
				// example while waiting for Control). Recheck before stopping or
				// rolling back so a late loader/directory advance is never erased.
				evidence, progressErr := newGameTx.observeNewGameProgress()
				if progressErr != nil {
					preserveForRecovery = true
					code = "new_game_progress_state_unknown"
				} else if evidence.Observed || evidence.Ambiguous {
					preserveForRecovery = true
				}
			}
			if preserveForRecovery {
				if newGameTx.record.Stage != newGameStateAmbiguous &&
					newGameTx.record.Stage != newGameStateProfilePending &&
					newGameTx.record.Stage != newGameStateFinalizing {
					newGameTx.record.Stage = newGameStateUnknown
				}
				newGameTx.record.Result = "unconfirmed"
				newGameTx.record.ErrorCode = code
				newGameTx.record.ErrorMessage = paneldocker.RedactString(retErr.Error())
				_ = newGameTx.persist()
				r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateError,
					"新建存档已有不可逆进展，现场已保留；请手动再次启动以恢复同一事务。", "new_game_recovery_required", jobCtx.ID)
				return
			}
			stage := newGameStateFailed
			if newGameTx.record.Stage == newGameStateUnknown || newGameTx.record.Stage == newGameStateAmbiguous {
				stage = newGameTx.record.Stage
			}
			if beginErr := newGameTx.beginRollback(retErr, code, stage); beginErr != nil {
				retErr = &NewGameTransactionError{Code: "new_game_rollback_failed", Message: "无法持久化回滚恢复点，未执行停服或文件恢复", Cause: retErr, RollbackError: beginErr}
				r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateError,
					"新建存档回滚 journal 写入失败；未继续破坏性操作，owner 已保留。", "new_game_rollback_failed", jobCtx.ID)
				return
			}
			if composeStarted {
				stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				composeStopErr := stopRuntimeServices(stopCtx, r.lifecycle, r.instance.DataDir)
				cancel()
				if composeStopErr != nil {
					rollbackErr := newGameTx.failRollback(fmt.Errorf("stop server before rollback: %w", composeStopErr))
					retErr = &NewGameTransactionError{Code: "new_game_rollback_failed", Message: "新建存档失败且无法确认服务器已停止；已保留 owner 和现场", Cause: retErr, RollbackError: rollbackErr}
					r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateError,
						"新建存档失败且服务器停止未确认；现场已保留，需要修复后再恢复。", "new_game_rollback_failed", jobCtx.ID)
					return
				}
				composeState, composeInspectErr := r.lifecycle.ComposePs(context.Background(), r.instance.DataDir)
				if composeInspectErr != nil || !runtimeServicesConfirmedStopped(composeState, runtimeStopServicesForSteamInvite(r.instance.DataDir)) {
					if composeInspectErr == nil {
						composeInspectErr = errors.New("selected runtime services remain active after stop")
					}
					rollbackErr := newGameTx.failRollback(fmt.Errorf("confirm server stopped before rollback: %w", composeInspectErr))
					retErr = &NewGameTransactionError{Code: "new_game_rollback_failed", Message: "停服后无法确认服务器终态；已保留 owner 和现场", Cause: retErr, RollbackError: rollbackErr}
					r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateError,
						"停服后无法确认服务器终态；未继续文件回滚。", "new_game_rollback_failed", jobCtx.ID)
					return
				}
				composeStarted = false
			}
			finalCode := code
			if rollbackErr := newGameTx.continueRollback(); rollbackErr != nil {
				retErr = &NewGameTransactionError{Code: "new_game_rollback_failed", Message: "新建存档失败且回滚未完整完成", Cause: retErr, RollbackError: rollbackErr}
				finalCode = "new_game_rollback_failed"
			} else if releaseErr := newGameTx.releaseOwner(); releaseErr != nil {
				retErr = &NewGameTransactionError{Code: "new_game_owner_release_failed", Message: "新建存档已完整回滚但 owner 清理失败", Cause: retErr, RollbackError: releaseErr}
				finalCode = "new_game_owner_release_failed"
			}
			finalState := storage.InstanceStateStopped
			if newGameTx.record.Stage == newGameStateRollbackFail || newGameTx.record.Stage == newGameStateRollingBack {
				finalState = storage.InstanceStateError
			}
			r.driver.updatePhase(context.Background(), r.instance.ID, finalState,
				"创建新存档失败: "+paneldocker.RedactString(retErr.Error()), finalCode, jobCtx.ID)
		}()
		needsConfigReplay := !resumed || newGameTx.record.Stage == newGameStatePreparing || newGameTx.record.Stage == newGameStateConfigured
		if resumed && !needsConfigReplay {
			if retErr = refreshPendingNewGameMarker(newGameTx); retErr != nil {
				return retErr
			}
			if newGameTx.record.DiskVerifiedAt == nil &&
				newGameTx.record.Stage != newGameStateMarkerWritten && newGameTx.record.Stage != newGameStateModsPrepared {
				if retErr = newGameTx.refreshRuntimeCatalogRequest(); retErr != nil {
					return retErr
				}
			}
		}
		changed, syncErr := r.driver.EnsureManagedSMAPIBundledMods(ctx, r.instance.DataDir, imageRef, func(line string) {
			_, _ = jobCtx.Info(context.Background(), "[smapi-sync] "+paneldocker.RedactString(line))
		})
		if syncErr != nil {
			r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateStopped,
				"SMAPI 内置支持 Mod 同步失败，已阻止创建存档", "smapi_bundled_sync_failed", jobCtx.ID)
			return &NewGameTransactionError{Code: "smapi_bundled_sync_failed", Message: "创建存档前同步 SMAPI 内置支持 Mod 失败", Cause: syncErr}
		}
		if changed {
			_, _ = jobCtx.Info(ctx, "已在持久 owner 保护下同步 SMAPI 内置支持 Mod。")
		}
		if needsConfigReplay {
			if retErr = newGameTx.prepareConfigAndMarker(); retErr != nil {
				return retErr
			}
			_, _ = jobCtx.Info(ctx, fmt.Sprintf("新建存档事务已准备：%s", newGameTx.record.TransactionID))
		} else {
			_, _ = jobCtx.Info(ctx, fmt.Sprintf("正在恢复新建存档事务：%s（阶段 %s）", newGameTx.record.TransactionID, newGameTx.record.Stage))
		}
	}
	r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateStarting, "正在启动服务器...", "starting", jobCtx.ID)

	if err := r.ensureJunimoServerMod(ctx, jobCtx); err != nil {
		r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateStopped,
			"同步 JunimoServer 官方 Mod 失败: "+err.Error(), "junimo_server_mod_failed", jobCtx.ID)
		return err
	}

	// A Control sync failure is a hard start gate. Running the game with an old
	// DLL while the Panel assumes the new command contract is unsafe.
	if !r.preserveControlMod {
		if err := installSMAPIMod(r.instance.DataDir); err != nil {
			r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateStopped,
				"Control Mod 同步失败，已阻止服务器启动", "control_sync_failed", jobCtx.ID)
			return fmt.Errorf("sync Control mod before start: %w", err)
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
	if err := r.clearRuntimeControlSnapshots(ctx, jobCtx); err != nil {
		r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateStopped,
			"无法清理上一轮 Control 运行快照，已阻止启动", "control_runtime_snapshot_cleanup_failed", jobCtx.ID)
		return fmt.Errorf("clear stale Control runtime snapshots: %w", err)
	}
	if r.newGame && newGameTx.record.DurableSaveCommandID != "" {
		_, _ = jobCtx.Info(ctx, "恢复事务保留了同一 save-now commandId 的命令结果现场用于幂等观察。")
	}

	if changed, err := ensureRuntimeContEnvFix(r.instance.DataDir, sjconfig.SteamInviteEnabled(r.instance.DataDir)); err != nil {
		_, _ = jobCtx.Info(ctx, fmt.Sprintf("warning: ensure JunimoServer static init compatibility mounts failed: %v", err))
	} else if changed {
		_, _ = jobCtx.Info(ctx, "JunimoServer static init compatibility mounts have been applied.")
	}

	if r.newGame {
		var modPrepareErr error
		if newGameTx.record.Stage == newGameStatePreparing || newGameTx.record.Stage == newGameStateConfigured || newGameTx.record.Stage == newGameStateMarkerWritten {
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
			if newGameSelection == nil {
				_, _ = jobCtx.Info(ctx, "New save mod defaults applied: third-party mods are disabled.")
			} else {
				_, _ = jobCtx.Info(ctx, fmt.Sprintf("已准备模组农场 %s 的必要 Mod 集合（%d 个组件）。", newGameSelection.FarmTypeID, len(newGameSelection.EnabledModKeys)))
			}
		}
		if newGameTx.record.Stage == newGameStateModsPrepared {
			if err := newGameTx.prepareRuntimeCatalogRequest(); err != nil {
				return err
			}
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

	// compose up can return non-zero after creating or starting only part of the
	// project. Treat the runtime as potentially live before invoking it so the
	// failure defer must confirm ComposeDown before restoring transaction files.
	if r.newGame {
		composeStarted = true
	}
	services := runtimeServicesForSteamInvite(r.instance.DataDir)
	result, err := r.lifecycle.ComposeRecreateServices(ctx, r.instance.DataDir, services...)
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
		return fmt.Errorf("docker compose up selected runtime services: %w", err)
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
	if stopped, err := r.waitForControlRuntime(ctx, jobCtx); err != nil {
		if stopped {
			composeStarted = false
		}
		return err
	}
	steamInviteEnabled := sjconfig.SteamInviteEnabled(r.instance.DataDir)
	if steamInviteEnabled {
		r.clearStaleInviteCode(ctx, jobCtx)
	}

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

	if steamInviteEnabled {
		r.driver.updatePhaseWithSteamInviteWarmup(ctx, r.instance.ID, storage.InstanceStateRunning,
			"服务器容器已启动，正在初始化游戏...", "server_initializing", jobCtx.ID)
	} else {
		r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateRunning,
			"服务器容器已启动，正在初始化游戏...", "server_initializing", jobCtx.ID)
	}

	if steamInviteEnabled {
		_, _ = jobCtx.Info(ctx, fmt.Sprintf("服务器已启动；Steam 邀请码将后台尝试获取，最多 %d 次，不影响局域网/IP 直连。", backgroundInviteAttempts))
		r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateRunning,
			"服务器运行中（Steam 邀请码后台获取中，局域网直连可用）", "running", jobCtx.ID)
		r.startInviteCodePolling()
	} else {
		_, _ = jobCtx.Info(ctx, "服务器已启动；当前未启用 Steam 邀请码，局域网/IP 直连可用。")
		r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateRunning,
			"服务器运行中（局域网/IP 直连）", "running", jobCtx.ID)
	}

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

	err := stopRuntimeServices(ctx, r.lifecycle, r.instance.DataDir)
	if err != nil {
		r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateError,
			"停止失败，请检查 Docker/Compose 状态。", "stop_failed", jobCtx.ID)
		return fmt.Errorf("stop runtime services: %w", err)
	}
	if r.driver != nil {
		r.driver.clearDriverPayloadInviteCode(context.Background(), r.instance.ID)
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
	if r.driver != nil {
		r.driver.clearDriverPayloadInviteCode(context.Background(), r.instance.ID)
	}
	r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateStarting, "正在重启...", "restarting", jobCtx.ID)
	if changed, warning, err := ensureServerPlayerAuthEnvironmentForLifecycle(r.instance.DataDir); err != nil {
		r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateError,
			"迁移玩家加入保护运行环境失败，已阻止重启", "player_auth_compose_migration_failed", jobCtx.ID)
		return fmt.Errorf("migrate player authentication Compose environment before restart: %w", err)
	} else if changed {
		_, _ = jobCtx.Info(ctx, "已为旧实例补齐玩家加入保护运行环境。")
	} else if warning != "" {
		_, _ = jobCtx.Info(ctx, "warning: "+warning)
	}
	steamInviteEnabled := sjconfig.SteamInviteEnabled(r.instance.DataDir)
	if steamInviteEnabled {
		r.removeInviteCodeFile(ctx, jobCtx)
	}
	if err := r.clearRuntimeControlSnapshots(ctx, jobCtx); err != nil {
		r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateError,
			"无法清理上一轮 Control 运行快照，已阻止重启", "control_runtime_snapshot_cleanup_failed", jobCtx.ID)
		return fmt.Errorf("clear stale Control runtime snapshots before restart: %w", err)
	}

	if err := r.ensureJunimoServerMod(ctx, jobCtx); err != nil {
		r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateError,
			"同步 JunimoServer 官方 Mod 失败: "+err.Error(), "junimo_server_mod_failed", jobCtx.ID)
		return err
	}

	if changed, err := ensureRuntimeContEnvFix(r.instance.DataDir, steamInviteEnabled); err != nil {
		_, _ = jobCtx.Info(ctx, fmt.Sprintf("warning: ensure JunimoServer static init compatibility mounts failed: %v", err))
	} else if changed {
		_, _ = jobCtx.Info(ctx, "JunimoServer static init compatibility mounts have been applied.")
	}
	if quarantined, err := QuarantineSMAPIBundledDuplicates(r.instance.DataDir); err != nil {
		r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateError,
			"repair duplicate SMAPI support mods failed: "+err.Error(), "smapi_mod_dedup_failed", jobCtx.ID)
		return err
	} else if len(quarantined) > 0 {
		_, _ = jobCtx.Info(ctx, fmt.Sprintf("已隔离重复的 SMAPI 内置组件：%s。原文件保留在私有隔离目录。", strings.Join(quarantined, "、")))
	}

	result, err := r.lifecycle.ComposeRecreateServices(ctx, r.instance.DataDir, runtimeServicesForSteamInvite(r.instance.DataDir)...)
	if err != nil {
		r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateError,
			"重启失败: "+result.Stderr, "restart_failed", jobCtx.ID)
		return fmt.Errorf("docker compose recreate server: %w", err)
	}
	_, _ = jobCtx.Info(ctx, "重启完成，等待服务器就绪...")

	if err := r.waitForServer(ctx, jobCtx); err != nil {
		r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateError,
			"重启后服务器未就绪", "restart_timeout", jobCtx.ID)
		return err
	}
	if _, err := r.waitForControlRuntime(ctx, jobCtx); err != nil {
		return err
	}
	if steamInviteEnabled {
		r.clearStaleInviteCode(ctx, jobCtx)
		_, _ = jobCtx.Info(ctx, fmt.Sprintf("服务器已重启；Steam 邀请码将后台尝试获取，最多 %d 次，不影响局域网/IP 直连。", backgroundInviteAttempts))
		r.driver.updatePhaseWithSteamInviteWarmup(ctx, r.instance.ID, storage.InstanceStateRunning,
			"服务器运行中（Steam 邀请码后台获取中，局域网直连可用）", "running", jobCtx.ID)
		r.startInviteCodePolling()
	} else {
		_, _ = jobCtx.Info(ctx, "服务器已重启；当前未启用 Steam 邀请码，局域网/IP 直连可用。")
		r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateRunning,
			"服务器运行中（局域网/IP 直连）", "running", jobCtx.ID)
	}

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
	hostWorkDir := workDir
	if r.driver != nil {
		hostWorkDir, err = r.driver.dockerHostPath(workDir)
		if err != nil {
			return fmt.Errorf("map JunimoServer sync directory for Docker: %w", err)
		}
	}
	extractedDir, err := extractJunimoServerMod(ctx, r.lifecycle, imageRef, workDir, hostWorkDir, expectedVersion)
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
	if !sjconfig.SteamInviteEnabled(r.instance.DataDir) {
		return ""
	}
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
	steamInviteEnabled := sjconfig.SteamInviteEnabled(r.instance.DataDir)
	if steamInviteEnabled && serverLogShowsSteamAuthUnavailable(result.Stdout) {
		// Junimo can emit this line while the optional Auth sidecar is still
		// restoring its persisted session. A recent server log is not definitive
		// evidence that the saved session is invalid, so keep the completed flag
		// and let invite-code polling surface the transient waiting state.
		_, _ = jobCtx.Info(ctx, "Steam 邀请服务仍在恢复授权会话，继续等待邀请码；局域网直连不受影响。")
	} else if steamInviteEnabled && sjconfig.SteamAuthLoggedIn(r.instance.DataDir) && serverLogShowsSteamAuthServiceNotReady(result.Stdout) {
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
	if !sjconfig.SteamInviteEnabled(r.instance.DataDir) {
		return
	}
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
	if !sjconfig.SteamInviteEnabled(r.instance.DataDir) {
		return
	}
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
	if !sjconfig.SteamInviteEnabled(r.instance.DataDir) {
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
	if !sjconfig.SteamInviteEnabled(r.instance.DataDir) {
		return ""
	}
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

// waitForControlRuntime keeps the instance in starting until this launch has
// produced an explicit Control runtime snapshot. A missing options.json is
// pending evidence and can only become a start timeout; version mismatch is
// reserved for a valid snapshot which names a different version.
//
// The bool result reports whether this helper successfully stopped Compose
// after a terminal gate failure. Callers use it to prevent duplicate cleanup.
func (r *lifecycleRunner) waitForControlRuntime(ctx context.Context, jobCtx *jobs.Context) (bool, error) {
	if r.preserveControlMod {
		return false, nil
	}
	timeout := r.driver.runtimeUpdateServerTimeout
	if timeout <= 0 {
		timeout = readyStateTimeout
	}
	deadline := time.Now().Add(timeout)
	startedAt := time.Now()
	r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateStarting,
		"服务器容器已启动，正在等待 SMAPI Control 就绪...", "control_runtime_starting", jobCtx.ID)
	_, _ = jobCtx.Info(ctx, fmt.Sprintf("等待本次启动的 Control 运行状态，最长 %s。", timeout.Round(time.Second)))

	for {
		result := InspectControlRuntimeGate(r.instance.DataDir)
		switch result.State {
		case ControlRuntimeGateReady:
			_, _ = jobCtx.Info(ctx, fmt.Sprintf("Control 运行版本已验收：%s。", result.Actual))
			return false, nil
		case ControlRuntimeGateVersionMismatch:
			message := fmt.Sprintf("SMAPI 明确加载了错误的 Control 版本（实际 %s，期望 %s），服务器已停止", result.Actual, result.Expected)
			return r.stopAfterControlRuntimeFailure(jobCtx, storage.InstanceStateError, message, ControlRuntimeCodeVersionMismatch,
				fmt.Errorf("Control runtime version mismatch: actual=%s expected=%s", result.Actual, result.Expected))
		case ControlRuntimeGateInvalid:
			message := "Control 运行状态无效，服务器已停止；游戏文件仍保留，请查看诊断后重试"
			return r.stopAfterControlRuntimeFailure(jobCtx, storage.InstanceStateError, message, result.Code,
				fmt.Errorf("invalid Control runtime evidence: %s", result.Code))
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			message := "等待 Control 运行状态超时，服务器已停止；游戏文件仍完整，可重试启动"
			return r.stopAfterControlRuntimeFailure(jobCtx, storage.InstanceStateStopped, message, "control_runtime_start_timeout",
				errors.New("timed out waiting for Control runtime state"))
		}
		wait := startProgressInterval
		if remaining < wait {
			wait = remaining
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return false, ctx.Err()
		case <-timer.C:
			elapsed := time.Since(startedAt).Round(time.Second)
			remaining = time.Until(deadline).Round(time.Second)
			if remaining < 0 {
				remaining = 0
			}
			_, _ = jobCtx.Info(ctx, fmt.Sprintf("Control 尚未写出明确运行状态，仍在启动（已等待 %s，剩余 %s）。", elapsed, remaining))
		}
	}
}

func (r *lifecycleRunner) stopAfterControlRuntimeFailure(
	jobCtx *jobs.Context,
	state string,
	message string,
	code string,
	cause error,
) (bool, error) {
	cleanupCtx, cancel := context.WithTimeout(context.Background(), controlCleanupTimeout)
	defer cancel()
	err := stopRuntimeServices(cleanupCtx, r.lifecycle, r.instance.DataDir)
	if err != nil {
		detail := err.Error()
		r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateError,
			"Control 启动验收失败，且未能确认服务器已停止，请立即检查 Docker 状态", "control_runtime_cleanup_failed", jobCtx.ID)
		return false, fmt.Errorf("%w; stop server after Control gate failure: %s", cause, paneldocker.RedactString(detail))
	}
	r.driver.updatePhase(context.Background(), r.instance.ID, state, message, code, jobCtx.ID)
	_, _ = jobCtx.Warn(context.Background(), message)
	return true, cause
}

func (r *lifecycleRunner) clearRuntimeControlSnapshots(ctx context.Context, jobCtx *jobs.Context) error {
	paths := []string{
		filepath.Join(controlDir(r.instance.DataDir), "status.json"),
		filepath.Join(controlDir(r.instance.DataDir), "players.json"),
		filepath.Join(controlDir(r.instance.DataDir), "options.json"),
	}
	removed := false
	var cleanupErr error
	for _, path := range paths {
		if err := os.Remove(path); err == nil {
			removed = true
		} else if err != nil && !os.IsNotExist(err) {
			_, _ = jobCtx.Warn(ctx, fmt.Sprintf("清理旧运行状态文件失败：%s: %v", filepath.Base(path), err))
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("remove %s: %w", filepath.Base(path), err))
		}
	}
	if removed {
		_, _ = jobCtx.Info(ctx, "已清理上一轮 SMAPI 运行状态快照，等待本次启动写入新状态。")
	}
	return cleanupErr
}

// clearStaleInviteCode removes /tmp/invite-code.txt only when it still contains
// the invite code recorded before this lifecycle operation. This prevents
// docker compose restart/up from reusing a stale /tmp file while avoiding
// deletion of a fresh code that Junimo may have already written.
func (r *lifecycleRunner) clearStaleInviteCode(ctx context.Context, jobCtx *jobs.Context) {
	if !sjconfig.SteamInviteEnabled(r.instance.DataDir) {
		return
	}
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
	if !sjconfig.SteamInviteEnabled(r.instance.DataDir) {
		return
	}
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
	if !sjconfig.SteamInviteEnabled(r.instance.DataDir) {
		return "", ErrSteamInviteDisabled
	}
	execCtx, cancel := context.WithTimeout(ctx, inviteCodeTimeout)
	defer cancel()

	catResult, catErr := r.lifecycle.ComposeExecPipe(execCtx, r.instance.DataDir, "server",
		"", "sh", "-c", inviteCodeReadScript)
	if catErr != nil {
		return "", fmt.Errorf("read /tmp/invite-code.txt: %w", catErr)
	}
	return strings.TrimSpace(catResult.Stdout), nil
}

// GetInviteCode fetches the invite code for a running instance (used by HTTP handler).
func (d *Driver) GetInviteCode(ctx context.Context, instance registry.Instance) (string, error) {
	stored, err := d.store.GetInstance(ctx, instance.ID)
	if err != nil {
		return "", fmt.Errorf("load instance: %w", err)
	}
	if !sjconfig.SteamInviteEnabled(stored.DataDir) {
		return "", ErrSteamInviteDisabled
	}
	ld, ok := d.docker.(LifecycleDockerService)
	if !ok {
		return "", fmt.Errorf("docker 服务不支持生命周期操作")
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

func (d *Driver) clearDriverPayloadInviteCode(ctx context.Context, instanceID string) {
	if d == nil {
		return
	}
	d.inviteCodeMu.Lock()
	delete(d.inviteCodeCache, instanceID)
	d.inviteCodeMu.Unlock()
	if d.store == nil {
		return
	}
	inst, err := d.store.GetInstance(ctx, instanceID)
	if err != nil {
		d.logger.Warn("clear invite code: load instance", "error", err)
		return
	}
	newPayload, changed := removeInviteCodeFromPayload(inst.DriverPayload)
	if !changed {
		return
	}
	if _, err := d.store.UpdateInstanceState(ctx, storage.UpdateInstanceStateParams{
		ID:            instanceID,
		State:         inst.State,
		StateMessage:  inst.StateMessage.String,
		DriverPhase:   inst.DriverPhase,
		DriverPayload: newPayload,
	}); err != nil {
		d.logger.Warn("clear invite code: update state", "error", err)
	}
}

// sendNewGameCommand waits for the JunimoServer HTTP API to be ready, then calls
// POST /newgame to create a fresh save using the current server-settings.json values.
// Existing saves are preserved; junimohost.gameloader.json is updated automatically.
func (r *lifecycleRunner) sendNewGameCommandLegacy(ctx context.Context, jobCtx *jobs.Context, tx *newGameTransaction) error {
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

func mergeSteamInviteWarmupStartedAt(existing string, startedAt time.Time) string {
	payload := map[string]any{}
	if strings.TrimSpace(existing) != "" {
		if err := jsonUnmarshal(existing, &payload); err != nil {
			return existing
		}
	}
	payload[steamInviteWarmupStartedAtPayloadKey] = startedAt.UTC().Format(time.RFC3339Nano)
	b, err := marshalJSON(payload)
	if err != nil {
		return existing
	}
	return strings.TrimSpace(string(b))
}

// SteamInviteWarmupStartedAt reads the dedicated persisted start time for the
// current optional Auth runtime generation. It is intentionally independent of
// instances.updated_at, which also changes for unrelated payload writes.
func SteamInviteWarmupStartedAt(existing string) (time.Time, bool) {
	if strings.TrimSpace(existing) == "" {
		return time.Time{}, false
	}
	payload := map[string]any{}
	if err := jsonUnmarshal(existing, &payload); err != nil {
		return time.Time{}, false
	}
	raw, ok := payload[steamInviteWarmupStartedAtPayloadKey].(string)
	if !ok {
		return time.Time{}, false
	}
	startedAt, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(raw))
	if err != nil {
		return time.Time{}, false
	}
	return startedAt.UTC(), true
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

func removeInviteCodeFromPayload(existing string) (string, bool) {
	if strings.TrimSpace(existing) == "" {
		return existing, false
	}
	payload := map[string]any{}
	if err := jsonUnmarshal(existing, &payload); err != nil {
		return existing, false
	}
	if _, ok := payload["invite_code"]; !ok {
		return existing, false
	}
	delete(payload, "invite_code")
	b, err := marshalJSON(payload)
	if err != nil {
		return existing, false
	}
	return strings.TrimSpace(string(b)), true
}

func jsonUnmarshal(s string, v any) error {
	return json.Unmarshal([]byte(s), v)
}
