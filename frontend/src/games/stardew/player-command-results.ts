import { getCommandOutcome } from '../../api.ts'
import type { CommandOutcome, CommandRunResult } from '../../types'

export type PrecisePlayerCommand =
  | 'warp-home'
  | 'kick'
  | 'approve-auth'
  | 'broadcast'
  | 'ban'
  | 'trigger-event'
  | 'enable-joja'
  | 'save-now'

export type PlayerCommandFeedback = {
  kind: 'processing' | 'succeeded' | 'dispatched' | 'failed' | 'unconfirmed' | 'legacy'
  message: string
}

const POLL_INTERVAL_MS = 500
const POLL_TIMEOUT_MS = 10_000

const ERROR_MESSAGES: Record<string, string> = {
  world_not_ready: '游戏世界尚未准备完成，请稍后再试。',
  bridge_unavailable: '控制模组桥接能力不可用，请升级控制模组并重启服务器。',
  invalid_player_id: '玩家联机 ID 无效，无法执行操作。',
  player_not_online: '玩家已经离线，操作未执行。',
  host_not_supported: '不能对服务器主机执行此操作。',
  warp_failed: '游戏未能将玩家传送回家。',
  kick_failed: '游戏未能踢出该玩家。',
  already_authenticated: '该玩家已经完成认证，无需再次批准。',
  authentication_rejected: '服务器拒绝了本次认证批准。',
  authentication_failed: '认证服务执行异常，未能确认批准结果。',
  empty_message: '喊话内容为空，消息未发送。',
  chat_unavailable: '游戏聊天系统当前不可用。',
  broadcast_failed: '游戏聊天系统未能接受这条消息。',
  player_not_found: '未找到要封禁的玩家。',
  ambiguous_player: '存在重名玩家，无法安全确认封禁目标。',
  admin_promotion_failed: '提升 JunimoServer 管理员权限失败，未执行封禁。',
  command_dispatch_failed: '无法将封禁指令交给 JunimoServer。',
  ban_failed: '游戏服务器未能封禁该玩家。',
  no_festival_today: '今天没有可触发的节日活动。',
  festival_not_active: '主机当前不在节日现场，无法启动节日主活动。',
  save_already_pending: '已有一个游戏内保存请求正在等待完成。',
  save_timeout: '游戏内保存请求已超时，未能确认保存完成。',
}

function successMessage(command: PrecisePlayerCommand, playerName: string): string {
  if (command === 'warp-home') return `已将 ${playerName} 传送回家。`
  if (command === 'kick') return `已踢出玩家 ${playerName}。`
  if (command === 'approve-auth') return `已批准玩家 ${playerName} 的认证。`
  if (command === 'ban') return `已封禁 ${playerName}。`
  if (command === 'trigger-event') return '已确认节日主活动启动。'
  if (command === 'enable-joja') return '已确认存档中的社区中心路线已永久禁用。'
  if (command === 'save-now') return '已确认游戏内保存完成。'
  return '消息已交给游戏聊天系统；这表示发送调用成功，不保证每个客户端都已实际收到。'
}

function failedMessage(command: PrecisePlayerCommand, errorCode: string): string {
  if (errorCode === 'admin_promotion_failed' && command === 'enable-joja') {
    return '主机的 JunimoServer 管理员权限提升失败，Joja 指令未派发。'
  }
  if (errorCode === 'command_dispatch_failed') {
    if (command === 'trigger-event') return '无法将节日指令发送给 JunimoServer。'
    if (command === 'enable-joja') return '无法将 Joja 指令发送给 JunimoServer。'
  }
  return ERROR_MESSAGES[errorCode] ?? '命令执行未成功，请查看服务器日志。'
}

export function classifyPlayerCommandOutcome(
  outcome: CommandOutcome,
  command: PrecisePlayerCommand,
  playerName: string,
): PlayerCommandFeedback {
  if (outcome.status === 'queued' || outcome.status === 'running') {
    return { kind: 'processing', message: '处理中…' }
  }
  if (outcome.status === 'succeeded') {
    return { kind: 'succeeded', message: successMessage(command, playerName) }
  }
  if (outcome.status === 'dispatched') {
    if (command === 'ban') {
      return { kind: 'dispatched', message: '封禁指令已发送给 JunimoServer，最终结果请结合游戏状态确认。' }
    }
    return { kind: 'dispatched', message: '指令已发送，等待游戏处理或需结合游戏状态确认。' }
  }
  if (outcome.status === 'failed') {
    return {
      kind: 'failed',
      message: failedMessage(command, outcome.errorCode ?? ''),
    }
  }
  return { kind: 'unconfirmed', message: '无法确认最终结果，请先检查当前游戏状态再决定是否重试。' }
}

export function hasCommandResultCapability(submission: CommandRunResult): boolean {
  return Boolean(submission.commandId && submission.status === 'queued')
}

export async function submitAndWaitForPlayerCommand(
  submit: () => Promise<CommandRunResult>,
  command: PrecisePlayerCommand,
  playerName: string,
  onFeedback: (feedback: PlayerCommandFeedback) => void,
  timeoutMs = POLL_TIMEOUT_MS,
): Promise<PlayerCommandFeedback> {
  const submission = await submit()
  if (!hasCommandResultCapability(submission)) {
    const legacy = { kind: 'legacy', message: submission.output?.trim() || '指令已提交。' } as const
    onFeedback(legacy)
    return legacy
  }

  onFeedback({ kind: 'processing', message: '处理中…' })
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    await new Promise((resolve) => setTimeout(resolve, POLL_INTERVAL_MS))
    let outcome: CommandOutcome
    try {
      outcome = await getCommandOutcome(submission.commandId!)
    } catch {
      const unconfirmed = { kind: 'unconfirmed', message: '无法确认最终结果，请先检查当前游戏状态再决定是否重试。' } as const
      onFeedback(unconfirmed)
      return unconfirmed
    }
    const feedback = classifyPlayerCommandOutcome(outcome, command, playerName)
    onFeedback(feedback)
    if (feedback.kind !== 'processing') return feedback
  }

  const timeout = { kind: 'unconfirmed', message: '无法确认最终结果，请先检查当前游戏状态再决定是否重试。' } as const
  onFeedback(timeout)
  return timeout
}
