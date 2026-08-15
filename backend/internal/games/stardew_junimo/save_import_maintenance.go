package stardew_junimo

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

const importMaintenancePhase = "save_import_maintenance"

type importMaintenanceOptions struct {
	ReadyTimeout time.Duration
	PollInterval time.Duration
}

func defaultImportMaintenanceOptions() importMaintenanceOptions {
	return importMaintenanceOptions{ReadyTimeout: 5 * time.Minute, PollInterval: time.Second}
}

// IsSaveImportMaintenanceOfflineState is the shared persisted-state contract
// for starting the private save-import runtime. Docker state remains the
// authoritative second gate: an allowed database state never proves that the
// server service is actually stopped.
func IsSaveImportMaintenanceOfflineState(state string) bool {
	switch state {
	case storage.InstanceStateGameInstalled,
		storage.InstanceStateSaveRequired,
		storage.InstanceStateReadyToStart,
		storage.InstanceStateStopped:
		return true
	default:
		return false
	}
}

func (d *Driver) inspectSaveImportMaintenanceOffline(ctx context.Context, instance registry.Instance) (storage.Instance, error) {
	if d.store == nil {
		return storage.Instance{}, &ImportTransactionError{Code: ImportErrorMaintenanceStart, Message: "instance state store is unavailable before save import maintenance"}
	}
	original, err := d.store.GetInstance(ctx, instance.ID)
	if err != nil {
		return storage.Instance{}, &ImportTransactionError{Code: ImportErrorMaintenanceStart, Message: "failed to load instance state before save import maintenance", Cause: err}
	}
	if !IsSaveImportMaintenanceOfflineState(original.State) {
		return original, &ImportTransactionError{Code: ImportErrorSaveInProgress, Message: "instance state does not allow save import maintenance startup"}
	}
	lifecycle, ok := d.docker.(LifecycleDockerService)
	if !ok {
		return original, &ImportTransactionError{Code: ImportErrorMaintenanceStart, Message: "maintenance runtime is unavailable", Cause: errors.New("docker lifecycle operations are unsupported")}
	}
	if err := ensureSaveImportServerStopped(ctx, lifecycle, original.DataDir); err != nil {
		return original, err
	}
	return original, nil
}

func ensureSaveImportServerStopped(ctx context.Context, lifecycle LifecycleDockerService, dataDir string) error {
	ps, err := lifecycle.ComposePsStrict(ctx, dataDir)
	if err != nil {
		return &ImportTransactionError{Code: ImportErrorMaintenanceStart, Message: "failed to inspect runtime before save import maintenance", Cause: err}
	}
	stopped, err := saveImportServerStoppedStrict(ps.Services)
	if err != nil {
		return &ImportTransactionError{Code: ImportErrorMaintenanceStart, Message: "server runtime state is not safely classifiable before save import maintenance", Cause: err}
	}
	if !stopped {
		return &ImportTransactionError{Code: ImportErrorSaveInProgress, Message: "server service is running; save import maintenance was not started"}
	}
	return nil
}

func saveImportServerStoppedStrict(services []paneldocker.ComposeService) (bool, error) {
	for _, service := range services {
		if !strings.EqualFold(strings.TrimSpace(service.Service), "server") {
			continue
		}
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(service.Status)), "up") {
			return false, nil
		}
		switch strings.ToLower(strings.TrimSpace(service.State)) {
		case "exited", "dead":
			continue
		case "running", "restarting", "paused", "created", "removing":
			return false, nil
		default:
			return false, fmt.Errorf("unrecognized server service state %q", service.State)
		}
	}
	return true, nil
}

// runImportMaintenance starts a deliberately non-joinable runtime. It bypasses
// the normal Start readiness path: no invite polling, new-game command, active
// pointer update, or import command is performed here.
func (d *Driver) runImportMaintenance(ctx context.Context, instance registry.Instance, operationID string, job *jobs.Context, options importMaintenanceOptions) (retErr error) {
	if options.ReadyTimeout <= 0 {
		options.ReadyTimeout = 5 * time.Minute
	}
	if options.PollInterval <= 0 {
		options.PollInterval = time.Second
	}

	original, err := d.inspectSaveImportMaintenanceOffline(ctx, instance)
	dataDir := instance.DataDir
	if strings.TrimSpace(original.DataDir) != "" {
		dataDir = original.DataDir
	}
	if err != nil {
		if typed, ok := AsImportTransactionError(err); ok {
			return d.recordMaintenanceFailure(dataDir, operationID, typed.Code, typed.Message, typed.Cause)
		}
		return d.recordMaintenanceFailure(dataDir, operationID, ImportErrorMaintenanceStart, "save import maintenance preflight failed", err)
	}
	lifecycle, ok := d.docker.(LifecycleDockerService)
	if !ok {
		return d.recordMaintenanceFailure(dataDir, operationID, ImportErrorMaintenanceStart, "maintenance runtime is unavailable", errors.New("docker lifecycle operations are unsupported"))
	}
	journal, err := LoadImportJournal(dataDir, operationID)
	if err != nil {
		return err
	}
	if !importStageAtLeast(journal.Stage, ImportStageBackupCreated) || journal.PreimportBackupName == "" {
		return d.recordMaintenanceFailure(dataDir, operationID, ImportErrorMaintenanceStart, "staging and preimport backup are incomplete", nil)
	}
	if err := validateImportStaticCapability(dataDir); err != nil {
		return d.recordMaintenanceFailure(dataDir, operationID, ImportErrorUnsupported, "Junimo .125 static capability check failed", err)
	}
	pointerBefore, pointerErr := readActivePointerStrict(dataDir)
	if pointerErr != nil || journal.OriginalActiveSave == "" || pointerBefore != journal.OriginalActiveSave {
		return d.recordMaintenanceFailure(dataDir, operationID, ImportErrorMaintenanceReady,
			"active save pointer is unavailable or changed before maintenance startup", pointerErr)
	}

	// Recheck immediately before publishing the maintenance phase and starting
	// Compose. Runtime state may have changed while static and pointer evidence
	// was validated above.
	if err := ensureSaveImportServerStopped(ctx, lifecycle, dataDir); err != nil {
		if typed, ok := AsImportTransactionError(err); ok {
			return d.recordMaintenanceFailure(dataDir, operationID, typed.Code, typed.Message, typed.Cause)
		}
		return d.recordMaintenanceFailure(dataDir, operationID, ImportErrorMaintenanceStart, "save import maintenance runtime recheck failed", err)
	}

	// Persist the exact pre-maintenance snapshot before changing the database.
	// The journal is the write-ahead recovery record for every later crash
	// window, so a failed snapshot write leaves both Compose and the instance
	// state untouched.
	captureOriginalInstanceSnapshot(&journal, original)
	journal.MaintenanceRecoveryState = importMaintenanceSnapshotCaptured
	if err := d.writeImportJournal(dataDir, journal); err != nil {
		return maintenanceRollbackError("failed to persist the pre-maintenance instance snapshot", err)
	}
	if err := d.updateImportMaintenancePhase(ctx, "Save import maintenance runtime is starting; the server is not join-ready.", original); err != nil {
		primary := d.recordMaintenanceFailure(dataDir, operationID, ImportErrorMaintenanceStart, "failed to publish save import maintenance state", err)
		if restoreErr := d.restoreImportMaintenanceSnapshot(dataDir, operationID, original); restoreErr != nil {
			return joinMaintenanceRollback(primary, "failed to restore the instance after maintenance phase publication failed", restoreErr)
		}
		return primary
	}

	// Mark the runtime as potentially started before ComposeUp. Once this write
	// is attempted, even an error may mean the atomic rename reached disk; the
	// failure path therefore performs ComposeDown plus a strict fresh probe.
	journal.MaintenanceStarted = true
	journal.MaintenanceRecoveryState = importMaintenanceStartIntentPersisted
	if err := d.writeImportJournal(dataDir, journal); err != nil {
		primary := d.recordMaintenanceFailure(dataDir, operationID, ImportErrorRecoveryRequired, "failed to persist save import maintenance start intent", err)
		stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if rollbackErr := d.stopAndRestoreImportMaintenance(stopCtx, lifecycle, dataDir, operationID, original); rollbackErr != nil {
			return joinMaintenanceRollback(primary, "failed to recover after maintenance start-intent persistence failed", rollbackErr)
		}
		return primary
	}
	defer func() {
		if retErr == nil {
			return
		}
		stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if rollbackErr := d.stopAndRestoreImportMaintenance(stopCtx, lifecycle, dataDir, operationID, original); rollbackErr != nil {
			retErr = joinMaintenanceRollback(retErr, "failed to stop and restore save import maintenance", rollbackErr)
		}
	}()
	maintenanceLog(job, "Starting save_import_maintenance runtime without publishing an invite code.")

	up, err := lifecycle.ComposeUp(ctx, dataDir)
	if err != nil || up.ExitCode != 0 {
		if errors.Is(ctx.Err(), context.Canceled) {
			return d.recordMaintenanceFailure(dataDir, operationID, ImportErrorMaintenanceCancel, "save import maintenance was canceled", ctx.Err())
		}
		if err == nil {
			err = fmt.Errorf("compose up exited with code %d", up.ExitCode)
		}
		return d.recordMaintenanceFailure(dataDir, operationID, ImportErrorMaintenanceStart, "maintenance container startup failed", err)
	}
	journal, err = LoadImportJournal(dataDir, operationID)
	if err != nil {
		return d.recordMaintenanceFailure(dataDir, operationID, ImportErrorRecoveryRequired, "maintenance container started but its journal could not be reloaded", err)
	}
	journal.MaintenanceRecoveryState = importMaintenanceComposeUpReturned
	if err := d.writeImportJournal(dataDir, journal); err != nil {
		return d.recordMaintenanceFailure(dataDir, operationID, ImportErrorRecoveryRequired, "maintenance container started but its state could not be persisted", err)
	}

	readyCtx, cancel := context.WithTimeout(ctx, options.ReadyTimeout)
	defer cancel()
	if err := waitForImportServerContainer(readyCtx, lifecycle, dataDir, options.PollInterval); err != nil {
		return d.maintenanceReadinessFailure(dataDir, operationID, err)
	}
	maintenanceLog(job, "Server container is running; checking FIFO, log, API, Junimo version, and saves command.")
	if err := waitForImportRuntimeProbes(readyCtx, lifecycle, dataDir, options.PollInterval); err != nil {
		return d.maintenanceReadinessFailure(dataDir, operationID, err)
	}

	identityBefore, err := readProcessIdentity(readyCtx, lifecycle, dataDir)
	if err != nil {
		return d.maintenanceReadinessFailure(dataDir, operationID, err)
	}
	if err := waitForImportSavesCommand(readyCtx, lifecycle, dataDir, options.PollInterval); err != nil {
		return d.maintenanceReadinessFailure(dataDir, operationID, err)
	}
	if err := rejectConnectedFarmhands(readyCtx, lifecycle, dataDir); err != nil {
		return d.maintenanceReadinessFailure(dataDir, operationID, err)
	}

	offset, err := strictServerLogSize(readyCtx, lifecycle, dataDir)
	if err != nil {
		return d.maintenanceReadinessFailure(dataDir, operationID, err)
	}
	snapshot, err := waitForImportEvidenceBaseline(readyCtx, lifecycle, dataDir, journal.SaveName,
		journal.OriginalActiveSave, *identityBefore, options.PollInterval)
	if err != nil {
		return d.maintenanceReadinessFailure(dataDir, operationID, err)
	}

	journal, err = LoadImportJournal(dataDir, operationID)
	if err != nil {
		return err
	}
	journal.RuntimeBaseline = &snapshot
	journal.ServerOutputLogOffset = &offset
	journal.Stage = ImportStageRuntimeReady
	journal.MaintenanceRecoveryState = importMaintenanceRuntimeReadyPersisted
	journal.LastErrorCode, journal.LastError = "", ""
	if err := d.writeImportJournal(dataDir, journal); err != nil {
		return d.recordMaintenanceFailure(dataDir, operationID, ImportErrorRecoveryRequired, "maintenance runtime became ready but its evidence could not be persisted", err)
	}
	if err := d.updateImportMaintenancePhase(ctx, "Save import maintenance runtime is ready; the server is not join-ready.", original); err != nil {
		return d.recordMaintenanceFailure(dataDir, operationID, ImportErrorMaintenanceReady, "failed to publish ready save import maintenance state", err)
	}
	maintenanceLog(job, "save_import_maintenance runtime ready; composite evidence baseline captured. No import command was sent.")
	return nil
}

// waitForImportEvidenceBaseline keeps the strict baseline gate, but tolerates
// the real startup window where the saves command is registered before the
// loaded world and diagnostics fields have become observable. It never accepts
// a different process generation or active pointer while waiting.
func waitForImportEvidenceBaseline(ctx context.Context, lifecycle LifecycleDockerService, dataDir, saveName, originalPointer string,
	expectedIdentity JunimoProcessIdentity, interval time.Duration) (JunimoImportEvidenceSnapshot, error) {
	var lastUnknown []string
	for {
		if err := rejectConnectedFarmhands(ctx, lifecycle, dataDir); err != nil {
			return JunimoImportEvidenceSnapshot{}, err
		}
		snapshot, err := CaptureJunimoImportEvidence(ctx, lifecycle, dataDir, saveName)
		if err != nil {
			return JunimoImportEvidenceSnapshot{}, err
		}
		lastUnknown = append(lastUnknown[:0], snapshot.UnknownFields...)
		if snapshot.ProcessIdentity != nil && *snapshot.ProcessIdentity != expectedIdentity {
			return JunimoImportEvidenceSnapshot{}, &ImportTransactionError{
				Code: ImportErrorMaintenanceProcess, Message: "maintenance process identity changed during readiness checks",
			}
		}
		if snapshot.ActivePointer != "" && snapshot.ActivePointer != originalPointer {
			return JunimoImportEvidenceSnapshot{}, &ImportTransactionError{
				Code: ImportErrorMaintenanceReady, Message: "active save pointer changed during maintenance startup",
			}
		}
		if snapshot.MainSaveSHA256 != "" && snapshot.FinalizeCount != nil && snapshot.ProcessIdentity != nil &&
			snapshot.ActivePointer == originalPointer {
			return snapshot, nil
		}
		if err := waitImportPoll(ctx, interval); err != nil {
			if errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				return JunimoImportEvidenceSnapshot{}, err
			}
			message := "maintenance evidence baseline is incomplete"
			if len(lastUnknown) > 0 {
				message += ": " + strings.Join(lastUnknown, ",")
			}
			return JunimoImportEvidenceSnapshot{}, &ImportTransactionError{Code: ImportErrorMaintenanceReady, Message: message}
		}
	}
}

// waitForImportSavesCommand tolerates the real .125 startup window where the
// HTTP API is already listening but the game is still loading and Junimo's
// console commands have not produced output yet. The probe is read-only; the
// formal import command is never retried here.
func waitForImportSavesCommand(ctx context.Context, lifecycle LifecycleDockerService, dataDir string, interval time.Duration) error {
	var lastErr error
	for {
		if err := rejectConnectedFarmhands(ctx, lifecycle, dataDir); err != nil {
			return err
		}
		output, exitCode, _, err := sendServerCommand(ctx, lifecycle, dataDir, "saves")
		if err == nil && exitCode == 0 && isSavesListOutput(output) {
			return nil
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = errors.New("saves list probe produced no recognized response")
		}
		if err := waitImportPoll(ctx, interval); err != nil {
			if errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			return &ImportTransactionError{Code: ImportErrorMaintenanceSaves, Message: "Junimo saves command is unavailable", Cause: lastErr}
		}
	}
}

func waitForImportServerContainer(ctx context.Context, lifecycle LifecycleDockerService, dataDir string, interval time.Duration) error {
	for {
		ps, err := lifecycle.ComposePs(ctx, dataDir)
		if err == nil && serverServiceUp(ps.Services) {
			return nil
		}
		for _, service := range ps.Services {
			if service.Service == "server" && strings.EqualFold(service.State, "exited") {
				return &ImportTransactionError{Code: ImportErrorMaintenanceStart, Message: "maintenance server container exited before readiness"}
			}
		}
		if err := waitImportPoll(ctx, interval); err != nil {
			return err
		}
	}
}

func waitForImportRuntimeProbes(ctx context.Context, lifecycle LifecycleDockerService, dataDir string, interval time.Duration) error {
	var lastErr error
	for {
		lastErr = probeImportRuntime(ctx, lifecycle, dataDir)
		if lastErr == nil {
			return nil
		}
		if typed, ok := AsImportTransactionError(lastErr); ok {
			if typed.Code == ImportErrorMaintenanceMod || typed.Code == ImportErrorMaintenanceControl {
				return typed
			}
		}
		if err := waitImportPoll(ctx, interval); err != nil {
			if errors.Is(err, context.Canceled) {
				return err
			}
			if typed, ok := AsImportTransactionError(lastErr); ok {
				return typed
			}
			return &ImportTransactionError{Code: ImportErrorMaintenanceReady, Message: "maintenance runtime did not become ready", Cause: lastErr}
		}
	}
}

func probeImportRuntime(ctx context.Context, lifecycle LifecycleDockerService, dataDir string) error {
	if result, err := lifecycle.ComposeExecPipe(ctx, dataDir, "server", "", "test", "-p", serverInputFIFO); err != nil || result.ExitCode != 0 {
		return &ImportTransactionError{Code: ImportErrorMaintenanceFIFO, Message: "SMAPI input FIFO is unavailable", Cause: err}
	}
	if result, err := lifecycle.ComposeExecPipe(ctx, dataDir, "server", "", "test", "-r", serverOutputLog); err != nil || result.ExitCode != 0 {
		return &ImportTransactionError{Code: ImportErrorMaintenanceLog, Message: "SMAPI output log is unavailable", Cause: err}
	}
	if _, err := strictServerLogSize(ctx, lifecycle, dataDir); err != nil {
		return &ImportTransactionError{Code: ImportErrorMaintenanceLog, Message: "SMAPI output log is unavailable", Cause: err}
	}
	if _, err := readJunimoAPI(ctx, lifecycle, dataDir, "/health"); err != nil {
		if _, statusErr := readJunimoAPI(ctx, lifecycle, dataDir, "/status"); statusErr != nil {
			return &ImportTransactionError{Code: ImportErrorMaintenanceAPI, Message: "Junimo health and status APIs are unavailable", Cause: statusErr}
		}
	}
	if err := verifyRunningJunimoVersion(ctx, lifecycle, dataDir); err != nil {
		return err
	}
	control := InspectControlRuntimeGate(dataDir)
	switch control.State {
	case ControlRuntimeGateReady:
		return nil
	case ControlRuntimeGatePending:
		return errors.New("Control runtime evidence is not ready")
	default:
		return &ImportTransactionError{Code: ImportErrorMaintenanceControl, Message: "running Control version or DLL does not match the Panel runtime manifest"}
	}
}

func verifyRunningJunimoVersion(ctx context.Context, lifecycle LifecycleDockerService, dataDir string) error {
	result, err := lifecycle.ComposeExecPipe(ctx, dataDir, "server", "", "cat", "/data/Mods/JunimoServer/manifest.json")
	if err != nil || result.ExitCode != 0 {
		return &ImportTransactionError{Code: ImportErrorMaintenanceMod, Message: "running Junimo manifest is unavailable", Cause: err}
	}
	var manifest struct {
		Version string `json:"Version"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &manifest); err != nil {
		return &ImportTransactionError{Code: ImportErrorMaintenanceMod, Message: "running Junimo manifest is invalid", Cause: err}
	}
	if strings.TrimSpace(manifest.Version) != TestedImageTag {
		return &ImportTransactionError{Code: ImportErrorMaintenanceMod, Message: "running Junimo version mismatch"}
	}
	return nil
}

func rejectConnectedFarmhands(ctx context.Context, lifecycle LifecycleDockerService, dataDir string) error {
	raw, err := readJunimoAPI(ctx, lifecycle, dataDir, "/status")
	if err != nil {
		return err
	}
	var status struct {
		PlayerCount *int `json:"playerCount"`
	}
	if err := json.Unmarshal(raw, &status); err != nil || status.PlayerCount == nil {
		return &ImportTransactionError{Code: ImportErrorMaintenanceReady, Message: "Junimo status is missing playerCount", Cause: err}
	}
	if *status.PlayerCount > 0 {
		return &ImportTransactionError{Code: ImportErrorPlayersConnected, Message: "farmhands connected during save import maintenance; no players were kicked"}
	}
	return nil
}

func strictServerLogSize(ctx context.Context, lifecycle LifecycleDockerService, dataDir string) (int64, error) {
	result, err := lifecycle.ComposeExecPipe(ctx, dataDir, "server", "", "wc", "-c", serverOutputLog)
	if err != nil || result.ExitCode != 0 {
		return 0, errors.New("SMAPI output log is not readable")
	}
	fields := strings.Fields(result.Stdout)
	if len(fields) == 0 {
		return 0, errors.New("SMAPI output log size is invalid")
	}
	size, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil || size < 0 {
		return 0, errors.New("SMAPI output log size is invalid")
	}
	return size, nil
}

func isSavesListOutput(output string) bool {
	lower := strings.ToLower(output)
	return strings.Contains(lower, "available saves:") || strings.Contains(lower, "no saves directory found")
}

func waitImportPoll(ctx context.Context, interval time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(interval):
		return nil
	}
}

func (d *Driver) maintenanceReadinessFailure(dataDir, operationID string, err error) error {
	if errors.Is(err, context.Canceled) {
		return d.recordMaintenanceFailure(dataDir, operationID, ImportErrorMaintenanceCancel, "save import maintenance was canceled", err)
	}
	if typed, ok := AsImportTransactionError(err); ok {
		return d.recordMaintenanceFailure(dataDir, operationID, typed.Code, typed.Message, typed.Cause)
	}
	return d.recordMaintenanceFailure(dataDir, operationID, ImportErrorMaintenanceReady, "save import maintenance runtime did not become ready", err)
}

func (d *Driver) writeImportJournal(dataDir string, journal ImportJournal) error {
	if d.importJournalWrite != nil {
		return d.importJournalWrite(dataDir, journal)
	}
	return WriteImportJournal(dataDir, journal)
}

func (d *Driver) recordMaintenanceFailure(dataDir, operationID, code, message string, cause error) error {
	primary := &ImportTransactionError{Code: code, Message: message, Cause: cause}
	journal, err := LoadImportJournal(dataDir, operationID)
	if err == nil {
		journal.LastErrorCode, journal.LastError = code, message
		if writeErr := d.writeImportJournal(dataDir, journal); writeErr != nil {
			return joinMaintenanceRollback(primary, "failed to persist save import maintenance failure", writeErr)
		}
	} else {
		return joinMaintenanceRollback(primary, "failed to load save import journal while recording maintenance failure", err)
	}
	return primary
}

func restoreInstanceState(d *Driver, original storage.Instance) error {
	if d.store == nil {
		return errors.New("instance state store is unavailable")
	}
	_, err := d.store.RestoreInstanceStateSnapshot(context.Background(), original)
	return err
}

func (d *Driver) updateImportMaintenancePhase(ctx context.Context, message string, original storage.Instance) error {
	if d.store == nil {
		return errors.New("instance state store is unavailable")
	}
	payload := map[string]any{}
	if strings.TrimSpace(original.DriverPayload) != "" {
		if err := json.Unmarshal([]byte(original.DriverPayload), &payload); err != nil {
			return fmt.Errorf("parse original driver payload: %w", err)
		}
	}
	delete(payload, "invite_code")
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode maintenance driver payload: %w", err)
	}
	_, err = d.store.UpdateInstanceState(ctx, storage.UpdateInstanceStateParams{
		ID: original.ID, State: storage.InstanceStateStopped, StateMessage: message,
		DriverPhase: importMaintenancePhase, DriverPayload: string(encoded),
	})
	return err
}

func maintenanceRollbackError(message string, cause error) error {
	if cause == nil {
		cause = errors.New(message)
	}
	return &ImportTransactionError{Code: ImportErrorRecoveryRequired, Message: message, Cause: cause}
}

func joinMaintenanceRollback(original error, message string, cause error) error {
	return &ImportTransactionError{
		Code: ImportErrorRecoveryRequired, Message: message,
		Cause: errors.Join(original, cause),
	}
}

func captureOriginalInstanceSnapshot(journal *ImportJournal, original storage.Instance) {
	journal.OriginalInstanceSnapshot = &ImportInstanceStateSnapshot{
		State:               original.State,
		StateMessageValid:   original.StateMessage.Valid,
		StateMessageBase64:  base64.StdEncoding.EncodeToString([]byte(original.StateMessage.String)),
		DriverPhaseBase64:   base64.StdEncoding.EncodeToString([]byte(original.DriverPhase)),
		DriverPayloadBase64: base64.StdEncoding.EncodeToString([]byte(original.DriverPayload)),
	}
	// Keep the legacy fields during the schema-1 compatibility window. New
	// recovery always prefers the byte-exact snapshot above.
	journal.OriginalInstanceState = original.State
	journal.OriginalInstanceStateMessage = original.StateMessage.String
	journal.OriginalInstanceStateMessageValid = original.StateMessage.Valid
	journal.OriginalInstanceDriverPhase = original.DriverPhase
	journal.OriginalInstanceDriverPayload = original.DriverPayload
}

func (d *Driver) stopAndRestoreImportMaintenance(ctx context.Context, lifecycle LifecycleDockerService, dataDir, operationID string, expected storage.Instance) error {
	down, err := lifecycle.ComposeDown(ctx, dataDir)
	if err != nil {
		return fmt.Errorf("compose down save import maintenance: %w", err)
	}
	if down.ExitCode != 0 {
		return fmt.Errorf("compose down save import maintenance exited with code %d", down.ExitCode)
	}
	if err := ensureSaveImportServerStopped(ctx, lifecycle, dataDir); err != nil {
		return fmt.Errorf("strictly prove save import maintenance stopped: %w", err)
	}
	return d.restoreImportMaintenanceSnapshot(dataDir, operationID, expected)
}

func (d *Driver) restoreImportMaintenanceSnapshot(dataDir, operationID string, expected storage.Instance) error {
	journal, err := LoadImportJournal(dataDir, operationID)
	if err != nil {
		return fmt.Errorf("load save import journal for snapshot restore: %w", err)
	}
	if journal.PhaseAFIFOWriteAttempted || journal.UpstreamSubmitted || journal.UpstreamConfirmed || importStageAtLeast(journal.Stage, ImportStageSubmitted) {
		return errors.New("save import journal cannot prove that upstream submission never started")
	}
	original, err := originalInstanceSnapshotFromJournal(journal, dataDir)
	if err != nil {
		return err
	}
	if expected.ID != "" && (original.ID != expected.ID || filepath.Clean(original.DataDir) != filepath.Clean(expected.DataDir)) {
		return errors.New("save import snapshot identity changed during recovery")
	}
	journal.MaintenanceStarted = false
	journal.MaintenanceRecoveryState = importMaintenanceSnapshotRestorePending
	if err := d.writeImportJournal(dataDir, journal); err != nil {
		return fmt.Errorf("persist MaintenanceStarted=false before snapshot restore: %w", err)
	}
	if err := restoreInstanceState(d, original); err != nil {
		return fmt.Errorf("restore exact pre-maintenance instance snapshot: %w", err)
	}
	journal.MaintenanceRecoveryState = importMaintenanceSnapshotRestored
	journal.RecoveryState = "safe_to_resume_or_cleanup"
	if err := d.writeImportJournal(dataDir, journal); err != nil {
		return fmt.Errorf("persist completed instance snapshot restore: %w", err)
	}
	return nil
}

func (d *Driver) recoverInterruptedImportMaintenance(ctx context.Context, instance registry.Instance, recoveries []ImportRecovery) ([]ImportRecovery, error) {
	for i := range recoveries {
		recovery := &recoveries[i]
		if recovery.State == "manual_required" {
			journal, err := LoadImportJournal(instance.DataDir, recovery.OperationID)
			if err != nil {
				return recoveries, maintenanceRollbackError("failed to inspect manual save import recovery", err)
			}
			if journal.MaintenanceStarted || journal.PhaseAFIFOWriteAttempted || journal.MaintenanceRecoveryState == importMaintenanceManualRecovery {
				lifecycle, ok := d.docker.(LifecycleDockerService)
				if !ok {
					return recoveries, d.persistImportManualRecovery(instance.DataDir, journal,
						"save import maintenance requires manual recovery before Panel preparation can continue",
						errors.New("docker lifecycle operations are unsupported"))
				}
				stopCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
				stopErr := stopImportPhaseAServer(stopCtx, lifecycle, instance.DataDir, time.Second)
				cancel()
				return recoveries, d.persistImportManualRecovery(instance.DataDir, journal,
					"save import maintenance requires manual recovery before Panel preparation can continue", stopErr)
			}
		}
		if recovery.State != importRecoveryMaintenanceStopAndRestore && recovery.State != importRecoveryMaintenanceRestoreSnapshot {
			continue
		}
		if d.store == nil {
			return recoveries, maintenanceRollbackError("save import maintenance recovery has no instance state store", nil)
		}
		authoritative, err := d.store.GetInstance(ctx, instance.ID)
		if err != nil {
			return recoveries, maintenanceRollbackError("failed to load authoritative instance during maintenance recovery", err)
		}
		if filepath.Clean(authoritative.DataDir) != filepath.Clean(instance.DataDir) {
			return recoveries, maintenanceRollbackError("instance data directory changed during maintenance recovery", nil)
		}
		journal, err := LoadImportJournal(authoritative.DataDir, recovery.OperationID)
		if err != nil {
			return recoveries, maintenanceRollbackError("failed to reload interrupted maintenance journal", err)
		}
		if journal.PhaseAFIFOWriteAttempted || journal.UpstreamSubmitted || journal.UpstreamConfirmed || importStageAtLeast(journal.Stage, ImportStageSubmitted) {
			return recoveries, d.persistImportManualRecovery(authoritative.DataDir, journal,
				"interrupted maintenance cannot prove that FIFO submission never started", nil)
		}
		lifecycle, ok := d.docker.(LifecycleDockerService)
		if !ok {
			return recoveries, d.persistImportManualRecovery(authoritative.DataDir, journal,
				"interrupted maintenance runtime cannot be stopped safely", errors.New("docker lifecycle operations are unsupported"))
		}
		recoveryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		if recovery.State == importRecoveryMaintenanceStopAndRestore {
			err = d.stopAndRestoreImportMaintenance(recoveryCtx, lifecycle, authoritative.DataDir, journal.OperationID, authoritative)
		} else {
			if probeErr := ensureSaveImportServerStopped(recoveryCtx, lifecycle, authoritative.DataDir); probeErr != nil {
				err = probeErr
			} else {
				err = d.restoreImportMaintenanceSnapshot(authoritative.DataDir, journal.OperationID, authoritative)
			}
		}
		cancel()
		if err != nil {
			return recoveries, d.persistImportManualRecovery(authoritative.DataDir, journal,
				"interrupted save import maintenance requires manual recovery", err)
		}
		recovery.State = "safe_to_resume_or_cleanup"
		recovery.ErrorCode = ""
	}
	return recoveries, nil
}

func (d *Driver) persistImportManualRecovery(dataDir string, journal ImportJournal, message string, cause error) error {
	journal.MaintenanceRecoveryState = importMaintenanceManualRecovery
	journal.RecoveryState = "manual_required"
	journal.LastErrorCode = ImportErrorRecoveryRequired
	journal.LastError = message
	if err := d.writeImportJournal(dataDir, journal); err != nil {
		cause = errors.Join(cause, fmt.Errorf("persist manual save import recovery evidence: %w", err))
	}
	return maintenanceRollbackError(message, cause)
}

func originalInstanceSnapshotFromJournal(j ImportJournal, dataDir string) (storage.Instance, error) {
	state := j.OriginalInstanceState
	message := j.OriginalInstanceStateMessage
	messageValid := j.OriginalInstanceStateMessageValid
	phase := j.OriginalInstanceDriverPhase
	payload := j.OriginalInstanceDriverPayload
	if j.OriginalInstanceSnapshot != nil {
		state = j.OriginalInstanceSnapshot.State
		messageBytes, err := base64.StdEncoding.DecodeString(j.OriginalInstanceSnapshot.StateMessageBase64)
		if err != nil {
			return storage.Instance{}, fmt.Errorf("decode original instance state message: %w", err)
		}
		phaseBytes, err := base64.StdEncoding.DecodeString(j.OriginalInstanceSnapshot.DriverPhaseBase64)
		if err != nil {
			return storage.Instance{}, fmt.Errorf("decode original instance driver phase: %w", err)
		}
		payloadBytes, err := base64.StdEncoding.DecodeString(j.OriginalInstanceSnapshot.DriverPayloadBase64)
		if err != nil {
			return storage.Instance{}, fmt.Errorf("decode original instance driver payload: %w", err)
		}
		message = string(messageBytes)
		messageValid = j.OriginalInstanceSnapshot.StateMessageValid
		phase = string(phaseBytes)
		payload = string(payloadBytes)
	}
	if !IsSaveImportMaintenanceOfflineState(state) {
		return storage.Instance{}, fmt.Errorf("import journal original instance state %q is not restorable", state)
	}
	if strings.TrimSpace(j.InstanceID) == "" {
		return storage.Instance{}, errors.New("import journal original instance identity is missing")
	}
	return storage.Instance{
		ID:            j.InstanceID,
		DataDir:       dataDir,
		State:         state,
		StateMessage:  sql.NullString{String: message, Valid: messageValid},
		DriverPhase:   phase,
		DriverPayload: payload,
	}, nil
}

func maintenanceLog(job *jobs.Context, message string) {
	if job != nil {
		_, _ = job.Info(context.Background(), message)
	}
}
