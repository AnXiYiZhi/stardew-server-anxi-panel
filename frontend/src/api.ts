import type {
  CommandRunResult,
  CommandsListResult,
  ComposeLogsResponse,
  ComposePsResponse,
  DockerStatusResponse,
  Instance,
  InstallJobResponse,
  InstallOptionsResponse,
  InstanceState,
  InstancesResponse,
  InviteCodeResult,
  JobLogsResponse,
  JobResponse,
  JobsResponse,
  LifecycleJobResponse,
  ModsListResult,
  NewGameConfig,
  PrepareResponse,
  PreflightResult,
  SavesListResult,
  UploadPreviewResult,
} from './types'

export const defaultInstanceId = 'stardew'

export class ApiError extends Error {
  code: string
  status: number

  constructor(status: number, code: string, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }
}

type RequestOptions = Omit<RequestInit, 'body'> & {
  body?: unknown
}

export async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { body, headers: optionHeaders, ...rest } = options
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
    throw await toApiError(response)
  }

  if (response.status === 204) {
    return undefined as T
  }

  return (await response.json()) as T
}

export function getDockerStatus() {
  return request<DockerStatusResponse>('/api/docker/status')
}

export function getComposePs(instanceId = defaultInstanceId) {
  return request<ComposePsResponse>(`/api/instances/${encodeURIComponent(instanceId)}/docker/ps`)
}

export function getComposeLogs(service = '', tail = 100) {
  const params = new URLSearchParams()
  if (service) {
    params.set('service', service)
  }
  params.set('tail', String(tail))
  return request<ComposeLogsResponse>(`/api/docker/logs?${params.toString()}`)
}

export function getInstances() {
  return request<InstancesResponse>('/api/instances')
}

export function getInstance(instanceId = defaultInstanceId) {
  return request<Instance>(`/api/instances/${encodeURIComponent(instanceId)}`)
}

export function getInstanceState(instanceId = defaultInstanceId) {
  return request<InstanceState>(`/api/instances/${encodeURIComponent(instanceId)}/state`)
}

export function getJobs() {
  return request<JobsResponse>('/api/jobs')
}

export function clearJobs() {
  return request<{ ok: boolean; deleted: number }>('/api/jobs', { method: 'DELETE' })
}

export function getJob(id: string) {
  return request<JobResponse>(`/api/jobs/${encodeURIComponent(id)}`)
}

export function getJobLogs(id: string, after = 0) {
  const params = new URLSearchParams()
  params.set('after', String(after))
  return request<JobLogsResponse>(`/api/jobs/${encodeURIComponent(id)}/logs?${params.toString()}`)
}

export function startTestJob() {
  return request<JobResponse>('/api/jobs/test', { method: 'POST' })
}

export function startFailingTestJob() {
  return request<JobResponse>('/api/jobs/test-fail', { method: 'POST' })
}

export function getStardewState() {
  return getInstanceState(defaultInstanceId)
}

export function prepareInstance(instanceId = defaultInstanceId) {
  return request<PrepareResponse>(`/api/instances/${encodeURIComponent(instanceId)}/prepare`, { method: 'POST' })
}

export function getInstallOptions(instanceId = defaultInstanceId) {
  return request<InstallOptionsResponse>(`/api/instances/${encodeURIComponent(instanceId)}/install-options`)
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

export function getSavesPreflight(instanceId = defaultInstanceId) {
  return request<PreflightResult>(`/api/instances/${encodeURIComponent(instanceId)}/saves/preflight`)
}

export function createNewGame(config: NewGameConfig, instanceId = defaultInstanceId) {
  return request<LifecycleJobResponse>(
    `/api/instances/${encodeURIComponent(instanceId)}/saves/custom-new-game`,
    { method: 'POST', body: config },
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
  cancel = false,
  instanceId = defaultInstanceId,
) {
  return request<LifecycleJobResponse>(
    `/api/instances/${encodeURIComponent(instanceId)}/saves/upload-commit-and-start`,
    { method: 'POST', body: { token, cancel } },
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

export function getSaves(instanceId = defaultInstanceId) {
  return request<SavesListResult>(`/api/instances/${encodeURIComponent(instanceId)}/saves`)
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

export function getMods(instanceId = defaultInstanceId) {
  return request<ModsListResult>(`/api/instances/${encodeURIComponent(instanceId)}/mods`)
}

export function uploadMod(file: File, instanceId = defaultInstanceId) {
  const form = new FormData()
  form.append('mod', file)
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

export function getCommands(instanceId = defaultInstanceId) {
  return request<CommandsListResult>(`/api/instances/${encodeURIComponent(instanceId)}/commands`)
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

// ── Version ──────────────────────────────────────────────────────────────────

export interface VersionInfo {
  version: string
  commit?: string
  buildDate?: string
}

export function getVersion() {
  return request<VersionInfo>('/api/version')
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

export function getHealthDiagnostics() {
  return request<HealthDiagnosticsResponse>('/api/health/diagnostics')
}

async function toApiError(response: Response): Promise<ApiError> {
  try {
    const payload = (await response.json()) as {
      error?: { code?: string; message?: string }
    }
    return new ApiError(
      response.status,
      payload.error?.code ?? 'request_failed',
      payload.error?.message ?? '请求失败',
    )
  } catch {
    return new ApiError(response.status, 'request_failed', '请求失败')
  }
}
