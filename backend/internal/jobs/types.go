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

type Spec struct {
	Type        string
	DisplayName string
	TargetType  string
	TargetID    string
	CreatedBy   int64
	Payload     string
	Timeout     time.Duration
	Run         Runner
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
