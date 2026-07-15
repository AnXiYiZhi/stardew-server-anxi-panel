package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

type farmTypeCatalogResponse struct {
	FarmTypes             []farmTypeCatalogItem `json:"farmTypes"`
	CatalogWarnings       []string              `json:"catalogWarnings"`
	ModdedCreationEnabled bool                  `json:"moddedCreationEnabled"`
}

type farmTypeCatalogItem struct {
	ID                        string                  `json:"id"`
	Label                     string                  `json:"label"`
	Description               string                  `json:"description"`
	Kind                      string                  `json:"kind"`
	ProviderModID             string                  `json:"providerModId,omitempty"`
	ProviderName              string                  `json:"providerName,omitempty"`
	ProviderVersion           string                  `json:"providerVersion,omitempty"`
	ProviderFolder            string                  `json:"providerFolder,omitempty"`
	Enabled                   bool                    `json:"enabled"`
	Confidence                string                  `json:"confidence"`
	Conditions                []farmTypeCondition     `json:"conditions"`
	Conflict                  bool                    `json:"conflict"`
	DependenciesReady         *bool                   `json:"dependenciesReady"`
	Selectable                bool                    `json:"selectable"`
	RequiresRuntimeValidation bool                    `json:"requiresRuntimeValidation"`
	IconURL                   string                  `json:"iconUrl,omitempty"`
	Warnings                  []string                `json:"warnings"`
	ModSelection              *sj.NewGameModSelection `json:"modSelection,omitempty"`
}

type farmTypeCondition struct {
	Kind string          `json:"kind"`
	When json.RawMessage `json:"when"`
}

var builtinFarmTypes = []farmTypeCatalogItem{
	{ID: "standard", Label: "标准农场", Kind: "builtin", Enabled: true, Confidence: "builtin", Selectable: true, IconURL: "/assets/stardew/new-game/farms/standard.png"},
	{ID: "riverland", Label: "河边农场", Kind: "builtin", Enabled: true, Confidence: "builtin", Selectable: true, IconURL: "/assets/stardew/new-game/farms/riverland.png"},
	{ID: "forest", Label: "森林农场", Kind: "builtin", Enabled: true, Confidence: "builtin", Selectable: true, IconURL: "/assets/stardew/new-game/farms/forest.png"},
	{ID: "hilltop", Label: "山顶农场", Kind: "builtin", Enabled: true, Confidence: "builtin", Selectable: true, IconURL: "/assets/stardew/new-game/farms/hilltop.png"},
	{ID: "wilderness", Label: "荒野农场", Kind: "builtin", Enabled: true, Confidence: "builtin", Selectable: true, IconURL: "/assets/stardew/new-game/farms/wilderness.png"},
	{ID: "fourcorners", Label: "四角农场", Kind: "builtin", Enabled: true, Confidence: "builtin", Selectable: true, IconURL: "/assets/stardew/new-game/farms/fourcorners.png"},
	{ID: "beach", Label: "海滩农场", Kind: "builtin", Enabled: true, Confidence: "builtin", Selectable: true, IconURL: "/assets/stardew/new-game/farms/beach.png"},
	{ID: "meadowlands", Label: "草原农场", Kind: "builtin", Enabled: true, Confidence: "builtin", Selectable: true, IconURL: "/assets/stardew/new-game/farms/meadowlands.png"},
}

func (s *server) handleFarmTypeCatalog(w http.ResponseWriter, r *http.Request, instanceID string) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}
	response := farmTypeCatalogResponse{
		FarmTypes:             cloneBuiltinFarmTypes(),
		CatalogWarnings:       []string{},
		ModdedCreationEnabled: s.config.EnableModdedFarmCreation,
	}
	catalog, err := s.farmCatalogScanner(instance.DataDir)
	if err != nil {
		s.logger.Warn("farm catalog scan failed", "instance", instanceID, "error", err)
		response.CatalogWarnings = append(response.CatalogWarnings, "模组农场目录扫描失败，已仅显示官方农场")
		writeJSON(w, http.StatusOK, response)
		return
	}
	response.CatalogWarnings = append(response.CatalogWarnings, catalog.Warnings...)
	selections, dependencyErr := sj.BuildFarmDependencySelections(instance.DataDir, catalog)
	if dependencyErr != nil {
		s.logger.Warn("farm dependency analysis failed", "instance", instanceID, "error", dependencyErr)
		response.CatalogWarnings = append(response.CatalogWarnings, "模组农场依赖分析失败")
	}
	idCounts := make(map[string]int, len(catalog.Farms))
	for _, farm := range catalog.Farms {
		idCounts[farm.ID]++
	}
	for i, farm := range catalog.Farms {
		var selection *sj.NewGameModSelection
		var dependenciesReady *bool
		if dependencyErr == nil && i < len(selections) {
			copySelection := selections[i]
			selection = &copySelection
			ready := copySelection.DependenciesReady
			dependenciesReady = &ready
		}
		selectable := s.config.EnableModdedFarmCreation && farm.Enabled && !farm.Conflict && farm.Confidence == sj.FarmCatalogConfidenceExplicit && selection != nil && selection.DependenciesReady
		item := farmTypeCatalogItem{
			ID: farm.ID, Label: farm.Label, Description: farm.Description, Kind: "modded",
			ProviderModID: farm.ProviderModID, ProviderName: farm.ProviderName, ProviderVersion: farm.ProviderVersion,
			ProviderFolder: farm.ProviderFolder, Enabled: farm.Enabled, Confidence: string(farm.Confidence),
			Conditions: farmTypeConditions(farm.Conditions), Conflict: farm.Conflict, DependenciesReady: dependenciesReady,
			Selectable: selectable, RequiresRuntimeValidation: true,
			Warnings: append([]string{}, farm.ParseWarnings...), ModSelection: selection,
		}
		if farm.IconFile != "" && !farm.Conflict && idCounts[farm.ID] == 1 {
			item.IconURL = fmt.Sprintf("/api/instances/%s/saves/farm-types/%s/icon", url.PathEscape(instanceID), url.PathEscape(farm.ID))
		}
		response.FarmTypes = append(response.FarmTypes, item)
	}
	writeJSON(w, http.StatusOK, response)
}

type prepareFarmTypeRequest struct {
	FarmTypeID string `json:"farmTypeId"`
}

func (s *server) handlePrepareFarmType(w http.ResponseWriter, r *http.Request, instanceID string) {
	actor, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}
	instance, ok = s.ensureInstanceNotRunning(w, r, instance)
	if !ok {
		return
	}
	var request prepareFarmTypeRequest
	if !decodeJSON(w, r, &request) {
		return
	}

	s.farmPrepareMu.Lock()
	defer s.farmPrepareMu.Unlock()
	active, err := s.jobs.Active(r.Context(), storage.ListActiveJobsFilter{TargetType: "instance", TargetID: instanceID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "jobs_read_failed", "读取实例任务状态失败")
		return
	}
	if len(active) > 0 {
		writeError(w, http.StatusConflict, "instance_busy", "实例存在进行中的任务，请等待任务结束后再准备 Mod")
		return
	}
	type farmModPreparer interface {
		PrepareFarmMods(context.Context, registry.Instance, string) (sj.NewGameModSelection, error)
	}
	var selection sj.NewGameModSelection
	driver, driverErr := s.registry.Get(instance.DriverID)
	if driverErr == nil {
		if preparer, supported := driver.(farmModPreparer); supported {
			selection, err = preparer.PrepareFarmMods(r.Context(), registry.Instance{ID: instance.ID, DriverID: instance.DriverID, DataDir: instance.DataDir, State: instance.State}, request.FarmTypeID)
		} else {
			selection, err = sj.PrepareNewGameMods(instance.DataDir, request.FarmTypeID)
		}
	} else {
		selection, err = sj.PrepareNewGameMods(instance.DataDir, request.FarmTypeID)
	}
	if err != nil {
		if selectionErr, typed := sj.IsNewGameModSelectionError(err); typed {
			status := http.StatusConflict
			if selectionErr.Code == "farm_type_not_found" {
				status = http.StatusNotFound
			} else if selectionErr.Code == "farm_type_required" {
				status = http.StatusBadRequest
			}
			writeError(w, status, selectionErr.Code, selectionErr.Message)
			return
		}
		s.logger.Error("prepare new-game mods failed", "instance", instanceID, "farmTypeId", request.FarmTypeID, "error", err)
		if strings.Contains(err.Error(), "rollback failed") {
			writeError(w, http.StatusInternalServerError, "farm_prepare_rollback_failed", "准备失败且未能完整恢复，请勿启动服务器并检查 Mod 状态")
			return
		}
		writeError(w, http.StatusInternalServerError, "farm_prepare_failed", "准备模组农场所需 Mod 失败，已恢复原状态")
		return
	}
	s.auditLog(r, &actor, "farm_mods_prepared", "instance", instanceID, auditMetadata(
		"farmTypeId", selection.FarmTypeID,
		"providerModId", selection.ProviderModID,
		"changedModKeys", strings.Join(selection.ChangedModKeys, ","),
	))
	writeJSON(w, http.StatusOK, selection)
}

func (s *server) handleFarmTypeIcon(w http.ResponseWriter, r *http.Request, instanceID, farmID string) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}
	catalog, err := s.farmCatalogScanner(instance.DataDir)
	if err != nil {
		writeError(w, http.StatusNotFound, "farm_icon_not_found", "农场图标不存在或已变化")
		return
	}
	var match *sj.FarmCatalogEntry
	for i := range catalog.Farms {
		farm := &catalog.Farms[i]
		if farm.ID != farmID || farm.Conflict || farm.IconFile == "" {
			continue
		}
		if match != nil {
			writeError(w, http.StatusNotFound, "farm_icon_not_found", "农场图标不存在或已变化")
			return
		}
		match = farm
	}
	if match == nil {
		writeError(w, http.StatusNotFound, "farm_icon_not_found", "农场图标不存在或已变化")
		return
	}
	asset, err := sj.ReadFarmCatalogIcon(instance.DataDir, *match)
	if err != nil {
		writeError(w, http.StatusNotFound, "farm_icon_not_found", "农场图标不存在或已变化")
		return
	}
	w.Header().Set("Content-Type", asset.MediaType)
	w.Header().Set("Content-Length", strconv.Itoa(len(asset.Data)))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(asset.Data)
}

func cloneBuiltinFarmTypes() []farmTypeCatalogItem {
	result := make([]farmTypeCatalogItem, len(builtinFarmTypes))
	copy(result, builtinFarmTypes)
	for i := range result {
		result[i].Conditions = []farmTypeCondition{}
		result[i].Warnings = []string{}
	}
	return result
}

func farmTypeConditions(conditions []sj.FarmCatalogCondition) []farmTypeCondition {
	result := make([]farmTypeCondition, 0, len(conditions))
	for _, condition := range conditions {
		result = append(result, farmTypeCondition{Kind: condition.Kind, When: append(json.RawMessage(nil), condition.When...)})
	}
	return result
}
