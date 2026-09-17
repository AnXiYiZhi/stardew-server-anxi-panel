package stardew_junimo

import (
	"context"
	"errors"
	"strings"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/installerrors"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func (r *installRunner) explainFailure(ctx context.Context, jobCtx *jobs.Context, err error) error {
	if err == nil || errors.Is(ctx.Err(), context.Canceled) {
		return err
	}
	// Detailed diagnostics stay in redacted logs. The persisted job summary uses
	// only catalog text, so refreshing or truncating the log tail loses no cause.
	safe := err.Error()
	for _, secret := range []string{r.password, r.vncPass} {
		if secret != "" {
			safe = strings.ReplaceAll(safe, secret, paneldocker.Redacted)
		}
	}
	_, _ = jobCtx.Error(context.Background(), "[install:diagnostic] "+paneldocker.RedactString(safe))
	phase := "preparing"
	current, stateErr := r.driver.store.GetInstance(context.Background(), r.instance.ID)
	if stateErr == nil && !r.authOnly {
		phase = current.DriverPhase
	}
	if r.authOnly {
		phase = "steam_invite_auth_failed"
	}
	message := r.failureEvidence.Message(err, phase)
	if stateErr == nil && !r.authOnly {
		state := storage.InstanceStateError
		if current.State == storage.InstanceStateCredentialsRequired {
			state = current.State
		}
		// Retain the existing machine phase for retry/recovery routing.
		r.driver.updatePhase(context.Background(), r.instance.ID, state, message, phase, jobCtx.ID)
	}
	return &installerrors.ExplainedError{Message: message, Cause: err}
}

func (r *installRunner) imagePullLineHandler(jobCtx *jobs.Context, prefix string, progress func(int, int)) func(string) {
	r.failureEvidence.Reset()
	handler := makeImagePullLineHandler(jobCtx, prefix, progress)
	return func(line string) {
		r.failureEvidence.Observe(line)
		handler(line)
	}
}
