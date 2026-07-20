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

const ACTIVE_FULL_STACK_PHASES = new Set([
  'waiting_panel', 'checking_runtime', 'notifying_players', 'saving_game',
  'backing_up_save', 'updating_runtime', 'verifying_runtime', 'restoring_server', 'rolling_back_runtime',
])

const TERMINAL_FULL_STACK_PHASES = new Set(['succeeded', 'not_needed', 'failed_safe', 'manual_action'])

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
	return Boolean(apply && (ACTIVE_PANEL_UPDATE_PHASES.has(apply.phase)
		|| apply.phase === 'succeeded' && apply.fullStack && ACTIVE_FULL_STACK_PHASES.has(apply.fullStack.phase)))
}

export function isPanelUpdateTerminal(apply: PanelUpdateApplyStatus | null): boolean {
	if (!apply || !TERMINAL_PANEL_UPDATE_PHASES.has(apply.phase)) return false
	if (apply.phase !== 'succeeded' || !apply.fullStack) return true
	return TERMINAL_FULL_STACK_PHASES.has(apply.fullStack.phase)
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
		case 'waiting_panel': return '正在更新 Panel'
		case 'checking_runtime': return '正在检查全部游戏实例'
		case 'notifying_players': return '正在通告在线玩家'
		case 'saving_game': return '正在保存游戏进度'
		case 'backing_up_save': return '正在创建整档保护备份'
		case 'updating_runtime': return '正在更新 Control 与运行栈'
		case 'verifying_runtime': return '正在验证 SMAPI 实际加载版本'
		case 'restoring_server': return '正在恢复服务器状态'
		case 'rolling_back_runtime': return '正在恢复原运行栈'
		case 'failed_safe': return '运行栈升级未完成，已安全停止或恢复'
		case 'manual_action': return '运行栈需要人工恢复'
		case 'not_needed': return '全栈已是目标版本'
    default: return '等待升级状态'
  }
}

export function panelUpdateSurface(
  status: PanelUpdateStatus | null,
  apply: PanelUpdateApplyStatus | null,
  versionInfo: VersionInfo | null,
): PanelUpdateSurface {
  const observedCurrent = status?.currentVersion || versionInfo?.version || ''
  const current = observedCurrent
    || (apply?.phase === 'succeeded' ? apply.toVersion : apply?.fromVersion)
    || ''
  const detectedTarget = status?.latestVersion || ''
  const applyTarget = apply?.toVersion || ''
  const applyOwnsSurface = isPanelUpdateActive(apply) || apply?.phase === 'rollback_failed'
  const succeededTargetsDetectedRelease = updateDisplayKind(status) === 'available'
    && withoutVersionPrefix(applyTarget) === withoutVersionPrefix(detectedTarget)
  const terminalApplyMatchesCurrent = apply?.phase === 'succeeded'
    ? withoutVersionPrefix(applyTarget) === withoutVersionPrefix(current) || succeededTargetsDetectedRelease
    : apply?.phase === 'failed_rolled_back'
      ? withoutVersionPrefix(apply.fromVersion) === withoutVersionPrefix(current)
      : false
  const detectedUpdateSupersedesTerminal = !applyOwnsSurface
    && updateDisplayKind(status) === 'available'
    && Boolean(detectedTarget)
    && withoutVersionPrefix(detectedTarget) !== withoutVersionPrefix(applyTarget)
  const applyOwnsTarget = applyOwnsSurface
    || (terminalApplyMatchesCurrent && !detectedUpdateSupersedesTerminal)
  const target = applyOwnsTarget
    ? applyTarget || detectedTarget
    : detectedTarget || current
	if (apply && isPanelUpdateActive(apply)) {
		if (apply.phase === 'rolling_back') {
      return {
        currentVersion: current, targetVersion: target,
        topbarText: '升级失败，正在恢复', mobileTopbarText: '恢复中', overviewText: '升级失败，正在恢复', tone: 'rollback',
      }
    }
		const progress = Math.max(0, Math.min(100, Math.round(apply.fullStack?.progress ?? apply.progress ?? 0)))
		const effectivePhase = apply.fullStack?.phase || apply.phase
		return {
			currentVersion: current, targetVersion: target,
			topbarText: progress > 0 ? `正在全栈升级 ${progress}%` : panelUpdatePhaseLabel(effectivePhase),
			mobileTopbarText: progress > 0 ? `升级 ${progress}%` : '升级中',
			overviewText: panelUpdatePhaseLabel(effectivePhase), tone: 'working',
    }
  }
  if (apply?.phase === 'failed_rolled_back' && terminalApplyMatchesCurrent && !detectedUpdateSupersedesTerminal) {
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
  if (apply?.phase === 'succeeded' && terminalApplyMatchesCurrent && !detectedUpdateSupersedesTerminal) {
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
