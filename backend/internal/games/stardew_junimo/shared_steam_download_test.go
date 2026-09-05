package stardew_junimo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	sharedsteamcmd "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/steamcmd"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func TestSharedSteamDownloadMigratesLegacyCredentialsAndUsesPanelVolumes(t *testing.T) {
	dataDir := t.TempDir()
	instanceDir := filepath.Join(dataDir, "instances", "stardew")
	if err := os.MkdirAll(instanceDir, 0o700); err != nil {
		t.Fatalf("create instance directory: %v", err)
	}
	if err := sjconfig.UpdateEnvFile(filepath.Join(instanceDir, ".env"), map[string]string{
		"STEAM_USERNAME": "legacy-user", "STEAM_PASSWORD": "legacy-pass", "STEAMCMD_AUTH_COMPLETED": "true",
	}); err != nil {
		t.Fatalf("write legacy instance credentials: %v", err)
	}
	manager := sharedsteamcmd.NewManager(dataDir, "panel-qa")
	driver := NewWithOptions(nil, nil, nil, nil, DriverOptions{ContainerDataDir: dataDir, SteamDownloads: manager})
	instance := registry.Instance{ID: "stardew", DriverID: DriverID, DataDir: instanceDir}

	credentials, found, err := driver.sharedSteamCredentials(instance)
	if err != nil || !found {
		t.Fatalf("migrate shared credentials: found=%v err=%v", found, err)
	}
	if credentials.Username != "legacy-user" || credentials.Password != "legacy-pass" || !credentials.AuthorizationCompleted {
		t.Fatalf("migrated credentials = %#v", credentials)
	}
	if err := driver.saveSharedSteamCredentials(instance, "new-user", "new-pass"); err != nil {
		t.Fatalf("save Panel credentials: %v", err)
	}
	credentials, found, err = driver.sharedSteamCredentials(instance)
	if err != nil || !found || credentials.Username != "new-user" || credentials.Password != "new-pass" || credentials.AuthorizationCompleted {
		t.Fatalf("updated credentials = %#v found=%v err=%v", credentials, found, err)
	}

	runner := installRunner{driver: driver, instance: makeStorageInstanceForSharedSteamTest(instance), username: "new-user", password: "new-pass"}
	opts, err := runner.buildSteamCMDOpts(DefaultSteamCMDImage)
	if err != nil {
		t.Fatalf("build shared SteamCMD options: %v", err)
	}
	joined := strings.Join(opts.Binds, "\n")
	for _, expected := range []string{"panel-qa_steamcmd-login:/home/steam/Steam", "panel-qa_steamcmd-home:/root/.steam"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("shared SteamCMD bind %q missing from %q", expected, joined)
		}
	}
	if strings.Contains(joined, "stardew_steamcmd-login") {
		t.Fatalf("instance-scoped SteamCMD authorization volume remained: %q", joined)
	}
}

func makeStorageInstanceForSharedSteamTest(instance registry.Instance) storage.Instance {
	return storage.Instance{ID: instance.ID, DriverID: instance.DriverID, DataDir: instance.DataDir}
}
