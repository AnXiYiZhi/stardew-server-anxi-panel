//go:build integration

package docker

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestJunimo140DaemonHelpersQuiesceBeforeRecoveryRealOptIn(t *testing.T) {
	if os.Getenv("ANXI_REAL_JUNIMO140_ARCHIVE") != "1" {
		t.Skip("set ANXI_REAL_JUNIMO140_ARCHIVE=1")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	dir := t.TempDir()
	client := NewClient(Options{DockerPath: "docker"})
	const ref = "sdvd/server:1.5.0-preview.140"
	image, err := client.RuntimeImageInspect(ctx, dir, ref)
	if err != nil || image.Digest != "sha256:f23fdc3a3e999032be363fa43291c2d72626555913ef2b3dcf71b0aec53c49db" {
		t.Fatalf("fixed image unavailable: %v", err)
	}
	var nonce [12]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	identity := hex.EncodeToString(nonce[:])
	applyID := "apply_" + identity
	scope := RuntimeJunimo140HelperScope{ApplyID: applyID, Projects: []string{"world-a"}, ImageRef: ref, ImageID: image.ID}
	run := func(args ...string) string {
		t.Helper()
		output, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
		if err != nil {
			t.Fatalf("isolated helper fixture failed: %v: %s", err, output)
		}
		return strings.TrimSpace(string(output))
	}
	if prior := run("ps", "-a", "-q", "--filter", "label=com.anxi-panel.runtime-update-apply-id="+applyID); prior != "" {
		t.Fatal("helper identity collision")
	}
	t.Cleanup(func() {
		out, err := exec.Command("docker", "ps", "-a", "-q", "--no-trunc", "--filter", "label=com.anxi-panel.helper-test="+identity).Output()
		if err != nil {
			t.Error("helper cleanup list failed")
			return
		}
		for _, id := range strings.Fields(string(out)) {
			raw, err := exec.Command("docker", "inspect", id).Output()
			var records []struct {
				Config     struct{ Labels map[string]string }
				HostConfig struct{ AutoRemove bool }
			}
			if err != nil || json.Unmarshal(raw, &records) != nil || len(records) != 1 || records[0].Config.Labels["com.anxi-panel.helper-test"] != identity || !records[0].HostConfig.AutoRemove {
				t.Error("helper cleanup ownership unknown")
				continue
			}
			if out, err := exec.Command("docker", "stop", "--time", "2", id).CombinedOutput(); err != nil {
				t.Errorf("helper cleanup failed: %v: %s", err, out)
			}
		}
	})
	start := func(project, operation string) string {
		return run("run", "-d", "--rm", "--pull", "never", "--network", "none", "--read-only", "--name", "anxi-j140-"+identity+"-"+operation,
			"--label", "anxi.task=junimo140-20261015", "--label", "com.anxi-panel.helper-test="+identity,
			"--label", "com.anxi-panel.junimo140-helper=true", "--label", "com.anxi-panel.compose-project="+project,
			"--label", "com.anxi-panel.runtime-update-apply-id="+applyID, "--label", "com.anxi-panel.junimo140-operation="+operation,
			"--entrypoint", "sleep", image.ID, "300")
	}
	owned := start("world-a", "clone")
	foreign := start("foreign-world", "install")
	if err := client.RuntimeStopJunimo140Helpers(ctx, dir, scope); err == nil {
		t.Fatal("foreign project helper accepted")
	}
	if alive := run("ps", "-q", "--no-trunc", "--filter", "label=com.anxi-panel.helper-test="+identity); len(strings.Fields(alive)) != 2 {
		t.Fatal("ownership rejection stopped a verified helper before validating the full set")
	}
	run("stop", "--time", "2", foreign)
	if err := client.RuntimeStopJunimo140Helpers(ctx, dir, scope); err != nil {
		t.Fatal(err)
	}
	if remaining := run("ps", "-a", "-q", "--no-trunc", "--filter", "id="+owned); remaining != "" {
		t.Fatal("daemon helper remained after recovery quiescence")
	}
	if err := client.RuntimeStopJunimo140Helpers(ctx, dir, scope); err != nil {
		t.Fatalf("idempotent quiescence failed: %v", err)
	}
}
