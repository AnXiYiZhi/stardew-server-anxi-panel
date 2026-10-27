package docker

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// The session volume must have no live or unrelated holder. This check is also
// used before restore, when the old and new auth containers may differ.
func (c *Client) assertJunimo140SessionQuiescent(ctx context.Context, dir, project, volume string) error {
	if !composeProjectPattern.MatchString(project) || !dockerVolumePattern.MatchString(volume) {
		return errors.New("invalid shared runtime session identity")
	}
	if _, err := c.RuntimeVolumeInspect(ctx, dir, volume); err != nil {
		return err
	}
	_, raw, err := c.runWithEnvironmentRaw(ctx, "list protected session holders", dir, c.timeouts.Ps, nil, "container", "ls", "--all", "--quiet", "--no-trunc", "--filter", "volume="+volume)
	if err != nil {
		return err
	}
	ids, err := junimo140HelperIDs(raw)
	if err != nil || len(ids) == 0 {
		return err
	}
	_, raw, err = c.runWithEnvironmentRaw(ctx, "inspect protected session holders", dir, c.timeouts.Ps, nil, append([]string{"container", "inspect"}, ids...)...)
	if err != nil {
		return err
	}
	return validateJunimo140SessionHolders(raw, ids, project, volume)
}

func validateJunimo140SessionHolders(raw string, ids []string, project, volume string) error {
	var records []struct {
		ID     string
		Config struct{ Labels map[string]string }
		State  struct{ Running, Restarting bool }
		Mounts []struct{ Type, Name string }
	}
	if json.Unmarshal([]byte(raw), &records) != nil || len(records) != len(ids) {
		return errors.New("protected session holder metadata unavailable")
	}
	expected := map[string]bool{}
	for _, id := range ids {
		expected[id] = true
	}
	for _, record := range records {
		labels := record.Config.Labels
		if !expected[record.ID] || labels["com.docker.compose.project"] != project || labels["com.docker.compose.service"] != "steam-auth" || strings.EqualFold(labels["com.docker.compose.oneoff"], "true") || record.State.Running || record.State.Restarting {
			return errors.New("protected session has a live or unknown holder")
		}
		mounted := false
		for _, mount := range record.Mounts {
			if mount.Type == "volume" && mount.Name == volume {
				mounted = true
			}
		}
		if !mounted {
			return errors.New("protected session holder mount changed")
		}
		delete(expected, record.ID)
	}
	if len(expected) != 0 {
		return errors.New("protected session holder identities changed")
	}
	return nil
}

// Store the session as an immutable ACL/numeric-owner tar in the same private
// group recovery directory as the instance archives. No credential is returned.
func (c *Client) RuntimeArchiveJunimo140Session(ctx context.Context, dir string, request RuntimeInstanceArchiveRequest, volume string) (string, error) {
	if err := validateRuntimeInstanceArchive(request); err != nil {
		return "", err
	}
	if err := c.pinnedSMAPIImage(ctx, dir, request.ImageRef, request.ImageID); err != nil {
		return "", err
	}
	if err := c.assertJunimo140SessionQuiescent(ctx, dir, request.Project, volume); err != nil {
		return "", err
	}
	const script = `set -eu
umask 077
test ! -e "/backup/$1"
test ! -L "/backup/$1"
tmp="/backup/$1.tmp"
test ! -e "$tmp"
test ! -L "$tmp"
trap 'rm -f "$tmp"' EXIT
tar --acls --xattrs --numeric-owner -cpf "$tmp" -C /session .
chmod 0600 "$tmp"
sync -f "$tmp"
ln "$tmp" "/backup/$1"
sync -f /backup
sha256sum "/backup/$1"`
	args := append([]string{"run", "--rm"}, junimo140InstanceHelperArgs(request, "session-archive")...)
	args = append(args, "--pull", "never", "--network", "none", "--read-only", "--tmpfs", "/tmp", "--mount", "type=volume,src="+volume+",dst=/session,readonly", "--mount", "type=bind,src="+request.RecoveryHostDir+",dst=/backup", "--entrypoint", "timeout", request.ImageID, "--kill-after=2s", "600s", "sh", "-c", script, "--", request.Filename)
	result, err := c.run(ctx, "archive quiescent protected runtime session", dir, 11*time.Minute, args...)
	if err != nil {
		return "", err
	}
	parts := strings.Fields(result.Stdout)
	if result.StdoutTruncated || len(parts) != 2 || !runtimeArchiveSHA256.MatchString(parts[0]) || parts[1] != "/backup/"+request.Filename {
		return "", errors.New("invalid protected session archive checksum")
	}
	return parts[0], nil
}

func (c *Client) RuntimeRestoreJunimo140Session(ctx context.Context, dir string, request RuntimeInstanceArchiveRequest, volume string) error {
	if err := validateRuntimeInstanceArchive(request); err != nil {
		return err
	}
	if !runtimeArchiveSHA256.MatchString(request.ExpectedSHA256) {
		return errors.New("verified protected session backup is required")
	}
	if err := c.pinnedSMAPIImage(ctx, dir, request.ImageRef, request.ImageID); err != nil {
		return err
	}
	if err := c.assertJunimo140SessionQuiescent(ctx, dir, request.Project, volume); err != nil {
		return err
	}
	const script = `set -eu
test -f "/backup/$1"
test ! -L "/backup/$1"
actual=$(sha256sum "/backup/$1"); actual="${actual%% *}"
test "$actual" = "$2"
tar -tf "/backup/$1" >/dev/null
find /session -mindepth 1 -maxdepth 1 -exec rm -rf -- {} +
tar --acls --xattrs --same-owner --same-permissions -xpf "/backup/$1" -C /session
sync -f /session`
	args := append([]string{"run", "--rm"}, junimo140InstanceHelperArgs(request, "session-restore")...)
	args = append(args, "--pull", "never", "--network", "none", "--read-only", "--tmpfs", "/tmp", "--mount", "type=volume,src="+volume+",dst=/session", "--mount", "type=bind,src="+request.RecoveryHostDir+",dst=/backup,readonly", "--entrypoint", "timeout", request.ImageID, "--kill-after=2s", "600s", "sh", "-c", script, "--", request.Filename, request.ExpectedSHA256)
	_, err := c.run(ctx, "restore verified protected runtime session", dir, 11*time.Minute, args...)
	return err
}
