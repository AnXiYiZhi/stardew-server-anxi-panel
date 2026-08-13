package stardew_junimo

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// dockerHostPath maps a path visible inside the Panel container to the
// equivalent source path visible to the Docker daemon. Legacy and test setups
// without PANEL_HOST_DATA_DIR keep the existing same-path behavior.
func (d *Driver) dockerHostPath(containerPath string) (string, error) {
	containerPath = strings.TrimSpace(containerPath)
	if containerPath == "" {
		return "", errors.New("Docker bind source is empty")
	}
	absPath, err := filepath.Abs(containerPath)
	if err != nil {
		return "", fmt.Errorf("resolve container data path: %w", err)
	}
	if d == nil || strings.TrimSpace(d.hostDataDir) == "" {
		return absPath, nil
	}
	containerRoot := filepath.Clean(strings.TrimSpace(d.containerDataDir))
	hostRoot := filepath.Clean(strings.TrimSpace(d.hostDataDir))
	if !filepath.IsAbs(containerRoot) || !filepath.IsAbs(hostRoot) {
		return "", errors.New("Panel container and host data directories must be absolute")
	}
	relative, err := filepath.Rel(containerRoot, absPath)
	if err != nil {
		return "", fmt.Errorf("map container data path: %w", err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", errors.New("Docker bind source is outside the Panel data directory")
	}
	return filepath.Join(hostRoot, relative), nil
}
