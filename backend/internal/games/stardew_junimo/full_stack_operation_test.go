package stardew_junimo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	appconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/updater"
)

func fullStackTestPanel(version, letter string) updater.PanelTarget {
	return updater.PanelTarget{Version: version, Image: "ghcr.io/anxiyizhi/stardew-server-anxi-panel:" + version, ImageID: "sha256:" + strings.Repeat(letter, 64), Revision: strings.Repeat(letter, 40), BuildDate: "2026-10-16T00:00:00Z"}
}

func fullStackTestOperation(t *testing.T) (*Driver, fullStackOperation, *junimo140CoordinatorDocker) {
	t.Helper()
	fake := &junimo140CoordinatorDocker{}
	driver := NewWithOptions(fake, nil, nil, nil, DriverOptions{PanelVersion: "0.7.2", ContainerDataDir: t.TempDir()})
	target, err := fixedFullStackJunimo140Target()
	if err != nil {
		t.Fatal(err)
	}
	op := fullStackOperation{Schema: 1, Target: target, Public: FullStackUpgradeStatus{OperationID: "stack_" + strings.Repeat("a", 32), ActorID: 41, Panel: fullStackTestPanel("0.7.3", "b"), CreatedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(time.Hour), Phase: "awaiting_confirmation", RuntimeVersion: target.Server.Tag, Groups: []FullStackGroupStatus{}}}
	op.Public.PlanSHA256 = op.digest()
	if err := driver.writeFullStackOperation(op); err != nil {
		t.Fatal(err)
	}
	return driver, op, fake
}

func TestFullStackUnresolvedRecoveryCannotBeReplaced(t *testing.T) {
	driver, op, _ := fullStackTestOperation(t)
	store, err := storage.Open(context.Background(), appconfig.Config{DataDir: driver.containerDataDir, DBPath: filepath.Join(driver.containerDataDir, "panel.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	driver.store, driver.jobs = store, jobs.NewManager(store, driver.logger)
	now := time.Now().UTC()
	op.Public.AuthorizedAt, op.Public.Phase = &now, "manual_action"
	if err := driver.writeFullStackOperation(op); err != nil {
		t.Fatal(err)
	}
	_, err = driver.PrepareFullStackUpgrade(context.Background(), op.Public.ActorID, op.Public.Panel, false)
	var validation *RuntimeUpdateValidationError
	if !errors.As(err, &validation) || validation.Code != "full_stack_busy" {
		t.Fatal("unresolved recovery was not retained", err)
	}
	current, err := driver.FullStackUpgradeStatus()
	if err != nil || current.OperationID != op.Public.OperationID || current.Phase != "manual_action" {
		t.Fatal("new preflight replaced the unresolved grant", err)
	}
}

func TestFullStackConfirmationBindsActorOperationTargetAndExpiry(t *testing.T) {
	for _, scenario := range []string{"wrong actor", "wrong operation", "wrong hash", "changed image", "expired"} {
		t.Run(scenario, func(t *testing.T) {
			driver, op, _ := fullStackTestOperation(t)
			actor, id, hash, pin := op.Public.ActorID, op.Public.OperationID, op.Public.PlanSHA256, op.Public.Panel
			switch scenario {
			case "wrong actor":
				actor++
			case "wrong operation":
				id = "stack_" + strings.Repeat("c", 32)
			case "wrong hash":
				hash = strings.Repeat("f", 64)
			case "changed image":
				pin.ImageID = "sha256:" + strings.Repeat("d", 64)
			case "expired":
				op.Public.ExpiresAt = time.Now().UTC().Add(-time.Second)
				op.Public.PlanSHA256 = op.digest()
				hash = op.Public.PlanSHA256
				if err := driver.writeFullStackOperation(op); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := driver.ConfirmFullStackUpgrade(context.Background(), actor, id, hash, pin); err == nil {
				t.Fatal("forged or expired confirmation was accepted")
			}
			current, err := driver.FullStackUpgradeStatus()
			if err != nil || current.AuthorizedAt != nil || current.Phase != "awaiting_confirmation" {
				t.Fatal("rejected confirmation changed authorization")
			}
		})
	}
}

type fullStackPanelFixture struct {
	current updater.PanelTarget
	status  updater.ApplyStatus
	helpers int
}

func (f *fullStackPanelFixture) ApplyStatus() (updater.ApplyStatus, error) {
	if f.status.UpdateID == "" {
		return f.status, updater.ErrNoApplyStatus
	}
	return f.status, nil
}
func (f *fullStackPanelFixture) PreparedPanelTarget(context.Context, string, string) (updater.PanelTarget, error) {
	return f.current, nil
}
func (f *fullStackPanelFixture) StartApplyPinned(_ context.Context, from string, target updater.PanelTarget, id string) (updater.ApplyStatus, error) {
	if f.status.UpdateID != "" && updater.IsActiveApplyPhase(f.status.Phase) {
		return f.status, nil
	}
	f.helpers++
	f.status = updater.ApplyStatus{UpdateID: "one-panel-task", OperationID: id, PinnedTarget: &target, FromVersion: from, ToVersion: target.Version, Phase: updater.PhaseWaitingHealth, SelectedDigest: target.ImageID}
	return f.status, nil
}

func TestFullStackRestartAndDuplicateConfirmationWaitForPanelTerminal(t *testing.T) {
	driver, op, fake := fullStackTestOperation(t)
	for n := 0; n < 3; n++ {
		if _, err := driver.ConfirmFullStackUpgrade(context.Background(), 41, op.Public.OperationID, op.Public.PlanSHA256, op.Public.Panel); err != nil {
			t.Fatal(err)
		}
	}
	panel := &fullStackPanelFixture{current: fullStackTestPanel("0.7.2", "a")}
	for n := 0; n < 3; n++ {
		if err := driver.reconcileFullStackPanel(context.Background(), panel); err != nil {
			t.Fatal(err)
		}
	}
	if panel.helpers != 1 {
		t.Fatal("duplicate confirmation launched duplicate Panel helpers")
	}
	restarted := NewWithOptions(fake, nil, nil, nil, DriverOptions{PanelVersion: "0.7.3", ContainerDataDir: driver.containerDataDir})
	panel.current = op.Public.Panel
	panel.current.Image = panel.current.ImageID
	if err := restarted.reconcileFullStackPanel(context.Background(), panel); err != nil {
		t.Fatal(err)
	}
	view, _ := restarted.FullStackUpgradeStatus()
	if view.Phase != "waiting_panel" || restarted.fullStackRunning {
		t.Fatal("a reachable Panel was mistaken for a successful Panel health terminal")
	}
	panel.status.Phase = updater.PhaseSucceeded
	if err := restarted.reconcileFullStackPanel(context.Background(), panel); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for {
		view, err := restarted.FullStackUpgradeStatus()
		if err != nil {
			t.Fatal(err)
		}
		if view.Phase == "succeeded" {
			if view.OperationID != op.Public.OperationID || view.PlanSHA256 != op.Public.PlanSHA256 || view.ActorID != 41 || view.PanelUpdateID != "one-panel-task" {
				t.Fatal("restart lost original bounded authorization")
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("empty-install operation did not finish: %s", view.Phase)
		}
		time.Sleep(time.Millisecond)
	}
	if len(fake.events) != 0 {
		t.Fatal("empty installation touched game resources")
	}
}

func TestFullStackPanelUnhealthyRollbackNeverStartsGameMutation(t *testing.T) {
	driver, op, fake := fullStackTestOperation(t)
	if _, err := driver.ConfirmFullStackUpgrade(context.Background(), 41, op.Public.OperationID, op.Public.PlanSHA256, op.Public.Panel); err != nil {
		t.Fatal(err)
	}
	panel := &fullStackPanelFixture{current: fullStackTestPanel("0.7.2", "a")}
	if err := driver.reconcileFullStackPanel(context.Background(), panel); err != nil {
		t.Fatal(err)
	}
	panel.status.Phase, panel.status.ErrorCode = updater.PhaseFailedRolledBack, updater.CodeHealthCheckFailed
	if err := driver.reconcileFullStackPanel(context.Background(), panel); err != nil {
		t.Fatal(err)
	}
	view, _ := driver.FullStackUpgradeStatus()
	if view.Phase != "panel_failed_rolled_back" || view.ErrorCode != updater.CodeHealthCheckFailed || len(fake.events) != 0 {
		t.Fatal("Panel failure did not preserve the untouched game")
	}
}

func TestFullStackChangedPanelContentRequiresNewConsent(t *testing.T) {
	driver, op, fake := fullStackTestOperation(t)
	op.Public.Continuation = true
	op.Public.PlanSHA256 = op.digest()
	if err := driver.writeFullStackOperation(op); err != nil {
		t.Fatal(err)
	}
	if _, err := driver.ConfirmFullStackUpgrade(context.Background(), 41, op.Public.OperationID, op.Public.PlanSHA256, op.Public.Panel); err != nil {
		t.Fatal(err)
	}
	panel := &fullStackPanelFixture{current: fullStackTestPanel("0.7.3", "c")}
	if err := driver.reconcileFullStackPanel(context.Background(), panel); err != nil {
		t.Fatal(err)
	}
	view, _ := driver.FullStackUpgradeStatus()
	if view.Phase != "scope_changed" || panel.helpers != 0 || len(fake.events) != 0 {
		t.Fatal("changed content executed under stale consent")
	}
}

func TestFullStackImmutableRecordAndMaintenanceScope(t *testing.T) {
	driver, op, _ := fullStackTestOperation(t)
	plan, txn := junimo140TransactionFixture(t)
	txn.ActorID = op.Public.ActorID
	txn.Recommendation = op.Target
	txn.InputSHA256 = txn.inputDigest()
	op.Groups = []fullStackPreparedGroup{{Plan: plan, Original: txn}}
	view := FullStackGroupStatus{PrimaryID: plan.PrimaryID, ApplyID: plan.ApplyID, ScopeSHA256: plan.membershipDigest(), Phase: "prepared"}
	for _, member := range plan.Members {
		view.Members = append(view.Members, member.ID)
	}
	op.Public.Groups = []FullStackGroupStatus{view}
	op.Public.PlanSHA256 = op.digest()
	now := time.Now().UTC()
	op.Public.AuthorizedAt = &now
	op.Public.Phase = "waiting_panel"
	if err := driver.writeFullStackOperation(op); err != nil {
		t.Fatal(err)
	}
	if err := driver.fullStackOwnsInstance(context.Background(), plan.Members[0].ID); err == nil {
		t.Fatal("affected world mutation was not fenced")
	}
	if err := driver.fullStackOwnsInstance(context.Background(), "unrelated"); err != nil {
		t.Fatal("unrelated world entered maintenance")
	}
	if err := driver.fullStackOwnsInstance(context.WithValue(context.Background(), fullStackContextKey{}, "stack_"+strings.Repeat("c", 32)), plan.Members[0].ID); err == nil {
		t.Fatal("another operation bypassed the guard")
	}
	if err := driver.fullStackOwnsInstance(context.WithValue(context.Background(), fullStackContextKey{}, op.Public.OperationID), plan.Members[0].ID); err != nil {
		t.Fatal(err)
	}
	op.Groups[0].Plan.SourceVolume = "different"
	op.Public.PlanSHA256 = op.digest()
	if err := driver.writeFullStackOperation(op); err == nil {
		t.Fatal("immutable confirmed scope was rewritten")
	}
	if _, err := driver.CancelPreparedFullStackUpgrade(42, op.Public.OperationID); err == nil {
		t.Fatal("another administrator silently revoked the authorization")
	}
	if _, err := driver.CancelPreparedFullStackUpgrade(41, op.Public.OperationID); err != nil && !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := driver.fullStackOwnsInstance(context.Background(), plan.Members[0].ID); err != nil {
		t.Fatal("safe pre-execution cancellation retained maintenance")
	}
}

func TestFullStackLatePreflightCannotRepublishSupersededAuthorization(t *testing.T) {
	driver, old, _ := fullStackTestOperation(t)
	current := old
	current.Public.OperationID = "stack_" + strings.Repeat("c", 32)
	current.Public.ActorID = 42
	current.Public.PlanSHA256 = current.digest()
	now := time.Now().UTC()
	current.Public.AuthorizedAt, current.Public.Phase = &now, "waiting_panel"
	if err := driver.writeFullStackOperation(current); err != nil {
		t.Fatal(err)
	}
	if err := driver.writeFullStackOperation(old); err == nil {
		t.Fatal("late preflight replaced a newer authorized operation")
	}
	view, err := driver.FullStackUpgradeStatus()
	if err != nil || view.OperationID != current.Public.OperationID || view.AuthorizedAt == nil {
		t.Fatal("late read-only work removed the active maintenance owner", err)
	}
}

func TestFullStackExpiredAuthorizationNeverStartsAnotherGroup(t *testing.T) {
	driver, op, fake := fullStackTestOperation(t)
	plan, txn := junimo140TransactionFixture(t)
	txn.ActorID, txn.Recommendation = op.Public.ActorID, op.Target
	txn.InputSHA256 = txn.inputDigest()
	op.Groups = []fullStackPreparedGroup{{Plan: plan, Original: txn}}
	view := FullStackGroupStatus{PrimaryID: plan.PrimaryID, ApplyID: plan.ApplyID, ScopeSHA256: plan.membershipDigest(), Phase: "prepared"}
	for _, member := range plan.Members {
		view.Members = append(view.Members, member.ID)
	}
	op.Public.Groups = []FullStackGroupStatus{view}
	op.Public.CreatedAt, op.Public.ExpiresAt = time.Now().UTC().Add(-3*time.Hour), time.Now().UTC().Add(-time.Hour)
	op.Public.PlanSHA256 = op.digest()
	now := time.Now().UTC().Add(-2 * time.Hour)
	op.Public.AuthorizedAt, op.Public.Phase = &now, "migrating"
	if err := driver.writeFullStackOperation(op); err != nil {
		t.Fatal(err)
	}
	driver.runFullStackGroups(context.Background(), op)
	status, err := driver.FullStackUpgradeStatus()
	if err != nil || status.Phase != "scope_changed" || status.ErrorCode != "full_stack_confirmation_expired" || len(fake.events) != 0 {
		t.Fatal("expired grant began a new migration", err, fake.events)
	}
	if fullStackHasStartedGroup(op) {
		t.Fatal("preflight was mistaken for a started transaction")
	}
	for _, member := range plan.Members {
		if err := os.MkdirAll(filepath.Join(member.DataDir, ".local-container", "junimo140-group"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := claimJunimo140Group(plan); err != nil {
		t.Fatal(err)
	}
	if err := writeJunimo140GroupTransaction(plan, &txn); err != nil {
		t.Fatal(err)
	}
	if !fullStackHasStartedGroup(op) {
		t.Fatal("existing durable transaction lost its recovery path after expiry")
	}
}
