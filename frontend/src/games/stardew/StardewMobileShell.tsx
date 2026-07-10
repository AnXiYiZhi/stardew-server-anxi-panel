import { useState } from 'react'
import type { CurrentUser } from '../../types'
import { stateLabel } from '../../core/helpers'
import { useStardewDashboardData } from './useStardewDashboardData'
import { MobileHomePage } from './mobile/MobileHomePage'
import { MobileControlPage } from './mobile/MobileControlPage'
import { MobilePlayersPage } from './mobile/MobilePlayersPage'
import { MobileModsPage } from './mobile/MobileModsPage'
import { MobileSavesPage } from './mobile/MobileSavesPage'
import './StardewMobileShell.css'

type MobileTabKey = 'overview' | 'server' | 'players' | 'mods' | 'saves' | 'more'

type StardewMobileShellProps = {
  user: CurrentUser
}

const MOBILE_TABS: { key: MobileTabKey; label: string; icon: string }[] = [
  { key: 'overview', label: '总览', icon: '/assets/stardew/ui/icons/icon_nav_overview_map_image2.png' },
  { key: 'server', label: '控制', icon: '/assets/stardew/ui/icons/icon_nav_server_rack_image2.png' },
  { key: 'players', label: '玩家', icon: '/assets/stardew/ui/icons/icon_nav_players_avatar_image2.png' },
  { key: 'mods', label: '模组', icon: '/assets/stardew/ui/icons/icon_nav_mods_crystal_image2.png' },
  { key: 'saves', label: '存档', icon: '/assets/stardew/ui/icons/icon_nav_saves_chest_image2.png' },
  { key: 'more', label: '更多', icon: '/assets/stardew/ui/icons/icon_nav_settings_gear_image2.png' },
]

function mobileStatusText(state: string | undefined, loading: boolean): string {
  if (loading || !state) return '初始化中'
  if (state === 'running') return '运行中'
  if (state === 'stopped') return '已停止'
  return stateLabel(state)
}

function mobileStatusDotClass(state: string | undefined, loading: boolean): string {
  if (loading || !state) return 'sd-dot sd-dot-yellow sd-dot-pulse'
  if (state === 'running') return 'sd-dot sd-dot-green sd-dot-pulse'
  if (state === 'stopped' || state === 'error') return 'sd-dot sd-dot-red'
  return 'sd-dot sd-dot-yellow'
}

export function StardewMobileShell({ user }: StardewMobileShellProps) {
  const dashboardData = useStardewDashboardData()
  const [activeTab, setActiveTab] = useState<MobileTabKey>('overview')

  const statusText = mobileStatusText(dashboardData.instanceState?.state, dashboardData.loading)
  const statusDotClass = mobileStatusDotClass(dashboardData.instanceState?.state, dashboardData.loading)

  return (
    <div className="sd-mshell">
      <header className="sd-mshell-topbar">
        <span className="sd-mshell-brand">Stardew Anxi Panel</span>
        <span className="sd-mshell-status">
          <span className={statusDotClass} aria-hidden="true" />
          <span className="sd-mshell-status-text">{statusText}</span>
        </span>
      </header>

      <main className="sd-mshell-body">
        <div className="sd-mshell-scroll">
        {activeTab === 'overview' ? (
          <MobileHomePage user={user} instanceState={dashboardData.instanceState} dashboardData={dashboardData} />
        ) : activeTab === 'server' ? (
          <MobileControlPage user={user} instanceState={dashboardData.instanceState} dashboardData={dashboardData} />
        ) : activeTab === 'players' ? (
          <MobilePlayersPage user={user} instanceState={dashboardData.instanceState} dashboardData={dashboardData} />
        ) : activeTab === 'mods' ? (
          <MobileModsPage user={user} instanceState={dashboardData.instanceState} dashboardData={dashboardData} />
        ) : activeTab === 'saves' ? (
          <MobileSavesPage user={user} instanceState={dashboardData.instanceState} dashboardData={dashboardData} />
        ) : (
          <section className="sd-mshell-card sd-panel">
            <p className="sd-mshell-card-title">移动端面板建设中</p>
            <p className="sd-mshell-card-hint">更完整的移动端体验正在开发中，敬请期待</p>
          </section>
        )}
        </div>
      </main>

      <nav className="sd-mshell-tabbar" aria-label="移动端主导航">
        {MOBILE_TABS.map((tab) => (
          <button
            key={tab.key}
            type="button"
            className={`sd-mshell-tab${activeTab === tab.key ? ' active' : ''}`}
            aria-current={activeTab === tab.key ? 'page' : undefined}
            onClick={() => setActiveTab(tab.key)}
          >
            <img src={tab.icon} alt="" className="sd-mshell-tab-icon" />
            <span className="sd-mshell-tab-label">{tab.label}</span>
          </button>
        ))}
      </nav>
    </div>
  )
}
