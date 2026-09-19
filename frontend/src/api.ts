import { currentSessionGeneration, markSessionAuthenticated, notifySessionExpired } from './auth-session-events.ts'
import { clearReadRequests, peekReadRequest, sharedRead } from './core/read-requests.ts'
import type {
  CommandRunResult,
  CommandsListResult,
  CommandOutcome,
  ControlCommandsResponse,
  ComposePsResponse,
  CreateInstanceRequest,
  CreateInstanceResponse,
  GameInstallation,
  Instance,
  InstallJobResponse,
  InstanceVNCConfig,
  InstancePlayerAuthConfig,
  UpdateInstancePlayerAuthConfig,
  InstancePasswordStatus,
  ServerRuntimeSettings,
  GameLanguageSettings,
  InstanceRenderingResult,
  InstanceState,
  JunimoUpdateInfo,
  JunimoUpdateDryRunStatus,
  JunimoUpdateApplyStatus,
  RuntimeComponentsInfo,
  RuntimeComponentsPreflight,
  SMAPIUpdateInfo,
  SMAPIUpdateWorkflowStatus,
  InstancesResponse,
  InviteCodeResult,
  JobLogsResponse,
  JobResponse,
  JobsResponse,
  LifecycleJobResponse,
  ModInfo,
  ModUpdateCheckResult,
  ModsListResult,
  ModSyncKind,
  NewGameConfig,
  FarmTypeCatalogResponse,
  NewGameModSelection,
  NexusModSearchResponse,
  NexusSettingsStatus,
  OKResponse,
  PanelUser,
  DirectConnectConfig,
  BackupCreateResult,
  BackupPolicy,
  BackupPolicyResult,
  BackupsListResult,
  RestoreBackupResult,
  RestartScheduleResult,
  RestartScheduleUpdate,
  ResourceMetricsResponse,
  ResourceOverview,
  SavesListResult,
  SaveImportHostHandling,
  SaveImportJobResponse,
  StardewPlayersResponse,
  PlayerModDetailsResult,
  UploadPreviewResult,
  UsersResponse,
} from './types'
import { activeInstanceId as defaultInstanceId, setActiveInstanceId } from './instance-id.ts'

export { activeInstanceId as defaultInstanceId } from './instance-id.ts'

// Live binding: every instance API reads the value at call time after App.boot
// consumes the backend's setup status. Invalid values fail closed to the legacy
// ID so a malformed response cannot alter request paths.
export function setDefaultInstanceId(value: unknown): string {
  return setActiveInstanceId(value)
}

export class ApiError extends Error {
  code: string
  status: number
  details?: unknown

  constructor(status: number, code: string, message: string, details?: unknown) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.details = details
  }
}

type RequestOptions = Omit<RequestInit, 'body'> & {
  body?: unknown
  timeoutMs?: number
  maxAge?: number
}

export function invalidateReadCache() { clearReadRequests() }

export function peekInstanceRead<T>(suffix: string, instanceId = defaultInstanceId): T | undefined {
  return peekReadRequest<T>(`${currentSessionGeneration()}:/api/instances/${encodeURIComponent(instanceId)}/${suffix}`)
}

function displayCacheAge(path: string) {
  if (/\/(runtime-components|smapi-update|junimo-update)$/.test(path)) return 30_000
  if (/\/(health\/diagnostics|docker\/ps)$/.test(path)) return 10_000
  if (/\/(metrics|resources)$/.test(path)) return 1_000
  return 0
}

export function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const sessionGeneration = currentSessionGeneration()
  const method = (options.method ?? 'GET').toUpperCase()
  if (method === 'GET') {
    // Custom headers/cache policies are separate representations. Only ordinary
    // JSON reads participate in the display cache.
    const suffix = options.headers || options.cache ? `:${JSON.stringify([Array.from(new Headers(options.headers)), options.cache])}` : ''
    const key = `${sessionGeneration}:${path}${suffix}`
    return sharedRead(key, signal => performRequest<T>(path, { ...options, signal }, sessionGeneration), {
      signal: options.signal, timeoutMs: options.timeoutMs,
      maxAge: options.cache === 'no-store' ? 0 : options.maxAge ?? displayCacheAge(path),
    })
  }
  clearReadRequests()
  return performRequest<T>(path, options).finally(clearReadRequests)
}

async function performRequest<T>(path: string, options: RequestOptions = {}, sessionGeneration = currentSessionGeneration()): Promise<T> {
  const { body, headers: optionHeaders, timeoutMs: _timeout, maxAge: _maxAge, ...rest } = options
  const headers = new Headers(optionHeaders)
  const init: RequestInit = {
    ...rest,
    headers,
    credentials: 'include',
  }

  if (body !== undefined) {
    headers.set('Content-Type', 'application/json')
    init.body = JSON.stringify(body)
  }

  const response = await fetch(path, init)
  if (!response.ok) {
    if (response.status === 401 && !path.startsWith('/api/auth/') && !path.startsWith('/api/setup/')) {
      notifySessionExpired(sessionGeneration)
    }
    throw await toApiError(response)
  }

  if (path === '/api/auth/login' || path === '/api/setup/admin') markSessionAuthenticated()

  if (response.status === 204) {
    return undefined as T
  }

  return (await response.json()) as T
}

export function getComposePs(instanceId = defaultInstanceId, signal?: AbortSignal) {
  return request<ComposePsResponse>(`/api/instances/${encodeURIComponent(instanceId)}/docker/ps`, { signal })
}

export function getInstances() {
  return request<InstancesResponse>('/api/instances')
}

export function createInstance(body: CreateInstanceRequest) {
  return request<CreateInstanceResponse>('/api/instances', { method: 'POST', body })
}

export function renameInstance(instanceId: string, name: string) {
  return request<{ instance: Instance }>(`/api/instances/${encodeURIComponent(instanceId)}`, { method: 'PATCH', body: { name } })
}

export function deleteInstance(instanceId: string) {
  return request<void>(`/api/instances/${encodeURIComponent(instanceId)}`, { method: 'DELETE' })
}

export function getStardewGameInstallation() {
  return request<GameInstallation>('/api/games/stardew/installation')
}

export function getInstanceState(instanceId = defaultInstanceId, signal?: AbortSignal) {
  return request<InstanceState>(`/api/instances/${encodeURIComponent(instanceId)}/state`, { signal })
}

export function getJunimoUpdate(instanceId = defaultInstanceId, signal?: AbortSignal) {
  return request<JunimoUpdateInfo>(`/api/instances/${encodeURIComponent(instanceId)}/junimo-update`, { signal })
}

export function getJunimoUpdateDryRun(instanceId = defaultInstanceId, signal?: AbortSignal) {
  return request<JunimoUpdateDryRunStatus>(`/api/instances/${encodeURIComponent(instanceId)}/junimo-update/dry-run`, { signal })
}

export function startJunimoUpdateDryRun(instanceId = defaultInstanceId) {
  return request<JunimoUpdateDryRunStatus>(`/api/instances/${encodeURIComponent(instanceId)}/junimo-update/dry-run`, { method: 'POST' })
}

export function getJunimoUpdateApply(instanceId = defaultInstanceId, signal?: AbortSignal) {
  return request<JunimoUpdateApplyStatus>(`/api/instances/${encodeURIComponent(instanceId)}/junimo-update/apply`, { signal })
}

export function startJunimoUpdateApply(instanceId = defaultInstanceId) {
  return request<JunimoUpdateApplyStatus>(`/api/instances/${encodeURIComponent(instanceId)}/junimo-update/apply`, {
    method: 'POST',
    body: { confirm: true },
  })
}

export function startJunimoUpdateRepair(instanceId = defaultInstanceId) {
  return request<JunimoUpdateApplyStatus>(`/api/instances/${encodeURIComponent(instanceId)}/junimo-update/repair`, {
    method: 'POST',
    body: { confirm: true },
  })
}

export function getRuntimeComponents(instanceId = defaultInstanceId, signal?: AbortSignal) {
  return request<RuntimeComponentsInfo>(`/api/instances/${encodeURIComponent(instanceId)}/runtime-components`, { signal })
}

export function getRuntimeComponentsPreflight(instanceId = defaultInstanceId, signal?: AbortSignal) {
  return request<RuntimeComponentsPreflight>(`/api/instances/${encodeURIComponent(instanceId)}/runtime-components/dry-run`, { signal })
}

export function startRuntimeComponentsPreflight(instanceId = defaultInstanceId) {
  return request<RuntimeComponentsPreflight>(`/api/instances/${encodeURIComponent(instanceId)}/runtime-components/dry-run`, { method: 'POST' })
}

export function getSMAPIUpdate(instanceId = defaultInstanceId, signal?: AbortSignal) {
  return request<SMAPIUpdateInfo>(`/api/instances/${encodeURIComponent(instanceId)}/smapi-update`, { signal })
}

export function getSMAPIUpdateDryRun(instanceId = defaultInstanceId, signal?: AbortSignal) {
  return request<SMAPIUpdateWorkflowStatus>(`/api/instances/${encodeURIComponent(instanceId)}/smapi-update/dry-run`, { signal })
}

export function startSMAPIUpdateDryRun(instanceId = defaultInstanceId) {
  return request<SMAPIUpdateWorkflowStatus>(`/api/instances/${encodeURIComponent(instanceId)}/smapi-update/dry-run`, { method: 'POST' })
}

export function getSMAPIUpdateApply(instanceId = defaultInstanceId, signal?: AbortSignal) {
  return request<SMAPIUpdateWorkflowStatus>(`/api/instances/${encodeURIComponent(instanceId)}/smapi-update/apply`, { signal })
}

export function startSMAPIUpdateApply(instanceId = defaultInstanceId) {
  return request<SMAPIUpdateWorkflowStatus>(`/api/instances/${encodeURIComponent(instanceId)}/smapi-update/apply`, {
    method: 'POST',
    body: { confirm: true },
  })
}

export function getInstanceMetrics(instanceId = defaultInstanceId, signal?: AbortSignal) {
  return request<ResourceMetricsResponse>(`/api/instances/${encodeURIComponent(instanceId)}/metrics`, { signal })
}

export function getResourceOverview(signal?: AbortSignal) {
  return request<ResourceOverview>('/api/resources', { signal })
}

export function getInstancePlayers(instanceId = defaultInstanceId, signal?: AbortSignal) {
  return request<StardewPlayersResponse>(`/api/instances/${encodeURIComponent(instanceId)}/players`, { signal })
}

export function getPlayerModDetails(
  uniqueMultiplayerId: string,
  instanceId = defaultInstanceId,
  signal?: AbortSignal,
) {
  return request<PlayerModDetailsResult>(
    `/api/instances/${encodeURIComponent(instanceId)}/players/${encodeURIComponent(uniqueMultiplayerId)}/mods`,
    { signal },
  )
}

export function getJobs(signal?: AbortSignal) {
  return request<JobsResponse>('/api/jobs', { signal })
}

export function clearJobs() {
  return request<{ ok: boolean; deleted: number }>('/api/jobs', { method: 'DELETE' })
}

export function clearJobErrorLogs() {
  return request<{ ok: boolean; deleted: number; messagesCleared: number }>('/api/jobs/error-logs', { method: 'DELETE' })
}

export function getJob(id: string) {
  return request<JobResponse>(`/api/jobs/${encodeURIComponent(id)}`)
}

export function getLatestJobLogs(id: string, limit = 1000) {
  const params = new URLSearchParams()
  params.set('latest', 'true')
  params.set('limit', String(Math.min(limit, 1000)))
  return request<JobLogsResponse>(`/api/jobs/${encodeURIComponent(id)}/logs?${params.toString()}`)
}

export function getStardewState(instanceId = defaultInstanceId, signal?: AbortSignal) {
  return getInstanceState(instanceId, signal)
}

export function installInstance(
  body: {
    steamUsername?: string
    steamPassword?: string
    vncPassword?: string
    imageTag?: string
    reuseCredentials?: boolean
  },
  instanceId = defaultInstanceId,
) {
  return request<InstallJobResponse>(`/api/instances/${encodeURIComponent(instanceId)}/install`, {
    method: 'POST',
    body,
  })
}

// steamAuthLogin starts a one-shot authorization only when no valid SteamAuth
// session is already ready. It stops after saving the invite-code session and
// never downloads the game. The server must be stopped for a new authorization.
export function steamAuthLogin(instanceId = defaultInstanceId) {
  return request<InstallJobResponse>(`/api/instances/${encodeURIComponent(instanceId)}/steam-auth/login`, {
    method: 'POST',
    body: {},
  })
}

export function submitSteamGuardInput(
  jobId: string,
  input: string,
  instanceId = defaultInstanceId,
) {
  return request<{ ok: boolean }>(`/api/instances/${encodeURIComponent(instanceId)}/steam-guard/input`, {
    method: 'POST',
    body: { jobId, input },
  })
}

export function createJobEventSource(id: string, after = 0) {
  const params = new URLSearchParams()
  if (after > 0) {
    params.set('after', String(after))
  }
  const query = params.toString()
  return new EventSource(`/api/jobs/${encodeURIComponent(id)}/stream${query ? `?${query}` : ''}`, {
    withCredentials: true,
  })
}

export function createNewGame(config: NewGameConfig, requestId: string, instanceId = defaultInstanceId) {
  return request<LifecycleJobResponse>(
    `/api/instances/${encodeURIComponent(instanceId)}/saves/custom-new-game`,
    { method: 'POST', body: config, headers: { 'Idempotency-Key': requestId } },
  )
}

export function uploadSavePreview(file: File, instanceId = defaultInstanceId) {
  const form = new FormData()
  form.append('save', file)
  return fetch(`/api/instances/${encodeURIComponent(instanceId)}/saves/upload-preview`, {
    method: 'POST',
    body: form,
    credentials: 'include',
  }).then(async (res) => {
    if (!res.ok) throw await toApiError(res)
    return (await res.json()) as UploadPreviewResult
  })
}

export function uploadSaveCommitAndStart(
  token: string,
  hostHandling: SaveImportHostHandling,
  instanceId = defaultInstanceId,
) {
  return request<SaveImportJobResponse>(
    `/api/instances/${encodeURIComponent(instanceId)}/saves/upload-commit-and-start`,
    { method: 'POST', body: { token, hostHandling } },
  )
}

export function cancelSaveUploadPreview(token: string, instanceId = defaultInstanceId) {
  return request<{ cancelled: boolean }>(
    `/api/instances/${encodeURIComponent(instanceId)}/saves/upload-commit-and-start`,
    { method: 'POST', body: { token, cancel: true } },
  )
}

export function startInstance(instanceId = defaultInstanceId) {
  return request<LifecycleJobResponse>(`/api/instances/${encodeURIComponent(instanceId)}/start`, {
    method: 'POST',
  })
}

export function stopInstance(instanceId = defaultInstanceId) {
  return request<{ ok: boolean }>(`/api/instances/${encodeURIComponent(instanceId)}/stop`, {
    method: 'POST',
  })
}

export function restartInstance(instanceId = defaultInstanceId) {
  return request<{ ok: boolean }>(`/api/instances/${encodeURIComponent(instanceId)}/restart`, {
    method: 'POST',
  })
}

export function getInviteCode(instanceId = defaultInstanceId) {
  return request<InviteCodeResult>(`/api/instances/${encodeURIComponent(instanceId)}/invite-code`)
}

export function getInstanceDirectConnect(instanceId = defaultInstanceId) {
  return request<DirectConnectConfig>(`/api/instances/${encodeURIComponent(instanceId)}/direct-connect`)
}

export function getInstanceVNCConfig(instanceId = defaultInstanceId) {
  return request<InstanceVNCConfig>(`/api/instances/${encodeURIComponent(instanceId)}/config/vnc-port`)
}

export function updateInstanceVNCPort(port: string, instanceId = defaultInstanceId) {
  return request<InstanceVNCConfig>(`/api/instances/${encodeURIComponent(instanceId)}/config/vnc-port`, {
    method: 'PUT',
    body: { port },
  })
}

export function getInstancePlayerAuthConfig(instanceId = defaultInstanceId) {
  return request<InstancePlayerAuthConfig>(`/api/instances/${encodeURIComponent(instanceId)}/config/player-auth`)
}

export function updateInstancePlayerAuthConfig(config: UpdateInstancePlayerAuthConfig, instanceId = defaultInstanceId) {
  return request<InstancePlayerAuthConfig>(`/api/instances/${encodeURIComponent(instanceId)}/config/player-auth`, {
    method: 'PUT',
    body: config,
  })
}

export function getInstancePasswordStatus(instanceId = defaultInstanceId) {
  return request<InstancePasswordStatus>(`/api/instances/${encodeURIComponent(instanceId)}/password-status`)
}

export function getInstanceServerRuntimeSettings(instanceId = defaultInstanceId) {
  return request<ServerRuntimeSettings>(`/api/instances/${encodeURIComponent(instanceId)}/config/server-runtime-settings`)
}

export function updateInstanceServerRuntimeSettings(settings: ServerRuntimeSettings, instanceId = defaultInstanceId) {
  return request<ServerRuntimeSettings>(`/api/instances/${encodeURIComponent(instanceId)}/config/server-runtime-settings`, {
    method: 'PUT',
    body: settings,
  })
}

export function setInstanceRenderingFPS(fps: number, instanceId = defaultInstanceId) {
  return request<InstanceRenderingResult>(`/api/instances/${encodeURIComponent(instanceId)}/rendering`, {
    method: 'POST',
    body: { fps },
  })
}

export function getInstanceRenderingFPS(instanceId = defaultInstanceId) {
  return request<InstanceRenderingResult>(`/api/instances/${encodeURIComponent(instanceId)}/rendering`)
}

export function getSaves(instanceId = defaultInstanceId, signal?: AbortSignal) {
  return request<SavesListResult>(`/api/instances/${encodeURIComponent(instanceId)}/saves`, { signal })
}

export function selectSave(name: string, instanceId = defaultInstanceId) {
  return request<{ activeSaveName: string }>(
    `/api/instances/${encodeURIComponent(instanceId)}/saves/select`,
    { method: 'POST', body: { name } },
  )
}

export function selectSaveAndStart(name: string, instanceId = defaultInstanceId) {
  return request<LifecycleJobResponse>(
    `/api/instances/${encodeURIComponent(instanceId)}/saves/select-and-start`,
    { method: 'POST', body: { name } },
  )
}

export function deleteSave(name: string, instanceId = defaultInstanceId) {
  return request<{ ok: boolean }>(
    `/api/instances/${encodeURIComponent(instanceId)}/saves/${encodeURIComponent(name)}`,
    { method: 'DELETE' },
  )
}

export function exportSave(name: string, instanceId = defaultInstanceId) {
  return fetch(`/api/instances/${encodeURIComponent(instanceId)}/saves/${encodeURIComponent(name)}/export`, {
    method: 'POST',
    credentials: 'include',
  }).then(async (res) => {
    if (!res.ok) throw await toApiError(res)
    const blob = await res.blob()
    const disposition = res.headers.get('Content-Disposition') ?? ''
    const match = disposition.match(/filename=(.+)/)
    const filename = match ? match[1] : `${name}.zip`
    return { blob, filename }
  })
}

export function getSaveBackups(instanceId = defaultInstanceId) {
  return request<BackupsListResult>(`/api/instances/${encodeURIComponent(instanceId)}/saves/backups`)
}

export function createSaveBackup(name: string, instanceId = defaultInstanceId) {
  return request<BackupCreateResult>(
    `/api/instances/${encodeURIComponent(instanceId)}/saves/${encodeURIComponent(name)}/backup`,
    { method: 'POST' },
  )
}

export function updateSaveBackupPolicy(policy: BackupPolicy, instanceId = defaultInstanceId) {
  return request<BackupPolicyResult>(`/api/instances/${encodeURIComponent(instanceId)}/saves/backups/policy`, {
    method: 'PUT',
    body: policy,
  })
}

export function getRestartSchedule(instanceId = defaultInstanceId, signal?: AbortSignal) {
  return request<RestartScheduleResult>(`/api/instances/${encodeURIComponent(instanceId)}/restart-schedule`, { signal })
}

export function updateRestartSchedule(schedule: RestartScheduleUpdate, instanceId = defaultInstanceId) {
  const body: RestartScheduleUpdate = {
    enabled: schedule.enabled,
    shutdownTime: schedule.shutdownTime,
    startupTime: schedule.startupTime,
    timezone: schedule.timezone,
    warningMinutes: schedule.warningMinutes,
    backupBeforeShutdown: schedule.backupBeforeShutdown,
    skipIfPlayersOnline: schedule.skipIfPlayersOnline,
  }

  return request<RestartScheduleResult>(`/api/instances/${encodeURIComponent(instanceId)}/restart-schedule`, {
    method: 'PUT',
    body,
  })
}

export function restoreSaveBackup(backupName: string, overwrite = false, autoRestart = false, instanceId = defaultInstanceId) {
  return request<RestoreBackupResult>(`/api/instances/${encodeURIComponent(instanceId)}/saves/backups/restore`, {
    method: 'POST',
    body: { backupName, overwrite, autoRestart },
  })
}

export function deleteSaveBackup(backupName: string, instanceId = defaultInstanceId) {
  return request<{ ok: boolean }>(
    `/api/instances/${encodeURIComponent(instanceId)}/saves/backups/${encodeURIComponent(backupName)}`,
    { method: 'DELETE' },
  )
}

export function getMods(instanceId = defaultInstanceId, signal?: AbortSignal) {
  return request<ModsListResult>(`/api/instances/${encodeURIComponent(instanceId)}/mods`, { signal })
}

export function getModUpdates(instanceId = defaultInstanceId) {
  return request<ModUpdateCheckResult>(`/api/instances/${encodeURIComponent(instanceId)}/mod-updates`)
}

export function checkModUpdates(instanceId = defaultInstanceId) {
  return request<ModUpdateCheckResult>(`/api/instances/${encodeURIComponent(instanceId)}/mod-updates/check`, {
    method: 'POST',
  })
}

export function uploadMods(files: File[], instanceId = defaultInstanceId) {
  const form = new FormData()
  for (const file of files) {
    form.append('mod', file)
  }
  return fetch(`/api/instances/${encodeURIComponent(instanceId)}/mods/upload`, {
    method: 'POST',
    body: form,
    credentials: 'include',
  }).then(async (res) => {
    if (!res.ok) throw await toApiError(res)
    return (await res.json()) as ModsListResult
  })
}

export function deleteMod(modId: string, instanceId = defaultInstanceId) {
  return request<{ ok: boolean }>(
    `/api/instances/${encodeURIComponent(instanceId)}/mods/${encodeURIComponent(modId)}`,
    { method: 'DELETE' },
  )
}

export function exportMods(instanceId = defaultInstanceId) {
  return fetch(`/api/instances/${encodeURIComponent(instanceId)}/mods/export`, {
    method: 'POST',
    credentials: 'include',
  }).then(async (res) => {
    if (!res.ok) throw await toApiError(res)
    const blob = await res.blob()
    const disposition = res.headers.get('Content-Disposition') ?? ''
    const match = disposition.match(/filename=(.+)/)
    const filename = match ? match[1] : 'stardew-mods.zip'
    return { blob, filename }
  })
}

export function updateModSyncClassification(
  modId: string,
  syncKind: ModSyncKind,
  syncNote?: string,
  instanceId = defaultInstanceId,
) {
  return request<{ mods: ModInfo[]; syncKind: ModSyncKind }>(
    `/api/instances/${encodeURIComponent(instanceId)}/mods/${encodeURIComponent(modId)}/sync-classification`,
    { method: 'PUT', body: { syncKind, syncNote } },
  )
}

export function updateModEnabled(
  modId: string,
  enabled: boolean,
  saveName?: string,
  instanceId = defaultInstanceId,
) {
  return request<{ mods: ModInfo[]; enabled: boolean; saveName: string }>(
    `/api/instances/${encodeURIComponent(instanceId)}/mods/${encodeURIComponent(modId)}/enabled`,
    { method: 'PUT', body: { enabled, saveName } },
  )
}

export function exportModSyncPack(instanceId = defaultInstanceId) {
  return fetch(`/api/instances/${encodeURIComponent(instanceId)}/mods/sync-pack/export`, {
    method: 'POST',
    credentials: 'include',
  }).then(async (res) => {
    if (!res.ok) throw await toApiError(res)
    const blob = await res.blob()
    const disposition = res.headers.get('Content-Disposition') ?? ''
    const match = disposition.match(/filename=(.+)/)
    const filename = match ? match[1] : 'stardew-player-sync-pack.zip'
    return { blob, filename }
  })
}

export function exportModSyncUpdatePack(instanceId = defaultInstanceId) {
  return fetch(`/api/instances/${encodeURIComponent(instanceId)}/mods/sync-pack/export-update`, {
    method: 'POST',
    credentials: 'include',
  }).then(async (res) => {
    if (!res.ok) throw await toApiError(res)
    const blob = await res.blob()
    const disposition = res.headers.get('Content-Disposition') ?? ''
    const match = disposition.match(/filename=(.+)/)
    const filename = match ? match[1] : 'stardew-player-mods-update-pack.zip'
    return { blob, filename }
  })
}

export function downloadNexusInstallerExtension(instanceId = defaultInstanceId) {
  return fetch(`/api/instances/${encodeURIComponent(instanceId)}/mods/nexus/extension/download`, {
    method: 'GET',
    credentials: 'include',
  }).then(async (res) => {
    if (!res.ok) throw await toApiError(res)
    const blob = await res.blob()
    const disposition = res.headers.get('Content-Disposition') ?? ''
    const match = disposition.match(/filename=(.+)/)
    const filename = match ? match[1] : 'anxi-nexus-installer.zip'
    return { blob, filename }
  })
}

export function searchNexusMods(query: string, page = 1, pageSize = 20, instanceId = defaultInstanceId) {
  const params = new URLSearchParams({ q: query, page: String(page), pageSize: String(pageSize) })
  return request<NexusModSearchResponse>(
    `/api/instances/${encodeURIComponent(instanceId)}/mods/nexus/search?${params.toString()}`,
  )
}

export function getNexusSettings() {
  return request<NexusSettingsStatus>('/api/settings/nexus')
}

export function saveNexusAPIKey(apiKey: string) {
  return request<NexusSettingsStatus>('/api/settings/nexus/api-key', {
    method: 'PUT',
    body: { apiKey },
  })
}

export function deleteNexusAPIKey() {
  return request<NexusSettingsStatus>('/api/settings/nexus/api-key', { method: 'DELETE' })
}

export function getCommands(instanceId = defaultInstanceId) {
  return request<CommandsListResult>(`/api/instances/${encodeURIComponent(instanceId)}/commands`)
}

export function getInstanceGameLanguage(instanceId = defaultInstanceId) {
  return request<GameLanguageSettings>(`/api/instances/${encodeURIComponent(instanceId)}/config/game-language`)
}

export function updateInstanceGameLanguage(settings: GameLanguageSettings, instanceId = defaultInstanceId) {
  return request<GameLanguageSettings>(`/api/instances/${encodeURIComponent(instanceId)}/config/game-language`, {
    method: 'PUT',
    body: settings,
  })
}

export function updateAllModsEnabled(
  enabled: boolean,
  saveName?: string,
  instanceId = defaultInstanceId,
) {
  return request<{ mods: ModInfo[]; enabled: boolean; saveName: string; changedCount: number }>(
    `/api/instances/${encodeURIComponent(instanceId)}/mods/enabled`,
    { method: 'PUT', body: { enabled, saveName } },
  )
}

export function getFarmTypeCatalog(instanceId = defaultInstanceId, signal?: AbortSignal) {
  return request<FarmTypeCatalogResponse>(
    `/api/instances/${encodeURIComponent(instanceId)}/saves/farm-types`,
    { signal },
  )
}

export function prepareFarmTypeMods(instanceId: string, farmTypeId: string, signal?: AbortSignal) {
  return request<NewGameModSelection>(
    `/api/instances/${encodeURIComponent(instanceId)}/saves/farm-types/prepare`,
    { method: 'POST', body: JSON.stringify({ farmTypeId }), signal },
  )
}

export function getControlCommands(instanceId = defaultInstanceId, limit = 50, signal?: AbortSignal) {
  return request<ControlCommandsResponse>(`/api/instances/${encodeURIComponent(instanceId)}/control-commands?limit=${limit}`, { signal })
}

export function getCommandOutcome(commandId: string, instanceId = defaultInstanceId) {
  return request<CommandOutcome>(
    `/api/instances/${encodeURIComponent(instanceId)}/commands/${encodeURIComponent(commandId)}`,
  )
}

const COMMAND_TIMEOUT_MS = 40_000 // 40 seconds — backend has 30s, frontend adds margin

export function runCommand(command: string, instanceId = defaultInstanceId) {
  const controller = new AbortController()
  const timer = setTimeout(() => controller.abort(), COMMAND_TIMEOUT_MS)
  return request<CommandRunResult>(
    `/api/instances/${encodeURIComponent(instanceId)}/commands/run`,
    { method: 'POST', body: { command }, signal: controller.signal },
  ).finally(() => clearTimeout(timer))
}

export function sendSay(message: string, instanceId = defaultInstanceId) {
  const controller = new AbortController()
  const timer = setTimeout(() => controller.abort(), COMMAND_TIMEOUT_MS)
  return request<CommandRunResult>(
    `/api/instances/${encodeURIComponent(instanceId)}/commands/say`,
    { method: 'POST', body: { message }, signal: controller.signal },
  ).finally(() => clearTimeout(timer))
}

export function kickPlayer(uniqueMultiplayerId: string, name: string, instanceId = defaultInstanceId) {
  return request<CommandRunResult>(
    `/api/instances/${encodeURIComponent(instanceId)}/players/kick`,
    { method: 'POST', body: { uniqueMultiplayerId, name } },
  )
}

export function warpPlayerHome(uniqueMultiplayerId: string, name: string, instanceId = defaultInstanceId) {
  return request<CommandRunResult>(
    `/api/instances/${encodeURIComponent(instanceId)}/players/warp-home`,
    { method: 'POST', body: { uniqueMultiplayerId, name } },
  )
}

export function approvePlayerAuth(uniqueMultiplayerId: string, instanceId = defaultInstanceId) {
  return request<CommandRunResult>(
    `/api/instances/${encodeURIComponent(instanceId)}/players/approve-auth`,
    { method: 'POST', body: { uniqueMultiplayerId } },
  )
}

export function banPlayer(name: string, uniqueMultiplayerId: string, instanceId = defaultInstanceId) {
  return request<CommandRunResult>(
    `/api/instances/${encodeURIComponent(instanceId)}/players/ban`,
    { method: 'POST', body: { name, uniqueMultiplayerId } },
  )
}

export function triggerFestivalEvent(instanceId = defaultInstanceId) {
  return request<CommandRunResult>(
    `/api/instances/${encodeURIComponent(instanceId)}/festival/event`,
    { method: 'POST' },
  )
}

export function enableJojaRoute(confirm: string, instanceId = defaultInstanceId) {
  return request<CommandRunResult>(
    `/api/instances/${encodeURIComponent(instanceId)}/joja/enable`,
    { method: 'POST', body: { confirm } },
  )
}

export function deleteFarmhand(
  uniqueMultiplayerId: string,
  expectedName: string,
  expectedSaveId: string,
  instanceId = defaultInstanceId,
) {
  return request<InstallJobResponse>(
    `/api/instances/${encodeURIComponent(instanceId)}/players/delete-farmhand`,
    { method: 'POST', body: { uniqueMultiplayerId, expectedName, expectedSaveId, acknowledged: true } },
  )
}

export function requestGameSave(instanceId = defaultInstanceId) {
  return request<CommandRunResult>(
    `/api/instances/${encodeURIComponent(instanceId)}/saves/save-now`,
    { method: 'POST' },
  )
}

// ── Version ──────────────────────────────────────────────────────────────────

export interface VersionInfo {
  version: string
  commit?: string
  buildDate?: string
}

export type PanelUpdateCheckStatus = 'pending' | 'checking' | 'ok' | 'error' | 'unavailable'

export interface PanelUpdateStatus {
  currentVersion: string
  currentCommit: string
  currentBuildDate: string
  latestVersion: string
  updateAvailable: boolean
  releaseUrl: string
  publishedAt: string | null
  checkedAt: string | null
  checkStatus: PanelUpdateCheckStatus
  checkError: string
}

export interface PanelUpdateCapability {
  supported: boolean
  reason: string
  code: string
  composeProject: string
  composeService: string
  composeFile: string
  installDir: string
  currentContainer: string
  currentImage: string
  dataMount: string
  dockerAvailable: boolean
  composeAvailable: boolean
  conversionRequired?: boolean
}

export interface PanelUpdaterLogEntry {
  at: string
  level: 'info' | 'warn' | 'error' | string
  message: string
}

export interface PanelUpdateDryRunStatus {
  id: string
  phase: 'starting' | 'running' | 'succeeded' | 'failed' | 'unsupported'
  targetVersion: string
  targetImage: string
  capability: PanelUpdateCapability
  logs: PanelUpdaterLogEntry[]
  startedAt: string
  updatedAt: string
  finishedAt: string | null
  errorCode: string
  error: string
}

export interface PanelUpdateApplyStatus {
  updateId: string
  phase: 'checking' | 'backing_up' | 'pulling' | 'recreating' | 'waiting_health' | 'succeeded' | 'rolling_back' | 'failed_rolled_back' | 'rollback_failed'
  progress: number
  fromVersion: string
  toVersion: string
  originalImage: string
  originalDigest: string
  selectedImage: string
  selectedDigest: string
  errorCode: string
  error: string
  result: string
  logs: PanelUpdaterLogEntry[]
  startedAt: string
  updatedAt: string
  finishedAt: string | null
  fullStack?: {
    phase: string
    progress: number
    instanceId?: string
    runtimeRequired: boolean
    serverWasRunning?: boolean
    onlinePlayers?: number
    backupName?: string
    errorCode?: string
    error?: string
    result?: string
    updatedAt?: string
    finishedAt?: string
    instances?: Array<{
      instanceId: string
      phase: string
      progress: number
      serverWasRunning?: boolean
      onlinePlayers?: number
      backupName?: string
      errorCode?: string
      error?: string
    }>
  }
}

export function getVersion() {
  return request<VersionInfo>('/api/version')
}

export function getPanelUpdateStatus() {
  return request<PanelUpdateStatus>('/api/system/update')
}

export function checkPanelUpdate() {
  return request<PanelUpdateStatus>('/api/system/update/check', { method: 'POST' })
}

export function getPanelUpdateDryRunStatus() {
  return request<PanelUpdateDryRunStatus>('/api/system/update/dry-run')
}

export function runPanelUpdateDryRun(targetVersion: string) {
  return request<PanelUpdateDryRunStatus>('/api/system/update/dry-run', {
    method: 'POST',
    body: { targetVersion },
  })
}

export function getPanelUpdateApplyStatus() {
  return request<PanelUpdateApplyStatus>('/api/system/update/apply')
}

export function applyPanelUpdate() {
	return request<PanelUpdateApplyStatus>('/api/system/update/apply', { method: 'POST', body: { confirmFullStack: true } })
}

// ── Support Bundle ──────────────────────────────────────────────────────────

export function downloadSupportBundle(instanceId = defaultInstanceId) {
  return fetch(`/api/instances/${encodeURIComponent(instanceId)}/support-bundle`, {
    method: 'POST',
    credentials: 'include',
  }).then(async (res) => {
    if (!res.ok) throw await toApiError(res)
    const blob = await res.blob()
    const disposition = res.headers.get('Content-Disposition') ?? ''
    const match = disposition.match(/filename=(.+)/)
    const filename = match ? match[1] : 'support-bundle.zip'
    return { blob, filename }
  })
}

// ── Users ─────────────────────────────────────────────────────────────────────

export function getUsers() {
  return request<UsersResponse>('/api/users')
}

export function createUser(username: string, password: string, role: string) {
  return request<{ user: PanelUser }>('/api/users', {
    method: 'POST',
    body: { username, password, role },
  })
}

export function updateUserRole(id: number, role: string) {
  return request<{ user: PanelUser }>(`/api/users/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: { role },
  })
}

export function updateUserPassword(id: number, password: string) {
  return request<{ user: PanelUser }>(`/api/users/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: { password },
  })
}

export function disableUser(id: number) {
  return request<OKResponse>(`/api/users/${encodeURIComponent(id)}`, { method: 'DELETE' })
}

export function enableUser(id: number) {
  return request<{ user: PanelUser }>(`/api/users/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: { isActive: true },
  })
}

export function deleteUserHard(id: number) {
  return request<OKResponse>(`/api/users/${encodeURIComponent(id)}?hard=true`, { method: 'DELETE' })
}

// ── Audit Logs ────────────────────────────────────────────────────────────────

export interface AuditLogEntry {
  id: number
  actorUserId: number | null
  actorName: string | null
  action: string
  targetType: string
  targetId: string | null
  metadataJson: string
  ipAddress: string | null
  userAgent: string | null
  createdAt: string
}

export interface AuditLogsResponse {
  logs: AuditLogEntry[]
  total: number
  limit: number
  offset: number
}

export function getAuditLogs(limit = 50, offset = 0) {
  return request<AuditLogsResponse>(`/api/audit-logs?limit=${limit}&offset=${offset}`)
}

// ── Health Diagnostics ────────────────────────────────────────────────────────

export interface HealthCheck {
  name: string
  status: 'ok' | 'warning' | 'error'
  message: string
}

export interface HealthDiagnosticsResponse {
  status: string
  checks: HealthCheck[]
}

export function getHealthDiagnostics(signal?: AbortSignal) {
  return request<HealthDiagnosticsResponse>('/api/health/diagnostics', { signal })
}

async function toApiError(response: Response): Promise<ApiError> {
  try {
    const payload = (await response.json()) as {
      error?: { code?: string; message?: string; details?: unknown }
    }
    return new ApiError(
      response.status,
      payload.error?.code ?? 'request_failed',
      payload.error?.message ?? '请求失败',
      payload.error?.details,
    )
  } catch {
    return new ApiError(response.status, 'request_failed', '请求失败')
  }
}
