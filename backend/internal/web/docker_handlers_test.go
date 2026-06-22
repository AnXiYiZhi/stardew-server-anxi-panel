package web

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

type fakeDockerService struct {
	versionResult paneldocker.CommandResult
	versionErr    error
	composeErr    error
	psResult      paneldocker.ComposePsResult
	psErr         error
	logsResult    paneldocker.CommandResult
	logsErr       error
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

func (f fakeDockerService) ComposeLogs(ctx context.Context, dir string, opts paneldocker.LogsOptions) (paneldocker.CommandResult, error) {
	return f.logsResult, f.logsErr
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

	return NewHandler(Deps{
			Config: config.Config{DataDir: dataDir, Secret: "test-secret", Version: "test"},
			Store:  store,
			Docker: fake,
		}), dataDir, func() {
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
