import assert from 'node:assert/strict'
import { canonicalInstallJobs, logsDescribeActiveInstall, reconcileJobSnapshots } from '../src/games/stardew/install-state.ts'
import type { Job, JobStatus } from '../src/types.ts'

function job(id: string, status: JobStatus, updatedAt: string, createdAt = '2026-08-11T00:00:00.000Z'): Job {
  return {
    id,
    type: 'stardew_install',
    status,
    targetType: 'instance',
    targetId: 'stardew',
    createdBy: 1,
    createdAt,
    startedAt: status === 'queued' ? null : createdAt,
    finishedAt: ['succeeded', 'failed', 'canceled'].includes(status) ? updatedAt : null,
    errorMessage: null,
    updatedAt,
  }
}

const staleDetail = job('job_install', 'running', '2026-08-11T00:00:05.000Z')
const terminalDashboard = job('job_install', 'succeeded', '2026-08-11T00:00:06.000Z')
assert.equal(reconcileJobSnapshots(staleDetail, terminalDashboard).status, 'succeeded')
assert.equal(reconcileJobSnapshots(terminalDashboard, staleDetail).status, 'succeeded')

const completed = canonicalInstallJobs([terminalDashboard], staleDetail)
assert.equal(completed.active, null)
assert.equal(completed.selected?.status, 'succeeded')
assert.equal(logsDescribeActiveInstall(completed.active, 'job_install'), false)

const active = job('job_new', 'running', '2026-08-11T00:01:05.000Z', '2026-08-11T00:01:00.000Z')
const withActive = canonicalInstallJobs([active, terminalDashboard], staleDetail)
assert.equal(withActive.active?.id, 'job_new')
assert.equal(withActive.latest?.id, 'job_new')
assert.equal(logsDescribeActiveInstall(withActive.active, 'job_install'), false)
assert.equal(logsDescribeActiveInstall(withActive.active, 'job_new'), true)

const terminalDetail = job('job_new', 'failed', '2026-08-11T00:01:06.000Z', '2026-08-11T00:01:00.000Z')
const terminalWinsOverDashboard = canonicalInstallJobs([active], terminalDetail)
assert.equal(terminalWinsOverDashboard.active, null)
assert.equal(terminalWinsOverDashboard.selected?.status, 'failed')
