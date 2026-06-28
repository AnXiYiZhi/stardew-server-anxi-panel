import type { StardewPageProps } from '../stardew-routes'

export function DiagnosticsPage(_props: StardewPageProps) {
  return (
    <div className="sd-page">
      <div className="sd-page-header">
        <img
          className="sd-page-icon"
          src="/assets/stardew/ui/icons/icon_nav_diagnostics.png"
          alt=""
        />
        <div>
          <h2 className="sd-page-title">诊断</h2>
          <p className="sd-page-desc">系统健康检查、Docker 服务状态、支持包导出。</p>
        </div>
      </div>

      <div className="sd-feature-list">
        <div className="sd-feature-item connected">
          <span className="sd-dot sd-dot-green" aria-hidden="true" />
          健康检查项读取（已接入）
        </div>
        <div className="sd-feature-item connected">
          <span className="sd-dot sd-dot-green" aria-hidden="true" />
          支持包导出（已接入）
        </div>
        <div className="sd-feature-item pending">
          <span className="sd-dot sd-dot-yellow" aria-hidden="true" />
          健康检查结果展示（待迁移）
        </div>
        <div className="sd-feature-item pending">
          <span className="sd-dot sd-dot-yellow" aria-hidden="true" />
          Docker 服务列表（待迁移）
        </div>
      </div>
    </div>
  )
}
