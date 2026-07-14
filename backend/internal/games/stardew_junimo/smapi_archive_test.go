package stardew_junimo

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
)

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
