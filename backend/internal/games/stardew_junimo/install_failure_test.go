package stardew_junimo

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func TestInstallFailurePersistsActionableCause(t *testing.T) {
	for _, tc := range []struct {
		name, line, want string
		code             int
		commandError     error
	}{
		{"password", "ERROR (Invalid Password)", "账号或密码错误", 5, nil},
		{"guard", "That Steam Guard code was invalid.", "验证码无效或已过期", 5, nil},
		{"license", "No subscription", "下载许可", 5, nil},
		{"license-sdk", "No subscription\nSuccess! App '1007' fully installed", "下载许可", 5, nil},
		{"space", "no space left on device", "存储空间", 8, nil},
		{"dns", "", "无法解析", 0, errors.New("lookup fixture.invalid: no such host token=fixture-secret")},
		{"code5", "", "无法确定原因", 5, nil},
		{"killed", "", "可能是内存不足或被外部停止", 137, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			dir := t.TempDir()
			store, err := storage.Open(ctx, config.Config{DataDir: dir, DBPath: filepath.Join(dir, "panel.db")})
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			if err := store.Migrate(ctx); err != nil {
				t.Fatal(err)
			}
			instance, err := store.EnsureDefaultInstance(ctx, storage.EnsureDefaultInstanceParams{ID: storage.DefaultInstanceID, DriverID: storage.DefaultDriverID, Name: "QA", DataDir: filepath.Join(dir, "instance")})
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(tc.line, "\n")
			// The final summary must survive well beyond the UI's 1000-line tail.
			for i := 0; i < 1100; i++ {
				lines = append(lines, "Waiting for client shutdown...")
			}
			fake := &fakeDocker{containerCode: tc.code, containerLines: lines, containerErr: tc.commandError}
			manager := jobs.NewManager(store, slog.Default())
			driver := New(fake, slog.Default(), manager, store)
			job, err := driver.Install(ctx, registry.InstallRequest{Instance: registry.Instance{ID: instance.ID}, SteamUsername: "qa-user", SteamPassword: "qa-password", VNCPassword: "qa-vnc", AutoDownload: true})
			if err != nil {
				t.Fatal(err)
			}
			waitForDriverTestJobStatus(t, store, job.ID, storage.JobStatusFailed)
			finished, err := store.GetJob(ctx, job.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(finished.ErrorMessage.String, tc.want) {
				t.Fatalf("summary: %s", finished.ErrorMessage.String)
			}
			state, err := store.GetInstance(ctx, instance.ID)
			if err != nil {
				t.Fatal(err)
			}
			if state.StateMessage.String != finished.ErrorMessage.String {
				t.Fatal("state and job cause disagree")
			}
			logs, hasEarlier, err := store.ListLatestJobLogs(ctx, job.ID, 1000)
			if err != nil {
				t.Fatal(err)
			}
			if !hasEarlier {
				t.Fatal("fixture must exercise log tail truncation")
			}
			rawDiagnostic := false
			for _, line := range logs {
				if strings.Contains(line.Message, "[install:diagnostic]") {
					rawDiagnostic = true
				}
				if strings.Contains(line.Message, "fixture-secret") || strings.Contains(line.Message, "qa-password") {
					t.Fatal("credential leaked in logs")
				}
			}
			if !rawDiagnostic {
				t.Fatal("missing original diagnostic")
			}
			if fake.steamAuthRuns != 0 {
				t.Fatal("diagnosis changed installer side effects")
			}
			for _, volume := range fake.removedVolumes {
				if strings.Contains(volume, "game-data") {
					t.Fatal("game data must be preserved")
				}
			}
		})
	}
}
