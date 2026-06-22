package web

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
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
