package stardew_junimo

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

type importMaintenanceStore struct {
	mu       sync.Mutex
	instance storage.Instance
	updates  []storage.UpdateInstanceStateParams
}

func (s *importMaintenanceStore) GetInstance(context.Context, string) (storage.Instance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.instance, nil
}

func (s *importMaintenanceStore) UpdateInstanceState(_ context.Context, p storage.UpdateInstanceStateParams) (storage.Instance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updates = append(s.updates, p)
	s.instance.State, s.instance.DriverPhase, s.instance.DriverPayload = p.State, p.DriverPhase, p.DriverPayload
	s.instance.StateMessage = sql.NullString{String: p.StateMessage, Valid: p.StateMessage != ""}
	return s.instance, nil
}

type maintenanceFakeConfig struct {
	missingFIFO, apiDown, apiTimeout, versionMismatch, playersConnected bool
	composeUpError, neverRunning, processIdentityChanges                bool
	savesProbeFailures, diagnosticsFailures                             int
}

type maintenanceFakeRecord struct {
	mu                  sync.Mutex
	started, down       bool
	fifoCheckedBeforeUp bool
	stdin               []string
	identityCalls       int
	savesCommands       int
	diagnosticsCalls    int
}

func newMaintenanceFake(cfg maintenanceFakeConfig) (*fakeConsoleDocker, *maintenanceFakeRecord) {
	record := &maintenanceFakeRecord{}
	fake := &fakeConsoleDocker{}
	fake.composeUpFunc = func(context.Context, string) (paneldocker.CommandResult, error) {
		record.mu.Lock()
		record.started = true
		record.mu.Unlock()
		if cfg.composeUpError {
			return paneldocker.CommandResult{ExitCode: 1}, errors.New("start failed")
		}
		return paneldocker.CommandResult{}, nil
	}
	fake.composeDownFunc = func(context.Context, string) (paneldocker.CommandResult, error) {
		record.mu.Lock()
		record.down = true
		record.started = false
		record.mu.Unlock()
		return paneldocker.CommandResult{}, nil
	}
	fake.composePsFunc = func(context.Context, string) (paneldocker.ComposePsResult, error) {
		record.mu.Lock()
		started := record.started
		record.mu.Unlock()
		if started && !cfg.neverRunning && !cfg.composeUpError {
			return paneldocker.ComposePsResult{Services: []paneldocker.ComposeService{{Service: "server", State: "running", Status: "Up"}}}, nil
		}
		return paneldocker.ComposePsResult{}, nil
	}
	fake.execFunc = func(ctx context.Context, _ string, _ string, stdin string, args ...string) (paneldocker.CommandResult, error) {
		record.mu.Lock()
		if stdin != "" {
			record.stdin = append(record.stdin, stdin)
			if stdin == "saves\n" {
				record.savesCommands++
			}
		}
		started := record.started
		record.mu.Unlock()
		joined := strings.Join(args, " ")
		if joined == "test -p "+serverInputFIFO {
			if !started {
				record.mu.Lock()
				record.fifoCheckedBeforeUp = true
				record.mu.Unlock()
			}
			if cfg.missingFIFO {
				return paneldocker.CommandResult{ExitCode: 1}, nil
			}
			return paneldocker.CommandResult{}, nil
		}
		if joined == "test -r "+serverOutputLog {
			return paneldocker.CommandResult{}, nil
		}
		if joined == "wc -c "+serverOutputLog {
			return paneldocker.CommandResult{Stdout: "128 /tmp/server-output.log\n"}, nil
		}
		if joined == "cat /data/Mods/JunimoServer/manifest.json" {
			version := TestedImageTag
			if cfg.versionMismatch {
				version = "1.5.0-preview.124"
			}
			return paneldocker.CommandResult{Stdout: `{"Version":"` + version + `"}`}, nil
		}
		if len(args) > 0 && args[0] == "curl" {
			if cfg.apiTimeout {
				<-ctx.Done()
				return paneldocker.CommandResult{}, ctx.Err()
			}
			if cfg.apiDown {
				return paneldocker.CommandResult{ExitCode: 7}, nil
			}
			endpoint := args[len(args)-1]
			switch {
			case strings.HasSuffix(endpoint, "/health"):
				return paneldocker.CommandResult{Stdout: `{}`}, nil
			case strings.HasSuffix(endpoint, "/status"):
				count := 0
				if cfg.playersConnected {
					count = 1
				}
				return paneldocker.CommandResult{Stdout: `{"playerCount":` + string(rune('0'+count)) + `,"dayTransitionComplete":true}`}, nil
			case strings.HasSuffix(endpoint, "/diagnostics/state"):
				record.mu.Lock()
				record.diagnosticsCalls++
				calls := record.diagnosticsCalls
				record.mu.Unlock()
				if calls <= cfg.diagnosticsFailures {
					return paneldocker.CommandResult{Stdout: `{"masterName":"Server","failedFields":[]}`}, nil
				}
				return paneldocker.CommandResult{Stdout: `{"saveImportFinalizeCount":4,"masterName":"Server","failedFields":[]}`}, nil
			}
		}
		if strings.Contains(joined, "__ANXI_PROCESS_ID__") {
			record.mu.Lock()
			record.identityCalls++
			calls := record.identityCalls
			record.mu.Unlock()
			if cfg.processIdentityChanges && calls > 1 {
				return paneldocker.CommandResult{Stdout: "__ANXI_PROCESS_ID__ container-b 456\n"}, nil
			}
			return paneldocker.CommandResult{Stdout: "__ANXI_PROCESS_ID__ container-a 123\n"}, nil
		}
		if len(args) > 0 && args[0] == "tail" {
			record.mu.Lock()
			savesCommands := record.savesCommands
			record.mu.Unlock()
			if savesCommands <= cfg.savesProbeFailures {
				return paneldocker.CommandResult{Stdout: "$>saves\n"}, nil
			}
			return paneldocker.CommandResult{Stdout: "Available Saves:\n    Upload_1\n"}, nil
		}
		return paneldocker.CommandResult{}, nil
	}
	return fake, record
}

func TestImportMaintenanceWaitsForEvidenceBaseline(t *testing.T) {
	dataDir, op, instance, store := prepareMaintenanceFixture(t)
	fake, record := newMaintenanceFake(maintenanceFakeConfig{diagnosticsFailures: 2})
	d := New(fake, nil, nil, store)
	if err := d.runImportMaintenance(context.Background(), instance, op, nil,
		importMaintenanceOptions{ReadyTimeout: 100 * time.Millisecond, PollInterval: time.Millisecond}); err != nil {
		t.Fatal(err)
	}
	journal, err := LoadImportJournal(dataDir, op)
	if err != nil || journal.Stage != ImportStageRuntimeReady || journal.RuntimeBaseline == nil || journal.RuntimeBaseline.FinalizeCount == nil {
		t.Fatalf("journal=%+v err=%v", journal, err)
	}
	record.mu.Lock()
	defer record.mu.Unlock()
	if record.diagnosticsCalls != 3 {
		t.Fatalf("diagnostics calls=%d, want 3", record.diagnosticsCalls)
	}
}

func TestImportMaintenanceWaitsForSavesCommandRegistration(t *testing.T) {
	dataDir, op, instance, store := prepareMaintenanceFixture(t)
	fake, record := newMaintenanceFake(maintenanceFakeConfig{savesProbeFailures: 2})
	d := New(fake, nil, nil, store)
	if err := d.runImportMaintenance(context.Background(), instance, op, nil,
		importMaintenanceOptions{ReadyTimeout: 100 * time.Millisecond, PollInterval: time.Millisecond}); err != nil {
		t.Fatal(err)
	}
	journal, err := LoadImportJournal(dataDir, op)
	if err != nil || journal.Stage != ImportStageRuntimeReady {
		t.Fatalf("journal=%+v err=%v", journal, err)
	}
	record.mu.Lock()
	defer record.mu.Unlock()
	if record.savesCommands != 3 {
		t.Fatalf("saves probes=%d, want 3", record.savesCommands)
	}
}

func prepareMaintenanceFixture(t *testing.T) (string, string, registry.Instance, *importMaintenanceStore) {
	t.Helper()
	dataDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dataDir, ".env"), []byte("IMAGE_VERSION="+TestedImageTag+"\nAPI_PORT=5110\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	modDir := junimoServerModDir(dataDir)
	if err := os.MkdirAll(modDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "manifest.json"), []byte(`{"Version":"`+TestedImageTag+`","UniqueID":"JunimoHost.Server"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "JunimoServer.dll"), []byte("dll"), 0o600); err != nil {
		t.Fatal(err)
	}
	saveName := "Upload_1"
	saveDir := filepath.Join(savesDir(dataDir), "Saves", saveName)
	if err := os.MkdirAll(saveDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(saveDir, saveName), []byte("main-save"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(saveDir, "SaveGameInfo"), []byte("info"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(gameloaderPath(dataDir)), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(gameloaderPath(dataDir), []byte(`{"SaveNameToLoad":"Old_1"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(controlDir(dataDir), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(controlDir(dataDir), "status.json"), []byte(`{"saveId":"Old_1"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	op := NewImportOperationID()
	journal := ImportJournal{SchemaVersion: 1, OperationID: op, InstanceID: "stardew", SaveName: saveName,
		OriginalActiveSave: "Old_1", Stage: ImportStageBackupCreated, SourceOwned: true,
		PreimportBackupName: "preimport.zip", CreatedAt: time.Now().UTC()}
	if err := WriteImportJournal(dataDir, journal); err != nil {
		t.Fatal(err)
	}
	stored := storage.Instance{ID: "stardew", DriverID: DriverID, DataDir: dataDir, State: storage.InstanceStateStopped,
		StateMessage: sql.NullString{String: "stopped before import", Valid: true}, DriverPhase: "container_stopped", DriverPayload: `{"invite_code":"stale","other":"kept"}`}
	store := &importMaintenanceStore{instance: stored}
	instance := registry.Instance{ID: stored.ID, DriverID: DriverID, DataDir: dataDir, State: stored.State,
		StateMessage: stored.StateMessage.String, DriverPhase: stored.DriverPhase, DriverPayload: stored.DriverPayload}
	return dataDir, op, instance, store
}

func TestImportMaintenanceRuntimeReadyBaselineAndSafety(t *testing.T) {
	dataDir, op, instance, store := prepareMaintenanceFixture(t)
	fake, record := newMaintenanceFake(maintenanceFakeConfig{})
	d := New(fake, nil, nil, store)
	err := d.runImportMaintenance(context.Background(), instance, op, nil,
		importMaintenanceOptions{ReadyTimeout: 100 * time.Millisecond, PollInterval: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	journal, err := LoadImportJournal(dataDir, op)
	if err != nil {
		t.Fatal(err)
	}
	if journal.Stage != ImportStageRuntimeReady || journal.RuntimeBaseline == nil || journal.RuntimeBaseline.MainSaveSHA256 == "" || journal.RuntimeBaseline.FinalizeCount == nil || journal.RuntimeBaseline.ProcessIdentity == nil {
		t.Fatalf("incomplete runtime baseline: %+v", journal)
	}
	if journal.ServerOutputLogOffset == nil || *journal.ServerOutputLogOffset != 128 {
		t.Fatalf("log offset=%v", journal.ServerOutputLogOffset)
	}
	pointer, pointerErr := readActivePointerStrict(dataDir)
	if journal.RuntimeBaseline.ActivePointer != "Old_1" || pointerErr != nil || pointer != "Old_1" {
		t.Fatalf("active pointer changed: %+v", journal.RuntimeBaseline)
	}
	record.mu.Lock()
	defer record.mu.Unlock()
	if record.down || record.fifoCheckedBeforeUp {
		t.Fatalf("down=%v fifoCheckedBeforeUp=%v", record.down, record.fifoCheckedBeforeUp)
	}
	commands := strings.Join(record.stdin, "\n")
	if !strings.Contains(commands, "saves\n") || strings.Contains(commands, "saves import") || strings.Contains(commands, "newgame") {
		t.Fatalf("unexpected commands: %q", commands)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	last := store.updates[len(store.updates)-1]
	if last.State != storage.InstanceStateStopped || last.DriverPhase != importMaintenancePhase || inviteCodeFromPayload(last.DriverPayload) != "" || !strings.Contains(last.DriverPayload, "kept") {
		t.Fatalf("maintenance state was published as join-ready: %+v", last)
	}
}

func TestImportMaintenanceFailuresStopAndRestore(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  maintenanceFakeConfig
		code string
	}{
		{"fifo_missing", maintenanceFakeConfig{missingFIFO: true}, ImportErrorMaintenanceFIFO},
		{"api_unavailable", maintenanceFakeConfig{apiDown: true}, ImportErrorMaintenanceAPI},
		{"api_timeout", maintenanceFakeConfig{apiTimeout: true}, ImportErrorMaintenanceAPI},
		{"dll_runtime_mismatch", maintenanceFakeConfig{versionMismatch: true}, ImportErrorMaintenanceMod},
		{"container_start", maintenanceFakeConfig{composeUpError: true}, ImportErrorMaintenanceStart},
		{"players_connected", maintenanceFakeConfig{playersConnected: true}, ImportErrorPlayersConnected},
		{"process_identity_changed", maintenanceFakeConfig{processIdentityChanges: true}, ImportErrorMaintenanceProcess},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dataDir, op, instance, store := prepareMaintenanceFixture(t)
			fake, record := newMaintenanceFake(tc.cfg)
			d := New(fake, nil, nil, store)
			err := d.runImportMaintenance(context.Background(), instance, op, nil,
				importMaintenanceOptions{ReadyTimeout: 15 * time.Millisecond, PollInterval: time.Millisecond})
			typed, ok := AsImportTransactionError(err)
			if !ok || typed.Code != tc.code {
				t.Fatalf("error=%v code=%v want=%s", err, typed, tc.code)
			}
			record.mu.Lock()
			down := record.down
			record.mu.Unlock()
			if !down {
				t.Fatal("maintenance runtime was not stopped")
			}
			journal, loadErr := LoadImportJournal(dataDir, op)
			if loadErr != nil || journal.LastErrorCode != tc.code || journal.Stage != ImportStageBackupCreated {
				t.Fatalf("journal=%+v err=%v", journal, loadErr)
			}
			store.mu.Lock()
			last := store.updates[len(store.updates)-1]
			store.mu.Unlock()
			if last.State != storage.InstanceStateStopped || last.DriverPhase != "container_stopped" || last.StateMessage != "stopped before import" {
				t.Fatalf("original state not restored: %+v", last)
			}
		})
	}
}

func TestImportMaintenanceCancellationStopsRuntime(t *testing.T) {
	_, op, instance, store := prepareMaintenanceFixture(t)
	fake, record := newMaintenanceFake(maintenanceFakeConfig{neverRunning: true})
	d := New(fake, nil, nil, store)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := d.runImportMaintenance(ctx, instance, op, nil, importMaintenanceOptions{ReadyTimeout: time.Second, PollInterval: time.Millisecond})
	typed, ok := AsImportTransactionError(err)
	if !ok || typed.Code != ImportErrorMaintenanceCancel {
		t.Fatalf("error=%v", err)
	}
	record.mu.Lock()
	defer record.mu.Unlock()
	if !record.down {
		t.Fatal("canceled maintenance runtime was not stopped")
	}
}

func TestImportMaintenanceStaticDLLMismatchDoesNotStartContainer(t *testing.T) {
	dataDir, op, instance, store := prepareMaintenanceFixture(t)
	if err := os.WriteFile(filepath.Join(junimoServerModDir(dataDir), "manifest.json"), []byte(`{"Version":"1.5.0-preview.124","UniqueID":"JunimoHost.Server"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	fake, record := newMaintenanceFake(maintenanceFakeConfig{})
	d := New(fake, nil, nil, store)
	err := d.runImportMaintenance(context.Background(), instance, op, nil, importMaintenanceOptions{ReadyTimeout: time.Second, PollInterval: time.Millisecond})
	typed, ok := AsImportTransactionError(err)
	if !ok || typed.Code != ImportErrorUnsupported {
		t.Fatalf("error=%v", err)
	}
	record.mu.Lock()
	defer record.mu.Unlock()
	if record.started {
		t.Fatal("container started before static DLL verification")
	}
}

func TestImportMaintenanceMissingActivePointerDoesNotStartContainer(t *testing.T) {
	dataDir, op, instance, store := prepareMaintenanceFixture(t)
	if err := os.Remove(gameloaderPath(dataDir)); err != nil {
		t.Fatal(err)
	}
	fake, record := newMaintenanceFake(maintenanceFakeConfig{})
	d := New(fake, nil, nil, store)
	err := d.runImportMaintenance(context.Background(), instance, op, nil,
		importMaintenanceOptions{ReadyTimeout: time.Second, PollInterval: time.Millisecond})
	typed, ok := AsImportTransactionError(err)
	if !ok || typed.Code != ImportErrorMaintenanceReady {
		t.Fatalf("error=%v", err)
	}
	record.mu.Lock()
	defer record.mu.Unlock()
	if record.started {
		t.Fatal("container started without a stable original active pointer")
	}
}

func TestReconcileStatePreservesImportMaintenanceStoppedState(t *testing.T) {
	_, _, _, store := prepareMaintenanceFixture(t)
	store.instance.DriverPhase = importMaintenancePhase
	fake, _ := newMaintenanceFake(maintenanceFakeConfig{})
	fake.composePsFunc = func(context.Context, string) (paneldocker.ComposePsResult, error) {
		return paneldocker.ComposePsResult{Services: []paneldocker.ComposeService{{Service: "server", State: "running"}}}, nil
	}
	d := New(fake, nil, nil, store)
	got, err := d.ReconcileState(context.Background(), store.instance)
	if err != nil || got.State != storage.InstanceStateStopped || got.DriverPhase != importMaintenancePhase {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}

func TestImportMaintenanceDoesNotExposeInviteCode(t *testing.T) {
	_, _, instance, store := prepareMaintenanceFixture(t)
	store.instance.DriverPhase = importMaintenancePhase
	fake, record := newMaintenanceFake(maintenanceFakeConfig{})
	d := New(fake, nil, nil, store)
	_, err := d.GetInviteCode(context.Background(), instance)
	typed, ok := AsImportTransactionError(err)
	if !ok || typed.Code != ImportErrorBusy {
		t.Fatalf("error=%v", err)
	}
	record.mu.Lock()
	defer record.mu.Unlock()
	if len(record.stdin) != 0 {
		t.Fatalf("invite retrieval reached runtime: %q", record.stdin)
	}
}
