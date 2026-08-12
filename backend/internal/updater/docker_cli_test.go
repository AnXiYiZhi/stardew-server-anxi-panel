package updater

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResolveComposeDeploymentValidatesPersistentPanelImageContract(t *testing.T) {
	installDir := t.TempDir()
	composeFile := filepath.Join(installDir, "docker-compose.yml")
	dataSource := filepath.Join(installDir, "data")
	container := ContainerInfo{
		ID: "1234567890abcdef", Image: "ghcr.io/anxiyizhi/stardew-server-anxi-panel:0.4.11", ImageID: "sha256:current",
	}
	configJSON, err := json.Marshal(map[string]any{
		"services": map[string]any{
			"panel": map[string]any{
				"image":   container.Image,
				"volumes": []map[string]string{{"type": "bind", "source": dataSource, "target": "/data"}},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name          string
		probeImage    string
		probeErr      error
		wantCode      string
		wantSucceeded bool
	}{
		{name: "managed image", probeImage: panelImageContractProbe, wantSucceeded: true},
		{name: "missing deployment env", probeErr: errors.New("missing env"), wantCode: CodeDeploymentEnvInvalid},
		{name: "hard-coded image", probeImage: container.Image, wantCode: CodeComposeImageUnmanaged},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var probeCalled bool
			cli := &DockerCLI{commandForTest: func(_ context.Context, _ time.Duration, args ...string) ([]byte, error) {
				joined := strings.Join(args, "\x00")
				if strings.Contains(joined, "PANEL_IMAGE="+panelImageContractProbe) {
					probeCalled = true
					if !strings.Contains(joined, "\x00--env-file\x00"+filepath.Join(installDir, ".env")+"\x00") {
						t.Fatalf("contract probe did not require the deployment env file: %#v", args)
					}
					return []byte(tt.probeImage + "\n"), tt.probeErr
				}
				if strings.Contains(joined, "\x00config\x00--format\x00json") {
					return configJSON, nil
				}
				if strings.Contains(joined, "\x00ps\x00-q\x00panel") {
					return []byte(container.ID + "\n"), nil
				}
				return nil, errors.New("unexpected docker command")
			}}

			service, resolveErr := cli.ResolveComposeDeployment(context.Background(), "anxi-panel", composeFile, container, "/data", dataSource)
			if service != "panel" || !probeCalled {
				t.Fatalf("service=%q probeCalled=%t err=%v", service, probeCalled, resolveErr)
			}
			if tt.wantSucceeded {
				if resolveErr != nil {
					t.Fatal(resolveErr)
				}
				return
			}
			var contractErr composeUpdateContractError
			if !errors.As(resolveErr, &contractErr) || contractErr.code != tt.wantCode {
				t.Fatalf("contract error=%+v raw=%v", contractErr, resolveErr)
			}
		})
	}
}
