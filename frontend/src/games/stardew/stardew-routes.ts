import type { HealthDiagnosticsResponse, VersionInfo } from '../../api'
import type { CurrentUser, InstanceState, Job, ModsListResult, SavesListResult, StardewPlayersResponse } from '../../types'

export type StardewRoute =
  | 'install'
  | 'overview'
  | 'server'
  | 'saves'
  | 'jobs'
  | 'players'
  | 'mods'
  | 'diagnostics'
  | 'settings'

export type StardewSaveAction = 'new' | 'upload'

export type StardewNavigateOptions = {
  saveAction?: StardewSaveAction
}

export type StardewSaveActionRequest = {
  action: StardewSaveAction
  nonce: number
}

// 公共数据层：由 useStardewDashboardData hook 填充，通过 StardewPanel 传给所有页面
export type StardewDashboardData = {
  // 核心数据
  instanceState: InstanceState | null
  saves: SavesListResult | null
  mods: ModsListResult | null
  players: StardewPlayersResponse | null
  jobs: Job[]
  health: HealthDiagnosticsResponse | null
  versionInfo: VersionInfo | null
  inviteCode: string | null
  // 降级错误摘要（不崩溃，只降级显示）
  savesError: string | null
  modsError: string | null
  playersError: string | null
  healthError: string | null
  inviteCodeError: string | null
  // 加载状态
  loading: boolean
  playersLoading: boolean
  // 刷新函数（供各页面在操作完成后主动刷新）
  refreshAll: () => void
  refreshInstanceState: () => void
  refreshSaves: () => void
  refreshMods: () => void
  refreshPlayers: () => void
  refreshJobs: () => void
  refreshHealth: () => void
  refreshInviteCode: () => void
  clearInviteCode: () => void
  requestInviteCodeRefresh: () => void
}

export type StardewPageProps = {
  user: CurrentUser
  instanceState: InstanceState | null  // 与 dashboardData.instanceState 相同，保留向后兼容
  dashboardData: StardewDashboardData
  onNavigate: (route: StardewRoute, options?: StardewNavigateOptions) => void
  saveActionRequest?: StardewSaveActionRequest | null
  onLogout: () => void
}

const ROUTE_BASE = '/instances/stardew'

const VALID_ROUTES: StardewRoute[] = [
  'install',
  'overview',
  'server',
  'saves',
  'jobs',
  'players',
  'mods',
  'diagnostics',
  'settings',
]

export function parseRoute(pathname: string): StardewRoute {
  const suffix = pathname.startsWith(ROUTE_BASE + '/')
    ? pathname.slice(ROUTE_BASE.length + 1).split('/')[0]
    : ''
  return VALID_ROUTES.find((r) => r === suffix) ?? 'overview'
}

export function routeToPath(route: StardewRoute): string {
  return `${ROUTE_BASE}/${route}`
}
