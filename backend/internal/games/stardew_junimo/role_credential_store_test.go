package stardew_junimo

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
)

func TestRoleCredentialStoreSerializesConcurrentMutationsAndSeparatesSaves(t *testing.T) {
	dataDir := t.TempDir()
	key := make([]byte, roleAuthKeyBytes)
	for index := range key {
		key[index] = byte(index + 1)
	}

	var start sync.WaitGroup
	start.Add(1)
	var workers sync.WaitGroup
	errors := make(chan error, 2)
	for roleID, password := range map[string]string{"2": "alpha", "3": "beta"} {
		roleID, password := roleID, password
		workers.Add(1)
		go func() {
			defer workers.Done()
			start.Wait()
			errors <- mutateRoleCredentialStore(dataDir, "Farm_100", rolePasswordPayload{}, func(records map[string]rolePasswordRecord) error {
				records[roleID] = rolePasswordRecord{Verifier: computeRolePasswordVerifier(key, roleID, password)}
				return nil
			})
		}()
	}
	start.Done()
	workers.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatalf("concurrent role credential mutation: %v", err)
		}
	}

	if err := mutateRoleCredentialStore(dataDir, "OtherFarm_200", rolePasswordPayload{}, func(records map[string]rolePasswordRecord) error {
		records["2"] = rolePasswordRecord{Verifier: computeRolePasswordVerifier(key, "2", "other")}
		return nil
	}); err != nil {
		t.Fatalf("write second save credential: %v", err)
	}
	snapshot, err := readRoleCredentialStore(dataDir)
	if err != nil {
		t.Fatalf("read role credential store: %v", err)
	}
	if len(snapshot.Store.Saves["Farm_100"].Roles) != 2 || len(snapshot.Store.Saves["OtherFarm_200"].Roles) != 1 {
		t.Fatalf("save-scoped records = %+v", snapshot.Store.Saves)
	}
	if verifyRolePassword(key, "2", "other", snapshot.Store.Saves["Farm_100"].Roles["2"].Verifier) {
		t.Fatal("a role credential from another save authenticated in the active save")
	}
	if _, err := os.Stat(filepath.Join(controlDir(dataDir), roleCredentialStoreLockFileName)); !os.IsNotExist(err) {
		t.Fatalf("role credential lock was not released: %v", err)
	}
	if err := os.Remove(roleCredentialStorePath(dataDir)); err != nil {
		t.Fatalf("remove initialized credential store fixture: %v", err)
	}
	if _, err := readRoleCredentialStore(dataDir); err == nil {
		t.Fatal("a missing initialized credential store was treated as unconfigured")
	}
}

func TestRoleCredentialStoreMigratesLegacyPayloadWithoutLosingOtherRoles(t *testing.T) {
	dataDir := t.TempDir()
	key := make([]byte, roleAuthKeyBytes)
	for index := range key {
		key[index] = byte(index + 1)
	}
	legacy := rolePasswordPayload{
		SchemaVersion: rolePasswordSchemaVersion,
		Roles: map[string]rolePasswordRecord{
			"2": {Name: "Leah", Verifier: computeRolePasswordVerifier(key, "2", "old-leah")},
			"3": {Name: "Sam", Verifier: computeRolePasswordVerifier(key, "3", "sam-secret")},
		},
	}
	records, usedLegacy, err := roleCredentialsForSave(dataDir, "Farm_100", legacy)
	if err != nil || !usedLegacy || len(records) != 2 {
		t.Fatalf("legacy fallback = records:%+v used:%v err:%v", records, usedLegacy, err)
	}
	if err := mutateRoleCredentialStore(dataDir, "Farm_100", legacy, func(records map[string]rolePasswordRecord) error {
		records["2"] = rolePasswordRecord{Name: "Leah", Verifier: computeRolePasswordVerifier(key, "2", "new-leah")}
		return nil
	}); err != nil {
		t.Fatalf("migrate legacy role credentials: %v", err)
	}
	snapshot, err := readRoleCredentialStore(dataDir)
	if err != nil || !snapshot.Exists {
		t.Fatalf("read migrated store: exists=%v err=%v", snapshot.Exists, err)
	}
	migrated := snapshot.Store.Saves["Farm_100"].Roles
	if !verifyRolePassword(key, "2", "new-leah", migrated["2"].Verifier) ||
		!verifyRolePassword(key, "3", "sam-secret", migrated["3"].Verifier) {
		t.Fatalf("legacy migration lost or changed a credential: %+v", migrated)
	}
}

func TestRoleCredentialStoreAcceptsCanonicalNegativeRoleID(t *testing.T) {
	dataDir := t.TempDir()
	key := make([]byte, roleAuthKeyBytes)
	roleID := "-7928143696348358209"
	if err := mutateRoleCredentialStore(dataDir, "Farm_100", rolePasswordPayload{}, func(records map[string]rolePasswordRecord) error {
		records[roleID] = rolePasswordRecord{Verifier: computeRolePasswordVerifier(key, roleID, "negative-id-secret")}
		return nil
	}); err != nil {
		t.Fatalf("write negative role ID credential: %v", err)
	}
	snapshot, err := readRoleCredentialStore(dataDir)
	if err != nil || !verifyRolePassword(key, roleID, "negative-id-secret", snapshot.Store.Saves["Farm_100"].Roles[roleID].Verifier) {
		t.Fatalf("read negative role ID credential: store=%+v err=%v", snapshot.Store, err)
	}
}

func TestPlayerAuthConfigReportsCorruptStoreAndNeverTreatsItAsWaiting(t *testing.T) {
	dataDir := t.TempDir()
	control := controlDir(dataDir)
	if err := os.MkdirAll(control, 0o700); err != nil {
		t.Fatalf("mkdir control: %v", err)
	}
	if err := os.WriteFile(filepath.Join(control, "status.json"), []byte(`{"saveId":"Farm_100"}`), 0o600); err != nil {
		t.Fatalf("write status: %v", err)
	}
	if err := os.WriteFile(filepath.Join(control, "players-cache.json"), []byte(`{
  "saveId":"Farm_100",
  "updatedAt":"2026-08-16T00:00:00Z",
  "players":[
    {"name":"Host","uniqueMultiplayerId":"1","isHost":true,"status":"offline"},
    {"name":"Leah","uniqueMultiplayerId":"2","status":"offline"}
  ]
}`), 0o600); err != nil {
		t.Fatalf("write players: %v", err)
	}
	driver := newTestDriver(&fakeConsoleDocker{})
	instance := makeStoppedInstance()
	instance.DataDir = dataDir
	current, err := driver.GetPlayerAuthConfig(context.Background(), instance)
	if err != nil {
		t.Fatalf("get initial config: %v", err)
	}
	configured, err := driver.UpdatePlayerAuthConfig(context.Background(), instance, UpdatePlayerAuthConfigRequest{
		ExpectedRevision: current.Revision,
		Mode:             PlayerAuthModeRole,
	})
	if err != nil {
		t.Fatalf("enable empty role mode: %v", err)
	}
	if configured.ConfiguredRoleCount != 0 || configured.UnconfiguredRoleCount != 1 {
		t.Fatalf("empty role mode did not expose first-login enrollment: %+v", configured)
	}
	if err := os.WriteFile(roleCredentialStorePath(dataDir), []byte(`{"schemaVersion":1,"saves":`), 0o600); err != nil {
		t.Fatalf("corrupt role credential store: %v", err)
	}
	corrupt, err := driver.GetPlayerAuthConfig(context.Background(), instance)
	if err != nil {
		t.Fatalf("read config with corrupt store: %v", err)
	}
	if corrupt.RoleCredentialStoreReady || corrupt.CredentialErrorCount != 1 || corrupt.UnconfiguredRoleCount != 0 || corrupt.Roles[0].CredentialStatus != RoleCredentialStatusError {
		t.Fatalf("corrupt store was not reported fail closed: %+v", corrupt)
	}
	_, err = driver.UpdatePlayerAuthConfig(context.Background(), instance, UpdatePlayerAuthConfigRequest{
		ExpectedRevision: configured.Revision,
		Mode:             PlayerAuthModeRole,
		RolePasswordUpdates: []PlayerAuthPasswordUpdate{
			{RoleID: "2", Password: "new-secret"},
		},
	})
	commandErr, ok := err.(*CommandError)
	if !ok || commandErr.Code != "role_credential_store_invalid" {
		t.Fatalf("corrupt store update error = %v", err)
	}
	raw, readErr := os.ReadFile(roleCredentialStorePath(dataDir))
	if readErr != nil || string(raw) != `{"schemaVersion":1,"saves":` {
		t.Fatalf("corrupt store was unexpectedly overwritten: %q err=%v", raw, readErr)
	}
}

func TestRoleModeWithNoRosterPersistsAnEmptyLegacyPayload(t *testing.T) {
	dataDir := t.TempDir()
	if err := sjconfig.UpdateEnvFile(filepath.Join(dataDir, ".env"), map[string]string{}); err != nil {
		t.Fatalf("write initial env: %v", err)
	}
	driver := newTestDriver(&fakeConsoleDocker{})
	instance := makeStoppedInstance()
	instance.DataDir = dataDir
	current, err := driver.GetPlayerAuthConfig(context.Background(), instance)
	if err != nil {
		t.Fatalf("get initial config: %v", err)
	}
	updated, err := driver.UpdatePlayerAuthConfig(context.Background(), instance, UpdatePlayerAuthConfigRequest{
		ExpectedRevision: current.Revision,
		Mode:             PlayerAuthModeRole,
	})
	if err != nil {
		t.Fatalf("enable role mode with no roster: %v", err)
	}
	if updated.Mode != PlayerAuthModeRole || len(updated.Roles) != 0 || !updated.RoleCredentialStoreReady {
		t.Fatalf("empty-roster role mode = %+v", updated)
	}
}

func TestRoleCredentialStoreCommitFailureRestoresAbsentStore(t *testing.T) {
	dataDir := t.TempDir()
	commitErr := errors.New("injected env commit failure")
	err := mutateRoleCredentialStoreAndCommit(dataDir, "Farm_100", rolePasswordPayload{}, func(records map[string]rolePasswordRecord) error {
		records["2"] = rolePasswordRecord{Verifier: computeRolePasswordVerifier(make([]byte, roleAuthKeyBytes), "2", "secret")}
		return nil
	}, func() error {
		return commitErr
	})
	if !errors.Is(err, commitErr) {
		t.Fatalf("commit error = %v, want injected failure", err)
	}
	for _, path := range []string{
		roleCredentialStorePath(dataDir),
		filepath.Join(controlDir(dataDir), roleCredentialStoreMarkerFileName),
		filepath.Join(controlDir(dataDir), roleCredentialStoreLockFileName),
	} {
		if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("transaction rollback left %s: %v", filepath.Base(path), statErr)
		}
	}
}

func TestRoleCredentialStoreCommitFailureRestoresExistingStore(t *testing.T) {
	dataDir := t.TempDir()
	key := make([]byte, roleAuthKeyBytes)
	if err := mutateRoleCredentialStore(dataDir, "Farm_100", rolePasswordPayload{}, func(records map[string]rolePasswordRecord) error {
		records["2"] = rolePasswordRecord{Verifier: computeRolePasswordVerifier(key, "2", "original")}
		return nil
	}); err != nil {
		t.Fatalf("write original credential: %v", err)
	}
	original, err := os.ReadFile(roleCredentialStorePath(dataDir))
	if err != nil {
		t.Fatalf("read original credential store: %v", err)
	}
	commitErr := errors.New("injected env commit failure")
	err = mutateRoleCredentialStoreAndCommit(dataDir, "Farm_100", rolePasswordPayload{}, func(records map[string]rolePasswordRecord) error {
		records["2"] = rolePasswordRecord{Verifier: computeRolePasswordVerifier(key, "2", "replacement")}
		return nil
	}, func() error {
		return commitErr
	})
	if !errors.Is(err, commitErr) {
		t.Fatalf("commit error = %v, want injected failure", err)
	}
	restored, err := os.ReadFile(roleCredentialStorePath(dataDir))
	if err != nil {
		t.Fatalf("read restored credential store: %v", err)
	}
	if string(restored) != string(original) {
		t.Fatalf("credential store was not restored exactly\noriginal: %s\nrestored: %s", original, restored)
	}
	if marker, markerErr := os.ReadFile(filepath.Join(controlDir(dataDir), roleCredentialStoreMarkerFileName)); markerErr != nil || string(marker) != "1\n" {
		t.Fatalf("credential marker after rollback = %q, err=%v", marker, markerErr)
	}
}

func TestRoleCredentialStoreInterruptedFirstPublishFailsClosed(t *testing.T) {
	dataDir := t.TempDir()
	missingTemp := filepath.Join(controlDir(dataDir), "missing-store-temp")
	if err := os.MkdirAll(controlDir(dataDir), 0o700); err != nil {
		t.Fatalf("mkdir control: %v", err)
	}
	if err := publishRoleCredentialStoreTemp(dataDir, missingTemp); err == nil {
		t.Fatal("publishing a missing store temp unexpectedly succeeded")
	}
	marker, err := os.ReadFile(filepath.Join(controlDir(dataDir), roleCredentialStoreMarkerFileName))
	if err != nil || string(marker) != "1\n" {
		t.Fatalf("interrupted first publish did not leave its fail-closed marker: %q err=%v", marker, err)
	}
	if _, err := readRoleCredentialStore(dataDir); err == nil {
		t.Fatal("marker without a store was treated as never initialized")
	}
}

func TestRoleCredentialStoreWithoutMarkerFailsClosed(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.MkdirAll(controlDir(dataDir), 0o700); err != nil {
		t.Fatalf("mkdir control: %v", err)
	}
	key := make([]byte, roleAuthKeyBytes)
	store := newRoleCredentialStore()
	store.Saves["Farm_100"] = roleCredentialSave{Roles: map[string]rolePasswordRecord{
		"2": {Verifier: computeRolePasswordVerifier(key, "2", "existing-password")},
	}}
	raw, err := json.Marshal(store)
	if err != nil {
		t.Fatalf("encode markerless store: %v", err)
	}
	if err := os.WriteFile(roleCredentialStorePath(dataDir), raw, 0o600); err != nil {
		t.Fatalf("write markerless store: %v", err)
	}
	if _, err := readRoleCredentialStore(dataDir); err == nil {
		t.Fatal("a store without its initialization marker was accepted")
	}
}

func TestPlayerAuthUpdateWithMissingSaveIDDoesNotPartiallyCommitEnv(t *testing.T) {
	dataDir := t.TempDir()
	envPath := filepath.Join(dataDir, ".env")
	if err := sjconfig.UpdateEnvFile(envPath, map[string]string{"CUSTOM_SENTINEL": "preserve"}); err != nil {
		t.Fatalf("write initial env: %v", err)
	}
	originalEnv, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("read initial env: %v", err)
	}
	control := controlDir(dataDir)
	if err := os.MkdirAll(control, 0o700); err != nil {
		t.Fatalf("mkdir control: %v", err)
	}
	if err := os.WriteFile(filepath.Join(control, "players-cache.json"), []byte(`{
  "saveId":"",
  "updatedAt":"2026-08-16T00:00:00Z",
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
	if len(current.Roles) != 1 || current.Roles[0].RoleID != "2" {
		t.Fatalf("eligible roles = %+v", current.Roles)
	}
	_, err = driver.UpdatePlayerAuthConfig(context.Background(), instance, UpdatePlayerAuthConfigRequest{
		ExpectedRevision: current.Revision,
		Mode:             PlayerAuthModeRole,
		RolePasswordUpdates: []PlayerAuthPasswordUpdate{
			{RoleID: "2", Password: "admin-secret"},
		},
	})
	commandErr, ok := err.(*CommandError)
	if !ok || commandErr.Code != "player_auth_save_unavailable" {
		t.Fatalf("missing-save update error = %v", err)
	}
	currentEnv, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("read env after rejected update: %v", err)
	}
	if string(currentEnv) != string(originalEnv) {
		t.Fatal("rejected role credential update partially changed .env")
	}
	if _, err := os.Stat(roleCredentialStorePath(dataDir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("rejected update created a credential store: %v", err)
	}
}
