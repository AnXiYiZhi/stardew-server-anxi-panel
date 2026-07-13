package updatecheck

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) Do(req *http.Request) (*http.Response, error) { return fn(req) }

func jsonResponse(body string) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}
}

func TestSemanticVersionComparison(t *testing.T) {
	tests := []struct {
		current, latest string
		want            int
	}{
		{"v0.1.14", "0.1.14", 0},
		{"0.1.13", "v0.1.14", -1},
		{"v1.10.0", "1.9.9", 1},
		{"1.0.0-rc.2", "1.0.0-rc.10", -1},
		{"1.0.0-rc.1", "1.0.0", -1},
	}
	for _, tt := range tests {
		t.Run(tt.current+"_"+tt.latest, func(t *testing.T) {
			current, ok := parseSemver(tt.current)
			if !ok {
				t.Fatalf("parse current %q", tt.current)
			}
			latest, ok := parseSemver(tt.latest)
			if !ok {
				t.Fatalf("parse latest %q", tt.latest)
			}
			got := compareSemver(current, latest)
			if got < 0 {
				got = -1
			} else if got > 0 {
				got = 1
			}
			if got != tt.want {
				t.Fatalf("compare = %d, want %d", got, tt.want)
			}
		})
	}

	for _, invalid := range []string{"", "dev", "v1.2", "1.2.3.4", "01.2.3", "1.2.3-"} {
		if _, ok := parseSemver(invalid); ok {
			t.Fatalf("%q unexpectedly parsed", invalid)
		}
	}
}

func TestCheckIgnoresDraftAndPrerelease(t *testing.T) {
	checkedAt := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	client := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.UserAgent() == "" {
			t.Fatal("missing user agent")
		}
		return jsonResponse(`[
			{"tag_name":"v9.0.0","html_url":"https://example/draft","draft":true,"prerelease":false,"published_at":"2026-07-13T10:00:00Z"},
			{"tag_name":"v2.0.0-rc.1","html_url":"https://example/rc","draft":false,"prerelease":true,"published_at":"2026-07-13T09:00:00Z"},
			{"tag_name":"v0.1.15","html_url":"https://example/stable","draft":false,"prerelease":false,"published_at":"2026-07-12T08:00:00Z"}
		]`), nil
	})
	svc := New(Options{CurrentVersion: "0.1.14", Commit: "abc", BuildDate: "today", Client: client, Now: func() time.Time { return checkedAt }})
	status := svc.Check(context.Background())
	if status.CheckStatus != StatusOK || !status.UpdateAvailable || status.LatestVersion != "v0.1.15" {
		t.Fatalf("unexpected status: %+v", status)
	}
	if status.ReleaseURL != "https://example/stable" || status.CheckedAt == nil || !status.CheckedAt.Equal(checkedAt) {
		t.Fatalf("unexpected release metadata: %+v", status)
	}
}

func TestNetworkFailureRetainsLastSuccessfulResult(t *testing.T) {
	calls := 0
	client := roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return jsonResponse(`[{"tag_name":"v0.2.0","html_url":"https://example/release","draft":false,"prerelease":false,"published_at":"2026-07-12T08:00:00Z"}]`), nil
		}
		return nil, errors.New("temporary network outage")
	})
	svc := New(Options{CurrentVersion: "v0.1.14", Client: client})
	first := svc.Check(context.Background())
	second := svc.Check(context.Background())
	if first.CheckStatus != StatusOK || !first.UpdateAvailable {
		t.Fatalf("first = %+v", first)
	}
	if second.CheckStatus != StatusError || !second.UpdateAvailable {
		t.Fatalf("second = %+v", second)
	}
	if second.LatestVersion != first.LatestVersion || second.ReleaseURL != first.ReleaseURL {
		t.Fatalf("failed check discarded successful cache: first=%+v second=%+v", first, second)
	}
	if second.CheckedAt == nil || first.CheckedAt == nil || !second.CheckedAt.Equal(*first.CheckedAt) {
		t.Fatalf("failed check changed successful checkedAt: first=%+v second=%+v", first, second)
	}
}

func TestInvalidDevelopmentVersionNeverChecksOrReportsUpdate(t *testing.T) {
	called := false
	client := roundTripFunc(func(*http.Request) (*http.Response, error) {
		called = true
		return nil, errors.New("must not be called")
	})
	for _, version := range []string{"dev", "", "not-a-version"} {
		svc := New(Options{CurrentVersion: version, Client: client})
		status := svc.Check(context.Background())
		if status.CheckStatus != StatusUnavailable || status.UpdateAvailable || status.LatestVersion != "" {
			t.Fatalf("version %q status = %+v", version, status)
		}
	}
	if called {
		t.Fatal("invalid current version triggered network request")
	}
}

func TestDefaultClientUsesNetDNSFallbackTransport(t *testing.T) {
	svc := New(Options{CurrentVersion: "1.0.0"})
	client, ok := svc.client.(*http.Client)
	if !ok || client.Transport == nil {
		t.Fatalf("default client = %#v", svc.client)
	}
	if client.Transport == http.DefaultTransport {
		t.Fatal("default client unexpectedly uses raw http.DefaultTransport")
	}
}
