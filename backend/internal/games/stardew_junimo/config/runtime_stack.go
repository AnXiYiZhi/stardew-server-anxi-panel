package config

import (
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

const (
	RuntimeStackStatusUpToDate        = "up_to_date"
	RuntimeStackStatusUpdateAvailable = "update_available"
	RuntimeStackStatusNotInstalled    = "not_installed"
	RuntimeStackStatusCustomImages    = "custom_images"
	RuntimeStackStatusInvalidConfig   = "invalid_config"
	RuntimeStackStatusWithdrawn       = "withdrawn"
	RuntimeStackStatusNotRecommended  = "not_recommended"
)

var (
	//go:embed runtime_stack_manifest.json
	runtimeStackManifestJSON []byte

	runtimeStackManifestOnce sync.Once
	runtimeStackManifest     RuntimeStackManifest
	runtimeStackManifestErr  error
)

type RuntimeStackManifest struct {
	SchemaVersion       int                             `json:"schemaVersion"`
	StackVersion        string                          `json:"stackVersion"`
	Channel             string                          `json:"channel"`
	MinimumPanelVersion string                          `json:"minimumPanelVersion"`
	Server              RuntimeStackManifestComponent   `json:"server"`
	SteamAuth           RuntimeStackManifestComponent   `json:"steamAuth"`
	Game                RuntimeContentManifestComponent `json:"game"`
	SDK                 RuntimeContentManifestComponent `json:"sdk"`
	SMAPI               SMAPIManifestComponent          `json:"smapi"`
	Control             ControlManifestComponent        `json:"controlMod"`
	Status              string                          `json:"status"`
	Withdrawal          *RuntimeStackWithdrawal         `json:"withdrawal,omitempty"`
	ReleaseNotes        []string                        `json:"releaseNotes"`
	Tested              bool                            `json:"-"`
}

const (
	RuntimeMatrixStatusRecommended = "recommended"
	RuntimeMatrixStatusWithdrawn   = "withdrawn"
)

type RuntimeStackWithdrawal struct {
	Reason               string `json:"reason"`
	FallbackStackVersion string `json:"fallbackStackVersion"`
	WithdrawnAt          string `json:"withdrawnAt,omitempty"`
}

type SMAPIManifestComponent struct {
	Version         string   `json:"version"`
	URLs            []string `json:"urls"`
	DownloadURL     string   `json:"-"`
	SHA256          string   `json:"sha256"`
	ArchiveBytes    int64    `json:"archiveBytes"`
	MaxArchiveBytes int64    `json:"maxArchiveBytes"`
	MaxExtractBytes int64    `json:"maxExtractBytes"`
	TrustedHosts    []string `json:"trustedHosts"`
}

type ControlManifestComponent struct {
	Version              string `json:"version"`
	DLLSHA256            string `json:"dllSha256"`
	CommandResultVersion int    `json:"commandResultVersion"`
}

type RuntimeContentManifestComponent struct {
	AppID                  string   `json:"appId"`
	BuildID                string   `json:"buildId"`
	ManifestVersion        string   `json:"manifestVersion"`
	Notes                  []string `json:"notes"`
	EstimatedDownloadBytes int64    `json:"estimatedDownloadBytes"`
}

type RuntimeStackManifestComponent struct {
	Tag               string            `json:"tag"`
	UpstreamRef       string            `json:"upstreamRef,omitempty"`
	SourceRevision    string            `json:"sourceRevision,omitempty"`
	Images            []string          `json:"images"`
	Digests           map[string]string `json:"digests"`
	Image             string            `json:"-"`
	TrustedCandidates []string          `json:"-"`
}

func (component *RuntimeStackManifestComponent) UnmarshalJSON(data []byte) error {
	type wire RuntimeStackManifestComponent
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*component = RuntimeStackManifestComponent(decoded)
	component.normalize()
	return nil
}

func (component *SMAPIManifestComponent) UnmarshalJSON(data []byte) error {
	type wire SMAPIManifestComponent
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*component = SMAPIManifestComponent(decoded)
	if len(component.URLs) > 0 {
		component.DownloadURL = strings.TrimSpace(component.URLs[0])
	}
	return nil
}

type RuntimeStackComponentState struct {
	Image string `json:"image,omitempty"`
	Tag   string `json:"tag,omitempty"`
}

type RuntimeStackCurrent struct {
	Server    RuntimeStackComponentState `json:"server"`
	SteamAuth RuntimeStackComponentState `json:"steamAuth"`
}

type RuntimeStackRecommendation struct {
	StackVersion        string                          `json:"stackVersion"`
	Channel             string                          `json:"channel"`
	MinimumPanelVersion string                          `json:"minimumPanelVersion"`
	Server              RuntimeStackManifestComponent   `json:"server"`
	SteamAuth           RuntimeStackManifestComponent   `json:"steamAuth"`
	Game                RuntimeContentManifestComponent `json:"game"`
	SDK                 RuntimeContentManifestComponent `json:"sdk"`
	SMAPI               SMAPIManifestComponent          `json:"smapi"`
	Control             ControlManifestComponent        `json:"controlMod"`
	Status              string                          `json:"status"`
	Withdrawal          *RuntimeStackWithdrawal         `json:"withdrawal,omitempty"`
	ReleaseNotes        []string                        `json:"releaseNotes"`
	Tested              bool                            `json:"tested"`
}

type RuntimeStackInspection struct {
	Available   bool                       `json:"available"`
	Supported   bool                       `json:"supported"`
	Status      string                     `json:"status"`
	Code        string                     `json:"code"`
	Reason      string                     `json:"reason"`
	Current     RuntimeStackCurrent        `json:"current"`
	Recommended RuntimeStackRecommendation `json:"recommended"`
}

var trustedRuntimeRepositories = map[string]map[string]struct{}{
	"server": {
		"dockerproxy.net/sdvd/server":    {},
		"docker.1ms.run/sdvd/server":     {},
		"docker.1panel.live/sdvd/server": {},
		"docker.jiaxin.site/sdvd/server": {},
		"dockerproxy.link/sdvd/server":   {},
		"sdvd/server":                    {},
	},
	"steamAuth": {
		"docker.1ms.run/anxiyizhi/junimo-steam-service-cn":                                              {},
		"crpi-9z3bkb9g7fxeohrg.cn-hangzhou.personal.cr.aliyuncs.com/anxi-panel/junimo-steam-service-cn": {},
		"docker.m.daocloud.io/anxiyizhi/junimo-steam-service-cn":                                        {},
		"ghcr.io/anxiyizhi/junimo-steam-service-cn":                                                     {},
		"anxiyizhi/junimo-steam-service-cn":                                                             {},
	},
}

func BuiltInRuntimeStackManifest() (RuntimeStackManifest, error) {
	runtimeStackManifestOnce.Do(func() {
		if err := json.Unmarshal(runtimeStackManifestJSON, &runtimeStackManifest); err != nil {
			runtimeStackManifestErr = fmt.Errorf("decode runtime stack manifest: %w", err)
			return
		}
		normalizeRuntimeStackManifest(&runtimeStackManifest)
		runtimeStackManifestErr = ValidateRuntimeStackManifest(runtimeStackManifest)
	})
	return runtimeStackManifest, runtimeStackManifestErr
}

func ValidateRuntimeStackManifest(manifest RuntimeStackManifest) error {
	if manifest.SchemaVersion <= 0 {
		return errors.New("schemaVersion must be positive")
	}
	if strings.TrimSpace(manifest.StackVersion) == "" {
		return errors.New("stackVersion is required")
	}
	if strings.TrimSpace(manifest.MinimumPanelVersion) == "" {
		return errors.New("minimumPanelVersion is required")
	}
	if manifest.Channel != "stable" && manifest.Channel != "preview" {
		return errors.New("channel must be stable or preview")
	}
	switch manifest.Status {
	case RuntimeMatrixStatusRecommended:
		if manifest.Withdrawal != nil {
			return errors.New("withdrawal is only valid for withdrawn matrices")
		}
	case RuntimeMatrixStatusWithdrawn:
		if manifest.Withdrawal == nil || strings.TrimSpace(manifest.Withdrawal.Reason) == "" || strings.TrimSpace(manifest.Withdrawal.FallbackStackVersion) == "" {
			return errors.New("withdrawn matrix requires reason and fallbackStackVersion")
		}
	default:
		return errors.New("invalid runtime matrix status")
	}
	if err := validateManifestComponent("server", manifest.Server); err != nil {
		return err
	}
	if err := validateManifestComponent("steamAuth", manifest.SteamAuth); err != nil {
		return err
	}
	if !strings.HasPrefix(manifest.SteamAuth.UpstreamRef, "refs/tags/") || strings.TrimSpace(strings.TrimPrefix(manifest.SteamAuth.UpstreamRef, "refs/tags/")) == "" {
		return errors.New("steamAuth upstreamRef must be an exact tag ref")
	}
	if len(manifest.SteamAuth.SourceRevision) != 40 || !isHexDigest40(strings.ToLower(manifest.SteamAuth.SourceRevision)) {
		return errors.New("steamAuth sourceRevision must be a 40-character git revision")
	}
	if err := validateContentManifestComponent("game", "413150", manifest.Game); err != nil {
		return err
	}
	if err := validateContentManifestComponent("sdk", "1007", manifest.SDK); err != nil {
		return err
	}
	if err := validateSMAPIManifestComponent(manifest.SMAPI); err != nil {
		return err
	}
	if err := validateControlManifestComponent(manifest.Control); err != nil {
		return err
	}
	if len(manifest.ReleaseNotes) == 0 {
		return errors.New("releaseNotes are required")
	}
	return nil
}

func normalizeRuntimeStackManifest(manifest *RuntimeStackManifest) {
	manifest.Server.normalize()
	manifest.SteamAuth.normalize()
	if len(manifest.SMAPI.URLs) > 0 {
		manifest.SMAPI.DownloadURL = strings.TrimSpace(manifest.SMAPI.URLs[0])
	}
	manifest.Tested = manifest.Status == RuntimeMatrixStatusRecommended
}

func (component *RuntimeStackManifestComponent) normalize() {
	if len(component.Images) == 0 {
		return
	}
	component.Image = strings.TrimSpace(component.Images[0])
	component.TrustedCandidates = append([]string(nil), component.Images...)
}

func (manifest RuntimeStackManifest) Installable() bool {
	return manifest.Status == RuntimeMatrixStatusRecommended
}

func PanelVersionSatisfies(current, minimum string) bool {
	current = strings.TrimPrefix(strings.TrimSpace(current), "v")
	minimum = strings.TrimPrefix(strings.TrimSpace(minimum), "v")
	if current == "dev" {
		return true
	}
	left, leftOK := numericVersion(current)
	right, rightOK := numericVersion(minimum)
	if !leftOK || !rightOK {
		return false
	}
	for i := 0; i < len(left); i++ {
		if left[i] != right[i] {
			return left[i] > right[i]
		}
	}
	return true
}

func numericVersion(value string) ([4]int, bool) {
	var result [4]int
	parts := strings.Split(value, ".")
	if len(parts) < 3 || len(parts) > 4 {
		return result, false
	}
	for i, part := range parts {
		number, err := strconv.Atoi(part)
		if err != nil || number < 0 {
			return result, false
		}
		result[i] = number
	}
	return result, true
}

func validateSMAPIManifestComponent(component SMAPIManifestComponent) error {
	if len(component.URLs) == 0 {
		return errors.New("smapi urls are required")
	}
	component.DownloadURL = strings.TrimSpace(component.URLs[0])
	if !validDottedVersion(component.Version) {
		return errors.New("smapi version must be an exact dotted version")
	}
	if !strings.HasPrefix(component.DownloadURL, "https://github.com/Pathoschild/SMAPI/releases/download/"+component.Version+"/") ||
		component.DownloadURL != "https://github.com/Pathoschild/SMAPI/releases/download/"+component.Version+"/SMAPI-"+component.Version+"-installer.zip" {
		return errors.New("smapi urls[0] must be the exact official GitHub release installer")
	}
	for _, raw := range component.URLs {
		if strings.TrimSpace(raw) != component.DownloadURL {
			return errors.New("smapi mirror urls require an explicitly reviewed manifest validator")
		}
	}
	if len(component.SHA256) != 64 || isHexDigest(strings.ToLower(component.SHA256)) == false {
		return errors.New("smapi sha256 must be a lowercase 64-character digest")
	}
	if component.SHA256 != strings.ToLower(component.SHA256) {
		return errors.New("smapi sha256 must be lowercase")
	}
	if component.ArchiveBytes <= 0 || component.MaxArchiveBytes < component.ArchiveBytes || component.MaxExtractBytes < component.MaxArchiveBytes {
		return errors.New("smapi archive size limits are invalid")
	}
	if len(component.TrustedHosts) == 0 || component.TrustedHosts[0] != "github.com" {
		return errors.New("smapi trustedHosts must include official GitHub first")
	}
	for _, host := range component.TrustedHosts {
		if host != "github.com" && host != "objects.githubusercontent.com" && host != "release-assets.githubusercontent.com" {
			return fmt.Errorf("untrusted smapi host %q", host)
		}
	}
	return nil
}

func validateControlManifestComponent(component ControlManifestComponent) error {
	if !validDottedVersion(component.Version) || !isHexDigest(strings.ToLower(component.DLLSHA256)) || component.DLLSHA256 != strings.ToLower(component.DLLSHA256) {
		return errors.New("control version or dllSha256 is invalid")
	}
	if component.CommandResultVersion <= 0 {
		return errors.New("control commandResultVersion must be positive")
	}
	return nil
}

func validDottedVersion(value string) bool {
	parts := strings.Split(strings.TrimSpace(value), ".")
	if len(parts) < 3 || len(parts) > 4 {
		return false
	}
	for _, part := range parts {
		if part == "" {
			return false
		}
		for _, r := range part {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}

func validateContentManifestComponent(kind, expectedAppID string, component RuntimeContentManifestComponent) error {
	if strings.TrimSpace(component.AppID) != expectedAppID {
		return fmt.Errorf("%s appId must be %s", kind, expectedAppID)
	}
	if !isDecimalID(component.BuildID) {
		return fmt.Errorf("%s buildId must be a positive decimal string", kind)
	}
	if strings.TrimSpace(component.ManifestVersion) == "" {
		return fmt.Errorf("%s manifestVersion is required", kind)
	}
	if len(component.Notes) == 0 || component.EstimatedDownloadBytes <= 0 {
		return fmt.Errorf("%s notes and estimatedDownloadBytes are required", kind)
	}
	return nil
}

func isDecimalID(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || value == "0" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func validateManifestComponent(kind string, component RuntimeStackManifestComponent) error {
	if len(component.Images) > 0 && component.Image != "" && strings.TrimSpace(component.Image) != strings.TrimSpace(component.Images[0]) {
		return fmt.Errorf("%s preferred image does not match images[0]", kind)
	}
	if component.TrustedCandidates != nil && !equalStrings(component.TrustedCandidates, component.Images) {
		return fmt.Errorf("%s trusted image aliases do not match images", kind)
	}
	component.normalize()
	tag := strings.TrimSpace(component.Tag)
	if err := validateRuntimeTag(tag); err != nil {
		return fmt.Errorf("%s tag: %w", kind, err)
	}
	if len(component.Images) == 0 || strings.TrimSpace(component.Image) == "" {
		return fmt.Errorf("%s images are required", kind)
	}
	canonicalDigest := ""
	seenImages := make(map[string]struct{}, len(component.Images))
	for _, ref := range component.Images {
		if _, exists := seenImages[ref]; exists {
			return fmt.Errorf("%s image %q is duplicated", kind, ref)
		}
		seenImages[ref] = struct{}{}
		repository, imageTag, err := parseRuntimeImageRef(ref)
		if err != nil {
			return fmt.Errorf("%s image %q: %w", kind, ref, err)
		}
		if _, ok := trustedRuntimeRepositories[kind][repository]; !ok {
			return fmt.Errorf("%s image %q uses an untrusted repository", kind, ref)
		}
		if imageTag != tag {
			return fmt.Errorf("%s image %q tag does not match %q", kind, ref, tag)
		}
		digest, ok := component.Digests[ref]
		if !ok || !validRuntimeDigest(digest) {
			return fmt.Errorf("%s image %q must have an exact sha256 digest", kind, ref)
		}
		digest = strings.ToLower(strings.TrimSpace(digest))
		if canonicalDigest == "" {
			canonicalDigest = digest
		} else if digest != canonicalDigest {
			return fmt.Errorf("%s image %q does not match the canonical digest", kind, ref)
		}
	}
	for ref := range component.Digests {
		if !containsString(component.Images, ref) {
			return fmt.Errorf("%s digest key %q is not listed in images", kind, ref)
		}
	}
	return nil
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func validRuntimeDigest(value string) bool {
	return strings.HasPrefix(value, "sha256:") && isHexDigest(strings.TrimPrefix(value, "sha256:"))
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func InspectRuntimeStack(dataDir string, installed bool) RuntimeStackInspection {
	manifest, err := BuiltInRuntimeStackManifest()
	result := RuntimeStackInspection{Recommended: recommendationFromManifest(manifest)}
	if err != nil {
		return invalidRuntimeStack(result, "invalid_config/manifest", "内置运行组件清单无效。")
	}
	if manifest.Status == RuntimeMatrixStatusWithdrawn {
		result.Status = RuntimeStackStatusWithdrawn
		result.Code = "matrix_withdrawn"
		result.Reason = "当前内置兼容矩阵已撤回；禁止新安装和升级。建议人工确认后回退到精确矩阵 " + manifest.Withdrawal.FallbackStackVersion + "。"
		return result
	}
	if !manifest.Installable() {
		result.Status = RuntimeStackStatusNotRecommended
		result.Code = "matrix_not_recommended"
		result.Reason = "当前内置兼容矩阵不是 recommended，不能用于正式安装或升级。"
		return result
	}
	if !installed {
		result.Supported = true
		result.Status = RuntimeStackStatusNotInstalled
		result.Code = "not_installed"
		result.Reason = "实例尚未安装 Junimo 运行组件。"
		return result
	}

	envPath := filepath.Join(dataDir, ".env")
	if _, err := os.Stat(envPath); err != nil {
		if os.IsNotExist(err) {
			return invalidRuntimeStack(result, "invalid_config/missing_env", "实例 .env 不存在，无法判断运行组件版本对。")
		}
		return invalidRuntimeStack(result, "invalid_config/read_env", "实例 .env 无法读取。")
	}
	values, err := ReadEnvFile(envPath)
	if err != nil {
		return invalidRuntimeStack(result, "invalid_config/read_env", "实例 .env 无法读取。")
	}
	required := []string{"IMAGE_VERSION", "SERVER_IMAGE", "SERVER_IMAGE_CANDIDATES", "STEAM_SERVICE_IMAGE", "STEAM_SERVICE_IMAGE_CANDIDATES"}
	for _, key := range required {
		if strings.TrimSpace(values[key]) == "" {
			return invalidRuntimeStack(result, "invalid_config/missing_field", "实例 .env 缺少运行组件版本字段。")
		}
	}

	server, code, reason := inspectConfiguredComponent("server", values["SERVER_IMAGE"], values["SERVER_IMAGE_CANDIDATES"])
	result.Current.Server = server
	auth, authCode, authReason := inspectConfiguredComponent("steamAuth", values["STEAM_SERVICE_IMAGE"], values["STEAM_SERVICE_IMAGE_CANDIDATES"])
	result.Current.SteamAuth = auth
	if code != "" {
		return classifiedRuntimeStack(result, code, reason)
	}
	if authCode != "" {
		return classifiedRuntimeStack(result, authCode, authReason)
	}
	if strings.TrimSpace(values["IMAGE_VERSION"]) != server.Tag {
		return invalidRuntimeStack(result, "invalid_config/server_version_mismatch", "IMAGE_VERSION 与 server 镜像 tag 不一致。")
	}

	result.Supported = true
	if server.Tag == manifest.Server.Tag && auth.Tag == manifest.SteamAuth.Tag {
		result.Status = RuntimeStackStatusUpToDate
		result.Code = "up_to_date"
		result.Reason = "当前 Junimo 运行组件版本对与推荐版本对完全匹配。"
		return result
	}
	result.Available = true
	result.Status = RuntimeStackStatusUpdateAvailable
	result.Code = "update_available"
	result.Reason = "当前 Junimo 运行组件版本对与推荐版本对不一致。"
	return result
}

func inspectConfiguredComponent(kind, image, candidates string) (RuntimeStackComponentState, string, string) {
	repository, tag, err := parseRuntimeImageRef(image)
	state := RuntimeStackComponentState{Image: strings.TrimSpace(image), Tag: tag}
	if err != nil {
		return state, "invalid_config/image_reference", "实例运行组件镜像引用无效。"
	}
	if _, ok := trustedRuntimeRepositories[kind][repository]; !ok {
		return state, "unsupported/custom_images", "实例使用了自定义镜像，阶段一仅展示且不判断能否覆盖。"
	}
	for _, ref := range strings.Split(candidates, ",") {
		repo, candidateTag, parseErr := parseRuntimeImageRef(ref)
		if parseErr != nil || candidateTag != tag {
			return state, "invalid_config/image_candidates", "实例运行组件候选镜像配置无效或 tag 不一致。"
		}
		if _, ok := trustedRuntimeRepositories[kind][repo]; !ok {
			return state, "unsupported/custom_images", "实例使用了自定义镜像，阶段一仅展示且不判断能否覆盖。"
		}
	}
	return state, "", ""
}

func classifiedRuntimeStack(result RuntimeStackInspection, code, reason string) RuntimeStackInspection {
	if code == "unsupported/custom_images" {
		result.Status = RuntimeStackStatusCustomImages
		result.Code = code
		result.Reason = reason
		return result
	}
	return invalidRuntimeStack(result, code, reason)
}

func invalidRuntimeStack(result RuntimeStackInspection, code, reason string) RuntimeStackInspection {
	result.Status = RuntimeStackStatusInvalidConfig
	result.Code = code
	result.Reason = reason
	return result
}

func recommendationFromManifest(manifest RuntimeStackManifest) RuntimeStackRecommendation {
	return RuntimeStackRecommendation{
		StackVersion: manifest.StackVersion, Channel: manifest.Channel, MinimumPanelVersion: manifest.MinimumPanelVersion,
		Server: manifest.Server, SteamAuth: manifest.SteamAuth, Game: manifest.Game, SDK: manifest.SDK, SMAPI: manifest.SMAPI, Control: manifest.Control,
		Status: manifest.Status, Withdrawal: manifest.Withdrawal, ReleaseNotes: manifest.ReleaseNotes, Tested: manifest.Tested,
	}
}

func parseRuntimeImageRef(ref string) (string, string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", "", errors.New("image reference is empty")
	}
	if strings.Contains(ref, "@") {
		return "", "", errors.New("digest image references are not allowed")
	}
	lastSlash := strings.LastIndex(ref, "/")
	lastColon := strings.LastIndex(ref, ":")
	if lastColon <= lastSlash || lastColon == len(ref)-1 {
		return "", "", errors.New("explicit image tag is required")
	}
	repository := strings.TrimSpace(ref[:lastColon])
	tag := strings.TrimSpace(ref[lastColon+1:])
	if repository == "" {
		return "", "", errors.New("repository is required")
	}
	if err := validateRuntimeTag(tag); err != nil {
		return "", "", err
	}
	return repository, tag, nil
}

func validateRuntimeTag(tag string) error {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return errors.New("tag is required")
	}
	if strings.EqualFold(tag, "latest") {
		return errors.New("latest is not allowed")
	}
	lower := strings.ToLower(tag)
	if strings.HasPrefix(lower, "sha256:") || strings.HasPrefix(lower, "sha256-") || isHexDigest(lower) {
		return errors.New("digest cannot be used as a tag")
	}
	return nil
}

func isHexDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func isHexDigest40(value string) bool {
	if len(value) != 40 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
