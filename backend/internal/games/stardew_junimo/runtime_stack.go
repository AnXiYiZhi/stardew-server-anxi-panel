package stardew_junimo

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

// InspectRuntimeStack reports the immutable, read-only Junimo server and
// steam-auth-cn version-pair status for an instance.
func InspectRuntimeStack(dataDir, state string) sjconfig.RuntimeStackInspection {
	installed := state != storage.InstanceStateUninitialized && state != storage.InstanceStateAdminCreated
	return sjconfig.InspectRuntimeStack(dataDir, installed)
}

// InspectManagedRuntimeStack extends the public component matrix with the
// Panel-owned Control files. Other inspectors (for example SMAPI-only update
// detection) intentionally keep their original component boundary.
func InspectManagedRuntimeStack(dataDir, state string) sjconfig.RuntimeStackInspection {
	inspection := InspectRuntimeStack(dataDir, state)
	if inspection.Status != sjconfig.RuntimeStackStatusUpToDate || !runtimeContentInstalled(state) {
		return inspection
	}
	manifest, err := sjconfig.BuiltInRuntimeStackManifest()
	if err != nil {
		return inspection
	}
	manifestData, manifestErr := os.ReadFile(filepath.Join(smapiModDir(dataDir), "manifest.json"))
	dllData, dllErr := os.ReadFile(filepath.Join(smapiModDir(dataDir), "StardewAnxiPanel.Control.dll"))
	var installed struct {
		Version string `json:"Version"`
	}
	code := "control_update_available"
	reason := "Control Mod 文件或版本与当前 Panel 不一致，必须通过受控重启重新同步。"
	if manifestErr == nil && dllErr == nil && json.Unmarshal(manifestData, &installed) == nil && strings.TrimSpace(installed.Version) == manifest.Control.Version && bytes.Equal(dllData, smapiModDLL) {
		return inspection
	}
	if dllErr == nil && !bytes.Equal(dllData, smapiModDLL) {
		code, reason = "control_dll_hash_mismatch", "Control Mod DLL SHA256 与当前 Panel 推荐产物不一致，必须通过受控重启重新同步。"
	}
	inspection.Available, inspection.Supported = true, true
	inspection.Status, inspection.Code, inspection.Reason = sjconfig.RuntimeStackStatusUpdateAvailable, code, reason
	return inspection
}
