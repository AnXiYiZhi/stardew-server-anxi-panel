package stardew_junimo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
)

type ControlRuntimeGateState string

const (
	ControlRuntimeGatePending         ControlRuntimeGateState = "pending"
	ControlRuntimeGateReady           ControlRuntimeGateState = "ready"
	ControlRuntimeGateVersionMismatch ControlRuntimeGateState = "version_mismatch"
	ControlRuntimeGateInvalid         ControlRuntimeGateState = "invalid"
)

const (
	ControlRuntimeCodePending                          = "control_runtime_pending"
	ControlRuntimeCodeReady                            = "control_runtime_ready"
	ControlRuntimeCodeVersionMismatch                  = "control_runtime_version_mismatch"
	ControlRuntimeCodeManifestInvalid                  = "control_runtime_manifest_invalid"
	ControlRuntimeCodeDLLMissing                       = "control_runtime_dll_missing"
	ControlRuntimeCodeDLLUnreadable                    = "control_runtime_dll_unreadable"
	ControlRuntimeCodeDLLHashMismatch                  = "control_runtime_dll_hash_mismatch"
	ControlRuntimeCodeOptionsInvalid                   = "control_runtime_options_invalid"
	ControlRuntimeCodeOptionsUnreadable                = "control_runtime_options_unreadable"
	ControlRuntimeCodeLoginChatPrivacyPatchUnavailable = "control_runtime_login_chat_privacy_patch_unavailable"
	ControlRuntimeCodeHostFarmhousePatchUnavailable    = "control_runtime_host_farmhouse_patch_unavailable"
	ControlRuntimeCodeHostAutomationBridgeUnavailable  = "control_runtime_host_automation_bridge_unavailable"
	ControlRuntimeCodeHostSleepSafetyPatchUnavailable  = "control_runtime_host_sleep_safety_patch_unavailable"
)

// ControlRuntimeGateResult distinguishes a runtime snapshot that has not been
// written yet from an explicit version mismatch or an invalid runtime contract.
// Expected and Actual are safe version identifiers and never contain credentials.
type ControlRuntimeGateResult struct {
	State    ControlRuntimeGateState
	Code     string
	Expected string
	Actual   string
}

// InspectControlRuntimeGate returns the current Control runtime evidence without
// waiting or mutating the instance. A missing options.json is normal while SMAPI
// is still starting, so it is pending rather than a version mismatch.
func InspectControlRuntimeGate(dataDir string) ControlRuntimeGateResult {
	manifest, err := sjconfig.BuiltInRuntimeStackManifest()
	if err != nil || verifyEmbeddedControlManifest(manifest) != nil {
		return ControlRuntimeGateResult{State: ControlRuntimeGateInvalid, Code: ControlRuntimeCodeManifestInvalid}
	}

	result := ControlRuntimeGateResult{Expected: strings.TrimSpace(manifest.Control.Version)}
	installedDLL, err := os.ReadFile(filepath.Join(smapiModDir(dataDir), "StardewAnxiPanel.Control.dll"))
	if errors.Is(err, os.ErrNotExist) {
		result.State, result.Code = ControlRuntimeGateInvalid, ControlRuntimeCodeDLLMissing
		return result
	}
	if err != nil {
		result.State, result.Code = ControlRuntimeGateInvalid, ControlRuntimeCodeDLLUnreadable
		return result
	}
	if !bytes.Equal(installedDLL, smapiModDLL) {
		result.State, result.Code = ControlRuntimeGateInvalid, ControlRuntimeCodeDLLHashMismatch
		return result
	}

	raw, err := os.ReadFile(filepath.Join(controlDir(dataDir), "options.json"))
	if errors.Is(err, os.ErrNotExist) {
		result.State, result.Code = ControlRuntimeGatePending, ControlRuntimeCodePending
		return result
	}
	if err != nil {
		result.State, result.Code = ControlRuntimeGateInvalid, ControlRuntimeCodeOptionsUnreadable
		return result
	}
	var options struct {
		ControlModVersion                       string `json:"controlModVersion"`
		LoginChatPrivacyPatchAvailable          *bool  `json:"loginChatPrivacyPatchAvailable"`
		HostFarmhousePreservationPatchAvailable *bool  `json:"hostFarmhousePreservationPatchAvailable"`
		HostAutomationBridgeAvailable           *bool  `json:"hostAutomationBridgeAvailable"`
		HostSleepSafetyPatchAvailable           *bool  `json:"hostSleepSafetyPatchAvailable"`
	}
	if json.Unmarshal(raw, &options) != nil {
		result.State, result.Code = ControlRuntimeGateInvalid, ControlRuntimeCodeOptionsInvalid
		return result
	}
	result.Actual = strings.TrimSpace(options.ControlModVersion)
	if result.Actual == "" {
		result.State, result.Code = ControlRuntimeGateInvalid, ControlRuntimeCodeOptionsInvalid
		return result
	}
	if result.Actual != result.Expected {
		result.State, result.Code = ControlRuntimeGateVersionMismatch, ControlRuntimeCodeVersionMismatch
		return result
	}
	if options.LoginChatPrivacyPatchAvailable == nil || !*options.LoginChatPrivacyPatchAvailable {
		result.State, result.Code = ControlRuntimeGateInvalid, ControlRuntimeCodeLoginChatPrivacyPatchUnavailable
		return result
	}
	if options.HostFarmhousePreservationPatchAvailable == nil || !*options.HostFarmhousePreservationPatchAvailable {
		result.State, result.Code = ControlRuntimeGateInvalid, ControlRuntimeCodeHostFarmhousePatchUnavailable
		return result
	}
	if options.HostAutomationBridgeAvailable == nil || !*options.HostAutomationBridgeAvailable {
		result.State, result.Code = ControlRuntimeGateInvalid, ControlRuntimeCodeHostAutomationBridgeUnavailable
		return result
	}
	if options.HostSleepSafetyPatchAvailable == nil || !*options.HostSleepSafetyPatchAvailable {
		result.State, result.Code = ControlRuntimeGateInvalid, ControlRuntimeCodeHostSleepSafetyPatchUnavailable
		return result
	}
	result.State, result.Code = ControlRuntimeGateReady, ControlRuntimeCodeReady
	return result
}

func waitForControlRuntimeGate(ctx context.Context, dataDir string, timeout time.Duration) (ControlRuntimeGateResult, error) {
	if timeout <= 0 {
		timeout = time.Minute
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		result := InspectControlRuntimeGate(dataDir)
		if result.State != ControlRuntimeGatePending {
			return result, nil
		}
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case <-deadline.C:
			return result, nil
		case <-ticker.C:
		}
	}
}

// runningControlMatchesManifest retains the bool contract used by the runtime
// updater while sharing the richer classification with normal lifecycle code.
func runningControlMatchesManifest(dataDir string) bool {
	return InspectControlRuntimeGate(dataDir).State == ControlRuntimeGateReady
}

// waitForRunningControlManifest retains the legacy bool waiter for existing
// callers. New lifecycle paths should use waitForControlRuntimeGate so context
// cancellation, pending timeout, invalid state, and mismatch remain distinct.
func waitForRunningControlManifest(ctx context.Context, dataDir string, timeout time.Duration) bool {
	result, err := waitForControlRuntimeGate(ctx, dataDir, timeout)
	return err == nil && result.State == ControlRuntimeGateReady
}
