package updater

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type ServiceOptions struct {
	Docker           DockerRuntime
	DataDir          string
	ContainerRef     string
	ContainerDataDir string
	HostInstallDir   string
	HostComposeFile  string
	HostDataDir      string
	ComposeProject   string
	Logger           *slog.Logger
	Now              func() time.Time
	Database         DatabaseBackupper
	DatabasePath     string
}

type DatabaseBackupper interface {
	BackupTo(context.Context, string) error
}

type Service struct {
	docker       DockerRuntime
	store        *StateStore
	applyStore   *ApplyStateStore
	detect       DetectOptions
	logger       *slog.Logger
	now          func() time.Time
	mu           sync.Mutex
	dataDir      string
	databasePath string
	database     DatabaseBackupper
}

func NewService(opts ServiceOptions) *Service {
	docker := opts.Docker
	if docker == nil {
		docker = NewDockerCLI()
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	dataDir := strings.TrimSpace(opts.DataDir)
	if dataDir == "" {
		dataDir = "/data"
	}
	return &Service{
		docker:     docker,
		store:      NewStateStore(filepath.Join(dataDir, "updater", "status.json")),
		applyStore: NewApplyStateStore(filepath.Join(dataDir, "updater", "apply-status.json")),
		detect: DetectOptions{
			ContainerRef: opts.ContainerRef, ContainerDataDir: opts.ContainerDataDir,
			HostInstallDir: opts.HostInstallDir, HostComposeFile: opts.HostComposeFile,
			HostDataDir: opts.HostDataDir, ComposeProject: opts.ComposeProject,
		},
		logger:       logger,
		now:          now,
		dataDir:      dataDir,
		databasePath: filepath.Clean(opts.DatabasePath),
		database:     opts.Database,
	}
}

func (s *Service) ApplyStatus() (ApplyStatus, error) { return s.applyStore.Read() }

// ReconcileCompletedImageCleanup lets a newly started Panel finish the image
// cleanup for an upgrade whose helper came from the previous release. The old
// helper can recreate and verify the new Panel, but cannot contain cleanup
// logic introduced by that new release.
func (s *Service) ReconcileCompletedImageCleanup(ctx context.Context, currentVersion string) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		done, err := s.reconcileCompletedImageCleanupOnce(ctx, currentVersion, ExecDocker{})
		if err != nil {
			s.logger.Warn("failed to reconcile completed panel image cleanup", "error", err)
			return
		}
		if done {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Service) reconcileCompletedImageCleanupOnce(ctx context.Context, currentVersion string, executor ApplyExecutor) (bool, error) {
	status, err := s.applyStore.Read()
	if err != nil {
		if errors.Is(err, ErrNoApplyStatus) {
			return true, nil
		}
		return true, err
	}
	current, normalizeErr := NormalizeTargetVersion(currentVersion)
	if normalizeErr != nil || status.ToVersion != current {
		return true, nil
	}
	if status.CleanupCompleted {
		return true, nil
	}
	if status.Phase != PhaseSucceeded {
		if IsActiveApplyPhase(status.Phase) {
			return false, nil
		}
		return true, nil
	}
	if status.OriginalImage == "" || status.OriginalDigest == "" || status.SelectedImage == "" || status.SelectedDigest == "" {
		return true, errors.New("completed panel update is missing image cleanup metadata")
	}
	cleanupPanelImages(ctx, executor, status.OriginalImage, status.OriginalDigest, status.SelectedImage, status.SelectedDigest, &status)
	status.CleanupCompleted = true
	status.UpdatedAt = s.now().UTC()
	if err := s.applyStore.Write(status); err != nil {
		return true, err
	}
	return true, nil
}

func (s *Service) StartApply(ctx context.Context, currentVersion, latestVersion string) (ApplyStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, err := s.applyStore.Read(); err == nil && IsActiveApplyPhase(existing.Phase) {
		return existing, ValidationError{Code: CodeUpdateInProgress, Message: "已有面板升级任务正在执行"}
	}
	comparison, err := CompareStableVersions(currentVersion, latestVersion)
	if err != nil {
		return ApplyStatus{}, ValidationError{Code: CodeInvalidTargetVersion, Message: "当前版本或目标版本不是可升级的正式版本"}
	}
	if comparison >= 0 {
		return ApplyStatus{}, ValidationError{Code: CodeUpdateNotAvailable, Message: "目标版本必须高于当前版本"}
	}
	fromVersion, _ := NormalizeTargetVersion(currentVersion)
	toVersion, _ := NormalizeTargetVersion(latestVersion)
	now := s.now().UTC()
	status := ApplyStatus{
		UpdateID: newDryRunID(), Phase: PhaseChecking, Progress: 5,
		FromVersion: fromVersion, ToVersion: toVersion, Logs: []LogEntry{}, StartedAt: now, UpdatedAt: now,
	}
	status.Logs = append(status.Logs, LogEntry{At: now, Level: "info", Message: "正在验证升级目标和部署能力"})
	if err := s.applyStore.Write(status); err != nil {
		return ApplyStatus{}, err
	}
	capability := s.Capability(ctx)
	if !capability.Supported {
		return s.abortBeforeHelper(status, capability.Code, capability.Reason), ValidationError{Code: capability.Code, Message: capability.Reason}
	}
	status.OriginalImage = capability.CurrentImage
	digest, err := s.docker.ImageDigest(ctx, capability.CurrentImage)
	if err != nil {
		aborted := s.abortBeforeHelper(status, "original_digest_unavailable", "无法保存当前面板镜像 digest")
		return aborted, ValidationError{Code: aborted.ErrorCode, Message: aborted.Error}
	}
	status.OriginalDigest = digest
	dbRelative, err := filepath.Rel(s.dataDir, s.databasePath)
	if err != nil || dbRelative == "." || filepath.IsAbs(dbRelative) || strings.HasPrefix(filepath.Clean(dbRelative), "..") {
		aborted := s.abortBeforeHelper(status, CodeDatabaseBackupFailed, "面板数据库不在可验证的数据目录内")
		return aborted, ValidationError{Code: aborted.ErrorCode, Message: aborted.Error}
	}
	backupDir := filepath.Join(s.dataDir, "updater", "backups", status.UpdateID)
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		aborted := s.abortBeforeHelper(status, CodeDatabaseBackupFailed, "无法创建升级备份目录")
		return aborted, ValidationError{Code: aborted.ErrorCode, Message: aborted.Error}
	}
	_ = os.Chmod(backupDir, 0o700)
	status.Phase, status.Progress, status.UpdatedAt = PhaseBackingUp, 15, s.now().UTC()
	status.Logs = append(status.Logs, LogEntry{At: status.UpdatedAt, Level: "info", Message: "正在创建 SQLite 一致性在线备份"})
	if err := s.applyStore.Write(status); err != nil {
		return ApplyStatus{}, err
	}
	if s.database == nil || s.database.BackupTo(ctx, filepath.Join(backupDir, "panel.db")) != nil {
		aborted := s.abortBeforeHelper(status, CodeDatabaseBackupFailed, "数据库一致性备份失败，升级已取消")
		return aborted, ValidationError{Code: aborted.ErrorCode, Message: aborted.Error}
	}
	status.Logs = append(status.Logs, LogEntry{At: s.now().UTC(), Level: "info", Message: "数据库一致性备份完成，正在移交独立 updater helper"})
	if err := s.applyStore.Write(status); err != nil {
		return ApplyStatus{}, err
	}
	spec := ApplyHelperSpec{
		Name: "anxi-panel-updater-apply-" + status.UpdateID, RuntimeImage: capability.CurrentImage,
		FromVersion: fromVersion, TargetVersion: toVersion, OriginalDigest: digest,
		CurrentContainer: capability.CurrentContainer, ComposeProject: capability.ComposeProject,
		HostInstallDir: capability.InstallDir, HostComposeFile: capability.ComposeFile,
		DataMount: capability.DataMount, StateFile: "/data/updater/apply-status.json",
		BackupDir: "/data/updater/backups/" + status.UpdateID, DatabaseRelativePath: filepath.ToSlash(dbRelative),
	}
	if err := s.docker.StartApplyHelper(ctx, spec); err != nil {
		s.logger.Warn("failed to start panel updater apply helper", "error", err)
		aborted := s.abortBeforeHelper(status, CodeHelperStartFailed, "无法启动独立升级 helper，当前面板未改变")
		return aborted, ValidationError{Code: aborted.ErrorCode, Message: aborted.Error}
	}
	return status, nil
}

func (s *Service) abortBeforeHelper(status ApplyStatus, code, message string) ApplyStatus {
	finished := s.now().UTC()
	status.Phase, status.Progress, status.ErrorCode, status.Error = PhaseFailedRolledBack, 100, code, message
	status.Result = "升级在修改部署前中止，原面板保持不变"
	status.UpdatedAt, status.FinishedAt = finished, &finished
	status.Logs = append(status.Logs, LogEntry{At: finished, Level: "error", Message: message})
	_ = s.applyStore.Write(status)
	return status
}

func (s *Service) Capability(ctx context.Context) Capability {
	return DetectCapability(ctx, s.docker, s.detect)
}

func (s *Service) Status() (DryRunStatus, error) { return s.store.Read() }

func (s *Service) StartDryRun(ctx context.Context, targetVersion string) (DryRunStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, err := s.store.Read(); err == nil && (existing.Phase == "starting" || existing.Phase == "running") {
		return existing, nil
	}
	normalized, err := NormalizeTargetVersion(targetVersion)
	if err != nil {
		return DryRunStatus{}, ValidationError{Code: CodeInvalidTargetVersion, Message: err.Error()}
	}
	capability := s.Capability(ctx)
	now := s.now().UTC()
	status := DryRunStatus{
		ID: newDryRunID(), Phase: "starting", TargetVersion: normalized, Capability: capability,
		Logs: []LogEntry{}, StartedAt: now, UpdatedAt: now,
	}
	if !capability.Supported {
		status.Phase = "unsupported"
		status.ErrorCode = capability.Code
		status.Error = capability.Reason
		status.Logs = append(status.Logs, LogEntry{At: now, Level: "error", Message: capability.Reason})
		status.FinishedAt = &now
		_ = s.store.Write(status)
		return status, nil
	}
	if _, err := TrustedImageCandidates(normalized, capability.CurrentImage); err != nil {
		return DryRunStatus{}, ValidationError{Code: CodeImageNotAllowed, Message: err.Error()}
	}
	status.Logs = append(status.Logs, LogEntry{At: now, Level: "info", Message: "部署环境识别通过，正在启动独立升级演练 helper"})
	status.Phase = "running"
	if err := s.store.Write(status); err != nil {
		return DryRunStatus{}, err
	}
	spec := HelperSpec{
		Name: "anxi-panel-updater-" + status.ID, RuntimeImage: capability.CurrentImage,
		TargetVersion: normalized, ComposeProject: capability.ComposeProject,
		HostInstallDir: capability.InstallDir, HostComposeFile: capability.ComposeFile,
		DataMount: capability.DataMount, StateFile: "/data/updater/status.json",
	}
	if err := s.docker.StartHelper(ctx, spec); err != nil {
		finished := s.now().UTC()
		status.Phase = "failed"
		status.ErrorCode = CodeHelperStartFailed
		status.Error = "无法启动独立升级演练 helper"
		status.UpdatedAt, status.FinishedAt = finished, &finished
		status.Logs = append(status.Logs, LogEntry{At: finished, Level: "error", Message: status.Error})
		_ = s.store.Write(status)
		s.logger.Warn("failed to start panel updater helper", "error", err)
		return status, nil
	}
	if latest, err := s.store.Read(); err == nil {
		return latest, nil
	}
	return status, nil
}

type ValidationError struct{ Code, Message string }

func (e ValidationError) Error() string { return e.Message }

func IsValidationError(err error) (ValidationError, bool) {
	var target ValidationError
	ok := errors.As(err, &target)
	return target, ok
}

func newDryRunID() string {
	var raw [6]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return time.Now().UTC().Format("20060102150405")
	}
	return hex.EncodeToString(raw[:])
}
