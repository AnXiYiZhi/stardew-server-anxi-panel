package docker

import (
	"context"
	"encoding/json"
	"regexp"
	"strconv"
)

var serviceNamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

func (c *Client) DockerVersion(ctx context.Context, workDir string) (CommandResult, error) {
	return c.run(ctx, "docker version", workDir, c.timeouts.Version, "version")
}

func (c *Client) ComposeVersion(ctx context.Context, workDir string) (CommandResult, error) {
	return c.run(ctx, "docker compose version", workDir, c.timeouts.Version, "compose", "version")
}

func (c *Client) ComposePs(ctx context.Context, dir string) (ComposePsResult, error) {
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
	return composeResult, err
}

func (c *Client) ComposePull(ctx context.Context, dir string) (CommandResult, error) {
	return c.run(ctx, "docker compose pull", dir, c.timeouts.Pull, "compose", "pull")
}

func (c *Client) ComposeUp(ctx context.Context, dir string) (CommandResult, error) {
	return c.run(ctx, "docker compose up", dir, c.timeouts.Up, "compose", "up", "-d")
}

func (c *Client) ComposeDown(ctx context.Context, dir string) (CommandResult, error) {
	return c.run(ctx, "docker compose down", dir, c.timeouts.Down, "compose", "down")
}

func (c *Client) ComposeRestart(ctx context.Context, dir string) (CommandResult, error) {
	return c.run(ctx, "docker compose restart", dir, c.timeouts.Restart, "compose", "restart")
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
	var raw []map[string]any
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		var single map[string]any
		if singleErr := json.Unmarshal([]byte(stdout), &single); singleErr != nil {
			return nil, err
		}
		raw = []map[string]any{single}
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

func intString(value int) string {
	return strconv.Itoa(value)
}
