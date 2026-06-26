package stardew_junimo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultCatalog_IsUnavailable(t *testing.T) {
	cat := DefaultCatalog()
	if cat.Status != "unavailable" {
		t.Errorf("expected status=unavailable, got %q", cat.Status)
	}
	if cat.Source != "" {
		t.Errorf("expected empty source for unavailable catalog, got %q", cat.Source)
	}
	// Items must be present (text labels) but without image URLs.
	if len(cat.FarmTypes) == 0 {
		t.Error("expected non-empty farmTypes in unavailable catalog")
	}
	if len(cat.PetTypes) == 0 {
		t.Error("expected non-empty petTypes in unavailable catalog")
	}
	if len(cat.PetBreeds) == 0 {
		t.Error("expected non-empty petBreeds in unavailable catalog")
	}
	for _, ft := range cat.FarmTypes {
		if ft.Image != "" {
			t.Errorf("expected no image in unavailable farmType %q, got %q", ft.ID, ft.Image[:min(30, len(ft.Image))])
		}
	}
}

func TestDefaultCatalog_ProfitMarginIDs(t *testing.T) {
	cat := DefaultCatalog()
	valid := map[string]bool{"100": true, "75": true, "50": true, "25": true}
	for _, pm := range cat.ProfitMargins {
		if !valid[pm.ID] {
			t.Errorf("unexpected profitMargin ID %q in unavailable catalog", pm.ID)
		}
	}
}

func TestDefaultCatalog_MoneyModeIDs(t *testing.T) {
	cat := DefaultCatalog()
	valid := map[string]bool{"shared": true, "separate": true}
	for _, mm := range cat.MoneyModes {
		if !valid[mm.ID] {
			t.Errorf("unexpected moneyMode ID %q in unavailable catalog", mm.ID)
		}
	}
}

func TestReadCatalog_UnavailableWhenNoFile(t *testing.T) {
	dir := t.TempDir()
	cat := ReadCatalog(dir)
	if cat.Status != "unavailable" {
		t.Errorf("expected unavailable when no options.json, got status=%q", cat.Status)
	}
}

func TestReadCatalog_ParsesSMAPIOptions(t *testing.T) {
	dir := t.TempDir()
	ctrlDir := filepath.Join(dir, ".local-container", "control")
	if err := os.MkdirAll(ctrlDir, 0o755); err != nil {
		t.Fatal(err)
	}

	opts := panelOptionsFile{
		Source:      "smapi",
		GeneratedAt: time.Now(),
		FarmTypes: []PanelOptionItem{
			{ID: "standard", Label: "Standard Farm", Image: "data:image/png;base64,abc"},
		},
		PetTypes: []PanelOptionItem{
			{ID: "Cat", Label: "Cat"},
		},
		PetBreeds: []PanelOptionItem{
			{ID: "0", Label: "Breed 0", Group: "Cat"},
		},
		Genders: []PanelOptionItem{
			{ID: "male", Label: "Male"},
		},
		CabinCounts: []PanelOptionItem{
			{ID: "0", Label: "1 Player"},
		},
		CabinLayouts: []PanelOptionItem{
			{ID: "nearby", Label: "Nearby"},
		},
		ProfitMargins: []PanelOptionItem{
			{ID: "100", Label: "100%"},
		},
		MoneyModes: []PanelOptionItem{
			{ID: "shared", Label: "Shared"},
		},
	}

	data, err := json.Marshal(opts)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ctrlDir, "options.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	// Clear any cached entry for this dir.
	InvalidateCatalogCache(dir)

	cat := ReadCatalog(dir)
	if cat.Status != "ready" {
		t.Errorf("expected status=ready, got %q", cat.Status)
	}
	if cat.Source != "smapi" {
		t.Errorf("expected source=smapi, got %q", cat.Source)
	}
	if len(cat.FarmTypes) != 1 || cat.FarmTypes[0].ID != "standard" {
		t.Errorf("unexpected farmTypes: %v", cat.FarmTypes)
	}
}

func TestReadCatalog_CachesResult(t *testing.T) {
	dir := t.TempDir()
	ctrlDir := filepath.Join(dir, ".local-container", "control")
	if err := os.MkdirAll(ctrlDir, 0o755); err != nil {
		t.Fatal(err)
	}

	opts := panelOptionsFile{
		Source:      "smapi",
		GeneratedAt: time.Now(),
		FarmTypes:   []PanelOptionItem{{ID: "standard", Label: "Standard"}},
		ProfitMargins: []PanelOptionItem{{ID: "100", Label: "100%"}},
		MoneyModes:    []PanelOptionItem{{ID: "shared", Label: "Shared"}},
	}
	data, _ := json.Marshal(opts)
	if err := os.WriteFile(filepath.Join(ctrlDir, "options.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	InvalidateCatalogCache(dir)
	cat1 := ReadCatalog(dir)
	cat2 := ReadCatalog(dir)

	// Both calls should return same catalog version (cache hit on second call).
	if cat1.CatalogVersion != cat2.CatalogVersion {
		t.Errorf("cache miss: versions differ %q vs %q", cat1.CatalogVersion, cat2.CatalogVersion)
	}
}

func TestReadCatalog_PerInstanceIsolation(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	// dir1 has options.json; dir2 does not.
	ctrlDir1 := filepath.Join(dir1, ".local-container", "control")
	if err := os.MkdirAll(ctrlDir1, 0o755); err != nil {
		t.Fatal(err)
	}
	opts := panelOptionsFile{
		Source:      "smapi",
		GeneratedAt: time.Now(),
		FarmTypes:   []PanelOptionItem{{ID: "beach", Label: "Beach"}},
		ProfitMargins: []PanelOptionItem{{ID: "100", Label: "100%"}},
		MoneyModes:    []PanelOptionItem{{ID: "shared", Label: "Shared"}},
	}
	data, _ := json.Marshal(opts)
	_ = os.WriteFile(filepath.Join(ctrlDir1, "options.json"), data, 0o644)

	InvalidateCatalogCache(dir1)
	InvalidateCatalogCache(dir2)

	cat1 := ReadCatalog(dir1)
	cat2 := ReadCatalog(dir2)

	if cat1.Status != "ready" {
		t.Errorf("dir1 expected status=ready, got %q", cat1.Status)
	}
	if cat2.Status != "unavailable" {
		t.Errorf("dir2 expected status=unavailable, got %q", cat2.Status)
	}
}

func TestNormalizeProfitMarginIDs(t *testing.T) {
	items := []PanelOptionItem{
		{ID: "normal", Label: "100%"},
		{ID: "75%", Label: "75%"},
		{ID: "50%", Label: "50%"},
		{ID: "25%", Label: "25%"},
		{ID: "100", Label: "already-ok"},
	}
	got := normalizeProfitMarginIDs(items)
	wantIDs := []string{"100", "75", "50", "25", "100"}
	for i, item := range got {
		if item.ID != wantIDs[i] {
			t.Errorf("[%d] got ID %q, want %q", i, item.ID, wantIDs[i])
		}
	}
}
