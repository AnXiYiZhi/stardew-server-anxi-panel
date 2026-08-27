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

func TestSteamAuthDockerRunArgsUsesExactOneOffScopeWithoutSecrets(t *testing.T) {
	opts := SteamAuthRunOpts{
		ImageRef: "example.test/steam-auth:fixed",
		Command:  []string{"login"},
		Env:      []string{"GAME_DIR=/tmp/auth-game", "STEAM_USERNAME=shared-user", "STEAM_PASSWORD=secret-value"},
		Binds:    []string{"preview_steam-session:/data/steam-session"},
	}
	got := steamAuthDockerRunArgs("anxi-steam-auth-test", opts)
	want := []string{
		"run", "--name", "anxi-steam-auth-test", "--rm", "--interactive", "--tty",
		"--env", "GAME_DIR", "--env", "STEAM_USERNAME", "--env", "STEAM_PASSWORD",
		"--volume", "preview_steam-session:/data/steam-session",
		"example.test/steam-auth:fixed", "login",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %#v, want %#v", got, want)
	}
	for _, arg := range got {
		if arg == "secret-value" || arg == "STEAM_PASSWORD=secret-value" {
			t.Fatalf("Steam password leaked into docker argv: %#v", got)
		}
	}
}

func TestEnvironmentWithOverridesRemovesConflictingHostValues(t *testing.T) {
	got := environmentWithOverrides(
		[]string{"PATH=/usr/bin", "STEAM_PASSWORD=host-secret", "GAME_DIR=/host/game"},
		[]string{"STEAM_PASSWORD=task-secret", "GAME_DIR=/tmp/auth-game"},
	)
	want := []string{"PATH=/usr/bin", "STEAM_PASSWORD=task-secret", "GAME_DIR=/tmp/auth-game"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("environment = %#v, want %#v", got, want)
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
