package docker

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type RuntimeJunimo140HelperScope struct {
	ApplyID  string
	Projects []string
	ImageRef string
	ImageID  string
}

func junimo140HelperIDs(raw string) ([]string, error) {
	ids := strings.Fields(raw)
	if len(ids) > 2048 {
		return nil, errors.New("shared runtime helper count exceeds supported bound")
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if !dockerContainerIDPattern.MatchString(id) || seen[id] {
			return nil, errors.New("invalid shared runtime helper identity")
		}
		seen[id] = true
	}
	return ids, nil
}

func validateJunimo140Helpers(raw string, ids []string, scope RuntimeJunimo140HelperScope) error {
	var records []struct {
		ID, Name, Image string
		Config          struct {
			Image  string
			Labels map[string]string
		}
		HostConfig struct{ AutoRemove bool }
	}
	if json.Unmarshal([]byte(raw), &records) != nil || len(records) != len(ids) {
		return errors.New("shared runtime helper metadata unavailable")
	}
	projects, expected := map[string]bool{}, map[string]bool{}
	for _, project := range scope.Projects {
		projects[project] = true
	}
	for _, id := range ids {
		expected[id] = true
	}
	operations := map[string]bool{"clone": true, "install": true, "stamp": true, "evidence-game": true, "evidence-sdk": true, "evidence-content": true, "check-sha256": true, "check-md5": true, "game-markers": true, "game-permissions": true, "mod-extract": true, "archive": true, "restore": true, "permissions": true, "session-archive": true, "session-restore": true}
	prefix := "/anxi-j140-" + strings.TrimPrefix(scope.ApplyID, "apply_") + "-"
	for _, record := range records {
		labels := record.Config.Labels
		op := strings.TrimPrefix(labels["com.anxi-panel.junimo140-operation"], "smapi-")
		if !expected[record.ID] || !record.HostConfig.AutoRemove || !strings.HasPrefix(record.Name, prefix) || record.Image != scope.ImageID || record.Config.Image != scope.ImageID || labels["com.anxi-panel.junimo140-helper"] != "true" || labels["com.anxi-panel.runtime-update-apply-id"] != scope.ApplyID || !projects[labels["com.anxi-panel.compose-project"]] || !operations[op] {
			return errors.New("shared runtime helper ownership mismatch; no helpers stopped")
		}
		delete(expected, record.ID)
	}
	if len(expected) != 0 {
		return errors.New("shared runtime helper identities changed")
	}
	return nil
}

// A cancelled docker CLI does not prove its daemon-side helper stopped. Before
// rollback touches data, snapshot and validate every matching helper, stop only
// those exact IDs, then wait for --rm removal. Any unknown identity fails closed.
func (c *Client) RuntimeStopJunimo140Helpers(ctx context.Context, dir string, scope RuntimeJunimo140HelperScope) error {
	id := strings.TrimPrefix(scope.ApplyID, "apply_")
	if !strings.HasPrefix(scope.ApplyID, "apply_") || len(id) != 24 || !runtimeArchiveSHA256.MatchString(id+strings.Repeat("0", 40)) || len(scope.Projects) == 0 || len(scope.Projects) > 256 {
		return errors.New("invalid shared runtime helper scope")
	}
	projects := map[string]bool{}
	for _, project := range scope.Projects {
		if !composeProjectPattern.MatchString(project) || projects[project] {
			return errors.New("ambiguous shared runtime helper scope")
		}
		projects[project] = true
	}
	if err := c.pinnedSMAPIImage(ctx, dir, scope.ImageRef, scope.ImageID); err != nil {
		return err
	}
	list := func() ([]string, error) {
		_, raw, err := c.runWithEnvironmentRaw(ctx, "list shared runtime transaction helpers", dir, c.timeouts.Ps, nil, "container", "ls", "--all", "--quiet", "--no-trunc", "--filter", "label=com.anxi-panel.junimo140-helper=true", "--filter", "label=com.anxi-panel.runtime-update-apply-id="+scope.ApplyID)
		if err != nil {
			return nil, err
		}
		return junimo140HelperIDs(raw)
	}
	ids, err := list()
	if err != nil || len(ids) == 0 {
		return err
	}
	_, raw, err := c.runWithEnvironmentRaw(ctx, "inspect shared runtime transaction helpers", dir, c.timeouts.Ps, nil, append([]string{"container", "inspect"}, ids...)...)
	if err != nil {
		return err
	}
	if err := validateJunimo140Helpers(raw, ids, scope); err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := c.run(ctx, "stop exact shared runtime transaction helper", dir, 15*time.Second, "container", "stop", "--time", "2", id); err != nil {
			// A verified --rm helper may have completed between inspect and stop.
			remaining, listErr := list()
			if listErr != nil {
				return listErr
			}
			for _, current := range remaining {
				if current == id {
					return err
				}
			}
		}
	}
	for attempt := 0; attempt < 50; attempt++ {
		remaining, err := list()
		if err != nil {
			return err
		}
		if len(remaining) == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return errors.New("shared runtime helpers have not been removed; recovery remains blocked")
}
