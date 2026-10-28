package config

import (
	"errors"
	"strings"
)

const Junimo140SteamAuthTag = "1.5.0-preview.140-anxi.1"

const Junimo140ServerDigest = "sha256:f23fdc3a3e999032be363fa43291c2d72626555913ef2b3dcf71b0aec53c49db"

// A rolled-back .125 world keeps its original reviewed Auth protocol even
// after the Panel's fresh-install default advances to .140.
func Junimo125SteamAuthTarget() RuntimeStackManifestComponent {
	images := []string{
		"docker.1ms.run/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2",
		"crpi-9z3bkb9g7fxeohrg.cn-hangzhou.personal.cr.aliyuncs.com/anxi-panel/junimo-steam-service-cn:1.5.0-anxi.2",
		"docker.m.daocloud.io/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2",
		"ghcr.io/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2",
		"anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2",
	}
	digests := map[string]string{}
	for _, image := range images {
		digests[image] = "sha256:99420ab30c09da019c425defd4d01796403ac03898ed261b9ee2a976f4bc6518"
	}
	return RuntimeStackManifestComponent{Tag: "1.5.0-anxi.2", UpstreamRef: "refs/tags/preview-1.5.0.121", SourceRevision: "a6cee498f91b318cfcacf035b05b0644bd16829a",
		Images: images, Image: images[0], TrustedCandidates: append([]string{}, images...), Digests: digests}
}

// The explicit migration target is separate from the default installation
// matrix. A bridge release omits it and cannot offer a game migration.
type RuntimeStackUpgradeTarget struct {
	PanelVersion string                        `json:"panelVersion"`
	Activate     bool                          `json:"activate"`
	Server       RuntimeStackManifestComponent `json:"server"`
	SteamAuth    RuntimeStackManifestComponent `json:"steamAuth"`
}

func validateFullStackUpgradeTarget(target *RuntimeStackUpgradeTarget) error {
	if target == nil {
		return nil
	}
	if !PanelVersionSatisfies(target.PanelVersion, target.PanelVersion) {
		return errors.New("full-stack upgrade requires an exact coordinated Panel version")
	}
	if err := validateManifestComponent("server", target.Server); err != nil {
		return err
	}
	if err := validateManifestComponent("steamAuth", target.SteamAuth); err != nil {
		return err
	}
	if target.Server.Tag != "1.5.0-preview.140" || !HasJunimo140SteamAuthTarget(target.SteamAuth) {
		return errors.New("full-stack upgrade requires the fixed Junimo .140 and compatible SteamAuth target")
	}
	for _, digest := range target.Server.Digests {
		if digest != Junimo140ServerDigest {
			return errors.New("full-stack upgrade server content does not match Junimo .140")
		}
	}
	if target.SteamAuth.UpstreamRef != "f318717ac06a780b6b21080b0263c16278d8e6e1" || target.SteamAuth.SourceRevision != "db3f7d53b8e7705b01bbf5a87633e07dabdd34a4" {
		return errors.New("full-stack SteamAuth requires exact upstream and source revisions")
	}
	return nil
}

func HasJunimo140SteamAuthTarget(component RuntimeStackManifestComponent) bool {
	if component.Tag != Junimo140SteamAuthTag || len(component.Images) == 0 {
		return false
	}
	var digest string
	for _, image := range component.Images {
		candidate := component.Digests[image]
		if !validRuntimeDigest(candidate) || candidate == "sha256:99420ab30c09da019c425defd4d01796403ac03898ed261b9ee2a976f4bc6518" || digest != "" && candidate != digest {
			return false
		}
		if candidate != "sha256:4649205b292a89dee5073614408bc7fdbbe01886a597beb38d24e906ddba20f6" {
			return false
		}
		digest = candidate
	}
	return true
}

// RuntimeUpgradePolicy describes the game migration shipped by this Panel.
// An ordinary Panel release must not authorize a different game stack.
type RuntimeUpgradePolicy struct {
	RequiresScopeConfirmation bool   `json:"requiresScopeConfirmation"`
	TargetPanelVersion        string `json:"targetPanelVersion,omitempty"`
	ServerVersion             string `json:"serverVersion"`
	AuthVersion               string `json:"authVersion"`
}

func RuntimeUpgradePolicyForManifest(manifest RuntimeStackManifest) RuntimeUpgradePolicy {
	server, auth := manifest.Server, manifest.SteamAuth
	activate, panelVersion := server.Tag == "1.5.0-preview.140", ""
	if manifest.FullStackUpgrade != nil {
		server, auth = manifest.FullStackUpgrade.Server, manifest.FullStackUpgrade.SteamAuth
		activate, panelVersion = manifest.FullStackUpgrade.Activate, manifest.FullStackUpgrade.PanelVersion
	}
	return RuntimeUpgradePolicy{
		RequiresScopeConfirmation: activate,
		TargetPanelVersion:        panelVersion,
		ServerVersion:             server.Tag,
		AuthVersion:               auth.Tag,
	}
}

// A bridge coordinates only its declared forward target. Unknown future
// releases use an ordinary Panel update followed by that release's own scope
// confirmation. Existing authorized operations remain independently resumable.
func (p RuntimeUpgradePolicy) AppliesToPanelVersion(current, target string) bool {
	normalize := func(version string) string {
		return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(version)), "v")
	}
	current, target = normalize(current), normalize(target)
	return target != "" && (p.TargetPanelVersion != "" && target == normalize(p.TargetPanelVersion) || p.RequiresScopeConfirmation && target == current)
}

func BuiltInRuntimeUpgradePolicy() (RuntimeUpgradePolicy, error) {
	manifest, err := BuiltInRuntimeStackManifest()
	if err != nil {
		return RuntimeUpgradePolicy{}, err
	}
	return RuntimeUpgradePolicyForManifest(manifest), nil
}
