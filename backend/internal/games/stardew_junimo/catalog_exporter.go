package stardew_junimo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
)

const (
	catalogLockFile  = "catalog_export.lock"
	catalogErrorFile = "catalog_export_error.json"
)

// AcquireCatalogLock atomically creates the export lock file.
// Returns an error if the lock already exists (another export is running).
func AcquireCatalogLock(dataDir string) error {
	path := filepath.Join(controlDir(dataDir), catalogLockFile)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(f, "%s", time.Now().UTC().Format(time.RFC3339))
	return f.Close()
}

// ReleaseCatalogLock removes the export lock file.
func ReleaseCatalogLock(dataDir string) {
	_ = os.Remove(filepath.Join(controlDir(dataDir), catalogLockFile))
}

// CatalogExportRunning reports whether the export lock file exists.
func CatalogExportRunning(dataDir string) bool {
	_, err := os.Stat(filepath.Join(controlDir(dataDir), catalogLockFile))
	return err == nil
}

// WriteCatalogExportError records that catalog export failed with the given message.
func WriteCatalogExportError(dataDir, msg string) {
	safe := strings.ReplaceAll(msg, `"`, `\"`)
	safe = strings.ReplaceAll(safe, "\n", " ")
	_ = os.WriteFile(
		filepath.Join(controlDir(dataDir), catalogErrorFile),
		[]byte(`{"error":"`+safe+`"}`),
		0o644,
	)
}

// ClearCatalogExportError removes the error marker so a fresh export can be tracked.
func ClearCatalogExportError(dataDir string) {
	_ = os.Remove(filepath.Join(controlDir(dataDir), catalogErrorFile))
}

// GetInstanceImageTag reads IMAGE_VERSION from the instance .env file.
// Falls back to TestedImageTag when the file or key is missing.
func GetInstanceImageTag(dataDir string) string {
	envVals, _ := sjconfig.ReadEnvFile(filepath.Join(dataDir, ".env"))
	if tag := strings.TrimSpace(envVals["IMAGE_VERSION"]); tag != "" {
		return tag
	}
	return TestedImageTag
}

// ExportCatalogContent starts the Compose asset-exporter service and waits until the
// SMAPI mod writes options.json to the bind-mounted control directory.  It does not
// manage the lock file; callers are responsible.
//
// The asset-exporter service is declared in docker-compose.yml with profile
// "catalog-export".  It shares the steam-auth dependency and the same volumes as the
// server service, but maps no ports and mounts a scratch saves directory so it cannot
// create or modify the instance's real saves.
func ExportCatalogContent(ctx context.Context, dataDir, _ string, logLine func(string)) error {
	ctrlDir := controlDir(dataDir)
	optionsPath := filepath.Join(ctrlDir, "options.json")

	// Ensure the compose file has the asset-exporter service (handles existing instances
	// that were installed before this feature was added).
	if _, err := migrateAssetExporterService(filepath.Join(dataDir, "docker-compose.yml")); err != nil {
		logLine(fmt.Sprintf("[catalog-export] 警告：无法更新 docker-compose.yml：%v", err))
	}

	// Record previous mtime so we can detect a NEW write.
	var prevMtime time.Time
	if stat, err := os.Stat(optionsPath); err == nil {
		prevMtime = stat.ModTime()
	}

	// Ensure the scratch saves directory exists; the compose service references it.
	scratchSavesDir := filepath.Join(dataDir, ".local-container", "catalog-export-saves")
	if err := os.MkdirAll(scratchSavesDir, 0o755); err != nil {
		return fmt.Errorf("create catalog export scratch saves directory: %w", err)
	}
	defer os.RemoveAll(scratchSavesDir)

	logLine("[catalog-export] 通过 Compose asset-exporter 启动临时服务，复用 steam-auth 与 game-data；不映射端口，不加载存档。")

	exportCtx, exportCancel := context.WithTimeout(ctx, 10*time.Minute)
	defer exportCancel()

	// Stream asset-exporter output via goroutine; poll host filesystem for options.json.
	exitCh := make(chan error, 1)
	go func() {
		cmd := exec.CommandContext(exportCtx, "docker",
			"compose", "--profile", "catalog-export",
			"up", "asset-exporter",
		)
		cmd.Dir = dataDir
		output := &lineCallbackWriter{fn: logLine}
		cmd.Stdout = output
		cmd.Stderr = cmd.Stdout
		err := cmd.Run()
		output.Flush()
		exitCh <- err
	}()

	defer func() {
		stopExporter := exec.Command("docker", "compose",
			"--profile", "catalog-export", "stop", "asset-exporter")
		stopExporter.Dir = dataDir
		_ = stopExporter.Run()
		stopSteamAuth := exec.Command("docker", "compose", "stop", "steam-auth")
		stopSteamAuth.Dir = dataDir
		_ = stopSteamAuth.Run()
	}()

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case err := <-exitCh:
			// Container exited on its own. Check if options.json was written just before exit.
			if stat, statErr := os.Stat(optionsPath); statErr == nil && stat.ModTime().After(prevMtime) {
				logLine("[catalog-export] 导出容器已退出，options.json 已写入，导出成功。")
				return nil
			}
			if err != nil {
				return fmt.Errorf("export container exited with error: %w", err)
			}
			return errors.New("export container exited without writing options.json")

		case <-ticker.C:
			if stat, err := os.Stat(optionsPath); err == nil && stat.ModTime().After(prevMtime) {
				logLine("[catalog-export] options.json 已写入，正在停止导出容器...")
				stopCtx, stopCancel := context.WithTimeout(context.Background(), 30*time.Second)
				stopCmd := exec.CommandContext(stopCtx, "docker", "compose",
					"--profile", "catalog-export", "stop", "asset-exporter")
				stopCmd.Dir = dataDir
				_ = stopCmd.Run()
				stopCancel()
				// Wait for compose goroutine to drain.
				select {
				case <-exitCh:
				case <-time.After(15 * time.Second):
					killCmd := exec.Command("docker", "compose",
						"--profile", "catalog-export", "kill", "asset-exporter")
					killCmd.Dir = dataDir
					_ = killCmd.Run()
					<-exitCh
				}
				logLine("[catalog-export] 素材目录导出完成。")
				return nil
			}

		case <-exportCtx.Done():
			if ctx.Err() != nil {
				return fmt.Errorf("catalog export cancelled: %w", ctx.Err())
			}
			return errors.New("catalog export timed out after 10 minutes")
		}
	}
}

// lineCallbackWriter splits Write calls into lines and invokes fn for each.
type lineCallbackWriter struct {
	fn  func(string)
	buf []byte
}

// Flush reports a final partial line. Containers often write fatal startup
// errors without a trailing newline, which would otherwise be lost on exit.
func (w *lineCallbackWriter) Flush() {
	if len(w.buf) == 0 {
		return
	}
	line := strings.TrimSpace(string(w.buf))
	w.buf = nil
	if line != "" {
		w.fn(line)
	}
}

func (w *lineCallbackWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		line := strings.TrimRight(string(w.buf[:i]), "\r")
		if line != "" {
			w.fn(line)
		}
		w.buf = w.buf[i+1:]
	}
	return len(p), nil
}
