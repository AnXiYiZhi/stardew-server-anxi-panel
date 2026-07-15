package storage

import (
	"context"
	"path/filepath"
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
