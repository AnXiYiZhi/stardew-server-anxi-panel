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
	NewGame  bool // When true, lifecycle job sends "settings newgame --confirm" after server starts.
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
	Name          string `json:"name"`
	FarmerName    string `json:"farmerName,omitempty"`
	FarmName      string `json:"farmName,omitempty"`
	GameYear      int    `json:"gameYear,omitempty"`
	GameSeason    string `json:"gameSeason,omitempty"`
	GameDay       int    `json:"gameDay,omitempty"`
	FarmType      string `json:"farmType,omitempty"`
	FileSizeBytes int64  `json:"fileSizeBytes,omitempty"`
	ModifiedAt    string `json:"modifiedAt,omitempty"`
	ParseError    string `json:"parseError,omitempty"`
	IsActive      bool   `json:"isActive,omitempty"`
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
	StartingCabins int    `json:"startingCabins"` // 0-7
	MaxPlayers     int    `json:"maxPlayers"`     // 1-100, total concurrent players
	CabinLayout    string `json:"cabinLayout"`    // nearby|separate
	ProfitMargin   string `json:"profitMargin"`   // "100"|"75"|"50"|"25"
	PetBreed       int    `json:"petBreed"`       // 0-4 (Stardew selectable breed index)
	MoneyMode      string `json:"moneyMode"`      // shared|separate
	// New-game advanced settings. These map directly to JunimoServer's
	// GameCreator settings and are persisted before its first world creation.
	RemixedCommunityCenter bool `json:"remixedCommunityCenter"`
	RemixedMineRewards     bool `json:"remixedMineRewards"`
	SpawnMonstersOnFarm    bool `json:"spawnMonstersOnFarm"`
	// The panel always creates a server game without the vanilla intro.
	// It remains explicit in the DTO so the saved creation payload is auditable.
	SkipIntro bool `json:"skipIntro"`

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
	HasSaves          bool       `json:"hasSaves"`
	Saves             []SaveInfo `json:"saves"`
	TemplateAvailable bool       `json:"templateAvailable"`
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

// SavesListResult is returned by GET .../saves.
type SavesListResult struct {
	Saves          []SaveInfo `json:"saves"`
	ActiveSaveName string     `json:"activeSaveName"`
}

type ModInfo struct {
	ID               string `json:"id"`
	UniqueID         string `json:"uniqueId,omitempty"`
	Name             string `json:"name,omitempty"`
	Version          string `json:"version,omitempty"`
	Author           string `json:"author,omitempty"`
	Description      string `json:"description,omitempty"`
	FolderName       string `json:"folderName"`
	ParseError       string `json:"parseError,omitempty"`
	Enabled          bool   `json:"enabled"`
	CanToggle        bool   `json:"canToggle,omitempty"`
	EnableNote       string `json:"enableNote,omitempty"`
	SyncKind         string `json:"syncKind"`
	SyncNote         string `json:"syncNote,omitempty"`
	BuiltIn          bool   `json:"builtIn,omitempty"`
	NexusSummary     string `json:"nexusSummary,omitempty"`
	UpdatedAt        string `json:"updatedAt,omitempty"`
	EndorsementCount int    `json:"endorsementCount,omitempty"`
	DownloadCount    int    `json:"downloadCount,omitempty"`
	PictureURL       string `json:"pictureUrl,omitempty"`
	NexusURL         string `json:"nexusUrl,omitempty"`
	// UpdateKeys is the manifest.json UpdateKeys list (e.g. "Nexus:123"),
	// used to resolve NexusModID. Not all mods declare it.
	UpdateKeys []string `json:"updateKeys,omitempty"`
	// NexusModID is parsed from a "Nexus:<id>" entry in UpdateKeys, if present.
	NexusModID       int             `json:"nexusModId,omitempty"`
	IsContentPack    bool            `json:"isContentPack,omitempty"`
	ContentPackFor   string          `json:"contentPackFor,omitempty"`
	OriginSource     string          `json:"originSource,omitempty"`
	OriginNexusModID int             `json:"originNexusModId,omitempty"`
	OriginModName    string          `json:"originModName,omitempty"`
	OriginModURL     string          `json:"originModUrl,omitempty"`
	Dependencies     []ModDependency `json:"dependencies,omitempty"`
}

type ModDependency struct {
	UniqueID         string `json:"uniqueId"`
	MinimumVersion   string `json:"minimumVersion,omitempty"`
	Required         bool   `json:"required"`
	Installed        bool   `json:"installed"`
	Enabled          bool   `json:"enabled"`
	InstalledVersion string `json:"installedVersion,omitempty"`
	Satisfied        bool   `json:"satisfied"`
	Status           string `json:"status,omitempty"`
}

// ModsListResult is returned by GET .../mods.
type ModsListResult struct {
	Mods            []ModInfo `json:"mods"`
	RestartRequired bool      `json:"restartRequired,omitempty"`
}

// Mod sync classification kinds. They describe whether a mod must be
// installed client-side by players to join the server.
const (
	ModSyncKindServerOnly     = "server_only"
	ModSyncKindClientRequired = "client_required"
	ModSyncKindUnknown        = "unknown"
)

// ValidModSyncKind reports whether kind is one of the known sync classifications.
func ValidModSyncKind(kind string) bool {
	switch kind {
	case ModSyncKindServerOnly, ModSyncKindClientRequired, ModSyncKindUnknown:
		return true
	}
	return false
}

// ModSyncSummary counts installed mods by sync classification.
type ModSyncSummary struct {
	Total          int `json:"total"`
	ServerOnly     int `json:"serverOnly"`
	ClientRequired int `json:"clientRequired"`
	Unknown        int `json:"unknown"`
}

// ModSyncPlanResult is returned by GET .../mods/sync-plan.
type ModSyncPlanResult struct {
	Mods    []ModInfo      `json:"mods"`
	Summary ModSyncSummary `json:"summary"`
}
