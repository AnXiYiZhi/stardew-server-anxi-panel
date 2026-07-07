package stardew_junimo

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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
	workDir           string
	psResult          paneldocker.ComposePsResult
	psErr             error
	pullResult        paneldocker.CommandResult
	pullErr           error
	composePulls      int
	pullErrByImage    map[string]error
	pulledImages      []string
	inspectResult     paneldocker.CommandResult
	inspectErr        error
	inspectErrByImage map[string]error
	inspectedImages   []string
	steamAuthCode     int
	steamAuthErr      error
	steamAuthRuns     int
	steamAuthLines    []string
	containerCode     int
	containerCodes    []int
	containerErr      error
	containerRuns     int
	containerLines    []string
	containerOpts     paneldocker.ContainerTTYRunOpts
	smapiRuns         int
	smapiLines        []string
	smapiOpts         paneldocker.ContainerTTYRunOpts
	removedVolumes    []string
	removedByVolumes  []string
	restartedServices []string
}

func (f *fakeDocker) ComposePs(ctx context.Context, dir string) (paneldocker.ComposePsResult, error) {
	f.workDir = dir
	return f.psResult, f.psErr
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

func (f *fakeDocker) RunSteamAuthTTY(_ context.Context, _ string, _ paneldocker.SteamAuthRunOpts, _ <-chan string, lineHandler func(string)) (int, error) {
	f.steamAuthRuns++
	for _, line := range f.steamAuthLines {
		lineHandler(line)
	}
	return f.steamAuthCode, f.steamAuthErr
}

func (f *fakeDocker) RunContainerTTY(_ context.Context, opts paneldocker.ContainerTTYRunOpts, _ <-chan string, lineHandler func(string)) (int, error) {
	command := strings.Join(opts.Command, " ")
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
	for _, line := range f.containerLines {
		lineHandler(line)
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
	return paneldocker.CommandResult{ExitCode: 0}, nil
}

func (f *fakeDocker) RemoveContainersByVolume(_ context.Context, _ string, names []string) (paneldocker.CommandResult, error) {
	f.removedByVolumes = append(f.removedByVolumes, names...)
	return paneldocker.CommandResult{ExitCode: 0}, nil
}

func (f *fakeDocker) ComposeRestartServices(_ context.Context, _ string, services ...string) (paneldocker.CommandResult, error) {
	f.restartedServices = append(f.restartedServices, services...)
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
	for _, want := range []string{
		"steam-auth:",
		"server:",
		"SERVER_IMAGE",
		"stdin_open: true",
		"tty: true",
		"steam-session:/data/steam-session",
		"game-data:/data/game",
		"./.local-container/saves:/config/xdg/config/StardewValley",
		"./.local-container/settings:/data/settings",
		"./.local-container/cont-env/APP_NAME:/etc/cont-env.d/APP_NAME:ro",
		"./.local-container/cont-env/DBUS_SESSION_BUS_ADDRESS:/etc/cont-env.d/DBUS_SESSION_BUS_ADDRESS:ro",
		"./.local-container/cont-groups/cinit/id:/etc/cont-groups.d/cinit/id:ro",
		"./.local-container/cont-users/root/home:/etc/cont-users.d/root/home:ro",
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
	driver := New(nil, nil, nil, nil)
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
	gotEnv, _ := os.ReadFile(filepath.Join(dataDir, ".env"))
	if string(gotEnv) != string(customEnv) {
		t.Error("Prepare should not overwrite existing .env")
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
	fake := &fakeDocker{
		psResult: paneldocker.ComposePsResult{
			Services: []paneldocker.ComposeService{{Service: "server", State: "running", Status: "Up 1 minute"}},
		},
	}
	store := &fakeStore{instance: storage.Instance{
		ID:            "stardew",
		DataDir:       "custom-dir",
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
	if fake.workDir != "custom-dir" {
		t.Fatalf("expected custom-dir workdir, got %q", fake.workDir)
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

func TestDriverInstallMarksSteamAuthFailedWhenRunErrors(t *testing.T) {
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
	fake := &fakeDocker{steamAuthErr: errors.New("docker run failed")}
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
	if updated.State != storage.InstanceStateSteamAuthFailed || updated.DriverPhase != "steam_auth_failed" {
		t.Fatalf("instance state not marked steam auth failed: state=%s phase=%s", updated.State, updated.DriverPhase)
	}
}

func TestDriverInstallFallsBackToSteamCMDAfterSuccessfulAuthDownloadFailure(t *testing.T) {
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
			"[SteamAuth:A0] Game license verified",
			"[SteamAuth:A0] Download failed: Download manifest failed across all CDN servers (403 Forbidden)",
			"[SteamService] Game download failed: Download manifest failed across all CDN servers (403 Forbidden)",
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
	updated, err := store.GetInstance(context.Background(), instance.ID)
	if err != nil {
		t.Fatalf("get instance: %v", err)
	}
	if updated.State != storage.InstanceStateGameInstalled || updated.DriverPhase != "game_installed" {
		t.Fatalf("instance state should be installed after steamcmd fallback: state=%s phase=%s", updated.State, updated.DriverPhase)
	}
	wantImage := steamCMDImageRefs(nil)[0]
	if fake.containerOpts.ImageRef != wantImage {
		t.Fatalf("expected SteamCMD fallback image %q, got %q", wantImage, fake.containerOpts.ImageRef)
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
	if !stringSliceContains(fake.containerOpts.Binds, storage.DefaultInstanceID+"_steamcmd-root-local:/root/.local/share/Steam") {
		t.Fatalf("SteamCMD should persist root self-update cache, binds=%v", fake.containerOpts.Binds)
	}
	if !stringSliceContains(fake.containerOpts.Binds, storage.DefaultInstanceID+"_steamcmd-user-local:/home/steam/.local/share/Steam") {
		t.Fatalf("SteamCMD should persist steam user self-update cache, binds=%v", fake.containerOpts.Binds)
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

func TestDriverAuthLoginOnlyMarksSteamAuthCompletedAndRefreshesService(t *testing.T) {
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
			"[SteamAuth:A0] Connecting... (1/5) +0.0s",
			"[SteamAuth:A0] Connected to Steam +4.7s",
			"[SteamAuth:A0] Logging in as 1517468252 with token (498 chars)... +0.0s",
			"[SteamAuth:A0] Token expires: 2027-02-02 11:30:38 UTC (210 days remaining) +0.0s",
			"[SteamAuth:A0] Logged in as [U:1:1231122837] +0.7s",
			"[SteamAuth:A0] Downloading app 413150... +0.0s",
			"[SteamAuth:A0] Download failed: Download manifest failed across all CDN servers (403 Forbidden). +3.6s",
			"[SteamService] Game download failed: Download manifest failed across all CDN servers (403 Forbidden). +12.6s",
		},
	}
	driver := New(fake, slog.Default(), manager, store)
	job, err := driver.Install(context.Background(), registry.InstallRequest{
		Instance:      registry.Instance{ID: instance.ID},
		SteamUsername: "steam-user",
		SteamPassword: "steam-pass",
		VNCPassword:   "vnc-pass",
		AuthLoginOnly: true,
	})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	waitForDriverTestJobStatus(t, store, job.ID, storage.JobStatusSucceeded)

	if !sjconfig.SteamAuthLoggedIn(instanceDir) {
		t.Fatal("expected STEAM_AUTH_COMPLETED=true after real steam-auth logged-in line")
	}
	if !reflect.DeepEqual(fake.restartedServices, []string{"steam-auth"}) {
		t.Fatalf("expected steam-auth service refresh, got %#v", fake.restartedServices)
	}
	if fake.containerRuns != 0 || fake.smapiRuns != 0 {
		t.Fatalf("auth-only must not run SteamCMD/SMAPI, containerRuns=%d smapiRuns=%d", fake.containerRuns, fake.smapiRuns)
	}
}

func TestDriverInstallRetriesSteamCMDAfterSegfaultClearsRuntimeCache(t *testing.T) {
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
		storage.DefaultInstanceID + "_steamcmd-user-local",
		storage.DefaultInstanceID + "_steamcmd-root-local",
	}
	for _, name := range wantVolumes {
		if !stringSliceContains(fake.removedByVolumes, name) {
			t.Fatalf("expected stale SteamCMD containers using volume %q to be removed, got %v", name, fake.removedByVolumes)
		}
		if !stringSliceContains(fake.removedVolumes, name) {
			t.Fatalf("expected SteamCMD runtime cache volume %q to be removed, got %v", name, fake.removedVolumes)
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

func TestServerImageRefsPrependsDefaultCandidatesToExistingSingleCandidate(t *testing.T) {
	envVals := map[string]string{
		"SERVER_IMAGE":            "sdvd/server:1.5.0-preview.121",
		"SERVER_IMAGE_CANDIDATES": "sdvd/server:1.5.0-preview.121",
	}
	refs := serverImageRefs(envVals, TestedImageTag)
	want := []string{
		"dockerproxy.net/sdvd/server:1.5.0-preview.121",
		"docker.1ms.run/sdvd/server:1.5.0-preview.121",
		"docker.1panel.live/sdvd/server:1.5.0-preview.121",
		"docker.jiaxin.site/sdvd/server:1.5.0-preview.121",
		"dockerproxy.link/sdvd/server:1.5.0-preview.121",
		"sdvd/server:1.5.0-preview.121",
	}
	if got := strings.Join(refs, ","); got != strings.Join(want, ",") {
		t.Fatalf("expected server image candidates %q, got %q", strings.Join(want, ","), got)
	}
}

func TestSMAPIDownloadURLsDefaultUseAccelerators(t *testing.T) {
	urls := smapiDownloadURLs(map[string]string{}, DefaultSMAPIVersion)
	want := []string{
		"https://gh.llkk.cc/https://github.com/Pathoschild/SMAPI/releases/download/4.5.2/SMAPI-4.5.2-installer.zip",
		"https://github.dpik.top/https://github.com/Pathoschild/SMAPI/releases/download/4.5.2/SMAPI-4.5.2-installer.zip",
		"https://ghfast.top/https://github.com/Pathoschild/SMAPI/releases/download/4.5.2/SMAPI-4.5.2-installer.zip",
		"https://github.com/Pathoschild/SMAPI/releases/download/4.5.2/SMAPI-4.5.2-installer.zip",
	}
	if got := strings.Join(urls, ","); got != strings.Join(want, ",") {
		t.Fatalf("expected SMAPI download URLs %q, got %q", strings.Join(want, ","), got)
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

func TestDriverInstallResumesSteamCMDDirectlyAfterSteamCMDFailure(t *testing.T) {
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

	manager := jobs.NewManager(store, slog.Default())
	fake := &fakeDocker{
		steamAuthErr: errors.New("steam-auth should not run"),
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
		t.Fatalf("steam-auth should be skipped on SteamCMD retry, ran %d times", fake.steamAuthRuns)
	}
	if fake.composePulls != 0 {
		t.Fatalf("Junimo compose pull should be skipped on SteamCMD retry, ran %d times", fake.composePulls)
	}
	if len(fake.pulledImages) != 0 {
		t.Fatalf("local SteamCMD image should not be pulled again, pulled %v", fake.pulledImages)
	}
	if fake.containerRuns != 1 {
		t.Fatalf("expected SteamCMD container to run once, ran %d times", fake.containerRuns)
	}
	updated, err := store.GetInstance(context.Background(), instance.ID)
	if err != nil {
		t.Fatalf("get instance: %v", err)
	}
	if updated.State != storage.InstanceStateGameInstalled || updated.DriverPhase != "game_installed" {
		t.Fatalf("instance state should be installed after direct steamcmd retry: state=%s phase=%s", updated.State, updated.DriverPhase)
	}
	// SteamCMD and steam-auth keep independent credentials: a SteamCMD login must
	// set only STEAMCMD_AUTH_COMPLETED, never STEAM_AUTH_COMPLETED.
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

func TestDriverInstallReRunsSteamAuthAfterPullFailureRetry(t *testing.T) {
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
		steamAuthLines: []string{"[SteamAuth:A0] Logged in as [U:1:1231122837]"},
	}
	driver := New(fake, slog.Default(), manager, store)
	// reuseCredentials retry (no re-input). Must re-pull images + run steam-auth,
	// NOT jump straight to the SteamCMD fallback.
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
	if fake.steamAuthRuns != 1 {
		t.Fatalf("steam-auth should run again after a pull failure, ran %d times", fake.steamAuthRuns)
	}
	if fake.containerRuns != 0 {
		t.Fatalf("SteamCMD fallback must not be entered directly on pull-failure retry, ran %d times", fake.containerRuns)
	}
}

func TestDriverInstallRepairUsesFullLoginAndAnonymousSDK(t *testing.T) {
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
	// Simulate an instance that has already authorized SteamCMD once. Repair still
	// skips steam-auth and the compose pull, but the game login is always a full
	// username+password login (this environment never caches a reusable
	// credential), and the SDK downloads anonymously.
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
	if !strings.Contains(command, `+login "$STEAM_USERNAME" "$STEAM_PASSWORD" +app_update 413150`) {
		t.Fatalf("repair game download should use a full username+password login, command=%q", command)
	}
	if !strings.Contains(command, `+login anonymous +app_update 1007`) {
		t.Fatalf("repair Steam SDK download should use anonymous login, command=%q", command)
	}
	if strings.Contains(command, `"$STEAM_USERNAME" +app_update 1007`) || strings.Contains(command, `"$STEAM_PASSWORD" +app_update 1007`) {
		t.Fatalf("Steam SDK download must not pass account credentials, command=%q", command)
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

	// The game (413150) always does a full username+password login: this SteamCMD
	// environment never caches a reusable credential, so a username-only login
	// would hang on "Cached credentials not found."
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

func waitForDriverTestJobStatus(t *testing.T, store *storage.Store, jobID string, status string) storage.Job {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
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
