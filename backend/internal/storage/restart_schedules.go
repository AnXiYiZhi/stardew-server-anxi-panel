package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

type RestartSchedule struct {
	InstanceID           string
	Enabled              bool
	ShutdownTime         string
	StartupTime          string
	Timezone             string
	WarningMinutes       []int
	BackupBeforeShutdown bool
	SkipIfPlayersOnline  bool
	LastShutdownAt       sql.NullString
	LastStartupAt        sql.NullString
	LastStatus           sql.NullString
	LastMessage          sql.NullString
	CreatedAt            string
	UpdatedAt            string
}

type UpsertRestartScheduleParams struct {
	InstanceID           string
	Enabled              bool
	ShutdownTime         string
	StartupTime          string
	Timezone             string
	WarningMinutes       []int
	BackupBeforeShutdown bool
	SkipIfPlayersOnline  bool
}

func (s *Store) GetRestartSchedule(ctx context.Context, instanceID string) (RestartSchedule, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT instance_id, enabled, shutdown_time, startup_time, timezone, warning_minutes_json,
			backup_before_shutdown, skip_if_players_online, last_shutdown_at, last_startup_at,
			last_status, last_message, created_at, updated_at
		FROM restart_schedules
		WHERE instance_id = ?
	`, instanceID)
	return scanRestartScheduleRow(row)
}

func (s *Store) ListEnabledRestartSchedules(ctx context.Context) ([]RestartSchedule, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT instance_id, enabled, shutdown_time, startup_time, timezone, warning_minutes_json,
			backup_before_shutdown, skip_if_players_online, last_shutdown_at, last_startup_at,
			last_status, last_message, created_at, updated_at
		FROM restart_schedules
		WHERE enabled = 1
		ORDER BY instance_id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list enabled restart schedules: %w", err)
	}
	defer rows.Close()

	schedules := []RestartSchedule{}
	for rows.Next() {
		schedule, err := scanRestartSchedule(rows)
		if err != nil {
			return nil, err
		}
		schedules = append(schedules, schedule)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list enabled restart schedules rows: %w", err)
	}
	return schedules, nil
}

func (s *Store) UpsertRestartSchedule(ctx context.Context, params UpsertRestartScheduleParams) (RestartSchedule, error) {
	warnings, err := json.Marshal(params.WarningMinutes)
	if err != nil {
		return RestartSchedule{}, fmt.Errorf("marshal warning minutes: %w", err)
	}
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO restart_schedules (
			instance_id, enabled, shutdown_time, startup_time, timezone, warning_minutes_json,
			backup_before_shutdown, skip_if_players_online
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(instance_id) DO UPDATE SET
			enabled = excluded.enabled,
			shutdown_time = excluded.shutdown_time,
			startup_time = excluded.startup_time,
			timezone = excluded.timezone,
			warning_minutes_json = excluded.warning_minutes_json,
			backup_before_shutdown = excluded.backup_before_shutdown,
			skip_if_players_online = excluded.skip_if_players_online,
			updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		RETURNING instance_id, enabled, shutdown_time, startup_time, timezone, warning_minutes_json,
			backup_before_shutdown, skip_if_players_online, last_shutdown_at, last_startup_at,
			last_status, last_message, created_at, updated_at
	`, params.InstanceID, boolInt(params.Enabled), params.ShutdownTime, params.StartupTime, params.Timezone,
		string(warnings), boolInt(params.BackupBeforeShutdown), boolInt(params.SkipIfPlayersOnline))
	return scanRestartScheduleRow(row)
}

func (s *Store) MarkRestartScheduleAction(ctx context.Context, instanceID, action, happenedAt, status, message string) error {
	column := "last_startup_at"
	if action == "shutdown" {
		column = "last_shutdown_at"
	}
	query := fmt.Sprintf(`
		UPDATE restart_schedules
		SET %s = ?, last_status = ?, last_message = ?, updated_at = strftime('%%Y-%%m-%%dT%%H:%%M:%%fZ', 'now')
		WHERE instance_id = ?
	`, column)
	if _, err := s.db.ExecContext(ctx, query, happenedAt, status, message, instanceID); err != nil {
		return fmt.Errorf("mark restart schedule action: %w", err)
	}
	return nil
}

func scanRestartScheduleRow(row *sql.Row) (RestartSchedule, error) {
	var schedule RestartSchedule
	var enabled, backupBeforeShutdown, skipIfPlayersOnline int
	var warningJSON string
	if err := row.Scan(&schedule.InstanceID, &enabled, &schedule.ShutdownTime, &schedule.StartupTime,
		&schedule.Timezone, &warningJSON, &backupBeforeShutdown, &skipIfPlayersOnline,
		&schedule.LastShutdownAt, &schedule.LastStartupAt, &schedule.LastStatus,
		&schedule.LastMessage, &schedule.CreatedAt, &schedule.UpdatedAt); err != nil {
		return RestartSchedule{}, mapScanErr(err, "scan restart schedule")
	}
	if err := json.Unmarshal([]byte(warningJSON), &schedule.WarningMinutes); err != nil {
		return RestartSchedule{}, fmt.Errorf("parse warning minutes: %w", err)
	}
	schedule.Enabled = enabled == 1
	schedule.BackupBeforeShutdown = backupBeforeShutdown == 1
	schedule.SkipIfPlayersOnline = skipIfPlayersOnline == 1
	return schedule, nil
}

func scanRestartSchedule(rows *sql.Rows) (RestartSchedule, error) {
	var schedule RestartSchedule
	var enabled, backupBeforeShutdown, skipIfPlayersOnline int
	var warningJSON string
	if err := rows.Scan(&schedule.InstanceID, &enabled, &schedule.ShutdownTime, &schedule.StartupTime,
		&schedule.Timezone, &warningJSON, &backupBeforeShutdown, &skipIfPlayersOnline,
		&schedule.LastShutdownAt, &schedule.LastStartupAt, &schedule.LastStatus,
		&schedule.LastMessage, &schedule.CreatedAt, &schedule.UpdatedAt); err != nil {
		return RestartSchedule{}, fmt.Errorf("scan restart schedule: %w", err)
	}
	if err := json.Unmarshal([]byte(warningJSON), &schedule.WarningMinutes); err != nil {
		return RestartSchedule{}, fmt.Errorf("parse warning minutes: %w", err)
	}
	schedule.Enabled = enabled == 1
	schedule.BackupBeforeShutdown = backupBeforeShutdown == 1
	schedule.SkipIfPlayersOnline = skipIfPlayersOnline == 1
	return schedule, nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
