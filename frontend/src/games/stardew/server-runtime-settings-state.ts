import type { ServerRuntimeSettings } from '../../types'

export const DEFAULT_SERVER_RUNTIME_SETTINGS: ServerRuntimeSettings = {
  maxPlayers: 10,
  cabinStrategy: 'None',
  existingCabinBehavior: 'KeepExisting',
  networkBroadcastPeriod: 1,
}

export function normalizeServerRuntimeSettings(
  settings: Partial<ServerRuntimeSettings> | null | undefined,
): ServerRuntimeSettings {
  return {
    maxPlayers: Number.isInteger(settings?.maxPlayers) ? Number(settings?.maxPlayers) : 10,
    cabinStrategy: settings?.cabinStrategy || 'None',
    existingCabinBehavior: settings?.existingCabinBehavior || 'KeepExisting',
    networkBroadcastPeriod: Number.isInteger(settings?.networkBroadcastPeriod)
      ? Number(settings?.networkBroadcastPeriod)
      : 1,
  }
}

export function maxPlayersValidationError(maxPlayers: number): string | null {
  if (!Number.isInteger(maxPlayers) || maxPlayers < 1 || maxPlayers > 100) {
    return '最大同时在线人数必须是 1~100 之间的整数。'
  }
  return null
}

export function runtimeSettingsEffectText({
  isRunning,
  currentMaxPlayers,
  configuredMaxPlayers,
}: {
  isRunning: boolean
  currentMaxPlayers: number | null
  configuredMaxPlayers: number
}): string {
  if (!isRunning) {
    return `服务器已停止；配置上限 ${configuredMaxPlayers} 人将在下次启动时生效。`
  }
  if (currentMaxPlayers == null) {
    return `当前生效上限暂时无法从 Junimo 读取；重启后配置为 ${configuredMaxPlayers} 人。`
  }
  if (currentMaxPlayers !== configuredMaxPlayers) {
    return `当前生效上限 ${currentMaxPlayers} 人；重启后配置 ${configuredMaxPlayers} 人，存在待重启变更。`
  }
  return `当前生效上限与重启后配置均为 ${configuredMaxPlayers} 人。`
}

export function runtimeSettingsOnlineWarning(
  configuredMaxPlayers: number,
  onlineCount: number | null,
): string | null {
  if (onlineCount == null || configuredMaxPlayers >= onlineCount) return null
  return `目标上限低于当前在线 ${onlineCount} 人；仍可仅保存，重启后才会生效。`
}

export function shouldShowPlayerLimitAction(canEditPlayerLimit: boolean, hasCallback: boolean): boolean {
  return canEditPlayerLimit && hasCallback
}
