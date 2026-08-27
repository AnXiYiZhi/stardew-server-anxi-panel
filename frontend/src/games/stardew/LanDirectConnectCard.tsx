import { useState } from 'react'
import { copyText } from './copy-text'
import type { StardewDashboardData } from './stardew-routes'

type LanDirectConnectCardProps = {
  dashboardData: StardewDashboardData
  className?: string
}

export function LanDirectConnectCard({ dashboardData, className }: LanDirectConnectCardProps) {
  const [copied, setCopied] = useState(false)
  const [copyError, setCopyError] = useState(false)

  function handleCopy() {
    const address = dashboardData.publicIP?.ip
    if (!address) return
    setCopyError(false)
    void copyText(address).then((ok) => {
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
      <div className="sd-players-invite-row sd-players-public-ip-row">
        <div className="sd-players-invite-copy">
          <span className="sd-players-invite-label">局域网直连</span>
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
              onClick={handleCopy}
              title="复制局域网/IP 直连地址"
            >
              {copied ? '已复制' : '复制'}
            </button>
          ) : null}
          <button
            className="sd-btn-tan sd-players-refresh-btn"
            onClick={() => { void dashboardData.refreshPublicIP(true) }}
            disabled={dashboardData.publicIPRefreshing}
            title="同步当前局域网/IP 直连地址"
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
