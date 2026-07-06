package web

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

type fakeDockerService struct {
	versionResult paneldocker.CommandResult
	versionErr    error
	composeErr    error
	psResult      paneldocker.ComposePsResult
	psErr         error
	statsResult   paneldocker.ComposeStatsResult
	statsErr      error
	logsResult    paneldocker.CommandResult
	logsErr       error
	execFunc      func(ctx context.Context, dir, service, stdinData string, args ...string) (paneldocker.CommandResult, error)
	instanceState string
}

func (f fakeDockerService) DockerVersion(ctx context.Context, workDir string) (paneldocker.CommandResult, error) {
	return f.versionResult, f.versionErr
}

func (f fakeDockerService) ComposeVersion(ctx context.Context, workDir string) (paneldocker.CommandResult, error) {
	return f.versionResult, f.composeErr
}

func (f fakeDockerService) ComposePs(ctx context.Context, dir string) (paneldocker.ComposePsResult, error) {
	return f.psResult, f.psErr
}

func (f fakeDockerService) ComposeStats(ctx context.Context, dir string) (paneldocker.ComposeStatsResult, error) {
	return f.statsResult, f.statsErr
}

func (f fakeDockerService) ComposeLogs(ctx context.Context, dir string, opts paneldocker.LogsOptions) (paneldocker.CommandResult, error) {
	return f.logsResult, f.logsErr
}

func (f fakeDockerService) ComposePullStreaming(ctx context.Context, dir string, services []string, lineHandler func(line string)) (paneldocker.CommandResult, error) {
	return paneldocker.CommandResult{ExitCode: 0}, nil
}

func (f fakeDockerService) PullImageStreaming(ctx context.Context, dir string, imageRef string, lineHandler func(line string)) (paneldocker.CommandResult, error) {
	return paneldocker.CommandResult{ExitCode: 0}, nil
}

func (f fakeDockerService) ImageInspect(ctx context.Context, dir string, imageRef string) (paneldocker.CommandResult, error) {
	return paneldocker.CommandResult{ExitCode: 0}, nil
}

func (f fakeDockerService) RunSteamAuthTTY(ctx context.Context, dataDir string, opts paneldocker.SteamAuthRunOpts, guardCh <-chan string, lineHandler func(string)) (int, error) {
	return 0, nil
}

func (f fakeDockerService) RunContainerTTY(ctx context.Context, opts paneldocker.ContainerTTYRunOpts, guardCh <-chan string, lineHandler func(string)) (int, error) {
	return 0, nil
}

func (f fakeDockerService) RemoveContainersByVolume(ctx context.Context, workDir string, names []string) (paneldocker.CommandResult, error) {
	return paneldocker.CommandResult{ExitCode: 0}, nil
}

func (f fakeDockerService) RemoveVolumes(ctx context.Context, workDir string, names []string) (paneldocker.CommandResult, error) {
	return paneldocker.CommandResult{ExitCode: 0}, nil
}

func (f fakeDockerService) ComposeUp(ctx context.Context, dir string) (paneldocker.CommandResult, error) {
	return paneldocker.CommandResult{ExitCode: 0}, nil
}

func (f fakeDockerService) ComposeDown(ctx context.Context, dir string) (paneldocker.CommandResult, error) {
	return paneldocker.CommandResult{ExitCode: 0}, nil
}

func (f fakeDockerService) ComposeRestart(ctx context.Context, dir string) (paneldocker.CommandResult, error) {
	return paneldocker.CommandResult{ExitCode: 0}, nil
}

func (f fakeDockerService) ComposeRestartServices(ctx context.Context, dir string, services ...string) (paneldocker.CommandResult, error) {
	return paneldocker.CommandResult{ExitCode: 0}, nil
}

func (f fakeDockerService) ComposeExecPipe(ctx context.Context, dir, service, stdinData string, args ...string) (paneldocker.CommandResult, error) {
	if f.execFunc != nil {
		return f.execFunc(ctx, dir, service, stdinData, args...)
	}
	return paneldocker.CommandResult{ExitCode: 0}, nil
}

func (f fakeDockerService) ComposeExecTTY(ctx context.Context, dir, service, stdinData string, args ...string) (paneldocker.ComposeExecTTYResult, error) {
	return paneldocker.ComposeExecTTYResult{ExitCode: 0}, nil
}

func TestDockerStatusRequiresAdmin(t *testing.T) {
	handler, _, closeStore := newDockerTestHandler(t, fakeDockerService{})
	defer closeStore()

	unauthorized, _ := doJSON(t, handler, http.MethodGet, "/api/docker/status", nil, nil)
	if unauthorized.Code != http.StatusServiceUnavailable {
		t.Fatalf("docker status before setup returned %d", unauthorized.Code)
	}

	adminCookie := setupDockerAdmin(t, handler)
	created, _ := doJSON(t, handler, http.MethodPost, "/api/users", map[string]string{
		"username": "player",
		"password": "123456",
		"role":     "user",
	}, adminCookie)
	if created.Code != http.StatusCreated {
		t.Fatalf("create player returned %d: %s", created.Code, created.Body.String())
	}
	login, playerCookie := doJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "player",
		"password": "123456",
	}, nil)
	if login.Code != http.StatusOK {
		t.Fatalf("player login returned %d", login.Code)
	}

	forbidden, _ := doJSON(t, handler, http.MethodGet, "/api/docker/status", nil, playerCookie)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("docker status for player returned %d", forbidden.Code)
	}
}

func TestDockerStatusForAdmin(t *testing.T) {
	fake := fakeDockerService{versionResult: paneldocker.CommandResult{ExitCode: 0, Stdout: "ok"}}
	handler, _, closeStore := newDockerTestHandler(t, fake)
	defer closeStore()
	adminCookie := setupDockerAdmin(t, handler)

	response, _ := doJSON(t, handler, http.MethodGet, "/api/docker/status", nil, adminCookie)
	if response.Code != http.StatusOK {
		t.Fatalf("docker status returned %d: %s", response.Code, response.Body.String())
	}
}

func TestDockerPsProjectNotReady(t *testing.T) {
	handler, _, closeStore := newDockerTestHandler(t, fakeDockerService{})
	defer closeStore()
	adminCookie := setupDockerAdmin(t, handler)

	response, _ := doJSON(t, handler, http.MethodGet, "/api/docker/ps", nil, adminCookie)
	if response.Code != http.StatusConflict {
		t.Fatalf("docker ps without compose project returned %d", response.Code)
	}
}

func TestDockerPsSuccess(t *testing.T) {
	fake := fakeDockerService{psResult: paneldocker.ComposePsResult{
		Result:   paneldocker.CommandResult{ExitCode: 0, Stdout: "[]"},
		Services: []paneldocker.ComposeService{{Name: "demo", Service: "app", State: "running"}},
	}}
	handler, dataDir, closeStore := newDockerTestHandler(t, fake)
	defer closeStore()
	adminCookie := setupDockerAdmin(t, handler)
	prepareComposeProject(t, dataDir)

	response, _ := doJSON(t, handler, http.MethodGet, "/api/docker/ps", nil, adminCookie)
	if response.Code != http.StatusOK {
		t.Fatalf("docker ps returned %d: %s", response.Code, response.Body.String())
	}
}

func TestDockerLogsValidatesQuery(t *testing.T) {
	handler, dataDir, closeStore := newDockerTestHandler(t, fakeDockerService{})
	defer closeStore()
	adminCookie := setupDockerAdmin(t, handler)
	prepareComposeProject(t, dataDir)

	response, _ := doJSON(t, handler, http.MethodGet, "/api/docker/logs?tail=999999", nil, adminCookie)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("docker logs invalid tail returned %d", response.Code)
	}
}

func TestInstanceMetricsReturnsStatsAndDisk(t *testing.T) {
	fake := fakeDockerService{statsResult: paneldocker.ComposeStatsResult{
		Result: paneldocker.CommandResult{ExitCode: 0, Stdout: "{}"},
		Services: []paneldocker.ComposeServiceStats{{
			Name:             "demo-server-1",
			Service:          "server",
			CPUPerc:          125.4,
			MemPerc:          45.6,
			MemUsedBytes:     512,
			MemLimitBytes:    1024,
			RawCPUPerc:       "125.4%",
			RawMemPerc:       "45.6%",
			RawMemUsedBytes:  "512B",
			RawMemLimitBytes: "1KiB",
		}},
	}}
	handler, dataDir, closeStore := newDockerTestHandler(t, fake)
	defer closeStore()
	adminCookie := setupDockerAdmin(t, handler)
	prepareComposeProject(t, dataDir)

	response, _ := doJSON(t, handler, http.MethodGet, "/api/instances/stardew/metrics", nil, adminCookie)
	if response.Code != http.StatusOK {
		t.Fatalf("metrics returned %d: %s", response.Code, response.Body.String())
	}
	var body resourceMetricsResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode metrics response: %v", err)
	}
	if body.Sample.CPUPercent == nil || *body.Sample.CPUPercent != 125.4 {
		t.Fatalf("metrics response should preserve raw CPU percent above 100: %s", response.Body.String())
	}
	if body.Sample.MemoryPercent == nil || *body.Sample.MemoryPercent != 45.6 {
		t.Fatalf("metrics response did not include memory stats: %s", response.Body.String())
	}
	if body.Sample.ContainerRunning != true || body.Service != "server" {
		t.Fatalf("metrics response did not mark server stats correctly: %+v", body)
	}
}

func TestInstanceMetricsDoesNotFallbackToNonServerStats(t *testing.T) {
	fake := fakeDockerService{statsResult: paneldocker.ComposeStatsResult{
		Result: paneldocker.CommandResult{ExitCode: 0, Stdout: "{}"},
		Services: []paneldocker.ComposeServiceStats{{
			Name:         "demo-steam-auth-1",
			Service:      "steam-auth",
			CPUPerc:      88,
			MemPerc:      12,
			MemUsedBytes: 128,
		}},
	}}
	handler, dataDir, closeStore := newDockerTestHandler(t, fake)
	defer closeStore()
	adminCookie := setupDockerAdmin(t, handler)
	prepareComposeProject(t, dataDir)

	response, _ := doJSON(t, handler, http.MethodGet, "/api/instances/stardew/metrics", nil, adminCookie)
	if response.Code != http.StatusOK {
		t.Fatalf("metrics returned %d: %s", response.Code, response.Body.String())
	}
	var body resourceMetricsResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode metrics response: %v", err)
	}
	if body.Sample.ContainerRunning {
		t.Fatalf("metrics should not mark server running from non-server stats: %+v", body)
	}
	if body.Sample.CPUPercent != nil || body.Sample.MemoryPercent != nil {
		t.Fatalf("metrics should not expose non-server CPU/memory stats: %+v", body.Sample)
	}
	if body.Service != "server" {
		t.Fatalf("metrics response should keep server service label when server stats are absent: %+v", body)
	}
}

func TestInstancePlayersReturnsParsedInfoOutput(t *testing.T) {
	step := 0
	fake := fakeDockerService{
		psResult: paneldocker.ComposePsResult{
			Services: []paneldocker.ComposeService{{Name: "demo-server-1", Service: "server", State: "running", Status: "Up 1 minute"}},
		},
		execFunc: func(_ context.Context, _, _, stdinData string, args ...string) (paneldocker.CommandResult, error) {
			step++
			switch step {
			case 1:
				return paneldocker.CommandResult{Stdout: "0 /tmp/server-output.log", ExitCode: 0}, nil
			case 2:
				if stdinData != "info\n" {
					t.Fatalf("stdin = %q, want info newline", stdinData)
				}
				return paneldocker.CommandResult{ExitCode: 0}, nil
			default:
				return paneldocker.CommandResult{
					Stdout:   "Players: 2/4\nOnline players: Abigail, Sam\n",
					ExitCode: 0,
				}, nil
			}
		},
	}
	handler, store, dataDir, closeStore := newDockerTestHandlerWithStore(t, fake)
	defer closeStore()
	adminCookie := setupDockerAdmin(t, handler)
	prepareComposeProject(t, dataDir)
	if _, err := store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID:            storage.DefaultInstanceID,
		State:         storage.InstanceStateRunning,
		StateMessage:  "test running",
		DriverPhase:   "running",
		DriverPayload: "{}",
	}); err != nil {
		t.Fatalf("set instance running: %v", err)
	}

	response, _ := doJSON(t, handler, http.MethodGet, "/api/instances/stardew/players", nil, adminCookie)
	if response.Code != http.StatusOK {
		t.Fatalf("players returned %d: %s", response.Code, response.Body.String())
	}
	var body sj.PlayersResult
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode players response: %v", err)
	}
	if body.OnlineCount == nil || *body.OnlineCount != 2 {
		t.Fatalf("online count = %#v, want 2", body.OnlineCount)
	}
	if body.MaxPlayers == nil || *body.MaxPlayers != 4 {
		t.Fatalf("max players = %#v, want 4", body.MaxPlayers)
	}
	if len(body.Players) != 2 || body.Players[0].Name != "Abigail" || body.Players[1].Name != "Sam" {
		t.Fatalf("players = %+v, want Abigail/Sam", body.Players)
	}
}

func TestInstanceVNCConfigReturnsDefaultWhenEnvMissing(t *testing.T) {
	handler, _, closeStore := newDockerTestHandler(t, fakeDockerService{})
	defer closeStore()
	adminCookie := setupDockerAdmin(t, handler)

	response, _ := doJSON(t, handler, http.MethodGet, "/api/instances/stardew/config/vnc-port", nil, adminCookie)
	if response.Code != http.StatusOK {
		t.Fatalf("vnc config returned %d: %s", response.Code, response.Body.String())
	}
	var body instanceVNCConfigResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode vnc config response: %v", err)
	}
	if body.VNCPort != "5800" {
		t.Fatalf("default vnc port = %q, want 5800", body.VNCPort)
	}
}

func TestInstanceVNCConfigUpdatesPort(t *testing.T) {
	handler, _, closeStore := newDockerTestHandler(t, fakeDockerService{})
	defer closeStore()
	adminCookie := setupDockerAdmin(t, handler)

	updated, _ := doJSON(t, handler, http.MethodPut, "/api/instances/stardew/config/vnc-port", map[string]string{
		"port": "5801",
	}, adminCookie)
	if updated.Code != http.StatusOK {
		t.Fatalf("vnc config update returned %d: %s", updated.Code, updated.Body.String())
	}

	response, _ := doJSON(t, handler, http.MethodGet, "/api/instances/stardew/config/vnc-port", nil, adminCookie)
	if response.Code != http.StatusOK {
		t.Fatalf("vnc config returned %d: %s", response.Code, response.Body.String())
	}
	var body instanceVNCConfigResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode vnc config response: %v", err)
	}
	if body.VNCPort != "5801" {
		t.Fatalf("updated vnc port = %q, want 5801", body.VNCPort)
	}
}

func TestInstanceRenderingOpenCallsJunimoAPI(t *testing.T) {
	var capturedArgs []string
	fake := fakeDockerService{
		psResult: paneldocker.ComposePsResult{
			Services: []paneldocker.ComposeService{{Name: "demo-server-1", Service: "server", State: "running", Status: "Up 1 minute"}},
		},
		execFunc: func(_ context.Context, _, service, stdinData string, args ...string) (paneldocker.CommandResult, error) {
			if service != "server" || stdinData != "" {
				t.Fatalf("unexpected exec service=%q stdin=%q", service, stdinData)
			}
			capturedArgs = append([]string(nil), args...)
			return paneldocker.CommandResult{Stdout: `{"fps":15}`, ExitCode: 0}, nil
		},
		instanceState: storage.InstanceStateRunning,
	}
	handler, dataDir, closeStore := newDockerTestHandler(t, fake)
	defer closeStore()
	adminCookie := setupDockerAdmin(t, handler)
	instanceDir := filepath.Join(dataDir, "instances", storage.DefaultInstanceID)
	if err := os.MkdirAll(instanceDir, 0o755); err != nil {
		t.Fatalf("create instance dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(instanceDir, ".env"), []byte("API_PORT=18080\nAPI_KEY=secret\n"), 0o600); err != nil {
		t.Fatalf("write env: %v", err)
	}

	response, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/rendering", map[string]int{
		"fps": 15,
	}, adminCookie)
	if response.Code != http.StatusOK {
		t.Fatalf("rendering returned %d: %s", response.Code, response.Body.String())
	}
	wantURL := "http://localhost:18080/rendering?fps=15"
	if len(capturedArgs) == 0 || capturedArgs[len(capturedArgs)-1] != wantURL {
		t.Fatalf("rendering call args = %#v, want URL %q", capturedArgs, wantURL)
	}
}

func TestInstanceRenderingStatusCallsJunimoAPI(t *testing.T) {
	var capturedArgs []string
	fake := fakeDockerService{
		psResult: paneldocker.ComposePsResult{
			Services: []paneldocker.ComposeService{{Name: "demo-server-1", Service: "server", State: "running", Status: "Up 1 minute"}},
		},
		execFunc: func(_ context.Context, _, service, stdinData string, args ...string) (paneldocker.CommandResult, error) {
			if service != "server" || stdinData != "" {
				t.Fatalf("unexpected exec service=%q stdin=%q", service, stdinData)
			}
			capturedArgs = append([]string(nil), args...)
			return paneldocker.CommandResult{Stdout: `{"fps":15}`, ExitCode: 0}, nil
		},
		instanceState: storage.InstanceStateRunning,
	}
	handler, dataDir, closeStore := newDockerTestHandler(t, fake)
	defer closeStore()
	adminCookie := setupDockerAdmin(t, handler)
	instanceDir := filepath.Join(dataDir, "instances", storage.DefaultInstanceID)
	if err := os.MkdirAll(instanceDir, 0o755); err != nil {
		t.Fatalf("create instance dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(instanceDir, ".env"), []byte("API_PORT=18080\nAPI_KEY=secret\n"), 0o600); err != nil {
		t.Fatalf("write env: %v", err)
	}

	response, _ := doJSON(t, handler, http.MethodGet, "/api/instances/stardew/rendering", nil, adminCookie)
	if response.Code != http.StatusOK {
		t.Fatalf("rendering status returned %d: %s", response.Code, response.Body.String())
	}
	var body struct {
		FPS int `json:"fps"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode rendering status response: %v", err)
	}
	if body.FPS != 15 {
		t.Fatalf("fps = %d, want 15", body.FPS)
	}
	wantURL := "http://localhost:18080/rendering"
	if len(capturedArgs) == 0 || capturedArgs[len(capturedArgs)-1] != wantURL {
		t.Fatalf("rendering status call args = %#v, want URL %q", capturedArgs, wantURL)
	}
}

func setupDockerAdmin(t *testing.T, handler http.Handler) *http.Cookie {
	t.Helper()
	response, cookie := doJSON(t, handler, http.MethodPost, "/api/setup/admin", map[string]string{
		"username":        "admin",
		"password":        "123456",
		"confirmPassword": "123456",
	}, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("setup admin returned %d: %s", response.Code, response.Body.String())
	}
	return cookie
}

func newDockerTestHandler(t *testing.T, fake fakeDockerService) (http.Handler, string, func()) {
	t.Helper()
	handler, _, dataDir, cleanup := newDockerTestHandlerWithStore(t, fake)
	return handler, dataDir, cleanup
}

func newDockerTestHandlerWithStore(t *testing.T, fake fakeDockerService) (http.Handler, *storage.Store, string, func()) {
	t.Helper()
	dataDir := t.TempDir()
	store, err := storage.Open(context.Background(), config.Config{
		Addr:    ":0",
		DataDir: dataDir,
		DBPath:  filepath.Join(dataDir, "panel.db"),
		Secret:  "test-secret",
		Version: "test",
	})
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	if err := store.Migrate(context.Background()); err != nil {
		_ = store.Close()
		t.Fatalf("migrate storage: %v", err)
	}
	if _, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{
		ID:       storage.DefaultInstanceID,
		DriverID: storage.DefaultDriverID,
		Name:     "Stardew Valley",
		DataDir:  filepath.Join(dataDir, "instances", storage.DefaultInstanceID),
	}); err != nil {
		_ = store.Close()
		t.Fatalf("ensure default instance: %v", err)
	}
	if fake.instanceState != "" {
		if _, err := store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
			ID:            storage.DefaultInstanceID,
			State:         fake.instanceState,
			StateMessage:  "test state",
			DriverPhase:   fake.instanceState,
			DriverPayload: "{}",
		}); err != nil {
			_ = store.Close()
			t.Fatalf("set test instance state: %v", err)
		}
	}

	driverRegistry := registry.New()
	if err := driverRegistry.Register(sj.New(fake, nil, nil, store)); err != nil {
		_ = store.Close()
		t.Fatalf("register stardew driver: %v", err)
	}

	handler := NewHandler(Deps{
		Config:   config.Config{DataDir: dataDir, Secret: "test-secret", Version: "test"},
		Store:    store,
		Docker:   fake,
		Registry: driverRegistry,
	})
	return handler, store, dataDir, func() {
		if err := store.Close(); err != nil {
			t.Fatalf("close storage: %v", err)
		}
	}
}

func prepareComposeProject(t *testing.T, dataDir string) {
	t.Helper()
	workDir := filepath.Join(dataDir, "instances", "stardew")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("create compose dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workDir, "compose.yml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}
}
