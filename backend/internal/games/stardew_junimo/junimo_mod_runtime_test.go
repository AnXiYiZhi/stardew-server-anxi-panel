package stardew_junimo

import (
	"context"
	"runtime"
	"strings"
	"testing"
)

func TestExtractJunimoServerModUsesNumericRootWithoutPasswdEntry(t *testing.T) {
	dir := t.TempDir()
	docker := &fakeDocker{containerHasNoNamedRoot: true, junimoExtractVersion: "1.5.0-preview.140"}
	extracted, err := extractJunimoServerMod(context.Background(), docker, serverImageDefault("1.5.0-preview.140"), dir, dir, "1.5.0-preview.140")
	if err != nil {
		t.Fatal(err)
	}
	if docker.containerRuns != 1 || docker.containerOpts.User != "0:0" {
		t.Fatalf("Junimo extraction runs=%d user=%q; want one numeric UID:GID 0:0 helper", docker.containerRuns, docker.containerOpts.User)
	}
	if err := validateExtractedJunimoServerMod(extracted, "1.5.0-preview.140"); err != nil {
		t.Fatal(err)
	}
}

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
