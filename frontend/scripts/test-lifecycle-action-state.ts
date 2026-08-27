import assert from 'node:assert/strict'
import { shouldClearPendingStartupAction } from '../src/games/stardew/lifecycle-action-state.ts'

assert.equal(
  shouldClearPendingStartupAction({
    action: 'restart',
    hasActiveLifecycleJob: false,
    isRunning: true,
    sawActiveLifecycleJob: false,
  }),
  false,
  'the stale running state must not unlock a restart before its job appears',
)

assert.equal(
  shouldClearPendingStartupAction({
    action: 'restart',
    hasActiveLifecycleJob: false,
    isRunning: true,
    sawActiveLifecycleJob: true,
  }),
  true,
  'restart should unlock after the observed lifecycle job reaches a terminal state',
)

assert.equal(
  shouldClearPendingStartupAction({
    action: 'start',
    hasActiveLifecycleJob: false,
    isRunning: true,
    sawActiveLifecycleJob: false,
  }),
  true,
  'a completed start may use the running projection when a short job was not observed',
)

for (const action of ['start', 'restart'] as const) {
  assert.equal(
    shouldClearPendingStartupAction({
      action,
      hasActiveLifecycleJob: true,
      isRunning: true,
      sawActiveLifecycleJob: true,
    }),
    false,
    `${action} must remain locked while a lifecycle job is active`,
  )

  assert.equal(
    shouldClearPendingStartupAction({
      action,
      hasActiveLifecycleJob: false,
      isRunning: false,
      sawActiveLifecycleJob: true,
    }),
    true,
    `${action} must unlock when its observed lifecycle job terminates in a stopped or error state`,
  )
}

assert.equal(
  shouldClearPendingStartupAction({
    action: null,
    hasActiveLifecycleJob: false,
    isRunning: true,
    sawActiveLifecycleJob: true,
  }),
  false,
)

console.log('lifecycle action state tests passed')
