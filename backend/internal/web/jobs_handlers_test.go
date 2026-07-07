package web

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func TestJobsAPIPermissionsAndLifecycle(t *testing.T) {
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

	created, _ := doJSON(t, handler, http.MethodPost, "/api/users", map[string]string{
		"username": "player",
		"password": "player-password",
		"role":     "user",
	}, adminCookie)
	if created.Code != http.StatusCreated {
		t.Fatalf("create user returned %d: %s", created.Code, created.Body.String())
	}

	login, userCookie := doJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "player",
		"password": "player-password",
	}, nil)
	if login.Code != http.StatusOK {
		t.Fatalf("player login returned %d: %s", login.Code, login.Body.String())
	}

	forbiddenCreate, _ := doJSON(t, handler, http.MethodPost, "/api/jobs/test", nil, userCookie)
	if forbiddenCreate.Code != http.StatusForbidden {
		t.Fatalf("ordinary user create job returned %d", forbiddenCreate.Code)
	}

	createdJob, _ := doJSON(t, handler, http.MethodPost, "/api/jobs/test-fail", nil, adminCookie)
	if createdJob.Code != http.StatusAccepted {
		t.Fatalf("create test-fail job returned %d: %s", createdJob.Code, createdJob.Body.String())
	}
	jobID := decodeJobID(t, createdJob.Body.Bytes())
	waitForHTTPJobStatus(t, handler, adminCookie, jobID, "failed")

	adminJobs, _ := doJSON(t, handler, http.MethodGet, "/api/jobs", nil, adminCookie)
	if adminJobs.Code != http.StatusOK {
		t.Fatalf("admin list jobs returned %d: %s", adminJobs.Code, adminJobs.Body.String())
	}

	userJobDetail, _ := doJSON(t, handler, http.MethodGet, "/api/jobs/"+jobID, nil, userCookie)
	if userJobDetail.Code != http.StatusForbidden {
		t.Fatalf("ordinary user read admin job returned %d", userJobDetail.Code)
	}

	logs, _ := doJSON(t, handler, http.MethodGet, "/api/jobs/"+jobID+"/logs", nil, adminCookie)
	if logs.Code != http.StatusOK {
		t.Fatalf("job logs returned %d: %s", logs.Code, logs.Body.String())
	}

	cancel, _ := doJSON(t, handler, http.MethodPost, "/api/jobs/"+jobID+"/cancel", nil, adminCookie)
	if cancel.Code != http.StatusNotImplemented {
		t.Fatalf("cancel returned %d", cancel.Code)
	}
}

func TestJobsAPIIncludesDisplayName(t *testing.T) {
	handler, store, closeStore := newTestHandlerWithStore(t)
	defer closeStore()

	setup, adminCookie := doJSON(t, handler, http.MethodPost, "/api/setup/admin", map[string]string{
		"username":        "admin",
		"password":        "admin-password",
		"confirmPassword": "admin-password",
	}, nil)
	if setup.Code != http.StatusOK {
		t.Fatalf("setup admin returned %d: %s", setup.Code, setup.Body.String())
	}

	if _, err := store.CreateJob(context.Background(), storage.CreateJobParams{
		Type:        "mod_remote_install",
		DisplayName: "Farm Type Manager (FTM) · mod_remote_install",
		TargetType:  "instance",
		TargetID:    storage.DefaultInstanceID,
	}); err != nil {
		t.Fatalf("create job: %v", err)
	}

	res, _ := doJSON(t, handler, http.MethodGet, "/api/jobs", nil, adminCookie)
	if res.Code != http.StatusOK {
		t.Fatalf("list jobs returned %d: %s", res.Code, res.Body.String())
	}
	var payload struct {
		Jobs []struct {
			Type        string  `json:"type"`
			DisplayName *string `json:"displayName"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode jobs: %v", err)
	}
	if len(payload.Jobs) != 1 {
		t.Fatalf("len(jobs) = %d, want 1", len(payload.Jobs))
	}
	if payload.Jobs[0].Type != "mod_remote_install" {
		t.Fatalf("type = %q, want mod_remote_install", payload.Jobs[0].Type)
	}
	if payload.Jobs[0].DisplayName == nil || *payload.Jobs[0].DisplayName != "Farm Type Manager (FTM) · mod_remote_install" {
		t.Fatalf("displayName = %#v, want mod name", payload.Jobs[0].DisplayName)
	}
}

func TestModInstallJobDisplayNameUsesModNameFirst(t *testing.T) {
	got := modInstallJobDisplayName("mod_remote_install", sj.NexusModSearchResult{
		ModID: 7286,
		Name:  "Ridgeside Village",
	})
	if got != "Ridgeside Village · mod_remote_install" {
		t.Fatalf("display name = %q, want mod name first", got)
	}
}

func TestAdminCanClearJobCenterWhenNoActiveJobs(t *testing.T) {
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

	activeJob, _ := doJSON(t, handler, http.MethodPost, "/api/jobs/test", nil, adminCookie)
	if activeJob.Code != http.StatusAccepted {
		t.Fatalf("create active job returned %d: %s", activeJob.Code, activeJob.Body.String())
	}
	activeJobID := decodeJobID(t, activeJob.Body.Bytes())

	blocked, _ := doJSON(t, handler, http.MethodDelete, "/api/jobs", nil, adminCookie)
	if blocked.Code != http.StatusConflict {
		t.Fatalf("clear with active job returned %d: %s", blocked.Code, blocked.Body.String())
	}

	waitForHTTPJobStatus(t, handler, adminCookie, activeJobID, "succeeded")

	cleared, _ := doJSON(t, handler, http.MethodDelete, "/api/jobs", nil, adminCookie)
	if cleared.Code != http.StatusOK {
		t.Fatalf("clear jobs returned %d: %s", cleared.Code, cleared.Body.String())
	}

	list, _ := doJSON(t, handler, http.MethodGet, "/api/jobs", nil, adminCookie)
	if list.Code != http.StatusOK {
		t.Fatalf("list jobs returned %d: %s", list.Code, list.Body.String())
	}
	var payload struct {
		Jobs []any `json:"jobs"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode jobs list: %v", err)
	}
	if len(payload.Jobs) != 0 {
		t.Fatalf("expected no jobs after clear, got %d", len(payload.Jobs))
	}
}

func TestAdminCanClearJobErrorLogs(t *testing.T) {
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

	created, _ := doJSON(t, handler, http.MethodPost, "/api/users", map[string]string{
		"username": "player",
		"password": "player-password",
		"role":     "user",
	}, adminCookie)
	if created.Code != http.StatusCreated {
		t.Fatalf("create user returned %d: %s", created.Code, created.Body.String())
	}
	login, userCookie := doJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "player",
		"password": "player-password",
	}, nil)
	if login.Code != http.StatusOK {
		t.Fatalf("player login returned %d: %s", login.Code, login.Body.String())
	}

	createdJob, _ := doJSON(t, handler, http.MethodPost, "/api/jobs/test-fail", nil, adminCookie)
	if createdJob.Code != http.StatusAccepted {
		t.Fatalf("create test-fail job returned %d: %s", createdJob.Code, createdJob.Body.String())
	}
	jobID := decodeJobID(t, createdJob.Body.Bytes())
	waitForHTTPJobStatus(t, handler, adminCookie, jobID, "failed")

	blocked, _ := doJSON(t, handler, http.MethodDelete, "/api/jobs/error-logs", nil, userCookie)
	if blocked.Code != http.StatusForbidden {
		t.Fatalf("ordinary user clear error logs returned %d", blocked.Code)
	}

	cleared, _ := doJSON(t, handler, http.MethodDelete, "/api/jobs/error-logs", nil, adminCookie)
	if cleared.Code != http.StatusOK {
		t.Fatalf("clear error logs returned %d: %s", cleared.Code, cleared.Body.String())
	}
	var clearPayload struct {
		Deleted         int64 `json:"deleted"`
		MessagesCleared int64 `json:"messagesCleared"`
	}
	if err := json.Unmarshal(cleared.Body.Bytes(), &clearPayload); err != nil {
		t.Fatalf("decode clear error logs response: %v", err)
	}
	if clearPayload.Deleted == 0 || clearPayload.MessagesCleared == 0 {
		t.Fatalf("expected deleted logs and cleared messages, got %#v", clearPayload)
	}

	detail, _ := doJSON(t, handler, http.MethodGet, "/api/jobs/"+jobID, nil, adminCookie)
	if detail.Code != http.StatusOK {
		t.Fatalf("job detail returned %d: %s", detail.Code, detail.Body.String())
	}
	var detailPayload struct {
		Job struct {
			ErrorMessage *string `json:"errorMessage"`
		} `json:"job"`
	}
	if err := json.Unmarshal(detail.Body.Bytes(), &detailPayload); err != nil {
		t.Fatalf("decode job detail: %v", err)
	}
	if detailPayload.Job.ErrorMessage != nil {
		t.Fatalf("expected errorMessage to be cleared, got %q", *detailPayload.Job.ErrorMessage)
	}

	logs, _ := doJSON(t, handler, http.MethodGet, "/api/jobs/"+jobID+"/logs", nil, adminCookie)
	if logs.Code != http.StatusOK {
		t.Fatalf("job logs returned %d: %s", logs.Code, logs.Body.String())
	}
	var logsPayload struct {
		Logs []struct {
			Level string `json:"level"`
		} `json:"logs"`
	}
	if err := json.Unmarshal(logs.Body.Bytes(), &logsPayload); err != nil {
		t.Fatalf("decode job logs: %v", err)
	}
	for _, logLine := range logsPayload.Logs {
		if logLine.Level == "error" {
			t.Fatal("expected error log lines to be cleared")
		}
	}
}

func TestStardewStateRequiresLogin(t *testing.T) {
	handler, closeStore := newTestHandler(t)
	defer closeStore()

	blocked, _ := doJSON(t, handler, http.MethodGet, "/api/instances/stardew/state", nil, nil)
	if blocked.Code != http.StatusServiceUnavailable {
		t.Fatalf("state before setup returned %d", blocked.Code)
	}

	setup, adminCookie := doJSON(t, handler, http.MethodPost, "/api/setup/admin", map[string]string{
		"username":        "admin",
		"password":        "admin-password",
		"confirmPassword": "admin-password",
	}, nil)
	if setup.Code != http.StatusOK {
		t.Fatalf("setup admin returned %d: %s", setup.Code, setup.Body.String())
	}
	state, _ := doJSON(t, handler, http.MethodGet, "/api/instances/stardew/state", nil, adminCookie)
	if state.Code != http.StatusOK {
		t.Fatalf("state returned %d: %s", state.Code, state.Body.String())
	}
}

func decodeJobID(t *testing.T, body []byte) string {
	t.Helper()
	var payload struct {
		Job struct {
			ID string `json:"id"`
		} `json:"job"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode job response: %v", err)
	}
	if payload.Job.ID == "" {
		t.Fatal("job id is empty")
	}
	return payload.Job.ID
}

func waitForHTTPJobStatus(t *testing.T, handler http.Handler, cookie *http.Cookie, jobID string, status string) {
	t.Helper()
	deadline := time.Now().Add(7 * time.Second)
	for time.Now().Before(deadline) {
		response, _ := doJSON(t, handler, http.MethodGet, "/api/jobs/"+jobID, nil, cookie)
		if response.Code != http.StatusOK {
			t.Fatalf("get job returned %d: %s", response.Code, response.Body.String())
		}
		var payload struct {
			Job struct {
				Status string `json:"status"`
			} `json:"job"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode job status: %v", err)
		}
		if payload.Job.Status == status {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("job %s did not reach %s", jobID, status)
}
