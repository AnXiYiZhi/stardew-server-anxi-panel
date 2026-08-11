//go:build !windows

package docker

import (
	"context"
	"errors"
	"reflect"
	"regexp"
	"testing"
	"time"
)

func TestSteamAuthComposeRunArgsAssignsExactOneOffName(t *testing.T) {
	opts := SteamAuthRunOpts{Command: []string{"setup", "--qr"}}
	got := steamAuthComposeRunArgs("/instance/docker-compose.yml", "anxi-steam-auth-test", opts)
	want := []string{
		"compose", "-f", "/instance/docker-compose.yml", "run",
		"--name", "anxi-steam-auth-test", "--rm", "--interactive", "--tty",
		"steam-auth", "setup", "--qr",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %#v, want %#v", got, want)
	}
}

func TestSteamAuthContainerNamesAreSafeAndUnique(t *testing.T) {
	pattern := regexp.MustCompile(`^anxi-steam-auth-[0-9a-f]+$`)
	seen := map[string]bool{}
	for range 64 {
		name := newSteamAuthContainerName()
		if !pattern.MatchString(name) {
			t.Fatalf("unsafe container name %q", name)
		}
		if seen[name] {
			t.Fatalf("duplicate container name %q", name)
		}
		seen[name] = true
	}
}

func TestRemoveNamedSteamAuthContainerWaitsForLateCreation(t *testing.T) {
	const containerName = "anxi-steam-auth-late"
	lsCalls := 0
	removed := false
	command := func(_ context.Context, args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "container" && args[1] == "ls" {
			lsCalls++
			if !removed && lsCalls == 4 {
				return []byte("late-container-id\n"), nil
			}
			return nil, nil
		}
		if len(args) == 4 && args[0] == "container" && args[1] == "rm" && args[3] == containerName {
			removed = true
			return []byte(containerName), nil
		}
		return nil, errors.New("unexpected cleanup command")
	}

	started := time.Now()
	err := removeNamedSteamAuthContainerWithCommand(
		containerName,
		500*time.Millisecond,
		40*time.Millisecond,
		5*time.Millisecond,
		command,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !removed {
		t.Fatal("late-created container was not removed")
	}
	if elapsed := time.Since(started); elapsed < 50*time.Millisecond {
		t.Fatalf("cleanup returned before stable absence window: %s", elapsed)
	}
}
