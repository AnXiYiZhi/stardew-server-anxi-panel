package storage

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
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

type CreateInstanceParams struct {
	ID            string
	DriverID      string
	Name          string
	DataDir       string
	State         string
	StateMessage  string
	DriverPhase   string
	DriverPayload string
}

type UpdateInstanceStateParams struct {
	ID            string
	State         string
	StateMessage  string
	DriverPhase   string
	DriverPayload string
}

type UpdateInstanceStateForActiveJobParams struct {
	UpdateInstanceStateParams
	JobID string
}

var ErrJobNotActive = errors.New("job does not own active instance state")

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

// AllocateInstanceOrdinal returns a monotonically increasing, per-game
// ordinal for a backend-generated instance ID. Existing instances seed the
// first value for upgraded databases, and allocated values are intentionally
// never returned when provisioning fails or an instance is deleted.
func (s *Store) AllocateInstanceOrdinal(ctx context.Context, gameID, driverID, idPrefix string) (int, error) {
	gameID = strings.TrimSpace(gameID)
	driverID = strings.TrimSpace(driverID)
	idPrefix = strings.TrimSpace(idPrefix)
	if gameID == "" || driverID == "" || idPrefix == "" {
		return 0, ErrConflict
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer rollback(tx)

	initialNext := 2
	rows, err := tx.QueryContext(ctx, `SELECT id FROM instances WHERE driver_id = ?`, driverID)
	if err != nil {
		return 0, err
	}
	instanceCount := 0
	prefix := idPrefix + "-"
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return 0, err
		}
		instanceCount++
		if !strings.HasPrefix(id, prefix) {
			continue
		}
		value, err := strconv.Atoi(strings.TrimPrefix(id, prefix))
		if err == nil && value >= initialNext {
			initialNext = value + 1
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, err
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	if countNext := instanceCount + 1; countNext > initialNext {
		initialNext = countNext
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO instance_id_sequences (game_id, next_value)
		VALUES (?, ?)
		ON CONFLICT(game_id) DO NOTHING
	`, gameID, initialNext); err != nil {
		return 0, err
	}
	var ordinal int
	if err := tx.QueryRowContext(ctx, `
		UPDATE instance_id_sequences
		SET next_value = next_value + 1,
			updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		WHERE game_id = ?
		RETURNING next_value - 1
	`, gameID).Scan(&ordinal); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return ordinal, nil
}

// CreateInstance reserves a new instance ID and data directory in one SQLite
// write transaction. data_dir is checked explicitly because older databases do
// not have a UNIQUE constraint on that column.
func (s *Store) CreateInstance(ctx context.Context, params CreateInstanceParams) (Instance, error) {
	if params.ID == "" || params.DriverID == "" || params.Name == "" || params.DataDir == "" || !IsValidInstanceState(params.State) {
		return Instance{}, ErrConflict
	}
	driverPhase := params.DriverPhase
	if driverPhase == "" {
		driverPhase = DefaultDriverPhase
	}
	driverPayload := params.DriverPayload
	if driverPayload == "" {
		driverPayload = "{}"
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Instance{}, err
	}
	defer rollback(tx)
	var conflict int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM instances WHERE id = ? OR data_dir = ?
	`, params.ID, params.DataDir).Scan(&conflict); err != nil {
		return Instance{}, err
	}
	if conflict != 0 {
		return Instance{}, ErrConflict
	}
	row := tx.QueryRowContext(ctx, `
		INSERT INTO instances (id, driver_id, name, data_dir, state, state_message, driver_phase, driver_payload)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id, driver_id, name, data_dir, state, state_message, driver_phase, driver_payload, created_at, updated_at
	`, params.ID, params.DriverID, params.Name, params.DataDir, params.State, nullString(params.StateMessage), driverPhase, driverPayload)
	instance, err := scanInstanceRow(row)
	if err != nil {
		return Instance{}, err
	}
	if err := tx.Commit(); err != nil {
		return Instance{}, err
	}
	return instance, nil
}

// DeleteInstanceIfPhase removes only a still-owned provisioning reservation.
// It cannot delete a completed or independently mutated instance.
func (s *Store) DeleteInstanceIfPhase(ctx context.Context, id, driverPhase string) (bool, error) {
	result, err := s.db.ExecContext(ctx, `DELETE FROM instances WHERE id = ? AND driver_phase = ?`, id, driverPhase)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count == 1, err
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

func (s *Store) RenameInstance(ctx context.Context, id, name string) (Instance, error) {
	row := s.db.QueryRowContext(ctx, `UPDATE instances SET name = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE id = ?
		RETURNING id, driver_id, name, data_dir, state, state_message, driver_phase, driver_payload, created_at, updated_at`, name, id)
	return scanInstanceRow(row)
}

// RestoreInstanceStateSnapshot restores the four persisted lifecycle fields
// exactly, including NULL versus empty/non-empty state_message semantics and
// the original driver phase/payload string bytes. Unlike UpdateInstanceState,
// this recovery-only contract never substitutes DefaultDriverPhase or "{}"
// for empty values. Callers must pass an authoritative captured Instance.
func (s *Store) RestoreInstanceStateSnapshot(ctx context.Context, snapshot Instance) (Instance, error) {
	if !IsValidInstanceState(snapshot.State) {
		return Instance{}, ErrInvalidStateTransition
	}
	var stateMessage any
	if snapshot.StateMessage.Valid {
		stateMessage = snapshot.StateMessage.String
	}
	row := s.db.QueryRowContext(ctx, `
		UPDATE instances
		SET state = ?, state_message = ?, driver_phase = ?, driver_payload = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		WHERE id = ?
		RETURNING id, driver_id, name, data_dir, state, state_message, driver_phase, driver_payload, created_at, updated_at
	`, snapshot.State, stateMessage, snapshot.DriverPhase, snapshot.DriverPayload, snapshot.ID)
	return scanInstanceRow(row)
}

// UpdateInstanceStateForActiveJob applies an instance transition only while
// the supplied job is queued/running and targets that instance. This turns the
// active job row into a durable write lease and blocks late writes from stale
// or already-terminal runners.
func (s *Store) UpdateInstanceStateForActiveJob(ctx context.Context, params UpdateInstanceStateForActiveJobParams) (Instance, error) {
	if !IsValidInstanceState(params.State) {
		return Instance{}, ErrInvalidStateTransition
	}
	if params.JobID == "" {
		return Instance{}, ErrJobNotActive
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
		  AND EXISTS (
		      SELECT 1
		      FROM jobs
		      WHERE jobs.id = ?
		        AND jobs.target_type = 'instance'
		        AND jobs.target_id = instances.id
		        AND jobs.status IN (?, ?)
		  )
		RETURNING id, driver_id, name, data_dir, state, state_message, driver_phase, driver_payload, created_at, updated_at
	`, params.State, nullString(params.StateMessage), driverPhase, driverPayload, params.ID, params.JobID, JobStatusQueued, JobStatusRunning)
	instance, err := scanInstanceRow(row)
	if errors.Is(err, ErrNotFound) {
		return Instance{}, ErrJobNotActive
	}
	return instance, err
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
