//go:build integration

package docker

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
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

func TestJunimo140InstallLaunchersRealOptIn(t *testing.T) {
	if os.Getenv("ANXI_REAL_JUNIMO140_LAUNCHERS") != "1" {
		t.Skip("set ANXI_REAL_JUNIMO140_LAUNCHERS=1 to use the fixed .140 Docker image")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	client := NewClient(Options{DockerPath: "docker"})
	dir := t.TempDir()
	const ref = "sdvd/server:1.5.0-preview.140"
	image, err := client.RuntimeImageInspect(ctx, dir, ref)
	if err != nil || image.Digest != "sha256:f23fdc3a3e999032be363fa43291c2d72626555913ef2b3dcf71b0aec53c49db" {
		t.Fatalf("fixed image unavailable: %v", err)
	}
	var nonce [12]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	const project = "junimo140launchertest"
	id := hex.EncodeToString(nonce[:])
	volume := project + "_anxi-smapi-update-" + id
	fixtureName := project + "-" + id
	if prior, err := exec.CommandContext(ctx, "docker", "ps", "-aq", "--filter", "name=^/"+fixtureName+"$").Output(); err != nil || len(prior) != 0 {
		t.Fatalf("launcher fixture container collision check failed: %v", err)
	}
	if exists, err := client.RuntimeVolumeExists(ctx, dir, volume); err != nil || exists {
		t.Fatalf("launcher fixture volume collision check failed: %v", err)
	}
	if err := client.RuntimeCreateSMAPIStagingVolume(ctx, dir, project, volume); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 30*time.Second)
		defer stop()
		if err := client.RuntimeRemoveSMAPIStagingVolume(cleanup, dir, project, volume); err != nil {
			t.Errorf("launcher fixture cleanup failed: %v", err)
		}
	})

	vanilla := sha256.Sum256([]byte("vanilla"))
	installed := sha256.Sum256([]byte("installed"))
	const reset = `rm -f StardewValley
if test -d StardewValley-original && test ! -L StardewValley-original; then
    rmdir StardewValley-original
else
    rm -f StardewValley-original
fi
printf vanilla > StardewValley
`
	for _, tc := range []struct {
		name, fixture, wantError string
		wantVanilla              bool
	}{
		{name: "clean download", wantVanilla: true},
		{name: "restored launcher with backup", fixture: "cp StardewValley StardewValley-original", wantVanilla: true},
		{name: "complete install", fixture: "cp StardewValley StardewValley-original; printf installed > StardewValley"},
		{name: "installed launcher without backup", fixture: "printf installed > StardewValley", wantError: "StardewValley-original"},
		{name: "unknown launcher", fixture: "printf broken > StardewValley", wantError: "StardewValley does not match"},
		{name: "linked launcher", fixture: "mv StardewValley StardewValley-original; ln -s StardewValley-original StardewValley", wantError: "StardewValley is missing or is not a regular file"},
		{name: "unknown backup", fixture: "printf broken > StardewValley-original", wantError: "StardewValley-original"},
		{name: "linked backup", fixture: "ln -s StardewValley StardewValley-original", wantError: "StardewValley-original"},
		{name: "dangling backup", fixture: "ln -s missing StardewValley-original", wantError: "StardewValley-original"},
		{name: "directory backup", fixture: "mkdir StardewValley-original", wantError: "StardewValley-original"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			output, err := exec.CommandContext(ctx, "docker", "run", "--rm", "--name", fixtureName, "--label", "anxi.task="+fixtureName,
				"--pull", "never", "--network", "none", "--read-only", "--user", "0:0",
				"--mount", "type=volume,src="+volume+",dst=/game,volume-nocopy", "--workdir", "/game",
				"--entrypoint", "timeout", image.ID, "--kill-after=2s", "15s", "sh", "-eu", "-c", reset+tc.fixture).CombinedOutput()
			if err != nil {
				t.Fatalf("write isolated launcher fixture: %v: %s", err, output)
			}
			got, err := client.RuntimeJunimo140InstallUsesVanillaLauncher(ctx, dir, volume, ref, image.ID, hex.EncodeToString(vanilla[:]), hex.EncodeToString(installed[:]))
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("launcher check did not reject fixture: %v", err)
				}
			} else if err != nil || got != tc.wantVanilla {
				t.Fatalf("unexpected launcher check result: vanilla=%t, err=%v", got, err)
			}
		})
	}
}
