package docker

import (
	"context"
	"errors"
	"regexp"
)

var provisionTokenPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

// A journal's random token identifies an allocation even if the process died
// immediately after Docker created it. A pre-existing same-name volume is kept.
func (c *Client) CleanupInstanceGameData(ctx context.Context, dir, project, token string) error {
	if !composeProjectPattern.MatchString(project) || !provisionTokenPattern.MatchString(token) {
		return errors.New("invalid provision owner")
	}
	all, err := c.deletionContainers(ctx, dir)
	if err != nil {
		return err
	}
	volume := project + "_game-data"
	helper := "anxi-provision-" + token
	for _, v := range all {
		if v.Config.Labels["com.anxi-panel.provision-token"] != token {
			continue
		}
		if v.Config.Labels["com.anxi-panel.compose-project"] != project {
			return errors.New("provision container owner changed")
		}
		// The name is checked with inspect as well as the token; only this request's
		// temporary copier can be stopped during recovery.
		var identity []struct{ Name string }
		if err = c.deletionJSON(ctx, dir, &identity, "inspect", v.ID); err != nil {
			return err
		}
		if len(identity) != 1 || identity[0].Name != "/"+helper {
			return errors.New("unknown provision token holder")
		}
		if _, err = c.run(ctx, "remove owned provision copier", dir, c.timeouts.Down, "rm", "-f", v.ID); err != nil {
			return err
		}
	}
	names, err := c.deletionList(ctx, dir, "volume", "ls", "-q")
	if err != nil {
		return err
	}
	for _, name := range names {
		if name != volume {
			continue
		}
		var rows []deletionVolume
		if err = c.deletionJSON(ctx, dir, &rows, "volume", "inspect", name); err != nil {
			return err
		}
		if len(rows) != 1 {
			return errors.New("ambiguous provision volume")
		}
		v := rows[0]
		if v.Labels["com.anxi-panel.provision-token"] != token {
			return nil
		}
		if v.Labels["com.anxi-panel.compose-project"] != project || v.Driver != "local" || len(v.Options) != 0 {
			return errors.New("provision volume ownership changed")
		}
		if _, err = c.run(ctx, "remove owned provision volume", dir, c.timeouts.Down, "volume", "rm", name); err != nil {
			return err
		}
	}
	return nil
}
