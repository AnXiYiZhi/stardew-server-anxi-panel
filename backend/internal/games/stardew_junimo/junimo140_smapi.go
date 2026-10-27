package stardew_junimo

import (
	"context"
	"errors"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
)

const junimo140SMAPIBuildID = "4.5.2-ec5447482be0"

// Extracted from /opt/smapi/internal/linux/install.dat in the fixed .140
// canonical image (junimo140Digest), archive SHA256
// 4e52a43b05ad9ac369dd7ed157f30a640c6133bf2c5341cc593929928275e855.
// This is a different build from the official 4.5.2 installer. A stamp alone
// cannot establish identity after another installer has overwritten the DLLs.
var junimo140SMAPICoreHashes = map[string]string{
	"StardewModdingAPI.dll":                "3c81bd222f11fe28e70794ab67bd51c0baeace100cd270d440379f73f26e8de3",
	"StardewModdingAPI.runtimeconfig.json": "a3c897cc8c0b0acb3b1f9617a9dde27ff240195e30fcb95457b75de9c8a40f7f",
	"smapi-internal/SMAPI.Toolkit.dll":     "55182e1d2eb764182e9f43c90fa2ddf0485dad41d11acbc452e482b9a58e3292",
	"smapi-internal/0Harmony.dll":          "4c5497325157c3855c45119098f34eff6009613db0e2e9c245a7ff46bc45da89",
}

func junimo140SMAPIIdentityVerified(metadata paneldocker.RuntimeSMAPIMetadata) bool {
	return metadata.BuildID == junimo140SMAPIBuildID && junimo140SMAPIPayloadVerified(metadata)
}

func junimo140SMAPIPayloadVerified(metadata paneldocker.RuntimeSMAPIMetadata) bool {
	if !metadata.Present || !metadata.RequiredFiles || metadata.ImageBuildID != junimo140SMAPIBuildID {
		return false
	}
	for name, hash := range junimo140SMAPICoreHashes {
		if metadata.CoreHashes[name] != hash {
			return false
		}
	}
	return true
}

type imageSMAPIStagingDocker interface {
	RuntimeReadPinnedSMAPIMetadata(context.Context, string, string, string, string) (paneldocker.RuntimeSMAPIMetadata, error)
	RuntimeInstallImageSMAPI(context.Context, string, string, string, string) error
	RuntimeCommitImageSMAPIStamp(context.Context, string, string, string, string, string) error
}

// A valid stamp is published only after the actual staged bytes match the
// fixed image's payload, then read back with the same immutable image ID.
func prepareJunimo140SMAPIStaging(ctx context.Context, docker imageSMAPIStagingDocker, dataDir, volume string, image RuntimeUpdateSelectedImage) (paneldocker.RuntimeSMAPIMetadata, error) {
	if image.Digest != junimo140Digest {
		return paneldocker.RuntimeSMAPIMetadata{}, errors.New("image-managed SMAPI requires the fixed Junimo .140 digest")
	}
	read := func() (paneldocker.RuntimeSMAPIMetadata, error) {
		return docker.RuntimeReadPinnedSMAPIMetadata(ctx, dataDir, volume, image.Image, image.ImageID)
	}
	current, err := read()
	if err != nil {
		return current, err
	}
	if junimo140SMAPIIdentityVerified(current) {
		return current, nil
	}
	if err := docker.RuntimeInstallImageSMAPI(ctx, dataDir, volume, image.Image, image.ImageID); err != nil {
		return current, err
	}
	staged, err := read()
	if err != nil {
		return staged, err
	}
	if !junimo140SMAPIPayloadVerified(staged) {
		return staged, errors.New("staged SMAPI does not match the fixed image payload; no build stamp was committed")
	}
	if err := docker.RuntimeCommitImageSMAPIStamp(ctx, dataDir, volume, image.Image, image.ImageID, junimo140SMAPIBuildID); err != nil {
		return staged, err
	}
	verified, err := read()
	if err != nil {
		return verified, err
	}
	if !junimo140SMAPIIdentityVerified(verified) {
		return verified, errors.New("image-managed SMAPI identity could not be verified after stamp commit")
	}
	return verified, nil
}
