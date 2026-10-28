package stardew_junimo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

var junimo140InstallVolumePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*_anxi-smapi-update-[a-f0-9]{24}$`)

type junimo140InstallDocker interface {
	junimo140SharedGameDocker
	RuntimeImageInspect(context.Context, string, string) (paneldocker.RuntimeImageMetadata, error)
	RuntimeVolumeExists(context.Context, string, string) (bool, error)
	RuntimeCreateSMAPIStagingVolume(context.Context, string, string, string) error
	RuntimeValidateUnusedSMAPIStaging(context.Context, string, string) error
}

// The selected volume is durable before Docker creates it. A failed or
// interrupted download therefore keeps the same owned destination on retry.
// Previously installed data stays in SourceVolume until ordinary retention or
// instance deletion can account for its holders; it is never deleted here.
type junimo140InstallVolume struct {
	Schema        int    `json:"schema"`
	Project       string `json:"project"`
	SourceVolume  string `json:"sourceVolume"`
	TargetVolume  string `json:"targetVolume"`
	Image         string `json:"image"`
	ImageID       string `json:"imageId"`
	Digest        string `json:"digest"`
	SourceExisted bool   `json:"sourceExisted"`
	Created       bool   `json:"created"`
	Prepared      bool   `json:"prepared"`
	SMAPIVerified bool   `json:"smapiVerified"`
}

func validateBaseInstallRuntime(instance storage.Instance, target string) error {
	if instance.State == storage.InstanceStateRunning || instance.State == storage.InstanceStateStarting {
		return &RuntimeUpdateValidationError{Code: "server_running", Message: "请先停止服务器，再安装或修复游戏文件。"}
	}
	if target == "1.5.0-preview.140" && instance.State != "" && runtimeContentInstalled(instance.State) {
		current := InspectRuntimeStack(instance.DataDir, instance.State)
		if current.Current.Server.Tag != target {
			return &RuntimeUpdateValidationError{Code: "shared_group_confirmation_required", Message: "已有世界必须在更新中心确认精确世界范围后升级，基础安装不能替代运行栈迁移。"}
		}
	}
	return nil
}

func installVolumePlanPath(dir, volume string) string {
	return filepath.Join(dir, ".local-container", "install-game-volumes", volume+".json")
}

func readInstallVolumePlan(dir, volume string) (junimo140InstallVolume, error) {
	var plan junimo140InstallVolume
	if !junimo140InstallVolumePattern.MatchString(volume) {
		return plan, errors.New("installation volume is not transaction-owned")
	}
	raw, err := os.ReadFile(installVolumePlanPath(dir, volume))
	if err != nil {
		return plan, err
	}
	if json.Unmarshal(raw, &plan) != nil || plan.Schema != 1 || plan.TargetVolume != volume ||
		!runtimeComposeProjectPattern.MatchString(plan.Project) ||
		!strings.HasPrefix(volume, plan.Project+"_anxi-smapi-update-") ||
		!gameDataVolumeNamePattern.MatchString(plan.SourceVolume) || plan.SourceVolume == volume ||
		!runtimeImageDigestPattern.MatchString(plan.ImageID) || plan.Digest != junimo140Digest {
		return plan, errors.New("invalid image-managed installation volume journal")
	}
	return plan, nil
}

// Target volumes are discovered by their existing per-project Docker labels.
// Only same-world retained sources may widen a stopped container's old game
// mount. Foreign/shared sources remain outside the deletion authorization.
func retainedInstallGameVolumes(dir, project string) ([]string, error) {
	root := filepath.Join(dir, ".local-container", "install-game-volumes")
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	set := map[string]bool{}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() || entry.Type()&os.ModeSymlink != 0 {
			return nil, errors.New("installation volume journal is not a regular file")
		}
		volume := strings.TrimSuffix(entry.Name(), ".json")
		plan, err := readInstallVolumePlan(dir, volume)
		if err != nil || plan.Project != project {
			return nil, errors.New("installation volume journal has a foreign owner")
		}
		if plan.SourceExisted && (plan.SourceVolume == project+"_game-data" || junimo140InstallVolumePattern.MatchString(plan.SourceVolume) && strings.HasPrefix(plan.SourceVolume, project+"_anxi-smapi-update-")) {
			set[plan.SourceVolume] = true
		}
	}
	volumes := make([]string, 0, len(set))
	for volume := range set {
		volumes = append(volumes, volume)
	}
	sort.Strings(volumes)
	return volumes, nil
}

func writeInstallVolumePlan(dir string, plan junimo140InstallVolume) error {
	raw, err := json.Marshal(plan)
	if err != nil {
		return err
	}
	path := installVolumePlanPath(dir, plan.TargetVolume)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return atomicWriteRaw(path, raw, 0o600)
}

func (r *installRunner) junimo140InstallImage(ctx context.Context, imageRef string) (junimo140InstallDocker, RuntimeUpdateSelectedImage, error) {
	docker, ok := r.driver.docker.(junimo140InstallDocker)
	if !ok {
		return nil, RuntimeUpdateSelectedImage{}, errors.New("Docker driver cannot safely install image-managed Junimo .140")
	}
	image, err := docker.RuntimeImageInspect(ctx, r.instance.DataDir, imageRef)
	if err != nil || image.Digest != junimo140Digest || !runtimeImageDigestPattern.MatchString(image.ID) {
		return nil, RuntimeUpdateSelectedImage{}, errors.New("Junimo .140 installation image does not match its fixed digest")
	}
	return docker, RuntimeUpdateSelectedImage{Image: imageRef, ImageID: image.ID, Digest: image.Digest}, nil
}

// Use the existing unused, labelled SMAPI staging contract from the first
// SteamCMD download onward. This avoids installing the official same-numbered
// SMAPI over the .140 image's patched build, or mutating a shared source volume.
func (r *installRunner) prepareJunimo140InstallVolume(ctx context.Context) error {
	if r.imageTag != "1.5.0-preview.140" {
		return nil
	}
	imageRef := gameInstallImage(r.instance.DataDir)
	docker, image, err := r.junimo140InstallImage(ctx, imageRef)
	if err != nil {
		return err
	}
	volume, err := GameDataVolumeName(r.instance.DataDir)
	if err != nil {
		return err
	}
	var plan junimo140InstallVolume
	// A failed installation resumes its exact bound journal. A new explicit
	// reinstall of completed content gets a fresh destination and preserves it.
	resume := r.instance.State == storage.InstanceStateError || r.instance.State == storage.InstanceStateUninitialized || r.instance.State == storage.InstanceStateAdminCreated
	if resume && junimo140InstallVolumePattern.MatchString(volume) {
		plan, err = readInstallVolumePlan(r.instance.DataDir, volume)
		if err != nil {
			return err
		}
		if plan.ImageID != image.ImageID || plan.Image != image.Image || plan.Digest != image.Digest {
			return errors.New("installation image changed; the existing download journal was retained")
		}
	} else {
		project := strings.ToLower(filepath.Base(filepath.Clean(r.instance.DataDir)))
		if !runtimeComposeProjectPattern.MatchString(project) {
			return errors.New("invalid image-managed installation project")
		}
		id, err := randomHex(12)
		if err != nil {
			return err
		}
		target := project + "_anxi-smapi-update-" + id
		exists, err := docker.RuntimeVolumeExists(ctx, r.instance.DataDir, target)
		if err != nil || exists {
			return errors.New("new installation volume collision check failed")
		}
		sourceExists, err := docker.RuntimeVolumeExists(ctx, r.instance.DataDir, volume)
		if err != nil {
			return err
		}
		plan = junimo140InstallVolume{Schema: 1, Project: project, SourceVolume: volume, TargetVolume: target,
			Image: image.Image, ImageID: image.ImageID, Digest: image.Digest, SourceExisted: sourceExists}
		if err := writeInstallVolumePlan(r.instance.DataDir, plan); err != nil {
			return err
		}
		// No Docker volume exists yet. Failure to bind leaves only an inert local
		// intent; it cannot orphan a volume or discard a partial download.
		if err := switchGameDataVolumeAtomic(r.instance.DataDir, target); err != nil {
			return err
		}
	}
	exists, err := docker.RuntimeVolumeExists(ctx, r.instance.DataDir, plan.TargetVolume)
	if err != nil {
		return err
	}
	if !exists {
		if plan.Created || plan.Prepared {
			return errors.New("recorded installation volume disappeared; refusing to replace its download data")
		}
		if err := docker.RuntimeCreateSMAPIStagingVolume(ctx, r.instance.DataDir, plan.Project, plan.TargetVolume); err != nil {
			return err
		}
	}
	if err := docker.RuntimeValidateUnusedSMAPIStaging(ctx, r.instance.DataDir, plan.TargetVolume); err != nil {
		return err
	}
	if !plan.Created {
		plan.Created = true
		if err := writeInstallVolumePlan(r.instance.DataDir, plan); err != nil {
			return err
		}
	}
	if plan.Prepared {
		return nil
	}
	if plan.SourceExisted {
		if err := docker.RuntimeClonePinnedJunimo140Game(ctx, r.instance.DataDir, plan.SourceVolume, plan.TargetVolume, image.Image, image.ImageID); err != nil {
			return err
		}
	}
	plan.Prepared = true
	return writeInstallVolumePlan(r.instance.DataDir, plan)
}

func (r *installRunner) ensureJunimo140SMAPIInstalled(ctx context.Context, jobCtx *jobs.Context, imageRef string) error {
	docker, image, err := r.junimo140InstallImage(ctx, imageRef)
	if err != nil {
		return err
	}
	volume, err := GameDataVolumeName(r.instance.DataDir)
	if err != nil {
		return err
	}
	current, err := docker.RuntimeReadPinnedSMAPIMetadata(ctx, r.instance.DataDir, volume, image.Image, image.ImageID)
	if err != nil {
		return err
	}
	if junimo140SMAPIIdentityVerified(current) {
		plan, err := readInstallVolumePlan(r.instance.DataDir, volume)
		if err != nil || !plan.Prepared || plan.ImageID != image.ImageID {
			return errors.New("verified SMAPI is outside the prepared installation volume journal")
		}
		plan.SMAPIVerified = true
		return writeInstallVolumePlan(r.instance.DataDir, plan)
	}
	plan, err := readInstallVolumePlan(r.instance.DataDir, volume)
	if err != nil || !plan.Prepared || plan.ImageID != image.ImageID {
		return errors.New("image-managed SMAPI requires the prepared installation volume journal")
	}
	r.driver.updatePhase(context.Background(), r.instance.ID, storage.InstanceStateSteamAuthRunning,
		"正在安装并验证 Junimo .140 镜像内置的 SMAPI...", "smapi_installing", jobCtx.ID)
	_, _ = jobCtx.Info(context.Background(), "[smapi] 使用固定 Junimo .140 镜像内置构建，校验核心文件后提交构建标记。")
	if err := prepareJunimo140GameMarkers(ctx, docker, r.instance.DataDir, volume, image, time.Now()); err != nil {
		return fmt.Errorf("verify downloaded game before patched SMAPI: %w", err)
	}
	if _, err := prepareJunimo140SMAPIStaging(ctx, docker, r.instance.DataDir, volume, image); err != nil {
		return err
	}
	if err := prepareJunimo140GameMarkers(ctx, docker, r.instance.DataDir, volume, image, time.Now()); err != nil {
		return fmt.Errorf("verify downloaded game after patched SMAPI: %w", err)
	}
	plan.SMAPIVerified = true
	return writeInstallVolumePlan(r.instance.DataDir, plan)
}
