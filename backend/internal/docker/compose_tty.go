package docker

import (
	"context"
)

// ComposeExecTTYResult holds the result of a TTY exec operation.
type ComposeExecTTYResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// ComposeExecTTY runs `docker compose exec <service> <args>` with a real TTY
// allocated, unlike ComposeExecPipe which uses -T (no TTY).
//
// This is needed for commands like JunimoServer's `attach-cli` which require
// a terminal to operate. stdinData is written to the container's stdin.
//
// On Windows: calls the Docker Engine API directly via the named pipe.
// On Linux/macOS: uses creack/pty to allocate a pseudo-terminal.
func (c *Client) ComposeExecTTY(
	ctx context.Context,
	dir string,
	service string,
	stdinData string,
	args ...string,
) (ComposeExecTTYResult, error) {
	return composeExecTTYPlatform(ctx, c, dir, service, stdinData, args)
}
