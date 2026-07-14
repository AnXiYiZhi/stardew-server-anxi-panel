package stardew_junimo

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func manifestFixture(appID, buildID, stateFlags, installDir string) []byte {
	return []byte("\ufeff \n\t\"AppState\"\n{\n\"installdir\" \"" + installDir + "\"\n\"StateFlags\"\t\"" + stateFlags + "\"\n\"buildid\" \"" + buildID + "\"\n\"LastUpdated\" \"1734826775\"\n\"appid\" \"" + appID + "\"\n\"Nested\" { \"ignored\" \"escaped \\\"value\\\" and \\\\ path\" }\n}\n")
}

func TestParseSteamAppManifestHandlesOrderWhitespaceAndEscapes(t *testing.T) {
	got, err := parseSteamAppManifest(manifestFixture("413150", "16826371", "4", "Stardew Valley"), "413150")
	if err != nil {
		t.Fatal(err)
	}
	if got.AppID != "413150" || got.BuildID != "16826371" || got.StateFlags != "4" || got.InstallDir != "Stardew Valley" || got.LastUpdated != "1734826775" {
		t.Fatalf("unexpected manifest: %#v", got)
	}
}

func TestParseSteamAppManifestRejectsWrongAppIDAndInvalidBuildID(t *testing.T) {
	for name, data := range map[string][]byte{
		"wrong app":     manifestFixture("1007", "16826371", "4", "Stardew Valley"),
		"missing build": manifestFixture("413150", "", "4", "Stardew Valley"),
		"invalid build": manifestFixture("413150", "latest", "4", "Stardew Valley"),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseSteamAppManifest(data, "413150"); err == nil {
				t.Fatal("invalid manifest accepted")
			}
		})
	}
}

func TestRuntimeComponentMatrixStatuses(t *testing.T) {
	matrix, err := sjconfig.BuiltInRuntimeStackManifest()
	if err != nil {
		t.Fatal(err)
	}
	game := manifestFixture("413150", matrix.Game.BuildID, "4", "Stardew Valley")
	sdk := manifestFixture("1007", matrix.SDK.BuildID, "4", "Steamworks Shared")
	tests := []struct {
		name      string
		game, sdk []byte
		want      string
	}{
		{"matching", game, sdk, RuntimeComponentsStatusUpToDate},
		{"game old sdk matching", manifestFixture("413150", "111", "4", "Stardew Valley"), sdk, RuntimeComponentsStatusUpdateAvailable},
		{"game matching sdk old", game, manifestFixture("1007", "222", "4", "Steamworks Shared"), RuntimeComponentsStatusUpdateAvailable},
		{"both mismatch", manifestFixture("413150", "111", "4", "Stardew Valley"), manifestFixture("1007", "222", "4", "Steamworks Shared"), RuntimeComponentsStatusUpdateAvailable},
		{"game missing", nil, sdk, RuntimeComponentsStatusGameMissing},
		{"sdk missing", game, nil, RuntimeComponentsStatusSDKMissing},
		{"invalid", manifestFixture("1007", "111", "4", "wrong"), sdk, RuntimeComponentsStatusManifestInvalid},
		{"unknown state", manifestFixture("413150", matrix.Game.BuildID, "2", "Stardew Valley"), sdk, RuntimeComponentsStatusCustomUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inspectRuntimeComponentData(matrix, tt.game, tt.sdk, "2026-07-14T00:00:00Z")
			if got.Status != tt.want {
				t.Fatalf("status=%s want=%s (%s)", got.Status, tt.want, got.Reason)
			}
			if tt.want == RuntimeComponentsStatusUpdateAvailable && !got.Available {
				t.Fatal("tested mismatch must be available")
			}
		})
	}
}

func TestRuntimeComponentsUninstalledIsNotServerError(t *testing.T) {
	driver := New(nil, nil, nil, nil)
	got, err := driver.InspectRuntimeComponents(context.Background(), registry.Instance{State: storage.InstanceStateUninitialized})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != RuntimeComponentsStatusGameMissing || got.Code != "not_installed" {
		t.Fatalf("unexpected: %#v", got)
	}
}

func TestRuntimeComponentJSONDoesNotExposeManifestOrSecrets(t *testing.T) {
	matrix, _ := sjconfig.BuiltInRuntimeStackManifest()
	game := append(manifestFixture("413150", matrix.Game.BuildID, "4", "Stardew Valley"), []byte("password secret-token")...)
	got := inspectRuntimeComponentData(matrix, game, manifestFixture("1007", matrix.SDK.BuildID, "4", "Steamworks Shared"), "")
	data, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(data))
	for _, forbidden := range []string{"password", "secret-token", "refresh_token", "steam_username"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("response leaked %q: %s", forbidden, data)
		}
	}
}
