package stardew_junimo

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestNewGameProgressEvidencePersistsLoaderBeforeDirectory(t *testing.T) {
	dataDir := t.TempDir()
	tx, _, err := beginOrResumeNewGameTransaction(dataDir, newGameTestConfig("standard"), "request-loader-progress", "job-loader")
	if err != nil {
		t.Fatal(err)
	}
	if err := writeGameloaderPointer(dataDir, "Future_101"); err != nil {
		t.Fatal(err)
	}
	evidence, err := tx.observeNewGameProgress()
	if err != nil {
		t.Fatal(err)
	}
	if !evidence.Observed || evidence.Kind != newGameProgressGameloader || evidence.SaveName != "" || evidence.Ambiguous {
		t.Fatalf("evidence = %#v", evidence)
	}
	loaded, err := LoadNewGameTransaction(dataDir, tx.record.TransactionID)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.ProgressObserved || loaded.ProgressKind != newGameProgressGameloader || loaded.ProgressSave != "" || loaded.ProgressObservedAt == nil {
		t.Fatalf("persisted record = %#v", loaded)
	}
	if err := os.MkdirAll(filepath.Join(savesDir(dataDir), "Saves", "Future_101"), 0o755); err != nil {
		t.Fatal(err)
	}
	evidence, err = tx.observeNewGameProgress()
	if err != nil {
		t.Fatal(err)
	}
	if evidence.SaveName != "Future_101" || evidence.Ambiguous {
		t.Fatalf("reconciled evidence = %#v", evidence)
	}
}

func TestNewGameProgressEvidenceRequiresExactControlTransaction(t *testing.T) {
	dataDir := t.TempDir()
	tx, _, err := beginOrResumeNewGameTransaction(dataDir, newGameTestConfig("standard"), "request-control-progress", "job-control")
	if err != nil {
		t.Fatal(err)
	}
	statusPath := filepath.Join(controlDir(dataDir), "status.json")
	writeNewGameSnapshotFixture(t, statusPath, []byte(`{
  "state": "save-creating",
  "saveId": "Old_9",
  "newGameTransactionId": "wrong-transaction",
  "newGameCreationObserved": true
}`))
	evidence, err := tx.observeNewGameProgress()
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Observed {
		t.Fatalf("wrong transaction produced progress: %#v", evidence)
	}

	writeNewGameSnapshotFixture(t, statusPath, []byte(fmt.Sprintf(`{
  "state": "save-creating",
  "saveId": "Old_9",
  "newGameTransactionId": %q,
  "newGameCreationObserved": true
}`, tx.record.TransactionID)))
	evidence, err = tx.observeNewGameProgress()
	if err != nil {
		t.Fatal(err)
	}
	if !evidence.Observed || evidence.Kind != newGameProgressControl || evidence.SaveName != "" {
		t.Fatalf("exact Control evidence = %#v", evidence)
	}
}

func TestNewGameProgressControlSaveIDRequiresNewDirectoryAgreement(t *testing.T) {
	dataDir := t.TempDir()
	tx, _, err := beginOrResumeNewGameTransaction(dataDir, newGameTestConfig("standard"), "request-control-candidate", "job-control-candidate")
	if err != nil {
		t.Fatal(err)
	}
	writeNewGameSnapshotFixture(t, filepath.Join(controlDir(dataDir), "status.json"), []byte(fmt.Sprintf(`{
  "state": "save-loaded",
  "saveId": "Old_9",
  "newGameTransactionId": %q,
  "newGameCreationObserved": true
}`, tx.record.TransactionID)))
	evidence, err := tx.observeNewGameProgress()
	if err != nil {
		t.Fatal(err)
	}
	if !evidence.Observed || evidence.SaveName != "" {
		t.Fatalf("old status save was promoted to candidate: %#v", evidence)
	}

	newDir := filepath.Join(savesDir(dataDir), "Saves", "Fresh_9")
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		t.Fatal(err)
	}
	evidence, err = tx.observeNewGameProgress()
	if err != nil {
		t.Fatal(err)
	}
	if evidence.SaveName != "Fresh_9" || evidence.Ambiguous {
		t.Fatalf("numeric suffix reconciliation = %#v", evidence)
	}
}

func TestNewGameProgressDirectoryEvidenceIncludesEmptyAndRejectsAmbiguous(t *testing.T) {
	dataDir := t.TempDir()
	tx, _, err := beginOrResumeNewGameTransaction(dataDir, newGameTestConfig("standard"), "request-directory-progress", "job-directory")
	if err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(savesDir(dataDir), "Saves", "First_1")
	if err := os.MkdirAll(first, 0o755); err != nil {
		t.Fatal(err)
	}
	evidence, err := tx.observeNewGameProgress()
	if err != nil {
		t.Fatal(err)
	}
	if !evidence.Observed || evidence.Kind != newGameProgressDirectory || evidence.SaveName != "First_1" || evidence.Ambiguous {
		t.Fatalf("empty-directory evidence = %#v", evidence)
	}
	if err := os.MkdirAll(filepath.Join(savesDir(dataDir), "Saves", "Second_2"), 0o755); err != nil {
		t.Fatal(err)
	}
	evidence, err = tx.observeNewGameProgress()
	if err != nil {
		t.Fatal(err)
	}
	if !evidence.Ambiguous || evidence.SaveName != "" {
		t.Fatalf("ambiguous evidence = %#v", evidence)
	}
	loaded, err := LoadNewGameTransaction(dataDir, tx.record.TransactionID)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.ProgressAmbiguous || len(loaded.DetectedSaveDirs) != 2 {
		t.Fatalf("persisted ambiguity = %#v", loaded)
	}
}

func TestNewGameProgressEvidenceIsStickyAfterSourceDisappears(t *testing.T) {
	dataDir := t.TempDir()
	tx, _, err := beginOrResumeNewGameTransaction(dataDir, newGameTestConfig("standard"), "request-sticky-progress", "job-sticky")
	if err != nil {
		t.Fatal(err)
	}
	if err := writeGameloaderPointer(dataDir, "Sticky_5"); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.observeNewGameProgress(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(gameloaderPath(dataDir)); err != nil {
		t.Fatal(err)
	}
	evidence, err := tx.observeNewGameProgress()
	if err != nil {
		t.Fatal(err)
	}
	if !evidence.Observed || evidence.Kind != newGameProgressGameloader || evidence.SaveName != "" {
		t.Fatalf("sticky evidence = %#v", evidence)
	}
}

func TestBindTargetSavePersistsExactMarkerAndCandidate(t *testing.T) {
	dataDir := t.TempDir()
	tx, _, err := beginOrResumeNewGameTransaction(dataDir, newGameTestConfig("standard"), "request-bind-target", "job-bind")
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.prepareConfigAndMarker(); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(savesDir(dataDir), "Saves", "Target_11"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := tx.bindTargetSave("Target_11"); err != nil {
		t.Fatal(err)
	}
	if err := tx.bindTargetSave("Target_11"); err != nil {
		t.Fatalf("idempotent bind failed: %v", err)
	}
	var marker newGamePendingMarker
	readJSONFileForTest(t, newGamePendingPath(dataDir), &marker)
	if marker.TransactionID != tx.record.TransactionID || marker.TargetSaveID != "Target_11" {
		t.Fatalf("bound marker = %#v", marker)
	}
	record, err := LoadNewGameTransaction(dataDir, tx.record.TransactionID)
	if err != nil {
		t.Fatal(err)
	}
	if record.CandidateSave != "Target_11" || record.ProgressSave != "Target_11" || !record.ProgressObserved {
		t.Fatalf("bound record = %#v", record)
	}
}

func TestBindTargetSaveRejectsUnprovenAmbiguousAndWrongMarker(t *testing.T) {
	t.Run("unproven", func(t *testing.T) {
		tx := prepareOwnerNewGameTransaction(t, "request-bind-unproven")
		assertNewGameTransactionCode(t, tx.bindTargetSave("Missing_1"), "new_game_target_unproven")
	})

	t.Run("ambiguous", func(t *testing.T) {
		tx := prepareOwnerNewGameTransaction(t, "request-bind-ambiguous")
		for _, name := range []string{"One_1", "Two_2"} {
			if err := os.MkdirAll(filepath.Join(savesDir(tx.dataDir), "Saves", name), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		assertNewGameTransactionCode(t, tx.bindTargetSave("One_1"), "new_game_target_ambiguous")
	})

	t.Run("wrong marker transaction", func(t *testing.T) {
		tx := prepareOwnerNewGameTransaction(t, "request-bind-wrong-marker")
		if err := os.MkdirAll(filepath.Join(savesDir(tx.dataDir), "Saves", "Target_3"), 0o755); err != nil {
			t.Fatal(err)
		}
		var marker newGamePendingMarker
		readJSONFileForTest(t, newGamePendingPath(tx.dataDir), &marker)
		marker.TransactionID = "00000000000000000000000000000000"
		payload, err := json.Marshal(marker)
		if err != nil {
			t.Fatal(err)
		}
		if err := atomicWriteValidatedJSON(newGamePendingPath(tx.dataDir), payload, 0o644); err != nil {
			t.Fatal(err)
		}
		assertNewGameTransactionCode(t, tx.bindTargetSave("Target_3"), "new_game_marker_mismatch")
		if tx.record.CandidateSave != "" {
			t.Fatalf("wrong marker persisted candidate %q", tx.record.CandidateSave)
		}
	})
}

func TestBindTargetSaveMarkerWriteFailureIsRetryableAndFenced(t *testing.T) {
	tx := prepareOwnerNewGameTransaction(t, "request-bind-retry")
	if err := os.MkdirAll(filepath.Join(savesDir(tx.dataDir), "Saves", "Target_8"), 0o755); err != nil {
		t.Fatal(err)
	}
	tx.writeJSON = func(string, []byte, os.FileMode) error { return errors.New("injected marker write failure") }
	assertNewGameTransactionCode(t, tx.bindTargetSave("Target_8"), "new_game_marker_write_failed")
	record, err := LoadNewGameTransaction(tx.dataDir, tx.record.TransactionID)
	if err != nil {
		t.Fatal(err)
	}
	if record.CandidateSave != "Target_8" {
		t.Fatalf("candidate was not durable before marker write: %#v", record)
	}
	tx.writeJSON = atomicWriteValidatedJSON
	if err := tx.bindTargetSave("Target_8"); err != nil {
		t.Fatalf("retry bind: %v", err)
	}

	owner, err := LoadNewGameOwner(tx.dataDir)
	if err != nil {
		t.Fatal(err)
	}
	owner.ExecutorID = "prior-panel-process"
	if err := persistNewGameOwner(tx.dataDir, owner); err != nil {
		t.Fatal(err)
	}
	if _, _, err := beginOrResumeNewGameTransactionWithJobStatus(
		tx.dataDir, tx.record.Config, tx.record.RequestID, "replacement-job",
		func(jobID string) (bool, error) { return jobID != tx.record.JobID, nil },
	); err != nil {
		t.Fatal(err)
	}
	if err := tx.bindTargetSave("Target_8"); !newGameOwnerHasCode(err, "new_game_owner_lost") {
		t.Fatalf("stale bind error = %v", err)
	}
}

func prepareOwnerNewGameTransaction(t *testing.T, requestID string) *newGameTransaction {
	t.Helper()
	tx, _, err := beginOrResumeNewGameTransaction(t.TempDir(), newGameTestConfig("standard"), requestID, "job-"+requestID)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.prepareConfigAndMarker(); err != nil {
		t.Fatal(err)
	}
	return tx
}

func assertNewGameTransactionCode(t *testing.T, err error, code string) {
	t.Helper()
	var typed *NewGameTransactionError
	if !errors.As(err, &typed) || typed.Code != code {
		t.Fatalf("error = %v, want %s", err, code)
	}
}
