import { useEffect, useRef, useState } from 'react'
import type { InstanceState, Job, StardewPlayersResponse } from '../../types'

export const HOST_ONLINE_WAIT_TIMEOUT_MS = 10 * 60_000

export type StardewLifecyclePhase =
  | 'starting'
  | 'waiting_for_host'
  | 'stopping'
  | 'running'
  | 'stopped'
  | 'save_required'
  | 'error'
  | 'unknown'

type LifecycleStateOptions = {
  instanceState: InstanceState | null
  jobs: Job[]
  players: StardewPlayersResponse | null
  pendingStartup?: boolean
  pendingStop?: boolean
}

export function useStardewLifecycleState({
  instanceState,
  jobs,
  players,
  pendingStartup = false,
  pendingStop = false,
}: LifecycleStateOptions) {
  const backendUIStatus = instanceState?.uiStatus
  const state = instanceState?.state ?? null
  const isRunning = state === 'running'
  const isStarting = state === 'starting'
  const isStopping = state === 'stopping'
  const isStopped = state === 'stopped' || state === 'ready_to_start' || state === 'game_installed'
  const activeLifecycleJob = jobs.find(
    (job) => job.type === 'stardew_lifecycle' && (job.status === 'running' || job.status === 'queued'),
  ) ?? null
  const activeLifecycleIsStopping = Boolean(activeLifecycleJob && instanceState?.driverPhase === 'stopping')
  const hostOnline = (players?.players ?? []).some(
    (player) => player.isHost && player.status === 'online',
  )

  const hostWaitStartedAtRef = useRef<number | null>(null)
  const [hostConfirmTimedOut, setHostConfirmTimedOut] = useState(false)

  useEffect(() => {
    if (!isRunning || hostOnline) {
      hostWaitStartedAtRef.current = null
      setHostConfirmTimedOut(false)
      return
    }

    if (hostWaitStartedAtRef.current === null) {
      hostWaitStartedAtRef.current = Date.now()
      setHostConfirmTimedOut(false)
    }
    const remaining = Math.max(
      0,
      HOST_ONLINE_WAIT_TIMEOUT_MS - (Date.now() - hostWaitStartedAtRef.current),
    )
    const timeout = window.setTimeout(() => setHostConfirmTimedOut(true), remaining)
    return () => window.clearTimeout(timeout)
  }, [isRunning, hostOnline])

  const awaitingHostConfirmation = isRunning && !hostOnline && !hostConfirmTimedOut
  // The live players poll is newer than the slower instance-state projection.
  // Once it sees the host online, startup is complete even if uiStatus or the
  // local submitted-action flag still reflects a lifecycle job waiting on the
  // background invite-code probe.
  const fallbackStartupInProgress =
    isStarting ||
    (pendingStartup && !hostOnline) ||
    (Boolean(activeLifecycleJob) && !activeLifecycleIsStopping && !isRunning) ||
    awaitingHostConfirmation
  const waitingForStop =
    pendingStop || isStopping || activeLifecycleIsStopping || backendUIStatus === 'stopping'
  const backendStartupInProgress = Boolean(
    backendUIStatus && ['starting_container', 'loading_save', 'waiting_for_host'].includes(backendUIStatus),
  )
  const startupInProgress =
    !waitingForStop && !(isRunning && hostOnline) &&
    (backendUIStatus ? backendStartupInProgress : fallbackStartupInProgress)

  let phase: StardewLifecyclePhase = 'unknown'
  if (waitingForStop) phase = 'stopping'
  else if (isRunning && hostOnline) phase = 'running'
  else if (backendUIStatus === 'ready') phase = 'running'
  else if (backendUIStatus === 'failed') phase = 'error'
  else if (backendUIStatus === 'stopped') phase = 'stopped'
  else if (backendUIStatus === 'waiting_for_host') phase = 'waiting_for_host'
  else if (backendUIStatus === 'starting_container' || backendUIStatus === 'loading_save') phase = 'starting'
  else if (startupInProgress) phase = awaitingHostConfirmation ? 'waiting_for_host' : 'starting'
  else if (isRunning) phase = 'running'
  else if (isStopped) phase = 'stopped'
  else if (state === 'save_required') phase = 'save_required'
  else if (state === 'error') phase = 'error'

  return {
    state,
    phase,
    isRunning,
    isStarting,
    isStopping,
    isStopped,
    activeLifecycleJob,
    hasActiveLifecycleJob: Boolean(activeLifecycleJob),
    activeLifecycleIsStopping,
    hostOnline,
    hostConfirmTimedOut,
    awaitingHostConfirmation,
    startupInProgress,
    waitingForStop,
  }
}
