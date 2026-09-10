import type { FarmhandDeleteIntent, FarmhandDeleteIntentStatus, FarmhandDeleteMode } from '../../types'

export function farmhandDeleteIntentActive(intent: FarmhandDeleteIntent | null): boolean {
  return intent?.status === 'waiting' || intent?.status === 'launching' || intent?.status === 'countdown' || intent?.status === 'active' || intent?.status === 'recovery_required'
}

export function farmhandDeleteIntentCancelable(intent: FarmhandDeleteIntent | null): boolean {
  return intent?.status === 'waiting' || intent?.status === 'countdown'
}

export function farmhandDeleteIntentNeedsRecovery(intent: FarmhandDeleteIntent | null): boolean {
  return intent?.status === 'recovery_required'
}

export function farmhandDeleteConfirmationReady(
  mode: FarmhandDeleteMode,
  riskAcknowledged: boolean,
  confirmationName: string,
  expectedName: string,
): boolean {
  return mode === 'wait' || (riskAcknowledged && confirmationName.trim() === expectedName.trim())
}

export function farmhandDeleteStatusLabel(status: FarmhandDeleteIntentStatus): string {
  switch (status) {
    case 'waiting': return '等待无人在线'
    case 'launching': return '正在准备维护'
    case 'countdown': return '维护倒计时中'
    case 'active': return '正在维护并删除'
    case 'recovery_required': return '需要恢复处理'
    case 'completed': return '删除完成'
    case 'canceled': return '操作已取消'
    case 'expired': return '等待已过期'
    case 'failed': return '操作失败'
  }
}

export function farmhandDeleteExpiryText(intent: FarmhandDeleteIntent | null, now = Date.now()): string {
  if (!intent || intent.status !== 'waiting') return ''
  const expiresAt = Date.parse(intent.expiresAt)
  if (!Number.isFinite(expiresAt)) return '等待期限未知'
  const minutes = Math.max(0, Math.ceil((expiresAt - now) / 60000))
  if (minutes >= 60) return `剩余约 ${Math.ceil(minutes / 60)} 小时`
  return `剩余约 ${minutes} 分钟`
}
