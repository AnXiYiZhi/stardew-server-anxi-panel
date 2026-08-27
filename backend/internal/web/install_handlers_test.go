package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func TestWriteActiveInstallConflictReturnsExistingJobID(t *testing.T) {
	recorder := httptest.NewRecorder()
	err := fmt.Errorf("start install job: %w", &storage.ActiveJobExistsError{Job: storage.Job{
		ID: "job_existing", Type: "stardew_install", TargetType: "instance", TargetID: storage.DefaultInstanceID,
	}})
	jobID, handled := writeActiveInstallConflict(recorder, err, "已有安装任务")
	if !handled || jobID != "job_existing" {
		t.Fatalf("handled=%v jobID=%q", handled, jobID)
	}
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", recorder.Code)
	}
	var payload struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			Details struct {
				JobID string `json:"jobId"`
			} `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Error.Code != "install_in_progress" || payload.Error.Message != "已有安装任务" || payload.Error.Details.JobID != "job_existing" {
		t.Fatalf("unexpected conflict payload: %#v", payload)
	}
}

func TestWriteActiveInstallConflictIgnoresUnrelatedError(t *testing.T) {
	recorder := httptest.NewRecorder()
	if _, handled := writeActiveInstallConflict(recorder, fmt.Errorf("boom"), "ignored"); handled {
		t.Fatal("unrelated error should not be handled as install conflict")
	}
}

func TestSteamInviteAuthorizationInstalledStateRequiresCompletedBaseInstall(t *testing.T) {
	installed := []string{
		storage.InstanceStateGameInstalled,
		storage.InstanceStateSaveRequired,
		storage.InstanceStateReadyToStart,
		storage.InstanceStateStarting,
		storage.InstanceStateRunning,
		storage.InstanceStateStopped,
	}
	for _, state := range installed {
		if !steamInviteAuthorizationInstalledState(state) {
			t.Fatalf("state %q should permit optional Steam invite authorization", state)
		}
	}
	for _, state := range []string{"", storage.InstanceStateUninitialized, storage.InstanceStateSteamAuthDone, storage.InstanceStateError} {
		if steamInviteAuthorizationInstalledState(state) {
			t.Fatalf("state %q should still require base installation", state)
		}
	}
}

func TestSteamCredentialsUpdateRequiresAdminAndPreservesAuthorizationState(t *testing.T) {
	handler, store, dataDir, cleanup := newDockerTestHandlerWithStore(t, fakeDockerService{
		instanceState: storage.InstanceStateStopped,
	})
	defer cleanup()
	adminCookie := setupDockerAdmin(t, handler)
	setDockerTestInstanceState(t, store, storage.InstanceStateStopped)
	created, _ := doJSON(t, handler, http.MethodPost, "/api/users", map[string]string{
		"username": "player", "password": "player-password", "role": "user",
	}, adminCookie)
	if created.Code != http.StatusCreated {
		t.Fatalf("create user returned %d: %s", created.Code, created.Body.String())
	}
	login, playerCookie := doJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "player", "password": "player-password",
	}, nil)
	if login.Code != http.StatusOK {
		t.Fatalf("player login returned %d: %s", login.Code, login.Body.String())
	}

	instanceDir := filepath.Join(dataDir, "instances", storage.DefaultInstanceID)
	if err := os.MkdirAll(instanceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	envPath := filepath.Join(instanceDir, ".env")
	before := map[string]string{
		"STEAM_USERNAME":          "old-user",
		"STEAM_PASSWORD":          "old-password",
		"VNC_PASSWORD":            "vnc-password",
		"STEAMCMD_AUTH_COMPLETED": "true",
		"STEAM_AUTH_COMPLETED":    "true",
		"STEAM_INVITE_ENABLED":    "true",
		"STEAM_INVITE_AUTH_STATE": sjconfig.SteamInviteAuthStateReady,
		"STEAM_REFRESH_TOKEN":     "existing-auth-token",
		"GAME_DATA_VOLUME":        "existing-game-data",
		"CUSTOM_SENTINEL":         "preserve-me",
	}
	if err := sjconfig.UpdateEnvFile(envPath, before); err != nil {
		t.Fatal(err)
	}

	forbidden, _ := doJSON(t, handler, http.MethodPut, "/api/instances/stardew/steam-credentials", map[string]string{
		"steamUsername": "forbidden-user", "steamPassword": "forbidden-password",
	}, playerCookie)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("ordinary user credential update returned %d: %s", forbidden.Code, forbidden.Body.String())
	}
	wrongMethod, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/steam-credentials", map[string]string{
		"steamUsername": "new-user", "steamPassword": "new-password",
	}, adminCookie)
	if wrongMethod.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST credential update returned %d: %s", wrongMethod.Code, wrongMethod.Body.String())
	}
	for _, body := range []map[string]string{
		{"steamUsername": "  ", "steamPassword": "new-password"},
		{"steamUsername": "new-user", "steamPassword": ""},
	} {
		invalid, _ := doJSON(t, handler, http.MethodPut, "/api/instances/stardew/steam-credentials", body, adminCookie)
		if invalid.Code != http.StatusBadRequest {
			t.Fatalf("invalid credential update %#v returned %d: %s", body, invalid.Code, invalid.Body.String())
		}
	}

	activeJob, err := store.CreateJob(context.Background(), storage.CreateJobParams{
		Type: "stardew_steam_auth", TargetType: "instance", TargetID: storage.DefaultInstanceID,
	})
	if err != nil {
		t.Fatalf("seed active credential consumer: %v", err)
	}
	busy, _ := doJSON(t, handler, http.MethodPut, "/api/instances/stardew/steam-credentials", map[string]string{
		"steamUsername": "new-user", "steamPassword": "new-password",
	}, adminCookie)
	if busy.Code != http.StatusConflict || !strings.Contains(busy.Body.String(), "steam_credentials_in_use") {
		t.Fatalf("active credential consumer returned %d: %s", busy.Code, busy.Body.String())
	}
	if _, err := store.FailJob(context.Background(), activeJob.ID, "fixture complete"); err != nil {
		t.Fatalf("finish active credential consumer fixture: %v", err)
	}

	updated, _ := doJSON(t, handler, http.MethodPut, "/api/instances/stardew/steam-credentials", map[string]string{
		"steamUsername": "  new-user  ", "steamPassword": "new-password",
	}, adminCookie)
	if updated.Code != http.StatusOK {
		t.Fatalf("credential update returned %d: %s", updated.Code, updated.Body.String())
	}
	if strings.Contains(updated.Body.String(), "new-password") || strings.Contains(updated.Body.String(), "old-password") {
		t.Fatalf("credential response exposed a password: %s", updated.Body.String())
	}
	var response steamCredentialsResponse
	if err := json.Unmarshal(updated.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.OK || response.InstanceID != storage.DefaultInstanceID || response.State != storage.InstanceStateStopped ||
		!response.SteamInviteEnabled || response.SteamInviteAuthState != sjconfig.SteamInviteAuthStateReady || !response.SteamAuthLoggedIn {
		t.Fatalf("unexpected credential response: %#v", response)
	}

	after, err := sjconfig.ReadEnvFile(envPath)
	if err != nil {
		t.Fatal(err)
	}
	if after["STEAM_USERNAME"] != "new-user" || after["STEAM_PASSWORD"] != "new-password" {
		t.Fatalf("updated credentials = %q/%q", after["STEAM_USERNAME"], after["STEAM_PASSWORD"])
	}
	for key, want := range before {
		if key == "STEAM_USERNAME" || key == "STEAM_PASSWORD" {
			continue
		}
		if got := after[key]; got != want {
			t.Fatalf("credential update changed %s = %q, want %q", key, got, want)
		}
	}
	listed, err := store.ListJobs(context.Background(), storage.ListJobsFilter{IsAdmin: true, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].ID != activeJob.ID || listed[0].Status != storage.JobStatusFailed {
		t.Fatalf("credential update changed job inventory: %#v", listed)
	}
}

func TestSteamCredentialsUpdateRejectsRunningDockerServer(t *testing.T) {
	handler, store, dataDir, cleanup := newDockerTestHandlerWithStore(t, fakeDockerService{
		instanceState: storage.InstanceStateStopped,
		psResult: paneldocker.ComposePsResult{Services: []paneldocker.ComposeService{{
			Service: "server", State: "running", Status: "Up 10 seconds",
		}}},
	})
	defer cleanup()
	adminCookie := setupDockerAdmin(t, handler)
	setDockerTestInstanceState(t, store, storage.InstanceStateStopped)
	instanceDir := filepath.Join(dataDir, "instances", storage.DefaultInstanceID)
	if err := os.MkdirAll(instanceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	envPath := filepath.Join(instanceDir, ".env")
	if err := sjconfig.UpdateEnvFile(envPath, map[string]string{
		"STEAM_USERNAME": "old-user", "STEAM_PASSWORD": "old-password",
	}); err != nil {
		t.Fatal(err)
	}

	response, _ := doJSON(t, handler, http.MethodPut, "/api/instances/stardew/steam-credentials", map[string]string{
		"steamUsername": "new-user", "steamPassword": "new-password",
	}, adminCookie)
	if response.Code != http.StatusConflict {
		t.Fatalf("running Docker credential update returned %d: %s", response.Code, response.Body.String())
	}
	var payload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Error.Code != "server_running" {
		t.Fatalf("running Docker credential error = %q", payload.Error.Code)
	}
	after, err := sjconfig.ReadEnvFile(envPath)
	if err != nil {
		t.Fatal(err)
	}
	if after["STEAM_USERNAME"] != "old-user" || after["STEAM_PASSWORD"] != "old-password" {
		t.Fatalf("running refusal changed credentials = %q/%q", after["STEAM_USERNAME"], after["STEAM_PASSWORD"])
	}
}

func TestSteamCredentialsUpdateRequiresCompletedBaseInstallationWithoutEnvSideEffects(t *testing.T) {
	t.Run("unprepared instance does not create sparse env", func(t *testing.T) {
		handler, store, dataDir, cleanup := newDockerTestHandlerWithStore(t, fakeDockerService{})
		defer cleanup()
		adminCookie := setupDockerAdmin(t, handler)
		envPath := filepath.Join(dataDir, "instances", storage.DefaultInstanceID, ".env")

		response, _ := doJSON(t, handler, http.MethodPut, "/api/instances/stardew/steam-credentials", map[string]string{
			"steamUsername": "new-user", "steamPassword": "new-password",
		}, adminCookie)
		assertSteamCredentialsInstallationRequired(t, response)
		if _, err := os.Stat(envPath); !os.IsNotExist(err) {
			t.Fatalf("unprepared credential update materialized .env: %v", err)
		}
		listed, err := store.ListJobs(context.Background(), storage.ListJobsFilter{IsAdmin: true, Limit: 100})
		if err != nil {
			t.Fatal(err)
		}
		if len(listed) != 0 {
			t.Fatalf("unprepared credential update created %d jobs", len(listed))
		}
	})

	t.Run("prepared but uninstalled instance does not modify env", func(t *testing.T) {
		handler, store, dataDir, cleanup := newDockerTestHandlerWithStore(t, fakeDockerService{})
		defer cleanup()
		adminCookie := setupDockerAdmin(t, handler)
		prepared, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/prepare", nil, adminCookie)
		if prepared.Code != http.StatusOK {
			t.Fatalf("prepare returned %d: %s", prepared.Code, prepared.Body.String())
		}
		envPath := filepath.Join(dataDir, "instances", storage.DefaultInstanceID, ".env")
		before, err := os.ReadFile(envPath)
		if err != nil {
			t.Fatal(err)
		}

		response, _ := doJSON(t, handler, http.MethodPut, "/api/instances/stardew/steam-credentials", map[string]string{
			"steamUsername": "new-user", "steamPassword": "new-password",
		}, adminCookie)
		assertSteamCredentialsInstallationRequired(t, response)
		after, err := os.ReadFile(envPath)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(after, before) {
			t.Fatalf("uninstalled credential update changed prepared .env")
		}
		listed, err := store.ListJobs(context.Background(), storage.ListJobsFilter{IsAdmin: true, Limit: 100})
		if err != nil {
			t.Fatal(err)
		}
		if len(listed) != 0 {
			t.Fatalf("uninstalled credential update created %d jobs", len(listed))
		}
	})
}

func assertSteamCredentialsInstallationRequired(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()
	if response.Code != http.StatusConflict {
		t.Fatalf("uninstalled credential update returned %d: %s", response.Code, response.Body.String())
	}
	var payload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Error.Code != "installation_required" {
		t.Fatalf("uninstalled credential error = %q", payload.Error.Code)
	}
}

func TestInstallRequestNoLongerExposesForceReauth(t *testing.T) {
	if _, ok := reflect.TypeOf(installRequestBody{}).FieldByName("ForceReauth"); ok {
		t.Fatal("base install request must not expose the removed ForceReauth bypass")
	}
}

func TestSteamAuthLoginRequiresAdminAndCompletedBaseInstall(t *testing.T) {
	handler, _, dataDir, cleanup := newDockerTestHandlerWithStore(t, fakeDockerService{})
	defer cleanup()
	adminCookie := setupDockerAdmin(t, handler)
	created, _ := doJSON(t, handler, http.MethodPost, "/api/users", map[string]string{
		"username": "player", "password": "player-password", "role": "user",
	}, adminCookie)
	if created.Code != http.StatusCreated {
		t.Fatalf("create user returned %d: %s", created.Code, created.Body.String())
	}
	login, playerCookie := doJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "player", "password": "player-password",
	}, nil)
	if login.Code != http.StatusOK {
		t.Fatalf("player login returned %d: %s", login.Code, login.Body.String())
	}

	forbidden, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/steam-auth/login", nil, playerCookie)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("ordinary user authorization returned %d: %s", forbidden.Code, forbidden.Body.String())
	}
	fresh, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/steam-auth/login", nil, adminCookie)
	if fresh.Code != http.StatusConflict {
		t.Fatalf("fresh instance authorization returned %d: %s", fresh.Code, fresh.Body.String())
	}
	var payload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(fresh.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Error.Code != "installation_required" {
		t.Fatalf("fresh instance error = %q, want installation_required", payload.Error.Code)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "instances", storage.DefaultInstanceID, ".env")); !os.IsNotExist(err) {
		t.Fatalf("fresh authorization request must not materialize invite env, stat err=%v", err)
	}
}

func TestSteamAuthLoginUsesDockerTruthForRunningServer(t *testing.T) {
	fake := fakeDockerService{
		instanceState: storage.InstanceStateGameInstalled,
		psResult: paneldocker.ComposePsResult{Services: []paneldocker.ComposeService{{
			Service: "server", State: "running", Status: "Up 10 seconds",
		}}},
	}
	handler, store, _, cleanup := newDockerTestHandlerWithStore(t, fake)
	defer cleanup()
	adminCookie := setupDockerAdmin(t, handler)
	if _, err := store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID: storage.DefaultInstanceID, State: storage.InstanceStateGameInstalled,
		StateMessage: "installed", DriverPhase: "game_installed", DriverPayload: `{}`,
	}); err != nil {
		t.Fatal(err)
	}

	response, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/steam-auth/login", nil, adminCookie)
	if response.Code != http.StatusConflict {
		t.Fatalf("running Docker server authorization returned %d: %s", response.Code, response.Body.String())
	}
	var payload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Error.Code != "server_running" {
		t.Fatalf("running Docker server error = %q, want server_running", payload.Error.Code)
	}
}

func TestSteamAuthLoginAlreadyReadyCreatesNoJobAndDoesNotProbeDocker(t *testing.T) {
	var psCalls atomic.Int32
	handler, store, dataDir, cleanup := newDockerTestHandlerWithStore(t, fakeDockerService{psCalls: &psCalls})
	defer cleanup()
	adminCookie := setupDockerAdmin(t, handler)
	if _, err := store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID: storage.DefaultInstanceID, State: storage.InstanceStateGameInstalled,
		StateMessage: "installed", DriverPhase: "game_installed", DriverPayload: `{}`,
	}); err != nil {
		t.Fatal(err)
	}
	instanceDir := filepath.Join(dataDir, "instances", storage.DefaultInstanceID)
	if err := os.MkdirAll(instanceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	envBefore := []byte(strings.Join([]string{
		"STEAM_USERNAME=old-user",
		"STEAM_PASSWORD=old-password",
		"VNC_PASSWORD=vnc-password",
		"STEAM_INVITE_ENABLED=true",
		"STEAM_AUTH_COMPLETED=true",
		"STEAM_INVITE_AUTH_STATE=ready",
		"STEAM_REFRESH_TOKEN=existing-auth-token",
		"",
	}, "\n"))
	if err := os.WriteFile(filepath.Join(instanceDir, ".env"), envBefore, 0o600); err != nil {
		t.Fatal(err)
	}

	response, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/steam-auth/login", nil, adminCookie)
	if response.Code != http.StatusConflict {
		t.Fatalf("already-ready authorization returned %d: %s", response.Code, response.Body.String())
	}
	var payload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Error.Code != "steam_invite_already_ready" {
		t.Fatalf("already-ready authorization error = %q", payload.Error.Code)
	}
	listed, err := store.ListJobs(context.Background(), storage.ListJobsFilter{IsAdmin: true, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 0 || psCalls.Load() != 0 {
		t.Fatalf("already-ready authorization caused side effects: jobs=%d dockerPs=%d", len(listed), psCalls.Load())
	}
	envAfter, err := os.ReadFile(filepath.Join(instanceDir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if string(envAfter) != string(envBefore) {
		t.Fatalf("already-ready authorization changed the saved session:\n%s", envAfter)
	}
}

func TestSteamAuthGuardInputPreservesInstalledLifecycleState(t *testing.T) {
	releaseAuth := make(chan struct{}, 1)
	removedByVolumes := make(chan []string, 1)
	removedVolumes := make(chan []string, 1)
	defer func() {
		select {
		case releaseAuth <- struct{}{}:
		default:
		}
	}()
	handler, store, dataDir, cleanup := newDockerTestHandlerWithStore(t, fakeDockerService{
		steamAuthGate:    releaseAuth,
		removedByVolumes: removedByVolumes,
		removedVolumes:   removedVolumes,
	})
	defer cleanup()
	adminCookie := setupDockerAdmin(t, handler)
	if _, err := store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID: storage.DefaultInstanceID, State: storage.InstanceStateGameInstalled,
		StateMessage: "installed", DriverPhase: "game_installed", DriverPayload: `{}`,
	}); err != nil {
		t.Fatal(err)
	}
	instanceDir := filepath.Join(dataDir, "instances", storage.DefaultInstanceID)
	if err := os.MkdirAll(instanceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(instanceDir, ".env"), []byte(
		"STEAM_USERNAME=test-user\nSTEAM_PASSWORD=test-password\nSTEAMCMD_AUTH_COMPLETED=true\n",
	), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(instanceDir, "docker-compose.yml"), []byte("services:\n  server: {}\n  steam-auth: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	started, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/steam-auth/login", nil, adminCookie)
	if started.Code != http.StatusAccepted {
		t.Fatalf("start invite authorization returned %d: %s", started.Code, started.Body.String())
	}
	var startedPayload struct {
		JobID string `json:"jobId"`
	}
	if err := json.Unmarshal(started.Body.Bytes(), &startedPayload); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		instance, err := store.GetInstance(context.Background(), storage.DefaultInstanceID)
		if err != nil {
			t.Fatal(err)
		}
		if instance.DriverPhase == "auth_method_required" {
			if instance.State != storage.InstanceStateGameInstalled {
				t.Fatalf("active invite authorization changed base state to %q", instance.State)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for auth method phase; state=%s phase=%s", instance.State, instance.DriverPhase)
		}
		time.Sleep(10 * time.Millisecond)
	}
	for label, gotCh := range map[string]<-chan []string{
		"session holder": removedByVolumes,
		"session volume": removedVolumes,
	} {
		select {
		case names := <-gotCh:
			want := storage.DefaultInstanceID + "_steam-session"
			if len(names) != 1 || names[0] != want {
				t.Fatalf("force reauth %s cleanup = %#v, want only %q", label, names, want)
			}
		case <-time.After(time.Second):
			t.Fatalf("force reauth did not clean exact Auth %s", label)
		}
	}
	envBytes, err := os.ReadFile(filepath.Join(instanceDir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(envBytes), "STEAMCMD_AUTH_COMPLETED=true") {
		t.Fatal("invite reauthorization must preserve the SteamCMD authorization cache flag")
	}

	selected, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/steam-guard/input", map[string]string{
		"jobId": startedPayload.JobID,
		"input": "1",
	}, adminCookie)
	if selected.Code != http.StatusOK {
		t.Fatalf("select credential auth returned %d: %s", selected.Code, selected.Body.String())
	}
	instance, err := store.GetInstance(context.Background(), storage.DefaultInstanceID)
	if err != nil {
		t.Fatal(err)
	}
	if instance.State != storage.InstanceStateGameInstalled || instance.DriverPhase != "steam_auth_running" {
		t.Fatalf("invite auth method update = state:%s phase:%s, want game_installed/steam_auth_running", instance.State, instance.DriverPhase)
	}
	releaseAuth <- struct{}{}
	deadline = time.Now().Add(2 * time.Second)
	for {
		job, err := store.GetJob(context.Background(), startedPayload.JobID)
		if err != nil {
			t.Fatal(err)
		}
		if job.Status == storage.JobStatusFailed || job.Status == storage.JobStatusSucceeded {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for auth job completion; status=%s", job.Status)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestSteamAuthGuardInputRejectsForeignAndTerminalJobsBeforeStateWrite(t *testing.T) {
	tests := []struct {
		name     string
		targetID string
		terminal bool
	}{
		{name: "foreign instance", targetID: "another-instance"},
		{name: "terminal job", targetID: storage.DefaultInstanceID, terminal: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler, store, _, cleanup := newDockerTestHandlerWithStore(t, fakeDockerService{})
			defer cleanup()
			adminCookie := setupDockerAdmin(t, handler)
			if _, err := store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
				ID: storage.DefaultInstanceID, State: storage.InstanceStateGameInstalled,
				StateMessage: "installed", DriverPhase: "auth_method_required", DriverPayload: `{"kept":true}`,
			}); err != nil {
				t.Fatal(err)
			}
			job, err := store.CreateJob(context.Background(), storage.CreateJobParams{
				Type: "stardew_steam_auth", TargetType: "instance", TargetID: test.targetID,
			})
			if err != nil {
				t.Fatal(err)
			}
			if test.terminal {
				if _, err := store.StartJob(context.Background(), job.ID); err != nil {
					t.Fatal(err)
				}
				if _, err := store.FailJob(context.Background(), job.ID, "terminal fixture"); err != nil {
					t.Fatal(err)
				}
			}

			response, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/steam-guard/input", map[string]string{
				"jobId": job.ID,
				"input": "1",
			}, adminCookie)
			if response.Code != http.StatusConflict {
				t.Fatalf("guard input returned %d: %s", response.Code, response.Body.String())
			}
			instance, err := store.GetInstance(context.Background(), storage.DefaultInstanceID)
			if err != nil {
				t.Fatal(err)
			}
			if instance.State != storage.InstanceStateGameInstalled || instance.DriverPhase != "auth_method_required" || instance.DriverPayload != `{"kept":true}` {
				t.Fatalf("rejected guard input changed instance: %#v", instance)
			}
		})
	}
}

func TestInviteCodeDisabledReturnsStableDTOWithoutDocker(t *testing.T) {
	var psCalls atomic.Int32
	handler, _, _, cleanup := newDockerTestHandlerWithStore(t, fakeDockerService{psCalls: &psCalls})
	defer cleanup()
	adminCookie := setupDockerAdmin(t, handler)

	response, _ := doJSON(t, handler, http.MethodGet, "/api/instances/stardew/invite-code", nil, adminCookie)
	if response.Code != http.StatusOK {
		t.Fatalf("disabled invite response = %d: %s", response.Code, response.Body.String())
	}
	var invite struct {
		SteamInviteEnabled bool   `json:"steamInviteEnabled"`
		Status             string `json:"status"`
		InviteCode         string `json:"inviteCode"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &invite); err != nil {
		t.Fatal(err)
	}
	if invite.SteamInviteEnabled || invite.Status != "disabled" || invite.InviteCode != "" {
		t.Fatalf("disabled invite DTO = %#v", invite)
	}
	if psCalls.Load() != 0 {
		t.Fatalf("disabled invite lookup probed Docker %d times", psCalls.Load())
	}

	state, _ := doJSON(t, handler, http.MethodGet, "/api/instances/stardew/state", nil, adminCookie)
	if state.Code != http.StatusOK {
		t.Fatalf("state response = %d: %s", state.Code, state.Body.String())
	}
	var statePayload struct {
		SteamInviteEnabled bool `json:"steamInviteEnabled"`
	}
	if err := json.Unmarshal(state.Body.Bytes(), &statePayload); err != nil {
		t.Fatal(err)
	}
	if statePayload.SteamInviteEnabled {
		t.Fatal("state must expose authoritative disabled invite intent")
	}
}

func TestInviteCodeStartupAndRuntimeStatuses(t *testing.T) {
	tests := []struct {
		name          string
		state         string
		execResult    paneldocker.CommandResult
		execErr       error
		wantStatus    string
		wantCode      string
		warmupExpired bool
	}{
		{
			name: "starting read failure is still waiting", state: storage.InstanceStateStarting,
			execResult: paneldocker.CommandResult{ExitCode: 1}, execErr: fmt.Errorf("server container is not ready"),
			wantStatus: "generating",
		},
		{
			name: "recent running read failure stays in bounded warmup", state: storage.InstanceStateRunning,
			execResult: paneldocker.CommandResult{ExitCode: 1}, execErr: fmt.Errorf("server container exec failed"),
			wantStatus: "generating",
		},
		{
			name: "expired running read failure is unavailable", state: storage.InstanceStateRunning,
			execResult: paneldocker.CommandResult{ExitCode: 1}, execErr: fmt.Errorf("server container exec failed"),
			wantStatus: "auth_unavailable", warmupExpired: true,
		},
		{
			name: "running without generated file is waiting", state: storage.InstanceStateRunning,
			execResult: paneldocker.CommandResult{ExitCode: 0}, wantStatus: "generating",
		},
		{
			name: "running with code is ready", state: storage.InstanceStateRunning,
			execResult: paneldocker.CommandResult{ExitCode: 0, Stdout: "LOCAL-CODE\n"},
			wantStatus: "ready", wantCode: "LOCAL-CODE",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler, store, dataDir, cleanup := newDockerTestHandlerWithStore(t, fakeDockerService{
				execFunc: func(context.Context, string, string, string, ...string) (paneldocker.CommandResult, error) {
					return test.execResult, test.execErr
				},
			})
			defer cleanup()
			adminCookie := setupDockerAdmin(t, handler)
			driverPayload := `{}`
			if test.state == storage.InstanceStateRunning {
				warmupStartedAt := time.Now().UTC()
				if test.warmupExpired {
					warmupStartedAt = warmupStartedAt.Add(-steamInviteRuntimeWarmupWindow - time.Second)
				}
				driverPayload = steamInviteWarmupDriverPayload(warmupStartedAt.Format(time.RFC3339Nano))
			}
			if _, err := store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
				ID: storage.DefaultInstanceID, State: test.state,
				StateMessage: "starting fixture", DriverPhase: test.state, DriverPayload: driverPayload,
			}); err != nil {
				t.Fatal(err)
			}
			instanceDir := filepath.Join(dataDir, "instances", storage.DefaultInstanceID)
			if err := os.MkdirAll(instanceDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(instanceDir, ".env"), []byte(strings.Join([]string{
				"STEAM_INVITE_ENABLED=true",
				"STEAM_AUTH_COMPLETED=true",
				"STEAM_INVITE_AUTH_STATE=ready",
				"",
			}, "\n")), 0o600); err != nil {
				t.Fatal(err)
			}

			response, _ := doJSON(t, handler, http.MethodGet, "/api/instances/stardew/invite-code", nil, adminCookie)
			if response.Code != http.StatusOK {
				t.Fatalf("invite response = %d: %s", response.Code, response.Body.String())
			}
			var invite struct {
				Status     string `json:"status"`
				InviteCode string `json:"inviteCode"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &invite); err != nil {
				t.Fatal(err)
			}
			if invite.Status != test.wantStatus || invite.InviteCode != test.wantCode {
				t.Fatalf("invite result = %#v, want status=%q code=%q", invite, test.wantStatus, test.wantCode)
			}
		})
	}
}

func TestSteamInviteFailureStatusWarmupBoundary(t *testing.T) {
	now := time.Date(2026, 8, 27, 9, 30, 0, 0, time.UTC)
	tests := []struct {
		name      string
		instance  storage.Instance
		wantState string
	}{
		{
			name:      "starting is generating regardless of timestamp",
			instance:  storage.Instance{State: storage.InstanceStateStarting, DriverPayload: steamInviteWarmupDriverPayload("invalid")},
			wantState: "generating",
		},
		{
			name:      "fresh running transition is generating",
			instance:  storage.Instance{State: storage.InstanceStateRunning, UpdatedAt: now.Add(-time.Hour).Format(time.RFC3339Nano), DriverPayload: steamInviteWarmupDriverPayload(now.Format(time.RFC3339Nano))},
			wantState: "generating",
		},
		{
			name:      "running just inside warmup is generating",
			instance:  storage.Instance{State: storage.InstanceStateRunning, DriverPayload: steamInviteWarmupDriverPayload(now.Add(-steamInviteRuntimeWarmupWindow + time.Nanosecond).Format(time.RFC3339Nano))},
			wantState: "generating",
		},
		{
			name:      "running at warmup boundary is unavailable",
			instance:  storage.Instance{State: storage.InstanceStateRunning, DriverPayload: steamInviteWarmupDriverPayload(now.Add(-steamInviteRuntimeWarmupWindow).Format(time.RFC3339Nano))},
			wantState: "auth_unavailable",
		},
		{
			name:      "unrelated recent updated at does not extend old warmup",
			instance:  storage.Instance{State: storage.InstanceStateRunning, UpdatedAt: now.Format(time.RFC3339Nano), DriverPayload: steamInviteWarmupDriverPayload(now.Add(-steamInviteRuntimeWarmupWindow - time.Second).Format(time.RFC3339Nano))},
			wantState: "auth_unavailable",
		},
		{
			name:      "invalid transition timestamp fails closed",
			instance:  storage.Instance{State: storage.InstanceStateRunning, DriverPayload: steamInviteWarmupDriverPayload("invalid")},
			wantState: "auth_unavailable",
		},
		{
			name:      "future transition timestamp fails closed",
			instance:  storage.Instance{State: storage.InstanceStateRunning, DriverPayload: steamInviteWarmupDriverPayload(now.Add(time.Second).Format(time.RFC3339Nano))},
			wantState: "auth_unavailable",
		},
		{
			name:      "stopped remains unavailable",
			instance:  storage.Instance{State: storage.InstanceStateStopped, DriverPayload: steamInviteWarmupDriverPayload(now.Format(time.RFC3339Nano))},
			wantState: "auth_unavailable",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := steamInviteFailureStatus(test.instance, now); got != test.wantState {
				t.Fatalf("steamInviteFailureStatus() = %q, want %q", got, test.wantState)
			}
		})
	}
}

func steamInviteWarmupDriverPayload(startedAt string) string {
	payload, _ := json.Marshal(map[string]string{"steam_invite_warmup_started_at": startedAt})
	return string(payload)
}
