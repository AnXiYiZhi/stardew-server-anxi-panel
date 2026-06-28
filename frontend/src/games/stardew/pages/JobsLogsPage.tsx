import type { StardewPageProps } from '../stardew-routes'

export function JobsLogsPage(_props: StardewPageProps) {
  return (
    <div className="sd-page">
      <div className="sd-page-header">
        <img
          className="sd-page-icon"
          src="/assets/stardew/ui/icons/icon_nav_tasks.png"
          alt=""
        />
        <div>
          <h2 className="sd-page-title">任务日志</h2>
          <p className="sd-page-desc">查看后台任务执行历史，包含 SSE 实时流式日志。</p>
        </div>
      </div>

      <div className="sd-feature-list">
        <div className="sd-feature-item connected">
          <span className="sd-dot sd-dot-green" aria-hidden="true" />
          任务列表读取（已接入，见右侧 OpsRail）
        </div>
        <div className="sd-feature-item pending">
          <span className="sd-dot sd-dot-yellow" aria-hidden="true" />
          任务选择 + SSE 日志流（待迁移）
        </div>
        <div className="sd-feature-item pending">
          <span className="sd-dot sd-dot-yellow" aria-hidden="true" />
          清空任务历史（待迁移）
        </div>
        <div className="sd-feature-item pending">
          <span className="sd-dot sd-dot-yellow" aria-hidden="true" />
          健康检查测试任务（待迁移）
        </div>
      </div>
    </div>
  )
}
