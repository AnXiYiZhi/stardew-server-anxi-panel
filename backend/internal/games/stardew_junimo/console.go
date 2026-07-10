package stardew_junimo

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

// CommandDef describes a single allowlisted console command.
type CommandDef struct {
	ID          string // unique key, e.g. "info", "invitecode"
	Name        string // display name, e.g. "服务器信息"
	Description string // short description for the frontend
	AdminOnly   bool   // only admin role can execute
	CommandLine string // single SMAPI/Junimo command written to /tmp/smapi-input
}

// commandDefs is the global allowlist. The ID is the key the frontend sends.
var commandDefs = []CommandDef{
	{
		ID:          "info",
		Name:        "服务器信息",
		Description: "查看服务器当前状态、玩家数、存档信息",
		AdminOnly:   false,
		CommandLine: "info",
	},
	{
		ID:          "invitecode",
		Name:        "邀请码",
		Description: "获取当前服务器邀请码",
		AdminOnly:   false,
		CommandLine: "invitecode",
	},
	{
		ID:          "settings-show",
		Name:        "查看设置",
		Description: "显示当前服务器设置",
		AdminOnly:   true,
		CommandLine: "settings show",
	},
	{
		ID:          "settings-validate",
		Name:        "校验设置",
		Description: "校验服务器设置是否有效",
		AdminOnly:   true,
		CommandLine: "settings validate",
	},
	{
		ID:          "rendering-status",
		Name:        "渲染状态",
		Description: "查看服务器渲染状态",
		AdminOnly:   true,
		CommandLine: "rendering status",
	},
	{
		ID:          "host-auto",
		Name:        "自动托管状态",
		Description: "查看自动托管（host-auto）设置",
		AdminOnly:   true,
		CommandLine: "host-auto",
	},
	{
		ID:          "host-visibility",
		Name:        "可见性状态",
		Description: "查看服务器可见性（host-visibility）设置",
		AdminOnly:   true,
		CommandLine: "host-visibility",
	},
}

// commandDefMap is indexed by ID for O(1) lookup.
var commandDefMap = func() map[string]CommandDef {
	m := make(map[string]CommandDef, len(commandDefs))
	for _, c := range commandDefs {
		m[c.ID] = c
	}
	return m
}()

const (
	// maxSayLength is the maximum number of characters for a say message.
	maxSayLength = 200

	// commandTimeout is the default timeout for one-shot console command execution.
	commandTimeout = 8 * time.Second

	serverInputFIFO = "/tmp/smapi-input"
	serverOutputLog = "/tmp/server-output.log"
)

// CommandRequest is the structured input from the frontend.
type CommandRequest struct {
	Command string `json:"command"` // allowlist ID
}

// CommandRunResult is the structured result returned to the frontend.
type CommandRunResult struct {
	Command    string `json:"command"`
	Output     string `json:"output,omitempty"`
	Error      string `json:"error,omitempty"`
	ExitCode   int    `json:"exitCode"`
	DurationMS int64  `json:"durationMs"`
}

// ConsoleCommandDef is the frontend-facing command definition.
type ConsoleCommandDef struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	AdminOnly   bool   `json:"adminOnly"`
}

// ListCommands returns the commands available to the given role.
func ListCommands(isAdmin bool) []ConsoleCommandDef {
	result := make([]ConsoleCommandDef, 0, len(commandDefs))
	for _, c := range commandDefs {
		if c.AdminOnly && !isAdmin {
			continue
		}
		result = append(result, ConsoleCommandDef{
			ID:          c.ID,
			Name:        c.Name,
			Description: c.Description,
			AdminOnly:   c.AdminOnly,
		})
	}
	return result
}

// commandExecutor is the interface used by RunAllowlistedCommand and SendSay.
// It allows tests to inject a fake executor.
type commandExecutor interface {
	ComposeExecPipe(ctx context.Context, dir, service, stdinData string, args ...string) (paneldocker.CommandResult, error)
}

// RunAllowlistedCommand validates the command against the allowlist and executes
// it via attach-cli. It requires the instance to be in running state.
func (d *Driver) RunAllowlistedCommand(ctx context.Context, instance registry.Instance, req CommandRequest, isAdmin bool) (*CommandRunResult, error) {
	return runCommand(ctx, d, instance, req, isAdmin)
}

// SendSay sends a server-wide message via attach-cli say command.
func (d *Driver) SendSay(ctx context.Context, instance registry.Instance, message string) (*CommandRunResult, error) {
	return sendSay(ctx, d, instance, message)
}

// KickPlayer disconnects the given player from the running server. It is
// fire-and-forget: the embedded StardewAnxiPanel.Control SMAPI mod consumes
// the command on its next tick and calls Game1.server.kick internally.
func (d *Driver) KickPlayer(ctx context.Context, instance registry.Instance, uniqueMultiplayerID, name string) (*CommandRunResult, error) {
	return kickPlayer(instance, uniqueMultiplayerID, name)
}

// TriggerFestivalEvent asks the embedded control mod to simulate the "!event"
// chat command, force-starting today's festival main event. Upstream JunimoServer
// applies no permission check to this command. It is fire-and-forget like kick/say.
func (d *Driver) TriggerFestivalEvent(ctx context.Context, instance registry.Instance) (*CommandRunResult, error) {
	return triggerFestivalEvent(instance)
}

// triggerFestivalEvent is the testable core of Driver.TriggerFestivalEvent.
func triggerFestivalEvent(instance registry.Instance) (*CommandRunResult, error) {
	if instance.State != storage.InstanceStateRunning {
		return nil, &CommandError{Code: "server_not_running", Message: "服务器未运行，无法触发节日活动"}
	}

	start := time.Now()
	if err := writePanelCommand(instance.DataDir, "trigger-event", nil); err != nil {
		return nil, fmt.Errorf("写入触发节日活动命令失败: %w", err)
	}
	return &CommandRunResult{
		Command:    "trigger-event",
		Output:     "指令已提交，控制模组会在游戏 tick 中模拟 !event 聊天指令；若当前没有进行中的节日则不会生效。",
		ExitCode:   0,
		DurationMS: time.Since(start).Milliseconds(),
	}, nil
}

// runCommand is the testable core of RunAllowlistedCommand.
func runCommand(ctx context.Context, d *Driver, instance registry.Instance, req CommandRequest, isAdmin bool) (*CommandRunResult, error) {
	// Validate command ID against allowlist.
	def, ok := commandDefMap[req.Command]
	if !ok {
		return nil, &CommandError{Code: "command_not_allowed", Message: "不允许执行该命令"}
	}

	// Check admin-only permission.
	if def.AdminOnly && !isAdmin {
		return nil, &CommandError{Code: "forbidden", Message: "该命令仅管理员可执行"}
	}

	// Check instance state — must be running.
	if instance.State != storage.InstanceStateRunning {
		return nil, &CommandError{Code: "server_not_running", Message: "服务器未运行，无法执行命令"}
	}

	// Get the lifecycle docker service for exec.
	ld, ok := d.docker.(LifecycleDockerService)
	if !ok {
		return nil, &CommandError{Code: "not_supported", Message: "Docker 服务不支持命令执行"}
	}

	// Junimo's attach-cli is a tmux UI and does not support one-shot stdin.
	// Write directly to the FIFO used by its input pane instead.
	cmdCtx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()

	start := time.Now()
	output, exitCode, errText, err := sendServerCommand(cmdCtx, ld, instance.DataDir, def.CommandLine)
	duration := time.Since(start).Milliseconds()
	if err != nil {
		// Check for timeout.
		if cmdCtx.Err() == context.DeadlineExceeded {
			return &CommandRunResult{
				Command:    def.ID,
				Error:      "命令执行超时",
				ExitCode:   exitCode,
				DurationMS: duration,
			}, nil
		}
		// Non-zero exit is not necessarily an error for CLI output.
		// Return the output even on non-zero exit.
		if exitCode != 0 {
			return &CommandRunResult{
				Command:    def.ID,
				Output:     strings.TrimSpace(output),
				Error:      strings.TrimSpace(errText),
				ExitCode:   exitCode,
				DurationMS: duration,
			}, nil
		}
		return nil, fmt.Errorf("执行命令失败: %w", err)
	}

	return &CommandRunResult{
		Command:    def.ID,
		Output:     strings.TrimSpace(output),
		ExitCode:   exitCode,
		DurationMS: duration,
	}, nil
}

// sendSay is the testable core of SendSay.
func sendSay(ctx context.Context, d *Driver, instance registry.Instance, message string) (*CommandRunResult, error) {
	// Validate message.
	message = strings.TrimSpace(message)
	if message == "" {
		return nil, &CommandError{Code: "empty_message", Message: "喊话内容不能为空"}
	}
	if len([]rune(message)) > maxSayLength {
		return nil, &CommandError{Code: "message_too_long", Message: fmt.Sprintf("喊话内容不能超过 %d 个字符", maxSayLength)}
	}

	// Sanitize: replace ALL control characters (including \n, \r, \t) with spaces.
	// This prevents newline injection that could add arbitrary attach-cli commands.
	message = sanitizeSayMessage(message)
	if message == "" {
		return nil, &CommandError{Code: "empty_message", Message: "喊话内容清理后为空"}
	}

	// Check instance state.
	if instance.State != storage.InstanceStateRunning {
		return nil, &CommandError{Code: "server_not_running", Message: "服务器未运行，无法发送喊话"}
	}

	start := time.Now()
	if err := writePanelBroadcastCommand(instance.DataDir, message); err != nil {
		return nil, fmt.Errorf("写入喊话命令失败: %w", err)
	}
	return &CommandRunResult{
		Command:    "say",
		Output:     "喊话已提交，控制模组会在游戏 tick 中发送给在线玩家。",
		ExitCode:   0,
		DurationMS: time.Since(start).Milliseconds(),
	}, nil
}

func writePanelBroadcastCommand(dataDir, message string) error {
	return writePanelCommand(dataDir, "broadcast", map[string]string{"message": message})
}

// kickPlayer is the testable core of Driver.KickPlayer.
func kickPlayer(instance registry.Instance, uniqueMultiplayerID, name string) (*CommandRunResult, error) {
	uniqueMultiplayerID = strings.TrimSpace(uniqueMultiplayerID)
	if uniqueMultiplayerID == "" {
		return nil, &CommandError{Code: "invalid_player", Message: "缺少玩家联机 ID"}
	}
	if instance.State != storage.InstanceStateRunning {
		return nil, &CommandError{Code: "server_not_running", Message: "服务器未运行，无法踢出玩家"}
	}

	start := time.Now()
	if err := writePanelCommand(instance.DataDir, "kick", map[string]string{
		"uniqueMultiplayerId": uniqueMultiplayerID,
		"name":                strings.TrimSpace(name),
	}); err != nil {
		return nil, fmt.Errorf("写入踢出命令失败: %w", err)
	}
	return &CommandRunResult{
		Command:    "kick",
		Output:     "踢出指令已提交，控制模组会在游戏 tick 中处理；无法踢出主机玩家。",
		ExitCode:   0,
		DurationMS: time.Since(start).Milliseconds(),
	}, nil
}

// writePanelCommand drops a JSON command file into the instance's control
// commands directory, where the embedded StardewAnxiPanel.Control SMAPI mod
// polls and consumes it (see ModEntry.cs ConsumeCommands/HandleCommand).
func writePanelCommand(dataDir, name string, payload map[string]string) error {
	commandsDir := filepath.Join(controlDir(dataDir), "commands")
	if err := os.MkdirAll(commandsDir, 0o755); err != nil {
		return fmt.Errorf("create commands dir: %w", err)
	}
	id, err := randomHex(8)
	if err != nil {
		return err
	}
	command := struct {
		Name      string            `json:"name"`
		Payload   map[string]string `json:"payload"`
		CreatedAt string            `json:"createdAt"`
	}{
		Name:      name,
		Payload:   payload,
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	data, err := json.MarshalIndent(command, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal command: %w", err)
	}
	path := filepath.Join(commandsDir, fmt.Sprintf("%s-%s.json", time.Now().UTC().Format("20060102150405.000000000"), id))
	return os.WriteFile(path, data, 0o644)
}

func randomHex(byteCount int) (string, error) {
	buf := make([]byte, byteCount)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate command id: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

func sendServerCommand(ctx context.Context, exec commandExecutor, dir, command string) (string, int, string, error) {
	before := readServerLogSize(ctx, exec, dir)
	writeResult, err := exec.ComposeExecPipe(ctx, dir, "server", command+"\n", "tee", "-a", serverInputFIFO)
	if err != nil {
		return writeResult.Stdout, writeResult.ExitCode, writeResult.Stderr, err
	}

	var output string
	deadline := time.Now().Add(2 * time.Second)
	for {
		output = readServerLogSince(ctx, exec, dir, before)
		if strings.TrimSpace(output) != "" || time.Now().After(deadline) {
			break
		}
		select {
		case <-ctx.Done():
			return output, -1, "", ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
	if strings.TrimSpace(output) == "" {
		output = writeResult.Stdout
	}
	return output, writeResult.ExitCode, writeResult.Stderr, nil
}

func readServerLogSize(ctx context.Context, exec commandExecutor, dir string) int64 {
	result, err := exec.ComposeExecPipe(ctx, dir, "server", "", "wc", "-c", serverOutputLog)
	if err != nil {
		return 0
	}
	fields := strings.Fields(result.Stdout)
	if len(fields) == 0 {
		return 0
	}
	size, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil {
		return 0
	}
	return size
}

func readServerLogSince(ctx context.Context, exec commandExecutor, dir string, offset int64) string {
	if offset > 0 {
		result, err := exec.ComposeExecPipe(ctx, dir, "server", "", "tail", "-c", fmt.Sprintf("+%d", offset+1), serverOutputLog)
		if err == nil {
			return result.Stdout
		}
	}
	result, err := exec.ComposeExecPipe(ctx, dir, "server", "", "tail", "-n", "80", serverOutputLog)
	if err != nil {
		return ""
	}
	return result.Stdout
}

// stripControlChars removes control characters except normal whitespace (\n, \r, \t, space).
func stripControlChars(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return -1
		}
		return r
	}, s)
}

// sanitizeSayMessage replaces ALL control characters (including \n, \r, \t) with
// spaces, then collapses consecutive spaces. This prevents newline injection in
// say messages that could add arbitrary attach-cli commands.
func sanitizeSayMessage(s string) string {
	replaced := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	// Collapse multiple spaces into one.
	return strings.Join(strings.Fields(replaced), " ")
}

// CommandError is a structured error with a stable code and user-facing message.
type CommandError struct {
	Code    string
	Message string
}

func (e *CommandError) Error() string {
	return e.Message
}
