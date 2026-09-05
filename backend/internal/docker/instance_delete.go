package docker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// DeletionPlan is persisted before the first removal. Never infer missing
// resources from a name during retry: names must retain their inspected identity.
type DeletionPlan struct {
	Project      string
	HostDir      string
	ContainerDir string
	Containers   []string
	Volumes      map[string]string
	Networks     map[string]string
}
type deletionContainer struct {
	ID     string
	Config struct{ Labels map[string]string }
	State  struct {
		Status                      string
		Running, Restarting, Paused bool
	}
	Mounts []struct{ Type, Name, Source, Destination string }
}
type deletionVolume struct {
	Name, CreatedAt, Driver string
	Labels, Options         map[string]string
}
type deletionNetwork struct {
	ID         string `json:"Id"`
	Name       string
	Labels     map[string]string
	Containers map[string]json.RawMessage
}

var anonymousDeletionVolume = regexp.MustCompile(`^[a-f0-9]{64}$`)

func deletionPath(s string) string {
	s = strings.ReplaceAll(s, `\`, "/")
	// Desktop's Engine uses /run/desktop/mnt/host/c/ for a drive bind,
	// while Compose labels retain a drive path. Map only this verified prefix.
	const desktop = "/run/desktop/mnt/host/"
	if strings.HasPrefix(s, desktop) {
		rest := strings.TrimPrefix(s, desktop)
		if len(rest) >= 2 && rest[1] == '/' && rest[0] >= 'a' && rest[0] <= 'z' {
			s = rest[:1] + ":" + rest[1:]
		}
	}
	s = path.Clean(s)
	if len(s) >= 3 && s[1] == ':' && s[2] == '/' {
		s = strings.ToLower(s)
	}
	return strings.TrimRight(s, "/")
}
func deletionWithin(root, child string) bool {
	root, child = deletionPath(root), deletionPath(child)
	return child == root || strings.HasPrefix(child, root+"/")
}
func deletionFingerprint(v deletionVolume) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func (c *Client) deletionJSON(ctx context.Context, dir string, out any, args ...string) error {
	r, raw, err := c.runWithEnvironmentRaw(ctx, "inspect world deletion resources", dir, c.timeouts.Ps, nil, args...)
	if err != nil {
		return err
	}
	if r.StdoutTruncated {
		return errors.New("resource inspection truncated")
	}
	return json.Unmarshal([]byte(raw), out)
}
func (c *Client) deletionList(ctx context.Context, dir string, args ...string) ([]string, error) {
	r, err := c.run(ctx, "list world deletion resources", dir, c.timeouts.Ps, args...)
	if err != nil {
		return nil, err
	}
	if r.StdoutTruncated {
		return nil, errors.New("resource list truncated")
	}
	return strings.Fields(r.Stdout), nil
}
func (c *Client) deletionContainers(ctx context.Context, dir string) ([]deletionContainer, error) {
	ids, err := c.deletionList(ctx, dir, "ps", "-aq", "--no-trunc")
	if err != nil {
		return nil, err
	}
	all := []deletionContainer{}
	for _, id := range ids {
		var rows []deletionContainer
		if err := c.deletionJSON(ctx, dir, &rows, "inspect", id); err != nil {
			return nil, err
		}
		if len(rows) != 1 {
			return nil, errors.New("ambiguous container")
		}
		all = append(all, rows[0])
	}
	return all, nil
}
func deletionContainerOwned(v deletionContainer, project, hostDir, containerDir string) bool {
	return v.Config.Labels["com.docker.compose.project"] == project &&
		(deletionPath(v.Config.Labels["com.docker.compose.project.working_dir"]) == deletionPath(hostDir) ||
			containerDir != "" && deletionPath(v.Config.Labels["com.docker.compose.project.working_dir"]) == deletionPath(containerDir)) &&
		(v.Config.Labels["com.docker.compose.service"] == "server" || v.Config.Labels["com.docker.compose.service"] == "steam-auth" || v.Config.Labels["com.docker.compose.service"] == "attach-cli")
}
func deletionStopped(v deletionContainer) bool {
	return !v.State.Running && !v.State.Restarting && !v.State.Paused && (v.State.Status == "exited" || v.State.Status == "created" || v.State.Status == "dead")
}

func (c *Client) PlanInstanceDeletion(ctx context.Context, dir, project, hostDir, gameVolume string) (DeletionPlan, error) {
	p := DeletionPlan{Project: project, HostDir: hostDir, ContainerDir: filepath.Join(dir, project), Volumes: map[string]string{}, Networks: map[string]string{}}
	if !composeProjectPattern.MatchString(project) || path.Base(deletionPath(hostDir)) != project || !dockerVolumePattern.MatchString(gameVolume) {
		return p, errors.New("unmanaged world resources")
	}
	all, err := c.deletionContainers(ctx, dir)
	if err != nil {
		return p, err
	}
	anonymous := map[string]bool{}
	for _, v := range all {
		if v.Config.Labels["com.docker.compose.project"] != project {
			continue
		}
		if !deletionContainerOwned(v, project, hostDir, p.ContainerDir) || !deletionStopped(v) {
			return p, errors.New("world container running or ownership uncertain")
		}
		p.Containers = append(p.Containers, v.ID)
		for _, m := range v.Mounts {
			if m.Type == "volume" && anonymousDeletionVolume.MatchString(m.Name) {
				anonymous[m.Name] = true
			}
			if (m.Destination == "/data/game" && (m.Type != "volume" || m.Name != gameVolume)) ||
				(m.Type == "bind" && !deletionWithin(hostDir, m.Source)) {
				return p, errors.New("world has unmanaged mounts")
			}
		}
	}
	names, err := c.deletionList(ctx, dir, "volume", "ls", "-q")
	if err != nil {
		return p, err
	}
	for _, name := range names {
		var rows []deletionVolume
		if err := c.deletionJSON(ctx, dir, &rows, "volume", "inspect", name); err != nil {
			return p, err
		}
		if len(rows) != 1 {
			return p, errors.New("ambiguous volume")
		}
		v := rows[0]
		owned := v.Labels["com.docker.compose.project"] == project || v.Labels["com.anxi-panel.compose-project"] == project
		if anonymous[name] && !owned && len(v.Labels) != 0 {
			return p, errors.New("anonymous mount has foreign ownership labels")
		}
		if !owned && !anonymous[name] {
			if name == gameVolume || name == project+"_steam-session" {
				return p, errors.New("world volume ownership unverified")
			}
			continue
		}
		if v.Driver != "local" || len(v.Options) != 0 || v.CreatedAt == "" {
			return p, errors.New("external world volume cannot be deleted")
		}
		p.Volumes[name] = deletionFingerprint(v)
	}
	ids, err := c.deletionList(ctx, dir, "network", "ls", "-q", "--no-trunc", "--filter", "label=com.docker.compose.project="+project)
	if err != nil {
		return p, err
	}
	for _, id := range ids {
		var rows []deletionNetwork
		if err := c.deletionJSON(ctx, dir, &rows, "network", "inspect", id); err != nil {
			return p, err
		}
		if len(rows) != 1 || rows[0].Labels["com.docker.compose.project"] != project {
			return p, errors.New("network owner unknown")
		}
		p.Networks[rows[0].Name] = rows[0].ID
	}
	return p, c.validateDeletionReferences(p, all)
}

func (c *Client) validateDeletionReferences(p DeletionPlan, all []deletionContainer) error {
	owned := map[string]bool{}
	for _, id := range p.Containers {
		owned[id] = true
	}
	for _, v := range all {
		if owned[v.ID] {
			if !deletionContainerOwned(v, p.Project, p.HostDir, p.ContainerDir) || !deletionStopped(v) {
				return errors.New("world container changed or is running")
			}
			for _, m := range v.Mounts {
				if m.Type == "volume" {
					if _, ok := p.Volumes[m.Name]; !ok {
						return errors.New("world has an unowned volume mount")
					}
				}
			}
			continue
		}
		if v.Config.Labels["com.docker.compose.project"] == p.Project {
			return errors.New("new world container appeared")
		}
		for _, m := range v.Mounts {
			if _, ok := p.Volumes[m.Name]; m.Type == "volume" && ok {
				return errors.New("world volume is shared with another container")
			}
			// Parent binds (the Panel data root) are expected; only another holder
			// rooted in the target world makes that world's files shared.
			if m.Type == "bind" && deletionWithin(p.HostDir, m.Source) {
				return errors.New("world directory is shared with another container")
			}
		}
	}
	return nil
}

func (c *Client) ApplyInstanceDeletion(ctx context.Context, dir string, p DeletionPlan) error {
	all, err := c.deletionContainers(ctx, dir)
	if err != nil {
		return err
	}
	if err = c.validateDeletionReferences(p, all); err != nil {
		return err
	}
	existing := map[string]bool{}
	for _, v := range all {
		existing[v.ID] = true
	}
	// Preflight every remaining identity before resuming any destructive step.
	names, err := c.deletionList(ctx, dir, "volume", "ls", "-q")
	if err != nil {
		return err
	}
	volumes := map[string]bool{}
	for _, name := range names {
		expected, ok := p.Volumes[name]
		if !ok {
			continue
		}
		var rows []deletionVolume
		if err = c.deletionJSON(ctx, dir, &rows, "volume", "inspect", name); err != nil {
			return err
		}
		if len(rows) != 1 || deletionFingerprint(rows[0]) != expected {
			return errors.New("world volume identity changed")
		}
		volumes[name] = true
	}
	ids, err := c.deletionList(ctx, dir, "network", "ls", "-q", "--no-trunc")
	if err != nil {
		return err
	}
	networks := map[string]bool{}
	for _, id := range ids {
		networks[id] = true
	}
	owned := map[string]bool{}
	for _, id := range p.Containers {
		owned[id] = true
	}
	for _, id := range p.Networks {
		if !networks[id] {
			continue
		}
		var rows []deletionNetwork
		if err = c.deletionJSON(ctx, dir, &rows, "network", "inspect", id); err != nil {
			return err
		}
		if len(rows) != 1 || rows[0].Labels["com.docker.compose.project"] != p.Project {
			return errors.New("network ownership changed")
		}
		for holder := range rows[0].Containers {
			if !owned[holder] {
				return errors.New("world network has another holder")
			}
		}
	}
	for _, id := range p.Containers {
		if existing[id] {
			if _, err = c.run(ctx, "remove stopped world container", dir, c.timeouts.Down, "rm", id); err != nil {
				return err
			}
		}
	}
	for name := range p.Volumes {
		if volumes[name] {
			if _, err = c.run(ctx, "remove owned world volume", dir, c.timeouts.Down, "volume", "rm", name); err != nil {
				return err
			}
		}
	}
	for _, id := range p.Networks {
		if networks[id] {
			if _, err = c.run(ctx, "remove owned world network", dir, c.timeouts.Down, "network", "rm", id); err != nil {
				return err
			}
		}
	}
	all, err = c.deletionContainers(ctx, dir)
	if err != nil {
		return err
	}
	if err = c.validateDeletionReferences(p, all); err != nil {
		return err
	}
	for _, v := range all {
		if owned[v.ID] {
			return fmt.Errorf("world container removal incomplete")
		}
	}
	// A resource newly created by an external operator is not part of this
	// immutable plan. Keep the journal instead of silently leaving it behind.
	for _, filter := range []string{"label=com.docker.compose.project=" + p.Project, "label=com.anxi-panel.compose-project=" + p.Project} {
		remaining, err := c.deletionList(ctx, dir, "volume", "ls", "-q", "--filter", filter)
		if err != nil {
			return err
		}
		if len(remaining) != 0 {
			return errors.New("world volumes remain outside the completed plan")
		}
	}
	remaining, err := c.deletionList(ctx, dir, "network", "ls", "-q", "--filter", "label=com.docker.compose.project="+p.Project)
	if err != nil {
		return err
	}
	if len(remaining) != 0 {
		return errors.New("world networks remain outside the completed plan")
	}
	return nil
}
