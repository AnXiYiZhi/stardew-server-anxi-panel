//go:build integration

package stardew_junimo

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

// This regression fixture represents Issue #9: /health is immediately usable
// while /steam/ready hangs like a blocked Steam connection. Runtime acceptance
// must use only /health and must not wait for Docker health or Steam login.
func TestRuntimeUpdateAuthAcceptanceUsesPureHealthAndNeverCallsSteamReady(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	suffix := strings.ToLower(strings.ReplaceAll(time.Now().UTC().Format("150405.000000"), ".", ""))
	project := "anxiauthoffline" + suffix
	image := project + ":integration"
	workDir := t.TempDir()
	composePath := filepath.Join(workDir, "docker-compose.yml")

	authServer := `import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

health_body = b'{"status":"ok","logged_in":false,"accounts":[]}'
request_log = "/tmp/auth-fixture-requests.log"

class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        with open(request_log, "a", encoding="utf-8") as handle:
            handle.write(self.path + "\n")
            handle.flush()
        if self.path == "/steam/ready":
            time.sleep(60)
            return
        if self.path != "/health":
            self.send_response(404)
            self.end_headers()
            return
        mode = "ok"
        try:
            with open("/tmp/auth-fixture-mode", "r", encoding="utf-8") as handle:
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
        body = b'not-json' if mode == "bad-json" else health_body
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
	dockerfile := "FROM alpine:3.20\nRUN apk add --no-cache bash python3\nCOPY auth-server.py /auth-server.py\nHEALTHCHECK --interval=100ms --timeout=1s --retries=1 CMD false\nENTRYPOINT [\"sh\",\"-c\",\"python3 /auth-server.py & echo $! > /tmp/auth-server.pid; exec tail -f /dev/null\"]\n"
	if err := os.WriteFile(filepath.Join(workDir, "Dockerfile"), []byte(dockerfile), 0o600); err != nil {
		t.Fatal(err)
	}
	build := exec.CommandContext(ctx, "docker", "build", "-t", image, workDir)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build offline auth fixture: %v: %s", err, output)
	}
	compose := "services:\n  steam-auth:\n    image: " + image + "\n"
	if err := os.WriteFile(composePath, []byte(compose), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = exec.Command("docker", "compose", "--project-name", project, "--file", composePath, "down", "--remove-orphans").Run()
		_ = exec.Command("docker", "image", "rm", "-f", image).Run()
	})
	if output, err := exec.CommandContext(ctx, "docker", "compose", "--project-name", project, "--file", composePath, "up", "-d").CombinedOutput(); err != nil {
		t.Fatalf("start offline auth fixture: %v: %s", err, output)
	}

	client := paneldocker.NewClient(paneldocker.Options{DockerPath: "docker"})
	imageMetadata, err := client.RuntimeImageInspect(ctx, workDir, image)
	if err != nil {
		t.Fatal(err)
	}
	var service paneldocker.RuntimeServiceMetadata
	for deadline := time.Now().Add(15 * time.Second); time.Now().Before(deadline); {
		service, err = client.RuntimeServiceInspect(ctx, workDir, project, "steam-auth")
		if err == nil && service.State == "running" && service.Health == "unhealthy" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil || service.State != "running" || service.Health != "unhealthy" {
		t.Fatalf("offline auth fixture did not reach intended state: service=%+v err=%v", service, err)
	}

	driver := &Driver{runtimeUpdateAuthTimeout: 5 * time.Second, runtimeUpdatePollInterval: 100 * time.Millisecond}
	started := time.Now()
	health, err := driver.waitRuntimeAuth(ctx, client, workDir, project, imageMetadata.ID)
	if err != nil {
		t.Fatalf("reachable logged-out auth API was rejected: %v", err)
	}
	if health.LoggedIn || health.AccountCount != 0 {
		t.Fatalf("offline fixture unexpectedly reported online capability: %+v", health)
	}
	if elapsed := time.Since(started); elapsed >= 2*time.Second {
		t.Fatalf("auth acceptance did not complete quickly through /health: %v", elapsed)
	}
	requests := runDockerIntegrationCommand(t, ctx, "docker", "compose", "--project-name", project, "--file", composePath, "exec", "-T", "steam-auth", "cat", "/tmp/auth-fixture-requests.log")
	if !strings.Contains(requests, "/health") || strings.Contains(requests, "/steam/ready") {
		t.Fatalf("unexpected auth probe paths: %q", requests)
	}

	manifest, err := sjconfig.BuiltInRuntimeStackManifest()
	if err != nil {
		t.Fatal(err)
	}
	probeClient := paneldocker.NewClient(paneldocker.Options{DockerPath: "docker", Timeouts: paneldocker.Timeouts{Ps: 2 * time.Second}})
	for _, failure := range []struct {
		mode string
		code string
	}{
		{mode: "404", code: "auth_health_http_status"},
		{mode: "500", code: "auth_health_http_status"},
		{mode: "bad-json", code: "auth_health_invalid_response"},
		{mode: "timeout", code: "auth_health_timeout"},
		{mode: "unreachable", code: "auth_health_unreachable"},
	} {
		t.Run("rollback_"+failure.mode, func(t *testing.T) {
			if failure.mode == "unreachable" {
				runDockerIntegrationCommand(t, ctx, "docker", "compose", "--project-name", project, "--file", composePath, "exec", "-T", "steam-auth", "sh", "-c", "kill $(cat /tmp/auth-server.pid)")
			} else {
				runDockerIntegrationCommand(t, ctx, "docker", "compose", "--project-name", project, "--file", composePath, "exec", "-T", "steam-auth", "sh", "-c", "printf '%s' "+failure.mode+" > /tmp/auth-fixture-mode")
			}
			driver, _, instance, fake := setupRuntimeApplyDriver(t, storage.InstanceStateRunning)
			for _, candidate := range manifest.SteamAuth.TrustedCandidates {
				metadata := fake.metadata[candidate]
				metadata.ID = imageMetadata.ID
				fake.metadata[candidate] = metadata
			}
			bridge := &runtimeApplyDockerHealthBridge{runtimeApplyFakeDocker: fake, client: probeClient, workDir: workDir, project: project}
			driver.docker = bridge
			driver.runtimeUpdateAuthTimeout = 20 * time.Millisecond
			if _, err := driver.StartRuntimeUpdateApply(context.Background(), instance, 0); err != nil {
				t.Fatal(err)
			}
			status := waitRuntimeApply(t, driver, instance)
			if status.Phase != RuntimeUpdateApplyFailedRolledBack || status.ErrorCode != failure.code || status.CauseCode != failure.code {
				t.Fatalf("mode=%s status=%#v", failure.mode, status)
			}
			if strings.TrimSpace(status.Error) == "" || !strings.Contains(strings.Join(fake.applyCalls, "\n"), "auth health original") {
				t.Fatalf("mode=%s lost last reason or skipped rollback health: status=%#v calls=%v", failure.mode, status, fake.applyCalls)
			}
		})
	}
}

func runDockerIntegrationCommand(t *testing.T, ctx context.Context, name string, args ...string) string {
	t.Helper()
	output, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v: %s", name, args, err, output)
	}
	return string(output)
}

type runtimeApplyDockerHealthBridge struct {
	*runtimeApplyFakeDocker
	client  *paneldocker.Client
	workDir string
	project string
}

func (b *runtimeApplyDockerHealthBridge) RuntimeServiceInspect(ctx context.Context, dataDir, project, service string) (paneldocker.RuntimeServiceMetadata, error) {
	if service == "steam-auth" && b.targetConfigured(dataDir) {
		return b.client.RuntimeServiceInspect(ctx, b.workDir, b.project, service)
	}
	return b.runtimeApplyFakeDocker.RuntimeServiceInspect(ctx, dataDir, project, service)
}

func (b *runtimeApplyDockerHealthBridge) RuntimeSteamAuthHealth(ctx context.Context, dataDir, project string) (paneldocker.RuntimeAuthServiceHealth, error) {
	if b.targetConfigured(dataDir) {
		b.applyCall("auth health target docker")
		return b.client.RuntimeSteamAuthHealth(ctx, b.workDir, b.project)
	}
	return b.runtimeApplyFakeDocker.RuntimeSteamAuthHealth(ctx, dataDir, project)
}
