package config

import (
	"os"
	"path/filepath"
	"strconv"
)

// Build-time variables set via -ldflags. These override the corresponding
// environment variables when non-empty.
var (
	buildVersion string
	buildCommit  string
	buildDate    string
)

const (
	DefaultAddr                         = ":8090"
	DefaultDataDir                      = "/data"
	DefaultVersion                      = "dev"
	DefaultPanelMode                    = "single"
	DefaultInstanceID                   = "stardew"
	DefaultDriverID                     = "stardew_junimo"
	DefaultControlCommandRetentionDays  = 30
	DefaultControlCommandRetentionCount = 1000
	PanelModeSingle                     = "single"
	PanelModeMulti                      = "multi"
)

// Config contains process-level settings loaded from the environment.
type Config struct {
	Addr                         string
	DataDir                      string
	DBPath                       string
	Secret                       string
	Version                      string
	Commit                       string
	BuildDate                    string
	PanelMode                    string
	DefaultInstanceID            string
	DefaultDriverID              string
	ControlCommandRetentionDays  int
	ControlCommandRetentionCount int
}

// Load reads panel configuration from environment variables and applies defaults.
// Build-time ldflags take precedence over environment variables for version metadata.
func Load() Config {
	dataDir := getEnv("PANEL_DATA_DIR", DefaultDataDir)

	version := getEnv("PANEL_VERSION", DefaultVersion)
	if buildVersion != "" {
		version = buildVersion
	}
	commit := getEnv("PANEL_COMMIT", "")
	if buildCommit != "" {
		commit = buildCommit
	}
	buildDateVal := getEnv("PANEL_BUILD_DATE", "")
	if buildDate != "" {
		buildDateVal = buildDate
	}

	return Config{
		Addr:                         getEnv("PANEL_ADDR", DefaultAddr),
		DataDir:                      dataDir,
		DBPath:                       getEnv("PANEL_DB_PATH", filepath.Join(dataDir, "panel.db")),
		Secret:                       os.Getenv("PANEL_SECRET"),
		Version:                      version,
		Commit:                       commit,
		BuildDate:                    buildDateVal,
		PanelMode:                    panelMode(getEnv("PANEL_MODE", DefaultPanelMode)),
		DefaultInstanceID:            getEnv("DEFAULT_INSTANCE_ID", DefaultInstanceID),
		DefaultDriverID:              getEnv("DEFAULT_DRIVER_ID", DefaultDriverID),
		ControlCommandRetentionDays:  getPositiveIntEnv("CONTROL_COMMAND_RETENTION_DAYS", DefaultControlCommandRetentionDays),
		ControlCommandRetentionCount: getPositiveIntEnv("CONTROL_COMMAND_RETENTION_COUNT", DefaultControlCommandRetentionCount),
	}
}

func getPositiveIntEnv(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
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
