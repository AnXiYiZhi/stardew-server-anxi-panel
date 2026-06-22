package docker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"
)

func (c *Client) run(ctx context.Context, op string, workDir string, timeout time.Duration, args ...string) (CommandResult, error) {
	started := time.Now()
	result := CommandResult{
		WorkDir:  workDir,
		Args:     RedactArgs(append([]string{c.dockerPath}, args...)),
		ExitCode: -1,
	}

	if workDir == "" {
		result.DurationMS = time.Since(started).Milliseconds()
		return result, CommandError{Op: op, Result: result, Err: ErrInvalidWorkDir}
	}
	if stat, err := os.Stat(workDir); err != nil || !stat.IsDir() {
		result.DurationMS = time.Since(started).Milliseconds()
		if err != nil {
			result.Stderr = RedactString(err.Error())
		}
		return result, CommandError{Op: op, Result: result, Err: ErrInvalidWorkDir}
	}

	commandCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(commandCtx, c.dockerPath, args...)
	cmd.Dir = workDir

	var stdout limitedBuffer
	var stderr limitedBuffer
	stdout.limit = c.maxOutputBytes
	stderr.limit = c.maxOutputBytes
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result.DurationMS = time.Since(started).Milliseconds()
	result.Stdout = RedactString(stdout.String())
	result.Stderr = RedactString(stderr.String())
	result.StdoutTruncated = stdout.truncated
	result.StderrTruncated = stderr.truncated

	if commandCtx.Err() != nil {
		result.TimedOut = true
		return result, CommandError{Op: op, Result: result, Err: ErrCommandTimeout}
	}

	if err == nil {
		result.ExitCode = 0
		return result, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, CommandError{Op: op, Result: result, Err: ErrCommandFailed}
	}

	result.Stderr = RedactString(err.Error())
	return result, CommandError{Op: op, Result: result, Err: fmt.Errorf("start docker command: %w", err)}
}

type limitedBuffer struct {
	limit     int64
	buffer    bytes.Buffer
	truncated bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.limit <= 0 {
		return len(p), nil
	}
	remaining := b.limit - int64(b.buffer.Len())
	if remaining <= 0 {
		b.truncated = true
		return len(p), nil
	}
	if int64(len(p)) > remaining {
		_, _ = b.buffer.Write(p[:remaining])
		b.truncated = true
		return len(p), nil
	}
	_, err := b.buffer.Write(p)
	if err != nil && !errors.Is(err, io.ErrShortWrite) {
		return 0, err
	}
	return len(p), nil
}

func (b *limitedBuffer) String() string {
	return b.buffer.String()
}
