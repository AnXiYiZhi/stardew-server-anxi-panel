import type { CreateInstanceRequest, Instance, InstanceState, Job, PublicIPResult } from '../types'
import { classifyInstallationState } from './stardew/installation-state.ts'

export const STARDEW_DRIVER_ID = 'stardew_junimo'

export type StardewCatalogItem = {
  instance: Instance
  state: InstanceState | null
  stateLoading: boolean
  stateError: string | null
  connection: PublicIPResult | null
  connectionLoading: boolean
  connectionError: string | null
  hasActiveInstallJob: boolean
  activeLifecycleJob: Job | null
}

export type StardewInstallCardState = {
  kind: 'loading' | 'unavailable' | 'installed' | 'not_installed' | 'installing' | 'install_failed' | 'repair_required' | 'diagnostic'
  label: string
  actionLabel: string
  actionDisabled: boolean
  progress: number | null
  tone: 'idle' | 'working' | 'ready' | 'error'
}

export type StardewGameDestination =
  | { kind: 'install'; instanceId: string }
  | { kind: 'worlds' }
  | { kind: 'unavailable' }

export type StardewInstanceDestination = 'install' | 'saves' | 'panel'

export type GameLibraryBackground = 'day' | 'night'

export type GameCardNavigationIntent = 'previous' | 'next' | 'first' | 'last'

export type StardewWorldLifecycleIntent = 'start' | 'stop' | null

export type StardewWorldLifecycleControl = {
  action: Exclude<StardewWorldLifecycleIntent, null> | null
  label: '启动' | '启动中…' | '停止' | '停止中…' | '需要存档' | '暂不可用'
  disabled: boolean
  busy: boolean
  tone: 'start' | 'stop' | 'busy' | 'disabled'
}

export function gameLibraryBackgroundPreference(value: string | null): GameLibraryBackground {
  return value === 'night' ? 'night' : 'day'
}

export function gameLibraryBackgroundForHour(hour: number): GameLibraryBackground {
  return hour >= 18 || hour < 6 ? 'night' : 'day'
}

export function gameLibraryBackgroundForDate(now: Date): GameLibraryBackground {
  return gameLibraryBackgroundForHour(now.getHours())
}

export function gameLibraryNextBackgroundBoundary(now: Date): number {
  const next = new Date(now)
  if (now.getHours() < 6) {
    next.setHours(6, 0, 0, 0)
  } else if (now.getHours() < 18) {
    next.setHours(18, 0, 0, 0)
  } else {
    next.setDate(next.getDate() + 1)
    next.setHours(6, 0, 0, 0)
  }
  return next.getTime()
}

export function gameLibraryManualBackgroundPreference(
  value: string | null,
  expiresAtValue: string | null,
  now: number,
): { background: GameLibraryBackground; expiresAt: number } | null {
  if (value !== 'day' && value !== 'night') return null
  const expiresAt = Number(expiresAtValue)
  if (!Number.isFinite(expiresAt) || expiresAt <= now) return null
  return { background: value, expiresAt }
}

export function gameCardNavigationIndex(
  currentIndex: number,
  enabledCards: readonly boolean[],
  intent: GameCardNavigationIntent,
): number {
  const enabledIndexes = enabledCards.flatMap((enabled, index) => enabled ? [index] : [])
  if (enabledIndexes.length === 0) return currentIndex
  if (intent === 'first') return enabledIndexes[0]
  if (intent === 'last') return enabledIndexes.at(-1) ?? enabledIndexes[0]

  const currentPosition = enabledIndexes.indexOf(currentIndex)
  if (currentPosition < 0) return enabledIndexes[0]
  const offset = intent === 'previous' ? -1 : 1
  const nextPosition = Math.max(0, Math.min(enabledIndexes.length - 1, currentPosition + offset))
  return enabledIndexes[nextPosition]
}

export function stardewCatalogItems(instances: Instance[]): Instance[] {
  return instances.filter((instance) => instance.driverId === STARDEW_DRIVER_ID)
}

export function initialStardewCatalogItem(instance: Instance): StardewCatalogItem {
  const state: InstanceState = {
    instanceId: instance.id,
    driverId: instance.driverId,
    name: instance.name,
    state: instance.state,
    stateMessage: instance.stateMessage,
    driverPhase: instance.driverPhase,
    updatedAt: instance.updatedAt,
    steamInviteEnabled: false,
  }
  const item: StardewCatalogItem = {
    instance,
    state,
    stateLoading: true,
    stateError: null,
    connection: null,
    connectionLoading: false,
    connectionError: null,
    hasActiveInstallJob: false,
    activeLifecycleJob: null,
  }
  item.connectionLoading = stardewShouldLoadConnection(item)
  return item
}

export function stardewRequiresSave(item: StardewCatalogItem): boolean {
  return (item.state?.state ?? item.instance.state) === 'save_required'
}

export function stardewShouldLoadConnection(item: StardewCatalogItem): boolean {
  return !stardewRequiresSave(item)
    && classifyInstallationState(item.state, item.hasActiveInstallJob).isInstalled
}

export function stardewGameDestination(
  items: StardewCatalogItem[],
  defaultInstanceId: string,
): StardewGameDestination {
  if (items.length === 0) return { kind: 'unavailable' }
  const installing = items.find((item) => classifyInstallationState(item.state, item.hasActiveInstallJob).kind === 'installing')
  if (installing) return { kind: 'install', instanceId: installing.instance.id }
  if (items.some((item) => classifyInstallationState(item.state, item.hasActiveInstallJob).isInstalled)) {
    return { kind: 'worlds' }
  }

  const preferred = items.find((item) => item.instance.id === defaultInstanceId) ?? items[0]
  const installation = classifyInstallationState(preferred.state, preferred.hasActiveInstallJob)
  if (
    installation.kind === 'not_installed'
    || installation.kind === 'install_failed'
    || installation.kind === 'repair_required'
    || installation.kind === 'installing'
  ) {
    return { kind: 'install', instanceId: preferred.instance.id }
  }
  return { kind: 'worlds' }
}

export function stardewInstanceDestination(item: StardewCatalogItem): StardewInstanceDestination {
  if (stardewRequiresSave(item)) return 'saves'
  const installation = classifyInstallationState(item.state, item.hasActiveInstallJob)
  return installation.kind === 'not_installed'
    || installation.kind === 'install_failed'
    || installation.kind === 'repair_required'
    || installation.kind === 'installing'
    ? 'install'
    : 'panel'
}

export function stardewInstallCardState(
  items: StardewCatalogItem[],
  defaultInstanceId: string,
  loading: boolean,
  error: string | null,
): StardewInstallCardState {
  if (loading) {
    return { kind: 'loading', label: '正在读取安装状态', actionLabel: '读取中…', actionDisabled: true, progress: null, tone: 'working' }
  }
  if (error || items.length === 0) {
    return { kind: 'unavailable', label: '安装状态暂时不可用', actionLabel: '当前不可用', actionDisabled: true, progress: null, tone: 'error' }
  }

  const installing = items.find((item) => classifyInstallationState(item.state, item.hasActiveInstallJob).kind === 'installing')
  if (installing) {
    return { kind: 'installing', label: '安装中 · 实时进度可查看', actionLabel: '查看安装进度', actionDisabled: false, progress: null, tone: 'working' }
  }
  if (items.some((item) => classifyInstallationState(item.state, item.hasActiveInstallJob).isInstalled)) {
    return {
      kind: 'installed',
      label: `已安装 · ${items.length} 个世界`,
      actionLabel: '选择世界',
      actionDisabled: false,
      progress: 100,
      tone: 'ready',
    }
  }
  const preferred = items.find((item) => item.instance.id === defaultInstanceId) ?? items[0]
  const installation = classifyInstallationState(preferred.state, preferred.hasActiveInstallJob)
  if (installation.kind === 'install_failed') {
    return { kind: 'install_failed', label: '安装失败', actionLabel: '处理安装问题', actionDisabled: false, progress: null, tone: 'error' }
  }
  if (installation.kind === 'repair_required') {
    return { kind: 'repair_required', label: '需要修复', actionLabel: '修复安装', actionDisabled: false, progress: null, tone: 'error' }
  }
  if (installation.kind === 'not_installed') {
    return { kind: 'not_installed', label: '未安装', actionLabel: '开始安装', actionDisabled: false, progress: 0, tone: 'idle' }
  }
  return { kind: 'diagnostic', label: '状态需要诊断', actionLabel: '查看世界', actionDisabled: false, progress: null, tone: 'error' }
}

export function stardewGameCardAriaLabel(state: StardewInstallCardState): string {
  if (state.kind === 'installed') return `星露谷物语，${state.label}，点击选择世界`
  if (state.kind === 'not_installed') return '星露谷物语，未安装，点击开始安装'
  if (state.kind === 'installing') return '星露谷物语，安装中，点击查看真实安装进度'
  if (state.kind === 'install_failed') return '星露谷物语，安装失败，点击处理安装问题'
  if (state.kind === 'repair_required') return '星露谷物语，需要修复，点击进入安装修复流程'
  return `星露谷物语，${state.label}`
}

export function stardewJoinAddressValue(item: StardewCatalogItem, panelAccessHost: string): string | null {
  if (stardewRequiresSave(item)) return null
  if (!classifyInstallationState(item.state, item.hasActiveInstallJob).isInstalled) return null
  if (item.connectionLoading) return null
  const ip = panelAccessHost.trim()
  const port = item.connection?.gamePort ?? 0
  if (!ip || !Number.isInteger(port) || port < 1 || port > 65535) return null
  const host = ip.includes(':') && !ip.startsWith('[') ? `[${ip}]` : ip
  return `${host}:${port}`
}

export function stardewJoinAddress(item: StardewCatalogItem, panelAccessHost: string): string {
  if (stardewRequiresSave(item)) return '创建或上传存档后提供'
  if (!classifyInstallationState(item.state, item.hasActiveInstallJob).isInstalled) return '安装完成后提供'
  if (item.connectionLoading) return '正在读取加入地址…'
  return stardewJoinAddressValue(item, panelAccessHost) ?? '暂时无法读取'
}

export function stardewWorldLifecycleControl(
  item: StardewCatalogItem,
  pending: StardewWorldLifecycleIntent,
  role: 'admin' | 'user',
): StardewWorldLifecycleControl {
  const state = item.state?.state ?? item.instance.state
  const uiStatus = item.state?.uiStatus
  const lifecycleOperation = item.activeLifecycleJob?.operation
  const activeLifecycleIsStopping = Boolean(item.activeLifecycleJob && (
    lifecycleOperation === 'stop'
    || lifecycleOperation === 'new_game_rollback'
    || item.state?.driverPhase === 'stopping'
    || uiStatus === 'stopping'
    || ((lifecycleOperation === 'restart' || lifecycleOperation === 'restore_restart') && state === 'running')
    || (lifecycleOperation == null && item.activeLifecycleJob?.createdBy === null && state === 'running' && uiStatus === 'ready')
  ))

  if (pending === 'stop' || state === 'stopping' || uiStatus === 'stopping' || activeLifecycleIsStopping) {
    return { action: null, label: '停止中…', disabled: true, busy: true, tone: 'busy' }
  }
  if (
    pending === 'start'
    || state === 'starting'
    || uiStatus === 'starting_container'
    || uiStatus === 'loading_save'
    || uiStatus === 'waiting_for_host'
    || Boolean(item.activeLifecycleJob)
  ) {
    return { action: null, label: '启动中…', disabled: true, busy: true, tone: 'busy' }
  }
  if (state === 'running' || uiStatus === 'ready') {
    return { action: 'stop', label: '停止', disabled: role !== 'admin', busy: false, tone: 'stop' }
  }
  if (stardewRequiresSave(item)) {
    return { action: null, label: '需要存档', disabled: true, busy: false, tone: 'disabled' }
  }

  const installation = classifyInstallationState(item.state, item.hasActiveInstallJob)
  const startable = state === 'stopped' || state === 'ready_to_start' || state === 'game_installed'
  if (installation.isInstalled && startable) {
    return { action: 'start', label: '启动', disabled: role !== 'admin', busy: false, tone: 'start' }
  }
  return { action: null, label: '暂不可用', disabled: true, busy: false, tone: 'disabled' }
}

export function canCreateWorld(role: 'admin' | 'user'): boolean {
  return role === 'admin'
}

export function stardewCreateInstanceRequest(name: string): CreateInstanceRequest {
  return { name: name.trim(), gameId: 'stardew' }
}
