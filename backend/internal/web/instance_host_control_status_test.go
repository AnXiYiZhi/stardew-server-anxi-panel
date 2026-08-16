package web

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func TestInstanceStateProjectsHostBedAndManualControl(t *testing.T) {
	fake := fakeDockerService{psResult: paneldocker.ComposePsResult{Services: []paneldocker.ComposeService{{
		Service: "server", State: "running", Status: "Up 1 minute",
	}}}}
	handler, store, dataDir, closeStore := newDockerTestHandlerWithStore(t, fake)
	defer closeStore()
	adminCookie := setupDockerAdmin(t, handler)

	instanceDir := filepath.Join(dataDir, "instances", storage.DefaultInstanceID)
	if err := sj.New(fake, nil, nil, nil).Prepare(context.Background(), registry.Instance{
		ID: storage.DefaultInstanceID, DriverID: storage.DefaultDriverID,
		Name: "Stardew Valley", DataDir: instanceDir, State: storage.InstanceStateUninitialized,
	}); err != nil {
		t.Fatal(err)
	}
	controlDir := filepath.Join(instanceDir, ".local-container", "control")
	status := []byte(`{
		"state":"save-loaded","saveId":"Farm_1","updatedAt":"2026-08-16T00:00:00Z",
		"hostBed":{"state":"repaired","healthy":true,"houseUpgradeLevel":2,"expectedBedType":"Double","actualBedType":"Double","bedTileX":33,"bedTileY":14,"playerBedSpotX":34,"playerBedSpotY":15,"repaired":true},
		"hostControl":{"mode":"manual","automationKnown":true,"automationEnabled":false,"manualControl":true,"paused":false,"pauseReason":"ManualControl","hostVisible":false,"displayFarmer":false,"farmerHidden":true,"visibilityConsistent":true,"connectedClients":0,"manualLeaseExpiresAt":"2026-08-16T00:10:00Z"}
	}`)
	if err := os.WriteFile(filepath.Join(controlDir, "status.json"), status, 0o600); err != nil {
		t.Fatal(err)
	}
	setDockerTestInstanceState(t, store, storage.InstanceStateRunning)

	response, _ := doJSON(t, handler, http.MethodGet, "/api/instances/stardew/state", nil, adminCookie)
	if response.Code != http.StatusOK {
		t.Fatalf("state returned %d: %s", response.Code, response.Body.String())
	}
	var body struct {
		StatusSource controlStatusSnapshot `json:"statusSource"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.StatusSource.HostBed.Healthy || !body.StatusSource.HostBed.Repaired || body.StatusSource.HostBed.HouseUpgradeLevel != 2 {
		t.Fatalf("host bed evidence was not projected: %+v", body.StatusSource.HostBed)
	}
	control := body.StatusSource.HostControl
	if control.Mode != "manual" || control.AutomationEnabled || !control.ManualControl || control.Paused ||
		control.HostVisible || control.DisplayFarmer || !control.FarmerHidden || !control.VisibilityConsistent {
		t.Fatalf("host manual/visibility state was not projected atomically: %+v", control)
	}
}
