package web

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

const serviceName = "stardew-anxi-panel"

// Deps contains the dependencies required by the HTTP layer.
type Deps struct {
	Config config.Config
	Store  *storage.Store
	Logger *slog.Logger
}

type server struct {
	config config.Config
	store  *storage.Store
	logger *slog.Logger
}

// NewHandler returns the HTTP routes for the panel backend.
func NewHandler(deps Deps) http.Handler {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}

	s := &server{
		config: deps.Config,
		store:  deps.Store,
		logger: logger,
	}

	return recoverMiddleware(logger, requestLogMiddleware(logger, s))
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !s.isSetupAllowed(r) {
		initialized, err := s.store.AdminExists(r.Context())
		if err != nil {
			s.logger.Error("failed to check setup status", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		if !initialized {
			writeError(w, http.StatusServiceUnavailable, "setup_required", "setup is required before using the panel")
			return
		}
	}

	s.route(w, r)
}

func (s *server) route(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/health":
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleHealth(w, r)
	case "/api/setup/status":
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleSetupStatus(w, r)
	case "/api/setup/admin":
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleSetupAdmin(w, r)
	case "/api/auth/login":
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleLogin(w, r)
	case "/api/auth/logout":
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleLogout(w, r)
	case "/api/auth/me":
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleMe(w, r)
	case "/api/users":
		s.handleUsers(w, r)
	default:
		if strings.HasPrefix(r.URL.Path, "/api/users/") {
			s.handleUserByID(w, r)
			return
		}
		writeError(w, http.StatusNotFound, "not_found", "resource not found")
	}
}

func (s *server) isSetupAllowed(r *http.Request) bool {
	return r.URL.Path == "/health" || r.URL.Path == "/api/setup/status" || r.URL.Path == "/api/setup/admin"
}

type healthResponse struct {
	Status   string         `json:"status"`
	Service  string         `json:"service"`
	Version  string         `json:"version"`
	Database healthDatabase `json:"database"`
}

type healthDatabase struct {
	Status string `json:"status"`
}

func (s *server) handleHealth(w http.ResponseWriter, r *http.Request) {
	statusCode := http.StatusOK
	response := healthResponse{
		Status:  "ok",
		Service: serviceName,
		Version: s.config.Version,
		Database: healthDatabase{
			Status: "ok",
		},
	}

	if err := s.store.Ping(r.Context()); err != nil {
		statusCode = http.StatusServiceUnavailable
		response.Status = "degraded"
		response.Database.Status = "error"
		s.logger.Error("database health check failed", "error", err)
	}

	writeJSON(w, statusCode, response)
}

type errorResponse struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, statusCode int, code string, message string) {
	writeJSON(w, statusCode, errorResponse{
		Error: errorBody{
			Code:    code,
			Message: message,
		},
	})
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		slog.Default().Error("failed to write json response", "error", err)
	}
}

func requestLogMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		logger.Info("http request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start).String())
	})
}

func recoverMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if value := recover(); value != nil {
				logger.Error("http panic recovered", "panic", value, "method", r.Method, "path", r.URL.Path)
				writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
