package docker

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

type currentReaderFixture struct {
	client                                           *Client
	dir, volume, image, state, started, events, args string
}

func newCurrentReaderFixture(t *testing.T, mode string) currentReaderFixture {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("executable shell fixture requires Linux")
	}
	f := currentReaderFixture{dir: t.TempDir(), volume: "existing-game", image: "sha256:" + strings.Repeat("f", 64)}
	if mode == "staging live" {
		f.volume = "world-a_anxi-smapi-update-" + strings.Repeat("a", 24)
	}
	f.state = filepath.Join(f.dir, "helper-state")
	f.started = filepath.Join(f.dir, "helper-started")
	f.events = filepath.Join(f.dir, "events")
	f.args = filepath.Join(f.dir, "create-args")
	project := "world-a"
	if mode == "foreign live" {
		project = "foreign"
	}
	sourceID, helperID, auto := strings.Repeat("a", 64), strings.Repeat("b", 64), strings.Repeat("c", 64)
	holders, _ := json.Marshal([]map[string]any{{"ID": sourceID, "Config": map[string]any{"Labels": map[string]string{"com.docker.compose.project": project, "com.docker.compose.service": "server", "com.docker.compose.oneoff": "False"}}, "State": map[string]bool{"Running": true, "Restarting": false}, "Mounts": []map[string]string{{"Type": "volume", "Name": f.volume, "Destination": "/data/game"}}}})
	metadata, _ := json.Marshal([]map[string]any{{"Name": f.volume, "Labels": map[string]string{"com.anxi-panel.smapi-update-staging": "true", "com.anxi-panel.compose-project": "world-a"}}})
	user := "0:0"
	if mode == "identity changed" {
		user = "1000"
	}
	helper, _ := json.Marshal([]map[string]any{{"ID": helperID, "Name": "/FIXTURE-NAME", "Image": f.image,
		"Config":     map[string]any{"Image": f.image, "User": user, "Labels": map[string]string{"com.anxi-panel.current-runtime-read": "true", "com.anxi-panel.game-data-volume": f.volume}, "Volumes": map[string]any{"/config": map[string]any{}}},
		"HostConfig": map[string]any{"NetworkMode": "none", "ReadonlyRootfs": true, "Privileged": false, "AutoRemove": false, "CapDrop": []string{"ALL"}, "CapAdd": []string{"DAC_READ_SEARCH"}, "SecurityOpt": []string{"no-new-privileges"}, "Binds": []string{}, "VolumesFrom": []string{}, "Mounts": []map[string]any{{"Type": "volume", "Source": f.volume, "Target": "/game", "ReadOnly": true, "VolumeOptions": map[string]bool{"NoCopy": true}}}},
		"Mounts":     []map[string]any{{"Type": "volume", "Name": f.volume, "Destination": "/game", "RW": false}, {"Type": "volume", "Name": auto, "Destination": "/config", "RW": true}}}})
	quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }
	imageOutput := `"` + f.image + `"|["sdvd/server@sha256:f23fdc3a3e999032be363fa43291c2d72626555913ef2b3dcf71b0aec53c49db"]`
	script := "#!/bin/sh\nset -eu\nstate=" + quote(f.state) + "\nevents=" + quote(f.events) + "\nstarted=" + quote(f.started) + "\ncase \"$1\" in\n"
	script += "volume) if [ \"$2\" = inspect ]; then printf '%s\\n' " + quote(string(metadata)) + "; fi ;;\nps) printf 'known-holder\\n' ;;\n"
	script += "image) case \"$4\" in *Config.Volumes*) printf '%s\\n' " + quote(`"`+f.image+`"|{"/config":{}}`) + " ;; *) printf '%s\\n' " + quote(imageOutput) + " ;; esac ;;\n"
	script += "create) printf 'CREATE\\n' >> \"$events\"; printf '%s\\n' \"$@\" > " + quote(f.args) + "; previous=''; name=''; for arg do if [ \"$previous\" = --name ]; then name=$arg; fi; previous=$arg; done; printf '%s' \"$name\" > \"$state\"; "
	if mode == "unknown response" {
		script += "printf 'unknown-response\\n'"
	} else {
		script += "printf '%s\\n' " + quote(helperID)
	}
	script += " ;;\nstart) printf 'START\\n' >> \"$events\"; : > \"$started\"; "
	if mode == "cancel" {
		script += "exec sleep 30"
	} else {
		script += "printf 'fixed-read-evidence'"
	}
	script += " ;;\ncontainer) case \"$2\" in\nls) last=''; for arg do last=$arg; done; case \"$last\" in volume=" + auto + ") if [ -f \"$state\" ]; then printf '%s\\n' " + quote(helperID) + "; fi ;; volume=*) printf '%s\\n' " + quote(sourceID) + " ;; *) if [ -f \"$state\" ]; then printf '%s\\n' " + quote(helperID) + "; fi ;; esac ;;\n"
	script += "inspect) if [ \"$3\" = " + quote(helperID) + " ]; then name=$(cat \"$state\"); printf '%s\\n' " + quote(string(helper)) + " | sed \"s/FIXTURE-NAME/$name/\"; else printf '%s\\n' " + quote(string(holders)) + "; fi ;;\n"
	script += "rm) printf 'REMOVE\\n' >> \"$events\"; rm -f \"$state\"; printf '%s\\n' " + quote(helperID) + " ;;\n*) exit 42 ;;\nesac ;;\n*) exit 41 ;;\nesac\n"
	binary := filepath.Join(f.dir, "docker-fixture")
	if err := os.WriteFile(binary, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	f.client = NewClient(Options{DockerPath: binary})
	return f
}

func TestCurrentJunimo140CanceledReadRemovesOnlyAcknowledgedHelper(t *testing.T) {
	f := newCurrentReaderFixture(t, "cancel")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := f.client.RuntimeReadCurrentJunimo140GameEvidence(ctx, f.dir, f.volume, "sdvd/server:1.5.0-preview.140", f.image, "game", []string{"world-a"})
		done <- err
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(f.started); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("owned helper never reached start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancellation became success")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("canceled reader did not finish precise cleanup")
	}
	if _, err := os.Stat(f.state); !os.IsNotExist(err) {
		t.Fatal("canceled reader left an owned holder")
	}
	events, _ := os.ReadFile(f.events)
	if string(events) != "CREATE\nSTART\nREMOVE\n" {
		t.Fatal("canceled read replayed a native action", string(events))
	}
}

func TestCurrentJunimo140UnknownCreateResponseIsReconciledWithoutRestart(t *testing.T) {
	f := newCurrentReaderFixture(t, "unknown response")
	if _, err := f.client.RuntimeReadCurrentJunimo140GameEvidence(context.Background(), f.dir, f.volume, "sdvd/server:1.5.0-preview.140", f.image, "game", []string{"world-a"}); err == nil {
		t.Fatal("unknown response became success")
	}
	events, _ := os.ReadFile(f.events)
	if string(events) != "CREATE\nREMOVE\n" {
		t.Fatal("unknown response caused replay/start or missed owned cleanup", string(events))
	}
	if _, err := os.Stat(f.state); !os.IsNotExist(err) {
		t.Fatal("same created helper was not reconciled")
	}
}

func TestCurrentJunimo140ChangedHelperIdentityIsNeverRemoved(t *testing.T) {
	f := newCurrentReaderFixture(t, "identity changed")
	if _, err := f.client.RuntimeReadCurrentJunimo140GameEvidence(context.Background(), f.dir, f.volume, "sdvd/server:1.5.0-preview.140", f.image, "game", []string{"world-a"}); err == nil {
		t.Fatal("changed ownership became success")
	}
	events, _ := os.ReadFile(f.events)
	if string(events) != "CREATE\n" {
		t.Fatal("changed helper was started or removed", string(events))
	}
	if _, err := os.Stat(f.state); err != nil {
		t.Fatal("foreign identity was guessed away")
	}
}
