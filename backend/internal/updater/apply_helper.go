package updater

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type ApplyExecutor interface {
	Run(context.Context, ...string) error
	Output(context.Context, ...string) (string, error)
}

func (e ExecDocker) Output(ctx context.Context, args ...string) (string, error) {
	path := e.Path
	if path == "" {
		path = "docker"
	}
	commandCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	output, err := execCommandOutput(commandCtx, path, args...)
	if err != nil {
		if commandCtx.Err() != nil {
			return "", errors.New("docker command timed out")
		}
		return "", errors.New("docker command failed")
	}
	return strings.TrimSpace(output), nil
}

var execCommandOutput = func(ctx context.Context, path string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, path, args...)
	var stdout limitedStringBuffer
	stdout.limit = 64 << 10
	cmd.Stdout = &stdout
	cmd.Stderr = &limitedDiscard{limit: 64 << 10}
	err := cmd.Run()
	return stdout.String(), err
}

type limitedStringBuffer struct {
	data  []byte
	limit int
}

func (b *limitedStringBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if b.limit > len(b.data) {
		remaining := b.limit - len(b.data)
		if len(p) > remaining {
			p = p[:remaining]
		}
		b.data = append(b.data, p...)
	}
	return n, nil
}
func (b *limitedStringBuffer) String() string { return string(b.data) }

type ApplyOptions struct {
	FromVersion      string
	TargetVersion    string
	CurrentImage     string
	OriginalDigest   string
	CurrentContainer string
	ComposeProject   string
	ComposeFile      string
	StateFile        string
	BackupDir        string
	DatabaseRelative string
	HealthTimeout    time.Duration
	PollInterval     time.Duration
	Executor         ApplyExecutor
	Now              func() time.Time
	DataDir          string
}

type applyManifest struct {
	FromVersion, ToVersion        string
	OriginalImage, OriginalDigest string
	ComposeProject, ComposeFile   string
	StartedAt                     time.Time
}

type applyFailure struct{ code, message string }

func (e applyFailure) Error() string { return e.message }

func RunApply(ctx context.Context, opts ApplyOptions) error {
	executor := opts.Executor
	if executor == nil {
		executor = ExecDocker{}
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	if opts.HealthTimeout <= 0 {
		opts.HealthTimeout = 120 * time.Second
	}
	if opts.PollInterval <= 0 {
		opts.PollInterval = 2 * time.Second
	}
	if opts.DataDir == "" {
		opts.DataDir = "/data"
	}
	store := NewApplyStateStore(opts.StateFile)
	status, err := store.Read()
	if err != nil {
		return err
	}
	setPhase := func(phase string, progress int, message string) error {
		status.Phase, status.Progress, status.UpdatedAt = phase, progress, now().UTC()
		if message != "" {
			status.Logs = append(status.Logs, LogEntry{At: status.UpdatedAt, Level: "info", Message: message})
		}
		return store.Write(status)
	}
	failTerminal := func(phase, code, message, result string) error {
		finished := now().UTC()
		status.Phase, status.Progress, status.ErrorCode, status.Error, status.Result = phase, 100, code, message, result
		status.UpdatedAt, status.FinishedAt = finished, &finished
		status.Logs = append(status.Logs, LogEntry{At: finished, Level: "error", Message: message})
		if err := store.Write(status); err != nil {
			return err
		}
		return errors.New(message)
	}
	from, err := NormalizeTargetVersion(opts.FromVersion)
	if err != nil {
		return failTerminal(PhaseRollbackFailed, CodeInvalidTargetVersion, "原版本不合法", "helper 参数校验失败")
	}
	to, err := NormalizeTargetVersion(opts.TargetVersion)
	if err != nil {
		return failTerminal(PhaseRollbackFailed, CodeInvalidTargetVersion, "目标版本不合法", "helper 参数校验失败")
	}
	if cmp, _ := CompareStableVersions(from, to); cmp >= 0 {
		return failTerminal(PhaseRollbackFailed, CodeUpdateNotAvailable, "目标版本必须高于原版本", "helper 参数校验失败")
	}
	if !containerReferencePattern.MatchString(opts.CurrentContainer) || !composeProjectPattern.MatchString(opts.ComposeProject) {
		return failTerminal(PhaseRollbackFailed, CodeComposeMetadataInvalid, "容器或 Compose 项目标识不合法", "helper 参数校验失败")
	}
	if err := secureApplyPaths(opts); err != nil {
		return failTerminal(PhaseRollbackFailed, CodeComposeMetadataInvalid, "升级路径参数不合法", "helper 参数校验失败")
	}

	backupCompose := filepath.Join(opts.BackupDir, "docker-compose.yml")
	backupEnv := filepath.Join(opts.BackupDir, "deployment.env")
	envFile := filepath.Join(filepath.Dir(opts.ComposeFile), ".env")
	if err := os.MkdirAll(opts.BackupDir, 0o700); err != nil {
		return failTerminal(PhaseFailedRolledBack, "deployment_backup_failed", "无法创建部署备份目录", "当前面板未改变")
	}
	_ = os.Chmod(opts.BackupDir, 0o700)
	if err := copyFileAtomic(opts.ComposeFile, backupCompose, 0o600); err != nil {
		return failTerminal(PhaseFailedRolledBack, "deployment_backup_failed", "Compose 文件备份失败", "当前面板未改变")
	}
	if err := copyFileAtomic(envFile, backupEnv, 0o600); err != nil {
		return failTerminal(PhaseFailedRolledBack, "deployment_backup_failed", "部署环境文件备份失败", "当前面板未改变")
	}
	manifest := applyManifest{FromVersion: from, ToVersion: to, OriginalImage: opts.CurrentImage, OriginalDigest: opts.OriginalDigest, ComposeProject: opts.ComposeProject, ComposeFile: filepath.Base(opts.ComposeFile), StartedAt: status.StartedAt}
	if err := writeJSONAtomic(filepath.Join(opts.BackupDir, "manifest.json"), manifest, 0o600); err != nil {
		return failTerminal(PhaseFailedRolledBack, "deployment_backup_failed", "升级清单备份失败", "当前面板未改变")
	}

	recreateAttempted := false
	selected := ""
	selectedDigest := ""
	operationErr := func() error {
		if err := setPhase(PhasePulling, 30, "正在拉取精确版本的可信面板镜像"); err != nil {
			return applyFailure{"state_write_failed", "无法更新升级状态"}
		}
		candidates, err := TrustedImageCandidates(to, opts.CurrentImage)
		if err != nil {
			return applyFailure{CodeImageNotAllowed, "无法生成可信镜像候选"}
		}
		for _, candidate := range candidates {
			if err := ValidateTrustedImage(candidate); err != nil {
				return applyFailure{CodeImageNotAllowed, "镜像候选未通过白名单"}
			}
			pullCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
			err := executor.Run(pullCtx, "pull", candidate)
			cancel()
			if err != nil {
				continue
			}
			digestCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			digest, digestErr := executor.Output(digestCtx, "image", "inspect", "--format", "{{.Id}}", candidate)
			cancel()
			if digestErr != nil || !strings.Contains(digest, "sha256:") {
				continue
			}
			selected, selectedDigest = candidate, digest
			break
		}
		if selected == "" {
			return applyFailure{CodeImagePullFailed, "所有可信目标镜像均拉取失败"}
		}
		status.SelectedImage, status.SelectedDigest = selected, selectedDigest
		if err := store.Write(status); err != nil {
			return applyFailure{"state_write_failed", "无法记录目标镜像"}
		}
		if err := updateEnvImage(envFile, selected); err != nil {
			return applyFailure{"deployment_update_failed", "无法更新部署镜像配置"}
		}
		composeArgs := composeBaseArgs(opts.ComposeProject, opts.ComposeFile, envFile)
		configCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		resolved, configErr := executor.Output(configCtx, append(composeArgs, "config", "--images", "panel")...)
		cancel()
		if configErr != nil || strings.TrimSpace(resolved) != selected {
			return applyFailure{"deployment_update_failed", "Compose 配置未精确解析到目标镜像"}
		}
		if err := setPhase(PhaseRecreating, 55, "正在由独立 helper 重建 panel 服务"); err != nil {
			return applyFailure{"state_write_failed", "无法更新升级状态"}
		}
		recreateAttempted = true
		recreateCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		err = executor.Run(recreateCtx, append(composeArgs, "up", "-d", "--pull", "always", "--force-recreate", "--no-deps", "panel")...)
		cancel()
		if err != nil {
			return applyFailure{CodeComposeRecreateFailed, "panel 服务重建失败"}
		}
		if err := setPhase(PhaseWaitingHealth, 75, "正在验收新 panel 的容器健康、HTTP 健康和版本"); err != nil {
			return applyFailure{"state_write_failed", "无法更新升级状态"}
		}
		if err := waitForPanel(ctx, executor, opts.CurrentContainer, to, opts.HealthTimeout, opts.PollInterval); err != nil {
			return err
		}
		return nil
	}()
	if operationErr == nil {
		cleanupPanelImages(ctx, executor, opts.CurrentImage, opts.OriginalDigest, selected, selectedDigest, &status)
		status.CleanupCompleted = true
		finished := now().UTC()
		status.Phase, status.Progress, status.Result = PhaseSucceeded, 100, "面板升级并验收成功"
		status.UpdatedAt, status.FinishedAt, status.Error, status.ErrorCode = finished, &finished, "", ""
		status.Logs = append(status.Logs, LogEntry{At: finished, Level: "info", Message: "新面板三项健康验收全部通过"})
		return store.Write(status)
	}
	failure := applyFailure{"update_failed", "面板升级失败"}
	if errors.As(operationErr, &failure) { /* structured */
	}
	if err := setPhase(PhaseRollingBack, 85, "升级未通过，正在自动恢复旧面板"); err != nil {
		return err
	}
	rollbackErr := rollbackApply(ctx, executor, opts, backupCompose, backupEnv, recreateAttempted)
	if rollbackErr != nil {
		return failTerminal(PhaseRollbackFailed, CodeRollbackFailed, failure.message+"；自动回滚失败", "需要管理员人工检查部署和数据库备份")
	}
	return failTerminal(PhaseFailedRolledBack, failure.code, failure.message, "已自动恢复并验收旧面板")
}

func cleanupPanelImages(ctx context.Context, executor ApplyExecutor, originalImage, originalDigest, selectedImage, selectedDigest string, status *ApplyStatus) {
	if strings.TrimSpace(originalImage) != "" && originalImage != selectedImage && strings.TrimSpace(originalDigest) != strings.TrimSpace(selectedDigest) {
		inspectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		actualDigest, err := executor.Output(inspectCtx, "image", "inspect", "--format", "{{.Id}}", originalImage)
		cancel()
		if err != nil {
			status.Logs = append(status.Logs, LogEntry{At: time.Now().UTC(), Level: "warn", Message: "旧面板镜像已不存在或无法核对，跳过旧 tag 清理"})
		} else if strings.TrimSpace(actualDigest) != strings.TrimSpace(originalDigest) {
			status.Logs = append(status.Logs, LogEntry{At: time.Now().UTC(), Level: "warn", Message: "旧面板镜像 tag 已指向其他 image ID，出于安全考虑未删除"})
		} else {
			removeCtx, removeCancel := context.WithTimeout(ctx, 2*time.Minute)
			removeErr := executor.Run(removeCtx, "image", "rm", originalImage)
			removeCancel()
			if removeErr != nil {
				status.Logs = append(status.Logs, LogEntry{At: time.Now().UTC(), Level: "warn", Message: "旧面板镜像仍被其他容器引用或删除失败，已保留供管理员检查"})
			} else {
				status.Logs = append(status.Logs, LogEntry{At: time.Now().UTC(), Level: "info", Message: "已删除升级前的旧面板镜像引用"})
			}
		}
	}
	cleanupTrustedPanelImageHistory(ctx, executor, selectedImage, selectedDigest, status)

	// Only prune dangling images produced by this project's build. Never use
	// `image prune -a`, which could remove rollback or unrelated images.
	pruneCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	err := executor.Run(pruneCtx, "image", "prune", "--force", "--filter", "label=org.opencontainers.image.title=stardew-server-anxi-panel")
	cancel()
	if err != nil {
		status.Logs = append(status.Logs, LogEntry{At: time.Now().UTC(), Level: "warn", Message: "面板 dangling 镜像定向清理失败；升级结果不受影响"})
	}
}

func cleanupTrustedPanelImageHistory(ctx context.Context, executor ApplyExecutor, selectedImage, selectedDigest string, status *ApplyStatus) {
	listCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	images, err := executor.Output(listCtx, "image", "ls", "--no-trunc", "--filter", "label=org.opencontainers.image.title=stardew-server-anxi-panel", "--format", "{{.Repository}}|{{.Tag}}|{{.ID}}")
	cancel()
	if err != nil {
		status.Logs = append(status.Logs, LogEntry{At: time.Now().UTC(), Level: "warn", Message: "无法枚举可信 Panel 历史镜像，跳过带 tag 历史清理"})
		return
	}
	containersCtx, containersCancel := context.WithTimeout(ctx, 30*time.Second)
	containers, err := executor.Output(containersCtx, "container", "ls", "--all", "--no-trunc", "--format", "{{.Image}}")
	containersCancel()
	if err != nil {
		status.Logs = append(status.Logs, LogEntry{At: time.Now().UTC(), Level: "warn", Message: "无法核对宿主容器镜像引用，跳过带 tag 历史清理"})
		return
	}
	protected := map[string]bool{strings.TrimSpace(selectedImage): true, strings.TrimSpace(selectedDigest): true}
	for _, line := range strings.Split(containers, "\n") {
		if value := strings.TrimSpace(line); value != "" {
			protected[value] = true
		}
	}
	removed := 0
	for _, line := range strings.Split(images, "\n") {
		parts := strings.Split(strings.TrimSpace(line), "|")
		if len(parts) != 3 || parts[0] == "<none>" || parts[1] == "<none>" {
			continue
		}
		repository, tag, listedID := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), strings.TrimSpace(parts[2])
		if _, ok := trustedRepositoryOf(repository); !ok {
			continue
		}
		if tag != "latest" {
			if _, normalizeErr := NormalizeTargetVersion(tag); normalizeErr != nil {
				continue
			}
		}
		ref := repository + ":" + tag
		if protected[ref] || protected[listedID] {
			continue
		}
		inspectCtx, inspectCancel := context.WithTimeout(ctx, 30*time.Second)
		actualID, inspectErr := executor.Output(inspectCtx, "image", "inspect", "--format", "{{.Id}}", ref)
		inspectCancel()
		if inspectErr != nil || strings.TrimSpace(actualID) != listedID {
			continue
		}
		removeCtx, removeCancel := context.WithTimeout(ctx, 2*time.Minute)
		removeErr := executor.Run(removeCtx, "image", "rm", ref)
		removeCancel()
		if removeErr == nil {
			removed++
		}
	}
	if removed > 0 {
		status.Logs = append(status.Logs, LogEntry{At: time.Now().UTC(), Level: "info", Message: fmt.Sprintf("已清理 %d 个未被容器引用的可信 Panel 历史镜像 tag", removed)})
	}
}

func secureApplyPaths(opts ApplyOptions) error {
	dataDir := filepath.Clean(opts.DataDir)
	if opts.StateFile != filepath.Join(dataDir, "updater", "apply-status.json") || !filepath.IsAbs(opts.ComposeFile) || !filepath.IsAbs(opts.BackupDir) {
		return errors.New("invalid paths")
	}
	backupRoot := filepath.Join(dataDir, "updater", "backups") + string(filepath.Separator)
	if !strings.HasPrefix(filepath.Clean(opts.BackupDir), backupRoot) {
		return errors.New("invalid backup")
	}
	db := filepath.Clean(opts.DatabaseRelative)
	if db == "." || filepath.IsAbs(db) || strings.HasPrefix(db, "..") {
		return errors.New("invalid database")
	}
	return nil
}

func composeBaseArgs(project, composeFile, envFile string) []string {
	return []string{"compose", "--project-name", project, "--env-file", envFile, "-f", composeFile}
}

func waitForPanel(ctx context.Context, executor ApplyExecutor, container, expectedVersion string, timeout, interval time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		inspectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		state, err := executor.Output(inspectCtx, "inspect", "--format", "{{.State.Status}}|{{if .State.Health}}{{.State.Health.Status}}{{end}}", container)
		cancel()
		state = strings.TrimSpace(state)
		parts := strings.SplitN(state, "|", 2)
		containerState := parts[0]
		healthState := ""
		if len(parts) == 2 {
			healthState = parts[1]
		}
		if err == nil && (containerState == "exited" || containerState == "dead") {
			return applyFailure{CodeHealthCheckFailed, "新 panel 容器启动后立即退出"}
		}
		if err == nil && containerState == "running" && healthState == "healthy" {
			healthCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			healthJSON, healthErr := executor.Output(healthCtx, "exec", container, "wget", "-qO-", "http://127.0.0.1:8090/health")
			cancel()
			versionCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			versionJSON, versionErr := executor.Output(versionCtx, "exec", container, "wget", "-qO-", "http://127.0.0.1:8090/api/version")
			cancel()
			if healthErr == nil && versionErr == nil {
				var health struct {
					Status string `json:"status"`
				}
				var version struct {
					Version string `json:"version"`
				}
				if json.Unmarshal([]byte(healthJSON), &health) == nil && health.Status == "ok" && json.Unmarshal([]byte(versionJSON), &version) == nil {
					actual, normalizeErr := NormalizeTargetVersion(version.Version)
					if normalizeErr == nil && actual != expectedVersion {
						return applyFailure{CodeVersionMismatch, "新 panel API 版本与目标版本不一致"}
					}
					if normalizeErr == nil && actual == expectedVersion {
						return nil
					}
				}
			}
		}
		select {
		case <-ctx.Done():
			return applyFailure{CodeHealthCheckFailed, "健康验收被取消"}
		case <-time.After(interval):
		}
	}
	return applyFailure{CodeHealthCheckFailed, "等待新 panel 健康超时"}
}

func rollbackApply(ctx context.Context, executor ApplyExecutor, opts ApplyOptions, backupCompose, backupEnv string, recreateAttempted bool) error {
	if recreateAttempted {
		stopCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		stopErr := executor.Run(stopCtx, "stop", "--time", "10", opts.CurrentContainer)
		cancel()
		if stopErr != nil {
			inspectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			state, inspectErr := executor.Output(inspectCtx, "inspect", "--format", "{{.State.Status}}", opts.CurrentContainer)
			cancel()
			if inspectErr == nil && strings.TrimSpace(state) != "exited" && strings.TrimSpace(state) != "dead" {
				return errors.New("failed panel container could not be stopped")
			}
		}
		backupDB := filepath.Join(opts.BackupDir, "panel.db")
		targetDB := filepath.Join(opts.DataDir, filepath.Clean(opts.DatabaseRelative))
		if err := copyFileAtomic(backupDB, targetDB, 0o600); err != nil {
			return err
		}
		_ = os.Remove(targetDB + "-wal")
		_ = os.Remove(targetDB + "-shm")
	}
	if err := copyFileAtomic(backupCompose, opts.ComposeFile, 0o600); err != nil {
		return err
	}
	envFile := filepath.Join(filepath.Dir(opts.ComposeFile), ".env")
	if err := copyFileAtomic(backupEnv, envFile, 0o600); err != nil {
		return err
	}
	if !recreateAttempted {
		return waitForPanel(ctx, executor, opts.CurrentContainer, opts.FromVersion, opts.HealthTimeout, opts.PollInterval)
	}
	composeArgs := composeBaseArgs(opts.ComposeProject, opts.ComposeFile, envFile)
	configCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	resolved, err := executor.Output(configCtx, append(composeArgs, "config", "--images", "panel")...)
	cancel()
	if err != nil || strings.TrimSpace(resolved) != opts.CurrentImage {
		return errors.New("restored compose image mismatch")
	}
	digestCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	digest, digestErr := executor.Output(digestCtx, "image", "inspect", "--format", "{{.Id}}", opts.CurrentImage)
	cancel()
	if digestErr != nil || strings.TrimSpace(digest) != strings.TrimSpace(opts.OriginalDigest) {
		return errors.New("restored image digest mismatch")
	}
	upCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	err = executor.Run(upCtx, append(composeArgs, "up", "-d", "--pull", "never", "--force-recreate", "--no-deps", "panel")...)
	cancel()
	if err != nil {
		return err
	}
	return waitForPanel(ctx, executor, opts.CurrentContainer, opts.FromVersion, opts.HealthTimeout, opts.PollInterval)
}

func updateEnvImage(path, image string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	found := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "PANEL_IMAGE=") {
			lines[i] = "PANEL_IMAGE=" + image
			found = true
		}
	}
	if !found {
		return errors.New("PANEL_IMAGE missing")
	}
	return writeFileAtomic(path, []byte(strings.Join(lines, "\n")), 0o600)
}

func copyFileAtomic(source, target string, mode os.FileMode) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return writeFileAtomic(target, data, mode)
}
func writeJSONAtomic(path string, value any, mode os.FileMode) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, data, mode)
}
func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".update-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(mode); err != nil {
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
	return atomicReplaceFile(tmpName, path)
}
