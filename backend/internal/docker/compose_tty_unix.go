//go:build !windows

package docker

import (
	"context"
	"fmt"
)

// composeExecTTYPlatform on Unix uses creack/pty to allocate a pseudo-terminal
// for Docker exec. This is a placeholder — the Unix implementation should be
// filled in when this project targets Linux/macOS deployment.
func composeExecTTYPlatform(
	ctx context.Context,
	c *Client,
	dir string,
	service string,
	stdinData string,
	args []string,
) (ComposeExecTTYResult, error) {
	// TODO: Implement using creack/pty or docker compose exec without -T.
	// For now, fall back to the non-TTY path which may not work for
	// attach-cli but is the best we can do without a PTY library.
	return ComposeExecTTYResult{}, fmt.Errorf("ComposeExecTTY not implemented on this platform; use HTTP API or attach-cli with -T")
}
