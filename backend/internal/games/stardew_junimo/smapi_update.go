package stardew_junimo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
)

const (
	SMAPIStatusUpToDate           = "up_to_date"
	SMAPIStatusUpdateAvailable    = "update_available"
	SMAPIStatusMissing            = "missing"
	SMAPIStatusInvalid            = "invalid"
	SMAPIStatusIncompatibleGame   = "incompatible_game"
	SMAPIStatusIncompatibleJunimo = "incompatible_junimo"
	SMAPIStatusCustomUnknown      = "custom_or_unknown"
)

var smapiInformationalVersionPattern = regexp.MustCompile(`^([0-9]+\.[0-9]+\.[0-9]+)(?:\.[0-9]+)?\+[0-9a-f]{7,64}$`)
var smapiExactVersionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:\.[0-9]+)?$`)

type SMAPIDetectionDockerService interface {
	RuntimeReadSMAPIMetadata(context.Context, string, string, string) (paneldocker.RuntimeSMAPIMetadata, error)
}

type SMAPICurrent struct {
	Version           string `json:"version,omitempty"`
	ConfiguredVersion string `json:"configuredVersion,omitempty"`
	VersionSource     string `json:"versionSource,omitempty"`
	Present           bool   `json:"present"`
	RequiredFiles     bool   `json:"requiredFiles"`
	GameDataVolume    string `json:"gameDataVolume,omitempty"`
}

type SMAPICompatibility struct {
	GameBuildID          string `json:"gameBuildId"`
	SDKBuildID           string `json:"sdkBuildId"`
	JunimoVersion        string `json:"junimoVersion"`
	SteamAuthVersion     string `json:"steamAuthVersion"`
	ControlVersion       string `json:"controlVersion"`
	ControlDLLSHA256     string `json:"controlDllSha256"`
	CommandResultVersion int    `json:"commandResultVersion"`
}

type SMAPIRecommendation struct {
	Version       string             `json:"version"`
	DownloadURL   string             `json:"downloadUrl"`
	SHA256        string             `json:"sha256"`
	ArchiveBytes  int64              `json:"archiveBytes"`
	Compatibility SMAPICompatibility `json:"compatibility"`
}

type SMAPIUpdateInfo struct {
	Available   bool                `json:"available"`
	Supported   bool                `json:"supported"`
	Status      string              `json:"status"`
	Code        string              `json:"code"`
	Reason      string              `json:"reason"`
	Current     SMAPICurrent        `json:"current"`
	Recommended SMAPIRecommendation `json:"recommended"`
	DetectedAt  string              `json:"detectedAt"`
}

func (d *Driver) InspectSMAPIUpdate(ctx context.Context, instance registry.Instance) (SMAPIUpdateInfo, error) {
	manifest, err := sjconfig.BuiltInRuntimeStackManifest()
	if err != nil {
		return SMAPIUpdateInfo{}, err
	}
	result := SMAPIUpdateInfo{Recommended: smapiRecommendation(manifest), DetectedAt: time.Now().UTC().Format(time.RFC3339)}
	values, _ := sjconfig.ReadEnvFile(filepath.Join(instance.DataDir, ".env"))
	result.Current.ConfiguredVersion = strings.TrimSpace(values["SMAPI_VERSION"])
	if !runtimeContentInstalled(instance.State) {
		result.Supported, result.Status, result.Code, result.Reason = true, SMAPIStatusMissing, "not_installed", "实例尚未安装游戏与 SMAPI。"
		return result, nil
	}
	volume, err := GameDataVolumeName(instance.DataDir)
	if err != nil {
		result.Status, result.Code, result.Reason = SMAPIStatusInvalid, "invalid_game_data_volume", "GAME_DATA_VOLUME 配置无效。"
		return result, nil
	}
	result.Current.GameDataVolume = volume
	stack := InspectRuntimeStack(instance.DataDir, instance.State)
	image := stack.Current.Server.Image
	if image == "" {
		image = manifest.Server.Image
	}
	reader, ok := d.docker.(SMAPIDetectionDockerService)
	if !ok {
		result.Status, result.Code, result.Reason = SMAPIStatusCustomUnknown, "unsupported/docker_contract", "当前 Docker driver 不支持 SMAPI 实际文件检测。"
		return result, nil
	}
	metadata, err := reader.RuntimeReadSMAPIMetadata(ctx, instance.DataDir, volume, image)
	if err != nil {
		result.Status, result.Code, result.Reason = SMAPIStatusCustomUnknown, "runtime_read_unavailable", "当前无法只读访问 SMAPI 安装产物。"
		return result, nil
	}
	result.Current.Present, result.Current.RequiredFiles, result.Current.VersionSource = metadata.Present, metadata.RequiredFiles, metadata.VersionEvidence
	if !metadata.Present {
		result.Supported, result.Available, result.Status, result.Code, result.Reason = true, true, SMAPIStatusMissing, "smapi_missing", "实际游戏目录中未找到 SMAPI 程序集。"
		return result, nil
	}
	match := smapiInformationalVersionPattern.FindStringSubmatch(metadata.Version)
	if !metadata.RequiredFiles || len(match) != 2 {
		result.Status, result.Code, result.Reason = SMAPIStatusInvalid, "smapi_install_invalid", "SMAPI 安装产物不完整或程序集版本元数据无效。"
		return result, nil
	}
	result.Current.Version = match[1]
	components, componentErr := d.InspectRuntimeComponents(ctx, instance)
	if componentErr != nil {
		return result, componentErr
	}
	if components.Status != RuntimeComponentsStatusUpToDate {
		result.Status, result.Code, result.Reason = SMAPIStatusIncompatibleGame, "incompatible_game_or_sdk", "当前 Stardew buildid 或 Steamworks SDK buildid 不匹配推荐前置矩阵。"
		return result, nil
	}
	if stack.Status != sjconfig.RuntimeStackStatusUpToDate {
		result.Status, result.Code, result.Reason = SMAPIStatusIncompatibleJunimo, "incompatible_junimo_or_auth", "当前 Junimo server 或 steam-auth-cn 不匹配推荐前置矩阵。"
		return result, nil
	}
	if err := verifyEmbeddedControlManifest(manifest); err != nil {
		result.Status, result.Code, result.Reason = SMAPIStatusCustomUnknown, "control_manifest_mismatch", "Panel 内置控制 Mod 与推荐矩阵不一致。"
		return result, nil
	}
	cmp := compareDottedVersions(result.Current.Version, manifest.SMAPI.Version)
	result.Supported = true
	if cmp == 0 {
		result.Status, result.Code, result.Reason = SMAPIStatusUpToDate, "up_to_date", "实际游戏目录中的 SMAPI 与 Panel 推荐版本完全匹配。"
	} else if cmp < 0 {
		result.Available, result.Status, result.Code, result.Reason = true, SMAPIStatusUpdateAvailable, "update_available", "检测到经过验证的 SMAPI 推荐升级。"
	} else {
		result.Status, result.Code, result.Reason = SMAPIStatusCustomUnknown, "newer_or_custom_smapi", "当前 SMAPI 高于 Panel 推荐版本或属于未知构建；Panel 不会自动降级。"
	}
	return result, nil
}

func smapiRecommendation(manifest sjconfig.RuntimeStackManifest) SMAPIRecommendation {
	return SMAPIRecommendation{Version: manifest.SMAPI.Version, DownloadURL: manifest.SMAPI.DownloadURL, SHA256: manifest.SMAPI.SHA256, ArchiveBytes: manifest.SMAPI.ArchiveBytes, Compatibility: SMAPICompatibility{
		GameBuildID: manifest.Game.BuildID, SDKBuildID: manifest.SDK.BuildID, JunimoVersion: manifest.Server.Tag, SteamAuthVersion: manifest.SteamAuth.Tag,
		ControlVersion: manifest.Control.Version, ControlDLLSHA256: manifest.Control.DLLSHA256, CommandResultVersion: manifest.Control.CommandResultVersion,
	}}
}

func verifyEmbeddedControlManifest(manifest sjconfig.RuntimeStackManifest) error {
	var control struct {
		Version string `json:"Version"`
	}
	if err := jsonUnmarshalStrictEnough(smapiModManifest, &control); err != nil || control.Version != manifest.Control.Version {
		return errors.New("control manifest version mismatch")
	}
	h := sha256.Sum256(smapiModDLL)
	if hex.EncodeToString(h[:]) != manifest.Control.DLLSHA256 {
		return errors.New("control dll digest mismatch")
	}
	return nil
}

func jsonUnmarshalStrictEnough(data []byte, target any) error {
	// Kept as a small seam for tests; embedded data is build-time trusted.
	return json.Unmarshal(data, target)
}

func compareDottedVersions(a, b string) int {
	parse := func(value string) []int {
		parts := strings.Split(value, ".")
		out := make([]int, len(parts))
		for i, part := range parts {
			out[i], _ = strconv.Atoi(part)
		}
		return out
	}
	left, right := parse(a), parse(b)
	for i := 0; i < 4; i++ {
		lv, rv := 0, 0
		if i < len(left) {
			lv = left[i]
		}
		if i < len(right) {
			rv = right[i]
		}
		if lv < rv {
			return -1
		}
		if lv > rv {
			return 1
		}
	}
	return 0
}
