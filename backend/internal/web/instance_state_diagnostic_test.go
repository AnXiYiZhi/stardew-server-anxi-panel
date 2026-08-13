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

func TestInstanceStateProjectsInstallationAndRuntimeControlDiagnostics(t *testing.T) {
	tests := []struct {
		name               string
		optionsBody        string
		wantRuntime        string
		wantObserved       string
		wantRuntimeMatch   bool
		wantRecommendation string
	}{
		{
			name:               "runtime not observed remains pending evidence",
			wantRuntime:        "not_observed",
			wantRecommendation: "retry_start",
		},
		{
			name:               "explicit old runtime is projected as mismatch",
			optionsBody:        `{"controlModVersion":"0.2.2"}`,
			wantRuntime:        "mismatch",
			wantObserved:       "0.2.2",
			wantRecommendation: "diagnose",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
			if tt.optionsBody != "" {
				controlDir := filepath.Join(instanceDir, ".local-container", "control")
				if err := os.WriteFile(filepath.Join(controlDir, "options.json"), []byte(tt.optionsBody), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			setDockerTestInstanceState(t, store, storage.InstanceStateRunning)

			response, _ := doJSON(t, handler, http.MethodGet, "/api/instances/stardew/state", nil, adminCookie)
			if response.Code != http.StatusOK {
				t.Fatalf("state returned %d: %s", response.Code, response.Body.String())
			}
			var body struct {
				InstallationDiagnostic *struct {
					Status            string `json:"status"`
					RequiredFiles     string `json:"requiredFiles"`
					Compose           string `json:"compose"`
					Image             string `json:"image"`
					ServerContainer   string `json:"serverContainer"`
					RecommendedAction string `json:"recommendedAction"`
					Control           struct {
						Static          string `json:"static"`
						Runtime         string `json:"runtime"`
						ObservedVersion string `json:"observedVersion"`
						ExpectedVersion string `json:"expectedVersion"`
					} `json:"control"`
				} `json:"installationDiagnostic"`
				RuntimeDiagnostic struct {
					ControlModVersion       string `json:"controlModVersion"`
					ExpectedControlMod      string `json:"expectedControlModVersion"`
					InstalledControlVersion string `json:"installedControlVersion"`
					InstalledControlMatches bool   `json:"installedControlMatches"`
					RuntimeControlState     string `json:"runtimeControlState"`
					RuntimeControlVersion   string `json:"runtimeControlVersion"`
					RuntimeControlMatches   bool   `json:"runtimeControlMatches"`
				} `json:"runtimeDiagnostic"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.InstallationDiagnostic == nil {
				t.Fatalf("installationDiagnostic missing: %s", response.Body.String())
			}
			diagnostic := body.InstallationDiagnostic
			if diagnostic.Status != "installed" || diagnostic.RequiredFiles != "ok" ||
				diagnostic.Compose != "ready" || diagnostic.Image != "available" || diagnostic.ServerContainer != "running" {
				t.Fatalf("installation evidence = %+v", diagnostic)
			}
			if diagnostic.Control.Static != "match" || diagnostic.Control.Runtime != tt.wantRuntime ||
				diagnostic.Control.ObservedVersion != tt.wantObserved || diagnostic.RecommendedAction != tt.wantRecommendation {
				t.Fatalf("Control installation diagnostic = %+v", diagnostic)
			}
			if diagnostic.Control.ExpectedVersion == "" {
				t.Fatal("expected Control version was not projected")
			}
			runtime := body.RuntimeDiagnostic
			if runtime.ControlModVersion != diagnostic.Control.ExpectedVersion ||
				runtime.ExpectedControlMod != diagnostic.Control.ExpectedVersion ||
				runtime.InstalledControlVersion != diagnostic.Control.ExpectedVersion ||
				!runtime.InstalledControlMatches {
				t.Fatalf("static runtime projection = %+v, installation=%+v", runtime, diagnostic.Control)
			}
			if runtime.RuntimeControlState != tt.wantRuntime || runtime.RuntimeControlVersion != tt.wantObserved ||
				runtime.RuntimeControlMatches != tt.wantRuntimeMatch {
				t.Fatalf("runtime Control projection = %+v", runtime)
			}
		})
	}
}
