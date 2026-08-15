package stardew_junimo

import (
	"context"
	"crypto/sha256"
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
	project, volume                string
	originalServer, originalAuth   RuntimeUpdateSelectedImage
	target                         RuntimeUpdateSelectedPair
	authWasRunning, controlChanged bool
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
		status.ResumeAfterRepair = false
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
		if phase == RuntimeUpdateApplySucceeded {
			if _, err := d.ReadRequiredRuntimeUpdateStatus(instance); err != nil && !errors.Is(err, os.ErrNotExist) {
				d.logger.Warn("reconcile required runtime status after successful apply", "instance", instance.ID, "error", err)
			}
		}
		return nil
	}

	if recovery != nil {
		if recovery.KeepServerStopped {
			status.ServerRunning = false
			status.ManualAction = "恢复已安全收敛；请确认状态后在面板中手动启动游戏服务器。"
			return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, *recovery, "panel_restart_recovery", "Panel 重启后已回滚中断的运行组件升级；游戏保持关闭。")
		}
		if !runtimeUpdateMutationStarted(*recovery) {
			if err := finish(RuntimeUpdateApplyFailedRolledBack, "panel_restart_before_change", "Panel 重启发生在实例修改前；实例保持原状。"); err != nil {
				return err
			}
			_ = os.RemoveAll(runtimeUpdateRecoveryDir(instance.DataDir, recovery.ApplyID))
			return nil
		}
		if status.RepairAttempts > 0 && (status.Phase == RuntimeUpdateApplyRollingBack || status.Phase == RuntimeUpdateApplyResumingUpgrade) {
			return d.runRuntimeUpdateRepair(ctx, job, docker, instance, status, *recovery)
		}
		if (!runtimeUpdateAuthChanged(*recovery) || runtimeUpdateAuthMayHaveBeenRecreated(*recovery)) && runtimeUpdateServerMayHaveBeenRecreated(*recovery) && (status.Phase == RuntimeUpdateApplyVerifyingServer || status.Phase == RuntimeUpdateApplyRestoringState) {
			if err := d.verifyRuntimeTarget(ctx, docker, instance, *recovery); err == nil {
				if err := d.restoreRuntimeRunningState(ctx, job, docker, instance, *recovery); err == nil {
					if err := finish(RuntimeUpdateApplySucceeded, "", "Panel 重启后已继续完成验收，Junimo 运行组件成对升级成功。"); err != nil {
						return err
					}
					if runtimeUpdateAuthSnapshotVolumeCreated(*recovery) {
						if err := docker.RuntimeRemoveSnapshotVolume(ctx, instance.DataDir, recovery.Project, recovery.SnapshotVolume); err != nil {
							status.Warnings = append(status.Warnings, "升级成功，但临时认证快照清理失败；Panel 下次启动会继续按事务精确名称清理。")
						}
					}
					cleanupOldRuntimeImages(ctx, docker, instance.DataDir, *recovery, &status)
					_ = writeRuntimeUpdateApplyStatus(instance.DataDir, status)
					_ = os.RemoveAll(runtimeUpdateRecoveryDir(instance.DataDir, recovery.ApplyID))
					return nil
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

	manifest := runtimeUpdateRecoveryManifest{SchemaVersion: 3, ApplyID: status.ApplyID, ActorID: status.CreatedBy, Project: preflight.project, SteamSessionVolume: preflight.volume, SnapshotVolume: preflight.project + "_anxi-junimo-update-" + strings.TrimPrefix(status.ApplyID, "apply_") + "-steam-session", ServerWasRunning: status.ServerWasRunning, AuthWasRunning: preflight.authWasRunning, ServerImageChanged: preflight.originalServer.ImageID != preflight.target.Server.ImageID || status.Current.Server.Tag != status.Target.Server.Tag, AuthImageChanged: preflight.originalAuth.ImageID != preflight.target.SteamAuth.ImageID || status.Current.SteamAuth.Tag != status.Target.SteamAuth.Tag, OriginalState: instance.State, OriginalServer: preflight.originalServer, OriginalAuth: preflight.originalAuth, Target: preflight.target, OriginalServerVersion: status.Current.Server.Tag, TargetServerVersion: status.Target.Server.Tag}
	changePlan := []string{}
	if preflight.controlChanged {
		changePlan = append(changePlan, "Control")
	}
	junimoModNeedsSync := validateExtractedJunimoServerMod(junimoServerModDir(instance.DataDir), manifest.TargetServerVersion) != nil
	if runtimeUpdateServerChanged(manifest) || junimoModNeedsSync {
		changePlan = append(changePlan, "server")
	}
	if runtimeUpdateAuthChanged(manifest) {
		changePlan = append(changePlan, "steam-auth-cn")
	}
	if len(changePlan) == 0 {
		changePlan = append(changePlan, "配置与控制契约复核")
	}
	status.Checks = append(status.Checks, RuntimeUpdateDryRunCheck{Name: "change_plan", Status: "ok", Message: "已按当前 tag 与实际 image ID 计算差异组件：" + strings.Join(changePlan, "、") + "；未变化的认证服务不会重建。"})
	if err := setPhase(RuntimeUpdateApplyBackingUp, 30, "正在创建私有恢复材料并计算需要更新的运行组件。"); err != nil {
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
	if manifest.OriginalEnvSHA256, err = runtimeRecoveryFileSHA256(filepath.Join(runtimeUpdateRecoveryDir(instance.DataDir, manifest.ApplyID), "original.env")); err != nil {
		return err
	}
	if manifest.OriginalComposeSHA256, err = runtimeRecoveryFileSHA256(filepath.Join(runtimeUpdateRecoveryDir(instance.DataDir, manifest.ApplyID), "original-compose.yml")); err != nil {
		return err
	}
	if manifest.ControlManifestPresent {
		if manifest.OriginalControlJSONSHA, err = runtimeRecoveryFileSHA256(filepath.Join(runtimeUpdateRecoveryDir(instance.DataDir, manifest.ApplyID), "original-control-manifest.json")); err != nil {
			return err
		}
	}
	if manifest.ControlDLLPresent {
		if manifest.OriginalControlDLLSHA, err = runtimeRecoveryFileSHA256(filepath.Join(runtimeUpdateRecoveryDir(instance.DataDir, manifest.ApplyID), "original-control-StardewAnxiPanel.Control.dll")); err != nil {
			return err
		}
	}
	if err := writeRuntimeUpdateRecoveryManifest(instance.DataDir, manifest); err != nil {
		return err
	}
	stored, err := d.store.GetInstance(ctx, instance.ID)
	if err != nil {
		if finishErr := finish(RuntimeUpdateApplyFailedRolledBack, "instance_reload_failed", "无法重新读取实例；实例未修改。"); finishErr != nil {
			return finishErr
		}
		_ = os.RemoveAll(runtimeUpdateRecoveryDir(instance.DataDir, manifest.ApplyID))
		return err
	}
	servicesToStop := []string{}
	if manifest.ServerWasRunning {
		servicesToStop = append(servicesToStop, "server")
	}
	if runtimeUpdateAuthChanged(manifest) && manifest.AuthWasRunning {
		servicesToStop = append(servicesToStop, "steam-auth")
	}
	if len(servicesToStop) > 0 {
		if err := setPhase(RuntimeUpdateApplyStopping, 40, "正在按组件差异安全停止需要更新的运行服务。"); err != nil {
			return err
		}
		manifest.MutationStarted, manifest.StopIntent, manifest.LastIntent = true, true, "stop_services"
		if err := writeRuntimeUpdateRecoveryManifest(instance.DataDir, manifest); err != nil {
			_ = finish(RuntimeUpdateApplyFailedRolledBack, "recovery_manifest_failed", "无法在停服前持久化恢复意图；实例未修改。")
			_ = os.RemoveAll(runtimeUpdateRecoveryDir(instance.DataDir, manifest.ApplyID))
			return err
		}
		// From this durable mutation intent onward, a restart must use the
		// transaction's normal rollback path rather than start a second retry.
		// The on-disk resume marker may still be true until the next status write;
		// RecoverRuntimeUpdateApply explicitly treats that combination as rollback.
		status.ResumeAfterRepair = false
		if err := d.stopRuntimeServicesWithRetry(ctx, docker, instance.DataDir, manifest.Project, servicesToStop...); err != nil {
			return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, manifest, "stop_failed", "安全停服失败。")
		}
		if manifest.ServerWasRunning {
			d.updatePhase(ctx, instance.ID, storage.InstanceStateStopped, "运行组件升级中，游戏服务已停止", "runtime_update_stopped", job.ID)
		}
	}
	if !manifest.MutationStarted {
		manifest.MutationStarted, manifest.LastIntent = true, "control_update"
	}
	manifest.ControlUpdateIntent, manifest.LastIntent = true, "control_update"
	if err := writeRuntimeUpdateRecoveryManifest(instance.DataDir, manifest); err != nil {
		return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, manifest, "recovery_manifest_failed", "无法在同步 Control 前持久化恢复意图。")
	}
	status.ResumeAfterRepair = false
	// Control is a host bind. Replace it only after the game process has fully
	// stopped so a live CLR process can never observe a half-updated DLL.
	if err := installSMAPIMod(instance.DataDir); err != nil {
		return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, manifest, "control_sync_failed", "Control Mod 同步失败。")
	}
	manifest.ControlUpdated = true
	if err := writeRuntimeUpdateRecoveryManifest(instance.DataDir, manifest); err != nil {
		return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, manifest, "control_recovery_manifest_failed", "Control 已同步但恢复清单写入失败。")
	}
	if runtimeUpdateAuthChanged(manifest) {
		// Clone only after steam-auth has quiesced. An unchanged running auth
		// container is deliberately left untouched and needs no session snapshot.
		manifest.AuthSnapshotCreateIntent, manifest.LastIntent = true, "create_auth_snapshot"
		if err := writeRuntimeUpdateRecoveryManifest(instance.DataDir, manifest); err != nil {
			return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, manifest, "recovery_manifest_failed", "无法在创建认证快照前持久化恢复意图。")
		}
		if err := docker.RuntimeCreateSnapshotVolume(ctx, instance.DataDir, manifest.Project, manifest.SnapshotVolume); err != nil {
			return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, manifest, "auth_snapshot_create_failed", "无法创建 Steam 认证卷临时快照。")
		}
		manifest.AuthSnapshotVolumeMade = true
		if err := writeRuntimeUpdateRecoveryManifest(instance.DataDir, manifest); err != nil {
			return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, manifest, "recovery_manifest_failed", "认证快照卷已创建但恢复清单写入失败。")
		}
		if err := docker.RuntimeCloneVolume(ctx, instance.DataDir, manifest.SteamSessionVolume, manifest.SnapshotVolume, manifest.Target.Server.Image); err != nil {
			return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, manifest, "auth_snapshot_failed", "Steam 认证卷保护失败。")
		}
		manifest.AuthSnapshotCreated = true
		status.Checks = append(status.Checks, RuntimeUpdateDryRunCheck{Name: "steam_session_snapshot", Status: "ok", Message: "steam-session 已在认证服务停稳后克隆到当前 Compose project 限定的临时 Docker volume；未读取 token 内容。"})
		if err := writeRuntimeUpdateRecoveryManifest(instance.DataDir, manifest); err != nil {
			return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, manifest, "recovery_manifest_failed", "恢复清单写入失败。")
		}
	} else {
		status.Checks = append(status.Checks, RuntimeUpdateDryRunCheck{Name: "steam_auth_preserved", Status: "ok", Message: "steam-auth-cn 镜像与版本未变化；保留现有容器和认证卷，不执行快照或重建。"})
	}

	if err := setPhase(RuntimeUpdateApplyWritingConfig, 50, "正在事务化同步目标 JunimoServer Mod 并写入推荐版本对配置。"); err != nil {
		return err
	}
	if runtimeUpdateServerChanged(manifest) || junimoModNeedsSync {
		recoveryDir := runtimeUpdateRecoveryDir(instance.DataDir, manifest.ApplyID)
		hostRecoveryDir, pathErr := d.dockerHostPath(recoveryDir)
		if pathErr != nil {
			return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, manifest, "junimo_mod_bind_path_failed", "无法映射 JunimoServer Mod 提取目录。")
		}
		extractedDir, err := extractJunimoServerMod(ctx, docker, manifest.Target.Server.Image, recoveryDir, hostRecoveryDir, manifest.TargetServerVersion)
		if err != nil {
			return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, manifest, "junimo_mod_extract_failed", "无法从目标 server 镜像提取并验证 JunimoServer Mod。")
		}
		manifest.JunimoModPrepared = true
		if err := writeRuntimeUpdateRecoveryManifest(instance.DataDir, manifest); err != nil {
			return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, manifest, "recovery_manifest_failed", "恢复清单写入失败。")
		}
		if _, statErr := os.Stat(junimoServerModDir(instance.DataDir)); statErr == nil {
			manifest.JunimoModOriginalPresent = true
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, manifest, "junimo_mod_directory_invalid", "无法确认宿主 JunimoServer Mod 原始状态。")
		}
		manifest.JunimoModReplaceIntent, manifest.LastIntent = true, "replace_junimo_mod"
		if err := writeRuntimeUpdateRecoveryManifest(instance.DataDir, manifest); err != nil {
			return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, manifest, "recovery_manifest_failed", "无法在替换 JunimoServer Mod 前持久化恢复意图。")
		}
		var originalPresent bool
		originalPresent, err = replaceJunimoServerMod(instance.DataDir, extractedDir, filepath.Join(recoveryDir, runtimeOriginalJunimoDir))
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
		if originalPresent != manifest.JunimoModOriginalPresent {
			return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, manifest, "junimo_mod_original_state_changed", "JunimoServer Mod 在升级期间被外部修改。")
		}
		manifest.JunimoModReplaced = true
		if err := writeRuntimeUpdateRecoveryManifest(instance.DataDir, manifest); err != nil {
			return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, manifest, "recovery_manifest_failed", "恢复清单写入失败。")
		}
	}
	manifest.ConfigWriteIntent, manifest.LastIntent = true, "write_target_config"
	if err := writeRuntimeUpdateRecoveryManifest(instance.DataDir, manifest); err != nil {
		return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, manifest, "recovery_manifest_failed", "无法在写入目标配置前持久化恢复意图。")
	}
	if err := writeRuntimeTargetEnvAtomic(instance.DataDir, status.Target, status.Selected); err != nil {
		return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, manifest, "env_write_failed", "实例 .env 原子更新失败。")
	}
	manifest.ConfigWritten = true
	if err := writeRuntimeUpdateRecoveryManifest(instance.DataDir, manifest); err != nil {
		return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, manifest, "recovery_manifest_failed", "恢复清单写入失败。")
	}

	authMessage := "steam-auth-cn 版本未变化，正在确认原容器保持就绪。"
	if runtimeUpdateAuthChanged(manifest) {
		authMessage = "正在单独重建新版 steam-auth-cn。"
	}
	if err := setPhase(RuntimeUpdateApplyRecreatingAuth, 60, authMessage); err != nil {
		return err
	}
	if runtimeUpdateAuthChanged(manifest) {
		manifest.AuthRecreateIntent, manifest.LastIntent = true, "recreate_auth"
		if err := writeRuntimeUpdateRecoveryManifest(instance.DataDir, manifest); err != nil {
			return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, manifest, "recovery_manifest_failed", "无法在重建 steam-auth-cn 前持久化恢复意图。")
		}
		if err := docker.RuntimeComposeUpService(ctx, instance.DataDir, manifest.Project, "steam-auth"); err != nil {
			return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, manifest, "auth_recreate_failed", "新版 steam-auth-cn 重建失败。")
		}
		manifest.AuthRecreated = true
	} else if !manifest.AuthWasRunning {
		manifest.AuthServiceStartIntent, manifest.LastIntent = true, "start_preserved_auth"
		if err := writeRuntimeUpdateRecoveryManifest(instance.DataDir, manifest); err != nil {
			return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, manifest, "recovery_manifest_failed", "无法在启动保留的 steam-auth-cn 前持久化恢复意图。")
		}
		if err := docker.RuntimeComposeUpServicePreserve(ctx, instance.DataDir, manifest.Project, "steam-auth"); err != nil {
			return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, manifest, "auth_start_failed", "未变化的 steam-auth-cn 无法启动用于验收。")
		}
	}
	if !runtimeUpdateAuthChanged(manifest) {
		if err := docker.RuntimeUpdateServiceCPUShares(ctx, instance.DataDir, manifest.Project, "steam-auth", 256); err != nil {
			return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, manifest, "auth_resource_update_failed", "未变化的 steam-auth-cn 无法原地应用资源权重。")
		}
	}
	if err := writeRuntimeUpdateRecoveryManifest(instance.DataDir, manifest); err != nil {
		return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, manifest, "recovery_manifest_failed", "steam-auth-cn 已启动但恢复清单写入失败。")
	}
	if err := setPhase(RuntimeUpdateApplyVerifyingAuth, 68, "正在验证 steam-auth-cn 容器、目标 digest 与纯服务健康接口；Steam 登录状态不属于升级硬门槛。"); err != nil {
		return err
	}
	authState, err := d.waitRuntimeAuth(ctx, docker, instance.DataDir, manifest.Project, manifest.Target.SteamAuth.ImageID)
	if err != nil {
		return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, manifest, runtimeUpdateErrorCode(err), runtimeUpdateErrorMessage(err, "steam-auth-cn 服务健康验收失败。"))
	}
	status.Checks = append(status.Checks, RuntimeUpdateDryRunCheck{Name: "steam_auth_ready", Status: "ok", Message: "steam-auth-cn 容器 running、镜像 digest 精确匹配，且 /health 返回受支持的 HTTP 200 严格 JSON 契约；Docker health 与 Steam 在线登录均不作为该服务健康验收的替代条件。"})
	if !authState.LoggedIn {
		status.Warnings = append(status.Warnings, "steam-auth-cn 当前未建立完整 Steam 在线会话，服务会在后台继续尝试连接 Steam；这不影响局域网模式或本次升级验收，需要邀请码时可稍后登录 Steam。")
	}

	if err := setPhase(RuntimeUpdateApplyRecreatingServer, 75, "正在重建同一推荐版本对的 Junimo server。"); err != nil {
		return err
	}
	(&lifecycleRunner{driver: d, lifecycle: docker, instance: stored}).clearRuntimeControlSnapshots(ctx, job)
	manifest.ServerRecreateIntent, manifest.LastIntent = true, "recreate_server"
	if err := writeRuntimeUpdateRecoveryManifest(instance.DataDir, manifest); err != nil {
		return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, manifest, "recovery_manifest_failed", "无法在重建 Junimo server 前持久化恢复意图。")
	}
	if err := docker.RuntimeComposeUpService(ctx, instance.DataDir, manifest.Project, "server"); err != nil {
		return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, manifest, "server_recreate_failed", "新版 server 重建失败。")
	}
	manifest.ServerRecreated = true
	if err := writeRuntimeUpdateRecoveryManifest(instance.DataDir, manifest); err != nil {
		return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, manifest, "recovery_manifest_failed", "Junimo server 已重建但恢复清单写入失败。")
	}
	if err := setPhase(RuntimeUpdateApplyVerifyingServer, 85, "正在验证容器、Junimo、SMAPI 与控制契约。"); err != nil {
		return err
	}
	if err := d.verifyRuntimeTarget(ctx, docker, instance, manifest); err != nil {
		return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, manifest, runtimeUpdateErrorCode(err), runtimeUpdateErrorMessage(err, "新版 Junimo server 运行验证失败。"))
	}
	status.Checks = append(status.Checks, RuntimeUpdateDryRunCheck{Name: "junimo_runtime", Status: "ok", Message: "server/auth digest、容器运行态、steam-auth /health、Junimo health/API 与控制契约均已验证；Steam 登录和邀请码不属于升级硬门槛。"})

	if err := setPhase(RuntimeUpdateApplyRestoringState, 95, "正在恢复升级前的运行/停止状态。"); err != nil {
		return err
	}
	if err := d.restoreRuntimeRunningState(ctx, job, docker, instance, manifest); err != nil {
		return d.rollbackRuntimeUpdate(ctx, job, docker, instance, &status, manifest, "restore_state_failed", "无法恢复升级前运行状态。")
	}
	if err := finish(RuntimeUpdateApplySucceeded, "", "Junimo server + steam-auth-cn 已作为一个版本对完成升级。"); err != nil {
		return err
	}
	if runtimeUpdateAuthSnapshotVolumeCreated(manifest) {
		if err := docker.RuntimeRemoveSnapshotVolume(ctx, instance.DataDir, manifest.Project, manifest.SnapshotVolume); err != nil {
			status.Warnings = append(status.Warnings, "升级成功，但临时认证快照清理失败；Panel 下次启动会继续按事务精确名称清理。")
		}
	}
	cleanupOldRuntimeImages(ctx, docker, instance.DataDir, manifest, &status)
	_ = writeRuntimeUpdateApplyStatus(instance.DataDir, status)
	_ = os.RemoveAll(runtimeUpdateRecoveryDir(instance.DataDir, manifest.ApplyID))
	return nil
}

func runtimeRecoveryFileSHA256(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("runtime recovery material is not a regular file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum[:]), nil
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
	return runtimeUpdatePreflight{project: project, volume: compose.SteamSessionVolume, originalServer: server, originalAuth: auth, target: RuntimeUpdateSelectedPair{Server: targetServer, SteamAuth: targetAuth}, authWasRunning: authWasRunning, controlChanged: !runningControlMatchesManifest(instance.DataDir)}, nil
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

func (d *Driver) waitRuntimeAuth(ctx context.Context, docker RuntimeUpdateApplyDockerService, dataDir, project, imageID string) (paneldocker.RuntimeAuthServiceHealth, error) {
	deadline := time.Now().Add(d.runtimeUpdateAuthTimeout)
	var last paneldocker.RuntimeAuthServiceHealth
	lastErr := runtimeAuthAcceptanceError("auth_container_not_running", "steam-auth-cn 容器未处于 running 状态。")
	for time.Now().Before(deadline) {
		metadata, err := docker.RuntimeServiceInspect(ctx, dataDir, project, "steam-auth")
		if err == nil && metadata.ImageID != imageID {
			return last, runtimeAuthAcceptanceError("auth_digest_mismatch", "steam-auth-cn 实际 image ID 与目标 digest 不匹配。")
		}
		if err != nil || !strings.EqualFold(metadata.State, "running") {
			lastErr = runtimeAuthAcceptanceError("auth_container_not_running", "steam-auth-cn 容器未处于 running 状态。")
		} else {
			last, err = docker.RuntimeSteamAuthHealth(ctx, dataDir, project)
			if err == nil {
				return last, nil
			}
			lastErr = normalizeRuntimeAuthHealthError(err)
		}
		select {
		case <-ctx.Done():
			return last, ctx.Err()
		case <-time.After(d.runtimeUpdatePollInterval):
		}
	}
	return last, lastErr
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

func runtimeUpdateErrorMessage(err error, fallback string) string {
	if validation, ok := IsRuntimeUpdateValidationError(err); ok && strings.TrimSpace(validation.Message) != "" {
		return validation.Message
	}
	return fallback
}

func runtimeAuthAcceptanceError(code, message string) error {
	return &RuntimeUpdateValidationError{Code: code, Message: message}
}

func normalizeRuntimeAuthHealthError(err error) error {
	if validation, ok := IsRuntimeUpdateValidationError(err); ok {
		return validation
	}
	var probeError *paneldocker.RuntimeAuthHealthError
	if errors.As(err, &probeError) {
		switch probeError.Code {
		case "auth_health_unreachable", "auth_health_timeout", "auth_health_http_status", "auth_health_invalid_response":
			return runtimeAuthAcceptanceError(probeError.Code, probeError.Message)
		}
	}
	return runtimeAuthAcceptanceError("auth_health_unreachable", "steam-auth-cn /health 无法连接。")
}
