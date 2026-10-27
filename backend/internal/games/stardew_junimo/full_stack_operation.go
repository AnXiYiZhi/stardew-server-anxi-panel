package stardew_junimo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"time"

	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/updater"
)

var fullStackOperationPattern = regexp.MustCompile(`^stack_[a-f0-9]{32}$`)

type FullStackGroupStatus struct {
	MemberNames   map[string]string `json:"memberNames"`
	RunningWorlds int               `json:"runningWorlds"`
	ColdTemplates int               `json:"coldTemplates"`
	SavedWorlds   int               `json:"savedWorlds"`
	StoppedWorlds int               `json:"stoppedWorlds"`
	UpdatedWorlds int               `json:"updatedWorlds"`
	PrimaryID     string            `json:"primaryId"`
	Members       []string          `json:"members"`
	ScopeSHA256   string            `json:"scopeSha256"`
	ApplyID       string            `json:"applyId"`
	Phase         string            `json:"phase"`
	ErrorCode     string            `json:"errorCode,omitempty"`
}

// This is the complete admin-facing representation; filesystem paths,
// original payloads and container identities stay in the private operation.
type FullStackUpgradeStatus struct {
	RuntimeDigest  string                 `json:"runtimeDigest"`
	ControlSHA256  string                 `json:"controlSha256"`
	GameBuildID    string                 `json:"gameBuildId"`
	SDKBuildID     string                 `json:"sdkBuildId"`
	SMAPIIdentity  string                 `json:"smapiIdentity"`
	AuthVersion    string                 `json:"authVersion,omitempty"`
	AuthDigest     string                 `json:"authDigest,omitempty"`
	OperationID    string                 `json:"operationId"`
	PlanSHA256     string                 `json:"planSha256"`
	Phase          string                 `json:"phase"`
	ActorID        int64                  `json:"actorId"`
	Panel          updater.PanelTarget    `json:"panel"`
	RuntimeVersion string                 `json:"runtimeVersion"`
	ControlVersion string                 `json:"controlVersion"`
	CreatedAt      time.Time              `json:"createdAt"`
	ExpiresAt      time.Time              `json:"expiresAt"`
	AuthorizedAt   *time.Time             `json:"authorizedAt,omitempty"`
	PanelUpdateID  string                 `json:"panelUpdateId,omitempty"`
	Continuation   bool                   `json:"continuation"`
	Groups         []FullStackGroupStatus `json:"groups"`
	ErrorCode      string                 `json:"errorCode,omitempty"`
	Error          string                 `json:"error,omitempty"`
	JobID          string                 `json:"jobId,omitempty"`
}

type fullStackPreparedGroup struct {
	SaveIDs  map[string]string         `json:"saveIds"`
	Plan     junimo140GroupPlan        `json:"plan"`
	Original junimo140GroupTransaction `json:"original"`
	DryRun   RuntimeUpdateDryRunStatus `json:"dryRun"`
}

type fullStackOperation struct {
	Schema int                                 `json:"schema"`
	Public FullStackUpgradeStatus              `json:"public"`
	Target sjconfig.RuntimeStackRecommendation `json:"target"`
	Groups []fullStackPreparedGroup            `json:"groups"`
}

type fullStackContextKey struct{}

func fullStackTerminal(phase string) bool {
	return phase == "succeeded" || phase == "failed_safe" || phase == "manual_action" || phase == "preflight_failed" || phase == "panel_failed_rolled_back"
}

func (d *Driver) FullStackUpgradeAvailable() bool {
	policy, err := sjconfig.BuiltInRuntimeUpgradePolicy()
	return err == nil && (policy.RequiresScopeConfirmation || policy.TargetPanelVersion != "") && filepath.IsAbs(d.containerDataDir) && d.jobs != nil && d.store != nil
}

func (d *Driver) FullStackUpgradeAppliesToPanelVersion(current, version string) bool {
	policy, err := sjconfig.BuiltInRuntimeUpgradePolicy()
	return err == nil && policy.AppliesToPanelVersion(current, version)
}

func (d *Driver) fullStackRoot() (string, error) {
	if !filepath.IsAbs(d.containerDataDir) {
		return "", errors.New("full-stack operation requires a persistent Panel data directory")
	}
	return filepath.Join(d.containerDataDir, "updater", "full-stack"), nil
}

func (op fullStackOperation) digest() string {
	// Only immutable authorization inputs participate. Runtime and Panel job
	// progress cannot silently change what the administrator confirmed.
	input := struct {
		ID               string
		Actor            int64
		Panel            updater.PanelTarget
		Target           sjconfig.RuntimeStackRecommendation
		Groups           []fullStackPreparedGroup
		Created, Expires time.Time
		Continuation     bool
	}{op.Public.OperationID, op.Public.ActorID, op.Public.Panel, op.Target, op.Groups, op.Public.CreatedAt, op.Public.ExpiresAt, op.Public.Continuation}
	raw, _ := json.Marshal(input)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func (op fullStackOperation) validateGroups() error {
	if len(op.Groups) != len(op.Public.Groups) || len(op.Groups) > 256 {
		return errors.New("full-stack group representation is incomplete")
	}
	seen := map[string]bool{}
	for i, group := range op.Groups {
		view := op.Public.Groups[i]
		if validateJunimo140GroupTransaction(group.Plan, group.Original) != nil || group.Original.ActorID != op.Public.ActorID || view.ApplyID != group.Plan.ApplyID || view.PrimaryID != group.Plan.PrimaryID || view.ScopeSHA256 != group.Plan.membershipDigest() || len(view.Members) != len(group.Plan.Members) {
			return errors.New("full-stack group does not match the original authorization")
		}
		if group.Original.Recommendation.Server.Tag != op.Target.Server.Tag || group.Original.Recommendation.Control.DLLSHA256 != op.Target.Control.DLLSHA256 {
			return errors.New("full-stack component identity differs between groups")
		}
		for n, member := range group.Plan.Members {
			if seen[member.ID] || view.Members[n] != member.ID {
				return errors.New("full-stack member scope is duplicated or changed")
			}
			seen[member.ID] = true
		}
	}
	if len(seen) > 256 {
		return errors.New("full-stack member limit exceeded")
	}
	return nil
}

func (d *Driver) writeFullStackOperation(op fullStackOperation) error {
	root, err := d.fullStackRoot()
	if err != nil {
		return err
	}
	if !fullStackOperationPattern.MatchString(op.Public.OperationID) {
		return errors.New("invalid full-stack operation identity")
	}
	if op.Schema != 1 || !op.Public.Panel.Valid() || op.Public.ActorID < 1 {
		return errors.New("invalid full-stack operation identity")
	}
	path := filepath.Join(root, op.Public.OperationID+".json")
	if raw, err := os.ReadFile(filepath.Join(root, "current.json")); err == nil {
		var pointer struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(raw, &pointer) != nil || !fullStackOperationPattern.MatchString(pointer.ID) {
			return errors.New("existing full-stack operation pointer is invalid")
		}
		if pointer.ID != op.Public.OperationID {
			if _, err := os.Stat(path); err == nil {
				return errors.New("superseded full-stack operation cannot replace the current operation")
			} else if !errors.Is(err, os.ErrNotExist) {
				return err
			}
			current, err := d.readFullStackOperation()
			if err != nil {
				return err
			}
			if current.Public.Phase == "preparing" || current.Public.AuthorizedAt != nil && (!fullStackTerminal(current.Public.Phase) || current.Public.Phase == "manual_action") {
				return errors.New("current full-stack operation still owns its work")
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if raw, err := os.ReadFile(path); err == nil {
		var previous fullStackOperation
		if json.Unmarshal(raw, &previous) != nil {
			return errors.New("existing full-stack operation is invalid")
		}
		if previous.Public.AuthorizedAt != nil && (previous.Public.PlanSHA256 != op.Public.PlanSHA256 || previous.digest() != op.digest() || op.Public.AuthorizedAt == nil || !previous.Public.AuthorizedAt.Equal(*op.Public.AuthorizedAt)) {
			return errors.New("confirmed full-stack authorization cannot be replaced")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := op.validateGroups(); err != nil {
		return err
	}
	if op.Public.PlanSHA256 != "" && op.Public.PlanSHA256 != op.digest() {
		return errors.New("full-stack immutable inputs changed")
	}
	if err := writePrivateRuntimeUpdateJSON(path, op); err != nil {
		return err
	}
	return writePrivateRuntimeUpdateJSON(filepath.Join(root, "current.json"), struct {
		ID string `json:"id"`
	}{op.Public.OperationID})
}

func (d *Driver) readFullStackOperation() (fullStackOperation, error) {
	root, err := d.fullStackRoot()
	if err != nil {
		return fullStackOperation{}, err
	}
	raw, err := os.ReadFile(filepath.Join(root, "current.json"))
	if err != nil {
		return fullStackOperation{}, err
	}
	var pointer struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(raw, &pointer) != nil || !fullStackOperationPattern.MatchString(pointer.ID) {
		return fullStackOperation{}, errors.New("invalid full-stack operation pointer")
	}
	raw, err = os.ReadFile(filepath.Join(root, pointer.ID+".json"))
	if err != nil {
		return fullStackOperation{}, err
	}
	var op fullStackOperation
	if json.Unmarshal(raw, &op) != nil || op.Schema != 1 || op.Public.OperationID != pointer.ID || !op.Public.Panel.Valid() || op.Public.ActorID < 1 {
		return op, errors.New("invalid full-stack operation")
	}
	if op.Public.PlanSHA256 != "" && op.Public.PlanSHA256 != op.digest() {
		return op, errors.New("full-stack authorization inputs do not match")
	}
	if err := op.validateGroups(); err != nil {
		return op, err
	}
	return op, nil
}

func (d *Driver) FullStackUpgradeStatus() (FullStackUpgradeStatus, error) {
	d.fullStackMu.Lock()
	defer d.fullStackMu.Unlock()
	op, err := d.readFullStackOperation()
	return op.Public, err
}

func (d *Driver) fullStackOwnsInstance(ctx context.Context, id string) error {
	if d.containerDataDir == "" {
		return nil
	}
	op, err := d.readFullStackOperation()
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return &RuntimeUpdateValidationError{Code: "full_stack_recovery_required", Message: "全栈升级记录需要恢复，请保留现有数据。"}
	}
	if op.Public.AuthorizedAt == nil || fullStackTerminal(op.Public.Phase) && op.Public.Phase != "manual_action" {
		return nil
	}
	for _, group := range op.Groups {
		for _, member := range group.Plan.Members {
			if member.ID == id && ctx.Value(fullStackContextKey{}) != op.Public.OperationID {
				return &RuntimeUpdateValidationError{Code: "full_stack_maintenance", Message: "此世界正在执行已确认的全栈升级，可继续查看进度和恢复入口。"}
			}
		}
	}
	return nil
}

type FullStackMaintenance struct {
	OperationID string `json:"operationId"`
	Phase       string `json:"phase"`
	Message     string `json:"message"`
}

func (d *Driver) InstanceFullStackMaintenance(id string) *FullStackMaintenance {
	if d.containerDataDir == "" {
		return nil
	}
	op, err := d.readFullStackOperation()
	if err != nil || op.Public.AuthorizedAt == nil || fullStackTerminal(op.Public.Phase) && op.Public.Phase != "manual_action" {
		return nil
	}
	for _, group := range op.Groups {
		for _, member := range group.Plan.Members {
			if member.ID == id {
				return &FullStackMaintenance{OperationID: op.Public.OperationID, Phase: op.Public.Phase, Message: "此世界正在执行全栈升级或恢复，普通变更已关闭；进度、诊断和恢复入口仍可访问。"}
			}
		}
	}
	return nil
}
