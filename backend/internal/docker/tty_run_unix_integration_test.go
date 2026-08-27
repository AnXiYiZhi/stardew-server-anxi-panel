//go:build integration && !windows

package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const ttyRunIntegrationOwnerLabel = "com.anxipanel.test.owner"

type ttyRunIntegrationResult struct {
	exitCode int
	err      error
}

type ttyRunContainerInspect struct {
	Name   string   `json:"Name"`
	Path   string   `json:"Path"`
	Args   []string `json:"Args"`
	Config struct {
		Cmd        []string          `json:"Cmd"`
		Entrypoint []string          `json:"Entrypoint"`
		Labels     map[string]string `json:"Labels"`
	} `json:"Config"`
	Mounts []struct {
		Type        string `json:"Type"`
		Name        string `json:"Name"`
		Destination string `json:"Destination"`
	} `json:"Mounts"`
}

type ttyRunVolumeInspect struct {
	Labels map[string]string `json:"Labels"`
}

type ttyRunImageInspect struct {
	Config struct {
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
}

// This exercises the production Linux docker-run + host-PTY path without a
// Steam account. The fixture proves stdin/stdout are real TTYs, writes only to
// a task-owned session volume, then waits for the caller to cancel it.
func TestRunSteamAuthTTYRealDockerCancellationKeepsAuthScopeIsolated(t *testing.T) {
	if testing.Short() {
		t.Skip("real Docker TTY integration is disabled by -short")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	if output, err := exec.CommandContext(ctx, "docker", "info").CombinedOutput(); err != nil {
		t.Fatalf("docker info: %v: %s", err, strings.TrimSpace(string(output)))
	}

	suffix := strings.TrimPrefix(newSteamAuthContainerName(), "anxi-steam-auth-")
	owner := "tty-run-integration-" + suffix
	image := "anxi-tty-run-fixture:" + suffix
	sessionVolume := "anxi-tty-run-session-" + suffix
	assertTTYRunOwnerResources(t, ctx, owner, 0, 0, 0)

	workDir := t.TempDir()
	script := `#!/bin/sh
set -eu
if ! test -t 0 || ! test -t 1; then
  printf 'TTY_FIXTURE_NO_TTY\n'
  exit 41
fi
test "${GAME_DIR:-}" = "/tmp/steam-auth-game"
test "${SESSION_DIR:-}" = "/data/steam-session"
test -d "${SESSION_DIR}"
printf 'session-only\n' > "${SESSION_DIR}/fixture.marker"
printf 'TTY_FIXTURE_SUCCESS\n'
printf 'TTY_FIXTURE_READY\n'
while :; do sleep 1; done
`
	if err := os.WriteFile(filepath.Join(workDir, "tty-fixture.sh"), []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	dockerfile := `FROM alpine:3.20
ARG TEST_OWNER
LABEL com.anxipanel.test.owner=$TEST_OWNER
COPY tty-fixture.sh /usr/local/bin/tty-fixture
RUN chmod 0555 /usr/local/bin/tty-fixture
ENTRYPOINT ["/usr/local/bin/tty-fixture"]
`
	if err := os.WriteFile(filepath.Join(workDir, "Dockerfile"), []byte(dockerfile), 0o600); err != nil {
		t.Fatal(err)
	}
	cleaned := false
	t.Cleanup(func() {
		if !cleaned {
			cleanupTTYRunOwnerResources(t, owner, image, sessionVolume)
		}
	})
	build := exec.CommandContext(ctx, "docker", "build", "--build-arg", "TEST_OWNER="+owner, "--tag", image, workDir)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build task-owned TTY fixture: %v: %s", err, strings.TrimSpace(string(output)))
	}
	if output, err := exec.CommandContext(ctx, "docker", "volume", "create", "--label", ttyRunIntegrationOwnerLabel+"="+owner, sessionVolume).CombinedOutput(); err != nil {
		t.Fatalf("create task-owned session volume: %v: %s", err, strings.TrimSpace(string(output)))
	}

	assertTTYRunOwnerResources(t, ctx, owner, 0, 1, 1)

	const fixturePassword = "tty-fixture-password-must-not-enter-argv"
	opts := SteamAuthRunOpts{
		ImageRef: image,
		Command:  []string{"login"},
		Env: []string{
			"GAME_DIR=/tmp/steam-auth-game",
			"SESSION_DIR=/data/steam-session",
			"STEAM_USERNAME=tty-fixture-user",
			"STEAM_PASSWORD=" + fixturePassword,
		},
		Binds: []string{sessionVolume + ":/data/steam-session"},
	}
	for _, arg := range steamAuthDockerRunArgs("anxi-steam-auth-inspect-only", opts) {
		if strings.Contains(arg, fixturePassword) {
			t.Fatalf("fixture password leaked into docker run argv")
		}
	}

	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()
	ready := make(chan struct{}, 1)
	result := make(chan ttyRunIntegrationResult, 1)
	var linesMu sync.Mutex
	var lines []string
	client := NewClient(Options{DockerPath: "docker"})
	go func() {
		exitCode, err := client.RunSteamAuthTTY(runCtx, workDir, opts, nil, func(line string) {
			linesMu.Lock()
			lines = append(lines, line)
			linesMu.Unlock()
			if strings.TrimSpace(line) == "TTY_FIXTURE_READY" {
				select {
				case ready <- struct{}{}:
				default:
				}
			}
		})
		result <- ttyRunIntegrationResult{exitCode: exitCode, err: err}
	}()

	select {
	case <-ready:
	case got := <-result:
		t.Fatalf("TTY runner exited before readiness: exit=%d err=%v lines=%q", got.exitCode, got.err, ttyRunLinesSnapshot(&linesMu, &lines))
	case <-time.After(30 * time.Second):
		t.Fatalf("TTY fixture did not become ready; lines=%q", ttyRunLinesSnapshot(&linesMu, &lines))
	}
	gotLines := ttyRunLinesSnapshot(&linesMu, &lines)
	if !containsTTYRunLine(gotLines, "TTY_FIXTURE_SUCCESS") || !containsTTYRunLine(gotLines, "TTY_FIXTURE_READY") {
		t.Fatalf("controlled TTY success/readiness logs missing: %q", gotLines)
	}

	containerNames := ttyRunContainerNames(t, ctx, owner)
	if len(containerNames) != 1 {
		t.Fatalf("task owner has %d running containers, want 1: %q", len(containerNames), containerNames)
	}
	containerName := containerNames[0]
	if !strings.HasPrefix(containerName, "anxi-steam-auth-") {
		t.Fatalf("unexpected one-shot container name %q", containerName)
	}
	inspection := inspectTTYRunContainer(t, ctx, containerName)
	if inspection.Config.Labels[ttyRunIntegrationOwnerLabel] != owner {
		t.Fatalf("container %q owner label mismatch", containerName)
	}
	if inspection.Config.Labels[steamInviteOneShotOwnerLabel] != steamInviteOneShotOwnerValue || inspection.Config.Labels[steamInviteOneShotProjectLabel] != filepath.Base(workDir) {
		t.Fatalf("container %q Steam invite ownership labels = %#v", containerName, inspection.Config.Labels)
	}
	if len(inspection.Mounts) != 1 {
		t.Fatalf("container mounts=%d, want only the session volume", len(inspection.Mounts))
	}
	mount := inspection.Mounts[0]
	if mount.Type != "volume" || mount.Name != sessionVolume || mount.Destination != "/data/steam-session" {
		t.Fatalf("unexpected auth-only mount: type=%q name=%q destination=%q", mount.Type, mount.Name, mount.Destination)
	}
	commandFields := append([]string{inspection.Path}, inspection.Args...)
	commandFields = append(commandFields, inspection.Config.Entrypoint...)
	commandFields = append(commandFields, inspection.Config.Cmd...)
	if joined := strings.Join(commandFields, "\x00"); strings.Contains(joined, fixturePassword) || strings.Contains(strings.ToLower(joined), "game-data") {
		t.Fatalf("container command leaked a secret or game-data scope")
	}

	cancelRun()
	select {
	case got := <-result:
		if got.exitCode != -1 || got.err != context.Canceled {
			t.Fatalf("intentional cancellation result: exit=%d err=%v, want -1 and exact context.Canceled", got.exitCode, got.err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("TTY runner did not finish after cancellation")
	}
	assertTTYRunOwnerResources(t, ctx, owner, 0, 1, 1)
	if output, err := exec.CommandContext(ctx, "docker", "container", "inspect", containerName).CombinedOutput(); err == nil {
		t.Fatalf("canceled one-shot container %q still exists", containerName)
	} else if strings.TrimSpace(string(output)) == "" {
		t.Fatalf("container absence check for %q returned no diagnostic", containerName)
	}

	if err := removeTTYRunOwnedVolume(ctx, owner, sessionVolume); err != nil {
		t.Fatal(err)
	}
	if err := removeTTYRunOwnedImage(ctx, owner, image); err != nil {
		t.Fatal(err)
	}
	assertTTYRunOwnerResources(t, ctx, owner, 0, 0, 0)
	cleaned = true
}

func ttyRunLinesSnapshot(mu *sync.Mutex, lines *[]string) []string {
	mu.Lock()
	defer mu.Unlock()
	return append([]string(nil), (*lines)...)
}

func containsTTYRunLine(lines []string, want string) bool {
	for _, line := range lines {
		if strings.TrimSpace(line) == want {
			return true
		}
	}
	return false
}

func ttyRunContainerNames(t *testing.T, ctx context.Context, owner string) []string {
	t.Helper()
	output, err := exec.CommandContext(ctx, "docker", "container", "ls", "-a", "--filter", "label="+ttyRunIntegrationOwnerLabel+"="+owner, "--format", "{{.Names}}").CombinedOutput()
	if err != nil {
		t.Fatalf("list task-owned containers: %v: %s", err, strings.TrimSpace(string(output)))
	}
	return strings.Fields(string(output))
}

func inspectTTYRunContainer(t *testing.T, ctx context.Context, name string) ttyRunContainerInspect {
	t.Helper()
	output, err := exec.CommandContext(ctx, "docker", "container", "inspect", name).Output()
	if err != nil {
		t.Fatalf("inspect task-owned container %q: %v", name, err)
	}
	var inspections []ttyRunContainerInspect
	if err := json.Unmarshal(output, &inspections); err != nil || len(inspections) != 1 {
		t.Fatalf("decode task-owned container %q inspection: count=%d err=%v", name, len(inspections), err)
	}
	return inspections[0]
}

func assertTTYRunOwnerResources(t *testing.T, ctx context.Context, owner string, containers, volumes, images int) {
	t.Helper()
	checks := []struct {
		kind string
		args []string
		want int
	}{
		{kind: "container", args: []string{"container", "ls", "-aq", "--filter", "label=" + ttyRunIntegrationOwnerLabel + "=" + owner}, want: containers},
		{kind: "volume", args: []string{"volume", "ls", "-q", "--filter", "label=" + ttyRunIntegrationOwnerLabel + "=" + owner}, want: volumes},
		{kind: "image", args: []string{"image", "ls", "-q", "--filter", "label=" + ttyRunIntegrationOwnerLabel + "=" + owner}, want: images},
	}
	for _, check := range checks {
		output, err := exec.CommandContext(ctx, "docker", check.args...).CombinedOutput()
		if err != nil {
			t.Fatalf("list task-owned %s resources: %v: %s", check.kind, err, strings.TrimSpace(string(output)))
		}
		if got := len(strings.Fields(string(output))); got != check.want {
			t.Fatalf("task-owned %s count=%d, want %d", check.kind, got, check.want)
		}
	}
}

func removeTTYRunOwnedVolume(ctx context.Context, owner, volume string) error {
	output, err := exec.CommandContext(ctx, "docker", "volume", "inspect", volume).Output()
	if err != nil {
		return fmt.Errorf("inspect task-owned volume %q: %w", volume, err)
	}
	var inspections []ttyRunVolumeInspect
	if err := json.Unmarshal(output, &inspections); err != nil || len(inspections) != 1 {
		return fmt.Errorf("decode task-owned volume %q inspection: count=%d err=%v", volume, len(inspections), err)
	}
	if inspections[0].Labels[ttyRunIntegrationOwnerLabel] != owner {
		return fmt.Errorf("refuse to remove volume %q with mismatched owner", volume)
	}
	if output, err := exec.CommandContext(ctx, "docker", "volume", "rm", volume).CombinedOutput(); err != nil {
		return fmt.Errorf("remove task-owned volume %q: %w: %s", volume, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func removeTTYRunOwnedImage(ctx context.Context, owner, image string) error {
	output, err := exec.CommandContext(ctx, "docker", "image", "inspect", image).Output()
	if err != nil {
		return fmt.Errorf("inspect task-owned image %q: %w", image, err)
	}
	var inspections []ttyRunImageInspect
	if err := json.Unmarshal(output, &inspections); err != nil || len(inspections) != 1 {
		return fmt.Errorf("decode task-owned image %q inspection: count=%d err=%v", image, len(inspections), err)
	}
	if inspections[0].Config.Labels[ttyRunIntegrationOwnerLabel] != owner {
		return fmt.Errorf("refuse to remove image %q with mismatched owner", image)
	}
	if output, err := exec.CommandContext(ctx, "docker", "image", "rm", image).CombinedOutput(); err != nil {
		return fmt.Errorf("remove task-owned image %q: %w: %s", image, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func cleanupTTYRunOwnerResources(t *testing.T, owner, image, volume string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "docker", "container", "ls", "-a", "--filter", "label="+ttyRunIntegrationOwnerLabel+"="+owner, "--format", "{{.Names}}").CombinedOutput()
	if err == nil {
		for _, name := range strings.Fields(string(output)) {
			if !strings.HasPrefix(name, "anxi-steam-auth-") {
				t.Errorf("refuse fallback cleanup for unexpected container name %q", name)
				continue
			}
			inspection := inspectTTYRunContainerForCleanup(ctx, name)
			if inspection != nil && inspection.Config.Labels[ttyRunIntegrationOwnerLabel] == owner {
				if removeOutput, removeErr := exec.CommandContext(ctx, "docker", "container", "rm", "-f", name).CombinedOutput(); removeErr != nil {
					t.Errorf("fallback remove task-owned container %q: %v: %s", name, removeErr, strings.TrimSpace(string(removeOutput)))
				}
			}
		}
	} else {
		t.Errorf("fallback list task-owned containers: %v: %s", err, strings.TrimSpace(string(output)))
	}
	if err := removeTTYRunOwnedVolume(ctx, owner, volume); err != nil {
		if inspectErr := exec.CommandContext(ctx, "docker", "volume", "inspect", volume).Run(); inspectErr == nil {
			t.Error(err)
		}
	}
	if err := removeTTYRunOwnedImage(ctx, owner, image); err != nil {
		if inspectErr := exec.CommandContext(ctx, "docker", "image", "inspect", image).Run(); inspectErr == nil {
			t.Error(err)
		}
	}
}

func inspectTTYRunContainerForCleanup(ctx context.Context, name string) *ttyRunContainerInspect {
	output, err := exec.CommandContext(ctx, "docker", "container", "inspect", name).Output()
	if err != nil {
		return nil
	}
	var inspections []ttyRunContainerInspect
	if json.Unmarshal(output, &inspections) != nil || len(inspections) != 1 {
		return nil
	}
	return &inspections[0]
}
