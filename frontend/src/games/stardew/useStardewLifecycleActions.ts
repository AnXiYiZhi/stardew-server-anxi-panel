import { useEffect, useState } from 'react'
import { ApiError, startInstance, stopInstance, restartInstance } from '../../api'
import { errorMessage } from '../../core/helpers'
import type { InstanceState } from '../../types'
import type { StardewDashboardData } from './stardew-routes'
import { useStardewLifecycleState } from './useStardewLifecycleState'

function saveStartBlocker(error: unknown): 'new' | 'saves' | null {
  if (!(error instanceof ApiError)) return null
  if (error.code === 'save_required') return 'new'
  if (error.code === 'active_save_required' || error.code === 'active_save_missing') return 'saves'
  return null
}

type LifecycleActionsOptions = {
  instanceState: InstanceState | null
  dashboardData: StardewDashboardData
  isAdmin: boolean
}

export function useStardewLifecycleActions({ instanceState, dashboardData, isAdmin }: LifecycleActionsOptions) {
  const [actionBusy, setActionBusy] = useState(false)
  const [actionError, setActionError] = useState<string | null>(null)
  const [saveRequiredDetected, setSaveRequiredDetected] = useState(false)
  const [confirmAction, setConfirmAction] = useState<'stop' | 'restart' | null>(null)
  const [pendingStartupAction, setPendingStartupAction] = useState<'start' | 'restart' | null>(null)
  const [pendingStopAction, setPendingStopAction] = useState(false)

  const state = instanceState?.state ?? null

  const lifecycle = useStardewLifecycleState({
    instanceState,
    jobs: dashboardData.jobs,
    players: dashboardData.players,
    pendingStartup: Boolean(pendingStartupAction),
    pendingStop: pendingStopAction,
  })
  const { isRunning, isStarting, isStopped, hasActiveLifecycleJob, startupInProgress, waitingForStop } = lifecycle

  const noSavesDetected = Boolean(dashboardData.saves && dashboardData.saves.saves.length === 0)
  const showSaveRequiredPrompt =
    (state === 'save_required' || saveRequiredDetected || noSavesDetected) && !isRunning && !isStarting

  const canStart = isAdmin && isStopped && !actionBusy && !startupInProgress && !waitingForStop
  const canStop = isAdmin && isRunning && !actionBusy && !waitingForStop
  const canRestart = isAdmin && isRunning && !actionBusy && !waitingForStop

  useEffect(() => {
    if (state && state !== 'save_required') {
      setSaveRequiredDetected(false)
    }
  }, [state])

  useEffect(() => {
    // Startup display follows the backend lifecycle job/state.
    if (!hasActiveLifecycleJob && isRunning) {
      setPendingStartupAction(null)
    }
  }, [hasActiveLifecycleJob, isRunning])

  useEffect(() => {
    if (state === 'stopped' || state === 'ready_to_start' || state === 'game_installed' || state === 'save_required' || state === 'error') {
      setPendingStopAction(false)
    }
  }, [state])

  async function handleStart() {
    setActionBusy(true)
    setPendingStartupAction('start')
    setPendingStopAction(false)
    setActionError(null)
    try {
      await startInstance()
      dashboardData.requestInviteCodeRefresh()
      setSaveRequiredDetected(false)
      dashboardData.refreshInstanceState()
      dashboardData.refreshJobs()
    } catch (e) {
      const saveBlocker = saveStartBlocker(e)
      if (saveBlocker) {
        setSaveRequiredDetected(saveBlocker === 'new')
        setActionError(saveBlocker === 'new' ? null : errorMessage(e))
        dashboardData.refreshInstanceState()
        dashboardData.refreshSaves()
        setPendingStartupAction(null)
        return
      }
      setActionError(errorMessage(e))
      setPendingStartupAction(null)
    } finally {
      setActionBusy(false)
    }
  }

  async function handleStop() {
    setActionBusy(true)
    setPendingStopAction(true)
    setPendingStartupAction(null)
    setActionError(null)
    dashboardData.clearInviteCode()
    try {
      await stopInstance()
      dashboardData.refreshInstanceState()
      dashboardData.refreshJobs()
    } catch (e) {
      setActionError(errorMessage(e))
      setPendingStopAction(false)
    } finally {
      setActionBusy(false)
    }
  }

  async function handleRestart() {
    setActionBusy(true)
    setPendingStartupAction('restart')
    setActionError(null)
    try {
      await restartInstance()
      dashboardData.requestInviteCodeRefresh()
      dashboardData.refreshInstanceState()
      dashboardData.refreshJobs()
    } catch (e) {
      setActionError(errorMessage(e))
      setPendingStartupAction(null)
    } finally {
      setActionBusy(false)
    }
  }

  function requestConfirm(action: 'stop' | 'restart') {
    setConfirmAction(action)
  }

  function cancelConfirm() {
    setConfirmAction(null)
  }

  function confirmPendingAction() {
    const action = confirmAction
    setConfirmAction(null)
    void (action === 'stop' ? handleStop() : handleRestart())
  }

  return {
    ...lifecycle,
    actionBusy,
    actionError,
    saveRequiredDetected,
    showSaveRequiredPrompt,
    confirmAction,
    canStart,
    canStop,
    canRestart,
    handleStart,
    handleStop,
    handleRestart,
    requestConfirm,
    cancelConfirm,
    confirmPendingAction,
  }
}
