import type { Job, JobLog } from '../../types'

function roundPercent(value: number): number {
  return Math.min(100, Math.max(0, Math.round(value * 10) / 10))
}

export type DownloadProgress = {
  filesDone: number
  filesTotal: number
  percent: number
  bytesDone: string
  bytesTotal: string
  itemLabel?: string
}

export type SMAPIArchiveProgress = {
  downloadedBytes: number
  totalBytes: number
  percent: number
  candidate: number
  candidateCount: number
  cached: boolean
}

type DownloadProgressKind = 'game' | 'sdk' | 'steamcmd_update'

const steamDownloadProgressRe = /\[steam\].*Progress:\s*(\d+)\/(\d+)\s+files\s+-\s+([^/()]+?)\/([^()]+?)\s+\((\d+(?:\.\d+)?)%\)/i
const steamCMDDownloadProgressRe = /\[steamcmd\].*progress:\s*(\d+(?:\.\d+)?)\s*\(([^/()]+?)\s*\/\s*([^()]+?)\)/i
const steamCMDBracketProgressRe = /\[steamcmd\]\s+\[\s*(\d+(?:\.\d+)?)%\]\s+downloading update\s+\(([\d,]+)\s+of\s+([\d,]+)\s+([^)]+?)\)/i
const smapiArchiveProgressRe = /^\[smapi:download:progress:(\d+):(\d+):(\d+):(\d+):(true|false)\]$/i

export function extractSMAPIArchiveProgress(logs: JobLog[], jobType: string | undefined): SMAPIArchiveProgress | null {
  if (jobType !== 'stardew_install') return null
  let latest: SMAPIArchiveProgress | null = null
  for (const log of logs) {
    const match = log.message.match(smapiArchiveProgressRe)
    if (!match) continue
    const downloadedBytes = Number(match[1])
    const totalBytes = Number(match[2])
    const candidate = Number(match[3])
    const candidateCount = Number(match[4])
    if (
      !Number.isSafeInteger(downloadedBytes) ||
      !Number.isSafeInteger(totalBytes) ||
      !Number.isSafeInteger(candidate) ||
      !Number.isSafeInteger(candidateCount) ||
      downloadedBytes < 0 ||
      totalBytes <= 0 ||
      downloadedBytes > totalBytes ||
      candidate <= 0 ||
      candidateCount <= 0 ||
      candidate > candidateCount
    ) continue
    latest = {
      downloadedBytes,
      totalBytes,
      percent: roundPercent((downloadedBytes / totalBytes) * 100),
      candidate,
      candidateCount,
      cached: match[5].toLowerCase() === 'true',
    }
  }
  return latest
}

export function extractSteamDownloadProgress(logs: JobLog[], jobType: string | undefined, kind: DownloadProgressKind): DownloadProgress | null {
  if (jobType !== 'stardew_install') return null
  let currentKind: 'game' | 'sdk' | null = null
  let sawSdk = false
  let sawSteamCMDGameDone = false
  let sawSteamCMDLogin = false
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
    if (lower.includes('[steamcmd]') && lower.includes("success! app '413150' fully installed")) {
      sawSteamCMDGameDone = true
      if (kind === 'game') latest = completeSteamDownloadProgress(latest)
      currentKind = 'sdk'
      sawSdk = true
    } else if (lower.includes('[steamcmd]') && lower.includes("success! app '1007' fully installed")) {
      if (kind === 'sdk') latest = completeSteamDownloadProgress(latest)
    }
    if (lower.includes('[steamcmd]') && (
      lower.includes('logging in user') ||
      lower.includes('waiting for user info') ||
      lower.includes('logged in ok') ||
      lower.includes("success! app '413150' fully installed")
    )) {
      sawSteamCMDLogin = true
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
      continue
    }

    const steamCMDMatch = log.message.match(steamCMDDownloadProgressRe)
    if (steamCMDMatch) {
      const progressKind = currentKind ?? (sawSdk || sawSteamCMDGameDone ? 'sdk' : 'game')
      if (progressKind === kind) {
        const percent = roundPercent(parseFloat(steamCMDMatch[1]))
        latest = {
          filesDone: percent,
          filesTotal: 100,
          bytesDone: steamCMDMatch[2].trim(),
          bytesTotal: steamCMDMatch[3].trim(),
          percent,
          itemLabel: `SteamCMD ${percent}%`,
        }
      }
      continue
    }

    const steamCMDBracketMatch = log.message.match(steamCMDBracketProgressRe)
    if (steamCMDBracketMatch) {
      const progressKind: DownloadProgressKind = !sawSteamCMDLogin && !currentKind
        ? 'steamcmd_update'
        : currentKind ?? (sawSdk || sawSteamCMDGameDone ? 'sdk' : 'game')
      if (progressKind === kind) {
        const percent = roundPercent(parseFloat(steamCMDBracketMatch[1]))
        const unit = steamCMDBracketMatch[4].trim()
        latest = {
          filesDone: percent,
          filesTotal: 100,
          bytesDone: `${steamCMDBracketMatch[2]} ${unit}`,
          bytesTotal: `${steamCMDBracketMatch[3]} ${unit}`,
          percent,
          itemLabel: progressKind === 'steamcmd_update' ? `SteamCMD 客户端 ${percent}%` : `SteamCMD ${percent}%`,
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

function completeSteamDownloadProgress(progress: DownloadProgress | null): DownloadProgress {
  if (!progress) {
    return {
      filesDone: 1,
      filesTotal: 1,
      bytesDone: '完成',
      bytesTotal: '完成',
      percent: 100,
      itemLabel: '100%',
    }
  }
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
  steamCMDClientProgress: DownloadProgress | null = null,
  smapiArchiveProgress: SMAPIArchiveProgress | null = null,
) {
  if (
    phase !== 'game_downloading' &&
    phase !== 'steam_sdk_downloading' &&
    phase !== 'steamcmd_downloading' &&
    phase !== 'smapi_installing'
  ) return null
  if (phase === 'smapi_installing') {
    const archivePercent = smapiArchiveProgress?.percent ?? 0
    const archiveReady = archivePercent >= 100
    const label = smapiArchiveProgress?.cached
      ? '本地 SMAPI 安装包缓存校验通过，正在安装运行环境。'
      : archiveReady
        ? 'SMAPI 安装包下载完成，正在校验并写入游戏运行目录。'
        : smapiArchiveProgress
          ? `正在下载 SMAPI 安装包（${archivePercent}%，下载源 ${smapiArchiveProgress.candidate}/${smapiArchiveProgress.candidateCount}）。`
          : '正在检查 SMAPI 安装包缓存；如需下载，进度会在这里持续更新。'
    return {
      done: archiveReady ? 3 : 2,
      total: 3,
      percent: roundPercent(80 + archivePercent * 0.2),
      label,
    }
  }
  if (phase === 'steam_sdk_downloading') {
    return {
      done: sdkProgress?.percent === 100 ? 2 : 1,
      total: 3,
      percent: sdkProgress?.percent === 100 ? 80 : roundPercent(50 + (sdkProgress?.percent ?? 0) * 0.3),
      label: sdkProgress?.percent === 100
        ? '游戏文件和 Steam SDK 均已下载完成。'
        : '游戏文件已下载完成，正在下载 Steam SDK 运行文件。',
    }
  }
  if (phase === 'steamcmd_downloading') {
    const sdkPercent = sdkProgress?.percent
    const gamePercent = gameProgress?.percent ?? 0
    if (steamCMDClientProgress && sdkPercent == null && gameProgress == null) {
      return {
        done: 0,
        total: 3,
        percent: roundPercent(steamCMDClientProgress.percent * 0.15),
        label: 'SteamCMD 镜像已就绪，正在更新 SteamCMD 客户端；这不是 Docker 镜像拉取。',
      }
    }
    return {
      done: sdkPercent === 100 ? 2 : gamePercent >= 100 ? 1 : 0,
      total: 3,
      percent: sdkPercent != null ? roundPercent(50 + sdkPercent * 0.3) : roundPercent(gamePercent / 2),
      label: sdkPercent != null
        ? 'SteamCMD 已完成游戏文件，正在处理 Steam SDK 运行文件。'
        : 'SteamCMD 正在下载并校验 Stardew Valley 游戏文件。',
    }
  }
  return {
    done: gameProgress?.percent === 100 ? 1 : 0,
    total: 3,
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
    'credentials_required',
    'download_failed',
    'post_auth_failed',
    'steamcmd_failed',
    'steamcmd_image_pull_failed',
  ].includes(phase)
  const isFailureState = state === 'error' || errorPhase || !!failedJob
  if (!isFailureState || state === 'game_installed') return ''

  const lastErrorLog = failedJob && selectedJob?.id === failedJob.id
    ? [...logs].reverse().find((log) => log.level === 'error')?.message ?? ''
    : ''
  const rawText = [stateMessage, failedJob?.errorMessage ?? '', lastErrorLog].filter(Boolean).join(' ')
  const lower = rawText.toLowerCase()

  if (phase === 'install_timeout' || lower.includes('任务超时') || lower.includes('timed out')) {
    return '安装任务超时：SteamCMD 授权或下载没有在限定时间内完成，请重试安装。'
  }
  if (phase === 'credentials_required' || lower.includes('invalid password') || lower.includes('incorrect password')) {
    return 'Steam 账号或密码错误（SteamCMD 登录失败），请修改后再试。'
  }
  if (phase === 'download_failed' || lower.includes('download failed')) {
    return '游戏文件下载失败：SteamCMD 授权可能已经成功，但下载阶段失败，请检查网络、磁盘空间后重试。'
  }
  if (phase === 'steamcmd_failed') {
    return 'SteamCMD 安装或修复失败；请检查任务日志、Steam 授权、网络和磁盘空间后重试。'
  }
  if (phase === 'steamcmd_image_pull_failed') {
    return 'SteamCMD 工具镜像拉取失败，请检查 Docker 网络或镜像源后重试。'
  }
  if (phase === 'post_auth_failed') {
    return 'SteamCMD 授权已经成功，但后续安装步骤失败；请使用已保存凭据重试，不需要重新输入账号密码。'
  }
  if (phase === 'pull_failed') {
    return 'Junimo 镜像拉取失败，请检查 Docker 网络或镜像地址后重试。'
  }

  if (stateMessage && !stateMessage.includes('正在') && !stateMessage.includes('请稍候')) return stateMessage
  if (lastErrorLog) return lastErrorLog.replace(/^\[steam\]\s*/, '')
  if (failedJob?.errorMessage) return failedJob.errorMessage
  return '安装任务失败，请查看任务中心日志后重试。'
}
