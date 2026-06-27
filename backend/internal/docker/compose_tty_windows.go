//go:build windows

package docker

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// composeExecTTYPlatform creates a Docker exec instance with TTY:true and
// attaches stdin/stdout via the Docker Engine API (Windows named pipe).
//
// Correct Docker exec flow:
//  1. POST /containers/{name}/exec — creates exec instance, returns exec ID
//  2. POST /exec/{id}/start with HTTP hijack — starts exec AND opens the
//     bidirectional stdin/stdout stream in one step (no separate start call)
//  3. Write stdinData to the hijack conn
//  4. Close conn write direction to signal EOF to the process (attach-cli)
//  5. Read stdout until EOF (process exits) or ctx timeout (close conn to unblock)
//  6. Poll /exec/{id}/json until Running=false to get exit code
func composeExecTTYPlatform(
	ctx context.Context,
	c *Client,
	dir string,
	service string,
	stdinData string,
	args []string,
) (ComposeExecTTYResult, error) {
	// Resolve the container name from compose project + service.
	containerName, err := resolveComposeContainer(ctx, c, dir, service)
	if err != nil {
		return ComposeExecTTYResult{}, fmt.Errorf("resolve container: %w", err)
	}

	// 1. Create exec instance with TTY.
	execID, err := dockerExecCreate(ctx, containerName, args)
	if err != nil {
		return ComposeExecTTYResult{}, fmt.Errorf("exec create: %w", err)
	}

	// 2. Attach+Start in one call (HTTP hijack). This starts the exec process
	//    and gives us the bidirectional stdin/stdout stream.
	conn, reader, err := dockerExecAttachStart(ctx, execID)
	if err != nil {
		return ComposeExecTTYResult{}, fmt.Errorf("exec attach-start: %w", err)
	}

	// Ensure conn is closed no matter how we exit — this unblocks reader.Read
	// and signals EOF to the exec process if it's still running.
	var connClosed sync.Once
	closeConn := func() { connClosed.Do(func() { conn.Close() }) }
	defer closeConn()

	// 3. Write stdin data and close write direction to signal EOF.
	if stdinData != "" {
		if _, err := io.WriteString(conn, stdinData); err != nil {
			c.logger.Debug("exec tty stdin write error", "error", err)
		}
	}
	// Close the write half of the conn so attach-cli receives EOF on its stdin.
	// For Windows named pipe connections this is done by closing the whole conn,
	// but we still need to read remaining output. We use a goroutine to read
	// first, then close after reading is done.
	// Instead, we close write via shutdown (not available on named pipes),
	// so we rely on the exec process exiting after processing stdin.

	// 4. Read output with context awareness.
	type readResult struct {
		data []byte
		err  error
	}
	readCh := make(chan readResult, 1)
	go func() {
		var buf bytes.Buffer
		tmp := make([]byte, 4096)
		for {
			n, readErr := reader.Read(tmp)
			if n > 0 {
				buf.Write(tmp[:n])
			}
			if readErr != nil {
				readCh <- readResult{data: buf.Bytes(), err: readErr}
				return
			}
		}
	}()

	// 5. Wait for either output EOF or context timeout.
	var stdout []byte
	select {
	case result := <-readCh:
		stdout = result.data
		// EOF is expected when the exec process exits.
		if result.err != nil && result.err != io.EOF {
			c.logger.Debug("exec tty read ended with error", "error", result.err)
		}
	case <-ctx.Done():
		// Timeout: close conn to unblock the reader goroutine.
		closeConn()
		// Wait for the reader goroutine to finish with whatever it collected.
		result := <-readCh
		return ComposeExecTTYResult{
			Stdout:   string(result.data),
			ExitCode: -1,
		}, ctx.Err()
	}

	// 6. Poll /exec/{id}/json until Running=false to get the exit code.
	//    The exec may still be finishing even after stdout EOF.
	exitCode, waitErr := dockerExecPollExit(ctx, execID, 10*time.Second)
	if waitErr != nil {
		c.logger.Debug("exec tty poll exit failed", "error", waitErr)
		// If we got output, return it even if we couldn't get the exit code.
		return ComposeExecTTYResult{
			Stdout:   string(stdout),
			ExitCode: -1,
		}, waitErr
	}

	return ComposeExecTTYResult{
		Stdout:   string(stdout),
		ExitCode: exitCode,
	}, nil
}

// resolveComposeContainer finds the container name for a compose service.
func resolveComposeContainer(ctx context.Context, c *Client, dir, service string) (string, error) {
	result, err := c.ComposePs(ctx, dir)
	if err != nil {
		return "", fmt.Errorf("compose ps: %w", err)
	}
	for _, svc := range result.Services {
		if svc.Service == service && svc.Name != "" {
			return svc.Name, nil
		}
	}
	return "", fmt.Errorf("service %q not found in compose project", service)
}

// dockerExecCreate creates a Docker exec instance via the Engine API.
func dockerExecCreate(ctx context.Context, containerName string, args []string) (string, error) {
	body := map[string]any{
		"AttachStdin":  true,
		"AttachStdout": true,
		"AttachStderr": true,
		"Tty":          true,
		"Cmd":          args,
	}
	bodyBytes, _ := json.Marshal(body)

	conn, err := winDockerDial(ctx)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	path := "/v1.41/containers/" + containerName + "/exec"
	reqStr := fmt.Sprintf("POST %s HTTP/1.1\r\nHost: localhost\r\nContent-Type: application/json\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s",
		path, len(bodyBytes), bodyBytes)
	if _, err := conn.Write([]byte(reqStr)); err != nil {
		return "", err
	}

	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, raw)
	}

	var result struct{ Id string }
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.Id, nil
}

// dockerExecAttachStart attaches to AND starts a Docker exec instance in a
// single HTTP hijack call. This is the correct Docker API flow:
// POST /exec/{id}/start with Upgrade: tcp hijacks the connection, starts the
// exec, and returns the bidirectional stdin/stdout stream.
//
// There is NO separate "start" step — the hijack request itself starts the exec.
func dockerExecAttachStart(ctx context.Context, execID string) (io.ReadWriteCloser, *bufio.Reader, error) {
	conn, err := winDockerDial(ctx)
	if err != nil {
		return nil, nil, err
	}

	path := fmt.Sprintf("/v1.41/exec/%s/start", execID)
	body := `{"Tty":true,"Detach":false}`
	reqStr := fmt.Sprintf("POST %s HTTP/1.1\r\nHost: localhost\r\nContent-Type: application/json\r\nUpgrade: tcp\r\nConnection: Upgrade\r\nContent-Length: %d\r\n\r\n%s",
		path, len(body), body)
	if _, err := conn.Write([]byte(reqStr)); err != nil {
		conn.Close()
		return nil, nil, err
	}

	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, nil)
	if err != nil {
		conn.Close()
		return nil, nil, err
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		resp.Body.Close()
		conn.Close()
		return nil, nil, fmt.Errorf("expected 101 Switching Protocols, got %d", resp.StatusCode)
	}

	// After 101, conn is the raw bidirectional stream (stdin+stdout with TTY).
	return conn, reader, nil
}

// dockerExecPollExit polls /exec/{id}/json until the exec is no longer running,
// then returns the exit code. Respects ctx cancellation.
func dockerExecPollExit(ctx context.Context, execID string, timeout time.Duration) (int, error) {
	deadline := time.Now().Add(timeout)
	for {
		if ctx.Err() != nil {
			return -1, ctx.Err()
		}
		if time.Now().After(deadline) {
			return -1, fmt.Errorf("exec exit poll timed out after %s", timeout)
		}

		code, running, err := dockerExecInspect(ctx, execID)
		if err != nil {
			return -1, fmt.Errorf("exec inspect: %w", err)
		}
		if !running {
			return code, nil
		}

		// Poll interval: short enough to be responsive, long enough to not spam.
		select {
		case <-time.After(200 * time.Millisecond):
		case <-ctx.Done():
			return -1, ctx.Err()
		}
	}
}

// dockerExecInspect queries /exec/{id}/json and returns (exitCode, running, error).
func dockerExecInspect(ctx context.Context, execID string) (int, bool, error) {
	conn, err := winDockerDial(ctx)
	if err != nil {
		return -1, false, err
	}
	defer conn.Close()

	path := "/v1.41/exec/" + execID + "/json"
	reqStr := "GET " + path + " HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n"
	if _, err := conn.Write([]byte(reqStr)); err != nil {
		return -1, false, err
	}

	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		return -1, false, err
	}
	defer resp.Body.Close()

	var result struct {
		ExitCode int
		Running  bool
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return -1, false, err
	}
	return result.ExitCode, result.Running, nil
}
