//go:build integration

package stardew_junimo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
)

func (f *junimo140RealGroupEngine) RuntimeStopJunimo140Services(ctx context.Context, dir string, snapshots []paneldocker.RuntimeJunimo140ServiceSnapshot, allowed map[string][]string) error {
	for _, snapshot := range snapshots {
		if snapshot.Service == "server" && snapshot.ImageID == f.targetImageID {
			if err := f.captureTargetBeforeStop(snapshot); err != nil {
				f.diagnosticErrors.Add(1)
				_ = os.WriteFile(filepath.Join(f.root, "target-diagnostic-error.txt"), []byte(err.Error()), 0600)
			}
		}
	}
	return f.stopAndDelaySavePeer(ctx, dir, snapshots, allowed)
}

func (f *junimo140RealGroupEngine) captureTargetBeforeStop(snapshot paneldocker.RuntimeJunimo140ServiceSnapshot) error {
	identity := strings.TrimPrefix(filepath.Base(f.root), "group-real-")
	if len(identity) != 24 || snapshot.Project != "j140-real-"+identity+"-a" && snapshot.Project != "j140-real-"+identity+"-b" && snapshot.Project != "j140-real-"+identity+"-c" {
		return errors.New("target diagnostic scope mismatch")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	raw, err := exec.CommandContext(ctx, "docker", "container", "inspect", snapshot.ContainerID).Output()
	if err != nil {
		return err
	}
	var records []struct {
		ID, Image string
		Config    struct{ Labels map[string]string }
		State     struct {
			Status, Error, StartedAt, FinishedAt string
			Running, OOMKilled                   bool
			ExitCode                             int
			Health                               struct{ Status string }
		}
	}
	if json.Unmarshal(raw, &records) != nil || len(records) != 1 || records[0].ID != snapshot.ContainerID || records[0].Image != f.targetImageID || records[0].Config.Labels["com.docker.compose.project"] != snapshot.Project {
		return errors.New("target diagnostic container identity mismatch")
	}
	sequence := f.diagnosticSequence.Add(1)
	stem := fmt.Sprintf("target-%s-stop-%03d", strings.TrimPrefix(snapshot.Project, "j140-real-"+identity+"-"), sequence)
	metadata, err := json.MarshalIndent(map[string]any{"project": snapshot.Project, "image": records[0].Image, "state": records[0].State, "utc": time.Now().UTC()}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(f.root, stem+".json"), metadata, 0600); err != nil {
		return err
	}
	logs, err := exec.CommandContext(ctx, "docker", "logs", "--tail", "600", snapshot.ContainerID).CombinedOutput()
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(f.root, stem+".log"), logs, 0600)
}
