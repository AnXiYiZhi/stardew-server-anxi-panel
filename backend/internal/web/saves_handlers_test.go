package web

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

// TestRunningProtection_ReturnsServerRunning verifies that save-switching,
// save-creation, and mod write operations return 409 server_running when the
// instance is running or starting.
func TestRunningProtection_ReturnsServerRunning(t *testing.T) {
	handler, store, closeFn := newTestHandlerWithStore(t)
	defer closeFn()

	// Setup admin.
	setup, adminCookie := doJSON(t, handler, http.MethodPost, "/api/setup/admin", map[string]string{
		"username":        "admin",
		"password":        "admin-password",
		"confirmPassword": "admin-password",
	}, nil)
	if setup.Code != http.StatusOK {
		t.Fatalf("setup admin returned %d: %s", setup.Code, setup.Body.String())
	}

	// Set instance to running state.
	_, err := store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID:           storage.DefaultInstanceID,
		State:        storage.InstanceStateRunning,
		StateMessage: "test running",
		DriverPhase:  "running",
	})
	if err != nil {
		t.Fatalf("set instance running: %v", err)
	}

	// All these should return 409 while running.
	tests := []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{"custom-new-game", http.MethodPost, "/api/instances/stardew/saves/custom-new-game",
			map[string]any{"farmName": "Test", "farmType": "standard"}},
		{"select", http.MethodPost, "/api/instances/stardew/saves/select",
			map[string]string{"name": "TestSave"}},
		{"select-and-start", http.MethodPost, "/api/instances/stardew/saves/select-and-start",
			map[string]string{"name": "TestSave"}},
		{"delete-mod", http.MethodDelete, "/api/instances/stardew/mods/TestMod", nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, _ := doJSON(t, handler, tc.method, tc.path, tc.body, adminCookie)
			if resp.Code != http.StatusConflict {
				t.Errorf("%s returned %d, want 409; body: %s", tc.name, resp.Code, resp.Body.String())
			}
		})
	}

	// Also test starting state.
	_, err = store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID:           storage.DefaultInstanceID,
		State:        storage.InstanceStateStarting,
		StateMessage: "test starting",
		DriverPhase:  "starting",
	})
	if err != nil {
		t.Fatalf("set instance starting: %v", err)
	}

	for _, tc := range tests {
		t.Run(tc.name+"_starting", func(t *testing.T) {
			resp, _ := doJSON(t, handler, tc.method, tc.path, tc.body, adminCookie)
			if resp.Code != http.StatusConflict {
				t.Errorf("%s (starting) returned %d, want 409; body: %s", tc.name, resp.Code, resp.Body.String())
			}
		})
	}
}

func TestSaveDelete_RunningProtectsOnlyActiveSave(t *testing.T) {
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

	instance, err := store.GetInstance(context.Background(), storage.DefaultInstanceID)
	if err != nil {
		t.Fatalf("get instance: %v", err)
	}
	for _, name := range []string{"ActiveSave", "OtherSave"} {
		saveDir := filepath.Join(instance.DataDir, ".local-container", "saves", "Saves", name)
		if err := os.MkdirAll(saveDir, 0o755); err != nil {
			t.Fatalf("create save %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(saveDir, "SaveGameInfo"), []byte("<SaveGame/>"), 0o644); err != nil {
			t.Fatalf("write save %s: %v", name, err)
		}
	}
	if err := sj.SetActiveSave(instance.DataDir, "ActiveSave"); err != nil {
		t.Fatalf("set active save: %v", err)
	}
	_, err = store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID:           storage.DefaultInstanceID,
		State:        storage.InstanceStateRunning,
		StateMessage: "test running",
		DriverPhase:  "running",
	})
	if err != nil {
		t.Fatalf("set instance running: %v", err)
	}

	activeResp, _ := doJSON(t, handler, http.MethodDelete, "/api/instances/stardew/saves/ActiveSave", nil, adminCookie)
	if activeResp.Code != http.StatusConflict {
		t.Fatalf("delete active save while running returned %d, want 409; body: %s", activeResp.Code, activeResp.Body.String())
	}

	otherResp, _ := doJSON(t, handler, http.MethodDelete, "/api/instances/stardew/saves/OtherSave", nil, adminCookie)
	if otherResp.Code != http.StatusOK {
		t.Fatalf("delete non-active save while running returned %d, want 200; body: %s", otherResp.Code, otherResp.Body.String())
	}
	if err := sj.ValidateSaveExists(instance.DataDir, "OtherSave"); err == nil {
		t.Fatal("non-active save still exists after delete")
	}
	if got := sj.GetActiveSaveName(instance.DataDir); got != "ActiveSave" {
		t.Fatalf("active save changed to %q, want ActiveSave", got)
	}
}

// TestRunningProtection_StoppedAllowsOperations verifies that save/mod operations
// are NOT blocked when the instance is stopped.
func TestRunningProtection_StoppedAllowsOperations(t *testing.T) {
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

	// Set instance to stopped state.
	_, err := store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID:           storage.DefaultInstanceID,
		State:        storage.InstanceStateStopped,
		StateMessage: "test stopped",
		DriverPhase:  "stopped",
	})
	if err != nil {
		t.Fatalf("set instance stopped: %v", err)
	}

	// These should NOT return 409 when stopped.
	// They may return other errors (404, 400), but not 409.
	selectResp, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/saves/select",
		map[string]string{"name": "NonExistent"}, adminCookie)
	if selectResp.Code == http.StatusConflict {
		t.Error("select save returned 409 when stopped — should not block")
	}

	modDeleteResp, _ := doJSON(t, handler, http.MethodDelete, "/api/instances/stardew/mods/NonExistent",
		nil, adminCookie)
	if modDeleteResp.Code == http.StatusConflict {
		t.Error("mod delete returned 409 when stopped — should not block")
	}
}

// TestSaveUploadCommitAndStart_RunningBlocked verifies that upload-commit-and-start
// returns 409 when running and does not import the save.
func TestSaveUploadCommitAndStart_RunningBlocked(t *testing.T) {
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

	_, err := store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID:           storage.DefaultInstanceID,
		State:        storage.InstanceStateRunning,
		StateMessage: "test running",
		DriverPhase:  "running",
	})
	if err != nil {
		t.Fatalf("set instance running: %v", err)
	}

	body, _ := json.Marshal(map[string]any{"token": "fake-token-123"})
	req := httptest.NewRequest(http.MethodPost, "/api/instances/stardew/saves/upload-commit-and-start", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	if adminCookie != nil {
		req.AddCookie(adminCookie)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("upload-commit-and-start running returned %d, want 409; body: %s", w.Code, w.Body.String())
	}
}

// TestModUpload_RunningBlocked verifies that mod upload returns 409 when running.
func TestModUpload_RunningBlocked(t *testing.T) {
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

	_, err := store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID:           storage.DefaultInstanceID,
		State:        storage.InstanceStateRunning,
		StateMessage: "test running",
		DriverPhase:  "running",
	})
	if err != nil {
		t.Fatalf("set instance running: %v", err)
	}

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("mod", "test.zip")
	_, _ = fw.Write([]byte("PKfake"))
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/instances/stardew/mods/upload", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if adminCookie != nil {
		req.AddCookie(adminCookie)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("mod upload running returned %d, want 409; body: %s", w.Code, w.Body.String())
	}
}
