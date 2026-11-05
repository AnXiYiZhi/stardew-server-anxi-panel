package updater

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
)

var ErrNoApplyStatus = errors.New("apply status not found")

type ApplyStateStore struct {
	path string
	mu   sync.Mutex
}

func NewApplyStateStore(path string) *ApplyStateStore { return &ApplyStateStore{path: path} }
func (s *ApplyStateStore) Path() string               { return s.path }

func (s *ApplyStateStore) Read() (ApplyStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readLocked()
}

func (s *ApplyStateStore) readLocked() (ApplyStatus, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return ApplyStatus{}, ErrNoApplyStatus
	}
	if err != nil {
		return ApplyStatus{}, fmt.Errorf("read apply status: %w", err)
	}
	var status ApplyStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return ApplyStatus{}, fmt.Errorf("decode apply status: %w", err)
	}
	if status.Logs == nil {
		status.Logs = []LogEntry{}
	}
	return status, nil
}

func (s *ApplyStateStore) Write(status ApplyStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeLocked(status)
}

// CompareAndSwap never replaces a newer helper, cleanup or administrator result.
func (s *ApplyStateStore) CompareAndSwap(expected, replacement ApplyStatus) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	actual, err := s.readLocked()
	if err != nil {
		return false, err
	}
	if !reflect.DeepEqual(actual, expected) {
		return false, nil
	}
	return true, s.writeLocked(replacement)
}

func (s *ApplyStateStore) writeLocked(status ApplyStatus) error {
	if status.Logs == nil {
		status.Logs = []LogEntry{}
	}
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return fmt.Errorf("encode apply status: %w", err)
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create apply status dir: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("secure apply status dir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".apply-*.tmp")
	if err != nil {
		return fmt.Errorf("create apply status temp: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
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
	if err := atomicReplaceFile(tmpName, s.path); err != nil {
		return fmt.Errorf("replace apply status: %w", err)
	}
	return nil
}
