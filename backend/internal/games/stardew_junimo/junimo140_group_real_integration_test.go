//go:build integration

package stardew_junimo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	appconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

type junimo140RealGroupEngine struct {
	*paneldocker.Client
	scenario, root, failedProject, targetImageID string
	targetFailed, restoreFailed                  atomic.Bool
	diagnosticSequence, diagnosticErrors         atomic.Uint32
	twoRunning                                   bool
	peerDir, firstDir, firstProject, firstID     string
	saveDelayObserved                            atomic.Bool
	saveDelayed                                  atomic.Bool
	saveDelayDone                                chan error
}

var _ junimo140GroupDocker = (*junimo140RealGroupEngine)(nil)

// The second world's real post-Saved status read is delayed while the first
// world has already stopped. No daemon state or game files are altered.
func (f *junimo140RealGroupEngine) stopAndDelaySavePeer(ctx context.Context, dir string, snapshots []paneldocker.RuntimeJunimo140ServiceSnapshot, images map[string][]string) error {
	if err := f.Client.RuntimeStopJunimo140Services(ctx, dir, snapshots, images); err != nil {
		return err
	}
	if f.twoRunning && len(snapshots) == 1 && snapshots[0].Project != f.failedProject && snapshots[0].Running && snapshots[0].ImageID != f.targetImageID && f.saveDelayed.CompareAndSwap(false, true) {
		f.firstDir, f.firstProject = dir, snapshots[0].Project
		f.firstID = snapshots[0].ContainerID
	}
	return nil
}

func (f *junimo140RealGroupEngine) ComposeExecPipe(ctx context.Context, dir, service, input string, args ...string) (paneldocker.CommandResult, error) {
	result, err := f.Client.ComposeExecPipe(ctx, dir, service, input, args...)
	if err != nil || !f.twoRunning || dir != f.peerDir || len(args) == 0 || !strings.HasSuffix(args[len(args)-1], "/status") || !f.saveDelayed.Load() || !f.saveDelayObserved.CompareAndSwap(false, true) {
		return result, err
	}
	var delayErr error
	rawInspect, inspectErr := exec.CommandContext(ctx, "docker", "inspect", f.firstID).Output()
	var first []struct {
		ID    string
		State struct {
			Status  string
			Running bool
		}
		Config struct{ Labels map[string]string }
	}
	if inspectErr != nil || json.Unmarshal(rawInspect, &first) != nil || len(first) != 1 || first[0].ID != f.firstID || first[0].State.Running || first[0].State.Status != "exited" || first[0].Config.Labels["com.docker.compose.project"] != f.firstProject {
		delayErr = errors.New("first world remained writable during peer save delay")
	}
	if delayErr == nil && exec.CommandContext(ctx, "docker", "exec", f.firstID, "true").Run() == nil {
		delayErr = errors.New("stopped world still accepted process execution")
	}
	if delayErr == nil {
		select {
		case <-ctx.Done():
			delayErr = ctx.Err()
		case <-time.After(time.Second):
		}
	}
	raw, _ := json.Marshal(map[string]any{"firstExited": delayErr == nil, "secondPostSavedReadDelayed": true, "delaySeconds": 1, "firstExecRejected": delayErr == nil})
	if writeErr := os.WriteFile(filepath.Join(f.root, "save-stop-window.json"), raw, 0600); writeErr != nil {
		delayErr = errors.Join(delayErr, writeErr)
	}
	f.saveDelayDone <- delayErr
	return result, delayErr
}

func (f *junimo140RealGroupEngine) RuntimeCreateJunimo140ServicePinned(ctx context.Context, dir, project, service, imageID string) error {
	if f.scenario != "success" && project == f.failedProject && service == "server" && imageID == f.targetImageID && f.targetFailed.CompareAndSwap(false, true) {
		return errors.New("injected isolated peer target recreation failure")
	}
	return f.Client.RuntimeCreateJunimo140ServicePinned(ctx, dir, project, service, imageID)
}

func (f *junimo140RealGroupEngine) RuntimeRestoreJunimo140Instance(ctx context.Context, dir string, request paneldocker.RuntimeInstanceArchiveRequest) error {
	if f.scenario == "recovery" && request.Project == f.failedProject && f.restoreFailed.CompareAndSwap(false, true) {
		rel, err := filepath.Rel(f.root, request.InstanceHostDir)
		if err != nil || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return errors.New("isolated restore fault escaped its fixture root")
		}
		if err := os.Remove(filepath.Join(request.InstanceHostDir, ".env")); err != nil {
			return err
		}
		return errors.New("injected isolated restore interruption with missing env")
	}
	return f.Client.RuntimeRestoreJunimo140Instance(ctx, dir, request)
}

func sjUpdateRealGroupEnv(dir, volume, identity, apiPort string) error {
	return sjconfig.UpdateEnvFile(filepath.Join(dir, ".env"), map[string]string{
		"IMAGE_VERSION": "1.5.0-preview.125", "SERVER_IMAGE": "sdvd/server:1.5.0-preview.125", "SERVER_IMAGE_CANDIDATES": "sdvd/server:1.5.0-preview.125",
		"STEAM_INVITE_ENABLED": "false", "STEAM_INVITE_AUTH_STATE": "disabled", "STEAM_AUTH_COMPLETED": "false", "STEAM_USERNAME": "", "STEAM_PASSWORD": "", "STEAM_REFRESH_TOKEN": "",
		"GAME_DATA_VOLUME": volume, "GAME_PORT": "0", "QUERY_PORT": "0", "VNC_PORT": "0", "API_PORT": apiPort, "API_ENABLED": "true", "API_KEY": identity,
		"VNC_PASSWORD": "fixture-" + identity, "ALLOW_INSECURE_SETUP": "false", "SERVER_TPS": "5", "SERVER_FPS": "0", "USER_ID": "1000", "GROUP_ID": "1000", "SAP_PLAYER_AUTH_MODE": "disabled",
	})
}

func sjRealGroupSourceControl(t *testing.T, ctx context.Context, repo, version string) ([]byte, []byte) {
	t.Helper()
	if version == "0.3.9" {
		return smapiModManifest, smapiModDLL
	}
	if version != "0.3.8" {
		t.Fatal("unknown isolated source Control version")
	}
	const commit = "c8ec1db143e451b458c158bc63c7c9f38fffc834"
	resolved, err := exec.CommandContext(ctx, "git", "-C", repo, "rev-parse", "v0.7.2^{}").Output()
	if err != nil || strings.TrimSpace(string(resolved)) != commit {
		t.Fatal("formal source tag does not match fixed v0.7.2 commit", err)
	}
	read := func(name string) []byte {
		raw, err := exec.CommandContext(ctx, "git", "-C", repo, "show", commit+":backend/internal/games/stardew_junimo/embedded/smapi-mod/"+name).Output()
		if err != nil || len(raw) == 0 {
			t.Fatal("read fixed formal Control artifact", err)
		}
		return raw
	}
	manifest, dll := read("manifest.json"), read("StardewAnxiPanel.Control.dll")
	var metadata struct{ Version string }
	if json.Unmarshal(manifest, &metadata) != nil || metadata.Version != version {
		t.Fatal("formal Control artifact version mismatch")
	}
	return manifest, dll
}

func sjWaitRealGroupSource038(ctx context.Context, driver *Driver, client *paneldocker.Client, instance registry.Instance, imageID string) error {
	deadline := time.Now().Add(driver.runtimeUpdateServerTimeout)
	for time.Now().Before(deadline) {
		metadata, inspectErr := client.RuntimeServiceInspect(ctx, instance.DataDir, filepath.Base(instance.DataDir), "server")
		if inspectErr == nil && metadata.ImageID != imageID {
			return errors.New("formal source image identity mismatch")
		}
		raw, readErr := os.ReadFile(filepath.Join(controlDir(instance.DataDir), "options.json"))
		var options struct{ ControlModVersion string }
		controlReady := readErr == nil && json.Unmarshal(raw, &options) == nil && options.ControlModVersion == "0.3.8" && commandResultSupported(instance.DataDir)
		state := readSMAPIStatus(instance.DataDir)
		if inspectErr == nil && metadata.State == "running" && (metadata.Health == "" || metadata.Health == "healthy") && controlReady && (state == "launched" || state == "save-loaded") && client.RuntimeServerHealth(ctx, instance.DataDir, filepath.Base(instance.DataDir)) == nil && runtimeInfoContractReady(ctx, client, instance.DataDir, "1.5.0-preview.125") {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(driver.runtimeUpdatePollInterval):
		}
	}
	return errors.New("fixed formal source runtime did not become ready")
}

// Opt-in host test. Run directly on the Docker host (or inside an owned DinD),
// never in a socket-only container whose private bind paths the daemon cannot see.
func TestJunimo140SharedGroupRealRunningStoppedAndColdOptIn(t *testing.T) {
	if os.Getenv("ANXI_REAL_JUNIMO140_GROUP") != "1" {
		t.Skip("set ANXI_REAL_JUNIMO140_GROUP=1 with the authorized local fixture")
	}
	scenario := os.Getenv("ANXI_REAL_JUNIMO140_GROUP_CASE")
	if scenario == "" {
		scenario = "success"
	}
	webMode := scenario == "web"
	fullStackWeb := scenario == "full-stack-web"
	if webMode || fullStackWeb {
		scenario = "success"
	}
	if scenario != "success" && scenario != "rollback" && scenario != "recovery" {
		t.Fatal("unknown isolated group test scenario")
	}
	sourceControl := os.Getenv("ANXI_REAL_JUNIMO140_GROUP_SOURCE_CONTROL")
	if sourceControl == "" {
		sourceControl = "0.3.9"
	}
	timeout := 15 * time.Minute
	if webMode || fullStackWeb {
		timeout = 25 * time.Minute
	}
	if fullStackWeb && os.Getenv("ANXI_REAL_FULL_STACK_FAULT") == "target-exit" {
		// The real Docker Panel retains its production 20-minute health gate.
		// Its source preparation and whole-group rollback need additional time.
		timeout = 60 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	for _, key := range []string{"STEAM_USERNAME", "STEAM_PASSWORD", "STEAM_REFRESH_TOKEN"} {
		t.Setenv(key, "")
	}
	client := paneldocker.NewClient(paneldocker.Options{DockerPath: "docker"})
	repo, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	sourceControlManifest, sourceControlDLL := sjRealGroupSourceControl(t, ctx, repo, sourceControl)
	sourceControlHash := sha256.Sum256(sourceControlDLL)
	baseline := filepath.Join(repo, "tmp", "junimo140-20261015", "house-bed-matrix-v1", "baseline.tar")
	if info, err := os.Lstat(baseline); err != nil || !info.Mode().IsRegular() {
		t.Fatal("authorized isolated save archive unavailable")
	}
	identity := strings.TrimPrefix(newRuntimeApplyID(), "apply_")
	root := filepath.Join(repo, "tmp", "junimo140-20261015", "group-real-"+identity)
	if fullStackWeb {
		if os.Getenv("ANXI_REAL_FULL_STACK_ISOLATED_DOCKER") != "1" || os.Getenv("ANXI_REAL_FULL_STACK_DATA_DIR") != "/data" {
			t.Fatal("full-stack Web fixture requires its dedicated DinD and /data mount")
		}
		root = "/data"
	}
	createRoot := os.Mkdir
	if fullStackWeb {
		createRoot = os.MkdirAll
	}
	if err := createRoot(root, 0700); err != nil {
		t.Fatal(err)
	}
	store, err := storage.Open(ctx, appconfig.Config{DataDir: root, DBPath: filepath.Join(root, "panel.db")})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	webPassword := ""
	if webMode {
		// Authentication initialization precedes installation in a real Panel.
		webPassword = sjSeedRealPanelAdmin(t, ctx, store)
	}
	manager := jobs.NewManager(store, slog.Default())
	twoRunning := os.Getenv("ANXI_REAL_JUNIMO140_GROUP_TWO_RUNNING") == "1"
	engine := &junimo140RealGroupEngine{Client: client, scenario: scenario, root: root, twoRunning: twoRunning, saveDelayDone: make(chan error, 1)}
	driver := New(engine, slog.Default(), manager, store)
	if fullStackWeb {
		driver.containerDataDir, driver.panelVersion = root, "0.7.3"
	}
	driver.runtimeUpdateServerTimeout = 3 * time.Minute
	driver.runtimeUpdatePollInterval = 500 * time.Millisecond
	const targetRef = "sdvd/server:1.5.0-preview.140"
	const sourceRef = "sha256:10f438581d741fc146ce710cbe20099475ac68908e99f565cf449f0b8192ccf6"
	targetImage, err := client.RuntimeImageInspect(ctx, root, targetRef)
	if err != nil || targetImage.Digest != junimo140Digest {
		t.Fatalf("fixed140 image unavailable: %v", err)
	}
	engine.targetImageID = targetImage.ID
	sourceImage, err := client.RuntimeImageInspect(ctx, root, sourceRef)
	if err != nil || sourceImage.Digest != "sha256:10f438581d741fc146ce710cbe20099475ac68908e99f565cf449f0b8192ccf6" {
		t.Fatalf("fixed125 image unavailable: %v", err)
	}
	var instances []registry.Instance
	for _, suffix := range []string{"a", "b", "c"} {
		project := "j140-real-" + identity + "-" + suffix
		dataDir := filepath.Join(root, project)
		stored, err := store.CreateInstance(ctx, storage.CreateInstanceParams{ID: suffix, DriverID: DriverID, Name: "isolated " + suffix, DataDir: dataDir, State: storage.InstanceStateAdminCreated, DriverPayload: `{"fixture":"shared-real"}`})
		if err != nil {
			t.Fatal(err)
		}
		instance := registry.Instance{ID: stored.ID, DriverID: DriverID, DataDir: dataDir, State: stored.State}
		if err := driver.Prepare(ctx, instance); err != nil {
			t.Fatal("prepare managed fixture", err)
		}
		instances = append(instances, instance)
	}
	primary := instances[0]
	engine.failedProject = filepath.Base(instances[1].DataDir)
	engine.peerDir = instances[1].DataDir
	primaryProject := filepath.Base(primary.DataDir)
	sourceApplyID, modApplyID, applyID := newRuntimeApplyID(), newRuntimeApplyID(), newRuntimeApplyID()
	sourceVolume := primaryProject + "_anxi-smapi-update-" + strings.TrimPrefix(sourceApplyID, "apply_")
	coldSourceVolume, coldSourceApplyID := "", ""
	plan := junimo140GroupPlan{}
	proof := map[string]any{"kind": "real group engineering E2E; no Steam/GOG authentication proof", "scenario": scenario, "identity": identity, "root": root, "sourceVolume": sourceVolume, "sourceApplyId": sourceApplyID, "modApplyId": modApplyID, "applyId": applyID, "sourceImageId": sourceImage.ID, "targetImageId": targetImage.ID, "phase": "preparing"}
	proof["sourceControlVersion"], proof["sourceControlSha256"] = sourceControl, hex.EncodeToString(sourceControlHash[:])
	if webMode {
		proof["entryPoint"] = "real_panel_http"
	}
	writeProof := func() {
		t.Helper()
		if err := writePrivateRuntimeUpdateJSON(filepath.Join(root, "proof.json"), proof); err != nil {
			t.Error("write isolated proof", err)
		}
	}
	writeProof()
	completed := false
	t.Cleanup(func() {
		if !completed {
			proof["phase"] = "failed_resources_retained"
			writeProof()
			t.Log("isolated group resources and recovery materials retained at", root)
			_ = store.Close()
			return
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cleanupCancel()
		if err := sjCleanupRealGroup(cleanupCtx, driver, client, plan, root, primaryProject, sourceVolume, sourceApplyID, sourceImage.ID, targetImage.ID); err != nil {
			t.Error("cleanup isolated group", err)
			return
		}
		if coldSourceVolume != "" {
			if err := client.RuntimeRemoveJunimo140Staging(cleanupCtx, root, filepath.Base(instances[2].DataDir), coldSourceVolume, coldSourceApplyID); err != nil {
				t.Error("cleanup independent cold source", err)
				return
			}
		}
		proof["cleaned"] = true
		writeProof()
		_ = store.Close()
	})
	if err := client.RuntimeCreateJunimo140Staging(ctx, root, primaryProject, sourceVolume, sourceApplyID); err != nil {
		t.Fatal(err)
	}
	// The authorized project game source is mounted read-only by this pinned
	// full-hash clone; its stopped compile holder and original data are untouched.
	if err := client.RuntimeClonePinnedJunimo140Game(ctx, root, "install-test-20260905_game-data", sourceVolume, targetRef, targetImage.ID); err != nil {
		t.Fatal("read-only source clone", err)
	}
	modRecovery := filepath.Join(primary.DataDir, ".local-container", "junimo140-group", modApplyID)
	if err := os.MkdirAll(modRecovery, 0700); err != nil {
		t.Fatal(err)
	}
	usedAPIPorts := map[string]bool{}
	for index, instance := range instances {
		project := filepath.Base(instance.DataDir)
		apiPort := ""
		for attempt := 0; attempt < 5; attempt++ {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			candidate := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
			if err := listener.Close(); err != nil {
				t.Fatal(err)
			}
			if !usedAPIPorts[candidate] {
				apiPort = candidate
				usedAPIPorts[candidate] = true
				break
			}
		}
		if apiPort == "" {
			t.Fatal("could not allocate unique loopback API ports")
		}
		if err := sjUpdateRealGroupEnv(instance.DataDir, sourceVolume, identity, apiPort); err != nil {
			t.Fatal(err)
		}
		if fullStackWeb && index == 2 && os.Getenv("ANXI_REAL_FULL_STACK_TWO_GROUPS") == "1" {
			coldSourceApplyID = newRuntimeApplyID()
			coldSourceVolume = project + "_anxi-smapi-update-" + strings.TrimPrefix(coldSourceApplyID, "apply_")
			if err := client.RuntimeCreateJunimo140Staging(ctx, root, project, coldSourceVolume, coldSourceApplyID); err != nil {
				t.Fatal(err)
			}
			if err := client.RuntimeClonePinnedJunimo140Game(ctx, root, "install-test-20260905_game-data", coldSourceVolume, targetRef, targetImage.ID); err != nil {
				t.Fatal(err)
			}
			if err := sjconfig.UpdateEnvFile(filepath.Join(instance.DataDir, ".env"), map[string]string{"GAME_DATA_VOLUME": coldSourceVolume}); err != nil {
				t.Fatal(err)
			}
			proof["coldSourceVolume"], proof["coldSourceApplyId"] = coldSourceVolume, coldSourceApplyID
		}
		if webMode || fullStackWeb {
			// Public preflight inspects the configured reference, so use the
			// exact trusted mirror tag actually cached on this fixture host.
			const cachedSource = "dockerproxy.net/sdvd/server:1.5.0-preview.125"
			metadata, err := client.RuntimeImageInspect(ctx, root, cachedSource)
			if err != nil || metadata.ID != sourceImage.ID || metadata.Digest != sourceImage.Digest {
				t.Fatal("cached Web source reference identity changed", err)
			}
			if err := sjconfig.UpdateEnvFile(filepath.Join(instance.DataDir, ".env"), map[string]string{"SERVER_IMAGE": cachedSource, "SERVER_IMAGE_CANDIDATES": cachedSource}); err != nil {
				t.Fatal(err)
			}
		}
		composeRaw, err := os.ReadFile(filepath.Join(instance.DataDir, "docker-compose.yml"))
		if err != nil {
			t.Fatal(err)
		}
		compose := strings.ReplaceAll(string(composeRaw), "\r\n", "\n")
		compose = strings.Replace(compose, "  server:\n", "  server:\n    labels:\n      anxi.task: junimo140-20261015\n      com.anxi-panel.group-real: "+identity+"\n", 1)
		for _, port := range []string{"GAME_PORT:-24642", "QUERY_PORT:-27015", "VNC_PORT:-5800", "API_PORT:-8080"} {
			compose = strings.ReplaceAll(compose, `"${`+port+`}:`, `"127.0.0.1:${`+port+`}:`)
		}
		if err := os.WriteFile(filepath.Join(instance.DataDir, "docker-compose.yml"), []byte(compose), 0600); err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256([]byte(instance.ID))
		archive := hex.EncodeToString(hash[:6]) + ".tar"
		request := paneldocker.RuntimeInstanceArchiveRequest{Project: project, InstanceHostDir: instance.DataDir, RecoveryHostDir: modRecovery, Filename: archive, ImageRef: sourceRef, ImageID: sourceImage.ID}
		if err := client.RuntimeExtractJunimo140Mod(ctx, root, request); err != nil {
			t.Fatal("extract real125 Mod", err)
		}
		extracted := filepath.Join(modRecovery, "target-mod-"+strings.TrimSuffix(archive, ".tar"))
		if err := validateExtractedJunimoServerMod(extracted, "1.5.0-preview.125"); err != nil {
			t.Fatal(err)
		}
		if _, err := replaceJunimoServerMod(instance.DataDir, extracted, filepath.Join(modRecovery, "unused-original-"+instance.ID)); err != nil {
			t.Fatal(err)
		}
		for name, raw := range map[string][]byte{"manifest.json": sourceControlManifest, "StardewAnxiPanel.Control.dll": sourceControlDLL} {
			if err := os.WriteFile(filepath.Join(smapiModDir(instance.DataDir), name), raw, 0600); err != nil {
				t.Fatal("seed fixed source Control", err)
			}
		}
		state := storage.InstanceStateSaveRequired
		if index < 2 {
			out, err := exec.CommandContext(ctx, "docker", "run", "--rm", "--pull", "never", "--network", "none", "--read-only", "--mount", "type=bind,src="+baseline+",dst=/baseline.tar,readonly", "--mount", "type=bind,src="+savesDir(instance.DataDir)+",dst=/saves", "--entrypoint", "tar", targetImage.ID, "-xpf", "/baseline.tar", "-C", "/saves").CombinedOutput()
			if err != nil {
				t.Fatalf("copy isolated frozen save: %v: %s", err, out)
			}
			entries, err := os.ReadDir(filepath.Join(savesDir(instance.DataDir), "Saves"))
			if err != nil || len(entries) != 1 || !entries[0].IsDir() {
				t.Fatal("frozen save scope is ambiguous")
			}
			if err := writeGameloaderPointer(instance.DataDir, entries[0].Name()); err != nil {
				t.Fatal(err)
			}
			if _, err := stableImportSaveDiskState(instance.DataDir, entries[0].Name()); err != nil {
				t.Fatal(err)
			}
			if err := client.RuntimeCreateJunimo140ServicePinned(ctx, instance.DataDir, project, "server", sourceImage.ID); err != nil {
				t.Fatal("create stopped125", err)
			}
			state = storage.InstanceStateStopped
		}
		if _, err := store.UpdateInstanceState(ctx, storage.UpdateInstanceStateParams{ID: instance.ID, State: state, DriverPhase: "fixture_ready", DriverPayload: `{"fixture":"shared-real"}`}); err != nil {
			t.Fatal(err)
		}
		instances[index].State = state
	}
	primary = instances[0]
	stored, err := store.ListInstances(ctx)
	if err != nil {
		t.Fatal(err)
	}
	plan, err = buildJunimo140GroupPlan(stored, primary, applyID)
	if err != nil {
		t.Fatal(err)
	}
	proof["planSha256"], proof["stageVolume"], proof["phase"] = plan.digest(), plan.StagingVolume, "starting_source"
	writeProof()
	snapshots, err := driver.snapshotJunimo140Group(ctx, client, plan)
	if err != nil {
		t.Fatal(err)
	}
	var start []paneldocker.RuntimeJunimo140ServiceSnapshot
	for _, snapshot := range snapshots {
		if snapshot.Project == primaryProject {
			start = append(start, snapshot)
		}
	}
	if len(start) != 1 {
		t.Fatal("source primary service missing")
	}
	if err := client.RuntimeStartJunimo140Services(ctx, root, start, map[string][]string{primaryProject + "/server": {sourceImage.ID}}); err != nil {
		t.Fatal(err)
	}
	manifest := runtimeUpdateRecoveryManifest{SchemaVersion: 4, Project: primaryProject, Target: RuntimeUpdateSelectedPair{Server: RuntimeUpdateSelectedImage{Image: sourceRef, ImageID: sourceImage.ID, Digest: sourceImage.Digest}}, TargetServerVersion: "1.5.0-preview.125"}
	var sourceReadyErr error
	if sourceControl == "0.3.8" {
		sourceReadyErr = sjWaitRealGroupSource038(ctx, driver, client, primary, sourceImage.ID)
	} else {
		sourceReadyErr = driver.verifyRuntimeTarget(ctx, client, primary, manifest)
	}
	if sourceReadyErr != nil {
		t.Fatal("real125 source acceptance", sourceReadyErr)
	}
	if err := driver.verifyJunimo140GroupWorld(ctx, client, primary.DataDir); err != nil {
		t.Fatal("source world ready", err)
	}
	if _, err := store.UpdateInstanceState(ctx, storage.UpdateInstanceStateParams{ID: primary.ID, State: storage.InstanceStateRunning, DriverPhase: "fixture_ready", DriverPayload: `{"fixture":"shared-real"}`}); err != nil {
		t.Fatal(err)
	}
	primary.State = storage.InstanceStateRunning
	if twoRunning {
		var peer []paneldocker.RuntimeJunimo140ServiceSnapshot
		for _, snapshot := range snapshots {
			if snapshot.Project == engine.failedProject {
				peer = append(peer, snapshot)
			}
		}
		if len(peer) != 1 {
			t.Fatal("second running source missing")
		}
		if err := client.RuntimeStartJunimo140Services(ctx, root, peer, map[string][]string{engine.failedProject + "/server": {sourceImage.ID}}); err != nil {
			t.Fatal(err)
		}
		peerManifest := manifest
		peerManifest.Project = engine.failedProject
		if err := driver.verifyRuntimeTarget(ctx, client, instances[1], peerManifest); err != nil {
			t.Fatal("second running source readiness", err)
		}
		if err := driver.verifyJunimo140GroupWorld(ctx, client, instances[1].DataDir); err != nil {
			t.Fatal("second source world", err)
		}
		if _, err := store.UpdateInstanceState(ctx, storage.UpdateInstanceStateParams{ID: instances[1].ID, State: storage.InstanceStateRunning, DriverPhase: "fixture_ready", DriverPayload: `{"fixture":"shared-real"}`}); err != nil {
			t.Fatal(err)
		}
		proof["twoRunningSources"] = true
	}
	if fullStackWeb {
		plan = sjRunFullStackActualWeb(t, ctx, driver, client, instances, proof)
		completed = true
		return
	}
	inspection := InspectManagedRuntimeStack(primary.DataDir, primary.State)
	target, err := fixedFullStackJunimo140Target()
	if err != nil {
		t.Fatal(err)
	}
	target.StackVersion = "local-test-junimo140-shared-control039"
	target.Server.Tag = "1.5.0-preview.140"
	target.Server.Images = []string{targetRef}
	target.Server.TrustedCandidates = []string{targetRef}
	target.Server.Digests = map[string]string{targetRef: junimo140Digest}
	status := RuntimeUpdateApplyStatus{ApplyID: applyID, Current: inspection.Current, Target: target, Phase: RuntimeUpdateApplyChecking, StartedAt: time.Now().UTC().Format(time.RFC3339)}
	dryRun := RuntimeUpdateDryRunStatus{Phase: RuntimeUpdatePhaseSucceeded, Target: target, SharedGroupSHA256: plan.membershipDigest(), SharedGroupMembers: len(plan.Members)}
	proof["phase"] = "applying"
	writeProof()
	var started RuntimeUpdateApplyStatus
	if webMode {
		started, plan = sjRunRealGroupViaPanelHTTP(t, ctx, repo, root, webPassword, primary, proof, &plan)
		proof["applyId"], proof["planSha256"], proof["stageVolume"] = plan.ApplyID, plan.digest(), plan.StagingVolume
	} else {
		started, err = driver.startJunimo140GroupLocked(ctx, primary, status, dryRun, dryRun.SharedGroupSHA256)
		if err != nil {
			t.Fatal("start exact confirmed real group", err)
		}
	}
	proof["jobId"] = started.JobID
	writeProof()
	for {
		job, err := store.GetJob(ctx, started.JobID)
		if err != nil {
			t.Fatal(err)
		}
		if job.Status == "succeeded" || job.Status == "failed" || job.Status == "cancelled" {
			proof["jobStatus"] = job.Status
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(time.Second):
		}
	}
	terminal, err := driver.RuntimeUpdateApplyStatus(primary)
	if err != nil {
		t.Fatal(err)
	}
	if scenario == "recovery" {
		if terminal.Phase != RuntimeUpdateApplyRollbackFailed || !engine.restoreFailed.Load() {
			t.Fatal("real partial restore fault did not retain its recovery owner")
		}
		if _, err := os.Stat(filepath.Join(instances[1].DataDir, ".env")); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("partial restore fixture did not lose env")
		}
		for _, member := range plan.Members {
			if err := rejectJunimo140GroupLease(member.DataDir); err == nil {
				t.Fatal("partial restore released peer lock")
			}
		}
		proof["phase"] = "partial_restore_recovery"
		writeProof()
		if err := driver.RecoverRuntimeUpdateApply(ctx, instances[1]); err != nil {
			t.Fatal("real peer recovery with missing env", err)
		}
		terminal, err = driver.RuntimeUpdateApplyStatus(primary)
		if err != nil {
			t.Fatal(err)
		}
	}
	wantPhase := RuntimeUpdateApplySucceeded
	wantImage, wantVolume := targetImage.ID, plan.StagingVolume
	if scenario != "success" {
		wantPhase, wantImage, wantVolume = RuntimeUpdateApplyFailedRolledBack, sourceImage.ID, sourceVolume
		if !engine.targetFailed.Load() {
			t.Fatal("real peer failure was not injected")
		}
	}
	if terminal.Phase != wantPhase {
		t.Fatalf("real group did not reach expected terminal state: phase=%s code=%s error=%v", terminal.Phase, terminal.ErrorCode, err)
	}
	snapshots, err = driver.snapshotJunimo140Group(ctx, client, plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 2 {
		t.Fatal("cold template unexpectedly created a runtime")
	}
	for _, snapshot := range snapshots {
		if snapshot.ImageID != wantImage || snapshot.GameVolume != wantVolume || snapshot.Running != ((snapshot.Project == primaryProject || twoRunning) && scenario != "recovery") {
			t.Fatal("terminal group did not retain exact image/volume/running state")
		}
	}
	transaction, err := readJunimo140GroupTransaction(plan)
	if err != nil {
		t.Fatal(err)
	}
	if twoRunning {
		if !engine.saveDelayed.Load() {
			t.Fatal("member stop did not precede delayed peer save")
		}
		select {
		case err := <-engine.saveDelayDone:
			if err != nil {
				t.Fatal(err)
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		for i := 0; i < 2; i++ {
			if !transaction.Worlds[i].Saved || !transaction.Worlds[i].Stopped {
				t.Fatal("two-world save/stop evidence missing")
			}
		}
		proof["saveStopWindowClosed"] = true
	}
	if scenario != "success" && (!transaction.StageRemoved || transaction.KeepServerStopped != (scenario == "recovery")) {
		t.Fatal("real rollback left stage or lost stopped recovery policy")
	}
	for _, member := range plan.Members {
		wantControl := sourceControlDLL
		if scenario == "success" {
			wantControl = smapiModDLL
		}
		actualControl, err := os.ReadFile(filepath.Join(smapiModDir(member.DataDir), "StardewAnxiPanel.Control.dll"))
		if err != nil || sha256.Sum256(actualControl) != sha256.Sum256(wantControl) {
			t.Fatal("terminal Control artifact did not match committed or restored version", err)
		}
		if err := rejectJunimo140GroupLease(member.DataDir); err != nil {
			t.Fatal("committed peer lease retained", err)
		}
		volume, err := GameDataVolumeName(member.DataDir)
		if err != nil || volume != wantVolume {
			t.Fatal("committed peer game binding differs")
		}
		state, err := store.GetInstance(ctx, member.ID)
		if err != nil || state.DriverPayload != `{"fixture":"shared-real"}` {
			t.Fatal("long-lived database payload changed")
		}
		wantState := storage.InstanceStateStopped
		if (member.ID == "a" || twoRunning && member.ID == "b") && scenario != "recovery" {
			wantState = storage.InstanceStateRunning
		}
		if member.ID == "c" {
			wantState = storage.InstanceStateSaveRequired
		}
		if state.State != wantState {
			t.Fatal("real group lost original runtime or cold-template state")
		}
		if member.ID == "c" {
			entries, err := os.ReadDir(filepath.Join(savesDir(member.DataDir), "Saves"))
			if err != nil || len(entries) != 0 {
				t.Fatal("cold template gained a farm")
			}
		}
	}
	if scenario != "success" {
		for i, member := range plan.Members {
			if err := validateJunimo140OriginalFiles(member, transaction.Worlds[i]); err != nil {
				t.Fatal("real rollback config digest mismatch", err)
			}
		}
	}
	proof["phase"], proof["worlds"], proof["coldFarmCreated"], proof["runningWorlds"] = transaction.Phase, 3, false, 1
	if twoRunning {
		proof["runningWorlds"] = 2
	}
	if scenario == "recovery" {
		proof["runningWorlds"] = 0
	}
	proof["coldStatePreserved"] = true
	proof["sourceRestored"] = scenario != "success"
	if engine.diagnosticErrors.Load() != 0 {
		t.Fatal("isolated target diagnostics were incomplete; retain fixture")
	}
	proof["targetDiagnosticSnapshots"] = engine.diagnosticSequence.Load()
	completed = !t.Failed()
	writeProof()
}
