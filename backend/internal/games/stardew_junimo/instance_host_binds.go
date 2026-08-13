package stardew_junimo

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	legacyManagedBindPrefix = "      - ./.local-container/"
	hostManagedBindPrefix   = "      - ${INSTANCE_HOST_DATA_DIR}/.local-container/"
)

// ensureInstanceDockerHostBindings keeps Compose bind sources in the Docker
// daemon namespace even when PANEL_DATA_DIR and PANEL_HOST_DATA_DIR differ.
// Both files are required together once an instance has been prepared.
func (d *Driver) ensureInstanceDockerHostBindings(dataDir string) error {
	envPath := filepath.Join(dataDir, ".env")
	composePath := filepath.Join(dataDir, "docker-compose.yml")
	envInfo, envErr := os.Stat(envPath)
	composeInfo, composeErr := os.Stat(composePath)
	if errors.Is(envErr, os.ErrNotExist) && errors.Is(composeErr, os.ErrNotExist) {
		return nil
	}
	if envErr != nil {
		return fmt.Errorf("inspect instance environment for Docker host binds: %w", envErr)
	}
	if composeErr != nil {
		return fmt.Errorf("inspect instance Compose file for Docker host binds: %w", composeErr)
	}
	if !envInfo.Mode().IsRegular() || !composeInfo.Mode().IsRegular() {
		return errors.New("instance environment and Compose paths must be regular files")
	}

	hostInstanceDir, err := d.dockerHostPath(dataDir)
	if err != nil {
		return fmt.Errorf("map instance data directory for Docker: %w", err)
	}
	if !filepath.IsAbs(hostInstanceDir) || strings.ContainsAny(hostInstanceDir, "\x00\r\n") ||
		(runtime.GOOS != "windows" && strings.Contains(hostInstanceDir, ":")) {
		return errors.New("mapped instance host data directory is not safe for a Compose bind source")
	}
	envBytes, err := os.ReadFile(envPath)
	if err != nil {
		return fmt.Errorf("read instance environment for Docker host binds: %w", err)
	}
	if err := writeRuntimeEnvUpdatesAtomic(dataDir, envBytes, map[string]string{"INSTANCE_HOST_DATA_DIR": hostInstanceDir}); err != nil {
		return fmt.Errorf("persist instance host data directory: %w", err)
	}

	composeBytes, err := os.ReadFile(composePath)
	if err != nil {
		return fmt.Errorf("read instance Compose file for Docker host binds: %w", err)
	}
	compose := string(composeBytes)
	migrated := strings.ReplaceAll(compose, legacyManagedBindPrefix, hostManagedBindPrefix)
	if migrated == compose {
		return nil
	}
	if err := atomicWriteRaw(composePath, []byte(migrated), composeInfo.Mode().Perm()); err != nil {
		return fmt.Errorf("migrate instance Compose host bind sources: %w", err)
	}
	return nil
}
