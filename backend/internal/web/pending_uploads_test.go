package web

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func TestImportMutexEndpointCoverage(t *testing.T) {
	blocked := [][]string{
		{"i1", "start"}, {"i1", "stop"}, {"i1", "restart"}, {"i1", "install"},
		{"i1", "saves", "custom-new-game"}, {"i1", "saves", "select"}, {"i1", "saves", "backups", "restore"},
		{"i1", "mods", "upload"}, {"i1", "mods", "enabled"}, {"i1", "junimo-update"}, {"i1", "smapi-update"},
	}
	for _, parts := range blocked {
		if !importMutexEndpoint(http.MethodPost, parts) {
			t.Errorf("not blocked: %v", parts)
		}
	}
	if importMutexEndpoint(http.MethodPost, []string{"i1", "saves", "upload-preview"}) {
		t.Fatal("preview should remain available")
	}
	if importMutexEndpoint(http.MethodPost, []string{"i1", "saves", "upload-commit-and-start"}) {
		t.Fatal("idempotent commit retry should remain available")
	}
	if importMutexEndpoint(http.MethodGet, []string{"i1", "mods"}) {
		t.Fatal("read-only endpoint blocked")
	}
}

func createPendingUpload(t *testing.T, store *durablePendingUploadStore, dataDir string) string {
	t.Helper()
	source := filepath.Join(t.TempDir(), "payload")
	if err := os.MkdirAll(source, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "save"), []byte("bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	token, err := store.put(dataDir, "i1", source, "Save_1", registry.SaveInfo{Name: "Save_1"})
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func TestPendingUploadConcurrentReserve(t *testing.T) {
	store := newDurablePendingUploadStore()
	dir := t.TempDir()
	token := createPendingUpload(t, store, dir)
	var wg sync.WaitGroup
	wg.Add(2)
	successes := 0
	var mu sync.Mutex
	for _, op := range []string{"00112233445566778899aabbccddeeff", "10112233445566778899aabbccddeeff"} {
		go func(operation string) {
			defer wg.Done()
			if _, err := store.reserve(dir, token, "i1", operation); err == nil {
				mu.Lock()
				successes++
				mu.Unlock()
			}
		}(op)
	}
	wg.Wait()
	if successes != 1 {
		t.Fatalf("successful reservations=%d", successes)
	}
}

func TestPendingUploadIdempotentReleaseCancelConsume(t *testing.T) {
	store := newDurablePendingUploadStore()
	dir := t.TempDir()
	token := createPendingUpload(t, store, dir)
	op := "00112233445566778899aabbccddeeff"
	first, err := store.reserve(dir, token, "i1", op)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.reserve(dir, token, "i1", op)
	if err != nil || first.StagedDir != second.StagedDir {
		t.Fatalf("repeat reserve err=%v", err)
	}
	if err := store.cancel(dir, token); err == nil {
		t.Fatal("reserved token was cancelled")
	}
	if err := store.release(dir, token, op); err != nil {
		t.Fatal(err)
	}
	if _, err := store.reserve(dir, token, "i1", op); err != nil {
		t.Fatal(err)
	}
	if err := store.consume(dir, token, op); err != nil {
		t.Fatal(err)
	}
	if err := store.cancel(dir, token); err == nil {
		t.Fatal("consumed token was cancelled")
	}
}

func TestPendingUploadExpiry(t *testing.T) {
	store := newDurablePendingUploadStore()
	now := time.Now()
	store.now = func() time.Time { return now }
	dir := t.TempDir()
	token := createPendingUpload(t, store, dir)
	now = now.Add(uploadTokenTTL + time.Second)
	if _, err := store.reserve(dir, token, "i1", "00112233445566778899aabbccddeeff"); err == nil {
		t.Fatal("expired token reserved")
	}
}

func TestPendingUploadOwnershipTransferAndRestartDiscovery(t *testing.T) {
	store := newDurablePendingUploadStore()
	dataDir := t.TempDir()
	token := createPendingUpload(t, store, dataDir)
	op := "70112233445566778899aabbccddeeff"
	entry, err := store.reserve(dataDir, token, "i1", op)
	if err != nil {
		t.Fatal(err)
	}
	target := transactionSourceDirForUpload(dataDir, op)
	if err := store.transferOwnership(dataDir, token, op, target); err != nil {
		t.Fatal(err)
	}
	if err := store.attachJob(dataDir, token, op, "job-persisted"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(target, "save")); err != nil {
		t.Fatalf("operation source missing: %v", err)
	}
	if _, err := os.Stat(entry.StagedDir); !os.IsNotExist(err) {
		t.Fatalf("token payload still owns source: %v", err)
	}
	restartedStore := newDurablePendingUploadStore()
	recovered, err := restartedStore.reserve(dataDir, token, "i1", op)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Status != "owned" || filepath.Clean(recovered.StagedDir) != filepath.Clean(target) {
		t.Fatalf("recovered=%+v", recovered)
	}
	if recovered.JobID != "job-persisted" {
		t.Fatalf("recovered job ID=%q", recovered.JobID)
	}
	if err := restartedStore.cancel(dataDir, token); err == nil {
		t.Fatal("owned token cancel removed transaction data")
	}
}

func TestPendingUploadRestartRecoversExactJobBindingCrashPoints(t *testing.T) {
	for _, tc := range []struct {
		name          string
		attachToken   bool
		attachJournal bool
	}{
		{name: "job created before journal and token"},
		{name: "journal attached before token", attachJournal: true},
		{name: "token attached before runner release", attachJournal: true, attachToken: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dataDir := t.TempDir()
			uploads := newDurablePendingUploadStore()
			token := createPendingUpload(t, uploads, dataDir)
			op := sj.NewImportOperationID()
			if _, err := uploads.reserve(dataDir, token, "i1", op); err != nil {
				t.Fatal(err)
			}
			req := registry.SaveImportRequest{Instance: registry.Instance{ID: "i1", DataDir: dataDir}, OperationID: op, SaveName: "Save_1", HostHandling: "server_owns_original"}
			journal, err := sj.CreateImportJournal(dataDir, req)
			if err != nil {
				t.Fatal(err)
			}
			if err := uploads.transferOwnership(dataDir, token, op, transactionSourceDirForUpload(dataDir, op)); err != nil {
				t.Fatal(err)
			}
			journal.SourceOwned = true
			if err := sj.WriteImportJournal(dataDir, journal); err != nil {
				t.Fatal(err)
			}

			db, err := storage.Open(context.Background(), config.Config{DataDir: dataDir, DBPath: filepath.Join(dataDir, "panel.db")})
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if err := db.Migrate(context.Background()); err != nil {
				t.Fatal(err)
			}
			payload, _ := json.Marshal(map[string]string{"operationId": op})
			job, err := db.CreateIdempotentJob(context.Background(), storage.CreateJobParams{
				Type: sj.SaveImportJobType, TargetType: "instance", TargetID: "i1",
				IdempotencyKey: sj.SaveImportJobIdempotencyKey(op), Payload: string(payload),
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := db.FailJob(context.Background(), job.ID, "interrupted before runner release"); err != nil {
				t.Fatal(err)
			}
			if tc.attachJournal {
				if err := sj.AttachImportJournalJobIdentity(dataDir, op, "i1", job.ID); err != nil {
					t.Fatal(err)
				}
			}
			if tc.attachToken {
				if err := uploads.attachJob(dataDir, token, op, job.ID); err != nil {
					t.Fatal(err)
				}
			}

			restarted := &server{jobs: jobs.NewManager(db, slog.Default()), pendingUploads: newDurablePendingUploadStore()}
			entry, err := restarted.pendingUploads.lookup(dataDir, token, "i1")
			if err != nil {
				t.Fatal(err)
			}
			recovered, err := restarted.reconcilePendingImportJobIdentity(context.Background(), storage.Instance{ID: "i1", DataDir: dataDir}, token, entry)
			if err != nil || recovered.ID != job.ID {
				t.Fatalf("recovered=%+v err=%v", recovered, err)
			}
			if err := restarted.verifyPendingImportJobBinding(storage.Instance{ID: "i1", DataDir: dataDir}, token, op, job.ID); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPendingUploadRecoveryRejectsMissingOrMismatchedExactJob(t *testing.T) {
	for _, mismatch := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing", true: "payload mismatch"}[mismatch], func(t *testing.T) {
			dataDir := t.TempDir()
			uploads := newDurablePendingUploadStore()
			token := createPendingUpload(t, uploads, dataDir)
			op := sj.NewImportOperationID()
			if _, err := uploads.reserve(dataDir, token, "i1", op); err != nil {
				t.Fatal(err)
			}
			journal, err := sj.CreateImportJournal(dataDir, registry.SaveImportRequest{Instance: registry.Instance{ID: "i1", DataDir: dataDir}, OperationID: op, SaveName: "Save_1", HostHandling: "server_owns_original"})
			if err != nil {
				t.Fatal(err)
			}
			if err := uploads.transferOwnership(dataDir, token, op, transactionSourceDirForUpload(dataDir, op)); err != nil {
				t.Fatal(err)
			}
			journal.SourceOwned = true
			if err := sj.WriteImportJournal(dataDir, journal); err != nil {
				t.Fatal(err)
			}
			db, err := storage.Open(context.Background(), config.Config{DataDir: dataDir, DBPath: filepath.Join(dataDir, "panel.db")})
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if err := db.Migrate(context.Background()); err != nil {
				t.Fatal(err)
			}
			if mismatch {
				payload, _ := json.Marshal(map[string]string{"operationId": sj.NewImportOperationID()})
				if _, err := db.CreateIdempotentJob(context.Background(), storage.CreateJobParams{Type: sj.SaveImportJobType, TargetType: "instance", TargetID: "i1", IdempotencyKey: sj.SaveImportJobIdempotencyKey(op), Payload: string(payload)}); err != nil {
					t.Fatal(err)
				}
			}
			srv := &server{jobs: jobs.NewManager(db, slog.Default()), pendingUploads: uploads}
			entry, err := uploads.lookup(dataDir, token, "i1")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := srv.reconcilePendingImportJobIdentity(context.Background(), storage.Instance{ID: "i1", DataDir: dataDir}, token, entry); err == nil {
				t.Fatal("missing or mismatched exact job was accepted")
			}
		})
	}
}

func completedCleanupReceiptFixture(t *testing.T) (*durablePendingUploadStore, *server, storage.Instance, string) {
	t.Helper()
	dataDir := t.TempDir()
	uploads := newDurablePendingUploadStore()
	token := createPendingUpload(t, uploads, dataDir)
	op := sj.NewImportOperationID()
	if _, err := uploads.reserve(dataDir, token, "i1", op); err != nil {
		t.Fatal(err)
	}
	if err := uploads.transferOwnership(dataDir, token, op, transactionSourceDirForUpload(dataDir, op)); err != nil {
		t.Fatal(err)
	}
	jobID := "job_00112233445566778899aabbccddeeff"
	if err := uploads.attachJob(dataDir, token, op, jobID); err != nil {
		t.Fatal(err)
	}
	if err := uploads.markCleanupCompleted(dataDir, token, op, jobID); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(durableUploadCleanupReceiptPath(dataDir, token)); err != nil {
		t.Fatal(err)
	} else if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("cleanup receipt mode=%v", info.Mode().Perm())
	}
	return uploads, &server{pendingUploads: uploads}, storage.Instance{ID: "i1", DataDir: dataDir}, token
}

func TestPendingUploadCleanupReceiptRetriesTokenDeletionAfterJournalRemoval(t *testing.T) {
	uploads, srv, instance, token := completedCleanupReceiptFixture(t)
	removeCalls := 0
	uploads.removeAll = func(path string) error {
		removeCalls++
		if removeCalls == 1 {
			return errors.New("injected token deletion failure")
		}
		return os.RemoveAll(path)
	}
	if err := srv.cancelPendingSaveUpload(context.Background(), instance, token); err == nil {
		t.Fatal("injected token deletion failure was hidden")
	}
	if _, err := os.Stat(durableUploadDir(instance.DataDir, token)); err != nil {
		t.Fatalf("failed deletion lost exact token record: %v", err)
	}
	if err := srv.cancelPendingSaveUpload(context.Background(), instance, token); err != nil {
		t.Fatal(err)
	}
	if err := srv.cancelPendingSaveUpload(context.Background(), instance, token); err != nil {
		t.Fatalf("idempotent receipt retry failed: %v", err)
	}
	if removeCalls != 2 {
		t.Fatalf("token removal calls=%d", removeCalls)
	}
}

func TestPendingUploadCleanupReceiptConvergesWithoutRawToken(t *testing.T) {
	uploads, _, instance, token := completedCleanupReceiptFixture(t)
	references, err := uploads.cleanupReferences(instance.DataDir, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(references) != 1 {
		t.Fatalf("cleanup references=%d", len(references))
	}
	reference := references[0]
	if reference.Upload.TokenHash != pendingUploadHash(token) {
		t.Fatalf("cleanup token hash=%q", reference.Upload.TokenHash)
	}
	if err := sj.FinalizeCanceledImportCleanup(instance.DataDir, reference.Receipt.OperationID); err != nil {
		t.Fatal(err)
	}
	if err := uploads.removeOwnedAfterCleanupByReference(instance.DataDir, reference.Upload, reference.Receipt); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(durableUploadDir(instance.DataDir, token)); !os.IsNotExist(err) {
		t.Fatalf("raw-tokenless cleanup left token directory: %v", err)
	}
	references, err = uploads.cleanupReferences(instance.DataDir, instance.ID)
	if err != nil || len(references) != 0 {
		t.Fatalf("cleanup references after convergence=%d err=%v", len(references), err)
	}
}

func TestPendingUploadConcurrentCancelUsesCleanupReceiptOnce(t *testing.T) {
	uploads, srv, instance, token := completedCleanupReceiptFixture(t)
	removeCalls := 0
	var callsMu sync.Mutex
	uploads.removeAll = func(path string) error {
		callsMu.Lock()
		removeCalls++
		callsMu.Unlock()
		return os.RemoveAll(path)
	}
	start := make(chan struct{})
	errs := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			errs <- srv.cancelPendingSaveUpload(context.Background(), instance, token)
		}()
	}
	close(start)
	for range 2 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	callsMu.Lock()
	defer callsMu.Unlock()
	if removeCalls != 1 {
		t.Fatalf("dangerous token removal calls=%d", removeCalls)
	}
}

func TestPendingUploadSucceededMetadataExpiresWithoutDeletingTransactionSource(t *testing.T) {
	store := newDurablePendingUploadStore()
	now := time.Now()
	store.now = func() time.Time { return now }
	dataDir := t.TempDir()
	token := createPendingUpload(t, store, dataDir)
	op := "71112233445566778899aabbccddeeff"
	if _, err := store.reserve(dataDir, token, "i1", op); err != nil {
		t.Fatal(err)
	}
	target := transactionSourceDirForUpload(dataDir, op)
	if err := store.transferOwnership(dataDir, token, op, target); err != nil {
		t.Fatal(err)
	}
	if err := store.attachJob(dataDir, token, op, "job-after-fast-success"); err != nil {
		t.Fatal(err)
	}
	if err := store.markSucceeded(dataDir, token, op); err != nil {
		t.Fatal(err)
	}
	now = now.Add(uploadTokenTTL + time.Second)
	_ = createPendingUpload(t, store, dataDir)
	compacted, err := readDurablePendingUpload(dataDir, token)
	if err != nil || !compacted.MetadataCompacted || compacted.Status != "succeeded" || compacted.JobID != "job-after-fast-success" || compacted.StagedDir != "" {
		t.Fatalf("succeeded token result tombstone=%+v err=%v", compacted, err)
	}
	if _, err := os.Stat(filepath.Join(target, "save")); err != nil {
		t.Fatalf("token metadata prune removed transaction-owned source: %v", err)
	}
}

func TestPendingUploadSucceededTombstonePreservesExactResultAndCompletedArtifacts(t *testing.T) {
	now := time.Now()
	dataDir := t.TempDir()
	uploads := newDurablePendingUploadStore()
	uploads.now = func() time.Time { return now }
	token := createPendingUpload(t, uploads, dataDir)
	op := sj.NewImportOperationID()
	if _, err := uploads.reserve(dataDir, token, "i1", op); err != nil {
		t.Fatal(err)
	}
	journal, err := sj.CreateImportJournal(dataDir, registry.SaveImportRequest{Instance: registry.Instance{ID: "i1", DataDir: dataDir}, OperationID: op, SaveName: "Save_1", HostHandling: "server_owns_original"})
	if err != nil {
		t.Fatal(err)
	}
	if err := uploads.transferOwnership(dataDir, token, op, transactionSourceDirForUpload(dataDir, op)); err != nil {
		t.Fatal(err)
	}
	journal.SourceOwned = true
	if err := sj.WriteImportJournal(dataDir, journal); err != nil {
		t.Fatal(err)
	}
	db, err := storage.Open(context.Background(), config.Config{DataDir: dataDir, DBPath: filepath.Join(dataDir, "panel.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(map[string]string{"operationId": op})
	job, err := db.CreateIdempotentJob(context.Background(), storage.CreateJobParams{Type: sj.SaveImportJobType, TargetType: "instance", TargetID: "i1", IdempotencyKey: sj.SaveImportJobIdempotencyKey(op), Payload: string(payload)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.FinishJob(context.Background(), job.ID); err != nil {
		t.Fatal(err)
	}
	if err := sj.AttachImportJournalJobIdentity(dataDir, op, "i1", job.ID); err != nil {
		t.Fatal(err)
	}
	if err := uploads.attachJob(dataDir, token, op, job.ID); err != nil {
		t.Fatal(err)
	}
	if err := sj.ConfirmImportJournalJobBinding(dataDir, op, "i1", job.ID); err != nil {
		t.Fatal(err)
	}
	journal, err = sj.LoadImportJournal(dataDir, op)
	if err != nil {
		t.Fatal(err)
	}
	journal.Stage = sj.ImportStageCompleted
	if err := sj.WriteImportJournal(dataDir, journal); err != nil {
		t.Fatal(err)
	}
	preimport := filepath.Join(dataDir, ".local-container", "backups", "saves", "preimport_fixture.zip")
	formal := filepath.Join(dataDir, ".local-container", "saves", "Saves", "Save_1", "Save_1")
	if err := os.MkdirAll(filepath.Dir(preimport), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(formal), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(preimport, []byte("backup"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(formal, []byte("formal"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := uploads.markSucceeded(dataDir, token, op); err != nil {
		t.Fatal(err)
	}
	now = now.Add(uploadTokenTTL + time.Second)
	_ = createPendingUpload(t, uploads, dataDir)
	compacted, err := uploads.lookup(dataDir, token, "i1")
	if err != nil || !compacted.MetadataCompacted {
		t.Fatalf("compacted=%+v err=%v", compacted, err)
	}
	srv := &server{jobs: jobs.NewManager(db, slog.Default()), pendingUploads: uploads}
	recovered, err := srv.reconcilePendingImportJobIdentity(context.Background(), storage.Instance{ID: "i1", DataDir: dataDir}, token, compacted)
	if err != nil || recovered.ID != job.ID || recovered.Status != storage.JobStatusSucceeded {
		t.Fatalf("recovered=%+v err=%v", recovered, err)
	}
	for _, path := range []string{preimport, formal, filepath.Join(dataDir, ".local-container", "control", "save-import-transactions", op, "journal.json")} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("succeeded metadata compaction removed %s: %v", path, err)
		}
	}
}

func TestPendingUploadReserveOrReuseAndCancelBeforeOwnership(t *testing.T) {
	store := newDurablePendingUploadStore()
	dataDir := t.TempDir()
	token := createPendingUpload(t, store, dataDir)
	first, err := store.reserveOrReuse(dataDir, token, "i1", "00112233445566778899aabbccddeeff")
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.reserveOrReuse(dataDir, token, "i1", "ffffffffffffffffffffffffffffffff")
	if err != nil || second.OperationID != first.OperationID {
		t.Fatalf("retry operation=%q want=%q err=%v", second.OperationID, first.OperationID, err)
	}
	if err := store.release(dataDir, token, first.OperationID); err != nil {
		t.Fatal(err)
	}
	if err := store.cancel(dataDir, token); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(durableUploadDir(dataDir, token)); !os.IsNotExist(err) {
		t.Fatalf("cancel-before-ownership left token data: %v", err)
	}
}

type capturingImportOwnershipDriver struct {
	registry.GameDriver
	sourceDir string
	request   registry.SaveImportRequest
	calls     int
	store     *storage.Store
}

func (d *capturingImportOwnershipDriver) ID() string   { return sj.DriverID }
func (d *capturingImportOwnershipDriver) Name() string { return "import ownership test" }
func (d *capturingImportOwnershipDriver) ImportSaveAndStart(_ context.Context, req registry.SaveImportRequest) (*registry.Job, error) {
	d.calls++
	d.request = req
	if _, err := sj.CreateImportJournal(req.Instance.DataDir, req); err != nil {
		return nil, err
	}
	target := filepath.Join(req.Instance.DataDir, ".local-container", "control", "save-import-transactions", req.OperationID, "source")
	if err := req.TransferSourceOwnership(target); err != nil {
		return nil, err
	}
	j, err := sj.LoadImportJournal(req.Instance.DataDir, req.OperationID)
	if err != nil {
		return nil, err
	}
	j.SourceOwned = true
	if err := sj.WriteImportJournal(req.Instance.DataDir, j); err != nil {
		return nil, err
	}
	payload, _ := json.Marshal(map[string]string{"operationId": req.OperationID})
	durableJob, err := d.store.CreateIdempotentJob(context.Background(), storage.CreateJobParams{
		Type: sj.SaveImportJobType, TargetType: "instance", TargetID: req.Instance.ID,
		IdempotencyKey: sj.SaveImportJobIdempotencyKey(req.OperationID), Payload: string(payload),
	})
	if err != nil {
		return nil, err
	}
	if err := sj.AttachImportJournalJobIdentity(req.Instance.DataDir, req.OperationID, req.Instance.ID, durableJob.ID); err != nil {
		return nil, err
	}
	if req.AttachJobIdentity == nil {
		return nil, errors.New("job identity callback missing")
	}
	if err := req.AttachJobIdentity(durableJob.ID); err != nil {
		return nil, &sj.ImportTransactionError{Code: sj.ImportErrorRecoveryRequired, Message: "job identity attachment failed", Cause: err}
	}
	if err := sj.ConfirmImportJournalJobBinding(req.Instance.DataDir, req.OperationID, req.Instance.ID, durableJob.ID); err != nil {
		return nil, err
	}
	d.sourceDir = target
	return &registry.Job{ID: durableJob.ID}, nil
}

func TestPendingUploadHandlerReturnKeepsTransactionSource(t *testing.T) {
	dataDir := t.TempDir()
	store, err := storage.Open(context.Background(), config.Config{Addr: ":0", DataDir: dataDir, DBPath: filepath.Join(dataDir, "panel.db"), Secret: "test-secret", Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	driver := &capturingImportOwnershipDriver{store: store}
	drivers := registry.New()
	if err := drivers.Register(driver); err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(Deps{Config: config.Config{DataDir: dataDir, Secret: "test-secret", Version: "test"}, Store: store, Registry: drivers, Logger: slog.Default()})
	setup, adminCookie := doJSON(t, handler, http.MethodPost, "/api/setup/admin", map[string]string{"username": "admin", "password": "admin-password", "confirmPassword": "admin-password"}, nil)
	if setup.Code != http.StatusOK {
		t.Fatalf("setup=%d: %s", setup.Code, setup.Body.String())
	}
	if _, err := store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID: storage.DefaultInstanceID, State: storage.InstanceStateGameInstalled, StateMessage: "game installed",
		DriverPhase: "game_installed", DriverPayload: `{"install":"complete"}`,
	}); err != nil {
		t.Fatal(err)
	}

	var zipBytes bytes.Buffer
	zw := zip.NewWriter(&zipBytes)
	for name, content := range map[string]string{
		"Imported_123/Imported_123": "uploaded-main",
		"Imported_123/SaveGameInfo": "uploaded-info",
	} {
		writer, createErr := zw.Create(name)
		if createErr != nil {
			t.Fatal(createErr)
		}
		_, _ = writer.Write([]byte(content))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	var form bytes.Buffer
	mw := multipart.NewWriter(&form)
	part, err := mw.CreateFormFile("save", "import.zip")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write(zipBytes.Bytes())
	_ = mw.Close()
	previewReq := httptest.NewRequest(http.MethodPost, "/api/instances/stardew/saves/upload-preview", &form)
	previewReq.Header.Set("Content-Type", mw.FormDataContentType())
	previewReq.AddCookie(adminCookie)
	previewResp := httptest.NewRecorder()
	handler.ServeHTTP(previewResp, previewReq)
	if previewResp.Code != http.StatusOK {
		t.Fatalf("preview=%d: %s", previewResp.Code, previewResp.Body.String())
	}
	var preview struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(previewResp.Body.Bytes(), &preview); err != nil || preview.Token == "" {
		t.Fatalf("preview token err=%v body=%s", err, previewResp.Body.String())
	}
	platformID := "18446744073709551615"
	commitBody := map[string]any{"token": preview.Token, "hostHandling": map[string]any{"mode": hostModeSwapToPlayer, "platformId": platformID}}
	commit, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/saves/upload-commit-and-start", commitBody, adminCookie)
	if commit.Code != http.StatusAccepted {
		t.Fatalf("commit=%d: %s", commit.Code, commit.Body.String())
	}
	if _, err := os.Stat(filepath.Join(driver.sourceDir, "Imported_123", "Imported_123")); err != nil {
		t.Fatalf("transaction source missing after handler return: %v", err)
	}
	var accepted saveUploadCommitResponse
	if err := json.Unmarshal(commit.Body.Bytes(), &accepted); err != nil || accepted.JobID == "" || len(accepted.OperationID) != 32 || accepted.SaveName != "Imported_123" {
		t.Fatalf("invalid 202 response err=%v response=%+v", err, accepted)
	}
	if driver.request.HostHandling != "swap_host_to" || driver.request.PlatformID != platformID {
		t.Fatalf("driver request host handling was not mapped: %+v", driver.request)
	}
	if driver.request.Instance.State != storage.InstanceStateGameInstalled || driver.request.Instance.DriverPhase != "game_installed" {
		t.Fatalf("first-install API request lost its real state: %+v", driver.request.Instance)
	}
	retry, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/saves/upload-commit-and-start", commitBody, adminCookie)
	if retry.Code != http.StatusAccepted || retry.Body.String() != commit.Body.String() || driver.calls != 1 {
		t.Fatalf("idempotent retry code=%d calls=%d first=%s retry=%s", retry.Code, driver.calls, commit.Body.String(), retry.Body.String())
	}
	logs, _, err := store.ListAuditLogs(context.Background(), storage.ListAuditLogsParams{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	foundAudit := false
	for _, entry := range logs {
		if entry.Action != "save_import_submit" {
			continue
		}
		foundAudit = true
		if strings.Contains(entry.MetadataJSON, platformID) || !strings.Contains(entry.MetadataJSON, hostModeSwapToPlayer) ||
			!strings.Contains(entry.MetadataJSON, accepted.OperationID) || !strings.Contains(entry.MetadataJSON, accepted.JobID) {
			t.Fatalf("unsafe or incomplete audit metadata: %s", entry.MetadataJSON)
		}
	}
	if !foundAudit {
		t.Fatal("save import audit entry missing")
	}
	journalBytes, err := os.ReadFile(filepath.Join(filepath.Dir(driver.sourceDir), "journal.json"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(journalBytes, []byte(platformID)) {
		t.Fatal("raw platform ID was persisted in import journal")
	}
	cancel, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/saves/upload-commit-and-start", map[string]any{"token": preview.Token, "cancel": true}, adminCookie)
	if cancel.Code != http.StatusConflict {
		t.Fatalf("cancel=%d: %s", cancel.Code, cancel.Body.String())
	}
	if _, err := os.Stat(driver.sourceDir); err != nil {
		t.Fatalf("owned transaction source was removed by token cancel: %v", err)
	}
}

func TestPendingUploadAttachJobFailureDoesNotReturnAccepted(t *testing.T) {
	root := t.TempDir()
	store, err := storage.Open(context.Background(), config.Config{Addr: ":0", DataDir: root, DBPath: filepath.Join(root, "panel.db"), Secret: "test-secret", Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	pending := newDurablePendingUploadStore()
	driver := &capturingImportOwnershipDriver{store: store}
	drivers := registry.New()
	if err := drivers.Register(driver); err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(Deps{
		Config: config.Config{DataDir: root, Secret: "test-secret", Version: "test"}, Store: store,
		Registry: drivers, Logger: slog.Default(), pendingUploads: pending,
	})
	setup, adminCookie := doJSON(t, handler, http.MethodPost, "/api/setup/admin", map[string]string{"username": "admin", "password": "admin-password", "confirmPassword": "admin-password"}, nil)
	if setup.Code != http.StatusOK {
		t.Fatalf("setup=%d: %s", setup.Code, setup.Body.String())
	}
	instance, err := store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID: storage.DefaultInstanceID, State: storage.InstanceStateGameInstalled, StateMessage: "game installed",
		DriverPhase: "game_installed", DriverPayload: `{}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "payload")
	if err := os.MkdirAll(source, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "save"), []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	token, err := pending.put(instance.DataDir, instance.ID, source, "Save_1", registry.SaveInfo{Name: "Save_1"})
	if err != nil {
		t.Fatal(err)
	}
	pending.write = func(dataDir, token string, entry *durablePendingUpload) error {
		if entry.JobID != "" {
			return errors.New("injected token job attachment failure")
		}
		return writeDurablePendingUpload(dataDir, token, entry)
	}
	commit, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/saves/upload-commit-and-start", map[string]any{
		"token": token, "hostHandling": map[string]any{"mode": hostModeVirtualHostTakeover, "acknowledged": true},
	}, adminCookie)
	if commit.Code != http.StatusConflict || !strings.Contains(commit.Body.String(), sj.ImportErrorRecoveryRequired) {
		t.Fatalf("attach failure response=%d: %s", commit.Code, commit.Body.String())
	}
	owned, err := readDurablePendingUpload(instance.DataDir, token)
	if err != nil || owned.Status != "owned" || owned.JobID != "" || owned.OperationID == "" {
		t.Fatalf("owned token=%+v err=%v", owned, err)
	}
	journal, err := sj.LoadImportJournal(instance.DataDir, owned.OperationID)
	if err != nil || journal.JobBindingState != "journal_attached" || journal.JobID == "" || journal.Stage != sj.ImportStageValidated {
		t.Fatalf("journal=%+v err=%v", journal, err)
	}
	if _, err := os.Stat(transactionSourceDirForUpload(instance.DataDir, owned.OperationID)); err != nil {
		t.Fatalf("transaction ownership evidence missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(instance.DataDir, ".local-container", "saves", "Saves", "Save_1")); !os.IsNotExist(err) {
		t.Fatalf("runner began staging after attachment failure: %v", err)
	}
}

func TestPendingUploadCommitRejectsFreshRunningServerBeforeOwnership(t *testing.T) {
	root := t.TempDir()
	store, err := storage.Open(context.Background(), config.Config{Addr: ":0", DataDir: root, DBPath: filepath.Join(root, "panel.db"), Secret: "test-secret", Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	fake := fakeDockerService{
		psResult:       paneldocker.ComposePsResult{Services: []paneldocker.ComposeService{{Service: "server", State: "exited", Status: "Exited (0)"}}},
		strictPsResult: paneldocker.ComposePsResult{Services: []paneldocker.ComposeService{{Service: "server", State: "running", Status: "Up 1 second"}}},
	}
	manager := jobs.NewManager(store, slog.Default())
	driver := sj.New(fake, slog.Default(), manager, store)
	drivers := registry.New()
	if err := drivers.Register(driver); err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(Deps{
		Config: config.Config{DataDir: root, Secret: "test-secret", Version: "test"}, Store: store,
		Registry: drivers, Docker: fake, Jobs: manager, Logger: slog.Default(),
	})
	setup, adminCookie := doJSON(t, handler, http.MethodPost, "/api/setup/admin", map[string]string{
		"username": "admin", "password": "admin-password", "confirmPassword": "admin-password",
	}, nil)
	if setup.Code != http.StatusOK {
		t.Fatalf("setup=%d: %s", setup.Code, setup.Body.String())
	}
	stored, err := store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID: storage.DefaultInstanceID, State: storage.InstanceStateGameInstalled, StateMessage: "installed",
		DriverPhase: "game_installed", DriverPayload: `{}`,
	})
	if err != nil {
		t.Fatal(err)
	}

	var zipBytes bytes.Buffer
	zw := zip.NewWriter(&zipBytes)
	for name, content := range map[string]string{
		"Upload_1/Upload_1":     `<SaveGame><player><name>Imported</name></player></SaveGame>`,
		"Upload_1/SaveGameInfo": `<Farmer><name>Imported</name></Farmer>`,
	} {
		writer, createErr := zw.Create(name)
		if createErr != nil {
			t.Fatal(createErr)
		}
		_, _ = writer.Write([]byte(content))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	var form bytes.Buffer
	mw := multipart.NewWriter(&form)
	part, err := mw.CreateFormFile("save", "running.zip")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write(zipBytes.Bytes())
	_ = mw.Close()
	previewReq := httptest.NewRequest(http.MethodPost, "/api/instances/stardew/saves/upload-preview", &form)
	previewReq.Header.Set("Content-Type", mw.FormDataContentType())
	previewReq.AddCookie(adminCookie)
	previewResp := httptest.NewRecorder()
	handler.ServeHTTP(previewResp, previewReq)
	if previewResp.Code != http.StatusOK {
		t.Fatalf("preview=%d: %s", previewResp.Code, previewResp.Body.String())
	}
	var preview struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(previewResp.Body.Bytes(), &preview); err != nil || preview.Token == "" {
		t.Fatalf("preview token err=%v body=%s", err, previewResp.Body.String())
	}
	commit, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/saves/upload-commit-and-start", map[string]any{
		"token": preview.Token, "hostHandling": map[string]any{"mode": hostModeVirtualHostTakeover, "acknowledged": true},
	}, adminCookie)
	if commit.Code != http.StatusConflict || !strings.Contains(commit.Body.String(), sj.ImportErrorSaveInProgress) {
		t.Fatalf("commit=%d: %s", commit.Code, commit.Body.String())
	}
	entry, err := newDurablePendingUploadStore().lookup(stored.DataDir, preview.Token, stored.ID)
	if err != nil || entry.Status != "available" {
		t.Fatalf("token entry=%+v err=%v", entry, err)
	}
	if _, err := os.Stat(entry.StagedDir); err != nil {
		t.Fatalf("strict rejection transferred or removed staged source: %v", err)
	}
	unfinished, err := sj.HasUnfinishedImportTransaction(stored.DataDir)
	if err != nil || unfinished {
		t.Fatalf("strict rejection created import journal: unfinished=%v err=%v", unfinished, err)
	}
}

func TestFailedFirstInstallImportCanSafelyCancelAndAutoRecoverOwnedTransaction(t *testing.T) {
	root := t.TempDir()
	store, err := storage.Open(context.Background(), config.Config{Addr: ":0", DataDir: root, DBPath: filepath.Join(root, "panel.db"), Secret: "test-secret", Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	instanceDir := filepath.Join(root, "instances", "stardew")
	if err := os.MkdirAll(instanceDir, 0o700); err != nil {
		t.Fatal(err)
	}
	stored, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{
		ID: storage.DefaultInstanceID, DriverID: sj.DriverID, Name: "Stardew Valley", DataDir: instanceDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	stored, err = store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID: stored.ID, State: storage.InstanceStateGameInstalled, StateMessage: "game installed exactly",
		DriverPhase: "game_installed", DriverPayload: `{"install":"complete","kept":true}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(instanceDir, ".env"), []byte("IMAGE_VERSION="+sj.TestedImageTag+"\nAPI_PORT=5110\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	modDir := filepath.Join(instanceDir, ".local-container", "mods", "JunimoServer")
	if err := os.MkdirAll(modDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "manifest.json"), []byte(`{"Version":"`+sj.TestedImageTag+`","UniqueID":"JunimoHost.Server"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "JunimoServer.dll"), []byte("dll"), 0o600); err != nil {
		t.Fatal(err)
	}

	fake := fakeDockerService{composeUp: paneldocker.CommandResult{ExitCode: 1}, composeUpErr: errors.New("injected maintenance startup failure")}
	manager := jobs.NewManager(store, slog.Default())
	driver := sj.New(fake, slog.Default(), manager, store)
	drivers := registry.New()
	if err := drivers.Register(driver); err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(Deps{
		Config: config.Config{DataDir: root, Secret: "test-secret", Version: "test"}, Store: store,
		Registry: drivers, Docker: fake, Jobs: manager, Logger: slog.Default(),
	})
	setup, adminCookie := doJSON(t, handler, http.MethodPost, "/api/setup/admin", map[string]string{
		"username": "admin", "password": "admin-password", "confirmPassword": "admin-password",
	}, nil)
	if setup.Code != http.StatusOK {
		t.Fatalf("setup=%d: %s", setup.Code, setup.Body.String())
	}
	stored, err = store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID: stored.ID, State: storage.InstanceStateGameInstalled, StateMessage: "game installed exactly",
		DriverPhase: "game_installed", DriverPayload: `{"install":"complete","kept":true}`,
	})
	if err != nil {
		t.Fatal(err)
	}

	var zipBytes bytes.Buffer
	zw := zip.NewWriter(&zipBytes)
	for name, content := range map[string]string{
		"Upload_1/Upload_1":     `<SaveGame><player><name>Imported</name></player></SaveGame>`,
		"Upload_1/SaveGameInfo": `<Farmer><name>Imported</name></Farmer>`,
	} {
		writer, createErr := zw.Create(name)
		if createErr != nil {
			t.Fatal(createErr)
		}
		_, _ = writer.Write([]byte(content))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	uploadPreview := func(filename string) string {
		t.Helper()
		var form bytes.Buffer
		mw := multipart.NewWriter(&form)
		part, createErr := mw.CreateFormFile("save", filename)
		if createErr != nil {
			t.Fatal(createErr)
		}
		_, _ = part.Write(zipBytes.Bytes())
		_ = mw.Close()
		previewReq := httptest.NewRequest(http.MethodPost, "/api/instances/stardew/saves/upload-preview", &form)
		previewReq.Header.Set("Content-Type", mw.FormDataContentType())
		previewReq.AddCookie(adminCookie)
		previewResp := httptest.NewRecorder()
		handler.ServeHTTP(previewResp, previewReq)
		if previewResp.Code != http.StatusOK {
			t.Fatalf("preview=%d: %s", previewResp.Code, previewResp.Body.String())
		}
		var preview struct {
			Token string `json:"token"`
		}
		if err := json.Unmarshal(previewResp.Body.Bytes(), &preview); err != nil || preview.Token == "" {
			t.Fatalf("preview token err=%v body=%s", err, previewResp.Body.String())
		}
		return preview.Token
	}
	previewToken := uploadPreview("first-upload.zip")
	commit, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/saves/upload-commit-and-start", map[string]any{
		"token": previewToken, "hostHandling": map[string]any{"mode": hostModeVirtualHostTakeover, "acknowledged": true},
	}, adminCookie)
	if commit.Code != http.StatusAccepted {
		t.Fatalf("commit=%d: %s", commit.Code, commit.Body.String())
	}
	var accepted saveUploadCommitResponse
	if err := json.Unmarshal(commit.Body.Bytes(), &accepted); err != nil {
		t.Fatal(err)
	}
	waitForHTTPJobStatus(t, handler, adminCookie, accepted.JobID, storage.JobStatusFailed)

	journal := waitForImportJournal(t, instanceDir, accepted.OperationID, func(current sj.ImportJournal) bool {
		return current.Stage == sj.ImportStageBackupCreated && current.StagedSaveCreated && current.BootstrapSaveCreated
	})
	if journal.MaintenanceStarted || journal.UpstreamSubmitted || journal.UpstreamConfirmed {
		t.Fatalf("failed pre-submit journal=%+v", journal)
	}
	preimportPath := filepath.Join(instanceDir, ".local-container", "backups", "saves", journal.PreimportBackupName)
	if _, err := os.Stat(preimportPath); err != nil {
		t.Fatalf("preimport backup missing before cleanup: %v", err)
	}
	failedState, err := store.GetInstance(context.Background(), stored.ID)
	if err != nil || failedState.State != storage.InstanceStateGameInstalled || failedState.DriverPhase != "game_installed" ||
		failedState.StateMessage.String != "game installed exactly" || failedState.DriverPayload != `{"install":"complete","kept":true}` {
		t.Fatalf("game_installed snapshot was not restored: %+v err=%v", failedState, err)
	}
	// Simulate two process interruption windows at once: the job row is durable
	// but its ID was not attached to the token, and driver cleanup completed but
	// token removal/finalization did not. The retry must recover the job through
	// its idempotency key and finish the canceled marker without becoming busy.
	owned, err := readDurablePendingUpload(instanceDir, previewToken)
	if err != nil {
		t.Fatal(err)
	}
	owned.JobID = ""
	owned.JobType = ""
	owned.JobIdempotencyKey = ""
	if err := writeDurablePendingUpload(instanceDir, previewToken, owned); err != nil {
		t.Fatal(err)
	}
	if err := driver.CleanupUnsubmittedSaveImport(context.Background(), makeRegistryInstance(failedState), accepted.OperationID); err != nil {
		t.Fatal(err)
	}
	canceledMarker, err := sj.LoadImportJournal(instanceDir, accepted.OperationID)
	if err != nil || canceledMarker.Stage != sj.ImportStageCanceled {
		t.Fatalf("durable canceled marker=%+v err=%v", canceledMarker, err)
	}
	if _, err := os.Stat(durableUploadDir(instanceDir, previewToken)); err != nil {
		t.Fatalf("interrupted cleanup lost owned token before Web finalization: %v", err)
	}

	cancel, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/saves/upload-commit-and-start", map[string]any{
		"token": previewToken, "cancel": true,
	}, adminCookie)
	if cancel.Code != http.StatusOK {
		t.Fatalf("cancel=%d: %s", cancel.Code, cancel.Body.String())
	}
	if _, err := sj.LoadImportJournal(instanceDir, accepted.OperationID); !os.IsNotExist(err) {
		t.Fatalf("journal survived safe cancel: %v", err)
	}
	if _, err := os.Stat(filepath.Join(instanceDir, ".local-container", "saves", "Saves", "Upload_1")); !os.IsNotExist(err) {
		t.Fatalf("staged target survived safe cancel: %v", err)
	}
	if _, err := os.Stat(filepath.Join(instanceDir, ".local-container", "saves", "Saves", journal.BootstrapSaveName)); !os.IsNotExist(err) {
		t.Fatalf("bootstrap survived safe cancel: %v", err)
	}
	if _, err := os.Stat(filepath.Join(instanceDir, ".local-container", "saves", ".smapi", "mod-data", "junimohost.server", "junimohost.gameloader.json")); !os.IsNotExist(err) {
		t.Fatalf("bootstrap pointer survived safe cancel: %v", err)
	}
	if _, err := os.Stat(durableUploadDir(instanceDir, previewToken)); !os.IsNotExist(err) {
		t.Fatalf("owned token survived safe cancel: %v", err)
	}
	if _, err := os.Stat(preimportPath); err != nil {
		t.Fatalf("safe cancel removed preimport backup: %v", err)
	}
	busy, err := sj.HasUnfinishedImportTransaction(instanceDir)
	if err != nil || busy {
		t.Fatalf("safe cancel left save_import_busy=%v err=%v", busy, err)
	}

	secondToken := uploadPreview("second-upload.zip")
	secondCommit, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/saves/upload-commit-and-start", map[string]any{
		"token": secondToken, "hostHandling": map[string]any{"mode": hostModeVirtualHostTakeover, "acknowledged": true},
	}, adminCookie)
	if secondCommit.Code != http.StatusAccepted {
		t.Fatalf("second commit=%d: %s", secondCommit.Code, secondCommit.Body.String())
	}
	var secondAccepted saveUploadCommitResponse
	if err := json.Unmarshal(secondCommit.Body.Bytes(), &secondAccepted); err != nil {
		t.Fatal(err)
	}
	waitForHTTPJobStatus(t, handler, adminCookie, secondAccepted.JobID, storage.JobStatusFailed)
	secondJournal, err := sj.LoadImportJournal(instanceDir, secondAccepted.OperationID)
	if err != nil || (secondJournal.Stage != sj.ImportStageStaged && secondJournal.Stage != sj.ImportStageBackupCreated) ||
		secondJournal.UpstreamSubmitted || secondJournal.UpstreamConfirmed {
		t.Fatalf("second failed pre-submit journal=%+v err=%v", secondJournal, err)
	}
	secondPreimportPath := ""
	if secondJournal.PreimportBackupName != "" {
		secondPreimportPath = filepath.Join(instanceDir, ".local-container", "backups", "saves", secondJournal.PreimportBackupName)
		if _, err := os.Stat(secondPreimportPath); err != nil {
			t.Fatalf("second preimport backup missing before auto recovery: %v", err)
		}
	}
	unsafeJournal := secondJournal
	unsafeJournal.PhaseAFIFOWriteAttempted = true
	if err := sj.WriteImportJournal(instanceDir, unsafeJournal); err != nil {
		t.Fatal(err)
	}
	var blockedForm bytes.Buffer
	blockedWriter := multipart.NewWriter(&blockedForm)
	blockedPart, err := blockedWriter.CreateFormFile("save", "blocked-upload.zip")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = blockedPart.Write(zipBytes.Bytes())
	_ = blockedWriter.Close()
	blockedReq := httptest.NewRequest(http.MethodPost, "/api/instances/stardew/saves/upload-preview", &blockedForm)
	blockedReq.Header.Set("Content-Type", blockedWriter.FormDataContentType())
	blockedReq.AddCookie(adminCookie)
	blockedResp := httptest.NewRecorder()
	handler.ServeHTTP(blockedResp, blockedReq)
	if blockedResp.Code != http.StatusConflict || !strings.Contains(blockedResp.Body.String(), sj.ImportErrorRecoveryRequired) {
		t.Fatalf("ambiguous preview=%d: %s", blockedResp.Code, blockedResp.Body.String())
	}
	if _, err := sj.LoadImportJournal(instanceDir, secondAccepted.OperationID); err != nil {
		t.Fatalf("ambiguous auto recovery removed journal: %v", err)
	}
	if _, err := os.Stat(durableUploadDir(instanceDir, secondToken)); err != nil {
		t.Fatalf("ambiguous auto recovery removed owned token: %v", err)
	}
	blockedClear, _ := doJSON(t, handler, http.MethodDelete, "/api/jobs", nil, adminCookie)
	if blockedClear.Code != http.StatusConflict || !strings.Contains(blockedClear.Body.String(), sj.ImportErrorRecoveryRequired) {
		t.Fatalf("ambiguous job clear=%d: %s", blockedClear.Code, blockedClear.Body.String())
	}
	keptJob, err := store.GetJob(context.Background(), secondAccepted.JobID)
	if err != nil || keptJob.Status != storage.JobStatusFailed {
		t.Fatalf("ambiguous job clear removed recovery evidence: job=%+v err=%v", keptJob, err)
	}
	if err := sj.WriteImportJournal(instanceDir, secondJournal); err != nil {
		t.Fatal(err)
	}
	secondOwned, err := readDurablePendingUpload(instanceDir, secondToken)
	if err != nil {
		t.Fatal(err)
	}
	secondOwned.JobID = ""
	secondOwned.JobType = ""
	secondOwned.JobIdempotencyKey = ""
	if err := writeDurablePendingUpload(instanceDir, secondToken, secondOwned); err != nil {
		t.Fatal(err)
	}

	// A fresh preview is the user's next normal action. It must automatically
	// converge the provably unsubmitted failed transaction before reading the
	// new ZIP; the original raw token is no longer available to the UI.
	thirdToken := uploadPreview("third-upload.zip")
	if _, err := sj.LoadImportJournal(instanceDir, secondAccepted.OperationID); !os.IsNotExist(err) {
		t.Fatalf("auto-recovered journal survived: %v", err)
	}
	if _, err := os.Stat(durableUploadDir(instanceDir, secondToken)); !os.IsNotExist(err) {
		t.Fatalf("auto-recovered owned token survived: %v", err)
	}
	if secondPreimportPath != "" {
		if _, err := os.Stat(secondPreimportPath); err != nil {
			t.Fatalf("auto recovery removed second preimport backup: %v", err)
		}
	}
	busy, err = sj.HasUnfinishedImportTransaction(instanceDir)
	if err != nil || busy {
		t.Fatalf("automatic recovery left save_import_busy=%v err=%v", busy, err)
	}
	thirdCancel, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/saves/upload-commit-and-start", map[string]any{
		"token": thirdToken, "cancel": true,
	}, adminCookie)
	if thirdCancel.Code != http.StatusOK {
		t.Fatalf("third preview cancel=%d: %s", thirdCancel.Code, thirdCancel.Body.String())
	}

	legacyToken := uploadPreview("legacy-cleared-job-upload.zip")
	legacyCommit, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/saves/upload-commit-and-start", map[string]any{
		"token": legacyToken, "hostHandling": map[string]any{"mode": hostModeVirtualHostTakeover, "acknowledged": true},
	}, adminCookie)
	if legacyCommit.Code == http.StatusConflict && strings.Contains(legacyCommit.Body.String(), sj.ImportErrorRecoveryRequired) {
		// A concurrent Windows journal replace can expose a transient read gap to
		// the immediate post-submit verifier. The same-token retry is the durable
		// API contract and must attach to the already-created job.
		legacyCommit, _ = doJSON(t, handler, http.MethodPost, "/api/instances/stardew/saves/upload-commit-and-start", map[string]any{
			"token": legacyToken, "hostHandling": map[string]any{"mode": hostModeVirtualHostTakeover, "acknowledged": true},
		}, adminCookie)
	}
	if legacyCommit.Code != http.StatusAccepted {
		t.Fatalf("legacy commit=%d: %s", legacyCommit.Code, legacyCommit.Body.String())
	}
	var legacyAccepted saveUploadCommitResponse
	if err := json.Unmarshal(legacyCommit.Body.Bytes(), &legacyAccepted); err != nil {
		t.Fatal(err)
	}
	waitForHTTPJobStatus(t, handler, adminCookie, legacyAccepted.JobID, storage.JobStatusFailed)
	legacyJournal, err := sj.LoadImportJournal(instanceDir, legacyAccepted.OperationID)
	if err != nil || legacyJournal.UpstreamSubmitted || legacyJournal.UpstreamConfirmed ||
		!sj.ImportJournalHasJobIdentity(legacyJournal, stored.ID, legacyAccepted.JobID) {
		t.Fatalf("legacy failed journal=%+v err=%v", legacyJournal, err)
	}
	legacyOwned, err := readDurablePendingUpload(instanceDir, legacyToken)
	if err != nil || legacyOwned.Status != "owned" || legacyOwned.JobID != legacyAccepted.JobID || legacyOwned.JobType != sj.SaveImportJobType ||
		legacyOwned.JobIdempotencyKey != sj.SaveImportJobIdempotencyKey(legacyAccepted.OperationID) {
		t.Fatalf("legacy owned token=%+v err=%v", legacyOwned, err)
	}
	legacyPreimportPath := ""
	if legacyJournal.PreimportBackupName != "" {
		legacyPreimportPath = filepath.Join(instanceDir, ".local-container", "backups", "saves", legacyJournal.PreimportBackupName)
	}

	// Reproduce v0.5.5: its clear endpoint first proved that no job was active,
	// deleted every terminal row, and only then wrote the summary audit. The
	// next ordinary upload must use that later audit plus the exact token/journal
	// binding as legacy terminality evidence instead of remaining busy forever.
	deleted, err := store.ClearJobs(context.Background())
	if err != nil || deleted == 0 {
		t.Fatalf("simulate legacy job clear deleted=%d err=%v", deleted, err)
	}
	if err := store.CreateAuditLog(context.Background(), storage.AuditLogParams{
		Action: "jobs_cleared", TargetType: "jobs", TargetID: "all", Metadata: `{"count":1}`,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetJob(context.Background(), legacyAccepted.JobID); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("legacy job survived simulated clear: %v", err)
	}
	clearedAt, found, err := store.LatestJobsClearedAt(context.Background())
	if err != nil || !found {
		t.Fatalf("legacy clear evidence time=%s found=%v err=%v", clearedAt, found, err)
	}
	legacyReference, err := newDurablePendingUploadStore().findOwnedByOperation(instanceDir, stored.ID, legacyAccepted.OperationID)
	if err != nil || legacyReference.Entry.JobID != legacyAccepted.JobID || legacyReference.RecordUpdatedAt.IsZero() ||
		clearedAt.Before(legacyReference.RecordUpdatedAt.UTC().Truncate(time.Millisecond)) {
		t.Fatalf("legacy upload reference=%+v err=%v", legacyReference, err)
	}
	activeImports, err := manager.Active(context.Background(), storage.ListActiveJobsFilter{TargetType: "instance", TargetID: stored.ID,
		Types: []string{sj.SaveImportJobType, sj.SaveImportRecoveryJobType}})
	if err != nil || len(activeImports) != 0 {
		t.Fatalf("legacy active imports=%+v err=%v", activeImports, err)
	}
	if _, err := manager.GetByIdempotencyKey(context.Background(), sj.SaveImportJobType, "instance", stored.ID,
		sj.SaveImportJobIdempotencyKey(legacyAccepted.OperationID)); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("legacy idempotent job lookup after clear: %v", err)
	}

	afterClearToken := uploadPreview("after-cleared-job-upload.zip")
	if _, err := sj.LoadImportJournal(instanceDir, legacyAccepted.OperationID); !os.IsNotExist(err) {
		t.Fatalf("legacy-cleared journal survived auto recovery: %v", err)
	}
	if _, err := os.Stat(durableUploadDir(instanceDir, legacyToken)); !os.IsNotExist(err) {
		t.Fatalf("legacy-cleared token survived auto recovery: %v", err)
	}
	if legacyPreimportPath != "" {
		if _, err := os.Stat(legacyPreimportPath); err != nil {
			t.Fatalf("legacy-cleared recovery removed preimport backup: %v", err)
		}
	}
	afterClearCancel, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/saves/upload-commit-and-start", map[string]any{
		"token": afterClearToken, "cancel": true,
	}, adminCookie)
	if afterClearCancel.Code != http.StatusOK {
		t.Fatalf("after-clear preview cancel=%d: %s", afterClearCancel.Code, afterClearCancel.Body.String())
	}
}

func waitForImportJournal(t *testing.T, dataDir, operationID string, ready func(sj.ImportJournal) bool) sj.ImportJournal {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	var (
		last    sj.ImportJournal
		lastErr error
	)
	for time.Now().Before(deadline) {
		last, lastErr = sj.LoadImportJournal(dataDir, operationID)
		if lastErr == nil && ready(last) {
			return last
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("import journal did not converge: journal=%+v err=%v", last, lastErr)
	return sj.ImportJournal{}
}
