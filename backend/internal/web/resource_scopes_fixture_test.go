//go:build integration

package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
)

const resourceScopesFixtureImage = "alpine:3.20"

type resourceScopesFixtureRunner func(context.Context, ...string) (paneldocker.CommandResult, error)

func runResourceScopesFixtureCommand(ctx context.Context, args ...string) (paneldocker.CommandResult, error) {
	var stdout, stderr bytes.Buffer
	command := exec.CommandContext(ctx, "docker", args...)
	command.Stdout, command.Stderr = &stdout, &stderr
	started := time.Now()
	err := command.Run()
	result := paneldocker.CommandResult{
		Args: paneldocker.RedactArgs(append([]string{"docker"}, args...)), ExitCode: -1,
		Stdout: paneldocker.RedactString(stdout.String()), Stderr: paneldocker.RedactString(stderr.String()),
		DurationMS: time.Since(started).Milliseconds(), TimedOut: ctx.Err() != nil,
	}
	if command.ProcessState != nil {
		result.ExitCode = command.ProcessState.ExitCode()
	}
	return result, err
}

func resourceScopesFixtureCommandError(operation string, result paneldocker.CommandResult, err error) error {
	return fmt.Errorf("%s: exit_code=%d timed_out=%t stdout=%q stderr=%q error=%s", operation,
		result.ExitCode, result.TimedOut, paneldocker.RedactString(result.Stdout),
		paneldocker.RedactString(result.Stderr), paneldocker.RedactString(err.Error()))
}

func prepareResourceScopesFixtureImage(ctx context.Context, run resourceScopesFixtureRunner) (bool, error) {
	inspect := func() error {
		inspectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		result, err := run(inspectCtx, "image", "inspect", resourceScopesFixtureImage)
		if err != nil {
			return resourceScopesFixtureCommandError("inspect resource fixture image", result, err)
		}
		var images []struct {
			ID string `json:"Id"`
		}
		if err := json.Unmarshal([]byte(result.Stdout), &images); err != nil || len(images) != 1 || !regexp.MustCompile(`^sha256:[0-9a-f]{64}$`).MatchString(images[0].ID) {
			return fmt.Errorf("resource fixture image inspection did not prove one local image")
		}
		return nil
	}
	if inspect() == nil {
		return false, nil
	}
	var last error
	for attempt := 1; attempt <= 3; attempt++ {
		pullCtx, cancel := context.WithTimeout(ctx, time.Minute)
		result, err := run(pullCtx, "pull", resourceScopesFixtureImage)
		cancel()
		if err != nil {
			last = resourceScopesFixtureCommandError(fmt.Sprintf("pull resource fixture image attempt %d/3", attempt), result, err)
			continue
		}
		if err := inspect(); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, last
}

func TestResourceScopesFixtureImagePreparation(t *testing.T) {
	image := paneldocker.CommandResult{ExitCode: 0, Stdout: `[{"Id":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}]`}
	for _, test := range []struct {
		name       string
		cached     bool
		pullErrors int
		wantPulls  int
		wantError  bool
	}{
		{name: "cached image", cached: true},
		{name: "empty daemon", wantPulls: 1},
		{name: "transient pull", pullErrors: 1, wantPulls: 2},
		{name: "bounded unavailable source", pullErrors: 3, wantPulls: 3, wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			available, pulls := test.cached, 0
			run := func(ctx context.Context, args ...string) (paneldocker.CommandResult, error) {
				if _, bounded := ctx.Deadline(); !bounded {
					t.Fatal("fixture Docker command has no deadline")
				}
				if args[0] == "image" {
					if len(args) != 3 || args[1] != "inspect" || args[2] != resourceScopesFixtureImage {
						t.Fatalf("unexpected inspect arguments: %q", args)
					}
					if available {
						return image, nil
					}
					return paneldocker.CommandResult{ExitCode: 1, Stderr: "No such image"}, fmt.Errorf("not present")
				}
				if len(args) != 2 || args[0] != "pull" || args[1] != resourceScopesFixtureImage {
					t.Fatalf("unexpected pull arguments: %q", args)
				}
				pulls++
				if pulls <= test.pullErrors {
					return paneldocker.CommandResult{ExitCode: 1, Stderr: "registry unavailable"}, fmt.Errorf("pull failed")
				}
				available = true
				return paneldocker.CommandResult{ExitCode: 0}, nil
			}
			pulled, err := prepareResourceScopesFixtureImage(context.Background(), run)
			if (err != nil) != test.wantError || pulls != test.wantPulls || pulled != (!test.cached && !test.wantError) {
				t.Fatalf("pulled=%t attempts=%d error=%v", pulled, pulls, err)
			}
		})
	}
}

func TestResourceScopesFixtureVerifiesSuccessfulPull(t *testing.T) {
	inspects, pulls := 0, 0
	run := func(_ context.Context, args ...string) (paneldocker.CommandResult, error) {
		if args[0] == "pull" {
			pulls++
			return paneldocker.CommandResult{ExitCode: 0}, nil
		}
		inspects++
		if inspects == 1 {
			return paneldocker.CommandResult{ExitCode: 1}, fmt.Errorf("image absent")
		}
		return paneldocker.CommandResult{ExitCode: 0, Stdout: "[]"}, nil
	}
	if pulled, err := prepareResourceScopesFixtureImage(context.Background(), run); err == nil || pulled || inspects != 2 || pulls != 1 {
		t.Fatalf("successful pull bypassed image verification: pulled=%t inspections=%d pulls=%d error=%v", pulled, inspects, pulls, err)
	}
}

func TestResourceScopesFixtureCommandDiagnostics(t *testing.T) {
	result := paneldocker.CommandResult{ExitCode: 37, TimedOut: true, Stdout: "fixture stdout", Stderr: "fixture stderr"}
	message := resourceScopesFixtureCommandError("fixture compose", result, fmt.Errorf("original failure")).Error()
	for _, evidence := range []string{"exit_code=37", "timed_out=true", `stdout="fixture stdout"`, `stderr="fixture stderr"`, "original failure"} {
		if !strings.Contains(message, evidence) {
			t.Fatalf("fixture failure dropped command evidence %q: %s", evidence, message)
		}
	}
}
