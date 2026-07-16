package web

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
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
	d.sourceDir = target
	return &registry.Job{ID: "job-import-ownership"}, nil
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
	driver := &capturingImportOwnershipDriver{}
	drivers := registry.New()
	if err := drivers.Register(driver); err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(Deps{Config: config.Config{DataDir: dataDir, Secret: "test-secret", Version: "test"}, Store: store, Registry: drivers, Logger: slog.Default()})
	setup, adminCookie := doJSON(t, handler, http.MethodPost, "/api/setup/admin", map[string]string{"username": "admin", "password": "admin-password", "confirmPassword": "admin-password"}, nil)
	if setup.Code != http.StatusOK {
		t.Fatalf("setup=%d: %s", setup.Code, setup.Body.String())
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
