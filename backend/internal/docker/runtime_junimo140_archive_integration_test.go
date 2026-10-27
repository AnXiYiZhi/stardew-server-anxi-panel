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

// All bind sources come from an explicitly created Docker volume's daemon
// mountpoint. No container-private t.TempDir is passed to the host daemon.
func TestJunimo140ArchiveRestoresOwnershipACLAndPreservesLeaseRealOptIn(t *testing.T) {
	if os.Getenv("ANXI_REAL_JUNIMO140_ARCHIVE") != "1" {
		t.Skip("set ANXI_REAL_JUNIMO140_ARCHIVE=1")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	client := NewClient(Options{DockerPath: "docker"})
	image, err := client.RuntimeImageInspect(ctx, t.TempDir(), "sdvd/server:1.5.0-preview.140")
	if err != nil || image.Digest != "sha256:f23fdc3a3e999032be363fa43291c2d72626555913ef2b3dcf71b0aec53c49db" {
		t.Fatalf("fixed image unavailable: %v", err)
	}
	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	volume := "junimo140-archive-test-" + hex.EncodeToString(nonce[:])
	run := func(input []byte, args ...string) string {
		t.Helper()
		command := exec.CommandContext(ctx, "docker", args...)
		command.Stdin = bytes.NewReader(input)
		out, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("isolated archive fixture command failed: %v: %s", err, out)
		}
		return string(out)
	}
	if existing := run(nil, "volume", "ls", "--filter", "name=^"+volume+"$", "--format", "{{.Name}}"); strings.TrimSpace(existing) != "" {
		t.Fatal("fixture volume collision")
	}
	run(nil, "volume", "create", "--label", "anxi.task=junimo140-20261015", "--label", "com.anxi-panel.junimo140-archive-test=true", volume)
	t.Cleanup(func() {
		out, err := exec.Command("docker", "volume", "inspect", volume).Output()
		var volumes []struct {
			Name   string
			Labels map[string]string
		}
		if err != nil || json.Unmarshal(out, &volumes) != nil || len(volumes) != 1 || volumes[0].Name != volume || volumes[0].Labels["com.anxi-panel.junimo140-archive-test"] != "true" {
			t.Error("archive fixture ownership check failed; retained")
			return
		}
		if out, err := exec.Command("docker", "volume", "rm", volume).CombinedOutput(); err != nil {
			t.Errorf("archive fixture cleanup failed: %v: %s", err, out)
		}
	})
	var volumes []struct{ Mountpoint string }
	if json.Unmarshal([]byte(run(nil, "volume", "inspect", volume)), &volumes) != nil || len(volumes) != 1 || !strings.HasPrefix(volumes[0].Mountpoint, "/") {
		t.Fatal("daemon volume path unavailable")
	}
	base := volumes[0].Mountpoint + "/world-a"
	apply := "apply_" + strings.Repeat("a", 24)
	backup := base + "/.local-container/junimo140-group/" + apply
	helper := func(script string) string {
		return run(nil, "run", "--rm", "--pull", "never", "--network", "none", "--read-only", "--mount", "type=volume,src="+volume+",dst=/fixture", "--entrypoint", "sh", image.ID, "-eu", "-c", script)
	}
	helper("mkdir -p /fixture/world-a/.local-container/junimo140-group/" + apply + "; printf old-lease > /fixture/world-a/.local-container/junimo140-lease.json; printf old-journal > /fixture/world-a/.local-container/junimo140-group/" + apply + "/manifest.json; mkfifo /fixture/world-a/fifo")
	helper("mkdir /fixture/world-a/.local-container/control; printf private-options > /fixture/world-a/.local-container/control/options.json; chmod 600 /fixture/world-a/.local-container/control/options.json; printf credential-fixture > /fixture/world-a/.env; chmod 600 /fixture/world-a/.env")
	var payload bytes.Buffer
	w := tar.NewWriter(&payload)
	content := []byte("original payload\n")
	if err := w.WriteHeader(&tar.Header{Name: "payload", Mode: 0o640, Uid: 1234, Gid: 124, Size: int64(len(content)), Format: tar.FormatPAX,
		PAXRecords: map[string]string{"SCHILY.acl.access": "user::rw-,user:12345:r--,group::---,mask::r--,other::---"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	run(payload.Bytes(), "run", "--rm", "-i", "--pull", "never", "--network", "none", "--read-only", "--mount", "type=volume,src="+volume+",dst=/fixture", "--entrypoint", "tar", image.ID, "--acls", "--same-owner", "--same-permissions", "-xpf", "-", "-C", "/fixture/world-a")
	// The named user can read only because of the extended ACL, not owner,
	// group membership or world permissions.
	if got := helper("setpriv --reuid=12345 --regid=12345 --clear-groups cat /fixture/world-a/payload"); got != string(content) {
		t.Fatal("ACL fixture was not established")
	}
	request := RuntimeInstanceArchiveRequest{Project: "world-a", InstanceHostDir: base, RecoveryHostDir: backup, Filename: strings.Repeat("b", 12) + ".tar", ImageRef: "sdvd/server:1.5.0-preview.140", ImageID: image.ID}
	checksum, err := client.RuntimeArchiveJunimo140Instance(ctx, t.TempDir(), request)
	if err != nil {
		t.Fatal(err)
	}
	// Never replace an already published backup, even with the same filename.
	if _, err := client.RuntimeArchiveJunimo140Instance(ctx, t.TempDir(), request); err == nil {
		t.Fatal("immutable archive was overwritten")
	}
	request.ExpectedSHA256 = checksum
	if err := client.RuntimePrepareJunimo140Permissions(ctx, t.TempDir(), request, "1000", "1000"); err != nil {
		t.Fatal(err)
	}
	if got := helper("stat -c '%u:%g:%a' /fixture/world-a/.env; stat -c '%u:%g:%a' /fixture/world-a/.local-container/control/options.json; setpriv --reuid=1000 --regid=1000 --clear-groups cat /fixture/world-a/.local-container/control/options.json"); got != "0:0:600\n1000:1000:600\nprivate-options" {
		t.Fatalf("runtime ownership changed private instance credentials: %q", got)
	}
	helper("printf mutated > /fixture/world-a/payload; chown 1000:1000 /fixture/world-a/payload; chmod 777 /fixture/world-a/payload; printf new > /fixture/world-a/new-file; printf new-lease > /fixture/world-a/.local-container/junimo140-lease.json; printf new-journal > /fixture/world-a/.local-container/junimo140-group/" + apply + "/manifest.json")
	request.ExpectedSHA256 = strings.Repeat("0", 64)
	if client.RuntimeRestoreJunimo140Instance(ctx, t.TempDir(), request) == nil {
		t.Fatal("wrong checksum accepted")
	}
	if got := helper("cat /fixture/world-a/payload; test -f /fixture/world-a/new-file"); got != "mutated" {
		t.Fatal("checksum rejection mutated data")
	}
	request.ExpectedSHA256 = checksum
	for attempt := 0; attempt < 2; attempt++ {
		if err := client.RuntimeRestoreJunimo140Instance(ctx, t.TempDir(), request); err != nil {
			t.Fatal(err)
		}
		got := helper("test ! -e /fixture/world-a/new-file; test -p /fixture/world-a/fifo; stat -c '%u:%g:%a' /fixture/world-a/payload; setpriv --reuid=12345 --regid=12345 --clear-groups cat /fixture/world-a/payload; cat /fixture/world-a/.local-container/junimo140-lease.json; cat /fixture/world-a/.local-container/junimo140-group/" + apply + "/manifest.json")
		if got != "1234:124:640\noriginal payload\nnew-leasenew-journal" {
			t.Fatalf("restored identity, ACL, FIFO or protected lease differs: %q", got)
		}
		if got := helper("stat -c '%u:%g:%a' /fixture/world-a/.local-container/control/options.json; stat -c '%u:%g:%a' /fixture/world-a/.env"); got != "0:0:600\n0:0:600\n" {
			t.Fatalf("original private permissions were not restored: %q", got)
		}
	}
}
