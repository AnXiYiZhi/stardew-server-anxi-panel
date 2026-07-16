package stardew_junimo

import (
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
)

var errImportCrossDevice = errors.New("cross-device import staging")

type importStageOps struct {
	renameNoReplace func(string, string) error
	copyTree        func(string, string) error
}

func defaultImportStageOps() importStageOps {
	return importStageOps{renameNoReplace: renameImportNoReplace, copyTree: copyImportTree}
}

func importTransactionDir(dataDir, operationID string) string {
	return filepath.Join(importTransactionsDir(dataDir), operationID)
}

func importTransactionSourceDir(dataDir, operationID string) string {
	return filepath.Join(importTransactionDir(dataDir, operationID), "source")
}

func moveImportSource(source, target string) error {
	if filepath.Clean(source) == filepath.Clean(target) {
		return tightenImportTree(target)
	}
	if _, err := os.Lstat(target); err == nil {
		return fmt.Errorf("transaction source already exists")
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	if err := os.Chmod(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	if err := os.Rename(source, target); err == nil {
		return tightenImportTree(target)
	}
	if err := copyImportTree(source, target); err != nil {
		_ = os.RemoveAll(target)
		return err
	}
	if err := tightenImportTree(target); err != nil {
		_ = os.RemoveAll(target)
		return err
	}
	return os.RemoveAll(source)
}

func tightenImportTree(root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("import source contains a symbolic link")
		}
		if entry.IsDir() {
			return os.Chmod(path, 0o700)
		}
		return os.Chmod(path, 0o600)
	})
}

// StageImportedSaveNoReplace publishes one validated save directory into
// Saves/<saveName> without deleting or replacing any existing target.
func StageImportedSaveNoReplace(dataDir, sourceRoot, saveName string) (string, error) {
	return stageImportedSaveNoReplace(dataDir, sourceRoot, saveName, defaultImportStageOps())
}

func stageImportedSaveNoReplace(dataDir, sourceRoot, saveName string, ops importStageOps) (string, error) {
	if err := validateSaveName(saveName); err != nil {
		return "", &ImportTransactionError{Code: "invalid_save", Message: "save name is invalid", Cause: err}
	}
	sourceSave, err := findSaveDir(sourceRoot, saveName)
	if err != nil {
		return "", fmt.Errorf("find uploaded save: %w", err)
	}
	if err := validateImportSaveDirectory(sourceSave, saveName); err != nil {
		return "", err
	}
	sourceFingerprint, err := importDirectoryFingerprint(sourceSave)
	if err != nil {
		return "", fmt.Errorf("fingerprint import source: %w", err)
	}

	savesRoot := filepath.Join(savesDir(dataDir), "Saves")
	if err := os.MkdirAll(savesRoot, 0o755); err != nil {
		return "", fmt.Errorf("create saves directory: %w", err)
	}
	target := filepath.Join(savesRoot, saveName)
	if _, err := os.Lstat(target); err == nil {
		return "", &ImportTransactionError{Code: ImportErrorSaveExists, Message: "a save with this name already exists"}
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("check save target: %w", err)
	}

	renameErr := ops.renameNoReplace(sourceSave, target)
	if renameErr == nil {
		fingerprint, err := importDirectoryFingerprint(target)
		if err != nil {
			return "", fmt.Errorf("fingerprint staged save: %w", err)
		}
		if fingerprint != sourceFingerprint {
			return "", fmt.Errorf("staged save fingerprint changed during atomic rename")
		}
		return fingerprint, nil
	}
	if errors.Is(renameErr, fs.ErrExist) {
		return "", &ImportTransactionError{Code: ImportErrorSaveExists, Message: "a save with this name already exists"}
	}
	if !errors.Is(renameErr, errImportCrossDevice) && !isImportCrossDeviceError(renameErr) {
		return "", fmt.Errorf("atomically stage imported save: %w", renameErr)
	}

	tempDir, err := os.MkdirTemp(savesRoot, ".import-stage-*")
	if err != nil {
		return "", fmt.Errorf("create hidden import staging directory: %w", err)
	}
	if err := os.Chmod(tempDir, 0o700); err != nil {
		_ = os.RemoveAll(tempDir)
		return "", err
	}
	defer func() { _ = os.RemoveAll(tempDir) }()
	if err := ops.copyTree(sourceSave, tempDir); err != nil {
		return "", fmt.Errorf("copy import source to hidden staging directory: %w", err)
	}
	if err := validateImportSaveDirectory(tempDir, saveName); err != nil {
		return "", fmt.Errorf("validate copied import: %w", err)
	}
	copiedFingerprint, err := importDirectoryFingerprint(tempDir)
	if err != nil {
		return "", err
	}
	if copiedFingerprint != sourceFingerprint {
		return "", fmt.Errorf("copied import fingerprint does not match source")
	}
	if err := ops.renameNoReplace(tempDir, target); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return "", &ImportTransactionError{Code: ImportErrorSaveExists, Message: "a save with this name already exists"}
		}
		return "", fmt.Errorf("publish copied import atomically: %w", err)
	}
	return copiedFingerprint, nil
}

func validateImportSaveDirectory(dir, saveName string) error {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("uploaded save directory is unavailable")
	}
	for _, name := range []string{saveName, "SaveGameInfo"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("uploaded save is missing required file %s", name)
		}
	}
	return nil
}

func copyImportTree(source, target string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("import source contains a symbolic link")
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, rel)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o700)
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(out, in)
		syncErr := out.Sync()
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		if syncErr != nil {
			return syncErr
		}
		return closeErr
	})
}

func importDirectoryFingerprint(root string) (string, error) {
	type item struct {
		path string
		dir  bool
	}
	var items []item
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("save contains a symbolic link")
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		items = append(items, item{path: filepath.ToSlash(rel), dir: entry.IsDir()})
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Slice(items, func(i, j int) bool { return items[i].path < items[j].path })
	h := sha256.New()
	for _, entry := range items {
		kind := "f"
		if entry.dir {
			kind = "d"
		}
		_, _ = io.WriteString(h, kind+"\x00"+entry.path+"\x00")
		if entry.dir {
			continue
		}
		file, err := os.Open(filepath.Join(root, filepath.FromSlash(entry.path)))
		if err != nil {
			return "", err
		}
		_, copyErr := io.Copy(h, file)
		closeErr := file.Close()
		if copyErr != nil {
			return "", copyErr
		}
		if closeErr != nil {
			return "", closeErr
		}
		_, _ = io.WriteString(h, "\x00")
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func importOperationDigest(operationID string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(operationID)))
	return hex.EncodeToString(sum[:])[:12]
}
