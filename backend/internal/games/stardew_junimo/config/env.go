// Package config manages stardew_junimo instance configuration files.
package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

const (
	DefaultSteamServiceImage                 = "anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2"
	DefaultSteamClientConnectTimeoutSeconds  = "60"
	DefaultSteamClientConnectRetries         = "5"
	DefaultSteamAuthSessionRetries           = "3"
	DefaultSteamAuthSessionRetryDelaySeconds = "5"
)

// ReadEnvFile reads key=value pairs from a .env file.
// Returns an empty map when the file does not exist.
func ReadEnvFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return make(map[string]string), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read .env: %w", err)
	}
	defer f.Close()

	result := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		idx := strings.IndexByte(trimmed, '=')
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(trimmed[:idx])
		value := unquoteEnvValue(strings.TrimSpace(trimmed[idx+1:]))
		if key != "" {
			result[key] = value
		}
	}
	return result, scanner.Err()
}

func unquoteEnvValue(value string) string {
	if len(value) < 2 {
		return value
	}
	quote := value[0]
	if (quote != '\'' && quote != '"') || value[len(value)-1] != quote {
		return value
	}
	return value[1 : len(value)-1]
}

// UpdateEnvFile reads the existing .env, merges updates, and writes back.
// Keys not present in updates are preserved unchanged.
// Callers must not log the updates map as it may contain passwords.
func UpdateEnvFile(path string, updates map[string]string) error {
	existing, err := ReadEnvFile(path)
	if err != nil {
		return err
	}
	for k, v := range updates {
		existing[k] = v
	}
	return writeEnvFile(path, existing)
}

// writeEnvFile serialises fields to path with 0600 permissions.
func writeEnvFile(path string, fields map[string]string) error {
	var sb strings.Builder

	// Write known keys in a stable order first.
	ordered := []string{
		"IMAGE_VERSION",
		"STEAM_SERVICE_IMAGE",
		"STEAM_USERNAME",
		"STEAM_PASSWORD",
		"STEAM_REFRESH_TOKEN",
		"STEAM_KEEP_LANGUAGES",
		"STEAM_CLIENT_CONNECT_TIMEOUT_SECONDS",
		"STEAM_CLIENT_CONNECT_RETRIES",
		"STEAM_AUTH_SESSION_RETRIES",
		"STEAM_AUTH_SESSION_RETRY_DELAY_SECONDS",
		"VNC_PASSWORD",
		"GAME_PORT",
		"QUERY_PORT",
		"VNC_PORT",
		"API_PORT",
		"STEAM_AUTH_PORT",
		"SERVER_TPS",
		"SERVER_FPS",
		"SERVER_PASSWORD",
		"MAX_LOGIN_ATTEMPTS",
		"AUTH_TIMEOUT_SECONDS",
		"API_ENABLED",
		"API_KEY",
		"ALLOW_INSECURE_SETUP",
		"DISCORD_BOT_TOKEN",
		"DISCORD_BOT_NICKNAME",
		"DISCORD_CHAT_CHANNEL_ID",
		"STATUS_DASHBOARD_CHANNEL_ID",
		"STATUS_DASHBOARD_REFRESH_RATE",
	}
	written := make(map[string]bool, len(fields))
	for _, k := range ordered {
		if v, ok := fields[k]; ok {
			sb.WriteString(k)
			sb.WriteByte('=')
			sb.WriteString(quoteEnvValue(v))
			sb.WriteByte('\n')
			written[k] = true
		}
	}
	// Append any remaining unknown keys.
	for k, v := range fields {
		if !written[k] {
			sb.WriteString(k)
			sb.WriteByte('=')
			sb.WriteString(quoteEnvValue(v))
			sb.WriteByte('\n')
		}
	}
	return os.WriteFile(path, []byte(sb.String()), 0o600)
}

func quoteEnvValue(value string) string {
	if value == "" || strings.ContainsAny(value, " \t#'\"") {
		escaped := strings.ReplaceAll(value, "\\", "\\\\")
		escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
		return "\"" + escaped + "\""
	}
	return value
}

// EmptyEnvTemplate returns placeholder key-value pairs for a fresh .env.
func EmptyEnvTemplate() map[string]string {
	return map[string]string{
		"IMAGE_VERSION":                          "",
		"STEAM_SERVICE_IMAGE":                    DefaultSteamServiceImage,
		"STEAM_USERNAME":                         "",
		"STEAM_PASSWORD":                         "",
		"STEAM_REFRESH_TOKEN":                    "",
		"STEAM_KEEP_LANGUAGES":                   "",
		"STEAM_CLIENT_CONNECT_TIMEOUT_SECONDS":   DefaultSteamClientConnectTimeoutSeconds,
		"STEAM_CLIENT_CONNECT_RETRIES":           DefaultSteamClientConnectRetries,
		"STEAM_AUTH_SESSION_RETRIES":             DefaultSteamAuthSessionRetries,
		"STEAM_AUTH_SESSION_RETRY_DELAY_SECONDS": DefaultSteamAuthSessionRetryDelaySeconds,
		"VNC_PASSWORD":                           "",
		"GAME_PORT":                              "24642",
		"QUERY_PORT":                             "27015",
		"VNC_PORT":                               "5800",
		"API_PORT":                               "8080",
		"STEAM_AUTH_PORT":                        "3001",
		"SERVER_TPS":                             "60",
		"SERVER_FPS":                             "0",
		"SERVER_PASSWORD":                        "",
		"MAX_LOGIN_ATTEMPTS":                     "3",
		"AUTH_TIMEOUT_SECONDS":                   "120",
		"API_ENABLED":                            "true",
		"API_KEY":                                "",
		"ALLOW_INSECURE_SETUP":                   "false",
		"DISCORD_BOT_TOKEN":                      "",
		"DISCORD_BOT_NICKNAME":                   "",
		"DISCORD_CHAT_CHANNEL_ID":                "",
		"STATUS_DASHBOARD_CHANNEL_ID":            "",
		"STATUS_DASHBOARD_REFRESH_RATE":          "30",
	}
}
