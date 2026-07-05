//go:build !windows

package docker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/creack/pty"
)

func runSteamAuthPlatform(
	ctx context.Context,
	dataDir string,
	opts SteamAuthRunOpts,
	guardCh <-chan string,
	lineHandler func(string),
) (int, error) {
	composePath := filepath.Join(dataDir, "docker-compose.yml")
	args := []string{"compose", "-f", composePath, "run", "--rm", "--interactive", "--tty", "steam-auth"}
	args = append(args, containerCommand(opts)...)
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Env = os.Environ()

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
