//go:build integration

package stardew_junimo

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
)

func TestSMAPIArchiveRealDownload(t *testing.T) {
	if os.Getenv("PANEL_RUN_SMAPI_DOWNLOAD_TEST") != "1" {
		t.Skip("set PANEL_RUN_SMAPI_DOWNLOAD_TEST=1 to run the reviewed accelerator download gate")
	}
	manifest, err := sjconfig.BuiltInRuntimeStackManifest()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	dataDir := t.TempDir()
	started := time.Now()
	target, err := ensureRecommendedSMAPIArchive(ctx, dataDir, manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateRecommendedSMAPIArchive(target, manifest); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("SMAPI cache mode=%#o, want 0600", info.Mode().Perm())
	}
	parts, err := filepath.Glob(filepath.Join(filepath.Dir(target), ".smapi-download-*.part"))
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 0 {
		t.Fatalf("temporary SMAPI downloads were retained: %v", parts)
	}
	t.Logf("downloaded %d bytes through reviewed candidates in %s", info.Size(), time.Since(started).Round(time.Second))
}
