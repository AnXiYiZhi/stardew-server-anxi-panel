package stardew_junimo

import (
	"runtime"
	"strings"
	"testing"
)

func TestExtractedJunimoOwnershipCommandUsesNumericIdentity(t *testing.T) {
	command, err := extractedJunimoOwnershipCommand()
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		if command != "" {
			t.Fatalf("Windows extraction should not request chown: %q", command)
		}
		return
	}
	if !strings.HasPrefix(command, "chown -R ") || !strings.HasSuffix(command, " /out/"+runtimeTargetJunimoDir) {
		t.Fatalf("unexpected ownership command: %q", command)
	}
}

func TestNumericIdentityRejectsShellSyntax(t *testing.T) {
	for _, value := range []string{"", "-1", "1:2", "1000;id", "$(id)", " 1000"} {
		if isNumericIdentity(value) {
			t.Fatalf("unsafe identity %q was accepted", value)
		}
	}
	for _, value := range []string{"0", "1000", "65534"} {
		if !isNumericIdentity(value) {
			t.Fatalf("numeric identity %q was rejected", value)
		}
	}
}
