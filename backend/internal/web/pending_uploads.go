package web

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
)

type durablePendingUpload struct {
	InstanceID        string            `json:"instanceId"`
	StagedDir         string            `json:"stagedDir,omitempty"`
	SaveName          string            `json:"saveName"`
	Preview           registry.SaveInfo `json:"preview,omitempty"`
	ExpiresAt         time.Time         `json:"expiresAt,omitempty"`
	Status            string            `json:"status"`
	OperationID       string            `json:"operationId,omitempty"`
	JobType           string            `json:"jobType,omitempty"`
	JobID             string            `json:"jobId,omitempty"`
	JobIdempotencyKey string            `json:"jobIdempotencyKey,omitempty"`
	LeaseUntil        time.Time         `json:"leaseUntil,omitempty"`
	MetadataCompacted bool              `json:"metadataCompacted,omitempty"`
}

type durablePendingUploadCleanupReceipt struct {
	SchemaVersion     int       `json:"schemaVersion"`
	InstanceID        string    `json:"instanceId"`
	OperationID       string    `json:"operationId"`
	JobType           string    `json:"jobType"`
	JobID             string    `json:"jobId"`
	JobIdempotencyKey string    `json:"jobIdempotencyKey"`
	CompletedAt       time.Time `json:"completedAt"`
}

// durablePendingUploadReference identifies a persisted token record by its
// one-way token hash. It lets recovery converge an already-owned transaction
// without retaining or reconstructing the original bearer token in the UI.
type durablePendingUploadReference struct {
	TokenHash       string
	Entry           durablePendingUpload
	RecordUpdatedAt time.Time
}

type durablePendingUploadCleanupReference struct {
	Upload  durablePendingUploadReference
	Receipt durablePendingUploadCleanupReceipt
}

type durablePendingUploadStore struct {
	mu        sync.Mutex
	now       func() time.Time
	write     func(string, string, *durablePendingUpload) error
	removeAll func(string) error
}

func newDurablePendingUploadStore() *durablePendingUploadStore {
	return &durablePendingUploadStore{now: time.Now, write: writeDurablePendingUpload, removeAll: os.RemoveAll}
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

func durableUploadDirByHash(dataDir, tokenHash string) string {
	return filepath.Join(durableUploadRoot(dataDir), tokenHash)
}

func durableUploadRecordPath(dataDir, token string) string {
	return filepath.Join(durableUploadDir(dataDir, token), "token.json")
}

func durableUploadCleanupReceiptRoot(dataDir string) string {
	return filepath.Join(dataDir, ".local-container", "control", "save-import-cleanup-receipts")
}

func durableUploadCleanupReceiptPath(dataDir, token string) string {
	return filepath.Join(durableUploadCleanupReceiptRoot(dataDir), pendingUploadHash(token)+".json")
}

func durableUploadCleanupReceiptPathByHash(dataDir, tokenHash string) string {
	return filepath.Join(durableUploadCleanupReceiptRoot(dataDir), tokenHash+".json")
}

func validPendingUploadHash(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && hex.EncodeToString(decoded) == value
}

func writeDurablePendingUpload(dataDir, token string, entry *durablePendingUpload) error {
	dir := durableUploadDir(dataDir, token)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	_ = os.Chmod(durableUploadRoot(dataDir), 0o700)
	_ = os.Chmod(dir, 0o700)
	return writeDurablePendingUploadRecord(dir, entry)
}

func writeDurablePendingUploadRecord(dir string, entry *durablePendingUpload) error {
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
	path := filepath.Join(dir, "token.json")
	if err := os.Rename(name, path); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
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

func readDurablePendingUploadByHash(dataDir, tokenHash string) (*durablePendingUpload, error) {
	if !validPendingUploadHash(tokenHash) {
		return nil, fmt.Errorf("invalid pending upload token hash")
	}
	data, err := os.ReadFile(filepath.Join(durableUploadDirByHash(dataDir, tokenHash), "token.json"))
	if err != nil {
		return nil, err
	}
	var entry durablePendingUpload
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

func writeDurablePendingUploadCleanupReceipt(dataDir, token string, receipt durablePendingUploadCleanupReceipt) error {
	return writeDurablePendingUploadCleanupReceiptByHash(dataDir, pendingUploadHash(token), receipt)
}

func writeDurablePendingUploadCleanupReceiptByHash(dataDir, tokenHash string, receipt durablePendingUploadCleanupReceipt) error {
	if !validPendingUploadHash(tokenHash) {
		return fmt.Errorf("invalid pending upload token hash")
	}
	dir := durableUploadCleanupReceiptRoot(dataDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".receipt-*.tmp")
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
	path := durableUploadCleanupReceiptPathByHash(dataDir, tokenHash)
	if err := os.Rename(name, path); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

func readDurablePendingUploadCleanupReceipt(dataDir, token string) (durablePendingUploadCleanupReceipt, error) {
	return readDurablePendingUploadCleanupReceiptByHash(dataDir, pendingUploadHash(token))
}

func readDurablePendingUploadCleanupReceiptByHash(dataDir, tokenHash string) (durablePendingUploadCleanupReceipt, error) {
	if !validPendingUploadHash(tokenHash) {
		return durablePendingUploadCleanupReceipt{}, fmt.Errorf("invalid pending upload token hash")
	}
	data, err := os.ReadFile(durableUploadCleanupReceiptPathByHash(dataDir, tokenHash))
	if err != nil {
		return durablePendingUploadCleanupReceipt{}, err
	}
	var receipt durablePendingUploadCleanupReceipt
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&receipt); err != nil {
		return durablePendingUploadCleanupReceipt{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return durablePendingUploadCleanupReceipt{}, fmt.Errorf("invalid save import cleanup receipt JSON")
	}
	if receipt.SchemaVersion != 1 || receipt.InstanceID == "" || receipt.OperationID == "" ||
		receipt.JobType != sj.SaveImportJobType || receipt.JobID == "" ||
		receipt.JobIdempotencyKey != sj.SaveImportJobIdempotencyKey(receipt.OperationID) || receipt.CompletedAt.IsZero() {
		return durablePendingUploadCleanupReceipt{}, fmt.Errorf("invalid save import cleanup receipt")
	}
	return receipt, nil
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
	s.pruneExpiredSucceededLocked(dataDir)
	target := filepath.Join(durableUploadDir(dataDir, token), "payload")
	if err := stagePendingUpload(sourceDir, target); err != nil {
		return "", err
	}
	entry := &durablePendingUpload{InstanceID: instanceID, StagedDir: target, SaveName: saveName, Preview: preview, ExpiresAt: s.now().Add(uploadTokenTTL), Status: "available"}
	if err := s.write(dataDir, token, entry); err != nil {
		_ = s.removeAll(durableUploadDir(dataDir, token))
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
	if err := s.write(dataDir, token, entry); err != nil {
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
	if err := s.write(dataDir, token, entry); err != nil {
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
	if (entry.Status != "owned" && entry.Status != "succeeded") || entry.OperationID != operationID {
		return fmt.Errorf("owned token mismatch")
	}
	if entry.JobID != "" || entry.JobType != "" || entry.JobIdempotencyKey != "" {
		if entry.JobID != jobID || (entry.JobType != "" && entry.JobType != sj.SaveImportJobType) ||
			(entry.JobIdempotencyKey != "" && entry.JobIdempotencyKey != sj.SaveImportJobIdempotencyKey(operationID)) {
			return fmt.Errorf("upload token already has a different job")
		}
	}
	entry.JobType = sj.SaveImportJobType
	entry.JobID = jobID
	entry.JobIdempotencyKey = sj.SaveImportJobIdempotencyKey(operationID)
	return s.write(dataDir, token, entry)
}

func (s *durablePendingUploadStore) markSucceeded(dataDir, token, operationID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, err := readDurablePendingUpload(dataDir, token)
	if err != nil {
		return err
	}
	if entry.Status != "owned" || entry.OperationID != operationID {
		return fmt.Errorf("owned token mismatch")
	}
	if entry.JobType != sj.SaveImportJobType || entry.JobID == "" || entry.JobIdempotencyKey != sj.SaveImportJobIdempotencyKey(operationID) {
		return fmt.Errorf("owned token job identity is incomplete")
	}
	entry.Status = "succeeded"
	entry.ExpiresAt = s.now().Add(uploadTokenTTL)
	entry.LeaseUntil = time.Time{}
	return s.write(dataDir, token, entry)
}

func (s *durablePendingUploadStore) pruneExpiredSucceededLocked(dataDir string) {
	entries, err := os.ReadDir(durableUploadRoot(dataDir))
	if err != nil {
		return
	}
	for _, item := range entries {
		if !item.IsDir() {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(durableUploadRoot(dataDir), item.Name(), "token.json"))
		if readErr != nil {
			continue
		}
		var entry durablePendingUpload
		if json.Unmarshal(data, &entry) != nil || entry.Status != "succeeded" || s.now().Before(entry.ExpiresAt) {
			continue
		}
		if entry.JobType != sj.SaveImportJobType || entry.JobID == "" || entry.OperationID == "" ||
			entry.JobIdempotencyKey != sj.SaveImportJobIdempotencyKey(entry.OperationID) {
			continue
		}
		entry.StagedDir = ""
		entry.Preview = registry.SaveInfo{}
		entry.ExpiresAt = time.Time{}
		entry.LeaseUntil = time.Time{}
		entry.MetadataCompacted = true
		_ = writeDurablePendingUploadRecord(filepath.Join(durableUploadRoot(dataDir), item.Name()), &entry)
	}
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
	if err := s.write(dataDir, token, entry); err != nil {
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

// findOwnedByOperation recovers the durable token side of an import from the
// operation identity. The raw bearer token is intentionally unavailable after
// the original HTTP submission, so automatic recovery uses the on-disk token
// hash only after the record proves the exact instance and operation owner.
func (s *durablePendingUploadStore) findOwnedByOperation(dataDir, instanceID, operationID string) (durablePendingUploadReference, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(durableUploadRoot(dataDir))
	if err != nil {
		return durablePendingUploadReference{}, err
	}
	var matched *durablePendingUploadReference
	for _, item := range entries {
		if !item.IsDir() || !validPendingUploadHash(item.Name()) {
			continue
		}
		entry, readErr := readDurablePendingUploadByHash(dataDir, item.Name())
		if readErr != nil {
			return durablePendingUploadReference{}, fmt.Errorf("read pending upload ownership record: %w", readErr)
		}
		if entry.Status != "owned" || entry.InstanceID != instanceID || entry.OperationID != operationID {
			continue
		}
		if matched != nil {
			return durablePendingUploadReference{}, fmt.Errorf("multiple owned upload records match the save import operation")
		}
		recordInfo, statErr := os.Stat(filepath.Join(durableUploadDirByHash(dataDir, item.Name()), "token.json"))
		if statErr != nil {
			return durablePendingUploadReference{}, fmt.Errorf("stat pending upload ownership record: %w", statErr)
		}
		matched = &durablePendingUploadReference{TokenHash: item.Name(), Entry: *entry, RecordUpdatedAt: recordInfo.ModTime().UTC()}
	}
	if matched == nil {
		return durablePendingUploadReference{}, os.ErrNotExist
	}
	return *matched, nil
}

func (s *durablePendingUploadStore) attachJobByReference(dataDir string, reference durablePendingUploadReference, jobID string) (durablePendingUploadReference, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, err := readDurablePendingUploadByHash(dataDir, reference.TokenHash)
	if err != nil {
		return durablePendingUploadReference{}, err
	}
	expected := reference.Entry
	if entry.Status != "owned" || entry.InstanceID != expected.InstanceID || entry.OperationID != expected.OperationID {
		return durablePendingUploadReference{}, fmt.Errorf("owned token changed before automatic job recovery")
	}
	expectedKey := sj.SaveImportJobIdempotencyKey(entry.OperationID)
	if (entry.JobID != "" && entry.JobID != jobID) || (entry.JobType != "" && entry.JobType != sj.SaveImportJobType) ||
		(entry.JobIdempotencyKey != "" && entry.JobIdempotencyKey != expectedKey) {
		return durablePendingUploadReference{}, fmt.Errorf("owned token already has a different job identity")
	}
	entry.JobType = sj.SaveImportJobType
	entry.JobID = jobID
	entry.JobIdempotencyKey = expectedKey
	if err := writeDurablePendingUploadRecord(durableUploadDirByHash(dataDir, reference.TokenHash), entry); err != nil {
		return durablePendingUploadReference{}, err
	}
	return durablePendingUploadReference{TokenHash: reference.TokenHash, Entry: *entry}, nil
}

func (s *durablePendingUploadStore) cleanupReferences(dataDir, instanceID string) ([]durablePendingUploadCleanupReference, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(durableUploadCleanupReceiptRoot(dataDir))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	result := make([]durablePendingUploadCleanupReference, 0)
	for _, item := range entries {
		if item.IsDir() || filepath.Ext(item.Name()) != ".json" {
			continue
		}
		tokenHash := strings.TrimSuffix(item.Name(), ".json")
		if !validPendingUploadHash(tokenHash) {
			continue
		}
		receipt, readErr := readDurablePendingUploadCleanupReceiptByHash(dataDir, tokenHash)
		if readErr != nil {
			return nil, fmt.Errorf("read save import cleanup receipt: %w", readErr)
		}
		if receipt.InstanceID != instanceID {
			continue
		}
		entry, readErr := readDurablePendingUploadByHash(dataDir, tokenHash)
		if os.IsNotExist(readErr) {
			continue
		}
		if readErr != nil {
			return nil, fmt.Errorf("read cleanup-owned upload record: %w", readErr)
		}
		if entry.Status != "owned" || entry.InstanceID != receipt.InstanceID || entry.OperationID != receipt.OperationID ||
			entry.JobType != receipt.JobType || entry.JobID != receipt.JobID || entry.JobIdempotencyKey != receipt.JobIdempotencyKey {
			return nil, fmt.Errorf("cleanup-owned upload record does not match its receipt")
		}
		result = append(result, durablePendingUploadCleanupReference{
			Upload:  durablePendingUploadReference{TokenHash: tokenHash, Entry: *entry},
			Receipt: receipt,
		})
	}
	return result, nil
}

func (s *durablePendingUploadStore) markCleanupCompleted(dataDir, token, operationID, jobID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, err := readDurablePendingUpload(dataDir, token)
	if err != nil {
		return err
	}
	if entry.Status != "owned" || entry.OperationID != operationID || entry.JobType != sj.SaveImportJobType ||
		entry.JobID != jobID || entry.JobIdempotencyKey != sj.SaveImportJobIdempotencyKey(operationID) {
		return fmt.Errorf("owned token mismatch")
	}
	receipt := durablePendingUploadCleanupReceipt{
		SchemaVersion: 1, InstanceID: entry.InstanceID, OperationID: operationID,
		JobType: sj.SaveImportJobType, JobID: jobID,
		JobIdempotencyKey: sj.SaveImportJobIdempotencyKey(operationID), CompletedAt: s.now().UTC(),
	}
	return writeDurablePendingUploadCleanupReceipt(dataDir, token, receipt)
}

func (s *durablePendingUploadStore) markCleanupCompletedByReference(dataDir string, reference durablePendingUploadReference) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, err := readDurablePendingUploadByHash(dataDir, reference.TokenHash)
	if err != nil {
		return err
	}
	expected := reference.Entry
	if entry.Status != "owned" || entry.InstanceID != expected.InstanceID || entry.OperationID != expected.OperationID ||
		entry.JobType != sj.SaveImportJobType || entry.JobID != expected.JobID ||
		entry.JobIdempotencyKey != sj.SaveImportJobIdempotencyKey(expected.OperationID) {
		return fmt.Errorf("owned token changed before automatic cleanup receipt")
	}
	receipt := durablePendingUploadCleanupReceipt{
		SchemaVersion: 1, InstanceID: entry.InstanceID, OperationID: entry.OperationID,
		JobType: sj.SaveImportJobType, JobID: entry.JobID,
		JobIdempotencyKey: entry.JobIdempotencyKey, CompletedAt: s.now().UTC(),
	}
	return writeDurablePendingUploadCleanupReceiptByHash(dataDir, reference.TokenHash, receipt)
}

func (s *durablePendingUploadStore) cleanupReceipt(dataDir, token, instanceID string) (durablePendingUploadCleanupReceipt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	receipt, err := readDurablePendingUploadCleanupReceipt(dataDir, token)
	if err != nil {
		return durablePendingUploadCleanupReceipt{}, err
	}
	if receipt.InstanceID != instanceID {
		return durablePendingUploadCleanupReceipt{}, fmt.Errorf("cleanup receipt instance mismatch")
	}
	return receipt, nil
}

func (s *durablePendingUploadStore) cleanupReceiptByReference(dataDir string, reference durablePendingUploadReference) (durablePendingUploadCleanupReceipt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	receipt, err := readDurablePendingUploadCleanupReceiptByHash(dataDir, reference.TokenHash)
	if err != nil {
		return durablePendingUploadCleanupReceipt{}, err
	}
	if receipt.InstanceID != reference.Entry.InstanceID || receipt.OperationID != reference.Entry.OperationID ||
		receipt.JobID != reference.Entry.JobID {
		return durablePendingUploadCleanupReceipt{}, fmt.Errorf("cleanup receipt does not match automatic recovery owner")
	}
	return receipt, nil
}

func (s *durablePendingUploadStore) removeOwnedAfterCleanup(dataDir, token string, receipt durablePendingUploadCleanupReceipt) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	durable, err := readDurablePendingUploadCleanupReceipt(dataDir, token)
	if err != nil {
		return err
	}
	if durable != receipt {
		return fmt.Errorf("cleanup receipt changed")
	}
	entry, err := readDurablePendingUpload(dataDir, token)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if entry.Status != "owned" || entry.InstanceID != receipt.InstanceID || entry.OperationID != receipt.OperationID ||
		entry.JobType != receipt.JobType || entry.JobID != receipt.JobID || entry.JobIdempotencyKey != receipt.JobIdempotencyKey {
		return fmt.Errorf("owned token does not match cleanup receipt")
	}
	return s.removeAll(durableUploadDir(dataDir, token))
}

func (s *durablePendingUploadStore) removeOwnedAfterCleanupByReference(dataDir string, reference durablePendingUploadReference, receipt durablePendingUploadCleanupReceipt) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	durable, err := readDurablePendingUploadCleanupReceiptByHash(dataDir, reference.TokenHash)
	if err != nil {
		return err
	}
	if durable != receipt {
		return fmt.Errorf("cleanup receipt changed")
	}
	entry, err := readDurablePendingUploadByHash(dataDir, reference.TokenHash)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if entry.Status != "owned" || entry.InstanceID != receipt.InstanceID || entry.OperationID != receipt.OperationID ||
		entry.JobType != receipt.JobType || entry.JobID != receipt.JobID || entry.JobIdempotencyKey != receipt.JobIdempotencyKey {
		return fmt.Errorf("owned token does not match cleanup receipt")
	}
	return s.removeAll(durableUploadDirByHash(dataDir, reference.TokenHash))
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
	entry.JobType, entry.JobID, entry.JobIdempotencyKey = "", "", ""
	return s.write(dataDir, token, entry)
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
	return s.write(dataDir, token, entry)
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
	return s.removeAll(durableUploadDir(dataDir, token))
}
