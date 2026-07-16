//go:build linux

package stardew_junimo

import (
	"errors"
	"syscall"

	"golang.org/x/sys/unix"
)

func renameImportNoReplace(source, target string) error {
	return unix.Renameat2(unix.AT_FDCWD, source, unix.AT_FDCWD, target, unix.RENAME_NOREPLACE)
}

func isImportCrossDeviceError(err error) bool { return errors.Is(err, syscall.EXDEV) }
