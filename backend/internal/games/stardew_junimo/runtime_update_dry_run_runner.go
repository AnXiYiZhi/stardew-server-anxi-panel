package stardew_junimo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
)

func (d *Driver) runRuntimeUpdateDryRun(ctx context.Context, job *jobs.Context, docker RuntimeUpdateDockerService, instance registry.Instance, status RuntimeUpdateDryRunStatus) error {
	setPhase := func(phase string, progress int, message string) error {
		status.Phase = phase
		status.Progress = progress
		status.ErrorCode = ""
		status.Error = ""
		status.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		status.Logs = append(status.Logs, RuntimeUpdateDryRunLog{At: status.UpdatedAt, Level: "info", Message: paneldocker.RedactString(message)})
		_, _ = job.Info(ctx, message)
		return writeRuntimeUpdateDryRunStatus(instance.DataDir, status)
	}
	addCheck := func(name, checkStatus, message string) error {
		status.Checks = append(status.Checks, RuntimeUpdateDryRunCheck{Name: name, Status: checkStatus, Message: paneldocker.RedactString(message)})
		status.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		return writeRuntimeUpdateDryRunStatus(instance.DataDir, status)
	}
	addWarning := func(message string) error {
		message = paneldocker.RedactString(message)
		now := time.Now().UTC().Format(time.RFC3339)
		status.Warnings = append(status.Warnings, message)
		status.Logs = append(status.Logs, RuntimeUpdateDryRunLog{At: now, Level: "warning", Message: message})
		status.UpdatedAt = now
		_, _ = job.Warn(ctx, message)
		return writeRuntimeUpdateDryRunStatus(instance.DataDir, status)
	}
	finish := func(phase, code, message string) error {
		now := time.Now().UTC().Format(time.RFC3339)
		status.Phase = phase
		status.UpdatedAt = now
		status.FinishedAt = now
		logMessage := paneldocker.RedactString(message)
		level := "error"
		if phase == RuntimeUpdatePhaseSucceeded {
			status.Progress = 100
			status.ErrorCode = ""
			status.Error = ""
			level = "info"
		} else {
			status.ErrorCode = code
			status.Error = logMessage
		}
		if phase == RuntimeUpdatePhaseUnsupported {
			level = "warning"
		}
		status.Logs = append(status.Logs, RuntimeUpdateDryRunLog{At: now, Level: level, Message: logMessage})
		if err := writeRuntimeUpdateDryRunStatus(instance.DataDir, status); err != nil {
			return err
		}
		if phase == RuntimeUpdatePhaseFailed {
			return errors.New(message)
		}
		return nil
	}
	fail := func(code, message string) error {
		return finish(RuntimeUpdatePhaseFailed, code, message)
	}
	unsupported := func(code, message string) error {
		return finish(RuntimeUpdatePhaseUnsupported, code, message)
	}

	if err := setPhase(RuntimeUpdatePhaseChecking, 10, "正在检查实例与内置推荐版本对。"); err != nil {
		return err
	}
	manifest, err := sjconfig.BuiltInRuntimeStackManifest()
	if err != nil || !manifest.Installable() || !sjconfig.PanelVersionSatisfies(d.panelVersion, manifest.MinimumPanelVersion) {
		_ = addCheck("manifest", "error", "内置推荐版本对清单无效或未测试。")
		return fail("manifest_invalid", "内置推荐版本对清单无效。")
	}
	if err := addCheck("manifest", "ok", "内置兼容矩阵合法且 status=recommended。"); err != nil {
		return err
	}
	if err := addCheck("instance", "ok", "实例已安装且 driver 为 stardew_junimo。"); err != nil {
		return err
	}

	composePath := filepath.Join(instance.DataDir, "docker-compose.yml")
	if !filepath.IsAbs(instance.DataDir) {
		return unsupported("instance_path_unsafe", "实例目录不是可验证的绝对路径。")
	}
	info, err := os.Lstat(composePath)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		_ = addCheck("compose_file", "error", "Compose 文件缺失、不是普通文件或是符号链接。")
		return unsupported("compose_file_unsafe", "Compose 文件无法安全识别。")
	}
	project := strings.ToLower(filepath.Base(filepath.Clean(instance.DataDir)))
	if !runtimeComposeProjectPattern.MatchString(project) {
		return unsupported("compose_project_unsafe", "Compose project 名称无法安全推导。")
	}
	if err := addCheck("compose_file", "ok", "已安全识别实例 docker-compose.yml 与 Compose project。"); err != nil {
		return err
	}

	if _, err := docker.DockerVersion(ctx, instance.DataDir); err != nil {
		_ = addCheck("docker", "error", "Docker 不可用。")
		return unsupported("docker_unavailable", "Docker 不可用，无法执行升级预检。")
	}
	if err := addCheck("docker", "ok", "Docker 可用。"); err != nil {
		return err
	}
	if _, err := docker.ComposeVersion(ctx, instance.DataDir); err != nil {
		_ = addCheck("compose", "error", "Docker Compose 不可用。")
		return unsupported("compose_unavailable", "Docker Compose 不可用，无法执行升级预检。")
	}
	if err := addCheck("compose", "ok", "Docker Compose 可用。"); err != nil {
		return err
	}

	composeConfig, err := docker.RuntimeComposeConfigInspect(ctx, instance.DataDir, project)
	if err != nil {
		_ = addCheck("compose_model", "error", "Compose 配置无法解析。")
		return unsupported("compose_config_invalid", "Compose 配置无法安全解析。")
	}
	if composeConfig.Project != "" && composeConfig.Project != project {
		return unsupported("compose_project_mismatch", "Compose project 与实例目录不一致。")
	}
	if !containsRuntimeService(composeConfig.Services, "server") || !containsRuntimeService(composeConfig.Services, "steam-auth") {
		_ = addCheck("compose_services", "error", "Compose 缺少 server 或 steam-auth 服务。")
		return unsupported("compose_services_missing", "Compose 必须同时包含 server 与 steam-auth 服务。")
	}
	if err := addCheck("compose_services", "ok", "Compose 同时包含 server 与 steam-auth 服务。"); err != nil {
		return err
	}

	serverMetadata, err := docker.RuntimeImageInspect(ctx, instance.DataDir, status.Current.Server.Image)
	if err != nil || !runtimeImageDigestPattern.MatchString(serverMetadata.Digest) {
		_ = addCheck("current_server_image", "error", "当前 server 镜像或 digest 无法 inspect。")
		return fail("current_server_digest_unavailable", "当前 server 镜像 digest 无法确认。")
	}
	if err := addCheck("current_server_image", "ok", "当前 server 镜像与 digest 已确认。"); err != nil {
		return err
	}
	authMetadata, err := docker.RuntimeImageInspect(ctx, instance.DataDir, status.Current.SteamAuth.Image)
	if err != nil || !runtimeImageDigestPattern.MatchString(authMetadata.Digest) {
		_ = addCheck("current_auth_image", "error", "当前 steam-auth-cn 镜像或 digest 无法 inspect。")
		return fail("current_auth_digest_unavailable", "当前 steam-auth-cn 镜像 digest 无法确认。")
	}
	if err := addCheck("current_auth_image", "ok", "当前 steam-auth-cn 镜像与 digest 已确认。"); err != nil {
		return err
	}

	ps, psErr := docker.ComposePs(ctx, instance.DataDir)
	if psErr != nil {
		if err := addWarning("无法读取当前 Compose 运行状态；预检继续，但 serverRunning 可能不是实时值。"); err != nil {
			return err
		}
	} else {
		status.ServerRunning = composeServiceRunning(ps.Services, "server")
		message := "server 当前未运行；预检不会启动它。"
		if status.ServerRunning {
			message = "server 当前正在运行；预检保持只读，不会停服或重启。"
		}
		if err := addCheck("server_runtime", "ok", message); err != nil {
			return err
		}
	}

	if composeConfig.SteamSessionVolume == "" {
		_ = addCheck("steam_session_volume", "error", "无法从 Compose 安全识别 steam-session volume。")
		return unsupported("steam_session_volume_unknown", "无法安全识别 steam-session 认证卷。")
	}
	volume, err := docker.RuntimeVolumeInspect(ctx, instance.DataDir, composeConfig.SteamSessionVolume)
	if err != nil {
		_ = addCheck("steam_session_volume", "error", "steam-session volume 不存在或无法 inspect。")
		return unsupported("steam_session_volume_missing", "steam-session 认证卷不存在或无法验证。")
	}
	if err := addCheck("steam_session_volume", "ok", "steam-session 认证卷存在且名称已由 Compose/inspect 验证。"); err != nil {
		return err
	}
	if volume.Mountpoint == "" {
		if err := addWarning("Docker 未提供可验证的认证卷挂载路径，无法可靠判断宿主文件系统可读性；未读取任何 token 内容。"); err != nil {
			return err
		}
		if err := addCheck("steam_session_readable", "warning", "认证卷存在，但可读性无法可靠判断。"); err != nil {
			return err
		}
	} else if handle, openErr := os.Open(volume.Mountpoint); openErr != nil {
		if err := addWarning("Panel 无法直接访问认证卷挂载路径，可读性检查降级为 warning；未读取任何 token 内容。"); err != nil {
			return err
		}
		if err := addCheck("steam_session_readable", "warning", "认证卷存在，但 Panel 无法可靠验证目录可读性。"); err != nil {
			return err
		}
	} else {
		_ = handle.Close()
		if err := addCheck("steam_session_readable", "ok", "认证卷目录可打开；未枚举文件且未读取 token 内容。"); err != nil {
			return err
		}
	}
	if err := addWarning("Docker 镜像层位于 daemon 主机，Panel 无法可靠获得其精确可用空间；请确认 Docker 数据盘有足够空间。"); err != nil {
		return err
	}
	if err := addCheck("disk_space", "warning", "Docker 数据盘精确可用空间无法可靠判断，未伪造数值。"); err != nil {
		return err
	}

	if err := setPhase(RuntimeUpdatePhasePullingServer, 50, "正在按可信候选顺序检查或拉取推荐 server 镜像。"); err != nil {
		return err
	}
	selectedServer, code := selectRuntimeUpdateImage(ctx, docker, instance.DataDir, status.Target.Server.TrustedCandidates, status.Target.Server.Digests)
	if code != "" {
		_ = addCheck("target_server_image", "error", "所有推荐 server 镜像候选均失败或无法取得 digest。")
		return fail(code, "推荐 server 镜像候选全部不可用。")
	}
	status.Selected.Server = selectedServer
	if err := addCheck("target_server_image", "ok", "已选择可信 server 镜像并确认 digest。"); err != nil {
		return err
	}

	if err := setPhase(RuntimeUpdatePhasePullingAuth, 70, "正在按可信候选顺序检查或拉取推荐 steam-auth-cn 镜像。"); err != nil {
		return err
	}
	selectedAuth, code := selectRuntimeUpdateImage(ctx, docker, instance.DataDir, status.Target.SteamAuth.TrustedCandidates, status.Target.SteamAuth.Digests)
	if code != "" {
		_ = addCheck("target_auth_image", "error", "所有推荐 steam-auth-cn 镜像候选均失败或无法取得 digest。")
		return fail(code, "推荐 steam-auth-cn 镜像候选全部不可用。")
	}
	status.Selected.SteamAuth = selectedAuth
	if err := addCheck("target_auth_image", "ok", "已选择可信 steam-auth-cn 镜像并确认 digest。"); err != nil {
		return err
	}

	if err := setPhase(RuntimeUpdatePhaseValidatingCompose, 90, "正在用受控镜像覆盖验证 docker compose config --quiet。"); err != nil {
		return err
	}
	if err := docker.RuntimeComposeConfigValidateImages(ctx, instance.DataDir, project, selectedServer.Image, selectedAuth.Image); err != nil {
		_ = addCheck("compose_target_config", "error", "推荐版本对未通过 Compose 配置验证。")
		return fail("compose_target_validation_failed", "推荐版本对的 Compose 配置验证失败。")
	}
	if err := addCheck("compose_target_config", "ok", "推荐 server + steam-auth-cn 版本对通过 Compose 配置验证。"); err != nil {
		return err
	}
	return finish(RuntimeUpdatePhaseSucceeded, "", "Junimo 运行组件升级预检通过；未修改实例或容器。")
}

func selectRuntimeUpdateImage(ctx context.Context, docker RuntimeUpdateDockerService, dataDir string, candidates []string, expectedDigests map[string]string) (RuntimeUpdateSelectedImage, string) {
	digestMissing := false
	digestMismatch := false
	for _, image := range candidates {
		expected := strings.ToLower(strings.TrimSpace(expectedDigests[image]))
		if !runtimeImageDigestPattern.MatchString(expected) {
			digestMissing = true
			continue
		}
		metadata, inspectErr := docker.RuntimeImageInspect(ctx, dataDir, image)
		if inspectErr == nil && strings.EqualFold(metadata.Digest, expected) && runtimeImageDigestPattern.MatchString(metadata.ID) {
			return RuntimeUpdateSelectedImage{Image: image, Digest: metadata.Digest, ImageID: metadata.ID}, ""
		}
		if inspectErr == nil && runtimeImageDigestPattern.MatchString(metadata.Digest) && !strings.EqualFold(metadata.Digest, expected) {
			digestMismatch = true
		}
		if inspectErr == nil && (!runtimeImageDigestPattern.MatchString(metadata.Digest) || !runtimeImageDigestPattern.MatchString(metadata.ID)) {
			digestMissing = true
		}
		if _, pullErr := docker.PullImageStreaming(ctx, dataDir, image, func(string) {}); pullErr != nil {
			continue
		}
		metadata, inspectErr = docker.RuntimeImageInspect(ctx, dataDir, image)
		if inspectErr != nil {
			continue
		}
		if !runtimeImageDigestPattern.MatchString(metadata.Digest) || !runtimeImageDigestPattern.MatchString(metadata.ID) {
			digestMissing = true
			continue
		}
		if !strings.EqualFold(metadata.Digest, expected) {
			digestMismatch = true
			continue
		}
		return RuntimeUpdateSelectedImage{Image: image, Digest: metadata.Digest, ImageID: metadata.ID}, ""
	}
	if digestMismatch {
		return RuntimeUpdateSelectedImage{}, "target_image_digest_mismatch"
	}
	if digestMissing {
		return RuntimeUpdateSelectedImage{}, "target_image_digest_unavailable"
	}
	return RuntimeUpdateSelectedImage{}, "target_image_candidates_failed"
}

func containsRuntimeService(services []string, expected string) bool {
	for _, service := range services {
		if service == expected {
			return true
		}
	}
	return false
}

func composeServiceRunning(services []paneldocker.ComposeService, expected string) bool {
	for _, service := range services {
		if service.Service == expected && strings.EqualFold(service.State, "running") {
			return true
		}
	}
	return false
}
