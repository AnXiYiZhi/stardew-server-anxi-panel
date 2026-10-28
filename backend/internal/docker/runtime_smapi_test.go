package docker

import (
	"context"
	"path/filepath"
	"testing"
)

func TestRuntimeSMAPIDockerContractRejectsUnscopedInputs(t *testing.T) {
	client := NewClient(Options{DockerPath: "docker-that-must-not-run"})
	ctx := context.Background()
	dir := t.TempDir()
	validID := "0123456789abcdef01234567"
	if err := client.RuntimeCreateSMAPIStagingVolume(ctx, dir, "safe_project", "other_anxi-smapi-update-"+validID); err == nil {
		t.Fatal("foreign staging volume accepted")
	}
	if err := client.RuntimeCloneGameData(ctx, dir, "safe_project_game-data;rm", "safe_project_anxi-smapi-update-"+validID, "sdvd/server:1.5.0-preview.121"); err == nil {
		t.Fatal("injected source volume accepted")
	}
	if err := client.RuntimeInstallSMAPIArchive(ctx, dir, "safe_project_anxi-smapi-update-"+validID, filepath.Join(dir, "SMAPI-4.5.2-installer.zip"), "sdvd/server:1.5.0-preview.121"); err == nil {
		t.Fatal("archive outside private package cache accepted")
	}
	if err := client.RuntimeRemoveSMAPIStagingVolume(ctx, dir, "safe_project", "safe_project_game-data"); err == nil {
		t.Fatal("production volume accepted for removal")
	}
	if err := client.RuntimeInstallImageSMAPI(ctx, dir, "safe_project_game-data", "sdvd/server:1.5.0-preview.140", "invalid"); err == nil || err.Error() != "invalid image-managed SMAPI install request" {
		t.Fatal("image installer did not reject unscoped input before invoking Docker")
	}
	if err := client.RuntimeCommitImageSMAPIStamp(ctx, dir, "safe_project_game-data", "sdvd/server:1.5.0-preview.140", "invalid", "../../bad"); err == nil || err.Error() != "invalid image-managed SMAPI stamp request" {
		t.Fatal("image stamp did not reject unscoped input before invoking Docker")
	}
	if _, err := client.RuntimeVolumeExists(ctx, dir, "game-data;bad"); err == nil || err.Error() != "invalid game-data volume name" {
		t.Fatal("volume presence query did not reject an unscoped input before invoking Docker")
	}
	if err := client.RuntimeValidateUnusedSMAPIStaging(ctx, dir, "safe_project_game-data"); err == nil || err.Error() != "invalid image-managed SMAPI staging volume" {
		t.Fatal("base installer accepted an ordinary game-data volume as owned staging")
	}
}
