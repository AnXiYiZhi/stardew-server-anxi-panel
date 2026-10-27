package docker

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

func validateJunimo140StagingIdentity(project, name, applyID string) error {
	if !composeProjectPattern.MatchString(project) || !smapiStagingVolumePattern.MatchString(name) || name != project+"_anxi-smapi-update-"+strings.TrimPrefix(applyID, "apply_") || !strings.HasPrefix(applyID, "apply_") || len(applyID) != 30 {
		return errors.New("invalid shared runtime staging identity")
	}
	return nil
}

func (c *Client) RuntimeCreateJunimo140Staging(ctx context.Context, dir, project, name, applyID string) error {
	if err := validateJunimo140StagingIdentity(project, name, applyID); err != nil {
		return err
	}
	_, raw, err := c.runWithEnvironmentRaw(ctx, "check new shared runtime staging identity", dir, c.timeouts.Version, nil, "volume", "ls", "--quiet", "--filter", "name="+name)
	if err != nil {
		return err
	}
	for _, candidate := range strings.Fields(raw) {
		if candidate == name {
			return errors.New("shared runtime staging identity already exists")
		}
	}
	_, err = c.run(ctx, "create private shared runtime staging", dir, c.timeouts.Version, "volume", "create",
		"--label", "com.anxi-panel.smapi-update-staging=true", "--label", "com.anxi-panel.compose-project="+project,
		"--label", "com.anxi-panel.runtime-update-apply-id="+applyID, name)
	return err
}

func (c *Client) RuntimeRemoveJunimo140Staging(ctx context.Context, dir, project, name, applyID string) error {
	if err := validateJunimo140StagingIdentity(project, name, applyID); err != nil {
		return err
	}
	_, raw, err := c.runWithEnvironmentRaw(ctx, "find exact shared runtime staging", dir, c.timeouts.Version, nil, "volume", "ls", "--quiet", "--filter", "name="+name)
	if err != nil {
		return err
	}
	found := false
	for _, candidate := range strings.Fields(raw) {
		found = found || candidate == name
	}
	if !found {
		return nil
	}
	_, metadata, err := c.runWithEnvironmentRaw(ctx, "inspect exact shared runtime staging owner", dir, c.timeouts.Version, nil, "volume", "inspect", name)
	if err != nil {
		return err
	}
	var records []struct {
		Name   string
		Labels map[string]string
	}
	if json.Unmarshal([]byte(metadata), &records) != nil || len(records) != 1 || records[0].Name != name {
		return errors.New("shared runtime staging metadata unavailable")
	}
	labels := records[0].Labels
	if labels["com.anxi-panel.smapi-update-staging"] != "true" || labels["com.anxi-panel.compose-project"] != project || labels["com.anxi-panel.runtime-update-apply-id"] != applyID {
		return errors.New("shared runtime staging ownership changed")
	}
	if err := c.unusedSMAPIStaging(ctx, dir, name); err != nil {
		return err
	}
	_, err = c.run(ctx, "remove exact unused shared runtime staging", dir, c.timeouts.Version, "volume", "rm", name)
	return err
}
