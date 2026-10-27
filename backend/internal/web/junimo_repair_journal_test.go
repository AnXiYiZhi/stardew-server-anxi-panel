package web

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

type junimoRepairJournalFixture struct {
	handler      http.Handler
	store        *storage.Store
	admin        *http.Cookie
	instance     storage.Instance
	status       sj.RuntimeUpdateApplyStatus
	manifest     map[string]any
	statusPath   string
	manifestPath string
	backupEnv    string
	paths        []string
}

func newJunimoRepairJournalFixture(t *testing.T) *junimoRepairJournalFixture {
	t.Helper()
	handler, store, _, cleanup := newDockerTestHandlerWithStore(t, fakeDockerService{instanceState: storage.InstanceStateStopped})
	t.Cleanup(cleanup)
	admin := setupDockerAdmin(t, handler)
	if _, err := store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID: storage.DefaultInstanceID, State: storage.InstanceStateStopped, DriverPhase: "stopped", DriverPayload: "{}",
	}); err != nil {
		t.Fatal(err)
	}
	instance, err := store.GetInstance(context.Background(), storage.DefaultInstanceID)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := sjconfig.BuiltInRuntimeStackManifest()
	if err != nil {
		t.Fatal(err)
	}
	write := func(path string, data []byte) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	controlDir := filepath.Join(instance.DataDir, ".local-container", "mods", "StardewAnxiPanel.Control")
	controlFiles := map[string][]byte{}
	for _, name := range []string{"manifest.json", "StardewAnxiPanel.Control.dll"} {
		data, err := os.ReadFile(filepath.Join("..", "games", "stardew_junimo", "embedded", "smapi-mod", name))
		if err != nil {
			t.Fatal(err)
		}
		controlFiles[name] = data
		write(filepath.Join(controlDir, name), data)
	}
	env := []byte(strings.Join([]string{
		"IMAGE_VERSION=" + manifest.Server.Tag,
		"SERVER_IMAGE=" + manifest.Server.Images[0],
		"SERVER_IMAGE_CANDIDATES=" + strings.Join(manifest.Server.Images, ","),
		"STEAM_SERVICE_IMAGE=" + manifest.SteamAuth.Images[0],
		"STEAM_SERVICE_IMAGE_CANDIDATES=" + strings.Join(manifest.SteamAuth.Images, ","),
		"STEAM_INVITE_ENABLED=true",
		"STEAM_AUTH_COMPLETED=true",
		"STEAM_PASSWORD=journal-get-secret",
	}, "\n") + "\n")
	compose := []byte("services:\n  server:\n    image: " + manifest.Server.Images[0] + "\n  steam-auth:\n    image: " + manifest.SteamAuth.Images[0] + "\n")
	write(filepath.Join(instance.DataDir, ".env"), env)
	write(filepath.Join(instance.DataDir, "docker-compose.yml"), compose)
	inspection := sj.InspectManagedRuntimeStack(instance.DataDir, instance.State)
	if inspection.Status != sjconfig.RuntimeStackStatusUpToDate {
		t.Fatalf("fixture does not prove the installed recommended runtime: status=%s code=%s", inspection.Status, inspection.Code)
	}
	selected := sj.RuntimeUpdateSelectedPair{
		Server: sj.RuntimeUpdateSelectedImage{
			Image: manifest.Server.Images[0], Digest: manifest.Server.Digests[manifest.Server.Images[0]],
			ImageID: "sha256:" + strings.Repeat("a", 64),
		},
		SteamAuth: sj.RuntimeUpdateSelectedImage{
			Image: manifest.SteamAuth.Images[0], Digest: manifest.SteamAuth.Digests[manifest.SteamAuth.Images[0]],
			ImageID: "sha256:" + strings.Repeat("b", 64),
		},
	}
	const applyID = "apply_666666666666666666666666"
	recoveryDir := filepath.Join(instance.DataDir, ".local-container", "junimo-update", "recovery", applyID)
	backups := map[string][]byte{
		"original.env": env, "original-compose.yml": compose,
		"original-control-manifest.json":                controlFiles["manifest.json"],
		"original-control-StardewAnxiPanel.Control.dll": controlFiles["StardewAnxiPanel.Control.dll"],
	}
	paths := []string{filepath.Join(instance.DataDir, ".env"), filepath.Join(instance.DataDir, "docker-compose.yml"), filepath.Join(controlDir, "manifest.json"), filepath.Join(controlDir, "StardewAnxiPanel.Control.dll")}
	for name, data := range backups {
		path := filepath.Join(recoveryDir, name)
		write(path, data)
		paths = append(paths, path)
	}
	hash := func(data []byte) string {
		sum := sha256.Sum256(data)
		return hex.EncodeToString(sum[:])
	}
	project := strings.ToLower(filepath.Base(instance.DataDir))
	f := &junimoRepairJournalFixture{
		handler: handler, store: store, admin: admin, instance: instance,
		statusPath:   filepath.Join(instance.DataDir, ".local-container", "junimo-update", "apply-status.json"),
		manifestPath: filepath.Join(recoveryDir, "manifest.json"), backupEnv: filepath.Join(recoveryDir, "original.env"),
		status: sj.RuntimeUpdateApplyStatus{
			ApplyID: applyID, Phase: sj.RuntimeUpdateApplyRollbackFailed, Current: inspection.Current,
			Target: inspection.Recommended, Selected: selected, RepairAttempts: 2,
			ErrorCode: "rollback_failed", CauseCode: "server_container_not_ready", RollbackCode: "rollback_verify_server_failed",
		},
		manifest: map[string]any{
			"schemaVersion": 3, "applyId": applyID, "actorId": 1, "project": project,
			"steamSessionVolume": project + "_steam-session",
			"snapshotVolume":     project + "_anxi-junimo-update-" + strings.TrimPrefix(applyID, "apply_") + "-steam-session",
			"serverWasRunning":   false, "authWasRunning": false, "originalState": storage.InstanceStateStopped,
			"originalServer": selected.Server, "originalAuth": selected.SteamAuth, "target": selected,
			"originalServerVersion": inspection.Current.Server.Tag, "targetServerVersion": inspection.Recommended.Server.Tag,
			"controlManifestPresent": true, "controlDllPresent": true, "controlUpdated": true,
			"mutationStarted": true, "stopIntent": true, "controlUpdateIntent": true, "lastIntent": "control_update",
			"originalEnvSha256": hash(env), "originalComposeSha256": hash(compose),
			"originalControlManifestSha256": hash(controlFiles["manifest.json"]),
			"originalControlDllSha256":      hash(controlFiles["StardewAnxiPanel.Control.dll"]),
		},
		paths: paths,
	}
	f.paths = append(f.paths, f.statusPath, f.manifestPath)
	f.writeStatus(t)
	f.writeManifest(t)
	return f
}

func (f *junimoRepairJournalFixture) writeStatus(t *testing.T) {
	t.Helper()
	writeJunimoRepairJournalJSON(t, f.statusPath, f.status)
}

func (f *junimoRepairJournalFixture) writeManifest(t *testing.T) {
	t.Helper()
	writeJunimoRepairJournalJSON(t, f.manifestPath, f.manifest)
}

func writeJunimoRepairJournalJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func (f *junimoRepairJournalFixture) snapshot(t *testing.T) map[string][32]byte {
	t.Helper()
	result := map[string][32]byte{}
	for _, path := range f.paths {
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		result[path] = sha256.Sum256(data)
	}
	return result
}

func TestJunimoUpdateJournalPlanTakesPrecedenceOverCurrentRuntime(t *testing.T) {
	for _, test := range []struct {
		name     string
		prepare  func(*testing.T, *junimoRepairJournalFixture)
		code     string
		action   string
		canRun   bool
		attempts int
	}{
		{name: "verified rollback journal", code: "repair/rollback_failed", action: "repair", canRun: true, attempts: 2},
		{name: "no journal", prepare: func(t *testing.T, f *junimoRepairJournalFixture) {
			if err := os.Remove(f.statusPath); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "completed journal", prepare: func(t *testing.T, f *junimoRepairJournalFixture) {
			f.status.Phase = sj.RuntimeUpdateApplySucceeded
			f.writeStatus(t)
		}},
		{name: "safely rolled back stale journal", prepare: func(t *testing.T, f *junimoRepairJournalFixture) {
			f.status.Phase = sj.RuntimeUpdateApplyFailedRolledBack
			f.writeStatus(t)
		}},
		{name: "corrupt status", prepare: func(t *testing.T, f *junimoRepairJournalFixture) {
			if err := os.WriteFile(f.statusPath, []byte("{broken"), 0o600); err != nil {
				t.Fatal(err)
			}
		}, code: "recovery_state_uncertain", action: "export"},
		{name: "foreign recovery scope", prepare: func(t *testing.T, f *junimoRepairJournalFixture) {
			f.manifest["project"] = "different-world"
			f.writeManifest(t)
		}, code: "recovery_state_uncertain", action: "export"},
		{name: "mismatched apply identity", prepare: func(t *testing.T, f *junimoRepairJournalFixture) {
			f.manifest["applyId"] = "apply_777777777777777777777777"
			f.writeManifest(t)
		}, code: "recovery_state_uncertain", action: "export"},
		{name: "changed recovery material", prepare: func(t *testing.T, f *junimoRepairJournalFixture) {
			if err := os.WriteFile(f.backupEnv, []byte("changed\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}, code: "recovery_material_invalid", action: "export"},
		{name: "active recovery", prepare: func(t *testing.T, f *junimoRepairJournalFixture) {
			f.status.Phase = sj.RuntimeUpdateApplyRollingBack
			f.writeStatus(t)
		}, code: "runtime_update_in_progress", action: "wait"},
		{name: "exhausted recovery", prepare: func(t *testing.T, f *junimoRepairJournalFixture) {
			f.status.RepairAttempts = 3
			f.writeStatus(t)
		}, code: "runtime_repair_exhausted", action: "export", attempts: 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newJunimoRepairJournalFixture(t)
			if test.prepare != nil {
				test.prepare(t, f)
			}
			before := f.snapshot(t)
			response, _ := doJSON(t, f.handler, http.MethodGet, "/api/instances/stardew/junimo-update", nil, f.admin)
			if response.Code != http.StatusOK {
				t.Fatalf("admin inspection = %d", response.Code)
			}
			var payload junimoUpdateResponse
			if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
				t.Fatal(err)
			}
			if payload.Status != sjconfig.RuntimeStackStatusUpToDate {
				t.Fatalf("runtime status changed: %s", payload.Status)
			}
			if test.code == "" {
				if payload.RepairPlan != nil {
					t.Fatalf("invented repair for a current runtime: code=%s", payload.RepairPlan.Code)
				}
			} else if plan := payload.RepairPlan; plan == nil || plan.Code != test.code || plan.Action != test.action || plan.ActionAvailable != test.canRun || plan.Attempts != test.attempts {
				t.Fatalf("repair plan = %+v, want code=%s action=%s available=%t attempts=%d", plan, test.code, test.action, test.canRun, test.attempts)
			}
			if strings.Contains(response.Body.String(), "journal-get-secret") || strings.Contains(response.Body.String(), f.instance.DataDir) {
				t.Fatal("public inspection exposed private recovery content")
			}
			after := f.snapshot(t)
			if len(before) != len(after) {
				t.Fatal("GET changed recovery file presence")
			}
			for path, digest := range before {
				if after[path] != digest {
					t.Fatal("GET modified the journal or its recovery material")
				}
			}
		})
	}
}

func TestJunimoUpdateJournalPlanKeepsAdminPermissionBoundary(t *testing.T) {
	f := newJunimoRepairJournalFixture(t)
	created, _ := doJSON(t, f.handler, http.MethodPost, "/api/users", map[string]string{
		"username": "journal-reader", "password": "reader-password", "role": "user",
	}, f.admin)
	if created.Code != http.StatusCreated {
		t.Fatalf("create ordinary reader = %d", created.Code)
	}
	login, reader := doJSON(t, f.handler, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "journal-reader", "password": "reader-password",
	}, nil)
	if login.Code != http.StatusOK || reader == nil {
		t.Fatalf("ordinary reader login = %d", login.Code)
	}
	for _, test := range []struct {
		name   string
		cookie *http.Cookie
		status int
	}{
		{name: "anonymous", status: http.StatusUnauthorized},
		{name: "ordinary reader", cookie: reader, status: http.StatusForbidden},
		{name: "administrator", cookie: f.admin, status: http.StatusOK},
	} {
		t.Run(test.name, func(t *testing.T) {
			response, _ := doJSON(t, f.handler, http.MethodGet, "/api/instances/stardew/junimo-update", nil, test.cookie)
			if response.Code != test.status {
				t.Fatalf("GET = %d, want %d", response.Code, test.status)
			}
			if test.status != http.StatusOK && strings.Contains(response.Body.String(), "repair/rollback_failed") {
				t.Fatal("non-administrator received the recovery plan")
			}
		})
	}
}
