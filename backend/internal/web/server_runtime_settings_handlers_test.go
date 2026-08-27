package web

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func TestServerRuntimeSettingsAPIPlayerLimitLifecycleAndAudit(t *testing.T) {
	handler, _, closeStore := newDockerTestHandler(t, fakeDockerService{})
	defer closeStore()
	adminCookie := setupDockerAdmin(t, handler)

	initialResponse, _ := doJSON(t, handler, http.MethodGet, "/api/instances/stardew/config/server-runtime-settings", nil, adminCookie)
	if initialResponse.Code != http.StatusOK {
		t.Fatalf("initial settings returned %d: %s", initialResponse.Code, initialResponse.Body.String())
	}
	var initial sj.ServerRuntimeSettings
	if err := json.Unmarshal(initialResponse.Body.Bytes(), &initial); err != nil {
		t.Fatalf("decode initial settings: %v", err)
	}
	if initial.MaxPlayers == nil || *initial.MaxPlayers != 10 {
		t.Fatalf("default maxPlayers = %#v, want 10", initial.MaxPlayers)
	}
	if initial.CabinStrategy != "None" {
		t.Fatalf("default cabinStrategy = %q, want None", initial.CabinStrategy)
	}

	for _, boundary := range []int{1, 100} {
		response, _ := doJSON(t, handler, http.MethodPut, "/api/instances/stardew/config/server-runtime-settings", map[string]any{
			"maxPlayers":             boundary,
			"cabinStrategy":          "CabinStack",
			"existingCabinBehavior":  "KeepExisting",
			"networkBroadcastPeriod": 1,
		}, adminCookie)
		if response.Code != http.StatusOK {
			t.Fatalf("maxPlayers=%d returned %d: %s", boundary, response.Code, response.Body.String())
		}
	}

	for _, invalid := range []int{0, 101} {
		response, _ := doJSON(t, handler, http.MethodPut, "/api/instances/stardew/config/server-runtime-settings", map[string]any{
			"maxPlayers":             invalid,
			"cabinStrategy":          "CabinStack",
			"existingCabinBehavior":  "KeepExisting",
			"networkBroadcastPeriod": 1,
		}, adminCookie)
		if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "invalid_settings") {
			t.Fatalf("maxPlayers=%d returned %d: %s", invalid, response.Code, response.Body.String())
		}
	}

	set37Response, _ := doJSON(t, handler, http.MethodPut, "/api/instances/stardew/config/server-runtime-settings", map[string]any{
		"maxPlayers":             37,
		"cabinStrategy":          "CabinStack",
		"existingCabinBehavior":  "KeepExisting",
		"networkBroadcastPeriod": 1,
	}, adminCookie)
	if set37Response.Code != http.StatusOK {
		t.Fatalf("set maxPlayers=37 returned %d: %s", set37Response.Code, set37Response.Body.String())
	}

	legacyResponse, _ := doJSON(t, handler, http.MethodPut, "/api/instances/stardew/config/server-runtime-settings", map[string]any{
		"cabinStrategy":          "None",
		"existingCabinBehavior":  "MoveToStack",
		"networkBroadcastPeriod": 3,
	}, adminCookie)
	if legacyResponse.Code != http.StatusOK {
		t.Fatalf("legacy settings update returned %d: %s", legacyResponse.Code, legacyResponse.Body.String())
	}
	var legacy sj.ServerRuntimeSettings
	if err := json.Unmarshal(legacyResponse.Body.Bytes(), &legacy); err != nil {
		t.Fatalf("decode legacy update: %v", err)
	}
	if legacy.MaxPlayers == nil || *legacy.MaxPlayers != 37 {
		t.Fatalf("legacy update maxPlayers = %#v, want preserved 37", legacy.MaxPlayers)
	}

	auditResponse, _ := doJSON(t, handler, http.MethodGet, "/api/audit-logs?limit=100", nil, adminCookie)
	if auditResponse.Code != http.StatusOK {
		t.Fatalf("audit logs returned %d: %s", auditResponse.Code, auditResponse.Body.String())
	}
	var auditResult struct {
		Logs []storage.AuditLogEntry `json:"logs"`
	}
	if err := json.Unmarshal(auditResponse.Body.Bytes(), &auditResult); err != nil {
		t.Fatalf("decode audit logs: %v", err)
	}
	found := false
	for _, entry := range auditResult.Logs {
		if entry.Action != "instance_server_runtime_settings_update" {
			continue
		}
		var metadata map[string]any
		if err := json.Unmarshal([]byte(entry.MetadataJSON), &metadata); err != nil {
			t.Fatalf("decode audit metadata: %v", err)
		}
		if metadata["maxPlayers"] == "37" && metadata["previousMaxPlayers"] == "100" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing maxPlayers old/new audit metadata: %+v", auditResult.Logs)
	}
}

func TestServerRuntimeSettingsAPIRequiresAdmin(t *testing.T) {
	handler, _, closeStore := newDockerTestHandler(t, fakeDockerService{})
	defer closeStore()
	adminCookie := setupDockerAdmin(t, handler)
	created, _ := doJSON(t, handler, http.MethodPost, "/api/users", map[string]string{
		"username": "player",
		"password": "player-password",
		"role":     "user",
	}, adminCookie)
	if created.Code != http.StatusCreated {
		t.Fatalf("create player returned %d: %s", created.Code, created.Body.String())
	}
	login, playerCookie := doJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "player",
		"password": "player-password",
	}, nil)
	if login.Code != http.StatusOK {
		t.Fatalf("player login returned %d: %s", login.Code, login.Body.String())
	}
	body := map[string]any{
		"maxPlayers":             24,
		"cabinStrategy":          "CabinStack",
		"existingCabinBehavior":  "KeepExisting",
		"networkBroadcastPeriod": 1,
	}
	for _, testCase := range []struct {
		name       string
		method     string
		cookie     *http.Cookie
		wantStatus int
	}{
		{name: "anonymous_get", method: http.MethodGet, wantStatus: http.StatusUnauthorized},
		{name: "anonymous_put", method: http.MethodPut, wantStatus: http.StatusUnauthorized},
		{name: "user_get", method: http.MethodGet, cookie: playerCookie, wantStatus: http.StatusForbidden},
		{name: "user_put", method: http.MethodPut, cookie: playerCookie, wantStatus: http.StatusForbidden},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			response, _ := doJSON(t, handler, testCase.method, "/api/instances/stardew/config/server-runtime-settings", body, testCase.cookie)
			if response.Code != testCase.wantStatus {
				t.Fatalf("returned %d, want %d: %s", response.Code, testCase.wantStatus, response.Body.String())
			}
		})
	}
}
