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
case "$1 $2 $3 $4 $5 $6" in
  "version "*) printf 'Docker version ok' ;;
  "compose version "*) printf 'Docker Compose version ok' ;;
		"compose ps --format json "*) printf '[{"Name":"demo","Service":"app","State":"running","Health":"healthy","ExitCode":0}]' ;;
  "compose stats --no-stream --format json"*) printf '{"Container":"demo-server-1","Name":"demo-server-1","Service":"server","CPUPerc":"2.50%%","MemUsage":"128MiB / 2GiB","MemPerc":"6.25%%"}' ;;
  "compose pull "*) printf 'pull ok' ;;
  "compose up -d --no-deps --force-recreate server") printf 'recreate server ok' ;;
  "compose up -d "*) printf 'up ok' ;;
  "compose down "*) printf 'down ok' ;;
  "compose restart server "*) printf 'restart server ok' ;;
  "compose restart "*) printf 'restart ok' ;;
  "compose logs --no-color --tail "*) printf 'logs ok tail=%s service=%s' "$5" "$6" ;;
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
	if result, err = client.ComposeRecreateServices(context.Background(), workDir, "server"); err != nil || result.ExitCode != 0 || !strings.Contains(result.Stdout, "recreate server ok") {
		t.Fatalf("ComposeRecreateServices result=%+v err=%v", result, err)
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

func TestComposeRecreateServicesRequiresValidatedExplicitTargets(t *testing.T) {
	client := NewClient(Options{DockerPath: "docker"})
	if _, err := client.ComposeRecreateServices(context.Background(), t.TempDir()); err == nil {
		t.Fatal("expected empty service list to fail")
	}
	if _, err := client.ComposeRecreateServices(context.Background(), t.TempDir(), "server;steam-auth"); err == nil {
		t.Fatal("expected invalid service name to fail")
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

func TestComposePsStrictBypassesCacheAcceptsEmptySetAndFailsClosedOnMalformed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("strict fake docker argument sequencing is covered on the Linux target filesystem")
	}
	workDir := t.TempDir()
	countPath := filepath.Join(t.TempDir(), "strict-ps-count.txt")
	dockerPath := fakeDocker(t, fmt.Sprintf(`
case "$1 $2 $3 $4 $5" in
  "compose ps --format json ") printf '[{"Service":"server","State":"exited","Status":"Exited (0)"}]' ;;
  "compose ps --all --format json")
    printf 'strict\n' >> %q
    case "$(wc -l < %q)" in
      1) printf '[{"Service":"server","State":"running","Status":"Up 1 second"}]' ;;
      2) printf '{bad json' ;;
      3) printf '' ;;
      4) printf '[{"Service":"server","Status":"Exited (0)"}]' ;;
      5) printf 'null' ;;
      *) printf '[{"Service":"server","State":"unknown"}]' ;;
    esac
    ;;
  *) printf 'unexpected args: %%s %%s %%s %%s %%s' "$1" "$2" "$3" "$4" "$5" >&2; exit 7 ;;
esac
`, countPath, countPath))
	client := NewClient(Options{DockerPath: dockerPath, ComposePsTTL: time.Minute})

	if _, err := client.ComposePs(context.Background(), workDir); err != nil {
		t.Fatalf("prime cached ComposePs: %v", err)
	}
	strict, err := client.ComposePsStrict(context.Background(), workDir)
	if err != nil || len(strict.Services) != 1 || strict.Services[0].State != "running" {
		t.Fatalf("strict result=%+v err=%v", strict, err)
	}
	if _, err := client.ComposePsStrict(context.Background(), workDir); err == nil || !strings.Contains(err.Error(), "parse docker compose ps --all") {
		t.Fatalf("malformed strict response error=%v", err)
	}
	empty, err := client.ComposePsStrict(context.Background(), workDir)
	if err != nil || len(empty.Services) != 0 {
		t.Fatalf("empty strict response=%+v err=%v", empty, err)
	}
	if _, err := client.ComposePsStrict(context.Background(), workDir); err == nil || !strings.Contains(err.Error(), "missing service or state") {
		t.Fatalf("incomplete strict response error=%v", err)
	}
	if _, err := client.ComposePsStrict(context.Background(), workDir); err == nil || !strings.Contains(err.Error(), "empty JSON value") {
		t.Fatalf("empty JSON strict response error=%v", err)
	}
	if _, err := client.ComposePsStrict(context.Background(), workDir); err == nil || !strings.Contains(err.Error(), "unknown state") {
		t.Fatalf("unknown state strict response error=%v", err)
	}
	if got := readCountFile(t, countPath); got != 6 {
		t.Fatalf("strict command count=%d, want 6 uncached calls", got)
	}
}

func TestComposePsStrictReadsFreshStateInBothCacheDirections(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("strict fake docker cache/parser coverage runs on the Linux target filesystem")
	}
	for _, tc := range []struct {
		name        string
		cachedState string
		freshState  string
	}{
		{name: "cached stopped fresh running", cachedState: "exited", freshState: "running"},
		{name: "cached running fresh stopped", cachedState: "running", freshState: "exited"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			workDir := t.TempDir()
			normal := fmt.Sprintf(`[{"Service":"server","State":%q,"Status":"cached"}]`, tc.cachedState)
			fresh := fmt.Sprintf(`[{"Service":"server","State":%q,"Status":"fresh"}]`, tc.freshState)
			client := NewClient(Options{
				DockerPath:   fakeDockerComposePsResponses(t, normal, fresh),
				ComposePsTTL: time.Minute,
			})

			cached, err := client.ComposePs(context.Background(), workDir)
			if err != nil || len(cached.Services) != 1 || cached.Services[0].State != tc.cachedState {
				t.Fatalf("prime cache result=%+v err=%v", cached, err)
			}
			strict, err := client.ComposePsStrict(context.Background(), workDir)
			if err != nil || len(strict.Services) != 1 || strict.Services[0].State != tc.freshState || strict.Services[0].Status != "fresh" {
				t.Fatalf("strict result=%+v err=%v", strict, err)
			}
		})
	}
}

func TestComposePsStrictRejectsTruncatedOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("strict fake docker limited-buffer coverage runs on the Linux target filesystem")
	}
	client := NewClient(Options{
		DockerPath:     fakeDockerComposePsResponses(t, `[]`, `[{"Service":"server","State":"exited","Status":"Exited (0)"}]`),
		MaxOutputBytes: 16,
	})
	if _, err := client.ComposePsStrict(context.Background(), t.TempDir()); err == nil || !strings.Contains(err.Error(), "truncated") {
		t.Fatalf("truncated strict response error=%v", err)
	}
}

func fakeDockerComposePsResponses(t *testing.T, normal, strict string) string {
	t.Helper()
	fixtureDir := t.TempDir()
	normalPath := filepath.Join(fixtureDir, "normal.json")
	strictPath := filepath.Join(fixtureDir, "strict.json")
	if err := os.WriteFile(normalPath, []byte(normal), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(strictPath, []byte(strict), 0o600); err != nil {
		t.Fatal(err)
	}
	return fakeDocker(t, fmt.Sprintf(`
case "$1 $2 $3 $4 $5" in
  "compose ps --format json ") cat %q ;;
  "compose ps --all --format json") cat %q ;;
  *) printf 'unexpected args: %%s %%s %%s %%s %%s' "$1" "$2" "$3" "$4" "$5" >&2; exit 7 ;;
esac
`, normalPath, strictPath))
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
			"if \"%1 %2 %3 %4 %5 %6\"==\"compose up -d --no-deps --force-recreate server\" (echo recreate server ok& exit /b 0)\r\n" +
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
