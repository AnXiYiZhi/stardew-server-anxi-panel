//go:build integration

package docker

import (
	"archive/zip"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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

// This fixture deliberately contains a sensitive-looking empty environment
// value and has bash but no Node.js. It protects the real image inspect and
// steam-auth readiness contracts from regressing to full-JSON parsing or a
// Node-dependent in-container probe.
func TestRuntimeInspectAndAuthProbeWithoutNode(t *testing.T) {
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
	build := exec.CommandContext(ctx, "docker", "build", "-t", image, "-")
	build.Stdin = strings.NewReader("FROM alpine:3.20\nRUN apk add --no-cache bash python3 && mkdir -p /www/steam && printf '{\"ready\":true,\"has_ticket\":true}' > /www/steam/ready\nENV VNC_PASSWORD=\nENTRYPOINT [\"python3\",\"-m\",\"http.server\",\"3001\",\"--directory\",\"/www\"]\n")
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
	client := NewClient(Options{DockerPath: "docker"})
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
	ready, err := client.RuntimeSteamAuthReady(ctx, workDir, project)
	if err != nil || !ready.Ready || !ready.HasTicket {
		t.Fatalf("ready=%+v err=%v", ready, err)
	}
}

// Opt-in release acceptance against the reviewed runtime images. It does not
// read credentials and does not require a logged-in Steam session; the auth
// assertion is that the real .NET image's /steam/ready response is reachable
// and parseable without Node.js inside the container.
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
		if _, lastErr = client.RuntimeSteamAuthReady(ctx, workDir, project); lastErr == nil {
			service, inspectErr := client.RuntimeServiceInspect(ctx, workDir, project, "steam-auth")
			if inspectErr != nil || service.State != "running" {
				t.Fatalf("real auth service=%+v err=%v", service, inspectErr)
			}
			return
		}
		time.Sleep(time.Second)
	}
	t.Fatalf("real auth readiness response remained unavailable: %v", lastErr)
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
