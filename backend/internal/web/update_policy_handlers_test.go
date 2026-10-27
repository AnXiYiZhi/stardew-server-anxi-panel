package web

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/updatecheck"
)

type recheckingRelease struct{ before, after updatecheck.Status }

func (r *recheckingRelease) Status() updatecheck.Status { return r.before }
func (r *recheckingRelease) Check(context.Context) updatecheck.Status {
	r.before = r.after
	return r.after
}

func TestApplyRechecksPolicyWithoutChangingConfirmedTarget(t *testing.T) {
	for _, scenario := range []string{"valid", "new release", "new release legacy body", "new notice", "withdrawn", "check failed"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			cfg := config.Config{DataDir: root, DBPath: filepath.Join(root, "panel.db"), Secret: "test-secret", Version: "0.7.3"}
			store, err := storage.Open(context.Background(), cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			if err := store.Migrate(context.Background()); err != nil {
				t.Fatal(err)
			}
			before := updatecheck.Status{CurrentVersion: "0.7.3", LatestVersion: "v0.7.4", UpdateAvailable: true, CheckStatus: updatecheck.StatusOK}
			after := before
			switch scenario {
			case "new release", "new release legacy body":
				after.LatestVersion = "v0.7.5"
			case "new notice":
				after.NoticeDigest = "new-notice"
			case "withdrawn":
				after.Content.Blocked = true
				after.UpdateAvailable = false
			case "check failed":
				after.CheckStatus = updatecheck.StatusError
			}
			fake := &fakeUpdaterService{}
			handler := NewHandler(Deps{Config: cfg, Store: store, Updater: fake, UpdateChecker: &recheckingRelease{before: before, after: after}})
			_, cookie := doJSON(t, handler, http.MethodPost, "/api/setup/admin", map[string]string{"username": "admin", "password": "admin-password", "confirmPassword": "admin-password"}, nil)
			body := map[string]any{"confirmFullStack": true, "targetVersion": "v0.7.4"}
			if scenario == "new release legacy body" {
				delete(body, "targetVersion")
			}
			response, _ := doJSON(t, handler, http.MethodPost, "/api/system/update/apply", body, cookie)
			if scenario == "valid" {
				if response.Code != http.StatusAccepted || fake.applies != 1 || fake.applyTo != "v0.7.4" {
					t.Fatalf("valid target rejected: %d", response.Code)
				}
			} else if response.Code != http.StatusConflict || fake.applies != 0 {
				t.Fatalf("changed policy authorized an update: %d, %d", response.Code, fake.applies)
			}
		})
	}
}
