//go:build integration

package docker

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestComposeRecreateServicesAppliesChangedEnvironmentWithoutRestartingDependency(t *testing.T) {
	dir := t.TempDir()
	compose := `services:
  dependency:
    image: alpine:3.20
    command: ["sh", "-c", "sleep 600"]
  server:
    image: alpine:3.20
    depends_on:
      - dependency
    command: ["sh", "-c", "sleep 600"]
    environment:
      SAP_PLAYER_AUTH_MODE: "${SAP_PLAYER_AUTH_MODE:-none}"
`
	if err := os.WriteFile(filepath.Join(dir, "compose.yml"), []byte(compose), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("SAP_PLAYER_AUTH_MODE=none\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	client := NewClient(Options{})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if result, err := client.ComposeUp(ctx, dir); err != nil || result.ExitCode != 0 {
		t.Fatalf("compose up: result=%+v err=%v", result, err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()
		_, _ = client.ComposeDown(cleanupCtx, dir)
	})

	dependencyBefore := composeContainerID(t, ctx, client, dir, "dependency")
	assertComposeEnvironment(t, ctx, client, dir, "none")
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("SAP_PLAYER_AUTH_MODE=role\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if result, err := client.ComposeRecreateServices(ctx, dir, "server"); err != nil || result.ExitCode != 0 {
		t.Fatalf("compose recreate: result=%+v err=%v", result, err)
	}
	assertComposeEnvironment(t, ctx, client, dir, "role")
	if dependencyAfter := composeContainerID(t, ctx, client, dir, "dependency"); dependencyAfter != dependencyBefore {
		t.Fatalf("dependency container changed: before=%q after=%q", dependencyBefore, dependencyAfter)
	}
}

func assertComposeEnvironment(t *testing.T, ctx context.Context, client *Client, dir, want string) {
	t.Helper()
	result, err := client.ComposeExecPipe(ctx, dir, "server", "", "printenv", "SAP_PLAYER_AUTH_MODE")
	if err != nil || result.ExitCode != 0 || strings.TrimSpace(result.Stdout) != want {
		t.Fatalf("runtime mode=%q result=%+v err=%v, want %q", strings.TrimSpace(result.Stdout), result, err, want)
	}
}

func composeContainerID(t *testing.T, ctx context.Context, client *Client, dir, service string) string {
	t.Helper()
	result, err := client.run(ctx, "docker compose ps -q", dir, client.timeouts.Ps, "compose", "ps", "-q", service)
	if err != nil || result.ExitCode != 0 || strings.TrimSpace(result.Stdout) == "" {
		t.Fatalf("compose ps -q %s: result=%+v err=%v", service, result, err)
	}
	return strings.TrimSpace(result.Stdout)
}
