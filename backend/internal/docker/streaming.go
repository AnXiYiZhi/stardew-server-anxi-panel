package docker

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ansiPattern matches ANSI/VT100 escape sequences.
var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]|\x1b[()][AB012]|\x1b[=>]`)

// stripANSI removes ANSI escape sequences and trims trailing carriage returns.
func stripANSI(s string) string {
	s = ansiPattern.ReplaceAllString(s, "")
	return strings.TrimRight(s, "\r")
}

// streamTTYOutput reads raw TTY bytes and emits complete lines plus selected
// interactive prompts that often do not end with a newline.
func streamTTYOutput(reader io.Reader, lineHandler func(string)) {
	var pending string
	lastPrompt := ""
	buf := make([]byte, 4096)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			pending += stripANSI(string(buf[:n]))
			for {
				idx := strings.IndexByte(pending, '\n')
				if idx < 0 {
					break
				}
				line := strings.TrimRight(pending[:idx], "\r")
				pending = pending[idx+1:]
				lastPrompt = ""
				if strings.TrimSpace(line) != "" {
					lineHandler(line)
				}
			}
			if prompt := interactivePrompt(pending); prompt != "" && prompt != lastPrompt {
				lastPrompt = prompt
				lineHandler(prompt)
			}
			if len(pending) > 16*1024 {
				line := pending
				pending = ""
				lastPrompt = ""
				if strings.TrimSpace(line) != "" {
					lineHandler(line)
				}
			}
		}
		if err != nil {
			line := strings.TrimRight(pending, "\r")
			if strings.TrimSpace(line) != "" && line != lastPrompt {
				lineHandler(line)
			}
			return
		}
	}
}

func interactivePrompt(s string) string {
	lower := strings.ToLower(s)
	for _, marker := range []string{
		"enter steam guard code",
		"enter verification code",
		"enter code sent to",
	} {
		if idx := strings.LastIndex(lower, marker); idx >= 0 {
			return strings.TrimRight(s[idx:], "\r")
		}
	}
	return ""
}

// ComposePullStreaming runs docker compose pull [services...] and invokes lineHandler
// for each output line in real time. Pass an empty services slice to pull all services.
// lineHandler receives already-redacted, ANSI-stripped lines; empty lines are skipped.
func (c *Client) ComposePullStreaming(ctx context.Context, dir string, services []string, lineHandler func(line string)) (CommandResult, error) {
	started := time.Now()
	args := append([]string{"compose", "pull"}, services...)
	result := CommandResult{
		WorkDir:  dir,
		Args:     RedactArgs(append([]string{c.dockerPath}, args...)),
		ExitCode: -1,
	}

	if dir == "" {
		result.DurationMS = time.Since(started).Milliseconds()
		return result, CommandError{Op: "docker compose pull", Result: result, Err: ErrInvalidWorkDir}
	}
	if stat, err := os.Stat(dir); err != nil || !stat.IsDir() {
		result.DurationMS = time.Since(started).Milliseconds()
		if err != nil {
			result.Stderr = RedactString(err.Error())
		}
		return result, CommandError{Op: "docker compose pull", Result: result, Err: ErrInvalidWorkDir}
	}

	commandCtx, cancel := context.WithTimeout(ctx, c.timeouts.Pull)
	defer cancel()

	cmd := exec.CommandContext(commandCtx, c.dockerPath, args...)
	cmd.Dir = dir

	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	if err := cmd.Start(); err != nil {
		_ = pr.Close()
		_ = pw.Close()
		result.DurationMS = time.Since(started).Milliseconds()
		result.Stderr = RedactString(err.Error())
		return result, CommandError{Op: "docker compose pull", Result: result, Err: fmt.Errorf("start: %w", err)}
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(pr)
		for scanner.Scan() {
			line := stripANSI(scanner.Text())
			if line == "" {
				continue
			}
			if redacted := RedactString(line); redacted != "" {
				lineHandler(redacted)
			}
		}
	}()

	cmdErr := cmd.Wait()
	_ = pw.Close()
	wg.Wait()

	result.DurationMS = time.Since(started).Milliseconds()

	if commandCtx.Err() != nil {
		result.TimedOut = true
		return result, CommandError{Op: "docker compose pull", Result: result, Err: ErrCommandTimeout}
	}

	if cmdErr == nil {
		result.ExitCode = 0
		return result, nil
	}

	var exitErr *exec.ExitError
	if errors.As(cmdErr, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, CommandError{Op: "docker compose pull", Result: result, Err: ErrCommandFailed}
	}

	result.Stderr = RedactString(cmdErr.Error())
	return result, CommandError{Op: "docker compose pull", Result: result, Err: fmt.Errorf("start docker command: %w", cmdErr)}
}
