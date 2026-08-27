package stardew_junimo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func TestMigrateServerSteamAuthDependencyPreservesOtherDependenciesEOLAndMode(t *testing.T) {
	dataDir := t.TempDir()
	path := filepath.Join(dataDir, "docker-compose.yml")
	before := "services:\r\n" +
		"  steam-auth:\r\n" +
		"    image: auth:test\r\n" +
		"  database:\r\n" +
		"    image: database:test\r\n" +
		"  server:\r\n" +
		"    image: server:test\r\n" +
		"    depends_on:\r\n" +
		"      steam-auth:\r\n" +
		"        condition: service_started\r\n" +
		"      database:\r\n" +
		"        condition: service_healthy\r\n" +
		"    environment:\r\n" +
		"      CUSTOM_VALUE: preserved\r\n"
	want := "services:\r\n" +
		"  steam-auth:\r\n" +
		"    image: auth:test\r\n" +
		"  database:\r\n" +
		"    image: database:test\r\n" +
		"  server:\r\n" +
		"    image: server:test\r\n" +
		"    depends_on:\r\n" +
		"      database:\r\n" +
		"        condition: service_healthy\r\n" +
		"    environment:\r\n" +
		"      CUSTOM_VALUE: preserved\r\n"
	if err := os.WriteFile(path, []byte(before), 0o640); err != nil {
		t.Fatalf("write compose: %v", err)
	}
	beforeInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat compose before migration: %v", err)
	}

	changed, err := migrateServerSteamAuthDependency(path)
	if err != nil {
		t.Fatalf("migrate dependency: %v", err)
	}
	if !changed {
		t.Fatal("legacy dependency was not reported as changed")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migrated compose: %v", err)
	}
	if string(got) != want {
		t.Fatalf("unexpected migrated compose:\nwant=%q\ngot =%q", want, string(got))
	}
	afterInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat compose after migration: %v", err)
	}
	if afterInfo.Mode().Perm() != beforeInfo.Mode().Perm() {
		t.Fatalf("compose mode changed: before=%o after=%o", beforeInfo.Mode().Perm(), afterInfo.Mode().Perm())
	}

	changed, err = migrateServerSteamAuthDependency(path)
	if err != nil || changed {
		t.Fatalf("idempotent migration changed=%v err=%v", changed, err)
	}
	again, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read compose after second migration: %v", err)
	}
	if string(again) != want {
		t.Fatal("idempotent migration changed the Compose file")
	}
}

func TestMigrateServerSteamAuthDependencyRemovesOnlyListEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "docker-compose.yml")
	before := "services:\n  server:\n    image: server:test\n    depends_on:\n      - steam-auth\n      - database\n    ports:\n      - 24642:24642/udp\n"
	want := "services:\n  server:\n    image: server:test\n    depends_on:\n      - database\n    ports:\n      - 24642:24642/udp\n"
	if err := os.WriteFile(path, []byte(before), 0o600); err != nil {
		t.Fatal(err)
	}
	changed, err := migrateServerSteamAuthDependency(path)
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("list dependency migration:\nwant=%q\ngot =%q", want, string(got))
	}
}

func TestMigrateServerSteamAuthDependencyRemovesEmptyDependsOnBlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "docker-compose.yml")
	before := "services:\n  server:\n    image: server:test\n    depends_on:\n      steam-auth:\n        condition: service_started\n    ports:\n      - 24642:24642/udp\n"
	want := "services:\n  server:\n    image: server:test\n    ports:\n      - 24642:24642/udp\n"
	if err := os.WriteFile(path, []byte(before), 0o600); err != nil {
		t.Fatal(err)
	}
	changed, err := migrateServerSteamAuthDependency(path)
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("single dependency migration:\nwant=%q\ngot =%q", want, string(got))
	}
}

func TestMigrateServerSteamAuthDependencyRejectsInlineLayoutWithoutWriting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "docker-compose.yml")
	before := "services:\n  server:\n    image: server:test\n    depends_on: [steam-auth, database]\n"
	if err := os.WriteFile(path, []byte(before), 0o600); err != nil {
		t.Fatal(err)
	}
	changed, err := migrateServerSteamAuthDependency(path)
	if changed || !errors.Is(err, ErrSteamInviteComposeDependencyUnsupported) {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != before {
		t.Fatal("unsupported Compose layout was modified")
	}
}

func TestDriverPrepareConvergesLegacyDisabledSteamInviteRuntimeExactlyOnce(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "legacy-disabled")
	writeLegacySteamInviteRuntimeFixture(t, dataDir, false)
	sentinelPath := filepath.Join(dataDir, "game-data-sentinel")
	if err := os.WriteFile(sentinelPath, []byte("preserved-game-data"), 0o600); err != nil {
		t.Fatal(err)
	}
	fake := &fakeDocker{}
	driver := New(fake, nil, nil, nil)
	instance := registry.Instance{ID: "legacy-disabled", DataDir: dataDir, State: storage.InstanceStateRunning}

	if err := driver.Prepare(context.Background(), instance); err != nil {
		t.Fatalf("prepare legacy disabled instance: %v", err)
	}
	wantVolume := []string{"legacy-disabled_steam-session"}
	if !reflect.DeepEqual(fake.removedByVolumes, wantVolume) || !reflect.DeepEqual(fake.removedVolumes, wantVolume) {
		t.Fatalf("cleanup holders=%v volumes=%v want=%v", fake.removedByVolumes, fake.removedVolumes, wantVolume)
	}
	if fake.workDir != "" || fake.composePulls != 0 || len(fake.pulledImages) != 0 || len(fake.inspectedImages) != 0 || fake.steamAuthRuns != 0 {
		t.Fatalf("disabled convergence probed or materialized Auth: workDir=%q composePulls=%d pulls=%v inspects=%v authRuns=%d", fake.workDir, fake.composePulls, fake.pulledImages, fake.inspectedImages, fake.steamAuthRuns)
	}
	if !sjconfig.SteamInviteRuntimeScopeCurrent(dataDir) {
		t.Fatal("successful disabled convergence did not persist its marker")
	}
	fields, err := sjconfig.ReadEnvFile(filepath.Join(dataDir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{
		"STEAM_INVITE_ENABLED":    "false",
		"STEAMCMD_AUTH_COMPLETED": "true",
		"STEAM_USERNAME":          "legacy-user",
		"STEAM_PASSWORD":          "legacy-password",
		"GAME_DATA_VOLUME":        "legacy-disabled-game-data",
	} {
		if fields[key] != want {
			t.Fatalf("disabled convergence changed %s: got %q want %q", key, fields[key], want)
		}
	}
	sentinel, err := os.ReadFile(sentinelPath)
	if err != nil || string(sentinel) != "preserved-game-data" {
		t.Fatalf("game data sentinel changed: data=%q err=%v", sentinel, err)
	}
	compose, err := os.ReadFile(filepath.Join(dataDir, "docker-compose.yml"))
	if err != nil {
		t.Fatal(err)
	}
	serverStart, serverEnd := composeServiceBounds(string(compose), "server")
	if serverStart < 0 || strings.Contains(string(compose)[serverStart:serverEnd], "steam-auth:") {
		t.Fatalf("server still depends on optional Auth:\n%s", compose)
	}
	if !strings.Contains(string(compose), "  steam-auth:\n") {
		t.Fatal("optional Auth service definition must remain available for later opt-in")
	}

	if err := driver.Prepare(context.Background(), instance); err != nil {
		t.Fatalf("repeat Prepare: %v", err)
	}
	if !reflect.DeepEqual(fake.removedByVolumes, wantVolume) || !reflect.DeepEqual(fake.removedVolumes, wantVolume) {
		t.Fatalf("marked runtime was cleaned again: holders=%v volumes=%v", fake.removedByVolumes, fake.removedVolumes)
	}
}

func TestDriverPrepareLegacyEnabledSteamInvitePreservesSessionWithoutDockerAccess(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "legacy-enabled")
	writeLegacySteamInviteRuntimeFixture(t, dataDir, true)
	fake := &fakeDocker{
		psErr:               errors.New("must not probe"),
		pullErr:             errors.New("must not pull"),
		inspectErr:          errors.New("must not inspect"),
		removeContainersErr: errors.New("must not remove session holder"),
		removeVolumesErr:    errors.New("must not remove session volume"),
	}
	driver := New(fake, nil, nil, nil)
	if err := driver.Prepare(context.Background(), registry.Instance{
		ID: "legacy-enabled", DataDir: dataDir, State: storage.InstanceStateStopped,
	}); err != nil {
		t.Fatalf("prepare legacy enabled instance: %v", err)
	}
	if !sjconfig.SteamInviteRuntimeScopeCurrent(dataDir) {
		t.Fatal("enabled runtime scope was not marked current")
	}
	if fake.workDir != "" || len(fake.removedByVolumes) != 0 || len(fake.removedVolumes) != 0 || fake.composePulls != 0 || len(fake.pulledImages) != 0 || len(fake.inspectedImages) != 0 {
		t.Fatalf("enabled migration touched Docker: workDir=%q holders=%v volumes=%v pulls=%v inspects=%v", fake.workDir, fake.removedByVolumes, fake.removedVolumes, fake.pulledImages, fake.inspectedImages)
	}
	fields, err := sjconfig.ReadEnvFile(filepath.Join(dataDir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if fields["STEAM_AUTH_COMPLETED"] != "true" || fields["STEAM_INVITE_ENABLED"] != "true" || fields["STEAMCMD_AUTH_COMPLETED"] != "true" {
		t.Fatalf("enabled authorization/session intent changed: %#v", fields)
	}
}

func TestDriverPrepareLegacyDisabledScopeFailureLeavesMarkerAbsentAndRetries(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "legacy-retry")
	writeLegacySteamInviteRuntimeFixture(t, dataDir, false)
	fake := &fakeDocker{removeVolumesErr: errors.New("session volume busy")}
	driver := New(fake, nil, nil, nil)
	instance := registry.Instance{ID: "legacy-retry", DataDir: dataDir, State: storage.InstanceStateStopped}

	if err := driver.Prepare(context.Background(), instance); err == nil {
		t.Fatal("expected exact session cleanup failure")
	}
	if sjconfig.SteamInviteRuntimeScopeCurrent(dataDir) {
		t.Fatal("failed convergence must not persist the scope marker")
	}
	if len(fake.removedByVolumes) != 1 || len(fake.removedVolumes) != 1 {
		t.Fatalf("unexpected first cleanup calls: holders=%v volumes=%v", fake.removedByVolumes, fake.removedVolumes)
	}

	fake.removeVolumesErr = nil
	if err := driver.Prepare(context.Background(), instance); err != nil {
		t.Fatalf("retry legacy convergence: %v", err)
	}
	if !sjconfig.SteamInviteRuntimeScopeCurrent(dataDir) {
		t.Fatal("successful retry did not persist the scope marker")
	}
	if len(fake.removedByVolumes) != 2 || len(fake.removedVolumes) != 2 {
		t.Fatalf("failed convergence was not retried: holders=%v volumes=%v", fake.removedByVolumes, fake.removedVolumes)
	}
}

func TestDriverPrepareLegacyDisabledUnknownSessionHolderFailsClosed(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "legacy-unknown-holder")
	writeLegacySteamInviteRuntimeFixture(t, dataDir, false)
	fake := &fakeDocker{removeContainersErr: errors.New("unknown container shares Steam invite session volume")}
	driver := New(fake, nil, nil, nil)
	instance := registry.Instance{ID: "legacy-unknown-holder", DataDir: dataDir, State: storage.InstanceStateStopped}

	if err := driver.Prepare(context.Background(), instance); err == nil || !strings.Contains(err.Error(), "unknown container") {
		t.Fatalf("Prepare error = %v, want unknown holder rejection", err)
	}
	if !reflect.DeepEqual(fake.removedByVolumes, []string{"legacy-unknown-holder_steam-session"}) {
		t.Fatalf("safe holder classification calls = %v", fake.removedByVolumes)
	}
	if len(fake.removedVolumes) != 0 {
		t.Fatalf("unknown holder rejection attempted volume removal: %v", fake.removedVolumes)
	}
	if sjconfig.SteamInviteRuntimeScopeCurrent(dataDir) {
		t.Fatal("unknown holder rejection must not persist the scope marker")
	}
}

func TestDriverPrepareConvergesSuccessfulAuthHolderWithoutDeletingSession(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "cleanup-pending-prepare")
	writeLegacySteamInviteRuntimeFixture(t, dataDir, true)
	if err := sjconfig.SetSteamInviteAuthState(dataDir, sjconfig.SteamInviteAuthStateCleanupPending); err != nil {
		t.Fatal(err)
	}
	fake := &fakeDocker{}
	driver := New(fake, nil, nil, nil)
	instance := registry.Instance{ID: "cleanup-pending-prepare", DataDir: dataDir, State: storage.InstanceStateStopped}

	if err := driver.Prepare(context.Background(), instance); err != nil {
		t.Fatalf("Prepare cleanup-pending instance: %v", err)
	}
	if !reflect.DeepEqual(fake.removedByVolumes, []string{"cleanup-pending-prepare_steam-session"}) {
		t.Fatalf("holder cleanup = %v", fake.removedByVolumes)
	}
	if len(fake.removedVolumes) != 0 {
		t.Fatalf("Prepare deleted successful session volume: %v", fake.removedVolumes)
	}
	if got := sjconfig.SteamInviteAuthState(dataDir); got != sjconfig.SteamInviteAuthStateReady {
		t.Fatalf("Prepare auth state = %q, want ready", got)
	}
}

func writeLegacySteamInviteRuntimeFixture(t *testing.T, dataDir string, enabled bool) {
	t.Helper()
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	compose := "services:\n" +
		"  steam-auth:\n" +
		"    image: auth:test\n" +
		"    volumes:\n" +
		"      - steam-session:/data/steam-session\n" +
		"  server:\n" +
		"    image: server:test\n" +
		"    depends_on:\n" +
		"      steam-auth:\n" +
		"        condition: service_started\n" +
		"    volumes:\n" +
		"      - game-data:/data/game\n" +
		"volumes:\n" +
		"  steam-session:\n" +
		"  game-data:\n" +
		"    name: ${GAME_DATA_VOLUME}\n"
	if err := os.WriteFile(filepath.Join(dataDir, "docker-compose.yml"), []byte(compose), 0o640); err != nil {
		t.Fatal(err)
	}
	inviteEnabled := "false"
	authCompleted := ""
	authState := sjconfig.SteamInviteAuthStateDisabled
	if enabled {
		inviteEnabled = "true"
		authCompleted = "true"
		authState = sjconfig.SteamInviteAuthStateReady
	}
	if err := sjconfig.UpdateEnvFile(filepath.Join(dataDir, ".env"), map[string]string{
		"GAME_DATA_VOLUME":        filepath.Base(dataDir) + "-game-data",
		"STEAMCMD_AUTH_COMPLETED": "true",
		"STEAM_AUTH_COMPLETED":    authCompleted,
		"STEAM_INVITE_ENABLED":    inviteEnabled,
		"STEAM_INVITE_AUTH_STATE": authState,
		"STEAM_USERNAME":          "legacy-user",
		"STEAM_PASSWORD":          "legacy-password",
	}); err != nil {
		t.Fatal(err)
	}
}
