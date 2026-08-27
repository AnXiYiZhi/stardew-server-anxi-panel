package web

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func TestVisibleInviteCodeRequiresEnabledRunningInstance(t *testing.T) {
	instance := storage.Instance{State: storage.InstanceStateRunning, DriverPayload: `{"invite_code":"OLD-CODE"}`}
	if got := visibleInviteCode(instance, true); got != "OLD-CODE" {
		t.Fatalf("running enabled invite code = %q", got)
	}
	for _, state := range []string{storage.InstanceStateStarting, storage.InstanceStateStopped, storage.InstanceStateError} {
		instance.State = state
		if got := visibleInviteCode(instance, true); got != "" {
			t.Fatalf("state %s exposed stale invite code %q", state, got)
		}
	}
	instance.State = storage.InstanceStateRunning
	if got := visibleInviteCode(instance, false); got != "" {
		t.Fatalf("disabled instance exposed invite code %q", got)
	}
}

func TestInstanceStateExposesSteamInviteCleanupPending(t *testing.T) {
	handler, store, dataDir, cleanup := newDockerTestHandlerWithStore(t, fakeDockerService{
		instanceState: storage.InstanceStateStopped,
	})
	defer cleanup()
	adminCookie := setupDockerAdmin(t, handler)
	setDockerTestInstanceState(t, store, storage.InstanceStateStopped)
	instanceDir := filepath.Join(dataDir, "instances", storage.DefaultInstanceID)
	if err := os.MkdirAll(instanceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := sjconfig.SetSteamAuthCompletedState(instanceDir, sjconfig.SteamInviteAuthStateCleanupPending); err != nil {
		t.Fatal(err)
	}

	response, _ := doJSON(t, handler, http.MethodGet, "/api/instances/stardew/state", nil, adminCookie)
	if response.Code != http.StatusOK {
		t.Fatalf("state response returned %d: %s", response.Code, response.Body.String())
	}
	assertJSONField(t, response.Body.Bytes(), "steamInviteEnabled", true)
	assertJSONField(t, response.Body.Bytes(), "steamInviteAuthState", sjconfig.SteamInviteAuthStateCleanupPending)
	assertJSONField(t, response.Body.Bytes(), "steamAuthLoggedIn", true)
}
