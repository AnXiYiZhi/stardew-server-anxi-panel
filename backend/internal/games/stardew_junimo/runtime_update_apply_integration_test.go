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
)

// This regression fixture represents the production incident: the auth API is
// reachable and explicitly reports a logged-out Steam session while Docker
// health remains unhealthy. Runtime acceptance must not wait for Steam login.
func TestRuntimeUpdateAuthAcceptanceDoesNotWaitForDockerHealth(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	suffix := strings.ToLower(strings.ReplaceAll(time.Now().UTC().Format("150405.000000"), ".", ""))
	project := "anxiauthoffline" + suffix
	image := project + ":integration"
	workDir := t.TempDir()
	composePath := filepath.Join(workDir, "docker-compose.yml")

	build := exec.CommandContext(ctx, "docker", "build", "-t", image, "-")
	build.Stdin = strings.NewReader("FROM alpine:3.20\nRUN apk add --no-cache bash python3 && mkdir -p /www/steam && printf '{\"ready\":false,\"has_ticket\":false}' > /www/steam/ready\nHEALTHCHECK --interval=100ms --timeout=1s --retries=1 CMD false\nENTRYPOINT [\"python3\",\"-m\",\"http.server\",\"3001\",\"--directory\",\"/www\"]\n")
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
	ready, err := driver.waitRuntimeAuth(ctx, client, workDir, project, imageMetadata.ID)
	if err != nil {
		t.Fatalf("reachable logged-out auth API was rejected: %v", err)
	}
	if ready.Ready || ready.HasTicket {
		t.Fatalf("offline fixture unexpectedly reported online capability: %+v", ready)
	}
	if elapsed := time.Since(started); elapsed >= 5*time.Second {
		t.Fatalf("auth acceptance waited for Docker health: %v", elapsed)
	}
}
