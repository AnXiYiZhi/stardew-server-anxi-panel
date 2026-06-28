import type { StardewPageProps } from '../stardew-routes'

export function ModsPage(_props: StardewPageProps) {
  return (
    <div className="sd-page">
      <div className="sd-page-header">
        <img
          className="sd-page-icon"
          src="/assets/stardew/ui/icons/icon_nav_mods.png"
          alt=""
        />
        <div>
          <h2 className="sd-page-title">模组管理</h2>
          <p className="sd-page-desc">安装、更新、删除 SMAPI 模组；上传自定义 mod ZIP 包。</p>
        </div>
      </div>

      <div className="sd-feature-list">
        <div className="sd-feature-item connected">
          <span className="sd-dot sd-dot-green" aria-hidden="true" />
          已安装模组列表读取（已接入）
        </div>
        <div className="sd-feature-item pending">
          <span className="sd-dot sd-dot-yellow" aria-hidden="true" />
          从 Nexus / URL 安装模组（待迁移）
        </div>
        <div className="sd-feature-item pending">
          <span className="sd-dot sd-dot-yellow" aria-hidden="true" />
          上传本地 ZIP 包（待迁移）
        </div>
        <div className="sd-feature-item pending">
          <span className="sd-dot sd-dot-yellow" aria-hidden="true" />
          删除模组（待迁移）
        </div>
      </div>
    </div>
  )
}
