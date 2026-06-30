package web

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func TestSetupLoginLogoutAndUserPermissions(t *testing.T) {
	handler, closeStore := newTestHandler(t)
	defer closeStore()

	status, _ := doJSON(t, handler, http.MethodGet, "/api/setup/status", nil, nil)
	if status.Code != http.StatusOK {
		t.Fatalf("setup status returned %d", status.Code)
	}
	assertJSONField(t, status.Body.Bytes(), "initialized", false)

	blocked, _ := doJSON(t, handler, http.MethodGet, "/api/auth/me", nil, nil)
	if blocked.Code != http.StatusServiceUnavailable {
		t.Fatalf("me before setup returned %d", blocked.Code)
	}

	setupBody := map[string]string{
		"username":        "admin",
		"password":        "admin-password",
		"confirmPassword": "admin-password",
	}
	setup, adminCookie := doJSON(t, handler, http.MethodPost, "/api/setup/admin", setupBody, nil)
	if setup.Code != http.StatusOK {
		t.Fatalf("setup admin returned %d: %s", setup.Code, setup.Body.String())
	}
	if adminCookie == nil || !adminCookie.HttpOnly {
		t.Fatal("setup admin must return an HttpOnly session cookie")
	}

	repeat, _ := doJSON(t, handler, http.MethodPost, "/api/setup/admin", setupBody, nil)
	if repeat.Code != http.StatusConflict {
		t.Fatalf("repeat setup returned %d", repeat.Code)
	}

	me, _ := doJSON(t, handler, http.MethodGet, "/api/auth/me", nil, adminCookie)
	if me.Code != http.StatusOK {
		t.Fatalf("me after setup returned %d: %s", me.Code, me.Body.String())
	}
	assertNestedJSONField(t, me.Body.Bytes(), "user", "isSuperAdmin", true)

	createUserBody := map[string]string{
		"username": "player",
		"password": "player-password",
		"role":     "user",
	}
	created, _ := doJSON(t, handler, http.MethodPost, "/api/users", createUserBody, adminCookie)
	if created.Code != http.StatusCreated {
		t.Fatalf("create user returned %d: %s", created.Code, created.Body.String())
	}

	logout, _ := doJSON(t, handler, http.MethodPost, "/api/auth/logout", nil, adminCookie)
	if logout.Code != http.StatusOK {
		t.Fatalf("logout returned %d", logout.Code)
	}

	loggedOut, _ := doJSON(t, handler, http.MethodGet, "/api/auth/me", nil, adminCookie)
	if loggedOut.Code != http.StatusUnauthorized {
		t.Fatalf("me after logout returned %d", loggedOut.Code)
	}

	login, userCookie := doJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "player",
		"password": "player-password",
	}, nil)
	if login.Code != http.StatusOK {
		t.Fatalf("player login returned %d: %s", login.Code, login.Body.String())
	}

	forbidden, _ := doJSON(t, handler, http.MethodGet, "/api/users", nil, userCookie)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("ordinary user list users returned %d", forbidden.Code)
	}
}

func TestLastAdminCannotBeDisabledOrDowngraded(t *testing.T) {
	handler, closeStore := newTestHandler(t)
	defer closeStore()

	setup, adminCookie := doJSON(t, handler, http.MethodPost, "/api/setup/admin", map[string]string{
		"username":        "admin",
		"password":        "admin-password",
		"confirmPassword": "admin-password",
	}, nil)
	if setup.Code != http.StatusOK {
		t.Fatalf("setup admin returned %d: %s", setup.Code, setup.Body.String())
	}

	downgrade, _ := doJSON(t, handler, http.MethodPatch, "/api/users/1", map[string]string{"role": "user"}, adminCookie)
	if downgrade.Code != http.StatusConflict {
		t.Fatalf("downgrade last admin returned %d", downgrade.Code)
	}

	disable, _ := doJSON(t, handler, http.MethodDelete, "/api/users/1", nil, adminCookie)
	if disable.Code != http.StatusBadRequest {
		t.Fatalf("disable current admin returned %d", disable.Code)
	}
}

func TestAdminCanEnableAndHardDeleteUser(t *testing.T) {
	handler, closeStore := newTestHandler(t)
	defer closeStore()

	setup, adminCookie := doJSON(t, handler, http.MethodPost, "/api/setup/admin", map[string]string{
		"username":        "admin",
		"password":        "123456",
		"confirmPassword": "123456",
	}, nil)
	if setup.Code != http.StatusOK {
		t.Fatalf("setup admin with 6 character password returned %d: %s", setup.Code, setup.Body.String())
	}

	created, _ := doJSON(t, handler, http.MethodPost, "/api/users", map[string]string{
		"username": "player",
		"password": "123456",
		"role":     "user",
	}, adminCookie)
	if created.Code != http.StatusCreated {
		t.Fatalf("create user with 6 character password returned %d: %s", created.Code, created.Body.String())
	}

	disable, _ := doJSON(t, handler, http.MethodPatch, "/api/users/2", map[string]bool{"isActive": false}, adminCookie)
	if disable.Code != http.StatusOK {
		t.Fatalf("disable user returned %d: %s", disable.Code, disable.Body.String())
	}

	enable, _ := doJSON(t, handler, http.MethodPatch, "/api/users/2", map[string]bool{"isActive": true}, adminCookie)
	if enable.Code != http.StatusOK {
		t.Fatalf("enable user returned %d: %s", enable.Code, enable.Body.String())
	}

	deleted, _ := doJSON(t, handler, http.MethodDelete, "/api/users/2?hard=true", nil, adminCookie)
	if deleted.Code != http.StatusOK {
		t.Fatalf("hard delete user returned %d: %s", deleted.Code, deleted.Body.String())
	}

	missing, _ := doJSON(t, handler, http.MethodPatch, "/api/users/2", map[string]bool{"isActive": true}, adminCookie)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("update deleted user returned %d", missing.Code)
	}
}

func TestSuperAdminControlsAdminRoleManagement(t *testing.T) {
	handler, closeStore := newTestHandler(t)
	defer closeStore()

	setup, superCookie := doJSON(t, handler, http.MethodPost, "/api/setup/admin", map[string]string{
		"username":        "root",
		"password":        "123456",
		"confirmPassword": "123456",
	}, nil)
	if setup.Code != http.StatusOK {
		t.Fatalf("setup super admin returned %d: %s", setup.Code, setup.Body.String())
	}
	assertNestedJSONField(t, setup.Body.Bytes(), "user", "isSuperAdmin", true)

	adminCreated, _ := doJSON(t, handler, http.MethodPost, "/api/users", map[string]string{
		"username": "manager",
		"password": "123456",
		"role":     "admin",
	}, superCookie)
	if adminCreated.Code != http.StatusCreated {
		t.Fatalf("super admin create admin returned %d: %s", adminCreated.Code, adminCreated.Body.String())
	}
	assertNestedJSONField(t, adminCreated.Body.Bytes(), "user", "isSuperAdmin", false)

	userCreated, _ := doJSON(t, handler, http.MethodPost, "/api/users", map[string]string{
		"username": "player",
		"password": "123456",
		"role":     "user",
	}, superCookie)
	if userCreated.Code != http.StatusCreated {
		t.Fatalf("super admin create user returned %d: %s", userCreated.Code, userCreated.Body.String())
	}

	login, managerCookie := doJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "manager",
		"password": "123456",
	}, nil)
	if login.Code != http.StatusOK {
		t.Fatalf("manager login returned %d: %s", login.Code, login.Body.String())
	}
	assertNestedJSONField(t, login.Body.Bytes(), "user", "isSuperAdmin", false)

	blockCreateAdmin, _ := doJSON(t, handler, http.MethodPost, "/api/users", map[string]string{
		"username": "admin2",
		"password": "123456",
		"role":     "admin",
	}, managerCookie)
	if blockCreateAdmin.Code != http.StatusForbidden {
		t.Fatalf("normal admin create admin returned %d", blockCreateAdmin.Code)
	}

	blockPromote, _ := doJSON(t, handler, http.MethodPatch, "/api/users/3", map[string]string{"role": "admin"}, managerCookie)
	if blockPromote.Code != http.StatusForbidden {
		t.Fatalf("normal admin promote user returned %d", blockPromote.Code)
	}

	blockDemote, _ := doJSON(t, handler, http.MethodPatch, "/api/users/1", map[string]string{"role": "user"}, managerCookie)
	if blockDemote.Code != http.StatusForbidden {
		t.Fatalf("normal admin demote admin returned %d", blockDemote.Code)
	}

	blockAdminDisable, _ := doJSON(t, handler, http.MethodDelete, "/api/users/1", nil, managerCookie)
	if blockAdminDisable.Code != http.StatusForbidden {
		t.Fatalf("normal admin disable admin returned %d", blockAdminDisable.Code)
	}

	disableUser, _ := doJSON(t, handler, http.MethodDelete, "/api/users/3", nil, managerCookie)
	if disableUser.Code != http.StatusOK {
		t.Fatalf("normal admin disable ordinary user returned %d: %s", disableUser.Code, disableUser.Body.String())
	}

	anotherUser, _ := doJSON(t, handler, http.MethodPost, "/api/users", map[string]string{
		"username": "player2",
		"password": "123456",
		"role":     "user",
	}, superCookie)
	if anotherUser.Code != http.StatusCreated {
		t.Fatalf("super admin create second user returned %d: %s", anotherUser.Code, anotherUser.Body.String())
	}

	promote, _ := doJSON(t, handler, http.MethodPatch, "/api/users/4", map[string]string{"role": "admin"}, superCookie)
	if promote.Code != http.StatusOK {
		t.Fatalf("super admin promote user returned %d: %s", promote.Code, promote.Body.String())
	}

	demote, _ := doJSON(t, handler, http.MethodPatch, "/api/users/2", map[string]string{"role": "user"}, superCookie)
	if demote.Code != http.StatusOK {
		t.Fatalf("super admin demote admin returned %d: %s", demote.Code, demote.Body.String())
	}
}

func newTestHandler(t *testing.T) (http.Handler, func()) {
	t.Helper()
	handler, store, cleanup := newTestHandlerWithStore(t)
	_ = store
	return handler, cleanup
}

func newTestHandlerWithStore(t *testing.T) (http.Handler, *storage.Store, func()) {
	t.Helper()
	dataDir := t.TempDir()
	store, err := storage.Open(context.Background(), config.Config{
		Addr:    ":0",
		DataDir: dataDir,
		DBPath:  filepath.Join(dataDir, "panel.db"),
		Secret:  "test-secret",
		Version: "test",
	})
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	if err := store.Migrate(context.Background()); err != nil {
		_ = store.Close()
		t.Fatalf("migrate storage: %v", err)
	}

	handler := NewHandler(Deps{Config: config.Config{Secret: "test-secret", Version: "test"}, Store: store})
	return handler, store, func() {
		if err := store.Close(); err != nil {
			t.Fatalf("close storage: %v", err)
		}
	}
}

func doJSON(t *testing.T, handler http.Handler, method string, path string, body any, cookie *http.Cookie) (*httptest.ResponseRecorder, *http.Cookie) {
	t.Helper()
	var payload bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&payload).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}

	request := httptest.NewRequest(method, path, &payload)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		request.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	for _, responseCookie := range response.Result().Cookies() {
		if responseCookie.Name == "anxi_session" && responseCookie.Value != "" {
			return response, responseCookie
		}
	}
	return response, nil
}

func assertJSONField(t *testing.T, body []byte, field string, expected any) {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if payload[field] != expected {
		t.Fatalf("expected %s=%v, got %v", field, expected, payload[field])
	}
}

func assertNestedJSONField(t *testing.T, body []byte, objectField string, field string, expected any) {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	object, ok := payload[objectField].(map[string]any)
	if !ok {
		t.Fatalf("expected %s to be an object, got %T", objectField, payload[objectField])
	}
	if object[field] != expected {
		t.Fatalf("expected %s.%s=%v, got %v", objectField, field, expected, object[field])
	}
}
