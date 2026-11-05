package updater

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRuntimeNoopCASPreservesNewerApplyOrHelperProgress(t *testing.T) {
	store := NewApplyStateStore(filepath.Join(t.TempDir(), "apply.json"))
	first := ApplyStatus{UpdateID: "one", Phase: PhaseSucceeded, Logs: []LogEntry{}}
	if err := store.Write(first); err != nil {
		t.Fatal(err)
	}
	expected, err := store.Read()
	if err != nil {
		t.Fatal(err)
	}
	for _, replacement := range []ApplyStatus{{UpdateID: "another", Phase: PhaseChecking, Logs: []LogEntry{}}, {UpdateID: "one", Phase: PhaseSucceeded, CleanupCompleted: true, Logs: []LogEntry{}}} {
		if err := store.Write(replacement); err != nil {
			t.Fatal(err)
		}
		noop := expected
		noop.FullStack = &FullStackStatus{Phase: "not_needed"}
		written, err := store.CompareAndSwap(expected, noop)
		if err != nil || written {
			t.Fatal("stale inspection overwrote newer state", err)
		}
		actual, _ := store.Read()
		if actual.UpdateID != replacement.UpdateID || actual.CleanupCompleted != replacement.CleanupCompleted {
			t.Fatal("newer state was lost")
		}
	}
}

func TestRuntimeNoopCASOnlyOneConcurrentPublication(t *testing.T) {
	store := NewApplyStateStore(filepath.Join(t.TempDir(), "apply.json"))
	if err := store.Write(ApplyStatus{UpdateID: "same", Phase: PhaseSucceeded}); err != nil {
		t.Fatal(err)
	}
	expected, err := store.Read()
	if err != nil {
		t.Fatal(err)
	}
	replacement := expected
	replacement.FullStack = &FullStackStatus{Phase: "not_needed"}
	results := make(chan bool, 12)
	var group sync.WaitGroup
	for n := 0; n < 12; n++ {
		group.Add(1)
		go func() {
			defer group.Done()
			written, err := store.CompareAndSwap(expected, replacement)
			results <- err == nil && written
		}()
	}
	group.Wait()
	close(results)
	count := 0
	for written := range results {
		if written {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("concurrent publications=%d", count)
	}
}

func TestRuntimeNoopRequiresExactRunningContentAndSameApply(t *testing.T) {
	for _, scenario := range []string{"valid", "different apply", "changed image", "authorized scope", "cleanup pending", "invalid scope hash"} {
		t.Run(scenario, func(t *testing.T) {
			service, runtime := newApplyServiceTest(t, &fakeDatabaseBackupper{})
			pin := testPanelPin("ghcr.io/anxiyizhi/stardew-server-anxi-panel:0.1.15")
			runtime.info.Image, runtime.info.ImageID = pin.Image, pin.ImageID
			inspector := &pinServiceRuntime{fakeRuntime: runtime, target: pin}
			service.docker = inspector
			status := ApplyStatus{UpdateID: "actual-apply", Phase: PhaseSucceeded, ToVersion: pin.Version, SelectedDigest: pin.ImageID, CleanupCompleted: true, Logs: []LogEntry{}}
			proof := &RuntimeNoopProof{UpdateID: status.UpdateID, Panel: pin, ScopeSHA256: strings.Repeat("a", 64), LocalSHA256: strings.Repeat("b", 64), VerifiedAt: time.Now().UTC()}
			full := FullStackStatus{Phase: "not_needed", Progress: 100, CurrentRuntimeVerified: proof}
			switch scenario {
			case "different apply":
				proof.UpdateID = "another"
			case "changed image":
				runtime.info.ImageID = "sha256:" + strings.Repeat("d", 64)
			case "authorized scope":
				status.OperationID = "stack_" + strings.Repeat("a", 32)
			case "cleanup pending":
				status.CleanupCompleted = false
			case "invalid scope hash":
				proof.ScopeSHA256 = strings.Repeat("a", 40) + strings.Repeat("z", 24)
			}
			if err := service.applyStore.Write(status); err != nil {
				t.Fatal(err)
			}
			status, err := service.ApplyStatus()
			if err != nil {
				t.Fatal(err)
			}
			written, err := service.RecordRuntimeNoop(context.Background(), status, pin, full)
			actual, _ := service.ApplyStatus()
			if scenario == "valid" {
				if err != nil || !written || actual.UpdateID != status.UpdateID || actual.Phase != PhaseSucceeded || actual.FullStack == nil || actual.FullStack.Phase != "not_needed" {
					t.Fatal("exact no-op not recorded", err)
				}
			} else if err == nil || written || actual.FullStack != nil {
				t.Fatal("invalid no-op changed apply", err)
			}
		})
	}
}
