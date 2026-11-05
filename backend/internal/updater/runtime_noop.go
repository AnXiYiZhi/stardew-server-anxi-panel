package updater

import (
	"context"
	"errors"
)

// RecordRuntimeNoop is called by the running driver coordinator, never by a
// browser confirmation. Only the still-current successful apply may receive it.
func (s *Service) RecordRuntimeNoop(ctx context.Context, expected ApplyStatus, target PanelTarget, full FullStackStatus) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	proof := full.CurrentRuntimeVerified
	if expected.Phase != PhaseSucceeded || !expected.CleanupCompleted || expected.UpdateID == "" || expected.OperationID != "" ||
		expected.ToVersion != target.Version || expected.SelectedDigest != target.ImageID ||
		full.Phase != "not_needed" || full.RuntimeRequired || proof == nil ||
		proof.UpdateID != expected.UpdateID || !proof.Panel.SameContent(target) ||
		!panelContentID.MatchString("sha256:"+proof.ScopeSHA256) || !panelContentID.MatchString("sha256:"+proof.LocalSHA256) || proof.VerifiedAt.IsZero() {
		return false, errors.New("runtime inspection does not match the successful Panel apply")
	}
	actual, err := s.PreparedPanelTarget(ctx, target.Version, target.Version)
	if err != nil || !actual.SameContent(target) {
		return false, errors.New("running Panel content changed during runtime inspection")
	}
	replacement := expected
	replacement.FullStack = &full
	return s.applyStore.CompareAndSwap(expected, replacement)
}
