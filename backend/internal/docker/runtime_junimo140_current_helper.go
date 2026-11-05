package docker

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strings"
	"time"
)

func currentReadArgument(args []string, flag string) string {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag {
			return args[i+1]
		}
	}
	return ""
}

// Current readers create and acknowledge a private helper before attaching.
// Canceling the Docker CLI is not a container cleanup primitive. No original
// server/Auth or source named volume is ever a removal target here.
func (c *Client) runJunimo140Read(ctx context.Context, op, dir string, timeout time.Duration, input io.Reader, volume, imageID string, projects []string, args ...string) (result CommandResult, raw string, resultErr error) {
	if projects == nil {
		if input != nil {
			result, resultErr = c.runWithInput(ctx, op, dir, timeout, input, args...)
			return result, result.Stdout, resultErr
		}
		return c.runWithEnvironmentRaw(ctx, op, dir, timeout, nil, args...)
	}
	name := currentReadArgument(args, "--name")
	if !strings.HasPrefix(name, "anxi-current-j140-") || len(name) != 42 || !runtimeDigestPattern.MatchString(imageID) || len(args) == 0 || args[0] != "run" {
		return result, "", errors.New("current reader requires a fresh exact helper identity")
	}
	imageVolumes, err := c.currentReadImageVolumes(ctx, dir, imageID)
	if err != nil {
		return result, "", err
	}
	ids, err := c.currentReadHelperIDs(ctx, dir, name, "")
	if err != nil || len(ids) != 0 {
		return result, "", errors.New("current reader helper identity already exists or is unavailable")
	}
	accepted := ""
	var automatic []string
	issued := false
	defer func() {
		if !issued {
			return
		}
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		resultErr = errors.Join(resultErr, c.removeCurrentReadHelper(cleanupCtx, dir, name, accepted, volume, imageID, imageVolumes, automatic))
	}()
	create := []string{"create"}
	for _, arg := range args[1:] {
		if arg != "--rm" {
			create = append(create, arg)
		}
	}
	issued = true
	created, createdRaw, err := c.runWithEnvironmentRaw(ctx, op+" create owned reader", dir, c.timeouts.Ps, nil, create...)
	if id := strings.TrimSpace(createdRaw); dockerContainerIDPattern.MatchString(id) && len(id) == 64 {
		accepted = id
	}
	if err != nil || accepted == "" {
		return created, "", errors.Join(err, errors.New("current reader create response did not acknowledge one exact ID"))
	}
	automatic, err = c.inspectCurrentReadHelper(ctx, dir, name, accepted, volume, imageID, imageVolumes)
	if err != nil {
		return created, "", err
	}
	start := []string{"start", "--attach"}
	if input != nil {
		start = append(start, "--interactive")
	}
	start = append(start, accepted)
	if input != nil {
		result, resultErr = c.runWithInput(ctx, op, dir, timeout, input, start...)
		raw = result.Stdout
	} else {
		result, raw, resultErr = c.runWithEnvironmentRaw(ctx, op, dir, timeout, nil, start...)
	}
	return
}

func (c *Client) currentReadImageVolumes(ctx context.Context, dir, imageID string) (map[string]json.RawMessage, error) {
	_, raw, err := c.runWithEnvironmentRaw(ctx, "inspect fixed reader image volumes", dir, c.timeouts.Ps, nil, "image", "inspect", "--format", `{{json .Id}}|{{json .Config.Volumes}}`, imageID)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(strings.TrimSpace(raw), "|")
	var id string
	var volumes map[string]json.RawMessage
	if len(parts) != 2 || json.Unmarshal([]byte(parts[0]), &id) != nil || id != imageID || json.Unmarshal([]byte(parts[1]), &volumes) != nil || len(volumes) > 16 {
		return nil, errors.New("fixed reader image volume declaration unavailable")
	}
	return volumes, nil
}

func (c *Client) currentReadHelperIDs(ctx context.Context, dir, name, id string) ([]string, error) {
	filter := "name=^/" + name + "$"
	if id != "" {
		filter = "id=" + id
	}
	_, raw, err := c.runWithEnvironmentRaw(ctx, "observe exact current reader", dir, c.timeouts.Ps, nil, "container", "ls", "--all", "--quiet", "--no-trunc", "--filter", filter)
	if err != nil {
		return nil, err
	}
	ids := strings.Fields(raw)
	if len(ids) > 1 || len(ids) == 1 && (!dockerContainerIDPattern.MatchString(ids[0]) || len(ids[0]) != 64 || id != "" && ids[0] != id) {
		return nil, errors.New("current reader identity is ambiguous")
	}
	return ids, nil
}

func (c *Client) inspectCurrentReadHelper(ctx context.Context, dir, name, id, volume, imageID string, imageVolumes map[string]json.RawMessage) ([]string, error) {
	_, raw, err := c.runWithEnvironmentRaw(ctx, "verify exact current reader ownership", dir, c.timeouts.Ps, nil, "container", "inspect", id)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		ID, Name, Image string
		Config          struct {
			Image, User string
			Labels      map[string]string
			Volumes     map[string]json.RawMessage
		}
		HostConfig struct {
			NetworkMode                            string
			ReadonlyRootfs, Privileged, AutoRemove *bool
			Binds, VolumesFrom                     []string
			CapAdd, CapDrop, SecurityOpt           []string
			Tmpfs                                  map[string]string
			Mounts                                 []struct {
				Type, Source, Target string
				ReadOnly             *bool
				VolumeOptions        *struct{ NoCopy *bool }
			}
		}
		Mounts []struct {
			Type, Name, Destination string
			RW                      *bool
		}
	}
	if json.Unmarshal([]byte(raw), &rows) != nil || len(rows) != 1 {
		return nil, errors.New("current reader ownership unavailable")
	}
	r := rows[0]
	labelVolume, hasVolumeLabel := r.Config.Labels["com.anxi-panel.game-data-volume"]
	if r.ID != id || r.Name != "/"+name || r.Image != imageID || r.Config.Image != imageID || r.Config.User != "0:0" || r.Config.Labels["com.anxi-panel.current-runtime-read"] != "true" || !hasVolumeLabel || labelVolume != volume || r.HostConfig.NetworkMode != "none" || r.HostConfig.ReadonlyRootfs == nil || !*r.HostConfig.ReadonlyRootfs || r.HostConfig.Privileged == nil || *r.HostConfig.Privileged || r.HostConfig.AutoRemove == nil || *r.HostConfig.AutoRemove || len(r.HostConfig.Binds) != 0 || len(r.HostConfig.VolumesFrom) != 0 {
		return nil, errors.New("current reader identity or safety configuration changed")
	}
	if len(r.HostConfig.CapDrop) != 1 || r.HostConfig.CapDrop[0] != "ALL" || len(r.HostConfig.CapAdd) != 1 || strings.TrimPrefix(r.HostConfig.CapAdd[0], "CAP_") != "DAC_READ_SEARCH" || len(r.HostConfig.SecurityOpt) != 1 || (r.HostConfig.SecurityOpt[0] != "no-new-privileges" && r.HostConfig.SecurityOpt[0] != "no-new-privileges:true") {
		return nil, errors.New("current reader DAC-only capability boundary changed")
	}
	if volume == "" {
		if len(r.HostConfig.Mounts) != 0 {
			return nil, errors.New("image-only reader acquired an unexpected mount")
		}
	} else {
		if len(r.HostConfig.Mounts) != 1 {
			return nil, errors.New("current reader source mount is ambiguous")
		}
		m := r.HostConfig.Mounts[0]
		if m.Type != "volume" || m.Source != volume || m.Target != "/game" || m.ReadOnly == nil || !*m.ReadOnly || m.VolumeOptions == nil || m.VolumeOptions.NoCopy == nil || !*m.VolumeOptions.NoCopy {
			return nil, errors.New("current reader lost readonly nocopy source")
		}
	}
	for destination := range r.HostConfig.Tmpfs {
		if destination != "/tmp" {
			return nil, errors.New("reader acquired an unexpected tmpfs")
		}
	}
	for destination := range imageVolumes {
		if _, ok := r.Config.Volumes[destination]; !ok {
			return nil, errors.New("reader image volume declaration changed")
		}
	}
	for destination := range r.Config.Volumes {
		if _, ok := imageVolumes[destination]; !ok && !(volume != "" && destination == "/game") {
			return nil, errors.New("reader added an undeclared automatic volume")
		}
	}
	seen := map[string]bool{}
	var anonymous []string
	foundGame := volume == ""
	for _, mount := range r.Mounts {
		if seen[mount.Destination] {
			return nil, errors.New("reader effective mounts are ambiguous")
		}
		seen[mount.Destination] = true
		if volume != "" && mount.Destination == "/game" {
			if mount.Type != "volume" || mount.Name != volume || mount.RW == nil || *mount.RW {
				return nil, errors.New("reader effective source mount changed")
			}
			foundGame = true
			continue
		}
		if mount.Type == "tmpfs" && mount.Destination == "/tmp" {
			if _, ok := r.HostConfig.Tmpfs["/tmp"]; ok {
				continue
			}
		}
		if _, ok := imageVolumes[mount.Destination]; !ok || mount.Type != "volume" || mount.RW == nil || mount.Name == volume || !dockerContainerIDPattern.MatchString(mount.Name) || len(mount.Name) != 64 {
			return nil, errors.New("reader automatic volume is not attributable to the fixed image")
		}
		_, holders, err := c.runWithEnvironmentRaw(ctx, "verify automatic reader volume sole holder", dir, c.timeouts.Ps, nil, "container", "ls", "--all", "--quiet", "--no-trunc", "--filter", "volume="+mount.Name)
		if err != nil || strings.TrimSpace(holders) != id {
			return nil, errors.New("reader automatic volume has unknown holders")
		}
		anonymous = append(anonymous, mount.Name)
	}
	if !foundGame {
		return nil, errors.New("reader effective source missing")
	}
	for destination := range imageVolumes {
		if !seen[destination] {
			return nil, errors.New("reader declared automatic volume has no effective mount")
		}
	}
	return anonymous, nil
}

func (c *Client) removeCurrentReadHelper(ctx context.Context, dir, name, accepted, volume, imageID string, imageVolumes map[string]json.RawMessage, createdAutomatic []string) error {
	ids, err := c.currentReadHelperIDs(ctx, dir, name, accepted)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return c.currentReadAutomaticAbsent(ctx, dir, createdAutomatic)
	}
	id := ids[0]
	anonymous, err := c.inspectCurrentReadHelper(ctx, dir, name, id, volume, imageID, imageVolumes)
	if err != nil {
		return err
	}
	if createdAutomatic != nil {
		sort.Strings(createdAutomatic)
		sort.Strings(anonymous)
		if strings.Join(createdAutomatic, "/") != strings.Join(anonymous, "/") {
			return errors.New("reader automatic volumes changed after create")
		}
	}
	if _, err := c.run(ctx, "remove exact owned current reader", dir, c.timeouts.Down, "container", "rm", "--force", "--volumes", id); err != nil {
		return err
	}
	remaining, err := c.currentReadHelperIDs(ctx, dir, name, id)
	if err != nil || len(remaining) != 0 {
		return errors.New("owned current reader removal is not proven")
	}
	return c.currentReadAutomaticAbsent(ctx, dir, anonymous)
}

func (c *Client) currentReadAutomaticAbsent(ctx context.Context, dir string, anonymous []string) error {
	for _, auto := range anonymous {
		_, raw, err := c.runWithEnvironmentRaw(ctx, "verify owned automatic reader volume absence", dir, c.timeouts.Ps, nil, "volume", "ls", "--quiet", "--filter", "name=^"+auto+"$")
		if err != nil || strings.TrimSpace(raw) != "" {
			return errors.New("owned automatic reader volume cleanup is not proven")
		}
	}
	return nil
}
