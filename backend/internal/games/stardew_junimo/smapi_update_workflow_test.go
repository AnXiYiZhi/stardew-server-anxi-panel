package stardew_junimo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

type smapiWorkflowFakeDocker struct {
	*runtimeApplyFakeDocker
	dataDir               string
	mu                    sync.Mutex
	calls                 []string
	stagingInstalled      bool
	healthFailure         bool
	rollbackHealthFailure bool
	controlFailure        bool
	composeUpCount        int
	failComposeUpAt       int
}

func (f *smapiWorkflowFakeDocker) stagingActive() bool {
	env, _ := os.ReadFile(filepath.Join(f.dataDir, ".env"))
	return strings.Contains(string(env), "GAME_DATA_VOLUME="+strings.ToLower(filepath.Base(f.dataDir))+"_anxi-smapi-update-")
}

func (f *smapiWorkflowFakeDocker) call(value string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, value)
}

func (f *smapiWorkflowFakeDocker) RuntimeReadContentManifests(context.Context, string, string, string) (paneldocker.RuntimeContentRead, error) {
	manifest, _ := sjconfig.BuiltInRuntimeStackManifest()
	acf := func(appID, buildID, installDir string) []byte {
		return []byte(`"AppState" { "appid" "` + appID + `" "buildid" "` + buildID + `" "StateFlags" "4" "installdir" "` + installDir + `" }`)
	}
	return paneldocker.RuntimeContentRead{
		GameManifest:  acf("413150", manifest.Game.BuildID, "Stardew Valley"),
		SDKManifest:   acf("1007", manifest.SDK.BuildID, "Steamworks Shared"),
		FreeBytes:     20 * 1024 * 1024 * 1024,
		GameDataBytes: 5 * 1024 * 1024 * 1024,
	}, nil
}

func (f *smapiWorkflowFakeDocker) RuntimeReadSMAPIMetadata(_ context.Context, _ string, volume, _ string) (paneldocker.RuntimeSMAPIMetadata, error) {
	version := "4.4.0+abcdef0"
	if strings.Contains(volume, "_anxi-smapi-update-") && f.stagingInstalled {
		version = "4.5.2+abcdef0"
	}
	return paneldocker.RuntimeSMAPIMetadata{Present: true, RequiredFiles: true, Version: version, VersionEvidence: "StardewModdingAPI.dll AssemblyInformationalVersion"}, nil
}

func (f *smapiWorkflowFakeDocker) RuntimeCreateSMAPIStagingVolume(context.Context, string, string, string) error {
	f.call("create staging")
	return nil
}

func (f *smapiWorkflowFakeDocker) RuntimeCloneGameData(context.Context, string, string, string, string) error {
	f.call("clone staging")
	return nil
}

func (f *smapiWorkflowFakeDocker) RuntimeInstallSMAPIArchive(context.Context, string, string, string, string) error {
	f.call("official installer")
	f.stagingInstalled = true
	return nil
}

func (f *smapiWorkflowFakeDocker) RuntimeRemoveSMAPIStagingVolume(context.Context, string, string, string) error {
	f.call("remove staging")
	return nil
}

func (f *smapiWorkflowFakeDocker) ComposeUp(ctx context.Context, dir string) (paneldocker.CommandResult, error) {
	f.composeUpCount++
	f.call("compose up")
	if f.failComposeUpAt > 0 && f.composeUpCount >= f.failComposeUpAt {
		return paneldocker.CommandResult{}, errors.New("password=must-not-leak")
	}
	f.fakeDocker.psResult = paneldocker.ComposePsResult{Services: []paneldocker.ComposeService{{Service: "server", State: "running", Status: "Up"}, {Service: "steam-auth", State: "running", Status: "Up"}}}
	if !f.controlFailure || !f.stagingActive() {
		manifest, _ := sjconfig.BuiltInRuntimeStackManifest()
		control := filepath.Join(f.dataDir, ".local-container", "control")
		_ = os.MkdirAll(control, 0o700)
		_ = os.WriteFile(filepath.Join(control, "options.json"), []byte(`{"controlModVersion":"`+manifest.Control.Version+`"}`), 0o600)
	}
	return f.runtimeApplyFakeDocker.ComposeUp(ctx, dir)
}

func (f *smapiWorkflowFakeDocker) RuntimeServerHealth(context.Context, string, string) error {
	if !f.controlFailure || !f.stagingActive() {
		control := filepath.Join(f.dataDir, ".local-container", "control")
		_ = os.MkdirAll(control, 0o700)
		_ = os.WriteFile(filepath.Join(control, "status.json"), []byte(`{"state":"save-loaded","updatedAt":"2026-07-14T00:00:00Z","commandResultVersion":1}`), 0o600)
		_ = os.WriteFile(filepath.Join(control, "players.json"), []byte(`{"updatedAt":"2026-07-14T00:00:00Z","players":[]}`), 0o600)
	}
	if f.healthFailure && f.stagingActive() || f.rollbackHealthFailure && !f.stagingActive() {
		return errors.New("Junimo health unavailable")
	}
	return nil
}

func TestSMAPIUpdateUsesRealDockerStateWhenDatabaseIsStopped(t *testing.T) {
	driver, instance, fake := setupSMAPIWorkflowDriver(t, storage.InstanceStateStopped)
	withFakeSMAPIArchive(t, instance.DataDir)
	fake.fakeDocker.psResult = paneldocker.ComposePsResult{Services: []paneldocker.ComposeService{{Service: "server", State: "running"}, {Service: "steam-auth", State: "running"}}}
	if _, err := driver.RunSMAPIUpdateDryRun(context.Background(), instance); err != nil {
		t.Fatal(err)
	}
	started, err := driver.StartSMAPIUpdateApply(context.Background(), instance, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !started.ServerWasRunning {
		t.Fatal("real running server was not recorded")
	}
	status := waitSMAPIApply(t, driver, instance)
	if status.Phase != SMAPIApplySucceeded || !status.ServerWasRunning {
		t.Fatalf("status=%#v", status)
	}
	if !strings.Contains(strings.Join(fake.applyCalls, "\n"), "compose down") {
		t.Fatal("real running stack was not gracefully stopped")
	}
}

func TestSMAPIUpdateRestartBeforeSwitchRestoresPreviouslyRunningServer(t *testing.T) {
	driver, instance, fake := setupSMAPIWorkflowDriver(t, storage.InstanceStateRunning)
	updateID := "apply_" + strings.Repeat("d", 24)
	project := strings.ToLower(filepath.Base(instance.DataDir))
	status := SMAPIUpdateStatus{UpdateID: updateID, Phase: SMAPIApplyStopping, Current: SMAPICurrent{Version: "4.4.0"}, ServerWasRunning: true, Checks: []RuntimeUpdateDryRunCheck{}, Warnings: []string{}, Logs: []RuntimeUpdateDryRunLog{}}
	recovery := smapiRecoveryManifest{SchemaVersion: 1, UpdateID: updateID, Project: project, OriginalVolume: project + "_game-data", StagingVolume: project + "_anxi-smapi-update-" + strings.Repeat("d", 24), ServerWasRunning: true}
	if err := writeSMAPIUpdateStatus(instance.DataDir, "apply-status.json", status); err != nil {
		t.Fatal(err)
	}
	if err := writeSMAPIRecoveryManifest(instance.DataDir, recovery); err != nil {
		t.Fatal(err)
	}
	if err := driver.RecoverSMAPIUpdateApply(context.Background(), instance); err != nil {
		t.Fatal(err)
	}
	restored := waitSMAPIApply(t, driver, instance)
	if restored.Phase != SMAPIApplyFailedRolledBack {
		t.Fatalf("status=%#v", restored)
	}
	if !strings.Contains(strings.Join(fake.calls, "\n"), "compose up") {
		t.Fatal("old running server was not restarted")
	}
}

func TestSMAPIRollbackAcceptanceFailureKeepsRecoveryMaterials(t *testing.T) {
	driver, instance, fake := setupSMAPIWorkflowDriver(t, storage.InstanceStateStopped)
	withFakeSMAPIArchive(t, instance.DataDir)
	fake.healthFailure = true
	fake.rollbackHealthFailure = true
	if _, err := driver.RunSMAPIUpdateDryRun(context.Background(), instance); err != nil {
		t.Fatal(err)
	}
	started, err := driver.StartSMAPIUpdateApply(context.Background(), instance, 0)
	if err != nil {
		t.Fatal(err)
	}
	status := waitSMAPIApply(t, driver, instance)
	if status.Phase != SMAPIApplyRollbackFailed {
		t.Fatalf("status=%#v", status)
	}
	if _, err := os.Stat(smapiRecoveryDir(instance.DataDir, started.UpdateID)); err != nil {
		t.Fatalf("recovery materials removed: %v", err)
	}
}

func (f *smapiWorkflowFakeDocker) ComposeLogs(context.Context, string, paneldocker.LogsOptions) (paneldocker.CommandResult, error) {
	logs := "[SMAPI] Loaded JunimoServer"
	if !f.controlFailure || !f.stagingActive() {
		logs += "\n[SMAPI] Loaded StardewAnxiPanel.Control"
	}
	return paneldocker.CommandResult{Stdout: logs}, nil
}

func setupSMAPIWorkflowDriver(t *testing.T, state string) (*Driver, registry.Instance, *smapiWorkflowFakeDocker) {
	t.Helper()
	base, store, instance, applyFake := setupRuntimeApplyDriver(t, state)
	manifest, err := sjconfig.BuiltInRuntimeStackManifest()
	if err != nil {
		t.Fatal(err)
	}
	project := strings.ToLower(filepath.Base(instance.DataDir))
	env := strings.Join([]string{
		"IMAGE_VERSION=" + manifest.Server.Tag,
		"SERVER_IMAGE=" + manifest.Server.Image,
		"SERVER_IMAGE_CANDIDATES=" + strings.Join(manifest.Server.TrustedCandidates, ","),
		"STEAM_SERVICE_IMAGE=" + manifest.SteamAuth.Image,
		"STEAM_SERVICE_IMAGE_CANDIDATES=" + strings.Join(manifest.SteamAuth.TrustedCandidates, ","),
		"GAME_DATA_VOLUME=" + project + "_game-data",
		"SMAPI_VERSION=4.4.0",
		"STEAMCMD_AUTH_COMPLETED=true",
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(instance.DataDir, ".env"), []byte(env), 0o600); err != nil {
		t.Fatal(err)
	}
	fake := &smapiWorkflowFakeDocker{runtimeApplyFakeDocker: applyFake, dataDir: instance.DataDir}
	driver := New(fake, base.logger, jobs.NewManager(store, base.logger), store)
	driver.runtimeUpdatePollInterval = time.Millisecond
	driver.runtimeUpdateAuthTimeout = 25 * time.Millisecond
	driver.runtimeUpdateServerTimeout = 25 * time.Millisecond
	return driver, instance, fake
}

func withFakeSMAPIArchive(t *testing.T, dataDir string) {
	t.Helper()
	manifest, _ := sjconfig.BuiltInRuntimeStackManifest()
	archive := filepath.Join(t.TempDir(), "SMAPI-"+manifest.SMAPI.Version+"-installer.zip")
	if err := os.WriteFile(archive, []byte("controlled test archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	oldEnsure, oldValidate := ensureSMAPIArchiveForUpdate, validateSMAPIArchiveForUpdate
	ensureSMAPIArchiveForUpdate = func(context.Context, string, sjconfig.RuntimeStackManifest) (string, error) { return archive, nil }
	validateSMAPIArchiveForUpdate = func(string, sjconfig.RuntimeStackManifest) error { return nil }
	t.Cleanup(func() {
		ensureSMAPIArchiveForUpdate, validateSMAPIArchiveForUpdate = oldEnsure, oldValidate
	})
}

func waitSMAPIApply(t *testing.T, driver *Driver, instance registry.Instance) SMAPIUpdateStatus {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	var last SMAPIUpdateStatus
	for time.Now().Before(deadline) {
		status, err := driver.SMAPIUpdateApplyStatus(instance)
		if err == nil {
			last = status
			if smapiApplyTerminal(status.Phase) {
				return status
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("SMAPI apply did not finish: %#v", last)
	return last
}

func TestSMAPIUpdateWorkflowSuccessUsesStagingAndPreservesOldVolume(t *testing.T) {
	driver, instance, fake := setupSMAPIWorkflowDriver(t, storage.InstanceStateStopped)
	withFakeSMAPIArchive(t, instance.DataDir)
	dry, err := driver.RunSMAPIUpdateDryRun(context.Background(), instance)
	if err != nil || dry.Phase != RuntimeUpdatePhaseSucceeded {
		t.Fatalf("dry-run: %#v, %v", dry, err)
	}
	if strings.Contains(strings.Join(fake.calls, "\n"), "create staging") {
		t.Fatal("dry-run created a staging volume")
	}
	if _, err := driver.StartSMAPIUpdateApply(context.Background(), instance, 0); err != nil {
		t.Fatal(err)
	}
	status := waitSMAPIApply(t, driver, instance)
	if status.Phase != SMAPIApplySucceeded {
		t.Fatalf("apply status: %#v", status)
	}
	env, _ := os.ReadFile(filepath.Join(instance.DataDir, ".env"))
	if !strings.Contains(string(env), "GAME_DATA_VOLUME="+strings.ToLower(filepath.Base(instance.DataDir))+"_anxi-smapi-update-") {
		t.Fatalf("staging volume not activated: %s", env)
	}
	calls := strings.Join(fake.calls, "\n")
	for _, want := range []string{"create staging", "clone staging", "official installer"} {
		if !strings.Contains(calls, want) {
			t.Fatalf("missing %q in %s", want, calls)
		}
	}
	if strings.Contains(calls, "remove staging") {
		t.Fatal("active verified game-data volume was removed")
	}
}

func TestSMAPIUpdateWorkflowAcceptsLoggedOutAuth(t *testing.T) {
	driver, instance, fake := setupSMAPIWorkflowDriver(t, storage.InstanceStateStopped)
	withFakeSMAPIArchive(t, instance.DataDir)
	fake.authFailTarget = true
	fake.authReady = false
	fake.authTicket = false
	fake.inviteUnavailable = true
	if _, err := driver.RunSMAPIUpdateDryRun(context.Background(), instance); err != nil {
		t.Fatal(err)
	}
	if _, err := driver.StartSMAPIUpdateApply(context.Background(), instance, 0); err != nil {
		t.Fatal(err)
	}
	status := waitSMAPIApply(t, driver, instance)
	if status.Phase != SMAPIApplySucceeded {
		t.Fatalf("logged-out LAN-only runtime was rejected: %#v", status)
	}
}

func TestSMAPIUpdateWorkflowVerificationFailuresRollbackVolumeAndControl(t *testing.T) {
	for _, test := range []struct {
		name      string
		configure func(*smapiWorkflowFakeDocker)
		want      string
	}{
		{"control mod not loaded", func(f *smapiWorkflowFakeDocker) { f.controlFailure = true }, SMAPIApplyFailedRolledBack},
		{"Junimo API failed", func(f *smapiWorkflowFakeDocker) { f.healthFailure = true }, SMAPIApplyFailedRolledBack},
		{"rollback start failed", func(f *smapiWorkflowFakeDocker) { f.healthFailure = true; f.failComposeUpAt = 2 }, SMAPIApplyRollbackFailed},
	} {
		t.Run(test.name, func(t *testing.T) {
			driver, instance, fake := setupSMAPIWorkflowDriver(t, storage.InstanceStateStopped)
			withFakeSMAPIArchive(t, instance.DataDir)
			if err := os.MkdirAll(smapiModDir(instance.DataDir), 0o700); err != nil {
				t.Fatal(err)
			}
			oldManifest := []byte("old-control-manifest")
			oldDLL := []byte("old-control-dll")
			if err := os.WriteFile(filepath.Join(smapiModDir(instance.DataDir), "manifest.json"), oldManifest, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(smapiModDir(instance.DataDir), "StardewAnxiPanel.Control.dll"), oldDLL, 0o600); err != nil {
				t.Fatal(err)
			}
			test.configure(fake)
			dry, _ := driver.RunSMAPIUpdateDryRun(context.Background(), instance)
			if dry.Phase != RuntimeUpdatePhaseSucceeded {
				t.Fatalf("dry-run: %#v", dry)
			}
			if _, err := driver.StartSMAPIUpdateApply(context.Background(), instance, 0); err != nil {
				t.Fatal(err)
			}
			status := waitSMAPIApply(t, driver, instance)
			if status.Phase != test.want {
				t.Fatalf("phase=%s error=%s", status.Phase, status.Error)
			}
			env, _ := os.ReadFile(filepath.Join(instance.DataDir, ".env"))
			if !strings.Contains(string(env), "GAME_DATA_VOLUME="+strings.ToLower(filepath.Base(instance.DataDir))+"_game-data") {
				t.Fatalf("old volume not restored: %s", env)
			}
			if status.Phase == SMAPIApplyFailedRolledBack {
				gotManifest, _ := os.ReadFile(filepath.Join(smapiModDir(instance.DataDir), "manifest.json"))
				gotDLL, _ := os.ReadFile(filepath.Join(smapiModDir(instance.DataDir), "StardewAnxiPanel.Control.dll"))
				if string(gotManifest) != string(oldManifest) || string(gotDLL) != string(oldDLL) {
					t.Fatal("old Control Mod was not restored")
				}
				if !strings.Contains(strings.Join(fake.calls, "\n"), "remove staging") {
					t.Fatal("failed staging volume was not removed")
				}
			} else {
				if status.ManualAction == "" {
					t.Fatal("rollback_failed omitted manual recovery guidance")
				}
				if _, err := os.Stat(smapiRecoveryDir(instance.DataDir, status.UpdateID)); err != nil {
					t.Fatal("rollback_failed removed recovery materials")
				}
				serialized := status.Error + status.ManualAction + strings.Join(status.Warnings, " ")
				if strings.Contains(serialized, "must-not-leak") || strings.Contains(serialized, "password=") {
					t.Fatalf("secret leaked: %s", serialized)
				}
			}
		})
	}
}

func TestSMAPIControlRollbackRestoresPreviouslyAbsentFiles(t *testing.T) {
	dataDir := t.TempDir()
	updateID := "apply_" + strings.Repeat("a", 24)
	if _, _, err := backupControlMod(dataDir, updateID); err != nil {
		t.Fatal(err)
	}
	if err := installSMAPIMod(dataDir); err != nil {
		t.Fatal(err)
	}
	if err := restoreControlMod(dataDir, smapiRecoveryManifest{UpdateID: updateID}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"manifest.json", "StardewAnxiPanel.Control.dll"} {
		if _, err := os.Stat(filepath.Join(smapiModDir(dataDir), name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("previously absent %s survived rollback: %v", name, err)
		}
	}
}

func TestRequiredSMAPIModsLoaded(t *testing.T) {
	junimo, control := requiredSMAPIModsLoaded("[SMAPI] JunimoServer 1.5\n[SMAPI] StardewAnxiPanel.Control 0.1")
	if !junimo || !control {
		t.Fatal("required Mod log evidence was not recognized")
	}
	junimo, control = requiredSMAPIModsLoaded("[SMAPI] JunimoServer 1.5")
	if !junimo || control {
		t.Fatal("missing Control Mod was not distinguished")
	}
}
