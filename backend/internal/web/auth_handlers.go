package web

import (
	"errors"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/auth"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{3,32}$`)

type setupStatusResponse struct {
	Initialized bool `json:"initialized"`
}

type userResponse struct {
	User auth.PublicUser `json:"user"`
}

type okResponse struct {
	OK bool `json:"ok"`
}

type setupAdminRequest struct {
	Username        string `json:"username"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirmPassword"`
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *server) handleSetupStatus(w http.ResponseWriter, r *http.Request) {
	initialized, err := s.store.AdminExists(r.Context())
	if err != nil {
		s.logger.Error("failed to check setup status", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "服务器内部错误")
		return
	}
	writeJSON(w, http.StatusOK, setupStatusResponse{Initialized: initialized})
}

func (s *server) handleSetupAdmin(w http.ResponseWriter, r *http.Request) {
	initialized, err := s.store.AdminExists(r.Context())
	if err != nil {
		s.logger.Error("failed to check setup status", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "服务器内部错误")
		return
	}
	if initialized {
		writeError(w, http.StatusConflict, "already_initialized", "面板已经完成初始化")
		return
	}

	var request setupAdminRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	request.Username = strings.TrimSpace(request.Username)
	if !validateUsername(w, request.Username) || !validatePassword(w, request.Password) {
		return
	}
	if request.Password != request.ConfirmPassword {
		writeError(w, http.StatusBadRequest, "password_mismatch", "两次输入的密码不一致")
		return
	}

	passwordHash, err := auth.HashPassword(request.Password)
	if err != nil {
		s.logger.Error("failed to hash setup password", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "服务器内部错误")
		return
	}
	token, tokenHash, expiresAt, ok := s.newSessionValues(w)
	if !ok {
		return
	}

	user, _, err := s.store.CreateFirstAdminWithSession(r.Context(), storage.CreateFirstAdminParams{
		Username:     request.Username,
		PasswordHash: passwordHash,
		TokenHash:    tokenHash,
		ExpiresAt:    expiresAt,
		IPAddress:    remoteIP(r),
		UserAgent:    userAgent(r),
	})
	if err != nil {
		if errors.Is(err, storage.ErrAlreadyInitialized) || errors.Is(err, storage.ErrConflict) {
			writeError(w, http.StatusConflict, "already_initialized", "面板已经完成初始化")
			return
		}
		s.logger.Error("failed to create first admin", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "服务器内部错误")
		return
	}
	if _, err := s.store.EnsureDefaultInstance(r.Context(), storage.EnsureDefaultInstanceParams{
		ID:       s.config.DefaultInstanceID,
		DriverID: s.config.DefaultDriverID,
		Name:     "Stardew Valley",
		DataDir:  filepath.Join(s.config.DataDir, "instances", s.config.DefaultInstanceID),
	}); err != nil {
		s.logger.Error("failed to ensure setup instance", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "服务器内部错误")
		return
	}
	if _, err := s.store.SetInstanceState(r.Context(), storage.SetInstanceStateParams{
		InstanceID:   s.config.DefaultInstanceID,
		DriverID:     s.config.DefaultDriverID,
		State:        storage.InstanceStateAdminCreated,
		StateMessage: "管理员已创建，等待后续 Junimo 准备流程。",
		UpdatedBy:    user.ID,
		AllowNoop:    true,
	}); err != nil {
		s.logger.Error("failed to update setup instance state", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "服务器内部错误")
		return
	}
	if _, err := s.store.UpdateInstanceState(r.Context(), storage.UpdateInstanceStateParams{
		ID:            s.config.DefaultInstanceID,
		State:         storage.InstanceStateAdminCreated,
		StateMessage:  "管理员已创建，等待后续 Junimo 准备流程。",
		DriverPhase:   storage.DefaultDriverPhase,
		DriverPayload: "{}",
	}); err != nil {
		s.logger.Error("failed to update setup instance", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "服务器内部错误")
		return
	}
	if err := s.store.CreateAuditLog(r.Context(), storage.AuditLogParams{
		ActorUserID: &user.ID,
		Action:      "instance_state_updated",
		TargetType:  "instance",
		TargetID:    s.config.DefaultInstanceID,
		Metadata:    `{"state":"admin_created"}`,
		IPAddress:   remoteIP(r),
		UserAgent:   userAgent(r),
	}); err != nil {
		s.logger.Error("failed to write instance state audit", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "服务器内部错误")
		return
	}

	http.SetCookie(w, auth.SessionCookie(token, expiresAt, isSecureRequest(r)))
	writeJSON(w, http.StatusOK, userResponse{User: user.Public()})
}

func (s *server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var request loginRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	request.Username = strings.TrimSpace(request.Username)

	user, err := s.store.GetUserByUsername(r.Context(), request.Username)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, "invalid_credentials", "用户名或密码错误")
			return
		}
		s.logger.Error("failed to load login user", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "服务器内部错误")
		return
	}

	matched, err := auth.VerifyPassword(request.Password, user.PasswordHash)
	if err != nil || !matched {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "用户名或密码错误")
		return
	}

	token, tokenHash, expiresAt, ok := s.newSessionValues(w)
	if !ok {
		return
	}
	if _, err := s.store.CreateSession(r.Context(), storage.CreateSessionParams{
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
		IPAddress: remoteIP(r),
		UserAgent: userAgent(r),
	}); err != nil {
		s.logger.Error("failed to create login session", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "服务器内部错误")
		return
	}
	if err := s.store.MarkUserLoggedIn(r.Context(), user.ID); err != nil {
		s.logger.Error("failed to update last login", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "服务器内部错误")
		return
	}
	if err := s.store.CreateAuditLog(r.Context(), storage.AuditLogParams{
		ActorUserID: &user.ID,
		Action:      "auth_login",
		TargetType:  "user",
		TargetID:    int64String(user.ID),
		Metadata:    "{}",
		IPAddress:   remoteIP(r),
		UserAgent:   userAgent(r),
	}); err != nil {
		s.logger.Error("failed to write login audit", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "服务器内部错误")
		return
	}

	http.SetCookie(w, auth.SessionCookie(token, expiresAt, isSecureRequest(r)))
	writeJSON(w, http.StatusOK, userResponse{User: user.Public()})
}

func (s *server) handleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(auth.SessionCookieName)
	if err == nil && cookie.Value != "" {
		tokenHash := auth.HashSessionToken(s.config.Secret, cookie.Value)
		if session, ok := s.requireAuthNoWrite(r, tokenHash); ok {
			actorID := session.User.ID
			if err := s.store.CreateAuditLog(r.Context(), storage.AuditLogParams{
				ActorUserID: &actorID,
				Action:      "auth_logout",
				TargetType:  "user",
				TargetID:    int64String(actorID),
				Metadata:    "{}",
				IPAddress:   remoteIP(r),
				UserAgent:   userAgent(r),
			}); err != nil {
				s.logger.Error("failed to write logout audit", "error", err)
			}
		}
		if err := s.store.RevokeSessionByTokenHash(r.Context(), tokenHash); err != nil {
			s.logger.Error("failed to revoke logout session", "error", err)
		}
	}

	http.SetCookie(w, auth.ClearSessionCookie(isSecureRequest(r)))
	writeJSON(w, http.StatusOK, okResponse{OK: true})
}

func (s *server) handleMe(w http.ResponseWriter, r *http.Request) {
	session, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, userResponse{User: session.User.Public()})
}

func (s *server) requireAuthNoWrite(r *http.Request, tokenHash string) (currentSession, bool) {
	session, err := s.store.GetSessionByTokenHash(r.Context(), tokenHash)
	if err != nil {
		return currentSession{}, false
	}
	return currentSession{User: session.User, TokenHash: tokenHash}, true
}

func (s *server) newSessionValues(w http.ResponseWriter) (string, string, time.Time, bool) {
	token, err := auth.NewSessionToken()
	if err != nil {
		s.logger.Error("failed to create session token", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "服务器内部错误")
		return "", "", time.Time{}, false
	}
	expiresAt := time.Now().UTC().Add(auth.SessionTTL)
	return token, auth.HashSessionToken(s.config.Secret, token), expiresAt, true
}

func validateUsername(w http.ResponseWriter, username string) bool {
	if !usernamePattern.MatchString(username) {
		writeError(w, http.StatusBadRequest, "invalid_username", "用户名需要 3-32 位，只能包含字母、数字、下划线、点或短横线")
		return false
	}
	return true
}

func validatePassword(w http.ResponseWriter, password string) bool {
	if len(password) < 6 {
		writeError(w, http.StatusBadRequest, "invalid_password", "密码至少需要 6 位")
		return false
	}
	return true
}

func int64String(value int64) string {
	return strconv.FormatInt(value, 10)
}
