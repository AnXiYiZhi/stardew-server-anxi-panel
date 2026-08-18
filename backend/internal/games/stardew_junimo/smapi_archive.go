package stardew_junimo

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/netdns"
)

const (
	maxSMAPIArchiveEntries            = 4096
	smapiArchiveChunkBytes            = int64(2 * 1024 * 1024)
	smapiArchiveChunkTimeout          = 2 * time.Minute
	smapiArchiveMaxNoProgressAttempts = 4
	smapiArchiveNoProgressRetryDelay  = time.Second
)

type smapiArchiveDownloadOptions struct {
	chunkBytes            int64
	requestTimeout        time.Duration
	maxNoProgressAttempts int
	retryDelay            time.Duration
	newClient             func(time.Duration) *http.Client
	onProgress            func(smapiArchiveDownloadProgress)
}

type smapiArchiveDownloadProgress struct {
	DownloadedBytes int64
	TotalBytes      int64
	Candidate       int
	CandidateCount  int
	Cached          bool
}

var defaultSMAPIArchiveDownloadOptions = smapiArchiveDownloadOptions{
	chunkBytes:            smapiArchiveChunkBytes,
	requestTimeout:        smapiArchiveChunkTimeout,
	maxNoProgressAttempts: smapiArchiveMaxNoProgressAttempts,
	retryDelay:            smapiArchiveNoProgressRetryDelay,
	newClient:             netdns.NewClient,
}

func recommendedSMAPIArchivePath(dataDir string, manifest sjconfig.RuntimeStackManifest) string {
	return filepath.Join(dataDir, ".local-container", "smapi-update", "packages", "SMAPI-"+manifest.SMAPI.Version+"-installer.zip")
}

func ensureRecommendedSMAPIArchive(ctx context.Context, dataDir string, manifest sjconfig.RuntimeStackManifest) (string, error) {
	return ensureRecommendedSMAPIArchiveWithOptions(ctx, dataDir, manifest, defaultSMAPIArchiveDownloadOptions)
}

func ensureRecommendedSMAPIArchiveWithProgress(ctx context.Context, dataDir string, manifest sjconfig.RuntimeStackManifest, onProgress func(smapiArchiveDownloadProgress)) (string, error) {
	opts := defaultSMAPIArchiveDownloadOptions
	opts.onProgress = onProgress
	return ensureRecommendedSMAPIArchiveWithOptions(ctx, dataDir, manifest, opts)
}

func ensureRecommendedSMAPIArchiveWithOptions(ctx context.Context, dataDir string, manifest sjconfig.RuntimeStackManifest, opts smapiArchiveDownloadOptions) (string, error) {
	target := recommendedSMAPIArchivePath(dataDir, manifest)
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return "", err
	}
	if err := validateRecommendedSMAPIArchive(target, manifest); err == nil {
		emitSMAPIArchiveProgress(opts, smapiArchiveDownloadProgress{
			DownloadedBytes: manifest.SMAPI.ArchiveBytes,
			TotalBytes:      manifest.SMAPI.ArchiveBytes,
			Candidate:       1,
			CandidateCount:  max(1, len(manifest.SMAPI.URLs)),
			Cached:          true,
		})
		return target, nil
	}

	tmp, err := os.CreateTemp(filepath.Dir(target), ".smapi-download-*.part")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return "", err
	}
	if err := downloadRecommendedSMAPIArchive(ctx, tmp, manifest, opts); err != nil {
		_ = tmp.Close()
		return "", err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := validateRecommendedSMAPIArchive(tmpName, manifest); err != nil {
		return "", err
	}
	if err := replaceRuntimeUpdateStatusFile(tmpName, target); err != nil {
		return "", err
	}
	return target, nil
}

func downloadRecommendedSMAPIArchive(ctx context.Context, dst *os.File, manifest sjconfig.RuntimeStackManifest, opts smapiArchiveDownloadOptions) error {
	if dst == nil {
		return errors.New("SMAPI archive destination is required")
	}
	if manifest.SMAPI.ArchiveBytes <= 0 || manifest.SMAPI.ArchiveBytes > manifest.SMAPI.MaxArchiveBytes {
		return errors.New("SMAPI archive size limit is invalid")
	}
	if opts.chunkBytes <= 0 || opts.requestTimeout <= 0 || opts.maxNoProgressAttempts <= 0 || opts.retryDelay < 0 || opts.newClient == nil {
		return errors.New("SMAPI archive download options are invalid")
	}
	if len(manifest.SMAPI.URLs) == 0 {
		return errors.New("SMAPI archive download candidates are required")
	}

	trusted := map[string]bool{}
	for _, host := range manifest.SMAPI.TrustedHosts {
		trusted[strings.ToLower(host)] = true
	}
	client := opts.newClient(opts.requestTimeout)
	if client == nil {
		return errors.New("SMAPI archive HTTP client is unavailable")
	}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 || req.URL.Scheme != "https" || !trusted[strings.ToLower(req.URL.Hostname())] {
			return errors.New("SMAPI download redirect left the trusted host allowlist")
		}
		return nil
	}
	var candidateErrors []error
	for candidateIndex, rawURL := range manifest.SMAPI.URLs {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := dst.Truncate(0); err != nil {
			return err
		}
		if _, err := dst.Seek(0, io.SeekStart); err != nil {
			return err
		}
		progress := smapiArchiveDownloadProgress{
			TotalBytes:     manifest.SMAPI.ArchiveBytes,
			Candidate:      candidateIndex + 1,
			CandidateCount: len(manifest.SMAPI.URLs),
		}
		emitSMAPIArchiveProgress(opts, progress)
		err := downloadRecommendedSMAPIArchiveCandidate(ctx, client, dst, manifest, strings.TrimSpace(rawURL), opts, func(downloaded int64) {
			progress.DownloadedBytes = downloaded
			emitSMAPIArchiveProgress(opts, progress)
		})
		if err == nil {
			err = dst.Sync()
		}
		if err == nil {
			err = validateRecommendedSMAPIArchive(dst.Name(), manifest)
		}
		if err == nil {
			return nil
		}
		host := "invalid-candidate"
		if parsed, parseErr := url.Parse(rawURL); parseErr == nil && parsed.Hostname() != "" {
			host = parsed.Hostname()
		}
		candidateErrors = append(candidateErrors, fmt.Errorf("%s: %w", host, err))
	}
	return fmt.Errorf("all reviewed SMAPI download candidates failed: %w", errors.Join(candidateErrors...))
}

func downloadRecommendedSMAPIArchiveCandidate(ctx context.Context, client *http.Client, dst *os.File, manifest sjconfig.RuntimeStackManifest, rawURL string, opts smapiArchiveDownloadOptions, onProgress func(int64)) error {
	var offset int64
	noProgressAttempts := 0
	for offset < manifest.SMAPI.ArchiveBytes {
		if err := ctx.Err(); err != nil {
			return err
		}
		end := min(offset+opts.chunkBytes-1, manifest.SMAPI.ArchiveBytes-1)
		written, err := downloadSMAPIArchiveRange(ctx, client, dst, manifest, rawURL, offset, end, onProgress)
		if written > 0 {
			offset += written
			noProgressAttempts = 0
		}
		if err == nil {
			continue
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if written == 0 {
			noProgressAttempts++
			if noProgressAttempts >= opts.maxNoProgressAttempts {
				return fmt.Errorf("download recommended SMAPI made no progress after %d attempts: %w", noProgressAttempts, err)
			}
		}
		if opts.retryDelay > 0 {
			timer := time.NewTimer(opts.retryDelay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
	}
	if offset != manifest.SMAPI.ArchiveBytes {
		return errors.New("SMAPI archive length mismatch")
	}
	return nil
}

func downloadSMAPIArchiveRange(ctx context.Context, client *http.Client, dst *os.File, manifest sjconfig.RuntimeStackManifest, rawURL string, start, end int64, onProgress func(int64)) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "anxi-panel-smapi-update")
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("download recommended SMAPI range %d-%d: %w", start, end, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent {
		return 0, fmt.Errorf("SMAPI range download returned HTTP %d", resp.StatusCode)
	}
	rangeStart, rangeEnd, total, err := parseSMAPIContentRange(resp.Header.Get("Content-Range"))
	if err != nil || rangeStart != start || rangeEnd != end || total != manifest.SMAPI.ArchiveBytes {
		return 0, errors.New("SMAPI range response does not match the reviewed archive")
	}
	want := end - start + 1
	if resp.ContentLength >= 0 && resp.ContentLength != want {
		return 0, errors.New("SMAPI range response length mismatch")
	}
	progressDst := io.Writer(dst)
	if onProgress != nil {
		progressDst = &smapiArchiveProgressWriter{dst: dst, downloaded: start, onProgress: onProgress}
	}
	written, copyErr := io.CopyN(progressDst, resp.Body, want)
	if copyErr != nil {
		return written, fmt.Errorf("read recommended SMAPI range %d-%d: %w", start, end, copyErr)
	}
	return written, nil
}

type smapiArchiveProgressWriter struct {
	dst        io.Writer
	downloaded int64
	onProgress func(int64)
}

func (w *smapiArchiveProgressWriter) Write(p []byte) (int, error) {
	n, err := w.dst.Write(p)
	if n > 0 {
		w.downloaded += int64(n)
		w.onProgress(w.downloaded)
	}
	return n, err
}

func emitSMAPIArchiveProgress(opts smapiArchiveDownloadOptions, progress smapiArchiveDownloadProgress) {
	if opts.onProgress != nil {
		opts.onProgress(progress)
	}
}

func parseSMAPIContentRange(raw string) (start, end, total int64, err error) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "bytes ") {
		return 0, 0, 0, errors.New("invalid SMAPI content range unit")
	}
	parts := strings.Split(strings.TrimPrefix(raw, "bytes "), "/")
	if len(parts) != 2 {
		return 0, 0, 0, errors.New("invalid SMAPI content range")
	}
	bounds := strings.Split(parts[0], "-")
	if len(bounds) != 2 {
		return 0, 0, 0, errors.New("invalid SMAPI content range bounds")
	}
	start, err = strconv.ParseInt(bounds[0], 10, 64)
	if err != nil {
		return 0, 0, 0, errors.New("invalid SMAPI content range start")
	}
	end, err = strconv.ParseInt(bounds[1], 10, 64)
	if err != nil {
		return 0, 0, 0, errors.New("invalid SMAPI content range end")
	}
	total, err = strconv.ParseInt(parts[1], 10, 64)
	if err != nil || start < 0 || end < start || total <= end {
		return 0, 0, 0, errors.New("invalid SMAPI content range total")
	}
	return start, end, total, nil
}

func validateRecommendedSMAPIArchive(filename string, manifest sjconfig.RuntimeStackManifest) error {
	info, err := os.Stat(filename)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() != manifest.SMAPI.ArchiveBytes || info.Size() > manifest.SMAPI.MaxArchiveBytes {
		return errors.New("SMAPI archive size mismatch")
	}
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	h := sha256.New()
	n, copyErr := io.Copy(h, io.LimitReader(f, manifest.SMAPI.MaxArchiveBytes+1))
	closeErr := f.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if n != manifest.SMAPI.ArchiveBytes || hex.EncodeToString(h.Sum(nil)) != manifest.SMAPI.SHA256 {
		return errors.New("SMAPI archive checksum mismatch")
	}
	return validateSMAPIZip(filename, manifest.SMAPI.MaxExtractBytes)
}

func validateSMAPIZip(filename string, maxExtractBytes int64) error {
	reader, err := zip.OpenReader(filename)
	if err != nil {
		return errors.New("SMAPI archive is damaged")
	}
	defer reader.Close()
	if len(reader.File) == 0 || len(reader.File) > maxSMAPIArchiveEntries {
		return errors.New("SMAPI archive entry count is invalid")
	}
	var extracted uint64
	linuxInstaller, windowsInstaller := 0, 0
	for _, entry := range reader.File {
		name := strings.ReplaceAll(entry.Name, "\\", "/")
		clean := path.Clean(name)
		if name == "" || clean == "." || path.IsAbs(name) || clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(name, ":") || clean != strings.TrimSuffix(name, "/") {
			if !(entry.FileInfo().IsDir() && clean == strings.TrimSuffix(name, "/")) {
				return fmt.Errorf("unsafe SMAPI ZIP path %q", entry.Name)
			}
		}
		if entry.Mode()&os.ModeSymlink != 0 || entry.Mode()&os.ModeDevice != 0 {
			return errors.New("SMAPI archive contains a link or device entry")
		}
		extracted += entry.UncompressedSize64
		if extracted > uint64(maxExtractBytes) {
			return errors.New("SMAPI archive expands beyond the allowed limit")
		}
		if entry.CompressedSize64 > 0 && entry.UncompressedSize64 > entry.CompressedSize64*200 {
			return errors.New("SMAPI archive contains an unsafe compression ratio")
		}
		if strings.HasSuffix(clean, "/internal/linux/SMAPI.Installer") {
			linuxInstaller++
		}
		if strings.HasSuffix(clean, "/internal/windows/SMAPI.Installer.exe") {
			windowsInstaller++
		}
	}
	if linuxInstaller != 1 || windowsInstaller != 1 {
		return errors.New("SMAPI archive installer structure is invalid")
	}
	return nil
}

func validateSMAPIManifestURL(raw string, manifest sjconfig.RuntimeStackManifest) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() != "github.com" || raw != manifest.SMAPI.DownloadURL {
		return errors.New("SMAPI URL is not the reviewed manifest URL")
	}
	return nil
}
