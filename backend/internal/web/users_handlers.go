package web

import (
	"errors"
	"net/http"
	"strings"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/auth"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

type panelUser struct {
	ID          int64   `json:"id"`
	Username    string  `json:"username"`
	Role        string  `json:"role"`
	IsActive    bool    `json:"isActive"`
	CreatedAt   string  `json:"createdAt"`
	UpdatedAt   string  `json:"updatedAt"`
	LastLoginAt *string `json:"lastLoginAt"`
}

type listUsersResponse struct {
	Users []panelUser `json:"users"`
}

type createUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

type updateUserRequest struct {
	Role     *string `json:"role"`
	IsActive *bool   `json:"isActive"`
	Password *string `json:"password"`
}

func (s *server) handleUsers(w http.ResponseWriter, r *http.Request) {
	session, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.handleListUsers(w, r)
	case http.MethodPost:
		s.handleCreateUser(w, r, session)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (s *server) handleUserByID(w http.ResponseWriter, r *http.Request) {
	session, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	id, ok := parseUserID(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "resource not found")
		return
	}

	switch r.Method {
	case http.MethodPatch:
		s.handleUpdateUser(w, r, session, id)
	case http.MethodDelete:
		s.handleDeleteUser(w, r, session, id)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (s *server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.ListUsers(r.Context())
	if err != nil {
		s.logger.Error("failed to list users", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "服务器内部错误")
		return
	}

	response := listUsersResponse{Users: make([]panelUser, 0, len(users))}
	for _, user := range users {
		response.Users = append(response.Users, toPanelUser(user))
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *server) handleCreateUser(w http.ResponseWriter, r *http.Request, session currentSession) {
	var request createUserRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	request.Username = strings.TrimSpace(request.Username)
	request.Role = strings.TrimSpace(request.Role)
	if !validateUsername(w, request.Username) || !validatePassword(w, request.Password) {
		return
	}
	if !auth.IsValidRole(request.Role) {
		writeError(w, http.StatusBadRequest, "invalid_role", "角色只能是 admin 或 user")
		return
	}

	passwordHash, err := auth.HashPassword(request.Password)
	if err != nil {
		s.logger.Error("failed to hash new user password", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "服务器内部错误")
		return
	}

	user, err := s.store.CreateUser(r.Context(), storage.CreateUserParams{
		Username:     request.Username,
		PasswordHash: passwordHash,
		Role:         request.Role,
	})
	if err != nil {
		if errors.Is(err, storage.ErrConflict) {
			writeError(w, http.StatusConflict, "username_taken", "用户名已被占用")
			return
		}
		s.logger.Error("failed to create user", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "服务器内部错误")
		return
	}

	actorID := session.User.ID
	if err := s.store.CreateAuditLog(r.Context(), storage.AuditLogParams{
		ActorUserID: &actorID,
		Action:      "user_created",
		TargetType:  "user",
		TargetID:    int64String(user.ID),
		Metadata:    `{"role":"` + user.Role + `"}`,
		IPAddress:   remoteIP(r),
		UserAgent:   userAgent(r),
	}); err != nil {
		s.logger.Error("failed to write user create audit", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "服务器内部错误")
		return
	}

	writeJSON(w, http.StatusCreated, struct {
		User panelUser `json:"user"`
	}{User: toPanelUser(user)})
}

func (s *server) handleUpdateUser(w http.ResponseWriter, r *http.Request, session currentSession, id int64) {
	var request updateUserRequest
	if !decodeJSON(w, r, &request) {
		return
	}

	params := storage.UpdateUserParams{}
	if request.Role != nil {
		role := strings.TrimSpace(*request.Role)
		if !auth.IsValidRole(role) {
			writeError(w, http.StatusBadRequest, "invalid_role", "角色只能是 admin 或 user")
			return
		}
		params.Role = &role
	}
	if request.IsActive != nil {
		params.IsActive = request.IsActive
	}
	if request.Password != nil {
		if !validatePassword(w, *request.Password) {
			return
		}
		passwordHash, err := auth.HashPassword(*request.Password)
		if err != nil {
			s.logger.Error("failed to hash updated password", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "服务器内部错误")
			return
		}
		params.PasswordHash = &passwordHash
	}

	if params.Role == nil && params.IsActive == nil && params.PasswordHash == nil {
		writeError(w, http.StatusBadRequest, "empty_update", "至少需要提供一个要修改的用户字段")
		return
	}

	user, err := s.store.UpdateUser(r.Context(), session.User.ID, id, params)
	if err != nil {
		if handleUserMutationError(w, err) {
			return
		}
		s.logger.Error("failed to update user", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "服务器内部错误")
		return
	}

	actorID := session.User.ID
	if err := s.store.CreateAuditLog(r.Context(), storage.AuditLogParams{
		ActorUserID: &actorID,
		Action:      auditActionForUpdate(params),
		TargetType:  "user",
		TargetID:    int64String(user.ID),
		Metadata:    metadataForUpdate(params, user),
		IPAddress:   remoteIP(r),
		UserAgent:   userAgent(r),
	}); err != nil {
		s.logger.Error("failed to write user update audit", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "服务器内部错误")
		return
	}

	writeJSON(w, http.StatusOK, struct {
		User panelUser `json:"user"`
	}{User: toPanelUser(user)})
}

func (s *server) handleDeleteUser(w http.ResponseWriter, r *http.Request, session currentSession, id int64) {
	actorID := session.User.ID
	if r.URL.Query().Get("hard") == "true" {
		if err := s.store.DeleteUser(r.Context(), session.User.ID, id); err != nil {
			if handleUserMutationError(w, err) {
				return
			}
			s.logger.Error("failed to delete user", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "服务器内部错误")
			return
		}
		if err := s.store.CreateAuditLog(r.Context(), storage.AuditLogParams{
			ActorUserID: &actorID,
			Action:      "user_deleted",
			TargetType:  "user",
			TargetID:    int64String(id),
			Metadata:    "{}",
			IPAddress:   remoteIP(r),
			UserAgent:   userAgent(r),
		}); err != nil {
			s.logger.Error("failed to write user delete audit", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "服务器内部错误")
			return
		}
		writeJSON(w, http.StatusOK, okResponse{OK: true})
		return
	}

	if err := s.store.DisableUser(r.Context(), session.User.ID, id); err != nil {
		if handleUserMutationError(w, err) {
			return
		}
		s.logger.Error("failed to disable user", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "服务器内部错误")
		return
	}

	if err := s.store.CreateAuditLog(r.Context(), storage.AuditLogParams{
		ActorUserID: &actorID,
		Action:      "user_disabled",
		TargetType:  "user",
		TargetID:    int64String(id),
		Metadata:    "{}",
		IPAddress:   remoteIP(r),
		UserAgent:   userAgent(r),
	}); err != nil {
		s.logger.Error("failed to write user disable audit", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "服务器内部错误")
		return
	}

	writeJSON(w, http.StatusOK, okResponse{OK: true})
}

func handleUserMutationError(w http.ResponseWriter, err error) bool {
	switch {
	case errors.Is(err, storage.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "用户不存在")
	case errors.Is(err, storage.ErrConflict):
		writeError(w, http.StatusConflict, "username_taken", "用户名已被占用")
	case errors.Is(err, storage.ErrLastAdmin):
		writeError(w, http.StatusConflict, "last_admin", "不能移除或降级最后一个启用的管理员")
	case errors.Is(err, storage.ErrSelfDisable):
		writeError(w, http.StatusBadRequest, "self_disable", "不能禁用或删除当前登录用户")
	default:
		return false
	}
	return true
}

func toPanelUser(user storage.User) panelUser {
	var lastLoginAt *string
	if user.LastLoginAt.Valid {
		value := user.LastLoginAt.String
		lastLoginAt = &value
	}
	return panelUser{
		ID:          user.ID,
		Username:    user.Username,
		Role:        user.Role,
		IsActive:    user.IsActive,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
		LastLoginAt: lastLoginAt,
	}
}

func auditActionForUpdate(params storage.UpdateUserParams) string {
	if params.PasswordHash != nil && params.Role == nil && params.IsActive == nil {
		return "user_password_changed"
	}
	return "user_updated"
}

func metadataForUpdate(params storage.UpdateUserParams, user storage.User) string {
	changes := make([]string, 0, 3)
	if params.Role != nil {
		changes = append(changes, "role")
	}
	if params.IsActive != nil {
		changes = append(changes, "is_active")
	}
	if params.PasswordHash != nil {
		changes = append(changes, "password")
	}
	return `{"role":"` + user.Role + `","changed":"` + strings.Join(changes, ",") + `"}`
}
