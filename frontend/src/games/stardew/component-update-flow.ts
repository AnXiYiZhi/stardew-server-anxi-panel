type WorkflowGeneration = {
  phase?: string
  startedAt?: string
  updatedAt?: string
  finishedAt?: string
}

function workflowTime(workflow: WorkflowGeneration | null | undefined): number {
  if (!workflow) return 0
  for (const value of [workflow.startedAt, workflow.updatedAt, workflow.finishedAt]) {
    if (!value) continue
    const timestamp = Date.parse(value)
    if (Number.isFinite(timestamp)) return timestamp
  }
  return 0
}

export function shouldStartRequestedApply(
  requestedDryRunId: string | null,
  currentDryRunId: string | undefined,
  currentPhase: string | undefined,
  applyBusy: boolean,
  applyActive: boolean,
): boolean {
  return requestedDryRunId !== null
    && requestedDryRunId === currentDryRunId
    && currentPhase === 'succeeded'
    && !applyBusy
    && !applyActive
}

export function preferDryRunWorkflow(
  dryRun: WorkflowGeneration | null | undefined,
  apply: WorkflowGeneration | null | undefined,
): boolean {
  if (!dryRun || dryRun.phase === 'idle') return false
  if (!apply || apply.phase === 'idle') return true
  const dryRunTime = workflowTime(dryRun)
  const applyTime = workflowTime(apply)
  return dryRunTime > 0 && dryRunTime > applyTime
}
