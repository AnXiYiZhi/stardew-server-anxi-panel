package storage

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
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
	Type        string
	DisplayName string
	TargetType  string
	TargetID    string
	CreatedBy   int64
	Payload     string
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
