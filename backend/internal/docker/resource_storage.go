package docker

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ResourceVolumeSizes scans only driver-owned local volumes. Listing names is
// metadata-only; system df would also walk unrelated volumes and container layers.
// Daemon-reported mountpoints are bind-mounted read-only so a concurrent volume
// deletion cannot silently recreate an empty named volume.
func (c *Client) ResourceVolumeSizes(ctx context.Context, dir, image string, names []string) (map[string]int64, error) {
	result := map[string]int64{}
	for _, name := range names {
		if !dockerVolumePattern.MatchString(name) {
			return nil, errors.New("invalid resource volume")
		}
		result[name] = 0
	}
	if len(result) == 0 {
		return result, nil
	}
	listed, err := c.run(ctx, "list resource volume identities", dir, c.timeouts.Ps, "volume", "ls", "--format", "{{.Name}}")
	if err != nil {
		return nil, err
	}
	if listed.StdoutTruncated {
		return nil, errors.New("volume identities truncated")
	}
	var existing []string
	for _, name := range strings.Fields(listed.Stdout) {
		if _, wanted := result[name]; wanted {
			existing = append(existing, name)
			result[name] = -1
		}
	}
	if len(existing) == 0 {
		return result, nil
	}
	if err := validateRestrictedImageRef(image); err != nil {
		return nil, err
	}
	sort.Strings(existing)
	// Keep argument size, mount count and cleanup bounded on large installations.
	for start := 0; start < len(existing); start += 32 {
		batch := existing[start:min(start+32, len(existing))]
		inspected, err := c.run(ctx, "inspect resource volumes", dir, c.timeouts.Ps, append([]string{"volume", "inspect"}, batch...)...)
		if err != nil {
			return nil, err
		}
		if inspected.StdoutTruncated {
			return nil, errors.New("volume metadata truncated")
		}
		targets, err := resourceVolumeTargets(inspected.Stdout, batch)
		if err != nil {
			return nil, err
		}
		if len(targets) == 0 {
			continue
		}
		sizes, err := c.scanResourceVolumes(ctx, dir, image, targets)
		if err != nil {
			return nil, err
		}
		for name, size := range sizes {
			result[name] = size
		}
	}
	return result, nil
}

type resourceVolumeTarget struct{ Name, Mountpoint string }

func resourceVolumeTargets(raw string, names []string) ([]resourceVolumeTarget, error) {
	var volumes []struct {
		Name, Mountpoint, Driver string
		Options                  map[string]string
	}
	if err := json.Unmarshal([]byte(raw), &volumes); err != nil {
		return nil, err
	}
	wanted := map[string]bool{}
	for _, name := range names {
		wanted[name] = true
	}
	var targets []resourceVolumeTarget
	for _, volume := range volumes {
		if !wanted[volume.Name] {
			return nil, errors.New("unexpected resource volume identity")
		}
		delete(wanted, volume.Name)
		// Remote/plugin and local bind volumes may be unmounted or overlap an
		// external tree. Report unknown instead of probing outside owned data.
		if volume.Driver != "local" || len(volume.Options) != 0 {
			continue
		}
		if !path.IsAbs(volume.Mountpoint) || path.Clean(volume.Mountpoint) == "/" || strings.ContainsAny(volume.Mountpoint, ",\r\n") {
			return nil, errors.New("invalid resource volume mountpoint")
		}
		targets = append(targets, resourceVolumeTarget{volume.Name, volume.Mountpoint})
	}
	if len(wanted) != 0 {
		return nil, errors.New("incomplete resource volume metadata")
	}
	return targets, nil
}

func (c *Client) scanResourceVolumes(ctx context.Context, dir, image string, targets []resourceVolumeTarget) (map[string]int64, error) {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	name := fmt.Sprintf("anxi-resource-scan-%x", nonce)
	args := []string{"run", "--rm", "--name", name, "--label", "io.anxi-panel.resource-scan=true", "--pull", "never", "--network", "none", "--read-only", "--cap-drop", "ALL", "--cap-add", "DAC_OVERRIDE", "--security-opt", "no-new-privileges", "--pids-limit", "32", "--memory", "128m", "--cpus", "0.5", "--tmpfs", "/tmp:rw,noexec,nosuid,size=16m", "--user", "0:0"}
	for i, target := range targets {
		args = append(args, "--mount", fmt.Sprintf("type=bind,src=%s,dst=/owned/%d,readonly", target.Mountpoint, i))
	}
	// stat does not follow symlinks. A failed traversal or full tmpfs fails the
	// sample; it must never publish a partial sum as a complete volume size.
	script := `set -eu; for root in /owned/*; do find "$root" -xdev -type f -exec stat -c %s {} + > /tmp/sizes; printf '%s ' "${root##*/}"; awk '{sum += $1} END {printf "%.0f\n", sum}' /tmp/sizes; done`
	args = append(args, "--entrypoint", "sh", image, "-c", script)
	defer func() {
		// Killing the CLI does not stop its daemon-side container. Always reclaim
		// this exact random name, with an independent cleanup deadline.
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = c.run(cleanup, "remove resource scanner", dir, 5*time.Second, "rm", "-f", "-v", name)
	}()
	run, err := c.run(ctx, "scan owned resource volumes", dir, c.timeouts.Stats, args...)
	if err != nil {
		return nil, err
	}
	if run.StdoutTruncated {
		return nil, errors.New("resource sizes truncated")
	}
	result := map[string]int64{}
	for _, line := range strings.Split(strings.TrimSpace(run.Stdout), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, errors.New("invalid resource size")
		}
		index, err := strconv.Atoi(fields[0])
		if err != nil || index < 0 || index >= len(targets) {
			return nil, errors.New("invalid resource size index")
		}
		size, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil || size < 0 {
			return nil, errors.New("invalid resource size value")
		}
		if _, exists := result[targets[index].Name]; exists {
			return nil, errors.New("duplicate resource size")
		}
		result[targets[index].Name] = size
	}
	if len(result) != len(targets) {
		return nil, errors.New("incomplete resource sizes")
	}
	return result, nil
}
