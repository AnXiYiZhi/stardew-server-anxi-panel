package stardew_junimo

import (
	"context"
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
	regexPullCount = regexp.MustCompile(`(?i)pulling (\d+)/(\d+)`)
	regexPullStart = regexp.MustCompile(`(?i)(^image \S+\s+pulling\s*$|^pulling \S+ \()`)
	regexPullDone  = regexp.MustCompile(`(?i)(^image \S+\s+pulled\s*$|^status: (downloaded newer image|image is up to date))`)
)

// installRunner carries everything needed to execute one install job.
type installRunner struct {
	driver   *Driver
	instance storage.Instance
	username string
	password string // never logged
	vncPass  string // never logged
	imageTag string
	autoMode bool
}

type steamAuthMode string

const (
	steamAuthModeCredentials steamAuthMode = "credentials"
	steamAuthModeQR          steamAuthMode = "qr"
)

// run is the job.Runner function executed by jobs.Manager in a goroutine.
func (r *installRunner) run(ctx context.Context, jobCtx *jobs.Context) error {
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
	updates := map[string]string{
		"IMAGE_VERSION":  r.imageTag,
		"STEAM_USERNAME": r.username,
		"STEAM_PASSWORD": r.password,
		"VNC_PASSWORD":   r.vncPass,
	}
	ensureEnvDefault(updates, envVals, "STEAM_SERVICE_IMAGE", DefaultSteamServiceImage)
	ensureEnvDefault(updates, envVals, "STEAM_CLIENT_CONNECT_TIMEOUT_SECONDS", DefaultSteamClientConnectTimeoutSeconds)
	ensureEnvDefault(updates, envVals, "STEAM_CLIENT_CONNECT_RETRIES", DefaultSteamClientConnectRetries)
	ensureEnvDefault(updates, envVals, "STEAM_AUTH_SESSION_RETRIES", DefaultSteamAuthSessionRetries)
	ensureEnvDefault(updates, envVals, "STEAM_AUTH_SESSION_RETRY_DELAY_SECONDS", DefaultSteamAuthSessionRetryDelaySeconds)
	if err := sjconfig.UpdateEnvFile(envPath, updates); err != nil {
		return fmt.Errorf("write .env: %w", err)
	}
	_, _ = jobCtx.Info(ctx, ".env 写入完成（密码已脱敏，不写入日志）。")
	if changed, err := migrateAllowInsecureSetup(envPath); err != nil {
		return fmt.Errorf("update ALLOW_INSECURE_SETUP in .env: %w", err)
	} else if changed {
		_, _ = jobCtx.Info(ctx, "已将 .env 中的 ALLOW_INSECURE_SETUP 设为 true（Junimo 需要此配置才能在无 API_KEY 时启动）。")
	}
	if changed, err := migrateSteamAuthComposeImage(filepath.Join(r.instance.DataDir, "docker-compose.yml")); err != nil {
		return fmt.Errorf("update steam-auth image in docker-compose.yml: %w", err)
	} else if changed {
		_, _ = jobCtx.Info(ctx, "已更新 docker-compose.yml 中的 steam-auth 镜像配置。")
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
	// ── Step 2: docker compose pull ─────────────────────────────────────
	r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateSteamAuthRunning,
		"正在检查 Junimo 镜像，请稍候...", "pull_running", jobCtx.ID)

	missing := r.missingServices(ctx, jobCtx)
	if len(missing) == 0 {
		_, _ = jobCtx.Info(ctx, "本地已存在所有 Junimo 镜像，跳过 docker compose pull。")
	} else {
		_, _ = jobCtx.Info(ctx, fmt.Sprintf("需要拉取 %d 个服务的镜像：%s", len(missing), strings.Join(missing, ", ")))
		pullResult, err := r.driver.docker.ComposePullStreaming(ctx, r.instance.DataDir, missing,
			makePullLineHandler(jobCtx, func(done, total int) {
				var msg string
				if done == total && total > 0 {
					msg = fmt.Sprintf("所有 %d 个镜像已拉取完成，准备启动 Steam 认证...", total)
				} else {
					msg = fmt.Sprintf("正在拉取 Junimo 镜像（%d/%d 完成），首次下载可能需要 10-30 分钟，请耐心等待...", done, total)
				}
				r.driver.updatePhase(context.Background(), r.instance.ID,
					storage.InstanceStateSteamAuthRunning, msg, "pull_running", jobCtx.ID)
			}),
		)
		if err != nil {
			if ctx.Err() != nil {
				r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateError,
					"安装任务超时，Steam 认证未完成，请重试安装。", "install_timeout", jobCtx.ID)
				return fmt.Errorf("install timed out: %w", ctx.Err())
			}
			// Use junimo_scaffolded (not error) so the user can click Install again to retry.
			r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateJunimoScaffolded,
				"docker compose pull 失败，请检查日志后重新安装。", "pull_failed", jobCtx.ID)
			return fmt.Errorf("docker compose pull failed (exit %d)", pullResult.ExitCode)
		}
	}
	_, _ = jobCtx.Info(ctx, "镜像检查完成，等待选择 Steam 登录方式...")

	// ── Step 3: steam-auth setup/download ────────────────────────────────
	if err := r.runSteamAuth(ctx, jobCtx); err != nil {
		return err
	}

	return nil
}

// missingServices returns the compose service names whose images are not present locally.
// It logs a line per image so the job log shows exactly what was found.
func (r *installRunner) missingServices(ctx context.Context, jobCtx *jobs.Context) []string {
	type serviceImage struct {
		service string
		image   string
	}
	envVals, _ := sjconfig.ReadEnvFile(filepath.Join(r.instance.DataDir, ".env"))
	all := []serviceImage{
		{"steam-auth", steamServiceImageRef(envVals)},
		{"server", "sdvd/server:" + r.imageTag},
	}
	var missing []string
	for _, si := range all {
		if _, err := r.driver.docker.ImageInspect(ctx, r.instance.DataDir, si.image); err != nil {
			_, _ = jobCtx.Info(ctx, fmt.Sprintf("[check] %s 镜像缺失，需要拉取。", si.image))
			missing = append(missing, si.service)
		} else {
			_, _ = jobCtx.Info(ctx, fmt.Sprintf("[check] %s 镜像已存在，跳过。", si.image))
		}
	}
	return missing
}

func (r *installRunner) runSteamAuth(ctx context.Context, jobCtx *jobs.Context) error {
	guardCh := make(chan string, 8)
	r.driver.setGuardChan(jobCtx.ID, guardCh)
	defer func() {
		r.driver.clearGuardChan(jobCtx.ID)
	}()

	mode := steamAuthModeCredentials
	if r.autoMode {
		_, _ = jobCtx.Info(ctx, "复用已保存的 Steam 凭据，直接校验并下载游戏文件。")
		r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateSteamAuthRunning,
			"正在复用已保存的 Steam 凭据校验已有文件并继续下载...", "game_downloading", jobCtx.ID)
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
			r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateSteamAuthRunning,
				"Steam 连接建立较慢，正在自动重试认证...", "steam_auth_retrying", jobCtx.ID)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateError,
					"安装任务超时，Steam 认证未完成，请重试安装。", "install_timeout", jobCtx.ID)
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
	r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateSteamAuthRunning,
		"请选择 Steam 登录方式。", "auth_method_required", jobCtx.ID)
	_, _ = jobCtx.Info(ctx, "等待管理员选择 Steam 登录方式：扫码登录，或账号密码/验证码登录。")

	for {
		select {
		case input := <-guardCh:
			switch strings.TrimSpace(input) {
			case "1":
				_, _ = jobCtx.Info(ctx, "已选择账号密码/验证码登录。")
				r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateSteamAuthRunning,
					"正在使用已保存的 Steam 凭据认证并下载游戏，请稍候...", "steam_auth_running", jobCtx.ID)
				return steamAuthModeCredentials, nil
			case "2":
				_, _ = jobCtx.Info(ctx, "已选择扫码登录。")
				r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateSteamAuthRunning,
					"正在启动 Steam 扫码登录，请等待二维码输出。", "steam_qr_required", jobCtx.ID)
				return steamAuthModeQR, nil
			default:
				_, _ = jobCtx.Warn(ctx, "收到无效的 Steam 登录方式选择，仍在等待 1 或 2。")
			}
		case <-ctx.Done():
			r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateError,
				"安装任务超时，Steam 认证未完成，请重试安装。", "install_timeout", jobCtx.ID)
			return "", fmt.Errorf("steam auth method selection timed out: %w", ctx.Err())
		}
	}
}

func (r *installRunner) runSteamAuthAttempt(ctx context.Context, jobCtx *jobs.Context, guardCh chan string, mode steamAuthMode, attempt, maxAttempts int) (bool, error) {
	var (
		outputMu         sync.Mutex
		authSucceeded    bool
		authFailed       bool
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

		switch {
		case containsAny(lower, "tryanothercm", "steamclient instance must be connected", "asyncjobfailedexception"):
			// TryAnotherCM and AsyncJobFailedException are Steam network/CM errors,
			// not credential failures. Both are retried automatically.
			connectionFailed = true
			authFailed = true

		case strings.Contains(lower, "qr authentication failed"):
			qrAuthFailed = true
			authFailed = true

		case containsAny(lower, "invalid password", "incorrect password", "wrong password", "bad password"):
			credentialFailed = true
			authFailed = true

		case containsAny(lower, "unhandled exception", "cannot read keys"):
			authFailed = true

		case isSteamAuthMethodMenu(lower):
			if mode == steamAuthModeQR {
				r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateSteamAuthRunning,
					"请使用 Steam 手机 App 扫描日志中显示的二维码。", "steam_qr_required", jobCtx.ID)
			} else {
				r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateSteamAuthRunning,
					"请选择 Steam 登录方式。", "auth_method_required", jobCtx.ID)
			}

		case isSteamGuardChoiceMenu(lower):
			guardChoiceShown = true
			r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateSteamAuthRunning,
				"Steam 需要二步验证，请在面板选择 Steam Guard 方式。",
				"steam_guard_choice_required", jobCtx.ID)

		case containsAny(lower, "waiting for approval", "open steam app", "choice [1]", "approve in steam mobile"):
			// After option 1 is selected (by user or auto), Steam waits for mobile approval.
			mobileApproval = true
			r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateSteamAuthRunning,
				"请打开 Steam 手机 App，批准此次登录请求。",
				"steam_guard_mobile_required", jobCtx.ID)

		case isSteamGuardCodePrompt(lower):
			r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateSteamAuthRunning,
				"Steam Guard 验证码已请求，请在面板输入验证码。",
				"steam_guard_required", jobCtx.ID)

		case containsAny(lower, "steam guard", "two-factor", "authenticator"):
			// Don't overwrite steam_guard_choice_required while the user is choosing.
			// The explicit "waiting for approval" / "choice [1]" case below handles
			// the transition once an actual selection is made.
			if !mobileApproval && !guardChoiceShown {
				r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateSteamAuthRunning,
					"请打开 Steam 手机 App，批准此次登录请求；如果你选择输入验证码，面板会再显示验证码输入框。",
					"steam_guard_mobile_required", jobCtx.ID)
			}

		case containsAny(lower, "qr code", "scan the qr", "scan this qr") && !strings.Contains(lower, "[2]"):
			r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateSteamAuthRunning,
				"请使用 Steam 手机 App 扫描日志中显示的二维码。",
				"steam_qr_required", jobCtx.ID)

		case containsAny(lower, "approve this", "confirm") && strings.Contains(lower, "steam"):
			mobileApproval = true
			r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateSteamAuthRunning,
				"请在 Steam 手机 App 上批准此次登录请求。",
				"steam_guard_mobile_required", jobCtx.ID)

		case containsAny(lower, "download complete", "app installed to"):
			if currentApp == "sdk" {
				sdkDownloaded = true
				authSucceeded = true
			}

		case strings.Contains(lower, "downloading app 1007") ||
			(strings.Contains(lower, "target directory:") && strings.Contains(lower, ".steam-sdk")):
			// Steamworks SDK Redistributable is downloaded after the Stardew depot.
			// Keep it as a separate phase so the frontend does not appear stuck at
			// 100% game download while SDK files are still being fetched.
			currentApp = "sdk"
			r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateSteamAuthRunning,
				"游戏文件下载完成，正在下载 Steam SDK 运行文件...",
				"steam_sdk_downloading", jobCtx.ID)

		case containsAny(lower, "downloading app"):
			// Auth succeeded; game depot download is starting. Update phase so the
			// frontend shows progress instead of the stale Steam Guard phase.
			currentApp = "game"
			r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateSteamAuthRunning,
				"Steam 认证成功，正在下载游戏文件，请耐心等待...",
				"game_downloading", jobCtx.ID)

		case containsAny(lower, "game download failed", "download failed:"):
			// Auth may have succeeded but the depot download failed (CDN error, low disk
			// space, etc.). Flag it so we report failure instead of false success.
			downloadFailed = true

		case containsAny(lower, "logged in", "login succeeded", "auth done",
			"authentication complete", "successfully logged", "login successful"):
			authSucceeded = true

		case containsAny(lower, "login failed", "authentication failed", "no accounts logged in", "skipping game download"):
			authFailed = true
		}
	}

	if mode == steamAuthModeQR {
		// The panel already collected the user's QR choice. The upstream setup
		// command still prints its own first menu, so pre-buffer "2" for it.
		select {
		case guardCh <- "2\n":
		default:
			_, _ = jobCtx.Warn(ctx, "Steam 输入队列已满，无法自动选择扫码登录。")
		}
	}

	// RunSteamAuthTTY creates the container via Docker API with Tty:true so
	// Console.ReadKey() works for the Steam Guard method selection menu.
	exitCode, cmdErr := r.driver.docker.RunSteamAuthTTY(ctx, r.instance.DataDir, r.buildSteamAuthOpts(mode), guardCh, lineHandler)
	if cmdErr != nil {
		if sdkDownloaded {
			r.markInstallSucceeded(jobCtx)
			return false, nil
		}
		if ctx.Err() != nil {
			r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateError,
				"安装任务超时，Steam 认证未完成，请重试安装。", "install_timeout", jobCtx.ID)
			return false, fmt.Errorf("steam-auth timed out: %w", ctx.Err())
		}
		return false, fmt.Errorf("steam-auth run error: %w", cmdErr)
	}

	if ctx.Err() != nil {
		if sdkDownloaded {
			r.markInstallSucceeded(jobCtx)
			return false, nil
		}
		r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateError,
			"安装任务超时，Steam 认证未完成，请重试安装。", "install_timeout", jobCtx.ID)
		return false, fmt.Errorf("steam-auth timed out: %w", ctx.Err())
	}

	if connectionFailed && !authSucceeded {
		if attempt < maxAttempts {
			return true, fmt.Errorf("steam client not connected")
		}
		r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateSteamAuthFailed,
			"Steam 连接建立超时，请检查网络后重试安装。", "steam_auth_connection_failed", jobCtx.ID)
		_, _ = jobCtx.Error(context.Background(), "Steam 认证失败：SteamClient 多次未能在上游等待时间内完成连接。")
		return false, fmt.Errorf("steam client not connected after %d attempts", maxAttempts)
	}

	if downloadFailed {
		r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateSteamAuthFailed,
			"游戏文件下载失败，请检查网络和磁盘空间后重试安装。", "download_failed", jobCtx.ID)
		_, _ = jobCtx.Error(context.Background(), "Steam 认证成功，但游戏文件下载失败，请检查任务日志中的具体错误。")
		return false, fmt.Errorf("game download failed after successful auth")
	}

	if qrAuthFailed {
		r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateSteamAuthFailed,
			"二维码登录失败，请改用账号密码或 Steam Guard 后重试。", "qr_auth_failed", jobCtx.ID)
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
		if exitCode == 139 {
			phase = "steam_auth_console_failed"
			message = "Steam 认证因上游交互控制台不可用而失败，请更新后重试。"
			errorLog = "Steam 认证失败：上游交互式密码读取需要真实控制台。"
		}
		r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateSteamAuthFailed,
			message, phase, jobCtx.ID)
		_, _ = jobCtx.Error(context.Background(), errorLog)
		return false, fmt.Errorf("steam authentication failed")
	}

	if authSucceeded {
		r.markInstallSucceeded(jobCtx)
		return false, nil
	}

	r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateSteamAuthFailed,
		"Steam 认证失败，请检查日志并重试。", "credentials_required", jobCtx.ID)
	_, _ = jobCtx.Error(context.Background(), fmt.Sprintf("steam-auth 以退出码 %d 结束。", exitCode))
	return false, fmt.Errorf("steam-auth download exited with code %d", exitCode)
}

func (r *installRunner) markInstallSucceeded(jobCtx *jobs.Context) {
	r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateGameInstalled,
		"游戏已安装。", "game_installed", jobCtx.ID)
	_, _ = jobCtx.Info(context.Background(), "安装流程全部完成。")
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

func isSteamGuardChoiceMenu(lower string) bool {
	return strings.Contains(lower, "steam guard authentication") ||
		(strings.Contains(lower, "[1]") && containsAny(lower, "approve in steam mobile", "approve in steam")) ||
		(strings.Contains(lower, "[2]") && containsAny(lower, "enter code", "email", "code from"))
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
	return paneldocker.SteamAuthRunOpts{
		ImageRef: steamServiceImageRef(envVals),
		Command:  steamAuthCommand(mode),
		Env: []string{
			"PORT=" + steamAuthPort,
			"GAME_DIR=/data/game",
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
		Binds: []string{
			projectName + "_steam-session:/data/steam-session",
			projectName + "_game-data:/data/game",
		},
	}
}

func steamAuthCommand(mode steamAuthMode) []string {
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
