//go:build windows

package docker

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	winio "github.com/Microsoft/go-winio"
)

// runSteamAuthPlatform calls the Docker Engine API directly via the Windows
// named pipe so no host terminal is required.
func runSteamAuthPlatform(
	ctx context.Context,
	dataDir string,
	opts SteamAuthRunOpts,
	guardCh <-chan string,
	lineHandler func(string),
) (int, error) {
	// 1. Create container with Tty:true.
	containerID, err := winDockerCreate(ctx, opts)
	if err != nil {
		return -1, err
	}

	// 2. Attach (HTTP hijack) BEFORE start so we don't miss early output.
	conn, reader, err := winDockerAttach(ctx, containerID)
	if err != nil {
		return -1, err
	}
	defer conn.Close()
	stopCancelWatch := make(chan struct{})
	defer close(stopCancelWatch)
	go func() {
		select {
		case <-ctx.Done():
			// Stop the one-shot container before unblocking streamTTYOutput.
			// Disconnecting attach alone leaves the process running forever at
			// an interactive prompt, so AutoRemove would never take effect.
			stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
			_ = winDockerStop(stopCtx, containerID)
			stopCancel()
			_ = conn.Close()
		case <-stopCancelWatch:
		}
	}()

	// 3. Wait goroutine BEFORE start so we don't miss the exit event.
	exitCh := make(chan int, 1)
	exitErrCh := make(chan error, 1)
	go func() {
		code, err := winDockerWait(ctx, containerID)
		if err != nil {
			exitErrCh <- err
		} else {
			exitCh <- code
		}
	}()

	// 4. Start container.
	if err := winDockerStart(ctx, containerID); err != nil {
		return -1, err
	}

	// 5. Forward stdin from guardCh.
	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			select {
			case input, ok := <-guardCh:
				if !ok {
					return
				}
				if _, err := io.WriteString(conn, input); err != nil {
					lineHandler(fmt.Sprintf("[panel:stdin] write error: %v", err))
					return
				}
				lineHandler(fmt.Sprintf("[panel:stdin] sent %d byte(s) to container stdin", len(input)))
			case <-done:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	// 6. Read output until EOF (container exits and closes the attach stream).
	// With Tty:true the stream is raw bytes, no Docker multiplexing header.
	streamTTYOutput(reader, lineHandler)

	// 7. Return exit code from the wait goroutine.
	select {
	case code := <-exitCh:
		return code, nil
	case err := <-exitErrCh:
		return -1, err
	case <-ctx.Done():
		return -1, ctx.Err()
	}
}

func winDockerStop(ctx context.Context, containerID string) error {
	resp, conn, err := winDockerCall(ctx, "POST", "/v1.41/containers/"+containerID+"/stop?t=3", "")
	if err != nil {
		return err
	}
	defer conn.Close()
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotModified || resp.StatusCode == http.StatusNotFound {
		return nil
	}
	raw, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("stop steam-auth: HTTP %d: %s", resp.StatusCode, raw)
}

// ─── Docker Engine API helpers ─────────────────────────────────────────────

func winDockerDial(ctx context.Context) (net.Conn, error) {
	timeout := 5 * time.Second
	conn, err := winio.DialPipe(`\\.\pipe\docker_engine`, &timeout)
	if err != nil {
		return nil, fmt.Errorf("dial docker named pipe: %w", err)
	}
	return conn, nil
}

type winCreateBody struct {
	Image        string        `json:"Image"`
	Entrypoint   []string      `json:"Entrypoint,omitempty"`
	Cmd          []string      `json:"Cmd"`
	User         string        `json:"User,omitempty"`
	Tty          bool          `json:"Tty"`
	OpenStdin    bool          `json:"OpenStdin"`
	AttachStdin  bool          `json:"AttachStdin"`
	AttachStdout bool          `json:"AttachStdout"`
	AttachStderr bool          `json:"AttachStderr"`
	Env          []string      `json:"Env"`
	HostConfig   winHostConfig `json:"HostConfig"`
}

type winHostConfig struct {
	AutoRemove bool     `json:"AutoRemove"`
	Binds      []string `json:"Binds"`
}

func winDockerCreate(ctx context.Context, opts SteamAuthRunOpts) (string, error) {
	body := winCreateBody{
		Image:        opts.ImageRef,
		Entrypoint:   opts.Entrypoint,
		Cmd:          containerCommand(opts),
		User:         opts.User,
		Tty:          true,
		OpenStdin:    true,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Env:          opts.Env,
		HostConfig:   winHostConfig{AutoRemove: true, Binds: opts.Binds},
	}
	bodyBytes, _ := json.Marshal(body)

	resp, conn, err := winDockerCall(ctx, "POST", "/v1.41/containers/create", string(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("create steam-auth: %w", err)
	}
	defer conn.Close()
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("create steam-auth: HTTP %d: %s", resp.StatusCode, raw)
	}
	var result struct{ Id string }
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("create response decode: %w", err)
	}
	return result.Id, nil
}

func runContainerTTYPlatform(
	ctx context.Context,
	_ string,
	opts ContainerTTYRunOpts,
	guardCh <-chan string,
	lineHandler func(string),
) (int, error) {
	return runSteamAuthPlatform(ctx, "", opts, guardCh, lineHandler)
}

func winDockerAttach(ctx context.Context, containerID string) (net.Conn, *bufio.Reader, error) {
	conn, err := winDockerDial(ctx)
	if err != nil {
		return nil, nil, err
	}
	path := fmt.Sprintf("/v1.41/containers/%s/attach?stdin=1&stdout=1&stderr=1&stream=1", containerID)
	req := "POST " + path + " HTTP/1.1\r\nHost: localhost\r\nUpgrade: tcp\r\nConnection: Upgrade\r\nContent-Length: 0\r\n\r\n"
	if _, err := conn.Write([]byte(req)); err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("attach request: %w", err)
	}
	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, nil)
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("attach response: %w", err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		resp.Body.Close()
		conn.Close()
		return nil, nil, fmt.Errorf("attach: expected 101 Switching Protocols, got %d", resp.StatusCode)
	}
	// After 101, conn/reader is the raw TTY stream.
	return conn, reader, nil
}

func winDockerStart(ctx context.Context, containerID string) error {
	resp, conn, err := winDockerCall(ctx, "POST", "/v1.41/containers/"+containerID+"/start", "")
	if err != nil {
		return fmt.Errorf("start steam-auth: %w", err)
	}
	resp.Body.Close()
	conn.Close()
	if resp.StatusCode < 200 || (resp.StatusCode >= 300 && resp.StatusCode != 304) {
		return fmt.Errorf("start steam-auth: HTTP %d", resp.StatusCode)
	}
	return nil
}

func winDockerWait(ctx context.Context, containerID string) (int, error) {
	conn, err := winDockerDial(ctx)
	if err != nil {
		return -1, err
	}
	// Close conn when ctx is cancelled so the blocking read unblocks.
	go func() {
		<-ctx.Done()
		conn.Close()
	}()

	path := "/v1.41/containers/" + containerID + "/wait?condition=not-running"
	req := "POST " + path + " HTTP/1.1\r\nHost: localhost\r\nContent-Length: 0\r\nConnection: close\r\n\r\n"
	if _, err := conn.Write([]byte(req)); err != nil {
		conn.Close()
		return -1, fmt.Errorf("wait request: %w", err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		conn.Close()
		if ctx.Err() != nil {
			return -1, ctx.Err()
		}
		return -1, fmt.Errorf("wait response: %w", err)
	}
	defer resp.Body.Close()
	defer conn.Close()

	var result struct {
		StatusCode int
		Error      *struct{ Message string }
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return -1, fmt.Errorf("wait decode: %w", err)
	}
	if result.Error != nil && result.Error.Message != "" {
		return -1, fmt.Errorf("container error: %s", result.Error.Message)
	}
	return result.StatusCode, nil
}

// winDockerCall makes a single HTTP request to the Docker named pipe.
// Caller must close both resp.Body and the returned conn.
func winDockerCall(ctx context.Context, method, path, jsonBody string) (*http.Response, net.Conn, error) {
	conn, err := winDockerDial(ctx)
	if err != nil {
		return nil, nil, err
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s %s HTTP/1.1\r\nHost: localhost\r\nContent-Type: application/json\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s",
		method, path, len(jsonBody), jsonBody)
	if _, err := conn.Write([]byte(sb.String())); err != nil {
		conn.Close()
		return nil, nil, err
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		conn.Close()
		return nil, nil, err
	}
	return resp, conn, nil
}
