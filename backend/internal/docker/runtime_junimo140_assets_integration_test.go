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

func TestJunimo140OwnedStagingAndModExtractionRealOptIn(t *testing.T) {
	if os.Getenv("ANXI_REAL_JUNIMO140_ARCHIVE") != "1" {
		t.Skip("set ANXI_REAL_JUNIMO140_ARCHIVE=1")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	client := NewClient(Options{DockerPath: "docker"})
	var nonce [12]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	identity := hex.EncodeToString(nonce[:])
	project, applyID := "j140-assets-"+identity, "apply_"+identity
	root := filepath.Join(t.TempDir(), project)
	backup := filepath.Join(root, ".local-container", "junimo140-group", applyID)
	if err := os.MkdirAll(backup, 0700); err != nil {
		t.Fatal(err)
	}
	image, err := client.RuntimeImageInspect(ctx, root, "sdvd/server:1.5.0-preview.140")
	if err != nil || image.Digest != "sha256:f23fdc3a3e999032be363fa43291c2d72626555913ef2b3dcf71b0aec53c49db" {
		t.Fatalf("fixed image unavailable: %v", err)
	}
	stage := project + "_anxi-smapi-update-" + identity
	if err := client.RuntimeCreateJunimo140Staging(ctx, root, project, stage, applyID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := client.RuntimeRemoveJunimo140Staging(context.Background(), root, project, stage, applyID); err != nil {
			t.Error("owned staging cleanup failed", err)
		}
	})
	if err := client.RuntimeCreateJunimo140Staging(ctx, root, project, stage, applyID); err == nil {
		t.Fatal("existing staging identity reused")
	}
	if err := client.RuntimePrepareJunimo140GamePermissions(ctx, root, stage, "sdvd/server:1.5.0-preview.140", image.ID, "1000", "1000"); err != nil {
		t.Fatal(err)
	}
	request := RuntimeInstanceArchiveRequest{Project: project, InstanceHostDir: root, RecoveryHostDir: backup, Filename: identity[:12] + ".tar", ImageRef: "sdvd/server:1.5.0-preview.140", ImageID: image.ID}
	if err := client.RuntimeExtractJunimo140Mod(ctx, root, request); err != nil {
		t.Fatal(err)
	}
	modDir := filepath.Join(backup, "target-mod-"+identity[:12])
	raw, err := os.ReadFile(filepath.Join(modDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct{ Version, UniqueID string }
	if json.Unmarshal(raw, &manifest) != nil || manifest.Version != "1.5.0-preview.140" || manifest.UniqueID != "JunimoHost.Server" {
		t.Fatal("wrong image Mod extracted")
	}
	if info, err := os.Stat(filepath.Join(modDir, "JunimoServer.dll")); err != nil || info.Size() == 0 {
		t.Fatal("image assembly missing")
	}
	if err := client.RuntimeExtractJunimo140Mod(ctx, root, request); err == nil {
		t.Fatal("existing extraction evidence overwritten")
	}
	if err := client.RuntimeRemoveJunimo140Staging(ctx, root, project, stage, applyID); err != nil {
		t.Fatal(err)
	}
	if err := client.RuntimeRemoveJunimo140Staging(ctx, root, project, stage, applyID); err != nil {
		t.Fatal("absent staging cleanup not idempotent", err)
	}
	// A matching name alone never authorizes removal of another owner's volume.
	foreignProject := project + "-foreign"
	foreign := foreignProject + "_anxi-smapi-update-" + identity
	output, err := exec.CommandContext(ctx, "docker", "volume", "create", "--label", "anxi.task=junimo140-20261015", "--label", "com.anxi-panel.assets-test="+identity, "--label", "com.anxi-panel.smapi-update-staging=true", "--label", "com.anxi-panel.compose-project=other-owner", foreign).CombinedOutput()
	if err != nil {
		t.Fatalf("isolated foreign-owner fixture failed: %v: %s", err, output)
	}
	t.Cleanup(func() {
		raw, err := exec.Command("docker", "volume", "inspect", foreign).Output()
		var records []struct {
			Name   string
			Labels map[string]string
		}
		if err != nil || json.Unmarshal(raw, &records) != nil || len(records) != 1 || records[0].Name != foreign || records[0].Labels["com.anxi-panel.assets-test"] != identity {
			t.Error("foreign-owner fixture cleanup identity unknown")
			return
		}
		if output, err := exec.Command("docker", "volume", "rm", foreign).CombinedOutput(); err != nil {
			t.Errorf("foreign-owner fixture cleanup failed: %v: %s", err, output)
		}
	})
	if err := client.RuntimeRemoveJunimo140Staging(ctx, root, foreignProject, foreign, applyID); err == nil || !strings.Contains(err.Error(), "ownership") {
		t.Fatalf("foreign staging accepted: %v", err)
	}
}
