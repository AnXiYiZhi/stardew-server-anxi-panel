package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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

func TestWorldDeletionLocksAreInstanceScoped(t *testing.T) {
	s := &server{}
	first := s.instanceOperationLock("target")
	first.Lock()
	defer first.Unlock()
	if s.instanceOperationLock("target").TryLock() {
		t.Fatal("duplicate deletion acquired lock")
	}
	other := s.instanceOperationLock("other")
	if !other.TryLock() {
		t.Fatal("non-target world blocked")
	}
	other.Unlock()
}

func TestWorldDeleteAdminAndConfiguredDefault(t *testing.T) {
	handler, s, root, cleanup := newDockerTestHandlerWithStore(t, fakeDockerService{})
	defer cleanup()
	admin := setupDockerAdmin(t, handler)
	_, err := s.CreateInstance(context.Background(), storage.CreateInstanceParams{ID: "protected-origin", DriverID: sj.DriverID, Name: "renamed default", DataDir: filepath.Join(root, "instances", "protected-origin"), State: storage.InstanceStateStopped})
	if err != nil {
		t.Fatal(err)
	}
	reg := registry.New()
	if err = reg.Register(sj.New(fakeDockerService{}, nil, jobs.NewManager(s, nil), s)); err != nil {
		t.Fatal(err)
	}
	handler = NewHandler(Deps{Config: config.Config{DataDir: root, Secret: "test-secret", Version: "test", DefaultInstanceID: "protected-origin"}, Store: s, Registry: reg, Docker: fakeDockerService{}, Jobs: jobs.NewManager(s, nil)})
	res, _ := doJSON(t, handler, http.MethodDelete, "/api/instances/protected-origin", nil, nil)
	if res.Code != 401 {
		t.Fatal(res.Code)
	}
	res, _ = doJSON(t, handler, http.MethodDelete, "/api/instances/protected-origin", nil, admin)
	if res.Code != 403 || !strings.Contains(res.Body.String(), "default_instance_protected") {
		t.Fatal(res.Code, res.Body.String())
	}
	res, _ = doJSON(t, handler, http.MethodGet, "/api/instances", nil, admin)
	if res.Code != 200 || !strings.Contains(res.Body.String(), `"isDefault":true`) {
		t.Fatal(res.Code, res.Body.String())
	}
	res, _ = doJSON(t, handler, http.MethodPost, "/api/users", map[string]string{"username": "delete-reader", "password": "reader-password", "role": "user"}, admin)
	if res.Code != 201 {
		t.Fatal(res.Code, res.Body.String())
	}
	res, reader := doJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]string{"username": "delete-reader", "password": "reader-password"}, nil)
	if res.Code != 200 {
		t.Fatal(res.Code)
	}
	res, _ = doJSON(t, handler, http.MethodDelete, "/api/instances/stardew", nil, reader)
	if res.Code != 403 {
		t.Fatal(res.Code, res.Body.String())
	}
}

// An opt-in real Engine test. Every mutation uses a random task prefix and
// synthetic data. No Steam login, image pulls, published ports or user worlds.
func TestWorldDeleteDockerE2E(t *testing.T) {
	if os.Getenv("TEST_WORLD_DELETE_DOCKER") != "1" {
		t.Skip("set TEST_WORLD_DELETE_DOCKER=1 for isolated real Docker deletion")
	}
	ctx := context.Background()
	docker := func(args ...string) string {
		t.Helper()
		command := exec.Command("docker", args...)
		out, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("fixture Docker command failed (%s): %v", args[0], err)
		}
		return strings.TrimSpace(string(out))
	}
	docker("info")
	image := os.Getenv("WORLD_DELETE_FIXTURE_IMAGE")
	if image == "" {
		image = "alpine:3.20"
	}
	docker("image", "inspect", image)
	handler, s, root, cleanup := newDockerTestHandlerWithStore(t, fakeDockerService{})
	defer cleanup()
	admin := setupDockerAdmin(t, handler)
	prefix := "anxi-delete-" + time.Now().UTC().Format("20060102-150405.000000000")
	prefix = strings.ReplaceAll(prefix, ".", "-")
	id := prefix + "-world"
	other := prefix + "-other"
	target := filepath.Join(root, "instances", id)
	sharedVolume := prefix + "-shared-steam-download"
	if docker("volume", "ls", "-q", "--filter", "name=^"+sharedVolume+"$") != "" {
		t.Fatal("shared fixture collision")
	}
	docker("volume", "create", "--label", "com.anxi-panel.test="+prefix, sharedVolume)
	t.Cleanup(func() {
		if docker("volume", "ls", "-q", "--filter", "label=com.anxi-panel.test="+prefix, "--filter", "name=^"+sharedVolume+"$") != "" {
			docker("volume", "rm", sharedVolume)
		}
	})
	sharedDir := filepath.Join(root, "shared", "steam-download")
	if err := os.MkdirAll(sharedDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sharedDir, "synthetic-authorization"), []byte("synthetic-only"), 0600); err != nil {
		t.Fatal(err)
	}
	engine := paneldocker.NewClient(paneldocker.Options{})
	manager := jobs.NewManager(s, nil)
	reg := registry.New()
	if err := reg.Register(sj.NewWithOptions(engine, nil, manager, s, sj.DriverOptions{ContainerDataDir: root})); err != nil {
		t.Fatal(err)
	}
	handler = NewHandler(Deps{Config: config.Config{DataDir: root, Secret: "test-secret", Version: "test"}, Store: s, Docker: engine, Jobs: manager, Registry: reg})
	for _, world := range []string{id, other} {
		dir := filepath.Join(root, "instances", world)
		for _, relative := range []string{".local-container/saves/Saves/Synthetic_1/SaveGameInfo", ".local-container/backups/saves/synthetic.zip", ".local-container/mods/Synthetic/manifest.json"} {
			file := filepath.Join(dir, filepath.FromSlash(relative))
			if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(file, []byte("synthetic-only"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		if err := sjconfig.UpdateEnvFile(filepath.Join(dir, ".env"), map[string]string{"INSTANCE_HOST_DATA_DIR": dir}); err != nil {
			t.Fatal(err)
		}
		if _, err := s.CreateInstance(ctx, storage.CreateInstanceParams{ID: world, DriverID: sj.DriverID, Name: "synthetic world", DataDir: dir, State: storage.InstanceStateStopped}); err != nil {
			t.Fatal(err)
		}
		volume := world + "_game-data"
		network := world + "_default"
		container := world + "-server"
		if docker("volume", "ls", "-q", "--filter", "name=^"+volume+"$") != "" || docker("ps", "-aq", "--filter", "name=^/"+container+"$") != "" {
			t.Fatal("fixture resource already exists")
		}
		docker("volume", "create", "--label", "com.anxi-panel.compose-project="+world, "--label", "com.anxi-panel.test="+prefix, volume)
		t.Cleanup(func() {
			// Recheck task ownership before exact-name fallback cleanup. Already
			// deleted targets yield empty filtered lists and are left alone.
			if docker("ps", "-aq", "--filter", "label=com.anxi-panel.test="+prefix, "--filter", "name=^/"+container+"$") != "" {
				docker("rm", "-f", container)
			}
			if docker("network", "ls", "-q", "--filter", "label=com.anxi-panel.test="+prefix, "--filter", "name=^"+network+"$") != "" {
				docker("network", "rm", network)
			}
			if docker("volume", "ls", "-q", "--filter", "label=com.anxi-panel.test="+prefix, "--filter", "name=^"+volume+"$") != "" {
				docker("volume", "rm", volume)
			}
		})
		docker("network", "create", "--label", "com.docker.compose.project="+world, "--label", "com.anxi-panel.test="+prefix, network)
		docker("run", "--rm", "--pull", "never", "--network", "none", "--mount", "type=volume,src="+volume+",dst=/data", image, "sh", "-c", "printf synthetic-game > /data/preserved")
		docker("create", "--name", container, "--network", network, "--label", "com.anxi-panel.test="+prefix, "--label", "com.docker.compose.project="+world, "--label", "com.docker.compose.service=server", "--label", "com.docker.compose.project.working_dir="+dir, "--mount", "type=volume,src="+volume+",dst=/data/game", "--mount", "type=bind,src="+filepath.Join(dir, ".local-container", "saves")+",dst=/config/xdg/config/StardewValley", image, "sleep", "300")
	}
	// The non-target world and its content stay intact even while running.
	docker("start", other+"-server")
	docker("start", id+"-server")
	res, _ := doJSON(t, handler, http.MethodDelete, "/api/instances/"+id, nil, admin)
	if res.Code != 409 {
		t.Fatal("running world was admitted", res.Code, res.Body.String())
	}
	docker("stop", id+"-server")
	// A foreign holder of the exact target volume must block the whole plan.
	holder := prefix + "-shared-holder"
	docker("create", "--name", holder, "--label", "com.anxi-panel.test="+prefix, "--mount", "type=volume,src="+id+"_game-data,dst=/shared", image, "true")
	t.Cleanup(func() {
		if docker("ps", "-aq", "--filter", "label=com.anxi-panel.test="+prefix, "--filter", "name=^/"+holder+"$") != "" {
			docker("rm", "-f", holder)
		}
	})
	res, _ = doJSON(t, handler, http.MethodDelete, "/api/instances/"+id, nil, admin)
	if res.Code != 409 {
		t.Fatal("shared volume was admitted", res.Code, res.Body.String())
	}
	docker("rm", holder)
	originalPlan, err := engine.PlanInstanceDeletion(ctx, filepath.Join(root, "instances"), id, target, id+"_game-data")
	if err != nil {
		var inspected []struct {
			Mounts []struct{ Type, Source, Destination string }
		}
		if json.Unmarshal([]byte(docker("inspect", id+"-server")), &inspected) == nil && len(inspected) == 1 {
			for _, mount := range inspected[0].Mounts {
				if mount.Type == "bind" {
					t.Logf("synthetic bind source=%s destination=%s expected-root=%s", mount.Source, mount.Destination, target)
				}
			}
		}
		t.Fatal("preflight", err)
	}
	res, _ = doJSON(t, handler, http.MethodDelete, "/api/instances/"+id, nil, admin)
	if res.Code != 204 {
		t.Fatal("delete failed", res.Code, res.Body.String())
	}
	res, _ = doJSON(t, handler, http.MethodDelete, "/api/instances/"+id, nil, admin)
	if res.Code != 204 {
		t.Fatal("duplicate delete", res.Code)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("saves/backups/mods not removed", err)
	}
	if _, err := s.GetInstance(ctx, id); !errors.Is(err, storage.ErrNotFound) {
		t.Fatal("database record retained", err)
	}
	if docker("volume", "ls", "-q", "--filter", "name=^"+id+"_game-data$") != "" || docker("ps", "-aq", "--filter", "name=^/"+id+"-server$") != "" || docker("network", "ls", "-q", "--filter", "name=^"+id+"_default$") != "" {
		t.Fatal("Docker target resources retained")
	}
	if docker("exec", other+"-server", "cat", "/data/game/preserved") != "synthetic-game" {
		t.Fatal("non-target data changed")
	}
	if _, err := os.Stat(filepath.Join(root, "instances", other, ".local-container/backups/saves/synthetic.zip")); err != nil {
		t.Fatal("non-target backup lost", err)
	}
	if _, err := s.GetInstance(ctx, storage.DefaultInstanceID); err != nil {
		t.Fatal("default record lost", err)
	}
	if docker("volume", "ls", "-q", "--filter", "name=^"+sharedVolume+"$") != sharedVolume {
		t.Fatal("shared Steam authorization volume lost")
	}
	if value, err := os.ReadFile(filepath.Join(sharedDir, "synthetic-authorization")); err != nil || string(value) != "synthetic-only" {
		t.Fatal("shared credentials changed", err)
	}
	// Recreating the same volume name must never satisfy an old plan.
	docker("volume", "create", "--label", "com.anxi-panel.compose-project="+id, "--label", "com.anxi-panel.test="+prefix, id+"_game-data")
	if err := engine.ApplyInstanceDeletion(ctx, filepath.Join(root, "instances"), originalPlan); err == nil {
		t.Fatal("old plan deleted a replacement volume")
	}
	if docker("volume", "ls", "-q", "--filter", "name=^"+id+"_game-data$") == "" {
		t.Fatal("replacement volume removed")
	}
	t.Log("real Web DELETE removed synthetic container, volume, network, saves, backups, Mods and record; running/shared guards and non-target preservation passed")
}
