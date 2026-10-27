package docker

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
)

// Every holder, including stopped containers, must belong to a known regular
// Compose runtime service. One-shot downloads and unknown holders fail closed.
func (c *Client) RuntimeAssertJunimo140GameGroup(ctx context.Context, dir, volume string, projects []string, quiescent bool) error {
	if !dockerVolumePattern.MatchString(volume) || len(projects) == 0 || len(projects) > 256 {
		return errors.New("invalid shared game group")
	}
	allowed := map[string]bool{}
	for _, project := range projects {
		if !composeProjectPattern.MatchString(project) || allowed[project] {
			return errors.New("ambiguous shared game group")
		}
		allowed[project] = true
	}
	_, raw, err := c.runWithEnvironmentRaw(ctx, "list shared game volume holders", dir, c.timeouts.Ps, nil,
		"container", "ls", "--all", "--quiet", "--no-trunc", "--filter", "volume="+volume)
	if err != nil {
		return err
	}
	ids := strings.Fields(raw)
	if len(ids) == 0 {
		return nil
	}
	if len(ids) > 1024 {
		return errors.New("shared game holder count exceeds the supported bound")
	}
	for _, id := range ids {
		if !dockerContainerIDPattern.MatchString(id) {
			return errors.New("invalid shared game holder identity")
		}
	}
	_, raw, err = c.runWithEnvironmentRaw(ctx, "inspect shared game volume holders", dir, c.timeouts.Ps, nil, append([]string{"container", "inspect"}, ids...)...)
	if err != nil {
		return err
	}
	return validateJunimo140GameHolders(raw, ids, allowed, volume, quiescent)
}

func validateJunimo140GameHolders(raw string, ids []string, allowed map[string]bool, volume string, quiescent bool) error {
	var holders []struct {
		ID     string
		Config struct{ Labels map[string]string }
		State  struct {
			Running    bool
			Restarting bool
		}
		Mounts []struct {
			Type        string
			Name        string
			Destination string
		}
	}
	if json.Unmarshal([]byte(raw), &holders) != nil || len(holders) != len(ids) {
		return errors.New("shared game holder metadata is unavailable")
	}
	seen := map[string]bool{}
	for _, holder := range holders {
		labels := holder.Config.Labels
		service := labels["com.docker.compose.service"]
		if !dockerContainerIDPattern.MatchString(holder.ID) || seen[holder.ID] || !allowed[labels["com.docker.compose.project"]] || (service != "server" && service != "steam-auth") || strings.EqualFold(labels["com.docker.compose.oneoff"], "true") {
			return errors.New("shared game volume has an unknown or one-shot holder")
		}
		seen[holder.ID] = true
		mounted := false
		for _, mount := range holder.Mounts {
			if mount.Type == "volume" && mount.Name == volume {
				if mount.Destination != "/data/game" {
					return errors.New("shared game volume uses an unsupported mount contract")
				}
				mounted = true
			}
		}
		if !mounted || (quiescent && (holder.State.Running || holder.State.Restarting)) {
			return errors.New("shared game volume is not quiescent")
		}
	}
	for _, id := range ids {
		if !seen[id] {
			return errors.New("shared game holder set changed during inspection")
		}
	}
	return nil
}

// A verified archive is mandatory before changing the four managed runtime
// binds. Environment, credentials, journals, backups and unrelated paths keep
// their original ownership. The outer group transaction owns rollback.
func (c *Client) RuntimePrepareJunimo140Permissions(ctx context.Context, dir string, archive RuntimeInstanceArchiveRequest, uid, gid string) error {
	if err := validateRuntimeInstanceArchive(archive); err != nil {
		return err
	}
	if !runtimeArchiveSHA256.MatchString(archive.ExpectedSHA256) {
		return errors.New("verified permissions backup is required")
	}
	for _, value := range []string{uid, gid} {
		number, err := strconv.ParseUint(value, 10, 32)
		if err != nil || number == 4294967295 || strconv.FormatUint(number, 10) != value {
			return errors.New("invalid runtime ownership")
		}
	}
	if err := c.pinnedSMAPIImage(ctx, dir, archive.ImageRef, archive.ImageID); err != nil {
		return err
	}
	const script = `set -eu
actual=$(sha256sum "/backup/$2"); actual="${actual%% *}"
test "$actual" = "$1"
test -d /instance/.local-container
test ! -L /instance/.local-container
for name in saves settings control mods; do
    root="/instance/.local-container/$name"
    test ! -L "$root"
    if test -e "$root"; then
        test -d "$root"
        test -z "$(find "$root" -type f -links +1 -print -quit)"
        test -z "$(find "$root" -type l -print -quit)"
    fi
done
for name in saves settings control mods; do
    root="/instance/.local-container/$name"
    mkdir -p "$root"
    chown -R -P --no-dereference "$3:$4" "$root"
    chmod -R u+rwX "$root"
done
sync -f /instance`
	args := append([]string{"run", "--rm"}, junimo140InstanceHelperArgs(archive, "permissions")...)
	args = append(args, "--pull", "never", "--network", "none", "--read-only", "--tmpfs", "/tmp",
		"--mount", "type=bind,src="+archive.InstanceHostDir+",dst=/instance",
		"--mount", "type=bind,src="+archive.RecoveryHostDir+",dst=/backup,readonly",
		"--entrypoint", "sh", archive.ImageID, "-c", script, "--", archive.ExpectedSHA256, archive.Filename, uid, gid)
	_, err := c.run(ctx, "prepare backed-up Junimo runtime bind ownership", dir, 30*time.Minute, args...)
	return err
}

// Recovery can detach a staging volume without briefly starting the game.
// Only the two managed services may be recreated and no dependencies start.
func (c *Client) RuntimeComposeCreateStopped(ctx context.Context, dir, project, service string) error {
	if !composeProjectPattern.MatchString(project) || !validRuntimeServices([]string{service}) {
		return errors.New("invalid stopped runtime recreation")
	}
	args := []string{"compose", "--project-name", project, "up", "--no-start", "--no-deps", "--no-build", "--force-recreate", "--pull", "never", service}
	c.invalidateComposePs(dir)
	var err error
	if service == "server" {
		_, err = c.runWithEnvironment(ctx, "recreate stopped runtime server", dir, c.timeouts.Up, []string{"STEAM_SERVICE_IMAGE=" + disabledSteamAuthImage}, args...)
	} else {
		_, err = c.run(ctx, "recreate stopped runtime auth", dir, c.timeouts.Up, args...)
	}
	c.invalidateComposePs(dir)
	return err
}
