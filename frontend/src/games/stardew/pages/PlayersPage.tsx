import type { StardewPageProps } from '../stardew-routes'

export function PlayersPage(_props: StardewPageProps) {
  return (
    <div className="sd-page">
      <div className="sd-page-header">
        <img
          className="sd-page-icon"
          src="/assets/stardew/ui/icons/icon_nav_players.png"
          alt=""
        />
        <div>
          <h2 className="sd-page-title">玩家管理</h2>
          <p className="sd-page-desc">查看在线玩家、角色槽位、踢出 / 白名单等（后端待接入）。</p>
        </div>
      </div>

      <div className="sd-state-card">
        <div className="sd-state-row">
          <span className="sd-dot sd-dot-yellow" aria-hidden="true" />
          <span className="sd-state-value" style={{ fontWeight: 400 }}>
            后端玩家 API 尚未接入，此页面暂无数据。
          </span>
        </div>
      </div>

      <div className="sd-feature-list">
        <div className="sd-feature-item pending">
          <span className="sd-dot sd-dot-yellow" aria-hidden="true" />
          在线玩家列表 / 角色槽（后端待接入）
        </div>
        <div className="sd-feature-item pending">
          <span className="sd-dot sd-dot-yellow" aria-hidden="true" />
          踢出玩家 / 白名单管理（后端待接入）
        </div>
        <div className="sd-feature-item pending">
          <span className="sd-dot sd-dot-yellow" aria-hidden="true" />
          玩家活动历史（后端待接入）
        </div>
      </div>
    </div>
  )
}
