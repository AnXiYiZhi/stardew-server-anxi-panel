package stardew_junimo

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
)

var ErrInvalidRemoteModURL = errors.New("远程 Mod 下载链接无效")

type NexusDownloadTicket struct {
	ModID   int
	FileID  int
	Key     string
	Expires string
}

func InstallRemoteMod(ctx context.Context, dataDir, rawURL, apiKey string, result NexusModSearchResult, logf NexusInstallLogFunc) ([]registry.ModInfo, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return nil, ErrInvalidRemoteModURL
	}

	if strings.HasPrefix(strings.ToLower(trimmed), "nxm://") {
		ticket, err := ParseNexusNXMURL(trimmed)
		if err != nil {
			return nil, err
		}
		return InstallNexusModWithTicket(ctx, dataDir, apiKey, result, ticket, logf)
	}

	return InstallModFromDirectURL(ctx, dataDir, trimmed, result, logf)
}

func InstallNexusModWithTicket(ctx context.Context, dataDir, apiKey string, result NexusModSearchResult, ticket NexusDownloadTicket, logf NexusInstallLogFunc) ([]registry.ModInfo, error) {
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
	return installRemoteArchive(ctx, dataDir, link, result, logf)
}

func InstallModFromDirectURL(ctx context.Context, dataDir, rawURL string, result NexusModSearchResult, logf NexusInstallLogFunc) ([]registry.ModInfo, error) {
	if err := validateRemoteArchiveURL(rawURL); err != nil {
		return nil, err
	}
	logNexusInstall(logf, "正在从远程链接下载 Mod 压缩包")
	return installRemoteArchive(ctx, dataDir, rawURL, result, logf)
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

func installRemoteArchive(ctx context.Context, dataDir, archiveURL string, result NexusModSearchResult, logf NexusInstallLogFunc) ([]registry.ModInfo, error) {
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

	logNexusInstall(logf, "正在校验并安装 Mod")
	imported, err := uploadModZip(dataDir, tmpPath, uploadModZipOptions{inferNexusPackageOrigin: false, allowAlreadyInstalled: true})
	if err != nil {
		return nil, err
	}
	if len(imported) == 0 {
		logNexusInstall(logf, "该 Mod 已安装，跳过重复导入")
		return nil, nil
	}
	if result.ModID > 0 {
		if err := SaveInstalledNexusMetadata(dataDir, imported, result); err != nil {
			return nil, err
		}
	}
	logNexusInstall(logf, fmt.Sprintf("安装完成，%d 个 Mod 已导入", len(imported)))
	if result.ModID > 0 {
		return ApplyNexusMetadataToMods(dataDir, imported), nil
	}
	return imported, nil
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
