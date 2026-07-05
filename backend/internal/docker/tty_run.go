package docker

import "context"

// ContainerTTYRunOpts holds everything the Docker layer needs to create an
// interactive one-shot container.
type ContainerTTYRunOpts struct {
	ImageRef   string   // e.g. "sdvd/steam-service:1.5.0-preview.121"
	Entrypoint []string // optional entrypoint override, e.g. ["/bin/sh"]
	Command    []string // steam-auth command, e.g. ["download"] or ["setup"]
	Env        []string // environment variables: "KEY=VALUE"
	Binds      []string // volume bind specs: "volumename:/container/path"
	User       string   // optional container user, e.g. "root"
}

// SteamAuthRunOpts is kept for the existing steam-auth call path.
type SteamAuthRunOpts = ContainerTTYRunOpts

func containerCommand(opts ContainerTTYRunOpts) []string {
	if len(opts.Command) == 0 {
		return []string{"download"}
	}
	return opts.Command
}

// RunSteamAuthTTY runs the steam-auth container with a real TTY so that
// Console.ReadKey() works for interactive menu selection.
//
// On Linux: wraps `docker compose run --tty` via creack/pty — the host PTY
// satisfies Docker CLI's terminal check, causing it to allocate a container PTY.
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
	return runSteamAuthPlatform(ctx, dataDir, opts, guardCh, lineHandler)
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
