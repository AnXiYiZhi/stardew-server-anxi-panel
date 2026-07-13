package web

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/updatecheck"
)

type fakeUpdateChecker struct {
	status updatecheck.Status
	checks int
}

func (f *fakeUpdateChecker) Status() updatecheck.Status { return f.status }
func (f *fakeUpdateChecker) Check(context.Context) updatecheck.Status {
	f.checks++
	f.status.CheckStatus = updatecheck.StatusOK
	return f.status
}

func TestSystemUpdatePermissions(t *testing.T) {
	dataDir := t.TempDir()
	store, err := storage.Open(context.Background(), config.Config{
		DataDir: dataDir, DBPath: filepath.Join(dataDir, "panel.db"), Secret: "test-secret", Version: "0.1.14",
	})
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	checkedAt := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	checker := &fakeUpdateChecker{status: updatecheck.Status{
		CurrentVersion: "0.1.14", LatestVersion: "v0.1.15", UpdateAvailable: true,
		ReleaseURL: "https://example/release", CheckedAt: &checkedAt, CheckStatus: updatecheck.StatusOK,
	}}
	handler := NewHandler(Deps{
		Config: config.Config{DataDir: dataDir, Secret: "test-secret", Version: "0.1.14"},
		Store:  store, UpdateChecker: checker,
	})

	_, adminCookie := doJSON(t, handler, http.MethodPost, "/api/setup/admin", map[string]string{
		"username": "admin", "password": "admin-password", "confirmPassword": "admin-password",
	}, nil)

	unauthorized, _ := doJSON(t, handler, http.MethodGet, "/api/system/update", nil, nil)
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized GET = %d", unauthorized.Code)
	}

	created, _ := doJSON(t, handler, http.MethodPost, "/api/users", map[string]string{
		"username": "player", "password": "player-password", "role": "user",
	}, adminCookie)
	if created.Code != http.StatusCreated {
		t.Fatalf("create player = %d: %s", created.Code, created.Body.String())
	}
	_, playerCookie := doJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "player", "password": "player-password",
	}, nil)

	visible, _ := doJSON(t, handler, http.MethodGet, "/api/system/update", nil, playerCookie)
	if visible.Code != http.StatusOK {
		t.Fatalf("player GET = %d: %s", visible.Code, visible.Body.String())
	}
	assertJSONField(t, visible.Body.Bytes(), "updateAvailable", true)

	forbidden, _ := doJSON(t, handler, http.MethodPost, "/api/system/update/check", nil, playerCookie)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("player POST = %d", forbidden.Code)
	}
	if checker.checks != 0 {
		t.Fatalf("forbidden refresh invoked checker %d times", checker.checks)
	}

	refreshed, _ := doJSON(t, handler, http.MethodPost, "/api/system/update/check", nil, adminCookie)
	if refreshed.Code != http.StatusOK {
		t.Fatalf("admin POST = %d: %s", refreshed.Code, refreshed.Body.String())
	}
	if checker.checks != 1 {
		t.Fatalf("admin refresh invoked checker %d times", checker.checks)
	}
}
