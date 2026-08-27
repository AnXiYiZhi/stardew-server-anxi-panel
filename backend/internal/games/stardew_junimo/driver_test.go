package stardew_junimo

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

type fakeDocker struct {
	workDir                     string
	psResult                    paneldocker.ComposePsResult
	psErr                       error
	composePsStarted            chan struct{}
	composePsRelease            chan struct{}
	pullResult                  paneldocker.CommandResult
	pullErr                     error
	composePulls                int
	pullErrByImage              map[string]error
	pulledImages                []string
	inspectResult               paneldocker.CommandResult
	inspectErr                  error
	inspectErrByImage           map[string]error
	inspectedImages             []string
	steamAuthCode               int
	steamAuthErr                error
	steamAuthRuns               int
	steamAuthLines              []string
	steamAuthOpts               paneldocker.SteamAuthRunOpts
	steamAuthInitialInput       []string
	steamAuthWaitForCancel      bool
	steamAuthSawCancel          bool
	containerCode               int
	containerCodes              []int
	containerErr                error
	containerRuns               int
	containerLines              []string
	containerRunLines           [][]string
	containerOpts               paneldocker.ContainerTTYRunOpts
	containerRunOpts            []paneldocker.ContainerTTYRunOpts
	containerWaitForCancelOnRun int
	containerSawCancel          bool
	junimoExtractVersion        string
	authMigrateRuns             int
	authMigrateOpts             paneldocker.ContainerTTYRunOpts
	authMigrateLines            []string
	smapiRuns                   int
	smapiLines                  []string
	smapiOpts                   paneldocker.ContainerTTYRunOpts
	bundledSyncRuns             int
	bundledSyncCode             int
	bundledSyncErr              error
	bundledSyncMods             []runtimeLoadedMod
	verifyRuns                  int
	verifyCode                  int
	verifyErr                   error
	verifyLines                 []string
	verifyOpts                  paneldocker.ContainerTTYRunOpts
	removedVolumes              []string
	removedByVolumes            []string
	removeVolumesErr            error
	removeContainersErr         error
	removeContainersStarted     chan struct{}
	removeContainersRelease     chan struct{}
	stoppedServices             []string
	restartedServices           []string
	recreatedServices           []string
}

func (f *fakeDocker) RecommendedSMAPIArchive(_ context.Context, dataDir string, manifest sjconfig.RuntimeStackManifest) (string, error) {
	path := filepath.Join(dataDir, ".local-container", "smapi-update", "packages", "SMAPI-"+manifest.SMAPI.Version+"-installer.zip")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte("test-only reviewed archive"), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func (f *fakeDocker) ComposePs(ctx context.Context, dir string) (paneldocker.ComposePsResult, error) {
	f.workDir = dir
	if f.composePsStarted != nil {
		close(f.composePsStarted)
	}
	if f.composePsRelease != nil {
		<-f.composePsRelease
	}
	return f.psResult, f.psErr
}

func (f *fakeDocker) ComposePsStrict(ctx context.Context, dir string) (paneldocker.ComposePsResult, error) {
	return f.ComposePs(ctx, dir)
}

func (f *fakeDocker) ComposePullStreaming(_ context.Context, _ string, _ []string, lineHandler func(string)) (paneldocker.CommandResult, error) {
	f.composePulls++
	if f.pullErr == nil {
		lineHandler("steam-auth Pulling")
		lineHandler("server Pulling")
	}
	return f.pullResult, f.pullErr
}

func (f *fakeDocker) PullImageStreaming(_ context.Context, _ string, imageRef string, lineHandler func(string)) (paneldocker.CommandResult, error) {
	f.pulledImages = append(f.pulledImages, imageRef)
	err := f.pullErr
	if f.pullErrByImage != nil {
		if imageErr, ok := f.pullErrByImage[imageRef]; ok {
			err = imageErr
		}
	}
	if err == nil {
		lineHandler("09b89f0e06d0: Pulling fs layer")
		lineHandler("a769fcf442cd: Pulling fs layer")
		lineHandler("09b89f0e06d0: Pull complete")
		lineHandler("a769fcf442cd: Pull complete")
	}
	return f.pullResult, err
}

func (f *fakeDocker) ImageInspect(ctx context.Context, dir string, imageRef string) (paneldocker.CommandResult, error) {
	f.inspectedImages = append(f.inspectedImages, imageRef)
	if f.inspectErrByImage != nil {
		if err, ok := f.inspectErrByImage[imageRef]; ok {
			return f.inspectResult, err
		}
	}
	return f.inspectResult, f.inspectErr
}

func (f *fakeDocker) RunSteamAuthTTY(ctx context.Context, _ string, opts paneldocker.SteamAuthRunOpts, guardCh <-chan string, lineHandler func(string)) (int, error) {
	f.steamAuthRuns++
	f.steamAuthOpts = opts
	select {
	case input := <-guardCh:
		f.steamAuthInitialInput = append(f.steamAuthInitialInput, input)
	default:
	}
	for _, line := range f.steamAuthLines {
		lineHandler(line)
	}
	if f.steamAuthWaitForCancel {
		select {
		case <-ctx.Done():
			f.steamAuthSawCancel = true
			if f.steamAuthErr != nil {
				return f.steamAuthCode, f.steamAuthErr
			}
			return f.steamAuthCode, ctx.Err()
		case <-time.After(time.Second):
			return -1, errors.New("timed out waiting for auth-only runner cancellation")
		}
	}
	return f.steamAuthCode, f.steamAuthErr
}

func (f *fakeDocker) RunContainerTTY(ctx context.Context, opts paneldocker.ContainerTTYRunOpts, _ <-chan string, lineHandler func(string)) (int, error) {
	command := strings.Join(opts.Command, " ")
	if strings.Contains(command, junimoModExtractMarker) {
		f.containerRuns++
		f.containerOpts = opts
		if len(opts.Binds) == 0 || !strings.HasSuffix(opts.Binds[0], ":/out") {
			return 1, errors.New("missing Junimo extraction bind")
		}
		workDir := strings.TrimSuffix(opts.Binds[0], ":/out")
		targetDir := filepath.Join(workDir, runtimeTargetJunimoDir)
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			return 1, err
		}
		version := f.junimoExtractVersion
		if version == "" {
			version = opts.ImageRef[strings.LastIndex(opts.ImageRef, ":")+1:]
		}
		manifest := `{"Name":"JunimoServer","Version":"` + version + `","UniqueID":"JunimoHost.Server"}`
		if err := os.WriteFile(filepath.Join(targetDir, junimoServerManifestName), []byte(manifest), 0o644); err != nil {
			return 1, err
		}
		if err := os.WriteFile(filepath.Join(targetDir, junimoServerAssemblyName), []byte("test JunimoServer assembly "+version), 0o644); err != nil {
			return 1, err
		}
		lineHandler(junimoModExtractMarker)
		return 0, nil
	}
	if strings.Contains(command, "anxi-steamcmd-auth-migrate") {
		f.authMigrateRuns++
		f.authMigrateOpts = opts
		lines := f.authMigrateLines
		if len(lines) == 0 {
			lines = []string{"anxi-steamcmd-auth-migrate: no legacy cache found"}
		}
		for _, line := range lines {
			lineHandler(line)
		}
		return 0, nil
	}
	if strings.Contains(command, "anxi-install-verify") {
		f.verifyRuns++
		f.verifyOpts = opts
		for _, line := range f.verifyLines {
			lineHandler(line)
		}
		return f.verifyCode, f.verifyErr
	}
	if strings.Contains(command, smapiBundledSyncMarker) {
		f.bundledSyncRuns++
		if f.bundledSyncErr != nil || f.bundledSyncCode != 0 {
			return f.bundledSyncCode, f.bundledSyncErr
		}
		stage := ""
		for _, bind := range opts.Binds {
			if strings.HasSuffix(bind, ":/managed") {
				stage = strings.TrimSuffix(bind, ":/managed")
				break
			}
		}
		if stage == "" {
			return 1, errors.New("missing SMAPI bundled staging bind")
		}
		mods := f.bundledSyncMods
		if len(mods) == 0 {
			mods = []runtimeLoadedMod{
				{UniqueID: consoleCommandsID, Version: "4.5.2"},
				{UniqueID: saveBackupID, Version: "4.5.2"},
			}
		}
		for index, mod := range mods {
			folder := fmt.Sprintf("Bundled-%d", index+1)
			dir := filepath.Join(stage, folder)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return 1, err
			}
			manifest := fmt.Sprintf(`{"Name":%q,"UniqueID":%q,"Version":%q}`, mod.UniqueID, mod.UniqueID, mod.Version)
			if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o644); err != nil {
				return 1, err
			}
		}
		lineHandler(smapiBundledSyncMarker + ": copy complete")
		return 0, nil
	}
	if strings.Contains(command, "SMAPI") || strings.Contains(command, "smapi") {
		f.smapiRuns++
		f.smapiOpts = opts
		lines := f.smapiLines
		if len(lines) == 0 {
			lines = []string{"SMAPI already installed at /data/game/StardewModdingAPI, skipping."}
		}
		for _, line := range lines {
			lineHandler(line)
		}
		return f.containerCode, f.containerErr
	}
	f.containerRuns++
	f.containerOpts = opts
	f.containerRunOpts = append(f.containerRunOpts, opts)
	lines := f.containerLines
	if len(f.containerRunLines) > 0 {
		lines = f.containerRunLines[0]
		f.containerRunLines = f.containerRunLines[1:]
	}
	for _, line := range lines {
		lineHandler(line)
	}
	if f.containerWaitForCancelOnRun == f.containerRuns {
		select {
		case <-ctx.Done():
			f.containerSawCancel = true
			return -1, ctx.Err()
		case <-time.After(time.Second):
			return -1, errors.New("timed out waiting for SteamCMD attempt cancellation")
		}
	}
	code := f.containerCode
	if len(f.containerCodes) > 0 {
		code = f.containerCodes[0]
		f.containerCodes = f.containerCodes[1:]
	}
	return code, f.containerErr
}

func (f *fakeDocker) RemoveVolumes(_ context.Context, _ string, names []string) (paneldocker.CommandResult, error) {
	f.removedVolumes = append(f.removedVolumes, names...)
	return paneldocker.CommandResult{ExitCode: 0}, f.removeVolumesErr
}

func (f *fakeDocker) RemoveContainersByVolume(_ context.Context, _ string, names []string) (paneldocker.CommandResult, error) {
	f.removedByVolumes = append(f.removedByVolumes, names...)
	return paneldocker.CommandResult{ExitCode: 0}, f.removeContainersErr
}

func (f *fakeDocker) RemoveSteamInviteAuthSessionHolders(_ context.Context, _, _, volume string) (paneldocker.CommandResult, error) {
	f.removedByVolumes = append(f.removedByVolumes, volume)
	if f.removeContainersStarted != nil {
		close(f.removeContainersStarted)
	}
	if f.removeContainersRelease != nil {
		<-f.removeContainersRelease
	}
	return paneldocker.CommandResult{ExitCode: 0}, f.removeContainersErr
}

func (f *fakeDocker) RuntimeComposeStopServices(_ context.Context, _, _ string, services ...string) error {
	f.stoppedServices = append(f.stoppedServices, services...)
	return nil
}

func (f *fakeDocker) ComposeRestartServices(_ context.Context, _ string, services ...string) (paneldocker.CommandResult, error) {
	f.restartedServices = append(f.restartedServices, services...)
	return paneldocker.CommandResult{ExitCode: 0}, nil
}

func (f *fakeDocker) ComposeRecreateServices(_ context.Context, _ string, services ...string) (paneldocker.CommandResult, error) {
	f.recreatedServices = append(f.recreatedServices, services...)
	return paneldocker.CommandResult{ExitCode: 0}, nil
}

type fakeStore struct {
	instance storage.Instance
	getErr   error
	updated  []storage.UpdateInstanceStateParams
}

func (f *fakeStore) GetInstance(_ context.Context, _ string) (storage.Instance, error) {
	return f.instance, f.getErr
}

func (f *fakeStore) UpdateInstanceState(_ context.Context, p storage.UpdateInstanceStateParams) (storage.Instance, error) {
	f.updated = append(f.updated, p)
	f.instance.State = p.State
	f.instance.StateMessage.String = p.StateMessage
	f.instance.StateMessage.Valid = p.StateMessage != ""
	f.instance.DriverPhase = p.DriverPhase
	f.instance.DriverPayload = p.DriverPayload
	return f.instance, nil
}

func (f *fakeStore) RestoreInstanceStateSnapshot(_ context.Context, snapshot storage.Instance) (storage.Instance, error) {
	f.instance.State = snapshot.State
	f.instance.StateMessage = snapshot.StateMessage
	f.instance.DriverPhase = snapshot.DriverPhase
	f.instance.DriverPayload = snapshot.DriverPayload
	return f.instance, nil
}

func (f *fakeStore) UpdateInstanceStateForActiveJob(_ context.Context, p storage.UpdateInstanceStateForActiveJobParams) (storage.Instance, error) {
	return f.UpdateInstanceState(context.Background(), p.UpdateInstanceStateParams)
}

func TestDriverIdentity(t *testing.T) {
	driver := New(nil, nil, nil, nil)
	if driver.ID() != DriverID {
		t.Fatalf("unexpected id %q", driver.ID())
	}
	if driver.Name() != DriverName {
		t.Fatalf("unexpected name %q", driver.Name())
	}
}

func TestDriverPrepare_CreatesDirectoryAndFiles(t *testing.T) {
	driver := New(nil, nil, nil, nil)
	dataDir := filepath.Join(t.TempDir(), "stardew")

	if err := driver.Prepare(context.Background(), registry.Instance{DataDir: dataDir}); err != nil {
		t.Fatalf("prepare: %v", err)
	}

	// Main directory must exist.
	if _, err := os.Stat(dataDir); err != nil {
		t.Fatalf("data dir not created: %v", err)
	}

	// Sub-directories.
	for _, sub := range []string{
		"saves", "mods", ".local-container",
		filepath.Join(".local-container", "settings"),
		filepath.Join(".local-container", "saves"),
		filepath.Join(".local-container", "saves", "Saves"),
		filepath.Join(".local-container", "saves-templates"),
	} {
		if _, err := os.Stat(filepath.Join(dataDir, sub)); err != nil {
			t.Errorf("sub-dir %s not created: %v", sub, err)
		}
	}

	// docker-compose.yml must exist and keep Junimo's official service and volume contracts.
	compose := filepath.Join(dataDir, "docker-compose.yml")
	composeBytes, err := os.ReadFile(compose)
	if err != nil {
		t.Fatalf("docker-compose.yml not created: %v", err)
	}
	if len(composeBytes) == 0 {
		t.Error("docker-compose.yml is empty")
	}
	composeText := string(composeBytes)
	if strings.Contains(composeText, "depends_on:") {
		t.Fatal("server must not have a default hard dependency on optional steam-auth")
	}
	for _, want := range []string{
		"steam-auth:",
		"server:",
		"SERVER_IMAGE",
		"stdin_open: true",
		"tty: true",
		"steam-session:/data/steam-session",
		"game-data:/data/game",
		"${INSTANCE_HOST_DATA_DIR}/.local-container/saves:/config/xdg/config/StardewValley",
		"${INSTANCE_HOST_DATA_DIR}/.local-container/settings:/data/settings",
		"${INSTANCE_HOST_DATA_DIR}/.local-container/cont-env/APP_NAME:/etc/cont-env.d/APP_NAME:ro",
		"${INSTANCE_HOST_DATA_DIR}/.local-container/cont-env/DBUS_SESSION_BUS_ADDRESS:/etc/cont-env.d/DBUS_SESSION_BUS_ADDRESS:ro",
		"${INSTANCE_HOST_DATA_DIR}/.local-container/cont-groups/cinit/id:/etc/cont-groups.d/cinit/id:ro",
		"${INSTANCE_HOST_DATA_DIR}/.local-container/cont-users/root/home:/etc/cont-users.d/root/home:ro",
	} {
		if !strings.Contains(composeText, want) {
			t.Errorf("docker-compose.yml missing %q", want)
		}
	}

	// .env must exist and use the official IMAGE_VERSION key.
	envBytes, err := os.ReadFile(filepath.Join(dataDir, ".env"))
	if err != nil {
		t.Fatalf(".env not created: %v", err)
	}
	envText := string(envBytes)
	if !strings.Contains(envText, "IMAGE_VERSION="+TestedImageTag) {
		t.Fatalf(".env should contain IMAGE_VERSION=%s, got:\n%s", TestedImageTag, envText)
	}
	if strings.Contains(envText, "JUNIMO_IMAGE_TAG") {
		t.Fatal(".env should not contain JUNIMO_IMAGE_TAG")
	}
	for _, staticValue := range []serverStaticInitValue{
		{serverContEnvDir + "/APP_NAME", "/etc/cont-env.d/APP_NAME", "DockerApp"},
		{serverContEnvDir + "/DBUS_SESSION_BUS_ADDRESS", "/etc/cont-env.d/DBUS_SESSION_BUS_ADDRESS", "unix:path=/tmp/dbus.base"},
		{serverContGroupsDir + "/cinit/id", "/etc/cont-groups.d/cinit/id", "72"},
		{serverContUsersDir + "/root/home", "/etc/cont-users.d/root/home", "/root"},
	} {
		script, err := os.ReadFile(filepath.Join(dataDir, filepath.FromSlash(staticValue.localPath)))
		if err != nil {
			t.Fatalf("%s static init fix script not created: %v", staticValue.localPath, err)
		}
		if string(script) != serverStaticInitScript(staticValue.value) {
			t.Fatalf("unexpected %s static init script:\n%s", staticValue.localPath, script)
		}
	}
}

func TestDriverPrepare_DoesNotOverwriteExistingFiles(t *testing.T) {
	driver := New(&fakeDocker{}, nil, nil, nil)
	dataDir := t.TempDir()

	// Pre-write compose and env with custom content.
	customCompose := []byte("# custom compose\n")
	if err := os.WriteFile(filepath.Join(dataDir, "docker-compose.yml"), customCompose, 0o644); err != nil {
		t.Fatal(err)
	}
	customEnv := []byte("MY_KEY=myvalue\n")
	if err := os.WriteFile(filepath.Join(dataDir, ".env"), customEnv, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := driver.Prepare(context.Background(), registry.Instance{DataDir: dataDir}); err != nil {
		t.Fatalf("prepare: %v", err)
	}

	got, _ := os.ReadFile(filepath.Join(dataDir, "docker-compose.yml"))
	if string(got) != string(customCompose) {
		t.Error("Prepare should not overwrite existing docker-compose.yml")
	}
	gotEnv, err := sjconfig.ReadEnvFile(filepath.Join(dataDir, ".env"))
	if err != nil {
		t.Fatalf("read migrated .env: %v", err)
	}
	if gotEnv["MY_KEY"] != "myvalue" {
		t.Errorf("Prepare should preserve existing .env values, got MY_KEY=%q", gotEnv["MY_KEY"])
	}
	if gotEnv["INSTANCE_HOST_DATA_DIR"] != dataDir {
		t.Errorf("Prepare should add the Docker host data path, got %q want %q", gotEnv["INSTANCE_HOST_DATA_DIR"], dataDir)
	}
}

func TestDriverPrepareMigratesHistoricalSteamInviteCapability(t *testing.T) {
	for _, tc := range []struct {
		name    string
		state   string
		phase   string
		payload string
	}{
		{name: "historical state", state: storage.InstanceStateSteamAuthDone, phase: "stopped", payload: `{}`},
		{name: "historical phase", phase: "steam_auth_done", payload: `{}`},
		{name: "stored invite code", phase: "stopped", payload: `{"invite_code":"LEGACY-CODE"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dataDir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dataDir, "docker-compose.yml"), []byte(junimoComposeTemplate), 0o644); err != nil {
				t.Fatalf("write compose: %v", err)
			}
			if err := os.WriteFile(filepath.Join(dataDir, ".env"), []byte("STEAMCMD_AUTH_COMPLETED=true\n"), 0o600); err != nil {
				t.Fatalf("write legacy env: %v", err)
			}
			driver := New(nil, nil, nil, nil)
			if err := driver.Prepare(context.Background(), registry.Instance{
				ID:            "legacy",
				DataDir:       dataDir,
				State:         tc.state,
				DriverPhase:   tc.phase,
				DriverPayload: tc.payload,
			}); err != nil {
				t.Fatalf("Prepare: %v", err)
			}
			if !sjconfig.SteamInviteEnabled(dataDir) {
				t.Fatal("historical invite capability must migrate to explicit enabled intent")
			}
			fields, err := sjconfig.ReadEnvFile(filepath.Join(dataDir, ".env"))
			if err != nil {
				t.Fatalf("read migrated env: %v", err)
			}
			if fields["STEAM_INVITE_ENABLED"] != "true" {
				t.Fatalf("explicit intent = %q, want true", fields["STEAM_INVITE_ENABLED"])
			}
			if fields["STEAM_AUTH_COMPLETED"] != "true" || fields["STEAM_INVITE_AUTH_STATE"] != sjconfig.SteamInviteAuthStateReady {
				t.Fatalf("strong historical authorization must migrate ready: %#v", fields)
			}
			if fields["STEAMCMD_AUTH_COMPLETED"] != "true" {
				t.Fatal("migration must preserve SteamCMD cache")
			}
		})
	}
}

func TestDriverPrepareMigratesMissingEnvInviteIntentOnlyFromStrongEvidence(t *testing.T) {
	for _, tc := range []struct {
		name        string
		instance    registry.Instance
		wantEnabled string
		wantState   string
		wantAuth    string
	}{
		{name: "completed authorization", instance: registry.Instance{State: storage.InstanceStateSteamAuthDone}, wantEnabled: "true", wantState: sjconfig.SteamInviteAuthStateReady, wantAuth: "true"},
		{name: "game installed is not auth evidence", instance: registry.Instance{State: storage.InstanceStateGameInstalled}, wantEnabled: "false", wantState: sjconfig.SteamInviteAuthStateDisabled},
		{name: "save required is not auth evidence", instance: registry.Instance{State: storage.InstanceStateSaveRequired}, wantEnabled: "false", wantState: sjconfig.SteamInviteAuthStateDisabled},
		{name: "ready to start is not auth evidence", instance: registry.Instance{State: storage.InstanceStateReadyToStart}, wantEnabled: "false", wantState: sjconfig.SteamInviteAuthStateDisabled},
		{name: "starting is not auth evidence", instance: registry.Instance{State: storage.InstanceStateStarting}, wantEnabled: "false", wantState: sjconfig.SteamInviteAuthStateDisabled},
		{name: "running is not auth evidence", instance: registry.Instance{State: storage.InstanceStateRunning}, wantEnabled: "false", wantState: sjconfig.SteamInviteAuthStateDisabled},
		{name: "stopped is not auth evidence", instance: registry.Instance{State: storage.InstanceStateStopped}, wantEnabled: "false", wantState: sjconfig.SteamInviteAuthStateDisabled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dataDir := t.TempDir()
			tc.instance.ID = "legacy"
			tc.instance.DataDir = dataDir
			if err := New(nil, nil, nil, nil).Prepare(context.Background(), tc.instance); err != nil {
				t.Fatalf("Prepare: %v", err)
			}
			fields, err := sjconfig.ReadEnvFile(filepath.Join(dataDir, ".env"))
			if err != nil {
				t.Fatal(err)
			}
			if fields["STEAM_INVITE_ENABLED"] != tc.wantEnabled || fields["STEAM_INVITE_AUTH_STATE"] != tc.wantState || fields["STEAM_AUTH_COMPLETED"] != tc.wantAuth {
				t.Fatalf("missing-env legacy migration = %#v", fields)
			}
		})
	}
}

func TestDriverPrepareMigratesSteamCMDOnlyLegacyInstanceToInviteDisabled(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dataDir, "docker-compose.yml"), []byte(junimoComposeTemplate), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, ".env"), []byte("STEAMCMD_AUTH_COMPLETED=true\nSTEAM_USERNAME=legacy-user\n"), 0o600); err != nil {
		t.Fatalf("write legacy env: %v", err)
	}
	if err := New(&fakeDocker{}, nil, nil, nil).Prepare(context.Background(), registry.Instance{
		ID:          "legacy-steamcmd",
		DataDir:     dataDir,
		State:       storage.InstanceStateGameInstalled,
		DriverPhase: "ready",
	}); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	fields, err := sjconfig.ReadEnvFile(filepath.Join(dataDir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if fields["STEAM_INVITE_ENABLED"] != "false" || fields["STEAM_INVITE_AUTH_STATE"] != sjconfig.SteamInviteAuthStateDisabled {
		t.Fatalf("SteamCMD-only legacy instance must migrate invite-disabled: %#v", fields)
	}
	if fields["STEAMCMD_AUTH_COMPLETED"] != "true" || fields["STEAM_USERNAME"] != "legacy-user" {
		t.Fatalf("migration must preserve SteamCMD authorization cache and credentials: %#v", fields)
	}
}

func TestMigrateSteamAuthComposeImage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "docker-compose.yml")
	oldCompose := `services:
  steam-auth:
    image: sdvd/steam-service:${IMAGE_VERSION:-1.5.0-preview.121}
    environment:
      STEAM_KEEP_LANGUAGES: "${STEAM_KEEP_LANGUAGES:-}"
`
	if err := os.WriteFile(path, []byte(oldCompose), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, err := migrateSteamAuthComposeImage(path)
	if err != nil {
		t.Fatalf("migrateSteamAuthComposeImage: %v", err)
	}
	if !changed {
		t.Fatal("expected compose migration to report changed")
	}

	gotBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(gotBytes)
	for _, want := range []string{
		"image: ${STEAM_SERVICE_IMAGE:-" + DefaultSteamServiceImage + "}",
		"STEAM_CLIENT_CONNECT_TIMEOUT_SECONDS",
		"STEAM_CLIENT_CONNECT_RETRIES",
		"STEAM_AUTH_SESSION_RETRIES",
		"STEAM_AUTH_SESSION_RETRY_DELAY_SECONDS",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("migrated compose missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "image: sdvd/steam-service:") {
		t.Fatalf("old steam-service image was not removed:\n%s", got)
	}
}

func TestMigrateOptionalSteamAuthComposeImageHonorsIntent(t *testing.T) {
	const oldCompose = "services:\n  steam-auth:\n    image: sdvd/steam-service:${IMAGE_VERSION:-1.5.0-preview.121}\n    environment:\n      STEAM_KEEP_LANGUAGES: \"${STEAM_KEEP_LANGUAGES:-}\"\n"
	for _, tc := range []struct {
		name     string
		enabled  bool
		changed  bool
		preserve bool
	}{
		{name: "disabled", preserve: true},
		{name: "enabled", enabled: true, changed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "docker-compose.yml")
			if err := os.WriteFile(path, []byte(oldCompose), 0o644); err != nil {
				t.Fatal(err)
			}
			changed, err := migrateOptionalSteamAuthComposeImage(path, tc.enabled)
			if err != nil || changed != tc.changed {
				t.Fatalf("changed=%v err=%v", changed, err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			got := string(data)
			if tc.preserve && got != oldCompose {
				t.Fatalf("disabled migration changed Auth compose section:\nbefore=%q\nafter=%q", oldCompose, got)
			}
			if tc.enabled && !strings.Contains(got, "image: ${STEAM_SERVICE_IMAGE:-"+DefaultSteamServiceImage+"}") {
				t.Fatalf("enabled migration did not maintain Auth image:\n%s", got)
			}
		})
	}
}

func TestMigrateServerComposeImage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "docker-compose.yml")
	oldCompose := `services:
  server:
    image: sdvd/server:${IMAGE_VERSION:-1.5.0-preview.121}
`
	if err := os.WriteFile(path, []byte(oldCompose), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, err := migrateServerComposeImage(path)
	if err != nil {
		t.Fatalf("migrateServerComposeImage: %v", err)
	}
	if !changed {
		t.Fatal("expected server compose migration to report changed")
	}

	gotBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(gotBytes)
	if !strings.Contains(got, "image: ${SERVER_IMAGE:-"+DefaultServerImage+"}") {
		t.Fatalf("migrated compose missing SERVER_IMAGE fallback:\n%s", got)
	}
	if strings.Contains(got, "image: sdvd/server:") {
		t.Fatalf("old server image was not removed:\n%s", got)
	}
}

func TestSteamAuthMenusAreClassifiedSeparately(t *testing.T) {
	authMenuLines := []string{
		"Choose authentication method:",
		"  [1] Username & Password",
		"  [2] QR Code (Steam Mobile App)",
	}
	for _, line := range authMenuLines {
		lower := strings.ToLower(line)
		if !isSteamAuthMethodMenu(lower) {
			t.Fatalf("expected auth method menu line to match: %q", line)
		}
		if isSteamGuardChoiceMenu(lower) {
			t.Fatalf("auth method menu line should not match Steam Guard choice: %q", line)
		}
	}

	guardMenuLines := []string{
		"║ Steam Guard Authentication ║",
		"║ [1] Approve in Steam Mobile App (recommended) ║",
		"║ [2] Enter code from Steam Mobile App or Email ║",
	}
	for _, line := range guardMenuLines {
		lower := strings.ToLower(line)
		if !isSteamGuardChoiceMenu(lower) {
			t.Fatalf("expected Steam Guard choice line to match: %q", line)
		}
		if isSteamAuthMethodMenu(lower) {
			t.Fatalf("Steam Guard choice line should not match auth method menu: %q", line)
		}
	}
}

func TestSteamAuthCommandKeepsInviteCredentialLoginNonInteractiveAndDownloadFree(t *testing.T) {
	tests := []struct {
		name     string
		mode     steamAuthMode
		authOnly bool
		want     []string
	}{
		{name: "invite credentials", mode: steamAuthModeCredentials, authOnly: true, want: []string{"serve"}},
		{name: "invite QR", mode: steamAuthModeQR, authOnly: true, want: []string{"login"}},
		{name: "legacy download credentials", mode: steamAuthModeCredentials, want: []string{"download"}},
		{name: "legacy download QR", mode: steamAuthModeQR, want: []string{"setup"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := steamAuthCommand(test.mode, test.authOnly); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("steamAuthCommand(%q, %v) = %#v, want %#v", test.mode, test.authOnly, got, test.want)
			}
		})
	}
}

func TestSteamGuardCodePromptMatchesEmailPrompt(t *testing.T) {
	line := "Enter Steam Guard code sent to qq.com:"
	if !isSteamGuardCodePrompt(strings.ToLower(line)) {
		t.Fatalf("expected Steam Guard email code prompt to match: %q", line)
	}
}

func TestSteamCMDGuardCodePromptMatchesSplitEmailPrompt(t *testing.T) {
	lines := []string{
		"This computer has not been authenticated for your account using Steam Guard.",
		"Please check your email for the message from Steam, and enter the Steam Guard",
		"code from that message.",
		"You can also enter this code at any time using 'set_steam_guard_code'",
	}
	for _, line := range lines {
		if !isSteamCMDGuardCodePrompt(strings.ToLower(line)) {
			t.Fatalf("expected SteamCMD split email code prompt to match: %q", line)
		}
	}
}

func TestQRCodeChoiceEchoDoesNotLookLikeMobileApproval(t *testing.T) {
	line := "Choice [1]: 2"
	if isSteamMobileApprovalPrompt(strings.ToLower(line)) {
		t.Fatalf("QR choice echo must not be classified as Steam Guard mobile approval: %q", line)
	}
}

func TestSteamMobileApprovalPromptMatchesApprovalText(t *testing.T) {
	lines := []string{
		"Waiting for approval in the Steam app...",
		"Open Steam app to approve this login.",
		"Approve in Steam Mobile App",
	}
	for _, line := range lines {
		if !isSteamMobileApprovalPrompt(strings.ToLower(line)) {
			t.Fatalf("expected mobile approval prompt to match: %q", line)
		}
	}
}

func TestSteamCMDMobileApprovalPromptMatchesConfirmationText(t *testing.T) {
	lines := []string{
		"Please confirm the login in the Steam Mobile app on your phone.",
		"Waiting for confirmation...",
	}
	for _, line := range lines {
		if !isSteamCMDMobileApprovalPrompt(strings.ToLower(line)) {
			t.Fatalf("expected SteamCMD mobile approval prompt to match: %q", line)
		}
	}
}

func TestDriverStatusUsesInstanceDataDir(t *testing.T) {
	fake := &fakeDocker{
		psResult: paneldocker.ComposePsResult{
			Services: []paneldocker.ComposeService{{Name: "demo", Service: "server", State: "running"}},
		},
	}
	driver := New(fake, nil, nil, nil)
	status, err := driver.Status(context.Background(), registry.Instance{
		ID:          "stardew",
		DriverID:    DriverID,
		Name:        "Stardew Valley",
		DataDir:     "custom-dir",
		State:       "running",
		DriverPhase: "empty",
	})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if fake.workDir != "custom-dir" {
		t.Fatalf("expected custom-dir workdir, got %q", fake.workDir)
	}
	if status.Runtime == nil || len(status.Runtime.Containers) != 1 {
		t.Fatalf("unexpected status: %#v", status)
	}
	if status.Runtime.Containers[0].Service != "server" {
		t.Fatalf("unexpected service: %q", status.Runtime.Containers[0].Service)
	}
}

func TestDriverReconcileStatePromotesStoppedWhenServerIsRunning(t *testing.T) {
	dataDir, expectedControl := setupControlRuntimeGateTest(t)
	writeControlRuntimeOptions(t, dataDir, readyControlRuntimeOptions(expectedControl))
	fake := &fakeDocker{
		psResult: paneldocker.ComposePsResult{
			Services: []paneldocker.ComposeService{{Service: "server", State: "running", Status: "Up 1 minute"}},
		},
	}
	store := &fakeStore{instance: storage.Instance{
		ID:            "stardew",
		DataDir:       dataDir,
		State:         storage.InstanceStateStopped,
		DriverPayload: `{"invite_code":"ABCD1234"}`,
	}}
	driver := New(fake, nil, nil, store)

	updated, err := driver.ReconcileState(context.Background(), store.instance)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if updated.State != storage.InstanceStateRunning {
		t.Fatalf("expected running, got %q", updated.State)
	}
	if len(store.updated) != 1 {
		t.Fatalf("expected one state update, got %d", len(store.updated))
	}
	if got := store.updated[0].DriverPayload; got != `{"invite_code":"ABCD1234"}` {
		t.Fatalf("driver payload was not preserved: %s", got)
	}
	if fake.workDir != dataDir {
		t.Fatalf("expected %q workdir, got %q", dataDir, fake.workDir)
	}
}

func TestDriverReconcileStateDoesNotPromoteRunningContainerWithoutControlRuntime(t *testing.T) {
	dataDir, _ := setupControlRuntimeGateTest(t)
	fake := &fakeDocker{psResult: paneldocker.ComposePsResult{Services: []paneldocker.ComposeService{{
		Service: "server", State: "running", Status: "Up 1 minute",
	}}}}
	store := &fakeStore{instance: storage.Instance{
		ID: "stardew", DataDir: dataDir, State: storage.InstanceStateStopped,
	}}
	driver := New(fake, nil, nil, store)

	updated, err := driver.ReconcileState(context.Background(), store.instance)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if updated.State != storage.InstanceStateStopped || len(store.updated) != 0 {
		t.Fatalf("pending Control must not promote state: updated=%+v writes=%d", updated, len(store.updated))
	}
}

func TestDriverReconcileStateStopsExpiredOrphanedStartingRuntime(t *testing.T) {
	dataDir, _ := setupControlRuntimeGateTest(t)
	var running atomic.Bool
	running.Store(true)
	var downs atomic.Int32
	fake := &fakeConsoleDocker{
		composePsFunc: func(context.Context, string) (paneldocker.ComposePsResult, error) {
			if !running.Load() {
				return paneldocker.ComposePsResult{}, nil
			}
			return paneldocker.ComposePsResult{Services: []paneldocker.ComposeService{{
				Service: "server", State: "running", Status: "Up 30 minutes",
			}}}, nil
		},
		composeDownFunc: func(context.Context, string) (paneldocker.CommandResult, error) {
			downs.Add(1)
			running.Store(false)
			return paneldocker.CommandResult{ExitCode: 0}, nil
		},
	}
	store := &fakeStore{instance: storage.Instance{
		ID: "stardew", DataDir: dataDir, State: storage.InstanceStateStarting,
		DriverPhase: "control_runtime_starting", DriverPayload: `{"kept":true}`,
		UpdatedAt: time.Now().UTC().Add(-readyStateTimeout - time.Minute).Format(time.RFC3339Nano),
	}}
	driver := New(fake, slog.Default(), nil, store)

	updated, err := driver.ReconcileState(context.Background(), store.instance)
	if err != nil {
		t.Fatalf("reconcile expired orphan: %v", err)
	}
	if downs.Load() != 1 {
		t.Fatalf("ComposeDown calls = %d, want 1", downs.Load())
	}
	if updated.State != storage.InstanceStateStopped || updated.DriverPhase != "control_runtime_orphan_stopped" {
		t.Fatalf("expired orphan final state = %+v", updated)
	}
	if len(store.updated) != 1 || store.updated[0].DriverPayload != `{"kept":true}` {
		t.Fatalf("unexpected orphan reconciliation writes = %#v", store.updated)
	}
}

func TestDriverReconcileStateKeepsFreshOrphanedStartingRuntimeWithinBudget(t *testing.T) {
	dataDir, _ := setupControlRuntimeGateTest(t)
	var downs atomic.Int32
	fake := &fakeConsoleDocker{
		composePsFunc: func(context.Context, string) (paneldocker.ComposePsResult, error) {
			return paneldocker.ComposePsResult{Services: []paneldocker.ComposeService{{
				Service: "server", State: "running", Status: "Up 1 minute",
			}}}, nil
		},
		composeDownFunc: func(context.Context, string) (paneldocker.CommandResult, error) {
			downs.Add(1)
			return paneldocker.CommandResult{ExitCode: 0}, nil
		},
	}
	store := &fakeStore{instance: storage.Instance{
		ID: "stardew", DataDir: dataDir, State: storage.InstanceStateStarting,
		DriverPhase: "control_runtime_starting", UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}}
	driver := New(fake, slog.Default(), nil, store)

	updated, err := driver.ReconcileState(context.Background(), store.instance)
	if err != nil {
		t.Fatal(err)
	}
	if downs.Load() != 0 || len(store.updated) != 0 || updated.State != storage.InstanceStateStarting {
		t.Fatalf("fresh orphan changed early: updated=%+v writes=%d downs=%d", updated, len(store.updated), downs.Load())
	}
}

func TestDriverReconcileStateMarksOrphanCleanupFailureWithoutClaimingStopped(t *testing.T) {
	dataDir, _ := setupControlRuntimeGateTest(t)
	fake := &fakeConsoleDocker{
		composePsFunc: func(context.Context, string) (paneldocker.ComposePsResult, error) {
			return paneldocker.ComposePsResult{Services: []paneldocker.ComposeService{{
				Service: "server", State: "running", Status: "Up 30 minutes",
			}}}, nil
		},
		composeDownFunc: func(context.Context, string) (paneldocker.CommandResult, error) {
			return paneldocker.CommandResult{ExitCode: 1, Stderr: "injected"}, errors.New("injected down failure")
		},
	}
	store := &fakeStore{instance: storage.Instance{
		ID: "stardew", DataDir: dataDir, State: storage.InstanceStateStarting,
		DriverPhase: "control_runtime_starting",
		UpdatedAt:   time.Now().UTC().Add(-readyStateTimeout - time.Minute).Format(time.RFC3339Nano),
	}}
	driver := New(fake, slog.Default(), nil, store)

	updated, err := driver.ReconcileState(context.Background(), store.instance)
	if err != nil {
		t.Fatal(err)
	}
	if updated.State != storage.InstanceStateError || updated.DriverPhase != "control_runtime_orphan_cleanup_failed" {
		t.Fatalf("cleanup failure was misreported: %+v", updated)
	}
}

func TestDriverReconcileStateDoesNotPromoteExplicitControlMismatch(t *testing.T) {
	dataDir, _ := setupControlRuntimeGateTest(t)
	writeControlRuntimeOptions(t, dataDir, `{"controlModVersion":"0.2.2"}`)
	fake := &fakeDocker{psResult: paneldocker.ComposePsResult{Services: []paneldocker.ComposeService{{
		Service: "server", State: "running", Status: "Up 1 minute",
	}}}}
	store := &fakeStore{instance: storage.Instance{
		ID: "stardew", DataDir: dataDir, State: storage.InstanceStateError,
	}}
	driver := New(fake, nil, nil, store)

	updated, err := driver.ReconcileState(context.Background(), store.instance)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if updated.State != storage.InstanceStateError || len(store.updated) != 0 {
		t.Fatalf("mismatched Control must not promote state: updated=%+v writes=%d", updated, len(store.updated))
	}
}

func TestDriverReconcileStateDoesNotOverrideActiveLifecycleOwner(t *testing.T) {
	dataDir, expectedControl := setupControlRuntimeGateTest(t)
	writeControlRuntimeOptions(t, dataDir, readyControlRuntimeOptions(expectedControl))
	store := newLifecycleTestStore(t)
	instance, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{
		ID: storage.DefaultInstanceID, DriverID: DriverID, Name: "Stardew", DataDir: dataDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	instance, err = store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID: instance.ID, State: storage.InstanceStateStarting, DriverPhase: "control_runtime_starting", DriverPayload: "{}",
	})
	if err != nil {
		t.Fatal(err)
	}
	manager := jobs.NewManager(store, slog.Default())
	release := make(chan struct{})
	job, err := manager.Start(context.Background(), jobs.Spec{
		Type: lifecycleJobType, TargetType: "instance", TargetID: instance.ID,
		Run: func(ctx context.Context, _ *jobs.Context) error {
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		close(release)
		waitForDriverTestJobStatus(t, store, job.ID, storage.JobStatusSucceeded)
	})
	fake := &fakeDocker{psResult: paneldocker.ComposePsResult{Services: []paneldocker.ComposeService{{
		Service: "server", State: "running", Status: "Up 1 minute",
	}}}}
	driver := New(fake, slog.Default(), manager, store)

	updated, err := driver.ReconcileState(context.Background(), instance)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if updated.State != storage.InstanceStateStarting || updated.DriverPhase != "control_runtime_starting" {
		t.Fatalf("active lifecycle owner was overwritten: %+v", updated)
	}
}

func TestDriverReconcileStateDoesNotPromoteWithoutServerService(t *testing.T) {
	fake := &fakeDocker{
		psResult: paneldocker.ComposePsResult{
			Services: []paneldocker.ComposeService{{Service: "steam-auth", State: "running", Status: "Up 1 minute"}},
		},
	}
	store := &fakeStore{instance: storage.Instance{
		ID:      "stardew",
		DataDir: "custom-dir",
		State:   storage.InstanceStateStopped,
	}}
	driver := New(fake, nil, nil, store)

	updated, err := driver.ReconcileState(context.Background(), store.instance)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if updated.State != storage.InstanceStateStopped {
		t.Fatalf("expected stopped, got %q", updated.State)
	}
	if len(store.updated) != 0 {
		t.Fatalf("expected no state update, got %d", len(store.updated))
	}
}

func TestDriverReconcileStateDemotesPersistedRunningWhenServerIsAbsent(t *testing.T) {
	fake := &fakeDocker{psResult: paneldocker.ComposePsResult{Services: []paneldocker.ComposeService{{
		Service: "steam-auth", State: "running", Status: "Up 1 minute",
	}}}}
	store := &fakeStore{instance: storage.Instance{
		ID: "stardew", DataDir: t.TempDir(), State: storage.InstanceStateRunning, DriverPayload: `{"kept":true}`,
	}}
	driver := New(fake, nil, nil, store)

	updated, err := driver.ReconcileState(context.Background(), store.instance)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if updated.State != storage.InstanceStateStopped || updated.DriverPhase != "container_stopped" {
		t.Fatalf("absent server must remain stopped after host restart: %+v", updated)
	}
	if len(store.updated) != 1 || store.updated[0].DriverPayload != `{"kept":true}` {
		t.Fatalf("unexpected reconcile writes: %+v", store.updated)
	}
}

func TestDriverInstall_ReturnsErrorWithoutJobManager(t *testing.T) {
	driver := New(nil, nil, nil, nil)
	_, err := driver.Install(context.Background(), registry.InstallRequest{
		SteamUsername: "user",
		SteamPassword: "pass",
		VNCPassword:   "vnc",
	})
	if err == nil {
		t.Fatal("expected error without job manager")
	}
}

func TestDriverInstall_ValidatesEmptyCredentials(t *testing.T) {
	driver := New(nil, nil, nil, &fakeStore{})
	cases := []struct {
		name string
		req  registry.InstallRequest
	}{
		{"empty username", registry.InstallRequest{SteamPassword: "p", VNCPassword: "v"}},
		{"empty password", registry.InstallRequest{SteamUsername: "u", VNCPassword: "v"}},
		{"empty vnc", registry.InstallRequest{SteamUsername: "u", SteamPassword: "p"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := driver.Install(context.Background(), tc.req)
			if err == nil {
				t.Fatalf("expected validation error for %q", tc.name)
			}
		})
	}
}

func TestDriverInstallSteamCMDErrorDoesNotBecomeSteamAuthFailure(t *testing.T) {
	dataDir := t.TempDir()
	store, err := storage.Open(context.Background(), config.Config{
		DataDir: dataDir,
		DBPath:  filepath.Join(dataDir, "panel.db"),
	})
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate storage: %v", err)
	}

	instanceDir := filepath.Join(dataDir, "instances", storage.DefaultInstanceID)
	instance, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{
		ID:       storage.DefaultInstanceID,
		DriverID: storage.DefaultDriverID,
		Name:     "Stardew Valley",
		DataDir:  instanceDir,
	})
	if err != nil {
		t.Fatalf("ensure instance: %v", err)
	}

	manager := jobs.NewManager(store, slog.Default())
	fake := &fakeDocker{steamAuthErr: errors.New("steam-auth must not run"), containerErr: errors.New("SteamCMD container failed")}
	driver := New(fake, slog.Default(), manager, store)
	job, err := driver.Install(context.Background(), registry.InstallRequest{
		Instance:      registry.Instance{ID: instance.ID},
		SteamUsername: "steam-user",
		SteamPassword: "steam-pass",
		VNCPassword:   "vnc-pass",
		AutoDownload:  true,
	})
	if err != nil {
		t.Fatalf("install: %v", err)
	}

	waitForDriverTestJobStatus(t, store, job.ID, storage.JobStatusFailed)
	updated, err := store.GetInstance(context.Background(), instance.ID)
	if err != nil {
		t.Fatalf("get instance: %v", err)
	}
	if updated.State != storage.InstanceStateError || updated.DriverPhase != "steamcmd_failed" {
		t.Fatalf("base SteamCMD failure must stay in install classification: state=%s phase=%s", updated.State, updated.DriverPhase)
	}
	if fake.steamAuthRuns != 0 {
		t.Fatalf("default install must not run steam-auth, ran %d times", fake.steamAuthRuns)
	}
}

func TestDriverInstallUsesSteamCMDPrimaryWithoutSteamAuth(t *testing.T) {
	dataDir := t.TempDir()
	store, err := storage.Open(context.Background(), config.Config{
		DataDir: dataDir,
		DBPath:  filepath.Join(dataDir, "panel.db"),
	})
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate storage: %v", err)
	}

	instanceDir := filepath.Join(dataDir, "instances", storage.DefaultInstanceID)
	instance, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{
		ID:       storage.DefaultInstanceID,
		DriverID: storage.DefaultDriverID,
		Name:     "Stardew Valley",
		DataDir:  instanceDir,
	})
	if err != nil {
		t.Fatalf("ensure instance: %v", err)
	}
	if err := os.MkdirAll(instanceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	authConfig := map[string]string{
		"STEAM_INVITE_ENABLED":                   "false",
		"STEAM_INVITE_AUTH_STATE":                sjconfig.SteamInviteAuthStateDisabled,
		"STEAM_SERVICE_IMAGE":                    "legacy.invalid/auth:keep",
		"STEAM_SERVICE_IMAGE_CANDIDATES":         "legacy.invalid/auth:keep",
		"STEAM_CLIENT_CONNECT_TIMEOUT_SECONDS":   "91",
		"STEAM_CLIENT_CONNECT_RETRIES":           "92",
		"STEAM_AUTH_SESSION_RETRIES":             "93",
		"STEAM_AUTH_SESSION_RETRY_DELAY_SECONDS": "94",
	}
	if err := sjconfig.UpdateEnvFile(filepath.Join(instanceDir, ".env"), authConfig); err != nil {
		t.Fatal(err)
	}

	manager := jobs.NewManager(store, slog.Default())
	fake := &fakeDocker{
		steamAuthErr: errors.New("steam-auth must not run during base install"),
		containerLines: []string{
			"Logging in user steam-user",
			"Success! App '413150' fully installed.",
			"Success! App '1007' fully installed.",
		},
	}
	driver := New(fake, slog.Default(), manager, store)
	job, err := driver.Install(context.Background(), registry.InstallRequest{
		Instance:      registry.Instance{ID: instance.ID},
		SteamUsername: "steam-user",
		SteamPassword: "steam-pass",
		VNCPassword:   "vnc-pass",
		AutoDownload:  true,
	})
	if err != nil {
		t.Fatalf("install: %v", err)
	}

	waitForDriverTestJobStatus(t, store, job.ID, storage.JobStatusSucceeded)
	if fake.steamAuthRuns != 0 {
		t.Fatalf("default install must not run steam-auth, ran %d times", fake.steamAuthRuns)
	}
	updated, err := store.GetInstance(context.Background(), instance.ID)
	if err != nil {
		t.Fatalf("get instance: %v", err)
	}
	if updated.State != storage.InstanceStateGameInstalled || updated.DriverPhase != "game_installed" {
		t.Fatalf("instance state should be installed after steamcmd primary flow: state=%s phase=%s", updated.State, updated.DriverPhase)
	}
	wantImage := steamCMDImageRefs(nil)[0]
	if fake.containerOpts.ImageRef != wantImage {
		t.Fatalf("expected SteamCMD primary image %q, got %q", wantImage, fake.containerOpts.ImageRef)
	}
	fields, err := sjconfig.ReadEnvFile(filepath.Join(instanceDir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range authConfig {
		if fields[key] != want {
			t.Fatalf("disabled base install changed optional Auth config %s: got %q want %q", key, fields[key], want)
		}
	}
	command := strings.Join(fake.containerOpts.Command, " ")
	if !strings.Contains(command, "app_update 413150") {
		t.Fatalf("SteamCMD command should download Stardew app 413150: %#v", fake.containerOpts.Command)
	}
	if !strings.Contains(command, "app_update 1007") {
		t.Fatalf("SteamCMD command should download Steam SDK app 1007: %#v", fake.containerOpts.Command)
	}
	if strings.Contains(command, "app_update 413150 validate +force_install_dir") {
		t.Fatalf("SteamCMD should run app 413150 and app 1007 in separate sessions so force_install_dir is before login, command=%q", command)
	}
	if !strings.Contains(command, "HOME=/home/steam USER=steam LOGNAME=steam") {
		t.Fatalf("SteamCMD should run as steam user with HOME=/home/steam, command=%q", command)
	}
	if !stringSliceContains(fake.containerOpts.Binds, storage.DefaultInstanceID+"_steamcmd-login:/root/.local/share/Steam") {
		t.Fatalf("SteamCMD should persist root login cache in the canonical authorization volume, binds=%v", fake.containerOpts.Binds)
	}
	if !stringSliceContains(fake.containerOpts.Binds, storage.DefaultInstanceID+"_steamcmd-login:/home/steam/.local/share/Steam") {
		t.Fatalf("SteamCMD should persist steam-user login cache in the canonical authorization volume, binds=%v", fake.containerOpts.Binds)
	}
}

func TestDriverRejectsBaseInstallForceReauthWithoutSideEffects(t *testing.T) {
	dataDir := t.TempDir()
	store, err := storage.Open(context.Background(), config.Config{
		DataDir: dataDir,
		DBPath:  filepath.Join(dataDir, "panel.db"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	instanceDir := filepath.Join(dataDir, "instances", storage.DefaultInstanceID)
	instance, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{
		ID: storage.DefaultInstanceID, DriverID: storage.DefaultDriverID, Name: "Stardew Valley", DataDir: instanceDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(instanceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := sjconfig.UpdateEnvFile(filepath.Join(instanceDir, ".env"), map[string]string{
		"STEAM_USERNAME":          "old-user",
		"STEAM_PASSWORD":          "old-pass",
		"VNC_PASSWORD":            "vnc-pass",
		"STEAMCMD_AUTH_COMPLETED": "true",
		"STEAM_INVITE_ENABLED":    "true",
		"STEAM_REFRESH_TOKEN":     "existing-auth-token",
		"STEAM_AUTH_COMPLETED":    "true",
		"STEAM_INVITE_AUTH_STATE": sjconfig.SteamInviteAuthStateReady,
	}); err != nil {
		t.Fatal(err)
	}

	beforeInstance, err := store.GetInstance(context.Background(), instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	envPath := filepath.Join(instanceDir, ".env")
	beforeEnv, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatal(err)
	}

	manager := jobs.NewManager(store, slog.Default())
	fake := &fakeDocker{}
	driver := New(fake, slog.Default(), manager, store)
	job, err := driver.Install(context.Background(), registry.InstallRequest{
		Instance: registry.Instance{ID: instance.ID}, SteamUsername: "new-user", SteamPassword: "new-pass",
		VNCPassword: "vnc-pass", AutoDownload: false, ForceReauth: true,
	})
	if job != nil || !errors.Is(err, ErrBaseInstallForceReauthUnsupported) {
		t.Fatalf("base ForceReauth = job:%#v err:%v, want nil/%v", job, err, ErrBaseInstallForceReauthUnsupported)
	}
	if fake.composePulls != 0 || fake.steamAuthRuns != 0 || fake.containerRuns != 0 {
		t.Fatalf("rejected base ForceReauth touched Docker: pulls=%d auth=%d containers=%d", fake.composePulls, fake.steamAuthRuns, fake.containerRuns)
	}
	afterEnv, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(afterEnv, beforeEnv) {
		t.Fatalf("rejected base ForceReauth changed .env:\n%s", afterEnv)
	}
	afterInstance, err := store.GetInstance(context.Background(), instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(afterInstance, beforeInstance) {
		t.Fatalf("rejected base ForceReauth changed instance:\nbefore=%#v\nafter=%#v", beforeInstance, afterInstance)
	}
	listed, err := store.ListJobs(context.Background(), storage.ListJobsFilter{IsAdmin: true, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 0 {
		t.Fatalf("rejected base ForceReauth created %d jobs", len(listed))
	}
}

func TestIsSteamAuthLoginSuccessLineRequiresSteamAuthPrefix(t *testing.T) {
	cases := []struct {
		line string
		want bool
	}{
		{"[SteamAuth:A0] Logged in as [U:1:1231122837]", true},
		{"[SteamService] A0: Logged in as 76561199191388565", true},
		{"[SteamAuth:A0] Logging in as 1517468252 with token (498 chars)...", false},
		{"[SteamAuth:A0] Download failed: Download manifest failed across all CDN servers", false},
		{"Logged in as steam-user", false},
		{"[steamcmd] Waiting for user info...OK", false},
	}
	for _, tc := range cases {
		if got := isSteamAuthLoginSuccessLine(strings.ToLower(tc.line)); got != tc.want {
			t.Fatalf("isSteamAuthLoginSuccessLine(%q) = %v, want %v", tc.line, got, tc.want)
		}
	}
}

func writeAuthOnlyComposeFixture(t *testing.T, instanceDir string) {
	t.Helper()
	if err := os.MkdirAll(instanceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(instanceDir, "docker-compose.yml"), []byte(junimoComposeTemplate), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newInstalledAuthOnlyFixture(t *testing.T) (*storage.Store, storage.Instance, string) {
	t.Helper()
	dataDir := t.TempDir()
	store, err := storage.Open(context.Background(), config.Config{
		DataDir: dataDir,
		DBPath:  filepath.Join(dataDir, "panel.db"),
	})
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate storage: %v", err)
	}
	instanceDir := filepath.Join(dataDir, "instances", storage.DefaultInstanceID)
	instance, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{
		ID: storage.DefaultInstanceID, DriverID: storage.DefaultDriverID, Name: "Stardew Valley", DataDir: instanceDir,
	})
	if err != nil {
		t.Fatalf("ensure instance: %v", err)
	}
	instance, err = store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID: instance.ID, State: storage.InstanceStateReadyToStart, StateMessage: "ready fixture",
		DriverPhase: "ready_to_start", DriverPayload: `{"install":"complete"}`,
	})
	if err != nil {
		t.Fatalf("seed installed state: %v", err)
	}
	writeAuthOnlyComposeFixture(t, instanceDir)
	return store, instance, instanceDir
}

func startAuthOnlyJob(t *testing.T, driver *Driver, instance storage.Instance) *registry.Job {
	t.Helper()
	job, err := driver.Install(context.Background(), registry.InstallRequest{
		Instance: registry.Instance{ID: instance.ID}, SteamUsername: "steam-user", SteamPassword: "steam-pass",
		VNCPassword: "vnc-pass", AuthLoginOnly: true, ForceReauth: true,
	})
	if err != nil {
		t.Fatalf("start authorization: %v", err)
	}
	return job
}

func TestDriverAuthLoginOnlyDoesNotRequireVNCPassword(t *testing.T) {
	store, instance, instanceDir := newInstalledAuthOnlyFixture(t)
	manager := jobs.NewManager(store, slog.Default())
	fake := &fakeDocker{steamAuthLines: []string{
		"[SteamAuth:A0] Authenticated as steam-user",
		"[SteamAuth:A0] Logged in as [U:1:1231122837]",
	}}
	driver := New(fake, slog.Default(), manager, store)
	job, err := driver.Install(context.Background(), registry.InstallRequest{
		Instance:      registry.Instance{ID: instance.ID},
		SteamUsername: "steam-user",
		SteamPassword: "steam-pass",
		AuthLoginOnly: true,
	})
	if err != nil {
		t.Fatalf("start authorization without VNC password: %v", err)
	}
	waitForDriverTestPhase(t, store, instance.ID, "auth_method_required")
	if err := driver.SendSteamGuardInput(job.ID, "2"); err != nil {
		t.Fatalf("select QR authorization: %v", err)
	}
	waitForDriverTestJobStatus(t, store, job.ID, storage.JobStatusSucceeded)
	if !sjconfig.SteamInviteEnabled(instanceDir) || !sjconfig.SteamAuthLoggedIn(instanceDir) {
		t.Fatal("authorization without VNC password did not persist ready invite capability")
	}
}

func TestDriverAuthLoginOnlyJobCreationFailureDoesNotEnableIntent(t *testing.T) {
	store, instance, instanceDir := newInstalledAuthOnlyFixture(t)
	manager := jobs.NewManager(store, slog.Default())
	driver := New(&fakeDocker{}, slog.Default(), manager, store)
	job, err := driver.Install(context.Background(), registry.InstallRequest{
		Instance:      registry.Instance{ID: instance.ID},
		ActorID:       9223372036854775807, // invalid foreign-key owner forces durable job creation to fail
		SteamUsername: "steam-user",
		SteamPassword: "steam-pass",
		AuthLoginOnly: true,
	})
	if err == nil || job != nil {
		t.Fatalf("job creation failure = job:%#v err:%v, want nil/non-nil", job, err)
	}
	if sjconfig.SteamInviteEnabled(instanceDir) || sjconfig.SteamInviteAuthState(instanceDir) != sjconfig.SteamInviteAuthStateDisabled {
		t.Fatalf("failed job creation changed invite intent: enabled=%v state=%s", sjconfig.SteamInviteEnabled(instanceDir), sjconfig.SteamInviteAuthState(instanceDir))
	}
}

func TestDriverAuthLoginOnlyAlreadyReadyDoesNotCreateJobOrTouchSession(t *testing.T) {
	store, instance, instanceDir := newInstalledAuthOnlyFixture(t)
	if err := sjconfig.UpdateEnvFile(filepath.Join(instanceDir, ".env"), map[string]string{
		"STEAM_USERNAME":          "steam-user",
		"STEAM_PASSWORD":          "steam-pass",
		"VNC_PASSWORD":            "vnc-pass",
		"STEAM_INVITE_ENABLED":    "true",
		"STEAM_AUTH_COMPLETED":    "true",
		"STEAM_INVITE_AUTH_STATE": sjconfig.SteamInviteAuthStateReady,
	}); err != nil {
		t.Fatal(err)
	}

	fake := &fakeDocker{}
	driver := New(fake, slog.Default(), jobs.NewManager(store, slog.Default()), store)
	job, err := driver.Install(context.Background(), registry.InstallRequest{
		Instance: registry.Instance{ID: instance.ID}, SteamUsername: "steam-user", SteamPassword: "steam-pass",
		VNCPassword: "vnc-pass", AuthLoginOnly: true, ForceReauth: true,
	})
	if job != nil || !errors.Is(err, ErrSteamInviteAlreadyAuthorized) {
		t.Fatalf("already-ready authorization = job:%#v err:%v, want nil/%v", job, err, ErrSteamInviteAlreadyAuthorized)
	}
	listed, listErr := store.ListJobs(context.Background(), storage.ListJobsFilter{IsAdmin: true, Limit: 100})
	if listErr != nil {
		t.Fatal(listErr)
	}
	if len(listed) != 0 || fake.steamAuthRuns != 0 || len(fake.removedByVolumes) != 0 || len(fake.removedVolumes) != 0 {
		t.Fatalf("already-ready authorization caused side effects: jobs=%d authRuns=%d holders=%v volumes=%v", len(listed), fake.steamAuthRuns, fake.removedByVolumes, fake.removedVolumes)
	}
	if !sjconfig.SteamAuthLoggedIn(instanceDir) || sjconfig.SteamInviteAuthState(instanceDir) != sjconfig.SteamInviteAuthStateReady {
		t.Fatal("already-ready authorization changed the saved SteamAuth session")
	}
}

func TestDriverAuthLoginOnlyUsesDedicatedJobAndPreservesInstallState(t *testing.T) {
	dataDir := t.TempDir()
	store, err := storage.Open(context.Background(), config.Config{
		DataDir: dataDir,
		DBPath:  filepath.Join(dataDir, "panel.db"),
	})
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate storage: %v", err)
	}

	instanceDir := filepath.Join(dataDir, "instances", storage.DefaultInstanceID)
	instance, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{
		ID:       storage.DefaultInstanceID,
		DriverID: storage.DefaultDriverID,
		Name:     "Stardew Valley",
		DataDir:  instanceDir,
	})
	if err != nil {
		t.Fatalf("ensure instance: %v", err)
	}
	writeAuthOnlyComposeFixture(t, instanceDir)

	manager := jobs.NewManager(store, slog.Default())
	fake := &fakeDocker{
		steamAuthWaitForCancel: true,
		steamAuthLines: []string{
			"[SteamService] HTTP API listening on port 3001",
			"[SteamService] 1 account(s) configured",
			"[SteamAuth:A0] Connecting... (1/5) +0.0s",
			"[SteamAuth:A0] Connected to Steam +4.7s",
			"[SteamAuth:A0] Logging in as 1517468252 with token (498 chars)... +0.0s",
			"[SteamAuth:A0] Token expires: 2027-02-02 11:30:38 UTC (210 days remaining) +0.0s",
			"[SteamAuth:A0] Logged in as [U:1:1231122837] +0.7s",
		},
	}
	driver := New(fake, slog.Default(), manager, store)
	job, err := driver.Install(context.Background(), registry.InstallRequest{
		Instance:      registry.Instance{ID: instance.ID},
		SteamUsername: "steam-user",
		SteamPassword: "steam-pass",
		VNCPassword:   "vnc-pass",
		AuthLoginOnly: true,
		ForceReauth:   true,
	})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	waitForDriverTestPhase(t, store, instance.ID, "auth_method_required")
	if err := driver.SendSteamGuardInput(job.ID, "1"); err != nil {
		t.Fatalf("select credential authorization: %v", err)
	}
	waitForDriverTestJobStatus(t, store, job.ID, storage.JobStatusSucceeded)
	storedJob, err := store.GetJob(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if storedJob.Type != "stardew_steam_auth" {
		t.Fatalf("auth-only job type = %q, want stardew_steam_auth", storedJob.Type)
	}
	if !storedJob.Payload.Valid || !strings.Contains(storedJob.Payload.String, `"schemaVersion":1`) || !strings.Contains(storedJob.Payload.String, `"stateMessageValid":`) {
		t.Fatalf("auth-only recovery payload is missing stable snapshot fields: %#v", storedJob.Payload)
	}
	if strings.Contains(storedJob.Payload.String, "steam-pass") || strings.Contains(storedJob.Payload.String, "steam-user") {
		t.Fatalf("auth-only recovery payload must not contain Steam credentials: %s", storedJob.Payload.String)
	}

	if !sjconfig.SteamAuthLoggedIn(instanceDir) {
		t.Fatal("expected STEAM_AUTH_COMPLETED=true after real steam-auth logged-in line")
	}
	if len(fake.restartedServices) != 0 {
		t.Fatalf("auth-only authorization must not materialize runtime services, got %#v", fake.restartedServices)
	}
	if fake.containerRuns != 0 || fake.smapiRuns != 0 {
		t.Fatalf("auth-only must not run SteamCMD/SMAPI, containerRuns=%d smapiRuns=%d", fake.containerRuns, fake.smapiRuns)
	}
	wantSessionVolume := storage.DefaultInstanceID + "_steam-session"
	if !reflect.DeepEqual(fake.removedByVolumes, []string{wantSessionVolume}) || !reflect.DeepEqual(fake.removedVolumes, []string{wantSessionVolume}) {
		t.Fatalf("force reauth must remove only the exact Auth session holder and volume: holders=%v volumes=%v", fake.removedByVolumes, fake.removedVolumes)
	}
	for _, name := range append(append([]string{}, fake.removedByVolumes...), fake.removedVolumes...) {
		if strings.Contains(name, "game-data") || strings.Contains(name, "steamcmd-") {
			t.Fatalf("force reauth crossed the Auth session cleanup boundary: %v", name)
		}
	}
	if !reflect.DeepEqual(fake.steamAuthOpts.Command, []string{"serve"}) {
		t.Fatalf("credential auth-only command = %#v, want non-downloading serve", fake.steamAuthOpts.Command)
	}
	if len(fake.steamAuthInitialInput) != 0 {
		t.Fatalf("credential auth-only must not pre-send setup menu input: %#v", fake.steamAuthInitialInput)
	}
	if !fake.steamAuthSawCancel {
		t.Fatal("credential auth-only must stop the one-off service after login success")
	}
	for _, bind := range fake.steamAuthOpts.Binds {
		if strings.Contains(bind, "game-data") || strings.HasSuffix(bind, ":/data/game") {
			t.Fatalf("auth-only authorization must not mount persistent game data: %#v", fake.steamAuthOpts.Binds)
		}
	}
	if !stringSliceContains(fake.steamAuthOpts.Env, "GAME_DIR=/tmp/anxi-steam-invite-game") {
		t.Fatalf("auth-only GAME_DIR must be container-local scratch: %#v", fake.steamAuthOpts.Env)
	}
	updated, err := store.GetInstance(context.Background(), instance.ID)
	if err != nil {
		t.Fatalf("get instance: %v", err)
	}
	if updated.State != instance.State || updated.DriverPhase != instance.DriverPhase {
		t.Fatalf("auth-only must preserve installation state, got state=%s phase=%s; want state=%s phase=%s", updated.State, updated.DriverPhase, instance.State, instance.DriverPhase)
	}
}

func TestDriverAuthLoginOnlyPreservesSuccessfulSessionAfterContainerCleanupError(t *testing.T) {
	store, instance, instanceDir := newInstalledAuthOnlyFixture(t)
	manager := jobs.NewManager(store, slog.Default())
	cleanupErr := errors.New("injected exact-container cleanup failure")
	fake := &fakeDocker{
		steamAuthWaitForCancel: true,
		steamAuthErr:           errors.Join(context.Canceled, cleanupErr),
		steamAuthLines:         []string{"[SteamAuth:A0] Logged in as [U:1:1231122837]"},
	}
	driver := New(fake, slog.Default(), manager, store)
	job := startAuthOnlyJob(t, driver, instance)
	waitForDriverTestPhase(t, store, instance.ID, "auth_method_required")
	if err := driver.SendSteamGuardInput(job.ID, "1"); err != nil {
		t.Fatalf("select credentials: %v", err)
	}
	waitForDriverTestJobStatus(t, store, job.ID, storage.JobStatusFailed)

	if !fake.steamAuthSawCancel {
		t.Fatal("credential serve was not canceled after the semantic success line")
	}
	if !sjconfig.SteamAuthLoggedIn(instanceDir) || sjconfig.SteamInviteAuthState(instanceDir) != sjconfig.SteamInviteAuthStateCleanupPending {
		t.Fatalf("real login success was discarded after container cleanup failure: loggedIn=%v state=%s", sjconfig.SteamAuthLoggedIn(instanceDir), sjconfig.SteamInviteAuthState(instanceDir))
	}
	if len(fake.removedVolumes) != 1 {
		t.Fatalf("post-success cleanup must preserve the newly written session volume: removals=%v", fake.removedVolumes)
	}
	updated, err := store.GetInstance(context.Background(), instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.State != instance.State || updated.StateMessage != instance.StateMessage || updated.DriverPhase != instance.DriverPhase || updated.DriverPayload != instance.DriverPayload {
		t.Fatalf("cleanup failure changed base snapshot: got %#v want %#v", updated, instance)
	}
	if second, err := driver.Install(context.Background(), registry.InstallRequest{
		Instance: registry.Instance{ID: instance.ID}, SteamUsername: "steam-user", SteamPassword: "steam-pass",
		VNCPassword: "vnc-pass", AuthLoginOnly: true, ForceReauth: true,
	}); second != nil || !errors.Is(err, ErrSteamInviteCleanupPending) {
		t.Fatalf("second authorization during unresolved cleanup = job %#v, err %v", second, err)
	}
}

func TestDriverAuthLoginOnlySessionCleanupFailsClosed(t *testing.T) {
	store, instance, instanceDir := newInstalledAuthOnlyFixture(t)
	if err := sjconfig.UpdateEnvFile(filepath.Join(instanceDir, ".env"), map[string]string{"STEAM_REFRESH_TOKEN": "preserve-existing-session-token"}); err != nil {
		t.Fatal(err)
	}
	manager := jobs.NewManager(store, slog.Default())
	fake := &fakeDocker{
		removeContainersErr:     errors.New("injected holder cleanup failure"),
		removeContainersStarted: make(chan struct{}),
		removeContainersRelease: make(chan struct{}),
	}
	driver := New(fake, slog.Default(), manager, store)
	job := startAuthOnlyJob(t, driver, instance)
	select {
	case <-fake.removeContainersStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("authorization did not reach safe holder classification")
	}
	if err := sjconfig.SetSteamAuthLoggedIn(instanceDir, true); err != nil {
		t.Fatal(err)
	}
	close(fake.removeContainersRelease)
	waitForDriverTestJobStatus(t, store, job.ID, storage.JobStatusFailed)

	if fake.steamAuthRuns != 0 {
		t.Fatalf("steam-auth ran despite failed session cleanup: %d", fake.steamAuthRuns)
	}
	if len(fake.removedVolumes) != 0 {
		t.Fatalf("volume removal continued after holder cleanup failure: %v", fake.removedVolumes)
	}
	if !sjconfig.SteamAuthLoggedIn(instanceDir) || sjconfig.SteamInviteAuthState(instanceDir) != sjconfig.SteamInviteAuthStateReady {
		t.Fatalf("failed holder classification invalidated the existing authorization: loggedIn=%v state=%s", sjconfig.SteamAuthLoggedIn(instanceDir), sjconfig.SteamInviteAuthState(instanceDir))
	}
	fields, err := sjconfig.ReadEnvFile(filepath.Join(instanceDir, ".env"))
	if err != nil || fields["STEAM_REFRESH_TOKEN"] != "preserve-existing-session-token" {
		t.Fatalf("failed holder classification changed the existing session payload: token=%q err=%v", fields["STEAM_REFRESH_TOKEN"], err)
	}
	updated, err := store.GetInstance(context.Background(), instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.State != instance.State || updated.DriverPhase != instance.DriverPhase || updated.DriverPayload != instance.DriverPayload {
		t.Fatalf("session cleanup failure changed base snapshot: got %#v want %#v", updated, instance)
	}
}

func TestDriverAuthLoginOnlyPreservesReadySessionWithoutRevalidatingPrepareFiles(t *testing.T) {
	store, instance, instanceDir := newInstalledAuthOnlyFixture(t)
	if err := os.WriteFile(filepath.Join(instanceDir, ".env"), []byte(
		"STEAM_INVITE_ENABLED=true\nSTEAM_AUTH_COMPLETED=true\nSTEAM_INVITE_AUTH_STATE=ready\n",
	), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(instanceDir, "docker-compose.yml")); err != nil {
		t.Fatal(err)
	}
	manager := jobs.NewManager(store, slog.Default())
	fake := &fakeDocker{}
	driver := New(fake, slog.Default(), manager, store)
	job, err := driver.Install(context.Background(), registry.InstallRequest{
		Instance: registry.Instance{ID: instance.ID}, SteamUsername: "steam-user", SteamPassword: "steam-pass",
		VNCPassword: "vnc-pass", AuthLoginOnly: true, ForceReauth: true,
	})
	if job != nil || !errors.Is(err, ErrSteamInviteAlreadyAuthorized) {
		t.Fatalf("already-ready authorization = job:%#v err:%v, want nil/%v", job, err, ErrSteamInviteAlreadyAuthorized)
	}

	if fake.steamAuthRuns != 0 {
		t.Fatalf("already-ready session unexpectedly ran steam-auth: %d", fake.steamAuthRuns)
	}
	if !sjconfig.SteamAuthLoggedIn(instanceDir) || sjconfig.SteamInviteAuthState(instanceDir) != sjconfig.SteamInviteAuthStateReady {
		t.Fatalf("ready session was invalidated by missing prepare files: loggedIn=%v state=%s", sjconfig.SteamAuthLoggedIn(instanceDir), sjconfig.SteamInviteAuthState(instanceDir))
	}
	updated, err := store.GetInstance(context.Background(), instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.State != instance.State || updated.DriverPhase != instance.DriverPhase || updated.DriverPayload != instance.DriverPayload {
		t.Fatalf("ready-session no-op changed base snapshot: got %#v want %#v", updated, instance)
	}
}

func TestDriverAuthLoginOnlyQRUsesOneShotLoginAndSharedAccount(t *testing.T) {
	store, instance, instanceDir := newInstalledAuthOnlyFixture(t)
	manager := jobs.NewManager(store, slog.Default())
	fake := &fakeDocker{steamAuthLines: []string{
		"[SteamAuth:A0] Authenticated as steam-user",
		"[SteamAuth:A0] Logged in as [U:1:1231122837]",
	}}
	driver := New(fake, slog.Default(), manager, store)
	job := startAuthOnlyJob(t, driver, instance)
	waitForDriverTestPhase(t, store, instance.ID, "auth_method_required")
	if err := driver.SendSteamGuardInput(job.ID, "2"); err != nil {
		t.Fatalf("select QR login: %v", err)
	}
	waitForDriverTestJobStatus(t, store, job.ID, storage.JobStatusSucceeded)

	if !sjconfig.SteamAuthLoggedIn(instanceDir) {
		t.Fatal("matching QR account did not persist Steam Auth completion")
	}
	if !reflect.DeepEqual(fake.steamAuthOpts.Command, []string{"login"}) {
		t.Fatalf("QR command=%v want one-shot login", fake.steamAuthOpts.Command)
	}
	if !reflect.DeepEqual(fake.steamAuthInitialInput, []string{"2\n"}) {
		t.Fatalf("QR initial input=%q want exact pre-buffered menu choice", fake.steamAuthInitialInput)
	}
	if fake.steamAuthSawCancel {
		t.Fatal("successful one-shot QR login should exit normally")
	}
	for _, bind := range fake.steamAuthOpts.Binds {
		if strings.Contains(bind, "game-data") || strings.HasSuffix(bind, ":/data/game") {
			t.Fatalf("QR authorization mounted persistent game data: %v", fake.steamAuthOpts.Binds)
		}
	}
}

func TestDriverAuthLoginOnlyQRTerminalFailureStopsOneShotContainer(t *testing.T) {
	store, instance, instanceDir := newInstalledAuthOnlyFixture(t)
	manager := jobs.NewManager(store, slog.Default())
	fake := &fakeDocker{
		steamAuthWaitForCancel: true,
		steamAuthLines:         []string{"[SteamAuth:A0] QR authentication failed"},
	}
	driver := New(fake, slog.Default(), manager, store)
	job := startAuthOnlyJob(t, driver, instance)
	waitForDriverTestPhase(t, store, instance.ID, "auth_method_required")
	if err := driver.SendSteamGuardInput(job.ID, "2"); err != nil {
		t.Fatalf("select QR login: %v", err)
	}
	waitForDriverTestJobStatus(t, store, job.ID, storage.JobStatusFailed)

	if !fake.steamAuthSawCancel {
		t.Fatal("terminal QR failure did not stop the one-shot authorization container")
	}
	if sjconfig.SteamAuthLoggedIn(instanceDir) || sjconfig.SteamInviteAuthState(instanceDir) != sjconfig.SteamInviteAuthStateFailed {
		t.Fatalf("failed QR authorization remained ready: loggedIn=%v state=%s", sjconfig.SteamAuthLoggedIn(instanceDir), sjconfig.SteamInviteAuthState(instanceDir))
	}
	updated, err := store.GetInstance(context.Background(), instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.State != instance.State || updated.DriverPhase != instance.DriverPhase || updated.DriverPayload != instance.DriverPayload {
		t.Fatalf("QR failure changed base snapshot: got %#v want %#v", updated, instance)
	}
}

func TestDriverAuthLoginOnlyQRRejectsDifferentSharedAccount(t *testing.T) {
	store, instance, instanceDir := newInstalledAuthOnlyFixture(t)
	manager := jobs.NewManager(store, slog.Default())
	fake := &fakeDocker{
		steamAuthWaitForCancel: true,
		steamAuthLines:         []string{"[SteamAuth:A0] Authenticated as another-user"},
		steamAuthErr:           errors.Join(context.Canceled, errors.New("injected one-off cleanup failure")),
	}
	driver := New(fake, slog.Default(), manager, store)
	job := startAuthOnlyJob(t, driver, instance)
	waitForDriverTestPhase(t, store, instance.ID, "auth_method_required")
	if err := driver.SendSteamGuardInput(job.ID, "2"); err != nil {
		t.Fatalf("select QR login: %v", err)
	}
	waitForDriverTestJobStatus(t, store, job.ID, storage.JobStatusFailed)

	if !fake.steamAuthSawCancel {
		t.Fatal("mismatched QR login was not canceled")
	}
	if sjconfig.SteamAuthLoggedIn(instanceDir) || sjconfig.SteamInviteAuthState(instanceDir) != sjconfig.SteamInviteAuthStateFailed {
		t.Fatalf("mismatched QR account remained ready: loggedIn=%v state=%s", sjconfig.SteamAuthLoggedIn(instanceDir), sjconfig.SteamInviteAuthState(instanceDir))
	}
	wantSession := storage.DefaultInstanceID + "_steam-session"
	if !reflect.DeepEqual(fake.removedByVolumes, []string{wantSession, wantSession}) || !reflect.DeepEqual(fake.removedVolumes, []string{wantSession, wantSession}) {
		t.Fatalf("mismatched QR session was not rejected exactly: holders=%v volumes=%v", fake.removedByVolumes, fake.removedVolumes)
	}
	updated, err := store.GetInstance(context.Background(), instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.State != instance.State || updated.DriverPhase != instance.DriverPhase || updated.DriverPayload != instance.DriverPayload {
		t.Fatalf("QR mismatch changed base snapshot: got %#v want %#v", updated, instance)
	}
}

func TestDriverAuthLoginOnlyExit139PreservesBaseStateAndSteamCMDCache(t *testing.T) {
	dataDir := t.TempDir()
	store, err := storage.Open(context.Background(), config.Config{
		DataDir: dataDir,
		DBPath:  filepath.Join(dataDir, "panel.db"),
	})
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate storage: %v", err)
	}

	instanceDir := filepath.Join(dataDir, "instances", storage.DefaultInstanceID)
	instance, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{
		ID: storage.DefaultInstanceID, DriverID: storage.DefaultDriverID, Name: "Stardew Valley", DataDir: instanceDir,
	})
	if err != nil {
		t.Fatalf("ensure instance: %v", err)
	}
	instance, err = store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID: instance.ID, State: storage.InstanceStateReadyToStart, StateMessage: "ready fixture",
		DriverPhase: "ready_to_start", DriverPayload: `{"save_strategy":"existing"}`,
	})
	if err != nil {
		t.Fatalf("seed installed state: %v", err)
	}
	writeAuthOnlyComposeFixture(t, instanceDir)
	if err := os.WriteFile(filepath.Join(instanceDir, ".env"), []byte("STEAMCMD_AUTH_COMPLETED=true\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	manager := jobs.NewManager(store, slog.Default())
	fake := &fakeDocker{steamAuthCode: 139, steamAuthLines: []string{"Authentication failed: interactive console unavailable"}}
	driver := New(fake, slog.Default(), manager, store)
	job, err := driver.Install(context.Background(), registry.InstallRequest{
		Instance: registry.Instance{ID: instance.ID}, SteamUsername: "steam-user", SteamPassword: "steam-pass",
		VNCPassword: "vnc-pass", AuthLoginOnly: true,
	})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	waitForDriverTestPhase(t, store, instance.ID, "auth_method_required")
	if err := driver.SendSteamGuardInput(job.ID, "1"); err != nil {
		t.Fatalf("select credential authorization: %v", err)
	}
	waitForDriverTestJobStatus(t, store, job.ID, storage.JobStatusFailed)

	updated, err := store.GetInstance(context.Background(), instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.State != instance.State || updated.StateMessage != instance.StateMessage || updated.DriverPhase != instance.DriverPhase || updated.DriverPayload != instance.DriverPayload {
		t.Fatalf("failed invite authorization changed base snapshot: got %#v want %#v", updated, instance)
	}
	if !sjconfig.SteamInviteEnabled(instanceDir) || sjconfig.SteamInviteAuthState(instanceDir) != sjconfig.SteamInviteAuthStateFailed {
		t.Fatalf("invite intent/state = enabled:%v auth:%s, want enabled/failed", sjconfig.SteamInviteEnabled(instanceDir), sjconfig.SteamInviteAuthState(instanceDir))
	}
	fields, err := sjconfig.ReadEnvFile(filepath.Join(instanceDir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if fields["STEAMCMD_AUTH_COMPLETED"] != "true" {
		t.Fatalf("SteamCMD authorization cache flag changed: %#v", fields)
	}
	if fake.containerRuns != 0 || fake.smapiRuns != 0 || len(fake.removedVolumes) != 0 {
		t.Fatalf("failed invite auth touched install path/cache: container=%d smapi=%d removed=%v", fake.containerRuns, fake.smapiRuns, fake.removedVolumes)
	}
}

func TestDriverAuthLoginOnlyImagePullFailurePreservesInstalledSnapshot(t *testing.T) {
	dataDir := t.TempDir()
	store, err := storage.Open(context.Background(), config.Config{
		DataDir: dataDir,
		DBPath:  filepath.Join(dataDir, "panel.db"),
	})
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate storage: %v", err)
	}

	instanceDir := filepath.Join(dataDir, "instances", storage.DefaultInstanceID)
	instance, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{
		ID: storage.DefaultInstanceID, DriverID: storage.DefaultDriverID, Name: "Stardew Valley", DataDir: instanceDir,
	})
	if err != nil {
		t.Fatalf("ensure instance: %v", err)
	}
	instance, err = store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID: instance.ID, State: storage.InstanceStateGameInstalled, StateMessage: "installed fixture",
		DriverPhase: "game_installed", DriverPayload: `{"install":"complete"}`,
	})
	if err != nil {
		t.Fatalf("seed installed state: %v", err)
	}
	writeAuthOnlyComposeFixture(t, instanceDir)
	if err := os.WriteFile(filepath.Join(instanceDir, ".env"), []byte("STEAMCMD_AUTH_COMPLETED=true\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	manager := jobs.NewManager(store, slog.Default())
	fake := &fakeDocker{
		inspectErr: errors.New("steam-auth image missing"),
		pullErr:    errors.New("registry unavailable"),
	}
	driver := New(fake, slog.Default(), manager, store)
	job, err := driver.Install(context.Background(), registry.InstallRequest{
		Instance: registry.Instance{ID: instance.ID}, SteamUsername: "steam-user", SteamPassword: "steam-pass",
		VNCPassword: "vnc-pass", AuthLoginOnly: true, ForceReauth: true,
	})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	waitForDriverTestJobStatus(t, store, job.ID, storage.JobStatusFailed)

	updated, err := store.GetInstance(context.Background(), instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.State != instance.State || updated.StateMessage != instance.StateMessage || updated.DriverPhase != instance.DriverPhase || updated.DriverPayload != instance.DriverPayload {
		t.Fatalf("Auth image pull failure changed base snapshot: got %#v want %#v", updated, instance)
	}
	if !sjconfig.SteamInviteEnabled(instanceDir) || sjconfig.SteamInviteAuthState(instanceDir) != sjconfig.SteamInviteAuthStateFailed {
		t.Fatalf("invite intent/state = enabled:%v auth:%s, want enabled/failed", sjconfig.SteamInviteEnabled(instanceDir), sjconfig.SteamInviteAuthState(instanceDir))
	}
	fields, err := sjconfig.ReadEnvFile(filepath.Join(instanceDir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if fields["STEAMCMD_AUTH_COMPLETED"] != "true" {
		t.Fatalf("SteamCMD authorization cache flag changed: %#v", fields)
	}
	wantSessionVolume := storage.DefaultInstanceID + "_steam-session"
	if !reflect.DeepEqual(fake.removedByVolumes, []string{wantSessionVolume}) || !reflect.DeepEqual(fake.removedVolumes, []string{wantSessionVolume}) {
		t.Fatalf("force reauth must clean only the Auth session before pull: holders=%v volumes=%v", fake.removedByVolumes, fake.removedVolumes)
	}
	if fake.steamAuthRuns != 0 || fake.containerRuns != 0 || fake.smapiRuns != 0 {
		t.Fatalf("Auth image pull failure must not run Auth or install steps: auth=%d container=%d smapi=%d", fake.steamAuthRuns, fake.containerRuns, fake.smapiRuns)
	}
}

func TestDriverReconcileStateMarksInstalledStateInvalidWhenGameVolumeFilesAreMissing(t *testing.T) {
	instance := storage.Instance{
		ID:          "stardew",
		DataDir:     t.TempDir(),
		State:       storage.InstanceStateGameInstalled,
		DriverPhase: "game_installed",
	}
	if err := os.WriteFile(filepath.Join(instance.DataDir, ".env"), []byte("SERVER_IMAGE=example.test/junimo:latest\n"), 0o600); err != nil {
		t.Fatalf("write env: %v", err)
	}
	store := &fakeStore{instance: instance}
	fake := &fakeDocker{verifyCode: installVerificationMissingExitCode}
	driver := New(fake, slog.Default(), nil, store)

	updated, err := driver.ReconcileState(context.Background(), instance)
	if err != nil {
		t.Fatalf("reconcile state: %v", err)
	}
	if updated.State != storage.InstanceStateError || updated.DriverPhase != "install_verification_failed" {
		t.Fatalf("missing runtime files should invalidate installed state, got state=%s phase=%s", updated.State, updated.DriverPhase)
	}
	if fake.verifyRuns != 1 {
		t.Fatalf("expected one game-volume verification, got %d", fake.verifyRuns)
	}
}

func TestDriverInstallFailsWhenRequiredGameRuntimeFilesAreMissing(t *testing.T) {
	dataDir := t.TempDir()
	store, err := storage.Open(context.Background(), config.Config{DataDir: dataDir, DBPath: filepath.Join(dataDir, "panel.db")})
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate storage: %v", err)
	}
	instance, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{
		ID: storage.DefaultInstanceID, DriverID: storage.DefaultDriverID, Name: "Stardew Valley", DataDir: filepath.Join(dataDir, "instances", storage.DefaultInstanceID),
	})
	if err != nil {
		t.Fatalf("ensure instance: %v", err)
	}
	manager := jobs.NewManager(store, slog.Default())
	fake := &fakeDocker{
		steamAuthLines: []string{
			"[SteamAuth:A0] Logged in as [U:1:123]",
			"[SteamAuth:A0] Downloading app 413150...",
			"[SteamService] Game download failed: CDN error",
		},
		containerLines: []string{
			"Logging in user steam-user",
			"Success! App '413150' fully installed.",
			"Success! App '1007' fully installed.",
		},
		verifyCode: installVerificationMissingExitCode,
	}
	driver := New(fake, slog.Default(), manager, store)
	job, err := driver.Install(context.Background(), registry.InstallRequest{
		Instance: registry.Instance{ID: instance.ID}, SteamUsername: "steam-user", SteamPassword: "steam-pass", VNCPassword: "vnc-pass", AutoDownload: true,
	})
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	waitForDriverTestJobStatus(t, store, job.ID, storage.JobStatusFailed)
	updated, err := store.GetInstance(context.Background(), instance.ID)
	if err != nil {
		t.Fatalf("get instance: %v", err)
	}
	if updated.State != storage.InstanceStateError || updated.DriverPhase != "install_verification_failed" {
		t.Fatalf("missing runtime files should fail installation, got state=%s phase=%s", updated.State, updated.DriverPhase)
	}
	verifyCommand := strings.Join(fake.verifyOpts.Command, " ")
	for _, required := range []string{
		"StardewValley",
		"Stardew Valley.dll",
		"appmanifest_413150.acf",
		"StardewModdingAPI",
		"StardewModdingAPI.dll",
		"Mods/ConsoleCommands/manifest.json",
		"Mods/SaveBackup/manifest.json",
		"appmanifest_1007.acf",
		"steamclient.so",
	} {
		if !strings.Contains(verifyCommand, required) {
			t.Fatalf("verification command must require %q, got %q", required, verifyCommand)
		}
	}
}

func TestDriverInstallRetriesSteamCMDAfterSegfaultPreservesAuthorizationVolumes(t *testing.T) {
	dataDir := t.TempDir()
	store, err := storage.Open(context.Background(), config.Config{
		DataDir: dataDir,
		DBPath:  filepath.Join(dataDir, "panel.db"),
	})
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate storage: %v", err)
	}

	instanceDir := filepath.Join(dataDir, "instances", storage.DefaultInstanceID)
	instance, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{
		ID:       storage.DefaultInstanceID,
		DriverID: storage.DefaultDriverID,
		Name:     "Stardew Valley",
		DataDir:  instanceDir,
	})
	if err != nil {
		t.Fatalf("ensure instance: %v", err)
	}

	manager := jobs.NewManager(store, slog.Default())
	fake := &fakeDocker{
		steamAuthLines: []string{
			"[SteamAuth:A0] Logged in as [U:1:1231122837]",
			"[SteamAuth:A0] Downloading app 413150...",
			"[SteamService] Game download failed: Download manifest failed across all CDN servers (403 Forbidden)",
		},
		containerCodes: []int{139, 0},
		containerLines: []string{
			"Logging in user steam-user",
			"Success! App '413150' fully installed.",
			"Success! App '1007' fully installed.",
		},
	}
	driver := New(fake, slog.Default(), manager, store)
	job, err := driver.Install(context.Background(), registry.InstallRequest{
		Instance:      registry.Instance{ID: instance.ID},
		SteamUsername: "steam-user",
		SteamPassword: "steam-pass",
		VNCPassword:   "vnc-pass",
		AutoDownload:  true,
	})
	if err != nil {
		t.Fatalf("install: %v", err)
	}

	waitForDriverTestJobStatus(t, store, job.ID, storage.JobStatusSucceeded)
	if fake.containerRuns != 2 {
		t.Fatalf("expected SteamCMD to retry once after exit 139, ran %d times", fake.containerRuns)
	}
	wantVolumes := []string{
		storage.DefaultInstanceID + "_steamcmd-login",
		storage.DefaultInstanceID + "_steamcmd-home",
	}
	for _, name := range wantVolumes {
		if !stringSliceContains(fake.removedByVolumes, name) {
			t.Fatalf("expected stale SteamCMD containers using volume %q to be removed, got %v", name, fake.removedByVolumes)
		}
		if stringSliceContains(fake.removedVolumes, name) {
			t.Fatalf("SteamCMD authorization volume %q must be preserved after exit 139, removed=%v", name, fake.removedVolumes)
		}
	}
}

func TestDriverInstallTriesNextSteamCMDImageCandidateAfterPullFailure(t *testing.T) {
	dataDir := t.TempDir()
	store, err := storage.Open(context.Background(), config.Config{
		DataDir: dataDir,
		DBPath:  filepath.Join(dataDir, "panel.db"),
	})
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate storage: %v", err)
	}

	instanceDir := filepath.Join(dataDir, "instances", storage.DefaultInstanceID)
	instance, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{
		ID:       storage.DefaultInstanceID,
		DriverID: storage.DefaultDriverID,
		Name:     "Stardew Valley",
		DataDir:  instanceDir,
	})
	if err != nil {
		t.Fatalf("ensure instance: %v", err)
	}

	firstImage := "dockerproxy.net/steamcmd/steamcmd:latest"
	secondImage := "docker.1ms.run/steamcmd/steamcmd:latest"
	manager := jobs.NewManager(store, slog.Default())
	fake := &fakeDocker{
		inspectErrByImage: map[string]error{
			firstImage:  errors.New("missing steamcmd image"),
			secondImage: errors.New("missing steamcmd image"),
			"docker.1panel.live/steamcmd/steamcmd:latest": errors.New("missing steamcmd image"),
			"docker.jiaxin.site/steamcmd/steamcmd:latest": errors.New("missing steamcmd image"),
			"dockerproxy.link/steamcmd/steamcmd:latest":   errors.New("missing steamcmd image"),
			"cm2network/steamcmd:latest":                  errors.New("missing steamcmd image"),
		},
		pullErrByImage: map[string]error{
			firstImage: errors.New("403 Forbidden"),
		},
		steamAuthLines: []string{
			"[SteamAuth:A0] Logged in as [U:1:1231122837]",
			"[SteamAuth:A0] Downloading app 413150...",
			"[SteamAuth:A0] Download failed: Download manifest failed across all CDN servers (403 Forbidden)",
		},
		containerLines: []string{
			"Logging in user steam-user",
			"Success! App '413150' fully installed.",
		},
	}
	driver := New(fake, slog.Default(), manager, store)
	job, err := driver.Install(context.Background(), registry.InstallRequest{
		Instance:      registry.Instance{ID: instance.ID},
		SteamUsername: "steam-user",
		SteamPassword: "steam-pass",
		VNCPassword:   "vnc-pass",
		AutoDownload:  true,
	})
	if err != nil {
		t.Fatalf("install: %v", err)
	}

	waitForDriverTestJobStatus(t, store, job.ID, storage.JobStatusSucceeded)
	if got, want := strings.Join(fake.pulledImages, ","), firstImage+","+secondImage; got != want {
		t.Fatalf("expected SteamCMD pulls %s, got %s", want, got)
	}
	if fake.containerOpts.ImageRef != secondImage {
		t.Fatalf("expected SteamCMD fallback to use second image %q, got %q", secondImage, fake.containerOpts.ImageRef)
	}
	logs, err := store.ListJobLogs(context.Background(), job.ID, 0, 1000)
	if err != nil {
		t.Fatalf("list logs: %v", err)
	}
	if !jobLogsContain(logs, "[pull:progress:2:2]") {
		t.Fatalf("expected SteamCMD image pull progress marker in job logs")
	}
	if got := strings.Join(fake.containerOpts.Entrypoint, " "); got != "/bin/sh" {
		t.Fatalf("expected SteamCMD fallback to override entrypoint to /bin/sh, got %q", got)
	}
	if !strings.Contains(strings.Join(fake.containerOpts.Command, " "), "command -v steamcmd") {
		t.Fatalf("SteamCMD command should support official steamcmd image path lookup: %#v", fake.containerOpts.Command)
	}
}

func TestSteamCMDImageRefsUsesMirrorCandidatesBeforeExistingCandidates(t *testing.T) {
	envVals := map[string]string{
		"STEAMCMD_IMAGE_CANDIDATES": "docker.xuanyuan.me/steamcmd/steamcmd:latest,steamcmd/steamcmd:latest,docker.m.daocloud.io/steamcmd/steamcmd:latest,ghcr.io/steamcmd/steamcmd:latest,cm2network/steamcmd:latest",
	}
	refs := steamCMDImageRefs(envVals)
	want := []string{
		"dockerproxy.net/steamcmd/steamcmd:latest",
		"docker.1ms.run/steamcmd/steamcmd:latest",
		"docker.1panel.live/steamcmd/steamcmd:latest",
		"docker.jiaxin.site/steamcmd/steamcmd:latest",
		"dockerproxy.link/steamcmd/steamcmd:latest",
		"cm2network/steamcmd:latest",
		"ghcr.io/steamcmd/steamcmd:latest",
	}
	if got := strings.Join(refs, ","); got != strings.Join(want, ",") {
		t.Fatalf("expected SteamCMD image candidates %q, got %q", strings.Join(want, ","), got)
	}
	if normalized := steamCMDImageCandidatesValue(envVals["STEAMCMD_IMAGE_CANDIDATES"]); normalized != strings.Join(want, ",") {
		t.Fatalf("expected normalized candidates %q, got %q", strings.Join(want, ","), normalized)
	}
}

func TestServerImageRefsDoesNotMixExistingCandidatesFromAnotherTag(t *testing.T) {
	envVals := map[string]string{
		"SERVER_IMAGE":            "sdvd/server:1.5.0-preview.121",
		"SERVER_IMAGE_CANDIDATES": "sdvd/server:1.5.0-preview.121",
	}
	refs := serverImageRefs(envVals, TestedImageTag)
	want := []string{
		"dockerproxy.net/sdvd/server:1.5.0-preview.125",
		"docker.1ms.run/sdvd/server:1.5.0-preview.125",
		"docker.1panel.live/sdvd/server:1.5.0-preview.125",
		"docker.jiaxin.site/sdvd/server:1.5.0-preview.125",
		"dockerproxy.link/sdvd/server:1.5.0-preview.125",
		"sdvd/server:1.5.0-preview.125",
	}
	if got := strings.Join(refs, ","); got != strings.Join(want, ",") {
		t.Fatalf("expected server image candidates %q, got %q", strings.Join(want, ","), got)
	}
}

func TestSteamServiceImageRefsPrependsDefaultCandidatesToExistingSingleCandidate(t *testing.T) {
	envVals := map[string]string{
		"STEAM_SERVICE_IMAGE":            DefaultSteamServiceImage,
		"STEAM_SERVICE_IMAGE_CANDIDATES": DefaultSteamServiceImage,
	}
	refs := steamServiceImageRefs(envVals)
	want := []string{
		"docker.1ms.run/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2",
		"crpi-9z3bkb9g7fxeohrg.cn-hangzhou.personal.cr.aliyuncs.com/anxi-panel/junimo-steam-service-cn:1.5.0-anxi.2",
		"docker.m.daocloud.io/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2",
		"ghcr.io/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2",
		"anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2",
	}
	if got := strings.Join(refs, ","); got != strings.Join(want, ",") {
		t.Fatalf("expected steam service image candidates %q, got %q", strings.Join(want, ","), got)
	}
}

func TestDriverInstallSteamCMDFailsWhenMobileApprovalTimesOut(t *testing.T) {
	dataDir := t.TempDir()
	store, err := storage.Open(context.Background(), config.Config{
		DataDir: dataDir,
		DBPath:  filepath.Join(dataDir, "panel.db"),
	})
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate storage: %v", err)
	}

	instanceDir := filepath.Join(dataDir, "instances", storage.DefaultInstanceID)
	instance, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{
		ID:       storage.DefaultInstanceID,
		DriverID: storage.DefaultDriverID,
		Name:     "Stardew Valley",
		DataDir:  instanceDir,
	})
	if err != nil {
		t.Fatalf("ensure instance: %v", err)
	}

	manager := jobs.NewManager(store, slog.Default())
	fake := &fakeDocker{
		steamAuthLines: []string{
			"[SteamAuth:A0] Logged in as [U:1:1231122837]",
			"[SteamAuth:A0] Downloading app 413150...",
			"[SteamAuth:A0] Download failed: Download manifest failed across all CDN servers (403 Forbidden)",
		},
		containerLines: []string{
			"Logging in user 'steam-user' [U:1:0] to Steam Public...This account is protected by a Steam Guard mobile authenticator.",
			"Please confirm the login in the Steam Mobile app on your phone.",
			"Waiting for confirmation...",
			"Wait for confirmation timed out.Timed out waiting for confirmation.",
			"ERROR (Timeout)",
			"Unloading Steam API...OK",
		},
	}
	driver := New(fake, slog.Default(), manager, store)
	job, err := driver.Install(context.Background(), registry.InstallRequest{
		Instance:      registry.Instance{ID: instance.ID},
		SteamUsername: "steam-user",
		SteamPassword: "steam-pass",
		VNCPassword:   "vnc-pass",
		AutoDownload:  true,
	})
	if err != nil {
		t.Fatalf("install: %v", err)
	}

	waitForDriverTestJobStatus(t, store, job.ID, storage.JobStatusFailed)
	updated, err := store.GetInstance(context.Background(), instance.ID)
	if err != nil {
		t.Fatalf("get instance: %v", err)
	}
	if updated.State != storage.InstanceStateError || updated.DriverPhase != "steamcmd_failed" {
		t.Fatalf("instance state should be steamcmd_failed after approval timeout: state=%s phase=%s", updated.State, updated.DriverPhase)
	}
}

func TestDriverInstallResumesSteamCMDAndFallsBackWhenCachedAuthorizationIsMissing(t *testing.T) {
	dataDir := t.TempDir()
	store, err := storage.Open(context.Background(), config.Config{
		DataDir: dataDir,
		DBPath:  filepath.Join(dataDir, "panel.db"),
	})
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate storage: %v", err)
	}

	instanceDir := filepath.Join(dataDir, "instances", storage.DefaultInstanceID)
	instance, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{
		ID:       storage.DefaultInstanceID,
		DriverID: storage.DefaultDriverID,
		Name:     "Stardew Valley",
		DataDir:  instanceDir,
	})
	if err != nil {
		t.Fatalf("ensure instance: %v", err)
	}
	if _, err := store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID:           instance.ID,
		State:        storage.InstanceStateError,
		StateMessage: "SteamCMD approval timed out",
		DriverPhase:  "steamcmd_failed",
	}); err != nil {
		t.Fatalf("set steamcmd failed phase: %v", err)
	}
	if err := os.MkdirAll(instanceDir, 0o755); err != nil {
		t.Fatalf("mkdir instance: %v", err)
	}
	if err := os.WriteFile(filepath.Join(instanceDir, ".env"), []byte("STEAMCMD_AUTH_COMPLETED=true\n"), 0o600); err != nil {
		t.Fatalf("seed cached SteamCMD flag: %v", err)
	}

	manager := jobs.NewManager(store, slog.Default())
	fake := &fakeDocker{
		steamAuthErr:   errors.New("steam-auth should not run"),
		containerCodes: []int{1, 0},
		containerRunLines: [][]string{
			{"Logging in user steam-user", "Cached credentials not found."},
			{
				"Logging in user steam-user",
				"Waiting for user info...OK",
				"Success! App '413150' fully installed.",
				"Success! App '1007' fully installed.",
			},
		},
	}
	driver := New(fake, slog.Default(), manager, store)
	job, err := driver.Install(context.Background(), registry.InstallRequest{
		Instance:      registry.Instance{ID: instance.ID},
		SteamUsername: "steam-user",
		SteamPassword: "steam-pass",
		VNCPassword:   "vnc-pass",
		AutoDownload:  true,
	})
	if err != nil {
		t.Fatalf("install: %v", err)
	}

	waitForDriverTestJobStatus(t, store, job.ID, storage.JobStatusSucceeded)
	if fake.steamAuthRuns != 0 {
		t.Fatalf("steam-auth should be skipped on SteamCMD retry, ran %d times", fake.steamAuthRuns)
	}
	if fake.composePulls != 0 {
		t.Fatalf("Junimo compose pull should be skipped on SteamCMD retry, ran %d times", fake.composePulls)
	}
	if len(fake.pulledImages) != 0 {
		t.Fatalf("local SteamCMD image should not be pulled again, pulled %v", fake.pulledImages)
	}
	if fake.containerRuns != 2 {
		t.Fatalf("expected cached SteamCMD login then one full-login fallback, ran %d times", fake.containerRuns)
	}
	finalCommand := strings.Join(fake.containerOpts.Command, " ")
	if !strings.Contains(finalCommand, `+login "$STEAM_USERNAME" "$STEAM_PASSWORD" +app_update 413150`) {
		t.Fatalf("cache failure should automatically retry with a full SteamCMD login, command=%q", finalCommand)
	}
	updated, err := store.GetInstance(context.Background(), instance.ID)
	if err != nil {
		t.Fatalf("get instance: %v", err)
	}
	if updated.State != storage.InstanceStateGameInstalled || updated.DriverPhase != "game_installed" {
		t.Fatalf("instance state should be installed after direct steamcmd retry: state=%s phase=%s", updated.State, updated.DriverPhase)
	}
	// SteamCMD and steam-auth share the saved account/password, but keep their
	// authorization completion state independent: a SteamCMD login must set only
	// STEAMCMD_AUTH_COMPLETED, never STEAM_AUTH_COMPLETED.
	envRaw, err := os.ReadFile(filepath.Join(instanceDir, ".env"))
	if err != nil {
		t.Fatalf("read .env: %v", err)
	}
	if !strings.Contains(string(envRaw), "STEAMCMD_AUTH_COMPLETED=true") {
		t.Fatalf("STEAMCMD_AUTH_COMPLETED should be set after SteamCMD login, .env=%s", envRaw)
	}
	if strings.Contains(string(envRaw), "STEAM_AUTH_COMPLETED=true") {
		t.Fatalf("SteamCMD login must NOT set STEAM_AUTH_COMPLETED, .env=%s", envRaw)
	}
}

func TestDriverInstallCancelsCachedSteamCMDGuardPromptAndImmediatelyFallsBackToFullLogin(t *testing.T) {
	testCases := []struct {
		name   string
		prompt string
	}{
		{name: "guard choice", prompt: "Steam Guard: [1] Approve in Steam Mobile [2] Enter code from email"},
		{name: "mobile approval", prompt: "Please confirm the login in the Steam Mobile app on your phone."},
		{name: "guard code", prompt: "Enter Steam Guard code sent to qq.com:"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			dataDir := t.TempDir()
			store, err := storage.Open(context.Background(), config.Config{
				DataDir: dataDir,
				DBPath:  filepath.Join(dataDir, "panel.db"),
			})
			if err != nil {
				t.Fatalf("open storage: %v", err)
			}
			defer store.Close()
			if err := store.Migrate(context.Background()); err != nil {
				t.Fatalf("migrate storage: %v", err)
			}

			instanceDir := filepath.Join(dataDir, "instances", storage.DefaultInstanceID)
			instance, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{
				ID:       storage.DefaultInstanceID,
				DriverID: storage.DefaultDriverID,
				Name:     "Stardew Valley",
				DataDir:  instanceDir,
			})
			if err != nil {
				t.Fatalf("ensure instance: %v", err)
			}
			if _, err := store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
				ID:           instance.ID,
				State:        storage.InstanceStateError,
				StateMessage: "SteamCMD approval interrupted",
				DriverPhase:  "steamcmd_failed",
			}); err != nil {
				t.Fatalf("set SteamCMD retry phase: %v", err)
			}
			if err := os.MkdirAll(instanceDir, 0o755); err != nil {
				t.Fatalf("mkdir instance: %v", err)
			}
			if err := os.WriteFile(filepath.Join(instanceDir, ".env"), []byte("STEAMCMD_AUTH_COMPLETED=true\n"), 0o600); err != nil {
				t.Fatalf("seed cached SteamCMD flag: %v", err)
			}

			manager := jobs.NewManager(store, slog.Default())
			fake := &fakeDocker{
				steamAuthErr:                errors.New("steam-auth should not run"),
				containerWaitForCancelOnRun: 1,
				containerRunLines: [][]string{
					{
						"Logging in user steam-user with cached credentials",
						testCase.prompt,
					},
					{
						"Logging in user steam-user",
						"Waiting for user info...OK",
						"Success! App '413150' fully installed.",
						"Success! App '1007' fully installed.",
					},
				},
			}
			driver := New(fake, slog.Default(), manager, store)
			job, err := driver.Install(context.Background(), registry.InstallRequest{
				Instance:      registry.Instance{ID: instance.ID},
				SteamUsername: "steam-user",
				SteamPassword: "steam-pass",
				VNCPassword:   "vnc-pass",
				AutoDownload:  true,
			})
			if err != nil {
				t.Fatalf("install: %v", err)
			}

			waitForDriverTestJobStatus(t, store, job.ID, storage.JobStatusSucceeded)
			if !fake.containerSawCancel {
				t.Fatalf("cached SteamCMD attempt did not observe its context cancellation")
			}
			if fake.containerRuns != 2 {
				t.Fatalf("expected canceled cached attempt followed immediately by one full login, ran %d times", fake.containerRuns)
			}
			if len(fake.containerRunOpts) != 2 {
				t.Fatalf("expected command history for both SteamCMD attempts, got %d", len(fake.containerRunOpts))
			}
			cachedCommand := strings.Join(fake.containerRunOpts[0].Command, " ")
			if !strings.Contains(cachedCommand, `+login "$STEAM_USERNAME" +app_update 413150`) ||
				strings.Contains(cachedCommand, `+login "$STEAM_USERNAME" "$STEAM_PASSWORD"`) {
				t.Fatalf("first SteamCMD attempt should use username-only cached login, command=%q", cachedCommand)
			}
			fullLoginCommand := strings.Join(fake.containerRunOpts[1].Command, " ")
			if !strings.Contains(fullLoginCommand, `+login "$STEAM_USERNAME" "$STEAM_PASSWORD" +app_update 413150`) {
				t.Fatalf("second SteamCMD attempt should immediately use the saved full-login credentials, command=%q", fullLoginCommand)
			}

			updated, err := store.GetInstance(context.Background(), instance.ID)
			if err != nil {
				t.Fatalf("get instance: %v", err)
			}
			if updated.State != storage.InstanceStateGameInstalled || updated.DriverPhase != "game_installed" {
				t.Fatalf("internal cached-attempt cancellation must not mark base installation failed: state=%s phase=%s", updated.State, updated.DriverPhase)
			}
			logs, err := store.ListJobLogs(context.Background(), job.ID, 0, 1000)
			if err != nil {
				t.Fatalf("list job logs: %v", err)
			}
			if !jobLogsContain(logs, "[steamcmd] 已保存的 SteamCMD 授权缓存不可用，正在自动切换为账号密码完整登录。") {
				t.Fatalf("expected cached authorization fallback log")
			}
			for _, logEntry := range logs {
				if strings.Contains(logEntry.Message, "SteamCMD 下载运行失败") {
					t.Fatalf("deliberate cached-attempt cancellation was logged as a base install failure: %q", logEntry.Message)
				}
			}
		})
	}
}

func TestDriverInstallClassifiesCombinedSteamCMDInvalidPasswordLineAsCredentialsRequired(t *testing.T) {
	dataDir := t.TempDir()
	store, err := storage.Open(context.Background(), config.Config{
		DataDir: dataDir,
		DBPath:  filepath.Join(dataDir, "panel.db"),
	})
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate storage: %v", err)
	}

	instanceDir := filepath.Join(dataDir, "instances", storage.DefaultInstanceID)
	instance, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{
		ID:       storage.DefaultInstanceID,
		DriverID: storage.DefaultDriverID,
		Name:     "Stardew Valley",
		DataDir:  instanceDir,
	})
	if err != nil {
		t.Fatalf("ensure instance: %v", err)
	}
	if _, err := store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID:           instance.ID,
		State:        storage.InstanceStateError,
		StateMessage: "SteamCMD fallback failed",
		DriverPhase:  "steamcmd_failed",
	}); err != nil {
		t.Fatalf("set steamcmd failed phase: %v", err)
	}
	if err := os.MkdirAll(instanceDir, 0o755); err != nil {
		t.Fatalf("mkdir instance: %v", err)
	}
	if err := os.WriteFile(filepath.Join(instanceDir, ".env"), []byte("STEAMCMD_AUTH_COMPLETED=true\n"), 0o600); err != nil {
		t.Fatalf("seed cached SteamCMD flag: %v", err)
	}

	manager := jobs.NewManager(store, slog.Default())
	fake := &fakeDocker{
		steamAuthErr:   errors.New("steam-auth should not run"),
		containerCodes: []int{1, 5},
		containerRunLines: [][]string{
			{
				"[  0%] Downloading update (0 of 58,938 KB)...",
				"Update state (0x0) unknown, progress: 0.00 (0 / 0)",
				"Waiting for user info...OK",
				"Cached credentials not found.",
			},
			{
				"[  0%] Downloading update (0 of 58,938 KB)...",
				"Update state (0x0) unknown, progress: 0.00 (0 / 0)",
				"Logging in user steam-user [U:1:0] to Steam Public...ERROR (Invalid Password)",
			},
		},
	}
	driver := New(fake, slog.Default(), manager, store)
	job, err := driver.Install(context.Background(), registry.InstallRequest{
		Instance:      registry.Instance{ID: instance.ID},
		SteamUsername: "steam-user",
		SteamPassword: "steam-pass",
		VNCPassword:   "vnc-pass",
		AutoDownload:  true,
	})
	if err != nil {
		t.Fatalf("install: %v", err)
	}

	waitForDriverTestJobStatus(t, store, job.ID, storage.JobStatusFailed)
	if fake.steamAuthRuns != 0 {
		t.Fatalf("steam-auth should be skipped on SteamCMD retry, ran %d times", fake.steamAuthRuns)
	}
	if fake.containerRuns != 2 {
		t.Fatalf("expected cached SteamCMD login then one full-login attempt, ran %d times", fake.containerRuns)
	}
	if len(fake.containerRunOpts) != 2 {
		t.Fatalf("expected command history for both SteamCMD attempts, got %d", len(fake.containerRunOpts))
	}
	cachedCommand := strings.Join(fake.containerRunOpts[0].Command, " ")
	if !strings.Contains(cachedCommand, `+login "$STEAM_USERNAME" +app_update 413150`) ||
		strings.Contains(cachedCommand, `+login "$STEAM_USERNAME" "$STEAM_PASSWORD"`) {
		t.Fatalf("first SteamCMD attempt should use username-only cached login, command=%q", cachedCommand)
	}
	fullLoginCommand := strings.Join(fake.containerRunOpts[1].Command, " ")
	if !strings.Contains(fullLoginCommand, `+login "$STEAM_USERNAME" "$STEAM_PASSWORD" +app_update 413150`) {
		t.Fatalf("second SteamCMD attempt should use the saved fallback password, command=%q", fullLoginCommand)
	}
	updated, err := store.GetInstance(context.Background(), instance.ID)
	if err != nil {
		t.Fatalf("get instance: %v", err)
	}
	if updated.State != storage.InstanceStateCredentialsRequired || updated.DriverPhase != "credentials_required" {
		t.Fatalf("combined login/password error must request fresh credentials, got state=%s phase=%s", updated.State, updated.DriverPhase)
	}
	if !strings.Contains(updated.StateMessage.String, "账号、密码或验证码不正确") {
		t.Fatalf("credential failure message should explain re-entry, got %q", updated.StateMessage.String)
	}
	envVals, err := sjconfig.ReadEnvFile(filepath.Join(instanceDir, ".env"))
	if err != nil {
		t.Fatalf("read .env: %v", err)
	}
	if strings.EqualFold(strings.TrimSpace(envVals["STEAMCMD_AUTH_COMPLETED"]), "true") {
		t.Fatalf("client update progress must not restore a failed SteamCMD cache authorization")
	}
}

func TestDriverInstallSkipsSteamAuthOnceCompletedFlagIsSet(t *testing.T) {
	dataDir := t.TempDir()
	store, err := storage.Open(context.Background(), config.Config{
		DataDir: dataDir,
		DBPath:  filepath.Join(dataDir, "panel.db"),
	})
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate storage: %v", err)
	}

	instanceDir := filepath.Join(dataDir, "instances", storage.DefaultInstanceID)
	instance, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{
		ID:       storage.DefaultInstanceID,
		DriverID: storage.DefaultDriverID,
		Name:     "Stardew Valley",
		DataDir:  instanceDir,
	})
	if err != nil {
		t.Fatalf("ensure instance: %v", err)
	}
	// steam-auth succeeded once before, but the phase was reset (e.g. an
	// interrupted install). Only the durable STEAM_AUTH_COMPLETED flag remains.
	if _, err := store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID:           instance.ID,
		State:        storage.InstanceStateError,
		StateMessage: "interrupted",
		DriverPhase:  "install_interrupted",
	}); err != nil {
		t.Fatalf("set interrupted phase: %v", err)
	}
	if err := os.MkdirAll(instanceDir, 0o755); err != nil {
		t.Fatalf("mkdir instance: %v", err)
	}
	if err := os.WriteFile(filepath.Join(instanceDir, ".env"), []byte("STEAM_AUTH_COMPLETED=true\n"), 0o600); err != nil {
		t.Fatalf("seed .env: %v", err)
	}

	manager := jobs.NewManager(store, slog.Default())
	fake := &fakeDocker{
		steamAuthErr: errors.New("steam-auth should not run once STEAM_AUTH_COMPLETED is set"),
		containerLines: []string{
			"Logging in user steam-user",
			"Success! App '413150' fully installed.",
			"Success! App '1007' fully installed.",
		},
	}
	driver := New(fake, slog.Default(), manager, store)
	job, err := driver.Install(context.Background(), registry.InstallRequest{
		Instance:      registry.Instance{ID: instance.ID},
		SteamUsername: "steam-user",
		SteamPassword: "steam-pass",
		VNCPassword:   "vnc-pass",
		AutoDownload:  true,
	})
	if err != nil {
		t.Fatalf("install: %v", err)
	}

	waitForDriverTestJobStatus(t, store, job.ID, storage.JobStatusSucceeded)
	if fake.steamAuthRuns != 0 {
		t.Fatalf("steam-auth must be skipped once STEAM_AUTH_COMPLETED is set, ran %d times", fake.steamAuthRuns)
	}
	if fake.containerRuns != 1 {
		t.Fatalf("expected SteamCMD to run once, ran %d times", fake.containerRuns)
	}
}

func TestDriverInstallRetryKeepsSteamCMDPrimary(t *testing.T) {
	dataDir := t.TempDir()
	store, err := storage.Open(context.Background(), config.Config{
		DataDir: dataDir,
		DBPath:  filepath.Join(dataDir, "panel.db"),
	})
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate storage: %v", err)
	}

	instanceDir := filepath.Join(dataDir, "instances", storage.DefaultInstanceID)
	instance, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{
		ID:       storage.DefaultInstanceID,
		DriverID: storage.DefaultDriverID,
		Name:     "Stardew Valley",
		DataDir:  instanceDir,
	})
	if err != nil {
		t.Fatalf("ensure instance: %v", err)
	}
	// Image pull failed before authentication ever happened.
	if _, err := store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID:           instance.ID,
		State:        storage.InstanceStateJunimoScaffolded,
		StateMessage: "pull failed",
		DriverPhase:  "pull_failed",
	}); err != nil {
		t.Fatalf("set pull_failed phase: %v", err)
	}

	manager := jobs.NewManager(store, slog.Default())
	fake := &fakeDocker{
		steamAuthErr: errors.New("steam-auth must not run during install retry"),
		containerLines: []string{
			"Logging in user steam-user",
			"Success! App '413150' fully installed.",
			"Success! App '1007' fully installed.",
		},
	}
	driver := New(fake, slog.Default(), manager, store)
	// reuseCredentials retry remains on the SteamCMD primary installation path.
	job, err := driver.Install(context.Background(), registry.InstallRequest{
		Instance:      registry.Instance{ID: instance.ID},
		SteamUsername: "steam-user",
		SteamPassword: "steam-pass",
		VNCPassword:   "vnc-pass",
		AutoDownload:  true,
	})
	if err != nil {
		t.Fatalf("install: %v", err)
	}

	waitForDriverTestJobStatus(t, store, job.ID, storage.JobStatusSucceeded)
	if fake.steamAuthRuns != 0 {
		t.Fatalf("steam-auth must remain absent after a pull failure retry, ran %d times", fake.steamAuthRuns)
	}
	if fake.containerRuns != 1 {
		t.Fatalf("SteamCMD primary path should run once on retry, ran %d times", fake.containerRuns)
	}
}

func TestDriverInstallRepairUsesCachedLoginAndAnonymousSDK(t *testing.T) {
	dataDir := t.TempDir()
	store, err := storage.Open(context.Background(), config.Config{
		DataDir: dataDir,
		DBPath:  filepath.Join(dataDir, "panel.db"),
	})
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate storage: %v", err)
	}

	instanceDir := filepath.Join(dataDir, "instances", storage.DefaultInstanceID)
	instance, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{
		ID:       storage.DefaultInstanceID,
		DriverID: storage.DefaultDriverID,
		Name:     "Stardew Valley",
		DataDir:  instanceDir,
	})
	if err != nil {
		t.Fatalf("ensure instance: %v", err)
	}
	if _, err := store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID:           instance.ID,
		State:        storage.InstanceStateGameInstalled,
		StateMessage: "Game installed",
		DriverPhase:  "game_installed",
	}); err != nil {
		t.Fatalf("set installed phase: %v", err)
	}
	// Simulate an instance that has already authorized SteamCMD once. Repair skips
	// steam-auth and the compose pull and uses SteamCMD's own cached login; the SDK
	// remains anonymous.
	if err := os.MkdirAll(instanceDir, 0o755); err != nil {
		t.Fatalf("mkdir instance: %v", err)
	}
	if err := os.WriteFile(filepath.Join(instanceDir, ".env"), []byte("STEAMCMD_AUTH_COMPLETED=true\n"), 0o600); err != nil {
		t.Fatalf("seed .env: %v", err)
	}

	manager := jobs.NewManager(store, slog.Default())
	fake := &fakeDocker{
		steamAuthErr: errors.New("steam-auth should not run during repair"),
		containerLines: []string{
			"Logging in user steam-user",
			"Success! App '413150' fully installed.",
			"Success! App '1007' fully installed.",
		},
	}
	driver := New(fake, slog.Default(), manager, store)
	job, err := driver.Install(context.Background(), registry.InstallRequest{
		Instance:      registry.Instance{ID: instance.ID},
		SteamUsername: "steam-user",
		SteamPassword: "steam-pass",
		VNCPassword:   "vnc-pass",
		AutoDownload:  true,
		SteamCMDRetry: true,
	})
	if err != nil {
		t.Fatalf("install: %v", err)
	}

	waitForDriverTestJobStatus(t, store, job.ID, storage.JobStatusSucceeded)
	if fake.steamAuthRuns != 0 {
		t.Fatalf("steam-auth should be skipped on repair, ran %d times", fake.steamAuthRuns)
	}
	if fake.composePulls != 0 {
		t.Fatalf("Junimo compose pull should be skipped on repair, ran %d times", fake.composePulls)
	}
	if fake.containerRuns != 1 {
		t.Fatalf("expected SteamCMD container to run once, ran %d times", fake.containerRuns)
	}
	command := strings.Join(fake.containerOpts.Command, " ")
	if !strings.Contains(command, `+@NoPromptForPassword 1 +force_install_dir /data/game +login "$STEAM_USERNAME" +app_update 413150`) {
		t.Fatalf("repair game download should use the cached SteamCMD login, command=%q", command)
	}
	if strings.Contains(command, `+login "$STEAM_USERNAME" "$STEAM_PASSWORD" +app_update 413150`) {
		t.Fatalf("cached SteamCMD login must not submit the password again, command=%q", command)
	}
	if !strings.Contains(command, `+login anonymous +app_update 1007`) {
		t.Fatalf("repair Steam SDK download should use anonymous login, command=%q", command)
	}
	if strings.Contains(command, `"$STEAM_USERNAME" +app_update 1007`) || strings.Contains(command, `"$STEAM_PASSWORD" +app_update 1007`) {
		t.Fatalf("Steam SDK download must not pass account credentials, command=%q", command)
	}
}

func TestDriverInstallRepairUsesMigratedSteamCMDAuthorizationImmediately(t *testing.T) {
	dataDir := t.TempDir()
	store, err := storage.Open(context.Background(), config.Config{
		DataDir: dataDir,
		DBPath:  filepath.Join(dataDir, "panel.db"),
	})
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate storage: %v", err)
	}

	instanceDir := filepath.Join(dataDir, "instances", storage.DefaultInstanceID)
	instance, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{
		ID:       storage.DefaultInstanceID,
		DriverID: storage.DefaultDriverID,
		Name:     "Stardew Valley",
		DataDir:  instanceDir,
	})
	if err != nil {
		t.Fatalf("ensure instance: %v", err)
	}
	if _, err := store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID:           instance.ID,
		State:        storage.InstanceStateGameInstalled,
		StateMessage: "Game installed",
		DriverPhase:  "game_installed",
	}); err != nil {
		t.Fatalf("set installed phase: %v", err)
	}

	manager := jobs.NewManager(store, slog.Default())
	fake := &fakeDocker{
		steamAuthErr: errors.New("steam-auth should not run during repair"),
		authMigrateLines: []string{
			"anxi-steamcmd-auth-migrate: checking legacy authorization cache",
			"anxi-steamcmd-auth-migrate: migrated legacy cache",
		},
		containerLines: []string{
			"Logging in user steam-user",
			"Waiting for user info...OK",
			"Success! App '413150' fully installed.",
			"Success! App '1007' fully installed.",
		},
	}
	driver := New(fake, slog.Default(), manager, store)
	job, err := driver.Install(context.Background(), registry.InstallRequest{
		Instance:      registry.Instance{ID: instance.ID},
		SteamUsername: "steam-user",
		SteamPassword: "steam-pass",
		VNCPassword:   "vnc-pass",
		AutoDownload:  true,
		SteamCMDRetry: true,
	})
	if err != nil {
		t.Fatalf("install: %v", err)
	}

	waitForDriverTestJobStatus(t, store, job.ID, storage.JobStatusSucceeded)
	if fake.authMigrateRuns != 1 {
		t.Fatalf("expected one legacy authorization migration, ran %d times", fake.authMigrateRuns)
	}
	if fake.containerRuns != 1 {
		t.Fatalf("migrated authorization should be tried once without a full-login rerun, ran %d times", fake.containerRuns)
	}
	command := strings.Join(fake.containerOpts.Command, " ")
	if !strings.Contains(command, `+@NoPromptForPassword 1 +force_install_dir /data/game +login "$STEAM_USERNAME" +app_update 413150`) {
		t.Fatalf("migrated authorization should be used immediately, command=%q", command)
	}
	if strings.Contains(command, `+login "$STEAM_USERNAME" "$STEAM_PASSWORD" +app_update 413150`) {
		t.Fatalf("migrated authorization must not submit the password before the cache is tried, command=%q", command)
	}
}

func TestDriverInstallUsesExistingLaterSteamCMDCandidateBeforePulling(t *testing.T) {
	dataDir := t.TempDir()
	store, err := storage.Open(context.Background(), config.Config{
		DataDir: dataDir,
		DBPath:  filepath.Join(dataDir, "panel.db"),
	})
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate storage: %v", err)
	}

	instanceDir := filepath.Join(dataDir, "instances", storage.DefaultInstanceID)
	instance, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{
		ID:       storage.DefaultInstanceID,
		DriverID: storage.DefaultDriverID,
		Name:     "Stardew Valley",
		DataDir:  instanceDir,
	})
	if err != nil {
		t.Fatalf("ensure instance: %v", err)
	}

	firstImage := "dockerproxy.net/steamcmd/steamcmd:latest"
	secondImage := "docker.1ms.run/steamcmd/steamcmd:latest"
	manager := jobs.NewManager(store, slog.Default())
	fake := &fakeDocker{
		inspectErrByImage: map[string]error{
			firstImage: errors.New("missing first steamcmd image"),
		},
		steamAuthLines: []string{
			"[SteamAuth:A0] Logged in as [U:1:1231122837]",
			"[SteamAuth:A0] Downloading app 413150...",
			"[SteamAuth:A0] Download failed: Download manifest failed across all CDN servers (403 Forbidden)",
		},
		containerLines: []string{
			"Logging in user steam-user",
			"Success! App '413150' fully installed.",
			"Success! App '1007' fully installed.",
		},
	}
	driver := New(fake, slog.Default(), manager, store)
	job, err := driver.Install(context.Background(), registry.InstallRequest{
		Instance:      registry.Instance{ID: instance.ID},
		SteamUsername: "steam-user",
		SteamPassword: "steam-pass",
		VNCPassword:   "vnc-pass",
		AutoDownload:  true,
	})
	if err != nil {
		t.Fatalf("install: %v", err)
	}

	waitForDriverTestJobStatus(t, store, job.ID, storage.JobStatusSucceeded)
	if len(fake.pulledImages) != 0 {
		t.Fatalf("existing later SteamCMD candidate should be used before pulling, pulled %v", fake.pulledImages)
	}
	if fake.containerOpts.ImageRef != secondImage {
		t.Fatalf("expected SteamCMD fallback to use existing second image %q, got %q", secondImage, fake.containerOpts.ImageRef)
	}
}

func TestDriverSendSteamGuardInput_NoActiveJob(t *testing.T) {
	driver := New(nil, nil, nil, nil)
	err := driver.SendSteamGuardInput("nonexistent-job-id", "12345")
	if err == nil {
		t.Fatal("expected error for nonexistent job")
	}
}

func jobLogsContain(logs []storage.JobLog, message string) bool {
	for _, log := range logs {
		if log.Message == message {
			return true
		}
	}
	return false
}

func stringSliceContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestBuildSteamCMDOptsGameFullLoginSDKAnonymous(t *testing.T) {
	opts := (&installRunner{instance: storage.Instance{DataDir: "/data/instances/stardew"}}).buildSteamCMDOpts("img:latest")
	script := opts.Command[len(opts.Command)-1]

	// Without a completed SteamCMD authorization flag, the first game login must
	// still use the full username+password form.
	if !strings.Contains(script, `+login "$STEAM_USERNAME" "$STEAM_PASSWORD" +app_update 413150`) {
		t.Fatalf("game command should do a full username+password login, got:\n%s", script)
	}
	// The SDK (1007) is public and must download anonymously so it never needs
	// credentials or a second Steam Guard approval.
	if !strings.Contains(script, `+login anonymous +app_update 1007`) {
		t.Fatalf("SDK command should use anonymous login, got:\n%s", script)
	}
	if strings.Contains(script, `"$STEAM_USERNAME" +app_update 1007`) || strings.Contains(script, `"$STEAM_PASSWORD" +app_update 1007`) {
		t.Fatalf("SDK command must not pass account credentials, got:\n%s", script)
	}
}

func TestBuildSteamCMDOptsCachedLoginUsesOneCrossImageAuthorizationVolume(t *testing.T) {
	opts := (&installRunner{
		instance:         storage.Instance{DataDir: "/data/instances/stardew"},
		steamCMDUseCache: true,
	}).buildSteamCMDOpts("img:latest")
	script := opts.Command[len(opts.Command)-1]

	if !strings.Contains(script, `+@NoPromptForPassword 1 +force_install_dir /data/game +login "$STEAM_USERNAME" +app_update 413150`) {
		t.Fatalf("cached game command should use username-only SteamCMD login, got:\n%s", script)
	}
	if strings.Contains(script, `+login "$STEAM_USERNAME" "$STEAM_PASSWORD" +app_update 413150`) {
		t.Fatalf("cached game command must not submit the password, got:\n%s", script)
	}
	wantBinds := []string{
		"stardew_steamcmd-login:/home/steam/Steam",
		"stardew_steamcmd-login:/home/steam/.local/share/Steam",
		"stardew_steamcmd-login:/root/Steam",
		"stardew_steamcmd-login:/root/.local/share/Steam",
	}
	for _, bind := range wantBinds {
		if !stringSliceContains(opts.Binds, bind) {
			t.Fatalf("SteamCMD authorization volume should cover cross-image path %q, binds=%v", bind, opts.Binds)
		}
	}
	for _, obsolete := range []string{"stardew_steamcmd-user-local:", "stardew_steamcmd-root-local:"} {
		for _, bind := range opts.Binds {
			if strings.HasPrefix(bind, obsolete) {
				t.Fatalf("split SteamCMD authorization volume should no longer be mounted: %q", bind)
			}
		}
	}
}

func TestBuildSteamCMDAuthMigrationOptsCopiesLegacyCachesIntoCanonicalVolume(t *testing.T) {
	opts := (&installRunner{
		instance: storage.Instance{DataDir: "/data/instances/stardew"},
	}).buildSteamCMDAuthMigrationOpts("img:latest")
	script := opts.Command[len(opts.Command)-1]

	for _, bind := range []string{
		"stardew_steamcmd-login:/auth",
		"stardew_steamcmd-root-local:/legacy/root:ro",
		"stardew_steamcmd-user-local:/legacy/user:ro",
	} {
		if !stringSliceContains(opts.Binds, bind) {
			t.Fatalf("SteamCMD auth migration should mount %q, binds=%v", bind, opts.Binds)
		}
	}
	if !strings.Contains(script, `[ -s /auth/config/config.vdf ]`) {
		t.Fatalf("migration must preserve an existing canonical cache, script=%q", script)
	}
	if !strings.Contains(script, `cp -a "${legacy}/config/." /auth/config/`) {
		t.Fatalf("migration must copy legacy SteamCMD config without printing it, script=%q", script)
	}
	if strings.Contains(script, "cat ") {
		t.Fatalf("migration must never print credential file contents, script=%q", script)
	}
}

func waitForDriverTestJobStatus(t *testing.T, store *storage.Store, jobID string, status string) storage.Job {
	t.Helper()
	// Several lifecycle fixtures use a 10-second job timeout. Under the full
	// package gate, concurrent SQLite/filesystem work can legitimately push a
	// terminal transition past five seconds, so the observer must outlive the
	// runner's own bounded timeout without becoming unbounded itself.
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		job, err := store.GetJob(context.Background(), jobID)
		if err != nil {
			t.Fatalf("get job: %v", err)
		}
		if job.Status == status {
			return job
		}
		time.Sleep(20 * time.Millisecond)
	}
	job, _ := store.GetJob(context.Background(), jobID)
	t.Fatalf("job %s did not reach %s, got %s", jobID, status, job.Status)
	return storage.Job{}
}

func waitForDriverTestPhase(t *testing.T, store *storage.Store, instanceID, phase string) storage.Instance {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		instance, err := store.GetInstance(context.Background(), instanceID)
		if err != nil {
			t.Fatalf("get instance: %v", err)
		}
		if instance.DriverPhase == phase {
			return instance
		}
		time.Sleep(20 * time.Millisecond)
	}
	instance, _ := store.GetInstance(context.Background(), instanceID)
	t.Fatalf("instance %s did not reach phase %s, got %s", instanceID, phase, instance.DriverPhase)
	return storage.Instance{}
}
