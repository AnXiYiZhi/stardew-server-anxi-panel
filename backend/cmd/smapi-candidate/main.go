// Command smapi-candidate discovers a SMAPI release candidate for maintainers.
// It never changes the embedded recommendation, tags, or a running instance.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/netdns"
)

const (
	releasesAPI = "https://api.github.com/repos/Pathoschild/SMAPI/releases"
	maxAsset    = int64(100 * 1024 * 1024)
)

var releasesAPIForTest = releasesAPI

type release struct {
	TagName     string  `json:"tag_name"`
	Name        string  `json:"name"`
	Draft       bool    `json:"draft"`
	Prerelease  bool    `json:"prerelease"`
	PublishedAt string  `json:"published_at"`
	HTMLURL     string  `json:"html_url"`
	Assets      []asset `json:"assets"`
}

type asset struct {
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Digest             string `json:"digest"`
}

type candidate struct {
	SchemaVersion int    `json:"schemaVersion"`
	DiscoveredAt  string `json:"discoveredAt"`
	Repository    string `json:"repository"`
	ManualTag     bool   `json:"manualTag"`
	Version       string `json:"version"`
	ReleaseURL    string `json:"releaseUrl"`
	PublishedAt   string `json:"publishedAt"`
	Prerelease    bool   `json:"prerelease"`
	Installer     struct {
		Name      string `json:"name"`
		URL       string `json:"url"`
		Bytes     int64  `json:"bytes"`
		SHA256    string `json:"sha256"`
		APIDigest string `json:"apiDigest,omitempty"`
	} `json:"installer"`
}

func main() {
	output := flag.String("output", "smapi-candidate.json", "candidate JSON output path")
	manualTag := flag.String("tag", "", "explicit release tag; may select a prerelease")
	flag.Parse()
	if err := run(*output, strings.TrimSpace(*manualTag)); err != nil {
		fmt.Fprintln(os.Stderr, "SMAPI candidate discovery failed; previous candidate was preserved:", err)
		os.Exit(1)
	}
}

func run(output, manualTag string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	client := netdns.NewClient(2 * time.Minute)
	selected, err := discover(ctx, client, manualTag)
	if err != nil {
		return err
	}
	version := strings.TrimPrefix(selected.TagName, "v")
	wantName := "SMAPI-" + version + "-installer.zip"
	var installer *asset
	for i := range selected.Assets {
		if selected.Assets[i].Name == wantName {
			installer = &selected.Assets[i]
			break
		}
	}
	if installer == nil {
		return fmt.Errorf("official installer asset %q is missing", wantName)
	}
	if installer.Size <= 0 || installer.Size > maxAsset {
		return fmt.Errorf("installer size %d is outside the allowed range", installer.Size)
	}
	if err := validateOfficialAssetURL(installer.BrowserDownloadURL, selected.TagName, wantName); err != nil {
		return err
	}
	hash, bytes, err := downloadHash(ctx, client, installer.BrowserDownloadURL, installer.Size)
	if err != nil {
		return err
	}
	if strings.HasPrefix(installer.Digest, "sha256:") && !strings.EqualFold(strings.TrimPrefix(installer.Digest, "sha256:"), hash) {
		return errors.New("downloaded SHA256 does not match GitHub asset digest")
	}
	result := candidate{SchemaVersion: 1, DiscoveredAt: time.Now().UTC().Format(time.RFC3339), Repository: "Pathoschild/SMAPI", ManualTag: manualTag != "", Version: version, ReleaseURL: selected.HTMLURL, PublishedAt: selected.PublishedAt, Prerelease: selected.Prerelease}
	result.Installer.Name, result.Installer.URL, result.Installer.Bytes, result.Installer.SHA256, result.Installer.APIDigest = wantName, installer.BrowserDownloadURL, bytes, hash, installer.Digest
	return writeAtomic(output, result)
}

func discover(ctx context.Context, client *http.Client, manualTag string) (release, error) {
	if manualTag != "" {
		var selected release
		err := getJSON(ctx, client, releasesAPIForTest+"/tags/"+url.PathEscape(manualTag), &selected)
		return selected, err
	}
	var releases []release
	if err := getJSON(ctx, client, releasesAPIForTest+"?per_page=100", &releases); err != nil {
		return release{}, err
	}
	stable := releases[:0]
	for _, item := range releases {
		if !item.Draft && !item.Prerelease && parseVersion(item.TagName) != nil {
			stable = append(stable, item)
		}
	}
	if len(stable) == 0 {
		return release{}, errors.New("GitHub returned no stable SMAPI releases")
	}
	sort.SliceStable(stable, func(i, j int) bool {
		return compareVersion(parseVersion(stable[i].TagName), parseVersion(stable[j].TagName)) > 0
	})
	return stable[0], nil
}

func getJSON(ctx context.Context, client *http.Client, endpoint string, target any) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "anxi-panel-smapi-candidate")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub API returned HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 8*1024*1024)).Decode(target)
}

func downloadHash(ctx context.Context, client *http.Client, endpoint string, expected int64) (string, int64, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	req.Header.Set("User-Agent", "anxi-panel-smapi-candidate")
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("installer download returned HTTP %d", resp.StatusCode)
	}
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(resp.Body, maxAsset+1))
	if err != nil {
		return "", n, err
	}
	if n > maxAsset || n != expected {
		return "", n, fmt.Errorf("installer length %d does not match expected %d", n, expected)
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

func validateOfficialAssetURL(raw, tag, name string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || !strings.EqualFold(u.Hostname(), "github.com") || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("installer URL is not an official GitHub release URL")
	}
	want := "/Pathoschild/SMAPI/releases/download/" + tag + "/" + name
	if !strings.EqualFold(u.EscapedPath(), want) {
		return errors.New("installer URL path does not match the selected release")
	}
	return nil
}

func parseVersion(raw string) []int {
	parts := strings.Split(strings.TrimPrefix(strings.TrimSpace(raw), "v"), ".")
	if len(parts) < 3 || len(parts) > 4 {
		return nil
	}
	values := make([]int, len(parts))
	for i, part := range parts {
		value, err := strconv.Atoi(part)
		if err != nil || value < 0 {
			return nil
		}
		values[i] = value
	}
	return values
}

func compareVersion(a, b []int) int {
	for i := 0; i < 4; i++ {
		av, bv := 0, 0
		if i < len(a) {
			av = a[i]
		}
		if i < len(b) {
			bv = b[i]
		}
		if av < bv {
			return -1
		}
		if av > bv {
			return 1
		}
	}
	return 0
}

func writeAtomic(path string, value any) error {
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".smapi-candidate-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
