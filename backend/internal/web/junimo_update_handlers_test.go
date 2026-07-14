package web

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func TestAcceptStrictEmptyJunimoDryRunBody(t *testing.T) {
	tests := []struct {
		body string
		want bool
	}{
		{"", true}, {"{}", true}, {"  { }\n", true}, {"null", false}, {"[]", false},
		{`{"image":"evil.invalid/server:latest"}`, false}, {`{"command":"docker compose down"}`, false},
	}
	for _, test := range tests {
		if got := acceptStrictEmptyObject(io.NopCloser(strings.NewReader(test.body))); got != test.want {
			t.Errorf("body %q: got %v, want %v", test.body, got, test.want)
		}
	}
}

func TestAcceptStrictJunimoApplyConfirmationRejectsTargetsAndCommands(t *testing.T) {
	tests := []struct {
		body string
		want bool
	}{
		{`{"confirm":true}`, true}, {`{"confirm":false}`, false}, {`{}`, false}, {`null`, false},
		{`{"confirm":true,"image":"evil.invalid/server:latest"}`, false},
		{`{"confirm":true,"tag":"latest"}`, false}, {`{"confirm":true,"service":"server"}`, false},
		{`{"confirm":true,"command":"docker compose down -v"}`, false},
	}
	for _, test := range tests {
		if got := acceptStrictApplyConfirmation(io.NopCloser(strings.NewReader(test.body))); got != test.want {
			t.Errorf("body %s: got %v want %v", test.body, got, test.want)
		}
	}
}

func TestJunimoUpdateApplyPermissions(t *testing.T) {
	handler, _, cleanup := newTestHandlerWithStore(t)
	defer cleanup()
	_, adminCookie := doJSON(t, handler, http.MethodPost, "/api/setup/admin", map[string]string{"username": "admin", "password": "admin-password", "confirmPassword": "admin-password"}, nil)
	created, _ := doJSON(t, handler, http.MethodPost, "/api/users", map[string]string{"username": "player", "password": "player-password", "role": "user"}, adminCookie)
	if created.Code != http.StatusCreated {
		t.Fatalf("create player: %s", created.Body.String())
	}
	_, playerCookie := doJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]string{"username": "player", "password": "player-password"}, nil)
	for _, test := range []struct {
		name   string
		cookie *http.Cookie
		want   int
	}{{"anonymous", nil, http.StatusUnauthorized}, {"ordinary user", playerCookie, http.StatusForbidden}} {
		t.Run(test.name, func(t *testing.T) {
			response, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/junimo-update/apply", map[string]bool{"confirm": true}, test.cookie)
			if response.Code != test.want {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
	response, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/junimo-update/apply", map[string]bool{"confirm": true}, adminCookie)
	if response.Code != http.StatusInternalServerError || !strings.Contains(response.Body.String(), "driver_not_registered") {
		t.Fatalf("admin was not authorized to reach driver contract: %d %s", response.Code, response.Body.String())
	}
}

func TestRuntimeComponentsPermissions(t *testing.T) {
	handler, _, cleanup := newTestHandlerWithStore(t)
	defer cleanup()
	_, adminCookie := doJSON(t, handler, http.MethodPost, "/api/setup/admin", map[string]string{"username": "admin", "password": "admin-password", "confirmPassword": "admin-password"}, nil)
	created, _ := doJSON(t, handler, http.MethodPost, "/api/users", map[string]string{"username": "player", "password": "player-password", "role": "user"}, adminCookie)
	if created.Code != http.StatusCreated {
		t.Fatalf("create player: %s", created.Body.String())
	}
	_, playerCookie := doJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]string{"username": "player", "password": "player-password"}, nil)
	for _, path := range []string{"/api/instances/stardew/runtime-components", "/api/instances/stardew/runtime-components/dry-run"} {
		for _, test := range []struct {
			name   string
			cookie *http.Cookie
			want   int
		}{{"anonymous", nil, http.StatusUnauthorized}, {"ordinary user", playerCookie, http.StatusForbidden}} {
			t.Run(path+test.name, func(t *testing.T) {
				response, _ := doJSON(t, handler, http.MethodGet, path, nil, test.cookie)
				if response.Code != test.want {
					t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
				}
			})
		}
	}
	response, _ := doJSON(t, handler, http.MethodGet, "/api/instances/stardew/runtime-components", nil, adminCookie)
	if response.Code != http.StatusInternalServerError || !strings.Contains(response.Body.String(), "driver_not_registered") {
		t.Fatalf("admin did not reach driver boundary: %d %s", response.Code, response.Body.String())
	}
}

func TestSMAPIUpdatePermissionsAndStrictBodies(t *testing.T) {
	handler, _, _, cleanup := newDockerTestHandlerWithStore(t, fakeDockerService{})
	defer cleanup()
	adminCookie := setupDockerAdmin(t, handler)
	created, _ := doJSON(t, handler, http.MethodPost, "/api/users", map[string]string{"username": "smapi-player", "password": "123456", "role": "user"}, adminCookie)
	if created.Code != http.StatusCreated {
		t.Fatalf("create player: %s", created.Body.String())
	}
	_, playerCookie := doJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]string{"username": "smapi-player", "password": "123456"}, nil)
	for _, path := range []string{"/api/instances/stardew/smapi-update", "/api/instances/stardew/smapi-update/dry-run", "/api/instances/stardew/smapi-update/apply"} {
		for _, permission := range []struct {
			cookie *http.Cookie
			want   int
		}{{nil, http.StatusUnauthorized}, {playerCookie, http.StatusForbidden}} {
			response, _ := doJSON(t, handler, http.MethodGet, path, nil, permission.cookie)
			if response.Code != permission.want {
				t.Fatalf("GET %s = %d: %s", path, response.Code, response.Body.String())
			}
		}
	}
	for _, injected := range []map[string]string{{"url": "https://evil.invalid/smapi.zip"}, {"version": "9.9.9"}, {"sha256": strings.Repeat("0", 64)}, {"command": "docker compose down -v"}} {
		response, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/smapi-update/dry-run", injected, adminCookie)
		if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "dry_run_body_not_allowed") {
			t.Fatalf("SMAPI dry-run target accepted: %d %s", response.Code, response.Body.String())
		}
		body := map[string]any{"confirm": true}
		for key, value := range injected {
			body[key] = value
		}
		response, _ = doJSON(t, handler, http.MethodPost, "/api/instances/stardew/smapi-update/apply", body, adminCookie)
		if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "apply_confirmation_required") {
			t.Fatalf("SMAPI apply target accepted: %d %s", response.Code, response.Body.String())
		}
	}
	response, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/smapi-update", map[string]string{"version": "9.9.9"}, adminCookie)
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST detection endpoint = %d: %s", response.Code, response.Body.String())
	}
}

func TestRuntimeComponentsUninstalledAPI(t *testing.T) {
	handler, _, _, cleanup := newDockerTestHandlerWithStore(t, fakeDockerService{})
	defer cleanup()
	adminCookie := setupDockerAdmin(t, handler)
	response, _ := doJSON(t, handler, http.MethodGet, "/api/instances/stardew/runtime-components", nil, adminCookie)
	if response.Code != http.StatusOK {
		t.Fatalf("GET = %d: %s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	if !strings.Contains(body, `"status":"game_missing"`) || !strings.Contains(body, `"code":"not_installed"`) {
		t.Fatalf("uninstalled contract missing: %s", body)
	}
	if !strings.Contains(body, `"smapi":`) || !strings.Contains(body, `"recommended":{"version":"4.5.2"`) {
		t.Fatalf("runtime-components did not include SMAPI recommendation: %s", body)
	}
	for _, secret := range []string{"STEAM_PASSWORD", "STEAM_USERNAME", "refresh_token", "ticket"} {
		if strings.Contains(strings.ToLower(body), strings.ToLower(secret)) {
			t.Fatalf("response leaked %q: %s", secret, body)
		}
	}
}

func TestSMAPIUpdateAPIPermissionsAndRejectsTargetParameters(t *testing.T) {
	handler, _, _, cleanup := newDockerTestHandlerWithStore(t, fakeDockerService{})
	defer cleanup()
	adminCookie := setupDockerAdmin(t, handler)
	created, _ := doJSON(t, handler, http.MethodPost, "/api/users", map[string]string{"username": "player", "password": "player-password", "role": "user"}, adminCookie)
	if created.Code != http.StatusCreated {
		t.Fatalf("create player: %s", created.Body.String())
	}
	_, playerCookie := doJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]string{"username": "player", "password": "player-password"}, nil)
	for _, endpoint := range []string{"/api/instances/stardew/smapi-update", "/api/instances/stardew/smapi-update/dry-run", "/api/instances/stardew/smapi-update/apply"} {
		for _, permission := range []struct {
			name   string
			cookie *http.Cookie
			want   int
		}{{"anonymous", nil, http.StatusUnauthorized}, {"ordinary user", playerCookie, http.StatusForbidden}} {
			method := http.MethodGet
			response, _ := doJSON(t, handler, method, endpoint, nil, permission.cookie)
			if response.Code != permission.want {
				t.Fatalf("%s %s = %d: %s", permission.name, endpoint, response.Code, response.Body.String())
			}
		}
	}
	inspection, _ := doJSON(t, handler, http.MethodGet, "/api/instances/stardew/smapi-update", nil, adminCookie)
	if inspection.Code != http.StatusOK || !strings.Contains(inspection.Body.String(), `"status":"missing"`) || !strings.Contains(inspection.Body.String(), `"code":"not_installed"`) {
		t.Fatalf("uninstalled SMAPI contract = %d: %s", inspection.Code, inspection.Body.String())
	}
	for _, secret := range []string{"STEAM_PASSWORD", "refresh_token", "app_ticket", "token"} {
		if strings.Contains(strings.ToLower(inspection.Body.String()), strings.ToLower(secret)) {
			t.Fatalf("SMAPI response leaked %q: %s", secret, inspection.Body.String())
		}
	}
	for _, body := range []map[string]any{
		{"version": "9.9.9"}, {"url": "https://example.invalid/smapi.zip"}, {"sha256": strings.Repeat("a", 64)}, {"command": "docker compose down -v"},
	} {
		response, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/smapi-update/dry-run", body, adminCookie)
		if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "dry_run_body_not_allowed") {
			t.Fatalf("dry-run target accepted: %d %s", response.Code, response.Body.String())
		}
	}
	for _, body := range []map[string]any{
		{"confirm": true, "version": "9.9.9"}, {"confirm": true, "url": "https://example.invalid/smapi.zip"}, {"confirm": true, "command": "sh"},
	} {
		response, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/smapi-update/apply", body, adminCookie)
		if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "apply_confirmation_required") {
			t.Fatalf("apply target accepted: %d %s", response.Code, response.Body.String())
		}
	}
}

func TestJunimoUpdateDryRunAPI(t *testing.T) {
	fake := fakeDockerService{instanceState: storage.InstanceStateGameInstalled, psResult: paneldocker.ComposePsResult{Services: []paneldocker.ComposeService{{Service: "server", State: "running"}}}}
	handler, store, dataRoot, cleanup := newDockerTestHandlerWithStore(t, fake)
	defer cleanup()
	adminCookie := setupDockerAdmin(t, handler)
	if _, err := store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{ID: storage.DefaultInstanceID, State: storage.InstanceStateGameInstalled, DriverPhase: "installed", DriverPayload: "{}"}); err != nil {
		t.Fatal(err)
	}
	created, _ := doJSON(t, handler, http.MethodPost, "/api/users", map[string]string{"username": "player", "password": "123456", "role": "user"}, adminCookie)
	if created.Code != http.StatusCreated {
		t.Fatalf("create player = %d: %s", created.Code, created.Body.String())
	}
	_, playerCookie := doJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]string{"username": "player", "password": "123456"}, nil)
	instanceDir := filepath.Join(dataRoot, "instances", storage.DefaultInstanceID)
	if err := os.MkdirAll(instanceDir, 0o700); err != nil {
		t.Fatal(err)
	}
	env := "IMAGE_VERSION=1.4.0-preview.1\nSERVER_IMAGE=sdvd/server:1.4.0-preview.1\nSERVER_IMAGE_CANDIDATES=sdvd/server:1.4.0-preview.1\nSTEAM_SERVICE_IMAGE=anxiyizhi/junimo-steam-service-cn:1.4.0-anxi.1\nSTEAM_SERVICE_IMAGE_CANDIDATES=anxiyizhi/junimo-steam-service-cn:1.4.0-anxi.1\nSTEAM_PASSWORD=never-return-this\nSTEAM_REFRESH_TOKEN=never-return-token\n"
	if err := os.WriteFile(filepath.Join(instanceDir, ".env"), []byte(env), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(instanceDir, "docker-compose.yml"), []byte("services:\n  server: {}\n  steam-auth: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, permission := range []struct {
		name   string
		cookie *http.Cookie
		want   int
	}{{"anonymous", nil, http.StatusUnauthorized}, {"ordinary user", playerCookie, http.StatusForbidden}} {
		t.Run(permission.name, func(t *testing.T) {
			response, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/junimo-update/dry-run", map[string]any{}, permission.cookie)
			if response.Code != permission.want {
				t.Fatalf("POST = %d: %s", response.Code, response.Body.String())
			}
		})
	}
	for _, injected := range []map[string]string{{"image": "evil.invalid/server:latest"}, {"command": "docker compose down"}, {"service": "server"}} {
		response, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/junimo-update/dry-run", injected, adminCookie)
		if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "dry_run_body_not_allowed") {
			t.Fatalf("injection accepted: %d %s", response.Code, response.Body.String())
		}
	}

	started, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/junimo-update/dry-run", map[string]any{}, adminCookie)
	if started.Code != http.StatusAccepted || !strings.Contains(started.Body.String(), `"phase":"starting"`) {
		t.Fatalf("start = %d: %s", started.Code, started.Body.String())
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
		status, _ := doJSON(t, handler, http.MethodGet, "/api/instances/stardew/junimo-update/dry-run", nil, adminCookie)
		if status.Code != http.StatusOK {
			t.Fatalf("GET = %d: %s", status.Code, status.Body.String())
		}
		if strings.Contains(status.Body.String(), "never-return-this") || strings.Contains(status.Body.String(), "never-return-token") || strings.Contains(status.Body.String(), "STEAM_PASSWORD") {
			t.Fatalf("status leaked secrets: %s", status.Body.String())
		}
		if strings.Contains(status.Body.String(), `"phase":"succeeded"`) {
			return
		}
	}
	instance, _ := store.GetInstance(context.Background(), storage.DefaultInstanceID)
	t.Fatalf("dry-run did not finish for %s", instance.DataDir)
}

func TestJunimoUpdatePermissionsAndSensitiveData(t *testing.T) {
	handler, store, cleanup := newTestHandlerWithStore(t)
	defer cleanup()

	_, adminCookie := doJSON(t, handler, http.MethodPost, "/api/setup/admin", map[string]string{
		"username": "admin", "password": "admin-password", "confirmPassword": "admin-password",
	}, nil)
	created, _ := doJSON(t, handler, http.MethodPost, "/api/users", map[string]string{
		"username": "player", "password": "player-password", "role": "user",
	}, adminCookie)
	if created.Code != http.StatusCreated {
		t.Fatalf("create user = %d: %s", created.Code, created.Body.String())
	}
	_, playerCookie := doJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "player", "password": "player-password",
	}, nil)

	instance, err := store.GetInstance(context.Background(), storage.DefaultInstanceID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(instance.DataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	env := strings.Join([]string{
		"IMAGE_VERSION=1.5.0-preview.121",
		"SERVER_IMAGE=sdvd/server:1.5.0-preview.121",
		"SERVER_IMAGE_CANDIDATES=sdvd/server:1.5.0-preview.121",
		"STEAM_SERVICE_IMAGE=anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2",
		"STEAM_SERVICE_IMAGE_CANDIDATES=anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2",
		"STEAM_AUTH_COMPLETED=true",
		"STEAM_PASSWORD=do-not-leak-password",
		"STEAM_REFRESH_TOKEN=do-not-leak-refresh-token",
		"PRIVATE_ENV=do-not-leak-private-env",
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(instance.DataDir, ".env"), []byte(env), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID: instance.ID, State: storage.InstanceStateGameInstalled, DriverPhase: "installed", DriverPayload: "{}",
	}); err != nil {
		t.Fatal(err)
	}

	unauthorized, _ := doJSON(t, handler, http.MethodGet, "/api/instances/stardew/junimo-update", nil, nil)
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized GET = %d", unauthorized.Code)
	}
	forbidden, _ := doJSON(t, handler, http.MethodGet, "/api/instances/stardew/junimo-update", nil, playerCookie)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("ordinary user GET = %d", forbidden.Code)
	}

	response, _ := doJSON(t, handler, http.MethodGet, "/api/instances/stardew/junimo-update", nil, adminCookie)
	if response.Code != http.StatusOK {
		t.Fatalf("admin GET = %d: %s", response.Code, response.Body.String())
	}
	assertJSONField(t, response.Body.Bytes(), "status", "up_to_date")
	assertJSONField(t, response.Body.Bytes(), "serverRunning", false)
	assertJSONField(t, response.Body.Bytes(), "steamAuthLoggedIn", true)
	for _, secret := range []string{"do-not-leak-password", "do-not-leak-refresh-token", "do-not-leak-private-env", "STEAM_PASSWORD", "STEAM_REFRESH_TOKEN", "PRIVATE_ENV"} {
		if strings.Contains(response.Body.String(), secret) {
			t.Fatalf("response leaked %q: %s", secret, response.Body.String())
		}
	}

	state, _ := doJSON(t, handler, http.MethodGet, "/api/instances/stardew/state", nil, playerCookie)
	if state.Code != http.StatusOK {
		t.Fatalf("ordinary user state = %d: %s", state.Code, state.Body.String())
	}
	if !strings.Contains(state.Body.String(), `"serverVersion":"1.5.0-preview.121"`) || !strings.Contains(state.Body.String(), `"junimoUpdateStatus":"up_to_date"`) {
		t.Fatalf("ordinary state omitted necessary version status: %s", state.Body.String())
	}
	if strings.Contains(state.Body.String(), "sdvd/server") || strings.Contains(state.Body.String(), "junimo-steam-service-cn") {
		t.Fatalf("ordinary state leaked image repositories: %s", state.Body.String())
	}
}
