package web

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
)

func writePlayerAuthRosterFixture(t *testing.T, dataDir string) {
	t.Helper()
	control := filepath.Join(dataDir, "instances", "stardew", ".local-container", "control")
	if err := os.MkdirAll(control, 0o700); err != nil {
		t.Fatalf("mkdir player auth control dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(control, "status.json"), []byte(`{"saveId":"Farm"}`), 0o600); err != nil {
		t.Fatalf("write player auth status: %v", err)
	}
	if err := os.WriteFile(filepath.Join(control, "players-cache.json"), []byte(`{
  "saveId":"Farm",
  "updatedAt":"2026-08-15T12:00:00Z",
  "players":[
    {"name":"Host","uniqueMultiplayerId":"1","isHost":true,"status":"offline"},
    {"name":"Leah","uniqueMultiplayerId":"2","status":"offline"}
  ]
}`), 0o600); err != nil {
		t.Fatalf("write player auth roster: %v", err)
	}
}

func TestPlayerAuthConfigAPIEnablesRolePasswordsWithoutLeakingSecrets(t *testing.T) {
	handler, dataDir, closeStore := newDockerTestHandler(t, fakeDockerService{})
	defer closeStore()
	adminCookie := setupDockerAdmin(t, handler)
	writePlayerAuthRosterFixture(t, dataDir)

	initialResponse, _ := doJSON(t, handler, http.MethodGet, "/api/instances/stardew/config/player-auth", nil, adminCookie)
	if initialResponse.Code != http.StatusOK {
		t.Fatalf("initial player auth config returned %d: %s", initialResponse.Code, initialResponse.Body.String())
	}
	var initial sj.PlayerAuthConfigResult
	if err := json.Unmarshal(initialResponse.Body.Bytes(), &initial); err != nil {
		t.Fatalf("decode initial player auth config: %v", err)
	}
	if initial.Mode != sj.PlayerAuthModeNone || len(initial.Roles) != 1 || initial.Roles[0].RoleID != "2" {
		t.Fatalf("initial player auth config = %+v", initial)
	}

	password := "role-secret-must-not-leak"
	updatedResponse, _ := doJSON(t, handler, http.MethodPut, "/api/instances/stardew/config/player-auth", map[string]any{
		"expectedRevision": initial.Revision,
		"mode":             sj.PlayerAuthModeRole,
		"rolePasswordUpdates": []map[string]string{
			{"roleId": "2", "password": password},
		},
	}, adminCookie)
	if updatedResponse.Code != http.StatusOK {
		t.Fatalf("player auth update returned %d: %s", updatedResponse.Code, updatedResponse.Body.String())
	}
	if strings.Contains(updatedResponse.Body.String(), password) {
		t.Fatalf("player auth response leaked plaintext: %s", updatedResponse.Body.String())
	}
	var updated sj.PlayerAuthConfigResult
	if err := json.Unmarshal(updatedResponse.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode updated player auth config: %v", err)
	}
	if updated.Mode != sj.PlayerAuthModeRole || updated.ConfiguredRoleCount != 1 || !updated.Roles[0].Configured {
		t.Fatalf("updated player auth config = %+v", updated)
	}

	legacyResponse, _ := doJSON(t, handler, http.MethodGet, "/api/instances/stardew/config/server-password", nil, adminCookie)
	if legacyResponse.Code != http.StatusConflict || !strings.Contains(legacyResponse.Body.String(), "role_auth_mode_active") {
		t.Fatalf("legacy password API did not fail closed: %d %s", legacyResponse.Code, legacyResponse.Body.String())
	}
	values, err := sjconfig.ReadEnvFile(filepath.Join(dataDir, "instances", "stardew", ".env"))
	if err != nil {
		t.Fatalf("read player auth env: %v", err)
	}
	if values["SERVER_PASSWORD"] == password || strings.Contains(values["SAP_ROLE_PASSWORDS_B64"], password) {
		t.Fatal("player auth env stored the role password as plaintext")
	}

	staleResponse, _ := doJSON(t, handler, http.MethodPut, "/api/instances/stardew/config/player-auth", map[string]any{
		"expectedRevision": initial.Revision,
		"mode":             sj.PlayerAuthModeNone,
	}, adminCookie)
	if staleResponse.Code != http.StatusConflict || !strings.Contains(staleResponse.Body.String(), "player_auth_revision_conflict") {
		t.Fatalf("stale player auth revision was not rejected: %d %s", staleResponse.Code, staleResponse.Body.String())
	}
}

func TestLegacyServerPasswordAPIMapsToExplicitModes(t *testing.T) {
	handler, _, closeStore := newDockerTestHandler(t, fakeDockerService{})
	defer closeStore()
	adminCookie := setupDockerAdmin(t, handler)

	setResponse, _ := doJSON(t, handler, http.MethodPut, "/api/instances/stardew/config/server-password", map[string]string{
		"password": "global-secret",
	}, adminCookie)
	if setResponse.Code != http.StatusOK {
		t.Fatalf("set legacy global password returned %d: %s", setResponse.Code, setResponse.Body.String())
	}
	configResponse, _ := doJSON(t, handler, http.MethodGet, "/api/instances/stardew/config/player-auth", nil, adminCookie)
	var config sj.PlayerAuthConfigResult
	if err := json.Unmarshal(configResponse.Body.Bytes(), &config); err != nil {
		t.Fatalf("decode global config: %v", err)
	}
	if config.Mode != sj.PlayerAuthModeGlobal || config.GlobalPassword != "global-secret" {
		t.Fatalf("legacy API did not map to global mode: %+v", config)
	}

	clearResponse, _ := doJSON(t, handler, http.MethodPut, "/api/instances/stardew/config/server-password", map[string]string{
		"password": "",
	}, adminCookie)
	if clearResponse.Code != http.StatusOK {
		t.Fatalf("clear legacy global password returned %d: %s", clearResponse.Code, clearResponse.Body.String())
	}
	configResponse, _ = doJSON(t, handler, http.MethodGet, "/api/instances/stardew/config/player-auth", nil, adminCookie)
	if err := json.Unmarshal(configResponse.Body.Bytes(), &config); err != nil {
		t.Fatalf("decode cleared config: %v", err)
	}
	if config.Mode != sj.PlayerAuthModeNone {
		t.Fatalf("legacy API did not map empty password to none mode: %+v", config)
	}
}
