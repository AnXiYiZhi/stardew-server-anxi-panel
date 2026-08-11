//go:build !windows

package docker

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	composePath := filepath.Join(dataDir, "docker-compose.yml")
	containerName := newSteamAuthContainerName()
	args := steamAuthComposeRunArgs(composePath, containerName, opts)
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Env = os.Environ()
	defer func() {
		if ctx.Err() == nil {
			return
		}
		if cleanupErr := removeNamedSteamAuthContainer("docker", containerName); cleanupErr != nil {
			runErr = errors.Join(runErr, fmt.Errorf("remove canceled steam-auth container: %w", cleanupErr))
			exitCode = -1
		}
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

func newSteamAuthContainerName() string {
	var random [8]byte
	if _, err := rand.Read(random[:]); err == nil {
		return "anxi-steam-auth-" + hex.EncodeToString(random[:])
	}
	return fmt.Sprintf("anxi-steam-auth-%x", time.Now().UTC().UnixNano())
}

func steamAuthComposeRunArgs(composePath, containerName string, opts SteamAuthRunOpts) []string {
	args := []string{"compose", "-f", composePath, "run", "--name", containerName, "--rm", "--interactive", "--tty", "steam-auth"}
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
		args = append(args, "--env", env)
	}
	for _, bind := range opts.Binds {
		args = append(args, "--volume", bind)
	}
	args = append(args, opts.ImageRef)
	args = append(args, containerCommand(opts)...)

	cmd := exec.CommandContext(ctx, dockerPath, args...)
	cmd.Env = os.Environ()

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
