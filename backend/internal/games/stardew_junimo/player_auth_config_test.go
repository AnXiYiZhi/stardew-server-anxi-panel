package stardew_junimo

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func TestPlayerAuthLegacyModeInference(t *testing.T) {
	dataDir := t.TempDir()
	if err := sjconfig.UpdateEnvFile(filepath.Join(dataDir, ".env"), map[string]string{"SERVER_PASSWORD": ""}); err != nil {
		t.Fatalf("write empty legacy env: %v", err)
	}
	state, err := readPlayerAuthEnvState(dataDir)
	if err != nil {
		t.Fatalf("read empty legacy env: %v", err)
	}
	if state.Mode != PlayerAuthModeNone || state.Revision != "legacy-none" {
		t.Fatalf("empty legacy state = %+v", state)
	}
	if err := sjconfig.UpdateEnvFile(filepath.Join(dataDir, ".env"), map[string]string{"SERVER_PASSWORD": "legacy-secret"}); err != nil {
		t.Fatalf("write protected legacy env: %v", err)
	}
	state, err = readPlayerAuthEnvState(dataDir)
	if err != nil {
		t.Fatalf("read protected legacy env: %v", err)
	}
	if state.Mode != PlayerAuthModeGlobal || state.Revision != "legacy-global" || state.ServerPassword != "legacy-secret" {
		t.Fatalf("protected legacy state = %+v", state)
	}
}

func TestPlayerAuthPolicySettingsPersistIdempotentlyAndRequireRestart(t *testing.T) {
	dataDir := t.TempDir()
	driver := newTestDriver(&fakeConsoleDocker{})
	instance := makeStoppedInstance()
	instance.DataDir = dataDir

	current, err := driver.GetPlayerAuthConfig(context.Background(), instance)
	if err != nil {
		t.Fatalf("get initial policy: %v", err)
	}
	if current.TimeoutSeconds != defaultAuthTimeoutSeconds || current.MaxAttempts != defaultMaxLoginAttempts {
		t.Fatalf("initial policy = %+v", current)
	}

	timeoutSeconds := 180
	maxAttempts := 5
	updated, err := driver.UpdatePlayerAuthConfig(context.Background(), instance, UpdatePlayerAuthConfigRequest{
		ExpectedRevision: current.Revision,
		Mode:             PlayerAuthModeNone,
		TimeoutSeconds:   &timeoutSeconds,
		MaxAttempts:      &maxAttempts,
	})
	if err != nil {
		t.Fatalf("update policy: %v", err)
	}
	if updated.Revision == current.Revision || updated.TimeoutSeconds != timeoutSeconds || updated.MaxAttempts != maxAttempts {
		t.Fatalf("updated policy = %+v", updated)
	}
	values, err := sjconfig.ReadEnvFile(instanceEnvPath(dataDir))
	if err != nil {
		t.Fatalf("read updated policy env: %v", err)
	}
	if values["AUTH_TIMEOUT_SECONDS"] != "180" || values["MAX_LOGIN_ATTEMPTS"] != "5" {
		t.Fatalf("policy env = %#v", values)
	}

	idempotent, err := driver.UpdatePlayerAuthConfig(context.Background(), instance, UpdatePlayerAuthConfigRequest{
		ExpectedRevision: updated.Revision,
		Mode:             PlayerAuthModeNone,
		TimeoutSeconds:   &timeoutSeconds,
		MaxAttempts:      &maxAttempts,
	})
	if err != nil {
		t.Fatalf("repeat policy update: %v", err)
	}
	if idempotent.Revision != updated.Revision {
		t.Fatalf("idempotent policy update changed revision: before=%q after=%q", updated.Revision, idempotent.Revision)
	}

	state, err := readPlayerAuthEnvState(dataDir)
	if err != nil {
		t.Fatalf("read policy state: %v", err)
	}
	running := instance
	running.State = storage.InstanceStateRunning
	runningResult := buildPlayerAuthConfigResult(running, state, &PlayersResult{})
	if !runningResult.RestartRequired {
		t.Fatalf("running policy change did not require restart: %+v", runningResult)
	}

	beforeInvalid, err := os.ReadFile(instanceEnvPath(dataDir))
	if err != nil {
		t.Fatalf("read env before invalid policy: %v", err)
	}
	invalidTimeout := -1
	_, err = driver.UpdatePlayerAuthConfig(context.Background(), instance, UpdatePlayerAuthConfigRequest{
		ExpectedRevision: idempotent.Revision,
		Mode:             PlayerAuthModeNone,
		TimeoutSeconds:   &invalidTimeout,
	})
	if commandErr, ok := err.(*CommandError); !ok || commandErr.Code != "invalid_auth_timeout" {
		t.Fatalf("invalid timeout error = %v", err)
	}
	invalidAttempts := maxLoginAttempts + 1
	_, err = driver.UpdatePlayerAuthConfig(context.Background(), instance, UpdatePlayerAuthConfigRequest{
		ExpectedRevision: idempotent.Revision,
		Mode:             PlayerAuthModeNone,
		MaxAttempts:      &invalidAttempts,
	})
	if commandErr, ok := err.(*CommandError); !ok || commandErr.Code != "invalid_max_login_attempts" {
		t.Fatalf("invalid max attempts error = %v", err)
	}
	afterInvalid, err := os.ReadFile(instanceEnvPath(dataDir))
	if err != nil {
		t.Fatalf("read env after invalid policy: %v", err)
	}
	if string(afterInvalid) != string(beforeInvalid) {
		t.Fatal("invalid policy request changed .env")
	}

	globalPassword := "preserve-policy"
	preserved, err := driver.UpdatePlayerAuthConfig(context.Background(), instance, UpdatePlayerAuthConfigRequest{
		ExpectedRevision: idempotent.Revision,
		Mode:             PlayerAuthModeGlobal,
		GlobalPassword:   &globalPassword,
	})
	if err != nil {
		t.Fatalf("update mode without policy fields: %v", err)
	}
	if preserved.TimeoutSeconds != timeoutSeconds || preserved.MaxAttempts != maxAttempts {
		t.Fatalf("omitted policy fields were reset: %+v", preserved)
	}
}

func TestRolePasswordVerifierIsRoleScopedAndDoesNotStorePlaintext(t *testing.T) {
	key := make([]byte, roleAuthKeyBytes)
	for index := range key {
		key[index] = byte(index + 1)
	}
	verifier := computeRolePasswordVerifier(key, "2", "role-secret")
	if strings.Contains(verifier, "role-secret") || !verifyRolePassword(key, "2", "role-secret", verifier) {
		t.Fatalf("valid role verifier was rejected or leaked plaintext: %q", verifier)
	}
	if verifyRolePassword(key, "3", "role-secret", verifier) || verifyRolePassword(key, "2", "wrong", verifier) {
		t.Fatal("role verifier accepted another role or password")
	}
	guard := deriveInternalServerPassword(key)
	if guard == "role-secret" || !strings.HasPrefix(guard, "sap_") {
		t.Fatalf("unexpected internal guard %q", guard)
	}
	malformedPayload, err := encodeRolePasswordPayload(rolePasswordPayload{Roles: map[string]rolePasswordRecord{
		"2": {Verifier: "bad"},
	}})
	if err != nil {
		t.Fatalf("encode malformed payload fixture: %v", err)
	}
	if _, err := decodeRolePasswordPayload(malformedPayload); err == nil {
		t.Fatal("malformed role verifier was accepted")
	}
}

func TestPlayerAuthPasswordMatchesJunimoChatRoundTrip(t *testing.T) {
	for _, password := range []string{
		"line\nbreak",
		"carriage\rreturn",
		"nul\x00byte",
		"tab\tvalue",
		" leading",
		"trailing ",
		"two  spaces",
		" ",
	} {
		if validPlayerAuthPassword(password) {
			t.Fatalf("password that cannot round-trip through Junimo chat was accepted: %q", password)
		}
	}
	for _, password := range []string{"x", "可用 密码 🔐", "内部 单个 空格", "不间断\u00a0空格"} {
		if !validPlayerAuthPassword(password) {
			t.Fatalf("chat-representable password was rejected: %q", password)
		}
	}
}

func TestStagedRoleKeyKeepsOldAuthModeAndCanRestoreExactEnv(t *testing.T) {
	dataDir := t.TempDir()
	envPath := filepath.Join(dataDir, ".env")
	if err := sjconfig.UpdateEnvFile(envPath, map[string]string{
		"SERVER_PASSWORD":       "old global password",
		"CUSTOM_STAGE_SENTINEL": "preserve",
	}); err != nil {
		t.Fatalf("write original env: %v", err)
	}
	snapshot, err := capturePlayerAuthEnvFile(dataDir)
	if err != nil {
		t.Fatalf("capture original env: %v", err)
	}
	key := make([]byte, roleAuthKeyBytes)
	for index := range key {
		key[index] = byte(index + 1)
	}
	if err := stagePlayerAuthRoleKey(dataDir, snapshot.Exists, key); err != nil {
		t.Fatalf("stage role key: %v", err)
	}
	staged, err := readPlayerAuthEnvState(dataDir)
	if err != nil {
		t.Fatalf("read staged env: %v", err)
	}
	if staged.Mode != PlayerAuthModeGlobal || staged.Revision != "legacy-global" || staged.ServerPassword != "old global password" {
		t.Fatalf("staged key changed the active authentication contract: %+v", staged)
	}
	if encodeRoleAuthKey(staged.RoleKey) != encodeRoleAuthKey(key) {
		t.Fatal("staged role key was not persisted")
	}
	if err := mutateRoleCredentialStore(dataDir, "Farm_100", rolePasswordPayload{}, func(records map[string]rolePasswordRecord) error {
		records["2"] = rolePasswordRecord{Verifier: computeRolePasswordVerifier(key, "2", "new password")}
		return nil
	}); err != nil {
		t.Fatalf("write credential after staged key: %v", err)
	}
	store, err := readRoleCredentialStore(dataDir)
	if err != nil || !verifyRolePassword(key, "2", "new password", store.Store.Saves["Farm_100"].Roles["2"].Verifier) {
		t.Fatalf("staged key and credential store were inconsistent: store=%+v err=%v", store, err)
	}
	if err := restorePlayerAuthEnvFile(dataDir, snapshot); err != nil {
		t.Fatalf("restore original env: %v", err)
	}
	restored, err := os.ReadFile(envPath)
	if err != nil || string(restored) != string(snapshot.Raw) {
		t.Fatalf("env was not restored exactly: %q err=%v", restored, err)
	}
}

func TestStagedRoleKeyForAbsentEnvRemainsClosedAndRollbackRemovesFile(t *testing.T) {
	dataDir := t.TempDir()
	snapshot, err := capturePlayerAuthEnvFile(dataDir)
	if err != nil || snapshot.Exists {
		t.Fatalf("absent env snapshot = %+v err=%v", snapshot, err)
	}
	key := make([]byte, roleAuthKeyBytes)
	if err := stagePlayerAuthRoleKey(dataDir, false, key); err != nil {
		t.Fatalf("stage role key for absent env: %v", err)
	}
	staged, err := readPlayerAuthEnvState(dataDir)
	if err != nil || staged.Mode != PlayerAuthModeNone || staged.Revision != "legacy-none" || len(staged.RoleKey) != roleAuthKeyBytes {
		t.Fatalf("absent env staging opened authentication: %+v err=%v", staged, err)
	}
	if err := restorePlayerAuthEnvFile(dataDir, snapshot); err != nil {
		t.Fatalf("restore absent env: %v", err)
	}
	if _, err := os.Stat(instanceEnvPath(dataDir)); !os.IsNotExist(err) {
		t.Fatalf("absent env rollback left a file: %v", err)
	}
}

func TestStagedRoleKeyAndStoreRetryConvergesWithOriginalRevision(t *testing.T) {
	dataDir := t.TempDir()
	if err := sjconfig.UpdateEnvFile(instanceEnvPath(dataDir), map[string]string{"SERVER_PASSWORD": "old-global"}); err != nil {
		t.Fatalf("write original global env: %v", err)
	}
	control := controlDir(dataDir)
	if err := os.MkdirAll(control, 0o700); err != nil {
		t.Fatalf("mkdir control: %v", err)
	}
	if err := os.WriteFile(filepath.Join(control, "status.json"), []byte(`{"saveId":"Farm_100"}`), 0o600); err != nil {
		t.Fatalf("write runtime status: %v", err)
	}
	if err := os.WriteFile(filepath.Join(control, "players-cache.json"), []byte(`{
  "saveId":"Farm_100",
  "updatedAt":"2026-08-16T00:00:00Z",
  "players":[
    {"name":"Host","uniqueMultiplayerId":"1","isHost":true,"status":"offline"},
    {"name":"Leah","uniqueMultiplayerId":"2","status":"offline"}
  ]
}`), 0o600); err != nil {
		t.Fatalf("write players cache: %v", err)
	}
	key := make([]byte, roleAuthKeyBytes)
	for index := range key {
		key[index] = byte(index + 1)
	}
	if err := stagePlayerAuthRoleKey(dataDir, true, key); err != nil {
		t.Fatalf("stage role key: %v", err)
	}
	password := "retry password"
	if err := mutateRoleCredentialStore(dataDir, "Farm_100", rolePasswordPayload{}, func(records map[string]rolePasswordRecord) error {
		records["2"] = rolePasswordRecord{Verifier: computeRolePasswordVerifier(key, "2", password)}
		return nil
	}); err != nil {
		t.Fatalf("write interrupted transaction store: %v", err)
	}
	driver := newTestDriver(&fakeConsoleDocker{})
	instance := makeStoppedInstance()
	instance.DataDir = dataDir
	current, err := driver.GetPlayerAuthConfig(context.Background(), instance)
	if err != nil {
		t.Fatalf("read interrupted transaction: %v", err)
	}
	if current.Mode != PlayerAuthModeGlobal || current.Revision != "legacy-global" || current.ConfiguredRoleCount != 1 {
		t.Fatalf("interrupted transaction did not retain old mode/revision with matching credentials: %+v", current)
	}
	updated, err := driver.UpdatePlayerAuthConfig(context.Background(), instance, UpdatePlayerAuthConfigRequest{
		ExpectedRevision: current.Revision,
		Mode:             PlayerAuthModeRole,
		RolePasswordUpdates: []PlayerAuthPasswordUpdate{
			{RoleID: "2", Password: password},
		},
	})
	if err != nil {
		t.Fatalf("retry interrupted transaction: %v", err)
	}
	if updated.Mode != PlayerAuthModeRole || updated.Revision == current.Revision || updated.ConfiguredRoleCount != 1 {
		t.Fatalf("interrupted transaction did not converge: %+v", updated)
	}
}

func TestUpdatePlayerAuthConfigCreatesFailClosedRoleMode(t *testing.T) {
	dataDir := t.TempDir()
	control := controlDir(dataDir)
	if err := os.MkdirAll(control, 0o700); err != nil {
		t.Fatalf("mkdir control: %v", err)
	}
	if err := os.WriteFile(filepath.Join(control, "status.json"), []byte(`{"saveId":"Farm"}`), 0o600); err != nil {
		t.Fatalf("write status: %v", err)
	}
	if err := os.WriteFile(filepath.Join(control, "players-cache.json"), []byte(`{
  "saveId":"Farm",
  "updatedAt":"2026-08-15T12:00:00Z",
  "players":[
    {"name":"Host","uniqueMultiplayerId":"1","isHost":true,"status":"offline"},
    {"name":"Leah","uniqueMultiplayerId":"2","status":"offline"}
  ]
}`), 0o600); err != nil {
		t.Fatalf("write player cache: %v", err)
	}
	driver := newTestDriver(&fakeConsoleDocker{})
	instance := makeStoppedInstance()
	instance.DataDir = dataDir

	current, err := driver.GetPlayerAuthConfig(context.Background(), instance)
	if err != nil {
		t.Fatalf("get initial player auth config: %v", err)
	}
	if current.Mode != PlayerAuthModeNone || len(current.Roles) != 1 || current.Roles[0].RoleID != "2" {
		t.Fatalf("initial player auth config = %+v", current)
	}
	password := "role-secret"
	updated, err := driver.UpdatePlayerAuthConfig(context.Background(), instance, UpdatePlayerAuthConfigRequest{
		ExpectedRevision: current.Revision,
		Mode:             PlayerAuthModeRole,
		RolePasswordUpdates: []PlayerAuthPasswordUpdate{
			{RoleID: "2", Password: password},
		},
	})
	if err != nil {
		t.Fatalf("enable role auth: %v", err)
	}
	if updated.Mode != PlayerAuthModeRole || updated.ConfiguredRoleCount != 1 || updated.UnconfiguredRoleCount != 0 {
		t.Fatalf("updated player auth config = %+v", updated)
	}
	values, err := sjconfig.ReadEnvFile(filepath.Join(dataDir, ".env"))
	if err != nil {
		t.Fatalf("read updated env: %v", err)
	}
	if values[playerAuthModeEnvKey] != PlayerAuthModeRole || values["SERVER_PASSWORD"] == "" || values["SERVER_PASSWORD"] == password {
		t.Fatalf("role mode did not install an internal guard: %#v", values)
	}
	if values["API_ENABLED"] != "true" || values["GAME_PORT"] != "24642" {
		t.Fatalf("first player auth write did not preserve the complete default env template: %#v", values)
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(values[rolePasswordsEnvKey])
	if err != nil {
		t.Fatalf("decode stored role payload: %v", err)
	}
	if strings.Contains(string(payloadBytes), password) || strings.Contains(values[rolePasswordsEnvKey], password) {
		t.Fatal("stored role password payload leaked plaintext")
	}
	state, err := readPlayerAuthEnvState(dataDir)
	if err != nil {
		t.Fatalf("read role auth state: %v", err)
	}
	if len(state.Payload.Roles) != 0 {
		t.Fatalf("new role verifier unexpectedly remained in legacy env payload: %+v", state.Payload.Roles)
	}
	store, err := readRoleCredentialStore(dataDir)
	if err != nil || !store.Exists {
		t.Fatalf("read runtime role credential store: exists=%v err=%v", store.Exists, err)
	}
	record := store.Store.Saves["Farm"].Roles["2"]
	if !verifyRolePassword(state.RoleKey, "2", password, record.Verifier) {
		t.Fatal("stored role verifier rejected its password")
	}
	cleared, err := driver.UpdatePlayerAuthConfig(context.Background(), instance, UpdatePlayerAuthConfigRequest{
		ExpectedRevision:     updated.Revision,
		Mode:                 PlayerAuthModeRole,
		RolePasswordRemovals: []string{"2"},
	})
	if err != nil {
		t.Fatalf("clear role credential: %v", err)
	}
	if cleared.Revision != updated.Revision || cleared.ConfiguredRoleCount != 0 || cleared.UnconfiguredRoleCount != 1 || cleared.Roles[0].CredentialStatus != RoleCredentialStatusWaiting {
		t.Fatalf("cleared role credential state = %+v", cleared)
	}
	_, err = driver.UpdatePlayerAuthConfig(context.Background(), instance, UpdatePlayerAuthConfigRequest{
		ExpectedRevision: current.Revision,
		Mode:             PlayerAuthModeNone,
	})
	if err == nil || err.(*CommandError).Code != "player_auth_revision_conflict" {
		t.Fatalf("stale revision was not rejected: %v", err)
	}
}

func TestRoleModeAllowsEveryCurrentRoleToWaitForFirstLogin(t *testing.T) {
	roles := &PlayersResult{Players: []PlayerInfo{
		{Name: "Host", UniqueMultiplayerID: "1", IsHost: true},
		{Name: "Leah", UniqueMultiplayerID: "-7928143696348358209"},
		{Name: "Sam", UniqueMultiplayerID: "3"},
		{Name: "Zero", UniqueMultiplayerID: "0"},
		{Name: "Plus", UniqueMultiplayerID: "+4"},
	}}
	eligible := eligiblePlayerAuthRoles(roles)
	if len(eligible) != 2 || eligible[0].RoleID != "-7928143696348358209" || eligible[0].Name != "Leah" || eligible[1].Name != "Sam" {
		t.Fatalf("eligible roles = %+v", eligible)
	}
}

func TestUpdatePlayerAuthConfigSupportsNegativeOldSaveRoleID(t *testing.T) {
	dataDir := t.TempDir()
	control := controlDir(dataDir)
	if err := os.MkdirAll(control, 0o700); err != nil {
		t.Fatalf("mkdir control: %v", err)
	}
	if err := os.WriteFile(filepath.Join(control, "status.json"), []byte(`{"saveId":"OldFarm_100"}`), 0o600); err != nil {
		t.Fatalf("write status: %v", err)
	}
	const roleID = "-7928143696348358209"
	if err := os.WriteFile(filepath.Join(control, "players-cache.json"), []byte(`{
  "saveId":"OldFarm_100",
  "updatedAt":"2026-08-16T00:00:00Z",
  "players":[
    {"name":"Host","uniqueMultiplayerId":"1","isHost":true,"status":"offline"},
    {"name":"Old Farmhand","uniqueMultiplayerId":"-7928143696348358209","status":"offline"}
  ]
}`), 0o600); err != nil {
		t.Fatalf("write players cache: %v", err)
	}
	driver := newTestDriver(&fakeConsoleDocker{})
	instance := makeStoppedInstance()
	instance.DataDir = dataDir
	current, err := driver.GetPlayerAuthConfig(context.Background(), instance)
	if err != nil || len(current.Roles) != 1 || current.Roles[0].RoleID != roleID {
		t.Fatalf("negative old-save role was not listed: config=%+v err=%v", current, err)
	}
	updated, err := driver.UpdatePlayerAuthConfig(context.Background(), instance, UpdatePlayerAuthConfigRequest{
		ExpectedRevision: current.Revision,
		Mode:             PlayerAuthModeRole,
		RolePasswordUpdates: []PlayerAuthPasswordUpdate{
			{RoleID: roleID, Password: "old-save-secret"},
		},
	})
	if err != nil || updated.ConfiguredRoleCount != 1 || updated.Roles[0].CredentialStatus != RoleCredentialStatusConfigured {
		t.Fatalf("configure negative old-save role: config=%+v err=%v", updated, err)
	}
	state, err := readPlayerAuthEnvState(dataDir)
	if err != nil {
		t.Fatalf("read player auth state: %v", err)
	}
	store, err := readRoleCredentialStore(dataDir)
	if err != nil {
		t.Fatalf("read negative old-save role credential: %v", err)
	}
	record := store.Store.Saves["OldFarm_100"].Roles[roleID]
	if !verifyRolePassword(state.RoleKey, roleID, "old-save-secret", record.Verifier) {
		t.Fatalf("negative old-save role credential was not persisted: store=%+v", store.Store)
	}
}
