package web

import (
	"context"
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/static"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

const serviceName = "stardew-anxi-panel"

// Deps contains the dependencies required by the HTTP layer.
type Deps struct {
	Config   config.Config
	Store    *storage.Store
	Logger   *slog.Logger
	Docker   DockerService
	Jobs     *jobs.Manager
	Registry *registry.Registry
}

type DockerService interface {
	DockerVersion(ctx context.Context, workDir string) (paneldocker.CommandResult, error)
	ComposeVersion(ctx context.Context, workDir string) (paneldocker.CommandResult, error)
	ComposePs(ctx context.Context, dir string) (paneldocker.ComposePsResult, error)
	ComposeStats(ctx context.Context, dir string) (paneldocker.ComposeStatsResult, error)
	ComposeLogs(ctx context.Context, dir string, opts paneldocker.LogsOptions) (paneldocker.CommandResult, error)
}

type server struct {
	config         config.Config
	store          *storage.Store
	logger         *slog.Logger
	docker         DockerService
	jobs           *jobs.Manager
	registry       *registry.Registry
	pendingUploads *pendingUploadStore
}

// NewHandler returns the HTTP routes for the panel backend.
func NewHandler(deps Deps) http.Handler {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}

	dockerClient := deps.Docker
	if dockerClient == nil {
		dockerClient = paneldocker.NewClient(paneldocker.Options{Logger: logger})
	}

	s := &server{
		config:         normalizeConfig(deps.Config),
		store:          deps.Store,
		logger:         logger,
		docker:         dockerClient,
		jobs:           deps.Jobs,
		registry:       deps.Registry,
		pendingUploads: newPendingUploadStore(),
	}
	if s.jobs == nil {
		s.jobs = jobs.NewManager(deps.Store, logger)
	}
	if s.registry == nil {
		s.registry = registry.New()
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
	case "/api/version":
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleVersion(w, r)
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
	case "/api/settings/nexus":
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleNexusSettingsStatus(w, r)
	case "/api/settings/nexus/api-key":
		s.handleNexusAPIKey(w, r)
	case "/api/users":
		s.handleUsers(w, r)
	case "/api/jobs":
		s.handleJobs(w, r)
	case "/api/jobs/error-logs":
		s.handleClearJobErrorLogs(w, r)
	case "/api/jobs/test":
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleTestJob(w, r, false)
	case "/api/jobs/test-fail":
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleTestJob(w, r, true)
	case "/api/instances":
		s.handleInstances(w, r)
	case "/api/audit-logs":
		s.handleAuditLogs(w, r)
	case "/api/health/diagnostics":
		s.handleHealthDiagnostics(w, r)
	case "/api/docker/status":
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleDockerStatus(w, r)
	case "/api/docker/ps":
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleDockerPs(w, r)
	case "/api/docker/logs":
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleDockerLogs(w, r)
	default:
		if strings.HasPrefix(r.URL.Path, "/api/users/") {
			s.handleUserByID(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/jobs/") {
			s.handleJobByID(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/instances/") {
			s.handleInstanceByID(w, r)
			return
		}
		s.serveStatic(w, r)
	}
}

func (s *server) isSetupAllowed(r *http.Request) bool {
	p := r.URL.Path
	return p == "/health" || p == "/api/setup/status" || p == "/api/setup/admin" || p == "/api/version" ||
		p == "/" || p == "/index.html" || strings.HasPrefix(p, "/assets/") || p == "/favicon.ico"
}

// isStaticAsset returns true for paths that refer to concrete static assets
// (JS bundles, CSS, images, fonts, favicon). These should return 404 when
// missing rather than falling back to index.html.
func isStaticAsset(p string) bool {
	if strings.HasPrefix(p, "assets/") {
		return true
	}
	switch p {
	case "favicon.ico", "index.html":
		return true
	}
	// Common asset file extensions.
	for _, ext := range []string{".js", ".css", ".png", ".jpg", ".jpeg", ".svg", ".ico", ".woff", ".woff2", ".ttf", ".map"} {
		if strings.HasSuffix(p, ext) {
			return true
		}
	}
	return false
}

// serveStatic serves the embedded frontend build. For known asset paths it
// returns 404 if the file is missing; for all other paths it falls back to
// index.html so the SPA router can handle client-side routes.
func (s *server) serveStatic(w http.ResponseWriter, r *http.Request) {
	sub, err := fs.Sub(static.FS, "frontend_dist")
	if err != nil {
		s.logger.Error("failed to access embedded frontend", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	p := strings.TrimPrefix(r.URL.Path, "/")
	if p == "" {
		p = "index.html"
	}

	// Try to serve the requested file.
	data, err := fs.ReadFile(sub, p)
	if err == nil {
		w.Header().Set("Content-Type", detectContentType(p, data))
		w.Write(data)
		return
	}

	// File not found — known asset paths get 404, page routes get SPA fallback.
	if isStaticAsset(p) {
		writeError(w, http.StatusNotFound, "not_found", "资源不存在")
		return
	}

	data, err = fs.ReadFile(sub, "index.html")
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "resource not found")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

func detectContentType(path string, data []byte) string {
	if strings.HasSuffix(path, ".js") {
		return "application/javascript"
	}
	if strings.HasSuffix(path, ".css") {
		return "text/css"
	}
	if strings.HasSuffix(path, ".html") {
		return "text/html; charset=utf-8"
	}
	if strings.HasSuffix(path, ".json") {
		return "application/json"
	}
	if strings.HasSuffix(path, ".svg") {
		return "image/svg+xml"
	}
	if strings.HasSuffix(path, ".png") {
		return "image/png"
	}
	if strings.HasSuffix(path, ".jpg") || strings.HasSuffix(path, ".jpeg") {
		return "image/jpeg"
	}
	if strings.HasSuffix(path, ".ico") {
		return "image/x-icon"
	}
	if strings.HasSuffix(path, ".woff2") {
		return "font/woff2"
	}
	if strings.HasSuffix(path, ".woff") {
		return "font/woff"
	}
	// Fallback to Go's built-in detection.
	return http.DetectContentType(data)
}

func normalizeConfig(cfg config.Config) config.Config {
	if cfg.DataDir == "" {
		cfg.DataDir = config.DefaultDataDir
	}
	if cfg.DBPath == "" {
		cfg.DBPath = cfg.DataDir + "/panel.db"
	}
	if cfg.Version == "" {
		cfg.Version = config.DefaultVersion
	}
	if cfg.PanelMode == "" {
		cfg.PanelMode = config.DefaultPanelMode
	}
	if cfg.DefaultInstanceID == "" {
		cfg.DefaultInstanceID = config.DefaultInstanceID
	}
	if cfg.DefaultDriverID == "" {
		cfg.DefaultDriverID = config.DefaultDriverID
	}
	return cfg
}

type healthResponse struct {
	Status    string         `json:"status"`
	Service   string         `json:"service"`
	Version   string         `json:"version"`
	Commit    string         `json:"commit,omitempty"`
	BuildDate string         `json:"buildDate,omitempty"`
	Database  healthDatabase `json:"database"`
}

type healthDatabase struct {
	Status string `json:"status"`
}

func (s *server) handleHealth(w http.ResponseWriter, r *http.Request) {
	statusCode := http.StatusOK
	response := healthResponse{
		Status:    "ok",
		Service:   serviceName,
		Version:   s.config.Version,
		Commit:    s.config.Commit,
		BuildDate: s.config.BuildDate,
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

type versionResponse struct {
	Version   string `json:"version"`
	Commit    string `json:"commit,omitempty"`
	BuildDate string `json:"buildDate,omitempty"`
}

func (s *server) handleVersion(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, versionResponse{
		Version:   s.config.Version,
		Commit:    s.config.Commit,
		BuildDate: s.config.BuildDate,
	})
}

type errorResponse struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func writeError(w http.ResponseWriter, statusCode int, code string, message string) {
	writeJSON(w, statusCode, errorResponse{
		Error: errorBody{
			Code:    code,
			Message: message,
		},
	})
}

func writeErrorWithDetails(w http.ResponseWriter, statusCode int, code string, message string, details any) {
	writeJSON(w, statusCode, errorResponse{
		Error: errorBody{
			Code:    code,
			Message: message,
			Details: details,
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
