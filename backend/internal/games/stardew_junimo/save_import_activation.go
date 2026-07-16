package stardew_junimo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
)

const (
	activationOutcomeSwapFinalized = "swap_finalized"
	activationOutcomeAsIsLoaded    = "as_is_loaded"
	activationOutcomeTimeout       = "activation_timeout"
	activationOutcomeRecovery      = "recovery_required"
	activationOutcomeUnconfirmed   = "result_unconfirmed"
)

type JunimoRuntimeWorldState struct {
	IsOnline              *bool  `json:"isOnline,omitempty"`
	IsReady               *bool  `json:"isReady,omitempty"`
	DayTransitionComplete *bool  `json:"dayTransitionComplete,omitempty"`
	PlayerCount           *int   `json:"playerCount,omitempty"`
	Version               *int64 `json:"version,omitempty"`
}

// JunimoImportActivationEvidence is deliberately read-only. PendingIntent's
// raw UserID is excluded by its json:"-" tag and is never persisted here.
type JunimoImportActivationEvidence struct {
	RuntimeSaveID            string                  `json:"runtimeSaveId,omitempty"`
	RuntimeSaveIDErrorCode   string                  `json:"runtimeSaveIdErrorCode,omitempty"`
	PendingIntent            JunimoSaveImportIntent  `json:"pendingIntent"`
	ProcessIdentity          *JunimoProcessIdentity  `json:"processIdentity,omitempty"`
	ProcessIdentityErrorCode string                  `json:"processIdentityErrorCode,omitempty"`
	FinalizeCount            *int                    `json:"finalizeCount,omitempty"`
	MasterName               *string                 `json:"masterName,omitempty"`
	DiagnosticsErrorCode     string                  `json:"diagnosticsErrorCode,omitempty"`
	World                    JunimoRuntimeWorldState `json:"world"`
	StatusErrorCode          string                  `json:"statusErrorCode,omitempty"`
	CapturedAt               time.Time               `json:"capturedAt"`
}

type JunimoActivationProcessBaseline struct {
	ProcessIdentity *JunimoProcessIdentity `json:"processIdentity"`
	FinalizeCount   *int                   `json:"finalizeCount,omitempty"`
}

type importActivationOptions struct {
	ReloadTimeout  time.Duration
	RestartTimeout time.Duration
	PollInterval   time.Duration
}

func defaultImportActivationOptions() importActivationOptions {
	return importActivationOptions{ReloadTimeout: 90 * time.Second, RestartTimeout: 5 * time.Minute, PollInterval: time.Second}
}

func readRuntimeWorldState(ctx context.Context, exec commandExecutor, dataDir string) (JunimoRuntimeWorldState, error) {
	raw, err := readJunimoAPI(ctx, exec, dataDir, "/status")
	if err != nil {
		return JunimoRuntimeWorldState{}, err
	}
	var state JunimoRuntimeWorldState
	if err := json.Unmarshal(raw, &state); err != nil {
		return state, &ImportEvidenceError{Code: "status_invalid_json", Message: "Junimo status JSON is invalid", Cause: err}
	}
	if state.IsOnline == nil || state.DayTransitionComplete == nil || state.PlayerCount == nil {
		return state, &ImportEvidenceError{Code: "status_field_missing", Message: "Junimo status response is missing activation evidence fields"}
	}
	return state, nil
}

func captureImportActivationEvidence(ctx context.Context, exec commandExecutor, dataDir string) (JunimoImportActivationEvidence, error) {
	evidence := JunimoImportActivationEvidence{CapturedAt: time.Now().UTC()}
	intent, err := ReadJunimoSaveImportIntent(dataDir)
	if err != nil {
		return evidence, err
	}
	evidence.PendingIntent = intent
	evidence.RuntimeSaveID, err = readRuntimeSaveID(dataDir)
	if err != nil {
		evidence.RuntimeSaveIDErrorCode = "runtime_save_id_unavailable"
	}
	evidence.ProcessIdentity, err = readProcessIdentity(ctx, exec, dataDir)
	evidence.ProcessIdentityErrorCode = activationEvidenceErrorCode(err)
	diagnostics, diagnosticsErr := ReadJunimoDiagnosticsState(ctx, exec, dataDir)
	evidence.FinalizeCount, evidence.MasterName = diagnostics.FinalizeCount, diagnostics.MasterName
	evidence.DiagnosticsErrorCode = activationEvidenceErrorCode(diagnosticsErr)
	evidence.World, err = readRuntimeWorldState(ctx, exec, dataDir)
	evidence.StatusErrorCode = activationEvidenceErrorCode(err)
	return evidence, nil
}

func activationEvidenceErrorCode(err error) string {
	if err == nil {
		return ""
	}
	var evidenceErr *ImportEvidenceError
	if errors.As(err, &evidenceErr) {
		return evidenceErr.Code
	}
	return "evidence_unavailable"
}

func processIdentityEqual(a, b *JunimoProcessIdentity) bool {
	return a != nil && b != nil && *a == *b
}

func activationWorldStable(e JunimoImportActivationEvidence) bool {
	return e.StatusErrorCode == "" && e.World.IsOnline != nil && *e.World.IsOnline &&
		e.World.DayTransitionComplete != nil && *e.World.DayTransitionComplete
}

func activationCountAdvanced(j ImportJournal, e JunimoImportActivationEvidence) (bool, bool) {
	if e.ProcessIdentity == nil || e.FinalizeCount == nil {
		return false, false
	}
	if j.PreSubmitEvidence != nil && j.PreSubmitEvidence.FinalizeCount != nil &&
		processIdentityEqual(e.ProcessIdentity, j.PreSubmitEvidence.ProcessIdentity) {
		return *e.FinalizeCount == *j.PreSubmitEvidence.FinalizeCount+1, true
	}
	// Maintenance readiness is captured before submission in the same process.
	// It remains a valid generation baseline when a later diagnostics sample is
	// transiently incomplete; missing data must not be replaced with zero.
	if j.RuntimeBaseline != nil && j.RuntimeBaseline.FinalizeCount != nil &&
		processIdentityEqual(e.ProcessIdentity, j.RuntimeBaseline.ProcessIdentity) {
		return *e.FinalizeCount == *j.RuntimeBaseline.FinalizeCount+1, true
	}
	if baseline := j.ActivationProcessBaseline; baseline != nil &&
		processIdentityEqual(e.ProcessIdentity, baseline.ProcessIdentity) && baseline.FinalizeCount != nil {
		return *e.FinalizeCount == *baseline.FinalizeCount+1, true
	}
	// The new process may load and finalize the target before its first API
	// snapshot can be captured. The upstream counter is process-local, so one
	// successful finalization in that new generation is exactly 1.
	return *e.FinalizeCount == 1, true
}

type activationDecision int

const (
	activationWait activationDecision = iota
	activationSuccess
	activationRecovery
	activationUnconfirmed
)

func evaluateImportActivation(j ImportJournal, e JunimoImportActivationEvidence, final bool) activationDecision {
	targetLoaded := e.RuntimeSaveID == j.SaveName
	worldStable := activationWorldStable(e)
	if final && e.RuntimeSaveIDErrorCode != "" {
		return activationUnconfirmed
	}
	if j.HostHandling == "server_owns_original" {
		if targetLoaded && !e.PendingIntent.Exists && worldStable {
			return activationSuccess
		}
		if final && !e.PendingIntent.Exists && e.RuntimeSaveID != "" && !targetLoaded {
			if !j.ActivationRestarted {
				return activationWait
			}
			return activationRecovery
		}
		return activationWait
	}
	if targetLoaded && !e.PendingIntent.Exists {
		if e.DiagnosticsErrorCode != "" || e.StatusErrorCode != "" {
			if final {
				return activationUnconfirmed
			}
			return activationWait
		}
		advanced, known := activationCountAdvanced(j, e)
		if known && !advanced && worldStable {
			return activationRecovery
		}
		if advanced && e.MasterName != nil && *e.MasterName == "Server" && worldStable {
			return activationSuccess
		}
		if final {
			return activationUnconfirmed
		}
	}
	if final && !e.PendingIntent.Exists && e.RuntimeSaveID != "" && !targetLoaded {
		return activationRecovery
	}
	return activationWait
}

func (d *Driver) runImportActivation(ctx context.Context, instance registry.Instance, operationID string, job *jobs.Context, options importActivationOptions) error {
	if options.ReloadTimeout <= 0 {
		options.ReloadTimeout = 90 * time.Second
	}
	if options.RestartTimeout <= 0 {
		options.RestartTimeout = 5 * time.Minute
	}
	if options.PollInterval <= 0 {
		options.PollInterval = time.Second
	}
	lifecycle, ok := d.docker.(LifecycleDockerService)
	if !ok {
		return recordImportActivationFailure(instance.DataDir, operationID, ImportErrorResultUnconfirmed, activationOutcomeUnconfirmed, "save activation runtime is unavailable", nil, nil)
	}
	j, err := LoadImportJournal(instance.DataDir, operationID)
	if err != nil {
		return err
	}
	if (j.Stage != ImportStageConfirmed && j.Stage != ImportStageSaveActivating) || !j.UpstreamSubmitted || !j.UpstreamConfirmed || j.PreSubmitEvidence == nil {
		return recordImportActivationFailure(instance.DataDir, operationID, ImportErrorRecoveryRequired, activationOutcomeRecovery, "save activation requires confirmed Phase A evidence", nil, nil)
	}
	maintenanceLog(job, "Phase A confirmed; waiting for the target runtime saveId and finalizer composite evidence.")

	if j.MaintenanceStarted {
		if evidence, decision, waitErr := waitForImportActivation(ctx, lifecycle, instance.DataDir, operationID, options.ReloadTimeout, options.PollInterval); decision != activationWait || waitErr != nil {
			if decision != activationWait {
				return finishImportActivation(instance.DataDir, operationID, evidence, decision, job)
			}
			if !errors.Is(waitErr, context.DeadlineExceeded) {
				return recordImportActivationFailure(instance.DataDir, operationID, ImportErrorResultUnconfirmed, activationOutcomeUnconfirmed, "save activation observation was interrupted", &evidence, waitErr)
			}
			if j.ActivationRestarted {
				return classifyFinalImportActivation(instance.DataDir, operationID, evidence, waitErr)
			}
		}
	}

	// A restart is destructive to connected clients. If player state cannot be
	// read, remain unconfirmed instead of assuming that nobody is connected.
	if err := rejectConnectedFarmhands(ctx, lifecycle, instance.DataDir); err != nil {
		if typed, ok := AsImportTransactionError(err); ok && typed.Code == ImportErrorPlayersConnected {
			return recordImportActivationFailure(instance.DataDir, operationID, typed.Code, activationOutcomeUnconfirmed, typed.Message, nil, typed.Cause)
		}
		j, _ = LoadImportJournal(instance.DataDir, operationID)
		if j.MaintenanceStarted {
			return recordImportActivationFailure(instance.DataDir, operationID, ImportErrorResultUnconfirmed, activationOutcomeUnconfirmed, "player connection state is unavailable; controlled restart was not attempted", nil, err)
		}
	}
	if err := ApplyModProfile(instance.DataDir, j.SaveName); err != nil {
		return recordImportActivationFailure(instance.DataDir, operationID, ImportErrorRecoveryRequired, activationOutcomeRecovery, "target save Mod profile could not be applied before activation", nil, err)
	}
	j, err = LoadImportJournal(instance.DataDir, operationID)
	if err != nil {
		return err
	}
	j.Stage = ImportStageSaveActivating
	j.ActivationRestarted = true
	j.LastErrorCode, j.LastError = "", ""
	if err := WriteImportJournal(instance.DataDir, j); err != nil {
		return err
	}
	maintenanceLog(job, "Reload did not establish the target world; performing the single controlled activation restart without re-sending import.")

	ps, psErr := lifecycle.ComposePs(ctx, instance.DataDir)
	running := psErr == nil && serverServiceUp(ps.Services)
	var resultErr error
	if running {
		result, restartErr := lifecycle.ComposeRestartServices(ctx, instance.DataDir, "server")
		if restartErr != nil || result.ExitCode != 0 {
			resultErr = restartErr
			if resultErr == nil {
				resultErr = fmt.Errorf("compose restart exited with code %d", result.ExitCode)
			}
		}
	} else {
		result, upErr := lifecycle.ComposeUp(ctx, instance.DataDir)
		if upErr != nil || result.ExitCode != 0 {
			resultErr = upErr
			if resultErr == nil {
				resultErr = fmt.Errorf("compose up exited with code %d", result.ExitCode)
			}
		}
	}
	if resultErr != nil {
		return recordImportActivationFailure(instance.DataDir, operationID, ImportErrorActivationTimeout, activationOutcomeTimeout, "controlled save activation restart failed", nil, resultErr)
	}
	j.MaintenanceStarted = true
	if err := WriteImportJournal(instance.DataDir, j); err != nil {
		return &ImportTransactionError{Code: ImportErrorResultUnconfirmed, Message: "activation restart completed but its runtime state could not be journaled", Cause: err}
	}

	readyCtx, cancel := context.WithTimeout(ctx, options.RestartTimeout)
	defer cancel()
	if err := waitForImportServerContainer(readyCtx, lifecycle, instance.DataDir, options.PollInterval); err != nil {
		return recordImportActivationFailure(instance.DataDir, operationID, ImportErrorActivationTimeout, activationOutcomeTimeout, "server did not start for save activation", nil, err)
	}
	if err := waitForImportRuntimeProbes(readyCtx, lifecycle, instance.DataDir, options.PollInterval); err != nil {
		return recordImportActivationFailure(instance.DataDir, operationID, ImportErrorResultUnconfirmed, activationOutcomeUnconfirmed, "activation runtime API did not become readable", nil, err)
	}
	first, captureErr := captureImportActivationEvidence(readyCtx, lifecycle, instance.DataDir)
	if captureErr == nil && first.ProcessIdentity != nil && first.FinalizeCount != nil && first.RuntimeSaveID != j.SaveName {
		j, _ = LoadImportJournal(instance.DataDir, operationID)
		count := *first.FinalizeCount
		identity := *first.ProcessIdentity
		j.ActivationProcessBaseline = &JunimoActivationProcessBaseline{ProcessIdentity: &identity, FinalizeCount: &count}
		j.ActivationEvidence = &first
		if err := WriteImportJournal(instance.DataDir, j); err != nil {
			return recordImportActivationFailure(instance.DataDir, operationID, ImportErrorResultUnconfirmed, activationOutcomeUnconfirmed, "new process activation baseline could not be journaled", &first, err)
		}
	}
	evidence, decision, waitErr := waitForImportActivation(readyCtx, lifecycle, instance.DataDir, operationID, options.RestartTimeout, options.PollInterval)
	if decision != activationWait {
		return finishImportActivation(instance.DataDir, operationID, evidence, decision, job)
	}
	if errors.Is(waitErr, context.Canceled) && errors.Is(ctx.Err(), context.Canceled) {
		return recordImportActivationFailure(instance.DataDir, operationID, ImportErrorResultUnconfirmed, activationOutcomeUnconfirmed, "save activation observation was canceled", &evidence, ctx.Err())
	}
	return classifyFinalImportActivation(instance.DataDir, operationID, evidence, waitErr)
}

func waitForImportActivation(ctx context.Context, lifecycle LifecycleDockerService, dataDir, operationID string, timeout, interval time.Duration) (JunimoImportActivationEvidence, activationDecision, error) {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	var last JunimoImportActivationEvidence
	for {
		j, err := LoadImportJournal(dataDir, operationID)
		if err != nil {
			return last, activationWait, err
		}
		if evidence, captureErr := captureImportActivationEvidence(ctx, lifecycle, dataDir); captureErr == nil {
			// Do not replace the last coherent snapshot with a partially canceled
			// capture at the outer activation deadline.
			if ctx.Err() == nil {
				last = evidence
				j.ActivationEvidence = &evidence
				_ = WriteImportJournal(dataDir, j)
				if decision := evaluateImportActivation(j, evidence, false); decision != activationWait {
					return evidence, decision, nil
				}
			}
		}
		select {
		case <-ctx.Done():
			return last, activationWait, ctx.Err()
		case <-deadline.C:
			return last, evaluateImportActivation(j, last, true), context.DeadlineExceeded
		case <-ticker.C:
		}
	}
}

func finishImportActivation(dataDir, operationID string, evidence JunimoImportActivationEvidence, decision activationDecision, job *jobs.Context) error {
	if decision == activationSuccess {
		j, err := LoadImportJournal(dataDir, operationID)
		if err != nil {
			return err
		}
		j.ActivationEvidence = &evidence
		j.Stage = ImportStageFinalizeConfirmed
		if j.HostHandling == "server_owns_original" {
			j.ActivationOutcome = activationOutcomeAsIsLoaded
		} else {
			j.ActivationOutcome = activationOutcomeSwapFinalized
		}
		j.LastErrorCode, j.LastError = "", ""
		if err := WriteImportJournal(dataDir, j); err != nil {
			return err
		}
		maintenanceLog(job, "Target runtime saveId and composite activation evidence verified; no import command was re-sent.")
		return nil
	}
	return classifyFinalImportActivation(dataDir, operationID, evidence, nil)
}

func classifyFinalImportActivation(dataDir, operationID string, evidence JunimoImportActivationEvidence, cause error) error {
	j, err := LoadImportJournal(dataDir, operationID)
	if err != nil {
		return err
	}
	decision := evaluateImportActivation(j, evidence, true)
	if decision == activationRecovery {
		return recordImportActivationFailure(dataDir, operationID, ImportErrorRecoveryRequired, activationOutcomeRecovery, "save activation evidence indicates a partial or failed finalizer; import must not be repeated", &evidence, cause)
	}
	if decision == activationUnconfirmed || evidence.RuntimeSaveIDErrorCode != "" || evidence.DiagnosticsErrorCode != "" || evidence.StatusErrorCode != "" {
		return recordImportActivationFailure(dataDir, operationID, ImportErrorResultUnconfirmed, activationOutcomeUnconfirmed, "save activation result is unconfirmed because required runtime evidence is unavailable", &evidence, cause)
	}
	return recordImportActivationFailure(dataDir, operationID, ImportErrorActivationTimeout, activationOutcomeTimeout, "target save or finalizer did not become ready before the activation deadline", &evidence, cause)
}

func recordImportActivationFailure(dataDir, operationID, code, outcome, message string, evidence *JunimoImportActivationEvidence, cause error) error {
	if j, err := LoadImportJournal(dataDir, operationID); err == nil {
		j.ActivationEvidence = evidence
		j.ActivationOutcome = outcome
		j.LastErrorCode, j.LastError = code, message
		_ = WriteImportJournal(dataDir, j)
	}
	return &ImportTransactionError{Code: code, Message: strings.TrimSpace(message), Cause: cause}
}
