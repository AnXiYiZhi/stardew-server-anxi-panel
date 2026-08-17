export type PendingStartupAction = 'start' | 'restart' | null

type PendingStartupCompletion = {
  action: PendingStartupAction
  hasActiveLifecycleJob: boolean
  isRunning: boolean
  sawActiveLifecycleJob: boolean
}

// A stale running projection is valid completion evidence for a start that was
// submitted from a stopped state, but not for restart: restart begins while the
// instance is already running and must stay locked until its lifecycle job has
// actually been observed and reached a terminal state.
export function shouldClearPendingStartupAction({
  action,
  hasActiveLifecycleJob,
  isRunning,
  sawActiveLifecycleJob,
}: PendingStartupCompletion): boolean {
  if (!action || hasActiveLifecycleJob) return false
  if (sawActiveLifecycleJob) return true
  return action === 'start' && isRunning
}
