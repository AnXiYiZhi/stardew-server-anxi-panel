package web

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
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

func TestModUpload_AcceptsMultipleZipFiles(t *testing.T) {
	handler, _, closeFn := newTestHandlerWithStore(t)
	defer closeFn()

	setup, adminCookie := doJSON(t, handler, http.MethodPost, "/api/setup/admin", map[string]string{
		"username":        "admin",
		"password":        "admin-password",
		"confirmPassword": "admin-password",
	}, nil)
	if setup.Code != http.StatusOK {
		t.Fatalf("setup admin returned %d: %s", setup.Code, setup.Body.String())
	}

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	addModZipPart(t, mw, "ModA.zip", "ModA", "author.moda", "Mod A")
	addModZipPart(t, mw, "ModB.zip", "ModB", "author.modb", "Mod B")
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/instances/stardew/mods/upload", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if adminCookie != nil {
		req.AddCookie(adminCookie)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("mod upload returned %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var result registry.ModsListResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(result.Mods) != 2 {
		t.Fatalf("len(mods) = %d, want 2: %+v", len(result.Mods), result.Mods)
	}
	if result.RestartRequired {
		t.Fatal("RestartRequired = true, want false for stopped-server upload")
	}
	if result.Upload == nil {
		t.Fatal("upload summary is nil")
	}
	if result.Upload.ArchiveCount != 2 || result.Upload.DiscoveredCount != 2 || result.Upload.ImportedCount != 2 || result.Upload.EnabledCount != 2 {
		t.Fatalf("unexpected upload summary: %+v", result.Upload)
	}
}

func TestModUpload_DuplicateUniqueIDReturnsModExists(t *testing.T) {
	handler, _, closeFn := newTestHandlerWithStore(t)
	defer closeFn()

	setup, adminCookie := doJSON(t, handler, http.MethodPost, "/api/setup/admin", map[string]string{
		"username":        "admin",
		"password":        "admin-password",
		"confirmPassword": "admin-password",
	}, nil)
	if setup.Code != http.StatusOK {
		t.Fatalf("setup admin returned %d: %s", setup.Code, setup.Body.String())
	}

	postMod := func(folderName string) *httptest.ResponseRecorder {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		addModZipPart(t, mw, folderName+".zip", folderName, "author.duplicate", folderName)
		if err := mw.Close(); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/api/instances/stardew/mods/upload", &buf)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		if adminCookie != nil {
			req.AddCookie(adminCookie)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w
	}

	first := postMod("DuplicateA")
	if first.Code != http.StatusOK {
		t.Fatalf("first mod upload returned %d, want 200; body: %s", first.Code, first.Body.String())
	}
	second := postMod("DuplicateB")
	if second.Code != http.StatusBadRequest {
		t.Fatalf("second mod upload returned %d, want 400; body: %s", second.Code, second.Body.String())
	}
	var payload errorResponse
	if err := json.Unmarshal(second.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if payload.Error.Code != "mod_exists" {
		t.Fatalf("error code = %q, want mod_exists; body: %s", payload.Error.Code, second.Body.String())
	}
}

func TestModsList_StoppedServerSuppressesStaleRestartRequired(t *testing.T) {
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
	if err := sj.SetModsRestartRequired(instance.DataDir); err != nil {
		t.Fatalf("set restart flag: %v", err)
	}
	_, err = store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID:           storage.DefaultInstanceID,
		State:        storage.InstanceStateStopped,
		StateMessage: "test stopped",
		DriverPhase:  "stopped",
	})
	if err != nil {
		t.Fatalf("set instance stopped: %v", err)
	}

	resp, _ := doJSON(t, handler, http.MethodGet, "/api/instances/stardew/mods", nil, adminCookie)
	if resp.Code != http.StatusOK {
		t.Fatalf("mods list returned %d, want 200; body: %s", resp.Code, resp.Body.String())
	}
	var result registry.ModsListResult
	if err := json.Unmarshal(resp.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result.RestartRequired {
		t.Fatal("RestartRequired = true, want false while server is stopped")
	}
}

// TestSavesBackupRestore_RunningWithoutAutoRestartReturns409 preserves the
// existing behavior for callers that don't opt into auto stop/restart: a
// plain restore request while running/starting must still be rejected.
func TestSavesBackupRestore_RunningWithoutAutoRestartReturns409(t *testing.T) {
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
	if _, err := store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID: storage.DefaultInstanceID, State: storage.InstanceStateRunning, StateMessage: "test running", DriverPhase: "running",
	}); err != nil {
		t.Fatalf("set instance running: %v", err)
	}

	resp, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/saves/backups/restore",
		map[string]any{"backupName": "whatever.zip", "overwrite": false}, adminCookie)
	if resp.Code != http.StatusConflict {
		t.Fatalf("restore without autoRestart while running returned %d, want 409; body: %s", resp.Code, resp.Body.String())
	}
	var payload errorResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if payload.Error.Code != "server_running" {
		t.Fatalf("error code = %q, want server_running", payload.Error.Code)
	}
}

// TestSavesBackupRestore_RunningWithAutoRestartBypassesRunningGate verifies
// that setting autoRestart:true takes a different code path than the plain
// 409 gate — it should proceed to load the driver (and fail there, since the
// test harness has no game driver registered) instead of short-circuiting
// with server_running. This is the cheapest way to assert the new branch is
// actually reached without standing up a full fake GameDriver.
func TestSavesBackupRestore_RunningWithAutoRestartBypassesRunningGate(t *testing.T) {
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
	if _, err := store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID: storage.DefaultInstanceID, State: storage.InstanceStateRunning, StateMessage: "test running", DriverPhase: "running",
	}); err != nil {
		t.Fatalf("set instance running: %v", err)
	}

	resp, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/saves/backups/restore",
		map[string]any{"backupName": "whatever.zip", "overwrite": false, "autoRestart": true}, adminCookie)
	if resp.Code == http.StatusConflict {
		t.Fatalf("autoRestart request should not hit the server_running gate; body: %s", resp.Body.String())
	}
	var payload errorResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if payload.Error.Code != "driver_not_registered" {
		t.Fatalf("expected to reach driver loading (driver_not_registered in this test harness), got code %q; body: %s", payload.Error.Code, resp.Body.String())
	}
}

// TestSavesBackupRestore_StoppedStillWorksSynchronously preserves the existing
// synchronous restore behavior when the server is stopped (no autoRestart
// needed or requested).
func TestSavesBackupRestore_StoppedStillWorksSynchronously(t *testing.T) {
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
	saveDir := filepath.Join(instance.DataDir, ".local-container", "saves", "Saves", "TestSave")
	if err := os.MkdirAll(saveDir, 0o755); err != nil {
		t.Fatalf("create save dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(saveDir, "SaveGameInfo"), []byte("<SaveGame/>"), 0o644); err != nil {
		t.Fatalf("write SaveGameInfo: %v", err)
	}
	backupPath, err := sj.BackupSave(instance.DataDir, "TestSave")
	if err != nil {
		t.Fatalf("BackupSave: %v", err)
	}
	if err := os.RemoveAll(saveDir); err != nil {
		t.Fatalf("remove save dir: %v", err)
	}
	if _, err := store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID: storage.DefaultInstanceID, State: storage.InstanceStateStopped, StateMessage: "test stopped", DriverPhase: "stopped",
	}); err != nil {
		t.Fatalf("set instance stopped: %v", err)
	}

	resp, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/saves/backups/restore",
		map[string]any{"backupName": filepath.Base(backupPath), "overwrite": false}, adminCookie)
	if resp.Code != http.StatusOK {
		t.Fatalf("restore while stopped returned %d, want 200; body: %s", resp.Code, resp.Body.String())
	}
	var result struct {
		SaveName string `json:"saveName"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result.SaveName != "TestSave" {
		t.Fatalf("saveName = %q, want TestSave", result.SaveName)
	}
}

func addModZipPart(t *testing.T, mw *multipart.Writer, filename, folderName, uniqueID, modName string) {
	t.Helper()
	fw, err := mw.CreateFormFile("mod", filename)
	if err != nil {
		t.Fatal(err)
	}
	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	manifest, err := zw.Create(folderName + "/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = manifest.Write([]byte(`{"Name":"` + modName + `","UniqueID":"` + uniqueID + `","Version":"1.0.0","Author":"Tester"}`))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(zipBuf.Bytes()); err != nil {
		t.Fatal(err)
	}
}
