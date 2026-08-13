package stardew_junimo

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestBeginOrResumeNewGameTransactionPersistsSchemaV2BaselinesAndWriter(t *testing.T) {
	dataDir := t.TempDir()
	writeNewGameTestSave(t, dataDir, "Existing_100", "0")
	writeNewGameSnapshotFixture(t, filepath.Join(savesDir(dataDir), "Saves", "Existing_100", "SaveGameInfo"), []byte("<Farmer/>"))
	if err := writeGameloaderPointer(dataDir, "Existing_100"); err != nil {
		t.Fatal(err)
	}
	writeNewGameSnapshotFixture(t, filepath.Join(controlDir(dataDir), "status.json"), []byte(`{
  "state": "save-loaded",
  "saveId": "Existing_100",
  "newGameTransactionId": "old-transaction"
}`))

	tx, resumed, err := beginOrResumeNewGameTransaction(dataDir, newGameTestConfig("standard"), "request-schema-v2", "job-1")
	if err != nil {
		t.Fatal(err)
	}
	if resumed {
		t.Fatal("new request unexpectedly resumed")
	}
	if tx.record.SchemaVersion != 2 || tx.record.InstanceDataDirHash == "" || tx.record.ConfigSHA256 == "" || tx.record.OwnerToken == "" {
		t.Fatalf("missing v2 identity: %#v", tx.record)
	}
	if tx.record.InitialGameloaderSave != "Existing_100" || tx.record.InitialRuntimeStatus.SaveID != "Existing_100" {
		t.Fatalf("missing transaction-start baselines: %#v", tx.record)
	}
	if tx.record.CreationWriter != newGameCreationWriterHTTP {
		t.Fatalf("creation writer = %q, want http", tx.record.CreationWriter)
	}
	owner, err := LoadNewGameOwner(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if owner.TransactionID != tx.record.TransactionID || owner.OwnerToken != tx.ownerToken || owner.State != newGameOwnerStateActive {
		t.Fatalf("owner = %#v, tx = %#v", owner, tx.record)
	}

	otherDir := t.TempDir()
	other, _, err := beginOrResumeNewGameTransaction(otherDir, newGameTestConfig("standard"), "request-startup", "job-2")
	if err != nil {
		t.Fatal(err)
	}
	if other.record.CreationWriter != newGameCreationWriterStartup {
		t.Fatalf("creation writer = %q, want startup", other.record.CreationWriter)
	}
}

func TestBeginOrResumeNewGameTransactionIsIdempotentAndConflicts(t *testing.T) {
	dataDir := t.TempDir()
	cfg := newGameTestConfig("standard")
	first, resumed, err := beginOrResumeNewGameTransaction(dataDir, cfg, "request-idempotent", "job-1")
	if err != nil || resumed {
		t.Fatalf("first begin = resumed %v, err %v", resumed, err)
	}
	if _, _, err := beginOrResumeNewGameTransaction(dataDir, cfg, "request-idempotent", "job-2"); !newGameOwnerHasCode(err, "new_game_in_progress") {
		t.Fatalf("same-process second job error = %v", err)
	}

	owner, err := LoadNewGameOwner(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	owner.ExecutorID = "prior-panel-process"
	if err := persistNewGameOwner(dataDir, owner); err != nil {
		t.Fatal(err)
	}
	resumedTx, wasResumed, err := beginOrResumeNewGameTransactionWithJobStatus(
		dataDir, cfg, "request-idempotent", "job-after-restart",
		func(jobID string) (bool, error) { return jobID != "job-1", nil },
	)
	if err != nil || !wasResumed {
		t.Fatalf("restart resume = resumed %v, err %v", wasResumed, err)
	}
	if resumedTx.record.TransactionID != first.record.TransactionID || resumedTx.ownerToken == first.ownerToken {
		t.Fatalf("resume identity = tx %q token %q; first tx %q token %q", resumedTx.record.TransactionID, resumedTx.ownerToken, first.record.TransactionID, first.ownerToken)
	}
	if err := first.mark(newGameStateObserving); !newGameOwnerHasCode(err, "new_game_owner_lost") {
		t.Fatalf("stale writer mark error = %v", err)
	}
	if _, _, err := beginOrResumeNewGameTransaction(dataDir, cfg, "other-request", "job-3"); !newGameOwnerHasCode(err, "new_game_in_progress") {
		t.Fatalf("other request error = %v", err)
	}
	changed := cfg
	changed.FarmName = "Different Farm"
	if _, _, err := beginOrResumeNewGameTransaction(dataDir, changed, "request-idempotent", "job-after-restart"); !newGameOwnerHasCode(err, "new_game_request_conflict") {
		t.Fatalf("same request changed config error = %v", err)
	}
}

func TestConcurrentBeginOrResumeNewGameTransactionHasOneLiveWriter(t *testing.T) {
	dataDir := t.TempDir()
	cfg := newGameTestConfig("standard")
	start := make(chan struct{})
	type result struct {
		tx  *newGameTransaction
		err error
	}
	results := make(chan result, 2)
	for i := 0; i < 2; i++ {
		go func() {
			<-start
			tx, _, err := beginOrResumeNewGameTransaction(dataDir, cfg, "request-concurrent", "same-job")
			results <- result{tx: tx, err: err}
		}()
	}
	close(start)
	successes := 0
	conflicts := 0
	for i := 0; i < 2; i++ {
		got := <-results
		switch {
		case got.err == nil && got.tx != nil:
			successes++
		case newGameOwnerHasCode(got.err, "new_game_in_progress"):
			conflicts++
		default:
			t.Fatalf("unexpected concurrent result: tx=%#v err=%v", got.tx, got.err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d", successes, conflicts)
	}
}

func TestBeginOrResumeNewGameTransactionSameExecutorRequiresInactiveJobProof(t *testing.T) {
	dataDir := t.TempDir()
	cfg := newGameTestConfig("standard")
	first, _, err := beginOrResumeNewGameTransaction(dataDir, cfg, "request-job-proof", "old-job")
	if err != nil {
		t.Fatal(err)
	}
	oldToken := first.ownerToken
	checkedJob := ""
	_, _, err = beginOrResumeNewGameTransactionWithJobStatus(
		dataDir, cfg, "request-job-proof", "replacement-job",
		func(jobID string) (bool, error) {
			checkedJob = jobID
			return true, nil
		},
	)
	if !newGameOwnerHasCode(err, "new_game_in_progress") || checkedJob != "old-job" {
		t.Fatalf("active job proof = checked %q err %v", checkedJob, err)
	}
	owner, err := LoadNewGameOwner(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if owner.OwnerToken != oldToken || owner.JobID != "old-job" {
		t.Fatalf("active owner was rotated: %#v", owner)
	}

	_, _, err = beginOrResumeNewGameTransactionWithJobStatus(
		dataDir, cfg, "request-job-proof", "replacement-job",
		func(string) (bool, error) { return false, errors.New("job store unavailable") },
	)
	if !newGameOwnerHasCode(err, "new_game_owner_job_check_failed") {
		t.Fatalf("job check failure = %v", err)
	}

	replacement, resumed, err := beginOrResumeNewGameTransactionWithJobStatus(
		dataDir, cfg, "request-job-proof", "replacement-job",
		func(jobID string) (bool, error) { return jobID != "old-job", nil },
	)
	if err != nil || !resumed {
		t.Fatalf("inactive job takeover = resumed %v err %v", resumed, err)
	}
	if replacement.ownerToken == oldToken || replacement.record.JobID != "replacement-job" {
		t.Fatalf("replacement = token %q record %#v", replacement.ownerToken, replacement.record)
	}
	if err := first.mark(newGameStateObserving); !newGameOwnerHasCode(err, "new_game_owner_lost") {
		t.Fatalf("old writer was not fenced: %v", err)
	}
}

func TestBeginOrResumeNewGameTransactionCrossProcessRejectsActiveOldJob(t *testing.T) {
	dataDir := t.TempDir()
	cfg := newGameTestConfig("standard")
	first, _, err := beginOrResumeNewGameTransaction(dataDir, cfg, "request-cross-process", "old-job")
	if err != nil {
		t.Fatal(err)
	}
	owner, err := LoadNewGameOwner(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	owner.ExecutorID = "prior-panel-process"
	if err := persistNewGameOwner(dataDir, owner); err != nil {
		t.Fatal(err)
	}
	checkedJob := ""
	_, _, err = beginOrResumeNewGameTransactionWithJobStatus(
		dataDir, cfg, "request-cross-process", "replacement-job",
		func(jobID string) (bool, error) {
			checkedJob = jobID
			return true, nil
		},
	)
	if !newGameOwnerHasCode(err, "new_game_in_progress") || checkedJob != "old-job" {
		t.Fatalf("cross-process active proof = checked %q err %v", checkedJob, err)
	}
	after, err := LoadNewGameOwner(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if after.OwnerToken != first.ownerToken || after.JobID != "old-job" || after.ExecutorID != "prior-panel-process" {
		t.Fatalf("active cross-process owner was rotated: %#v", after)
	}
}

func TestBeginOrResumeNewGameTransactionRecoversReservedOwnerAndInterruptedRotation(t *testing.T) {
	t.Run("reserved owner before transaction record", func(t *testing.T) {
		dataDir := t.TempDir()
		cfg := newGameTestConfig("standard")
		dataDirHash, err := newGameInstanceDataDirHash(dataDir)
		if err != nil {
			t.Fatal(err)
		}
		configHash, err := newGameConfigSHA256(cfg)
		if err != nil {
			t.Fatal(err)
		}
		txID, _ := newGameRandomHex(16)
		token, _ := newGameRandomHex(32)
		claimed, err := claimNewGameOwner(dataDir, NewGameOwnerRecord{
			SchemaVersion: newGameOwnerSchemaVersion, InstanceDataDirHash: dataDirHash,
			RequestID: "request-reserved", ConfigSHA256: configHash, TransactionID: txID,
			JobID: "old-job", ExecutorID: "prior-panel-process", OwnerToken: token, State: newGameOwnerStateReserved,
		})
		if err != nil || !claimed {
			t.Fatalf("claim = %v, %v", claimed, err)
		}
		tx, resumed, err := beginOrResumeNewGameTransactionWithJobStatus(
			dataDir, cfg, "request-reserved", "new-job",
			func(jobID string) (bool, error) { return jobID != "old-job", nil },
		)
		if err != nil || !resumed || tx.record.TransactionID != txID {
			t.Fatalf("reserved resume = tx %#v resumed %v err %v", tx, resumed, err)
		}
	})

	t.Run("owner token rotation directory", func(t *testing.T) {
		dataDir := t.TempDir()
		cfg := newGameTestConfig("standard")
		first, _, err := beginOrResumeNewGameTransaction(dataDir, cfg, "request-rotation", "old-job")
		if err != nil {
			t.Fatal(err)
		}
		owner, err := LoadNewGameOwner(dataDir)
		if err != nil {
			t.Fatal(err)
		}
		owner.ExecutorID = "prior-panel-process"
		if err := persistNewGameOwner(dataDir, owner); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(newGameOwnerDir(dataDir), newGameOwnerRotationDir(dataDir)); err != nil {
			t.Fatal(err)
		}
		resumedTx, resumed, err := beginOrResumeNewGameTransactionWithJobStatus(
			dataDir, cfg, "request-rotation", "new-job",
			func(jobID string) (bool, error) { return jobID != "old-job", nil },
		)
		if err != nil || !resumed || resumedTx.record.TransactionID != first.record.TransactionID {
			t.Fatalf("rotation recovery = tx %#v resumed %v err %v", resumedTx, resumed, err)
		}
		if _, err := os.Stat(newGameOwnerRotationDir(dataDir)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("rotation directory survived recovery: %v", err)
		}
	})
}

func TestNewGameOwnerAtomicClaimAllowsExactlyOneWinner(t *testing.T) {
	dataDir := t.TempDir()
	dataDirHash, err := newGameInstanceDataDirHash(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	configHash, err := newGameConfigSHA256(newGameTestConfig("standard"))
	if err != nil {
		t.Fatal(err)
	}
	const contenders = 12
	start := make(chan struct{})
	results := make(chan bool, contenders)
	errs := make(chan error, contenders)
	var wg sync.WaitGroup
	for i := 0; i < contenders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			txID, randomErr := newGameRandomHex(16)
			if randomErr != nil {
				errs <- randomErr
				return
			}
			token, randomErr := newGameRandomHex(32)
			if randomErr != nil {
				errs <- randomErr
				return
			}
			<-start
			claimed, claimErr := claimNewGameOwner(dataDir, NewGameOwnerRecord{
				SchemaVersion: newGameOwnerSchemaVersion, InstanceDataDirHash: dataDirHash,
				RequestID: "atomic-request", ConfigSHA256: configHash, TransactionID: txID,
				ExecutorID: "test-executor", OwnerToken: token, State: newGameOwnerStateReserved,
			})
			if claimErr != nil {
				errs <- claimErr
				return
			}
			results <- claimed
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	winners := 0
	for claimed := range results {
		if claimed {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("owner winners = %d, want 1", winners)
	}
	if _, err := LoadNewGameOwner(dataDir); err != nil {
		t.Fatal(err)
	}
}

func TestNewGameOwnerCrashBeforePublishLeavesInertStaging(t *testing.T) {
	t.Run("complete staged claim", func(t *testing.T) {
		dataDir := t.TempDir()
		stagedOwner := newGameOwnerRecordForTest(t, dataDir, "request-staged-crash")
		stagingDir, err := stageNewGameOwnerClaim(dataDir, stagedOwner)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := cleanupNewGameOwnerClaimStaging(stagingDir); err != nil {
				t.Error(err)
			}
		})
		if _, err := LoadNewGameOwner(dataDir); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("staging became visible owner: %v", err)
		}
		if runtime.GOOS != "windows" {
			info, err := os.Stat(filepath.Join(stagingDir, "owner.json"))
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != 0o600 {
				t.Fatalf("staged owner mode = %04o, want 0600", info.Mode().Perm())
			}
		}

		winner := newGameOwnerRecordForTest(t, dataDir, "request-after-staged-crash")
		claimed, err := claimNewGameOwner(dataDir, winner)
		if err != nil || !claimed {
			t.Fatalf("claim after staged crash = claimed %v, err %v", claimed, err)
		}
		got, err := LoadNewGameOwner(dataDir)
		if err != nil {
			t.Fatal(err)
		}
		if got.RequestID != winner.RequestID || got.OwnerToken != winner.OwnerToken {
			t.Fatalf("published owner = %#v, want winner %#v", got, winner)
		}
		if _, err := os.Stat(stagingDir); err != nil {
			t.Fatalf("foreign crash staging was removed: %v", err)
		}
	})

	t.Run("incomplete staged directory", func(t *testing.T) {
		dataDir := t.TempDir()
		if err := ensureNewGameControlDir(dataDir); err != nil {
			t.Fatal(err)
		}
		stagingDir, err := os.MkdirTemp(controlDir(dataDir), newGameOwnerClaimStagingPrefix)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := cleanupNewGameOwnerClaimStaging(stagingDir); err != nil {
				t.Error(err)
			}
		})
		winner := newGameOwnerRecordForTest(t, dataDir, "request-after-empty-staging")
		claimed, err := claimNewGameOwner(dataDir, winner)
		if err != nil || !claimed {
			t.Fatalf("claim after incomplete staging = claimed %v, err %v", claimed, err)
		}
	})
}

func TestNewGameOwnerPublishNeverReplacesExistingWinner(t *testing.T) {
	dataDir := t.TempDir()
	winner := newGameOwnerRecordForTest(t, dataDir, "request-published-winner")
	claimed, err := claimNewGameOwner(dataDir, winner)
	if err != nil || !claimed {
		t.Fatalf("first claim = claimed %v, err %v", claimed, err)
	}
	contender := newGameOwnerRecordForTest(t, dataDir, "request-losing-contender")
	stagingDir, err := stageNewGameOwnerClaim(dataDir, contender)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := cleanupNewGameOwnerClaimStaging(stagingDir); err != nil {
			t.Error(err)
		}
	})
	claimed, err = publishNewGameOwnerClaim(dataDir, stagingDir)
	if err != nil || claimed {
		t.Fatalf("publish against existing winner = claimed %v, err %v", claimed, err)
	}
	got, err := LoadNewGameOwner(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if got.OwnerToken != winner.OwnerToken || got.RequestID != winner.RequestID {
		t.Fatalf("existing winner was replaced: %#v", got)
	}
}

func TestNewGameOwnerNoReplaceRenameRejectsEmptyDestination(t *testing.T) {
	dataDir := t.TempDir()
	contender := newGameOwnerRecordForTest(t, dataDir, "request-empty-destination")
	stagingDir, err := stageNewGameOwnerClaim(dataDir, contender)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := cleanupNewGameOwnerClaimStaging(stagingDir); err != nil {
			t.Error(err)
		}
	})
	if err := os.Mkdir(newGameOwnerDir(dataDir), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := renameNewGameOwnerNoReplace(stagingDir, newGameOwnerDir(dataDir)); err == nil {
		t.Fatal("no-replace rename unexpectedly replaced an empty destination directory")
	}
	assertEmptyOwnerDirectoryForTest(t, dataDir)
	if _, err := os.Stat(filepath.Join(stagingDir, "owner.json")); err != nil {
		t.Fatalf("losing staging was changed: %v", err)
	}
}

func TestNewGameOwnerRecoversOnlyProvenLegacyEmptyClaim(t *testing.T) {
	t.Run("old empty directory without evidence", func(t *testing.T) {
		dataDir := t.TempDir()
		makeLegacyEmptyNewGameOwnerDir(t, dataDir)
		winner := newGameOwnerRecordForTest(t, dataDir, "request-empty-recovery")
		claimed, err := claimNewGameOwner(dataDir, winner)
		if err != nil || !claimed {
			t.Fatalf("legacy empty recovery = claimed %v, err %v", claimed, err)
		}
		got, err := LoadNewGameOwner(dataDir)
		if err != nil {
			t.Fatal(err)
		}
		if got.OwnerToken != winner.OwnerToken {
			t.Fatalf("recovered owner = %#v", got)
		}
		entries, err := os.ReadDir(controlDir(dataDir))
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), newGameOwnerEmptyRecoveryPrefix) {
				t.Fatalf("empty recovery quarantine survived: %s", entry.Name())
			}
		}
	})

	t.Run("same-process empty directory", func(t *testing.T) {
		dataDir := t.TempDir()
		if err := ensureNewGameControlDir(dataDir); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(newGameOwnerDir(dataDir), 0o700); err != nil {
			t.Fatal(err)
		}
		contender := newGameOwnerRecordForTest(t, dataDir, "request-current-empty")
		if claimed, err := claimNewGameOwner(dataDir, contender); !newGameOwnerHasCode(err, "new_game_recovery_required") || claimed {
			t.Fatalf("same-process empty claim = claimed %v, err %v", claimed, err)
		}
		assertEmptyOwnerDirectoryForTest(t, dataDir)
	})

	t.Run("unknown file", func(t *testing.T) {
		dataDir := t.TempDir()
		makeLegacyEmptyNewGameOwnerDir(t, dataDir)
		unknownPath := filepath.Join(newGameOwnerDir(dataDir), "unknown")
		if err := os.WriteFile(unknownPath, []byte("do not delete"), 0o600); err != nil {
			t.Fatal(err)
		}
		contender := newGameOwnerRecordForTest(t, dataDir, "request-unknown-owner")
		if claimed, err := claimNewGameOwner(dataDir, contender); !newGameOwnerHasCode(err, "new_game_recovery_required") || claimed {
			t.Fatalf("unknown owner claim = claimed %v, err %v", claimed, err)
		}
		if raw, err := os.ReadFile(unknownPath); err != nil || string(raw) != "do not delete" {
			t.Fatalf("unknown evidence was changed: %q, %v", raw, err)
		}
	})

	t.Run("rotation directory", func(t *testing.T) {
		dataDir := t.TempDir()
		makeLegacyEmptyNewGameOwnerDir(t, dataDir)
		if err := os.Mkdir(newGameOwnerRotationDir(dataDir), 0o700); err != nil {
			t.Fatal(err)
		}
		contender := newGameOwnerRecordForTest(t, dataDir, "request-rotation-uncertain")
		if claimed, err := claimNewGameOwner(dataDir, contender); !newGameOwnerHasCode(err, "new_game_recovery_required") || claimed {
			t.Fatalf("rotation uncertainty claim = claimed %v, err %v", claimed, err)
		}
		assertEmptyOwnerDirectoryForTest(t, dataDir)
		if _, err := os.Stat(newGameOwnerRotationDir(dataDir)); err != nil {
			t.Fatalf("rotation evidence was removed: %v", err)
		}
	})

	t.Run("pending marker", func(t *testing.T) {
		dataDir := t.TempDir()
		makeLegacyEmptyNewGameOwnerDir(t, dataDir)
		if err := atomicWriteValidatedJSON(newGamePendingPath(dataDir), []byte(`{"state":"pending"}`), 0o600); err != nil {
			t.Fatal(err)
		}
		contender := newGameOwnerRecordForTest(t, dataDir, "request-marker-uncertain")
		if claimed, err := claimNewGameOwner(dataDir, contender); !newGameOwnerHasCode(err, "new_game_recovery_required") || claimed {
			t.Fatalf("pending marker claim = claimed %v, err %v", claimed, err)
		}
		if _, err := os.Stat(newGamePendingPath(dataDir)); err != nil {
			t.Fatalf("pending marker was removed: %v", err)
		}
	})

	t.Run("runtime progress", func(t *testing.T) {
		dataDir := t.TempDir()
		makeLegacyEmptyNewGameOwnerDir(t, dataDir)
		writeNewGameSnapshotFixture(t, filepath.Join(controlDir(dataDir), "status.json"), []byte(`{
  "state": "save-creating",
  "newGameTransactionId": "unmatched-runtime-transaction",
  "newGameCreationObserved": true
}`))
		contender := newGameOwnerRecordForTest(t, dataDir, "request-progress-uncertain")
		if claimed, err := claimNewGameOwner(dataDir, contender); !newGameOwnerHasCode(err, "new_game_recovery_required") || claimed {
			t.Fatalf("runtime progress claim = claimed %v, err %v", claimed, err)
		}
		assertEmptyOwnerDirectoryForTest(t, dataDir)
	})

	t.Run("nonterminal transaction from another request", func(t *testing.T) {
		dataDir := t.TempDir()
		cfg := newGameTestConfig("standard")
		configHash, err := newGameConfigSHA256(cfg)
		if err != nil {
			t.Fatal(err)
		}
		txID, err := newGameRandomHex(16)
		if err != nil {
			t.Fatal(err)
		}
		tx, err := beginNewGameTransactionWithIdentity(
			dataDir, cfg, txID, "request-uncertain-original", "old-job", configHash, "", false,
		)
		if err != nil {
			t.Fatal(err)
		}
		makeLegacyEmptyNewGameOwnerDir(t, dataDir)
		contender := newGameOwnerRecordForTest(t, dataDir, "request-must-not-take-over")
		if claimed, err := claimNewGameOwner(dataDir, contender); !newGameOwnerHasCode(err, "new_game_recovery_required") || claimed {
			t.Fatalf("uncertain transaction claim = claimed %v, err %v", claimed, err)
		}
		persisted, err := LoadNewGameTransaction(dataDir, tx.record.TransactionID)
		if err != nil || persisted.RequestID != "request-uncertain-original" || persisted.Stage != newGameStatePreparing {
			t.Fatalf("uncertain transaction was changed: %#v, %v", persisted, err)
		}
		assertEmptyOwnerDirectoryForTest(t, dataDir)
	})
}

func TestNewGameOwnerReleaseRequiresMatchingTerminalRecord(t *testing.T) {
	dataDir := t.TempDir()
	tx, _, err := beginOrResumeNewGameTransaction(dataDir, newGameTestConfig("standard"), "request-release", "job-release")
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.releaseOwner(); !newGameOwnerHasCode(err, "new_game_owner_not_terminal") {
		t.Fatalf("nonterminal release error = %v", err)
	}
	recordCompleteNewGameDurabilityEvidence(t, tx, "Fresh_1")
	if err := tx.complete("Fresh_1"); err != nil {
		t.Fatal(err)
	}
	if err := tx.releaseOwner(); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadNewGameOwner(dataDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("owner after release = %v", err)
	}
	resumed, wasResumed, err := beginOrResumeNewGameTransaction(dataDir, newGameTestConfig("standard"), "request-release", "new-job")
	if err != nil || !wasResumed || resumed.record.TransactionID != tx.record.TransactionID {
		t.Fatalf("terminal idempotent resume = %#v, resumed %v, err %v", resumed, wasResumed, err)
	}
	if _, err := LoadNewGameOwner(dataDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("terminal retry unexpectedly acquired owner: %v", err)
	}
}

func TestNewGameOwnerReleaseAfterCompleteRollback(t *testing.T) {
	dataDir := t.TempDir()
	tx, _, err := beginOrResumeNewGameTransaction(dataDir, newGameTestConfig("standard"), "request-rollback-release", "job-rollback")
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.rollback(errors.New("injected failure"), "new_game_failed", newGameStateFailed); err != nil {
		t.Fatal(err)
	}
	if tx.record.Stage != newGameStateRolledBack || !tx.record.RollbackCompleted {
		t.Fatalf("rollback terminal = %#v", tx.record)
	}
	if err := tx.releaseOwner(); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadNewGameOwner(dataDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("owner after rolled-back release = %v", err)
	}
}

func newGameOwnerHasCode(err error, code string) bool {
	var typed *NewGameOwnerError
	return errors.As(err, &typed) && typed.Code == code
}

func newGameOwnerRecordForTest(t *testing.T, dataDir, requestID string) NewGameOwnerRecord {
	t.Helper()
	dataDirHash, err := newGameInstanceDataDirHash(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	configHash, err := newGameConfigSHA256(newGameTestConfig("standard"))
	if err != nil {
		t.Fatal(err)
	}
	transactionID, err := newGameRandomHex(16)
	if err != nil {
		t.Fatal(err)
	}
	ownerToken, err := newGameRandomHex(32)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	return NewGameOwnerRecord{
		SchemaVersion:       newGameOwnerSchemaVersion,
		InstanceDataDirHash: dataDirHash,
		RequestID:           requestID,
		ConfigSHA256:        configHash,
		TransactionID:       transactionID,
		JobID:               "test-job",
		ExecutorID:          "test-executor",
		OwnerToken:          ownerToken,
		State:               newGameOwnerStateReserved,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
}

func makeLegacyEmptyNewGameOwnerDir(t *testing.T, dataDir string) {
	t.Helper()
	if err := ensureNewGameControlDir(dataDir); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(newGameOwnerDir(dataDir), 0o700); err != nil {
		t.Fatal(err)
	}
	old := newGameOwnerProcessStartedAt.Add(-newGameOwnerLegacyClaimMinAge - time.Minute)
	if err := os.Chtimes(newGameOwnerDir(dataDir), old, old); err != nil {
		t.Fatal(err)
	}
}

func assertEmptyOwnerDirectoryForTest(t *testing.T, dataDir string) {
	t.Helper()
	entries, err := os.ReadDir(newGameOwnerDir(dataDir))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("owner directory entries = %v, want empty", entries)
	}
}
