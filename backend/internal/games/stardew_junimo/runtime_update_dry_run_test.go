package stardew_junimo

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	appconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

type runtimeUpdateFakeDocker struct {
	*fakeDocker
	mu                 sync.Mutex
	calls              []string
	metadata           map[string]paneldocker.RuntimeImageMetadata
	pulled             map[string]bool
	pullErrors         map[string]error
	digestMissing      map[string]bool
	composeConfig      paneldocker.RuntimeComposeConfig
	composeConfigErr   error
	composeValidateErr error
	volume             paneldocker.RuntimeVolumeMetadata
	volumeErr          error
	pullLogLine        string
}

func newRuntimeUpdateFakeDocker(dataDir string) *runtimeUpdateFakeDocker {
	manifest, _ := sjconfig.BuiltInRuntimeStackManifest()
	f := &runtimeUpdateFakeDocker{
		fakeDocker: &fakeDocker{psResult: paneldocker.ComposePsResult{Services: []paneldocker.ComposeService{{Service: "server", State: "running"}, {Service: "steam-auth", State: "running"}}}},
		metadata: map[string]paneldocker.RuntimeImageMetadata{
			"sdvd/server:1.4.0-preview.1":                    {ID: "sha256:" + strings.Repeat("b", 64), Digest: "sha256:" + strings.Repeat("b", 64)},
			"anxiyizhi/junimo-steam-service-cn:1.4.0-anxi.1": {ID: "sha256:" + strings.Repeat("c", 64), Digest: "sha256:" + strings.Repeat("c", 64)},
		},
		pulled: map[string]bool{}, pullErrors: map[string]error{}, digestMissing: map[string]bool{},
		composeConfig: paneldocker.RuntimeComposeConfig{Project: strings.ToLower(filepath.Base(dataDir)), Services: []string{"server", "steam-auth"}, SteamSessionVolume: "stardew_steam-session"},
		volume:        paneldocker.RuntimeVolumeMetadata{Name: "stardew_steam-session", Mountpoint: filepath.Join(dataDir, "steam-session")},
	}
	_ = os.MkdirAll(f.volume.Mountpoint, 0o700)
	for _, candidate := range manifest.Server.TrustedCandidates {
		f.metadata[candidate] = paneldocker.RuntimeImageMetadata{ID: "sha256:" + strings.Repeat("a", 64), Digest: manifest.Server.Digests[candidate]}
	}
	for _, candidate := range manifest.SteamAuth.TrustedCandidates {
		f.metadata[candidate] = paneldocker.RuntimeImageMetadata{ID: "sha256:" + strings.Repeat("a", 64), Digest: manifest.SteamAuth.Digests[candidate]}
	}
	return f
}

func (f *runtimeUpdateFakeDocker) record(call string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, call)
}
func (f *runtimeUpdateFakeDocker) DockerVersion(context.Context, string) (paneldocker.CommandResult, error) {
	f.record("docker version")
	return paneldocker.CommandResult{ExitCode: 0}, nil
}
func (f *runtimeUpdateFakeDocker) ComposeVersion(context.Context, string) (paneldocker.CommandResult, error) {
	f.record("docker compose version")
	return paneldocker.CommandResult{ExitCode: 0}, nil
}
func (f *runtimeUpdateFakeDocker) ComposePs(ctx context.Context, dir string) (paneldocker.ComposePsResult, error) {
	f.record("docker compose ps")
	return f.fakeDocker.ComposePs(ctx, dir)
}
func (f *runtimeUpdateFakeDocker) RuntimeImageInspect(_ context.Context, _ string, image string) (paneldocker.RuntimeImageMetadata, error) {
	f.record("docker image inspect")
	metadata, ok := f.metadata[image]
	if !ok {
		return paneldocker.RuntimeImageMetadata{}, errors.New("not found")
	}
	if strings.Contains(image, "1.5.0-") && !f.pulled[image] {
		return paneldocker.RuntimeImageMetadata{}, errors.New("not found")
	}
	if f.digestMissing[image] {
		metadata.Digest = ""
	}
	return metadata, nil
}
func (f *runtimeUpdateFakeDocker) PullImageStreaming(_ context.Context, _ string, image string, lineHandler func(string)) (paneldocker.CommandResult, error) {
	f.record("docker image pull")
	if f.pullLogLine != "" {
		lineHandler(f.pullLogLine)
	}
	if err := f.pullErrors[image]; err != nil {
		return paneldocker.CommandResult{}, err
	}
	f.pulled[image] = true
	return paneldocker.CommandResult{ExitCode: 0}, nil
}
func (f *runtimeUpdateFakeDocker) RuntimeComposeConfigInspect(context.Context, string, string) (paneldocker.RuntimeComposeConfig, error) {
	f.record("docker compose config inspect")
	return f.composeConfig, f.composeConfigErr
}
func (f *runtimeUpdateFakeDocker) RuntimeComposeConfigValidateImages(context.Context, string, string, string, string) error {
	f.record("docker compose config --quiet")
	return f.composeValidateErr
}
func (f *runtimeUpdateFakeDocker) RuntimeVolumeInspect(context.Context, string, string) (paneldocker.RuntimeVolumeMetadata, error) {
	f.record("docker volume inspect")
	return f.volume, f.volumeErr
}

func setupRuntimeUpdateDriver(t *testing.T, state string) (*Driver, *storage.Store, registry.Instance, *runtimeUpdateFakeDocker) {
	t.Helper()
	root := t.TempDir()
	dataDir := filepath.Join(root, "instance")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	env := "IMAGE_VERSION=1.4.0-preview.1\nSERVER_IMAGE=sdvd/server:1.4.0-preview.1\nSERVER_IMAGE_CANDIDATES=sdvd/server:1.4.0-preview.1\nSTEAM_SERVICE_IMAGE=anxiyizhi/junimo-steam-service-cn:1.4.0-anxi.1\nSTEAM_SERVICE_IMAGE_CANDIDATES=anxiyizhi/junimo-steam-service-cn:1.4.0-anxi.1\n"
	if err := os.WriteFile(filepath.Join(dataDir, ".env"), []byte(env), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "docker-compose.yml"), []byte("services:\n  server:\n    image: ${SERVER_IMAGE}\n  steam-auth:\n    image: ${STEAM_SERVICE_IMAGE}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := storage.Open(context.Background(), appconfig.Config{DataDir: root, DBPath: filepath.Join(root, "panel.db")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	stored, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{ID: "stardew", DriverID: DriverID, Name: "test", DataDir: dataDir})
	if err != nil {
		t.Fatal(err)
	}
	if state != storage.InstanceStateUninitialized {
		stored, err = store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{ID: stored.ID, State: state, DriverPhase: "ready", DriverPayload: "{}"})
		if err != nil {
			t.Fatal(err)
		}
	}
	instance := registry.Instance{ID: stored.ID, DriverID: stored.DriverID, Name: stored.Name, DataDir: stored.DataDir, State: stored.State, DriverPhase: stored.DriverPhase, DriverPayload: stored.DriverPayload}
	fake := newRuntimeUpdateFakeDocker(dataDir)
	driver := New(fake, slog.Default(), jobs.NewManager(store, slog.Default()), store)
	return driver, store, instance, fake
}

func waitRuntimeUpdateStatus(t *testing.T, driver *Driver, instance registry.Instance) RuntimeUpdateDryRunStatus {
	t.Helper()
	// The fake completes almost instantly; avoid continuously opening the file
	// while Windows is atomically replacing it between each check.
	time.Sleep(250 * time.Millisecond)
	deadline := time.Now().Add(5 * time.Second)
	var last RuntimeUpdateDryRunStatus
	for time.Now().Before(deadline) {
		status, err := driver.RuntimeUpdateDryRunStatus(instance)
		if err != nil {
			t.Fatal(err)
		}
		if status.Phase == RuntimeUpdatePhaseSucceeded || status.Phase == RuntimeUpdatePhaseFailed || status.Phase == RuntimeUpdatePhaseUnsupported {
			return status
		}
		last = status
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for dry-run; last=%#v", last)
	return RuntimeUpdateDryRunStatus{}
}

func TestRuntimeUpdateDryRunSucceedsWithoutDestructiveCommandsAndPersists(t *testing.T) {
	driver, store, instance, fake := setupRuntimeUpdateDriver(t, storage.InstanceStateRunning)
	started, err := driver.StartRuntimeUpdateDryRun(context.Background(), instance, 0)
	if err != nil {
		t.Fatal(err)
	}
	if started.JobID == "" || started.DryRunID == "" || !started.ServerRunning {
		t.Fatalf("unexpected start status: %#v", started)
	}
	status := waitRuntimeUpdateStatus(t, driver, instance)
	if status.Phase != RuntimeUpdatePhaseSucceeded || status.Selected.Server.Digest == "" || status.Selected.SteamAuth.Digest == "" {
		t.Fatalf("unexpected terminal status: %#v", status)
	}
	if status.Error != "" || status.ErrorCode != "" {
		t.Fatalf("success must not expose an error: %#v", status)
	}
	for _, call := range fake.calls {
		for _, forbidden := range []string{" up", " down", "restart", " rm", " stop", "volume delete", "volume rm"} {
			if strings.Contains(call, forbidden) {
				t.Fatalf("destructive call %q", call)
			}
		}
	}
	reconstructed := New(fake, slog.Default(), jobs.NewManager(store, slog.Default()), store)
	restored, err := reconstructed.RuntimeUpdateDryRunStatus(instance)
	if err != nil || restored.DryRunID != status.DryRunID || restored.Phase != RuntimeUpdatePhaseSucceeded {
		t.Fatalf("status not restored: %#v, %v", restored, err)
	}
}

func TestRuntimeUpdateDryRunCandidateFallback(t *testing.T) {
	_, _, instance, fake := setupRuntimeUpdateDriver(t, storage.InstanceStateGameInstalled)
	first := "registry.example/verified/server:1.5.0-preview.121"
	second := "registry-backup.example/verified/server:1.5.0-preview.121"
	digest := "sha256:" + strings.Repeat("d", 64)
	fake.metadata[first] = paneldocker.RuntimeImageMetadata{ID: "sha256:" + strings.Repeat("a", 64), Digest: digest}
	fake.metadata[second] = paneldocker.RuntimeImageMetadata{ID: "sha256:" + strings.Repeat("a", 64), Digest: digest}
	fake.pullErrors[first] = errors.New("registry unavailable")
	selected, code := selectRuntimeUpdateImage(context.Background(), fake, instance.DataDir, []string{first, second}, map[string]string{first: digest, second: digest})
	if code != "" || selected.Image != second {
		t.Fatalf("expected second verified candidate, got %#v code=%q", selected, code)
	}
}

func TestRuntimeUpdateDryRunFailures(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*runtimeUpdateFakeDocker, sjconfig.RuntimeStackManifest)
		code   string
		phase  string
	}{
		{"all candidates fail", func(f *runtimeUpdateFakeDocker, m sjconfig.RuntimeStackManifest) {
			for _, image := range m.Server.TrustedCandidates {
				f.pullErrors[image] = errors.New("denied")
			}
		}, "target_image_candidates_failed", RuntimeUpdatePhaseFailed},
		{"digest missing", func(f *runtimeUpdateFakeDocker, m sjconfig.RuntimeStackManifest) {
			for _, image := range m.Server.TrustedCandidates {
				f.digestMissing[image] = true
			}
		}, "target_image_digest_unavailable", RuntimeUpdatePhaseFailed},
		{"digest mismatch", func(f *runtimeUpdateFakeDocker, m sjconfig.RuntimeStackManifest) {
			for _, image := range m.Server.TrustedCandidates {
				metadata := f.metadata[image]
				metadata.Digest = "sha256:" + strings.Repeat("f", 64)
				f.metadata[image] = metadata
			}
		}, "target_image_digest_mismatch", RuntimeUpdatePhaseFailed},
		{"steam session missing", func(f *runtimeUpdateFakeDocker, _ sjconfig.RuntimeStackManifest) {
			f.volumeErr = errors.New("not found")
		}, "steam_session_volume_missing", RuntimeUpdatePhaseUnsupported},
		{"compose validation", func(f *runtimeUpdateFakeDocker, _ sjconfig.RuntimeStackManifest) {
			f.composeValidateErr = errors.New("bad config with PASSWORD=secret")
		}, "compose_target_validation_failed", RuntimeUpdatePhaseFailed},
	}
	manifest, _ := sjconfig.BuiltInRuntimeStackManifest()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			driver, _, instance, fake := setupRuntimeUpdateDriver(t, storage.InstanceStateGameInstalled)
			test.mutate(fake, manifest)
			if _, err := driver.StartRuntimeUpdateDryRun(context.Background(), instance, 0); err != nil {
				t.Fatal(err)
			}
			status := waitRuntimeUpdateStatus(t, driver, instance)
			if status.Phase != test.phase || status.ErrorCode != test.code {
				t.Fatalf("expected %s, got %#v", test.code, status)
			}
			encoded := status.Error + strings.Join(status.Warnings, " ")
			for _, log := range status.Logs {
				encoded += " " + log.Message
			}
			if strings.Contains(encoded, "PASSWORD=") || strings.Contains(encoded, "secret") {
				t.Fatalf("sensitive Docker error leaked: %s", encoded)
			}
		})
	}
}

func TestRuntimeUpdateDryRunRejectsUnsupportedInstances(t *testing.T) {
	t.Run("not installed", func(t *testing.T) {
		driver, _, instance, _ := setupRuntimeUpdateDriver(t, storage.InstanceStateUninitialized)
		_, err := driver.StartRuntimeUpdateDryRun(context.Background(), instance, 0)
		validation, ok := IsRuntimeUpdateValidationError(err)
		if !ok || validation.Code != "not_installed" {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	t.Run("custom images", func(t *testing.T) {
		driver, _, instance, _ := setupRuntimeUpdateDriver(t, storage.InstanceStateGameInstalled)
		path := filepath.Join(instance.DataDir, ".env")
		data, _ := os.ReadFile(path)
		data = []byte(strings.Replace(string(data), "sdvd/server:1.4.0-preview.1", "example.invalid/custom/server:1.4.0-preview.1", 2))
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := driver.StartRuntimeUpdateDryRun(context.Background(), instance, 0)
		validation, ok := IsRuntimeUpdateValidationError(err)
		if !ok || validation.Code != "unsupported/custom_images" {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestRuntimeUpdateDryRunRejectsConflictingJobs(t *testing.T) {
	for _, jobType := range []string{"stardew_install", "stardew_lifecycle", "stardew_junimo_update_apply", RuntimeUpdateDryRunJobType} {
		t.Run(jobType, func(t *testing.T) {
			driver, store, instance, _ := setupRuntimeUpdateDriver(t, storage.InstanceStateGameInstalled)
			job, err := store.CreateJob(context.Background(), storage.CreateJobParams{Type: jobType, TargetType: "instance", TargetID: instance.ID})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := store.StartJob(context.Background(), job.ID); err != nil {
				t.Fatal(err)
			}
			_, err = driver.StartRuntimeUpdateDryRun(context.Background(), instance, 0)
			validation, ok := IsRuntimeUpdateValidationError(err)
			if !ok || validation.Code != "runtime_update_busy" {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestRuntimeUpdateDryRunIgnoresRawPullLogs(t *testing.T) {
	driver, _, instance, fake := setupRuntimeUpdateDriver(t, storage.InstanceStateGameInstalled)
	fake.pullLogLine = "Authorization: Bearer super-secret refresh_token=abc"
	if _, err := driver.StartRuntimeUpdateDryRun(context.Background(), instance, 0); err != nil {
		t.Fatal(err)
	}
	status := waitRuntimeUpdateStatus(t, driver, instance)
	data, _ := os.ReadFile(runtimeUpdateDryRunStatusPath(instance.DataDir))
	if status.Phase != RuntimeUpdatePhaseSucceeded || strings.Contains(string(data), "super-secret") || strings.Contains(string(data), "refresh_token") {
		t.Fatalf("raw pull log leaked: %s", data)
	}
}
