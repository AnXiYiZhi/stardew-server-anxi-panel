//go:build linux

package stardew_junimo

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// renameNewGameOwnerNoReplace is the single ownership linearization point on
// Linux. RENAME_NOREPLACE also refuses an unexpected empty legacy directory,
// unlike plain rename(2), which may replace one.
func renameNewGameOwnerNoReplace(oldPath, newPath string) error {
	err := unix.Renameat2(unix.AT_FDCWD, oldPath, unix.AT_FDCWD, newPath, unix.RENAME_NOREPLACE)
	if err == nil || !newGameOwnerRenameNeedsFallback(err) {
		return err
	}
	return renameNewGameOwnerViaExclusiveDirectory(oldPath, newPath)
}

func newGameOwnerRenameNeedsFallback(err error) bool {
	return errors.Is(err, unix.EINVAL) || errors.Is(err, unix.ENOSYS) || errors.Is(err, unix.EOPNOTSUPP)
}

// renameNewGameOwnerViaExclusiveDirectory preserves the no-replace ownership
// boundary on Linux filesystems such as Docker Desktop's 9p/DrvFS mount, which
// reject renameat2(RENAME_NOREPLACE). The successful mkdir is the single winner;
// contenders cannot replace its directory or owner.json.
func renameNewGameOwnerViaExclusiveDirectory(oldPath, newPath string) (retErr error) {
	stagingInfo, err := os.Lstat(oldPath)
	if err != nil {
		return err
	}
	if !stagingInfo.IsDir() || stagingInfo.Mode()&os.ModeSymlink != 0 {
		return errors.New("owner staging is not a plain directory")
	}

	stagedOwner := filepath.Join(oldPath, "owner.json")
	ownerInfo, err := os.Lstat(stagedOwner)
	if err != nil {
		return err
	}
	if !ownerInfo.Mode().IsRegular() || ownerInfo.Mode()&os.ModeSymlink != 0 {
		return errors.New("staged owner is not a plain file")
	}

	if err := os.Mkdir(newPath, 0o700); err != nil {
		return err
	}
	defer func() {
		if retErr == nil {
			return
		}
		if cleanupErr := os.Remove(newPath); cleanupErr != nil && !errors.Is(cleanupErr, os.ErrNotExist) {
			retErr = errors.Join(retErr, fmt.Errorf("remove incomplete owner directory: %w", cleanupErr))
		}
	}()

	return os.Rename(stagedOwner, filepath.Join(newPath, "owner.json"))
}
