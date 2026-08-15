package jobs

import (
	"context"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

const (
	EventLog      = "log"
	EventJob      = "job"
	EventFinished = "finished"
)

type Runner func(ctx context.Context, job *Context) error

// BeforeRun is invoked after the job row has been durably created but before
// any runner goroutine is started. Callers use it to persist external ownership
// records which must exist before the job is allowed to mutate runtime state.
type BeforeRun func(ctx context.Context, job storage.Job) error

type Spec struct {
	Type           string
	DisplayName    string
	TargetType     string
	TargetID       string
	Exclusive      bool
	IdempotencyKey string
	CreatedBy      int64
	Payload        string
	Timeout        time.Duration
	BeforeRun      BeforeRun
	Run            Runner
}

type Context struct {
	ID      string
	manager *Manager
}

func (c *Context) Info(ctx context.Context, message string) (storage.JobLog, error) {
	return c.manager.AppendLog(ctx, c.ID, storage.JobLogLevelInfo, message)
}

func (c *Context) Warn(ctx context.Context, message string) (storage.JobLog, error) {
	return c.manager.AppendLog(ctx, c.ID, storage.JobLogLevelWarn, message)
}

func (c *Context) Error(ctx context.Context, message string) (storage.JobLog, error) {
	return c.manager.AppendLog(ctx, c.ID, storage.JobLogLevelError, message)
}

func (c *Context) Debug(ctx context.Context, message string) (storage.JobLog, error) {
	return c.manager.AppendLog(ctx, c.ID, storage.JobLogLevelDebug, message)
}

type Event struct {
	Type string
	Job  *storage.Job
	Log  *storage.JobLog
}
