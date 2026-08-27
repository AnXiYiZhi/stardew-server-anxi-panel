import { useCallback, useEffect, useMemo, useState } from 'react'
import type { CSSProperties, FormEvent } from 'react'
import * as QRCode from 'qrcode'
import type { ImageTagOption, Job, JobLog } from '../../../types'
import type { StardewPageProps } from '../stardew-routes'
import {
  ApiError,
  createJobEventSource,
  getInstallOptions,
  getJob,
  getLatestJobLogs,
  installInstance,
  submitSteamGuardInput,
  updateSteamCredentials,
} from '../../../api'
import {
  appendUniqueLog,
  errorMessage,
  formatBytes,
  isTerminalJobStatus,
} from '../../../core/helpers'
import {
  calcSteamDownloadTaskProgress,
  extractSMAPIArchiveProgress,
  extractSteamDownloadProgress,
  hasSteamSdkDownloadStarted,
} from '../install-helpers'
import { canonicalInstallJobs, canonicalInstallPageJobs, installJobForDisplay, latestInstallLogsFirst } from '../install-state'
import { classifyInstallationState } from '../installation-state'
import { useSteamAuthLogin } from '../useSteamAuthLogin'

// ── 进度工具 ──────────────────────────────────────────────────────────────────

const PULL_PROGRESS_RE = /^\[pull:progress:(\d+):(\d+)\]$/
const SMAPI_PROGRESS_RE = /^\[smapi:download:progress:/
const STEAM_QR_URL_RE = /^or open:\s*(https?:\/\/s\.team\/q\/\S+)/i
const STEAMCMD_BRACKET_PROGRESS_RE = /\[steamcmd\]\s+\[\s*\d+(?:\.\d+)?%\]\s+downloading update\s+\(/i

function extractPullProgress(logs: JobLog[]): { done: number; total: number; percent: number } | null {
  let latest: { done: number; total: number } | null = null
  for (const log of logs) {
    const m = log.message.match(PULL_PROGRESS_RE)
    if (m) latest = { done: parseInt(m[1], 10), total: parseInt(m[2], 10) }
  }
  if (!latest || latest.total === 0) return null
  return { ...latest, percent: Math.round((latest.done / latest.total) * 100) }
}

type SteamQrPayload = {
  art: string
  url: string
}

function isQrArtLine(line: string): boolean {
  const trimmed = line.trim()
  if (!trimmed) return false
  const lower = trimmed.toLowerCase()
  if (
    lower.startsWith('or open:') ||
    lower.startsWith('scan this qr') ||
    lower.startsWith('choose authentication') ||
    lower.startsWith('choice') ||
    lower.startsWith('[steamauth') ||
    lower.startsWith('[steamservice') ||
    lower.startsWith('connecting') ||
    lower.startsWith('disconnected') ||
    lower.includes('steam connection attempt failed')
  ) {
    return false
  }
  return /[█▀▄▌▐■□▓▒░]/.test(line)
}

function extractQrPayload(logs: JobLog[]): SteamQrPayload | null {
  const steamLines = logs
    .filter((l) => l.message.startsWith('[steam] '))
    .map((l) => l.message.slice('[steam] '.length))

  for (let i = steamLines.length - 1; i >= 0; i -= 1) {
    const match = steamLines[i].match(STEAM_QR_URL_RE)
    if (!match) continue

    const artLines: string[] = []
    for (let j = i - 1; j >= 0 && isQrArtLine(steamLines[j]); j -= 1) {
      artLines.unshift(steamLines[j].replace(/\s+$/g, ''))
    }
    return { art: artLines.join('\n'), url: match[1] }
  }

  return null
}

function qrCodeFontSize(text: string): number {
  const lines = text.split('\n')
  const longest = lines.reduce((max, line) => Math.max(max, line.length), 0)
  if (lines.length > 42 || longest > 92) return 9
  if (lines.length > 36 || longest > 82) return 10
  if (lines.length > 30 || longest > 72) return 11
  return 12
}

type SteamAuthLogPhase =
  | 'auth_method_required'
  | 'steam_auth_running'
  | 'steam_guard_choice_required'
  | 'steam_guard_required'
  | 'steam_guard_mobile_required'
  | 'steam_qr_required'
  | 'steamcmd_auth_running'
  | 'steamcmd_guard_choice_required'
  | 'steamcmd_guard_required'
  | 'steamcmd_guard_mobile_required'
  | 'steamcmd_downloading'
  | 'smapi_installing'

function inferLatestSteamAuthLogPhase(logs: JobLog[]): SteamAuthLogPhase | null {
  let activeMenu: 'auth' | 'guard' | 'steamcmd_guard' | null = null
  let latestPhase: SteamAuthLogPhase | null = null

  for (const log of logs) {
    const message = log.message.toLowerCase()
    const isAuthMenu = message.includes('choose authentication method') || message.includes('[2] qr code')
    const isGuardMenu = message.includes('steam guard authentication') ||
      (message.includes('[1]') && message.includes('approve in steam') && message.includes('[2]') && message.includes('enter code'))
    const isSteamCMD = message.includes('[steamcmd]')
    const isSMAPI = message.startsWith('[smapi')
    const isSteamCMDGuardMenu = isSteamCMD && message.includes('steam guard') &&
      message.includes('[1]') && message.includes('[2]') &&
      (message.includes('approve') || message.includes('code') || message.includes('email'))

    if (isAuthMenu) {
      activeMenu = 'auth'
      latestPhase = 'auth_method_required'
    }
    if (isGuardMenu) {
      activeMenu = 'guard'
      latestPhase = 'steam_guard_choice_required'
    }
    if (isSteamCMDGuardMenu) {
      activeMenu = 'steamcmd_guard'
      latestPhase = 'steamcmd_guard_choice_required'
    }

    if (isSteamCMD && (
      message.includes('steamcmd 需要重新授权；请打开 steam 手机 app') ||
      message.includes('waiting for approval') ||
      message.includes('waiting for confirmation') ||
      message.includes('please confirm the login in the steam mobile app') ||
      message.includes('approve this login') ||
      message.includes('check your steam mobile app')
    )) {
      activeMenu = null
      latestPhase = 'steamcmd_guard_mobile_required'
    } else if (message.includes('waiting for approval on your steam mobile app') ||
      message.includes('open steam app on your phone and approve')) {
      activeMenu = null
      latestPhase = 'steam_guard_mobile_required'
    }
    if (isSteamCMD && (
      message.includes('steamcmd 需要重新授权；请输入') ||
      message.includes('steam guard code') ||
      message.includes('code sent to') ||
      message.includes('enter verification code') ||
      message.includes('this computer has not been authenticated') ||
      message.includes('please check your email') ||
      message.includes('enter the steam guard') ||
      message.includes('code from that message') ||
      message.includes('set_steam_guard_code')
    )) {
      activeMenu = null
      latestPhase = 'steamcmd_guard_required'
    } else if (message.includes('enter steam guard code')) {
      activeMenu = null
      latestPhase = 'steam_guard_required'
    }
    if (isSteamCMD && (
      message.includes('steamcmd 已授权，正在兜底下载') ||
      message.includes('steamcmd 兜底下载完成') ||
      message.includes("success! app '413150' fully installed") ||
      message.includes('update state') ||
      message.includes('downloading, progress') ||
      STEAMCMD_BRACKET_PROGRESS_RE.test(message)
    )) {
      activeMenu = null
      latestPhase = 'steamcmd_downloading'
    }
    if (isSMAPI) {
      activeMenu = null
      latestPhase = 'smapi_installing'
    }
    if (isSteamCMD && message.includes('正在使用已保存账号密码登录 steamcmd')) {
      latestPhase = 'steamcmd_auth_running'
    }
    if (message.includes('or open:') && message.includes('s.team/q/')) {
      activeMenu = null
      latestPhase = 'steam_qr_required'
    }
    if (message.includes('已选择扫码登录')) {
      activeMenu = null
      latestPhase = 'steam_qr_required'
    }

    const choice = message.match(/choice\s*(?:\[[^\]]+\])?\s*:\s*([12])/i)?.[1]
    if (choice && activeMenu === 'guard') {
      activeMenu = null
      latestPhase = choice === '1' ? 'steam_guard_mobile_required' : 'steam_guard_required'
    } else if (choice && activeMenu === 'steamcmd_guard') {
      activeMenu = null
      latestPhase = choice === '1' ? 'steamcmd_guard_mobile_required' : 'steamcmd_guard_required'
    } else if (choice && activeMenu === 'auth') {
      activeMenu = null
      latestPhase = choice === '2' ? 'steam_qr_required' : 'steam_auth_running'
    }
  }

  return latestPhase
}

function logsShowSteamGameDownloadStarted(logs: JobLog[]): boolean {
  return logs.some((log) => {
    const message = log.message.toLowerCase()
    return message.includes('[steam]') && (
      message.includes('downloading app 413150') ||
      message.includes('target directory: /data/game') ||
      message.includes('manifest contains') ||
      message.includes('progress:')
    )
  })
}

// ── 阶段→步骤状态 ──────────────────────────────────────────────────────────────

function logsShowSteamAuthSucceeded(logs: JobLog[]): boolean {
  return logs.some((log) => {
    const message = log.message.toLowerCase()
    return message.includes('[steam]') && (
      message.includes('logged in as') ||
      message.includes('token expires:') ||
      message.includes('game license verified') ||
      message.includes('got depot decryption key') ||
      message.includes('downloading app 413150') ||
      message.includes('target directory: /data/game')
    )
  })
}

type StepStatus = 'pending' | 'active' | 'done' | 'error'

const BASE_AUTH_FAILED_PHASES = [
  'credentials_required', 'install_interrupted', 'steamcmd_failed', 'steamcmd_image_pull_failed',
]

const STEAM_INVITE_AUTH_FAILED_PHASES = [
  'steam_auth_failed', 'qr_auth_failed', 'steam_auth_console_failed', 'steam_auth_connection_failed',
]

const ALL_AUTH_FAILED_PHASES = [...BASE_AUTH_FAILED_PHASES, ...STEAM_INVITE_AUTH_FAILED_PHASES]

function calcStepStatuses(
  installed: boolean,
  phase: string,
  authFailed: boolean,
  isInstalling: boolean,
): [StepStatus, StepStatus, StepStatus, StepStatus, StepStatus] {
  // Phases where Steam authentication is actively happening (not yet done)
  const authPhases = [
    'steam_auth_running', 'auth_method_required', 'steam_guard_choice_required',
    'steam_guard_required', 'steam_guard_mobile_required', 'steam_qr_required',
    'steam_auth_done', 'steamcmd_auth_running', 'steamcmd_guard_choice_required',
    'steamcmd_guard_required', 'steamcmd_guard_mobile_required',
  ]
  // Phases where auth is already complete and game download is in progress
  const downloadPhases = ['game_downloading', 'steam_sdk_downloading', 'steamcmd_downloading', 'smapi_installing']

  const isAuthPhase = authPhases.includes(phase)
  const isDownloadPhase = downloadPhases.includes(phase)
  const started = isInstalling || installed || authFailed
    || phase === 'pull_failed' || phase === 'install_timeout'

  const s1: StepStatus = started ? 'done' : 'pending'
  const s2: StepStatus =
    installed || isAuthPhase || isDownloadPhase || authFailed || phase === 'install_timeout' ? 'done'
    : phase === 'pull_failed' ? 'error'
    : isInstalling ? 'active'
    : 'pending'
  const s3: StepStatus =
    installed || isDownloadPhase ? 'done'        // auth done, now downloading
    : authFailed || phase === 'install_timeout' ? 'error'
    : isAuthPhase ? 'active'
    : 'pending'
  const s4: StepStatus =
    installed ? 'done'
    : isDownloadPhase ? 'active'
    : phase === 'download_failed' || phase === 'post_auth_failed' || phase === 'smapi_install_failed' || phase === 'smapi_bundled_sync_failed' ? 'error'
    : 'pending'
  const s5: StepStatus = installed ? 'done' : 'pending'
  return [s1, s2, s3, s4, s5]
}

function phaseLabel(phase: string, isInstalling: boolean, authFailed: boolean, installed: boolean): string {
  if (phase === 'smapi_install_failed') return 'SMAPI 运行环境安装失败，请检查任务日志后重试'
  if (phase === 'smapi_bundled_sync_failed') return 'SMAPI 内置支持 Mod 同步失败，请检查任务日志后重试'
  if (installed) return '安装完成'
  if (phase === 'download_failed') return '游戏文件下载失败，请检查网络/磁盘后重试'
  if (phase === 'post_auth_failed') return 'SteamCMD 已授权，后续安装步骤失败，请使用已保存凭据重试'
  if (phase === 'steamcmd_failed') return 'SteamCMD 安装或修复失败，请查看任务日志后重试'
  if (phase === 'steamcmd_image_pull_failed') return 'SteamCMD 工具镜像拉取失败，请检查 Docker 网络'
  if (phase === 'qr_auth_failed') return 'Steam 邀请码二维码授权失败，可改用账号密码或 Steam Guard 重试'
  if (phase === 'credentials_required' && authFailed) return 'SteamCMD 登录失败，账号或密码错误'
  if (phase === 'install_interrupted') return '安装任务已中断，请重新发起安装'
  if (STEAM_INVITE_AUTH_FAILED_PHASES.includes(phase)) return 'Steam 邀请码授权失败，基础安装与局域网/IP 直连不受影响'
  if (authFailed) return 'SteamCMD 授权失败，请查看任务日志'
  if (phase === 'pull_failed') return '镜像拉取失败，请检查网络后重试'
  if (phase === 'install_timeout') return '安装任务超时，请重试安装'
  if (phase === 'steam_auth_connection_failed') return 'Steam 邀请码授权连接超时，可重试；局域网/IP 直连仍可使用'
  if (phase === 'steam_auth_retrying') return 'Steam 邀请码授权连接较慢，正在自动重试...'
  if (!isInstalling) return ''
  const labels: Record<string, string> = {
    smapi_installing: '游戏文件和 Steam SDK 已完成，正在安装 SMAPI 运行环境...',
    junimo_scaffolded: '目录已准备，正在拉取镜像...',
    pull_running: '正在拉取 JunimoServer 镜像...',
    steam_auth_running: '正在进行 Steam 邀请码授权（不会重新下载游戏）...',
    auth_method_required: '等待选择 Steam 登录方式...',
    steam_guard_choice_required: '等待选择 Steam Guard 验证方式...',
    steam_guard_required: '等待 Steam Guard 验证码...',
    steam_guard_mobile_required: '请在手机 App 批准登录...',
    steam_qr_required: '请扫描 Steam 二维码...',
    game_downloading: '正在下载游戏文件（约 10–30 分钟）...',
    steam_sdk_downloading: '游戏文件已下载，正在下载 Steam SDK 运行文件...',
    steamcmd_image_pulling: '正在拉取 SteamCMD 安装工具镜像...',
    steamcmd_auth_running: 'SteamCMD 正在复用已保存授权登录...',
    steamcmd_guard_choice_required: 'SteamCMD 需要重新授权，请选择验证方式...',
    steamcmd_guard_required: 'SteamCMD 需要 App 或邮箱验证码...',
    steamcmd_guard_mobile_required: '请在 Steam 手机 App 批准 SteamCMD 登录...',
    steamcmd_downloading: 'SteamCMD 正在下载并校验游戏本体与 Steamworks SDK...',
    steam_auth_done: 'Steam 邀请码授权成功。',
  }
  return labels[phase] ?? '正在准备安装环境...'
}

const STEP_ICON: Record<StepStatus, string> = {
  done: '✓', error: '✗', active: '↻', pending: '○',
}
const STEPS = ['准备环境', '拉取服务镜像', 'SteamCMD 授权', '下载与安装', '完成'] as const
const STEP_ICON_SRC = [
  '/assets/stardew/ui/install/icon_install_step_seed_image2_regen.png',
  '/assets/stardew/ui/install/icon_install_step_box_image2.png',
  '/assets/stardew/ui/install/icon_install_step_steam_image2_regen.png',
  '/assets/stardew/ui/install/icon_install_step_download_image2_regen.png',
  '/assets/stardew/ui/install/icon_install_step_star_image2.png',
] as const
const STEAM_STEP_ICON_SRC = STEP_ICON_SRC[2]

// ── 组件 ──────────────────────────────────────────────────────────────────────

export function InstallPage({ user, instanceState, dashboardData, onNavigate, requestedInstallJobId }: StardewPageProps) {
  const phase = instanceState?.driverPhase ?? ''
  const stateMessage = instanceState?.stateMessage ?? ''

  const isAdmin = user.role === 'admin'

  // ── 镜像选项 ──────────────────────────────────────────────────────────────────
  const [imageTagOptions, setImageTagOptions] = useState<ImageTagOption[]>([])
  const [optionsLoading, setOptionsLoading] = useState(true)
  const [imageTag, setImageTag] = useState('')

  useEffect(() => {
    setOptionsLoading(true)
    getInstallOptions()
      .then((res) => {
        setImageTagOptions(res.imageTagOptions)
        setImageTag((prev) => {
          if (prev) return prev
          const rec = res.imageTagOptions.find((o) => o.recommended) ?? res.imageTagOptions[0]
          return rec?.tag ?? ''
        })
      })
      .catch(() => { /* 静默失败，镜像列表为空时不显示选择 */ })
      .finally(() => setOptionsLoading(false))
  }, [])

  // ── 安装 Job ──────────────────────────────────────────────────────────────────
  const [installJobId, setInstallJobId] = useState<string | null>(() => requestedInstallJobId ?? null)
  const [installJob, setInstallJob] = useState<Job | null>(null)
  const [logs, setLogs] = useState<JobLog[]>([])
  const [sseError, setSseError] = useState('')

  // Phases that indicate an install is actively running, even before installJob loads from async effect
  const BASE_INSTALLING_PHASES = [
    'pull_running', 'game_downloading', 'steam_sdk_downloading', 'steamcmd_image_pulling',
    'steamcmd_auth_running', 'steamcmd_guard_choice_required', 'steamcmd_guard_required',
    'steamcmd_guard_mobile_required', 'steamcmd_downloading', 'smapi_installing',
  ]
  const installJobs = canonicalInstallJobs(dashboardData.jobs, installJob, installJobId)
  const installPageJobs = canonicalInstallPageJobs(dashboardData.jobs, installJob, installJobId)
  const activeSteamTaskJob = installPageJobs.active
  const latestInstallJob = installJobId ? installJobs.selected : installJobForDisplay(installJobs)
  const latestSteamTaskJob = installJobId ? installPageJobs.selected : installJobForDisplay(installPageJobs)
  const displayedTaskIsActive = latestSteamTaskJob !== null && !isTerminalJobStatus(latestSteamTaskJob.status)
  const hasActiveInstallJob = displayedTaskIsActive && latestSteamTaskJob.type === 'stardew_install'
  const hasActiveSteamAuthJob = displayedTaskIsActive && latestSteamTaskJob.type === 'stardew_steam_auth'
  const selectedTaskIsSteamAuth = latestSteamTaskJob?.type === 'stardew_steam_auth'
  const installation = classifyInstallationState(instanceState, hasActiveInstallJob)
  const isInstalled = installation.isInstalled
  const staleInstallingPhase = !isInstalled
    && BASE_INSTALLING_PHASES.includes(phase)
    && !hasActiveInstallJob
    && (latestInstallJob === null || isTerminalJobStatus(latestInstallJob.status))
  const basePhase = staleInstallingPhase ? 'install_interrupted' : phase
  const authSucceededInLogs = !selectedTaskIsSteamAuth && logsShowSteamAuthSucceeded(logs)
  const canDirectRetry = installation.kind === 'installed'
    || installation.kind === 'repair_required'
    || staleInstallingPhase
    || authSucceededInLogs
    || ['pull_failed', 'install_timeout', 'install_interrupted', 'download_failed', 'post_auth_failed', 'smapi_bundled_sync_failed'].includes(basePhase)
  const isInstalling = hasActiveInstallJob
    || (!staleInstallingPhase && !isInstalled && BASE_INSTALLING_PHASES.includes(phase))

  // A job ID returned by the authorization POST is authoritative. Keep it in
  // the URL so cross-page retry buttons and stale dashboard snapshots cannot
  // switch the log window back to an older task.
  useEffect(() => {
    if (!requestedInstallJobId || requestedInstallJobId === installJobId) return
    setInstallJobId(requestedInstallJobId)
    setInstallJob(null)
    setLogs([])
    setSseError('')
  }, [installJobId, requestedInstallJobId])

  // 当 dashboardData.jobs 变化时，自动拾取活跃的安装任务；显式 URL 任务优先。
  useEffect(() => {
    if (requestedInstallJobId) return
    if (activeSteamTaskJob && activeSteamTaskJob.id !== installJobId) {
      setInstallJobId(activeSteamTaskJob.id)
      setInstallJob(null)
      setLogs([])
      setSseError('')
      return
    }
    if (installJobId) return
    const latest = dashboardData.jobs.find(
      (job) => job.type === 'stardew_install' || job.type === 'stardew_steam_auth',
    )
    if (latest) {
      setInstallJobId(latest.id)
      setInstallJob(null)
      setLogs([])
      setSseError('')
    }
  }, [activeSteamTaskJob, dashboardData.jobs, installJobId, requestedInstallJobId])

  // 当 installJobId 变化时加载详情 + 日志，并连接 SSE
  useEffect(() => {
    if (!installJobId) return
    let cancelled = false
    let es: EventSource | null = null

    const load = async () => {
      try {
        const jobRes = await getJob(installJobId)
        const logsRes = await getLatestJobLogs(installJobId, 1000)
        if (cancelled) return
        setInstallJob(jobRes.job)
        setLogs(logsRes.logs)

        if (isTerminalJobStatus(jobRes.job.status)) return

        const lastSeq = logsRes.logs.length > 0 ? logsRes.logs[logsRes.logs.length - 1].sequence : 0
        const currentJobId = installJobId
        es = createJobEventSource(currentJobId, lastSeq)

        es.addEventListener('log', (ev) => {
          if (cancelled) { es?.close(); return }
          try {
            const entry = JSON.parse((ev as MessageEvent<string>).data) as JobLog
            setLogs((prev) => appendUniqueLog(prev, { ...entry, jobId: currentJobId }))
          } catch { /* 忽略格式错误 */ }
        })

        es.addEventListener('finished', () => {
          es?.close()
          if (cancelled) return
          void getJob(currentJobId).then((r) => {
            if (!cancelled) setInstallJob(r.job)
          })
          dashboardData.refreshJobs()
          dashboardData.refreshInstanceState()
        })

        es.onerror = () => {
          if (!cancelled) setSseError('实时日志连接已断开，可手动刷新查看最新日志。')
          es?.close()
        }
      } catch {
        if (!cancelled) setInstallJob(null)
      }
    }

    void load()
    return () => {
      cancelled = true
      es?.close()
    }
  }, [installJobId]) // dashboardData.refresh* 是稳定引用，intentionally omitted

  // ── 表单 ──────────────────────────────────────────────────────────────────────
  const [showForm, setShowForm] = useState(false)
  const [editingSteamCredentials, setEditingSteamCredentials] = useState(false)
  const [steamUsername, setSteamUsername] = useState('')
  const [steamPassword, setSteamPassword] = useState('')
  const [vncPassword, setVncPassword] = useState('')
  const [showSteamPwd, setShowSteamPwd] = useState(false)
  const [showVncPwd, setShowVncPwd] = useState(false)
  const [installBusy, setInstallBusy] = useState(false)
  const [installError, setInstallError] = useState('')
  const steamAuth = useSteamAuthLogin({
    instanceState: dashboardData.instanceState,
    onNavigate,
    onStarted: (jobId) => {
      setInstallJobId(jobId)
      setInstallJob(null)
      setLogs([])
      setSseError('')
      void dashboardData.refreshInstanceState()
      void dashboardData.refreshJobs()
    },
  })

  const handleInstallSubmit = useCallback(async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    if (!isAdmin) return
    setInstallError('')
    setInstallBusy(true)
    try {
      const body = canDirectRetry
        ? { reuseCredentials: true, imageTag }
        : { steamUsername, steamPassword, vncPassword, imageTag }
      const res = await installInstance(body)
      onNavigate('install', { installJobId: res.jobId })
      setInstallJobId(res.jobId)
      setLogs([])
      setInstallJob(null)
      setSseError('')
      setShowForm(false)
      setEditingSteamCredentials(false)
      dashboardData.refreshJobs()
      dashboardData.refreshInstanceState()
    } catch (err) {
      if (err instanceof ApiError && err.code === 'install_in_progress') {
        const jobId = typeof err.details === 'object' && err.details !== null && 'jobId' in err.details
          ? String((err.details as { jobId?: unknown }).jobId ?? '')
          : ''
        if (jobId) {
          onNavigate('install', { installJobId: jobId })
          if (jobId !== installJobId) {
            setInstallJobId(jobId)
            setInstallJob(null)
            setLogs([])
            setSseError('')
          }
          setShowForm(false)
          setEditingSteamCredentials(false)
          dashboardData.refreshJobs()
          dashboardData.refreshInstanceState()
          return
        }
      }
      setInstallError(errorMessage(err))
    } finally {
      setInstallBusy(false)
    }
  }, [isAdmin, canDirectRetry, imageTag, steamUsername, steamPassword, vncPassword, dashboardData, installJobId, onNavigate])

  const handleSteamCredentialsSubmit = useCallback(async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    if (!isAdmin) return
    setInstallError('')
    setInstallBusy(true)
    try {
      await updateSteamCredentials({ steamUsername, steamPassword })
      setSteamPassword('')
      setShowSteamPwd(false)
      setShowForm(false)
      setEditingSteamCredentials(false)
      await dashboardData.refreshInstanceState()
    } catch (err) {
      setInstallError(errorMessage(err))
    } finally {
      setInstallBusy(false)
    }
  }, [isAdmin, steamUsername, steamPassword, dashboardData])

  // ── Steam Guard ───────────────────────────────────────────────────────────────
  const [guardInput, setGuardInput] = useState('')
  const [guardBusy, setGuardBusy] = useState(false)
  const [guardError, setGuardError] = useState('')
  const [guardSubmittedKind, setGuardSubmittedKind] = useState<'steam' | 'steamcmd' | null>(null)
  const [optimisticPhase, setOptimisticPhase] = useState<string | null>(null)

  const handleGuardSubmit = useCallback(async (e: FormEvent<HTMLFormElement>, kind: 'steam' | 'steamcmd') => {
    e.preventDefault()
    if (!installJobId) return
    setGuardError('')
    setGuardBusy(true)
    try {
      await submitSteamGuardInput(installJobId, guardInput)
      setGuardInput('')
      setGuardSubmittedKind(kind)
      dashboardData.refreshInstanceState()
    } catch (err) {
      setGuardSubmittedKind(null)
      setGuardError(errorMessage(err))
    } finally {
      setGuardBusy(false)
    }
  }, [installJobId, guardInput, dashboardData])

  const handleAuthMethodSelect = useCallback(async (choice: string) => {
    if (!installJobId) return
    let nextOptimisticPhase: string | null = null
    if (phase === 'auth_method_required') {
      nextOptimisticPhase = choice === '2' ? 'steam_qr_required' : 'steam_auth_running'
    } else if (phase === 'steam_guard_choice_required') {
      nextOptimisticPhase = choice === '1' ? 'steam_guard_mobile_required' : 'steam_guard_required'
    } else if (phase === 'steamcmd_guard_choice_required') {
      nextOptimisticPhase = choice === '1' ? 'steamcmd_guard_mobile_required' : 'steamcmd_guard_required'
    }
    if (nextOptimisticPhase) setOptimisticPhase(nextOptimisticPhase)
    setGuardBusy(true)
    setGuardError('')
    try {
      await submitSteamGuardInput(installJobId, choice)
      setGuardSubmittedKind(null)
      dashboardData.refreshInstanceState()
    } catch (err) {
      setOptimisticPhase(null)
      setGuardError(errorMessage(err))
    } finally {
      setGuardBusy(false)
    }
  }, [installJobId, phase, dashboardData])

  // ── QR 弹窗 ───────────────────────────────────────────────────────────────────
  const [showQrModal, setShowQrModal] = useState(false)
  const [qrImageSrc, setQrImageSrc] = useState('')
  const [qrImageError, setQrImageError] = useState('')

  // ── 计算值 ───────────────────────────────────────────────────────────────────
  const selectedSteamTaskLogs = latestSteamTaskJob?.id === installJobId ? logs : []
  const pullProgress = extractPullProgress(selectedSteamTaskLogs)
  const installJobType = latestSteamTaskJob?.type ?? (installJobId ? 'stardew_install' : undefined)
  const steamGameProgress = extractSteamDownloadProgress(selectedSteamTaskLogs, installJobType, 'game')
  const steamSdkProgress = extractSteamDownloadProgress(selectedSteamTaskLogs, installJobType, 'sdk')
  const steamCMDClientProgress = extractSteamDownloadProgress(selectedSteamTaskLogs, installJobType, 'steamcmd_update')
  const smapiArchiveProgress = extractSMAPIArchiveProgress(selectedSteamTaskLogs, installJobType)
  const logAuthPhase = inferLatestSteamAuthLogPhase(selectedSteamTaskLogs)
  const logDownloadPhase = logAuthPhase === 'smapi_installing'
    ? 'smapi_installing'
    : logAuthPhase === 'steamcmd_downloading'
      ? 'steamcmd_downloading'
    : logAuthPhase?.startsWith('steamcmd_guard')
      ? null
        : hasSteamSdkDownloadStarted(selectedSteamTaskLogs, installJobType) || steamSdkProgress
          ? 'steam_sdk_downloading'
        : steamGameProgress || logsShowSteamGameDownloadStarted(selectedSteamTaskLogs)
          ? 'game_downloading'
          : null
  const qrPayload = extractQrPayload(selectedSteamTaskLogs)
  const qrUrl = qrPayload?.url ?? ''
  const qrText = qrPayload?.art ?? ''
  const basePhaseIsFailure = (selectedTaskIsSteamAuth
    ? STEAM_INVITE_AUTH_FAILED_PHASES
    : BASE_AUTH_FAILED_PHASES).includes(basePhase)
    || basePhase === 'smapi_install_failed'
    || basePhase === 'smapi_bundled_sync_failed'
  const selectedTaskFailurePhase = latestSteamTaskJob?.status === 'failed'
    ? selectedTaskIsSteamAuth
      ? STEAM_INVITE_AUTH_FAILED_PHASES.includes(basePhase) ? basePhase : 'steam_auth_failed'
      : basePhaseIsFailure ? basePhase : 'install_interrupted'
    : null
  const logQrPhaseIsCurrent = logAuthPhase === 'steam_qr_required' &&
    optimisticPhase !== 'steam_guard_required' &&
    optimisticPhase !== 'steam_guard_mobile_required' &&
    optimisticPhase !== 'steam_guard_choice_required' &&
    optimisticPhase !== 'steamcmd_guard_required' &&
    optimisticPhase !== 'steamcmd_guard_mobile_required' &&
    optimisticPhase !== 'steamcmd_guard_choice_required' &&
    basePhase !== 'steam_guard_required' &&
    basePhase !== 'steam_guard_choice_required' &&
    basePhase !== 'steamcmd_guard_required' &&
    basePhase !== 'steamcmd_guard_choice_required'
  const effectivePhase = staleInstallingPhase
    ? 'install_interrupted'
    : selectedTaskFailurePhase
      ? selectedTaskFailurePhase
    : basePhaseIsFailure
      ? basePhase
      : logDownloadPhase
        ? logDownloadPhase
        : logAuthPhase && logAuthPhase !== 'steam_qr_required'
          ? logAuthPhase
          : optimisticPhase
            ? optimisticPhase
            : logQrPhaseIsCurrent
              ? 'steam_qr_required'
              : basePhase
  const authFailed = (selectedTaskIsSteamAuth
    ? STEAM_INVITE_AUTH_FAILED_PHASES
    : BASE_AUTH_FAILED_PHASES).includes(effectivePhase)
  const isQrAuthError = effectivePhase === 'qr_auth_failed'
  const postAuthRecoverable = canDirectRetry && (
    authSucceededInLogs ||
    effectivePhase === 'download_failed' ||
    effectivePhase === 'post_auth_failed' ||
    effectivePhase === 'smapi_install_failed' ||
    effectivePhase === 'smapi_bundled_sync_failed'
  )
  const steamCMDRecoverable = canDirectRetry && (
    effectivePhase === 'steamcmd_failed' ||
    effectivePhase === 'steamcmd_image_pull_failed'
  )
  const needsAuthMethodChoice = effectivePhase === 'auth_method_required'
  const needsGuardChoice = effectivePhase === 'steam_guard_choice_required'
  const needsGuard = effectivePhase === 'steam_guard_required' || effectivePhase === 'steam_guard_mobile_required'
  const needsSteamCMDGuardChoice = effectivePhase === 'steamcmd_guard_choice_required'
  const needsSteamCMDGuard = effectivePhase === 'steamcmd_guard_required' || effectivePhase === 'steamcmd_guard_mobile_required'
  const needsQrCode = effectivePhase === 'steam_qr_required'
  const needsInstallRepair = installation.kind === 'repair_required'
  const needsInstallationDiagnosis = installation.kind === 'runtime_error' || installation.kind === 'unknown'
  const installWorkflowFailed = installation.kind === 'install_failed'
  const showPrimaryInstallAction = installation.kind === 'not_installed'
    || installWorkflowFailed
    || needsInstallRepair
  const installationWorkflowComplete = isInstalled && !isInstalling
  const steamInviteAuthorizationCleanupPending = instanceState?.steamInviteEnabled === true
    && instanceState.steamInviteAuthState === 'cleanup_pending'
  const steamInviteAuthorizationReady = instanceState?.steamInviteEnabled === true
    && instanceState.steamInviteAuthState === 'ready'
    && instanceState.steamAuthLoggedIn === true
  const stepStatuses = calcStepStatuses(installationWorkflowComplete, effectivePhase, authFailed, isInstalling)
  const showProgress = isInstalling || isInstalled || authFailed
    || ['pull_failed', 'install_timeout', 'download_failed', 'post_auth_failed', 'install_interrupted', 'steamcmd_failed', 'steamcmd_image_pull_failed', 'smapi_install_failed', 'smapi_bundled_sync_failed'].includes(effectivePhase)
  const progressLabel = phaseLabel(
    effectivePhase,
    isInstalling || hasActiveSteamAuthJob,
    authFailed,
    installationWorkflowComplete && !hasActiveSteamAuthJob && !selectedTaskIsSteamAuth,
  )
  const selectedOption = imageTagOptions.find((o) => o.tag === imageTag)
  const steamDownloadProgress = effectivePhase === 'steam_sdk_downloading'
    ? steamSdkProgress
      : effectivePhase === 'game_downloading'
      ? steamGameProgress
      : effectivePhase === 'steamcmd_downloading'
        ? steamSdkProgress ?? steamGameProgress ?? steamCMDClientProgress
        : null
  const isSteamCMDClientUpdating = effectivePhase === 'steamcmd_downloading' &&
    !!steamCMDClientProgress &&
    !steamGameProgress &&
    !steamSdkProgress
  const steamDownloadTaskProgress = calcSteamDownloadTaskProgress(
    effectivePhase,
    steamGameProgress,
    steamSdkProgress,
    steamCMDClientProgress,
    smapiArchiveProgress,
  )

  const displayableLogs = useMemo(
    () => logs.filter((log) => !PULL_PROGRESS_RE.test(log.message) && !SMAPI_PROGRESS_RE.test(log.message)),
    [logs],
  )
  const visibleLogs = useMemo(() => latestInstallLogsFirst(displayableLogs), [displayableLogs])

  useEffect(() => {
    if (!optimisticPhase) return
    if (phase === optimisticPhase || ALL_AUTH_FAILED_PHASES.includes(phase)) {
      setOptimisticPhase(null)
    }
  }, [phase, optimisticPhase])

  useEffect(() => {
    if (!guardSubmittedKind) return
    const stillWaitingForSteamCode = guardSubmittedKind === 'steam' &&
      (effectivePhase === 'steam_guard_required' || effectivePhase === 'steam_guard_mobile_required')
    const stillWaitingForSteamCMDCode = guardSubmittedKind === 'steamcmd' &&
      (effectivePhase === 'steamcmd_guard_required' || effectivePhase === 'steamcmd_guard_mobile_required')
    if (!stillWaitingForSteamCode && !stillWaitingForSteamCMDCode) {
      setGuardSubmittedKind(null)
    }
  }, [effectivePhase, guardSubmittedKind])

  useEffect(() => {
    let canceled = false
    setQrImageSrc('')
    setQrImageError('')
    if (!qrUrl) return undefined

    void QRCode.toDataURL(qrUrl, {
      errorCorrectionLevel: 'M',
      margin: 4,
      scale: 10,
      width: 320,
      color: {
        dark: '#17110a',
        light: '#fff7df',
      },
    })
      .then((dataUrl) => {
        if (!canceled) setQrImageSrc(dataUrl)
      })
      .catch(() => {
        if (!canceled) setQrImageError('二维码图片生成失败，请使用下方链接在手机上打开。')
      })

    return () => {
      canceled = true
    }
  }, [qrUrl])

  const finishedStepCount = stepStatuses.filter((status) => status === 'done').length
  const hasActiveStep = stepStatuses.some((status) => status === 'active')
  const downloadOverallProgress = steamDownloadTaskProgress
    ? Math.min(96, Math.round(60 + steamDownloadTaskProgress.percent * 0.36))
    : null
  const pullOverallProgress = pullProgress && effectivePhase === 'pull_running'
    ? Math.min(52, Math.round(18 + pullProgress.percent * 0.34))
    : pullProgress && effectivePhase === 'steamcmd_image_pulling'
      ? Math.min(74, Math.round(58 + pullProgress.percent * 0.16))
      : null
  const overallProgress = installationWorkflowComplete
    ? 100
    : downloadOverallProgress !== null
      ? downloadOverallProgress
    : pullOverallProgress !== null
      ? pullOverallProgress
    : showProgress
      ? Math.min(96, (finishedStepCount * 20) + (hasActiveStep ? 8 : 0))
      : 0
  const activityPanelTitle = selectedTaskIsSteamAuth
    ? 'Steam 邀请码授权'
    : effectivePhase === 'smapi_installing'
    ? 'SMAPI 安装'
    : effectivePhase === 'pull_running' || effectivePhase === 'steamcmd_image_pulling'
      ? '镜像下载'
      : effectivePhase === 'game_downloading' || effectivePhase === 'steam_sdk_downloading' || effectivePhase === 'steamcmd_downloading'
        ? '下载任务'
        : 'SteamCMD 授权'
  const activityPanelUsesDownloadIcon = activityPanelTitle !== 'Steam 邀请码授权'

  return (
    <div className="sd-page sd-install-page">
      {/* ── 页面头部 + 状态横幅 ───────────────────────────────────────────── */}
      <section className="sd-install-hero" aria-labelledby="sd-install-title">
        <div className="sd-install-title-strip sd-page-header">
          <img
            className="sd-page-icon"
            src="/assets/stardew/ui/icons/icon_nav_install_package_image2.png"
            alt=""
          />
          <h2 id="sd-install-title" className="sd-page-title">首次安装向导</h2>
        </div>

        <div className="sd-install-status-banner">
          <div className="sd-install-seed-art" aria-hidden="true">
            <img
              className="sd-install-seed-img"
              src="/assets/stardew/ui/install/icon_install_status_seed_image2.png"
              alt=""
            />
          </div>
          <div className="sd-state-card">
            <div className="sd-state-row">
              <span className="sd-state-label">安装状态</span>
              {isInstalling ? (
                <><span className="sd-dot sd-dot-yellow" aria-hidden="true" /><span className="sd-state-value">安装中…</span></>
              ) : installation.kind === 'installed' ? (
                <><span className="sd-dot sd-dot-green" aria-hidden="true" /><span className="sd-state-value">已安装</span></>
              ) : effectivePhase === 'steamcmd_failed' || effectivePhase === 'steamcmd_image_pull_failed' ? (
                <><span className="sd-dot sd-dot-red" aria-hidden="true" /><span className="sd-state-value">SteamCMD 失败</span></>
              ) : postAuthRecoverable ? (
                <><span className="sd-dot sd-dot-red" aria-hidden="true" /><span className="sd-state-value">下载失败</span></>
              ) : authFailed ? (
                <><span className="sd-dot sd-dot-red" aria-hidden="true" /><span className="sd-state-value">认证失败</span></>
              ) : needsInstallRepair ? (
                <><span className="sd-dot sd-dot-red" aria-hidden="true" /><span className="sd-state-value">需要修复</span></>
              ) : installation.kind === 'runtime_error' ? (
                <><span className="sd-dot sd-dot-red" aria-hidden="true" /><span className="sd-state-value">运行异常</span></>
              ) : installWorkflowFailed ? (
                <><span className="sd-dot sd-dot-red" aria-hidden="true" /><span className="sd-state-value">安装未完成</span></>
              ) : installation.kind === 'unknown' ? (
                <><span className="sd-dot sd-dot-yellow" aria-hidden="true" /><span className="sd-state-value">状态待诊断</span></>
              ) : (
                <><span className="sd-dot sd-dot-gray" aria-hidden="true" /><span className="sd-state-value">未安装</span></>
              )}
            </div>
            <div className="sd-state-row">
              <span className="sd-state-label">当前阶段</span>
              {effectivePhase && effectivePhase !== 'empty' ? (
                <span className="sd-install-phase-tag">{effectivePhase}</span>
              ) : (
                <span className="sd-install-state-msg">等待开始</span>
              )}
            </div>
            <div className="sd-state-row">
              <span className="sd-state-label">状态说明</span>
              <span className="sd-install-state-msg">
                {stateMessage || (installation.kind === 'installed'
                  ? 'Stardew Valley Dedicated Server 已成功安装并可运行！'
                  : needsInstallRepair
                    ? '安装文件或运行栈不完整，请执行校验与修复。'
                    : installation.kind === 'runtime_error'
                      ? '服务器运行异常，但当前没有证据表明游戏未安装；请先查看诊断。'
                      : installWorkflowFailed
                        ? '上次安装流程未完成，请根据当前阶段继续或重试。'
                        : installation.kind === 'unknown'
                          ? '暂时无法确认安装完整性，请先查看诊断，避免重复安装。'
                          : '配置 Steam 账号密码并安装 Stardew Valley、Steamworks SDK、SMAPI 与 JunimoServer。')}
              </span>
            </div>
          </div>
        </div>
      </section>

      {/* ── 安装进度区 ──────────────────────────────────────────────────── */}
      <section className="sd-install-progress-section">
        <div className="sd-install-section-title">安装进度</div>

        {/* 步骤条 */}
        <ol
          className="sd-install-steps"
          style={{ '--sd-install-progress-line': `${overallProgress}%` } as CSSProperties}
        >
          {STEPS.map((label, i) => (
            <li key={i} className={`sd-install-step sd-install-step-${stepStatuses[i]}`}>
              <span className="sd-install-step-number">{i + 1}</span>
              <img className="sd-install-step-art" src={STEP_ICON_SRC[i]} alt="" aria-hidden="true" />
              <span className="sd-install-step-label">{label}</span>
              <span className="sd-install-step-icon">{STEP_ICON[stepStatuses[i]]}</span>
            </li>
          ))}
        </ol>

        <div className="sd-install-overall-progress">
          <div
            className="sd-install-overall-track"
            role="progressbar"
            aria-label="安装总进度"
            aria-valuemin={0}
            aria-valuemax={100}
            aria-valuenow={overallProgress}
          >
            <div className="sd-install-overall-fill" style={{ width: `${overallProgress}%` }} />
          </div>
          <span>{overallProgress}%</span>
        </div>

        {/* 进度说明 */}
        {steamDownloadTaskProgress?.label || progressLabel ? (
          <div className="sd-install-progress-label">{steamDownloadTaskProgress?.label ?? progressLabel}</div>
        ) : null}
      </section>

      <div className="sd-install-workbench">
        <section className="sd-install-column sd-install-config-panel">
          <div className="sd-install-column-title">安装配置</div>

          {/* ── 已安装成功卡 ───────────────────────────────────────────────── */}
          {installation.kind === 'installed' ? (
            <div className="sd-install-complete-card">
              <span className="sd-install-complete-icon">✓</span>
              <div className="sd-install-complete-body">
                <div className="sd-install-complete-title">Stardew Valley 已安装</div>
                <div className="sd-install-complete-desc">服务器已就绪，可以前往「服务器控制」启动游戏。</div>
              </div>
              <button className="sd-btn-green" type="button" onClick={() => onNavigate('server')}>
                前往服务器控制
              </button>
              {isAdmin ? (
                <button
                  className="sd-btn-tan"
                  type="button"
                  onClick={() => { setEditingSteamCredentials(false); setShowForm(true); setInstallError('') }}
                >
                  校验 / 修复安装
                </button>
              ) : null}
            </div>
          ) : null}

          {needsInstallRepair ? (
            <div className="sd-install-complete-card sd-install-repair-card">
              <span className="sd-install-complete-icon">!</span>
              <div className="sd-install-complete-body">
                <div className="sd-install-complete-title">安装需要修复</div>
                <div className="sd-install-complete-desc">已确认必需文件、Compose 或镜像不完整；修复不会删除现有存档。</div>
              </div>
              {isAdmin ? (
                <button
                  className="sd-btn-green"
                  type="button"
                  onClick={() => { setEditingSteamCredentials(false); setShowForm(true); setInstallError('') }}
                >
                  检查并修复安装
                </button>
              ) : null}
            </div>
          ) : null}

          {needsInstallationDiagnosis ? (
            <div className={`sd-install-complete-card sd-install-diagnose-card${installation.kind === 'runtime_error' ? ' sd-install-diagnose-card--error' : ''}`}>
              <span className="sd-install-complete-icon">!</span>
              <div className="sd-install-complete-body">
                <div className="sd-install-complete-title">
                  {installation.kind === 'runtime_error' ? '服务器运行异常' : '安装状态尚未确认'}
                </div>
                <div className="sd-install-complete-desc">
                  当前没有“游戏未安装”的可靠证据。请先查看诊断，避免无依据地重新安装。
                </div>
              </div>
              <button className="sd-btn-tan" type="button" onClick={() => onNavigate('diagnostics')}>
                查看诊断
              </button>
              {installation.action === 'retry_start' ? (
                <button className="sd-btn-green" type="button" onClick={() => onNavigate('server')}>
                  前往服务器控制
                </button>
              ) : null}
            </div>
          ) : null}

          <div className="sd-install-actions">
            {isAdmin ? (
              <>
                <button
                  className="sd-btn-green"
                  type="button"
                  onClick={() => { void steamAuth.login() }}
                  disabled={steamInviteAuthorizationReady || steamInviteAuthorizationCleanupPending || steamAuth.busy || steamAuth.requiresStop || isInstalling || hasActiveSteamAuthJob || !isInstalled}
                  title={steamInviteAuthorizationCleanupPending
                    ? '授权已成功，正在安全收尾，请稍后刷新'
                    : steamInviteAuthorizationReady
                    ? undefined
                    : !isInstalled
                    ? '请先完成 SteamCMD 基础安装，再启用 Steam 邀请码'
                    : hasActiveSteamAuthJob
                      ? 'Steam 邀请码授权任务正在进行'
                      : steamAuth.title}
                >
                  {steamInviteAuthorizationCleanupPending
                    ? 'Steam 邀请码授权收尾中…'
                    : steamInviteAuthorizationReady
                    ? 'Steam 邀请码已启用'
                    : steamAuth.busy || hasActiveSteamAuthJob
                    ? '正在启用 Steam 邀请码…'
                    : '启用 Steam 邀请码（需要再次登录授权）'}
                </button>
                {isInstalled ? (
                  <button
                    className="sd-btn-tan"
                    type="button"
                    onClick={() => { setEditingSteamCredentials(true); setShowForm(true); setInstallError('') }}
                    disabled={isInstalling || hasActiveSteamAuthJob || installBusy || steamAuth.requiresStop || editingSteamCredentials}
                    title={steamAuth.requiresStop
                      ? '请先停止服务器，再修改 Steam 账号密码'
                      : isInstalling || hasActiveSteamAuthJob
                        ? '请等待当前任务结束后再修改 Steam 账号密码'
                        : '输入新的 Steam 账号密码'}
                  >
                    修改 Steam 账号密码
                  </button>
                ) : null}
              </>
            ) : null}
            {steamAuth.message ? (
              <div className="sd-srv-hint" style={{ color: '#b94040' }}>{steamAuth.message}</div>
            ) : null}
          </div>

          {/* ── 非 admin 提示 ──────────────────────────────────────────────── */}
          {!isAdmin && installation.canOpenInstallForm ? (
            <div className="sd-install-info-bar">
              仅管理员可以启动安装。请联系管理员完成 SteamCMD 授权和游戏安装。
            </div>
          ) : null}

          {/* ── 操作按钮区 ────────────────────────────────────────────────── */}
          {isAdmin && !isInstalling && !showForm ? (
            <div className="sd-install-actions">
              {showPrimaryInstallAction ? (
                <button
                  className="sd-btn-green sd-btn--lg"
                  type="button"
                  onClick={() => { setEditingSteamCredentials(false); setShowForm(true); setInstallError('') }}
                >
                    {isQrAuthError
                    ? '重试 Steam 邀请码授权'
                    : steamCMDRecoverable
                      ? '重试 SteamCMD 安装/修复'
                    : postAuthRecoverable
                      ? '重试下载（不重新输入账号）'
                    : authFailed
                        ? '重新输入 Steam 凭据'
                      : needsInstallRepair
                        ? '检查并修复安装'
                      : canDirectRetry
                        ? '继续 / 重试安装'
                        : '安装游戏'}
                </button>
              ) : null}
            </div>
          ) : null}

          {isAdmin && isInstalling && !showForm ? (
            <div className="sd-install-config-placeholder">
              安装配置已提交，当前任务正在执行。需要 Steam 交互时请在中间区域完成认证。
            </div>
          ) : null}

          {/* ── 安装配置表单 ───────────────────────────────────────────────── */}
          {showForm && isAdmin && (installation.canOpenInstallForm || editingSteamCredentials) ? (
            <div className="sd-install-form-card">
              <div className="sd-install-form-title">
                {editingSteamCredentials
                  ? '修改 Steam 账号密码'
                  : steamCMDRecoverable
                  ? '重试 SteamCMD 安装 / 修复'
                  : postAuthRecoverable
                  ? '重试下载 / 继续安装'
                  : isQrAuthError || (authFailed && !isInstalled)
                  ? '重新输入 Steam 凭据'
                  : installation.kind === 'installed'
                    ? '校验 / 修复安装'
                    : needsInstallRepair
                      ? '修复安装文件'
                    : canDirectRetry
                      ? '确认重试安装'
                      : '安装配置'}
              </div>
              {!editingSteamCredentials ? (
                <p className="sd-install-form-hint">
                  {steamCMDRecoverable
                  ? '上次 SteamCMD 授权、下载或校验未完成；本次会复用已保存账号密码继续安装/修复，本地已有工具镜像时不会重新拉取。'
                  : postAuthRecoverable
                  ? 'SteamCMD 授权已经成功，本次只会复用已保存凭据重试下载/后续安装步骤，不需要重新输入账号密码。'
                  : installation.kind === 'installed'
                    ? '本次会复用已保存凭据和 SteamCMD 安装授权缓存校验/修复游戏文件；不会启动 SteamAuth。'
                    : needsInstallRepair
                      ? '将复用已保存凭据校验并补齐缺失文件，不会删除现有存档。'
                  : canDirectRetry
                    ? '将使用已保存的 Steam 凭据继续未完成的安装，只需确认镜像版本。'
                    : '请输入 Steam 账号信息和 VNC 密码。密码不会出现在任何日志中。'}
                </p>
              ) : null}

              <form onSubmit={editingSteamCredentials ? handleSteamCredentialsSubmit : handleInstallSubmit} autoComplete="off">
                {!editingSteamCredentials && !optionsLoading && imageTagOptions.length > 0 ? (
                  <div className="sd-install-field">
                    <label className="sd-install-field-label">JunimoServer 镜像版本</label>
                    <select
                      className="sd-install-select"
                      value={imageTag}
                      onChange={(e) => setImageTag(e.target.value)}
                    >
                      {imageTagOptions.map((opt) => (
                        <option key={opt.tag} value={opt.tag}>
                          {opt.label}{opt.recommended ? ' ★' : ''}{opt.isLatest ? ' 已是最新版' : ''}
                        </option>
                      ))}
                    </select>
                    {selectedOption?.warning ? (
                      <p className="sd-install-version-warn">{selectedOption.warning}</p>
                    ) : null}
                  </div>
                ) : null}

                {!canDirectRetry || editingSteamCredentials ? (
                  <>
                    <div className="sd-install-field">
                      <label className="sd-install-field-label">Steam 用户名</label>
                      <input
                        className="sd-install-input"
                        type="text"
                        value={steamUsername}
                        autoComplete="steam-account"
                        required
                        onChange={(e) => setSteamUsername(e.target.value)}
                      />
                    </div>
                    <div className="sd-install-field">
                      <label className="sd-install-field-label">Steam 密码</label>
                      <div className="sd-install-pwd-row">
                        <input
                          className="sd-install-input"
                          type={showSteamPwd ? 'text' : 'password'}
                          value={steamPassword}
                          autoComplete="new-password"
                          required
                          onChange={(e) => setSteamPassword(e.target.value)}
                        />
                        <button
                          className="sd-btn-tan"
                          type="button"
                          onClick={() => setShowSteamPwd((v) => !v)}
                        >
                          {showSteamPwd ? '隐藏' : '显示'}
                        </button>
                      </div>
                    </div>
                    {!editingSteamCredentials ? (
                      <div className="sd-install-field">
                        <label className="sd-install-field-label">VNC 密码</label>
                        <div className="sd-install-pwd-row">
                          <input
                            className="sd-install-input"
                            type={showVncPwd ? 'text' : 'password'}
                            value={vncPassword}
                            autoComplete="new-password"
                            required
                            onChange={(e) => setVncPassword(e.target.value)}
                          />
                          <button
                            className="sd-btn-tan"
                            type="button"
                            onClick={() => setShowVncPwd((v) => !v)}
                          >
                            {showVncPwd ? '隐藏' : '显示'}
                          </button>
                        </div>
                      </div>
                    ) : null}
                    <p className="sd-install-form-hint" style={{ marginTop: 2 }}>
                      密码不会打印到任何日志或浏览器控制台。
                    </p>
                  </>
                ) : null}

                {installError ? (
                  <div className="sd-install-error-bar" style={{ marginTop: 8 }}>{installError}</div>
                ) : null}

                <div className="sd-install-form-actions">
                  <button className="sd-btn-green" type="submit" disabled={installBusy}>
                    {installBusy
                      ? editingSteamCredentials ? '正在保存…' : '正在启动安装…'
                      : editingSteamCredentials
                        ? '确认修改 Steam 账号密码'
                        : steamCMDRecoverable
                        ? '确认重试 SteamCMD 安装'
                        : installation.kind === 'installed' || needsInstallRepair
                          ? '确认修复 / 更新'
                        : canDirectRetry
                          ? '确认重试'
                          : '确认安装'}
                  </button>
                  <button
                    className="sd-btn-tan"
                    type="button"
                    disabled={installBusy}
                    onClick={() => { setEditingSteamCredentials(false); setShowForm(false); setInstallError('') }}
                  >
                    取消
                  </button>
                </div>
              </form>
            </div>
          ) : null}
        </section>

        <section className="sd-install-column sd-install-auth-panel">
          <div className={`sd-install-column-title ${activityPanelUsesDownloadIcon ? 'sd-install-column-title-download' : 'sd-install-column-title-steam'}`}>
            {activityPanelTitle}
          </div>

          {/* 拉取镜像进度卡 */}
          {(effectivePhase === 'pull_running' || effectivePhase === 'steamcmd_image_pulling') && isInstalling ? (
            <div className="sd-install-pull-card">
              <div className="sd-install-pull-header">
                <span className="sd-install-pull-icon">↓</span>
                <div>
                  <div className="sd-install-pull-title">
                    {effectivePhase === 'steamcmd_image_pulling' ? '正在下载 SteamCMD 安装工具镜像' : '正在下载 JunimoServer 镜像'}
                  </div>
                  <div className="sd-install-pull-sub">{stateMessage || '正在准备拉取镜像，请稍候...'}</div>
                </div>
              </div>
              {pullProgress ? (
                <div className="sd-install-prog-wrap">
                  <div
                    className="sd-install-prog-track"
                    role="progressbar"
                    aria-label="Docker 镜像下载进度"
                    aria-valuemin={0}
                    aria-valuemax={100}
                    aria-valuenow={pullProgress.percent}
                  >
                    <div
                      className={`sd-install-prog-fill${pullProgress.done === pullProgress.total ? ' done' : ''}`}
                      style={{ width: `${pullProgress.percent}%` }}
                    />
                  </div>
                  <span className="sd-install-prog-pct">约 {pullProgress.percent}% ({pullProgress.done}/{pullProgress.total})</span>
                </div>
              ) : (
                <div className="sd-install-waiting">等待 Docker 开始下载...</div>
              )}
              <p className="sd-install-pull-hint">
                {effectivePhase === 'steamcmd_image_pulling'
                  ? 'SteamCMD 是一次性安装/修复下载工具，拉取完成后会继续安装，不会加入日常运行栈。'
                  : '首次下载约需 10–30 分钟，取决于网络速度。'}
              </p>
            </div>
          ) : null}

          {/* 游戏/SDK 下载提示 */}
          {effectivePhase === 'smapi_installing' && isInstalling ? (
            <div className="sd-install-download-card sd-install-smapi-card">
              <div className="sd-install-dl-title" role="status" aria-live="polite" aria-atomic="true">
                {smapiArchiveProgress && smapiArchiveProgress.percent < 100
                  ? '正在下载 SMAPI 安装包…'
                  : '正在安装 SMAPI 运行环境…'}
              </div>
              <div className="sd-install-dl-hint">
                {smapiArchiveProgress?.cached
                  ? '本地安装包缓存已通过校验，无需重复下载，正在写入游戏运行目录。'
                  : smapiArchiveProgress?.percent === 100
                    ? '安装包已下载完成，正在校验并写入游戏运行目录。'
                    : smapiArchiveProgress
                      ? `正在从下载源 ${smapiArchiveProgress.candidate}/${smapiArchiveProgress.candidateCount} 获取安装包；进度会自动更新。`
                      : '正在检查本地安装包缓存；如需下载，页面会在收到首个数据块后显示实时字节进度。'}
              </div>
              {smapiArchiveProgress ? (
                <div className="sd-install-prog-wrap sd-install-smapi-progress">
                  <div
                    className="sd-install-prog-track"
                    role="progressbar"
                    aria-label="SMAPI 安装包下载进度"
                    aria-valuemin={0}
                    aria-valuemax={100}
                    aria-valuenow={smapiArchiveProgress.percent}
                  >
                    <div
                      className={`sd-install-prog-fill${smapiArchiveProgress.percent >= 100 ? ' done' : ''}`}
                      style={{ width: `${smapiArchiveProgress.percent}%` }}
                    />
                  </div>
                  <span className="sd-install-prog-pct">
                    {smapiArchiveProgress.cached
                      ? '缓存已校验'
                      : `${formatBytes(smapiArchiveProgress.downloadedBytes)} / ${formatBytes(smapiArchiveProgress.totalBytes)} · ${smapiArchiveProgress.percent}%`}
                  </span>
                </div>
              ) : null}
              <div className="sd-install-activity">
                <span className="sd-install-activity-dot" />
                <span>
                  {smapiArchiveProgress?.percent === 100 || smapiArchiveProgress?.cached
                    ? '下载阶段已结束，安装任务仍在继续'
                    : smapiArchiveProgress
                      ? '下载仍在进行，进度会自动更新'
                      : '安装任务仍在进行，状态会自动更新'}
                </span>
              </div>
            </div>
          ) : null}

          {(effectivePhase === 'game_downloading' || effectivePhase === 'steam_sdk_downloading' || effectivePhase === 'steamcmd_downloading') && isInstalling ? (
            <div className="sd-install-download-card">
              <div className="sd-install-dl-title">
                {isSteamCMDClientUpdating
                  ? 'SteamCMD 正在更新客户端中…'
                  : effectivePhase === 'steamcmd_downloading'
                  ? 'SteamCMD 正在下载并校验游戏文件…'
                  : effectivePhase === 'steam_sdk_downloading'
                  ? '下载 Steam SDK 运行文件中…'
                  : '下载 Stardew Valley 游戏文件中…'}
              </div>
              <div className="sd-install-dl-hint">
                {isSteamCMDClientUpdating
                  ? 'Docker 镜像已经就绪；这里显示的是 SteamCMD 容器内客户端自更新进度，完成后会进入登录授权。'
                  : effectivePhase === 'steamcmd_downloading'
                  ? 'SteamCMD 正在复用已保存凭据和安装授权缓存，依次准备 Stardew Valley 与 Steamworks SDK。'
                  : '大文件下载中，请耐心等待（约 10–30 分钟）。下载完成后面板会自动继续。'}
              </div>
              {steamDownloadProgress ? (
                <div className="sd-install-prog-wrap">
                  <div
                    className="sd-install-prog-track"
                    role="progressbar"
                    aria-label="Steam 下载进度"
                    aria-valuemin={0}
                    aria-valuemax={100}
                    aria-valuenow={steamDownloadProgress.percent}
                  >
                    <div
                      className={`sd-install-prog-fill${steamDownloadProgress.percent >= 100 ? ' done' : ''}`}
                      style={{ width: `${steamDownloadProgress.percent}%` }}
                    />
                  </div>
                  <span className="sd-install-prog-pct">
                    {steamDownloadProgress.itemLabel ?? `${steamDownloadProgress.filesDone}/${steamDownloadProgress.filesTotal} 个文件`}
                    {' · '}
                    {steamDownloadProgress.bytesDone}/{steamDownloadProgress.bytesTotal}
                  </span>
                </div>
              ) : effectivePhase === 'steamcmd_downloading' ? (
                <div className="sd-install-waiting">SteamCMD 下载进度以任务日志为准，完成后会自动进入下一步。</div>
              ) : (
                <div className="sd-install-waiting">正在等待 Steam 输出下载进度...</div>
              )}
            </div>
          ) : null}

          {/* ── Steam 认证交互区 ───────────────────────────────────────────── */}
          {(needsAuthMethodChoice || needsGuardChoice || needsGuard || needsQrCode || needsSteamCMDGuardChoice || needsSteamCMDGuard) && !isAdmin ? (
            <div className="sd-install-guard-section">
              <div className="sd-install-guard-block">
                <div className="sd-install-guard-desc">
                  {needsSteamCMDGuardChoice || needsSteamCMDGuard
                    ? 'SteamCMD 基础安装正在等待管理员完成 Steam Guard 授权。'
                    : 'Steam 邀请码授权正在进行中，请等待管理员完成验证。'}
                </div>
              </div>
            </div>
          ) : (needsAuthMethodChoice || needsGuardChoice || needsGuard || needsQrCode || needsSteamCMDGuardChoice || needsSteamCMDGuard) ? (
            <div className="sd-install-guard-section">
              {/* 选择登录方式 */}
              {needsAuthMethodChoice ? (
                <div className="sd-install-guard-block">
                  <div className="sd-install-guard-title">选择 Steam 登录方式</div>
                  <p className="sd-install-guard-desc">
                    请选择扫码登录（Steam 手机 App），或使用已填写的账号密码继续。
                    账号密码方式如触发二次验证，会再提示选择 Steam Guard 方式。
                  </p>
                  <div className="sd-install-guard-actions">
                    <button
                      className="sd-btn-green"
                      type="button"
                      disabled={guardBusy}
                      onClick={() => void handleAuthMethodSelect('2')}
                    >
                      {guardBusy ? '提交中…' : '扫码登录'}
                    </button>
                    <button
                      className="sd-btn-tan"
                      type="button"
                      disabled={guardBusy}
                      onClick={() => void handleAuthMethodSelect('1')}
                    >
                      {guardBusy ? '提交中…' : '账号密码 / 验证码登录'}
                    </button>
                  </div>
                  {guardError ? <div className="sd-install-guard-error">{guardError}</div> : null}
                </div>
              ) : null}

              {/* 选择 Guard 方式 */}
              {needsGuardChoice ? (
                <div className="sd-install-guard-block">
                  <div className="sd-install-guard-title">选择 Steam Guard 验证方式</div>
                  <p className="sd-install-guard-desc">Steam 要求二步验证，请选择与任务日志菜单一致的方式。</p>
                  <div className="sd-install-guard-actions">
                    <button
                      className="sd-btn-green"
                      type="button"
                      disabled={guardBusy}
                      onClick={() => void handleAuthMethodSelect('1')}
                    >
                      {guardBusy ? '提交中…' : '手机 App 批准'}
                    </button>
                    <button
                      className="sd-btn-tan"
                      type="button"
                      disabled={guardBusy}
                      onClick={() => void handleAuthMethodSelect('2')}
                    >
                      {guardBusy ? '提交中…' : '输入验证码'}
                    </button>
                  </div>
                  {guardError ? <div className="sd-install-guard-error">{guardError}</div> : null}
                </div>
              ) : null}

              {/* Guard 验证码输入 / 手机批准 */}
              {needsGuard ? (
                <div className="sd-install-guard-block">
                  <div className="sd-install-guard-title">Steam Guard 验证</div>
                  {effectivePhase === 'steam_guard_required' ? (
                    <>
                      {guardSubmittedKind === 'steam' ? (
                        <div className="sd-install-guard-mobile">
                          <span className="sd-dot sd-dot-yellow" aria-hidden="true" />
                          <span>验证码已提交，正在等待 Steam 响应；如果长时间没有进展，可以重新输入验证码。</span>
                          <button
                            className="sd-btn-tan"
                            type="button"
                            onClick={() => setGuardSubmittedKind(null)}
                          >
                            重新输入
                          </button>
                        </div>
                      ) : (
                        <>
                          <p className="sd-install-guard-desc">Steam 已发送验证码，请在下方输入：</p>
                          <form
                            className="sd-install-guard-form"
                            onSubmit={(e) => void handleGuardSubmit(e, 'steam')}
                          >
                            <input
                              className="sd-install-input"
                              type="text"
                              placeholder="输入 Steam Guard 验证码"
                              value={guardInput}
                              onChange={(e) => setGuardInput(e.target.value)}
                              autoComplete="off"
                              required
                            />
                            <button className="sd-btn-green" type="submit" disabled={guardBusy}>
                              {guardBusy ? '提交中…' : '提交验证码'}
                            </button>
                          </form>
                        </>
                      )}
                      {guardError ? <div className="sd-install-guard-error">{guardError}</div> : null}
                    </>
                  ) : null}
                  {effectivePhase === 'steam_guard_mobile_required' ? (
                    <div className="sd-install-guard-mobile">
                      <span className="sd-dot sd-dot-yellow" aria-hidden="true" />
                      <span>请打开 Steam 手机 App，批准此次登录请求后面板会自动继续。</span>
                    </div>
                  ) : null}
                </div>
              ) : null}

              {/* SteamCMD 安装授权 */}
              {needsSteamCMDGuardChoice ? (
                <div className="sd-install-guard-block">
                  <div className="sd-install-guard-title">SteamCMD 需要重新授权</div>
                  <p className="sd-install-guard-desc">
                    SteamCMD 是基础安装主链，正在使用已保存账号密码登录。请选择与 Steam 提示一致的授权方式。
                  </p>
                  <div className="sd-install-guard-actions">
                    <button
                      className="sd-btn-green"
                      type="button"
                      disabled={guardBusy}
                      onClick={() => void handleAuthMethodSelect('1')}
                    >
                      {guardBusy ? '提交中…' : '手机 App 批准'}
                    </button>
                    <button
                      className="sd-btn-tan"
                      type="button"
                      disabled={guardBusy}
                      onClick={() => void handleAuthMethodSelect('2')}
                    >
                      {guardBusy ? '提交中…' : 'App / 邮箱验证码'}
                    </button>
                  </div>
                  {guardError ? <div className="sd-install-guard-error">{guardError}</div> : null}
                </div>
              ) : null}

              {needsSteamCMDGuard ? (
                <div className="sd-install-guard-block">
                  <div className="sd-install-guard-title">授权 SteamCMD 安装下载</div>
                  {effectivePhase === 'steamcmd_guard_required' ? (
                    <>
                      {guardSubmittedKind === 'steamcmd' ? (
                        <div className="sd-install-guard-mobile">
                          <span className="sd-dot sd-dot-yellow" aria-hidden="true" />
                          <span>验证码已提交，正在等待 SteamCMD 响应；通过后会自动进入下载/校验阶段。</span>
                          <button
                            className="sd-btn-tan"
                            type="button"
                            onClick={() => setGuardSubmittedKind(null)}
                          >
                            重新输入
                          </button>
                        </div>
                      ) : (
                        <>
                          <p className="sd-install-guard-desc">
                            请输入 Steam 手机 App 或邮箱收到的验证码。验证码只会发送给当前 SteamCMD 登录进程，不会写入日志。
                          </p>
                          <form
                            className="sd-install-guard-form"
                            onSubmit={(e) => void handleGuardSubmit(e, 'steamcmd')}
                          >
                            <input
                              className="sd-install-input"
                              type="text"
                              placeholder="输入 SteamCMD 验证码"
                              value={guardInput}
                              onChange={(e) => setGuardInput(e.target.value)}
                              autoComplete="off"
                              required
                            />
                            <button className="sd-btn-green" type="submit" disabled={guardBusy}>
                              {guardBusy ? '提交中…' : '提交验证码'}
                            </button>
                          </form>
                        </>
                      )}
                      {guardError ? <div className="sd-install-guard-error">{guardError}</div> : null}
                    </>
                  ) : null}
                  {effectivePhase === 'steamcmd_guard_mobile_required' ? (
                    <div className="sd-install-guard-mobile">
                      <span className="sd-dot sd-dot-yellow" aria-hidden="true" />
                      <span>请打开 Steam 手机 App，批准 SteamCMD 登录请求；批准后安装下载会自动继续。</span>
                    </div>
                  ) : null}
                </div>
              ) : null}

              {/* QR 扫码 */}
              {needsQrCode ? (
                <div className="sd-install-guard-block">
                  <div className="sd-install-guard-title">Steam 手机扫码</div>
                  <p className="sd-install-guard-desc">
                    请使用 Steam 手机 App 扫描日志中输出的二维码。如二维码还未出现，请稍等几秒。
                  </p>
                  <div className="sd-install-guard-actions">
                    <button
                      className="sd-btn-green"
                      type="button"
                      disabled={!qrUrl}
                      onClick={() => setShowQrModal(true)}
                    >
                      打开扫码窗口
                    </button>
                  </div>
                  {!qrUrl ? (
                    <p className="sd-install-guard-desc" style={{ marginTop: 4 }}>
                      正在等待容器输出二维码...
                    </p>
                  ) : null}
                </div>
              ) : null}
            </div>
          ) : (
            <div className="sd-install-auth-placeholder">
              <img className="sd-install-auth-orb" src={STEAM_STEP_ICON_SRC} alt="" aria-hidden="true" />
              <p>
                {installation.kind === 'installed'
                  ? instanceState?.steamInviteEnabled === true
                    ? '基础安装已完成。Steam 邀请码授权状态与交互会在这里显示；局域网/IP 直连不依赖它。'
                    : 'SteamCMD 基础安装已完成。Steam 邀请码默认关闭，需要时可由管理员按需启用。'
                  : isInstalling
                    ? 'SteamCMD 安装流程运行中，需要 Guard 或验证码时会在这里显示。'
                    : needsInstallRepair
                      ? '修复安装会复用已保存凭据；现有存档不会被删除。'
                      : needsInstallationDiagnosis
                        ? '当前异常尚未证实由安装缺失导致，请先查看诊断，避免重复安装。'
                    : '启动安装后，这里会显示 SteamCMD Guard 或验证码输入。Steam 邀请码扫码授权需安装完成后按需启用。'}
              </p>
            </div>
          )}

          {/* ── SSE 断线提示 ─────────────────────────────────────────────── */}
          {sseError ? (
            <div className="sd-install-sse-warn">{sseError}</div>
          ) : null}
        </section>

        <section className="sd-install-column sd-install-log-panel">
          {/* ── 安装 / 邀请码授权日志预览 ─────────────────────────────────── */}
          <div className="sd-install-log-section">
            <div className="sd-install-section-title">
              安装 / 邀请码授权日志
              {displayedTaskIsActive ? (
                <span className="sd-jobs-sse-dot" aria-label="实时接收中" />
              ) : null}
              <span className="sd-install-log-order-note">最新日志在最上方（倒序显示）</span>
            </div>
            {!installJobId ? (
              <div className="sd-install-log-empty">等待安装或邀请码授权任务启动...</div>
            ) : logs.length === 0 ? (
              <div className="sd-install-log-empty">等待日志输出...</div>
            ) : (
              <div className="sd-install-log-window">
                {visibleLogs.map((log) => (
                  <div
                    key={`${log.jobId ?? ''}-${log.sequence}`}
                    className={`sd-install-log-line sd-install-log-${log.level}`}
                  >
                    <span className="sd-install-log-seq">{log.sequence}</span>
                    <span className="sd-install-log-level">{log.level}</span>
                    <span className="sd-install-log-msg">{log.message}</span>
                  </div>
                ))}
              </div>
            )}
            {displayableLogs.length > visibleLogs.length ? (
              <p className="sd-install-log-hint">
                仅显示最近 50 条。查看完整日志请前往{' '}
                <button
                  className="sd-inline-nav"
                  type="button"
                  onClick={() => onNavigate('jobs')}
                >
                  任务与日志
                </button>
                。
              </p>
            ) : null}
          </div>
        </section>
      </div>

      {/* ── QR 弹窗 ──────────────────────────────────────────────────────── */}
      {showQrModal ? (
        <div className="sd-install-qr-overlay" role="dialog" aria-modal="true">
          <div className="sd-install-qr-card">
            <div className="sd-install-qr-header">
              <span className="sd-install-qr-title">Steam 手机扫码</span>
              <button className="sd-btn-tan" type="button" onClick={() => setShowQrModal(false)}>
                关闭
              </button>
            </div>
            {qrUrl ? (
              <>
                {qrImageSrc ? (
                  <div className="sd-install-qr-image-wrap">
                    <img className="sd-install-qr-image" src={qrImageSrc} alt="Steam 登录二维码" />
                  </div>
                ) : qrImageError && qrText ? (
                  <pre className="sd-install-qr-pre" style={{ fontSize: `${qrCodeFontSize(qrText)}px` }}>
                    {qrText}
                  </pre>
                ) : (
                  <p className="sd-install-guard-desc">正在生成二维码图片...</p>
                )}
                {qrImageError ? (
                  <p className="sd-install-guard-error">{qrImageError}</p>
                ) : null}
                <p className="sd-install-qr-link">
                  扫不了时可在手机上打开：<span>{qrUrl}</span>
                </p>
              </>
            ) : (
              <p className="sd-install-guard-desc">正在等待容器输出二维码...</p>
            )}
          </div>
        </div>
      ) : null}

    </div>
  )
}
import './InstallPage.css'
