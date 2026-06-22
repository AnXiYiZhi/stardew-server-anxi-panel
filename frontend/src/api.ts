import type {
  ComposeLogsResponse,
  ComposePsResponse,
  DockerStatusResponse,
  Instance,
  InstanceState,
  InstancesResponse,
  JobLogsResponse,
  JobResponse,
  JobsResponse,
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
