package docker

import (
	"strings"
	"testing"
)

func TestRedactString(t *testing.T) {
	input := strings.Join([]string{
		"password=plain",
		"Password: plain",
		"STEAM_PASSWORD=steam-secret",
		"VNC_PASSWORD=vnc-secret",
		"refresh_token=refresh-secret",
		"app_ticket=ticket-secret",
		`{"STEAM_REFRESH_TOKEN":"json-refresh","STEAM_APP_TICKET":"json-ticket"}`,
		`{"token":"token-secret","secret":"top-secret"}`,
		"--password=flag-secret",
	}, "\n")

	output := RedactString(input)
	for _, secret := range []string{"plain", "steam-secret", "vnc-secret", "refresh-secret", "ticket-secret", "json-refresh", "json-ticket", "token-secret", "top-secret", "flag-secret"} {
		if strings.Contains(output, secret) {
			t.Fatalf("expected %q to be redacted from %q", secret, output)
		}
	}
	if !strings.Contains(output, Redacted) {
		t.Fatalf("expected redaction marker in %q", output)
	}
}

func TestRedactArgs(t *testing.T) {
	args := RedactArgs([]string{"docker", "login", "--password", "secret", "--token=abc"})
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "secret") || strings.Contains(joined, "abc") {
		t.Fatalf("expected args to be redacted, got %q", joined)
	}
}

func TestRedactString_SessionCookieAuth(t *testing.T) {
	tests := []struct {
		name  string
		input string
		leak  string // must NOT appear in output
	}{
		{"session in JSON", `{"session":"abc123session"}`, "abc123session"},
		{"cookie assignment", "cookie=monster-session-value", "monster-session-value"},
		{"authorization header", "Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.signature", "eyJhbGciOiJIUzI1NiJ9.signature"},
		{"api_key in JSON", `{"api_key":"sk-1234567890abcdef"}`, "sk-1234567890abcdef"},
		{"apikey assignment", "apikey=my-secret-api-key-12345", "my-secret-api-key-12345"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := RedactString(tt.input)
			if strings.Contains(output, tt.leak) {
				t.Errorf("sensitive value %q leaked in output: %s", tt.leak, output)
			}
		})
	}
}

func TestRedactString_InviteCode(t *testing.T) {
	tests := []struct {
		name  string
		input string
		leak  string
	}{
		{"invite code english", "invite code: ABCD1234", "ABCD1234"},
		{"invite code chinese", "邀请码=EFGH5678", "EFGH5678"},
		{"invite code spaced", "invite code = XYZW9012", "XYZW9012"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := RedactString(tt.input)
			if strings.Contains(output, tt.leak) {
				t.Errorf("invite code %q leaked in output: %s", tt.leak, output)
			}
		})
	}
}

func TestRedactString_BearerToken(t *testing.T) {
	input := "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U"
	output := RedactString(input)
	if strings.Contains(output, "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9") {
		t.Errorf("Bearer token leaked in output: %s", output)
	}
	if !strings.Contains(output, "Bearer") {
		t.Errorf("Bearer prefix should be preserved: %s", output)
	}
}

func TestRedactString_EmptyInput(t *testing.T) {
	if RedactString("") != "" {
		t.Error("expected empty string for empty input")
	}
}

func TestRedactString_NoFalsePositives(t *testing.T) {
	// These should NOT be redacted
	safe := []string{
		"Server is running",
		"Farm name: My Farm",
		"Year 3, Spring, Day 15",
		"Player count: 4",
	}
	for _, s := range safe {
		output := RedactString(s)
		if output != s {
			t.Errorf("false positive redaction on %q: got %q", s, output)
		}
	}
}

func TestRedactArgs_TwoArgFlags(t *testing.T) {
	args := RedactArgs([]string{"docker", "exec", "--env", "SECRET_KEY=abc123", "container"})
	joined := strings.Join(args, " ")
	// The --env flag with a separate value should redact the next arg
	if strings.Contains(joined, "abc123") {
		t.Errorf("two-arg flag value leaked: %s", joined)
	}
}
