package stardew_junimo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

const phaseATestPlatformID = "76561198000000001"

type phaseATestFixture struct {
	dataDir  string
	op       string
	instance registryInstanceAlias
	store    *importMaintenanceStore
	fake     *fakeConsoleDocker
	record   *maintenanceFakeRecord
	preHash  string
	teeCalls int
}

// Alias keeps the fixture declaration compact without introducing a second
// representation of registry.Instance.
type registryInstanceAlias = registry.Instance

func preparePhaseATestFixture(t *testing.T, hostHandling string) *phaseATestFixture {
	t.Helper()
	dataDir, op, instance, store := prepareMaintenanceFixture(t)
	backupPath, backupSHA, err := BackupPreImport(dataDir, "Upload_1", op)
	if err != nil {
		t.Fatal(err)
	}
	preHash, err := stableFileSHA256(filepath.Join(savesDir(dataDir), "Saves", "Upload_1", "Upload_1"))
	if err != nil {
		t.Fatal(err)
	}
	journal, err := LoadImportJournal(dataDir, op)
	if err != nil {
		t.Fatal(err)
	}
	journal.HostHandling = hostHandling
	journal.StagedSaveCreated = true
	journal.StagedSaveFingerprint, err = importDirectoryFingerprint(filepath.Join(savesDir(dataDir), "Saves", "Upload_1"))
	if err != nil {
		t.Fatal(err)
	}
	journal.PreimportBackupName = filepath.Base(backupPath)
	journal.PreimportBackupSHA256 = backupSHA
	journal.Stage = ImportStageRuntimeReady
	journal.MaintenanceStarted = true
	disabled := false
	journal.MaintenanceSteamInviteEnabled = &disabled
	store.mu.Lock()
	original := store.instance
	store.mu.Unlock()
	captureOriginalInstanceSnapshot(&journal, original)
	journal.MaintenanceRecoveryState = importMaintenanceRuntimeReadyPersisted
	journal.RuntimeBaseline = &JunimoImportEvidenceSnapshot{
		MainSaveSHA256: preHash, ActivePointer: "Old_1",
		ProcessIdentity: &JunimoProcessIdentity{ContainerID: "container-a", ProcessStartTicks: "123"},
	}
	if hostHandling == "swap_host_to" {
		journal.PlatformIDFingerprint = platformFingerprint(op, phaseATestPlatformID)
	}
	if err := WriteImportJournal(dataDir, journal); err != nil {
		t.Fatal(err)
	}
	fake, record := newMaintenanceFake(maintenanceFakeConfig{})
	record.started = true
	return &phaseATestFixture{dataDir: dataDir, op: op, instance: instance, store: store, fake: fake, record: record, preHash: preHash}
}

func TestImportPhaseAPreSubmitEvidenceFailureStopsAndRestores(t *testing.T) {
	f := preparePhaseATestFixture(t, "server_owns_original")
	f.store.mu.Lock()
	f.store.instance.State = storage.InstanceStateStopped
	f.store.instance.StateMessage = sql.NullString{String: "maintenance", Valid: true}
	f.store.instance.DriverPhase = importMaintenancePhase
	f.store.instance.DriverPayload = `{"other":"kept"}`
	f.store.mu.Unlock()
	if err := os.Remove(filepath.Join(savesDir(f.dataDir), "Saves", "Upload_1", "Upload_1")); err != nil {
		t.Fatal(err)
	}
	driver := New(f.fake, nil, nil, f.store)
	err := driver.runImportPhaseA(context.Background(), f.instance, f.op, "", nil, phaseATestOptions())
	if err == nil {
		t.Fatal("pre-submit evidence failure unexpectedly succeeded")
	}
	if f.teeCalls != 0 {
		t.Fatalf("FIFO writes=%d, want zero", f.teeCalls)
	}
	f.record.mu.Lock()
	down := f.record.down
	f.record.mu.Unlock()
	if !down {
		t.Fatal("maintenance runtime was not stopped after pre-submit evidence failure")
	}
	journal, loadErr := LoadImportJournal(f.dataDir, f.op)
	if loadErr != nil || journal.MaintenanceStarted || journal.UpstreamSubmitted {
		t.Fatalf("journal=%+v err=%v", journal, loadErr)
	}
	f.store.mu.Lock()
	restored := f.store.instance
	f.store.mu.Unlock()
	if restored.DriverPhase != "container_stopped" || restored.StateMessage.String != "stopped before import" {
		t.Fatalf("original instance snapshot not restored: %+v", restored)
	}
}

func TestImportPhaseAMissingJournalStopsConservativeScopeWithoutRestore(t *testing.T) {
	f := preparePhaseATestFixture(t, "server_owns_original")
	f.store.mu.Lock()
	f.store.instance.State = storage.InstanceStateStopped
	f.store.instance.StateMessage = sql.NullString{String: "maintenance", Valid: true}
	f.store.instance.DriverPhase = importMaintenancePhase
	f.store.instance.DriverPayload = `{"maintenance":true}`
	f.store.mu.Unlock()
	if err := os.Remove(importJournalPath(f.dataDir, f.op)); err != nil {
		t.Fatal(err)
	}
	var stoppedServices []string
	f.fake.stopServicesFunc = func(_ context.Context, _, _ string, services ...string) error {
		stoppedServices = append([]string(nil), services...)
		f.record.mu.Lock()
		f.record.down = true
		f.record.started = false
		f.record.mu.Unlock()
		return nil
	}
	driver := New(f.fake, nil, nil, f.store)
	err := driver.runImportPhaseA(context.Background(), f.instance, f.op, "", nil, phaseATestOptions())
	typed, ok := AsImportTransactionError(err)
	if !ok || typed.Code != ImportErrorRecoveryRequired {
		t.Fatalf("error=%T %v, want recovery_required", err, err)
	}
	if strings.Join(stoppedServices, ",") != "server,steam-auth" {
		t.Fatalf("conservative stop services=%v, want server and Auth", stoppedServices)
	}
	if f.teeCalls != 0 {
		t.Fatalf("FIFO writes=%d, want zero", f.teeCalls)
	}
	f.store.mu.Lock()
	current := f.store.instance
	f.store.mu.Unlock()
	if current.DriverPhase != importMaintenancePhase {
		t.Fatalf("missing journal incorrectly restored ordinary state: %+v", current)
	}
}

func TestImportPhaseAPreSubmitPanicStopsAndRestores(t *testing.T) {
	f := preparePhaseATestFixture(t, "server_owns_original")
	f.store.mu.Lock()
	f.store.instance.State = storage.InstanceStateStopped
	f.store.instance.StateMessage = sql.NullString{String: "maintenance", Valid: true}
	f.store.instance.DriverPhase = importMaintenancePhase
	f.store.instance.DriverPayload = `{"maintenance":true}`
	f.store.mu.Unlock()
	f.fake.execFunc = func(context.Context, string, string, string, ...string) (paneldocker.CommandResult, error) {
		panic("injected Phase A pre-submit panic")
	}
	driver := New(f.fake, nil, nil, f.store)
	var panicValue any
	func() {
		defer func() { panicValue = recover() }()
		_ = driver.runImportPhaseA(context.Background(), f.instance, f.op, "", nil, phaseATestOptions())
	}()
	if panicValue == nil {
		t.Fatal("Phase A pre-submit panic was swallowed")
	}
	if strings.Contains(fmt.Sprint(panicValue), "injected Phase A pre-submit panic") {
		t.Fatalf("Phase A pre-submit panic leaked its raw internal value: %v", panicValue)
	}
	f.record.mu.Lock()
	down := f.record.down
	f.record.mu.Unlock()
	if !down {
		t.Fatal("Phase A pre-submit panic did not stop maintenance runtime")
	}
	journal, err := LoadImportJournal(f.dataDir, f.op)
	if err != nil || journal.MaintenanceStarted || journal.MaintenanceRecoveryState != importMaintenanceSnapshotRestored || journal.PhaseAFIFOWriteAttempted {
		t.Fatalf("journal=%+v err=%v", journal, err)
	}
	f.store.mu.Lock()
	restored := f.store.instance
	f.store.mu.Unlock()
	if restored.DriverPhase != "container_stopped" || restored.StateMessage.String != "stopped before import" {
		t.Fatalf("pre-submit panic did not restore original instance: %+v", restored)
	}
}

func TestImportPhaseAPostFIFOIntentPanicStopsAndStaysManual(t *testing.T) {
	f := preparePhaseATestFixture(t, "server_owns_original")
	f.store.mu.Lock()
	f.store.instance.State = storage.InstanceStateStopped
	f.store.instance.StateMessage = sql.NullString{String: "maintenance", Valid: true}
	f.store.instance.DriverPhase = importMaintenancePhase
	f.store.instance.DriverPayload = `{"maintenance":true}`
	f.store.mu.Unlock()
	f.interceptFIFO(func(string) (paneldocker.CommandResult, error) {
		panic("injected Phase A FIFO panic")
	})
	driver := New(f.fake, nil, nil, f.store)
	var panicValue any
	func() {
		defer func() { panicValue = recover() }()
		_ = driver.runImportPhaseA(context.Background(), f.instance, f.op, "", nil, phaseATestOptions())
	}()
	if panicValue == nil {
		t.Fatal("Phase A FIFO panic was swallowed")
	}
	if strings.Contains(fmt.Sprint(panicValue), "injected Phase A FIFO panic") {
		t.Fatalf("Phase A FIFO panic leaked its raw internal value: %v", panicValue)
	}
	f.record.mu.Lock()
	down := f.record.down
	f.record.mu.Unlock()
	if !down {
		t.Fatal("ambiguous Phase A panic left maintenance runtime running")
	}
	journal, err := LoadImportJournal(f.dataDir, f.op)
	if err != nil || !journal.MaintenanceStarted || !journal.PhaseAFIFOWriteAttempted || journal.UpstreamSubmitted ||
		journal.MaintenanceRecoveryState != importMaintenanceManualRecovery || journal.RecoveryState != "manual_required" {
		t.Fatalf("journal=%+v err=%v", journal, err)
	}
	f.store.mu.Lock()
	current := f.store.instance
	f.store.mu.Unlock()
	if current.DriverPhase != importMaintenancePhase {
		t.Fatalf("ambiguous Phase A panic incorrectly restored ordinary state: %+v", current)
	}
}

func TestImportPhaseAFIFOSuccessJournalLossStopsWithoutResubmitting(t *testing.T) {
	f := preparePhaseATestFixture(t, "server_owns_original")
	f.store.mu.Lock()
	f.store.instance.State = storage.InstanceStateStopped
	f.store.instance.StateMessage = sql.NullString{String: "maintenance", Valid: true}
	f.store.instance.DriverPhase = importMaintenancePhase
	f.store.instance.DriverPayload = `{"maintenance":true}`
	f.store.mu.Unlock()
	var removeErr error
	f.interceptFIFO(func(string) (paneldocker.CommandResult, error) {
		removeErr = os.Remove(importJournalPath(f.dataDir, f.op))
		return paneldocker.CommandResult{}, nil
	})
	driver := New(f.fake, nil, nil, f.store)
	err := driver.runImportPhaseA(context.Background(), f.instance, f.op, "", nil, phaseATestOptions())
	if removeErr != nil {
		t.Fatalf("remove post-FIFO journal fixture: %v", removeErr)
	}
	typed, ok := AsImportTransactionError(err)
	if !ok || typed.Code != ImportErrorRecoveryRequired {
		t.Fatalf("error=%v typed=%+v, want recovery_required", err, typed)
	}
	if f.teeCalls != 1 {
		t.Fatalf("FIFO writes=%d, want exactly one", f.teeCalls)
	}
	f.record.mu.Lock()
	down := f.record.down
	f.record.mu.Unlock()
	if !down {
		t.Fatal("post-FIFO journal loss left maintenance runtime running")
	}
	if _, loadErr := LoadImportJournal(f.dataDir, f.op); !os.IsNotExist(loadErr) {
		t.Fatalf("journal loss fixture unexpectedly recovered a journal: %v", loadErr)
	}
	f.store.mu.Lock()
	current := f.store.instance
	f.store.mu.Unlock()
	if current.DriverPhase != importMaintenancePhase {
		t.Fatalf("post-FIFO journal loss incorrectly restored ordinary state: %+v", current)
	}
}

func (f *phaseATestFixture) interceptFIFO(effect func(command string) (paneldocker.CommandResult, error)) {
	base := f.fake.execFunc
	f.fake.execFunc = func(ctx context.Context, dir, service, stdin string, args ...string) (paneldocker.CommandResult, error) {
		if len(args) > 0 && args[0] == "tee" && strings.HasPrefix(stdin, "saves import ") {
			f.teeCalls++
			return effect(strings.TrimSuffix(stdin, "\n"))
		}
		return base(ctx, dir, service, stdin, args...)
	}
}

func (f *phaseATestFixture) writeMain(t *testing.T, value string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(savesDir(f.dataDir), "Saves", "Upload_1", "Upload_1"), []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
}

func (f *phaseATestFixture) writePointer(t *testing.T, value string) {
	t.Helper()
	if err := os.WriteFile(gameloaderPath(f.dataDir), []byte(`{"SaveNameToLoad":"`+value+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
}

func (f *phaseATestFixture) writePending(t *testing.T, saveName string, ownerUID int64, userID string) {
	t.Helper()
	value := `{"Pending":{"SaveName":"` + saveName + `","OwnerUid":` + strconv.FormatInt(ownerUID, 10) + `,"UserId":"` + userID + `"}}`
	if err := os.WriteFile(saveImportIntentPath(f.dataDir), []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
}

func phaseATestOptions() importPhaseAOptions {
	return importPhaseAOptions{ObservationTimeout: 12 * time.Millisecond, PollInterval: time.Millisecond, StopTimeout: 100 * time.Millisecond}
}

func TestImportPhaseASwapCompleteCompositeEvidence(t *testing.T) {
	f := preparePhaseATestFixture(t, "swap_host_to")
	f.interceptFIFO(func(command string) (paneldocker.CommandResult, error) {
		if command != "saves import Upload_1 --swap-host-to "+phaseATestPlatformID+" --reload" {
			t.Fatalf("command=%q", command)
		}
		f.writeMain(t, "transformed")
		f.writePending(t, "Upload_1", 1234, phaseATestPlatformID)
		f.writePointer(t, "Upload_1")
		return paneldocker.CommandResult{Stdout: "Imported successfully"}, nil
	})
	d := New(f.fake, nil, nil, f.store)
	if err := d.runImportPhaseA(context.Background(), f.instance, f.op, phaseATestPlatformID, nil, phaseATestOptions()); err != nil {
		t.Fatal(err)
	}
	journal, _ := LoadImportJournal(f.dataDir, f.op)
	if journal.Stage != ImportStageConfirmed || !journal.UpstreamSubmitted || !journal.UpstreamConfirmed || journal.PhaseAOutcome != phaseAOutcomeConfirmedSwap {
		t.Fatalf("journal=%+v", journal)
	}
	if f.teeCalls != 1 {
		t.Fatalf("FIFO writes=%d", f.teeCalls)
	}
	rawJournal, err := os.ReadFile(importJournalPath(f.dataDir, f.op))
	if err != nil || strings.Contains(string(rawJournal), phaseATestPlatformID) {
		t.Fatalf("raw platform ID leaked into journal: err=%v", err)
	}
}

func TestImportPhaseAConfirmWriteFailureStopsAndStaysManual(t *testing.T) {
	f := preparePhaseATestFixture(t, "swap_host_to")
	f.store.mu.Lock()
	f.store.instance.State = storage.InstanceStateStopped
	f.store.instance.StateMessage = sql.NullString{String: "maintenance", Valid: true}
	f.store.instance.DriverPhase = importMaintenancePhase
	f.store.instance.DriverPayload = `{"maintenance":true}`
	f.store.mu.Unlock()
	f.interceptFIFO(func(string) (paneldocker.CommandResult, error) {
		f.writeMain(t, "transformed")
		f.writePending(t, "Upload_1", 1234, phaseATestPlatformID)
		f.writePointer(t, "Upload_1")
		return paneldocker.CommandResult{Stdout: "Imported successfully"}, nil
	})
	driver := New(f.fake, nil, nil, f.store)
	confirmationWriteFailed := false
	driver.importJournalWrite = func(dataDir string, journal ImportJournal) error {
		if journal.Stage == ImportStageConfirmed && !confirmationWriteFailed {
			confirmationWriteFailed = true
			return errors.New("injected confirmation journal failure")
		}
		return WriteImportJournal(dataDir, journal)
	}
	err := driver.runImportPhaseA(context.Background(), f.instance, f.op, phaseATestPlatformID, nil, phaseATestOptions())
	typed, ok := AsImportTransactionError(err)
	if !ok || typed.Code != ImportErrorRecoveryRequired {
		t.Fatalf("error=%T %v, want recovery_required", err, err)
	}
	if !confirmationWriteFailed || f.teeCalls != 1 {
		t.Fatalf("confirmationWriteFailed=%v FIFO writes=%d, want one failed confirmation write and one submission", confirmationWriteFailed, f.teeCalls)
	}
	f.record.mu.Lock()
	down := f.record.down
	f.record.mu.Unlock()
	if !down {
		t.Fatal("confirmation journal failure left maintenance runtime running")
	}
	journal, loadErr := LoadImportJournal(f.dataDir, f.op)
	if loadErr != nil || journal.Stage != ImportStageSubmitted || !journal.PhaseAFIFOWriteAttempted || !journal.UpstreamSubmitted || journal.UpstreamConfirmed ||
		journal.MaintenanceRecoveryState != importMaintenanceManualRecovery || journal.RecoveryState != "manual_required" {
		t.Fatalf("journal=%+v err=%v", journal, loadErr)
	}
	f.store.mu.Lock()
	current := f.store.instance
	f.store.mu.Unlock()
	if current.DriverPhase != importMaintenancePhase {
		t.Fatalf("ambiguous confirmation failure incorrectly restored ordinary state: %+v", current)
	}
}

func TestImportPhaseAAsIsCompleteCompositeEvidence(t *testing.T) {
	f := preparePhaseATestFixture(t, "server_owns_original")
	f.interceptFIFO(func(command string) (paneldocker.CommandResult, error) {
		if command != "saves import Upload_1 --reload" {
			t.Fatalf("command=%q", command)
		}
		f.writePointer(t, "Upload_1")
		return paneldocker.CommandResult{}, nil
	})
	d := New(f.fake, nil, nil, f.store)
	if err := d.runImportPhaseA(context.Background(), f.instance, f.op, "", nil, phaseATestOptions()); err != nil {
		t.Fatal(err)
	}
	journal, _ := LoadImportJournal(f.dataDir, f.op)
	if journal.PhaseAOutcome != phaseAOutcomeConfirmedAsIs || journal.PhaseAEvidence.MainSaveSHA256 != f.preHash {
		t.Fatalf("journal=%+v", journal)
	}
}

func TestImportPhaseAIncompleteEvidenceMatrix(t *testing.T) {
	for _, tc := range []struct {
		name   string
		effect func(*testing.T, *phaseATestFixture)
		code   string
	}{
		{"success_log_only", func(_ *testing.T, _ *phaseATestFixture) {}, ImportErrorCommandFailed},
		{"pointer_only", func(t *testing.T, f *phaseATestFixture) { f.writePointer(t, "Upload_1") }, ImportErrorResultUnconfirmed},
		{"pending_save_mismatch", func(t *testing.T, f *phaseATestFixture) {
			f.writeMain(t, "changed")
			f.writePending(t, "Other_1", 9, phaseATestPlatformID)
			f.writePointer(t, "Upload_1")
		}, ImportErrorResultUnconfirmed},
		{"fingerprint_mismatch", func(t *testing.T, f *phaseATestFixture) {
			f.writeMain(t, "changed")
			f.writePending(t, "Upload_1", 9, "76561198000000002")
			f.writePointer(t, "Upload_1")
		}, ImportErrorResultUnconfirmed},
		{"owner_zero", func(t *testing.T, f *phaseATestFixture) {
			f.writeMain(t, "changed")
			f.writePending(t, "Upload_1", 0, phaseATestPlatformID)
			f.writePointer(t, "Upload_1")
		}, ImportErrorResultUnconfirmed},
		{"matching_pending_pointer_old", func(t *testing.T, f *phaseATestFixture) {
			f.writeMain(t, "changed")
			f.writePending(t, "Upload_1", 9, phaseATestPlatformID)
		}, ImportErrorRecoveryRequired},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := preparePhaseATestFixture(t, "swap_host_to")
			f.interceptFIFO(func(string) (paneldocker.CommandResult, error) {
				tc.effect(t, f)
				return paneldocker.CommandResult{Stdout: "Imported successfully"}, nil
			})
			d := New(f.fake, nil, nil, f.store)
			err := d.runImportPhaseA(context.Background(), f.instance, f.op, phaseATestPlatformID, nil, phaseATestOptions())
			typed, ok := AsImportTransactionError(err)
			if !ok || typed.Code != tc.code {
				t.Fatalf("error=%v", err)
			}
			if f.teeCalls != 1 {
				t.Fatalf("command was retried: %d", f.teeCalls)
			}
			f.record.mu.Lock()
			down := f.record.down
			f.record.mu.Unlock()
			if !down {
				t.Fatal("server was not stopped before final classification")
			}
		})
	}
}

func TestImportPhaseAFIFOWriteFailureIsNotSubmittedAndRedactsID(t *testing.T) {
	f := preparePhaseATestFixture(t, "swap_host_to")
	f.interceptFIFO(func(string) (paneldocker.CommandResult, error) {
		return paneldocker.CommandResult{Stderr: "failed for " + phaseATestPlatformID, ExitCode: 1}, errors.New("fifo failed " + phaseATestPlatformID)
	})
	d := New(f.fake, nil, nil, f.store)
	err := d.runImportPhaseA(context.Background(), f.instance, f.op, phaseATestPlatformID, nil, phaseATestOptions())
	typed, ok := AsImportTransactionError(err)
	if !ok || typed.Code != ImportErrorRecoveryRequired {
		t.Fatalf("error=%v", err)
	}
	journal, _ := LoadImportJournal(f.dataDir, f.op)
	if journal.UpstreamSubmitted || !journal.PhaseAFIFOWriteAttempted || journal.Stage != ImportStageRuntimeReady ||
		journal.MaintenanceRecoveryState != importMaintenanceManualRecovery || strings.Contains(journal.LastError, phaseATestPlatformID) {
		t.Fatalf("journal=%+v", journal)
	}
}

func TestImportPhaseATimeoutStopsBeforeFinalEvidenceAndFindsSuccess(t *testing.T) {
	f := preparePhaseATestFixture(t, "swap_host_to")
	f.interceptFIFO(func(string) (paneldocker.CommandResult, error) { return paneldocker.CommandResult{}, nil })
	baseDown := f.fake.composeDownFunc
	f.fake.composeDownFunc = func(ctx context.Context, dir string) (paneldocker.CommandResult, error) {
		result, err := baseDown(ctx, dir)
		f.writeMain(t, "late-transformed")
		f.writePending(t, "Upload_1", 77, phaseATestPlatformID)
		f.writePointer(t, "Upload_1")
		return result, err
	}
	d := New(f.fake, nil, nil, f.store)
	if err := d.runImportPhaseA(context.Background(), f.instance, f.op, phaseATestPlatformID, nil, phaseATestOptions()); err != nil {
		t.Fatal(err)
	}
	journal, _ := LoadImportJournal(f.dataDir, f.op)
	if journal.Stage != ImportStageConfirmed || journal.MaintenanceStarted || journal.PhaseAOutcome != phaseAOutcomeConfirmedSwap {
		t.Fatalf("journal=%+v", journal)
	}
}

func TestImportPhaseAHalfConversionRestoresPreimport(t *testing.T) {
	f := preparePhaseATestFixture(t, "swap_host_to")
	f.interceptFIFO(func(string) (paneldocker.CommandResult, error) {
		f.writeMain(t, "half-transformed")
		return paneldocker.CommandResult{}, nil
	})
	d := New(f.fake, nil, nil, f.store)
	err := d.runImportPhaseA(context.Background(), f.instance, f.op, phaseATestPlatformID, nil, phaseATestOptions())
	typed, ok := AsImportTransactionError(err)
	if !ok || typed.Code != ImportErrorRecoveryRequired {
		t.Fatalf("error=%v", err)
	}
	hash, hashErr := stableFileSHA256(filepath.Join(savesDir(f.dataDir), "Saves", "Upload_1", "Upload_1"))
	journal, _ := LoadImportJournal(f.dataDir, f.op)
	if hashErr != nil || hash != f.preHash || journal.PhaseAOutcome != phaseAOutcomeHalfRestored || journal.PhaseARestoredSHA256 != f.preHash {
		t.Fatalf("hash=%s err=%v journal=%+v", hash, hashErr, journal)
	}
}

func TestImportPhaseAHalfConversionRestoreHashMismatch(t *testing.T) {
	f := preparePhaseATestFixture(t, "swap_host_to")
	f.interceptFIFO(func(string) (paneldocker.CommandResult, error) {
		f.writeMain(t, "half-transformed")
		journal, _ := LoadImportJournal(f.dataDir, f.op)
		if err := os.WriteFile(filepath.Join(backupsDir(f.dataDir), journal.PreimportBackupName), []byte("corrupt"), 0o600); err != nil {
			t.Fatal(err)
		}
		return paneldocker.CommandResult{}, nil
	})
	d := New(f.fake, nil, nil, f.store)
	err := d.runImportPhaseA(context.Background(), f.instance, f.op, phaseATestPlatformID, nil, phaseATestOptions())
	typed, ok := AsImportTransactionError(err)
	if !ok || typed.Code != ImportErrorRecoveryRequired {
		t.Fatalf("error=%v", err)
	}
	journal, _ := LoadImportJournal(f.dataDir, f.op)
	if journal.PhaseAOutcome != phaseAOutcomeHalfRestoreFailed {
		t.Fatalf("journal=%+v", journal)
	}
}

func TestRecoverImportPhaseASubmittedAfterPanelRestart(t *testing.T) {
	f := preparePhaseATestFixture(t, "swap_host_to")
	journal, _ := LoadImportJournal(f.dataDir, f.op)
	journal.Stage, journal.UpstreamSubmitted = ImportStageSubmitted, true
	journal.PhaseAOutcome = ""
	if err := WriteImportJournal(f.dataDir, journal); err != nil {
		t.Fatal(err)
	}
	recoveries, err := RecoverImportTransactions(f.dataDir)
	if err != nil || len(recoveries) != 1 || recoveries[0].State != "manual_required" || recoveries[0].ErrorCode != ImportErrorResultUnconfirmed {
		t.Fatalf("recoveries=%+v err=%v", recoveries, err)
	}
}

func TestImportPhaseANoEffectRestoresSnapshotAndAllowsCleanup(t *testing.T) {
	f := preparePhaseATestFixture(t, "swap_host_to")
	baseExec := f.fake.execFunc
	f.fake.execFunc = func(ctx context.Context, dir, service, stdin string, args ...string) (paneldocker.CommandResult, error) {
		if f.teeCalls > 0 && len(args) >= 2 && args[0] == "wc" && args[1] == "-c" {
			return paneldocker.CommandResult{Stdout: "256 /tmp/server-output.log\n"}, nil
		}
		if f.teeCalls > 0 && len(args) >= 2 && args[0] == "tail" && args[1] == "-c" {
			return paneldocker.CommandResult{Stdout: "saves import: invalid platform " + phaseATestPlatformID}, nil
		}
		return baseExec(ctx, dir, service, stdin, args...)
	}
	f.interceptFIFO(func(string) (paneldocker.CommandResult, error) {
		return paneldocker.CommandResult{Stdout: "success text only"}, nil
	})
	d := New(f.fake, nil, nil, f.store)
	err := d.runImportPhaseA(context.Background(), f.instance, f.op, phaseATestPlatformID, nil, phaseATestOptions())
	typed, ok := AsImportTransactionError(err)
	if !ok || typed.Code != ImportErrorCommandFailed || f.teeCalls != 1 {
		t.Fatalf("error=%v writes=%d", err, f.teeCalls)
	}
	journal, err := LoadImportJournal(f.dataDir, f.op)
	if err != nil || journal.MaintenanceStarted || journal.MaintenanceRecoveryState != importMaintenanceSnapshotRestored ||
		journal.RecoveryState != "safe_to_resume_or_cleanup" || !journal.PhaseAFIFOWriteAttempted || !journal.UpstreamSubmitted ||
		journal.UpstreamConfirmed || !importJournalProvesPhaseANoEffect(journal) ||
		!strings.Contains(journal.PhaseALogDetail, "[redacted-platform-id]") || strings.Contains(journal.PhaseALogDetail, phaseATestPlatformID) {
		t.Fatalf("journal=%+v err=%v", journal, err)
	}
	f.store.mu.Lock()
	restored := f.store.instance
	f.store.mu.Unlock()
	if restored.State != storage.InstanceStateStopped || restored.StateMessage.String != "stopped before import" || restored.DriverPhase != "container_stopped" {
		t.Fatalf("original instance snapshot not restored: %+v", restored)
	}
	sourceDir := importTransactionSourceDir(f.dataDir, f.op)
	if err := os.MkdirAll(sourceDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "owned-source"), []byte("source"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := CleanupUnsubmittedImport(f.dataDir, f.op); err != nil {
		t.Fatal(err)
	}
	if err := FinalizeCanceledImportCleanup(f.dataDir, f.op); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(savesDir(f.dataDir), "Saves", "Upload_1")); !os.IsNotExist(err) {
		t.Fatalf("staged target survived proven-no-effect cleanup: %v", err)
	}
	if _, err := LoadImportJournal(f.dataDir, f.op); !os.IsNotExist(err) {
		t.Fatalf("finalized no-effect journal survived: %v", err)
	}
}

func TestRecoverImportPhaseANoEffectAfterPanelRestart(t *testing.T) {
	f := preparePhaseATestFixture(t, "swap_host_to")
	f.interceptFIFO(func(string) (paneldocker.CommandResult, error) { return paneldocker.CommandResult{}, nil })
	driver := New(f.fake, nil, nil, f.store)
	if err := driver.runImportPhaseA(context.Background(), f.instance, f.op, phaseATestPlatformID, nil, phaseATestOptions()); err == nil {
		t.Fatal("no-effect import unexpectedly succeeded")
	}
	journal, err := LoadImportJournal(f.dataDir, f.op)
	if err != nil || !importJournalProvesPhaseANoEffect(journal) {
		t.Fatalf("journal=%+v err=%v", journal, err)
	}
	journal.MaintenanceStarted = true
	journal.MaintenanceRecoveryState = importMaintenanceManualRecovery
	journal.RecoveryState = "manual_required"
	if err := WriteImportJournal(f.dataDir, journal); err != nil {
		t.Fatal(err)
	}
	f.store.mu.Lock()
	f.store.instance.State = storage.InstanceStateStopped
	f.store.instance.StateMessage = sql.NullString{String: "maintenance", Valid: true}
	f.store.instance.DriverPhase = importMaintenancePhase
	f.store.instance.DriverPayload = `{"phase":"manual"}`
	f.store.mu.Unlock()

	recoveries, err := RecoverImportTransactions(f.dataDir)
	if err != nil || len(recoveries) != 1 || recoveries[0].State != importRecoveryMaintenanceStopAndRestore {
		t.Fatalf("recoveries=%+v err=%v", recoveries, err)
	}
	recovered, err := driver.recoverInterruptedImportMaintenance(context.Background(), f.instance, recoveries)
	if err != nil || len(recovered) != 1 || recovered[0].State != "safe_to_resume_or_cleanup" {
		t.Fatalf("recoveries=%+v err=%v", recovered, err)
	}
	journal, err = LoadImportJournal(f.dataDir, f.op)
	if err != nil || journal.MaintenanceStarted || journal.MaintenanceRecoveryState != importMaintenanceSnapshotRestored {
		t.Fatalf("journal=%+v err=%v", journal, err)
	}
	f.store.mu.Lock()
	restored := f.store.instance
	f.store.mu.Unlock()
	if restored.State != storage.InstanceStateStopped || restored.StateMessage.String != "stopped before import" || restored.DriverPhase != "container_stopped" {
		t.Fatalf("restart recovery did not restore original snapshot: %+v", restored)
	}
}

func TestRecoverImportPhaseANoEffectFinishesPendingSnapshotRestore(t *testing.T) {
	f := preparePhaseATestFixture(t, "swap_host_to")
	f.interceptFIFO(func(string) (paneldocker.CommandResult, error) { return paneldocker.CommandResult{}, nil })
	driver := New(f.fake, nil, nil, f.store)
	if err := driver.runImportPhaseA(context.Background(), f.instance, f.op, phaseATestPlatformID, nil, phaseATestOptions()); err == nil {
		t.Fatal("no-effect import unexpectedly succeeded")
	}
	journal, err := LoadImportJournal(f.dataDir, f.op)
	if err != nil || !importJournalProvesPhaseANoEffect(journal) {
		t.Fatalf("journal=%+v err=%v", journal, err)
	}
	journal.MaintenanceStarted = false
	journal.MaintenanceRecoveryState = importMaintenanceSnapshotRestorePending
	journal.RecoveryState = ""
	if err := WriteImportJournal(f.dataDir, journal); err != nil {
		t.Fatal(err)
	}
	f.store.mu.Lock()
	f.store.instance.State = storage.InstanceStateStopped
	f.store.instance.StateMessage = sql.NullString{String: "snapshot restore interrupted", Valid: true}
	f.store.instance.DriverPhase = importMaintenancePhase
	f.store.instance.DriverPayload = `{"phase":"restore_pending"}`
	f.store.mu.Unlock()

	recoveries, err := RecoverImportTransactions(f.dataDir)
	if err != nil || len(recoveries) != 1 || recoveries[0].State != importRecoveryMaintenanceRestoreSnapshot {
		t.Fatalf("recoveries=%+v err=%v", recoveries, err)
	}
	recovered, err := driver.recoverInterruptedImportMaintenance(context.Background(), f.instance, recoveries)
	if err != nil || len(recovered) != 1 || recovered[0].State != "safe_to_resume_or_cleanup" {
		t.Fatalf("recoveries=%+v err=%v", recovered, err)
	}
	journal, err = LoadImportJournal(f.dataDir, f.op)
	if err != nil || journal.MaintenanceRecoveryState != importMaintenanceSnapshotRestored || journal.RecoveryState != "safe_to_resume_or_cleanup" {
		t.Fatalf("journal=%+v err=%v", journal, err)
	}
}

func TestRecoverImportPhaseANoEffectRejectsDiskDrift(t *testing.T) {
	f := preparePhaseATestFixture(t, "swap_host_to")
	f.interceptFIFO(func(string) (paneldocker.CommandResult, error) { return paneldocker.CommandResult{}, nil })
	driver := New(f.fake, nil, nil, f.store)
	if err := driver.runImportPhaseA(context.Background(), f.instance, f.op, phaseATestPlatformID, nil, phaseATestOptions()); err == nil {
		t.Fatal("no-effect import unexpectedly succeeded")
	}
	journal, err := LoadImportJournal(f.dataDir, f.op)
	if err != nil {
		t.Fatal(err)
	}
	journal.MaintenanceStarted = true
	journal.MaintenanceRecoveryState = importMaintenanceManualRecovery
	journal.RecoveryState = "manual_required"
	if err := WriteImportJournal(f.dataDir, journal); err != nil {
		t.Fatal(err)
	}
	f.writeMain(t, "drift-after-restart")
	recoveries, err := RecoverImportTransactions(f.dataDir)
	if err != nil || len(recoveries) != 1 || recoveries[0].State != importRecoveryMaintenanceStopAndRestore {
		t.Fatalf("recoveries=%+v err=%v", recoveries, err)
	}
	if _, err := driver.recoverInterruptedImportMaintenance(context.Background(), f.instance, recoveries); err == nil {
		t.Fatal("restart recovery accepted drifted no-effect evidence")
	}
	journal, err = LoadImportJournal(f.dataDir, f.op)
	if err != nil || journal.MaintenanceRecoveryState != importMaintenanceManualRecovery || journal.RecoveryState != "manual_required" {
		t.Fatalf("journal=%+v err=%v", journal, err)
	}
}

func TestImportPhaseANoEffectLabelWithoutEvidenceRemainsManual(t *testing.T) {
	f := preparePhaseATestFixture(t, "swap_host_to")
	journal, err := LoadImportJournal(f.dataDir, f.op)
	if err != nil {
		t.Fatal(err)
	}
	journal.Stage = ImportStageSubmitted
	journal.MaintenanceStarted = false
	journal.MaintenanceRecoveryState = importMaintenanceSnapshotRestored
	journal.PhaseAFIFOWriteAttempted = true
	journal.UpstreamSubmitted = true
	journal.PhaseAOutcome = phaseAOutcomeNoEffect
	journal.PreSubmitEvidence = nil
	journal.PhaseAEvidence = nil
	if err := WriteImportJournal(f.dataDir, journal); err != nil {
		t.Fatal(err)
	}
	if importJournalProvesPhaseANoEffect(journal) {
		t.Fatal("no-effect label without composite evidence was accepted")
	}
	if err := CleanupUnsubmittedImport(f.dataDir, f.op); err == nil {
		t.Fatal("cleanup accepted a no-effect label without composite evidence")
	}
	if _, err := os.Stat(filepath.Join(savesDir(f.dataDir), "Saves", "Upload_1")); err != nil {
		t.Fatalf("manual-recovery target was changed: %v", err)
	}
}

func TestBuildImportPhaseACommandRejectsInjectionAndInvalidPlatformID(t *testing.T) {
	op := NewImportOperationID()
	journal := ImportJournal{OperationID: op, SaveName: "Good_1", HostHandling: "swap_host_to", PlatformIDFingerprint: platformFingerprint(op, phaseATestPlatformID)}
	if _, err := buildImportPhaseACommand(journal, phaseATestPlatformID); err != nil {
		t.Fatal(err)
	}
	for _, badSave := range []string{"Bad\ninfo", "Bad\tName", "--reload", `Bad"Name`, "Bad;info"} {
		journal.SaveName = badSave
		if _, err := buildImportPhaseACommand(journal, phaseATestPlatformID); err == nil {
			t.Fatalf("accepted save name %q", badSave)
		}
	}
	journal.SaveName = "Good_1"
	for _, badID := range []string{"", "0", "+1", " 1", "1\ninfo", "18446744073709551616"} {
		if _, err := buildImportPhaseACommand(journal, badID); err == nil {
			t.Fatalf("accepted platform ID %q", badID)
		}
	}
}
