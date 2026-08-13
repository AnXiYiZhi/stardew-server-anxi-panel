package stardew_junimo

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

const (
	DriverID   = "stardew_junimo"
	DriverName = "Stardew Valley / JunimoServer"

	installJobTimeout = 2 * time.Hour

	// TestedImageTag is the JunimoServer image tag this panel version was validated against.
	// Server image: sdvd/server:<tag>. The steam-auth sidecar is patched separately
	// through DefaultSteamServiceImage because slow Steam networks need connection/auth retries.
	// Update this constant (and bump panel version) after testing a new JunimoServer release.
	TestedImageTag = "1.5.0-preview.125"

	// DefaultSteamServiceImage is the patched steam-auth sidecar used by new instances.
	// It should match https://github.com/AnXiYiZhi/junimo-server-steam-service-cn.
	DefaultServerImage                 = sjconfig.DefaultServerImage
	DefaultServerImageCandidates       = sjconfig.DefaultServerImageCandidates
	DefaultSteamServiceImage           = sjconfig.DefaultSteamServiceImage
	DefaultSteamServiceImageCandidates = sjconfig.DefaultSteamServiceImageCandidates
	DefaultSteamCMDImage               = sjconfig.DefaultSteamCMDImage
	DefaultSteamCMDImageCandidates     = sjconfig.DefaultSteamCMDImageCandidates
	DefaultSMAPIVersion                = sjconfig.DefaultSMAPIVersion
	DefaultSMAPIDownloadURLs           = sjconfig.DefaultSMAPIDownloadURLs

	DefaultSteamClientConnectTimeoutSeconds  = sjconfig.DefaultSteamClientConnectTimeoutSeconds
	DefaultSteamClientConnectRetries         = sjconfig.DefaultSteamClientConnectRetries
	DefaultSteamAuthSessionRetries           = sjconfig.DefaultSteamAuthSessionRetries
	DefaultSteamAuthSessionRetryDelaySeconds = sjconfig.DefaultSteamAuthSessionRetryDelaySeconds

	// LatestImageTag is always "latest"; it follows upstream and may break compatibility.
	LatestImageTag = "latest"

	// installVerificationMissingExitCode is emitted by the short-lived verifier
	// container when the game-data volume is readable but incomplete.
	installVerificationMissingExitCode = 42
)

// DockerService defines what the driver needs from the Docker layer.
type DockerService interface {
	ComposePs(ctx context.Context, dir string) (paneldocker.ComposePsResult, error)
	ComposePullStreaming(ctx context.Context, dir string, services []string, lineHandler func(line string)) (paneldocker.CommandResult, error)
	PullImageStreaming(ctx context.Context, dir string, imageRef string, lineHandler func(line string)) (paneldocker.CommandResult, error)
	ImageInspect(ctx context.Context, dir string, imageRef string) (paneldocker.CommandResult, error)
	// RunSteamAuthTTY creates the steam-auth container via the Docker API with Tty:true
	// so Console.ReadKey() works for interactive menu selection. guardCh provides raw
	// stdin bytes (callers append "\n" for ReadLine, omit "\n" for ReadKey).
	RunSteamAuthTTY(ctx context.Context, dataDir string, opts paneldocker.SteamAuthRunOpts, guardCh <-chan string, lineHandler func(string)) (int, error)
	RunContainerTTY(ctx context.Context, opts paneldocker.ContainerTTYRunOpts, guardCh <-chan string, lineHandler func(string)) (int, error)
	// RemoveContainersByVolume force-removes containers still holding the given volumes.
	RemoveContainersByVolume(ctx context.Context, workDir string, names []string) (paneldocker.CommandResult, error)
	// RemoveVolumes deletes the named Docker volumes (force: missing volumes are a no-op).
	RemoveVolumes(ctx context.Context, workDir string, names []string) (paneldocker.CommandResult, error)
}

// StateStore defines what the driver needs from the storage layer.
type StateStore interface {
	GetInstance(ctx context.Context, id string) (storage.Instance, error)
	UpdateInstanceState(ctx context.Context, params storage.UpdateInstanceStateParams) (storage.Instance, error)
}

type activeJobStateStore interface {
	UpdateInstanceStateForActiveJob(ctx context.Context, params storage.UpdateInstanceStateForActiveJobParams) (storage.Instance, error)
}

// InstanceMutationGuard is an optional driver capability used by the Web layer
// before operations that require a fully stopped, transaction-free instance.
// It keeps Stardew-specific owner knowledge in the driver instead of copying
// transaction file rules into HTTP handlers.
type InstanceMutationGuard interface {
	EnsureOfflineMutationAllowed(ctx context.Context, instance registry.Instance) error
}

// InstanceMutationOwnershipGuard is used by automatic workflows that may run
// while the server is online (for example a scheduled shutdown). It checks only
// the persistent new-game transaction owner and performs no Docker inspection.
type InstanceMutationOwnershipGuard interface {
	EnsureMutationOwnershipAvailable(ctx context.Context, instance registry.Instance) error
}

// InstanceMutationExecutor linearizes Web-owned filesystem mutations with
// lifecycle/new-game ownership. The callback executes while runtimeUpdateMu is
// held, so a successful owner check cannot be invalidated before the write.
// Callers must not invoke another Driver method which acquires runtimeUpdateMu
// from inside the callback.
type InstanceMutationExecutor interface {
	WithMutationOwnership(ctx context.Context, instance registry.Instance, mutate func() error) error
	WithOfflineMutation(ctx context.Context, instance registry.Instance, mutate func() error) error
}

// Driver implements registry.GameDriver for Stardew Valley / JunimoServer.
type Driver struct {
	docker           DockerService
	logger           *slog.Logger
	jobs             *jobs.Manager
	store            StateStore
	panelVersion     string
	containerDataDir string
	hostDataDir      string

	// guardChans maps running install job ID → channel for Steam Guard input.
	mu         sync.Mutex
	guardChans map[string]chan string

	runtimeUpdateMu            sync.Mutex
	inviteCodeMu               sync.Mutex
	inviteCodeCache            map[string]inviteCodeCacheEntry
	inviteCodeFlights          map[string]*inviteCodeFlight
	saveImportRunMu            sync.Mutex
	runtimeUpdatePollInterval  time.Duration
	runtimeUpdateAuthTimeout   time.Duration
	runtimeUpdateServerTimeout time.Duration
	runtimeUpdateStopTimeout   time.Duration
	backupMaintenanceInterval  time.Duration
	requiredRuntimeMu          sync.Mutex
	requiredRuntimeRunning     map[string]bool
	installationEvidenceMu     sync.Mutex
	installationEvidence       map[string]requiredFilesEvidence
}

type DriverOptions struct {
	PanelVersion     string
	ContainerDataDir string
	HostDataDir      string
}

// New creates a Driver.  jobs and store may be nil for tests that only use
// the driver skeleton (Prepare, Status).
func New(docker DockerService, logger *slog.Logger, jobManager *jobs.Manager, store StateStore, panelVersions ...string) *Driver {
	options := DriverOptions{}
	if len(panelVersions) > 0 {
		options.PanelVersion = panelVersions[0]
	}
	return NewWithOptions(docker, logger, jobManager, store, options)
}

// NewWithOptions creates a Driver with the Panel container-to-host data path
// mapping required by Docker bind mounts created through the host daemon.
func NewWithOptions(docker DockerService, logger *slog.Logger, jobManager *jobs.Manager, store StateStore, options DriverOptions) *Driver {
	if logger == nil {
		logger = slog.Default()
	}
	panelVersion := "dev"
	if strings.TrimSpace(options.PanelVersion) != "" {
		panelVersion = strings.TrimSpace(options.PanelVersion)
	}
	return &Driver{
		docker:                     docker,
		logger:                     logger,
		jobs:                       jobManager,
		store:                      store,
		panelVersion:               panelVersion,
		containerDataDir:           strings.TrimSpace(options.ContainerDataDir),
		hostDataDir:                strings.TrimSpace(options.HostDataDir),
		guardChans:                 make(map[string]chan string),
		inviteCodeCache:            make(map[string]inviteCodeCacheEntry),
		inviteCodeFlights:          make(map[string]*inviteCodeFlight),
		runtimeUpdatePollInterval:  2 * time.Second,
		runtimeUpdateAuthTimeout:   10 * time.Minute,
		runtimeUpdateServerTimeout: 20 * time.Minute,
		runtimeUpdateStopTimeout:   10 * time.Minute,
		backupMaintenanceInterval:  2 * time.Second,
		requiredRuntimeRunning:     make(map[string]bool),
		installationEvidence:       make(map[string]requiredFilesEvidence),
	}
}

// ── registry.GameDriver interface ─────────────────────────────────────────────

func (d *Driver) ID() string   { return DriverID }
func (d *Driver) Name() string { return DriverName }

// PrepareFarmMods serializes the transient new-game Mod preparation with
// lifecycle/runtime update operations. It never creates a save or starts one.
func (d *Driver) PrepareFarmMods(ctx context.Context, instance registry.Instance, farmTypeID string) (NewGameModSelection, error) {
	d.runtimeUpdateMu.Lock()
	defer d.runtimeUpdateMu.Unlock()
	if err := rejectUnfinishedNewGameOwner(instance.DataDir); err != nil {
		return NewGameModSelection{}, err
	}
	if instance.State == storage.InstanceStateRunning || instance.State == storage.InstanceStateStarting {
		return NewGameModSelection{}, &NewGameModSelectionError{Code: "server_running", Message: "服务器运行中，无法准备模组农场"}
	}
	if d.jobs != nil {
		active, err := d.jobs.Active(ctx, storage.ListActiveJobsFilter{TargetType: "instance", TargetID: instance.ID})
		if err != nil {
			return NewGameModSelection{}, fmt.Errorf("list conflicting jobs: %w", err)
		}
		if len(active) > 0 {
			return NewGameModSelection{}, &NewGameModSelectionError{Code: "instance_busy", Message: "实例存在进行中的任务，请等待任务结束后再准备 Mod"}
		}
	}
	return PrepareNewGameMods(instance.DataDir, farmTypeID)
}

// CommandOutcome returns the current file-protocol state without waiting for
// the control mod or retrying ambiguous commands.
func (d *Driver) CommandOutcome(ctx context.Context, instance registry.Instance, commandID string) (CommandOutcome, error) {
	return GetCommandOutcome(instance.DataDir, commandID)
}

// Prepare ensures the instance working directory, docker-compose.yml, and .env
// exist.  It never overwrites files the user has already modified.
func (d *Driver) Prepare(ctx context.Context, instance registry.Instance) error {
	d.runtimeUpdateMu.Lock()
	defer d.runtimeUpdateMu.Unlock()
	if instance.DataDir == "" {
		return errors.New("instance data dir is empty")
	}
	// Prepare is also called during Panel bootstrap. An unfinished new-game
	// transaction exclusively owns its runtime files until a user explicitly
	// resumes it; bootstrap must not repair or rewrite anything underneath it.
	if err := rejectUnfinishedNewGameOwner(instance.DataDir); err != nil {
		return err
	}
	recoveries, err := RecoverImportTransactions(instance.DataDir)
	if err != nil {
		return fmt.Errorf("recover save import transactions: %w", err)
	} else if len(recoveries) > 0 {
		d.logger.Warn("discovered unfinished save import transactions", "instance", instance.ID, "count", len(recoveries))
	}

	// Create main directory and sub-directories. The named Docker volumes remain
	// official Junimo storage; local saves/mods directories are panel-owned future
	// extension points.
	for _, sub := range []string{
		"", "saves", "mods", ".local-container",
		filepath.Join(".local-container", "settings"),
		filepath.Join(".local-container", "saves"),
		filepath.Join(".local-container", "saves", "Saves"),
		filepath.Join(".local-container", "saves-templates"),
		filepath.Join(".local-container", "control"),
		filepath.Join(".local-container", "control", "commands"),
		filepath.Join(".local-container", "control", "command-results"),
		filepath.Join(".local-container", "cont-env"),
		filepath.Join(".local-container", "mods"),
		filepath.Join(".local-container", "mods-disabled"),
	} {
		dir := filepath.Join(instance.DataDir, sub)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create dir %s: %w", dir, err)
		}
	}

	// Never replace a Control DLL underneath a live or previously installed
	// runtime. The required runtime coordinator owns the guarded
	// save/backup/stop/sync/start/verify transaction for those instances.
	if instance.State == storage.InstanceStateUninitialized || instance.State == storage.InstanceStateAdminCreated {
		if err := installSMAPIMod(instance.DataDir); err != nil {
			d.logger.Warn("SMAPI mod install failed (non-fatal)", "instance", instance.ID, "error", err)
		}
	}

	// Write docker-compose.yml only when not already present.
	composePath := filepath.Join(instance.DataDir, "docker-compose.yml")
	if _, err := os.Stat(composePath); err == nil {
		// keep user edits
	} else if os.IsNotExist(err) {
		if err := os.WriteFile(composePath, []byte(junimoComposeTemplate), 0o644); err != nil {
			return fmt.Errorf("write docker-compose.yml: %w", err)
		}
		if _, err := os.Stat(composePath); err != nil {
			return fmt.Errorf("verify docker-compose.yml: %w", err)
		}
		d.logger.Info("wrote docker-compose.yml", "instance", instance.ID)
	} else {
		return fmt.Errorf("stat docker-compose.yml: %w", err)
	}
	if _, err := EnsureServerContEnvFix(instance.DataDir); err != nil {
		return fmt.Errorf("ensure server static init compatibility fix: %w", err)
	}

	// Write .env only when not already present.
	envPath := filepath.Join(instance.DataDir, ".env")
	if _, err := os.Stat(envPath); err == nil {
		// keep user edits
	} else if os.IsNotExist(err) {
		tpl := sjconfig.EmptyEnvTemplate()
		tpl["IMAGE_VERSION"] = TestedImageTag
		if err := sjconfig.UpdateEnvFile(envPath, tpl); err != nil {
			return fmt.Errorf("write initial .env: %w", err)
		}
		if _, err := os.Stat(envPath); err != nil {
			return fmt.Errorf("verify initial .env: %w", err)
		}
		d.logger.Info("wrote initial .env", "instance", instance.ID)
	} else {
		return fmt.Errorf("stat .env: %w", err)
	}
	if err := EnsureGameDataVolumeBinding(instance.DataDir); err != nil {
		return fmt.Errorf("ensure explicit game data volume binding: %w", err)
	}
	if err := d.resumeRecoveredImportDurableSaves(ctx, instance, recoveries); err != nil {
		return fmt.Errorf("resume durable save import verification: %w", err)
	}

	return nil
}

// Install validates credentials, creates an async install job, and returns its ID.
func (d *Driver) Install(ctx context.Context, req registry.InstallRequest) (*registry.Job, error) {
	if err := rejectUnfinishedNewGameOwner(req.Instance.DataDir); err != nil {
		return nil, err
	}
	manifest, manifestErr := sjconfig.BuiltInRuntimeStackManifest()
	if manifestErr != nil || !manifest.Installable() || !sjconfig.PanelVersionSatisfies(d.panelVersion, manifest.MinimumPanelVersion) {
		return nil, fmt.Errorf("内置兼容矩阵不是可安装的 recommended 状态")
	}
	if d.jobs == nil {
		return nil, fmt.Errorf("driver: job manager not configured")
	}
	if d.store == nil {
		return nil, fmt.Errorf("driver: state store not configured")
	}
	if req.SteamUsername == "" {
		return nil, fmt.Errorf("Steam 用户名不能为空")
	}
	if req.SteamPassword == "" {
		return nil, fmt.Errorf("Steam 密码不能为空")
	}
	if req.VNCPassword == "" {
		return nil, fmt.Errorf("VNC 密码不能为空")
	}
	d.runtimeUpdateMu.Lock()
	defer d.runtimeUpdateMu.Unlock()
	if err := rejectUnfinishedNewGameOwner(req.Instance.DataDir); err != nil {
		return nil, err
	}
	if err := d.rejectActiveSaveImport(ctx, req.Instance.ID); err != nil {
		return nil, err
	}
	if err := d.rejectActiveRuntimeUpdate(ctx, req.Instance.ID); err != nil {
		return nil, err
	}

	// Persist the instance so the runner has a stable snapshot.
	instance, err := d.store.GetInstance(ctx, req.Instance.ID)
	if err != nil {
		return nil, fmt.Errorf("load instance: %w", err)
	}

	imageTag := req.ImageTag
	if imageTag == "" {
		imageTag = TestedImageTag
	}
	if imageTag != manifest.Server.Tag {
		return nil, fmt.Errorf("只能安装当前 Panel 内置兼容矩阵中的精确 Junimo server 版本 %s", manifest.Server.Tag)
	}

	// reuse: reuse saved credentials without re-prompting the user for input.
	reuse := req.AutoDownload || req.SteamCMDRetry || req.AuthLoginOnly
	// steamAuthCompleted: durable ".env" flag set only after the steam-auth log
	// confirms login success. It backstops the phase inference in
	// authAlreadySucceeded so that even if the persisted phase was reset (e.g. an
	// interrupted install marked install_interrupted) we still skip steam-auth.
	envVals, _ := sjconfig.ReadEnvFile(filepath.Join(instance.DataDir, ".env"))
	steamAuthCompleted := strings.EqualFold(envVals["STEAM_AUTH_COMPLETED"], "true")
	// steamCMDDirect: skip image pull + steam-auth and resume the SteamCMD path
	// directly. Only when reusing credentials AND the instance has already passed
	// Steam authentication (resuming a SteamCMD phase, a post-auth download/
	// installed state, or the durable STEAM_AUTH_COMPLETED flag). Pre-auth failures
	// (pull_failed, timeouts) must NOT take this shortcut — they re-pull images and
	// run steam-auth again.
	// AuthLoginOnly must run steam-auth (that is where the login for invite codes
	// happens), so it must NOT take the SteamCMD shortcut even though the game is
	// already installed.
	steamCMDDirect := reuse && !req.ForceReauth && !req.AuthLoginOnly &&
		(shouldResumeSteamCMD(instance.DriverPhase) ||
			authAlreadySucceeded(instance.State, instance.DriverPhase) ||
			steamAuthCompleted)

	runner := &installRunner{
		driver:         d,
		instance:       instance,
		username:       req.SteamUsername,
		password:       req.SteamPassword,
		vncPass:        req.VNCPassword,
		imageTag:       imageTag,
		reuse:          reuse,
		steamCMDDirect: steamCMDDirect,
		forceReauth:    req.ForceReauth,
		authOnly:       req.AuthLoginOnly,
	}

	job, err := d.jobs.Start(ctx, jobs.Spec{
		Type:       "stardew_install",
		TargetType: "instance",
		TargetID:   req.Instance.ID,
		Exclusive:  true,
		CreatedBy:  req.ActorID,
		Timeout:    installJobTimeout,
		Run:        runner.run,
	})
	if err != nil {
		return nil, fmt.Errorf("start install job: %w", err)
	}

	d.logger.Info("install job started", "job_id", job.ID, "instance", req.Instance.ID)
	return &registry.Job{ID: job.ID}, nil
}

func shouldResumeSteamCMD(phase string) bool {
	switch phase {
	case "steamcmd_auth_running",
		"steamcmd_guard_choice_required",
		"steamcmd_guard_required",
		"steamcmd_guard_mobile_required",
		"steamcmd_downloading",
		"steamcmd_failed",
		"steamcmd_image_pull_failed":
		return true
	default:
		return false
	}
}

// authAlreadySucceeded reports whether the instance has already passed Steam
// authentication at least once, based on its persisted phase/state. These
// phases only occur after auth succeeds (download started/failed, post-auth
// failure) or once the game is installed, so they double as a durable,
// cross-session "auth done" signal that lets later operations skip steam-auth.
func authAlreadySucceeded(state, phase string) bool {
	switch phase {
	case "download_failed",
		"post_auth_failed",
		"smapi_install_failed",
		"game_downloading",
		"steam_sdk_downloading",
		"smapi_installing",
		"game_installed":
		return true
	}
	switch state {
	case storage.InstanceStateGameInstalled,
		storage.InstanceStateSaveRequired,
		storage.InstanceStateReadyToStart,
		storage.InstanceStateStarting,
		storage.InstanceStateRunning,
		storage.InstanceStateStopped:
		return true
	}
	return false
}

// SendSteamGuardInput writes a Steam Guard code to the active install job's
// stdin pipe.  Implements registry.SteamGuardSender.
func (d *Driver) SendSteamGuardInput(jobID string, input string) error {
	d.mu.Lock()
	ch, ok := d.guardChans[jobID]
	d.mu.Unlock()
	if !ok {
		return fmt.Errorf("没有正在等待 Steam Guard 输入的安装任务 %s", jobID)
	}
	select {
	case ch <- input:
		return nil
	default:
		return fmt.Errorf("Steam Guard 输入缓冲区已满，请稍后重试")
	}
}

// ReconcileState corrects stale states:
//   - "running"/"starting" when the Docker container is no longer actually up → "stopped"
//   - installed states when the expected local install directory is still empty → "error"
func (d *Driver) ReconcileState(ctx context.Context, instance storage.Instance) (storage.Instance, error) {
	if d.store == nil {
		return instance, nil
	}

	// Reconcile the persisted state against Docker's runtime truth whenever we can.
	if d.docker != nil {
		ps, err := d.docker.ComposePs(ctx, instance.DataDir)
		if err == nil {
			if serverServiceUp(ps.Services) {
				// A save-import maintenance runtime is intentionally kept in the
				// persisted stopped state. It must never be promoted to normal
				// running/ready merely because its private server container is up.
				if instance.DriverPhase == importMaintenancePhase {
					return instance, nil
				}
				if instance.State != storage.InstanceStateRunning {
					// A lifecycle job owns the transition from starting to running.
					// Reconcile must not publish running merely because Compose has
					// created the container while SMAPI/Control is still starting.
					if d.activeLifecycleOwner(ctx, instance.ID) {
						return instance, nil
					}
					// A completed Compose start is still owned by the persistent
					// new-game transaction until all four durability gates commit.
					// Do not erase recovery_required/error just because the container
					// remains up to preserve an ambiguous writer's evidence.
					if unfinishedNewGameOwnerExists(instance.DataDir) {
						return instance, nil
					}
					// For a container started outside the Panel, require fresh runtime
					// evidence before promoting persisted state. A missing options file
					// is pending, not proof that the expected Control version is loaded.
					controlGate := InspectControlRuntimeGate(instance.DataDir)
					if controlGate.State != ControlRuntimeGateReady {
						if instance.State == storage.InstanceStateStarting && controlGate.State == ControlRuntimeGatePending {
							return d.reconcileOrphanedStartingRuntime(ctx, instance)
						}
						return instance, nil
					}
					payload := instance.DriverPayload
					if payload == "" {
						payload = "{}"
					}
					return d.store.UpdateInstanceState(ctx, storage.UpdateInstanceStateParams{
						ID:            instance.ID,
						State:         storage.InstanceStateRunning,
						StateMessage:  "检测到 server 容器正在运行",
						DriverPhase:   "running",
						DriverPayload: payload,
					})
				}
				return instance, nil
			}
			if isRunningState(instance.State) {
				payload := instance.DriverPayload
				if payload == "" {
					payload = "{}"
				}
				return d.store.UpdateInstanceState(ctx, storage.UpdateInstanceStateParams{
					ID:            instance.ID,
					State:         storage.InstanceStateStopped,
					StateMessage:  "服务器容器已停止",
					DriverPhase:   "container_stopped",
					DriverPayload: payload,
				})
			}
		}
		// If ComposePs itself errors, don't change state — could be a transient Docker issue.
	}

	if !requiresInstalledFiles(instance.State) {
		return instance, nil
	}

	// The game files live in a named Docker volume, not the instance directory.
	// Never turn a transient Docker problem into a false "files missing" error.
	imageRef := gameInstallImage(instance.DataDir)
	if _, err := d.docker.ImageInspect(ctx, instance.DataDir, imageRef); err != nil {
		d.logger.Warn("skip installed-file reconciliation because server image is unavailable", "instance", instance.ID, "error", err)
		return instance, nil
	}
	ok, err := d.verifyGameDataVolume(ctx, instance.DataDir, imageRef, nil)
	if err != nil {
		d.logger.Warn("installed-file reconciliation failed", "instance", instance.ID, "error", err)
		return instance, nil
	}
	if ok {
		d.rememberInstallationEvidence(instance.ID, "ok")
		return instance, nil
	}
	d.rememberInstallationEvidence(instance.ID, "missing")
	return d.store.UpdateInstanceState(ctx, storage.UpdateInstanceStateParams{
		ID:           instance.ID,
		State:        storage.InstanceStateError,
		StateMessage: "游戏运行文件不完整，请重新安装或修复。",
		DriverPhase:  "install_verification_failed",
	})
}

func unfinishedNewGameOwnerExists(dataDir string) bool {
	owner, err := LoadNewGameOwner(dataDir)
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	if err != nil {
		return true
	}
	record, err := LoadNewGameTransaction(dataDir, owner.TransactionID)
	if err != nil {
		return true
	}
	return !isTerminalNewGameOwnerStage(record.Stage)
}

func (d *Driver) EnsureOfflineMutationAllowed(ctx context.Context, instance registry.Instance) error {
	return d.WithOfflineMutation(ctx, instance, nil)
}

func (d *Driver) EnsureMutationOwnershipAvailable(ctx context.Context, instance registry.Instance) error {
	return d.WithMutationOwnership(ctx, instance, nil)
}

func (d *Driver) WithMutationOwnership(ctx context.Context, instance registry.Instance, mutate func() error) error {
	d.runtimeUpdateMu.Lock()
	defer d.runtimeUpdateMu.Unlock()
	if err := rejectUnfinishedNewGameOwner(instance.DataDir); err != nil {
		return err
	}
	if d.jobs != nil {
		active, operation, found, err := d.findActiveLifecycleJob(ctx, instance.ID)
		if err != nil {
			return err
		}
		if found {
			return lifecycleConflict(operation, active.ID)
		}
	}
	if mutate == nil {
		return nil
	}
	return mutate()
}

func (d *Driver) WithOfflineMutation(ctx context.Context, instance registry.Instance, mutate func() error) error {
	d.runtimeUpdateMu.Lock()
	defer d.runtimeUpdateMu.Unlock()
	if err := rejectUnfinishedNewGameOwner(instance.DataDir); err != nil {
		return err
	}
	if d.jobs != nil {
		active, operation, found, err := d.findActiveLifecycleJob(ctx, instance.ID)
		if err != nil {
			return err
		}
		if found {
			return lifecycleConflict(operation, active.ID)
		}
	}
	if d.docker == nil {
		return &NewGameOwnerError{Code: "server_state_unknown", Message: "无法确认服务器已停止，请检查 Docker 状态后重试"}
	}
	ps, err := d.docker.ComposePs(ctx, instance.DataDir)
	if err != nil {
		return &NewGameOwnerError{Code: "server_state_unknown", Message: "无法确认服务器已停止，请检查 Docker 状态后重试", Cause: err}
	}
	if serverServiceUp(ps.Services) {
		return &NewGameOwnerError{Code: "server_running", Message: "服务器容器仍在运行，请先停止服务器再操作"}
	}
	if mutate == nil {
		return nil
	}
	return mutate()
}

// UpdateServerRuntimeSettings serializes the server-settings.json write with
// lifecycle/new-game ownership. A Web-side check followed by a direct file
// write would leave a TOCTOU window in which Start could reserve and snapshot
// the old file between those two operations.
func (d *Driver) UpdateServerRuntimeSettings(_ context.Context, instance registry.Instance, settings ServerRuntimeSettings) error {
	d.runtimeUpdateMu.Lock()
	defer d.runtimeUpdateMu.Unlock()
	if err := rejectUnfinishedNewGameOwner(instance.DataDir); err != nil {
		return err
	}
	return UpdateServerRuntimeSettings(instance.DataDir, settings)
}

func serverServiceUp(services []paneldocker.ComposeService) bool {
	for _, svc := range services {
		if svc.Service != "server" {
			continue
		}
		state := strings.ToLower(svc.State)
		return state == "running" || strings.HasPrefix(strings.ToLower(svc.Status), "up")
	}
	return false
}

func (d *Driver) reconcileOrphanedStartingRuntime(ctx context.Context, instance storage.Instance) (storage.Instance, error) {
	payload := instance.DriverPayload
	if payload == "" {
		payload = "{}"
	}
	updatedAt, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(instance.UpdatedAt))
	if err != nil {
		// Persist the first observation so a malformed historical timestamp cannot
		// reset the grace period on every status poll after a Panel restart.
		return d.store.UpdateInstanceState(ctx, storage.UpdateInstanceStateParams{
			ID:            instance.ID,
			State:         storage.InstanceStateStarting,
			StateMessage:  "检测到启动任务已中断；正在等待 Control 启动验收的剩余窗口，超时后将安全停服",
			DriverPhase:   "control_runtime_orphan_wait",
			DriverPayload: payload,
		})
	}
	if time.Now().UTC().Before(updatedAt.UTC().Add(readyStateTimeout)) {
		return instance, nil
	}

	lifecycle, ok := d.docker.(LifecycleDockerService)
	if !ok {
		return d.store.UpdateInstanceState(ctx, storage.UpdateInstanceStateParams{
			ID:            instance.ID,
			State:         storage.InstanceStateError,
			StateMessage:  "启动任务已中断且无法执行安全停服；游戏容器可能仍在运行，请检查 Docker 状态",
			DriverPhase:   "control_runtime_orphan_cleanup_failed",
			DriverPayload: payload,
		})
	}
	cleanupCtx, cancel := context.WithTimeout(context.Background(), controlCleanupTimeout)
	defer cancel()
	result, downErr := lifecycle.ComposeDown(cleanupCtx, instance.DataDir)
	if downErr == nil && result.ExitCode != 0 {
		downErr = fmt.Errorf("ComposeDown exited with code %d", result.ExitCode)
	}
	if downErr == nil {
		var ps paneldocker.ComposePsResult
		ps, downErr = lifecycle.ComposePs(cleanupCtx, instance.DataDir)
		if downErr == nil && serverServiceUp(ps.Services) {
			downErr = errors.New("server service remains running after ComposeDown")
		}
	}
	if downErr != nil {
		d.logger.Error("failed to stop orphaned starting runtime", "instance", instance.ID, "error", downErr)
		return d.store.UpdateInstanceState(ctx, storage.UpdateInstanceStateParams{
			ID:            instance.ID,
			State:         storage.InstanceStateError,
			StateMessage:  "启动任务已中断，且未能确认游戏容器已停止；请立即检查 Docker 状态",
			DriverPhase:   "control_runtime_orphan_cleanup_failed",
			DriverPayload: payload,
		})
	}
	return d.store.UpdateInstanceState(ctx, storage.UpdateInstanceStateParams{
		ID:            instance.ID,
		State:         storage.InstanceStateStopped,
		StateMessage:  "启动任务在 Control 就绪前中断并超时，游戏容器已安全停止；请手动重新启动",
		DriverPhase:   "control_runtime_orphan_stopped",
		DriverPayload: payload,
	})
}

func (d *Driver) activeLifecycleOwner(ctx context.Context, instanceID string) bool {
	if d.jobs == nil {
		return false
	}
	active, err := d.jobs.Active(ctx, storage.ListActiveJobsFilter{
		TargetType: "instance",
		TargetID:   instanceID,
		Types:      []string{lifecycleJobType},
	})
	if err != nil {
		// Fail closed: a transient job-store error must not let an unowned
		// reconcile write over the lifecycle runner's starting state.
		d.logger.Warn("failed to inspect active lifecycle owner during reconcile", "instance", instanceID, "error", err)
		return true
	}
	return len(active) > 0
}

// isRunningState returns true if the instance state indicates the container should be up.
func isRunningState(state string) bool {
	return state == storage.InstanceStateRunning ||
		state == storage.InstanceStateStarting
}

// Status returns a combined runtime + state view of the instance.
func (d *Driver) Status(ctx context.Context, instance registry.Instance) (*registry.ServerStatus, error) {
	status := &registry.ServerStatus{
		InstanceID:   instance.ID,
		DriverID:     instance.DriverID,
		DriverName:   d.Name(),
		Name:         instance.Name,
		State:        instance.State,
		StateMessage: instance.StateMessage,
		DriverPhase:  instance.DriverPhase,
	}
	if d.docker == nil {
		status.Warnings = append(status.Warnings, registry.StatusWarning{
			Code:    "runtime_unavailable",
			Message: "Docker runtime status is unavailable",
		})
		return status, nil
	}
	ps, err := d.docker.ComposePs(ctx, instance.DataDir)
	if err != nil {
		d.logger.Debug("stardew compose ps unavailable", "instance", instance.ID, "error", err)
		status.Warnings = append(status.Warnings, registry.StatusWarning{
			Code:    "runtime_unavailable",
			Message: "Docker runtime status is unavailable",
		})
		return status, nil
	}
	containers := make([]registry.ContainerSummary, 0, len(ps.Services))
	for _, svc := range ps.Services {
		containers = append(containers, registry.ContainerSummary{
			Name:    svc.Name,
			Service: svc.Service,
			State:   svc.State,
			Status:  svc.Status,
			Health:  svc.Health,
		})
	}
	status.Runtime = &registry.RuntimeStatus{Containers: containers}
	return status, nil
}

// ── Unimplemented methods ─────────────────────────────────────────────────────

func (d *Driver) Logs(ctx context.Context, instance registry.Instance) (<-chan registry.LogLine, error) {
	return nil, registry.ErrNotImplemented
}
func (d *Driver) ExecCommand(ctx context.Context, cmd string) (*registry.CommandResult, error) {
	return nil, registry.ErrNotImplemented
}
func (d *Driver) UploadSave(ctx context.Context, file registry.UploadedFile) error {
	return registry.ErrNotImplemented
}
func (d *Driver) SelectSave(ctx context.Context, name string) error {
	return registry.ErrNotImplemented
}
func (d *Driver) DeleteSave(ctx context.Context, name string) error {
	return registry.ErrNotImplemented
}
func (d *Driver) ListMods(ctx context.Context, instance registry.Instance) ([]registry.ModInfo, error) {
	return nil, registry.ErrNotImplemented
}
func (d *Driver) UploadMod(ctx context.Context, file registry.UploadedFile) error {
	return registry.ErrNotImplemented
}
func (d *Driver) DeleteMod(ctx context.Context, id string) error {
	return registry.ErrNotImplemented
}

// ── Internal helpers ──────────────────────────────────────────────────────────

func (d *Driver) setGuardChan(jobID string, ch chan string) {
	d.mu.Lock()
	d.guardChans[jobID] = ch
	d.mu.Unlock()
}

func (d *Driver) clearGuardChan(jobID string) {
	d.mu.Lock()
	delete(d.guardChans, jobID)
	d.mu.Unlock()
}

// InstallOptions returns the image tag options available in the install UI.
func (d *Driver) InstallOptions() []registry.ImageTagOption {
	return []registry.ImageTagOption{
		{
			Tag:         TestedImageTag,
			Label:       TestedImageTag + " (已验证版本)",
			Recommended: true,
		},
	}
}

func requiresInstalledFiles(state string) bool {
	switch state {
	case storage.InstanceStateGameInstalled,
		storage.InstanceStateSaveRequired,
		storage.InstanceStateReadyToStart,
		storage.InstanceStateStarting,
		storage.InstanceStateRunning,
		storage.InstanceStateStopped:
		return true
	default:
		return false
	}
}

func gameInstallImage(dataDir string) string {
	envVals, _ := sjconfig.ReadEnvFile(filepath.Join(dataDir, ".env"))
	return envWithDefault(envVals, "SERVER_IMAGE", serverImageDefault(TestedImageTag))
}

// verifyGameDataVolume checks every runtime artifact installed by this panel.
// SteamCMD `validate` remains the full-depot integrity check; this prevents a
// stale success log or later state transition from accepting an empty volume.
func (d *Driver) verifyGameDataVolume(ctx context.Context, dataDir, imageRef string, lineHandler func(string)) (bool, error) {
	gameDataVolume := resolvedGameDataVolumeName(dataDir)
	script := `set -u
echo "anxi-install-verify"
missing=""
require_file() { if [ ! -f "$1" ]; then missing="${missing} $2"; fi; }
require_exec() { if [ ! -x "$1" ]; then missing="${missing} $2"; fi; }
require_exec /data/game/StardewValley StardewValley
require_file "/data/game/Stardew Valley.dll" StardewValleyDLL
require_file /data/game/steamapps/appmanifest_413150.acf StardewManifest
require_exec /data/game/StardewModdingAPI StardewModdingAPI
require_file /data/game/StardewModdingAPI.dll StardewModdingAPIDLL
require_file /data/game/Mods/ConsoleCommands/manifest.json SMAPIConsoleCommands
require_file /data/game/Mods/SaveBackup/manifest.json SMAPISaveBackup
require_file /data/game/.steam-sdk/steamapps/appmanifest_1007.acf SteamSDKManifest
if ! find /data/game/.steam-sdk -type f -name steamclient.so -print -quit | grep -q .; then
  missing="${missing} SteamClientLibrary"
fi
if [ -n "${missing}" ]; then
  echo "INSTALL_REQUIRED_FILES_MISSING:${missing}" >&2
  exit 42
fi
echo "INSTALL_REQUIRED_FILES_OK"
`
	exitCode, err := d.docker.RunContainerTTY(ctx, paneldocker.ContainerTTYRunOpts{
		ImageRef:   imageRef,
		Entrypoint: []string{"/bin/sh"},
		User:       "root",
		Command:    []string{"-lc", script},
		Binds:      []string{gameDataVolume + ":/data/game"},
	}, nil, func(line string) {
		if lineHandler != nil {
			lineHandler(line)
		}
	})
	if err != nil {
		return false, fmt.Errorf("run game-data verifier: %w", err)
	}
	if exitCode == 0 {
		return true, nil
	}
	if exitCode == installVerificationMissingExitCode {
		return false, nil
	}
	return false, fmt.Errorf("game-data verifier exited with code %d", exitCode)
}

// updatePhase attempts a best-effort instance state update; errors are only logged.
// Preserves the existing DriverPayload to avoid wiping stored metadata.
func (d *Driver) updatePhase(ctx context.Context, instanceID, state, message, phase, jobID string) {
	if d.store == nil {
		return
	}
	existing := "{}"
	if inst, err := d.store.GetInstance(ctx, instanceID); err == nil {
		if inst.DriverPayload != "" {
			existing = inst.DriverPayload
		}
	}
	if jobID == "" {
		_, err := d.store.UpdateInstanceState(ctx, storage.UpdateInstanceStateParams{
			ID: instanceID, State: state, StateMessage: message, DriverPhase: phase, DriverPayload: existing,
		})
		if err != nil {
			d.logger.Warn("failed to update instance state", "instance", instanceID, "state", state, "error", err)
		}
		return
	}
	ownedStore, supportsOwnedUpdates := d.store.(activeJobStateStore)
	if !supportsOwnedUpdates {
		_, err := d.store.UpdateInstanceState(ctx, storage.UpdateInstanceStateParams{
			ID: instanceID, State: state, StateMessage: message, DriverPhase: phase, DriverPayload: existing,
		})
		if err != nil {
			d.logger.Warn("failed to update instance state", "instance", instanceID, "state", state, "error", err)
		}
		return
	}
	_, err := ownedStore.UpdateInstanceStateForActiveJob(ctx, storage.UpdateInstanceStateForActiveJobParams{
		JobID: jobID,
		UpdateInstanceStateParams: storage.UpdateInstanceStateParams{
			ID:            instanceID,
			State:         state,
			StateMessage:  message,
			DriverPhase:   phase,
			DriverPayload: existing,
		},
	})
	if err != nil {
		if errors.Is(err, storage.ErrJobNotActive) {
			d.logger.Debug("ignored stale job instance state update", "instance", instanceID, "job_id", jobID, "state", state, "phase", phase)
			return
		}
		d.logger.Warn("failed to update instance state", "instance", instanceID, "state", state, "error", err)
	}
}
