package stardew_junimo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

type runtimeUpdatePreflight struct {
	project, volume              string
	originalServer, originalAuth RuntimeUpdateSelectedImage
	target                       RuntimeUpdateSelectedPair
	authWasRunning               bool
}

func (d *Driver) runRuntimeUpdateApply(ctx context.Context, job *jobs.Context, docker RuntimeUpdateApplyDockerService, instance registry.Instance, status RuntimeUpdateApplyStatus, recovery *runtimeUpdateRecoveryManifest) error {
	setPhase := func(phase string, progress int, message string) error {
		status.Phase, status.Progress, status.ErrorCode, status.Error = phase, progress, "", ""
		status.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		status.Logs = append(status.Logs, RuntimeUpdateDryRunLog{At: status.UpdatedAt, Level: "info", Message: paneldocker.RedactString(message)})
		_, _ = job.Info(ctx, message)
		return writeRuntimeUpdateApplyStatus(instance.DataDir, status)
	}
	finish := func(phase, code, message string) error {
		now := time.Now().UTC().Format(time.RFC3339)
		status.Phase, status.Progress, status.UpdatedAt, status.FinishedAt = phase, 100, now, now
		status.ErrorCode, status.Error = code, paneldocker.RedactString(message)
		level := "error"
		if phase == RuntimeUpdateApplySucceeded {
			level = "info"
			status.ErrorCode, status.Error = "", ""
		}
		status.Logs = append(status.Logs, RuntimeUpdateDryRunLog{At: now, Level: level, Message: paneldocker.RedactString(message)})
		status.ServerRunning = status.ServerWasRunning && phase == RuntimeUpdateApplySucceeded
		if err := writeRuntimeUpdateApplyStatus(instance.DataDir, status); err != nil {
			return err
		}
		d.auditRuntimeUpdateTerminal(ctx, instance.ID, status)
		return nil
	}

	if recovery != nil {
		if !recovery.ConfigWritten {
			_ = os.RemoveAll(runtimeUpdateRecoveryDir(instance.DataDir, recovery.ApplyID))
			return finish(RuntimeUpdateApplyFailedRolledBack, "panel_restart_before_change", "Panel 重启发生在实例修改前；实例保持原状。")
		}
		if recovery.AuthRecreated && recovery.ServerRecreated && (status.Phase == RuntimeUpdateApplyVerifyingServer || status.Phase == RuntimeUpdateApplyRestoringState) {
			if err := d.verifyRuntimeTarget(ctx, docker, instance, *recovery); err == nil {
				if err := d.restoreRuntimeRunningState(ctx, job, docker, instance, *recovery); err == nil {
					_ = docker.RuntimeRemoveSnapshotVolume(ctx, instance.DataDir, recovery.Project, recovery.SnapshotVolume)
					_ = os.RemoveAll(runtimeUpdateRecoveryDir(instance.DataDir, recovery.ApplyID))
					cleanupOldRuntimeImages(ctx, docker, instance.DataDir, *recovery, &status)
					return finish(RuntimeUpdateApplySucceeded, "", "Panel 重启后已继续完成验收，Junimo 运行组件成对升级成功。")
				}
			}
		}
		return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, *recovery, "panel_restart_recovery", "Panel 重启后进入受控回滚。")
	}

	if err := setPhase(RuntimeUpdateApplyChecking, 5, "正在重新执行关键升级预检。"); err != nil {
		return err
	}
	preflight, err := d.runtimeUpdateApplyPreflight(ctx, job, docker, instance, &status)
	if err != nil {
		_ = finish(RuntimeUpdateApplyFailedRolledBack, runtimeUpdateErrorCode(err), "关键预检失败；实例未修改。")
		return err
	}
	status.Checks = append(status.Checks, RuntimeUpdateDryRunCheck{Name: "critical_preflight", Status: "ok", Message: "清单、实例、Docker/Compose、当前 digest、认证卷和两个目标镜像均已重新确认。"})

	if err := setPhase(RuntimeUpdateApplyPulling, 20, "推荐 server 与 steam-auth-cn 镜像已按可信候选拉取并确认 digest。"); err != nil {
		return err
	}
	status.Selected = preflight.target
	if err := writeRuntimeUpdateApplyStatus(instance.DataDir, status); err != nil {
		return err
	}

	manifest := runtimeUpdateRecoveryManifest{SchemaVersion: 1, ApplyID: status.ApplyID, ActorID: status.CreatedBy, Project: preflight.project, SteamSessionVolume: preflight.volume, SnapshotVolume: preflight.project + "_anxi-junimo-update-" + strings.TrimPrefix(status.ApplyID, "apply_") + "-steam-session", ServerWasRunning: status.ServerWasRunning, AuthWasRunning: preflight.authWasRunning, OriginalState: instance.State, OriginalServer: preflight.originalServer, OriginalAuth: preflight.originalAuth, Target: preflight.target, OriginalServerVersion: status.Current.Server.Tag, TargetServerVersion: status.Target.Server.Tag}
	if err := setPhase(RuntimeUpdateApplyBackingUp, 30, "正在创建私有恢复材料并保护 Steam 认证卷。"); err != nil {
		return err
	}
	if err := createRuntimeRecoveryFiles(instance.DataDir, manifest); err != nil {
		_ = finish(RuntimeUpdateApplyFailedRolledBack, "backup_failed", "无法创建私有恢复材料；实例未修改。")
		return err
	}
	manifest.ControlManifestPresent, manifest.ControlDLLPresent, err = backupRuntimeControlMod(instance.DataDir, manifest.ApplyID)
	if err != nil {
		_ = finish(RuntimeUpdateApplyFailedRolledBack, "control_backup_failed", "无法备份升级前的 Control Mod；实例未修改。")
		return err
	}
	if err := writeRuntimeUpdateRecoveryManifest(instance.DataDir, manifest); err != nil {
		return err
	}
	stored, err := d.store.GetInstance(ctx, instance.ID)
	if err != nil {
		return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, manifest, "instance_reload_failed", "无法重新读取实例。")
	}
	if manifest.ServerWasRunning || manifest.AuthWasRunning {
		if err := setPhase(RuntimeUpdateApplyStopping, 40, "正在复用现有生命周期能力安全停止实例。"); err != nil {
			return err
		}
		lr := &lifecycleRunner{driver: d, lifecycle: docker, instance: stored, operation: "stop"}
		if err := lr.doStop(ctx, job); err != nil {
			return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, manifest, "stop_failed", "安全停服失败。")
		}
	}
	// Control is a host bind. Replace it only after the game process has fully
	// stopped so a live CLR process can never observe a half-updated DLL.
	if err := installSMAPIMod(instance.DataDir); err != nil {
		now := time.Now().UTC().Format(time.RFC3339)
		status.Phase, status.Progress, status.ErrorCode, status.Error = RuntimeUpdateApplyFailedRolledBack, 100, "control_sync_failed", "Control Mod 同步失败；实例保持停止，禁止继续启动旧 DLL。"
		status.UpdatedAt, status.FinishedAt, status.ServerRunning = now, now, false
		status.Logs = append(status.Logs, RuntimeUpdateDryRunLog{At: now, Level: "error", Message: status.Error})
		_ = writeRuntimeUpdateApplyStatus(instance.DataDir, status)
		d.updatePhase(ctx, instance.ID, storage.InstanceStateError, status.Error, "control_sync_failed", job.ID)
		d.auditRuntimeUpdateTerminal(ctx, instance.ID, status)
		return err
	}
	manifest.ControlUpdated = true
	if err := writeRuntimeUpdateRecoveryManifest(instance.DataDir, manifest); err != nil {
		now := time.Now().UTC().Format(time.RFC3339)
		status.Phase, status.Progress, status.ErrorCode, status.Error = RuntimeUpdateApplyRollbackFailed, 100, "control_recovery_manifest_failed", "Control 已同步但恢复清单写入失败；实例保持停止并需要人工核对。"
		status.UpdatedAt, status.FinishedAt, status.ServerRunning = now, now, false
		status.ManualAction = "核对 Control manifest/DLL 与当前 Panel 内置版本和 SHA256；确认一致后再从 Panel 启动实例。"
		_ = writeRuntimeUpdateApplyStatus(instance.DataDir, status)
		d.updatePhase(ctx, instance.ID, storage.InstanceStateError, status.Error, "control_recovery_manifest_failed", job.ID)
		d.auditRuntimeUpdateTerminal(ctx, instance.ID, status)
		return err
	}
	// Clone only after the existing lifecycle stop has quiesced steam-auth, so
	// the saved session is a consistent volume snapshot.
	if err := docker.RuntimeCreateSnapshotVolume(ctx, instance.DataDir, manifest.Project, manifest.SnapshotVolume); err != nil {
		return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, manifest, "auth_snapshot_create_failed", "无法创建 Steam 认证卷临时快照。")
	}
	if err := docker.RuntimeCloneVolume(ctx, instance.DataDir, manifest.SteamSessionVolume, manifest.SnapshotVolume, manifest.Target.Server.Image); err != nil {
		return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, manifest, "auth_snapshot_failed", "Steam 认证卷保护失败。")
	}
	status.Checks = append(status.Checks, RuntimeUpdateDryRunCheck{Name: "steam_session_snapshot", Status: "ok", Message: "steam-session 已在停服后克隆到当前 Compose project 限定的临时 Docker volume；未读取 token 内容。"})
	if err := writeRuntimeUpdateRecoveryManifest(instance.DataDir, manifest); err != nil {
		return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, manifest, "recovery_manifest_failed", "恢复清单写入失败。")
	}

	if err := setPhase(RuntimeUpdateApplyWritingConfig, 50, "正在事务化同步目标 JunimoServer Mod 并写入推荐版本对配置。"); err != nil {
		return err
	}
	recoveryDir := runtimeUpdateRecoveryDir(instance.DataDir, manifest.ApplyID)
	extractedDir, err := extractJunimoServerMod(ctx, docker, manifest.Target.Server.Image, recoveryDir, manifest.TargetServerVersion)
	if err != nil {
		return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, manifest, "junimo_mod_extract_failed", "无法从目标 server 镜像提取并验证 JunimoServer Mod。")
	}
	manifest.JunimoModPrepared = true
	if err := writeRuntimeUpdateRecoveryManifest(instance.DataDir, manifest); err != nil {
		return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, manifest, "recovery_manifest_failed", "恢复清单写入失败。")
	}
	manifest.JunimoModOriginalPresent, err = replaceJunimoServerMod(instance.DataDir, extractedDir, filepath.Join(recoveryDir, runtimeOriginalJunimoDir))
	if err != nil {
		code := "junimo_mod_replace_failed"
		if errors.Is(err, os.ErrPermission) {
			code = "junimo_mod_directory_locked"
		} else if errors.Is(err, os.ErrExist) {
			code = "junimo_mod_backup_exists"
		} else if errors.Is(err, os.ErrNotExist) {
			code = "junimo_mod_directory_missing"
		} else if strings.Contains(err.Error(), "move current JunimoServer") {
			code = "junimo_mod_backup_rename_failed"
		} else if strings.Contains(err.Error(), "activate target JunimoServer") {
			code = "junimo_mod_activate_rename_failed"
		}
		return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, manifest, code, "无法原子替换宿主 JunimoServer Mod。")
	}
	manifest.JunimoModReplaced = true
	if err := writeRuntimeUpdateRecoveryManifest(instance.DataDir, manifest); err != nil {
		return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, manifest, "recovery_manifest_failed", "恢复清单写入失败。")
	}
	if err := writeRuntimeTargetEnvAtomic(instance.DataDir, status.Target, status.Selected); err != nil {
		return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, manifest, "env_write_failed", "实例 .env 原子更新失败。")
	}
	manifest.ConfigWritten = true
	if err := writeRuntimeUpdateRecoveryManifest(instance.DataDir, manifest); err != nil {
		return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, manifest, "recovery_manifest_failed", "恢复清单写入失败。")
	}

	if err := setPhase(RuntimeUpdateApplyRecreatingAuth, 60, "正在单独重建新版 steam-auth-cn。"); err != nil {
		return err
	}
	if err := docker.RuntimeComposeUpService(ctx, instance.DataDir, manifest.Project, "steam-auth"); err != nil {
		return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, manifest, "auth_recreate_failed", "新版 steam-auth-cn 重建失败。")
	}
	manifest.AuthRecreated = true
	_ = writeRuntimeUpdateRecoveryManifest(instance.DataDir, manifest)
	if err := setPhase(RuntimeUpdateApplyVerifyingAuth, 68, "正在验证 steam-auth-cn 容器与服务接口。"); err != nil {
		return err
	}
	authState, err := d.waitRuntimeAuth(ctx, docker, instance.DataDir, manifest.Project, manifest.Target.SteamAuth.ImageID)
	if err != nil {
		return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, manifest, runtimeUpdateErrorCode(err), "新版 steam-auth-cn 认证验证失败。")
	}
	status.Checks = append(status.Checks, RuntimeUpdateDryRunCheck{Name: "steam_auth_ready", Status: "ok", Message: "新版 steam-auth-cn 容器运行、服务接口可解析，且镜像 digest 匹配目标。"})
	if !authState.Ready || !authState.HasTicket {
		status.Warnings = append(status.Warnings, "steam-auth-cn 当前未建立完整 Steam 在线会话；这不影响局域网模式或本次升级验收，需要邀请码时可稍后登录 Steam。")
	}

	if err := setPhase(RuntimeUpdateApplyRecreatingServer, 75, "正在重建同一推荐版本对的 Junimo server。"); err != nil {
		return err
	}
	(&lifecycleRunner{driver: d, lifecycle: docker, instance: stored}).clearRuntimeControlSnapshots(ctx, job)
	if err := docker.RuntimeComposeUpService(ctx, instance.DataDir, manifest.Project, "server"); err != nil {
		return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, manifest, "server_recreate_failed", "新版 server 重建失败。")
	}
	manifest.ServerRecreated = true
	_ = writeRuntimeUpdateRecoveryManifest(instance.DataDir, manifest)
	if err := setPhase(RuntimeUpdateApplyVerifyingServer, 85, "正在验证容器、Junimo、SMAPI 与控制契约。"); err != nil {
		return err
	}
	if err := d.verifyRuntimeTarget(ctx, docker, instance, manifest); err != nil {
		return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, manifest, runtimeUpdateErrorCode(err), "新版 Junimo server 运行验证失败。")
	}
	status.Checks = append(status.Checks, RuntimeUpdateDryRunCheck{Name: "junimo_runtime", Status: "ok", Message: "server/auth digest、容器健康、Junimo health/API 与控制契约均已验证；邀请码不属于升级硬门槛。"})

	if err := setPhase(RuntimeUpdateApplyRestoringState, 95, "正在恢复升级前的运行/停止状态。"); err != nil {
		return err
	}
	if err := d.restoreRuntimeRunningState(ctx, job, docker, instance, manifest); err != nil {
		return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, manifest, "restore_state_failed", "无法恢复升级前运行状态。")
	}
	if err := docker.RuntimeRemoveSnapshotVolume(ctx, instance.DataDir, manifest.Project, manifest.SnapshotVolume); err != nil {
		status.Warnings = append(status.Warnings, "升级成功，但临时认证快照清理失败；请人工检查私有快照卷。")
	}
	_ = os.RemoveAll(runtimeUpdateRecoveryDir(instance.DataDir, manifest.ApplyID))
	cleanupOldRuntimeImages(ctx, docker, instance.DataDir, manifest, &status)
	return finish(RuntimeUpdateApplySucceeded, "", "Junimo server + steam-auth-cn 已作为一个版本对完成升级。")
}

func cleanupOldRuntimeImages(ctx context.Context, docker RuntimeUpdateApplyDockerService, dataDir string, manifest runtimeUpdateRecoveryManifest, status *RuntimeUpdateApplyStatus) {
	pairs := []struct {
		name     string
		original RuntimeUpdateSelectedImage
		target   RuntimeUpdateSelectedImage
	}{
		{name: "Junimo server", original: manifest.OriginalServer, target: manifest.Target.Server},
		{name: "steam-auth-cn", original: manifest.OriginalAuth, target: manifest.Target.SteamAuth},
	}
	for _, pair := range pairs {
		if pair.original.Image == "" || pair.original.Image == pair.target.Image || pair.original.ImageID == pair.target.ImageID {
			continue
		}
		if err := docker.RuntimeRemoveImage(ctx, dataDir, pair.original.Image, pair.original.ImageID); err != nil {
			status.Warnings = append(status.Warnings, pair.name+" 旧镜像仍被其他容器引用、tag 已变化或删除失败；已保留供管理员检查。")
			continue
		}
		status.Logs = append(status.Logs, RuntimeUpdateDryRunLog{At: time.Now().UTC().Format(time.RFC3339), Level: "info", Message: "已删除升级前的 " + pair.name + " 镜像引用。"})
	}
}

func (d *Driver) runtimeUpdateApplyPreflight(ctx context.Context, job *jobs.Context, docker RuntimeUpdateApplyDockerService, instance registry.Instance, status *RuntimeUpdateApplyStatus) (runtimeUpdatePreflight, error) {
	if changed, err := EnsureServerContEnvFix(instance.DataDir); err != nil {
		return runtimeUpdatePreflight{}, errors.New("compose_compatibility_migration_failed")
	} else if changed {
		_, _ = job.Info(ctx, "已补齐低资源启动调度权重与现有 Junimo 运行兼容配置。")
	}
	inspection := InspectManagedRuntimeStack(instance.DataDir, instance.State)
	if inspection.Status != sjconfig.RuntimeStackStatusUpdateAvailable {
		return runtimeUpdatePreflight{}, &RuntimeUpdateValidationError{Code: inspection.Code, Message: inspection.Reason}
	}
	manifest, err := sjconfig.BuiltInRuntimeStackManifest()
	if err != nil || !manifest.Installable() || !sjconfig.PanelVersionSatisfies(d.panelVersion, manifest.MinimumPanelVersion) {
		return runtimeUpdatePreflight{}, &RuntimeUpdateValidationError{Code: "manifest_invalid", Message: "内置推荐版本对无效或未测试。"}
	}
	if _, err := docker.DockerVersion(ctx, instance.DataDir); err != nil {
		return runtimeUpdatePreflight{}, errors.New("docker_unavailable")
	}
	if _, err := docker.ComposeVersion(ctx, instance.DataDir); err != nil {
		return runtimeUpdatePreflight{}, errors.New("compose_unavailable")
	}
	project := strings.ToLower(filepath.Base(filepath.Clean(instance.DataDir)))
	if !filepath.IsAbs(instance.DataDir) || !runtimeComposeProjectPattern.MatchString(project) {
		return runtimeUpdatePreflight{}, errors.New("compose_project_unsafe")
	}
	composePath := filepath.Join(instance.DataDir, "docker-compose.yml")
	composeFile, err := os.Lstat(composePath)
	if err != nil || !composeFile.Mode().IsRegular() || composeFile.Mode()&os.ModeSymlink != 0 {
		return runtimeUpdatePreflight{}, errors.New("compose_file_unsafe")
	}
	compose, err := docker.RuntimeComposeConfigInspect(ctx, instance.DataDir, project)
	if err != nil || compose.Project != "" && compose.Project != project || !containsRuntimeService(compose.Services, "server") || !containsRuntimeService(compose.Services, "steam-auth") || compose.SteamSessionVolume == "" {
		return runtimeUpdatePreflight{}, errors.New("compose_config_invalid")
	}
	ps, err := docker.ComposePs(ctx, instance.DataDir)
	if err != nil {
		return runtimeUpdatePreflight{}, errors.New("runtime_state_unavailable")
	}
	status.ServerWasRunning = composeServiceRunning(ps.Services, "server")
	status.ServerRunning = status.ServerWasRunning
	authWasRunning := composeServiceRunning(ps.Services, "steam-auth")
	server, err := currentRuntimeImage(ctx, docker, instance.DataDir, project, "server", inspection.Current.Server.Image, status.ServerWasRunning)
	if err != nil {
		return runtimeUpdatePreflight{}, errors.New("current_server_digest_unavailable")
	}
	auth, err := currentRuntimeImage(ctx, docker, instance.DataDir, project, "steam-auth", inspection.Current.SteamAuth.Image, authWasRunning)
	if err != nil {
		return runtimeUpdatePreflight{}, errors.New("current_auth_digest_unavailable")
	}
	if _, err := docker.RuntimeVolumeInspect(ctx, instance.DataDir, compose.SteamSessionVolume); err != nil {
		return runtimeUpdatePreflight{}, errors.New("steam_session_volume_missing")
	}
	pullProgress := func(component string, base, span int) func(string) {
		return makeImagePullLineHandler(job, "[runtime-update:"+component+":pull] ", func(done, total int) {
			if total <= 0 {
				return
			}
			percent := done * 100 / total
			status.Download = &RuntimeUpdateDownloadProgress{Component: component, DoneLayers: done, TotalLayers: total, Percent: percent}
			status.Progress = base + percent*span/100
			status.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
			_ = writeRuntimeUpdateApplyStatus(instance.DataDir, *status)
		})
	}
	targetServer, code := selectRuntimeUpdateImageWithProgress(ctx, docker, instance.DataDir, inspection.Recommended.Server.TrustedCandidates, inspection.Recommended.Server.Digests, pullProgress("server", 5, 7))
	if code != "" {
		return runtimeUpdatePreflight{}, errors.New(code)
	}
	status.Download = &RuntimeUpdateDownloadProgress{Component: "server", Image: targetServer.Image, DoneLayers: 1, TotalLayers: 1, Percent: 100}
	targetAuth, code := selectRuntimeUpdateImageWithProgress(ctx, docker, instance.DataDir, inspection.Recommended.SteamAuth.TrustedCandidates, inspection.Recommended.SteamAuth.Digests, pullProgress("steam-auth-cn", 12, 7))
	if code != "" {
		return runtimeUpdatePreflight{}, errors.New(code)
	}
	status.Download = &RuntimeUpdateDownloadProgress{Component: "steam-auth-cn", Image: targetAuth.Image, DoneLayers: 1, TotalLayers: 1, Percent: 100}
	if err := docker.RuntimeComposeConfigValidateImages(ctx, instance.DataDir, project, targetServer.Image, targetAuth.Image); err != nil {
		return runtimeUpdatePreflight{}, errors.New("compose_target_validation_failed")
	}
	status.Current, status.Target = inspection.Current, inspection.Recommended
	status.Warnings = append(status.Warnings, "Docker 数据盘精确可用空间无法可靠判断；升级未伪造磁盘空间数值。")
	if reader, ok := docker.(interface {
		RuntimeHostCapacity(context.Context, string) (paneldocker.RuntimeHostCapacity, error)
	}); ok {
		if capacity, capacityErr := reader.RuntimeHostCapacity(ctx, instance.DataDir); capacityErr == nil && (capacity.CPUs <= 2 || capacity.MemoryBytes < 3*1024*1024*1024) {
			status.Warnings = append(status.Warnings, fmt.Sprintf("检测到低资源 Docker 主机（%d CPU，%.1f GiB 内存）；server 冷启动验收会持续等待最多 %v。若宿主机禁用换页，请先在宿主机配置可用 swap/swappiness。", capacity.CPUs, float64(capacity.MemoryBytes)/(1024*1024*1024), d.runtimeUpdateServerTimeout))
		}
	}
	return runtimeUpdatePreflight{project: project, volume: compose.SteamSessionVolume, originalServer: server, originalAuth: auth, target: RuntimeUpdateSelectedPair{Server: targetServer, SteamAuth: targetAuth}, authWasRunning: authWasRunning}, nil
}

func currentRuntimeImage(ctx context.Context, docker RuntimeUpdateApplyDockerService, dataDir, project, service, configuredImage string, running bool) (RuntimeUpdateSelectedImage, error) {
	if running {
		metadata, err := docker.RuntimeServiceInspect(ctx, dataDir, project, service)
		if err != nil || !strings.EqualFold(metadata.State, "running") || !runtimeImageDigestPattern.MatchString(metadata.ImageID) {
			return RuntimeUpdateSelectedImage{}, errors.New("running container image unavailable")
		}
		image := strings.TrimSpace(metadata.Image)
		if image == "" {
			image = configuredImage
		}
		return RuntimeUpdateSelectedImage{Image: image, Digest: metadata.ImageID, ImageID: metadata.ImageID}, nil
	}
	metadata, err := docker.RuntimeImageInspect(ctx, dataDir, configuredImage)
	if err != nil || !runtimeImageDigestPattern.MatchString(metadata.Digest) || !runtimeImageDigestPattern.MatchString(metadata.ID) {
		return RuntimeUpdateSelectedImage{}, errors.New("configured image unavailable")
	}
	return RuntimeUpdateSelectedImage{Image: configuredImage, Digest: metadata.Digest, ImageID: metadata.ID}, nil
}

func createRuntimeRecoveryFiles(dataDir string, manifest runtimeUpdateRecoveryManifest) error {
	dir := runtimeUpdateRecoveryDir(dataDir, manifest.ApplyID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	for source, name := range map[string]string{filepath.Join(dataDir, ".env"): "original.env", filepath.Join(dataDir, "docker-compose.yml"): "original-compose.yml"} {
		data, err := os.ReadFile(source)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			return err
		}
	}
	return writeRuntimeUpdateRecoveryManifest(dataDir, manifest)
}

func backupRuntimeControlMod(dataDir, applyID string) (bool, bool, error) {
	recoveryDir := runtimeUpdateRecoveryDir(dataDir, applyID)
	if err := os.MkdirAll(recoveryDir, 0o700); err != nil {
		return false, false, err
	}
	present := map[string]bool{}
	for _, name := range []string{"manifest.json", "StardewAnxiPanel.Control.dll"} {
		source := filepath.Join(smapiModDir(dataDir), name)
		data, err := os.ReadFile(source)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return false, false, err
		}
		if err := os.WriteFile(filepath.Join(recoveryDir, "original-control-"+name), data, 0o600); err != nil {
			return false, false, err
		}
		present[name] = true
	}
	return present["manifest.json"], present["StardewAnxiPanel.Control.dll"], nil
}

func restoreRuntimeControlMod(dataDir string, manifest runtimeUpdateRecoveryManifest) error {
	dir := smapiModDir(dataDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for name, present := range map[string]bool{"manifest.json": manifest.ControlManifestPresent, "StardewAnxiPanel.Control.dll": manifest.ControlDLLPresent} {
		target := filepath.Join(dir, name)
		if !present {
			if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			continue
		}
		data, err := os.ReadFile(filepath.Join(runtimeUpdateRecoveryDir(dataDir, manifest.ApplyID), "original-control-"+name))
		if err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func writeRuntimeTargetEnvAtomic(dataDir string, target sjconfig.RuntimeStackRecommendation, selected RuntimeUpdateSelectedPair) error {
	envPath := filepath.Join(dataDir, ".env")
	data, err := os.ReadFile(envPath)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dataDir, ".runtime-update-env-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(tmpName)
	if err := os.WriteFile(tmpName, data, 0o600); err != nil {
		return err
	}
	values := map[string]string{"IMAGE_VERSION": target.Server.Tag, "SERVER_IMAGE": selected.Server.Image, "SERVER_IMAGE_CANDIDATES": strings.Join(target.Server.TrustedCandidates, ","), "STEAM_SERVICE_IMAGE": selected.SteamAuth.Image, "STEAM_SERVICE_IMAGE_CANDIDATES": strings.Join(target.SteamAuth.TrustedCandidates, ",")}
	if err := sjconfig.UpdateEnvFile(tmpName, values); err != nil {
		return err
	}
	return replaceRuntimeUpdateStatusFile(tmpName, envPath)
}

func (d *Driver) waitRuntimeAuth(ctx context.Context, docker RuntimeUpdateApplyDockerService, dataDir, project, imageID string) (paneldocker.RuntimeSteamReady, error) {
	deadline := time.Now().Add(d.runtimeUpdateAuthTimeout)
	var last paneldocker.RuntimeSteamReady
	for time.Now().Before(deadline) {
		metadata, err := docker.RuntimeServiceInspect(ctx, dataDir, project, "steam-auth")
		if err == nil && metadata.ImageID != imageID {
			return last, errors.New("auth_digest_mismatch")
		}
		if err == nil && strings.EqualFold(metadata.State, "running") && (metadata.Health == "" || strings.EqualFold(metadata.Health, "healthy")) {
			last, err = docker.RuntimeSteamAuthReady(ctx, dataDir, project)
			if err == nil {
				return last, nil
			}
		}
		select {
		case <-ctx.Done():
			return last, ctx.Err()
		case <-time.After(d.runtimeUpdatePollInterval):
		}
	}
	return last, errors.New("auth_service_not_ready")
}

func (d *Driver) verifyRuntimeTarget(ctx context.Context, docker RuntimeUpdateApplyDockerService, instance registry.Instance, manifest runtimeUpdateRecoveryManifest) error {
	if _, err := d.waitRuntimeAuth(ctx, docker, instance.DataDir, manifest.Project, manifest.Target.SteamAuth.ImageID); err != nil {
		return err
	}
	deadline := time.Now().Add(d.runtimeUpdateServerTimeout)
	lastFailure := "server_container_not_ready"
	for time.Now().Before(deadline) {
		metadata, err := docker.RuntimeServiceInspect(ctx, instance.DataDir, manifest.Project, "server")
		if err == nil && metadata.ImageID != manifest.Target.Server.ImageID {
			return errors.New("server_digest_mismatch")
		}
		if err != nil || !strings.EqualFold(metadata.State, "running") || metadata.Health != "" && !strings.EqualFold(metadata.Health, "healthy") {
			lastFailure = "server_container_not_ready"
		} else if docker.RuntimeServerHealth(ctx, instance.DataDir, manifest.Project) != nil {
			lastFailure = "junimo_health_not_ready"
		} else {
			controlState := readSMAPIStatus(instance.DataDir)
			if controlState != "launched" && controlState != "save-loaded" {
				lastFailure = "smapi_runtime_not_ready"
			} else if !commandResultSupported(instance.DataDir) {
				lastFailure = "control_contract_not_ready"
			} else if !runtimeInfoContractReady(ctx, docker, instance.DataDir, manifest.TargetServerVersion) {
				lastFailure = "junimo_contract_not_ready"
			} else if !runningControlMatchesManifest(instance.DataDir) {
				lastFailure = "control_runtime_version_mismatch"
			} else {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(d.runtimeUpdatePollInterval):
		}
	}
	return errors.New(lastFailure)
}

func runningControlMatchesManifest(dataDir string) bool {
	manifest, err := sjconfig.BuiltInRuntimeStackManifest()
	if err != nil {
		return false
	}
	raw, err := os.ReadFile(filepath.Join(dataDir, ".local-container", "control", "options.json"))
	if err != nil {
		return false
	}
	var options struct {
		ControlModVersion string `json:"controlModVersion"`
	}
	if json.Unmarshal(raw, &options) != nil || strings.TrimSpace(options.ControlModVersion) != manifest.Control.Version {
		return false
	}
	installedDLL, err := os.ReadFile(filepath.Join(smapiModDir(dataDir), "StardewAnxiPanel.Control.dll"))
	if err != nil {
		return false
	}
	return bytes.Equal(installedDLL, smapiModDLL) && verifyEmbeddedControlManifest(manifest) == nil
}

func waitForRunningControlManifest(ctx context.Context, dataDir string, timeout time.Duration) bool {
	if timeout <= 0 {
		timeout = time.Minute
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		if runningControlMatchesManifest(dataDir) {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-deadline.C:
			return false
		case <-ticker.C:
		}
	}
}

// runtimeInfoContractReady verifies the same FIFO-backed control path used by
// normal Panel console commands. Junimo's attach-cli is a tmux UI and rejects
// docker compose exec -T with "not a terminal", so it must not be used as a
// one-shot health probe.
func runtimeInfoContractReady(ctx context.Context, docker RuntimeUpdateApplyDockerService, dataDir, expectedVersion string) bool {
	sizeResult, err := docker.ComposeExecPipe(ctx, dataDir, "server", "", "wc", "-c", serverOutputLog)
	if err != nil {
		return false
	}
	fields := strings.Fields(sizeResult.Stdout)
	if len(fields) == 0 {
		return false
	}
	offset, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil || offset < 0 {
		return false
	}
	if _, err := docker.ComposeExecPipe(ctx, dataDir, "server", "info\n", "tee", "-a", serverInputFIFO); err != nil {
		return false
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		result, tailErr := docker.ComposeExecPipe(ctx, dataDir, "server", "", "tail", "-c", fmt.Sprintf("+%d", offset+1), serverOutputLog)
		output := stripControlChars(result.Stdout)
		versionReady := strings.TrimSpace(expectedVersion) == "" || strings.Contains(output, "Version: "+expectedVersion)
		if tailErr == nil && strings.Contains(output, "--- Server Info ---") && versionReady {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(250 * time.Millisecond):
		}
	}
}

func (d *Driver) restoreRuntimeRunningState(ctx context.Context, job *jobs.Context, docker RuntimeUpdateApplyDockerService, instance registry.Instance, manifest runtimeUpdateRecoveryManifest) error {
	if manifest.ServerWasRunning {
		d.updatePhase(ctx, instance.ID, storage.InstanceStateRunning, "运行组件升级完成，服务器正在运行", "running", job.ID)
		return nil
	}
	if err := d.stopRuntimeServicesWithRetry(ctx, docker, instance.DataDir, manifest.Project, "server", "steam-auth"); err != nil {
		return err
	}
	d.updatePhase(ctx, instance.ID, storage.InstanceStateStopped, "运行组件升级验证完成，已恢复停止状态", "stopped", job.ID)
	return nil
}

func runtimeUpdateErrorCode(err error) string {
	if v, ok := IsRuntimeUpdateValidationError(err); ok {
		return v.Code
	}
	code := strings.TrimSpace(err.Error())
	if code != "" && !strings.ContainsAny(code, " \r\n\t:") {
		return code
	}
	return "runtime_update_failed"
}
