import type { JunimoUpdateApplyPhase, JunimoUpdateDryRunPhase, JunimoUpdateStatus } from '../../types'

export function junimoUpdateStatusLabel(status?: JunimoUpdateStatus): string {
  switch (status) {
    case 'up_to_date': return '推荐版本对已匹配'
    case 'update_available': return 'Junimo 运行组件可更新'
    case 'not_installed': return 'Junimo 运行组件尚未安装'
    case 'custom_images': return '自定义镜像不支持自动判断'
    case 'invalid_config': return '运行组件配置无法判断'
		case 'withdrawn': return '当前兼容矩阵已撤回，禁止新升级'
		case 'not_recommended': return '当前矩阵尚未正式推荐'
    default: return '版本状态未知'
  }
}

export function junimoDryRunPhaseLabel(phase?: JunimoUpdateDryRunPhase): string {
  switch (phase) {
    case 'idle': return '尚未运行预检'
    case 'starting': return '正在启动预检'
    case 'checking': return '正在检查实例与 Docker'
    case 'pulling_server': return '正在检查或拉取 server 镜像'
    case 'pulling_auth': return '正在检查或拉取 steam-auth-cn 镜像'
    case 'validating_compose': return '正在验证 Compose 配置'
    case 'succeeded': return '升级预检通过'
    case 'failed': return '升级预检失败'
    case 'unsupported': return '当前实例不支持升级预检'
    default: return '预检状态未知'
  }
}

export function junimoDryRunActive(phase?: JunimoUpdateDryRunPhase): boolean {
  return phase === 'starting' || phase === 'checking' || phase === 'pulling_server' || phase === 'pulling_auth' || phase === 'validating_compose'
}

export function junimoPairMatches(status?: JunimoUpdateStatus): boolean {
  return status === 'up_to_date'
}

export function junimoApplyActive(phase?: JunimoUpdateApplyPhase): boolean {
  return !!phase && !['idle', 'succeeded', 'failed_rolled_back', 'rollback_failed'].includes(phase)
}

export function junimoMaintenanceNeedsAttention(
  status?: JunimoUpdateStatus,
  available = false,
  applyPhase?: JunimoUpdateApplyPhase,
  loadFailed = false,
): boolean {
  if (loadFailed || applyPhase === 'rollback_failed' || junimoApplyActive(applyPhase)) return true
  if (available || status === 'update_available') return true
  return !!status && !['up_to_date', 'not_installed'].includes(status)
}

export function junimoApplyPhaseLabel(phase?: JunimoUpdateApplyPhase): string {
  const labels: Record<JunimoUpdateApplyPhase, string> = {
    idle: '尚未执行升级', checking: '重新预检', pulling: '拉取版本对', backing_up: '保护认证卷', stopping: '安全停服',
    writing_config: '原子写入配置', recreating_auth: '重建 steam-auth-cn', verifying_auth: '验证 Steam 登录',
    recreating_server: '重建 Junimo server', verifying_server: '验证运行链路', restoring_state: '恢复原运行状态',
    succeeded: '升级成功', rolling_back: '正在成对回滚', failed_rolled_back: '升级失败，已成功回滚', rollback_failed: '自动回滚失败，需人工处理',
  }
  return labels[phase ?? 'idle']
}
