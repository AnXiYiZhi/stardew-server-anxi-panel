package docker

import (
	"context"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
)

type RuntimeContentRead struct {
	GameManifest  []byte
	SDKManifest   []byte
	FreeBytes     int64
	GameDataBytes int64
}

// RuntimeReadContentManifests mounts the already-existing game-data volume
// read-only, with networking disabled and pulling forbidden. Only the two fixed
// ACF files and df free-space figure are returned to the caller.
func (c *Client) RuntimeReadContentManifests(ctx context.Context, dir, volumeName, imageRef string) (RuntimeContentRead, error) {
	if !dockerVolumePattern.MatchString(volumeName) {
		return RuntimeContentRead{}, errors.New("invalid game data volume")
	}
	if err := validateRestrictedImageRef(imageRef); err != nil {
		return RuntimeContentRead{}, err
	}
	if _, err := c.RuntimeVolumeInspect(ctx, dir, volumeName); err != nil {
		return RuntimeContentRead{}, err
	}
	if _, err := c.RuntimeImageInspect(ctx, dir, imageRef); err != nil {
		return RuntimeContentRead{}, errors.New("local runtime image is unavailable")
	}
	const script = `emit(){ printf '%s\n' "$1"; if [ -f "$2" ]; then base64 "$2" | tr -d '\n'; fi; printf '\n'; }; emit GAME /game/steamapps/appmanifest_413150.acf; emit SDK /game/.steam-sdk/steamapps/appmanifest_1007.acf; printf 'FREEKB\n'; df -Pk /game | awk 'NR==2 {print $4}'; printf 'GAMEKB\n'; du -sk /game | awk '{print $1}'`
	result, err := c.run(ctx, "read runtime component manifests", dir, c.timeouts.Ps,
		"run", "--rm", "--pull", "never", "--network", "none",
		"--mount", "type=volume,src="+volumeName+",dst=/game,readonly",
		"--entrypoint", "sh", imageRef, "-c", script)
	if err != nil {
		return RuntimeContentRead{}, err
	}
	lines := strings.Split(strings.ReplaceAll(result.Stdout, "\r\n", "\n"), "\n")
	values := map[string]string{}
	for i := 0; i+1 < len(lines); i++ {
		if lines[i] == "GAME" || lines[i] == "SDK" || lines[i] == "FREEKB" || lines[i] == "GAMEKB" {
			values[lines[i]] = strings.TrimSpace(lines[i+1])
			i++
		}
	}
	decode := func(key string) ([]byte, error) {
		if values[key] == "" {
			return nil, nil
		}
		decoded, decodeErr := base64.StdEncoding.DecodeString(values[key])
		if decodeErr != nil || len(decoded) > 1024*1024 {
			return nil, errors.New("invalid manifest read response")
		}
		return decoded, nil
	}
	game, err := decode("GAME")
	if err != nil {
		return RuntimeContentRead{}, err
	}
	sdk, err := decode("SDK")
	if err != nil {
		return RuntimeContentRead{}, err
	}
	freeKB, _ := strconv.ParseInt(values["FREEKB"], 10, 64)
	gameKB, _ := strconv.ParseInt(values["GAMEKB"], 10, 64)
	return RuntimeContentRead{GameManifest: game, SDKManifest: sdk, FreeBytes: freeKB * 1024, GameDataBytes: gameKB * 1024}, nil
}
