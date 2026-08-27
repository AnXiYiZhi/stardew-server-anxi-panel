import { useState } from 'react'
import { ApiError, steamAuthLogin } from '../../api'
import { errorMessage } from '../../core/helpers'
import { routeToPath } from './stardew-routes'
import type { StardewDashboardData, StardewNavigateOptions, StardewRoute } from './stardew-routes'

type UseSteamAuthLoginOptions = {
  instanceState: StardewDashboardData['instanceState']
  onNavigate?: (route: StardewRoute, options?: StardewNavigateOptions) => void
  onStarted?: (jobId: string) => void
}

export function useSteamAuthLogin({ instanceState, onNavigate, onStarted }: UseSteamAuthLoginOptions) {
  const [busy, setBusy] = useState(false)
  const [message, setMessage] = useState<string | null>(null)
  const state = instanceState?.state ?? null
  const requiresStop = state === 'running' || state === 'starting'
  const label = busy ? '发起中…' : requiresStop ? '停服后授权' : '重新授权'

  async function login() {
    setBusy(true)
    setMessage(null)
    try {
      const response = await steamAuthLogin()
      onStarted?.(response.jobId)
      if (onNavigate) onNavigate('install', { installJobId: response.jobId })
      else window.location.href = routeToPath('install', { installJobId: response.jobId })
    } catch (error) {
      if (error instanceof ApiError && error.code === 'install_in_progress') {
        const jobId = typeof error.details === 'object' && error.details !== null && 'jobId' in error.details
          ? String((error.details as { jobId?: unknown }).jobId ?? '')
          : ''
        if (jobId) onStarted?.(jobId)
        if (onNavigate) onNavigate('install', jobId ? { installJobId: jobId } : undefined)
        else window.location.href = routeToPath('install', jobId ? { installJobId: jobId } : undefined)
        return
      }
      setMessage(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }

  return {
    busy,
    label,
    message,
    requiresStop,
    login,
    title: requiresStop
      ? '请先停止服务器，再进行 Steam 邀请码授权'
      : undefined,
  }
}
