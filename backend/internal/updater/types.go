package updater

import "time"

const (
	CodeSupported              = "supported"
	CodeDockerUnavailable      = "docker_unavailable"
	CodeComposeUnavailable     = "compose_unavailable"
	CodeSelfInspectFailed      = "self_inspect_failed"
	CodeDockerSocketMissing    = "docker_socket_missing"
	CodeComposeLabelsMissing   = "compose_labels_missing"
	CodeComposeMetadataInvalid = "compose_metadata_invalid"
	CodeDataMountMissing       = "data_mount_missing"
	CodeImageNotAllowed        = "image_not_allowed"
	CodeInvalidTargetVersion   = "invalid_target_version"
	CodeHelperStartFailed      = "helper_start_failed"
	CodeUpdateInProgress       = "update_in_progress"
	CodeUpdateNotAvailable     = "update_not_available"
	CodeDatabaseBackupFailed   = "database_backup_failed"
	CodeImagePullFailed        = "image_pull_failed"
	CodeComposeRecreateFailed  = "compose_recreate_failed"
	CodeHealthCheckFailed      = "health_check_failed"
	CodeVersionMismatch        = "version_mismatch"
	CodeRollbackFailed         = "rollback_failed"
)

const (
	PhaseChecking         = "checking"
	PhaseBackingUp        = "backing_up"
	PhasePulling          = "pulling"
	PhaseRecreating       = "recreating"
	PhaseWaitingHealth    = "waiting_health"
	PhaseSucceeded        = "succeeded"
	PhaseRollingBack      = "rolling_back"
	PhaseFailedRolledBack = "failed_rolled_back"
	PhaseRollbackFailed   = "rollback_failed"
)

type Capability struct {
	Supported          bool   `json:"supported"`
	Reason             string `json:"reason"`
	Code               string `json:"code"`
	ComposeProject     string `json:"composeProject"`
	ComposeService     string `json:"composeService"`
	ComposeFile        string `json:"composeFile"`
	InstallDir         string `json:"installDir"`
	CurrentContainer   string `json:"currentContainer"`
	CurrentImage       string `json:"currentImage"`
	DataMount          string `json:"dataMount"`
	DockerAvailable    bool   `json:"dockerAvailable"`
	ComposeAvailable   bool   `json:"composeAvailable"`
	ConversionRequired bool   `json:"conversionRequired,omitempty"`
}

type LogEntry struct {
	At      time.Time `json:"at"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
}

type DryRunStatus struct {
	ID            string     `json:"id"`
	Phase         string     `json:"phase"`
	TargetVersion string     `json:"targetVersion"`
	TargetImage   string     `json:"targetImage"`
	Capability    Capability `json:"capability"`
	Logs          []LogEntry `json:"logs"`
	StartedAt     time.Time  `json:"startedAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
	FinishedAt    *time.Time `json:"finishedAt"`
	ErrorCode     string     `json:"errorCode"`
	Error         string     `json:"error"`
}

type ApplyStatus struct {
	UpdateID         string           `json:"updateId"`
	Phase            string           `json:"phase"`
	Progress         int              `json:"progress"`
	FromVersion      string           `json:"fromVersion"`
	ToVersion        string           `json:"toVersion"`
	OriginalImage    string           `json:"originalImage"`
	OriginalDigest   string           `json:"originalDigest"`
	SelectedImage    string           `json:"selectedImage"`
	SelectedDigest   string           `json:"selectedDigest"`
	ErrorCode        string           `json:"errorCode"`
	Error            string           `json:"error"`
	Result           string           `json:"result"`
	CleanupCompleted bool             `json:"cleanupCompleted,omitempty"`
	Logs             []LogEntry       `json:"logs"`
	StartedAt        time.Time        `json:"startedAt"`
	UpdatedAt        time.Time        `json:"updatedAt"`
	FinishedAt       *time.Time       `json:"finishedAt"`
	FullStack        *FullStackStatus `json:"fullStack,omitempty"`
}

// FullStackStatus extends a successful Panel image replacement with the
// required game-runtime reconciliation performed by the new Panel process.
// It is deliberately separate from Phase: the helper remains the authority
// for Panel rollback while this status describes the second, resumable stage.
type FullStackStatus struct {
	Phase            string                    `json:"phase"`
	Progress         int                       `json:"progress"`
	InstanceID       string                    `json:"instanceId,omitempty"`
	RuntimeRequired  bool                      `json:"runtimeRequired"`
	ServerWasRunning bool                      `json:"serverWasRunning,omitempty"`
	OnlinePlayers    int                       `json:"onlinePlayers,omitempty"`
	BackupName       string                    `json:"backupName,omitempty"`
	ErrorCode        string                    `json:"errorCode,omitempty"`
	Error            string                    `json:"error,omitempty"`
	Result           string                    `json:"result,omitempty"`
	UpdatedAt        string                    `json:"updatedAt,omitempty"`
	FinishedAt       string                    `json:"finishedAt,omitempty"`
	Instances        []FullStackInstanceStatus `json:"instances,omitempty"`
}

type FullStackInstanceStatus struct {
	InstanceID       string `json:"instanceId"`
	Phase            string `json:"phase"`
	Progress         int    `json:"progress"`
	ServerWasRunning bool   `json:"serverWasRunning,omitempty"`
	OnlinePlayers    int    `json:"onlinePlayers,omitempty"`
	BackupName       string `json:"backupName,omitempty"`
	ErrorCode        string `json:"errorCode,omitempty"`
	Error            string `json:"error,omitempty"`
}

func IsActiveApplyPhase(phase string) bool {
	switch phase {
	case PhaseChecking, PhaseBackingUp, PhasePulling, PhaseRecreating, PhaseWaitingHealth, PhaseRollingBack:
		return true
	default:
		return false
	}
}

func Unsupported(code, reason string) Capability {
	return Capability{Supported: false, Code: code, Reason: reason}
}
