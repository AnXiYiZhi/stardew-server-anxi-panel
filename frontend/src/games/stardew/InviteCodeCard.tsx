import { useState } from 'react'
import { routeToPath } from './stardew-routes'
import type { StardewDashboardData, StardewNavigateOptions, StardewRoute } from './stardew-routes'

type InviteCodeCardProps = {
  instanceState: StardewDashboardData['instanceState']
  dashboardData: StardewDashboardData
  className?: string
  label?: string
  description?: string
  onNavigate?: (route: StardewRoute, options?: StardewNavigateOptions) => void
}

export function InviteCodeCard({
  instanceState,
  dashboardData,
  className,
  label = '邀请码',
  description = '',
  onNavigate,
}: InviteCodeCardProps) {
  const [copied, setCopied] = useState(false)
  const [ipCopied, setIpCopied] = useState(false)
  const [copyError, setCopyError] = useState(false)

  const state = instanceState?.state ?? null
  const canRefreshInvite = state === 'running' || state === 'starting'
  // Invite codes need Steam authentication to have succeeded at least once
  // (steamAuthLoggedIn = STEAM_AUTH_COMPLETED). If it never has, point the user to
  // the install page to authenticate instead of showing an endless "获取中…".
  const needAuthLogin = instanceState?.steamAuthLoggedIn === false

  function handleGoAuth() {
    if (onNavigate) onNavigate('install')
    else window.location.href = routeToPath('install')
  }

  function handleCopyInvite() {
    const code = dashboardData.inviteCode
    if (!code) return
    setCopyError(false)
    navigator.clipboard.writeText(code).then(
      () => {
        setCopied(true)
        setTimeout(() => setCopied(false), 2000)
      },
      () => {
        setCopyError(true)
        setTimeout(() => setCopyError(false), 3000)
      },
    )
  }

  function handleCopyPublicIP() {
    const ip = dashboardData.publicIP?.ip
    if (!ip) return
    setCopyError(false)
    navigator.clipboard.writeText(ip).then(
      () => {
        setIpCopied(true)
        setTimeout(() => setIpCopied(false), 2000)
      },
      () => {
        setCopyError(true)
        setTimeout(() => setCopyError(false), 3000)
      },
    )
  }

  return (
    <div className={['sd-invite-card-wrap', className].filter(Boolean).join(' ')}>
      <div className="sd-players-invite-row">
        <div className="sd-players-invite-copy">
          <span className="sd-players-invite-label">{label}</span>
          {description ? <span>{description}</span> : null}
        </div>
        {dashboardData.inviteCode ? (
          <span className="sd-players-invite-code">{dashboardData.inviteCode}</span>
        ) : needAuthLogin ? (
          <span className="sd-players-invite-empty">需登录 Steam 授权</span>
        ) : canRefreshInvite ? (
          dashboardData.inviteCodeError ? (
            <span className="sd-players-invite-error">获取失败</span>
          ) : (
            <span className="sd-players-invite-loading">获取中…</span>
          )
        ) : (
          <span className="sd-players-invite-empty">服务器未运行</span>
        )}
        <div className="sd-players-invite-actions">
          {dashboardData.inviteCode ? (
            <button
              className="sd-btn-green sd-players-copy-btn"
              onClick={handleCopyInvite}
              title="复制邀请码"
            >
              {copied ? '已复制' : '复制'}
            </button>
          ) : null}
          {needAuthLogin ? (
            <button
              className="sd-btn-green sd-players-copy-btn"
              onClick={handleGoAuth}
              title="前往安装页登录 Steam 授权以开启邀请码"
            >
              登录授权
            </button>
          ) : (
            <button
              className="sd-btn-tan sd-players-refresh-btn"
              onClick={() => { void dashboardData.refreshInviteCode() }}
              disabled={!canRefreshInvite}
              title={canRefreshInvite ? '刷新邀请码' : '服务器未运行时无法获取邀请码'}
            >
              刷新
            </button>
          )}
        </div>
      </div>
      {needAuthLogin ? (
        <div className="sd-srv-hint" style={{ marginTop: 4 }}>
          邀请码需要先在安装页完成 Steam 授权，当前未登录。点【登录授权】前往安装页登录；也可先用下方「局域网邀请」IP 直连进入。
        </div>
      ) : null}
      <div className="sd-players-invite-row sd-players-public-ip-row">
        <div className="sd-players-invite-copy">
          <span className="sd-players-invite-label">局域网邀请</span>
        </div>
        {dashboardData.publicIP?.ip ? (
          <span className="sd-players-invite-code sd-players-public-ip-code">{dashboardData.publicIP.ip}</span>
        ) : dashboardData.publicIPRefreshing ? (
          <span className="sd-players-invite-loading">检测中…</span>
        ) : dashboardData.publicIPError ? (
          <span className="sd-players-invite-error">检测失败</span>
        ) : (
          <span className="sd-players-invite-empty">未检测</span>
        )}
        <div className="sd-players-invite-actions">
          {dashboardData.publicIP?.ip ? (
            <button
              className="sd-btn-green sd-players-copy-btn"
              onClick={handleCopyPublicIP}
              title="复制服务器公网 IP"
            >
              {ipCopied ? '已复制' : '复制'}
            </button>
          ) : null}
          <button
            className="sd-btn-tan sd-players-refresh-btn"
            onClick={() => { void dashboardData.refreshPublicIP(true) }}
            disabled={dashboardData.publicIPRefreshing}
            title="重新检测服务器公网 IP"
          >
            {dashboardData.publicIPRefreshing ? '检测中' : '刷新'}
          </button>
        </div>
      </div>
      {copyError ? (
        <div className="sd-srv-hint" style={{ color: '#b94040', marginTop: 4 }}>
          复制失败，请手动选取文字。
        </div>
      ) : null}
    </div>
  )
}
