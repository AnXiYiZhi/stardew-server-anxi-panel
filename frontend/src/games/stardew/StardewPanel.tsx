import { useEffect, useState } from 'react'
import type { CurrentUser } from '../../types'
import { stateLabel } from '../../core/helpers'
import { parseRoute, routeToPath } from './stardew-routes'
import type { StardewNavigateOptions, StardewRoute, StardewSaveActionRequest } from './stardew-routes'
import { useStardewDashboardData } from './useStardewDashboardData'
import { InstallPage } from './pages/InstallPage'
import { OverviewPage } from './pages/OverviewPage'
import { ServerControlPage } from './pages/ServerControlPage'
import { SavesPage } from './pages/SavesPage'
import { JobsLogsPage } from './pages/JobsLogsPage'
import { PlayersPage } from './pages/PlayersPage'
import { ModsPage } from './pages/ModsPage'
import { DiagnosticsPage } from './pages/DiagnosticsPage'
import { SettingsPage } from './pages/SettingsPage'
import './StardewPanel.css'

type NavEntry = {
  route: StardewRoute
  label: string
  icon: string
}

const NAV_ENTRIES: NavEntry[] = [
  { route: 'overview', label: '总览', icon: '/assets/stardew/ui/icons/icon_nav_overview_map_image2.png' },
  { route: 'server', label: '服务器', icon: '/assets/stardew/ui/icons/icon_nav_server_rack_image2.png' },
  { route: 'saves', label: '存档', icon: '/assets/stardew/ui/icons/icon_nav_saves_chest_image2.png' },
  { route: 'jobs', label: '任务日志', icon: '/assets/stardew/ui/icons/icon_nav_tasks_scroll_image2.png' },
  { route: 'players', label: '玩家', icon: '/assets/stardew/ui/icons/icon_nav_players_avatar_image2.png' },
  { route: 'mods', label: '模组', icon: '/assets/stardew/ui/icons/icon_nav_mods_crystal_image2.png' },
  { route: 'diagnostics', label: '诊断', icon: '/assets/stardew/ui/icons/icon_nav_diagnostics_monitor_image2.png' },
  { route: 'install', label: '安装', icon: '/assets/stardew/ui/icons/icon_nav_install_package_image2.png' },
  { route: 'settings', label: '设置', icon: '/assets/stardew/ui/icons/icon_nav_settings_gear_image2.png' },
]

const RIGHT_RAIL_TITLE_ICONS = {
  health: '/assets/stardew/ui/icons/icon_right_rail_health_heart_image2.png',
  active: '/assets/stardew/ui/icons/icon_right_rail_in_progress_clock_image2.png',
  recent: '/assets/stardew/ui/icons/icon_right_rail_recent_tasks_clipboard_image2.png',
} as const

const JOB_STATUS_DOT: Record<string, string> = {
  running: 'sd-dot sd-dot-green sd-dot-pulse',
  queued: 'sd-dot sd-dot-yellow',
  succeeded: 'sd-dot sd-dot-green',
  failed: 'sd-dot sd-dot-red',
  canceled: 'sd-dot sd-dot-gray',
}

function healthSummaryDot(status: string | undefined): string {
  if (status === 'ok') return 'sd-dot sd-dot-green'
  if (status === 'warning') return 'sd-dot sd-dot-yellow'
  if (status === 'error') return 'sd-dot sd-dot-red'
  return 'sd-dot sd-dot-gray'
}

function roleLabel(role: CurrentUser['role']): string {
  return role === 'admin' ? '管理员' : '普通用户'
}

function topbarStatusText(state: string | undefined, loading: boolean): string {
  if (!state) return loading ? '读取中' : '未知'
  if (state === 'running') return '运行中'
  if (state === 'stopped') return '已停止'
  if (state === 'starting') return '启动中'
  if (state === 'stopping') return '停止中'
  if (state === 'error') return '异常'
  return stateLabel(state)
}

function topbarStatusDotClassName(state: string | undefined, loading: boolean): string {
  if (state === 'running') return 'sd-dot sd-dot-green sd-dot-pulse'
  if (state === 'starting' || state === 'stopping' || loading) return 'sd-dot sd-dot-yellow sd-dot-pulse'
  if (state === 'stopped' || state === 'error' || state === 'ready_to_start' || state === 'save_required') {
    return 'sd-dot sd-dot-red'
  }
  if (state === 'game_installed') return 'sd-dot sd-dot-yellow'
  return 'sd-dot sd-dot-gray'
}

export function StardewPanel({
  user,
  onLogout,
}: {
  user: CurrentUser
  onLogout: () => void
}) {
  const [route, setRoute] = useState<StardewRoute>(() =>
    parseRoute(window.location.pathname),
  )
  const [saveActionRequest, setSaveActionRequest] = useState<StardewSaveActionRequest | null>(null)

  const dashboardData = useStardewDashboardData()
  const { instanceState, jobs, health, versionInfo, saves } = dashboardData

  useEffect(() => {
    const onPop = () => setRoute(parseRoute(window.location.pathname))
    window.addEventListener('popstate', onPop)
    return () => window.removeEventListener('popstate', onPop)
  }, [])

  function navigate(next: StardewRoute, options?: StardewNavigateOptions) {
    if (options?.saveAction) {
      setSaveActionRequest({ action: options.saveAction, nonce: Date.now() })
    } else if (next !== 'saves') {
      setSaveActionRequest(null)
    }
    if (next === route) return
    window.history.pushState(null, '', routeToPath(next))
    setRoute(next)
  }

  const pageProps = { user, instanceState, dashboardData, onNavigate: navigate, saveActionRequest, onLogout }

  function renderPage() {
    switch (route) {
      case 'install':
        return <InstallPage {...pageProps} />
      case 'overview':
        return <OverviewPage {...pageProps} />
      case 'server':
        return <ServerControlPage {...pageProps} />
      case 'saves':
        return <SavesPage {...pageProps} />
      case 'jobs':
        return <JobsLogsPage {...pageProps} />
      case 'players':
        return <PlayersPage {...pageProps} />
      case 'mods':
        return <ModsPage {...pageProps} />
      case 'diagnostics':
        return <DiagnosticsPage {...pageProps} />
      case 'settings':
        return <SettingsPage {...pageProps} />
    }
  }

  const activeJobs = jobs.filter((j) => j.status === 'running' || j.status === 'queued')
  const recentIdleJobs = jobs
    .filter((j) => j.status !== 'running' && j.status !== 'queued')
    .slice(0, 5)

  const activeSaveName = saves?.activeSaveName
  const healthStatus = health?.status
  const errorChecks = health?.checks.filter((c) => c.status === 'error').length ?? 0
  const warnChecks = health?.checks.filter((c) => c.status === 'warning').length ?? 0
  const activeSave = activeSaveName
    ? saves?.saves.find((save) => save.isActive || save.name === activeSaveName) ?? null
    : null
  const topbarFarmName = activeSave?.farmName || activeSave?.name || activeSaveName || '选择存档'
  const topbarVersion = versionInfo?.version ? `v${versionInfo.version}` : 'v--'
  const topbarStateLabel = topbarStatusText(instanceState?.state, dashboardData.loading)
  const topbarStatusDotClass = topbarStatusDotClassName(instanceState?.state, dashboardData.loading)
  const topbarStatusUsesGreenIcon = topbarStatusDotClass.includes('sd-dot-green')
  const topbarStatusClassName = `sd-topbar-status sd-topbar-status-${instanceState?.state ?? 'unknown'}`

  return (
    <div className="sd-shell">
      {/* ── 顶部状态栏 ──────────────────────────────────────── */}
      <header className="sd-topbar" aria-label="Stardew Anxi Panel top bar">
        <div className="sd-topbar-bg" aria-hidden="true">
          <span className="sd-topbar-bg-left" />
          <span className="sd-topbar-bg-mid" />
          <span className="sd-topbar-bg-right" />
        </div>

        <div className="sd-topbar-brand" aria-label="Stardew Anxi Panel">
          <img
            className="sd-topbar-brand-icon"
            src="/assets/stardew/ui/topbar/icon_topbar_chicken_image2_v2.png"
            alt=""
          />
          <span className="sd-topbar-brand-copy">
            <span className="sd-topbar-brand-text">Stardew Anxi Panel</span>
            <img
              className="sd-topbar-brand-leaf"
              src="/assets/stardew/ui/topbar/icon_topbar_leaf_image2_v2.png"
              alt=""
            />
          </span>
        </div>

        <button
          type="button"
          className={topbarStatusClassName}
          onClick={() => navigate('server')}
          aria-label={`服务器状态：${topbarStateLabel}`}
          title={`服务器状态：${topbarStateLabel}`}
        >
          {topbarStatusUsesGreenIcon ? (
            <img
              className="sd-topbar-green-dot sd-topbar-status-dot"
              src="/assets/stardew/ui/topbar/icon_topbar_green_dot_image2_v2.png"
              alt=""
            />
          ) : (
            <span className={topbarStatusDotClass} aria-hidden="true" />
          )}
          <span className="sd-topbar-status-text">{topbarStateLabel}</span>
        </button>

        <button
          type="button"
          className="sd-topbar-save"
          onClick={() => navigate('saves')}
          aria-label={`当前农场：${topbarFarmName}`}
          title={`当前农场：${topbarFarmName}`}
        >
          <span className="sd-topbar-frame sd-topbar-save-frame" aria-hidden="true">
            <span className="sd-topbar-frame-left" />
            <span className="sd-topbar-frame-mid" />
            <span className="sd-topbar-frame-right" />
          </span>
          <img
            className="sd-topbar-save-icon"
            src="/assets/stardew/ui/topbar/icon_topbar_farm_image2_v2.png"
            alt=""
          />
          <span className="sd-topbar-save-name">{topbarFarmName}</span>
          <img
            className="sd-topbar-dropdown"
            src="/assets/stardew/ui/topbar/icon_topbar_dropdown_arrow_image2_v2.png"
            alt=""
          />
        </button>

        <button
          type="button"
          className="sd-topbar-version"
          onClick={() => navigate('settings')}
          aria-label={`面板版本：${topbarVersion}`}
          title={`面板版本：${topbarVersion}`}
        >
          <span>{topbarVersion}</span>
        </button>

        <button
          type="button"
          className="sd-topbar-user"
          onClick={() => navigate('settings')}
          aria-label={`${user.username} · ${roleLabel(user.role)}`}
          title={`${user.username} · ${roleLabel(user.role)}`}
        >
          <span className="sd-topbar-frame sd-topbar-user-frame" aria-hidden="true">
            <span className="sd-topbar-frame-left" />
            <span className="sd-topbar-frame-mid" />
            <span className="sd-topbar-frame-right" />
          </span>
          <img
            className="sd-topbar-user-avatar"
            src="/assets/stardew/ui/topbar/icon_topbar_user_avatar_image2_v2.png"
            alt=""
          />
          <span className="sd-topbar-user-role">{roleLabel(user.role)}</span>
          <img
            className="sd-topbar-green-dot sd-topbar-user-dot"
            src="/assets/stardew/ui/topbar/icon_topbar_green_dot_image2_v2.png"
            alt=""
          />
          <img
            className="sd-topbar-dropdown"
            src="/assets/stardew/ui/topbar/icon_topbar_dropdown_arrow_image2_v2.png"
            alt=""
          />
        </button>

        <button
          type="button"
          className="sd-topbar-logout-btn"
          onClick={onLogout}
          aria-label="登出"
          title="登出"
        >
          <img
            className="sd-topbar-logout-icon"
            src="/assets/stardew/ui/topbar/icon_topbar_logout_image2_v2.png"
            alt=""
          />
          <span>登出</span>
        </button>
      </header>

      {/* ── 左侧导航 ────────────────────────────────────────── */}
      <nav className="sd-sidebar" aria-label="主导航">
        <div className="sd-nav-list">
          {NAV_ENTRIES.map((entry) => (
            <div className="sd-nav-row" key={entry.route}>
              <button
                className={`sd-nav-item${route === entry.route ? ' active' : ''}`}
                data-route={entry.route}
                aria-current={route === entry.route ? 'page' : undefined}
                aria-label={entry.label}
                title={entry.label}
                onClick={() => navigate(entry.route)}
              >
                <img className="sd-nav-icon" src={entry.icon} alt="" />
                <span className="sd-nav-label">{entry.label}</span>
              </button>
            </div>
          ))}
        </div>
      </nav>

      {/* ── 主内容区 ─────────────────────────────────────────── */}
      <main className="sd-main">
        <div className="sd-main-scroll">{renderPage()}</div>
      </main>

      {/* ── 右侧 OpsRail ────────────────────────────────────── */}
      <aside className="sd-opsrail" aria-label="任务状态">
        <div className="sd-opsrail-bg" aria-hidden="true" />

        <div className="sd-opsrail-stack">
          {/* 健康摘要 */}
          <section className="sd-ops-card sd-ops-card-health sd-opsrail-section sd-opsrail-health">
            <h2 className="sd-opsrail-heading">
              <img className="sd-opsrail-title-icon" src={RIGHT_RAIL_TITLE_ICONS.health} alt="" />
              <span>系统健康</span>
            </h2>
            {health ? (
              <div className="sd-opsrail-job sd-opsrail-health-summary">
                <div className="sd-opsrail-job-meta">
                  <span className={healthSummaryDot(healthStatus)} aria-hidden="true" />
                  <span className="sd-opsrail-job-type">
                    {healthStatus === 'ok' ? '全部正常' : `${errorChecks} 错误 · ${warnChecks} 警告`}
                  </span>
                </div>
              </div>
            ) : dashboardData.healthError ? (
              <p className="sd-opsrail-empty">健康检查失败</p>
            ) : (
              <p className="sd-opsrail-empty">检查中…</p>
            )}
            <button type="button" className="sd-opsrail-link" onClick={() => navigate('diagnostics')}>
              查看诊断 →
            </button>
          </section>

          {/* 进行中的任务 */}
          <section className="sd-ops-card sd-ops-card-active sd-opsrail-section sd-opsrail-active">
            <h2 className="sd-opsrail-heading">
              <img className="sd-opsrail-title-icon" src={RIGHT_RAIL_TITLE_ICONS.active} alt="" />
              <span>进行中</span>
            </h2>
            <div className="sd-opsrail-list">
              {activeJobs.length === 0 ? (
                <p className="sd-opsrail-empty">暂无进行中的任务</p>
              ) : (
                activeJobs.map((job) => (
                  <div key={job.id} className="sd-opsrail-job">
                    <span className="sd-opsrail-job-type">{job.type}</span>
                    <div className="sd-opsrail-job-meta">
                      <span
                        className={JOB_STATUS_DOT[job.status] ?? 'sd-dot sd-dot-gray'}
                        aria-hidden="true"
                      />
                      {job.status}
                    </div>
                  </div>
                ))
              )}
            </div>
          </section>

          {/* 近期任务 */}
          <section className="sd-ops-card sd-ops-card-recent sd-opsrail-section sd-opsrail-recent">
            <h2 className="sd-opsrail-heading">
              <img className="sd-opsrail-title-icon" src={RIGHT_RAIL_TITLE_ICONS.recent} alt="" />
              <span>近期任务</span>
            </h2>
            <div className="sd-opsrail-list">
              {recentIdleJobs.length === 0 && activeJobs.length === 0 ? (
                <p className="sd-opsrail-empty">暂无任务记录</p>
              ) : recentIdleJobs.length === 0 ? null : (
                recentIdleJobs.map((job) => (
                  <div key={job.id} className="sd-opsrail-job">
                    <span className="sd-opsrail-job-type">{job.type}</span>
                    <div className="sd-opsrail-job-meta">
                      <span
                        className={JOB_STATUS_DOT[job.status] ?? 'sd-dot sd-dot-gray'}
                        aria-hidden="true"
                      />
                      {job.status}
                    </div>
                  </div>
                ))
              )}
            </div>
            {dashboardData.mods?.restartRequired ? (
              <div className="sd-opsrail-mod-note">
                <div className="sd-opsrail-job-meta">
                  <span className="sd-dot sd-dot-yellow" aria-hidden="true" />
                  <span className="sd-opsrail-job-type">Mod 变更需要重启</span>
                </div>
                <button type="button" className="sd-opsrail-link sd-opsrail-mod-link" onClick={() => navigate('mods')}>
                  查看模组 →
                </button>
              </div>
            ) : null}
            <button type="button" className="sd-opsrail-link" onClick={() => navigate('jobs')}>
              查看全部任务 →
            </button>
          </section>
        </div>
      </aside>
    </div>
  )
}
