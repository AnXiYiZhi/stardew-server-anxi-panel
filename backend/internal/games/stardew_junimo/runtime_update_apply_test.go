package stardew_junimo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

type runtimeApplyFakeDocker struct {
	*runtimeUpdateFakeDocker
	applyMu                      sync.Mutex
	applyCalls                   []string
	authLoggedIn                 bool
	authHealth                   string
	authContainerState           string
	authContainerStateAfterUp    string
	authUseTargetState           bool
	authProbeErrorTarget         bool
	authProbeErrorCode           string
	inviteUnavailable            bool
	serverHealthFailTarget       bool
	controlContractFail          bool
	loadedVersionMismatch        bool
	digestMismatchService        string
	digestMismatchAfterUpService string
	runtimeServicesApplied       bool
	upErrorService               string
	restoreError                 bool
	removeImageError             bool
	removeImageStarted           chan struct{}
	removeImageRelease           <-chan struct{}
	removeImageStartOnce         sync.Once
	stopErrorsRemaining          int
}

func newRuntimeApplyFakeDocker(dataDir string) *runtimeApplyFakeDocker {
	return &runtimeApplyFakeDocker{runtimeUpdateFakeDocker: newRuntimeUpdateFakeDocker(dataDir), authLoggedIn: true, authHealth: "healthy", authContainerState: "running", authProbeErrorCode: "auth_health_unreachable"}
}
func (f *runtimeApplyFakeDocker) applyCall(call string) {
	f.applyMu.Lock()
	defer f.applyMu.Unlock()
	f.applyCalls = append(f.applyCalls, call)
}
func (f *runtimeApplyFakeDocker) ComposeUp(context.Context, string) (paneldocker.CommandResult, error) {
	f.applyCall("compose up")
	return paneldocker.CommandResult{}, nil
}
func (f *runtimeApplyFakeDocker) ComposeDown(context.Context, string) (paneldocker.CommandResult, error) {
	f.applyCall("compose down")
	return paneldocker.CommandResult{}, nil
}
func (f *runtimeApplyFakeDocker) ComposeRestart(context.Context, string) (paneldocker.CommandResult, error) {
	f.applyCall("compose restart")
	return paneldocker.CommandResult{}, nil
}
func (f *runtimeApplyFakeDocker) ComposeRestartServices(context.Context, string, ...string) (paneldocker.CommandResult, error) {
	f.applyCall("compose restart services")
	return paneldocker.CommandResult{}, nil
}
func (f *runtimeApplyFakeDocker) ComposeRecreateServices(context.Context, string, ...string) (paneldocker.CommandResult, error) {
	f.applyCall("compose recreate services")
	return paneldocker.CommandResult{}, nil
}
func (f *runtimeApplyFakeDocker) ComposeExecPipe(_ context.Context, dataDir string, service, stdin string, args ...string) (paneldocker.CommandResult, error) {
	f.applyCall("compose exec " + service + " " + strings.Join(args, " "))
	if service == "server" && len(args) > 0 && args[0] == "cat" {
		if f.inviteUnavailable {
			return paneldocker.CommandResult{}, errors.New("invite code unavailable")
		}
		return paneldocker.CommandResult{Stdout: "ABC123\n"}, nil
	}
	if service == "server" && len(args) > 0 && args[0] == "wc" {
		return paneldocker.CommandResult{Stdout: "128 /tmp/server-output.log\n"}, nil
	}
	if service == "server" && len(args) > 0 && args[0] == "tail" {
		if f.targetConfigured(dataDir) && f.controlContractFail {
			return paneldocker.CommandResult{}, nil
		}
		version := "1.4.0-preview.1"
		if f.targetConfigured(dataDir) {
			version = "1.5.0-preview.125"
			if f.loadedVersionMismatch {
				version = "1.5.0-preview.121"
			}
		}
		return paneldocker.CommandResult{Stdout: "[INFO JunimoServer] --- Server Info ---\n[INFO JunimoServer] Version: " + version + "\n[INFO JunimoServer] Status: Ready\n"}, nil
	}
	return paneldocker.CommandResult{Stdout: "Junimo API ok\nABC123\n"}, nil
}
func (f *runtimeApplyFakeDocker) ComposeExecTTY(context.Context, string, string, string, ...string) (paneldocker.ComposeExecTTYResult, error) {
	return paneldocker.ComposeExecTTYResult{}, nil
}
func (f *runtimeApplyFakeDocker) ComposeLogs(context.Context, string, paneldocker.LogsOptions) (paneldocker.CommandResult, error) {
	return paneldocker.CommandResult{}, nil
}
func (f *runtimeApplyFakeDocker) RuntimeComposeStopServices(_ context.Context, _ string, _ string, services ...string) error {
	f.applyCall("stop " + strings.Join(services, ","))
	if f.stopErrorsRemaining > 0 {
		f.stopErrorsRemaining--
		return errors.New("docker command timed out")
	}
	return nil
}
func (f *runtimeApplyFakeDocker) RuntimeComposeUpService(_ context.Context, dataDir string, _ string, service string) error {
	f.applyCall("up " + service)
	f.runtimeServicesApplied = true
	if service == f.upErrorService {
		return errors.New("injected up failure")
	}
	if service == "server" {
		manifest, _ := sjconfig.BuiltInRuntimeStackManifest()
		_ = os.MkdirAll(filepath.Join(dataDir, ".local-container", "control"), 0o755)
		_ = os.WriteFile(filepath.Join(dataDir, ".local-container", "control", "options.json"), []byte(readyControlRuntimeOptions(manifest.Control.Version)), 0o600)
		_ = os.WriteFile(filepath.Join(dataDir, ".local-container", "control", "status.json"), []byte(`{"state":"save-loaded","commandResultVersion":1,"updatedAt":"2026-07-20T00:00:00Z"}`), 0o600)
		_ = os.WriteFile(filepath.Join(dataDir, ".local-container", "control", "players.json"), []byte(`{"players":[],"updatedAt":"2026-07-20T00:00:00Z"}`), 0o600)
	}
	return nil
}
func (f *runtimeApplyFakeDocker) RuntimeComposeUpServicePreserve(_ context.Context, dataDir string, _ string, service string) error {
	f.applyCall("up preserve " + service)
	f.runtimeServicesApplied = true
	if service == f.upErrorService {
		return errors.New("injected preserve up failure")
	}
	return nil
}
func (f *runtimeApplyFakeDocker) RuntimeUpdateServiceCPUShares(_ context.Context, _ string, _ string, service string, shares int64) error {
	f.applyCall(fmt.Sprintf("cpu shares %s %d", service, shares))
	return nil
}
func (f *runtimeApplyFakeDocker) targetConfigured(dataDir string) bool {
	env, _ := os.ReadFile(filepath.Join(dataDir, ".env"))
	return strings.Contains(string(env), "IMAGE_VERSION=1.5.0-preview.125")
}
func (f *runtimeApplyFakeDocker) RuntimeServiceInspect(_ context.Context, dataDir, _ string, service string) (paneldocker.RuntimeServiceMetadata, error) {
	f.applyCall("inspect " + service)
	digest := "sha256:" + strings.Repeat("a", 64)
	if !f.targetConfigured(dataDir) {
		if service == "server" {
			digest = "sha256:" + strings.Repeat("b", 64)
		} else {
			digest = "sha256:" + strings.Repeat("c", 64)
		}
	}
	if f.targetConfigured(dataDir) && (f.digestMismatchService == service || f.runtimeServicesApplied && f.digestMismatchAfterUpService == service) {
		digest = "sha256:" + strings.Repeat("d", 64)
	}
	health := "healthy"
	state := "running"
	if service == "steam-auth" {
		health = f.authHealth
		if f.targetConfigured(dataDir) {
			state = f.authContainerState
			if f.runtimeServicesApplied && f.authContainerStateAfterUp != "" {
				state = f.authContainerStateAfterUp
			}
		}
	}
	return paneldocker.RuntimeServiceMetadata{ContainerID: strings.Repeat("a", 12), ImageID: digest, State: state, Health: health}, nil
}
func (f *runtimeApplyFakeDocker) RuntimeSteamAuthHealth(_ context.Context, dataDir, _ string) (paneldocker.RuntimeAuthServiceHealth, error) {
	scope := "original"
	if f.targetConfigured(dataDir) {
		scope = "target"
	}
	f.applyCall("auth health " + scope)
	if f.targetConfigured(dataDir) && f.authProbeErrorTarget {
		return paneldocker.RuntimeAuthServiceHealth{}, &paneldocker.RuntimeAuthHealthError{Code: f.authProbeErrorCode, Message: "steam-auth-cn /health 受控测试失败。"}
	}
	if f.targetConfigured(dataDir) && f.authUseTargetState {
		return paneldocker.RuntimeAuthServiceHealth{LoggedIn: f.authLoggedIn}, nil
	}
	return paneldocker.RuntimeAuthServiceHealth{LoggedIn: true, AccountCount: 1}, nil
}
func (f *runtimeApplyFakeDocker) RuntimeServerHealth(_ context.Context, dataDir, _ string) error {
	if f.targetConfigured(dataDir) && f.serverHealthFailTarget {
		return errors.New("health failed")
	}
	values, err := sjconfig.ReadEnvFile(filepath.Join(dataDir, ".env"))
	if err != nil {
		return err
	}
	err = validateExtractedJunimoServerMod(junimoServerModDir(dataDir), values["IMAGE_VERSION"])
	if err != nil {
		f.applyCall("server health " + err.Error())
	}
	return err
}
func (f *runtimeApplyFakeDocker) RuntimeCreateSnapshotVolume(context.Context, string, string, string) error {
	f.applyCall("volume create snapshot")
	return nil
}
func (f *runtimeApplyFakeDocker) RuntimeCloneVolume(context.Context, string, string, string, string) error {
	f.applyCall("volume clone snapshot")
	return nil
}
func (f *runtimeApplyFakeDocker) RuntimeRestoreVolume(context.Context, string, string, string, string) error {
	f.applyCall("volume restore snapshot")
	if f.restoreError {
		return errors.New("refresh_token=super-secret-rollback-token")
	}
	return nil
}
func (f *runtimeApplyFakeDocker) RuntimeRemoveSnapshotVolume(context.Context, string, string, string) error {
	f.applyCall("volume rm snapshot")
	return nil
}
func (f *runtimeApplyFakeDocker) RuntimeRemoveImage(_ context.Context, _ string, image, expectedID string) error {
	f.applyCall("image rm " + image + " " + expectedID)
	if f.removeImageStarted != nil {
		f.removeImageStartOnce.Do(func() { close(f.removeImageStarted) })
		if f.removeImageRelease != nil {
			<-f.removeImageRelease
		}
	}
	if f.removeImageError {
		return errors.New("image still in use")
	}
	return nil
}

func setupRuntimeApplyDriver(t *testing.T, state string) (*Driver, *storage.Store, registry.Instance, *runtimeApplyFakeDocker) {
	base, store, instance, _ := setupRuntimeUpdateDriver(t, state)
	fake := newRuntimeApplyFakeDocker(instance.DataDir)
	if state == storage.InstanceStateStopped {
		fake.fakeDocker.psResult = paneldocker.ComposePsResult{Services: []paneldocker.ComposeService{{Service: "server", State: "exited"}, {Service: "steam-auth", State: "exited"}}}
	}
	driver := New(fake, base.logger, base.jobs, store)
	driver.runtimeUpdatePollInterval = time.Millisecond
	driver.runtimeUpdateAuthTimeout = 15 * time.Millisecond
	driver.runtimeUpdateServerTimeout = 15 * time.Millisecond
	driver.runtimeUpdateStopTimeout = 25 * time.Millisecond
	if err := installSMAPIMod(instance.DataDir); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(instance.DataDir, ".local-container", "control"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(instance.DataDir, ".local-container", "control", "status.json"), []byte(`{"state":"save-loaded","commandResultVersion":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	manifest, _ := sjconfig.BuiltInRuntimeStackManifest()
	if err := os.WriteFile(filepath.Join(instance.DataDir, ".local-container", "control", "options.json"), []byte(readyControlRuntimeOptions(manifest.Control.Version)), 0o600); err != nil {
		t.Fatal(err)
	}
	oldJunimoDir := junimoServerModDir(instance.DataDir)
	if err := os.MkdirAll(oldJunimoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldJunimoDir, junimoServerManifestName), []byte(`{"Name":"JunimoServer","Version":"1.4.0-preview.1","UniqueID":"JunimoHost.Server"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldJunimoDir, junimoServerAssemblyName), []byte("old assembly"), 0o644); err != nil {
		t.Fatal(err)
	}
	inspection := InspectRuntimeStack(instance.DataDir, instance.State)
	status := RuntimeUpdateDryRunStatus{DryRunID: "dryrun_test", Phase: RuntimeUpdatePhaseSucceeded, Current: inspection.Current, Target: inspection.Recommended, Selected: RuntimeUpdateSelectedPair{Server: RuntimeUpdateSelectedImage{Image: inspection.Recommended.Server.TrustedCandidates[0], Digest: "sha256:" + strings.Repeat("a", 64)}, SteamAuth: RuntimeUpdateSelectedImage{Image: inspection.Recommended.SteamAuth.TrustedCandidates[0], Digest: "sha256:" + strings.Repeat("a", 64)}}}
	if err := writeRuntimeUpdateDryRunStatus(instance.DataDir, status); err != nil {
		t.Fatal(err)
	}
	return driver, store, instance, fake
}

func configureDisabledRuntimeApplyFixture(t *testing.T, instance registry.Instance, fake *runtimeApplyFakeDocker) {
	t.Helper()
	if err := sjconfig.SetSteamInviteEnabled(instance.DataDir, false); err != nil {
		t.Fatal(err)
	}
	if err := sjconfig.UpdateEnvFile(filepath.Join(instance.DataDir, ".env"), map[string]string{
		"STEAM_AUTH_COMPLETED":           "true",
		"STEAM_SERVICE_IMAGE":            "invalid optional auth image",
		"STEAM_SERVICE_IMAGE_CANDIDATES": "invalid optional auth candidates",
		"STEAM_INVITE_AUTH_STATE":        sjconfig.SteamInviteAuthStateDisabled,
	}); err != nil {
		t.Fatal(err)
	}
	fake.composeConfig.Services = []string{"server"}
	fake.composeConfig.SteamSessionVolume = ""
	serverState := "running"
	if instance.State == storage.InstanceStateStopped {
		serverState = "exited"
	}
	fake.fakeDocker.psResult = paneldocker.ComposePsResult{Services: []paneldocker.ComposeService{{Service: "server", State: serverState}}}
	inspection := InspectManagedRuntimeStack(instance.DataDir, instance.State)
	status := RuntimeUpdateDryRunStatus{
		DryRunID: "dryrun_disabled_test",
		Phase:    RuntimeUpdatePhaseSucceeded,
		Current:  inspection.Current,
		Target:   inspection.Recommended,
		Selected: RuntimeUpdateSelectedPair{Server: RuntimeUpdateSelectedImage{
			Image:  inspection.Recommended.Server.TrustedCandidates[0],
			Digest: "sha256:" + strings.Repeat("a", 64),
		}},
	}
	if err := writeRuntimeUpdateDryRunStatus(instance.DataDir, status); err != nil {
		t.Fatal(err)
	}
}

func assertNoOptionalAuthRuntimeCalls(t *testing.T, fake *runtimeApplyFakeDocker) {
	t.Helper()
	calls := strings.Join(append(append([]string{}, fake.calls...), fake.applyCalls...), "\n")
	for _, forbidden := range []string{
		"steam-auth",
		"auth health",
		"docker volume inspect",
		"full stack",
		"volume create",
		"volume clone",
		"volume restore",
		"volume rm",
	} {
		if strings.Contains(calls, forbidden) {
			t.Fatalf("disabled runtime update touched optional Auth via %q:\n%s", forbidden, calls)
		}
	}
}

func waitRuntimeApply(t *testing.T, driver *Driver, instance registry.Instance) RuntimeUpdateApplyStatus {
	t.Helper()
	time.Sleep(250 * time.Millisecond)
	// A clean Linux release gate compiles and exercises several SQLite-heavy
	// packages at once. Leave enough scheduling headroom for the async job to
	// start; the injected apply path itself still uses millisecond timeouts.
	deadline := time.Now().Add(20 * time.Second)
	var last RuntimeUpdateApplyStatus
	for time.Now().Before(deadline) {
		status, err := driver.RuntimeUpdateApplyStatus(instance)
		if err == nil {
			last = status
			if runtimeUpdateApplyTerminal(status.Phase) {
				return status
			}
		}
		time.Sleep(75 * time.Millisecond)
	}
	t.Fatalf("apply did not finish: %#v", last)
	return last
}

func TestRuntimeUpdateApplySuccessUpdatesPairAndPreservesSafetyBoundary(t *testing.T) {
	driver, _, instance, fake := setupRuntimeApplyDriver(t, storage.InstanceStateRunning)
	if _, err := driver.StartRuntimeUpdateApply(context.Background(), instance, 0); err != nil {
		t.Fatal(err)
	}
	status := waitRuntimeApply(t, driver, instance)
	if status.Phase != RuntimeUpdateApplySucceeded || !status.ServerRunning {
		t.Fatalf("unexpected status: %#v", status)
	}
	cleanupCalls := strings.Join(fake.applyCalls, "\n")
	if strings.Count(cleanupCalls, "image rm ") != 2 {
		t.Fatalf("successful runtime apply did not clean both old images: %s", cleanupCalls)
	}
	env, _ := os.ReadFile(filepath.Join(instance.DataDir, ".env"))
	text := string(env)
	if !strings.Contains(text, "IMAGE_VERSION=1.5.0-preview.125") || !strings.Contains(text, "STEAM_SERVICE_IMAGE=") {
		t.Fatalf("version pair not written: %s", text)
	}
	calls := strings.Join(fake.applyCalls, "\n")
	for _, forbidden := range []string{"down -v", "volume rm stardew_steam-session", "stop server\nup server\nup steam-auth"} {
		if strings.Contains(calls, forbidden) {
			t.Fatalf("forbidden operation %q: %s", forbidden, calls)
		}
	}
	if !strings.Contains(calls, "up steam-auth") || !strings.Contains(calls, "up server") {
		t.Fatalf("pair not recreated: %s", calls)
	}
	version, err := readJunimoServerModVersion(junimoServerModDir(instance.DataDir))
	if err != nil || version != "1.5.0-preview.125" {
		t.Fatalf("host-mounted JunimoServer mod was not upgraded: version=%q err=%v", version, err)
	}
	if !strings.Contains(calls, "tee -a "+serverInputFIFO) || strings.Contains(calls, "attach-cli") {
		t.Fatalf("runtime verification did not use the FIFO control contract: %s", calls)
	}
}

func TestRuntimeUpdateApplyDisabledScopesSuccessToServerOnly(t *testing.T) {
	for _, state := range []string{storage.InstanceStateRunning, storage.InstanceStateStopped} {
		t.Run(state, func(t *testing.T) {
			driver, _, instance, fake := setupRuntimeApplyDriver(t, state)
			configureDisabledRuntimeApplyFixture(t, instance, fake)
			if _, err := driver.StartRuntimeUpdateApply(context.Background(), instance, 0); err != nil {
				t.Fatal(err)
			}
			status := waitRuntimeApply(t, driver, instance)
			if status.Phase != RuntimeUpdateApplySucceeded || status.ServerRunning != (state == storage.InstanceStateRunning) {
				t.Fatalf("disabled apply status=%#v calls=%v", status, fake.applyCalls)
			}
			if status.Selected.SteamAuth != (RuntimeUpdateSelectedImage{}) {
				t.Fatalf("disabled apply selected Auth image: %#v", status.Selected.SteamAuth)
			}
			calls := strings.Join(fake.applyCalls, "\n")
			if !strings.Contains(calls, "up server") || !strings.Contains(calls, "stop server") {
				t.Fatalf("disabled apply did not maintain only server: %s", calls)
			}
			fields, err := sjconfig.ReadEnvFile(filepath.Join(instance.DataDir, ".env"))
			if err != nil {
				t.Fatal(err)
			}
			if fields["STEAM_INVITE_ENABLED"] != "false" || fields["STEAM_AUTH_COMPLETED"] != "true" || fields["STEAM_SERVICE_IMAGE"] != "invalid optional auth image" {
				t.Fatalf("disabled apply changed optional Auth intent/session/config: %#v", fields)
			}
			assertNoOptionalAuthRuntimeCalls(t, fake)
		})
	}
}

func TestRuntimeUpdateApplyDisabledRollbackNeverTouchesAuth(t *testing.T) {
	driver, _, instance, fake := setupRuntimeApplyDriver(t, storage.InstanceStateRunning)
	configureDisabledRuntimeApplyFixture(t, instance, fake)
	fake.serverHealthFailTarget = true
	if _, err := driver.StartRuntimeUpdateApply(context.Background(), instance, 0); err != nil {
		t.Fatal(err)
	}
	status := waitRuntimeApply(t, driver, instance)
	if status.Phase != RuntimeUpdateApplyFailedRolledBack || !status.ServerRunning {
		t.Fatalf("disabled rollback status=%#v calls=%v", status, fake.applyCalls)
	}
	fields, err := sjconfig.ReadEnvFile(filepath.Join(instance.DataDir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if fields["STEAM_INVITE_ENABLED"] != "false" || fields["STEAM_SERVICE_IMAGE"] != "invalid optional auth image" {
		t.Fatalf("disabled rollback did not restore original Auth fields: %#v", fields)
	}
	if calls := strings.Join(fake.applyCalls, "\n"); !strings.Contains(calls, "up server") || !strings.Contains(calls, "stop server") {
		t.Fatalf("disabled rollback did not restore server: %s", calls)
	}
	assertNoOptionalAuthRuntimeCalls(t, fake)
}

func configureControlOnlyRuntimeFixture(t *testing.T, instance registry.Instance) sjconfig.RuntimeStackManifest {
	t.Helper()
	manifest, err := sjconfig.BuiltInRuntimeStackManifest()
	if err != nil {
		t.Fatal(err)
	}
	if err := sjconfig.UpdateEnvFile(filepath.Join(instance.DataDir, ".env"), map[string]string{
		"IMAGE_VERSION":                  manifest.Server.Tag,
		"SERVER_IMAGE":                   manifest.Server.TrustedCandidates[0],
		"SERVER_IMAGE_CANDIDATES":        strings.Join(manifest.Server.TrustedCandidates, ","),
		"STEAM_SERVICE_IMAGE":            manifest.SteamAuth.TrustedCandidates[0],
		"STEAM_SERVICE_IMAGE_CANDIDATES": strings.Join(manifest.SteamAuth.TrustedCandidates, ","),
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(instance.DataDir, ".local-container", "control", "options.json"), []byte(`{"controlModVersion":"0.2.2"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	junimoManifest := fmt.Sprintf(`{"Name":"JunimoServer","Version":%q,"UniqueID":"JunimoHost.Server"}`, manifest.Server.Tag)
	if err := os.WriteFile(filepath.Join(junimoServerModDir(instance.DataDir), junimoServerManifestName), []byte(junimoManifest), 0o644); err != nil {
		t.Fatal(err)
	}
	inspection := InspectManagedRuntimeStack(instance.DataDir, instance.State)
	if inspection.Status != sjconfig.RuntimeStackStatusUpdateAvailable || inspection.Code != "control_update_available" {
		t.Fatalf("control-only fixture=%+v", inspection)
	}
	return manifest
}

func TestRuntimeUpdateControlOnlyPreservesRunningAuthContainer(t *testing.T) {
	driver, _, instance, fake := setupRuntimeApplyDriver(t, storage.InstanceStateRunning)
	// The incident was triggered by a strict health probe waiting behind a slow
	// Steam reconnect. An unchanged auth container remains an exact, running
	// dependency, but its live health snapshot is advisory for Control-only work.
	fake.authHealth = "unhealthy"
	fake.authUseTargetState = true
	fake.authLoggedIn = false
	fake.authProbeErrorTarget = true
	fake.authProbeErrorCode = "auth_health_timeout"
	manifest := configureControlOnlyRuntimeFixture(t, instance)
	if _, err := driver.StartRuntimeUpdateApply(context.Background(), instance, 0); err != nil {
		t.Fatal(err)
	}
	status := waitRuntimeApply(t, driver, instance)
	if status.Phase != RuntimeUpdateApplySucceeded {
		t.Fatalf("control-only phase=%s code=%s error=%s cause=%s/%s rollback=%s/%s calls=%v", status.Phase, status.ErrorCode, status.Error, status.CauseCode, status.CauseError, status.RollbackCode, status.RollbackError, fake.applyCalls)
	}
	calls := strings.Join(fake.applyCalls, "\n")
	for _, forbidden := range []string{"stop server,steam-auth", "up steam-auth", "up preserve steam-auth", "volume create snapshot", "volume clone snapshot", "volume restore snapshot"} {
		if strings.Contains(calls, forbidden) {
			t.Fatalf("control-only update mutated unchanged auth via %q: %s", forbidden, calls)
		}
	}
	if !strings.Contains(calls, "stop server") || !strings.Contains(calls, "up server") {
		t.Fatalf("control-only update did not restart server: %s", calls)
	}
	if !strings.Contains(calls, "cpu shares steam-auth 256") {
		t.Fatalf("preserved auth did not receive in-place resource weight: %s", calls)
	}
	if !strings.Contains(strings.Join(status.Warnings, "\n"), "不会拖住本次 Control-only 升级") {
		t.Fatalf("advisory auth health warning missing: %#v", status.Warnings)
	}
	var advisoryCheck *RuntimeUpdateDryRunCheck
	for index := range status.Checks {
		if status.Checks[index].Name == "steam_auth_ready" {
			advisoryCheck = &status.Checks[index]
			break
		}
	}
	if advisoryCheck == nil || advisoryCheck.Status != "warning" || !strings.Contains(advisoryCheck.Message, "不阻塞 Control-only 升级") {
		t.Fatalf("control-only auth check was not downgraded to an explicit warning: %#v", status.Checks)
	}
	if got := strings.Count(calls, "auth health target"); got != 1 {
		t.Fatalf("control-only update retried advisory auth health %d times, want one bounded snapshot: %s", got, calls)
	}
	if status.Selected.SteamAuth.ImageID == "" || status.Selected.Server.ImageID == "" {
		t.Fatalf("selected immutable image IDs missing: %+v", status.Selected)
	}
	version, err := readJunimoServerModVersion(junimoServerModDir(instance.DataDir))
	if err != nil || version != manifest.Server.Tag {
		t.Fatalf("control-only update did not preserve the required JunimoServer mod: version=%q err=%v", version, err)
	}
	if fake.fakeDocker.containerRuns != 0 {
		t.Fatalf("control-only update extracted unchanged JunimoServer %d times, want 0", fake.fakeDocker.containerRuns)
	}
}

func TestRuntimeUpdateControlOnlyStoppedAuthReconnectDoesNotBlock(t *testing.T) {
	driver, _, instance, fake := setupRuntimeApplyDriver(t, storage.InstanceStateStopped)
	driver.runtimeUpdateAuthAdvisoryTimeout = time.Millisecond
	fake.authProbeErrorTarget = true
	fake.authProbeErrorCode = "auth_health_timeout"
	manifest := configureControlOnlyRuntimeFixture(t, instance)
	fake.pulled[manifest.Server.TrustedCandidates[0]] = true
	fake.pulled[manifest.SteamAuth.TrustedCandidates[0]] = true

	if _, err := driver.StartRuntimeUpdateApply(context.Background(), instance, 0); err != nil {
		t.Fatal(err)
	}
	status := waitRuntimeApply(t, driver, instance)
	if status.Phase != RuntimeUpdateApplySucceeded || status.ServerRunning {
		t.Fatalf("stopped Control-only update did not finish and restore stopped state: %#v calls=%v", status, fake.applyCalls)
	}
	calls := strings.Join(fake.applyCalls, "\n")
	for _, required := range []string{"up preserve steam-auth", "up server", "stop server,steam-auth", "cpu shares steam-auth 256"} {
		if !strings.Contains(calls, required) {
			t.Fatalf("stopped Control-only update missed %q: %s", required, calls)
		}
	}
	for _, forbidden := range []string{"volume create snapshot", "volume clone snapshot", "volume restore snapshot"} {
		if strings.Contains(calls, forbidden) {
			t.Fatalf("stopped Control-only update mutated unchanged auth via %q: %s", forbidden, calls)
		}
	}
	if got := strings.Count(calls, "auth health target"); got != 1 {
		t.Fatalf("stopped Control-only update retried advisory auth health %d times, want one bounded snapshot: %s", got, calls)
	}
	if !strings.Contains(strings.Join(status.Warnings, "\n"), "不会拖住本次 Control-only 升级") {
		t.Fatalf("stopped auth reconnect warning missing: %#v", status.Warnings)
	}
}

func TestRuntimeUpdateControlOnlyKeepsAuthIdentityHardGates(t *testing.T) {
	tests := []struct {
		name      string
		code      string
		configure func(*runtimeApplyFakeDocker)
	}{
		{name: "container not running", code: "auth_container_not_running", configure: func(fake *runtimeApplyFakeDocker) {
			fake.authContainerStateAfterUp = "exited"
		}},
		{name: "digest mismatch", code: "auth_digest_mismatch", configure: func(fake *runtimeApplyFakeDocker) {
			fake.digestMismatchAfterUpService = "steam-auth"
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			driver, _, instance, fake := setupRuntimeApplyDriver(t, storage.InstanceStateRunning)
			test.configure(fake)
			configureControlOnlyRuntimeFixture(t, instance)

			if _, err := driver.StartRuntimeUpdateApply(context.Background(), instance, 0); err != nil {
				t.Fatal(err)
			}
			status := waitRuntimeApply(t, driver, instance)
			if status.Phase != RuntimeUpdateApplyFailedRolledBack || status.ErrorCode != test.code || status.CauseCode != test.code {
				t.Fatalf("Control-only auth identity failure was not a hard gate: %#v calls=%v", status, fake.applyCalls)
			}
		})
	}
}

func TestRuntimeUpdateAuthHealthIsAdvisoryOnlyForControlOnly(t *testing.T) {
	tests := []struct {
		name     string
		manifest runtimeUpdateRecoveryManifest
		strict   bool
	}{
		{name: "Control only", manifest: runtimeUpdateRecoveryManifest{SchemaVersion: 3}, strict: false},
		{name: "server image changed", manifest: runtimeUpdateRecoveryManifest{SchemaVersion: 3, ServerImageChanged: true}, strict: true},
		{name: "auth image changed", manifest: runtimeUpdateRecoveryManifest{SchemaVersion: 3, AuthImageChanged: true}, strict: true},
		{name: "Junimo host mod changed", manifest: runtimeUpdateRecoveryManifest{SchemaVersion: 3, JunimoModReplaceIntent: true}, strict: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := runtimeUpdateRequiresStrictAuthHealth(test.manifest); got != test.strict {
				t.Fatalf("strict auth health=%v, want %v for %#v", got, test.strict, test.manifest)
			}
		})
	}
}

func TestRuntimeUpdateRepairMaterializesMissingJunimoFromLegacyControlOnlyTransaction(t *testing.T) {
	driver, _, instance, fake := setupRuntimeApplyDriver(t, storage.InstanceStateStopped)
	stackManifest, err := sjconfig.BuiltInRuntimeStackManifest()
	if err != nil {
		t.Fatal(err)
	}
	if err := sjconfig.UpdateEnvFile(filepath.Join(instance.DataDir, ".env"), map[string]string{
		"IMAGE_VERSION":                  stackManifest.Server.Tag,
		"SERVER_IMAGE":                   stackManifest.Server.TrustedCandidates[0],
		"SERVER_IMAGE_CANDIDATES":        strings.Join(stackManifest.Server.TrustedCandidates, ","),
		"STEAM_SERVICE_IMAGE":            stackManifest.SteamAuth.TrustedCandidates[0],
		"STEAM_SERVICE_IMAGE_CANDIDATES": strings.Join(stackManifest.SteamAuth.TrustedCandidates, ","),
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(instance.DataDir, ".local-container", "control", "options.json"), []byte(`{"controlModVersion":"0.2.2"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(junimoServerModDir(instance.DataDir)); err != nil {
		t.Fatal(err)
	}
	inspection := InspectManagedRuntimeStack(instance.DataDir, instance.State)
	if inspection.Status != sjconfig.RuntimeStackStatusUpdateAvailable || inspection.Code != "control_update_available" {
		t.Fatalf("legacy control-only fixture=%+v", inspection)
	}
	applyID := "apply_" + strings.Repeat("6", 24)
	project := strings.ToLower(filepath.Base(instance.DataDir))
	selected := RuntimeUpdateSelectedPair{
		Server: RuntimeUpdateSelectedImage{
			Image: stackManifest.Server.TrustedCandidates[0], Digest: "sha256:" + strings.Repeat("a", 64), ImageID: "sha256:" + strings.Repeat("a", 64),
		},
		SteamAuth: RuntimeUpdateSelectedImage{
			Image: stackManifest.SteamAuth.TrustedCandidates[0], Digest: "sha256:" + strings.Repeat("a", 64), ImageID: "sha256:" + strings.Repeat("a", 64),
		},
	}
	recovery := runtimeUpdateRecoveryManifest{
		SchemaVersion: 3, ApplyID: applyID, Project: project, SteamSessionVolume: "stardew_steam-session",
		SnapshotVolume: project + "_anxi-junimo-update-" + strings.Repeat("6", 24) + "-steam-session",
		OriginalState:  storage.InstanceStateStopped,
		OriginalServer: selected.Server, OriginalAuth: selected.SteamAuth, Target: selected,
		OriginalServerVersion: stackManifest.Server.Tag, TargetServerVersion: stackManifest.Server.Tag,
		MutationStarted: true, StopIntent: true, ControlUpdateIntent: true, ControlUpdated: true,
	}
	if err := createRuntimeRecoveryFiles(instance.DataDir, recovery); err != nil {
		t.Fatal(err)
	}
	recovery.ControlManifestPresent, recovery.ControlDLLPresent, err = backupRuntimeControlMod(instance.DataDir, applyID)
	if err != nil {
		t.Fatal(err)
	}
	recovery.OriginalEnvSHA256, _ = runtimeRecoveryFileSHA256(filepath.Join(runtimeUpdateRecoveryDir(instance.DataDir, applyID), "original.env"))
	recovery.OriginalComposeSHA256, _ = runtimeRecoveryFileSHA256(filepath.Join(runtimeUpdateRecoveryDir(instance.DataDir, applyID), "original-compose.yml"))
	recovery.OriginalControlJSONSHA, _ = runtimeRecoveryFileSHA256(filepath.Join(runtimeUpdateRecoveryDir(instance.DataDir, applyID), "original-control-manifest.json"))
	recovery.OriginalControlDLLSHA, _ = runtimeRecoveryFileSHA256(filepath.Join(runtimeUpdateRecoveryDir(instance.DataDir, applyID), "original-control-StardewAnxiPanel.Control.dll"))
	if err := writeRuntimeUpdateRecoveryManifest(instance.DataDir, recovery); err != nil {
		t.Fatal(err)
	}
	failed := RuntimeUpdateApplyStatus{
		ApplyID: applyID, Phase: RuntimeUpdateApplyRollbackFailed, Progress: 100,
		Current: inspection.Current, Target: inspection.Recommended, Selected: selected,
		Checks: []RuntimeUpdateDryRunCheck{}, Warnings: []string{}, Logs: []RuntimeUpdateDryRunLog{},
		CauseCode: "server_container_not_ready", CauseError: "新版 Junimo server 运行验证失败。",
		RollbackCode: "rollback_verify_server_failed", RollbackError: "升级前的 Junimo server 未能恢复就绪。",
	}
	if err := writeRuntimeUpdateApplyStatus(instance.DataDir, failed); err != nil {
		t.Fatal(err)
	}
	fake.fakeDocker.junimoExtractVersion = stackManifest.Server.Tag
	plan := DetectRuntimeUpdateRepairPlan(instance)
	if plan == nil || !plan.ActionAvailable || plan.Code != "repair/rollback_failed" {
		t.Fatalf("legacy rollback repair plan=%#v", plan)
	}
	started, err := driver.StartRuntimeUpdateRepair(context.Background(), instance, 0)
	if err != nil {
		t.Fatal(err)
	}
	final := waitRuntimeApply(t, driver, instance)
	if final.Phase != RuntimeUpdateApplySucceeded || final.RepairAttempts != 1 || final.RepairSourceApplyID != applyID {
		t.Fatalf("legacy control-only rollback was not repaired: started=%#v final=%#v", started, final)
	}
	version, err := readJunimoServerModVersion(junimoServerModDir(instance.DataDir))
	if err != nil || version != stackManifest.Server.Tag {
		t.Fatalf("legacy repair did not materialize JunimoServer: version=%q err=%v", version, err)
	}
	if fake.fakeDocker.containerRuns != 1 {
		t.Fatalf("legacy repair extracted JunimoServer %d times, want 1", fake.fakeDocker.containerRuns)
	}
}

func TestRuntimeUpdateApplyImageCleanupFailureIsWarning(t *testing.T) {
	driver, _, instance, fake := setupRuntimeApplyDriver(t, storage.InstanceStateRunning)
	fake.removeImageError = true
	fake.removeImageStarted = make(chan struct{})
	releaseCleanup := make(chan struct{})
	fake.removeImageRelease = releaseCleanup
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(releaseCleanup) }) })
	if _, err := driver.StartRuntimeUpdateApply(context.Background(), instance, 0); err != nil {
		t.Fatal(err)
	}
	select {
	case <-fake.removeImageStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("runtime apply did not reach old-image cleanup")
	}
	inflight, err := driver.RuntimeUpdateApplyStatus(instance)
	if err != nil {
		t.Fatal(err)
	}
	if runtimeUpdateApplyTerminal(inflight.Phase) {
		t.Fatalf("runtime apply published terminal status before cleanup warnings were complete: %#v", inflight)
	}
	releaseOnce.Do(func() { close(releaseCleanup) })
	status := waitRuntimeApply(t, driver, instance)
	if status.Phase != RuntimeUpdateApplySucceeded || !strings.Contains(strings.Join(status.Warnings, "\n"), "旧镜像") {
		t.Fatalf("cleanup failure changed success semantics or omitted warning: %#v", status)
	}
}

func TestRuntimeUpdateApplyPinsRunningContainerImageIDsWithoutPersistingDigestConfig(t *testing.T) {
	driver, _, instance, fake := setupRuntimeApplyDriver(t, storage.InstanceStateRunning)
	fake.metadata["sdvd/server:1.4.0-preview.1"] = paneldocker.RuntimeImageMetadata{ID: "sha256:" + strings.Repeat("d", 64), Digest: "sha256:" + strings.Repeat("d", 64)}
	fake.metadata["anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2"] = paneldocker.RuntimeImageMetadata{ID: "sha256:" + strings.Repeat("e", 64), Digest: "sha256:" + strings.Repeat("e", 64)}
	fake.authProbeErrorTarget = true
	if _, err := driver.StartRuntimeUpdateApply(context.Background(), instance, 0); err != nil {
		t.Fatal(err)
	}
	status := waitRuntimeApply(t, driver, instance)
	if status.Phase != RuntimeUpdateApplyFailedRolledBack {
		t.Fatalf("status=%#v", status)
	}
	env, err := sjconfig.ReadEnvFile(filepath.Join(instance.DataDir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if env["SERVER_IMAGE"] != "sdvd/server:1.4.0-preview.1" || env["STEAM_SERVICE_IMAGE"] != "anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2" {
		t.Fatalf("rollback leaked temporary digest pins into persistent config: %#v", env)
	}
	inspection := sjconfig.InspectRuntimeStack(instance.DataDir, true)
	if inspection.Status != sjconfig.RuntimeStackStatusUpdateAvailable || !inspection.Available {
		t.Fatalf("restored tagged config no longer reports the recommended update: %#v", inspection)
	}
}

func TestRuntimeUpdateApplyStopsAuthBeforeSnapshotWhenOnlyAuthIsRunning(t *testing.T) {
	driver, _, instance, fake := setupRuntimeApplyDriver(t, storage.InstanceStateStopped)
	fake.fakeDocker.psResult = paneldocker.ComposePsResult{Services: []paneldocker.ComposeService{{Service: "server", State: "exited"}, {Service: "steam-auth", State: "running"}}}
	if _, err := driver.StartRuntimeUpdateApply(context.Background(), instance, 0); err != nil {
		t.Fatal(err)
	}
	status := waitRuntimeApply(t, driver, instance)
	if status.Phase != RuntimeUpdateApplySucceeded || status.ServerRunning {
		t.Fatalf("status=%#v", status)
	}
	calls := strings.Join(fake.applyCalls, "\n")
	stopAt, cloneAt := strings.Index(calls, "stop steam-auth"), strings.Index(calls, "volume clone snapshot")
	if stopAt < 0 || cloneAt < 0 || stopAt > cloneAt {
		t.Fatalf("auth was not quiesced before snapshot:\n%s", calls)
	}
}

func TestRuntimeUpdateApplyRestoresStoppedStateAndDoesNotLeakSecrets(t *testing.T) {
	driver, _, instance, fake := setupRuntimeApplyDriver(t, storage.InstanceStateStopped)
	envPath := filepath.Join(instance.DataDir, ".env")
	env, _ := os.ReadFile(envPath)
	if err := os.WriteFile(envPath, append(env, []byte("STEAM_PASSWORD=super-secret-password\nSTEAM_REFRESH_TOKEN=super-secret-token\n")...), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := driver.StartRuntimeUpdateApply(context.Background(), instance, 0); err != nil {
		t.Fatal(err)
	}
	status := waitRuntimeApply(t, driver, instance)
	if status.Phase != RuntimeUpdateApplySucceeded || status.ServerRunning {
		t.Fatalf("stopped state not restored: %#v", status)
	}
	serialized, _ := json.Marshal(status)
	if strings.Contains(string(serialized), "super-secret") || strings.Contains(string(serialized), "STEAM_REFRESH_TOKEN") {
		t.Fatalf("apply status leaked secrets: %s", serialized)
	}
	if !strings.Contains(strings.Join(fake.applyCalls, "\n"), "stop server,steam-auth") {
		t.Fatal("temporary verification runtime was not stopped")
	}
}

func TestRuntimeUpdateApplyAcceptsLoggedOutAuthAndDoesNotRequireInviteCode(t *testing.T) {
	driver, _, instance, fake := setupRuntimeApplyDriver(t, storage.InstanceStateRunning)
	fake.authUseTargetState = true
	fake.authLoggedIn = false
	if _, err := driver.StartRuntimeUpdateApply(context.Background(), instance, 0); err != nil {
		t.Fatal(err)
	}
	status := waitRuntimeApply(t, driver, instance)
	if status.Phase != RuntimeUpdateApplySucceeded {
		t.Fatalf("logged-out LAN-only runtime was rejected: %#v", status)
	}
	if !strings.Contains(strings.Join(status.Warnings, "\n"), "不影响局域网模式") {
		t.Fatalf("logged-out capability warning missing: %#v", status.Warnings)
	}
	if strings.Contains(strings.Join(fake.applyCalls, "\n"), "/tmp/invite-code.txt") {
		t.Fatalf("runtime acceptance still probed an invite code: %s", strings.Join(fake.applyCalls, "\n"))
	}
	if got := strings.Count(strings.Join(fake.applyCalls, "\n"), "auth health target"); got < 2 {
		t.Fatalf("initial and final target verification did not share /health: calls=%v", fake.applyCalls)
	}
}

func TestRuntimeUpdateAuthFailureCodesAndLastReasonSurviveRollback(t *testing.T) {
	for _, test := range []struct {
		name      string
		code      string
		configure func(*runtimeApplyFakeDocker)
	}{
		{name: "container not running", code: "auth_container_not_running", configure: func(f *runtimeApplyFakeDocker) { f.authContainerState = "exited" }},
		{name: "digest mismatch", code: "auth_digest_mismatch", configure: func(f *runtimeApplyFakeDocker) { f.digestMismatchService = "steam-auth" }},
		{name: "health unreachable", code: "auth_health_unreachable", configure: func(f *runtimeApplyFakeDocker) {
			f.authProbeErrorTarget = true
			f.authProbeErrorCode = "auth_health_unreachable"
		}},
		{name: "health timeout", code: "auth_health_timeout", configure: func(f *runtimeApplyFakeDocker) {
			f.authProbeErrorTarget = true
			f.authProbeErrorCode = "auth_health_timeout"
		}},
		{name: "health http status", code: "auth_health_http_status", configure: func(f *runtimeApplyFakeDocker) {
			f.authProbeErrorTarget = true
			f.authProbeErrorCode = "auth_health_http_status"
		}},
		{name: "health invalid response", code: "auth_health_invalid_response", configure: func(f *runtimeApplyFakeDocker) {
			f.authProbeErrorTarget = true
			f.authProbeErrorCode = "auth_health_invalid_response"
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			driver, _, instance, fake := setupRuntimeApplyDriver(t, storage.InstanceStateRunning)
			test.configure(fake)
			if _, err := driver.StartRuntimeUpdateApply(context.Background(), instance, 0); err != nil {
				t.Fatal(err)
			}
			status := waitRuntimeApply(t, driver, instance)
			if status.Phase != RuntimeUpdateApplyFailedRolledBack || status.ErrorCode != test.code || status.CauseCode != test.code {
				t.Fatalf("status=%#v", status)
			}
			if strings.TrimSpace(status.Error) == "" || status.Error == "steam-auth-cn 认证服务接口验证失败。" {
				t.Fatalf("last sanitized health failure reason was lost: %#v", status)
			}
			if !strings.Contains(strings.Join(fake.applyCalls, "\n"), "auth health original") {
				t.Fatalf("rollback did not verify the original auth image through the same /health contract: %v", fake.applyCalls)
			}
		})
	}
}

func TestRuntimeUpdateRollbackAuthHealthFailureRetainsSanitizedProbeReason(t *testing.T) {
	err := fmt.Errorf("verify old auth: %w", &RuntimeUpdateValidationError{Code: "auth_health_timeout", Message: "steam-auth-cn /health 探针超时，未在单次探针预算内返回。"})
	code, message := runtimeUpdateRollbackFailure(err)
	if code != "rollback_verify_auth_failed" || !strings.Contains(message, "/health 探针超时") {
		t.Fatalf("code=%q message=%q", code, message)
	}
}

func TestRequiredRuntimeUpdateAutomaticallyChainsDryRunAndApply(t *testing.T) {
	driver, _, instance, fake := setupRuntimeApplyDriver(t, storage.InstanceStateRunning)
	driver.panelVersion = "0.3.5"
	if err := os.Remove(runtimeUpdateDryRunStatusPath(instance.DataDir)); err != nil {
		t.Fatal(err)
	}
	fake.authUseTargetState = true
	fake.authLoggedIn = false
	fake.inviteUnavailable = true
	manifest, err := sjconfig.BuiltInRuntimeStackManifest()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := driver.runRequiredRuntimeUpdate(ctx, instance, manifest); err != nil {
		t.Fatalf("%v calls=%v metadata=%v", err, fake.calls, fake.metadata)
	}
	status, err := readRequiredRuntimeUpdateStatus(instance.DataDir)
	if err != nil || status.Phase != requiredRuntimePhaseSucceeded || status.StackVersion != manifest.StackVersion {
		t.Fatalf("required coordinator status=%#v err=%v", status, err)
	}
	inspection := InspectRuntimeStack(instance.DataDir, storage.InstanceStateRunning)
	if inspection.Status != sjconfig.RuntimeStackStatusUpToDate || inspection.Current.Server.Tag != "1.5.0-preview.125" {
		t.Fatalf("required stack was not applied: %#v", inspection)
	}
	calls := strings.Join(fake.applyCalls, "\n")
	if !strings.Contains(calls, "up steam-auth") || !strings.Contains(calls, "up server") {
		t.Fatalf("required coordinator did not recreate the pair: %s", calls)
	}
}

func TestRequiredRuntimeUpdateDisabledChainsServerOnlyWithoutAuth(t *testing.T) {
	driver, _, instance, fake := setupRuntimeApplyDriver(t, storage.InstanceStateRunning)
	configureDisabledRuntimeApplyFixture(t, instance, fake)
	driver.panelVersion = "0.6.0"
	if err := os.Remove(runtimeUpdateDryRunStatusPath(instance.DataDir)); err != nil {
		t.Fatal(err)
	}
	manifest, err := sjconfig.BuiltInRuntimeStackManifest()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := driver.runRequiredRuntimeUpdate(ctx, instance, manifest); err != nil {
		t.Fatalf("%v calls=%v applyCalls=%v", err, fake.calls, fake.applyCalls)
	}
	required, err := readRequiredRuntimeUpdateStatus(instance.DataDir)
	if err != nil || required.Phase != requiredRuntimePhaseSucceeded || required.StackVersion != manifest.StackVersion {
		t.Fatalf("required disabled status=%#v err=%v", required, err)
	}
	apply, err := driver.RuntimeUpdateApplyStatus(instance)
	if err != nil || apply.Phase != RuntimeUpdateApplySucceeded || apply.Selected.SteamAuth != (RuntimeUpdateSelectedImage{}) {
		t.Fatalf("disabled apply status=%#v err=%v", apply, err)
	}
	if inspection := InspectRuntimeStack(instance.DataDir, storage.InstanceStateRunning); inspection.Status != sjconfig.RuntimeStackStatusUpToDate {
		t.Fatalf("disabled server stack was not applied: %#v", inspection)
	}
	assertNoOptionalAuthRuntimeCalls(t, fake)
}

func TestRequiredRuntimeUpdateFailureIsPersistedAndNotRetriedOnSamePanel(t *testing.T) {
	driver, _, instance, fake := setupRuntimeApplyDriver(t, storage.InstanceStateStopped)
	driver.panelVersion = "0.3.5"
	if err := os.Remove(runtimeUpdateDryRunStatusPath(instance.DataDir)); err != nil {
		t.Fatal(err)
	}
	fake.authProbeErrorTarget = true
	manifest, _ := sjconfig.BuiltInRuntimeStackManifest()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := driver.runRequiredRuntimeUpdate(ctx, instance, manifest); err == nil {
		t.Fatal("required update failure was ignored")
	}
	status, err := readRequiredRuntimeUpdateStatus(instance.DataDir)
	if err != nil || status.Phase != requiredRuntimePhaseFailed {
		t.Fatalf("required failure status=%#v err=%v", status, err)
	}
	if observed, readErr := driver.ReadRequiredRuntimeUpdateStatus(instance); readErr != nil || observed.Phase != requiredRuntimePhaseFailed {
		t.Fatalf("current required failure was incorrectly resolved: %#v err=%v", observed, readErr)
	}
	before := len(fake.applyCalls)
	driver.StartRequiredRuntimeUpdate(context.Background(), instance)
	time.Sleep(25 * time.Millisecond)
	if len(fake.applyCalls) != before {
		t.Fatalf("identical failed Panel/stack auto-retried: before=%d after=%d", before, len(fake.applyCalls))
	}
}

func TestSuccessfulRuntimeApplyResolvesStaleRequiredFailure(t *testing.T) {
	driver, _, instance, _ := setupRuntimeApplyDriver(t, storage.InstanceStateStopped)
	driver.panelVersion = "0.4.15"
	manifest, err := sjconfig.BuiltInRuntimeStackManifest()
	if err != nil {
		t.Fatal(err)
	}
	stale := RequiredRuntimeUpdateStatus{
		SchemaVersion: 1,
		PanelVersion:  driver.panelVersion,
		StackVersion:  manifest.StackVersion,
		Phase:         requiredRuntimePhaseFailed,
		ErrorCode:     "runtime_update_save_failed",
		Error:         "historical save confirmation failure",
		FinishedAt:    "2026-08-14T01:15:41Z",
	}
	if err := writeRequiredRuntimeUpdateStatus(instance.DataDir, stale); err != nil {
		t.Fatal(err)
	}
	if _, err := driver.StartRuntimeUpdateApply(context.Background(), instance, 0); err != nil {
		t.Fatal(err)
	}
	if apply := waitRuntimeApply(t, driver, instance); apply.Phase != RuntimeUpdateApplySucceeded {
		t.Fatalf("runtime apply did not succeed: %#v", apply)
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		resolved, readErr := readRequiredRuntimeUpdateStatus(instance.DataDir)
		if readErr == nil && resolved.Phase == requiredRuntimePhaseSucceeded {
			if resolved.ErrorCode != "" || resolved.Error != "" {
				t.Fatalf("resolved required status retained stale error: %#v", resolved)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	resolved, readErr := readRequiredRuntimeUpdateStatus(instance.DataDir)
	t.Fatalf("successful runtime apply did not resolve required failure: %#v err=%v", resolved, readErr)
}

func TestReadRequiredRuntimeStatusResolvesHistoricalFailure(t *testing.T) {
	driver, _, instance, _ := setupRuntimeApplyDriver(t, storage.InstanceStateStopped)
	driver.panelVersion = "0.4.15"
	if _, err := driver.StartRuntimeUpdateApply(context.Background(), instance, 0); err != nil {
		t.Fatal(err)
	}
	if apply := waitRuntimeApply(t, driver, instance); apply.Phase != RuntimeUpdateApplySucceeded {
		t.Fatalf("runtime apply did not succeed: %#v", apply)
	}
	manifest, err := sjconfig.BuiltInRuntimeStackManifest()
	if err != nil {
		t.Fatal(err)
	}
	if err := writeRequiredRuntimeUpdateStatus(instance.DataDir, RequiredRuntimeUpdateStatus{
		SchemaVersion: 1,
		PanelVersion:  driver.panelVersion,
		StackVersion:  manifest.StackVersion,
		Phase:         requiredRuntimePhaseFailed,
		ErrorCode:     "runtime_update_save_failed",
		Error:         "historical save confirmation failure",
		FinishedAt:    "2026-08-14T01:15:41Z",
	}); err != nil {
		t.Fatal(err)
	}

	resolved, err := driver.ReadRequiredRuntimeUpdateStatus(instance)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Phase != requiredRuntimePhaseSucceeded || resolved.ErrorCode != "" || resolved.Error != "" {
		t.Fatalf("historical required failure was not resolved on read: %#v", resolved)
	}
	persisted, err := readRequiredRuntimeUpdateStatus(instance.DataDir)
	if err != nil || persisted.Phase != requiredRuntimePhaseSucceeded {
		t.Fatalf("resolved required status was not persisted: %#v err=%v", persisted, err)
	}
}

func TestRequiredRuntimeUpdateResumesAfterCurrentAuthImageBecomesAvailable(t *testing.T) {
	driver, _, instance, _ := setupRuntimeApplyDriver(t, storage.InstanceStateStopped)
	driver.panelVersion = "0.3.5"
	manifest, err := sjconfig.BuiltInRuntimeStackManifest()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(runtimeUpdateDryRunStatusPath(instance.DataDir)); err != nil {
		t.Fatal(err)
	}
	if err := writeRequiredRuntimeUpdateStatus(instance.DataDir, RequiredRuntimeUpdateStatus{
		SchemaVersion: 1, PanelVersion: driver.panelVersion, StackVersion: manifest.StackVersion,
		Phase: requiredRuntimePhaseFailed, ErrorCode: "current_auth_digest_unavailable",
	}); err != nil {
		t.Fatal(err)
	}

	driver.StartRequiredRuntimeUpdate(context.Background(), instance)
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		status, readErr := readRequiredRuntimeUpdateStatus(instance.DataDir)
		if readErr == nil && status.Phase == requiredRuntimePhaseSucceeded {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	status, _ := readRequiredRuntimeUpdateStatus(instance.DataDir)
	t.Fatalf("current-auth digest failure did not resume after image recovery: %#v", status)
}

func TestRequiredRuntimeUpdateResumesAfterPanelContextCancellation(t *testing.T) {
	driver, _, instance, _ := setupRuntimeApplyDriver(t, storage.InstanceStateStopped)
	driver.panelVersion = "0.3.5"
	manifest, err := sjconfig.BuiltInRuntimeStackManifest()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(runtimeUpdateDryRunStatusPath(instance.DataDir)); err != nil {
		t.Fatal(err)
	}
	if err := writeRequiredRuntimeUpdateStatus(instance.DataDir, RequiredRuntimeUpdateStatus{
		SchemaVersion: 1, PanelVersion: driver.panelVersion, StackVersion: manifest.StackVersion,
		Phase: requiredRuntimePhaseFailed, ErrorCode: "context_cancelled",
	}); err != nil {
		t.Fatal(err)
	}
	driver.StartRequiredRuntimeUpdate(context.Background(), instance)
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		status, readErr := readRequiredRuntimeUpdateStatus(instance.DataDir)
		if readErr == nil && status.Phase == requiredRuntimePhaseSucceeded {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	status, _ := readRequiredRuntimeUpdateStatus(instance.DataDir)
	t.Fatalf("context-cancelled full-stack update did not resume: %#v", status)
}

func TestRequiredRuntimeUpdateRepairsTrustedLegacyCandidates(t *testing.T) {
	driver, _, instance, fake := setupRuntimeApplyDriver(t, storage.InstanceStateRunning)
	driver.panelVersion = "0.3.5"
	fake.metadata["dockerproxy.net/sdvd/server:1.5.0-preview.121"] = paneldocker.RuntimeImageMetadata{ID: "sha256:" + strings.Repeat("b", 64), Digest: "sha256:" + strings.Repeat("b", 64)}
	fake.pulled["dockerproxy.net/sdvd/server:1.5.0-preview.121"] = true
	fake.pulled["anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2"] = true
	legacy := strings.Join([]string{
		"IMAGE_VERSION=1.5.0-preview.121",
		"SERVER_IMAGE=dockerproxy.net/sdvd/server:1.5.0-preview.121",
		"SERVER_IMAGE_CANDIDATES=dockerproxy.net/sdvd/server:1.5.0-preview.125,sdvd/server:1.5.0-preview.121,docker.m.daocloud.io/sdvd/server:1.5.0-preview.121,ghcr.io/sdvd/server:1.5.0-preview.121",
		"STEAM_SERVICE_IMAGE=anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2",
		"STEAM_SERVICE_IMAGE_CANDIDATES=anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2",
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(instance.DataDir, ".env"), []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(runtimeUpdateDryRunStatusPath(instance.DataDir)); err != nil {
		t.Fatal(err)
	}
	inspection := InspectRuntimeStack(instance.DataDir, instance.State)
	if !inspection.Repairable {
		t.Fatalf("fixture is not repairable: %#v", inspection)
	}
	manifest, _ := sjconfig.BuiltInRuntimeStackManifest()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := driver.runRequiredRuntimeUpdate(ctx, instance, manifest); err != nil {
		t.Fatalf("%v calls=%v metadata=%v", err, fake.calls, fake.metadata)
	}
	if got := InspectRuntimeStack(instance.DataDir, instance.State); got.Status != sjconfig.RuntimeStackStatusUpToDate {
		t.Fatalf("trusted legacy config was not repaired and upgraded: %#v", got)
	}
	backups, err := filepath.Glob(filepath.Join(instance.DataDir, ".local-container", "junimo-update", "config-repair", "*", "original.env"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("private repair backup missing: %v %v", backups, err)
	}
}

func TestRequiredRuntimeUpdateRollbackFailureRequiresManualAction(t *testing.T) {
	driver, _, instance, fake := setupRuntimeApplyDriver(t, storage.InstanceStateStopped)
	driver.panelVersion = "0.3.5"
	if err := os.Remove(runtimeUpdateDryRunStatusPath(instance.DataDir)); err != nil {
		t.Fatal(err)
	}
	fake.authProbeErrorTarget = true
	fake.restoreError = true
	manifest, _ := sjconfig.BuiltInRuntimeStackManifest()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := driver.runRequiredRuntimeUpdate(ctx, instance, manifest); err == nil {
		t.Fatal("rollback failure was ignored")
	}
	status, err := readRequiredRuntimeUpdateStatus(instance.DataDir)
	if err != nil || status.Phase != requiredRuntimePhaseManual {
		t.Fatalf("rollback failure did not require manual action: %#v err=%v", status, err)
	}
}

func TestRequiredRuntimeUpdateRejectsCustomImagesWithoutMutation(t *testing.T) {
	driver, _, instance, fake := setupRuntimeApplyDriver(t, storage.InstanceStateStopped)
	driver.panelVersion = "0.3.5"
	envPath := filepath.Join(instance.DataDir, ".env")
	env, _ := os.ReadFile(envPath)
	custom := strings.ReplaceAll(string(env), "sdvd/server:1.4.0-preview.1", "registry.example/custom/server:1.4.0-preview.1")
	if err := os.WriteFile(envPath, []byte(custom), 0o600); err != nil {
		t.Fatal(err)
	}
	manifest, _ := sjconfig.BuiltInRuntimeStackManifest()
	if err := driver.runRequiredRuntimeUpdate(context.Background(), instance, manifest); err == nil {
		t.Fatal("custom image was force-overwritten")
	}
	status, err := readRequiredRuntimeUpdateStatus(instance.DataDir)
	if err != nil || status.Phase != requiredRuntimePhaseManual {
		t.Fatalf("custom image did not enter manual state: %#v err=%v", status, err)
	}
	if strings.Contains(strings.Join(fake.applyCalls, "\n"), "compose down") {
		t.Fatalf("custom runtime was mutated: %v", fake.applyCalls)
	}
}

func TestRequiredRuntimeStackBlocksStartingOldRuntime(t *testing.T) {
	driver, _, instance, _ := setupRuntimeApplyDriver(t, storage.InstanceStateStopped)
	err := driver.requireCurrentRuntimeStack(instance)
	validation, ok := IsRuntimeUpdateValidationError(err)
	if !ok || validation.Code != "required_runtime_update" {
		t.Fatalf("old required runtime was allowed: %v", err)
	}
}

func TestRuntimeUpdateApplyFailuresRollbackPairAndState(t *testing.T) {
	for _, test := range []struct {
		name      string
		configure func(*runtimeApplyFakeDocker)
		want      string
	}{
		{"auth endpoint unavailable", func(f *runtimeApplyFakeDocker) { f.authProbeErrorTarget = true }, RuntimeUpdateApplyFailedRolledBack},
		{"server health", func(f *runtimeApplyFakeDocker) { f.serverHealthFailTarget = true }, RuntimeUpdateApplyFailedRolledBack},
		{"server control contract", func(f *runtimeApplyFakeDocker) { f.controlContractFail = true }, RuntimeUpdateApplyFailedRolledBack},
		{"loaded Junimo version mismatch", func(f *runtimeApplyFakeDocker) { f.loadedVersionMismatch = true }, RuntimeUpdateApplyFailedRolledBack},
		{"target Junimo package version mismatch", func(f *runtimeApplyFakeDocker) { f.fakeDocker.junimoExtractVersion = "1.5.0-preview.121" }, RuntimeUpdateApplyFailedRolledBack},
		{"auth digest mismatch", func(f *runtimeApplyFakeDocker) { f.digestMismatchService = "steam-auth" }, RuntimeUpdateApplyFailedRolledBack},
		{"server digest mismatch", func(f *runtimeApplyFakeDocker) { f.digestMismatchService = "server" }, RuntimeUpdateApplyFailedRolledBack},
		{"rollback failed", func(f *runtimeApplyFakeDocker) { f.authProbeErrorTarget = true; f.restoreError = true }, RuntimeUpdateApplyRollbackFailed},
	} {
		t.Run(test.name, func(t *testing.T) {
			driver, _, instance, fake := setupRuntimeApplyDriver(t, storage.InstanceStateStopped)
			test.configure(fake)
			if _, err := driver.StartRuntimeUpdateApply(context.Background(), instance, 0); err != nil {
				t.Fatal(err)
			}
			status := waitRuntimeApply(t, driver, instance)
			if status.Phase != test.want {
				t.Fatalf("phase=%s error=%s", status.Phase, status.Error)
			}
			env, err := sjconfig.ReadEnvFile(filepath.Join(instance.DataDir, ".env"))
			if err != nil {
				t.Fatal(err)
			}
			if strings.HasPrefix(env["SERVER_IMAGE"], "sha256:") || strings.HasPrefix(env["STEAM_SERVICE_IMAGE"], "sha256:") {
				t.Fatalf("rollback terminal state leaked temporary digest pins: %#v", env)
			}
			version, versionErr := readJunimoServerModVersion(junimoServerModDir(instance.DataDir))
			if versionErr != nil || version != "1.4.0-preview.1" {
				t.Fatalf("rollback did not restore original host JunimoServer mod: version=%q err=%v", version, versionErr)
			}
			calls := strings.Join(fake.applyCalls, "\n")
			requiresVolumeRestore := test.name != "target Junimo package version mismatch"
			if test.want == RuntimeUpdateApplyFailedRolledBack && ((!strings.Contains(calls, "volume restore snapshot") && requiresVolumeRestore) || !strings.Contains(calls, "up steam-auth") || !strings.Contains(calls, "up server")) {
				t.Fatalf("pair/auth not rolled back: %s", calls)
			}
			if test.want == RuntimeUpdateApplyRollbackFailed {
				if status.ManualAction == "" {
					t.Fatal("missing manual action")
				}
				if status.CauseCode == "" || status.CauseError == "" || status.RollbackCode != "rollback_restore_auth_volume_failed" || status.RollbackError == "" {
					t.Fatalf("rollback failure details missing: %#v", status)
				}
				if _, err := os.Stat(runtimeUpdateRecoveryDir(instance.DataDir, status.ApplyID)); err != nil {
					t.Fatal("recovery materials removed")
				}
				serialized, _ := json.Marshal(status)
				if strings.Contains(string(serialized), "super-secret") || strings.Contains(string(serialized), "refresh_token") {
					t.Fatalf("rollback status leaked secret: %s", serialized)
				}
				if _, err := driver.StartRuntimeUpdateApply(context.Background(), instance, 0); err == nil || !strings.Contains(err.Error(), "禁止自动重试") {
					t.Fatalf("rollback_failed allowed automatic retry: %v", err)
				}
			}
		})
	}
}

func TestRuntimeUpdateRepairRetriesPartialRollbackIdempotently(t *testing.T) {
	driver, _, instance, fake := setupRuntimeApplyDriver(t, storage.InstanceStateStopped)
	fake.authProbeErrorTarget = true
	fake.restoreError = true
	if _, err := driver.StartRuntimeUpdateApply(context.Background(), instance, 0); err != nil {
		t.Fatal(err)
	}
	failed := waitRuntimeApply(t, driver, instance)
	if failed.Phase != RuntimeUpdateApplyRollbackFailed || failed.RollbackCode != "rollback_restore_auth_volume_failed" {
		t.Fatalf("expected injected rollback failure, got %#v", failed)
	}
	if _, err := os.Stat(filepath.Join(runtimeUpdateRecoveryDir(instance.DataDir, failed.ApplyID), "failed-target-junimo-server")); err != nil {
		t.Fatalf("first partial rollback did not leave deterministic Junimo evidence: %v", err)
	}
	plan := DetectRuntimeUpdateRepairPlan(instance)
	if plan == nil || !plan.ActionAvailable || plan.Code != "repair/rollback_failed" || plan.ButtonLabel != "修复：恢复旧版后升级" {
		t.Fatalf("rollback repair plan = %#v", plan)
	}

	fake.restoreError = false
	fake.authProbeErrorTarget = false
	started, err := driver.StartRuntimeUpdateRepair(context.Background(), instance, 0)
	if err != nil {
		t.Fatal(err)
	}
	if started.Phase != RuntimeUpdateApplyRollingBack || started.RepairAttempts != 1 {
		t.Fatalf("unexpected repair start: %#v", started)
	}
	repaired := waitRuntimeApply(t, driver, instance)
	if repaired.Phase != RuntimeUpdateApplySucceeded || repaired.RepairAttempts != 1 || repaired.ManualAction != "" {
		t.Fatalf("partial rollback was not repaired and upgraded: %#v", repaired)
	}
	if repaired.ApplyID == failed.ApplyID || repaired.RepairSourceApplyID != failed.ApplyID {
		t.Fatalf("repair retry transaction was not linked: failed=%s repaired=%#v", failed.ApplyID, repaired)
	}
	checks, _ := json.Marshal(repaired.Checks)
	if !bytes.Contains(checks, []byte(`"repair_materials"`)) || !bytes.Contains(checks, []byte(`"repair_upgrade_preflight"`)) || !bytes.Contains(checks, []byte(`"change_plan"`)) {
		t.Fatalf("repair diagnostics were not retained: %s", checks)
	}
	if _, err := os.Stat(runtimeUpdateRecoveryDir(instance.DataDir, failed.ApplyID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("successful repair did not remove owned recovery directory: %v", err)
	}
	if _, err := driver.StartRuntimeUpdateRepair(context.Background(), instance, 0); err == nil {
		t.Fatal("completed repair was not idempotently rejected")
	} else if validation, ok := IsRuntimeUpdateValidationError(err); !ok || validation.Code != "runtime_repair_not_needed" {
		t.Fatalf("repeat repair error=%v", err)
	}
}

func TestRuntimeUpdateRepairRetryFailureKeepsOriginalRuntimeSafe(t *testing.T) {
	driver, _, instance, fake := setupRuntimeApplyDriver(t, storage.InstanceStateStopped)
	fake.authProbeErrorTarget = true
	fake.restoreError = true
	if _, err := driver.StartRuntimeUpdateApply(context.Background(), instance, 0); err != nil {
		t.Fatal(err)
	}
	failed := waitRuntimeApply(t, driver, instance)
	fake.restoreError = false
	if _, err := driver.StartRuntimeUpdateRepair(context.Background(), instance, 0); err != nil {
		t.Fatal(err)
	}
	final := waitRuntimeApply(t, driver, instance)
	if final.Phase != RuntimeUpdateApplyFailedRolledBack || final.ApplyID == failed.ApplyID || final.RepairSourceApplyID != failed.ApplyID || final.RepairAttempts != 1 {
		t.Fatalf("repair retry failure did not close safely: failed=%#v final=%#v", failed, final)
	}
	if final.ResumeAfterRepair || final.ServerRunning {
		t.Fatalf("safe terminal state retained a resume marker or wrong stopped state: %#v", final)
	}
}

func TestRuntimeUpdateRepairRetriesSafelyRolledBackFailure(t *testing.T) {
	driver, _, instance, fake := setupRuntimeApplyDriver(t, storage.InstanceStateStopped)
	fake.serverHealthFailTarget = true
	if _, err := driver.StartRuntimeUpdateApply(context.Background(), instance, 0); err != nil {
		t.Fatal(err)
	}
	failed := waitRuntimeApply(t, driver, instance)
	if failed.Phase != RuntimeUpdateApplyFailedRolledBack {
		t.Fatalf("expected safe rollback, got %#v", failed)
	}
	plan := DetectRuntimeUpdateRepairPlan(instance)
	if plan == nil || !plan.ActionAvailable || plan.Code != "repair/safe_retry" || plan.ButtonLabel != "修复：重新预检并升级" {
		t.Fatalf("safe retry plan = %#v", plan)
	}
	fake.serverHealthFailTarget = false
	started, err := driver.StartRuntimeUpdateRepair(context.Background(), instance, 0)
	if err != nil {
		t.Fatal(err)
	}
	if started.Phase != RuntimeUpdateApplyResumingUpgrade || started.RepairSourceApplyID != failed.ApplyID || started.RepairAttempts != 1 {
		t.Fatalf("safe retry start = %#v", started)
	}
	final := waitRuntimeApply(t, driver, instance)
	if final.Phase != RuntimeUpdateApplySucceeded || final.ApplyID == failed.ApplyID || final.RepairSourceApplyID != failed.ApplyID || final.RepairAttempts != 1 {
		t.Fatalf("safe retry final = %#v", final)
	}
	checks, _ := json.Marshal(final.Checks)
	if !bytes.Contains(checks, []byte(`"repair_plan"`)) || !bytes.Contains(checks, []byte(`"repair_upgrade_preflight"`)) {
		t.Fatalf("safe retry diagnostic trail missing: %s", checks)
	}
}

func TestRuntimeUpdateRepairPlanCatalogFailsClosed(t *testing.T) {
	t.Run("corrupt status", func(t *testing.T) {
		_, _, instance, _ := setupRuntimeApplyDriver(t, storage.InstanceStateStopped)
		if err := os.MkdirAll(filepath.Dir(runtimeUpdateApplyStatusPath(instance.DataDir)), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(runtimeUpdateApplyStatusPath(instance.DataDir), []byte("{broken"), 0o600); err != nil {
			t.Fatal(err)
		}
		plan := DetectRuntimeUpdateRepairPlan(instance)
		if plan == nil || plan.ActionAvailable || plan.Action != "export" || plan.Code != "recovery_state_uncertain" {
			t.Fatalf("corrupt-state plan = %#v", plan)
		}
	})

	t.Run("custom image", func(t *testing.T) {
		_, _, instance, _ := setupRuntimeApplyDriver(t, storage.InstanceStateStopped)
		values, err := sjconfig.ReadEnvFile(filepath.Join(instance.DataDir, ".env"))
		if err != nil {
			t.Fatal(err)
		}
		if err := sjconfig.UpdateEnvFile(filepath.Join(instance.DataDir, ".env"), map[string]string{
			"SERVER_IMAGE":            "custom.invalid/server:" + values["IMAGE_VERSION"],
			"SERVER_IMAGE_CANDIDATES": "custom.invalid/server:" + values["IMAGE_VERSION"],
		}); err != nil {
			t.Fatal(err)
		}
		plan := DetectRuntimeUpdateRepairPlan(instance)
		if plan == nil || plan.ActionAvailable || plan.Action != "export" || plan.Code != "unsupported/custom_images" || !strings.Contains(plan.Method, "自定义镜像") {
			t.Fatalf("custom-image plan = %#v", plan)
		}
	})

	t.Run("attempts exhausted", func(t *testing.T) {
		_, _, instance, _ := setupRuntimeApplyDriver(t, storage.InstanceStateStopped)
		inspection := InspectManagedRuntimeStack(instance.DataDir, instance.State)
		status := RuntimeUpdateApplyStatus{
			ApplyID: "apply_" + strings.Repeat("d", 24), Phase: RuntimeUpdateApplyFailedRolledBack,
			Current: inspection.Current, Target: inspection.Recommended, RepairAttempts: runtimeUpdateRepairAttemptLimit,
			Checks: []RuntimeUpdateDryRunCheck{}, Warnings: []string{}, Logs: []RuntimeUpdateDryRunLog{},
		}
		if err := writeRuntimeUpdateApplyStatus(instance.DataDir, status); err != nil {
			t.Fatal(err)
		}
		plan := DetectRuntimeUpdateRepairPlan(instance)
		if plan == nil || plan.ActionAvailable || plan.Action != "export" || plan.Code != "runtime_repair_exhausted" || plan.Attempts != runtimeUpdateRepairAttemptLimit {
			t.Fatalf("exhausted plan = %#v", plan)
		}
	})

	t.Run("active recovery takes precedence over config repair", func(t *testing.T) {
		_, _, instance, _ := setupRuntimeApplyDriver(t, storage.InstanceStateStopped)
		values, err := sjconfig.ReadEnvFile(filepath.Join(instance.DataDir, ".env"))
		if err != nil {
			t.Fatal(err)
		}
		if err := sjconfig.UpdateEnvFile(filepath.Join(instance.DataDir, ".env"), map[string]string{
			"SERVER_IMAGE_CANDIDATES": "dockerproxy.net/sdvd/server:" + values["IMAGE_VERSION"] + ",docker.m.daocloud.io/sdvd/server:" + values["IMAGE_VERSION"],
		}); err != nil {
			t.Fatal(err)
		}
		inspection := InspectManagedRuntimeStack(instance.DataDir, instance.State)
		status := RuntimeUpdateApplyStatus{
			ApplyID: "apply_" + strings.Repeat("e", 24), Phase: RuntimeUpdateApplyResumingUpgrade,
			Current: inspection.Current, Target: inspection.Recommended,
			Checks: []RuntimeUpdateDryRunCheck{}, Warnings: []string{}, Logs: []RuntimeUpdateDryRunLog{},
		}
		if err := writeRuntimeUpdateApplyStatus(instance.DataDir, status); err != nil {
			t.Fatal(err)
		}
		plan := DetectRuntimeUpdateRepairPlan(instance)
		if plan == nil || plan.ActionAvailable || plan.Action != "wait" || plan.Code != "runtime_update_in_progress" || plan.ButtonLabel != "等待自动恢复" {
			t.Fatalf("active-recovery plan = %#v", plan)
		}
	})
}

func TestRuntimeUpdateRepairDetectsKnownLegacyConfigAndCompletesUpgrade(t *testing.T) {
	driver, _, instance, _ := setupRuntimeApplyDriver(t, storage.InstanceStateStopped)
	values, err := sjconfig.ReadEnvFile(filepath.Join(instance.DataDir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if err := sjconfig.UpdateEnvFile(filepath.Join(instance.DataDir, ".env"), map[string]string{
		"SERVER_IMAGE_CANDIDATES": "dockerproxy.net/sdvd/server:" + values["IMAGE_VERSION"] + ",docker.m.daocloud.io/sdvd/server:" + values["IMAGE_VERSION"],
	}); err != nil {
		t.Fatal(err)
	}
	plan := DetectRuntimeUpdateRepairPlan(instance)
	if plan == nil || !plan.ActionAvailable || plan.Code != "repairable/legacy_candidates" || plan.ButtonLabel != "修复：规范配置并升级" {
		t.Fatalf("legacy config plan = %#v", plan)
	}
	started, err := driver.StartRuntimeUpdateRepair(context.Background(), instance, 0)
	if err != nil {
		t.Fatal(err)
	}
	if started.Phase != RuntimeUpdateApplyResumingUpgrade || started.RepairAttempts != 1 || !started.ResumeAfterRepair {
		t.Fatalf("known config diagnosis did not start the repair workflow: %#v", started)
	}
	final := waitRuntimeApply(t, driver, instance)
	if final.Phase != RuntimeUpdateApplySucceeded || final.RepairSourceApplyID != started.ApplyID || final.ApplyID == started.ApplyID {
		t.Fatalf("known config repair did not complete a fresh upgrade: started=%#v final=%#v", started, final)
	}
	checks, _ := json.Marshal(final.Checks)
	if !bytes.Contains(checks, []byte(`"known_issue_detection"`)) || !bytes.Contains(checks, []byte(`"known_legacy_config_repaired"`)) || !bytes.Contains(checks, []byte(`"repair_upgrade_preflight"`)) {
		t.Fatalf("known config diagnostic trail missing: %s", checks)
	}
}

func TestRuntimeUpdateRepairResumeAfterCleanupKeepsServerStopped(t *testing.T) {
	driver, _, instance, fake := setupRuntimeApplyDriver(t, storage.InstanceStateStopped)
	values, err := sjconfig.ReadEnvFile(filepath.Join(instance.DataDir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if err := sjconfig.UpdateEnvFile(filepath.Join(instance.DataDir, ".env"), map[string]string{
		"SERVER_IMAGE_CANDIDATES": "dockerproxy.net/sdvd/server:" + values["IMAGE_VERSION"] + ",docker.m.daocloud.io/sdvd/server:" + values["IMAGE_VERSION"],
	}); err != nil {
		t.Fatal(err)
	}
	inspection := InspectManagedRuntimeStack(instance.DataDir, instance.State)
	if !inspection.Repairable {
		t.Fatalf("historical trusted candidate failure was not detected: %#v", inspection)
	}
	sourceID := "apply_" + strings.Repeat("9", 24)
	status := RuntimeUpdateApplyStatus{
		ApplyID: sourceID, Phase: RuntimeUpdateApplyResumingUpgrade, Current: inspection.Current, Target: inspection.Recommended,
		Checks: []RuntimeUpdateDryRunCheck{{Name: "repair_original_runtime", Status: "ok", Message: "restored"}}, Warnings: []string{}, Logs: []RuntimeUpdateDryRunLog{},
		RepairAttempts: 1, RepairSourceApplyID: sourceID, ResumeAfterRepair: true,
	}
	if err := writeRuntimeUpdateApplyStatus(instance.DataDir, status); err != nil {
		t.Fatal(err)
	}
	before := len(fake.applyCalls)
	if err := driver.RecoverRuntimeUpdateApply(context.Background(), instance); err != nil {
		t.Fatal(err)
	}
	final := waitRuntimeApply(t, driver, instance)
	if final.Phase != RuntimeUpdateApplyFailedRolledBack || final.ApplyID != sourceID || final.ResumeAfterRepair || final.ServerRunning || final.ManualAction == "" {
		t.Fatalf("post-repair restart did not converge to manual stopped state: %#v", final)
	}
	backups, err := filepath.Glob(filepath.Join(instance.DataDir, ".local-container", "junimo-update", "config-repair", "*", "original.env"))
	if err != nil || len(backups) != 0 {
		t.Fatalf("Panel bootstrap unexpectedly resumed config repair: %v %v", backups, err)
	}
	calls := strings.Join(fake.applyCalls[before:], "\n")
	if strings.Contains(calls, "up server") || strings.Contains(calls, "up preserve server") || strings.Contains(calls, "compose up") {
		t.Fatalf("Panel bootstrap recovery started the game: %s", calls)
	}
}

func TestRuntimeUpdateRepairResumeAfterRetryManifestBeforeMutationKeepsServerStopped(t *testing.T) {
	driver, _, instance, fake := setupRuntimeApplyDriver(t, storage.InstanceStateStopped)
	inspection := InspectRuntimeStack(instance.DataDir, instance.State)
	applyID := "apply_" + strings.Repeat("8", 24)
	sourceID := "apply_" + strings.Repeat("7", 24)
	target := RuntimeUpdateSelectedPair{
		Server:    RuntimeUpdateSelectedImage{Image: inspection.Recommended.Server.TrustedCandidates[0], Digest: "sha256:" + strings.Repeat("a", 64), ImageID: "sha256:" + strings.Repeat("a", 64)},
		SteamAuth: RuntimeUpdateSelectedImage{Image: inspection.Recommended.SteamAuth.TrustedCandidates[0], Digest: "sha256:" + strings.Repeat("a", 64), ImageID: "sha256:" + strings.Repeat("a", 64)},
	}
	project := strings.ToLower(filepath.Base(instance.DataDir))
	manifest := runtimeUpdateRecoveryManifest{
		SchemaVersion: 3, ApplyID: applyID, Project: project, SteamSessionVolume: "stardew_steam-session",
		SnapshotVolume: project + "_anxi-junimo-update-" + strings.Repeat("8", 24) + "-steam-session",
		OriginalState:  storage.InstanceStateStopped,
		OriginalServer: RuntimeUpdateSelectedImage{Image: inspection.Current.Server.Image, Digest: "sha256:" + strings.Repeat("b", 64), ImageID: "sha256:" + strings.Repeat("b", 64)},
		OriginalAuth:   RuntimeUpdateSelectedImage{Image: inspection.Current.SteamAuth.Image, Digest: "sha256:" + strings.Repeat("c", 64), ImageID: "sha256:" + strings.Repeat("c", 64)},
		Target:         target, OriginalServerVersion: inspection.Current.Server.Tag, TargetServerVersion: inspection.Recommended.Server.Tag,
		ServerImageChanged: true, AuthImageChanged: true,
	}
	if err := createRuntimeRecoveryFiles(instance.DataDir, manifest); err != nil {
		t.Fatal(err)
	}
	manifest.ControlManifestPresent, manifest.ControlDLLPresent, _ = backupRuntimeControlMod(instance.DataDir, applyID)
	manifest.OriginalEnvSHA256, _ = runtimeRecoveryFileSHA256(filepath.Join(runtimeUpdateRecoveryDir(instance.DataDir, applyID), "original.env"))
	manifest.OriginalComposeSHA256, _ = runtimeRecoveryFileSHA256(filepath.Join(runtimeUpdateRecoveryDir(instance.DataDir, applyID), "original-compose.yml"))
	manifest.OriginalControlJSONSHA, _ = runtimeRecoveryFileSHA256(filepath.Join(runtimeUpdateRecoveryDir(instance.DataDir, applyID), "original-control-manifest.json"))
	manifest.OriginalControlDLLSHA, _ = runtimeRecoveryFileSHA256(filepath.Join(runtimeUpdateRecoveryDir(instance.DataDir, applyID), "original-control-StardewAnxiPanel.Control.dll"))
	if err := writeRuntimeUpdateRecoveryManifest(instance.DataDir, manifest); err != nil {
		t.Fatal(err)
	}
	status := RuntimeUpdateApplyStatus{
		ApplyID: applyID, Phase: RuntimeUpdateApplyBackingUp, Current: inspection.Current, Target: inspection.Recommended, Selected: target,
		Checks: []RuntimeUpdateDryRunCheck{{Name: "repair_upgrade_preflight", Status: "ok", Message: "passed"}}, Warnings: []string{}, Logs: []RuntimeUpdateDryRunLog{},
		RepairAttempts: 1, RepairSourceApplyID: sourceID, ResumeAfterRepair: true,
	}
	if err := writeRuntimeUpdateApplyStatus(instance.DataDir, status); err != nil {
		t.Fatal(err)
	}
	before := len(fake.applyCalls)
	if err := driver.RecoverRuntimeUpdateApply(context.Background(), instance); err != nil {
		t.Fatal(err)
	}
	final := waitRuntimeApply(t, driver, instance)
	if final.Phase != RuntimeUpdateApplyFailedRolledBack || final.ApplyID != applyID || final.RepairSourceApplyID != sourceID || final.ResumeAfterRepair || final.ServerRunning || final.ManualAction == "" {
		t.Fatalf("no-mutation retry restart did not converge to manual stopped state: %#v", final)
	}
	calls := strings.Join(fake.applyCalls[before:], "\n")
	if strings.Contains(calls, "up server") || strings.Contains(calls, "up preserve server") || strings.Contains(calls, "compose up") {
		t.Fatalf("Panel bootstrap recovery started the game: %s", calls)
	}
	if _, err := os.Stat(runtimeUpdateRecoveryDir(instance.DataDir, applyID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("successful recovered retry did not clean its recovery directory: %v", err)
	}
}

func TestRuntimeUpdateRepairRejectsTamperedMaterialWithoutMutation(t *testing.T) {
	driver, _, instance, fake := setupRuntimeApplyDriver(t, storage.InstanceStateStopped)
	fake.authProbeErrorTarget = true
	fake.restoreError = true
	if _, err := driver.StartRuntimeUpdateApply(context.Background(), instance, 0); err != nil {
		t.Fatal(err)
	}
	failed := waitRuntimeApply(t, driver, instance)
	if failed.Phase != RuntimeUpdateApplyRollbackFailed {
		t.Fatalf("phase=%s", failed.Phase)
	}
	backup := filepath.Join(runtimeUpdateRecoveryDir(instance.DataDir, failed.ApplyID), "original.env")
	if err := os.WriteFile(backup, []byte("SERVER_IMAGE=untrusted.example/tampered:latest\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	plan := DetectRuntimeUpdateRepairPlan(instance)
	if plan == nil || plan.ActionAvailable || plan.Action != "export" || plan.Code != "recovery_material_invalid" {
		t.Fatalf("tampered material plan = %#v", plan)
	}
	before := len(fake.applyCalls)
	_, err := driver.StartRuntimeUpdateRepair(context.Background(), instance, 0)
	if validation, ok := IsRuntimeUpdateValidationError(err); !ok || validation.Code != "recovery_material_invalid" {
		t.Fatalf("tampered material was not rejected: %v", err)
	}
	if len(fake.applyCalls) != before {
		t.Fatalf("tampered material caused Docker mutation: before=%d calls=%v", before, fake.applyCalls[before:])
	}
}

func TestRuntimeUpdateRollbackRetriesTransientDockerStopTimeout(t *testing.T) {
	driver, _, instance, fake := setupRuntimeApplyDriver(t, storage.InstanceStateStopped)
	fake.serverHealthFailTarget = true
	fake.stopErrorsRemaining = 2
	driver.runtimeUpdateStopTimeout = 100 * time.Millisecond
	if _, err := driver.StartRuntimeUpdateApply(context.Background(), instance, 0); err != nil {
		t.Fatal(err)
	}
	status := waitRuntimeApply(t, driver, instance)
	if status.Phase != RuntimeUpdateApplyFailedRolledBack {
		t.Fatalf("transient stop timeout should still roll back safely: %#v", status)
	}
	if got := strings.Count(strings.Join(fake.applyCalls, "\n"), "stop server,steam-auth"); got < 3 {
		t.Fatalf("stop was not retried after timeout: calls=%v", fake.applyCalls)
	}
}

func TestRuntimeUpdateDefaultTimeoutsCoverSlowColdStart(t *testing.T) {
	driver := New(nil, nil, nil, nil)
	if driver.runtimeUpdateAuthTimeout < 10*time.Minute {
		t.Fatalf("auth verification timeout=%v, want at least 10m", driver.runtimeUpdateAuthTimeout)
	}
	if driver.runtimeUpdateAuthAdvisoryTimeout <= 0 || driver.runtimeUpdateAuthAdvisoryTimeout > 5*time.Second {
		t.Fatalf("Control-only auth advisory timeout=%v, want a positive budget no longer than 5s", driver.runtimeUpdateAuthAdvisoryTimeout)
	}
	if driver.runtimeUpdateServerTimeout < 20*time.Minute {
		t.Fatalf("server verification timeout=%v, want at least 20m", driver.runtimeUpdateServerTimeout)
	}
	if driver.runtimeUpdateStopTimeout < 10*time.Minute {
		t.Fatalf("runtime stop retry timeout=%v, want at least 10m", driver.runtimeUpdateStopTimeout)
	}
}

func TestRuntimeUpdateApplyPreMutationFailureAndRepeatRejected(t *testing.T) {
	driver, _, instance, fake := setupRuntimeApplyDriver(t, storage.InstanceStateRunning)
	for candidate := range fake.metadata {
		if strings.Contains(candidate, "1.5.0-") {
			fake.pullErrors[candidate] = errors.New("pull failed")
			delete(fake.metadata, candidate)
		}
	}
	if _, err := driver.StartRuntimeUpdateApply(context.Background(), instance, 0); err != nil {
		t.Fatal(err)
	}
	status := waitRuntimeApply(t, driver, instance)
	if status.Phase != RuntimeUpdateApplyFailedRolledBack {
		t.Fatalf("phase=%s", status.Phase)
	}
	if strings.Contains(strings.Join(fake.applyCalls, "\n"), "compose down") {
		t.Fatal("instance modified after pull failure")
	}
	current, _ := os.ReadFile(filepath.Join(instance.DataDir, ".env"))
	recommended, _ := sjconfig.BuiltInRuntimeStackManifest()
	_ = os.WriteFile(filepath.Join(instance.DataDir, ".env"), []byte("IMAGE_VERSION="+recommended.Server.Tag+"\nSERVER_IMAGE="+recommended.Server.Image+"\nSERVER_IMAGE_CANDIDATES="+strings.Join(recommended.Server.TrustedCandidates, ",")+"\nSTEAM_SERVICE_IMAGE="+recommended.SteamAuth.Image+"\nSTEAM_SERVICE_IMAGE_CANDIDATES="+strings.Join(recommended.SteamAuth.TrustedCandidates, ",")+"\n"), 0600)
	_, err := driver.StartRuntimeUpdateApply(context.Background(), instance, 0)
	_ = current
	if v, ok := IsRuntimeUpdateValidationError(err); !ok || v.Code != "already_up_to_date" {
		t.Fatalf("repeat not rejected: %v", err)
	}
}

func TestRuntimeUpdateApplyRestartRecoveryDoesNotGuess(t *testing.T) {
	driver, _, instance, _ := setupRuntimeApplyDriver(t, storage.InstanceStateStopped)
	status := RuntimeUpdateApplyStatus{ApplyID: "apply_" + strings.Repeat("a", 24), Phase: RuntimeUpdateApplyWritingConfig, Checks: []RuntimeUpdateDryRunCheck{}, Warnings: []string{}, Logs: []RuntimeUpdateDryRunLog{}}
	if err := writeRuntimeUpdateApplyStatus(instance.DataDir, status); err != nil {
		t.Fatal(err)
	}
	if err := driver.RecoverRuntimeUpdateApply(context.Background(), instance); err != nil {
		t.Fatal(err)
	}
	restored, _ := driver.RuntimeUpdateApplyStatus(instance)
	if restored.Phase != RuntimeUpdateApplyRollbackFailed || restored.ManualAction == "" {
		t.Fatalf("uncertain recovery guessed: %#v", restored)
	}
}

func TestRuntimeUpdateApplyRestartBeforeManifestKeepsServerStopped(t *testing.T) {
	driver, store, instance, fake := setupRuntimeApplyDriver(t, storage.InstanceStateRunning)
	if err := sjconfig.SetSteamInviteEnabled(instance.DataDir, true); err != nil {
		t.Fatal(err)
	}
	status := RuntimeUpdateApplyStatus{
		ApplyID: "apply_" + strings.Repeat("e", 24), Phase: RuntimeUpdateApplyBackingUp,
		ServerWasRunning: true, Checks: []RuntimeUpdateDryRunCheck{}, Warnings: []string{}, Logs: []RuntimeUpdateDryRunLog{},
	}
	if err := writeRuntimeUpdateApplyStatus(instance.DataDir, status); err != nil {
		t.Fatal(err)
	}
	before := len(fake.applyCalls)
	if err := driver.RecoverRuntimeUpdateApply(context.Background(), instance); err != nil {
		t.Fatal(err)
	}
	restored, err := driver.RuntimeUpdateApplyStatus(instance)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Phase != RuntimeUpdateApplyFailedRolledBack || restored.ErrorCode != "panel_restart_before_change" || restored.ServerRunning || restored.ManualAction == "" {
		t.Fatalf("pre-mutation restart was not finalized safely: %#v", restored)
	}
	calls := strings.Join(fake.applyCalls[before:], "\n")
	if !strings.Contains(calls, "stop steam-auth,server") {
		t.Fatalf("pre-mutation recovery did not ensure the runtime is stopped: %s", calls)
	}
	if strings.Contains(calls, "up server") || strings.Contains(calls, "up preserve server") || strings.Contains(calls, "compose up") {
		t.Fatalf("pre-mutation recovery started the game: %s", calls)
	}
	stored, err := store.GetInstance(context.Background(), instance.ID)
	if err != nil || stored.State != storage.InstanceStateStopped {
		t.Fatalf("instance state after recovery = %#v, %v", stored, err)
	}
}

func TestRuntimeUpdateRecoveryManifestUsesPersistedTransactionAcrossPanelVersions(t *testing.T) {
	_, _, instance, _ := setupRuntimeApplyDriver(t, storage.InstanceStateStopped)
	inspection := InspectRuntimeStack(instance.DataDir, instance.State)
	status := RuntimeUpdateApplyStatus{
		ApplyID: "apply_" + strings.Repeat("f", 24), Current: inspection.Current, Target: inspection.Recommended,
	}
	status.Target.Server.TrustedCandidates = []string{"registry.example.invalid/server:old-transaction"}
	status.Target.SteamAuth.TrustedCandidates = []string{"registry.example.invalid/auth:old-transaction"}
	status.Selected = RuntimeUpdateSelectedPair{
		Server:    RuntimeUpdateSelectedImage{Image: status.Target.Server.TrustedCandidates[0], Digest: "sha256:" + strings.Repeat("a", 64), ImageID: "sha256:" + strings.Repeat("b", 64)},
		SteamAuth: RuntimeUpdateSelectedImage{Image: status.Target.SteamAuth.TrustedCandidates[0], Digest: "sha256:" + strings.Repeat("c", 64), ImageID: "sha256:" + strings.Repeat("d", 64)},
	}
	project := strings.ToLower(filepath.Base(instance.DataDir))
	manifest := runtimeUpdateRecoveryManifest{
		SchemaVersion: 3, ApplyID: status.ApplyID, Project: project,
		SnapshotVolume: project + "_anxi-junimo-update-" + strings.Repeat("f", 24) + "-steam-session",
		Target:         status.Selected, OriginalServerVersion: status.Current.Server.Tag, TargetServerVersion: status.Target.Server.Tag,
		OriginalServer: RuntimeUpdateSelectedImage{Image: "old/server", Digest: "sha256:" + strings.Repeat("e", 64), ImageID: "sha256:" + strings.Repeat("e", 64)},
		OriginalAuth:   RuntimeUpdateSelectedImage{Image: "old/auth", Digest: "sha256:" + strings.Repeat("f", 64), ImageID: "sha256:" + strings.Repeat("f", 64)},
	}
	if !validRuntimeUpdateRecoveryManifest(instance, status, manifest) {
		t.Fatal("a transaction valid under its persisted recommendation was coupled to the current Panel manifest")
	}
	manifest.Target.Server.ImageID = "sha256:" + strings.Repeat("0", 64)
	if validRuntimeUpdateRecoveryManifest(instance, status, manifest) {
		t.Fatal("manifest target drift from persisted selected pair was accepted")
	}
}

func TestRuntimeUpdateSnapshotCreateIntentOwnsPossibleCrashWindowVolume(t *testing.T) {
	manifest := runtimeUpdateRecoveryManifest{SchemaVersion: 3, AuthSnapshotCreateIntent: true}
	if !runtimeUpdateAuthSnapshotVolumeCreated(manifest) {
		t.Fatal("write-ahead create intent did not retain cleanup ownership")
	}
}

func TestRuntimeUpdateApplyRestartRollsBackSchema3WriteAheadIntent(t *testing.T) {
	driver, _, instance, fake := setupRuntimeApplyDriver(t, storage.InstanceStateStopped)
	inspection := InspectRuntimeStack(instance.DataDir, instance.State)
	if err := sjconfig.SetSteamInviteEnabled(instance.DataDir, false); err != nil {
		t.Fatal(err)
	}
	applyID := "apply_" + strings.Repeat("d", 24)
	target := RuntimeUpdateSelectedPair{
		Server:    RuntimeUpdateSelectedImage{Image: inspection.Recommended.Server.TrustedCandidates[0], Digest: "sha256:" + strings.Repeat("a", 64), ImageID: "sha256:" + strings.Repeat("a", 64)},
		SteamAuth: RuntimeUpdateSelectedImage{Image: inspection.Recommended.SteamAuth.TrustedCandidates[0], Digest: "sha256:" + strings.Repeat("a", 64), ImageID: "sha256:" + strings.Repeat("a", 64)},
	}
	manifest := runtimeUpdateRecoveryManifest{
		SchemaVersion: 3, ApplyID: applyID, Project: strings.ToLower(filepath.Base(instance.DataDir)), SteamSessionVolume: "stardew_steam-session",
		SnapshotVolume: strings.ToLower(filepath.Base(instance.DataDir)) + "_anxi-junimo-update-" + strings.Repeat("d", 24) + "-steam-session",
		OriginalState:  storage.InstanceStateStopped, OriginalServer: RuntimeUpdateSelectedImage{Image: inspection.Current.Server.Image, Digest: "sha256:" + strings.Repeat("b", 64), ImageID: "sha256:" + strings.Repeat("b", 64)},
		OriginalAuth: RuntimeUpdateSelectedImage{Image: inspection.Current.SteamAuth.Image, Digest: "sha256:" + strings.Repeat("c", 64), ImageID: "sha256:" + strings.Repeat("c", 64)},
		Target:       target, OriginalServerVersion: inspection.Current.Server.Tag, TargetServerVersion: inspection.Recommended.Server.Tag,
		ServerImageChanged: true, AuthImageChanged: true, MutationStarted: true, StopIntent: true, ControlUpdateIntent: true, LastIntent: "control_update",
	}
	if err := createRuntimeRecoveryFiles(instance.DataDir, manifest); err != nil {
		t.Fatal(err)
	}
	manifest.ControlManifestPresent, manifest.ControlDLLPresent, _ = backupRuntimeControlMod(instance.DataDir, applyID)
	manifest.OriginalEnvSHA256, _ = runtimeRecoveryFileSHA256(filepath.Join(runtimeUpdateRecoveryDir(instance.DataDir, applyID), "original.env"))
	manifest.OriginalComposeSHA256, _ = runtimeRecoveryFileSHA256(filepath.Join(runtimeUpdateRecoveryDir(instance.DataDir, applyID), "original-compose.yml"))
	manifest.OriginalControlJSONSHA, _ = runtimeRecoveryFileSHA256(filepath.Join(runtimeUpdateRecoveryDir(instance.DataDir, applyID), "original-control-manifest.json"))
	manifest.OriginalControlDLLSHA, _ = runtimeRecoveryFileSHA256(filepath.Join(runtimeUpdateRecoveryDir(instance.DataDir, applyID), "original-control-StardewAnxiPanel.Control.dll"))
	if err := writeRuntimeUpdateRecoveryManifest(instance.DataDir, manifest); err != nil {
		t.Fatal(err)
	}
	originalControl, err := os.ReadFile(filepath.Join(smapiModDir(instance.DataDir), "StardewAnxiPanel.Control.dll"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(smapiModDir(instance.DataDir), "StardewAnxiPanel.Control.dll"), []byte("interrupted target control"), 0o644); err != nil {
		t.Fatal(err)
	}
	status := RuntimeUpdateApplyStatus{ApplyID: applyID, Phase: RuntimeUpdateApplyStopping, Current: inspection.Current, Target: inspection.Recommended, Selected: target, Checks: []RuntimeUpdateDryRunCheck{}, Warnings: []string{}, Logs: []RuntimeUpdateDryRunLog{}}
	if err := writeRuntimeUpdateApplyStatus(instance.DataDir, status); err != nil {
		t.Fatal(err)
	}
	before := len(fake.applyCalls)
	if err := driver.RecoverRuntimeUpdateApply(context.Background(), instance); err != nil {
		t.Fatal(err)
	}
	restored := waitRuntimeApply(t, driver, instance)
	if restored.Phase != RuntimeUpdateApplyFailedRolledBack || restored.ErrorCode != "panel_restart_recovery" || restored.ServerRunning || restored.ManualAction == "" {
		t.Fatalf("write-ahead recovery did not roll back: %#v", restored)
	}
	calls := strings.Join(fake.applyCalls[before:], "\n")
	if strings.Contains(calls, "up server") || strings.Contains(calls, "up preserve server") || strings.Contains(calls, "compose up") {
		t.Fatalf("write-ahead recovery started the game: %s", calls)
	}
	if !strings.Contains(calls, "stop server,steam-auth") {
		t.Fatalf("legacy schema 3 recovery must conservatively retain Auth transaction scope: %s", calls)
	}
	gotControl, err := os.ReadFile(filepath.Join(smapiModDir(instance.DataDir), "StardewAnxiPanel.Control.dll"))
	if err != nil || !bytes.Equal(gotControl, originalControl) {
		t.Fatalf("Control was not restored after interrupted intent: equal=%v err=%v", bytes.Equal(gotControl, originalControl), err)
	}
}

func TestRuntimeUpdateApplyRestartSchema4DisabledRecoversServerOnly(t *testing.T) {
	driver, _, instance, fake := setupRuntimeApplyDriver(t, storage.InstanceStateStopped)
	configureDisabledRuntimeApplyFixture(t, instance, fake)
	inspection := InspectRuntimeStack(instance.DataDir, instance.State)
	applyID := "apply_" + strings.Repeat("9", 24)
	target := RuntimeUpdateSelectedPair{Server: RuntimeUpdateSelectedImage{
		Image:   inspection.Recommended.Server.TrustedCandidates[0],
		Digest:  "sha256:" + strings.Repeat("a", 64),
		ImageID: "sha256:" + strings.Repeat("a", 64),
	}}
	project := strings.ToLower(filepath.Base(instance.DataDir))
	manifest := runtimeUpdateRecoveryManifest{
		SchemaVersion:      4,
		SteamInviteEnabled: false,
		ApplyID:            applyID,
		Project:            project,
		SnapshotVolume:     project + "_anxi-junimo-update-" + strings.Repeat("9", 24) + "-steam-session",
		OriginalState:      storage.InstanceStateStopped,
		OriginalServer: RuntimeUpdateSelectedImage{
			Image:   inspection.Current.Server.Image,
			Digest:  "sha256:" + strings.Repeat("b", 64),
			ImageID: "sha256:" + strings.Repeat("b", 64),
		},
		Target:                   target,
		OriginalServerVersion:    inspection.Current.Server.Tag,
		TargetServerVersion:      inspection.Recommended.Server.Tag,
		ServerImageChanged:       true,
		AuthImageChanged:         true,
		AuthSnapshotCreated:      true,
		MutationStarted:          true,
		StopIntent:               true,
		ControlUpdateIntent:      true,
		AuthSnapshotCreateIntent: true,
		AuthSnapshotVolumeMade:   true,
		AuthRecreateIntent:       true,
		AuthServiceStartIntent:   true,
		LastIntent:               "control_update",
	}
	if err := createRuntimeRecoveryFiles(instance.DataDir, manifest); err != nil {
		t.Fatal(err)
	}
	manifest.ControlManifestPresent, manifest.ControlDLLPresent, _ = backupRuntimeControlMod(instance.DataDir, applyID)
	manifest.OriginalEnvSHA256, _ = runtimeRecoveryFileSHA256(filepath.Join(runtimeUpdateRecoveryDir(instance.DataDir, applyID), "original.env"))
	manifest.OriginalComposeSHA256, _ = runtimeRecoveryFileSHA256(filepath.Join(runtimeUpdateRecoveryDir(instance.DataDir, applyID), "original-compose.yml"))
	manifest.OriginalControlJSONSHA, _ = runtimeRecoveryFileSHA256(filepath.Join(runtimeUpdateRecoveryDir(instance.DataDir, applyID), "original-control-manifest.json"))
	manifest.OriginalControlDLLSHA, _ = runtimeRecoveryFileSHA256(filepath.Join(runtimeUpdateRecoveryDir(instance.DataDir, applyID), "original-control-StardewAnxiPanel.Control.dll"))
	if err := writeRuntimeUpdateRecoveryManifest(instance.DataDir, manifest); err != nil {
		t.Fatal(err)
	}
	status := RuntimeUpdateApplyStatus{
		ApplyID:  applyID,
		Phase:    RuntimeUpdateApplyStopping,
		Current:  inspection.Current,
		Target:   inspection.Recommended,
		Selected: target,
		Checks:   []RuntimeUpdateDryRunCheck{},
		Warnings: []string{},
		Logs:     []RuntimeUpdateDryRunLog{},
	}
	if err := writeRuntimeUpdateApplyStatus(instance.DataDir, status); err != nil {
		t.Fatal(err)
	}
	beforeCalls := len(fake.calls)
	beforeApplyCalls := len(fake.applyCalls)
	if err := driver.RecoverRuntimeUpdateApply(context.Background(), instance); err != nil {
		t.Fatal(err)
	}
	restored := waitRuntimeApply(t, driver, instance)
	if restored.Phase != RuntimeUpdateApplyFailedRolledBack || restored.ErrorCode != "panel_restart_recovery" || restored.ServerRunning || restored.ManualAction == "" {
		t.Fatalf("schema 4 disabled recovery status=%#v", restored)
	}
	fake.calls = fake.calls[beforeCalls:]
	fake.applyCalls = fake.applyCalls[beforeApplyCalls:]
	if calls := strings.Join(fake.applyCalls, "\n"); !strings.Contains(calls, "stop server") || strings.Contains(calls, "up server") {
		t.Fatalf("schema 4 disabled recovery did not converge server-only and stopped: %s", calls)
	}
	assertNoOptionalAuthRuntimeCalls(t, fake)
}

func TestRuntimeUpdateApplyRestartRollsBackFinalVerificationAndKeepsStopped(t *testing.T) {
	driver, store, instance, fake := setupRuntimeApplyDriver(t, storage.InstanceStateRunning)
	inspection := InspectRuntimeStack(instance.DataDir, instance.State)
	applyID := "apply_" + strings.Repeat("b", 24)
	target := RuntimeUpdateSelectedPair{
		Server:    RuntimeUpdateSelectedImage{Image: inspection.Recommended.Server.TrustedCandidates[0], Digest: "sha256:" + strings.Repeat("a", 64), ImageID: "sha256:" + strings.Repeat("a", 64)},
		SteamAuth: RuntimeUpdateSelectedImage{Image: inspection.Recommended.SteamAuth.TrustedCandidates[0], Digest: "sha256:" + strings.Repeat("a", 64), ImageID: "sha256:" + strings.Repeat("a", 64)},
	}
	manifest := runtimeUpdateRecoveryManifest{SchemaVersion: 1, ApplyID: applyID, Project: strings.ToLower(filepath.Base(instance.DataDir)), SteamSessionVolume: "stardew_steam-session", SnapshotVolume: strings.ToLower(filepath.Base(instance.DataDir)) + "_anxi-junimo-update-" + strings.Repeat("b", 24) + "-steam-session", ServerWasRunning: true, OriginalState: storage.InstanceStateRunning, OriginalServer: RuntimeUpdateSelectedImage{Image: inspection.Current.Server.Image, Digest: "sha256:" + strings.Repeat("b", 64), ImageID: "sha256:" + strings.Repeat("b", 64)}, OriginalAuth: RuntimeUpdateSelectedImage{Image: inspection.Current.SteamAuth.Image, Digest: "sha256:" + strings.Repeat("c", 64), ImageID: "sha256:" + strings.Repeat("c", 64)}, Target: target, ConfigWritten: true, AuthRecreated: true, ServerRecreated: true}
	if err := createRuntimeRecoveryFiles(instance.DataDir, manifest); err != nil {
		t.Fatal(err)
	}
	if err := writeRuntimeTargetEnvAtomic(instance.DataDir, inspection.Recommended, target); err != nil {
		t.Fatal(err)
	}
	status := RuntimeUpdateApplyStatus{ApplyID: applyID, Phase: RuntimeUpdateApplyVerifyingServer, Current: inspection.Current, Target: inspection.Recommended, Selected: target, ServerWasRunning: true, Checks: []RuntimeUpdateDryRunCheck{}, Warnings: []string{}, Logs: []RuntimeUpdateDryRunLog{}}
	if err := writeRuntimeUpdateApplyStatus(instance.DataDir, status); err != nil {
		t.Fatal(err)
	}
	before := len(fake.applyCalls)
	if err := driver.RecoverRuntimeUpdateApply(context.Background(), instance); err != nil {
		t.Fatal(err)
	}
	restored := waitRuntimeApply(t, driver, instance)
	if restored.Phase != RuntimeUpdateApplyFailedRolledBack || restored.ErrorCode != "panel_restart_recovery" || restored.ServerRunning || restored.ManualAction == "" {
		t.Fatalf("final-verification restart did not converge to manual stopped state: %#v", restored)
	}
	calls := strings.Join(fake.applyCalls[before:], "\n")
	if strings.Contains(calls, "up server") || strings.Contains(calls, "up preserve server") || strings.Contains(calls, "compose up") {
		t.Fatalf("final-verification recovery started the game: %s", calls)
	}
	stored, err := store.GetInstance(context.Background(), instance.ID)
	if err != nil || stored.State != storage.InstanceStateStopped {
		t.Fatalf("instance state after recovery = %#v, %v", stored, err)
	}
}

func TestRuntimeUpdateApplyRejectsConflictingJobs(t *testing.T) {
	for _, jobType := range []string{"stardew_lifecycle", "stardew_steam_auth"} {
		t.Run(jobType, func(t *testing.T) {
			driver, _, instance, _ := setupRuntimeApplyDriver(t, storage.InstanceStateStopped)
			release := make(chan struct{})
			job, err := driver.jobs.Start(context.Background(), jobs.Spec{Type: jobType, TargetType: "instance", TargetID: instance.ID, Run: func(context.Context, *jobs.Context) error { <-release; return nil }})
			if err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(time.Second)
			for time.Now().Before(deadline) {
				active, _ := driver.jobs.Active(context.Background(), storage.ListActiveJobsFilter{TargetType: "instance", TargetID: instance.ID, Types: []string{jobType}})
				if len(active) > 0 {
					break
				}
				time.Sleep(time.Millisecond)
			}
			_, err = driver.StartRuntimeUpdateApply(context.Background(), instance, 0)
			close(release)
			if validation, ok := IsRuntimeUpdateValidationError(err); !ok || validation.Code != "runtime_update_busy" {
				t.Fatalf("concurrent job not rejected (job %s): %v", job.ID, err)
			}
		})
	}
}
