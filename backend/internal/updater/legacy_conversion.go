package updater

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"
)

type LegacyConversionOptions struct {
	FromVersion, TargetVersion   string
	CurrentImage, OriginalDigest string
	CurrentContainer, StateFile  string
	BackupDir, DatabaseRelative  string
	ScriptPath                   string
	Now                          func() time.Time
}

// RunLegacyConversion hands cutover to an independent helper process. The
// shell implementation performs a second complete Docker inspection, writes a
// standard Compose deployment, retains the old container under a recovery
// name, and restores it if exact health/version verification fails.
func RunLegacyConversion(ctx context.Context, opts LegacyConversionOptions) error {
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	if opts.ScriptPath == "" {
		opts.ScriptPath = "/app/migrate-fnos.sh"
	}
	store := NewApplyStateStore(opts.StateFile)
	status, err := store.Read()
	if err != nil {
		return err
	}
	fail := func(phase, code, message, result string) error {
		finished := now().UTC()
		status.Phase, status.Progress, status.ErrorCode, status.Error, status.Result = phase, 100, code, message, result
		status.UpdatedAt, status.FinishedAt = finished, &finished
		status.Logs = append(status.Logs, LogEntry{At: finished, Level: "error", Message: message})
		if writeErr := store.Write(status); writeErr != nil {
			return writeErr
		}
		return errors.New(message)
	}
	from, err := NormalizeTargetVersion(opts.FromVersion)
	if err != nil {
		return fail(PhaseFailedRolledBack, CodeInvalidTargetVersion, "旧 Panel 版本无效", "当前容器未修改")
	}
	to, err := NormalizeTargetVersion(opts.TargetVersion)
	if err != nil {
		return fail(PhaseFailedRolledBack, CodeInvalidTargetVersion, "目标 Panel 版本无效", "当前容器未修改")
	}
	stateFrom, fromErr := NormalizeTargetVersion(status.FromVersion)
	stateTo, toErr := NormalizeTargetVersion(status.ToVersion)
	expectedBackupDir := "/data/updater/backups/" + status.UpdateID
	if fromErr != nil || toErr != nil || stateFrom != from || stateTo != to || status.OriginalImage != opts.CurrentImage || status.OriginalDigest != opts.OriginalDigest || opts.BackupDir != expectedBackupDir || status.Phase != PhaseBackingUp {
		return fail(PhaseFailedRolledBack, CodeComposeMetadataInvalid, "持久化升级状态与 helper 参数不一致", "当前容器未修改")
	}
	if !containerReferencePattern.MatchString(opts.CurrentContainer) || strings.TrimSpace(opts.CurrentImage) == "" || strings.ContainsAny(opts.CurrentImage, "\r\n") || !strings.HasPrefix(opts.OriginalDigest, "sha256:") || len(opts.OriginalDigest) != 71 {
		return fail(PhaseFailedRolledBack, CodeComposeMetadataInvalid, "旧容器身份或镜像 digest 无效", "当前容器未修改")
	}
	if !strings.HasPrefix(opts.BackupDir, "/data/updater/backups/") || opts.DatabaseRelative == "" || strings.HasPrefix(opts.DatabaseRelative, "/") || strings.Contains(opts.DatabaseRelative, "..") {
		return fail(PhaseFailedRolledBack, CodeDatabaseBackupFailed, "数据库恢复参数不安全", "当前容器未修改")
	}
	status.Phase, status.Progress, status.UpdatedAt = PhaseRecreating, 45, now().UTC()
	status.Logs = append(status.Logs, LogEntry{At: status.UpdatedAt, Level: "info", Message: "独立 helper 正在把飞牛旧容器转换为标准 Compose 部署"})
	if err := store.Write(status); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "/bin/bash", opts.ScriptPath)
	cmd.Env = append(os.Environ(), "PANEL_CONTAINER="+opts.CurrentContainer, "TARGET_VERSION="+to, "YES=1",
		"EXPECTED_ORIGINAL_IMAGE="+opts.CurrentImage, "EXPECTED_ORIGINAL_DIGEST="+opts.OriginalDigest,
		"MIGRATION_DATABASE_BACKUP="+opts.BackupDir+"/panel.db", "MIGRATION_DATABASE_RELATIVE="+opts.DatabaseRelative)
	cmd.Stdout = &limitedDiscard{limit: 256 << 10}
	cmd.Stderr = &limitedDiscard{limit: 256 << 10}
	if err := cmd.Run(); err != nil {
		probeCtx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		if waitForPanel(probeCtx, ExecDocker{}, opts.CurrentContainer, from, 40*time.Second, 2*time.Second) == nil {
			return fail(PhaseFailedRolledBack, "legacy_conversion_failed", "标准部署转换失败，旧 Panel 已自动恢复并通过健康检查", "已恢复旧容器；迁移备份保留在 Panel 数据目录")
		}
		return fail(PhaseRollbackFailed, CodeRollbackFailed, "标准部署转换失败，旧 Panel 未能通过自动恢复验证", "请保留旧容器和 updater/fnos-migration 恢复材料并人工处理")
	}
	finished := now().UTC()
	status.Phase, status.Progress, status.Result = PhaseSucceeded, 100, "已转换为标准 Compose 部署并完成 Panel 健康、版本校验"
	status.ErrorCode, status.Error, status.UpdatedAt, status.FinishedAt = "", "", finished, &finished
	status.Logs = append(status.Logs, LogEntry{At: finished, Level: "info", Message: "标准部署转换与 Panel 更新成功；新 Panel 将继续全实例运行栈校验"})
	return store.Write(status)
}
