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
	applyMu                sync.Mutex
	applyCalls             []string
	authReady, authTicket  bool
	authFailTarget         bool
	authProbeErrorTarget   bool
	inviteUnavailable      bool
	serverHealthFailTarget bool
	controlContractFail    bool
	loadedVersionMismatch  bool
	digestMismatchService  string
	upErrorService         string
	restoreError           bool
	removeImageError       bool
	stopErrorsRemaining    int
}

func newRuntimeApplyFakeDocker(dataDir string) *runtimeApplyFakeDocker {
	return &runtimeApplyFakeDocker{runtimeUpdateFakeDocker: newRuntimeUpdateFakeDocker(dataDir), authReady: true, authTicket: true}
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
	if service == f.upErrorService {
		return errors.New("injected up failure")
	}
	if service == "server" {
		manifest, _ := sjconfig.BuiltInRuntimeStackManifest()
		_ = os.MkdirAll(filepath.Join(dataDir, ".local-container", "control"), 0o755)
		_ = os.WriteFile(filepath.Join(dataDir, ".local-container", "control", "options.json"), []byte(`{"controlModVersion":"`+manifest.Control.Version+`"}`), 0o600)
		_ = os.WriteFile(filepath.Join(dataDir, ".local-container", "control", "status.json"), []byte(`{"state":"save-loaded","commandResultVersion":1,"updatedAt":"2026-07-20T00:00:00Z"}`), 0o600)
		_ = os.WriteFile(filepath.Join(dataDir, ".local-container", "control", "players.json"), []byte(`{"players":[],"updatedAt":"2026-07-20T00:00:00Z"}`), 0o600)
	}
	return nil
}
func (f *runtimeApplyFakeDocker) RuntimeComposeUpServicePreserve(_ context.Context, dataDir string, _ string, service string) error {
	f.applyCall("up preserve " + service)
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
	digest := "sha256:" + strings.Repeat("a", 64)
	if !f.targetConfigured(dataDir) {
		if service == "server" {
			digest = "sha256:" + strings.Repeat("b", 64)
		} else {
			digest = "sha256:" + strings.Repeat("c", 64)
		}
	}
	if f.targetConfigured(dataDir) && f.digestMismatchService == service {
		digest = "sha256:" + strings.Repeat("d", 64)
	}
	return paneldocker.RuntimeServiceMetadata{ContainerID: strings.Repeat("a", 12), ImageID: digest, State: "running", Health: "healthy"}, nil
}
func (f *runtimeApplyFakeDocker) RuntimeSteamAuthReady(_ context.Context, dataDir, _ string) (paneldocker.RuntimeSteamReady, error) {
	if f.targetConfigured(dataDir) && f.authProbeErrorTarget {
		return paneldocker.RuntimeSteamReady{}, errors.New("auth endpoint unavailable")
	}
	if f.targetConfigured(dataDir) && f.authFailTarget {
		return paneldocker.RuntimeSteamReady{Ready: f.authReady, HasTicket: f.authTicket}, nil
	}
	return paneldocker.RuntimeSteamReady{Ready: true, HasTicket: true}, nil
}
func (f *runtimeApplyFakeDocker) RuntimeServerHealth(_ context.Context, dataDir, _ string) error {
	if f.targetConfigured(dataDir) && f.serverHealthFailTarget {
		return errors.New("health failed")
	}
	return nil
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
	if err := os.WriteFile(filepath.Join(instance.DataDir, ".local-container", "control", "options.json"), []byte(`{"controlModVersion":"`+manifest.Control.Version+`"}`), 0o600); err != nil {
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

func waitRuntimeApply(t *testing.T, driver *Driver, instance registry.Instance) RuntimeUpdateApplyStatus {
	t.Helper()
	time.Sleep(250 * time.Millisecond)
	deadline := time.Now().Add(8 * time.Second)
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

func TestRuntimeUpdateControlOnlyPreservesRunningAuthContainer(t *testing.T) {
	driver, _, instance, fake := setupRuntimeApplyDriver(t, storage.InstanceStateRunning)
	// The incident was triggered by a slow/offline Steam login path. A healthy,
	// unchanged auth container must not be re-probed through that network path.
	fake.authProbeErrorTarget = true
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
	inspection := InspectManagedRuntimeStack(instance.DataDir, instance.State)
	if inspection.Status != sjconfig.RuntimeStackStatusUpdateAvailable || inspection.Code != "control_update_available" {
		t.Fatalf("control-only fixture=%+v", inspection)
	}
	if _, err := driver.StartRuntimeUpdateApply(context.Background(), instance, 0); err != nil {
		t.Fatal(err)
	}
	status := waitRuntimeApply(t, driver, instance)
	if status.Phase != RuntimeUpdateApplySucceeded {
		t.Fatalf("control-only phase=%s code=%s error=%s", status.Phase, status.ErrorCode, status.Error)
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
	if status.Selected.SteamAuth.ImageID == "" || status.Selected.Server.ImageID == "" {
		t.Fatalf("selected immutable image IDs missing: %+v", status.Selected)
	}
}

func TestRuntimeUpdateApplyImageCleanupFailureIsWarning(t *testing.T) {
	driver, _, instance, fake := setupRuntimeApplyDriver(t, storage.InstanceStateRunning)
	fake.removeImageError = true
	if _, err := driver.StartRuntimeUpdateApply(context.Background(), instance, 0); err != nil {
		t.Fatal(err)
	}
	status := waitRuntimeApply(t, driver, instance)
	if status.Phase != RuntimeUpdateApplySucceeded || !strings.Contains(strings.Join(status.Warnings, "\n"), "旧镜像") {
		t.Fatalf("cleanup failure changed success semantics or omitted warning: %#v", status)
	}
}

func TestRuntimeUpdateApplyPinsRunningContainerImageIDsWithoutPersistingDigestConfig(t *testing.T) {
	driver, _, instance, fake := setupRuntimeApplyDriver(t, storage.InstanceStateRunning)
	fake.metadata["sdvd/server:1.4.0-preview.1"] = paneldocker.RuntimeImageMetadata{ID: "sha256:" + strings.Repeat("d", 64), Digest: "sha256:" + strings.Repeat("d", 64)}
	fake.metadata["anxiyizhi/junimo-steam-service-cn:1.4.0-anxi.1"] = paneldocker.RuntimeImageMetadata{ID: "sha256:" + strings.Repeat("e", 64), Digest: "sha256:" + strings.Repeat("e", 64)}
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
	if env["SERVER_IMAGE"] != "sdvd/server:1.4.0-preview.1" || env["STEAM_SERVICE_IMAGE"] != "anxiyizhi/junimo-steam-service-cn:1.4.0-anxi.1" {
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
	fake.authFailTarget = true
	fake.authReady = false
	fake.authTicket = false
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
}

func TestRequiredRuntimeUpdateAutomaticallyChainsDryRunAndApply(t *testing.T) {
	driver, _, instance, fake := setupRuntimeApplyDriver(t, storage.InstanceStateRunning)
	driver.panelVersion = "0.3.5"
	if err := os.Remove(runtimeUpdateDryRunStatusPath(instance.DataDir)); err != nil {
		t.Fatal(err)
	}
	fake.authFailTarget = true
	fake.authReady = false
	fake.authTicket = false
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
	before := len(fake.applyCalls)
	driver.StartRequiredRuntimeUpdate(context.Background(), instance)
	time.Sleep(25 * time.Millisecond)
	if len(fake.applyCalls) != before {
		t.Fatalf("identical failed Panel/stack auto-retried: before=%d after=%d", before, len(fake.applyCalls))
	}
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

func TestRuntimeUpdateRepairResumeAfterCleanupRestartsFullDetection(t *testing.T) {
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
	if err := driver.RecoverRuntimeUpdateApply(context.Background(), instance); err != nil {
		t.Fatal(err)
	}
	final := waitRuntimeApply(t, driver, instance)
	if final.Phase != RuntimeUpdateApplySucceeded || final.ApplyID == sourceID || final.RepairSourceApplyID != sourceID || final.ResumeAfterRepair {
		t.Fatalf("post-repair restart did not resume detection and upgrade: %#v", final)
	}
	backups, err := filepath.Glob(filepath.Join(instance.DataDir, ".local-container", "junimo-update", "config-repair", "*", "original.env"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("historical config repair did not create its private backup: %v %v", backups, err)
	}
	checks, _ := json.Marshal(final.Checks)
	if !bytes.Contains(checks, []byte(`"known_legacy_config_repaired"`)) {
		t.Fatalf("historical repair diagnosis was not retained: %s", checks)
	}
}

func TestRuntimeUpdateRepairResumeAfterRetryManifestBeforeMutation(t *testing.T) {
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
	if final.Phase != RuntimeUpdateApplySucceeded || final.ApplyID != applyID || final.RepairSourceApplyID != sourceID || final.ResumeAfterRepair {
		t.Fatalf("no-mutation retry restart did not continue the same upgrade: %#v", final)
	}
	if len(fake.applyCalls) == before {
		t.Fatal("no-mutation retry restart did not execute the recovered apply")
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

func TestRuntimeUpdateApplyRestartBeforeManifestFinishesWithoutDockerMutation(t *testing.T) {
	driver, _, instance, fake := setupRuntimeApplyDriver(t, storage.InstanceStateRunning)
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
	if restored.Phase != RuntimeUpdateApplyFailedRolledBack || restored.ErrorCode != "panel_restart_before_change" || !restored.ServerRunning || restored.ManualAction != "" {
		t.Fatalf("pre-mutation restart was not finalized safely: %#v", restored)
	}
	if len(fake.applyCalls) != before {
		t.Fatalf("pre-mutation recovery touched Docker: %v", fake.applyCalls[before:])
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
	driver, _, instance, _ := setupRuntimeApplyDriver(t, storage.InstanceStateStopped)
	inspection := InspectRuntimeStack(instance.DataDir, instance.State)
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
	if err := driver.RecoverRuntimeUpdateApply(context.Background(), instance); err != nil {
		t.Fatal(err)
	}
	restored := waitRuntimeApply(t, driver, instance)
	if restored.Phase != RuntimeUpdateApplyFailedRolledBack || restored.ErrorCode != "panel_restart_recovery" {
		t.Fatalf("write-ahead recovery did not roll back: %#v", restored)
	}
	gotControl, err := os.ReadFile(filepath.Join(smapiModDir(instance.DataDir), "StardewAnxiPanel.Control.dll"))
	if err != nil || !bytes.Equal(gotControl, originalControl) {
		t.Fatalf("Control was not restored after interrupted intent: equal=%v err=%v", bytes.Equal(gotControl, originalControl), err)
	}
}

func TestRuntimeUpdateApplyRestartSafelyContinuesFinalVerification(t *testing.T) {
	driver, _, instance, _ := setupRuntimeApplyDriver(t, storage.InstanceStateStopped)
	inspection := InspectRuntimeStack(instance.DataDir, instance.State)
	applyID := "apply_" + strings.Repeat("b", 24)
	target := RuntimeUpdateSelectedPair{
		Server:    RuntimeUpdateSelectedImage{Image: inspection.Recommended.Server.TrustedCandidates[0], Digest: "sha256:" + strings.Repeat("a", 64), ImageID: "sha256:" + strings.Repeat("a", 64)},
		SteamAuth: RuntimeUpdateSelectedImage{Image: inspection.Recommended.SteamAuth.TrustedCandidates[0], Digest: "sha256:" + strings.Repeat("a", 64), ImageID: "sha256:" + strings.Repeat("a", 64)},
	}
	manifest := runtimeUpdateRecoveryManifest{SchemaVersion: 1, ApplyID: applyID, Project: strings.ToLower(filepath.Base(instance.DataDir)), SteamSessionVolume: "stardew_steam-session", SnapshotVolume: strings.ToLower(filepath.Base(instance.DataDir)) + "_anxi-junimo-update-" + strings.Repeat("b", 24) + "-steam-session", OriginalState: storage.InstanceStateStopped, OriginalServer: RuntimeUpdateSelectedImage{Image: inspection.Current.Server.Image, Digest: "sha256:" + strings.Repeat("b", 64), ImageID: "sha256:" + strings.Repeat("b", 64)}, OriginalAuth: RuntimeUpdateSelectedImage{Image: inspection.Current.SteamAuth.Image, Digest: "sha256:" + strings.Repeat("c", 64), ImageID: "sha256:" + strings.Repeat("c", 64)}, Target: target, ConfigWritten: true, AuthRecreated: true, ServerRecreated: true}
	if err := createRuntimeRecoveryFiles(instance.DataDir, manifest); err != nil {
		t.Fatal(err)
	}
	if err := writeRuntimeTargetEnvAtomic(instance.DataDir, inspection.Recommended, target); err != nil {
		t.Fatal(err)
	}
	status := RuntimeUpdateApplyStatus{ApplyID: applyID, Phase: RuntimeUpdateApplyVerifyingServer, Current: inspection.Current, Target: inspection.Recommended, Selected: target, Checks: []RuntimeUpdateDryRunCheck{}, Warnings: []string{}, Logs: []RuntimeUpdateDryRunLog{}}
	if err := writeRuntimeUpdateApplyStatus(instance.DataDir, status); err != nil {
		t.Fatal(err)
	}
	if err := driver.RecoverRuntimeUpdateApply(context.Background(), instance); err != nil {
		t.Fatal(err)
	}
	restored := waitRuntimeApply(t, driver, instance)
	if restored.Phase != RuntimeUpdateApplySucceeded || restored.ServerRunning {
		t.Fatalf("safe continuation failed: %#v", restored)
	}
}

func TestRuntimeUpdateApplyRejectsConcurrentLifecycleJob(t *testing.T) {
	driver, _, instance, _ := setupRuntimeApplyDriver(t, storage.InstanceStateStopped)
	release := make(chan struct{})
	job, err := driver.jobs.Start(context.Background(), jobs.Spec{Type: "stardew_lifecycle", TargetType: "instance", TargetID: instance.ID, Run: func(context.Context, *jobs.Context) error { <-release; return nil }})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		active, _ := driver.jobs.Active(context.Background(), storage.ListActiveJobsFilter{TargetType: "instance", TargetID: instance.ID, Types: []string{"stardew_lifecycle"}})
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
}
