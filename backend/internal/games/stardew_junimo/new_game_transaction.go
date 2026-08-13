package stardew_junimo

import (
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

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
)

const (
	newGameTransactionSchemaVersion = 2
	newGameMarkerSchemaVersion      = 1
	newGameMarkerTTL                = 2 * time.Hour
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
	newGameStateFinalizing     NewGameTransactionState = "finalizing"
	newGameStateProfilePending NewGameTransactionState = "profile_commit_pending"
	newGameStateSuccess        NewGameTransactionState = "success"
	newGameStateFailed         NewGameTransactionState = "failed"
	newGameStateUnknown        NewGameTransactionState = "unknown"
	newGameStateAmbiguous      NewGameTransactionState = "ambiguous"
	newGameStateRollingBack    NewGameTransactionState = "rolling_back"
	newGameStateRolledBack     NewGameTransactionState = "rolled_back"
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
	SchemaVersion                 int                          `json:"schemaVersion"`
	TransactionID                 string                       `json:"transactionId"`
	InstanceDataDirHash           string                       `json:"instanceDataDirHash,omitempty"`
	RequestID                     string                       `json:"requestId,omitempty"`
	JobID                         string                       `json:"jobId,omitempty"`
	ConfigSHA256                  string                       `json:"configSha256,omitempty"`
	OwnerToken                    string                       `json:"ownerToken,omitempty"`
	Config                        registry.NewGameConfig       `json:"config"`
	RequestedFarmType             string                       `json:"requestedFarmType"`
	CreatedAt                     time.Time                    `json:"createdAt"`
	UpdatedAt                     time.Time                    `json:"updatedAt"`
	Stage                         NewGameTransactionState      `json:"stage"`
	Result                        string                       `json:"result,omitempty"`
	CommandCalled                 bool                         `json:"commandCalled"`
	CommandCalledAt               *time.Time                   `json:"commandCalledAt,omitempty"`
	PreexistingSaveDirs           []string                     `json:"preexistingSaveDirs"`
	ActiveSave                    string                       `json:"activeSave,omitempty"`
	CreationWriter                string                       `json:"creationWriter,omitempty"`
	InitialGameloaderSave         string                       `json:"initialGameloaderSave,omitempty"`
	InitialRuntimeStatus          NewGameRuntimeStatusSnapshot `json:"initialRuntimeStatus"`
	ProgressObserved              bool                         `json:"progressObserved"`
	ProgressKind                  string                       `json:"progressKind,omitempty"`
	ProgressSave                  string                       `json:"progressSave,omitempty"`
	ProgressControlState          string                       `json:"progressControlState,omitempty"`
	ProgressObservedAt            *time.Time                   `json:"progressObservedAt,omitempty"`
	ProgressAmbiguous             bool                         `json:"progressAmbiguous"`
	CandidateSave                 string                       `json:"candidateSave,omitempty"`
	ServerSettings                newGameFileSnapshot          `json:"serverSettings"`
	ServerInit                    newGameFileSnapshot          `json:"serverInit"`
	Gameloader                    newGameFileSnapshot          `json:"gameloader"`
	PendingMarker                 newGameFileSnapshot          `json:"pendingMarker"`
	ModProfiles                   newGameFileSnapshot          `json:"modProfiles"`
	CatalogRequest                newGameFileSnapshot          `json:"catalogRequest"`
	RuntimeOptions                newGameFileSnapshot          `json:"runtimeOptions"`
	Mods                          []newGameModSnapshot         `json:"mods"`
	ExpectedFingerprint           string                       `json:"expectedModFingerprint,omitempty"`
	ResolvedFarmType              string                       `json:"resolvedFarmType,omitempty"`
	EnabledModKeys                []string                     `json:"enabledModKeys,omitempty"`
	ModSelection                  *NewGameModSelection         `json:"modSelection,omitempty"`
	CreatedSave                   string                       `json:"createdSave,omitempty"`
	DetectedSaveDirs              []string                     `json:"detectedSaveDirs,omitempty"`
	SaveLoadedAt                  *time.Time                   `json:"saveLoadedAt,omitempty"`
	CustomizationVerifiedAt       *time.Time                   `json:"customizationVerifiedAt,omitempty"`
	DurableSaveCommandID          string                       `json:"durableSaveCommandId,omitempty"`
	DurableSaveCommandPublishedAt *time.Time                   `json:"durableSaveCommandPublishedAt,omitempty"`
	DurableGameLoopSavedAt        *time.Time                   `json:"durableGameLoopSavedAt,omitempty"`
	MainSaveSHA256                string                       `json:"mainSaveSha256,omitempty"`
	SaveGameInfoSHA256            string                       `json:"saveGameInfoSha256,omitempty"`
	DiskVerifiedAt                *time.Time                   `json:"diskVerifiedAt,omitempty"`
	QuarantinedSaveDirs           []string                     `json:"quarantinedSaveDirs,omitempty"`
	ErrorCode                     string                       `json:"errorCode,omitempty"`
	ErrorMessage                  string                       `json:"errorMessage,omitempty"`
	RollbackCompleted             bool                         `json:"rollbackCompleted"`
	RollbackError                 string                       `json:"rollbackError,omitempty"`
	RollbackStartedAt             *time.Time                   `json:"rollbackStartedAt,omitempty"`
	RollbackOriginalStage         NewGameTransactionState      `json:"rollbackOriginalStage,omitempty"`
	RollbackPlanReady             bool                         `json:"rollbackPlanReady"`
	RollbackPlannedSaveDirs       []string                     `json:"rollbackPlannedSaveDirs,omitempty"`
	RollbackCurrentStep           string                       `json:"rollbackCurrentStep,omitempty"`
	RollbackCompletedSteps        []string                     `json:"rollbackCompletedSteps,omitempty"`
}

type newGamePendingMarker struct {
	SchemaVersion     int       `json:"schemaVersion"`
	TransactionID     string    `json:"transactionId"`
	RequestedFarmType string    `json:"requestedFarmType"`
	TargetSaveID      string    `json:"targetSaveId,omitempty"`
	CreatedAt         time.Time `json:"createdAt"`
	ExpiresAt         time.Time `json:"expiresAt"`
	State             string    `json:"state"`
}

type newGameTransaction struct {
	dataDir       string
	dir           string
	record        NewGameTransactionRecord
	ownerToken    string
	writeJSON     func(string, []byte, os.FileMode) error
	writeState    func(string, []byte, os.FileMode) error
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
	id, err := newGameRandomHex(16)
	if err != nil {
		return nil, fmt.Errorf("generate transaction id: %w", err)
	}
	configHash, err := newGameConfigSHA256(cfg)
	if err != nil {
		return nil, err
	}
	return beginNewGameTransactionWithIdentity(dataDir, cfg, id, "", "", configHash, "", false)
}

func beginNewGameTransactionWithIdentity(
	dataDir string,
	cfg registry.NewGameConfig,
	id string,
	requestID string,
	jobID string,
	configHash string,
	ownerToken string,
	allowExistingDir bool,
) (*newGameTransaction, error) {
	if !isValidNewGameTransactionID(id) {
		return nil, fmt.Errorf("invalid transaction id")
	}
	if err := ensureNewGameControlDir(dataDir); err != nil {
		return nil, fmt.Errorf("create control directory: %w", err)
	}
	root := newGameTransactionsDir(dataDir)
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("create transaction root: %w", err)
	}
	if err := os.Chmod(root, 0o700); err != nil {
		return nil, fmt.Errorf("secure transaction root: %w", err)
	}
	dir := filepath.Join(root, id)
	if err := os.Mkdir(dir, 0o700); err != nil && !(allowExistingDir && os.IsExist(err)) {
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
		dataDir: dataDir, dir: dir, ownerToken: ownerToken,
		writeJSON:   atomicWriteValidatedJSON,
		writeState:  atomicWriteValidatedJSON,
		restoreFile: restoreNewGameFile,
		restoreMods: restoreNewGameMods,
	}
	dataDirHash, err := newGameInstanceDataDirHash(dataDir)
	if err != nil {
		return nil, err
	}
	tx.record = NewGameTransactionRecord{
		SchemaVersion:       newGameTransactionSchemaVersion,
		TransactionID:       id,
		InstanceDataDirHash: dataDirHash,
		RequestID:           requestID,
		JobID:               jobID,
		ConfigSHA256:        configHash,
		OwnerToken:          ownerToken,
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
	tx.record.InitialGameloaderSave = newGameGameloaderSaveName(tx.record.Gameloader)
	status, err := readNewGameRuntimeStatusSnapshot(dataDir)
	if err != nil {
		return nil, fmt.Errorf("snapshot runtime status: %w", err)
	}
	tx.record.InitialRuntimeStatus = status
	tx.record.CreationWriter = chooseNewGameCreationWriter(dataDir, tx.record.ActiveSave)
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
	if err := tx.assertOwner(); err != nil {
		return err
	}
	if tx.ownerToken != "" {
		tx.record.OwnerToken = tx.ownerToken
	}
	tx.record.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(tx.record, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal transaction state: %w", err)
	}
	writer := tx.writeState
	if writer == nil {
		writer = atomicWriteValidatedJSON
	}
	return writer(filepath.Join(tx.dir, "transaction.json"), data, 0o600)
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
	if record.SchemaVersion != 1 && record.SchemaVersion != newGameTransactionSchemaVersion {
		return NewGameTransactionRecord{}, fmt.Errorf("unsupported transaction schema version %d", record.SchemaVersion)
	}
	if record.TransactionID != transactionID {
		return NewGameTransactionRecord{}, fmt.Errorf("transaction identity does not match directory")
	}
	if record.SchemaVersion >= 2 && record.RequestID != "" {
		if validateNewGameRequestID(record.RequestID) != nil || !isValidNewGameSHA256(record.InstanceDataDirHash) ||
			!isValidNewGameSHA256(record.ConfigSHA256) ||
			(record.OwnerToken != "" && !isValidNewGameSHA256(record.OwnerToken)) {
			return NewGameTransactionRecord{}, fmt.Errorf("transaction owner identity is invalid")
		}
	}
	return record, nil
}

func (tx *newGameTransaction) prepareConfigAndMarker() error {
	if err := tx.assertOwner(); err != nil {
		return err
	}
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
	if err := tx.assertOwner(); err != nil {
		return err
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
		RequestedFarmType: tx.record.RequestedFarmType, TargetSaveID: tx.record.CandidateSave, CreatedAt: now,
		ExpiresAt: now.Add(newGameMarkerTTL), State: "pending",
	}
	markerData, err := json.MarshalIndent(marker, "", "  ")
	if err != nil {
		return &NewGameTransactionError{Code: "new_game_marker_write_failed", Message: "生成 pending marker 失败", Cause: err}
	}
	if err := tx.assertOwner(); err != nil {
		return err
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
	if err := tx.assertOwner(); err != nil {
		return err
	}
	if saveName == "" || tx.record.CandidateSave != saveName || tx.record.SaveLoadedAt == nil ||
		tx.record.CustomizationVerifiedAt == nil || tx.record.DurableSaveCommandID == "" ||
		tx.record.DurableGameLoopSavedAt == nil || tx.record.DiskVerifiedAt == nil ||
		!isValidNewGameSHA256(tx.record.MainSaveSHA256) || !isValidNewGameSHA256(tx.record.SaveGameInfoSHA256) {
		return &NewGameTransactionError{Code: "new_game_durability_incomplete", Message: "四段耐久化证据不完整，禁止把新建存档事务标记成功"}
	}
	tx.record.CreatedSave = saveName
	tx.record.Result = ""
	tx.record.Stage = newGameStateFinalizing
	tx.record.ErrorCode = ""
	tx.record.ErrorMessage = ""
	if err := tx.persist(); err != nil {
		return err
	}
	if err := tx.assertOwner(); err != nil {
		return err
	}
	if err := tx.restoreFile(serverInitPath(tx.dataDir), tx.record.ServerInit); err != nil {
		return fmt.Errorf("restore server-init after completed new-game: %w", err)
	}
	if err := tx.restoreFile(farmCatalogRequestPath(tx.dataDir), tx.record.CatalogRequest); err != nil {
		return fmt.Errorf("restore runtime catalog request after completed new-game: %w", err)
	}
	// Keep the marker until every other reversible cleanup has succeeded. If a
	// cleanup write fails, manual recovery can still refresh the exact target;
	// removing it first would strand a fully durable transaction in finalizing.
	if err := os.Remove(newGamePendingPath(tx.dataDir)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove completed new-game marker: %w", err)
	}
	tx.record.Stage = newGameStateSuccess
	tx.record.Result = "success"
	return tx.persist()
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
	if err := tx.beginRollback(cause, code, stage); err != nil {
		return err
	}
	return tx.continueRollback()
}

const (
	newGameRollbackStepQuarantine     = "quarantine_new_saves"
	newGameRollbackStepServerSettings = "restore_server_settings"
	newGameRollbackStepServerInit     = "restore_server_init"
	newGameRollbackStepGameloader     = "restore_gameloader"
	newGameRollbackStepPendingMarker  = "restore_pending_marker"
	newGameRollbackStepModProfiles    = "restore_mod_profiles"
	newGameRollbackStepCatalogRequest = "restore_catalog_request"
	newGameRollbackStepRuntimeOptions = "restore_runtime_options"
	newGameRollbackStepMods           = "restore_mods"
)

type newGameRollbackFileStep struct {
	id       string
	path     string
	snapshot newGameFileSnapshot
}

// beginRollback is the write-ahead boundary for every rollback. It snapshots
// the exact set of newly-created save directories and durably records the
// rolling_back state before ComposeDown, quarantine, restore, or Mod moves may
// occur. A failed persist therefore leaves every rollback target untouched.
func (tx *newGameTransaction) beginRollback(cause error, code string, stage NewGameTransactionState) error {
	if err := tx.assertOwner(); err != nil {
		return err
	}
	if code == "" {
		code = tx.record.ErrorCode
	}
	if code == "" {
		code = "new_game_failed"
	}
	if stage == "" || stage == newGameStateRollingBack || stage == newGameStateRollbackFail {
		stage = tx.record.RollbackOriginalStage
	}
	if stage == "" {
		stage = newGameStateFailed
	}
	if tx.record.RollbackStartedAt == nil {
		now := time.Now().UTC()
		tx.record.RollbackStartedAt = &now
	}
	if tx.record.RollbackOriginalStage == "" {
		tx.record.RollbackOriginalStage = stage
	}
	if !tx.record.RollbackPlanReady {
		if code != "mod_profile_commit_failed" {
			planned, err := tx.planRollbackSaveDirs()
			if err != nil {
				return fmt.Errorf("plan new-save quarantine: %w", err)
			}
			tx.record.RollbackPlannedSaveDirs = planned
		}
		tx.record.RollbackPlanReady = true
	}
	tx.record.Stage = newGameStateRollingBack
	tx.record.Result = string(newGameStateRollingBack)
	tx.record.ErrorCode = code
	if cause != nil && tx.record.ErrorMessage == "" {
		tx.record.ErrorMessage = paneldocker.RedactString(cause.Error())
	}
	tx.record.RollbackCompleted = false
	tx.record.RollbackError = ""
	return tx.persist()
}

func (tx *newGameTransaction) continueRollback() error {
	if err := tx.assertOwner(); err != nil {
		return err
	}
	if tx.record.Stage != newGameStateRollingBack && tx.record.Stage != newGameStateRollbackFail {
		return fmt.Errorf("new-game transaction is not rollback-recoverable from stage %s", tx.record.Stage)
	}
	if tx.record.Stage == newGameStateRollbackFail {
		if err := tx.beginRollback(nil, tx.record.ErrorCode, tx.record.RollbackOriginalStage); err != nil {
			return err
		}
	}
	if tx.record.ErrorCode != "mod_profile_commit_failed" {
		if err := tx.runRollbackStep(newGameRollbackStepQuarantine, func() error {
			if tx.quarantineNew != nil {
				return tx.quarantineNew()
			}
			return tx.quarantineRollbackSaveDirs()
		}); err != nil {
			return err
		}
	}
	for _, step := range tx.rollbackFileSteps() {
		step := step
		if err := tx.runRollbackStep(step.id, func() error {
			return tx.restoreFile(step.path, step.snapshot)
		}); err != nil {
			return err
		}
	}
	if err := tx.runRollbackStep(newGameRollbackStepMods, func() error {
		return tx.restoreMods(tx.dataDir, tx.record.Mods)
	}); err != nil {
		return err
	}
	tx.record.RollbackCompleted = true
	tx.record.RollbackCurrentStep = ""
	tx.record.RollbackError = ""
	if tx.record.ErrorCode == "mod_profile_commit_failed" {
		tx.record.Stage = newGameStateProfilePending
		tx.record.Result = "recoverable"
	} else {
		tx.record.Stage = newGameStateRolledBack
		tx.record.Result = string(newGameStateRolledBack)
	}
	return tx.persist()
}

func (tx *newGameTransaction) runRollbackStep(step string, action func() error) error {
	if tx.rollbackStepCompleted(step) {
		return nil
	}
	if err := tx.prepareRollbackStep(step); err != nil {
		return err
	}
	if err := tx.assertOwner(); err != nil {
		return err
	}
	if err := action(); err != nil {
		return tx.failRollback(fmt.Errorf("%s: %w", step, err))
	}
	return tx.completeRollbackStep(step)
}

func (tx *newGameTransaction) prepareRollbackStep(step string) error {
	if tx.rollbackStepCompleted(step) {
		return nil
	}
	tx.record.Stage = newGameStateRollingBack
	tx.record.Result = string(newGameStateRollingBack)
	tx.record.RollbackCurrentStep = step
	tx.record.RollbackError = ""
	return tx.persist()
}

func (tx *newGameTransaction) completeRollbackStep(step string) error {
	if !tx.rollbackStepCompleted(step) {
		tx.record.RollbackCompletedSteps = append(tx.record.RollbackCompletedSteps, step)
		sort.Strings(tx.record.RollbackCompletedSteps)
	}
	if tx.record.RollbackCurrentStep == step {
		tx.record.RollbackCurrentStep = ""
	}
	return tx.persist()
}

func (tx *newGameTransaction) rollbackStepCompleted(step string) bool {
	for _, completed := range tx.record.RollbackCompletedSteps {
		if completed == step {
			return true
		}
	}
	return false
}

func (tx *newGameTransaction) failRollback(rollbackErr error) error {
	tx.record.Stage = newGameStateRollbackFail
	tx.record.Result = "failed"
	tx.record.RollbackCompleted = false
	tx.record.RollbackError = paneldocker.RedactString(rollbackErr.Error())
	if persistErr := tx.persist(); persistErr != nil {
		return errors.Join(rollbackErr, fmt.Errorf("persist rollback failure: %w", persistErr))
	}
	return rollbackErr
}

func (tx *newGameTransaction) rollbackFileSteps() []newGameRollbackFileStep {
	return []newGameRollbackFileStep{
		{id: newGameRollbackStepServerSettings, path: serverSettingsPath(tx.dataDir), snapshot: tx.record.ServerSettings},
		{id: newGameRollbackStepServerInit, path: serverInitPath(tx.dataDir), snapshot: tx.record.ServerInit},
		{id: newGameRollbackStepGameloader, path: gameloaderPath(tx.dataDir), snapshot: tx.record.Gameloader},
		{id: newGameRollbackStepPendingMarker, path: newGamePendingPath(tx.dataDir), snapshot: tx.record.PendingMarker},
		{id: newGameRollbackStepModProfiles, path: modProfileFilePath(tx.dataDir), snapshot: tx.record.ModProfiles},
		{id: newGameRollbackStepCatalogRequest, path: farmCatalogRequestPath(tx.dataDir), snapshot: tx.record.CatalogRequest},
		{id: newGameRollbackStepRuntimeOptions, path: runtimeOptionsPath(tx.dataDir), snapshot: tx.record.RuntimeOptions},
	}
}

func (tx *newGameTransaction) planRollbackSaveDirs() ([]string, error) {
	planned := make(map[string]struct{})
	for _, name := range tx.record.RollbackPlannedSaveDirs {
		if err := validateSaveName(name); err != nil {
			return nil, err
		}
		planned[name] = struct{}{}
	}
	for _, name := range tx.record.QuarantinedSaveDirs {
		if err := validateSaveName(name); err != nil {
			return nil, err
		}
		planned[name] = struct{}{}
	}
	before := make(map[string]struct{}, len(tx.record.PreexistingSaveDirs))
	for _, name := range tx.record.PreexistingSaveDirs {
		before[name] = struct{}{}
	}
	current, err := listSaveDirs(tx.dataDir)
	if err != nil {
		return nil, err
	}
	for _, name := range current {
		if _, existed := before[name]; !existed {
			planned[name] = struct{}{}
		}
	}
	quarantineRoot := filepath.Join(tx.dataDir, ".local-container", "saves-quarantine", "new-game", tx.record.TransactionID)
	entries, err := os.ReadDir(quarantineRoot)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if err := validateSaveName(entry.Name()); err != nil {
			return nil, err
		}
		planned[entry.Name()] = struct{}{}
	}
	result := make([]string, 0, len(planned))
	for name := range planned {
		result = append(result, name)
	}
	sort.Strings(result)
	return result, nil
}

func (tx *newGameTransaction) quarantineRollbackSaveDirs() error {
	if len(tx.record.RollbackPlannedSaveDirs) == 0 {
		return nil
	}
	root := filepath.Join(tx.dataDir, ".local-container", "saves-quarantine", "new-game", tx.record.TransactionID)
	if err := os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(root, 0o700); err != nil {
		return err
	}
	for _, name := range tx.record.RollbackPlannedSaveDirs {
		if err := tx.assertOwner(); err != nil {
			return err
		}
		if err := validateSaveName(name); err != nil {
			return err
		}
		src := filepath.Join(savesDir(tx.dataDir), "Saves", name)
		dst := filepath.Join(root, name)
		srcInfo, srcErr := os.Stat(src)
		dstInfo, dstErr := os.Stat(dst)
		srcExists := srcErr == nil
		dstExists := dstErr == nil
		if srcErr != nil && !errors.Is(srcErr, os.ErrNotExist) {
			return srcErr
		}
		if dstErr != nil && !errors.Is(dstErr, os.ErrNotExist) {
			return dstErr
		}
		if srcExists && dstExists {
			return fmt.Errorf("save %s exists in both active and quarantine roots", name)
		}
		if dstExists {
			if !dstInfo.IsDir() {
				return fmt.Errorf("quarantined save %s is not a directory", name)
			}
			continue
		}
		if !srcExists {
			return fmt.Errorf("planned save %s is missing from active and quarantine roots", name)
		}
		if !srcInfo.IsDir() {
			return fmt.Errorf("active save %s is not a directory", name)
		}
		if err := os.Rename(src, dst); err != nil {
			return err
		}
	}
	tx.record.QuarantinedSaveDirs = append([]string{}, tx.record.RollbackPlannedSaveDirs...)
	return nil
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
		if err := tx.persist(); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (tx *newGameTransaction) quarantineNewSaveDirs() error {
	if err := tx.assertOwner(); err != nil {
		return err
	}
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
		if err := tx.assertOwner(); err != nil {
			return err
		}
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
