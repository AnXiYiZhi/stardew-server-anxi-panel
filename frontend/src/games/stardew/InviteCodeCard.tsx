import { useState } from 'react'
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
  label = '邀请加入码',
  description = '分享此代码邀请新玩家加入服务器',
}: InviteCodeCardProps) {
  const [copied, setCopied] = useState(false)
  const [copyError, setCopyError] = useState(false)

  const state = instanceState?.state ?? null
  const canRefreshInvite = state === 'running' || state === 'starting'

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

  return (
    <div className={['sd-invite-card-wrap', className].filter(Boolean).join(' ')}>
      <div className="sd-players-invite-row">
        <div className="sd-players-invite-copy">
          <span className="sd-players-invite-label">{label}</span>
          {description ? <span>{description}</span> : null}
        </div>
        {dashboardData.inviteCode ? (
          <span className="sd-players-invite-code">{dashboardData.inviteCode}</span>
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
          <button
            className="sd-btn-tan sd-players-refresh-btn"
            onClick={() => { void dashboardData.refreshInviteCode() }}
            disabled={!canRefreshInvite}
            title={canRefreshInvite ? '刷新邀请码' : '服务器未运行时无法获取邀请码'}
          >
            刷新
          </button>
        </div>
      </div>
      {copyError ? (
        <div className="sd-srv-hint" style={{ color: '#b94040', marginTop: 4 }}>
          复制失败，请手动选取邀请码文字。
        </div>
      ) : null}
    </div>
  )
}
