package stardew_junimo

import (
	"bytes"
	_ "embed"
	"os"
	"path/filepath"
)

//go:embed embedded/smapi-mod/manifest.json
var smapiModManifest []byte

//go:embed embedded/smapi-mod/StardewAnxiPanel.Control.dll
var smapiModDLL []byte

// smapiModDir returns the host-side path where the SMAPI mod files are installed.
func smapiModDir(dataDir string) string {
	return filepath.Join(dataDir, ".local-container", "mods", "StardewAnxiPanel.Control")
}

// installSMAPIMod writes the embedded SMAPI mod to the instance's bind-mounted mods directory.
// It is idempotent: existing files with identical content are not overwritten.
func installSMAPIMod(dataDir string) error {
	dir := smapiModDir(dataDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := writeFileIfDifferent(filepath.Join(dir, "manifest.json"), smapiModManifest); err != nil {
		return err
	}
	return writeFileIfDifferent(filepath.Join(dir, "StardewAnxiPanel.Control.dll"), smapiModDLL)
}

func writeFileIfDifferent(path string, data []byte) error {
	existing, err := os.ReadFile(path)
	if err == nil && bytes.Equal(existing, data) {
		return nil
	}
	return os.WriteFile(path, data, 0o644)
}
