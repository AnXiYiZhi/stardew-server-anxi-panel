import { Suspense, lazy, useLayoutEffect, useRef, useState } from 'react'
import type { CurrentUser } from '../../types'
import { stateLabel } from '../../core/helpers'
import { routeToPath } from './stardew-routes'
import type { StardewRoute } from './stardew-routes'
import { useStardewDashboardData } from './useStardewDashboardData'
import { UpdateDetailsDialog } from './UpdateDetailsDialog'
import { panelUpdateSurface } from './panel-update-machine'
import { useStardewLifecycleActions } from './useStardewLifecycleActions'
import './StardewMobileShell.css'

const MobileHomePage = lazy(() => import('./mobile/MobileHomePage').then((m) => ({ default: m.MobileHomePage })))
const MobileControlPage = lazy(() =>
  import('./mobile/MobileControlPage').then((m) => ({ default: m.MobileControlPage })),
)
const MobilePlayersPage = lazy(() =>
  import('./mobile/MobilePlayersPage').then((m) => ({ default: m.MobilePlayersPage })),
)
const MobileModsPage = lazy(() => import('./mobile/MobileModsPage').then((m) => ({ default: m.MobileModsPage })))
const MobileSavesPage = lazy(() => import('./mobile/MobileSavesPage').then((m) => ({ default: m.MobileSavesPage })))

function MobilePageLoadingFallback() {
  return (
    <section className="sd-mshell-card sd-panel">
      <p className="sd-mshell-card-title">加载中…</p>
    </section>
  )
}

type MobileTabKey = 'overview' | 'server' | 'players' | 'mods' | 'saves' | 'more'

type StardewMobileShellProps = {
  user: CurrentUser
  instanceId: string
  onLogout?: () => void
  onUseDesktop?: () => void
  onBackToWorlds: () => void
}

const MOBILE_TABS: { key: MobileTabKey; label: string; icon: string }[] = [
  { key: 'overview', label: '总览', icon: '/assets/stardew/ui/icons/icon_nav_overview_map_image2.png' },
  { key: 'server', label: '控制', icon: '/assets/stardew/ui/icons/icon_nav_server_rack_image2.png' },
  { key: 'players', label: '玩家', icon: '/assets/stardew/ui/icons/icon_nav_players_avatar_image2.png' },
  { key: 'mods', label: '模组', icon: '/assets/stardew/ui/icons/icon_nav_mods_crystal_image2.png' },
  { key: 'saves', label: '存档', icon: '/assets/stardew/ui/icons/icon_nav_saves_chest_image2.png' },
  { key: 'more', label: '更多', icon: '/assets/stardew/ui/icons/icon_nav_settings_gear_image2.png' },
]

const MOBILE_SHELL_MOUNTED_CLASS = 'sd-mobile-shell-mounted'

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

export function StardewMobileShell({ user, instanceId, onLogout, onUseDesktop, onBackToWorlds }: StardewMobileShellProps) {
  const dashboardData = useStardewDashboardData(instanceId)
  const lifecycleActions = useStardewLifecycleActions({
    instanceState: dashboardData.instanceState,
    dashboardData,
    isAdmin: user.role === 'admin',
  })
  const [activeTab, setActiveTab] = useState<MobileTabKey>(() =>
    window.location.pathname.endsWith('/player-mods') || window.location.pathname.endsWith('/players')
      ? 'players'
      : 'overview',
  )
  const mainScrollRef = useRef<HTMLDivElement | null>(null)

  useLayoutEffect(() => {
    const appRoot = document.getElementById('root')
    document.body.classList.add(MOBILE_SHELL_MOUNTED_CLASS)
    appRoot?.classList.add(MOBILE_SHELL_MOUNTED_CLASS)

    return () => {
      document.body.classList.remove(MOBILE_SHELL_MOUNTED_CLASS)
      appRoot?.classList.remove(MOBILE_SHELL_MOUNTED_CLASS)
    }
  }, [])

  useLayoutEffect(() => {
    const mainScroll = mainScrollRef.current
    if (!mainScroll) return
    mainScroll.scrollTop = 0
    mainScroll.scrollLeft = 0
  }, [activeTab])

  const statusText = lifecycleActions.restartInProgress
    ? '正在重启'
    : mobileStatusText(dashboardData.instanceState?.state, dashboardData.loading)
  const statusDotClass = lifecycleActions.restartInProgress
    ? 'sd-dot sd-dot-yellow sd-dot-pulse'
    : mobileStatusDotClass(dashboardData.instanceState?.state, dashboardData.loading)
  const updateSurface = panelUpdateSurface(dashboardData.updateStatus, dashboardData.updateApply, dashboardData.versionInfo)

  const useDesktopRoute = (route?: StardewRoute) => {
	if (route) {
	  window.history.pushState({}, '', routeToPath(route, undefined, instanceId))
	  if (route === 'install') window.dispatchEvent(new PopStateEvent('popstate'))
	}
    onUseDesktop?.()
  }

  return (
    <div className="sd-mshell">
      <header className="sd-mshell-topbar">
        <button
          type="button"
          className="sd-mshell-worlds"
          onClick={onBackToWorlds}
          aria-label="返回世界列表"
        >
          ← 世界
        </button>
        <span className="sd-mshell-brand">Stardew Anxi Panel</span>
        <button
          type="button"
          className={`sd-mshell-update sd-mshell-update--${updateSurface.tone}`}
          onClick={dashboardData.openUpdateDialog}
          aria-label={`面板更新：${updateSurface.topbarText}`}
        >
          {updateSurface.mobileTopbarText}
        </button>
        <span className="sd-mshell-status">
          <span className={statusDotClass} aria-hidden="true" />
          <span className="sd-mshell-status-text">{statusText}</span>
        </span>
      </header>

      <main className="sd-mshell-body">
        <div ref={mainScrollRef} className="sd-mshell-scroll" tabIndex={0} aria-label="面板主内容">
        <Suspense fallback={<MobilePageLoadingFallback />}>
        {activeTab === 'overview' ? (
          <MobileHomePage
            user={user}
            instanceState={dashboardData.instanceState}
            dashboardData={dashboardData}
            onUseDesktop={useDesktopRoute}
          />
        ) : activeTab === 'server' ? (
          <MobileControlPage
            user={user}
            instanceState={dashboardData.instanceState}
            dashboardData={dashboardData}
            restartInProgress={lifecycleActions.restartInProgress}
            onPlayerAuthRestart={lifecycleActions.handleRestart}
          />
        ) : activeTab === 'players' ? (
          <MobilePlayersPage
            user={user}
            instanceId={instanceId}
            instanceState={dashboardData.instanceState}
            dashboardData={dashboardData}
          />
        ) : activeTab === 'mods' ? (
          <MobileModsPage user={user} instanceState={dashboardData.instanceState} dashboardData={dashboardData} />
        ) : activeTab === 'saves' ? (
          <MobileSavesPage user={user} instanceState={dashboardData.instanceState} dashboardData={dashboardData} />
        ) : (
          <section className="sd-mshell-card sd-panel">
            <p className="sd-mshell-card-title">更多功能</p>
            <p className="sd-mshell-card-hint">任务日志、诊断、安装和设置可在完整桌面版中使用。</p>
            <div className="sd-mshell-more-actions">
              <button type="button" className="sd-btn-green" onClick={onUseDesktop} disabled={!onUseDesktop}>
                切换到完整桌面版
              </button>
              <button type="button" className="sd-btn-delete" onClick={onLogout} disabled={!onLogout}>
                退出登录
              </button>
            </div>
          </section>
        )}
        </Suspense>
        </div>
      </main>
      <UpdateDetailsDialog user={user} dashboardData={dashboardData} />

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
