package stardew_junimo

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
)

//go:embed config/junimo140_game_integrity.json
var junimo140LegacyGamePins []byte

type junimo140DepotPin struct{ AppID, BuildID, DepotID, ManifestID string }
type junimo140GamePins struct {
	SchemaVersion int               `json:"schemaVersion"`
	Game          junimo140DepotPin `json:"game"`
	SDK           junimo140DepotPin `json:"sdk"`
	Content       struct {
		ManifestSHA256 string `json:"manifestSHA256"`
		Entries        int    `json:"entries"`
	} `json:"content"`
	CoreFiles map[string]string `json:"coreFiles"`
}

type junimo140GameIntegrityDocker interface {
	RuntimeReadJunimo140GameEvidence(context.Context, string, string, string, string, string) ([]byte, error)
	RuntimeVerifyJunimo140GameFiles(context.Context, string, string, string, string, string, map[string]string) (paneldocker.RuntimeGameFileStats, error)
	RuntimeCommitJunimo140GameMarkers(context.Context, string, string, string, string, paneldocker.RuntimeGameDownloadMarker, paneldocker.RuntimeGameDownloadMarker) error
}

type junimo140SharedGameDocker interface {
	junimo140GameIntegrityDocker
	imageSMAPIStagingDocker
	RuntimeClonePinnedJunimo140Game(context.Context, string, string, string, string, string) error
}

// The group coordinator must publish only the result of this complete staging
// operation. An interrupted/failed operation leaves an unpublished owned clone;
// it never edits the source volume or claims the game is ready on file presence.
func prepareJunimo140SharedGameStaging(ctx context.Context, docker junimo140SharedGameDocker, dir, source, staging string, image RuntimeUpdateSelectedImage) (paneldocker.RuntimeSMAPIMetadata, error) {
	if image.Digest != junimo140Digest {
		return paneldocker.RuntimeSMAPIMetadata{}, errors.New("shared game staging requires fixed Junimo .140")
	}
	if err := docker.RuntimeClonePinnedJunimo140Game(ctx, dir, source, staging, image.Image, image.ImageID); err != nil {
		return paneldocker.RuntimeSMAPIMetadata{}, err
	}
	if err := prepareJunimo140GameMarkers(ctx, docker, dir, staging, image, time.Now()); err != nil {
		return paneldocker.RuntimeSMAPIMetadata{}, err
	}
	metadata, err := prepareJunimo140SMAPIStaging(ctx, docker, dir, staging, image)
	if err != nil {
		return metadata, err
	}
	// The image installer may replace launchers. Recheck the supported game/SDK
	// payload and read back both committed markers after that last mutation.
	if err := prepareJunimo140GameMarkers(ctx, docker, dir, staging, image, time.Now()); err != nil {
		return metadata, err
	}
	return metadata, nil
}

func fixedJunimo140GamePins() (junimo140GamePins, error) {
	var pins junimo140GamePins
	if json.Unmarshal(junimo140LegacyGamePins, &pins) != nil || pins.SchemaVersion != 1 || len(pins.CoreFiles) != 228 || pins.Content.Entries != 3560 || len(pins.Content.ManifestSHA256) != 64 {
		return pins, errors.New("invalid embedded legacy game integrity catalog")
	}
	return pins, nil
}

func verifyFixedDepotManifest(data []byte, pin junimo140DepotPin) error {
	manifest, err := parseSteamAppManifest(data, pin.AppID)
	if err != nil || manifest.BuildID != pin.BuildID || manifest.StateFlags != "4" || len(manifest.InstalledDepots) != 1 || manifest.InstalledDepots[pin.DepotID] != pin.ManifestID {
		return errors.New("game or SDK does not match the supported complete legacy build")
	}
	return nil
}

func fixedJunimo140ContentFiles(data []byte, pins junimo140GamePins) (map[string]string, error) {
	hash := sha256.Sum256(data)
	if hex.EncodeToString(hash[:]) != pins.Content.ManifestSHA256 {
		return nil, errors.New("unsupported or incomplete legacy content catalog")
	}
	var catalog map[string]string
	if json.Unmarshal(data, &catalog) != nil || len(catalog) != pins.Content.Entries {
		return nil, errors.New("invalid fixed legacy content catalog")
	}
	files := make(map[string]string, len(catalog))
	for name, hash := range catalog {
		files["Content/"+name] = hash
	}
	return files, nil
}

// This migrates only a byte-for-byte known, previously tested SteamCMD payload.
// It does not validate a license or replace Steam/GOG authentication. Unknown
// builds and filtered content require a separately authorized download instead.
// Every operation is restricted to an unused transaction-owned staging volume.
func prepareJunimo140GameMarkers(ctx context.Context, docker junimo140GameIntegrityDocker, dir, volume string, image RuntimeUpdateSelectedImage, now time.Time) error {
	if image.Digest != junimo140Digest {
		return errors.New("fixed game migration requires the pinned Junimo .140 image")
	}
	pins, err := fixedJunimo140GamePins()
	if err != nil {
		return err
	}
	for _, item := range []struct {
		kind string
		pin  junimo140DepotPin
	}{{"game", pins.Game}, {"sdk", pins.SDK}} {
		data, err := docker.RuntimeReadJunimo140GameEvidence(ctx, dir, volume, image.Image, image.ImageID, item.kind)
		if err != nil {
			return fmt.Errorf("read legacy %s identity: %w", item.kind, err)
		}
		if err := verifyFixedDepotManifest(data, item.pin); err != nil {
			return err
		}
	}
	data, err := docker.RuntimeReadJunimo140GameEvidence(ctx, dir, volume, image.Image, image.ImageID, "content")
	if err != nil {
		return err
	}
	content, err := fixedJunimo140ContentFiles(data, pins)
	if err != nil {
		return err
	}
	core := make(map[string]string, len(pins.CoreFiles)+1)
	for file, hash := range pins.CoreFiles {
		core[file] = hash
	}
	core["Content/ContentHashes.json"] = pins.Content.ManifestSHA256
	coreStats, err := docker.RuntimeVerifyJunimo140GameFiles(ctx, dir, volume, image.Image, image.ImageID, "sha256", core)
	if err != nil {
		return fmt.Errorf("verify complete legacy game and SDK core: %w", err)
	}
	contentStats, err := docker.RuntimeVerifyJunimo140GameFiles(ctx, dir, volume, image.Image, image.ImageID, "md5", content)
	if err != nil {
		return fmt.Errorf("verify complete legacy game content: %w", err)
	}
	marker := func(pin junimo140DepotPin, files, bytes int64) paneldocker.RuntimeGameDownloadMarker {
		app, _ := strconv.ParseUint(pin.AppID, 10, 32)
		depot, _ := strconv.ParseUint(pin.DepotID, 10, 32)
		manifest, _ := strconv.ParseUint(pin.ManifestID, 10, 64)
		return paneldocker.RuntimeGameDownloadMarker{AppID: uint32(app), DepotID: uint32(depot), ManifestID: manifest, TargetOS: "linux", TotalBytes: bytes, TotalFiles: files, DownloadedAt: now.UTC().Format(time.RFC3339Nano)}
	}
	if contentStats.SDKFiles != 0 || contentStats.GameFiles != int64(pins.Content.Entries) || coreStats.SDKFiles == 0 || coreStats.GameFiles == 0 {
		return errors.New("incomplete legacy game integrity proof")
	}
	return docker.RuntimeCommitJunimo140GameMarkers(ctx, dir, volume, image.Image, image.ImageID,
		marker(pins.Game, coreStats.GameFiles+contentStats.GameFiles, coreStats.GameBytes+contentStats.GameBytes), marker(pins.SDK, coreStats.SDKFiles, coreStats.SDKBytes))
}
