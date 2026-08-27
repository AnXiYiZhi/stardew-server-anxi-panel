import type { Job, JobLog } from '../../types'

export type CanonicalInstallJobs = {
  active: Job | null
  latest: Job | null
  selected: Job | null
}

const BASE_INSTALL_JOB_TYPES = new Set(['stardew_install'])
const INSTALL_PAGE_JOB_TYPES = new Set(['stardew_install', 'stardew_steam_auth'])

function terminal(status: Job['status']): boolean {
  return status === 'succeeded' || status === 'failed' || status === 'canceled'
}

function timestamp(value: string | null | undefined): number {
  const parsed = value ? Date.parse(value) : Number.NaN
  return Number.isFinite(parsed) ? parsed : 0
}

function newerJob(a: Job, b: Job): Job {
  const aTime = Math.max(timestamp(a.updatedAt), timestamp(a.finishedAt), timestamp(a.startedAt), timestamp(a.createdAt))
  const bTime = Math.max(timestamp(b.updatedAt), timestamp(b.finishedAt), timestamp(b.startedAt), timestamp(b.createdAt))
  return bTime > aTime ? b : a
}

// Job state is monotonic. Once any source has observed a terminal snapshot for
// an ID, a delayed queued/running response must never revive that same job.
export function reconcileJobSnapshots(a: Job, b: Job): Job {
  const aTerminal = terminal(a.status)
  const bTerminal = terminal(b.status)
  if (aTerminal !== bTerminal) return aTerminal ? a : b
  return newerJob(a, b)
}

function canonicalJobs(
  dashboardJobs: Job[],
  detailJob: Job | null,
  acceptedTypes: ReadonlySet<string>,
  selectedJobId: string | null = detailJob?.id ?? null,
): CanonicalInstallJobs {
  const byId = new Map<string, Job>()
  for (const job of dashboardJobs) {
    if (!acceptedTypes.has(job.type)) continue
    const existing = byId.get(job.id)
    byId.set(job.id, existing ? reconcileJobSnapshots(existing, job) : job)
  }
  if (detailJob && acceptedTypes.has(detailJob.type)) {
    const existing = byId.get(detailJob.id)
    byId.set(detailJob.id, existing ? reconcileJobSnapshots(existing, detailJob) : detailJob)
  }

  const jobs = [...byId.values()].sort((a, b) => {
    const createdDiff = timestamp(b.createdAt) - timestamp(a.createdAt)
    return createdDiff !== 0 ? createdDiff : b.id.localeCompare(a.id)
  })
  return {
    active: jobs.find((job) => !terminal(job.status)) ?? null,
    latest: jobs[0] ?? null,
    selected: selectedJobId ? byId.get(selectedJobId) ?? null : null,
  }
}

export function canonicalInstallJobs(
  dashboardJobs: Job[],
  detailJob: Job | null,
  selectedJobId?: string | null,
): CanonicalInstallJobs {
  return canonicalJobs(dashboardJobs, detailJob, BASE_INSTALL_JOB_TYPES, selectedJobId)
}

export function canonicalInstallPageJobs(
  dashboardJobs: Job[],
  detailJob: Job | null,
  selectedJobId?: string | null,
): CanonicalInstallJobs {
  return canonicalJobs(dashboardJobs, detailJob, INSTALL_PAGE_JOB_TYPES, selectedJobId)
}

export function installJobForDisplay(jobs: CanonicalInstallJobs): Job | null {
  return jobs.selected ?? jobs.active ?? jobs.latest
}

export function logsDescribeActiveInstall(activeJob: Job | null, selectedJobId: string | null): boolean {
  return activeJob !== null && selectedJobId === activeJob.id
}

export function latestInstallLogsFirst(logs: readonly JobLog[], limit = 50): JobLog[] {
  return [...logs]
    .sort((a, b) => b.sequence - a.sequence)
    .slice(0, Math.max(0, limit))
}
