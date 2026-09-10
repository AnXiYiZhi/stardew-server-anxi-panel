package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

const (
	FarmhandDeleteModeWait           = "wait"
	FarmhandDeleteModeMaintenanceNow = "maintenance_now"

	FarmhandDeleteIntentWaiting          = "waiting"
	FarmhandDeleteIntentLaunching        = "launching"
	FarmhandDeleteIntentCountdown        = "countdown"
	FarmhandDeleteIntentActive           = "active"
	FarmhandDeleteIntentRecoveryRequired = "recovery_required"
	FarmhandDeleteIntentCompleted        = "completed"
	FarmhandDeleteIntentCanceled         = "canceled"
	FarmhandDeleteIntentExpired          = "expired"
	FarmhandDeleteIntentFailed           = "failed"
)

type FarmhandDeleteIntent struct {
	InstanceID     string
	OperationID    string
	Mode           string
	Status         string
	PlayerID       string
	ExpectedName   string
	ExpectedSaveID string
	CreatedBy      sql.NullInt64
	JobID          sql.NullString
	ExpiresAt      string
	BackupName     string
	LastError      string
	CreatedAt      string
	UpdatedAt      string
}

type CreateFarmhandDeleteIntentParams struct {
	InstanceID     string
	OperationID    string
	Mode           string
	Status         string
	PlayerID       string
	ExpectedName   string
	ExpectedSaveID string
	CreatedBy      int64
	ExpiresAt      string
}

func (s *Store) CreateFarmhandDeleteIntent(ctx context.Context, params CreateFarmhandDeleteIntentParams) (FarmhandDeleteIntent, error) {
	if params.InstanceID == "" || params.OperationID == "" || params.PlayerID == "" || params.ExpectedSaveID == "" ||
		!validFarmhandDeleteMode(params.Mode) || (params.Status != FarmhandDeleteIntentWaiting && params.Status != FarmhandDeleteIntentLaunching) {
		return FarmhandDeleteIntent{}, ErrConflict
	}
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO farmhand_delete_intents (
			instance_id, operation_id, mode, status, player_id, expected_name,
			expected_save_id, created_by, expires_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(instance_id) DO UPDATE SET
			operation_id=excluded.operation_id,
			mode=excluded.mode,
			status=excluded.status,
			player_id=excluded.player_id,
			expected_name=excluded.expected_name,
			expected_save_id=excluded.expected_save_id,
			created_by=excluded.created_by,
			job_id=NULL,
			expires_at=excluded.expires_at,
			backup_name='',
			last_error='',
			created_at=strftime('%Y-%m-%dT%H:%M:%fZ', 'now'),
			updated_at=strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		WHERE farmhand_delete_intents.status IN ('completed', 'canceled', 'expired', 'failed')
		RETURNING instance_id, operation_id, mode, status, player_id, expected_name,
			expected_save_id, created_by, job_id, expires_at, backup_name, last_error,
			created_at, updated_at
	`, params.InstanceID, params.OperationID, params.Mode, params.Status, params.PlayerID,
		params.ExpectedName, params.ExpectedSaveID, optionalCreatedBy(params.CreatedBy), params.ExpiresAt)
	intent, err := scanFarmhandDeleteIntentRow(row)
	if errors.Is(err, ErrNotFound) {
		return FarmhandDeleteIntent{}, ErrConflict
	}
	return intent, err
}

func (s *Store) GetFarmhandDeleteIntent(ctx context.Context, instanceID string) (FarmhandDeleteIntent, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT instance_id, operation_id, mode, status, player_id, expected_name,
			expected_save_id, created_by, job_id, expires_at, backup_name, last_error,
			created_at, updated_at
		FROM farmhand_delete_intents WHERE instance_id=?
	`, instanceID)
	return scanFarmhandDeleteIntentRow(row)
}

func (s *Store) ListActionableFarmhandDeleteIntents(ctx context.Context) ([]FarmhandDeleteIntent, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT instance_id, operation_id, mode, status, player_id, expected_name,
			expected_save_id, created_by, job_id, expires_at, backup_name, last_error,
			created_at, updated_at
		FROM farmhand_delete_intents
		WHERE status IN ('waiting', 'launching', 'countdown', 'active', 'recovery_required')
		ORDER BY created_at ASC, instance_id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list farmhand delete intents: %w", err)
	}
	defer rows.Close()
	intents := []FarmhandDeleteIntent{}
	for rows.Next() {
		intent, err := scanFarmhandDeleteIntent(rows)
		if err != nil {
			return nil, err
		}
		intents = append(intents, intent)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list farmhand delete intents rows: %w", err)
	}
	return intents, nil
}

func (s *Store) BeginFarmhandDeleteRecovery(ctx context.Context, instanceID, operationID string) (FarmhandDeleteIntent, error) {
	row := s.db.QueryRowContext(ctx, `
		UPDATE farmhand_delete_intents
		SET status='launching', job_id=NULL, updated_at=strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		WHERE instance_id=? AND operation_id=? AND status='recovery_required'
		RETURNING instance_id, operation_id, mode, status, player_id, expected_name,
			expected_save_id, created_by, job_id, expires_at, backup_name, last_error,
			created_at, updated_at
	`, instanceID, operationID)
	return scanFarmhandDeleteIntentRow(row)
}

func (s *Store) ClaimFarmhandDeleteIntent(ctx context.Context, instanceID, operationID string) (FarmhandDeleteIntent, error) {
	row := s.db.QueryRowContext(ctx, `
		UPDATE farmhand_delete_intents
		SET status='launching', updated_at=strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		WHERE instance_id=? AND operation_id=? AND status='waiting'
		RETURNING instance_id, operation_id, mode, status, player_id, expected_name,
			expected_save_id, created_by, job_id, expires_at, backup_name, last_error,
			created_at, updated_at
	`, instanceID, operationID)
	return scanFarmhandDeleteIntentRow(row)
}

func (s *Store) BindFarmhandDeleteIntentJob(ctx context.Context, instanceID, operationID, jobID string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE farmhand_delete_intents
		SET status=CASE WHEN mode='maintenance_now' THEN 'countdown' ELSE 'active' END,
			job_id=?, updated_at=strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		WHERE instance_id=? AND operation_id=? AND status='launching'
	`, jobID, instanceID, operationID)
	if err != nil {
		return err
	}
	return requireOneRow(result)
}

func (s *Store) BindFarmhandDeleteRecoveryJob(ctx context.Context, instanceID, operationID, jobID string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE farmhand_delete_intents
		SET status='active', job_id=?, updated_at=strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		WHERE instance_id=? AND operation_id=? AND status='launching'
	`, jobID, instanceID, operationID)
	if err != nil {
		return err
	}
	return requireOneRow(result)
}

func (s *Store) ActivateFarmhandDeleteIntent(ctx context.Context, instanceID, operationID string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE farmhand_delete_intents
		SET status='active', updated_at=strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		WHERE instance_id=? AND operation_id=? AND status='countdown'
	`, instanceID, operationID)
	if err != nil {
		return err
	}
	return requireOneRow(result)
}

func (s *Store) CancelFarmhandDeleteCountdown(ctx context.Context, instanceID, operationID, reason string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE farmhand_delete_intents
		SET status='canceled', last_error=?, updated_at=strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		WHERE instance_id=? AND operation_id=? AND status='countdown'
	`, reason, instanceID, operationID)
	if err != nil {
		return err
	}
	return requireOneRow(result)
}

func (s *Store) UpdateFarmhandDeleteIntent(ctx context.Context, instanceID, operationID, status, backupName, lastError string) error {
	if !validFarmhandDeleteStatus(status) {
		return ErrConflict
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE farmhand_delete_intents
		SET status=?, backup_name=?, last_error=?, updated_at=strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		WHERE instance_id=? AND operation_id=?
	`, status, backupName, lastError, instanceID, operationID)
	if err != nil {
		return err
	}
	return requireOneRow(result)
}

func (s *Store) ResetLaunchingFarmhandDeleteIntent(ctx context.Context, instanceID, operationID string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE farmhand_delete_intents
		SET status='waiting', job_id=NULL, updated_at=strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		WHERE instance_id=? AND operation_id=? AND status='launching'
	`, instanceID, operationID)
	if err != nil {
		return err
	}
	return requireOneRow(result)
}

// ReconcileFarmhandDeleteIntent only changes the observation it was based on.
// A runner's completion or a newly bound recovery job wins over a stale poll.
func (s *Store) ReconcileFarmhandDeleteIntent(ctx context.Context, previous FarmhandDeleteIntent, status, reason string) error {
	if !validFarmhandDeleteStatus(status) {
		return ErrConflict
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE farmhand_delete_intents
		SET status=?, last_error=?, updated_at=strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		WHERE instance_id=? AND operation_id=? AND status=? AND job_id IS ? AND updated_at=?
	`, status, reason, previous.InstanceID, previous.OperationID, previous.Status, previous.JobID, previous.UpdatedAt)
	if err != nil {
		return err
	}
	return requireOneRow(result)
}

func (s *Store) CancelWaitingFarmhandDeleteIntent(ctx context.Context, instanceID, operationID, reason string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE farmhand_delete_intents
		SET status='canceled', last_error=?, updated_at=strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		WHERE instance_id=? AND operation_id=? AND status='waiting'
	`, reason, instanceID, operationID)
	if err != nil {
		return err
	}
	return requireOneRow(result)
}

func requireOneRow(result sql.Result) error {
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrConflict
	}
	return nil
}

func scanFarmhandDeleteIntentRow(row *sql.Row) (FarmhandDeleteIntent, error) {
	var intent FarmhandDeleteIntent
	err := row.Scan(&intent.InstanceID, &intent.OperationID, &intent.Mode, &intent.Status,
		&intent.PlayerID, &intent.ExpectedName, &intent.ExpectedSaveID, &intent.CreatedBy,
		&intent.JobID, &intent.ExpiresAt, &intent.BackupName, &intent.LastError,
		&intent.CreatedAt, &intent.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return FarmhandDeleteIntent{}, ErrNotFound
	}
	if err != nil {
		return FarmhandDeleteIntent{}, fmt.Errorf("scan farmhand delete intent: %w", err)
	}
	return intent, nil
}

func scanFarmhandDeleteIntent(rows *sql.Rows) (FarmhandDeleteIntent, error) {
	var intent FarmhandDeleteIntent
	if err := rows.Scan(&intent.InstanceID, &intent.OperationID, &intent.Mode, &intent.Status,
		&intent.PlayerID, &intent.ExpectedName, &intent.ExpectedSaveID, &intent.CreatedBy,
		&intent.JobID, &intent.ExpiresAt, &intent.BackupName, &intent.LastError,
		&intent.CreatedAt, &intent.UpdatedAt); err != nil {
		return FarmhandDeleteIntent{}, fmt.Errorf("scan farmhand delete intent row: %w", err)
	}
	return intent, nil
}

func validFarmhandDeleteMode(mode string) bool {
	return mode == FarmhandDeleteModeWait || mode == FarmhandDeleteModeMaintenanceNow
}

func validFarmhandDeleteStatus(status string) bool {
	switch status {
	case FarmhandDeleteIntentWaiting, FarmhandDeleteIntentLaunching, FarmhandDeleteIntentCountdown, FarmhandDeleteIntentActive,
		FarmhandDeleteIntentRecoveryRequired, FarmhandDeleteIntentCompleted,
		FarmhandDeleteIntentCanceled, FarmhandDeleteIntentExpired, FarmhandDeleteIntentFailed:
		return true
	default:
		return false
	}
}
