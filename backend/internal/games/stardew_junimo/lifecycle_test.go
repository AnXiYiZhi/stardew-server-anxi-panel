package stardew_junimo

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func TestParseInviteCode_ValidPatterns(t *testing.T) {
	cases := []struct {
		output string
		want   string
	}{
		{"Invite Code: ABCD-1234-EFGH", "ABCD-1234-EFGH"},
		{"invitecode: XY12-3456-ABCD", "XY12-3456-ABCD"},
		{"InviteCode: AA11-BB22-CC33", "AA11-BB22-CC33"},
		{"some output\nABCD-1234\nmore", "ABCD-1234"},
		// Galaxy P2P codes have no hyphens
		{"Invite Code: SGCWS0Z572F2", "SGCWS0Z572F2"},
		{"(Invite code: SGCWS0Z572F2)", "SGCWS0Z572F2"},
		{"some output\nSGCWS0Z572F2\nmore", "SGCWS0Z572F2"},
		{"no code here", ""},
		{"", ""},
	}
	for _, tc := range cases {
		got := parseInviteCode(tc.output)
		if got != tc.want {
			t.Errorf("parseInviteCode(%q) = %q, want %q", tc.output, got, tc.want)
		}
	}
}

func TestMergeInviteCodeInPayload(t *testing.T) {
	result := mergeInviteCodeInPayload(`{"save_strategy":"new_game"}`, "ABCD-1234-WXYZ")
	if !containsStr(result, `"invite_code"`) {
		t.Errorf("invite_code not in payload: %s", result)
	}
	if !containsStr(result, "ABCD-1234-WXYZ") {
		t.Errorf("invite code value not in payload: %s", result)
	}
	if !containsStr(result, "save_strategy") {
		t.Errorf("existing key lost in merge: %s", result)
	}
}

func TestMergeInviteCodeInPayload_EmptyExisting(t *testing.T) {
	result := mergeInviteCodeInPayload("", "XXXX-1111")
	if !containsStr(result, `"invite_code"`) {
		t.Errorf("invite_code not in payload: %s", result)
	}
}

func TestInviteCodeFromPayload(t *testing.T) {
	if got := inviteCodeFromPayload(`{"invite_code":"SGD0XEES7LO2"}`); got != "SGD0XEES7LO2" {
		t.Fatalf("inviteCodeFromPayload() = %q", got)
	}
	if got := inviteCodeFromPayload(`{"other":"value"}`); got != "" {
		t.Fatalf("expected empty invite code, got %q", got)
	}
}

func TestClearStaleInviteCodeRemovesOnlyStoredOldCode(t *testing.T) {
	var calls [][]string
	fake := &fakeConsoleDocker{
		execFunc: func(_ context.Context, _, _, _ string, args ...string) (paneldocker.CommandResult, error) {
			calls = append(calls, append([]string{}, args...))
			if reflect.DeepEqual(args, []string{"cat", "/tmp/invite-code.txt"}) {
				return paneldocker.CommandResult{Stdout: "OLD-CODE\n", ExitCode: 0}, nil
			}
			return paneldocker.CommandResult{ExitCode: 0}, nil
		},
	}
	runner := &lifecycleRunner{
		lifecycle: fake,
		instance: storage.Instance{
			DataDir:       "custom-dir",
			DriverPayload: `{"invite_code":"OLD-CODE"}`,
		},
	}

	runner.clearStaleInviteCode(context.Background(), nil)

	if len(calls) != 2 {
		t.Fatalf("expected cat and rm calls, got %d: %#v", len(calls), calls)
	}
	if !reflect.DeepEqual(calls[1], []string{"rm", "-f", "/tmp/invite-code.txt"}) {
		t.Fatalf("expected rm stale invite call, got %#v", calls[1])
	}
}

func TestClearStaleInviteCodeKeepsFreshCode(t *testing.T) {
	var calls [][]string
	fake := &fakeConsoleDocker{
		execFunc: func(_ context.Context, _, _, _ string, args ...string) (paneldocker.CommandResult, error) {
			calls = append(calls, append([]string{}, args...))
			return paneldocker.CommandResult{Stdout: "NEW-CODE\n", ExitCode: 0}, nil
		},
	}
	runner := &lifecycleRunner{
		lifecycle: fake,
		instance: storage.Instance{
			DataDir:       "custom-dir",
			DriverPayload: `{"invite_code":"OLD-CODE"}`,
		},
	}

	runner.clearStaleInviteCode(context.Background(), nil)

	if len(calls) != 1 {
		t.Fatalf("expected only cat call, got %d: %#v", len(calls), calls)
	}
	if !reflect.DeepEqual(calls[0], []string{"cat", "/tmp/invite-code.txt"}) {
		t.Fatalf("expected cat invite call, got %#v", calls[0])
	}
}

func TestLooksLikePortBindFailure(t *testing.T) {
	cases := []struct {
		name string
		text string
		want bool
	}{
		{
			name: "windows reserved port",
			text: "ports are not available: exposing port TCP 0.0.0.0:5800 -> 127.0.0.1:0: listen tcp 0.0.0.0:5800: bind: An attempt was made to access a socket in a way forbidden by its access permissions.",
			want: true,
		},
		{
			name: "already allocated",
			text: "Bind for 0.0.0.0:5800 failed: port is already allocated",
			want: true,
		},
		{
			name: "non port docker error",
			text: "docker compose up: docker command failed",
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := looksLikePortBindFailure(tc.text)
			if got != tc.want {
				t.Fatalf("looksLikePortBindFailure() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestEnsureJunimoServerModCopiesFromServerImage(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".local-container", "mods"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("SERVER_IMAGE=sdvd/server:custom\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var gotOpts paneldocker.ContainerTTYRunOpts
	fake := &fakeConsoleDocker{
		runContainerFunc: func(_ context.Context, opts paneldocker.ContainerTTYRunOpts, _ <-chan string, lineHandler func(string)) (int, error) {
			gotOpts = opts
			lineHandler("JunimoServer synced")
			return 0, nil
		},
	}
	runner := &lifecycleRunner{
		lifecycle: fake,
		instance:  storage.Instance{DataDir: dir},
	}

	if err := runner.ensureJunimoServerMod(context.Background(), nil); err != nil {
		t.Fatalf("ensureJunimoServerMod: %v", err)
	}
	if gotOpts.ImageRef != "sdvd/server:custom" {
		t.Fatalf("ImageRef = %q, want custom server image", gotOpts.ImageRef)
	}
	if len(gotOpts.Entrypoint) != 1 || gotOpts.Entrypoint[0] != "/bin/sh" {
		t.Fatalf("unexpected entrypoint: %#v", gotOpts.Entrypoint)
	}
	if len(gotOpts.Command) != 2 || !strings.Contains(gotOpts.Command[1], "/data/Mods/JunimoServer") {
		t.Fatalf("copy command should reference JunimoServer, got %#v", gotOpts.Command)
	}
	if len(gotOpts.Binds) != 1 || !strings.HasSuffix(gotOpts.Binds[0], string(filepath.Separator)+".local-container"+string(filepath.Separator)+"mods:/out") {
		t.Fatalf("unexpected binds: %#v", gotOpts.Binds)
	}
}

func TestEnsureJunimoServerModSkipsWhenAlreadyPresent(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, ".local-container", "mods", "JunimoServer", "manifest.json")
	if err := os.MkdirAll(filepath.Dir(manifest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	called := false
	fake := &fakeConsoleDocker{
		runContainerFunc: func(context.Context, paneldocker.ContainerTTYRunOpts, <-chan string, func(string)) (int, error) {
			called = true
			return 0, nil
		},
	}
	runner := &lifecycleRunner{
		lifecycle: fake,
		instance:  storage.Instance{DataDir: dir},
	}

	if err := runner.ensureJunimoServerMod(context.Background(), nil); err != nil {
		t.Fatalf("ensureJunimoServerMod: %v", err)
	}
	if called {
		t.Fatal("RunContainerTTY should not be called when JunimoServer manifest exists")
	}
}
