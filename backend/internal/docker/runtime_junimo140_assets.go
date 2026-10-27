package docker

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"
)

func (c *Client) RuntimeExtractJunimo140Mod(ctx context.Context, dir string, request RuntimeInstanceArchiveRequest) error {
	if err := validateRuntimeInstanceArchive(request); err != nil {
		return err
	}
	if err := c.pinnedSMAPIImage(ctx, dir, request.ImageRef, request.ImageID); err != nil {
		return err
	}
	const script = `set -eu
umask 077
test ! -e "/backup/target-mod-$1"
test ! -L "/backup/target-mod-$1"
test -d /data/Mods/JunimoServer
test -z "$(find /data/Mods/JunimoServer -type l -print -quit)"
cp -a /data/Mods/JunimoServer "/backup/target-mod-$1"
find "/backup/target-mod-$1" -type d -exec chmod 755 {} +
find "/backup/target-mod-$1" -type f -exec chmod 644 {} +
sync -f /backup`
	args := append([]string{"run", "--rm"}, junimo140InstanceHelperArgs(request, "mod-extract")...)
	args = append(args, "--pull", "never", "--network", "none", "--read-only", "--tmpfs", "/tmp",
		"--mount", "type=bind,src="+request.RecoveryHostDir+",dst=/backup", "--entrypoint", "sh", request.ImageID, "-c", script, "--", strings.TrimSuffix(request.Filename, ".tar"))
	_, err := c.run(ctx, "extract exact shared runtime Junimo Mod", dir, 2*time.Minute, args...)
	return err
}

func (c *Client) RuntimePrepareJunimo140GamePermissions(ctx context.Context, dir, volume, imageRef, imageID, uid, gid string) error {
	if !smapiStagingVolumePattern.MatchString(volume) {
		return errors.New("invalid shared runtime staging ownership scope")
	}
	for _, value := range []string{uid, gid} {
		number, err := strconv.ParseUint(value, 10, 32)
		if err != nil || number == 4294967295 || strconv.FormatUint(number, 10) != value {
			return errors.New("invalid runtime ownership")
		}
	}
	if err := c.unusedSMAPIStaging(ctx, dir, volume); err != nil {
		return err
	}
	if err := c.pinnedSMAPIImage(ctx, dir, imageRef, imageID); err != nil {
		return err
	}
	const script = `set -eu
test -z "$(find /game -type f -links +1 -print -quit)"
chown -R -P --no-dereference "$1:$2" /game
chmod -R u+rwX /game
sync -f /game`
	args := append([]string{"run", "--rm"}, imageSMAPIHelperArgs(volume, "game-permissions")...)
	args = append(args, "--pull", "never", "--network", "none", "--read-only", "--tmpfs", "/tmp",
		"--mount", "type=volume,src="+volume+",dst=/game", "--entrypoint", "sh", imageID, "-c", script, "--", uid, gid)
	_, err := c.run(ctx, "prepare unused shared runtime game ownership", dir, 12*time.Minute, args...)
	return err
}
