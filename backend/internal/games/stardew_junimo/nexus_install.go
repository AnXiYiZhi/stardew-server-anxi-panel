package stardew_junimo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
)

var ErrNexusNoDownloadableFiles = errors.New("Nexus Mod 没有可下载的主文件")

type NexusInstallLogFunc func(string)

type nexusV1FilesResponse struct {
	Files []nexusV1File `json:"files"`
}

type nexusV1File struct {
	FileID       int    `json:"file_id"`
	Name         string `json:"name"`
	Version      string `json:"version"`
	CategoryID   int    `json:"category_id"`
	CategoryName string `json:"category_name"`
	IsPrimary    bool   `json:"is_primary"`
	FileName     string `json:"file_name"`
}

type nexusV1DownloadLink struct {
	Name string `json:"name"`
	URI  string `json:"URI"`
	URI2 string `json:"uri"`
}

func InstallNexusMod(ctx context.Context, dataDir, apiKey string, result NexusModSearchResult, logf NexusInstallLogFunc) ([]registry.ModInfo, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, ErrNexusAPIKeyMissing
	}
	if result.ModID <= 0 {
		return nil, ErrInvalidNexusQuery
	}
	if result.NexusURL == "" {
		result.NexusURL = nexusModURL(result.ModID)
	}

	logNexusInstall(logf, "正在读取 Nexus 文件列表")
	file, err := nexusChooseDownloadFile(ctx, apiKey, result.ModID)
	if err != nil {
		return nil, err
	}
	logNexusInstall(logf, fmt.Sprintf("选择文件：%s", nexusFileDisplayName(file)))

	link, err := nexusGetDownloadLink(ctx, apiKey, result.ModID, file.FileID)
	if err != nil {
		return nil, err
	}

	tmp, err := os.CreateTemp("", "stardew-nexus-mod-*.zip")
	if err != nil {
		return nil, fmt.Errorf("创建临时下载文件失败: %w", err)
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	defer func() { _ = os.Remove(tmpPath) }()

	logNexusInstall(logf, "正在从 Nexus 下载 Mod 压缩包")
	if err := nexusDownloadArchive(ctx, link, tmpPath); err != nil {
		return nil, err
	}

	logNexusInstall(logf, "正在校验并安装 Mod")
	imported, err := uploadModZip(dataDir, tmpPath, uploadModZipOptions{inferNexusPackageOrigin: false})
	if err != nil {
		return nil, err
	}
	if err := SaveInstalledNexusMetadata(dataDir, imported, result); err != nil {
		return nil, err
	}
	logNexusInstall(logf, fmt.Sprintf("安装完成：%d 个 Mod 已导入", len(imported)))
	return ApplyNexusMetadataToMods(dataDir, imported), nil
}

func nexusChooseDownloadFile(ctx context.Context, apiKey string, modID int) (nexusV1File, error) {
	files, err := nexusListModFiles(ctx, apiKey, modID)
	if err != nil {
		return nexusV1File{}, err
	}
	if len(files) == 0 {
		return nexusV1File{}, ErrNexusNoDownloadableFiles
	}
	for _, file := range files {
		if file.IsPrimary {
			return file, nil
		}
	}
	for _, file := range files {
		if file.CategoryID == 1 || strings.EqualFold(file.CategoryName, "MAIN") || strings.EqualFold(file.CategoryName, "Main files") {
			return file, nil
		}
	}
	return files[0], nil
}

func nexusListModFiles(ctx context.Context, apiKey string, modID int) ([]nexusV1File, error) {
	url := fmt.Sprintf("%s/games/%s/mods/%d/files.json", nexusV1BaseURL, nexusGameDomain, modID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build nexus files request: %w", err)
	}
	setNexusHeaders(req, apiKey)

	body, err := doNexusRequest(req)
	if err != nil {
		return nil, err
	}
	var response nexusV1FilesResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("解析 Nexus 文件列表失败: %w", err)
	}
	return response.Files, nil
}

func nexusGetDownloadLink(ctx context.Context, apiKey string, modID, fileID int) (string, error) {
	url := fmt.Sprintf("%s/games/%s/mods/%d/files/%d/download_link.json", nexusV1BaseURL, nexusGameDomain, modID, fileID)
	return nexusGetDownloadLinkURL(ctx, apiKey, url)
}

func nexusGetDownloadLinkURL(ctx context.Context, apiKey string, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("build nexus download-link request: %w", err)
	}
	setNexusHeaders(req, apiKey)

	body, err := doNexusRequest(req)
	if err != nil {
		return "", err
	}
	var links []nexusV1DownloadLink
	if err := json.Unmarshal(body, &links); err != nil {
		return "", fmt.Errorf("解析 Nexus 下载链接失败: %w", err)
	}
	for _, link := range links {
		uri := strings.TrimSpace(link.URI)
		if uri == "" {
			uri = strings.TrimSpace(link.URI2)
		}
		if uri != "" {
			return uri, nil
		}
	}
	return "", ErrNexusNoDownloadableFiles
}

func nexusDownloadArchive(ctx context.Context, uri, destPath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return fmt.Errorf("build nexus archive request: %w", err)
	}
	req.Header.Set("User-Agent", nexusUserAgent)

	resp, err := nexusArchiveHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("remote archive download failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return &NexusAPIError{StatusCode: resp.StatusCode}
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("创建下载目录失败: %w", err)
	}
	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("创建下载文件失败: %w", err)
	}
	defer func() { _ = out.Close() }()

	limited := &io.LimitedReader{R: resp.Body, N: maxModZipBytes + 1}
	if _, err := io.Copy(out, limited); err != nil {
		return fmt.Errorf("写入 Nexus 下载文件失败: %w", err)
	}
	if limited.N <= 0 {
		return fmt.Errorf("Nexus 下载文件超过 %d MB 限制", maxModZipBytes/1024/1024)
	}
	return nil
}

func nexusFileDisplayName(file nexusV1File) string {
	if file.Name != "" && file.Version != "" {
		return fmt.Sprintf("%s v%s", file.Name, file.Version)
	}
	if file.Name != "" {
		return file.Name
	}
	if file.FileName != "" {
		return file.FileName
	}
	return fmt.Sprintf("file #%d", file.FileID)
}

func logNexusInstall(logf NexusInstallLogFunc, message string) {
	if logf != nil {
		logf(message)
	}
}
