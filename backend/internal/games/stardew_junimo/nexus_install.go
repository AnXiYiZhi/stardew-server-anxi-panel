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
	"strconv"
	"strings"
	"sync/atomic"
	"time"

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
	if err := nexusDownloadArchive(ctx, link, tmpPath, logf); err != nil {
		return nil, err
	}

	logNexusInstall(logf, "正在校验并安装 Mod")
	imported, err := uploadModZip(dataDir, tmpPath, uploadModZipOptions{inferNexusPackageOrigin: false, allowAlreadyInstalled: true})
	if err != nil {
		return nil, err
	}
	if len(imported) == 0 {
		logNexusInstall(logf, "该 Nexus Mod 已安装，跳过重复导入")
		return nil, nil
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

func nexusDownloadArchive(ctx context.Context, uri, destPath string, logf NexusInstallLogFunc) error {
	return nexusDownloadArchiveResumable(ctx, uri, destPath, logf)
}

func nexusDownloadArchiveResumable(ctx context.Context, uri, destPath string, logf NexusInstallLogFunc) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("创建下载目录失败: %w", err)
	}
	partPath := destPath + ".part"
	_ = os.Remove(destPath)
	_ = os.Remove(partPath)

	downloadCtx, cancel := context.WithTimeout(ctx, nexusArchiveTimeout)
	defer cancel()

	var progress *archiveDownloadProgress
	var lastErr error
	for attempt := 1; attempt <= archiveDownloadMaxAttempts; attempt++ {
		offset := archivePartSize(partPath)
		if offset > maxModZipBytes {
			_ = os.Remove(partPath)
			return fmt.Errorf("Nexus 下载文件超过 %d MB 限制", maxModZipBytes/1024/1024)
		}
		if offset > 0 {
			logNexusInstall(logf, fmt.Sprintf("下载中断，已保留 %s，正在尝试断点续传 %d/%d", formatDownloadBytes(offset), attempt, archiveDownloadMaxAttempts))
		} else if attempt > 1 {
			logNexusInstall(logf, fmt.Sprintf("正在重新尝试下载 %d/%d", attempt, archiveDownloadMaxAttempts))
		}

		total, err := nexusDownloadArchiveAttempt(downloadCtx, uri, partPath, offset, &progress, logf)
		if err == nil {
			if total > maxModZipBytes {
				_ = os.Remove(partPath)
				return fmt.Errorf("Nexus 下载文件超过 %d MB 限制", maxModZipBytes/1024/1024)
			}
			if err := os.Rename(partPath, destPath); err != nil {
				_ = os.Remove(partPath)
				return fmt.Errorf("保存 Nexus 下载文件失败: %w", err)
			}
			if progress != nil {
				progress.finish()
			}
			return nil
		}
		lastErr = err
		if downloadCtx.Err() != nil {
			break
		}
		if !archiveDownloadShouldRetry(err) || attempt == archiveDownloadMaxAttempts {
			break
		}
	}
	_ = os.Remove(partPath)
	if lastErr != nil {
		return lastErr
	}
	return downloadCtx.Err()
}

func nexusDownloadArchiveAttempt(ctx context.Context, uri, partPath string, offset int64, progress **archiveDownloadProgress, logf NexusInstallLogFunc) (int64, error) {
	attemptCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	req, err := http.NewRequestWithContext(attemptCtx, http.MethodGet, uri, nil)
	if err != nil {
		return 0, fmt.Errorf("build nexus archive request: %w", err)
	}
	req.Header.Set("User-Agent", nexusUserAgent)
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}

	logNexusInstall(logf, "正在连接远程下载服务器")
	resp, err := nexusArchiveHTTPClient.Do(req)
	if err != nil {
		return offset, fmt.Errorf("remote archive download failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	logNexusInstall(logf, fmt.Sprintf("远程下载服务器已响应：HTTP %d", resp.StatusCode))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return offset, &NexusAPIError{StatusCode: resp.StatusCode}
	}
	contentType := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
	if contentType != "" {
		logNexusInstall(logf, fmt.Sprintf("远程响应类型：%s", contentType))
	}
	if strings.Contains(contentType, "text/html") {
		_, _ = io.Copy(io.Discard, resp.Body)
		return offset, fmt.Errorf("远程下载返回的是网页，不是 ZIP 压缩包；请确认浏览器扩展已经拿到 Nexus CDN ZIP 下载链接")
	}

	start := offset
	total := resp.ContentLength
	if offset > 0 {
		if resp.StatusCode == http.StatusPartialContent {
			rangeStart, rangeTotal, ok := parseContentRange(resp.Header.Get("Content-Range"))
			if !ok || rangeStart != offset {
				return offset, fmt.Errorf("Nexus CDN 返回的断点续传范围不匹配")
			}
			total = rangeTotal
		} else {
			logNexusInstall(logf, "远程服务器未接受断点续传，将从头重新下载")
			offset = 0
			start = 0
			if err := os.Remove(partPath); err != nil && !os.IsNotExist(err) {
				return 0, fmt.Errorf("清理临时下载文件失败: %w", err)
			}
			total = resp.ContentLength
		}
	}
	if total > maxModZipBytes {
		return offset, fmt.Errorf("Nexus 下载文件超过 %d MB 限制", maxModZipBytes/1024/1024)
	}

	flags := os.O_CREATE | os.O_WRONLY
	if offset > 0 {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}
	out, err := os.OpenFile(partPath, flags, 0o600)
	if err != nil {
		return offset, fmt.Errorf("创建下载文件失败: %w", err)
	}
	defer func() { _ = out.Close() }()

	if *progress == nil || start == 0 {
		*progress = newArchiveDownloadProgress(total, logf)
		(*progress).downloaded = start
	} else {
		(*progress).setTotal(total)
	}
	written, err := copyArchiveWithStallTimeout(attemptCtx, cancel, resp.Body, out, *progress, maxModZipBytes-offset+1)
	if err != nil {
		return offset + written, fmt.Errorf("写入 Nexus 下载文件失败: %w", err)
	}
	if offset+written > maxModZipBytes {
		return offset + written, fmt.Errorf("Nexus 下载文件超过 %d MB 限制", maxModZipBytes/1024/1024)
	}
	return offset + written, nil
}

const (
	archiveProgressLogEveryBytes = int64(5 * 1024 * 1024)
	archiveProgressLogEveryTime  = 2 * time.Second
	archiveDownloadMaxAttempts   = 4
)

var nexusArchiveStallTimeout = 120 * time.Second

type archiveDownloadProgress struct {
	total           int64
	downloaded      int64
	lastLoggedBytes int64
	lastLoggedAt    time.Time
	logf            NexusInstallLogFunc
}

func newArchiveDownloadProgress(total int64, logf NexusInstallLogFunc) *archiveDownloadProgress {
	p := &archiveDownloadProgress{total: total, logf: logf}
	if logf != nil {
		if total > 0 {
			logf(fmt.Sprintf("远程压缩包大小：%s", formatDownloadBytes(total)))
		} else {
			logf("远程压缩包大小未知，开始下载")
		}
	}
	return p
}

func (p *archiveDownloadProgress) Write(b []byte) (int, error) {
	n := len(b)
	if n == 0 {
		return 0, nil
	}
	p.downloaded += int64(n)
	p.maybeLog(false)
	return n, nil
}

func (p *archiveDownloadProgress) setTotal(total int64) {
	if total > 0 && p.total != total {
		p.total = total
	}
}

func (p *archiveDownloadProgress) finish() {
	p.maybeLog(true)
}

func (p *archiveDownloadProgress) maybeLog(force bool) {
	if p.logf == nil || p.downloaded == 0 {
		return
	}
	now := time.Now()
	if !force && p.lastLoggedBytes > 0 && p.downloaded-p.lastLoggedBytes < archiveProgressLogEveryBytes && now.Sub(p.lastLoggedAt) < archiveProgressLogEveryTime {
		return
	}
	p.lastLoggedBytes = p.downloaded
	p.lastLoggedAt = now

	if p.total > 0 {
		remaining := p.total - p.downloaded
		if remaining < 0 {
			remaining = 0
		}
		percent := float64(p.downloaded) * 100 / float64(p.total)
		if percent > 100 {
			percent = 100
		}
		p.logf(fmt.Sprintf("下载进度：已下载 %s / %s，剩余 %s（%.1f%%）", formatDownloadBytes(p.downloaded), formatDownloadBytes(p.total), formatDownloadBytes(remaining), percent))
		return
	}
	p.logf(fmt.Sprintf("下载进度：已下载 %s（总大小未知）", formatDownloadBytes(p.downloaded)))
}

func archivePartSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func archiveDownloadShouldRetry(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *NexusAPIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode >= 500
	}
	return true
}

func parseContentRange(header string) (int64, int64, bool) {
	header = strings.TrimSpace(header)
	if !strings.HasPrefix(strings.ToLower(header), "bytes ") {
		return 0, 0, false
	}
	value := strings.TrimSpace(header[len("bytes "):])
	rangePart, totalPart, ok := strings.Cut(value, "/")
	if !ok {
		return 0, 0, false
	}
	startPart, _, ok := strings.Cut(rangePart, "-")
	if !ok {
		return 0, 0, false
	}
	start, err := strconv.ParseInt(strings.TrimSpace(startPart), 10, 64)
	if err != nil || start < 0 {
		return 0, 0, false
	}
	totalPart = strings.TrimSpace(totalPart)
	if totalPart == "*" {
		return start, -1, true
	}
	total, err := strconv.ParseInt(totalPart, 10, 64)
	if err != nil || total < 0 {
		return 0, 0, false
	}
	return start, total, true
}

func copyArchiveWithStallTimeout(ctx context.Context, cancel context.CancelFunc, src io.Reader, dst io.Writer, progress io.Writer, limit int64) (int64, error) {
	var stalled atomic.Bool
	timer := time.AfterFunc(nexusArchiveStallTimeout, func() {
		stalled.Store(true)
		cancel()
	})
	defer timer.Stop()

	buf := make([]byte, 64*1024)
	var written int64
	for {
		n, readErr := src.Read(buf)
		if n > 0 {
			timer.Reset(nexusArchiveStallTimeout)
			written += int64(n)
			if written > limit {
				return written, nil
			}
			if _, err := dst.Write(buf[:n]); err != nil {
				return written, err
			}
			if progress != nil {
				if _, err := progress.Write(buf[:n]); err != nil {
					return written, err
				}
			}
		}
		if readErr == io.EOF {
			return written, nil
		}
		if readErr != nil {
			if stalled.Load() {
				return written, fmt.Errorf("120 秒内没有收到新的下载数据")
			}
			if ctx.Err() != nil {
				return written, ctx.Err()
			}
			return written, readErr
		}
	}
}

func formatDownloadBytes(size int64) string {
	const (
		kib = 1024
		mib = 1024 * kib
		gib = 1024 * mib
	)
	switch {
	case size >= gib:
		return fmt.Sprintf("%.1f GB", float64(size)/gib)
	case size >= mib:
		return fmt.Sprintf("%.1f MB", float64(size)/mib)
	case size >= kib:
		return fmt.Sprintf("%.1f KB", float64(size)/kib)
	default:
		return fmt.Sprintf("%d B", size)
	}
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
