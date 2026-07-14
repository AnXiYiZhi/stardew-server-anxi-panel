import type { CurrentUser } from '../../types'
import type { PanelUpdateApplyStatus, PanelUpdateDryRunStatus, PanelUpdateStatus, VersionInfo } from '../../api'
import { updateDisplayKind, withoutVersionPrefix, withVersionPrefix } from './update-status.ts'

export const ACTIVE_PANEL_UPDATE_PHASES = new Set([
  'checking',
  'backing_up',
  'pulling',
  'recreating',
  'waiting_health',
  'rolling_back',
])

export const TERMINAL_PANEL_UPDATE_PHASES = new Set([
  'succeeded',
  'failed_rolled_back',
  'rollback_failed',
])

export type PanelUpdateTone = 'latest' | 'available' | 'working' | 'rollback' | 'restored' | 'error' | 'muted'

export type PanelUpdateSurface = {
  currentVersion: string
  targetVersion: string
  topbarText: string
  mobileTopbarText: string
  overviewText: string
  tone: PanelUpdateTone
}

export function isPanelUpdateActive(apply: PanelUpdateApplyStatus | null): boolean {
  return Boolean(apply && ACTIVE_PANEL_UPDATE_PHASES.has(apply.phase))
}

export function isPanelUpdateTerminal(apply: PanelUpdateApplyStatus | null): boolean {
  return Boolean(apply && TERMINAL_PANEL_UPDATE_PHASES.has(apply.phase))
}

export function panelUpdatePhaseLabel(phase: string): string {
  switch (phase) {
    case 'checking': return '正在检查升级条件'
    case 'backing_up': return '正在备份'
    case 'pulling': return '正在拉取镜像'
    case 'recreating': return '正在重建面板'
    case 'waiting_health': return '正在等待新版本健康'
    case 'rolling_back': return '正在回滚'
    case 'succeeded': return '升级成功'
    case 'failed_rolled_back': return '升级失败，已恢复'
    case 'rollback_failed': return '自动恢复未完成'
    default: return '等待升级状态'
  }
}

export function panelUpdateSurface(
  status: PanelUpdateStatus | null,
  apply: PanelUpdateApplyStatus | null,
  versionInfo: VersionInfo | null,
): PanelUpdateSurface {
  const current = status?.currentVersion || versionInfo?.version || apply?.fromVersion || ''
  const detectedTarget = status?.latestVersion || ''
  const applyTarget = apply?.toVersion || ''
  const applyOwnsSurface = isPanelUpdateActive(apply) || apply?.phase === 'rollback_failed'
  const detectedUpdateSupersedesTerminal = !applyOwnsSurface
    && updateDisplayKind(status) === 'available'
    && Boolean(detectedTarget)
    && withoutVersionPrefix(detectedTarget) !== withoutVersionPrefix(applyTarget)
  const target = detectedUpdateSupersedesTerminal
    ? detectedTarget
    : applyTarget || detectedTarget
  if (apply && ACTIVE_PANEL_UPDATE_PHASES.has(apply.phase)) {
    if (apply.phase === 'rolling_back') {
      return {
        currentVersion: current, targetVersion: target,
        topbarText: '升级失败，正在恢复', mobileTopbarText: '恢复中', overviewText: '升级失败，正在恢复', tone: 'rollback',
      }
    }
    const progress = Math.max(0, Math.min(100, Math.round(apply.progress || 0)))
    return {
      currentVersion: current, targetVersion: target,
      topbarText: progress > 0 ? `正在升级 ${progress}%` : panelUpdatePhaseLabel(apply.phase),
      mobileTopbarText: progress > 0 ? `升级 ${progress}%` : '升级中',
      overviewText: '正在升级…', tone: 'working',
    }
  }
  if (apply?.phase === 'failed_rolled_back' && !detectedUpdateSupersedesTerminal) {
    return {
      currentVersion: current, targetVersion: target,
      topbarText: '升级失败，已恢复', mobileTopbarText: '已恢复', overviewText: '升级失败，已恢复', tone: 'restored',
    }
  }
  if (apply?.phase === 'rollback_failed') {
    return {
      currentVersion: current, targetVersion: target,
      topbarText: '升级需要处理', mobileTopbarText: '升级异常', overviewText: '自动恢复未完成', tone: 'error',
    }
  }
  if (apply?.phase === 'succeeded' && !detectedUpdateSupersedesTerminal) {
    const upgraded = apply.toVersion || current
    return {
      currentVersion: upgraded, targetVersion: upgraded,
      topbarText: withVersionPrefix(upgraded), mobileTopbarText: withVersionPrefix(upgraded), overviewText: '✓ 最新', tone: 'latest',
    }
  }
  if (updateDisplayKind(status) === 'available') {
    return {
      currentVersion: current, targetVersion: target,
      topbarText: `发现新版本 ${withVersionPrefix(target)}`,
      mobileTopbarText: `↑ ${withVersionPrefix(target)}`,
      overviewText: `发现新版本 ${withVersionPrefix(target)}`,
      tone: 'available',
    }
  }
  const kind = updateDisplayKind(status)
  return {
    currentVersion: current, targetVersion: target,
    topbarText: withVersionPrefix(current), mobileTopbarText: withVersionPrefix(current),
    overviewText: kind === 'latest' ? '✓ 最新' : kind === 'checking' ? '检查中…' : kind === 'unavailable' ? '开发版本' : '检查失败',
    tone: kind === 'latest' ? 'latest' : kind === 'error' ? 'error' : 'muted',
  }
}

export function canStartPanelUpdate(
  user: CurrentUser,
  status: PanelUpdateStatus | null,
  dryRun: PanelUpdateDryRunStatus | null,
  apply: PanelUpdateApplyStatus | null,
): boolean {
  const latestVersion = withoutVersionPrefix(status?.latestVersion)
  const dryRunMatchesTarget = Boolean(latestVersion)
    && withoutVersionPrefix(dryRun?.targetVersion) === latestVersion
  const completedThisTarget = apply?.phase === 'succeeded'
    && withoutVersionPrefix(apply.toVersion) === latestVersion
  return user.role === 'admin'
    && Boolean(status?.updateAvailable)
    && dryRun?.phase === 'succeeded'
    && dryRunMatchesTarget
    && !isPanelUpdateActive(apply)
    && apply?.phase !== 'rollback_failed'
    && !completedThisTarget
}

export function reconnectDelay(attempt: number): number {
  return [800, 1_200, 2_000, 3_500, 5_000, 8_000, 10_000][Math.min(Math.max(attempt, 0), 6)]
}
