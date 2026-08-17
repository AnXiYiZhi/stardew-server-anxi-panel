//go:build integration

package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

// TestRealExistingSavePlayerLimitRestartOptIn proves the end-to-end contract on
// a clone of a real existing save. The source instance directory and game-data
// volume are read-only; every write and container belongs to a unique project.
func TestRealExistingSavePlayerLimitRestartOptIn(t *testing.T) {
	sourceInstanceDir := strings.TrimSpace(os.Getenv("ANXI_REAL_MAX_PLAYERS_SOURCE_INSTANCE"))
	sourceGameVolume := strings.TrimSpace(os.Getenv("ANXI_REAL_MAX_PLAYERS_SOURCE_GAME_VOLUME"))
	if sourceInstanceDir == "" || sourceGameVolume == "" {
		t.Skip("set ANXI_REAL_MAX_PLAYERS_SOURCE_INSTANCE and ANXI_REAL_MAX_PLAYERS_SOURCE_GAME_VOLUME")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Minute)
	defer cancel()
	suffix := strings.ToLower(strings.ReplaceAll(time.Now().UTC().Format("150405.000000"), ".", ""))
	project := "anxirealmaxplayers" + suffix
	instanceDir := filepath.Join(os.TempDir(), project)
	panelDir := t.TempDir()
	gameVolume := project + "_game-data"
	steamVolume := project + "_steam-session"
	t.Logf("real max-players isolated project=%s", project)

	runDocker := func(args ...string) string {
		t.Helper()
		output, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
		if err != nil {
			t.Fatalf("docker command failed: %v: %s", err, paneldocker.RedactString(string(output)))
		}
		return string(output)
	}
	if err := os.MkdirAll(instanceDir, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cleanupCancel()
		_ = exec.CommandContext(cleanupCtx, "docker", "compose", "--project-name", project, "--project-directory", instanceDir, "down", "--volumes", "--remove-orphans").Run()
		_ = exec.CommandContext(cleanupCtx, "docker", "volume", "rm", "-f", gameVolume, steamVolume).Run()
		_ = os.RemoveAll(instanceDir)
	})

	if err := copyRealMaxPlayersFixture(sourceInstanceDir, instanceDir); err != nil {
		t.Fatal(err)
	}
	rawEnv, err := os.ReadFile(filepath.Join(sourceInstanceDir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	envPath := filepath.Join(instanceDir, ".env")
	if err := os.WriteFile(envPath, sanitizeRealMaxPlayersEnv(rawEnv), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := sjconfig.UpdateEnvFile(envPath, map[string]string{
		"COMPOSE_PROJECT_NAME": project,
		"GAME_DATA_VOLUME":     gameVolume,
		"GAME_PORT":            "0",
		"QUERY_PORT":           "0",
		"VNC_PORT":             "0",
		"API_PORT":             reserveRealMaxPlayersPort(t),
		"VNC_PASSWORD":         "real-max-players-e2e",
		"STEAM_USERNAME":       "",
		"STEAM_PASSWORD":       "",
		"STEAM_REFRESH_TOKEN":  "",
	}); err != nil {
		t.Fatal(err)
	}
	composeConfigRaw := runDocker("compose", "--project-directory", instanceDir, "config", "--format", "json")
	var composeConfig struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(composeConfigRaw), &composeConfig); err != nil {
		t.Fatalf("decode cloned Compose project identity: %v", err)
	}
	if composeConfig.Name != project {
		t.Fatalf("cloned Compose project=%q, want isolated project %q", composeConfig.Name, project)
	}
	runDocker("volume", "create", "--label", "com.anxi.task=server-runtime-maxplayers", gameVolume)
	runDocker("volume", "create", "--label", "com.anxi.task=server-runtime-maxplayers", steamVolume)
	runDocker("run", "--rm", "--network", "none",
		"--mount", "type=volume,src="+sourceGameVolume+",dst=/source,readonly",
		"--mount", "type=volume,src="+gameVolume+",dst=/target",
		"alpine:3.20", "sh", "-c", "cd /source && tar cf - . | tar xf - -C /target")

	store, err := storage.Open(ctx, config.Config{DataDir: panelDir, DBPath: filepath.Join(panelDir, "panel-e2e.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	stored, err := store.EnsureDefaultInstance(ctx, storage.EnsureDefaultInstanceParams{
		ID: storage.DefaultInstanceID, DriverID: sj.DriverID, Name: "real max players", DataDir: instanceDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	stored, err = store.UpdateInstanceState(ctx, storage.UpdateInstanceStateParams{
		ID: stored.ID, State: storage.InstanceStateStopped, StateMessage: "real fixture stopped", DriverPhase: "stopped", DriverPayload: "{}",
	})
	if err != nil {
		t.Fatal(err)
	}

	client := paneldocker.NewClient(paneldocker.Options{DockerPath: "docker"})
	manager := jobs.NewManager(store, slog.Default())
	driver := sj.New(client, slog.Default(), manager, store, "0.4.12")
	instance := registry.Instance{
		ID: stored.ID, DriverID: stored.DriverID, Name: stored.Name, DataDir: stored.DataDir,
		State: stored.State, DriverPhase: stored.DriverPhase, DriverPayload: stored.DriverPayload,
	}
	if err := driver.Prepare(ctx, instance); err != nil {
		t.Fatalf("prepare cloned existing instance: %v", err)
	}
	drivers := registry.New()
	if err := drivers.Register(driver); err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(Deps{
		Config: config.Config{DataDir: panelDir, Secret: "real-max-players-secret", Version: "0.4.12"},
		Store:  store, Docker: client, Jobs: manager, Registry: drivers, Logger: slog.Default(),
	})
	adminCookie := setupDockerAdmin(t, handler)

	settingsResponse, _ := doJSON(t, handler, http.MethodGet, "/api/instances/stardew/config/server-runtime-settings", nil, adminCookie)
	if settingsResponse.Code != http.StatusOK {
		t.Fatalf("GET runtime settings returned %d: %s", settingsResponse.Code, settingsResponse.Body.String())
	}
	var settings sj.ServerRuntimeSettings
	if err := json.Unmarshal(settingsResponse.Body.Bytes(), &settings); err != nil {
		t.Fatal(err)
	}
	baseline, target := 11, 12
	settings.MaxPlayers = &baseline
	putBaseline, _ := doJSON(t, handler, http.MethodPut, "/api/instances/stardew/config/server-runtime-settings", settings, adminCookie)
	if putBaseline.Code != http.StatusOK {
		t.Fatalf("PUT baseline maxPlayers returned %d: %s", putBaseline.Code, putBaseline.Body.String())
	}

	startResponse, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/start", nil, adminCookie)
	if startResponse.Code != http.StatusAccepted {
		t.Fatalf("start existing save returned %d: %s", startResponse.Code, startResponse.Body.String())
	}
	var started struct {
		JobID string `json:"jobId"`
	}
	if err := json.Unmarshal(startResponse.Body.Bytes(), &started); err != nil || started.JobID == "" {
		t.Fatalf("decode start job: id=%q err=%v", started.JobID, err)
	}
	waitRealMaxPlayersJob(t, ctx, store, started.JobID, 15*time.Minute)
	waitRealMaxPlayersAPIValue(t, ctx, handler, adminCookie, baseline, 3*time.Minute)

	settings.MaxPlayers = &target
	putTarget, _ := doJSON(t, handler, http.MethodPut, "/api/instances/stardew/config/server-runtime-settings", settings, adminCookie)
	if putTarget.Code != http.StatusOK {
		t.Fatalf("PUT running maxPlayers returned %d: %s", putTarget.Code, putTarget.Body.String())
	}
	playersBefore := getRealMaxPlayers(t, handler, adminCookie)
	if playersBefore.MaxPlayers == nil || *playersBefore.MaxPlayers != baseline {
		t.Fatalf("running maxPlayers before restart=%v, want current live value %d", playersBefore.MaxPlayers, baseline)
	}

	jobsBefore, err := store.ListJobs(ctx, storage.ListJobsFilter{IsAdmin: true, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	knownJobs := make(map[string]bool, len(jobsBefore))
	for _, job := range jobsBefore {
		knownJobs[job.ID] = true
	}
	restartResponse, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/restart", nil, adminCookie)
	if restartResponse.Code != http.StatusAccepted {
		t.Fatalf("restart existing save returned %d: %s", restartResponse.Code, restartResponse.Body.String())
	}
	restartJobID := waitRealMaxPlayersNewLifecycleJob(t, ctx, store, knownJobs, 30*time.Second)
	waitRealMaxPlayersJob(t, ctx, store, restartJobID, 15*time.Minute)
	waitRealMaxPlayersAPIValue(t, ctx, handler, adminCookie, target, 3*time.Minute)

	configuredResponse, _ := doJSON(t, handler, http.MethodGet, "/api/instances/stardew/config/server-runtime-settings", nil, adminCookie)
	if configuredResponse.Code != http.StatusOK {
		t.Fatalf("GET configured settings after restart returned %d: %s", configuredResponse.Code, configuredResponse.Body.String())
	}
	var configured sj.ServerRuntimeSettings
	if err := json.Unmarshal(configuredResponse.Body.Bytes(), &configured); err != nil {
		t.Fatal(err)
	}
	if configured.MaxPlayers == nil || *configured.MaxPlayers != target {
		t.Fatalf("configured maxPlayers after restart=%v, want %d", configured.MaxPlayers, target)
	}
}

func waitRealMaxPlayersJob(t *testing.T, ctx context.Context, store *storage.Store, jobID string, timeout time.Duration) {
	t.Helper()
	var current storage.Job
	for deadline := time.Now().Add(timeout); time.Now().Before(deadline); {
		job, err := store.GetJob(ctx, jobID)
		if err != nil {
			t.Fatal(err)
		}
		current = job
		if job.Status != storage.JobStatusQueued && job.Status != storage.JobStatusRunning {
			if job.Status != storage.JobStatusSucceeded {
				t.Fatalf("lifecycle job failed: status=%s error=%s", job.Status, paneldocker.RedactString(job.ErrorMessage.String))
			}
			return
		}
		time.Sleep(time.Second)
	}
	t.Fatalf("lifecycle job %s timed out with status=%s", jobID, current.Status)
}

func waitRealMaxPlayersNewLifecycleJob(t *testing.T, ctx context.Context, store *storage.Store, known map[string]bool, timeout time.Duration) string {
	t.Helper()
	for deadline := time.Now().Add(timeout); time.Now().Before(deadline); {
		listed, err := store.ListJobs(ctx, storage.ListJobsFilter{IsAdmin: true, Limit: 100})
		if err != nil {
			t.Fatal(err)
		}
		for _, job := range listed {
			if job.Type == "stardew_lifecycle" && !known[job.ID] && strings.Contains(job.Payload.String, `"operation":"restart"`) {
				return job.ID
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatal("restart lifecycle job was not created")
	return ""
}

func waitRealMaxPlayersAPIValue(t *testing.T, ctx context.Context, handler http.Handler, cookie *http.Cookie, want int, timeout time.Duration) {
	t.Helper()
	for deadline := time.Now().Add(timeout); time.Now().Before(deadline); {
		if err := ctx.Err(); err != nil {
			t.Fatal(err)
		}
		players := getRealMaxPlayers(t, handler, cookie)
		if players.MaxPlayers != nil && *players.MaxPlayers == want {
			return
		}
		time.Sleep(time.Second)
	}
	t.Fatalf("players API did not report maxPlayers=%d", want)
}

func getRealMaxPlayers(t *testing.T, handler http.Handler, cookie *http.Cookie) sj.PlayersResult {
	t.Helper()
	response, _ := doJSON(t, handler, http.MethodGet, "/api/instances/stardew/players", nil, cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("GET players returned %d: %s", response.Code, response.Body.String())
	}
	var players sj.PlayersResult
	if err := json.Unmarshal(response.Body.Bytes(), &players); err != nil {
		t.Fatal(err)
	}
	return players
}

func reserveRealMaxPlayersPort(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return strconv.Itoa(port)
}

func copyRealMaxPlayersFixture(sourceDir, targetDir string) error {
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
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("fixture contains unsupported symlink")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o600)
	})
}

func sanitizeRealMaxPlayersEnv(data []byte) []byte {
	sensitive := map[string]bool{
		"STEAM_USERNAME": true, "STEAM_PASSWORD": true, "STEAM_REFRESH_TOKEN": true,
		"VNC_PASSWORD": true, "API_KEY": true, "SERVER_PASSWORD": true,
		"DISCORD_BOT_TOKEN": true,
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	for index, line := range lines {
		key, _, found := strings.Cut(line, "=")
		if found && sensitive[strings.TrimSpace(key)] {
			lines[index] = strings.TrimSpace(key) + "="
		}
	}
	return []byte(strings.Join(lines, "\n"))
}
