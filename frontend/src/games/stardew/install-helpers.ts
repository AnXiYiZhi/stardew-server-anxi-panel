import type { Job, JobLog } from '../../types'
import { roundPercent } from '../../core/helpers'

type DownloadProgress = {
  filesDone: number
  filesTotal: number
  percent: number
  bytesDone: string
  bytesTotal: string
}

const steamDownloadProgressRe = /\[steam\].*Progress:\s*(\d+)\/(\d+)\s+files\s+-\s+([^/()]+?)\/([^()]+?)\s+\((\d+(?:\.\d+)?)%\)/i

export function extractSteamDownloadProgress(logs: JobLog[], jobType: string | undefined, kind: 'game' | 'sdk'): DownloadProgress | null {
  if (jobType !== 'stardew_install') return null
  let currentKind: 'game' | 'sdk' | null = null
  let sawSdk = false
  let latest: DownloadProgress | null = null
  for (const log of logs) {
    const lower = log.message.toLowerCase()
    if (lower.includes('[steam]') && lower.includes('downloading app 413150')) {
      currentKind = 'game'
    } else if (
      lower.includes('[steam]') &&
      (lower.includes('downloading app 1007') || lower.includes('.steam-sdk'))
    ) {
      currentKind = 'sdk'
      sawSdk = true
    }

    const m = log.message.match(steamDownloadProgressRe)
    if (m) {
      const progressKind = currentKind ?? (sawSdk ? 'sdk' : 'game')
      if (progressKind === kind) {
        latest = {
          filesDone: parseInt(m[1], 10),
          filesTotal: parseInt(m[2], 10),
          bytesDone: m[3].trim(),
          bytesTotal: m[4].trim(),
          percent: fileCountPercent(parseInt(m[1], 10), parseInt(m[2], 10)),
        }
      }
    } else if (lower.includes('[steam]') && lower.includes('download complete')) {
      if (currentKind === kind) latest = completeSteamDownloadProgress(latest)
    }
  }
  return latest
}

export function hasSteamSdkDownloadStarted(logs: JobLog[], jobType: string | undefined): boolean {
  if (jobType !== 'stardew_install') return false
  return logs.some((log) => {
    const lower = log.message.toLowerCase()
    return lower.includes('[steam]') && (lower.includes('downloading app 1007') || lower.includes('.steam-sdk'))
  })
}

export function hasSteamSdkDownloadCompleted(logs: JobLog[], jobType: string | undefined): boolean {
  if (jobType !== 'stardew_install') return false
  return logs.some((log) => {
    const lower = log.message.toLowerCase()
    return lower.includes('[steam]') && lower.includes('app installed to:') && lower.includes('/data/game/.steam-sdk')
  })
}

function fileCountPercent(done: number, total: number): number {
  if (total <= 0) return 0
  return roundPercent((done / total) * 100)
}

function completeSteamDownloadProgress(progress: DownloadProgress | null): DownloadProgress | null {
  if (!progress) return null
  return {
    filesDone: progress.filesTotal,
    filesTotal: progress.filesTotal,
    bytesDone: progress.bytesTotal,
    bytesTotal: progress.bytesTotal,
    percent: 100,
  }
}

export function calcSteamDownloadTaskProgress(
  phase: string,
  gameProgress: DownloadProgress | null,
  sdkProgress: DownloadProgress | null,
) {
  if (phase !== 'game_downloading' && phase !== 'steam_sdk_downloading') return null
  if (phase === 'steam_sdk_downloading') {
    return {
      done: sdkProgress?.percent === 100 ? 2 : 1,
      total: 2,
      percent: sdkProgress?.percent === 100 ? 100 : roundPercent(50 + (sdkProgress?.percent ?? 0) / 2),
      label: sdkProgress?.percent === 100
        ? '游戏文件和 Steam SDK 均已下载完成。'
        : '游戏文件已下载完成，正在下载 Steam SDK 运行文件。',
    }
  }
  return {
    done: gameProgress?.percent === 100 ? 1 : 0,
    total: 2,
    percent: roundPercent((gameProgress?.percent ?? 0) / 2),
    label: '正在校验/下载 Stardew Valley 游戏文件；已存在且校验通过的文件会自动跳过。',
  }
}

export function extractRecentSteamQrText(logs: JobLog[]): string {
  const steamLines = logs
    .filter((log) => log.message.startsWith('[steam] '))
    .map((log) => log.message.replace(/^\[steam\] /, ''))
  let qrIndex = -1
  for (let i = steamLines.length - 1; i >= 0; i -= 1) {
    if (steamLines[i].toLowerCase().includes('qr')) {
      qrIndex = i
      break
    }
  }
  if (qrIndex < 0) return steamLines.slice(-20).join('\n')
  return steamLines.slice(qrIndex, qrIndex + 40).join('\n')
}

export function installFailureDisplayMessage(
  state: string,
  phase: string,
  stateMessage: string,
  latestInstallJob: Job | undefined,
  selectedJob: Job | null,
  logs: JobLog[],
): string {
  const failedInstallJob = latestInstallJob?.type === 'stardew_install' && latestInstallJob.status === 'failed'
    ? latestInstallJob
    : null
  const failedSelectedInstallJob = selectedJob?.type === 'stardew_install' && selectedJob.status === 'failed'
    ? selectedJob
    : null
  const failedJob = failedInstallJob ?? failedSelectedInstallJob
  const errorPhase = [
    'pull_failed',
    'install_timeout',
    'steam_auth_connection_failed',
    'steam_auth_failed',
    'credentials_required',
    'qr_auth_failed',
    'download_failed',
    'steam_auth_console_failed',
  ].includes(phase)
  const isFailureState = state === 'steam_auth_failed' || state === 'error' || errorPhase || !!failedJob
  if (!isFailureState || state === 'game_installed') return ''

  const lastErrorLog = failedJob && selectedJob?.id === failedJob.id
    ? [...logs].reverse().find((log) => log.level === 'error')?.message ?? ''
    : ''
  const rawText = [stateMessage, failedJob?.errorMessage ?? '', lastErrorLog].filter(Boolean).join(' ')
  const lower = rawText.toLowerCase()

  if (phase === 'install_timeout' || lower.includes('任务超时') || lower.includes('timed out')) {
    return '安装任务超时：Steam 认证或下载没有在限定时间内完成，请重试安装。'
  }
  if (
    phase === 'steam_auth_connection_failed' ||
    lower.includes('tryanothercm') ||
    lower.includes('steam client not connected') ||
    lower.includes('steamclient') ||
    lower.includes('cm')
  ) {
    return 'Steam CM 连接失败或超时：当前网络连接 Steam 会话不稳定，请稍后重试；如果一直失败，建议改用扫码登录或先在可用网络完成一次 refresh token。'
  }
  if (phase === 'credentials_required' || lower.includes('invalid password') || lower.includes('incorrect password')) {
    return 'Steam 账号或密码认证失败，请重新输入凭据后再试。'
  }
  if (phase === 'qr_auth_failed') {
    return 'Steam 二维码登录失败：当前 steam-auth 容器未能连接 SteamClient，请改用账号密码/验证码登录。'
  }
  if (phase === 'download_failed' || lower.includes('download failed')) {
    return '游戏文件下载失败：Steam 认证可能已经成功，但下载阶段失败，请检查网络、磁盘空间后重试。'
  }
  if (phase === 'pull_failed') {
    return 'Junimo 镜像拉取失败，请检查 Docker 网络或镜像地址后重试。'
  }

  if (stateMessage && !stateMessage.includes('正在') && !stateMessage.includes('请稍候')) return stateMessage
  if (lastErrorLog) return lastErrorLog.replace(/^\[steam\]\s*/, '')
  if (failedJob?.errorMessage) return failedJob.errorMessage
  return '安装任务失败，请查看任务中心日志后重试。'
}
