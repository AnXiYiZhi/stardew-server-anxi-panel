package stardew_junimo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
)

const (
	junimoServerModFolder    = "JunimoServer"
	junimoServerManifestName = "manifest.json"
	junimoServerAssemblyName = "JunimoServer.dll"
	junimoModExtractMarker   = "anxi-junimo-mod-extract"
	runtimeOriginalJunimoDir = "original-junimo-server"
	runtimeTargetJunimoDir   = "target-junimo-server"
)

type junimoServerManifest struct {
	Name     string `json:"Name"`
	Version  string `json:"Version"`
	UniqueID string `json:"UniqueID"`
}

func junimoServerModDir(dataDir string) string {
	return filepath.Join(modsDir(dataDir), junimoServerModFolder)
}

func readJunimoServerModVersion(dir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, junimoServerManifestName))
	if err != nil {
		return "", err
	}
	var manifest junimoServerManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return "", fmt.Errorf("parse JunimoServer manifest: %w", err)
	}
	if !strings.EqualFold(strings.TrimSpace(manifest.UniqueID), "JunimoHost.Server") {
		return "", errors.New("JunimoServer manifest UniqueID mismatch")
	}
	version := strings.TrimSpace(manifest.Version)
	if version == "" {
		return "", errors.New("JunimoServer manifest version is empty")
	}
	return version, nil
}

func validateExtractedJunimoServerMod(dir, expectedVersion string) error {
	expectedVersion = strings.TrimSpace(expectedVersion)
	if expectedVersion == "" {
		return errors.New("expected JunimoServer version is empty")
	}
	version, err := readJunimoServerModVersion(dir)
	if err != nil {
		return err
	}
	if version != expectedVersion {
		return fmt.Errorf("JunimoServer version mismatch: got %s, want %s", version, expectedVersion)
	}
	assembly := filepath.Join(dir, junimoServerAssemblyName)
	info, err := os.Lstat(assembly)
	if err != nil {
		return fmt.Errorf("stat JunimoServer assembly: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return errors.New("JunimoServer assembly is not a non-empty regular file")
	}
	return filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("JunimoServer package contains symlink: %s", filepath.Base(path))
		}
		return nil
	})
}

func extractJunimoServerMod(ctx context.Context, docker DockerService, imageRef, workDir, expectedVersion string) (string, error) {
	if err := os.MkdirAll(workDir, 0o700); err != nil {
		return "", err
	}
	if err := os.Chmod(workDir, 0o700); err != nil {
		return "", err
	}
	targetDir := filepath.Join(workDir, runtimeTargetJunimoDir)
	if err := os.RemoveAll(targetDir); err != nil {
		return "", err
	}
	absWorkDir, err := filepath.Abs(workDir)
	if err != nil {
		return "", err
	}
	script := strings.Join([]string{
		"set -eu",
		"echo " + junimoModExtractMarker,
		"test -d /data/Mods/JunimoServer",
		"rm -rf /out/" + runtimeTargetJunimoDir,
		"cp -a /data/Mods/JunimoServer /out/" + runtimeTargetJunimoDir,
		"find /out/" + runtimeTargetJunimoDir + " -type d -exec chmod 755 {} \\;",
		"find /out/" + runtimeTargetJunimoDir + " -type f -exec chmod 644 {} \\;",
	}, "; ")
	extractCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	exitCode, err := docker.RunContainerTTY(extractCtx, paneldocker.ContainerTTYRunOpts{
		ImageRef: imageRef, Entrypoint: []string{"/bin/sh"}, Command: []string{"-lc", script},
		Binds: []string{absWorkDir + ":/out"}, User: "root",
	}, nil, func(string) {})
	if err != nil {
		return "", fmt.Errorf("extract JunimoServer from target image: %w", err)
	}
	if exitCode != 0 {
		return "", fmt.Errorf("extract JunimoServer from target image exited with code %d", exitCode)
	}
	if err := validateExtractedJunimoServerMod(targetDir, expectedVersion); err != nil {
		return "", err
	}
	return targetDir, nil
}

func replaceJunimoServerMod(dataDir, extractedDir, backupDir string) (bool, error) {
	targetDir := junimoServerModDir(dataDir)
	if err := os.MkdirAll(filepath.Dir(targetDir), 0o755); err != nil {
		return false, fmt.Errorf("create Mods directory: %w", err)
	}
	if _, err := os.Stat(backupDir); err == nil {
		return false, errors.New("JunimoServer backup directory already exists")
	} else if !os.IsNotExist(err) {
		return false, err
	}
	originalPresent := false
	if _, err := os.Stat(targetDir); err == nil {
		originalPresent = true
		if err := moveRuntimeModDirToBackup(targetDir, backupDir); err != nil {
			return false, fmt.Errorf("move current JunimoServer to recovery: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return false, err
	}
	if err := renameRuntimeModDir(extractedDir, targetDir, 15*time.Second); err != nil {
		if originalPresent {
			_ = renameRuntimeModDir(backupDir, targetDir, 15*time.Second)
		}
		return false, fmt.Errorf("activate target JunimoServer: %w", err)
	}
	return originalPresent, nil
}

func moveRuntimeModDirToBackup(source, target string) error {
	err := renameRuntimeModDir(source, target, 15*time.Second)
	if err == nil || runtime.GOOS != "windows" || !errors.Is(err, os.ErrPermission) {
		return err
	}
	// Docker Desktop can retain a Windows directory rename handle briefly even
	// after Compose removed the bind-mounted container. Preserve a complete
	// recovery copy before deleting the quiescent source directory.
	if copyErr := copyDir(source, target); copyErr != nil {
		_ = os.RemoveAll(target)
		return copyErr
	}
	if removeErr := os.RemoveAll(source); removeErr != nil {
		_ = os.RemoveAll(target)
		return removeErr
	}
	return nil
}

func renameRuntimeModDir(source, target string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		if err := os.Rename(source, target); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if timeout <= 0 || time.Now().After(deadline) {
			return lastErr
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func restoreRuntimeJunimoServerMod(dataDir, applyID string, originalPresent bool) error {
	recoveryDir := runtimeUpdateRecoveryDir(dataDir, applyID)
	targetDir := junimoServerModDir(dataDir)
	originalDir := filepath.Join(recoveryDir, runtimeOriginalJunimoDir)
	discardedDir := filepath.Join(recoveryDir, "failed-target-junimo-server")
	_ = os.RemoveAll(discardedDir)
	if _, err := os.Stat(targetDir); err == nil {
		if err := os.Rename(targetDir, discardedDir); err != nil {
			return fmt.Errorf("move failed target JunimoServer aside: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if !originalPresent {
		return nil
	}
	if err := os.Rename(originalDir, targetDir); err != nil {
		_ = os.Rename(discardedDir, targetDir)
		return fmt.Errorf("restore original JunimoServer: %w", err)
	}
	return nil
}
