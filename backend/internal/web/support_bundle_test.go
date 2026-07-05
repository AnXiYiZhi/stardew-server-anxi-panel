package web

import (
	"archive/zip"
	"bytes"
	"net/http"
	"testing"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
)

func TestSupportBundleStreamsValidZip(t *testing.T) {
	fake := fakeDockerService{
		versionResult: paneldocker.CommandResult{ExitCode: 0, Stdout: "ok"},
		psResult: paneldocker.ComposePsResult{
			Result:   paneldocker.CommandResult{ExitCode: 0, Stdout: "[]"},
			Services: []paneldocker.ComposeService{{Name: "demo-server-1", Service: "server", State: "running"}},
		},
		logsResult: paneldocker.CommandResult{ExitCode: 0, Stdout: "server log tail"},
	}
	handler, _, closeStore := newDockerTestHandler(t, fake)
	defer closeStore()
	adminCookie := setupDockerAdmin(t, handler)

	response, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/support-bundle", nil, adminCookie)
	if response.Code != http.StatusOK {
		t.Fatalf("support bundle returned %d: %s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Content-Type"); got != "application/zip" {
		t.Fatalf("content type = %q, want application/zip", got)
	}
	if got := response.Header().Get("Content-Length"); got != "" {
		t.Fatalf("streamed support bundle should not set Content-Length, got %q", got)
	}

	reader, err := zip.NewReader(bytes.NewReader(response.Body.Bytes()), int64(response.Body.Len()))
	if err != nil {
		t.Fatalf("support bundle is not a valid zip: %v", err)
	}
	names := make(map[string]bool, len(reader.File))
	for _, file := range reader.File {
		names[file.Name] = true
	}
	for _, name := range []string{"version.json", "health.json", "instance-state.json", "jobs.json", "audit-logs.json", "compose-ps.json", "server-logs.txt"} {
		if !names[name] {
			t.Fatalf("support bundle missing %s; entries=%v", name, names)
		}
	}
}
