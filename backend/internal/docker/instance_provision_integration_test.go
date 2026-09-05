package docker

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"
)

func TestInstanceProvisionDockerOwnership(t *testing.T) {
	if os.Getenv("PANEL_PROVISION_DOCKER_TEST") != "1" {
		t.Skip("set PANEL_PROVISION_DOCKER_TEST=1 for isolated Docker ownership regression")
	}
	c := NewClient(Options{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	dir := t.TempDir() // CLI workdir only; no host paths are mounted.
	run := func(args ...string) string {
		t.Helper()
		r, err := c.run(ctx, "provision fixture", dir, time.Minute, args...)
		if err != nil {
			t.Fatalf("%v: %v %s", args, err, r.Stderr)
		}
		return strings.TrimSpace(r.Stdout)
	}
	run("info")
	run("image", "inspect", "alpine:3.20")
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	token := hex.EncodeToString(b)
	project := "anxi-provision-review-" + token[:12]
	source, target := project+"-source", project+"_game-data"
	helper := "anxi-provision-" + token
	for _, name := range []string{source, target} {
		if _, err := c.run(ctx, "preflight", dir, time.Minute, "volume", "inspect", name); err == nil {
			t.Fatal("fixture volume collision")
		}
	}
	if _, err := c.run(ctx, "preflight", dir, time.Minute, "inspect", helper); err == nil {
		t.Fatal("fixture container collision")
	}
	t.Cleanup(func() {
		cleanupCtx, done := context.WithTimeout(context.Background(), time.Minute)
		defer done()
		if err := c.CleanupInstanceGameData(cleanupCtx, dir, project, token); err != nil {
			t.Error(err)
		}
		for _, name := range []string{target, source} {
			var rows []deletionVolume
			if err := c.deletionJSON(cleanupCtx, dir, &rows, "volume", "inspect", name); err != nil {
				continue
			}
			if len(rows) != 1 || rows[0].Labels["qa-owner"] != token {
				t.Error("refusing foreign fixture cleanup")
				continue
			}
			if _, err := c.run(cleanupCtx, "fixture cleanup", dir, time.Minute, "volume", "rm", name); err != nil {
				t.Error(err)
			}
		}
	})
	for _, name := range []string{source, target} {
		run("volume", "create", "--label", "qa-owner="+token, name)
	}
	run("run", "--rm", "--pull", "never", "--network", "none", "--mount", "type=volume,src="+source+",dst=/data", "alpine:3.20", "sh", "-c", "echo source-sentinel > /data/sentinel")
	run("run", "--rm", "--pull", "never", "--network", "none", "--mount", "type=volume,src="+target+",dst=/data", "alpine:3.20", "sh", "-c", "echo foreign-sentinel > /data/sentinel")
	read := func(volume string) string {
		return run("run", "--rm", "--pull", "never", "--network", "none", "--mount", "type=volume,src="+volume+",dst=/data,readonly", "alpine:3.20", "cat", "/data/sentinel")
	}
	if err := c.ProvisionInstanceGameData(ctx, dir, project, source, target, "alpine:3.20", token); err == nil {
		t.Fatal("pre-existing target accepted")
	}
	if err := c.CleanupInstanceGameData(ctx, dir, project, token); err != nil {
		t.Fatal(err)
	}
	if read(target) != "foreign-sentinel" {
		t.Fatal("pre-existing target changed")
	}
	run("volume", "rm", target) // Exact task-owned target checked above.
	if err := c.ProvisionInstanceGameData(ctx, dir, project, source, target, "alpine:3.20", token); err != nil {
		t.Fatal(err)
	}
	if read(target) != "source-sentinel" {
		t.Fatal("clone incomplete")
	}
	// Simulate a copier surviving a Panel crash; recovery must stop only it.
	run("run", "-d", "--name", helper, "--pull", "never", "--network", "none", "--label", "com.anxi-panel.provision-token="+token, "--label", "com.anxi-panel.compose-project="+project, "--mount", "type=volume,src="+target+",dst=/data", "alpine:3.20", "sleep", "120")
	for i := 0; i < 2; i++ {
		if err := c.CleanupInstanceGameData(ctx, dir, project, token); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := c.run(ctx, "removed target", dir, time.Minute, "volume", "inspect", target); err == nil {
		t.Fatal("owned target leaked")
	}
	if _, err := c.run(ctx, "removed helper", dir, time.Minute, "inspect", helper); err == nil {
		t.Fatal("copier leaked")
	}
	if read(source) != "source-sentinel" {
		t.Fatal("template was changed")
	}
}
