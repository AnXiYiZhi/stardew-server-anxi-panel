package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var serviceNamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

func (c *Client) DockerVersion(ctx context.Context, workDir string) (CommandResult, error) {
	return c.run(ctx, "docker version", workDir, c.timeouts.Version, "version")
}

func (c *Client) ComposeVersion(ctx context.Context, workDir string) (CommandResult, error) {
	return c.run(ctx, "docker compose version", workDir, c.timeouts.Version, "compose", "version")
}

func (c *Client) ComposePs(ctx context.Context, dir string) (ComposePsResult, error) {
	if cached, ok := c.cachedComposePs(dir, time.Now()); ok {
		return cached, nil
	}

	result, err := c.run(ctx, "docker compose ps", dir, c.timeouts.Ps, "compose", "ps", "--format", "json")
	composeResult := ComposePsResult{Result: result}
	if result.Stdout != "" {
		services, parseErr := parseComposeServices(result.Stdout)
		if parseErr != nil {
			c.logger.Debug("failed to parse docker compose ps json", "error", parseErr)
		} else {
			composeResult.Services = services
		}
	}
	if err == nil {
		c.storeComposePs(dir, composeResult, time.Now())
	}
	return composeResult, err
}

func (c *Client) ComposeStats(ctx context.Context, dir string) (ComposeStatsResult, error) {
	result, err := c.run(ctx, "docker compose stats", dir, c.timeouts.Stats, "compose", "stats", "--no-stream", "--format", "json")
	statsResult := ComposeStatsResult{Result: result}
	if result.Stdout != "" {
		services, parseErr := parseComposeStats(result.Stdout)
		if parseErr != nil {
			c.logger.Debug("failed to parse docker compose stats json", "error", parseErr)
		} else {
			statsResult.Services = services
		}
	}
	return statsResult, err
}

func (c *Client) ComposePull(ctx context.Context, dir string) (CommandResult, error) {
	return c.run(ctx, "docker compose pull", dir, c.timeouts.Pull, "compose", "pull")
}

func (c *Client) ImageInspect(ctx context.Context, dir string, imageRef string) (CommandResult, error) {
	return c.run(ctx, "docker image inspect", dir, c.timeouts.Ps, "image", "inspect", imageRef)
}

func (c *Client) ComposeUp(ctx context.Context, dir string) (CommandResult, error) {
	c.invalidateComposePs(dir)
	result, err := c.run(ctx, "docker compose up", dir, c.timeouts.Up, "compose", "up", "-d")
	c.invalidateComposePs(dir)
	return result, err
}

func (c *Client) ComposeDown(ctx context.Context, dir string) (CommandResult, error) {
	c.invalidateComposePs(dir)
	result, err := c.run(ctx, "docker compose down", dir, c.timeouts.Down, "compose", "down")
	c.invalidateComposePs(dir)
	return result, err
}

func (c *Client) ComposeRestart(ctx context.Context, dir string) (CommandResult, error) {
	c.invalidateComposePs(dir)
	result, err := c.run(ctx, "docker compose restart", dir, c.timeouts.Restart, "compose", "restart")
	c.invalidateComposePs(dir)
	return result, err
}

func (c *Client) ComposeRestartServices(ctx context.Context, dir string, services ...string) (CommandResult, error) {
	if len(services) == 0 {
		return c.ComposeRestart(ctx, dir)
	}
	args := []string{"compose", "restart"}
	for _, service := range services {
		if !serviceNamePattern.MatchString(service) {
			result := CommandResult{WorkDir: dir, Args: RedactArgs(append([]string{c.dockerPath}, args...)), ExitCode: -1}
			return result, CommandError{Op: "docker compose restart", Result: result, Err: fmt.Errorf("invalid compose service name %q", service)}
		}
		args = append(args, service)
	}
	c.invalidateComposePs(dir)
	result, err := c.run(ctx, "docker compose restart", dir, c.timeouts.Restart, args...)
	c.invalidateComposePs(dir)
	return result, err
}

// ComposeExecPipe runs `docker compose exec -T <service> <args>` with stdinData piped to the
// process stdin.  The -T flag disables pseudo-TTY allocation so stdin can be redirected.
func (c *Client) ComposeExecPipe(ctx context.Context, dir, service, stdinData string, args ...string) (CommandResult, error) {
	execArgs := append([]string{"compose", "exec", "-T", service}, args...)
	started := time.Now()
	result := CommandResult{
		WorkDir:  dir,
		Args:     RedactArgs(append([]string{c.dockerPath}, execArgs...)),
		ExitCode: -1,
	}
	if dir == "" {
		result.DurationMS = time.Since(started).Milliseconds()
		return result, CommandError{Op: "docker compose exec", Result: result, Err: ErrInvalidWorkDir}
	}
	commandCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(commandCtx, c.dockerPath, execArgs...)
	cmd.Dir = dir
	if stdinData != "" {
		cmd.Stdin = strings.NewReader(stdinData)
	}
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
		return result, CommandError{Op: "docker compose exec", Result: result, Err: ErrCommandTimeout}
	}
	if err == nil {
		result.ExitCode = 0
		return result, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, CommandError{Op: "docker compose exec", Result: result, Err: ErrCommandFailed}
	}
	result.Stderr = RedactString(err.Error())
	return result, CommandError{Op: "docker compose exec", Result: result, Err: fmt.Errorf("start docker compose exec: %w", err)}
}

func (c *Client) ComposeLogs(ctx context.Context, dir string, opts LogsOptions) (CommandResult, error) {
	tail := opts.Tail
	if tail == 0 {
		tail = DefaultLogTail
	}
	if tail < 1 || tail > MaxLogTail {
		return CommandResult{WorkDir: dir, ExitCode: -1}, ErrInvalidTail
	}

	args := []string{"compose", "logs", "--no-color", "--tail", intString(tail)}
	if opts.Service != "" {
		if !serviceNamePattern.MatchString(opts.Service) {
			return CommandResult{WorkDir: dir, ExitCode: -1}, ErrInvalidService
		}
		args = append(args, opts.Service)
	}
	return c.run(ctx, "docker compose logs", dir, c.timeouts.Logs, args...)
}

func parseComposeServices(stdout string) ([]ComposeService, error) {
	raw, err := parseComposeJSON(stdout)
	if err != nil {
		return nil, err
	}
	services := make([]ComposeService, 0, len(raw))
	for _, item := range raw {
		services = append(services, ComposeService{
			Name:     firstString(item, "Name", "name"),
			Service:  firstString(item, "Service", "service"),
			State:    firstString(item, "State", "state"),
			Status:   firstString(item, "Status", "status"),
			Health:   firstString(item, "Health", "health"),
			ExitCode: firstInt(item, "ExitCode", "exitCode"),
		})
	}
	return services, nil
}

func (c *Client) cachedComposePs(dir string, now time.Time) (ComposePsResult, bool) {
	if c.composePsTTL <= 0 || dir == "" {
		return ComposePsResult{}, false
	}
	c.composePsMu.Lock()
	defer c.composePsMu.Unlock()
	entry, ok := c.composePsCache[dir]
	if !ok || now.After(entry.expiresAt) {
		if ok {
			delete(c.composePsCache, dir)
		}
		return ComposePsResult{}, false
	}
	return cloneComposePsResult(entry.result), true
}

func (c *Client) storeComposePs(dir string, result ComposePsResult, now time.Time) {
	if c.composePsTTL <= 0 || dir == "" {
		return
	}
	c.composePsMu.Lock()
	defer c.composePsMu.Unlock()
	c.composePsCache[dir] = composePsCacheEntry{
		result:    cloneComposePsResult(result),
		expiresAt: now.Add(c.composePsTTL),
	}
}

func (c *Client) invalidateComposePs(dir string) {
	if dir == "" {
		return
	}
	c.composePsMu.Lock()
	defer c.composePsMu.Unlock()
	delete(c.composePsCache, dir)
}

func cloneComposePsResult(result ComposePsResult) ComposePsResult {
	cloned := result
	cloned.Result.Args = append([]string(nil), result.Result.Args...)
	cloned.Services = append([]ComposeService(nil), result.Services...)
	return cloned
}

func parseComposeStats(stdout string) ([]ComposeServiceStats, error) {
	raw, err := parseComposeJSON(stdout)
	if err != nil {
		return nil, err
	}
	services := make([]ComposeServiceStats, 0, len(raw))
	for _, item := range raw {
		memUsage := firstString(item, "MemUsage", "Mem Usage", "memoryUsage", "memUsage")
		memUsed, memLimit := parseMemoryUsage(memUsage)
		rawCPU := firstString(item, "CPUPerc", "CPU %", "CPUPercentage", "cpuPercent")
		rawMem := firstString(item, "MemPerc", "Mem %", "MemoryPercentage", "memoryPercent")
		services = append(services, ComposeServiceStats{
			Name:             firstString(item, "Name", "name"),
			Container:        firstString(item, "Container", "container"),
			ID:               firstString(item, "ID", "id"),
			Service:          firstString(item, "Service", "service"),
			CPUPerc:          parsePercent(rawCPU),
			MemPerc:          parsePercent(rawMem),
			MemUsage:         memUsage,
			MemUsedBytes:     memUsed,
			MemLimitBytes:    memLimit,
			NetIO:            firstString(item, "NetIO", "Net I/O", "netIO"),
			BlockIO:          firstString(item, "BlockIO", "Block I/O", "blockIO"),
			PIDs:             firstString(item, "PIDs", "pids"),
			RawCPUPerc:       rawCPU,
			RawMemPerc:       rawMem,
			RawMemUsedBytes:  firstString(item, "MemUsed", "MemoryUsed", "memoryUsed"),
			RawMemLimitBytes: firstString(item, "MemLimit", "MemoryLimit", "memoryLimit"),
		})
	}
	return services, nil
}

// parseComposeJSON handles three output formats from `docker compose ps --format json`:
//   - JSON array  (Compose v2 < 2.21)
//   - Single JSON object (single-service project)
//   - JSONL — one JSON object per line (Compose v2.21+ / v5+)
func parseComposeJSON(stdout string) ([]map[string]any, error) {
	stdout = strings.TrimSpace(stdout)
	if stdout == "" {
		return nil, nil
	}
	// JSON array.
	var arr []map[string]any
	if err := json.Unmarshal([]byte(stdout), &arr); err == nil {
		return arr, nil
	}
	// Single JSON object.
	var single map[string]any
	if err := json.Unmarshal([]byte(stdout), &single); err == nil {
		return []map[string]any{single}, nil
	}
	// JSONL: one JSON object per line.
	var result []map[string]any
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var item map[string]any
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			return nil, fmt.Errorf("parse compose ps line %q: %w", line, err)
		}
		result = append(result, item)
	}
	return result, nil
}

func firstString(item map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := item[key]; ok {
			switch typed := value.(type) {
			case string:
				return typed
			}
		}
	}
	return ""
}

func firstInt(item map[string]any, keys ...string) int {
	for _, key := range keys {
		if value, ok := item[key]; ok {
			switch typed := value.(type) {
			case float64:
				return int(typed)
			case int:
				return typed
			}
		}
	}
	return 0
}

func parsePercent(value string) float64 {
	value = strings.TrimSpace(strings.TrimSuffix(value, "%"))
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func parseMemoryUsage(value string) (int64, int64) {
	parts := strings.Split(value, "/")
	if len(parts) != 2 {
		return 0, 0
	}
	return parseByteSize(parts[0]), parseByteSize(parts[1])
}

func parseByteSize(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}

	fields := strings.Fields(value)
	if len(fields) == 2 {
		value = fields[0] + fields[1]
	}

	unitStart := len(value)
	for i, r := range value {
		if (r < '0' || r > '9') && r != '.' {
			unitStart = i
			break
		}
	}
	if unitStart == 0 {
		return 0
	}
	number, err := strconv.ParseFloat(strings.TrimSpace(value[:unitStart]), 64)
	if err != nil {
		return 0
	}
	unit := strings.ToLower(strings.TrimSpace(value[unitStart:]))
	multiplier := float64(1)
	switch unit {
	case "b", "byte", "bytes", "":
		multiplier = 1
	case "kb":
		multiplier = 1000
	case "mb":
		multiplier = 1000 * 1000
	case "gb":
		multiplier = 1000 * 1000 * 1000
	case "tb":
		multiplier = 1000 * 1000 * 1000 * 1000
	case "kib":
		multiplier = 1024
	case "mib":
		multiplier = 1024 * 1024
	case "gib":
		multiplier = 1024 * 1024 * 1024
	case "tib":
		multiplier = 1024 * 1024 * 1024 * 1024
	default:
		return 0
	}
	return int64(number * multiplier)
}

func intString(value int) string {
	return strconv.Itoa(value)
}
