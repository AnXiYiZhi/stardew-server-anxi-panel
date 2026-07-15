package stardew_junimo

import (
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
)

const sveContentPackUniqueID = "FlashShifter.StardewValleyExpandedCP"

// DetectModCompatibilityWarnings reports save state which proves that a world
// overhaul was enabled after the save serialized its initial world/quest data.
// Detection is read-only and deliberately conservative; failure to inspect a
// save never blocks the Mods API.
func DetectModCompatibilityWarnings(dataDir, saveName string, mods []registry.ModInfo) []registry.ModCompatibilityWarning {
	if strings.TrimSpace(saveName) == "" || !enabledModUniqueID(mods, sveContentPackUniqueID) {
		return nil
	}
	savesRoot := filepath.Join(savesDir(dataDir), "Saves")
	saveDir := resolveSavePath(savesRoot, saveName)
	if saveDir == "" {
		return nil
	}
	totals := readIntroductionsQuestTotals(filepath.Join(saveDir, saveName))
	stale := false
	for _, total := range totals {
		if total > 0 && total <= 28 {
			stale = true
			break
		}
	}
	if !stale {
		return nil
	}
	return []registry.ModCompatibilityWarning{{
		Code:     "existing_save_world_overhaul_not_rebuilt",
		Severity: "warning",
		Title:    "当前存档未按 SVE 重新初始化",
		Message:  "该存档仍保存原版 28 人介绍任务；已有树木、地形物件和坐标也不会因后来启用 SVE 自动重建。SVE 文件可以正常加载，但要获得完整的新地图初始化与 32 人介绍任务，请在所有模组启用后新建存档。面板不会自动改写旧存档。",
		SaveName: saveName,
	}}
}

func enabledModUniqueID(mods []registry.ModInfo, uniqueID string) bool {
	for _, mod := range mods {
		if mod.Enabled && strings.EqualFold(strings.TrimSpace(mod.UniqueID), uniqueID) {
			return true
		}
	}
	return false
}

func readIntroductionsQuestTotals(path string) []int {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer func() { _ = file.Close() }()

	decoder := xml.NewDecoder(io.LimitReader(file, maxSaveInspectBytes))
	var totals []int
	for {
		token, err := decoder.Token()
		if err != nil {
			return totals
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "Quest" {
			continue
		}
		isSocialize := false
		for _, attr := range start.Attr {
			if attr.Name.Local == "type" && attr.Value == "SocializeQuest" {
				isSocialize = true
				break
			}
		}
		if !isSocialize {
			continue
		}
		var quest struct {
			Title string `xml:"questTitle"`
			Total int    `xml:"total"`
		}
		if err := decoder.DecodeElement(&quest, &start); err != nil {
			return totals
		}
		if strings.EqualFold(strings.TrimSpace(quest.Title), "Introductions") {
			totals = append(totals, quest.Total)
		}
	}
}

const maxSaveInspectBytes = 128 * 1024 * 1024
