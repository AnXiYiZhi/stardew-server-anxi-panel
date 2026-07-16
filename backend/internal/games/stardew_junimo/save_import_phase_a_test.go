package stardew_junimo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
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
	if !ok || typed.Code != ImportErrorCommandFailed {
		t.Fatalf("error=%v", err)
	}
	journal, _ := LoadImportJournal(f.dataDir, f.op)
	if journal.UpstreamSubmitted || journal.Stage != ImportStageRuntimeReady || strings.Contains(journal.PhaseALogDetail, phaseATestPlatformID) || !strings.Contains(journal.PhaseALogDetail, "[redacted-platform-id]") {
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

func TestImportPhaseANoEffectCanBeSafelyCleanedWithoutRetry(t *testing.T) {
	f := preparePhaseATestFixture(t, "swap_host_to")
	f.interceptFIFO(func(string) (paneldocker.CommandResult, error) {
		return paneldocker.CommandResult{Stdout: "success text only"}, nil
	})
	d := New(f.fake, nil, nil, f.store)
	err := d.runImportPhaseA(context.Background(), f.instance, f.op, phaseATestPlatformID, nil, phaseATestOptions())
	typed, ok := AsImportTransactionError(err)
	if !ok || typed.Code != ImportErrorCommandFailed || f.teeCalls != 1 {
		t.Fatalf("error=%v writes=%d", err, f.teeCalls)
	}
	if err := CleanupUnsubmittedImport(f.dataDir, f.op); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(savesDir(f.dataDir), "Saves", "Upload_1")); !os.IsNotExist(err) {
		t.Fatalf("staged target still exists: %v", err)
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
