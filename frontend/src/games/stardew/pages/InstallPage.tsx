import type { StardewPageProps } from '../stardew-routes'

export function InstallPage({ instanceState }: StardewPageProps) {
  const isInstalled = instanceState !== null

  return (
    <div className="sd-page">
      <div className="sd-page-header">
        <img
          className="sd-page-icon"
          src="/assets/stardew/ui/icons/icon_sidebar_chicken.png"
          alt=""
        />
        <div>
          <h2 className="sd-page-title">安装</h2>
          <p className="sd-page-desc">配置并安装 Stardew Valley 服务器（含 SMAPI）。</p>
        </div>
      </div>

      <div className="sd-state-card">
        <div className="sd-state-row">
          <span className="sd-state-label">安装状态</span>
          {isInstalled ? (
            <>
              <span className="sd-dot sd-dot-green" aria-hidden="true" />
              <span className="sd-state-value">已安装</span>
            </>
          ) : (
            <>
              <span className="sd-dot sd-dot-gray" aria-hidden="true" />
              <span className="sd-state-value">未安装</span>
            </>
          )}
        </div>
      </div>

      <div className="sd-feature-list">
        <div className="sd-feature-item connected">
          <span className="sd-dot sd-dot-green" aria-hidden="true" />
          当前版本信息、安装状态读取（已接入）
        </div>
        <div className="sd-feature-item pending">
          <span className="sd-dot sd-dot-yellow" aria-hidden="true" />
          安装向导、Steam 账号配置、SMAPI 版本选择（待迁移）
        </div>
        <div className="sd-feature-item pending">
          <span className="sd-dot sd-dot-yellow" aria-hidden="true" />
          Steam Guard 验证输入（待迁移）
        </div>
        <div className="sd-feature-item pending">
          <span className="sd-dot sd-dot-yellow" aria-hidden="true" />
          安装进度流式日志（待迁移）
        </div>
      </div>
    </div>
  )
}
