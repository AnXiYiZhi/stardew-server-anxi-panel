package stardew_junimo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func (d *Driver) rollbackRuntimeUpdate(ctx context.Context, job *jobs.Context, docker RuntimeUpdateApplyDockerService, instance registry.Instance, status *RuntimeUpdateApplyStatus, manifest runtimeUpdateRecoveryManifest, causeCode, causeMessage string) error {
	status.Phase, status.Progress, status.ErrorCode, status.Error = RuntimeUpdateApplyRollingBack, 90, causeCode, paneldocker.RedactString(causeMessage)
	status.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	status.Logs = append(status.Logs, RuntimeUpdateDryRunLog{At: status.UpdatedAt, Level: "warning", Message: "升级验收失败，正在成对回滚 server 与 steam-auth-cn。"})
	_ = writeRuntimeUpdateApplyStatus(instance.DataDir, *status)
	_, _ = job.Warn(ctx, "升级验收失败，正在成对回滚 server 与 steam-auth-cn。")

	rollbackErr := d.performRuntimeUpdateRollback(ctx, job, docker, instance, manifest)
	now := time.Now().UTC().Format(time.RFC3339)
	status.Progress, status.UpdatedAt, status.FinishedAt = 100, now, now
	if rollbackErr != nil {
		status.Phase = RuntimeUpdateApplyRollbackFailed
		status.ErrorCode = "rollback_failed"
		status.Error = "自动回滚未能完成；私有恢复材料已保留。"
		status.ManualAction = "停止对该实例执行更新操作，保留 .local-container/junimo-update/recovery 下的私有材料，并由管理员核对原镜像 digest、.env、Compose 与 steam-session 快照后人工恢复。"
		status.Logs = append(status.Logs, RuntimeUpdateDryRunLog{At: now, Level: "error", Message: status.Error})
		_ = writeRuntimeUpdateApplyStatus(instance.DataDir, *status)
		d.auditRuntimeUpdateTerminal(ctx, instance.ID, *status)
		return errors.New("自动回滚未能完成；请按 apply 状态中的人工指引处理。")
	}
	status.Phase = RuntimeUpdateApplyFailedRolledBack
	status.ErrorCode, status.Error = causeCode, paneldocker.RedactString(causeMessage)
	status.ManualAction = ""
	status.ServerRunning = manifest.ServerWasRunning
	status.Logs = append(status.Logs, RuntimeUpdateDryRunLog{At: now, Level: "warning", Message: "升级失败，但原 server/auth 版本对、认证卷与运行状态已恢复。"})
	_ = writeRuntimeUpdateApplyStatus(instance.DataDir, *status)
	d.auditRuntimeUpdateTerminal(ctx, instance.ID, *status)
	_ = docker.RuntimeRemoveSnapshotVolume(ctx, instance.DataDir, manifest.Project, manifest.SnapshotVolume)
	_ = os.RemoveAll(runtimeUpdateRecoveryDir(instance.DataDir, manifest.ApplyID))
	return errors.New(causeMessage)
}

func (d *Driver) performRuntimeUpdateRollback(ctx context.Context, job *jobs.Context, docker RuntimeUpdateApplyDockerService, instance registry.Instance, manifest runtimeUpdateRecoveryManifest) error {
	if manifest.ConfigWritten || manifest.AuthRecreated || manifest.ServerRecreated {
		if err := docker.RuntimeComposeStopServices(ctx, instance.DataDir, manifest.Project, "server", "steam-auth"); err != nil {
			return fmt.Errorf("stop new runtime pair: %w", err)
		}
	}
	if err := restoreRuntimeRecoveryFile(instance.DataDir, manifest.ApplyID, "original.env", ".env"); err != nil {
		return fmt.Errorf("restore env: %w", err)
	}
	if err := restoreRuntimeRecoveryFile(instance.DataDir, manifest.ApplyID, "original-compose.yml", "docker-compose.yml"); err != nil {
		return fmt.Errorf("restore compose: %w", err)
	}
	if err := pinRuntimeRollbackImages(instance.DataDir, manifest); err != nil {
		return fmt.Errorf("pin original runtime images: %w", err)
	}
	if manifest.AuthRecreated {
		if err := docker.RuntimeRestoreVolume(ctx, instance.DataDir, manifest.SnapshotVolume, manifest.SteamSessionVolume, manifest.OriginalServer.Image); err != nil {
			return fmt.Errorf("restore steam session: %w", err)
		}
	}
	if err := docker.RuntimeComposeUpService(ctx, instance.DataDir, manifest.Project, "steam-auth"); err != nil {
		return fmt.Errorf("recreate old auth: %w", err)
	}
	if err := d.waitRuntimeAuth(ctx, docker, instance.DataDir, manifest.Project, manifest.OriginalAuth.ImageID); err != nil {
		return fmt.Errorf("verify old auth: %w", err)
	}
	if err := docker.RuntimeComposeUpService(ctx, instance.DataDir, manifest.Project, "server"); err != nil {
		return fmt.Errorf("recreate old server: %w", err)
	}
	if err := d.verifyRuntimeOriginalServer(ctx, docker, instance, manifest); err != nil {
		return err
	}
	if manifest.ServerWasRunning {
		d.updatePhase(ctx, instance.ID, storage.InstanceStateRunning, "运行组件升级失败，已回滚并恢复运行", "running", job.ID)
		return nil
	}
	if err := docker.RuntimeComposeStopServices(ctx, instance.DataDir, manifest.Project, "server", "steam-auth"); err != nil {
		return fmt.Errorf("restore stopped state: %w", err)
	}
	d.updatePhase(ctx, instance.ID, storage.InstanceStateStopped, "运行组件升级失败，已回滚并恢复停止", "stopped", job.ID)
	return nil
}

func pinRuntimeRollbackImages(dataDir string, manifest runtimeUpdateRecoveryManifest) error {
	envPath := filepath.Join(dataDir, ".env")
	if !runtimeImageDigestPattern.MatchString(manifest.OriginalServer.ImageID) || !runtimeImageDigestPattern.MatchString(manifest.OriginalAuth.ImageID) {
		return errors.New("original runtime image IDs are invalid")
	}
	return sjconfig.UpdateEnvFile(envPath, map[string]string{
		"SERVER_IMAGE":        manifest.OriginalServer.ImageID,
		"STEAM_SERVICE_IMAGE": manifest.OriginalAuth.ImageID,
	})
}

func (d *Driver) verifyRuntimeOriginalServer(ctx context.Context, docker RuntimeUpdateApplyDockerService, instance registry.Instance, manifest runtimeUpdateRecoveryManifest) error {
	deadline := time.Now().Add(d.runtimeUpdateServerTimeout)
	for time.Now().Before(deadline) {
		metadata, err := docker.RuntimeServiceInspect(ctx, instance.DataDir, manifest.Project, "server")
		if err == nil && metadata.ImageID != manifest.OriginalServer.ImageID {
			return errors.New("rollback server digest mismatch")
		}
		if err == nil && strings.EqualFold(metadata.State, "running") && docker.RuntimeServerHealth(ctx, instance.DataDir, manifest.Project) == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(d.runtimeUpdatePollInterval):
		}
	}
	return errors.New("rollback Junimo verification failed")
}

func restoreRuntimeRecoveryFile(dataDir, applyID, backupName, targetName string) error {
	source := filepath.Join(runtimeUpdateRecoveryDir(dataDir, applyID), backupName)
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	target := filepath.Join(dataDir, targetName)
	tmp, err := os.CreateTemp(dataDir, ".runtime-restore-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(0o600); err != nil {
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
	return replaceRuntimeUpdateStatusFile(name, target)
}
