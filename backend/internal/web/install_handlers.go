package web

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"regexp"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

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
	writeJSON(w, http.StatusOK, makeInstanceStateResponse(instance))
}

// installRequestBody is the JSON body for POST …/install.
type installRequestBody struct {
	SteamUsername    string `json:"steamUsername"`
	SteamPassword    string `json:"steamPassword"`
	VNCPassword      string `json:"vncPassword"`
	ImageTag         string `json:"imageTag"`
	ReuseCredentials bool   `json:"reuseCredentials"` // if true, read creds from existing .env
	ForceReauth      bool   `json:"forceReauth"`      // if true, clear saved auth caches and re-run full auth
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

	// forceReauth always requires freshly entered credentials (account/password change).
	if body.ForceReauth {
		body.ReuseCredentials = false
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
		ForceReauth:   body.ForceReauth,
	})
	if err != nil {
		s.logger.Error("install failed to start", "instance", instanceID, "error", err)
		writeError(w, http.StatusInternalServerError, "install_failed", sanitizeErrorMsg(err, "安装任务启动失败"))
		return
	}

	// Log that install was requested (never log credentials).
	s.logger.Info("install job started", "instance", instanceID, "job_id", job.ID, "actor", actor.User.ID)
	s.auditLog(r, &actor, "instance_install", "instance", instanceID, auditMetadata("jobId", job.ID))

	writeJSON(w, http.StatusAccepted, map[string]string{"jobId": job.ID})
}

// handleInstanceSteamAuthLogin handles POST /api/instances/:id/steam-auth/login.
// It re-runs ONLY steam-auth using the saved account/password (no re-entry) to obtain
// a fresh STEAM_REFRESH_TOKEN — required for the server to generate Steam/Galaxy
// invite codes. The game server must be stopped first: steam-auth and the server
// share the compose project and would conflict. Steam Guard prompts, if any, reuse
// the existing POST …/steam-guard/input flow.
func (s *server) handleInstanceSteamAuthLogin(w http.ResponseWriter, r *http.Request, instanceID string) {
	actor, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}
	if instance.State == storage.InstanceStateRunning || instance.State == storage.InstanceStateStarting {
		writeError(w, http.StatusConflict, "server_running", "请先停止服务器，再登录 Steam 授权（steam-auth 与游戏服务器不能同时运行）。")
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
	})
	if err != nil {
		s.logger.Error("steam-auth login failed to start", "instance", instanceID, "error", err)
		writeError(w, http.StatusInternalServerError, "auth_login_failed", sanitizeErrorMsg(err, "登录授权任务启动失败"))
		return
	}
	s.logger.Info("steam-auth login job started", "instance", instanceID, "job_id", job.ID, "actor", actor.User.ID)
	s.auditLog(r, &actor, "instance_steam_auth_login", "instance", instanceID, auditMetadata("jobId", job.ID))
	writeJSON(w, http.StatusAccepted, map[string]string{"jobId": job.ID})
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
		nextMessage = "steam-auth 国内网络下载失败，正在等待 SteamCMD 手机 App 批准。"
	case instance.DriverPhase == "steamcmd_guard_choice_required" && body.Input == "2":
		nextPhase = "steamcmd_guard_required"
		nextMessage = "steam-auth 国内网络下载失败，SteamCMD 需要输入 App 或邮箱验证码。"
	}
	if nextPhase != "" {
		if _, err := s.store.UpdateInstanceState(r.Context(), storage.UpdateInstanceStateParams{
			ID:           instanceID,
			State:        storage.InstanceStateSteamAuthRunning,
			StateMessage: nextMessage,
			DriverPhase:  nextPhase,
		}); err != nil {
			s.logger.Warn("proactive steam auth phase update failed", "instance", instanceID, "phase", nextPhase, "error", err)
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

	// Check whether the tested tag is still the latest released version.
	// Uses a short timeout; failure is logged and IsLatest stays false.
	isLatest := checkTestedTagIsLatest(r.Context(), s.logger, "sdvd/steam-service", stardew_junimo.TestedImageTag)
	for i := range options {
		if options[i].Recommended {
			options[i].IsLatest = isLatest
		}
	}

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
	resp, err := http.DefaultClient.Do(req)
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
