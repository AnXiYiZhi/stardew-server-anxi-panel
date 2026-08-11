//go:build integration

package stardew_junimo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
)

func TestDockerSMAPIBundledSyncMaterializesBeforeServerStart(t *testing.T) {
	if os.Getenv("PANEL_RUN_SMAPI_BUNDLED_SYNC_TEST") != "1" {
		t.Skip("set PANEL_RUN_SMAPI_BUNDLED_SYNC_TEST=1 to run the real Docker bundled-Mod sync gate")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	client := paneldocker.NewClient(paneldocker.Options{})
	dataDir := filepath.Join(t.TempDir(), "stardew")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	volume := "anxi-smapi-sync-" + strings.ToLower(fmt.Sprintf("%x", time.Now().UnixNano()))
	if err := sjconfig.UpdateEnvFile(filepath.Join(dataDir, ".env"), map[string]string{"GAME_DATA_VOLUME": volume}); err != nil {
		t.Fatal(err)
	}
	image := strings.TrimSpace(os.Getenv("PANEL_SMAPI_SYNC_IMAGE"))
	if image == "" {
		image = serverImageDefault(TestedImageTag)
	}
	defer func() {
		_, _ = client.RemoveContainersByVolume(context.Background(), dataDir, []string{volume})
		_, _ = client.RemoveVolumes(context.Background(), dataDir, []string{volume})
	}()

	populate := `set -eu
mkdir -p /data/game/Mods/ConsoleCommands /data/game/Mods/SaveBackup
printf '%s\n' '{"Name":"Console Commands","UniqueID":"SMAPI.ConsoleCommands","Version":"4.5.2"}' > /data/game/Mods/ConsoleCommands/manifest.json
printf '%s\n' '{"Name":"Save Backup","UniqueID":"SMAPI.SaveBackup","Version":"4.5.2"}' > /data/game/Mods/SaveBackup/manifest.json`
	exitCode, err := client.RunContainerTTY(ctx, paneldocker.ContainerTTYRunOpts{
		ImageRef: image, Entrypoint: []string{"/bin/sh"}, User: "root", Command: []string{"-c", populate},
		Binds: []string{volume + ":/data/game"},
	}, nil, func(string) {})
	if err != nil || exitCode != 0 {
		t.Fatalf("seed real game-data volume: exit=%d err=%v", exitCode, err)
	}
	driver := New(client, nil, nil, nil)
	changed, err := driver.EnsureManagedSMAPIBundledMods(ctx, dataDir, image, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("fresh real Docker sync should publish managed SMAPI Mods")
	}
	loaded, err := bundledSMAPIRuntimeLoadedMods(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 2 {
		t.Fatalf("loaded managed SMAPI mods = %#v", loaded)
	}
	changed, err = driver.EnsureManagedSMAPIBundledMods(ctx, dataDir, image, nil)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("second real Docker sync should be idempotent")
	}

	root := modsDir(dataDir)
	destination := filepath.Join(root, "smapi")
	interruptedBackup := filepath.Join(root, ".smapi-backup-integration-interrupted")
	if err := os.Rename(destination, interruptedBackup); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".smapi-sync-integration-interrupted"), 0o755); err != nil {
		t.Fatal(err)
	}
	changed, err = driver.EnsureManagedSMAPIBundledMods(ctx, dataDir, image, nil)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("recovered identical real Docker sync should not republish")
	}
	ownedArtifacts, err := filepath.Glob(filepath.Join(root, ".smapi-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ownedArtifacts) != 0 {
		t.Fatalf("recovered sync left owned artifacts: %v", ownedArtifacts)
	}
}
