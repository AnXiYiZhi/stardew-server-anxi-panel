import { useState } from 'react'
import { copyText } from './copy-text'
import { steamInviteIsEnabled, steamInvitePresentation } from './steam-invite-state'
import { useSteamAuthLogin } from './useSteamAuthLogin'
import type { StardewDashboardData, StardewNavigateOptions, StardewRoute } from './stardew-routes'

type InviteCodeCardProps = {
  instanceState: StardewDashboardData['instanceState']
  dashboardData: StardewDashboardData
  className?: string
  label?: string
  description?: string
  canManageSteamInvite?: boolean
  onNavigate?: (route: StardewRoute, options?: StardewNavigateOptions) => void
}

export function InviteCodeCard({
  instanceState,
  dashboardData,
  className,
  label = 'Steam 邀请码',
  description = '',
  canManageSteamInvite = false,
  onNavigate,
}: InviteCodeCardProps) {
  const [copied, setCopied] = useState(false)
  const [copyError, setCopyError] = useState(false)
  const steamAuth = useSteamAuthLogin({
    instanceState,
    onNavigate,
    onStarted: () => {
      void dashboardData.refreshInstanceState()
      void dashboardData.refreshJobs()
    },
  })
  const enabled = steamInviteIsEnabled(instanceState)
  const presentation = steamInvitePresentation(
    enabled,
    dashboardData.inviteCodeStatus,
    dashboardData.inviteCode,
    dashboardData.inviteCodeError,
    instanceState?.steamInviteAuthState,
    instanceState?.state,
    dashboardData.inviteCodeRefreshing,
  )
  const state = instanceState?.state ?? null
  const canRefreshInvite = enabled && (state === 'running' || state === 'starting')
  const authRequiresStop = presentation.retryAuthorization && steamAuth.requiresStop

  if (!enabled) return null

  function handleCopyInvite() {
    const code = dashboardData.inviteCode
    if (!code) return
    setCopyError(false)
    void copyText(code).then((ok) => {
      if (ok) {
        setCopied(true)
        setTimeout(() => setCopied(false), 2000)
      } else {
        setCopyError(true)
        setTimeout(() => setCopyError(false), 3000)
      }
    })
  }

  return (
    <div className={['sd-invite-card-wrap', className].filter(Boolean).join(' ')}>
      <div className="sd-players-invite-row">
        <div className="sd-players-invite-copy">
          <span className="sd-players-invite-label">{label}</span>
          {description ? <span>{description}</span> : null}
        </div>
        <span
          className={
            presentation.copyable
              ? 'sd-players-invite-code'
              : presentation.tone === 'error'
                ? 'sd-players-invite-error'
                : presentation.tone === 'loading'
                  ? 'sd-players-invite-loading'
                  : 'sd-players-invite-empty'
          }
        >
          {presentation.text}
        </span>
        <div className="sd-players-invite-actions">
          {presentation.copyable ? (
            <button
              className="sd-btn-green sd-players-copy-btn"
              onClick={handleCopyInvite}
              title="复制 Steam 邀请码"
            >
              {copied ? '已复制' : '复制'}
            </button>
          ) : null}
          {presentation.retryAuthorization ? (
            canManageSteamInvite ? (
              <button
                className="sd-btn-green sd-players-copy-btn"
                onClick={() => { void steamAuth.login() }}
                disabled={steamAuth.busy || authRequiresStop}
                title={steamAuth.title}
              >
                {steamAuth.busy ? '发起中…' : '重新授权'}
              </button>
            ) : null
          ) : (
            <button
              className="sd-btn-tan sd-players-refresh-btn"
              onClick={() => { void dashboardData.refreshInviteCode() }}
              disabled={!canRefreshInvite || dashboardData.inviteCodeRefreshing}
              title={canRefreshInvite ? '刷新 Steam 邀请码' : '服务器未运行时无法获取 Steam 邀请码'}
            >
              {dashboardData.inviteCodeRefreshing ? '刷新中' : '刷新'}
            </button>
          )}
        </div>
      </div>
      {presentation.retryAuthorization ? (
        <div className="sd-srv-hint" style={{ marginTop: 4 }}>
          {authRequiresStop
            ? '请先停止服务器，再重新完成 Steam 邀请码授权；局域网/IP 直连仍可使用。'
            : canManageSteamInvite
              ? 'Steam 邀请码授权尚未就绪，可重新授权；局域网/IP 直连仍可使用。'
              : 'Steam 邀请码授权尚未就绪，请联系管理员处理；局域网/IP 直连仍可使用。'}
        </div>
      ) : null}
      {steamAuth.message ? (
        <div className="sd-srv-hint" style={{ marginTop: 4, color: '#b94040' }}>{steamAuth.message}</div>
      ) : null}
      {copyError ? (
        <div className="sd-srv-hint" style={{ color: '#b94040', marginTop: 4 }}>
          复制失败，请手动选取文字。
        </div>
      ) : null}
    </div>
  )
}
