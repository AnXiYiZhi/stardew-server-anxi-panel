package stardew_junimo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
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

func TestEmbeddedControlManifestMatchesRuntimeStack(t *testing.T) {
	manifest, err := legacy125RuntimeManifest()
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyEmbeddedControlManifest(manifest); err != nil {
		t.Fatal(err)
	}
}

func (f *smapiDetectionFakeDocker) RuntimeReadSMAPIMetadata(context.Context, string, string, string) (paneldocker.RuntimeSMAPIMetadata, error) {
	return f.metadata, nil
}
func (f *smapiDetectionFakeDocker) RuntimeReadContentManifests(context.Context, string, string, string) (paneldocker.RuntimeContentRead, error) {
	return paneldocker.RuntimeContentRead{GameManifest: f.game, SDKManifest: f.sdk, FreeBytes: 10 << 30}, nil
}

func (f *smapiDetectionFakeDocker) RuntimeReadContentVersions(ctx context.Context, dir, volume, image string) (paneldocker.RuntimeContentRead, error) {
	return f.RuntimeReadContentManifests(ctx, dir, volume, image)
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
	manifest, err := legacy125RuntimeManifest()
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
	return useLegacy125RuntimeFixture(New(fake, nil, nil, nil)), registry.Instance{ID: "stardew", DriverID: DriverID, DataDir: dir, State: storage.InstanceStateStopped}, fake
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

func TestSMAPIDetectionAcceptsIndependentCompleteSDKBuild(t *testing.T) {
	for _, smapi := range []struct {
		version string
		present bool
		want    string
	}{{"4.5.1", true, SMAPIStatusUpdateAvailable}, {"4.5.2", true, SMAPIStatusUpToDate}, {"4.5.1", false, SMAPIStatusMissing}} {
		t.Run(smapi.want, func(t *testing.T) {
			driver, instance, fake := smapiDetectionFixture(t, smapi.version)
			fake.sdk = installedSDKFixture("25329704", "7900000000000000001")
			fake.metadata.Present = smapi.present
			got, err := driver.InspectSMAPIUpdate(context.Background(), instance)
			if err != nil || got.Status != smapi.want || !got.Supported {
				t.Fatalf("complete independent SDK was rejected: %#v %v", got, err)
			}
			if got.Available != (smapi.want != SMAPIStatusUpToDate) {
				t.Fatalf("incorrect SMAPI availability: %#v", got)
			}
		})
	}
}

func TestSMAPIDetectionRejectsInvalidPrerequisitesWhenPresentOrMissing(t *testing.T) {
	for _, present := range []bool{true, false} {
		for _, name := range []string{"game build", "game and SDK", "SDK missing", "SDK incomplete", "SDK wrong depot", "SDK missing depot", "SDK invalid"} {
			t.Run(name+"/present="+map[bool]string{true: "true", false: "false"}[present], func(t *testing.T) {
				driver, instance, fake := smapiDetectionFixture(t, "4.5.1")
				fake.metadata.Present = present
				switch name {
				case "game build", "game and SDK":
					fake.game = manifestFixture("413150", "111", "4", "Stardew Valley")
					if name == "game and SDK" {
						fake.sdk = nil
					}
				case "SDK missing":
					fake.sdk = nil
				case "SDK incomplete":
					fake.sdk = manifestFixture("1007", "25329704", "6", "Steamworks Shared")
				case "SDK wrong depot":
					fake.sdk = []byte(strings.Replace(string(installedSDKFixture("25329704", "123")), `"1006"`, `"1005"`, 1))
				case "SDK missing depot":
					fake.sdk = []byte(strings.Replace(string(fake.sdk), `"InstalledDepots"`, `"UnknownDepots"`, 1))
				case "SDK invalid":
					fake.sdk = []byte("invalid ACF")
				}
				got, err := driver.InspectSMAPIUpdate(context.Background(), instance)
				if err != nil || got.Status != SMAPIStatusIncompatibleGame || got.Available || got.Code != "incompatible_game_or_sdk" {
					t.Fatalf("unsafe SMAPI change was offered: %#v %v", got, err)
				}
				dry, err := driver.RunSMAPIUpdateDryRun(context.Background(), instance)
				if err != nil || dry.Phase != RuntimeUpdatePhaseFailed || dry.ErrorCode != "incompatible_game_or_sdk" {
					t.Fatalf("invalid prerequisites passed SMAPI preflight: %#v %v", dry, err)
				}
			})
		}
	}
}

type smapiApplyContentGateFake struct {
	*smapiWorkflowFakeDocker
	content paneldocker.RuntimeContentRead
	missing bool
}

func (f *smapiApplyContentGateFake) RuntimeReadContentVersions(context.Context, string, string, string) (paneldocker.RuntimeContentRead, error) {
	return f.content, nil
}

func (f *smapiApplyContentGateFake) RuntimeReadSMAPIMetadata(ctx context.Context, dir, volume, image string) (paneldocker.RuntimeSMAPIMetadata, error) {
	if f.missing {
		return paneldocker.RuntimeSMAPIMetadata{}, nil
	}
	return f.smapiWorkflowFakeDocker.RuntimeReadSMAPIMetadata(ctx, dir, volume, image)
}

func TestSMAPIApplyRechecksPrerequisitesForMissingSMAPIAfterDryRun(t *testing.T) {
	for _, name := range []string{"SDK missing", "SDK incomplete", "game unsupported"} {
		t.Run(name, func(t *testing.T) {
			driver, instance, base := setupSMAPIWorkflowDriver(t, storage.InstanceStateStopped)
			read, err := base.fakeDocker.RuntimeReadContentVersions(context.Background(), "", "", "")
			if err != nil {
				t.Fatal(err)
			}
			fake := &smapiApplyContentGateFake{smapiWorkflowFakeDocker: base, content: read}
			driver.docker = fake
			dry, err := driver.RunSMAPIUpdateDryRun(context.Background(), instance)
			if err != nil || dry.Phase != RuntimeUpdatePhaseSucceeded {
				t.Fatalf("compatible preflight failed: %#v %v", dry, err)
			}
			fake.missing = true
			switch name {
			case "SDK missing":
				fake.content.SDKManifest = nil
			case "SDK incomplete":
				fake.content.SDKManifest = manifestFixture("1007", "25329704", "6", "Steamworks Shared")
			case "game unsupported":
				fake.content.GameManifest = manifestFixture("413150", "111", "4", "Stardew Valley")
			}
			_, err = driver.StartSMAPIUpdateApply(context.Background(), instance, 0)
			var validation *RuntimeUpdateValidationError
			if !errors.As(err, &validation) || validation.Code != "incompatible_game_or_sdk" {
				t.Fatalf("old dry-run authorized a changed invalid installation: %v", err)
			}
			if len(base.calls) != 0 || len(base.applyCalls) != 0 {
				t.Fatalf("prerequisite failure mutated runtime: calls=%v apply=%v", base.calls, base.applyCalls)
			}
		})
	}
}
