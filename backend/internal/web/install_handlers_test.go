package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

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
