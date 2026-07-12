package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type ControlCommand struct {
	CommandID       string            `json:"commandId"`
	InstanceID      string            `json:"instanceId"`
	CommandType     string            `json:"commandType"`
	TargetType      string            `json:"targetType,omitempty"`
	TargetID        string            `json:"targetId,omitempty"`
	TargetLabel     string            `json:"targetLabel,omitempty"`
	ActorUserID     *int64            `json:"actorUserId,omitempty"`
	ActorUsername   string            `json:"actorUsername,omitempty"`
	Status          string            `json:"status"`
	ResultSupported bool              `json:"resultSupported"`
	ErrorCode       string            `json:"errorCode,omitempty"`
	ResultMessage   string            `json:"resultMessage,omitempty"`
	ResultDetails   map[string]string `json:"resultDetails,omitempty"`
	SubmittedAt     time.Time         `json:"submittedAt"`
	CompletedAt     *time.Time        `json:"completedAt,omitempty"`
	UpdatedAt       time.Time         `json:"updatedAt"`
	ImportedAt      *time.Time        `json:"importedAt,omitempty"`
}

type CreateControlCommandParams struct {
	CommandID       string
	InstanceID      string
	CommandType     string
	TargetType      string
	TargetID        string
	TargetLabel     string
	ActorUserID     *int64
	ActorUsername   string
	Status          string
	ResultSupported bool
	SubmittedAt     time.Time
}

type ImportControlCommandResultParams struct {
	CommandID     string
	InstanceID    string
	Status        string
	ErrorCode     string
	ResultMessage string
	ResultDetails map[string]string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	ImportedAt    time.Time
}

func (s *Store) CreateControlCommand(ctx context.Context, p CreateControlCommandParams) error {
	if p.SubmittedAt.IsZero() {
		p.SubmittedAt = time.Now().UTC()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin create control command: %w", err)
	}
	defer rollback(tx)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO control_commands (
			command_id, instance_id, command_type, target_type, target_id, target_label,
			actor_user_id, actor_username, status, result_supported, submitted_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(command_id) DO UPDATE SET
			instance_id = excluded.instance_id,
			command_type = excluded.command_type,
			target_type = excluded.target_type,
			target_id = excluded.target_id,
			target_label = excluded.target_label,
			actor_user_id = excluded.actor_user_id,
			actor_username = excluded.actor_username,
			result_supported = MAX(control_commands.result_supported, excluded.result_supported),
			submitted_at = MIN(control_commands.submitted_at, excluded.submitted_at)
	`, p.CommandID, p.InstanceID, p.CommandType, nullString(p.TargetType), nullString(p.TargetID), nullString(p.TargetLabel),
		optionalInt64(p.ActorUserID), nullString(p.ActorUsername), p.Status, boolToInt(p.ResultSupported),
		formatDBTime(p.SubmittedAt), formatDBTime(p.SubmittedAt))
	if err != nil {
		return fmt.Errorf("create control command: %w", err)
	}
	if err := writeFinalControlCommandAuditTx(ctx, tx, p.CommandID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit create control command: %w", err)
	}
	return nil
}

func (s *Store) ImportControlCommandResult(ctx context.Context, p ImportControlCommandResultParams) error {
	if p.UpdatedAt.IsZero() {
		p.UpdatedAt = p.ImportedAt
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = p.UpdatedAt
	}
	details, err := json.Marshal(p.ResultDetails)
	if err != nil {
		return fmt.Errorf("marshal control command result details: %w", err)
	}
	completed := any(nil)
	if isControlCommandTerminal(p.Status) {
		completed = formatDBTime(p.UpdatedAt)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin control command import: %w", err)
	}
	defer rollback(tx)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO control_commands (
			command_id, instance_id, command_type, status, result_supported,
			error_code, result_message, result_details_json, submitted_at, completed_at,
			updated_at, imported_at
		) VALUES (?, ?, 'unknown', ?, 1, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(command_id) DO UPDATE SET
			status = excluded.status,
			error_code = excluded.error_code,
			result_message = excluded.result_message,
			result_details_json = excluded.result_details_json,
			completed_at = excluded.completed_at,
			updated_at = excluded.updated_at,
			imported_at = excluded.imported_at
		WHERE excluded.updated_at >= control_commands.updated_at
	`, p.CommandID, p.InstanceID, p.Status, nullString(p.ErrorCode), nullString(p.ResultMessage), string(details),
		formatDBTime(p.CreatedAt), completed, formatDBTime(p.UpdatedAt), formatDBTime(p.ImportedAt))
	if err != nil {
		return fmt.Errorf("upsert control command result: %w", err)
	}
	if err := writeFinalControlCommandAuditTx(ctx, tx, p.CommandID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit control command import: %w", err)
	}
	return nil
}

func writeFinalControlCommandAuditTx(ctx context.Context, tx *sql.Tx, commandID string) error {
	var written bool
	var status, commandType string
	if err := tx.QueryRowContext(ctx, `SELECT final_audit_written, status, command_type FROM control_commands WHERE command_id = ?`, commandID).Scan(&written, &status, &commandType); err != nil {
		return fmt.Errorf("read control command audit state: %w", err)
	}
	if written || commandType == "unknown" || !isControlCommandTerminal(status) {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `
				INSERT INTO audit_logs (actor_user_id, action, target_type, target_id, metadata_json)
				SELECT actor_user_id, 'control_command_completed', 'control_command', command_id,
					json_object(
						'commandId', command_id, 'commandType', command_type,
						'targetType', COALESCE(target_type, ''), 'targetId', COALESCE(target_id, ''),
						'targetLabel', COALESCE(target_label, ''), 'status', status,
						'errorCode', COALESCE(error_code, '')
					)
				FROM control_commands WHERE command_id = ?
			`, commandID); err != nil {
		return fmt.Errorf("write control command final audit: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE control_commands SET final_audit_written = 1 WHERE command_id = ?`, commandID); err != nil {
		return fmt.Errorf("mark control command final audit: %w", err)
	}
	return nil
}

func (s *Store) GetControlCommand(ctx context.Context, commandID string) (ControlCommand, error) {
	row := s.db.QueryRowContext(ctx, controlCommandSelect+` WHERE c.command_id = ?`, commandID)
	return scanControlCommand(row)
}

func (s *Store) ListControlCommands(ctx context.Context, instanceID string, limit int) ([]ControlCommand, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, controlCommandSelect+` WHERE c.instance_id = ? ORDER BY c.submitted_at DESC, c.command_id DESC LIMIT ?`, instanceID, limit)
	if err != nil {
		return nil, fmt.Errorf("list control commands: %w", err)
	}
	defer rows.Close()
	result := make([]ControlCommand, 0)
	for rows.Next() {
		item, err := scanControlCommand(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) HasImportedControlCommandResult(ctx context.Context, commandID string, updatedAt time.Time) (bool, error) {
	var value sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT imported_at FROM control_commands WHERE command_id = ? AND updated_at >= ?`, commandID, formatDBTime(updatedAt)).Scan(&value)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil && value.Valid, err
}

func (s *Store) CleanupControlCommands(ctx context.Context, before time.Time, keep int) (int64, error) {
	if keep <= 0 {
		keep = 1000
	}
	res, err := s.db.ExecContext(ctx, `
		DELETE FROM control_commands
		WHERE status NOT IN ('queued', 'running')
		  AND (submitted_at < ? OR command_id NOT IN (
			SELECT command_id FROM control_commands
			ORDER BY submitted_at DESC, command_id DESC LIMIT ?
		  ))
	`, formatDBTime(before), keep)
	if err != nil {
		return 0, fmt.Errorf("cleanup control commands: %w", err)
	}
	return res.RowsAffected()
}

func (s *Store) MarkStaleControlCommandsUnknown(ctx context.Context, before time.Time) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin stale control command update: %w", err)
	}
	defer rollback(tx)
	res, err := tx.ExecContext(ctx, `
		UPDATE control_commands
		SET status = 'unknown', error_code = 'execution_interrupted',
			result_message = 'Command result could not be confirmed; it will not be retried.',
			completed_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now'),
			updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		WHERE status = 'running' AND updated_at < ?
	`, formatDBTime(before))
	if err != nil {
		return 0, fmt.Errorf("mark stale control commands unknown: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO audit_logs (actor_user_id, action, target_type, target_id, metadata_json)
		SELECT actor_user_id, 'control_command_completed', 'control_command', command_id,
			json_object(
				'commandId', command_id, 'commandType', command_type,
				'targetType', COALESCE(target_type, ''), 'targetId', COALESCE(target_id, ''),
				'targetLabel', COALESCE(target_label, ''), 'status', 'unknown',
				'errorCode', 'execution_interrupted'
			)
		FROM control_commands
		WHERE status = 'unknown' AND error_code = 'execution_interrupted' AND final_audit_written = 0
	`); err != nil {
		return 0, fmt.Errorf("audit stale control commands: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE control_commands SET final_audit_written = 1
		WHERE status = 'unknown' AND error_code = 'execution_interrupted' AND final_audit_written = 0
	`); err != nil {
		return 0, fmt.Errorf("mark stale control command audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit stale control command update: %w", err)
	}
	return res.RowsAffected()
}

func (s *Store) LatestControlCommandUpdate(ctx context.Context, instanceID string) (*time.Time, error) {
	var value sql.NullString
	if err := s.db.QueryRowContext(ctx, `SELECT MAX(updated_at) FROM control_commands WHERE instance_id = ? AND imported_at IS NOT NULL`, instanceID).Scan(&value); err != nil {
		return nil, err
	}
	if !value.Valid {
		return nil, nil
	}
	parsed, err := parseDBTime(value.String)
	return &parsed, err
}

const controlCommandSelect = `
	SELECT c.command_id, c.instance_id, c.command_type, c.target_type, c.target_id, c.target_label,
		c.actor_user_id, COALESCE(c.actor_username, u.username, ''), c.status, c.result_supported,
		c.error_code, c.result_message, c.result_details_json, c.submitted_at, c.completed_at,
		c.updated_at, c.imported_at
	FROM control_commands c LEFT JOIN users u ON u.id = c.actor_user_id`

type controlCommandScanner interface{ Scan(...any) error }

func scanControlCommand(row controlCommandScanner) (ControlCommand, error) {
	var c ControlCommand
	var targetType, targetID, targetLabel, actorUsername, errorCode, message sql.NullString
	var actorID sql.NullInt64
	var details, submitted, updated string
	var completed, imported sql.NullString
	if err := row.Scan(&c.CommandID, &c.InstanceID, &c.CommandType, &targetType, &targetID, &targetLabel,
		&actorID, &actorUsername, &c.Status, &c.ResultSupported, &errorCode, &message, &details,
		&submitted, &completed, &updated, &imported); err != nil {
		return ControlCommand{}, mapScanErr(err, "scan control command")
	}
	c.TargetType, c.TargetID, c.TargetLabel = targetType.String, targetID.String, targetLabel.String
	c.ActorUsername, c.ErrorCode, c.ResultMessage = actorUsername.String, errorCode.String, message.String
	if actorID.Valid {
		c.ActorUserID = &actorID.Int64
	}
	_ = json.Unmarshal([]byte(details), &c.ResultDetails)
	var err error
	if c.SubmittedAt, err = parseDBTime(submitted); err != nil {
		return ControlCommand{}, err
	}
	if c.UpdatedAt, err = parseDBTime(updated); err != nil {
		return ControlCommand{}, err
	}
	if completed.Valid {
		t, e := parseDBTime(completed.String)
		if e != nil {
			return ControlCommand{}, e
		}
		c.CompletedAt = &t
	}
	if imported.Valid {
		t, e := parseDBTime(imported.String)
		if e != nil {
			return ControlCommand{}, e
		}
		c.ImportedAt = &t
	}
	return c, nil
}

func isControlCommandTerminal(status string) bool {
	switch status {
	case "succeeded", "dispatched", "failed", "expired", "unknown":
		return true
	default:
		return false
	}
}
