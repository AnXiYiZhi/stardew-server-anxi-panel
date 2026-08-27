package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/netdns"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

// dockerHubHTTPClient talks to Docker Hub's public API. It uses the
// DNS-fallback transport so a flaky host resolver doesn't break version checks
// (the same resolver failure that broke Nexus also broke these — see netdns).
var dockerHubHTTPClient = netdns.NewClient(10 * time.Second)

// imageTagPattern allows alphanumeric, dots, hyphens, and underscores.
// This matches standard Docker tag conventions (no colons, no slashes).
var imageTagPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// handleInstancePrepare handles POST /api/instances/:id/prepare.
// Creates the working directory, docker-compose.yml, and .env template.
// Requires admin.
func (s *server) handleInstancePrepare(w http.ResponseWriter, r *http.Request, instanceID string) {
	actor, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}
	driver, ok := s.loadDriver(w, instance.DriverID)
	if !ok {
		return
	}

	if err := driver.Prepare(r.Context(), makeRegistryInstance(instance)); err != nil {
		s.logger.Error("prepare failed", "instance", instanceID, "error", err)
		writeError(w, http.StatusInternalServerError, "prepare_failed", sanitizeErrorMsg(err, "准备实例目录失败"))
		return
	}

	// Advance state to junimo_scaffolded only from initial states.
	if instance.State == storage.InstanceStateAdminCreated ||
		instance.State == storage.InstanceStateUninitialized {
		updated, err := s.store.UpdateInstanceState(r.Context(), storage.UpdateInstanceStateParams{
			ID:           instanceID,
			State:        storage.InstanceStateJunimoScaffolded,
			StateMessage: "Junimo 配置已准备，等待 Steam 凭据。",
			DriverPhase:  "junimo_scaffolded",
		})
		if err != nil {
			s.logger.Warn("could not advance state to junimo_scaffolded", "instance", instanceID, "error", err)
		} else {
			instance = updated
		}
	}

	s.auditLog(r, &actor, "instance_prepare", "instance", instanceID, "{}")
	writeJSON(w, http.StatusOK, s.makeInstanceStateResponse(r.Context(), instance))
}

// installRequestBody is the JSON body for POST …/install.
type installRequestBody struct {
	SteamUsername    string `json:"steamUsername"`
	SteamPassword    string `json:"steamPassword"`
	VNCPassword      string `json:"vncPassword"`
	ImageTag         string `json:"imageTag"`
	ReuseCredentials bool   `json:"reuseCredentials"` // if true, read creds from existing .env
}

// handleInstanceInstall handles POST /api/instances/:id/install.
// Requires admin.  Creates a long-running install job and returns its ID.
func (s *server) handleInstanceInstall(w http.ResponseWriter, r *http.Request, instanceID string) {
	actor, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	var body installRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "请求体解析失败")
		return
	}

	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}

	// reuseCredentials: reload creds from .env so the user doesn't have to re-enter them.
	if body.ReuseCredentials {
		envVals, err := sjconfig.ReadEnvFile(filepath.Join(instance.DataDir, ".env"))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "env_read_failed", sanitizeErrorMsg(err, "读取实例配置失败"))
			return
		}
		body.SteamUsername = envVals["STEAM_USERNAME"]
		body.SteamPassword = envVals["STEAM_PASSWORD"]
		body.VNCPassword = envVals["VNC_PASSWORD"]
	}

	if body.SteamUsername == "" {
		writeError(w, http.StatusBadRequest, "missing_field", "steamUsername 不能为空")
		return
	}
	if body.SteamPassword == "" {
		writeError(w, http.StatusBadRequest, "missing_field", "steamPassword 不能为空")
		return
	}
	if body.VNCPassword == "" {
		writeError(w, http.StatusBadRequest, "missing_field", "vncPassword 不能为空")
		return
	}
	if body.ImageTag == "" {
		body.ImageTag = stardew_junimo.TestedImageTag
	}
	if !imageTagPattern.MatchString(body.ImageTag) {
		writeError(w, http.StatusBadRequest, "invalid_field", "imageTag 格式不合法")
		return
	}

	driver, ok := s.loadDriver(w, instance.DriverID)
	if !ok {
		return
	}

	// Auto-prepare before install (idempotent: skips files that already exist).
	if err := driver.Prepare(r.Context(), makeRegistryInstance(instance)); err != nil {
		if writeStardewMutationGuardConflict(w, err) {
			return
		}
		s.logger.Error("auto-prepare failed", "instance", instanceID, "error", err)
		writeError(w, http.StatusInternalServerError, "prepare_failed", sanitizeErrorMsg(err, "准备实例目录失败"))
		return
	}
	if instance.State == storage.InstanceStateAdminCreated ||
		instance.State == storage.InstanceStateUninitialized {
		updated, err := s.store.UpdateInstanceState(r.Context(), storage.UpdateInstanceStateParams{
			ID:           instanceID,
			State:        storage.InstanceStateJunimoScaffolded,
			StateMessage: "Junimo 配置已准备，等待 Steam 凭据。",
			DriverPhase:  "junimo_scaffolded",
		})
		if err != nil {
			s.logger.Warn("could not advance state after prepare", "instance", instanceID, "error", err)
		} else {
			instance = updated
		}
	}

	job, err := driver.Install(r.Context(), registry.InstallRequest{
		Instance:      makeRegistryInstance(instance),
		ActorID:       actor.User.ID,
		SteamUsername: body.SteamUsername,
		SteamPassword: body.SteamPassword,
		VNCPassword:   body.VNCPassword,
		ImageTag:      body.ImageTag,
		AutoDownload:  body.ReuseCredentials,
	})
	if err != nil {
		if writeStardewMutationGuardConflict(w, err) {
			return
		}
		if activeJobID, handled := writeActiveInstallConflict(w, err, "该实例已有安装任务正在进行。"); handled {
			s.logger.Info("install request attached to active job", "instance", instanceID, "job_id", activeJobID)
			return
		}
		s.logger.Error("install failed to start", "instance", instanceID, "error", err)
		writeError(w, http.StatusInternalServerError, "install_failed", sanitizeErrorMsg(err, "安装任务启动失败"))
		return
	}

	// Log that install was requested (never log credentials).
	s.logger.Info("install job started", "instance", instanceID, "job_id", job.ID, "actor", actor.User.ID)
	s.auditLog(r, &actor, "instance_install", "instance", instanceID, auditMetadata("jobId", job.ID))

	writeJSON(w, http.StatusAccepted, map[string]string{"jobId": job.ID})
}

type steamCredentialsRequestBody struct {
	SteamUsername string `json:"steamUsername"`
	SteamPassword string `json:"steamPassword"`
}

type steamCredentialsResponse struct {
	OK                   bool   `json:"ok"`
	InstanceID           string `json:"instanceId"`
	State                string `json:"state"`
	DriverPhase          string `json:"driverPhase"`
	SteamInviteEnabled   bool   `json:"steamInviteEnabled"`
	SteamInviteAuthState string `json:"steamInviteAuthState"`
	SteamAuthLoggedIn    bool   `json:"steamAuthLoggedIn"`
}

// handleInstanceSteamCredentials handles PUT /api/instances/:id/steam-credentials.
// It only replaces the shared Steam account/password in .env. Existing SteamCMD
// authorization caches, SteamAuth sessions, invite intent, game data, and all
// other settings are deliberately left untouched.
func (s *server) handleInstanceSteamCredentials(w http.ResponseWriter, r *http.Request, instanceID string) {
	actor, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	var body steamCredentialsRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "请求体解析失败")
		return
	}
	body.SteamUsername = strings.TrimSpace(body.SteamUsername)
	if body.SteamUsername == "" {
		writeError(w, http.StatusBadRequest, "missing_field", "steamUsername 不能为空")
		return
	}
	if body.SteamPassword == "" {
		writeError(w, http.StatusBadRequest, "missing_field", "steamPassword 不能为空")
		return
	}

	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}
	driver, ok := s.loadDriver(w, instance.DriverID)
	if !ok {
		return
	}
	if !steamCredentialsInstallationReady(r.Context(), driver, instance) {
		writeError(w, http.StatusConflict, "installation_required", "请先完成 Stardew Valley 基础安装，再修改 Steam 账号密码。")
		return
	}
	updater, ok := driver.(interface {
		UpdateSteamCredentials(context.Context, registry.Instance, string, string) error
	})
	if !ok {
		writeError(w, http.StatusInternalServerError, "steam_credentials_update_unsupported", "当前游戏驱动不支持修改 Steam 凭据。")
		return
	}
	if err := updater.UpdateSteamCredentials(r.Context(), makeRegistryInstance(instance), body.SteamUsername, body.SteamPassword); err != nil {
		if errors.Is(err, stardew_junimo.ErrSteamCredentialsInstallationRequired) {
			writeError(w, http.StatusConflict, "installation_required", "请先完成 Stardew Valley 基础安装，再修改 Steam 账号密码。")
			return
		}
		if errors.Is(err, stardew_junimo.ErrSteamCredentialsInUse) {
			writeError(w, http.StatusConflict, "steam_credentials_in_use", "安装或 Steam 授权任务正在使用当前凭据，请等待任务结束后再修改。")
			return
		}
		if writeStardewMutationGuardConflict(w, err) {
			return
		}
		s.logger.Error("update shared Steam credentials failed", "instance", instanceID, "error", err)
		writeError(w, http.StatusInternalServerError, "steam_credentials_update_failed", sanitizeErrorMsg(err, "保存 Steam 账号密码失败"))
		return
	}

	s.auditLog(r, &actor, "instance_steam_credentials_update", "instance", instanceID, "{}")
	writeJSON(w, http.StatusOK, steamCredentialsResponse{
		OK:                   true,
		InstanceID:           instance.ID,
		State:                instance.State,
		DriverPhase:          instance.DriverPhase,
		SteamInviteEnabled:   sjconfig.SteamInviteEnabled(instance.DataDir),
		SteamInviteAuthState: sjconfig.SteamInviteAuthState(instance.DataDir),
		SteamAuthLoggedIn:    sjconfig.SteamAuthLoggedIn(instance.DataDir),
	})
}

func steamCredentialsInstallationReady(ctx context.Context, driver registry.GameDriver, instance storage.Instance) bool {
	if !steamInviteAuthorizationInstalledState(instance.State) {
		return false
	}
	provider, ok := driver.(interface {
		InstallationDiagnostic(context.Context, registry.Instance) stardew_junimo.InstallationDiagnostic
	})
	if !ok {
		return false
	}
	diagnostic := provider.InstallationDiagnostic(ctx, makeRegistryInstance(instance))
	return diagnostic.RequiredFiles == "ok"
}

// handleInstanceSteamAuthLogin handles POST /api/instances/:id/steam-auth/login.
// It records explicit invite-code opt-in, pulls steam-auth only on demand, and
// runs login authorization without downloading the game or touching SteamCMD
// caches. The game server must be stopped first. Steam Guard prompts reuse the
// existing POST …/steam-guard/input flow.
func (s *server) handleInstanceSteamAuthLogin(w http.ResponseWriter, r *http.Request, instanceID string) {
	actor, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}
	if !steamInviteAuthorizationInstalledState(instance.State) {
		writeError(w, http.StatusConflict, "installation_required", "请先完成 Stardew Valley 基础安装，再启用 Steam 邀请码。")
		return
	}
	if sjconfig.SteamInviteEnabled(instance.DataDir) &&
		sjconfig.SteamInviteAuthState(instance.DataDir) == sjconfig.SteamInviteAuthStateReady &&
		sjconfig.SteamAuthLoggedIn(instance.DataDir) {
		writeError(w, http.StatusConflict, "steam_invite_already_ready", "Steam 邀请码授权仍然有效，无需重复登录。")
		return
	}
	instance, ok = s.ensureInstanceNotRunning(w, r, instance)
	if !ok {
		return
	}
	envVals, err := sjconfig.ReadEnvFile(filepath.Join(instance.DataDir, ".env"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "env_read_failed", sanitizeErrorMsg(err, "读取实例配置失败"))
		return
	}
	if envVals["STEAM_USERNAME"] == "" || envVals["STEAM_PASSWORD"] == "" {
		writeError(w, http.StatusBadRequest, "credentials_missing", "未找到已保存的 Steam 账号密码，请先在安装页面完成一次凭据填写。")
		return
	}
	driver, ok := s.loadDriver(w, instance.DriverID)
	if !ok {
		return
	}
	job, err := driver.Install(r.Context(), registry.InstallRequest{
		Instance:      makeRegistryInstance(instance),
		ActorID:       actor.User.ID,
		SteamUsername: envVals["STEAM_USERNAME"],
		SteamPassword: envVals["STEAM_PASSWORD"],
		VNCPassword:   envVals["VNC_PASSWORD"],
		ImageTag:      stardew_junimo.TestedImageTag,
		AuthLoginOnly: true,
		ForceReauth:   true,
	})
	if err != nil {
		if errors.Is(err, stardew_junimo.ErrSteamInviteAlreadyAuthorized) {
			writeError(w, http.StatusConflict, "steam_invite_already_ready", "Steam 邀请码授权仍然有效，无需重复登录。")
			return
		}
		if errors.Is(err, stardew_junimo.ErrSteamInviteCleanupPending) {
			writeError(w, http.StatusConflict, "steam_invite_cleanup_pending", "上次授权已成功，但一次性授权容器尚未安全收敛；请检查 Docker 状态后重试。")
			return
		}
		if writeStardewMutationGuardConflict(w, err) {
			return
		}
		if activeJobID, handled := writeActiveInstallConflict(w, err, "该实例已有安装或 Steam 授权任务正在进行。"); handled {
			s.logger.Info("steam-auth login attached to active install job", "instance", instanceID, "job_id", activeJobID)
			return
		}
		s.logger.Error("steam-auth login failed to start", "instance", instanceID, "error", err)
		writeError(w, http.StatusInternalServerError, "auth_login_failed", sanitizeErrorMsg(err, "登录授权任务启动失败"))
		return
	}
	s.logger.Info("steam-auth login job started", "instance", instanceID, "job_id", job.ID, "actor", actor.User.ID)
	s.auditLog(r, &actor, "instance_steam_auth_login", "instance", instanceID, auditMetadata("jobId", job.ID))
	writeJSON(w, http.StatusAccepted, map[string]any{"jobId": job.ID, "steamInviteEnabled": true})
}

func steamInviteAuthorizationInstalledState(state string) bool {
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

func writeActiveInstallConflict(w http.ResponseWriter, err error, message string) (string, bool) {
	var activeJob *storage.ActiveJobExistsError
	if !errors.As(err, &activeJob) {
		return "", false
	}
	writeErrorWithDetails(w, http.StatusConflict, "install_in_progress", message, map[string]string{"jobId": activeJob.Job.ID})
	return activeJob.Job.ID, true
}

// steamGuardInputBody is the JSON body for POST …/steam-guard/input.
type steamGuardInputBody struct {
	JobID string `json:"jobId"`
	Input string `json:"input"`
}

// handleInstanceSteamGuardInput handles POST /api/instances/:id/steam-guard/input.
// Requires admin.
func (s *server) handleInstanceSteamGuardInput(w http.ResponseWriter, r *http.Request, instanceID string) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var body steamGuardInputBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "请求体解析失败")
		return
	}
	if body.JobID == "" {
		writeError(w, http.StatusBadRequest, "missing_field", "jobId 不能为空")
		return
	}
	if body.Input == "" {
		writeError(w, http.StatusBadRequest, "missing_field", "input 不能为空")
		return
	}

	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}
	job, err := s.store.GetJob(r.Context(), body.JobID)
	if err != nil {
		writeError(w, http.StatusConflict, "guard_input_failed", sanitizeError(err, "认证任务不存在或已结束"))
		return
	}
	if job.TargetType != "instance" || job.TargetID != instanceID ||
		(job.Type != "stardew_install" && job.Type != "stardew_steam_auth") {
		writeError(w, http.StatusConflict, "guard_input_failed", "认证任务不属于当前实例")
		return
	}
	if job.Status != storage.JobStatusQueued && job.Status != storage.JobStatusRunning {
		writeError(w, http.StatusConflict, "guard_input_failed", "认证任务已结束，不能再提交输入")
		return
	}
	s.logger.Info("steam guard input received", "instance", instanceID, "job_id", body.JobID, "phase", instance.DriverPhase)
	if (instance.DriverPhase == "auth_method_required" ||
		instance.DriverPhase == "steam_guard_choice_required" ||
		instance.DriverPhase == "steamcmd_guard_choice_required") &&
		body.Input != "1" && body.Input != "2" {
		writeError(w, http.StatusBadRequest, "invalid_field", "认证方式只能选择 1 或 2")
		return
	}
	driver, ok := s.loadDriver(w, instance.DriverID)
	if !ok {
		return
	}

	sender, supported := driver.(registry.SteamGuardSender)
	if !supported {
		writeError(w, http.StatusBadRequest, "not_supported", "该 driver 不支持 Steam Guard 输入")
		return
	}

	// The container's PTY runs in canonical mode: the PTY line discipline buffers
	// input until '\n', so ALL input (menu selection and guard code) needs a
	// trailing newline to be delivered to the application's Console.ReadLine().
	input := body.Input + "\n"
	if err := sender.SendSteamGuardInput(body.JobID, input); err != nil {
		writeError(w, http.StatusConflict, "guard_input_failed", sanitizeError(err, "Steam Guard 输入失败"))
		return
	}

	nextPhase := ""
	nextMessage := ""
	switch {
	case instance.DriverPhase == "auth_method_required" && body.Input == "1":
		nextPhase = "steam_auth_running"
		nextMessage = "已选择账号密码登录，正在等待 Steam 响应。"
	case instance.DriverPhase == "auth_method_required" && body.Input == "2":
		nextPhase = "steam_qr_required"
		nextMessage = "已选择二维码登录，请使用 Steam 手机 App 扫码。"
	case instance.DriverPhase == "steam_guard_choice_required" && body.Input == "1":
		nextPhase = "steam_guard_mobile_required"
		nextMessage = "请打开 Steam 手机 App，批准此次登录请求。"
	case instance.DriverPhase == "steam_guard_choice_required" && body.Input == "2":
		// The container's next prompt ("Enter Steam Guard code: ") has no trailing
		// newline and is never seen by our line-by-line scanner. Proactively advance
		// the phase so the frontend shows the code input box without waiting for it.
		nextPhase = "steam_guard_required"
		nextMessage = "Steam Guard 验证码已请求，请在面板输入验证码。"
	case instance.DriverPhase == "steamcmd_guard_choice_required" && body.Input == "1":
		nextPhase = "steamcmd_guard_mobile_required"
		nextMessage = "正在等待 SteamCMD 手机 App 批准。"
	case instance.DriverPhase == "steamcmd_guard_choice_required" && body.Input == "2":
		nextPhase = "steamcmd_guard_required"
		nextMessage = "SteamCMD 需要输入 App 或邮箱验证码。"
	}
	if nextPhase != "" {
		nextState := storage.InstanceStateSteamAuthRunning
		if job.Type == "stardew_steam_auth" {
			// Invite authorization is optional. Its method/Guard phase must never
			// replace the durable base-install lifecycle state.
			nextState = instance.State
		}
		if _, err := s.store.UpdateInstanceStateForActiveJob(r.Context(), storage.UpdateInstanceStateForActiveJobParams{
			JobID: body.JobID,
			UpdateInstanceStateParams: storage.UpdateInstanceStateParams{
				ID:            instanceID,
				State:         nextState,
				StateMessage:  nextMessage,
				DriverPhase:   nextPhase,
				DriverPayload: instance.DriverPayload,
			},
		}); err != nil {
			if errors.Is(err, storage.ErrJobNotActive) {
				s.logger.Info("skipped late steam auth phase update for terminal job", "instance", instanceID, "job_id", body.JobID, "phase", nextPhase)
			} else {
				s.logger.Warn("proactive steam auth phase update failed", "instance", instanceID, "job_id", body.JobID, "phase", nextPhase, "error", err)
			}
		}
	}

	// Log that input was submitted but do NOT log the actual code or method value.
	s.logger.Info("steam auth input submitted", "instance", instanceID, "job_id", body.JobID)

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// installOptionsResponse is the response body for GET …/install-options.
type installOptionsResponse struct {
	ImageTagOptions []registry.ImageTagOption `json:"imageTagOptions"`
}

// handleInstanceInstallOptions handles GET /api/instances/:id/install-options.
// Returns selectable image tag options for the install UI.  Requires admin.
func (s *server) handleInstanceInstallOptions(w http.ResponseWriter, r *http.Request, instanceID string) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}
	driver, ok := s.loadDriver(w, instance.DriverID)
	if !ok {
		return
	}
	_ = instance

	type optionsProvider interface {
		InstallOptions() []registry.ImageTagOption
	}
	provider, supported := driver.(optionsProvider)
	if !supported {
		writeJSON(w, http.StatusOK, installOptionsResponse{
			ImageTagOptions: []registry.ImageTagOption{
				{Tag: "latest", Label: "latest", Recommended: true},
			},
		})
		return
	}

	options := provider.InstallOptions()

	writeJSON(w, http.StatusOK, installOptionsResponse{ImageTagOptions: options})
}

// checkTestedTagIsLatest returns true when the "latest" tag on Docker Hub points
// to the same image digest as testedTag.  Network failure is logged and returns false.
func checkTestedTagIsLatest(ctx context.Context, logger *slog.Logger, repo, testedTag string) bool {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	latestDigest := dockerHubTagDigest(ctx, logger, repo, "latest")
	if latestDigest == "" {
		logger.Warn("docker hub version check: could not fetch 'latest' digest", "repo", repo)
		return false
	}
	testedDigest := dockerHubTagDigest(ctx, logger, repo, testedTag)
	if testedDigest == "" {
		logger.Warn("docker hub version check: could not fetch tested tag digest", "repo", repo, "tag", testedTag)
		return false
	}
	match := latestDigest == testedDigest
	logger.Info("docker hub version check", "repo", repo, "tested_tag", testedTag, "is_latest", match,
		"latest_digest_prefix", latestDigest[:min(16, len(latestDigest))],
		"tested_digest_prefix", testedDigest[:min(16, len(testedDigest))])
	return match
}

// dockerHubTagDigest returns the manifest digest for the given Docker Hub tag.
// Returns empty string on any error.
func dockerHubTagDigest(ctx context.Context, logger *slog.Logger, repo, tag string) string {
	url := fmt.Sprintf("https://hub.docker.com/v2/repositories/%s/tags/%s/", repo, tag)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ""
	}
	resp, err := dockerHubHTTPClient.Do(req)
	if err != nil {
		logger.Warn("docker hub tag fetch failed", "repo", repo, "tag", tag, "error", err)
		return ""
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		logger.Warn("docker hub tag fetch: unexpected status", "repo", repo, "tag", tag, "status", resp.StatusCode)
		return ""
	}
	var payload struct {
		Digest string `json:"digest"`
		Images []struct {
			Digest string `json:"digest"`
		} `json:"images"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		logger.Warn("docker hub tag fetch: decode failed", "repo", repo, "tag", tag, "error", err)
		return ""
	}
	if payload.Digest != "" {
		return payload.Digest
	}
	if len(payload.Images) > 0 && payload.Images[0].Digest != "" {
		return payload.Images[0].Digest
	}
	logger.Warn("docker hub tag fetch: no digest in response", "repo", repo, "tag", tag)
	return ""
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
