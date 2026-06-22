package jobs

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
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

func TestManagerRecoversPanicAsFailed(t *testing.T) {
	manager, store, closeStore := newJobsTestManager(t)
	defer closeStore()

	job, err := manager.Start(context.Background(), Spec{
		Type:       "panic",
		TargetType: "instance",
		TargetID:   storage.DefaultInstanceID,
		Timeout:    3 * time.Second,
		Run: func(ctx context.Context, job *Context) error {
			panic("bad runner")
		},
	})
	if err != nil {
		t.Fatalf("start panic job: %v", err)
	}
	failed := waitForJobStatus(t, store, job.ID, storage.JobStatusFailed)
	if failed.ErrorMessage.String == "" {
		t.Fatal("panic job should save error message")
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
