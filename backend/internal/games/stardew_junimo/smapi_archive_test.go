package stardew_junimo

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
)

type smapiArchiveRoundTripFunc func(*http.Request) (*http.Response, error)

func (f smapiArchiveRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type smapiArchiveChunkedReader struct {
	reader *bytes.Reader
	max    int
}

func (r *smapiArchiveChunkedReader) Read(p []byte) (int, error) {
	if len(p) > r.max {
		p = p[:r.max]
	}
	return r.reader.Read(p)
}

func writeSMAPITestZip(t *testing.T, entries map[string]string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "smapi.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	for name, content := range entries {
		entry, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func validSMAPIEntries() map[string]string {
	return map[string]string{
		"SMAPI 4.5.2 installer/internal/linux/SMAPI.Installer":       "linux",
		"SMAPI 4.5.2 installer/internal/windows/SMAPI.Installer.exe": "windows",
	}
}

func TestValidateSMAPIZip(t *testing.T) {
	if err := validateSMAPIZip(writeSMAPITestZip(t, validSMAPIEntries()), 1024); err != nil {
		t.Fatal(err)
	}
}

func TestValidateSMAPIZipRejectsZipSlipAndDamagedArchive(t *testing.T) {
	entries := validSMAPIEntries()
	entries["../escape"] = "bad"
	if err := validateSMAPIZip(writeSMAPITestZip(t, entries), 1024); err == nil {
		t.Fatal("ZIP Slip accepted")
	}
	bad := filepath.Join(t.TempDir(), "bad.zip")
	if err := os.WriteFile(bad, []byte("not a zip"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateSMAPIZip(bad, 1024); err == nil {
		t.Fatal("damaged ZIP accepted")
	}
}

func TestValidateSMAPIZipRejectsOversizedExtraction(t *testing.T) {
	entries := validSMAPIEntries()
	entries["SMAPI 4.5.2 installer/large"] = strings.Repeat("x", 4096)
	if err := validateSMAPIZip(writeSMAPITestZip(t, entries), 2048); err == nil {
		t.Fatal("oversized ZIP accepted")
	}
}

func TestValidateSMAPIManifestURLRejectsNonOfficial(t *testing.T) {
	manifest, err := sjconfig.BuiltInRuntimeStackManifest()
	if err != nil {
		t.Fatal(err)
	}
	if err := validateSMAPIManifestURL(manifest.SMAPI.DownloadURL, manifest); err != nil {
		t.Fatal(err)
	}
	if err := validateSMAPIManifestURL("https://example.com/SMAPI.zip", manifest); err == nil {
		t.Fatal("non-official URL accepted")
	}
}

func TestEnsureRecommendedSMAPIArchiveDownloadsReviewedRanges(t *testing.T) {
	payload := readValidSMAPITestZip(t)
	manifest := smapiArchiveManifestForPayload(payload)
	var requested []string
	opts := smapiArchiveTestDownloadOptions(func(req *http.Request) (*http.Response, error) {
		requested = append(requested, req.Header.Get("Range"))
		start, end := parseRequestedSMAPIRange(t, req.Header.Get("Range"))
		return smapiRangeResponse(payload, start, end, int64(len(payload))), nil
	})
	opts.chunkBytes = 31

	target, err := ensureRecommendedSMAPIArchiveWithOptions(context.Background(), t.TempDir(), manifest, opts)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatal("downloaded SMAPI archive differs from the reviewed payload")
	}
	wantRequests := (len(payload) + int(opts.chunkBytes) - 1) / int(opts.chunkBytes)
	if len(requested) != wantRequests {
		t.Fatalf("range request count=%d, want %d (%v)", len(requested), wantRequests, requested)
	}
	if requested[0] != "bytes=0-30" {
		t.Fatalf("first range=%q", requested[0])
	}
}

func TestEnsureRecommendedSMAPIArchiveReportsStreamingProgress(t *testing.T) {
	payload := readValidSMAPITestZip(t)
	manifest := smapiArchiveManifestForPayload(payload)
	var progress []smapiArchiveDownloadProgress
	opts := smapiArchiveTestDownloadOptions(func(req *http.Request) (*http.Response, error) {
		start, end := parseRequestedSMAPIRange(t, req.Header.Get("Range"))
		resp := smapiRangeResponse(payload, start, end, int64(len(payload)))
		resp.Body = io.NopCloser(&smapiArchiveChunkedReader{
			reader: bytes.NewReader(payload[start : end+1]),
			max:    7,
		})
		return resp, nil
	})
	opts.chunkBytes = int64(len(payload))
	opts.onProgress = func(item smapiArchiveDownloadProgress) {
		progress = append(progress, item)
	}

	if _, err := ensureRecommendedSMAPIArchiveWithOptions(context.Background(), t.TempDir(), manifest, opts); err != nil {
		t.Fatal(err)
	}
	if len(progress) < 3 {
		t.Fatalf("progress updates=%d, want initial, intermediate, and complete updates", len(progress))
	}
	if progress[0].DownloadedBytes != 0 || progress[0].Candidate != 1 || progress[0].CandidateCount != 1 {
		t.Fatalf("initial progress=%+v", progress[0])
	}
	last := progress[len(progress)-1]
	if last.DownloadedBytes != int64(len(payload)) || last.TotalBytes != int64(len(payload)) || last.Cached {
		t.Fatalf("final progress=%+v", last)
	}
	foundIntermediate := false
	for _, item := range progress {
		if item.DownloadedBytes > 0 && item.DownloadedBytes < item.TotalBytes {
			foundIntermediate = true
			break
		}
	}
	if !foundIntermediate {
		t.Fatalf("progress did not include an intermediate byte count: %+v", progress)
	}
}

func TestEnsureRecommendedSMAPIArchiveReportsValidatedCacheHit(t *testing.T) {
	payload := readValidSMAPITestZip(t)
	manifest := smapiArchiveManifestForPayload(payload)
	opts := smapiArchiveTestDownloadOptions(func(req *http.Request) (*http.Response, error) {
		start, end := parseRequestedSMAPIRange(t, req.Header.Get("Range"))
		return smapiRangeResponse(payload, start, end, int64(len(payload))), nil
	})
	dataDir := t.TempDir()
	if _, err := ensureRecommendedSMAPIArchiveWithOptions(context.Background(), dataDir, manifest, opts); err != nil {
		t.Fatal(err)
	}

	var progress []smapiArchiveDownloadProgress
	opts.onProgress = func(item smapiArchiveDownloadProgress) {
		progress = append(progress, item)
	}
	if _, err := ensureRecommendedSMAPIArchiveWithOptions(context.Background(), dataDir, manifest, opts); err != nil {
		t.Fatal(err)
	}
	if len(progress) != 1 || !progress[0].Cached || progress[0].DownloadedBytes != int64(len(payload)) {
		t.Fatalf("cache progress=%+v", progress)
	}
}

func TestEnsureRecommendedSMAPIArchiveResumesAfterPartialRange(t *testing.T) {
	payload := readValidSMAPITestZip(t)
	manifest := smapiArchiveManifestForPayload(payload)
	cut := int64(len(payload) / 3)
	var requested []string
	opts := smapiArchiveTestDownloadOptions(func(req *http.Request) (*http.Response, error) {
		requested = append(requested, req.Header.Get("Range"))
		start, end := parseRequestedSMAPIRange(t, req.Header.Get("Range"))
		resp := smapiRangeResponse(payload, start, end, int64(len(payload)))
		if len(requested) == 1 {
			resp.Body = io.NopCloser(bytes.NewReader(payload[:cut]))
		}
		return resp, nil
	})
	opts.chunkBytes = int64(len(payload))

	target, err := ensureRecommendedSMAPIArchiveWithOptions(context.Background(), t.TempDir(), manifest, opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(requested) != 2 {
		t.Fatalf("ranges=%v, want initial plus resumed request", requested)
	}
	wantSecond := fmt.Sprintf("bytes=%d-%d", cut, len(payload)-1)
	if requested[1] != wantSecond {
		t.Fatalf("resumed range=%q, want %q", requested[1], wantSecond)
	}
	if err := validateRecommendedSMAPIArchive(target, manifest); err != nil {
		t.Fatalf("resumed archive did not pass final validation: %v", err)
	}
}

func TestEnsureRecommendedSMAPIArchiveFallsBackAfterInvalidMirrorArchive(t *testing.T) {
	payload := readValidSMAPITestZip(t)
	corrupt := append([]byte(nil), payload...)
	corrupt[len(corrupt)/2] ^= 0xff
	manifest := smapiArchiveManifestForPayload(payload)
	manifest.SMAPI.URLs = []string{
		"https://mirror.example/SMAPI-installer.zip",
		manifest.SMAPI.DownloadURL,
	}
	manifest.SMAPI.TrustedHosts = []string{"mirror.example", "github.com"}
	var hosts []string
	opts := smapiArchiveTestDownloadOptions(func(req *http.Request) (*http.Response, error) {
		hosts = append(hosts, req.URL.Hostname())
		start, end := parseRequestedSMAPIRange(t, req.Header.Get("Range"))
		candidate := payload
		if req.URL.Hostname() == "mirror.example" {
			candidate = corrupt
		}
		return smapiRangeResponse(candidate, start, end, int64(len(candidate))), nil
	})
	opts.chunkBytes = int64(len(payload))

	target, err := ensureRecommendedSMAPIArchiveWithOptions(context.Background(), t.TempDir(), manifest, opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 2 || hosts[0] != "mirror.example" || hosts[1] != "github.com" {
		t.Fatalf("candidate order=%v", hosts)
	}
	if err := validateRecommendedSMAPIArchive(target, manifest); err != nil {
		t.Fatalf("official fallback archive did not pass validation: %v", err)
	}
}

func TestEnsureRecommendedSMAPIArchiveStopsAfterNoProgress(t *testing.T) {
	payload := readValidSMAPITestZip(t)
	manifest := smapiArchiveManifestForPayload(payload)
	calls := 0
	opts := smapiArchiveTestDownloadOptions(func(req *http.Request) (*http.Response, error) {
		calls++
		start, end := parseRequestedSMAPIRange(t, req.Header.Get("Range"))
		resp := smapiRangeResponse(payload, start, end, int64(len(payload)))
		resp.Body = io.NopCloser(bytes.NewReader(nil))
		return resp, nil
	})
	opts.maxNoProgressAttempts = 3
	opts.chunkBytes = int64(len(payload))
	dataDir := t.TempDir()

	if _, err := ensureRecommendedSMAPIArchiveWithOptions(context.Background(), dataDir, manifest, opts); err == nil || !strings.Contains(err.Error(), "made no progress after 3 attempts") {
		t.Fatalf("expected bounded no-progress failure, got %v", err)
	}
	if calls != 3 {
		t.Fatalf("requests=%d, want 3", calls)
	}
	if _, err := os.Stat(recommendedSMAPIArchivePath(dataDir, manifest)); !os.IsNotExist(err) {
		t.Fatalf("invalid target was retained: %v", err)
	}
}

func TestEnsureRecommendedSMAPIArchiveRejectsFinalChecksumMismatch(t *testing.T) {
	payload := readValidSMAPITestZip(t)
	manifest := smapiArchiveManifestForPayload(payload)
	manifest.SMAPI.SHA256 = strings.Repeat("0", 64)
	opts := smapiArchiveTestDownloadOptions(func(req *http.Request) (*http.Response, error) {
		start, end := parseRequestedSMAPIRange(t, req.Header.Get("Range"))
		return smapiRangeResponse(payload, start, end, int64(len(payload))), nil
	})
	opts.chunkBytes = int64(len(payload))
	dataDir := t.TempDir()

	if _, err := ensureRecommendedSMAPIArchiveWithOptions(context.Background(), dataDir, manifest, opts); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("expected final checksum rejection, got %v", err)
	}
	if _, err := os.Stat(recommendedSMAPIArchivePath(dataDir, manifest)); !os.IsNotExist(err) {
		t.Fatalf("checksum-mismatched target was retained: %v", err)
	}
}

func TestParseSMAPIContentRange(t *testing.T) {
	start, end, total, err := parseSMAPIContentRange("bytes 10-19/100")
	if err != nil || start != 10 || end != 19 || total != 100 {
		t.Fatalf("parsed %d-%d/%d, err=%v", start, end, total, err)
	}
	for _, raw := range []string{"", "items 0-1/2", "bytes 2-1/3", "bytes 0-2/2", "bytes */100"} {
		if _, _, _, err := parseSMAPIContentRange(raw); err == nil {
			t.Fatalf("invalid Content-Range accepted: %q", raw)
		}
	}
}

func readValidSMAPITestZip(t *testing.T) []byte {
	t.Helper()
	payload, err := os.ReadFile(writeSMAPITestZip(t, validSMAPIEntries()))
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func smapiArchiveManifestForPayload(payload []byte) sjconfig.RuntimeStackManifest {
	digest := sha256.Sum256(payload)
	component := sjconfig.SMAPIManifestComponent{
		Version: "4.5.2", DownloadURL: "https://github.com/Pathoschild/SMAPI/releases/download/4.5.2/SMAPI-4.5.2-installer.zip",
		SHA256: hex.EncodeToString(digest[:]), ArchiveBytes: int64(len(payload)), MaxArchiveBytes: int64(len(payload)) + 1024,
		MaxExtractBytes: 1024 * 1024, TrustedHosts: []string{"github.com"},
	}
	component.URLs = []string{component.DownloadURL}
	return sjconfig.RuntimeStackManifest{SMAPI: component}
}

func smapiArchiveTestDownloadOptions(roundTrip smapiArchiveRoundTripFunc) smapiArchiveDownloadOptions {
	return smapiArchiveDownloadOptions{
		chunkBytes: 64, requestTimeout: time.Second, maxNoProgressAttempts: 2, retryDelay: 0,
		newClient: func(time.Duration) *http.Client { return &http.Client{Transport: roundTrip} },
	}
}

func parseRequestedSMAPIRange(t *testing.T, raw string) (start, end int64) {
	t.Helper()
	if _, err := fmt.Sscanf(raw, "bytes=%d-%d", &start, &end); err != nil {
		t.Fatalf("parse request range %q: %v", raw, err)
	}
	return start, end
}

func smapiRangeResponse(payload []byte, start, end, total int64) *http.Response {
	body := append([]byte(nil), payload[start:end+1]...)
	header := make(http.Header)
	header.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, total))
	return &http.Response{
		StatusCode: http.StatusPartialContent,
		Header:     header, Body: io.NopCloser(bytes.NewReader(body)), ContentLength: int64(len(body)),
	}
}
