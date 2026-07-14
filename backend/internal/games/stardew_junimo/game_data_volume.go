package stardew_junimo

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
)

var gameDataVolumeNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)

// GameDataVolumeName returns the explicit selected volume. Legacy instances
// without GAME_DATA_VOLUME safely resolve to their historical Compose volume.
func GameDataVolumeName(dataDir string) (string, error) {
	if !filepath.IsAbs(dataDir) {
		return "", errors.New("instance data directory must be absolute")
	}
	project := strings.ToLower(filepath.Base(filepath.Clean(dataDir)))
	if !runtimeComposeProjectPattern.MatchString(project) {
		return "", errors.New("invalid instance compose project")
	}
	values, err := sjconfig.ReadEnvFile(filepath.Join(dataDir, ".env"))
	if err != nil {
		return "", err
	}
	name := strings.TrimSpace(values["GAME_DATA_VOLUME"])
	if name == "" {
		name = project + "_game-data"
	}
	if !gameDataVolumeNamePattern.MatchString(name) {
		return "", errors.New("invalid GAME_DATA_VOLUME")
	}
	return name, nil
}

func resolvedGameDataVolumeName(dataDir string) string {
	if name, err := GameDataVolumeName(dataDir); err == nil {
		return name
	}
	return strings.ToLower(filepath.Base(filepath.Clean(dataDir))) + "_game-data"
}

// EnsureGameDataVolumeBinding migrates legacy Compose files to an explicit
// top-level volume name and records the current historical volume in .env.
// It never creates, copies, removes, or switches a Docker volume.
func EnsureGameDataVolumeBinding(dataDir string) error {
	volume, err := GameDataVolumeName(dataDir)
	if err != nil {
		return err
	}
	composePath := filepath.Join(dataDir, "docker-compose.yml")
	data, err := os.ReadFile(composePath)
	if err != nil {
		return err
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	if strings.Contains(text, "name: ${GAME_DATA_VOLUME}") {
		return sjconfig.UpdateEnvFile(filepath.Join(dataDir, ".env"), map[string]string{"GAME_DATA_VOLUME": volume})
	}
	needle := "volumes:\n  steam-session:\n  game-data:\n"
	if !strings.Contains(text, needle) {
		// Preserve user-managed/custom Compose and env files. Read-only runtime
		// inspection will classify unsupported layouts without rewriting them.
		return nil
	}
	envPath := filepath.Join(dataDir, ".env")
	if err := sjconfig.UpdateEnvFile(envPath, map[string]string{"GAME_DATA_VOLUME": volume}); err != nil {
		return fmt.Errorf("persist GAME_DATA_VOLUME: %w", err)
	}
	text = strings.Replace(text, needle, "volumes:\n  steam-session:\n  game-data:\n    name: ${GAME_DATA_VOLUME}\n", 1)
	tmp, err := os.CreateTemp(dataDir, ".compose-game-volume-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.WriteString(text); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return replaceRuntimeUpdateStatusFile(name, composePath)
}

func switchGameDataVolumeAtomic(dataDir, volume string) error {
	if !gameDataVolumeNamePattern.MatchString(volume) {
		return errors.New("invalid target GAME_DATA_VOLUME")
	}
	envPath := filepath.Join(dataDir, ".env")
	values, err := sjconfig.ReadEnvFile(envPath)
	if err != nil {
		return err
	}
	values["GAME_DATA_VOLUME"] = volume
	tmp, err := os.CreateTemp(dataDir, ".env-smapi-switch-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if err := tmp.Close(); err != nil {
		return err
	}
	defer os.Remove(tmpName)
	if err := sjconfig.UpdateEnvFile(tmpName, values); err != nil {
		return err
	}
	return replaceRuntimeUpdateStatusFile(tmpName, envPath)
}
