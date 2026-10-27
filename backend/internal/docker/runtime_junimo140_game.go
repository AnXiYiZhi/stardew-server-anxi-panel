package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Evidence reads are closed to the three files used by the fixed legacy payload
// verifier. Callers cannot turn this into a general game-volume file reader.
func (c *Client) RuntimeReadJunimo140GameEvidence(ctx context.Context, dir, volume, imageRef, imageID, kind string) ([]byte, error) {
	files := map[string]string{"game": "steamapps/appmanifest_413150.acf", "sdk": ".steam-sdk/steamapps/appmanifest_1007.acf", "content": "Content/ContentHashes.json"}
	file, ok := files[kind]
	if !ok {
		return nil, errors.New("invalid fixed game evidence kind")
	}
	if err := c.unusedSMAPIStaging(ctx, dir, volume); err != nil {
		return nil, err
	}
	if err := c.pinnedSMAPIImage(ctx, dir, imageRef, imageID); err != nil {
		return nil, err
	}
	const script = `set -eu
cd /game
file=$1
part=$file
while test "$part" != .; do test ! -L "$part"; part=$(dirname "$part"); done
test -f "$file"
test "$(stat -c %s "$file")" -le 1048576
cat "$file"`
	args := append([]string{"run", "--rm"}, imageSMAPIHelperArgs(volume, "evidence-"+kind)...)
	args = append(args, "--pull", "never", "--network", "none", "--read-only", "--mount", "type=volume,src="+volume+",dst=/game,readonly", "--entrypoint", "timeout", imageID, "--kill-after=2s", "30s", "sh", "-c", script, "--", file)
	result, raw, err := c.runWithEnvironmentRaw(ctx, "read fixed legacy game evidence", dir, time.Minute, nil, args...)
	if err != nil {
		return nil, err
	}
	if result.StdoutTruncated || len(raw) == 0 || len(raw) > 1024*1024 {
		return nil, errors.New("fixed game evidence is empty or too large")
	}
	return []byte(raw), nil
}

type RuntimeGameFileStats struct{ GameFiles, GameBytes, SDKFiles, SDKBytes int64 }

var fixedFileHashPattern = regexp.MustCompile(`^[0-9a-fA-F]+$`)

func fixedGameChecksumInput(files map[string]string, algorithm string) (string, error) {
	width := 64
	if algorithm == "md5" {
		width = 32
	} else if algorithm != "sha256" {
		return "", errors.New("invalid game checksum algorithm")
	}
	if len(files) == 0 || len(files) > 10000 {
		return "", errors.New("invalid game checksum count")
	}
	keys := make([]string, 0, len(files))
	for name, hash := range files {
		if name == "." || name == "" || path.Clean(name) != name || strings.HasPrefix(name, "/") || strings.HasPrefix(name, "../") || strings.ContainsAny(name, "\\\r\n\x00") || strings.ContainsAny(name, "\t") || len(name) > 512 || len(hash) != width || !fixedFileHashPattern.MatchString(hash) {
			return "", errors.New("invalid fixed game checksum entry")
		}
		for _, ch := range name {
			if ch < 32 || ch == 127 {
				return "", errors.New("invalid fixed game checksum path")
			}
		}
		keys = append(keys, name)
	}
	sort.Strings(keys)
	var input strings.Builder
	for _, name := range keys {
		fmt.Fprintf(&input, "%s  %s\n", strings.ToLower(files[name]), name)
	}
	return input.String(), nil
}

// Verify every expected file on an unused staging volume, including rejecting
// symlinked parents. Hash lists arrive on stdin and are never interpolated into
// shell source. The source volume and running game are never modified.
func (c *Client) RuntimeVerifyJunimo140GameFiles(ctx context.Context, dir, volume, imageRef, imageID, algorithm string, files map[string]string) (RuntimeGameFileStats, error) {
	input, err := fixedGameChecksumInput(files, algorithm)
	if err != nil {
		return RuntimeGameFileStats{}, err
	}
	if err := c.unusedSMAPIStaging(ctx, dir, volume); err != nil {
		return RuntimeGameFileStats{}, err
	}
	if err := c.pinnedSMAPIImage(ctx, dir, imageRef, imageID); err != nil {
		return RuntimeGameFileStats{}, err
	}
	const script = `set -euo pipefail
cd /game
test ! -L /game
cat > /tmp/expected
width=$1
algorithm=$2
while IFS= read -r line; do
    file=${line:width}
    part=$file
    while test "$part" != .; do test ! -L "$part"; part=$(dirname "$part"); done
    test -f "$file"
done < /tmp/expected
"${algorithm}sum" --check --strict --quiet /tmp/expected
while IFS= read -r line; do
    file=${line:width}
    case "$file" in .steam-sdk/*) kind=sdk ;; *) kind=game ;; esac
    printf '%s ' "$kind"
    stat --printf='%s\n' -- "$file"
done < /tmp/expected | awk '{n[$1]++; b[$1]+=$2} END {printf "%.0f %.0f %.0f %.0f\n", n["game"],b["game"],n["sdk"],b["sdk"]}'`
	width := "66"
	if algorithm == "md5" {
		width = "34"
	}
	args := append([]string{"run", "--rm", "-i"}, imageSMAPIHelperArgs(volume, "check-"+algorithm)...)
	args = append(args, "--pull", "never", "--network", "none", "--read-only", "--tmpfs", "/tmp:size=16m", "--mount", "type=volume,src="+volume+",dst=/game,readonly", "--entrypoint", "timeout", imageID, "--kill-after=2s", "600s", "bash", "-euo", "pipefail", "-c", script, "--", width, algorithm)
	result, err := c.runWithInput(ctx, "verify fixed legacy game files", dir, 11*time.Minute, strings.NewReader(input), args...)
	if err != nil {
		return RuntimeGameFileStats{}, err
	}
	parts := strings.Fields(result.Stdout)
	if result.StdoutTruncated || len(parts) != 4 {
		return RuntimeGameFileStats{}, errors.New("invalid fixed game verification statistics")
	}
	var values [4]int64
	for i, part := range parts {
		values[i], err = strconv.ParseInt(part, 10, 64)
		if err != nil || values[i] < 0 {
			return RuntimeGameFileStats{}, errors.New("invalid fixed game verification statistics")
		}
	}
	if values[0]+values[2] != int64(len(files)) {
		return RuntimeGameFileStats{}, errors.New("incomplete fixed game verification")
	}
	return RuntimeGameFileStats{values[0], values[1], values[2], values[3]}, nil
}

type RuntimeGameDownloadMarker struct {
	AppID        uint32 `json:"appId"`
	DepotID      uint32 `json:"depotId"`
	ManifestID   uint64 `json:"manifestId"`
	TargetOS     string `json:"targetOs"`
	TotalBytes   int64  `json:"totalBytes"`
	TotalFiles   int64  `json:"totalFiles"`
	DownloadedAt string `json:"downloadedAt"`
}

func (c *Client) RuntimeCommitJunimo140GameMarkers(ctx context.Context, dir, volume, imageRef, imageID string, game, sdk RuntimeGameDownloadMarker) error {
	for i, marker := range []RuntimeGameDownloadMarker{game, sdk} {
		app, depot := uint32(413150), uint32(413153)
		if i == 1 {
			app, depot = 1007, 1006
		}
		stamp, err := time.Parse(time.RFC3339Nano, marker.DownloadedAt)
		if marker.AppID != app || marker.DepotID != depot || marker.ManifestID == 0 || marker.TargetOS != "linux" || marker.TotalBytes <= 0 || marker.TotalFiles <= 0 || err != nil || stamp.Location() != time.UTC {
			return errors.New("invalid fixed game download marker")
		}
	}
	if err := c.unusedSMAPIStaging(ctx, dir, volume); err != nil {
		return err
	}
	if err := c.pinnedSMAPIImage(ctx, dir, imageRef, imageID); err != nil {
		return err
	}
	a, _ := json.Marshal(game)
	b, _ := json.Marshal(sdk)
	const script = `set -eu
cd /game
test -d .steam-sdk
test ! -L .steam-sdk
IFS= read -r game
IFS= read -r sdk
for dir in . .steam-sdk; do test ! -L "$dir"; done
first=$(mktemp .download-manifest-413150.XXXXXX)
second=$(mktemp .steam-sdk/.download-manifest-1007.XXXXXX)
trap 'rm -f "$first" "$second"' EXIT
printf '%s\n' "$game" > "$first"
printf '%s\n' "$sdk" > "$second"
chmod 0644 "$first" "$second"
sync -f "$first"; sync -f "$second"
mv -fT "$first" .download-manifest-413150
mv -fT "$second" .steam-sdk/.download-manifest-1007
sync -f .; sync -f .steam-sdk
test "$(cat .download-manifest-413150)" = "$game"
test "$(cat .steam-sdk/.download-manifest-1007)" = "$sdk"`
	args := append([]string{"run", "--rm", "-i"}, imageSMAPIHelperArgs(volume, "game-markers")...)
	args = append(args, "--pull", "never", "--network", "none", "--read-only", "--tmpfs", "/tmp", "--mount", "type=volume,src="+volume+",dst=/game", "--entrypoint", "timeout", imageID, "--kill-after=2s", "30s", "sh", "-c", script)
	_, err := c.runWithInput(ctx, "commit verified fixed game download markers", dir, time.Minute, strings.NewReader(string(a)+"\n"+string(b)+"\n"), args...)
	return err
}
