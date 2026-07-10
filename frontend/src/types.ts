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

export type JobStatus = 'queued' | 'running' | 'succeeded' | 'failed' | 'canceled'

export type Job = {
  id: string
  type: string
  displayName?: string | null
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
  // Durable UI flag: true after steam-auth login succeeds; cleared when server logs prove steam-auth has no account.
  steamAuthLoggedIn?: boolean
  // Diagnostic runtime flag: true only when the running steam-auth service probes ready.
  steamAuthReady?: boolean
  // Last invite code recorded by the backend background probe, if any.
  inviteCode?: string
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
  maxPlayers: number
  cabinLayout: string
  cabinMode?: string     // "recommended"|"vanilla": recommended=隐藏小屋堆叠, vanilla=小屋出现在真实农场位置
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
  kind: 'manual' | 'latest' | 'daily' | 'scheduled' | string
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

export type BackupPolicy = {
  gameSaveBackups: boolean
  dailySnapshots: boolean
  dailyRetentionDays: number
  scheduledBackups: boolean
  scheduledHour: number
  scheduledIntervalHours?: number
}

export type BackupMaintenanceResult = {
  createdBackupNames: string[]
  consumedEvents: number
}

export type BackupsListResult = {
  backups: BackupInfo[]
  policy?: BackupPolicy
  maintenance?: BackupMaintenanceResult
}

export type RestoreBackupResult = {
  saveName: string
}

export type BackupCreateResult = {
  backupName: string
}

export type BackupPolicyResult = {
  policy: BackupPolicy
}

export type RestartSchedule = {
  instanceId: string
  enabled: boolean
  shutdownTime: string
  startupTime: string
  timezone: string
  warningMinutes: number[]
  backupBeforeShutdown: boolean
  skipIfPlayersOnline: boolean
  nextShutdownAt?: string
  nextStartupAt?: string
  lastShutdownAt?: string
  lastStartupAt?: string
  lastStatus?: string
  lastMessage?: string
  createdAt?: string
  updatedAt?: string
}

export type RestartScheduleUpdate = Pick<
  RestartSchedule,
  | 'enabled'
  | 'shutdownTime'
  | 'startupTime'
  | 'timezone'
  | 'warningMinutes'
  | 'backupBeforeShutdown'
  | 'skipIfPlayersOnline'
>

export type RestartScheduleResult = {
  schedule: RestartSchedule
}

export type UploadPreviewResult = {
  token: string
  preview: SaveInfo
  saveName: string
}

export type InviteCodeResult = {
  inviteCode: string
}

export type PublicIPResult = {
  ip: string
  checkedAt: string
  source?: string
  cached: boolean
}

export type LifecycleJobResponse = {
  jobId: string
  saveName?: string
}

export type InstanceVNCConfig = {
  vncPort: string
}

export type InstanceServerPasswordConfig = {
  serverPassword: string
}

export type ServerRuntimeSettings = {
  cabinStrategy: string          // "CabinStack"|"FarmhouseStack"|"None"
  existingCabinBehavior: string  // "KeepExisting"|"MoveToStack"
  networkBroadcastPeriod: number // 1-10
}

export type InstancePasswordStatus = {
  enabled: boolean
  authenticatedCount: number
  pendingCount: number
  timeoutSeconds: number
  maxAttempts: number
}

export type InstanceRenderingResult = {
  fps: number
  output?: string
}

export type ModSyncKind = 'server_only' | 'client_required' | 'unknown'

export type ModDependency = {
  uniqueId: string
  minimumVersion?: string
  required: boolean
  installed: boolean
  enabled: boolean
  installedVersion?: string
  satisfied: boolean
  status?: string
}

export type ModInfo = {
  id: string
  uniqueId?: string
  name?: string
  version?: string
  author?: string
  description?: string
  folderName: string
  parseError?: string
  enabled: boolean
  canToggle?: boolean
  enableNote?: string
  syncKind: ModSyncKind
  syncNote?: string
  builtIn?: boolean
  nexusSummary?: string
  updatedAt?: string
  endorsementCount?: number
  downloadCount?: number
  pictureUrl?: string
  nexusUrl?: string
  updateKeys?: string[]
  nexusModId?: number
  isContentPack?: boolean
  contentPackFor?: string
  originSource?: string
  originNexusModId?: number
  originModName?: string
  originModUrl?: string
  dependencies?: ModDependency[]
}

export type ModsListResult = {
  mods: ModInfo[]
  restartRequired?: boolean
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
  installedEnabled: boolean
  installedFolderName?: string
  installedVersion?: string
  requiredMods?: NexusRequiredMod[]
}

export type NexusRequiredMod = {
  modId: number
  name: string
  notes?: string
  nexusUrl: string
  installed: boolean
  installedEnabled: boolean
  installedFolderName?: string
  installedVersion?: string
}

export type NexusModSearchResponse = {
  query: string
  results: NexusModSearchResult[]
  page: number
  pageSize: number
  total: number
  hasMore: boolean
}

export type ModSource = 'nexus' | 'nexus_package' | 'local' | 'builtin'

export type ModInstallMethod = 'none' | 'nexus_premium' | 'nexus_extension' | 'manual'

export type ModSearchResult = {
  id: string
  source: ModSource
  sourceName: string
  sourceModId?: string
  sourceDetail?: string
  name: string
  summary?: string
  author?: string
  version?: string
  updatedAt?: string
  endorsementCount?: number
  downloadCount?: number
  pictureUrl?: string
  pageUrl?: string
  externalLabel: string
  installMethod: ModInstallMethod
  installLabel: string
  nexusModId?: number
  installed: boolean
  installedEnabled?: boolean
  installedFolderName?: string
  installedVersion?: string
}

export type NexusSettingsStatus = {
  configured: boolean
  last4?: string
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
