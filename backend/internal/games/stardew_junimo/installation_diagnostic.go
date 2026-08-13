package stardew_junimo

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

// InstallationDiagnostic is the ordinary-user-safe evidence used by the UI to
// distinguish a runtime failure from an incomplete or absent installation.
// Unknown means the evidence could not be refreshed; it must never be rendered
// as "not installed".
type InstallationDiagnostic struct {
	Status            string                    `json:"status"`
	RequiredFiles     string                    `json:"requiredFiles"`
	Compose           string                    `json:"compose"`
	Image             string                    `json:"image"`
	ServerContainer   string                    `json:"serverContainer"`
	Control           InstallationControlStatus `json:"control"`
	RecommendedAction string                    `json:"recommendedAction"`
	CheckedAt         string                    `json:"checkedAt"`
}

type InstallationControlStatus struct {
	Static          string `json:"static"`
	Runtime         string `json:"runtime"`
	ObservedVersion string `json:"observedVersion,omitempty"`
	ExpectedVersion string `json:"expectedVersion"`
}

type requiredFilesEvidence struct {
	State     string
	CheckedAt time.Time
}

const installationEvidenceTTL = 10 * time.Minute

func (d *Driver) InstallationDiagnostic(ctx context.Context, instance registry.Instance) InstallationDiagnostic {
	diagnostic := InstallationDiagnostic{
		Status:            "unknown",
		RequiredFiles:     "unknown",
		Compose:           inspectComposeScaffold(instance.DataDir),
		Image:             "unavailable",
		ServerContainer:   "unknown",
		RecommendedAction: "diagnose",
		CheckedAt:         time.Now().UTC().Format(time.RFC3339),
	}
	diagnostic.Control = inspectInstalledControl(instance.DataDir)

	if d.docker != nil {
		if ps, err := d.docker.ComposePs(ctx, instance.DataDir); err == nil {
			diagnostic.ServerContainer = "stopped"
			if serverServiceUp(ps.Services) {
				diagnostic.ServerContainer = "running"
			}
			if len(ps.Services) == 0 && diagnostic.Compose == "missing" {
				diagnostic.ServerContainer = "missing"
			}
		}

		imageRef := gameInstallImage(instance.DataDir)
		result, err := d.docker.ImageInspect(ctx, instance.DataDir, imageRef)
		if err == nil {
			diagnostic.Image = "available"
		} else if explicitImageMissing(result.Stdout + "\n" + result.Stderr + "\n" + err.Error()) {
			diagnostic.Image = "missing"
		}

		if evidence, ok := d.cachedInstallationEvidence(instance.ID); ok {
			diagnostic.RequiredFiles = evidence.State
		} else if diagnostic.Image == "available" && instance.State == storage.InstanceStateError {
			ok, err := d.verifyGameDataVolume(ctx, instance.DataDir, imageRef, nil)
			if err == nil {
				if ok {
					diagnostic.RequiredFiles = "ok"
				} else {
					diagnostic.RequiredFiles = "missing"
				}
				d.rememberInstallationEvidence(instance.ID, diagnostic.RequiredFiles)
			}
		}
	}

	if diagnostic.RequiredFiles == "unknown" && requiresInstalledFiles(instance.State) {
		// ReconcileState and every normal Start already gate these states on the
		// same verifier. Error is intentionally refreshed above because its cause
		// may be unrelated to installation integrity.
		diagnostic.RequiredFiles = "ok"
	}

	inspectRuntimeControl(instance.DataDir, diagnostic.ServerContainer == "running", &diagnostic.Control)
	diagnostic.classify(instance.State)
	return diagnostic
}

func (d *InstallationDiagnostic) classify(instanceState string) {
	if instanceState == storage.InstanceStateUninitialized || instanceState == storage.InstanceStateAdminCreated || instanceState == storage.InstanceStateJunimoScaffolded {
		if d.RequiredFiles != "ok" {
			d.Status = "not_installed"
			d.RecommendedAction = "install"
			return
		}
	}
	if (instanceState == storage.InstanceStateUninitialized || instanceState == storage.InstanceStateAdminCreated) && d.Compose == "missing" {
		d.Status = "not_installed"
		d.RecommendedAction = "install"
		return
	}
	if d.RequiredFiles == "missing" || (d.Compose == "missing" || d.Compose == "invalid") && instanceState != storage.InstanceStateUninitialized && instanceState != storage.InstanceStateAdminCreated || d.Image == "missing" {
		d.Status = "incomplete"
		d.RecommendedAction = "repair_install"
		return
	}
	if d.RequiredFiles == "ok" && d.Compose == "ready" && d.Image == "available" {
		d.Status = "installed"
		if d.ServerContainer == "running" && (d.Control.Runtime == "mismatch" || d.Control.Runtime == "invalid") {
			d.RecommendedAction = "diagnose"
		} else {
			d.RecommendedAction = "retry_start"
		}
		return
	}
	// Docker/image/volume evidence which cannot currently be read is unknown,
	// never absent. The UI should offer diagnostics/retry rather than reinstall.
	d.Status = "unknown"
	d.RecommendedAction = "diagnose"
}

func (d *Driver) rememberInstallationEvidence(instanceID, state string) {
	if state != "ok" && state != "missing" {
		return
	}
	d.installationEvidenceMu.Lock()
	d.installationEvidence[instanceID] = requiredFilesEvidence{State: state, CheckedAt: time.Now().UTC()}
	d.installationEvidenceMu.Unlock()
}

func (d *Driver) cachedInstallationEvidence(instanceID string) (requiredFilesEvidence, bool) {
	d.installationEvidenceMu.Lock()
	defer d.installationEvidenceMu.Unlock()
	evidence, ok := d.installationEvidence[instanceID]
	if !ok || time.Since(evidence.CheckedAt) > installationEvidenceTTL {
		delete(d.installationEvidence, instanceID)
		return requiredFilesEvidence{}, false
	}
	return evidence, true
}

func inspectComposeScaffold(dataDir string) string {
	raw, err := os.ReadFile(filepath.Join(dataDir, "docker-compose.yml"))
	if os.IsNotExist(err) {
		return "missing"
	}
	if err != nil {
		return "unavailable"
	}
	text := string(raw)
	if !strings.Contains(text, "services:") || !strings.Contains(text, "server:") {
		return "invalid"
	}
	return "ready"
}

func explicitImageMissing(detail string) bool {
	detail = strings.ToLower(detail)
	return strings.Contains(detail, "no such image") || strings.Contains(detail, "image not found")
}

func inspectInstalledControl(dataDir string) InstallationControlStatus {
	result := InstallationControlStatus{Static: "unknown", Runtime: "not_observed"}
	manifest, err := sjconfig.BuiltInRuntimeStackManifest()
	if err != nil {
		return result
	}
	result.ExpectedVersion = manifest.Control.Version
	manifestRaw, manifestErr := os.ReadFile(filepath.Join(smapiModDir(dataDir), "manifest.json"))
	dllRaw, dllErr := os.ReadFile(filepath.Join(smapiModDir(dataDir), "StardewAnxiPanel.Control.dll"))
	if os.IsNotExist(manifestErr) || os.IsNotExist(dllErr) {
		result.Static = "missing"
		return result
	}
	if manifestErr != nil || dllErr != nil {
		return result
	}
	var installed struct {
		Version string `json:"Version"`
	}
	if json.Unmarshal(manifestRaw, &installed) != nil {
		result.Static = "mismatch"
		return result
	}
	if strings.TrimSpace(installed.Version) == result.ExpectedVersion && bytes.Equal(dllRaw, smapiModDLL) {
		result.Static = "match"
	} else {
		result.Static = "mismatch"
	}
	return result
}

func inspectRuntimeControl(dataDir string, serverRunning bool, result *InstallationControlStatus) {
	if !serverRunning {
		result.Runtime = "not_observed"
		return
	}
	raw, err := os.ReadFile(filepath.Join(controlDir(dataDir), "options.json"))
	if os.IsNotExist(err) {
		result.Runtime = "not_observed"
		return
	}
	if err != nil {
		result.Runtime = "unknown"
		return
	}
	var options struct {
		Version string `json:"controlModVersion"`
	}
	if json.Unmarshal(raw, &options) != nil || strings.TrimSpace(options.Version) == "" {
		result.Runtime = "invalid"
		return
	}
	result.ObservedVersion = strings.TrimSpace(options.Version)
	if result.ObservedVersion == result.ExpectedVersion {
		result.Runtime = "match"
	} else {
		result.Runtime = "mismatch"
	}
}
