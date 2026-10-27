package stardew_junimo

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
)

type passwdlessVerifierDocker struct{ fakeDocker }

func (f *passwdlessVerifierDocker) RunContainerTTY(ctx context.Context, opts paneldocker.ContainerTTYRunOpts, guard <-chan string, line func(string)) (int, error) {
	if opts.User != "0" {
		return 125, errors.New("unable to find named user: image has no passwd file")
	}
	return f.fakeDocker.RunContainerTTY(ctx, opts, guard, line)
}

func TestGameDataVerifierSupportsImageWithoutPasswd(t *testing.T) {
	for _, tc := range []struct {
		name string
		code int
	}{{"complete", 0}, {"missing files", installVerificationMissingExitCode}} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &passwdlessVerifierDocker{fakeDocker: fakeDocker{verifyCode: tc.code}}
			driver := New(fake, slog.Default(), nil, &fakeStore{})
			ok, err := driver.verifyGameDataVolume(context.Background(), t.TempDir(), "junimo@sha256:fixed", func(string) {})
			if err != nil || ok != (tc.code == 0) || fake.verifyRuns != 1 {
				t.Fatalf("passwdless image verifier: ok=%v err=%v runs=%d exit=%d", ok, err, fake.verifyRuns, tc.code)
			}
		})
	}
}
