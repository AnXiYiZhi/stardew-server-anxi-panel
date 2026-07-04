import type { StardewPageProps } from '../stardew-routes'
import { SavesSection } from '../SavesSection'

export function SavesPage({ user, instanceState, dashboardData, onNavigate, saveActionRequest }: StardewPageProps) {
  return (
    <div className="sd-page sd-saves-page">
      <h2 className="sd-saves-page-title">
        <img
          className="sd-page-icon"
          src="/assets/stardew/ui/icons/icon_nav_saves.png"
          alt=""
        />
        存档管理
      </h2>

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
