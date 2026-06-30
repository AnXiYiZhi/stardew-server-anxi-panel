package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

const defaultJobTimeout = 30 * time.Minute

var ErrCancelNotImplemented = errors.New("job cancellation is not implemented")

type Manager struct {
	store  *storage.Store
	logger *slog.Logger

	mu          sync.Mutex
	cancels     map[string]context.CancelFunc
	subscribers map[string]map[chan Event]struct{}
}

func NewManager(store *storage.Store, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	return &Manager{
		store:       store,
		logger:      logger,
		cancels:     map[string]context.CancelFunc{},
		subscribers: map[string]map[chan Event]struct{}{},
	}
}

func (m *Manager) RecoverInterruptedJobs(ctx context.Context) error {
	interrupted, err := m.store.ListInterruptedJobs(ctx)
	if err != nil {
		return err
	}
	const message = "面板重启前任务未完成，已标记为失败。"
	count, err := m.store.FailInterruptedJobs(ctx, message)
	if err != nil {
		return err
	}
	for _, job := range interrupted {
		if _, err := m.store.AppendJobLog(ctx, job.ID, storage.JobLogLevelError, message); err != nil {
			m.logger.Error("failed to append interrupted job log", "job_id", job.ID, "error", err)
		}
	}
	if count > 0 {
		m.logger.Warn("marked interrupted jobs as failed", "count", count)
	}
	return nil
}

func (m *Manager) Start(ctx context.Context, spec Spec) (storage.Job, error) {
	if spec.Type == "" || spec.TargetType == "" || spec.TargetID == "" {
		return storage.Job{}, fmt.Errorf("job type and target are required")
	}
	if spec.Run == nil {
		return storage.Job{}, fmt.Errorf("job runner is required")
	}

	job, err := m.store.CreateJob(ctx, storage.CreateJobParams{
		Type:       spec.Type,
		TargetType: spec.TargetType,
		TargetID:   spec.TargetID,
		CreatedBy:  spec.CreatedBy,
	})
	if err != nil {
		return storage.Job{}, err
	}
	m.publish(Event{Type: EventJob, Job: &job})

	timeout := spec.Timeout
	if timeout <= 0 {
		timeout = defaultJobTimeout
	}
	runCtx, cancel := context.WithTimeout(context.Background(), timeout)
	m.mu.Lock()
	m.cancels[job.ID] = cancel
	m.mu.Unlock()

	go m.run(runCtx, cancel, job, spec.Run)
	return job, nil
}

func (m *Manager) Get(ctx context.Context, id string) (storage.Job, error) {
	return m.store.GetJob(ctx, id)
}

func (m *Manager) List(ctx context.Context, filter storage.ListJobsFilter) ([]storage.Job, error) {
	return m.store.ListJobs(ctx, filter)
}

func (m *Manager) Clear(ctx context.Context) (int64, error) {
	return m.store.ClearJobs(ctx)
}

func (m *Manager) ClearErrorLogs(ctx context.Context) (int64, int64, error) {
	return m.store.ClearJobErrorLogs(ctx)
}

func (m *Manager) Logs(ctx context.Context, jobID string, afterSequence int64, limit int) ([]storage.JobLog, error) {
	return m.store.ListJobLogs(ctx, jobID, afterSequence, limit)
}

func (m *Manager) AppendLog(ctx context.Context, jobID string, level string, message string) (storage.JobLog, error) {
	logLine, err := m.store.AppendJobLog(ctx, jobID, level, message)
	if err != nil {
		return storage.JobLog{}, err
	}
	m.publish(Event{Type: EventLog, Log: &logLine})
	return logLine, nil
}

func (m *Manager) Subscribe(jobID string) (<-chan Event, func()) {
	ch := make(chan Event, 32)
	m.mu.Lock()
	if m.subscribers[jobID] == nil {
		m.subscribers[jobID] = map[chan Event]struct{}{}
	}
	m.subscribers[jobID][ch] = struct{}{}
	m.mu.Unlock()

	unsubscribe := func() {
		m.mu.Lock()
		if subscribers, ok := m.subscribers[jobID]; ok {
			delete(subscribers, ch)
			if len(subscribers) == 0 {
				delete(m.subscribers, jobID)
			}
		}
		m.mu.Unlock()
		close(ch)
	}
	return ch, unsubscribe
}

func (m *Manager) Cancel(ctx context.Context, jobID string) error {
	return ErrCancelNotImplemented
}

func (m *Manager) run(ctx context.Context, cancel context.CancelFunc, job storage.Job, runner Runner) {
	defer cancel()
	defer func() {
		m.mu.Lock()
		delete(m.cancels, job.ID)
		m.mu.Unlock()
	}()

	started, err := m.store.StartJob(ctx, job.ID)
	if err != nil {
		m.logger.Error("failed to start job", "job_id", job.ID, "error", err)
		return
	}
	m.publish(Event{Type: EventJob, Job: &started})
	_, _ = m.AppendLog(ctx, job.ID, storage.JobLogLevelInfo, "任务已开始。")

	defer func() {
		if value := recover(); value != nil {
			message := fmt.Sprintf("任务执行异常：%v", value)
			_, _ = m.AppendLog(context.Background(), job.ID, storage.JobLogLevelError, message)
			failed, err := m.store.FailJob(context.Background(), job.ID, message)
			if err != nil {
				m.logger.Error("failed to mark panicked job failed", "job_id", job.ID, "error", err)
				return
			}
			m.publish(Event{Type: EventFinished, Job: &failed})
		}
	}()

	jobContext := &Context{ID: job.ID, manager: m}
	if err := runner(ctx, jobContext); err != nil {
		message := err.Error()
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			message = "任务超时。"
		} else if errors.Is(ctx.Err(), context.Canceled) {
			message = "任务已取消。"
		}
		_, _ = m.AppendLog(context.Background(), job.ID, storage.JobLogLevelError, message)
		failed, failErr := m.store.FailJob(context.Background(), job.ID, message)
		if failErr != nil {
			m.logger.Error("failed to mark job failed", "job_id", job.ID, "error", failErr)
			return
		}
		m.publish(Event{Type: EventFinished, Job: &failed})
		return
	}

	_, _ = m.AppendLog(context.Background(), job.ID, storage.JobLogLevelInfo, "任务已完成。")
	finished, err := m.store.FinishJob(context.Background(), job.ID)
	if err != nil {
		m.logger.Error("failed to finish job", "job_id", job.ID, "error", err)
		return
	}
	m.publish(Event{Type: EventFinished, Job: &finished})
}

func (m *Manager) publish(event Event) {
	jobID := ""
	if event.Job != nil {
		jobID = event.Job.ID
	}
	if event.Log != nil {
		jobID = event.Log.JobID
	}
	if jobID == "" {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	for ch := range m.subscribers[jobID] {
		select {
		case ch <- event:
		default:
			m.logger.Warn("dropping slow job subscriber event", "job_id", jobID)
		}
	}
}
