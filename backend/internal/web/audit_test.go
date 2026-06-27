package web

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func parseJSON(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

// TestPermissionHardening_AdminOnlyEndpoints verifies that admin-only endpoints
// reject unauthenticated users (401) and non-admin users (403).
func TestPermissionHardening_AdminOnlyEndpoints(t *testing.T) {
	handler, store, closeFn := newTestHandlerWithStore(t)
	defer closeFn()

	// Setup admin.
	_, adminCookie := doJSON(t, handler, http.MethodPost, "/api/setup/admin", map[string]string{
		"username":        "admin",
		"password":        "admin-password",
		"confirmPassword": "admin-password",
	}, nil)

	// Create a non-admin user.
	doJSON(t, handler, http.MethodPost, "/api/users", map[string]string{
		"username": "player",
		"password": "player-password",
		"role":     "user",
	}, adminCookie)

	// Login as non-admin.
	_, playerCookie := doJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "player",
		"password": "player-password",
	}, nil)

	// Set instance to a state that won't block on running checks.
	_, _ = store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID:           storage.DefaultInstanceID,
		State:        storage.InstanceStateGameInstalled,
		StateMessage: "test",
		DriverPhase:  "game_installed",
	})

	adminEndpoints := []struct {
		name   string
		method string
		path   string
		body   any
	}{
		// Install/prepare
		{"prepare", http.MethodPost, "/api/instances/stardew/prepare", nil},
		{"install", http.MethodPost, "/api/instances/stardew/install", map[string]string{"steamUsername": "x", "steamPassword": "y", "vncPassword": "z"}},
		// Lifecycle
		{"start", http.MethodPost, "/api/instances/stardew/start", nil},
		{"stop", http.MethodPost, "/api/instances/stardew/stop", nil},
		{"restart", http.MethodPost, "/api/instances/stardew/restart", nil},
		// Save writes
		{"save-select", http.MethodPost, "/api/instances/stardew/saves/select", map[string]string{"name": "TestSave"}},
		// Mod writes
		{"mod-delete", http.MethodDelete, "/api/instances/stardew/mods/TestMod", nil},
		// User management
		{"list-users", http.MethodGet, "/api/users", nil},
		// Docker
		{"docker-status", http.MethodGet, "/api/docker/status", nil},
		{"docker-ps", http.MethodGet, "/api/docker/ps", nil},
		// Jobs
		{"test-job", http.MethodPost, "/api/jobs/test", nil},
		// Audit logs
		{"audit-logs", http.MethodGet, "/api/audit-logs", nil},
	}

	for _, tc := range adminEndpoints {
		t.Run(tc.name+"_no_auth", func(t *testing.T) {
			resp, _ := doJSON(t, handler, tc.method, tc.path, tc.body, nil)
			if resp.Code != http.StatusUnauthorized && resp.Code != http.StatusServiceUnavailable {
				t.Errorf("%s %s no auth: got %d, want 401 or 503", tc.method, tc.path, resp.Code)
			}
		})

		t.Run(tc.name+"_non_admin", func(t *testing.T) {
			resp, _ := doJSON(t, handler, tc.method, tc.path, tc.body, playerCookie)
			if resp.Code != http.StatusForbidden {
				t.Errorf("%s %s non-admin: got %d, want 403; body: %s", tc.method, tc.path, resp.Code, resp.Body.String())
			}
		})
	}
}

// TestPermissionHardening_AuthEndpoints verifies that auth-only endpoints
// allow non-admin users but reject unauthenticated users.
func TestPermissionHardening_AuthEndpoints(t *testing.T) {
	handler, _, closeFn := newTestHandlerWithStore(t)
	defer closeFn()

	// Setup admin.
	_, adminCookie := doJSON(t, handler, http.MethodPost, "/api/setup/admin", map[string]string{
		"username":        "admin",
		"password":        "admin-password",
		"confirmPassword": "admin-password",
	}, nil)

	// Create a non-admin user.
	doJSON(t, handler, http.MethodPost, "/api/users", map[string]string{
		"username": "player",
		"password": "player-password",
		"role":     "user",
	}, adminCookie)

	// Login as non-admin.
	_, playerCookie := doJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "player",
		"password": "player-password",
	}, nil)

	authEndpoints := []struct {
		name   string
		method string
		path   string
	}{
		{"list-saves", http.MethodGet, "/api/instances/stardew/saves"},
		{"list-mods", http.MethodGet, "/api/instances/stardew/mods"},
		{"list-commands", http.MethodGet, "/api/instances/stardew/commands"},
		{"instance-detail", http.MethodGet, "/api/instances/stardew"},
		{"instance-state", http.MethodGet, "/api/instances/stardew/state"},
		{"instances-list", http.MethodGet, "/api/instances"},
	}

	for _, tc := range authEndpoints {
		t.Run(tc.name+"_non_admin_allowed", func(t *testing.T) {
			resp, _ := doJSON(t, handler, tc.method, tc.path, nil, playerCookie)
			// Non-admin should get 200 (or at least not 401/403).
			if resp.Code == http.StatusUnauthorized || resp.Code == http.StatusForbidden {
				t.Errorf("%s %s non-admin: got %d, should be allowed", tc.method, tc.path, resp.Code)
			}
		})
	}
}

// TestAuditLogsAPI_Permissions verifies that audit logs API is admin-only.
func TestAuditLogsAPI_Permissions(t *testing.T) {
	handler, _, closeFn := newTestHandlerWithStore(t)
	defer closeFn()

	// Before setup: blocked by setup check.
	resp, _ := doJSON(t, handler, http.MethodGet, "/api/audit-logs", nil, nil)
	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("audit logs before setup: got %d, want 503", resp.Code)
	}

	// Setup admin.
	_, adminCookie := doJSON(t, handler, http.MethodPost, "/api/setup/admin", map[string]string{
		"username":        "admin",
		"password":        "admin-password",
		"confirmPassword": "admin-password",
	}, nil)

	// Admin can access.
	resp, _ = doJSON(t, handler, http.MethodGet, "/api/audit-logs", nil, adminCookie)
	if resp.Code != http.StatusOK {
		t.Fatalf("audit logs admin: got %d, want 200; body: %s", resp.Code, resp.Body.String())
	}

	// Create non-admin user.
	doJSON(t, handler, http.MethodPost, "/api/users", map[string]string{
		"username": "player",
		"password": "player-password",
		"role":     "user",
	}, adminCookie)
	_, playerCookie := doJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "player",
		"password": "player-password",
	}, nil)

	// Non-admin cannot access.
	resp, _ = doJSON(t, handler, http.MethodGet, "/api/audit-logs", nil, playerCookie)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("audit logs non-admin: got %d, want 403; body: %s", resp.Code, resp.Body.String())
	}
}

// TestAuditLogsAPI_ContainsSetupLog verifies that the setup admin creation
// is recorded in the audit log.
func TestAuditLogsAPI_ContainsSetupLog(t *testing.T) {
	handler, _, closeFn := newTestHandlerWithStore(t)
	defer closeFn()

	// Setup admin (this should create an audit log entry).
	_, adminCookie := doJSON(t, handler, http.MethodPost, "/api/setup/admin", map[string]string{
		"username":        "admin",
		"password":        "admin-password",
		"confirmPassword": "admin-password",
	}, nil)

	// Read audit logs.
	resp, _ := doJSON(t, handler, http.MethodGet, "/api/audit-logs", nil, adminCookie)
	if resp.Code != http.StatusOK {
		t.Fatalf("audit logs: got %d", resp.Code)
	}

	// Parse response.
	var result struct {
		Logs  []storage.AuditLogEntry `json:"logs"`
		Total int                     `json:"total"`
	}
	if err := parseJSON(resp.Body.Bytes(), &result); err != nil {
		t.Fatalf("parse audit logs: %v", err)
	}

	if result.Total < 1 {
		t.Fatalf("expected at least 1 audit log, got %d", result.Total)
	}

	// Check that setup_admin_created action exists.
	found := false
	for _, log := range result.Logs {
		if log.Action == "setup_admin_created" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected setup_admin_created action in audit logs")
	}
}
