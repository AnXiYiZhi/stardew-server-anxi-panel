//go:build linux

package stardew_junimo

import "golang.org/x/sys/unix"

// renameNewGameOwnerNoReplace is the single ownership linearization point on
// Linux. RENAME_NOREPLACE also refuses an unexpected empty legacy directory,
// unlike plain rename(2), which may replace one.
func renameNewGameOwnerNoReplace(oldPath, newPath string) error {
	return unix.Renameat2(unix.AT_FDCWD, oldPath, unix.AT_FDCWD, newPath, unix.RENAME_NOREPLACE)
}
