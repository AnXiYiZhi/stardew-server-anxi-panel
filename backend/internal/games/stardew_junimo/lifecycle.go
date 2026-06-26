package stardew_junimo

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

const (
	lifecycleJobTimeout    = 30 * time.Minute
	startServerWaitTimeout = 5 * time.Minute // Docker container reaches "running" within seconds; 5m is ample
	startCheckInterval     = 3 * time.Second
	startProgressInterval  = 30 * time.Second

	// readyStateTimeout covers the entire window from "container running" to "invite code obtained".
	// New-game world generation can take 15+ min, but the invite code may arrive earlier
	// (JunimoServer writes it as soon as the lobby is created, before save load completes).
	readyStateTimeout   = 20 * time.Minute
	readyInviteInterval = 15 * time.Second // how often to attempt attach-cli invitecode
	readyLogInterval    = 60 * time.Second // how often to tail container logs
	readySMAPIInterval  = 5 * time.Second  // how often to poll status.json

	inviteCodeTimeout = 30 * time.Second
)

// LifecycleDockerService extends DockerService with lifecycle operations.
type LifecycleDockerService interface {
	DockerService
	ComposeUp(ctx context.Context, dir string) (paneldocker.CommandResult, error)
	ComposeDown(ctx context.Context, dir string) (paneldocker.CommandResult, error)
	ComposeRestart(ctx context.Context, dir string) (paneldocker.CommandResult, error)
	ComposeExecPipe(ctx context.Context, dir, service, stdinData string, args ...string) (paneldocker.CommandResult, error)
	ComposeLogs(ctx context.Context, dir string, opts paneldocker.LogsOptions) (paneldocker.CommandResult, error)
}

// lifecycleRunner handles start/stop/restart job execution.
type lifecycleRunner struct {
	driver     *Driver
	lifecycle  LifecycleDockerService
	instance   storage.Instance
	operation  string // "start", "stop", "restart"
	actorID    int64
	newGame    bool // When true, send "settings newgame --confirm" after server starts.
}

// Start implements registry.GameDriver.Start.
// Creates an async job that runs docker compose up and retrieves the invite code.
func (d *Driver) Start(ctx context.Context, req registry.StartRequest) (*registry.Job, error) {
	if d.jobs == nil {
		return nil, fmt.Errorf("driver: job manager not configured")
	}
	ld, ok := d.docker.(LifecycleDockerService)
	if !ok {
		return nil, fmt.Errorf("docker 服务不支持生命周期操作")
	}
	instance, err := d.store.GetInstance(ctx, req.Instance.ID)
	if err != nil {
		return nil, fmt.Errorf("load instance: %w", err)
	}
	runner := &lifecycleRunner{
		driver:    d,
		lifecycle: ld,
		instance:  instance,
		operation: "start",
		actorID:   req.ActorID,
		newGame:   req.NewGame,
	}
	job, err := d.jobs.Start(ctx, jobs.Spec{
		Type:       "stardew_lifecycle",
		TargetType: "instance",
		TargetID:   req.Instance.ID,
		CreatedBy:  req.ActorID,
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
	ld, ok := d.docker.(LifecycleDockerService)
	if !ok {
		return fmt.Errorf("docker 服务不支持生命周期操作")
	}
	stored, err := d.store.GetInstance(ctx, instance.ID)
	if err != nil {
		return fmt.Errorf("load instance: %w", err)
	}
	runner := &lifecycleRunner{
		driver:    d,
		lifecycle: ld,
		instance:  stored,
		operation: "stop",
	}
	if _, err := d.jobs.Start(ctx, jobs.Spec{
		Type:       "stardew_lifecycle",
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
	ld, ok := d.docker.(LifecycleDockerService)
	if !ok {
		return fmt.Errorf("docker 服务不支持生命周期操作")
	}
	stored, err := d.store.GetInstance(ctx, instance.ID)
	if err != nil {
		return fmt.Errorf("load instance: %w", err)
	}
	runner := &lifecycleRunner{
		driver:    d,
		lifecycle: ld,
		instance:  stored,
		operation: "restart",
	}
	if _, err := d.jobs.Start(ctx, jobs.Spec{
		Type:       "stardew_lifecycle",
		TargetType: "instance",
		TargetID:   instance.ID,
		CreatedBy:  0,
		Timeout:    lifecycleJobTimeout,
		Run:        runner.run,
	}); err != nil {
		return fmt.Errorf("start restart job: %w", err)
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
	default:
		return fmt.Errorf("未知的生命周期操作: %s", r.operation)
	}
}

func (r *lifecycleRunner) doStart(ctx context.Context, jobCtx *jobs.Context) error {
	_, _ = jobCtx.Info(ctx, "正在启动 Stardew 服务器...")
	r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateStarting, "正在启动服务器...", "starting", jobCtx.ID)

	// Ensure the latest SMAPI mod DLL is deployed (idempotent; needed for IP direct-connect).
	if err := installSMAPIMod(r.instance.DataDir); err != nil {
		_, _ = jobCtx.Info(ctx, fmt.Sprintf("警告：SMAPI mod 部署失败（不影响启动）：%v", err))
	}

	result, err := r.lifecycle.ComposeUp(ctx, r.instance.DataDir)
	if err != nil {
		r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateStopped,
			"启动失败: "+result.Stderr, "start_failed", jobCtx.ID)
		return fmt.Errorf("docker compose up: %w", err)
	}
	_, _ = jobCtx.Info(ctx, "docker compose up 完成，等待服务器就绪...")

	if err := r.waitForServer(ctx, jobCtx); err != nil {
		r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateStopped,
			"服务器启动失败", "start_failed", jobCtx.ID)
		return err
	}

	// Container is running; poll for invite code and SMAPI status concurrently.
	// JunimoServer writes the invite code as soon as the lobby is created (before save load),
	// so we must not gate invite-code polling on SMAPI save-loaded.

	// If this is a new-game request, send "settings newgame --confirm" once SMAPI is ready.
	// This creates a fresh save using the server-settings.json values without deleting old saves.
	if r.newGame {
		if err := r.sendNewGameCommand(ctx, jobCtx); err != nil {
			_, _ = jobCtx.Warn(ctx, fmt.Sprintf("创建新存档失败（服务器将继续加载已有存档）：%v", err))
		}
	}

	r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateRunning,
		"服务器容器已启动，正在初始化游戏...", "server_initializing", jobCtx.ID)

	inviteCode := r.waitForReadyState(ctx, jobCtx)
	if inviteCode == "" {
		_, _ = jobCtx.Info(ctx, "未能获取邀请码，服务器可能仍在初始化，可在面板手动刷新邀请码。")
		r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateRunning,
			"服务器运行中（邀请码待就绪）", "running", jobCtx.ID)
	} else {
		msg := "服务器运行中，邀请码：" + inviteCode
		_, _ = jobCtx.Info(ctx, msg)
		r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateRunning,
			msg, "running", jobCtx.ID)
		r.driver.updateDriverPayloadInviteCode(ctx, r.instance.ID, inviteCode)
	}
	return nil
}

func (r *lifecycleRunner) doStop(ctx context.Context, jobCtx *jobs.Context) error {
	_, _ = jobCtx.Info(ctx, "正在停止 Stardew 服务器...")
	r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateStopped, "正在停止...", "stopping", jobCtx.ID)

	result, err := r.lifecycle.ComposeDown(ctx, r.instance.DataDir)
	if err != nil {
		r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateError,
			"停止失败: "+result.Stderr, "stop_failed", jobCtx.ID)
		return fmt.Errorf("docker compose down: %w", err)
	}
	r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateStopped, "服务器已停止", "stopped", jobCtx.ID)
	_, _ = jobCtx.Info(ctx, "服务器已停止")
	return nil
}

func (r *lifecycleRunner) doRestart(ctx context.Context, jobCtx *jobs.Context) error {
	_, _ = jobCtx.Info(ctx, "正在重启 Stardew 服务器...")
	r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateStarting, "正在重启...", "restarting", jobCtx.ID)

	result, err := r.lifecycle.ComposeRestart(ctx, r.instance.DataDir)
	if err != nil {
		r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateError,
			"重启失败: "+result.Stderr, "restart_failed", jobCtx.ID)
		return fmt.Errorf("docker compose restart: %w", err)
	}
	_, _ = jobCtx.Info(ctx, "重启完成，等待服务器就绪...")

	if err := r.waitForServer(ctx, jobCtx); err != nil {
		r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateError,
			"重启后服务器未就绪", "restart_timeout", jobCtx.ID)
		return err
	}

	inviteCode := r.waitForReadyState(ctx, jobCtx)
	if inviteCode == "" {
		_, _ = jobCtx.Info(ctx, "未能获取邀请码，可在面板手动刷新。")
		r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateRunning,
			"服务器重启完成（邀请码待就绪）", "running", jobCtx.ID)
	} else {
		msg := "服务器运行中，邀请码：" + inviteCode
		_, _ = jobCtx.Info(ctx, msg)
		r.driver.updatePhase(ctx, r.instance.ID, storage.InstanceStateRunning,
			msg, "running", jobCtx.ID)
		r.driver.updateDriverPayloadInviteCode(ctx, r.instance.ID, inviteCode)
	}
	return nil
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
	_, _ = jobCtx.Info(ctx, fmt.Sprintf("[server 容器日志 —最后 %d 行]\n%s", tail, result.Stdout))
}

// fetchInviteCode reads /tmp/invite-code.txt directly from the server container
// (written by JunimoServer as soon as the lobby is created), then falls back to
// attach-cli if the file is empty or the exec fails.
func (r *lifecycleRunner) fetchInviteCode(ctx context.Context) (string, error) {
	execCtx, cancel := context.WithTimeout(ctx, inviteCodeTimeout)
	defer cancel()

	// Primary: read the file JunimoServer writes directly — no parsing needed.
	catResult, catErr := r.lifecycle.ComposeExecPipe(execCtx, r.instance.DataDir, "server",
		"", "cat", "/tmp/invite-code.txt")
	if catErr == nil {
		if code := strings.TrimSpace(catResult.Stdout); code != "" {
			return code, nil
		}
	}

	// Fallback: ask attach-cli and parse its output.
	result, err := r.lifecycle.ComposeExecPipe(execCtx, r.instance.DataDir, "server",
		"invitecode\nquit\n", "attach-cli")
	combined := result.Stdout + result.Stderr
	if code := parseInviteCode(combined); code != "" {
		return code, nil
	}
	if err != nil {
		return "", fmt.Errorf("cat /tmp/invite-code.txt: %v; attach-cli: %w", catErr, err)
	}
	return "", fmt.Errorf("无法从 attach-cli 输出中解析邀请码，输出: %q", combined)
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
	runner := &lifecycleRunner{
		driver:    d,
		lifecycle: ld,
		instance:  stored,
	}
	return runner.fetchInviteCode(ctx)
}

// Galaxy P2P invite codes have no hyphens (e.g. "SGCWS0Z572F2"); Steam lobby codes use
// hyphenated groups (e.g. "ABCD-1234-EFGH"). Accept both.
var inviteCodePattern = regexp.MustCompile(`(?i)(?:invite\s*code[:\s]+|invitecode[:\s]+)([A-Z0-9]{8,}|[A-Z0-9]{4,}-[A-Z0-9]{4,}[A-Z0-9-]*)`)

// parseInviteCode extracts the invite code from attach-cli output.
// Returns empty string if not found.
func parseInviteCode(output string) string {
	if m := inviteCodePattern.FindStringSubmatch(output); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	// Fallback: bare code on its own line (hyphenated or 8+ char no-hyphen).
	standalone := regexp.MustCompile(`^([A-Z0-9]{8,}|[A-Z0-9]{4,}-[A-Z0-9]{4,}[A-Z0-9-]*)$`)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if m := standalone.FindStringSubmatch(line); len(m) > 1 {
			return m[1]
		}
	}
	return ""
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
func (r *lifecycleRunner) sendNewGameCommand(ctx context.Context, jobCtx *jobs.Context) error {
	_, _ = jobCtx.Info(ctx, "等待服务器 API 就绪后创建新存档...")

	// Poll the HTTP API until /status responds (server is up and accepting requests).
	apiURL := "http://localhost:8080/status"
	deadline := time.Now().Add(5 * time.Minute)
	for time.Now().Before(deadline) {
		reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		result, err := r.lifecycle.ComposeExecPipe(reqCtx, r.instance.DataDir, "server",
			"", "curl", "-sf", apiURL)
		cancel()
		if err == nil && result.ExitCode == 0 {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}

	_, _ = jobCtx.Info(ctx, "服务器 API 就绪，发送创建新存档请求...")

	// Call POST /newgame.  JunimoServer reads server-settings.json and creates a new save.
	// The gameloader config is updated automatically.
	cmdCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	result, err := r.lifecycle.ComposeExecPipe(cmdCtx, r.instance.DataDir, "server",
		"", "curl", "-sf", "-X", "POST", "-H", "Content-Type: application/json", "-d", "{}",
		"http://localhost:8080/newgame")
	if err != nil {
		return fmt.Errorf("POST /newgame: %w", err)
	}
	_, _ = jobCtx.Info(ctx, fmt.Sprintf("新存档创建响应：%s", strings.TrimSpace(result.Stdout)))

	// Wait for the new save to appear in the Saves directory.
	saveDeadline := time.Now().Add(5 * time.Minute)
	for time.Now().Before(saveDeadline) {
		gameloaderPath := filepath.Join(savesDir(r.instance.DataDir), ".smapi", "mod-data",
			"junimohost.server", "junimohost.gameloader.json")
		data, err := os.ReadFile(gameloaderPath)
		if err == nil {
			var gl struct {
				SaveNameToLoad string `json:"SaveNameToLoad"`
			}
			if json.Unmarshal(data, &gl) == nil && gl.SaveNameToLoad != "" {
				saveDir := filepath.Join(savesDir(r.instance.DataDir), "Saves", gl.SaveNameToLoad)
				if _, err := os.Stat(saveDir); err == nil {
					_, _ = jobCtx.Info(ctx, fmt.Sprintf("新存档已创建：%s", gl.SaveNameToLoad))
					return nil
				}
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}

	_, _ = jobCtx.Warn(ctx, "新存档请求已发送但未检测到新存档目录，请检查服务器日志。")
	return nil
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

func jsonUnmarshal(s string, v any) error {
	return json.Unmarshal([]byte(s), v)
}
