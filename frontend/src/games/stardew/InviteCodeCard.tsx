import { useState } from 'react'
import { steamAuthLogin } from '../../api'
import type { StardewDashboardData } from './stardew-routes'

type InviteCodeCardProps = {
  instanceState: StardewDashboardData['instanceState']
  dashboardData: StardewDashboardData
  className?: string
  label?: string
  description?: string
}

export function InviteCodeCard({
  instanceState,
  dashboardData,
  className,
  label = '邀请码',
  description = '',
}: InviteCodeCardProps) {
  const [copied, setCopied] = useState(false)
  const [ipCopied, setIpCopied] = useState(false)
  const [copyError, setCopyError] = useState(false)
  const [authBusy, setAuthBusy] = useState(false)
  const [authMsg, setAuthMsg] = useState<string | null>(null)

  const state = instanceState?.state ?? null
  const canRefreshInvite = state === 'running' || state === 'starting'
  // Invite codes need a valid steam-auth login (STEAM_REFRESH_TOKEN) for the host to
  // log into Steam/Galaxy. Without it no invite code can ever be generated, so tell the
  // user to log in rather than showing an endless "获取中…".
  const needAuthLogin = instanceState?.steamAuthLoggedIn === false
  const serverRunning = canRefreshInvite

  async function handleAuthLogin() {
    if (serverRunning) {
      setAuthMsg('请先停止服务器，再登录 Steam 授权。')
      return
    }
    setAuthBusy(true)
    setAuthMsg(null)
    try {
      await steamAuthLogin()
      setAuthMsg('已发起 Steam 授权登录。若提示手机批准或验证码，请在安装/授权状态处完成；登录成功后启动服务器即可获得邀请码。')
      dashboardData.refreshInstanceState()
    } catch (e) {
      setAuthMsg(e instanceof Error ? e.message : '登录授权发起失败')
    } finally {
      setAuthBusy(false)
    }
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
              onClick={() => { void handleAuthLogin() }}
              disabled={authBusy || serverRunning}
              title={serverRunning ? '请先停止服务器再登录授权' : '用已保存账号登录 Steam 授权以开启邀请码'}
            >
              {authBusy ? '登录中…' : '登录授权'}
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
          邀请码需要登录 Steam 授权（steam-auth）才能生成，当前未登录。
          {serverRunning ? '请先停止服务器再点【登录授权】；' : '点【登录授权】用已保存账号登录；'}
          也可先用下方「局域网邀请」IP 直连进入。
        </div>
      ) : null}
      {authMsg ? (
        <div className="sd-srv-hint" style={{ marginTop: 4 }}>{authMsg}</div>
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
