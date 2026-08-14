package stardew_junimo

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

const (
	SaveImportJobType = "stardew_import_save_and_start"

	ImportStageValidated         = "validated"
	ImportStageStaged            = "staged"
	ImportStageBackupCreated     = "backup_created"
	ImportStageRuntimeReady      = "runtime_ready"
	ImportStageSubmitted         = "import_submitted"
	ImportStageConfirmed         = "import_confirmed"
	ImportStageSaveActivating    = "save_activating"
	ImportStageFinalizeConfirmed = "finalize_confirmed"
	ImportStageSavePersisting    = "save_persisting"
	ImportStageSaveVerified      = "save_verified"
	ImportStageCompleted         = "completed"
	ImportStageCanceled          = "canceled"

	ImportErrorResultUnconfirmed  = "import_result_unconfirmed"
	ImportErrorRecoveryRequired   = "import_recovery_required"
	ImportErrorActivationTimeout  = "import_activation_timeout"
	ImportErrorUnsupported        = "junimo_import_unsupported"
	ImportErrorSaveExists         = "save_exists"
	ImportErrorBusy               = "save_import_busy"
	ImportErrorMaintenanceStart   = "save_import_maintenance_start_failed"
	ImportErrorMaintenanceReady   = "save_import_maintenance_not_ready"
	ImportErrorPlayersConnected   = "save_import_players_connected"
	ImportErrorMaintenanceCancel  = "save_import_maintenance_canceled"
	ImportErrorMaintenanceFIFO    = "save_import_maintenance_fifo_unavailable"
	ImportErrorMaintenanceLog     = "save_import_maintenance_log_unavailable"
	ImportErrorMaintenanceAPI     = "save_import_maintenance_api_unavailable"
	ImportErrorMaintenanceMod     = "save_import_maintenance_version_mismatch"
	ImportErrorMaintenanceControl = "save_import_maintenance_control_mismatch"
	ImportErrorMaintenanceSaves   = "save_import_maintenance_saves_unavailable"
	ImportErrorMaintenanceProcess = "save_import_maintenance_process_changed"
	ImportErrorRuntimePrepare     = "save_import_runtime_prepare_failed"
	ImportErrorCommandFailed      = "import_command_failed"
	ImportErrorSaveInProgress     = "save_in_progress"
)

func SaveImportJobIdempotencyKey(operationID string) string {
	return "save-import:" + operationID
}

var importStagingMu sync.Mutex

type ImportTransactionError struct {
	Code    string
	Message string
	Cause   error
}

func (e *ImportTransactionError) Error() string { return e.Message }
func (e *ImportTransactionError) Unwrap() error { return e.Cause }

func AsImportTransactionError(err error) (*ImportTransactionError, bool) {
	var typed *ImportTransactionError
	ok := errors.As(err, &typed)
	return typed, ok
}

type ImportJournal struct {
	SchemaVersion                     int                              `json:"schemaVersion"`
	OperationID                       string                           `json:"operationId"`
	InstanceID                        string                           `json:"instanceId"`
	SaveName                          string                           `json:"saveName"`
	OriginalActiveSave                string                           `json:"originalActiveSave,omitempty"`
	BootstrapSaveName                 string                           `json:"bootstrapSaveName,omitempty"`
	BootstrapSaveFingerprint          string                           `json:"bootstrapSaveFingerprint,omitempty"`
	BootstrapSaveCreated              bool                             `json:"bootstrapSaveCreated,omitempty"`
	BootstrapCleanupCompleted         bool                             `json:"bootstrapCleanupCompleted,omitempty"`
	HostHandling                      string                           `json:"hostHandling"`
	PlatformIDFingerprint             string                           `json:"platformIdFingerprint,omitempty"`
	SourceOwned                       bool                             `json:"sourceOwned"`
	Stage                             string                           `json:"stage"`
	StagedSaveCreated                 bool                             `json:"stagedSaveCreated"`
	StagedSaveFingerprint             string                           `json:"stagedSaveFingerprint,omitempty"`
	PreimportBackupName               string                           `json:"preimportBackupName,omitempty"`
	PreimportBackupSHA256             string                           `json:"preimportBackupSha256,omitempty"`
	MaintenanceStarted                bool                             `json:"maintenanceStarted"`
	OriginalInstanceState             string                           `json:"originalInstanceState,omitempty"`
	OriginalInstanceStateMessage      string                           `json:"originalInstanceStateMessage,omitempty"`
	OriginalInstanceStateMessageValid bool                             `json:"originalInstanceStateMessageValid"`
	OriginalInstanceDriverPhase       string                           `json:"originalInstanceDriverPhase,omitempty"`
	OriginalInstanceDriverPayload     string                           `json:"originalInstanceDriverPayload,omitempty"`
	RuntimeBaseline                   *JunimoImportEvidenceSnapshot    `json:"runtimeBaseline,omitempty"`
	ServerOutputLogOffset             *int64                           `json:"serverOutputLogOffset,omitempty"`
	PreSubmitEvidence                 *JunimoImportEvidenceSnapshot    `json:"preSubmitEvidence,omitempty"`
	PreSubmitLogOffset                *int64                           `json:"preSubmitLogOffset,omitempty"`
	PhaseAEvidence                    *JunimoImportEvidenceSnapshot    `json:"phaseAEvidence,omitempty"`
	PhaseAOutcome                     string                           `json:"phaseAOutcome,omitempty"`
	PhaseARestoredSHA256              string                           `json:"phaseARestoredSha256,omitempty"`
	PhaseALogDetail                   string                           `json:"phaseALogDetail,omitempty"`
	ActivationEvidence                *JunimoImportActivationEvidence  `json:"activationEvidence,omitempty"`
	ActivationOutcome                 string                           `json:"activationOutcome,omitempty"`
	ActivationRestarted               bool                             `json:"activationRestarted"`
	ActivationProcessBaseline         *JunimoActivationProcessBaseline `json:"activationProcessBaseline,omitempty"`
	DurableSaveCommandID              string                           `json:"durableSaveCommandId,omitempty"`
	DurableSaveSubmittedAt            *time.Time                       `json:"durableSaveSubmittedAt,omitempty"`
	DurableSaveSubmissionFailed       bool                             `json:"durableSaveSubmissionFailed"`
	DurableSaveBefore                 *JunimoImportSaveDiskState       `json:"durableSaveBefore,omitempty"`
	DurableSaveAfter                  *JunimoImportSaveDiskState       `json:"durableSaveAfter,omitempty"`
	DurableStatusBaselineVersion      *int64                           `json:"durableStatusBaselineVersion,omitempty"`
	DurableStatusAfterSavedVersion    *int64                           `json:"durableStatusAfterSavedVersion,omitempty"`
	DurableGameLoopSaved              bool                             `json:"durableGameLoopSaved"`
	DurableTransitionComplete         *bool                            `json:"durableTransitionComplete,omitempty"`
	DurableSaveWarning                string                           `json:"durableSaveWarning,omitempty"`
	FarmhandUnbindVerified            bool                             `json:"farmhandUnbindVerified"`
	FarmhandCount                     int                              `json:"farmhandCount,omitempty"`
	CustomizedFarmhandCount           int                              `json:"customizedFarmhandCount,omitempty"`
	UpstreamSubmittedAt               *time.Time                       `json:"upstreamSubmittedAt,omitempty"`
	UpstreamSubmitted                 bool                             `json:"upstreamSubmitted"`
	UpstreamConfirmed                 bool                             `json:"upstreamConfirmed"`
	LastErrorCode                     string                           `json:"lastErrorCode,omitempty"`
	LastError                         string                           `json:"lastError,omitempty"`
	RecoveryState                     string                           `json:"recoveryState,omitempty"`
	CreatedAt                         time.Time                        `json:"createdAt"`
	UpdatedAt                         time.Time                        `json:"updatedAt"`
}

type ImportRecovery struct {
	OperationID     string `json:"operationId"`
	Stage           string `json:"stage"`
	State           string `json:"state"`
	ErrorCode       string `json:"errorCode,omitempty"`
	SourceAvailable bool   `json:"sourceAvailable"`
}

func NewImportOperationID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func importTransactionsDir(dataDir string) string {
	return filepath.Join(controlDir(dataDir), "save-import-transactions")
}

func importJournalPath(dataDir, operationID string) string {
	return filepath.Join(importTransactionsDir(dataDir), operationID, "journal.json")
}

func validImportOperationID(id string) bool {
	if len(id) != 32 {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil
}

func platformFingerprint(operationID, platformID string) string {
	if strings.TrimSpace(platformID) == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(operationID + "\x00" + platformID))
	return hex.EncodeToString(sum[:])
}

// ImportJournalMatchesRequest verifies an idempotent retry without exposing or
// persisting the raw platform ID.
func ImportJournalMatchesRequest(j ImportJournal, req registry.SaveImportRequest) bool {
	return j.OperationID == req.OperationID &&
		j.InstanceID == req.Instance.ID &&
		j.SaveName == req.SaveName &&
		j.HostHandling == req.HostHandling &&
		j.PlatformIDFingerprint == platformFingerprint(req.OperationID, req.PlatformID)
}

func CreateImportJournal(dataDir string, req registry.SaveImportRequest) (ImportJournal, error) {
	if !validImportOperationID(req.OperationID) {
		return ImportJournal{}, &ImportTransactionError{Code: "invalid_operation_id", Message: "operationId is invalid"}
	}
	if err := validateSaveName(req.SaveName); err != nil {
		return ImportJournal{}, &ImportTransactionError{Code: "invalid_save", Message: "save name is invalid", Cause: err}
	}
	if existing, err := LoadImportJournal(dataDir, req.OperationID); err == nil {
		if ImportJournalMatchesRequest(existing, req) {
			return existing, nil
		}
		return ImportJournal{}, &ImportTransactionError{Code: "operation_conflict", Message: "operationId belongs to another import"}
	} else if !os.IsNotExist(err) {
		return ImportJournal{}, err
	}
	_, saveErr := os.Stat(filepath.Join(savesDir(dataDir), "Saves", req.SaveName))
	if saveErr == nil {
		return ImportJournal{}, &ImportTransactionError{Code: ImportErrorSaveExists, Message: "a save with this name already exists"}
	}
	if !os.IsNotExist(saveErr) {
		return ImportJournal{}, fmt.Errorf("check existing save: %w", saveErr)
	}
	now := time.Now().UTC()
	j := ImportJournal{SchemaVersion: 1, OperationID: req.OperationID, InstanceID: req.Instance.ID,
		SaveName: req.SaveName, OriginalActiveSave: GetActiveSaveName(dataDir), HostHandling: req.HostHandling,
		PlatformIDFingerprint: platformFingerprint(req.OperationID, req.PlatformID), Stage: ImportStageValidated,
		CreatedAt: now, UpdatedAt: now}
	if err := WriteImportJournal(dataDir, j); err != nil {
		return ImportJournal{}, err
	}
	return j, nil
}

func WriteImportJournal(dataDir string, j ImportJournal) error {
	if !validImportOperationID(j.OperationID) {
		return fmt.Errorf("invalid operation id")
	}
	if err := validateImportJournal(j, j.OperationID); err != nil {
		return err
	}
	dir := filepath.Dir(importJournalPath(dataDir, j.OperationID))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(importTransactionsDir(dataDir), 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	j.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteValidatedJSON(importJournalPath(dataDir, j.OperationID), data, 0o600)
}

func LoadImportJournal(dataDir, operationID string) (ImportJournal, error) {
	if !validImportOperationID(operationID) {
		return ImportJournal{}, fmt.Errorf("invalid operation id")
	}
	data, err := os.ReadFile(importJournalPath(dataDir, operationID))
	if err != nil {
		return ImportJournal{}, err
	}
	var j ImportJournal
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&j); err != nil {
		return ImportJournal{}, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("multiple JSON documents")
		}
		return ImportJournal{}, fmt.Errorf("invalid import journal JSON: %w", err)
	}
	if err := validateImportJournal(j, operationID); err != nil {
		return ImportJournal{}, err
	}
	return j, nil
}

func validateImportJournal(j ImportJournal, operationID string) error {
	if j.SchemaVersion != 1 {
		return fmt.Errorf("unsupported import journal schema version %d", j.SchemaVersion)
	}
	if j.OperationID != operationID || !validImportOperationID(j.OperationID) {
		return fmt.Errorf("import journal operation identity mismatch")
	}
	if strings.TrimSpace(j.InstanceID) == "" {
		return fmt.Errorf("import journal instance identity is missing")
	}
	if err := validateSaveName(j.SaveName); err != nil {
		return fmt.Errorf("import journal save name is invalid: %w", err)
	}
	if !isKnownImportStage(j.Stage) {
		return fmt.Errorf("unsupported import journal stage %q", j.Stage)
	}
	return nil
}

func isKnownImportStage(stage string) bool {
	switch stage {
	case ImportStageValidated, ImportStageStaged, ImportStageBackupCreated,
		ImportStageRuntimeReady, ImportStageSubmitted, ImportStageConfirmed,
		ImportStageSaveActivating, ImportStageFinalizeConfirmed,
		ImportStageSavePersisting, ImportStageSaveVerified,
		ImportStageCompleted, ImportStageCanceled:
		return true
	default:
		return false
	}
}

func RecoverImportTransactions(dataDir string) ([]ImportRecovery, error) {
	entries, err := os.ReadDir(importTransactionsDir(dataDir))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var result []ImportRecovery
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if !validImportOperationID(entry.Name()) {
			return nil, &ImportTransactionError{Code: ImportErrorRecoveryRequired, Message: "unknown save import transaction directory requires recovery"}
		}
		j, err := LoadImportJournal(dataDir, entry.Name())
		if err != nil {
			return nil, err
		}
		if j.Stage == ImportStageCompleted || j.Stage == ImportStageCanceled {
			continue
		}
		_, sourceErr := os.Stat(importTransactionSourceDir(dataDir, j.OperationID))
		r := ImportRecovery{OperationID: j.OperationID, Stage: j.Stage, SourceAvailable: sourceErr == nil}
		if j.DurableSaveSubmissionFailed {
			r.State, r.ErrorCode = "manual_required", ImportErrorRecoveryRequired
		} else if j.Stage == ImportStageFinalizeConfirmed || j.Stage == ImportStageSavePersisting || j.Stage == ImportStageSaveVerified {
			r.State = "resume_save_verification"
		} else if j.UpstreamConfirmed && (j.Stage == ImportStageConfirmed || j.Stage == ImportStageSaveActivating) {
			// Re-observe the already submitted transaction. This path never calls
			// Phase A and therefore cannot publish another saves import command.
			r.State = "resume_activation_verification"
		} else if j.PhaseAOutcome == phaseAOutcomeNoEffect && !j.MaintenanceStarted && !j.UpstreamConfirmed {
			r.State = "safe_to_resume_or_cleanup"
		} else if j.UpstreamConfirmed || importStageAtLeast(j.Stage, ImportStageConfirmed) {
			r.State, r.ErrorCode = "manual_required", ImportErrorRecoveryRequired
		} else if j.UpstreamSubmitted || importStageAtLeast(j.Stage, ImportStageSubmitted) {
			r.State, r.ErrorCode = "manual_required", ImportErrorResultUnconfirmed
		} else {
			r.State = "safe_to_resume_or_cleanup"
		}
		j.RecoveryState, j.LastErrorCode = r.State, r.ErrorCode
		if err := WriteImportJournal(dataDir, j); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, nil
}

func (d *Driver) resumeRecoveredImportDurableSaves(ctx context.Context, instance registry.Instance, recoveries []ImportRecovery) error {
	if d.jobs == nil {
		return nil
	}
	active, err := d.jobs.Active(ctx, storage.ListActiveJobsFilter{TargetType: "instance", TargetID: instance.ID, Types: []string{SaveImportJobType}})
	if err != nil {
		return err
	}
	if len(active) > 0 {
		return nil
	}
	for _, recovery := range recoveries {
		if recovery.State != "resume_save_verification" && recovery.State != "resume_activation_verification" {
			continue
		}
		operationID := recovery.OperationID
		resume := "durable_save"
		if recovery.State == "resume_activation_verification" {
			resume = "activation_verification"
		}
		payload, _ := json.Marshal(map[string]string{"operationId": operationID, "resume": resume})
		_, err := d.jobs.Start(ctx, jobs.Spec{
			Type: SaveImportJobType, DisplayName: "Resume imported save verification", TargetType: "instance",
			TargetID: instance.ID, Payload: string(payload), Timeout: 10 * time.Minute,
			Run: func(runCtx context.Context, job *jobs.Context) error {
				d.saveImportRunMu.Lock()
				defer d.saveImportRunMu.Unlock()
				if recovery.State == "resume_activation_verification" {
					if err := d.runImportActivation(runCtx, instance, operationID, job, defaultImportActivationOptions()); err != nil {
						return err
					}
				}
				return d.runImportDurableSave(runCtx, instance, operationID, job, defaultImportDurableSaveOptions())
			},
		})
		if err != nil {
			return err
		}
		return nil // global import invariant permits at most one unfinished operation
	}
	return nil
}

func HasUnfinishedImportTransaction(dataDir string) (bool, error) {
	return HasUnfinishedImportTransactionOtherThan(dataDir, "")
}

// HasUnfinishedImportTransactionOtherThan allows an idempotent retry to
// resume its own durable operation while still rejecting every other owner.
func HasUnfinishedImportTransactionOtherThan(dataDir, allowedOperationID string) (bool, error) {
	entries, err := os.ReadDir(importTransactionsDir(dataDir))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if !validImportOperationID(entry.Name()) {
			return false, &ImportTransactionError{Code: ImportErrorRecoveryRequired, Message: "unknown save import transaction directory requires recovery"}
		}
		j, err := LoadImportJournal(dataDir, entry.Name())
		if err != nil {
			return false, err
		}
		if j.OperationID == allowedOperationID {
			continue
		}
		if j.Stage != ImportStageCompleted && j.Stage != ImportStageCanceled {
			return true, nil
		}
	}
	return false, nil
}

// CleanupUnsubmittedImport removes the operation source and a staged target
// only when the journal proves this operation created it and its full-tree
// fingerprint is unchanged. The preimport ZIP is retained as the explicit
// recovery policy. Once submission may have reached Junimo, cleanup is forbidden.
func CleanupUnsubmittedImport(dataDir, operationID string) error {
	importStagingMu.Lock()
	defer importStagingMu.Unlock()
	j, err := LoadImportJournal(dataDir, operationID)
	if err != nil {
		return err
	}
	if j.Stage == ImportStageCanceled {
		return nil
	}
	if j.MaintenanceStarted {
		return &ImportTransactionError{Code: ImportErrorRecoveryRequired, Message: "save import maintenance may still be running; automatic cleanup is unsafe"}
	}
	safelyProvenNoEffect := j.PhaseAOutcome == phaseAOutcomeNoEffect && !j.MaintenanceStarted && !j.UpstreamConfirmed
	if !safelyProvenNoEffect && (j.UpstreamSubmitted || j.UpstreamConfirmed || importStageAtLeast(j.Stage, ImportStageSubmitted)) {
		return &ImportTransactionError{Code: ImportErrorRecoveryRequired, Message: "submitted import requires manual recovery"}
	}
	bootstrapPlan, err := planUnsubmittedImportBootstrapCleanup(dataDir, j)
	if err != nil {
		return err
	}
	stagedTarget := ""
	if j.StagedSaveCreated {
		stagedTarget = filepath.Join(savesDir(dataDir), "Saves", j.SaveName)
		if _, statErr := os.Stat(stagedTarget); statErr == nil {
			fingerprint, fingerprintErr := importDirectoryFingerprint(stagedTarget)
			if fingerprintErr != nil || fingerprint != j.StagedSaveFingerprint {
				return &ImportTransactionError{Code: ImportErrorRecoveryRequired, Message: "staged save changed; automatic cleanup is unsafe", Cause: fingerprintErr}
			}
		} else if !os.IsNotExist(statErr) {
			return fmt.Errorf("check staged import target: %w", statErr)
		}
	}
	// No mutation is allowed before every ownership, pointer and fingerprint
	// check above has succeeded. This prevents a later staged-target mismatch
	// from leaving a partially deleted bootstrap transaction.
	if err := executeUnsubmittedImportBootstrapCleanup(dataDir, bootstrapPlan); err != nil {
		return err
	}
	if stagedTarget != "" {
		if err := os.RemoveAll(stagedTarget); err != nil {
			return fmt.Errorf("remove staged import target: %w", err)
		}
	}
	if err := os.RemoveAll(importTransactionSourceDir(dataDir, operationID)); err != nil {
		return fmt.Errorf("remove import transaction source: %w", err)
	}
	j.Stage = ImportStageCanceled
	j.SourceOwned = false
	j.StagedSaveCreated = false
	j.StagedSaveFingerprint = ""
	j.BootstrapSaveCreated = false
	j.BootstrapSaveFingerprint = ""
	j.BootstrapCleanupCompleted = true
	j.LastErrorCode, j.LastError = "", ""
	return WriteImportJournal(dataDir, j)
}

// FinalizeCanceledImportCleanup removes only a transaction which has already
// reached the durable canceled marker. The marker makes cleanup retryable if
// the process stops between filesystem cleanup and token deletion.
func FinalizeCanceledImportCleanup(dataDir, operationID string) error {
	importStagingMu.Lock()
	defer importStagingMu.Unlock()
	j, err := LoadImportJournal(dataDir, operationID)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if j.Stage != ImportStageCanceled || j.MaintenanceStarted || j.UpstreamSubmitted || j.UpstreamConfirmed {
		return &ImportTransactionError{Code: ImportErrorRecoveryRequired, Message: "save import cleanup is not durably canceled"}
	}
	return os.RemoveAll(filepath.Dir(importJournalPath(dataDir, operationID)))
}

// CleanupUnsubmittedSaveImport serializes the final runtime preflight and
// filesystem cleanup with lifecycle mutations. The Web layer must not split
// the Compose stopped check from ownership/fingerprint guarded deletion.
func (d *Driver) CleanupUnsubmittedSaveImport(ctx context.Context, instance registry.Instance, operationID string) error {
	d.runtimeUpdateMu.Lock()
	defer d.runtimeUpdateMu.Unlock()
	original, err := d.inspectSaveImportMaintenanceOffline(ctx, instance)
	if err != nil {
		return err
	}
	return CleanupUnsubmittedImport(original.DataDir, operationID)
}

func importStageAtLeast(stage, threshold string) bool {
	order := []string{ImportStageValidated, ImportStageStaged, ImportStageBackupCreated, ImportStageRuntimeReady, ImportStageSubmitted, ImportStageConfirmed, ImportStageSaveActivating, ImportStageFinalizeConfirmed, ImportStageSavePersisting, ImportStageSaveVerified, ImportStageCompleted}
	index := func(value string) int {
		for i, v := range order {
			if v == value {
				return i
			}
		}
		return -1
	}
	return index(stage) >= index(threshold) && index(threshold) >= 0
}

func ValidateImportCapability(dataDir string, fifoAvailable bool) error {
	if err := validateImportStaticCapability(dataDir); err != nil {
		return err
	}
	if !fifoAvailable {
		return &ImportTransactionError{Code: ImportErrorUnsupported, Message: "Junimo command channel is unavailable"}
	}
	return nil
}

func validateImportStaticCapability(dataDir string) error {
	values, err := sjconfig.ReadEnvFile(filepath.Join(dataDir, ".env"))
	if err != nil {
		return &ImportTransactionError{Code: ImportErrorUnsupported, Message: "Junimo import capability cannot be verified", Cause: err}
	}
	imageVersion := strings.TrimSpace(values["IMAGE_VERSION"])
	if imageVersion != TestedImageTag {
		return &ImportTransactionError{Code: ImportErrorUnsupported, Message: "Junimo .125 runtime is required for transactional import"}
	}
	if err := validateExtractedJunimoServerMod(junimoServerModDir(dataDir), imageVersion); err != nil {
		return &ImportTransactionError{Code: ImportErrorUnsupported, Message: "mounted JunimoServer mod does not match the image", Cause: err}
	}
	return nil
}

func (d *Driver) rejectActiveSaveImport(ctx context.Context, instanceID, allowedOperationID string) error {
	if d.jobs == nil {
		return nil
	}
	active, err := d.jobs.Active(ctx, storage.ListActiveJobsFilter{TargetType: "instance", TargetID: instanceID, Types: []string{SaveImportJobType}})
	if err != nil {
		return err
	}
	if len(active) > 0 {
		return &ImportTransactionError{Code: ImportErrorBusy, Message: "a save import transaction is active"}
	}
	if d.store != nil {
		instance, loadErr := d.store.GetInstance(ctx, instanceID)
		if loadErr == nil {
			unfinished, checkErr := HasUnfinishedImportTransactionOtherThan(instance.DataDir, allowedOperationID)
			if checkErr != nil {
				return checkErr
			}
			if unfinished {
				return &ImportTransactionError{Code: ImportErrorBusy, Message: "an unfinished save import transaction requires recovery"}
			}
		}
	}
	return nil
}

func (d *Driver) ImportSaveAndStart(ctx context.Context, req registry.SaveImportRequest) (*registry.Job, error) {
	if d.jobs == nil {
		return nil, fmt.Errorf("driver: job manager not configured")
	}
	d.runtimeUpdateMu.Lock()
	defer d.runtimeUpdateMu.Unlock()
	if err := d.rejectActiveRuntimeUpdate(ctx, req.Instance.ID); err != nil {
		return nil, err
	}
	if d.store == nil {
		return nil, &ImportTransactionError{Code: ImportErrorMaintenanceStart, Message: "instance state store is unavailable before save import submission"}
	}
	loaded, err := d.store.GetInstance(ctx, req.Instance.ID)
	if err != nil {
		return nil, &ImportTransactionError{Code: ImportErrorMaintenanceStart, Message: "failed to load authoritative instance before save import submission", Cause: err}
	}
	if filepath.Clean(loaded.DataDir) != filepath.Clean(req.Instance.DataDir) {
		return nil, &ImportTransactionError{Code: ImportErrorRecoveryRequired, Message: "instance data directory changed during save import submission"}
	}
	if err := rejectUnfinishedNewGameOwner(loaded.DataDir); err != nil {
		return nil, err
	}
	// This is the pre-ownership authoritative gate. It reloads the persisted
	// state and checks Compose before a journal is created or the upload is
	// transferred into transaction ownership.
	authoritative, err := d.inspectSaveImportMaintenanceOffline(ctx, req.Instance)
	if err != nil {
		return nil, err
	}
	if filepath.Clean(authoritative.DataDir) != filepath.Clean(loaded.DataDir) {
		return nil, &ImportTransactionError{Code: ImportErrorRecoveryRequired, Message: "instance data directory changed during save import preflight"}
	}
	req.Instance = registry.Instance{
		ID: authoritative.ID, DriverID: authoritative.DriverID, Name: authoritative.Name,
		DataDir: authoritative.DataDir, State: authoritative.State,
		StateMessage: authoritative.StateMessage.String, DriverPhase: authoritative.DriverPhase,
		DriverPayload: authoritative.DriverPayload, CreatedAt: authoritative.CreatedAt, UpdatedAt: authoritative.UpdatedAt,
	}
	if err := d.rejectActiveSaveImport(ctx, req.Instance.ID, req.OperationID); err != nil {
		return nil, err
	}
	// A freshly installed instance has not necessarily started once, so its
	// host-mounted JunimoServer Mod may not have been extracted from the image
	// yet. Prepare that static runtime asset before taking ownership of the
	// upload. This does not start or exec the long-running game service.
	if err := d.prepareImportRuntimeAssets(ctx, req.Instance); err != nil {
		return nil, err
	}
	j, err := CreateImportJournal(req.Instance.DataDir, req)
	if err != nil {
		return nil, err
	}
	if j.Stage == ImportStageCanceled {
		return nil, &ImportTransactionError{Code: ImportErrorBusy, Message: "save import operation was already canceled"}
	}
	sourceDir := importTransactionSourceDir(req.Instance.DataDir, req.OperationID)
	if !j.SourceOwned {
		if req.TransferSourceOwnership != nil {
			err = req.TransferSourceOwnership(sourceDir)
		} else {
			err = moveImportSource(req.StagedDir, sourceDir)
		}
		if err != nil {
			j.LastErrorCode, j.LastError = "source_ownership_failed", "failed to transfer upload into transaction source"
			_ = WriteImportJournal(req.Instance.DataDir, j)
			return nil, &ImportTransactionError{Code: "source_ownership_failed", Message: j.LastError, Cause: err}
		}
		j.SourceOwned = true
		j.LastErrorCode, j.LastError = "", ""
		if err := WriteImportJournal(req.Instance.DataDir, j); err != nil {
			return nil, err
		}
	}
	payload, _ := json.Marshal(map[string]string{"operationId": req.OperationID})
	job, err := d.jobs.Start(ctx, jobs.Spec{Type: SaveImportJobType, DisplayName: "Import save and start", TargetType: "instance", TargetID: req.Instance.ID, IdempotencyKey: SaveImportJobIdempotencyKey(req.OperationID), CreatedBy: req.ActorID, Payload: string(payload), Timeout: 30 * time.Minute,
		Run: func(runCtx context.Context, job *jobs.Context) error {
			d.saveImportRunMu.Lock()
			defer d.saveImportRunMu.Unlock()
			_, _ = job.Info(context.Background(), "Import transaction journal created.")
			if err := prepareImportStaging(req.Instance.DataDir, req.OperationID); err != nil {
				return err
			}
			_, _ = job.Info(context.Background(), "Staging and preimport backup completed; starting save_import_maintenance runtime.")
			if err := d.runImportMaintenance(runCtx, req.Instance, req.OperationID, job, defaultImportMaintenanceOptions()); err != nil {
				return err
			}
			if err := d.runImportPhaseA(runCtx, req.Instance, req.OperationID, req.PlatformID, job, defaultImportPhaseAOptions()); err != nil {
				return err
			}
			if err := d.runImportActivation(runCtx, req.Instance, req.OperationID, job, defaultImportActivationOptions()); err != nil {
				return err
			}
			if err := d.runImportDurableSave(runCtx, req.Instance, req.OperationID, job, defaultImportDurableSaveOptions()); err != nil {
				return err
			}
			if req.MarkUploadSucceeded != nil {
				if err := req.MarkUploadSucceeded(); err != nil {
					_, _ = job.Warn(context.Background(), "Save import completed, but upload token metadata cleanup was deferred.")
				}
			}
			return nil
		},
	})
	if err != nil {
		var existing *storage.IdempotentJobExistsError
		if errors.As(err, &existing) {
			return &registry.Job{ID: existing.Job.ID}, nil
		}
		j.LastErrorCode = ImportErrorSaveInProgress
		j.LastError = "import job creation failed after upload ownership transfer"
		_ = WriteImportJournal(req.Instance.DataDir, j)
		return nil, &ImportTransactionError{Code: ImportErrorSaveInProgress, Message: "import transaction owns the upload but its job could not be created", Cause: err}
	}
	return &registry.Job{ID: job.ID}, nil
}

func prepareImportStaging(dataDir, operationID string) error {
	importStagingMu.Lock()
	defer importStagingMu.Unlock()
	j, err := LoadImportJournal(dataDir, operationID)
	if err != nil {
		return err
	}
	if !j.SourceOwned {
		return &ImportTransactionError{Code: "source_not_owned", Message: "transaction source ownership is incomplete"}
	}
	if !importStageAtLeast(j.Stage, ImportStageStaged) {
		fingerprint, stageErr := StageImportedSaveNoReplace(dataDir, importTransactionSourceDir(dataDir, operationID), j.SaveName)
		if stageErr != nil {
			j.LastErrorCode, j.LastError = "save_stage_failed", "failed to stage uploaded save"
			if typed, ok := AsImportTransactionError(stageErr); ok {
				j.LastErrorCode = typed.Code
			}
			_ = WriteImportJournal(dataDir, j)
			return stageErr
		}
		j.StagedSaveCreated = true
		j.StagedSaveFingerprint = fingerprint
		j.Stage = ImportStageStaged
		j.LastErrorCode, j.LastError = "", ""
		if err := WriteImportJournal(dataDir, j); err != nil {
			return err
		}
	}
	if !importStageAtLeast(j.Stage, ImportStageBackupCreated) {
		backupPath, backupSHA, backupErr := BackupPreImport(dataDir, j.SaveName, operationID)
		if backupErr != nil {
			j.LastErrorCode, j.LastError = "preimport_backup_failed", "preimport backup of uploaded target failed"
			_ = WriteImportJournal(dataDir, j)
			return backupErr
		}
		j.PreimportBackupName = filepath.Base(backupPath)
		j.PreimportBackupSHA256 = backupSHA
		j.Stage = ImportStageBackupCreated
		j.LastErrorCode, j.LastError = "", ""
		if err := WriteImportJournal(dataDir, j); err != nil {
			return err
		}
	}
	if j.OriginalActiveSave == "" {
		if err := prepareImportBootstrap(dataDir, operationID); err != nil {
			return err
		}
	}
	return nil
}

func (d *Driver) prepareImportRuntimeAssets(ctx context.Context, instance registry.Instance) error {
	values, err := sjconfig.ReadEnvFile(filepath.Join(instance.DataDir, ".env"))
	if err != nil {
		return &ImportTransactionError{Code: ImportErrorRuntimePrepare, Message: "Junimo import runtime configuration cannot be read", Cause: err}
	}
	imageVersion := strings.TrimSpace(values["IMAGE_VERSION"])
	if imageVersion == "" {
		return &ImportTransactionError{Code: ImportErrorRuntimePrepare, Message: "Junimo import runtime version is not configured"}
	}
	if imageVersion != TestedImageTag {
		return &ImportTransactionError{Code: ImportErrorUnsupported, Message: "Junimo .125 runtime is required for transactional import"}
	}
	if err := validateExtractedJunimoServerMod(junimoServerModDir(instance.DataDir), TestedImageTag); err == nil {
		return nil
	}
	lifecycle, ok := d.docker.(LifecycleDockerService)
	if !ok {
		return &ImportTransactionError{Code: ImportErrorRuntimePrepare, Message: "Junimo import runtime assets cannot be prepared", Cause: errors.New("docker lifecycle operations are unsupported")}
	}
	runner := &lifecycleRunner{
		driver: d, lifecycle: lifecycle,
		instance: storage.Instance{ID: instance.ID, DriverID: instance.DriverID, DataDir: instance.DataDir, State: instance.State},
	}
	if err := runner.ensureJunimoServerMod(ctx, nil); err != nil {
		return &ImportTransactionError{Code: ImportErrorRuntimePrepare, Message: "Junimo import runtime assets could not be synchronized", Cause: err}
	}
	if err := validateImportStaticCapability(instance.DataDir); err != nil {
		return &ImportTransactionError{Code: ImportErrorRuntimePrepare, Message: "Junimo import runtime assets failed verification", Cause: err}
	}
	return nil
}
