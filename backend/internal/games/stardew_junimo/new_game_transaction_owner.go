package stardew_junimo

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
)

const (
	newGameOwnerSchemaVersion = 1
	newGameOwnerStateReserved = "reserved"
	newGameOwnerStateActive   = "active"

	newGameOwnerClaimStagingPrefix  = ".new-game-owner.claim-"
	newGameOwnerEmptyRecoveryPrefix = ".new-game-owner.empty-recovery-"
	newGameOwnerLegacyClaimMinAge   = 5 * time.Second
)

// NewGameOwnerRecord is the fixed, persistent single-writer lease for an
// instance. Transaction history remains after this lease is released.
type NewGameOwnerRecord struct {
	SchemaVersion       int       `json:"schemaVersion"`
	InstanceDataDirHash string    `json:"instanceDataDirHash"`
	RequestID           string    `json:"requestId"`
	ConfigSHA256        string    `json:"configSha256"`
	TransactionID       string    `json:"transactionId"`
	JobID               string    `json:"jobId,omitempty"`
	ExecutorID          string    `json:"executorId"`
	OwnerToken          string    `json:"ownerToken"`
	State               string    `json:"state"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

type NewGameOwnerError struct {
	Code    string
	Message string
	Owner   *NewGameOwnerRecord
	Cause   error
}

func (e *NewGameOwnerError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return e.Code + ": " + e.Message
}

func (e *NewGameOwnerError) Unwrap() error { return e.Cause }

var newGameOwnerOperationMu sync.Mutex

var (
	newGameExecutorOnce sync.Once
	newGameExecutorID   string
	newGameExecutorErr  error

	// An empty fixed owner directory is only recoverable when it predates this
	// process. New code never exposes that intermediate state; this timestamp
	// fences an in-process/just-started legacy claimant from being removed.
	newGameOwnerProcessStartedAt = time.Now().UTC()
)

func newGameOwnerDir(dataDir string) string {
	return filepath.Join(controlDir(dataDir), "new-game-owner")
}

func newGameOwnerPath(dataDir string) string {
	return filepath.Join(newGameOwnerDir(dataDir), "owner.json")
}

func newGameOwnerRotationDir(dataDir string) string {
	return filepath.Join(controlDir(dataDir), "new-game-owner.rotation")
}

func ensureNewGameControlDir(dataDir string) error {
	dir := controlDir(dataDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.Chmod(dir, 0o755)
}

func currentNewGameExecutorID() (string, error) {
	newGameExecutorOnce.Do(func() {
		newGameExecutorID, newGameExecutorErr = newGameRandomHex(32)
	})
	return newGameExecutorID, newGameExecutorErr
}

func newGameRandomHex(byteCount int) (string, error) {
	data := make([]byte, byteCount)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}

func isValidNewGameTransactionID(value string) bool {
	if len(value) != 32 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func isValidNewGameSHA256(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func newGameInstanceDataDirHash(dataDir string) (string, error) {
	abs, err := filepath.Abs(filepath.Clean(dataDir))
	if err != nil {
		return "", fmt.Errorf("resolve instance data directory: %w", err)
	}
	canonical := filepath.ToSlash(abs)
	if runtime.GOOS == "windows" {
		canonical = strings.ToLower(canonical)
	}
	digest := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(digest[:]), nil
}

func newGameConfigSHA256(cfg registry.NewGameConfig) (string, error) {
	payload, err := json.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("marshal new-game config fingerprint: %w", err)
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func validateNewGameRequestID(requestID string) error {
	if strings.TrimSpace(requestID) == "" {
		return errors.New("request id is empty")
	}
	if len(requestID) > 256 || !utf8.ValidString(requestID) || strings.ContainsAny(requestID, "\x00\r\n") {
		return errors.New("request id is invalid")
	}
	return nil
}

// beginOrResumeNewGameTransaction atomically acquires the instance owner or
// resumes the exact persisted request. A different request/config never takes
// over a nonterminal transaction.
func beginOrResumeNewGameTransaction(
	dataDir string,
	cfg registry.NewGameConfig,
	requestID string,
	jobID string,
) (*newGameTransaction, bool, error) {
	return beginOrResumeNewGameTransactionWithJobStatus(dataDir, cfg, requestID, jobID, nil)
}

// beginOrResumeNewGameTransactionWithJobStatus permits same-Panel-process
// recovery only when the caller proves the persisted owner job is no longer
// active. The default wrapper passes nil and therefore remains fail-closed.
func beginOrResumeNewGameTransactionWithJobStatus(
	dataDir string,
	cfg registry.NewGameConfig,
	requestID string,
	jobID string,
	isJobActive func(string) (bool, error),
) (*newGameTransaction, bool, error) {
	newGameOwnerOperationMu.Lock()
	defer newGameOwnerOperationMu.Unlock()
	if err := recoverNewGameOwnerRotation(dataDir); err != nil {
		return nil, false, newGameOwnerRecoveryError("恢复中断的 owner token 轮换失败", err)
	}
	if err := recoverLegacyEmptyNewGameOwner(dataDir); err != nil {
		return nil, false, err
	}

	if err := validateNewGameRequestID(requestID); err != nil {
		return nil, false, &NewGameOwnerError{Code: "new_game_request_invalid", Message: "新建存档请求 ID 无效", Cause: err}
	}
	normalized, err := NormalizeNewGameConfigWithModded(cfg, true)
	if err != nil {
		return nil, false, &NewGameOwnerError{Code: "new_game_payload_invalid", Message: "新建存档配置无效", Cause: err}
	}
	configHash, err := newGameConfigSHA256(normalized)
	if err != nil {
		return nil, false, err
	}
	dataDirHash, err := newGameInstanceDataDirHash(dataDir)
	if err != nil {
		return nil, false, err
	}

	matched, err := findNewGameTransactionByRequest(dataDir, requestID)
	if err != nil {
		return nil, false, err
	}
	if matched != nil {
		if matched.ConfigSHA256 != configHash || matched.InstanceDataDirHash != dataDirHash {
			return nil, false, newGameOwnerConflict("new_game_request_conflict", "同一请求 ID 已绑定到不同的新建存档配置", nil)
		}
		if isTerminalNewGameOwnerStage(matched.Stage) {
			owner, ownerErr := LoadNewGameOwner(dataDir)
			if ownerErr == nil && owner.TransactionID == matched.TransactionID {
				if releaseErr := releaseNewGameOwner(rehydrateNewGameTransaction(dataDir, *matched, owner.OwnerToken)); releaseErr != nil {
					return nil, false, releaseErr
				}
			} else if ownerErr != nil && !errors.Is(ownerErr, os.ErrNotExist) {
				return nil, false, newGameOwnerRecoveryError("终态事务的 owner 无法读取", ownerErr)
			}
			return rehydrateNewGameTransaction(dataDir, *matched, ""), true, nil
		}
		owner, loadErr := LoadNewGameOwner(dataDir)
		if loadErr != nil {
			return nil, false, newGameOwnerRecoveryError("事务未完成但持久 owner 缺失或损坏", loadErr)
		}
		return resumeNewGameTransaction(dataDir, *matched, owner, requestID, jobID, configHash, dataDirHash, isJobActive)
	}

	owner, loadErr := LoadNewGameOwner(dataDir)
	if loadErr == nil {
		if staleErr := releaseTerminalNewGameOwner(dataDir, owner); staleErr == nil {
			return startNewGameTransaction(dataDir, normalized, requestID, jobID, configHash, dataDirHash, isJobActive)
		}
		return resumeNewGameTransactionWithoutRecord(dataDir, normalized, owner, requestID, jobID, configHash, dataDirHash, isJobActive)
	}
	if !errors.Is(loadErr, os.ErrNotExist) {
		return nil, false, newGameOwnerRecoveryError("持久 owner 无法读取", loadErr)
	}
	return startNewGameTransaction(dataDir, normalized, requestID, jobID, configHash, dataDirHash, isJobActive)
}

func startNewGameTransaction(
	dataDir string,
	cfg registry.NewGameConfig,
	requestID string,
	jobID string,
	configHash string,
	dataDirHash string,
	isJobActive func(string) (bool, error),
) (*newGameTransaction, bool, error) {
	transactionID, err := newGameRandomHex(16)
	if err != nil {
		return nil, false, err
	}
	ownerToken, err := newGameRandomHex(32)
	if err != nil {
		return nil, false, err
	}
	executorID, err := currentNewGameExecutorID()
	if err != nil {
		return nil, false, err
	}
	now := time.Now().UTC()
	owner := NewGameOwnerRecord{
		SchemaVersion:       newGameOwnerSchemaVersion,
		InstanceDataDirHash: dataDirHash,
		RequestID:           requestID,
		ConfigSHA256:        configHash,
		TransactionID:       transactionID,
		JobID:               jobID,
		ExecutorID:          executorID,
		OwnerToken:          ownerToken,
		State:               newGameOwnerStateReserved,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	claimed, err := claimNewGameOwner(dataDir, owner)
	if err != nil {
		return nil, false, err
	}
	if !claimed {
		existing, loadErr := LoadNewGameOwner(dataDir)
		if loadErr != nil {
			return nil, false, newGameOwnerRecoveryError("owner 竞争后无法读取获胜者", loadErr)
		}
		return resumeNewGameTransactionWithoutRecord(dataDir, cfg, existing, requestID, jobID, configHash, dataDirHash, isJobActive)
	}

	tx, err := beginNewGameTransactionWithIdentity(
		dataDir, cfg, transactionID, requestID, jobID, configHash, ownerToken, false,
	)
	if err != nil {
		return nil, false, err
	}
	owner.State = newGameOwnerStateActive
	owner.UpdatedAt = time.Now().UTC()
	if err := persistNewGameOwner(dataDir, owner); err != nil {
		return nil, false, err
	}
	if err := assertNewGameOwner(dataDir, transactionID, ownerToken); err != nil {
		return nil, false, err
	}
	return tx, false, nil
}

func resumeNewGameTransactionWithoutRecord(
	dataDir string,
	cfg registry.NewGameConfig,
	owner NewGameOwnerRecord,
	requestID string,
	jobID string,
	configHash string,
	dataDirHash string,
	isJobActive func(string) (bool, error),
) (*newGameTransaction, bool, error) {
	if owner.RequestID != requestID || owner.ConfigSHA256 != configHash || owner.InstanceDataDirHash != dataDirHash {
		return nil, false, newGameOwnerConflict("new_game_in_progress", "另一个新建存档事务仍持有实例 owner", &owner)
	}
	record, err := LoadNewGameTransaction(dataDir, owner.TransactionID)
	if err == nil {
		return resumeNewGameTransaction(dataDir, record, owner, requestID, jobID, configHash, dataDirHash, isJobActive)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, false, newGameOwnerRecoveryError("owner 指向的事务记录无法读取", err)
	}

	rotated, err := resumeNewGameOwner(dataDir, owner, jobID, isJobActive)
	if err != nil {
		return nil, false, err
	}
	tx, err := beginNewGameTransactionWithIdentity(
		dataDir, cfg, owner.TransactionID, requestID, jobID, configHash, rotated.OwnerToken, true,
	)
	if err != nil {
		return nil, false, err
	}
	rotated.State = newGameOwnerStateActive
	rotated.UpdatedAt = time.Now().UTC()
	if err := persistNewGameOwner(dataDir, rotated); err != nil {
		return nil, false, err
	}
	return tx, true, nil
}

func resumeNewGameTransaction(
	dataDir string,
	record NewGameTransactionRecord,
	owner NewGameOwnerRecord,
	requestID string,
	jobID string,
	configHash string,
	dataDirHash string,
	isJobActive func(string) (bool, error),
) (*newGameTransaction, bool, error) {
	if owner.RequestID != requestID || owner.ConfigSHA256 != configHash || owner.InstanceDataDirHash != dataDirHash ||
		owner.TransactionID != record.TransactionID {
		return nil, false, newGameOwnerConflict("new_game_in_progress", "另一个新建存档事务仍持有实例 owner", &owner)
	}
	if record.RequestID != requestID || record.ConfigSHA256 != configHash || record.InstanceDataDirHash != dataDirHash {
		return nil, false, newGameOwnerRecoveryError("owner 与事务记录身份不一致", nil)
	}
	rotated, err := resumeNewGameOwner(dataDir, owner, jobID, isJobActive)
	if err != nil {
		return nil, false, err
	}
	record.JobID = jobID
	record.OwnerToken = rotated.OwnerToken
	tx := rehydrateNewGameTransaction(dataDir, record, rotated.OwnerToken)
	if err := tx.persist(); err != nil {
		return nil, false, err
	}
	return tx, true, nil
}

func rehydrateNewGameTransaction(dataDir string, record NewGameTransactionRecord, ownerToken string) *newGameTransaction {
	return &newGameTransaction{
		dataDir:     dataDir,
		dir:         filepath.Join(newGameTransactionsDir(dataDir), record.TransactionID),
		record:      record,
		ownerToken:  ownerToken,
		writeJSON:   atomicWriteValidatedJSON,
		writeState:  atomicWriteValidatedJSON,
		restoreFile: restoreNewGameFile,
		restoreMods: restoreNewGameMods,
	}
}

func findNewGameTransactionByRequest(dataDir, requestID string) (*NewGameTransactionRecord, error) {
	entries, err := os.ReadDir(newGameTransactionsDir(dataDir))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var match *NewGameTransactionRecord
	for _, entry := range entries {
		if !entry.IsDir() || !isValidNewGameTransactionID(entry.Name()) {
			continue
		}
		record, loadErr := LoadNewGameTransaction(dataDir, entry.Name())
		if errors.Is(loadErr, os.ErrNotExist) {
			continue
		}
		if loadErr != nil {
			return nil, newGameOwnerRecoveryError("事务历史包含损坏记录", loadErr)
		}
		if record.SchemaVersion < 2 || record.RequestID != requestID {
			continue
		}
		if match != nil && match.TransactionID != record.TransactionID {
			return nil, newGameOwnerRecoveryError("同一请求 ID 对应多个事务记录", nil)
		}
		copy := record
		match = &copy
	}
	return match, nil
}

func claimNewGameOwner(dataDir string, owner NewGameOwnerRecord) (claimed bool, retErr error) {
	if err := ensureNewGameControlDir(dataDir); err != nil {
		return false, err
	}
	if err := recoverLegacyEmptyNewGameOwner(dataDir); err != nil {
		return false, err
	}
	if _, err := os.Stat(newGameOwnerRotationDir(dataDir)); err == nil {
		return false, newGameOwnerConflict("new_game_owner_busy", "事务 owner 正在变更", nil)
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	if _, err := LoadNewGameOwner(dataDir); err == nil {
		return false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, newGameOwnerRecoveryError("持久 owner 已存在但无法验证", err)
	}

	stagingDir, err := stageNewGameOwnerClaim(dataDir, owner)
	if err != nil {
		return false, err
	}
	defer func() {
		cleanupErr := cleanupNewGameOwnerClaimStaging(stagingDir)
		if cleanupErr != nil && retErr == nil {
			claimed = false
			retErr = newGameOwnerRecoveryError("清理未发布的 owner staging 失败", cleanupErr)
		}
	}()
	return publishNewGameOwnerClaim(dataDir, stagingDir)
}

// stageNewGameOwnerClaim makes the complete claim durable before the fixed
// owner path becomes visible. A crash here leaves an inert, uniquely named
// sibling directory; it never constitutes ownership and never blocks a later
// atomic publication.
func stageNewGameOwnerClaim(dataDir string, owner NewGameOwnerRecord) (string, error) {
	if err := ensureNewGameControlDir(dataDir); err != nil {
		return "", err
	}
	if err := validateNewGameOwnerRecord(owner); err != nil {
		return "", err
	}
	stagingDir, err := os.MkdirTemp(controlDir(dataDir), newGameOwnerClaimStagingPrefix)
	if err != nil {
		return "", err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = cleanupNewGameOwnerClaimStaging(stagingDir)
		}
	}()
	if err := os.Chmod(stagingDir, 0o700); err != nil {
		return "", err
	}
	stagedPath := filepath.Join(stagingDir, "owner.json")
	if err := persistNewGameOwnerAt(stagedPath, owner); err != nil {
		return "", err
	}
	staged, err := loadNewGameOwnerAt(stagedPath)
	if err != nil {
		return "", fmt.Errorf("verify staged new-game owner: %w", err)
	}
	if staged.TransactionID != owner.TransactionID || staged.OwnerToken != owner.OwnerToken ||
		staged.RequestID != owner.RequestID || staged.ConfigSHA256 != owner.ConfigSHA256 ||
		staged.InstanceDataDirHash != owner.InstanceDataDirHash || staged.JobID != owner.JobID ||
		staged.ExecutorID != owner.ExecutorID || staged.State != owner.State ||
		!staged.CreatedAt.Equal(owner.CreatedAt) || !staged.UpdatedAt.Equal(owner.UpdatedAt) {
		return "", errors.New("staged new-game owner identity does not match claim")
	}
	if runtime.GOOS != "windows" {
		info, statErr := os.Stat(stagedPath)
		if statErr != nil {
			return "", statErr
		}
		if info.Mode().Perm() != 0o600 {
			return "", fmt.Errorf("staged new-game owner mode is %04o, want 0600", info.Mode().Perm())
		}
		if err := syncNewGameOwnerFile(stagedPath); err != nil {
			return "", fmt.Errorf("sync staged new-game owner file: %w", err)
		}
	}
	if err := syncNewGameOwnerDirectory(stagingDir); err != nil {
		return "", fmt.Errorf("sync staged new-game owner: %w", err)
	}
	cleanup = false
	return stagingDir, nil
}

// publishNewGameOwnerClaim is the ownership linearization point. It does not
// infer a lost race from a platform-specific rename error; it re-reads the
// fixed path so Windows and Unix preserve the same winner contract.
func publishNewGameOwnerClaim(dataDir, stagingDir string) (bool, error) {
	if !isNewGameOwnerClaimStagingPath(dataDir, stagingDir) {
		return false, errors.New("owner staging path is outside the control directory")
	}
	if _, err := loadNewGameOwnerAt(filepath.Join(stagingDir, "owner.json")); err != nil {
		return false, fmt.Errorf("validate owner staging before publish: %w", err)
	}
	if _, err := LoadNewGameOwner(dataDir); err == nil {
		return false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, newGameOwnerRecoveryError("目标 owner 现场不确定，禁止发布 staging", err)
	}

	if err := renameNewGameOwnerNoReplace(stagingDir, newGameOwnerDir(dataDir)); err != nil {
		if _, loadErr := LoadNewGameOwner(dataDir); loadErr == nil {
			return false, nil
		} else if !errors.Is(loadErr, os.ErrNotExist) {
			return false, newGameOwnerRecoveryError("owner 原子发布竞争后的目标现场无法验证", errors.Join(err, loadErr))
		}
		return false, err
	}
	if err := syncNewGameOwnerDirectory(controlDir(dataDir)); err != nil {
		return true, fmt.Errorf("sync published new-game owner: %w", err)
	}
	return true, nil
}

func isNewGameOwnerClaimStagingPath(dataDir, stagingDir string) bool {
	controlAbs, err := filepath.Abs(filepath.Clean(controlDir(dataDir)))
	if err != nil {
		return false
	}
	stagingAbs, err := filepath.Abs(filepath.Clean(stagingDir))
	if err != nil {
		return false
	}
	parentMatches := filepath.Dir(stagingAbs) == controlAbs
	if runtime.GOOS == "windows" {
		parentMatches = strings.EqualFold(filepath.Dir(stagingAbs), controlAbs)
	}
	return parentMatches && strings.HasPrefix(filepath.Base(stagingAbs), newGameOwnerClaimStagingPrefix)
}

func cleanupNewGameOwnerClaimStaging(stagingDir string) error {
	info, err := os.Lstat(stagingDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("owner staging is not a plain directory")
	}
	ownerPath := filepath.Join(stagingDir, "owner.json")
	if err := os.Remove(ownerPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Remove(stagingDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func syncNewGameOwnerDirectory(path string) error {
	// Windows doesn't expose a portable directory-fsync operation through
	// os.File.Sync. owner.json itself is fsynced before publication, and rename
	// remains the atomic visibility boundary there.
	if runtime.GOOS == "windows" {
		return nil
	}
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func syncNewGameOwnerFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return file.Sync()
}

func persistNewGameOwner(dataDir string, owner NewGameOwnerRecord) error {
	return persistNewGameOwnerAt(newGameOwnerPath(dataDir), owner)
}

func persistNewGameOwnerAt(path string, owner NewGameOwnerRecord) error {
	payload, err := json.MarshalIndent(owner, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteValidatedJSON(path, payload, 0o600)
}

func LoadNewGameOwner(dataDir string) (NewGameOwnerRecord, error) {
	owner, err := loadNewGameOwnerAt(newGameOwnerPath(dataDir))
	if errors.Is(err, os.ErrNotExist) {
		if _, rotationErr := os.Stat(newGameOwnerRotationDir(dataDir)); rotationErr == nil {
			return NewGameOwnerRecord{}, newGameOwnerConflict("new_game_owner_busy", "事务 owner token 正在轮换", nil)
		}
		if _, dirErr := os.Stat(newGameOwnerDir(dataDir)); dirErr == nil {
			return NewGameOwnerRecord{}, fmt.Errorf("owner directory exists without owner.json")
		}
		return NewGameOwnerRecord{}, os.ErrNotExist
	}
	if err != nil {
		return NewGameOwnerRecord{}, err
	}
	return owner, nil
}

func loadNewGameOwnerAt(path string) (NewGameOwnerRecord, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return NewGameOwnerRecord{}, err
	}
	var owner NewGameOwnerRecord
	if err := json.Unmarshal(payload, &owner); err != nil {
		return NewGameOwnerRecord{}, fmt.Errorf("parse new-game owner: %w", err)
	}
	if err := validateNewGameOwnerRecord(owner); err != nil {
		return NewGameOwnerRecord{}, err
	}
	return owner, nil
}

func validateNewGameOwnerRecord(owner NewGameOwnerRecord) error {
	if owner.SchemaVersion != newGameOwnerSchemaVersion || !isValidNewGameTransactionID(owner.TransactionID) ||
		!isValidNewGameSHA256(owner.OwnerToken) || !isValidNewGameSHA256(owner.InstanceDataDirHash) ||
		!isValidNewGameSHA256(owner.ConfigSHA256) || validateNewGameRequestID(owner.RequestID) != nil ||
		(owner.State != newGameOwnerStateReserved && owner.State != newGameOwnerStateActive) {
		return fmt.Errorf("new-game owner identity is invalid")
	}
	return nil
}

func resumeNewGameOwner(
	dataDir string,
	owner NewGameOwnerRecord,
	jobID string,
	isJobActive func(string) (bool, error),
) (NewGameOwnerRecord, error) {
	executorID, err := currentNewGameExecutorID()
	if err != nil {
		return NewGameOwnerRecord{}, err
	}
	// ExecutorID is diagnostic evidence, not a lease expiry signal. A second
	// Panel process can overlap during an updater cutover or accidental duplicate
	// deployment, so every takeover must prove the persisted job is terminal.
	if isJobActive == nil {
		message := "事务 owner 的旧任务状态未经验证，禁止接管"
		if owner.ExecutorID == executorID {
			message = "当前 Panel 进程中已有执行者持有事务 owner"
		}
		return NewGameOwnerRecord{}, newGameOwnerConflict("new_game_in_progress", message, &owner)
	}
	if strings.TrimSpace(owner.JobID) == "" {
		return NewGameOwnerRecord{}, newGameOwnerRecoveryError("owner 未记录可验证的旧任务 ID", nil)
	}
	active, checkErr := isJobActive(owner.JobID)
	if checkErr != nil {
		return NewGameOwnerRecord{}, &NewGameOwnerError{Code: "new_game_owner_job_check_failed", Message: "验证旧事务任务状态失败", Owner: &owner, Cause: checkErr}
	}
	if active {
		return NewGameOwnerRecord{}, newGameOwnerConflict("new_game_in_progress", "旧事务任务仍处于活动状态，禁止接管 owner", &owner)
	}
	return rotateNewGameOwner(dataDir, owner, jobID, executorID)
}

func rotateNewGameOwner(dataDir string, owner NewGameOwnerRecord, jobID, executorID string) (NewGameOwnerRecord, error) {
	current, err := LoadNewGameOwner(dataDir)
	if err != nil {
		return NewGameOwnerRecord{}, err
	}
	if current.TransactionID != owner.TransactionID || current.OwnerToken != owner.OwnerToken {
		return NewGameOwnerRecord{}, newGameOwnerConflict("new_game_owner_lost", "事务 owner 已被新的执行者接管", &current)
	}
	rotationDir := newGameOwnerRotationDir(dataDir)
	if err := os.Rename(newGameOwnerDir(dataDir), rotationDir); err != nil {
		return NewGameOwnerRecord{}, err
	}
	restored := false
	defer func() {
		if !restored {
			_ = os.Rename(rotationDir, newGameOwnerDir(dataDir))
		}
	}()
	token, err := newGameRandomHex(32)
	if err != nil {
		return NewGameOwnerRecord{}, err
	}
	current.OwnerToken = token
	current.JobID = jobID
	current.ExecutorID = executorID
	current.State = newGameOwnerStateActive
	current.UpdatedAt = time.Now().UTC()
	if err := persistNewGameOwnerAt(filepath.Join(rotationDir, "owner.json"), current); err != nil {
		return NewGameOwnerRecord{}, err
	}
	if err := os.Rename(rotationDir, newGameOwnerDir(dataDir)); err != nil {
		return NewGameOwnerRecord{}, err
	}
	restored = true
	if err := assertNewGameOwner(dataDir, current.TransactionID, token); err != nil {
		return NewGameOwnerRecord{}, err
	}
	return current, nil
}

func assertNewGameOwner(dataDir, transactionID, ownerToken string) error {
	owner, err := LoadNewGameOwner(dataDir)
	if err != nil {
		return newGameOwnerConflict("new_game_owner_lost", "事务 owner 缺失或不可读", nil).withCause(err)
	}
	dataDirHash, err := newGameInstanceDataDirHash(dataDir)
	if err != nil {
		return newGameOwnerConflict("new_game_owner_lost", "无法验证事务 owner 的实例路径", &owner).withCause(err)
	}
	if owner.TransactionID != transactionID || owner.OwnerToken != ownerToken || owner.InstanceDataDirHash != dataDirHash {
		return newGameOwnerConflict("new_game_owner_lost", "事务 owner token 不匹配", &owner)
	}
	return nil
}

func (tx *newGameTransaction) assertOwner() error {
	if tx == nil || tx.ownerToken == "" {
		return nil
	}
	return assertNewGameOwner(tx.dataDir, tx.record.TransactionID, tx.ownerToken)
}

func releaseNewGameOwner(tx *newGameTransaction) error {
	if tx == nil || tx.ownerToken == "" {
		return nil
	}
	diskRecord, err := LoadNewGameTransaction(tx.dataDir, tx.record.TransactionID)
	if err != nil {
		return newGameOwnerRecoveryError("释放 owner 前无法读取事务终态", err)
	}
	if !isTerminalNewGameOwnerStage(diskRecord.Stage) {
		return newGameOwnerConflict("new_game_owner_not_terminal", "事务尚未成功或完整回滚，禁止释放 owner", nil)
	}
	if diskRecord.OwnerToken != tx.ownerToken {
		return newGameOwnerConflict("new_game_owner_lost", "事务终态不是由当前 owner token 写入", nil)
	}
	if err := tx.assertOwner(); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			tx.ownerToken = ""
			return nil
		}
		return err
	}
	releasedDir := newGameOwnerDir(tx.dataDir) + ".released-" + tx.ownerToken
	if _, err := os.Stat(releasedDir); err == nil {
		suffix, randomErr := newGameRandomHex(4)
		if randomErr != nil {
			return randomErr
		}
		releasedDir += "-" + suffix
	}
	if err := os.Rename(newGameOwnerDir(tx.dataDir), releasedDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			tx.ownerToken = ""
			return nil
		}
		return err
	}
	tx.ownerToken = ""
	ownerPath := filepath.Join(releasedDir, "owner.json")
	if err := os.Remove(ownerPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Remove(releasedDir); err != nil {
		return err
	}
	return nil
}

// recoverLegacyEmptyNewGameOwner removes only the exact crash artifact left by
// the old "mkdir fixed owner, then write owner.json" protocol. The directory
// must predate this process, remain empty across an atomic quarantine, and have
// no nonterminal transaction, marker, catalog request, or unmatched runtime
// progress. Anything else is an operator-visible recovery condition.
func recoverLegacyEmptyNewGameOwner(dataDir string) error {
	ownerDir := newGameOwnerDir(dataDir)
	info, err := os.Lstat(ownerDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return newGameOwnerRecoveryError("检查遗留 owner 目录失败", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return newGameOwnerRecoveryError("owner 路径不是普通目录，禁止自动恢复", nil)
	}
	if _, err := os.Lstat(newGameOwnerPath(dataDir)); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return newGameOwnerRecoveryError("检查遗留 owner.json 失败", err)
	}
	if _, err := os.Lstat(newGameOwnerRotationDir(dataDir)); err == nil {
		return newGameOwnerRecoveryError("owner 轮换目录与空 owner 同时存在，禁止自动恢复", nil)
	} else if !errors.Is(err, os.ErrNotExist) {
		return newGameOwnerRecoveryError("检查 owner 轮换现场失败", err)
	}
	entries, err := os.ReadDir(ownerDir)
	if err != nil {
		return newGameOwnerRecoveryError("读取遗留 owner 目录失败", err)
	}
	if len(entries) != 0 {
		return newGameOwnerRecoveryError("owner 目录缺少 owner.json 但包含未知文件，禁止自动恢复", nil)
	}
	if !info.ModTime().Before(newGameOwnerProcessStartedAt) || time.Since(info.ModTime()) < newGameOwnerLegacyClaimMinAge {
		return newGameOwnerRecoveryError("空 owner 目录可能仍由当前或重叠进程初始化，禁止自动恢复", nil)
	}
	uncertain, err := hasUncertainLegacyNewGameOwnerEvidence(dataDir)
	if err != nil {
		return newGameOwnerRecoveryError("验证空 owner 对应事务现场失败", err)
	}
	if uncertain {
		return newGameOwnerRecoveryError("空 owner 旁存在事务、marker 或创建进展，禁止自动恢复", nil)
	}

	recoveryID, err := newGameRandomHex(8)
	if err != nil {
		return err
	}
	quarantineDir := filepath.Join(controlDir(dataDir), newGameOwnerEmptyRecoveryPrefix+recoveryID)
	if err := os.Rename(ownerDir, quarantineDir); err != nil {
		if _, loadErr := LoadNewGameOwner(dataDir); loadErr == nil {
			return nil
		}
		return newGameOwnerRecoveryError("隔离遗留空 owner 目录失败", err)
	}
	restore := func(cause error) error {
		restoreErr := restoreQuarantinedLegacyNewGameOwner(dataDir, quarantineDir)
		if restoreErr != nil {
			return newGameOwnerRecoveryError("空 owner 现场复核失败且无法恢复隔离目录", errors.Join(cause, restoreErr))
		}
		return newGameOwnerRecoveryError("空 owner 现场复核失败，已恢复原目录", cause)
	}
	entries, err = os.ReadDir(quarantineDir)
	if err != nil {
		return restore(err)
	}
	if len(entries) != 0 {
		return restore(errors.New("quarantined legacy owner directory is no longer empty"))
	}
	uncertain, err = hasUncertainLegacyNewGameOwnerEvidence(dataDir)
	if err != nil {
		return restore(err)
	}
	if uncertain {
		return restore(errors.New("new-game evidence appeared while quarantining legacy owner"))
	}
	if err := os.Remove(quarantineDir); err != nil {
		return restore(err)
	}
	if err := syncNewGameOwnerDirectory(controlDir(dataDir)); err != nil {
		return newGameOwnerRecoveryError("同步遗留空 owner 清理结果失败", err)
	}
	return nil
}

func restoreQuarantinedLegacyNewGameOwner(dataDir, quarantineDir string) error {
	if _, err := os.Lstat(newGameOwnerDir(dataDir)); err == nil {
		return errors.New("fixed owner path was claimed while legacy directory was quarantined")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Rename(quarantineDir, newGameOwnerDir(dataDir)); err != nil {
		return err
	}
	return syncNewGameOwnerDirectory(controlDir(dataDir))
}

func hasUncertainLegacyNewGameOwnerEvidence(dataDir string) (bool, error) {
	terminalTransactions := make(map[string]struct{})
	entries, err := os.ReadDir(newGameTransactionsDir(dataDir))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return true, err
	}
	for _, entry := range entries {
		if !entry.IsDir() || !isValidNewGameTransactionID(entry.Name()) {
			return true, fmt.Errorf("unknown entry in new-game transaction root: %s", entry.Name())
		}
		record, loadErr := LoadNewGameTransaction(dataDir, entry.Name())
		if loadErr != nil {
			return true, loadErr
		}
		if !isTerminalNewGameOwnerStage(record.Stage) {
			return true, nil
		}
		terminalTransactions[record.TransactionID] = struct{}{}
	}

	if _, err := os.Lstat(newGamePendingPath(dataDir)); err == nil {
		return true, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return true, err
	}
	if raw, err := os.ReadFile(farmCatalogRequestPath(dataDir)); err == nil {
		var request runtimeCatalogRequest
		if json.Unmarshal(raw, &request) != nil || request.TransactionID == "" {
			return true, errors.New("runtime catalog request is invalid")
		}
		if _, terminal := terminalTransactions[request.TransactionID]; !terminal {
			return true, nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return true, err
	}
	status, err := readNewGameRuntimeStatusSnapshot(dataDir)
	if err != nil {
		return true, err
	}
	if status.TransactionID != "" && (status.CreationObserved || strings.EqualFold(status.State, "save-creating")) {
		if _, terminal := terminalTransactions[status.TransactionID]; !terminal {
			return true, nil
		}
	}
	return false, nil
}

func recoverNewGameOwnerRotation(dataDir string) error {
	if _, err := os.Stat(newGameOwnerDir(dataDir)); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	rotationDir := newGameOwnerRotationDir(dataDir)
	if _, err := os.Stat(rotationDir); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	if err := os.Rename(rotationDir, newGameOwnerDir(dataDir)); err != nil {
		return err
	}
	return syncNewGameOwnerDirectory(controlDir(dataDir))
}

func (tx *newGameTransaction) releaseOwner() error {
	return releaseNewGameOwner(tx)
}

func releaseTerminalNewGameOwner(dataDir string, owner NewGameOwnerRecord) error {
	record, err := LoadNewGameTransaction(dataDir, owner.TransactionID)
	if err != nil {
		return err
	}
	if !isTerminalNewGameOwnerStage(record.Stage) {
		return errors.New("owner transaction is not terminal")
	}
	tx := rehydrateNewGameTransaction(dataDir, record, owner.OwnerToken)
	return releaseNewGameOwner(tx)
}

func isTerminalNewGameOwnerStage(stage NewGameTransactionState) bool {
	return stage == newGameStateSuccess || stage == newGameStateRolledBack
}

func newGameOwnerConflict(code, message string, owner *NewGameOwnerRecord) *NewGameOwnerError {
	return &NewGameOwnerError{Code: code, Message: message, Owner: owner}
}

func newGameOwnerRecoveryError(message string, cause error) *NewGameOwnerError {
	return &NewGameOwnerError{Code: "new_game_recovery_required", Message: message, Cause: cause}
}

func (e *NewGameOwnerError) withCause(cause error) *NewGameOwnerError {
	e.Cause = cause
	return e
}
