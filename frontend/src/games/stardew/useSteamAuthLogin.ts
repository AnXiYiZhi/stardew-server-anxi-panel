import { useState } from 'react'
import { steamAuthLogin } from '../../api'
import { errorMessage } from '../../core/helpers'
import { routeToPath } from './stardew-routes'
import type { StardewDashboardData, StardewNavigateOptions, StardewRoute } from './stardew-routes'

type UseSteamAuthLoginOptions = {
  instanceState: StardewDashboardData['instanceState']
  onNavigate?: (route: StardewRoute, options?: StardewNavigateOptions) => void
}

export function useSteamAuthLogin({ instanceState, onNavigate }: UseSteamAuthLoginOptions) {
  const [busy, setBusy] = useState(false)
  const [message, setMessage] = useState<string | null>(null)
  const state = instanceState?.state ?? null
  const requiresStop = state === 'running' || state === 'starting'
  const label = busy ? '发起中…' : requiresStop ? '停服后登录授权' : '登录授权'

  async function login() {
    setBusy(true)
    setMessage(null)
    try {
      await steamAuthLogin()
      if (onNavigate) onNavigate('install')
      else window.location.href = routeToPath('install')
    } catch (error) {
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
      ? '请先停止服务器，再登录 Steam 授权'
      : '登录 Steam 授权并前往安装页查看认证日志',
  }
}
