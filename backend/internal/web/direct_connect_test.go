package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func TestDirectConnectReadsStoppedWorldConfigWithoutPublicIPLookup(t *testing.T) {
	var calls atomic.Int32
	ipServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, "offline", http.StatusServiceUnavailable)
	}))
	defer ipServer.Close()
	oldProviders := defaultPublicIPProviders
	defaultPublicIPProviders = []string{ipServer.URL}
	t.Cleanup(func() { defaultPublicIPProviders = oldProviders })

	handler, store, dataDir, closeStore := newDockerTestHandlerWithStore(t, fakeDockerService{})
	defer closeStore()
	worldDir := filepath.Join(dataDir, "instances", "direct-connect-world")
	if _, err := store.CreateInstance(context.Background(), storage.CreateInstanceParams{
		ID: "direct-connect-world", DriverID: storage.DefaultDriverID, Name: "Direct connect world",
		DataDir: worldDir, State: storage.InstanceStateStopped,
	}); err != nil {
		t.Fatal(err)
	}
	_, cookie := doJSON(t, handler, http.MethodPost, "/api/setup/admin", map[string]string{
		"username": "admin", "password": "secret123", "confirmPassword": "secret123",
	}, nil)
	endpoint := "/api/instances/direct-connect-world/direct-connect"
	unauthorized, _ := doJSON(t, handler, http.MethodGet, endpoint, nil, nil)
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d", unauthorized.Code)
	}
	wrongMethod, _ := doJSON(t, handler, http.MethodPost, endpoint, nil, cookie)
	if wrongMethod.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d", wrongMethod.Code)
	}
	missing, _ := doJSON(t, handler, http.MethodGet, "/api/instances/missing/direct-connect", nil, cookie)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing world status = %d", missing.Code)
	}

	// No game process or env file is needed for the driver's Compose default.
	assertPort := func(path string, want int) {
		t.Helper()
		response, _ := doJSON(t, handler, http.MethodGet, path, nil, cookie)
		if response.Code != http.StatusOK {
			t.Fatalf("connection status = %d: %s", response.Code, response.Body.String())
		}
		var config registry.DirectConnectConfig
		if err := json.Unmarshal(response.Body.Bytes(), &config); err != nil {
			t.Fatal(err)
		}
		if config.GamePort != want || config.Protocol != "udp" {
			t.Fatalf("config = %+v, want port %d/udp", config, want)
		}
	}
	assertPort(endpoint, 24642)
	for _, port := range []string{"24643", "24644"} {
		if err := sjconfig.UpdateEnvFile(filepath.Join(worldDir, ".env"), map[string]string{"GAME_PORT": port}); err != nil {
			t.Fatal(err)
		}
		want := 24643
		if port == "24644" {
			want = 24644
		}
		assertPort(endpoint, want)
		assertPort("/api/instances/stardew/direct-connect", 24642)
	}
	if err := sjconfig.UpdateEnvFile(filepath.Join(worldDir, ".env"), map[string]string{"GAME_PORT": "invalid"}); err != nil {
		t.Fatal(err)
	}
	invalid, _ := doJSON(t, handler, http.MethodGet, endpoint, nil, cookie)
	if invalid.Code != http.StatusInternalServerError {
		t.Fatalf("invalid config status = %d", invalid.Code)
	}
	if calls.Load() != 0 {
		t.Fatalf("direct connection made %d external IP lookups", calls.Load())
	}
}
