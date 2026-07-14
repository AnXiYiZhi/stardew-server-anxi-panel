import assert from 'node:assert/strict'
import { preferDryRunWorkflow, shouldStartRequestedApply } from '../src/games/stardew/component-update-flow.ts'

assert.equal(shouldStartRequestedApply('new-dry-run', 'old-dry-run', 'succeeded', false, false), false)
assert.equal(shouldStartRequestedApply('new-dry-run', 'new-dry-run', 'checking', false, false), false)
assert.equal(shouldStartRequestedApply('new-dry-run', 'new-dry-run', 'succeeded', false, false), true)
assert.equal(shouldStartRequestedApply('new-dry-run', 'new-dry-run', 'succeeded', true, false), false)
assert.equal(shouldStartRequestedApply('new-dry-run', 'new-dry-run', 'succeeded', false, true), false)

const oldFailedApply = {
  phase: 'failed_rolled_back',
  startedAt: '2026-07-14T15:27:11Z',
  finishedAt: '2026-07-14T15:33:30Z',
}
const newDryRun = {
  phase: 'checking',
  startedAt: '2026-07-14T15:47:28Z',
  updatedAt: '2026-07-14T15:47:29Z',
}
const newerApply = {
  phase: 'checking',
  startedAt: '2026-07-14T15:47:31Z',
}

assert.equal(preferDryRunWorkflow(newDryRun, oldFailedApply), true)
assert.equal(preferDryRunWorkflow(newDryRun, newerApply), false)
assert.equal(preferDryRunWorkflow({ phase: 'idle' }, oldFailedApply), false)

console.log('component update flow tests passed')
