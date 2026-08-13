package stardew_junimo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
)

const (
	smapiBundledSyncMarker     = "anxi-smapi-bundled-sync"
	smapiBundledSourceMissing  = 42
	maxSMAPIBundledRuntimeSize = int64(512 * 1024 * 1024)
)

// EnsureManagedSMAPIBundledMods materializes the exact Mods directory bundled
// with the installed SMAPI runtime into the host-managed smapi namespace. The
// Junimo image performs the same copy during container startup; doing it here
// makes pre-start fingerprints deterministic on the very first launch.
func (d *Driver) EnsureManagedSMAPIBundledMods(ctx context.Context, dataDir, imageRef string, lineHandler func(string)) (bool, error) {
	if d == nil || d.docker == nil {
		return false, errors.New("docker service is not configured")
	}
	if lineHandler == nil {
		lineHandler = func(string) {}
	}
	if !filepath.IsAbs(dataDir) {
		return false, errors.New("instance data directory must be absolute")
	}
	root := modsDir(dataDir)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return false, fmt.Errorf("create managed mods root: %w", err)
	}

	lock := modProfileLockFor(dataDir)
	lock.Lock()
	defer lock.Unlock()

	destination := filepath.Join(root, "smapi")
	if err := recoverManagedSMAPIBundledPublication(root, destination); err != nil {
		return false, err
	}

	stage, err := os.MkdirTemp(root, ".smapi-sync-")
	if err != nil {
		return false, fmt.Errorf("create SMAPI bundled staging directory: %w", err)
	}
	defer os.RemoveAll(stage)
	hostStage, err := d.dockerHostPath(stage)
	if err != nil {
		return false, fmt.Errorf("map SMAPI bundled staging directory for Docker: %w", err)
	}

	script := `set -eu
echo "` + smapiBundledSyncMarker + `: copying installed SMAPI support mods"
if [ ! -d /data/game/Mods ]; then
  echo "installed SMAPI Mods directory is missing" >&2
  exit 42
fi
cp -R /data/game/Mods/. /managed/
echo "` + smapiBundledSyncMarker + `: copy complete"`
	exitCode, err := d.docker.RunContainerTTY(ctx, paneldocker.ContainerTTYRunOpts{
		ImageRef:   imageRef,
		Entrypoint: []string{"/bin/sh"},
		User:       "root",
		Command:    []string{"-c", script},
		Binds: []string{
			resolvedGameDataVolumeName(dataDir) + ":/data/game:ro",
			hostStage + ":/managed",
		},
	}, nil, lineHandler)
	if err != nil {
		return false, fmt.Errorf("copy installed SMAPI support mods: %w", err)
	}
	if exitCode == smapiBundledSourceMissing {
		return false, errors.New("installed SMAPI Mods directory is missing")
	}
	if exitCode != 0 {
		return false, fmt.Errorf("SMAPI bundled mod sync container exited with code %d", exitCode)
	}

	if err := validateManagedSMAPIBundledStage(stage); err != nil {
		return false, err
	}
	stageDigest, err := managedSMAPIBundledTreeDigest(stage)
	if err != nil {
		return false, fmt.Errorf("fingerprint staged SMAPI support mods: %w", err)
	}
	if currentDigest, digestErr := managedSMAPIBundledTreeDigest(destination); digestErr == nil && currentDigest == stageDigest {
		if err := cleanupManagedSMAPIBundledArtifacts(root); err != nil && d.logger != nil {
			d.logger.Warn("failed to clean recovered SMAPI bundled artifacts", "error", err)
		}
		return false, nil
	}

	backup, err := reserveSiblingPath(root, ".smapi-backup-")
	if err != nil {
		return false, fmt.Errorf("reserve SMAPI bundled backup path: %w", err)
	}
	destinationExisted := false
	if _, statErr := os.Lstat(destination); statErr == nil {
		destinationExisted = true
		if err := os.Rename(destination, backup); err != nil {
			return false, fmt.Errorf("stage current SMAPI support mods for replacement: %w", err)
		}
	} else if !os.IsNotExist(statErr) {
		return false, fmt.Errorf("inspect current SMAPI support mods: %w", statErr)
	}
	if err := os.Rename(stage, destination); err != nil {
		if destinationExisted {
			if restoreErr := os.Rename(backup, destination); restoreErr != nil {
				return false, fmt.Errorf("publish SMAPI support mods: %w; restore previous tree: %v (backup retained at %s)", err, restoreErr, backup)
			}
		}
		return false, fmt.Errorf("publish SMAPI support mods: %w", err)
	}
	if destinationExisted {
		if err := os.RemoveAll(backup); err != nil && d.logger != nil {
			d.logger.Warn("failed to remove replaced SMAPI bundled backup", "path", backup, "error", err)
		}
	}
	if err := cleanupManagedSMAPIBundledArtifacts(root); err != nil && d.logger != nil {
		d.logger.Warn("failed to clean recovered SMAPI bundled artifacts", "error", err)
	}
	return true, nil
}

// recoverManagedSMAPIBundledPublication repairs the only two durable crash
// windows in the publish sequence: an abandoned staging directory, or a
// previous tree renamed to its sibling backup before the new tree was
// published. Only Panel-owned exact prefixes inside the managed mods root are
// considered; user Mod directories are never scanned or removed.
func recoverManagedSMAPIBundledPublication(root, destination string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return fmt.Errorf("read managed mods root for SMAPI recovery: %w", err)
	}
	backups := make([]string, 0)
	for _, entry := range entries {
		name := entry.Name()
		path := filepath.Join(root, name)
		switch {
		case strings.HasPrefix(name, ".smapi-sync-"):
			if err := os.RemoveAll(path); err != nil {
				return fmt.Errorf("remove abandoned SMAPI staging directory %s: %w", name, err)
			}
		case strings.HasPrefix(name, ".smapi-backup-"):
			backups = append(backups, path)
		}
	}

	if _, err := os.Lstat(destination); err == nil {
		if validateManagedSMAPIBundledStage(destination) == nil {
			if _, digestErr := managedSMAPIBundledTreeDigest(destination); digestErr == nil {
				return cleanupManagedSMAPIBundledArtifacts(root)
			}
		}
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect managed SMAPI destination during recovery: %w", err)
	}

	sort.Slice(backups, func(i, j int) bool {
		left, leftErr := os.Stat(backups[i])
		right, rightErr := os.Stat(backups[j])
		if leftErr != nil || rightErr != nil {
			return backups[i] > backups[j]
		}
		return left.ModTime().After(right.ModTime())
	})
	for _, backup := range backups {
		if validateManagedSMAPIBundledStage(backup) != nil {
			continue
		}
		if _, err := managedSMAPIBundledTreeDigest(backup); err != nil {
			continue
		}
		if err := os.Rename(backup, destination); err != nil {
			return fmt.Errorf("restore interrupted SMAPI bundled publication: %w", err)
		}
		return cleanupManagedSMAPIBundledArtifacts(root)
	}
	return nil
}

func cleanupManagedSMAPIBundledArtifacts(root string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, ".smapi-sync-") && !strings.HasPrefix(name, ".smapi-backup-") {
			continue
		}
		if err := os.RemoveAll(filepath.Join(root, name)); err != nil {
			return err
		}
	}
	return nil
}

func validateManagedSMAPIBundledStage(root string) error {
	mods, err := managedSMAPIRuntimeLoadedMods(root)
	if err != nil {
		return fmt.Errorf("validate staged SMAPI support mods: %w", err)
	}
	if len(mods) == 0 {
		return errors.New("installed SMAPI Mods directory contains no valid manifests")
	}
	seen := make(map[string]struct{}, len(mods))
	for _, mod := range mods {
		key := strings.ToLower(strings.TrimSpace(mod.UniqueID))
		if key == "" || strings.TrimSpace(mod.Version) == "" {
			return errors.New("installed SMAPI support mod manifest is missing UniqueID or Version")
		}
		if _, exists := seen[key]; exists {
			return fmt.Errorf("installed SMAPI support mods contain duplicate UniqueID %s", mod.UniqueID)
		}
		seen[key] = struct{}{}
	}
	for required := range smapiBundledSupportUniqueIDs {
		if _, ok := seen[required]; !ok {
			return fmt.Errorf("installed SMAPI support mod %s is missing", required)
		}
	}
	return nil
}

func managedSMAPIRuntimeLoadedMods(root string) ([]runtimeLoadedMod, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	loaded := make([]runtimeLoadedMod, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		mod := readModInfo(filepath.Join(root, entry.Name()), entry.Name())
		if mod.ParseError != "" || strings.TrimSpace(mod.UniqueID) == "" {
			continue
		}
		loaded = append(loaded, runtimeLoadedMod{
			UniqueID: strings.TrimSpace(mod.UniqueID),
			Version:  strings.TrimSpace(mod.Version),
		})
	}
	sort.Slice(loaded, func(i, j int) bool {
		return strings.ToLower(loaded[i].UniqueID) < strings.ToLower(loaded[j].UniqueID)
	})
	return loaded, nil
}

func managedSMAPIBundledTreeDigest(root string) (string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errors.New("SMAPI support mod path is not a directory")
	}
	hash := sha256.New()
	var total int64
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root || entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("SMAPI support mod tree contains symbolic link %s", entry.Name())
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("SMAPI support mod tree contains non-regular file %s", entry.Name())
		}
		total += info.Size()
		if total > maxSMAPIBundledRuntimeSize {
			return fmt.Errorf("SMAPI support mod tree exceeds %d bytes", maxSMAPIBundledRuntimeSize)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		_, _ = io.WriteString(hash, filepath.ToSlash(rel))
		_, _ = io.WriteString(hash, "\x00")
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(hash, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		_, _ = io.WriteString(hash, "\x00")
		return nil
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func reserveSiblingPath(root, pattern string) (string, error) {
	path, err := os.MkdirTemp(root, pattern)
	if err != nil {
		return "", err
	}
	if err := os.Remove(path); err != nil {
		return "", err
	}
	return path, nil
}
