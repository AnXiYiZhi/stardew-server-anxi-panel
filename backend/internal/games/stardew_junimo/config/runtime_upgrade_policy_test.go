package config

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRuntimeUpgradePolicySeparatesBridgeAndGameMigration(t *testing.T) {
	for _, tc := range []struct {
		name, server, auth string
		confirmation       bool
	}{
		{"bridge", "1.5.0-preview.125", "1.5.0-anxi.2", false},
		{"junimo140", "1.5.0-preview.140", Junimo140SteamAuthTag, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manifest := RuntimeStackManifest{
				Server:    RuntimeStackManifestComponent{Tag: tc.server},
				SteamAuth: RuntimeStackManifestComponent{Tag: tc.auth},
			}
			policy := RuntimeUpgradePolicyForManifest(manifest)
			if policy.RequiresScopeConfirmation != tc.confirmation || policy.ServerVersion != tc.server || policy.AuthVersion != tc.auth {
				t.Fatalf("release policy does not describe its embedded target: %+v", policy)
			}
		})
	}
}

func TestExplicitRuntimeUpgradeTargetKeepsDefaultInstallPair(t *testing.T) {
	manifest, err := BuiltInRuntimeStackManifest()
	if err != nil {
		t.Fatal(err)
	}
	if manifest.FullStackUpgrade == nil {
		t.Fatal("migration release has no explicit target")
	}
	policy := RuntimeUpgradePolicyForManifest(manifest)
	if policy.RequiresScopeConfirmation != manifest.FullStackUpgrade.Activate || policy.TargetPanelVersion != manifest.FullStackUpgrade.PanelVersion || policy.ServerVersion != "1.5.0-preview.140" || policy.AuthVersion != Junimo140SteamAuthTag {
		t.Fatalf("explicit migration not described by release: %+v", policy)
	}
	manifest.FullStackUpgrade = nil
	bridge := RuntimeUpgradePolicyForManifest(manifest)
	if bridge.RequiresScopeConfirmation || bridge.ServerVersion != "1.5.0-preview.125" || bridge.AuthVersion != "1.5.0-anxi.2" {
		t.Fatalf("bridge changed its default installation pair: %+v", bridge)
	}
}

func TestRuntimeUpgradePolicyBindsBridgeToKnownForwardRelease(t *testing.T) {
	for _, tc := range []struct {
		name, current, target string
		activate, allowed     bool
	}{
		{"bridge current patch", "0.7.3", "0.7.3", false, false},
		{"bridge unrelated patch", "0.7.3", "0.7.4", false, false},
		{"bridge declared major", "0.7.3", "v0.8.0", false, true},
		{"bridge unknown future", "0.7.3", "0.8.1", false, false},
		{"major continuation", "0.8.0", "0.8.0", true, true},
		{"future continuation", "0.8.1", "0.8.1", true, true},
		{"major unknown target", "0.8.0", "0.8.1", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			policy := RuntimeUpgradePolicy{RequiresScopeConfirmation: tc.activate, TargetPanelVersion: "0.8.0"}
			if got := policy.AppliesToPanelVersion(tc.current, tc.target); got != tc.allowed {
				t.Fatalf("upgrade target applicability = %v, want %v", got, tc.allowed)
			}
		})
	}
}

func TestExplicitRuntimeUpgradeRejectsIncompleteContent(t *testing.T) {
	builtIn, err := BuiltInRuntimeStackManifest()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(builtIn)
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"valid", "server content", "auth content", "auth tag", "source revision", "upstream revision", "untrusted auth repository"} {
		t.Run(scenario, func(t *testing.T) {
			var manifest RuntimeStackManifest
			if err := json.Unmarshal(raw, &manifest); err != nil {
				t.Fatal(err)
			}
			target := manifest.FullStackUpgrade
			switch scenario {
			case "server content":
				for image := range target.Server.Digests {
					target.Server.Digests[image] = "sha256:" + strings.Repeat("b", 64)
				}
			case "auth content":
				for image := range target.SteamAuth.Digests {
					target.SteamAuth.Digests[image] = "sha256:99420ab30c09da019c425defd4d01796403ac03898ed261b9ee2a976f4bc6518"
				}
			case "auth tag":
				target.SteamAuth.Tag = "1.5.0-anxi.2"
			case "source revision":
				target.SteamAuth.SourceRevision = "main"
			case "upstream revision":
				target.SteamAuth.UpstreamRef = "refs/heads/main"
			case "untrusted auth repository":
				target.SteamAuth.Images = []string{"evil.example/steam-auth:1.5.0-anxi.3"}
				target.SteamAuth.Digests = map[string]string{target.SteamAuth.Images[0]: "sha256:" + strings.Repeat("b", 64)}
			}
			if err := ValidateRuntimeStackManifest(manifest); (err == nil) != (scenario == "valid") {
				t.Fatalf("target validation for %s: %v", scenario, err)
			}
		})
	}
}

func TestJunimo140AuthTargetRejectsMissingOrRelabeledLegacyContent(t *testing.T) {
	for _, scenario := range []string{"reviewed", "legacy tag", "missing digest", "different mirrors", "legacy content"} {
		t.Run(scenario, func(t *testing.T) {
			component := RuntimeStackManifestComponent{
				Tag:     Junimo140SteamAuthTag,
				Images:  []string{"ghcr.io/anxiyizhi/junimo-steam-service-cn:" + Junimo140SteamAuthTag, "anxiyizhi/junimo-steam-service-cn:" + Junimo140SteamAuthTag},
				Digests: map[string]string{},
			}
			for _, image := range component.Images {
				component.Digests[image] = "sha256:4649205b292a89dee5073614408bc7fdbbe01886a597beb38d24e906ddba20f6"
			}
			switch scenario {
			case "legacy tag":
				component.Tag = "1.5.0-anxi.2"
			case "missing digest":
				delete(component.Digests, component.Images[0])
			case "different mirrors":
				component.Digests[component.Images[1]] = "sha256:" + strings.Repeat("c", 64)
			case "legacy content":
				for _, image := range component.Images {
					component.Digests[image] = "sha256:99420ab30c09da019c425defd4d01796403ac03898ed261b9ee2a976f4bc6518"
				}
			}
			if got := HasJunimo140SteamAuthTarget(component); got != (scenario == "reviewed") {
				t.Fatalf("Auth target acceptance = %v for %s", got, scenario)
			}
		})
	}
}
