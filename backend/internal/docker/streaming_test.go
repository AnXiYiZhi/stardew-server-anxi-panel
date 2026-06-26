package docker

import (
	"strings"
	"testing"
)

func TestStreamTTYOutputEmitsInteractivePromptWithoutNewline(t *testing.T) {
	var lines []string
	streamTTYOutput(strings.NewReader("[SteamAuth:A0] Connected to Steam\r\nEnter Steam Guard code sent to qq.com: "), func(line string) {
		lines = append(lines, line)
	})

	if len(lines) != 2 {
		t.Fatalf("expected 2 emitted lines, got %d: %#v", len(lines), lines)
	}
	if !strings.Contains(lines[1], "Enter Steam Guard code sent to qq.com:") {
		t.Fatalf("expected guard prompt, got %#v", lines)
	}
}
