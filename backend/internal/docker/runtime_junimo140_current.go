package docker

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
)

// The current reader reports the two fixed Galaxy hashes in this order. It
// does not return library bytes or accept an arbitrary filename from a caller.
const RuntimeCurrentJunimo140GalaxyPairEvidence = "galaxy-pair-sha256"

const currentJunimo140GalaxyPairScript = `set -eu
test ! -L /game
cd /game
for file in libGalaxy64.so libGalaxyCSharpGlue.so; do
    test ! -L "$file"
    test -f "$file"
    size=$(stat -c %s -- "$file")
    test "$size" -gt 0 && test "$size" -le 67108864 || exit 1
    checksum=$(sha256sum -- "$file")
    printf '%s\n' "${checksum%% *}"
done`

// Nil is reserved for the original staging callers: their unused-volume guard
// is unchanged. Current readers need a nonempty authoritative project scope,
// and recheck every actual volume holder before each read/helper invocation.
func (c *Client) junimo140ReadVolume(ctx context.Context, dir, volume string, projects []string) error {
	if projects == nil {
		return c.unusedSMAPIStaging(ctx, dir, volume)
	}
	if len(projects) == 0 {
		return errors.New("current game reads require the complete known group")
	}
	return c.RuntimeAssertJunimo140GameGroup(ctx, dir, volume, projects, false)
}

func junimo140ReadMount(volume string, projects []string) string {
	mount := "type=volume,src=" + volume + ",dst=/game,readonly"
	if projects != nil {
		mount += ",volume-nocopy"
	}
	return mount
}

func junimo140ReadHelperArgs(volume, operation string, projects []string) ([]string, error) {
	if projects == nil {
		return imageSMAPIHelperArgs(volume, operation), nil
	}
	var id [12]byte
	if _, err := rand.Read(id[:]); err != nil {
		return nil, err
	}
	return []string{"--name", "anxi-current-j140-" + hex.EncodeToString(id[:]), "--user", "0:0", "--cap-drop", "ALL", "--cap-add", "DAC_READ_SEARCH", "--security-opt", "no-new-privileges",
		"--label", "com.anxi-panel.current-runtime-read=true", "--label", "com.anxi-panel.game-data-volume=" + volume}, nil
}

func (c *Client) RuntimeReadCurrentJunimo140GameEvidence(ctx context.Context, dir, volume, imageRef, imageID, kind string, projects []string) ([]byte, error) {
	if len(projects) == 0 {
		return nil, errors.New("current game scope missing")
	}
	if kind == RuntimeCurrentJunimo140GalaxyPairEvidence {
		return c.runJunimo140GameEvidence(ctx, dir, volume, imageRef, imageID, kind, projects, currentJunimo140GalaxyPairScript)
	}
	return c.readJunimo140GameEvidence(ctx, dir, volume, imageRef, imageID, kind, projects)
}

func (c *Client) RuntimeVerifyCurrentJunimo140GameFiles(ctx context.Context, dir, volume, imageRef, imageID, algorithm string, files map[string]string, projects []string) (RuntimeGameFileStats, error) {
	if len(projects) == 0 {
		return RuntimeGameFileStats{}, errors.New("current game scope missing")
	}
	return c.verifyJunimo140GameFiles(ctx, dir, volume, imageRef, imageID, algorithm, files, projects)
}

func (c *Client) RuntimeInspectCurrentJunimo140SDK(ctx context.Context, dir, volume, imageRef, imageID string, projects []string) (RuntimeGameFileStats, error) {
	if len(projects) == 0 {
		return RuntimeGameFileStats{}, errors.New("current game scope missing")
	}
	return inspectSDKELFMetadata(func(requests [2]sdkELFRange, previous *[2]sdkELFMetadata) ([2]sdkELFMetadata, error) {
		return c.readJunimo140SDKMetadataWithScope(ctx, dir, volume, imageRef, imageID, requests, previous, projects)
	})
}

func (c *Client) RuntimeReadCurrentPinnedSMAPIMetadata(ctx context.Context, dir, volume, imageRef, imageID string, projects []string) (RuntimeSMAPIMetadata, error) {
	if len(projects) == 0 {
		return RuntimeSMAPIMetadata{}, errors.New("current game scope missing")
	}
	if err := c.junimo140ReadVolume(ctx, dir, volume, projects); err != nil {
		return RuntimeSMAPIMetadata{}, err
	}
	if err := c.pinnedSMAPIImage(ctx, dir, imageRef, imageID); err != nil {
		return RuntimeSMAPIMetadata{}, err
	}
	return c.readSMAPIMetadataFromImageIDWithCurrentScope(ctx, dir, volume, imageID, projects)
}
