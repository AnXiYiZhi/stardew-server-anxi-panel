package stardew_junimo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

const (
	runtimeFarmCatalogSchemaVersion = 2
	runtimeCatalogRequestVersion    = 1
	maxRuntimeOptionsBytes          = 2 * 1024 * 1024
	runtimeCatalogRequestTTL        = 10 * time.Minute
)

type runtimeCatalogRequest struct {
	SchemaVersion     int       `json:"schemaVersion"`
	RequestID         string    `json:"requestId"`
	TransactionID     string    `json:"transactionId"`
	RequestedFarmType string    `json:"requestedFarmType"`
	GeneratedAt       time.Time `json:"generatedAt"`
	ExpiresAt         time.Time `json:"expiresAt"`
}

type runtimeLoadedMod struct {
	UniqueID string `json:"uniqueId"`
	Version  string `json:"version"`
}

type runtimeFarmType struct {
	ID          string    `json:"id"`
	Label       string    `json:"label"`
	Image       string    `json:"image,omitempty"`
	Kind        string    `json:"kind"`
	GeneratedAt time.Time `json:"generatedAt"`
}

type runtimeFarmCatalog struct {
	SchemaVersion     int                `json:"schemaVersion"`
	Source            string             `json:"source"`
	RequestID         string             `json:"requestId"`
	TransactionID     string             `json:"transactionId"`
	GeneratedAt       time.Time          `json:"generatedAt"`
	ControlModVersion string             `json:"controlModVersion"`
	LoadedMods        []runtimeLoadedMod `json:"loadedMods"`
	ModFingerprint    string             `json:"modFingerprint"`
	FarmTypes         []runtimeFarmType  `json:"farmTypes"`
}

func (tx *newGameTransaction) prepareRuntimeCatalogRequest() error {
	fingerprint, err := expectedRuntimeModFingerprint(tx.dataDir)
	if err != nil {
		return &NewGameTransactionError{Code: "runtime_catalog_prepare_failed", Message: "计算预期 Mod 指纹失败", Cause: err}
	}
	now := time.Now().UTC()
	request := runtimeCatalogRequest{
		SchemaVersion: runtimeCatalogRequestVersion,
		RequestID:     tx.record.TransactionID, TransactionID: tx.record.TransactionID,
		RequestedFarmType: tx.record.RequestedFarmType,
		GeneratedAt:       now, ExpiresAt: now.Add(runtimeCatalogRequestTTL),
	}
	data, err := json.MarshalIndent(request, "", "  ")
	if err != nil {
		return &NewGameTransactionError{Code: "runtime_catalog_prepare_failed", Message: "生成运行时农场目录请求失败", Cause: err}
	}
	if err := os.Remove(runtimeOptionsPath(tx.dataDir)); err != nil && !os.IsNotExist(err) {
		return &NewGameTransactionError{Code: "runtime_catalog_prepare_failed", Message: "清理旧运行时农场目录失败", Cause: err}
	}
	if err := tx.writeJSON(farmCatalogRequestPath(tx.dataDir), data, 0o644); err != nil {
		return &NewGameTransactionError{Code: "runtime_catalog_prepare_failed", Message: "写入运行时农场目录请求失败", Cause: err}
	}
	tx.record.ExpectedFingerprint = fingerprint
	tx.record.Stage = newGameStateCatalogAsked
	return tx.persist()
}

func expectedRuntimeModFingerprint(dataDir string) (string, error) {
	mods, err := listPhysicalMods(dataDir)
	if err != nil {
		return "", err
	}
	loaded := make([]runtimeLoadedMod, 0, len(mods))
	for _, mod := range mods {
		if isSMAPIRuntimeMod(mod) || mod.ParseError != "" || strings.TrimSpace(mod.UniqueID) == "" {
			continue
		}
		// SMAPI loads these bundled utilities from the game runtime even when
		// the panel's mirrored folders are in mods-disabled.
		if !mod.Enabled && !isBundledSMAPILoadedMod(mod.UniqueID) {
			continue
		}
		loaded = append(loaded, runtimeLoadedMod{UniqueID: strings.TrimSpace(mod.UniqueID), Version: strings.TrimSpace(mod.Version)})
	}
	return runtimeModFingerprint(loaded), nil
}

func isBundledSMAPILoadedMod(uniqueID string) bool {
	return strings.EqualFold(strings.TrimSpace(uniqueID), "SMAPI.ConsoleCommands") ||
		strings.EqualFold(strings.TrimSpace(uniqueID), "SMAPI.SaveBackup")
}

func runtimeModFingerprint(mods []runtimeLoadedMod) string {
	canonical := make([]string, 0, len(mods))
	for _, mod := range mods {
		if id := strings.TrimSpace(mod.UniqueID); id != "" {
			canonical = append(canonical, strings.ToLower(id)+"@"+strings.TrimSpace(mod.Version))
		}
	}
	sort.Strings(canonical)
	payload := ""
	if len(canonical) > 0 {
		payload = strings.Join(canonical, "\n") + "\n"
	}
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func readRuntimeFarmCatalog(path string) (runtimeFarmCatalog, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return runtimeFarmCatalog{}, err
	}
	if !stat.Mode().IsRegular() {
		return runtimeFarmCatalog{}, fmt.Errorf("options is not a regular file")
	}
	if stat.Size() > maxRuntimeOptionsBytes {
		return runtimeFarmCatalog{}, fmt.Errorf("options exceeds %d bytes", maxRuntimeOptionsBytes)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return runtimeFarmCatalog{}, err
	}
	var catalog runtimeFarmCatalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		return runtimeFarmCatalog{}, fmt.Errorf("parse runtime options: %w", err)
	}
	return catalog, nil
}

func (tx *newGameTransaction) waitForRuntimeFarmCatalog(ctx context.Context, timeout, interval time.Duration) error {
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	if interval <= 0 {
		interval = time.Second
	}
	deadline := time.Now().Add(timeout)
	path := runtimeOptionsPath(tx.dataDir)
	targetMissing := false
	for time.Now().Before(deadline) {
		catalog, err := readRuntimeFarmCatalog(path)
		if err == nil {
			if catalog.SchemaVersion < runtimeFarmCatalogSchemaVersion {
				if isBuiltinFarmType(tx.record.RequestedFarmType) {
					return nil
				}
				return &NewGameTransactionError{Code: "control_mod_catalog_unsupported", Message: "控制组件不支持模组农场运行时目录"}
			}
			if catalog.SchemaVersion != runtimeFarmCatalogSchemaVersion || catalog.Source != "smapi-runtime" {
				return &NewGameTransactionError{Code: "runtime_catalog_schema_invalid", Message: "运行时农场目录 schema/source 无效"}
			}
			if catalog.RequestID != tx.record.TransactionID || catalog.TransactionID != tx.record.TransactionID {
				return &NewGameTransactionError{Code: "runtime_catalog_stale", Message: "运行时农场目录不属于当前事务"}
			}
			if catalog.GeneratedAt.Before(tx.record.CreatedAt.Add(-time.Second)) || catalog.GeneratedAt.After(time.Now().UTC().Add(5*time.Minute)) {
				return &NewGameTransactionError{Code: "runtime_catalog_stale", Message: "运行时农场目录时间戳无效"}
			}
			if catalog.ModFingerprint == "" || !strings.EqualFold(catalog.ModFingerprint, tx.record.ExpectedFingerprint) {
				return &NewGameTransactionError{Code: "mod_fingerprint_mismatch", Message: "控制组件报告的已加载 Mod 集合与创建准备状态不一致"}
			}
			resolvedFarmType := runtimeCatalogResolvedFarm(catalog.FarmTypes, tx.record.RequestedFarmType)
			if resolvedFarmType == "" {
				targetMissing = true
				goto waitNextCatalog
			}
			tx.record.ResolvedFarmType = resolvedFarmType
			tx.record.Stage = newGameStateCatalogReady
			return tx.persist()
		}
		if !os.IsNotExist(err) {
			code := "runtime_catalog_invalid"
			if strings.Contains(err.Error(), "exceeds") {
				code = "runtime_catalog_too_large"
			}
			return &NewGameTransactionError{Code: code, Message: "读取运行时农场目录失败", Cause: err}
		}
	waitNextCatalog:
		select {
		case <-ctx.Done():
			return &NewGameTransactionError{Code: "runtime_catalog_wait_canceled", Message: "等待运行时农场目录被取消", Cause: ctx.Err()}
		case <-time.After(interval):
		}
	}
	if isBuiltinFarmType(tx.record.RequestedFarmType) {
		// Compatibility with old Control Mod versions: official farms retain the
		// stage-5 behavior, while modded farms can never use this fallback.
		return nil
	}
	if targetMissing {
		return &NewGameTransactionError{Code: "farm_type_not_loaded", Message: "目标农场未在本次启动的 Data/AdditionalFarms 中加载"}
	}
	return &NewGameTransactionError{Code: "runtime_catalog_timeout", Message: "等待匹配 transactionId 的运行时农场目录超时"}
}

func runtimeCatalogHasFarm(farms []runtimeFarmType, requested string) bool {
	return runtimeCatalogResolvedFarm(farms, requested) != ""
}

func runtimeCatalogResolvedFarm(farms []runtimeFarmType, requested string) string {
	wanted := strings.ToLower(strings.TrimSpace(requested))
	aliases := map[string][]string{
		"fourcorners": {"fourcorners", "four_corners", "four-corners"},
		"meadowlands": {"meadowlands", "meadowlandsfarm"},
	}
	candidates := aliases[wanted]
	if len(candidates) == 0 {
		candidates = []string{wanted}
	}
	for _, farm := range farms {
		id := strings.ToLower(strings.TrimSpace(farm.ID))
		for _, candidate := range candidates {
			if id == candidate {
				return farm.ID
			}
		}
	}
	return ""
}
