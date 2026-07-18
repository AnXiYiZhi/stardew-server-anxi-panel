package web

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func TestSameDiagnosticSaveIdentity(t *testing.T) {
	cases := []struct {
		left, right string
		want        bool
	}{
		{"Farm", "Farm", true},
		{"Farm", "Farm_442923526", true},
		{"Farm_442923526", "Farm", true},
		{"Farm", "Farm_backup", false},
		{"", "Farm", false},
	}
	for _, tc := range cases {
		if got := sameDiagnosticSaveIdentity(tc.left, tc.right); got != tc.want {
			t.Fatalf("sameDiagnosticSaveIdentity(%q, %q) = %v, want %v", tc.left, tc.right, got, tc.want)
		}
	}
}

func TestDurationBetween(t *testing.T) {
	start := "2026-07-11T10:00:00Z"
	end := "2026-07-11T10:02:30Z"
	got := durationBetween(start, end)
	if got == nil || *got != (150*time.Second).Milliseconds() {
		t.Fatalf("durationBetween = %v, want 150000", got)
	}
	if durationBetween(end, start) != nil || durationBetween("bad", end) != nil {
		t.Fatal("invalid or reversed timestamps must not produce a duration")
	}
	if durationBetween(start, "2026-07-11T14:02:30Z") != nil {
		t.Fatal("timestamps from different startup observations must not produce a duration")
	}
}

func TestRuntimeDiagnosticUsesRecommendedControlVersion(t *testing.T) {
	manifest, err := sjconfig.BuiltInRuntimeStackManifest()
	if err != nil {
		t.Fatal(err)
	}
	dataDir := t.TempDir()
	modDir := filepath.Join(dataDir, ".local-container", "mods", "StardewAnxiPanel.Control")
	if err := os.MkdirAll(modDir, 0o700); err != nil {
		t.Fatal(err)
	}
	controlManifest := []byte(fmt.Sprintf(`{"Version":%q}`, manifest.Control.Version))
	if err := os.WriteFile(filepath.Join(modDir, "manifest.json"), controlManifest, 0o600); err != nil {
		t.Fatal(err)
	}
	diagnostic := buildRuntimeDiagnostic(storage.Instance{DataDir: dataDir, State: storage.InstanceStateStopped}, controlStatusSnapshot{}, controlPlayersSnapshot{})
	if diagnostic.ExpectedControlMod != manifest.Control.Version || !diagnostic.ControlModMatches {
		t.Fatalf("control diagnostic=%+v", diagnostic)
	}
}
