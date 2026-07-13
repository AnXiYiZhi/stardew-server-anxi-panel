package updater

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

var ErrNoDryRunStatus = errors.New("dry-run status not found")

type StateStore struct {
	path string
	mu   sync.Mutex
}

func NewStateStore(path string) *StateStore { return &StateStore{path: path} }

func (s *StateStore) Path() string { return s.path }

func (s *StateStore) Read() (DryRunStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return DryRunStatus{}, ErrNoDryRunStatus
	}
	if err != nil {
		return DryRunStatus{}, fmt.Errorf("read updater status: %w", err)
	}
	var status DryRunStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return DryRunStatus{}, fmt.Errorf("decode updater status: %w", err)
	}
	if status.Logs == nil {
		status.Logs = []LogEntry{}
	}
	return status, nil
}

func (s *StateStore) Write(status DryRunStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if status.Logs == nil {
		status.Logs = []LogEntry{}
	}
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return fmt.Errorf("encode updater status: %w", err)
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create updater status dir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".status-*.tmp")
	if err != nil {
		return fmt.Errorf("create updater status temp file: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	defer cleanup()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod updater status: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write updater status: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync updater status: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close updater status: %w", err)
	}
	if err := atomicReplaceFile(tmpName, s.path); err != nil {
		return fmt.Errorf("replace updater status: %w", err)
	}
	if dirHandle, err := os.Open(dir); err == nil {
		_ = dirHandle.Sync()
		_ = dirHandle.Close()
	}
	return nil
}
