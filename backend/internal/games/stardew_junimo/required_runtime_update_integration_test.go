//go:build integration

package stardew_junimo

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	appconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

// TestRequiredRuntimeReal121To125OptIn is the release-candidate acceptance for
// the exact supported legacy runtime. It never mutates the source instance or
// source volume: every subtest uses a copied instance directory, a cloned game
// volume, an empty steam-session volume, unique ports, and a unique Compose
// project. Steam credentials and tokens are blanked before the temporary .env
// is written.
func TestRequiredRuntimeReal121To125OptIn(t *testing.T) {
	sourceDir := strings.TrimSpace(os.Getenv("ANXI_REAL_UPGRADE_SOURCE_INSTANCE"))
	sourceGameVolume := strings.TrimSpace(os.Getenv("ANXI_REAL_UPGRADE_SOURCE_GAME_VOLUME"))
	if sourceDir == "" || sourceGameVolume == "" {
		t.Skip("set ANXI_REAL_UPGRADE_SOURCE_INSTANCE and ANXI_REAL_UPGRADE_SOURCE_GAME_VOLUME")
	}
	for _, initialState := range []string{storage.InstanceStateStopped, storage.InstanceStateRunning} {
		t.Run(initialState, func(t *testing.T) {
			runRequiredRuntimeRealUpgrade(t, sourceDir, sourceGameVolume, initialState)
		})
	}
}

// TestRequiredRuntimeRealControlUpgradeOptIn covers the Panel-upgrade case in
// which server/auth are already current but the running Control Mod reported by
// options.json is older than the embedded recommendation. Prepare synchronizes
// the new files, and the required runtime transaction must restart the exact
// same image pair so the game actually loads them.
func TestRequiredRuntimeRealControlUpgradeOptIn(t *testing.T) {
	sourceDir := strings.TrimSpace(os.Getenv("ANXI_REAL_CONTROL_UPGRADE_SOURCE_INSTANCE"))
	sourceGameVolume := strings.TrimSpace(os.Getenv("ANXI_REAL_CONTROL_UPGRADE_SOURCE_GAME_VOLUME"))
	if sourceDir == "" || sourceGameVolume == "" {
		t.Skip("set ANXI_REAL_CONTROL_UPGRADE_SOURCE_INSTANCE and ANXI_REAL_CONTROL_UPGRADE_SOURCE_GAME_VOLUME")
	}
	for _, initialState := range []string{storage.InstanceStateStopped, storage.InstanceStateRunning} {
		t.Run(initialState, func(t *testing.T) {
			runRequiredRuntimeRealUpgrade(t, sourceDir, sourceGameVolume, initialState)
		})
	}
}

// TestFreshInstall125ReachesSteamLoginOptIn verifies the first-install boundary
// without using a Steam account. Panel startup only prepares files; the real
// image pull begins after Install, and the test selects QR login then stops as
// soon as the auth container reaches the QR/login phase.
func TestFreshInstall125ReachesSteamLoginOptIn(t *testing.T) {
	if os.Getenv("ANXI_REAL_FRESH_INSTALL") != "1" {
		t.Skip("set ANXI_REAL_FRESH_INSTALL=1")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	suffix := strings.ToLower(strings.ReplaceAll(time.Now().UTC().Format("150405.000000"), ".", ""))
	project := "anxifreshinstall" + suffix
	dataDir := filepath.Join(os.TempDir(), project)
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = exec.Command("docker", "compose", "--project-name", project, "--project-directory", dataDir, "down", "--volumes", "--remove-orphans").Run()
		_ = exec.Command("docker", "volume", "rm", "-f", project+"_game-data", project+"_steam-session").Run()
		_ = os.RemoveAll(dataDir)
	})
	store, err := storage.Open(ctx, appconfig.Config{DataDir: dataDir, DBPath: filepath.Join(dataDir, "panel-e2e.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	stored, err := store.EnsureDefaultInstance(ctx, storage.EnsureDefaultInstanceParams{ID: "stardew", DriverID: DriverID, Name: "fresh install", DataDir: dataDir})
	if err != nil {
		t.Fatal(err)
	}
	stored, err = store.UpdateInstanceState(ctx, storage.UpdateInstanceStateParams{ID: stored.ID, State: storage.InstanceStateAdminCreated, DriverPhase: "empty", DriverPayload: "{}"})
	if err != nil {
		t.Fatal(err)
	}
	instance := registry.Instance{ID: stored.ID, DriverID: stored.DriverID, Name: stored.Name, DataDir: stored.DataDir, State: stored.State, DriverPhase: stored.DriverPhase, DriverPayload: stored.DriverPayload}
	client := paneldocker.NewClient(paneldocker.Options{DockerPath: "docker"})
	manager := jobs.NewManager(store, slog.Default())
	driver := New(client, slog.Default(), manager, store, "0.3.5")
	if err := driver.Prepare(ctx, instance); err != nil {
		t.Fatal(err)
	}
	if err := sjconfig.UpdateEnvFile(filepath.Join(dataDir, ".env"), map[string]string{"GAME_DATA_VOLUME": project + "_game-data"}); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.CommandContext(ctx, "docker", "ps", "-aq", "--filter", "label=com.docker.compose.project="+project).CombinedOutput(); err != nil || strings.TrimSpace(string(output)) != "" {
		t.Fatalf("fresh Panel startup created game containers before Install: %v %s", err, output)
	}
	for _, volume := range []string{project + "_game-data", project + "_steam-session"} {
		if err := exec.CommandContext(ctx, "docker", "volume", "inspect", volume).Run(); err == nil {
			t.Fatalf("fresh Panel startup created volume %s before Install", volume)
		}
	}

	job, err := driver.Install(ctx, registry.InstallRequest{
		Instance:      instance,
		SteamUsername: "qr-login-placeholder",
		SteamPassword: "qr-login-placeholder",
		VNCPassword:   "fresh-install-e2e",
	})
	if err != nil {
		t.Fatal(err)
	}
	waitPhase := func(wanted string, timeout time.Duration) storage.Instance {
		deadline := time.Now().Add(timeout)
		var current storage.Instance
		for time.Now().Before(deadline) {
			current, err = store.GetInstance(ctx, stored.ID)
			if err == nil && current.DriverPhase == wanted {
				return current
			}
			if err == nil && (current.DriverPhase == "pull_failed" || current.DriverPhase == "steam_auth_failed") {
				t.Fatalf("fresh install failed before %s: state=%s phase=%s message=%s", wanted, current.State, current.DriverPhase, current.StateMessage.String)
			}
			time.Sleep(time.Second)
		}
		t.Fatalf("fresh install did not reach %s; last state=%s phase=%s message=%s", wanted, current.State, current.DriverPhase, current.StateMessage.String)
		return current
	}
	waitPhase("auth_method_required", 5*time.Minute)
	env, err := sjconfig.ReadEnvFile(filepath.Join(dataDir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if env["IMAGE_VERSION"] != TestedImageTag || !strings.Contains(env["SERVER_IMAGE"], TestedImageTag) || !strings.Contains(env["STEAM_SERVICE_IMAGE"], "1.5.0-anxi.2") {
		t.Fatalf("fresh install did not select exact 125/auth pair: IMAGE_VERSION=%q SERVER_IMAGE=%q STEAM_SERVICE_IMAGE=%q", env["IMAGE_VERSION"], env["SERVER_IMAGE"], env["STEAM_SERVICE_IMAGE"])
	}
	if err := driver.SendSteamGuardInput(job.ID, "2"); err != nil {
		t.Fatal(err)
	}
	waitPhase("steam_qr_required", 3*time.Minute)
	if err := manager.Cancel(context.Background(), job.ID); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); {
		current, getErr := store.GetJob(context.Background(), job.ID)
		if getErr == nil && current.Status != storage.JobStatusQueued && current.Status != storage.JobStatusRunning {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); {
		output, psErr := exec.Command("docker", "ps", "-aq", "--filter", "volume="+project+"_game-data").CombinedOutput()
		if psErr == nil && strings.TrimSpace(string(output)) == "" {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatal("Steam login container did not stop after install job cancellation")
}

func runRequiredRuntimeRealUpgrade(t *testing.T, sourceDir, sourceGameVolume, initialState string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Minute)
	defer cancel()
	suffix := strings.ToLower(strings.ReplaceAll(time.Now().UTC().Format("150405.000000"), ".", ""))
	project := "anxirealupgrade" + suffix
	dataDir := filepath.Join(os.TempDir(), project)
	gameVolume := project + "_game-data"
	steamVolume := project + "_steam-session"
	run := func(args ...string) string {
		output, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
		if err != nil {
			t.Fatalf("docker %v: %v: %s", args, err, output)
		}
		return string(output)
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = exec.Command("docker", "compose", "--project-name", project, "--project-directory", dataDir, "down", "--volumes", "--remove-orphans").Run()
		_ = exec.Command("docker", "volume", "rm", "-f", gameVolume, steamVolume).Run()
		_ = os.RemoveAll(dataDir)
	})
	if err := copyRuntimeFixture(sourceDir, dataDir); err != nil {
		t.Fatal(err)
	}
	rawEnv, err := os.ReadFile(filepath.Join(sourceDir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	envPath := filepath.Join(dataDir, ".env")
	if err := os.WriteFile(envPath, sanitizeRealUpgradeEnv(rawEnv), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := sjconfig.UpdateEnvFile(envPath, map[string]string{
		"GAME_DATA_VOLUME":    gameVolume,
		"GAME_PORT":           "0",
		"QUERY_PORT":          "0",
		"VNC_PORT":            "0",
		"API_PORT":            "8080",
		"STEAM_AUTH_PORT":     "3001",
		"STEAM_USERNAME":      "",
		"STEAM_PASSWORD":      "",
		"STEAM_REFRESH_TOKEN": "",
	}); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("ANXI_REAL_UPGRADE_FORCE_121") == "1" {
		run("pull", "dockerproxy.net/sdvd/server:1.5.0-preview.121")
		if err := sjconfig.UpdateEnvFile(envPath, map[string]string{
			"IMAGE_VERSION":           "1.5.0-preview.121",
			"SERVER_IMAGE":            "dockerproxy.net/sdvd/server:1.5.0-preview.121",
			"SERVER_IMAGE_CANDIDATES": "dockerproxy.net/sdvd/server:1.5.0-preview.121,sdvd/server:1.5.0-preview.121,docker.m.daocloud.io/sdvd/server:1.5.0-preview.121,ghcr.io/sdvd/server:1.5.0-preview.121",
		}); err != nil {
			t.Fatal(err)
		}
	}
	run("volume", "create", gameVolume)
	run("volume", "create", steamVolume)
	run("run", "--rm", "--network", "none",
		"--mount", "type=volume,src="+sourceGameVolume+",dst=/source,readonly",
		"--mount", "type=volume,src="+gameVolume+",dst=/target",
		"alpine:3.20", "sh", "-c", "cd /source && tar cf - . | tar xf - -C /target")
	if initialState == storage.InstanceStateRunning {
		run("compose", "--project-name", project, "--project-directory", dataDir, "up", "-d")
	}

	store, err := storage.Open(ctx, appconfig.Config{DataDir: dataDir, DBPath: filepath.Join(dataDir, "panel-e2e.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	stored, err := store.EnsureDefaultInstance(ctx, storage.EnsureDefaultInstanceParams{ID: "stardew", DriverID: DriverID, Name: "real upgrade", DataDir: dataDir})
	if err != nil {
		t.Fatal(err)
	}
	stored, err = store.UpdateInstanceState(ctx, storage.UpdateInstanceStateParams{ID: stored.ID, State: initialState, DriverPhase: "ready", DriverPayload: "{}"})
	if err != nil {
		t.Fatal(err)
	}
	instance := registry.Instance{ID: stored.ID, DriverID: stored.DriverID, Name: stored.Name, DataDir: stored.DataDir, State: stored.State, DriverPhase: stored.DriverPhase, DriverPayload: stored.DriverPayload}
	client := paneldocker.NewClient(paneldocker.Options{DockerPath: "docker"})
	driver := New(client, slog.Default(), jobs.NewManager(store, slog.Default()), store, "0.3.5")
	if err := driver.Prepare(ctx, instance); err != nil {
		t.Fatalf("prepare real legacy instance with current embedded Control: %v", err)
	}
	driver.StartRequiredRuntimeUpdate(ctx, instance)

	var required RequiredRuntimeUpdateStatus
	for deadline := time.Now().Add(30 * time.Minute); time.Now().Before(deadline); {
		required, err = readRequiredRuntimeUpdateStatus(dataDir)
		if err == nil && (required.Phase == requiredRuntimePhaseSucceeded || required.Phase == requiredRuntimePhaseFailed || required.Phase == requiredRuntimePhaseManual) {
			break
		}
		time.Sleep(2 * time.Second)
	}
	if required.Phase != requiredRuntimePhaseSucceeded {
		apply, _ := driver.RuntimeUpdateApplyStatus(instance)
		t.Fatalf("required upgrade phase=%s code=%s error=%s apply=%+v", required.Phase, required.ErrorCode, required.Error, apply)
	}
	inspection := InspectRuntimeStack(dataDir, initialState)
	if inspection.Status != sjconfig.RuntimeStackStatusUpToDate || inspection.Current.Server.Tag != TestedImageTag {
		t.Fatalf("runtime inspection after upgrade=%+v", inspection)
	}
	manifestData, err := os.ReadFile(filepath.Join(junimoServerModDir(dataDir), junimoServerManifestName))
	if err != nil || !strings.Contains(string(manifestData), TestedImageTag) {
		t.Fatalf("host JunimoServer manifest does not contain %s: %v %s", TestedImageTag, err, manifestData)
	}
	runtimeManifest, err := sjconfig.BuiltInRuntimeStackManifest()
	if err != nil {
		t.Fatal(err)
	}
	optionsData, err := os.ReadFile(filepath.Join(controlDir(dataDir), "options.json"))
	if err != nil {
		t.Fatalf("read real Control options: %v", err)
	}
	var controlOptions struct {
		ControlModVersion string `json:"controlModVersion"`
	}
	if err := json.Unmarshal(optionsData, &controlOptions); err != nil {
		t.Fatalf("parse real Control options: %v", err)
	}
	if controlOptions.ControlModVersion != runtimeManifest.Control.Version {
		t.Fatalf("real Control version=%s want=%s", controlOptions.ControlModVersion, runtimeManifest.Control.Version)
	}
	composeData, err := os.ReadFile(filepath.Join(dataDir, "docker-compose.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(composeData), "cpu_shares: 256") || !strings.Contains(string(composeData), "cpu_shares: 768") {
		t.Fatalf("runtime compose did not receive low-resource CPU shares: %s", composeData)
	}
	ps := run("compose", "--project-name", project, "--project-directory", dataDir, "ps", "--format", "json", "--all")
	if initialState == storage.InstanceStateRunning && !strings.Contains(strings.ToLower(ps), "running") {
		t.Fatalf("running state was not restored: %s", ps)
	}
	if initialState == storage.InstanceStateRunning {
		serverShares := strings.TrimSpace(run("inspect", "--format", "{{.HostConfig.CpuShares}}", project+"-server-1"))
		authShares := strings.TrimSpace(run("inspect", "--format", "{{.HostConfig.CpuShares}}", project+"-steam-auth-1"))
		if serverShares != "768" || authShares != "256" {
			t.Fatalf("runtime CPU shares server=%s steam-auth=%s", serverShares, authShares)
		}
	}
	if initialState == storage.InstanceStateStopped && strings.Contains(strings.ToLower(ps), `"state":"running"`) {
		t.Fatalf("stopped state was not restored: %s", ps)
	}
}

func copyRuntimeFixture(sourceDir, targetDir string) error {
	return filepath.WalkDir(sourceDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		if rel == ".env" || strings.HasPrefix(rel, filepath.Join(".local-container", "junimo-update")) || strings.HasPrefix(rel, filepath.Join(".local-container", "smapi-update")) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(targetDir, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o600)
	})
}

func sanitizeRealUpgradeEnv(data []byte) []byte {
	sensitive := map[string]bool{
		"STEAM_USERNAME": true, "STEAM_PASSWORD": true, "STEAM_REFRESH_TOKEN": true,
		"VNC_PASSWORD": true, "API_KEY": true, "SERVER_PASSWORD": true,
		"DISCORD_BOT_TOKEN": true,
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	for index, line := range lines {
		key, _, found := strings.Cut(line, "=")
		if found && sensitive[strings.TrimSpace(key)] {
			lines[index] = fmt.Sprintf("%s=", strings.TrimSpace(key))
		}
	}
	return []byte(strings.Join(lines, "\n"))
}
