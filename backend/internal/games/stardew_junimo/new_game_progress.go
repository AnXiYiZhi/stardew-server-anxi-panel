package stardew_junimo

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	newGameCreationWriterStartup = "startup"
	newGameCreationWriterHTTP    = "http"

	newGameProgressControl    = "control"
	newGameProgressGameloader = "gameloader"
	newGameProgressDirectory  = "directory"
)

// NewGameRuntimeStatusSnapshot is the transaction-start Control status
// baseline. New fields can be published by newer Control Mod versions without
// making older status files unreadable.
type NewGameRuntimeStatusSnapshot struct {
	Present          bool       `json:"present"`
	State            string     `json:"state,omitempty"`
	SaveID           string     `json:"saveId,omitempty"`
	TransactionID    string     `json:"newGameTransactionId,omitempty"`
	CreationObserved bool       `json:"newGameCreationObserved"`
	UpdatedAt        *time.Time `json:"updatedAt,omitempty"`
}

// NewGameProgressEvidence is a composite, persisted view of evidence that a
// save-creation writer has made progress. Once Observed is true, callers must
// not submit another creation command for the transaction.
type NewGameProgressEvidence struct {
	Observed      bool
	Kind          string
	SaveName      string
	Ambiguous     bool
	ControlStatus NewGameRuntimeStatusSnapshot
	NewSaveDirs   []string
}

func readNewGameRuntimeStatusSnapshot(dataDir string) (NewGameRuntimeStatusSnapshot, error) {
	raw, err := readRuntimeStatusFile(filepath.Join(controlDir(dataDir), "status.json"))
	if errors.Is(err, os.ErrNotExist) {
		return NewGameRuntimeStatusSnapshot{}, nil
	}
	if err != nil {
		return NewGameRuntimeStatusSnapshot{}, err
	}
	var payload struct {
		State            string     `json:"state"`
		SaveID           string     `json:"saveId"`
		TransactionID    string     `json:"newGameTransactionId"`
		CreationObserved bool       `json:"newGameCreationObserved"`
		UpdatedAt        *time.Time `json:"updatedAt"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return NewGameRuntimeStatusSnapshot{}, fmt.Errorf("parse Control status: %w", err)
	}
	return NewGameRuntimeStatusSnapshot{
		Present:          true,
		State:            strings.TrimSpace(payload.State),
		SaveID:           strings.TrimSpace(payload.SaveID),
		TransactionID:    strings.TrimSpace(payload.TransactionID),
		CreationObserved: payload.CreationObserved,
		UpdatedAt:        payload.UpdatedAt,
	}, nil
}

func newGameGameloaderSaveName(snapshot newGameFileSnapshot) string {
	if !snapshot.Exists || len(snapshot.Data) == 0 {
		return ""
	}
	var payload struct {
		SaveName string `json:"SaveNameToLoad"`
	}
	if json.Unmarshal(snapshot.Data, &payload) != nil {
		return ""
	}
	return strings.TrimSpace(payload.SaveName)
}

func chooseNewGameCreationWriter(dataDir, activeSave string) string {
	if !isCompleteNewGameActiveSave(dataDir, activeSave) {
		return newGameCreationWriterStartup
	}
	return newGameCreationWriterHTTP
}

func isCompleteNewGameActiveSave(dataDir, saveName string) bool {
	if validateSaveName(saveName) != nil {
		return false
	}
	root := filepath.Join(savesDir(dataDir), "Saves", saveName)
	if !newGameWellFormedXMLFile(filepath.Join(root, saveName)) {
		return false
	}
	return newGameWellFormedXMLFile(filepath.Join(root, "SaveGameInfo")) ||
		newGameWellFormedXMLFile(filepath.Join(root, "SaveGameInfo.xml"))
}

func newGameWellFormedXMLFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxSingleFileBytes {
		return false
	}
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	decoder := xml.NewDecoder(io.LimitReader(file, maxSingleFileBytes+1))
	for {
		if _, err := decoder.Token(); err == io.EOF {
			return true
		} else if err != nil {
			return false
		}
	}
}

func (tx *newGameTransaction) detectNewSaveDirs() ([]string, error) {
	current, err := listSaveDirs(tx.dataDir)
	if err != nil {
		return nil, err
	}
	before := make(map[string]struct{}, len(tx.record.PreexistingSaveDirs))
	for _, name := range tx.record.PreexistingSaveDirs {
		before[name] = struct{}{}
	}
	result := make([]string, 0, len(current))
	for _, name := range current {
		if _, existed := before[name]; !existed {
			result = append(result, name)
		}
	}
	sort.Strings(result)
	return result, nil
}

func (tx *newGameTransaction) observeNewGameProgress() (NewGameProgressEvidence, error) {
	status, err := readNewGameRuntimeStatusSnapshot(tx.dataDir)
	if err != nil {
		return NewGameProgressEvidence{}, err
	}

	loaderSnapshot := newGameFileSnapshot{}
	loaderData, loaderErr := os.ReadFile(gameloaderPath(tx.dataDir))
	if loaderErr == nil {
		loaderSnapshot.Exists = true
		loaderSnapshot.Data = loaderData
	} else if !errors.Is(loaderErr, os.ErrNotExist) {
		return NewGameProgressEvidence{}, loaderErr
	}
	loaderSave := newGameGameloaderSaveName(loaderSnapshot)
	loaderChanged := loaderSave != "" && loaderSave != tx.record.InitialGameloaderSave

	newDirs, err := tx.detectNewSaveDirs()
	if err != nil {
		return NewGameProgressEvidence{}, err
	}
	controlObserved := status.TransactionID == tx.record.TransactionID &&
		(status.CreationObserved || strings.EqualFold(status.State, "save-creating"))

	currentKind := ""
	switch {
	case controlObserved:
		currentKind = newGameProgressControl
	case loaderChanged:
		currentKind = newGameProgressGameloader
	case len(newDirs) > 0:
		currentKind = newGameProgressDirectory
	}
	currentObserved := currentKind != ""
	currentSave, currentAmbiguous := reconcileNewGameProgressSave(status, controlObserved, loaderSave, loaderChanged, newDirs)

	before := tx.record
	changed := false
	if strings.Join(tx.record.DetectedSaveDirs, "\x00") != strings.Join(newDirs, "\x00") {
		tx.record.DetectedSaveDirs = append([]string{}, newDirs...)
		changed = true
	}
	if currentObserved {
		if !tx.record.ProgressObserved {
			now := time.Now().UTC()
			tx.record.ProgressObserved = true
			tx.record.ProgressObservedAt = &now
			changed = true
		}
		if newGameProgressRank(currentKind) < newGameProgressRank(tx.record.ProgressKind) {
			tx.record.ProgressKind = currentKind
			changed = true
		}
		if controlObserved && tx.record.ProgressControlState != status.State {
			tx.record.ProgressControlState = status.State
			changed = true
		}
		if currentSave != "" {
			switch {
			case tx.record.ProgressSave == "":
				tx.record.ProgressSave = currentSave
				changed = true
			case tx.record.ProgressSave != currentSave:
				currentAmbiguous = true
			}
		}
	}
	if currentAmbiguous && !tx.record.ProgressAmbiguous {
		tx.record.ProgressAmbiguous = true
		changed = true
	}
	if tx.record.ProgressAmbiguous && tx.record.ProgressSave != "" {
		tx.record.ProgressSave = ""
		changed = true
	}
	if changed {
		if err := tx.persist(); err != nil {
			tx.record = before
			return NewGameProgressEvidence{}, err
		}
	}

	return NewGameProgressEvidence{
		Observed:      tx.record.ProgressObserved,
		Kind:          tx.record.ProgressKind,
		SaveName:      tx.record.ProgressSave,
		Ambiguous:     tx.record.ProgressAmbiguous,
		ControlStatus: status,
		NewSaveDirs:   append([]string{}, newDirs...),
	}, nil
}

func (tx *newGameTransaction) hasNewGameCreationProgress() (bool, error) {
	evidence, err := tx.observeNewGameProgress()
	return evidence.Observed, err
}

// bindTargetSave durably binds a proven, unambiguous candidate to the exact
// transaction marker consumed by the Control Mod. CandidateSave is persisted
// first so an interrupted marker write is safely retryable with the same ID.
func (tx *newGameTransaction) bindTargetSave(saveID string) error {
	saveID = strings.TrimSpace(saveID)
	if err := validateSaveName(saveID); err != nil {
		return &NewGameTransactionError{Code: "new_game_target_invalid", Message: "目标存档 ID 无效", Cause: err}
	}
	if err := tx.assertOwner(); err != nil {
		return err
	}
	evidence, err := tx.observeNewGameProgress()
	if err != nil {
		return &NewGameTransactionError{Code: "new_game_target_evidence_failed", Message: "读取目标存档进展证据失败", Cause: err}
	}
	if evidence.Ambiguous {
		return &NewGameTransactionError{Code: "new_game_target_ambiguous", Message: "检测到多个或互相冲突的目标存档证据，禁止绑定"}
	}
	if !evidence.Observed || evidence.SaveName == "" || evidence.SaveName != saveID {
		return &NewGameTransactionError{Code: "new_game_target_unproven", Message: "目标存档尚未由当前事务的 loader 或新目录证据证明"}
	}

	markerData, err := os.ReadFile(newGamePendingPath(tx.dataDir))
	if err != nil {
		return &NewGameTransactionError{Code: "new_game_marker_read_failed", Message: "读取 pending marker 失败", Cause: err}
	}
	var marker newGamePendingMarker
	if err := json.Unmarshal(markerData, &marker); err != nil {
		return &NewGameTransactionError{Code: "new_game_marker_invalid", Message: "pending marker JSON 无效", Cause: err}
	}
	if marker.SchemaVersion != newGameMarkerSchemaVersion || marker.TransactionID != tx.record.TransactionID ||
		marker.RequestedFarmType != tx.record.RequestedFarmType || !strings.EqualFold(marker.State, "pending") ||
		marker.ExpiresAt.IsZero() || !marker.ExpiresAt.After(time.Now().UTC()) {
		return &NewGameTransactionError{Code: "new_game_marker_mismatch", Message: "pending marker 不属于当前有效事务"}
	}
	if marker.TargetSaveID != "" && marker.TargetSaveID != saveID {
		return &NewGameTransactionError{Code: "new_game_target_conflict", Message: "pending marker 已绑定到另一个目标存档"}
	}
	if tx.record.CandidateSave != "" && tx.record.CandidateSave != saveID {
		return &NewGameTransactionError{Code: "new_game_target_conflict", Message: "事务已绑定到另一个目标存档"}
	}

	if tx.record.CandidateSave == "" {
		before := tx.record
		tx.record.CandidateSave = saveID
		if err := tx.persist(); err != nil {
			tx.record = before
			return &NewGameTransactionError{Code: "new_game_state_write_failed", Message: "持久化目标存档绑定失败", Cause: err}
		}
	}
	if marker.TargetSaveID == saveID {
		return nil
	}
	if err := tx.assertOwner(); err != nil {
		return err
	}
	marker.TargetSaveID = saveID
	payload, err := json.MarshalIndent(marker, "", "  ")
	if err != nil {
		return &NewGameTransactionError{Code: "new_game_marker_write_failed", Message: "生成目标存档 marker 失败", Cause: err}
	}
	if err := tx.writeJSON(newGamePendingPath(tx.dataDir), payload, 0o644); err != nil {
		return &NewGameTransactionError{Code: "new_game_marker_write_failed", Message: "原子写入目标存档 marker 失败", Cause: err}
	}
	return nil
}

func newGameProgressRank(kind string) int {
	switch kind {
	case newGameProgressControl:
		return 1
	case newGameProgressGameloader:
		return 2
	case newGameProgressDirectory:
		return 3
	default:
		return 4
	}
}

func reconcileNewGameProgressSave(
	status NewGameRuntimeStatusSnapshot,
	controlObserved bool,
	loaderSave string,
	loaderChanged bool,
	newDirs []string,
) (string, bool) {
	candidates := make(map[string]struct{})
	addCandidate := func(name string) {
		name = strings.TrimSpace(name)
		if name != "" {
			candidates[name] = struct{}{}
		}
	}
	resolveAgainstDirs := func(name string) string {
		for _, candidate := range newDirs {
			if candidate == name {
				return candidate
			}
		}
		if repaired := uniqueNumericSuffixCandidate(name, newDirs); repaired != "" {
			return repaired
		}
		return name
	}
	matchesDirs := func(name string) bool {
		if len(newDirs) == 0 {
			return false
		}
		resolved := resolveAgainstDirs(name)
		for _, candidate := range newDirs {
			if candidate == resolved {
				return true
			}
		}
		return false
	}

	// A loader pointer can advance before the save directory exists. That is
	// irreversible creation progress (and therefore suppresses /newgame), but
	// it is not yet a bindable target. Wait until the new directory agrees so
	// a truncated/non-existent loader value can never pass the candidate gate.
	if loaderChanged && loaderSave != "" && matchesDirs(loaderSave) {
		addCandidate(resolveAgainstDirs(loaderSave))
	}
	// Control's process-scoped creationObserved flag can survive a later load
	// of an unrelated old save. Without transaction-bound target evidence, a
	// status saveId is only a candidate when a newly appeared directory agrees.
	if controlObserved && status.SaveID != "" && matchesDirs(status.SaveID) {
		addCandidate(resolveAgainstDirs(status.SaveID))
	}
	for _, name := range newDirs {
		addCandidate(name)
	}
	if len(newDirs) > 1 || len(candidates) > 1 {
		return "", true
	}
	for candidate := range candidates {
		return candidate, false
	}
	return "", false
}
