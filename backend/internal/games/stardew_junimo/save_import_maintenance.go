package stardew_junimo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

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

	original, err := d.store.GetInstance(ctx, instance.ID)
	if err != nil {
		return d.recordMaintenanceFailure(instance.DataDir, operationID, ImportErrorMaintenanceStart, "failed to load stopped instance state", err)
	}
	if original.State != storage.InstanceStateStopped {
		return d.recordMaintenanceFailure(instance.DataDir, operationID, ImportErrorMaintenanceStart, "instance must remain stopped before maintenance startup", nil)
	}
	journal, err := LoadImportJournal(instance.DataDir, operationID)
	if err != nil {
		return err
	}
	if !importStageAtLeast(journal.Stage, ImportStageBackupCreated) || journal.PreimportBackupName == "" {
		return d.recordMaintenanceFailure(instance.DataDir, operationID, ImportErrorMaintenanceStart, "staging and preimport backup are incomplete", nil)
	}
	if err := validateImportStaticCapability(instance.DataDir); err != nil {
		return d.recordMaintenanceFailure(instance.DataDir, operationID, ImportErrorUnsupported, "Junimo .125 static capability check failed", err)
	}
	pointerBefore, pointerErr := readActivePointerStrict(instance.DataDir)
	if pointerErr != nil || journal.OriginalActiveSave == "" || pointerBefore != journal.OriginalActiveSave {
		return d.recordMaintenanceFailure(instance.DataDir, operationID, ImportErrorMaintenanceReady,
			"active save pointer is unavailable or changed before maintenance startup", pointerErr)
	}

	ps, err := lifecycle.ComposePs(ctx, instance.DataDir)
	if err != nil {
		return d.recordMaintenanceFailure(instance.DataDir, operationID, ImportErrorMaintenanceStart, "failed to inspect the stopped runtime", err)
	}
	if serverServiceUp(ps.Services) {
		return d.recordMaintenanceFailure(instance.DataDir, operationID, ImportErrorMaintenanceStart, "server container was already running before maintenance startup", nil)
	}

	d.updateImportMaintenancePhase("Save import maintenance runtime is starting; the server is not join-ready.", original)
	maintenanceLog(job, "Starting save_import_maintenance runtime without publishing an invite code.")
	startedByJob := true
	defer func() {
		if retErr == nil || !startedByJob {
			return
		}
		stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, _ = lifecycle.ComposeDown(stopCtx, instance.DataDir)
		if failedJournal, err := LoadImportJournal(instance.DataDir, operationID); err == nil {
			failedJournal.MaintenanceStarted = false
			_ = WriteImportJournal(instance.DataDir, failedJournal)
		}
		restoreInstanceState(d, original)
	}()

	up, err := lifecycle.ComposeUp(ctx, instance.DataDir)
	if err != nil || up.ExitCode != 0 {
		if errors.Is(ctx.Err(), context.Canceled) {
			return d.recordMaintenanceFailure(instance.DataDir, operationID, ImportErrorMaintenanceCancel, "save import maintenance was canceled", ctx.Err())
		}
		if err == nil {
			err = fmt.Errorf("compose up exited with code %d", up.ExitCode)
		}
		return d.recordMaintenanceFailure(instance.DataDir, operationID, ImportErrorMaintenanceStart, "maintenance container startup failed", err)
	}
	journal.MaintenanceStarted = true
	if err := WriteImportJournal(instance.DataDir, journal); err != nil {
		return err
	}

	readyCtx, cancel := context.WithTimeout(ctx, options.ReadyTimeout)
	defer cancel()
	if err := waitForImportServerContainer(readyCtx, lifecycle, instance.DataDir, options.PollInterval); err != nil {
		return d.maintenanceReadinessFailure(instance.DataDir, operationID, err)
	}
	maintenanceLog(job, "Server container is running; checking FIFO, log, API, Junimo version, and saves command.")
	if err := waitForImportRuntimeProbes(readyCtx, lifecycle, instance.DataDir, options.PollInterval); err != nil {
		return d.maintenanceReadinessFailure(instance.DataDir, operationID, err)
	}

	identityBefore, err := readProcessIdentity(readyCtx, lifecycle, instance.DataDir)
	if err != nil {
		return d.maintenanceReadinessFailure(instance.DataDir, operationID, err)
	}
	if err := waitForImportSavesCommand(readyCtx, lifecycle, instance.DataDir, options.PollInterval); err != nil {
		return d.maintenanceReadinessFailure(instance.DataDir, operationID, err)
	}
	if err := rejectConnectedFarmhands(readyCtx, lifecycle, instance.DataDir); err != nil {
		return d.maintenanceReadinessFailure(instance.DataDir, operationID, err)
	}

	offset, err := strictServerLogSize(readyCtx, lifecycle, instance.DataDir)
	if err != nil {
		return d.maintenanceReadinessFailure(instance.DataDir, operationID, err)
	}
	snapshot, err := waitForImportEvidenceBaseline(readyCtx, lifecycle, instance.DataDir, journal.SaveName,
		journal.OriginalActiveSave, *identityBefore, options.PollInterval)
	if err != nil {
		return d.maintenanceReadinessFailure(instance.DataDir, operationID, err)
	}

	journal, err = LoadImportJournal(instance.DataDir, operationID)
	if err != nil {
		return err
	}
	journal.RuntimeBaseline = &snapshot
	journal.ServerOutputLogOffset = &offset
	journal.Stage = ImportStageRuntimeReady
	journal.LastErrorCode, journal.LastError = "", ""
	if err := WriteImportJournal(instance.DataDir, journal); err != nil {
		return err
	}
	d.updateImportMaintenancePhase("Save import maintenance runtime is ready; the server is not join-ready.", original)
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
		if typed, ok := AsImportTransactionError(lastErr); ok && typed.Code == ImportErrorMaintenanceMod {
			return typed
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
	return verifyRunningJunimoVersion(ctx, lifecycle, dataDir)
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
	if journal, err := LoadImportJournal(dataDir, operationID); err == nil {
		journal.LastErrorCode, journal.LastError = code, message
		_ = WriteImportJournal(dataDir, journal)
	}
	return &ImportTransactionError{Code: code, Message: message, Cause: cause}
}

func restoreInstanceState(d *Driver, original storage.Instance) {
	if d.store == nil {
		return
	}
	message := ""
	if original.StateMessage.Valid {
		message = original.StateMessage.String
	}
	_, _ = d.store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID: original.ID, State: original.State, StateMessage: message,
		DriverPhase: original.DriverPhase, DriverPayload: original.DriverPayload,
	})
}

func (d *Driver) updateImportMaintenancePhase(message string, original storage.Instance) {
	if d.store == nil {
		return
	}
	payload := map[string]any{}
	if strings.TrimSpace(original.DriverPayload) != "" {
		_ = json.Unmarshal([]byte(original.DriverPayload), &payload)
	}
	delete(payload, "invite_code")
	encoded, err := json.Marshal(payload)
	if err != nil {
		encoded = []byte("{}")
	}
	_, _ = d.store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID: original.ID, State: storage.InstanceStateStopped, StateMessage: message,
		DriverPhase: importMaintenancePhase, DriverPayload: string(encoded),
	})
}

func maintenanceLog(job *jobs.Context, message string) {
	if job != nil {
		_, _ = job.Info(context.Background(), message)
	}
}
