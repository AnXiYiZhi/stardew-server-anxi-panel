package stardew_junimo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
)

type importEvidenceExec struct {
	run func(context.Context, []string) (paneldocker.CommandResult, error)
}

func (f importEvidenceExec) ComposeExecPipe(ctx context.Context, _, _, _ string, args ...string) (paneldocker.CommandResult, error) {
	return f.run(ctx, args)
}

func writeImportEvidenceIntent(t *testing.T, dataDir, body string) {
	t.Helper()
	path := saveImportIntentPath(dataDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func importEvidenceErrorCode(t *testing.T, err error) string {
	t.Helper()
	var evidenceErr *ImportEvidenceError
	if !errors.As(err, &evidenceErr) {
		t.Fatalf("error = %T %v, want ImportEvidenceError", err, err)
	}
	return evidenceErr.Code
}

func TestImportEvidencePendingFileMissing(t *testing.T) {
	intent, err := ReadJunimoSaveImportIntent(t.TempDir())
	if err != nil || intent.Exists {
		t.Fatalf("intent=%+v err=%v", intent, err)
	}
}

func TestImportEvidencePendingNull(t *testing.T) {
	dir := t.TempDir()
	writeImportEvidenceIntent(t, dir, `{"Pending":null}`)
	intent, err := ReadJunimoSaveImportIntent(dir)
	if err != nil || intent.Exists {
		t.Fatalf("intent=%+v err=%v", intent, err)
	}
}

func TestImportEvidencePendingComplete(t *testing.T) {
	dir := t.TempDir()
	writeImportEvidenceIntent(t, dir, `{"Pending":{"SaveName":"Farm_123","OwnerUid":42,"UserId":"private-platform-id"}}`)
	intent, err := ReadJunimoSaveImportIntent(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !intent.Exists || intent.SaveName != "Farm_123" || intent.OwnerUID != 42 || intent.UserID != "private-platform-id" {
		t.Fatalf("unexpected intent: %+v", intent)
	}
}

func TestImportEvidencePendingInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	writeImportEvidenceIntent(t, dir, `{"Pending":`)
	errText := "private-platform-id"
	_, err := ReadJunimoSaveImportIntent(dir)
	if got := importEvidenceErrorCode(t, err); got != "pending_intent_invalid_json" {
		t.Fatalf("code=%q", got)
	}
	if strings.Contains(err.Error(), errText) {
		t.Fatal("error leaked UserID")
	}
}

func TestImportEvidenceSaveNameMismatch(t *testing.T) {
	op := strings.Repeat("a", 32)
	pending := JunimoSaveImportIntent{Exists: true, SaveName: "Other_2", UserID: "private-platform-id"}
	journal := ImportJournal{OperationID: op, SaveName: "Farm_1", PlatformIDFingerprint: platformFingerprint(op, pending.UserID)}
	if got := ComparePendingIntent(journal, pending); got != EvidenceMismatch {
		t.Fatalf("result=%q", got)
	}
}

func TestImportEvidenceFingerprintMatchAndMismatch(t *testing.T) {
	op := strings.Repeat("b", 32)
	pending := JunimoSaveImportIntent{Exists: true, UserID: "private-platform-id"}
	if got := ComparePendingPlatformFingerprint(op, platformFingerprint(op, pending.UserID), pending); got != EvidenceMatch {
		t.Fatalf("match result=%q", got)
	}
	if got := ComparePendingPlatformFingerprint(op, platformFingerprint(op, "different-id"), pending); got != EvidenceMismatch {
		t.Fatalf("mismatch result=%q", got)
	}
	if got := ComparePendingPlatformFingerprint(op, "", pending); got != EvidenceUnknown {
		t.Fatalf("unknown result=%q", got)
	}
}

func TestImportEvidenceDiagnosticsNormal(t *testing.T) {
	exec := importEvidenceExec{run: func(_ context.Context, _ []string) (paneldocker.CommandResult, error) {
		return paneldocker.CommandResult{Stdout: `{"saveImportFinalizeCount":3,"masterName":"Server","failedFields":[]}`}, nil
	}}
	state, err := ReadJunimoDiagnosticsState(context.Background(), exec, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if state.FinalizeCount == nil || *state.FinalizeCount != 3 || state.MasterName == nil || *state.MasterName != "Server" {
		t.Fatalf("state=%+v", state)
	}
}

func TestImportEvidenceDiagnosticsMissingFinalizeCount(t *testing.T) {
	exec := importEvidenceExec{run: func(_ context.Context, _ []string) (paneldocker.CommandResult, error) {
		return paneldocker.CommandResult{Stdout: `{"masterName":"Server","failedFields":[]}`}, nil
	}}
	_, err := ReadJunimoDiagnosticsState(context.Background(), exec, t.TempDir())
	if got := importEvidenceErrorCode(t, err); got != "diagnostics_field_missing" {
		t.Fatalf("code=%q", got)
	}
}

func TestImportEvidenceDiagnosticsFailedFinalizeCount(t *testing.T) {
	exec := importEvidenceExec{run: func(_ context.Context, _ []string) (paneldocker.CommandResult, error) {
		return paneldocker.CommandResult{Stdout: `{"saveImportFinalizeCount":0,"masterName":"Server","failedFields":["saveImportFinalizeCount"]}`}, nil
	}}
	_, err := ReadJunimoDiagnosticsState(context.Background(), exec, t.TempDir())
	if got := importEvidenceErrorCode(t, err); got != "diagnostics_field_failed" {
		t.Fatalf("code=%q", got)
	}
}

func TestImportEvidenceDiagnosticsAPITimeout(t *testing.T) {
	exec := importEvidenceExec{run: func(_ context.Context, _ []string) (paneldocker.CommandResult, error) {
		return paneldocker.CommandResult{}, context.DeadlineExceeded
	}}
	_, err := ReadJunimoDiagnosticsState(context.Background(), exec, t.TempDir())
	if got := importEvidenceErrorCode(t, err); got != "junimo_api_timeout" {
		t.Fatalf("code=%q", got)
	}
}

func TestImportEvidenceDiagnosticsAPIUnavailable(t *testing.T) {
	exec := importEvidenceExec{run: func(_ context.Context, _ []string) (paneldocker.CommandResult, error) {
		return paneldocker.CommandResult{ExitCode: 7}, errors.New("connection refused")
	}}
	_, err := ReadJunimoDiagnosticsState(context.Background(), exec, t.TempDir())
	if got := importEvidenceErrorCode(t, err); got != "junimo_api_unavailable" {
		t.Fatalf("code=%q", got)
	}
}

func TestImportEvidenceMainSaveHash(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Farm_1")
	data := []byte("read-only save bytes")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := stableFileSHA256(path)
	if err != nil {
		t.Fatal(err)
	}
	wantSum := sha256.Sum256(data)
	if want := hex.EncodeToString(wantSum[:]); got != want {
		t.Fatalf("hash=%q want=%q", got, want)
	}
}

func TestImportEvidenceFileChangesWhileReading(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Farm_1")
	if err := os.WriteFile(path, []byte("before"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := stableFileSHA256WithObserver(path, func() {
		if writeErr := os.WriteFile(path, []byte("after-content"), 0o600); writeErr != nil {
			t.Fatal(writeErr)
		}
	})
	if got := importEvidenceErrorCode(t, err); got != "evidence_file_changed" {
		t.Fatalf("code=%q", got)
	}
}

func TestImportEvidenceProcessIdentityChanges(t *testing.T) {
	output := "Connected to the docker container shell.\n__ANXI_PROCESS_ID__ container-a 100\n"
	exec := importEvidenceExec{run: func(_ context.Context, _ []string) (paneldocker.CommandResult, error) {
		return paneldocker.CommandResult{Stdout: output}, nil
	}}
	first, err := readProcessIdentity(context.Background(), exec, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	output = "Connected to the docker container shell.\n__ANXI_PROCESS_ID__ container-b 200\n"
	second, err := readProcessIdentity(context.Background(), exec, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if *first == *second {
		t.Fatalf("identity did not change: %+v", first)
	}
}

func TestImportEvidenceProcessIdentityRejectsUnmarkedBanner(t *testing.T) {
	exec := importEvidenceExec{run: func(_ context.Context, _ []string) (paneldocker.CommandResult, error) {
		return paneldocker.CommandResult{Stdout: "Connected to the docker container shell.\nExit and run make cli\n"}, nil
	}}
	_, err := readProcessIdentity(context.Background(), exec, t.TempDir())
	if got := importEvidenceErrorCode(t, err); got != "process_identity_invalid" {
		t.Fatalf("code=%q", got)
	}
}

func TestImportEvidenceUserIDNeverSerializedOrReturnedInErrors(t *testing.T) {
	secret := "private-platform-id"
	intent := JunimoSaveImportIntent{Exists: true, SaveName: "Farm_1", OwnerUID: 42, UserID: secret}
	raw, err := json.Marshal(intent)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), secret) || strings.Contains(string(raw), "UserID") || strings.Contains(string(raw), "userId") {
		t.Fatalf("serialized intent leaked UserID: %s", raw)
	}
	bad := t.TempDir()
	writeImportEvidenceIntent(t, bad, `{"Pending":`)
	_, readErr := ReadJunimoSaveImportIntent(bad)
	if readErr == nil || strings.Contains(readErr.Error(), secret) {
		t.Fatalf("unsafe error: %v", readErr)
	}
}
