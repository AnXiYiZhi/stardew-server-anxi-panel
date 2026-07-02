package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func TestNexusAPIKeySettingsLifecycle(t *testing.T) {
	handler, store, closeFn := newTestHandlerWithStore(t)
	defer closeFn()

	setup, adminCookie := doJSON(t, handler, http.MethodPost, "/api/setup/admin", map[string]string{
		"username":        "admin",
		"password":        "admin-password",
		"confirmPassword": "admin-password",
	}, nil)
	if setup.Code != http.StatusOK {
		t.Fatalf("setup admin returned %d: %s", setup.Code, setup.Body.String())
	}

	status, _ := doJSON(t, handler, http.MethodGet, "/api/settings/nexus", nil, adminCookie)
	if status.Code != http.StatusOK {
		t.Fatalf("initial status returned %d: %s", status.Code, status.Body.String())
	}
	assertJSONField(t, status.Body.Bytes(), "configured", false)

	const apiKey = "key-from-panel-1234567890"
	saved, _ := doJSON(t, handler, http.MethodPut, "/api/settings/nexus/api-key", map[string]string{
		"apiKey": apiKey,
	}, adminCookie)
	if saved.Code != http.StatusOK {
		t.Fatalf("save key returned %d: %s", saved.Code, saved.Body.String())
	}
	assertJSONField(t, saved.Body.Bytes(), "configured", true)
	assertJSONField(t, saved.Body.Bytes(), "last4", "7890")
	if got, err := store.GetPanelSetting(context.Background(), panelSettingNexusAPIKey); err != nil || got != apiKey {
		t.Fatalf("stored key = %q, %v; want %q", got, err, apiKey)
	}
	if containsJSONValue(t, saved.Body.Bytes(), apiKey) {
		t.Fatal("save response leaked full API key")
	}

	status, _ = doJSON(t, handler, http.MethodGet, "/api/settings/nexus", nil, adminCookie)
	if status.Code != http.StatusOK {
		t.Fatalf("saved status returned %d: %s", status.Code, status.Body.String())
	}
	assertJSONField(t, status.Body.Bytes(), "configured", true)
	assertJSONField(t, status.Body.Bytes(), "last4", "7890")

	deleted, _ := doJSON(t, handler, http.MethodDelete, "/api/settings/nexus/api-key", nil, adminCookie)
	if deleted.Code != http.StatusOK {
		t.Fatalf("delete key returned %d: %s", deleted.Code, deleted.Body.String())
	}
	assertJSONField(t, deleted.Body.Bytes(), "configured", false)
	if _, err := store.GetPanelSetting(context.Background(), panelSettingNexusAPIKey); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("load deleted key: %v", err)
	}
}

func TestNexusAPIKeySettingsRequireAdmin(t *testing.T) {
	handler, _, closeFn := newTestHandlerWithStore(t)
	defer closeFn()

	_, adminCookie := doJSON(t, handler, http.MethodPost, "/api/setup/admin", map[string]string{
		"username":        "admin",
		"password":        "admin-password",
		"confirmPassword": "admin-password",
	}, nil)
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

	status, _ := doJSON(t, handler, http.MethodGet, "/api/settings/nexus", nil, playerCookie)
	if status.Code != http.StatusForbidden {
		t.Fatalf("player status returned %d, want 403", status.Code)
	}
	save, _ := doJSON(t, handler, http.MethodPut, "/api/settings/nexus/api-key", map[string]string{
		"apiKey": "player-key",
	}, playerCookie)
	if save.Code != http.StatusForbidden {
		t.Fatalf("player save returned %d, want 403", save.Code)
	}
}

func containsJSONValue(t *testing.T, body []byte, value string) bool {
	t.Helper()
	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	return containsValue(payload, value)
}

func containsValue(payload any, value string) bool {
	switch v := payload.(type) {
	case string:
		return v == value
	case []any:
		for _, item := range v {
			if containsValue(item, value) {
				return true
			}
		}
	case map[string]any:
		for _, item := range v {
			if containsValue(item, value) {
				return true
			}
		}
	}
	return false
}
