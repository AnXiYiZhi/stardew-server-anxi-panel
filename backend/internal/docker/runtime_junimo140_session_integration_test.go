//go:build integration

package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestJunimo140ProtectedSessionArchiveRealOptIn(t *testing.T) {
	if os.Getenv("ANXI_REAL_JUNIMO140_ARCHIVE") != "1" {
		t.Skip("set ANXI_REAL_JUNIMO140_ARCHIVE=1")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	client, dir := NewClient(Options{DockerPath: "docker"}), t.TempDir()
	const ref = "sdvd/server:1.5.0-preview.140"
	image, err := client.RuntimeImageInspect(ctx, dir, ref)
	if err != nil || image.Digest != "sha256:f23fdc3a3e999032be363fa43291c2d72626555913ef2b3dcf71b0aec53c49db" {
		t.Fatalf("fixed image unavailable: %v", err)
	}
	var nonce [12]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	identity := hex.EncodeToString(nonce[:])
	prefix := "junimo140-session-" + identity
	fixture, session, writer := prefix+"-fixture", prefix+"-session", prefix+"-writer"
	run := func(input []byte, args ...string) string {
		t.Helper()
		cmd := exec.CommandContext(ctx, "docker", args...)
		cmd.Stdin = bytes.NewReader(input)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("isolated session fixture failed: %v: %s", err, output)
		}
		return string(output)
	}
	for _, volume := range []string{fixture, session} {
		if prior := strings.TrimSpace(run(nil, "volume", "ls", "--filter", "name=^"+volume+"$", "--format", "{{.Name}}")); prior != "" {
			t.Fatal("session fixture collision")
		}
		run(nil, "volume", "create", "--label", "anxi.task=junimo140-20261015", "--label", "com.anxi-panel.session-test="+identity, volume)
		t.Cleanup(func() {
			raw, err := exec.Command("docker", "volume", "inspect", volume).Output()
			var records []struct {
				Name   string
				Labels map[string]string
			}
			if err != nil || json.Unmarshal(raw, &records) != nil || len(records) != 1 || records[0].Name != volume || records[0].Labels["com.anxi-panel.session-test"] != identity {
				t.Error("session fixture cleanup ownership unknown")
				return
			}
			if output, err := exec.Command("docker", "volume", "rm", volume).CombinedOutput(); err != nil {
				t.Errorf("session fixture volume cleanup failed: %v: %s", err, output)
			}
		})
	}
	var records []struct{ Mountpoint string }
	if json.Unmarshal([]byte(run(nil, "volume", "inspect", fixture)), &records) != nil || len(records) != 1 {
		t.Fatal("session fixture daemon path unavailable")
	}
	backup := records[0].Mountpoint + "/world-a/.local-container/junimo140-group/apply_" + identity
	helper := func(script string) string {
		return run(nil, "run", "--rm", "--pull", "never", "--network", "none", "--read-only", "--mount", "type=volume,src="+fixture+",dst=/fixture", "--mount", "type=volume,src="+session+",dst=/session", "--entrypoint", "sh", image.ID, "-eu", "-c", script)
	}
	helper("mkdir -p /fixture/world-a/.local-container/junimo140-group/apply_" + identity)
	var payload bytes.Buffer
	archive := tar.NewWriter(&payload)
	content := []byte("disposable-session\n")
	if err := archive.WriteHeader(&tar.Header{Name: "token.fixture", Mode: 0o640, Uid: 1000, Gid: 1000, Size: int64(len(content)), Format: tar.FormatPAX, PAXRecords: map[string]string{"SCHILY.acl.access": "user::rw-,user:12345:r--,group::---,mask::r--,other::---"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := archive.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	run(payload.Bytes(), "run", "--rm", "-i", "--pull", "never", "--network", "none", "--read-only", "--mount", "type=volume,src="+session+",dst=/session", "--entrypoint", "tar", image.ID, "--acls", "--same-owner", "--same-permissions", "-xpf", "-", "-C", "/session")
	helper("ln -s token.fixture /session/token-link")
	request := RuntimeInstanceArchiveRequest{Project: "world-a", InstanceHostDir: records[0].Mountpoint + "/world-a", RecoveryHostDir: backup, Filename: "012345abcdef.tar", ImageRef: ref, ImageID: image.ID}
	checksum, err := client.RuntimeArchiveJunimo140Session(ctx, dir, request, session)
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedSHA256 = checksum
	if _, err := client.RuntimeArchiveJunimo140Session(ctx, dir, request, session); err == nil {
		t.Fatal("published session backup overwritten")
	}
	helper("printf mutated > /session/token.fixture; chmod 777 /session/token.fixture; chown 0:0 /session/token.fixture; printf extra > /session/extra")
	wrong := request
	wrong.ExpectedSHA256 = strings.Repeat("0", 64)
	if err := client.RuntimeRestoreJunimo140Session(ctx, dir, wrong, session); err == nil {
		t.Fatal("session restore accepted wrong archive checksum")
	}
	if got := helper("cat /session/token.fixture; test -f /session/extra"); got != "mutated" {
		t.Fatal("rejected session restore purged data")
	}
	run(nil, "run", "-d", "--rm", "--name", writer, "--label", "anxi.task=junimo140-20261015", "--label", "com.anxi-panel.session-test="+identity, "--label", "com.docker.compose.project=world-a", "--label", "com.docker.compose.service=steam-auth", "--mount", "type=volume,src="+session+",dst=/session", "--entrypoint", "sleep", image.ID, "300")
	t.Cleanup(func() {
		raw, err := exec.Command("docker", "ps", "-q", "--filter", "name=^/"+writer+"$", "--filter", "label=com.anxi-panel.session-test="+identity).Output()
		if err != nil {
			t.Error("session writer cleanup list failed")
			return
		}
		if strings.TrimSpace(string(raw)) != "" {
			if output, err := exec.Command("docker", "stop", "--time", "2", writer).CombinedOutput(); err != nil {
				t.Errorf("session writer cleanup failed: %v: %s", err, output)
			}
		}
	})
	if err := client.RuntimeRestoreJunimo140Session(ctx, dir, request, session); err == nil {
		t.Fatal("session restore accepted live auth holder")
	}
	run(nil, "stop", "--time", "2", writer)
	for attempt := 0; attempt < 2; attempt++ {
		if err := client.RuntimeRestoreJunimo140Session(ctx, dir, request, session); err != nil {
			t.Fatal(err)
		}
		if got := helper("test ! -e /session/extra; test -L /session/token-link; stat -c '%u:%g:%a' /session/token.fixture; setpriv --reuid=12345 --regid=12345 --clear-groups cat /session/token.fixture"); got != "1000:1000:640\ndisposable-session\n" {
			t.Fatalf("session owner, mode, ACL or content restore mismatch: %q", got)
		}
	}
}
