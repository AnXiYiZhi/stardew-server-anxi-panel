//go:build windows

package stardew_junimo

import (
	"errors"

	"golang.org/x/sys/windows"
)

func renameImportNoReplace(source, target string) error {
	from, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	return windows.MoveFile(from, to)
}

func isImportCrossDeviceError(err error) bool { return errors.Is(err, windows.ERROR_NOT_SAME_DEVICE) }
