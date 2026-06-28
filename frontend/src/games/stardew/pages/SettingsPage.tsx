import type { StardewPageProps } from '../stardew-routes'

export function SettingsPage({ user }: StardewPageProps) {
  return (
    <div className="sd-page">
      <div className="sd-page-header">
        <img
          className="sd-page-icon"
          src="/assets/stardew/ui/icons/icon_nav_settings.png"
          alt=""
        />
        <div>
          <h2 className="sd-page-title">设置</h2>
          <p className="sd-page-desc">面板用户管理、审计日志、版本信息。</p>
        </div>
      </div>

      <div className="sd-state-card">
        <div className="sd-state-row">
          <span className="sd-state-label">当前用户</span>
          <span className="sd-state-value">{user.username}</span>
          <span className="sd-tag sd-tag-blue" style={{ marginLeft: 4 }}>{user.role}</span>
        </div>
      </div>

      <div className="sd-feature-list">
        <div className="sd-feature-item connected">
          <span className="sd-dot sd-dot-green" aria-hidden="true" />
          当前用户信息（已接入）
        </div>
        <div className="sd-feature-item pending">
          <span className="sd-dot sd-dot-yellow" aria-hidden="true" />
          面板用户列表 / 创建 / 角色修改（待迁移）
        </div>
        <div className="sd-feature-item pending">
          <span className="sd-dot sd-dot-yellow" aria-hidden="true" />
          审计日志（待迁移）
        </div>
        <div className="sd-feature-item pending">
          <span className="sd-dot sd-dot-yellow" aria-hidden="true" />
          版本信息与构建号（待迁移）
        </div>
      </div>
    </div>
  )
}
