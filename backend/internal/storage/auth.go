package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/auth"
)

var (
	ErrNotFound           = errors.New("not found")
	ErrConflict           = errors.New("conflict")
	ErrAlreadyInitialized = errors.New("already initialized")
	ErrLastAdmin          = errors.New("last active admin")
	ErrLastSuperAdmin     = errors.New("last active super admin")
	ErrSelfDisable        = errors.New("cannot disable current user")
	ErrSuperAdminRequired = errors.New("super admin required")
)

type User struct {
	ID           int64
	Username     string
	PasswordHash string
	Role         string
	IsSuperAdmin bool
	IsActive     bool
	CreatedAt    string
	UpdatedAt    string
	LastLoginAt  sql.NullString
}

func (u User) Public() auth.PublicUser {
	return auth.PublicUser{
		ID:           u.ID,
		Username:     u.Username,
		Role:         u.Role,
		IsSuperAdmin: u.IsSuperAdmin,
	}
}

type Session struct {
	ID        int64
	UserID    int64
	TokenHash string
	ExpiresAt time.Time
}

type SessionWithUser struct {
	Session Session
	User    User
}

type CreateFirstAdminParams struct {
	Username     string
	PasswordHash string
	TokenHash    string
	ExpiresAt    time.Time
	IPAddress    string
	UserAgent    string
}

type CreateUserParams struct {
	Username     string
	PasswordHash string
	Role         string
	IsSuperAdmin bool
}

type UpdateUserParams struct {
	Role         *string
	IsActive     *bool
	PasswordHash *string
}

type CreateSessionParams struct {
	UserID    int64
	TokenHash string
	ExpiresAt time.Time
	IPAddress string
	UserAgent string
}

type AuditLogParams struct {
	ActorUserID *int64
	Action      string
	TargetType  string
	TargetID    string
	Metadata    string
	IPAddress   string
	UserAgent   string
}

func (s *Store) AdminExists(ctx context.Context) (bool, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role = ? AND is_active = 1`, auth.RoleAdmin).Scan(&count); err != nil {
		return false, fmt.Errorf("count admins: %w", err)
	}
	return count > 0, nil
}

func (s *Store) CreateFirstAdminWithSession(ctx context.Context, params CreateFirstAdminParams) (User, Session, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, Session{}, fmt.Errorf("begin setup transaction: %w", err)
	}
	defer rollback(tx)

	adminExists, err := adminExistsTx(ctx, tx)
	if err != nil {
		return User{}, Session{}, err
	}
	if adminExists {
		return User{}, Session{}, ErrAlreadyInitialized
	}

	user, err := createUserTx(ctx, tx, CreateUserParams{
		Username:     params.Username,
		PasswordHash: params.PasswordHash,
		Role:         auth.RoleAdmin,
		IsSuperAdmin: true,
	})
	if err != nil {
		return User{}, Session{}, err
	}

	session, err := createSessionTx(ctx, tx, CreateSessionParams{
		UserID:    user.ID,
		TokenHash: params.TokenHash,
		ExpiresAt: params.ExpiresAt,
		IPAddress: params.IPAddress,
		UserAgent: params.UserAgent,
	})
	if err != nil {
		return User{}, Session{}, err
	}

	if err := createAuditLogTx(ctx, tx, AuditLogParams{
		ActorUserID: &user.ID,
		Action:      "setup_admin_created",
		TargetType:  "user",
		TargetID:    fmt.Sprint(user.ID),
		Metadata:    `{"role":"admin"}`,
		IPAddress:   params.IPAddress,
		UserAgent:   params.UserAgent,
	}); err != nil {
		return User{}, Session{}, err
	}

	if err := tx.Commit(); err != nil {
		return User{}, Session{}, fmt.Errorf("commit setup transaction: %w", err)
	}
	return user, session, nil
}

func (s *Store) CreateUser(ctx context.Context, params CreateUserParams) (User, error) {
	return createUser(ctx, s.db, params)
}

func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, username, password_hash, role, is_super_admin, is_active, created_at, updated_at, last_login_at
		FROM users
		ORDER BY id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list users rows: %w", err)
	}
	return users, nil
}

func (s *Store) GetUserByID(ctx context.Context, id int64) (User, error) {
	return getUserByID(ctx, s.db, id)
}

func (s *Store) GetUserByUsername(ctx context.Context, username string) (User, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, username, password_hash, role, is_super_admin, is_active, created_at, updated_at, last_login_at
		FROM users
		WHERE username = ? AND is_active = 1
	`, username)
	return scanUserRow(row)
}

func (s *Store) UpdateUser(ctx context.Context, actorID int64, targetID int64, params UpdateUserParams) (User, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, fmt.Errorf("begin update user transaction: %w", err)
	}
	defer rollback(tx)

	target, err := getUserByIDTx(ctx, tx, targetID)
	if err != nil {
		return User{}, err
	}
	actor, err := getUserByIDTx(ctx, tx, actorID)
	if err != nil {
		return User{}, err
	}

	if target.Role == auth.RoleAdmin && targetID != actorID && !actor.IsSuperAdmin {
		return User{}, ErrSuperAdminRequired
	}
	if params.Role != nil && !actor.IsSuperAdmin {
		return User{}, ErrSuperAdminRequired
	}

	if params.Role != nil && target.Role == auth.RoleAdmin && *params.Role != auth.RoleAdmin {
		if target.IsSuperAdmin {
			if err := ensureNotLastSuperAdminTx(ctx, tx); err != nil {
				return User{}, err
			}
		}
		if err := ensureNotLastAdminTx(ctx, tx); err != nil {
			return User{}, err
		}
	}
	if params.IsActive != nil && !*params.IsActive {
		if actorID == targetID {
			return User{}, ErrSelfDisable
		}
		if target.Role == auth.RoleAdmin {
			if target.IsSuperAdmin {
				if err := ensureNotLastSuperAdminTx(ctx, tx); err != nil {
					return User{}, err
				}
			}
			if err := ensureNotLastAdminTx(ctx, tx); err != nil {
				return User{}, err
			}
		}
	}

	role := target.Role
	if params.Role != nil {
		role = *params.Role
	}
	isSuperAdmin := target.IsSuperAdmin
	if role != auth.RoleAdmin {
		isSuperAdmin = false
	}
	isActive := target.IsActive
	if params.IsActive != nil {
		isActive = *params.IsActive
	}
	passwordHash := target.PasswordHash
	passwordChanged := false
	if params.PasswordHash != nil {
		passwordHash = *params.PasswordHash
		passwordChanged = true
	}

	row := tx.QueryRowContext(ctx, `
		UPDATE users
		SET role = ?, is_super_admin = ?, is_active = ?, password_hash = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		WHERE id = ?
		RETURNING id, username, password_hash, role, is_super_admin, is_active, created_at, updated_at, last_login_at
	`, role, boolToInt(isSuperAdmin), boolToInt(isActive), passwordHash, targetID)
	updated, err := scanUserRow(row)
	if err != nil {
		return User{}, err
	}

	if !isActive || passwordChanged {
		if _, err := tx.ExecContext(ctx, `
			UPDATE sessions
			SET revoked_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
			WHERE user_id = ? AND revoked_at IS NULL
		`, targetID); err != nil {
			return User{}, fmt.Errorf("revoke user sessions: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return User{}, fmt.Errorf("commit update user transaction: %w", err)
	}
	return updated, nil
}

func (s *Store) DisableUser(ctx context.Context, actorID int64, targetID int64) error {
	inactive := false
	_, err := s.UpdateUser(ctx, actorID, targetID, UpdateUserParams{IsActive: &inactive})
	return err
}

func (s *Store) DeleteUser(ctx context.Context, actorID int64, targetID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete user transaction: %w", err)
	}
	defer rollback(tx)

	target, err := getUserByIDTx(ctx, tx, targetID)
	if err != nil {
		return err
	}
	actor, err := getUserByIDTx(ctx, tx, actorID)
	if err != nil {
		return err
	}
	if actorID == targetID {
		return ErrSelfDisable
	}
	if target.Role == auth.RoleAdmin && !actor.IsSuperAdmin {
		return ErrSuperAdminRequired
	}
	if target.Role == auth.RoleAdmin && target.IsActive {
		if target.IsSuperAdmin {
			if err := ensureNotLastSuperAdminTx(ctx, tx); err != nil {
				return err
			}
		}
		if err := ensureNotLastAdminTx(ctx, tx); err != nil {
			return err
		}
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, targetID); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete user transaction: %w", err)
	}
	return nil
}

func (s *Store) CreateSession(ctx context.Context, params CreateSessionParams) (Session, error) {
	return createSession(ctx, s.db, params)
}

func (s *Store) GetSessionByTokenHash(ctx context.Context, tokenHash string) (SessionWithUser, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT
			s.id, s.user_id, s.token_hash, s.expires_at,
			u.id, u.username, u.password_hash, u.role, u.is_super_admin, u.is_active, u.created_at, u.updated_at, u.last_login_at
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = ?
		  AND s.revoked_at IS NULL
		  AND s.expires_at > strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		  AND u.is_active = 1
	`, tokenHash)

	var result SessionWithUser
	var expiresAt string
	if err := row.Scan(
		&result.Session.ID,
		&result.Session.UserID,
		&result.Session.TokenHash,
		&expiresAt,
		&result.User.ID,
		&result.User.Username,
		&result.User.PasswordHash,
		&result.User.Role,
		&result.User.IsSuperAdmin,
		&result.User.IsActive,
		&result.User.CreatedAt,
		&result.User.UpdatedAt,
		&result.User.LastLoginAt,
	); err != nil {
		return SessionWithUser{}, mapScanErr(err, "get session")
	}
	expires, err := parseDBTime(expiresAt)
	if err != nil {
		return SessionWithUser{}, err
	}
	result.Session.ExpiresAt = expires

	if _, err := s.db.ExecContext(ctx, `
		UPDATE sessions
		SET last_seen_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		WHERE id = ?
	`, result.Session.ID); err != nil {
		return SessionWithUser{}, fmt.Errorf("update session last seen: %w", err)
	}

	return result, nil
}

func (s *Store) RevokeSessionByTokenHash(ctx context.Context, tokenHash string) error {
	if _, err := s.db.ExecContext(ctx, `
		UPDATE sessions
		SET revoked_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		WHERE token_hash = ? AND revoked_at IS NULL
	`, tokenHash); err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}

func (s *Store) RevokeUserSessions(ctx context.Context, userID int64) error {
	if _, err := s.db.ExecContext(ctx, `
		UPDATE sessions
		SET revoked_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		WHERE user_id = ? AND revoked_at IS NULL
	`, userID); err != nil {
		return fmt.Errorf("revoke user sessions: %w", err)
	}
	return nil
}

func (s *Store) MarkUserLoggedIn(ctx context.Context, userID int64) error {
	if _, err := s.db.ExecContext(ctx, `
		UPDATE users
		SET last_login_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now'),
		    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		WHERE id = ?
	`, userID); err != nil {
		return fmt.Errorf("mark user logged in: %w", err)
	}
	return nil
}

func (s *Store) CreateAuditLog(ctx context.Context, params AuditLogParams) error {
	return createAuditLog(ctx, s.db, params)
}

// AuditLogEntry represents a single audit log record for API responses.
type AuditLogEntry struct {
	ID           int64   `json:"id"`
	ActorUserID  *int64  `json:"actorUserId"`
	ActorName    *string `json:"actorName"`
	Action       string  `json:"action"`
	TargetType   string  `json:"targetType"`
	TargetID     *string `json:"targetId"`
	MetadataJSON string  `json:"metadataJson"`
	IPAddress    *string `json:"ipAddress"`
	UserAgent    *string `json:"userAgent"`
	CreatedAt    string  `json:"createdAt"`
}

// ListAuditLogsParams controls pagination and filtering for audit log queries.
type ListAuditLogsParams struct {
	Limit  int
	Offset int
}

// ListAuditLogs returns recent audit logs with actor username joined.
func (s *Store) ListAuditLogs(ctx context.Context, params ListAuditLogsParams) ([]AuditLogEntry, int, error) {
	if params.Limit <= 0 || params.Limit > 200 {
		params.Limit = 50
	}
	if params.Offset < 0 {
		params.Offset = 0
	}

	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_logs`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count audit logs: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT a.id, a.actor_user_id, u.username, a.action, a.target_type, a.target_id,
		       a.metadata_json, a.ip_address, a.user_agent, a.created_at
		FROM audit_logs a
		LEFT JOIN users u ON u.id = a.actor_user_id
		ORDER BY a.id DESC
		LIMIT ? OFFSET ?
	`, params.Limit, params.Offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list audit logs: %w", err)
	}
	defer rows.Close()

	var entries []AuditLogEntry
	for rows.Next() {
		var e AuditLogEntry
		if err := rows.Scan(&e.ID, &e.ActorUserID, &e.ActorName, &e.Action, &e.TargetType, &e.TargetID,
			&e.MetadataJSON, &e.IPAddress, &e.UserAgent, &e.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan audit log: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("audit logs rows: %w", err)
	}
	return entries, total, nil
}

func adminExistsTx(ctx context.Context, tx *sql.Tx) (bool, error) {
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role = ? AND is_active = 1`, auth.RoleAdmin).Scan(&count); err != nil {
		return false, fmt.Errorf("count admins: %w", err)
	}
	return count > 0, nil
}

func ensureNotLastAdminTx(ctx context.Context, tx *sql.Tx) error {
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role = ? AND is_active = 1`, auth.RoleAdmin).Scan(&count); err != nil {
		return fmt.Errorf("count admins: %w", err)
	}
	if count <= 1 {
		return ErrLastAdmin
	}
	return nil
}

func ensureNotLastSuperAdminTx(ctx context.Context, tx *sql.Tx) error {
	var count int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM users
		WHERE role = ? AND is_super_admin = 1 AND is_active = 1
	`, auth.RoleAdmin).Scan(&count); err != nil {
		return fmt.Errorf("count super admins: %w", err)
	}
	if count <= 1 {
		return ErrLastSuperAdmin
	}
	return nil
}

func createUser(ctx context.Context, db *sql.DB, params CreateUserParams) (User, error) {
	row := db.QueryRowContext(ctx, `
		INSERT INTO users (username, password_hash, role, is_super_admin)
		VALUES (?, ?, ?, ?)
		RETURNING id, username, password_hash, role, is_super_admin, is_active, created_at, updated_at, last_login_at
	`, params.Username, params.PasswordHash, params.Role, boolToInt(params.IsSuperAdmin))
	return scanUserRow(row)
}

func createUserTx(ctx context.Context, tx *sql.Tx, params CreateUserParams) (User, error) {
	row := tx.QueryRowContext(ctx, `
		INSERT INTO users (username, password_hash, role, is_super_admin)
		VALUES (?, ?, ?, ?)
		RETURNING id, username, password_hash, role, is_super_admin, is_active, created_at, updated_at, last_login_at
	`, params.Username, params.PasswordHash, params.Role, boolToInt(params.IsSuperAdmin))
	return scanUserRow(row)
}

func getUserByID(ctx context.Context, db *sql.DB, id int64) (User, error) {
	row := db.QueryRowContext(ctx, `
		SELECT id, username, password_hash, role, is_super_admin, is_active, created_at, updated_at, last_login_at
		FROM users
		WHERE id = ?
	`, id)
	return scanUserRow(row)
}

func getUserByIDTx(ctx context.Context, tx *sql.Tx, id int64) (User, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT id, username, password_hash, role, is_super_admin, is_active, created_at, updated_at, last_login_at
		FROM users
		WHERE id = ?
	`, id)
	return scanUserRow(row)
}

func createSession(ctx context.Context, db *sql.DB, params CreateSessionParams) (Session, error) {
	row := db.QueryRowContext(ctx, `
		INSERT INTO sessions (user_id, token_hash, expires_at, ip_address, user_agent)
		VALUES (?, ?, ?, ?, ?)
		RETURNING id, user_id, token_hash, expires_at
	`, params.UserID, params.TokenHash, formatDBTime(params.ExpiresAt), nullString(params.IPAddress), nullString(params.UserAgent))
	return scanSessionRow(row)
}

func createSessionTx(ctx context.Context, tx *sql.Tx, params CreateSessionParams) (Session, error) {
	row := tx.QueryRowContext(ctx, `
		INSERT INTO sessions (user_id, token_hash, expires_at, ip_address, user_agent)
		VALUES (?, ?, ?, ?, ?)
		RETURNING id, user_id, token_hash, expires_at
	`, params.UserID, params.TokenHash, formatDBTime(params.ExpiresAt), nullString(params.IPAddress), nullString(params.UserAgent))
	return scanSessionRow(row)
}

func createAuditLog(ctx context.Context, db *sql.DB, params AuditLogParams) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO audit_logs (actor_user_id, action, target_type, target_id, metadata_json, ip_address, user_agent)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, optionalInt64(params.ActorUserID), params.Action, params.TargetType, nullString(params.TargetID), metadataOrEmpty(params.Metadata), nullString(params.IPAddress), nullString(params.UserAgent))
	if err != nil {
		return fmt.Errorf("create audit log: %w", err)
	}
	return nil
}

func createAuditLogTx(ctx context.Context, tx *sql.Tx, params AuditLogParams) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO audit_logs (actor_user_id, action, target_type, target_id, metadata_json, ip_address, user_agent)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, optionalInt64(params.ActorUserID), params.Action, params.TargetType, nullString(params.TargetID), metadataOrEmpty(params.Metadata), nullString(params.IPAddress), nullString(params.UserAgent))
	if err != nil {
		return fmt.Errorf("create audit log: %w", err)
	}
	return nil
}

func scanUserRow(row *sql.Row) (User, error) {
	var user User
	if err := row.Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.IsSuperAdmin, &user.IsActive, &user.CreatedAt, &user.UpdatedAt, &user.LastLoginAt); err != nil {
		return User{}, mapScanErr(err, "scan user")
	}
	return user, nil
}

func scanUser(rows *sql.Rows) (User, error) {
	var user User
	if err := rows.Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.IsSuperAdmin, &user.IsActive, &user.CreatedAt, &user.UpdatedAt, &user.LastLoginAt); err != nil {
		return User{}, fmt.Errorf("scan user: %w", err)
	}
	return user, nil
}

func scanSessionRow(row *sql.Row) (Session, error) {
	var session Session
	var expiresAt string
	if err := row.Scan(&session.ID, &session.UserID, &session.TokenHash, &expiresAt); err != nil {
		return Session{}, mapScanErr(err, "scan session")
	}
	expires, err := parseDBTime(expiresAt)
	if err != nil {
		return Session{}, err
	}
	session.ExpiresAt = expires
	return session, nil
}

func mapScanErr(err error, op string) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if isSQLiteConstraint(err) {
		return ErrConflict
	}
	return fmt.Errorf("%s: %w", op, err)
}

func isSQLiteConstraint(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "constraint")
}

func rollback(tx *sql.Tx) {
	_ = tx.Rollback()
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func optionalInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func metadataOrEmpty(value string) string {
	if value == "" {
		return "{}"
	}
	return value
}

func formatDBTime(value time.Time) string {
	return value.UTC().Format("2006-01-02T15:04:05.000Z")
}

func parseDBTime(value string) (time.Time, error) {
	parsed, err := time.Parse("2006-01-02T15:04:05.000Z", value)
	if err == nil {
		return parsed, nil
	}
	parsed, err = time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse database time: %w", err)
	}
	return parsed, nil
}
