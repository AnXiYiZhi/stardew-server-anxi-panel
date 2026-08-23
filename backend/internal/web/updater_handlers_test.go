package web

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"net/http"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
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

func TestFullStackInstanceStatusSkipsUninstalledInstance(t *testing.T) {
	s := &server{}
	for _, state := range []string{storage.InstanceStateUninitialized, storage.InstanceStateAdminCreated} {
		t.Run(state, func(t *testing.T) {
			status := s.fullStackInstanceStatus(context.Background(), registry.Instance{ID: "stardew", State: state})
			if status.Phase != "not_needed" || status.Progress != 100 || status.RuntimeRequired {
				t.Fatalf("uninstalled full-stack status = %+v", status)
			}
			if status.InstanceID != "stardew" || status.Result == "" {
				t.Fatalf("uninstalled full-stack details = %+v", status)
			}
		})
	}
}

func TestApplyFullStackRuntimePhaseSeparatesAuthFromSMAPIVerification(t *testing.T) {
	tests := []struct {
		name       string
		applyPhase string
		wantPhase  string
		wantResult string
	}{
		{name: "auth", applyPhase: sj.RuntimeUpdateApplyVerifyingAuth, wantPhase: "verifying_auth", wantResult: "不等待 Steam 登录"},
		{name: "server", applyPhase: sj.RuntimeUpdateApplyVerifyingServer, wantPhase: "verifying_runtime", wantResult: "SMAPI 实际加载版本"},
		{name: "restore", applyPhase: sj.RuntimeUpdateApplyRestoringState, wantPhase: "restoring_server", wantResult: "恢复升级前"},
		{name: "rollback", applyPhase: sj.RuntimeUpdateApplyRollingBack, wantPhase: "rolling_back_runtime", wantResult: "自动恢复原版本"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			full := &updater.FullStackStatus{Phase: "updating_runtime", Result: "default"}
			applyFullStackRuntimePhase(full, sj.RuntimeUpdateApplyStatus{Phase: test.applyPhase})
			if full.Phase != test.wantPhase || !strings.Contains(full.Result, test.wantResult) {
				t.Fatalf("phase=%q result=%q", full.Phase, full.Result)
			}
		})
	}
}

func TestEnrichFullStackStatusCompletesForUninstalledInstances(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	store, err := storage.Open(ctx, config.Config{
		DataDir: dataDir, DBPath: filepath.Join(dataDir, "panel.db"), Secret: "test-secret", Version: "0.4.6",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := store.EnsureDefaultInstance(ctx, storage.EnsureDefaultInstanceParams{
		ID: storage.DefaultInstanceID, DriverID: storage.DefaultDriverID, DataDir: filepath.Join(dataDir, "instances", "stardew"),
	}); err != nil {
		t.Fatal(err)
	}

	s := &server{config: config.Config{Version: "0.4.6"}, store: store, registry: registry.New()}
	status := updater.ApplyStatus{Phase: updater.PhaseSucceeded, ToVersion: "v0.4.6"}
	s.enrichFullStackUpdateStatus(ctx, &status)
	if status.FullStack == nil || status.FullStack.Phase != "not_needed" || status.FullStack.Progress != 100 || status.FullStack.RuntimeRequired {
		t.Fatalf("aggregate full-stack status = %+v", status.FullStack)
	}
	if len(status.FullStack.Instances) != 1 || status.FullStack.Instances[0].Phase != "not_needed" {
		t.Fatalf("aggregate instance status = %+v", status.FullStack.Instances)
	}
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
