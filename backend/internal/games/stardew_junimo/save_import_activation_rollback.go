package stardew_junimo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
)

const (
	importActivationRollbackPending          = "pending"
	importActivationRollbackServerStopped    = "server_stopped"
	importActivationRollbackSaveRestored     = "save_restored"
	importActivationRollbackPointerRestored  = "pointer_restored"
	importActivationRollbackInstanceRestored = "instance_restored"
	importActivationRollbackCompleted        = "completed"
	importActivationRollbackManualRecovery   = "manual_recovery"
	importActivationRollbackStopTimeout      = saveImportRuntimeStopTimeout
)

func (d *Driver) rollbackConfirmedHostSwapFailure(
	dataDir,
	operationID string,
	job *jobs.Context,
	primary error,
) error {
	causeCode := ImportErrorRecoveryRequired
	if typed, ok := AsImportTransactionError(primary); ok && typed.Code != "" {
		causeCode = typed.Code
	}

	lifecycle, ok := d.docker.(LifecycleDockerService)
	if !ok || d.store == nil {
		return d.recordImportActivationRollbackFailure(dataDir, operationID, primary, errors.New("activation rollback runtime is unavailable"))
	}

	j, err := LoadImportJournal(dataDir, operationID)
	if err != nil {
		return maintenanceRollbackError("confirmed host swap failed and its journal cannot be loaded", errors.Join(primary, err))
	}
	if j.Stage == ImportStageCompleted || j.Stage == ImportStageCanceled || j.Stage == ImportStageRolledBack {
		return primary
	}
	if j.HostHandling != "swap_host_to" {
		return primary
	}
	if !j.UpstreamConfirmed || j.PreSubmitEvidence == nil || j.PreSubmitEvidence.MainSaveSHA256 == "" {
		return d.recordImportActivationRollbackFailure(dataDir, operationID, primary, errors.New("journal does not prove a rollback-safe host swap"))
	}
	if err := validatePreimportForPhaseA(dataDir, j, j.PreSubmitEvidence.MainSaveSHA256); err != nil {
		return d.recordImportActivationRollbackFailure(dataDir, operationID, primary, fmt.Errorf("validate preimport rollback archive: %w", err))
	}
	original, err := originalInstanceSnapshotFromJournal(j, dataDir)
	if err != nil {
		return d.recordImportActivationRollbackFailure(dataDir, operationID, primary, fmt.Errorf("load original instance snapshot: %w", err))
	}

	j.RollbackState = importActivationRollbackPending
	j.RollbackCauseCode = causeCode
	if err := WriteImportJournal(dataDir, j); err != nil {
		return maintenanceRollbackError("host swap rollback intent could not be persisted", errors.Join(primary, err))
	}

	// Use the same full Docker stop budget as every other import-maintenance
	// recovery path. The scoped stop may legitimately consume Docker's complete
	// grace period before the independent strict proof can classify the runtime.
	rollbackCtx, cancel := context.WithTimeout(context.Background(), importActivationRollbackStopTimeout)
	defer cancel()
	if err := stopImportPhaseAForJournal(rollbackCtx, lifecycle, dataDir, operationID, time.Second); err != nil {
		return d.recordImportActivationRollbackFailure(dataDir, operationID, primary, fmt.Errorf("stop activation runtime: %w", err))
	}
	if err := d.advanceImportActivationRollback(dataDir, operationID, importActivationRollbackServerStopped, "", ""); err != nil {
		return maintenanceRollbackError("stopped activation runtime could not be journaled", errors.Join(primary, err))
	}

	pointer, err := readActivePointerStrict(dataDir)
	if err != nil || pointer != j.SaveName {
		return d.recordImportActivationRollbackFailure(dataDir, operationID, primary, fmt.Errorf("active pointer changed before rollback: got %q: %w", pointer, err))
	}
	restoredHash, err := restorePreimportForPhaseA(dataDir, j, j.PreSubmitEvidence.MainSaveSHA256)
	if err != nil || restoredHash != j.PreSubmitEvidence.MainSaveSHA256 {
		return d.recordImportActivationRollbackFailure(dataDir, operationID, primary, fmt.Errorf("restore preimport save: hash=%q: %w", restoredHash, err))
	}
	restoredFingerprint, err := importDirectoryFingerprint(filepath.Join(savesDir(dataDir), "Saves", j.SaveName))
	if err != nil || restoredFingerprint != j.StagedSaveFingerprint {
		return d.recordImportActivationRollbackFailure(dataDir, operationID, primary, fmt.Errorf("restored save tree fingerprint mismatch: %w", err))
	}
	if err := d.advanceImportActivationRollback(dataDir, operationID, importActivationRollbackSaveRestored, restoredHash, restoredFingerprint); err != nil {
		return maintenanceRollbackError("restored preimport save could not be journaled", errors.Join(primary, err))
	}

	if j.BootstrapSaveName != "" {
		if j.BootstrapSaveName != importBootstrapSaveName(j.OperationID) || !j.BootstrapSaveCreated {
			return d.recordImportActivationRollbackFailure(dataDir, operationID, primary, errors.New("bootstrap rollback ownership is invalid"))
		}
		if err := os.Remove(gameloaderPath(dataDir)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return d.recordImportActivationRollbackFailure(dataDir, operationID, primary, fmt.Errorf("remove bootstrap pointer: %w", err))
		}
		bootstrapDir := filepath.Join(savesDir(dataDir), "Saves", j.BootstrapSaveName)
		if err := os.RemoveAll(bootstrapDir); err != nil {
			return d.recordImportActivationRollbackFailure(dataDir, operationID, primary, fmt.Errorf("remove transaction bootstrap: %w", err))
		}
		if err := os.RemoveAll(importBootstrapSourceRoot(dataDir, j.OperationID)); err != nil {
			return d.recordImportActivationRollbackFailure(dataDir, operationID, primary, fmt.Errorf("remove transaction bootstrap source: %w", err))
		}
	} else {
		if j.OriginalActiveSave == "" {
			return d.recordImportActivationRollbackFailure(dataDir, operationID, primary, errors.New("original active save pointer is missing"))
		}
		if err := writeGameloaderPointer(dataDir, j.OriginalActiveSave); err != nil {
			return d.recordImportActivationRollbackFailure(dataDir, operationID, primary, fmt.Errorf("restore active save pointer: %w", err))
		}
		if err := ApplyModProfile(dataDir, j.OriginalActiveSave); err != nil {
			return d.recordImportActivationRollbackFailure(dataDir, operationID, primary, fmt.Errorf("restore original Mod profile: %w", err))
		}
	}
	if err := d.advanceImportActivationRollback(dataDir, operationID, importActivationRollbackPointerRestored, restoredHash, restoredFingerprint); err != nil {
		return maintenanceRollbackError("restored active pointer could not be journaled", errors.Join(primary, err))
	}

	if err := restoreInstanceState(d, original); err != nil {
		return d.recordImportActivationRollbackFailure(dataDir, operationID, primary, fmt.Errorf("restore exact instance snapshot: %w", err))
	}
	if err := d.advanceImportActivationRollback(dataDir, operationID, importActivationRollbackInstanceRestored, restoredHash, restoredFingerprint); err != nil {
		return maintenanceRollbackError("restored instance snapshot could not be journaled", errors.Join(primary, err))
	}

	j, err = LoadImportJournal(dataDir, operationID)
	if err != nil {
		return maintenanceRollbackError("completed activation rollback journal cannot be loaded", errors.Join(primary, err))
	}
	now := time.Now().UTC()
	j.Stage = ImportStageRolledBack
	j.MaintenanceStarted = false
	j.RollbackState = importActivationRollbackCompleted
	j.RollbackRestoredSHA256 = restoredHash
	j.RollbackRestoredFingerprint = restoredFingerprint
	j.RolledBackAt = &now
	j.RecoveryState = "rolled_back"
	j.LastErrorCode = causeCode
	j.LastError = fmt.Sprintf("confirmed host swap was rolled back after post-import failure: %s", primary.Error())
	if err := WriteImportJournal(dataDir, j); err != nil {
		return maintenanceRollbackError("completed activation rollback could not be persisted", errors.Join(primary, err))
	}
	maintenanceLog(job, "Confirmed host swap failed; the complete preimport save, active pointer, Mod profile, and instance snapshot were restored.")
	return primary
}

func (d *Driver) advanceImportActivationRollback(dataDir, operationID, state, restoredHash, restoredFingerprint string) error {
	j, err := LoadImportJournal(dataDir, operationID)
	if err != nil {
		return err
	}
	j.RollbackState = state
	if restoredHash != "" {
		j.RollbackRestoredSHA256 = restoredHash
	}
	if restoredFingerprint != "" {
		j.RollbackRestoredFingerprint = restoredFingerprint
	}
	return WriteImportJournal(dataDir, j)
}

func (d *Driver) recordImportActivationRollbackFailure(dataDir, operationID string, primary, rollbackErr error) error {
	if j, err := LoadImportJournal(dataDir, operationID); err == nil {
		j.RollbackState = importActivationRollbackManualRecovery
		j.RecoveryState = "manual_required"
		j.LastErrorCode = ImportErrorRecoveryRequired
		j.LastError = "confirmed host swap failed and automatic preimport rollback could not be completed"
		_ = WriteImportJournal(dataDir, j)
	}
	return maintenanceRollbackError("confirmed host swap failed and automatic rollback requires manual recovery", errors.Join(primary, rollbackErr))
}
