package stardew_junimo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFullStackConfirmationReadinessSeparatesPreflightAndExpiry(t *testing.T) {
	now := time.Date(2026, time.November, 4, 8, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, phase, code string
		expires           time.Time
	}{
		{"running preflight", "preparing", "full_stack_preflight_incomplete", now.Add(time.Hour)},
		{"failed preflight", "preflight_failed", "full_stack_preflight_failed", now.Add(time.Hour)},
		{"cancelled operation", "failed_safe", "full_stack_confirmation_unavailable", now.Add(time.Hour)},
		{"changed scope", "scope_changed", "full_stack_confirmation_unavailable", now.Add(time.Hour)},
		{"before exact expiry", "awaiting_confirmation", "", now.Add(time.Nanosecond)},
		{"at exact expiry", "awaiting_confirmation", "full_stack_confirmation_expired", now},
		{"after expiry", "awaiting_confirmation", "full_stack_confirmation_expired", now.Add(-time.Nanosecond)},
		{"incomplete old preflight", "preparing", "full_stack_preflight_incomplete", now.Add(-time.Hour)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateFullStackConfirmationReady(FullStackUpgradeStatus{Phase: tc.phase, ExpiresAt: tc.expires}, now)
			if tc.code == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			var validation *RuntimeUpdateValidationError
			if !errors.As(err, &validation) || validation.Code != tc.code {
				t.Fatalf("expected %s, got %v", tc.code, err)
			}
		})
	}
}

func TestFullStackStatusProjectsExpiryWithoutChangingAuthorization(t *testing.T) {
	for _, tc := range []struct {
		name, phase string
		authorized  bool
		future      bool
		expired     bool
	}{
		{"expired pending consent", "awaiting_confirmation", false, false, true},
		{"valid pending consent", "awaiting_confirmation", false, true, false},
		{"already authorized", "awaiting_confirmation", true, false, false},
		{"running transaction", "migrating", true, false, false},
		{"unfinished preflight", "preparing", false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			driver, op, _ := fullStackTestOperation(t)
			op.Public.Phase = tc.phase
			op.Public.ExpiresAt = time.Now().Add(-time.Hour)
			if tc.future {
				op.Public.ExpiresAt = time.Now().Add(time.Hour)
			}
			if tc.authorized {
				now := time.Now()
				op.Public.AuthorizedAt = &now
			}
			op.Public.PlanSHA256 = op.digest()
			if err := driver.writeFullStackOperation(op); err != nil {
				t.Fatal(err)
			}
			root, err := driver.fullStackRoot()
			if err != nil {
				t.Fatal(err)
			}
			paths := []string{filepath.Join(root, op.Public.OperationID+".json"), filepath.Join(root, "current.json")}
			before := make([][]byte, len(paths))
			for i, path := range paths {
				before[i], err = os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
			}
			status, err := driver.FullStackUpgradeStatus()
			if err != nil || status.ConfirmationExpired == nil || *status.ConfirmationExpired != tc.expired || status.Phase != tc.phase || status.PlanSHA256 != op.Public.PlanSHA256 {
				t.Fatal("expiry projection changed the operation identity or phase", err)
			}
			encoded, err := json.Marshal(status)
			if err != nil {
				t.Fatal(err)
			}
			var wire map[string]json.RawMessage
			if err := json.Unmarshal(encoded, &wire); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(wire["confirmationExpired"], []byte("true")) && !bytes.Equal(wire["confirmationExpired"], []byte("false")) {
				t.Fatal("GET omitted the authoritative false expiry observation")
			}
			for i, path := range paths {
				after, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(before[i], after) {
					t.Fatal("reading expiry wrote the persistent operation", err)
				}
			}
			persisted, err := driver.readFullStackOperation()
			if err != nil || persisted.Public.ConfirmationExpired != nil || persisted.digest() != op.digest() {
				t.Fatal("derived expiry entered the persistent grant or plan digest", err)
			}
			projected := op
			projected.Public = status
			if err := driver.writeFullStackOperation(projected); err != nil {
				t.Fatal(err)
			}
			for i, path := range paths {
				after, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(before[i], after) {
					t.Fatal("writer persisted a read-only expiry observation", err)
				}
			}
		})
	}
}

func TestFullStackRejectedPreflightConfirmationRetainsOriginalGrant(t *testing.T) {
	for _, phase := range []string{"preparing", "preflight_failed", "failed_safe"} {
		t.Run(phase, func(t *testing.T) {
			driver, op, fake := fullStackTestOperation(t)
			op.Public.Phase = phase
			if err := driver.writeFullStackOperation(op); err != nil {
				t.Fatal(err)
			}
			_, err := driver.ConfirmFullStackUpgrade(context.Background(), op.Public.ActorID, op.Public.OperationID, op.Public.PlanSHA256, op.Public.Panel)
			if err == nil {
				t.Fatal("unready preflight accepted confirmation")
			}
			current, err := driver.readFullStackOperation()
			if err != nil || current.Public.Phase != phase || current.Public.AuthorizedAt != nil || current.Public.PlanSHA256 != op.Public.PlanSHA256 || len(fake.events) != 0 {
				t.Fatal("rejected confirmation changed the original operation or touched Docker", err)
			}
		})
	}
}
