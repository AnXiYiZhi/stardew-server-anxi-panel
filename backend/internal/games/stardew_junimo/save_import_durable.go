package stardew_junimo

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

const (
	durableSaveWarningTransitionTimeout = "GameLoop.Saved confirmed disk persistence, but dayTransitionComplete did not become true before the deadline"
	durableSaveWarningTransitionMissing = "GameLoop.Saved confirmed disk persistence, but dayTransitionComplete is unavailable; no fallback was assumed"
)

type JunimoImportSaveDiskState struct {
	SHA256     string    `json:"sha256"`
	Size       int64     `json:"size"`
	ModifiedAt time.Time `json:"modifiedAt"`
	CapturedAt time.Time `json:"capturedAt"`
}

type importDurableSaveOptions struct {
	CommandTimeout    time.Duration
	TransitionTimeout time.Duration
	PollInterval      time.Duration
	SubmitCommand     func(dataDir, commandID string) error
	GetOutcome        func(dataDir, commandID string) (CommandOutcome, error)
	WaitTransition    func(context.Context, commandExecutor, string, int64, time.Duration, time.Duration) error
}

func defaultImportDurableSaveOptions() importDurableSaveOptions {
	return importDurableSaveOptions{
		CommandTimeout: 130 * time.Second, TransitionTimeout: 90 * time.Second, PollInterval: 500 * time.Millisecond,
	}
}

func stableImportSaveDiskState(dataDir, saveName string) (JunimoImportSaveDiskState, error) {
	path := filepath.Join(savesDir(dataDir), "Saves", saveName, saveName)
	before, err := os.Stat(path)
	if err != nil {
		return JunimoImportSaveDiskState{}, err
	}
	hash, err := stableFileSHA256(path)
	if err != nil {
		return JunimoImportSaveDiskState{}, err
	}
	after, err := os.Stat(path)
	if err != nil {
		return JunimoImportSaveDiskState{}, err
	}
	if !os.SameFile(before, after) || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		return JunimoImportSaveDiskState{}, &ImportEvidenceError{Code: "evidence_file_changed", Message: "save evidence file changed while being read"}
	}
	if err := validateImportMainSaveXML(path); err != nil {
		return JunimoImportSaveDiskState{}, err
	}
	final, err := os.Stat(path)
	if err != nil {
		return JunimoImportSaveDiskState{}, err
	}
	if !os.SameFile(after, final) || after.Size() != final.Size() || !after.ModTime().Equal(final.ModTime()) {
		return JunimoImportSaveDiskState{}, &ImportEvidenceError{Code: "evidence_file_changed", Message: "save evidence file changed while XML was being parsed"}
	}
	return JunimoImportSaveDiskState{SHA256: hash, Size: final.Size(), ModifiedAt: final.ModTime().UTC(), CapturedAt: time.Now().UTC()}, nil
}

func waitStableImportSaveDiskState(ctx context.Context, dataDir, saveName string, timeout time.Duration) (JunimoImportSaveDiskState, error) {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	var lastErr error
	for {
		state, err := stableImportSaveDiskState(dataDir, saveName)
		if err == nil {
			return state, nil
		}
		lastErr = err
		var pathErr *os.PathError
		var evidenceErr *ImportEvidenceError
		retryable := errors.As(err, &pathErr) || (errors.As(err, &evidenceErr) && evidenceErr.Code == "evidence_file_changed")
		if !retryable {
			return JunimoImportSaveDiskState{}, err
		}
		select {
		case <-ctx.Done():
			return JunimoImportSaveDiskState{}, errors.Join(lastErr, ctx.Err())
		case <-deadline.C:
			return JunimoImportSaveDiskState{}, lastErr
		case <-ticker.C:
		}
	}
}

func validateImportMainSaveXML(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	decoder := xml.NewDecoder(f)
	rootSeen := false
	rootClosed := false
	depth := 0
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("parse target save XML: %w", err)
		}
		switch value := token.(type) {
		case xml.StartElement:
			if depth == 0 {
				if rootSeen {
					return errors.New("target save XML contains multiple root elements")
				}
				rootSeen = true
				if value.Name.Local != "SaveGame" {
					return fmt.Errorf("target save XML root is %q, expected SaveGame", value.Name.Local)
				}
			}
			depth++
		case xml.EndElement:
			depth--
			if depth < 0 {
				return errors.New("target save XML has an unmatched closing element")
			}
			if depth == 0 {
				rootClosed = true
			}
		case xml.CharData:
			text := string(value)
			if depth == 0 && !rootSeen {
				text = strings.TrimPrefix(text, "\uFEFF")
			}
			if depth == 0 && strings.TrimSpace(text) != "" {
				return errors.New("target save XML contains data outside its root element")
			}
		}
	}
	if !rootSeen || !rootClosed || depth != 0 {
		return errors.New("target save XML is incomplete")
	}
	return nil
}

func importSaveDiskChanged(before, after JunimoImportSaveDiskState) bool {
	return before.SHA256 != after.SHA256 || !before.ModifiedAt.Equal(after.ModifiedAt)
}

func (d *Driver) runImportDurableSave(ctx context.Context, instance registry.Instance, operationID string, job *jobs.Context, options importDurableSaveOptions) error {
	lifecycle, ok := d.docker.(LifecycleDockerService)
	if !ok {
		return recordImportDurableFailure(instance.DataDir, operationID, ImportErrorResultUnconfirmed, "durable save runtime is unavailable", "", nil)
	}
	if options.CommandTimeout <= 0 {
		options.CommandTimeout = 130 * time.Second
	}
	if options.TransitionTimeout <= 0 {
		options.TransitionTimeout = 90 * time.Second
	}
	if options.PollInterval <= 0 {
		options.PollInterval = 500 * time.Millisecond
	}
	if options.SubmitCommand == nil {
		options.SubmitCommand = func(dataDir, commandID string) error {
			return writePanelCommandWithID(dataDir, commandID, "save-now", nil)
		}
	}
	if options.GetOutcome == nil {
		options.GetOutcome = func(dataDir, commandID string) (CommandOutcome, error) {
			return d.importCommandOutcome(ctx, instance.ID, dataDir, commandID)
		}
	}
	if options.WaitTransition == nil {
		options.WaitTransition = waitForJunimoDayTransitionComplete
	}

	j, err := LoadImportJournal(instance.DataDir, operationID)
	if err != nil {
		return err
	}
	if j.Stage == ImportStageCompleted {
		return nil
	}
	if j.Stage != ImportStageFinalizeConfirmed && j.Stage != ImportStageSavePersisting && j.Stage != ImportStageSaveVerified {
		return recordImportDurableFailure(instance.DataDir, operationID, ImportErrorRecoveryRequired, "durable save requires a confirmed finalizer", "", nil)
	}
	if j.ActivationOutcome == activationOutcomeAsIsLoaded {
		if err := completeAsIsImportDurably(ctx, lifecycle, instance.DataDir, operationID, j); err != nil {
			return err
		}
		d.markCompletedImportRuntimeRunning(instance)
		return nil
	}
	if j.ActivationOutcome != activationOutcomeSwapFinalized {
		return recordImportDurableFailure(instance.DataDir, operationID, ImportErrorRecoveryRequired, "swap finalizer confirmation is missing", "", nil)
	}
	if j.DurableSaveSubmissionFailed {
		return recordImportDurableFailure(instance.DataDir, operationID, ImportErrorRecoveryRequired, "save-now submission previously failed and must not be retried", "", nil)
	}
	if !commandResultSupported(instance.DataDir) {
		return recordImportDurableFailure(instance.DataDir, operationID, ImportErrorRecoveryRequired, "Control command-result protocol is unavailable for durable save", "", nil)
	}

	shouldSubmit := false
	if j.DurableSaveCommandID == "" {
		before, captureErr := waitStableImportSaveDiskState(ctx, instance.DataDir, j.SaveName, 10*time.Second)
		if captureErr != nil {
			return recordImportDurableFailure(instance.DataDir, operationID, ImportErrorRecoveryRequired, "target save is unreadable before durable save", "", captureErr)
		}
		status, statusErr := readRuntimeWorldState(ctx, lifecycle, instance.DataDir)
		if statusErr != nil || status.Version == nil || status.DayTransitionComplete == nil {
			return recordImportDurableFailure(instance.DataDir, operationID, ImportErrorResultUnconfirmed, "Junimo status baseline is incomplete before durable save", durableSaveWarningTransitionMissing, statusErr)
		}
		commandID, idErr := randomHex(16)
		if idErr != nil {
			return recordImportDurableFailure(instance.DataDir, operationID, ImportErrorRecoveryRequired, "could not reserve durable save commandId", "", idErr)
		}
		j.DurableSaveBefore = &before
		j.DurableStatusBaselineVersion = status.Version
		j.DurableSaveCommandID = commandID
		j.Stage = ImportStageSavePersisting
		j.LastErrorCode, j.LastError, j.DurableSaveWarning = "", "", ""
		if err := WriteImportJournal(instance.DataDir, j); err != nil {
			return err
		}
		shouldSubmit = true
	}

	if shouldSubmit {
		if err := options.SubmitCommand(instance.DataDir, j.DurableSaveCommandID); err != nil {
			j.DurableSaveSubmissionFailed = true
			_ = WriteImportJournal(instance.DataDir, j)
			return recordImportDurableFailure(instance.DataDir, operationID, ImportErrorRecoveryRequired, "save-now command submission failed; the maintenance runtime was left running", "", err)
		}
		now := time.Now().UTC()
		j.DurableSaveSubmittedAt = &now
		if err := WriteImportJournal(instance.DataDir, j); err != nil {
			return recordImportDurableFailure(instance.DataDir, operationID, ImportErrorResultUnconfirmed, "save-now was published but its submission marker could not be journaled", "", err)
		}
		maintenanceLog(job, "Durable save-now submitted through Control; waiting for the same commandId GameLoop.Saved result.")
	} else {
		maintenanceLog(job, "Resuming durable save observation for the journaled commandId without submitting another command.")
	}

	outcome, err := waitForDurableSaveOutcome(ctx, instance.DataDir, j.DurableSaveCommandID, options)
	if err != nil {
		if typed, ok := AsImportTransactionError(err); ok {
			return recordImportDurableFailure(instance.DataDir, operationID, typed.Code, typed.Message, "", typed.Cause)
		}
		return recordImportDurableFailure(instance.DataDir, operationID, ImportErrorResultUnconfirmed, "save-now result could not be confirmed", "", err)
	}
	if outcome.CommandID != j.DurableSaveCommandID || outcome.Status != CommandStatusSucceeded {
		return recordImportDurableFailure(instance.DataDir, operationID, ImportErrorResultUnconfirmed, "save-now returned an unrelated or non-success result", "", nil)
	}
	j, err = LoadImportJournal(instance.DataDir, operationID)
	if err != nil {
		return err
	}
	j.DurableGameLoopSaved = true
	if err := WriteImportJournal(instance.DataDir, j); err != nil {
		return err
	}

	postSavedStatus, statusErr := readRuntimeWorldState(ctx, lifecycle, instance.DataDir)
	if statusErr != nil || postSavedStatus.Version == nil || postSavedStatus.DayTransitionComplete == nil {
		return recordImportDurableFailure(instance.DataDir, operationID, ImportErrorResultUnconfirmed, "post-Saved Junimo status cursor is incomplete", durableSaveWarningTransitionMissing, statusErr)
	}
	j, _ = LoadImportJournal(instance.DataDir, operationID)
	j.DurableStatusAfterSavedVersion = postSavedStatus.Version
	if err := WriteImportJournal(instance.DataDir, j); err != nil {
		return err
	}
	// This cursor is sampled only after the matching GameLoop.Saved result, so
	// a stale pre-save dayTransitionComplete=true snapshot cannot satisfy it.
	transitionErr := options.WaitTransition(ctx, lifecycle, instance.DataDir, *postSavedStatus.Version, options.TransitionTimeout, options.PollInterval)
	if transitionErr != nil {
		warning := durableSaveWarningTransitionTimeout
		if typed := activationEvidenceErrorCode(transitionErr); typed == "status_field_missing" || typed == "wait_status_version_missing" {
			warning = durableSaveWarningTransitionMissing
		}
		value := false
		j, _ = LoadImportJournal(instance.DataDir, operationID)
		j.DurableTransitionComplete = &value
		j.DurableSaveWarning = warning
		_ = WriteImportJournal(instance.DataDir, j)
		return recordImportDurableFailure(instance.DataDir, operationID, ImportErrorResultUnconfirmed, "GameLoop.Saved succeeded but world stability is unconfirmed", warning, transitionErr)
	}
	value := true
	j, _ = LoadImportJournal(instance.DataDir, operationID)
	j.DurableTransitionComplete = &value
	j.DurableSaveWarning = ""
	if err := WriteImportJournal(instance.DataDir, j); err != nil {
		return err
	}

	after, err := waitStableImportSaveDiskState(ctx, instance.DataDir, j.SaveName, 10*time.Second)
	if err != nil {
		return recordImportDurableFailure(instance.DataDir, operationID, ImportErrorRecoveryRequired, "target save is invalid after GameLoop.Saved", "", err)
	}
	if j.DurableSaveBefore == nil || !importSaveDiskChanged(*j.DurableSaveBefore, after) {
		return recordImportDurableFailure(instance.DataDir, operationID, ImportErrorRecoveryRequired, "GameLoop.Saved did not produce a verifiable target save file change", "", nil)
	}
	j.DurableSaveAfter = &after
	j.Stage = ImportStageSaveVerified
	j.LastErrorCode, j.LastError = "", ""
	if err := WriteImportJournal(instance.DataDir, j); err != nil {
		return err
	}
	// Keep save_verified as a durable crash boundary, then close the transaction.
	j.Stage = ImportStageCompleted
	if err := WriteImportJournal(instance.DataDir, j); err != nil {
		return err
	}
	maintenanceLog(job, "GameLoop.Saved, dayTransitionComplete, stable XML, and target save disk change verified; import transaction completed.")
	d.markCompletedImportRuntimeRunning(instance)
	return nil
}

func (d *Driver) importCommandOutcome(ctx context.Context, instanceID, dataDir, commandID string) (CommandOutcome, error) {
	outcome, err := GetCommandOutcome(dataDir, commandID)
	persistedStore, ok := d.store.(interface {
		GetControlCommand(context.Context, string) (storage.ControlCommand, error)
	})
	if err != nil || outcome.Status != CommandStatusUnknown || !ok {
		return outcome, err
	}
	persisted, persistedErr := persistedStore.GetControlCommand(ctx, commandID)
	if persistedErr == nil && persisted.InstanceID == instanceID {
		return CommandOutcome{CommandID: persisted.CommandID, Status: CommandStatus(persisted.Status),
			ErrorCode: persisted.ErrorCode, Message: persisted.ResultMessage,
			CreatedAt: persisted.SubmittedAt, UpdatedAt: persisted.UpdatedAt, Details: persisted.ResultDetails}, nil
	}
	if persistedErr != nil && !errors.Is(persistedErr, storage.ErrNotFound) {
		return outcome, persistedErr
	}
	return outcome, nil
}

// markCompletedImportRuntimeRunning is deliberately called only after the
// completed journal boundary. The maintenance runtime must not be advertised as
// joinable while Phase A, activation, finalization, or durable saving is active.
func (d *Driver) markCompletedImportRuntimeRunning(instance registry.Instance) {
	d.updatePhase(context.Background(), instance.ID, storage.InstanceStateRunning,
		"服务器运行中（存档导入已完成）", "running", "")
}

func waitForDurableSaveOutcome(ctx context.Context, dataDir, commandID string, options importDurableSaveOptions) (CommandOutcome, error) {
	deadline := time.NewTimer(options.CommandTimeout)
	defer deadline.Stop()
	ticker := time.NewTicker(options.PollInterval)
	defer ticker.Stop()
	for {
		outcome, err := options.GetOutcome(dataDir, commandID)
		if err != nil {
			return outcome, err
		}
		if outcome.CommandID != commandID {
			return outcome, &ImportTransactionError{Code: ImportErrorResultUnconfirmed, Message: "save-now commandId result mismatch"}
		}
		switch outcome.Status {
		case CommandStatusSucceeded:
			return outcome, nil
		case CommandStatusFailed:
			return outcome, &ImportTransactionError{Code: ImportErrorRecoveryRequired, Message: "save-now failed before GameLoop.Saved confirmed persistence"}
		case CommandStatusExpired:
			return outcome, &ImportTransactionError{Code: ImportErrorResultUnconfirmed, Message: "save-now command result is unknown or expired"}
		case CommandStatusUnknown, CommandStatusDispatched, CommandStatusQueued, CommandStatusRunning:
		default:
			return outcome, &ImportTransactionError{Code: ImportErrorResultUnconfirmed, Message: "save-now command result status is invalid"}
		}
		select {
		case <-ctx.Done():
			return outcome, ctx.Err()
		case <-deadline.C:
			return outcome, &ImportTransactionError{Code: ImportErrorResultUnconfirmed, Message: "save-now command result timed out"}
		case <-ticker.C:
		}
	}
}

func completeAsIsImportDurably(ctx context.Context, lifecycle LifecycleDockerService, dataDir, operationID string, j ImportJournal) error {
	evidence, err := captureImportActivationEvidence(ctx, lifecycle, dataDir)
	if err != nil || evidence.RuntimeSaveID != j.SaveName || evidence.PendingIntent.Exists || !activationWorldStable(evidence) {
		return recordImportDurableFailure(dataDir, operationID, ImportErrorResultUnconfirmed, "as-is target world is not stably loaded", "", err)
	}
	if j.DurableSaveCommandID != "" {
		return recordImportDurableFailure(dataDir, operationID, ImportErrorRecoveryRequired, "as-is import unexpectedly contains a save-now command", "", nil)
	}
	j.Stage = ImportStageSaveVerified
	j.LastErrorCode, j.LastError = "", ""
	if err := WriteImportJournal(dataDir, j); err != nil {
		return err
	}
	j.Stage = ImportStageCompleted
	return WriteImportJournal(dataDir, j)
}

func readJunimoWaitStatus(ctx context.Context, exec commandExecutor, dataDir string, since int64) (JunimoRuntimeWorldState, bool, bool, error) {
	apiPort, apiKey, err := readJunimoAPIConfig(dataDir)
	if err != nil {
		return JunimoRuntimeWorldState{}, false, false, err
	}
	endpoint := "http://localhost:" + apiPort + "/wait/status?since=" + strconv.FormatInt(since, 10) + "&dayTransitionComplete=true&timeout=8000"
	args := []string{"curl", "-sS", "--max-time", "10", "-w", "\n%{http_code}"}
	if apiKey != "" {
		args = append(args, "-H", "Authorization: Bearer "+apiKey)
	}
	args = append(args, endpoint)
	requestCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	result, execErr := exec.ComposeExecPipe(requestCtx, dataDir, "server", "", args...)
	if execErr != nil || result.ExitCode != 0 {
		return JunimoRuntimeWorldState{}, false, false, &ImportEvidenceError{Code: "junimo_api_unavailable", Message: "Junimo wait status API is unavailable", Cause: execErr}
	}
	output := strings.TrimRight(result.Stdout, "\r\n")
	idx := strings.LastIndex(output, "\n")
	if idx < 0 {
		if len(output) == 3 {
			idx = 0
			output = "\n" + output
		} else {
			return JunimoRuntimeWorldState{}, false, false, &ImportEvidenceError{Code: "status_invalid_json", Message: "Junimo wait status response is invalid"}
		}
	}
	body, statusCode := strings.TrimSpace(output[:idx]), strings.TrimSpace(output[idx+1:])
	switch statusCode {
	case "408":
		return JunimoRuntimeWorldState{}, false, false, nil
	case "404", "405":
		return JunimoRuntimeWorldState{}, false, true, nil
	case "200":
	default:
		return JunimoRuntimeWorldState{}, false, false, &ImportEvidenceError{Code: "junimo_api_unavailable", Message: "Junimo wait status API returned HTTP " + statusCode}
	}
	var state JunimoRuntimeWorldState
	if err := json.Unmarshal([]byte(body), &state); err != nil {
		return state, false, false, &ImportEvidenceError{Code: "status_invalid_json", Message: "Junimo wait status JSON is invalid", Cause: err}
	}
	if state.Version == nil {
		return state, false, false, &ImportEvidenceError{Code: "wait_status_version_missing", Message: "Junimo wait status response is missing version"}
	}
	if state.DayTransitionComplete == nil {
		return state, false, false, &ImportEvidenceError{Code: "status_field_missing", Message: "Junimo wait status response is missing dayTransitionComplete"}
	}
	return state, *state.DayTransitionComplete, false, nil
}

func waitForJunimoDayTransitionComplete(ctx context.Context, exec commandExecutor, dataDir string, since int64, timeout, pollInterval time.Duration) error {
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cursor := since
	useWait := true
	if pollInterval < 2*time.Second {
		pollInterval = 2 * time.Second
	}
	for {
		if useWait {
			state, matched, unsupported, err := readJunimoWaitStatus(waitCtx, exec, dataDir, cursor)
			if err != nil {
				return err
			}
			if unsupported {
				useWait = false
			} else {
				if state.Version != nil {
					cursor = *state.Version
				}
				if matched {
					return nil
				}
				continue
			}
		}
		state, err := readRuntimeWorldState(waitCtx, exec, dataDir)
		if err != nil {
			return err
		}
		if state.DayTransitionComplete == nil {
			return &ImportEvidenceError{Code: "status_field_missing", Message: "Junimo status response is missing dayTransitionComplete"}
		}
		if *state.DayTransitionComplete {
			return nil
		}
		select {
		case <-waitCtx.Done():
			return waitCtx.Err()
		case <-time.After(pollInterval):
		}
	}
}

func recordImportDurableFailure(dataDir, operationID, code, message, warning string, cause error) error {
	if j, err := LoadImportJournal(dataDir, operationID); err == nil {
		j.LastErrorCode, j.LastError = code, message
		if warning != "" {
			j.DurableSaveWarning = warning
		}
		_ = WriteImportJournal(dataDir, j)
	}
	return &ImportTransactionError{Code: code, Message: message, Cause: cause}
}
