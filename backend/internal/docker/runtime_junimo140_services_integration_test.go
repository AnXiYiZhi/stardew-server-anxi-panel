//go:build integration

package docker

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestJunimo140ServicesStopWithoutComposeOrEnvRealOptIn(t *testing.T) {
	if os.Getenv("ANXI_REAL_JUNIMO140_ARCHIVE") != "1" {
		t.Skip("set ANXI_REAL_JUNIMO140_ARCHIVE=1")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	client := NewClient(Options{DockerPath: "docker"})
	dir := t.TempDir()
	image, err := client.RuntimeImageInspect(ctx, dir, "sdvd/server:1.5.0-preview.140")
	if err != nil || image.Digest != "sha256:f23fdc3a3e999032be363fa43291c2d72626555913ef2b3dcf71b0aec53c49db" {
		t.Fatalf("fixed image unavailable: %v", err)
	}
	var nonce [12]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	identity := "j140-service-" + hex.EncodeToString(nonce[:])
	run := func(args ...string) string {
		t.Helper()
		out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
		if err != nil {
			t.Fatalf("isolated service fixture failed: %v: %s", err, out)
		}
		return strings.TrimSpace(string(out))
	}
	volume := identity + "-game"
	if got := run("volume", "ls", "-q", "--filter", "name="+volume); got != "" {
		t.Fatal("fixture volume collision")
	}
	run("volume", "create", "--label", "anxi.task=junimo140-20261015", "--label", "com.anxi-panel.service-test="+identity, volume)
	var ids []string
	t.Cleanup(func() {
		for _, id := range ids {
			out, err := exec.Command("docker", "inspect", id).Output()
			var records []struct {
				Config struct{ Labels map[string]string }
			}
			if err != nil || json.Unmarshal(out, &records) != nil || len(records) != 1 || records[0].Config.Labels["com.anxi-panel.service-test"] != identity {
				t.Error("service cleanup ownership unknown")
				continue
			}
			if out, err := exec.Command("docker", "rm", "-f", "-v", id).CombinedOutput(); err != nil {
				t.Errorf("service cleanup failed: %v: %s", err, out)
			}
		}
		out, err := exec.Command("docker", "volume", "inspect", volume).Output()
		var records []struct{ Labels map[string]string }
		if err != nil || json.Unmarshal(out, &records) != nil || len(records) != 1 || records[0].Labels["com.anxi-panel.service-test"] != identity {
			t.Error("service volume cleanup ownership unknown")
			return
		}
		if out, err := exec.Command("docker", "volume", "rm", volume).CombinedOutput(); err != nil {
			t.Errorf("service volume cleanup failed: %v: %s", err, out)
		}
	})
	worlds := []RuntimeJunimo140WorldScope{}
	for _, suffix := range []string{"a", "b"} {
		root := filepath.Join(dir, suffix)
		for _, leaf := range []string{"saves", "mods", "settings", "control"} {
			if err := os.MkdirAll(filepath.Join(root, ".local-container", leaf), 0700); err != nil {
				t.Fatal(err)
			}
		}
		project := identity + "-" + suffix
		worlds = append(worlds, RuntimeJunimo140WorldScope{Project: project, HostDataDir: root})
		args := []string{"run", "-d", "--pull", "never", "--network", "none", "--name", project, "--label", "anxi.task=junimo140-20261015", "--label", "com.anxi-panel.service-test=" + identity, "--label", "com.docker.compose.project=" + project, "--label", "com.docker.compose.service=server", "--mount", "type=volume,src=" + volume + ",dst=/data/game"}
		for target, leaf := range map[string]string{"/config/xdg/config/StardewValley": "saves", "/data/Mods": "mods", "/data/settings": "settings", "/data/control": "control"} {
			args = append(args, "--mount", "type=bind,src="+filepath.Join(root, ".local-container", leaf)+",dst="+target)
		}
		args = append(args, "--entrypoint", "sleep", image.ID, "300")
		ids = append(ids, run(args...))
	}
	snapshots, err := client.RuntimeSnapshotJunimo140Services(ctx, dir, worlds, []string{volume})
	if err != nil || len(snapshots) != 2 || !snapshots[0].Running || !snapshots[1].Running {
		t.Fatalf("raw managed snapshot failed: %v", err)
	}
	allowed := map[string][]string{worlds[0].Project + "/server": {image.ID}, worlds[1].Project + "/server": {"sha256:" + strings.Repeat("f", 64)}}
	if err := client.RuntimeStopJunimo140Services(ctx, dir, snapshots, allowed); err == nil {
		t.Fatal("wrong image accepted")
	}
	snapshots, err = client.RuntimeSnapshotJunimo140Services(ctx, dir, worlds, []string{volume})
	if err != nil || len(snapshots) != 2 || !snapshots[0].Running || !snapshots[1].Running {
		t.Fatal("unknown identity caused partial stop")
	}
	allowed[worlds[1].Project+"/server"] = []string{image.ID}
	if err := client.RuntimeStopJunimo140Services(ctx, dir, snapshots, allowed); err != nil {
		t.Fatal(err)
	}
	snapshots, err = client.RuntimeSnapshotJunimo140Services(ctx, dir, worlds, []string{volume})
	if err != nil || len(snapshots) != 2 || snapshots[0].Running || snapshots[1].Running {
		t.Fatalf("services not quiescent: %v", err)
	}
	if err := client.RuntimeStopJunimo140Services(ctx, dir, snapshots, allowed); err != nil {
		t.Fatal("repeated stop failed", err)
	}
}
