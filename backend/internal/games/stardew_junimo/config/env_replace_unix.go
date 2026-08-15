//go:build !windows

package config

import "os"

func replaceEnvFile(source, target string) error {
	return os.Rename(source, target)
}
