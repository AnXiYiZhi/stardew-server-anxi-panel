import type { StardewPageProps } from '../stardew-routes'
import { SavesSection } from '../SavesSection'

export function SavesPage({ user, instanceState, dashboardData, onNavigate, saveActionRequest }: StardewPageProps) {
  return (
    <div className="sd-page sd-saves-page">
      <div className="sd-page-header">
        <img
          className="sd-page-icon"
          src="/assets/stardew/ui/icons/icon_nav_saves_chest_image2.png"
          alt=""
        />
        <div>
          <h2 className="sd-page-title">存档管理</h2>
        </div>
      </div>

      <SavesSection
        state={instanceState?.state ?? ''}
        isAdmin={user.role === 'admin'}
        onJobStarted={(_jobId) => {
          dashboardData.requestInviteCodeRefresh()
          dashboardData.refreshJobs()
          dashboardData.refreshInstanceState()
          onNavigate('overview')
        }}
        onStateRefresh={dashboardData.refreshInstanceState}
        onSavesChanged={() => {
          dashboardData.refreshSaves()
          dashboardData.refreshMods()
        }}
        saveActionRequest={saveActionRequest}
      />
    </div>
  )
}
import './SavesPage.css'
