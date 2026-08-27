package stardew_junimo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

// Pull progress patterns.
// Docker Compose V2 non-TTY output has two observed formats:
//
//	Modern: "Image sdvd/server:tag Pulling" / "Image sdvd/server:tag Pulled"
//	Older:  "Pulling steam-auth (sdvd/...)" / "Status: Downloaded newer image for ..."
//
// regexPullCount covers the "[+] Pulling X/Y" summary line present in some versions.
var (
	regexPullCount      = regexp.MustCompile(`(?i)pulling (\d+)/(\d+)`)
	regexPullStart      = regexp.MustCompile(`(?i)(^image \S+\s+pulling\s*$|^pulling \S+ \()`)
	regexPullDone       = regexp.MustCompile(`(?i)(^image \S+\s+pulled\s*$|^status: (downloaded newer image|image is up to date))`)
	regexImagePullLayer = regexp.MustCompile(`(?i)^([0-9a-f]+):\s+(pulling fs layer|waiting|downloading|extracting|download complete|pull complete|already exists)\s*$`)
)

// installRunner carries everything needed to execute one install job.
//
// Three orthogonal routing decisions replace the old single steamCMDRetry flag:
//   - reuse:          reuse saved credentials without re-prompting for input.
//   - steamCMDUseCache: SteamCMD logs in with the cached authorization
//     (username only) instead of a full username+password login. Derived from
//     the persisted STEAMCMD_AUTH_COMPLETED flag in run().
//   - forceReauth:    AuthLoginOnly resets only its failed/pending session; base
//     installs reject this internal flag and save shared credentials separately.
type installRunner struct {
	driver           *Driver
	instance         storage.Instance
	username         string
	password         string // never logged
	vncPass          string // never logged
	imageTag         string
	reuse            bool
	steamCMDUseCache bool
	forceReauth      bool
	authOnly         bool // run steam-auth for login only; stop after auth succeeds (no download/fallback/SMAPI)
}

type steamAuthMode string

const (
	steamAuthModeCredentials steamAuthMode = "credentials"
	steamAuthModeQR          steamAuthMode = "qr"
)

// run is the job.Runner function executed by jobs.Manager in a goroutine.
func (r *installRunner) run(ctx context.Context, jobCtx *jobs.Context) error {
	if !r.authOnly {
		return r.runPrepared(ctx, jobCtx)
	}
	if err := sjconfig.SetSteamInviteEnabled(r.instance.DataDir, true); err != nil {
		return fmt.Errorf("enable Steam invite capability: %w", err)
	}
	// Keep any previously successful completion bit until ForceReauth has safely
	// classified and removed the exact Auth session. Preparation, image pull, or
	// an unknown volume holder must not invalidate a session that still exists.
	if err := sjconfig.SetSteamInviteAuthState(r.instance.DataDir, sjconfig.SteamInviteAuthStateAuthorizing); err != nil {
		return fmt.Errorf("record Steam invite authorization state: %w", err)
	}
	authorizationSucceeded := false
	defer func() {
		var err error
		if sjconfig.SteamInviteAuthState(r.instance.DataDir) == sjconfig.SteamInviteAuthStateCleanupPending {
			// A successful session whose one-shot holder was not safely classified
			// remains gated until Prepare/start recovery converges the holder only.
		} else if authorizationSucceeded || sjconfig.SteamAuthLoggedIn(r.instance.DataDir) {
			err = sjconfig.SetSteamInviteAuthState(r.instance.DataDir, sjconfig.SteamInviteAuthStateReady)
		} else {
			err = sjconfig.SetSteamAuthLoggedIn(r.instance.DataDir, false)
		}
		if err != nil {
			_, _ = jobCtx.Warn(context.Background(), "记录 Steam 邀请码授权状态失败。")
		}
		if _, err := r.driver.store.RestoreInstanceStateSnapshot(context.Background(), r.instance); err != nil {
			_, _ = jobCtx.Warn(context.Background(), "恢复基础安装状态失败，请刷新实例状态后重试。")
		}
	}()
	if err := r.runInviteAuthorizationPrepared(ctx, jobCtx); err != nil {
		return err
	}
	authorizationSucceeded = true
	return nil
}

func (r *installRunner) runInviteAuthorizationPrepared(ctx context.Context, jobCtx *jobs.Context) error {
	if err := verifyRequiredInstanceFiles(r.instance.DataDir); err != nil {
		return fmt.Errorf("verify installed instance before Steam invite authorization: %w", err)
	}
	composePath := filepath.Join(r.instance.DataDir, "docker-compose.yml")
	if changed, err := migrateSteamAuthComposeImage(composePath); err != nil {
		return fmt.Errorf("update optional steam-auth image in docker-compose.yml: %w", err)
	} else if changed {
		_, _ = jobCtx.Info(ctx, "已更新 docker-compose.yml 中的可选 steam-auth 镜像配置。")
	}
	envPath := filepath.Join(r.instance.DataDir, ".env")
	envVals, err := sjconfig.ReadEnvFile(envPath)
	if err != nil {
		return fmt.Errorf("read Steam invite authorization environment: %w", err)
	}
	if r.forceReauth {
		if err := r.clearSteamAuthSessionVolumes(ctx, jobCtx); err != nil {
			return err
		}
	}

	updates := map[string]string{
		"STEAM_USERNAME": r.username,
		"STEAM_PASSWORD": r.password,
	}
	if r.forceReauth {
		updates["STEAM_REFRESH_TOKEN"] = ""
		updates["STEAM_AUTH_COMPLETED"] = ""
	}
	ensureEnvDefault(updates, envVals, "STEAM_SERVICE_IMAGE", DefaultSteamServiceImage)
	ensureEnvDefault(updates, envVals, "STEAM_SERVICE_IMAGE_CANDIDATES", DefaultSteamServiceImageCandidates)
	if normalized := strings.Join(steamServiceImageRefs(envVals), ","); normalized != "" && normalized != strings.TrimSpace(envVals["STEAM_SERVICE_IMAGE_CANDIDATES"]) {
		updates["STEAM_SERVICE_IMAGE_CANDIDATES"] = normalized
	}
	ensureEnvDefault(updates, envVals, "STEAM_CLIENT_CONNECT_TIMEOUT_SECONDS", DefaultSteamClientConnectTimeoutSeconds)
	ensureEnvDefault(updates, envVals, "STEAM_CLIENT_CONNECT_RETRIES", DefaultSteamClientConnectRetries)
	ensureEnvDefault(updates, envVals, "STEAM_AUTH_SESSION_RETRIES", DefaultSteamAuthSessionRetries)
	ensureEnvDefault(updates, envVals, "STEAM_AUTH_SESSION_RETRY_DELAY_SECONDS", DefaultSteamAuthSessionRetryDelaySeconds)
	if err := sjconfig.UpdateEnvFile(envPath, updates); err != nil {
		return fmt.Errorf("write Steam invite authorization environment: %w", err)
	}
	_, _ = jobCtx.Info(ctx, "Steam 邀请码授权使用已保存的共享 Steam 账号密码；不会运行基础安装迁移或触碰游戏文件。")
	return r.runInviteAuthorization(ctx, jobCtx)
}

func (r *installRunner) runPrepared(ctx context.Context, jobCtx *jobs.Context) error {
	_, _ = jobCtx.Info(ctx, "正在检查 Junimo 工作目录...")
	_, _ = jobCtx.Info(ctx, fmt.Sprintf("实例目录：%s", r.instance.DataDir))
	_, _ = jobCtx.Info(ctx, fmt.Sprintf("Compose 文件：%s", filepath.Join(r.instance.DataDir, "docker-compose.yml")))
	composeWasMissing := fileMissing(filepath.Join(r.instance.DataDir, "docker-compose.yml"))
	envWasMissing := fileMissing(filepath.Join(r.instance.DataDir, ".env"))
	if err := r.driver.Prepare(ctx, registry.Instance{
		ID:            r.instance.ID,
		DriverID:      r.instance.DriverID,
		Name:          r.instance.Name,
		DataDir:       r.instance.DataDir,
		State:         r.instance.State,
		StateMessage:  r.instance.StateMessage.String,
		DriverPhase:   r.instance.DriverPhase,
		DriverPayload: r.instance.DriverPayload,
		CreatedAt:     r.instance.CreatedAt,
		UpdatedAt:     r.instance.UpdatedAt,
	}); err != nil {
		return fmt.Errorf("prepare Junimo workdir: %w", err)
	}
	if err := verifyRequiredInstanceFiles(r.instance.DataDir); err != nil {
		return err
	}
	if composeWasMissing {
		_, _ = jobCtx.Info(ctx, "docker-compose.yml 缺失，已重新生成。")
	}
	if envWasMissing {
		_, _ = jobCtx.Info(ctx, ".env 缺失，已重新生成。")
	}

	// ── Step 1: write .env ──────────────────────────────────────────────
	_, _ = jobCtx.Info(ctx, "正在写入 .env 凭据...")
	envPath := r.instance.DataDir + "/.env"
	envVals, _ := sjconfig.ReadEnvFile(envPath)
	maintainSteamAuth := sjconfig.SteamInviteEnabled(r.instance.DataDir)
	// SteamCMD persists its machine authorization in the mounted Steam config
	// volumes. After one successful full login, use the username-only form so
	// SteamCMD consumes that cached authorization instead of starting a fresh
	// password/Steam Guard login on every repair. runSteamCMDFallback falls back to
	// a full login automatically if Steam reports that the cache has expired or is
	// missing, so a stale flag cannot leave the install flow stuck. Saving a new
	// fallback username/password never invalidates a still-usable machine cache.
	r.steamCMDUseCache = strings.EqualFold(strings.TrimSpace(envVals["STEAMCMD_AUTH_COMPLETED"]), "true")
	updates := map[string]string{
		"IMAGE_VERSION":  r.imageTag,
		"STEAM_USERNAME": r.username,
		"STEAM_PASSWORD": r.password,
		"VNC_PASSWORD":   r.vncPass,
	}
	ensureEnvDefault(updates, envVals, "SERVER_IMAGE", serverImageDefault(r.imageTag))
	ensureEnvDefault(updates, envVals, "SERVER_IMAGE_CANDIDATES", serverImageCandidatesDefault(r.imageTag))
	if normalized := strings.Join(serverImageRefs(envVals, r.imageTag), ","); normalized != "" && normalized != strings.TrimSpace(envVals["SERVER_IMAGE_CANDIDATES"]) {
		updates["SERVER_IMAGE_CANDIDATES"] = normalized
	}
	if maintainSteamAuth {
		ensureEnvDefault(updates, envVals, "STEAM_SERVICE_IMAGE", DefaultSteamServiceImage)
		ensureEnvDefault(updates, envVals, "STEAM_SERVICE_IMAGE_CANDIDATES", DefaultSteamServiceImageCandidates)
		if normalized := strings.Join(steamServiceImageRefs(envVals), ","); normalized != "" && normalized != strings.TrimSpace(envVals["STEAM_SERVICE_IMAGE_CANDIDATES"]) {
			updates["STEAM_SERVICE_IMAGE_CANDIDATES"] = normalized
		}
		ensureEnvDefault(updates, envVals, "STEAM_CLIENT_CONNECT_TIMEOUT_SECONDS", DefaultSteamClientConnectTimeoutSeconds)
		ensureEnvDefault(updates, envVals, "STEAM_CLIENT_CONNECT_RETRIES", DefaultSteamClientConnectRetries)
		ensureEnvDefault(updates, envVals, "STEAM_AUTH_SESSION_RETRIES", DefaultSteamAuthSessionRetries)
		ensureEnvDefault(updates, envVals, "STEAM_AUTH_SESSION_RETRY_DELAY_SECONDS", DefaultSteamAuthSessionRetryDelaySeconds)
	}
	ensureEnvDefault(updates, envVals, "STEAMCMD_IMAGE", DefaultSteamCMDImage)
	ensureEnvDefault(updates, envVals, "STEAMCMD_IMAGE_CANDIDATES", DefaultSteamCMDImageCandidates)
	ensureEnvDefault(updates, envVals, "SMAPI_VERSION", DefaultSMAPIVersion)
	ensureEnvDefault(updates, envVals, "SMAPI_DOWNLOAD_URLS", DefaultSMAPIDownloadURLs)
	if normalized := steamCMDImageCandidatesValue(envVals["STEAMCMD_IMAGE_CANDIDATES"]); normalized != "" && normalized != strings.TrimSpace(envVals["STEAMCMD_IMAGE_CANDIDATES"]) {
		updates["STEAMCMD_IMAGE_CANDIDATES"] = normalized
	}
	if err := sjconfig.UpdateEnvFile(envPath, updates); err != nil {
		return fmt.Errorf("write .env: %w", err)
	}
	_, _ = jobCtx.Info(ctx, ".env 写入完成（密码已脱敏，不写入日志）。")
	if changed, err := migrateAllowInsecureSetup(envPath); err != nil {
		return fmt.Errorf("update ALLOW_INSECURE_SETUP in .env: %w", err)
	} else if changed {
		_, _ = jobCtx.Info(ctx, "已将 .env 中的 ALLOW_INSECURE_SETUP 设为 true（Junimo 需要此配置才能在无 API_KEY 时启动）。")
	}
	if changed, err := migrateOptionalSteamAuthComposeImage(filepath.Join(r.instance.DataDir, "docker-compose.yml"), maintainSteamAuth); err != nil {
		return fmt.Errorf("update steam-auth image in docker-compose.yml: %w", err)
	} else if changed {
		_, _ = jobCtx.Info(ctx, "已更新 docker-compose.yml 中的 steam-auth 镜像配置。")
	}
	if changed, err := migrateServerComposeImage(filepath.Join(r.instance.DataDir, "docker-compose.yml")); err != nil {
		return fmt.Errorf("update server image in docker-compose.yml: %w", err)
	} else if changed {
		_, _ = jobCtx.Info(ctx, "已更新 docker-compose.yml 中的 server 镜像配置。")
	}
	if changed, err := migrateSavesVolume(filepath.Join(r.instance.DataDir, "docker-compose.yml")); err != nil {
		return fmt.Errorf("migrate saves volume in docker-compose.yml: %w", err)
	} else if changed {
		_, _ = jobCtx.Info(ctx, "已将 docker-compose.yml 中的 saves 命名卷迁移为 bind mount。")
	}
	if changed, err := migrateRemoveAssetExporterService(filepath.Join(r.instance.DataDir, "docker-compose.yml")); err != nil {
		return fmt.Errorf("remove legacy asset exporter from docker-compose.yml: %w", err)
	} else if changed {
		_, _ = jobCtx.Info(ctx, "已移除旧版运行时素材导出服务；前端将使用随镜像发布的素材。")
	}
	if changed, err := migrateModsCompose(filepath.Join(r.instance.DataDir, "docker-compose.yml")); err != nil {
		return fmt.Errorf("migrate mods compose mount: %w", err)
	} else if changed {
		_, _ = jobCtx.Info(ctx, "已添加 mods 目录挂载到 docker-compose.yml。")
	}
	if changed, err := ensureRuntimeContEnvFix(r.instance.DataDir, maintainSteamAuth); err != nil {
		return fmt.Errorf("ensure server static init compatibility fix: %w", err)
	} else if changed {
		_, _ = jobCtx.Info(ctx, "JunimoServer static init compatibility mounts have been applied.")
	}
	if changed, err := EnsureServerPlayerAuthEnvironment(r.instance.DataDir); err != nil {
		return fmt.Errorf("ensure server player authentication environment: %w", err)
	} else if changed {
		_, _ = jobCtx.Info(ctx, "已为旧实例补齐玩家加入保护运行环境。")
	}
	// ── Step 2: Junimo server image ─────────────────────────────────────
	r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateSteamAuthRunning,
		"正在检查 JunimoServer 镜像，请稍候...", "server_image_checking", jobCtx.ID)
	if _, err := r.ensureServerImage(ctx, jobCtx); err != nil {
		return err
	}

	// ── Step 3: SteamCMD primary download ───────────────────────────────
	_, _ = jobCtx.Info(ctx, "基础安装使用 SteamCMD 下载并校验 Stardew Valley 与 Steamworks SDK；不会拉取或启动 steam-auth。")
	guardCh := make(chan string, 8)
	r.driver.setGuardChan(jobCtx.ID, guardCh)
	defer r.driver.clearGuardChan(jobCtx.ID)
	if err := r.runSteamCMDFallback(ctx, jobCtx, guardCh); err != nil {
		return err
	}
	return r.completeInstall(ctx, jobCtx)
}

func (r *installRunner) ensureSteamAuthImage(ctx context.Context, jobCtx *jobs.Context) (string, error) {
	envPath := filepath.Join(r.instance.DataDir, ".env")
	envVals, _ := sjconfig.ReadEnvFile(envPath)

	steamImage, err := r.ensureCandidateImage(ctx, jobCtx, imageCandidatePullOptions{
		Service:       "steam-auth",
		Label:         "steam-auth-cn",
		EnvKey:        "STEAM_SERVICE_IMAGE",
		Refs:          steamServiceImageRefs(envVals),
		PullLogPrefix: "[steam-auth:pull] ",
	})
	if err != nil {
		return "", err
	}
	if err := sjconfig.UpdateEnvFile(envPath, map[string]string{
		"STEAM_SERVICE_IMAGE":            steamImage,
		"STEAM_SERVICE_IMAGE_CANDIDATES": strings.Join(steamServiceImageRefs(envVals), ","),
	}); err != nil {
		return "", fmt.Errorf("write selected steam-auth image to .env: %w", err)
	}
	return steamImage, nil
}

func (r *installRunner) ensureServerImage(ctx context.Context, jobCtx *jobs.Context) (string, error) {
	envPath := filepath.Join(r.instance.DataDir, ".env")
	envVals, _ := sjconfig.ReadEnvFile(envPath)
	serverImage, err := r.ensureCandidateImage(ctx, jobCtx, imageCandidatePullOptions{
		Service:       "server",
		Label:         "JunimoServer",
		EnvKey:        "SERVER_IMAGE",
		Refs:          serverImageRefs(envVals, r.imageTag),
		PullLogPrefix: "[server:pull] ",
	})
	if err != nil {
		return "", err
	}

	updates := map[string]string{
		"SERVER_IMAGE":            serverImage,
		"SERVER_IMAGE_CANDIDATES": strings.Join(serverImageRefs(envVals, r.imageTag), ","),
	}
	if err := sjconfig.UpdateEnvFile(envPath, updates); err != nil {
		return "", fmt.Errorf("write selected JunimoServer image to .env: %w", err)
	}
	return serverImage, nil
}

func (r *installRunner) runInviteAuthorization(ctx context.Context, jobCtx *jobs.Context) error {
	_, _ = jobCtx.Info(ctx, "正在按需检查 steam-auth 镜像；该镜像仅用于 Steam 邀请码授权，不下载游戏本体。")
	imageRef, err := r.ensureSteamAuthImage(ctx, jobCtx)
	if err != nil {
		return err
	}
	_, _ = jobCtx.Info(ctx, fmt.Sprintf("[steam-auth] 授权镜像 %s 已就绪；将启动一次性登录容器，保存 session 后立即停止。", imageRef))
	if err := r.runSteamAuth(ctx, jobCtx); err != nil {
		return err
	}
	if !sjconfig.SteamAuthLoggedIn(r.instance.DataDir) {
		return fmt.Errorf("steam-auth authorization finished without a durable login confirmation")
	}
	return nil
}

type imageCandidatePullOptions struct {
	Service       string
	Label         string
	EnvKey        string
	Refs          []string
	PullLogPrefix string
}

// updatePhase keeps optional Steam invite authorization from replacing the
// instance's durable base-install state. Auth progress still uses driverPhase,
// stateMessage, and the dedicated STEAM_INVITE_AUTH_STATE flag; state remains
// game_installed/ready_to_start/stopped (or whatever snapshot preceded auth).
func (r *installRunner) updatePhase(ctx context.Context, state, message, phase, jobID string) {
	if r.authOnly && strings.TrimSpace(r.instance.State) != "" {
		state = r.instance.State
	}
	r.driver.updatePhase(ctx, r.instance.ID, state, message, phase, jobID)
}

func (r *installRunner) ensureCandidateImage(ctx context.Context, jobCtx *jobs.Context, opts imageCandidatePullOptions) (string, error) {
	failureState := storage.InstanceStateJunimoScaffolded
	failurePhase := "pull_failed"
	timeoutState := storage.InstanceStateError
	timeoutPhase := "install_timeout"
	timeoutMessage := "安装任务超时，镜像拉取未完成，请重试安装。"
	if r.authOnly {
		failureState = r.instance.State
		failurePhase = "steam_invite_auth_failed"
		timeoutState = r.instance.State
		timeoutPhase = "steam_invite_auth_failed"
		timeoutMessage = "Steam 邀请码授权超时，Auth 镜像拉取未完成；基础安装不受影响。"
	}
	if len(opts.Refs) == 0 {
		r.updatePhase(context.Background(), failureState,
			fmt.Sprintf("%s 镜像未配置，请检查实例 .env。", opts.Label), failurePhase, jobCtx.ID)
		return "", fmt.Errorf("%s image candidates are empty", opts.Service)
	}

	for i, imageRef := range opts.Refs {
		if _, err := r.driver.docker.ImageInspect(ctx, r.instance.DataDir, imageRef); err == nil {
			if i == 0 {
				_, _ = jobCtx.Info(context.Background(), fmt.Sprintf("[%s] 本地已有镜像 %s，直接使用。", opts.Service, imageRef))
			} else {
				_, _ = jobCtx.Info(context.Background(), fmt.Sprintf("[%s] 本地已有备用镜像 %s，直接使用。", opts.Service, imageRef))
			}
			return imageRef, nil
		}
	}

	var failures []string
	for i, imageRef := range opts.Refs {
		if _, err := r.driver.docker.ImageInspect(ctx, r.instance.DataDir, imageRef); err == nil {
			return imageRef, nil
		}

		r.updatePhase(context.Background(), storage.InstanceStateSteamAuthRunning,
			fmt.Sprintf("正在拉取 %s 镜像（%d/%d），请稍候...", opts.Label, i+1, len(opts.Refs)), "pull_running", jobCtx.ID)
		_, _ = jobCtx.Info(context.Background(), fmt.Sprintf("[%s] 本地缺少镜像 %s，正在拉取（%d/%d）。", opts.Service, imageRef, i+1, len(opts.Refs)))

		if _, pullErr := r.driver.docker.PullImageStreaming(ctx, r.instance.DataDir, imageRef,
			makeImagePullLineHandler(jobCtx, opts.PullLogPrefix, func(done, total int) {
				percent := 0
				if total > 0 {
					percent = int(float64(done) * 100 / float64(total))
				}
				message := fmt.Sprintf("正在拉取 %s 镜像（约 %d%%，%d/%d 层，候选 %d/%d），首次下载可能需要 10-30 分钟...",
					opts.Label, percent, done, total, i+1, len(opts.Refs))
				if done == total && total > 0 {
					message = fmt.Sprintf("%s 镜像拉取完成，正在检查下一个组件...", opts.Label)
				}
				r.updatePhase(context.Background(),
					storage.InstanceStateSteamAuthRunning, message, "pull_running", jobCtx.ID)
			})); pullErr != nil {
			if ctx.Err() != nil {
				r.updatePhase(context.Background(), timeoutState,
					timeoutMessage, timeoutPhase, jobCtx.ID)
				return "", fmt.Errorf("%s image pull timed out: %w", opts.Service, ctx.Err())
			}
			safeErr := paneldocker.RedactString(pullErr.Error())
			failures = append(failures, fmt.Sprintf("%s: %s", imageRef, safeErr))
			if i+1 < len(opts.Refs) {
				_, _ = jobCtx.Warn(context.Background(), fmt.Sprintf("%s 镜像 %s 拉取失败，正在尝试备用镜像。", opts.Label, imageRef))
				continue
			}
			r.updatePhase(context.Background(), failureState,
				fmt.Sprintf("%s 镜像全部拉取失败，请检查 Docker 镜像源或实例 .env 中的 %s。", opts.Label, opts.EnvKey), failurePhase, jobCtx.ID)
			_, _ = jobCtx.Error(context.Background(), fmt.Sprintf("%s 镜像拉取失败：%s", opts.Label, strings.Join(failures, "；")))
			return "", fmt.Errorf("pull %s image candidates: %s", opts.Service, strings.Join(failures, "; "))
		}
		return imageRef, nil
	}

	return "", fmt.Errorf("%s image candidates are empty", opts.Service)
}

func (r *installRunner) runSteamAuth(ctx context.Context, jobCtx *jobs.Context) error {
	guardCh := make(chan string, 8)
	r.driver.setGuardChan(jobCtx.ID, guardCh)
	defer func() {
		r.driver.clearGuardChan(jobCtx.ID)
	}()

	mode := steamAuthModeCredentials
	if r.reuse {
		message := "复用已保存的 Steam 凭据，直接校验并下载游戏文件。"
		phaseMessage := "正在复用已保存的 Steam 凭据校验已有文件并继续下载..."
		phase := "game_downloading"
		if r.authOnly {
			message = "复用已保存的 Steam 凭据，仅执行 Steam 邀请码登录授权。"
			phaseMessage = "正在登录 Steam 邀请码授权；不会下载游戏本体。"
			phase = "steam_invite_authorizing"
		}
		_, _ = jobCtx.Info(ctx, message)
		r.updatePhase(ctx, storage.InstanceStateSteamAuthRunning,
			phaseMessage, phase, jobCtx.ID)
	} else {
		selectedMode, err := r.waitSteamAuthMode(ctx, jobCtx, guardCh)
		if err != nil {
			return err
		}
		mode = selectedMode
	}

	const maxAttempts = 5
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			delay := time.Duration(attempt*3) * time.Second
			_, _ = jobCtx.Warn(context.Background(), fmt.Sprintf("SteamClient 尚未连接，等待 %s 后重试 Steam 认证（%d/%d）...", delay, attempt, maxAttempts))
			r.updatePhase(context.Background(), storage.InstanceStateSteamAuthRunning,
				"Steam 连接建立较慢，正在自动重试认证...", "steam_auth_retrying", jobCtx.ID)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				message := "安装任务超时，Steam 认证未完成，请重试安装。"
				phase := "install_timeout"
				if r.authOnly {
					message = "Steam 邀请码授权连接重试超时，请重试授权。"
					phase = "steam_invite_auth_failed"
				}
				r.updatePhase(context.Background(), storage.InstanceStateError, message, phase, jobCtx.ID)
				return fmt.Errorf("steam-auth timed out: %w", ctx.Err())
			}
		}

		retry, err := r.runSteamAuthAttempt(ctx, jobCtx, guardCh, mode, attempt, maxAttempts)
		if retry && attempt < maxAttempts {
			continue
		}
		return err
	}

	return fmt.Errorf("steam authentication failed after retries")
}

func (r *installRunner) waitSteamAuthMode(ctx context.Context, jobCtx *jobs.Context, guardCh <-chan string) (steamAuthMode, error) {
	r.updatePhase(ctx, storage.InstanceStateSteamAuthRunning,
		"请选择 Steam 登录方式。", "auth_method_required", jobCtx.ID)
	_, _ = jobCtx.Info(ctx, "等待管理员选择 Steam 登录方式：扫码登录，或账号密码/验证码登录。")

	for {
		select {
		case input := <-guardCh:
			switch strings.TrimSpace(input) {
			case "1":
				_, _ = jobCtx.Info(ctx, "已选择账号密码/验证码登录。")
				message := "正在使用已保存的 Steam 凭据认证并下载游戏，请稍候..."
				phase := "steam_auth_running"
				if r.authOnly {
					message = "正在使用已保存的 Steam 凭据完成邀请码授权；不会下载游戏本体。"
					phase = "steam_invite_authorizing"
				}
				r.updatePhase(ctx, storage.InstanceStateSteamAuthRunning, message, phase, jobCtx.ID)
				return steamAuthModeCredentials, nil
			case "2":
				_, _ = jobCtx.Info(ctx, "已选择扫码登录。")
				r.updatePhase(ctx, storage.InstanceStateSteamAuthRunning,
					"正在启动 Steam 扫码登录，请等待二维码输出。", "steam_qr_required", jobCtx.ID)
				return steamAuthModeQR, nil
			default:
				_, _ = jobCtx.Warn(ctx, "收到无效的 Steam 登录方式选择，仍在等待 1 或 2。")
			}
		case <-ctx.Done():
			message := "安装任务超时，Steam 认证未完成，请重试安装。"
			phase := "install_timeout"
			if r.authOnly {
				message = "邀请码授权超时，登录方式尚未确认，请重试授权。"
				phase = "steam_invite_authorization_timeout"
			}
			r.updatePhase(context.Background(), storage.InstanceStateError, message, phase, jobCtx.ID)
			return "", fmt.Errorf("steam auth method selection timed out: %w", ctx.Err())
		}
	}
}

func (r *installRunner) runSteamAuthAttempt(ctx context.Context, jobCtx *jobs.Context, guardCh chan string, mode steamAuthMode, attempt, maxAttempts int) (bool, error) {
	runCtx := ctx
	stopRun := context.CancelFunc(func() {})
	if r.authOnly {
		// Credential authorization uses the long-running, no-download service
		// command and must be stopped after a durable result. QR authorization uses
		// the one-shot login command, but still needs a cancellation path when the
		// authenticated account does not match the saved shared Steam account.
		runCtx, stopRun = context.WithCancel(ctx)
		defer stopRun()
	}
	cancelAuthOnSuccess := r.authOnly && mode == steamAuthModeCredentials
	cancelAuthOnFailure := r.authOnly

	var (
		outputMu         sync.Mutex
		authSucceeded    bool
		authFailed       bool
		accountMismatch  bool
		connectionFailed bool
		credentialFailed bool
		qrAuthFailed     bool
		mobileApproval   bool
		downloadFailed   bool
		guardChoiceShown bool
		currentApp       string
		sdkDownloaded    bool
	)

	lineHandler := func(line string) {
		outputMu.Lock()
		defer outputMu.Unlock()

		// Steam Guard and auth prompts are informational (not sensitive).
		_, _ = jobCtx.Info(context.Background(), "[steam] "+line)

		lower := strings.ToLower(line)
		if mode == steamAuthModeQR {
			if account, ok := steamAuthAuthenticatedAccount(line); ok {
				if !strings.EqualFold(strings.TrimSpace(account), strings.TrimSpace(r.username)) {
					accountMismatch = true
					authFailed = true
					r.updatePhase(ctx, storage.InstanceStateSteamAuthFailed,
						"扫码登录的 Steam 账号与已保存的共享账号不一致，请使用同一账号重试。",
						"steam_invite_auth_failed", jobCtx.ID)
					stopRun()
					return
				}
			}
		}

		switch {
		case containsAny(lower, "tryanothercm", "steamclient instance must be connected", "asyncjobfailedexception"):
			// TryAnotherCM and AsyncJobFailedException are Steam network/CM errors,
			// not credential failures. Both are retried automatically.
			connectionFailed = true
			authFailed = true
			if cancelAuthOnFailure {
				stopRun()
			}

		case strings.Contains(lower, "qr authentication failed"):
			qrAuthFailed = true
			authFailed = true
			if cancelAuthOnFailure {
				stopRun()
			}

		case containsAny(lower, "invalid password", "incorrect password", "wrong password", "bad password"):
			credentialFailed = true
			authFailed = true
			if cancelAuthOnFailure {
				stopRun()
			}

		case containsAny(lower, "unhandled exception", "cannot read keys"):
			authFailed = true
			if cancelAuthOnFailure {
				stopRun()
			}

		case isSteamAuthMethodMenu(lower):
			if mode == steamAuthModeQR {
				r.updatePhase(ctx, storage.InstanceStateSteamAuthRunning,
					"请使用 Steam 手机 App 扫描日志中显示的二维码。", "steam_qr_required", jobCtx.ID)
			} else {
				r.updatePhase(ctx, storage.InstanceStateSteamAuthRunning,
					"请选择 Steam 登录方式。", "auth_method_required", jobCtx.ID)
			}

		case isSteamGuardChoiceMenu(lower):
			guardChoiceShown = true
			r.updatePhase(ctx, storage.InstanceStateSteamAuthRunning,
				"Steam 需要二步验证，请在面板选择 Steam Guard 方式。",
				"steam_guard_choice_required", jobCtx.ID)

		case mode != steamAuthModeQR && isSteamMobileApprovalPrompt(lower):
			// After option 1 is selected (by user or auto), Steam waits for mobile approval.
			mobileApproval = true
			r.updatePhase(ctx, storage.InstanceStateSteamAuthRunning,
				"请打开 Steam 手机 App，批准此次登录请求。",
				"steam_guard_mobile_required", jobCtx.ID)

		case isSteamGuardCodePrompt(lower):
			r.updatePhase(ctx, storage.InstanceStateSteamAuthRunning,
				"Steam Guard 验证码已请求，请在面板输入验证码。",
				"steam_guard_required", jobCtx.ID)

		case mode != steamAuthModeQR && containsAny(lower, "steam guard", "two-factor", "authenticator"):
			// Don't overwrite steam_guard_choice_required while the user is choosing.
			// The explicit "waiting for approval" / mobile-app text case handles the
			// transition once approval is actually requested.
			if !mobileApproval && !guardChoiceShown {
				r.updatePhase(ctx, storage.InstanceStateSteamAuthRunning,
					"请打开 Steam 手机 App，批准此次登录请求；如果你选择输入验证码，面板会再显示验证码输入框。",
					"steam_guard_mobile_required", jobCtx.ID)
			}

		case containsAny(lower, "qr code", "scan the qr", "scan this qr") && !strings.Contains(lower, "[2]"):
			r.updatePhase(ctx, storage.InstanceStateSteamAuthRunning,
				"请使用 Steam 手机 App 扫描日志中显示的二维码。",
				"steam_qr_required", jobCtx.ID)

		case containsAny(lower, "approve this", "confirm") && strings.Contains(lower, "steam"):
			mobileApproval = true
			r.updatePhase(ctx, storage.InstanceStateSteamAuthRunning,
				"请在 Steam 手机 App 上批准此次登录请求。",
				"steam_guard_mobile_required", jobCtx.ID)

		case containsAny(lower, "download complete", "app installed to"):
			if currentApp == "sdk" {
				sdkDownloaded = true
			}

		case strings.Contains(lower, "downloading app 1007") ||
			(strings.Contains(lower, "target directory:") && strings.Contains(lower, ".steam-sdk")):
			// Steamworks SDK Redistributable is downloaded after the Stardew depot.
			// Keep it as a separate phase so the frontend does not appear stuck at
			// 100% game download while SDK files are still being fetched.
			currentApp = "sdk"
			r.updatePhase(ctx, storage.InstanceStateSteamAuthRunning,
				"游戏文件下载完成，正在下载 Steam SDK 运行文件...",
				"steam_sdk_downloading", jobCtx.ID)

		case containsAny(lower, "downloading app"):
			// Auth succeeded; game depot download is starting. Update phase so the
			// frontend shows progress instead of the stale Steam Guard phase.
			currentApp = "game"
			r.updatePhase(ctx, storage.InstanceStateSteamAuthRunning,
				"Steam 认证成功，正在下载游戏文件，请耐心等待...",
				"game_downloading", jobCtx.ID)

		case containsAny(lower, "game download failed", "download failed:"):
			// Auth may have succeeded but the depot download failed (CDN error, low disk
			// space, etc.). Flag it so we report failure instead of false success.
			downloadFailed = true

		case isSteamAuthLoginSuccessLine(lower):
			authSucceeded = true
			if cancelAuthOnSuccess {
				stopRun()
			}

		case containsAny(lower, "login failed", "authentication failed", "no accounts logged in", "skipping game download"):
			authFailed = true
			if cancelAuthOnFailure {
				stopRun()
			}
		}
	}

	if mode == steamAuthModeQR {
		// The interactive login command prints its own first menu. Pre-buffer the choice that
		// the Panel already made. Credential authorization uses the no-download
		// service command and consumes STEAM_USERNAME/STEAM_PASSWORD directly.
		choice := "2\n"
		select {
		case guardCh <- choice:
		default:
			_, _ = jobCtx.Warn(ctx, "Steam 输入队列已满，无法自动选择扫码登录。")
		}
	}

	// RunSteamAuthTTY creates the container via Docker API with Tty:true so
	// Console.ReadKey() works for the Steam Guard method selection menu.
	exitCode, cmdErr := r.driver.docker.RunSteamAuthTTY(runCtx, r.instance.DataDir, r.buildSteamAuthOpts(mode), guardCh, lineHandler)
	if r.authOnly && ctx.Err() == nil && cmdErr == context.Canceled && (authSucceeded || authFailed) {
		// A plain context.Canceled is the platform runner's proof that the exact
		// one-shot container stopped and cleanup succeeded. Joined/wrapped cleanup
		// errors are intentionally not normalized.
		cmdErr = nil
	}

	// Persist steam-auth/Galaxy authorization only when the steam-auth login log
	// itself confirms success. SteamCMD fallback and later depot-download success
	// must not set this flag; it controls whether invite-code auth is considered
	// done for the main UI.
	if authSucceeded && !r.authOnly {
		_ = r.markSteamAuthCompleted(jobCtx, sjconfig.SteamInviteAuthStateReady)
	}

	if r.authOnly {
		var semanticErr error
		if downloadFailed || sdkDownloaded || currentApp != "" {
			r.updatePhase(context.Background(), storage.InstanceStateSteamAuthFailed,
				"Steam 邀请码授权流程返回了意外的游戏下载状态；未修改基础安装。", "steam_invite_auth_failed", jobCtx.ID)
			semanticErr = errors.New("Steam invite authorization entered an unexpected download state")
		}
		if accountMismatch {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
			cleanupErr := r.clearSteamAuthSessionVolumes(cleanupCtx, jobCtx)
			cleanupCancel()
			r.updatePhase(context.Background(), storage.InstanceStateSteamAuthFailed,
				"扫码登录账号与已保存的共享 Steam 账号不一致，授权 session 已拒绝。", "steam_invite_auth_failed", jobCtx.ID)
			if cmdErr != nil {
				_, _ = jobCtx.Error(context.Background(), "拒绝错误账号 session 时，一次性授权容器还报告了运行或清理错误："+paneldocker.RedactString(cmdErr.Error()))
			}
			return false, errors.Join(errors.New("Steam QR account mismatch"), semanticErr, cmdErr, cleanupErr)
		}
		if semanticErr != nil {
			return false, errors.Join(semanticErr, cmdErr)
		}
		if authSucceeded {
			completedState := sjconfig.SteamInviteAuthStateReady
			if cmdErr != nil {
				completedState = sjconfig.SteamInviteAuthStateCleanupPending
			}
			if err := r.markSteamAuthCompleted(jobCtx, completedState); err != nil {
				r.updatePhase(context.Background(), storage.InstanceStateSteamAuthFailed,
					"Steam 登录已完成，但持久化邀请码授权状态失败，请重试授权。", "steam_invite_auth_failed", jobCtx.ID)
				return false, fmt.Errorf("persist Steam invite authorization: %w", err)
			}
			// This operation only refreshes Steam/Galaxy authorization. It must not
			// change an incomplete-download error into game_installed.
			r.updatePhase(context.Background(), r.instance.State,
				r.instance.StateMessage.String, r.instance.DriverPhase, jobCtx.ID)
			if cmdErr != nil {
				_, _ = jobCtx.Warn(context.Background(), "Steam 登录 session 已保存，但一次性授权容器未能确认安全停止；不会删除已成功的 session，请检查 Docker 状态。")
				return false, fmt.Errorf("Steam invite authorization succeeded but exact container cleanup failed: %w", cmdErr)
			}
			_, _ = jobCtx.Info(context.Background(), "一次性 SteamAuth 授权容器已安全停止；登录 session 已保存，游戏安装状态保持不变。")
			return false, nil
		}
		if cmdErr != nil {
			r.updatePhase(context.Background(), storage.InstanceStateSteamAuthFailed,
				"Steam 邀请码授权容器未能安全停止，请检查 Docker 状态和任务日志后重试授权。", "steam_invite_auth_failed", jobCtx.ID)
			_, _ = jobCtx.Error(context.Background(), "Steam 邀请码授权容器运行或清理失败："+paneldocker.RedactString(cmdErr.Error()))
			return false, fmt.Errorf("Steam invite authorization container failed: %w", cmdErr)
		}
	}

	if cmdErr != nil {
		if sdkDownloaded {
			if err := r.completeInstall(ctx, jobCtx); err != nil {
				return false, err
			}
			return false, nil
		}
		if authSucceeded {
			r.updatePhase(context.Background(), storage.InstanceStateError,
				"Steam 认证已成功，但后续安装步骤失败；请检查任务日志后使用已保存凭据重试。", "post_auth_failed", jobCtx.ID)
			_, _ = jobCtx.Error(context.Background(), "Steam 认证已成功，但后续安装步骤失败："+paneldocker.RedactString(cmdErr.Error()))
			return false, fmt.Errorf("post-auth install error: %w", cmdErr)
		}
		if ctx.Err() != nil {
			r.updatePhase(context.Background(), storage.InstanceStateError,
				"安装任务超时，Steam 认证未完成，请重试安装。", "install_timeout", jobCtx.ID)
			return false, fmt.Errorf("steam-auth timed out: %w", ctx.Err())
		}
		r.updatePhase(context.Background(), storage.InstanceStateSteamAuthFailed,
			"Steam 认证容器运行失败，请检查 Docker 状态和任务日志后重试安装。", "steam_auth_failed", jobCtx.ID)
		_, _ = jobCtx.Error(context.Background(), "steam-auth 容器运行失败："+paneldocker.RedactString(cmdErr.Error()))
		return false, fmt.Errorf("steam-auth run error: %w", cmdErr)
	}

	if ctx.Err() != nil {
		if r.authOnly {
			r.updatePhase(context.Background(), storage.InstanceStateSteamAuthFailed,
				"Steam 邀请码授权超时，请重试授权。", "steam_invite_auth_failed", jobCtx.ID)
			return false, fmt.Errorf("Steam invite authorization timed out: %w", ctx.Err())
		}
		if sdkDownloaded {
			if err := r.completeInstall(ctx, jobCtx); err != nil {
				return false, err
			}
			return false, nil
		}
		r.updatePhase(context.Background(), storage.InstanceStateError,
			"安装任务超时，Steam 认证未完成，请重试安装。", "install_timeout", jobCtx.ID)
		return false, fmt.Errorf("steam-auth timed out: %w", ctx.Err())
	}

	if connectionFailed && !authSucceeded {
		if attempt < maxAttempts {
			return true, fmt.Errorf("steam client not connected")
		}
		message := "Steam 连接建立超时，请检查网络后重试安装。"
		phase := "steam_auth_connection_failed"
		if r.authOnly {
			message = "Steam 邀请码授权连接超时，请检查网络后重试授权。"
			phase = "steam_invite_auth_failed"
		}
		r.updatePhase(context.Background(), storage.InstanceStateSteamAuthFailed,
			message, phase, jobCtx.ID)
		_, _ = jobCtx.Error(context.Background(), "Steam 认证失败：SteamClient 多次未能在上游等待时间内完成连接。")
		return false, fmt.Errorf("steam client not connected after %d attempts", maxAttempts)
	}

	if downloadFailed {
		if r.authOnly {
			r.updatePhase(context.Background(), storage.InstanceStateSteamAuthFailed,
				"Steam 邀请码授权失败；不会启动 SteamCMD 或修改游戏文件。", "steam_invite_auth_failed", jobCtx.ID)
			return false, errors.New("Steam invite authorization failed")
		}
		_, _ = jobCtx.Warn(context.Background(), "旧版 steam-auth 游戏下载失败；正在改用 SteamCMD 安装主链修复游戏文件。")
		if err := r.runSteamCMDFallback(ctx, jobCtx, guardCh); err != nil {
			return false, err
		}
		if err := r.completeInstall(ctx, jobCtx); err != nil {
			return false, err
		}
		return false, nil
	}

	if qrAuthFailed {
		message := "二维码登录失败，请改用账号密码或 Steam Guard 后重试。"
		phase := "qr_auth_failed"
		if r.authOnly {
			message = "Steam 邀请码二维码授权失败，请改用账号密码或 Steam Guard 后重试授权。"
			phase = "steam_invite_auth_failed"
		}
		r.updatePhase(context.Background(), storage.InstanceStateSteamAuthFailed,
			message, phase, jobCtx.ID)
		_, _ = jobCtx.Error(context.Background(), "Steam 二维码登录失败：SteamClient 未连接，请改用账号密码或 Steam Guard。")
		return false, fmt.Errorf("steam qr authentication failed")
	}

	if authFailed {
		phase := "steam_auth_failed"
		message := "Steam 认证失败，请检查日志后重试。"
		errorLog := "Steam 认证失败：请检查任务日志中的 Steam 返回信息。"
		if credentialFailed {
			phase = "credentials_required"
			message = "Steam 账号或密码认证失败，请重新输入凭据后重试。"
			errorLog = "Steam 认证失败：账号或密码可能不正确。"
		}
		if r.authOnly {
			phase = "steam_invite_auth_failed"
			message = "Steam 邀请码授权失败，请检查账号、密码或验证码后重试授权。"
			errorLog = "Steam 邀请码授权失败：请检查任务日志中的 Steam 返回信息。"
		}
		if exitCode == 139 {
			phase = "steam_auth_console_failed"
			message = "Steam 认证因上游交互控制台不可用而失败，请更新后重试。"
			errorLog = "Steam 认证失败：上游交互式密码读取需要真实控制台。"
			if r.authOnly {
				phase = "steam_invite_auth_failed"
				message = "Steam 邀请码授权控制台异常，请更新后重试授权。"
				errorLog = "Steam 邀请码授权失败：上游交互式密码读取需要真实控制台。"
			}
		}
		r.updatePhase(context.Background(), storage.InstanceStateSteamAuthFailed,
			message, phase, jobCtx.ID)
		_, _ = jobCtx.Error(context.Background(), errorLog)
		return false, fmt.Errorf("steam authentication failed")
	}

	if authSucceeded || sdkDownloaded {
		if err := r.completeInstall(ctx, jobCtx); err != nil {
			return false, err
		}
		return false, nil
	}

	message := "Steam 认证失败，请检查日志并重试。"
	phase := "steam_auth_failed"
	if r.authOnly {
		message = "Steam 邀请码授权失败，请检查日志并重试授权。"
		phase = "steam_invite_auth_failed"
	}
	r.updatePhase(context.Background(), storage.InstanceStateSteamAuthFailed,
		message, phase, jobCtx.ID)
	_, _ = jobCtx.Error(context.Background(), fmt.Sprintf("steam-auth 以退出码 %d 结束。", exitCode))
	if r.authOnly {
		return false, fmt.Errorf("Steam invite authorization exited with code %d", exitCode)
	}
	return false, fmt.Errorf("steam-auth download exited with code %d", exitCode)
}

func (r *installRunner) clearSteamAuthSessionVolumes(ctx context.Context, jobCtx *jobs.Context) error {
	_, _ = jobCtx.Info(ctx, "正在清除 Steam 邀请码授权 session；SteamCMD 缓存和游戏文件保持不变。")
	result, sessionVolume, err := r.driver.removeSteamInviteAuthSessionHolders(ctx, r.instance.DataDir)
	if err != nil {
		message := "无法移除占用 Steam 邀请码授权 session 的旧容器，已停止重新授权：" + paneldocker.RedactString(err.Error())
		if detail := dockerResultDetail(result); detail != "" {
			message += ": " + detail
		}
		_, _ = jobCtx.Error(ctx, message)
		return fmt.Errorf("remove Steam invite session holders: %w", err)
	}
	names := []string{sessionVolume}
	if result, err := r.driver.docker.RemoveVolumes(ctx, r.instance.DataDir, names); err != nil {
		message := "无法清除 Steam 邀请码授权 session，已停止重新授权：" + paneldocker.RedactString(err.Error())
		if detail := dockerResultDetail(result); detail != "" {
			message += ": " + detail
		}
		_, _ = jobCtx.Error(ctx, message)
		return fmt.Errorf("remove Steam invite session volume: %w", err)
	}
	_, _ = jobCtx.Info(ctx, "Steam 邀请码授权 session 已清理并释放。")
	return nil
}

func (r *installRunner) clearSteamCMDAuthorizationVolumes(ctx context.Context, jobCtx *jobs.Context) {
	projectName := strings.ToLower(filepath.Base(r.instance.DataDir))
	names := []string{
		projectName + "_steamcmd-login",
		projectName + "_steamcmd-home",
		projectName + "_steamcmd-user-local",
		projectName + "_steamcmd-root-local",
	}
	_, _ = jobCtx.Info(ctx, "正在清除 SteamCMD 安装授权缓存；Steam 邀请码 session 和游戏文件保持不变。")
	if _, err := r.driver.docker.RemoveVolumes(ctx, r.instance.DataDir, names); err != nil {
		_, _ = jobCtx.Warn(ctx, "清除 SteamCMD 授权缓存卷时出现问题，将继续尝试重新认证："+paneldocker.RedactString(err.Error()))
	}
}

// markSteamAuthCompleted persists that Steam authentication has succeeded at
// least once. It is a durable, cross-session signal that lets later operations
// skip the interactive steam-auth login step (see authAlreadySucceeded).
func (r *installRunner) markSteamAuthCompleted(jobCtx *jobs.Context, state string) error {
	if err := sjconfig.SetSteamAuthCompletedState(r.instance.DataDir, state); err != nil {
		_, _ = jobCtx.Warn(context.Background(), "记录 Steam 认证状态失败，后续可能需要再次认证。")
		return err
	}
	return nil
}

func (r *installRunner) completeInstall(ctx context.Context, jobCtx *jobs.Context) error {
	if err := r.ensureSMAPIInstalled(ctx, jobCtx); err != nil {
		r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateError,
			"SMAPI 运行环境安装失败，请检查任务日志后重试。", "smapi_install_failed", jobCtx.ID)
		_, _ = jobCtx.Error(context.Background(), "SMAPI 运行环境安装失败："+paneldocker.RedactString(err.Error()))
		return fmt.Errorf("install smapi runtime: %w", err)
	}
	imageRef := gameInstallImage(r.instance.DataDir)
	_, _ = jobCtx.Info(context.Background(), "正在同步 SMAPI 内置支持 Mod，确保首次启动前运行目录完整...")
	changed, err := r.driver.EnsureManagedSMAPIBundledMods(ctx, r.instance.DataDir, imageRef, func(line string) {
		_, _ = jobCtx.Info(context.Background(), "[smapi-sync] "+paneldocker.RedactString(line))
	})
	if err != nil {
		r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateError,
			"SMAPI 内置支持 Mod 同步失败，请检查任务日志后重试。", "smapi_bundled_sync_failed", jobCtx.ID)
		_, _ = jobCtx.Error(context.Background(), "SMAPI 内置支持 Mod 同步失败："+paneldocker.RedactString(err.Error()))
		return fmt.Errorf("sync SMAPI bundled runtime mods: %w", err)
	}
	if changed {
		_, _ = jobCtx.Info(context.Background(), "SMAPI 内置支持 Mod 已在首次启动前完成物化。")
	} else {
		_, _ = jobCtx.Info(context.Background(), "SMAPI 内置支持 Mod 已是最新，无需替换。")
	}
	_, _ = jobCtx.Info(context.Background(), "正在验证 Stardew、SMAPI 与 Steam SDK 运行文件...")
	ok, err := r.driver.verifyGameDataVolume(ctx, r.instance.DataDir, imageRef, func(line string) {
		_, _ = jobCtx.Info(context.Background(), "[verify] "+paneldocker.RedactString(line))
	})
	if err != nil || !ok {
		message := "游戏运行文件不完整，请重新安装或修复。"
		if err != nil {
			message = "验证游戏运行文件失败，请检查任务日志后重试。"
		}
		r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateError,
			message, "install_verification_failed", jobCtx.ID)
		if err != nil {
			_, _ = jobCtx.Error(context.Background(), "游戏运行文件验证失败："+paneldocker.RedactString(err.Error()))
			return fmt.Errorf("verify game runtime files: %w", err)
		}
		_, _ = jobCtx.Error(context.Background(), "游戏运行文件不完整：Stardew、SMAPI 或 Steam SDK 的必需文件缺失。")
		return fmt.Errorf("game runtime files are incomplete")
	}
	r.markInstallSucceeded(jobCtx)
	return nil
}

func (r *installRunner) ensureSMAPIInstalled(ctx context.Context, jobCtx *jobs.Context) error {
	envVals, _ := sjconfig.ReadEnvFile(filepath.Join(r.instance.DataDir, ".env"))
	r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateSteamAuthRunning,
		"游戏文件和 Steam SDK 已完成，正在检查 SMAPI 安装包缓存...", "smapi_installing", jobCtx.ID)

	imageRef := envWithDefault(envVals, "SERVER_IMAGE", serverImageDefault(r.imageTag))
	manifest, err := sjconfig.BuiltInRuntimeStackManifest()
	if err != nil {
		return err
	}
	_, _ = jobCtx.Info(context.Background(), "[smapi] 正在检查本地 SMAPI 安装包缓存；如需下载，页面会显示实时进度。")
	var (
		progressObserved      bool
		lastProgressBytes     int64 = -1
		lastProgressAt              = time.Time{}
		lastProgressCandidate       = -1
	)
	emitProgress := func(progress smapiArchiveDownloadProgress) {
		now := time.Now()
		candidateChanged := progress.Candidate != lastProgressCandidate
		complete := progress.TotalBytes > 0 && progress.DownloadedBytes >= progress.TotalBytes
		if progressObserved && !candidateChanged && !progress.Cached && !complete &&
			progress.DownloadedBytes-lastProgressBytes < 512*1024 && now.Sub(lastProgressAt) < 2*time.Second {
			return
		}
		progressObserved = true
		lastProgressBytes = progress.DownloadedBytes
		lastProgressAt = now
		lastProgressCandidate = progress.Candidate
		_, _ = jobCtx.Info(context.Background(), fmt.Sprintf(
			"[smapi:download:progress:%d:%d:%d:%d:%t]",
			progress.DownloadedBytes,
			progress.TotalBytes,
			progress.Candidate,
			progress.CandidateCount,
			progress.Cached,
		))

		percent := 0
		if progress.TotalBytes > 0 {
			percent = int(float64(progress.DownloadedBytes) * 100 / float64(progress.TotalBytes))
		}
		message := fmt.Sprintf("正在下载 SMAPI 安装包（%d%%，下载源 %d/%d），请稍候...", percent, progress.Candidate, progress.CandidateCount)
		if progress.Cached {
			message = "本地 SMAPI 安装包缓存校验通过，正在准备安装..."
		} else if complete {
			message = "SMAPI 安装包下载完成，正在校验完整性..."
		}
		r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateSteamAuthRunning,
			message, "smapi_installing", jobCtx.ID)
	}
	archivePath := ""
	if provider, ok := r.driver.docker.(interface {
		RecommendedSMAPIArchive(context.Context, string, sjconfig.RuntimeStackManifest) (string, error)
	}); ok {
		archivePath, err = provider.RecommendedSMAPIArchive(ctx, r.instance.DataDir, manifest)
	} else {
		archivePath, err = ensureRecommendedSMAPIArchiveWithProgress(ctx, r.instance.DataDir, manifest, emitProgress)
	}
	if err != nil {
		return fmt.Errorf("download reviewed SMAPI installer: %w", err)
	}
	if !progressObserved {
		emitProgress(smapiArchiveDownloadProgress{
			DownloadedBytes: manifest.SMAPI.ArchiveBytes,
			TotalBytes:      manifest.SMAPI.ArchiveBytes,
			Candidate:       1,
			CandidateCount:  max(1, len(manifest.SMAPI.URLs)),
			Cached:          true,
		})
	}
	_, _ = jobCtx.Info(context.Background(), "[smapi] SMAPI 安装包已通过完整性校验，正在写入游戏运行目录。")
	hostArchivePath, err := r.driver.dockerHostPath(archivePath)
	if err != nil {
		return fmt.Errorf("map reviewed SMAPI installer for Docker: %w", err)
	}
	_, _ = jobCtx.Info(context.Background(), fmt.Sprintf("[smapi] 使用 JunimoServer 镜像 %s 预安装 SMAPI。", imageRef))
	exitCode, err := r.driver.docker.RunContainerTTY(ctx, r.buildSMAPIInstallOpts(imageRef, hostArchivePath), nil, func(line string) {
		_, _ = jobCtx.Info(context.Background(), "[smapi] "+paneldocker.RedactString(line))
	})
	if err != nil {
		if ctx.Err() != nil {
			r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateError,
				"安装任务超时，SMAPI 运行环境未安装完成，请重试安装。", "install_timeout", jobCtx.ID)
			return fmt.Errorf("smapi preinstall timed out: %w", ctx.Err())
		}
		return fmt.Errorf("smapi preinstall container: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("smapi preinstall exited with code %d", exitCode)
	}
	_, _ = jobCtx.Info(context.Background(), "[smapi] SMAPI 运行环境已安装完成。")
	return nil
}

func (r *installRunner) markInstallSucceeded(jobCtx *jobs.Context) {
	r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateGameInstalled,
		"游戏已安装。", "game_installed", jobCtx.ID)
	_, _ = jobCtx.Info(context.Background(), "安装流程全部完成。")
}

func (r *installRunner) runSteamCMDFallback(ctx context.Context, jobCtx *jobs.Context, guardCh <-chan string) error {
	envVals, _ := sjconfig.ReadEnvFile(filepath.Join(r.instance.DataDir, ".env"))
	imageRef, err := r.ensureSteamCMDImage(ctx, jobCtx, steamCMDImageRefs(envVals))
	if err != nil {
		return err
	}
	legacyCacheAvailable := r.migrateLegacySteamCMDAuthCache(ctx, jobCtx, imageRef)
	if !r.steamCMDUseCache && legacyCacheAvailable {
		r.steamCMDUseCache = true
		_, _ = jobCtx.Info(context.Background(), "[steamcmd] 检测到已保存的 SteamCMD 授权缓存，本次先尝试免验证登录。")
	}

	downloadMessage := "正在使用 SteamCMD 下载并校验 Stardew Valley 与 Steamworks SDK 文件..."
	if r.steamCMDUseCache {
		downloadMessage = "正在复用已保存凭据和 SteamCMD 授权缓存下载/校验游戏文件..."
	}
	r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateSteamAuthRunning,
		downloadMessage, "steamcmd_downloading", jobCtx.ID)
	_, _ = jobCtx.Info(context.Background(), fmt.Sprintf("[steamcmd] 使用 SteamCMD 镜像 %s。", imageRef))
	_, _ = jobCtx.Info(context.Background(), "[steamcmd] Docker 镜像检查已完成；后续若看到 [----] Downloading update，是 SteamCMD 客户端自更新，不是 Docker 镜像拉取。")
	if r.steamCMDUseCache {
		_, _ = jobCtx.Info(context.Background(), "[steamcmd] 跳过 steam-auth，优先使用已保留的 SteamCMD 登录授权直接下载/校验。")
	} else {
		_, _ = jobCtx.Info(context.Background(), "[steamcmd] 使用已保存的 Steam 账号密码启动 SteamCMD 安装；如 Steam 要求重新授权，页面会提示选择手机批准或输入验证码。")
	}

	var (
		outputMu             sync.Mutex
		app413150Done        bool
		app1007Done          bool
		credentialFailed     bool
		guardPrompted        bool
		mobileApproval       bool
		authTimedOut         bool
		steamCMDLoggedIn     bool
		downloadStarted      bool
		downloadCompleted    bool
		cachedPromptCanceled bool
	)
	resetAttemptState := func() {
		outputMu.Lock()
		defer outputMu.Unlock()
		app413150Done = false
		app1007Done = false
		credentialFailed = false
		guardPrompted = false
		mobileApproval = false
		authTimedOut = false
		steamCMDLoggedIn = false
		downloadStarted = false
		downloadCompleted = false
		cachedPromptCanceled = false
	}
	lineHandler := func(useCachedAuth bool, cancelAttempt context.CancelFunc) func(string) {
		return func(line string) {
			outputMu.Lock()
			defer outputMu.Unlock()

			safeLine := sanitizeSteamOutputLine(line, r.password)
			_, _ = jobCtx.Info(context.Background(), "[steamcmd] "+safeLine)
			lower := strings.ToLower(line)

			cancelCachedAttempt := func() bool {
				if !useCachedAuth {
					return false
				}
				credentialFailed = true
				if !cachedPromptCanceled {
					cachedPromptCanceled = true
					r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateSteamAuthRunning,
						"SteamCMD 已保存授权缓存需要重新验证，正在自动切换为账号密码完整登录...",
						"steamcmd_auth_running", jobCtx.ID)
					cancelAttempt()
				}
				return true
			}

			switch {
			case isSteamCMDGuardChoiceMenu(lower):
				guardPrompted = true
				if cancelCachedAttempt() {
					return
				}
				r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateSteamAuthRunning,
					"SteamCMD 需要登录授权；请选择手机 App 批准或输入 App/邮箱验证码。",
					"steamcmd_guard_choice_required", jobCtx.ID)
			case containsAny(lower, "timed out waiting for confirmation", "wait for confirmation timed out", "error (timeout)"):
				authTimedOut = true
			case isSteamCMDMobileApprovalPrompt(lower):
				guardPrompted = true
				mobileApproval = true
				if cancelCachedAttempt() {
					return
				}
				r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateSteamAuthRunning,
					"SteamCMD 需要登录授权；请打开 Steam 手机 App 批准本次登录。",
					"steamcmd_guard_mobile_required", jobCtx.ID)
			case isSteamCMDGuardCodePrompt(lower):
				guardPrompted = true
				if cancelCachedAttempt() {
					return
				}
				r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateSteamAuthRunning,
					"SteamCMD 需要登录授权；请输入 Steam App 或邮箱验证码。",
					"steamcmd_guard_required", jobCtx.ID)
			case containsAny(lower,
				"invalid password",
				"password check for user failed",
				"login failure",
				"invalid login auth code",
				"cached credentials not found",
				"no cached credentials",
				"using cached credentials failed",
			):
				// SteamCMD can report login progress and the terminal credential error
				// on the same line (for example, "Logging in user ... Invalid
				// Password"). Keep this before the generic progress cases so the
				// overlapping line cannot be misclassified as a download failure.
				credentialFailed = true
			case containsAny(lower, "waiting for user info...ok", "logged in ok"):
				steamCMDLoggedIn = true
			case containsAny(lower, "logging in user", "waiting for user info"):
				if !guardPrompted && !mobileApproval {
					message := "正在使用已保存账号密码登录 SteamCMD..."
					if useCachedAuth {
						message = "正在使用已保存的 SteamCMD 授权缓存登录..."
					}
					r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateSteamAuthRunning,
						message, "steamcmd_auth_running", jobCtx.ID)
				}
			case containsAny(lower, "downloading update", "update state", "downloading, progress", "validating"):
				// SteamCMD emits client self-update progress before it authenticates.
				// Treat progress as a game download only after an explicit login marker;
				// otherwise a failed cached login could be persisted as authorized.
				if steamCMDLoggedIn {
					downloadStarted = true
					r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateSteamAuthRunning,
						"SteamCMD 已授权，正在下载并校验游戏文件...", "steamcmd_downloading", jobCtx.ID)
				}
			case strings.Contains(lower, "success! app '413150' fully installed"):
				app413150Done = true
				downloadCompleted = true
				r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateSteamAuthRunning,
					"SteamCMD 已完成 Stardew Valley 游戏文件下载，正在处理 Steam SDK 运行文件...", "steamcmd_downloading", jobCtx.ID)
			case strings.Contains(lower, "success! app '1007' fully installed"):
				app1007Done = true
				downloadCompleted = true
			}
		}
	}

	runSteamCMD := func() (int, error) {
		resetAttemptState()
		useCachedAuth := r.steamCMDUseCache
		attemptCtx, cancelAttempt := context.WithCancel(ctx)
		defer cancelAttempt()
		exitCode, err := r.driver.docker.RunContainerTTY(
			attemptCtx,
			r.buildSteamCMDOpts(imageRef),
			guardCh,
			lineHandler(useCachedAuth, cancelAttempt),
		)
		outputMu.Lock()
		deliberatelyCanceled := cachedPromptCanceled
		outputMu.Unlock()
		if deliberatelyCanceled && ctx.Err() == nil && errors.Is(err, context.Canceled) {
			return exitCode, nil
		}
		return exitCode, err
	}
	exitCode, err := runSteamCMD()
	if err != nil {
		if ctx.Err() != nil {
			r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateError,
				"安装任务超时，SteamCMD 下载未完成，请重试安装。", "install_timeout", jobCtx.ID)
			return fmt.Errorf("SteamCMD install timed out: %w", ctx.Err())
		}
		r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateError,
			"SteamCMD 下载运行失败，请检查任务日志后重试。", "steamcmd_failed", jobCtx.ID)
		_, _ = jobCtx.Error(context.Background(), "SteamCMD 下载运行失败："+paneldocker.RedactString(err.Error()))
		return fmt.Errorf("SteamCMD install run error: %w", err)
	}
	if r.steamCMDUseCache && credentialFailed {
		_, _ = jobCtx.Warn(context.Background(), "[steamcmd] 已保存的 SteamCMD 授权缓存不可用，正在自动切换为账号密码完整登录。")
		if updateErr := sjconfig.UpdateEnvFile(filepath.Join(r.instance.DataDir, ".env"), map[string]string{
			"STEAMCMD_AUTH_COMPLETED": "",
		}); updateErr != nil {
			_, _ = jobCtx.Warn(context.Background(), "清除失效的 SteamCMD 授权状态失败；本次仍会尝试重新认证。")
		}
		r.steamCMDUseCache = false
		exitCode, err = runSteamCMD()
		if err != nil {
			if ctx.Err() != nil {
				r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateError,
					"SteamCMD 重新认证超时，请重试安装。", "install_timeout", jobCtx.ID)
				return fmt.Errorf("steamcmd full login timed out after cached login failed: %w", ctx.Err())
			}
			return fmt.Errorf("steamcmd full login run error after cached login failed: %w", err)
		}
	}
	if exitCode == 139 {
		_, _ = jobCtx.Warn(context.Background(), "[steamcmd] SteamCMD exited with segmentation fault (139); preserving authorization volumes and retrying once.")
		r.clearSteamCMDRuntimeCache(context.Background(), jobCtx)
		exitCode, err = runSteamCMD()
		if err != nil {
			if ctx.Err() != nil {
				r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateError,
					"SteamCMD retry timed out after clearing runtime cache.", "install_timeout", jobCtx.ID)
				return fmt.Errorf("SteamCMD install timed out after cache cleanup: %w", ctx.Err())
			}
			r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateError,
				"SteamCMD retry failed after clearing runtime cache.", "steamcmd_failed", jobCtx.ID)
			_, _ = jobCtx.Error(context.Background(), "SteamCMD retry failed after clearing runtime cache: "+paneldocker.RedactString(err.Error()))
			return fmt.Errorf("SteamCMD install retry run error: %w", err)
		}
	}
	if ctx.Err() != nil {
		r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateError,
			"安装任务超时，SteamCMD 下载未完成，请重试安装。", "install_timeout", jobCtx.ID)
		return fmt.Errorf("SteamCMD install timed out: %w", ctx.Err())
	}
	if steamCMDLoggedIn || app413150Done || app1007Done {
		// SteamCMD authorized successfully at least once (explicit "logged in ok",
		// or an app success marker, which necessarily follows login): persist ONLY
		// the SteamCMD flag so its next run reuses the
		// cached (username-only) login. steam-auth and SteamCMD share the saved
		// STEAM_USERNAME/STEAM_PASSWORD, but keep authorization caches, sessions,
		// and completion state independent. A SteamCMD login therefore must NOT
		// imply steam-auth succeeded — that is tracked separately by
		// STEAM_AUTH_COMPLETED. Recorded even if the download later failed, because
		// the authorization is valid.
		if err := sjconfig.UpdateEnvFile(filepath.Join(r.instance.DataDir, ".env"), map[string]string{
			"STEAMCMD_AUTH_COMPLETED": "true",
		}); err != nil {
			_, _ = jobCtx.Warn(context.Background(), "记录 SteamCMD 授权状态失败，后续可能需要再次授权。")
		}
	}
	if authTimedOut {
		r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateError,
			"SteamCMD 等待 Steam 手机 App 批准超时，请重试并及时批准，或改用验证码方式。", "steamcmd_failed", jobCtx.ID)
		_, _ = jobCtx.Error(context.Background(), "SteamCMD 等待 Steam 手机 App 批准超时，未开始下载游戏文件。")
		return fmt.Errorf("SteamCMD install authorization confirmation timed out")
	}
	if exitCode != 0 {
		phase := "steamcmd_failed"
		message := "SteamCMD 下载失败，请检查任务日志后重试。"
		state := storage.InstanceStateError
		if credentialFailed {
			if r.steamCMDUseCache {
				message = "SteamCMD 已保存授权缓存不可用，无法直接修复下载；请先完成一次 SteamCMD 授权后再重试。"
			} else {
				phase = "credentials_required"
				message = "SteamCMD 重新授权失败：账号、密码或验证码不正确，请重新输入 Steam 凭据后重试。"
				state = storage.InstanceStateCredentialsRequired
			}
		}
		r.driver.updatePhase(context.Background(), r.instance.ID, state, message, phase, jobCtx.ID)
		_, _ = jobCtx.Error(context.Background(), fmt.Sprintf("SteamCMD 下载以退出码 %d 结束。", exitCode))
		return fmt.Errorf("SteamCMD install exited with code %d", exitCode)
	}

	if !app413150Done {
		r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateError,
			"SteamCMD 下载未确认完成，请检查任务日志后重试。", "steamcmd_failed", jobCtx.ID)
		if downloadStarted || downloadCompleted {
			_, _ = jobCtx.Error(context.Background(), "SteamCMD 输出过下载进度，但没有看到 Stardew Valley 游戏文件安装完成信号。")
		} else {
			_, _ = jobCtx.Error(context.Background(), "SteamCMD 下载结束，但日志中没有看到游戏文件安装完成信号。")
		}
		return fmt.Errorf("SteamCMD install finished without success marker")
	}
	if !app1007Done {
		_, _ = jobCtx.Warn(context.Background(), "SteamCMD 未输出 Steam SDK 完成标记；如果后续启动缺少 SDK 运行文件，请重新执行安装修复。")
	}
	_, _ = jobCtx.Info(context.Background(), "[steamcmd] SteamCMD 下载完成。")
	return nil
}

func (r *installRunner) ensureSteamCMDImage(ctx context.Context, jobCtx *jobs.Context, imageRefs []string) (string, error) {
	if len(imageRefs) == 0 {
		r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateError,
			"SteamCMD 安装镜像未配置，请在实例 .env 中配置 STEAMCMD_IMAGE 或 STEAMCMD_IMAGE_CANDIDATES。", "steamcmd_image_pull_failed", jobCtx.ID)
		_, _ = jobCtx.Error(context.Background(), "SteamCMD 安装镜像未配置。")
		return "", fmt.Errorf("steamcmd image candidates are empty")
	}

	for i, imageRef := range imageRefs {
		if _, err := r.driver.docker.ImageInspect(ctx, r.instance.DataDir, imageRef); err == nil {
			if i == 0 {
				_, _ = jobCtx.Info(context.Background(), fmt.Sprintf("[steamcmd] 本地已有 SteamCMD 镜像 %s，直接使用。", imageRef))
			} else {
				_, _ = jobCtx.Info(context.Background(), fmt.Sprintf("[steamcmd] 使用已存在的备用 SteamCMD 镜像 %s。", imageRef))
			}
			return imageRef, nil
		}
	}

	var failures []string
	for i, imageRef := range imageRefs {
		if _, err := r.driver.docker.ImageInspect(ctx, r.instance.DataDir, imageRef); err == nil {
			if i > 0 {
				_, _ = jobCtx.Info(context.Background(), fmt.Sprintf("[steamcmd] 使用已存在的备用 SteamCMD 镜像 %s。", imageRef))
			}
			return imageRef, nil
		}

		r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateSteamAuthRunning,
			"正在拉取 SteamCMD 安装镜像，请稍候...", "steamcmd_image_pulling", jobCtx.ID)
		_, _ = jobCtx.Info(context.Background(), fmt.Sprintf("[steamcmd] 本地缺少 SteamCMD 镜像 %s，正在拉取（%d/%d）。", imageRef, i+1, len(imageRefs)))
		if _, pullErr := r.driver.docker.PullImageStreaming(ctx, r.instance.DataDir, imageRef,
			makeImagePullLineHandler(jobCtx, "[steamcmd:pull] ", func(done, total int) {
				percent := 0
				if total > 0 {
					percent = int(float64(done) * 100 / float64(total))
				}
				message := fmt.Sprintf("正在拉取 SteamCMD 安装镜像（约 %d%%，%d/%d 层），请稍候...", percent, done, total)
				if done == total && total > 0 {
					message = "SteamCMD 安装镜像拉取完成，正在启动 SteamCMD..."
				}
				r.driver.updatePhase(context.Background(), r.instance.ID,
					storage.InstanceStateSteamAuthRunning, message, "steamcmd_image_pulling", jobCtx.ID)
			})); pullErr != nil {
			safeErr := paneldocker.RedactString(pullErr.Error())
			failures = append(failures, fmt.Sprintf("%s: %s", imageRef, safeErr))
			if i+1 < len(imageRefs) {
				_, _ = jobCtx.Warn(context.Background(), fmt.Sprintf("SteamCMD 镜像 %s 拉取失败，正在尝试备用镜像。", imageRef))
				continue
			}
			r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateError,
				"SteamCMD 安装镜像全部拉取失败，请检查 Docker 镜像源或在实例 .env 中配置 STEAMCMD_IMAGE_CANDIDATES。", "steamcmd_image_pull_failed", jobCtx.ID)
			_, _ = jobCtx.Error(context.Background(), "SteamCMD 安装镜像拉取失败："+strings.Join(failures, "；"))
			return "", fmt.Errorf("pull steamcmd image candidates: %s", strings.Join(failures, "; "))
		}
		return imageRef, nil
	}

	r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateError,
		"SteamCMD 安装镜像未配置，请在实例 .env 中配置 STEAMCMD_IMAGE 或 STEAMCMD_IMAGE_CANDIDATES。", "steamcmd_image_pull_failed", jobCtx.ID)
	_, _ = jobCtx.Error(context.Background(), "SteamCMD 安装镜像未配置。")
	return "", fmt.Errorf("steamcmd image candidates are empty")
}

func makeImagePullLineHandler(jobCtx *jobs.Context, prefix string, onProgress func(done, total int)) func(string) {
	var (
		mu       sync.Mutex
		layers   = map[string]bool{}
		lastEmit string
	)

	return func(line string) {
		_, _ = jobCtx.Info(context.Background(), prefix+line)

		m := regexImagePullLayer.FindStringSubmatch(line)
		if m == nil {
			return
		}

		mu.Lock()
		defer mu.Unlock()

		layerID := strings.ToLower(m[1])
		status := strings.ToLower(m[2])
		doneStatus := status == "pull complete" || status == "already exists"
		if _, ok := layers[layerID]; !ok {
			layers[layerID] = false
		}
		if doneStatus {
			layers[layerID] = true
		}

		total := len(layers)
		done := 0
		for _, layerDone := range layers {
			if layerDone {
				done++
			}
		}
		if total == 0 {
			return
		}

		tag := fmt.Sprintf("[pull:progress:%d:%d]", done, total)
		if tag == lastEmit {
			return
		}
		lastEmit = tag
		_, _ = jobCtx.Info(context.Background(), tag)
		if onProgress != nil {
			onProgress(done, total)
		}
	}
}

func (r *installRunner) buildSteamCMDOpts(imageRef string) paneldocker.ContainerTTYRunOpts {
	projectName := strings.ToLower(filepath.Base(r.instance.DataDir))
	gameCommand := `"$STEAMCMD_BIN" +force_install_dir /data/game +login "$STEAM_USERNAME" "$STEAM_PASSWORD" +app_update 413150 validate +quit`
	if r.steamCMDUseCache {
		// NoPromptForPassword makes a missing/expired cache fail promptly instead of
		// hanging at an interactive password prompt. The caller detects that failure
		// and automatically reruns this command with the full credentials.
		gameCommand = `"$STEAMCMD_BIN" +@NoPromptForPassword 1 +force_install_dir /data/game +login "$STEAM_USERNAME" +app_update 413150 validate +quit`
	}
	// The Steamworks SDK redistributable (app 1007) is public and downloads with an
	// anonymous login — no account credentials and no Steam Guard. Only the game
	// login needs the real account, so a whole install needs at most one guard
	// approval, and the SDK step can never stall waiting for credentials.
	sdkCommand := `"$STEAMCMD_BIN" +force_install_dir /data/game/.steam-sdk +login anonymous +app_update 1007 validate +quit`
	steamHomePrefix := `HOME=/home/steam USER=steam LOGNAME=steam `
	suGameCommand := strings.ReplaceAll(steamHomePrefix+gameCommand, `'`, `'"'"'`)
	suSDKCommand := strings.ReplaceAll(steamHomePrefix+sdkCommand, `'`, `'"'"'`)
	script := strings.Join([]string{
		"set -e",
		"mkdir -p /data/game /data/game/.steam-sdk /home/steam/Steam /home/steam/.steam /home/steam/.local/share/Steam /root/Steam /root/.steam /root/.local/share/Steam",
		"if id steam >/dev/null 2>&1; then chown -R steam:steam /data/game /home/steam/Steam /home/steam/.steam /home/steam/.local/share/Steam /root/Steam /root/.steam /root/.local/share/Steam; fi",
		`if [ -x /home/steam/steamcmd/steamcmd.sh ]; then steamcmd_bin=/home/steam/steamcmd/steamcmd.sh; elif command -v steamcmd >/dev/null 2>&1; then steamcmd_bin=$(command -v steamcmd); elif [ -x /usr/games/steamcmd ]; then steamcmd_bin=/usr/games/steamcmd; elif [ -x /steamcmd/steamcmd.sh ]; then steamcmd_bin=/steamcmd/steamcmd.sh; else echo "SteamCMD executable not found in container" >&2; exit 127; fi`,
		`export STEAMCMD_BIN="$steamcmd_bin"`,
		`echo "Running SteamCMD app_update 413150..."`,
		`if id steam >/dev/null 2>&1 && command -v su >/dev/null 2>&1; then su -m steam -c '` + suGameCommand + `'; else ` + gameCommand + `; fi`,
		`echo "Running SteamCMD app_update 1007..."`,
		`if id steam >/dev/null 2>&1 && command -v su >/dev/null 2>&1; then su -m steam -c '` + suSDKCommand + `'; else ` + sdkCommand + `; fi`,
	}, "; ")
	return paneldocker.ContainerTTYRunOpts{
		ImageRef:   imageRef,
		Entrypoint: []string{"/bin/sh"},
		User:       "root",
		Command:    []string{"-lc", script},
		Env: []string{
			"STEAM_USERNAME=" + r.username,
			"STEAM_PASSWORD=" + r.password,
		},
		Binds: []string{
			// SteamCMD images do not agree on HOME or the canonical Steam data
			// directory. Use one authorization volume for every observed config
			// root so a session created by the root-based official image remains
			// visible to the steam-user CM2 image and vice versa.
			projectName + "_steamcmd-login:/home/steam/Steam",
			projectName + "_steamcmd-login:/home/steam/.local/share/Steam",
			projectName + "_steamcmd-home:/home/steam/.steam",
			projectName + "_steamcmd-login:/root/Steam",
			projectName + "_steamcmd-login:/root/.local/share/Steam",
			projectName + "_steamcmd-home:/root/.steam",
			resolvedGameDataVolumeName(r.instance.DataDir) + ":/data/game",
		},
	}
}

func (r *installRunner) buildSteamCMDAuthMigrationOpts(imageRef string) paneldocker.ContainerTTYRunOpts {
	projectName := strings.ToLower(filepath.Base(r.instance.DataDir))
	script := `set -eu
echo "anxi-steamcmd-auth-migrate: checking legacy authorization cache"
if [ -s /auth/config/config.vdf ]; then
  echo "anxi-steamcmd-auth-migrate: canonical cache already present"
  exit 0
fi
for legacy in /legacy/root /legacy/user; do
  if [ -s "${legacy}/config/config.vdf" ]; then
    mkdir -p /auth/config
    cp -a "${legacy}/config/." /auth/config/
    find "${legacy}" -maxdepth 1 -type f -name 'ssfn*' -exec cp -p '{}' /auth/ ';'
    echo "anxi-steamcmd-auth-migrate: migrated legacy cache"
    exit 0
  fi
done
echo "anxi-steamcmd-auth-migrate: no legacy cache found"
`
	return paneldocker.ContainerTTYRunOpts{
		ImageRef:   imageRef,
		Entrypoint: []string{"/bin/sh"},
		User:       "root",
		Command:    []string{"-lc", script},
		Binds: []string{
			projectName + "_steamcmd-login:/auth",
			projectName + "_steamcmd-root-local:/legacy/root:ro",
			projectName + "_steamcmd-user-local:/legacy/user:ro",
		},
	}
}

func (r *installRunner) migrateLegacySteamCMDAuthCache(ctx context.Context, jobCtx *jobs.Context, imageRef string) bool {
	cacheAvailable := false
	exitCode, err := r.driver.docker.RunContainerTTY(ctx, r.buildSteamCMDAuthMigrationOpts(imageRef), nil, func(line string) {
		if strings.HasPrefix(line, "anxi-steamcmd-auth-migrate:") {
			switch strings.TrimSpace(line) {
			case "anxi-steamcmd-auth-migrate: canonical cache already present", "anxi-steamcmd-auth-migrate: migrated legacy cache":
				cacheAvailable = true
			}
			_, _ = jobCtx.Info(context.Background(), "[steamcmd] "+strings.TrimPrefix(line, "anxi-steamcmd-auth-migrate: "))
		}
	})
	if err != nil || exitCode != 0 {
		message := "[steamcmd] 旧授权缓存迁移失败，将继续使用当前统一授权卷。"
		if err != nil {
			message += " " + paneldocker.RedactString(err.Error())
		}
		_, _ = jobCtx.Warn(context.Background(), message)
		return false
	}
	return cacheAvailable
}

func (r *installRunner) clearSteamCMDRuntimeCache(ctx context.Context, jobCtx *jobs.Context) {
	projectName := strings.ToLower(filepath.Base(r.instance.DataDir))
	names := []string{
		projectName + "_steamcmd-login",
		projectName + "_steamcmd-home",
	}
	if result, err := r.driver.docker.RemoveContainersByVolume(ctx, r.instance.DataDir, names); err != nil {
		detail := dockerResultDetail(result)
		message := "[steamcmd] Failed to remove stale SteamCMD containers before cache cleanup: " + paneldocker.RedactString(err.Error())
		if detail != "" {
			message += ": " + detail
		}
		_, _ = jobCtx.Warn(ctx, message)
	} else {
		_, _ = jobCtx.Info(ctx, "[steamcmd] Stale SteamCMD containers have been checked before runtime cache cleanup.")
	}
	// Do not delete these volumes: SteamCMD stores its login sentry/config under
	// Steam or .local/share/Steam depending on the image. Removing them after a 139
	// crash discards the approved-machine identity and forces Steam Guard again.
	_, _ = jobCtx.Info(ctx, "[steamcmd] SteamCMD authorization volumes were preserved for retry.")
}

func dockerResultDetail(result paneldocker.CommandResult) string {
	detail := strings.TrimSpace(result.Stderr)
	if detail == "" {
		detail = strings.TrimSpace(result.Stdout)
	}
	return paneldocker.RedactString(detail)
}

func (r *installRunner) buildSMAPIInstallOpts(imageRef, archivePath string) paneldocker.ContainerTTYRunOpts {
	manifest, manifestErr := sjconfig.BuiltInRuntimeStackManifest()
	version, checksum, archiveBytes, maxArchiveBytes := DefaultSMAPIVersion, "", int64(0), int64(100*1024*1024)
	if manifestErr == nil {
		version, checksum, archiveBytes, maxArchiveBytes = manifest.SMAPI.Version, manifest.SMAPI.SHA256, manifest.SMAPI.ArchiveBytes, manifest.SMAPI.MaxArchiveBytes
	}
	script := `set -eu
game_dir="/data/game"
smapi_bin="${game_dir}/StardewModdingAPI"
version="${SMAPI_VERSION:-4.5.2}"
if [ -x "${smapi_bin}" ] && [ -f "${game_dir}/Mods/ConsoleCommands/manifest.json" ] && [ -f "${game_dir}/Mods/SaveBackup/manifest.json" ]; then
  echo "SMAPI already installed at ${smapi_bin}, skipping."
  exit 0
fi
if [ ! -e "${game_dir}/StardewValley" ] && [ ! -e "${game_dir}/Stardew Valley.dll" ]; then
  echo "Stardew Valley game files are missing under ${game_dir}." >&2
  exit 2
fi
tmp_dir="/tmp/anxi-smapi-install"
rm -rf "${tmp_dir}"
mkdir -p "${tmp_dir}/extract"
echo "Installing Panel-recommended SMAPI ${version} from the reviewed read-only package."
actual_bytes="$(wc -c < /package/smapi.zip | tr -d ' ')"
if [ "${actual_bytes}" != "${SMAPI_ARCHIVE_BYTES}" ]; then echo "SMAPI archive length mismatch." >&2; exit 3; fi
actual_sha="$(sha256sum /package/smapi.zip | awk '{print $1}')"
if [ "${actual_sha}" != "${SMAPI_SHA256}" ]; then echo "SMAPI archive checksum mismatch." >&2; exit 3; fi
if ! unzip -t /package/smapi.zip >/dev/null; then echo "SMAPI archive is invalid." >&2; exit 3; fi
unzip -q /package/smapi.zip -d "${tmp_dir}/extract"
installer="${tmp_dir}/extract/SMAPI ${version} installer/internal/linux/SMAPI.Installer"
if [ ! -f "${installer}" ]; then
  installer="$(find "${tmp_dir}/extract" -path '*/internal/linux/SMAPI.Installer' -type f | head -n 1)"
fi
if [ -z "${installer}" ] || [ ! -f "${installer}" ]; then
  echo "SMAPI Linux installer was not found in the archive." >&2
  exit 4
fi
chmod +x "${installer}"
printf "2\n\n" | "${installer}" --install --game-path "${game_dir}"
if [ ! -x "${smapi_bin}" ]; then
  echo "SMAPI executable was not created at ${smapi_bin}." >&2
  exit 5
fi
mkdir -p "${game_dir}/smapi-internal"
cp -rf /data/smapi-config.json "${game_dir}/smapi-internal/config.user.json" 2>/dev/null || true
rm -rf "${tmp_dir}"
echo "SMAPI preinstall complete."
`
	return paneldocker.ContainerTTYRunOpts{
		ImageRef:   imageRef,
		Entrypoint: []string{"/bin/sh"},
		User:       "root",
		Command:    []string{"-lc", script},
		Env: []string{
			"SMAPI_VERSION=" + version,
			"SMAPI_SHA256=" + checksum,
			"SMAPI_ARCHIVE_BYTES=" + strconv.FormatInt(archiveBytes, 10),
			"SMAPI_MAX_ARCHIVE_BYTES=" + strconv.FormatInt(maxArchiveBytes, 10),
		},
		Binds: []string{
			resolvedGameDataVolumeName(r.instance.DataDir) + ":/data/game",
			archivePath + ":/package/smapi.zip:ro",
		},
	}
}

func isSteamGuardCodePrompt(lower string) bool {
	// The Steam Guard menu contains "[2] Enter code..." as an option. That line
	// is not an active code prompt; the default is still mobile-app approval.
	if strings.Contains(lower, "[2]") {
		return false
	}
	if strings.Contains(lower, "enter code from steam mobile") || strings.Contains(lower, "enter code from") {
		return true
	}
	return containsAny(lower, "enter steam guard code", "verification code")
}

func isSteamAuthMethodMenu(lower string) bool {
	return containsAny(lower, "choose authentication method", "username & password", "username and password") ||
		(strings.Contains(lower, "[2]") && strings.Contains(lower, "qr code"))
}

func isSteamAuthLoginSuccessLine(lower string) bool {
	if !strings.Contains(lower, "[steamauth:") && !strings.Contains(lower, "[steamservice]") {
		return false
	}
	return containsAny(lower,
		"logged in as",
		"login succeeded",
		"login successful",
		"successfully logged",
		"authentication complete",
		"auth done",
	)
}

func steamAuthAuthenticatedAccount(line string) (string, bool) {
	const marker = " authenticated as "
	lower := strings.ToLower(line)
	if !strings.Contains(lower, "[steamauth:") {
		return "", false
	}
	idx := strings.Index(lower, marker)
	if idx < 0 {
		return "", false
	}
	fields := strings.Fields(strings.TrimSpace(line[idx+len(marker):]))
	if len(fields) == 0 {
		return "", false
	}
	account := strings.Trim(fields[0], "\"'")
	return account, account != ""
}

func isSteamGuardChoiceMenu(lower string) bool {
	return strings.Contains(lower, "steam guard authentication") ||
		(strings.Contains(lower, "[1]") && containsAny(lower, "approve in steam mobile", "approve in steam")) ||
		(strings.Contains(lower, "[2]") && containsAny(lower, "enter code", "email", "code from"))
}

func isSteamMobileApprovalPrompt(lower string) bool {
	return containsAny(lower, "waiting for approval", "open steam app", "approve in steam mobile")
}

func isSteamCMDGuardChoiceMenu(lower string) bool {
	return strings.Contains(lower, "steam guard") &&
		(strings.Contains(lower, "[1]") || strings.Contains(lower, "1:")) &&
		(strings.Contains(lower, "[2]") || strings.Contains(lower, "2:")) &&
		containsAny(lower, "approve", "mobile", "email", "code")
}

func isSteamCMDGuardCodePrompt(lower string) bool {
	return containsAny(lower,
		"steam guard code",
		"two-factor code",
		"enter code from",
		"enter the current code",
		"enter verification code",
		"code sent to",
		"this computer has not been authenticated",
		"please check your email",
		"enter the steam guard",
		"code from that message",
		"set_steam_guard_code",
	)
}

func isSteamCMDMobileApprovalPrompt(lower string) bool {
	return containsAny(lower,
		"approve this login",
		"approve in steam mobile",
		"waiting for approval",
		"waiting for confirmation",
		"please confirm the login in the steam mobile app",
		"check your steam mobile app",
		"steam mobile app for a confirmation",
	)
}

func sanitizeSteamOutputLine(line, password string) string {
	line = paneldocker.RedactString(line)
	if password != "" {
		line = strings.ReplaceAll(line, password, "[REDACTED]")
	}
	return line
}

// containsAny returns true if s contains any of the given substrings.
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// truncate returns s truncated to at most n bytes.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// buildSteamAuthOpts reads the .env file and returns the container configuration
// needed by RunSteamAuthTTY. The compose project name (= lowercase dir basename)
// is used to derive the Docker named-volume identifiers.
func (r *installRunner) buildSteamAuthOpts(mode steamAuthMode) paneldocker.SteamAuthRunOpts {
	envVals, _ := sjconfig.ReadEnvFile(r.instance.DataDir + "/.env")
	steamAuthPort := envVals["STEAM_AUTH_PORT"]
	if steamAuthPort == "" {
		steamAuthPort = "3001"
	}
	// Docker Compose prefixes named volumes with the project name (= lowercase dir basename).
	projectName := strings.ToLower(filepath.Base(r.instance.DataDir))
	gameDir := "/data/game"
	binds := []string{projectName + "_steam-session:/data/steam-session"}
	if r.authOnly {
		// The authorization-only login process must not materialize or mutate the
		// persistent game-data volume. Give upstream a container-local scratch
		// directory in case it still initializes GAME_DIR during login.
		gameDir = "/tmp/anxi-steam-invite-game"
	} else {
		binds = append(binds, resolvedGameDataVolumeName(r.instance.DataDir)+":/data/game")
	}
	return paneldocker.SteamAuthRunOpts{
		ImageRef: steamServiceImageRef(envVals),
		Command:  steamAuthCommand(mode, r.authOnly),
		Env: []string{
			"PORT=" + steamAuthPort,
			"GAME_DIR=" + gameDir,
			"SESSION_DIR=/data/steam-session",
			"STEAM_USERNAME=" + r.username,
			"STEAM_PASSWORD=" + r.password,
			"STEAM_REFRESH_TOKEN=" + envVals["STEAM_REFRESH_TOKEN"],
			"STEAM_KEEP_LANGUAGES=" + envVals["STEAM_KEEP_LANGUAGES"],
			"STEAM_CLIENT_CONNECT_TIMEOUT_SECONDS=" + envWithDefault(envVals, "STEAM_CLIENT_CONNECT_TIMEOUT_SECONDS", DefaultSteamClientConnectTimeoutSeconds),
			"STEAM_CLIENT_CONNECT_RETRIES=" + envWithDefault(envVals, "STEAM_CLIENT_CONNECT_RETRIES", DefaultSteamClientConnectRetries),
			"STEAM_AUTH_SESSION_RETRIES=" + envWithDefault(envVals, "STEAM_AUTH_SESSION_RETRIES", DefaultSteamAuthSessionRetries),
			"STEAM_AUTH_SESSION_RETRY_DELAY_SECONDS=" + envWithDefault(envVals, "STEAM_AUTH_SESSION_RETRY_DELAY_SECONDS", DefaultSteamAuthSessionRetryDelaySeconds),
		},
		Binds: binds,
	}
}

func steamAuthCommand(mode steamAuthMode, authOnly bool) []string {
	if authOnly && mode == steamAuthModeCredentials {
		// "serve" performs non-interactive auto-login from the shared
		// STEAM_USERNAME/STEAM_PASSWORD environment without downloading either
		// app. runSteamAuthAttempt cancels it immediately after login is confirmed.
		return []string{"serve"}
	}
	if authOnly && mode == steamAuthModeQR {
		// "login" persists a QR/Guard session and exits without DownloadAll.
		// Upstream "setup" continues into game downloads and is therefore never
		// valid for the optional invite-only authorization flow.
		return []string{"login"}
	}
	if mode == steamAuthModeQR {
		return []string{"setup"}
	}
	return []string{"download"}
}

func ensureEnvDefault(updates, existing map[string]string, key, value string) {
	if strings.TrimSpace(existing[key]) == "" {
		updates[key] = value
	}
}

func envWithDefault(envVals map[string]string, key, fallback string) string {
	if value := strings.TrimSpace(envVals[key]); value != "" {
		return value
	}
	return fallback
}

func steamServiceImageRef(envVals map[string]string) string {
	return envWithDefault(envVals, "STEAM_SERVICE_IMAGE", DefaultSteamServiceImage)
}

func steamServiceImageRefs(envVals map[string]string) []string {
	return imageRefsFromEnv(envVals, "STEAM_SERVICE_IMAGE_CANDIDATES", DefaultSteamServiceImageCandidates, "STEAM_SERVICE_IMAGE", DefaultSteamServiceImage)
}

func serverImageDefault(imageTag string) string {
	tag := strings.TrimSpace(imageTag)
	if tag == "" {
		tag = TestedImageTag
	}
	return "sdvd/server:" + tag
}

func serverImageCandidatesDefault(imageTag string) string {
	tag := strings.TrimSpace(imageTag)
	if tag == "" {
		tag = TestedImageTag
	}
	return strings.Join([]string{
		"dockerproxy.net/sdvd/server:" + tag,
		"docker.1ms.run/sdvd/server:" + tag,
		"docker.1panel.live/sdvd/server:" + tag,
		"docker.jiaxin.site/sdvd/server:" + tag,
		"dockerproxy.link/sdvd/server:" + tag,
		"sdvd/server:" + tag,
	}, ",")
}

func serverImageRefs(envVals map[string]string, imageTag string) []string {
	tag := strings.TrimSpace(imageTag)
	if tag == "" {
		tag = TestedImageTag
	}
	refs := splitImageRefs(serverImageCandidatesDefault(tag))
	for _, ref := range splitImageRefs(envVals["SERVER_IMAGE_CANDIDATES"]) {
		if imageReferenceTag(ref) == tag {
			refs = appendUniqueString(refs, ref)
		}
	}
	if primary := strings.TrimSpace(envVals["SERVER_IMAGE"]); imageReferenceTag(primary) == tag {
		refs = appendUniqueString(refs, primary)
	}
	return refs
}

func imageReferenceTag(ref string) string {
	ref = strings.TrimSpace(ref)
	lastSlash, lastColon := strings.LastIndex(ref, "/"), strings.LastIndex(ref, ":")
	if lastColon <= lastSlash || lastColon == len(ref)-1 {
		return ""
	}
	return strings.TrimSpace(ref[lastColon+1:])
}

func steamCMDImageRef(envVals map[string]string) string {
	refs := steamCMDImageRefs(envVals)
	if len(refs) > 0 {
		return refs[0]
	}
	return DefaultSteamCMDImage
}

func steamCMDImageRefs(envVals map[string]string) []string {
	refs := steamCMDImageCandidateRefs(envVals["STEAMCMD_IMAGE_CANDIDATES"])
	refs = appendSteamCMDImageRef(refs, envWithDefault(envVals, "STEAMCMD_IMAGE", DefaultSteamCMDImage))
	if len(refs) == 0 {
		return []string{DefaultSteamCMDImage}
	}
	return refs
}

func steamCMDImageCandidatesValue(value string) string {
	return strings.Join(steamCMDImageCandidateRefs(value), ",")
}

func steamCMDImageCandidateRefs(value string) []string {
	defaultRefs := splitImageRefs(DefaultSteamCMDImageCandidates)
	refs := splitImageRefs(value)
	if len(refs) == 0 {
		return defaultRefs
	}
	ordered := make([]string, 0, len(defaultRefs)+len(refs))
	for _, ref := range defaultRefs {
		ordered = appendSteamCMDImageRef(ordered, ref)
	}
	for _, ref := range refs {
		ordered = appendSteamCMDImageRef(ordered, ref)
	}
	return ordered
}

func appendSteamCMDImageRef(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" ||
		value == "steamcmd/steamcmd:latest" ||
		value == "docker.xuanyuan.me/steamcmd/steamcmd:latest" ||
		value == "docker.m.daocloud.io/steamcmd/steamcmd:latest" {
		return values
	}
	return appendUniqueString(values, value)
}

func imageRefsFromEnv(envVals map[string]string, candidatesKey string, defaultCandidates string, primaryKey string, defaultPrimary string) []string {
	refs := splitImageRefs(defaultCandidates)
	for _, ref := range splitImageRefs(envVals[candidatesKey]) {
		refs = appendUniqueString(refs, ref)
	}
	refs = appendUniqueString(refs, envWithDefault(envVals, primaryKey, defaultPrimary))
	if len(refs) == 0 {
		return []string{defaultPrimary}
	}
	return refs
}

func splitImageRefs(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	})
	refs := make([]string, 0, len(fields))
	for _, field := range fields {
		refs = appendUniqueString(refs, strings.TrimSpace(field))
	}
	return refs
}

func appendUniqueString(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func fileMissing(path string) bool {
	_, err := os.Stat(path)
	return os.IsNotExist(err)
}

func verifyRequiredInstanceFiles(dataDir string) error {
	for _, name := range []string{"docker-compose.yml", ".env"} {
		path := filepath.Join(dataDir, name)
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("required instance file %s missing after prepare: %w", path, err)
		}
		if info.IsDir() {
			return fmt.Errorf("required instance file %s is a directory", path)
		}
	}
	return nil
}

func migrateOptionalSteamAuthComposeImage(path string, maintainSteamAuth bool) (bool, error) {
	if !maintainSteamAuth {
		return false, nil
	}
	return migrateSteamAuthComposeImage(path)
}

func migrateSteamAuthComposeImage(path string) (bool, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	text := string(raw)
	changed := false
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if strings.Contains(line, "image: sdvd/steam-service:") {
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			lines[i] = indent + "image: ${STEAM_SERVICE_IMAGE:-" + DefaultSteamServiceImage + "}"
			changed = true
		}
	}

	text = strings.Join(lines, "\n")
	for _, envLine := range []string{
		`      STEAM_CLIENT_CONNECT_TIMEOUT_SECONDS: "${STEAM_CLIENT_CONNECT_TIMEOUT_SECONDS:-` + DefaultSteamClientConnectTimeoutSeconds + `}"`,
		`      STEAM_CLIENT_CONNECT_RETRIES: "${STEAM_CLIENT_CONNECT_RETRIES:-` + DefaultSteamClientConnectRetries + `}"`,
		`      STEAM_AUTH_SESSION_RETRIES: "${STEAM_AUTH_SESSION_RETRIES:-` + DefaultSteamAuthSessionRetries + `}"`,
		`      STEAM_AUTH_SESSION_RETRY_DELAY_SECONDS: "${STEAM_AUTH_SESSION_RETRY_DELAY_SECONDS:-` + DefaultSteamAuthSessionRetryDelaySeconds + `}"`,
	} {
		if !strings.Contains(text, strings.TrimSpace(strings.SplitN(envLine, ":", 2)[0])+":") {
			inserted := insertLineAfter(text, `      STEAM_KEEP_LANGUAGES: "${STEAM_KEEP_LANGUAGES:-}"`, envLine)
			if inserted != text {
				text = inserted
				changed = true
			}
		}
	}

	if !changed {
		return false, nil
	}
	info, statErr := os.Stat(path)
	mode := os.FileMode(0o644)
	if statErr == nil {
		mode = info.Mode().Perm()
	}
	return true, os.WriteFile(path, []byte(text), mode)
}

func migrateServerComposeImage(path string) (bool, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	text := string(raw)
	changed := false
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "image: sdvd/server:") {
			continue
		}
		indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		lines[i] = indent + "image: ${SERVER_IMAGE:-" + DefaultServerImage + "}"
		changed = true
	}
	if !changed {
		return false, nil
	}
	text = strings.Join(lines, "\n")
	info, statErr := os.Stat(path)
	mode := os.FileMode(0o644)
	if statErr == nil {
		mode = info.Mode().Perm()
	}
	return true, os.WriteFile(path, []byte(text), mode)
}

// migrateSavesVolume rewrites docker-compose.yml to replace the `saves` named volume
// mount with a bind mount (./.local-container/saves) so the panel can access save files.
// Idempotent: returns (false, nil) when no change is needed.
func migrateSavesVolume(path string) (bool, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	const oldMount = "- saves:/config/xdg/config/StardewValley"
	const newMount = "- ./.local-container/saves:/config/xdg/config/StardewValley"
	text := string(raw)
	if !strings.Contains(text, oldMount) {
		return false, nil
	}
	text = strings.ReplaceAll(text, oldMount, newMount)
	// Remove the `saves:` entry from the top-level volumes section if present.
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		if strings.TrimSpace(l) == "saves:" {
			continue
		}
		out = append(out, l)
	}
	text = strings.Join(out, "\n")
	info, statErr := os.Stat(path)
	mode := os.FileMode(0o644)
	if statErr == nil {
		mode = info.Mode().Perm()
	}
	return true, os.WriteFile(path, []byte(text), mode)
}

func insertLineAfter(text, marker, line string) string {
	if !strings.Contains(text, marker) {
		return text
	}
	return strings.Replace(text, marker, marker+"\n"+line, 1)
}

// migrateRemoveAssetExporterService removes the old optional Compose service that
// launched a temporary game on the user's machine to create UI assets. Assets are
// now extracted once during panel development and packed in the frontend image.
func migrateRemoveAssetExporterService(path string) (bool, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	text := string(raw)
	const serviceStart = "\n  asset-exporter:\n"
	const volumesStart = "\nvolumes:\n"
	start := strings.Index(text, serviceStart)
	if start < 0 {
		return false, nil
	}
	relEnd := strings.Index(text[start:], volumesStart)
	if relEnd < 0 {
		return false, fmt.Errorf("asset-exporter block has no following volumes section")
	}
	end := start + relEnd
	updated := text[:start] + text[end:]
	info, statErr := os.Stat(path)
	mode := os.FileMode(0o644)
	if statErr == nil {
		mode = info.Mode().Perm()
	}
	return true, os.WriteFile(path, []byte(updated), mode)
}

// migrateAllowInsecureSetup sets ALLOW_INSECURE_SETUP=true in the instance .env when
// the server would otherwise refuse to start.  Junimo requires either API_KEY or
// ALLOW_INSECURE_SETUP=true; new panel instances don't set an API_KEY, so the only
// way to start is to allow insecure setup.
// Idempotent: returns (false, nil) if the value is already "true".
func migrateAllowInsecureSetup(envPath string) (bool, error) {
	vals, err := sjconfig.ReadEnvFile(envPath)
	if err != nil {
		return false, err
	}
	if strings.EqualFold(vals["ALLOW_INSECURE_SETUP"], "true") {
		return false, nil
	}
	return true, sjconfig.UpdateEnvFile(envPath, map[string]string{
		"ALLOW_INSECURE_SETUP": "true",
	})
}

// makePullLineHandler returns a stateful line handler for docker compose pull output.
// It writes each line to the job log with a "[pull] " prefix, emits a special
// "[pull:progress:X:Y]" log entry whenever the count changes, and calls onProgress.
func makePullLineHandler(jobCtx *jobs.Context, onProgress func(done, total int)) func(string) {
	var (
		mu       sync.Mutex
		total    int
		done     int
		lastEmit string
		useCount bool
	)

	return func(line string) {
		_, _ = jobCtx.Info(context.Background(), "[pull] "+line)

		mu.Lock()
		defer mu.Unlock()

		changed := false

		if m := regexPullCount.FindStringSubmatch(line); m != nil {
			d, _ := strconv.Atoi(m[1])
			tot, _ := strconv.Atoi(m[2])
			if tot > 0 {
				useCount = true
				done = d
				if tot > total {
					total = tot
				}
				changed = true
			}
		} else if !useCount {
			if regexPullStart.MatchString(line) {
				total++
				changed = true
			} else if regexPullDone.MatchString(line) {
				done++
				changed = true
			}
		}

		if changed && total > 0 {
			tag := fmt.Sprintf("[pull:progress:%d:%d]", done, total)
			if tag != lastEmit {
				lastEmit = tag
				_, _ = jobCtx.Info(context.Background(), tag)
				onProgress(done, total)
			}
		}
	}
}
