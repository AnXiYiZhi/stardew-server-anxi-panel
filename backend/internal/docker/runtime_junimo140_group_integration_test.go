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

func TestJunimo140StoppedRecreationAndHolderScopeRealOptIn(t *testing.T) {
	if os.Getenv("ANXI_REAL_JUNIMO140_ARCHIVE") != "1" {
		t.Skip("set ANXI_REAL_JUNIMO140_ARCHIVE=1")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	dir := t.TempDir()
	client := NewClient(Options{DockerPath: "docker"})
	image, err := client.RuntimeImageInspect(ctx, dir, "sdvd/server:1.5.0-preview.140")
	if err != nil || image.Digest != "sha256:f23fdc3a3e999032be363fa43291c2d72626555913ef2b3dcf71b0aec53c49db" {
		t.Fatalf("fixed image unavailable: %v", err)
	}
	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	project := "junimo140-scope-" + hex.EncodeToString(nonce[:])
	volume, unknown := project+"-game", project+"-unknown"
	run := func(args ...string) string {
		t.Helper()
		command := exec.CommandContext(ctx, "docker", args...)
		command.Dir = dir
		out, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("isolated group fixture failed: %v: %s", err, out)
		}
		return string(out)
	}
	if strings.TrimSpace(run("container", "ls", "--all", "--quiet", "--filter", "label=com.docker.compose.project="+project)) != "" || strings.TrimSpace(run("volume", "ls", "--filter", "name=^"+volume+"$", "--format", "{{.Name}}")) != "" {
		t.Fatal("group fixture resource collision")
	}
	run("volume", "create", "--label", "anxi.task=junimo140-20261015", "--label", "com.anxi-panel.junimo140-group-test=true", volume)
	t.Cleanup(func() {
		out, err := exec.Command("docker", "volume", "inspect", volume).Output()
		var found []struct {
			Name   string
			Labels map[string]string
		}
		if err != nil || json.Unmarshal(out, &found) != nil || len(found) != 1 || found[0].Name != volume || found[0].Labels["com.anxi-panel.junimo140-group-test"] != "true" {
			t.Error("group fixture volume ownership changed; retained")
			return
		}
		if out, err := exec.Command("docker", "volume", "rm", volume).CombinedOutput(); err != nil {
			t.Errorf("group fixture volume cleanup: %v: %s", err, out)
		}
	})
	compose := "services:\n  server:\n    image: " + image.ID + "\n    entrypoint: [\"sh\", \"-c\", \"exec sleep 300\"]\n    depends_on: [steam-auth]\n    volumes:\n      - game-data:/data/game\n  steam-auth:\n    image: " + image.ID + "\n    entrypoint: [\"sh\", \"-c\", \"exec sleep 300\"]\nvolumes:\n  game-data:\n    external: true\n    name: " + volume + "\n"
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte(compose), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		command := exec.Command("docker", "compose", "--project-name", project, "down", "--remove-orphans")
		command.Dir = dir
		if out, err := command.CombinedOutput(); err != nil {
			t.Errorf("group fixture compose cleanup: %v: %s", err, out)
		}
	})
	if err := client.RuntimeComposeCreateStopped(ctx, dir, project, "server"); err != nil {
		t.Fatal(err)
	}
	ids := strings.Fields(run("container", "ls", "--all", "--quiet", "--no-trunc", "--filter", "label=com.docker.compose.project="+project))
	if len(ids) != 1 {
		t.Fatal("stopped recreation created a dependency")
	}
	var containers []struct {
		State struct {
			Status  string
			Running bool
		}
		Image string
	}
	if json.Unmarshal([]byte(run("container", "inspect", ids[0])), &containers) != nil || len(containers) != 1 || containers[0].State.Status != "created" || containers[0].State.Running || containers[0].Image != image.ID {
		t.Fatal("recovery started the game or used a different image")
	}
	if err := client.RuntimeAssertJunimo140GameGroup(ctx, dir, volume, []string{project}, true); err != nil {
		t.Fatal(err)
	}
	run("start", ids[0])
	if client.RuntimeAssertJunimo140GameGroup(ctx, dir, volume, []string{project}, true) == nil {
		t.Fatal("running shared game accepted as quiescent")
	}
	if err := client.RuntimeAssertJunimo140GameGroup(ctx, dir, volume, []string{project}, false); err != nil {
		t.Fatal(err)
	}
	run("stop", "--time", "1", ids[0])
	run("create", "--name", unknown, "--label", "anxi.task=junimo140-20261015", "--label", "com.anxi-panel.junimo140-group-test=true", "--mount", "type=volume,src="+volume+",dst=/data/game", "--entrypoint", "true", image.ID)
	t.Cleanup(func() {
		out, err := exec.Command("docker", "container", "inspect", unknown).Output()
		var found []struct {
			Config struct{ Labels map[string]string }
		}
		if err != nil || json.Unmarshal(out, &found) != nil || len(found) != 1 || found[0].Config.Labels["com.anxi-panel.junimo140-group-test"] != "true" {
			t.Error("unknown-holder fixture ownership changed; retained")
			return
		}
		if out, err := exec.Command("docker", "rm", unknown).CombinedOutput(); err != nil {
			t.Errorf("unknown-holder fixture cleanup: %v: %s", err, out)
		}
	})
	if client.RuntimeAssertJunimo140GameGroup(ctx, dir, volume, []string{project}, true) == nil {
		t.Fatal("unknown shared game holder accepted")
	}
	// Refusal is read-only: both the stopped service and foreign holder remain.
	if got := strings.Fields(run("container", "ls", "--all", "--quiet", "--filter", "volume="+volume)); len(got) != 2 {
		t.Fatal("holder refusal removed a container")
	}
}
