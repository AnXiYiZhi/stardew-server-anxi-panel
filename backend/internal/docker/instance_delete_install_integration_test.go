package docker

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// This fixture has no game data or host mounts. Run it only against a dedicated
// test daemon. Its image preparation budget is separate from each scenario.
func TestInstallGameVolumeDeletionDocker(t *testing.T) {
	if os.Getenv("PANEL_INSTALL_DELETE_DOCKER_TEST") != "1" {
		t.Skip("set PANEL_INSTALL_DELETE_DOCKER_TEST=1 for isolated installation deletion regression")
	}
	prepareCtx, prepareCancel := context.WithTimeout(context.Background(), 4*time.Minute)
	client := NewClient(Options{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	prepareDir := t.TempDir()
	pulled, err := prepareInstallDeleteFixtureImage(prepareCtx, func(ctx context.Context, args ...string) (CommandResult, error) {
		return client.run(ctx, "prepare install deletion image", prepareDir, time.Minute, args...)
	})
	prepareCancel()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("installation deletion image ready: pulled=%t", pulled)
	for _, retainedSource := range []bool{false, true} {
		name := "fresh"
		if retainedSource {
			name = "retained-source"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			c := NewClient(Options{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
			dir := t.TempDir()
			run := func(args ...string) string {
				t.Helper()
				result, err := c.run(ctx, "install deletion fixture", dir, time.Minute, args...)
				if err != nil {
					t.Fatal(installDeleteFixtureCommandError("fixture command "+args[0], result, err))
				}
				return strings.TrimSpace(result.Stdout)
			}
			run("info")
			identity := make([]byte, 16)
			if _, err := rand.Read(identity); err != nil {
				t.Fatal(err)
			}
			token := hex.EncodeToString(identity)
			project := "anxi-install-delete-" + token[:12]
			source := project + "_game-data"
			target := project + "_anxi-smapi-update-" + token[:24]
			sentinel := project + "-foreign-sentinel"
			server, holder := project+"-server", project+"-foreign-holder"
			volumes := []string{target, sentinel}
			if retainedSource {
				volumes = append(volumes, source)
			}
			for _, volume := range volumes {
				exists, err := c.RuntimeVolumeExists(ctx, dir, volume)
				if err != nil || exists {
					t.Fatal("volume collision or uncertain fixture inventory")
				}
			}
			for _, container := range []string{server, holder} {
				if run("ps", "-aq", "--filter", "name=^/"+container+"$") != "" {
					t.Fatal("container fixture collision")
				}
			}
			t.Cleanup(func() {
				cleanup, done := context.WithTimeout(context.Background(), time.Minute)
				defer done()
				all, err := c.deletionContainers(cleanup, dir)
				if err != nil {
					t.Error("cannot verify fixture container cleanup ownership")
					return
				}
				for _, container := range all {
					if container.Config.Labels["qa-owner"] != token {
						continue
					}
					if !deletionStopped(container) {
						t.Error("refusing to clean a running fixture container")
						continue
					}
					if _, err := c.run(cleanup, "remove owned fixture container", dir, time.Minute, "rm", container.ID); err != nil {
						t.Error(err)
					}
				}
				for _, volume := range volumes {
					exists, err := c.RuntimeVolumeExists(cleanup, dir, volume)
					if err != nil {
						t.Error(err)
						continue
					}
					if !exists {
						continue
					}
					var rows []deletionVolume
					if err := c.deletionJSON(cleanup, dir, &rows, "volume", "inspect", volume); err != nil || len(rows) != 1 || rows[0].Labels["qa-owner"] != token {
						t.Error("refusing unverified fixture volume cleanup")
						continue
					}
					if _, err := c.run(cleanup, "remove owned fixture volume", dir, time.Minute, "volume", "rm", volume); err != nil {
						t.Error(err)
					}
				}
			})
			run("volume", "create", "--label", "qa-owner="+token, sentinel)
			run("volume", "create", "--label", "qa-owner="+token, "--label", "com.anxi-panel.smapi-update-staging=true", "--label", "com.anxi-panel.compose-project="+project, target)
			mountedGame := target
			var retained []string
			if retainedSource {
				run("volume", "create", "--label", "qa-owner="+token, "--label", "com.docker.compose.project="+project, source)
				mountedGame, retained = source, []string{source}
			}
			hostDir := filepath.Join(dir, project)
			run("create", "--name", server, "--pull", "never", "--network", "none", "--label", "qa-owner="+token,
				"--label", "com.docker.compose.project="+project, "--label", "com.docker.compose.service=server",
				"--label", "com.docker.compose.project.working_dir="+hostDir,
				"--mount", "type=volume,src="+mountedGame+",dst=/data/game", "alpine:3.20", "true")
			if retainedSource {
				if _, err := c.PlanInstanceDeletion(ctx, dir, project, hostDir, target); err == nil {
					t.Fatal("unrecorded old source mount was accepted")
				}
			}
			plan, err := c.PlanInstanceDeletionWithRetainedGameVolumes(ctx, dir, project, hostDir, target, retained)
			if err != nil || len(plan.Containers) != 1 || len(plan.Volumes) != len(volumes)-1 {
				t.Fatal("owned install volumes were not fully planned")
			}
			if _, ok := plan.Volumes[sentinel]; ok {
				t.Fatal("foreign sentinel entered product deletion plan")
			}
			run("create", "--name", holder, "--pull", "never", "--network", "none", "--label", "qa-owner="+token,
				"--mount", "type=volume,src="+target+",dst=/foreign", "alpine:3.20", "true")
			if _, err := c.PlanInstanceDeletionWithRetainedGameVolumes(ctx, dir, project, hostDir, target, retained); err == nil {
				t.Fatal("foreign holder was accepted by deletion planning")
			}
			if err := c.ApplyInstanceDeletion(ctx, dir, plan); err == nil {
				t.Fatal("foreign holder was ignored after deletion planning")
			}
			run("rm", holder) // This exact stopped holder was created above.
			if err := c.ApplyInstanceDeletion(ctx, dir, plan); err != nil {
				t.Fatal(err)
			}
			if run("ps", "-aq", "--filter", "name=^/"+server+"$") != "" {
				t.Fatal("product deletion leaked the old stopped server")
			}
			for _, volume := range volumes {
				exists, err := c.RuntimeVolumeExists(ctx, dir, volume)
				if err != nil || exists != (volume == sentinel) {
					t.Fatal("product deletion leaked owned data or removed the foreign sentinel")
				}
			}
		})
	}
}

type installDeleteFixtureRunner func(context.Context, ...string) (CommandResult, error)

func installDeleteFixtureCommandError(operation string, result CommandResult, err error) error {
	return fmt.Errorf("%s: exit_code=%d timed_out=%t stdout=%q stderr=%q error=%s", operation,
		result.ExitCode, result.TimedOut, RedactString(result.Stdout), RedactString(result.Stderr), RedactString(fmt.Sprint(err)))
}

func prepareInstallDeleteFixtureImage(ctx context.Context, run installDeleteFixtureRunner) (bool, error) {
	inspect := func() error {
		inspectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		result, err := run(inspectCtx, "image", "inspect", "alpine:3.20")
		if err != nil || result.ExitCode != 0 || result.TimedOut {
			return installDeleteFixtureCommandError("inspect install deletion image", result, err)
		}
		var images []struct {
			ID string `json:"Id"`
		}
		if json.Unmarshal([]byte(result.Stdout), &images) != nil || len(images) != 1 || !regexp.MustCompile(`^sha256:[a-f0-9]{64}$`).MatchString(images[0].ID) {
			return fmt.Errorf("install deletion image inspection did not prove one local image")
		}
		return nil
	}
	if inspect() == nil {
		return false, nil
	}
	var last error
	for attempt := 1; attempt <= 3; attempt++ {
		pullCtx, cancel := context.WithTimeout(ctx, time.Minute)
		result, err := run(pullCtx, "pull", "alpine:3.20")
		cancel()
		if err != nil || result.ExitCode != 0 || result.TimedOut {
			last = installDeleteFixtureCommandError(fmt.Sprintf("pull install deletion image attempt %d/3", attempt), result, err)
			continue
		}
		if err := inspect(); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, last
}

func TestInstallDeleteFixtureImagePreparation(t *testing.T) {
	for _, scenario := range []struct {
		name        string
		cached      bool
		failPulls   int
		nonzeroOnly bool
		badReadback bool
		wantPulls   int
		wantError   bool
	}{
		{name: "cached", cached: true},
		{name: "empty daemon", wantPulls: 1},
		{name: "transient source", failPulls: 2, wantPulls: 3},
		{name: "bounded unavailable source", failPulls: 3, wantPulls: 3, wantError: true},
		{name: "nonzero native exit", failPulls: 3, nonzeroOnly: true, wantPulls: 3, wantError: true},
		{name: "invalid readback", badReadback: true, wantPulls: 1, wantError: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			pulls := 0
			pulled, err := prepareInstallDeleteFixtureImage(context.Background(), func(ctx context.Context, args ...string) (CommandResult, error) {
				deadline, ok := ctx.Deadline()
				if !ok || time.Until(deadline) > time.Minute || time.Until(deadline) <= 0 {
					t.Fatal("fixture command lost its bounded context")
				}
				switch strings.Join(args, " ") {
				case "image inspect alpine:3.20":
					if !scenario.cached && pulls == 0 {
						return CommandResult{ExitCode: 1}, fmt.Errorf("not present")
					}
					if scenario.badReadback {
						return CommandResult{ExitCode: 0, Stdout: `[{"Id":"unknown"}]`}, nil
					}
					return CommandResult{ExitCode: 0, Stdout: `[{"Id":"sha256:` + strings.Repeat("a", 64) + `"}]`}, nil
				case "pull alpine:3.20":
					pulls++
					if pulls <= scenario.failPulls {
						var pullErr error
						if !scenario.nonzeroOnly {
							pullErr = fmt.Errorf("temporary source failure")
						}
						return CommandResult{ExitCode: 7, Stderr: "source unavailable"}, pullErr
					}
					return CommandResult{ExitCode: 0}, nil
				default:
					t.Fatal("fixture preparation changed the fixed image reference")
					return CommandResult{}, nil
				}
			})
			if (err != nil) != scenario.wantError || pulls != scenario.wantPulls || pulled != (!scenario.cached && !scenario.wantError) {
				t.Fatalf("preparation: pulls=%d pulled=%t error=%v", pulls, pulled, err)
			}
			if scenario.failPulls == 3 && (!strings.Contains(err.Error(), "exit_code=7") || !strings.Contains(err.Error(), "timed_out=false")) {
				t.Fatal("fixture source failure lost original exit or timeout diagnosis")
			}
		})
	}
}
