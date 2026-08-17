package stardew_junimo

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
)

var ErrInvalidRemoteModURL = errors.New("远程 Mod 下载链接无效")
var ErrInvalidRemoteModExpectedVersion = errors.New("期望的 Mod 版本号无效")
var ErrInvalidRemoteModReplaceUniqueID = errors.New("要替换的 Mod UniqueID 无效")

const maxRemoteModManifestBytes = 1024 * 1024

type RemoteModVersionMismatchError struct {
	Expected string
	Actual   []string
}

func (e *RemoteModVersionMismatchError) Error() string {
	actual := "未知"
	if e != nil && len(e.Actual) > 0 {
		actual = strings.Join(e.Actual, "、")
	}
	expected := "未知"
	if e != nil && e.Expected != "" {
		expected = e.Expected
	}
	return fmt.Sprintf("下载包版本不匹配：期望 v%s，实际 manifest 版本为 %s；已取消安装，未覆盖现有 Mod", expected, actual)
}

type NexusDownloadTicket struct {
	ModID   int
	FileID  int
	Key     string
	Expires string
}

func InstallRemoteMod(ctx context.Context, dataDir, rawURL, apiKey string, result NexusModSearchResult, expectedVersion string, logf NexusInstallLogFunc) ([]registry.ModInfo, error) {
	return installRemoteMod(ctx, dataDir, rawURL, apiKey, result, expectedVersion, "", logf)
}

func UpdateRemoteMod(ctx context.Context, dataDir, rawURL, apiKey string, result NexusModSearchResult, expectedVersion, replaceUniqueID string, logf NexusInstallLogFunc) ([]registry.ModInfo, error) {
	replaceUniqueID, err := NormalizeRemoteModReplaceUniqueID(replaceUniqueID)
	if err != nil {
		return nil, err
	}
	expectedVersion, err = NormalizeRemoteModExpectedVersion(expectedVersion)
	if err != nil || expectedVersion == "" {
		return nil, ErrInvalidRemoteModExpectedVersion
	}
	return installRemoteMod(ctx, dataDir, rawURL, apiKey, result, expectedVersion, replaceUniqueID, logf)
}

func installRemoteMod(ctx context.Context, dataDir, rawURL, apiKey string, result NexusModSearchResult, expectedVersion, replaceUniqueID string, logf NexusInstallLogFunc) ([]registry.ModInfo, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return nil, ErrInvalidRemoteModURL
	}

	if strings.HasPrefix(strings.ToLower(trimmed), "nxm://") {
		ticket, err := ParseNexusNXMURL(trimmed)
		if err != nil {
			return nil, err
		}
		return installNexusModWithTicketExpected(ctx, dataDir, apiKey, result, ticket, expectedVersion, replaceUniqueID, logf)
	}

	return installModFromDirectURLExpected(ctx, dataDir, trimmed, result, expectedVersion, replaceUniqueID, logf)
}

func InstallNexusModWithTicket(ctx context.Context, dataDir, apiKey string, result NexusModSearchResult, ticket NexusDownloadTicket, logf NexusInstallLogFunc) ([]registry.ModInfo, error) {
	return installNexusModWithTicketExpected(ctx, dataDir, apiKey, result, ticket, "", "", logf)
}

func installNexusModWithTicketExpected(ctx context.Context, dataDir, apiKey string, result NexusModSearchResult, ticket NexusDownloadTicket, expectedVersion, replaceUniqueID string, logf NexusInstallLogFunc) ([]registry.ModInfo, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, ErrNexusAPIKeyMissing
	}
	if ticket.ModID <= 0 || ticket.FileID <= 0 || ticket.Key == "" || ticket.Expires == "" {
		return nil, ErrInvalidRemoteModURL
	}
	if result.ModID > 0 && result.ModID != ticket.ModID {
		return nil, ErrInvalidRemoteModURL
	}
	if result.ModID == 0 {
		result.ModID = ticket.ModID
	}
	if result.Name == "" {
		result.Name = fmt.Sprintf("Nexus Mod #%d", ticket.ModID)
	}
	if result.NexusURL == "" {
		result.NexusURL = nexusModURL(ticket.ModID)
	}

	logNexusInstall(logf, fmt.Sprintf("正在使用 NXM 授权安装 Nexus Mod #%d", ticket.ModID))
	link, err := nexusGetDownloadLinkWithTicket(ctx, apiKey, ticket)
	if err != nil {
		return nil, err
	}
	return installRemoteArchive(ctx, dataDir, link, result, expectedVersion, replaceUniqueID, logf)
}

func InstallModFromDirectURL(ctx context.Context, dataDir, rawURL string, result NexusModSearchResult, logf NexusInstallLogFunc) ([]registry.ModInfo, error) {
	return installModFromDirectURLExpected(ctx, dataDir, rawURL, result, "", "", logf)
}

func installModFromDirectURLExpected(ctx context.Context, dataDir, rawURL string, result NexusModSearchResult, expectedVersion, replaceUniqueID string, logf NexusInstallLogFunc) ([]registry.ModInfo, error) {
	if err := validateRemoteArchiveURL(rawURL); err != nil {
		return nil, err
	}
	logNexusInstall(logf, "正在从远程链接下载 Mod 压缩包")
	return installRemoteArchive(ctx, dataDir, rawURL, result, expectedVersion, replaceUniqueID, logf)
}

func ParseNexusNXMURL(raw string) (NexusDownloadTicket, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return NexusDownloadTicket{}, ErrInvalidRemoteModURL
	}
	if !strings.EqualFold(u.Scheme, "nxm") || !strings.EqualFold(u.Host, nexusGameDomain) {
		return NexusDownloadTicket{}, ErrInvalidRemoteModURL
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 4 || !strings.EqualFold(parts[0], "mods") || !strings.EqualFold(parts[2], "files") {
		return NexusDownloadTicket{}, ErrInvalidRemoteModURL
	}
	modID, ok := parsePositiveInt(parts[1])
	if !ok {
		return NexusDownloadTicket{}, ErrInvalidRemoteModURL
	}
	fileID, ok := parsePositiveInt(parts[3])
	if !ok {
		return NexusDownloadTicket{}, ErrInvalidRemoteModURL
	}

	q := u.Query()
	key := strings.TrimSpace(q.Get("key"))
	expires := strings.TrimSpace(q.Get("expires"))
	if key == "" || expires == "" {
		return NexusDownloadTicket{}, ErrInvalidRemoteModURL
	}
	if _, err := strconv.ParseInt(expires, 10, 64); err != nil {
		return NexusDownloadTicket{}, ErrInvalidRemoteModURL
	}

	return NexusDownloadTicket{ModID: modID, FileID: fileID, Key: key, Expires: expires}, nil
}

func nexusGetDownloadLinkWithTicket(ctx context.Context, apiKey string, ticket NexusDownloadTicket) (string, error) {
	endpoint := fmt.Sprintf("%s/games/%s/mods/%d/files/%d/download_link.json", nexusV1BaseURL, nexusGameDomain, ticket.ModID, ticket.FileID)
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("build nexus download-link request: %w", err)
	}
	q := u.Query()
	q.Set("key", ticket.Key)
	q.Set("expires", ticket.Expires)
	u.RawQuery = q.Encode()
	return nexusGetDownloadLinkURL(ctx, apiKey, u.String())
}

func installRemoteArchive(ctx context.Context, dataDir, archiveURL string, result NexusModSearchResult, expectedVersion, replaceUniqueID string, logf NexusInstallLogFunc) ([]registry.ModInfo, error) {
	tmp, err := os.CreateTemp("", "stardew-remote-mod-*.zip")
	if err != nil {
		return nil, fmt.Errorf("创建临时下载文件失败: %w", err)
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	defer func() { _ = os.Remove(tmpPath) }()

	if err := nexusDownloadArchive(ctx, archiveURL, tmpPath, logf); err != nil {
		return nil, err
	}
	expectedVersion, err = NormalizeRemoteModExpectedVersion(expectedVersion)
	if err != nil {
		return nil, err
	}
	if expectedVersion != "" {
		logNexusInstall(logf, fmt.Sprintf("正在核对下载包版本，期望 v%s", expectedVersion))
		if err := verifyRemoteModArchiveVersion(tmpPath, expectedVersion); err != nil {
			return nil, err
		}
	}

	updateMode := replaceUniqueID != ""
	oldFolderName := ""
	if updateMode {
		oldFolderName, _ = FindModByUniqueID(dataDir, replaceUniqueID)
		logNexusInstall(logf, fmt.Sprintf("正在校验并安全替换 Mod %s", replaceUniqueID))
	} else {
		logNexusInstall(logf, "正在校验并安装 Mod")
	}
	imported, err := uploadModZip(dataDir, tmpPath, uploadModZipOptions{
		inferNexusPackageOrigin: false,
		allowAlreadyInstalled:   !updateMode,
		replaceUniqueID:         replaceUniqueID,
		expectedVersion:         expectedVersion,
	})
	if err != nil {
		return nil, err
	}
	if len(imported) == 0 {
		logNexusInstall(logf, "该 Mod 已安装，跳过重复导入")
		return nil, nil
	}
	if result.ModID > 0 {
		if err := SaveInstalledNexusMetadata(dataDir, imported, result); err != nil {
			if !updateMode {
				return nil, err
			}
			logNexusInstall(logf, fmt.Sprintf("新版文件已换入，但 Nexus 展示信息保存失败：%v", err))
		} else if updateMode && len(imported) == 1 && oldFolderName != "" && imported[0].FolderName != oldFolderName {
			// Metadata is keyed by physical folder. The old entry is only removed
			// after the new entry has been durably written.
			_ = DeleteInstalledNexusMetadata(dataDir, []string{oldFolderName})
		}
	}
	if updateMode {
		logNexusInstall(logf, fmt.Sprintf("更新完成，%d 个 Mod 已安全替换", len(imported)))
	} else {
		logNexusInstall(logf, fmt.Sprintf("安装完成，%d 个 Mod 已导入", len(imported)))
	}
	if result.ModID > 0 {
		return ApplyNexusMetadataToMods(dataDir, imported), nil
	}
	return imported, nil
}

func NormalizeRemoteModReplaceUniqueID(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" || len(value) > 256 {
		return "", ErrInvalidRemoteModReplaceUniqueID
	}
	for _, r := range value {
		if r < 0x21 || r == 0x7f {
			return "", ErrInvalidRemoteModReplaceUniqueID
		}
	}
	return value, nil
}

func NormalizeRemoteModExpectedVersion(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if len(value) > 0 && (value[0] == 'v' || value[0] == 'V') {
		value = value[1:]
	}
	if value == "" {
		return "", nil
	}
	if len(value) > 64 {
		return "", ErrInvalidRemoteModExpectedVersion
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || strings.ContainsRune("._+-", r) {
			continue
		}
		return "", ErrInvalidRemoteModExpectedVersion
	}
	return value, nil
}

func verifyRemoteModArchiveVersion(zipPath, expectedVersion string) error {
	expected, err := NormalizeRemoteModExpectedVersion(expectedVersion)
	if err != nil || expected == "" {
		return err
	}
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("打开 ZIP 失败: %w", err)
	}
	defer func() { _ = zr.Close() }()

	actualSet := map[string]struct{}{}
	for _, file := range zr.File {
		if file.FileInfo().IsDir() || !strings.EqualFold(path.Base(strings.ReplaceAll(file.Name, "\\", "/")), "manifest.json") {
			continue
		}
		if file.UncompressedSize64 > maxRemoteModManifestBytes {
			return fmt.Errorf("manifest.json 超过 %d KB 限制", maxRemoteModManifestBytes/1024)
		}
		reader, openErr := file.Open()
		if openErr != nil {
			return fmt.Errorf("读取 manifest.json 失败: %w", openErr)
		}
		data, readErr := io.ReadAll(io.LimitReader(reader, maxRemoteModManifestBytes+1))
		closeErr := reader.Close()
		if readErr != nil {
			return fmt.Errorf("读取 manifest.json 失败: %w", readErr)
		}
		if len(data) > maxRemoteModManifestBytes {
			return fmt.Errorf("manifest.json 超过 %d KB 限制", maxRemoteModManifestBytes/1024)
		}
		if closeErr != nil {
			return fmt.Errorf("关闭 manifest.json 失败: %w", closeErr)
		}
		var manifest modManifest
		if decodeModManifest(data, &manifest) != nil {
			continue
		}
		actual := strings.TrimSpace(manifest.Version)
		if actual == "" {
			continue
		}
		if strings.EqualFold(normalizeRuntimeModVersion(actual), normalizeRuntimeModVersion(expected)) {
			return nil
		}
		if safe, safeErr := NormalizeRemoteModExpectedVersion(actual); safeErr == nil && safe != "" {
			actualSet[safe] = struct{}{}
		}
	}

	actual := make([]string, 0, len(actualSet))
	for version := range actualSet {
		actual = append(actual, version)
	}
	sort.Strings(actual)
	return &RemoteModVersionMismatchError{Expected: expected, Actual: actual}
}

func validateRemoteArchiveURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || !strings.EqualFold(u.Scheme, "https") {
		return ErrInvalidRemoteModURL
	}
	host := u.Hostname()
	if host == "" || strings.EqualFold(host, "localhost") {
		return ErrInvalidRemoteModURL
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return ErrInvalidRemoteModURL
		}
	}
	ext := strings.ToLower(path.Ext(u.Path))
	if ext != ".zip" {
		return ErrInvalidRemoteModURL
	}
	return nil
}
