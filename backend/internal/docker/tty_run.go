package docker

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

const (
	steamInviteOneShotOwnerLabel   = "io.anxi-panel.steam-invite.owner"
	steamInviteOneShotOwnerValue   = "one-shot-auth"
	steamInviteOneShotProjectLabel = "io.anxi-panel.steam-invite.project"
)

// ContainerTTYRunOpts holds everything the Docker layer needs to create an
// interactive one-shot container.
type ContainerTTYRunOpts struct {
	ImageRef   string   // e.g. "sdvd/steam-service:1.5.0-preview.121"
	Entrypoint []string // optional entrypoint override, e.g. ["/bin/sh"]
	Command    []string // steam-auth command, e.g. ["download"] or ["setup"]
	Env        []string // environment variables: "KEY=VALUE"
	Binds      []string // volume bind specs: "volumename:/container/path"
	Labels     map[string]string
	User       string // optional container user, e.g. "root"
}

// SteamAuthRunOpts is kept for the existing steam-auth call path.
type SteamAuthRunOpts = ContainerTTYRunOpts

func containerCommand(opts ContainerTTYRunOpts) []string {
	if len(opts.Command) == 0 {
		return []string{"download"}
	}
	return opts.Command
}

func newSteamAuthContainerName() string {
	var random [8]byte
	if _, err := rand.Read(random[:]); err == nil {
		return "anxi-steam-auth-" + hex.EncodeToString(random[:])
	}
	return fmt.Sprintf("anxi-steam-auth-%x", time.Now().UTC().UnixNano())
}

func labelSteamInviteOneShot(dataDir string, opts SteamAuthRunOpts) (SteamAuthRunOpts, error) {
	project := strings.ToLower(filepath.Base(filepath.Clean(dataDir)))
	if !filepath.IsAbs(dataDir) || !composeProjectPattern.MatchString(project) {
		return SteamAuthRunOpts{}, errors.New("Steam invite one-shot project cannot be derived safely")
	}
	labels := make(map[string]string, len(opts.Labels)+2)
	for key, value := range opts.Labels {
		labels[key] = value
	}
	labels[steamInviteOneShotOwnerLabel] = steamInviteOneShotOwnerValue
	labels[steamInviteOneShotProjectLabel] = project
	opts.Labels = labels
	return opts, nil
}

func steamAuthCancellationError(runErr, cancelErr, cleanupErr error) error {
	if cleanupErr != nil {
		return errors.Join(runErr, cancelErr, fmt.Errorf("cleanup canceled steam-auth container: %w", cleanupErr))
	}
	if runErr == nil {
		// Keep this exact so the installer can recognize intentional cancellation
		// only when no substantive runner or cleanup error also occurred.
		return cancelErr
	}
	return errors.Join(runErr, cancelErr)
}

// RunSteamAuthTTY runs the steam-auth container with a real TTY so that
// Console.ReadKey() works for interactive menu selection.
//
// On Linux: wraps an explicit `docker run --tty` via creack/pty — the host PTY
// satisfies Docker CLI's terminal check, causing it to allocate a container PTY.
// The explicit run is important: auth-only callers provide a scratch GAME_DIR
// and a session-only bind, which must not be replaced by Compose service mounts.
//
// On Windows: calls the Docker Engine API directly via the named pipe so no
// host terminal is required (the Docker CLI terminal check is bypassed).
//
// Each string from guardCh is written verbatim to the container stdin.
// Callers append "\n" for Console.ReadLine, omit "\n" for Console.ReadKey.
// lineHandler is called for each ANSI-stripped, non-empty output line.
// Returns the container exit code.
func (c *Client) RunSteamAuthTTY(
	ctx context.Context,
	dataDir string,
	opts SteamAuthRunOpts,
	guardCh <-chan string,
	lineHandler func(string),
) (int, error) {
	labeledOpts, err := labelSteamInviteOneShot(dataDir, opts)
	if err != nil {
		return -1, err
	}
	return runSteamAuthPlatform(ctx, dataDir, labeledOpts, guardCh, lineHandler)
}

// RunContainerTTY runs an arbitrary one-shot container with a TTY and forwards
// guardCh to stdin. It is used for fallback tools that need Steam Guard input.
func (c *Client) RunContainerTTY(
	ctx context.Context,
	opts ContainerTTYRunOpts,
	guardCh <-chan string,
	lineHandler func(string),
) (int, error) {
	return runContainerTTYPlatform(ctx, c.dockerPath, opts, guardCh, lineHandler)
}
