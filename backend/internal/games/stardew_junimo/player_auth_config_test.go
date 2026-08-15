package stardew_junimo

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
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

func TestPlayerAuthPasswordRejectsControlCharacters(t *testing.T) {
	for _, password := range []string{"line\nbreak", "carriage\rreturn", "nul\x00byte", "tab\tvalue"} {
		if validPlayerAuthPassword(password) {
			t.Fatalf("control-character password was accepted: %q", password)
		}
	}
	if !validPlayerAuthPassword("可用 密码 🔐") {
		t.Fatal("ordinary Unicode password was rejected")
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
	record := state.Payload.Roles["2"]
	if !verifyRolePassword(state.RoleKey, "2", password, record.Verifier) {
		t.Fatal("stored role verifier rejected its password")
	}
	_, err = driver.UpdatePlayerAuthConfig(context.Background(), instance, UpdatePlayerAuthConfigRequest{
		ExpectedRevision: current.Revision,
		Mode:             PlayerAuthModeNone,
	})
	if err == nil || err.(*CommandError).Code != "player_auth_revision_conflict" {
		t.Fatalf("stale revision was not rejected: %v", err)
	}
}

func TestRoleModeRequiresEveryCurrentRolePassword(t *testing.T) {
	roles := &PlayersResult{Players: []PlayerInfo{
		{Name: "Host", UniqueMultiplayerID: "1", IsHost: true},
		{Name: "Leah", UniqueMultiplayerID: "2"},
		{Name: "Sam", UniqueMultiplayerID: "3"},
	}}
	eligible := eligiblePlayerAuthRoles(roles)
	if len(eligible) != 2 || eligible[0].Name != "Leah" || eligible[1].Name != "Sam" {
		t.Fatalf("eligible roles = %+v", eligible)
	}
}
