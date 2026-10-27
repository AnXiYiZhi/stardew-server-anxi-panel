package docker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
	"time"
)

type RuntimeInstanceArchiveRequest struct {
	Project         string
	InstanceHostDir string
	RecoveryHostDir string
	Filename        string
	ExpectedSHA256  string
	ImageRef        string
	ImageID         string
}

var runtimeGroupArchiveName = regexp.MustCompile(`^[a-f0-9]{12}\.tar$`)
var runtimeGroupRecoveryPath = regexp.MustCompile(`/\.local-container/junimo140-group/apply_[a-f0-9]{24}$`)
var runtimeArchiveSHA256 = regexp.MustCompile(`^[a-f0-9]{64}$`)

func junimo140InstanceHelperArgs(request RuntimeInstanceArchiveRequest, operation string) []string {
	recovery := strings.TrimRight(strings.ReplaceAll(request.RecoveryHostDir, `\`, "/"), "/")
	applyID := recovery[strings.LastIndex(recovery, "/")+1:]
	hash := sha256.Sum256([]byte(request.Project + "\n" + request.InstanceHostDir))
	name := "anxi-j140-" + strings.TrimPrefix(applyID, "apply_") + "-" + hex.EncodeToString(hash[:6]) + "-" + operation
	return []string{"--name", name,
		"--label", "com.anxi-panel.junimo140-helper=true",
		"--label", "com.anxi-panel.compose-project=" + request.Project,
		"--label", "com.anxi-panel.runtime-update-apply-id=" + applyID,
		"--label", "com.anxi-panel.junimo140-operation=" + operation}
}

func validateRuntimeInstanceArchive(request RuntimeInstanceArchiveRequest) error {
	normalize := func(value string) string { return strings.TrimRight(strings.ReplaceAll(value, `\`, "/"), "/") }
	source, recovery := normalize(request.InstanceHostDir), normalize(request.RecoveryHostDir)
	absolute := func(value string) bool {
		return strings.HasPrefix(value, "/") || (len(value) > 3 && value[1] == ':' && value[2] == '/')
	}
	if !composeProjectPattern.MatchString(request.Project) || !runtimeGroupArchiveName.MatchString(request.Filename) ||
		!absolute(source) || !absolute(recovery) || strings.Count(source, "/") < 2 ||
		!runtimeGroupRecoveryPath.MatchString(recovery) || strings.ContainsAny(source+recovery, ",\r\n\x00") ||
		!strings.EqualFold(source[strings.LastIndex(source, "/")+1:], request.Project) ||
		validateRestrictedImageRef(request.ImageRef) != nil || !runtimeDigestPattern.MatchString(request.ImageID) {
		return errors.New("invalid private instance archive request")
	}
	for _, value := range []string{source, recovery} {
		for _, part := range strings.Split(value, "/") {
			if part == "." || part == ".." {
				return errors.New("instance archive path is not canonical")
			}
		}
	}
	return nil
}

// RuntimeArchiveJunimo140Instance preserves numeric ownership, mode and ACLs.
// Group journals/leases are excluded so restoring data cannot erase the owner
// which protects a partially recovered group from unrelated mutations.
func (c *Client) RuntimeArchiveJunimo140Instance(ctx context.Context, dir string, request RuntimeInstanceArchiveRequest) (string, error) {
	if err := validateRuntimeInstanceArchive(request); err != nil {
		return "", err
	}
	if err := c.pinnedSMAPIImage(ctx, dir, request.ImageRef, request.ImageID); err != nil {
		return "", err
	}
	const script = `set -eu
umask 077
tmp="/backup/$1.tmp"
test ! -e "/backup/$1"
test ! -L "/backup/$1"
test ! -L /instance/.local-container
trap 'rm -f "$tmp"' EXIT
tar --acls --xattrs --numeric-owner -cpf "$tmp" \
    --exclude=./.local-container/junimo140-group \
    --exclude=./.local-container/junimo140-lease.json \
    --exclude=./.local-container/junimo-update \
    -C /instance .
sync -f "$tmp"
ln "$tmp" "/backup/$1"
rm -f "$tmp"
sync -f /backup
sha256sum "/backup/$1"`
	args := append([]string{"run", "--rm"}, junimo140InstanceHelperArgs(request, "archive")...)
	args = append(args, "--pull", "never", "--network", "none", "--read-only", "--tmpfs", "/tmp",
		"--mount", "type=bind,src="+request.InstanceHostDir+",dst=/instance,readonly",
		"--mount", "type=bind,src="+request.RecoveryHostDir+",dst=/backup",
		"--entrypoint", "sh", request.ImageID, "-c", script, "--", request.Filename)
	result, err := c.run(ctx, "archive quiescent Junimo instance data", dir, 30*time.Minute, args...)
	if err != nil {
		return "", err
	}
	fields := strings.Fields(result.Stdout)
	if len(fields) != 2 || !runtimeArchiveSHA256.MatchString(fields[0]) {
		return "", errors.New("instance archive checksum is unavailable")
	}
	return fields[0], nil
}

func (c *Client) RuntimeRestoreJunimo140Instance(ctx context.Context, dir string, request RuntimeInstanceArchiveRequest) error {
	if err := validateRuntimeInstanceArchive(request); err != nil {
		return err
	}
	if !runtimeArchiveSHA256.MatchString(request.ExpectedSHA256) {
		return errors.New("invalid instance archive checksum")
	}
	if err := c.pinnedSMAPIImage(ctx, dir, request.ImageRef, request.ImageID); err != nil {
		return err
	}
	const script = `set -eu
actual=$(sha256sum "/backup/$2"); actual="${actual%% *}"
test "$actual" = "$1"
test ! -L /instance/.local-container
tar -tf "/backup/$2" >/dev/null
find /instance -mindepth 1 -maxdepth 1 ! -name .local-container -exec rm -rf -- {} +
if test -d /instance/.local-container; then
    find /instance/.local-container -mindepth 1 -maxdepth 1 \
        ! -name junimo140-group ! -name junimo140-lease.json ! -name junimo-update \
        -exec rm -rf -- {} +
fi
tar --acls --xattrs --numeric-owner --same-owner --same-permissions -xpf "/backup/$2" -C /instance
sync -f /instance`
	args := append([]string{"run", "--rm"}, junimo140InstanceHelperArgs(request, "restore")...)
	args = append(args, "--pull", "never", "--network", "none", "--read-only", "--tmpfs", "/tmp",
		"--mount", "type=bind,src="+request.InstanceHostDir+",dst=/instance",
		"--mount", "type=bind,src="+request.RecoveryHostDir+",dst=/backup,readonly",
		"--entrypoint", "sh", request.ImageID, "-c", script, "--", request.ExpectedSHA256, request.Filename)
	_, err := c.run(ctx, "restore verified Junimo instance data and permissions", dir, 30*time.Minute, args...)
	return err
}
