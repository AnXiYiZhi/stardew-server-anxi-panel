import { useEffect, useRef, useState } from 'react'
import type { BackupPolicy, CurrentUser, Job, JobLog, ResourceMetricSample, RestartSchedule } from '../../types'
import { getInstanceMetrics, getRestartSchedule, getSaveBackupPolicy } from '../../api'
import { jobDisplayName, stateLabel } from '../../core/helpers'
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

function metricPercentText(value: number | null | undefined): string {
  return value == null ? '—' : `${Math.round(value)}%`
}

function metricPercentWidth(value: number | null | undefined): number {
  if (value == null) return 0
  return Math.max(0, Math.min(100, value))
}

type HealthStatLevel = 'ok' | 'warn' | 'crit'

function usageLevel(value: number | null | undefined): HealthStatLevel {
  if (value == null) return 'ok'
  if (value >= 85) return 'crit'
  if (value >= 60) return 'warn'
  return 'ok'
}

function latencyLevel(ms: number | null): HealthStatLevel {
  if (ms == null) return 'ok'
  if (ms >= 300) return 'crit'
  if (ms >= 100) return 'warn'
  return 'ok'
}

const DAY_MS = 24 * 60 * 60 * 1000
const DEFAULT_JOB_DURATION_MS = 60_000
const REMOTE_INSTALL_JOB_TYPES = new Set(['mod_remote_install', 'mod_nexus_install'])
const DOWNLOAD_PROGRESS_RE = /下载进度：已下载[\s\S]*?[（(]\s*([0-9]+(?:\.[0-9]+)?)%\s*[）)]/
const OPS_RAIL_COLLAPSE_MAIN_WIDTH = 820
const OPS_RAIL_EXPAND_MAIN_WIDTH = 880

function clampNumber(min: number, value: number, max: number): number {
  return Math.max(min, Math.min(max, value))
}

function expandedMainWidthForShell(shellWidth: number): number {
  const sidebarWidth = clampNumber(210, shellWidth * 0.168, 252)
  const opsRailWidth = clampNumber(340, shellWidth * 0.27, 430)
  return shellWidth - sidebarWidth - opsRailWidth
}

function shouldAutoCollapseOpsRail(shellWidth: number, currentlyCollapsed: boolean): boolean {
  if (shellWidth <= 640) return false
  const expandedMainWidth = expandedMainWidthForShell(shellWidth)
  const threshold = currentlyCollapsed ? OPS_RAIL_EXPAND_MAIN_WIDTH : OPS_RAIL_COLLAPSE_MAIN_WIDTH
  return expandedMainWidth < threshold
}

function formatCountdown(ms: number): string {
  const totalSeconds = Math.max(0, Math.floor(ms / 1000))
  const pad = (n: number) => String(n).padStart(2, '0')
  const hours = Math.floor(totalSeconds / 3600)
  const minutes = Math.floor((totalSeconds % 3600) / 60)
  return `${pad(hours)}:${pad(minutes)}:${pad(totalSeconds % 60)}`
}

// 普通运行中任务按同类型历史耗时估算；远程 Mod 安装优先读下载日志百分比，
// 避免大文件下载阶段在右栏长期显示为未知或直接跳到 95%。
function expectedJobDurationMs(type: string, jobs: Job[]): number {
  const durations = jobs
    .filter((j) => j.type === type && j.status === 'succeeded' && j.startedAt && j.finishedAt)
    .map((j) => new Date(j.finishedAt as string).getTime() - new Date(j.startedAt as string).getTime())
    .filter((ms) => ms > 0)
    .sort((a, b) => a - b)
  if (durations.length === 0) return DEFAULT_JOB_DURATION_MS
  return durations[Math.floor(durations.length / 2)]
}

function isRemoteInstallJob(type: string): boolean {
  return REMOTE_INSTALL_JOB_TYPES.has(type)
}

function remoteInstallLogPercent(logs: JobLog[]): number | null {
  for (let i = logs.length - 1; i >= 0; i -= 1) {
    const message = logs[i].message
    if (message.includes('正在校验并安装 Mod')) return 92
    if (message.includes('已完成') || message.includes('安装完成')) return 98
    const match = message.match(DOWNLOAD_PROGRESS_RE)
    if (match) {
      const downloadPercent = Number.parseFloat(match[1])
      if (Number.isFinite(downloadPercent)) {
        return Math.max(12, Math.min(90, Math.round(12 + downloadPercent * 0.78)))
      }
    }
    if (message.includes('远程压缩包大小')) return 12
    if (message.includes('远程下载服务器已响应')) return 10
    if (message.includes('正在从远程链接下载 Mod 压缩包')) return 8
    if (message.includes('正在连接远程下载服务器')) return 6
    if (message.includes('准备从远程链接安装 Mod')) return 4
    if (message.includes('任务已开始')) return 3
  }
  return null
}

function runningJobPercent(job: Job, jobs: Job[], now: number, logs: JobLog[] = []): number {
  if (job.status !== 'running') return 0
  if (isRemoteInstallJob(job.type)) {
    const logPercent = remoteInstallLogPercent(logs)
    if (logPercent !== null) return logPercent
  }
  const startedAt = new Date(job.startedAt ?? job.createdAt).getTime()
  if (!Number.isFinite(startedAt)) return 0
  const elapsed = now - startedAt
  if (elapsed <= 0) return 0
  const estimated = Math.min(95, Math.round((elapsed / expectedJobDurationMs(job.type, jobs)) * 100))
  return isRemoteInstallJob(job.type) ? Math.min(15, estimated) : estimated
}

// 进行中卡：维护计划/定时备份倒计时 + 运行中任务进度。
// 每秒 tick 只重渲染本组件，不影响主内容区
function OpsRailActiveCard({ jobs, jobLogsByJobId }: { jobs: Job[]; jobLogsByJobId: Record<string, JobLog[]> }) {
  const [schedule, setSchedule] = useState<RestartSchedule | null>(null)
  const [backupPolicy, setBackupPolicy] = useState<BackupPolicy | null>(null)
  const [now, setNow] = useState(() => Date.now())

  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 1000)
    return () => window.clearInterval(timer)
  }, [])

  useEffect(() => {
    let alive = true
    let timer: number | undefined

    async function loadConfig() {
      try {
        const res = await getRestartSchedule()
        if (alive) setSchedule(res.schedule)
      } catch {
        if (alive) setSchedule(null)
      }
      try {
        const res = await getSaveBackupPolicy()
        if (alive) setBackupPolicy(res.policy)
      } catch {
        // 备份策略接口仅管理员可读，普通用户静默隐藏定时备份行
        if (alive) setBackupPolicy(null)
      }
      if (alive) {
        timer = window.setTimeout(() => void loadConfig(), 60_000)
      }
    }

    void loadConfig()
    return () => {
      alive = false
      if (timer != null) window.clearTimeout(timer)
    }
  }, [])

  const activeJobs = jobs.filter((j) => j.status === 'running' || j.status === 'queued')

  const countdowns: { key: string; label: string; at: number }[] = []
  if (schedule?.enabled) {
    const shutdownAt = schedule.nextShutdownAt ? new Date(schedule.nextShutdownAt).getTime() : NaN
    if (Number.isFinite(shutdownAt) && shutdownAt > now) {
      countdowns.push({ key: 'auto-shutdown', label: '自动关机', at: shutdownAt })
    }
    const startupAt = schedule.nextStartupAt ? new Date(schedule.nextStartupAt).getTime() : NaN
    if (Number.isFinite(startupAt) && startupAt > now) {
      countdowns.push({ key: 'auto-startup', label: '自动开机', at: startupAt })
    }
  }
  if (backupPolicy?.scheduledBackups) {
    // 定时备份为每日 scheduledHour 整点（以面板本地时间近似后端本地时间）
    const next = new Date(now)
    next.setHours(backupPolicy.scheduledHour, 0, 0, 0)
    if (next.getTime() <= now) next.setDate(next.getDate() + 1)
    countdowns.push({ key: 'scheduled-backup', label: '定时备份', at: next.getTime() })
  }
  countdowns.sort((a, b) => a.at - b.at)

  return (
    <section className="sd-ops-card sd-ops-card-active sd-opsrail-section sd-opsrail-active">
      <h2 className="sd-opsrail-heading">
        <img className="sd-opsrail-title-icon" src={RIGHT_RAIL_TITLE_ICONS.active} alt="" />
        <span>进行中</span>
      </h2>
      <div className="sd-opsrail-list">
        {countdowns.length === 0 && activeJobs.length === 0 ? (
          <p className="sd-opsrail-empty">暂无进行中的任务</p>
        ) : (
          <>
            {countdowns.map((entry) => {
              // 倒计时事件都是每日一次，进度条按 24h 周期内已经过的比例填充
              const width = Math.max(0, Math.min(100, ((DAY_MS - (entry.at - now)) / DAY_MS) * 100))
              return (
                <div key={entry.key} className="sd-opsrail-hstat sd-opsrail-hstat--info">
                  <div className="sd-opsrail-hstat-row">
                    <span className="sd-opsrail-hstat-orb" aria-hidden="true" />
                    <span className="sd-opsrail-hstat-label">{entry.label}</span>
                    <span className="sd-opsrail-hstat-value">{formatCountdown(entry.at - now)}</span>
                  </div>
                  <div className="sd-opsrail-hstat-bar">
                    <span className="sd-opsrail-hstat-fill" style={{ width: `${width}%` }} />
                  </div>
                </div>
              )
            })}
            {activeJobs.map((job) => {
              const percent = runningJobPercent(job, jobs, now, jobLogsByJobId[job.id] ?? [])
              return (
                <div key={job.id} className="sd-opsrail-hstat">
                  <div className="sd-opsrail-hstat-row">
                    <span className="sd-opsrail-hstat-orb" aria-hidden="true" />
                    <span className="sd-opsrail-hstat-label" title={jobDisplayName(job)}>{jobDisplayName(job)}</span>
                    <span className="sd-opsrail-hstat-value">
                      {job.status === 'queued' ? '排队中' : `${percent}%`}
                    </span>
                  </div>
                  <div className="sd-opsrail-hstat-bar">
                    <span className="sd-opsrail-hstat-fill" style={{ width: `${percent}%` }} />
                  </div>
                </div>
              )
            })}
          </>
        )}
      </div>
    </section>
  )
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
  const [opsRailAutoCollapsed, setOpsRailAutoCollapsed] = useState(() =>
    shouldAutoCollapseOpsRail(window.innerWidth, false),
  )
  const shellRef = useRef<HTMLDivElement | null>(null)

  const dashboardData = useStardewDashboardData()
  const { instanceState, jobs, versionInfo, saves } = dashboardData
  const [metricSample, setMetricSample] = useState<ResourceMetricSample | null>(null)
  const [apiLatencyMs, setApiLatencyMs] = useState<number | null>(null)

  useEffect(() => {
    const onPop = () => setRoute(parseRoute(window.location.pathname))
    window.addEventListener('popstate', onPop)
    return () => window.removeEventListener('popstate', onPop)
  }, [])

  useEffect(() => {
    const shell = shellRef.current
    if (!shell) return

    let frameId: number | null = null
    const measure = () => {
      const shellWidth = shell.getBoundingClientRect().width || window.innerWidth
      setOpsRailAutoCollapsed((current) => shouldAutoCollapseOpsRail(shellWidth, current))
      frameId = null
    }
    const scheduleMeasure = () => {
      if (frameId != null) return
      frameId = window.requestAnimationFrame(measure)
    }

    const resizeObserver =
      typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(scheduleMeasure)
    resizeObserver?.observe(shell)
    window.addEventListener('resize', scheduleMeasure)
    scheduleMeasure()

    return () => {
      resizeObserver?.disconnect()
      window.removeEventListener('resize', scheduleMeasure)
      if (frameId != null) window.cancelAnimationFrame(frameId)
    }
  }, [])

  // 系统健康卡：轮询容器资源指标，网络延迟取本次请求的往返耗时
  useEffect(() => {
    let alive = true
    let timer: number | undefined

    async function loadMetrics() {
      const startedAt = performance.now()
      try {
        const res = await getInstanceMetrics()
        if (!alive) return
        setMetricSample(res.sample)
        setApiLatencyMs(Math.round(performance.now() - startedAt))
      } catch {
        if (!alive) return
        setMetricSample(null)
        setApiLatencyMs(null)
      } finally {
        if (alive) {
          timer = window.setTimeout(() => {
            void loadMetrics()
          }, 5000)
        }
      }
    }

    void loadMetrics()
    return () => {
      alive = false
      if (timer != null) window.clearTimeout(timer)
    }
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
  const railResourceStats = [
    { label: 'CPU 使用率', value: metricSample?.cpuPercent },
    { label: '内存使用率', value: metricSample?.memoryPercent },
    { label: '磁盘使用率', value: metricSample?.diskPercent },
  ]
  const onlineCount = dashboardData.players?.onlineCount
  const maxPlayers = dashboardData.players?.maxPlayers
  const railPlayerSummary =
    onlineCount != null
      ? maxPlayers != null
        ? `${onlineCount}/${maxPlayers}`
        : String(onlineCount)
      : '—'
  const railPlayerLevel: HealthStatLevel = onlineCount === 0 ? 'crit' : 'ok'
  const railLatencyLevel = latencyLevel(apiLatencyMs)
  const activeSave = activeSaveName
    ? saves?.saves.find((save) => save.isActive || save.name === activeSaveName) ?? null
    : null
  const topbarFarmName = activeSave?.farmName || activeSave?.name || activeSaveName || '选择存档'
  const topbarVersion = versionInfo?.version ? `v${versionInfo.version}` : 'v--'
  const topbarStateLabel = topbarStatusText(instanceState?.state, dashboardData.loading)
  const topbarStatusDotClass = topbarStatusDotClassName(instanceState?.state, dashboardData.loading)
  const topbarStatusUsesGreenIcon = topbarStatusDotClass.includes('sd-dot-green')
  const topbarStatusClassName = `sd-topbar-status sd-topbar-status-${instanceState?.state ?? 'unknown'}`
  const shellClassName = `sd-shell${opsRailAutoCollapsed ? ' sd-shell--opsrail-auto-collapsed' : ''}`

  return (
    <div
      ref={shellRef}
      className={shellClassName}
      data-opsrail-collapsed={opsRailAutoCollapsed ? 'auto' : undefined}
    >
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
            <div className="sd-opsrail-hstat-list">
              {railResourceStats.map((stat) => (
                <div key={stat.label} className={`sd-opsrail-hstat sd-opsrail-hstat--${usageLevel(stat.value)}`}>
                  <div className="sd-opsrail-hstat-row">
                    <span className="sd-opsrail-hstat-orb" aria-hidden="true" />
                    <span className="sd-opsrail-hstat-label">{stat.label}</span>
                    <span className="sd-opsrail-hstat-value">{metricPercentText(stat.value)}</span>
                  </div>
                  <div className="sd-opsrail-hstat-bar">
                    <span
                      className="sd-opsrail-hstat-fill"
                      style={{ width: `${metricPercentWidth(stat.value)}%` }}
                    />
                  </div>
                </div>
              ))}
              <div className={`sd-opsrail-hstat sd-opsrail-hstat--${railPlayerLevel}`}>
                <div className="sd-opsrail-hstat-row">
                  <span className="sd-opsrail-hstat-orb" aria-hidden="true" />
                  <span className="sd-opsrail-hstat-label">在线玩家</span>
                  <span className="sd-opsrail-hstat-value">{railPlayerSummary}</span>
                </div>
              </div>
              <div className={`sd-opsrail-hstat sd-opsrail-hstat--${railLatencyLevel}`}>
                <div className="sd-opsrail-hstat-row">
                  <span className="sd-opsrail-hstat-orb" aria-hidden="true" />
                  <span className="sd-opsrail-hstat-label">网络延迟</span>
                  <span className="sd-opsrail-hstat-value">
                    {apiLatencyMs != null ? `${apiLatencyMs}ms` : '—'}
                  </span>
                </div>
              </div>
            </div>
            <button type="button" className="sd-opsrail-link" onClick={() => navigate('diagnostics')}>
              查看详情 →
            </button>
          </section>

          {/* 进行中：维护计划/定时备份倒计时 + 运行中任务进度 */}
          <OpsRailActiveCard jobs={jobs} jobLogsByJobId={dashboardData.jobLogsByJobId} />

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
                    <span className="sd-opsrail-job-type" title={jobDisplayName(job)}>{jobDisplayName(job)}</span>
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
