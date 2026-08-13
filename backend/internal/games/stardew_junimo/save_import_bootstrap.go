package stardew_junimo

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const importBootstrapPrefix = "AnxiImportBootstrap_"

func importBootstrapSaveName(operationID string) string {
	return importBootstrapPrefix + operationID
}

func importBootstrapSourceRoot(dataDir, operationID string) string {
	return filepath.Join(importTransactionDir(dataDir, operationID), "bootstrap-source")
}

// prepareImportBootstrap provisions a transaction-owned maintenance world when
// the instance has no active save. Junimo rejects importing the currently
// loaded save, while a normal zero-save startup creates an unrelated new world.
// A private clone therefore gives the command a stable, non-target world to run
// against without altering the uploaded target before Junimo processes it.
func prepareImportBootstrap(dataDir, operationID string) error {
	j, err := LoadImportJournal(dataDir, operationID)
	if err != nil {
		return err
	}
	if j.OriginalActiveSave != "" {
		return nil
	}
	expectedName := importBootstrapSaveName(operationID)
	if j.BootstrapSaveName != "" && j.BootstrapSaveName != expectedName {
		return &ImportTransactionError{Code: ImportErrorRecoveryRequired, Message: "import bootstrap identity is inconsistent"}
	}
	if j.BootstrapSaveName == "" {
		j.BootstrapSaveName = expectedName
		if err := WriteImportJournal(dataDir, j); err != nil {
			return err
		}
	}

	bootstrapDir := filepath.Join(savesDir(dataDir), "Saves", expectedName)
	if j.BootstrapSaveFingerprint == "" {
		if _, statErr := os.Lstat(bootstrapDir); statErr == nil {
			return &ImportTransactionError{Code: ImportErrorRecoveryRequired, Message: "an unjournaled import bootstrap already exists"}
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return fmt.Errorf("check import bootstrap target: %w", statErr)
		}
		fingerprint, buildErr := buildImportBootstrapSource(dataDir, j, expectedName)
		if buildErr != nil {
			return buildErr
		}
		j, err = LoadImportJournal(dataDir, operationID)
		if err != nil {
			return err
		}
		j.BootstrapSaveName = expectedName
		j.BootstrapSaveFingerprint = fingerprint
		if err := WriteImportJournal(dataDir, j); err != nil {
			return err
		}
	}

	if !j.BootstrapSaveCreated {
		if _, statErr := os.Lstat(bootstrapDir); statErr == nil {
			return &ImportTransactionError{Code: ImportErrorRecoveryRequired, Message: "import bootstrap publication is ambiguous"}
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return fmt.Errorf("check import bootstrap publication: %w", statErr)
		}
		fingerprint, stageErr := StageImportedSaveNoReplace(dataDir, importBootstrapSourceRoot(dataDir, operationID), expectedName)
		if stageErr != nil {
			return fmt.Errorf("publish import bootstrap: %w", stageErr)
		}
		if fingerprint != j.BootstrapSaveFingerprint {
			return &ImportTransactionError{Code: ImportErrorRecoveryRequired, Message: "published import bootstrap fingerprint changed"}
		}
		j.BootstrapSaveCreated = true
		if err := WriteImportJournal(dataDir, j); err != nil {
			return err
		}
	}
	fingerprint, fingerprintErr := importDirectoryFingerprint(bootstrapDir)
	if fingerprintErr != nil || fingerprint != j.BootstrapSaveFingerprint {
		return &ImportTransactionError{Code: ImportErrorRecoveryRequired, Message: "import bootstrap no longer matches its journal", Cause: fingerprintErr}
	}

	if err := writeGameloaderPointer(dataDir, expectedName); err != nil {
		return fmt.Errorf("select import bootstrap: %w", err)
	}
	j, err = LoadImportJournal(dataDir, operationID)
	if err != nil {
		return err
	}
	j.OriginalActiveSave = expectedName
	j.BootstrapCleanupCompleted = false
	return WriteImportJournal(dataDir, j)
}

func buildImportBootstrapSource(dataDir string, j ImportJournal, bootstrapName string) (string, error) {
	targetDir := filepath.Join(savesDir(dataDir), "Saves", j.SaveName)
	before, err := importDirectoryFingerprint(targetDir)
	if err != nil {
		return "", fmt.Errorf("fingerprint import target before bootstrap copy: %w", err)
	}
	if before != j.StagedSaveFingerprint {
		return "", &ImportTransactionError{Code: ImportErrorRecoveryRequired, Message: "staged import target changed before bootstrap copy"}
	}

	root := importBootstrapSourceRoot(dataDir, j.OperationID)
	if err := os.RemoveAll(root); err != nil {
		return "", fmt.Errorf("reset transaction bootstrap source: %w", err)
	}
	cloneDir := filepath.Join(root, bootstrapName)
	if err := copyImportTree(targetDir, cloneDir); err != nil {
		_ = os.RemoveAll(root)
		return "", fmt.Errorf("copy import bootstrap: %w", err)
	}
	originalMain := filepath.Join(cloneDir, j.SaveName)
	bootstrapMain := filepath.Join(cloneDir, bootstrapName)
	if err := renameImportNoReplace(originalMain, bootstrapMain); err != nil {
		_ = os.RemoveAll(root)
		return "", fmt.Errorf("rename import bootstrap main save: %w", err)
	}
	if err := validateImportSaveDirectory(cloneDir, bootstrapName); err != nil {
		_ = os.RemoveAll(root)
		return "", fmt.Errorf("validate import bootstrap: %w", err)
	}
	after, err := importDirectoryFingerprint(targetDir)
	if err != nil || after != before {
		_ = os.RemoveAll(root)
		return "", &ImportTransactionError{Code: ImportErrorRecoveryRequired, Message: "staged import target changed during bootstrap copy", Cause: err}
	}
	fingerprint, err := importDirectoryFingerprint(cloneDir)
	if err != nil {
		_ = os.RemoveAll(root)
		return "", err
	}
	return fingerprint, nil
}

func cleanupCompletedImportBootstrap(dataDir string, j *ImportJournal) error {
	if j.BootstrapSaveName == "" {
		return nil
	}
	if j.BootstrapSaveName != importBootstrapSaveName(j.OperationID) || j.BootstrapSaveName == j.SaveName {
		return &ImportTransactionError{Code: ImportErrorRecoveryRequired, Message: "import bootstrap cleanup identity is invalid"}
	}
	if !j.BootstrapSaveCreated {
		return &ImportTransactionError{Code: ImportErrorRecoveryRequired, Message: "import bootstrap ownership is unconfirmed"}
	}
	pointer, err := readActivePointerStrict(dataDir)
	if err != nil || pointer != j.SaveName {
		return &ImportTransactionError{Code: ImportErrorRecoveryRequired, Message: "target save is not active before bootstrap cleanup", Cause: err}
	}
	bootstrapDir := filepath.Join(savesDir(dataDir), "Saves", j.BootstrapSaveName)
	if err := os.RemoveAll(bootstrapDir); err != nil {
		return fmt.Errorf("remove completed import bootstrap: %w", err)
	}
	if _, err := os.Stat(bootstrapDir); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("verify completed import bootstrap removal: %w", err)
	}
	if err := os.RemoveAll(importBootstrapSourceRoot(dataDir, j.OperationID)); err != nil {
		return fmt.Errorf("remove completed import bootstrap source: %w", err)
	}
	j.BootstrapCleanupCompleted = true
	return nil
}

func cleanupUnsubmittedImportBootstrap(dataDir string, j ImportJournal) error {
	if j.BootstrapSaveName == "" {
		return nil
	}
	if j.BootstrapSaveName != importBootstrapSaveName(j.OperationID) || j.BootstrapSaveName == j.SaveName {
		return &ImportTransactionError{Code: ImportErrorRecoveryRequired, Message: "import bootstrap cleanup identity is invalid"}
	}
	if !j.BootstrapSaveCreated {
		_, err := readActivePointerStrict(dataDir)
		if err == nil || !errors.Is(err, os.ErrNotExist) {
			return &ImportTransactionError{Code: ImportErrorRecoveryRequired, Message: "import bootstrap ownership is unconfirmed while an active pointer exists", Cause: err}
		}
		bootstrapDir := filepath.Join(savesDir(dataDir), "Saves", j.BootstrapSaveName)
		if _, err := os.Lstat(bootstrapDir); err == nil {
			return &ImportTransactionError{Code: ImportErrorRecoveryRequired, Message: "an unowned import bootstrap path requires recovery"}
		} else if !errors.Is(err, os.ErrNotExist) {
			return &ImportTransactionError{Code: ImportErrorRecoveryRequired, Message: "import bootstrap ownership cannot be verified", Cause: err}
		}
		if err := os.RemoveAll(importBootstrapSourceRoot(dataDir, j.OperationID)); err != nil {
			return fmt.Errorf("remove unowned import bootstrap source: %w", err)
		}
		return nil
	}
	pointer, err := readActivePointerStrict(dataDir)
	if err == nil {
		if pointer != j.BootstrapSaveName {
			return &ImportTransactionError{Code: ImportErrorRecoveryRequired, Message: "active pointer changed before import bootstrap cleanup"}
		}
		if err := os.Remove(gameloaderPath(dataDir)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove import bootstrap pointer: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return &ImportTransactionError{Code: ImportErrorRecoveryRequired, Message: "import bootstrap pointer is unreadable", Cause: err}
	}
	if err := os.RemoveAll(filepath.Join(savesDir(dataDir), "Saves", j.BootstrapSaveName)); err != nil {
		return fmt.Errorf("remove unsubmitted import bootstrap: %w", err)
	}
	if err := os.RemoveAll(importBootstrapSourceRoot(dataDir, j.OperationID)); err != nil {
		return fmt.Errorf("remove import bootstrap source: %w", err)
	}
	return nil
}
