package storage

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

const (
	JobStatusQueued    = "queued"
	JobStatusRunning   = "running"
	JobStatusSucceeded = "succeeded"
	JobStatusFailed    = "failed"
	JobStatusCanceled  = "canceled"

	JobLogLevelInfo  = "info"
	JobLogLevelWarn  = "warn"
	JobLogLevelError = "error"
	JobLogLevelDebug = "debug"
)

var ErrInvalidJobStatus = errors.New("invalid job status")
var ErrActiveJobsExist = errors.New("active jobs exist")
var ErrIdempotentJobExists = errors.New("idempotent job exists")

// ActiveJobExistsError identifies the active job which owns an exclusive
// target operation. Callers can attach to Job instead of starting a duplicate.
type ActiveJobExistsError struct {
	Job Job
}

func (e *ActiveJobExistsError) Error() string {
	return fmt.Sprintf("active job %s already exists for %s %s", e.Job.ID, e.Job.TargetType, e.Job.TargetID)
}

func (e *ActiveJobExistsError) Unwrap() error {
	return ErrActiveJobsExist
}

// IdempotentJobExistsError identifies the durable job already created for an
// idempotency key. Callers must return Job instead of launching another runner.
type IdempotentJobExistsError struct {
	Job Job
}

func (e *IdempotentJobExistsError) Error() string {
	return fmt.Sprintf("job %s already exists for the idempotency key", e.Job.ID)
}

func (e *IdempotentJobExistsError) Unwrap() error {
	return ErrIdempotentJobExists
}

type Job struct {
	ID           string
	Type         string
	DisplayName  sql.NullString
	Status       string
	TargetType   string
	TargetID     string
	CreatedBy    sql.NullInt64
	CreatedAt    string
	StartedAt    sql.NullString
	FinishedAt   sql.NullString
	ErrorMessage sql.NullString
	Payload      sql.NullString
	UpdatedAt    string
}

type JobLog struct {
	ID        int64
	JobID     string
	Level     string
	Message   string
	CreatedAt string
	Sequence  int64
}

type CreateJobParams struct {
	Type           string
	DisplayName    string
	TargetType     string
	TargetID       string
	CreatedBy      int64
	Payload        string
	IdempotencyKey string
}

type ListJobsFilter struct {
	UserID  int64
	IsAdmin bool
	Limit   int
}

type ListActiveJobsFilter struct {
	TargetType string
	TargetID   string
	Types      []string
}

func NewJobID() (string, error) {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", fmt.Errorf("create job id: %w", err)
	}
	return "job_" + hex.EncodeToString(data[:]), nil
}

func (s *Store) CreateJob(ctx context.Context, params CreateJobParams) (Job, error) {
	id, err := NewJobID()
	if err != nil {
		return Job{}, err
	}
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO jobs (id, type, display_name, status, target_type, target_id, created_by, payload)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id, type, display_name, status, target_type, target_id, created_by, created_at, started_at, finished_at, error_message, payload, updated_at
	`, id, params.Type, nullStringParam(params.DisplayName), JobStatusQueued, params.TargetType, params.TargetID, optionalCreatedBy(params.CreatedBy), nullStringParam(params.Payload))
	return scanJobRow(row)
}

// CreateIdempotentJob atomically creates a durable job identity. The same key
// remains bound to the original job after it reaches a terminal state so a
// caller can safely recover from a lost HTTP response or process restart.
func (s *Store) CreateIdempotentJob(ctx context.Context, params CreateJobParams) (Job, error) {
	if params.IdempotencyKey == "" {
		return Job{}, errors.New("idempotency key is required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Job{}, fmt.Errorf("begin idempotent job transaction: %w", err)
	}
	defer rollback(tx)

	existing, err := findJobByIdempotencyKey(ctx, tx, params.Type, params.TargetType, params.TargetID, params.IdempotencyKey)
	if err == nil {
		return Job{}, &IdempotentJobExistsError{Job: existing}
	}
	if !errors.Is(err, ErrNotFound) {
		return Job{}, err
	}

	id, err := NewJobID()
	if err != nil {
		return Job{}, err
	}
	row := tx.QueryRowContext(ctx, `
		INSERT INTO jobs (id, type, display_name, status, target_type, target_id, created_by, payload, idempotency_key)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id, type, display_name, status, target_type, target_id, created_by, created_at, started_at, finished_at, error_message, payload, updated_at
	`, id, params.Type, nullStringParam(params.DisplayName), JobStatusQueued, params.TargetType, params.TargetID, optionalCreatedBy(params.CreatedBy), nullStringParam(params.Payload), params.IdempotencyKey)
	job, err := scanJobRow(row)
	if err != nil {
		_ = tx.Rollback()
		if existing, findErr := s.findJobByIdempotencyKey(ctx, params.Type, params.TargetType, params.TargetID, params.IdempotencyKey); findErr == nil {
			return Job{}, &IdempotentJobExistsError{Job: existing}
		}
		return Job{}, err
	}
	if err := tx.Commit(); err != nil {
		return Job{}, fmt.Errorf("commit idempotent job transaction: %w", err)
	}
	return job, nil
}

// CreateExclusiveJob atomically creates a job only when no queued/running job
// with the same type and target exists. The database partial unique index is a
// second line of defense against callers from another Panel process.
func (s *Store) CreateExclusiveJob(ctx context.Context, params CreateJobParams) (Job, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Job{}, fmt.Errorf("begin exclusive job transaction: %w", err)
	}
	defer rollback(tx)

	existing, err := findActiveJob(ctx, tx, params.Type, params.TargetType, params.TargetID)
	if err == nil {
		return Job{}, &ActiveJobExistsError{Job: existing}
	}
	if !errors.Is(err, ErrNotFound) {
		return Job{}, err
	}

	id, err := NewJobID()
	if err != nil {
		return Job{}, err
	}
	row := tx.QueryRowContext(ctx, `
		INSERT INTO jobs (id, type, display_name, status, target_type, target_id, created_by, payload)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id, type, display_name, status, target_type, target_id, created_by, created_at, started_at, finished_at, error_message, payload, updated_at
	`, id, params.Type, nullStringParam(params.DisplayName), JobStatusQueued, params.TargetType, params.TargetID, optionalCreatedBy(params.CreatedBy), nullStringParam(params.Payload))
	job, err := scanJobRow(row)
	if err != nil {
		// A different process may have won after our preflight query. Roll back
		// this transaction and return its active job when it is now visible.
		_ = tx.Rollback()
		if active, findErr := s.findActiveJob(ctx, params.Type, params.TargetType, params.TargetID); findErr == nil {
			return Job{}, &ActiveJobExistsError{Job: active}
		}
		return Job{}, err
	}
	if err := tx.Commit(); err != nil {
		return Job{}, fmt.Errorf("commit exclusive job transaction: %w", err)
	}
	return job, nil
}

type jobQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (s *Store) findActiveJob(ctx context.Context, jobType, targetType, targetID string) (Job, error) {
	return findActiveJob(ctx, s.db, jobType, targetType, targetID)
}

func findActiveJob(ctx context.Context, queryer jobQueryer, jobType, targetType, targetID string) (Job, error) {
	row := queryer.QueryRowContext(ctx, `
		SELECT id, type, display_name, status, target_type, target_id, created_by, created_at, started_at, finished_at, error_message, payload, updated_at
		FROM jobs
		WHERE type = ? AND target_type = ? AND target_id = ? AND status IN (?, ?)
		ORDER BY created_at DESC, id DESC
		LIMIT 1
	`, jobType, targetType, targetID, JobStatusQueued, JobStatusRunning)
	return scanJobRow(row)
}

func (s *Store) findJobByIdempotencyKey(ctx context.Context, jobType, targetType, targetID, idempotencyKey string) (Job, error) {
	return findJobByIdempotencyKey(ctx, s.db, jobType, targetType, targetID, idempotencyKey)
}

// GetJobByIdempotencyKey returns the durable owner for an idempotent request.
// It is used before mutable runtime preconditions so a replay can recover the
// original response even when the instance state has since changed.
func (s *Store) GetJobByIdempotencyKey(ctx context.Context, jobType, targetType, targetID, idempotencyKey string) (Job, error) {
	if idempotencyKey == "" {
		return Job{}, ErrNotFound
	}
	return s.findJobByIdempotencyKey(ctx, jobType, targetType, targetID, idempotencyKey)
}

func findJobByIdempotencyKey(ctx context.Context, queryer jobQueryer, jobType, targetType, targetID, idempotencyKey string) (Job, error) {
	row := queryer.QueryRowContext(ctx, `
		SELECT id, type, display_name, status, target_type, target_id, created_by, created_at, started_at, finished_at, error_message, payload, updated_at
		FROM jobs
		WHERE type = ? AND target_type = ? AND target_id = ? AND idempotency_key = ?
		LIMIT 1
	`, jobType, targetType, targetID, idempotencyKey)
	return scanJobRow(row)
}

func (s *Store) StartJob(ctx context.Context, id string) (Job, error) {
	row := s.db.QueryRowContext(ctx, `
		UPDATE jobs
		SET status = ?, started_at = COALESCE(started_at, strftime('%Y-%m-%dT%H:%M:%fZ', 'now')), updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		WHERE id = ?
		RETURNING id, type, display_name, status, target_type, target_id, created_by, created_at, started_at, finished_at, error_message, payload, updated_at
	`, JobStatusRunning, id)
	return scanJobRow(row)
}

func (s *Store) FinishJob(ctx context.Context, id string) (Job, error) {
	row := s.db.QueryRowContext(ctx, `
		UPDATE jobs
		SET status = ?, finished_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now'), error_message = NULL, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		WHERE id = ?
		RETURNING id, type, display_name, status, target_type, target_id, created_by, created_at, started_at, finished_at, error_message, payload, updated_at
	`, JobStatusSucceeded, id)
	return scanJobRow(row)
}

func (s *Store) FailJob(ctx context.Context, id string, errorMessage string) (Job, error) {
	row := s.db.QueryRowContext(ctx, `
		UPDATE jobs
		SET status = ?, finished_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now'), error_message = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		WHERE id = ?
		RETURNING id, type, display_name, status, target_type, target_id, created_by, created_at, started_at, finished_at, error_message, payload, updated_at
	`, JobStatusFailed, errorMessage, id)
	return scanJobRow(row)
}

func (s *Store) CancelJob(ctx context.Context, id string, errorMessage string) (Job, error) {
	row := s.db.QueryRowContext(ctx, `
		UPDATE jobs
		SET status = ?, finished_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now'), error_message = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		WHERE id = ?
		RETURNING id, type, display_name, status, target_type, target_id, created_by, created_at, started_at, finished_at, error_message, payload, updated_at
	`, JobStatusCanceled, errorMessage, id)
	return scanJobRow(row)
}

func (s *Store) ListActiveJobs(ctx context.Context, filter ListActiveJobsFilter) ([]Job, error) {
	query := `
		SELECT id, type, display_name, status, target_type, target_id, created_by, created_at, started_at, finished_at, error_message, payload, updated_at
		FROM jobs
		WHERE status IN (?, ?)
	`
	args := []any{JobStatusQueued, JobStatusRunning}
	if filter.TargetType != "" {
		query += ` AND target_type = ?`
		args = append(args, filter.TargetType)
	}
	if filter.TargetID != "" {
		query += ` AND target_id = ?`
		args = append(args, filter.TargetID)
	}
	if len(filter.Types) > 0 {
		query += ` AND type IN (`
		for i, typ := range filter.Types {
			if i > 0 {
				query += `, `
			}
			query += `?`
			args = append(args, typ)
		}
		query += `)`
	}
	query += ` ORDER BY created_at ASC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list active jobs: %w", err)
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list active jobs rows: %w", err)
	}
	return jobs, nil
}

func (s *Store) GetJob(ctx context.Context, id string) (Job, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, type, display_name, status, target_type, target_id, created_by, created_at, started_at, finished_at, error_message, payload, updated_at
		FROM jobs
		WHERE id = ?
	`, id)
	return scanJobRow(row)
}

func (s *Store) ListJobs(ctx context.Context, filter ListJobsFilter) ([]Job, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	query := `
		SELECT id, type, display_name, status, target_type, target_id, created_by, created_at, started_at, finished_at, error_message, payload, updated_at
		FROM jobs
	`
	args := []any{}
	if !filter.IsAdmin {
		query += `WHERE created_by = ?
	`
		args = append(args, filter.UserID)
	}
	query += `ORDER BY created_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list jobs rows: %w", err)
	}
	return jobs, nil
}

func (s *Store) ClearJobs(ctx context.Context) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin clear jobs transaction: %w", err)
	}
	defer rollback(tx)

	var active int64
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM jobs WHERE status IN (?, ?)
	`, JobStatusQueued, JobStatusRunning).Scan(&active); err != nil {
		return 0, fmt.Errorf("count active jobs: %w", err)
	}
	if active > 0 {
		return 0, ErrActiveJobsExist
	}

	var count int64
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM jobs`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count jobs: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM job_logs`); err != nil {
		return 0, fmt.Errorf("delete job logs: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM jobs`); err != nil {
		return 0, fmt.Errorf("delete jobs: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit clear jobs transaction: %w", err)
	}
	return count, nil
}

// LatestJobsClearedAt returns the durable audit time of the most recent
// successful whole job-center clear. The Web layer uses this only as legacy
// terminality evidence for an import whose exact job row was removed by an
// older Panel version after ClearJobs had already proved there were no active
// jobs.
func (s *Store) LatestJobsClearedAt(ctx context.Context) (time.Time, bool, error) {
	var value string
	err := s.db.QueryRowContext(ctx, `
		SELECT created_at
		FROM audit_logs
		WHERE action = 'jobs_cleared' AND target_type = 'jobs' AND target_id = 'all'
		ORDER BY id DESC
		LIMIT 1
	`).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, fmt.Errorf("load latest jobs clear audit: %w", err)
	}
	parsed, err := parseDBTime(value)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("parse latest jobs clear audit: %w", err)
	}
	return parsed, true, nil
}

func (s *Store) ClearJobErrorLogs(ctx context.Context) (int64, int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("begin clear error logs transaction: %w", err)
	}
	defer rollback(tx)

	logResult, err := tx.ExecContext(ctx, `DELETE FROM job_logs WHERE level = ?`, JobLogLevelError)
	if err != nil {
		return 0, 0, fmt.Errorf("delete error job logs: %w", err)
	}
	logsDeleted, err := logResult.RowsAffected()
	if err != nil {
		return 0, 0, fmt.Errorf("count deleted error job logs: %w", err)
	}

	messageResult, err := tx.ExecContext(ctx, `
		UPDATE jobs
		SET error_message = NULL, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		WHERE error_message IS NOT NULL
	`)
	if err != nil {
		return 0, 0, fmt.Errorf("clear job error messages: %w", err)
	}
	messagesCleared, err := messageResult.RowsAffected()
	if err != nil {
		return 0, 0, fmt.Errorf("count cleared job error messages: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("commit clear error logs transaction: %w", err)
	}
	return logsDeleted, messagesCleared, nil
}

func (s *Store) AppendJobLog(ctx context.Context, jobID string, level string, message string) (JobLog, error) {
	if !IsValidJobLogLevel(level) {
		return JobLog{}, ErrInvalidJobStatus
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return JobLog{}, fmt.Errorf("begin append job log transaction: %w", err)
	}
	defer rollback(tx)

	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM jobs WHERE id = ?`, jobID).Scan(&exists); err != nil {
		return JobLog{}, fmt.Errorf("check job exists: %w", err)
	}
	if exists == 0 {
		return JobLog{}, ErrNotFound
	}

	var sequence int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(sequence), 0) + 1 FROM job_logs WHERE job_id = ?`, jobID).Scan(&sequence); err != nil {
		return JobLog{}, fmt.Errorf("next job log sequence: %w", err)
	}

	row := tx.QueryRowContext(ctx, `
		INSERT INTO job_logs (job_id, level, message, sequence)
		VALUES (?, ?, ?, ?)
		RETURNING id, job_id, level, message, created_at, sequence
	`, jobID, level, message, sequence)
	logLine, err := scanJobLogRow(row)
	if err != nil {
		return JobLog{}, err
	}
	if err := tx.Commit(); err != nil {
		return JobLog{}, fmt.Errorf("commit append job log transaction: %w", err)
	}
	return logLine, nil
}

func (s *Store) ListJobLogs(ctx context.Context, jobID string, afterSequence int64, limit int) ([]JobLog, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, job_id, level, message, created_at, sequence
		FROM job_logs
		WHERE job_id = ? AND sequence > ?
		ORDER BY sequence ASC
		LIMIT ?
	`, jobID, afterSequence, limit)
	if err != nil {
		return nil, fmt.Errorf("list job logs: %w", err)
	}
	defer rows.Close()

	var logs []JobLog
	for rows.Next() {
		logLine, err := scanJobLog(rows)
		if err != nil {
			return nil, err
		}
		logs = append(logs, logLine)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list job logs rows: %w", err)
	}
	return logs, nil
}

func (s *Store) ListLatestJobLogs(ctx context.Context, jobID string, limit int) ([]JobLog, bool, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, job_id, level, message, created_at, sequence
		FROM job_logs
		WHERE job_id = ?
		ORDER BY sequence DESC
		LIMIT ?
	`, jobID, limit+1)
	if err != nil {
		return nil, false, fmt.Errorf("list latest job logs: %w", err)
	}
	defer rows.Close()

	logs := make([]JobLog, 0, limit+1)
	for rows.Next() {
		logLine, err := scanJobLog(rows)
		if err != nil {
			return nil, false, err
		}
		logs = append(logs, logLine)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("list latest job logs rows: %w", err)
	}

	hasEarlier := len(logs) > limit
	if hasEarlier {
		logs = logs[:limit]
	}
	for left, right := 0, len(logs)-1; left < right; left, right = left+1, right-1 {
		logs[left], logs[right] = logs[right], logs[left]
	}
	return logs, hasEarlier, nil
}

func (s *Store) ListInterruptedJobs(ctx context.Context) ([]Job, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, type, display_name, status, target_type, target_id, created_by, created_at, started_at, finished_at, error_message, payload, updated_at
		FROM jobs
		WHERE status IN (?, ?)
		ORDER BY created_at ASC
	`, JobStatusQueued, JobStatusRunning)
	if err != nil {
		return nil, fmt.Errorf("list interrupted jobs: %w", err)
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list interrupted jobs rows: %w", err)
	}
	return jobs, nil
}

func (s *Store) FailInterruptedJobs(ctx context.Context, errorMessage string) (int64, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE jobs
		SET status = ?, finished_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now'), error_message = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		WHERE status IN (?, ?)
	`, JobStatusFailed, errorMessage, JobStatusQueued, JobStatusRunning)
	if err != nil {
		return 0, fmt.Errorf("fail interrupted jobs: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("count interrupted jobs: %w", err)
	}
	return count, nil
}

func IsValidJobStatus(status string) bool {
	switch status {
	case JobStatusQueued, JobStatusRunning, JobStatusSucceeded, JobStatusFailed, JobStatusCanceled:
		return true
	default:
		return false
	}
}

func IsValidJobLogLevel(level string) bool {
	switch level {
	case JobLogLevelInfo, JobLogLevelWarn, JobLogLevelError, JobLogLevelDebug:
		return true
	default:
		return false
	}
}

func scanJobRow(row *sql.Row) (Job, error) {
	var job Job
	if err := row.Scan(&job.ID, &job.Type, &job.DisplayName, &job.Status, &job.TargetType, &job.TargetID, &job.CreatedBy, &job.CreatedAt, &job.StartedAt, &job.FinishedAt, &job.ErrorMessage, &job.Payload, &job.UpdatedAt); err != nil {
		return Job{}, mapScanErr(err, "scan job")
	}
	return job, nil
}

func scanJob(rows *sql.Rows) (Job, error) {
	var job Job
	if err := rows.Scan(&job.ID, &job.Type, &job.DisplayName, &job.Status, &job.TargetType, &job.TargetID, &job.CreatedBy, &job.CreatedAt, &job.StartedAt, &job.FinishedAt, &job.ErrorMessage, &job.Payload, &job.UpdatedAt); err != nil {
		return Job{}, fmt.Errorf("scan job: %w", err)
	}
	return job, nil
}

func scanJobLogRow(row *sql.Row) (JobLog, error) {
	var logLine JobLog
	if err := row.Scan(&logLine.ID, &logLine.JobID, &logLine.Level, &logLine.Message, &logLine.CreatedAt, &logLine.Sequence); err != nil {
		return JobLog{}, mapScanErr(err, "scan job log")
	}
	return logLine, nil
}

func scanJobLog(rows *sql.Rows) (JobLog, error) {
	var logLine JobLog
	if err := rows.Scan(&logLine.ID, &logLine.JobID, &logLine.Level, &logLine.Message, &logLine.CreatedAt, &logLine.Sequence); err != nil {
		return JobLog{}, fmt.Errorf("scan job log: %w", err)
	}
	return logLine, nil
}

func optionalCreatedBy(value int64) any {
	if value <= 0 {
		return nil
	}
	return value
}

func nullStringParam(value string) any {
	if value == "" {
		return nil
	}
	return value
}
