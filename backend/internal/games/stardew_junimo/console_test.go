package stardew_junimo

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

// fakeConsoleDocker implements LifecycleDockerService for console tests.
type fakeConsoleDocker struct {
	paneldocker.Client // embed to satisfy interface; methods panic if called unexpectedly

	execFunc         func(ctx context.Context, dir, service, stdinData string, args ...string) (paneldocker.CommandResult, error)
	runContainerFunc func(ctx context.Context, opts paneldocker.ContainerTTYRunOpts, guardCh <-chan string, lineHandler func(string)) (int, error)
	composeLogsFunc  func(ctx context.Context, dir string, opts paneldocker.LogsOptions) (paneldocker.CommandResult, error)
	restartFunc      func(ctx context.Context, dir string, services ...string) (paneldocker.CommandResult, error)
	composeDownFunc  func(ctx context.Context, dir string) (paneldocker.CommandResult, error)
	composeUpFunc    func(ctx context.Context, dir string) (paneldocker.CommandResult, error)
}

func (f *fakeConsoleDocker) ComposeDown(ctx context.Context, dir string) (paneldocker.CommandResult, error) {
	if f.composeDownFunc != nil {
		return f.composeDownFunc(ctx, dir)
	}
	return paneldocker.CommandResult{ExitCode: 0}, nil
}

func (f *fakeConsoleDocker) ComposeUp(ctx context.Context, dir string) (paneldocker.CommandResult, error) {
	if f.composeUpFunc != nil {
		return f.composeUpFunc(ctx, dir)
	}
	return paneldocker.CommandResult{ExitCode: 0}, nil
}

func (f *fakeConsoleDocker) ComposeExecPipe(ctx context.Context, dir, service, stdinData string, args ...string) (paneldocker.CommandResult, error) {
	if f.execFunc != nil {
		return f.execFunc(ctx, dir, service, stdinData, args...)
	}
	return paneldocker.CommandResult{Stdout: "ok", ExitCode: 0}, nil
}

func (f *fakeConsoleDocker) ComposeExecTTY(ctx context.Context, dir, service, stdinData string, args ...string) (paneldocker.ComposeExecTTYResult, error) {
	return paneldocker.ComposeExecTTYResult{Stdout: "ok", ExitCode: 0}, nil
}

func (f *fakeConsoleDocker) ComposePs(_ context.Context, _ string) (paneldocker.ComposePsResult, error) {
	return paneldocker.ComposePsResult{}, nil
}

func (f *fakeConsoleDocker) ComposeLogs(ctx context.Context, dir string, opts paneldocker.LogsOptions) (paneldocker.CommandResult, error) {
	if f.composeLogsFunc != nil {
		return f.composeLogsFunc(ctx, dir, opts)
	}
	return paneldocker.CommandResult{}, nil
}

func (f *fakeConsoleDocker) ComposeRestartServices(ctx context.Context, dir string, services ...string) (paneldocker.CommandResult, error) {
	if f.restartFunc != nil {
		return f.restartFunc(ctx, dir, services...)
	}
	return paneldocker.CommandResult{ExitCode: 0}, nil
}

func (f *fakeConsoleDocker) RunContainerTTY(ctx context.Context, opts paneldocker.ContainerTTYRunOpts, guardCh <-chan string, lineHandler func(string)) (int, error) {
	if f.runContainerFunc != nil {
		return f.runContainerFunc(ctx, opts, guardCh, lineHandler)
	}
	return 0, nil
}

// makeRunningInstance returns a registry.Instance in running state.
func makeRunningInstance() registry.Instance {
	return registry.Instance{
		ID:       "stardew",
		DriverID: DriverID,
		Name:     "Stardew Valley",
		DataDir:  "/tmp/test-instance",
		State:    storage.InstanceStateRunning,
	}
}

// makeStoppedInstance returns a registry.Instance in stopped state.
func makeStoppedInstance() registry.Instance {
	return registry.Instance{
		ID:       "stardew",
		DriverID: DriverID,
		Name:     "Stardew Valley",
		DataDir:  "/tmp/test-instance",
		State:    storage.InstanceStateStopped,
	}
}

// newTestDriver creates a Driver with a fake docker service for console tests.
func newTestDriver(docker *fakeConsoleDocker) *Driver {
	return &Driver{
		docker:     docker,
		logger:     nil,
		guardChans: make(map[string]chan string),
	}
}

// ── Command allowlist tests ──────────────────────────────────────────────────

func TestRunCommand_RejectsNonAllowlistedCommand(t *testing.T) {
	d := newTestDriver(&fakeConsoleDocker{})
	instance := makeRunningInstance()

	_, err := runCommand(context.Background(), d, instance, CommandRequest{Command: "rm -rf /"}, true)
	if err == nil {
		t.Fatal("expected error for non-allowlisted command")
	}
	ce, ok := err.(*CommandError)
	if !ok {
		t.Fatalf("expected *CommandError, got %T", err)
	}
	if ce.Code != "command_not_allowed" {
		t.Errorf("expected code 'command_not_allowed', got %q", ce.Code)
	}
}

func TestRunCommand_RejectsEmptyCommand(t *testing.T) {
	d := newTestDriver(&fakeConsoleDocker{})
	instance := makeRunningInstance()

	_, err := runCommand(context.Background(), d, instance, CommandRequest{Command: ""}, true)
	if err == nil {
		t.Fatal("expected error for empty command")
	}
	ce := err.(*CommandError)
	if ce.Code != "command_not_allowed" {
		t.Errorf("expected code 'command_not_allowed', got %q", ce.Code)
	}
}

func TestRunCommand_RejectsShellSpecialChars(t *testing.T) {
	d := newTestDriver(&fakeConsoleDocker{})
	instance := makeRunningInstance()

	// These should all be rejected because they're not in the allowlist.
	malicious := []string{
		"info; rm -rf /",
		"info | cat /etc/passwd",
		"info && echo pwned",
		"info`whoami`",
		"info$(whoami)",
		"invitecode; drop table users",
	}
	for _, cmd := range malicious {
		_, err := runCommand(context.Background(), d, instance, CommandRequest{Command: cmd}, true)
		if err == nil {
			t.Errorf("expected rejection for malicious command %q", cmd)
		}
	}
}

// ── Permission tests ─────────────────────────────────────────────────────────

func TestRunCommand_AdminOnly_RejectsNonAdmin(t *testing.T) {
	d := newTestDriver(&fakeConsoleDocker{})
	instance := makeRunningInstance()

	adminCommands := []string{"settings-show", "settings-validate", "rendering-status", "host-auto", "host-visibility"}
	for _, cmd := range adminCommands {
		_, err := runCommand(context.Background(), d, instance, CommandRequest{Command: cmd}, false)
		if err == nil {
			t.Errorf("expected rejection for admin-only command %q by non-admin", cmd)
			continue
		}
		ce := err.(*CommandError)
		if ce.Code != "forbidden" {
			t.Errorf("command %q: expected code 'forbidden', got %q", cmd, ce.Code)
		}
	}
}

func TestRunCommand_AdminOnly_AllowsAdmin(t *testing.T) {
	d := newTestDriver(&fakeConsoleDocker{
		execFunc: func(_ context.Context, _, _, _ string, _ ...string) (paneldocker.CommandResult, error) {
			return paneldocker.CommandResult{Stdout: "settings output", ExitCode: 0}, nil
		},
	})
	instance := makeRunningInstance()

	adminCommands := []string{"settings-show", "settings-validate", "rendering-status", "host-auto", "host-visibility"}
	for _, cmd := range adminCommands {
		result, err := runCommand(context.Background(), d, instance, CommandRequest{Command: cmd}, true)
		if err != nil {
			t.Errorf("admin command %q: unexpected error: %v", cmd, err)
		}
		if result == nil {
			t.Errorf("admin command %q: expected result", cmd)
		}
	}
}

func TestRunCommand_NonAdminCommands_AllowNonAdmin(t *testing.T) {
	d := newTestDriver(&fakeConsoleDocker{
		execFunc: func(_ context.Context, _, _, _ string, _ ...string) (paneldocker.CommandResult, error) {
			return paneldocker.CommandResult{Stdout: "info output", ExitCode: 0}, nil
		},
	})
	instance := makeRunningInstance()

	publicCommands := []string{"info", "invitecode"}
	for _, cmd := range publicCommands {
		result, err := runCommand(context.Background(), d, instance, CommandRequest{Command: cmd}, false)
		if err != nil {
			t.Errorf("public command %q: unexpected error: %v", cmd, err)
		}
		if result == nil {
			t.Errorf("public command %q: expected result", cmd)
		}
	}
}

// ── State tests ──────────────────────────────────────────────────────────────

func TestRunCommand_ServerNotRunning(t *testing.T) {
	d := newTestDriver(&fakeConsoleDocker{})
	instance := makeStoppedInstance()

	_, err := runCommand(context.Background(), d, instance, CommandRequest{Command: "info"}, true)
	if err == nil {
		t.Fatal("expected error for stopped server")
	}
	ce := err.(*CommandError)
	if ce.Code != "server_not_running" {
		t.Errorf("expected code 'server_not_running', got %q", ce.Code)
	}
}

func TestRunCommand_ServerStarting(t *testing.T) {
	d := newTestDriver(&fakeConsoleDocker{})
	instance := makeRunningInstance()
	instance.State = storage.InstanceStateStarting

	_, err := runCommand(context.Background(), d, instance, CommandRequest{Command: "info"}, true)
	if err == nil {
		t.Fatal("expected error for starting server")
	}
	ce := err.(*CommandError)
	if ce.Code != "server_not_running" {
		t.Errorf("expected code 'server_not_running', got %q", ce.Code)
	}
}

// ── Command execution tests ──────────────────────────────────────────────────

func TestRunCommand_ExecutesAttachCli(t *testing.T) {
	var capturedStdin string
	var capturedArgs []string

	d := newTestDriver(&fakeConsoleDocker{
		execFunc: func(_ context.Context, _, _, stdinData string, args ...string) (paneldocker.CommandResult, error) {
			if len(args) > 0 && args[0] == "tee" {
				capturedStdin = stdinData
				capturedArgs = args
			}
			return paneldocker.CommandResult{Stdout: "Server info output", ExitCode: 0}, nil
		},
	})
	instance := makeRunningInstance()

	result, err := runCommand(context.Background(), d, instance, CommandRequest{Command: "info"}, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Command != "info" {
		t.Errorf("expected command 'info', got %q", result.Command)
	}
	if result.Output != "Server info output" {
		t.Errorf("expected output 'Server info output', got %q", result.Output)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if result.DurationMS < 0 {
		t.Errorf("expected non-negative duration, got %d", result.DurationMS)
	}
	// Verify stdin contains exactly one server command.
	if capturedStdin != "info\n" {
		t.Errorf("expected stdin 'info\\n', got %q", capturedStdin)
	}
	// Verify args write to Junimo's SMAPI input FIFO.
	if len(capturedArgs) != 3 || capturedArgs[0] != "tee" || capturedArgs[1] != "-a" || capturedArgs[2] != serverInputFIFO {
		t.Errorf("expected args ['tee', '-a', %q], got %v", serverInputFIFO, capturedArgs)
	}
}

func TestRunCommand_HandlesNonZeroExit(t *testing.T) {
	d := newTestDriver(&fakeConsoleDocker{
		execFunc: func(_ context.Context, _, _, _ string, _ ...string) (paneldocker.CommandResult, error) {
			return paneldocker.CommandResult{
				Stdout:   "partial output",
				Stderr:   "some error",
				ExitCode: 1,
			}, fmt.Errorf("exec failed with exit code 1")
		},
	})
	instance := makeRunningInstance()

	result, err := runCommand(context.Background(), d, instance, CommandRequest{Command: "info"}, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 1 {
		t.Errorf("expected exit code 1, got %d", result.ExitCode)
	}
	if result.Error != "some error" {
		t.Errorf("expected stderr 'some error', got %q", result.Error)
	}
}

// ── Say tests ────────────────────────────────────────────────────────────────

func TestSendSay_RejectsEmptyMessage(t *testing.T) {
	d := newTestDriver(&fakeConsoleDocker{})
	instance := makeRunningInstance()

	_, err := sendSay(context.Background(), d, instance, "")
	if err == nil {
		t.Fatal("expected error for empty message")
	}
	ce := err.(*CommandError)
	if ce.Code != "empty_message" {
		t.Errorf("expected code 'empty_message', got %q", ce.Code)
	}
}

func TestSendSay_RejectsWhitespaceOnly(t *testing.T) {
	d := newTestDriver(&fakeConsoleDocker{})
	instance := makeRunningInstance()

	_, err := sendSay(context.Background(), d, instance, "   \n\t  ")
	if err == nil {
		t.Fatal("expected error for whitespace-only message")
	}
	ce := err.(*CommandError)
	if ce.Code != "empty_message" {
		t.Errorf("expected code 'empty_message', got %q", ce.Code)
	}
}

func TestSendSay_RejectsTooLongMessage(t *testing.T) {
	d := newTestDriver(&fakeConsoleDocker{})
	instance := makeRunningInstance()

	longMsg := strings.Repeat("a", 201)
	_, err := sendSay(context.Background(), d, instance, longMsg)
	if err == nil {
		t.Fatal("expected error for too-long message")
	}
	ce := err.(*CommandError)
	if ce.Code != "message_too_long" {
		t.Errorf("expected code 'message_too_long', got %q", ce.Code)
	}
}

func TestSendSay_AcceptsMaxLength(t *testing.T) {
	d := newTestDriver(&fakeConsoleDocker{})
	instance := makeRunningInstance()
	instance.DataDir = t.TempDir()

	msg200 := strings.Repeat("x", 200)
	result, err := sendSay(context.Background(), d, instance, msg200)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Command != "say" || result.ExitCode != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestSendSay_ServerNotRunning(t *testing.T) {
	d := newTestDriver(&fakeConsoleDocker{})
	instance := makeStoppedInstance()

	_, err := sendSay(context.Background(), d, instance, "hello")
	if err == nil {
		t.Fatal("expected error for stopped server")
	}
	ce := err.(*CommandError)
	if ce.Code != "server_not_running" {
		t.Errorf("expected code 'server_not_running', got %q", ce.Code)
	}
}

func TestSendSay_WritesBroadcastCommandFile(t *testing.T) {
	d := newTestDriver(&fakeConsoleDocker{})
	instance := makeRunningInstance()
	instance.DataDir = t.TempDir()

	result, err := sendSay(context.Background(), d, instance, "hello\nsettings show")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output == "" {
		t.Fatal("expected user-facing output")
	}

	files, err := os.ReadDir(filepath.Join(instance.DataDir, ".local-container", "control", "commands"))
	if err != nil {
		t.Fatalf("read commands dir: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 command file, got %d", len(files))
	}

	raw, err := os.ReadFile(filepath.Join(instance.DataDir, ".local-container", "control", "commands", files[0].Name()))
	if err != nil {
		t.Fatalf("read command file: %v", err)
	}
	var command struct {
		Name    string            `json:"name"`
		Payload map[string]string `json:"payload"`
	}
	if err := json.Unmarshal(raw, &command); err != nil {
		t.Fatalf("unmarshal command: %v", err)
	}
	if command.Name != "broadcast" {
		t.Fatalf("expected broadcast command, got %q", command.Name)
	}
	if got := command.Payload["message"]; got != "hello settings show" {
		t.Fatalf("expected sanitized message, got %q", got)
	}
}

// ── ListCommands tests ───────────────────────────────────────────────────────

func TestListCommands_Admin(t *testing.T) {
	cmds := ListCommands(true)
	if len(cmds) != len(commandDefs) {
		t.Errorf("expected %d commands for admin, got %d", len(commandDefs), len(cmds))
	}
}

func TestListCommands_NonAdmin(t *testing.T) {
	cmds := ListCommands(false)
	// Non-admin should only see info and invitecode.
	if len(cmds) != 2 {
		t.Errorf("expected 2 commands for non-admin, got %d: %v", len(cmds), cmds)
	}
	for _, c := range cmds {
		if c.AdminOnly {
			t.Errorf("non-admin should not see admin-only command %q", c.ID)
		}
	}
}

func TestListCommands_ContainsExpectedIDs(t *testing.T) {
	cmds := ListCommands(true)
	ids := make(map[string]bool)
	for _, c := range cmds {
		ids[c.ID] = true
	}
	expected := []string{"info", "invitecode", "settings-show", "settings-validate", "rendering-status", "host-auto", "host-visibility"}
	for _, id := range expected {
		if !ids[id] {
			t.Errorf("expected command %q in list", id)
		}
	}
}

// ── stripControlChars tests ──────────────────────────────────────────────────

func TestStripControlChars(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"hello\nworld", "hello\nworld"},
		{"hello\tworld", "hello\tworld"},
		{"hello\x00world", "helloworld"},
		{"hello\x07world", "helloworld"},
		{"\x01\x02\x03", ""},
		{"", ""},
		{"中文测试", "中文测试"},
		{"中文\x00测试", "中文测试"},
	}
	for _, tt := range tests {
		got := stripControlChars(tt.input)
		if got != tt.want {
			t.Errorf("stripControlChars(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ── sanitizeSayMessage tests ─────────────────────────────────────────────────

func TestSanitizeSayMessage(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"hello world", "hello world"},
		{"hello\nworld", "hello world"},
		{"hello\r\nworld", "hello world"},
		{"hello\tworld", "hello world"},
		{"hello\x00world", "hello world"},
		{"hello\x07world", "hello world"},
		{"  hello  world  ", "hello world"},
		{"hello\n\n\nworld", "hello world"},
		{"\x01\x02\x03", ""},
		{"", ""},
		{"中文测试", "中文测试"},
		{"中文\n测试", "中文 测试"},
		{"hello\nsettings show\nquit", "hello settings show quit"},
	}
	for _, tt := range tests {
		got := sanitizeSayMessage(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeSayMessage(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
