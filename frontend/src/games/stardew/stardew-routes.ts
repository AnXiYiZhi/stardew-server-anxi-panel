import type { HealthDiagnosticsResponse, PanelUpdateApplyStatus, PanelUpdateDryRunStatus, PanelUpdateStatus, VersionInfo } from '../../api'
import type { CurrentUser, InstanceState, Job, JobLog, ModsListResult, PublicIPResult, SavesListResult, StardewPlayersResponse } from '../../types'

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
  jobLogsByJobId: Record<string, JobLog[]>
  health: HealthDiagnosticsResponse | null
  versionInfo: VersionInfo | null
  updateStatus: PanelUpdateStatus | null
  updateDryRun: PanelUpdateDryRunStatus | null
  updateApply: PanelUpdateApplyStatus | null
  inviteCode: string | null
  publicIP: PublicIPResult | null
  // 降级错误摘要（不崩溃，只降级显示）
  savesError: string | null
  modsError: string | null
  playersError: string | null
  healthError: string | null
  inviteCodeError: string | null
  publicIPError: string | null
  updateError: string | null
  updateDryRunError: string | null
  updateApplyError: string | null
  // 加载状态
  loading: boolean
  playersLoading: boolean
  inviteCodeRefreshing: boolean
  publicIPRefreshing: boolean
  updateChecking: boolean
  updateDryRunChecking: boolean
  updateApplyStarting: boolean
  updateDialogOpen: boolean
  // 刷新函数（供各页面在操作完成后主动刷新）
  refreshAll: () => void
  refreshInstanceState: () => void
  refreshSaves: () => void
  refreshMods: () => void
  refreshPlayers: () => void
  refreshJobs: () => void
  refreshHealth: () => void
  applyHealthDiagnostics: (health: HealthDiagnosticsResponse) => void
  refreshInviteCode: () => void
  refreshPublicIP: (force?: boolean) => void
  refreshUpdateStatus: (manual?: boolean) => Promise<void>
  runUpdateDryRun: (targetVersion: string) => Promise<void>
  applyUpdate: () => Promise<void>
  openUpdateDialog: () => void
  closeUpdateDialog: () => void
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
