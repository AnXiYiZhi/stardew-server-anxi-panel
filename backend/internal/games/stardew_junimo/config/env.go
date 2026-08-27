// Package config manages stardew_junimo instance configuration files.
package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	DefaultServerImage                       = "sdvd/server:1.5.0-preview.125"
	DefaultServerImageCandidates             = "dockerproxy.net/sdvd/server:1.5.0-preview.125,docker.1ms.run/sdvd/server:1.5.0-preview.125,docker.1panel.live/sdvd/server:1.5.0-preview.125,docker.jiaxin.site/sdvd/server:1.5.0-preview.125,dockerproxy.link/sdvd/server:1.5.0-preview.125,sdvd/server:1.5.0-preview.125"
	DefaultSteamServiceImage                 = "docker.1ms.run/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2"
	DefaultSteamServiceImageCandidates       = "docker.1ms.run/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2,crpi-9z3bkb9g7fxeohrg.cn-hangzhou.personal.cr.aliyuncs.com/anxi-panel/junimo-steam-service-cn:1.5.0-anxi.2,docker.m.daocloud.io/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2,ghcr.io/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2,anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2"
	DefaultSteamCMDImage                     = "docker.1ms.run/steamcmd/steamcmd:latest"
	DefaultSteamCMDImageCandidates           = "dockerproxy.net/steamcmd/steamcmd:latest,docker.1ms.run/steamcmd/steamcmd:latest,docker.1panel.live/steamcmd/steamcmd:latest,docker.jiaxin.site/steamcmd/steamcmd:latest,dockerproxy.link/steamcmd/steamcmd:latest,cm2network/steamcmd:latest"
	DefaultSMAPIVersion                      = "4.5.2"
	DefaultSMAPIDownloadURLs                 = "https://gh.llkk.cc/https://github.com/Pathoschild/SMAPI/releases/download/4.5.2/SMAPI-4.5.2-installer.zip,https://github.dpik.top/https://github.com/Pathoschild/SMAPI/releases/download/4.5.2/SMAPI-4.5.2-installer.zip,https://ghfast.top/https://github.com/Pathoschild/SMAPI/releases/download/4.5.2/SMAPI-4.5.2-installer.zip,https://github.com/Pathoschild/SMAPI/releases/download/4.5.2/SMAPI-4.5.2-installer.zip"
	DefaultSteamClientConnectTimeoutSeconds  = "60"
	DefaultSteamClientConnectRetries         = "5"
	DefaultSteamAuthSessionRetries           = "3"
	DefaultSteamAuthSessionRetryDelaySeconds = "5"

	SteamInviteAuthStateDisabled       = "disabled"
	SteamInviteAuthStatePending        = "pending"
	SteamInviteAuthStateAuthorizing    = "authorizing"
	SteamInviteAuthStateCleanupPending = "cleanup_pending"
	SteamInviteAuthStateReady          = "ready"
	SteamInviteAuthStateFailed         = "failed"

	// SteamInviteRuntimeScopeVersion marks instances whose optional Auth
	// runtime has been converged to the opt-in service scope. Fresh instances
	// start at this version; legacy instances write it only after their exact
	// steam-session runtime has either been preserved (enabled) or removed
	// (disabled).
	SteamInviteRuntimeScopeVersion = "1"
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
		line := strings.TrimPrefix(scanner.Text(), "\ufeff")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		idx := strings.IndexByte(trimmed, '=')
		if idx < 0 {
			continue
		}
		key := strings.TrimPrefix(strings.TrimSpace(trimmed[:idx]), "\ufeff")
		value := unquoteEnvValue(strings.TrimSpace(trimmed[idx+1:]))
		if key != "" {
			result[key] = value
		}
	}
	return result, scanner.Err()
}

// SteamAuthLoggedIn reports whether Steam authentication has succeeded for the
// instance at least once — the durable STEAM_AUTH_COMPLETED flag, which the driver
// sets when the steam-auth log shows a successful login (see markSteamAuthCompleted).
// Login persistence here is NOT the STEAM_REFRESH_TOKEN in .env (it is empty even in
// working setups); "authenticated per the log" is the correct signal.
func SteamAuthLoggedIn(dataDir string) bool {
	vals, err := ReadEnvFile(filepath.Join(dataDir, ".env"))
	if err != nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(vals["STEAM_AUTH_COMPLETED"]), "true")
}

// SteamInviteEnabled is the durable user intent for running the optional
// steam-auth sidecar and exposing Steam invite codes. Fresh instances write an
// explicit false value. Instances created before this flag existed remain
// enabled when they already completed steam-auth authorization.
func SteamInviteEnabled(dataDir string) bool {
	enabled, err := SteamInviteEnabledStrict(dataDir)
	return err == nil && enabled
}

// SteamInviteEnabledStrict returns the same durable intent as
// SteamInviteEnabled, but preserves read and malformed-value failures for
// recovery paths which must not silently shrink an already-started service
// scope.
func SteamInviteEnabledStrict(dataDir string) (bool, error) {
	envPath := filepath.Join(dataDir, ".env")
	info, err := os.Stat(envPath)
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf("Steam invite environment is not a regular file")
	}
	vals, err := ReadEnvFile(envPath)
	if err != nil {
		return false, err
	}
	if raw, exists := vals["STEAM_INVITE_ENABLED"]; exists {
		switch strings.ToLower(strings.TrimSpace(raw)) {
		case "true", "1", "yes", "on":
			return true, nil
		case "false", "0", "no", "off":
			return false, nil
		default:
			return false, fmt.Errorf("invalid STEAM_INVITE_ENABLED value")
		}
	}
	return strings.EqualFold(strings.TrimSpace(vals["STEAM_AUTH_COMPLETED"]), "true"), nil
}

// EnsureSteamInviteIntent writes the explicit intent key for instances that
// predate it. Existing explicit user intent always wins. Only durable
// steam-auth authorization evidence keeps the optional capability enabled;
// generic installed/running state and SteamCMD completion are not evidence.
func EnsureSteamInviteIntent(dataDir string, historicalAuthorization bool) (bool, error) {
	envPath := filepath.Join(dataDir, ".env")
	vals, err := ReadEnvFile(envPath)
	if err != nil {
		return false, err
	}
	if _, exists := vals["STEAM_INVITE_ENABLED"]; exists {
		return false, nil
	}

	authorized := strings.EqualFold(strings.TrimSpace(vals["STEAM_AUTH_COMPLETED"]), "true")
	if historicalAuthorization {
		authorized = true
	}
	enabled := authorized
	state := SteamInviteAuthStateDisabled
	if enabled {
		state = SteamInviteAuthStatePending
		if authorized {
			state = SteamInviteAuthStateReady
		}
	}
	updates := map[string]string{
		"STEAM_INVITE_ENABLED":    fmt.Sprintf("%t", enabled),
		"STEAM_INVITE_AUTH_STATE": state,
	}
	if authorized {
		updates["STEAM_AUTH_COMPLETED"] = "true"
	}
	return true, UpdateEnvFile(envPath, updates)
}

// SteamInviteRuntimeScopeCurrent reports whether the one-time optional Auth
// runtime convergence has completed. It reads only the instance .env and has
// no Docker side effects.
func SteamInviteRuntimeScopeCurrent(dataDir string) bool {
	vals, err := ReadEnvFile(filepath.Join(dataDir, ".env"))
	if err != nil {
		return false
	}
	return strings.TrimSpace(vals["STEAM_INVITE_RUNTIME_SCOPE_VERSION"]) == SteamInviteRuntimeScopeVersion
}

// MarkSteamInviteRuntimeScopeCurrent persists the convergence marker without
// changing SteamCMD credentials/cache or the steam-auth authorization flags.
func MarkSteamInviteRuntimeScopeCurrent(dataDir string) error {
	return UpdateEnvFile(filepath.Join(dataDir, ".env"), map[string]string{
		"STEAM_INVITE_RUNTIME_SCOPE_VERSION": SteamInviteRuntimeScopeVersion,
	})
}

// SetSteamInviteEnabled records explicit opt-in/out independently from either
// the SteamCMD authorization cache or the steam-auth login session.
func SetSteamInviteEnabled(dataDir string, enabled bool) error {
	state := SteamInviteAuthStateDisabled
	value := "false"
	if enabled {
		value = "true"
		state = SteamInviteAuthStatePending
		if SteamAuthLoggedIn(dataDir) {
			state = SteamInviteAuthStateReady
		}
	}
	return UpdateEnvFile(filepath.Join(dataDir, ".env"), map[string]string{
		"STEAM_INVITE_ENABLED":    value,
		"STEAM_INVITE_AUTH_STATE": state,
	})
}

// SteamInviteAuthState reports the optional capability state without changing
// the instance's base installation/lifecycle state.
func SteamInviteAuthState(dataDir string) string {
	if !SteamInviteEnabled(dataDir) {
		return SteamInviteAuthStateDisabled
	}
	vals, err := ReadEnvFile(filepath.Join(dataDir, ".env"))
	if err != nil {
		return SteamInviteAuthStatePending
	}
	state := strings.ToLower(strings.TrimSpace(vals["STEAM_INVITE_AUTH_STATE"]))
	if state == SteamInviteAuthStateCleanupPending {
		return state
	}
	if SteamAuthLoggedIn(dataDir) {
		return SteamInviteAuthStateReady
	}
	switch state {
	case SteamInviteAuthStatePending, SteamInviteAuthStateAuthorizing, SteamInviteAuthStateFailed:
		return state
	default:
		return SteamInviteAuthStatePending
	}
}

// SetSteamInviteAuthState updates only the optional invite authorization
// status. It never changes SteamCMD cache flags or game data.
func SetSteamInviteAuthState(dataDir, state string) error {
	switch state {
	case SteamInviteAuthStatePending, SteamInviteAuthStateAuthorizing, SteamInviteAuthStateCleanupPending, SteamInviteAuthStateReady, SteamInviteAuthStateFailed:
	default:
		return fmt.Errorf("invalid Steam invite auth state %q", state)
	}
	return UpdateEnvFile(filepath.Join(dataDir, ".env"), map[string]string{
		"STEAM_INVITE_AUTH_STATE": state,
	})
}

// SetSteamAuthLoggedIn persists whether the steam-auth service has a usable
// login. This flag is intentionally specific to steam-auth/Galaxy invite-code
// authorization; SteamCMD fallback success must not set it.
func SetSteamAuthLoggedIn(dataDir string, loggedIn bool) error {
	if loggedIn {
		return SetSteamAuthCompletedState(dataDir, SteamInviteAuthStateReady)
	}
	enabled := SteamInviteEnabled(dataDir)
	state := SteamInviteAuthStateDisabled
	if enabled {
		state = SteamInviteAuthStateFailed
	}
	return UpdateEnvFile(filepath.Join(dataDir, ".env"), map[string]string{
		"STEAM_AUTH_COMPLETED":    "",
		"STEAM_INVITE_ENABLED":    fmt.Sprintf("%t", enabled),
		"STEAM_INVITE_AUTH_STATE": state,
	})
}

// SetSteamAuthCompletedState records the successful Auth session and its
// one-shot holder cleanup state in one atomic env-file rewrite. This removes a
// crash window where STEAM_AUTH_COMPLETED could become true while the durable
// state still looked ready even though the one-shot container was unresolved.
func SetSteamAuthCompletedState(dataDir, state string) error {
	switch state {
	case SteamInviteAuthStateReady, SteamInviteAuthStateCleanupPending:
	default:
		return fmt.Errorf("invalid completed Steam invite auth state %q", state)
	}
	return UpdateEnvFile(filepath.Join(dataDir, ".env"), map[string]string{
		"STEAM_AUTH_COMPLETED":    "true",
		"STEAM_INVITE_ENABLED":    "true",
		"STEAM_INVITE_AUTH_STATE": state,
	})
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
		"GAME_DATA_VOLUME",
		"INSTANCE_HOST_DATA_DIR",
		"IMAGE_VERSION",
		"SERVER_IMAGE",
		"SERVER_IMAGE_CANDIDATES",
		"STEAM_SERVICE_IMAGE",
		"STEAM_SERVICE_IMAGE_CANDIDATES",
		"STEAMCMD_IMAGE",
		"STEAMCMD_IMAGE_CANDIDATES",
		"SMAPI_VERSION",
		"SMAPI_DOWNLOAD_URLS",
		"STEAMCMD_AUTH_COMPLETED",
		"STEAM_AUTH_COMPLETED",
		"STEAM_INVITE_ENABLED",
		"STEAM_INVITE_AUTH_STATE",
		"STEAM_INVITE_RUNTIME_SCOPE_VERSION",
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
		"SAP_PLAYER_AUTH_MODE",
		"SAP_PLAYER_AUTH_REVISION",
		"SAP_ROLE_AUTH_KEY",
		"SAP_ROLE_PASSWORDS_B64",
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
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create .env directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".env-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary .env: %w", err)
	}
	tmpPath := tmp.Name()
	keepTemp := true
	defer func() {
		_ = tmp.Close()
		if keepTemp {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		return fmt.Errorf("chmod temporary .env: %w", err)
	}
	if _, err := tmp.WriteString(sb.String()); err != nil {
		return fmt.Errorf("write temporary .env: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync temporary .env: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary .env: %w", err)
	}
	if err := replaceEnvFile(tmpPath, path); err != nil {
		return fmt.Errorf("replace .env: %w", err)
	}
	keepTemp = false
	return nil
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
		"GAME_DATA_VOLUME":                       "",
		"INSTANCE_HOST_DATA_DIR":                 "",
		"IMAGE_VERSION":                          "",
		"SERVER_IMAGE":                           DefaultServerImage,
		"SERVER_IMAGE_CANDIDATES":                DefaultServerImageCandidates,
		"STEAM_SERVICE_IMAGE":                    DefaultSteamServiceImage,
		"STEAM_SERVICE_IMAGE_CANDIDATES":         DefaultSteamServiceImageCandidates,
		"STEAMCMD_IMAGE":                         DefaultSteamCMDImage,
		"STEAMCMD_IMAGE_CANDIDATES":              DefaultSteamCMDImageCandidates,
		"SMAPI_VERSION":                          DefaultSMAPIVersion,
		"SMAPI_DOWNLOAD_URLS":                    DefaultSMAPIDownloadURLs,
		"STEAMCMD_AUTH_COMPLETED":                "",
		"STEAM_AUTH_COMPLETED":                   "",
		"STEAM_INVITE_ENABLED":                   "false",
		"STEAM_INVITE_AUTH_STATE":                SteamInviteAuthStateDisabled,
		"STEAM_INVITE_RUNTIME_SCOPE_VERSION":     SteamInviteRuntimeScopeVersion,
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
		"SAP_PLAYER_AUTH_MODE":                   "",
		"SAP_PLAYER_AUTH_REVISION":               "",
		"SAP_ROLE_AUTH_KEY":                      "",
		"SAP_ROLE_PASSWORDS_B64":                 "",
		"SERVER_PASSWORD":                        "",
		"MAX_LOGIN_ATTEMPTS":                     "3",
		"AUTH_TIMEOUT_SECONDS":                   "120",
		"API_ENABLED":                            "true",
		"API_KEY":                                "",
		"ALLOW_INSECURE_SETUP":                   "true",
		"DISCORD_BOT_TOKEN":                      "",
		"DISCORD_BOT_NICKNAME":                   "",
		"DISCORD_CHAT_CHANNEL_ID":                "",
		"STATUS_DASHBOARD_CHANNEL_ID":            "",
		"STATUS_DASHBOARD_REFRESH_RATE":          "30",
	}
}
