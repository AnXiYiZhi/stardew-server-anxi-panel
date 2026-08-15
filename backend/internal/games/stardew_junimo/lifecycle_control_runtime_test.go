package stardew_junimo

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func TestStartAndRestartWaitForFreshControlRuntime(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		wantUp    int32
		wantReset int32
	}{
		{name: "ordinary start", operation: "start", wantUp: 1},
		{name: "restart", operation: "restart", wantReset: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newLifecycleTestStore(t)
			dataDir := filepath.Join(t.TempDir(), "stardew")
			instance := prepareControlLifecycleInstance(t, store, dataDir)

			var composeUps atomic.Int32
			var restarts atomic.Int32
			launched := make(chan struct{})
			var launchedOnce sync.Once
			markLaunched := func() { launchedOnce.Do(func() { close(launched) }) }
			fake := &fakeConsoleDocker{
				composeUpFunc: func(context.Context, string) (paneldocker.CommandResult, error) {
					composeUps.Add(1)
					markLaunched()
					return paneldocker.CommandResult{ExitCode: 0}, nil
				},
				restartFunc: func(context.Context, string, ...string) (paneldocker.CommandResult, error) {
					restarts.Add(1)
					markLaunched()
					return paneldocker.CommandResult{ExitCode: 0}, nil
				},
				composePsFunc: func(context.Context, string) (paneldocker.ComposePsResult, error) {
					return paneldocker.ComposePsResult{Services: []paneldocker.ComposeService{{
						Service: "server", State: "running", Status: "Up 1 second",
					}}}, nil
				},
			}
			manager := jobs.NewManager(store, slog.Default())
			driver := New(fake, slog.Default(), manager, store)
			driver.runtimeUpdateServerTimeout = 2 * time.Second
			runner := &lifecycleRunner{
				driver: driver, lifecycle: fake, instance: instance, operation: tt.operation,
			}
			job, err := manager.Start(context.Background(), jobs.Spec{
				Type: lifecycleJobType, TargetType: "instance", TargetID: instance.ID,
				Timeout: 5 * time.Second, Run: runner.run,
			})
			if err != nil {
				t.Fatal(err)
			}

			select {
			case <-launched:
			case <-time.After(2 * time.Second):
				t.Fatalf("%s never reached its Compose launch", tt.operation)
			}
			waitForControlLifecyclePhase(t, store, instance.ID, "control_runtime_starting")
			pendingJob, err := store.GetJob(context.Background(), job.ID)
			if err != nil {
				t.Fatal(err)
			}
			if pendingJob.Status != storage.JobStatusRunning {
				t.Fatalf("%s completed before fresh Control evidence: status=%s", tt.operation, pendingJob.Status)
			}

			manifest := InspectControlRuntimeGate(dataDir)
			if manifest.Expected == "" || manifest.State != ControlRuntimeGatePending {
				t.Fatalf("pre-write Control gate = %+v, want pending with expected version", manifest)
			}
			writeControlRuntimeOptions(t, dataDir, `{"controlModVersion":"`+manifest.Expected+`","hostFarmhousePreservationPatchAvailable":true}`)
			waitForDriverTestJobStatus(t, store, job.ID, storage.JobStatusSucceeded)

			updated, err := store.GetInstance(context.Background(), instance.ID)
			if err != nil {
				t.Fatal(err)
			}
			if updated.State != storage.InstanceStateRunning || updated.DriverPhase != "running" {
				t.Fatalf("%s final state = %+v, want running", tt.operation, updated)
			}
			if got := composeUps.Load(); got != tt.wantUp {
				t.Fatalf("%s ComposeUp calls = %d, want %d", tt.operation, got, tt.wantUp)
			}
			if got := restarts.Load(); got != tt.wantReset {
				t.Fatalf("%s restart calls = %d, want %d", tt.operation, got, tt.wantReset)
			}
		})
	}
}

func TestControlRuntimePendingTimeoutIsNotVersionMismatch(t *testing.T) {
	result := runControlRuntimeGateFailureTest(t, controlRuntimeFailureFixture{
		runtimeTimeout: 25 * time.Millisecond,
	})
	if result.instance.State != storage.InstanceStateStopped || result.instance.DriverPhase != "control_runtime_start_timeout" {
		t.Fatalf("pending timeout state = %+v, want stopped/control_runtime_start_timeout", result.instance)
	}
	if result.downCalls != 1 {
		t.Fatalf("pending timeout ComposeDown calls = %d, want 1", result.downCalls)
	}
	if strings.Contains(strings.ToLower(result.job.ErrorMessage.String), "version mismatch") ||
		strings.Contains(result.instance.DriverPhase, "version_mismatch") {
		t.Fatalf("pending timeout was mislabeled as version mismatch: job=%+v instance=%+v", result.job, result.instance)
	}
}

func TestControlRuntimeExplicitOldVersionIsMismatch(t *testing.T) {
	result := runControlRuntimeGateFailureTest(t, controlRuntimeFailureFixture{
		optionsBody: `{"controlModVersion":"0.2.2"}`,
	})
	if result.instance.State != storage.InstanceStateError || result.instance.DriverPhase != ControlRuntimeCodeVersionMismatch {
		t.Fatalf("explicit mismatch state = %+v, want error/%s", result.instance, ControlRuntimeCodeVersionMismatch)
	}
	if result.downCalls != 1 {
		t.Fatalf("explicit mismatch ComposeDown calls = %d, want 1", result.downCalls)
	}
	if !strings.Contains(result.job.ErrorMessage.String, "actual=0.2.2") {
		t.Fatalf("explicit version was not preserved in job error: %+v", result.job)
	}
}

func TestControlRuntimeHostFarmhousePatchFailureStopsServer(t *testing.T) {
	result := runControlRuntimeGateFailureTest(t, controlRuntimeFailureFixture{
		optionsBody: `{"controlModVersion":"{{expected}}","hostFarmhousePreservationPatchAvailable":false}`,
	})
	if result.instance.State != storage.InstanceStateError || result.instance.DriverPhase != ControlRuntimeCodeHostFarmhousePatchUnavailable {
		t.Fatalf("host farmhouse patch failure state = %+v, want error/%s", result.instance, ControlRuntimeCodeHostFarmhousePatchUnavailable)
	}
	if result.downCalls != 1 {
		t.Fatalf("host farmhouse patch failure ComposeDown calls = %d, want 1", result.downCalls)
	}
	if !strings.Contains(result.job.ErrorMessage.String, ControlRuntimeCodeHostFarmhousePatchUnavailable) {
		t.Fatalf("host farmhouse patch failure code missing from job: %+v", result.job)
	}
}

func TestControlRuntimeContextCancellationDoesNotCleanup(t *testing.T) {
	result := runControlRuntimeGateFailureTest(t, controlRuntimeFailureFixture{
		runtimeTimeout: time.Second,
		jobTimeout:     30 * time.Millisecond,
	})
	if result.downCalls != 0 {
		t.Fatalf("context cancellation must not stop Compose, calls=%d", result.downCalls)
	}
	if result.instance.State != storage.InstanceStateStarting || result.instance.DriverPhase != "control_runtime_starting" {
		t.Fatalf("context cancellation should preserve in-flight state, got %+v", result.instance)
	}
}

func TestControlRuntimeCleanupFailureIsExplicit(t *testing.T) {
	result := runControlRuntimeGateFailureTest(t, controlRuntimeFailureFixture{
		optionsBody: `{"controlModVersion":"0.2.2"}`,
		downErr:     errors.New("docker daemon unavailable"),
	})
	if result.downCalls != 1 {
		t.Fatalf("cleanup failure ComposeDown calls = %d, want 1", result.downCalls)
	}
	if result.instance.State != storage.InstanceStateError || result.instance.DriverPhase != "control_runtime_cleanup_failed" {
		t.Fatalf("cleanup failure state = %+v, want error/control_runtime_cleanup_failed", result.instance)
	}
	if !strings.Contains(result.job.ErrorMessage.String, "docker daemon unavailable") {
		t.Fatalf("cleanup failure detail missing from job: %+v", result.job)
	}
}

func TestStartSnapshotCleanupFailureNeverCallsComposeUp(t *testing.T) {
	store := newLifecycleTestStore(t)
	dataDir := filepath.Join(t.TempDir(), "stardew")
	instance := prepareControlLifecycleInstance(t, store, dataDir)
	blockedPath := filepath.Join(controlDir(dataDir), "options.json")
	if err := os.MkdirAll(filepath.Join(blockedPath, "child"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(blockedPath, "child", "keep"), []byte("non-empty"), 0o600); err != nil {
		t.Fatal(err)
	}

	var composeUps atomic.Int32
	fake := &fakeConsoleDocker{composeUpFunc: func(context.Context, string) (paneldocker.CommandResult, error) {
		composeUps.Add(1)
		return paneldocker.CommandResult{ExitCode: 0}, nil
	}}
	manager := jobs.NewManager(store, slog.Default())
	driver := New(fake, slog.Default(), manager, store)
	runner := &lifecycleRunner{driver: driver, lifecycle: fake, instance: instance, operation: "start"}
	job, err := manager.Start(context.Background(), jobs.Spec{
		Type: lifecycleJobType, TargetType: "instance", TargetID: instance.ID,
		Timeout: 5 * time.Second, Run: runner.run,
	})
	if err != nil {
		t.Fatal(err)
	}
	failed := waitForDriverTestJobStatus(t, store, job.ID, storage.JobStatusFailed)
	if got := composeUps.Load(); got != 0 {
		t.Fatalf("ComposeUp calls = %d, want zero after snapshot cleanup failure", got)
	}
	updated, err := store.GetInstance(context.Background(), instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.State != storage.InstanceStateStopped || updated.DriverPhase != "control_runtime_snapshot_cleanup_failed" {
		t.Fatalf("snapshot cleanup failure state = %+v", updated)
	}
	if !strings.Contains(failed.ErrorMessage.String, "clear stale Control runtime snapshots") {
		t.Fatalf("snapshot cleanup failure missing from job: %+v", failed)
	}
}

type controlRuntimeFailureFixture struct {
	optionsBody    string
	runtimeTimeout time.Duration
	jobTimeout     time.Duration
	downErr        error
}

type controlRuntimeFailureResult struct {
	job       storage.Job
	instance  storage.Instance
	downCalls int
}

func runControlRuntimeGateFailureTest(t *testing.T, fixture controlRuntimeFailureFixture) controlRuntimeFailureResult {
	t.Helper()
	dataDir, expected := setupControlRuntimeGateTest(t)
	if fixture.optionsBody != "" {
		writeControlRuntimeOptions(t, dataDir, strings.ReplaceAll(fixture.optionsBody, "{{expected}}", expected))
	}
	store := newLifecycleTestStore(t)
	instance, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{
		ID: storage.DefaultInstanceID, DriverID: DriverID, Name: "Stardew", DataDir: dataDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	instance, err = store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID: instance.ID, State: storage.InstanceStateStopped, DriverPhase: "stopped", DriverPayload: "{}",
	})
	if err != nil {
		t.Fatal(err)
	}
	downCalls := 0
	fake := &fakeConsoleDocker{composeDownFunc: func(context.Context, string) (paneldocker.CommandResult, error) {
		downCalls++
		return paneldocker.CommandResult{Stderr: errorText(fixture.downErr), ExitCode: 1}, fixture.downErr
	}}
	manager := jobs.NewManager(store, slog.Default())
	driver := New(fake, slog.Default(), manager, store)
	driver.runtimeUpdateServerTimeout = fixture.runtimeTimeout
	if driver.runtimeUpdateServerTimeout <= 0 {
		driver.runtimeUpdateServerTimeout = time.Second
	}
	runner := &lifecycleRunner{driver: driver, lifecycle: fake, instance: instance, operation: "start"}
	jobTimeout := fixture.jobTimeout
	if jobTimeout <= 0 {
		jobTimeout = 2 * time.Second
	}
	job, err := manager.Start(context.Background(), jobs.Spec{
		Type: lifecycleJobType, TargetType: "instance", TargetID: instance.ID, Timeout: jobTimeout,
		Run: func(ctx context.Context, jobCtx *jobs.Context) error {
			_, gateErr := runner.waitForControlRuntime(ctx, jobCtx)
			return gateErr
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	failed := waitForDriverTestJobStatus(t, store, job.ID, storage.JobStatusFailed)
	updated, err := store.GetInstance(context.Background(), instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	return controlRuntimeFailureResult{job: failed, instance: updated, downCalls: downCalls}
}

func prepareControlLifecycleInstance(t *testing.T, store *storage.Store, dataDir string) storage.Instance {
	t.Helper()
	instance, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{
		ID: storage.DefaultInstanceID, DriverID: DriverID, Name: "Stardew", DataDir: dataDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := New(nil, slog.Default(), nil, nil).Prepare(context.Background(), registry.Instance{
		ID: instance.ID, DriverID: DriverID, Name: instance.Name, DataDir: dataDir,
		State: storage.InstanceStateUninitialized,
	}); err != nil {
		t.Fatal(err)
	}
	junimoDir := junimoServerModDir(dataDir)
	if err := os.MkdirAll(junimoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := fmt.Sprintf(`{"Name":"JunimoServer","Version":%q,"UniqueID":"JunimoHost.Server"}`, TestedImageTag)
	if err := os.WriteFile(filepath.Join(junimoDir, junimoServerManifestName), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(junimoDir, junimoServerAssemblyName), []byte("test JunimoServer assembly"), 0o644); err != nil {
		t.Fatal(err)
	}
	instance, err = store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID: instance.ID, State: storage.InstanceStateStopped, DriverPhase: "stopped", DriverPayload: "{}",
	})
	if err != nil {
		t.Fatal(err)
	}
	return instance
}

func waitForControlLifecyclePhase(t *testing.T, store *storage.Store, instanceID, phase string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		instance, err := store.GetInstance(context.Background(), instanceID)
		if err != nil {
			t.Fatal(err)
		}
		if instance.DriverPhase == phase {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	instance, _ := store.GetInstance(context.Background(), instanceID)
	t.Fatalf("instance did not reach %s, got %+v", phase, instance)
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
