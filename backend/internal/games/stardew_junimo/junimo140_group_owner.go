package stardew_junimo

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

type junimo140GroupLease struct {
	SchemaVersion int    `json:"schemaVersion"`
	ApplyID       string `json:"applyId"`
	PrimaryID     string `json:"primaryId"`
	PlanSHA256    string `json:"planSha256"`
}

type junimo140GroupJournal struct {
	SchemaVersion int                `json:"schemaVersion"`
	Plan          junimo140GroupPlan `json:"plan"`
	PlanSHA256    string             `json:"planSha256"`
	Phase         string             `json:"phase"`
	UpdatedAt     string             `json:"updatedAt"`
}

func junimo140GroupLeasePath(dataDir string) string {
	return filepath.Join(dataDir, ".local-container", "junimo140-lease.json")
}

func junimo140GroupJournalPath(plan junimo140GroupPlan) (string, error) {
	primary, err := plan.primary()
	if err != nil || !junimo140GroupApplyID.MatchString(plan.ApplyID) {
		return "", errors.New("invalid shared runtime journal identity")
	}
	return filepath.Join(primary.DataDir, ".local-container", "junimo140-group", plan.ApplyID, "manifest.json"), nil
}

func ensureJunimo140GroupDirectories(root string, parts ...string) error {
	if err := plainJunimoGroupRoot(root); err != nil {
		return err
	}
	path := root
	for _, part := range parts {
		if part == "" || filepath.Base(part) != part || part == "." || part == ".." {
			return errors.New("invalid shared runtime private directory")
		}
		path = filepath.Join(path, part)
		if err := os.Mkdir(path, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
			return err
		} else if err == nil {
			if err := syncNewGameOwnerDirectory(filepath.Dir(path)); err != nil {
				return err
			}
		}
		info, err := os.Lstat(path)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("shared runtime private directory is unsafe")
		}
	}
	return nil
}

func readJunimo140GroupJSON(path string, target any) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() > 2*1024*1024 {
		return errors.New("shared runtime private record is unsafe")
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, 2*1024*1024+1))
	if err != nil || len(raw) > 2*1024*1024 || !utf8.Valid(raw) {
		return errors.New("shared runtime private record cannot be read")
	}
	keys := json.NewDecoder(bytes.NewReader(raw))
	if err := uniqueJunimo140GroupJSONKeys(keys, 0); err != nil {
		return errors.New("shared runtime private record has duplicate or invalid fields")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return errors.New("shared runtime private record has an unknown or invalid schema")
	}
	if decoder.Decode(new(any)) != io.EOF {
		return errors.New("shared runtime private record has trailing content")
	}
	return nil
}

func uniqueJunimo140GroupJSONKeys(decoder *json.Decoder, depth int) error {
	if depth > 64 {
		return errors.New("private record nesting exceeds limit")
	}
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, nested := token.(json.Delim)
	if !nested {
		return nil
	}
	switch delimiter {
	case '{':
		seen := map[string]bool{}
		for decoder.More() {
			key, err := decoder.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			canonical := strings.ToLower(name)
			if !ok || seen[canonical] {
				return errors.New("duplicate private record key")
			}
			seen[canonical] = true
			if err := uniqueJunimo140GroupJSONKeys(decoder, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			if err := uniqueJunimo140GroupJSONKeys(decoder, depth+1); err != nil {
				return err
			}
		}
	default:
		return errors.New("invalid private record delimiter")
	}
	_, err = decoder.Token()
	return err
}

func readJunimo140GroupLease(dataDir string) (junimo140GroupLease, error) {
	var lease junimo140GroupLease
	if err := ensureExistingJunimo140GroupParent(dataDir); err != nil {
		return lease, err
	}
	err := readJunimo140GroupJSON(junimo140GroupLeasePath(dataDir), &lease)
	if err != nil {
		return lease, err
	}
	if lease.SchemaVersion != 1 || !junimo140GroupApplyID.MatchString(lease.ApplyID) || lease.PrimaryID == "" || !junimo140GroupSHA256.MatchString(lease.PlanSHA256) {
		return lease, errors.New("shared runtime lease identity is invalid")
	}
	return lease, nil
}

func ensureExistingJunimo140GroupParent(dataDir string) error {
	if err := plainJunimoGroupRoot(dataDir); err != nil {
		return err
	}
	info, err := os.Lstat(filepath.Join(dataDir, ".local-container"))
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("shared runtime lease directory is unsafe")
	}
	return nil
}

func knownJunimo140GroupPhase(phase string) bool {
	switch phase {
	case "claiming", "claimed", "quiescing", "backing_up", "preparing_game", "updating_worlds", "restoring_state", "committed", "rolling_back", "rolled_back", "rollback_failed", "claim_failed":
		return true
	default:
		return false
	}
}

func writeJunimo140GroupJournal(journal *junimo140GroupJournal, phase string) error {
	if !knownJunimo140GroupPhase(phase) {
		return errors.New("unknown shared runtime transaction phase")
	}
	if journal.SchemaVersion != 1 || journal.Plan.SchemaVersion != 1 || journal.PlanSHA256 != journal.Plan.digest() {
		return errors.New("shared runtime plan identity changed")
	}
	path, err := junimo140GroupJournalPath(journal.Plan)
	if err != nil {
		return err
	}
	primary, _ := journal.Plan.primary()
	if err := ensureJunimo140GroupDirectories(primary.DataDir, ".local-container", "junimo140-group", journal.Plan.ApplyID); err != nil {
		return err
	}
	journal.Phase, journal.UpdatedAt = phase, time.Now().UTC().Format(time.RFC3339Nano)
	if err := writePrivateRuntimeUpdateJSON(path, journal); err != nil {
		return err
	}
	return syncNewGameOwnerDirectory(filepath.Dir(path))
}

// A peer lease resolves through the current store, not a path supplied by the
// lease. Rediscovery accepts only the original or transaction staging volume,
// allowing a crash during the per-world switch while rejecting new members,
// removed members, changed roots and unrelated game-volume bindings.
func readRecoverableJunimo140Group(instances []storage.Instance, caller registry.Instance) (junimo140GroupJournal, error) {
	var journal junimo140GroupJournal
	if caller.ID == "" || caller.DriverID != DriverID {
		return journal, errors.New("invalid shared runtime recovery caller")
	}
	lease, err := readJunimo140GroupLease(caller.DataDir)
	if err != nil {
		return journal, err
	}
	var primary registry.Instance
	for _, instance := range instances {
		if instance.ID == lease.PrimaryID {
			if primary.ID != "" {
				return journal, errors.New("shared runtime primary identity is ambiguous")
			}
			primary = registry.Instance{ID: instance.ID, DriverID: instance.DriverID, DataDir: instance.DataDir}
		}
	}
	if primary.ID == "" || primary.DriverID != DriverID {
		return journal, errors.New("shared runtime primary is no longer available")
	}
	if err := plainJunimoGroupRoot(primary.DataDir); err != nil {
		return journal, err
	}
	path := primary.DataDir
	for _, part := range []string{".local-container", "junimo140-group", lease.ApplyID} {
		path = filepath.Join(path, part)
		info, err := os.Lstat(path)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return journal, errors.New("shared runtime journal directory is unavailable or unsafe")
		}
	}
	if err := readJunimo140GroupJSON(filepath.Join(path, "manifest.json"), &journal); err != nil {
		return journal, err
	}
	if journal.SchemaVersion != 1 || journal.Plan.SchemaVersion != 1 || !knownJunimo140GroupPhase(journal.Phase) || journal.Plan.ApplyID != lease.ApplyID || journal.Plan.PrimaryID != lease.PrimaryID || journal.PlanSHA256 != lease.PlanSHA256 || journal.Plan.digest() != lease.PlanSHA256 {
		return journal, errors.New("shared runtime journal does not match its peer lease")
	}
	transaction, transactionErr := readJunimo140GroupTransaction(journal.Plan)
	if transactionErr == nil {
		if err := validateJunimo140RecoveryMembership(instances, journal.Plan, transaction); err != nil {
			return journal, err
		}
		if (journal.Phase == "committed" || journal.Phase == "rolled_back") && journal.Phase != transaction.Phase {
			return journal, errors.New("shared runtime terminal owner and transaction disagree")
		}
		terminal := (journal.Phase == "committed" || journal.Phase == "rolled_back" || journal.Phase == "claim_failed") && journal.Phase == transaction.Phase
		for _, member := range journal.Plan.Members {
			actual, err := readJunimo140GroupLease(member.DataDir)
			if errors.Is(err, os.ErrNotExist) && terminal {
				continue
			}
			if err != nil || actual != lease {
				return journal, errors.New("shared runtime peer lease changed; no automatic restoration")
			}
		}
	} else if errors.Is(transactionErr, os.ErrNotExist) {
		current, err := discoverJunimo140GroupPlan(instances, primary, lease.ApplyID, journal.Plan.SourceVolume, journal.Plan.StagingVolume)
		if err != nil || current.digest() != journal.PlanSHA256 {
			return journal, errors.New("shared runtime membership changed; recovery requires manual review")
		}
	} else {
		return journal, transactionErr
	}
	for _, member := range journal.Plan.Members {
		if member.ID == caller.ID && sameJunimoGroupPath(member.DataDir, caller.DataDir) {
			return journal, nil
		}
	}
	return journal, errors.New("requesting world does not belong to this shared runtime transaction")
}

func claimJunimo140Group(plan junimo140GroupPlan) (journal junimo140GroupJournal, resultErr error) {
	primary, err := plan.primary()
	if err != nil || plan.SchemaVersion != 1 || len(plan.Members) == 0 || !junimo140GroupApplyID.MatchString(plan.ApplyID) {
		return journal, errors.New("invalid shared runtime claim")
	}
	if err := ensureJunimo140GroupDirectories(primary.DataDir, ".local-container", "junimo140-group"); err != nil {
		return journal, err
	}
	dir := filepath.Join(primary.DataDir, ".local-container", "junimo140-group", plan.ApplyID)
	// An apply identity is immutable. Never reuse/overwrite another attempt's
	// journal, including a previous incomplete claim.
	if err := os.Mkdir(dir, 0o700); err != nil {
		return journal, err
	}
	if err := syncNewGameOwnerDirectory(filepath.Dir(dir)); err != nil {
		return journal, err
	}
	journal = junimo140GroupJournal{SchemaVersion: 1, Plan: plan, PlanSHA256: plan.digest()}
	if err := writeJunimo140GroupJournal(&journal, "claiming"); err != nil {
		return journal, err
	}
	lease := junimo140GroupLease{SchemaVersion: 1, ApplyID: plan.ApplyID, PrimaryID: plan.PrimaryID, PlanSHA256: journal.PlanSHA256}
	raw, _ := json.Marshal(lease)
	var claimed []junimo140GroupMember
	defer func() {
		if resultErr == nil {
			return
		}
		// Only this call's proven leases may be released on a failed claim.
		for _, member := range claimed {
			if err := removeJunimo140GroupLease(member.DataDir, lease); err != nil {
				resultErr = errors.Join(resultErr, err)
			}
		}
		if err := writeJunimo140GroupJournal(&journal, "claim_failed"); err != nil {
			resultErr = errors.Join(resultErr, err)
		}
	}()
	for _, member := range plan.Members {
		if err := ensureJunimo140GroupDirectories(member.DataDir, ".local-container"); err != nil {
			return journal, err
		}
		file, err := os.OpenFile(junimo140GroupLeasePath(member.DataDir), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return journal, errors.New("a shared runtime world already has an owner")
		}
		claimed = append(claimed, member)
		_, writeErr := file.Write(raw)
		syncErr := file.Sync()
		closeErr := file.Close()
		if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
			return journal, err
		}
		if err := syncNewGameOwnerDirectory(filepath.Dir(junimo140GroupLeasePath(member.DataDir))); err != nil {
			return journal, err
		}
	}
	if err := writeJunimo140GroupJournal(&journal, "claimed"); err != nil {
		return journal, err
	}
	return journal, nil
}

func removeJunimo140GroupLease(dataDir string, expected junimo140GroupLease) error {
	actual, err := readJunimo140GroupLease(dataDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil || actual != expected {
		return errors.New("shared runtime lease changed; recovery materials retained")
	}
	if err := os.Remove(junimo140GroupLeasePath(dataDir)); err != nil {
		return err
	}
	return syncNewGameOwnerDirectory(filepath.Dir(junimo140GroupLeasePath(dataDir)))
}

func releaseJunimo140Group(journal junimo140GroupJournal) error {
	if journal.SchemaVersion != 1 || journal.Plan.SchemaVersion != 1 || journal.PlanSHA256 != journal.Plan.digest() || (journal.Phase != "committed" && journal.Phase != "rolled_back" && journal.Phase != "claim_failed") {
		return errors.New("shared runtime group cannot be released before a durable terminal result")
	}
	path, err := junimo140GroupJournalPath(journal.Plan)
	if err != nil {
		return err
	}
	var stored junimo140GroupJournal
	if err := readJunimo140GroupJSON(path, &stored); err != nil {
		return err
	}
	if stored.SchemaVersion != 1 || stored.PlanSHA256 != journal.PlanSHA256 || stored.Plan.digest() != journal.PlanSHA256 || stored.Phase != journal.Phase {
		return errors.New("shared runtime terminal result is not durably confirmed")
	}
	lease := junimo140GroupLease{SchemaVersion: 1, ApplyID: journal.Plan.ApplyID, PrimaryID: journal.Plan.PrimaryID, PlanSHA256: journal.PlanSHA256}
	// Validate the full holder set before removing the first peer's lease.
	for _, member := range journal.Plan.Members {
		actual, err := readJunimo140GroupLease(member.DataDir)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil || actual != lease {
			return errors.New("shared runtime lease changed; group remains protected")
		}
	}
	for _, member := range journal.Plan.Members {
		if err := removeJunimo140GroupLease(member.DataDir, lease); err != nil {
			return err
		}
	}
	return nil
}
