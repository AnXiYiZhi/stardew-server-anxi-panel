import type { SMAPIUpdateInfo, SMAPIUpdatePhase, SMAPIUpdateStatus } from '../../types'

export function smapiStatusLabel(status?: SMAPIUpdateStatus): string {
  const labels: Record<SMAPIUpdateStatus, string> = {
    up_to_date: 'SMAPI 已匹配推荐版本',
    update_available: '有经过验证的 SMAPI 更新',
    missing: '实际游戏目录未安装 SMAPI',
    invalid: 'SMAPI 安装产物无效',
    incompatible_game: '游戏或 Steamworks SDK 前置不兼容',
    incompatible_junimo: 'Junimo 或 steam-auth-cn 前置不兼容',
    custom_or_unknown: '自定义或未知 SMAPI 构建',
  }
  return status ? labels[status] : 'SMAPI 状态未知'
}

export function smapiPhaseLabel(phase?: SMAPIUpdatePhase): string {
  const labels: Record<SMAPIUpdatePhase, string> = {
    idle: '尚未执行', checking: '检查兼容矩阵', downloading: '下载可信安装器', validating_archive: '校验 SHA256 与 ZIP',
    creating_staging: '创建 staging volume', cloning: '复制当前 game-data', installing: '运行官方 Linux 安装器',
    verifying_staging: '验证 staging 版本', stopping: '安全停服', switching: '原子切换 GAME_DATA_VOLUME',
    starting: '启动完整推荐 stack', verifying_stack: '验证 SMAPI、Mod、Junimo 与认证链路',
    restoring_state: '恢复原运行状态', succeeded: '升级成功', rolling_back: '正在切回旧 volume',
    failed_rolled_back: '升级失败，已自动回滚', rollback_failed: '自动回滚失败，需人工处理', failed: '预检失败',
  }
  return labels[phase ?? 'idle']
}

export function smapiPhaseActive(phase?: SMAPIUpdatePhase): boolean {
  return !!phase && !['idle', 'succeeded', 'failed', 'failed_rolled_back', 'rollback_failed'].includes(phase)
}

export function shouldShowSMAPIUpdate(info?: SMAPIUpdateInfo | null): boolean {
  return info?.available === true && info.supported === true && info.status === 'update_available'
}
