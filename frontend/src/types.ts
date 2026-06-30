export type Role = 'admin' | 'user'

export type CurrentUser = {
  id: number
  username: string
  role: Role
  isSuperAdmin: boolean
}

export type PanelUser = CurrentUser & {
  isActive: boolean
  createdAt: string
  updatedAt: string
  lastLoginAt: string | null
}

export type SetupStatus = {
  initialized: boolean
}

export type UserResponse = {
  user: CurrentUser
}

export type PanelUserResponse = {
  user: PanelUser
}

export type UsersResponse = {
  users: PanelUser[]
}

export type OKResponse = {
  ok: boolean
}

export type CommandResult = {
  workDir: string
  args?: string[]
  stdout: string
  stderr: string
  exitCode: number
  durationMs: number
  timedOut: boolean
  stdoutTruncated?: boolean
  stderrTruncated?: boolean
}

export type DockerAvailability = {
  available: boolean
  result?: CommandResult
}

export type ComposeProjectStatus = {
  workDir: string
  workDirExists: boolean
  composeFileExists: boolean
  ready: boolean
}

export type DockerStatusResponse = {
  docker: DockerAvailability
  compose: DockerAvailability
  composeProject: ComposeProjectStatus
}

export type ComposeService = {
  name?: string
  service?: string
  state?: string
  status?: string
  health?: string
  exitCode?: number
}

export type ComposePsResponse = {
  workDir: string
  result: CommandResult
  services: ComposeService[]
}

export type ComposeLogsResponse = {
  workDir: string
  service: string
  tail: number
  result: CommandResult
}

export type JobStatus = 'queued' | 'running' | 'succeeded' | 'failed' | 'canceled'

export type Job = {
  id: string
  type: string
  status: JobStatus
  targetType: string
  targetId: string
  createdBy: number | null
  createdAt: string
  startedAt: string | null
  finishedAt: string | null
  errorMessage: string | null
  updatedAt: string
}

export type JobLogLevel = 'info' | 'warn' | 'error' | 'debug'

export type JobLog = {
  id: number
  jobId: string
  level: JobLogLevel
  message: string
  createdAt: string
  sequence: number
}

export type JobsResponse = {
  jobs: Job[]
}

export type JobResponse = {
  job: Job
}

export type JobLogsResponse = {
  logs: JobLog[]
}

export type Instance = {
  id: string
  driverId: string
  driverName?: string
  name: string
  state: string
  stateMessage: string | null
  driverPhase: string
  createdAt: string
  updatedAt: string
}

export type InstancesResponse = {
  instances: Instance[]
}

export type InstanceState = {
  instanceId: string
  driverId: string
  name: string
  state: string
  stateMessage: string | null
  driverPhase: string
  updatedAt: string
}

export type ResourceMetricSample = {
  timestamp: string
  cpuPercent: number | null
  memoryPercent: number | null
  memoryUsedBytes?: number
  memoryLimitBytes?: number
  diskPercent: number | null
  diskUsedBytes?: number
  diskTotalBytes?: number
  containerRunning: boolean
  message?: string
}

export type ResourceMetricsResponse = {
  instanceId: string
  service: string
  sample: ResourceMetricSample
}

export type StardewPlayerInfo = {
  name: string
  role?: string
  location?: string
  locationName?: string
  locationDisplayName?: string
  tileX?: number
  tileY?: number
  pixelX?: number
  pixelY?: number
  onlineFor?: string
  status: 'online' | string
  source: string
  uniqueMultiplayerId?: string
  isHost?: boolean
  money?: number
  farmIncome?: number
  personalIncome?: number
  totalMoneyEarned?: number
  walletMode?: 'shared' | 'separate' | string
  lastSeen?: string
}

export type StardewPlayerEvent = {
  id: string
  type: 'seen' | 'joined' | 'left' | string
  playerName: string
  uniqueMultiplayerId?: string
  isHost?: boolean
  location?: string
  locationName?: string
  locationDisplayName?: string
  saveId?: string
  at: string
  message: string
}

export type StardewPlayersResponse = {
  instanceId: string
  state: string
  source?: string
  saveId?: string
  onlineCount: number | null
  maxPlayers: number | null
  players: StardewPlayerInfo[]
  recentEvents?: StardewPlayerEvent[]
  rawInfo?: string
  parseStatus: 'exact' | 'partial' | 'unavailable' | string
  message?: string
  updatedAt: string
}

export type InstallJobResponse = {
  jobId: string
}

export type PrepareResponse = {
  instanceId: string
  state: string
  stateMessage: string | null
  driverPhase: string
}

export type ImageTagOption = {
  tag: string
  label: string
  recommended: boolean
  warning?: string
  isLatest?: boolean
}

export type InstallOptionsResponse = {
  imageTagOptions: ImageTagOption[]
}

export type SaveInfo = {
  name: string
  farmerName?: string
  farmName?: string
  gameYear?: number
  gameSeason?: string
  gameDay?: number
  farmType?: string
  fileSizeBytes?: number
  modifiedAt?: string
  parseError?: string
  isActive?: boolean
}

export type RgbColor = {
  r: number
  g: number
  b: number
}

export type NewGameConfig = {
  farmName: string
  farmType: string
  startingCabins: number
  cabinLayout: string
  profitMargin: string  // "100"|"75"|"50"|"25"
  petBreed: number      // 0-4 Stardew selectable breed index
  moneyMode: string     // "shared"|"separate"
  remixedCommunityCenter: boolean
  remixedMineRewards: boolean
  spawnMonstersOnFarm: boolean
  skipIntro: true

  // SMAPI character fields (applied by StardewAnxiPanel.Control mod)
  farmerName?: string
  favoriteThing?: string
  gender?: string       // "male"|"female"
  petType?: string      // "Cat"|"Dog"
  petBreedId?: string   // SMAPI breed string ID
  skin?: number
  hair?: number
  shirt?: string
  pants?: string
  accessory?: number
  eyeColor?: RgbColor
  hairColor?: RgbColor
  pantsColor?: RgbColor
}

export type PreflightResult = {
  hasSaves: boolean
  saves: SaveInfo[]
  templateAvailable: boolean
}

export type SavesListResult = {
  saves: SaveInfo[]
  activeSaveName: string
}

export type BackupInfo = {
  name: string
  saveName: string
  size: number
  createdAt: string
  farmerName?: string
  farmName?: string
  gameYear?: number
  gameSeason?: string
  gameDay?: number
  farmType?: string
  fileSizeBytes?: number
  parseError?: string
}

export type BackupsListResult = {
  backups: BackupInfo[]
}

export type RestoreBackupResult = {
  saveName: string
}

export type UploadPreviewResult = {
  token: string
  preview: SaveInfo
  saveName: string
}

export type InviteCodeResult = {
  inviteCode: string
}

export type LifecycleJobResponse = {
  jobId: string
  saveName?: string
}

export type InstanceVNCConfig = {
  vncPort: string
}

export type ModSyncKind = 'server_only' | 'client_required' | 'unknown'

export type ModInfo = {
  id: string
  uniqueId?: string
  name?: string
  version?: string
  author?: string
  description?: string
  folderName: string
  parseError?: string
  syncKind: ModSyncKind
  syncNote?: string
  updateKeys?: string[]
  nexusModId?: number
}

export type ModsListResult = {
  mods: ModInfo[]
  restartRequired?: boolean
}

export type ModSyncSummary = {
  total: number
  serverOnly: number
  clientRequired: number
  unknown: number
}

export type ModSyncPlanResult = {
  mods: ModInfo[]
  summary: ModSyncSummary
}

export type NexusModSearchResult = {
  modId: number
  name: string
  summary?: string
  author?: string
  version?: string
  updatedAt?: string
  endorsementCount: number
  downloadCount: number
  pictureUrl?: string
  nexusUrl: string
  installed: boolean
  installedFolderName?: string
  installedVersion?: string
}

export type NexusModSearchResponse = {
  query: string
  results: NexusModSearchResult[]
}

export type ConsoleCommandDef = {
  id: string
  name: string
  description: string
  adminOnly: boolean
}

export type CommandsListResult = {
  commands: ConsoleCommandDef[]
}

export type CommandRunResult = {
  command: string
  output?: string
  error?: string
  exitCode: number
  durationMs: number
}
