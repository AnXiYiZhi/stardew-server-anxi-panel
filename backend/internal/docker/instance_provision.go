package docker

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ProvisionInstanceGameData creates one exact instance-owned game-data volume
// and copies an installed source volume into it without network access. The
// source is mounted read-only and an existing target is never reused.
func (c *Client) ProvisionInstanceGameData(ctx context.Context, dir, project, source, target, trustedImage, token string) error {
	if !composeProjectPattern.MatchString(project) || !dockerVolumePattern.MatchString(source) ||
		!dockerVolumePattern.MatchString(target) || target != project+"_game-data" || source == target ||
		validateRestrictedImageRef(trustedImage) != nil || !provisionTokenPattern.MatchString(token) {
		return errors.New("invalid instance game-data provision request")
	}

	inspect, inspectErr := c.run(ctx, "inspect instance game-data target", dir, c.timeouts.Version, "volume", "inspect", target)
	if inspectErr == nil {
		return errors.New("instance game-data target already exists")
	}
	inspectDetail := strings.ToLower(inspect.Stdout + "\n" + inspect.Stderr + "\n" + inspectErr.Error())
	if !strings.Contains(inspectDetail, "no such volume") {
		return fmt.Errorf("inspect instance game-data target: %w", inspectErr)
	}

	if _, err := c.run(ctx, "create instance game-data volume", dir, c.timeouts.Version,
		"volume", "create",
		"--label", "com.anxi-panel.instance-provision=true",
		"--label", "com.anxi-panel.compose-project="+project,
		"--label", "com.anxi-panel.provision-token="+token,
		target); err != nil {
		return err
	}
	// volume create is idempotent: an external creator may win after inspect.
	// Verify the unique allocation token before mounting or writing anything.
	var created []deletionVolume
	if err := c.deletionJSON(ctx, dir, &created, "volume", "inspect", target); err != nil {
		return err
	}
	if len(created) != 1 || created[0].Labels["com.anxi-panel.provision-token"] != token {
		return errors.New("target allocation ownership changed")
	}

	_, cloneErr := c.run(ctx, "clone installed game runtime for instance", dir, c.timeouts.Up,
		"run", "--name", "anxi-provision-"+token, "--rm", "--pull", "never", "--network", "none", "--entrypoint", "sh",
		"--label", "com.anxi-panel.instance-provision=true",
		"--label", "com.anxi-panel.compose-project="+project,
		"--label", "com.anxi-panel.provision-token="+token,
		"--mount", "type=volume,src="+source+",dst=/source,readonly",
		"--mount", "type=volume,src="+target+",dst=/target",
		trustedImage, "-c", runtimeVolumeCloneScript)
	return cloneErr
}
