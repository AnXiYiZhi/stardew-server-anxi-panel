package web

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

const (
	defaultControlCommandRetentionDays  = 30
	defaultControlCommandRetentionCount = 1000
	stuckQueuedCommandAge               = 30 * time.Second
)

func (s *server) recordControlCommandSubmission(ctx context.Context, actor currentSession, instanceID string, result *sj.CommandRunResult, targetType, targetID, targetLabel string) {
	if result == nil || strings.TrimSpace(result.CommandID) == "" {
		return
	}
	status := string(result.Status)
	supported := status != ""
	if !supported {
		status = string(sj.CommandStatusDispatched)
	}
	now := time.Now().UTC()
	actorID := actor.User.ID
	err := s.store.CreateControlCommand(ctx, storage.CreateControlCommandParams{
		CommandID: result.CommandID, InstanceID: instanceID, CommandType: result.Command,
		TargetType: targetType, TargetID: safeCommandField(targetID, 128), TargetLabel: safeCommandField(targetLabel, 128),
		ActorUserID: &actorID, ActorUsername: safeCommandField(actor.User.Username, 128),
		Status: status, ResultSupported: supported, SubmittedAt: now,
	})
	if err != nil {
		s.logger.Error("failed to persist control command submission", "command_id", result.CommandID, "error", err)
		return
	}
	metadata := auditMetadata("commandId", result.CommandID, "commandType", result.Command, "targetType", targetType, "targetId", targetID, "targetLabel", targetLabel, "status", status)
	if err := s.store.CreateAuditLog(ctx, storage.AuditLogParams{
		ActorUserID: &actorID, Action: "control_command_submitted", TargetType: "control_command",
		TargetID: result.CommandID, Metadata: metadata,
	}); err != nil {
		s.logger.Error("failed to write control command submission audit", "command_id", result.CommandID, "error", err)
	}
}

func safeCommandField(value string, max int) string {
	value = strings.TrimSpace(value)
	if len([]rune(value)) > max {
		return string([]rune(value)[:max])
	}
	return value
}

func safeCommandDetails(details map[string]string) map[string]string {
	result := map[string]string{}
	for _, key := range []string{"playerId", "playerName"} {
		if value := safeCommandField(details[key], 128); value != "" {
			result[key] = value
		}
	}
	return result
}

func safeCommandResultMessage(value string) string {
	lower := strings.ToLower(value)
	for _, marker := range []string{"password", "passwd", "steam credential", "access_token", "refresh_token", "bearer ", "token="} {
		if strings.Contains(lower, marker) {
			return "[敏感结果信息已脱敏]"
		}
	}
	return safeCommandField(value, 500)
}

func syncControlCommandResults(ctx context.Context, store *storage.Store, instance storage.Instance, logger *slog.Logger) error {
	files, err := sj.ListCommandResultFiles(instance.DataDir)
	if err != nil {
		return err
	}
	for _, file := range files {
		o := file.Outcome
		if err := store.ImportControlCommandResult(ctx, storage.ImportControlCommandResultParams{
			CommandID: o.CommandID, InstanceID: instance.ID, Status: string(o.Status), ErrorCode: safeCommandField(o.ErrorCode, 128),
			ResultMessage: safeCommandResultMessage(o.Message), ResultDetails: safeCommandDetails(o.Details),
			CreatedAt: o.CreatedAt, UpdatedAt: o.UpdatedAt, ImportedAt: time.Now().UTC(),
		}); err != nil {
			return err
		}
		if err := sj.DeleteImportedCommandResult(instance.DataDir, file); err != nil {
			logger.Warn("failed to remove imported command result", "command_id", o.CommandID, "error", err)
		}
	}
	return nil
}

func (s *server) handleControlCommandHistory(w http.ResponseWriter, r *http.Request, instanceID string) {
	if _, ok := s.requireAuth(w, r); !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}
	if err := syncControlCommandResults(r.Context(), s.store, instance, s.logger); err != nil {
		s.logger.Warn("control command result sync failed", "instance", instanceID, "error", err)
	}
	limit := 50
	if value := r.URL.Query().Get("limit"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 && parsed <= 200 {
			limit = parsed
		}
	}
	commands, err := s.store.ListControlCommands(r.Context(), instanceID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "control_commands_read_failed", "读取控制命令历史失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"commands": commands})
}

func (s *server) commandProtocolDiagnostics(ctx context.Context, instance storage.Instance) map[string]any {
	_ = syncControlCommandResults(ctx, s.store, instance, s.logger)
	d := sj.InspectCommandQueue(instance.DataDir)
	files, _ := sj.ListCommandResultFiles(instance.DataDir)
	unimported := 0
	for _, file := range files {
		ok, err := s.store.HasImportedControlCommandResult(ctx, file.Outcome.CommandID, file.Outcome.UpdatedAt)
		if err != nil || !ok {
			unimported++
		}
	}
	latest, _ := s.store.LatestControlCommandUpdate(ctx, instance.ID)
	warnings := make([]string, 0)
	if instance.State == storage.InstanceStateRunning && d.PendingCommandCount > 0 && d.OldestPendingAt != nil && time.Since(*d.OldestPendingAt) > stuckQueuedCommandAge {
		warnings = append(warnings, "服务器运行中，但有控制命令长时间未被消费")
	}
	if d.CommandResultVersion < sj.CommandResultVersion {
		warnings = append(warnings, "控制模组版本过旧，无法获取精确命令结果")
	}
	if !d.CommandsWritable {
		warnings = append(warnings, "commands 目录不可写")
	}
	if !d.ResultsWritable {
		warnings = append(warnings, "command-results 目录不可写")
	}
	return map[string]any{
		"commandResultVersion":    d.CommandResultVersion,
		"pendingCommandCount":     d.PendingCommandCount,
		"unimportedResultCount":   unimported,
		"oldestPendingAt":         d.OldestPendingAt,
		"lastControlModConsumeAt": latest,
		"commandsWritable":        d.CommandsWritable,
		"commandResultsWritable":  d.ResultsWritable,
		"warnings":                warnings,
	}
}

type ControlCommandSchedulerDeps struct {
	Store          *storage.Store
	Logger         *slog.Logger
	RetentionDays  int
	RetentionCount int
}
type ControlCommandScheduler struct {
	store                         *storage.Store
	logger                        *slog.Logger
	retentionDays, retentionCount int
	lastCleanup                   time.Time
}

func NewControlCommandScheduler(deps ControlCommandSchedulerDeps) *ControlCommandScheduler {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	retentionDays := deps.RetentionDays
	if retentionDays <= 0 {
		retentionDays = defaultControlCommandRetentionDays
	}
	retentionCount := deps.RetentionCount
	if retentionCount <= 0 {
		retentionCount = defaultControlCommandRetentionCount
	}
	return &ControlCommandScheduler{store: deps.Store, logger: logger, retentionDays: retentionDays, retentionCount: retentionCount}
}

func (s *ControlCommandScheduler) Run(ctx context.Context) {
	s.runOnce(ctx)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runOnce(ctx)
		}
	}
}

func (s *ControlCommandScheduler) runOnce(ctx context.Context) {
	instances, err := s.store.ListInstances(ctx)
	if err != nil {
		s.logger.Warn("list instances for command result sync failed", "error", err)
		return
	}
	for _, instance := range instances {
		if instance.DriverID != sj.DriverID {
			continue
		}
		if err := syncControlCommandResults(ctx, s.store, instance, s.logger); err != nil {
			s.logger.Warn("background command result sync failed", "instance", instance.ID, "error", err)
		}
	}
	_, _ = s.store.MarkStaleControlCommandsUnknown(ctx, time.Now().UTC().Add(-5*time.Minute))
	now := time.Now().UTC()
	if s.lastCleanup.IsZero() || now.Sub(s.lastCleanup) >= time.Hour {
		if _, err := s.store.CleanupControlCommands(ctx, now.AddDate(0, 0, -s.retentionDays), s.retentionCount); err != nil {
			s.logger.Warn("control command history cleanup failed", "error", err)
		} else {
			s.lastCleanup = now
		}
	}
}

func commandOutcomeFromStorage(c storage.ControlCommand) sj.CommandOutcome {
	return sj.CommandOutcome{CommandID: c.CommandID, Status: sj.CommandStatus(c.Status), ErrorCode: c.ErrorCode,
		Message: c.ResultMessage, CreatedAt: c.SubmittedAt, UpdatedAt: c.UpdatedAt, Details: c.ResultDetails}
}

func (s *server) persistedCommandOutcome(ctx context.Context, instance storage.Instance, commandID string) (sj.CommandOutcome, error) {
	if err := syncControlCommandResults(ctx, s.store, instance, s.logger); err != nil {
		return sj.CommandOutcome{}, err
	}
	c, err := s.store.GetControlCommand(ctx, commandID)
	if err == nil {
		if c.InstanceID != instance.ID {
			return sj.CommandOutcome{}, fmt.Errorf("control command not found")
		}
		return commandOutcomeFromStorage(c), nil
	}
	if !errors.Is(err, storage.ErrNotFound) {
		return sj.CommandOutcome{}, err
	}
	return sj.CommandOutcome{}, fmt.Errorf("control command not found")
}
