//go:build windows

package stardew_junimo

import "golang.org/x/sys/windows"

// renameNewGameOwnerNoReplace publishes a fully staged directory without ever
// replacing an existing fixed owner path. MoveFile has no replace-existing
// flag, which is the required ownership contract on Windows.
func renameNewGameOwnerNoReplace(oldPath, newPath string) error {
	oldPtr, err := windows.UTF16PtrFromString(oldPath)
	if err != nil {
		return err
	}
	newPtr, err := windows.UTF16PtrFromString(newPath)
	if err != nil {
		return err
	}
	return windows.MoveFile(oldPtr, newPtr)
}
