package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

const (
	DefaultInstanceID = "stardew"
	DefaultDriverID   = "stardew_junimo"

	InstanceStateUninitialized       = "uninitialized"
	InstanceStateAdminCreated        = "admin_created"
	InstanceStateJunimoScaffolded    = "junimo_scaffolded"
	InstanceStateCredentialsRequired = "credentials_required"
	InstanceStateSteamAuthRunning    = "steam_auth_running"
	InstanceStateSteamAuthFailed     = "steam_auth_failed"
	InstanceStateSteamAuthDone       = "steam_auth_done"
	InstanceStateGameInstalled       = "game_installed"
	InstanceStateSaveRequired        = "save_required"
	InstanceStateReadyToStart        = "ready_to_start"
	InstanceStateStarting            = "starting"
	InstanceStateRunning             = "running"
	InstanceStateStopped             = "stopped"
	InstanceStateError               = "error"
)

var ErrInvalidStateTransition = errors.New("invalid instance state transition")

type InstanceState struct {
	InstanceID   string
	DriverID     string
	State        string
	StateMessage sql.NullString
	LastJobID    sql.NullString
	UpdatedAt    string
	UpdatedBy    sql.NullInt64
}

type SetInstanceStateParams struct {
	InstanceID   string
	DriverID     string
	State        string
	StateMessage string
	LastJobID    string
	UpdatedBy    int64
	AllowNoop    bool
	SkipValidate bool
}

func (s *Store) EnsureDefaultInstanceState(ctx context.Context) (InstanceState, error) {
	state, err := s.GetInstanceState(ctx, DefaultInstanceID)
	if err == nil {
		return state, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return InstanceState{}, err
	}

	initialState := InstanceStateUninitialized
	message := "面板尚未初始化管理员。"
	adminExists, err := s.AdminExists(ctx)
	if err != nil {
		return InstanceState{}, err
	}
	if adminExists {
		initialState = InstanceStateAdminCreated
		message = "管理员已创建，等待后续 Junimo 准备流程。"
	}

	row := s.db.QueryRowContext(ctx, `
		INSERT INTO instance_state (instance_id, driver_id, state, state_message)
		VALUES (?, ?, ?, ?)
		RETURNING instance_id, driver_id, state, state_message, last_job_id, updated_at, updated_by
	`, DefaultInstanceID, DefaultDriverID, initialState, message)
	return scanInstanceStateRow(row)
}

func (s *Store) GetInstanceState(ctx context.Context, instanceID string) (InstanceState, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT instance_id, driver_id, state, state_message, last_job_id, updated_at, updated_by
		FROM instance_state
		WHERE instance_id = ?
	`, instanceID)
	return scanInstanceStateRow(row)
}

func (s *Store) SetInstanceState(ctx context.Context, params SetInstanceStateParams) (InstanceState, error) {
	instanceID := params.InstanceID
	if instanceID == "" {
		instanceID = DefaultInstanceID
	}
	driverID := params.DriverID
	if driverID == "" {
		driverID = DefaultDriverID
	}
	if !IsValidInstanceState(params.State) {
		return InstanceState{}, ErrInvalidStateTransition
	}

	current, err := s.GetInstanceState(ctx, instanceID)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			return InstanceState{}, err
		}
		row := s.db.QueryRowContext(ctx, `
			INSERT INTO instance_state (instance_id, driver_id, state, state_message, last_job_id, updated_by)
			VALUES (?, ?, ?, ?, ?, ?)
			RETURNING instance_id, driver_id, state, state_message, last_job_id, updated_at, updated_by
		`, instanceID, driverID, params.State, nullString(params.StateMessage), nullString(params.LastJobID), optionalCreatedBy(params.UpdatedBy))
		return scanInstanceStateRow(row)
	}

	if current.State == params.State && params.AllowNoop {
		return current, nil
	}
	if !params.SkipValidate && !CanTransitionInstanceState(current.State, params.State) {
		return InstanceState{}, ErrInvalidStateTransition
	}

	row := s.db.QueryRowContext(ctx, `
		UPDATE instance_state
		SET driver_id = ?, state = ?, state_message = ?, last_job_id = ?, updated_by = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		WHERE instance_id = ?
		RETURNING instance_id, driver_id, state, state_message, last_job_id, updated_at, updated_by
	`, driverID, params.State, nullString(params.StateMessage), nullString(params.LastJobID), optionalCreatedBy(params.UpdatedBy), instanceID)
	return scanInstanceStateRow(row)
}

func IsValidInstanceState(state string) bool {
	switch state {
	case InstanceStateUninitialized,
		InstanceStateAdminCreated,
		InstanceStateJunimoScaffolded,
		InstanceStateCredentialsRequired,
		InstanceStateSteamAuthRunning,
		InstanceStateSteamAuthFailed,
		InstanceStateSteamAuthDone,
		InstanceStateGameInstalled,
		InstanceStateSaveRequired,
		InstanceStateReadyToStart,
		InstanceStateStarting,
		InstanceStateRunning,
		InstanceStateStopped,
		InstanceStateError:
		return true
	default:
		return false
	}
}

func CanTransitionInstanceState(from string, to string) bool {
	if from == to {
		return true
	}
	if !IsValidInstanceState(from) || !IsValidInstanceState(to) {
		return false
	}
	if to == InstanceStateError {
		return true
	}

	allowed := map[string][]string{
		InstanceStateUninitialized:       {InstanceStateAdminCreated},
		InstanceStateAdminCreated:        {InstanceStateJunimoScaffolded},
		InstanceStateJunimoScaffolded:    {InstanceStateCredentialsRequired},
		InstanceStateCredentialsRequired: {InstanceStateSteamAuthRunning},
		InstanceStateSteamAuthRunning:    {InstanceStateSteamAuthFailed, InstanceStateSteamAuthDone, InstanceStateCredentialsRequired},
		InstanceStateSteamAuthFailed:     {InstanceStateCredentialsRequired, InstanceStateSteamAuthRunning},
		InstanceStateSteamAuthDone:       {InstanceStateGameInstalled},
		InstanceStateGameInstalled:       {InstanceStateSaveRequired, InstanceStateReadyToStart},
		InstanceStateSaveRequired:        {InstanceStateReadyToStart},
		InstanceStateReadyToStart:        {InstanceStateStarting},
		InstanceStateStarting:            {InstanceStateRunning, InstanceStateStopped},
		InstanceStateRunning:             {InstanceStateStopped, InstanceStateStarting},
		InstanceStateStopped:             {InstanceStateStarting},
		InstanceStateError: {
			InstanceStateAdminCreated,
			InstanceStateJunimoScaffolded,
			InstanceStateCredentialsRequired,
			InstanceStateGameInstalled,
			InstanceStateSaveRequired,
			InstanceStateReadyToStart,
			InstanceStateStopped,
		},
	}
	for _, candidate := range allowed[from] {
		if candidate == to {
			return true
		}
	}
	return false
}

func scanInstanceStateRow(row *sql.Row) (InstanceState, error) {
	var state InstanceState
	if err := row.Scan(&state.InstanceID, &state.DriverID, &state.State, &state.StateMessage, &state.LastJobID, &state.UpdatedAt, &state.UpdatedBy); err != nil {
		return InstanceState{}, mapScanErr(err, "scan instance state")
	}
	return state, nil
}

func (i InstanceState) String() string {
	return fmt.Sprintf("%s:%s", i.InstanceID, i.State)
}
