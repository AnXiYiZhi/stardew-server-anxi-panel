package stardew_junimo

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

type smapiDetectionFakeDocker struct {
	fakeDocker
	metadata paneldocker.RuntimeSMAPIMetadata
	game     []byte
	sdk      []byte
}

func (f *smapiDetectionFakeDocker) RuntimeReadSMAPIMetadata(context.Context, string, string, string) (paneldocker.RuntimeSMAPIMetadata, error) {
	return f.metadata, nil
}
func (f *smapiDetectionFakeDocker) RuntimeReadContentManifests(context.Context, string, string, string) (paneldocker.RuntimeContentRead, error) {
	return paneldocker.RuntimeContentRead{GameManifest: f.game, SDKManifest: f.sdk, FreeBytes: 10 << 30}, nil
}
func (f *smapiDetectionFakeDocker) DockerVersion(context.Context, string) (paneldocker.CommandResult, error) {
	return paneldocker.CommandResult{}, nil
}
func (f *smapiDetectionFakeDocker) RuntimeImageInspect(context.Context, string, string) (paneldocker.RuntimeImageMetadata, error) {
	return paneldocker.RuntimeImageMetadata{}, nil
}

func smapiDetectionFixture(t *testing.T, version string) (*Driver, registry.Instance, *smapiDetectionFakeDocker) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "stardew")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest, err := sjconfig.BuiltInRuntimeStackManifest()
	if err != nil {
		t.Fatal(err)
	}
	if err := sjconfig.UpdateEnvFile(filepath.Join(dir, ".env"), map[string]string{
		"GAME_DATA_VOLUME": "stardew_game-data", "IMAGE_VERSION": manifest.Server.Tag,
		"SERVER_IMAGE": manifest.Server.Image, "SERVER_IMAGE_CANDIDATES": manifest.Server.Image,
		"STEAM_SERVICE_IMAGE": manifest.SteamAuth.Image, "STEAM_SERVICE_IMAGE_CANDIDATES": manifest.SteamAuth.Image,
		"SMAPI_VERSION": "0.0.1",
	}); err != nil {
		t.Fatal(err)
	}
	fake := &smapiDetectionFakeDocker{
		metadata: paneldocker.RuntimeSMAPIMetadata{Present: true, RequiredFiles: true, Version: version + "+821167e5c511bf3a2d98f604e5e838561c469219", VersionEvidence: "assembly"},
		game:     manifestFixture("413150", manifest.Game.BuildID, "4", "Stardew Valley"),
		sdk:      manifestFixture("1007", manifest.SDK.BuildID, "4", "Steamworks SDK Redist"),
	}
	return New(fake, nil, nil, nil), registry.Instance{ID: "stardew", DriverID: DriverID, DataDir: dir, State: storage.InstanceStateStopped}, fake
}

func TestSMAPIDetectionUsesAssemblyMetadataNotEnv(t *testing.T) {
	driver, instance, _ := smapiDetectionFixture(t, "4.5.2")
	got, err := driver.InspectSMAPIUpdate(context.Background(), instance)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != SMAPIStatusUpToDate || got.Current.Version != "4.5.2" || got.Current.ConfiguredVersion != "0.0.1" {
		t.Fatalf("unexpected detection: %#v", got)
	}
}

func TestSMAPIDetectionStatuses(t *testing.T) {
	driver, instance, fake := smapiDetectionFixture(t, "4.5.1")
	got, err := driver.InspectSMAPIUpdate(context.Background(), instance)
	if err != nil || got.Status != SMAPIStatusUpdateAvailable {
		t.Fatalf("got %#v err=%v", got, err)
	}
	fake.metadata.Present = false
	got, _ = driver.InspectSMAPIUpdate(context.Background(), instance)
	if got.Status != SMAPIStatusMissing {
		t.Fatalf("missing=%#v", got)
	}
	fake.metadata = paneldocker.RuntimeSMAPIMetadata{Present: true, RequiredFiles: false, Version: "garbage"}
	got, _ = driver.InspectSMAPIUpdate(context.Background(), instance)
	if got.Status != SMAPIStatusInvalid {
		t.Fatalf("invalid=%#v", got)
	}
}

func TestSMAPIDetectionRejectsIncompatibleGameAndJunimo(t *testing.T) {
	driver, instance, fake := smapiDetectionFixture(t, "4.5.1")
	fake.game = manifestFixture("413150", "111", "4", "Stardew Valley")
	got, _ := driver.InspectSMAPIUpdate(context.Background(), instance)
	if got.Status != SMAPIStatusIncompatibleGame {
		t.Fatalf("game=%#v", got)
	}
	fake.game = manifestFixture("413150", "16826371", "4", "Stardew Valley")
	values, _ := sjconfig.ReadEnvFile(filepath.Join(instance.DataDir, ".env"))
	values["IMAGE_VERSION"] = "1.5.0-preview.120"
	values["SERVER_IMAGE"] = "sdvd/server:1.5.0-preview.120"
	values["SERVER_IMAGE_CANDIDATES"] = "sdvd/server:1.5.0-preview.120"
	if err := sjconfig.UpdateEnvFile(filepath.Join(instance.DataDir, ".env"), values); err != nil {
		t.Fatal(err)
	}
	got, _ = driver.InspectSMAPIUpdate(context.Background(), instance)
	if got.Status != SMAPIStatusIncompatibleJunimo {
		t.Fatalf("junimo=%#v", got)
	}
}
