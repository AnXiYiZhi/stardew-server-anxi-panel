package docker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestComposeCommandsUseFixedArguments(t *testing.T) {
	workDir := t.TempDir()
	dockerPath := fakeDocker(t, `
case "$1 $2 $3 $4" in
  "version   ") printf 'Docker version ok' ;;
  "compose version  ") printf 'Docker Compose version ok' ;;
		"compose ps --format json") printf '[{"Name":"demo","Service":"app","State":"running","Health":"healthy","ExitCode":0}]' ;;
		"compose stats --no-stream --format json") printf '{"Container":"demo-server-1","Name":"demo-server-1","Service":"server","CPUPerc":"2.50%%","MemUsage":"128MiB / 2GiB","MemPerc":"6.25%%"}' ;;
  "compose pull  ") printf 'pull ok' ;;
  "compose up -d ") printf 'up ok' ;;
  "compose down  ") printf 'down ok' ;;
  "compose restart  ") printf 'restart ok' ;;
  "compose restart server ") printf 'restart server ok' ;;
  "compose logs --no-color --tail") printf 'logs ok tail=%s service=%s' "$5" "$6" ;;
  *) printf 'unexpected args: %s %s %s %s %s %s' "$1" "$2" "$3" "$4" "$5" "$6" >&2; exit 7 ;;
esac
`)
	client := NewClient(Options{DockerPath: dockerPath})

	result, err := client.DockerVersion(context.Background(), workDir)
	if err != nil || result.ExitCode != 0 || !strings.Contains(result.Stdout, "Docker version ok") {
		t.Fatalf("DockerVersion result=%+v err=%v", result, err)
	}

	result, err = client.ComposeVersion(context.Background(), workDir)
	if err != nil || result.ExitCode != 0 || !strings.Contains(result.Stdout, "Docker Compose version ok") {
		t.Fatalf("ComposeVersion result=%+v err=%v", result, err)
	}

	ps, err := client.ComposePs(context.Background(), workDir)
	if err != nil || len(ps.Services) != 1 || ps.Services[0].Service != "app" {
		t.Fatalf("ComposePs result=%+v err=%v", ps, err)
	}

	if result, err = client.ComposePull(context.Background(), workDir); err != nil || result.ExitCode != 0 {
		t.Fatalf("ComposePull result=%+v err=%v", result, err)
	}
	stats, err := client.ComposeStats(context.Background(), workDir)
	if err != nil || len(stats.Services) != 1 || stats.Services[0].Service != "server" {
		t.Fatalf("ComposeStats result=%+v err=%v", stats, err)
	}
	if stats.Services[0].CPUPerc != 2.5 || stats.Services[0].MemUsedBytes != 128*1024*1024 {
		t.Fatalf("parsed stats = %+v", stats.Services[0])
	}
	if result, err = client.ComposeUp(context.Background(), workDir); err != nil || result.ExitCode != 0 {
		t.Fatalf("ComposeUp result=%+v err=%v", result, err)
	}
	if result, err = client.ComposeDown(context.Background(), workDir); err != nil || result.ExitCode != 0 {
		t.Fatalf("ComposeDown result=%+v err=%v", result, err)
	}
	if result, err = client.ComposeRestart(context.Background(), workDir); err != nil || result.ExitCode != 0 {
		t.Fatalf("ComposeRestart result=%+v err=%v", result, err)
	}
	if result, err = client.ComposeRestartServices(context.Background(), workDir, "server"); err != nil || result.ExitCode != 0 || !strings.Contains(result.Stdout, "restart server ok") {
		t.Fatalf("ComposeRestartServices result=%+v err=%v", result, err)
	}
	if result, err = client.ComposeLogs(context.Background(), workDir, LogsOptions{Service: "app", Tail: 25}); err != nil || !strings.Contains(result.Stdout, "tail=25 service=app") {
		t.Fatalf("ComposeLogs result=%+v err=%v", result, err)
	}
}

func TestParseComposeJSON_Formats(t *testing.T) {
	jsonArray := `[{"Service":"server","State":"running","Status":"Up 2 minutes"},{"Service":"steam-auth","State":"running","Status":"Up 3 minutes"}]`
	jsonSingle := `{"Service":"server","State":"running","Status":"Up 2 minutes"}`
	jsonl := "{\"Service\":\"server\",\"State\":\"running\",\"Status\":\"Up 2 minutes\"}\n{\"Service\":\"steam-auth\",\"State\":\"running\",\"Status\":\"Up 3 minutes\"}"

	cases := []struct {
		name    string
		input   string
		wantLen int
		wantSvc string
	}{
		{"json array", jsonArray, 2, "server"},
		{"json single", jsonSingle, 1, "server"},
		{"jsonl", jsonl, 2, "server"},
		{"empty", "", 0, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			items, err := parseComposeJSON(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(items) != tc.wantLen {
				t.Fatalf("got %d items, want %d", len(items), tc.wantLen)
			}
			if tc.wantSvc != "" {
				svc := firstString(items[0], "Service", "service")
				if svc != tc.wantSvc {
					t.Fatalf("first service = %q, want %q", svc, tc.wantSvc)
				}
			}
		})
	}
}

func TestComposeLogsValidatesInput(t *testing.T) {
	client := NewClient(Options{DockerPath: "docker"})
	if _, err := client.ComposeLogs(context.Background(), t.TempDir(), LogsOptions{Service: "bad/service", Tail: 100}); err != ErrInvalidService {
		t.Fatalf("expected ErrInvalidService, got %v", err)
	}
	if _, err := client.ComposeLogs(context.Background(), t.TempDir(), LogsOptions{Tail: MaxLogTail + 1}); err != ErrInvalidTail {
		t.Fatalf("expected ErrInvalidTail, got %v", err)
	}
}

func TestComposePsUsesShortTTLCacheAndInvalidatesAfterStateChange(t *testing.T) {
	workDir := t.TempDir()
	countPath := filepath.Join(t.TempDir(), "ps-count.txt")
	dockerPath := fakeDockerCountingPs(t, countPath)
	client := NewClient(Options{DockerPath: dockerPath, ComposePsTTL: 50 * time.Millisecond})

	for i := 0; i < 2; i++ {
		ps, err := client.ComposePs(context.Background(), workDir)
		if err != nil || len(ps.Services) != 1 || ps.Services[0].Service != "server" {
			t.Fatalf("ComposePs #%d result=%+v err=%v", i+1, ps, err)
		}
	}
	if got := readCountFile(t, countPath); got != 1 {
		t.Fatalf("ComposePs command count after cache hit = %d, want 1", got)
	}

	if result, err := client.ComposeUp(context.Background(), workDir); err != nil || result.ExitCode != 0 {
		t.Fatalf("ComposeUp result=%+v err=%v", result, err)
	}
	if _, err := client.ComposePs(context.Background(), workDir); err != nil {
		t.Fatalf("ComposePs after ComposeUp err=%v", err)
	}
	if got := readCountFile(t, countPath); got != 2 {
		t.Fatalf("ComposePs command count after invalidation = %d, want 2", got)
	}

	time.Sleep(60 * time.Millisecond)
	if _, err := client.ComposePs(context.Background(), workDir); err != nil {
		t.Fatalf("ComposePs after TTL expiry err=%v", err)
	}
	if got := readCountFile(t, countPath); got != 3 {
		t.Fatalf("ComposePs command count after TTL expiry = %d, want 3", got)
	}
}

func readCountFile(t *testing.T, path string) int {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read count file: %v", err)
	}
	count := 0
	for _, line := range strings.Split(strings.TrimSpace(string(content)), "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

func TestRunCapturesFailure(t *testing.T) {
	workDir := t.TempDir()
	dockerPath := fakeDocker(t, `printf 'password=secret' >&2; exit 9`)
	client := NewClient(Options{DockerPath: dockerPath})

	result, err := client.DockerVersion(context.Background(), workDir)
	if err == nil {
		t.Fatal("expected command error")
	}
	if result.ExitCode != 9 {
		t.Fatalf("expected exit code 9, got %d", result.ExitCode)
	}
	if strings.Contains(result.Stderr, "secret") || !strings.Contains(result.Stderr, Redacted) {
		t.Fatalf("expected stderr to be redacted, got %q", result.Stderr)
	}
}

func TestIsMissingVolumeRemove(t *testing.T) {
	result := CommandResult{Stderr: "Error response from daemon: get demo: no such volume"}
	if !isMissingVolumeRemove(result, ErrCommandFailed) {
		t.Fatal("expected missing Docker volume error to be ignored")
	}
	result = CommandResult{Stderr: "Error response from daemon: remove demo: volume is in use"}
	if isMissingVolumeRemove(result, ErrCommandFailed) {
		t.Fatal("expected in-use Docker volume error to remain fatal")
	}
}

func fakeDockerCountingPs(t *testing.T, countPath string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		path := filepath.Join(t.TempDir(), "docker.cmd")
		content := fmt.Sprintf("@echo off\r\n"+
			"if \"%%1 %%2 %%3 %%4\"==\"compose ps --format json\" (echo ps>>\"%s\"& echo [{\"Name\":\"demo-server-1\",\"Service\":\"server\",\"State\":\"running\",\"Health\":\"healthy\",\"ExitCode\":0}]& exit /b 0)\r\n"+
			"if \"%%1 %%2 %%3 %%4\"==\"compose up -d \" (echo up ok& exit /b 0)\r\n"+
			"echo unexpected args: %%1 %%2 %%3 %%4 %%5 %%6 1>&2\r\nexit /b 7\r\n",
			countPath,
		)
		if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
			t.Fatalf("write fake docker: %v", err)
		}
		return path
	}

	path := filepath.Join(t.TempDir(), "docker")
	content := fmt.Sprintf(`#!/bin/sh
case "$1 $2 $3 $4" in
  "compose ps --format json") printf 'ps\n' >> %q; printf '[{"Name":"demo-server-1","Service":"server","State":"running","Health":"healthy","ExitCode":0}]' ;;
  "compose up -d ") printf 'up ok' ;;
  *) printf 'unexpected args: %%s %%s %%s %%s %%s %%s' "$1" "$2" "$3" "$4" "$5" "$6" >&2; exit 7 ;;
esac
`, countPath)
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write fake docker: %v", err)
	}
	return path
}

func fakeDocker(t *testing.T, script string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		path := filepath.Join(t.TempDir(), "docker.cmd")
		content := "@echo off\r\n" +
			"if \"%1 %2 %3 %4\"==\"version   \" (echo Docker version ok& exit /b 0)\r\n" +
			"if \"%1 %2 %3 %4\"==\"compose version  \" (echo Docker Compose version ok& exit /b 0)\r\n" +
			"if \"%1 %2 %3 %4\"==\"compose ps --format json\" (echo [{\"Name\":\"demo\",\"Service\":\"app\",\"State\":\"running\",\"Health\":\"healthy\",\"ExitCode\":0}]& exit /b 0)\r\n" +
			"if \"%1 %2 %3 %4 %5\"==\"compose stats --no-stream --format json\" (echo {\"Container\":\"demo-server-1\",\"Name\":\"demo-server-1\",\"Service\":\"server\",\"CPUPerc\":\"2.50%%\",\"MemUsage\":\"128MiB / 2GiB\",\"MemPerc\":\"6.25%%\"}& exit /b 0)\r\n" +
			"if \"%1 %2 %3 %4\"==\"compose pull  \" (echo pull ok& exit /b 0)\r\n" +
			"if \"%1 %2 %3 %4\"==\"compose up -d \" (echo up ok& exit /b 0)\r\n" +
			"if \"%1 %2 %3 %4\"==\"compose down  \" (echo down ok& exit /b 0)\r\n" +
			"if \"%1 %2 %3 %4\"==\"compose restart  \" (echo restart ok& exit /b 0)\r\n" +
			"if \"%1 %2 %3 %4\"==\"compose restart server \" (echo restart server ok& exit /b 0)\r\n" +
			"if \"%1 %2 %3 %4\"==\"compose logs --no-color --tail\" (echo logs ok tail=%5 service=%6& exit /b 0)\r\n" +
			"echo password=secret 1>&2\r\nexit /b 9\r\n"
		if strings.Contains(script, "exit 9") {
			content = "@echo off\r\necho password=secret 1>&2\r\nexit /b 9\r\n"
		}
		if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
			t.Fatalf("write fake docker: %v", err)
		}
		return path
	}

	path := filepath.Join(t.TempDir(), "docker")
	content := "#!/bin/sh\n" + script + "\n"
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write fake docker: %v", err)
	}
	return path
}
