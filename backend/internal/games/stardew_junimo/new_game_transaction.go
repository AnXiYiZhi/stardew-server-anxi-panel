package stardew_junimo

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
)

const (
	newGameTransactionSchemaVersion = 1
	newGameMarkerSchemaVersion      = 1
	newGameMarkerTTL                = 30 * time.Minute
)

type NewGameTransactionState string

const (
	newGameStatePreparing      NewGameTransactionState = "preparing"
	newGameStateConfigured     NewGameTransactionState = "configured"
	newGameStateMarkerWritten  NewGameTransactionState = "marker_written"
	newGameStateModsPrepared   NewGameTransactionState = "mods_prepared"
	newGameStateComposeUp      NewGameTransactionState = "compose_started"
	newGameStateCatalogAsked   NewGameTransactionState = "catalog_requested"
	newGameStateCatalogReady   NewGameTransactionState = "catalog_validated"
	newGameStateCommandCalled  NewGameTransactionState = "command_called"
	newGameStateObserving      NewGameTransactionState = "observing"
	newGameStateProfilePending NewGameTransactionState = "profile_commit_pending"
	newGameStateSuccess        NewGameTransactionState = "success"
	newGameStateFailed         NewGameTransactionState = "failed"
	newGameStateUnknown        NewGameTransactionState = "unknown"
	newGameStateAmbiguous      NewGameTransactionState = "ambiguous"
	newGameStateRollbackFail   NewGameTransactionState = "rollback_failed"
)

type NewGameTransactionError struct {
	Code          string
	Message       string
	Cause         error
	RollbackError error
}

func (e *NewGameTransactionError) Error() string {
	if e.RollbackError != nil {
		return fmt.Sprintf("%s: %s: %v (rollback failed: %v)", e.Code, e.Message, e.Cause, e.RollbackError)
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return e.Code + ": " + e.Message
}

func (e *NewGameTransactionError) Unwrap() error { return e.Cause }

type newGameFileSnapshot struct {
	Exists bool   `json:"exists"`
	Data   []byte `json:"data,omitempty"`
}

type newGameModSnapshot struct {
	FolderName string `json:"folderName"`
	UniqueID   string `json:"uniqueId,omitempty"`
	Enabled    bool   `json:"enabled"`
}

type NewGameTransactionRecord struct {
	SchemaVersion       int                     `json:"schemaVersion"`
	TransactionID       string                  `json:"transactionId"`
	InstanceDataDirHash string                  `json:"instanceDataDirHash,omitempty"`
	Config              registry.NewGameConfig  `json:"config"`
	RequestedFarmType   string                  `json:"requestedFarmType"`
	CreatedAt           time.Time               `json:"createdAt"`
	UpdatedAt           time.Time               `json:"updatedAt"`
	Stage               NewGameTransactionState `json:"stage"`
	Result              string                  `json:"result,omitempty"`
	CommandCalled       bool                    `json:"commandCalled"`
	CommandCalledAt     *time.Time              `json:"commandCalledAt,omitempty"`
	PreexistingSaveDirs []string                `json:"preexistingSaveDirs"`
	ActiveSave          string                  `json:"activeSave,omitempty"`
	ServerSettings      newGameFileSnapshot     `json:"serverSettings"`
	ServerInit          newGameFileSnapshot     `json:"serverInit"`
	Gameloader          newGameFileSnapshot     `json:"gameloader"`
	PendingMarker       newGameFileSnapshot     `json:"pendingMarker"`
	ModProfiles         newGameFileSnapshot     `json:"modProfiles"`
	CatalogRequest      newGameFileSnapshot     `json:"catalogRequest"`
	RuntimeOptions      newGameFileSnapshot     `json:"runtimeOptions"`
	Mods                []newGameModSnapshot    `json:"mods"`
	ExpectedFingerprint string                  `json:"expectedModFingerprint,omitempty"`
	ResolvedFarmType    string                  `json:"resolvedFarmType,omitempty"`
	EnabledModKeys      []string                `json:"enabledModKeys,omitempty"`
	ModSelection        *NewGameModSelection    `json:"modSelection,omitempty"`
	CreatedSave         string                  `json:"createdSave,omitempty"`
	DetectedSaveDirs    []string                `json:"detectedSaveDirs,omitempty"`
	QuarantinedSaveDirs []string                `json:"quarantinedSaveDirs,omitempty"`
	ErrorCode           string                  `json:"errorCode,omitempty"`
	ErrorMessage        string                  `json:"errorMessage,omitempty"`
	RollbackCompleted   bool                    `json:"rollbackCompleted"`
	RollbackError       string                  `json:"rollbackError,omitempty"`
}

type newGamePendingMarker struct {
	SchemaVersion     int       `json:"schemaVersion"`
	TransactionID     string    `json:"transactionId"`
	RequestedFarmType string    `json:"requestedFarmType"`
	CreatedAt         time.Time `json:"createdAt"`
	ExpiresAt         time.Time `json:"expiresAt"`
	State             string    `json:"state"`
}

type newGameTransaction struct {
	dataDir       string
	dir           string
	record        NewGameTransactionRecord
	writeJSON     func(string, []byte, os.FileMode) error
	restoreFile   func(string, newGameFileSnapshot) error
	restoreMods   func(string, []newGameModSnapshot) error
	quarantineNew func() error
}

func newGameTransactionsDir(dataDir string) string {
	return filepath.Join(controlDir(dataDir), "new-game-transactions")
}

func gameloaderPath(dataDir string) string {
	return filepath.Join(savesDir(dataDir), ".smapi", "mod-data", "junimohost.server", "junimohost.gameloader.json")
}

func farmCatalogRequestPath(dataDir string) string {
	return filepath.Join(controlDir(dataDir), "farm-catalog-request.json")
}

func runtimeOptionsPath(dataDir string) string {
	return filepath.Join(controlDir(dataDir), "options.json")
}

func beginNewGameTransaction(dataDir string, cfg registry.NewGameConfig) (*newGameTransaction, error) {
	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return nil, fmt.Errorf("generate transaction id: %w", err)
	}
	id := hex.EncodeToString(idBytes)
	root := newGameTransactionsDir(dataDir)
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("create transaction root: %w", err)
	}
	if err := os.Chmod(root, 0o700); err != nil {
		return nil, fmt.Errorf("secure transaction root: %w", err)
	}
	dir := filepath.Join(root, id)
	if err := os.Mkdir(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create transaction directory: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return nil, fmt.Errorf("secure transaction directory: %w", err)
	}

	now := time.Now().UTC()
	names, err := listSaveDirs(dataDir)
	if err != nil {
		return nil, fmt.Errorf("snapshot save directories: %w", err)
	}
	sort.Strings(names)
	mods, err := listPhysicalMods(dataDir)
	if err != nil {
		return nil, fmt.Errorf("snapshot mod state: %w", err)
	}
	modSnapshot := make([]newGameModSnapshot, 0, len(mods))
	for _, mod := range mods {
		modSnapshot = append(modSnapshot, newGameModSnapshot{FolderName: mod.FolderName, UniqueID: mod.UniqueID, Enabled: mod.Enabled})
	}
	sort.Slice(modSnapshot, func(i, j int) bool {
		return strings.ToLower(modSnapshot[i].FolderName) < strings.ToLower(modSnapshot[j].FolderName)
	})

	tx := &newGameTransaction{
		dataDir: dataDir, dir: dir,
		writeJSON:   atomicWriteValidatedJSON,
		restoreFile: restoreNewGameFile,
		restoreMods: restoreNewGameMods,
	}
	tx.record = NewGameTransactionRecord{
		SchemaVersion:       newGameTransactionSchemaVersion,
		TransactionID:       id,
		Config:              cfg,
		RequestedFarmType:   cfg.FarmType,
		CreatedAt:           now,
		UpdatedAt:           now,
		Stage:               newGameStatePreparing,
		PreexistingSaveDirs: append([]string{}, names...),
		ActiveSave:          GetActiveSaveName(dataDir),
		Mods:                modSnapshot,
	}
	for path, dst := range map[string]*newGameFileSnapshot{
		serverSettingsPath(dataDir):     &tx.record.ServerSettings,
		serverInitPath(dataDir):         &tx.record.ServerInit,
		gameloaderPath(dataDir):         &tx.record.Gameloader,
		newGamePendingPath(dataDir):     &tx.record.PendingMarker,
		modProfileFilePath(dataDir):     &tx.record.ModProfiles,
		farmCatalogRequestPath(dataDir): &tx.record.CatalogRequest,
		runtimeOptionsPath(dataDir):     &tx.record.RuntimeOptions,
	} {
		if err := snapshotNewGameFile(path, dst); err != nil {
			return nil, err
		}
	}
	if err := tx.persist(); err != nil {
		return nil, err
	}
	return tx, nil
}

func snapshotNewGameFile(path string, dst *newGameFileSnapshot) error {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("snapshot %s: %w", filepath.Base(path), err)
	}
	dst.Exists = true
	dst.Data = data
	return nil
}

func (tx *newGameTransaction) persist() error {
	tx.record.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(tx.record, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal transaction state: %w", err)
	}
	return atomicWriteValidatedJSON(filepath.Join(tx.dir, "transaction.json"), data, 0o600)
}

func LoadNewGameTransaction(dataDir, transactionID string) (NewGameTransactionRecord, error) {
	if len(transactionID) != 32 {
		return NewGameTransactionRecord{}, fmt.Errorf("invalid transaction id")
	}
	if _, err := hex.DecodeString(transactionID); err != nil {
		return NewGameTransactionRecord{}, fmt.Errorf("invalid transaction id")
	}
	data, err := os.ReadFile(filepath.Join(newGameTransactionsDir(dataDir), transactionID, "transaction.json"))
	if err != nil {
		return NewGameTransactionRecord{}, err
	}
	var record NewGameTransactionRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return NewGameTransactionRecord{}, fmt.Errorf("parse transaction state: %w", err)
	}
	return record, nil
}

func (tx *newGameTransaction) prepareConfigAndMarker() error {
	settings, err := newGameServerSettingsJSON(tx.record.Config)
	if err != nil {
		return &NewGameTransactionError{Code: "new_game_config_write_failed", Message: "生成新建存档配置失败", Cause: err}
	}
	initData, err := newGameInitConfigJSONForTransaction(tx.record.Config, tx.record.TransactionID)
	if err != nil {
		return &NewGameTransactionError{Code: "new_game_config_write_failed", Message: "生成新建存档初始化配置失败", Cause: err}
	}
	if err := tx.writeJSON(serverSettingsPath(tx.dataDir), settings, 0o644); err != nil {
		return &NewGameTransactionError{Code: "new_game_config_write_failed", Message: "写入 server-settings.json 失败", Cause: err}
	}
	if err := tx.writeJSON(serverInitPath(tx.dataDir), initData, 0o644); err != nil {
		return &NewGameTransactionError{Code: "new_game_config_write_failed", Message: "写入 server-init.json 失败", Cause: err}
	}
	tx.record.Stage = newGameStateConfigured
	if err := tx.persist(); err != nil {
		return err
	}

	now := time.Now().UTC()
	marker := newGamePendingMarker{
		SchemaVersion: newGameMarkerSchemaVersion, TransactionID: tx.record.TransactionID,
		RequestedFarmType: tx.record.RequestedFarmType, CreatedAt: now,
		ExpiresAt: now.Add(newGameMarkerTTL), State: "pending",
	}
	markerData, err := json.MarshalIndent(marker, "", "  ")
	if err != nil {
		return &NewGameTransactionError{Code: "new_game_marker_write_failed", Message: "生成 pending marker 失败", Cause: err}
	}
	if err := tx.writeJSON(newGamePendingPath(tx.dataDir), markerData, 0o644); err != nil {
		return &NewGameTransactionError{Code: "new_game_marker_write_failed", Message: "写入 pending marker 失败", Cause: err}
	}
	tx.record.Stage = newGameStateMarkerWritten
	return tx.persist()
}

func atomicWriteValidatedJSON(path string, data []byte, mode os.FileMode) error {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("validate JSON: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".new-game-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if err := tmp.Chmod(mode); err != nil {
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
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	return os.Chmod(path, mode)
}

func (tx *newGameTransaction) mark(stage NewGameTransactionState) error {
	tx.record.Stage = stage
	return tx.persist()
}

func (tx *newGameTransaction) markCommandCalled() error {
	if tx.record.CommandCalled {
		return nil
	}
	now := time.Now().UTC()
	tx.record.CommandCalled = true
	tx.record.CommandCalledAt = &now
	tx.record.Stage = newGameStateCommandCalled
	return tx.persist()
}

func (tx *newGameTransaction) complete(saveName string) error {
	tx.record.CreatedSave = saveName
	tx.record.Result = "success"
	tx.record.Stage = newGameStateSuccess
	tx.record.ErrorCode = ""
	tx.record.ErrorMessage = ""
	if err := tx.persist(); err != nil {
		return err
	}
	// The legacy Control Mod only checks marker existence and may already have
	// removed it on SaveLoaded. Go success is based on filesystem/XML validation.
	_ = os.Remove(newGamePendingPath(tx.dataDir))
	return nil
}

func (tx *newGameTransaction) setFailure(stage NewGameTransactionState, code string, cause error) {
	tx.record.Stage = stage
	tx.record.Result = string(stage)
	tx.record.ErrorCode = code
	if cause != nil {
		tx.record.ErrorMessage = cause.Error()
	}
	_ = tx.persist()
}

func (tx *newGameTransaction) rollback(cause error, code string, stage NewGameTransactionState) error {
	tx.setFailure(stage, code, cause)
	var rollbackErrors []error
	if code != "mod_profile_commit_failed" {
		quarantine := tx.quarantineNewSaveDirs
		if tx.quarantineNew != nil {
			quarantine = tx.quarantineNew
		}
		if err := quarantine(); err != nil {
			rollbackErrors = append(rollbackErrors, fmt.Errorf("quarantine new saves: %w", err))
		}
	}
	for path, snapshot := range map[string]newGameFileSnapshot{
		serverSettingsPath(tx.dataDir):     tx.record.ServerSettings,
		serverInitPath(tx.dataDir):         tx.record.ServerInit,
		gameloaderPath(tx.dataDir):         tx.record.Gameloader,
		newGamePendingPath(tx.dataDir):     tx.record.PendingMarker,
		modProfileFilePath(tx.dataDir):     tx.record.ModProfiles,
		farmCatalogRequestPath(tx.dataDir): tx.record.CatalogRequest,
		runtimeOptionsPath(tx.dataDir):     tx.record.RuntimeOptions,
	} {
		if err := tx.restoreFile(path, snapshot); err != nil {
			rollbackErrors = append(rollbackErrors, err)
		}
	}
	if err := tx.restoreMods(tx.dataDir, tx.record.Mods); err != nil {
		rollbackErrors = append(rollbackErrors, fmt.Errorf("restore mod state: %w", err))
	}
	if len(rollbackErrors) == 0 {
		tx.record.RollbackCompleted = true
		if code == "mod_profile_commit_failed" {
			tx.record.Stage = newGameStateProfilePending
			tx.record.Result = "recoverable"
		}
		_ = tx.persist()
		return nil
	}
	rollbackErr := errors.Join(rollbackErrors...)
	tx.record.Stage = newGameStateRollbackFail
	tx.record.Result = "failed"
	tx.record.RollbackError = rollbackErr.Error()
	_ = tx.persist()
	return rollbackErr
}

func restoreNewGameFile(path string, snapshot newGameFileSnapshot) error {
	if !snapshot.Exists {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %s: %w", filepath.Base(path), err)
		}
		return nil
	}
	return atomicWriteRaw(path, snapshot.Data, 0o644)
}

func atomicWriteRaw(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".new-game-restore-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if err := tmp.Chmod(mode); err != nil {
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
	return os.Rename(tmpPath, path)
}

func restoreNewGameMods(dataDir string, snapshot []newGameModSnapshot) error {
	lock := modProfileLockFor(dataDir)
	lock.Lock()
	defer lock.Unlock()
	current, err := listPhysicalMods(dataDir)
	if err != nil {
		return err
	}
	byFolder := make(map[string]bool, len(current))
	for _, mod := range current {
		byFolder[strings.ToLower(mod.FolderName)] = mod.Enabled
	}
	var errs []error
	for _, prior := range snapshot {
		if enabled, ok := byFolder[strings.ToLower(prior.FolderName)]; ok && enabled != prior.Enabled {
			if err := moveModFolder(dataDir, prior.FolderName, prior.Enabled); err != nil {
				errs = append(errs, fmt.Errorf("restore %s: %w", prior.UniqueID, err))
			}
		}
	}
	return errors.Join(errs...)
}

func (tx *newGameTransaction) newSaveDirs() ([]string, error) {
	current, err := listSaveDirs(tx.dataDir)
	if err != nil {
		return nil, err
	}
	before := make(map[string]struct{}, len(tx.record.PreexistingSaveDirs))
	for _, name := range tx.record.PreexistingSaveDirs {
		before[name] = struct{}{}
	}
	var result []string
	for _, name := range current {
		if _, existed := before[name]; !existed {
			result = append(result, name)
		}
	}
	sort.Strings(result)
	if strings.Join(tx.record.DetectedSaveDirs, "\x00") != strings.Join(result, "\x00") {
		tx.record.DetectedSaveDirs = append([]string{}, result...)
		_ = tx.persist()
	}
	return result, nil
}

func (tx *newGameTransaction) quarantineNewSaveDirs() error {
	names, err := tx.newSaveDirs()
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return nil
	}
	root := filepath.Join(tx.dataDir, ".local-container", "saves-quarantine", "new-game", tx.record.TransactionID)
	if err := os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	for _, name := range names {
		if err := validateSaveName(name); err != nil {
			return err
		}
		src := filepath.Join(savesDir(tx.dataDir), "Saves", name)
		dst := filepath.Join(root, name)
		if err := os.Rename(src, dst); err != nil && !os.IsNotExist(err) {
			return err
		}
		tx.record.QuarantinedSaveDirs = append(tx.record.QuarantinedSaveDirs, name)
	}
	sort.Strings(tx.record.QuarantinedSaveDirs)
	return tx.persist()
}

type newGameFileStability struct {
	size    int64
	modTime time.Time
	count   int
}

func validateStableNewGameSave(dataDir, name, requestedFarm string, previous *newGameFileStability) (bool, error) {
	if err := validateSaveName(name); err != nil {
		return false, err
	}
	mainPath := filepath.Join(savesDir(dataDir), "Saves", name, name)
	stat, err := os.Stat(mainPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if !stat.Mode().IsRegular() || stat.Size() <= 0 {
		return false, nil
	}
	if previous.count == 0 || previous.size != stat.Size() || !previous.modTime.Equal(stat.ModTime()) {
		previous.size, previous.modTime, previous.count = stat.Size(), stat.ModTime(), 1
		return false, nil
	}
	previous.count++
	if previous.count < 2 {
		return false, nil
	}
	file, err := os.Open(mainPath)
	if err != nil {
		return false, err
	}
	decoder := xml.NewDecoder(io.LimitReader(file, maxSingleFileBytes+1))
	for {
		_, decodeErr := decoder.Token()
		if decodeErr == io.EOF {
			break
		}
		if decodeErr != nil {
			_ = file.Close()
			return false, fmt.Errorf("new save XML is invalid: %w", decodeErr)
		}
	}
	_ = file.Close()
	actual := readWhichFarmFromMainFile(filepath.Dir(mainPath), name)
	if actual == "" || actual == "unknown" {
		return false, fmt.Errorf("new save whichFarm is missing or unknown")
	}
	requested, normalizeErr := NormalizeNewGameFarmType(requestedFarm)
	if normalizeErr != nil {
		return false, normalizeErr
	}
	wanted := requested.ID
	if actual != wanted {
		return false, fmt.Errorf("new save farm type mismatch: requested %s, got %s", requestedFarm, actual)
	}
	return true, nil
}

func uniqueNumericSuffixCandidate(pointer string, candidates []string) string {
	idx := strings.LastIndex(pointer, "_")
	if idx < 0 || idx == len(pointer)-1 {
		return ""
	}
	suffix := pointer[idx+1:]
	if _, err := strconv.ParseUint(suffix, 10, 64); err != nil {
		return ""
	}
	match := ""
	for _, candidate := range candidates {
		candidateIdx := strings.LastIndex(candidate, "_")
		if candidateIdx < 0 || candidate[candidateIdx+1:] != suffix {
			continue
		}
		if match != "" {
			return ""
		}
		match = candidate
	}
	return match
}
