//go:build integration

package docker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResourceVolumeScopedScanAndCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	name := fmt.Sprintf("anxi-resource-scope-%d", time.Now().UnixNano())
	owned, unrelated, absent := name+"-owned", name+"-unrelated", name+"-absent"
	image := name + ":slow"
	cli := func(args ...string) string {
		t.Helper()
		out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
		if err != nil {
			t.Fatalf("fixture docker command failed: %v %s", err, out)
		}
		return strings.TrimSpace(string(out))
	}
	for _, volume := range []string{owned, unrelated} {
		cli("volume", "create", "--label", "anxi.resource-test="+name, volume)
		t.Cleanup(func() {
			cleanup, stop := context.WithTimeout(context.Background(), 15*time.Second)
			defer stop()
			if out, err := exec.CommandContext(cleanup, "docker", "volume", "rm", volume).CombinedOutput(); err != nil {
				t.Errorf("cleanup volume: %v %s", err, out)
			}
		})
	}
	cli("run", "--rm", "--pull", "never", "--network", "none", "--mount", "type=volume,src="+owned+",dst=/owned", "--mount", "type=volume,src="+unrelated+",dst=/unrelated", "alpine:3.20", "sh", "-c", "printf '1234567' > /owned/data; printf 'unrelated-content' > /unrelated/data; ln -s /unrelated/data /owned/link")
	client := NewClient(Options{})
	dir := t.TempDir()
	sizes, err := client.ResourceVolumeSizes(ctx, dir, "alpine:3.20", []string{owned, owned, absent})
	if err != nil || len(sizes) != 2 || sizes[owned] != 7 || sizes[absent] != 0 {
		t.Fatalf("scoped sizes=%v err=%v", sizes, err)
	}
	if runtimeImage := os.Getenv("ANXI_RESOURCE_PROBE_IMAGE"); runtimeImage != "" {
		sizes, err = client.ResourceVolumeSizes(ctx, dir, runtimeImage, []string{owned, absent})
		if err != nil || sizes[owned] != 7 || sizes[absent] != 0 {
			t.Fatalf("runtime image probe failed: %v %v", sizes, err)
		}
	}
	if names := cli("volume", "ls", "--format", "{{.Name}}"); strings.Contains(names, absent) {
		t.Fatal("read created absent volume")
	}
	if remaining := cli("ps", "-a", "--filter", "label=io.anxi-panel.resource-scan=true", "--filter", "ancestor=alpine:3.20", "--format", "{{.Names}}"); remaining != "" {
		t.Fatal("successful scan left a helper")
	}
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM alpine:3.20\nCOPY slow-stat /usr/local/bin/stat\nRUN chmod 755 /usr/local/bin/stat\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "slow-stat"), []byte("#!/bin/sh\nsleep 60\nexec /bin/stat \"$@\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cli("build", "--pull=false", "--label", "anxi.resource-test="+name, "-t", image, dir)
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 15*time.Second)
		defer stop()
		if out, err := exec.CommandContext(cleanup, "docker", "image", "rm", image).CombinedOutput(); err != nil {
			t.Errorf("cleanup image: %v %s", err, out)
		}
	})
	scanCtx, stopScan := context.WithCancel(ctx)
	defer stopScan()
	finished := make(chan error, 1)
	go func() { _, err := client.ResourceVolumeSizes(scanCtx, dir, image, []string{owned}); finished <- err }()
	started := false
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if cli("ps", "--filter", "label=io.anxi-panel.resource-scan=true", "--filter", "ancestor="+image, "--format", "{{.Names}}") != "" {
			started = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	stopScan()
	select {
	case err := <-finished:
		if !started || err == nil {
			t.Fatalf("cancellation did not interrupt active scan: started=%t err=%v", started, err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("canceled scanner did not return")
	}
	if remaining := cli("ps", "-a", "--filter", "label=io.anxi-panel.resource-scan=true", "--filter", "ancestor="+image, "--format", "{{.Names}}"); remaining != "" {
		t.Fatal("cancellation left a helper")
	}
}
