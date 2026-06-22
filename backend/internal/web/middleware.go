package web

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/auth"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

const maxJSONBodyBytes = 1 << 20

type currentSession struct {
	User      storage.User
	TokenHash string
}

func (s *server) requireAuth(w http.ResponseWriter, r *http.Request) (currentSession, bool) {
	cookie, err := r.Cookie(auth.SessionCookieName)
	if err != nil || cookie.Value == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return currentSession{}, false
	}

	tokenHash := auth.HashSessionToken(s.config.Secret, cookie.Value)
	session, err := s.store.GetSessionByTokenHash(r.Context(), tokenHash)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
			return currentSession{}, false
		}
		s.logger.Error("failed to load session", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return currentSession{}, false
	}

	return currentSession{User: session.User, TokenHash: tokenHash}, true
}

func (s *server) requireAdmin(w http.ResponseWriter, r *http.Request) (currentSession, bool) {
	session, ok := s.requireAuth(w, r)
	if !ok {
		return currentSession{}, false
	}
	if session.User.Role != auth.RoleAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "admin access required")
		return currentSession{}, false
	}
	return session, true
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must contain a single JSON object")
		return false
	}
	return true
}

func remoteIP(r *http.Request) string {
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func userAgent(r *http.Request) string {
	return r.UserAgent()
}

func isSecureRequest(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func parseUserID(path string) (int64, bool) {
	value := strings.TrimPrefix(path, "/api/users/")
	if value == "" || strings.Contains(value, "/") {
		return 0, false
	}
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}
