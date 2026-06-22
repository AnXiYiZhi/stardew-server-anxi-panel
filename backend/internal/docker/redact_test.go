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
		`{"token":"token-secret","secret":"top-secret"}`,
		"--password=flag-secret",
	}, "\n")

	output := RedactString(input)
	for _, secret := range []string{"plain", "steam-secret", "vnc-secret", "token-secret", "top-secret", "flag-secret"} {
		if strings.Contains(output, secret) {
			t.Fatalf("expected %q to be redacted from %q", secret, output)
		}
	}
	if !strings.Contains(output, redacted) {
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
