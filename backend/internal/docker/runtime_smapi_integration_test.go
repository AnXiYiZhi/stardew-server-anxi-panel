//go:build integration

package docker

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// The source and staging volumes are uniquely prefixed and never reference an
// installed Panel instance. This verifies copy isolation and scoped cleanup.
func TestRuntimeSMAPIIsolatedGameDataClone(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	project := "anxismapitest" + strings.ToLower(strings.ReplaceAll(time.Now().UTC().Format("150405.000000"), ".", ""))
	source := project + "_game-data"
	staging := project + "_anxi-smapi-update-0123456789abcdef01234567"
	run := func(args ...string) string {
		output, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
		if err != nil {
			t.Fatalf("docker %v: %v: %s", args, err, output)
		}
		return string(output)
	}
	run("volume", "create", source)
	t.Cleanup(func() { _ = exec.Command("docker", "volume", "rm", "-f", staging, source).Run() })
	run("run", "--rm", "--network", "none", "--mount", "type=volume,src="+source+",dst=/data", "alpine:3.20", "sh", "-c", "printf old-game-data > /data/version.marker")
	client := NewClient(Options{DockerPath: "docker"})
	if err := client.RuntimeCreateSMAPIStagingVolume(ctx, t.TempDir(), project, staging); err != nil {
		t.Fatal(err)
	}
	if err := client.RuntimeCloneGameData(ctx, t.TempDir(), source, staging, "alpine:3.20"); err != nil {
		t.Fatal(err)
	}
	run("run", "--rm", "--network", "none", "--mount", "type=volume,src="+staging+",dst=/data", "alpine:3.20", "sh", "-c", "printf staged-smapi > /data/version.marker")
	original := strings.TrimSpace(run("run", "--rm", "--network", "none", "--mount", "type=volume,src="+source+",dst=/data,readonly", "alpine:3.20", "cat", "/data/version.marker"))
	if original != "old-game-data" {
		t.Fatalf("source was changed through staging: %q", original)
	}
	if err := client.RuntimeRemoveSMAPIStagingVolume(ctx, t.TempDir(), project, staging); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(run("volume", "inspect", "--format", "{{.Name}}", source)); got != source {
		t.Fatalf("source volume was not preserved: %q", got)
	}
}
