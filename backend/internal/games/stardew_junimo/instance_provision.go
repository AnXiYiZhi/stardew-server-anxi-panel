package stardew_junimo

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

var (
	ErrInstanceProvisionTemplateRequired  = errors.New("an installed game template is required")
	ErrInstanceProvisionTemplateBusy      = errors.New("game template has an active task")
	ErrInstanceProvisionDockerUnsupported = errors.New("instance game-data provisioning is unavailable")
)

type instanceGameDataProvisionDocker interface {
	ProvisionInstanceGameData(ctx context.Context, dir, project, source, target, trustedImage, token string) error
	CleanupInstanceGameData(ctx context.Context, dir, project, token string) error
}

type provisionPorts struct {
	game  int
	query int
	vnc   int
	api   int
}

func (d *Driver) ProvisionInstance(ctx context.Context, req registry.InstanceProvisionRequest) (registry.InstanceProvisionResult, error) {
	d.runtimeUpdateMu.Lock()
	defer d.runtimeUpdateMu.Unlock()
	if req.Template.ID == "" || req.Target.ID == "" || req.Template.ID == req.Target.ID ||
		req.Template.DriverID != DriverID || req.Target.DriverID != DriverID {
		return registry.InstanceProvisionResult{}, ErrInstanceProvisionTemplateRequired
	}
	if d.jobs == nil {
		return registry.InstanceProvisionResult{}, ErrInstanceProvisionDockerUnsupported
	}
	active, err := d.jobs.Active(ctx, storage.ListActiveJobsFilter{TargetType: "instance", TargetID: req.Template.ID})
	if err != nil {
		return registry.InstanceProvisionResult{}, fmt.Errorf("inspect game template tasks: %w", err)
	}
	if len(active) != 0 {
		return registry.InstanceProvisionResult{}, ErrInstanceProvisionTemplateBusy
	}
	if err := rejectUnfinishedNewGameOwner(req.Template.DataDir); err != nil {
		return registry.InstanceProvisionResult{}, err
	}
	if busy, err := HasUnfinishedImportTransaction(req.Template.DataDir); err != nil || busy {
		return registry.InstanceProvisionResult{}, ErrInstanceProvisionTemplateBusy
	}
	if status, err := readRuntimeUpdateApplyStatus(req.Template.DataDir); err != nil && !os.IsNotExist(err) {
		return registry.InstanceProvisionResult{}, ErrInstanceProvisionTemplateBusy
	} else if err == nil && status.Phase != "idle" && status.Phase != RuntimeUpdateApplySucceeded && status.Phase != RuntimeUpdateApplyFailedRolledBack {
		return registry.InstanceProvisionResult{}, ErrInstanceProvisionTemplateBusy
	}
	if status, err := readSMAPIUpdateStatus(req.Template.DataDir, "apply-status.json"); err != nil && !os.IsNotExist(err) {
		return registry.InstanceProvisionResult{}, ErrInstanceProvisionTemplateBusy
	} else if err == nil && status.Phase != SMAPIApplyIdle && status.Phase != SMAPIApplySucceeded && status.Phase != SMAPIApplyFailedRolledBack {
		return registry.InstanceProvisionResult{}, ErrInstanceProvisionTemplateBusy
	}
	store, ok := d.store.(instanceProvisionStore)
	if !ok {
		return registry.InstanceProvisionResult{}, ErrInstanceProvisionDockerUnsupported
	}
	if _, err := d.provisionTarget(req.Target); err != nil {
		return registry.InstanceProvisionResult{}, err
	}
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		return registry.InstanceProvisionResult{}, err
	}
	plan := storage.InstanceProvision{InstanceID: req.Target.ID, TemplateID: req.Template.ID, Token: hex.EncodeToString(tokenBytes)}
	if err := store.BeginInstanceProvision(ctx, plan); err != nil {
		return registry.InstanceProvisionResult{}, ErrInstanceProvisionTemplateBusy
	}
	ctx = context.WithValue(ctx, provisionContextKey{}, plan.Token)

	templateImage := gameInstallImage(req.Template.DataDir)
	templateReady, err := d.verifyGameDataVolume(ctx, req.Template.DataDir, templateImage, nil)
	if err != nil {
		return registry.InstanceProvisionResult{}, fmt.Errorf("verify game installation template: %w", err)
	}
	if !templateReady {
		return registry.InstanceProvisionResult{}, ErrInstanceProvisionTemplateRequired
	}

	ports, err := allocateProvisionPorts(req.Existing, req.Target.ID)
	if err != nil {
		return registry.InstanceProvisionResult{}, err
	}
	if err := createProvisionDirectory(req.Target.DataDir, plan.Token); err != nil {
		return registry.InstanceProvisionResult{}, err
	}
	if err := d.prepareLocked(ctx, req.Target); err != nil {
		return registry.InstanceProvisionResult{}, fmt.Errorf("prepare target instance: %w", err)
	}
	if err := sjconfig.UpdateEnvFile(filepath.Join(req.Target.DataDir, ".env"), map[string]string{
		"GAME_PORT":  strconv.Itoa(ports.game),
		"QUERY_PORT": strconv.Itoa(ports.query),
		"VNC_PORT":   strconv.Itoa(ports.vnc),
		"API_PORT":   strconv.Itoa(ports.api),
	}); err != nil {
		return registry.InstanceProvisionResult{}, fmt.Errorf("assign target instance ports: %w", err)
	}
	installEnv, err := installationTemplateEnv(req.Template.DataDir, templateImage)
	if err != nil {
		return registry.InstanceProvisionResult{}, fmt.Errorf("read game installation template configuration: %w", err)
	}
	if err := sjconfig.UpdateEnvFile(filepath.Join(req.Target.DataDir, ".env"), installEnv); err != nil {
		return registry.InstanceProvisionResult{}, fmt.Errorf("inherit game installation template configuration: %w", err)
	}

	project := strings.ToLower(filepath.Base(filepath.Clean(req.Target.DataDir)))
	if project != req.Target.ID || !runtimeComposeProjectPattern.MatchString(project) {
		return registry.InstanceProvisionResult{}, errors.New("target instance project is invalid")
	}
	templateVolume, err := GameDataVolumeName(req.Template.DataDir)
	if err != nil {
		return registry.InstanceProvisionResult{}, fmt.Errorf("resolve game template volume: %w", err)
	}
	targetVolume, err := GameDataVolumeName(req.Target.DataDir)
	if err != nil {
		return registry.InstanceProvisionResult{}, fmt.Errorf("resolve target game-data volume: %w", err)
	}
	cloner, ok := d.docker.(instanceGameDataProvisionDocker)
	if !ok {
		return registry.InstanceProvisionResult{}, ErrInstanceProvisionDockerUnsupported
	}
	if err := cloner.ProvisionInstanceGameData(ctx, req.Target.DataDir, project, templateVolume, targetVolume, templateImage, plan.Token); err != nil {
		return registry.InstanceProvisionResult{}, fmt.Errorf("copy installed game runtime: %w", err)
	}
	targetReady, err := d.verifyGameDataVolume(ctx, req.Target.DataDir, templateImage, nil)
	if err != nil {
		return registry.InstanceProvisionResult{}, fmt.Errorf("verify copied game runtime: %w", err)
	}
	if !targetReady {
		return registry.InstanceProvisionResult{}, errors.New("copied game runtime is incomplete")
	}
	d.rememberInstallationEvidence(req.Target.ID, "ok")
	stage := filepath.Join(filepath.Dir(req.Target.DataDir), ".provision-"+plan.Token)
	if err := os.Remove(stage); err != nil && !os.IsNotExist(err) {
		return registry.InstanceProvisionResult{}, err
	}
	if err := store.FinishInstanceProvision(ctx, plan, true); err != nil {
		return registry.InstanceProvisionResult{}, fmt.Errorf("publish target instance: %w", err)
	}

	return registry.InstanceProvisionResult{
		GamePort: ports.game, QueryPort: ports.query, VNCPort: ports.vnc, APIPort: ports.api, Protocol: "udp",
	}, nil
}

// installationTemplateEnv carries only the installed game runtime identity to
// a new instance. This is intentionally narrower than copying the template's
// whole .env: Steam invite credentials, passwords and instance settings remain
// instance-owned, while legacy mirror/custom registry image references keep
// working after an in-place Panel upgrade.
func installationTemplateEnv(dataDir, verifiedImage string) (map[string]string, error) {
	values, err := sjconfig.ReadEnvFile(filepath.Join(dataDir, ".env"))
	if err != nil {
		return nil, err
	}
	updates := map[string]string{"SERVER_IMAGE": strings.TrimSpace(verifiedImage)}
	for _, key := range []string{"IMAGE_VERSION", "SERVER_IMAGE_CANDIDATES", "SMAPI_VERSION", "SMAPI_DOWNLOAD_URLS"} {
		if value := strings.TrimSpace(values[key]); value != "" {
			updates[key] = value
		}
	}
	if tag := imageReferenceTag(verifiedImage); tag != "" {
		updates["IMAGE_VERSION"] = tag
	}
	if strings.TrimSpace(updates["SERVER_IMAGE_CANDIDATES"]) == "" {
		updates["SERVER_IMAGE_CANDIDATES"] = strings.TrimSpace(verifiedImage)
	}
	return updates, nil
}

// ConvergeProvisionedInstanceTemplate repairs the narrow compatibility window
// where an early multi-instance build cloned valid game-data but left the new
// instance on the current default SERVER_IMAGE instead of the legacy template's
// locally available mirror. It never downloads or rewrites game-data and only
// touches untouched save-required instances published by this provisioner.
func (d *Driver) ConvergeProvisionedInstanceTemplate(ctx context.Context, template, target registry.Instance) (bool, error) {
	if d.docker == nil || template.ID == "" || target.ID == "" || template.ID == target.ID ||
		template.DriverID != DriverID || target.DriverID != DriverID ||
		target.State != storage.InstanceStateSaveRequired || target.DriverPhase != "instance_ready" {
		return false, nil
	}
	root := filepath.Clean(filepath.Join(d.containerDataDir, "instances"))
	targetDir := filepath.Clean(target.DataDir)
	rel, err := filepath.Rel(root, targetDir)
	if err != nil || rel != target.ID || filepath.IsAbs(rel) || strings.Contains(rel, string(filepath.Separator)) {
		return false, errors.New("provisioned instance directory is outside the managed root")
	}
	targetVolume, err := GameDataVolumeName(targetDir)
	if err != nil {
		return false, err
	}
	if targetVolume != strings.ToLower(target.ID)+"_game-data" {
		return false, nil
	}

	templateImage := gameInstallImage(template.DataDir)
	targetImage := gameInstallImage(targetDir)
	if templateImage == targetImage || imageReferenceTag(templateImage) == "" || imageReferenceTag(templateImage) != imageReferenceTag(targetImage) {
		return false, nil
	}
	inspect, inspectErr := d.docker.ImageInspect(ctx, targetDir, targetImage)
	if inspectErr == nil {
		return false, nil
	}
	if !explicitImageMissing(inspect.Stdout + "\n" + inspect.Stderr + "\n" + inspectErr.Error()) {
		return false, fmt.Errorf("inspect provisioned instance image: %w", inspectErr)
	}
	if _, err := d.docker.ImageInspect(ctx, template.DataDir, templateImage); err != nil {
		return false, fmt.Errorf("inspect installed template image: %w", err)
	}
	ready, err := d.verifyGameDataVolume(ctx, targetDir, templateImage, nil)
	if err != nil {
		return false, fmt.Errorf("verify provisioned instance game-data: %w", err)
	}
	if !ready {
		return false, nil
	}
	updates, err := installationTemplateEnv(template.DataDir, templateImage)
	if err != nil {
		return false, err
	}
	if err := sjconfig.UpdateEnvFile(filepath.Join(targetDir, ".env"), updates); err != nil {
		return false, err
	}
	d.rememberInstallationEvidence(target.ID, "ok")
	return true, nil
}

func allocateProvisionPorts(existing []registry.Instance, targetID string) (provisionPorts, error) {
	usedUDP := map[int]struct{}{}
	usedTCP := map[int]struct{}{}
	for _, instance := range existing {
		if instance.ID == targetID || instance.DriverID != DriverID {
			continue
		}
		values, err := sjconfig.ReadEnvFile(filepath.Join(instance.DataDir, ".env"))
		if err != nil && !os.IsNotExist(err) {
			return provisionPorts{}, fmt.Errorf("read ports for instance %s: %w", instance.ID, err)
		}
		for _, item := range []struct {
			key      string
			fallback int
			used     map[int]struct{}
		}{
			{key: "GAME_PORT", fallback: 24642, used: usedUDP},
			{key: "QUERY_PORT", fallback: 27015, used: usedUDP},
			{key: "VNC_PORT", fallback: 5800, used: usedTCP},
			{key: "API_PORT", fallback: 8080, used: usedTCP},
		} {
			value := strings.TrimSpace(values[item.key])
			port := item.fallback
			if value != "" {
				parsed, parseErr := strconv.Atoi(value)
				if parseErr != nil || parsed < 1 || parsed > 65535 {
					return provisionPorts{}, fmt.Errorf("instance %s has invalid %s", instance.ID, item.key)
				}
				port = parsed
			}
			item.used[port] = struct{}{}
		}
	}

	game, err := firstFreeProvisionPort(24642, usedUDP)
	if err != nil {
		return provisionPorts{}, err
	}
	usedUDP[game] = struct{}{}
	query, err := firstFreeProvisionPort(27015, usedUDP)
	if err != nil {
		return provisionPorts{}, err
	}
	vnc, err := firstFreeProvisionPort(5800, usedTCP)
	if err != nil {
		return provisionPorts{}, err
	}
	usedTCP[vnc] = struct{}{}
	api, err := firstFreeProvisionPort(8080, usedTCP)
	if err != nil {
		return provisionPorts{}, err
	}
	return provisionPorts{game: game, query: query, vnc: vnc, api: api}, nil
}

func firstFreeProvisionPort(start int, used map[int]struct{}) (int, error) {
	for port := start; port <= 65535; port++ {
		if _, exists := used[port]; !exists {
			return port, nil
		}
	}
	return 0, errors.New("no free instance port is available")
}

func (d *Driver) CleanupProvisionedInstance(ctx context.Context, instance registry.Instance) error {
	d.runtimeUpdateMu.Lock()
	defer d.runtimeUpdateMu.Unlock()
	return d.cleanupProvisionLocked(ctx, instance)
}
