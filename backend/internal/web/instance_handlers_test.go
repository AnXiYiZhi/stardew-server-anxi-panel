package web

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func (f fakeDockerService) ProvisionInstanceGameData(context.Context, string, string, string, string, string, string) error {
	return nil
}

func (f fakeDockerService) CleanupInstanceGameData(context.Context, string, string, string) error {
	return nil
}

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

func TestInstanceRenamePersistsNameAndPreservesRuntime(t *testing.T) {
	handler, store, _, cleanup := newDockerTestHandlerWithStore(t, fakeDockerService{})
	defer cleanup()
	cookie := setupDockerAdmin(t, handler)
	before, err := store.GetInstance(context.Background(), storage.DefaultInstanceID)
	if err != nil {
		t.Fatal(err)
	}
	path := "/api/instances/" + storage.DefaultInstanceID
	response, _ := doJSON(t, handler, http.MethodPatch, path, map[string]string{"name": "新世界"}, nil)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status=%d", response.Code)
	}
	for _, name := range []string{"", "  ", strings.Repeat("界", 41), "bad\nname"} {
		response, _ = doJSON(t, handler, http.MethodPatch, path, map[string]string{"name": name}, cookie)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid name status=%d", response.Code)
		}
	}
	response, _ = doJSON(t, handler, http.MethodPatch, path, map[string]string{"name": " 森林小屋 "}, cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("rename status=%d: %s", response.Code, response.Body.String())
	}
	after, err := store.GetInstance(context.Background(), storage.DefaultInstanceID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Name != "森林小屋" || after.ID != before.ID || after.DataDir != before.DataDir || after.State != before.State || after.DriverPayload != before.DriverPayload {
		t.Fatal("rename did not preserve instance identity and runtime")
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

func TestGameInstallationStatusAndWorldCreationUsePanelTemplate(t *testing.T) {
	handler, store, dataDir, cleanup := newDockerTestHandlerWithStore(t, fakeDockerService{})
	defer cleanup()
	// Provisioning validates the configured managed root, just as production's
	// NewWithOptions driver does. The skeleton fixture has no root by default.
	manager := jobs.NewManager(store, nil)
	drivers := registry.New()
	if err := drivers.Register(sj.NewWithOptions(fakeDockerService{}, nil, manager, store, sj.DriverOptions{ContainerDataDir: dataDir})); err != nil {
		t.Fatal(err)
	}
	handler = NewHandler(Deps{Config: config.Config{DataDir: dataDir, Secret: "test-secret", Version: "test"}, Store: store, Docker: fakeDockerService{}, Jobs: manager, Registry: drivers})
	adminCookie := setupDockerAdmin(t, handler)

	unauthenticated, _ := doJSON(t, handler, http.MethodGet, "/api/games/stardew/installation", nil, nil)
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated game installation status returned %d: %s", unauthenticated.Code, unauthenticated.Body.String())
	}

	prepare, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/prepare", map[string]string{}, adminCookie)
	if prepare.Code != http.StatusOK {
		t.Fatalf("prepare installation target returned %d: %s", prepare.Code, prepare.Body.String())
	}
	if _, err := store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID: storage.DefaultInstanceID, State: storage.InstanceStateGameInstalled,
		StateMessage: "installed", DriverPhase: "game_installed", DriverPayload: "{}",
	}); err != nil {
		t.Fatalf("mark game installation ready: %v", err)
	}
	instanceDir := filepath.Join(dataDir, "instances", storage.DefaultInstanceID)
	if err := sjconfig.UpdateEnvFile(filepath.Join(instanceDir, ".env"), map[string]string{
		"STEAM_USERNAME": "shared-user", "STEAM_PASSWORD": "secret", "STEAMCMD_AUTH_COMPLETED": "true",
	}); err != nil {
		t.Fatalf("seed legacy shared credentials: %v", err)
	}

	status, _ := doJSON(t, handler, http.MethodGet, "/api/games/stardew/installation", nil, adminCookie)
	if status.Code != http.StatusOK {
		t.Fatalf("game installation status returned %d: %s", status.Code, status.Body.String())
	}
	assertJSONField(t, status.Body.Bytes(), "gameId", "stardew")
	assertJSONField(t, status.Body.Bytes(), "installationTargetId", storage.DefaultInstanceID)
	assertJSONField(t, status.Body.Bytes(), "installed", true)
	assertJSONField(t, status.Body.Bytes(), "credentialsConfigured", true)
	assertJSONField(t, status.Body.Bytes(), "authorizationCached", true)
	if body := status.Body.String(); strings.Contains(body, "secret") || strings.Contains(body, "shared-user") {
		t.Fatalf("installation status exposed Steam credentials: %s", body)
	}
	createdUser, _ := doJSON(t, handler, http.MethodPost, "/api/users", map[string]string{
		"username": "installation-reader", "password": "reader-password", "role": "user",
	}, adminCookie)
	if createdUser.Code != http.StatusCreated {
		t.Fatalf("create installation reader returned %d: %s", createdUser.Code, createdUser.Body.String())
	}
	login, userCookie := doJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "installation-reader", "password": "reader-password",
	}, nil)
	if login.Code != http.StatusOK {
		t.Fatalf("installation reader login returned %d: %s", login.Code, login.Body.String())
	}
	memberStatus, _ := doJSON(t, handler, http.MethodGet, "/api/games/stardew/installation", nil, userCookie)
	if memberStatus.Code != http.StatusOK {
		t.Fatalf("ordinary user game installation status returned %d: %s", memberStatus.Code, memberStatus.Body.String())
	}

	clientSelectedID, _ := doJSON(t, handler, http.MethodPost, "/api/instances", map[string]string{
		"id": "river-farm", "name": "河湾农场", "gameId": "stardew",
	}, adminCookie)
	if clientSelectedID.Code != http.StatusBadRequest {
		t.Fatalf("client-selected instance id returned %d: %s", clientSelectedID.Code, clientSelectedID.Body.String())
	}

	created, _ := doJSON(t, handler, http.MethodPost, "/api/instances", map[string]string{
		"name": "河湾农场", "gameId": "stardew",
	}, adminCookie)
	if created.Code != http.StatusCreated {
		t.Fatalf("create world returned %d: %s", created.Code, created.Body.String())
	}
	assertJSONField(t, created.Body.Bytes(), "gameId", "stardew")
	assertNestedJSONField(t, created.Body.Bytes(), "instance", "id", "stardew-2")
	stored, err := store.GetInstance(context.Background(), "stardew-2")
	if err != nil {
		t.Fatalf("load created world: %v", err)
	}
	if stored.State != storage.InstanceStateSaveRequired || stored.DriverPhase != "instance_ready" {
		t.Fatalf("created world state = %s/%s", stored.State, stored.DriverPhase)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "instances", "stardew-2", "docker-compose.yml")); err != nil {
		t.Fatalf("created world Compose missing: %v", err)
	}
}

func TestWorldCreationRequiresAdministrator(t *testing.T) {
	handler, _, _, cleanup := newDockerTestHandlerWithStore(t, fakeDockerService{})
	defer cleanup()
	adminCookie := setupDockerAdmin(t, handler)
	createdUser, _ := doJSON(t, handler, http.MethodPost, "/api/users", map[string]string{
		"username": "player", "password": "player-password", "role": "user",
	}, adminCookie)
	if createdUser.Code != http.StatusCreated {
		t.Fatalf("create ordinary user returned %d: %s", createdUser.Code, createdUser.Body.String())
	}
	login, userCookie := doJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "player", "password": "player-password",
	}, nil)
	if login.Code != http.StatusOK {
		t.Fatalf("ordinary user login returned %d: %s", login.Code, login.Body.String())
	}
	response, _ := doJSON(t, handler, http.MethodPost, "/api/instances", map[string]string{
		"name": "河湾农场", "gameId": "stardew",
	}, userCookie)
	if response.Code != http.StatusForbidden {
		t.Fatalf("ordinary user create world returned %d: %s", response.Code, response.Body.String())
	}
}
