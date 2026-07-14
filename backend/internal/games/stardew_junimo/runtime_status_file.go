package stardew_junimo

import (
	"errors"
	"os"
	"time"
)

// readRuntimeStatusFile tolerates the brief sharing violation Windows can
// expose while an atomically replaced status file is being closed.
func readRuntimeStatusFile(path string) ([]byte, error) {
	const attempts = 8
	var (
		data []byte
		err  error
	)
	for attempt := 0; attempt < attempts; attempt++ {
		data, err = os.ReadFile(path)
		if err == nil || errors.Is(err, os.ErrNotExist) {
			return data, err
		}
		if attempt+1 < attempts {
			time.Sleep(5 * time.Millisecond)
		}
	}
	return nil, err
}
