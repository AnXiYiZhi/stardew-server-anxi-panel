//go:build integration

package stardew_junimo

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
)

func TestJunimo140ImageSMAPIStagingRealOptIn(t *testing.T) {
	source := os.Getenv("ANXI_REAL_IMAGE_SMAPI_SOURCE_VOLUME")
	if source == "" {
		t.Skip("set ANXI_REAL_IMAGE_SMAPI_SOURCE_VOLUME to an authorized game-only fixture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	dir := t.TempDir()
	project := "junimo140image"
	id, err := randomHex(12)
	if err != nil {
		t.Fatal(err)
	}
	staging := project + "_anxi-smapi-update-" + id
	prior, err := exec.CommandContext(ctx, "docker", "volume", "ls", "--filter", "name=^"+staging+"$", "--format", "{{.Name}}").Output()
	if err != nil || strings.TrimSpace(string(prior)) != "" {
		t.Fatal("staging collision check failed")
	}
	client := paneldocker.NewClient(paneldocker.Options{DockerPath: "docker"})
	const imageRef = "sdvd/server:1.5.0-preview.140"
	image, err := client.RuntimeImageInspect(ctx, dir, imageRef)
	if err != nil || image.Digest != junimo140Digest {
		t.Fatalf("fixed image identity unavailable: %v; digest=%s", err, image.Digest)
	}
	if err := client.RuntimeCreateSMAPIStagingVolume(ctx, dir, project, staging); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := client.RuntimeRemoveSMAPIStagingVolume(context.Background(), dir, project, staging); err != nil {
			t.Errorf("owned staging cleanup failed: %v", err)
		}
	})
	selected := RuntimeUpdateSelectedImage{Image: imageRef, ImageID: image.ID, Digest: image.Digest}
	metadata, err := prepareJunimo140SharedGameStaging(ctx, client, dir, source, staging, selected)
	if err != nil || !junimo140SMAPIIdentityVerified(metadata) {
		t.Fatalf("complete shared game staging failed: %v", err)
	}
	fixture := func(script string) string {
		t.Helper()
		output, err := exec.CommandContext(ctx, "docker", "run", "--rm", "--pull", "never", "--network", "none", "--user", "0:0", "--read-only", "--mount", "type=volume,src="+staging+",dst=/game", "--entrypoint", "sh", image.ID, "-eu", "-c", script).CombinedOutput()
		if err != nil {
			t.Fatalf("isolated game corruption fixture failed: %v: %s", err, output)
		}
		return string(output)
	}
	t.Run("cold patched SMAPI installation", func(t *testing.T) {
		// Damage only the newly owned clone. The authorized source stays readonly.
		fixture(`rm -f /game/smapi-internal/.sdvd-smapi-build /game/StardewModdingAPI.dll`)
		before, err := client.RuntimeReadPinnedSMAPIMetadata(ctx, dir, staging, imageRef, image.ID)
		if err != nil || junimo140SMAPIIdentityVerified(before) {
			t.Fatalf("cold SMAPI fixture was not established: %v", err)
		}
		after, err := prepareJunimo140SMAPIStaging(ctx, client, dir, staging, selected)
		if err != nil || !junimo140SMAPIIdentityVerified(after) {
			t.Fatalf("production cold patched SMAPI installation/readback failed: %v", err)
		}
	})
	const markerHashes = `sha256sum /game/.download-manifest-413150 /game/.steam-sdk/.download-manifest-1007`
	beforeMarkers := fixture(markerHashes)
	for _, scenario := range []struct{ name, damage, repair string }{
		{"core corruption", `mv "/game/Stardew Valley.dll" /game/core.original; printf broken > "/game/Stardew Valley.dll"`, `mv /game/core.original "/game/Stardew Valley.dll"`},
		{"missing content", `mv "/game/Content/Animals/BabyBlue Chicken.xnb" /game/asset.original`, `mv /game/asset.original "/game/Content/Animals/BabyBlue Chicken.xnb"`},
		{"symlinked content root", `mv /game/Content /game/Content.original; ln -s Content.original /game/Content`, `rm /game/Content; mv /game/Content.original /game/Content`},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			fixture(scenario.damage)
			defer fixture(scenario.repair)
			if err := prepareJunimo140GameMarkers(ctx, client, dir, staging, selected, time.Now()); err == nil {
				t.Fatal("damaged or symlinked payload accepted")
			}
			if after := fixture(markerHashes); after != beforeMarkers {
				t.Fatal("failed integrity proof changed completion markers")
			}
		})
	}
	// A second preparation must recognize the exact installed bytes and stamp.
	if _, err := prepareJunimo140SMAPIStaging(ctx, client, dir, staging, selected); err != nil {
		t.Fatal(err)
	}
	if err := prepareJunimo140GameMarkers(ctx, client, dir, staging, selected, time.Now()); err != nil {
		t.Fatalf("repeated payload proof after patched SMAPI install failed: %v", err)
	}
}
