//go:build !windows

package docker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/creack/pty"
)

func runSteamAuthPlatform(
	ctx context.Context,
	dataDir string,
	opts SteamAuthRunOpts,
	guardCh <-chan string,
	lineHandler func(string),
) (exitCode int, runErr error) {
	containerName := newSteamAuthContainerName()
	args := steamAuthDockerRunArgs(containerName, opts)
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Env = environmentWithOverrides(os.Environ(), opts.Env)
	if strings.TrimSpace(dataDir) != "" {
		cmd.Dir = dataDir
	}
	defer func() {
		if ctx.Err() == nil {
			return
		}
		cleanupErr := removeNamedSteamAuthContainer("docker", containerName)
		runErr = steamAuthCancellationError(runErr, ctx.Err(), cleanupErr)
		exitCode = -1
	}()

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return -1, fmt.Errorf("start steam-auth with pty: %w", err)
	}
	defer ptmx.Close()

	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			select {
			case input, ok := <-guardCh:
				if !ok {
					return
				}
				_, _ = fmt.Fprint(ptmx, input)
			case <-done:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	streamTTYOutput(ptmx, lineHandler)

	if err := cmd.Wait(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), nil
		}
		return -1, fmt.Errorf("steam-auth exited: %w", err)
	}
	return 0, nil
}

func steamAuthDockerRunArgs(containerName string, opts SteamAuthRunOpts) []string {
	args := []string{"run", "--name", containerName, "--rm", "--interactive", "--tty"}
	labelKeys := make([]string, 0, len(opts.Labels))
	for key := range opts.Labels {
		labelKeys = append(labelKeys, key)
	}
	sort.Strings(labelKeys)
	for _, key := range labelKeys {
		args = append(args, "--label", key+"="+opts.Labels[key])
	}
	if opts.User != "" {
		args = append(args, "--user", opts.User)
	}
	if len(opts.Entrypoint) > 0 && opts.Entrypoint[0] != "" {
		args = append(args, "--entrypoint", opts.Entrypoint[0])
	}
	for _, env := range opts.Env {
		if key, _, ok := strings.Cut(env, "="); ok && strings.TrimSpace(key) != "" {
			// Pass only the key in argv; the child environment below supplies the
			// value so Steam passwords are not exposed in the process command line.
			args = append(args, "--env", key)
		}
	}
	for _, bind := range opts.Binds {
		args = append(args, "--volume", bind)
	}
	args = append(args, opts.ImageRef)
	return append(args, containerCommand(opts)...)
}

func removeNamedSteamAuthContainer(dockerPath, containerName string) error {
	return removeNamedSteamAuthContainerWithCommand(
		containerName,
		20*time.Second,
		3*time.Second,
		100*time.Millisecond,
		func(ctx context.Context, args ...string) ([]byte, error) {
			return exec.CommandContext(ctx, dockerPath, args...).CombinedOutput()
		},
	)
}

type steamAuthCleanupCommand func(context.Context, ...string) ([]byte, error)

func removeNamedSteamAuthContainerWithCommand(
	containerName string,
	timeout time.Duration,
	stableAbsence time.Duration,
	pollInterval time.Duration,
	command steamAuthCleanupCommand,
) error {
	cleanupCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if stableAbsence <= 0 || pollInterval <= 0 {
		return errors.New("steam-auth cleanup timings must be positive")
	}

	var absentSince time.Time
	var lastErr error
	for {
		output, err := command(cleanupCtx, "container", "ls", "-aq", "--filter", "name=^/"+containerName+"$")
		if err == nil {
			ids := strings.Fields(string(output))
			if len(ids) != 1 {
				if len(ids) == 0 {
					if absentSince.IsZero() {
						absentSince = time.Now()
					}
					if time.Since(absentSince) >= stableAbsence {
						return nil
					}
				} else {
					return fmt.Errorf("expected at most one exact steam-auth container, found %d", len(ids))
				}
			} else {
				absentSince = time.Time{}
				removeOutput, removeErr := command(cleanupCtx, "container", "rm", "-f", containerName)
				if removeErr != nil {
					lastErr = fmt.Errorf("remove steam-auth container: %w: %s", removeErr, strings.TrimSpace(string(removeOutput)))
				} else {
					lastErr = nil
				}
			}
		} else {
			absentSince = time.Time{}
			lastErr = fmt.Errorf("list steam-auth container: %w: %s", err, strings.TrimSpace(string(output)))
		}
		select {
		case <-cleanupCtx.Done():
			if lastErr != nil {
				return errors.Join(cleanupCtx.Err(), lastErr)
			}
			return fmt.Errorf("steam-auth container absence did not remain stable: %w", cleanupCtx.Err())
		case <-time.After(pollInterval):
		}
	}
}

func runContainerTTYPlatform(
	ctx context.Context,
	dockerPath string,
	opts ContainerTTYRunOpts,
	guardCh <-chan string,
	lineHandler func(string),
) (int, error) {
	args := []string{"run", "--rm", "--interactive", "--tty"}
	if opts.User != "" {
		args = append(args, "--user", opts.User)
	}
	if len(opts.Entrypoint) > 0 && opts.Entrypoint[0] != "" {
		args = append(args, "--entrypoint", opts.Entrypoint[0])
	}
	for _, env := range opts.Env {
		if key, _, ok := strings.Cut(env, "="); ok && strings.TrimSpace(key) != "" {
			args = append(args, "--env", key)
		}
	}
	for _, bind := range opts.Binds {
		args = append(args, "--volume", bind)
	}
	args = append(args, opts.ImageRef)
	args = append(args, containerCommand(opts)...)

	cmd := exec.CommandContext(ctx, dockerPath, args...)
	cmd.Env = environmentWithOverrides(os.Environ(), opts.Env)

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return -1, fmt.Errorf("start container with pty: %w", err)
	}
	defer ptmx.Close()

	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			select {
			case input, ok := <-guardCh:
				if !ok {
					return
				}
				_, _ = fmt.Fprint(ptmx, input)
			case <-done:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	streamTTYOutput(ptmx, lineHandler)

	if err := cmd.Wait(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), nil
		}
		return -1, fmt.Errorf("container exited: %w", err)
	}
	return 0, nil
}

func environmentWithOverrides(base, overrides []string) []string {
	overridden := make(map[string]struct{}, len(overrides))
	for _, entry := range overrides {
		if key, _, ok := strings.Cut(entry, "="); ok && key != "" {
			overridden[key] = struct{}{}
		}
	}
	result := make([]string, 0, len(base)+len(overrides))
	for _, entry := range base {
		key, _, ok := strings.Cut(entry, "=")
		if ok {
			if _, replace := overridden[key]; replace {
				continue
			}
		}
		result = append(result, entry)
	}
	return append(result, overrides...)
}
