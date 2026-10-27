package stardew_junimo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
)

type fullStackCompatibility struct {
	OperationID           string `json:"operationId"`
	PlanSHA256            string `json:"planSha256"`
	PanelVersion          string `json:"panelVersion"`
	ApplyID               string `json:"applyId"`
	MemberID              string `json:"memberId"`
	Outcome               string `json:"outcome"`
	ServerImage           string `json:"serverImage"`
	ServerImageID         string `json:"serverImageId"`
	ServerDigest          string `json:"serverDigest"`
	ServerVersion         string `json:"serverVersion"`
	GameVolume            string `json:"gameVolume"`
	ControlSHA256         string `json:"controlSha256"`
	ControlManifestSHA256 string `json:"controlManifestSha256"`
	ControlVersion        string `json:"controlVersion"`
	JunimoSHA256          string `json:"junimoSha256"`
	InviteEnabled         bool   `json:"inviteEnabled"`
}

func fullStackCompatibilityPath(dir string) string {
	return filepath.Join(dir, ".local-container", "full-stack-compatibility.json")
}

func (d *Driver) writeFullStackCompatibility(op fullStackOperation, index int) error {
	group := op.Groups[index]
	txn, err := readJunimo140GroupTransaction(group.Plan)
	if err != nil || txn.Phase != "committed" && txn.Phase != "rolled_back" {
		return errors.New("full-stack compatibility requires a proven group terminal")
	}
	return d.writeFullStackCompatibilityReceipt(op, index, txn)
}

func (d *Driver) writeFullStackCompatibilityReceipt(op fullStackOperation, index int, txn junimo140GroupTransaction) error {
	identity := d.panelIdentity.Load()
	if identity == nil || !identity.SameContent(op.Public.Panel) {
		return errors.New("compatibility receipt requires the confirmed running Panel content")
	}
	group := op.Groups[index]
	for i, member := range group.Plan.Members {
		if rejectJunimo140GroupLease(member.DataDir) != nil {
			return errors.New("unfinished group cannot receive a compatibility receipt")
		}
		fields, err := sjconfig.ReadEnvFile(filepath.Join(member.DataDir, ".env"))
		if err != nil {
			return err
		}
		junimoSHA, err := runtimeRecoveryFileSHA256(filepath.Join(member.DataDir, ".local-container", "mods", "JunimoServer", "JunimoServer.dll"))
		if err != nil {
			return err
		}
		volume, err := GameDataVolumeName(member.DataDir)
		if err != nil {
			return err
		}
		control, err := runtimeRecoveryFileSHA256(filepath.Join(smapiModDir(member.DataDir), "StardewAnxiPanel.Control.dll"))
		if err != nil {
			return err
		}
		manifestPath := filepath.Join(smapiModDir(member.DataDir), "manifest.json")
		manifestSHA, err := runtimeRecoveryFileSHA256(manifestPath)
		if err != nil {
			return err
		}
		raw, err := os.ReadFile(manifestPath)
		var controlManifest struct{ Version string }
		if err != nil || json.Unmarshal(raw, &controlManifest) != nil || controlManifest.Version == "" {
			return errors.New("restored Control manifest is invalid")
		}
		image := txn.Target.Server
		if txn.Phase != "committed" {
			image = txn.Worlds[i].Original.Server
		}
		if !runtimeImageDigestPattern.MatchString(image.ImageID) || !runtimeImageDigestPattern.MatchString(image.Digest) {
			return errors.New("compatibility image identity is incomplete")
		}
		receipt := fullStackCompatibility{OperationID: op.Public.OperationID, PlanSHA256: op.Public.PlanSHA256, PanelVersion: op.Public.Panel.Version, ApplyID: group.Plan.ApplyID, MemberID: member.ID, Outcome: txn.Phase, ServerImage: fields["SERVER_IMAGE"], ServerImageID: image.ImageID, ServerDigest: image.Digest, GameVolume: volume, ControlSHA256: control, InviteEnabled: sjconfig.SteamInviteEnabled(member.DataDir)}
		receipt.ControlManifestSHA256, receipt.ControlVersion = manifestSHA, controlManifest.Version
		receipt.JunimoSHA256 = junimoSHA
		receipt.ServerVersion = txn.Recommendation.Server.Tag
		if txn.Phase != "committed" {
			receipt.ServerVersion = txn.Worlds[i].Original.Current.Server.Tag
		}
		if err := writePrivateRuntimeUpdateJSON(fullStackCompatibilityPath(member.DataDir), receipt); err != nil {
			return err
		}
	}
	return nil
}

func fullStackClaimWithoutMutation(txn junimo140GroupTransaction) bool {
	if txn.Phase != "claim_failed" || txn.StageCreateIntent || txn.WorldChangesStarted {
		return false
	}
	for _, world := range txn.Worlds {
		if world.SaveIntent || world.StopIntent || world.UpdateIntent || world.RestoreIntent {
			return false
		}
	}
	return true
}

// A later group may never have started when an earlier one fails. Its own
// confirmed original snapshot, revalidated without saving/stopping it, is the
// bounded proof for keeping that original stack usable on the new Panel.
func (d *Driver) writeFullStackTerminalCompatibility(ctx context.Context, op *fullStackOperation, index int) error {
	group := op.Groups[index]
	txn, err := readJunimo140GroupTransaction(group.Plan)
	if err == nil && (txn.Phase == "committed" || txn.Phase == "rolled_back") {
		return d.writeFullStackCompatibility(*op, index)
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) || err == nil && !fullStackClaimWithoutMutation(txn) {
		return errors.New("remaining group has an unresolved mutation journal")
	}
	if err := d.validateFullStackGroup(ctx, *op, index); err != nil {
		return err
	}
	txn = group.Original
	txn.Phase = "untouched"
	if err := d.writeFullStackCompatibilityReceipt(*op, index, txn); err != nil {
		return err
	}
	op.Public.Groups[index].Phase = "unchanged"
	return nil
}

// A failed .140 migration may restore .125 on the new Panel. This narrowly
// scoped, journal-backed receipt prevents that exact group from being locked
// into automatic retries. Different targets/configuration/Panel versions do
// not inherit the exception.
func (d *Driver) acceptsFullStackCompatibility(instance registry.Instance) bool {
	if d.containerDataDir == "" {
		return false
	}
	raw, err := os.ReadFile(fullStackCompatibilityPath(instance.DataDir))
	if err != nil {
		return false
	}
	var receipt fullStackCompatibility
	if json.Unmarshal(raw, &receipt) != nil || !fullStackOperationPattern.MatchString(receipt.OperationID) || receipt.MemberID != instance.ID || receipt.PanelVersion != d.panelVersion {
		return false
	}
	root, err := d.fullStackRoot()
	if err != nil {
		return false
	}
	raw, err = os.ReadFile(filepath.Join(root, receipt.OperationID+".json"))
	if err != nil {
		return false
	}
	var op fullStackOperation
	if json.Unmarshal(raw, &op) != nil || op.validateGroups() != nil || op.Public.AuthorizedAt == nil || op.Public.OperationID != receipt.OperationID || op.Public.PlanSHA256 != receipt.PlanSHA256 || op.digest() != receipt.PlanSHA256 || op.Public.Panel.Version != receipt.PanelVersion {
		return false
	}
	identity := d.panelIdentity.Load()
	if identity == nil || !identity.SameContent(op.Public.Panel) {
		return false
	}
	var group *fullStackPreparedGroup
	for i := range op.Groups {
		if op.Groups[i].Plan.ApplyID == receipt.ApplyID {
			group = &op.Groups[i]
			break
		}
	}
	if group == nil {
		return false
	}
	memberFound := false
	memberIndex := 0
	for i, member := range group.Plan.Members {
		if member.ID == instance.ID && sameJunimoGroupPath(member.DataDir, instance.DataDir) {
			memberFound = true
			memberIndex = i
		}
	}
	if !memberFound {
		return false
	}
	txn, err := readJunimo140GroupTransaction(group.Plan)
	if receipt.Outcome == "untouched" {
		if err != nil && !errors.Is(err, os.ErrNotExist) || err == nil && !fullStackClaimWithoutMutation(txn) {
			return false
		}
		txn = group.Original
		txn.Phase = "untouched"
	} else if err != nil || txn.Phase != receipt.Outcome || txn.Phase != "committed" && txn.Phase != "rolled_back" {
		return false
	}
	if rejectJunimo140GroupLease(instance.DataDir) != nil {
		return false
	}
	expected, expectedVolume, expectedVersion := txn.Target.Server, group.Plan.StagingVolume, txn.Recommendation.Server.Tag
	if txn.Phase != "committed" {
		original := txn.Worlds[memberIndex].Original
		expected, expectedVolume, expectedVersion = original.Server, group.Plan.SourceVolume, original.Current.Server.Tag
		if original.ControlDLLSHA256 == "" || original.ControlDLLSHA256 != receipt.ControlSHA256 || original.ControlManifestSHA256 != receipt.ControlManifestSHA256 {
			return false
		}
		if original.JunimoDLLSHA256 != "" && original.JunimoDLLSHA256 != receipt.JunimoSHA256 {
			return false
		}
	} else if receipt.ControlSHA256 != txn.Recommendation.Control.DLLSHA256 {
		return false
	}
	if receipt.ServerImageID != expected.ImageID || receipt.ServerDigest != expected.Digest || receipt.GameVolume != expectedVolume || receipt.ServerVersion != expectedVersion {
		return false
	}
	fields, err := sjconfig.ReadEnvFile(filepath.Join(instance.DataDir, ".env"))
	if err != nil || fields["SERVER_IMAGE"] != receipt.ServerImage {
		return false
	}
	if txn.Phase != "committed" && sjconfig.SteamInviteEnabled(instance.DataDir) != receipt.InviteEnabled {
		return false
	}
	// Optional authentication may be toggled after a committed migration. It
	// must still use the exact Auth content authorized by that transaction.
	if txn.Phase == "committed" && sjconfig.SteamInviteEnabled(instance.DataDir) {
		if !sjconfig.HasJunimo140SteamAuthTarget(txn.Recommendation.SteamAuth) {
			return false
		}
		expectedDigest := txn.Recommendation.SteamAuth.Digests[fields["STEAM_SERVICE_IMAGE"]]
		reader, ok := d.docker.(RuntimeUpdateDockerService)
		if !ok || !runtimeImageDigestPattern.MatchString(expectedDigest) {
			return false
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		metadata, inspectErr := reader.RuntimeImageInspect(ctx, instance.DataDir, fields["STEAM_SERVICE_IMAGE"])
		cancel()
		if inspectErr != nil || metadata.Digest != expectedDigest || !runtimeImageDigestPattern.MatchString(metadata.ID) {
			return false
		}
	}
	volume, err := GameDataVolumeName(instance.DataDir)
	if err != nil || volume != receipt.GameVolume {
		return false
	}
	control, err := runtimeRecoveryFileSHA256(filepath.Join(smapiModDir(instance.DataDir), "StardewAnxiPanel.Control.dll"))
	if err != nil {
		return false
	}
	manifestSHA, err := runtimeRecoveryFileSHA256(filepath.Join(smapiModDir(instance.DataDir), "manifest.json"))
	if err != nil {
		return false
	}
	baseline := control == receipt.ControlSHA256 && manifestSHA == receipt.ControlManifestSHA256
	dllSum, manifestSum := sha256.Sum256(smapiModDLL), sha256.Sum256(smapiModManifest)
	panelControl := control == hex.EncodeToString(dllSum[:]) && manifestSHA == hex.EncodeToString(manifestSum[:])
	if !baseline && !panelControl {
		return false
	}
	junimoSHA, err := runtimeRecoveryFileSHA256(filepath.Join(instance.DataDir, ".local-container", "mods", "JunimoServer", "JunimoServer.dll"))
	if err != nil || junimoSHA != receipt.JunimoSHA256 {
		return false
	}
	docker, ok := d.docker.(RuntimeUpdateDockerService)
	if !ok {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	image, err := docker.RuntimeImageInspect(ctx, instance.DataDir, receipt.ServerImage)
	return err == nil && image.ID == receipt.ServerImageID && image.Digest == receipt.ServerDigest
}

func (d *Driver) EffectiveRuntimeInspection(instance registry.Instance) sjconfig.RuntimeStackInspection {
	inspection := InspectManagedRuntimeStack(instance.DataDir, instance.State)
	if !d.acceptsFullStackCompatibility(instance) {
		return inspection
	}
	raw, err := os.ReadFile(fullStackCompatibilityPath(instance.DataDir))
	if err != nil {
		return inspection
	}
	var receipt fullStackCompatibility
	if json.Unmarshal(raw, &receipt) != nil {
		return inspection
	}
	root, err := d.fullStackRoot()
	if err != nil {
		return inspection
	}
	raw, err = os.ReadFile(filepath.Join(root, receipt.OperationID+".json"))
	if err != nil {
		return inspection
	}
	var op fullStackOperation
	if json.Unmarshal(raw, &op) != nil {
		return inspection
	}
	manifest, err := sjconfig.BuiltInRuntimeStackManifest()
	if err != nil {
		return inspection
	}
	if receipt.Outcome == "committed" {
		manifest.StackVersion, manifest.Server, manifest.SteamAuth = op.Target.StackVersion, op.Target.Server, op.Target.SteamAuth
		manifest.SMAPI, manifest.Game, manifest.SDK = op.Target.SMAPI, op.Target.Game, op.Target.SDK
	} else {
		manifest.StackVersion = "rollback-compatible-" + receipt.OperationID
		manifest.Server.Tag = receipt.ServerVersion
		manifest.Server.Images = []string{receipt.ServerImage}
		manifest.Server.TrustedCandidates = []string{receipt.ServerImage}
		manifest.Server.Digests = map[string]string{receipt.ServerImage: receipt.ServerDigest}
	}
	// Restored Control is accepted only by its exact receipt and original
	// terminal journal; ordinary version-wide safety gates remain in force.
	raw, err = os.ReadFile(filepath.Join(smapiModDir(instance.DataDir), "manifest.json"))
	var control struct{ Version string }
	if err != nil || json.Unmarshal(raw, &control) != nil {
		return inspection
	}
	actualControlSHA, err := runtimeRecoveryFileSHA256(filepath.Join(smapiModDir(instance.DataDir), "StardewAnxiPanel.Control.dll"))
	if err != nil {
		return inspection
	}
	manifest.Control.Version, manifest.Control.DLLSHA256 = control.Version, actualControlSHA
	return sjconfig.InspectRuntimeStackWithManifest(instance.DataDir, runtimeContentInstalled(instance.State), sjconfig.SteamInviteEnabled(instance.DataDir), manifest)
}
