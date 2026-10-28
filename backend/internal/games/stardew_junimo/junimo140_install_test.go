package stardew_junimo

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func freshInstallRunner(t *testing.T, fake *fakeDocker) *installRunner {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "freshworld")
	driver := New(fake, nil, nil, nil)
	instance := registry.Instance{ID: "freshworld", DataDir: dir, State: storage.InstanceStateUninitialized}
	if err := driver.Prepare(context.Background(), instance); err != nil {
		t.Fatal(err)
	}
	return &installRunner{driver: driver, instance: storage.Instance{ID: instance.ID, DataDir: dir, State: instance.State}, imageTag: TestedImageTag}
}

func TestJunimo140FreshPrepareDefaultsAndScopedLegacyPreservation(t *testing.T) {
	fake := &fakeDocker{}
	runner := freshInstallRunner(t, fake)
	values, err := sjconfig.ReadEnvFile(filepath.Join(runner.instance.DataDir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if values["IMAGE_VERSION"] != "1.5.0-preview.140" || !strings.HasSuffix(values["SERVER_IMAGE"], ":1.5.0-preview.140") || !strings.HasSuffix(values["STEAM_SERVICE_IMAGE"], ":1.5.0-preview.140-anxi.1") {
		t.Fatal("fresh environment does not select the fixed .140/Auth140 pair")
	}
	compose, err := os.ReadFile(filepath.Join(runner.instance.DataDir, "docker-compose.yml"))
	if err != nil || !strings.Contains(string(compose), `SDVD_COMPOSE_REV: "4"`) || strings.Contains(string(compose), ":/etc/cont-env.d/") {
		t.Fatal("fresh .140 Compose did not apply the fixed UID/API/init contract")
	}
	legacy := filepath.Join(t.TempDir(), "legacyworld")
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	legacyValues := sjconfig.EmptyEnvTemplate()
	legacyValues["IMAGE_VERSION"] = "1.5.0-preview.125"
	legacyValues["SERVER_IMAGE"] = "sdvd/server:1.5.0-preview.125"
	legacyValues["SERVER_IMAGE_CANDIDATES"] = legacyValues["SERVER_IMAGE"]
	legacyValues["STEAM_SERVICE_IMAGE"] = "anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2"
	legacyValues["STEAM_SERVICE_IMAGE_CANDIDATES"] = legacyValues["STEAM_SERVICE_IMAGE"]
	if err := sjconfig.UpdateEnvFile(filepath.Join(legacy, ".env"), legacyValues); err != nil {
		t.Fatal(err)
	}
	if err := runner.driver.Prepare(context.Background(), registry.Instance{ID: "legacyworld", DataDir: legacy, State: storage.InstanceStateStopped}); err != nil {
		t.Fatal(err)
	}
	after, err := sjconfig.ReadEnvFile(filepath.Join(legacy, ".env"))
	if err != nil || after["SERVER_IMAGE"] != legacyValues["SERVER_IMAGE"] || after["IMAGE_VERSION"] != legacyValues["IMAGE_VERSION"] || after["STEAM_SERVICE_IMAGE"] != legacyValues["STEAM_SERVICE_IMAGE"] || fake.installVolumeCreates != 0 {
		t.Fatal("bootstrap rewrote a legacy world or created an installation volume")
	}
	before, _ := os.ReadFile(filepath.Join(legacy, ".env"))
	err = validateBaseInstallRuntime(storage.Instance{DataDir: legacy, State: storage.InstanceStateStopped}, TestedImageTag)
	var validation *RuntimeUpdateValidationError
	if !errors.As(err, &validation) || validation.Code != "shared_group_confirmation_required" {
		t.Fatal("base installer bypassed the legacy world's scope confirmation")
	}
	unchanged, _ := os.ReadFile(filepath.Join(legacy, ".env"))
	if string(before) != string(unchanged) {
		t.Fatal("scope rejection changed the deployed runtime")
	}
	missing := filepath.Join(t.TempDir(), "missinglegacy")
	err = runner.driver.Prepare(context.Background(), registry.Instance{ID: "missinglegacy", DataDir: missing, State: storage.InstanceStateStopped})
	if !errors.As(err, &validation) || validation.Code != "shared_group_confirmation_required" {
		t.Fatal("bootstrap invented a new default for an existing world with missing configuration")
	}
	if _, err := os.Stat(filepath.Join(missing, ".env")); !os.IsNotExist(err) {
		t.Fatal("scope rejection generated a .140 environment for an unknown old runtime")
	}
	runner.driver.StartRequiredRuntimeUpdate(context.Background(), registry.Instance{ID: "legacyworld", DataDir: legacy, State: storage.InstanceStateStopped})
	status, err := runner.driver.ReadRequiredRuntimeUpdateStatus(registry.Instance{DataDir: legacy})
	if err != nil || status.Phase != requiredRuntimePhaseManual || status.ErrorCode != "shared_group_confirmation_required" {
		t.Fatal("legacy automatic coordinator did not require an explicit .140 scope")
	}
	unchanged, _ = os.ReadFile(filepath.Join(legacy, ".env"))
	if string(before) != string(unchanged) || fake.installVolumeCreates != 0 {
		t.Fatal("legacy bootstrap or automatic coordinator migrated a world")
	}
}

func TestJunimo140InstallationVolumeResumesWithoutLosingDownload(t *testing.T) {
	for _, sourceExists := range []bool{false, true} {
		t.Run(map[bool]string{false: "empty fresh download", true: "preserved repair source"}[sourceExists], func(t *testing.T) {
			fake := &fakeDocker{installVolumes: map[string]bool{"freshworld_game-data": sourceExists}}
			runner := freshInstallRunner(t, fake)
			if err := runner.prepareJunimo140InstallVolume(context.Background()); err != nil {
				t.Fatal(err)
			}
			volume, err := GameDataVolumeName(runner.instance.DataDir)
			if err != nil || !junimo140InstallVolumePattern.MatchString(volume) {
				t.Fatal("fresh download destination is not the owned durable volume")
			}
			plan, err := readInstallVolumePlan(runner.instance.DataDir, volume)
			if err != nil || !plan.Created || !plan.Prepared || plan.SourceExisted != sourceExists || plan.SourceVolume != "freshworld_game-data" {
				t.Fatalf("incomplete installation journal: %v", err)
			}
			// The failure/retry boundary must not clone over a partially completed
			// SteamCMD download, or allocate a second game-data volume.
			runner.instance.State = storage.InstanceStateError
			if err := runner.prepareJunimo140InstallVolume(context.Background()); err != nil {
				t.Fatal(err)
			}
			after, _ := GameDataVolumeName(runner.instance.DataDir)
			clones := 0
			if sourceExists {
				clones = 1
			}
			if after != volume || fake.installVolumeCreates != 1 || fake.installVolumeClones != clones {
				t.Fatal("retry replaced the destination or repeated source cloning")
			}
		})
	}
}

func TestJunimo140InstallationResolvesUncertainCreateWithoutReplay(t *testing.T) {
	fake := &fakeDocker{installVolumeCreateError: errors.New("client lost create response")}
	runner := freshInstallRunner(t, fake)
	if err := runner.prepareJunimo140InstallVolume(context.Background()); err == nil {
		t.Fatal("uncertain create was reported as successful")
	}
	volume, _ := GameDataVolumeName(runner.instance.DataDir)
	if !fake.installVolumes[volume] {
		t.Fatal("uncertain create identity was not retained by the bound journal")
	}
	fake.installVolumeCreateError = nil
	if err := runner.prepareJunimo140InstallVolume(context.Background()); err != nil {
		t.Fatal(err)
	}
	if fake.installVolumeCreates != 1 {
		t.Fatal("retry repeated a create whose actual volume already existed")
	}
}

func TestJunimo140InstallationFailsClosedOnOwnershipAndMissingDownload(t *testing.T) {
	fake := &fakeDocker{installVolumeOwnershipError: errors.New("unexpected volume holder")}
	runner := freshInstallRunner(t, fake)
	if err := runner.prepareJunimo140InstallVolume(context.Background()); err == nil || fake.installVolumeClones != 0 {
		t.Fatal("unowned or referenced volume was used as a download destination")
	}
	fake.installVolumeOwnershipError = nil
	if err := runner.prepareJunimo140InstallVolume(context.Background()); err != nil {
		t.Fatal(err)
	}
	volume, _ := GameDataVolumeName(runner.instance.DataDir)
	delete(fake.installVolumes, volume)
	if err := runner.prepareJunimo140InstallVolume(context.Background()); err == nil || fake.installVolumeCreates != 1 {
		t.Fatal("a missing previously prepared download was silently recreated")
	}
}

func TestJunimo140InstallationReusesVerifiedPatchedSMAPI(t *testing.T) {
	fake := &fakeDocker{}
	runner := freshInstallRunner(t, fake)
	if err := runner.prepareJunimo140InstallVolume(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := runner.ensureJunimo140SMAPIInstalled(context.Background(), nil, gameInstallImage(runner.instance.DataDir)); err != nil {
		t.Fatal(err)
	}
	volume, _ := GameDataVolumeName(runner.instance.DataDir)
	plan, err := readInstallVolumePlan(runner.instance.DataDir, volume)
	if err != nil || !plan.SMAPIVerified || fake.imageSMAPIReads != 1 || fake.smapiRuns != 0 {
		t.Fatal("verified image-owned SMAPI was not retained without an official installer")
	}
	if err := runner.ensureJunimo140SMAPIInstalled(context.Background(), nil, gameInstallImage(runner.instance.DataDir)); err != nil || fake.installVolumeCreates != 1 || fake.smapiRuns != 0 {
		t.Fatal("idempotent SMAPI verification recreated game data or invoked the official installer")
	}
	current := registry.Instance{ID: runner.instance.ID, DataDir: runner.instance.DataDir, State: storage.InstanceStateGameInstalled}
	runner.driver.StartRequiredRuntimeUpdate(context.Background(), current)
	if _, err := runner.driver.ReadRequiredRuntimeUpdateStatus(current); !os.IsNotExist(err) {
		t.Fatal("an already current fresh .140 runtime was scheduled for migration during bootstrap")
	}
}

func TestJunimo140DefaultKeepsLegacy125AuthOnlyPair(t *testing.T) {
	store, instance, dir := newInstalledAuthOnlyFixture(t)
	legacy, err := legacy125RuntimeManifest()
	if err != nil {
		t.Fatal(err)
	}
	before := map[string]string{"IMAGE_VERSION": legacy.Server.Tag, "SERVER_IMAGE": legacy.Server.Image,
		"SERVER_IMAGE_CANDIDATES": strings.Join(legacy.Server.Images, ","), "STEAM_SERVICE_IMAGE": legacy.SteamAuth.Image,
		"STEAM_SERVICE_IMAGE_CANDIDATES": strings.Join(legacy.SteamAuth.Images, ","), "GAME_DATA_VOLUME": "stardew_game-data"}
	if err := sjconfig.UpdateEnvFile(filepath.Join(dir, ".env"), before); err != nil {
		t.Fatal(err)
	}
	fake := &fakeDocker{steamAuthLines: []string{"[SteamAuth:A0] Authenticated as steam-user", "[SteamAuth:A0] Logged in as [U:1:1231122837]"}}
	driver := New(fake, nil, jobs.NewManager(store, slog.Default()), store)
	job, err := driver.Install(context.Background(), registry.InstallRequest{Instance: registry.Instance{ID: instance.ID}, SteamUsername: "steam-user", SteamPassword: "steam-pass", AuthLoginOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	waitForDriverTestJobStatus(t, store, job.ID, storage.JobStatusSucceeded)
	after, err := sjconfig.ReadEnvFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range before {
		if after[key] != value {
			t.Fatalf("legacy auth-only changed runtime field %s", key)
		}
	}
	if fake.steamAuthOpts.ImageRef != legacy.SteamAuth.Image || fake.installVolumeCreates != 0 || fake.containerRuns != 0 || fake.smapiRuns != 0 {
		t.Fatal("legacy auth-only used the fresh Auth140 default or a base installation path")
	}
}

func TestJunimo140RetainedInstallSourcesRemainInWorldNamespace(t *testing.T) {
	fake := &fakeDocker{installVolumes: map[string]bool{"freshworld_game-data": true}}
	runner := freshInstallRunner(t, fake)
	if err := runner.prepareJunimo140InstallVolume(context.Background()); err != nil {
		t.Fatal(err)
	}
	retained, err := retainedInstallGameVolumes(runner.instance.DataDir, "freshworld")
	if err != nil || len(retained) != 1 || retained[0] != "freshworld_game-data" {
		t.Fatal("same-world source was not retained for exact deletion planning")
	}
	volume, _ := GameDataVolumeName(runner.instance.DataDir)
	plan, _ := readInstallVolumePlan(runner.instance.DataDir, volume)
	plan.SourceVolume = "other-world_game-data"
	if err := writeInstallVolumePlan(runner.instance.DataDir, plan); err != nil {
		t.Fatal(err)
	}
	retained, err = retainedInstallGameVolumes(runner.instance.DataDir, "freshworld")
	if err != nil || len(retained) != 0 {
		t.Fatal("foreign shared source became eligible for world deletion")
	}
	plan.Project = "other-world"
	if err := writeInstallVolumePlan(runner.instance.DataDir, plan); err != nil {
		t.Fatal(err)
	}
	if _, err := retainedInstallGameVolumes(runner.instance.DataDir, "freshworld"); err == nil {
		t.Fatal("tampered installation journal was accepted for deletion")
	}
}
