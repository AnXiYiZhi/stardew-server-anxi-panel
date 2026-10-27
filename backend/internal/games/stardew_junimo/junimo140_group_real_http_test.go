//go:build integration

package stardew_junimo

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/auth"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func sjSeedRealPanelAdmin(t *testing.T, ctx context.Context, store *storage.Store) string {
	t.Helper()
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		t.Fatal(err)
	}
	password := hex.EncodeToString(secret)
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	if exists, err := store.AdminExists(ctx); err != nil {
		t.Fatal(err)
	} else if exists {
		user, err := store.GetUserByUsername(ctx, "engineering-admin")
		if err != nil || user.Role != auth.RoleAdmin || !user.IsSuperAdmin {
			t.Fatal("isolated resume admin identity changed", err)
		}
		if _, err := store.UpdateUser(ctx, user.ID, user.ID, storage.UpdateUserParams{PasswordHash: &hash}); err != nil {
			t.Fatal(err)
		}
		return password
	}
	if _, _, err := store.CreateFirstAdminWithSession(ctx, storage.CreateFirstAdminParams{Username: "engineering-admin", PasswordHash: hash, TokenHash: auth.HashSessionToken(password, "unused-fixture-session"), ExpiresAt: time.Now().Add(-time.Minute)}); err != nil {
		t.Fatal(err)
	}
	return password
}

// The QA executable is built from the frozen production source with only the
// embedded recommendation changed to .140/recommended. It adds no API hooks.
func sjRunRealGroupViaPanelHTTP(t *testing.T, ctx context.Context, repo, root, password string, primary registry.Instance, proof map[string]any, livePlan *junimo140GroupPlan) (RuntimeUpdateApplyStatus, junimo140GroupPlan) {
	t.Helper()
	executable := filepath.Join(repo, "tmp", "junimo140-20261015", "panel-http-qa-v8.exe")
	bin, err := os.ReadFile(executable)
	if err != nil || len(bin) == 0 {
		t.Fatal("isolated QA executable missing", err)
	}
	hash := sha256.Sum256(bin)
	proof["panelExecutableSha256"] = hex.EncodeToString(hash[:])
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := "127.0.0.1:" + strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	logName := "panel-http-" + strings.TrimPrefix(newRuntimeApplyID(), "apply_") + ".log"
	proof["panelLog"] = logName
	logFile, err := os.OpenFile(filepath.Join(root, logName), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer logFile.Close()
	childCtx, stop := context.WithCancel(ctx)
	defer stop()
	cmd := exec.CommandContext(childCtx, executable)
	cmd.Dir, cmd.Stdout, cmd.Stderr = repo, logFile, logFile
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		key = strings.ToUpper(key)
		if !strings.HasPrefix(key, "PANEL_") && !strings.HasPrefix(key, "DEFAULT_") && !strings.HasPrefix(key, "STEAM_") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		t.Fatal(err)
	}
	cmd.Env = append(cmd.Env, "PANEL_ADDR="+addr, "PANEL_DATA_DIR="+root, "PANEL_DB_PATH="+filepath.Join(root, "panel.db"), "PANEL_HOST_DATA_DIR="+root, "PANEL_VERSION=0.7.3", "PANEL_MODE=multi", "DEFAULT_INSTANCE_ID="+primary.ID, "PANEL_SECRET="+hex.EncodeToString(secret))
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	proof["panelProcess"] = map[string]any{"pid": cmd.Process.Pid, "executable": executable, "loopback": addr}
	proof["panelProcessStopped"] = false
	finished := make(chan error, 1)
	go func() { finished <- cmd.Wait() }()
	defer func() {
		// This exact handle belongs to the child just started above. Wait for
		// termination before the caller's Docker fixture cleanup can begin.
		stop()
		select {
		case <-finished:
			proof["panelProcessStopped"] = true
		case <-time.After(10 * time.Second):
			t.Error("isolated Panel process did not exit; retain fixture")
		}
	}()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Timeout: 25 * time.Second, Jar: jar}
	base := "http://" + addr
	request := func(method, path string, body any, want int, target any) {
		t.Helper()
		var input io.Reader
		if body != nil {
			raw, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			input = bytes.NewReader(raw)
		}
		req, err := http.NewRequestWithContext(ctx, method, base+path, input)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		response, err := client.Do(req)
		if err != nil {
			t.Fatal("isolated Panel HTTP request", path, err)
		}
		defer response.Body.Close()
		raw, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != want {
			t.Fatalf("isolated Panel %s %s returned %d: %s", method, path, response.StatusCode, raw)
		}
		if target != nil {
			if err := json.Unmarshal(raw, target); err != nil {
				t.Fatal("isolated Panel JSON", path, err)
			}
		}
	}
	deadline := time.Now().Add(time.Minute)
	for {
		response, err := client.Get(base + "/health")
		if err == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("isolated Panel readiness timeout; inspect panel-http.log")
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(300 * time.Millisecond):
		}
	}
	request(http.MethodPost, "/api/auth/login", map[string]string{"username": "engineering-admin", "password": password}, http.StatusOK, nil)
	path := "/api/instances/" + primary.ID + "/junimo-update"
	var inspection struct {
		Recommended struct {
			Server struct {
				Tag string `json:"tag"`
			} `json:"server"`
		} `json:"recommended"`
	}
	request(http.MethodGet, path, nil, http.StatusOK, &inspection)
	if inspection.Recommended.Server.Tag != "1.5.0-preview.140" {
		t.Fatal("QA recommendation did not select exact140")
	}
	if os.Getenv("ANXI_REAL_JUNIMO140_PANEL_RESUME") == "1" {
		var status RuntimeUpdateApplyStatus
		request(http.MethodGet, path+"/apply", nil, http.StatusOK, &status)
		if status.ApplyID != livePlan.ApplyID || status.Phase != RuntimeUpdateApplySucceeded {
			t.Fatal("resume lost the committed real group")
		}
		client.Timeout = 3 * time.Minute
		sjRunRealPanelBusiness(t, ctx, repo, root, primary, client, base, request, proof)
		return status, *livePlan
	}
	var dry RuntimeUpdateDryRunStatus
	request(http.MethodPost, path+"/dry-run", map[string]any{}, http.StatusAccepted, &dry)
	for dry.Phase != RuntimeUpdatePhaseSucceeded {
		if dry.Phase == RuntimeUpdatePhaseFailed {
			t.Fatalf("real Web dry-run failed: %s %s", dry.ErrorCode, dry.Error)
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(time.Second):
		}
		request(http.MethodGet, path+"/dry-run", nil, http.StatusOK, &dry)
	}
	if dry.SharedGroupMembers != 3 || !junimo140GroupSHA256.MatchString(dry.SharedGroupSHA256) {
		t.Fatal("real Web dry-run lost three-world confirmation scope")
	}
	proof["webDryRun"] = dry
	var before, after RuntimeUpdateApplyStatus
	request(http.MethodGet, path+"/apply", nil, http.StatusOK, &before)
	var rejection struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	request(http.MethodPost, path+"/apply", map[string]any{"confirm": true}, http.StatusConflict, &rejection)
	if rejection.Error.Code != "shared_group_confirmation_required" {
		t.Fatal("real Web apply without scope was not refused before submission", rejection.Error.Code)
	}
	request(http.MethodPost, path+"/apply", map[string]any{"confirm": true, "sharedGroupSha256": strings.Repeat("0", 64)}, http.StatusConflict, &rejection)
	if rejection.Error.Code != "shared_group_confirmation_required" {
		t.Fatal("real Web apply with stale scope was not refused", rejection.Error.Code)
	}
	request(http.MethodGet, path+"/apply", nil, http.StatusOK, &after)
	if before.ApplyID != after.ApplyID || before.JobID != after.JobID || before.Phase != after.Phase {
		t.Fatal("unconfirmed Web requests changed the apply owner")
	}
	proof["webUnconfirmedRequestsNoEffect"] = true
	var status RuntimeUpdateApplyStatus
	request(http.MethodPost, path+"/apply", map[string]any{"confirm": true, "sharedGroupSha256": dry.SharedGroupSHA256}, http.StatusAccepted, &status)
	proof["webConfirmedScope"], proof["webApplyId"] = dry.SharedGroupSHA256, status.ApplyID
	if !junimo140GroupApplyID.MatchString(status.ApplyID) {
		t.Fatal("invalid Web apply identity")
	}
	var journal junimo140GroupJournal
	if err := readJunimo140GroupJSON(filepath.Join(primary.DataDir, ".local-container", "junimo140-group", status.ApplyID, "manifest.json"), &journal); err != nil {
		t.Fatal(err)
	}
	if journal.Plan.ApplyID != status.ApplyID || journal.Plan.PrimaryID != primary.ID || journal.PlanSHA256 != journal.Plan.digest() || journal.Plan.membershipDigest() != dry.SharedGroupSHA256 {
		t.Fatal("real Web transaction did not retain confirmed group identity")
	}
	*livePlan = journal.Plan
	proof["applyId"], proof["jobId"], proof["planSha256"], proof["stageVolume"] = status.ApplyID, status.JobID, journal.Plan.digest(), journal.Plan.StagingVolume
	if err := writePrivateRuntimeUpdateJSON(filepath.Join(root, "proof.json"), proof); err != nil {
		t.Fatal(err)
	}
	for status.Phase != RuntimeUpdateApplySucceeded {
		if status.Phase == RuntimeUpdateApplyFailedRolledBack || status.Phase == RuntimeUpdateApplyRollbackFailed {
			t.Fatalf("real Web apply failed: %s %s", status.Phase, status.ErrorCode)
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(time.Second):
		}
		request(http.MethodGet, path+"/apply", nil, http.StatusOK, &status)
	}
	proof["webApplyTerminal"] = status
	if os.Getenv("ANXI_REAL_JUNIMO140_PANEL_BUSINESS") == "1" {
		client.Timeout = 3 * time.Minute
		sjRunRealPanelBusiness(t, ctx, repo, root, primary, client, base, request, proof)
	}
	return status, journal.Plan
}
