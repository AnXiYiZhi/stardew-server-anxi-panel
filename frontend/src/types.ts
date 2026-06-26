export type Role = 'admin' | 'user'

export type CurrentUser = {
  id: number
  username: string
  role: Role
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
