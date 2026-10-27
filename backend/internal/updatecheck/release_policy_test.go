package updatecheck

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSequentialReleaseSelection(t *testing.T) {
	releases := []githubRelease{
		{TagName: "v0.8.0", Body: releaseMetadataStart + `{"schemaVersion":1,"title":"实例大版本更新","notices":[{"level":"warning","title":"停服维护","message":"保存、备份并重启世界","requiresConfirmation":true}]}` + releaseMetadataEnd},
		{TagName: "v0.7.3", Body: releaseMetadataStart + `{"schemaVersion":1,"title":"升级准备"}` + releaseMetadataEnd},
		{TagName: "v8.0.0", Draft: true},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _ = json.NewEncoder(w).Encode(releases) }))
	defer server.Close()
	for _, test := range []struct{ current, expected string }{{"0.7.2", "v0.7.3"}, {"0.7.3", "v0.8.0"}, {"0.8.0", "v0.8.0"}} {
		t.Run(test.current, func(t *testing.T) {
			status := New(Options{CurrentVersion: test.current, LatestReleaseURL: server.URL}).Check(context.Background())
			if status.CheckStatus != StatusOK || status.LatestVersion != test.expected {
				t.Fatalf("unexpected release: %+v", status)
			}
			if test.current == "0.7.3" {
				if status.NoticeDigest == "" || status.ValidateAction(status.LatestVersion, "", true) == nil {
					t.Fatal("important notice did not block confirmation")
				}
				if err := status.ValidateAction(status.LatestVersion, status.NoticeDigest, true); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestReleasePolicyFailsClosed(t *testing.T) {
	if ValidateTransition("0.7.2", "v0.8.0") == nil {
		t.Fatal("legacy bypass accepted")
	}
	if err := ValidateTransition("0.7.3", "v0.8.0"); err != nil {
		t.Fatal(err)
	}
	content, err := releaseContent(githubRelease{TagName: "v0.8.0", Body: releaseMetadataStart + `{"schemaVersion":1,"blocked":true}` + releaseMetadataEnd})
	if err != nil || !content.Blocked || content.MinimumUpdaterVersion != BridgeVersion {
		t.Fatal("release guard missing")
	}
	if _, err := releaseContent(githubRelease{Body: releaseMetadataStart + "broken"}); err == nil {
		t.Fatal("malformed policy accepted")
	}
	status := Status{CurrentVersion: "0.7.3", LatestVersion: "v0.8.0", UpdateAvailable: true, CheckStatus: StatusError}
	if status.ValidateAction(status.LatestVersion, "", false) == nil {
		t.Fatal("stale check accepted")
	}
}

func TestLegacyLatestCannotSkipBridge(t *testing.T) {
	for _, returned := range []string{"v0.7.3", "v0.8.0"} {
		t.Run(returned, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tag := "v0.8.0"
				if r.URL.Path == "/tags/v0.7.3" {
					tag = returned
				}
				_ = json.NewEncoder(w).Encode(githubRelease{TagName: tag})
			}))
			defer server.Close()
			status := New(Options{CurrentVersion: "0.7.2", LatestReleaseURL: server.URL + "/latest"}).Check(context.Background())
			if returned == "v0.7.3" && (status.CheckStatus != StatusOK || status.LatestVersion != returned) {
				t.Fatal("bridge was not selected")
			}
			if returned != "v0.7.3" && (status.CheckStatus != StatusError || status.UpdateAvailable) {
				t.Fatal("mismatched bridge was accepted")
			}
		})
	}
}

func TestReleasePolicyAcceptsGitHubCRLFAndBindsVersion(t *testing.T) {
	body := releaseMetadataStart + `{"schemaVersion":1,"version":"0.8.0","blocked":true}` + releaseMetadataEnd
	content, err := releaseContent(githubRelease{TagName: "v0.8.0", Body: strings.ReplaceAll(body, "\n", "\r\n")})
	if err != nil || !content.Blocked {
		t.Fatal("CRLF policy was not recognized", err)
	}
	if _, err := releaseContent(githubRelease{TagName: "v0.8.1", Body: body}); err == nil {
		t.Fatal("notes for a different version were accepted")
	}
}
