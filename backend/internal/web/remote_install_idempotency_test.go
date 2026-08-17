package web

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

type stoppedRemoteInstallDriver struct {
	registry.GameDriver
}

func (d *stoppedRemoteInstallDriver) ID() string   { return sj.DriverID }
func (d *stoppedRemoteInstallDriver) Name() string { return "test" }
func (d *stoppedRemoteInstallDriver) Status(_ context.Context, instance registry.Instance) (*registry.ServerStatus, error) {
	return &registry.ServerStatus{InstanceID: instance.ID, Runtime: &registry.RuntimeStatus{}}, nil
}

func TestRemoteInstallReusesPersistedIdempotentJob(t *testing.T) {
	handler, store, closeStore := newRemoteInstallIdempotencyHandler(t)
	defer closeStore()
	_, adminCookie := doJSON(t, handler, http.MethodPost, "/api/setup/admin", map[string]string{
		"username": "admin", "password": "admin-password", "confirmPassword": "admin-password",
	}, nil)

	first, err := store.CreateIdempotentJob(context.Background(), storage.CreateJobParams{
		Type: "mod_remote_install", TargetType: "instance", TargetID: storage.DefaultInstanceID,
		IdempotencyKey: "nexus-http-request-1",
	})
	if err != nil {
		t.Fatalf("create persisted job: %v", err)
	}
	if _, err := store.FinishJob(context.Background(), first.ID); err != nil {
		t.Fatalf("finish persisted job: %v", err)
	}
	if _, err := store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID: storage.DefaultInstanceID, State: storage.InstanceStateRunning, DriverPhase: "running", DriverPayload: "{}",
	}); err != nil {
		t.Fatalf("mark instance running: %v", err)
	}

	response := doRemoteInstallJSON(t, handler, adminCookie, "nexus-http-request-1")
	if response.Code != http.StatusAccepted {
		t.Fatalf("repeat remote install = %d: %s", response.Code, response.Body.String())
	}
	var body struct {
		JobID   string `json:"jobId"`
		Deduped bool   `json:"deduped"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.JobID != first.ID || !body.Deduped {
		t.Fatalf("response = %#v, want reused job %s", body, first.ID)
	}

	allJobs, err := store.ListJobs(context.Background(), storage.ListJobsFilter{IsAdmin: true, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	remoteJobs := 0
	for _, job := range allJobs {
		if job.Type == "mod_remote_install" {
			remoteJobs++
		}
	}
	if remoteJobs != 1 {
		t.Fatalf("remote install jobs = %d, want one", remoteJobs)
	}
}

func TestRemoteInstallRejectsInvalidIdempotencyKey(t *testing.T) {
	handler, _, closeStore := newRemoteInstallIdempotencyHandler(t)
	defer closeStore()
	_, adminCookie := doJSON(t, handler, http.MethodPost, "/api/setup/admin", map[string]string{
		"username": "admin", "password": "admin-password", "confirmPassword": "admin-password",
	}, nil)

	response := doRemoteInstallJSON(t, handler, adminCookie, strings.Repeat("x", maxRemoteInstallIdempotencyKeyBytes+1))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid idempotency key = %d: %s", response.Code, response.Body.String())
	}
	assertNestedJSONField(t, response.Body.Bytes(), "error", "code", "invalid_idempotency_key")
	assertNestedJSONField(t, response.Body.Bytes(), "error", "message", "Idempotency-Key 必须是 1 到 128 字节的可见 ASCII 字符")
}

func TestRemoteInstallRejectsInvalidReplaceUniqueID(t *testing.T) {
	handler, _, closeStore := newRemoteInstallIdempotencyHandler(t)
	defer closeStore()
	_, adminCookie := doJSON(t, handler, http.MethodPost, "/api/setup/admin", map[string]string{
		"username": "admin", "password": "admin-password", "confirmPassword": "admin-password",
	}, nil)

	response, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/mods/remote/install", map[string]any{
		"url":             "https://supporter-files.nexus-cdn.com/mods/1303/1234/example.zip?key=secret",
		"mod":             map[string]any{"modId": 1234, "name": "Example"},
		"replaceUniqueId": "Pathoschild.ContentPatcher\nforged",
	}, adminCookie)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid replace UniqueID = %d: %s", response.Code, response.Body.String())
	}
	assertNestedJSONField(t, response.Body.Bytes(), "error", "code", "invalid_replace_unique_id")
}

func TestRemoteInstallUpdateRequiresExactNexusTarget(t *testing.T) {
	handler, _, closeStore := newRemoteInstallIdempotencyHandler(t)
	defer closeStore()
	_, adminCookie := doJSON(t, handler, http.MethodPost, "/api/setup/admin", map[string]string{
		"username": "admin", "password": "admin-password", "confirmPassword": "admin-password",
	}, nil)

	for _, test := range []struct {
		name string
		body map[string]any
		code string
	}{
		{
			name: "missing expected version",
			body: map[string]any{"url": "https://supporter-files.nexus-cdn.com/mods/1303/1234/example.zip", "mod": map[string]any{"modId": 1234}, "nexusFileId": 160463, "replaceUniqueId": "Pathoschild.ContentPatcher"},
			code: "invalid_expected_mod_version",
		},
		{
			name: "missing file id",
			body: map[string]any{"url": "https://supporter-files.nexus-cdn.com/mods/1303/1234/example.zip", "mod": map[string]any{"modId": 1234}, "expectedVersion": "2.9.1", "replaceUniqueId": "Pathoschild.ContentPatcher"},
			code: "invalid_nexus_file_id",
		},
		{
			name: "missing mod id",
			body: map[string]any{"url": "https://supporter-files.nexus-cdn.com/mods/1303/1234/example.zip", "mod": map[string]any{"name": "Content Patcher"}, "expectedVersion": "2.9.1", "nexusFileId": 160463, "replaceUniqueId": "Pathoschild.ContentPatcher"},
			code: "invalid_nexus_mod_id",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			response, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/mods/remote/install", test.body, adminCookie)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("update target validation = %d: %s", response.Code, response.Body.String())
			}
			assertNestedJSONField(t, response.Body.Bytes(), "error", "code", test.code)
		})
	}
}

func newRemoteInstallIdempotencyHandler(t *testing.T) (http.Handler, *storage.Store, func()) {
	t.Helper()
	dataDir := t.TempDir()
	store, err := storage.Open(context.Background(), config.Config{
		Addr: ":0", DataDir: dataDir, DBPath: filepath.Join(dataDir, "panel.db"), Secret: "test-secret", Version: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(context.Background()); err != nil {
		_ = store.Close()
		t.Fatal(err)
	}
	if _, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{
		ID: storage.DefaultInstanceID, DriverID: sj.DriverID, Name: "test", DataDir: filepath.Join(dataDir, "instances", storage.DefaultInstanceID),
	}); err != nil {
		_ = store.Close()
		t.Fatal(err)
	}
	if _, err := store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID: storage.DefaultInstanceID, State: storage.InstanceStateStopped, DriverPhase: "stopped", DriverPayload: "{}",
	}); err != nil {
		_ = store.Close()
		t.Fatal(err)
	}
	drivers := registry.New()
	if err := drivers.Register(&stoppedRemoteInstallDriver{}); err != nil {
		_ = store.Close()
		t.Fatal(err)
	}
	manager := jobs.NewManager(store, nil)
	handler := NewHandler(Deps{
		Config: config.Config{Addr: ":0", DataDir: dataDir, DBPath: filepath.Join(dataDir, "panel.db"), Secret: "test-secret", Version: "test"},
		Store:  store, Jobs: manager, Registry: drivers,
	})
	return handler, store, func() {
		if err := store.Close(); err != nil {
			t.Fatalf("close store: %v", err)
		}
	}
}

func doRemoteInstallJSON(t *testing.T, handler http.Handler, cookie *http.Cookie, idempotencyKey string) *httptest.ResponseRecorder {
	t.Helper()
	var payload bytes.Buffer
	if err := json.NewEncoder(&payload).Encode(map[string]any{
		"url": "https://supporter-files.nexus-cdn.com/mods/1303/1234/example.zip?key=secret",
		"mod": map[string]any{"modId": 1234, "name": "Example"},
	}); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/instances/stardew/mods/remote/install", &payload)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", idempotencyKey)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
