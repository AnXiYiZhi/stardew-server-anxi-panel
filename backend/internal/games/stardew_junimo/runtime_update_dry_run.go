package stardew_junimo

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

const (
	RuntimeUpdateDryRunJobType = "stardew_junimo_update_dry_run"

	RuntimeUpdatePhaseIdle              = "idle"
	RuntimeUpdatePhaseStarting          = "starting"
	RuntimeUpdatePhaseChecking          = "checking"
	RuntimeUpdatePhasePullingServer     = "pulling_server"
	RuntimeUpdatePhasePullingAuth       = "pulling_auth"
	RuntimeUpdatePhaseValidatingCompose = "validating_compose"
	RuntimeUpdatePhaseSucceeded         = "succeeded"
	RuntimeUpdatePhaseFailed            = "failed"
	RuntimeUpdatePhaseUnsupported       = "unsupported"
)

var runtimeComposeProjectPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
var runtimeImageDigestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

type RuntimeUpdateDockerService interface {
	DockerVersion(context.Context, string) (paneldocker.CommandResult, error)
	ComposeVersion(context.Context, string) (paneldocker.CommandResult, error)
	ComposePs(context.Context, string) (paneldocker.ComposePsResult, error)
	PullImageStreaming(context.Context, string, string, func(string)) (paneldocker.CommandResult, error)
	RuntimeImageInspect(context.Context, string, string) (paneldocker.RuntimeImageMetadata, error)
	RuntimeComposeConfigInspect(context.Context, string, string) (paneldocker.RuntimeComposeConfig, error)
	RuntimeComposeConfigValidateImages(context.Context, string, string, string, string) error
	RuntimeVolumeInspect(context.Context, string, string) (paneldocker.RuntimeVolumeMetadata, error)
}

type RuntimeUpdateDryRunCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type RuntimeUpdateDryRunLog struct {
	At      string `json:"at"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

type RuntimeUpdateSelectedImage struct {
	Image   string `json:"image,omitempty"`
	Digest  string `json:"digest,omitempty"`
	ImageID string `json:"imageId,omitempty"`
}

type RuntimeUpdateSelectedPair struct {
	Server    RuntimeUpdateSelectedImage `json:"server"`
	SteamAuth RuntimeUpdateSelectedImage `json:"steamAuth"`
}

type RuntimeUpdateDownloadProgress struct {
	Component   string `json:"component"`
	Image       string `json:"image,omitempty"`
	DoneLayers  int    `json:"doneLayers"`
	TotalLayers int    `json:"totalLayers"`
	Percent     int    `json:"percent"`
}

type RuntimeUpdateDryRunStatus struct {
	DryRunID      string                              `json:"dryRunId,omitempty"`
	JobID         string                              `json:"jobId,omitempty"`
	Phase         string                              `json:"phase"`
	Progress      int                                 `json:"progress"`
	Download      *RuntimeUpdateDownloadProgress      `json:"download,omitempty"`
	Current       sjconfig.RuntimeStackCurrent        `json:"current"`
	Target        sjconfig.RuntimeStackRecommendation `json:"target"`
	Selected      RuntimeUpdateSelectedPair           `json:"selected"`
	Checks        []RuntimeUpdateDryRunCheck          `json:"checks"`
	Warnings      []string                            `json:"warnings"`
	Logs          []RuntimeUpdateDryRunLog            `json:"logs"`
	ServerRunning bool                                `json:"serverRunning"`
	ErrorCode     string                              `json:"errorCode,omitempty"`
	Error         string                              `json:"error,omitempty"`
	StartedAt     string                              `json:"startedAt,omitempty"`
	UpdatedAt     string                              `json:"updatedAt,omitempty"`
	FinishedAt    string                              `json:"finishedAt,omitempty"`
}

type RuntimeUpdateValidationError struct {
	Code    string
	Message string
}

func (e *RuntimeUpdateValidationError) Error() string { return e.Message }

func IsRuntimeUpdateValidationError(err error) (*RuntimeUpdateValidationError, bool) {
	var validation *RuntimeUpdateValidationError
	return validation, errors.As(err, &validation)
}

func (d *Driver) rejectActiveRuntimeUpdate(ctx context.Context, instanceID string) error {
	if d.jobs == nil {
		return nil
	}
	active, err := d.jobs.Active(ctx, storage.ListActiveJobsFilter{
		TargetType: "instance",
		TargetID:   instanceID,
		Types:      []string{RuntimeUpdateDryRunJobType, "stardew_junimo_update_apply", SMAPIUpdateDryRunJobType, SMAPIUpdateApplyJobType},
	})
	if err != nil {
		return fmt.Errorf("list runtime update jobs: %w", err)
	}
	if len(active) > 0 {
		return &RuntimeUpdateValidationError{Code: "runtime_update_busy", Message: "实例存在运行组件升级预检或执行任务，请等待任务结束。"}
	}
	return nil
}

func (d *Driver) StartRuntimeUpdateDryRun(ctx context.Context, instance registry.Instance, createdBy int64) (RuntimeUpdateDryRunStatus, error) {
	if d.jobs == nil || d.store == nil {
		return RuntimeUpdateDryRunStatus{}, errors.New("runtime update dry-run service is not configured")
	}
	if instance.DriverID != DriverID {
		return RuntimeUpdateDryRunStatus{}, &RuntimeUpdateValidationError{Code: "unsupported/driver", Message: "实例不是 stardew_junimo driver。"}
	}
	inspection := InspectManagedRuntimeStack(instance.DataDir, instance.State)
	switch inspection.Status {
	case sjconfig.RuntimeStackStatusNotInstalled:
		return RuntimeUpdateDryRunStatus{}, &RuntimeUpdateValidationError{Code: "not_installed", Message: "实例尚未安装 Junimo 运行组件。"}
	case sjconfig.RuntimeStackStatusCustomImages:
		return RuntimeUpdateDryRunStatus{}, &RuntimeUpdateValidationError{Code: "unsupported/custom_images", Message: inspection.Reason}
	case sjconfig.RuntimeStackStatusInvalidConfig:
		return RuntimeUpdateDryRunStatus{}, &RuntimeUpdateValidationError{Code: inspection.Code, Message: inspection.Reason}
	}
	runtimeDocker, ok := d.docker.(RuntimeUpdateDockerService)
	if !ok {
		return RuntimeUpdateDryRunStatus{}, &RuntimeUpdateValidationError{Code: "unsupported/docker_contract", Message: "当前 Docker driver 不支持 Junimo 升级预检。"}
	}

	d.runtimeUpdateMu.Lock()
	defer d.runtimeUpdateMu.Unlock()
	active, err := d.jobs.Active(ctx, storage.ListActiveJobsFilter{
		TargetType: "instance",
		TargetID:   instance.ID,
		Types:      []string{"stardew_install", "stardew_lifecycle", "stardew_junimo_update_apply", RuntimeUpdateDryRunJobType, SMAPIUpdateDryRunJobType, SMAPIUpdateApplyJobType},
	})
	if err != nil {
		return RuntimeUpdateDryRunStatus{}, fmt.Errorf("list conflicting jobs: %w", err)
	}
	if len(active) > 0 {
		return RuntimeUpdateDryRunStatus{}, &RuntimeUpdateValidationError{Code: "runtime_update_busy", Message: "实例存在安装、生命周期或运行组件更新任务，请等待任务结束。"}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	status := RuntimeUpdateDryRunStatus{
		DryRunID:      newRuntimeDryRunID(),
		Phase:         RuntimeUpdatePhaseStarting,
		Current:       inspection.Current,
		Target:        inspection.Recommended,
		Checks:        []RuntimeUpdateDryRunCheck{},
		Warnings:      []string{},
		Logs:          []RuntimeUpdateDryRunLog{},
		ServerRunning: instance.State == storage.InstanceStateRunning,
		StartedAt:     now,
		UpdatedAt:     now,
	}
	gate := make(chan struct{})
	var initialWriteErr error
	job, err := d.jobs.Start(ctx, jobs.Spec{
		Type:        RuntimeUpdateDryRunJobType,
		DisplayName: "Junimo 运行组件升级预检",
		TargetType:  "instance",
		TargetID:    instance.ID,
		CreatedBy:   createdBy,
		Timeout:     time.Hour,
		Run: func(runCtx context.Context, jobContext *jobs.Context) error {
			<-gate
			if initialWriteErr != nil {
				return errors.New("升级预检状态初始化失败。")
			}
			return d.runRuntimeUpdateDryRun(runCtx, jobContext, runtimeDocker, instance, status)
		},
	})
	if err != nil {
		return RuntimeUpdateDryRunStatus{}, fmt.Errorf("start runtime update dry-run job: %w", err)
	}
	status.JobID = job.ID
	initialWriteErr = writeRuntimeUpdateDryRunStatus(instance.DataDir, status)
	close(gate)
	if initialWriteErr != nil {
		return RuntimeUpdateDryRunStatus{}, initialWriteErr
	}
	return status, nil
}

func (d *Driver) RuntimeUpdateDryRunStatus(instance registry.Instance) (RuntimeUpdateDryRunStatus, error) {
	status, err := readRuntimeUpdateDryRunStatus(instance.DataDir)
	if errors.Is(err, os.ErrNotExist) {
		inspection := InspectManagedRuntimeStack(instance.DataDir, instance.State)
		return RuntimeUpdateDryRunStatus{
			Phase:         RuntimeUpdatePhaseIdle,
			Current:       inspection.Current,
			Target:        inspection.Recommended,
			Checks:        []RuntimeUpdateDryRunCheck{},
			Warnings:      []string{},
			Logs:          []RuntimeUpdateDryRunLog{},
			ServerRunning: instance.State == storage.InstanceStateRunning,
		}, nil
	}
	return status, err
}

func newRuntimeDryRunID() string {
	var value [12]byte
	if _, err := rand.Read(value[:]); err != nil {
		return fmt.Sprintf("dryrun_%d", time.Now().UnixNano())
	}
	return "dryrun_" + hex.EncodeToString(value[:])
}

func runtimeUpdateDryRunStatusPath(dataDir string) string {
	return filepath.Join(dataDir, ".local-container", "junimo-update", "dry-run-status.json")
}

func readRuntimeUpdateDryRunStatus(dataDir string) (RuntimeUpdateDryRunStatus, error) {
	data, err := readRuntimeStatusFile(runtimeUpdateDryRunStatusPath(dataDir))
	if err != nil {
		return RuntimeUpdateDryRunStatus{}, err
	}
	var status RuntimeUpdateDryRunStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return RuntimeUpdateDryRunStatus{}, fmt.Errorf("decode runtime update dry-run status: %w", err)
	}
	if status.Checks == nil {
		status.Checks = []RuntimeUpdateDryRunCheck{}
	}
	if status.Warnings == nil {
		status.Warnings = []string{}
	}
	if status.Logs == nil {
		status.Logs = []RuntimeUpdateDryRunLog{}
	}
	return status, nil
}

func writeRuntimeUpdateDryRunStatus(dataDir string, status RuntimeUpdateDryRunStatus) error {
	path := runtimeUpdateDryRunStatusPath(dataDir)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create runtime update status dir: %w", err)
	}
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".dry-run-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return replaceRuntimeUpdateStatusFile(tmpName, path)
}
