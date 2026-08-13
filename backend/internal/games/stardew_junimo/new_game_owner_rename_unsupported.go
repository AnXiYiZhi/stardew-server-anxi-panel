//go:build !linux && !windows

package stardew_junimo

import "errors"

// The supported Panel hosts are Linux containers and Windows development
// machines. Failing closed is safer than silently using rename semantics that
// could replace an empty directory on an unverified platform.
func renameNewGameOwnerNoReplace(_, _ string) error {
	return errors.New("atomic no-replace directory rename is unsupported on this platform")
}
