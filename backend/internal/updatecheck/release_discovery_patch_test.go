package updatecheck

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBridgeClientDiscoversCurrentAndFuturePatchFromReleaseList(t *testing.T) {
	policy, err := os.ReadFile(filepath.Join("..", "..", "..", "deploy", "updates", "0.8.1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var currentPolicy ReleaseContent
	if err := json.Unmarshal(policy, &currentPolicy); err != nil {
		t.Fatal(err)
	}
	if currentPolicy.Version != "0.8.1" || currentPolicy.MinimumUpdaterVersion != BridgeVersion {
		t.Fatal("published patch policy is not consumable by the bridge")
	}
	for _, latest := range []string{"0.8.1", "0.8.2"} {
		t.Run(latest, func(t *testing.T) {
			futurePolicy := currentPolicy
			futurePolicy.Version, futurePolicy.Title = latest, "v"+latest
			encoded, err := json.Marshal(futurePolicy)
			if err != nil {
				t.Fatal(err)
			}
			releases := []githubRelease{
				{TagName: "v0.7.3"},
				{TagName: "v0.8.0"},
				{TagName: "v99.0.0", Draft: true},
				{TagName: "v98.0.0", Prerelease: true},
				{TagName: "v0.9.0", Body: releaseMetadataStart + `{"schemaVersion":1,"minimumUpdaterVersion":"0.8.0"}` + releaseMetadataEnd},
				{TagName: "v" + latest, Body: releaseMetadataStart + string(encoded) + releaseMetadataEnd},
			}
			payload, err := json.Marshal(releases)
			if err != nil {
				t.Fatal(err)
			}
			requests := 0
			client := roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests++
				if req.URL.String() != strings.TrimSuffix(defaultLatestReleaseURL, "/latest")+"?per_page=100" {
					t.Fatalf("bridge client followed the legacy latest pointer: %s", req.URL.String())
				}
				return jsonResponse(string(payload)), nil
			})
			status := New(Options{CurrentVersion: BridgeVersion, Client: client}).Check(context.Background())
			if requests != 1 || status.CheckStatus != StatusOK || status.LatestVersion != "v"+latest || !status.UpdateAvailable || status.Content.MinimumUpdaterVersion != BridgeVersion {
				t.Fatalf("bridge failed to discover the newest applicable patch: %+v", status)
			}
			if status.NoticeDigest == "" || status.ValidateAction(status.LatestVersion, status.NoticeDigest, true) != nil {
				t.Fatal("discovered patch lost its exact acknowledged release policy")
			}
		})
	}
}
