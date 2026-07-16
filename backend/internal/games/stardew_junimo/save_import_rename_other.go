//go:build !linux && !windows

package stardew_junimo

import (
	"errors"
	"io/fs"
	"os"
	"syscall"
)

func renameImportNoReplace(source, target string) error {
	if _, err := os.Lstat(target); err == nil {
		return fs.ErrExist
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.Rename(source, target)
}

func isImportCrossDeviceError(err error) bool { return errors.Is(err, syscall.EXDEV) }
