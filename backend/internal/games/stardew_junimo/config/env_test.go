package config_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
)

func TestReadEnvFile_NotExist(t *testing.T) {
	fields, err := config.ReadEnvFile(filepath.Join(t.TempDir(), "missing.env"))
	if err != nil {
		t.Fatalf("ReadEnvFile on nonexistent file: %v", err)
	}
	if len(fields) != 0 {
		t.Errorf("expected empty map, got %v", fields)
	}
}

func TestUpdateEnvFile_NewFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")

	if err := config.UpdateEnvFile(path, map[string]string{
		"STEAM_USERNAME": "testuser",
		"STEAM_PASSWORD": "s3cr3t",
		"VNC_PASSWORD":   "vncp4ss",
	}); err != nil {
		t.Fatalf("UpdateEnvFile: %v", err)
	}

	fields, err := config.ReadEnvFile(path)
	if err != nil {
		t.Fatalf("ReadEnvFile: %v", err)
	}
	if fields["STEAM_USERNAME"] != "testuser" {
		t.Errorf("STEAM_USERNAME = %q, want %q", fields["STEAM_USERNAME"], "testuser")
	}
	if fields["STEAM_PASSWORD"] == "" {
		t.Error("STEAM_PASSWORD should not be empty after write")
	}
	if fields["VNC_PASSWORD"] == "" {
		t.Error("VNC_PASSWORD should not be empty after write")
	}

	// Verify 0600 permissions (Unix only; Windows does not map Unix mode bits).
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat .env: %v", err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf(".env permissions = %o, want 0600", perm)
		}
	}
}

func TestUpdateEnvFile_UpdatesExistingField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")

	if err := config.UpdateEnvFile(path, map[string]string{
		"STEAM_USERNAME": "olduser",
		"VNC_PASSWORD":   "oldvnc",
	}); err != nil {
		t.Fatalf("initial write: %v", err)
	}

	if err := config.UpdateEnvFile(path, map[string]string{
		"STEAM_USERNAME": "newuser",
	}); err != nil {
		t.Fatalf("update write: %v", err)
	}

	fields, err := config.ReadEnvFile(path)
	if err != nil {
		t.Fatalf("ReadEnvFile: %v", err)
	}
	if fields["STEAM_USERNAME"] != "newuser" {
		t.Errorf("STEAM_USERNAME = %q, want %q", fields["STEAM_USERNAME"], "newuser")
	}
	if fields["VNC_PASSWORD"] != "oldvnc" {
		t.Errorf("VNC_PASSWORD should be preserved, got %q", fields["VNC_PASSWORD"])
	}

	temporaryFiles, err := filepath.Glob(filepath.Join(dir, ".env-*.tmp"))
	if err != nil {
		t.Fatalf("glob temporary env files: %v", err)
	}
	if len(temporaryFiles) != 0 {
		t.Fatalf("atomic env updates should not leave temporary files, got %v", temporaryFiles)
	}
}

func TestUpdateEnvFile_PreservesUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")

	if err := os.WriteFile(path, []byte("CUSTOM_KEY=custom_value\nSTEAM_USERNAME=olduser\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := config.UpdateEnvFile(path, map[string]string{
		"STEAM_USERNAME": "newuser",
	}); err != nil {
		t.Fatalf("UpdateEnvFile: %v", err)
	}

	fields, err := config.ReadEnvFile(path)
	if err != nil {
		t.Fatalf("ReadEnvFile: %v", err)
	}
	if fields["CUSTOM_KEY"] != "custom_value" {
		t.Errorf("CUSTOM_KEY should be preserved, got %q", fields["CUSTOM_KEY"])
	}
	if fields["STEAM_USERNAME"] != "newuser" {
		t.Errorf("STEAM_USERNAME = %q, want %q", fields["STEAM_USERNAME"], "newuser")
	}
}

func TestUpdateEnvFile_NormalizesBOMPrefixedKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	content := "IMAGE_VERSION=old\n\ufeffIMAGE_VERSION=1.5.0-preview.121\nSERVER_IMAGE=sdvd/server:old\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := config.UpdateEnvFile(path, map[string]string{
		"VNC_PASSWORD": "vnc",
	}); err != nil {
		t.Fatalf("UpdateEnvFile: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read .env: %v", err)
	}
	if strings.Contains(string(raw), "\ufeff") {
		t.Fatalf(".env should not preserve BOM-prefixed keys:\n%s", raw)
	}

	fields, err := config.ReadEnvFile(path)
	if err != nil {
		t.Fatalf("ReadEnvFile: %v", err)
	}
	if fields["IMAGE_VERSION"] != "1.5.0-preview.121" {
		t.Errorf("IMAGE_VERSION = %q, want %q", fields["IMAGE_VERSION"], "1.5.0-preview.121")
	}
	if _, ok := fields["\ufeffIMAGE_VERSION"]; ok {
		t.Fatal("BOM-prefixed IMAGE_VERSION key should be normalized")
	}
}

func TestEmptyEnvTemplate_UsesOfficialJunimoKeys(t *testing.T) {
	fields := config.EmptyEnvTemplate()
	for _, key := range []string{
		"IMAGE_VERSION",
		"SERVER_IMAGE",
		"SERVER_IMAGE_CANDIDATES",
		"STEAM_SERVICE_IMAGE",
		"STEAM_SERVICE_IMAGE_CANDIDATES",
		"STEAMCMD_IMAGE",
		"STEAMCMD_IMAGE_CANDIDATES",
		"STEAMCMD_AUTH_COMPLETED",
		"STEAM_AUTH_COMPLETED",
		"STEAM_INVITE_ENABLED",
		"STEAM_INVITE_AUTH_STATE",
		"STEAM_INVITE_RUNTIME_SCOPE_VERSION",
		"STEAM_USERNAME",
		"STEAM_PASSWORD",
		"STEAM_REFRESH_TOKEN",
		"STEAM_KEEP_LANGUAGES",
		"VNC_PASSWORD",
		"GAME_PORT",
		"QUERY_PORT",
		"VNC_PORT",
		"API_PORT",
		"STEAM_AUTH_PORT",
		"SERVER_TPS",
		"SERVER_FPS",
		"SERVER_PASSWORD",
		"SAP_PLAYER_AUTH_MODE",
		"SAP_PLAYER_AUTH_REVISION",
		"SAP_ROLE_AUTH_KEY",
		"SAP_ROLE_PASSWORDS_B64",
		"MAX_LOGIN_ATTEMPTS",
		"AUTH_TIMEOUT_SECONDS",
		"API_ENABLED",
		"API_KEY",
		"ALLOW_INSECURE_SETUP",
	} {
		if _, ok := fields[key]; !ok {
			t.Fatalf("official Junimo env key %s missing", key)
		}
	}
	if _, ok := fields["JUNIMO_IMAGE_TAG"]; ok {
		t.Fatal("JUNIMO_IMAGE_TAG should not be used; Junimo expects IMAGE_VERSION")
	}
	if fields["GAME_PORT"] != "24642" || fields["QUERY_PORT"] != "27015" || fields["API_ENABLED"] != "true" {
		t.Fatalf("unexpected defaults: %#v", fields)
	}
	if !strings.Contains(fields["SERVER_IMAGE_CANDIDATES"], "docker.1ms.run/sdvd/server:1.5.0-preview.125") {
		t.Fatalf("SERVER_IMAGE_CANDIDATES should include the docker.1ms.run mirror, got %q", fields["SERVER_IMAGE_CANDIDATES"])
	}
	if !strings.Contains(fields["STEAM_SERVICE_IMAGE_CANDIDATES"], "ghcr.io/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2") {
		t.Fatalf("STEAM_SERVICE_IMAGE_CANDIDATES should include ghcr.io fallback, got %q", fields["STEAM_SERVICE_IMAGE_CANDIDATES"])
	}
	if !strings.Contains(fields["STEAM_SERVICE_IMAGE_CANDIDATES"], "crpi-9z3bkb9g7fxeohrg.cn-hangzhou.personal.cr.aliyuncs.com/anxi-panel/junimo-steam-service-cn:1.5.0-anxi.2") {
		t.Fatalf("STEAM_SERVICE_IMAGE_CANDIDATES should include Aliyun ACR fallback, got %q", fields["STEAM_SERVICE_IMAGE_CANDIDATES"])
	}
	if fields["STEAM_INVITE_ENABLED"] != "false" || fields["STEAM_INVITE_AUTH_STATE"] != config.SteamInviteAuthStateDisabled {
		t.Fatalf("fresh instances must default Steam invite capability off, got enabled=%q state=%q", fields["STEAM_INVITE_ENABLED"], fields["STEAM_INVITE_AUTH_STATE"])
	}
	if fields["STEAM_INVITE_RUNTIME_SCOPE_VERSION"] != config.SteamInviteRuntimeScopeVersion {
		t.Fatalf("fresh instance runtime scope version = %q, want %q", fields["STEAM_INVITE_RUNTIME_SCOPE_VERSION"], config.SteamInviteRuntimeScopeVersion)
	}
}

func TestSteamInviteRuntimeScopeMarkerPreservesCredentialsAndAuthorizationCaches(t *testing.T) {
	dataDir := t.TempDir()
	envPath := filepath.Join(dataDir, ".env")
	if err := config.UpdateEnvFile(envPath, map[string]string{
		"STEAM_USERNAME":             "saved-user",
		"STEAM_PASSWORD":             "saved-password",
		"STEAMCMD_AUTH_COMPLETED":    "true",
		"STEAM_AUTH_COMPLETED":       "true",
		"STEAM_INVITE_ENABLED":       "true",
		"STEAM_INVITE_AUTH_STATE":    config.SteamInviteAuthStateReady,
		"GAME_DATA_VOLUME":           "preserved-game-data",
		"UNRELATED_CUSTOM_ENV_VALUE": "preserved",
	}); err != nil {
		t.Fatalf("seed legacy env: %v", err)
	}
	if config.SteamInviteRuntimeScopeCurrent(dataDir) {
		t.Fatal("legacy env without marker must require runtime scope convergence")
	}
	if err := config.MarkSteamInviteRuntimeScopeCurrent(dataDir); err != nil {
		t.Fatalf("mark runtime scope: %v", err)
	}
	if !config.SteamInviteRuntimeScopeCurrent(dataDir) {
		t.Fatal("runtime scope marker was not persisted")
	}
	fields, err := config.ReadEnvFile(envPath)
	if err != nil {
		t.Fatalf("read marked env: %v", err)
	}
	for key, want := range map[string]string{
		"STEAM_USERNAME":             "saved-user",
		"STEAM_PASSWORD":             "saved-password",
		"STEAMCMD_AUTH_COMPLETED":    "true",
		"STEAM_AUTH_COMPLETED":       "true",
		"STEAM_INVITE_ENABLED":       "true",
		"STEAM_INVITE_AUTH_STATE":    config.SteamInviteAuthStateReady,
		"GAME_DATA_VOLUME":           "preserved-game-data",
		"UNRELATED_CUSTOM_ENV_VALUE": "preserved",
	} {
		if fields[key] != want {
			t.Fatalf("marker changed %s: got %q want %q", key, fields[key], want)
		}
	}
}

func TestSteamInviteEnabledLegacyCompatibilityAndExplicitIntent(t *testing.T) {
	dataDir := t.TempDir()
	envPath := filepath.Join(dataDir, ".env")
	if err := config.UpdateEnvFile(envPath, map[string]string{
		"STEAM_AUTH_COMPLETED":    "true",
		"STEAMCMD_AUTH_COMPLETED": "true",
	}); err != nil {
		t.Fatalf("write legacy env: %v", err)
	}
	if !config.SteamInviteEnabled(dataDir) {
		t.Fatal("legacy instance with completed steam-auth must remain enabled")
	}
	if got := config.SteamInviteAuthState(dataDir); got != config.SteamInviteAuthStateReady {
		t.Fatalf("legacy authorized state = %q, want ready", got)
	}

	if err := config.SetSteamInviteEnabled(dataDir, false); err != nil {
		t.Fatalf("disable invite: %v", err)
	}
	if config.SteamInviteEnabled(dataDir) {
		t.Fatal("explicit disabled intent must win over legacy completed auth")
	}
	fields, err := config.ReadEnvFile(envPath)
	if err != nil {
		t.Fatalf("read disabled env: %v", err)
	}
	if fields["STEAMCMD_AUTH_COMPLETED"] != "true" {
		t.Fatal("changing invite intent must preserve SteamCMD authorization cache")
	}
}

func TestSteamInviteEnabledStrictRejectsMissingAndMalformedIntent(t *testing.T) {
	dataDir := t.TempDir()
	if _, err := config.SteamInviteEnabledStrict(dataDir); err == nil {
		t.Fatal("strict Steam invite intent accepted a missing .env")
	}
	envPath := filepath.Join(dataDir, ".env")
	if err := config.UpdateEnvFile(envPath, map[string]string{"STEAM_INVITE_ENABLED": "maybe"}); err != nil {
		t.Fatal(err)
	}
	if _, err := config.SteamInviteEnabledStrict(dataDir); err == nil {
		t.Fatal("strict Steam invite intent accepted a malformed explicit value")
	}
	if config.SteamInviteEnabled(dataDir) {
		t.Fatal("compatibility helper must fail closed on malformed intent")
	}
}

func TestSteamAuthStateChangesPreserveSteamCMDCacheAndInviteIntent(t *testing.T) {
	dataDir := t.TempDir()
	envPath := filepath.Join(dataDir, ".env")
	if err := config.UpdateEnvFile(envPath, map[string]string{
		"STEAMCMD_AUTH_COMPLETED": "true",
	}); err != nil {
		t.Fatalf("seed SteamCMD cache: %v", err)
	}
	if err := config.SetSteamInviteEnabled(dataDir, true); err != nil {
		t.Fatalf("enable invite: %v", err)
	}
	if got := config.SteamInviteAuthState(dataDir); got != config.SteamInviteAuthStatePending {
		t.Fatalf("newly enabled state = %q, want pending", got)
	}
	if err := config.SetSteamInviteAuthState(dataDir, config.SteamInviteAuthStateAuthorizing); err != nil {
		t.Fatalf("mark authorizing: %v", err)
	}
	if got := config.SteamInviteAuthState(dataDir); got != config.SteamInviteAuthStateAuthorizing {
		t.Fatalf("authorizing state = %q", got)
	}
	if err := config.SetSteamAuthLoggedIn(dataDir, true); err != nil {
		t.Fatalf("mark steam-auth logged in: %v", err)
	}
	if !config.SteamInviteEnabled(dataDir) || config.SteamInviteAuthState(dataDir) != config.SteamInviteAuthStateReady {
		t.Fatal("successful steam-auth login must leave invite enabled and ready")
	}
	if err := config.SetSteamAuthCompletedState(dataDir, config.SteamInviteAuthStateCleanupPending); err != nil {
		t.Fatalf("atomically mark successful session cleanup pending: %v", err)
	}
	if got := config.SteamInviteAuthState(dataDir); got != config.SteamInviteAuthStateCleanupPending {
		t.Fatalf("successful session with an unresolved holder = %q, want cleanup_pending", got)
	}
	completedFields, err := config.ReadEnvFile(envPath)
	if err != nil {
		t.Fatalf("read completed cleanup-pending env: %v", err)
	}
	if completedFields["STEAM_AUTH_COMPLETED"] != "true" || completedFields["STEAM_INVITE_ENABLED"] != "true" || completedFields["STEAM_INVITE_AUTH_STATE"] != config.SteamInviteAuthStateCleanupPending {
		t.Fatalf("successful session and holder state were not persisted together: %#v", completedFields)
	}
	if err := config.SetSteamAuthLoggedIn(dataDir, false); err != nil {
		t.Fatalf("clear steam-auth session: %v", err)
	}
	if !config.SteamInviteEnabled(dataDir) || config.SteamInviteAuthState(dataDir) != config.SteamInviteAuthStateFailed {
		t.Fatal("steam-auth session failure must preserve enabled intent and become failed")
	}
	fields, err := config.ReadEnvFile(envPath)
	if err != nil {
		t.Fatalf("read final env: %v", err)
	}
	if fields["STEAMCMD_AUTH_COMPLETED"] != "true" {
		t.Fatal("steam-auth session changes must not clear SteamCMD authorization cache")
	}
}

func TestEnsureSteamInviteIntentMigratesHistoricalEvidenceOnlyWhenKeyMissing(t *testing.T) {
	for _, tc := range []struct {
		name           string
		env            map[string]string
		strongEvidence bool
		wantEnabled    bool
		wantAuthState  string
		wantChanged    bool
	}{
		{name: "historical driver authorization", env: map[string]string{"STEAMCMD_AUTH_COMPLETED": "true", "STEAM_USERNAME": "legacy-user"}, strongEvidence: true, wantEnabled: true, wantAuthState: config.SteamInviteAuthStateReady, wantChanged: true},
		{name: "legacy auth flag", env: map[string]string{"STEAM_AUTH_COMPLETED": "true"}, wantEnabled: true, wantAuthState: config.SteamInviteAuthStateReady, wantChanged: true},
		{name: "SteamCMD completion is not invite evidence", env: map[string]string{"STEAMCMD_AUTH_COMPLETED": "true"}, wantEnabled: false, wantAuthState: config.SteamInviteAuthStateDisabled, wantChanged: true},
		{name: "no evidence", env: map[string]string{}, wantEnabled: false, wantAuthState: config.SteamInviteAuthStateDisabled, wantChanged: true},
		{name: "explicit false wins", env: map[string]string{"STEAM_INVITE_ENABLED": "false", "STEAM_AUTH_COMPLETED": "true"}, strongEvidence: true, wantEnabled: false, wantAuthState: config.SteamInviteAuthStateDisabled, wantChanged: false},
		{name: "explicit true without authorization stays pending", env: map[string]string{"STEAM_INVITE_ENABLED": "true"}, wantEnabled: true, wantAuthState: config.SteamInviteAuthStatePending, wantChanged: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dataDir := t.TempDir()
			if err := config.UpdateEnvFile(filepath.Join(dataDir, ".env"), tc.env); err != nil {
				t.Fatalf("seed env: %v", err)
			}
			changed, err := config.EnsureSteamInviteIntent(dataDir, tc.strongEvidence)
			if err != nil {
				t.Fatalf("EnsureSteamInviteIntent: %v", err)
			}
			if changed != tc.wantChanged {
				t.Fatalf("changed = %v, want %v", changed, tc.wantChanged)
			}
			if got := config.SteamInviteEnabled(dataDir); got != tc.wantEnabled {
				t.Fatalf("enabled = %v, want %v", got, tc.wantEnabled)
			}
			if got := config.SteamInviteAuthState(dataDir); got != tc.wantAuthState {
				t.Fatalf("auth state = %q, want %q", got, tc.wantAuthState)
			}
			fields, err := config.ReadEnvFile(filepath.Join(dataDir, ".env"))
			if err != nil {
				t.Fatal(err)
			}
			if tc.strongEvidence && !config.SteamAuthLoggedIn(dataDir) {
				t.Fatal("strong historical evidence must restore ready authorization")
			}
			if tc.env["STEAMCMD_AUTH_COMPLETED"] != "" && fields["STEAMCMD_AUTH_COMPLETED"] != tc.env["STEAMCMD_AUTH_COMPLETED"] {
				t.Fatal("migration changed the SteamCMD authorization cache")
			}
			if tc.env["STEAM_USERNAME"] != "" && fields["STEAM_USERNAME"] != tc.env["STEAM_USERNAME"] {
				t.Fatal("migration changed the saved Steam account")
			}
		})
	}
}
