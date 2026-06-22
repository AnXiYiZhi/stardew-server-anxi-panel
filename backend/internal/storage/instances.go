package storage

import (
	"context"
	"database/sql"
	"errors"
)

const DefaultDriverPhase = "empty"

// Instance is the persistent model used by the instance-based API and drivers.
type Instance struct {
	ID            string
	DriverID      string
	Name          string
	DataDir       string
	State         string
	StateMessage  sql.NullString
	DriverPhase   string
	DriverPayload string
	CreatedAt     string
	UpdatedAt     string
}

type EnsureDefaultInstanceParams struct {
	ID       string
	DriverID string
	Name     string
	DataDir  string
}

type UpdateInstanceStateParams struct {
	ID            string
	State         string
	StateMessage  string
	DriverPhase   string
	DriverPayload string
}

func (s *Store) EnsureDefaultInstance(ctx context.Context, params EnsureDefaultInstanceParams) (Instance, error) {
	id := params.ID
	if id == "" {
		id = DefaultInstanceID
	}
	driverID := params.DriverID
	if driverID == "" {
		driverID = DefaultDriverID
	}
	name := params.Name
	if name == "" {
		name = "Stardew Valley"
	}

	instance, err := s.GetInstance(ctx, id)
	if err == nil {
		return instance, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return Instance{}, err
	}

	state := InstanceStateUninitialized
	message := "面板尚未初始化管理员。"
	legacyState, err := s.GetInstanceState(ctx, id)
	if err == nil {
		state = legacyState.State
		message = legacyState.StateMessage.String
	} else if errors.Is(err, ErrNotFound) {
		adminExists, err := s.AdminExists(ctx)
		if err != nil {
			return Instance{}, err
		}
		if adminExists {
			state = InstanceStateAdminCreated
			message = "管理员已创建，等待后续 Junimo 准备流程。"
		}
	} else {
		return Instance{}, err
	}

	row := s.db.QueryRowContext(ctx, `
		INSERT INTO instances (id, driver_id, name, data_dir, state, state_message, driver_phase, driver_payload)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id, driver_id, name, data_dir, state, state_message, driver_phase, driver_payload, created_at, updated_at
	`, id, driverID, name, params.DataDir, state, nullString(message), DefaultDriverPhase, "{}")
	return scanInstanceRow(row)
}

func (s *Store) ListInstances(ctx context.Context) ([]Instance, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, driver_id, name, data_dir, state, state_message, driver_phase, driver_payload, created_at, updated_at
		FROM instances
		ORDER BY created_at ASC, id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	instances := make([]Instance, 0)
	for rows.Next() {
		instance, err := scanInstance(rows)
		if err != nil {
			return nil, err
		}
		instances = append(instances, instance)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return instances, nil
}

func (s *Store) GetInstance(ctx context.Context, id string) (Instance, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, driver_id, name, data_dir, state, state_message, driver_phase, driver_payload, created_at, updated_at
		FROM instances
		WHERE id = ?
	`, id)
	return scanInstanceRow(row)
}

func (s *Store) UpdateInstanceState(ctx context.Context, params UpdateInstanceStateParams) (Instance, error) {
	if !IsValidInstanceState(params.State) {
		return Instance{}, ErrInvalidStateTransition
	}
	driverPhase := params.DriverPhase
	if driverPhase == "" {
		driverPhase = DefaultDriverPhase
	}
	driverPayload := params.DriverPayload
	if driverPayload == "" {
		driverPayload = "{}"
	}
	row := s.db.QueryRowContext(ctx, `
		UPDATE instances
		SET state = ?, state_message = ?, driver_phase = ?, driver_payload = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		WHERE id = ?
		RETURNING id, driver_id, name, data_dir, state, state_message, driver_phase, driver_payload, created_at, updated_at
	`, params.State, nullString(params.StateMessage), driverPhase, driverPayload, params.ID)
	return scanInstanceRow(row)
}

func scanInstanceRow(row *sql.Row) (Instance, error) {
	var instance Instance
	if err := row.Scan(
		&instance.ID,
		&instance.DriverID,
		&instance.Name,
		&instance.DataDir,
		&instance.State,
		&instance.StateMessage,
		&instance.DriverPhase,
		&instance.DriverPayload,
		&instance.CreatedAt,
		&instance.UpdatedAt,
	); err != nil {
		return Instance{}, mapScanErr(err, "scan instance")
	}
	return instance, nil
}

func scanInstance(row interface {
	Scan(dest ...any) error
}) (Instance, error) {
	var instance Instance
	if err := row.Scan(
		&instance.ID,
		&instance.DriverID,
		&instance.Name,
		&instance.DataDir,
		&instance.State,
		&instance.StateMessage,
		&instance.DriverPhase,
		&instance.DriverPayload,
		&instance.CreatedAt,
		&instance.UpdatedAt,
	); err != nil {
		return Instance{}, mapScanErr(err, "scan instance")
	}
	return instance, nil
}
