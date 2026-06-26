package stardew_junimo

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

type fakeDocker struct {
	workDir       string
	psResult      paneldocker.ComposePsResult
	psErr         error
	pullResult    paneldocker.CommandResult
	pullErr       error
	inspectResult paneldocker.CommandResult
	inspectErr    error
}

func (f *fakeDocker) ComposePs(ctx context.Context, dir string) (paneldocker.ComposePsResult, error) {
	f.workDir = dir
	return f.psResult, f.psErr
}

func (f *fakeDocker) ComposePullStreaming(_ context.Context, _ string, _ []string, lineHandler func(string)) (paneldocker.CommandResult, error) {
	if f.pullErr == nil {
		lineHandler("steam-auth Pulling")
		lineHandler("server Pulling")
	}
	return f.pullResult, f.pullErr
}

func (f *fakeDocker) ImageInspect(ctx context.Context, dir string, imageRef string) (paneldocker.CommandResult, error) {
	return f.inspectResult, f.inspectErr
}

func (f *fakeDocker) RunSteamAuthTTY(_ context.Context, _ string, _ paneldocker.SteamAuthRunOpts, _ <-chan string, _ func(string)) (int, error) {
	return 0, nil
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
	for _, sub := range []string{"saves", "mods", ".local-container", filepath.Join(".local-container", "settings")} {
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
		"IMAGE_VERSION",
		"stdin_open: true",
		"tty: true",
		"steam-session:/data/steam-session",
		"game-data:/data/game",
		"saves:/config/xdg/config/StardewValley",
		"./.local-container/settings:/data/settings",
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

func TestDriverSendSteamGuardInput_NoActiveJob(t *testing.T) {
	driver := New(nil, nil, nil, nil)
	err := driver.SendSteamGuardInput("nonexistent-job-id", "12345")
	if err == nil {
		t.Fatal("expected error for nonexistent job")
	}
}
