package web

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"net/http"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/updatecheck"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/updater"
)

type fakeUpdaterService struct {
	capability updater.Capability
	status     updater.DryRunStatus
	starts     int
	applies    int
	applyFrom  string
	applyTo    string
}

func (f *fakeUpdaterService) Capability(context.Context) updater.Capability { return f.capability }
func (f *fakeUpdaterService) Status() (updater.DryRunStatus, error)         { return f.status, nil }
func (f *fakeUpdaterService) StartDryRun(_ context.Context, version string) (updater.DryRunStatus, error) {
	f.starts++
	f.status.TargetVersion = version
	return f.status, nil
}
func (f *fakeUpdaterService) ApplyStatus() (updater.ApplyStatus, error) {
	return updater.ApplyStatus{UpdateID: "apply", Phase: updater.PhaseChecking}, nil
}

func (f *fakeUpdaterService) StartApply(_ context.Context, from, to string) (updater.ApplyStatus, error) {
	f.applies++
	f.applyFrom, f.applyTo = from, to
	return updater.ApplyStatus{UpdateID: "apply", Phase: updater.PhaseBackingUp}, nil
}

func TestUpdaterEndpointsAreAdminOnly(t *testing.T) {
	dataDir := t.TempDir()
	store, err := storage.Open(context.Background(), config.Config{
		DataDir: dataDir, DBPath: filepath.Join(dataDir, "panel.db"), Secret: "test-secret", Version: "0.1.14",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	fake := &fakeUpdaterService{
		capability: updater.Capability{Supported: true, Code: updater.CodeSupported, ComposeProject: "anxi-panel"},
		status:     updater.DryRunStatus{ID: "dry-run", Phase: "running", StartedAt: time.Now(), UpdatedAt: time.Now(), Logs: []updater.LogEntry{}},
	}
	handler := NewHandler(Deps{
		Config: config.Config{DataDir: dataDir, Secret: "test-secret", Version: "0.1.14"}, Store: store, Updater: fake,
		UpdateChecker: &fakeUpdateChecker{status: updatecheck.Status{CurrentVersion: "0.1.14", LatestVersion: "v0.1.15", UpdateAvailable: true, CheckStatus: updatecheck.StatusOK}},
	})
	_, adminCookie := doJSON(t, handler, http.MethodPost, "/api/setup/admin", map[string]string{
		"username": "admin", "password": "admin-password", "confirmPassword": "admin-password",
	}, nil)
	created, _ := doJSON(t, handler, http.MethodPost, "/api/users", map[string]string{
		"username": "player", "password": "player-password", "role": "user",
	}, adminCookie)
	if created.Code != http.StatusCreated {
		t.Fatalf("create user = %d", created.Code)
	}
	_, playerCookie := doJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "player", "password": "player-password",
	}, nil)
	for _, path := range []string{"/api/system/update/capability", "/api/system/update/dry-run"} {
		method := http.MethodGet
		if path == "/api/system/update/dry-run" {
			method = http.MethodPost
		}
		response, _ := doJSON(t, handler, method, path, map[string]string{"targetVersion": "0.1.15"}, playerCookie)
		if response.Code != http.StatusForbidden {
			t.Fatalf("player %s %s = %d", method, path, response.Code)
		}
	}
	forbiddenApply, _ := doJSON(t, handler, http.MethodPost, "/api/system/update/apply", nil, playerCookie)
	if forbiddenApply.Code != http.StatusForbidden {
		t.Fatalf("player apply = %d", forbiddenApply.Code)
	}
	capability, _ := doJSON(t, handler, http.MethodGet, "/api/system/update/capability", nil, adminCookie)
	if capability.Code != http.StatusOK {
		t.Fatalf("admin capability = %d: %s", capability.Code, capability.Body.String())
	}
	dryRun, _ := doJSON(t, handler, http.MethodPost, "/api/system/update/dry-run", map[string]string{"targetVersion": "0.1.15"}, adminCookie)
	if dryRun.Code != http.StatusAccepted || fake.starts != 1 {
		t.Fatalf("admin dry-run = %d, starts=%d: %s", dryRun.Code, fake.starts, dryRun.Body.String())
	}
	rejectedBody, _ := doJSON(t, handler, http.MethodPost, "/api/system/update/apply", map[string]string{"targetVersion": "9.9.9"}, adminCookie)
	if rejectedBody.Code != http.StatusBadRequest || fake.applies != 0 {
		t.Fatalf("apply body must be rejected: code=%d applies=%d", rejectedBody.Code, fake.applies)
	}
	apply, _ := doJSON(t, handler, http.MethodPost, "/api/system/update/apply", map[string]bool{"confirmFullStack": true}, adminCookie)
	if apply.Code != http.StatusAccepted || fake.applies != 1 || fake.applyFrom != "0.1.14" || fake.applyTo != "v0.1.15" {
		t.Fatalf("admin apply=%d applies=%d from=%s to=%s: %s", apply.Code, fake.applies, fake.applyFrom, fake.applyTo, apply.Body.String())
	}
}
