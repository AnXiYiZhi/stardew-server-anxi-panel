import type { RuntimeComponentsInfo } from '../../types'

export function runtimeComponentsStatusLabel(status?: RuntimeComponentsInfo['status']): string {
  return ({
    up_to_date: '已匹配推荐版本',
    update_available: '游戏运行文件可更新',
    game_missing: '游戏文件缺失',
    sdk_missing: '联机运行库缺失',
    manifest_invalid: '版本清单损坏',
    custom_or_unknown: '自定义或未知状态',
  } as Record<string, string>)[status ?? ''] ?? '尚未检测'
}

export function shouldShowRuntimeComponentsUpdate(info?: RuntimeComponentsInfo | null): boolean {
  return info?.recommended.tested === true && info.status === 'update_available'
}
