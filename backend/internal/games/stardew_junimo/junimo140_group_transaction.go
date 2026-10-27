package stardew_junimo

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

// Capture before stopping any peer. This immutable part includes the database
// state and exact image identities needed even when .env is absent mid-restore.
type junimo140WorldOriginal struct {
	ControlDLLSHA256      string                       `json:"controlDllSha256,omitempty"`
	ControlManifestSHA256 string                       `json:"controlManifestSha256,omitempty"`
	JunimoDLLSHA256       string                       `json:"junimoDllSha256,omitempty"`
	Instance              storage.Instance             `json:"instance"`
	Current               sjconfig.RuntimeStackCurrent `json:"current"`
	Server                RuntimeUpdateSelectedImage   `json:"server"`
	Auth                  RuntimeUpdateSelectedImage   `json:"auth"`
	ServerPresent         bool                         `json:"serverPresent"`
	AuthPresent           bool                         `json:"authPresent"`
	ServerContainerID     string                       `json:"serverContainerId,omitempty"`
	AuthContainerID       string                       `json:"authContainerId,omitempty"`
	ServerWasRunning      bool                         `json:"serverWasRunning"`
	AuthWasRunning        bool                         `json:"authWasRunning"`
	HasWorld              bool                         `json:"hasWorld"`
	SteamInviteEnabled    bool                         `json:"steamInviteEnabled"`
	SessionVolume         string                       `json:"sessionVolume,omitempty"`
	EnvSHA256             string                       `json:"envSha256"`
	ComposeSHA256         string                       `json:"composeSha256"`
}

type junimo140WorldProgress struct {
	Original          junimo140WorldOriginal `json:"original"`
	ChildApplyID      string                 `json:"childApplyId"`
	AuthArchive       string                 `json:"authArchive,omitempty"`
	SaveCommandID     string                 `json:"saveCommandId,omitempty"`
	SaveIntent        bool                   `json:"saveIntent,omitempty"`
	Saved             bool                   `json:"saved,omitempty"`
	StopIntent        bool                   `json:"stopIntent,omitempty"`
	Stopped           bool                   `json:"stopped,omitempty"`
	ArchiveSHA256     string                 `json:"archiveSha256,omitempty"`
	AuthArchiveSHA256 string                 `json:"authArchiveSha256,omitempty"`
	UpdateIntent      bool                   `json:"updateIntent,omitempty"`
	Updated           bool                   `json:"updated,omitempty"`
	RestoreIntent     bool                   `json:"restoreIntent,omitempty"`
	Restored          bool                   `json:"restored,omitempty"`
}

// transaction.json is the authoritative mutation checkpoint. The separate
// manifest/peer leases retain the immutable owner while a full root is restored.
type junimo140GroupTransaction struct {
	SchemaVersion       int                                 `json:"schemaVersion"`
	PlanSHA256          string                              `json:"planSha256"`
	InputSHA256         string                              `json:"inputSha256"`
	ActorID             int64                               `json:"actorId"`
	JobID               string                              `json:"jobId"`
	Recommendation      sjconfig.RuntimeStackRecommendation `json:"recommendation"`
	Target              RuntimeUpdateSelectedPair           `json:"target"`
	Worlds              []junimo140WorldProgress            `json:"worlds"`
	Phase               string                              `json:"phase"`
	StageCreateIntent   bool                                `json:"stageCreateIntent,omitempty"`
	StageCreated        bool                                `json:"stageCreated,omitempty"`
	StageReady          bool                                `json:"stageReady,omitempty"`
	StageRemoved        bool                                `json:"stageRemoved,omitempty"`
	KeepServerStopped   bool                                `json:"keepServerStopped,omitempty"`
	WorldChangesStarted bool                                `json:"worldChangesStarted,omitempty"`
	UpdatedAt           string                              `json:"updatedAt"`
}

var junimo140SavedCommandPattern = regexp.MustCompile(`^(cmd_)?[a-f0-9]{32}$`)
var junimo140GroupJobPattern = regexp.MustCompile(`^job_[a-f0-9]{32}$`)
var junimo140TransactionArchivePattern = regexp.MustCompile(`^[a-f0-9]{12}[.]tar$`)

// Manifest components normalize derived fields in UnmarshalJSON. Validate their
// persisted wire shapes separately so that normalization cannot hide unknown
// recovery instructions nested under recommendation metadata.
func (txn *junimo140GroupTransaction) UnmarshalJSON(data []byte) error {
	type transactionWire junimo140GroupTransaction
	var decoded transactionWire
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil {
		return err
	}
	var raw struct {
		Recommendation json.RawMessage `json:"recommendation"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	type recommendationWire sjconfig.RuntimeStackRecommendation
	type componentWire sjconfig.RuntimeStackManifestComponent
	type smapiWire sjconfig.SMAPIManifestComponent
	var recommendation struct {
		recommendationWire
		Server    componentWire `json:"server"`
		SteamAuth componentWire `json:"steamAuth"`
		SMAPI     smapiWire     `json:"smapi"`
	}
	strict := json.NewDecoder(bytes.NewReader(raw.Recommendation))
	strict.DisallowUnknownFields()
	if err := strict.Decode(&recommendation); err != nil {
		return err
	}
	*txn = junimo140GroupTransaction(decoded)
	return nil
}

func junimo140ChildApplyID(parent, member string) string {
	hash := sha256.Sum256([]byte(parent + "\nchild\n" + member))
	return "apply_" + hex.EncodeToString(hash[:12])
}

func junimo140AuthArchiveName(member string) string {
	hash := sha256.Sum256([]byte("auth\n" + member))
	return hex.EncodeToString(hash[:6]) + ".tar"
}

func (txn junimo140GroupTransaction) inputDigest() string {
	originals := make([]junimo140WorldOriginal, len(txn.Worlds))
	for i, world := range txn.Worlds {
		originals[i] = world.Original
	}
	raw, _ := json.Marshal(struct {
		Plan           string
		Actor          int64
		Job            string
		Recommendation sjconfig.RuntimeStackRecommendation
		Target         RuntimeUpdateSelectedPair
		Originals      []junimo140WorldOriginal
	}{txn.PlanSHA256, txn.ActorID, txn.JobID, txn.Recommendation, txn.Target, originals})
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:])
}

func validateJunimo140GroupTransaction(plan junimo140GroupPlan, txn junimo140GroupTransaction) error {
	if plan.SchemaVersion != 1 || !junimo140GroupApplyID.MatchString(plan.ApplyID) || txn.SchemaVersion != 1 || txn.PlanSHA256 != plan.digest() || !junimo140GroupSHA256.MatchString(txn.InputSHA256) || txn.inputDigest() != txn.InputSHA256 || txn.ActorID < 0 || !junimo140GroupJobPattern.MatchString(txn.JobID) || !knownJunimo140GroupPhase(txn.Phase) || len(txn.Worlds) != len(plan.Members) || len(txn.Worlds) == 0 || len(txn.Worlds) > 256 {
		return errors.New("shared runtime transaction identity is invalid")
	}
	if txn.Recommendation.Server.Tag != "1.5.0-preview.140" || txn.Target.Server.Digest != junimo140Digest || !runtimeImageDigestPattern.MatchString(txn.Target.Server.ImageID) || txn.Recommendation.Server.Digests[txn.Target.Server.Image] != junimo140Digest {
		return errors.New("shared runtime transaction target is not the fixed Junimo build")
	}
	archives := map[string]bool{}
	ids := map[string]bool{}
	for _, member := range plan.Members {
		if member.ID == "" || ids[member.ID] || !junimo140TransactionArchivePattern.MatchString(member.Archive) || archives[member.Archive] {
			return errors.New("shared runtime member or archive identity is ambiguous")
		}
		ids[member.ID] = true
		archives[member.Archive] = true
	}
	allBackedUp, needsAuth := true, false
	if txn.KeepServerStopped && (!junimo140GroupHasStopIntent(txn) || txn.Phase != "rolling_back" && txn.Phase != "rollback_failed" && txn.Phase != "rolled_back") {
		return errors.New("shared runtime stopped recovery policy has no rollback stop intent")
	}
	if txn.Phase == "claim_failed" && (txn.StageCreateIntent || txn.WorldChangesStarted) {
		return errors.New("shared runtime failed claim contains mutation intent")
	}
	for i, world := range txn.Worlds {
		member := plan.Members[i]
		original := world.Original
		for _, hash := range []string{original.ControlDLLSHA256, original.ControlManifestSHA256, original.JunimoDLLSHA256} {
			if hash != "" && !junimo140GroupSHA256.MatchString(hash) {
				return errors.New("shared runtime original artifact checksum is invalid")
			}
		}
		if txn.Phase == "claim_failed" && (world.SaveIntent || world.StopIntent || world.UpdateIntent || world.RestoreIntent) {
			return errors.New("shared runtime failed claim contains member mutation intent")
		}
		needsAuth = needsAuth || original.SteamInviteEnabled
		if original.Instance.ID != member.ID || original.Instance.DriverID != DriverID || !sameJunimoGroupPath(original.Instance.DataDir, member.DataDir) || world.ChildApplyID != junimo140ChildApplyID(plan.ApplyID, member.ID) || world.ChildApplyID == plan.ApplyID || !junimo140GroupSHA256.MatchString(original.EnvSHA256) || !junimo140GroupSHA256.MatchString(original.ComposeSHA256) {
			return errors.New("shared runtime member original identity is invalid")
		}
		if original.ServerWasRunning && !original.ServerPresent || original.AuthWasRunning && !original.AuthPresent || original.ServerPresent && !runtimeImageDigestPattern.MatchString(original.Server.ImageID) || original.AuthPresent && !runtimeImageDigestPattern.MatchString(original.Auth.ImageID) {
			return errors.New("shared runtime member image or running state is invalid")
		}
		for _, containerID := range []string{original.ServerContainerID, original.AuthContainerID} {
			if containerID != "" && !junimo140GroupSHA256.MatchString(containerID) {
				return errors.New("shared runtime original container identity is invalid")
			}
		}
		if original.AuthPresent && original.SessionVolume == "" {
			return errors.New("shared runtime authentication container has no protected session scope")
		}
		if original.SessionVolume != "" {
			if !gameDataVolumeNamePattern.MatchString(original.SessionVolume) || original.SessionVolume == plan.SourceVolume || original.SessionVolume == plan.StagingVolume || world.AuthArchive != junimo140AuthArchiveName(member.ID) || archives[world.AuthArchive] {
				return errors.New("shared runtime authentication backup identity is invalid")
			}
			archives[world.AuthArchive] = true
		} else if world.AuthArchive != "" || world.AuthArchiveSHA256 != "" {
			return errors.New("unscoped shared runtime authentication backup")
		}
		if world.SaveIntent && (!original.ServerWasRunning || !original.HasWorld || !junimo140SavedCommandPattern.MatchString(world.SaveCommandID)) || world.Saved && !world.SaveIntent || !world.SaveIntent && world.SaveCommandID != "" {
			return errors.New("shared runtime durable save intent is invalid")
		}
		if world.Stopped && !world.StopIntent || world.Updated && !world.UpdateIntent || world.Restored && !world.RestoreIntent {
			return errors.New("shared runtime member completion has no durable intent")
		}
		if world.StopIntent && original.ServerWasRunning && original.HasWorld && !world.Saved {
			return errors.New("shared runtime stop precedes durable save")
		}
		if world.ArchiveSHA256 != "" && !junimo140GroupSHA256.MatchString(world.ArchiveSHA256) || world.AuthArchiveSHA256 != "" && !junimo140GroupSHA256.MatchString(world.AuthArchiveSHA256) {
			return errors.New("shared runtime archive checksum is invalid")
		}
		backed := world.Stopped && world.ArchiveSHA256 != "" && (original.SessionVolume == "" || world.AuthArchiveSHA256 != "")
		allBackedUp = allBackedUp && backed
		if (world.UpdateIntent || world.RestoreIntent) && (!txn.WorldChangesStarted || !backed) {
			return errors.New("shared runtime member mutation precedes complete backup")
		}
	}
	if txn.StageCreated && !txn.StageCreateIntent || txn.StageReady && !txn.StageCreated || txn.StageCreateIntent && !allBackedUp || txn.WorldChangesStarted && (!allBackedUp || !txn.StageReady) {
		return errors.New("shared runtime game preparation precedes complete group backup")
	}
	if txn.StageRemoved && (!txn.StageCreateIntent || txn.Phase != "rolling_back" && txn.Phase != "rollback_failed" && txn.Phase != "rolled_back") {
		return errors.New("shared runtime stage removal has no rollback intent")
	}
	if needsAuth && (!runtimeImageDigestPattern.MatchString(txn.Target.SteamAuth.ImageID) || !runtimeImageDigestPattern.MatchString(txn.Target.SteamAuth.Digest) || txn.Recommendation.SteamAuth.Digests[txn.Target.SteamAuth.Image] != txn.Target.SteamAuth.Digest) {
		return errors.New("shared runtime authentication target identity is invalid")
	}
	if txn.Phase == "committed" {
		for _, world := range txn.Worlds {
			if !world.Updated {
				return errors.New("shared runtime commit precedes complete member acceptance")
			}
		}
	}
	if txn.Phase == "rolled_back" && txn.WorldChangesStarted {
		for _, world := range txn.Worlds {
			if !world.Restored {
				return errors.New("shared runtime terminal rollback precedes complete file restoration")
			}
		}
	}
	return nil
}

func junimo140GroupTransactionPath(plan junimo140GroupPlan) (string, error) {
	journal, err := junimo140GroupJournalPath(plan)
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(journal), "transaction.json"), nil
}

func readJunimo140GroupTransaction(plan junimo140GroupPlan) (junimo140GroupTransaction, error) {
	var txn junimo140GroupTransaction
	if err := existingJunimo140TransactionDirectory(plan); err != nil {
		return txn, err
	}
	path, err := junimo140GroupTransactionPath(plan)
	if err != nil {
		return txn, err
	}
	if err := readJunimo140GroupJSON(path, &txn); err != nil {
		return txn, err
	}
	if _, err := time.Parse(time.RFC3339Nano, txn.UpdatedAt); err != nil {
		return txn, errors.New("shared runtime transaction timestamp is invalid")
	}
	return txn, validateJunimo140GroupTransaction(plan, txn)
}

func writeJunimo140GroupTransaction(plan junimo140GroupPlan, txn *junimo140GroupTransaction) error {
	if err := validateJunimo140GroupTransaction(plan, *txn); err != nil {
		return err
	}
	if err := existingJunimo140TransactionDirectory(plan); err != nil {
		return err
	}
	ownerPath, err := junimo140GroupJournalPath(plan)
	if err != nil {
		return err
	}
	var owner junimo140GroupJournal
	if err := readJunimo140GroupJSON(ownerPath, &owner); err != nil {
		return err
	}
	if owner.SchemaVersion != 1 || owner.PlanSHA256 != plan.digest() || owner.Plan.digest() != plan.digest() || owner.Phase == "committed" || owner.Phase == "rolled_back" || owner.Phase == "claim_failed" {
		return errors.New("shared runtime transaction has no active immutable owner")
	}
	expectedLease := junimo140GroupLease{SchemaVersion: 1, ApplyID: plan.ApplyID, PrimaryID: plan.PrimaryID, PlanSHA256: plan.digest()}
	for _, member := range plan.Members {
		lease, err := readJunimo140GroupLease(member.DataDir)
		if err != nil || lease != expectedLease {
			return errors.New("shared runtime member lease changed before checkpoint")
		}
	}
	path, err := junimo140GroupTransactionPath(plan)
	if err != nil {
		return err
	}
	if prior, err := readJunimo140GroupTransaction(plan); err == nil {
		if prior.InputSHA256 != txn.InputSHA256 {
			return errors.New("shared runtime transaction inputs changed")
		}
		if err := validateJunimo140TransactionProgress(prior, *txn); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	txn.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	raw, err := json.Marshal(txn)
	if err != nil || len(raw) > 2*1024*1024 {
		return errors.New("shared runtime transaction exceeds its private record bound")
	}
	if err := writePrivateRuntimeUpdateJSON(path, txn); err != nil {
		return err
	}
	return syncNewGameOwnerDirectory(filepath.Dir(path))
}

func existingJunimo140TransactionDirectory(plan junimo140GroupPlan) error {
	if !junimo140GroupApplyID.MatchString(plan.ApplyID) {
		return errors.New("invalid shared runtime transaction directory identity")
	}
	primary, err := plan.primary()
	if err != nil {
		return err
	}
	if err := ensureExistingJunimo140GroupParent(primary.DataDir); err != nil {
		return err
	}
	path := filepath.Join(primary.DataDir, ".local-container")
	for _, part := range []string{"junimo140-group", plan.ApplyID} {
		path = filepath.Join(path, part)
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("shared runtime transaction parent is unsafe")
		}
	}
	return nil
}

func validateJunimo140TransactionProgress(before, after junimo140GroupTransaction) error {
	if before.Phase == "committed" || before.Phase == "rolled_back" || before.Phase == "claim_failed" {
		return errors.New("shared runtime terminal transaction is immutable")
	}
	regressed := func(old, current bool) bool { return old && !current }
	if regressed(before.StageCreateIntent, after.StageCreateIntent) || regressed(before.StageCreated, after.StageCreated) || regressed(before.StageReady, after.StageReady) || regressed(before.StageRemoved, after.StageRemoved) || regressed(before.KeepServerStopped, after.KeepServerStopped) || regressed(before.WorldChangesStarted, after.WorldChangesStarted) {
		return errors.New("shared runtime group mutation evidence regressed")
	}
	for i, old := range before.Worlds {
		current := after.Worlds[i]
		if regressed(old.SaveIntent, current.SaveIntent) || regressed(old.Saved, current.Saved) || regressed(old.StopIntent, current.StopIntent) || regressed(old.Stopped, current.Stopped) || regressed(old.UpdateIntent, current.UpdateIntent) || regressed(old.Updated, current.Updated) || regressed(old.RestoreIntent, current.RestoreIntent) || regressed(old.Restored, current.Restored) || old.SaveCommandID != "" && old.SaveCommandID != current.SaveCommandID || old.ArchiveSHA256 != "" && old.ArchiveSHA256 != current.ArchiveSHA256 || old.AuthArchiveSHA256 != "" && old.AuthArchiveSHA256 != current.AuthArchiveSHA256 {
			return errors.New("shared runtime member mutation evidence regressed")
		}
	}
	return nil
}

func (plan junimo140GroupPlan) membershipDigest() string {
	plan.ApplyID, plan.StagingVolume = "", ""
	plan.Members = append([]junimo140GroupMember(nil), plan.Members...)
	for i := range plan.Members {
		plan.Members[i].Archive = ""
	}
	raw, _ := json.Marshal(plan)
	sum := sha256.Sum256(raw)
	return strings.ToLower(hex.EncodeToString(sum[:]))
}
