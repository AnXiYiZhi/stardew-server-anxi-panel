//go:build integration

package docker

import (
	"archive/zip"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestComposePsStrictAcceptsEmptyProjectAfterComposeDown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	project := "anxistrictps" + strings.ToLower(strings.ReplaceAll(time.Now().UTC().Format("150405.000000"), ".", ""))
	workDir := t.TempDir()
	composePath := filepath.Join(workDir, "docker-compose.yml")
	compose := "name: " + project + "\nservices:\n  server:\n    image: alpine:3.20\n    command: [\"sh\", \"-c\", \"sleep 300\"]\n"
	if err := os.WriteFile(composePath, []byte(compose), 0o600); err != nil {
		t.Fatal(err)
	}
	client := NewClient(Options{DockerPath: "docker"})
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		_, _ = client.ComposeDown(cleanupCtx, workDir)
	})
	if result, err := client.ComposeUp(ctx, workDir); err != nil || result.ExitCode != 0 {
		t.Fatalf("compose up result=%+v err=%v", result, err)
	}
	running, err := client.ComposePsStrict(ctx, workDir)
	if err != nil || len(running.Services) != 1 || running.Services[0].Service != "server" || running.Services[0].State != "running" {
		t.Fatalf("running strict result=%+v err=%v", running, err)
	}
	if result, err := client.ComposeDown(ctx, workDir); err != nil || result.ExitCode != 0 {
		t.Fatalf("compose down result=%+v err=%v", result, err)
	}
	containerNames, err := exec.CommandContext(ctx, "docker", "ps", "-a", "--filter", "label=com.docker.compose.project="+project, "--format", "{{.Names}}").CombinedOutput()
	if err != nil {
		t.Fatalf("inspect compose project containers: %v: %s", err, containerNames)
	}
	if strings.TrimSpace(string(containerNames)) != "" {
		t.Fatalf("compose down left project containers: %s", containerNames)
	}
	empty, err := client.ComposePsStrict(ctx, workDir)
	if err != nil || len(empty.Services) != 0 {
		t.Fatalf("empty strict result=%+v err=%v", empty, err)
	}
}

func TestRuntimeServerOnlyComposeIgnoresInvalidOptionalAuthConfig(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if output, err := exec.CommandContext(ctx, "docker", "info").CombinedOutput(); err != nil {
		t.Fatalf("docker info: %v: %s", err, output)
	}
	project := "anxiserveronly" + strings.ToLower(strings.ReplaceAll(time.Now().UTC().Format("150405.000000"), ".", ""))
	workDir := t.TempDir()
	compose := "name: " + project + `
services:
  server:
    image: alpine:3.20
    command: ["sh", "-c", "sleep 300"]
    labels:
      com.anxi-panel.test-owner: "` + project + `"
  steam-auth:
    image: "${STEAM_SERVICE_IMAGE}"
    command: ["sh", "-c", "sleep 300"]
    volumes:
      - steam-session:/data/steam-session
    labels:
      com.anxi-panel.test-owner: "` + project + `"
volumes:
  steam-session:
    labels:
      com.anxi-panel.test-owner: "` + project + `"
`
	if err := os.WriteFile(filepath.Join(workDir, "docker-compose.yml"), []byte(compose), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workDir, ".env"), []byte("STEAM_SERVICE_IMAGE=invalid optional auth image\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cleanup := func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		command := exec.CommandContext(cleanupCtx, "docker", "compose", "--project-name", project, "--project-directory", workDir, "down", "--volumes", "--remove-orphans")
		command.Env = append(os.Environ(), "STEAM_SERVICE_IMAGE="+disabledSteamAuthImage)
		_ = command.Run()
	}
	t.Cleanup(cleanup)
	if output, err := exec.CommandContext(ctx, "docker", "pull", "alpine:3.20").CombinedOutput(); err != nil {
		t.Fatalf("pull alpine fixture: %v: %s", err, output)
	}
	client := NewClient(Options{DockerPath: "docker"})
	config, err := client.RuntimeComposeConfigInspectServer(ctx, workDir, project)
	if err != nil || config.Project != project || !containsStringValue(config.Services, "server") {
		t.Fatalf("server-only config=%+v err=%v", config, err)
	}
	if err := client.RuntimeComposeConfigValidateServerImage(ctx, workDir, project, "alpine:3.20"); err != nil {
		t.Fatalf("server-only config validation: %v", err)
	}
	if err := client.RuntimeComposeUpService(ctx, workDir, project, "server"); err != nil {
		t.Fatalf("server-only up: %v", err)
	}
	ps, err := client.RuntimeComposePsServer(ctx, workDir, project)
	if err != nil || len(ps.Services) != 1 || ps.Services[0].Service != "server" || ps.Services[0].State != "running" {
		t.Fatalf("server-only ps=%+v err=%v", ps, err)
	}
	metadata, err := client.RuntimeServiceInspect(ctx, workDir, project, "server")
	if err != nil || metadata.State != "running" {
		t.Fatalf("server-only inspect=%+v err=%v", metadata, err)
	}
	authContainers, err := exec.CommandContext(ctx, "docker", "ps", "-aq", "--filter", "label=com.docker.compose.project="+project, "--filter", "label=com.docker.compose.service=steam-auth").CombinedOutput()
	if err != nil || strings.TrimSpace(string(authContainers)) != "" {
		t.Fatalf("server-only path materialized Auth: %v %s", err, authContainers)
	}
	if err := exec.CommandContext(ctx, "docker", "volume", "inspect", project+"_steam-session").Run(); err == nil {
		t.Fatal("server-only path materialized steam-session volume")
	}
	if err := client.RuntimeComposeStopServices(ctx, workDir, project, "server"); err != nil {
		t.Fatalf("server-only stop: %v", err)
	}
	cleanup()
	remaining, err := exec.CommandContext(ctx, "docker", "ps", "-aq", "--filter", "label=com.anxi-panel.test-owner="+project).CombinedOutput()
	if err != nil || strings.TrimSpace(string(remaining)) != "" {
		t.Fatalf("task-owned containers remain: %v %s", err, remaining)
	}
}

func containsStringValue(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

// This test creates only uniquely prefixed disposable volumes and never uses a
// Compose project or volume supplied by a real Panel instance.
func TestRuntimeApplyIsolatedSteamSessionCloneRestore(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	project := "anxijunimotest" + strings.ToLower(strings.ReplaceAll(time.Now().UTC().Format("150405.000000"), ".", ""))
	source := project + "_steam-session"
	snapshot := project + "_anxi-junimo-update-0123456789abcdef01234567-steam-session"
	run := func(args ...string) string {
		output, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
		if err != nil {
			t.Fatalf("docker %v: %v: %s", args, err, output)
		}
		return string(output)
	}
	run("volume", "create", source)
	t.Cleanup(func() { _ = exec.Command("docker", "volume", "rm", "-f", snapshot, source).Run() })
	run("run", "--rm", "--network", "none", "--mount", "type=volume,src="+source+",dst=/data", "alpine:3.20", "sh", "-c", "printf original-session > /data/session.marker")
	client := NewClient(Options{DockerPath: "docker"})
	if err := client.RuntimeCreateSnapshotVolume(ctx, t.TempDir(), project, snapshot); err != nil {
		t.Fatal(err)
	}
	if err := client.RuntimeCloneVolume(ctx, t.TempDir(), source, snapshot, "alpine:3.20"); err != nil {
		t.Fatal(err)
	}
	run("run", "--rm", "--network", "none", "--mount", "type=volume,src="+source+",dst=/data", "alpine:3.20", "sh", "-c", "printf migrated-session > /data/session.marker")
	if err := client.RuntimeRestoreVolume(ctx, t.TempDir(), snapshot, source, "alpine:3.20"); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(run("run", "--rm", "--network", "none", "--mount", "type=volume,src="+source+",dst=/data,readonly", "alpine:3.20", "cat", "/data/session.marker"))
	if got != "original-session" {
		t.Fatalf("restored marker=%q", got)
	}
	if err := client.RuntimeRemoveSnapshotVolume(ctx, t.TempDir(), project, snapshot); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeRemoveImageExactIDAndContainerProtection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	suffix := strings.ToLower(strings.ReplaceAll(time.Now().UTC().Format("150405.000000"), ".", ""))
	image := "anxiruntimecleanup" + suffix + "/server:1.0.0"
	container := "anxi-runtime-cleanup-" + suffix
	build := exec.CommandContext(ctx, "docker", "build", "-t", image, "-")
	build.Stdin = strings.NewReader("FROM alpine:3.20\nLABEL anxipanel.runtime-cleanup-fixture=\"" + suffix + "\"\n")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build cleanup fixture: %v: %s", err, output)
	}
	t.Cleanup(func() {
		_ = exec.Command("docker", "rm", "-f", container).Run()
		_ = exec.Command("docker", "image", "rm", "-f", image).Run()
	})
	client := NewClient(Options{DockerPath: "docker"})
	metadata, err := client.RuntimeImageInspect(ctx, t.TempDir(), image)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.RuntimeRemoveImage(ctx, t.TempDir(), image, "sha256:"+strings.Repeat("f", 64)); err == nil {
		t.Fatal("image cleanup accepted a mismatched expected image ID")
	}
	if output, err := exec.CommandContext(ctx, "docker", "create", "--name", container, image, "sleep", "60").CombinedOutput(); err != nil {
		t.Fatalf("create protected container: %v: %s", err, output)
	}
	if err := client.RuntimeRemoveImage(ctx, t.TempDir(), image, metadata.ID); err == nil {
		t.Fatal("image cleanup removed an image referenced by a container")
	}
	if err := exec.CommandContext(ctx, "docker", "rm", "-f", container).Run(); err != nil {
		t.Fatal(err)
	}
	if err := client.RuntimeRemoveImage(ctx, t.TempDir(), image, metadata.ID); err != nil {
		t.Fatalf("remove exact unreferenced image: %v", err)
	}
	if err := exec.CommandContext(ctx, "docker", "image", "inspect", image).Run(); err == nil {
		t.Fatal("exact runtime image reference still exists after cleanup")
	}
}

// This fixture deliberately contains a sensitive-looking empty environment
// value and has bash but no Node.js. /steam/ready hangs, while /health can be
// switched among the supported and fail-closed cases without external Steam.
func TestRuntimeInspectAndAuthHealthProbeWithoutNode(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	project := "anxiauthprobe" + strings.ToLower(strings.ReplaceAll(time.Now().UTC().Format("150405.000000"), ".", ""))
	image := project + ":integration"
	workDir := t.TempDir()
	composePath := filepath.Join(workDir, "docker-compose.yml")
	run := func(args ...string) string {
		output, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
		if err != nil {
			t.Fatalf("docker %v: %v: %s", args, err, output)
		}
		return string(output)
	}
	authServer := `import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

valid_body = b'{"status":"ok","logged_in":false,"accounts":[]}'

class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        with open("/tmp/auth-health-requests.log", "a", encoding="utf-8") as handle:
            handle.write(self.path + "\n")
            handle.flush()
        if self.path == "/steam/ready":
            time.sleep(60)
            return
        mode = "ok"
        try:
            with open("/tmp/auth-health-mode", "r", encoding="utf-8") as handle:
                mode = handle.read().strip()
        except FileNotFoundError:
            pass
        if mode == "timeout":
            time.sleep(3)
            return
        if mode == "404":
            self.send_response(404)
            self.end_headers()
            return
        if mode == "500":
            self.send_response(500)
            self.end_headers()
            return
        body = b'not-json' if mode == "bad-json" else valid_body
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, format, *args):
        pass

ThreadingHTTPServer(("0.0.0.0", 3001), Handler).serve_forever()
`
	if err := os.WriteFile(filepath.Join(workDir, "auth-server.py"), []byte(authServer), 0o600); err != nil {
		t.Fatal(err)
	}
	dockerfile := "FROM alpine:3.20\nRUN apk add --no-cache bash python3\nCOPY auth-server.py /auth-server.py\nENV VNC_PASSWORD=\nENTRYPOINT [\"sh\",\"-c\",\"python3 /auth-server.py & echo $! > /tmp/auth-server.pid; exec tail -f /dev/null\"]\n"
	if err := os.WriteFile(filepath.Join(workDir, "Dockerfile"), []byte(dockerfile), 0o600); err != nil {
		t.Fatal(err)
	}
	build := exec.CommandContext(ctx, "docker", "build", "-t", image, workDir)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build auth fixture: %v: %s", err, output)
	}
	compose := "services:\n  steam-auth:\n    image: " + image + "\n    environment:\n      VNC_PASSWORD: \"\"\n"
	if err := os.WriteFile(composePath, []byte(compose), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = exec.Command("docker", "compose", "--project-name", project, "--file", composePath, "down", "--remove-orphans").Run()
		_ = exec.Command("docker", "image", "rm", "-f", image).Run()
	})
	client := NewClient(Options{DockerPath: "docker", Timeouts: Timeouts{Ps: time.Second}})
	imageMetadata, err := client.RuntimeImageInspect(ctx, workDir, image)
	if err != nil || !runtimeDigestPattern.MatchString(imageMetadata.ID) {
		t.Fatalf("image inspect metadata=%+v err=%v", imageMetadata, err)
	}
	run("compose", "--project-name", project, "--file", composePath, "up", "-d")
	var service RuntimeServiceMetadata
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); {
		service, err = client.RuntimeServiceInspect(ctx, workDir, project, "steam-auth")
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil || service.State != "running" || service.ImageID != imageMetadata.ID {
		t.Fatalf("service metadata=%+v err=%v compose=%s", service, err, run("compose", "--project-name", project, "--file", composePath, "ps", "-a"))
	}
	var health RuntimeAuthServiceHealth
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); {
		health, err = client.RuntimeSteamAuthHealth(ctx, workDir, project)
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil || health.LoggedIn || health.AccountCount != 0 {
		t.Fatalf("health=%+v err=%v", health, err)
	}
	for _, test := range []struct {
		mode string
		code string
	}{
		{mode: "timeout", code: "auth_health_timeout"},
		{mode: "404", code: "auth_health_http_status"},
		{mode: "500", code: "auth_health_http_status"},
		{mode: "bad-json", code: "auth_health_invalid_response"},
	} {
		t.Run(test.mode, func(t *testing.T) {
			run("compose", "--project-name", project, "--file", composePath, "exec", "-T", "steam-auth", "sh", "-c", "printf '%s' "+test.mode+" > /tmp/auth-health-mode")
			_, probeErr := client.RuntimeSteamAuthHealth(ctx, workDir, project)
			var healthErr *RuntimeAuthHealthError
			if !errors.As(probeErr, &healthErr) || healthErr.Code != test.code {
				t.Fatalf("mode=%s error=%v typed=%+v", test.mode, probeErr, healthErr)
			}
		})
	}
	run("compose", "--project-name", project, "--file", composePath, "exec", "-T", "steam-auth", "sh", "-c", "kill $(cat /tmp/auth-server.pid)")
	_, probeErr := client.RuntimeSteamAuthHealth(ctx, workDir, project)
	var healthErr *RuntimeAuthHealthError
	if !errors.As(probeErr, &healthErr) || healthErr.Code != "auth_health_unreachable" {
		t.Fatalf("unreachable error=%v typed=%+v", probeErr, healthErr)
	}
	requests := run("compose", "--project-name", project, "--file", composePath, "exec", "-T", "steam-auth", "cat", "/tmp/auth-health-requests.log")
	if !strings.Contains(requests, "/health") || strings.Contains(requests, "/steam/ready") {
		t.Fatalf("unexpected auth probe paths: %q", requests)
	}
}

// Opt-in release acceptance against the reviewed runtime images. It does not
// read credentials and does not require a logged-in Steam session; the auth
// assertion is that the real .NET image's pure /health response satisfies the
// strict service-health contract without Node.js or a Steam account.
func TestRuntimeRealImagesOptIn(t *testing.T) {
	serverImage := os.Getenv("ANXI_REAL_SERVER_IMAGE")
	authImage := os.Getenv("ANXI_REAL_AUTH_IMAGE")
	if serverImage == "" || authImage == "" {
		t.Skip("set ANXI_REAL_SERVER_IMAGE and ANXI_REAL_AUTH_IMAGE")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	project := "anxirealimages" + strings.ToLower(strings.ReplaceAll(time.Now().UTC().Format("150405.000000"), ".", ""))
	workDir := t.TempDir()
	composePath := filepath.Join(workDir, "docker-compose.yml")
	compose := "services:\n  steam-auth:\n    image: " + authImage + "\n    environment:\n      PORT: \"3001\"\n      GAME_DIR: /data/game\n      SESSION_DIR: /data/steam-session\n      STEAM_USERNAME: \"\"\n      STEAM_PASSWORD: \"\"\n      STEAM_REFRESH_TOKEN: \"\"\n"
	if err := os.WriteFile(composePath, []byte(compose), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = exec.Command("docker", "compose", "--project-name", project, "--file", composePath, "down", "--volumes", "--remove-orphans").Run()
	})
	client := NewClient(Options{DockerPath: "docker"})
	for _, image := range []string{serverImage, authImage} {
		metadata, err := client.RuntimeImageInspect(ctx, workDir, image)
		if err != nil || !runtimeDigestPattern.MatchString(metadata.ID) || !runtimeDigestPattern.MatchString(metadata.Digest) {
			t.Fatalf("image %s metadata=%+v err=%v", image, metadata, err)
		}
	}
	if output, err := exec.CommandContext(ctx, "docker", "compose", "--project-name", project, "--file", composePath, "up", "-d").CombinedOutput(); err != nil {
		t.Fatalf("start real auth image: %v: %s", err, output)
	}
	var lastErr error
	for deadline := time.Now().Add(2 * time.Minute); time.Now().Before(deadline); {
		if _, lastErr = client.RuntimeSteamAuthHealth(ctx, workDir, project); lastErr == nil {
			service, inspectErr := client.RuntimeServiceInspect(ctx, workDir, project, "steam-auth")
			if inspectErr != nil || service.State != "running" {
				t.Fatalf("real auth service=%+v err=%v", service, inspectErr)
			}
			return
		}
		time.Sleep(time.Second)
	}
	t.Fatalf("real auth health response remained unavailable: %v", lastErr)
}

// The installer fixture only emulates the official installer's CLI contract;
// production still accepts exclusively the reviewed official ZIP and SHA256.
// All volumes and the helper image are uniquely named and disposable.
func TestRuntimeSMAPIIsolatedStagingCloneAndInstaller(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	project := "anxismapitest" + strings.ToLower(strings.ReplaceAll(time.Now().UTC().Format("150405.000000"), ".", ""))
	source := project + "_game-data"
	staging := project + "_anxi-smapi-update-0123456789abcdef01234567"
	image := project + ":integration"
	run := func(args ...string) string {
		output, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
		if err != nil {
			t.Fatalf("docker %v: %v: %s", args, err, output)
		}
		return string(output)
	}
	build := exec.CommandContext(ctx, "docker", "build", "-t", image, "-")
	build.Stdin = strings.NewReader("FROM alpine:3.20\nRUN apk add --no-cache unzip\n")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build isolated helper: %v: %s", err, output)
	}
	run("volume", "create", source)
	t.Cleanup(func() {
		_ = exec.Command("docker", "volume", "rm", "-f", staging, source).Run()
		_ = exec.Command("docker", "image", "rm", "-f", image).Run()
	})
	run("run", "--rm", "--network", "none", "--mount", "type=volume,src="+source+",dst=/game", "alpine:3.20", "sh", "-c", "printf original-game > /game/game.marker")

	workDir := t.TempDir()
	packageDir := filepath.Join(workDir, ".local-container", "smapi-update", "packages")
	if err := os.MkdirAll(packageDir, 0o700); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(packageDir, "SMAPI-4.5.2-installer.zip")
	f, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	header := &zip.FileHeader{Name: "SMAPI 4.5.2 installer/internal/linux/SMAPI.Installer", Method: zip.Deflate}
	header.SetMode(0o755)
	w, err := zw.CreateHeader(header)
	if err != nil {
		t.Fatal(err)
	}
	installer := `#!/bin/sh
set -eu
printf '#!/bin/sh\n' > /game/StardewModdingAPI
chmod +x /game/StardewModdingAPI
printf '4.5.2+abcdef0' > /game/StardewModdingAPI.dll
printf '{}' > /game/StardewModdingAPI.deps.json
printf '{}' > /game/StardewModdingAPI.runtimeconfig.json
mkdir -p /game/smapi-internal
printf '{}' > /game/smapi-internal/config.json
`
	if _, err := w.Write([]byte(installer)); err != nil {
		t.Fatal(err)
	}
	if _, err := zw.Create("SMAPI 4.5.2 installer/internal/windows/SMAPI.Installer.exe"); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	client := NewClient(Options{DockerPath: "docker"})
	if err := client.RuntimeCreateSMAPIStagingVolume(ctx, workDir, project, staging); err != nil {
		t.Fatal(err)
	}
	if err := client.RuntimeCloneGameData(ctx, workDir, source, staging, image); err != nil {
		t.Fatal(err)
	}
	if err := client.RuntimeInstallSMAPIArchive(ctx, workDir, staging, archive, image); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(run("run", "--rm", "--network", "none", "--mount", "type=volume,src="+staging+",dst=/game,readonly", "alpine:3.20", "sh", "-c", "cat /game/game.marker; test -x /game/StardewModdingAPI; cat /game/StardewModdingAPI.dll"))
	if !strings.Contains(got, "original-game") || !strings.Contains(got, "4.5.2+abcdef0") {
		t.Fatalf("staging verification output=%q", got)
	}
	if err := client.RuntimeRemoveSMAPIStagingVolume(ctx, workDir, project, staging); err != nil {
		t.Fatal(err)
	}
}
