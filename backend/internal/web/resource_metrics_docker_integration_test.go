//go:build integration

package web

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
)

func TestDockerResourceMetricsConcurrentRequestsShareSample(t *testing.T) {
	dir := t.TempDir()
	compose := "services:\n  server:\n    image: bash:5.2\n    command: [\"sleep\", \"600\"]\n"
	if err := os.WriteFile(filepath.Join(dir, "compose.yml"), []byte(compose), 0o600); err != nil {
		t.Fatal(err)
	}
	client := paneldocker.NewClient(paneldocker.Options{})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if result, err := client.ComposeUp(ctx, dir); err != nil || result.ExitCode != 0 {
		t.Fatalf("compose up: result=%+v err=%v", result, err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()
		_, _ = client.ComposeDown(cleanupCtx, dir)
	})

	s := &server{docker: client}
	responses := make([]resourceMetricsResponse, 12)
	var wg sync.WaitGroup
	for i := range responses {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			responses[index] = s.resourceMetrics("stardew", dir)
		}(i)
	}
	wg.Wait()
	first := responses[0]
	if !first.Sample.ContainerRunning || first.Sample.CPUPercent == nil || first.Sample.MemoryPercent == nil {
		t.Fatalf("real Docker stats missing: %+v", first)
	}
	for i, response := range responses[1:] {
		if response.Sample.Timestamp != first.Sample.Timestamp {
			t.Fatalf("response %d did not share cached sample: %q != %q", i+1, response.Sample.Timestamp, first.Sample.Timestamp)
		}
	}
}
