package stardew_junimo

import (
	"encoding/json"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"strings"

	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
)

// Historical ordinary updater/official-SMAPI recovery tests exercise the exact
// .125 protocol. Give each driver its own immutable fixture rather than changing
// the process-wide .140 default or weakening its scope-confirmation boundary.
func legacy125RuntimeManifest() (sjconfig.RuntimeStackManifest, error) {
	current, err := sjconfig.BuiltInRuntimeStackManifest()
	if err != nil {
		return current, err
	}
	raw, err := json.Marshal(current)
	if err != nil {
		return current, err
	}
	var fixture sjconfig.RuntimeStackManifest
	if err := json.Unmarshal(raw, &fixture); err != nil {
		return fixture, err
	}
	fixture.StackVersion = "junimo-1.5.0-preview.125_auth-1.5.0-anxi.2_game-16826371_sdk-20939719_smapi-4.5.2_control-0.3.9"
	serverImages := []string{}
	for _, ref := range current.Server.Images {
		serverImages = append(serverImages, strings.TrimSuffix(ref, ":1.5.0-preview.140")+":1.5.0-preview.125")
	}
	component := func(tag, upstream, revision, digest string, images []string) sjconfig.RuntimeStackManifestComponent {
		pins := map[string]string{}
		for _, image := range images {
			pins[image] = digest
		}
		return sjconfig.RuntimeStackManifestComponent{Tag: tag, UpstreamRef: upstream, SourceRevision: revision,
			Images: append([]string{}, images...), Image: images[0], TrustedCandidates: append([]string{}, images...), Digests: pins}
	}
	fixture.Server = component("1.5.0-preview.125", "", "", "sha256:10f438581d741fc146ce710cbe20099475ac68908e99f565cf449f0b8192ccf6", serverImages)
	fixture.SteamAuth = component("1.5.0-anxi.2", "refs/tags/preview-1.5.0.121", "a6cee498f91b318cfcacf035b05b0644bd16829a", "sha256:99420ab30c09da019c425defd4d01796403ac03898ed261b9ee2a976f4bc6518", []string{
		"docker.1ms.run/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2",
		"crpi-9z3bkb9g7fxeohrg.cn-hangzhou.personal.cr.aliyuncs.com/anxi-panel/junimo-steam-service-cn:1.5.0-anxi.2",
		"docker.m.daocloud.io/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2",
		"ghcr.io/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2",
		"anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2",
	})
	if fixture.FullStackUpgrade != nil {
		fixture.FullStackUpgrade.Activate = false
	}
	return fixture, sjconfig.ValidateRuntimeStackManifest(fixture)
}

func useLegacy125RuntimeFixture(driver *Driver) *Driver {
	driver.runtimeManifestProvider = legacy125RuntimeManifest
	return driver
}

func legacy125RuntimeInspection(dataDir, state string) sjconfig.RuntimeStackInspection {
	return useLegacy125RuntimeFixture(New(nil, nil, nil, nil)).inspectRuntimeStack(registry.Instance{DataDir: dataDir, State: state})
}

func legacy125ManagedRuntimeInspection(dataDir, state string) sjconfig.RuntimeStackInspection {
	return useLegacy125RuntimeFixture(New(nil, nil, nil, nil)).inspectManagedRuntimeStack(registry.Instance{DataDir: dataDir, State: state})
}

func legacy125ConfigInspection(dataDir string, installed bool) sjconfig.RuntimeStackInspection {
	manifest, err := legacy125RuntimeManifest()
	if err != nil {
		return sjconfig.RuntimeStackInspection{Status: sjconfig.RuntimeStackStatusInvalidConfig, Code: "invalid_config/manifest"}
	}
	return sjconfig.InspectRuntimeStackWithManifest(dataDir, installed, true, manifest)
}

func legacy125RepairPlan(instance registry.Instance) *RuntimeUpdateRepairPlan {
	return DetectRuntimeUpdateRepairPlanWithInspection(instance, legacy125ManagedRuntimeInspection(instance.DataDir, instance.State))
}
