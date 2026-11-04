package stardew_junimo

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
)

// SteamCMD and Junimo install the current public SDK independently of the game
// build. Its ACF proves the installed Linux depot; its build is diagnostic only.
func parseCompleteSDKManifest(data []byte) (steamAppManifest, error) {
	manifest, err := parseSteamAppManifest(data, "1007")
	if err != nil {
		return manifest, errors.New("Steamworks SDK installation manifest is invalid")
	}
	if manifest.StateFlags != "4" || len(manifest.InstalledDepots) != 1 || !positiveDecimal(manifest.InstalledDepots["1006"]) {
		return manifest, errors.New("Steamworks SDK installation is incomplete or its Linux depot is missing")
	}
	if _, err := strconv.ParseUint(manifest.InstalledDepots["1006"], 10, 64); err != nil {
		return manifest, errors.New("Steamworks SDK installed depot manifest is invalid")
	}
	return manifest, nil
}

func installedSDKDownloadMarker(data []byte, stats paneldocker.RuntimeGameFileStats, now time.Time) (paneldocker.RuntimeGameDownloadMarker, error) {
	manifest, err := parseCompleteSDKManifest(data)
	if err != nil {
		return paneldocker.RuntimeGameDownloadMarker{}, err
	}
	if stats.SDKFiles != 2 || stats.SDKBytes <= 0 || stats.GameFiles != 0 || stats.GameBytes != 0 {
		return paneldocker.RuntimeGameDownloadMarker{}, errors.New("installed SDK runtime file verification is incomplete")
	}
	id, _ := strconv.ParseUint(manifest.InstalledDepots["1006"], 10, 64)
	return paneldocker.RuntimeGameDownloadMarker{AppID: 1007, DepotID: 1006, ManifestID: id, TargetOS: "linux", TotalFiles: stats.SDKFiles, TotalBytes: stats.SDKBytes, DownloadedAt: now.UTC().Format(time.RFC3339Nano)}, nil
}

func (d *Driver) preflightJunimo140Content(ctx context.Context, docker RuntimeUpdateDockerService, instance registry.Instance, image string) error {
	reader, ok := docker.(interface {
		RuntimeReadContentVersions(context.Context, string, string, string) (paneldocker.RuntimeContentRead, error)
	})
	if !ok {
		return errors.New("read-only game and SDK installation inspection is unavailable")
	}
	volume, err := GameDataVolumeName(instance.DataDir)
	if err != nil {
		return err
	}
	read, err := reader.RuntimeReadContentVersions(ctx, instance.DataDir, volume, image)
	if err != nil {
		return fmt.Errorf("read game and SDK installation manifests before stopping: %w", err)
	}
	pins, err := fixedJunimo140GamePins()
	if err != nil {
		return err
	}
	if err := verifyFixedDepotManifest(read.GameManifest, pins.Game); err != nil {
		return err
	}
	_, err = parseCompleteSDKManifest(read.SDKManifest)
	return err
}
