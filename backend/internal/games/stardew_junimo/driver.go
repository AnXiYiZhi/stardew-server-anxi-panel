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

// Driver implements registry.GameDriver for Stardew Valley / JunimoServer.
type Driver struct {
	docker       DockerService
	logger       *slog.Logger
	jobs         *jobs.Manager
	store        StateStore
	panelVersion string

	// guardChans maps running install job ID → channel for Steam Guard input.
	mu         sync.Mutex
	guardChans map[string]chan string

	runtimeUpdateMu            sync.Mutex
	runtimeUpdatePollInterval  time.Duration
	runtimeUpdateAuthTimeout   time.Duration
	runtimeUpdateServerTimeout time.Duration
}

// New creates a Driver.  jobs and store may be nil for tests that only use
// the driver skeleton (Prepare, Status).
func New(docker DockerService, logger *slog.Logger, jobManager *jobs.Manager, store StateStore, panelVersions ...string) *Driver {
	if logger == nil {
		logger = slog.Default()
	}
	panelVersion := "dev"
	if len(panelVersions) > 0 && strings.TrimSpace(panelVersions[0]) != "" {
		panelVersion = strings.TrimSpace(panelVersions[0])
	}
	return &Driver{
		docker:                     docker,
		logger:                     logger,
		jobs:                       jobManager,
		store:                      store,
		panelVersion:               panelVersion,
		guardChans:                 make(map[string]chan string),
		runtimeUpdatePollInterval:  2 * time.Second,
		runtimeUpdateAuthTimeout:   90 * time.Second,
		runtimeUpdateServerTimeout: 5 * time.Minute,
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
	if instance.DataDir == "" {
		return errors.New("instance data dir is empty")
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

	if err := installSMAPIMod(instance.DataDir); err != nil {
		d.logger.Warn("SMAPI mod install failed (non-fatal)", "instance", instance.ID, "error", err)
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

	return nil
}

// Install validates credentials, creates an async install job, and returns its ID.
func (d *Driver) Install(ctx context.Context, req registry.InstallRequest) (*registry.Job, error) {
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
				if instance.State != storage.InstanceStateRunning {
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
		return instance, nil
	}
	return d.store.UpdateInstanceState(ctx, storage.UpdateInstanceStateParams{
		ID:           instance.ID,
		State:        storage.InstanceStateError,
		StateMessage: "游戏运行文件不完整，请重新安装或修复。",
		DriverPhase:  "install_verification_failed",
	})
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
	_, err := d.store.UpdateInstanceState(ctx, storage.UpdateInstanceStateParams{
		ID:            instanceID,
		State:         state,
		StateMessage:  message,
		DriverPhase:   phase,
		DriverPayload: existing,
	})
	if err != nil {
		d.logger.Warn("failed to update instance state", "instance", instanceID, "state", state, "error", err)
	}
}
