import type {
  CatalogResponse,
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
  NewGameConfig,
  PrepareResponse,
  PreflightResult,
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

export function getCustomNewGameCatalog(instanceId = defaultInstanceId) {
  return request<CatalogResponse>(
    `/api/instances/${encodeURIComponent(instanceId)}/custom-new-game/catalog`,
  )
}

export function refreshCustomNewGameCatalog(instanceId = defaultInstanceId) {
  return request<CatalogResponse>(
    `/api/instances/${encodeURIComponent(instanceId)}/custom-new-game/catalog`,
    { method: 'POST' },
  )
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
