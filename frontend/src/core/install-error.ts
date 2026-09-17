import catalog from '../../../backend/internal/games/installerrors/catalog.json' with { type: 'json' }
import type { Job, JobLog } from '../types.ts'

const rules = catalog.rules.map(rule => ({ ...rule, matcher: new RegExp(rule.pattern, 'i') }))
const phases: Record<string, string> = catalog.phases
const exits: Record<string, string> = catalog.exits
const knownMessages = new Set([...rules.map(rule => rule.message), ...Object.values(phases), ...Object.values(exits), catalog.unknown])
const exitPattern = /(?:exit(?:ed)?(?: with)?(?: code| status)?|exitcode|退出码|\bcode)\s*[:=]?\s*([1-9][0-9]*)\b/i

export function matchInstallationError(text: string): string | undefined {
  return rules.find(rule => rule.matcher.test(text))?.message
}

export function installationRequestErrorMessage(text: string | null): string {
  const raw = text?.trim() ?? ''
  if (/failed to fetch|network request failed|networkerror|load failed/i.test(raw)) {
    return '无法连接面板服务，请检查网络并确认面板仍在运行后重试。'
  }
  return matchInstallationError(raw) ?? (meaningfulMessage(raw) || knownMessages.has(raw)
    ? raw : '安装请求未能完成，当前信息不足以确定原因，请检查面板连接或任务日志后重试。')
}

function meaningfulMessage(text: string): boolean {
  return /[\u3400-\u9fff]/u.test(text) && !/正在|请稍候|等待输入|需要登录授权|已授权|已完成|退出码|容器诊断|install:diagnostic|^任务超时[。.]?$|：(?:操作失败|未知错误)$/.test(text)
}

// Old jobs have raw English results. Only the selected job's current command
// attempt contributes log evidence; recovered failures must not become diagnoses.
function logDiagnosis(job: Job, logs: JobLog[]): string | undefined {
  let found: (typeof rules)[number] | undefined
  let gameDownloaded = false
  for (const log of logs.filter(log => log.jobId === job.id).sort((a, b) => a.sequence - b.sequence)) {
    const line = log.message
    if (/正在自动切换为账号密码完整登录|retrying once|\[smapi\].*(?:预安装|写入游戏运行目录)|\[steamcmd\] 使用 SteamCMD 镜像|\[steam\].*(?:attempt|重试)/i.test(line)) { found = undefined; gameDownloaded = false }
    if (/success! app '413150' fully installed/i.test(line)) { found = undefined; gameDownloaded = true }
    if (gameDownloaded && /success! app '1007' fully installed/i.test(line)) found = undefined
    if (/logged in ok|waiting for user info\.\.\.\s*ok|waiting for confirmation\.\.\.\s*ok/i.test(line)
      && (found?.group === 'auth' || found?.group === 'network')) found = undefined
    const next = rules.find(rule => rule.matcher.test(line))
    if (next && (!['steam_app_state', 'missing_success'].includes(next.id) || !found)) found = next
  }
  return found?.message
}

export function installationFailureMessage(options: {
  phase?: string
  stateMessage?: string | null
  job?: Job | null
  logs?: JobLog[]
  error?: string | null
  requiredFilesMissing?: boolean
}): string {
  const { phase = '', stateMessage = '', job, logs = [], requiredFilesMissing = false } = options
  if (requiredFilesMissing) return phases.install_verification_failed
  if (job?.status === 'canceled') return '任务已取消，可在准备好后重新安装或授权。'
  const raw = (options.error ?? job?.errorMessage ?? '').trim()
  // New results are authoritative and already safe; don't replace them with old
  // log lines from an earlier retry or a restored base-installation state.
  if (knownMessages.has(raw)) return raw
  const direct = rules.find(rule => rule.matcher.test(raw))
  if (direct && !['steam_app_state', 'missing_success'].includes(direct.id)) return direct.message
  if (job?.status === 'failed' && ['stardew_install', 'stardew_steam_auth'].includes(job.type)) {
    const diagnosis = logDiagnosis(job, logs)
    if (diagnosis) return diagnosis
  }
  if (direct) return direct.message
  if (meaningfulMessage(raw)) return raw
  // Invite-only jobs restore the base state; it cannot explain their failure.
  if (job?.type !== 'stardew_steam_auth' && stateMessage) {
    if (knownMessages.has(stateMessage)) return stateMessage
    if (meaningfulMessage(stateMessage)) return matchInstallationError(stateMessage) ?? stateMessage
  }
  const exit = raw.match(exitPattern)?.[1]
  if (exit) return exits[exit] ?? `安装进程异常退出（退出码 ${exit}），请展开本次任务日志查看失败前的原因后重试。`
  if (/context canceled|operation cancelle?d/i.test(raw)) return '任务已取消，可在准备好后重新安装或授权。'
  if (/timed?\s*out|timeout|context deadline exceeded|任务超时/i.test(raw)) return phases.install_timeout
  return phases[phase] ?? catalog.unknown
}
