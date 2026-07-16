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

export type ControlCommandStatus = 'queued' | 'running' | 'succeeded' | 'dispatched' | 'failed' | 'expired' | 'unknown'

export type ControlCommand = {
  commandId: string
  instanceId: string
  commandType: string
  targetType?: string
  targetId?: string
  targetLabel?: string
  actorUserId?: number
  actorUsername?: string
  status: ControlCommandStatus
  resultSupported: boolean
  errorCode?: string
  resultMessage?: string
  resultDetails?: Record<string, string>
  submittedAt: string
  completedAt?: string
  updatedAt: string
  importedAt?: string
}

export type ControlCommandsResponse = { commands: ControlCommand[] }

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

export type JunimoUpdateStatus =
  | 'up_to_date'
  | 'update_available'
  | 'not_installed'
  | 'custom_images'
  | 'invalid_config'
	| 'withdrawn'
	| 'not_recommended'

export type CompatibilityMatrixStatus = 'recommended' | 'withdrawn'

export type JunimoRuntimeComponent = {
  image?: string
  tag?: string
}

export type JunimoRecommendedComponent = {
  tag: string
  images: string[]
  digests: Record<string, string>
  upstreamRef?: string
  sourceRevision?: string
  image?: string
  trustedCandidates?: string[]
}

export type JunimoUpdateInfo = {
  available: boolean
  supported: boolean
  repairable: boolean
  repairCode?: string
  repairReason?: string
  status: JunimoUpdateStatus
  code: string
  reason: string
  current: {
    server: JunimoRuntimeComponent
    steamAuth: JunimoRuntimeComponent
  }
  recommended: {
    stackVersion: string
		channel: 'stable' | 'preview'
		status: CompatibilityMatrixStatus
		withdrawal?: { reason: string; fallbackStackVersion: string; withdrawnAt?: string }
    minimumPanelVersion: string
		runtimeUpdatePolicy: 'recommended' | 'required'
    server: JunimoRecommendedComponent
    steamAuth: JunimoRecommendedComponent
    releaseNotes: string[]
    tested: boolean
  }
  releaseNotes: string[]
  serverRunning: boolean
  steamAuthLoggedIn: boolean
}

export type JunimoConfigRepairResult = JunimoUpdateInfo & {
  repaired: boolean
  backupId: string
}

export type JunimoUpdateDryRunPhase =
  | 'idle'
  | 'starting'
  | 'checking'
  | 'pulling_server'
  | 'pulling_auth'
  | 'validating_compose'
  | 'succeeded'
  | 'failed'
  | 'unsupported'

export type JunimoUpdateDryRunStatus = {
  dryRunId?: string
  jobId?: string
  phase: JunimoUpdateDryRunPhase
  progress: number
  download?: { component: string; image?: string; doneLayers: number; totalLayers: number; percent: number }
  current: { server: JunimoRuntimeComponent; steamAuth: JunimoRuntimeComponent }
  target: JunimoUpdateInfo['recommended']
  selected: {
    server: { image?: string; digest?: string; imageId?: string }
    steamAuth: { image?: string; digest?: string; imageId?: string }
  }
  checks: Array<{ name: string; status: 'ok' | 'warning' | 'error'; message: string }>
  warnings: string[]
  logs: Array<{ at: string; level: 'info' | 'warning' | 'error'; message: string }>
  serverRunning: boolean
  errorCode?: string
  error?: string
  startedAt?: string
  updatedAt?: string
  finishedAt?: string
}

export type JunimoUpdateApplyPhase =
  | 'idle' | 'checking' | 'pulling' | 'backing_up' | 'stopping' | 'writing_config'
  | 'recreating_auth' | 'verifying_auth' | 'recreating_server' | 'verifying_server'
  | 'restoring_state' | 'succeeded' | 'rolling_back' | 'failed_rolled_back' | 'rollback_failed'

export type JunimoUpdateApplyStatus = {
  applyId?: string
  jobId?: string
  phase: JunimoUpdateApplyPhase
  progress: number
  download?: JunimoUpdateDryRunStatus['download']
  current: JunimoUpdateDryRunStatus['current']
  target: JunimoUpdateInfo['recommended']
  selected: JunimoUpdateDryRunStatus['selected']
  checks: JunimoUpdateDryRunStatus['checks']
  warnings: string[]
  logs: JunimoUpdateDryRunStatus['logs']
  serverWasRunning: boolean
  serverRunning: boolean
  errorCode?: string
  error?: string
  causeCode?: string
  causeError?: string
  rollbackCode?: string
  rollbackError?: string
  manualAction?: string
  startedAt?: string
  updatedAt?: string
  finishedAt?: string
}

export type RuntimeComponentsStatus =
  | 'up_to_date' | 'update_available' | 'game_missing' | 'sdk_missing' | 'manifest_invalid' | 'custom_or_unknown'

export type RuntimeContentComponent = {
  appId: string
  buildId?: string
  stateFlags?: string
  installDir?: string
  lastUpdated?: string
  manifestPath: string
  status: RuntimeComponentsStatus
  code: string
  reason: string
}

export type RuntimeContentRecommendation = {
  appId: string
  buildId: string
  manifestVersion: string
  notes: string[]
  estimatedDownloadBytes: number
}

export type SMAPIUpdateStatus =
  | 'up_to_date'
  | 'update_available'
  | 'missing'
  | 'invalid'
  | 'incompatible_game'
  | 'incompatible_junimo'
  | 'custom_or_unknown'

export type SMAPIUpdateInfo = {
  available: boolean
  supported: boolean
  status: SMAPIUpdateStatus
  code: string
  reason: string
  current: {
    version?: string
    configuredVersion?: string
    versionSource?: string
    present: boolean
    requiredFiles: boolean
    gameDataVolume?: string
  }
  recommended: {
    version: string
    downloadUrl: string
    sha256: string
    archiveBytes: number
    compatibility: {
      gameBuildId: string
      sdkBuildId: string
      junimoVersion: string
      steamAuthVersion: string
      controlVersion: string
      controlDllSha256: string
      commandResultVersion: number
    }
  }
  detectedAt: string
}

export type SMAPIUpdatePhase =
  | 'idle' | 'checking' | 'downloading' | 'validating_archive' | 'creating_staging'
  | 'cloning' | 'installing' | 'verifying_staging' | 'stopping' | 'switching'
  | 'starting' | 'verifying_stack' | 'restoring_state' | 'succeeded'
  | 'rolling_back' | 'failed_rolled_back' | 'rollback_failed' | 'failed'

export type SMAPIUpdateWorkflowStatus = {
  updateId?: string
  jobId?: string
  phase: SMAPIUpdatePhase
  progress: number
  current: SMAPIUpdateInfo['current']
  target: SMAPIUpdateInfo['recommended']
  checks: Array<{ name: string; status: 'ok' | 'warning' | 'error'; message: string }>
  warnings: string[]
  logs: Array<{ at: string; level: 'info' | 'warning' | 'error'; message: string }>
  serverWasRunning: boolean
  requiredBytes?: number
  freeBytes?: number
  errorCode?: string
  error?: string
  manualAction?: string
  startedAt?: string
  updatedAt?: string
  finishedAt?: string
}

export type RuntimeComponentsInfo = {
  available: boolean
  supported: boolean
  status: RuntimeComponentsStatus
  code: string
  reason: string
  current: { game: RuntimeContentComponent; sdk: RuntimeContentComponent }
  recommended: {
		stackVersion: string
		channel: 'stable' | 'preview'
		status: CompatibilityMatrixStatus
		minimumPanelVersion: string
		runtimeUpdatePolicy: 'recommended' | 'required'
    game: RuntimeContentRecommendation
    sdk: RuntimeContentRecommendation
    tested: boolean
    releaseNotes: string[]
  }
  detectedAt: string
  smapi?: SMAPIUpdateInfo
}

export type RuntimeComponentsPreflight = {
  phase: 'idle' | 'checking' | 'succeeded' | 'failed'
  progress: number
  target: RuntimeComponentsInfo['recommended']
  checks: Array<{ name: string; status: 'ok' | 'warning' | 'error'; message: string }>
  warnings: string[]
  requiredBytes: number
  freeBytes?: number
  gameDataBytes?: number
  errorCode?: string
  error?: string
  updatedAt?: string
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
  uiStatus?: 'stopped' | 'starting_container' | 'loading_save' | 'waiting_for_host' | 'ready' | 'stopping' | 'failed'
  uiStatusUpdatedAt?: string
  statusSource?: { state?: string; saveId?: string; updatedAt?: string }
  playersSource?: { saveId?: string; updatedAt?: string; players?: Array<{ isHost?: boolean; status?: string }> }
  runtimeDiagnostic?: {
    activeSaveId?: string; saveDirectory?: string; cacheSaveId?: string; cacheMatchesActive: boolean
    controlModVersion?: string; expectedControlModVersion: string; controlModMatches: boolean
    junimoStackVersion: string
    junimoUpdateStatus: JunimoUpdateStatus
    junimoUpdateCode: string
    junimoUpdateReason: string
    junimoUpdateSupported: boolean
    serverVersion?: string; expectedServerVersion: string
    steamAuthVersion?: string; expectedSteamAuthVersion: string
    junimoVersionMatches: boolean
    containerToSaveMs?: number; saveToHostMs?: number
    commandProtocol?: {
      commandResultVersion: number
      pendingCommandCount: number
      unimportedResultCount: number
      oldestPendingAt?: string
      lastControlModConsumeAt?: string
      commandsWritable: boolean
      commandResultsWritable: boolean
      warnings?: string[]
    }
  }
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
  isAuthenticated?: boolean | null
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
  farmTypeLabel?: string
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

export type FarmTypeCatalogCondition = {
  kind: string
  when: unknown
}

export type NewGameModComponent = {
  key: string
  uniqueId?: string
  folderName: string
  name?: string
  version?: string
  packageKey?: string
  enabled: boolean
  provider: boolean
}

export type NewGameModSelection = {
  farmTypeId: string
  providerModKey?: string
  providerModId?: string
  providerName?: string
  providerVersion?: string
  requiredModKeys: string[]
  optionalDependencyKeys: string[]
  enabledModKeys: string[]
  disabledRequiredModKeys: string[]
  missingRequiredModKeys: string[]
  conflictingProviderModKeys: string[]
  components: NewGameModComponent[]
  changedModKeys?: string[]
  warnings: string[]
  readiness: 'ready' | 'needs_enable' | 'missing_required' | 'conflict'
  dependenciesReady: boolean
}

export type FarmTypeCatalogItem = {
  id: string
  label: string
  description: string
  kind: 'builtin' | 'modded'
  providerModId?: string
  providerName?: string
  providerVersion?: string
  providerFolder?: string
  enabled: boolean
  confidence: string
  conditions: FarmTypeCatalogCondition[]
  conflict: boolean
  dependenciesReady: boolean | null
  selectable: boolean
  requiresRuntimeValidation: boolean
  iconUrl?: string
  warnings: string[]
  modSelection?: NewGameModSelection
}

export type FarmTypeCatalogResponse = {
  farmTypes: FarmTypeCatalogItem[]
  catalogWarnings: string[]
  moddedCreationEnabled: boolean
}

export type SavesListResult = {
  saves: SaveInfo[]
  activeSaveName: string
}

export type BackupInfo = {
  name: string
  saveName: string
  kind: 'auto' | 'manual' | 'predelete' | 'prerestore' | 'latest' | 'daily' | 'scheduled' | string
  size: number
  createdAt: string
  farmerName?: string
  farmName?: string
  gameYear?: number
  gameSeason?: string
  gameDay?: number
  gameDayOrdinal?: number
  farmType?: string
  fileSizeBytes?: number
  parseError?: string
}

export type BackupPolicy = {
  gameSaveBackups: boolean
  retainGameDays: number
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
  // Present when the server was stopped, restored, and started synchronously
  // (no restart needed).
  saveName?: string
  // Present when the server was running: the restore was submitted as an
  // async stop -> restore -> start job, tracked like any other lifecycle job.
  jobId?: string
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

export type SaveImportHostHandling =
  | { mode: 'swap_to_player'; platformId: string }
  | { mode: 'virtual_host_takeover'; acknowledged: true }

export type SaveImportJobResponse = {
  jobId: string
  operationId: string
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
  passwordBridgeAvailable?: boolean
  passwordBridgeDetail?: string
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
  packageKey?: string
  packageName?: string
  dependencies?: ModDependency[]
}

export type GameLanguageSettings = {
  languageCode: string
}

export type ModsListResult = {
  mods: ModInfo[]
  restartRequired?: boolean
  upload?: ModUploadSummary
  compatibilityWarnings?: ModCompatibilityWarning[]
}

export type ModUploadSummary = {
  archiveCount: number
  discoveredCount: number
  importedCount: number
  enabledCount: number
  skippedBuiltInCount?: number
  skippedBuiltInNames?: string[]
  activeSaveName?: string
}

export type ModCompatibilityWarning = {
  code: string
  severity: 'warning' | 'error' | string
  title: string
  message: string
  saveName?: string
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
  commandId?: string
  status?: CommandResultStatus
  output?: string
  error?: string
  exitCode: number
  durationMs: number
}

export type CommandResultStatus =
  | 'queued'
  | 'running'
  | 'succeeded'
  | 'failed'
  | 'dispatched'
  | 'expired'
  | 'unknown'

export type CommandOutcome = {
  commandId: string
  status: CommandResultStatus
  errorCode?: string
  message?: string
  createdAt?: string
  updatedAt: string
  details?: Record<string, string>
}
