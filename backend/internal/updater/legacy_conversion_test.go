package updater

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLegacyConversionRejectsStateHelperMismatchBeforeCutover(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "apply-status.json")
	now := time.Now().UTC()
	digest := "sha256:" + strings.Repeat("a", 64)
	store := NewApplyStateStore(stateFile)
	if err := store.Write(ApplyStatus{
		UpdateID: "state-id", Phase: PhaseBackingUp, FromVersion: "0.3.7", ToVersion: "0.3.13",
		OriginalImage: "anxiyizhi/stardew-server-anxi-panel:0.3.7", OriginalDigest: digest,
		StartedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	err := RunLegacyConversion(context.Background(), LegacyConversionOptions{
		FromVersion: "0.3.7", TargetVersion: "0.3.13",
		CurrentImage: "anxiyizhi/stardew-server-anxi-panel:0.3.7", OriginalDigest: digest,
		CurrentContainer: "fnos-panel", StateFile: stateFile,
		BackupDir: "/data/updater/backups/different-id", DatabaseRelative: "panel.db",
		ScriptPath: filepath.Join(t.TempDir(), "must-not-run.sh"),
	})
	if err == nil {
		t.Fatal("mismatched persisted transaction was accepted")
	}
	status, readErr := store.Read()
	if readErr != nil || status.Phase != PhaseFailedRolledBack || status.ErrorCode != CodeComposeMetadataInvalid {
		t.Fatalf("status=%+v readErr=%v", status, readErr)
	}
}
