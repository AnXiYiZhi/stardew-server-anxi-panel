//go:build integration

package stardew_junimo

import (
	"archive/tar"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
)

// Inputs are authorized, game-only raw tar archives visible to the Linux test
// process. They are streamed to Docker, so daemon-side bind paths are not needed.
// GAME_TAR contains the game root; SDK_TAR contains linux64/ and steamapps/
// directly, without a .steam-sdk parent directory. No Steam login occurs here.
func TestJunimo140SDKPublicBuildRealOptIn(t *testing.T) {
	if os.Getenv("ANXI_REAL_JUNIMO140_SDK") != "1" {
		t.Skip("set ANXI_REAL_JUNIMO140_SDK=1 with authorized game and SDK archives")
	}
	if runtime.GOOS != "linux" {
		t.Fatal("the real SDK staging test requires a Linux runner")
	}
	gameArchive := os.Getenv("ANXI_REAL_JUNIMO140_GAME_TAR")
	sdkArchive := os.Getenv("ANXI_REAL_JUNIMO140_SDK_TAR")
	validateSDKRealArchive(t, gameArchive, []string{"Stardew Valley.dll", "Content/ContentHashes.json"})
	validateSDKRealArchive(t, sdkArchive, []string{"linux64/steamclient.so", "linux64/libsteamwebrtc.so", "steamapps/appmanifest_1007.acf"})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	if err := exec.CommandContext(ctx, "docker", "info").Run(); err != nil {
		t.Fatalf("Docker is unavailable: %v", err)
	}
	dir := t.TempDir()
	client := paneldocker.NewClient(paneldocker.Options{DockerPath: "docker"})
	imageRef := os.Getenv("ANXI_REAL_JUNIMO140_SDK_IMAGE")
	if imageRef == "" {
		imageRef = "sdvd/server:1.5.0-preview.140"
	}
	image, err := client.RuntimeImageInspect(ctx, dir, imageRef)
	if err != nil || image.Digest != junimo140Digest {
		t.Fatalf("the fixed Junimo .140 image is unavailable: %v", err)
	}
	selected := RuntimeUpdateSelectedImage{Image: imageRef, ImageID: image.ID, Digest: image.Digest}
	identity, err := randomHex(8)
	if err != nil {
		t.Fatal(err)
	}
	project := "sdk-real-" + identity
	type ownedVolume struct{ name, applyID string }
	var owned []ownedVolume
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cleanupCancel()
		for i := len(owned) - 1; i >= 0; i-- {
			volume := owned[i]
			scope := paneldocker.RuntimeJunimo140HelperScope{ApplyID: volume.applyID, Projects: []string{project}, ImageRef: imageRef, ImageID: image.ID}
			if err := client.RuntimeStopJunimo140Helpers(cleanupCtx, dir, scope); err != nil {
				t.Errorf("owned SDK test helper cleanup failed: %v", err)
				continue
			}
			if err := client.RuntimeRemoveJunimo140Staging(cleanupCtx, dir, project, volume.name, volume.applyID); err != nil {
				t.Errorf("owned SDK test volume cleanup failed: %v", err)
			}
		}
	})
	create := func() ownedVolume {
		t.Helper()
		id := newRuntimeApplyID()
		volume := ownedVolume{name: project + "_anxi-smapi-update-" + strings.TrimPrefix(id, "apply_"), applyID: id}
		// Retain the intended identity even if Docker's create response is lost;
		// cleanup still requires the original labels and an unused exact volume.
		owned = append(owned, volume)
		if err := client.RuntimeCreateJunimo140Staging(ctx, dir, project, volume.name, volume.applyID); err != nil {
			t.Fatalf("create owned SDK test volume: %v", err)
		}
		return volume
	}
	source, good, bad, badSDK := create(), create(), create(), create()
	run := func(volume ownedVolume, suffix, operation, script, archive string) []byte {
		t.Helper()
		name := "anxi-j140-" + strings.TrimPrefix(volume.applyID, "apply_") + "-sdk-real-" + suffix
		prior, err := exec.CommandContext(ctx, "docker", "container", "ls", "--all", "--filter", "name=^/"+name+"$", "--format", "{{.Names}}").Output()
		if err != nil || strings.TrimSpace(string(prior)) != "" {
			t.Fatal("SDK test helper name collision or unavailable inventory")
		}
		args := []string{"run", "--rm", "--name", name,
			"--label", "com.anxi-panel.junimo140-helper=true",
			"--label", "com.anxi-panel.compose-project=" + project,
			"--label", "com.anxi-panel.runtime-update-apply-id=" + volume.applyID,
			"--label", "com.anxi-panel.junimo140-operation=" + operation,
			"--pull", "never", "--network", "none", "--user", "0:0", "--read-only",
			"--mount", "type=volume,src=" + volume.name + ",dst=/game", "--entrypoint", "sh"}
		if archive != "" {
			args = append(args, "-i")
		}
		args = append(args, image.ID, "-eu", "-c", script)
		command := exec.CommandContext(ctx, "docker", args...)
		if archive != "" {
			file, err := os.Open(archive)
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			command.Stdin = file
		}
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("owned SDK fixture %s failed: %v: %s", suffix, err, output)
		}
		return output
	}
	run(source, "game-import", "restore", `tar -xpf - -C /game; rm -f /game/.download-manifest-413150`, gameArchive)
	run(source, "sdk-import", "restore", `rm -rf /game/.steam-sdk; mkdir /game/.steam-sdk; tar -xpf - -C /game/.steam-sdk; rm -f /game/.steam-sdk/.download-manifest-1007`, sdkArchive)
	sdkData, err := client.RuntimeReadJunimo140GameEvidence(ctx, dir, source.name, imageRef, image.ID, "sdk")
	if err != nil {
		t.Fatal(err)
	}
	sdk, err := parseCompleteSDKManifest(sdkData)
	if err != nil {
		t.Fatalf("real SDK installation manifest was rejected: %v", err)
	}
	actualManifest := sdk.InstalledDepots["1006"]
	if actualManifest == "6403079453713498174" {
		t.Fatal("this regression requires a real SDK depot different from the old fixed SDK")
	}
	const sdkFingerprint = `cd /game; sha256sum -- .steam-sdk/linux64/steamclient.so .steam-sdk/linux64/libsteamwebrtc.so .steam-sdk/steamapps/appmanifest_1007.acf`
	before := string(run(source, "sdk-before", "check-sha256", sdkFingerprint, ""))
	metadata, err := prepareJunimo140SharedGameStaging(ctx, client, dir, source.name, good.name, selected)
	if err != nil || !junimo140SMAPIIdentityVerified(metadata) {
		t.Fatalf("production staging rejected the real current SDK: %v", err)
	}
	// Exercise the current-reader subprocess path too. These owned test volumes
	// have no game/Auth holders; this is not running-game or SDK ABI evidence.
	currentResources := func() []string {
		t.Helper()
		var snapshot []string
		for _, args := range [][]string{
			{"container", "ls", "--all", "--quiet", "--no-trunc", "--filter", "label=com.anxi-panel.current-runtime-read=true"},
			{"volume", "ls", "--quiet"},
		} {
			output, err := exec.CommandContext(ctx, "docker", args...).Output()
			if err != nil {
				t.Fatal("current SDK test resource inventory unavailable")
			}
			entries := strings.Fields(string(output))
			sort.Strings(entries)
			snapshot = append(snapshot, strings.Join(entries, "\n"))
		}
		return snapshot
	}
	beforeCurrent := currentResources()
	currentSDK, err := client.RuntimeInspectCurrentJunimo140SDK(ctx, dir, good.name, imageRef, image.ID, []string{project})
	if err != nil || currentSDK.SDKFiles != 2 || currentSDK.SDKBytes <= 0 {
		t.Fatalf("current reader rejected the real installed SDK: %v", err)
	}
	currentSMAPI, err := client.RuntimeReadCurrentPinnedSMAPIMetadata(ctx, dir, good.name, imageRef, image.ID, []string{project})
	if err != nil || !junimo140SMAPIIdentityVerified(currentSMAPI) || !reflect.DeepEqual(currentSMAPI, metadata) {
		t.Fatalf("current reader lost the verified patched SMAPI metadata: %v", err)
	}
	modHashes, err := client.RuntimeReadJunimo140ModHashes(ctx, dir, imageRef, image.ID)
	if err != nil || len(modHashes) != 2 || len(modHashes["JunimoServer.dll"]) != 64 || len(modHashes["manifest.json"]) != 64 {
		t.Fatalf("image-only reader lost the fixed Junimo Mod hashes: %v", err)
	}
	if !reflect.DeepEqual(beforeCurrent, currentResources()) {
		t.Fatal("current metadata readers left a helper or automatic volume behind")
	}
	if after := string(run(good, "sdk-after", "check-sha256", sdkFingerprint, "")); after != before {
		t.Fatal("production staging changed the installed SDK payload or ACF")
	}
	if after := string(run(source, "source-after", "check-sha256", sdkFingerprint, "")); after != before {
		t.Fatal("production staging changed the source SDK")
	}
	var markers [2]paneldocker.RuntimeGameDownloadMarker
	for i, markerPath := range []string{"/game/.download-manifest-413150", "/game/.steam-sdk/.download-manifest-1007"} {
		data := run(good, "marker-"+strconv.Itoa(i), "game-markers", "cat "+markerPath, "")
		if err := json.Unmarshal(data, &markers[i]); err != nil {
			t.Fatalf("production completion marker could not be decoded: %v", err)
		}
	}
	pins, err := fixedJunimo140GamePins()
	if err != nil {
		t.Fatal(err)
	}
	gameManifestID, _ := strconv.ParseUint(pins.Game.ManifestID, 10, 64)
	sdkManifestID, _ := strconv.ParseUint(actualManifest, 10, 64)
	if markers[0].AppID != 413150 || markers[0].ManifestID != gameManifestID || markers[0].TotalFiles <= 3560 || markers[0].TotalBytes <= 0 {
		t.Fatal("production game marker lost the verified legacy game identity")
	}
	if markers[1].AppID != 1007 || markers[1].DepotID != 1006 || markers[1].ManifestID != sdkManifestID || markers[1].TotalFiles != 2 || markers[1].TotalBytes <= 0 {
		t.Fatal("production SDK marker did not record the real installed SDK depot")
	}
	if err := client.RuntimeClonePinnedJunimo140Game(ctx, dir, source.name, bad.name, imageRef, image.ID); err != nil {
		t.Fatal(err)
	}
	run(bad, "corrupt-core", "restore", `dd if=/dev/zero of="/game/Stardew Valley.dll" bs=1 count=1 conv=notrunc status=none`, "")
	if err := prepareJunimo140GameMarkers(ctx, client, dir, bad.name, selected, time.Now()); err == nil {
		t.Fatal("a damaged game core was accepted after SDK decoupling")
	}
	run(bad, "rejected-markers", "game-markers", `test ! -e /game/.download-manifest-413150; test ! -e /game/.steam-sdk/.download-manifest-1007`, "")
	if err := client.RuntimeClonePinnedJunimo140Game(ctx, dir, source.name, badSDK.name, imageRef, image.ID); err != nil {
		t.Fatal(err)
	}
	// Preserve the actual SDK's ELF header while removing its program table
	// and payload. Header shape alone must never publish a completion marker.
	run(badSDK, "truncate-sdk", "restore", `truncate -s 96 /game/.steam-sdk/linux64/steamclient.so`, "")
	beforeCurrent = currentResources()
	if _, err := client.RuntimeInspectCurrentJunimo140SDK(ctx, dir, badSDK.name, imageRef, image.ID, []string{project}); err == nil {
		t.Fatal("current reader accepted the truncated real SDK")
	}
	if !reflect.DeepEqual(beforeCurrent, currentResources()) {
		t.Fatal("rejected current SDK read left a helper or automatic volume behind")
	}
	if err := prepareJunimo140GameMarkers(ctx, client, dir, badSDK.name, selected, time.Now()); err == nil {
		t.Fatal("a truncated SDK with its original ELF header was accepted")
	}
	run(badSDK, "rejected-sdk-markers", "game-markers", `test ! -e /game/.download-manifest-413150; test ! -e /game/.steam-sdk/.download-manifest-1007`, "")
	run(source, "source-markers", "game-markers", `test ! -e /game/.download-manifest-413150; test ! -e /game/.steam-sdk/.download-manifest-1007`, "")
	t.Logf("real SDK build %s / depot manifest %s preserved; fixed game verified; damaged game and truncated SDK refused without publishing markers", sdk.BuildID, actualManifest)
}

func validateSDKRealArchive(t *testing.T, archive string, required []string) {
	t.Helper()
	if archive == "" {
		t.Fatal("both authorized game-root and SDK-root raw tar archives are required")
	}
	file, err := os.Open(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	found := make(map[string]bool)
	reader := tar.NewReader(file)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("invalid authorized input archive: %v", err)
		}
		clean := path.Clean(header.Name)
		regular := header.Typeflag == tar.TypeReg || header.Typeflag == tar.TypeRegA
		if path.IsAbs(header.Name) || clean == ".." || strings.HasPrefix(clean, "../") || strings.ContainsAny(header.Name, "\\\x00\r\n") || header.Typeflag != tar.TypeDir && !regular {
			t.Fatal("authorized fixture archive contains an unsafe path or non-file entry")
		}
		if regular {
			if found[clean] {
				t.Fatal("authorized fixture archive contains a duplicate file")
			}
			found[clean] = true
		}
	}
	for _, name := range required {
		if !found[name] {
			t.Fatalf("authorized fixture archive has the wrong root layout; missing %s", name)
		}
	}
}
