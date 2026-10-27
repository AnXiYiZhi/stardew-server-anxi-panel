package docker

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
)

type RuntimeJunimo140WorldScope struct {
	Project     string
	HostDataDir string
}

type RuntimeJunimo140ServiceSnapshot struct {
	Project string
	Service string
	RuntimeServiceMetadata
	Running       bool
	GameVolume    string
	SessionVolume string
}

// Query by authoritative project labels rather than Compose files: a rollback
// must remain able to inspect/stop peers while .env is temporarily absent.
func (c *Client) RuntimeSnapshotJunimo140Services(ctx context.Context, dir string, worlds []RuntimeJunimo140WorldScope, gameVolumes []string) ([]RuntimeJunimo140ServiceSnapshot, error) {
	if len(worlds) == 0 || len(worlds) > 256 || len(gameVolumes) == 0 || len(gameVolumes) > 2 {
		return nil, errors.New("invalid shared runtime service scope")
	}
	allowed := map[string]bool{}
	for _, volume := range gameVolumes {
		if !dockerVolumePattern.MatchString(volume) || allowed[volume] {
			return nil, errors.New("invalid shared runtime game scope")
		}
		allowed[volume] = true
	}
	seen := map[string]bool{}
	var result []RuntimeJunimo140ServiceSnapshot
	for _, world := range worlds {
		root := deletionPath(world.HostDataDir)
		if !composeProjectPattern.MatchString(world.Project) || seen[world.Project] || root == "" || strings.ContainsAny(root, ",\r\n\x00") || !strings.HasPrefix(root, "/") && !(len(root) > 3 && root[1] == ':' && root[2] == '/') {
			return nil, errors.New("ambiguous shared runtime world scope")
		}
		seen[world.Project] = true
		_, raw, err := c.runWithEnvironmentRaw(ctx, "list exact shared runtime project services", dir, c.timeouts.Ps, nil, "container", "ls", "--all", "--quiet", "--no-trunc", "--filter", "label=com.docker.compose.project="+world.Project)
		if err != nil {
			return nil, err
		}
		ids, err := junimo140HelperIDs(raw)
		if err != nil {
			return nil, err
		}
		if len(ids) == 0 {
			continue
		}
		if len(ids) > 2 {
			return nil, errors.New("shared runtime project has unexpected service holders")
		}
		_, raw, err = c.runWithEnvironmentRaw(ctx, "inspect exact shared runtime project services", dir, c.timeouts.Ps, nil, append([]string{"container", "inspect"}, ids...)...)
		if err != nil {
			return nil, err
		}
		services, err := parseJunimo140ServiceSnapshots(raw, ids, world, allowed)
		if err != nil {
			return nil, err
		}
		result = append(result, services...)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Project == result[j].Project {
			return result[i].Service < result[j].Service
		}
		return result[i].Project < result[j].Project
	})
	return result, nil
}

func parseJunimo140ServiceSnapshots(raw string, ids []string, world RuntimeJunimo140WorldScope, volumes map[string]bool) ([]RuntimeJunimo140ServiceSnapshot, error) {
	var records []struct {
		ID, Image string
		Config    struct {
			Image  string
			Labels map[string]string
		}
		HostConfig struct{ Privileged bool }
		State      struct {
			Status                            string
			Running, Restarting, Paused, Dead bool
			Health                            struct{ Status string }
		}
		Mounts []struct {
			Type, Name, Source, Destination string
			RW                              bool
		}
	}
	if json.Unmarshal([]byte(raw), &records) != nil || len(records) != len(ids) {
		return nil, errors.New("shared runtime service metadata unavailable")
	}
	expected := map[string]bool{}
	for _, id := range ids {
		if !dockerContainerIDPattern.MatchString(id) || expected[id] {
			return nil, errors.New("invalid shared runtime service identities")
		}
		expected[id] = true
	}
	seen := map[string]bool{}
	var result []RuntimeJunimo140ServiceSnapshot
	for _, record := range records {
		labels := record.Config.Labels
		service := labels["com.docker.compose.service"]
		if !expected[record.ID] || labels["com.docker.compose.project"] != world.Project || (service != "server" && service != "steam-auth") || seen[service] || strings.EqualFold(labels["com.docker.compose.oneoff"], "true") || !runtimeDigestPattern.MatchString(record.Image) || record.Config.Image == "" || record.State.Paused || record.State.Dead || record.HostConfig.Privileged {
			return nil, errors.New("shared runtime service ownership or state is unsupported")
		}
		delete(expected, record.ID)
		seen[service] = true
		snapshot := RuntimeJunimo140ServiceSnapshot{Project: world.Project, Service: service, RuntimeServiceMetadata: RuntimeServiceMetadata{ContainerID: record.ID, Image: record.Config.Image, ImageID: record.Image, State: record.State.Status, Health: record.State.Health.Status}, Running: record.State.Running || record.State.Restarting}
		required := map[string]string{}
		if service == "server" {
			required = map[string]string{"/config/xdg/config/StardewValley": "saves", "/data/control": "control", "/data/settings": "settings", "/data/Mods": "mods"}
		}
		mountTargets := map[string]bool{}
		for _, mount := range record.Mounts {
			if mountTargets[mount.Destination] {
				return nil, errors.New("shared runtime service has duplicate mount targets")
			}
			mountTargets[mount.Destination] = true
			if mount.Type == "bind" {
				if !deletionWithin(world.HostDataDir, mount.Source) {
					return nil, errors.New("shared runtime service binds data outside its backed-up root")
				}
				if suffix, ok := required[mount.Destination]; ok {
					if deletionPath(mount.Source) != deletionPath(world.HostDataDir+"/.local-container/"+suffix) || !mount.RW {
						return nil, errors.New("shared runtime critical bind does not match its instance")
					}
					delete(required, mount.Destination)
				}
				continue
			}
			if mount.Type != "volume" {
				return nil, errors.New("shared runtime service uses an unsupported mount type")
			}
			switch mount.Destination {
			case "/data/game":
				if !volumes[mount.Name] || snapshot.GameVolume != "" {
					return nil, errors.New("shared runtime service game binding changed")
				}
				snapshot.GameVolume = mount.Name
			case "/data/steam-session":
				if service != "steam-auth" || !dockerVolumePattern.MatchString(mount.Name) || volumes[mount.Name] || snapshot.SessionVolume != "" {
					return nil, errors.New("shared runtime service session binding is unsafe")
				}
				snapshot.SessionVolume = mount.Name
			case "/config":
				// The image declares this volatile desktop parent volume; the
				// actual save leaf must still match the explicit critical bind.
				if service != "server" || !anonymousDeletionVolume.MatchString(mount.Name) {
					return nil, errors.New("shared runtime desktop volume is customized")
				}
			default:
				return nil, errors.New("shared runtime service has an unprotected volume")
			}
		}
		if snapshot.GameVolume == "" || len(required) != 0 || service == "steam-auth" && snapshot.SessionVolume == "" {
			return nil, errors.New("shared runtime service is missing its managed data bindings")
		}
		result = append(result, snapshot)
	}
	if len(expected) != 0 {
		return nil, errors.New("shared runtime service identities changed")
	}
	return result, nil
}

// Validate a complete fresh snapshot against the captured original/target image
// sets before stopping any ID. Re-query independently after this call before
// restoring data; no Compose or environment file is needed to stop services.
func (c *Client) RuntimeStopJunimo140Services(ctx context.Context, dir string, snapshots []RuntimeJunimo140ServiceSnapshot, allowedImages map[string][]string) error {
	seen := map[string]bool{}
	for _, snapshot := range snapshots {
		key := snapshot.Project + "/" + snapshot.Service
		if !composeProjectPattern.MatchString(snapshot.Project) || !validRuntimeServices([]string{snapshot.Service}) || !dockerContainerIDPattern.MatchString(snapshot.ContainerID) || seen[key] || !containsExactRuntimeImage(allowedImages[key], snapshot.ImageID) {
			return errors.New("shared runtime stop identity changed; no services stopped")
		}
		seen[key] = true
	}
	for _, snapshot := range snapshots {
		if snapshot.Running {
			if _, err := c.run(ctx, "stop exact shared runtime service", dir, 45*time.Second, "container", "stop", "--time", "20", snapshot.ContainerID); err != nil {
				return err
			}
		}
	}
	return nil
}

func containsExactRuntimeImage(images []string, image string) bool {
	if !runtimeDigestPattern.MatchString(image) {
		return false
	}
	for _, candidate := range images {
		if candidate == image {
			return true
		}
	}
	return false
}

// Containers are created stopped and pinned to local immutable image IDs. This
// also detaches staging after rollback without running an old game process.
func (c *Client) RuntimeCreateJunimo140ServicePinned(ctx context.Context, dir, project, service, imageID string) error {
	if !composeProjectPattern.MatchString(project) || !validRuntimeServices([]string{service}) || !runtimeDigestPattern.MatchString(imageID) {
		return errors.New("invalid pinned shared runtime recreation")
	}
	environment := []string{"SERVER_IMAGE=" + imageID, "STEAM_SERVICE_IMAGE=" + disabledSteamAuthImage}
	if service == "steam-auth" {
		environment = []string{"STEAM_SERVICE_IMAGE=" + imageID}
	}
	c.invalidateComposePs(dir)
	defer c.invalidateComposePs(dir)
	_, err := c.runWithEnvironment(ctx, "create stopped pinned shared runtime service", dir, c.timeouts.Up, environment,
		"compose", "--project-name", project, "up", "--no-start", "--no-deps", "--no-build", "--force-recreate", "--pull", "never", service)
	return err
}

func (c *Client) RuntimeStartJunimo140Services(ctx context.Context, dir string, snapshots []RuntimeJunimo140ServiceSnapshot, allowedImages map[string][]string) error {
	if err := validateJunimo140ServiceActions(snapshots, allowedImages); err != nil {
		return err
	}
	for _, snapshot := range snapshots {
		if !snapshot.Running {
			if _, err := c.run(ctx, "start exact shared runtime service", dir, c.timeouts.Up, "container", "start", snapshot.ContainerID); err != nil {
				return err
			}
		}
	}
	return nil
}

// rm --volumes removes only anonymous image volumes; protected game/session
// volumes are named and critical save/config data are verified host binds.
func (c *Client) RuntimeRemoveJunimo140Services(ctx context.Context, dir string, snapshots []RuntimeJunimo140ServiceSnapshot, allowedImages map[string][]string) error {
	if err := validateJunimo140ServiceActions(snapshots, allowedImages); err != nil {
		return err
	}
	for _, snapshot := range snapshots {
		if snapshot.Running {
			return errors.New("shared runtime removal requires quiescent services")
		}
	}
	for _, snapshot := range snapshots {
		if _, err := c.run(ctx, "remove stopped exact shared runtime service", dir, c.timeouts.Up, "container", "rm", "--volumes", snapshot.ContainerID); err != nil {
			return err
		}
	}
	return nil
}

func validateJunimo140ServiceActions(snapshots []RuntimeJunimo140ServiceSnapshot, allowedImages map[string][]string) error {
	seen := map[string]bool{}
	for _, snapshot := range snapshots {
		key := snapshot.Project + "/" + snapshot.Service
		if !composeProjectPattern.MatchString(snapshot.Project) || !validRuntimeServices([]string{snapshot.Service}) || !dockerContainerIDPattern.MatchString(snapshot.ContainerID) || seen[key] || !containsExactRuntimeImage(allowedImages[key], snapshot.ImageID) {
			return errors.New("shared runtime service identity changed; no services modified")
		}
		seen[key] = true
	}
	return nil
}
