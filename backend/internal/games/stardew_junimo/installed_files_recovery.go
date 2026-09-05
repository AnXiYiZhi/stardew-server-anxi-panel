package stardew_junimo

import (
	"context"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func recoverableInstalledFilesError(instance storage.Instance) bool {
	return instance.State == storage.InstanceStateError &&
		(instance.DriverPhase == "install_verification_failed" || instance.DriverPhase == "steamcmd_failed")
}

// Recheck restored files without installing, authorizing, or starting a world.
// The same mutation lock used when creating installation/lifecycle jobs keeps
// a new owner from acquiring this instance between verification and recovery.
func (d *Driver) reconcileRestoredGameFiles(ctx context.Context, instance storage.Instance) (storage.Instance, error) {
	if d.docker == nil || !d.runtimeUpdateMu.TryLock() {
		return instance, nil
	}
	defer d.runtimeUpdateMu.Unlock()
	current, err := d.store.GetInstance(ctx, instance.ID)
	if err != nil {
		return instance, err
	}
	if !recoverableInstalledFilesError(current) || unfinishedNewGameOwnerExists(current.DataDir) {
		return current, nil
	}
	if d.jobs != nil {
		active, err := d.jobs.Active(ctx, storage.ListActiveJobsFilter{TargetType: "instance", TargetID: current.ID})
		if err != nil || len(active) != 0 {
			return current, nil
		}
	}
	ps, err := d.docker.ComposePsStrict(ctx, current.DataDir)
	if err != nil || serverServiceUp(ps.Services) {
		return current, nil
	}
	// Failed probes are bounded just like ordinary installed-file polling.
	if evidence, cached := d.cachedInstallationEvidence(current.ID); cached && evidence.State == "missing" {
		return current, nil
	}
	imageRef := gameInstallImage(current.DataDir)
	if _, err := d.docker.ImageInspect(ctx, current.DataDir, imageRef); err != nil {
		return current, nil
	}
	ok, err := d.verifyGameDataVolume(ctx, current.DataDir, imageRef, nil)
	if err != nil {
		return current, nil
	}
	if !ok {
		d.rememberInstallationEvidence(current.ID, "missing")
		return current, nil
	}
	latest, err := d.store.GetInstance(ctx, current.ID)
	if err != nil {
		return current, err
	}
	if latest.State != current.State || latest.DriverPhase != current.DriverPhase || latest.UpdatedAt != current.UpdatedAt {
		return latest, nil
	}
	d.rememberInstallationEvidence(current.ID, "ok")
	return d.store.UpdateInstanceState(ctx, storage.UpdateInstanceStateParams{
		ID: current.ID, State: storage.InstanceStateStopped,
		StateMessage: "游戏运行文件已验证完整，服务器已停止，可重新启动。",
		DriverPhase:  "game_files_restored", DriverPayload: current.DriverPayload,
	})
}
