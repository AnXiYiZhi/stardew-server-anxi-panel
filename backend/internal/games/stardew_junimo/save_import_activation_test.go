package stardew_junimo

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

type activationTestRuntime struct {
	fixture           *phaseATestFixture
	identity          JunimoProcessIdentity
	finalizeCount     int
	masterName        string
	diagnosticsFailed bool
	diagnosticsDown   bool
	online            bool
	ready             bool
	dayComplete       bool
	players           int
	version           int64
	restarts          int
	restartServices   []string
	importWrites      int
	onRestart         func()
}

func prepareActivationTestRuntime(t *testing.T, hostHandling string) *activationTestRuntime {
	t.Helper()
	f := preparePhaseATestFixture(t, hostHandling)
	j, err := LoadImportJournal(f.dataDir, f.op)
	if err != nil {
		t.Fatal(err)
	}
	zero := 0
	identity := JunimoProcessIdentity{ContainerID: "container-a", ProcessStartTicks: "123"}
	j.Stage = ImportStageConfirmed
	j.UpstreamSubmitted = true
	j.UpstreamConfirmed = true
	j.PreSubmitEvidence = &JunimoImportEvidenceSnapshot{
		MainSaveSHA256: f.preHash, ActivePointer: "Old_1", FinalizeCount: &zero, ProcessIdentity: &identity,
	}
	j.MaintenanceStarted = true
	if err := WriteImportJournal(f.dataDir, j); err != nil {
		t.Fatal(err)
	}
	runtime := &activationTestRuntime{fixture: f, identity: identity, masterName: "Server", online: true, ready: true, dayComplete: true, version: 10}
	baseExec := f.fake.execFunc
	f.fake.execFunc = func(ctx context.Context, dir, service, stdin string, args ...string) (paneldocker.CommandResult, error) {
		if strings.HasPrefix(stdin, "saves import ") {
			runtime.importWrites++
		}
		joined := strings.Join(args, " ")
		if joined == "sh -lc hostname; awk '{print $22}' /proc/1/stat" {
			return paneldocker.CommandResult{Stdout: runtime.identity.ContainerID + "\n" + runtime.identity.ProcessStartTicks + "\n"}, nil
		}
		if len(args) > 0 && args[0] == "curl" {
			endpoint := args[len(args)-1]
			if strings.HasSuffix(endpoint, "/status") {
				return paneldocker.CommandResult{Stdout: `{"playerCount":` + strconv.Itoa(runtime.players) + `,"isOnline":` + strconv.FormatBool(runtime.online) + `,"isReady":` + strconv.FormatBool(runtime.ready) + `,"dayTransitionComplete":` + strconv.FormatBool(runtime.dayComplete) + `,"version":` + strconv.FormatInt(runtime.version, 10) + `}`}, nil
			}
			if strings.HasSuffix(endpoint, "/diagnostics/state") {
				if runtime.diagnosticsDown {
					return paneldocker.CommandResult{ExitCode: 7}, nil
				}
				failed := "[]"
				if runtime.diagnosticsFailed {
					failed = `["saveImportFinalizeCount"]`
				}
				return paneldocker.CommandResult{Stdout: `{"saveImportFinalizeCount":` + strconv.Itoa(runtime.finalizeCount) + `,"masterName":"` + runtime.masterName + `","failedFields":` + failed + `}`}, nil
			}
		}
		return baseExec(ctx, dir, service, stdin, args...)
	}
	f.fake.recreateFunc = func(_ context.Context, _ string, services ...string) (paneldocker.CommandResult, error) {
		runtime.restarts++
		runtime.restartServices = append([]string(nil), services...)
		f.record.mu.Lock()
		f.record.started = true
		f.record.mu.Unlock()
		if runtime.onRestart != nil {
			runtime.onRestart()
		}
		return paneldocker.CommandResult{}, nil
	}
	return runtime
}

func (r *activationTestRuntime) setRuntimeSave(t *testing.T, saveName string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(controlDir(r.fixture.dataDir), "status.json"), []byte(`{"saveId":"`+saveName+`","hostBed":{"state":"healthy","healthy":true,"houseUpgradeLevel":0,"expectedBedType":"Single","actualBedType":"Single","bedTileX":9,"bedTileY":8,"playerBedSpotX":10,"playerBedSpotY":9}}`), 0o600); err != nil {
		t.Fatal(err)
	}
}

func (r *activationTestRuntime) clearPending(t *testing.T) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(saveImportIntentPath(r.fixture.dataDir)), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(saveImportIntentPath(r.fixture.dataDir), []byte(`{"Pending":null}`), 0o600); err != nil {
		t.Fatal(err)
	}
}

func activationTestOptions() importActivationOptions {
	return importActivationOptions{ReloadTimeout: 12 * time.Millisecond, RestartTimeout: 100 * time.Millisecond, PollInterval: time.Millisecond}
}

func TestImportActivationReloadDirectSwapSameProcessCountPlusOne(t *testing.T) {
	r := prepareActivationTestRuntime(t, "swap_host_to")
	r.setRuntimeSave(t, "Upload_1")
	r.clearPending(t)
	r.finalizeCount = 1
	d := New(r.fixture.fake, nil, nil, r.fixture.store)
	if err := d.runImportActivation(context.Background(), r.fixture.instance, r.fixture.op, nil, activationTestOptions()); err != nil {
		t.Fatal(err)
	}
	j, _ := LoadImportJournal(r.fixture.dataDir, r.fixture.op)
	if j.Stage != ImportStageFinalizeConfirmed || j.ActivationOutcome != activationOutcomeSwapFinalized || r.restarts != 0 {
		t.Fatalf("journal=%+v restarts=%d", j, r.restarts)
	}
	if r.importWrites != 0 {
		t.Fatalf("import command was repeated %d time(s)", r.importWrites)
	}
}

func TestImportActivationReloadSkippedThenControlledRestartNewProcessCountOne(t *testing.T) {
	r := prepareActivationTestRuntime(t, "swap_host_to")
	r.fixture.writePending(t, "Upload_1", 99, phaseATestPlatformID)
	r.onRestart = func() {
		r.identity = JunimoProcessIdentity{ContainerID: "container-b", ProcessStartTicks: "456"}
		r.finalizeCount = 1
		r.setRuntimeSave(t, "Upload_1")
		r.clearPending(t)
	}
	d := New(r.fixture.fake, nil, nil, r.fixture.store)
	if err := d.runImportActivation(context.Background(), r.fixture.instance, r.fixture.op, nil, activationTestOptions()); err != nil {
		t.Fatal(err)
	}
	j, _ := LoadImportJournal(r.fixture.dataDir, r.fixture.op)
	if !j.ActivationRestarted || r.restarts != 1 || j.Stage != ImportStageFinalizeConfirmed {
		t.Fatalf("journal=%+v restarts=%d", j, r.restarts)
	}
}

func TestImportActivationDisabledColdStartDoesNotMaterializeSteamAuth(t *testing.T) {
	r := prepareActivationTestRuntime(t, "swap_host_to")
	r.fixture.writePending(t, "Upload_1", 99, phaseATestPlatformID)
	if err := sjconfig.UpdateEnvFile(filepath.Join(r.fixture.dataDir, ".env"), map[string]string{"STEAM_INVITE_ENABLED": "true"}); err != nil {
		t.Fatalf("change live invite intent after transaction freeze: %v", err)
	}
	r.fixture.record.mu.Lock()
	r.fixture.record.started = false
	r.fixture.record.mu.Unlock()
	r.onRestart = func() {
		r.identity = JunimoProcessIdentity{ContainerID: "container-b", ProcessStartTicks: "456"}
		r.finalizeCount = 1
		r.setRuntimeSave(t, "Upload_1")
		r.clearPending(t)
	}
	driver := New(r.fixture.fake, nil, nil, r.fixture.store)
	if err := driver.runImportActivation(context.Background(), r.fixture.instance, r.fixture.op, nil, activationTestOptions()); err != nil {
		t.Fatalf("runImportActivation: %v", err)
	}
	if got := strings.Join(r.restartServices, ","); got != "server" {
		t.Fatalf("cold activation services = %v, want frozen disabled server-only scope", r.restartServices)
	}
}

func TestImportActivationEnabledColdStartUsesFrozenAuthScopeAfterEnvChanges(t *testing.T) {
	r := prepareActivationTestRuntime(t, "swap_host_to")
	r.fixture.writePending(t, "Upload_1", 99, phaseATestPlatformID)
	j, err := LoadImportJournal(r.fixture.dataDir, r.fixture.op)
	if err != nil {
		t.Fatal(err)
	}
	enabled := true
	j.MaintenanceSteamInviteEnabled = &enabled
	if err := WriteImportJournal(r.fixture.dataDir, j); err != nil {
		t.Fatal(err)
	}
	if err := sjconfig.UpdateEnvFile(filepath.Join(r.fixture.dataDir, ".env"), map[string]string{"STEAM_INVITE_ENABLED": "false"}); err != nil {
		t.Fatalf("change live invite intent after transaction freeze: %v", err)
	}
	r.fixture.record.mu.Lock()
	r.fixture.record.started = false
	r.fixture.record.mu.Unlock()
	r.onRestart = func() {
		r.identity = JunimoProcessIdentity{ContainerID: "container-b", ProcessStartTicks: "456"}
		r.finalizeCount = 1
		r.setRuntimeSave(t, "Upload_1")
		r.clearPending(t)
	}
	driver := New(r.fixture.fake, nil, nil, r.fixture.store)
	if err := driver.runImportActivation(context.Background(), r.fixture.instance, r.fixture.op, nil, activationTestOptions()); err != nil {
		t.Fatalf("runImportActivation: %v", err)
	}
	if got := strings.Join(r.restartServices, ","); got != "steam-auth,server" {
		t.Fatalf("cold activation services = %v, want frozen enabled Auth+server scope", r.restartServices)
	}
}

func TestConfirmedImportStepPanicsRollBackFrozenEnabledScope(t *testing.T) {
	for _, tc := range []struct {
		name       string
		activation func() error
		durable    func() error
	}{
		{name: "activation", activation: func() error { panic("sensitive activation panic") }},
		{name: "durable save", activation: func() error { return nil }, durable: func() error { panic("sensitive durable panic") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := prepareActivationTestRuntime(t, "swap_host_to")
			r.fixture.writeMain(t, "transformed")
			r.fixture.writePointer(t, "Upload_1")
			journal, err := LoadImportJournal(r.fixture.dataDir, r.fixture.op)
			if err != nil {
				t.Fatal(err)
			}
			enabled := true
			journal.MaintenanceSteamInviteEnabled = &enabled
			if err := WriteImportJournal(r.fixture.dataDir, journal); err != nil {
				t.Fatal(err)
			}
			var stoppedServices []string
			r.fixture.fake.stopServicesFunc = func(_ context.Context, _, _ string, services ...string) error {
				stoppedServices = append([]string(nil), services...)
				r.fixture.record.mu.Lock()
				r.fixture.record.down = true
				r.fixture.record.started = false
				r.fixture.record.mu.Unlock()
				return nil
			}
			driver := New(r.fixture.fake, nil, nil, r.fixture.store)
			err = driver.runConfirmedImportSteps(r.fixture.instance, r.fixture.op, nil, tc.activation, tc.durable)
			typed, ok := AsImportTransactionError(err)
			if !ok || typed.Code != ImportErrorRecoveryRequired || strings.Contains(err.Error(), "sensitive") {
				t.Fatalf("panic recovery error=%T %v, want redacted recovery_required", err, err)
			}
			if got := strings.Join(stoppedServices, ","); got != "server,steam-auth" {
				t.Fatalf("panic rollback stop services=%v, want frozen enabled server+Auth scope", stoppedServices)
			}
			journal, err = LoadImportJournal(r.fixture.dataDir, r.fixture.op)
			if err != nil || journal.Stage != ImportStageRolledBack || journal.RollbackState != importActivationRollbackCompleted || journal.MaintenanceStarted {
				t.Fatalf("panic rollback journal=%+v err=%v", journal, err)
			}
		})
	}
}

func TestConfirmedAsIsImportPanicStopsFrozenDisabledScopeAndStaysManual(t *testing.T) {
	r := prepareActivationTestRuntime(t, "server_owns_original")
	var stoppedServices []string
	r.fixture.fake.stopServicesFunc = func(_ context.Context, _, _ string, services ...string) error {
		stoppedServices = append([]string(nil), services...)
		r.fixture.record.mu.Lock()
		r.fixture.record.down = true
		r.fixture.record.started = false
		r.fixture.record.mu.Unlock()
		return nil
	}
	driver := New(r.fixture.fake, nil, nil, r.fixture.store)
	err := driver.runConfirmedImportSteps(r.fixture.instance, r.fixture.op, nil, func() error {
		panic("sensitive as-is panic")
	}, nil)
	typed, ok := AsImportTransactionError(err)
	if !ok || typed.Code != ImportErrorRecoveryRequired || strings.Contains(err.Error(), "sensitive") {
		t.Fatalf("panic recovery error=%T %v, want redacted recovery_required", err, err)
	}
	if got := strings.Join(stoppedServices, ","); got != "server" {
		t.Fatalf("as-is panic stop services=%v, want frozen disabled server-only scope", stoppedServices)
	}
	journal, loadErr := LoadImportJournal(r.fixture.dataDir, r.fixture.op)
	if loadErr != nil || journal.MaintenanceRecoveryState != importMaintenanceManualRecovery || journal.RecoveryState != "manual_required" || !journal.MaintenanceStarted {
		t.Fatalf("as-is panic journal=%+v err=%v", journal, loadErr)
	}
}

func TestConfirmedImportPanicAfterCompletedJournalPublishesRunningWithoutRollback(t *testing.T) {
	r := prepareActivationTestRuntime(t, "swap_host_to")
	r.fixture.store.mu.Lock()
	r.fixture.store.instance.State = storage.InstanceStateStopped
	r.fixture.store.instance.StateMessage = sql.NullString{String: "private maintenance", Valid: true}
	r.fixture.store.instance.DriverPhase = importMaintenancePhase
	r.fixture.store.instance.DriverPayload = `{"save_import_operation_id":"` + r.fixture.op + `"}`
	r.fixture.store.mu.Unlock()
	stopCalls := 0
	r.fixture.fake.stopServicesFunc = func(context.Context, string, string, ...string) error {
		stopCalls++
		return nil
	}
	driver := New(r.fixture.fake, nil, nil, r.fixture.store)
	err := driver.runConfirmedImportSteps(r.fixture.instance, r.fixture.op, nil, nil, func() error {
		journal, loadErr := LoadImportJournal(r.fixture.dataDir, r.fixture.op)
		if loadErr != nil {
			return loadErr
		}
		journal.Stage = ImportStageCompleted
		if writeErr := WriteImportJournal(r.fixture.dataDir, journal); writeErr != nil {
			return writeErr
		}
		panic("sensitive post-completion panic")
	})
	if err != nil {
		t.Fatalf("post-completion panic was not reconciled: %v", err)
	}
	if stopCalls != 0 {
		t.Fatalf("post-completion panic triggered rollback stop %d time(s)", stopCalls)
	}
	journal, loadErr := LoadImportJournal(r.fixture.dataDir, r.fixture.op)
	if loadErr != nil || journal.Stage != ImportStageCompleted || journal.RollbackState != "" {
		t.Fatalf("completed journal was rolled back: %+v err=%v", journal, loadErr)
	}
	r.fixture.store.mu.Lock()
	current := r.fixture.store.instance
	r.fixture.store.mu.Unlock()
	if current.State != storage.InstanceStateRunning || current.DriverPhase != "running" || strings.Contains(current.DriverPayload, saveImportOperationIDPayloadKey) {
		t.Fatalf("completed journal did not publish running state: %+v", current)
	}
}

func TestImportActivationAsIsDoesNotRequireFinalizeCountIncrease(t *testing.T) {
	r := prepareActivationTestRuntime(t, "server_owns_original")
	r.setRuntimeSave(t, "Upload_1")
	r.clearPending(t)
	r.finalizeCount = 0
	d := New(r.fixture.fake, nil, nil, r.fixture.store)
	if err := d.runImportActivation(context.Background(), r.fixture.instance, r.fixture.op, nil, activationTestOptions()); err != nil {
		t.Fatal(err)
	}
	j, _ := LoadImportJournal(r.fixture.dataDir, r.fixture.op)
	if j.ActivationOutcome != activationOutcomeAsIsLoaded {
		t.Fatalf("journal=%+v", j)
	}
}

func TestImportActivationAsIsReloadSkippedUsesOneRestart(t *testing.T) {
	r := prepareActivationTestRuntime(t, "server_owns_original")
	r.clearPending(t)
	r.onRestart = func() {
		journal, err := LoadImportJournal(r.fixture.dataDir, r.fixture.op)
		if err != nil || journal.Stage != ImportStageSaveActivating {
			t.Fatalf("restart occurred before save_activating journal: journal=%+v err=%v", journal, err)
		}
		r.identity = JunimoProcessIdentity{ContainerID: "container-b", ProcessStartTicks: "456"}
		r.setRuntimeSave(t, "Upload_1")
	}
	d := New(r.fixture.fake, nil, nil, r.fixture.store)
	if err := d.runImportActivation(context.Background(), r.fixture.instance, r.fixture.op, nil, activationTestOptions()); err != nil {
		t.Fatal(err)
	}
	if r.restarts != 1 || r.importWrites != 0 {
		t.Fatalf("restarts=%d importWrites=%d", r.restarts, r.importWrites)
	}
}

func TestImportActivationPendingClearedWithoutCountRequiresRecovery(t *testing.T) {
	r := prepareActivationTestRuntime(t, "swap_host_to")
	r.setRuntimeSave(t, "Upload_1")
	r.clearPending(t)
	d := New(r.fixture.fake, nil, nil, r.fixture.store)
	err := d.runImportActivation(context.Background(), r.fixture.instance, r.fixture.op, nil, activationTestOptions())
	assertImportActivationCode(t, err, ImportErrorRecoveryRequired)
	if r.restarts != 0 {
		t.Fatalf("partial finalizer triggered restart")
	}
}

func TestImportActivationSwapRejectsMissingHostBed(t *testing.T) {
	r := prepareActivationTestRuntime(t, "swap_host_to")
	if err := os.WriteFile(
		filepath.Join(controlDir(r.fixture.dataDir), "status.json"),
		[]byte(`{"saveId":"Upload_1","hostBed":{"state":"missing","healthy":false,"errorCode":"host_bed_missing","failureReason":"host_bed_layout_unresolved","houseUpgradeLevel":0}}`),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	r.clearPending(t)
	r.finalizeCount = 1
	d := New(r.fixture.fake, nil, nil, r.fixture.store)
	err := d.runImportActivation(context.Background(), r.fixture.instance, r.fixture.op, nil, activationTestOptions())
	assertImportActivationCode(t, err, ImportErrorHostBedMissing)
	j, _ := LoadImportJournal(r.fixture.dataDir, r.fixture.op)
	if j.Stage == ImportStageFinalizeConfirmed || j.ActivationEvidence == nil || !activationHostBedFault(*j.ActivationEvidence) {
		t.Fatalf("missing host bed was accepted: %+v", j)
	}
}

func TestConfirmedHostSwapFailureRollsBackCompleteSaveAndTransaction(t *testing.T) {
	r := prepareActivationTestRuntime(t, "swap_host_to")
	saveDir := filepath.Join(savesDir(r.fixture.dataDir), "Saves", "Upload_1")
	for name, value := range map[string]string{
		"Upload_1":     "converted-main",
		"Upload_1_old": "converted-old",
		"SaveGameInfo": "converted-info",
	} {
		if err := os.WriteFile(filepath.Join(saveDir, name), []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	r.fixture.writePointer(t, "Upload_1")
	r.fixture.store.mu.Lock()
	r.fixture.store.instance.State = "running"
	r.fixture.store.instance.DriverPhase = importMaintenancePhase
	r.fixture.store.instance.StateMessage.String = "host bed failure"
	r.fixture.store.mu.Unlock()

	d := New(r.fixture.fake, nil, nil, r.fixture.store)
	primary := &ImportTransactionError{Code: ImportErrorHostBedMissing, Message: "missing host bed"}
	err := d.rollbackConfirmedHostSwapFailure(r.fixture.dataDir, r.fixture.op, nil, primary)
	assertImportActivationCode(t, err, ImportErrorHostBedMissing)

	want := map[string]string{
		"Upload_1":     "main-save",
		"Upload_1_old": "old-save",
		"SaveGameInfo": "info",
	}
	for name, expected := range want {
		actual, readErr := os.ReadFile(filepath.Join(saveDir, name))
		if readErr != nil || string(actual) != expected {
			t.Fatalf("restored %s=%q err=%v, want %q", name, actual, readErr, expected)
		}
	}
	if pointer, pointerErr := readActivePointerStrict(r.fixture.dataDir); pointerErr != nil || pointer != "Old_1" {
		t.Fatalf("active pointer=%q err=%v, want Old_1", pointer, pointerErr)
	}
	j, loadErr := LoadImportJournal(r.fixture.dataDir, r.fixture.op)
	if loadErr != nil || j.Stage != ImportStageRolledBack || j.RollbackState != importActivationRollbackCompleted ||
		j.RollbackRestoredSHA256 != r.fixture.preHash || j.RollbackRestoredFingerprint != j.StagedSaveFingerprint ||
		j.RolledBackAt == nil || j.MaintenanceStarted || j.RollbackCauseCode != ImportErrorHostBedMissing ||
		j.LastErrorCode != ImportErrorHostBedMissing {
		t.Fatalf("rollback journal=%+v err=%v", j, loadErr)
	}
	unfinished, unfinishedErr := HasUnfinishedImportTransaction(r.fixture.dataDir)
	if unfinishedErr != nil || unfinished {
		t.Fatalf("rolled-back import remained unfinished: unfinished=%v err=%v", unfinished, unfinishedErr)
	}
	r.fixture.store.mu.Lock()
	restoredInstance := r.fixture.store.instance
	r.fixture.store.mu.Unlock()
	if restoredInstance.State != "stopped" || restoredInstance.DriverPhase != "container_stopped" || restoredInstance.StateMessage.String != "stopped before import" {
		t.Fatalf("instance snapshot was not restored: %+v", restoredInstance)
	}
	entries, readErr := os.ReadDir(filepath.Dir(saveDir))
	if readErr != nil {
		t.Fatal(readErr)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".phase-a-") {
			t.Fatalf("rollback left temporary save path %q", entry.Name())
		}
	}
}

func TestConfirmedHostSwapDurableFailureUsesGenericRollback(t *testing.T) {
	r := prepareActivationTestRuntime(t, "swap_host_to")
	r.fixture.writePointer(t, "Upload_1")
	d := New(r.fixture.fake, nil, nil, r.fixture.store)
	primary := &ImportTransactionError{Code: ImportErrorResultUnconfirmed, Message: "durable save result was not confirmed"}

	err := d.rollbackConfirmedHostSwapFailure(r.fixture.dataDir, r.fixture.op, nil, primary)
	assertImportActivationCode(t, err, ImportErrorResultUnconfirmed)
	j, loadErr := LoadImportJournal(r.fixture.dataDir, r.fixture.op)
	if loadErr != nil || j.Stage != ImportStageRolledBack || j.RollbackState != importActivationRollbackCompleted ||
		j.RollbackCauseCode != ImportErrorResultUnconfirmed || j.LastErrorCode != ImportErrorResultUnconfirmed {
		t.Fatalf("generic rollback journal=%+v err=%v", j, loadErr)
	}
}

func TestPartialActivationRollbackNeverResumesActivation(t *testing.T) {
	for _, rollbackState := range []string{
		importActivationRollbackPending,
		importActivationRollbackServerStopped,
		importActivationRollbackSaveRestored,
		importActivationRollbackPointerRestored,
		importActivationRollbackInstanceRestored,
		importActivationRollbackManualRecovery,
	} {
		t.Run(rollbackState, func(t *testing.T) {
			r := prepareActivationTestRuntime(t, "swap_host_to")
			journal, err := LoadImportJournal(r.fixture.dataDir, r.fixture.op)
			if err != nil {
				t.Fatal(err)
			}
			journal.RollbackState = rollbackState
			if rollbackState == importActivationRollbackManualRecovery {
				journal.RecoveryState = "manual_required"
			}
			if err := WriteImportJournal(r.fixture.dataDir, journal); err != nil {
				t.Fatal(err)
			}
			recoveries, err := RecoverImportTransactions(r.fixture.dataDir)
			if err != nil || len(recoveries) != 1 || recoveries[0].State != "manual_required" || recoveries[0].ErrorCode != ImportErrorRecoveryRequired {
				t.Fatalf("partial rollback recovery=%+v err=%v, want manual recovery", recoveries, err)
			}
			persisted, err := LoadImportJournal(r.fixture.dataDir, r.fixture.op)
			if err != nil || persisted.RollbackState != rollbackState ||
				(rollbackState == importActivationRollbackManualRecovery && persisted.RecoveryState != "manual_required") {
				t.Fatalf("partial rollback journal was reclassified: %+v err=%v", persisted, err)
			}
		})
	}
}

func TestAsIsPostImportFailureDoesNotRunHostSwapRollback(t *testing.T) {
	r := prepareActivationTestRuntime(t, "as_is")
	d := New(r.fixture.fake, nil, nil, r.fixture.store)
	primary := &ImportTransactionError{Code: ImportErrorActivationTimeout, Message: "activation timed out"}

	err := d.rollbackConfirmedHostSwapFailure(r.fixture.dataDir, r.fixture.op, nil, primary)
	if err != primary {
		t.Fatalf("as-is failure=%v, want original error", err)
	}
	j, loadErr := LoadImportJournal(r.fixture.dataDir, r.fixture.op)
	if loadErr != nil || j.RollbackState != "" {
		t.Fatalf("as-is import unexpectedly entered host-swap rollback: %+v err=%v", j, loadErr)
	}
}

func TestImportActivationHostBedEvidenceSupportsEveryHouseLayout(t *testing.T) {
	for level := 0; level <= 3; level++ {
		expected := "Double"
		if level == 0 {
			expected = "Single"
		}
		healthy := true
		x, y := 20+level, 30+level
		evidence := JunimoImportActivationEvidence{HostBed: &ControlHostBedEvidence{
			State: "healthy", Healthy: &healthy, HouseUpgradeLevel: &level,
			ExpectedBedType: expected, ActualBedType: expected,
			BedTileX: &x, BedTileY: &y,
		}}
		spotX, spotY := x+1, y+1
		evidence.HostBed.PlayerBedSpotX, evidence.HostBed.PlayerBedSpotY = &spotX, &spotY
		if !activationHostBedHealthy(evidence) {
			t.Fatalf("level %d map-derived bed evidence was rejected: %+v", level, evidence.HostBed)
		}
	}
}

func TestImportActivationRuntimeSaveNeverSwitchesAndPendingPersists(t *testing.T) {
	r := prepareActivationTestRuntime(t, "swap_host_to")
	r.fixture.writePending(t, "Upload_1", 99, phaseATestPlatformID)
	d := New(r.fixture.fake, nil, nil, r.fixture.store)
	err := d.runImportActivation(context.Background(), r.fixture.instance, r.fixture.op, nil, activationTestOptions())
	assertImportActivationCode(t, err, ImportErrorActivationTimeout)
	if r.restarts != 1 {
		t.Fatalf("restarts=%d", r.restarts)
	}
}

func TestImportActivationDiagnosticsFailedOrUnavailableIsUnconfirmed(t *testing.T) {
	for _, test := range []struct {
		name string
		set  func(*activationTestRuntime)
	}{
		{"failed_fields", func(r *activationTestRuntime) { r.diagnosticsFailed = true }},
		{"api_unavailable", func(r *activationTestRuntime) { r.diagnosticsDown = true }},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := prepareActivationTestRuntime(t, "swap_host_to")
			r.setRuntimeSave(t, "Upload_1")
			r.clearPending(t)
			r.finalizeCount = 1
			test.set(r)
			d := New(r.fixture.fake, nil, nil, r.fixture.store)
			err := d.runImportActivation(context.Background(), r.fixture.instance, r.fixture.op, nil, activationTestOptions())
			assertImportActivationCode(t, err, ImportErrorResultUnconfirmed)
		})
	}
}

func TestImportActivationConnectedPlayerPreventsRestart(t *testing.T) {
	r := prepareActivationTestRuntime(t, "swap_host_to")
	r.fixture.writePending(t, "Upload_1", 99, phaseATestPlatformID)
	r.players = 1
	d := New(r.fixture.fake, nil, nil, r.fixture.store)
	err := d.runImportActivation(context.Background(), r.fixture.instance, r.fixture.op, nil, activationTestOptions())
	assertImportActivationCode(t, err, ImportErrorPlayersConnected)
	if r.restarts != 0 {
		t.Fatalf("connected player was forcibly restarted")
	}
}

func TestImportActivationNewProcessBaselineDelta(t *testing.T) {
	zero, one := 0, 1
	oldIdentity := &JunimoProcessIdentity{ContainerID: "old", ProcessStartTicks: "1"}
	newIdentity := &JunimoProcessIdentity{ContainerID: "new", ProcessStartTicks: "2"}
	j := ImportJournal{
		PreSubmitEvidence:         &JunimoImportEvidenceSnapshot{ProcessIdentity: oldIdentity, FinalizeCount: &zero},
		ActivationProcessBaseline: &JunimoActivationProcessBaseline{ProcessIdentity: newIdentity, FinalizeCount: &zero},
	}
	advanced, known := activationCountAdvanced(j, JunimoImportActivationEvidence{ProcessIdentity: newIdentity, FinalizeCount: &one})
	if !known || !advanced {
		t.Fatalf("new process baseline delta not recognized")
	}
}

func TestImportActivationUsesMaintenanceBaselineWhenPreSubmitDiagnosticsMissing(t *testing.T) {
	zero, one := 0, 1
	identity := &JunimoProcessIdentity{ContainerID: "same-process", ProcessStartTicks: "100"}
	j := ImportJournal{
		PreSubmitEvidence: &JunimoImportEvidenceSnapshot{ProcessIdentity: identity},
		RuntimeBaseline: &JunimoImportEvidenceSnapshot{
			ProcessIdentity: identity,
			FinalizeCount:   &zero,
		},
	}
	advanced, known := activationCountAdvanced(j, JunimoImportActivationEvidence{
		ProcessIdentity: identity,
		FinalizeCount:   &one,
	})
	if !known || !advanced {
		t.Fatalf("maintenance baseline was not used: known=%v advanced=%v", known, advanced)
	}
}

func assertImportActivationCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %s", code)
	}
	var typed *ImportTransactionError
	if !errors.As(err, &typed) || typed.Code != code {
		t.Fatalf("error=%v code=%v want=%s", err, typed, code)
	}
}
