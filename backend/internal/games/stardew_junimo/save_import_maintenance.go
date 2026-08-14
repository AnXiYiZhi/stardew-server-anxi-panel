package stardew_junimo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
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
		return storage.Instance{}, &ImportTransactionError{Code: ImportErrorSaveInProgress, Message: "instance state does not allow save import maintenance startup"}
	}
	lifecycle, ok := d.docker.(LifecycleDockerService)
	if !ok {
		return storage.Instance{}, &ImportTransactionError{Code: ImportErrorMaintenanceStart, Message: "maintenance runtime is unavailable", Cause: errors.New("docker lifecycle operations are unsupported")}
	}
	if err := ensureSaveImportServerStopped(ctx, lifecycle, original.DataDir); err != nil {
		return storage.Instance{}, err
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
		switch strings.ToLower(strings.TrimSpace(service.State)) {
		case "exited", "dead", "created":
			continue
		case "running", "restarting", "paused", "removing":
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
	lifecycle, ok := d.docker.(LifecycleDockerService)
	if !ok {
		return d.recordMaintenanceFailure(instance.DataDir, operationID, ImportErrorMaintenanceStart, "maintenance runtime is unavailable", errors.New("docker lifecycle operations are unsupported"))
	}
	if options.ReadyTimeout <= 0 {
		options.ReadyTimeout = 5 * time.Minute
	}
	if options.PollInterval <= 0 {
		options.PollInterval = time.Second
	}

	original, err := d.inspectSaveImportMaintenanceOffline(ctx, instance)
	if err != nil {
		if typed, ok := AsImportTransactionError(err); ok {
			return d.recordMaintenanceFailure(instance.DataDir, operationID, typed.Code, typed.Message, typed.Cause)
		}
		return d.recordMaintenanceFailure(instance.DataDir, operationID, ImportErrorMaintenanceStart, "save import maintenance preflight failed", err)
	}
	dataDir := original.DataDir
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

	// Mark the runtime as potentially started before ComposeUp. A partial Docker
	// failure must remain non-cleanable until ComposeDown plus ComposePs proves
	// the server is stopped and the deferred path clears this flag.
	journal.MaintenanceStarted = true
	journal.OriginalInstanceState = original.State
	journal.OriginalInstanceStateMessage = original.StateMessage.String
	journal.OriginalInstanceStateMessageValid = original.StateMessage.Valid
	journal.OriginalInstanceDriverPhase = original.DriverPhase
	journal.OriginalInstanceDriverPayload = original.DriverPayload
	if err := WriteImportJournal(dataDir, journal); err != nil {
		return err
	}
	defer func() {
		if retErr == nil {
			return
		}
		stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		down, downErr := lifecycle.ComposeDown(stopCtx, dataDir)
		if downErr != nil || down.ExitCode != 0 {
			retErr = joinMaintenanceRollback(retErr, "failed to stop save import maintenance runtime", downErr)
			return
		}
		if err := ensureSaveImportServerStopped(stopCtx, lifecycle, dataDir); err != nil {
			retErr = joinMaintenanceRollback(retErr, "failed to prove save import maintenance runtime stopped", err)
			return
		}
		failedJournal, loadErr := LoadImportJournal(dataDir, operationID)
		if loadErr != nil {
			retErr = joinMaintenanceRollback(retErr, "failed to reload save import journal during rollback", loadErr)
			return
		}
		failedJournal.MaintenanceStarted = false
		if writeErr := WriteImportJournal(dataDir, failedJournal); writeErr != nil {
			retErr = joinMaintenanceRollback(retErr, "failed to persist stopped save import maintenance state", writeErr)
			return
		}
		if restoreErr := restoreInstanceState(d, original); restoreErr != nil {
			retErr = joinMaintenanceRollback(retErr, "failed to restore instance state after save import maintenance", restoreErr)
		}
	}()
	if err := d.updateImportMaintenancePhase(ctx, "Save import maintenance runtime is starting; the server is not join-ready.", original); err != nil {
		return d.recordMaintenanceFailure(dataDir, operationID, ImportErrorMaintenanceStart, "failed to publish save import maintenance state", err)
	}
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
	journal.LastErrorCode, journal.LastError = "", ""
	if err := WriteImportJournal(dataDir, journal); err != nil {
		return err
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

func (d *Driver) recordMaintenanceFailure(dataDir, operationID, code, message string, cause error) error {
	journal, err := LoadImportJournal(dataDir, operationID)
	if err == nil {
		journal.LastErrorCode, journal.LastError = code, message
		if writeErr := WriteImportJournal(dataDir, journal); writeErr != nil {
			cause = errors.Join(cause, fmt.Errorf("persist save import failure: %w", writeErr))
		}
	} else {
		cause = errors.Join(cause, fmt.Errorf("load save import journal while recording failure: %w", err))
	}
	return &ImportTransactionError{Code: code, Message: message, Cause: cause}
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

func originalInstanceSnapshotFromJournal(j ImportJournal, dataDir string) (storage.Instance, error) {
	if !IsSaveImportMaintenanceOfflineState(j.OriginalInstanceState) {
		return storage.Instance{}, fmt.Errorf("import journal original instance state %q is not restorable", j.OriginalInstanceState)
	}
	if strings.TrimSpace(j.InstanceID) == "" {
		return storage.Instance{}, errors.New("import journal original instance identity is missing")
	}
	return storage.Instance{
		ID:            j.InstanceID,
		DataDir:       dataDir,
		State:         j.OriginalInstanceState,
		StateMessage:  sql.NullString{String: j.OriginalInstanceStateMessage, Valid: j.OriginalInstanceStateMessageValid},
		DriverPhase:   j.OriginalInstanceDriverPhase,
		DriverPayload: j.OriginalInstanceDriverPayload,
	}, nil
}

func maintenanceLog(job *jobs.Context, message string) {
	if job != nil {
		_, _ = job.Info(context.Background(), message)
	}
}
