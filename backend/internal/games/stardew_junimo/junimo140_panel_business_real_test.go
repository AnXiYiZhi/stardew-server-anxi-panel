//go:build integration

package stardew_junimo

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
)

func sjRealBaselineZip(t *testing.T, repo string) []byte {
	t.Helper()
	file, err := os.Open(filepath.Join(repo, "tmp", "junimo140-20261015", "house-bed-matrix-v1", "baseline.tar"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := tar.NewReader(file)
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	files := 0
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		name := strings.TrimPrefix(header.Name, "./")
		name, found := strings.CutPrefix(name, "Saves/")
		if !found || header.Typeflag != tar.TypeReg {
			continue
		}
		if path.Clean(name) != name || strings.HasPrefix(name, "/") || strings.HasPrefix(name, "../") || strings.Count(name, "/") != 1 {
			t.Fatal("baseline save ZIP scope is not a single plain save folder")
		}
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(entry, reader); err != nil {
			t.Fatal(err)
		}
		files++
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if files < 2 {
		t.Fatal("baseline save ZIP did not contain main save and metadata")
	}
	return output.Bytes()
}

func sjRunRealPanelBusiness(t *testing.T, ctx context.Context, repo, root string, primary registry.Instance, client *http.Client, base string, request func(string, string, any, int, any), proof map[string]any) {
	t.Helper()
	result := map[string]any{"phase": "players_read", "real140Game": true, "steamLoginUsed": false}
	if previous, ok := proof["panelBusiness"].(map[string]any); ok {
		result = previous
	}
	proof["panelBusiness"] = result
	checkpoint := func(phase string) {
		t.Helper()
		result["phase"] = phase
		if err := writePrivateRuntimeUpdateJSON(filepath.Join(root, "proof.json"), proof); err != nil {
			t.Fatal(err)
		}
	}
	waitJob := func(id string) {
		t.Helper()
		if id == "" {
			t.Fatal("real Panel business did not return a job identity")
		}
		for {
			var response struct {
				Job struct {
					ID           string  `json:"id"`
					Status       string  `json:"status"`
					ErrorMessage *string `json:"errorMessage"`
				} `json:"job"`
			}
			request(http.MethodGet, "/api/jobs/"+id, nil, http.StatusOK, &response)
			if response.Job.ID != id {
				t.Fatal("real Panel returned another job")
			}
			if response.Job.Status == "succeeded" {
				return
			}
			if response.Job.Status == "failed" || response.Job.Status == "cancelled" {
				var logs any
				request(http.MethodGet, "/api/jobs/"+id+"/logs", nil, http.StatusOK, &logs)
				if err := writePrivateRuntimeUpdateJSON(filepath.Join(root, "failed-panel-business-job-logs.json"), logs); err != nil {
					t.Error(err)
				}
				message := ""
				if response.Job.ErrorMessage != nil {
					message = *response.Job.ErrorMessage
				}
				t.Fatalf("real Panel business job ended %s: %s", response.Job.Status, message)
			}
			select {
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			case <-time.After(time.Second):
			}
		}
	}
	playersPath := "/api/instances/" + primary.ID + "/players"
	if operation, ok := result["importOperationId"].(string); ok && operation != "" {
		journal, err := LoadImportJournal(filepath.Join(root, "j140-real-"+proof["identity"].(string)+"-b"), operation)
		if err != nil {
			t.Fatal(err)
		}
		sjAcceptRealAsIsImport(t, ctx, journal, result, request)
		checkpoint("as_is_import_accepted")
	}
	if result["asIsImportCompleted"] == true && result["asIsPeerStopped"] != true {
		sjStopRealPanelPeer(t, ctx, request)
		result["asIsPeerStopped"] = true
		checkpoint("as_is_peer_stopped")
	}
	if result["offlineRoleDeleted"] != true {
		var players PlayersResult
		var target PlayerInfo
		deadline := time.Now().Add(40 * time.Second)
		for {
			request(http.MethodGet, playersPath, nil, http.StatusOK, &players)
			for _, player := range players.Players {
				if player.Name == "Junimo140IP" && !player.IsHost && player.SaveCharacterPresent && player.CanDeleteCharacter {
					target = player
				}
			}
			if target.UniqueMultiplayerID != "" && players.SaveID != "" {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("real Panel did not expose the original offline deletable role")
			}
			select {
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			case <-time.After(time.Second):
			}
		}
		body := map[string]any{"uniqueMultiplayerId": target.UniqueMultiplayerID, "expectedName": target.Name, "expectedSaveId": players.SaveID, "acknowledged": false}
		request(http.MethodPost, playersPath+"/delete-farmhand", body, http.StatusBadRequest, nil)
		body["acknowledged"] = true
		var deletion struct {
			JobID string `json:"jobId"`
		}
		request(http.MethodPost, playersPath+"/delete-farmhand", body, http.StatusAccepted, &deletion)
		result["deleteJobId"] = deletion.JobID
		checkpoint("deleting_offline_role")
		waitJob(deletion.JobID)
		deadline = time.Now().Add(30 * time.Second)
		for {
			request(http.MethodGet, playersPath, nil, http.StatusOK, &players)
			present := false
			for _, player := range players.Players {
				if player.UniqueMultiplayerID == target.UniqueMultiplayerID && player.SaveCharacterPresent {
					present = true
				}
			}
			if !present {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("real Panel deletion left the farmhand in the active save")
			}
			select {
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			case <-time.After(time.Second):
			}
		}
		result["offlineRoleDeleted"] = true
	} else {
		var players PlayersResult
		request(http.MethodGet, playersPath, nil, http.StatusOK, &players)
		for _, player := range players.Players {
			if player.Name == "Junimo140IP" && player.SaveCharacterPresent {
				t.Fatal("resume deletion evidence disagrees with the active save")
			}
		}
		result["completedDeletionNotReplayed"] = true
	}
	checkpoint("uploading_real_save")
	archive := sjRealBaselineZip(t, repo)
	zipHash := sha256.Sum256(archive)
	result["importArchiveSha256"] = hex.EncodeToString(zipHash[:])
	var form bytes.Buffer
	multipartWriter := multipart.NewWriter(&form)
	part, err := multipartWriter.CreateFormFile("save", "isolated-baseline.zip")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(archive); err != nil {
		t.Fatal(err)
	}
	if err := multipartWriter.Close(); err != nil {
		t.Fatal(err)
	}
	importPath := "/api/instances/b/saves/"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+importPath+"upload-preview", &form)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	response, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, readErr := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	response.Body.Close()
	if readErr != nil {
		t.Fatal(readErr)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("real Panel save preview returned %d: %s", response.StatusCode, raw)
	}
	var preview registry.UploadPreviewResult
	if err := json.Unmarshal(raw, &preview); err != nil || preview.Token == "" || preview.SaveName == "" {
		t.Fatal("real Panel preview missing transaction identity", err)
	}
	if previous, ok := proof["previousPreSubmitImport"].(map[string]any); ok {
		operation, _ := previous["operationId"].(string)
		job, _ := previous["jobId"].(string)
		peerDir := filepath.Join(root, "j140-real-"+proof["identity"].(string)+"-b")
		sjProveWebFailedImportCleanup(t, peerDir, operation, job)
		result["failedPreSubmitAutoRecoveredViaWeb"] = true
	}
	if result["destinationSaveDeletedForImport"] != true {
		// This fixture deliberately imports the same authorized baseline. Make
		// space through the normal backup-before-delete API on the stopped peer.
		var deleted struct {
			OK            bool `json:"ok"`
			BackupCreated bool `json:"backupCreated"`
		}
		request(http.MethodDelete, importPath+url.PathEscape(preview.SaveName), nil, http.StatusOK, &deleted)
		if !deleted.OK || !deleted.BackupCreated {
			t.Fatal("stopped peer save deletion did not retain its backup")
		}
		result["destinationSaveDeletedForImport"] = true
		checkpoint("uploading_real_save")
	}
	importBody := map[string]any{"token": preview.Token, "hostHandling": map[string]any{"mode": "virtual_host_takeover", "acknowledged": true}}
	swapMode := result["asIsImportCompleted"] == true
	if swapMode {
		// A synthetic identifier exercises the local binding contract without
		// logging in or contacting any platform account.
		importBody["hostHandling"] = map[string]any{"mode": "swap_to_player", "platformId": "76561198000000001"}
		result["syntheticPlatformIdentifierOnly"] = true
	}
	var imported struct {
		JobID       string `json:"jobId"`
		OperationID string `json:"operationId"`
		SaveName    string `json:"saveName"`
	}
	request(http.MethodPost, importPath+"upload-commit-and-start", importBody, http.StatusAccepted, &imported)
	result["importJobId"], result["importOperationId"] = imported.JobID, imported.OperationID
	checkpoint("importing_real_save")
	var repeated struct {
		JobID       string `json:"jobId"`
		OperationID string `json:"operationId"`
	}
	request(http.MethodPost, importPath+"upload-commit-and-start", importBody, http.StatusAccepted, &repeated)
	if imported.JobID != repeated.JobID || imported.OperationID != repeated.OperationID {
		t.Fatal("real import retry submitted a duplicate job or command owner")
	}
	result["importRetrySameOwner"] = true
	waitJob(imported.JobID)
	peerDir := filepath.Join(root, "j140-real-"+proof["identity"].(string)+"-b")
	journal, err := LoadImportJournal(peerDir, imported.OperationID)
	if !swapMode {
		if err != nil {
			t.Fatal(err)
		}
		sjAcceptRealAsIsImport(t, ctx, journal, result, request)
		checkpoint("as_is_import_accepted")
		sjRunRealPanelBusiness(t, ctx, repo, root, primary, client, base, request, proof)
		return
	}
	if err != nil || journal.Stage != ImportStageCompleted || !journal.UpstreamConfirmed || !journal.FarmhandUnbindVerified || !journal.DurableGameLoopSaved || journal.DurableSaveCommandID == "" || journal.DurableSaveAfter == nil {
		t.Fatal("real import did not prove release, exact Saved and durable save state", err)
	}
	result["importCompleted"], result["farmhandUnbindVerified"], result["exactSavedAndDiskVerified"] = true, true, true
	checkpoint("restoring_stopped_peer")
	sjStopRealPanelPeer(t, ctx, request)
	result["passed"] = true
	checkpoint("completed")
}

func sjRealAsIsImportComplete(journal ImportJournal) bool {
	return journal.Stage == ImportStageCompleted && journal.HostHandling == "server_owns_original" && journal.ActivationOutcome == activationOutcomeAsIsLoaded && journal.UpstreamSubmitted && journal.UpstreamConfirmed && journal.DurableSaveCommandID == "" && (!journal.BootstrapSaveCreated || journal.BootstrapCleanupCompleted)
}

func sjAcceptRealAsIsImport(t *testing.T, ctx context.Context, journal ImportJournal, result map[string]any, request func(string, string, any, int, any)) {
	t.Helper()
	if !sjRealAsIsImportComplete(journal) {
		t.Fatal("as-is import did not retain its stable-load/no-extra-save contract")
	}
	result["asIsImportCompleted"], result["asIsLoadedWithoutExtraSaveCommand"] = true, true
	result["asIsJobId"], result["asIsOperationId"] = journal.JobID, journal.OperationID
	sjStopRealPanelPeer(t, ctx, request)
	result["asIsPeerStopped"] = true
	delete(result, "importJobId")
	delete(result, "importOperationId")
	delete(result, "importRetrySameOwner")
	result["destinationSaveDeletedForImport"] = false
}

func sjStopRealPanelPeer(t *testing.T, ctx context.Context, request func(string, string, any, int, any)) {
	t.Helper()
	request(http.MethodPost, "/api/instances/b/stop", map[string]any{}, http.StatusAccepted, nil)
	deadline := time.Now().Add(5 * time.Minute)
	for {
		var status struct {
			Instance struct {
				State string `json:"state"`
			} `json:"instance"`
			Status registry.ServerStatus `json:"status"`
		}
		request(http.MethodGet, "/api/instances/b/status", nil, http.StatusOK, &status)
		serverStopped := false
		if status.Status.Runtime != nil {
			for _, container := range status.Status.Runtime.Containers {
				if container.Service == "server" {
					serverStopped = container.State == "exited"
					break
				}
			}
		}
		if status.Instance.State == "stopped" && status.Status.State == "stopped" && serverStopped {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("real Panel stop did not reach the durable stopped state")
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(time.Second):
		}
	}
}

func sjProveWebFailedImportCleanup(t *testing.T, dataDir, operationID, jobID string) {
	t.Helper()
	if operationID == "" || jobID == "" {
		t.Fatal("cleanup proof lacks its operation and job")
	}
	if _, err := LoadImportJournal(dataDir, operationID); !os.IsNotExist(err) {
		t.Fatal("cleanup did not remove the finalized journal", err)
	}
	dir := filepath.Join(controlDir(dataDir), "save-import-cleanup-receipts")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	matches := 0
	for _, entry := range entries {
		if entry.IsDir() || !junimo140GroupSHA256.MatchString(strings.TrimSuffix(entry.Name(), ".json")) {
			t.Fatal("cleanup receipt directory has an unexpected entry")
		}
		var receipt struct {
			SchemaVersion int       `json:"schemaVersion"`
			InstanceID    string    `json:"instanceId"`
			OperationID   string    `json:"operationId"`
			JobID         string    `json:"jobId"`
			JobType       string    `json:"jobType"`
			JobKey        string    `json:"jobIdempotencyKey"`
			CompletedAt   time.Time `json:"completedAt"`
		}
		if err := readJunimo140GroupJSON(filepath.Join(dir, entry.Name()), &receipt); err != nil {
			t.Fatal(err)
		}
		if receipt.OperationID != operationID {
			continue
		}
		if receipt.SchemaVersion != 1 || receipt.InstanceID != "b" || receipt.JobID != jobID || receipt.JobType != SaveImportJobType || receipt.JobKey != SaveImportJobIdempotencyKey(operationID) || receipt.CompletedAt.IsZero() {
			t.Fatal("cleanup receipt identity mismatch")
		}
		matches++
	}
	if matches != 1 {
		t.Fatal("cleanup operation has no unique durable receipt")
	}
	if busy, err := HasUnfinishedImportTransaction(dataDir); err != nil || busy {
		t.Fatal("cleanup left an unfinished import", err)
	}
}
