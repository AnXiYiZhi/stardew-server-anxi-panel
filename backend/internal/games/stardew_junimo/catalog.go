package stardew_junimo

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// PanelOptionItem is one selectable item in the visual game creator.
// Image is a data URL (from SMAPI asset export) or empty string.
type PanelOptionItem struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Group       string `json:"group,omitempty"`
	Description string `json:"description,omitempty"`
	Image       string `json:"image,omitempty"`
}

// CatalogResponse is returned by GET /api/instances/:id/custom-new-game/catalog.
//
// Status values:
//   - "ready"       — options.json exists and is valid; real SMAPI images are included.
//   - "generating"  — export container is running; poll again in a few seconds.
//   - "failed"      — export finished with an error; Error field contains details.
//   - "unavailable" — game not installed or export has never been attempted.
//
// For non-ready statuses the item slices are populated with text labels but
// no Image fields, so the form remains usable without showing fake assets.
type CatalogResponse struct {
	Status         string            `json:"status"`                   // "ready"|"generating"|"failed"|"unavailable"
	Source         string            `json:"source,omitempty"`         // "smapi" (only when ready)
	GeneratedAt    *time.Time        `json:"generatedAt,omitempty"`
	CatalogVersion string            `json:"catalogVersion,omitempty"` // options.json mtime when ready
	Error          string            `json:"error,omitempty"`          // set when status=="failed"
	FarmTypes      []PanelOptionItem `json:"farmTypes"`
	PetTypes       []PanelOptionItem `json:"petTypes"`
	PetBreeds      []PanelOptionItem `json:"petBreeds"`
	Genders        []PanelOptionItem `json:"genders"`
	CabinCounts    []PanelOptionItem `json:"cabinCounts"`
	CabinLayouts   []PanelOptionItem `json:"cabinLayouts"`
	ProfitMargins  []PanelOptionItem `json:"profitMargins"`
	MoneyModes     []PanelOptionItem `json:"moneyModes"`
}

// panelOptionsFile is the structure of options.json written by the SMAPI mod.
type panelOptionsFile struct {
	Source        string            `json:"source"`
	GeneratedAt   time.Time         `json:"generatedAt"`
	FarmTypes     []PanelOptionItem `json:"farmTypes"`
	PetTypes      []PanelOptionItem `json:"petTypes"`
	PetBreeds     []PanelOptionItem `json:"petBreeds"`
	Genders       []PanelOptionItem `json:"genders"`
	CabinCounts   []PanelOptionItem `json:"cabinCounts"`
	CabinLayouts  []PanelOptionItem `json:"cabinLayouts"`
	ProfitMargins []PanelOptionItem `json:"profitMargins"`
	MoneyModes    []PanelOptionItem `json:"moneyModes"`
}

type catalogEntry struct {
	mtime time.Time
	resp  CatalogResponse
}

var catalogMu sync.Mutex
var catalogByDir = map[string]catalogEntry{}

// controlDir returns the host-side path to the SMAPI control directory.
func controlDir(dataDir string) string {
	return filepath.Join(dataDir, ".local-container", "control")
}

// ReadCatalog reads the SMAPI-generated options.json from the control directory
// and returns a status-aware response.  The status field tells the caller whether
// real game assets are available (ready), still being generated (generating),
// failed (failed), or not yet started (unavailable).
func ReadCatalog(dataDir string) CatalogResponse {
	ctrlDir := controlDir(dataDir)
	optPath := filepath.Join(ctrlDir, "options.json")

	// Prefer options.json when it exists — even during a re-export the old assets
	// are better than an empty state.
	if stat, err := os.Stat(optPath); err == nil {
		catalogMu.Lock()
		entry, ok := catalogByDir[dataDir]
		catalogMu.Unlock()

		if ok && entry.mtime.Equal(stat.ModTime()) {
			return entry.resp
		}

		data, readErr := os.ReadFile(optPath)
		if readErr == nil {
			var opts panelOptionsFile
			if jsonErr := json.Unmarshal(data, &opts); jsonErr == nil {
				t := opts.GeneratedAt
				resp := CatalogResponse{
					Status:         "ready",
					Source:         "smapi",
					GeneratedAt:    &t,
					CatalogVersion: stat.ModTime().UTC().Format(time.RFC3339),
					FarmTypes:      orDefault(opts.FarmTypes, defaultFarmTypes()),
					PetTypes:       orDefault(opts.PetTypes, defaultPetTypes()),
					PetBreeds:      orDefault(opts.PetBreeds, defaultPetBreeds()),
					Genders:        orDefault(opts.Genders, defaultGenders()),
					CabinCounts:    orDefault(opts.CabinCounts, defaultCabinCounts()),
					CabinLayouts:   orDefault(opts.CabinLayouts, defaultCabinLayouts()),
					ProfitMargins:  normalizeProfitMarginIDs(orDefault(opts.ProfitMargins, defaultProfitMargins())),
					MoneyModes:     normalizeMoneyModeIDs(orDefault(opts.MoneyModes, defaultMoneyModes())),
				}
				catalogMu.Lock()
				catalogByDir[dataDir] = catalogEntry{mtime: stat.ModTime(), resp: resp}
				catalogMu.Unlock()
				return resp
			}
		}
	}

	// No valid options.json. Check the export lock.
	if _, err := os.Stat(filepath.Join(ctrlDir, catalogLockFile)); err == nil {
		return noImageCatalog("generating", "")
	}

	// Check error marker.
	if data, err := os.ReadFile(filepath.Join(ctrlDir, catalogErrorFile)); err == nil {
		var errFile struct {
			Error string `json:"error"`
		}
		msg := "素材导出失败"
		if json.Unmarshal(data, &errFile) == nil && errFile.Error != "" {
			msg = errFile.Error
		}
		return noImageCatalog("failed", msg)
	}

	// No files at all — export has never run or game is not installed.
	return noImageCatalog("unavailable", "")
}

// DefaultCatalog returns an unavailable catalog (no images).
// Kept for use in tests that check catalog structure without SMAPI.
func DefaultCatalog() CatalogResponse {
	return noImageCatalog("unavailable", "")
}

// InvalidateCatalogCache removes the cached catalog for an instance,
// forcing the next ReadCatalog to re-read options.json.
func InvalidateCatalogCache(dataDir string) {
	catalogMu.Lock()
	delete(catalogByDir, dataDir)
	catalogMu.Unlock()
}

// noImageCatalog builds a catalog response for non-ready states.
// All item lists are populated with text labels but no image fields.
func noImageCatalog(status, errMsg string) CatalogResponse {
	return CatalogResponse{
		Status:        status,
		Error:         errMsg,
		FarmTypes:     stripImages(defaultFarmTypes()),
		PetTypes:      stripImages(defaultPetTypes()),
		PetBreeds:     stripImages(defaultPetBreeds()),
		Genders:       defaultGenders(),
		CabinCounts:   defaultCabinCounts(),
		CabinLayouts:  defaultCabinLayouts(),
		ProfitMargins: defaultProfitMargins(),
		MoneyModes:    defaultMoneyModes(),
	}
}

// stripImages returns a copy of items with the Image field cleared.
func stripImages(items []PanelOptionItem) []PanelOptionItem {
	out := make([]PanelOptionItem, len(items))
	for i, item := range items {
		item.Image = ""
		out[i] = item
	}
	return out
}

func orDefault(items, fallback []PanelOptionItem) []PanelOptionItem {
	if len(items) > 0 {
		return items
	}
	return fallback
}

// normalizeProfitMarginIDs maps any SMAPI label-based IDs to numeric strings
// ("100", "75", "50", "25") that match backend validation.
func normalizeProfitMarginIDs(items []PanelOptionItem) []PanelOptionItem {
	remap := map[string]string{"normal": "100", "100%": "100", "75%": "75", "50%": "50", "25%": "25"}
	out := make([]PanelOptionItem, len(items))
	for i, item := range items {
		if id, ok := remap[item.ID]; ok {
			item.ID = id
		}
		out[i] = item
	}
	return out
}

// normalizeMoneyModeIDs maps any SMAPI label-based IDs to "shared" or "separate".
func normalizeMoneyModeIDs(items []PanelOptionItem) []PanelOptionItem {
	remap := map[string]string{"normal": "shared", "0": "separate", "1": "shared"}
	out := make([]PanelOptionItem, len(items))
	for i, item := range items {
		if id, ok := remap[item.ID]; ok {
			item.ID = id
		}
		out[i] = item
	}
	return out
}

func defaultFarmTypes() []PanelOptionItem {
	return []PanelOptionItem{
		{ID: "standard", Label: "标准农场", Image: farmSVG("标准", "#79a75f", "#d6edbd")},
		{ID: "riverland", Label: "河边农场", Image: farmSVG("河边", "#4d99b5", "#c8edf3")},
		{ID: "forest", Label: "森林农场", Image: farmSVG("森林", "#4d7f48", "#cfe6b8")},
		{ID: "hilltop", Label: "山顶农场", Image: farmSVG("山顶", "#8f7860", "#e6d1b4")},
		{ID: "wilderness", Label: "荒野农场", Image: farmSVG("荒野", "#695784", "#d8cbe8")},
		{ID: "fourcorners", Label: "四角农场", Image: farmSVG("四角", "#b38b3b", "#ecd99b")},
		{ID: "beach", Label: "海滩农场", Image: farmSVG("海滩", "#d8ad5a", "#f4ddb0")},
	}
}

func defaultPetTypes() []PanelOptionItem {
	return []PanelOptionItem{
		{ID: "Cat", Label: "猫", Image: petSVG("猫", "#d08d3c", "#f5d19a")},
		{ID: "Dog", Label: "狗", Image: petSVG("狗", "#8d6b4f", "#e4c6a5")},
	}
}

func defaultPetBreeds() []PanelOptionItem {
	items := make([]PanelOptionItem, 0, 12)
	for i := 0; i < 6; i++ {
		items = append(items, PanelOptionItem{
			ID:    fmt.Sprintf("%d", i),
			Label: fmt.Sprintf("猫 %d", i+1),
			Group: "Cat",
			Image: petSVG(fmt.Sprintf("猫%d", i+1), "#d08d3c", "#f5d19a"),
		})
	}
	for i := 0; i < 6; i++ {
		items = append(items, PanelOptionItem{
			ID:    fmt.Sprintf("%d", i),
			Label: fmt.Sprintf("狗 %d", i+1),
			Group: "Dog",
			Image: petSVG(fmt.Sprintf("狗%d", i+1), "#8d6b4f", "#e4c6a5"),
		})
	}
	return items
}

func defaultGenders() []PanelOptionItem {
	return []PanelOptionItem{
		{ID: "male", Label: "男"},
		{ID: "female", Label: "女"},
	}
}

func defaultCabinCounts() []PanelOptionItem {
	items := make([]PanelOptionItem, 4)
	for i := 0; i < 4; i++ {
		items[i] = PanelOptionItem{ID: fmt.Sprintf("%d", i), Label: fmt.Sprintf("%d 人", i+1)}
	}
	return items
}

func defaultCabinLayouts() []PanelOptionItem {
	return []PanelOptionItem{
		{ID: "nearby", Label: "靠近", Description: "联机小屋靠近农舍。"},
		{ID: "separate", Label: "分散", Description: "联机小屋分布在地图上。"},
	}
}

func defaultProfitMargins() []PanelOptionItem {
	return []PanelOptionItem{
		{ID: "100", Label: "100%"},
		{ID: "75", Label: "75%"},
		{ID: "50", Label: "50%"},
		{ID: "25", Label: "25%"},
	}
}

func defaultMoneyModes() []PanelOptionItem {
	return []PanelOptionItem{
		{ID: "shared", Label: "共享资金"},
		{ID: "separate", Label: "分开资金"},
	}
}

func farmSVG(text, bg, terrain string) string {
	svg := fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 240 132"><rect width="240" height="132" rx="8" fill="%s"/><path d="M0 92c48-18 82 19 128 0s72-6 112 12v28H0z" fill="%s"/><circle cx="190" cy="35" r="18" fill="#f7d76b"/><text x="16" y="78" font-family="Arial,'Microsoft YaHei',sans-serif" font-size="28" font-weight="700" fill="#1f2b1f" opacity=".85">%s</text></svg>`,
		bg, terrain, text,
	)
	return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(svg))
}

func petSVG(text, bg, spot string) string {
	svg := fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 96 96"><rect width="96" height="96" rx="8" fill="%s"/><circle cx="48" cy="42" r="26" fill="%s"/><text x="48" y="82" text-anchor="middle" font-family="Arial,'Microsoft YaHei',sans-serif" font-size="18" font-weight="700" fill="#1f2b1f" opacity=".85">%s</text></svg>`,
		bg, spot, text,
	)
	return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(svg))
}
