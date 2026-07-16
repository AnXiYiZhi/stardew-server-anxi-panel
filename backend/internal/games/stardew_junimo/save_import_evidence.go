package stardew_junimo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	junimoSaveImportDataFile = "junimohost.saveimport.json"
	importEvidenceAPITimeout = 4 * time.Second

	EvidenceMatch    = "match"
	EvidenceMismatch = "mismatch"
	EvidenceUnknown  = "unknown"
)

// JunimoSaveImportIntent is the read-only view of JunimoHost.SaveImport.
// UserID is deliberately excluded from every JSON representation. It may only
// be held briefly in memory for the operation-salted fingerprint comparison.
type JunimoSaveImportIntent struct {
	Exists   bool   `json:"exists"`
	SaveName string `json:"saveName,omitempty"`
	OwnerUID int64  `json:"ownerUid,omitempty"`
	UserID   string `json:"-"`
}

type JunimoDiagnosticsState struct {
	FinalizeCount *int     `json:"finalizeCount,omitempty"`
	MasterName    *string  `json:"masterName,omitempty"`
	FailedFields  []string `json:"failedFields"`
}

type JunimoProcessIdentity struct {
	ContainerID       string `json:"containerId,omitempty"`
	ProcessStartTicks string `json:"processStartTicks,omitempty"`
}

type JunimoImportEvidenceSnapshot struct {
	MainSaveSHA256        string                 `json:"mainSaveSha256,omitempty"`
	SaveGameInfoSHA256    string                 `json:"saveGameInfoSha256,omitempty"`
	ActivePointer         string                 `json:"activePointer,omitempty"`
	PendingIntent         JunimoSaveImportIntent `json:"pendingIntent"`
	FinalizeCount         *int                   `json:"finalizeCount,omitempty"`
	MasterName            *string                `json:"masterName,omitempty"`
	ProcessIdentity       *JunimoProcessIdentity `json:"processIdentity,omitempty"`
	RuntimeSaveID         string                 `json:"runtimeSaveId,omitempty"`
	DayTransitionComplete *bool                  `json:"dayTransitionComplete,omitempty"`
	UnknownFields         []string               `json:"unknownFields,omitempty"`
	CapturedAt            time.Time              `json:"capturedAt"`
}

type ImportEvidenceError struct {
	Code    string
	Message string
	Cause   error
}

func (e *ImportEvidenceError) Error() string { return e.Message }
func (e *ImportEvidenceError) Unwrap() error { return e.Cause }

func saveImportIntentPath(dataDir string) string {
	return filepath.Join(savesDir(dataDir), ".smapi", "mod-data", "junimohost.server", junimoSaveImportDataFile)
}

// ReadJunimoSaveImportIntent only reads SMAPI's global-data file. Missing data
// and Pending=null mean no intent; malformed JSON is an explicit error.
func ReadJunimoSaveImportIntent(dataDir string) (JunimoSaveImportIntent, error) {
	raw, err := os.ReadFile(saveImportIntentPath(dataDir))
	if errors.Is(err, os.ErrNotExist) {
		return JunimoSaveImportIntent{}, nil
	}
	if err != nil {
		return JunimoSaveImportIntent{}, fmt.Errorf("read Junimo save-import intent: %w", err)
	}
	var data struct {
		Pending *struct {
			SaveName string `json:"SaveName"`
			OwnerUID int64  `json:"OwnerUid"`
			UserID   string `json:"UserId"`
		} `json:"Pending"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return JunimoSaveImportIntent{}, &ImportEvidenceError{
			Code: "pending_intent_invalid_json", Message: "Junimo save-import intent JSON is invalid", Cause: err,
		}
	}
	if data.Pending == nil {
		return JunimoSaveImportIntent{}, nil
	}
	return JunimoSaveImportIntent{
		Exists: true, SaveName: data.Pending.SaveName, OwnerUID: data.Pending.OwnerUID, UserID: data.Pending.UserID,
	}, nil
}

// ComparePendingPlatformFingerprint never returns or formats the raw UserID.
func ComparePendingPlatformFingerprint(operationID, expectedFingerprint string, pending JunimoSaveImportIntent) string {
	if !pending.Exists || pending.UserID == "" || expectedFingerprint == "" || !validImportOperationID(operationID) {
		return EvidenceUnknown
	}
	if platformFingerprint(operationID, pending.UserID) == expectedFingerprint {
		return EvidenceMatch
	}
	return EvidenceMismatch
}

// ComparePendingIntent requires both the target save and the salted platform
// fingerprint to agree with the journal.
func ComparePendingIntent(journal ImportJournal, pending JunimoSaveImportIntent) string {
	if !pending.Exists {
		return EvidenceUnknown
	}
	if pending.SaveName != journal.SaveName {
		return EvidenceMismatch
	}
	return ComparePendingPlatformFingerprint(journal.OperationID, journal.PlatformIDFingerprint, pending)
}

func readJunimoAPI(ctx context.Context, exec commandExecutor, dataDir, endpoint string) ([]byte, error) {
	apiPort, apiKey, err := readJunimoAPIConfig(dataDir)
	if err != nil {
		return nil, err
	}
	args := []string{"curl", "-sf", "--max-time", "3"}
	if apiKey != "" {
		args = append(args, "-H", "Authorization: Bearer "+apiKey)
	}
	args = append(args, "http://localhost:"+apiPort+endpoint)
	reqCtx, cancel := context.WithTimeout(ctx, importEvidenceAPITimeout)
	defer cancel()
	result, err := exec.ComposeExecPipe(reqCtx, dataDir, "server", "", args...)
	if errors.Is(reqCtx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		cause := err
		if cause == nil {
			cause = reqCtx.Err()
		}
		return nil, &ImportEvidenceError{Code: "junimo_api_timeout", Message: "Junimo diagnostics API timed out", Cause: cause}
	}
	if errors.Is(reqCtx.Err(), context.Canceled) {
		return nil, &ImportEvidenceError{Code: "evidence_capture_canceled", Message: "Junimo evidence capture was canceled", Cause: reqCtx.Err()}
	}
	if err != nil || result.ExitCode != 0 {
		cause := err
		if cause == nil {
			cause = fmt.Errorf("curl exit code %d", result.ExitCode)
		}
		return nil, &ImportEvidenceError{Code: "junimo_api_unavailable", Message: "Junimo diagnostics API is unavailable", Cause: cause}
	}
	return []byte(result.Stdout), nil
}

func ReadJunimoDiagnosticsState(ctx context.Context, exec commandExecutor, dataDir string) (JunimoDiagnosticsState, error) {
	raw, err := readJunimoAPI(ctx, exec, dataDir, "/diagnostics/state")
	if err != nil {
		return JunimoDiagnosticsState{}, err
	}
	var payload struct {
		FinalizeCount *int     `json:"saveImportFinalizeCount"`
		MasterName    *string  `json:"masterName"`
		FailedFields  []string `json:"failedFields"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return JunimoDiagnosticsState{}, &ImportEvidenceError{Code: "diagnostics_invalid_json", Message: "Junimo diagnostics JSON is invalid", Cause: err}
	}
	state := JunimoDiagnosticsState{FinalizeCount: payload.FinalizeCount, MasterName: payload.MasterName, FailedFields: payload.FailedFields}
	requiredFieldFailed := false
	for _, field := range payload.FailedFields {
		switch field {
		case "saveImportFinalizeCount":
			state.FinalizeCount = nil
			requiredFieldFailed = true
		case "masterState", "masterName", "gameThreadTimeout":
			state.MasterName = nil
			requiredFieldFailed = true
		}
	}
	if requiredFieldFailed {
		return state, &ImportEvidenceError{Code: "diagnostics_field_failed", Message: "Junimo diagnostics could not read a required import evidence field"}
	}
	if payload.FinalizeCount == nil || payload.MasterName == nil {
		return state, &ImportEvidenceError{Code: "diagnostics_field_missing", Message: "Junimo diagnostics response is missing a required import evidence field"}
	}
	return state, nil
}

func readDayTransitionComplete(ctx context.Context, exec commandExecutor, dataDir string) (*bool, error) {
	raw, err := readJunimoAPI(ctx, exec, dataDir, "/status")
	if err != nil {
		return nil, err
	}
	var payload struct {
		Value *bool `json:"dayTransitionComplete"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, &ImportEvidenceError{Code: "status_invalid_json", Message: "Junimo status JSON is invalid", Cause: err}
	}
	if payload.Value == nil {
		return nil, &ImportEvidenceError{Code: "status_field_missing", Message: "Junimo status response is missing dayTransitionComplete"}
	}
	return payload.Value, nil
}

func readProcessIdentity(ctx context.Context, exec commandExecutor, dataDir string) (*JunimoProcessIdentity, error) {
	reqCtx, cancel := context.WithTimeout(ctx, importEvidenceAPITimeout)
	defer cancel()
	const marker = "__ANXI_PROCESS_ID__"
	result, err := exec.ComposeExecPipe(reqCtx, dataDir, "server", "", "sh", "-lc", "printf '__ANXI_PROCESS_ID__ %s ' \"$(hostname)\"; awk '{print $22}' /proc/1/stat")
	if err != nil || result.ExitCode != 0 {
		return nil, &ImportEvidenceError{Code: "process_identity_unavailable", Message: "Junimo process identity is unavailable", Cause: err}
	}
	for _, line := range strings.Split(result.Stdout, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 || fields[0] != marker || fields[1] == "" {
			continue
		}
		if _, parseErr := strconv.ParseUint(fields[2], 10, 64); parseErr != nil {
			continue
		}
		return &JunimoProcessIdentity{ContainerID: fields[1], ProcessStartTicks: fields[2]}, nil
	}
	return nil, &ImportEvidenceError{Code: "process_identity_invalid", Message: "Junimo process identity response is invalid"}
}

func stableFileSHA256(path string) (string, error) {
	return stableFileSHA256WithObserver(path, nil)
}

func stableFileSHA256WithObserver(path string, betweenReads func()) (string, error) {
	read := func() ([sha256.Size]byte, os.FileInfo, error) {
		var zero [sha256.Size]byte
		f, err := os.Open(path)
		if err != nil {
			return zero, nil, err
		}
		defer f.Close()
		before, err := f.Stat()
		if err != nil {
			return zero, nil, err
		}
		h := sha256.New()
		if _, err := io.Copy(h, f); err != nil {
			return zero, nil, err
		}
		after, err := f.Stat()
		if err != nil {
			return zero, nil, err
		}
		if before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
			return zero, nil, &ImportEvidenceError{Code: "evidence_file_changed", Message: "save evidence file changed while being read"}
		}
		var sum [sha256.Size]byte
		copy(sum[:], h.Sum(nil))
		return sum, after, nil
	}
	first, firstInfo, err := read()
	if err != nil {
		return "", err
	}
	if betweenReads != nil {
		betweenReads()
	}
	second, secondInfo, err := read()
	if err != nil {
		return "", err
	}
	if first != second || firstInfo.Size() != secondInfo.Size() || !firstInfo.ModTime().Equal(secondInfo.ModTime()) || !os.SameFile(firstInfo, secondInfo) {
		return "", &ImportEvidenceError{Code: "evidence_file_changed", Message: "save evidence file changed while being read"}
	}
	return hex.EncodeToString(first[:]), nil
}

func readActivePointerStrict(dataDir string) (string, error) {
	raw, err := os.ReadFile(gameloaderPath(dataDir))
	if err != nil {
		return "", err
	}
	var payload struct {
		SaveName string `json:"SaveNameToLoad"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", err
	}
	if strings.TrimSpace(payload.SaveName) == "" {
		return "", errors.New("active pointer is empty")
	}
	return payload.SaveName, nil
}

func readRuntimeSaveID(dataDir string) (string, error) {
	raw, err := readRuntimeStatusFile(filepath.Join(controlDir(dataDir), "status.json"))
	if err != nil {
		return "", err
	}
	var payload struct {
		SaveID string `json:"saveId"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", err
	}
	if strings.TrimSpace(payload.SaveID) == "" {
		return "", errors.New("runtime save id is empty")
	}
	return payload.SaveID, nil
}

// CaptureJunimoImportEvidence takes a read-only, best-effort composite
// snapshot. Unreadable optional sources are recorded as unknown. A malformed
// pending-intent file is returned as an error because treating it as absent
// would erase safety-critical evidence.
func CaptureJunimoImportEvidence(ctx context.Context, exec commandExecutor, dataDir, saveName string) (JunimoImportEvidenceSnapshot, error) {
	snapshot := JunimoImportEvidenceSnapshot{CapturedAt: time.Now().UTC()}
	unknown := map[string]bool{}
	markUnknown := func(name string) { unknown[name] = true }

	intent, err := ReadJunimoSaveImportIntent(dataDir)
	if err != nil {
		return snapshot, err
	}
	snapshot.PendingIntent = intent

	saveRoot := filepath.Join(savesDir(dataDir), "Saves", saveName)
	if snapshot.MainSaveSHA256, err = stableFileSHA256(filepath.Join(saveRoot, saveName)); err != nil {
		markUnknown("mainSaveSha256")
	}
	if snapshot.SaveGameInfoSHA256, err = stableFileSHA256(filepath.Join(saveRoot, "SaveGameInfo")); err != nil {
		markUnknown("saveGameInfoSha256")
	}
	if snapshot.ActivePointer, err = readActivePointerStrict(dataDir); err != nil {
		markUnknown("activePointer")
	}
	if snapshot.RuntimeSaveID, err = readRuntimeSaveID(dataDir); err != nil {
		markUnknown("runtimeSaveId")
	}
	diagnostics, _ := ReadJunimoDiagnosticsState(ctx, exec, dataDir)
	snapshot.FinalizeCount, snapshot.MasterName = diagnostics.FinalizeCount, diagnostics.MasterName
	if snapshot.FinalizeCount == nil {
		markUnknown("finalizeCount")
	}
	if snapshot.MasterName == nil {
		markUnknown("masterName")
	}
	if snapshot.ProcessIdentity, err = readProcessIdentity(ctx, exec, dataDir); err != nil {
		markUnknown("processIdentity")
	}
	if snapshot.DayTransitionComplete, err = readDayTransitionComplete(ctx, exec, dataDir); err != nil {
		markUnknown("dayTransitionComplete")
	}
	for field := range unknown {
		snapshot.UnknownFields = append(snapshot.UnknownFields, field)
	}
	sort.Strings(snapshot.UnknownFields)
	return snapshot, nil
}
