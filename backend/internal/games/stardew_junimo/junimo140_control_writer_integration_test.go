//go:build integration

package stardew_junimo

import (
	"os"
	"strconv"
	"testing"
	"time"
)

// The Panel test process and game process must share an isolated control
// directory. This exercises the production atomic writer, not docker cp.
func TestJunimo140ControlWriterRealOptIn(t *testing.T) {
	dir := os.Getenv("ANXI_REAL_CONTROL_DATA_DIR")
	save := os.Getenv("ANXI_REAL_CONTROL_SAVE")
	count := os.Getenv("ANXI_REAL_CONTROL_ROLE_COUNT")
	if dir == "" || save == "" || count == "" {
		t.Skip("set ANXI_REAL_CONTROL_DATA_DIR, ANXI_REAL_CONTROL_SAVE and ANXI_REAL_CONTROL_ROLE_COUNT")
	}
	expected, err := strconv.Atoi(count)
	if err != nil || expected <= 0 {
		t.Fatal("invalid real Control fixture role count")
	}
	id, err := writePanelCommand(dir, "save-now", map[string]string{
		"preSaveAction": importFarmhandUnbindAction, "preSaveActionSaveId": save,
	})
	if err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(time.Minute); time.Now().Before(deadline); {
		outcome, err := GetCommandOutcome(dir, id)
		if err != nil {
			t.Fatal(err)
		}
		if outcome.Status == CommandStatusFailed {
			t.Fatalf("real Control refused the command: %s", outcome.ErrorCode)
		}
		if outcome.Status == CommandStatusSucceeded {
			bindings, err := importFarmhandUnbindSummaryFromOutcome(outcome, save, true)
			if err != nil || bindings.TotalFarmhands != expected || outcome.Details["event"] != "GameLoop.Saved" {
				t.Fatalf("real Control durable ownership receipt mismatch: %v", err)
			}
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("real Control result unknown; the writer was not resubmitted")
}
