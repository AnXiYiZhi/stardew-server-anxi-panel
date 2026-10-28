package docker

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"
)

var smapiImageBuildIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.+_-]{0,127}$`)

func imageSMAPIHelperArgs(volume, operation string) []string {
	boundary := strings.LastIndex(volume, "_anxi-smapi-update-")
	project, id := volume[:boundary], volume[boundary+len("_anxi-smapi-update-"):]
	return []string{"--name", "anxi-j140-" + id + "-smapi-" + operation,
		"--label", "com.anxi-panel.junimo140-helper=true",
		"--label", "com.anxi-panel.compose-project=" + project,
		"--label", "com.anxi-panel.runtime-update-apply-id=apply_" + id,
		"--label", "com.anxi-panel.junimo140-operation=smapi-" + operation}
}

// Clone only to an unused transaction-owned volume, using the immutable .140
// helper image. Both sides of the tar pipeline must succeed; all source file
// bytes are then checked before an installer is allowed to alter the clone.
// This proves a consistent copy, not the source game's depot authenticity.
func (c *Client) RuntimeClonePinnedJunimo140Game(ctx context.Context, dir, source, staging, imageRef, imageID string) error {
	if !dockerVolumePattern.MatchString(source) || !smapiStagingVolumePattern.MatchString(staging) || source == staging {
		return errors.New("invalid pinned game staging clone")
	}
	if err := c.unusedSMAPIStaging(ctx, dir, staging); err != nil {
		return err
	}
	if _, err := c.RuntimeVolumeInspect(ctx, dir, source); err != nil {
		return err
	}
	if err := c.pinnedSMAPIImage(ctx, dir, imageRef, imageID); err != nil {
		return err
	}
	const script = `set -euo pipefail
cd /source
find . -type f -print0 | sort -z | xargs -0 -r sha256sum > /tmp/source.sha256
test -s /tmp/source.sha256
find /target -mindepth 1 -maxdepth 1 -exec rm -rf -- {} +
tar --acls --xattrs --numeric-owner -cpf - -C /source . | tar --acls --xattrs --same-owner --same-permissions -xpf - -C /target
cd /target
sha256sum --check --quiet /tmp/source.sha256
sync -f /target`
	args := append([]string{"run", "--rm"}, imageSMAPIHelperArgs(staging, "clone")...)
	args = append(args, "--pull", "never", "--network", "none", "--read-only", "--tmpfs", "/tmp:size=64m",
		"--mount", "type=volume,src="+source+",dst=/source,readonly",
		"--mount", "type=volume,src="+staging+",dst=/target",
		"--entrypoint", "timeout", imageID, "--kill-after=2s", "600s", "bash", "-euo", "pipefail", "-c", script)
	_, err := c.run(ctx, "clone and verify pinned shared game staging", dir, 12*time.Minute, args...)
	return err
}

func (c *Client) pinnedSMAPIImage(ctx context.Context, dir, imageRef, expectedID string) error {
	if validateRestrictedImageRef(imageRef) != nil || !runtimeDigestPattern.MatchString(expectedID) {
		return errors.New("invalid pinned SMAPI image")
	}
	image, err := c.RuntimeImageInspect(ctx, dir, imageRef)
	if err != nil || image.ID != expectedID {
		return errors.New("SMAPI image identity changed after preflight")
	}
	return nil
}

func (c *Client) unusedSMAPIStaging(ctx context.Context, dir, volume string) error {
	if !smapiStagingVolumePattern.MatchString(volume) {
		return errors.New("invalid image-managed SMAPI staging volume")
	}
	project := volume[:strings.LastIndex(volume, "_anxi-smapi-update-")]
	_, raw, err := c.runWithEnvironmentRaw(ctx, "verify SMAPI staging ownership", dir, c.timeouts.Ps, nil, "volume", "inspect", volume)
	if err != nil {
		return err
	}
	var metadata []struct {
		Name   string
		Labels map[string]string
	}
	if json.Unmarshal([]byte(raw), &metadata) != nil || len(metadata) != 1 || metadata[0].Name != volume ||
		metadata[0].Labels["com.anxi-panel.smapi-update-staging"] != "true" || metadata[0].Labels["com.anxi-panel.compose-project"] != project {
		return errors.New("SMAPI staging ownership mismatch")
	}
	result, err := c.run(ctx, "verify unused SMAPI staging", dir, c.timeouts.Ps,
		"ps", "-a", "--filter", "volume="+volume, "--format", "{{.Names}}")
	if err != nil {
		return err
	}
	if strings.TrimSpace(result.Stdout) != "" {
		return errors.New("SMAPI staging volume is already referenced")
	}
	return nil
}

// RuntimeValidateUnusedSMAPIStaging exposes the same ownership and no-holder
// gate to the base installer before SteamCMD writes its first byte.
func (c *Client) RuntimeValidateUnusedSMAPIStaging(ctx context.Context, dir, volume string) error {
	return c.unusedSMAPIStaging(ctx, dir, volume)
}

// RuntimeVolumeExists distinguishes an absent volume from an inspect failure.
// The exact-name filter cannot select another installation's volume.
func (c *Client) RuntimeVolumeExists(ctx context.Context, dir, volume string) (bool, error) {
	if !dockerVolumePattern.MatchString(volume) {
		return false, errors.New("invalid game-data volume name")
	}
	result, err := c.run(ctx, "check exact game-data volume", dir, c.timeouts.Ps,
		"volume", "ls", "--filter", "name=^"+regexp.QuoteMeta(volume)+"$", "--format", "{{.Name}}")
	if err != nil {
		return false, err
	}
	value := strings.TrimSpace(result.Stdout)
	if value == "" {
		return false, nil
	}
	if value != volume {
		return false, errors.New("exact game-data volume query returned an unexpected identity")
	}
	return true, nil
}

// RuntimeInstallImageSMAPI operates only on an unused transaction-owned staging
// volume. The installer is taken from the already verified immutable image ID.
// The driver must verify the payload hashes before publishing its build stamp.
func (c *Client) RuntimeInstallImageSMAPI(ctx context.Context, dir, volume, imageRef, imageID string) error {
	if !smapiStagingVolumePattern.MatchString(volume) || validateRestrictedImageRef(imageRef) != nil || !runtimeDigestPattern.MatchString(imageID) {
		return errors.New("invalid image-managed SMAPI install request")
	}
	if err := c.unusedSMAPIStaging(ctx, dir, volume); err != nil {
		return err
	}
	if err := c.pinnedSMAPIImage(ctx, dir, imageRef, imageID); err != nil {
		return err
	}
	const script = `set -eu
test -s /opt/smapi/BUILD_ID
test -x /opt/smapi/internal/linux/SMAPI.Installer
rm -f /game/smapi-internal/.sdvd-smapi-build
for path in StardewModdingAPI StardewModdingAPI.dll StardewModdingAPI.deps.json StardewModdingAPI.runtimeconfig.json smapi-internal/SMAPI.Toolkit.dll smapi-internal/0Harmony.dll Mods/ConsoleCommands/manifest.json; do
    rm -f "/game/$path"
done
HOME=/tmp timeout --kill-after=2s 120s /opt/smapi/internal/linux/SMAPI.Installer --no-prompt --install --game-path /game
for path in StardewModdingAPI StardewModdingAPI.dll StardewModdingAPI.deps.json StardewModdingAPI.runtimeconfig.json smapi-internal/SMAPI.Toolkit.dll smapi-internal/0Harmony.dll Mods/ConsoleCommands/manifest.json; do
    test -s "/game/$path"
    test ! -L "/game/$path"
done`
	args := append([]string{"run", "--rm"}, imageSMAPIHelperArgs(volume, "install")...)
	args = append(args, "--pull", "never", "--network", "none", "--read-only", "--tmpfs", "/tmp:exec,size=768m",
		"--mount", "type=volume,src="+volume+",dst=/game", "--entrypoint", "sh", imageID, "-c", script)
	_, err := c.run(ctx, "install image-managed SMAPI on staging", dir, c.timeouts.Up, args...)
	return err
}

func (c *Client) RuntimeReadPinnedSMAPIMetadata(ctx context.Context, dir, volume, imageRef, imageID string) (RuntimeSMAPIMetadata, error) {
	if !dockerVolumePattern.MatchString(volume) {
		return RuntimeSMAPIMetadata{}, errors.New("invalid game-data volume")
	}
	if err := c.pinnedSMAPIImage(ctx, dir, imageRef, imageID); err != nil {
		return RuntimeSMAPIMetadata{}, err
	}
	if _, err := c.RuntimeVolumeInspect(ctx, dir, volume); err != nil {
		return RuntimeSMAPIMetadata{}, err
	}
	return c.readSMAPIMetadataFromImageID(ctx, dir, volume, imageID)
}

func (c *Client) RuntimeCommitImageSMAPIStamp(ctx context.Context, dir, volume, imageRef, imageID, buildID string) error {
	if !smapiStagingVolumePattern.MatchString(volume) || validateRestrictedImageRef(imageRef) != nil ||
		!runtimeDigestPattern.MatchString(imageID) || !smapiImageBuildIDPattern.MatchString(buildID) {
		return errors.New("invalid image-managed SMAPI stamp request")
	}
	if err := c.unusedSMAPIStaging(ctx, dir, volume); err != nil {
		return err
	}
	if err := c.pinnedSMAPIImage(ctx, dir, imageRef, imageID); err != nil {
		return err
	}
	const script = `set -eu
test "$(cat /opt/smapi/BUILD_ID)" = "$1"
test -d /game/smapi-internal
test ! -L /game/smapi-internal
sync -f /game
tmp=$(mktemp /game/smapi-internal/.sdvd-smapi-build.XXXXXX)
trap 'rm -f "$tmp"' EXIT
printf '%s\n' "$1" > "$tmp"
sync -f "$tmp"
mv -f "$tmp" /game/smapi-internal/.sdvd-smapi-build
sync -f /game/smapi-internal`
	args := append([]string{"run", "--rm"}, imageSMAPIHelperArgs(volume, "stamp")...)
	args = append(args, "--pull", "never", "--network", "none", "--read-only", "--tmpfs", "/tmp",
		"--mount", "type=volume,src="+volume+",dst=/game", "--entrypoint", "sh", imageID, "-c", script, "--", buildID)
	_, err := c.run(ctx, "commit verified image-managed SMAPI identity", dir, c.timeouts.Ps, args...)
	return err
}
