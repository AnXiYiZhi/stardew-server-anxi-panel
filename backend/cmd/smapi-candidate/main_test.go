package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseAndCompareVersion(t *testing.T) {
	if compareVersion(parseVersion("v4.5.2"), parseVersion("4.5.1")) <= 0 {
		t.Fatal("stable semantic comparison failed")
	}
	if parseVersion("4.6.0-beta") != nil {
		t.Fatal("prerelease tag accepted as stable dotted version")
	}
}

func TestValidateOfficialAssetURL(t *testing.T) {
	good := "https://github.com/Pathoschild/SMAPI/releases/download/4.5.2/SMAPI-4.5.2-installer.zip"
	if err := validateOfficialAssetURL(good, "4.5.2", "SMAPI-4.5.2-installer.zip"); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{
		"https://example.com/Pathoschild/SMAPI/releases/download/4.5.2/SMAPI-4.5.2-installer.zip",
		"https://github.com/Pathoschild/SMAPI/releases/download/4.5.1/SMAPI-4.5.2-installer.zip",
		good + "?token=secret",
	} {
		if err := validateOfficialAssetURL(raw, "4.5.2", "SMAPI-4.5.2-installer.zip"); err == nil {
			t.Fatalf("unsafe URL accepted: %s", raw)
		}
	}
}

func TestDiscoveryFiltersDraftAndPrerelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"tag_name":"4.7.0","draft":true},
			{"tag_name":"4.6.0","prerelease":true},
			{"tag_name":"4.5.1","draft":false,"prerelease":false},
			{"tag_name":"4.5.2","draft":false,"prerelease":false}
		]`))
	}))
	defer server.Close()
	old := releasesAPIForTest
	releasesAPIForTest = server.URL
	defer func() { releasesAPIForTest = old }()
	got, err := discover(context.Background(), server.Client(), "")
	if err != nil {
		t.Fatal(err)
	}
	if got.TagName != "4.5.2" {
		t.Fatalf("selected %s", got.TagName)
	}
}

func TestFailedWritePreservesPreviousCandidate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "candidate.json")
	if err := os.WriteFile(path, []byte("previous"), 0o600); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(path, "child")
	if err := writeAtomic(bad, candidate{}); err == nil {
		t.Fatal("invalid destination unexpectedly succeeded")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != "previous" {
		t.Fatal("previous candidate was changed")
	}
}

func TestDiscoveryFailureDoesNotModifyRecommendedMatrix(t *testing.T) {
	matrix := filepath.Join("..", "..", "internal", "games", "stardew_junimo", "config", "runtime_stack_manifest.json")
	before, err := os.ReadFile(matrix)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "temporary GitHub failure", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	old := releasesAPIForTest
	releasesAPIForTest = server.URL
	defer func() { releasesAPIForTest = old }()
	output := filepath.Join(t.TempDir(), "candidate.json")
	if err := run(output, ""); err == nil {
		t.Fatal("failed GitHub lookup was reported as success")
	}
	after, err := os.ReadFile(matrix)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("candidate discovery modified the tested recommendation matrix")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("failed discovery wrote a candidate: %v", err)
	}
}
