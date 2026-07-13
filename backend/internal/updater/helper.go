package updater

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type DryRunOptions struct {
	TargetVersion  string
	CurrentImage   string
	ComposeProject string
	ComposeFile    string
	StateFile      string
	Now            func() time.Time
	Executor       SafeExecutor
}

type SafeExecutor interface {
	Run(context.Context, ...string) error
}

type ExecDocker struct{ Path string }

func (e ExecDocker) Run(ctx context.Context, args ...string) error {
	path := e.Path
	if path == "" {
		path = "docker"
	}
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Stdout = &limitedDiscard{limit: 64 << 10}
	cmd.Stderr = &limitedDiscard{limit: 64 << 10}
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return errors.New("docker command timed out")
		}
		return errors.New("docker command failed")
	}
	return nil
}

func RunDryRun(ctx context.Context, opts DryRunOptions) error {
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	executor := opts.Executor
	if executor == nil {
		executor = ExecDocker{}
	}
	store := NewStateStore(opts.StateFile)
	status, err := store.Read()
	if err != nil && !errors.Is(err, ErrNoDryRunStatus) {
		return err
	}
	if status.ID == "" {
		started := now().UTC()
		status = DryRunStatus{ID: newDryRunID(), Phase: "running", TargetVersion: opts.TargetVersion, Logs: []LogEntry{}, StartedAt: started, UpdatedAt: started}
	}
	appendLog := func(level, message string) {
		status.UpdatedAt = now().UTC()
		status.Logs = append(status.Logs, LogEntry{At: status.UpdatedAt, Level: level, Message: message})
		_ = store.Write(status)
	}
	fail := func(code, message string) error {
		finished := now().UTC()
		status.Phase, status.ErrorCode, status.Error = "failed", code, message
		status.UpdatedAt, status.FinishedAt = finished, &finished
		status.Logs = append(status.Logs, LogEntry{At: finished, Level: "error", Message: message})
		if writeErr := store.Write(status); writeErr != nil {
			return writeErr
		}
		return errors.New(message)
	}
	version, err := NormalizeTargetVersion(opts.TargetVersion)
	if err != nil {
		return fail(CodeInvalidTargetVersion, "目标版本不合法")
	}
	candidates, err := TrustedImageCandidates(version, opts.CurrentImage)
	if err != nil {
		return fail(CodeImageNotAllowed, "无法生成可信升级镜像候选")
	}
	if !composeProjectPattern.MatchString(opts.ComposeProject) || !filepath.IsAbs(opts.ComposeFile) {
		return fail(CodeComposeMetadataInvalid, "Compose 项目参数不合法")
	}
	status.Phase = "running"
	status.TargetVersion = version
	status.Error, status.ErrorCode = "", ""
	appendLog("info", "开始检查精确版本镜像和 Compose 配置；不会执行 stop/rm/up/restart")

	var selected string
	for _, candidate := range candidates {
		if err := ValidateTrustedImage(candidate); err != nil {
			return fail(CodeImageNotAllowed, "镜像候选未通过项目白名单")
		}
		inspectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		err := executor.Run(inspectCtx, "image", "inspect", candidate)
		cancel()
		if err == nil {
			selected = candidate
			appendLog("info", "已在本机找到可信目标镜像 "+candidate)
			break
		}
		appendLog("info", "正在拉取可信目标镜像 "+candidate)
		pullCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
		err = executor.Run(pullCtx, "pull", candidate)
		cancel()
		if err == nil {
			selected = candidate
			appendLog("info", "目标镜像拉取成功 "+candidate)
			break
		}
		appendLog("warn", "该可信镜像候选不可用，继续尝试下一个")
	}
	if selected == "" {
		return fail("image_unavailable", "所有可信目标镜像候选均不可用")
	}
	overrideDir, err := os.MkdirTemp("", "panel-updater-")
	if err != nil {
		return fail("compose_validation_failed", "无法创建 Compose 校验临时目录")
	}
	defer os.RemoveAll(overrideDir)
	overrideFile := filepath.Join(overrideDir, "docker-compose.updater.yml")
	override := []byte("services:\n  panel:\n    image: " + selected + "\n")
	if err := os.WriteFile(overrideFile, override, 0o600); err != nil {
		return fail("compose_validation_failed", "无法创建 Compose 校验覆盖文件")
	}
	args := []string{"compose", "--project-name", opts.ComposeProject}
	if envFile := filepath.Join(filepath.Dir(opts.ComposeFile), ".env"); fileExists(envFile) {
		args = append(args, "--env-file", envFile)
	}
	args = append(args, "-f", opts.ComposeFile, "-f", overrideFile, "config", "--quiet")
	composeCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	err = executor.Run(composeCtx, args...)
	cancel()
	if err != nil {
		return fail("compose_validation_failed", "目标镜像的 Compose config 校验失败")
	}
	finished := now().UTC()
	status.Phase, status.TargetImage = "succeeded", selected
	status.UpdatedAt, status.FinishedAt = finished, &finished
	status.Logs = append(status.Logs, LogEntry{At: finished, Level: "info", Message: "升级环境演练通过；未改变当前面板容器"})
	if err := store.Write(status); err != nil {
		return fmt.Errorf("write successful dry-run status: %w", err)
	}
	return nil
}

func fileExists(path string) bool {
	stat, err := os.Stat(path)
	return err == nil && !stat.IsDir()
}

func IsDestructiveDockerArgs(args []string) bool {
	if len(args) == 0 {
		return false
	}
	if args[0] == "stop" || args[0] == "rm" || args[0] == "restart" {
		return true
	}
	if args[0] == "compose" {
		for _, arg := range args[1:] {
			switch strings.ToLower(arg) {
			case "up", "down", "stop", "rm", "restart", "start", "kill":
				return true
			}
		}
	}
	return false
}
