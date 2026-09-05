import type { InstanceState, Job, JobLog } from '../../types'
import {
  extractPullProgress,
  extractSMAPIArchiveProgress,
  extractSteamDownloadProgress,
} from './install-helpers.ts'
import { classifyInstallationState } from './installation-state.ts'

export const GAME_INSTALL_STEPS = [
  '准备环境',
  '拉取服务镜像',
  'SteamCMD 授权',
  '下载与安装',
  '完成',
] as const

export type GameInstallStepStatus = 'pending' | 'active' | 'done' | 'error'

export type GameInstallProgressPresentation = {
  mode: 'idle' | 'active' | 'failed' | 'complete'
  title: string
  detail: string
  stepIndex: number
  steps: readonly GameInstallStepStatus[]
  percent: number | null
  approximate: boolean
  overallPercent: number
}

export function gameInstallStepProgressLabel(presentation: GameInstallProgressPresentation): string {
  if (presentation.mode === 'failed') return '请重试'
  if (presentation.mode === 'idle') return '等待安装'
  if (presentation.percent === null) return '进行中'
  return `${presentation.approximate ? '约 ' : ''}${presentation.percent}%`
}

const IMAGE_PHASES = new Set(['pull_running', 'pull_failed', 'steamcmd_image_pulling', 'steamcmd_image_pull_failed'])
const AUTH_PHASES = new Set([
  'steam_auth_running',
  'auth_method_required',
  'steam_guard_choice_required',
  'steam_guard_required',
  'steam_guard_mobile_required',
  'steam_qr_required',
  'steam_auth_done',
  'steamcmd_auth_running',
  'steamcmd_guard_choice_required',
  'steamcmd_guard_required',
  'steamcmd_guard_mobile_required',
  'credentials_required',
])
const DOWNLOAD_PHASES = new Set([
  'game_downloading',
  'steam_sdk_downloading',
  'steamcmd_downloading',
  'smapi_installing',
  'download_failed',
  'post_auth_failed',
  'smapi_install_failed',
  'smapi_bundled_sync_failed',
])

function isTerminalInstallJob(job: Job): boolean {
  return job.status === 'succeeded' || job.status === 'failed' || job.status === 'canceled'
}

function phaseStepIndex(phase: string): number {
  if (IMAGE_PHASES.has(phase)) return 1
  if (AUTH_PHASES.has(phase)) return 2
  if (DOWNLOAD_PHASES.has(phase)) return 3
  if (phase === 'game_installed' || phase === 'installed') return 4
  return 0
}

function phaseTitle(phase: string, mode: GameInstallProgressPresentation['mode']): string {
  if (mode === 'complete') return '安装完成'
  if (mode === 'failed') return '安装未完成'
  const titles: Record<string, string> = {
    pull_running: '拉取服务镜像',
    steamcmd_image_pulling: '拉取 SteamCMD 工具',
    steam_auth_running: 'Steam 授权中',
    auth_method_required: '使用账号密码登录',
    steam_guard_choice_required: '确认 Steam Guard',
    steam_guard_required: '输入 Steam Guard 验证码',
    steam_guard_mobile_required: '等待手机确认',
    steam_qr_required: '旧版登录任务',
    steam_auth_done: 'Steam 授权完成',
    steamcmd_auth_running: 'SteamCMD 登录中',
    steamcmd_guard_choice_required: '确认 SteamCMD 验证',
    steamcmd_guard_required: '输入 SteamCMD 验证码',
    steamcmd_guard_mobile_required: '等待手机确认 SteamCMD 登录',
    game_downloading: '下载游戏文件',
    steam_sdk_downloading: '下载 Steam SDK',
    steamcmd_downloading: 'SteamCMD 下载与校验',
    smapi_installing: '安装 SMAPI 环境',
  }
  return titles[phase] ?? '准备安装环境'
}

function phaseDetail(phase: string): string {
  const details: Record<string, string> = {
    pull_running: '正在拉取 JunimoServer 运行镜像。',
    steamcmd_image_pulling: '正在拉取一次性 SteamCMD 安装工具。',
    steam_auth_running: '正在连接 Steam 并验证下载权限。',
    auth_method_required: '正在自动使用已保存的 Steam 账号密码登录。',
    steam_guard_choice_required: '已自动选择 Steam 手机 App 批准。',
    steam_guard_required: '请输入 Steam App 或邮箱收到的验证码。',
    steam_guard_mobile_required: '请在 Steam 手机 App 中批准本次登录。',
    steam_qr_required: '当前旧登录任务无法继续，请使用账号密码重新开始。',
    steam_auth_done: '授权已保存，正在进入下载阶段。',
    steamcmd_auth_running: '正在复用已保存凭据登录 SteamCMD。',
    steamcmd_guard_choice_required: '已自动选择 Steam 手机 App 批准 SteamCMD 登录。',
    steamcmd_guard_required: '请输入 Steam App 或邮箱收到的验证码。',
    steamcmd_guard_mobile_required: '请在 Steam 手机 App 中批准 SteamCMD 登录。',
    game_downloading: '正在校验并下载 Stardew Valley 游戏文件。',
    steam_sdk_downloading: '游戏本体已完成，正在下载联机运行文件。',
    steamcmd_downloading: 'SteamCMD 正在下载并校验游戏本体与 Steamworks SDK。',
    smapi_installing: '正在下载、校验并安装 SMAPI 运行环境。',
  }
  return details[phase] ?? '正在准备目录、权限和安装运行环境。'
}

function exactPhaseProgress(
  phase: string,
  jobType: string | undefined,
  logs: JobLog[],
): { percent: number; approximate: boolean } | null {
  if (phase === 'pull_running' || phase === 'steamcmd_image_pulling') {
    const progress = extractPullProgress(logs)
    return progress ? { percent: progress.percent, approximate: true } : null
  }
  if (phase === 'smapi_installing') {
    const progress = extractSMAPIArchiveProgress(logs, jobType)
    return progress ? { percent: progress.percent, approximate: false } : null
  }
  if (phase === 'game_downloading') {
    const progress = extractSteamDownloadProgress(logs, jobType, 'game')
    return progress ? { percent: progress.percent, approximate: false } : null
  }
  if (phase === 'steam_sdk_downloading') {
    const progress = extractSteamDownloadProgress(logs, jobType, 'sdk')
    return progress ? { percent: progress.percent, approximate: false } : null
  }
  if (phase === 'steamcmd_downloading') {
    const sdk = extractSteamDownloadProgress(logs, jobType, 'sdk')
    const game = extractSteamDownloadProgress(logs, jobType, 'game')
    const client = extractSteamDownloadProgress(logs, jobType, 'steamcmd_update')
    const progress = sdk ?? game ?? client
    return progress ? { percent: progress.percent, approximate: false } : null
  }
  return null
}

function stepStatuses(
  mode: GameInstallProgressPresentation['mode'],
  stepIndex: number,
): readonly GameInstallStepStatus[] {
  return GAME_INSTALL_STEPS.map((_, index) => {
    if (mode === 'complete') return 'done'
    if (index < stepIndex) return 'done'
    if (index > stepIndex) return 'pending'
    if (mode === 'failed') return 'error'
    if (mode === 'active') return 'active'
    return 'pending'
  })
}

// Overall progress is a stage-weighted estimate, not a byte or time estimate.
// Missing telemetry stays at the stage boundary; only verified completion is 100%.
function overallInstallPercent(
  mode: GameInstallProgressPresentation['mode'],
  phase: string,
  jobType: string | undefined,
  logs: JobLog[],
  phasePercent: number | null,
): number {
  if (mode === 'complete') return 100
  if (mode === 'idle') return 0
  const within = (start: number, end: number, percent: number | null) =>
    Math.floor(start + (end - start) * Math.min(100, Math.max(0, percent ?? 0)) / 100)
  if (phase === 'pull_running' || phase === 'pull_failed') return within(5, 15, phasePercent)
  if (phase === 'steamcmd_image_pulling' || phase === 'steamcmd_image_pull_failed') return within(15, 20, phasePercent)
  if (AUTH_PHASES.has(phase)) return phase === 'steam_auth_done' ? 25 : 20
  if (phase === 'game_downloading') return within(25, 85, phasePercent)
  if (phase === 'steam_sdk_downloading') return within(85, 90, phasePercent)
  if (phase === 'smapi_installing' || phase === 'smapi_install_failed' || phase === 'smapi_bundled_sync_failed') {
    return within(90, 99, phasePercent)
  }
  if (phase === 'steamcmd_downloading' || phase === 'download_failed' || phase === 'post_auth_failed') {
    const sdk = extractSteamDownloadProgress(logs, jobType, 'sdk')
    const game = extractSteamDownloadProgress(logs, jobType, 'game')
    if (sdk) return within(85, 90, sdk.percent)
    if (game) return within(25, 85, game.percent)
    // This phase also covers SteamCMD's self-update before login: its 100%
    // must never imply that the game itself has been downloaded.
    return 20
  }
  if (phase === 'game_installed' || phase === 'installed') return 99
  return 0
}

export function gameInstallProgressPresentation(
  state: InstanceState | null,
  job: Job | null,
  logs: JobLog[],
  requiredFilesMissing = false,
): GameInstallProgressPresentation {
  if (job?.type === 'stardew_steam_auth') {
    const mode = job.status === 'succeeded' ? 'complete'
      : job.status === 'failed' || job.status === 'canceled' ? 'failed' : 'active'
    const phase = state?.driverPhase ?? ''
    return {
      mode,
      title: mode === 'complete' ? 'Steam 邀请授权完成' : mode === 'failed' ? 'Steam 邀请授权未完成' : phaseTitle(phase, mode),
      detail: mode === 'complete' ? '此世界的 Steam 邀请授权已完成。'
        : mode === 'failed' ? job.errorMessage || state?.stateMessage || '请重新授权。'
          : state?.stateMessage || phaseDetail(phase),
      stepIndex: 2, steps: stepStatuses(mode, 2), percent: mode === 'complete' ? 100 : null,
      approximate: false, overallPercent: mode === 'complete' ? 100 : 0,
    }
  }
  const activeJob = job?.type === 'stardew_install' && !isTerminalInstallJob(job)
  const installation = classifyInstallationState(state, activeJob)
  const phase = state?.driverPhase ?? ''
  const failed = job?.status === 'failed'
    || job?.status === 'canceled'
    || installation.kind === 'install_failed'
    || installation.kind === 'repair_required'
    || requiredFilesMissing
  const complete = !failed && installation.isInstalled && !activeJob
  const mode: GameInstallProgressPresentation['mode'] = complete
    ? 'complete'
    : failed
      ? 'failed'
      : activeJob || installation.kind === 'installing'
        ? 'active'
        : 'idle'
  const stepIndex = complete ? 4 : requiredFilesMissing ? 3 : phaseStepIndex(phase)
  const progress = mode === 'complete' ? { percent: 100, approximate: false } : exactPhaseProgress(phase, job?.type, logs)
  const invalidPassword = mode === 'failed' && job?.type === 'stardew_install' && logs.some((log) =>
    log.jobId === job.id && /invalid password|incorrect password|password check for user failed/i.test(log.message),
  )
  const detail = mode === 'failed'
    ? invalidPassword
      ? 'Steam 账号或密码错误，请修改后重试。'
      : /SteamCMD install authorization confirmation timed out/i.test(job?.errorMessage ?? '')
        ? 'Steam 登录授权确认超时，请重新安装并及时完成 Steam 验证。'
      : requiredFilesMissing
      ? '游戏运行文件不完整，请重新安装或修复。'
      : phase === 'credentials_required' && state?.stateMessage
        ? state.stateMessage
      : job?.errorMessage || state?.stateMessage || '安装任务失败，请核对当前步骤后重试。'
    : mode === 'complete'
      ? '游戏本体、Steamworks SDK 与运行环境均已就绪。'
      : state?.stateMessage || phaseDetail(phase)

  return {
    mode,
    title: phaseTitle(phase, mode),
    detail,
    stepIndex,
    steps: stepStatuses(mode, stepIndex),
    percent: progress?.percent ?? null,
    approximate: progress?.approximate ?? false,
    overallPercent: requiredFilesMissing && !job ? 0 : overallInstallPercent(mode, phase, job?.type, logs, progress?.percent ?? null),
  }
}
