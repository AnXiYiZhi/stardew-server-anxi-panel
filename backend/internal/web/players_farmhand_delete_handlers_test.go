package web

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

type farmhandDeleteAPIDriver struct {
	intent          *sj.FarmhandDeleteIntentResult
	lastRequest     sj.FarmhandDeleteRequest
	canceled        string
	recoveryRequest string
}

func (d *farmhandDeleteAPIDriver) ID() string                                       { return storage.DefaultDriverID }
func (d *farmhandDeleteAPIDriver) Name() string                                     { return "Stardew test" }
func (d *farmhandDeleteAPIDriver) Prepare(context.Context, registry.Instance) error { return nil }
func (d *farmhandDeleteAPIDriver) Install(context.Context, registry.InstallRequest) (*registry.Job, error) {
	return &registry.Job{ID: "install"}, nil
}
func (d *farmhandDeleteAPIDriver) Start(context.Context, registry.StartRequest) (*registry.Job, error) {
	return &registry.Job{ID: "start"}, nil
}
func (d *farmhandDeleteAPIDriver) Stop(context.Context, registry.Instance) error    { return nil }
func (d *farmhandDeleteAPIDriver) Restart(context.Context, registry.Instance) error { return nil }
func (d *farmhandDeleteAPIDriver) Status(context.Context, registry.Instance) (*registry.ServerStatus, error) {
	return &registry.ServerStatus{State: storage.InstanceStateRunning}, nil
}
func (d *farmhandDeleteAPIDriver) Logs(context.Context, registry.Instance) (<-chan registry.LogLine, error) {
	logs := make(chan registry.LogLine)
	close(logs)
	return logs, nil
}
func (d *farmhandDeleteAPIDriver) ExecCommand(context.Context, string) (*registry.CommandResult, error) {
	return &registry.CommandResult{}, nil
}
func (d *farmhandDeleteAPIDriver) ListSaves(context.Context, registry.Instance) ([]registry.SaveInfo, error) {
	return nil, nil
}
func (d *farmhandDeleteAPIDriver) UploadSave(context.Context, registry.UploadedFile) error {
	return nil
}
func (d *farmhandDeleteAPIDriver) SelectSave(context.Context, string) error { return nil }
func (d *farmhandDeleteAPIDriver) DeleteSave(context.Context, string) error { return nil }
func (d *farmhandDeleteAPIDriver) ListMods(context.Context, registry.Instance) ([]registry.ModInfo, error) {
	return nil, nil
}
func (d *farmhandDeleteAPIDriver) UploadMod(context.Context, registry.UploadedFile) error { return nil }
func (d *farmhandDeleteAPIDriver) DeleteMod(context.Context, string) error                { return nil }

func (d *farmhandDeleteAPIDriver) DeleteFarmhand(_ context.Context, req sj.FarmhandDeleteRequest) (*sj.FarmhandDeleteSubmission, error) {
	d.lastRequest = req
	d.intent = &sj.FarmhandDeleteIntentResult{
		OperationID: "0123456789abcdef0123456789abcdef", Mode: req.Mode, Status: storage.FarmhandDeleteIntentWaiting,
		PlayerID: req.PlayerID, ExpectedName: req.ExpectedName, ExpectedSaveID: req.ExpectedSave, ExpiresAt: "2026-09-10T12:00:00Z",
	}
	if req.Mode == storage.FarmhandDeleteModeMaintenanceNow {
		d.intent.Status = storage.FarmhandDeleteIntentCountdown
		d.intent.JobID = "delete-job"
	}
	return &sj.FarmhandDeleteSubmission{Status: d.intent.Status, JobID: d.intent.JobID, Intent: d.intent}, nil
}

func (d *farmhandDeleteAPIDriver) GetFarmhandDeleteIntent(context.Context, registry.Instance) (*sj.FarmhandDeleteIntentResult, error) {
	return d.intent, nil
}

func (d *farmhandDeleteAPIDriver) CancelFarmhandDeleteIntent(_ context.Context, _ registry.Instance, operationID string) error {
	d.canceled = operationID
	return nil
}

func (d *farmhandDeleteAPIDriver) RetryFarmhandDeletePersistence(_ context.Context, _ registry.Instance, operationID string, _ int64) (*sj.FarmhandDeleteSubmission, error) {
	d.recoveryRequest = operationID
	return &sj.FarmhandDeleteSubmission{Status: storage.FarmhandDeleteIntentActive, JobID: "recovery-job", Intent: d.intent}, nil
}

func TestFarmhandDeleteAPIWaitImmediateCancelAndRecovery(t *testing.T) {
	dataDir := t.TempDir()
	store, err := storage.Open(context.Background(), config.Config{
		Addr: ":0", DataDir: dataDir, DBPath: filepath.Join(dataDir, "panel.db"), Secret: "test-secret", Version: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{
		ID: storage.DefaultInstanceID, DriverID: storage.DefaultDriverID, Name: "Stardew", DataDir: filepath.Join(dataDir, "instances", storage.DefaultInstanceID),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID: storage.DefaultInstanceID, State: storage.InstanceStateRunning, StateMessage: "running", DriverPhase: "running",
	}); err != nil {
		t.Fatal(err)
	}
	driver := &farmhandDeleteAPIDriver{}
	drivers := registry.New()
	if err := drivers.Register(driver); err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(Deps{
		Config: config.Config{DataDir: dataDir, Secret: "test-secret", Version: "test"}, Store: store, Registry: drivers,
	})
	adminCookie := setupDockerAdmin(t, handler)
	path := "/api/instances/stardew/players/delete-farmhand"

	unauthorized, _ := doJSON(t, handler, http.MethodGet, path, nil, nil)
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized GET = %d, want 401", unauthorized.Code)
	}
	empty, _ := doJSON(t, handler, http.MethodGet, path, nil, adminCookie)
	if empty.Code != http.StatusOK || empty.Body.String() != "{\"intent\":null}\n" {
		t.Fatalf("empty GET = %d %s", empty.Code, empty.Body.String())
	}
	legacyDefault, _ := doJSON(t, handler, http.MethodPost, path, map[string]any{
		"uniqueMultiplayerId": "42", "expectedName": "Leah", "expectedSaveId": "Farm_1", "acknowledged": true,
	}, adminCookie)
	if legacyDefault.Code != http.StatusAccepted || driver.lastRequest.Mode != storage.FarmhandDeleteModeWait {
		t.Fatalf("missing mode did not default to wait: %d %s; request=%+v", legacyDefault.Code, legacyDefault.Body.String(), driver.lastRequest)
	}

	waitResponse, _ := doJSON(t, handler, http.MethodPost, path, map[string]any{
		"uniqueMultiplayerId": "42", "expectedName": "Leah", "expectedSaveId": "Farm_1", "acknowledged": true, "mode": "wait",
	}, adminCookie)
	if waitResponse.Code != http.StatusAccepted || driver.lastRequest.Mode != storage.FarmhandDeleteModeWait {
		t.Fatalf("wait POST = %d %s; request=%+v", waitResponse.Code, waitResponse.Body.String(), driver.lastRequest)
	}
	statusResponse, _ := doJSON(t, handler, http.MethodGet, path, nil, adminCookie)
	if statusResponse.Code != http.StatusOK || driver.intent == nil {
		t.Fatalf("status GET = %d %s", statusResponse.Code, statusResponse.Body.String())
	}
	cancelResponse, _ := doJSON(t, handler, http.MethodDelete, path+"?operationId="+driver.intent.OperationID, nil, adminCookie)
	if cancelResponse.Code != http.StatusOK || driver.canceled != driver.intent.OperationID {
		t.Fatalf("cancel DELETE = %d %s; canceled=%q", cancelResponse.Code, cancelResponse.Body.String(), driver.canceled)
	}

	missingRisk, _ := doJSON(t, handler, http.MethodPost, path, map[string]any{
		"uniqueMultiplayerId": "42", "expectedName": "Leah", "expectedSaveId": "Farm_1", "acknowledged": true, "mode": "maintenance_now",
	}, adminCookie)
	if missingRisk.Code != http.StatusBadRequest {
		t.Fatalf("immediate without risk confirmation = %d %s", missingRisk.Code, missingRisk.Body.String())
	}
	wrongName, _ := doJSON(t, handler, http.MethodPost, path, map[string]any{
		"uniqueMultiplayerId": "42", "expectedName": "Leah", "expectedSaveId": "Farm_1", "acknowledged": true,
		"mode": "maintenance_now", "riskAcknowledged": true, "confirmationName": "Other",
	}, adminCookie)
	if wrongName.Code != http.StatusBadRequest {
		t.Fatalf("immediate with wrong name = %d %s", wrongName.Code, wrongName.Body.String())
	}
	immediate, _ := doJSON(t, handler, http.MethodPost, path, map[string]any{
		"uniqueMultiplayerId": "42", "expectedName": "Leah", "expectedSaveId": "Farm_1", "acknowledged": true,
		"mode": "maintenance_now", "riskAcknowledged": true, "confirmationName": "Leah",
	}, adminCookie)
	if immediate.Code != http.StatusAccepted || driver.lastRequest.Mode != storage.FarmhandDeleteModeMaintenanceNow {
		t.Fatalf("immediate POST = %d %s; request=%+v", immediate.Code, immediate.Body.String(), driver.lastRequest)
	}

	recoveryPath := path + "/recovery"
	invalidRecovery, _ := doJSON(t, handler, http.MethodPost, recoveryPath, map[string]string{
		"operationId": driver.intent.OperationID, "action": "unknown",
	}, adminCookie)
	if invalidRecovery.Code != http.StatusBadRequest {
		t.Fatalf("invalid recovery = %d %s", invalidRecovery.Code, invalidRecovery.Body.String())
	}
	recovery, _ := doJSON(t, handler, http.MethodPost, recoveryPath, map[string]string{
		"operationId": driver.intent.OperationID, "action": "retry_save",
	}, adminCookie)
	if recovery.Code != http.StatusAccepted || driver.recoveryRequest != driver.intent.OperationID {
		t.Fatalf("recovery POST = %d %s; operation=%q", recovery.Code, recovery.Body.String(), driver.recoveryRequest)
	}
}
