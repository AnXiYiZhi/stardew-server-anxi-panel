package web

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
)

type durablePendingUpload struct {
	InstanceID  string            `json:"instanceId"`
	StagedDir   string            `json:"stagedDir"`
	SaveName    string            `json:"saveName"`
	Preview     registry.SaveInfo `json:"preview"`
	ExpiresAt   time.Time         `json:"expiresAt"`
	Status      string            `json:"status"`
	OperationID string            `json:"operationId,omitempty"`
	JobID       string            `json:"jobId,omitempty"`
	LeaseUntil  time.Time         `json:"leaseUntil,omitempty"`
}

type durablePendingUploadStore struct {
	mu  sync.Mutex
	now func() time.Time
}

func newDurablePendingUploadStore() *durablePendingUploadStore {
	return &durablePendingUploadStore{now: time.Now}
}

func pendingUploadHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func durableUploadRoot(dataDir string) string {
	return filepath.Join(dataDir, ".local-container", "control", "pending-save-uploads")
}

func durableUploadDir(dataDir, token string) string {
	return filepath.Join(durableUploadRoot(dataDir), pendingUploadHash(token))
}

func durableUploadRecordPath(dataDir, token string) string {
	return filepath.Join(durableUploadDir(dataDir, token), "token.json")
}

func writeDurablePendingUpload(dataDir, token string, entry *durablePendingUpload) error {
	dir := durableUploadDir(dataDir, token)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	_ = os.Chmod(durableUploadRoot(dataDir), 0o700)
	_ = os.Chmod(dir, 0o700)
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".token-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer func() { _ = os.Remove(name) }()
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
	if err := os.Rename(name, durableUploadRecordPath(dataDir, token)); err != nil {
		return err
	}
	return os.Chmod(durableUploadRecordPath(dataDir, token), 0o600)
}

func readDurablePendingUpload(dataDir, token string) (*durablePendingUpload, error) {
	data, err := os.ReadFile(durableUploadRecordPath(dataDir, token))
	if err != nil {
		return nil, err
	}
	var entry durablePendingUpload
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

func stagePendingUpload(source, target string) error {
	if _, err := os.Lstat(target); err == nil {
		return fmt.Errorf("upload ownership target already exists")
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	if err := os.Rename(source, target); err == nil {
		return tightenPendingUploadTree(target)
	}
	tempTarget, err := os.MkdirTemp(filepath.Dir(target), ".upload-transfer-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tempTarget) }()
	if err := filepath.WalkDir(source, func(path string, item os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		dst := filepath.Join(tempTarget, rel)
		if item.IsDir() {
			return os.MkdirAll(dst, 0o700)
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(out, in)
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	}); err != nil {
		return err
	}
	if err := tightenPendingUploadTree(tempTarget); err != nil {
		return err
	}
	if _, err := os.Lstat(target); err == nil {
		return fmt.Errorf("upload ownership target already exists")
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(tempTarget, target); err != nil {
		return err
	}
	return os.RemoveAll(source)
}

func tightenPendingUploadTree(root string) error {
	return filepath.WalkDir(root, func(path string, item os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if item.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("upload payload contains a symbolic link")
		}
		if item.IsDir() {
			return os.Chmod(path, 0o700)
		}
		return os.Chmod(path, 0o600)
	})
}

func (s *durablePendingUploadStore) put(dataDir, instanceID, sourceDir, saveName string, preview registry.SaveInfo) (string, error) {
	token := newToken()
	s.mu.Lock()
	defer s.mu.Unlock()
	target := filepath.Join(durableUploadDir(dataDir, token), "payload")
	if err := stagePendingUpload(sourceDir, target); err != nil {
		return "", err
	}
	entry := &durablePendingUpload{InstanceID: instanceID, StagedDir: target, SaveName: saveName, Preview: preview, ExpiresAt: s.now().Add(uploadTokenTTL), Status: "available"}
	if err := writeDurablePendingUpload(dataDir, token, entry); err != nil {
		_ = os.RemoveAll(durableUploadDir(dataDir, token))
		return "", err
	}
	return token, nil
}

func (s *durablePendingUploadStore) reserve(dataDir, token, instanceID, operationID string) (*durablePendingUpload, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, err := readDurablePendingUpload(dataDir, token)
	if err != nil {
		return nil, fmt.Errorf("upload token invalid")
	}
	if entry.InstanceID != instanceID {
		return nil, fmt.Errorf("upload token instance mismatch")
	}
	if entry.Status == "reserved" {
		if entry.OperationID == operationID {
			return entry, nil
		}
		return nil, fmt.Errorf("upload token reserved")
	}
	if entry.Status == "owned" {
		if entry.OperationID == operationID {
			return entry, nil
		}
		return nil, fmt.Errorf("upload token owned by another operation")
	}
	if entry.Status != "available" {
		return nil, fmt.Errorf("upload token unavailable")
	}
	if s.now().After(entry.ExpiresAt) {
		_ = os.RemoveAll(durableUploadDir(dataDir, token))
		return nil, fmt.Errorf("upload token expired")
	}
	entry.Status, entry.OperationID, entry.LeaseUntil = "reserved", operationID, s.now().Add(uploadTokenTTL)
	if err := writeDurablePendingUpload(dataDir, token, entry); err != nil {
		return nil, err
	}
	return entry, nil
}

// reserveOrReuse reserves an available token, or returns the operation which
// already owns that token. This lets a client retry the same commit without
// supplying or inventing a second operation ID.
func (s *durablePendingUploadStore) reserveOrReuse(dataDir, token, instanceID, operationID string) (*durablePendingUpload, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, err := readDurablePendingUpload(dataDir, token)
	if err != nil {
		return nil, fmt.Errorf("upload token invalid")
	}
	if entry.InstanceID != instanceID {
		return nil, fmt.Errorf("upload token instance mismatch")
	}
	if entry.Status == "reserved" || entry.Status == "owned" {
		if entry.OperationID == "" {
			return nil, fmt.Errorf("upload token has invalid ownership")
		}
		return entry, nil
	}
	if entry.Status != "available" {
		return nil, fmt.Errorf("upload token unavailable")
	}
	if s.now().After(entry.ExpiresAt) {
		_ = os.RemoveAll(durableUploadDir(dataDir, token))
		return nil, fmt.Errorf("upload token expired")
	}
	entry.Status, entry.OperationID, entry.LeaseUntil = "reserved", operationID, s.now().Add(uploadTokenTTL)
	if err := writeDurablePendingUpload(dataDir, token, entry); err != nil {
		return nil, err
	}
	return entry, nil
}

func (s *durablePendingUploadStore) lookup(dataDir, token, instanceID string) (*durablePendingUpload, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, err := readDurablePendingUpload(dataDir, token)
	if err != nil {
		return nil, fmt.Errorf("upload token invalid")
	}
	if entry.InstanceID != instanceID {
		return nil, fmt.Errorf("upload token instance mismatch")
	}
	return entry, nil
}

func (s *durablePendingUploadStore) attachJob(dataDir, token, operationID, jobID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, err := readDurablePendingUpload(dataDir, token)
	if err != nil {
		return err
	}
	if entry.Status != "owned" || entry.OperationID != operationID {
		return fmt.Errorf("owned token mismatch")
	}
	if entry.JobID != "" && entry.JobID != jobID {
		return fmt.Errorf("upload token already has a different job")
	}
	entry.JobID = jobID
	return writeDurablePendingUpload(dataDir, token, entry)
}

func transactionSourceDirForUpload(dataDir, operationID string) string {
	return filepath.Join(dataDir, ".local-container", "control", "save-import-transactions", operationID, "source")
}

// transferOwnership moves the payload out of token storage and into the
// operation directory. The journal is created by the driver before invoking
// this method, so a successful move makes the operation the durable owner.
func (s *durablePendingUploadStore) transferOwnership(dataDir, token, operationID, target string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, err := readDurablePendingUpload(dataDir, token)
	if err != nil {
		return fmt.Errorf("upload token invalid")
	}
	expected := transactionSourceDirForUpload(dataDir, operationID)
	if filepath.Clean(target) != filepath.Clean(expected) {
		return fmt.Errorf("invalid transaction source target")
	}
	if entry.OperationID != operationID || (entry.Status != "reserved" && entry.Status != "owned") {
		return fmt.Errorf("token lease mismatch")
	}
	if entry.Status == "owned" {
		if info, statErr := os.Stat(entry.StagedDir); statErr == nil && info.IsDir() {
			return nil
		}
		return fmt.Errorf("owned transaction source is missing")
	}
	if err := stagePendingUpload(entry.StagedDir, target); err != nil {
		return err
	}
	entry.Status = "owned"
	entry.StagedDir = target
	entry.LeaseUntil = time.Time{}
	if err := writeDurablePendingUpload(dataDir, token, entry); err != nil {
		return fmt.Errorf("persist upload ownership: %w", err)
	}
	return nil
}

func (s *durablePendingUploadStore) ownedOperation(dataDir, token string) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, err := readDurablePendingUpload(dataDir, token)
	if err != nil {
		return "", false, err
	}
	return entry.OperationID, entry.Status == "owned", nil
}

func (s *durablePendingUploadStore) removeOwned(dataDir, token, operationID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, err := readDurablePendingUpload(dataDir, token)
	if err != nil {
		return err
	}
	if entry.Status != "owned" || entry.OperationID != operationID {
		return fmt.Errorf("owned token mismatch")
	}
	return os.RemoveAll(durableUploadDir(dataDir, token))
}

func (s *durablePendingUploadStore) release(dataDir, token, operationID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, err := readDurablePendingUpload(dataDir, token)
	if err != nil {
		return err
	}
	if entry.Status != "reserved" || entry.OperationID != operationID {
		return fmt.Errorf("token lease mismatch")
	}
	entry.Status, entry.OperationID, entry.LeaseUntil = "available", "", time.Time{}
	return writeDurablePendingUpload(dataDir, token, entry)
}

func (s *durablePendingUploadStore) consume(dataDir, token, operationID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, err := readDurablePendingUpload(dataDir, token)
	if err != nil {
		return err
	}
	if entry.Status != "reserved" || entry.OperationID != operationID {
		return fmt.Errorf("token lease mismatch")
	}
	entry.Status = "consumed"
	return writeDurablePendingUpload(dataDir, token, entry)
}

func (s *durablePendingUploadStore) cancel(dataDir, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, err := readDurablePendingUpload(dataDir, token)
	if err != nil {
		return err
	}
	if entry.Status == "reserved" || entry.OperationID != "" {
		return fmt.Errorf("upload is part of an import transaction")
	}
	return os.RemoveAll(durableUploadDir(dataDir, token))
}
