import { useState } from 'react'
import { steamAuthLogin } from '../../api'
import { errorMessage } from '../../core/helpers'
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

// navigator.clipboard requires a secure context (HTTPS or localhost). This
// panel is commonly reached over plain HTTP via a LAN/public IP, where
// navigator.clipboard is undefined and calling it throws synchronously —
// falling back to the legacy execCommand path keeps the copy buttons working.
async function copyText(text: string): Promise<boolean> {
  if (typeof navigator !== 'undefined' && navigator.clipboard && window.isSecureContext) {
    try {
      await navigator.clipboard.writeText(text)
      return true
    } catch {
      // fall through to legacy fallback below
    }
  }
  try {
    const textarea = document.createElement('textarea')
    textarea.value = text
    textarea.style.position = 'fixed'
    textarea.style.opacity = '0'
    document.body.appendChild(textarea)
    textarea.focus()
    textarea.select()
    const ok = document.execCommand('copy')
    document.body.removeChild(textarea)
    return ok
  } catch {
    return false
  }
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
  const [authBusy, setAuthBusy] = useState(false)
  const [authMsg, setAuthMsg] = useState<string | null>(null)

  const state = instanceState?.state ?? null
  const canRefreshInvite = state === 'running' || state === 'starting'
  // Main UI follows the durable steam-auth flag. The backend sets it true only
  // after steam-auth login succeeds, and clears it if server logs prove the auth
  // service has no logged-in account.
  const needAuthLogin = !dashboardData.inviteCode && instanceState?.steamAuthLoggedIn !== true
  const authRequiresStop = needAuthLogin && canRefreshInvite
  const authButtonLabel = authBusy
    ? '发起中…'
    : authRequiresStop ? '停服后登录授权' : '登录授权'

  // Kick off a steam-auth login (login only — the backend stops after auth succeeds)
  // and jump to the install page so the user can watch the logs and answer any Steam
  // Guard prompt there. The server must be stopped; a 409 surfaces as an inline message.
  async function handleGoAuth() {
    setAuthBusy(true)
    setAuthMsg(null)
    try {
      await steamAuthLogin()
      if (onNavigate) onNavigate('install')
      else window.location.href = routeToPath('install')
    } catch (e) {
      setAuthMsg(errorMessage(e))
    } finally {
      setAuthBusy(false)
    }
  }

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

  function handleCopyPublicIP() {
    const ip = dashboardData.publicIP?.ip
    if (!ip) return
    setCopyError(false)
    void copyText(ip).then((ok) => {
      if (ok) {
        setIpCopied(true)
        setTimeout(() => setIpCopied(false), 2000)
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
        {dashboardData.inviteCode ? (
          <span className="sd-players-invite-code">{dashboardData.inviteCode}</span>
        ) : needAuthLogin ? (
          <span className="sd-players-invite-empty">
            需登录 Steam 授权
          </span>
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
              onClick={() => { void handleGoAuth() }}
              disabled={authBusy || authRequiresStop}
              title={authRequiresStop
                ? '请先停止服务器，再登录 Steam 授权'
                : '登录 Steam 授权并前往安装页查看认证日志'}
            >
              {authButtonLabel}
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
          {authRequiresStop
            ? '当前未完成 Steam 授权。请先停止服务器，再登录 Steam 授权；也可先用下方「局域网邀请」IP 直连进入。'
            : '邀请码需要先完成 Steam 授权。点【登录授权】会用已保存账号发起登录并跳转到安装页查看认证日志（如需手机批准/验证码在那里完成）；也可先用下方「局域网邀请」IP 直连进入。'}
        </div>
      ) : null}
      {authMsg ? (
        <div className="sd-srv-hint" style={{ marginTop: 4, color: '#b94040' }}>{authMsg}</div>
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
              title="复制当前面板访问地址"
            >
              {ipCopied ? '已复制' : '复制'}
            </button>
          ) : null}
          <button
            className="sd-btn-tan sd-players-refresh-btn"
            onClick={() => { void dashboardData.refreshPublicIP(true) }}
            disabled={dashboardData.publicIPRefreshing}
            title="同步当前面板访问地址"
          >
            {dashboardData.publicIPRefreshing ? '同步中' : '同步'}
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
