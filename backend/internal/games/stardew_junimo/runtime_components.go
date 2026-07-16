package stardew_junimo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

const (
	RuntimeComponentsStatusUpToDate        = "up_to_date"
	RuntimeComponentsStatusUpdateAvailable = "update_available"
	RuntimeComponentsStatusGameMissing     = "game_missing"
	RuntimeComponentsStatusSDKMissing      = "sdk_missing"
	RuntimeComponentsStatusManifestInvalid = "manifest_invalid"
	RuntimeComponentsStatusCustomUnknown   = "custom_or_unknown"
)

type RuntimeComponentsDockerService interface {
	DockerVersion(context.Context, string) (paneldocker.CommandResult, error)
	RuntimeImageInspect(context.Context, string, string) (paneldocker.RuntimeImageMetadata, error)
	RuntimeReadContentManifests(context.Context, string, string, string) (paneldocker.RuntimeContentRead, error)
}

type RuntimeContentComponentState struct {
	AppID        string `json:"appId"`
	BuildID      string `json:"buildId,omitempty"`
	StateFlags   string `json:"stateFlags,omitempty"`
	InstallDir   string `json:"installDir,omitempty"`
	LastUpdated  string `json:"lastUpdated,omitempty"`
	ManifestPath string `json:"manifestPath"`
	Status       string `json:"status"`
	Code         string `json:"code"`
	Reason       string `json:"reason"`
}

type RuntimeComponentsCurrent struct {
	Game RuntimeContentComponentState `json:"game"`
	SDK  RuntimeContentComponentState `json:"sdk"`
}

type RuntimeComponentsRecommendation struct {
	StackVersion        string                                   `json:"stackVersion"`
	Channel             string                                   `json:"channel"`
	Status              string                                   `json:"status"`
	MinimumPanelVersion string                                   `json:"minimumPanelVersion"`
	RuntimeUpdatePolicy string                                   `json:"runtimeUpdatePolicy"`
	Game                sjconfig.RuntimeContentManifestComponent `json:"game"`
	SDK                 sjconfig.RuntimeContentManifestComponent `json:"sdk"`
	Tested              bool                                     `json:"tested"`
	ReleaseNotes        []string                                 `json:"releaseNotes"`
}

type RuntimeComponentsInspection struct {
	Available   bool                            `json:"available"`
	Supported   bool                            `json:"supported"`
	Status      string                          `json:"status"`
	Code        string                          `json:"code"`
	Reason      string                          `json:"reason"`
	Current     RuntimeComponentsCurrent        `json:"current"`
	Recommended RuntimeComponentsRecommendation `json:"recommended"`
	DetectedAt  string                          `json:"detectedAt"`
}

type RuntimeComponentsPreflightCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type RuntimeComponentsPreflight struct {
	Phase         string                            `json:"phase"`
	Progress      int                               `json:"progress"`
	Target        RuntimeComponentsRecommendation   `json:"target"`
	Checks        []RuntimeComponentsPreflightCheck `json:"checks"`
	Warnings      []string                          `json:"warnings"`
	RequiredBytes int64                             `json:"requiredBytes"`
	FreeBytes     int64                             `json:"freeBytes,omitempty"`
	GameDataBytes int64                             `json:"gameDataBytes,omitempty"`
	ErrorCode     string                            `json:"errorCode,omitempty"`
	Error         string                            `json:"error,omitempty"`
	UpdatedAt     string                            `json:"updatedAt,omitempty"`
}

func (d *Driver) InspectRuntimeComponents(ctx context.Context, instance registry.Instance) (RuntimeComponentsInspection, error) {
	manifest, err := sjconfig.BuiltInRuntimeStackManifest()
	if err != nil {
		return RuntimeComponentsInspection{}, fmt.Errorf("load runtime component matrix: %w", err)
	}
	result := newRuntimeComponentsInspection(manifest)
	if !sjconfig.PanelVersionSatisfies(d.panelVersion, manifest.MinimumPanelVersion) {
		result.Status = RuntimeComponentsStatusCustomUnknown
		result.Code = "panel_version_incompatible"
		result.Reason = "当前 Panel 版本低于兼容矩阵要求的最低版本。"
		return result, nil
	}
	if !runtimeContentInstalled(instance.State) {
		result.Supported = true
		result.Status = RuntimeComponentsStatusGameMissing
		result.Code = "not_installed"
		result.Reason = "实例尚未安装游戏运行文件。"
		result.Current.Game = missingContentState("413150", "steamapps/appmanifest_413150.acf", RuntimeComponentsStatusGameMissing)
		result.Current.SDK = missingContentState("1007", ".steam-sdk/steamapps/appmanifest_1007.acf", RuntimeComponentsStatusSDKMissing)
		return result, nil
	}
	dockerReader, ok := d.docker.(RuntimeComponentsDockerService)
	if !ok {
		result.Status = RuntimeComponentsStatusCustomUnknown
		result.Code = "unsupported/docker_contract"
		result.Reason = "当前 Docker driver 不支持只读运行文件检测。"
		return result, nil
	}
	project := strings.ToLower(filepath.Base(filepath.Clean(instance.DataDir)))
	if !filepath.IsAbs(instance.DataDir) || !runtimeComposeProjectPattern.MatchString(project) {
		result.Status = RuntimeComponentsStatusCustomUnknown
		result.Code = "unsupported/instance_path"
		result.Reason = "实例目录无法安全映射到 game-data volume。"
		return result, nil
	}
	stack := InspectRuntimeStack(instance.DataDir, instance.State)
	image := stack.Current.Server.Image
	if strings.TrimSpace(image) == "" {
		image = manifest.Server.Image
	}
	volume, volumeErr := GameDataVolumeName(instance.DataDir)
	if volumeErr != nil {
		result.Status = RuntimeComponentsStatusCustomUnknown
		result.Code = "invalid_game_data_volume"
		result.Reason = "GAME_DATA_VOLUME 配置无效。"
		return result, nil
	}
	read, err := dockerReader.RuntimeReadContentManifests(ctx, instance.DataDir, volume, image)
	if err != nil {
		result.Status = RuntimeComponentsStatusCustomUnknown
		result.Code = "runtime_read_unavailable"
		result.Reason = "game-data volume 或本地运行镜像当前不可只读访问。"
		return result, nil
	}
	return inspectRuntimeComponentData(manifest, read.GameManifest, read.SDKManifest, result.DetectedAt), nil
}

func inspectRuntimeComponentData(manifest sjconfig.RuntimeStackManifest, gameData, sdkData []byte, detectedAt string) RuntimeComponentsInspection {
	result := newRuntimeComponentsInspection(manifest)
	if detectedAt != "" {
		result.DetectedAt = detectedAt
	}
	result.Supported = true
	if len(gameData) == 0 {
		result.Status = RuntimeComponentsStatusGameMissing
		result.Code = "game_manifest_missing"
		result.Reason = "Stardew Valley appmanifest 不存在。"
		result.Current.Game = missingContentState("413150", "steamapps/appmanifest_413150.acf", RuntimeComponentsStatusGameMissing)
		result.Current.SDK = inspectContentManifest(sdkData, "1007", ".steam-sdk/steamapps/appmanifest_1007.acf", manifest.SDK.BuildID)
		return result
	}
	if len(sdkData) == 0 {
		result.Status = RuntimeComponentsStatusSDKMissing
		result.Code = "sdk_manifest_missing"
		result.Reason = "Steamworks SDK appmanifest 不存在。"
		result.Current.Game = inspectContentManifest(gameData, "413150", "steamapps/appmanifest_413150.acf", manifest.Game.BuildID)
		result.Current.SDK = missingContentState("1007", ".steam-sdk/steamapps/appmanifest_1007.acf", RuntimeComponentsStatusSDKMissing)
		return result
	}
	result.Current.Game = inspectContentManifest(gameData, "413150", "steamapps/appmanifest_413150.acf", manifest.Game.BuildID)
	result.Current.SDK = inspectContentManifest(sdkData, "1007", ".steam-sdk/steamapps/appmanifest_1007.acf", manifest.SDK.BuildID)
	if result.Current.Game.Status == RuntimeComponentsStatusManifestInvalid || result.Current.SDK.Status == RuntimeComponentsStatusManifestInvalid {
		result.Status = RuntimeComponentsStatusManifestInvalid
		result.Code = "manifest_invalid"
		result.Reason = "游戏或联机运行库 appmanifest 无效。"
		return result
	}
	if result.Current.Game.Status == RuntimeComponentsStatusCustomUnknown || result.Current.SDK.Status == RuntimeComponentsStatusCustomUnknown || !manifest.Installable() {
		result.Status = RuntimeComponentsStatusCustomUnknown
		result.Code = "custom_or_unknown"
		result.Reason = "manifest 可读取，但安装状态或推荐矩阵无法确认。"
		return result
	}
	if result.Current.Game.BuildID == manifest.Game.BuildID && result.Current.SDK.BuildID == manifest.SDK.BuildID {
		result.Status = RuntimeComponentsStatusUpToDate
		result.Code = "up_to_date"
		result.Reason = "游戏版本与联机运行库均匹配已测试的推荐矩阵。"
		return result
	}
	result.Available = true
	result.Status = RuntimeComponentsStatusUpdateAvailable
	result.Code = "update_available"
	result.Reason = "当前游戏运行文件与已测试的推荐矩阵不一致。"
	return result
}

func inspectContentManifest(data []byte, appID, path, recommendedBuildID string) RuntimeContentComponentState {
	state := RuntimeContentComponentState{AppID: appID, ManifestPath: path}
	parsed, err := parseSteamAppManifest(data, appID)
	if err != nil {
		state.Status = RuntimeComponentsStatusManifestInvalid
		state.Code = "manifest_invalid"
		state.Reason = "appmanifest 缺少必需字段、字段非法或 appid 不匹配。"
		return state
	}
	state.BuildID = parsed.BuildID
	state.StateFlags = parsed.StateFlags
	state.InstallDir = parsed.InstallDir
	state.LastUpdated = parsed.LastUpdated
	flags, _ := strconv.ParseUint(parsed.StateFlags, 10, 64)
	if flags&4 == 0 {
		state.Status = RuntimeComponentsStatusCustomUnknown
		state.Code = "state_flags_unknown"
		state.Reason = "Steam StateFlags 未标记为完整安装。"
		return state
	}
	if parsed.BuildID == recommendedBuildID {
		state.Status = RuntimeComponentsStatusUpToDate
		state.Code = "up_to_date"
		state.Reason = "buildid 匹配推荐矩阵。"
	} else {
		state.Status = RuntimeComponentsStatusUpdateAvailable
		state.Code = "build_mismatch"
		state.Reason = "buildid 与推荐矩阵不一致；未按数字大小判断新旧。"
	}
	return state
}

func newRuntimeComponentsInspection(manifest sjconfig.RuntimeStackManifest) RuntimeComponentsInspection {
	return RuntimeComponentsInspection{
		Recommended: runtimeComponentsRecommendation(manifest),
		DetectedAt:  time.Now().UTC().Format(time.RFC3339),
	}
}

func runtimeComponentsRecommendation(manifest sjconfig.RuntimeStackManifest) RuntimeComponentsRecommendation {
	return RuntimeComponentsRecommendation{
		StackVersion: manifest.StackVersion, Channel: manifest.Channel, Status: manifest.Status,
		MinimumPanelVersion: manifest.MinimumPanelVersion, RuntimeUpdatePolicy: manifest.RuntimeUpdatePolicy, Game: manifest.Game, SDK: manifest.SDK,
		Tested: manifest.Tested, ReleaseNotes: append([]string{}, manifest.ReleaseNotes...),
	}
}

func missingContentState(appID, path, status string) RuntimeContentComponentState {
	return RuntimeContentComponentState{AppID: appID, ManifestPath: path, Status: status, Code: "manifest_missing", Reason: "appmanifest 不存在。"}
}

func runtimeContentInstalled(state string) bool {
	return state != storage.InstanceStateUninitialized && state != storage.InstanceStateAdminCreated
}

func (d *Driver) RunRuntimeComponentsPreflight(ctx context.Context, instance registry.Instance) (RuntimeComponentsPreflight, error) {
	inspection, err := d.InspectRuntimeComponents(ctx, instance)
	if err != nil {
		return RuntimeComponentsPreflight{}, err
	}
	preflight := RuntimeComponentsPreflight{Phase: "checking", Progress: 10, Target: inspection.Recommended, Checks: []RuntimeComponentsPreflightCheck{}, Warnings: []string{}, UpdatedAt: time.Now().UTC().Format(time.RFC3339)}
	add := func(name, status, message string) {
		preflight.Checks = append(preflight.Checks, RuntimeComponentsPreflightCheck{Name: name, Status: status, Message: paneldocker.RedactString(message)})
	}
	finishError := func(code, message string) (RuntimeComponentsPreflight, error) {
		preflight.Phase, preflight.Progress, preflight.ErrorCode, preflight.Error = "failed", 100, code, message
		_ = writeRuntimeComponentsPreflight(instance.DataDir, preflight)
		return preflight, nil
	}
	if inspection.Recommended.Status != sjconfig.RuntimeMatrixStatusRecommended || !sjconfig.PanelVersionSatisfies(d.panelVersion, inspection.Recommended.MinimumPanelVersion) {
		add("recommended_matrix", "error", "内置推荐矩阵未经 tested 标记。")
		return finishError("matrix_untested", "内置推荐矩阵未经验证。")
	}
	add("recommended_matrix", "ok", "目标 buildid 仅来自当前 Panel 内置的 tested 推荐矩阵。")
	if inspection.Status == RuntimeComponentsStatusGameMissing || inspection.Status == RuntimeComponentsStatusSDKMissing || inspection.Status == RuntimeComponentsStatusManifestInvalid || inspection.Status == RuntimeComponentsStatusCustomUnknown {
		add("current_manifests", "error", inspection.Reason)
		return finishError(inspection.Code, inspection.Reason)
	}
	add("current_manifests", "ok", "两份固定 appmanifest 已只读解析且 appid/buildid/StateFlags 合法。")
	dockerReader, ok := d.docker.(RuntimeComponentsDockerService)
	if !ok {
		return finishError("unsupported/docker_contract", "当前 Docker driver 不支持只读预检。")
	}
	project := strings.ToLower(filepath.Base(filepath.Clean(instance.DataDir)))
	stack := InspectRuntimeStack(instance.DataDir, instance.State)
	image := stack.Current.Server.Image
	if image == "" {
		image = sjconfig.DefaultServerImage
	}
	volume, volumeErr := GameDataVolumeName(instance.DataDir)
	if volumeErr != nil {
		return finishError("invalid_game_data_volume", "GAME_DATA_VOLUME 配置无效。")
	}
	read, readErr := dockerReader.RuntimeReadContentManifests(ctx, instance.DataDir, volume, image)
	preflight.RequiredBytes = (inspection.Recommended.Game.EstimatedDownloadBytes + inspection.Recommended.SDK.EstimatedDownloadBytes) * 2
	if readErr != nil {
		return finishError("disk_check_unavailable", "无法只读获取 game-data 可用空间。")
	}
	preflight.FreeBytes = read.FreeBytes
	preflight.GameDataBytes = read.GameDataBytes
	if read.FreeBytes > 0 && read.FreeBytes < preflight.RequiredBytes {
		add("disk_space", "error", "game-data 可用空间低于下载与 staging 保守估算。")
		return finishError("disk_space_insufficient", "可用磁盘空间不足。")
	}
	if read.FreeBytes == 0 {
		add("disk_space", "warning", "无法取得可靠可用空间数值。")
		preflight.Warnings = append(preflight.Warnings, "磁盘空间只能在执行阶段重新确认；本阶段不会创建 staging volume。")
	} else {
		add("disk_space", "ok", "game-data 可用空间满足下载与 staging 保守估算。")
	}
	steamCMDReady := false
	for _, candidate := range strings.Split(sjconfig.DefaultSteamCMDImageCandidates, ",") {
		if _, inspectErr := dockerReader.RuntimeImageInspect(ctx, instance.DataDir, strings.TrimSpace(candidate)); inspectErr == nil {
			steamCMDReady = true
			break
		}
	}
	if !steamCMDReady {
		add("steam_download", "warning", "本地没有可确认的 SteamCMD 候选镜像；预检不会拉取镜像。")
		preflight.Warnings = append(preflight.Warnings, "未联网登录 Steam，也未执行 app_info_print 或 app_update。")
	} else {
		values, _ := sjconfig.ReadEnvFile(filepath.Join(instance.DataDir, ".env"))
		if strings.EqualFold(strings.TrimSpace(values["STEAMCMD_AUTH_COMPLETED"]), "true") {
			add("steam_download", "ok", "本地 SteamCMD 与游戏账号缓存标志存在；SDK 可匿名获取。未发起网络或下载。")
		} else {
			add("steam_download", "warning", "本地 SteamCMD 存在，但 413150 下载仍可能需要管理员重新完成 Steam 授权。")
		}
	}
	if _, dockerErr := dockerReader.DockerVersion(ctx, instance.DataDir); dockerErr != nil || !runtimeComposeProjectPattern.MatchString(project) {
		return finishError("staging_capability_unavailable", "Docker daemon 或 staging 命名能力不可用。")
	}
	add("staging_volume", "ok", "Docker daemon 与受控 staging volume 命名规则可用；本阶段没有创建 volume。")
	preflight.Phase, preflight.Progress, preflight.UpdatedAt = "succeeded", 100, time.Now().UTC().Format(time.RFC3339)
	if err := writeRuntimeComponentsPreflight(instance.DataDir, preflight); err != nil {
		return RuntimeComponentsPreflight{}, err
	}
	return preflight, nil
}

func (d *Driver) RuntimeComponentsPreflight(instance registry.Instance) (RuntimeComponentsPreflight, error) {
	data, err := os.ReadFile(runtimeComponentsPreflightPath(instance.DataDir))
	if errors.Is(err, os.ErrNotExist) {
		manifest, manifestErr := sjconfig.BuiltInRuntimeStackManifest()
		if manifestErr != nil {
			return RuntimeComponentsPreflight{}, manifestErr
		}
		return RuntimeComponentsPreflight{Phase: "idle", Target: runtimeComponentsRecommendation(manifest), Checks: []RuntimeComponentsPreflightCheck{}, Warnings: []string{}}, nil
	}
	if err != nil {
		return RuntimeComponentsPreflight{}, err
	}
	var status RuntimeComponentsPreflight
	if err := json.Unmarshal(data, &status); err != nil {
		return RuntimeComponentsPreflight{}, err
	}
	if status.Checks == nil {
		status.Checks = []RuntimeComponentsPreflightCheck{}
	}
	if status.Warnings == nil {
		status.Warnings = []string{}
	}
	return status, nil
}

func runtimeComponentsPreflightPath(dataDir string) string {
	return filepath.Join(dataDir, ".local-container", "runtime-components", "preflight-status.json")
}

func writeRuntimeComponentsPreflight(dataDir string, status RuntimeComponentsPreflight) error {
	path := runtimeComponentsPreflightPath(dataDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".preflight-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return replaceRuntimeUpdateStatusFile(name, path)
}
