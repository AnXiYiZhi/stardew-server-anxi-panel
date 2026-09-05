package stardew_junimo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

type instanceProvisionStore interface {
	GetInstanceProvision(context.Context, string) (storage.InstanceProvision, error)
	BeginInstanceProvision(context.Context, storage.InstanceProvision) error
	FinishInstanceProvision(context.Context, storage.InstanceProvision, bool) error
	DeleteInstanceIfPhase(context.Context, string, string) (bool, error)
}

var provisionTokenPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

type provisionContextKey struct{}

func (d *Driver) provisionTarget(instance registry.Instance) (string, error) {
	root, err := filepath.Abs(filepath.Join(d.containerDataDir, "instances"))
	if err != nil {
		return "", err
	}
	target, err := filepath.Abs(instance.DataDir)
	if err != nil {
		return "", err
	}
	if !runtimeComposeProjectPattern.MatchString(instance.ID) || target != filepath.Join(root, instance.ID) {
		return "", errors.New("unmanaged provision target")
	}
	for p := target; ; p = filepath.Dir(p) {
		if _, err := os.Lstat(p); err == nil {
			resolved, e := filepath.EvalSymlinks(p)
			if e != nil {
				return "", e
			}
			if resolved != p {
				return "", errors.New("provision path alias")
			}
		} else if !os.IsNotExist(err) {
			return "", err
		}
		if p == filepath.Dir(p) {
			break
		}
	}
	return target, nil
}

func createProvisionDirectory(target, token string) error {
	stage := filepath.Join(filepath.Dir(target), ".provision-"+token)
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	if err := os.Mkdir(stage, 0700); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(stage, "owner.json"), []byte(token), 0600); err != nil {
		return err
	}
	// Reuse the tested cross-platform exclusive publication primitive. Its
	// Linux fallback publishes owner.json before any runtime files are written.
	return renameNewGameOwnerNoReplace(stage, target)
}

func removeProvisionDirectory(path, token string) error {
	raw, err := os.ReadFile(filepath.Join(path, "owner.json"))
	if os.IsNotExist(err) {
		if _, e := os.Lstat(path); os.IsNotExist(e) {
			return nil
		}
		// A crash between exclusive mkdir and owner publication can leave only an
		// empty directory. Remove never traverses or discards an unowned file.
		return os.Remove(path)
	}
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(raw)) != token {
		return errors.New("provision directory owner changed")
	}
	if err = validateWorldDeletionTree(path); err != nil {
		return err
	}
	return os.RemoveAll(path)
}

func (d *Driver) cleanupProvisionLocked(ctx context.Context, instance registry.Instance) error {
	target, err := d.provisionTarget(instance)
	if err != nil {
		return err
	}
	store, ok := d.store.(instanceProvisionStore)
	if !ok {
		return nil
	}
	plan, err := store.GetInstanceProvision(ctx, instance.ID)
	if errors.Is(err, storage.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if !provisionTokenPattern.MatchString(plan.Token) {
		return errors.New("invalid provision journal")
	}
	engine, ok := d.docker.(instanceGameDataProvisionDocker)
	if !ok {
		return ErrInstanceProvisionDockerUnsupported
	}
	// All paths checked before Docker cleanup, including aliases inside target.
	if err = validateWorldDeletionTree(target); err != nil {
		return err
	}
	stage := filepath.Join(filepath.Dir(target), ".provision-"+plan.Token)
	if err = validateWorldDeletionTree(stage); err != nil {
		return err
	}
	if err = engine.CleanupInstanceGameData(ctx, filepath.Dir(target), instance.ID, plan.Token); err != nil {
		return err
	}
	if err = removeProvisionDirectory(target, plan.Token); err != nil {
		return err
	}
	if err = removeProvisionDirectory(stage, plan.Token); err != nil {
		return err
	}
	return store.FinishInstanceProvision(ctx, plan, false)
}

// Called before any bootstrap Prepare/updater/scheduler. Success publication
// removed its journal atomically; only unfinished allocations reach rollback.
func (d *Driver) RecoverInstanceProvision(ctx context.Context, instance registry.Instance) (bool, error) {
	d.runtimeUpdateMu.Lock()
	defer d.runtimeUpdateMu.Unlock()
	store, ok := d.store.(instanceProvisionStore)
	if !ok {
		return false, nil
	}
	_, err := store.GetInstanceProvision(ctx, instance.ID)
	if errors.Is(err, storage.ErrNotFound) {
		current, loadErr := d.store.GetInstance(ctx, instance.ID)
		if errors.Is(loadErr, storage.ErrNotFound) {
			return false, nil
		}
		if loadErr != nil {
			return true, loadErr
		}
		instance.DriverPhase = current.DriverPhase
		if instance.DriverPhase != "instance_provisioning" {
			return false, nil
		}
		// Reservation persisted before allocation began. Legacy nonempty remnants
		// are preserved and exposed for explicit deletion rather than guessed.
		target, e := d.provisionTarget(instance)
		if e != nil {
			return true, e
		}
		if _, e = os.Lstat(target); os.IsNotExist(e) {
			_, e = store.DeleteInstanceIfPhase(ctx, instance.ID, "instance_provisioning")
			return true, e
		}
		_, e = d.store.UpdateInstanceState(ctx, storage.UpdateInstanceStateParams{ID: instance.ID, State: storage.InstanceStateError, DriverPhase: "instance_provision_cleanup_failed", StateMessage: "世界创建已中断，请重试删除以清理未完成资源。", DriverPayload: instance.DriverPayload})
		return true, e
	}
	if err != nil {
		return true, err
	}
	err = d.cleanupProvisionLocked(ctx, instance)
	if err != nil {
		_, _ = d.store.UpdateInstanceState(ctx, storage.UpdateInstanceStateParams{ID: instance.ID, State: storage.InstanceStateError, DriverPhase: "instance_provision_cleanup_failed", StateMessage: "世界创建恢复未完成，请检查资源占用后重试删除。", DriverPayload: instance.DriverPayload})
	}
	return true, err
}
