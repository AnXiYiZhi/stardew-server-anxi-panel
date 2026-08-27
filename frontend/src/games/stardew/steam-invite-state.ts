import type { InstanceState, SteamInviteAuthState, SteamInviteStatus } from '../../types'

export type SteamInvitePresentation = {
  text: string
  copyable: boolean
  retryAuthorization: boolean
  tone: 'normal' | 'loading' | 'muted' | 'error'
}

export function steamInviteIsEnabled(
  state: Pick<InstanceState, 'steamInviteEnabled'> | null | undefined,
): boolean {
  return state?.steamInviteEnabled === true
}

export function shouldPollSteamInvite(
  state: Pick<InstanceState, 'steamInviteEnabled' | 'state'> | null | undefined,
  requested: boolean,
  inviteCode: string | null,
  status: SteamInviteStatus | null,
): boolean {
  if (!steamInviteIsEnabled(state) || !requested || Boolean(inviteCode)) return false
  if (state?.state !== 'running' && state?.state !== 'starting') return false
  return status === null || status === 'generating' || status === 'ready'
}

export function shouldRestartSteamInvitePolling(
  previousRuntimeState: InstanceState['state'] | null,
  nextRuntimeState: InstanceState['state'],
  lastStatus: SteamInviteStatus | null,
): boolean {
  const previousActive = previousRuntimeState === 'starting' || previousRuntimeState === 'running'
  const nextActive = nextRuntimeState === 'starting' || nextRuntimeState === 'running'
  if (!nextActive) return false
  if (!previousActive) return true
  return previousRuntimeState === 'starting'
    && nextRuntimeState === 'running'
    && lastStatus !== 'auth_unavailable'
}

export function isCurrentSteamInviteRequest(
  requestGeneration: number,
  currentGeneration: number,
  enabled: boolean,
): boolean {
  return enabled && requestGeneration === currentGeneration
}

export function isCurrentSteamInviteProjection(
  requestGeneration: number,
  currentGeneration: number,
): boolean {
  return requestGeneration === currentGeneration
}

export function preserveSteamInviteProjection(
  nextState: InstanceState,
  currentState: InstanceState | null,
): InstanceState {
  if (!currentState) return nextState
  return {
    ...nextState,
    steamInviteEnabled: currentState.steamInviteEnabled,
    inviteCode: currentState.inviteCode,
  }
}

export function shouldResumeSteamInviteAfterRuntimeReset(
  enabled: boolean,
  inviteCode: string | null,
  lastStatus: SteamInviteStatus | null,
): boolean {
  return enabled
    && !inviteCode
    && (lastStatus === null || lastStatus === 'generating')
}

export function steamInvitePollBudgetExhausted(attempts: number, maximum: number): boolean {
  return attempts >= maximum
}

export function steamInvitePresentation(
  enabled: boolean,
  status: SteamInviteStatus | null,
  inviteCode: string | null,
  error: string | null,
  authState?: SteamInviteAuthState,
  runtimeState?: InstanceState['state'],
  polling = false,
): SteamInvitePresentation {
  if (!enabled) {
    return { text: '', copyable: false, retryAuthorization: false, tone: 'muted' }
  }
  if (inviteCode) {
    return { text: inviteCode, copyable: true, retryAuthorization: false, tone: 'normal' }
  }
  if (status === 'authorization_failed' || authState === 'failed') {
    return { text: '授权失败，可重试', copyable: false, retryAuthorization: true, tone: 'error' }
  }
  if (authState === 'cleanup_pending') {
    return { text: '等待中…', copyable: false, retryAuthorization: false, tone: 'loading' }
  }
  if (status === 'auth_unavailable') {
    return { text: 'Auth 异常（直连仍可用）', copyable: false, retryAuthorization: true, tone: 'error' }
  }
  if (polling && error && status === 'generating'
    && (runtimeState === 'starting' || (runtimeState === 'running' && authState === 'ready'))) {
    return { text: '等待中…', copyable: false, retryAuthorization: false, tone: 'loading' }
  }
  if (error) {
    return { text: 'Auth 异常（直连仍可用）', copyable: false, retryAuthorization: true, tone: 'error' }
  }
  if (status === 'waiting_authorization' || authState === 'pending') {
    return { text: '等待 Steam 授权', copyable: false, retryAuthorization: true, tone: 'muted' }
  }
  if (authState === 'authorizing') {
    return { text: '正在授权…', copyable: false, retryAuthorization: false, tone: 'loading' }
  }
  if (status === 'server_stopped') {
    return { text: '服务器未运行', copyable: false, retryAuthorization: false, tone: 'muted' }
  }
  return { text: '等待中…', copyable: false, retryAuthorization: false, tone: 'loading' }
}
