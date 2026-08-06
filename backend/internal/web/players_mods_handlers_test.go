package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func TestPlayerModDetailsAPIContract(t *testing.T) {
	fake := fakeDockerService{
		psResult: paneldocker.ComposePsResult{
			Services: []paneldocker.ComposeService{{Name: "demo-server-1", Service: "server", State: "running", Status: "Up 1 minute"}},
		},
	}
	handler, store, testDataDir, closeStore := newDockerTestHandlerWithStore(t, fake)
	defer closeStore()
	prepareComposeProject(t, testDataDir)

	adminCookie := setupDockerAdmin(t, handler)
	unauthorized, _ := doJSON(t, handler, http.MethodGet, "/api/instances/stardew/players/123/mods", nil, nil)
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d, want 401", unauthorized.Code)
	}
	instance, err := store.GetInstance(context.Background(), storage.DefaultInstanceID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID: storage.DefaultInstanceID, State: storage.InstanceStateRunning, StateMessage: "test running", DriverPhase: "running",
	}); err != nil {
		t.Fatal(err)
	}

	controlDir := filepath.Join(instance.DataDir, ".local-container", "control")
	modDir := filepath.Join(instance.DataDir, ".local-container", "mods", "Required")
	if err := os.MkdirAll(controlDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(modDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "manifest.json"), []byte(`{
  "Name":"Required", "UniqueID":"Example.Required", "Version":"1.0.0", "Author":"test"
}`), 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 6, 4, 5, 6, 0, time.UTC).Format(time.RFC3339Nano)
	if err := os.WriteFile(filepath.Join(controlDir, "options.json"), []byte(`{
  "schemaVersion":2,
  "source":"smapi-runtime",
  "generatedAt":"`+now+`",
  "gameVersion":"1.6.15",
  "apiVersion":"4.5.2",
  "loadedMods":[{"uniqueId":"Example.Required","version":"1.0.0"}]
}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(controlDir, "player-mod-contexts.json"), []byte(`{
  "schemaVersion":1,
  "updatedAt":"`+now+`",
  "players":{
    "123":{
      "uniqueMultiplayerId":"123",
      "hasSmapi":true,
      "gameVersion":"1.6.15",
      "apiVersion":"4.5.2",
      "mods":[{"uniqueId":"Example.Required","name":"Required","version":"1.0"}],
      "contextStatus":"reported",
      "reportedAt":"`+now+`",
      "updatedAt":"`+now+`"
    }
  }
}`), 0o600); err != nil {
		t.Fatal(err)
	}

	response, _ := doJSON(t, handler, http.MethodGet, "/api/instances/stardew/players/123/mods", nil, adminCookie)
	if response.Code != http.StatusOK {
		t.Fatalf("player mod details returned %d: %s", response.Code, response.Body.String())
	}
	var body sj.PlayerModDetailsResult
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.UniqueMultiplayerID != "123" || body.ContextStatus != sj.PlayerModContextReported || body.Comparison.Status != "available" {
		t.Fatalf("response contract = %+v", body)
	}
	if body.Comparison.Summary.Match != 1 || len(body.Comparison.Items) != 1 {
		t.Fatalf("comparison = %+v, want only the required Mod match", body.Comparison)
	}
	if body.ServerContext == nil || len(body.ServerContext.LoadedMods) != 1 || body.ServerContext.LoadedMods[0].UniqueID != "Example.Required" {
		t.Fatalf("server context = %+v", body.ServerContext)
	}

	wrongMethod, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/players/123/mods", nil, adminCookie)
	if wrongMethod.Code != http.StatusMethodNotAllowed {
		t.Fatalf("wrong method status = %d, want 405", wrongMethod.Code)
	}
	invalid, _ := doJSON(t, handler, http.MethodGet, "/api/instances/stardew/players/not-a-player/mods", nil, adminCookie)
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid player status = %d, want 400; body=%s", invalid.Code, invalid.Body.String())
	}
}

func TestPlayerModDetailsAPIPreservesUnavailableModsNull(t *testing.T) {
	fake := fakeDockerService{
		psResult: paneldocker.ComposePsResult{
			Services: []paneldocker.ComposeService{{Name: "demo-server-1", Service: "server", State: "running", Status: "Up 1 minute"}},
		},
	}
	handler, store, testDataDir, closeStore := newDockerTestHandlerWithStore(t, fake)
	defer closeStore()
	prepareComposeProject(t, testDataDir)
	adminCookie := setupDockerAdmin(t, handler)
	instance, err := store.GetInstance(context.Background(), storage.DefaultInstanceID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID: storage.DefaultInstanceID, State: storage.InstanceStateRunning, StateMessage: "test running", DriverPhase: "running",
	}); err != nil {
		t.Fatal(err)
	}
	controlDir := filepath.Join(instance.DataDir, ".local-container", "control")
	if err := os.MkdirAll(controlDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(controlDir, "player-mod-contexts.json"), []byte(`{
  "schemaVersion":1,
  "updatedAt":"2026-08-06T04:05:06Z",
  "players":{"123":{"uniqueMultiplayerId":"123","hasSmapi":false,"mods":null,"contextStatus":"pending","reportedAt":null,"updatedAt":"2026-08-06T04:05:06Z"}}
}`), 0o600); err != nil {
		t.Fatal(err)
	}

	response, _ := doJSON(t, handler, http.MethodGet, "/api/instances/stardew/players/123/mods", nil, adminCookie)
	if response.Code != http.StatusOK {
		t.Fatalf("pending context returned %d: %s", response.Code, response.Body.String())
	}
	var raw map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if value, exists := raw["mods"]; !exists || value != nil {
		t.Fatalf("mods = %#v (exists=%v), want explicit null", value, exists)
	}
	comparison, ok := raw["comparison"].(map[string]any)
	if !ok || comparison["status"] != sj.PlayerModComparisonUnavailable {
		t.Fatalf("comparison = %#v", raw["comparison"])
	}
}

func TestPlayerModDetailsLoopbackHTTPCompatibilityMatrix(t *testing.T) {
	fake := fakeDockerService{
		psResult: paneldocker.ComposePsResult{
			Services: []paneldocker.ComposeService{{Name: "demo-server-1", Service: "server", State: "running", Status: "Up 1 minute"}},
		},
	}
	handler, store, testDataDir, closeStore := newDockerTestHandlerWithStore(t, fake)
	defer closeStore()
	prepareComposeProject(t, testDataDir)
	adminCookie := setupDockerAdmin(t, handler)
	instance, err := store.GetInstance(context.Background(), storage.DefaultInstanceID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID: storage.DefaultInstanceID, State: storage.InstanceStateRunning, StateMessage: "test running", DriverPhase: "running",
	}); err != nil {
		t.Fatal(err)
	}
	controlDir := filepath.Join(instance.DataDir, ".local-container", "control")
	if err := os.MkdirAll(controlDir, 0o755); err != nil {
		t.Fatal(err)
	}
	now := "2026-08-06T05:06:07Z"
	if err := os.WriteFile(filepath.Join(controlDir, "options.json"), []byte(`{
  "schemaVersion":2,"source":"smapi-runtime","generatedAt":"`+now+`",
  "gameVersion":"1.6.15","apiVersion":"4.5.2","loadedMods":[]
}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(controlDir, "player-mod-contexts.json"), []byte(`{
  "schemaVersion":1,"updatedAt":"`+now+`","players":{
    "100":{"uniqueMultiplayerId":"100","hasSmapi":false,"mods":null,"contextStatus":"unavailable","reportedAt":null,"updatedAt":"`+now+`"},
    "200":{"uniqueMultiplayerId":"200","hasSmapi":true,"gameVersion":"1.6.15","apiVersion":"4.5.2","mods":[
      {"uniqueId":"CJBok.CheatsMenu","name":"CJB Cheats Menu","version":"1.37.2"},
      {"uniqueId":"CJBok.ItemSpawner","name":"CJB Item Spawner","version":"2.5.1"}
    ],"contextStatus":"reported","reportedAt":"`+now+`","updatedAt":"`+now+`"},
    "300":{"uniqueMultiplayerId":"300","hasSmapi":true,"gameVersion":"1.6.15","apiVersion":"4.5.2","mods":[
      {"uniqueId":"Example.Old","name":"Old report","version":"1.0.0"}
    ],"contextStatus":"stale","reportedAt":"`+now+`","updatedAt":"`+now+`"},
    "400":{"uniqueMultiplayerId":"400","hasSmapi":true,"mods":null,"contextStatus":"pending","reportedAt":null,"updatedAt":"`+now+`"}
  }
}`), 0o600); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(handler)
	defer server.Close()
	getDetails := func(playerID string) sj.PlayerModDetailsResult {
		t.Helper()
		request, err := http.NewRequest(http.MethodGet, server.URL+"/api/instances/stardew/players/"+playerID+"/mods", nil)
		if err != nil {
			t.Fatal(err)
		}
		request.AddCookie(adminCookie)
		response, err := server.Client().Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("player %s returned HTTP %d", playerID, response.StatusCode)
		}
		var result sj.PlayerModDetailsResult
		if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
			t.Fatal(err)
		}
		return result
	}

	vanilla := getDetails("100")
	if vanilla.ContextStatus != sj.PlayerModContextUnavailable || vanilla.Mods != nil || vanilla.Comparison.Status != sj.PlayerModComparisonUnavailable {
		t.Fatalf("vanilla/mobile unavailable contract = %+v", vanilla)
	}
	reported := getDetails("200")
	if reported.ContextStatus != sj.PlayerModContextReported || len(reported.Mods) != 2 || reported.Mods[0].Name != "CJB Cheats Menu" || reported.Mods[0].Version != "1.37.2" {
		t.Fatalf("reported CJB contract = %+v", reported)
	}
	if len(reported.RiskFlags) != 1 || reported.RiskFlags[0] != "cjb" {
		t.Fatalf("reported CJB risk flags = %#v", reported.RiskFlags)
	}
	cjbItems := 0
	for _, item := range reported.Comparison.Items {
		if len(item.RiskFlags) == 1 && item.RiskFlags[0] == "cjb" {
			cjbItems++
		}
	}
	if cjbItems != 2 {
		t.Fatalf("official CJB comparison items flagged = %d, want 2; items=%+v", cjbItems, reported.Comparison.Items)
	}
	stale := getDetails("300")
	if stale.ContextStatus != sj.PlayerModContextStale || len(stale.Mods) != 1 || stale.Comparison.UnavailableReason != sj.PlayerModContextStale {
		t.Fatalf("stale contract = %+v", stale)
	}
	pending := getDetails("400")
	if pending.ContextStatus != sj.PlayerModContextPending || pending.Mods != nil || pending.Comparison.UnavailableReason != sj.PlayerModContextPending {
		t.Fatalf("pending contract = %+v", pending)
	}
	unknown := getDetails("500")
	if unknown.UniqueMultiplayerID != "500" || unknown.Mods != nil || unknown.ContextStatus != sj.PlayerModContextUnavailable {
		t.Fatalf("unknown player inherited another context: %+v", unknown)
	}
}
