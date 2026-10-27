package web

import (
	"encoding/json"
	"net/http"
	"testing"

	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
)

func TestVersionExposesEmbeddedRuntimeUpgradePolicy(t *testing.T) {
	handler, closeStore := newTestHandler(t)
	defer closeStore()
	response, _ := doJSON(t, handler, http.MethodGet, "/api/version", nil, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("version endpoint: %d", response.Code)
	}
	var body versionResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	want, err := sjconfig.BuiltInRuntimeUpgradePolicy()
	if err != nil {
		t.Fatal(err)
	}
	if body.RuntimeUpgrade == nil || *body.RuntimeUpgrade != want {
		t.Fatalf("public release policy differs from the embedded target: %+v", body.RuntimeUpgrade)
	}
}
