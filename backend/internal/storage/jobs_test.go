package storage

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
)

func TestJobsStorageLifecycle(t *testing.T) {
	store, closeStore := newStorageTestStore(t)
	defer closeStore()

	job, err := store.CreateJob(context.Background(), CreateJobParams{
		Type:        "test",
		DisplayName: "测试任务 · Farm Type Manager",
		TargetType:  "instance",
		TargetID:    DefaultInstanceID,
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	if job.Status != JobStatusQueued {
		t.Fatalf("expected queued, got %s", job.Status)
	}
	if !job.DisplayName.Valid || job.DisplayName.String != "测试任务 · Farm Type Manager" {
		t.Fatalf("display name = %#v, want saved display name", job.DisplayName)
	}

	if _, err := store.StartJob(context.Background(), job.ID); err != nil {
		t.Fatalf("start job: %v", err)
	}
	started, err := store.GetJob(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("get started job: %v", err)
	}
	if !started.DisplayName.Valid || started.DisplayName.String != "测试任务 · Farm Type Manager" {
		t.Fatalf("started display name = %#v, want saved display name", started.DisplayName)
	}
	firstLog, err := store.AppendJobLog(context.Background(), job.ID, JobLogLevelInfo, "first")
	if err != nil {
		t.Fatalf("append first log: %v", err)
	}
	secondLog, err := store.AppendJobLog(context.Background(), job.ID, JobLogLevelWarn, "second")
	if err != nil {
		t.Fatalf("append second log: %v", err)
	}
	if firstLog.Sequence != 1 || secondLog.Sequence != 2 {
		t.Fatalf("unexpected sequences: %d, %d", firstLog.Sequence, secondLog.Sequence)
	}

	logs, err := store.ListJobLogs(context.Background(), job.ID, 1, 10)
	if err != nil {
		t.Fatalf("list logs: %v", err)
	}
	if len(logs) != 1 || logs[0].Message != "second" {
		t.Fatalf("unexpected logs: %#v", logs)
	}

	finished, err := store.FinishJob(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("finish job: %v", err)
	}
	if finished.Status != JobStatusSucceeded || !finished.FinishedAt.Valid {
		t.Fatalf("job was not finished: %#v", finished)
	}
}

func TestListLatestJobLogsReturnsTailInAscendingOrder(t *testing.T) {
	store, closeStore := newStorageTestStore(t)
	defer closeStore()

	job, err := store.CreateJob(context.Background(), CreateJobParams{
		Type:       "test",
		TargetType: "instance",
		TargetID:   DefaultInstanceID,
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	for index := 1; index <= 5; index++ {
		if _, err := store.AppendJobLog(context.Background(), job.ID, JobLogLevelInfo, fmt.Sprintf("line-%d", index)); err != nil {
			t.Fatalf("append log %d: %v", index, err)
		}
	}

	logs, hasEarlier, err := store.ListLatestJobLogs(context.Background(), job.ID, 3)
	if err != nil {
		t.Fatalf("list latest logs: %v", err)
	}
	if !hasEarlier {
		t.Fatal("expected earlier logs to be reported")
	}
	if len(logs) != 3 || logs[0].Sequence != 3 || logs[1].Sequence != 4 || logs[2].Sequence != 5 {
		t.Fatalf("latest logs = %#v, want sequences 3, 4, 5", logs)
	}

	allLogs, hasEarlier, err := store.ListLatestJobLogs(context.Background(), job.ID, 5)
	if err != nil {
		t.Fatalf("list exact latest logs: %v", err)
	}
	if hasEarlier {
		t.Fatal("did not expect earlier logs when the limit equals the total")
	}
	if len(allLogs) != 5 || allLogs[0].Sequence != 1 || allLogs[4].Sequence != 5 {
		t.Fatalf("all latest logs = %#v, want sequences 1 through 5", allLogs)
	}
}

func TestFailInterruptedJobs(t *testing.T) {
	store, closeStore := newStorageTestStore(t)
	defer closeStore()

	job, err := store.CreateJob(context.Background(), CreateJobParams{Type: "test", TargetType: "instance", TargetID: DefaultInstanceID})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	if _, err := store.StartJob(context.Background(), job.ID); err != nil {
		t.Fatalf("start job: %v", err)
	}
	count, err := store.FailInterruptedJobs(context.Background(), "restarted")
	if err != nil {
		t.Fatalf("fail interrupted: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 interrupted job, got %d", count)
	}
	failed, err := store.GetJob(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("get failed job: %v", err)
	}
	if failed.Status != JobStatusFailed || failed.ErrorMessage.String != "restarted" {
		t.Fatalf("unexpected failed job: %#v", failed)
	}
}

func TestJobPayloadPersists(t *testing.T) {
	store, closeStore := newStorageTestStore(t)
	defer closeStore()
	job, err := store.CreateJob(context.Background(), CreateJobParams{
		Type: "stardew_lifecycle", TargetType: "instance", TargetID: DefaultInstanceID,
		Payload: `{"farmType":"standard","farmName":"Farm"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := store.GetJob(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.Payload.Valid || loaded.Payload.String != `{"farmType":"standard","farmName":"Farm"}` {
		t.Fatalf("payload = %#v", loaded.Payload)
	}
}

func TestCreateExclusiveJobReturnsExistingActiveJob(t *testing.T) {
	store, closeStore := newStorageTestStore(t)
	defer closeStore()

	params := CreateJobParams{Type: "stardew_install", TargetType: "instance", TargetID: DefaultInstanceID}
	first, err := store.CreateExclusiveJob(context.Background(), params)
	if err != nil {
		t.Fatalf("create first exclusive job: %v", err)
	}
	_, err = store.CreateExclusiveJob(context.Background(), params)
	var active *ActiveJobExistsError
	if !errors.As(err, &active) {
		t.Fatalf("second exclusive job error = %v, want ActiveJobExistsError", err)
	}
	if active.Job.ID != first.ID {
		t.Fatalf("active job id = %s, want %s", active.Job.ID, first.ID)
	}
	if !errors.Is(err, ErrActiveJobsExist) {
		t.Fatalf("error %v does not unwrap to ErrActiveJobsExist", err)
	}

	if _, err := store.FinishJob(context.Background(), first.ID); err != nil {
		t.Fatalf("finish first exclusive job: %v", err)
	}
	if _, err := store.CreateExclusiveJob(context.Background(), params); err != nil {
		t.Fatalf("create exclusive job after terminal state: %v", err)
	}
}

func TestCreateExclusiveJobConcurrentRequestsHaveOneWinner(t *testing.T) {
	store, closeStore := newStorageTestStore(t)
	defer closeStore()

	const callers = 12
	start := make(chan struct{})
	results := make(chan error, callers)
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := store.CreateExclusiveJob(context.Background(), CreateJobParams{
				Type: "stardew_install", TargetType: "instance", TargetID: DefaultInstanceID,
			})
			results <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	winners := 0
	conflicts := 0
	for err := range results {
		switch {
		case err == nil:
			winners++
		case errors.Is(err, ErrActiveJobsExist):
			conflicts++
		default:
			t.Fatalf("unexpected exclusive create error: %v", err)
		}
	}
	if winners != 1 || conflicts != callers-1 {
		t.Fatalf("winners=%d conflicts=%d, want 1/%d", winners, conflicts, callers-1)
	}
}

func TestCreateIdempotentJobReturnsOriginalTerminalJob(t *testing.T) {
	store, closeStore := newStorageTestStore(t)
	defer closeStore()

	params := CreateJobParams{
		Type: "mod_remote_install", TargetType: "instance", TargetID: DefaultInstanceID,
		IdempotencyKey: "nexus-request-1",
	}
	first, err := store.CreateIdempotentJob(context.Background(), params)
	if err != nil {
		t.Fatalf("create first idempotent job: %v", err)
	}
	if _, err := store.FinishJob(context.Background(), first.ID); err != nil {
		t.Fatalf("finish first idempotent job: %v", err)
	}

	_, err = store.CreateIdempotentJob(context.Background(), params)
	var existing *IdempotentJobExistsError
	if !errors.As(err, &existing) {
		t.Fatalf("repeat idempotent job error = %v, want IdempotentJobExistsError", err)
	}
	if existing.Job.ID != first.ID || existing.Job.Status != JobStatusSucceeded {
		t.Fatalf("existing job = %#v, want succeeded job %s", existing.Job, first.ID)
	}
	if !errors.Is(err, ErrIdempotentJobExists) {
		t.Fatalf("error %v does not unwrap to ErrIdempotentJobExists", err)
	}

	second, err := store.CreateIdempotentJob(context.Background(), CreateJobParams{
		Type: "mod_remote_install", TargetType: "instance", TargetID: DefaultInstanceID,
		IdempotencyKey: "nexus-request-2",
	})
	if err != nil {
		t.Fatalf("create different idempotency key: %v", err)
	}
	if second.ID == first.ID {
		t.Fatal("different idempotency keys must create different jobs")
	}
}

func TestCreateIdempotentJobConcurrentRequestsHaveOneWinner(t *testing.T) {
	store, closeStore := newStorageTestStore(t)
	defer closeStore()

	const callers = 12
	type result struct {
		job Job
		err error
	}
	start := make(chan struct{})
	results := make(chan result, callers)
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			job, err := store.CreateIdempotentJob(context.Background(), CreateJobParams{
				Type: "mod_remote_install", TargetType: "instance", TargetID: DefaultInstanceID,
				IdempotencyKey: "nexus-concurrent-request",
			})
			results <- result{job: job, err: err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	winners := 0
	conflicts := 0
	observedIDs := map[string]struct{}{}
	for got := range results {
		if got.err == nil {
			winners++
			observedIDs[got.job.ID] = struct{}{}
			continue
		}
		var existing *IdempotentJobExistsError
		if !errors.As(got.err, &existing) {
			t.Fatalf("unexpected idempotent create error: %v", got.err)
		}
		conflicts++
		observedIDs[existing.Job.ID] = struct{}{}
	}
	if winners != 1 || conflicts != callers-1 {
		t.Fatalf("winners=%d conflicts=%d, want 1/%d", winners, conflicts, callers-1)
	}
	if len(observedIDs) != 1 {
		t.Fatalf("observed job ids = %v, want one durable owner", observedIDs)
	}
}

func TestExclusiveInstallMigrationRetiresLegacyDuplicateOwners(t *testing.T) {
	dataDir := t.TempDir()
	store, err := Open(context.Background(), config.Config{
		DataDir: dataDir,
		DBPath:  filepath.Join(dataDir, "panel.db"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.db.ExecContext(context.Background(), `
		CREATE TABLE schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
		)
	`); err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{
		"001_foundation.sql", "002_auth.sql", "003_jobs_state.sql", "004_instances.sql",
		"005_super_admin.sql", "006_restart_schedules.sql", "007_job_display_name.sql",
		"008_player_roster.sql", "009_control_commands.sql", "010_job_payload.sql",
		"011_player_roster_character_delete.sql",
	} {
		if err := store.applyMigration(context.Background(), version); err != nil {
			t.Fatalf("apply legacy migration %s: %v", version, err)
		}
	}
	for i := 0; i < 2; i++ {
		if _, err := store.CreateJob(context.Background(), CreateJobParams{
			Type: "stardew_install", TargetType: "instance", TargetID: DefaultInstanceID,
		}); err != nil {
			t.Fatalf("seed duplicate active install %d: %v", i, err)
		}
	}
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("apply exclusive install migration: %v", err)
	}
	active, err := store.ListActiveJobs(context.Background(), ListActiveJobsFilter{
		TargetType: "instance", TargetID: DefaultInstanceID, Types: []string{"stardew_install"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 1 {
		t.Fatalf("active installs after migration = %d, want 1", len(active))
	}
	if _, err := store.CreateJob(context.Background(), CreateJobParams{
		Type: "stardew_install", TargetType: "instance", TargetID: DefaultInstanceID,
	}); err == nil {
		t.Fatal("partial unique index should reject another active install")
	}
}

func newStorageTestStore(t *testing.T) (*Store, func()) {
	t.Helper()
	dataDir := t.TempDir()
	store, err := Open(context.Background(), config.Config{
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
	return store, func() {
		if err := store.Close(); err != nil {
			t.Fatalf("close store: %v", err)
		}
	}
}
