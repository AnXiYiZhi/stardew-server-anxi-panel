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

// DirectConnectConfig describes the driver-owned network endpoint exposed by
// one instance. The web layer combines this with its public-IP resolver rather
// than guessing game-specific ports.
type DirectConnectConfig struct {
	GamePort int    `json:"gamePort"`
	Protocol string `json:"protocol"`
}

// DirectConnectConfigProvider is an optional capability for games that expose
// a player-facing direct-connect endpoint.
type DirectConnectConfigProvider interface {
	DirectConnectConfig(ctx context.Context, instance Instance) (DirectConnectConfig, error)
}

// InstanceProvisionRequest asks a driver to turn a newly reserved instance row
// into an independently runnable server instance. Template is the driver-owned
// game installation target selected by the Panel, never by an API caller. The
// new instance keeps its own directory, Compose project, ports and data volume.
type InstanceProvisionRequest struct {
	Template Instance
	Target   Instance
	Existing []Instance
	ActorID  int64
}

// InstanceProvisionResult describes driver-owned resources assigned during
// provisioning. It is safe to return to an administrator and contains no
// credentials, host paths or Docker identifiers.
type InstanceProvisionResult struct {
	GamePort  int    `json:"gamePort"`
	QueryPort int    `json:"queryPort"`
	VNCPort   int    `json:"vncPort"`
	APIPort   int    `json:"apiPort"`
	Protocol  string `json:"protocol"`
}

// InstanceProvisioner is an optional driver capability. The web layer owns
// authentication, target path construction and the durable instance row;
// game-specific runtime copying and resource allocation stay in the driver.
type InstanceProvisioner interface {
	ProvisionInstance(ctx context.Context, req InstanceProvisionRequest) (InstanceProvisionResult, error)
	CleanupProvisionedInstance(ctx context.Context, instance Instance) error
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
	AutoDownload  bool   // reuse saved credentials without re-prompting; routing is decided from the instance phase
	SteamCMDRetry bool   // legacy: retained for compatibility; routing now derives from the instance phase
	ForceReauth   bool   // internal AuthLoginOnly retry resets only a failed/pending Auth session; base installs must reject this flag
	AuthLoginOnly bool   // reuse the shared credentials to run ONLY optional SteamAuth invite authorization (no game download, SteamCMD fallback, or SMAPI)
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
	Instance      Instance
	ActorID       int64
	NewGame       bool           // When true, lifecycle job creates a new official-farm save.
	NewGameConfig *NewGameConfig // Normalized, validated payload persisted by the lifecycle transaction.
	RequestID     string         // Stable idempotency key for a new-game request and all of its retries.
}

// SaveImportRequest starts the durable save-import transaction. PlatformID is
// deliberately transient: drivers must only persist a one-way fingerprint.
type SaveImportRequest struct {
	Instance    Instance
	ActorID     int64
	OperationID string
	Token       string
	StagedDir   string
	// TransferSourceOwnership moves the durable upload payload into the
	// operation-owned source directory. It is transient and never persisted.
	TransferSourceOwnership func(targetDir string) error
	// AttachJobIdentity durably binds the operation's upload token to the exact
	// save-import job. A successful return is the runner's permission to start;
	// implementations must not treat this callback as best-effort metadata.
	AttachJobIdentity func(jobID string) error
	// MarkUploadSucceeded terminalizes the durable upload token after the
	// import journal is fully completed. It is best-effort metadata cleanup and
	// must not change an already successful save import into a failed job.
	MarkUploadSucceeded func() error
	SaveName            string
	HostHandling        string
	PlatformID          string
}

// SaveImportStarter is an optional driver capability used by the web layer.
// Keeping it separate avoids widening the contract for unrelated game drivers.
type SaveImportStarter interface {
	ImportSaveAndStart(ctx context.Context, req SaveImportRequest) (*Job, error)
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
	NameWarning   string `json:"nameWarning,omitempty"`
	FarmerName    string `json:"farmerName,omitempty"`
	FarmName      string `json:"farmName,omitempty"`
	GameYear      int    `json:"gameYear,omitempty"`
	GameSeason    string `json:"gameSeason,omitempty"`
	GameDay       int    `json:"gameDay,omitempty"`
	FarmType      string `json:"farmType,omitempty"`
	FarmTypeLabel string `json:"farmTypeLabel,omitempty"`
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
// SMAPI-owned fields (farm cave, Gender, PetType, PetBreedID, appearance) are
// written to server-init.json and applied by Control inside the guarded new-game
// transaction when the target save is first loaded.
type NewGameConfig struct {
	FarmName       string `json:"farmName"`
	FarmType       string `json:"farmType"`       // standard|riverland|forest|hilltop|wilderness|fourcorners|beach
	FarmCaveChoice string `json:"farmCaveChoice"` // vanilla|bats|mushrooms
	StartingCabins int    `json:"startingCabins"` // 0-7
	MaxPlayers     int    `json:"maxPlayers"`     // 1-100, total concurrent players
	CabinLayout    string `json:"cabinLayout"`    // nearby|separate
	CabinMode      string `json:"cabinMode"`      // recommended|vanilla: recommended=CabinStack hidden cabins, vanilla=None visible cabins on the farm map (default)
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
	SteamInviteEnabled bool   `json:"steamInviteEnabled"`
	Status             string `json:"status"`
	InviteCode         string `json:"inviteCode"`
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
	InstalledAt      string `json:"installedAt,omitempty"`
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
	NexusModID       int    `json:"nexusModId,omitempty"`
	IsContentPack    bool   `json:"isContentPack,omitempty"`
	ContentPackFor   string `json:"contentPackFor,omitempty"`
	OriginSource     string `json:"originSource,omitempty"`
	OriginNexusModID int    `json:"originNexusModId,omitempty"`
	OriginModName    string `json:"originModName,omitempty"`
	OriginModURL     string `json:"originModUrl,omitempty"`
	// PackageKey identifies folders installed from the same physical package.
	// It is intentionally separate from Nexus identity: an aggregate Mods ZIP
	// may contain several unrelated Nexus packages.
	PackageKey   string          `json:"packageKey,omitempty"`
	PackageName  string          `json:"packageName,omitempty"`
	Dependencies []ModDependency `json:"dependencies,omitempty"`
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
	Mods                  []ModInfo                 `json:"mods"`
	RestartRequired       bool                      `json:"restartRequired,omitempty"`
	Upload                *ModUploadSummary         `json:"upload,omitempty"`
	CompatibilityWarnings []ModCompatibilityWarning `json:"compatibilityWarnings,omitempty"`
}

// ModUpdateInfo describes an update suggested by SMAPI's update service for
// one installed physical mod. The upstream page remains available for manual
// updates; eligible Nexus-backed single-mod packages can also use the explicit
// administrator-triggered safe replacement flow.
type ModUpdateInfo struct {
	ID             string `json:"id"`
	UniqueID       string `json:"uniqueId"`
	Name           string `json:"name"`
	FolderName     string `json:"folderName"`
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	URL            string `json:"url"`
}

// ModUpdateCheckResult is returned by GET .../mod-updates and
// POST .../mod-updates/check. Status is "ok" or "error"; an upstream failure
// remains a successful HTTP response so the page can keep showing the last
// known result together with CheckError.
type ModUpdateCheckResult struct {
	Status        string          `json:"status"`
	CheckedAt     string          `json:"checkedAt,omitempty"`
	Updates       []ModUpdateInfo `json:"updates"`
	EligibleCount int             `json:"eligibleCount"`
	SkippedCount  int             `json:"skippedCount"`
	CheckError    string          `json:"checkError,omitempty"`
	Cached        bool            `json:"cached"`
}

// ModUploadSummary describes the complete, atomic result of a manual upload.
// DiscoveredCount includes valid manifests supplied for SMAPI-bundled support
// mods; ImportedCount excludes those intentionally skipped duplicate copies.
type ModUploadSummary struct {
	ArchiveCount        int      `json:"archiveCount"`
	DiscoveredCount     int      `json:"discoveredCount"`
	ImportedCount       int      `json:"importedCount"`
	EnabledCount        int      `json:"enabledCount"`
	SkippedBuiltInCount int      `json:"skippedBuiltInCount,omitempty"`
	SkippedBuiltInNames []string `json:"skippedBuiltInNames,omitempty"`
	ActiveSaveName      string   `json:"activeSaveName,omitempty"`
}

// ModCompatibilityWarning reports a save-specific condition which cannot be
// repaired safely by merely enabling a mod. It is intentionally advisory: the
// panel never rewrites world terrain or serialized quest state automatically.
type ModCompatibilityWarning struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Title    string `json:"title"`
	Message  string `json:"message"`
	SaveName string `json:"saveName,omitempty"`
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
