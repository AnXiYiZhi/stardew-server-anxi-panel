import type { PlayerModComparisonItem, PlayerModDetailsResult, StardewPlayerInfo } from '../../types'

export const PLAYER_MOD_UNAVAILABLE_COPY =
  '该客户端未上报 Mod 清单，常见于原版 PC、手机、平板或不兼容的 SMAPI；这不代表客户端一定没有 Mod。'
export const PLAYER_CJB_DETECTED_LABEL = '检测到 CJB 作弊'
export const PLAYER_CJB_BANNER_TITLE = '检测到该玩家使用了 CJB 作弊工具'

const CJB_UNIQUE_IDS = new Set(['cjbok.cheatsmenu', 'cjbok.itemspawner'])
const IGNORED_COMPARISON_UNIQUE_IDS = new Set([
  'pathoschild.smapi',
  'junimohost.server',
  'anxiyizhi.stardewanxipanel.control',
])

const RESULT_PRIORITY: Record<string, number> = {
  version_mismatch: 4,
  missing_on_client: 3,
  client_only: 2,
  match: 1,
}

export type PlayerModViewKind =
  | 'request_error'
  | 'pending'
  | 'unavailable'
  | 'stale'
  | 'comparison_unavailable'
  | 'available'

export type PlayerModViewState = {
  kind: PlayerModViewKind
  showComparison: boolean
  message: string
}

export type PlayerModGroups = {
  match: PlayerModComparisonItem[]
  missingOnClient: PlayerModComparisonItem[]
  versionMismatch: PlayerModComparisonItem[]
  clientOnly: PlayerModComparisonItem[]
}

function unavailableComparisonMessage(details: PlayerModDetailsResult): string {
  if (details.message) return details.message
  switch (details.comparison.unavailableReason) {
    case 'server_not_running':
      return '已收到客户端清单，但服务器当前未运行，暂时无法生成实际加载 Mod 的比较基准。'
    case 'server_context_unavailable':
      return '已收到客户端清单，但服务器实际加载的 Mod 上下文暂不可用。'
    case 'context_file_invalid':
      return '玩家 Mod 上下文文件暂时无法读取，请稍后重试。'
    default:
      return '已收到客户端清单，但当前缺少可用的服务器比较基准。'
  }
}

export function resolvePlayerModViewState(
  details: PlayerModDetailsResult | null,
  requestError: string | null,
): PlayerModViewState {
  if (requestError) {
    return { kind: 'request_error', showComparison: false, message: requestError }
  }
  if (!details) {
    return { kind: 'pending', showComparison: false, message: '正在读取玩家上报的 Mod 清单…' }
  }
  if (details.contextStatus === 'pending') {
    return { kind: 'pending', showComparison: false, message: '玩家已连接，正在等待客户端上报 SMAPI Mod 清单。' }
  }
  if (details.contextStatus === 'stale') {
    return {
      kind: 'stale',
      showComparison: details.comparison.status === 'available',
      message: '该玩家已断开，以下是最后一次上报记录，可能不再代表客户端当前状态。',
    }
  }
  if (details.contextStatus === 'unavailable' || details.mods === null) {
    return { kind: 'unavailable', showComparison: false, message: PLAYER_MOD_UNAVAILABLE_COPY }
  }
  if (details.comparison.status !== 'available') {
    return {
      kind: 'comparison_unavailable',
      showComparison: false,
      message: unavailableComparisonMessage(details),
    }
  }
  return { kind: 'available', showComparison: true, message: '' }
}

export function isCjbMod(item: Pick<PlayerModComparisonItem, 'uniqueId'>): boolean {
  return CJB_UNIQUE_IDS.has(item.uniqueId.trim().toLocaleLowerCase('en-US'))
}

export function isIgnoredPlayerModComparison(item: Pick<PlayerModComparisonItem, 'uniqueId'>): boolean {
  return IGNORED_COMPARISON_UNIQUE_IDS.has(item.uniqueId.trim().toLocaleLowerCase('en-US'))
}

export function hasCjbRisk(details: PlayerModDetailsResult | null): boolean {
  if (!details) return false
  return (
    details.riskFlags.some((flag) => flag.toLocaleLowerCase('en-US') === 'cjb') ||
    details.comparison.items.some(isCjbMod)
  )
}

export function hasPlayerCjbRisk(player: Pick<StardewPlayerInfo, 'modRiskFlags'> | null | undefined): boolean {
  return player?.modRiskFlags?.some((flag) => flag.trim().toLocaleLowerCase('en-US') === 'cjb') ?? false
}

export function playerModActionLabel(player: Pick<StardewPlayerInfo, 'modRiskFlags'> | null | undefined): string {
  return hasPlayerCjbRisk(player) ? PLAYER_CJB_DETECTED_LABEL : '查看上报 Mod'
}

export function groupPlayerModItems(items: PlayerModComparisonItem[]): PlayerModGroups {
  const deduplicated = new Map<string, PlayerModComparisonItem>()

  for (const item of items) {
    if (!item.uniqueId.trim()) continue
    if (isIgnoredPlayerModComparison(item)) continue
    if (item.result === 'missing_on_client' && item.syncKind === 'server_only') continue

    const key = item.uniqueId.trim().toLocaleLowerCase('en-US')
    const previous = deduplicated.get(key)
    if (!previous || (RESULT_PRIORITY[item.result] ?? 0) > (RESULT_PRIORITY[previous.result] ?? 0)) {
      deduplicated.set(key, item)
    }
  }

  const groups: PlayerModGroups = {
    match: [],
    missingOnClient: [],
    versionMismatch: [],
    clientOnly: [],
  }
  for (const item of deduplicated.values()) {
    if (item.result === 'match') groups.match.push(item)
    if (item.result === 'missing_on_client') groups.missingOnClient.push(item)
    if (item.result === 'version_mismatch') groups.versionMismatch.push(item)
    if (item.result === 'client_only') groups.clientOnly.push(item)
  }

  const byName = (left: PlayerModComparisonItem, right: PlayerModComparisonItem) =>
    (left.name || left.uniqueId).localeCompare(right.name || right.uniqueId, 'zh-CN', { sensitivity: 'base' })
  groups.match.sort(byName)
  groups.missingOnClient.sort(byName)
  groups.versionMismatch.sort(byName)
  groups.clientOnly.sort(byName)
  return groups
}
