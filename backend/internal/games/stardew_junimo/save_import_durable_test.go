package stardew_junimo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

type durableTestFixture struct {
	runtime     *activationTestRuntime
	driver      *Driver
	beforeXML   string
	afterXML    string
	submissions int
	outcomes    []CommandStatus
	outcomeCall int
	waitErr     error
}

type durableCommandStore struct {
	*importMaintenanceStore
	command storage.ControlCommand
}

func (s *durableCommandStore) GetControlCommand(context.Context, string) (storage.ControlCommand, error) {
	return s.command, nil
}

func prepareDurableTestFixture(t *testing.T, hostHandling string) *durableTestFixture {
	t.Helper()
	r := prepareActivationTestRuntime(t, hostHandling)
	beforeXML := `<SaveGame><player><name>Before</name></player><year>1</year><currentSeason>spring</currentSeason><dayOfMonth>1</dayOfMonth></SaveGame>`
	afterXML := `<SaveGame><player><name>After</name></player><year>1</year><currentSeason>spring</currentSeason><dayOfMonth>1</dayOfMonth></SaveGame>`
	writeDurableMain(t, r.fixture.dataDir, beforeXML)
	r.setRuntimeSave(t, "Upload_1")
	if err := os.WriteFile(filepath.Join(controlDir(r.fixture.dataDir), "status.json"), []byte(`{"saveId":"Upload_1","commandResultVersion":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	r.clearPending(t)
	if hostHandling == "swap_host_to" {
		r.finalizeCount = 1
	}
	d := New(r.fixture.fake, nil, nil, r.fixture.store)
	if err := d.runImportActivation(context.Background(), r.fixture.instance, r.fixture.op, nil, activationTestOptions()); err != nil {
		t.Fatal(err)
	}
	return &durableTestFixture{runtime: r, driver: d, beforeXML: beforeXML, afterXML: afterXML}
}

func writeDurableMain(t *testing.T, dataDir, content string) {
	t.Helper()
	path := filepath.Join(savesDir(dataDir), "Saves", "Upload_1", "Upload_1")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Add(time.Duration(len(content)) * time.Millisecond)
	if err := os.Chtimes(path, now, now); err != nil {
		t.Fatal(err)
	}
}

func (f *durableTestFixture) options(t *testing.T) importDurableSaveOptions {
	t.Helper()
	return importDurableSaveOptions{
		CommandTimeout: 20 * time.Millisecond, TransitionTimeout: 20 * time.Millisecond, PollInterval: time.Millisecond,
		SubmitCommand: func(dataDir, commandID string) error {
			f.submissions++
			if err := writePanelCommandWithID(dataDir, commandID, "save-now", nil); err != nil {
				return err
			}
			writeDurableMain(t, dataDir, f.afterXML)
			return nil
		},
		GetOutcome: func(_ string, commandID string) (CommandOutcome, error) {
			status := CommandStatusSucceeded
			if len(f.outcomes) > 0 {
				index := f.outcomeCall
				if index >= len(f.outcomes) {
					index = len(f.outcomes) - 1
				}
				status = f.outcomes[index]
				f.outcomeCall++
			}
			return CommandOutcome{CommandID: commandID, Status: status, UpdatedAt: time.Now().UTC()}, nil
		},
		WaitTransition: func(context.Context, commandExecutor, string, int64, time.Duration, time.Duration) error {
			return f.waitErr
		},
	}
}

func TestImportDurableFinalizeSaveNowSavedAndCompleted(t *testing.T) {
	f := prepareDurableTestFixture(t, "swap_host_to")
	f.outcomes = []CommandStatus{CommandStatusQueued, CommandStatusUnknown, CommandStatusRunning, CommandStatusSucceeded}
	eventDir := saveEventsDir(f.runtime.fixture.dataDir)
	if err := os.MkdirAll(eventDir, 0o700); err != nil {
		t.Fatal(err)
	}
	eventPath := filepath.Join(eventDir, "saved-event.json")
	if err := os.WriteFile(eventPath, []byte(`{"saveName":"Upload_1"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := f.driver.runImportDurableSave(context.Background(), f.runtime.fixture.instance, f.runtime.fixture.op, nil, f.options(t)); err != nil {
		t.Fatal(err)
	}
	j, _ := LoadImportJournal(f.runtime.fixture.dataDir, f.runtime.fixture.op)
	if j.Stage != ImportStageCompleted || !j.DurableGameLoopSaved || j.DurableTransitionComplete == nil || !*j.DurableTransitionComplete || j.DurableSaveBefore == nil || j.DurableSaveAfter == nil {
		t.Fatalf("journal=%+v", j)
	}
	if !importSaveDiskChanged(*j.DurableSaveBefore, *j.DurableSaveAfter) || f.submissions != 1 || f.runtime.importWrites != 0 {
		t.Fatalf("change/submission/import mismatch: submissions=%d imports=%d", f.submissions, f.runtime.importWrites)
	}
	if _, err := os.Stat(eventPath); err != nil {
		t.Fatalf("save-event was consumed by durable-save stage: %v", err)
	}
	assertCompletedImportRuntimeRunning(t, f)
}

func TestImportDurableReadsResultArchivedByScheduler(t *testing.T) {
	f := prepareDurableTestFixture(t, "swap_host_to")
	commandID := "0123456789abcdef0123456789abcdef"
	now := time.Now().UTC()
	f.driver.store = &durableCommandStore{importMaintenanceStore: f.runtime.fixture.store, command: storage.ControlCommand{
		CommandID: commandID, InstanceID: f.runtime.fixture.instance.ID, CommandType: "save-now",
		Status: string(CommandStatusSucceeded), ResultSupported: true, SubmittedAt: now,
	}}
	outcome, err := f.driver.importCommandOutcome(context.Background(), f.runtime.fixture.instance.ID, f.runtime.fixture.dataDir, commandID)
	if err != nil || outcome.Status != CommandStatusSucceeded {
		t.Fatalf("persisted outcome=%+v err=%v", outcome, err)
	}
}

func TestImportDurableXMLAllowsLeadingUTF8BOM(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Save")
	if err := os.WriteFile(path, []byte("\xEF\xBB\xBF<SaveGame></SaveGame>"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateImportMainSaveXML(path); err != nil {
		t.Fatalf("Stardew UTF-8 BOM was rejected: %v", err)
	}
}

func TestImportDurableDayTransitionTrueWithoutSavedDoesNotComplete(t *testing.T) {
	f := prepareDurableTestFixture(t, "swap_host_to")
	f.outcomes = []CommandStatus{CommandStatusRunning}
	err := f.driver.runImportDurableSave(context.Background(), f.runtime.fixture.instance, f.runtime.fixture.op, nil, f.options(t))
	assertImportActivationCode(t, err, ImportErrorResultUnconfirmed)
	j, _ := LoadImportJournal(f.runtime.fixture.dataDir, f.runtime.fixture.op)
	if j.Stage == ImportStageCompleted || j.DurableGameLoopSaved {
		t.Fatalf("dayTransitionComplete alone completed transaction: %+v", j)
	}
}

func TestImportDurableSavedTransitionTimeoutKeepsRuntime(t *testing.T) {
	f := prepareDurableTestFixture(t, "swap_host_to")
	f.waitErr = context.DeadlineExceeded
	err := f.driver.runImportDurableSave(context.Background(), f.runtime.fixture.instance, f.runtime.fixture.op, nil, f.options(t))
	assertImportActivationCode(t, err, ImportErrorResultUnconfirmed)
	j, _ := LoadImportJournal(f.runtime.fixture.dataDir, f.runtime.fixture.op)
	if !j.DurableGameLoopSaved || j.DurableSaveWarning != durableSaveWarningTransitionTimeout || j.Stage != ImportStageSavePersisting {
		t.Fatalf("journal=%+v", j)
	}
	f.runtime.fixture.record.mu.Lock()
	down := f.runtime.fixture.record.down
	f.runtime.fixture.record.mu.Unlock()
	if down || f.runtime.restarts != 0 {
		t.Fatalf("runtime stopped/restarted after Saved: down=%v restarts=%d", down, f.runtime.restarts)
	}
}

func TestImportDurableSavedMissingTransitionFieldIsUnconfirmed(t *testing.T) {
	f := prepareDurableTestFixture(t, "swap_host_to")
	f.waitErr = &ImportEvidenceError{Code: "status_field_missing", Message: "missing dayTransitionComplete"}
	err := f.driver.runImportDurableSave(context.Background(), f.runtime.fixture.instance, f.runtime.fixture.op, nil, f.options(t))
	assertImportActivationCode(t, err, ImportErrorResultUnconfirmed)
	j, _ := LoadImportJournal(f.runtime.fixture.dataDir, f.runtime.fixture.op)
	if !j.DurableGameLoopSaved || j.DurableSaveWarning != durableSaveWarningTransitionMissing || j.Stage == ImportStageCompleted {
		t.Fatalf("journal=%+v", j)
	}
}

func TestImportDurableSaveNowSubmissionFailureDoesNotStopOrRestart(t *testing.T) {
	f := prepareDurableTestFixture(t, "swap_host_to")
	options := f.options(t)
	options.SubmitCommand = func(string, string) error { return errors.New("injected submission failure") }
	err := f.driver.runImportDurableSave(context.Background(), f.runtime.fixture.instance, f.runtime.fixture.op, nil, options)
	assertImportActivationCode(t, err, ImportErrorRecoveryRequired)
	j, _ := LoadImportJournal(f.runtime.fixture.dataDir, f.runtime.fixture.op)
	if !j.DurableSaveSubmissionFailed {
		t.Fatalf("submission failure was not journaled: %+v", j)
	}
	recoveries, recoveryErr := RecoverImportTransactions(f.runtime.fixture.dataDir)
	if recoveryErr != nil || len(recoveries) != 1 || recoveries[0].State != "manual_required" || recoveries[0].ErrorCode != ImportErrorRecoveryRequired {
		t.Fatalf("submission failure recovery=%+v err=%v", recoveries, recoveryErr)
	}
	f.runtime.fixture.record.mu.Lock()
	down := f.runtime.fixture.record.down
	f.runtime.fixture.record.mu.Unlock()
	if down || f.runtime.restarts != 0 || f.runtime.importWrites != 0 {
		t.Fatalf("destructive action after submission failure: down=%v restarts=%d imports=%d", down, f.runtime.restarts, f.runtime.importWrites)
	}
}

func TestImportDurableCommandUnknownIsUnconfirmed(t *testing.T) {
	f := prepareDurableTestFixture(t, "swap_host_to")
	f.outcomes = []CommandStatus{CommandStatusUnknown}
	err := f.driver.runImportDurableSave(context.Background(), f.runtime.fixture.instance, f.runtime.fixture.op, nil, f.options(t))
	assertImportActivationCode(t, err, ImportErrorResultUnconfirmed)
}

func TestImportDurableCommandFailedRequiresRecoveryAndKeepsRuntime(t *testing.T) {
	f := prepareDurableTestFixture(t, "swap_host_to")
	f.outcomes = []CommandStatus{CommandStatusRunning, CommandStatusFailed}
	err := f.driver.runImportDurableSave(context.Background(), f.runtime.fixture.instance, f.runtime.fixture.op, nil, f.options(t))
	assertImportActivationCode(t, err, ImportErrorRecoveryRequired)
	f.runtime.fixture.record.mu.Lock()
	down := f.runtime.fixture.record.down
	f.runtime.fixture.record.mu.Unlock()
	if down || f.runtime.restarts != 0 {
		t.Fatalf("runtime changed after command failure: down=%v restarts=%d", down, f.runtime.restarts)
	}
}

func TestImportDurableSavedWithInvalidXMLRequiresRecovery(t *testing.T) {
	f := prepareDurableTestFixture(t, "swap_host_to")
	f.afterXML = `<SaveGame><player>`
	err := f.driver.runImportDurableSave(context.Background(), f.runtime.fixture.instance, f.runtime.fixture.op, nil, f.options(t))
	assertImportActivationCode(t, err, ImportErrorRecoveryRequired)
	j, _ := LoadImportJournal(f.runtime.fixture.dataDir, f.runtime.fixture.op)
	if j.Stage == ImportStageCompleted {
		t.Fatalf("invalid XML passed completed gate")
	}
}

func TestImportDurableSavedWithoutFileChangeFailsCompletedGate(t *testing.T) {
	f := prepareDurableTestFixture(t, "swap_host_to")
	options := f.options(t)
	options.SubmitCommand = func(dataDir, commandID string) error {
		f.submissions++
		return writePanelCommandWithID(dataDir, commandID, "save-now", nil)
	}
	err := f.driver.runImportDurableSave(context.Background(), f.runtime.fixture.instance, f.runtime.fixture.op, nil, options)
	assertImportActivationCode(t, err, ImportErrorRecoveryRequired)
	j, _ := LoadImportJournal(f.runtime.fixture.dataDir, f.runtime.fixture.op)
	if j.Stage == ImportStageCompleted {
		t.Fatalf("unchanged file passed completed gate")
	}
}

func TestImportDurableAsIsDoesNotRewriteSave(t *testing.T) {
	f := prepareDurableTestFixture(t, "server_owns_original")
	before, err := stableFileSHA256(filepath.Join(savesDir(f.runtime.fixture.dataDir), "Saves", "Upload_1", "Upload_1"))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.driver.runImportDurableSave(context.Background(), f.runtime.fixture.instance, f.runtime.fixture.op, nil, f.options(t)); err != nil {
		t.Fatal(err)
	}
	after, _ := stableFileSHA256(filepath.Join(savesDir(f.runtime.fixture.dataDir), "Saves", "Upload_1", "Upload_1"))
	j, _ := LoadImportJournal(f.runtime.fixture.dataDir, f.runtime.fixture.op)
	if before != after || f.submissions != 0 || j.Stage != ImportStageCompleted || j.DurableSaveCommandID != "" {
		t.Fatalf("as-is was rewritten: before=%s after=%s submissions=%d journal=%+v", before, after, f.submissions, j)
	}
	assertCompletedImportRuntimeRunning(t, f)
}

func assertCompletedImportRuntimeRunning(t *testing.T, f *durableTestFixture) {
	t.Helper()
	instance, err := f.runtime.fixture.store.GetInstance(context.Background(), f.runtime.fixture.instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if instance.State != storage.InstanceStateRunning || instance.DriverPhase != "running" {
		t.Fatalf("completed import runtime state=%q phase=%q", instance.State, instance.DriverPhase)
	}
}

func TestImportDurablePanelRestartResumesSameCommandWithoutResubmit(t *testing.T) {
	f := prepareDurableTestFixture(t, "swap_host_to")
	f.outcomes = []CommandStatus{CommandStatusRunning}
	options := f.options(t)
	options.CommandTimeout = 5 * time.Millisecond
	err := f.driver.runImportDurableSave(context.Background(), f.runtime.fixture.instance, f.runtime.fixture.op, nil, options)
	assertImportActivationCode(t, err, ImportErrorResultUnconfirmed)
	j, _ := LoadImportJournal(f.runtime.fixture.dataDir, f.runtime.fixture.op)
	commandID := j.DurableSaveCommandID
	if commandID == "" || f.submissions != 1 {
		t.Fatalf("first submission not journaled: %+v submissions=%d", j, f.submissions)
	}
	f.outcomes = []CommandStatus{CommandStatusSucceeded}
	f.outcomeCall = 0
	if err := f.driver.runImportDurableSave(context.Background(), f.runtime.fixture.instance, f.runtime.fixture.op, nil, f.options(t)); err != nil {
		t.Fatal(err)
	}
	j, _ = LoadImportJournal(f.runtime.fixture.dataDir, f.runtime.fixture.op)
	if f.submissions != 1 || j.DurableSaveCommandID != commandID || j.Stage != ImportStageCompleted {
		t.Fatalf("resume resubmitted or changed ID: submissions=%d journal=%+v", f.submissions, j)
	}
}

func TestImportDurableWaitStatusFalseThenTrue(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dataDir, ".env"), []byte("API_PORT=5110\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	calls := 0
	fake := &fakeConsoleDocker{}
	fake.execFunc = func(_ context.Context, _, _ string, _ string, args ...string) (paneldocker.CommandResult, error) {
		if len(args) == 0 || args[0] != "curl" || !strings.Contains(args[len(args)-1], "/wait/status?") {
			return paneldocker.CommandResult{ExitCode: 1}, nil
		}
		calls++
		if calls == 1 {
			return paneldocker.CommandResult{Stdout: `{"dayTransitionComplete":false,"version":11}` + "\n200"}, nil
		}
		return paneldocker.CommandResult{Stdout: `{"dayTransitionComplete":true,"version":12}` + "\n200"}, nil
	}
	if err := waitForJunimoDayTransitionComplete(context.Background(), fake, dataDir, 10, 100*time.Millisecond, time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("wait calls=%d", calls)
	}
}
