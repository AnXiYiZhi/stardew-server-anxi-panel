//go:build integration

package docker

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestRuntimeVersionsReadWithoutCapacityScan(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	volume := fmt.Sprintf("anxi-perf-versions-%d", time.Now().UnixNano())
	run := func(args ...string) string {
		t.Helper()
		out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
		if err != nil {
			t.Fatalf("Docker fixture: %v: %s", err, out)
		}
		return string(out)
	}
	run("volume", "create", "--label", "anxi.test=runtime-versions", volume)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cleanupCancel()
		if out, err := exec.CommandContext(cleanupCtx, "docker", "volume", "rm", volume).CombinedOutput(); err != nil {
			t.Errorf("cleanup: %v %s", err, out)
		}
	})
	run("run", "--rm", "--pull", "never", "--network", "none", "--mount", "type=volume,src="+volume+",dst=/game", "alpine:3.20", "sh", "-c", `mkdir -p /game/steamapps /game/.steam-sdk/steamapps; printf game-manifest > /game/steamapps/appmanifest_413150.acf; printf sdk-manifest > /game/.steam-sdk/steamapps/appmanifest_1007.acf`)
	client := NewClient(Options{})
	versions, err := client.RuntimeReadContentVersions(ctx, t.TempDir(), volume, "alpine:3.20")
	if err != nil || string(versions.GameManifest) != "game-manifest" || string(versions.SDKManifest) != "sdk-manifest" {
		t.Fatal(versions, err)
	}
	if versions.GameDataBytes != 0 || versions.FreeBytes != 0 {
		t.Fatal("display read collected capacity")
	}
	preflight, err := client.RuntimeReadContentManifests(ctx, t.TempDir(), volume, "alpine:3.20")
	if err != nil || preflight.GameDataBytes <= 0 || preflight.FreeBytes <= 0 {
		t.Fatal("preflight lost live capacity", preflight, err)
	}
	if strings.TrimSpace(run("run", "--rm", "--pull", "never", "--network", "none", "--mount", "type=volume,src="+volume+",dst=/game,readonly", "alpine:3.20", "cat", "/game/steamapps/appmanifest_413150.acf")) != "game-manifest" {
		t.Fatal("read modified data")
	}
}
