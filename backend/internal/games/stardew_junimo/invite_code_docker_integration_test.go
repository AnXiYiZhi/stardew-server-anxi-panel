//go:build integration

package stardew_junimo

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func TestDockerInviteCodeReadIsFileOnlyAndConcurrentSafe(t *testing.T) {
	dir := t.TempDir()
	if err := sjconfig.SetSteamInviteEnabled(dir, true); err != nil {
		t.Fatalf("enable Steam invite fixture: %v", err)
	}
	compose := "services:\n  server:\n    image: bash:5.2\n    command: [\"bash\", \"-lc\", \"exec sleep 600\"]\n"
	if err := os.WriteFile(filepath.Join(dir, "compose.yml"), []byte(compose), 0o600); err != nil {
		t.Fatal(err)
	}
	client := paneldocker.NewClient(paneldocker.Options{})
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

	instance := storage.Instance{ID: "stardew", DataDir: dir, State: storage.InstanceStateRunning}
	store := &fakeStore{instance: instance}
	driver := New(client, nil, nil, store)
	if code, err := driver.GetInviteCode(ctx, registry.Instance{ID: instance.ID}); err != nil || code != "" {
		t.Fatalf("empty invite file = %q, %v; attach-cli fallback must not run", code, err)
	}
	if result, err := client.ComposeExecPipe(ctx, dir, "server", "", "bash", "-lc", "printf %s REALDOCKERCODE >/tmp/invite-code.txt"); err != nil || result.ExitCode != 0 {
		t.Fatalf("write invite fixture: result=%+v err=%v", result, err)
	}

	// A fresh driver bypasses the intentional five-second empty-result cache.
	driver = New(client, nil, nil, store)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if code, err := driver.GetInviteCode(ctx, registry.Instance{ID: instance.ID}); err != nil || code != "REALDOCKERCODE" {
				t.Errorf("concurrent invite read = %q, %v", code, err)
			}
		}()
	}
	wg.Wait()
}

func TestDockerSteamInviteServiceScope(t *testing.T) {
	dir := t.TempDir()
	compose := `services:
  steam-auth:
    image: alpine:3.20
    command: ["sh", "-c", "exec sleep 600"]
    labels:
      com.openai.codex.owner: steam-invite-optin-integration
  server:
    image: alpine:3.20
    command: ["sh", "-c", "exec sleep 600"]
    labels:
      com.openai.codex.owner: steam-invite-optin-integration
`
	if err := os.WriteFile(filepath.Join(dir, "compose.yml"), []byte(compose), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := sjconfig.SetSteamInviteEnabled(dir, false); err != nil {
		t.Fatalf("disable Steam invite fixture: %v", err)
	}

	client := paneldocker.NewClient(paneldocker.Options{})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()
		_, _ = client.ComposeDown(cleanupCtx, dir)
	})

	result, err := client.ComposeRecreateServices(ctx, dir, runtimeServicesForSteamInvite(dir)...)
	if err != nil || result.ExitCode != 0 {
		t.Fatalf("start disabled service scope: result=%+v err=%v", result, err)
	}
	assertDockerServiceScope(t, ctx, client, dir, map[string]bool{"server": true, "steam-auth": false})

	if result, err = client.ComposeDown(ctx, dir); err != nil || result.ExitCode != 0 {
		t.Fatalf("reset service scope fixture: result=%+v err=%v", result, err)
	}
	if err := sjconfig.SetSteamInviteEnabled(dir, true); err != nil {
		t.Fatalf("enable Steam invite fixture: %v", err)
	}
	result, err = client.ComposeRecreateServices(ctx, dir, runtimeServicesForSteamInvite(dir)...)
	if err != nil || result.ExitCode != 0 {
		t.Fatalf("start enabled service scope: result=%+v err=%v", result, err)
	}
	assertDockerServiceScope(t, ctx, client, dir, map[string]bool{"server": true, "steam-auth": true})
}

func assertDockerServiceScope(t *testing.T, ctx context.Context, client *paneldocker.Client, dir string, expected map[string]bool) {
	t.Helper()
	ps, err := client.ComposePs(ctx, dir)
	if err != nil {
		t.Fatalf("compose ps: %v", err)
	}
	running := map[string]bool{}
	for _, service := range ps.Services {
		running[service.Service] = service.State == "running"
	}
	for service, want := range expected {
		if running[service] != want {
			t.Fatalf("service %s running=%v, want %v; services=%+v", service, running[service], want, ps.Services)
		}
	}
}
