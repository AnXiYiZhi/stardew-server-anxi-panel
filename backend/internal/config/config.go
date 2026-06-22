package config

import (
	"os"
	"path/filepath"
)

const (
	DefaultAddr       = ":8090"
	DefaultDataDir    = "/data"
	DefaultVersion    = "dev"
	DefaultPanelMode  = "single"
	DefaultInstanceID = "stardew"
	DefaultDriverID   = "stardew_junimo"
	PanelModeSingle   = "single"
	PanelModeMulti    = "multi"
)

// Config contains process-level settings loaded from the environment.
type Config struct {
	Addr              string
	DataDir           string
	DBPath            string
	Secret            string
	Version           string
	PanelMode         string
	DefaultInstanceID string
	DefaultDriverID   string
}

// Load reads panel configuration from environment variables and applies defaults.
func Load() Config {
	dataDir := getEnv("PANEL_DATA_DIR", DefaultDataDir)

	return Config{
		Addr:              getEnv("PANEL_ADDR", DefaultAddr),
		DataDir:           dataDir,
		DBPath:            getEnv("PANEL_DB_PATH", filepath.Join(dataDir, "panel.db")),
		Secret:            os.Getenv("PANEL_SECRET"),
		Version:           getEnv("PANEL_VERSION", DefaultVersion),
		PanelMode:         panelMode(getEnv("PANEL_MODE", DefaultPanelMode)),
		DefaultInstanceID: getEnv("DEFAULT_INSTANCE_ID", DefaultInstanceID),
		DefaultDriverID:   getEnv("DEFAULT_DRIVER_ID", DefaultDriverID),
	}
}

func getEnv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func panelMode(value string) string {
	switch value {
	case PanelModeSingle, PanelModeMulti:
		return value
	default:
		return DefaultPanelMode
	}
}
