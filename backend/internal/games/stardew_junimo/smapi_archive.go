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
	"strings"
	"time"

	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/netdns"
)

const maxSMAPIArchiveEntries = 4096

func recommendedSMAPIArchivePath(dataDir string, manifest sjconfig.RuntimeStackManifest) string {
	return filepath.Join(dataDir, ".local-container", "smapi-update", "packages", "SMAPI-"+manifest.SMAPI.Version+"-installer.zip")
}

func ensureRecommendedSMAPIArchive(ctx context.Context, dataDir string, manifest sjconfig.RuntimeStackManifest) (string, error) {
	target := recommendedSMAPIArchivePath(dataDir, manifest)
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return "", err
	}
	if err := validateRecommendedSMAPIArchive(target, manifest); err == nil {
		return target, nil
	}

	trusted := map[string]bool{}
	for _, host := range manifest.SMAPI.TrustedHosts {
		trusted[strings.ToLower(host)] = true
	}
	client := netdns.NewClient(2 * time.Minute)
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 || req.URL.Scheme != "https" || !trusted[strings.ToLower(req.URL.Hostname())] {
			return errors.New("SMAPI download redirect left the trusted host allowlist")
		}
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, manifest.SMAPI.DownloadURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "anxi-panel-smapi-update")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download recommended SMAPI: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("SMAPI download returned HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > manifest.SMAPI.MaxArchiveBytes {
		return "", errors.New("SMAPI archive exceeds the size limit")
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
	h := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(tmp, h), io.LimitReader(resp.Body, manifest.SMAPI.MaxArchiveBytes+1))
	if copyErr != nil {
		_ = tmp.Close()
		return "", copyErr
	}
	if n != manifest.SMAPI.ArchiveBytes || n > manifest.SMAPI.MaxArchiveBytes {
		_ = tmp.Close()
		return "", errors.New("SMAPI archive length mismatch")
	}
	if hex.EncodeToString(h.Sum(nil)) != manifest.SMAPI.SHA256 {
		_ = tmp.Close()
		return "", errors.New("SMAPI archive SHA256 mismatch")
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := validateSMAPIZip(tmpName, manifest.SMAPI.MaxExtractBytes); err != nil {
		return "", err
	}
	if err := replaceRuntimeUpdateStatusFile(tmpName, target); err != nil {
		return "", err
	}
	return target, nil
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
