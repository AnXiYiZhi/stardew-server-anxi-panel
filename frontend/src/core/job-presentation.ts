import type { Job } from '../types.ts'
import { installationFailureMessage, matchInstallationError } from './install-error.ts'

const JOB_NAMES: Record<string, string> = {
  stardew_install: '安装游戏', stardew_prepare: '准备游戏环境',
  stardew_start: '启动服务器', stardew_stop: '停止服务器', stardew_restart: '重启服务器',
  stardew_lifecycle: '服务器操作', stardew_custom_new_game: '新建存档',
  stardew_select_save_and_start: '选择存档并启动', stardew_upload_save_and_start: '导入存档并启动',
  stardew_farmhand_delete: '删除存档人物', stardew_steam_auth: '游戏账号授权',
  stardew_backup: '备份存档', stardew_restore: '恢复存档',
  mod_remote_install: '安装远程模组', mod_nexus_install: '安装社区模组',
  test: '测试任务', test_fail: '失败测试',
}

const OPERATIONS: Record<string, string> = {
  start: '启动服务器', stop: '停止服务器', restart: '重启服务器',
  restore_restart: '恢复存档并重启', new_game_rollback: '恢复新建前状态',
}

export function jobStatusLabel(status: string): string {
  return ({ queued: '排队中', running: '进行中', succeeded: '已完成', failed: '失败', canceled: '已取消' } as Record<string, string>)[status] ?? '状态未知'
}

export function localizedJobName(job: Pick<Job, 'type' | 'operation' | 'displayName'>): string {
  const fallback = (job.type === 'stardew_lifecycle' && job.operation ? OPERATIONS[job.operation] : undefined) ?? JOB_NAMES[job.type] ?? '后台任务'
  const displayName = job.displayName?.trim()
  if (!displayName || displayName === job.type) return fallback
  // Preserve mod/user names while translating the machine task suffix.
  return displayName.replace(/\b[a-z][a-z0-9]*_[a-z0-9_]+\b/g, (token) => token === job.type ? fallback : JOB_NAMES[token] ?? token)
}

export function jobEventLabel(job: Pick<Job, 'type' | 'operation' | 'displayName' | 'status'>): string {
  const name = localizedJobName(job)
  if (job.status === 'succeeded') {
    const complete: Record<string, string> = {
      '启动服务器': '服务器已启动', '停止服务器': '服务器已停止', '重启服务器': '服务器已重启',
      '安装游戏': '游戏安装完成', '新建存档': '存档已创建', '服务器操作': '服务器操作已完成',
    }
    return complete[name] ?? `${name} · 已完成`
  }
  if (job.status === 'failed') return name === '安装游戏' ? '游戏安装失败' : `${name}失败`
  return `${name} · ${jobStatusLabel(job.status)}`
}

export function jobErrorSummary(message: string | null | undefined): string {
  const raw = message?.trim()
  if (!raw) return '任务未能完成，请查看任务日志。'
  if (/steamcmd|steam.auth|smapi|容器.*退出码/i.test(raw)) return installationFailureMessage({ error: raw })
  const installationError = matchInstallationError(raw)
  if (installationError) return installationError
  const rules: [RegExp, string][] = [
    [/SteamCMD install finished without success marker/i, '游戏下载流程已结束，但未确认安装成功，请检查下载日志后重试。'],
    [/steam.*(?:auth|login).*timed?\s*out|steam.*authorization.*timeout/i, '游戏账号授权超时，请重新授权并及时完成验证。'],
    [/context deadline exceeded|timed?\s*out|timeout/i, '操作等待超时，请检查连接和服务器状态后重试。'],
    [/context canceled|operation cancelle?d/i, '操作已取消。'],
    [/port is already allocated|address already in use|ports are not available/i, '端口已被占用，请更换端口或检查占用服务。'],
    [/permission denied|access is denied/i, '访问被拒绝，请检查目录或容器的权限。'],
    [/no space left on device/i, '存储空间不足，请清理空间后重试。'],
    [/connection refused|failed to fetch|network error/i, '连接失败，请检查网络与服务状态后重试。'],
    [/no such file or directory|file not found/i, '所需文件或目录不存在，请检查安装文件与存档。'],
  ]
  for (const [pattern, translated] of rules) if (pattern.test(raw)) return translated
  if (/[\u3400-\u9fff]/u.test(raw)) return raw
  return '任务执行失败，请查看任务日志中的原始诊断信息。'
}

export function shortEventTime(value: string): string {
  const date = new Date(value)
  if (!Number.isFinite(date.getTime())) return '时间未知'
  return `${date.getMonth() + 1}/${date.getDate()} ${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`
}
