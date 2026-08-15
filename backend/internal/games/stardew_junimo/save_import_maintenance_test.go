package stardew_junimo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

type importMaintenanceStore struct {
	mu         sync.Mutex
	instance   storage.Instance
	updates    []storage.UpdateInstanceStateParams
	updateErr  error
	restoreErr error
}

func (s *importMaintenanceStore) GetInstance(context.Context, string) (storage.Instance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.instance, nil
}

func (s *importMaintenanceStore) UpdateInstanceState(_ context.Context, p storage.UpdateInstanceStateParams) (storage.Instance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.updateErr != nil {
		return storage.Instance{}, s.updateErr
	}
	s.updates = append(s.updates, p)
	s.instance.State, s.instance.DriverPhase, s.instance.DriverPayload = p.State, p.DriverPhase, p.DriverPayload
	s.instance.StateMessage = sql.NullString{String: p.StateMessage, Valid: p.StateMessage != ""}
	return s.instance, nil
}

func (s *importMaintenanceStore) RestoreInstanceStateSnapshot(_ context.Context, snapshot storage.Instance) (storage.Instance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.restoreErr != nil {
		return storage.Instance{}, s.restoreErr
	}
	s.updates = append(s.updates, storage.UpdateInstanceStateParams{
		ID: snapshot.ID, State: snapshot.State, StateMessage: snapshot.StateMessage.String,
		DriverPhase: snapshot.DriverPhase, DriverPayload: snapshot.DriverPayload,
	})
	s.instance.State = snapshot.State
	s.instance.StateMessage = snapshot.StateMessage
	s.instance.DriverPhase = snapshot.DriverPhase
	s.instance.DriverPayload = snapshot.DriverPayload
	return s.instance, nil
}

func TestSaveImportServerStoppedStrictMatrix(t *testing.T) {
	for _, tc := range []struct {
		name     string
		services []paneldocker.ComposeService
		stopped  bool
		wantErr  bool
	}{
		{name: "absent", stopped: true},
		{name: "all terminal", services: []paneldocker.ComposeService{{Service: "server", State: "exited"}, {Service: "server", State: "dead"}, {Service: "steam-auth", State: "running"}}, stopped: true},
		{name: "running", services: []paneldocker.ComposeService{{Service: "server", State: "running"}}},
		{name: "status up", services: []paneldocker.ComposeService{{Service: "server", State: "exited", Status: "Up 1 second"}}},
		{name: "created", services: []paneldocker.ComposeService{{Service: "server", State: "created"}}},
		{name: "one of multiple running", services: []paneldocker.ComposeService{{Service: "server", State: "exited"}, {Service: "server", State: "running"}}},
		{name: "one of multiple unknown", services: []paneldocker.ComposeService{{Service: "server", State: "dead"}, {Service: "server", State: "mystery"}}, wantErr: true},
		{name: "restarting", services: []paneldocker.ComposeService{{Service: "server", State: "restarting"}}},
		{name: "paused", services: []paneldocker.ComposeService{{Service: "server", State: "paused"}}},
		{name: "removing", services: []paneldocker.ComposeService{{Service: "server", State: "removing"}}},
		{name: "unknown", services: []paneldocker.ComposeService{{Service: "server", State: "mystery"}}, wantErr: true},
		{name: "missing state", services: []paneldocker.ComposeService{{Service: "server"}}, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stopped, err := saveImportServerStoppedStrict(tc.services)
			if (err != nil) != tc.wantErr || stopped != tc.stopped {
				t.Fatalf("stopped=%v err=%v", stopped, err)
			}
		})
	}
}

func TestImportMaintenanceJournalSnapshotPreservesRawLifecycleBytes(t *testing.T) {
	dataDir, op, _, _ := prepareMaintenanceFixture(t)
	journal, err := LoadImportJournal(dataDir, op)
	if err != nil {
		t.Fatal(err)
	}
	original := storage.Instance{
		ID: "stardew", DataDir: dataDir, State: storage.InstanceStateStopped,
		StateMessage:  sql.NullString{String: string([]byte{'m', 0, 0xff}), Valid: true},
		DriverPhase:   string([]byte{'p', 0, 0xfe}),
		DriverPayload: string([]byte{'{', 0xff, 0, '}'}),
	}
	captureOriginalInstanceSnapshot(&journal, original)
	if err := WriteImportJournal(dataDir, journal); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadImportJournal(dataDir, op)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := originalInstanceSnapshotFromJournal(loaded, dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if restored.StateMessage != original.StateMessage || restored.DriverPhase != original.DriverPhase || restored.DriverPayload != original.DriverPayload {
		t.Fatalf("snapshot bytes changed: got=%+v", restored)
	}
}

func TestEnsureSaveImportServerStoppedUsesFreshDockerClientState(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("controlled Docker Client cache/parser test runs on the Linux target filesystem")
	}
	for _, tc := range []struct {
		name        string
		cachedState string
		freshState  string
		wantErr     bool
	}{
		{name: "cached stopped fresh running rejects", cachedState: "exited", freshState: "running", wantErr: true},
		{name: "cached running fresh exited passes", cachedState: "running", freshState: "exited"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			workDir := t.TempDir()
			client := paneldocker.NewClient(paneldocker.Options{
				DockerPath:     writeSaveImportProbeDocker(t, tc.cachedState, tc.freshState),
				ComposePsTTL:   time.Minute,
				MaxOutputBytes: 1 << 20,
			})
			cached, err := client.ComposePs(context.Background(), workDir)
			if err != nil || len(cached.Services) != 1 || cached.Services[0].State != tc.cachedState {
				t.Fatalf("prime cache result=%+v err=%v", cached, err)
			}
			err = ensureSaveImportServerStopped(context.Background(), client, workDir)
			if (err != nil) != tc.wantErr {
				t.Fatalf("fresh state %q error=%v", tc.freshState, err)
			}
		})
	}
}

func TestRunImportMaintenanceRecordsStrictPreflightFailureInAuthoritativeDataDir(t *testing.T) {
	root := t.TempDir()
	authoritativeDir := filepath.Join(root, "authoritative")
	staleDir := filepath.Join(root, "stale")
	if err := os.MkdirAll(authoritativeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(staleDir, 0o700); err != nil {
		t.Fatal(err)
	}
	op := NewImportOperationID()
	if _, err := CreateImportJournal(authoritativeDir, registry.SaveImportRequest{
		Instance:    registry.Instance{ID: "stardew", DriverID: DriverID, DataDir: authoritativeDir},
		OperationID: op, SaveName: "Upload_1", HostHandling: "server_owns_original",
	}); err != nil {
		t.Fatal(err)
	}
	store := &importMaintenanceStore{instance: storage.Instance{
		ID: "stardew", DriverID: DriverID, DataDir: authoritativeDir, State: storage.InstanceStateGameInstalled,
	}}
	fake := &fakeConsoleDocker{composePsStrictFunc: func(context.Context, string) (paneldocker.ComposePsResult, error) {
		return paneldocker.ComposePsResult{}, errors.New("strict probe failed")
	}}
	driver := New(fake, nil, nil, store)
	err := driver.runImportMaintenance(context.Background(), registry.Instance{
		ID: "stardew", DriverID: DriverID, DataDir: staleDir, State: storage.InstanceStateGameInstalled,
	}, op, nil, defaultImportMaintenanceOptions())
	typed, ok := AsImportTransactionError(err)
	if !ok || typed.Code != ImportErrorMaintenanceStart {
		t.Fatalf("error=%v", err)
	}
	j, err := LoadImportJournal(authoritativeDir, op)
	if err != nil || j.LastErrorCode != ImportErrorMaintenanceStart {
		t.Fatalf("authoritative journal=%+v err=%v", j, err)
	}
	if _, err := os.Stat(importJournalPath(staleDir, op)); !os.IsNotExist(err) {
		t.Fatalf("stale directory was modified: %v", err)
	}
}

func writeSaveImportProbeDocker(t *testing.T, cachedState, freshState string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "docker")
	script := fmt.Sprintf(`#!/bin/sh
case "$1 $2 $3 $4 $5" in
  "compose ps --format json ") printf '[{"Service":"server","State":"%s","Status":"cached"}]' ;;
  "compose ps --all --format json") printf '[{"Service":"server","State":"%s","Status":"fresh"}]' ;;
  *) printf 'unexpected args' >&2; exit 7 ;;
esac
`, cachedState, freshState)
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

type maintenanceFakeConfig struct {
	missingFIFO, apiDown, apiTimeout, versionMismatch, playersConnected bool
	composeUpError, neverRunning, processIdentityChanges                bool
	composeDownError, strictStopError, strictStopUnknown                bool
	savesProbeFailures, diagnosticsFailures                             int
}

type maintenanceFakeRecord struct {
	mu                  sync.Mutex
	started, down       bool
	upCalls             int
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
		record.upCalls++
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
		if cfg.composeDownError {
			record.mu.Unlock()
			return paneldocker.CommandResult{ExitCode: 1}, errors.New("injected compose down failure")
		}
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
	fake.composePsStrictFunc = func(ctx context.Context, dir string) (paneldocker.ComposePsResult, error) {
		record.mu.Lock()
		down := record.down
		record.mu.Unlock()
		if down && cfg.strictStopError {
			return paneldocker.ComposePsResult{}, errors.New("injected strict stop probe failure")
		}
		if down && cfg.strictStopUnknown {
			return paneldocker.ComposePsResult{Services: []paneldocker.ComposeService{{Service: "server", State: "unknown"}}}, nil
		}
		return fake.ComposePs(ctx, dir)
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

func TestImportMaintenanceAcceptsFirstUploadBootstrapWorld(t *testing.T) {
	dataDir, op, instance, store := prepareMaintenanceFixture(t)
	setMaintenanceFixtureState(store, &instance, storage.InstanceStateGameInstalled, "game installed; save required", "game_installed", `{"install":"complete","invite_code":"stale"}`)
	j, err := LoadImportJournal(dataDir, op)
	if err != nil {
		t.Fatal(err)
	}
	j.OriginalActiveSave = ""
	j.StagedSaveFingerprint, err = importDirectoryFingerprint(filepath.Join(savesDir(dataDir), "Saves", j.SaveName))
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteImportJournal(dataDir, j); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(gameloaderPath(dataDir)); err != nil {
		t.Fatal(err)
	}
	if err := prepareImportBootstrap(dataDir, op); err != nil {
		t.Fatal(err)
	}
	j, err = LoadImportJournal(dataDir, op)
	if err != nil || j.BootstrapSaveName == "" || j.OriginalActiveSave != j.BootstrapSaveName {
		t.Fatalf("bootstrap journal=%+v err=%v", j, err)
	}
	fake, _ := newMaintenanceFake(maintenanceFakeConfig{})
	driver := New(fake, nil, nil, store)
	if err := driver.runImportMaintenance(context.Background(), instance, op, nil,
		importMaintenanceOptions{ReadyTimeout: 100 * time.Millisecond, PollInterval: time.Millisecond}); err != nil {
		t.Fatal(err)
	}
	j, err = LoadImportJournal(dataDir, op)
	if err != nil || j.Stage != ImportStageRuntimeReady || j.RuntimeBaseline == nil || j.RuntimeBaseline.ActivePointer != j.BootstrapSaveName {
		t.Fatalf("runtime bootstrap baseline=%+v err=%v", j, err)
	}
}

func setMaintenanceFixtureState(store *importMaintenanceStore, instance *registry.Instance, state, message, phase, payload string) {
	store.mu.Lock()
	store.instance.State = state
	store.instance.StateMessage = sql.NullString{String: message, Valid: message != ""}
	store.instance.DriverPhase = phase
	store.instance.DriverPayload = payload
	store.mu.Unlock()
	instance.State = state
	instance.StateMessage = message
	instance.DriverPhase = phase
	instance.DriverPayload = payload
}

func TestSaveImportMaintenanceOfflineStateContract(t *testing.T) {
	allowed := map[string]bool{
		storage.InstanceStateGameInstalled: true,
		storage.InstanceStateSaveRequired:  true,
		storage.InstanceStateReadyToStart:  true,
		storage.InstanceStateStopped:       true,
	}
	states := []string{
		storage.InstanceStateUninitialized,
		storage.InstanceStateAdminCreated,
		storage.InstanceStateJunimoScaffolded,
		storage.InstanceStateCredentialsRequired,
		storage.InstanceStateSteamAuthRunning,
		storage.InstanceStateSteamAuthFailed,
		storage.InstanceStateSteamAuthDone,
		storage.InstanceStateGameInstalled,
		storage.InstanceStateSaveRequired,
		storage.InstanceStateReadyToStart,
		storage.InstanceStateStarting,
		storage.InstanceStateRunning,
		storage.InstanceStateStopped,
		storage.InstanceStateError,
	}
	for _, state := range states {
		if got := IsSaveImportMaintenanceOfflineState(state); got != allowed[state] {
			t.Errorf("state %q allowed=%v, want %v", state, got, allowed[state])
		}
	}
}

func TestImportMaintenanceAllowedOfflineStateMatrix(t *testing.T) {
	for _, state := range []string{
		storage.InstanceStateGameInstalled,
		storage.InstanceStateSaveRequired,
		storage.InstanceStateReadyToStart,
		storage.InstanceStateStopped,
	} {
		t.Run(state, func(t *testing.T) {
			dataDir, op, instance, store := prepareMaintenanceFixture(t)
			setMaintenanceFixtureState(store, &instance, state, "original "+state, "phase_"+state, `{"kept":"`+state+`"}`)
			fake, _ := newMaintenanceFake(maintenanceFakeConfig{})
			driver := New(fake, nil, nil, store)
			if err := driver.runImportMaintenance(context.Background(), instance, op, nil,
				importMaintenanceOptions{ReadyTimeout: time.Second, PollInterval: time.Millisecond}); err != nil {
				t.Fatal(err)
			}
			journal, err := LoadImportJournal(dataDir, op)
			if err != nil || journal.Stage != ImportStageRuntimeReady {
				t.Fatalf("journal=%+v err=%v", journal, err)
			}
		})
	}
}

func TestImportMaintenanceRejectsRunningComposeBeforeStartup(t *testing.T) {
	dataDir, op, instance, store := prepareMaintenanceFixture(t)
	setMaintenanceFixtureState(store, &instance, storage.InstanceStateGameInstalled, "installed", "game_installed", `{"kept":true}`)
	fake, record := newMaintenanceFake(maintenanceFakeConfig{})
	fake.composePsFunc = func(context.Context, string) (paneldocker.ComposePsResult, error) {
		return paneldocker.ComposePsResult{Services: []paneldocker.ComposeService{{Service: "server", State: "running", Status: "Up"}}}, nil
	}
	driver := New(fake, nil, nil, store)
	err := driver.runImportMaintenance(context.Background(), instance, op, nil,
		importMaintenanceOptions{ReadyTimeout: 100 * time.Millisecond, PollInterval: time.Millisecond})
	typed, ok := AsImportTransactionError(err)
	if !ok || typed.Code != ImportErrorSaveInProgress || strings.Contains(typed.Message, "must remain stopped") {
		t.Fatalf("error=%v typed=%+v", err, typed)
	}
	record.mu.Lock()
	started := record.started
	record.mu.Unlock()
	if started {
		t.Fatal("maintenance ComposeUp ran while the server service was already running")
	}
	journal, loadErr := LoadImportJournal(dataDir, op)
	if loadErr != nil || journal.LastErrorCode != ImportErrorSaveInProgress {
		t.Fatalf("journal=%+v err=%v", journal, loadErr)
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
	if err := installSMAPIMod(dataDir); err != nil {
		t.Fatal(err)
	}
	runtimeManifest, err := sjconfig.BuiltInRuntimeStackManifest()
	if err != nil {
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
	if err := os.WriteFile(filepath.Join(controlDir(dataDir), "options.json"), []byte(`{"controlModVersion":"`+runtimeManifest.Control.Version+`","hostFarmhousePreservationPatchAvailable":true}`), 0o600); err != nil {
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
				importMaintenanceOptions{ReadyTimeout: 250 * time.Millisecond, PollInterval: time.Millisecond})
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

func TestImportMaintenanceGameInstalledFailuresRestoreExactSnapshot(t *testing.T) {
	for _, tc := range []struct {
		name   string
		cfg    maintenanceFakeConfig
		cancel bool
	}{
		{name: "startup", cfg: maintenanceFakeConfig{composeUpError: true}},
		{name: "readiness", cfg: maintenanceFakeConfig{missingFIFO: true}},
		{name: "canceled", cfg: maintenanceFakeConfig{neverRunning: true}, cancel: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, op, instance, store := prepareMaintenanceFixture(t)
			const originalPayload = `{"install":"complete","nested":{"kept":true},"invite_code":"stale"}`
			setMaintenanceFixtureState(store, &instance, storage.InstanceStateGameInstalled,
				"exact game-installed message", "game_installed", originalPayload)
			fake, record := newMaintenanceFake(tc.cfg)
			driver := New(fake, nil, nil, store)
			ctx := context.Background()
			if tc.cancel {
				canceled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = canceled
			}
			err := driver.runImportMaintenance(ctx, instance, op, nil,
				importMaintenanceOptions{ReadyTimeout: 30 * time.Millisecond, PollInterval: time.Millisecond})
			if err == nil {
				t.Fatal("maintenance failure fixture unexpectedly succeeded")
			}
			record.mu.Lock()
			down := record.down
			record.mu.Unlock()
			if !down {
				t.Fatal("failed maintenance runtime was not stopped")
			}
			store.mu.Lock()
			restored := store.instance
			store.mu.Unlock()
			if restored.State != storage.InstanceStateGameInstalled || restored.DriverPhase != "game_installed" ||
				restored.StateMessage.String != "exact game-installed message" || !restored.StateMessage.Valid || restored.DriverPayload != originalPayload {
				t.Fatalf("original snapshot not restored exactly: %+v", restored)
			}
		})
	}
}

func TestImportMaintenanceFailureRestoresAllOfflineStatesAndMessageTriState(t *testing.T) {
	states := []string{
		storage.InstanceStateGameInstalled,
		storage.InstanceStateSaveRequired,
		storage.InstanceStateReadyToStart,
		storage.InstanceStateStopped,
	}
	messages := []struct {
		name  string
		value sql.NullString
	}{
		{name: "null", value: sql.NullString{String: "ignored", Valid: false}},
		{name: "empty", value: sql.NullString{String: "", Valid: true}},
		{name: "ordinary", value: sql.NullString{String: "exact ordinary message", Valid: true}},
	}
	for _, state := range states {
		for _, message := range messages {
			t.Run(state+"/"+message.name, func(t *testing.T) {
				_, op, instance, store := prepareMaintenanceFixture(t)
				phase := "phase_" + state
				payload := `{"state":"` + state + `"}`
				if state == storage.InstanceStateGameInstalled && message.name == "null" {
					phase, payload = "", ""
				}
				store.mu.Lock()
				store.instance.State = state
				store.instance.StateMessage = message.value
				store.instance.DriverPhase = phase
				store.instance.DriverPayload = payload
				original := store.instance
				store.mu.Unlock()
				instance.State = state
				instance.StateMessage = message.value.String
				instance.DriverPhase = phase
				instance.DriverPayload = payload
				fake, _ := newMaintenanceFake(maintenanceFakeConfig{composeUpError: true})
				driver := New(fake, nil, nil, store)
				if err := driver.runImportMaintenance(context.Background(), instance, op, nil,
					importMaintenanceOptions{ReadyTimeout: time.Second, PollInterval: time.Millisecond}); err == nil {
					t.Fatal("maintenance failure fixture unexpectedly succeeded")
				}
				store.mu.Lock()
				restored := store.instance
				store.mu.Unlock()
				if restored.State != original.State || restored.StateMessage != original.StateMessage ||
					restored.DriverPhase != original.DriverPhase || restored.DriverPayload != original.DriverPayload {
					t.Fatalf("snapshot not restored exactly: got=%+v want=%+v", restored, original)
				}
			})
		}
	}
}

func TestImportMaintenanceStateWriteFailurePreventsComposeUp(t *testing.T) {
	dataDir, op, instance, store := prepareMaintenanceFixture(t)
	store.updateErr = errors.New("injected state write failure")
	fake, record := newMaintenanceFake(maintenanceFakeConfig{})
	driver := New(fake, nil, nil, store)
	err := driver.runImportMaintenance(context.Background(), instance, op, nil,
		importMaintenanceOptions{ReadyTimeout: time.Second, PollInterval: time.Millisecond})
	typed, ok := AsImportTransactionError(err)
	if !ok || typed.Code != ImportErrorMaintenanceStart {
		t.Fatalf("error=%T %v", err, err)
	}
	record.mu.Lock()
	upCalls := record.upCalls
	record.mu.Unlock()
	if upCalls != 0 {
		t.Fatal("ComposeUp ran after maintenance state write failed")
	}
	journal, loadErr := LoadImportJournal(dataDir, op)
	if loadErr != nil || journal.MaintenanceStarted || journal.MaintenanceRecoveryState != importMaintenanceSnapshotRestored {
		t.Fatalf("journal=%+v err=%v", journal, loadErr)
	}
}

func TestImportMaintenanceStartedJournalFailurePreventsComposeUpAndRestoresSnapshot(t *testing.T) {
	dataDir, op, instance, store := prepareMaintenanceFixture(t)
	store.mu.Lock()
	original := store.instance
	store.mu.Unlock()
	fake, record := newMaintenanceFake(maintenanceFakeConfig{})
	driver := New(fake, nil, nil, store)
	writes := 0
	driver.importJournalWrite = func(dir string, journal ImportJournal) error {
		writes++
		if writes == 2 {
			return errors.New("injected MaintenanceStarted journal failure")
		}
		return WriteImportJournal(dir, journal)
	}
	err := driver.runImportMaintenance(context.Background(), instance, op, nil,
		importMaintenanceOptions{ReadyTimeout: time.Second, PollInterval: time.Millisecond})
	typed, ok := AsImportTransactionError(err)
	if !ok || typed.Code != ImportErrorRecoveryRequired {
		t.Fatalf("error=%T %v", err, err)
	}
	record.mu.Lock()
	upCalls, down := record.upCalls, record.down
	record.mu.Unlock()
	if upCalls != 0 || !down {
		t.Fatalf("upCalls=%d down=%v", upCalls, down)
	}
	store.mu.Lock()
	restored := store.instance
	store.mu.Unlock()
	if restored.State != original.State || restored.StateMessage != original.StateMessage ||
		restored.DriverPhase != original.DriverPhase || restored.DriverPayload != original.DriverPayload {
		t.Fatalf("snapshot not restored: got=%+v want=%+v", restored, original)
	}
	journal, loadErr := LoadImportJournal(dataDir, op)
	if loadErr != nil || journal.MaintenanceStarted || journal.MaintenanceRecoveryState != importMaintenanceSnapshotRestored {
		t.Fatalf("journal=%+v err=%v", journal, loadErr)
	}
}

func TestImportMaintenanceRollbackRequiresDownStrictProbeAndFlagPersistence(t *testing.T) {
	for _, tc := range []struct {
		name             string
		cfg              maintenanceFakeConfig
		failJournalWrite int
	}{
		{name: "compose down failure", cfg: maintenanceFakeConfig{composeUpError: true, composeDownError: true}},
		{name: "strict probe failure", cfg: maintenanceFakeConfig{composeUpError: true, strictStopError: true}},
		{name: "strict unknown state", cfg: maintenanceFakeConfig{composeUpError: true, strictStopUnknown: true}},
		{name: "MaintenanceStarted false write failure", cfg: maintenanceFakeConfig{composeUpError: true}, failJournalWrite: 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dataDir, op, instance, store := prepareMaintenanceFixture(t)
			fake, _ := newMaintenanceFake(tc.cfg)
			driver := New(fake, nil, nil, store)
			if tc.failJournalWrite > 0 {
				writes := 0
				driver.importJournalWrite = func(dir string, journal ImportJournal) error {
					writes++
					if writes == tc.failJournalWrite {
						return errors.New("injected rollback journal failure")
					}
					return WriteImportJournal(dir, journal)
				}
			}
			err := driver.runImportMaintenance(context.Background(), instance, op, nil,
				importMaintenanceOptions{ReadyTimeout: time.Second, PollInterval: time.Millisecond})
			typed, ok := AsImportTransactionError(err)
			if !ok || typed.Code != ImportErrorRecoveryRequired {
				t.Fatalf("error=%T %v", err, err)
			}
			store.mu.Lock()
			current := store.instance
			store.mu.Unlock()
			if current.DriverPhase != importMaintenancePhase {
				t.Fatalf("ordinary offline snapshot was restored without complete proof: %+v", current)
			}
			journal, loadErr := LoadImportJournal(dataDir, op)
			if loadErr != nil || !journal.MaintenanceStarted {
				t.Fatalf("journal safety evidence was cleared: %+v err=%v", journal, loadErr)
			}
		})
	}
}

func TestRecordMaintenanceFailureJournalErrorIsRecoveryRequired(t *testing.T) {
	dataDir, op, _, store := prepareMaintenanceFixture(t)
	injected := errors.New("injected LastError journal failure")
	driver := New(&fakeConsoleDocker{}, nil, nil, store)
	driver.importJournalWrite = func(string, ImportJournal) error { return injected }
	err := driver.recordMaintenanceFailure(dataDir, op, ImportErrorMaintenanceReady, "primary readiness failure", errors.New("primary"))
	typed, ok := AsImportTransactionError(err)
	if !ok || typed.Code != ImportErrorRecoveryRequired || !errors.Is(err, injected) {
		t.Fatalf("error=%T %v", err, err)
	}
}

func TestRecoverInterruptedImportMaintenanceCrashWindows(t *testing.T) {
	for _, tc := range []struct {
		name             string
		maintenanceState string
		started          bool
		recoveryState    string
		wantDown         bool
	}{
		{name: "flag true before ComposeUp", maintenanceState: importMaintenanceStartIntentPersisted, recoveryState: importRecoveryMaintenanceStopAndRestore, wantDown: true},
		{name: "ComposeUp returned before runtime ready", maintenanceState: importMaintenanceComposeUpReturned, started: true, recoveryState: importRecoveryMaintenanceStopAndRestore, wantDown: true},
		{name: "ComposeDown returned before flag clear", maintenanceState: importMaintenanceComposeUpReturned, recoveryState: importRecoveryMaintenanceStopAndRestore, wantDown: true},
		{name: "flag clear before snapshot restore", maintenanceState: importMaintenanceSnapshotRestorePending, recoveryState: importRecoveryMaintenanceRestoreSnapshot},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dataDir, op, instance, store := prepareMaintenanceFixture(t)
			store.mu.Lock()
			original := store.instance
			store.instance.State = storage.InstanceStateStopped
			store.instance.StateMessage = sql.NullString{String: "private maintenance", Valid: true}
			store.instance.DriverPhase = importMaintenancePhase
			store.instance.DriverPayload = `{}`
			store.mu.Unlock()
			journal, err := LoadImportJournal(dataDir, op)
			if err != nil {
				t.Fatal(err)
			}
			captureOriginalInstanceSnapshot(&journal, original)
			journal.MaintenanceStarted = tc.maintenanceState != importMaintenanceSnapshotRestorePending
			journal.MaintenanceRecoveryState = tc.maintenanceState
			if err := WriteImportJournal(dataDir, journal); err != nil {
				t.Fatal(err)
			}
			fake, record := newMaintenanceFake(maintenanceFakeConfig{})
			record.started = tc.started
			driver := New(fake, nil, nil, store)
			recoveries, err := RecoverImportTransactions(dataDir)
			if err != nil || len(recoveries) != 1 || recoveries[0].State != tc.recoveryState {
				t.Fatalf("recoveries=%+v err=%v", recoveries, err)
			}
			recoveries, err = driver.recoverInterruptedImportMaintenance(context.Background(), instance, recoveries)
			if err != nil || recoveries[0].State != "safe_to_resume_or_cleanup" {
				t.Fatalf("recoveries=%+v err=%v", recoveries, err)
			}
			record.mu.Lock()
			down := record.down
			record.mu.Unlock()
			if down != tc.wantDown {
				t.Fatalf("ComposeDown=%v want=%v", down, tc.wantDown)
			}
			store.mu.Lock()
			restored := store.instance
			store.mu.Unlock()
			if restored.State != original.State || restored.StateMessage != original.StateMessage ||
				restored.DriverPhase != original.DriverPhase || restored.DriverPayload != original.DriverPayload {
				t.Fatalf("snapshot not restored: got=%+v want=%+v", restored, original)
			}
			journal, err = LoadImportJournal(dataDir, op)
			if err != nil || journal.MaintenanceStarted || journal.MaintenanceRecoveryState != importMaintenanceSnapshotRestored {
				t.Fatalf("journal=%+v err=%v", journal, err)
			}
		})
	}
}

func TestRecoverInterruptedImportMaintenanceAmbiguousFIFOStaysManual(t *testing.T) {
	dataDir, op, instance, store := prepareMaintenanceFixture(t)
	journal, err := LoadImportJournal(dataDir, op)
	if err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	original := store.instance
	store.instance.DriverPhase = importMaintenancePhase
	store.mu.Unlock()
	captureOriginalInstanceSnapshot(&journal, original)
	journal.Stage = ImportStageRuntimeReady
	journal.MaintenanceStarted = true
	journal.MaintenanceRecoveryState = importMaintenanceRuntimeReadyPersisted
	journal.PhaseAFIFOWriteAttempted = true
	if err := WriteImportJournal(dataDir, journal); err != nil {
		t.Fatal(err)
	}
	fake, record := newMaintenanceFake(maintenanceFakeConfig{})
	record.started = true
	driver := New(fake, nil, nil, store)
	recoveries, err := RecoverImportTransactions(dataDir)
	if err != nil || len(recoveries) != 1 || recoveries[0].State != "manual_required" {
		t.Fatalf("recoveries=%+v err=%v", recoveries, err)
	}
	if _, err := driver.recoverInterruptedImportMaintenance(context.Background(), instance, recoveries); err == nil {
		t.Fatal("ambiguous FIFO recovery did not fail closed")
	}
	record.mu.Lock()
	down := record.down
	record.mu.Unlock()
	if !down {
		t.Fatal("ambiguous FIFO recovery left the private maintenance runtime running")
	}
	journal, err = LoadImportJournal(dataDir, op)
	if err != nil || !journal.MaintenanceStarted || !journal.PhaseAFIFOWriteAttempted || journal.MaintenanceRecoveryState != importMaintenanceManualRecovery {
		t.Fatalf("manual recovery evidence changed: %+v err=%v", journal, err)
	}
}

func TestImportMaintenanceRestoreFailureSurfacesRecoveryRequired(t *testing.T) {
	dataDir, op, instance, store := prepareMaintenanceFixture(t)
	store.restoreErr = errors.New("injected exact restore failure")
	fake, _ := newMaintenanceFake(maintenanceFakeConfig{composeUpError: true})
	driver := New(fake, nil, nil, store)
	err := driver.runImportMaintenance(context.Background(), instance, op, nil,
		importMaintenanceOptions{ReadyTimeout: time.Second, PollInterval: time.Millisecond})
	typed, ok := AsImportTransactionError(err)
	if !ok || typed.Code != ImportErrorRecoveryRequired {
		t.Fatalf("error=%T %v", err, err)
	}
	journal, loadErr := LoadImportJournal(dataDir, op)
	if loadErr != nil || journal.MaintenanceStarted || journal.MaintenanceRecoveryState != importMaintenanceSnapshotRestorePending {
		t.Fatalf("journal=%+v err=%v", journal, loadErr)
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

func TestImportMaintenanceControlMismatchStopsRuntime(t *testing.T) {
	dataDir, op, instance, store := prepareMaintenanceFixture(t)
	if err := os.WriteFile(filepath.Join(controlDir(dataDir), "options.json"), []byte(`{"controlModVersion":"0.0.0"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	fake, record := newMaintenanceFake(maintenanceFakeConfig{})
	d := New(fake, nil, nil, store)
	err := d.runImportMaintenance(context.Background(), instance, op, nil,
		importMaintenanceOptions{ReadyTimeout: 100 * time.Millisecond, PollInterval: time.Millisecond})
	typed, ok := AsImportTransactionError(err)
	if !ok || typed.Code != ImportErrorMaintenanceControl {
		t.Fatalf("error=%v code=%v", err, typed)
	}
	record.mu.Lock()
	defer record.mu.Unlock()
	if !record.down {
		t.Fatal("mismatched Control runtime was not stopped")
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
