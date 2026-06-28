import type { StardewPageProps } from '../stardew-routes'
import { SavesSection } from '../SavesSection'

export function SavesPage({ user, instanceState, dashboardData, onNavigate }: StardewPageProps) {
  return (
    <div className="sd-page">
      <div className="sd-page-header">
        <img
          className="sd-page-icon"
          src="/assets/stardew/ui/icons/icon_nav_saves.png"
          alt=""
        />
        <div>
          <h2 className="sd-page-title">存档管理</h2>
          <p className="sd-page-desc">查看、选择、创建或上传 Stardew Valley 存档。</p>
        </div>
      </div>

      <SavesSection
        state={instanceState?.state ?? ''}
        isAdmin={user.role === 'admin'}
        onJobStarted={(_jobId) => {
          dashboardData.refreshJobs()
          dashboardData.refreshInstanceState()
          onNavigate('jobs')
        }}
        onStateRefresh={dashboardData.refreshInstanceState}
        onSavesChanged={dashboardData.refreshSaves}
      />
    </div>
  )
}
