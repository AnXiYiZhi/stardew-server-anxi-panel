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
	AutoDownload  bool   // skip auth method choice and run steam-auth download directly
}

// ImageTagOption describes one selectable image tag in the install UI.
type ImageTagOption struct {
	Tag         string `json:"tag"`
	Label       string `json:"label"`
	Recommended bool   `json:"recommended"`
	Warning     string `json:"warning,omitempty"`
	IsLatest    bool   `json:"isLatest,omitempty"`
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
	Name         string `json:"name"`
	FarmerName   string `json:"farmerName,omitempty"`
	FarmName     string `json:"farmName,omitempty"`
	GameYear     int    `json:"gameYear,omitempty"`
	GameSeason   string `json:"gameSeason,omitempty"`
	GameDay      int    `json:"gameDay,omitempty"`
	FarmType     string `json:"farmType,omitempty"`
	FileSizeBytes int64  `json:"fileSizeBytes,omitempty"`
	ModifiedAt   string `json:"modifiedAt,omitempty"`
	ParseError   string `json:"parseError,omitempty"`
}

// RgbColor is an RGB color for character appearance customization.
type RgbColor struct {
	R int `json:"r"`
	G int `json:"g"`
	B int `json:"b"`
}

// NewGameConfig holds parameters for creating a new game.
// Junimo server-settings fields are always applied.
// SMAPI character fields (Gender, PetType, PetBreedID, appearance) are written to
// server-init.json and applied by the SMAPI mod on the SaveCreating event.
type NewGameConfig struct {
	FarmName       string `json:"farmName"`
	FarmType       string `json:"farmType"`       // standard|riverland|forest|hilltop|wilderness|fourcorners|beach
	StartingCabins int    `json:"startingCabins"` // 0-3
	CabinLayout    string `json:"cabinLayout"`    // nearby|separate
	ProfitMargin   string `json:"profitMargin"`   // "100"|"75"|"50"|"25"
	PetBreed       int    `json:"petBreed"`       // 0-3 (Junimo server-settings index)
	MoneyMode      string `json:"moneyMode"`      // shared|separate

	// SMAPI character fields — require the StardewAnxiPanel.Control mod to be installed.
	FarmerName    string    `json:"farmerName,omitempty"`
	FavoriteThing string    `json:"favoriteThing,omitempty"`
	Gender        string    `json:"gender,omitempty"`     // male|female
	PetType       string    `json:"petType,omitempty"`    // Cat|Dog
	PetBreedID    string    `json:"petBreedId,omitempty"` // SMAPI breed string ID
	Skin          *int      `json:"skin,omitempty"`
	Hair          *int      `json:"hair,omitempty"`
	Shirt         string    `json:"shirt,omitempty"`
	Pants         string    `json:"pants,omitempty"`
	Accessory     *int      `json:"accessory,omitempty"`
	EyeColor      *RgbColor `json:"eyeColor,omitempty"`
	HairColor     *RgbColor `json:"hairColor,omitempty"`
	PantsColor    *RgbColor `json:"pantsColor,omitempty"`
}

// PreflightResult is returned by GET .../saves/preflight.
type PreflightResult struct {
	HasSaves    bool       `json:"hasSaves"`
	Saves       []SaveInfo `json:"saves"`
	TemplateAvailable bool `json:"templateAvailable"`
}

// UploadPreviewResult is returned by POST .../saves/upload-preview.
type UploadPreviewResult struct {
	Token    string   `json:"token"`
	Preview  SaveInfo `json:"preview"`
	SaveName string   `json:"saveName"`
}

// InviteCodeResult is returned by GET .../invite-code.
type InviteCodeResult struct {
	InviteCode string `json:"inviteCode"`
}

type ModInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
