package stardew_junimo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/updater"
)

// Deliberately excludes pull/install/marker/clone/save/stop/recreate operations.
type currentFullStackDocker interface {
	RuntimeReadCurrentJunimo140GameEvidence(context.Context, string, string, string, string, string, []string) ([]byte, error)
	RuntimeVerifyCurrentJunimo140GameFiles(context.Context, string, string, string, string, string, map[string]string, []string) (paneldocker.RuntimeGameFileStats, error)
	RuntimeInspectCurrentJunimo140SDK(context.Context, string, string, string, string, []string) (paneldocker.RuntimeGameFileStats, error)
	RuntimeReadCurrentPinnedSMAPIMetadata(context.Context, string, string, string, string, []string) (paneldocker.RuntimeSMAPIMetadata, error)
	RuntimeReadJunimo140ModHashes(context.Context, string, string, string) (map[string]string, error)
	RuntimeSnapshotJunimo140Services(context.Context, string, []paneldocker.RuntimeJunimo140WorldScope, []string) ([]paneldocker.RuntimeJunimo140ServiceSnapshot, error)
	RuntimeAssertJunimo140GameGroup(context.Context, string, string, []string, bool) error
	RuntimeImageInspect(context.Context, string, string) (paneldocker.RuntimeImageMetadata, error)
	RuntimeServerHealth(context.Context, string, string) error
	RuntimeSteamAuthHealth(context.Context, string, string) (paneldocker.RuntimeAuthServiceHealth, error)
}

type currentGameEvidenceReader struct {
	docker      currentFullStackDocker
	projects    []string
	coreProfile string
}

const (
	galaxyLibrary     = "libGalaxy64.so"
	galaxyGlueLibrary = "libGalaxyCSharpGlue.so"
	// These exact installed .140 bytes differ from the pinned depot pair only
	// in the cleared execute bit on the unique ELF PT_GNU_STACK header.
	galaxyNoExecSHA256     = "b4b885643bbe50159e051413f3b45cade9e9741116a409c3f2e0d79cb4e79e0a"
	galaxyGlueNoExecSHA256 = "d6588b76a63f77d5c022ac805d1b6abd175da98e99025bad210df4681ea30fc4"
)

func (r *currentGameEvidenceReader) RuntimeReadJunimo140GameEvidence(ctx context.Context, dir, volume, imageRef, imageID, kind string) ([]byte, error) {
	return r.docker.RuntimeReadCurrentJunimo140GameEvidence(ctx, dir, volume, imageRef, imageID, kind, r.projects)
}
func (r *currentGameEvidenceReader) RuntimeVerifyJunimo140GameFiles(ctx context.Context, dir, volume, imageRef, imageID, algorithm string, files map[string]string) (paneldocker.RuntimeGameFileStats, error) {
	if algorithm == "sha256" {
		evidence, err := r.RuntimeReadJunimo140GameEvidence(ctx, dir, volume, imageRef, imageID, paneldocker.RuntimeCurrentJunimo140GalaxyPairEvidence)
		if err != nil {
			return paneldocker.RuntimeGameFileStats{}, fmt.Errorf("read current Galaxy pair: %w", err)
		}
		pair := strings.Fields(string(evidence))
		legacyGalaxy, hasGalaxy := files[galaxyLibrary]
		legacyGlue, hasGlue := files[galaxyGlueLibrary]
		if len(pair) != 2 || !hasGalaxy || !hasGlue {
			return paneldocker.RuntimeGameFileStats{}, errors.New("incomplete current Galaxy pair")
		}
		switch {
		case pair[0] == legacyGalaxy && pair[1] == legacyGlue:
			r.coreProfile = "legacy-depot"
		case pair[0] == galaxyNoExecSHA256 && pair[1] == galaxyGlueNoExecSHA256:
			r.coreProfile = "fixed140-galaxy-gnu-stack-noexec"
			files = maps.Clone(files)
			files[galaxyLibrary], files[galaxyGlueLibrary] = galaxyNoExecSHA256, galaxyGlueNoExecSHA256
		default:
			return paneldocker.RuntimeGameFileStats{}, errors.New("unsupported current Galaxy pair")
		}
		// The complete verifier rechecks both selected hashes along with every
		// other core file, so a change after the profile read cannot pass.
	}
	return r.docker.RuntimeVerifyCurrentJunimo140GameFiles(ctx, dir, volume, imageRef, imageID, algorithm, files, r.projects)
}
func (r *currentGameEvidenceReader) RuntimeInspectJunimo140SDK(ctx context.Context, dir, volume, imageRef, imageID string) (paneldocker.RuntimeGameFileStats, error) {
	return r.docker.RuntimeInspectCurrentJunimo140SDK(ctx, dir, volume, imageRef, imageID, r.projects)
}

type currentRuntimeMember struct {
	ID, Root, Project, Volume string
	Files                     map[string]string
	Invite                    bool
}

func currentRuntimeHash(value any) string {
	raw, _ := json.Marshal(value)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func (d *Driver) currentRuntimeScopeIdle() bool {
	op, err := d.readFullStackOperation()
	return errors.Is(err, os.ErrNotExist) || err == nil && fullStackTerminal(op.Public.Phase) && op.Public.Phase != "manual_action"
}

func (d *Driver) latestFailedRuntimeContinuation(panel updater.PanelTarget) (string, time.Time) {
	op, err := d.readFullStackOperation()
	if err == nil && op.Public.Continuation && op.Public.Phase == "failed_safe" && op.Public.AuthorizedAt != nil &&
		op.Public.ErrorCode != "administrator_cancelled" && op.Public.Panel.SameContent(panel) {
		return op.Public.OperationID, *op.Public.AuthorizedAt
	}
	return "", time.Time{}
}

// Local fingerprints are cheap to re-read for a completed receipt. The full
// game/SDK helpers run only in the bounded background coordinator, never GET.
func (d *Driver) currentRuntimeMembers(ctx context.Context) ([]storage.Instance, []currentRuntimeMember, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	lister, ok := d.store.(junimo140InstanceLister)
	if !ok || d.jobs == nil || !d.currentRuntimeScopeIdle() {
		return nil, nil, errors.New("current runtime inspection has an unresolved owner")
	}
	instances, err := lister.ListInstances(ctx)
	if err != nil {
		return nil, nil, err
	}
	var members []currentRuntimeMember
	for _, stored := range instances {
		if stored.DriverID != DriverID || !runtimeContentInstalled(stored.State) {
			continue
		}
		if stored.State != storage.InstanceStateRunning && stored.State != storage.InstanceStateStopped {
			return nil, nil, errors.New("installed runtime state is not settled")
		}
		instance := registry.Instance{ID: stored.ID, DriverID: stored.DriverID, DataDir: stored.DataDir, State: stored.State}
		if err := plainJunimoGroupRoot(instance.DataDir); err != nil {
			return nil, nil, err
		}
		// Do not use EffectiveRuntimeInspection: a rollback exception is not
		// positive evidence of matching the present built-in recommendation.
		if InspectManagedRuntimeStack(instance.DataDir, instance.State).Status != sjconfig.RuntimeStackStatusUpToDate {
			return nil, nil, errors.New("installed runtime needs explicit scope confirmation")
		}
		if err := rejectJunimo140GroupLease(instance.DataDir); err != nil {
			return nil, nil, err
		}
		if err := rejectUnfinishedNewGameOwner(instance.DataDir); err != nil {
			return nil, nil, err
		}
		if err := d.RejectInstanceDeletion(ctx, instance.ID); err != nil {
			return nil, nil, err
		}
		if err := d.rejectActiveSaveImport(ctx, instance.ID, ""); err != nil {
			return nil, nil, err
		}
		active, err := d.jobs.Active(ctx, storage.ListActiveJobsFilter{TargetType: "instance", TargetID: instance.ID})
		if err != nil || len(active) != 0 {
			return nil, nil, errors.New("installed runtime has an active job")
		}
		status, err := readRuntimeUpdateApplyStatus(instance.DataDir)
		if !errors.Is(err, os.ErrNotExist) && (err != nil || status.Phase != RuntimeUpdateApplySucceeded && status.Phase != RuntimeUpdateApplyFailedRolledBack) {
			return nil, nil, errors.New("installed runtime has pending or opaque recovery")
		}
		volume, err := GameDataVolumeName(instance.DataDir)
		if err != nil {
			return nil, nil, err
		}
		project := strings.ToLower(filepath.Base(instance.DataDir))
		member := currentRuntimeMember{ID: stored.ID, Root: stored.DataDir, Project: project, Volume: volume,
			Invite: sjconfig.SteamInviteEnabled(stored.DataDir), Files: map[string]string{}}
		for file, path := range map[string]string{
			"env": filepath.Join(instance.DataDir, ".env"), "compose": filepath.Join(instance.DataDir, "docker-compose.yml"),
			"controlManifest": filepath.Join(smapiModDir(instance.DataDir), "manifest.json"),
			"controlDLL":      filepath.Join(smapiModDir(instance.DataDir), "StardewAnxiPanel.Control.dll"),
			"junimoManifest":  filepath.Join(junimoServerModDir(instance.DataDir), "manifest.json"),
			"junimoDLL":       filepath.Join(junimoServerModDir(instance.DataDir), "JunimoServer.dll")} {
			info, err := os.Lstat(path)
			if err != nil || !info.Mode().IsRegular() {
				return nil, nil, errors.New("runtime file identity unavailable")
			}
			hash, err := runtimeRecoveryFileSHA256(path)
			if err != nil {
				return nil, nil, err
			}
			member.Files[file] = hash
		}
		members = append(members, member)
	}
	sort.Slice(members, func(i, j int) bool { return members[i].ID < members[j].ID })
	return instances, members, nil
}

// HasCurrentRuntimeNoop validates a historical completion receipt for this
// apply, not future authorization. New owners/configuration invalidate its UI
// use immediately without polling thousands of game files or starting helpers.
func (d *Driver) HasCurrentRuntimeNoop(ctx context.Context, status updater.ApplyStatus) bool {
	full := status.FullStack
	if full == nil || full.Phase != "not_needed" || full.RuntimeRequired || full.CurrentRuntimeVerified == nil {
		return false
	}
	proof := full.CurrentRuntimeVerified
	trusted := d.currentRuntimeVerified.Load()
	if trusted == nil || trusted.UpdateID != proof.UpdateID || trusted.ScopeSHA256 != proof.ScopeSHA256 ||
		trusted.LocalSHA256 != proof.LocalSHA256 || !trusted.Panel.SameContent(proof.Panel) || !trusted.VerifiedAt.Equal(proof.VerifiedAt) {
		return false
	}
	identity := d.panelIdentity.Load()
	if identity == nil || !identity.SameContent(proof.Panel) || status.Phase != updater.PhaseSucceeded ||
		status.UpdateID != proof.UpdateID || status.ToVersion != proof.Panel.Version || status.SelectedDigest != proof.Panel.ImageID {
		return false
	}
	if _, authorizedAt := d.latestFailedRuntimeContinuation(proof.Panel); !proof.VerifiedAt.After(authorizedAt) {
		return false
	}
	_, members, err := d.currentRuntimeMembers(ctx)
	return err == nil && len(members) > 0 && currentRuntimeHash(members) == proof.LocalSHA256
}

func (d *Driver) inspectCurrentFullStack(ctx context.Context, panel updater.PanelTarget, status updater.ApplyStatus) (updater.FullStackStatus, error) {
	var full updater.FullStackStatus
	docker, ok := d.docker.(currentFullStackDocker)
	if !ok {
		return full, errors.New("read-only current runtime inspection unavailable")
	}
	instances, members, err := d.currentRuntimeMembers(ctx)
	if err != nil || len(members) == 0 {
		return full, errors.New("complete installed runtime scope unavailable")
	}
	before := currentRuntimeHash(members)
	target, err := fixedFullStackJunimo140Target()
	if err != nil {
		return full, err
	}
	seen := map[string]bool{}
	var observations []string
	type checkedGroup struct {
		plan      junimo140GroupPlan
		scopes    []paneldocker.RuntimeJunimo140WorldScope
		snapshots []paneldocker.RuntimeJunimo140ServiceSnapshot
	}
	var checkedGroups []checkedGroup
	for _, member := range members {
		if seen[member.ID] {
			continue
		}
		plan, err := buildJunimo140GroupPlan(instances, registry.Instance{ID: member.ID, DriverID: DriverID, DataDir: member.Root}, newRuntimeApplyID())
		if err != nil {
			return full, err
		}
		for _, peer := range plan.Members {
			seen[peer.ID] = true
		}
		scopes, err := junimo140WorldScopes(plan, d.dockerHostPath)
		if err != nil {
			return full, err
		}
		if err := docker.RuntimeAssertJunimo140GameGroup(ctx, member.Root, plan.SourceVolume, junimo140Projects(plan), false); err != nil {
			return full, err
		}
		first, err := docker.RuntimeSnapshotJunimo140Services(ctx, member.Root, scopes, []string{plan.SourceVolume})
		if err != nil {
			return full, err
		}
		var image RuntimeUpdateSelectedImage
		for _, peer := range plan.Members {
			invite := sjconfig.SteamInviteEnabled(peer.DataDir)
			serverCount, authCount := 0, 0
			for _, snapshot := range first {
				if snapshot.Project != peer.Project {
					continue
				}
				if snapshot.GameVolume != plan.SourceVolume || snapshot.ContainerID == "" {
					return full, errors.New("current service volume identity differs")
				}
				if snapshot.Running && snapshot.State != "running" || !snapshot.Running && snapshot.State != "exited" && snapshot.State != "created" {
					return full, errors.New("current service state is not settled")
				}
				metadata, err := docker.RuntimeImageInspect(ctx, peer.DataDir, snapshot.ImageID)
				if err != nil || metadata.ID != snapshot.ImageID {
					return full, errors.New("current image identity unavailable")
				}
				if snapshot.Service == "server" {
					serverCount++
					if metadata.Digest != junimo140Digest {
						return full, errors.New("current server is not fixed140")
					}
					image = RuntimeUpdateSelectedImage{Image: snapshot.Image, ImageID: metadata.ID, Digest: metadata.Digest}
					if snapshot.Running && (docker.RuntimeServerHealth(ctx, peer.DataDir, peer.Project) != nil || !runningControlMatchesManifest(peer.DataDir) || !commandResultSupported(peer.DataDir) || readSMAPIStatus(peer.DataDir) != "save-loaded" && readSMAPIStatus(peer.DataDir) != "launched") {
						return full, errors.New("running server or Control is not ready")
					}
				} else if snapshot.Service == "steam-auth" {
					authCount++
					matched := false
					for _, digest := range target.SteamAuth.Digests {
						matched = matched || metadata.Digest == digest
					}
					if !invite || !matched || snapshot.SessionVolume == "" {
						return full, errors.New("current authentication identity differs")
					}
					if snapshot.Running {
						if _, err := docker.RuntimeSteamAuthHealth(ctx, peer.DataDir, peer.Project); err != nil {
							return full, err
						}
					}
				} else {
					return full, errors.New("opaque current service")
				}
			}
			if serverCount != 1 || invite && authCount != 1 || !invite && authCount != 0 {
				return full, errors.New("current group service missing or ambiguous")
			}
			modHashes, err := docker.RuntimeReadJunimo140ModHashes(ctx, peer.DataDir, image.Image, image.ImageID)
			if err != nil {
				return full, err
			}
			for _, name := range []string{"JunimoServer.dll", "manifest.json"} {
				actual, err := runtimeRecoveryFileSHA256(filepath.Join(junimoServerModDir(peer.DataDir), name))
				if err != nil || modHashes[name] != actual {
					return full, errors.New("current Junimo payload differs from fixed image")
				}
			}
			full.Instances = append(full.Instances, updater.FullStackInstanceStatus{InstanceID: peer.ID, Phase: "not_needed", Progress: 100})
		}
		var payload junimo140VerifiedGamePayload
		reader := &currentGameEvidenceReader{docker: docker, projects: junimo140Projects(plan)}
		if err := verifyJunimo140GamePayload(ctx, reader, member.Root, plan.SourceVolume, image, time.Now(), &payload); err != nil {
			return full, err
		}
		smapi, err := docker.RuntimeReadCurrentPinnedSMAPIMetadata(ctx, member.Root, plan.SourceVolume, image.Image, image.ImageID, junimo140Projects(plan))
		if err != nil || !junimo140SMAPIIdentityVerified(smapi) {
			return full, errors.New("current patched SMAPI identity unavailable")
		}
		last, err := docker.RuntimeSnapshotJunimo140Services(ctx, member.Root, scopes, []string{plan.SourceVolume})
		if err != nil || !reflect.DeepEqual(first, last) {
			return full, errors.New("current services changed during inspection")
		}
		observations = append(observations, currentRuntimeHash([]any{plan.membershipDigest(), first, payload, smapi, reader.coreProfile}))
		checkedGroups = append(checkedGroups, checkedGroup{plan: plan, scopes: scopes, snapshots: first})
	}
	for _, group := range checkedGroups {
		primary, _ := group.plan.primary()
		if err := docker.RuntimeAssertJunimo140GameGroup(ctx, primary.DataDir, group.plan.SourceVolume, junimo140Projects(group.plan), false); err != nil {
			return full, err
		}
		latest, err := docker.RuntimeSnapshotJunimo140Services(ctx, primary.DataDir, group.scopes, []string{group.plan.SourceVolume})
		if err != nil || !reflect.DeepEqual(latest, group.snapshots) {
			return full, errors.New("an earlier group changed before inspection completed")
		}
	}
	_, after, err := d.currentRuntimeMembers(ctx)
	if err != nil || before != currentRuntimeHash(after) || len(seen) != len(members) {
		return full, errors.New("installed scope changed during inspection")
	}
	full.Phase, full.Progress, full.RuntimeRequired = "not_needed", 100, false
	full.Result = "Panel 已更新；全部现有游戏组已通过只读完整核验，无需重复保存、停服或迁移。"
	full.CurrentRuntimeVerified = &updater.RuntimeNoopProof{UpdateID: status.UpdateID, Panel: panel, LocalSHA256: before, ScopeSHA256: currentRuntimeHash(observations), VerifiedAt: time.Now().UTC()}
	return full, nil
}

type currentRuntimeNoopAttempts struct {
	key   string
	count int
	next  time.Time
}

// Three bounded background attempts per apply/local input identity. Failed or
// unknown evidence remains the ordinary manual confirmation path, never success.
func (d *Driver) reconcileCurrentRuntimeNoop(ctx context.Context, panel FullStackPanelCoordinator, attempts *currentRuntimeNoopAttempts) {
	recorder, ok := panel.(interface {
		RecordRuntimeNoop(context.Context, updater.ApplyStatus, updater.PanelTarget, updater.FullStackStatus) (bool, error)
	})
	if !ok {
		return
	}
	status, err := panel.ApplyStatus()
	identity := d.panelIdentity.Load()
	if err != nil || identity == nil || status.Phase != updater.PhaseSucceeded || !status.CleanupCompleted || status.OperationID != "" || status.ToVersion != identity.Version || status.SelectedDigest != identity.ImageID || d.HasCurrentRuntimeNoop(ctx, status) {
		return
	}
	_, members, err := d.currentRuntimeMembers(ctx)
	if err != nil {
		return
	}
	failedOperation, _ := d.latestFailedRuntimeContinuation(*identity)
	key := status.UpdateID + "/" + identity.ImageID + "/" + currentRuntimeHash(members) + "/" + failedOperation
	if attempts.key != key {
		*attempts = currentRuntimeNoopAttempts{key: key}
	}
	if attempts.count >= 3 || time.Now().Before(attempts.next) {
		return
	}
	attempts.count++
	attempts.next = time.Now().Add(30 * time.Second)
	bounded, cancel := context.WithTimeout(ctx, 2*time.Minute)
	d.fullStackMu.Lock()
	if !d.currentRuntimeScopeIdle() || d.currentRuntimeReadDone != nil {
		d.fullStackMu.Unlock()
		cancel()
		return
	}
	done := make(chan struct{})
	d.currentRuntimeReadCancel, d.currentRuntimeReadDone = cancel, done
	d.fullStackMu.Unlock()
	defer func() {
		cancel()
		d.fullStackMu.Lock()
		d.currentRuntimeReadCancel, d.currentRuntimeReadDone = nil, nil
		close(done)
		d.fullStackMu.Unlock()
	}()
	full, err := d.inspectCurrentFullStack(bounded, *identity, status)
	if err != nil {
		d.logger.Warn("read-only current runtime inspection did not prove no-op", "causeSHA256", currentRuntimeHash(err.Error()))
		return
	}
	d.fullStackMu.Lock()
	defer d.fullStackMu.Unlock()
	if !d.currentRuntimeScopeIdle() {
		return
	}
	written, err := recorder.RecordRuntimeNoop(bounded, status, *identity, full)
	if err == nil && written {
		proof := *full.CurrentRuntimeVerified
		d.currentRuntimeVerified.Store(&proof)
	}
}
