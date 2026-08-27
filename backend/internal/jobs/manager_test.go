package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func TestManagerMarksSuccessfulAndFailedJobs(t *testing.T) {
	manager, store, closeStore := newJobsTestManager(t)
	defer closeStore()

	succeeded, err := manager.Start(context.Background(), Spec{
		Type:       "test",
		TargetType: "instance",
		TargetID:   storage.DefaultInstanceID,
		Timeout:    3 * time.Second,
		Run: func(ctx context.Context, job *Context) error {
			_, err := job.Info(ctx, "ok")
			return err
		},
	})
	if err != nil {
		t.Fatalf("start success job: %v", err)
	}
	waitForJobStatus(t, store, succeeded.ID, storage.JobStatusSucceeded)

	failed, err := manager.Start(context.Background(), Spec{
		Type:       "test_fail",
		TargetType: "instance",
		TargetID:   storage.DefaultInstanceID,
		Timeout:    3 * time.Second,
		Run: func(ctx context.Context, job *Context) error {
			return errors.New("boom")
		},
	})
	if err != nil {
		t.Fatalf("start failed job: %v", err)
	}
	finished := waitForJobStatus(t, store, failed.ID, storage.JobStatusFailed)
	if finished.ErrorMessage.String != "boom" {
		t.Fatalf("expected boom error, got %#v", finished.ErrorMessage)
	}
}

func TestManagerDoesNotPublishFailureBeforeRunnerDefersComplete(t *testing.T) {
	manager, store, closeStore := newJobsTestManager(t)
	defer closeStore()

	deferStarted := make(chan struct{})
	releaseDefer := make(chan struct{})
	deferFinished := make(chan struct{})
	job, err := manager.Start(context.Background(), Spec{
		Type: "deferred_failure", TargetType: "instance", TargetID: storage.DefaultInstanceID,
		Timeout: 3 * time.Second,
		Run: func(context.Context, *Context) error {
			defer func() {
				close(deferStarted)
				<-releaseDefer
				close(deferFinished)
			}()
			return errors.New("deferred boom")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	<-deferStarted
	duringDefer, err := store.GetJob(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if duringDefer.Status == storage.JobStatusFailed {
		t.Fatal("job became failed before the runner defer completed")
	}
	close(releaseDefer)
	failed := waitForJobStatus(t, store, job.ID, storage.JobStatusFailed)
	if failed.ErrorMessage.String != "deferred boom" {
		t.Fatalf("error=%q", failed.ErrorMessage.String)
	}
	select {
	case <-deferFinished:
	default:
		t.Fatal("job became failed before the runner defer completion signal")
	}
}

func TestManagerBeforeRunFailureNeverLaunchesRunner(t *testing.T) {
	manager, store, closeStore := newJobsTestManager(t)
	defer closeStore()

	runnerCalls := 0
	preparedJobID := ""
	job, err := manager.Start(context.Background(), Spec{
		Type: "save-import-test", TargetType: "instance", TargetID: storage.DefaultInstanceID,
		IdempotencyKey: "operation-1",
		BeforeRun: func(_ context.Context, durable storage.Job) error {
			preparedJobID = durable.ID
			return errors.New("injected ownership persistence failure")
		},
		Run: func(context.Context, *Context) error {
			runnerCalls++
			return nil
		},
	})
	var preparation *StartPreparationError
	if !errors.As(err, &preparation) || job.ID == "" || preparedJobID != job.ID {
		t.Fatalf("job=%+v prepared=%q err=%v", job, preparedJobID, err)
	}
	if runnerCalls != 0 {
		t.Fatalf("runner calls=%d", runnerCalls)
	}
	stored, err := store.GetJob(context.Background(), job.ID)
	if err != nil || stored.Status != storage.JobStatusFailed {
		t.Fatalf("stored job=%+v err=%v", stored, err)
	}
}

func TestManagerRecoversPanicAsFailed(t *testing.T) {
	manager, store, closeStore := newJobsTestManager(t)
	defer closeStore()
	const secretPanic = "STEAM_PASSWORD=do-not-persist platform=76561198000000001"

	job, err := manager.Start(context.Background(), Spec{
		Type:       "panic",
		TargetType: "instance",
		TargetID:   storage.DefaultInstanceID,
		Timeout:    3 * time.Second,
		Run: func(ctx context.Context, job *Context) error {
			panic(secretPanic)
		},
	})
	if err != nil {
		t.Fatalf("start panic job: %v", err)
	}
	failed := waitForJobStatus(t, store, job.ID, storage.JobStatusFailed)
	if failed.ErrorMessage.String != "任务执行异常。" || strings.Contains(failed.ErrorMessage.String, secretPanic) {
		t.Fatalf("panic job error=%q", failed.ErrorMessage.String)
	}
	logs, err := store.ListJobLogs(context.Background(), job.ID, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range logs {
		if strings.Contains(line.Message, secretPanic) {
			t.Fatalf("panic secret persisted in job log: %q", line.Message)
		}
	}
}

func TestManagerCancelMarksRunningJobCanceled(t *testing.T) {
	manager, store, closeStore := newJobsTestManager(t)
	defer closeStore()

	started := make(chan struct{})
	job, err := manager.Start(context.Background(), Spec{
		Type:       "cancel_me",
		TargetType: "instance",
		TargetID:   storage.DefaultInstanceID,
		Timeout:    3 * time.Second,
		Run: func(ctx context.Context, job *Context) error {
			close(started)
			<-ctx.Done()
			return ctx.Err()
		},
	})
	if err != nil {
		t.Fatalf("start cancel job: %v", err)
	}
	<-started

	if err := manager.Cancel(context.Background(), job.ID); err != nil {
		t.Fatalf("cancel job: %v", err)
	}
	canceled := waitForJobStatus(t, store, job.ID, storage.JobStatusCanceled)
	if canceled.ErrorMessage.String == "" {
		t.Fatal("canceled job should record a message")
	}
}

func TestManagerCancelActiveFiltersTarget(t *testing.T) {
	manager, store, closeStore := newJobsTestManager(t)
	defer closeStore()

	block := func(started chan struct{}) Runner {
		return func(ctx context.Context, job *Context) error {
			close(started)
			<-ctx.Done()
			return ctx.Err()
		}
	}
	firstStarted := make(chan struct{})
	first, err := manager.Start(context.Background(), Spec{
		Type:       "stardew_lifecycle",
		TargetType: "instance",
		TargetID:   "stardew",
		Timeout:    3 * time.Second,
		Run:        block(firstStarted),
	})
	if err != nil {
		t.Fatalf("start first job: %v", err)
	}
	secondStarted := make(chan struct{})
	second, err := manager.Start(context.Background(), Spec{
		Type:       "stardew_lifecycle",
		TargetType: "instance",
		TargetID:   "other",
		Timeout:    3 * time.Second,
		Run:        block(secondStarted),
	})
	if err != nil {
		t.Fatalf("start second job: %v", err)
	}
	<-firstStarted
	<-secondStarted

	canceled, err := manager.CancelActive(context.Background(), storage.ListActiveJobsFilter{
		TargetType: "instance",
		TargetID:   "stardew",
		Types:      []string{"stardew_lifecycle"},
	}, "")
	if err != nil {
		t.Fatalf("cancel active: %v", err)
	}
	if len(canceled) != 1 || canceled[0].ID != first.ID {
		t.Fatalf("unexpected canceled jobs: %#v", canceled)
	}
	waitForJobStatus(t, store, first.ID, storage.JobStatusCanceled)

	stillRunning, err := store.GetJob(context.Background(), second.ID)
	if err != nil {
		t.Fatalf("get second job: %v", err)
	}
	if stillRunning.Status != storage.JobStatusRunning {
		t.Fatalf("second job status = %s, want running", stillRunning.Status)
	}
	if err := manager.Cancel(context.Background(), second.ID); err != nil {
		t.Fatalf("cleanup second job: %v", err)
	}
	waitForJobStatus(t, store, second.ID, storage.JobStatusCanceled)
}

func TestManagerExclusiveJobReturnsCurrentOwner(t *testing.T) {
	manager, store, closeStore := newJobsTestManager(t)
	defer closeStore()

	started := make(chan struct{})
	first, err := manager.Start(context.Background(), Spec{
		Type: "stardew_install", TargetType: "instance", TargetID: storage.DefaultInstanceID,
		Exclusive: true, Timeout: 3 * time.Second,
		Run: func(ctx context.Context, _ *Context) error {
			close(started)
			<-ctx.Done()
			return ctx.Err()
		},
	})
	if err != nil {
		t.Fatalf("start first exclusive job: %v", err)
	}
	<-started

	_, err = manager.Start(context.Background(), Spec{
		Type: "stardew_install", TargetType: "instance", TargetID: storage.DefaultInstanceID,
		Exclusive: true, Run: func(context.Context, *Context) error { return nil },
	})
	var active *storage.ActiveJobExistsError
	if !errors.As(err, &active) || active.Job.ID != first.ID {
		t.Fatalf("second start error = %v, active=%#v, want owner %s", err, active, first.ID)
	}

	if err := manager.Cancel(context.Background(), first.ID); err != nil {
		t.Fatalf("cancel first job: %v", err)
	}
	waitForJobStatus(t, store, first.ID, storage.JobStatusCanceled)
}

func TestManagerIdempotentJobStartsOneRunner(t *testing.T) {
	manager, store, closeStore := newJobsTestManager(t)
	defer closeStore()

	started := make(chan struct{})
	release := make(chan struct{})
	first, err := manager.Start(context.Background(), Spec{
		Type: "mod_remote_install", TargetType: "instance", TargetID: storage.DefaultInstanceID,
		IdempotencyKey: "nexus-manager-request", Timeout: 3 * time.Second,
		Run: func(context.Context, *Context) error {
			close(started)
			<-release
			return nil
		},
	})
	if err != nil {
		t.Fatalf("start first idempotent job: %v", err)
	}
	<-started

	_, err = manager.Start(context.Background(), Spec{
		Type: "mod_remote_install", TargetType: "instance", TargetID: storage.DefaultInstanceID,
		IdempotencyKey: "nexus-manager-request",
		Run:            func(context.Context, *Context) error { return errors.New("duplicate runner started") },
	})
	var existing *storage.IdempotentJobExistsError
	if !errors.As(err, &existing) || existing.Job.ID != first.ID {
		t.Fatalf("repeat start error = %v, existing=%#v, want job %s", err, existing, first.ID)
	}

	close(release)
	waitForJobStatus(t, store, first.ID, storage.JobStatusSucceeded)
}

func TestManagerRecoverInterruptedInstallMarksInstanceError(t *testing.T) {
	manager, store, closeStore := newJobsTestManager(t)
	defer closeStore()

	dataDir := filepath.Join(t.TempDir(), "instances", storage.DefaultInstanceID)
	if _, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{
		ID:       storage.DefaultInstanceID,
		DriverID: storage.DefaultDriverID,
		Name:     "Stardew Valley",
		DataDir:  dataDir,
	}); err != nil {
		t.Fatalf("ensure instance: %v", err)
	}
	if _, err := store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID:           storage.DefaultInstanceID,
		State:        storage.InstanceStateSteamAuthRunning,
		StateMessage: "running steam auth",
		DriverPhase:  "steam_auth_running",
	}); err != nil {
		t.Fatalf("set running install state: %v", err)
	}

	job, err := store.CreateJob(context.Background(), storage.CreateJobParams{
		Type:       "stardew_install",
		TargetType: "instance",
		TargetID:   storage.DefaultInstanceID,
	})
	if err != nil {
		t.Fatalf("create install job: %v", err)
	}
	if _, err := store.StartJob(context.Background(), job.ID); err != nil {
		t.Fatalf("start install job: %v", err)
	}

	if err := manager.RecoverInterruptedJobs(context.Background()); err != nil {
		t.Fatalf("recover interrupted jobs: %v", err)
	}

	failed, err := store.GetJob(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("get failed job: %v", err)
	}
	if failed.Status != storage.JobStatusFailed {
		t.Fatalf("job status = %s, want failed", failed.Status)
	}
	instance, err := store.GetInstance(context.Background(), storage.DefaultInstanceID)
	if err != nil {
		t.Fatalf("get instance: %v", err)
	}
	if instance.State != storage.InstanceStateError || instance.DriverPhase != "install_interrupted" {
		t.Fatalf("instance state not marked interrupted: state=%s phase=%s", instance.State, instance.DriverPhase)
	}
}

func TestManagerRecoverInterruptedSteamAuthPreservesCompletedSessionAfterCrash(t *testing.T) {
	manager, store, closeStore := newJobsTestManager(t)
	defer closeStore()

	dataDir := filepath.Join(t.TempDir(), "instances", storage.DefaultInstanceID)
	instance, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{
		ID:       storage.DefaultInstanceID,
		DriverID: storage.DefaultDriverID,
		Name:     "Stardew Valley",
		DataDir:  dataDir,
	})
	if err != nil {
		t.Fatalf("ensure instance: %v", err)
	}
	instance.State = storage.InstanceStateReadyToStart
	instance.StateMessage.String = ""
	instance.StateMessage.Valid = false
	instance.DriverPhase = "control_ready"
	instance.DriverPayload = `{"install":"complete","invite_code":"legacy"}`
	original, err := store.RestoreInstanceStateSnapshot(context.Background(), instance)
	if err != nil {
		t.Fatalf("set original base state: %v", err)
	}
	if err := sjconfig.UpdateEnvFile(filepath.Join(dataDir, ".env"), map[string]string{
		"STEAMCMD_AUTH_COMPLETED": "true",
		"STEAM_AUTH_COMPLETED":    "true",
		"STEAM_INVITE_ENABLED":    "true",
		"STEAM_INVITE_AUTH_STATE": sjconfig.SteamInviteAuthStateAuthorizing,
	}); err != nil {
		t.Fatalf("prepare authorization state: %v", err)
	}
	if _, err := store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID:            storage.DefaultInstanceID,
		State:         storage.InstanceStateSteamAuthRunning,
		StateMessage:  "authorizing optional Steam invite",
		DriverPhase:   "steam_invite_authorizing",
		DriverPayload: original.DriverPayload,
	}); err != nil {
		t.Fatalf("set temporary authorization state: %v", err)
	}
	payloadBytes, err := json.Marshal(stardewSteamAuthRecoveryPayload{
		SchemaVersion:     stardewSteamAuthPayloadSchemaVersion,
		State:             original.State,
		StateMessage:      original.StateMessage.String,
		StateMessageValid: original.StateMessage.Valid,
		DriverPhase:       original.DriverPhase,
		DriverPayload:     original.DriverPayload,
	})
	if err != nil {
		t.Fatalf("encode recovery payload: %v", err)
	}
	job, err := store.CreateJob(context.Background(), storage.CreateJobParams{
		Type:       stardewSteamAuthJobType,
		TargetType: "instance",
		TargetID:   storage.DefaultInstanceID,
		Payload:    string(payloadBytes),
	})
	if err != nil {
		t.Fatalf("create Steam authorization job: %v", err)
	}
	if _, err := store.StartJob(context.Background(), job.ID); err != nil {
		t.Fatalf("start Steam authorization job: %v", err)
	}

	if err := manager.RecoverInterruptedJobs(context.Background()); err != nil {
		t.Fatalf("recover interrupted jobs: %v", err)
	}

	failed, err := store.GetJob(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("get recovered job: %v", err)
	}
	if failed.Status != storage.JobStatusFailed {
		t.Fatalf("job status = %s, want failed", failed.Status)
	}
	restored, err := store.GetInstance(context.Background(), storage.DefaultInstanceID)
	if err != nil {
		t.Fatalf("get restored instance: %v", err)
	}
	if restored.State != original.State || restored.StateMessage != original.StateMessage ||
		restored.DriverPhase != original.DriverPhase || restored.DriverPayload != original.DriverPayload {
		t.Fatalf("restored base state = %#v, want lifecycle fields from %#v", restored, original)
	}
	if !sjconfig.SteamInviteEnabled(dataDir) || sjconfig.SteamInviteAuthState(dataDir) != sjconfig.SteamInviteAuthStateReady {
		t.Fatalf("invite capability = enabled:%t auth:%q, want enabled ready", sjconfig.SteamInviteEnabled(dataDir), sjconfig.SteamInviteAuthState(dataDir))
	}
	fields, err := sjconfig.ReadEnvFile(filepath.Join(dataDir, ".env"))
	if err != nil {
		t.Fatalf("read recovered environment: %v", err)
	}
	if fields["STEAMCMD_AUTH_COMPLETED"] != "true" {
		t.Fatalf("SteamCMD cache flag changed to %q", fields["STEAMCMD_AUTH_COMPLETED"])
	}
	if fields["STEAM_AUTH_COMPLETED"] != "true" {
		t.Fatalf("steam-auth session flag = %q, want preserved", fields["STEAM_AUTH_COMPLETED"])
	}
	if fields["STEAM_INVITE_AUTH_STATE"] != sjconfig.SteamInviteAuthStateReady {
		t.Fatalf("steam invite auth state = %q, want ready", fields["STEAM_INVITE_AUTH_STATE"])
	}
	if err := sjconfig.SetSteamInviteAuthState(dataDir, sjconfig.SteamInviteAuthStateCleanupPending); err != nil {
		t.Fatal(err)
	}
	manager.recoverInterruptedSteamAuthInstance(context.Background(), job)
	if got := sjconfig.SteamInviteAuthState(dataDir); got != sjconfig.SteamInviteAuthStateCleanupPending {
		t.Fatalf("restart recovery erased successful holder cleanup state: got %q", got)
	}
}

func TestManagerRecoverInterruptedSteamAuthMalformedPayloadLeavesStableBaseState(t *testing.T) {
	manager, store, closeStore := newJobsTestManager(t)
	defer closeStore()

	dataDir := filepath.Join(t.TempDir(), "instances", storage.DefaultInstanceID)
	if _, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{
		ID:       storage.DefaultInstanceID,
		DriverID: storage.DefaultDriverID,
		Name:     "Stardew Valley",
		DataDir:  dataDir,
	}); err != nil {
		t.Fatalf("ensure instance: %v", err)
	}
	if err := sjconfig.SetSteamInviteEnabled(dataDir, true); err != nil {
		t.Fatalf("enable Steam invite: %v", err)
	}
	if _, err := store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID:            storage.DefaultInstanceID,
		State:         storage.InstanceStateSteamAuthRunning,
		StateMessage:  "authorizing optional Steam invite",
		DriverPhase:   "steam_invite_authorizing",
		DriverPayload: `{"install":"complete"}`,
	}); err != nil {
		t.Fatalf("set temporary authorization state: %v", err)
	}
	job, err := store.CreateJob(context.Background(), storage.CreateJobParams{
		Type:       stardewSteamAuthJobType,
		TargetType: "instance",
		TargetID:   storage.DefaultInstanceID,
		Payload:    `{}`,
	})
	if err != nil {
		t.Fatalf("create Steam authorization job: %v", err)
	}
	if _, err := store.StartJob(context.Background(), job.ID); err != nil {
		t.Fatalf("start Steam authorization job: %v", err)
	}

	if err := manager.RecoverInterruptedJobs(context.Background()); err != nil {
		t.Fatalf("recover interrupted jobs: %v", err)
	}
	restored, err := store.GetInstance(context.Background(), storage.DefaultInstanceID)
	if err != nil {
		t.Fatalf("get restored instance: %v", err)
	}
	if restored.State != storage.InstanceStateStopped || restored.DriverPhase != "stopped" {
		t.Fatalf("fallback state = %s/%s, want stopped/stopped", restored.State, restored.DriverPhase)
	}
	if restored.DriverPayload != `{"install":"complete"}` {
		t.Fatalf("fallback changed driver payload to %q", restored.DriverPayload)
	}
	if sjconfig.SteamInviteAuthState(dataDir) != sjconfig.SteamInviteAuthStateFailed {
		t.Fatalf("fallback invite auth state = %q, want failed", sjconfig.SteamInviteAuthState(dataDir))
	}
}

func newJobsTestManager(t *testing.T) (*Manager, *storage.Store, func()) {
	t.Helper()
	dataDir := t.TempDir()
	store, err := storage.Open(context.Background(), config.Config{
		DataDir: dataDir,
		DBPath:  filepath.Join(dataDir, "panel.db"),
	})
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	if err := store.Migrate(context.Background()); err != nil {
		_ = store.Close()
		t.Fatalf("migrate storage: %v", err)
	}
	return NewManager(store, slog.Default()), store, func() {
		if err := store.Close(); err != nil {
			t.Fatalf("close store: %v", err)
		}
	}
}

func waitForJobStatus(t *testing.T, store *storage.Store, jobID string, status string) storage.Job {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		job, err := store.GetJob(context.Background(), jobID)
		if err != nil {
			t.Fatalf("get job: %v", err)
		}
		if job.Status == status {
			return job
		}
		time.Sleep(20 * time.Millisecond)
	}
	job, _ := store.GetJob(context.Background(), jobID)
	t.Fatalf("job %s did not reach %s, got %s", jobID, status, job.Status)
	return storage.Job{}
}
