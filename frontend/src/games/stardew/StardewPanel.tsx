import { Suspense, lazy, useEffect, useRef, useState } from 'react'
import type {
  CurrentUser,
  InstanceState,
  Job,
  JobLog,
  ResourceMetricSample,
  RestartSchedule,
  SaveInfo,
} from '../../types'
import { getInstanceMetrics, getRestartSchedule } from '../../api'
import { jobDisplayName, stateLabel } from '../../core/helpers'
import { parseRoute, routeToPath } from './stardew-routes'
import type { StardewNavigateOptions, StardewRoute, StardewSaveActionRequest } from './stardew-routes'
import { useStardewDashboardData } from './useStardewDashboardData'
import { UpdateDetailsDialog } from './UpdateDetailsDialog'
import { panelUpdateSurface } from './panel-update-machine'
import './StardewPanel.css'

const InstallPage = lazy(() => import('./pages/InstallPage').then((m) => ({ default: m.InstallPage })))
const OverviewPage = lazy(() => import('./pages/OverviewPage').then((m) => ({ default: m.OverviewPage })))
const ServerControlPage = lazy(() =>
  import('./pages/ServerControlPage').then((m) => ({ default: m.ServerControlPage })),
)
const SavesPage = lazy(() => import('./pages/SavesPage').then((m) => ({ default: m.SavesPage })))
const JobsLogsPage = lazy(() => import('./pages/JobsLogsPage').then((m) => ({ default: m.JobsLogsPage })))
const PlayersPage = lazy(() => import('./pages/PlayersPage').then((m) => ({ default: m.PlayersPage })))
const ModsPage = lazy(() => import('./pages/ModsPage').then((m) => ({ default: m.ModsPage })))
const DiagnosticsPage = lazy(() => import('./pages/DiagnosticsPage').then((m) => ({ default: m.DiagnosticsPage })))
const SettingsPage = lazy(() => import('./pages/SettingsPage').then((m) => ({ default: m.SettingsPage })))

function PageLoadingFallback() {
  return (
    <div className="sd-placeholder-grid">
      <div className="sd-placeholder-card">加载中…</div>
    </div>
  )
}

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
const SHELL_DESIGN_WIDTH = 1536
const SHELL_DESIGN_HEIGHT = 1024
const SHELL_MIN_UI_SCALE = 0.72
const OPS_RAIL_COLLAPSE_MAIN_WIDTH = 400
const OPS_RAIL_EXPAND_MAIN_WIDTH = 460
const OPS_RAIL_METRICS_REFRESH_MS = 2000
const GAME_INSTALLED_STATES = new Set(['game_installed', 'save_required', 'ready_to_start', 'starting', 'running', 'stopped'])
const ACTIVE_INSTALL_JOB_STATUSES = new Set(['queued', 'running'])

function clampNumber(min: number, value: number, max: number): number {
  return Math.max(min, Math.min(max, value))
}

function shellUiScale(shellWidth: number): number {
  const viewportHeight = window.innerHeight || SHELL_DESIGN_HEIGHT
  return Math.max(
    SHELL_MIN_UI_SCALE,
    Math.min(shellWidth / SHELL_DESIGN_WIDTH, viewportHeight / SHELL_DESIGN_HEIGHT),
  )
}

function expandedMainWidthForShell(shellWidth: number): number {
  const scale = shellUiScale(shellWidth)
  const sidebarWidth = clampNumber(196, shellWidth * 0.14, 216) * scale
  const opsRailWidth = clampNumber(268, shellWidth * 0.19, 300) * scale
  return shellWidth - sidebarWidth - opsRailWidth
}

function shouldAutoCollapseOpsRail(shellWidth: number, currentlyCollapsed: boolean): boolean {
  if (shellWidth <= 720) return false
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

function parseClockMinutes(value: string): number | null {
  const match = /^(\d{1,2}):(\d{2})$/.exec(value.trim())
  if (!match) return null
  const hour = Number.parseInt(match[1], 10)
  const minute = Number.parseInt(match[2], 10)
  if (hour < 0 || hour > 23 || minute < 0 || minute > 59) return null
  return hour * 60 + minute
}

function localScheduledTime(baseMs: number, clockMinutes: number, dayOffset: number): number {
  const date = new Date(baseMs)
  date.setDate(date.getDate() + dayOffset)
  date.setHours(Math.floor(clockMinutes / 60), clockMinutes % 60, 0, 0)
  return date.getTime()
}

type RestartWindow = {
  shutdownAt: number
  startupAt: number
}

function restartWindowForDay(schedule: RestartSchedule, now: number, dayOffset: number): RestartWindow | null {
  const shutdownMinutes = parseClockMinutes(schedule.shutdownTime)
  const startupMinutes = parseClockMinutes(schedule.startupTime)
  if (shutdownMinutes == null || startupMinutes == null) return null
  const shutdownAt = localScheduledTime(now, shutdownMinutes, dayOffset)
  let startupAt = localScheduledTime(now, startupMinutes, dayOffset)
  if (startupAt <= shutdownAt) startupAt = localScheduledTime(now, startupMinutes, dayOffset + 1)
  return { shutdownAt, startupAt }
}

function sameScheduledMinute(value: string | undefined, expectedAt: number): boolean {
  if (!value) return false
  const actual = new Date(value).getTime()
  return Number.isFinite(actual) && Math.abs(actual - expectedAt) < 60_000
}

function jobIdFromScheduleMessage(message: string | undefined): string | null {
  if (!message) return null
  try {
    const parsed = JSON.parse(message) as { jobId?: unknown }
    return typeof parsed.jobId === 'string' ? parsed.jobId : null
  } catch {
    return null
  }
}

function lifecycleJobKind(job: Job, logs: JobLog[]): 'startup' | 'shutdown' | 'restart' | null {
  if (job.type !== 'stardew_lifecycle') return null
  const text = logs.map((entry) => entry.message).join('\n')
  if (text.includes('正在重启 Stardew 服务器')) return 'restart'
  if (text.includes('正在停止 Stardew 服务器') || text.includes('服务器已停止')) return 'shutdown'
  if (text.includes('正在启动 Stardew 服务器') || text.includes('服务器运行中，邀请码')) return 'startup'
  return null
}

function scheduledStartupJob(
  jobs: Job[],
  jobLogsByJobId: Record<string, JobLog[]>,
  schedule: RestartSchedule,
  window: RestartWindow,
): Job | null {
  const messageJobId =
    schedule.lastStatus === 'startup_queued' && sameScheduledMinute(schedule.lastStartupAt, window.startupAt)
      ? jobIdFromScheduleMessage(schedule.lastMessage)
      : null
  if (messageJobId) {
    const matched = jobs.find((job) => job.id === messageJobId)
    if (matched) return matched
  }
  return jobs.find((job) => {
    if (job.type !== 'stardew_lifecycle') return false
    const createdAt = new Date(job.createdAt).getTime()
    if (!Number.isFinite(createdAt) || createdAt < window.startupAt - 30_000 || createdAt > window.startupAt + 10 * 60_000) {
      return false
    }
    const kind = lifecycleJobKind(job, jobLogsByJobId[job.id] ?? [])
    return kind === 'startup' || kind === null
  }) ?? null
}

function isWindowStartupComplete(
  schedule: RestartSchedule,
  window: RestartWindow,
  jobs: Job[],
  jobLogsByJobId: Record<string, JobLog[]>,
  instanceState: InstanceState | null,
  now: number,
): boolean {
  if (now < window.startupAt) return false
  const startupJob = scheduledStartupJob(jobs, jobLogsByJobId, schedule, window)
  if (startupJob) return startupJob.status === 'succeeded'
  if (schedule.lastStatus === 'skipped_already_running' && sameScheduledMinute(schedule.lastStartupAt, window.startupAt)) {
    return true
  }
  return instanceState?.state === 'running' && now - window.startupAt > 10 * 60_000
}

function activeRestartWindow(
  schedule: RestartSchedule,
  now: number,
  jobs: Job[],
  jobLogsByJobId: Record<string, JobLog[]>,
  instanceState: InstanceState | null,
): RestartWindow | null {
  const windows = [-1, 0, 1]
    .map((offset) => restartWindowForDay(schedule, now, offset))
    .filter((window): window is RestartWindow => window !== null)
    .sort((a, b) => a.shutdownAt - b.shutdownAt)
  const latestStarted = [...windows].reverse().find((window) => now >= window.shutdownAt)
  if (
    latestStarted &&
    !isWindowStartupComplete(schedule, latestStarted, jobs, jobLogsByJobId, instanceState, now)
  ) {
    return latestStarted
  }
  return windows.find((window) => window.shutdownAt > now) ?? null
}

type MaintenanceRow = {
  key: string
  label: string
  value: string
  width: number
  level?: 'info' | 'warn'
}

function maintenanceRows(
  schedule: RestartSchedule | null,
  now: number,
  jobs: Job[],
  jobLogsByJobId: Record<string, JobLog[]>,
  instanceState: InstanceState | null,
): { rows: MaintenanceRow[]; hiddenJobIds: Set<string> } {
  const hiddenJobIds = new Set<string>()
  if (!schedule?.enabled) return { rows: [], hiddenJobIds }
  const window = activeRestartWindow(schedule, now, jobs, jobLogsByJobId, instanceState)
  if (!window) return { rows: [], hiddenJobIds }

  if (now < window.shutdownAt) {
    return {
      hiddenJobIds,
      rows: [
        {
          key: 'auto-shutdown',
          label: '自动关机',
          value: formatCountdown(window.shutdownAt - now),
          width: Math.max(0, Math.min(100, ((DAY_MS - (window.shutdownAt - now)) / DAY_MS) * 100)),
        },
        {
          key: 'auto-startup',
          label: '自动开机',
          value: formatCountdown(window.startupAt - now),
          width: Math.max(0, Math.min(100, ((DAY_MS - (window.startupAt - now)) / DAY_MS) * 100)),
        },
      ],
    }
  }

  if (now < window.startupAt) {
    if (instanceState?.state !== 'stopped') {
      for (const job of jobs) {
        if (lifecycleJobKind(job, jobLogsByJobId[job.id] ?? []) === 'shutdown') hiddenJobIds.add(job.id)
      }
      return {
        hiddenJobIds,
        rows: [{
          key: 'auto-shutdown-running',
          label: '关机中',
          value: '等待关机结束',
          width: Math.min(95, Math.max(15, ((now - window.shutdownAt) / DEFAULT_JOB_DURATION_MS) * 100)),
          level: 'warn',
        }],
      }
    }
    return {
      hiddenJobIds,
      rows: [{
        key: 'auto-startup-waiting',
        label: '自动开机',
        value: formatCountdown(window.startupAt - now),
        width: Math.max(0, Math.min(100, ((now - window.shutdownAt) / (window.startupAt - window.shutdownAt)) * 100)),
      }],
    }
  }

  const startupJob = scheduledStartupJob(jobs, jobLogsByJobId, schedule, window)
  if (startupJob) hiddenJobIds.add(startupJob.id)
  return {
    hiddenJobIds,
    rows: [{
      key: 'auto-startup-running',
      label: '开机中',
      value: '等待开机结束',
      width: startupJob ? runningJobPercent(startupJob, jobs, now, jobLogsByJobId[startupJob.id] ?? []) : 65,
      level: 'warn',
    }],
  }
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
function OpsRailActiveCard({
  jobs,
  jobLogsByJobId,
  instanceState,
}: {
  jobs: Job[]
  jobLogsByJobId: Record<string, JobLog[]>
  instanceState: InstanceState | null
}) {
  const [schedule, setSchedule] = useState<RestartSchedule | null>(null)
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

  const { rows: restartRows, hiddenJobIds } = maintenanceRows(schedule, now, jobs, jobLogsByJobId, instanceState)
  const activeJobs = jobs.filter((j) => (j.status === 'running' || j.status === 'queued') && !hiddenJobIds.has(j.id))

  return (
    <section className="sd-ops-card sd-ops-card-active sd-opsrail-section sd-opsrail-active">
      <h2 className="sd-opsrail-heading">
        <img className="sd-opsrail-title-icon" src={RIGHT_RAIL_TITLE_ICONS.active} alt="" />
        <span>进行中</span>
      </h2>
      <div className="sd-opsrail-list">
        {restartRows.length === 0 && activeJobs.length === 0 ? (
          <p className="sd-opsrail-empty">暂无进行中的任务</p>
        ) : (
          <>
            {restartRows.map((entry) => (
              <div
                key={entry.key}
                className={`sd-opsrail-hstat sd-opsrail-hstat--${entry.level ?? 'info'}`}
              >
                <div className="sd-opsrail-hstat-row">
                  <span className="sd-opsrail-hstat-orb" aria-hidden="true" />
                  <span className="sd-opsrail-hstat-label">{entry.label}</span>
                  <span className="sd-opsrail-hstat-value">{entry.value}</span>
                </div>
                <div className="sd-opsrail-hstat-bar">
                  <span className="sd-opsrail-hstat-fill" style={{ width: `${entry.width}%` }} />
                </div>
              </div>
            ))}
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

const TOPBAR_SEASON_LABEL: Record<string, string> = {
  spring: '春',
  summer: '夏',
  fall: '秋',
  autumn: '秋',
  winter: '冬',
}

const TOPBAR_YEAR_LABEL: Record<number, string> = {
  1: '一',
  2: '二',
  3: '三',
  4: '四',
  5: '五',
  6: '六',
  7: '七',
  8: '八',
  9: '九',
  10: '十',
}

function topbarSaveTimeLabel(save: SaveInfo | null): string {
  if (!save?.gameYear && !save?.gameSeason) return ''
  const year =
    typeof save.gameYear === 'number' && save.gameYear > 0
      ? `第${TOPBAR_YEAR_LABEL[save.gameYear] ?? save.gameYear}年`
      : ''
  const season = TOPBAR_SEASON_LABEL[save.gameSeason?.toLowerCase() ?? ''] ?? save.gameSeason ?? ''
  return `${year}${season}`.trim()
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
  const [installPromptPending, setInstallPromptPending] = useState(true)
  const [showMissingGameInstallPrompt, setShowMissingGameInstallPrompt] = useState(false)
  const [railMetric, setRailMetric] = useState<ResourceMetricSample | null>(null)

  useEffect(() => {
    const onPop = () => setRoute(parseRoute(window.location.pathname))
    window.addEventListener('popstate', onPop)
    return () => window.removeEventListener('popstate', onPop)
  }, [])

  useEffect(() => {
    if (!installPromptPending || dashboardData.loading || !instanceState) return
    const installJobActive = jobs.some(
      (job) => job.type === 'stardew_install' && ACTIVE_INSTALL_JOB_STATUSES.has(job.status),
    )
    const gameInstalled = GAME_INSTALLED_STATES.has(instanceState.state)
    if (gameInstalled || installJobActive || route === 'install') {
      setInstallPromptPending(false)
      return
    }
    setShowMissingGameInstallPrompt(true)
    setInstallPromptPending(false)
  }, [dashboardData.loading, installPromptPending, instanceState, jobs, route])

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

  useEffect(() => {
    let alive = true
    let timer: number | undefined

    function clearTimer() {
      if (timer != null) {
        window.clearTimeout(timer)
        timer = undefined
      }
    }

    function scheduleNext() {
	  if (!alive || document.visibilityState !== 'visible') return
      clearTimer()
      timer = window.setTimeout(() => {
        void loadMetrics()
      }, OPS_RAIL_METRICS_REFRESH_MS)
    }

    async function loadMetrics() {
	  if (document.visibilityState !== 'visible') return
      try {
        const res = await getInstanceMetrics()
        if (!alive) return
        setRailMetric(res.sample)
      } catch {
        // Keep the previous sample so the right rail does not flicker during brief Docker/API hiccups.
      } finally {
        scheduleNext()
      }
    }

	function handleVisibilityChange() {
	  if (document.visibilityState === 'visible') {
		void loadMetrics()
		return
	  }
	  clearTimer()
	}

	document.addEventListener('visibilitychange', handleVisibilityChange)
	if (document.visibilityState === 'visible') {
	  void loadMetrics()
	}
    return () => {
      alive = false
      clearTimer()
	  document.removeEventListener('visibilitychange', handleVisibilityChange)
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
    { label: 'CPU 使用率', value: null },
    { label: '内存使用率', value: null },
    { label: '磁盘使用率', value: null },
  ]
  const railMetricValues = [railMetric?.cpuPercent, railMetric?.memoryPercent, railMetric?.diskPercent]
  const liveRailResourceStats = railResourceStats.map((stat, index) => ({
    ...stat,
    value: railMetricValues[index] ?? null,
  }))
  const onlineCount = dashboardData.players?.onlineCount
  const maxPlayers = dashboardData.players?.maxPlayers
  const railPlayerSummary =
    onlineCount != null
      ? maxPlayers != null
        ? `${onlineCount}/${maxPlayers}`
        : String(onlineCount)
      : '—'
  const railPlayerLevel: HealthStatLevel = onlineCount === 0 ? 'crit' : 'ok'
  const railLatencyLevel = latencyLevel(null)
  const activeSave = activeSaveName
    ? saves?.saves.find((save) => save.isActive || save.name === activeSaveName) ?? null
    : null
  const topbarSaveName = activeSave?.farmName || activeSave?.name || activeSaveName || '选择存档'
  const topbarSaveTime = topbarSaveTimeLabel(activeSave)
  const topbarSaveTitle = topbarSaveTime ? `${topbarSaveName}：${topbarSaveTime}` : topbarSaveName
  const updateSurface = panelUpdateSurface(dashboardData.updateStatus, dashboardData.updateApply, versionInfo)
  const topbarVersion = updateSurface.topbarText
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
          aria-label={`当前存档：${topbarSaveTitle}`}
          title={`当前存档：${topbarSaveTitle}`}
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
          <span className="sd-topbar-save-copy">
            <span className="sd-topbar-save-name">{topbarSaveName}</span>
            {topbarSaveTime ? (
              <span className="sd-topbar-save-time">：{topbarSaveTime}</span>
            ) : null}
          </span>
        </button>

        <button
          type="button"
          className={`sd-topbar-version sd-topbar-version--${updateSurface.tone}`}
          onClick={dashboardData.openUpdateDialog}
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
        <div className="sd-main-scroll">
          <Suspense fallback={<PageLoadingFallback />}>{renderPage()}</Suspense>
        </div>
      </main>
      <UpdateDetailsDialog user={user} dashboardData={dashboardData} />

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
              {liveRailResourceStats.map((stat) => (
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
                  <span className="sd-opsrail-hstat-value">按需诊断</span>
                </div>
              </div>
            </div>
            <button type="button" className="sd-opsrail-link" onClick={() => navigate('diagnostics')}>
              查看详情 →
            </button>
          </section>

          {/* 进行中：维护计划/定时备份倒计时 + 运行中任务进度 */}
          <OpsRailActiveCard
            jobs={jobs}
            jobLogsByJobId={dashboardData.jobLogsByJobId}
            instanceState={instanceState}
          />

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

      {showMissingGameInstallPrompt ? (
        <div className="sd-confirm-overlay" role="presentation">
          <div className="sd-confirm-dialog sd-confirm-dialog-wide" role="dialog" aria-modal="true" aria-labelledby="sd-first-install-title">
            <h3 id="sd-first-install-title">请先安装游戏</h3>
            <p>
              当前实例还没有检测到 Stardew Valley 游戏文件。请先进入安装界面完成 Steam
              认证和游戏下载，之后再创建存档并启动服务器。
            </p>
            <div className="sd-confirm-actions">
              <button className="sd-btn-tan" type="button" onClick={() => setShowMissingGameInstallPrompt(false)}>
                稍后
              </button>
              <button
                className="sd-btn-green"
                type="button"
                onClick={() => {
                  setShowMissingGameInstallPrompt(false)
                  navigate('install')
                }}
              >
                去安装游戏
              </button>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  )
}
