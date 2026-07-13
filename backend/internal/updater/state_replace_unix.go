//go:build !windows

package updater

import "os"

func atomicReplaceFile(source, target string) error { return os.Rename(source, target) }
