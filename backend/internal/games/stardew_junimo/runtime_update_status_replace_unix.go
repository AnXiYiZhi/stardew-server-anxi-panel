//go:build !windows

package stardew_junimo

import "os"

func replaceRuntimeUpdateStatusFile(source, target string) error { return os.Rename(source, target) }
