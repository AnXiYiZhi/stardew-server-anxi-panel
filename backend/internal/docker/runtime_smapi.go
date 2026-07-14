package docker

import (
	"context"
	"errors"
	"path/filepath"
	"regexp"
	"strings"
)

var smapiStagingVolumePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*_anxi-smapi-update-[a-f0-9]{24}$`)
var smapiInstallerFilePattern = regexp.MustCompile(`^SMAPI-[0-9]+\.[0-9]+\.[0-9]+-installer\.zip$`)

type RuntimeSMAPIMetadata struct {
	Present         bool
	RequiredFiles   bool
	Version         string
	VersionEvidence string
}

func (c *Client) RuntimeCreateSMAPIStagingVolume(ctx context.Context, dir, project, name string) error {
	if !composeProjectPattern.MatchString(project) || !smapiStagingVolumePattern.MatchString(name) || !strings.HasPrefix(name, project+"_anxi-smapi-update-") {
		return errors.New("invalid SMAPI staging volume")
	}
	_, err := c.run(ctx, "create SMAPI staging volume", dir, c.timeouts.Version,
		"volume", "create", "--label", "com.anxi-panel.smapi-update-staging=true", "--label", "com.anxi-panel.compose-project="+project, name)
	return err
}

func (c *Client) RuntimeCloneGameData(ctx context.Context, dir, source, target, trustedImage string) error {
	if !dockerVolumePattern.MatchString(source) || !smapiStagingVolumePattern.MatchString(target) || validateRestrictedImageRef(trustedImage) != nil {
		return errors.New("invalid game-data staging clone request")
	}
	_, err := c.run(ctx, "clone game-data to SMAPI staging", dir, c.timeouts.Up,
		"run", "--rm", "--network", "none", "--entrypoint", "sh",
		"--mount", "type=volume,src="+source+",dst=/source,readonly",
		"--mount", "type=volume,src="+target+",dst=/target",
		trustedImage, "-c", runtimeVolumeCloneScript)
	return err
}

func (c *Client) RuntimeInstallSMAPIArchive(ctx context.Context, dir, volume, archivePath, trustedImage string) error {
	if !smapiStagingVolumePattern.MatchString(volume) || validateRestrictedImageRef(trustedImage) != nil || !filepath.IsAbs(archivePath) || !smapiInstallerFilePattern.MatchString(filepath.Base(archivePath)) {
		return errors.New("invalid SMAPI staging install request")
	}
	root, err := filepath.Abs(filepath.Join(dir, ".local-container", "smapi-update", "packages"))
	if err != nil {
		return err
	}
	archive, err := filepath.Abs(archivePath)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, archive)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return errors.New("SMAPI archive is outside the private package directory")
	}
	const script = `set -eu
tmp=/tmp/anxi-smapi-update
rm -rf "$tmp"; mkdir -p "$tmp"
unzip -q /package/smapi.zip -d "$tmp"
installer="$(find "$tmp" -path '*/internal/linux/SMAPI.Installer' -type f | head -n 1)"
[ -n "$installer" ] && [ -f "$installer" ] || { echo 'SMAPI Linux installer missing' >&2; exit 4; }
chmod +x "$installer"
printf '2\n\n' | "$installer" --install --game-path /game
[ -x /game/StardewModdingAPI ] && [ -f /game/StardewModdingAPI.dll ] || { echo 'SMAPI install incomplete' >&2; exit 5; }`
	_, err = c.run(ctx, "install SMAPI on staging volume", dir, c.timeouts.Up,
		"run", "--rm", "--pull", "never", "--network", "none", "--read-only", "--tmpfs", "/tmp:exec,size=768m", "--entrypoint", "sh",
		"--mount", "type=volume,src="+volume+",dst=/game",
		"--mount", "type=bind,src="+archive+",dst=/package/smapi.zip,readonly",
		trustedImage, "-c", script)
	return err
}

func (c *Client) RuntimeRemoveSMAPIStagingVolume(ctx context.Context, dir, project, name string) error {
	if !composeProjectPattern.MatchString(project) || !smapiStagingVolumePattern.MatchString(name) || !strings.HasPrefix(name, project+"_anxi-smapi-update-") {
		return errors.New("invalid SMAPI staging volume removal")
	}
	_, err := c.run(ctx, "remove unused SMAPI staging volume", dir, c.timeouts.Version, "volume", "rm", name)
	return err
}

// RuntimeReadSMAPIMetadata reads only fixed, reliable SMAPI installation
// artifacts from the selected game-data volume. It doesn't trust .env and it
// doesn't execute the game or SMAPI.
func (c *Client) RuntimeReadSMAPIMetadata(ctx context.Context, dir, volumeName, imageRef string) (RuntimeSMAPIMetadata, error) {
	if !dockerVolumePattern.MatchString(volumeName) {
		return RuntimeSMAPIMetadata{}, errors.New("invalid game data volume")
	}
	if err := validateRestrictedImageRef(imageRef); err != nil {
		return RuntimeSMAPIMetadata{}, err
	}
	if _, err := c.RuntimeVolumeInspect(ctx, dir, volumeName); err != nil {
		return RuntimeSMAPIMetadata{}, err
	}
	if _, err := c.RuntimeImageInspect(ctx, dir, imageRef); err != nil {
		return RuntimeSMAPIMetadata{}, errors.New("local runtime image is unavailable")
	}
	const script = `set -eu
if [ ! -f /game/StardewModdingAPI.dll ]; then printf 'PRESENT=0\nFILES=0\n'; exit 0; fi
printf 'PRESENT=1\n'
files=1
for path in /game/StardewModdingAPI /game/StardewModdingAPI.dll /game/StardewModdingAPI.deps.json /game/StardewModdingAPI.runtimeconfig.json /game/smapi-internal/config.json; do [ -e "$path" ] || files=0; done
printf 'FILES=%s\n' "$files"
version="$(grep -aoE '[0-9]+\.[0-9]+\.[0-9]+(\.[0-9]+)?\+[0-9a-f]{7,64}' /game/StardewModdingAPI.dll 2>/dev/null | sort -u | head -n 1 || true)"
printf 'VERSION=%s\n' "$version"`
	result, err := c.run(ctx, "read SMAPI assembly metadata", dir, c.timeouts.Ps,
		"run", "--rm", "--pull", "never", "--network", "none", "--read-only", "--tmpfs", "/tmp",
		"--mount", "type=volume,src="+volumeName+",dst=/game,readonly",
		"--entrypoint", "sh", imageRef, "-c", script)
	if err != nil {
		return RuntimeSMAPIMetadata{}, err
	}
	values := map[string]string{}
	for _, line := range strings.Split(strings.ReplaceAll(result.Stdout, "\r\n", "\n"), "\n") {
		if key, value, ok := strings.Cut(line, "="); ok {
			values[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	return RuntimeSMAPIMetadata{
		Present: values["PRESENT"] == "1", RequiredFiles: values["FILES"] == "1",
		Version: values["VERSION"], VersionEvidence: "StardewModdingAPI.dll AssemblyInformationalVersion",
	}, nil
}
