package docker

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestLabelSteamInviteOneShotAddsAuthoritativeOwnerAndProject(t *testing.T) {
	opts := SteamAuthRunOpts{Labels: map[string]string{"fixture": "preserved"}}
	labeled, err := labelSteamInviteOneShot(filepath.Join(t.TempDir(), "legacy-disabled"), opts)
	if err != nil {
		t.Fatal(err)
	}
	if labeled.Labels[steamInviteOneShotOwnerLabel] != steamInviteOneShotOwnerValue || labeled.Labels[steamInviteOneShotProjectLabel] != "legacy-disabled" || labeled.Labels["fixture"] != "preserved" {
		t.Fatalf("one-shot labels = %#v", labeled.Labels)
	}
	if _, mutated := opts.Labels[steamInviteOneShotOwnerLabel]; mutated {
		t.Fatal("labeling mutated the caller-owned options map")
	}
}

func TestSteamAuthCancellationErrorIsExactOnlyWithoutOtherFailures(t *testing.T) {
	if got := steamAuthCancellationError(nil, context.Canceled, nil); got != context.Canceled {
		t.Fatalf("clean intentional cancellation = %v, want exact context.Canceled", got)
	}

	runErr := errors.New("substantive runner failure")
	got := steamAuthCancellationError(runErr, context.Canceled, nil)
	if got == context.Canceled || !errors.Is(got, runErr) || !errors.Is(got, context.Canceled) {
		t.Fatalf("runner failure was hidden by cancellation: %v", got)
	}

	cleanupErr := errors.New("exact container cleanup failed")
	got = steamAuthCancellationError(nil, context.Canceled, cleanupErr)
	if got == context.Canceled || !errors.Is(got, cleanupErr) || !errors.Is(got, context.Canceled) {
		t.Fatalf("cleanup failure was hidden by cancellation: %v", got)
	}
}
