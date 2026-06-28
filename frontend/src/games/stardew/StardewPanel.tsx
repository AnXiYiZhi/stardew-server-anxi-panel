import { useEffect, useState } from 'react'
import type { CurrentUser } from '../../types'
import { stateLabel } from '../../core/helpers'
import { parseRoute, routeToPath } from './stardew-routes'
import type { StardewRoute } from './stardew-routes'
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
  { route: 'overview', label: '总览', icon: '/assets/stardew/ui/icons/icon_nav_overview_home.png' },
  { route: 'server', label: '服务器', icon: '/assets/stardew/ui/icons/icon_nav_server_control.png' },
  { route: 'saves', label: '存档', icon: '/assets/stardew/ui/icons/icon_nav_saves.png' },
  { route: 'jobs', label: '任务日志', icon: '/assets/stardew/ui/icons/icon_nav_tasks.png' },
  { route: 'players', label: '玩家', icon: '/assets/stardew/ui/icons/icon_nav_players.png' },
  { route: 'mods', label: '模组', icon: '/assets/stardew/ui/icons/icon_nav_mods.png' },
  { route: 'diagnostics', label: '诊断', icon: '/assets/stardew/ui/icons/icon_nav_diagnostics.png' },
  { route: 'install', label: '安装', icon: '/assets/stardew/ui/icons/icon_sidebar_chicken.png' },
  { route: 'settings', label: '设置', icon: '/assets/stardew/ui/icons/icon_nav_settings.png' },
]

const JOB_STATUS_DOT: Record<string, string> = {
  running: 'sd-dot sd-dot-green sd-dot-pulse',
  queued: 'sd-dot sd-dot-yellow',
  succeeded: 'sd-dot sd-dot-green',
  failed: 'sd-dot sd-dot-red',
  canceled: 'sd-dot sd-dot-gray',
}

function stateStatusDot(state: string | undefined): string {
  if (state === 'running') return 'sd-dot sd-dot-green sd-dot-pulse'
  if (state === 'error') return 'sd-dot sd-dot-red'
  if (state === 'stopped') return 'sd-dot sd-dot-gray'
  return 'sd-dot sd-dot-yellow'
}

function healthSummaryDot(status: string | undefined): string {
  if (status === 'ok') return 'sd-dot sd-dot-green'
  if (status === 'warning') return 'sd-dot sd-dot-yellow'
  if (status === 'error') return 'sd-dot sd-dot-red'
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

  const dashboardData = useStardewDashboardData()
  const { instanceState, jobs, health, versionInfo, saves } = dashboardData

  useEffect(() => {
    const onPop = () => setRoute(parseRoute(window.location.pathname))
    window.addEventListener('popstate', onPop)
    return () => window.removeEventListener('popstate', onPop)
  }, [])

  function navigate(next: StardewRoute) {
    if (next === route) return
    window.history.pushState(null, '', routeToPath(next))
    setRoute(next)
  }

  const pageProps = { user, instanceState, dashboardData, onNavigate: navigate, onLogout }

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

  return (
    <div className="sd-shell">
      {/* ── 顶部状态栏 ──────────────────────────────────────── */}
      <header className="sd-topbar">
        <div className="sd-topbar-brand">
          <img
            className="sd-topbar-logo"
            src="/assets/stardew/ui/icons/icon_sidebar_chicken.png"
            alt="Stardew"
          />
          <span className="sd-topbar-name">Stardew Anxi Panel</span>
          {versionInfo ? (
            <span className="sd-topbar-version">v{versionInfo.version}</span>
          ) : null}
        </div>

        <div className="sd-topbar-divider" />

        {/* 实例状态 */}
        <div className="sd-topbar-state">
          <span className={stateStatusDot(instanceState?.state)} aria-hidden="true" />
          <span className="sd-topbar-state-text">
            {instanceState?.state
              ? stateLabel(instanceState.state)
              : dashboardData.loading
                ? '读取中…'
                : '未知'}
          </span>
        </div>

        {/* 当前存档 */}
        {activeSaveName ? (
          <>
            <div className="sd-topbar-divider" />
            <div className="sd-topbar-save">
              <img
                src="/assets/stardew/ui/icons/icon_top_summary_save.png"
                alt=""
                className="sd-topbar-meta-icon"
              />
              <span className="sd-topbar-state-text">{activeSaveName}</span>
            </div>
          </>
        ) : null}

        <div className="sd-topbar-spacer" />

        <div className="sd-topbar-user">
          <span className="sd-topbar-username">{user.username}</span>
          <span className="sd-tag sd-tag-blue">{user.role}</span>
          <button className="sd-topbar-logout-btn" onClick={onLogout}>
            登出
          </button>
        </div>
      </header>

      {/* ── 左侧导航 ────────────────────────────────────────── */}
      <nav className="sd-sidebar" aria-label="主导航">
        {NAV_ENTRIES.slice(0, 7).map((entry) => (
          <button
            key={entry.route}
            className={`sd-nav-item${route === entry.route ? ' active' : ''}`}
            data-route={entry.route}
            aria-current={route === entry.route ? 'page' : undefined}
            onClick={() => navigate(entry.route)}
          >
            <img className="sd-nav-icon" src={entry.icon} alt="" />
            <span>{entry.label}</span>
          </button>
        ))}

        <div className="sd-nav-divider" />

        {NAV_ENTRIES.slice(7).map((entry) => (
          <button
            key={entry.route}
            className={`sd-nav-item${route === entry.route ? ' active' : ''}`}
            data-route={entry.route}
            aria-current={route === entry.route ? 'page' : undefined}
            onClick={() => navigate(entry.route)}
          >
            <img className="sd-nav-icon" src={entry.icon} alt="" />
            <span>{entry.label}</span>
          </button>
        ))}
      </nav>

      {/* ── 主内容区 ─────────────────────────────────────────── */}
      <main className="sd-main">{renderPage()}</main>

      {/* ── 右侧 OpsRail ────────────────────────────────────── */}
      <aside className="sd-opsrail" aria-label="任务状态">

        {/* 健康摘要 */}
        <section className="sd-opsrail-section">
          <p className="sd-opsrail-heading">系统健康</p>
          {health ? (
            <div className="sd-opsrail-job">
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
          <button className="sd-opsrail-link" onClick={() => navigate('diagnostics')}>
            查看诊断 →
          </button>
        </section>

        {/* 进行中的任务 */}
        {activeJobs.length > 0 && (
          <section className="sd-opsrail-section">
            <p className="sd-opsrail-heading">进行中</p>
            {activeJobs.map((job) => (
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
            ))}
          </section>
        )}

        {/* 近期任务 */}
        <section className="sd-opsrail-section">
          <p className="sd-opsrail-heading">近期任务</p>
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
          <button className="sd-opsrail-link" onClick={() => navigate('jobs')}>
            查看全部任务 →
          </button>
        </section>

        {/* Mod 重启提示 */}
        {dashboardData.mods?.restartRequired ? (
          <section className="sd-opsrail-section">
            <p className="sd-opsrail-heading">注意</p>
            <div className="sd-opsrail-job">
              <div className="sd-opsrail-job-meta">
                <span className="sd-dot sd-dot-yellow" aria-hidden="true" />
                <span className="sd-opsrail-job-type">Mod 变更需要重启</span>
              </div>
            </div>
            <button className="sd-opsrail-link" onClick={() => navigate('mods')}>
              查看模组 →
            </button>
          </section>
        ) : null}
      </aside>
    </div>
  )
}
