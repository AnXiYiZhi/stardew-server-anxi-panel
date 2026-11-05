package docker

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

// This opt-in test uses only its own synthetic inner-Docker volume. It does not
// establish game/SDK ABI compatibility or touch an existing game/Auth holder.
func TestCurrentJunimo140NativeReadPermissionsAndCancellation(t *testing.T) {
	root := os.Getenv("PANEL_CURRENT_READ_NATIVE_EVIDENCE")
	if root == "" {
		t.Skip("requires an explicitly owned isolated Docker test daemon")
	}
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	prefix := "anxi-noop-read-" + hex.EncodeToString(nonce[:])
	volume := prefix + "-game"
	const image = "sha256:f23fdc3a3e999032be363fa43291c2d72626555913ef2b3dcf71b0aec53c49db"
	const taskLabel = "com.anxi-panel.native-current-read"
	sequence := 0
	command := func(write bool, args ...string) ([]byte, error) {
		sequence++
		stem := filepath.Join(root, fmt.Sprintf("%03d", sequence))
		if write {
			encoded, _ := json.Marshal(args)
			digest := sha256.Sum256(encoded)
			intent, _ := json.Marshal(map[string]any{"argumentCount": len(args), "argvSHA256": hex.EncodeToString(digest[:]), "taskPrefix": prefix})
			if err := os.WriteFile(stem+".intent.safe.json", intent, 0600); err != nil {
				t.Fatal(err)
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
		if write {
			// Only this test's explicitly addressed resources occur in write output.
			if saveErr := os.WriteFile(stem+".response.private.bin", out, 0600); saveErr != nil {
				t.Fatal(saveErr)
			}
		}
		return out, err
	}
	must := func(write bool, args ...string) []byte {
		t.Helper()
		out, err := command(write, args...)
		if err != nil {
			t.Fatalf("native command failed (%s), private evidence retained", args[0])
		}
		return out
	}
	inventory := func() []string {
		var result []string
		for _, args := range [][]string{{"container", "ls", "--all", "--quiet", "--no-trunc"}, {"volume", "ls", "--quiet"}, {"network", "ls", "--quiet", "--no-trunc"}} {
			rows := strings.Fields(string(must(false, args...)))
			sort.Strings(rows)
			result = append(result, strings.Join(rows, "\n"))
		}
		return result
	}
	must(false, "info", "--format", "{{.ServerVersion}}")
	before := inventory()
	imageRaw := must(false, "image", "inspect", image)
	var images []struct {
		ID          string
		RepoDigests []string
		Config      struct{ Volumes map[string]json.RawMessage }
	}
	if json.Unmarshal(imageRaw, &images) != nil || len(images) != 1 || images[0].ID != image {
		t.Fatal("fixed 140 image unavailable")
	}
	rootBound := false
	for _, digest := range images[0].RepoDigests {
		rootBound = rootBound || strings.HasSuffix(digest, "@"+image)
	}
	if !rootBound {
		t.Fatal("fixed 140 repository digest unavailable")
	}
	if len(strings.TrimSpace(string(must(false, "volume", "ls", "--quiet", "--filter", "name=^"+volume+"$")))) != 0 {
		t.Fatal("fixture volume already exists")
	}
	createdVolume := strings.TrimSpace(string(must(true, "volume", "create", "--label", taskLabel+"="+prefix, volume)))
	if createdVolume != volume {
		t.Fatal("volume create was not acknowledged")
	}
	if err := os.WriteFile(filepath.Join(root, "accepted-volume.private.txt"), []byte(volume), 0600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if strings.TrimSpace(string(must(false, "container", "ls", "--all", "--quiet", "--filter", "volume="+volume))) != "" {
			t.Error("owned source still has a holder")
			return
		}
		var rows []struct {
			Name   string
			Labels map[string]string
		}
		if json.Unmarshal(must(false, "volume", "inspect", volume), &rows) != nil || len(rows) != 1 || rows[0].Name != volume || rows[0].Labels[taskLabel] != prefix {
			t.Error("owned source attribution changed")
			return
		}
		must(true, "volume", "rm", volume)
		if !reflect.DeepEqual(before, inventory()) {
			t.Error("isolated daemon resource inventory changed")
		}
	})
	// These setup/negative readers override every fixed image automatic volume
	// with tmpfs; production readers below exercise automatic-volume cleanup.
	owned := func(name string, readonly bool, caps []string, script string) ([]byte, error) {
		t.Helper()
		args := []string{"create", "--name", name, "--label", taskLabel + "=" + prefix, "--pull", "never", "--network", "none", "--user", "0:0", "--read-only", "--security-opt", "no-new-privileges"}
		args = append(args, caps...)
		mount := "type=volume,src=" + volume + ",dst=/game,volume-nocopy"
		if readonly {
			mount += ",readonly"
		}
		args = append(args, "--mount", mount)
		var destinations []string
		for destination := range images[0].Config.Volumes {
			destinations = append(destinations, destination)
		}
		sort.Strings(destinations)
		for _, destination := range destinations {
			if destination == "/game" {
				t.Fatal("unexpected fixed image /game declaration")
			}
			args = append(args, "--mount", "type=tmpfs,dst="+destination)
		}
		args = append(args, "--entrypoint", "sh", image, "-c", script)
		id := strings.TrimSpace(string(must(true, args...)))
		if !dockerContainerIDPattern.MatchString(id) || len(id) != 64 {
			t.Fatal("owned native helper ID was not acknowledged")
		}
		if err := os.WriteFile(filepath.Join(root, name+".accepted.private.txt"), []byte(id), 0600); err != nil {
			t.Fatal(err)
		}
		var rows []struct {
			ID, Image  string
			Config     struct{ Labels map[string]string }
			HostConfig struct{ NetworkMode string }
			Mounts     []struct{ Type, Name, Destination string }
		}
		if json.Unmarshal(must(false, "container", "inspect", id), &rows) != nil || len(rows) != 1 || rows[0].ID != id || rows[0].Image != image || rows[0].Config.Labels[taskLabel] != prefix || rows[0].HostConfig.NetworkMode != "none" {
			t.Fatal("owned fixture helper attribution changed")
		}
		for _, m := range rows[0].Mounts {
			if m.Type == "volume" && (m.Name != volume || m.Destination != "/game") {
				t.Fatal("unexpected automatic fixture volume")
			}
		}
		defer must(true, "container", "rm", "--force", id)
		return command(true, "start", "--attach", id)
	}
	const payload = "synthetic-private-sdk-acf"
	if _, err := owned(prefix+"-seed", false, nil, "set -eu; mkdir -p /game/.steam-sdk/steamapps; printf '%s' '"+payload+"' > /game/.steam-sdk/steamapps/appmanifest_1007.acf; chown -R 1000:1000 /game/.steam-sdk; chmod 0700 /game/.steam-sdk /game/.steam-sdk/steamapps; chmod 0600 /game/.steam-sdk/steamapps/appmanifest_1007.acf"); err != nil {
		t.Fatal("fixture setup failed")
	}
	if _, err := owned(prefix+"-denied", true, []string{"--cap-drop", "ALL"}, "cat /game/.steam-sdk/steamapps/appmanifest_1007.acf"); err == nil {
		t.Fatal("UID0 without DAC capability unexpectedly read owner1000/0600")
	}
	c := NewClient(Options{})
	dir := t.TempDir()
	got, err := c.RuntimeReadCurrentJunimo140GameEvidence(context.Background(), dir, volume, "sdvd/server:1.5.0-preview.140", image, "sdk", []string{prefix})
	if err != nil || string(got) != payload {
		t.Fatalf("fixed140 DAC-only production reader failed: %v", err)
	}
	// Cancel while the actual helper is running. The production independent
	// cleanup context must remove that exact container and its automatic volume.
	helperArgs, err := junimo140ReadHelperArgs(volume, "native-cancel", []string{prefix})
	if err != nil {
		t.Fatal(err)
	}
	name := currentReadArgument(helperArgs, "--name")
	args := append([]string{"run", "--rm"}, helperArgs...)
	args = append(args, "--pull", "never", "--network", "none", "--read-only", "--mount", junimo140ReadMount(volume, []string{prefix}), "--entrypoint", "sleep", image, "30")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, _, readErr := c.runJunimo140Read(ctx, "native cancellation test", dir, time.Minute, nil, volume, image, []string{prefix}, args...)
		done <- readErr
	}()
	deadline := time.Now().Add(10 * time.Second)
	running := false
	for time.Now().Before(deadline) {
		out, _ := command(false, "container", "ls", "--quiet", "--filter", "name=^/"+name+"$")
		if len(strings.Fields(string(out))) == 1 {
			running = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancellation unexpectedly succeeded")
		}
	case <-time.After(25 * time.Second):
		t.Fatal("cancellation cleanup exceeded its budget")
	}
	if !running {
		t.Fatal("actual production helper did not enter running state")
	}
	if strings.TrimSpace(string(must(false, "container", "ls", "--all", "--quiet", "--filter", "name=^/"+name+"$"))) != "" {
		t.Fatal("canceled production helper remained")
	}
	if _, err := owned(prefix+"-unchanged", true, []string{"--cap-drop", "ALL", "--cap-add", "DAC_READ_SEARCH"}, "set -eu; test \"$(stat -c '%u:%g:%a' /game/.steam-sdk/steamapps/appmanifest_1007.acf)\" = '1000:1000:600'; test \"$(cat /game/.steam-sdk/steamapps/appmanifest_1007.acf)\" = '"+payload+"'"); err != nil {
		t.Fatal("source content or permissions changed")
	}
	t.Log("fixed140 native: no-cap denied; DAC_READ_SEARCH read owner1000/0600; cancellation removed owned helper/automatic volumes; source unchanged")
}
