package stardew_junimo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
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
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
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

func TestSaveImportRuntimeServicesStoppedStrictScope(t *testing.T) {
	for _, tc := range []struct {
		name     string
		services []paneldocker.ComposeService
		selected []string
		stopped  bool
		wantErr  bool
	}{
		{name: "disabled ignores auth", services: []paneldocker.ComposeService{{Service: "server", State: "exited"}, {Service: "steam-auth", State: "running"}}, selected: []string{"server"}, stopped: true},
		{name: "enabled requires auth", services: []paneldocker.ComposeService{{Service: "server", State: "exited"}, {Service: "steam-auth", State: "running"}}, selected: []string{"server", "steam-auth"}},
		{name: "contradictory status cannot override running state", services: []paneldocker.ComposeService{{Service: "server", State: "running", Status: "Exited (0)"}}, selected: []string{"server"}},
		{name: "unknown selected state", services: []paneldocker.ComposeService{{Service: "server", State: "exited"}, {Service: "steam-auth", State: "mystery"}}, selected: []string{"server", "steam-auth"}, wantErr: true},
		{name: "unselected orphan is outside runtime scope", services: []paneldocker.ComposeService{{Service: "server", State: "dead"}, {Service: "fixture", State: "running"}}, selected: []string{"server"}, stopped: true},
		{name: "duplicate selected active copy rejects", services: []paneldocker.ComposeService{{Service: "steam-auth", State: "dead"}, {Service: "steam-auth", State: "running"}}, selected: []string{"server", "steam-auth"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stopped, err := saveImportRuntimeServicesStoppedStrict(tc.services, tc.selected)
			if stopped != tc.stopped || (err != nil) != tc.wantErr {
				t.Fatalf("stopped=%v err=%v", stopped, err)
			}
		})
	}
}

func TestSaveImportRuntimeServicesRunningStrictScope(t *testing.T) {
	for _, tc := range []struct {
		name     string
		services []paneldocker.ComposeService
		running  bool
		wantErr  bool
	}{
		{name: "server running", services: []paneldocker.ComposeService{{Service: "server", State: "running", Status: "Up"}}, running: true},
		{name: "auth is advisory", services: []paneldocker.ComposeService{{Service: "server", State: "running", Status: "Up"}, {Service: "steam-auth", State: "exited"}}, running: true},
		{name: "server absent"},
		{name: "server exited", services: []paneldocker.ComposeService{{Service: "server", State: "exited"}}},
		{name: "server unknown", services: []paneldocker.ComposeService{{Service: "server", State: "mystery"}}, wantErr: true},
		{name: "duplicate server", services: []paneldocker.ComposeService{{Service: "server", State: "running"}, {Service: "server", State: "running"}}, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			running, err := saveImportRuntimeServicesRunningStrict(tc.services, []string{"server"})
			if running != tc.running || (err != nil) != tc.wantErr {
				t.Fatalf("running=%v err=%v", running, err)
			}
		})
	}
}

func TestSaveImportDefaultStopBudgetsExceedDockerCommandTimeouts(t *testing.T) {
	maintenance := defaultImportMaintenanceOptions()
	phaseA := defaultImportPhaseAOptions()
	if maintenance.StopTimeout <= paneldocker.DefaultDownTimeout || phaseA.StopTimeout <= paneldocker.DefaultDownTimeout {
		t.Fatalf("maintenance stop=%v Phase A stop=%v, both must exceed Docker Down timeout=%v",
			maintenance.StopTimeout, phaseA.StopTimeout, paneldocker.DefaultDownTimeout)
	}
	if saveImportRuntimeFinalProbeTimeout <= paneldocker.DefaultPsTimeout {
		t.Fatalf("final strict probe=%v must exceed Docker Ps timeout=%v",
			saveImportRuntimeFinalProbeTimeout, paneldocker.DefaultPsTimeout)
	}
	if importActivationRollbackStopTimeout <= paneldocker.DefaultDownTimeout {
		t.Fatalf("activation rollback stop=%v must exceed Docker Down timeout=%v",
			importActivationRollbackStopTimeout, paneldocker.DefaultDownTimeout)
	}
}

func TestStopImportPhaseAServerUsesFreshStrictProofAfterExpiredStopContext(t *testing.T) {
	dataDir := t.TempDir()
	fake, _ := newMaintenanceFake(maintenanceFakeConfig{composeDownErrorAfterStop: true})
	probeSawFreshContext := false
	fake.composePsStrictFunc = func(ctx context.Context, _ string) (paneldocker.ComposePsResult, error) {
		if err := ctx.Err(); err != nil {
			return paneldocker.ComposePsResult{}, fmt.Errorf("strict proof reused expired stop context: %w", err)
		}
		probeSawFreshContext = true
		return paneldocker.ComposePsResult{Services: []paneldocker.ComposeService{
			{Service: "server", State: "exited", Status: "Exited (0)"},
		}}, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := stopImportPhaseAServer(ctx, fake, dataDir, time.Millisecond); err != nil {
		t.Fatalf("fresh strict stopped proof was rejected after ambiguous stop result: %v", err)
	}
	if !probeSawFreshContext {
		t.Fatal("strict stopped proof was not executed with a fresh context")
	}
}

func TestStopImportPhaseAServerKeepsFullIndependentProbeBudgetAfterStopDeadline(t *testing.T) {
	dataDir := t.TempDir()
	fake, record := newMaintenanceFake(maintenanceFakeConfig{})
	fake.stopServicesFunc = func(ctx context.Context, _, _ string, _ ...string) error {
		<-ctx.Done()
		record.mu.Lock()
		record.down = true
		record.started = false
		record.mu.Unlock()
		return paneldocker.ErrCommandTimeout
	}
	probeSawFullBudget := false
	fake.composePsStrictFunc = func(ctx context.Context, _ string) (paneldocker.ComposePsResult, error) {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) <= paneldocker.DefaultPsTimeout {
			return paneldocker.ComposePsResult{}, errors.New("strict proof did not receive its independent full budget")
		}
		probeSawFullBudget = true
		return paneldocker.ComposePsResult{Services: []paneldocker.ComposeService{
			{Service: "server", State: "exited", Status: "Exited (137)"},
		}}, nil
	}
	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	if err := stopImportPhaseAServer(stopCtx, fake, dataDir, time.Millisecond); err != nil {
		t.Fatalf("fresh strict proof failed after stop deadline: %v", err)
	}
	if !probeSawFullBudget {
		t.Fatal("strict proof did not run with a full independent budget")
	}
}

func TestStopImportPhaseAServerPollsAfterDockerTimeoutUntilStrictStop(t *testing.T) {
	dataDir := t.TempDir()
	fake, record := newMaintenanceFake(maintenanceFakeConfig{})
	fake.composeDownFunc = func(context.Context, string) (paneldocker.CommandResult, error) {
		record.mu.Lock()
		record.down = true
		record.mu.Unlock()
		return paneldocker.CommandResult{}, paneldocker.ErrCommandTimeout
	}
	probeCalls := 0
	fake.composePsStrictFunc = func(ctx context.Context, _ string) (paneldocker.ComposePsResult, error) {
		if err := ctx.Err(); err != nil {
			return paneldocker.ComposePsResult{}, err
		}
		probeCalls++
		if probeCalls == 1 {
			return paneldocker.ComposePsResult{Services: []paneldocker.ComposeService{
				{Service: "server", State: "running", Status: "Up"},
			}}, nil
		}
		return paneldocker.ComposePsResult{Services: []paneldocker.ComposeService{
			{Service: "server", State: "exited", Status: "Exited (137)"},
		}}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := stopImportPhaseAServer(ctx, fake, dataDir, time.Millisecond); err != nil {
		t.Fatalf("strict proof did not converge after Docker timeout: %v", err)
	}
	if probeCalls < 2 {
		t.Fatalf("strict proof calls=%d, want at least two", probeCalls)
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
	composeUpError, panicDuringComposeUp, neverRunning                  bool
	processIdentityChanges                                              bool
	composeDownError, composeDownErrorAfterStop                         bool
	strictStopError, strictStopUnknown, authStillRunningAfterStop       bool
	savesProbeFailures, diagnosticsFailures                             int
}

type maintenanceFakeRecord struct {
	mu                  sync.Mutex
	started, down       bool
	upCalls             int
	upServices          []string
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
	fake.recreateFunc = func(_ context.Context, _ string, services ...string) (paneldocker.CommandResult, error) {
		record.mu.Lock()
		record.upCalls++
		record.upServices = append([]string(nil), services...)
		record.started = true
		record.mu.Unlock()
		if cfg.panicDuringComposeUp {
			panic("injected maintenance ComposeUp panic")
		}
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
		if cfg.composeDownErrorAfterStop {
			return paneldocker.CommandResult{ExitCode: 1}, errors.New("injected ambiguous compose stop result")
		}
		return paneldocker.CommandResult{}, nil
	}
	fake.composePsFunc = func(context.Context, string) (paneldocker.ComposePsResult, error) {
		record.mu.Lock()
		started := record.started
		down := record.down
		record.mu.Unlock()
		if started && !cfg.neverRunning {
			return paneldocker.ComposePsResult{Services: []paneldocker.ComposeService{{Service: "server", State: "running", Status: "Up"}}}, nil
		}
		if down {
			// Scoped runtime Stop is non-destructive. Model the retained server
			// container so recovery tests prove exited/dead is accepted instead
			// of relying only on the older ComposeDown empty-project shape.
			return paneldocker.ComposePsResult{Services: []paneldocker.ComposeService{{Service: "server", State: "exited", Status: "Exited (0)"}}}, nil
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
		if down && cfg.authStillRunningAfterStop {
			return paneldocker.ComposePsResult{Services: []paneldocker.ComposeService{
				{Service: "server", State: "exited", Status: "Exited (0)"},
				{Service: "steam-auth", State: "running", Status: "Up"},
			}}, nil
		}
		return fake.ComposePs(ctx, dir)
	}
	fake.execFunc = func(ctx context.Context, _ string, _ string, stdin string, args ...string) (paneldocker.CommandResult, error) {
		record.mu.Lock()
		if stdin != "" {
			record.stdin = append(record.stdin, stdin)
			if stdin == "saves info Upload_1\n" {
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
				return paneldocker.CommandResult{Stdout: "$>saves info Upload_1\n"}, nil
			}
			return paneldocker.CommandResult{Stdout: "Save: Upload_1\n  Farm Type: Standard\n"}, nil
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

func TestImportMaintenanceScopesOptionalSteamAuthService(t *testing.T) {
	for _, tc := range []struct {
		name     string
		enabled  bool
		services []string
	}{
		{name: "disabled starts only server", services: []string{"server"}},
		{name: "enabled starts auth and server", enabled: true, services: []string{"steam-auth", "server"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dataDir, op, instance, store := prepareMaintenanceFixture(t)
			if err := sjconfig.SetSteamInviteEnabled(dataDir, tc.enabled); err != nil {
				t.Fatalf("set Steam invite intent: %v", err)
			}
			fake, record := newMaintenanceFake(maintenanceFakeConfig{})
			driver := New(fake, nil, nil, store)
			if err := driver.runImportMaintenance(context.Background(), instance, op, nil,
				importMaintenanceOptions{ReadyTimeout: 100 * time.Millisecond, PollInterval: time.Millisecond}); err != nil {
				t.Fatalf("runImportMaintenance: %v", err)
			}
			record.mu.Lock()
			got := append([]string(nil), record.upServices...)
			record.mu.Unlock()
			if strings.Join(got, ",") != strings.Join(tc.services, ",") {
				t.Fatalf("maintenance services = %v, want %v", got, tc.services)
			}
			journal, err := LoadImportJournal(dataDir, op)
			if err != nil || journal.MaintenanceSteamInviteEnabled == nil || *journal.MaintenanceSteamInviteEnabled != tc.enabled {
				t.Fatalf("frozen maintenance invite scope=%v err=%v, want enabled=%v", journal.MaintenanceSteamInviteEnabled, err, tc.enabled)
			}
		})
	}
}

func TestImportMaintenanceRollbackUsesFrozenEnabledScopeAfterEnvChanges(t *testing.T) {
	dataDir, op, instance, store := prepareMaintenanceFixture(t)
	if err := sjconfig.SetSteamInviteEnabled(dataDir, true); err != nil {
		t.Fatal(err)
	}
	fake, record := newMaintenanceFake(maintenanceFakeConfig{composeUpError: true})
	baseRecreate := fake.recreateFunc
	var intentUpdateErr error
	fake.recreateFunc = func(ctx context.Context, dir string, services ...string) (paneldocker.CommandResult, error) {
		result, err := baseRecreate(ctx, dir, services...)
		intentUpdateErr = sjconfig.SetSteamInviteEnabled(dataDir, false)
		return result, err
	}
	var stoppedServices []string
	fake.stopServicesFunc = func(_ context.Context, _, _ string, services ...string) error {
		stoppedServices = append([]string(nil), services...)
		record.mu.Lock()
		record.down = true
		record.started = false
		record.mu.Unlock()
		return nil
	}
	driver := New(fake, nil, nil, store)
	if err := driver.runImportMaintenance(context.Background(), instance, op, nil,
		importMaintenanceOptions{ReadyTimeout: time.Second, PollInterval: time.Millisecond}); err == nil {
		t.Fatal("injected startup failure unexpectedly succeeded")
	}
	if intentUpdateErr != nil {
		t.Fatalf("change invite intent after frozen startup scope: %v", intentUpdateErr)
	}
	if strings.Join(stoppedServices, ",") != "server,steam-auth" {
		t.Fatalf("rollback stop services=%v, want frozen enabled scope", stoppedServices)
	}
	journal, err := LoadImportJournal(dataDir, op)
	if err != nil || journal.MaintenanceSteamInviteEnabled == nil || !*journal.MaintenanceSteamInviteEnabled ||
		journal.MaintenanceStarted || journal.MaintenanceRecoveryState != importMaintenanceSnapshotRestored {
		t.Fatalf("journal=%+v err=%v", journal, err)
	}
}

func TestImportMaintenanceMalformedInviteIntentDoesNotStartRuntime(t *testing.T) {
	dataDir, op, instance, store := prepareMaintenanceFixture(t)
	if err := sjconfig.UpdateEnvFile(filepath.Join(dataDir, ".env"), map[string]string{"STEAM_INVITE_ENABLED": "maybe"}); err != nil {
		t.Fatal(err)
	}
	fake, record := newMaintenanceFake(maintenanceFakeConfig{})
	driver := New(fake, nil, nil, store)
	err := driver.runImportMaintenance(context.Background(), instance, op, nil,
		importMaintenanceOptions{ReadyTimeout: time.Second, PollInterval: time.Millisecond})
	typed, ok := AsImportTransactionError(err)
	if !ok || typed.Code != ImportErrorMaintenanceStart {
		t.Fatalf("error=%T %v, want maintenance start failure", err, err)
	}
	record.mu.Lock()
	upCalls := record.upCalls
	record.mu.Unlock()
	if upCalls != 0 {
		t.Fatalf("maintenance runtime starts=%d with malformed invite intent", upCalls)
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

func TestImportMaintenanceWaitsUntilJunimoCanReadStagedTarget(t *testing.T) {
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

func TestSaveInfoReadinessOutputMustNameExactTarget(t *testing.T) {
	if !isSaveInfoOutput("[JunimoServer] Save: Upload_1\n  Farm Type: Standard", "Upload_1") {
		t.Fatal("exact target info response was not recognized")
	}
	for _, output := range []string{
		"Available Saves:\n    Upload_1",
		"Save 'Upload_1' not found.",
		"Save: Other_1",
	} {
		if isSaveInfoOutput(output, "Upload_1") {
			t.Fatalf("non-target response was accepted: %q", output)
		}
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
	if err := os.WriteFile(filepath.Join(saveDir, saveName+"_old"), []byte("old-save"), 0o600); err != nil {
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
	if err := os.WriteFile(filepath.Join(controlDir(dataDir), "options.json"), []byte(readyControlRuntimeOptions(runtimeManifest.Control.Version)), 0o600); err != nil {
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

func prepareCompletedMaintenanceFixture(t *testing.T) (string, string, registry.Instance, *importMaintenanceStore) {
	t.Helper()
	dataDir, operationID, instance, store := prepareMaintenanceFixture(t)
	if err := sjconfig.UpdateEnvFile(filepath.Join(dataDir, ".env"), map[string]string{
		"STEAM_INVITE_ENABLED":               "false",
		"STEAM_INVITE_RUNTIME_SCOPE_VERSION": sjconfig.SteamInviteRuntimeScopeVersion,
	}); err != nil {
		t.Fatal(err)
	}
	journal, err := LoadImportJournal(dataDir, operationID)
	if err != nil {
		t.Fatal(err)
	}
	disabled := false
	journal.Stage = ImportStageCompleted
	journal.MaintenanceStarted = true
	journal.MaintenanceSteamInviteEnabled = &disabled
	if err := WriteImportJournal(dataDir, journal); err != nil {
		t.Fatal(err)
	}
	payload := `{"save_import_operation_id":"` + operationID + `"}`
	store.mu.Lock()
	store.instance.State = storage.InstanceStateStopped
	store.instance.StateMessage = sql.NullString{String: "private maintenance", Valid: true}
	store.instance.DriverPhase = importMaintenancePhase
	store.instance.DriverPayload = payload
	store.mu.Unlock()
	instance.State = storage.InstanceStateStopped
	instance.StateMessage = "private maintenance"
	instance.DriverPhase = importMaintenancePhase
	instance.DriverPayload = payload
	return dataDir, operationID, instance, store
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
	if !strings.Contains(commands, "saves info Upload_1\n") || strings.Contains(commands, "saves import") || strings.Contains(commands, "newgame") {
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

func TestImportMaintenanceAmbiguousStopErrorWithStrictStoppedProofRestoresSnapshot(t *testing.T) {
	dataDir, op, instance, store := prepareMaintenanceFixture(t)
	store.mu.Lock()
	original := store.instance
	store.mu.Unlock()
	fake, _ := newMaintenanceFake(maintenanceFakeConfig{
		composeUpError: true, composeDownErrorAfterStop: true,
	})
	driver := New(fake, nil, nil, store)
	err := driver.runImportMaintenance(context.Background(), instance, op, nil,
		importMaintenanceOptions{ReadyTimeout: time.Second, PollInterval: time.Millisecond, StopTimeout: 50 * time.Millisecond})
	typed, ok := AsImportTransactionError(err)
	if !ok || typed.Code != ImportErrorMaintenanceStart {
		t.Fatalf("error=%T %v", err, err)
	}
	store.mu.Lock()
	restored := store.instance
	store.mu.Unlock()
	if restored.State != original.State || restored.StateMessage != original.StateMessage ||
		restored.DriverPhase != original.DriverPhase || restored.DriverPayload != original.DriverPayload {
		t.Fatalf("strictly stopped runtime did not restore exact snapshot: got=%+v want=%+v", restored, original)
	}
	journal, loadErr := LoadImportJournal(dataDir, op)
	if loadErr != nil || journal.MaintenanceStarted || journal.MaintenanceRecoveryState != importMaintenanceSnapshotRestored {
		t.Fatalf("journal=%+v err=%v", journal, loadErr)
	}
}

func TestImportMaintenancePanicAfterRuntimeMayStartStopsAndRestoresSnapshot(t *testing.T) {
	dataDir, op, instance, store := prepareMaintenanceFixture(t)
	store.mu.Lock()
	original := store.instance
	store.mu.Unlock()
	fake, record := newMaintenanceFake(maintenanceFakeConfig{panicDuringComposeUp: true})
	driver := New(fake, nil, nil, store)
	var panicValue any
	func() {
		defer func() { panicValue = recover() }()
		_ = driver.runImportMaintenance(context.Background(), instance, op, nil,
			importMaintenanceOptions{ReadyTimeout: time.Second, PollInterval: time.Millisecond})
	}()
	if panicValue == nil {
		t.Fatal("maintenance ComposeUp panic was swallowed")
	}
	if strings.Contains(fmt.Sprint(panicValue), "injected maintenance ComposeUp panic") {
		t.Fatalf("maintenance panic leaked its raw internal value: %v", panicValue)
	}
	record.mu.Lock()
	down := record.down
	record.mu.Unlock()
	if !down {
		t.Fatal("panicked maintenance runtime was not stopped")
	}
	store.mu.Lock()
	restored := store.instance
	store.mu.Unlock()
	if restored.State != original.State || restored.StateMessage != original.StateMessage ||
		restored.DriverPhase != original.DriverPhase || restored.DriverPayload != original.DriverPayload {
		t.Fatalf("panicked maintenance did not restore exact snapshot: got=%+v want=%+v", restored, original)
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
		enableInvite     bool
	}{
		{name: "compose down failure", cfg: maintenanceFakeConfig{composeUpError: true, composeDownError: true}},
		{name: "strict probe failure", cfg: maintenanceFakeConfig{composeUpError: true, strictStopError: true}},
		{name: "strict unknown state", cfg: maintenanceFakeConfig{composeUpError: true, strictStopUnknown: true}},
		{name: "enabled auth remains running", cfg: maintenanceFakeConfig{composeUpError: true, authStillRunningAfterStop: true}, enableInvite: true},
		{name: "MaintenanceStarted false write failure", cfg: maintenanceFakeConfig{composeUpError: true}, failJournalWrite: 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dataDir, op, instance, store := prepareMaintenanceFixture(t)
			if tc.enableInvite {
				if err := sjconfig.SetSteamInviteEnabled(dataDir, true); err != nil {
					t.Fatal(err)
				}
			}
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
				importMaintenanceOptions{ReadyTimeout: time.Second, PollInterval: time.Millisecond, StopTimeout: 20 * time.Millisecond})
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
		{name: "flag clear before snapshot restore", maintenanceState: importMaintenanceSnapshotRestorePending, recoveryState: importRecoveryMaintenanceRestoreSnapshot, wantDown: true},
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

func TestPrepareOrphanedImportMaintenanceStopsConservativeScope(t *testing.T) {
	for _, tc := range []struct {
		name   string
		damage func(*testing.T, string, string)
	}{
		{
			name: "missing transaction directory",
			damage: func(t *testing.T, dataDir, _ string) {
				t.Helper()
				if err := os.RemoveAll(importTransactionsDir(dataDir)); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "unreadable journal",
			damage: func(t *testing.T, dataDir, operationID string) {
				t.Helper()
				if err := os.WriteFile(importJournalPath(dataDir, operationID), []byte("{not-json"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dataDir, operationID, instance, store := prepareMaintenanceFixture(t)
			store.mu.Lock()
			store.instance.State = storage.InstanceStateStopped
			store.instance.StateMessage = sql.NullString{String: "private maintenance", Valid: true}
			store.instance.DriverPhase = importMaintenancePhase
			store.instance.DriverPayload = `{"maintenance":true}`
			store.mu.Unlock()
			instance.State = storage.InstanceStateStopped
			instance.StateMessage = "private maintenance"
			instance.DriverPhase = importMaintenancePhase
			instance.DriverPayload = `{"maintenance":true}`
			tc.damage(t, dataDir, operationID)

			fake, record := newMaintenanceFake(maintenanceFakeConfig{})
			record.started = true
			var stoppedServices []string
			fake.stopServicesFunc = func(_ context.Context, _, _ string, services ...string) error {
				stoppedServices = append([]string(nil), services...)
				record.mu.Lock()
				record.down = true
				record.started = false
				record.mu.Unlock()
				return nil
			}
			driver := New(fake, nil, nil, store)
			err := driver.Prepare(context.Background(), instance)
			typed, ok := AsImportTransactionError(err)
			if !ok || typed.Code != ImportErrorRecoveryRequired {
				t.Fatalf("Prepare error=%T %v, want recovery_required", err, err)
			}
			if got := strings.Join(stoppedServices, ","); got != "server,steam-auth" {
				t.Fatalf("orphaned maintenance stop services=%v, want conservative server+Auth", stoppedServices)
			}
			record.mu.Lock()
			down, started := record.down, record.started
			record.mu.Unlock()
			if !down || started {
				t.Fatalf("orphaned maintenance runtime was not stopped: down=%v started=%v", down, started)
			}
			store.mu.Lock()
			current := store.instance
			store.mu.Unlock()
			if current.DriverPhase != importMaintenancePhase || current.DriverPayload != `{"maintenance":true}` {
				t.Fatalf("orphaned maintenance incorrectly restored or rewrote instance state: %+v", current)
			}
		})
	}
}

func TestPrepareMalformedCompletedMaintenanceOwnerStopsConservativeScope(t *testing.T) {
	for _, tc := range []struct {
		name    string
		payload func(string) string
	}{
		{name: "invalid json", payload: func(string) string { return `{not-json` }},
		{name: "null payload", payload: func(string) string { return `null` }},
		{name: "empty owner", payload: func(string) string { return `{"save_import_operation_id":""}` }},
		{name: "null owner", payload: func(string) string { return `{"save_import_operation_id":null}` }},
		{name: "numeric owner", payload: func(string) string { return `{"save_import_operation_id":7}` }},
		{name: "invalid owner", payload: func(string) string { return `{"save_import_operation_id":"not-an-operation"}` }},
		{name: "different valid owner", payload: func(string) string { return `{"save_import_operation_id":"` + NewImportOperationID() + `"}` }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dataDir, operationID, instance, store := prepareCompletedMaintenanceFixture(t)
			payload := tc.payload(operationID)
			store.mu.Lock()
			store.instance.DriverPayload = payload
			original := store.instance
			store.mu.Unlock()
			instance.DriverPayload = payload

			fake, record := newMaintenanceFake(maintenanceFakeConfig{})
			record.started = true
			var stoppedServices []string
			stopCalls := 0
			fake.stopServicesFunc = func(_ context.Context, _, _ string, services ...string) error {
				stopCalls++
				stoppedServices = append([]string(nil), services...)
				record.mu.Lock()
				record.down = true
				record.started = false
				record.mu.Unlock()
				return nil
			}
			driver := New(fake, slog.Default(), nil, store)
			err := driver.Prepare(context.Background(), instance)
			typed, ok := AsImportTransactionError(err)
			if !ok || typed.Code != ImportErrorRecoveryRequired {
				t.Fatalf("Prepare error=%T %v, want recovery_required", err, err)
			}
			if stopCalls != 1 || strings.Join(stoppedServices, ",") != "server,steam-auth" {
				t.Fatalf("stop calls=%d services=%v, want one conservative stop", stopCalls, stoppedServices)
			}
			record.mu.Lock()
			upCalls := record.upCalls
			record.mu.Unlock()
			if upCalls != 0 {
				t.Fatalf("malformed owner recreated runtime %d time(s)", upCalls)
			}
			store.mu.Lock()
			current := store.instance
			store.mu.Unlock()
			if current.State != original.State || current.StateMessage != original.StateMessage ||
				current.DriverPhase != original.DriverPhase || current.DriverPayload != original.DriverPayload {
				t.Fatalf("malformed owner changed authoritative state: got=%+v want=%+v", current, original)
			}
			journal, loadErr := LoadImportJournal(dataDir, operationID)
			if loadErr != nil || journal.Stage != ImportStageCompleted || journal.RollbackState != "" {
				t.Fatalf("completed journal changed: %+v err=%v", journal, loadErr)
			}
		})
	}
}

func TestPrepareMultipleLegacyCompletedMaintenanceOwnersStopsConservativeScope(t *testing.T) {
	dataDir, operationID, instance, store := prepareCompletedMaintenanceFixture(t)
	first, err := LoadImportJournal(dataDir, operationID)
	if err != nil {
		t.Fatal(err)
	}
	second := first
	second.OperationID = NewImportOperationID()
	if err := WriteImportJournal(dataDir, second); err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	store.instance.DriverPayload = `{}`
	store.instance.UpdatedAt = time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano)
	original := store.instance
	store.mu.Unlock()
	instance.DriverPayload = `{}`

	fake, record := newMaintenanceFake(maintenanceFakeConfig{})
	record.started = true
	stopCalls := 0
	var stoppedServices []string
	fake.stopServicesFunc = func(_ context.Context, _, _ string, services ...string) error {
		stopCalls++
		stoppedServices = append([]string(nil), services...)
		record.mu.Lock()
		record.down = true
		record.started = false
		record.mu.Unlock()
		return nil
	}
	driver := New(fake, slog.Default(), nil, store)
	err = driver.Prepare(context.Background(), instance)
	typed, ok := AsImportTransactionError(err)
	if !ok || typed.Code != ImportErrorRecoveryRequired {
		t.Fatalf("Prepare error=%T %v, want recovery_required", err, err)
	}
	if stopCalls != 1 || strings.Join(stoppedServices, ",") != "server,steam-auth" {
		t.Fatalf("stop calls=%d services=%v, want one conservative stop", stopCalls, stoppedServices)
	}
	store.mu.Lock()
	current := store.instance
	store.mu.Unlock()
	if current.State != original.State || current.StateMessage != original.StateMessage ||
		current.DriverPhase != original.DriverPhase || current.DriverPayload != original.DriverPayload {
		t.Fatalf("ambiguous legacy owners changed authoritative state: got=%+v want=%+v", current, original)
	}
}

func TestPrepareDoesNotRecoverCompletedJournalWhileImportRunnerIsActive(t *testing.T) {
	dataDir, operationID, _, _ := prepareMaintenanceFixture(t)
	journal, err := LoadImportJournal(dataDir, operationID)
	if err != nil {
		t.Fatal(err)
	}
	journal.Stage = ImportStageCompleted
	if err := WriteImportJournal(dataDir, journal); err != nil {
		t.Fatal(err)
	}
	store := newLifecycleTestStore(t)
	stored, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{
		ID: "stardew", DriverID: DriverID, Name: "Stardew Valley", DataDir: dataDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	payload := `{"save_import_operation_id":"` + operationID + `"}`
	stored, err = store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID: stored.ID, State: storage.InstanceStateStopped, StateMessage: "private maintenance",
		DriverPhase: importMaintenancePhase, DriverPayload: payload,
	})
	if err != nil {
		t.Fatal(err)
	}
	manager := jobs.NewManager(store, slog.Default())
	started := make(chan struct{})
	release := make(chan struct{})
	runnerDone := make(chan struct{})
	released := false
	defer func() {
		if !released {
			close(release)
		}
	}()
	job, err := manager.Start(context.Background(), jobs.Spec{
		Type: SaveImportJobType, TargetType: "instance", TargetID: stored.ID,
		Run: func(context.Context, *jobs.Context) error {
			close(started)
			<-release
			close(runnerDone)
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("save import runner did not start")
	}
	fake, record := newMaintenanceFake(maintenanceFakeConfig{})
	record.started = true
	stopCalls := 0
	fake.stopServicesFunc = func(context.Context, string, string, ...string) error {
		stopCalls++
		return nil
	}
	driver := New(fake, slog.Default(), manager, store)
	instance := registry.Instance{
		ID: stored.ID, DriverID: stored.DriverID, Name: stored.Name, DataDir: stored.DataDir,
		State: stored.State, StateMessage: stored.StateMessage.String, DriverPhase: stored.DriverPhase, DriverPayload: stored.DriverPayload,
	}
	err = driver.Prepare(context.Background(), instance)
	typed, ok := AsImportTransactionError(err)
	if !ok || typed.Code != ImportErrorBusy {
		t.Fatalf("Prepare error=%T %v, want active import busy", err, err)
	}
	if stopCalls != 0 {
		t.Fatalf("concurrent Prepare stopped active import runtime %d time(s)", stopCalls)
	}
	close(release)
	released = true
	select {
	case <-runnerDone:
	case <-time.After(time.Second):
		t.Fatal("save import runner did not finish")
	}
	deadline := time.Now().Add(time.Second)
	for {
		current, getErr := manager.Get(context.Background(), job.ID)
		if getErr == nil && current.Status != storage.JobStatusQueued && current.Status != storage.JobStatusRunning {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("save import job did not reach terminal state: job=%+v err=%v", current, getErr)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestSaveImportOperationIDFromMaintenancePayloadRejectsMalformedOwner(t *testing.T) {
	operationID := NewImportOperationID()
	for _, tc := range []struct {
		name    string
		payload string
		want    string
		wantErr bool
	}{
		{name: "empty payload"},
		{name: "missing owner", payload: `{"other":true}`},
		{name: "valid owner", payload: `{"save_import_operation_id":"` + operationID + `"}`, want: operationID},
		{name: "null payload", payload: `null`, wantErr: true},
		{name: "empty owner", payload: `{"save_import_operation_id":""}`, wantErr: true},
		{name: "null owner", payload: `{"save_import_operation_id":null}`, wantErr: true},
		{name: "numeric owner", payload: `{"save_import_operation_id":7}`, wantErr: true},
		{name: "malformed owner", payload: `{"save_import_operation_id":"not-an-operation"}`, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := saveImportOperationIDFromMaintenancePayload(tc.payload)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("owner=%q, want fail-closed error", got)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("owner=%q err=%v, want %q", got, err, tc.want)
			}
		})
	}
}

func TestCompletedMaintenanceStatePublicationRejectsMalformedOwner(t *testing.T) {
	for _, tc := range []struct {
		name    string
		payload func(string) string
	}{
		{name: "null payload", payload: func(string) string { return `null` }},
		{name: "empty owner", payload: func(string) string { return `{"save_import_operation_id":""}` }},
		{name: "null owner", payload: func(string) string { return `{"save_import_operation_id":null}` }},
		{name: "numeric owner", payload: func(string) string { return `{"save_import_operation_id":7}` }},
		{name: "malformed owner", payload: func(string) string { return `{"save_import_operation_id":"not-an-operation"}` }},
		{name: "different valid owner", payload: func(string) string { return `{"save_import_operation_id":"` + NewImportOperationID() + `"}` }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, operationID, instance, store := prepareMaintenanceFixture(t)
			payload := tc.payload(operationID)
			store.mu.Lock()
			store.instance.State = storage.InstanceStateStopped
			store.instance.StateMessage = sql.NullString{String: "private maintenance", Valid: true}
			store.instance.DriverPhase = importMaintenancePhase
			store.instance.DriverPayload = payload
			store.mu.Unlock()

			driver := New(nil, slog.Default(), nil, store)
			err := driver.markCompletedImportRuntimeRunning(instance, operationID, false)
			typed, ok := AsImportTransactionError(err)
			if !ok || typed.Code != ImportErrorRecoveryRequired {
				t.Fatalf("publication error=%T %v, want recovery_required", err, err)
			}
			store.mu.Lock()
			current := store.instance
			store.mu.Unlock()
			if current.State != storage.InstanceStateStopped || current.DriverPhase != importMaintenancePhase || current.DriverPayload != payload {
				t.Fatalf("malformed owner changed authoritative state: %+v", current)
			}
		})
	}
}

func TestCompletedMaintenanceStatePublicationRejectsInvalidOperationIDArgument(t *testing.T) {
	_, _, instance, store := prepareMaintenanceFixture(t)
	store.mu.Lock()
	store.instance.State = storage.InstanceStateStopped
	store.instance.DriverPhase = importMaintenancePhase
	store.instance.DriverPayload = `{}`
	store.mu.Unlock()
	driver := New(nil, slog.Default(), nil, store)
	err := driver.markCompletedImportRuntimeRunning(instance, "not-an-operation", false)
	typed, ok := AsImportTransactionError(err)
	if !ok || typed.Code != ImportErrorRecoveryRequired {
		t.Fatalf("publication error=%T %v, want recovery_required", err, err)
	}
	store.mu.Lock()
	current := store.instance
	store.mu.Unlock()
	if current.State != storage.InstanceStateStopped || current.DriverPhase != importMaintenancePhase || current.DriverPayload != `{}` {
		t.Fatalf("invalid operation argument changed authoritative state: %+v", current)
	}
}

func TestPrepareRecoversCompletedMaintenanceCommitWithFrozenRuntimeScope(t *testing.T) {
	dataDir, operationID, instance, store := prepareMaintenanceFixture(t)
	journal, err := LoadImportJournal(dataDir, operationID)
	if err != nil {
		t.Fatal(err)
	}
	enabled := true
	journal.Stage = ImportStageCompleted
	journal.MaintenanceStarted = true
	journal.MaintenanceSteamInviteEnabled = &enabled
	if err := WriteImportJournal(dataDir, journal); err != nil {
		t.Fatal(err)
	}
	if err := sjconfig.UpdateEnvFile(filepath.Join(dataDir, ".env"), map[string]string{
		"STEAM_INVITE_ENABLED":               "true",
		"STEAM_INVITE_RUNTIME_SCOPE_VERSION": sjconfig.SteamInviteRuntimeScopeVersion,
	}); err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	store.instance.State = storage.InstanceStateStopped
	store.instance.StateMessage = sql.NullString{String: "private maintenance", Valid: true}
	store.instance.DriverPhase = importMaintenancePhase
	store.instance.DriverPayload = `{"other":"kept","save_import_operation_id":"` + operationID + `"}`
	store.mu.Unlock()
	instance.State = storage.InstanceStateStopped
	instance.StateMessage = "private maintenance"
	instance.DriverPhase = importMaintenancePhase
	instance.DriverPayload = `{"other":"kept","save_import_operation_id":"` + operationID + `"}`
	fake, record := newMaintenanceFake(maintenanceFakeConfig{})
	record.started = true
	fake.composePsStrictFunc = func(context.Context, string) (paneldocker.ComposePsResult, error) {
		return paneldocker.ComposePsResult{Services: []paneldocker.ComposeService{
			{Service: "server", State: "running", Status: "Up"},
			{Service: "steam-auth", State: "exited", Status: "Exited (1)"},
		}}, nil
	}
	stopCalls := 0
	fake.stopServicesFunc = func(context.Context, string, string, ...string) error {
		stopCalls++
		return nil
	}
	driver := New(fake, slog.Default(), nil, store)
	if err := driver.Prepare(context.Background(), instance); err != nil {
		t.Fatalf("Prepare completed maintenance recovery: %v", err)
	}
	if stopCalls != 0 {
		t.Fatalf("completed maintenance recovery stopped the committed runtime %d time(s)", stopCalls)
	}
	store.mu.Lock()
	current := store.instance
	store.mu.Unlock()
	if current.State != storage.InstanceStateRunning || current.DriverPhase != "running" ||
		strings.Contains(current.DriverPayload, saveImportOperationIDPayloadKey) || !strings.Contains(current.DriverPayload, "kept") ||
		!strings.Contains(current.DriverPayload, steamInviteWarmupStartedAtPayloadKey) {
		t.Fatalf("completed maintenance state was not committed exactly once: %+v", current)
	}
}

func TestPrepareCompletedMaintenanceVerificationFailureDoesNotStopCommittedRuntime(t *testing.T) {
	for _, tc := range []struct {
		name     string
		services []paneldocker.ComposeService
		probeErr error
	}{
		{name: "probe error", probeErr: errors.New("injected completed runtime probe failure")},
		{name: "server absent"},
		{name: "server exited", services: []paneldocker.ComposeService{{Service: "server", State: "exited"}}},
		{name: "server unknown", services: []paneldocker.ComposeService{{Service: "server", State: "mystery"}}},
		{name: "duplicate server", services: []paneldocker.ComposeService{{Service: "server", State: "running"}, {Service: "server", State: "running"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dataDir, operationID, instance, store := prepareCompletedMaintenanceFixture(t)
			store.mu.Lock()
			original := store.instance
			store.mu.Unlock()
			fake, record := newMaintenanceFake(maintenanceFakeConfig{})
			record.started = true
			fake.composePsStrictFunc = func(context.Context, string) (paneldocker.ComposePsResult, error) {
				return paneldocker.ComposePsResult{Services: tc.services}, tc.probeErr
			}
			stopCalls := 0
			fake.stopServicesFunc = func(context.Context, string, string, ...string) error {
				stopCalls++
				return nil
			}
			driver := New(fake, slog.Default(), nil, store)
			err := driver.Prepare(context.Background(), instance)
			typed, ok := AsImportTransactionError(err)
			if !ok || typed.Code != ImportErrorRecoveryRequired {
				t.Fatalf("Prepare error=%T %v, want recovery_required", err, err)
			}
			if stopCalls != 0 {
				t.Fatalf("verified completed owner was stopped %d time(s)", stopCalls)
			}
			store.mu.Lock()
			current := store.instance
			store.mu.Unlock()
			if current.State != original.State || current.StateMessage != original.StateMessage ||
				current.DriverPhase != original.DriverPhase || current.DriverPayload != original.DriverPayload {
				t.Fatalf("failed completed verification changed state: got=%+v want=%+v", current, original)
			}
			journal, loadErr := LoadImportJournal(dataDir, operationID)
			if loadErr != nil || journal.Stage != ImportStageCompleted || journal.RollbackState != "" {
				t.Fatalf("failed verification changed completed journal: %+v err=%v", journal, loadErr)
			}
		})
	}
}

func TestPrepareCompletedMaintenanceStateUpdateFailureRetriesWithoutStop(t *testing.T) {
	_, operationID, instance, store := prepareCompletedMaintenanceFixture(t)
	store.mu.Lock()
	store.updateErr = errors.New("injected completed state update failure")
	store.mu.Unlock()
	fake, record := newMaintenanceFake(maintenanceFakeConfig{})
	record.started = true
	fake.composePsStrictFunc = func(context.Context, string) (paneldocker.ComposePsResult, error) {
		return paneldocker.ComposePsResult{Services: []paneldocker.ComposeService{{Service: "server", State: "running", Status: "Up"}}}, nil
	}
	stopCalls := 0
	fake.stopServicesFunc = func(context.Context, string, string, ...string) error {
		stopCalls++
		return nil
	}
	driver := New(fake, slog.Default(), nil, store)
	err := driver.Prepare(context.Background(), instance)
	typed, ok := AsImportTransactionError(err)
	if !ok || typed.Code != ImportErrorRecoveryRequired {
		t.Fatalf("first Prepare error=%T %v, want recovery_required", err, err)
	}
	if stopCalls != 0 {
		t.Fatalf("state publication failure stopped committed runtime %d time(s)", stopCalls)
	}
	store.mu.Lock()
	if store.instance.DriverPhase != importMaintenancePhase || !strings.Contains(store.instance.DriverPayload, operationID) {
		store.mu.Unlock()
		t.Fatalf("failed publication lost maintenance owner: %+v", store.instance)
	}
	store.updateErr = nil
	store.mu.Unlock()
	if err := driver.Prepare(context.Background(), instance); err != nil {
		t.Fatalf("retry Prepare: %v", err)
	}
	if stopCalls != 0 {
		t.Fatalf("successful retry stopped committed runtime %d time(s)", stopCalls)
	}
	store.mu.Lock()
	current := store.instance
	store.mu.Unlock()
	if current.State != storage.InstanceStateRunning || current.DriverPhase != "running" || strings.Contains(current.DriverPayload, saveImportOperationIDPayloadKey) {
		t.Fatalf("retry did not publish completed state: %+v", current)
	}
}

func TestCompletedMaintenanceStateUpdateFailureIsRetryable(t *testing.T) {
	dataDir, operationID, instance, store := prepareMaintenanceFixture(t)
	journal, err := LoadImportJournal(dataDir, operationID)
	if err != nil {
		t.Fatal(err)
	}
	disabled := false
	journal.Stage = ImportStageCompleted
	journal.MaintenanceSteamInviteEnabled = &disabled
	if err := WriteImportJournal(dataDir, journal); err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	store.instance.DriverPhase = importMaintenancePhase
	store.instance.DriverPayload = `{"save_import_operation_id":"` + operationID + `"}`
	store.updateErr = errors.New("injected completed state update failure")
	store.mu.Unlock()
	driver := New(nil, slog.Default(), nil, store)
	err = driver.markCompletedImportRuntimeRunning(instance, operationID, false)
	typed, ok := AsImportTransactionError(err)
	if !ok || typed.Code != ImportErrorRecoveryRequired {
		t.Fatalf("first completion state error=%T %v, want recovery_required", err, err)
	}
	store.mu.Lock()
	if store.instance.DriverPhase != importMaintenancePhase {
		store.mu.Unlock()
		t.Fatalf("failed completion state update changed phase: %+v", store.instance)
	}
	store.updateErr = nil
	store.mu.Unlock()
	if err := driver.markCompletedImportRuntimeRunning(instance, operationID, false); err != nil {
		t.Fatalf("retry completed state update: %v", err)
	}
	store.mu.Lock()
	current := store.instance
	store.mu.Unlock()
	if current.State != storage.InstanceStateRunning || current.DriverPhase != "running" || strings.Contains(current.DriverPayload, saveImportOperationIDPayloadKey) {
		t.Fatalf("retry did not publish completed running state: %+v", current)
	}
	publishedPayload := current.DriverPayload
	if err := driver.markCompletedImportRuntimeRunning(instance, operationID, false); err != nil {
		t.Fatalf("idempotent completed state publication: %v", err)
	}
	store.mu.Lock()
	current = store.instance
	store.mu.Unlock()
	if current.State != storage.InstanceStateRunning || current.DriverPhase != "running" || current.DriverPayload != publishedPayload {
		t.Fatalf("idempotent publication changed completed state: %+v", current)
	}
	journal, err = LoadImportJournal(dataDir, operationID)
	if err != nil || journal.Stage != ImportStageCompleted {
		t.Fatalf("completed journal changed across state retry: %+v err=%v", journal, err)
	}
}

func TestCompletedMaintenanceEnabledStatePublicationIsIdempotent(t *testing.T) {
	_, operationID, instance, store := prepareCompletedMaintenanceFixture(t)
	store.mu.Lock()
	store.instance.DriverPayload = `{"kept":true,"save_import_operation_id":"` + operationID + `"}`
	store.mu.Unlock()
	driver := New(nil, slog.Default(), nil, store)
	if err := driver.markCompletedImportRuntimeRunning(instance, operationID, true); err != nil {
		t.Fatalf("first publication: %v", err)
	}
	store.mu.Lock()
	first := store.instance
	firstUpdateCount := len(store.updates)
	store.mu.Unlock()
	if !strings.Contains(first.DriverPayload, steamInviteWarmupStartedAtPayloadKey) {
		t.Fatalf("enabled publication omitted warmup marker: %+v", first)
	}
	time.Sleep(2 * time.Millisecond)
	if err := driver.markCompletedImportRuntimeRunning(instance, operationID, true); err != nil {
		t.Fatalf("idempotent publication: %v", err)
	}
	store.mu.Lock()
	second := store.instance
	secondUpdateCount := len(store.updates)
	store.mu.Unlock()
	if second.State != first.State || second.StateMessage != first.StateMessage || second.DriverPhase != first.DriverPhase ||
		second.DriverPayload != first.DriverPayload || secondUpdateCount != firstUpdateCount {
		t.Fatalf("idempotent publication rewrote completed state: first=%+v second=%+v updates=%d->%d",
			first, second, firstUpdateCount, secondUpdateCount)
	}
}

func TestCompletedMaintenanceStatePublicationDoesNotOverwriteOtherPhase(t *testing.T) {
	_, operationID, instance, store := prepareMaintenanceFixture(t)
	store.mu.Lock()
	store.instance.State = storage.InstanceStateStopped
	store.instance.StateMessage = sql.NullString{String: "runtime update recovery", Valid: true}
	store.instance.DriverPhase = "runtime_update_recovery"
	store.instance.DriverPayload = `{}`
	original := store.instance
	store.mu.Unlock()
	driver := New(nil, slog.Default(), nil, store)
	err := driver.markCompletedImportRuntimeRunning(instance, operationID, false)
	typed, ok := AsImportTransactionError(err)
	if !ok || typed.Code != ImportErrorRecoveryRequired {
		t.Fatalf("publication error=%T %v, want recovery_required", err, err)
	}
	store.mu.Lock()
	current := store.instance
	store.mu.Unlock()
	if current.State != original.State || current.StateMessage != original.StateMessage ||
		current.DriverPhase != original.DriverPhase || current.DriverPayload != original.DriverPayload {
		t.Fatalf("publication overwrote another phase: got=%+v want=%+v", current, original)
	}
}

func TestRecoverInterruptedSnapshotPendingRequiresFrozenAuthStopped(t *testing.T) {
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
	enabled := true
	journal.MaintenanceSteamInviteEnabled = &enabled
	journal.MaintenanceStarted = false
	journal.MaintenanceRecoveryState = importMaintenanceSnapshotRestorePending
	if err := WriteImportJournal(dataDir, journal); err != nil {
		t.Fatal(err)
	}
	fake, record := newMaintenanceFake(maintenanceFakeConfig{authStillRunningAfterStop: true})
	var stoppedServices []string
	fake.stopServicesFunc = func(_ context.Context, _, _ string, services ...string) error {
		stoppedServices = append([]string(nil), services...)
		record.mu.Lock()
		record.down = true
		record.started = false
		record.mu.Unlock()
		return nil
	}
	driver := New(fake, nil, nil, store)
	recoveries, err := RecoverImportTransactions(dataDir)
	if err != nil || len(recoveries) != 1 || recoveries[0].State != importRecoveryMaintenanceRestoreSnapshot {
		t.Fatalf("recoveries=%+v err=%v", recoveries, err)
	}
	recoveryCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	_, err = driver.recoverInterruptedImportMaintenance(recoveryCtx, instance, recoveries)
	cancel()
	typed, ok := AsImportTransactionError(err)
	if !ok || typed.Code != ImportErrorRecoveryRequired {
		t.Fatalf("recovery error=%T %v", err, err)
	}
	if strings.Join(stoppedServices, ",") != "server,steam-auth" {
		t.Fatalf("recovery stop services=%v, want frozen enabled scope", stoppedServices)
	}
	store.mu.Lock()
	current := store.instance
	store.mu.Unlock()
	if current.DriverPhase != importMaintenancePhase {
		t.Fatalf("snapshot restored while Auth remained running: %+v", current)
	}
	journal, err = LoadImportJournal(dataDir, op)
	if err != nil || journal.MaintenanceRecoveryState != importMaintenanceManualRecovery || journal.RecoveryState != "manual_required" {
		t.Fatalf("journal=%+v err=%v", journal, err)
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
	if err := sjconfig.SetSteamInviteEnabled(instance.DataDir, true); err != nil {
		t.Fatalf("enable Steam invite capability: %v", err)
	}
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
