//go:build !windows

package stardew_junimo

import "os"

func replaceRoleCredentialStoreFile(source, target string) error {
	return os.Rename(source, target)
}
