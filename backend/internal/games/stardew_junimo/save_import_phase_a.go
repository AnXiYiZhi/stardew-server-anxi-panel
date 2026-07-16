package stardew_junimo

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
)

const (
	phaseAOutcomeConfirmedSwap     = "swap_confirmed"
	phaseAOutcomeConfirmedAsIs     = "as_is_confirmed"
	phaseAOutcomeNoEffect          = "command_failed_no_effect"
	phaseAOutcomeRecoveryRequired  = "recovery_required"
	phaseAOutcomeHalfRestored      = "half_conversion_restored"
	phaseAOutcomeHalfRestoreFailed = "half_conversion_restore_failed"
	phaseAOutcomeResultUnconfirmed = "result_unconfirmed"
	phaseAOutcomeFIFOWriteFailed   = "fifo_write_failed"
)

type importPhaseAOptions struct {
	ObservationTimeout time.Duration
	PollInterval       time.Duration
	StopTimeout        time.Duration
}

func defaultImportPhaseAOptions() importPhaseAOptions {
	return importPhaseAOptions{
		ObservationTimeout: 30 * time.Second,
		PollInterval:       250 * time.Millisecond,
		StopTimeout:        30 * time.Second,
	}
}

type phaseAClassification int

const (
	phaseAContradictory phaseAClassification = iota
	phaseAConfirmedSwap
	phaseAConfirmedAsIs
	phaseANoEffect
	phaseARecoveryRequired
	phaseAHalfConversion
)

func (d *Driver) runImportPhaseA(ctx context.Context, instance registry.Instance, operationID, platformID string, job *jobs.Context, options importPhaseAOptions) error {
	if options.ObservationTimeout <= 0 {
		options.ObservationTimeout = 30 * time.Second
	}
	if options.PollInterval <= 0 {
		options.PollInterval = 250 * time.Millisecond
	}
	if options.StopTimeout <= 0 {
		options.StopTimeout = 30 * time.Second
	}
	lifecycle, ok := d.docker.(LifecycleDockerService)
	if !ok {
		return &ImportTransactionError{Code: ImportErrorRecoveryRequired, Message: "save import Phase A runtime is unavailable"}
	}

	journal, pre, offset, command, err := prepareImportPhaseASubmission(ctx, lifecycle, instance.DataDir, operationID, platformID)
	if err != nil {
		return err
	}
	journal.PreSubmitEvidence = &pre
	journal.PreSubmitLogOffset = &offset
	journal.LastErrorCode, journal.LastError, journal.PhaseALogDetail = "", "", ""
	if err := WriteImportJournal(instance.DataDir, journal); err != nil {
		return err
	}
	maintenanceLog(job, "Phase A pre-submit evidence and server-output offset captured; writing one import command to FIFO.")

	writeResult, writeErr := lifecycle.ComposeExecPipe(ctx, instance.DataDir, "server", command+"\n", "tee", "-a", serverInputFIFO)
	if writeErr != nil || writeResult.ExitCode != 0 {
		if writeErr == nil {
			writeErr = fmt.Errorf("FIFO writer exited with code %d", writeResult.ExitCode)
		}
		stopCtx, cancel := context.WithTimeout(context.Background(), options.StopTimeout)
		stopErr := stopImportPhaseAServer(stopCtx, lifecycle, instance.DataDir, options.PollInterval)
		cancel()
		if stopErr != nil {
			return recordImportPhaseAFailure(instance.DataDir, operationID, ImportErrorRecoveryRequired, phaseAOutcomeRecoveryRequired, "FIFO write failed and the maintenance server could not be stopped", nil, stopErr)
		}
		journal, _ = LoadImportJournal(instance.DataDir, operationID)
		journal.MaintenanceStarted = false
		journal.PhaseAOutcome = phaseAOutcomeFIFOWriteFailed
		journal.PhaseALogDetail = redactPhaseALog(writeResult.Stdout+" "+writeResult.Stderr+" "+writeErr.Error(), platformID)
		journal.LastErrorCode, journal.LastError = ImportErrorCommandFailed, "Junimo import command could not be written to FIFO"
		_ = WriteImportJournal(instance.DataDir, journal)
		return &ImportTransactionError{Code: ImportErrorCommandFailed, Message: journal.LastError, Cause: writeErr}
	}

	// This write is deliberately the first action after FIFO write completion.
	// No log text or result inference is consulted before submission is durable.
	journal, err = LoadImportJournal(instance.DataDir, operationID)
	if err != nil {
		return &ImportTransactionError{Code: ImportErrorResultUnconfirmed, Message: "FIFO write completed but submission journal could not be reloaded", Cause: err}
	}
	now := time.Now().UTC()
	journal.UpstreamSubmitted = true
	journal.UpstreamSubmittedAt = &now
	journal.Stage = ImportStageSubmitted
	journal.LastErrorCode, journal.LastError = "", ""
	if err := WriteImportJournal(instance.DataDir, journal); err != nil {
		// The command must never be re-sent. Stop it, then classify only from
		// final disk state even if the first durable journal write failed.
		_ = WriteImportJournal(instance.DataDir, journal)
		return d.finalizeTimedOutImportPhaseA(instance.DataDir, operationID, lifecycle, pre, options, job, err)
	}

	deadline := time.NewTimer(options.ObservationTimeout)
	defer deadline.Stop()
	ticker := time.NewTicker(options.PollInterval)
	defer ticker.Stop()
	for {
		evidence, captureErr := capturePhaseADiskEvidence(instance.DataDir, journal.SaveName)
		if captureErr == nil {
			classification := classifyImportPhaseA(journal, pre, evidence)
			if classification == phaseAConfirmedSwap || classification == phaseAConfirmedAsIs {
				return confirmImportPhaseA(instance.DataDir, operationID, evidence, classification, true, job)
			}
		}
		select {
		case <-ctx.Done():
			return d.finalizeTimedOutImportPhaseA(instance.DataDir, operationID, lifecycle, pre, options, job, ctx.Err())
		case <-deadline.C:
			return d.finalizeTimedOutImportPhaseA(instance.DataDir, operationID, lifecycle, pre, options, job, context.DeadlineExceeded)
		case <-ticker.C:
		}
	}
}

func prepareImportPhaseASubmission(ctx context.Context, lifecycle LifecycleDockerService, dataDir, operationID, platformID string) (ImportJournal, JunimoImportEvidenceSnapshot, int64, string, error) {
	journal, err := LoadImportJournal(dataDir, operationID)
	if err != nil {
		return journal, JunimoImportEvidenceSnapshot{}, 0, "", err
	}
	if journal.Stage != ImportStageRuntimeReady || journal.RuntimeBaseline == nil || !journal.MaintenanceStarted {
		return journal, JunimoImportEvidenceSnapshot{}, 0, "", &ImportTransactionError{Code: ImportErrorRecoveryRequired, Message: "save import Phase A requires a live runtime_ready transaction"}
	}
	command, err := buildImportPhaseACommand(journal, platformID)
	if err != nil {
		return journal, JunimoImportEvidenceSnapshot{}, 0, "", err
	}
	if err := rejectConnectedFarmhands(ctx, lifecycle, dataDir); err != nil {
		return journal, JunimoImportEvidenceSnapshot{}, 0, "", err
	}
	pre, err := CaptureJunimoImportEvidence(ctx, lifecycle, dataDir, journal.SaveName)
	if err != nil {
		return journal, pre, 0, "", err
	}
	if pre.MainSaveSHA256 == "" || pre.ActivePointer == "" || pre.ProcessIdentity == nil {
		return journal, pre, 0, "", &ImportTransactionError{Code: ImportErrorRecoveryRequired, Message: "pre-submit import evidence is incomplete"}
	}
	if pre.PendingIntent.Exists {
		return journal, pre, 0, "", &ImportTransactionError{Code: ImportErrorRecoveryRequired, Message: "a pending save-import intent already exists before submission"}
	}
	if pre.ActivePointer == journal.SaveName {
		return journal, pre, 0, "", &ImportTransactionError{Code: ImportErrorRecoveryRequired, Message: "Panel must not preselect the import target before Phase A"}
	}
	if journal.RuntimeBaseline.MainSaveSHA256 == "" || pre.MainSaveSHA256 != journal.RuntimeBaseline.MainSaveSHA256 {
		return journal, pre, 0, "", &ImportTransactionError{Code: ImportErrorRecoveryRequired, Message: "target save changed after runtime_ready baseline"}
	}
	if journal.RuntimeBaseline.ProcessIdentity == nil || *pre.ProcessIdentity != *journal.RuntimeBaseline.ProcessIdentity {
		return journal, pre, 0, "", &ImportTransactionError{Code: ImportErrorRecoveryRequired, Message: "maintenance process identity changed before Phase A submission"}
	}
	if err := validatePreimportForPhaseA(dataDir, journal, pre.MainSaveSHA256); err != nil {
		return journal, pre, 0, "", &ImportTransactionError{Code: ImportErrorRecoveryRequired, Message: "preimport backup validation failed", Cause: err}
	}
	offset, err := strictServerLogSize(ctx, lifecycle, dataDir)
	if err != nil {
		return journal, pre, 0, "", &ImportTransactionError{Code: ImportErrorMaintenanceLog, Message: "server-output log is unavailable before Phase A", Cause: err}
	}
	identity, err := readProcessIdentity(ctx, lifecycle, dataDir)
	if err != nil || identity == nil || *identity != *pre.ProcessIdentity {
		return journal, pre, 0, "", &ImportTransactionError{Code: ImportErrorRecoveryRequired, Message: "maintenance process changed during Phase A pre-submit checks", Cause: err}
	}
	if err := rejectConnectedFarmhands(ctx, lifecycle, dataDir); err != nil {
		return journal, pre, 0, "", err
	}
	return journal, pre, offset, command, nil
}

func buildImportPhaseACommand(journal ImportJournal, platformID string) (string, error) {
	if err := validateSaveName(journal.SaveName); err != nil || !safeImportCommandToken(journal.SaveName) {
		return "", &ImportTransactionError{Code: "invalid_save", Message: "save name cannot be represented safely in a Junimo command", Cause: err}
	}
	base := "saves import " + journal.SaveName
	switch journal.HostHandling {
	case "server_owns_original":
		return base + " --reload", nil
	case "swap_host_to":
		if !validImportPlatformID(platformID) || platformFingerprint(journal.OperationID, platformID) != journal.PlatformIDFingerprint {
			return "", &ImportTransactionError{Code: "invalid_platform_id", Message: "platform ID is invalid or does not match the transaction"}
		}
		return base + " --swap-host-to " + platformID + " --reload", nil
	default:
		return "", &ImportTransactionError{Code: "invalid_host_handling", Message: "host handling mode is invalid"}
	}
}

func safeImportCommandToken(value string) bool {
	if value == "" || strings.HasPrefix(value, "-") {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) || unicode.IsSpace(r) || (!unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-' && r != '.') {
			return false
		}
	}
	return true
}

func validImportPlatformID(value string) bool {
	if value == "" || strings.TrimSpace(value) != value {
		return false
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	return err == nil && parsed != 0
}

func capturePhaseADiskEvidence(dataDir, saveName string) (JunimoImportEvidenceSnapshot, error) {
	snapshot := JunimoImportEvidenceSnapshot{CapturedAt: time.Now().UTC()}
	intent, err := ReadJunimoSaveImportIntent(dataDir)
	if err != nil {
		return snapshot, err
	}
	snapshot.PendingIntent = intent
	snapshot.MainSaveSHA256, err = stableFileSHA256(filepath.Join(savesDir(dataDir), "Saves", saveName, saveName))
	if err != nil {
		return snapshot, err
	}
	snapshot.ActivePointer, err = readActivePointerStrict(dataDir)
	if err != nil {
		return snapshot, err
	}
	return snapshot, nil
}

func classifyImportPhaseA(journal ImportJournal, pre, after JunimoImportEvidenceSnapshot) phaseAClassification {
	changed := after.MainSaveSHA256 != pre.MainSaveSHA256
	pointerTarget := after.ActivePointer == journal.SaveName
	pointerUnchanged := after.ActivePointer == pre.ActivePointer
	pending := after.PendingIntent

	if journal.HostHandling == "server_owns_original" {
		if !changed && !pending.Exists && pointerTarget && pre.ActivePointer != journal.SaveName {
			return phaseAConfirmedAsIs
		}
		if !changed && !pending.Exists && pointerUnchanged {
			return phaseANoEffect
		}
		if changed && !pending.Exists {
			return phaseAHalfConversion
		}
		return phaseAContradictory
	}

	pendingMatches := pending.Exists && pending.SaveName == journal.SaveName && pending.OwnerUID != 0 &&
		ComparePendingPlatformFingerprint(journal.OperationID, journal.PlatformIDFingerprint, pending) == EvidenceMatch
	if changed && pendingMatches && pointerTarget {
		return phaseAConfirmedSwap
	}
	if !changed && !pending.Exists && pointerUnchanged {
		return phaseANoEffect
	}
	if changed && pendingMatches && !pointerTarget {
		return phaseARecoveryRequired
	}
	if changed && !pending.Exists {
		return phaseAHalfConversion
	}
	return phaseAContradictory
}

func (d *Driver) finalizeTimedOutImportPhaseA(dataDir, operationID string, lifecycle LifecycleDockerService, pre JunimoImportEvidenceSnapshot, options importPhaseAOptions, job *jobs.Context, observationErr error) error {
	stopCtx, cancel := context.WithTimeout(context.Background(), options.StopTimeout)
	defer cancel()
	if err := stopImportPhaseAServer(stopCtx, lifecycle, dataDir, options.PollInterval); err != nil {
		return recordImportPhaseAFailure(dataDir, operationID, ImportErrorRecoveryRequired, phaseAOutcomeRecoveryRequired, "server could not be stopped after Phase A timeout", nil, err)
	}
	journal, err := LoadImportJournal(dataDir, operationID)
	if err != nil {
		return err
	}
	journal.MaintenanceStarted = false
	_ = WriteImportJournal(dataDir, journal)
	after, err := capturePhaseADiskEvidence(dataDir, journal.SaveName)
	if err != nil {
		return recordImportPhaseAFailure(dataDir, operationID, ImportErrorResultUnconfirmed, phaseAOutcomeResultUnconfirmed, "final Phase A disk evidence is unreadable", nil, err)
	}
	classification := classifyImportPhaseA(journal, pre, after)
	switch classification {
	case phaseAConfirmedSwap, phaseAConfirmedAsIs:
		return confirmImportPhaseA(dataDir, operationID, after, classification, false, job)
	case phaseANoEffect:
		return recordImportPhaseAFailure(dataDir, operationID, ImportErrorCommandFailed, phaseAOutcomeNoEffect, "Junimo import command produced no disk effect", &after, observationErr)
	case phaseARecoveryRequired:
		return recordImportPhaseAFailure(dataDir, operationID, ImportErrorRecoveryRequired, phaseAOutcomeRecoveryRequired, "save transformed and pending matched, but the boot target was not set", &after, observationErr)
	case phaseAHalfConversion:
		restoreHash, restoreErr := restorePreimportForPhaseA(dataDir, journal, pre.MainSaveSHA256)
		journal, _ = LoadImportJournal(dataDir, operationID)
		journal.PhaseAEvidence = &after
		journal.PhaseARestoredSHA256 = restoreHash
		if restoreErr != nil || restoreHash != pre.MainSaveSHA256 {
			journal.PhaseAOutcome = phaseAOutcomeHalfRestoreFailed
			journal.LastErrorCode, journal.LastError = ImportErrorRecoveryRequired, "half-converted save could not be verified after preimport restore"
			_ = WriteImportJournal(dataDir, journal)
			return &ImportTransactionError{Code: ImportErrorRecoveryRequired, Message: journal.LastError, Cause: restoreErr}
		}
		journal.PhaseAOutcome = phaseAOutcomeHalfRestored
		journal.LastErrorCode, journal.LastError = ImportErrorRecoveryRequired, "half-converted save was restored from preimport; manual recovery is still required"
		_ = WriteImportJournal(dataDir, journal)
		return &ImportTransactionError{Code: ImportErrorRecoveryRequired, Message: journal.LastError}
	default:
		return recordImportPhaseAFailure(dataDir, operationID, ImportErrorResultUnconfirmed, phaseAOutcomeResultUnconfirmed, "Phase A disk evidence is contradictory", &after, observationErr)
	}
}

func stopImportPhaseAServer(ctx context.Context, lifecycle LifecycleDockerService, dataDir string, interval time.Duration) error {
	result, err := lifecycle.ComposeDown(ctx, dataDir)
	if err != nil || result.ExitCode != 0 {
		if err == nil {
			err = fmt.Errorf("compose down exited with code %d", result.ExitCode)
		}
		return err
	}
	for {
		ps, err := lifecycle.ComposePs(ctx, dataDir)
		if err == nil && !serverServiceUp(ps.Services) {
			return nil
		}
		if err := waitImportPoll(ctx, interval); err != nil {
			return err
		}
	}
}

func confirmImportPhaseA(dataDir, operationID string, evidence JunimoImportEvidenceSnapshot, classification phaseAClassification, runtimeStillRunning bool, job *jobs.Context) error {
	journal, err := LoadImportJournal(dataDir, operationID)
	if err != nil {
		return err
	}
	journal.UpstreamSubmitted = true
	if journal.UpstreamSubmittedAt == nil {
		now := time.Now().UTC()
		journal.UpstreamSubmittedAt = &now
	}
	journal.UpstreamConfirmed = true
	journal.Stage = ImportStageConfirmed
	journal.PhaseAEvidence = &evidence
	journal.MaintenanceStarted = runtimeStillRunning
	if classification == phaseAConfirmedSwap {
		journal.PhaseAOutcome = phaseAOutcomeConfirmedSwap
	} else {
		journal.PhaseAOutcome = phaseAOutcomeConfirmedAsIs
	}
	journal.LastErrorCode, journal.LastError = "", ""
	if err := WriteImportJournal(dataDir, journal); err != nil {
		return err
	}
	maintenanceLog(job, "Phase A composite disk evidence confirmed. Final save activation and migration verification remain pending.")
	return nil
}

func recordImportPhaseAFailure(dataDir, operationID, code, outcome, message string, evidence *JunimoImportEvidenceSnapshot, cause error) error {
	if journal, err := LoadImportJournal(dataDir, operationID); err == nil {
		journal.PhaseAOutcome = outcome
		journal.PhaseAEvidence = evidence
		journal.LastErrorCode, journal.LastError = code, message
		_ = WriteImportJournal(dataDir, journal)
	}
	return &ImportTransactionError{Code: code, Message: message, Cause: cause}
}

func validatePreimportForPhaseA(dataDir string, journal ImportJournal, expectedMainHash string) error {
	if journal.PreimportBackupName == "" || journal.PreimportBackupSHA256 == "" {
		return errors.New("preimport backup metadata is missing")
	}
	path := filepath.Join(backupsDir(dataDir), journal.PreimportBackupName)
	archiveHash, err := stableFileSHA256(path)
	if err != nil {
		return err
	}
	if archiveHash != journal.PreimportBackupSHA256 {
		return errors.New("preimport backup hash mismatch")
	}
	zr, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer zr.Close()
	if err := validateZipEntries(zr.File); err != nil {
		return err
	}
	name, err := detectSaveFolderName(zr)
	if err != nil || name != journal.SaveName {
		return errors.New("preimport backup save name mismatch")
	}
	mainHash, err := hashZipEntry(zr.File, filepath.ToSlash(filepath.Join(journal.SaveName, journal.SaveName)))
	if err != nil {
		return err
	}
	if mainHash != expectedMainHash {
		return errors.New("preimport backup main save hash mismatch")
	}
	return nil
}

func hashZipEntry(files []*zip.File, name string) (string, error) {
	for _, file := range files {
		if strings.TrimSuffix(file.Name, "/") != name {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			return "", err
		}
		h := sha256.New()
		_, copyErr := io.Copy(h, reader)
		closeErr := reader.Close()
		if copyErr != nil {
			return "", copyErr
		}
		if closeErr != nil {
			return "", closeErr
		}
		return hex.EncodeToString(h.Sum(nil)), nil
	}
	return "", errors.New("preimport backup is missing the main save")
}

func restorePreimportForPhaseA(dataDir string, journal ImportJournal, expectedMainHash string) (string, error) {
	if err := validatePreimportForPhaseA(dataDir, journal, expectedMainHash); err != nil {
		return "", err
	}
	archivePath := filepath.Join(backupsDir(dataDir), journal.PreimportBackupName)
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer zr.Close()
	savesRoot := filepath.Join(savesDir(dataDir), "Saves")
	tempRoot, err := os.MkdirTemp(savesRoot, ".phase-a-restore-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tempRoot)
	if err := extractZipSecure(zr, tempRoot); err != nil {
		return "", err
	}
	extracted := filepath.Join(tempRoot, journal.SaveName)
	if err := validateImportSaveDirectory(extracted, journal.SaveName); err != nil {
		return "", err
	}
	extractedHash, err := stableFileSHA256(filepath.Join(extracted, journal.SaveName))
	if err != nil || extractedHash != expectedMainHash {
		return extractedHash, errors.New("extracted preimport hash mismatch")
	}
	target := filepath.Join(savesRoot, journal.SaveName)
	quarantine := filepath.Join(savesRoot, ".phase-a-replaced-"+importOperationDigest(journal.OperationID))
	if _, err := os.Lstat(quarantine); err == nil {
		return "", errors.New("prior Phase A restore quarantine already exists")
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err := os.Rename(target, quarantine); err != nil {
		return "", err
	}
	if err := os.Rename(extracted, target); err != nil {
		_ = os.Rename(quarantine, target)
		return "", err
	}
	restoredHash, err := stableFileSHA256(filepath.Join(target, journal.SaveName))
	if err != nil || restoredHash != expectedMainHash {
		bad := quarantine + ".bad"
		_ = os.Rename(target, bad)
		_ = os.Rename(quarantine, target)
		return restoredHash, errors.New("published preimport restore hash mismatch")
	}
	if err := os.RemoveAll(quarantine); err != nil {
		return restoredHash, err
	}
	return restoredHash, nil
}

func redactPhaseALog(value, platformID string) string {
	if platformID != "" {
		value = strings.ReplaceAll(value, platformID, "[redacted-platform-id]")
	}
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, value)
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) > 1024 {
		value = string(runes[:1024])
	}
	return value
}
