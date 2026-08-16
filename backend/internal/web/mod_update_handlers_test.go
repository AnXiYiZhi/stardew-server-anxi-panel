package web

import (
	"encoding/json"
	"net/http"
	"testing"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
)

func TestModUpdateCheckPermissionsAndRoutes(t *testing.T) {
	handler, _, dataDir, closeStore := newDockerTestHandlerWithStore(t, fakeDockerService{
		psResult: paneldocker.ComposePsResult{},
	})
	defer closeStore()
	prepareComposeProject(t, dataDir)

	adminCookie := setupDockerAdmin(t, handler)
	unauthorized, _ := doJSON(t, handler, http.MethodGet, "/api/instances/stardew/mod-updates", nil, nil)
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous GET = %d, want 401", unauthorized.Code)
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

	visible, _ := doJSON(t, handler, http.MethodGet, "/api/instances/stardew/mod-updates", nil, playerCookie)
	if visible.Code != http.StatusOK {
		t.Fatalf("player GET = %d: %s", visible.Code, visible.Body.String())
	}
	var result registry.ModUpdateCheckResult
	if err := json.Unmarshal(visible.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Status != "ok" || result.Updates == nil || result.EligibleCount != 0 {
		t.Fatalf("player GET result = %+v", result)
	}

	forbidden, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/mod-updates/check", nil, playerCookie)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("player POST = %d, want 403", forbidden.Code)
	}
	refreshed, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/mod-updates/check", nil, adminCookie)
	if refreshed.Code != http.StatusOK {
		t.Fatalf("admin POST = %d: %s", refreshed.Code, refreshed.Body.String())
	}

	wrongGet, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/mod-updates", nil, adminCookie)
	if wrongGet.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST collection = %d, want 405", wrongGet.Code)
	}
	wrongCheck, _ := doJSON(t, handler, http.MethodGet, "/api/instances/stardew/mod-updates/check", nil, adminCookie)
	if wrongCheck.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET check = %d, want 405", wrongCheck.Code)
	}
}
