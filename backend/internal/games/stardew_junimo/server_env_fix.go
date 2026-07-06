package stardew_junimo

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	serverContEnvDir          = ".local-container/cont-env"
	serverAppNameContEnvFile  = serverContEnvDir + "/APP_NAME"
	serverAppNameComposeMount = "      - ./.local-container/cont-env/APP_NAME:/etc/cont-env.d/APP_NAME:ro"
	serverAppNameScript       = "#!/bin/sh\nprintf '%s\\n' 'DockerApp'\n"
)

// EnsureServerContEnvFix masks a malformed APP_NAME cont-env script in the
// JunimoServer image. Some upstream builds contain a bare "DockerApp" token in
// /etc/cont-env.d/APP_NAME, which the init loader executes as a shell command.
func EnsureServerContEnvFix(dataDir string) (bool, error) {
	if dataDir == "" {
		return false, nil
	}
	changed := false
	target := filepath.Join(dataDir, filepath.FromSlash(serverAppNameContEnvFile))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return false, err
	}
	if current, err := os.ReadFile(target); err != nil || string(current) != serverAppNameScript {
		if err := os.WriteFile(target, []byte(serverAppNameScript), 0o755); err != nil {
			return false, err
		}
		changed = true
	}

	composeChanged, err := migrateServerContEnvFixMount(filepath.Join(dataDir, "docker-compose.yml"))
	if err != nil {
		return changed, err
	}
	return changed || composeChanged, nil
}

func migrateServerContEnvFixMount(path string) (bool, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	text := string(raw)
	if strings.Contains(text, "/etc/cont-env.d/APP_NAME") {
		return false, nil
	}

	for _, marker := range []string{
		"      - ./.local-container/control:/data/control",
		"      - ./.local-container/mods/StardewAnxiPanel.Control:/data/Mods/StardewAnxiPanel.Control",
		"      - ./.local-container/settings:/data/settings",
	} {
		updated := insertLineAfter(text, marker, serverAppNameComposeMount)
		if updated == text {
			continue
		}
		info, statErr := os.Stat(path)
		mode := os.FileMode(0o644)
		if statErr == nil {
			mode = info.Mode().Perm()
		}
		return true, os.WriteFile(path, []byte(updated), mode)
	}
	return false, nil
}
