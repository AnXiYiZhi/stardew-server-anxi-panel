package registry

import "context"

type GameDriver interface {
	ID() string
	Name() string

	Prepare(ctx context.Context, instance Instance) error
	Install(ctx context.Context, req InstallRequest) (*Job, error)
	Start(ctx context.Context, req StartRequest) (*Job, error)
	Stop(ctx context.Context, instance Instance) error
	Restart(ctx context.Context, instance Instance) error

	Status(ctx context.Context, instance Instance) (*ServerStatus, error)
	Logs(ctx context.Context, instance Instance) (<-chan LogLine, error)
	ExecCommand(ctx context.Context, cmd string) (*CommandResult, error)

	ListSaves(ctx context.Context, instance Instance) ([]SaveInfo, error)
	UploadSave(ctx context.Context, file UploadedFile) error
	SelectSave(ctx context.Context, name string) error
	DeleteSave(ctx context.Context, name string) error

	ListMods(ctx context.Context, instance Instance) ([]ModInfo, error)
	UploadMod(ctx context.Context, file UploadedFile) error
	DeleteMod(ctx context.Context, id string) error
}

type Instance struct {
	ID            string
	DriverID      string
	Name          string
	DataDir       string
	State         string
	StateMessage  string
	DriverPhase   string
	DriverPayload string
	CreatedAt     string
	UpdatedAt     string
}

type InstallRequest struct {
	Instance      Instance
	ActorID       int64
	SteamUsername string
	SteamPassword string // never log this field
	VNCPassword   string // never log this field
	ImageTag      string // docker image tag, e.g. "latest" or a pinned version
}

// ImageTagOption describes one selectable image tag in the install UI.
type ImageTagOption struct {
	Tag         string `json:"tag"`
	Label       string `json:"label"`
	Recommended bool   `json:"recommended"`
	Warning  string `json:"warning,omitempty"`
	IsLatest bool   `json:"isLatest,omitempty"`
}

// SteamGuardSender is an optional capability for drivers that handle Steam
// two-factor authentication.  The web layer type-asserts against this interface
// when handling POST …/steam-guard/input.
type SteamGuardSender interface {
	SendSteamGuardInput(jobID string, input string) error
}

type StartRequest struct {
	Instance Instance
	ActorID  int64
}

type Job struct {
	ID string `json:"id"`
}

type ServerStatus struct {
	InstanceID   string          `json:"instanceId"`
	DriverID     string          `json:"driverId"`
	DriverName   string          `json:"driverName"`
	Name         string          `json:"name"`
	State        string          `json:"state"`
	StateMessage string          `json:"stateMessage,omitempty"`
	DriverPhase  string          `json:"driverPhase,omitempty"`
	Runtime      *RuntimeStatus  `json:"runtime,omitempty"`
	Warnings     []StatusWarning `json:"warnings,omitempty"`
}

type RuntimeStatus struct {
	Containers []ContainerSummary `json:"containers"`
}

type ContainerSummary struct {
	Name    string `json:"name,omitempty"`
	Service string `json:"service,omitempty"`
	State   string `json:"state,omitempty"`
	Status  string `json:"status,omitempty"`
	Health  string `json:"health,omitempty"`
}

type StatusWarning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type LogLine struct {
	Message string `json:"message"`
	Level   string `json:"level"`
}

type CommandResult struct {
	ExitCode int    `json:"exitCode"`
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
}

type UploadedFile struct {
	Name string
	Size int64
}

type SaveInfo struct {
	Name string `json:"name"`
}

type ModInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
