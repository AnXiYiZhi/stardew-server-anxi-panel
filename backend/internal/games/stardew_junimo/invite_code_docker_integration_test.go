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
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func TestDockerInviteCodeReadIsFileOnlyAndConcurrentSafe(t *testing.T) {
	dir := t.TempDir()
	compose := "services:\n  server:\n    image: bash:5.2\n    command: [\"bash\", \"-lc\", \"touch /tmp/invite-code.txt; exec sleep 600\"]\n"
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
	if code, err := driver.GetInviteCode(ctx, registry.Instance{ID: instance.ID}); err != nil || code != "n/a" {
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
