package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
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
	want = want.ForPanelVersion(body.Version)
	if body.RuntimeUpgrade == nil || *body.RuntimeUpgrade != want {
		t.Fatalf("public release policy differs from the embedded target: %+v", body.RuntimeUpgrade)
	}
}

func TestVersionRuntimeUpgradeTargetsTheLoadedPatch(t *testing.T) {
	for _, version := range []string{"0.8.1", "v0.8.2"} {
		t.Run(version, func(t *testing.T) {
			s := &server{config: appconfig.Config{Version: version}}
			response := httptest.NewRecorder()
			s.handleVersion(response, httptest.NewRequest(http.MethodGet, "/api/version", nil))
			var body versionResponse
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if response.Code != http.StatusOK || body.RuntimeUpgrade == nil || !body.RuntimeUpgrade.RequiresScopeConfirmation || body.RuntimeUpgrade.TargetPanelVersion != strings.TrimPrefix(version, "v") {
				t.Fatalf("loaded patch advertised the wrong runtime confirmation target: %+v", body)
			}
		})
	}
}
