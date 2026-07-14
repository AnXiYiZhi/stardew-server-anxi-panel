//go:build windows

package stardew_junimo

import (
	"errors"
	"time"

	"golang.org/x/sys/windows"
)

func replaceRuntimeUpdateStatusFile(source, target string) error {
	sourcePtr, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	targetPtr, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	var moveErr error
	for attempt := 0; attempt < 20; attempt++ {
		moveErr = windows.MoveFileEx(sourcePtr, targetPtr, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
		if moveErr == nil {
			return nil
		}
		if !errors.Is(moveErr, windows.ERROR_SHARING_VIOLATION) && !errors.Is(moveErr, windows.ERROR_ACCESS_DENIED) {
			return moveErr
		}
		time.Sleep(25 * time.Millisecond)
	}
	return moveErr
}
