package stardew_junimo

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	CommandResultVersion = 1
	commandResultTTL     = 7 * 24 * time.Hour
	expiredTombstoneTTL  = 24 * time.Hour
	commandRunningStale  = 5 * time.Minute
)

type CommandStatus string

const (
	CommandStatusQueued     CommandStatus = "queued"
	CommandStatusRunning    CommandStatus = "running"
	CommandStatusSucceeded  CommandStatus = "succeeded"
	CommandStatusFailed     CommandStatus = "failed"
	CommandStatusDispatched CommandStatus = "dispatched"
	CommandStatusExpired    CommandStatus = "expired"
	CommandStatusUnknown    CommandStatus = "unknown"
)

type CommandOutcome struct {
	CommandID string            `json:"commandId"`
	Status    CommandStatus     `json:"status"`
	ErrorCode string            `json:"errorCode,omitempty"`
	Message   string            `json:"message,omitempty"`
	CreatedAt time.Time         `json:"createdAt,omitempty"`
	UpdatedAt time.Time         `json:"updatedAt"`
	Details   map[string]string `json:"details,omitempty"`
}

func commandResultsDir(dataDir string) string {
	return filepath.Join(controlDir(dataDir), "command-results")
}

func commandResultSupported(dataDir string) bool {
	var status struct {
		CommandResultVersion int `json:"commandResultVersion"`
	}
	raw, err := os.ReadFile(filepath.Join(controlDir(dataDir), "status.json"))
	return err == nil && json.Unmarshal(raw, &status) == nil && status.CommandResultVersion >= CommandResultVersion
}

func GetCommandOutcome(dataDir, commandID string) (CommandOutcome, error) {
	if !validCommandID(commandID) {
		return CommandOutcome{}, &CommandError{Code: "invalid_command_id", Message: "命令 ID 格式错误"}
	}
	resultPath := filepath.Join(commandResultsDir(dataDir), commandID+".json")
	if raw, err := os.ReadFile(resultPath); err == nil {
		var outcome CommandOutcome
		if err := json.Unmarshal(raw, &outcome); err != nil {
			return CommandOutcome{CommandID: commandID, Status: CommandStatusUnknown, ErrorCode: "result_invalid", UpdatedAt: time.Now().UTC()}, nil
		}
		return outcome, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return CommandOutcome{}, fmt.Errorf("read command result: %w", err)
	}
	queued, err := commandFileExists(dataDir, commandID)
	if err != nil {
		return CommandOutcome{}, err
	}
	status := CommandStatusUnknown
	if queued {
		status = CommandStatusQueued
	}
	return CommandOutcome{CommandID: commandID, Status: status, UpdatedAt: time.Now().UTC()}, nil
}

type CommandResultFile struct {
	Path    string
	Outcome CommandOutcome
}

type CommandQueueDiagnostics struct {
	CommandResultVersion int        `json:"commandResultVersion"`
	PendingCommandCount  int        `json:"pendingCommandCount"`
	ResultFileCount      int        `json:"resultFileCount"`
	OldestPendingAt      *time.Time `json:"oldestPendingAt,omitempty"`
	CommandsWritable     bool       `json:"commandsWritable"`
	ResultsWritable      bool       `json:"resultsWritable"`
}

func ListCommandResultFiles(dataDir string) ([]CommandResultFile, error) {
	dir := commandResultsDir(dataDir)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read command result files: %w", err)
	}
	files := make([]CommandResultFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read command result %s: %w", entry.Name(), err)
		}
		var outcome CommandOutcome
		if err := json.Unmarshal(raw, &outcome); err != nil || !validCommandID(outcome.CommandID) || !validCommandStatus(outcome.Status) {
			continue
		}
		if filepath.Clean(path) != filepath.Clean(filepath.Join(dir, outcome.CommandID+".json")) {
			continue
		}
		files = append(files, CommandResultFile{Path: path, Outcome: outcome})
	}
	return files, nil
}

func validCommandStatus(status CommandStatus) bool {
	switch status {
	case CommandStatusQueued, CommandStatusRunning, CommandStatusSucceeded, CommandStatusFailed,
		CommandStatusDispatched, CommandStatusExpired, CommandStatusUnknown:
		return true
	default:
		return false
	}
}

func DeleteImportedCommandResult(dataDir string, file CommandResultFile) error {
	expected := filepath.Join(commandResultsDir(dataDir), file.Outcome.CommandID+".json")
	if filepath.Clean(file.Path) != filepath.Clean(expected) {
		return fmt.Errorf("command result path mismatch")
	}
	if file.Outcome.Status == CommandStatusRunning {
		queued, err := commandFileExists(dataDir, file.Outcome.CommandID)
		if err != nil {
			return err
		}
		if queued {
			return nil // preserve the durable no-retry gate across container restarts
		}
	}
	if err := os.Remove(expected); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove imported command result: %w", err)
	}
	return nil
}

func InspectCommandQueue(dataDir string) CommandQueueDiagnostics {
	control := controlDir(dataDir)
	var status struct {
		CommandResultVersion int `json:"commandResultVersion"`
	}
	if raw, err := os.ReadFile(filepath.Join(control, "status.json")); err == nil {
		_ = json.Unmarshal(raw, &status)
	}
	d := CommandQueueDiagnostics{
		CommandResultVersion: status.CommandResultVersion,
		CommandsWritable:     directoryWritable(filepath.Join(control, "commands")),
		ResultsWritable:      directoryWritable(filepath.Join(control, "command-results")),
	}
	if entries, err := os.ReadDir(filepath.Join(control, "commands")); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
				continue
			}
			d.PendingCommandCount++
			raw, err := os.ReadFile(filepath.Join(control, "commands", entry.Name()))
			if err != nil {
				continue
			}
			var command struct {
				CreatedAt time.Time `json:"createdAt"`
			}
			if json.Unmarshal(raw, &command) == nil && !command.CreatedAt.IsZero() && (d.OldestPendingAt == nil || command.CreatedAt.Before(*d.OldestPendingAt)) {
				t := command.CreatedAt
				d.OldestPendingAt = &t
			}
		}
	}
	if entries, err := os.ReadDir(filepath.Join(control, "command-results")); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
				d.ResultFileCount++
			}
		}
	}
	return d
}

func directoryWritable(dir string) bool {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false
	}
	f, err := os.CreateTemp(dir, ".write-check-*")
	if err != nil {
		return false
	}
	path := f.Name()
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return false
	}
	return os.Remove(path) == nil
}

func validCommandID(id string) bool {
	if len(id) != 32 {
		return false
	}
	for _, r := range id {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return false
		}
	}
	return true
}

func commandFileExists(dataDir, commandID string) (bool, error) {
	entries, err := os.ReadDir(filepath.Join(controlDir(dataDir), "commands"))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read commands dir: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), "-"+commandID+".json") {
			return true, nil
		}
	}
	return false, nil
}

func cleanupCommandResults(dataDir string, now time.Time) error {
	dir := commandResultsDir(dataDir)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read command results dir: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			continue
		}
		var outcome CommandOutcome
		if json.Unmarshal(raw, &outcome) != nil || outcome.CommandID == "" {
			continue
		}
		age := now.Sub(outcome.UpdatedAt)
		if outcome.Status == CommandStatusRunning {
			if age > commandRunningStale {
				outcome.Status = CommandStatusUnknown
				outcome.ErrorCode = "execution_interrupted"
				outcome.Message = "命令执行状态中断，结果未知；不会自动重试"
				outcome.UpdatedAt = now
				if err := writeJSONAtomic(path, outcome); err != nil {
					return err
				}
			}
			continue
		}
		if outcome.Status == CommandStatusQueued {
			continue
		}
		if outcome.Status == CommandStatusExpired {
			if age > expiredTombstoneTTL {
				_ = os.Remove(path)
			}
			continue
		}
		if age > commandResultTTL {
			outcome.Status = CommandStatusExpired
			outcome.ErrorCode = "result_expired"
			outcome.Message = "命令结果已过期"
			outcome.UpdatedAt = now
			if err := writeJSONAtomic(path, outcome); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeJSONAtomic(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
