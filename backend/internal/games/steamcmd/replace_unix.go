//go:build !windows

package steamcmd

import "os"

func replaceCredentialsFile(source, target string) error {
	return os.Rename(source, target)
}
