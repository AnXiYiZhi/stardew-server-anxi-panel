import type { FullStackUpgradeStatus, PanelUpdateApplyStatus } from '../../api'

export const fullStackTerminal = (phase: string) => ['succeeded','failed_safe','manual_action','preflight_failed','panel_failed_rolled_back'].includes(phase)

function explicitlyLinkedApply(base: PanelUpdateApplyStatus | null, op: FullStackUpgradeStatus): boolean {
  return Boolean(base && (base.operationId && base.operationId === op.operationId || op.panelUpdateId && op.panelUpdateId === base.updateId))
}

function panelRuntimeCheckComplete(base: PanelUpdateApplyStatus | null, currentVersion: string): boolean {
  const normalize = (value: string) => value.replace(/^[vV]/, '')
  if (!base || base.phase !== 'succeeded' || normalize(base.toVersion) !== normalize(currentVersion) || base.fullStack?.phase !== 'not_needed' || base.fullStack.runtimeRequired) return false
  const proof = base.fullStack.currentRuntimeVerified
  // Legacy not_needed also represents an authoritative empty installed scope.
  return !proof || proof.updateId === base.updateId && normalize(proof.panel.version) === normalize(currentVersion) && proof.panel.imageId === base.selectedDigest
}

export function selectCurrentFullStackUpgrade(base: PanelUpdateApplyStatus | null, op: FullStackUpgradeStatus | null, currentVersion: string): FullStackUpgradeStatus | null {
  if (!op || op.phase === 'failed_safe' && op.errorCode === 'administrator_cancelled') return null
  const proof = base?.fullStack?.currentRuntimeVerified
  if (op.continuation && op.phase === 'failed_safe' && op.authorizedAt && proof && panelRuntimeCheckComplete(base, currentVersion)
    && proof.panel.version.replace(/^[vV]/, '') === op.panel.version.replace(/^[vV]/, '')
    && proof.panel.imageId === op.panel.imageId && Date.parse(proof.verifiedAt) > Date.parse(op.authorizedAt)) return null
  if (explicitlyLinkedApply(base, op)) return op
  // A current preparation or recovery remains actionable independently of when
  // the browser loaded the Panel status. Only unrelated terminal history ages out.
  if (!fullStackTerminal(op.phase)) return op
  const normalize = (value: string) => value.replace(/^[vV]/, '')
  const current = normalize(currentVersion || (base?.phase === 'succeeded' ? base.toVersion : base?.fromVersion) || '')
  const target = normalize(op.panel.version)
  if (current && target !== current || base && normalize(base.toVersion) !== target) return null
  const created = Date.parse(op.createdAt)
  const panelStarted = Date.parse(base?.startedAt || '')
  if (Number.isFinite(created) && Number.isFinite(panelStarted) && created < panelStarted) return null
  return op
}

export function selectHistoricalFullStackRecovery(raw: FullStackUpgradeStatus | null, current: FullStackUpgradeStatus | null): FullStackUpgradeStatus | null {
  return raw && raw !== current && raw.authorizedAt && raw.phase === 'manual_action' ? raw : null
}

function currentRuntimeCheckComplete(op: FullStackUpgradeStatus, currentVersion: string): boolean {
  return op.phase === 'succeeded' && op.continuation && !op.authorizedAt && op.groups.length === 0
    && op.panel.version.replace(/^[vV]/, '') === currentVersion.replace(/^[vV]/, '')
}

export function currentGameUpgradeComplete(base: PanelUpdateApplyStatus | null, op: FullStackUpgradeStatus | null, currentVersion: string, historicalRecovery?: FullStackUpgradeStatus | null): boolean {
  if (historicalRecovery) return false
  const current = selectCurrentFullStackUpgrade(base, op, currentVersion)
  const normalize = (value: string) => value.replace(/^[vV]/, '')
  if (current) return currentRuntimeCheckComplete(current, currentVersion) || current.phase === 'succeeded' && Boolean(current.authorizedAt) && normalize(current.panel.version) === normalize(currentVersion)
  return panelRuntimeCheckComplete(base, currentVersion)
}

export function fullStackConfirmationExpired(op: FullStackUpgradeStatus | null): boolean {
  if (!op || op.authorizedAt || op.phase !== 'awaiting_confirmation') return false
  return op.confirmationExpired === true
}

export const fullStackPhaseLabel = (phase: string) => ({
  preparing:'正在下载和预检共享范围',
  prepared:'预检完成，等待确认',
  applying:'共享组事务已启动',
  recovering:'正在恢复本次游戏组事务',
  verifying_remaining:'正在核对其余组的原运行栈',
  unchanged:'保持原运行栈，未执行迁移',
  committed:'共享组已完成持久提交',
  rolled_back:'共享组已恢复原运行栈',
  failed_rolled_back:'共享组升级失败并已恢复',
  awaiting_confirmation:'请确认精确目标和共享世界范围',
  waiting_panel:'正在更新 Panel',
  migrating:'正在保存、备份并迁移游戏组',
  quiescing:'正在保存并逐世界停服',
  backing_up:'正在完成全部世界的保护备份',
  preparing_game:'正在准备共享游戏与 patched SMAPI',
  updating_worlds:'正在替换并验收各世界',
  restoring_state:'正在恢复原运行选择',
  rolling_back:'正在整组恢复原运行栈',
  rollback_failed:'整组恢复需要管理员处理',
  scope_changed:'范围或目标变化，需要重新确认',
  succeeded:'Panel 与全部目标游戏组验收完成',
  failed_safe:'升级未完成，已保留或恢复原游戏组',
  manual_action:'恢复需要管理员处理',
  preflight_failed:'预检未通过，原游戏保持',
  panel_failed_rolled_back:'Panel 更新失败，已恢复旧 Panel',
}[phase] ?? phase)

export function projectFullStackApply(base: PanelUpdateApplyStatus | null, op: FullStackUpgradeStatus | null, currentVersion: string): PanelUpdateApplyStatus | null {
  op = selectCurrentFullStackUpgrade(base, op, currentVersion)
  if (op && currentRuntimeCheckComplete(op, currentVersion) && base?.phase === 'succeeded' && base.toVersion.replace(/^[vV]/, '') === op.panel.version.replace(/^[vV]/, '')) {
    return { ...base, fullStack: { phase: 'not_needed', progress: 100, runtimeRequired: false, result: '当前游戏组件已是目标版本，无需迁移。' } }
  }
  if (!op?.authorizedAt) return base
  const matching = base && (explicitlyLinkedApply(base, op) || op.continuation && base.toVersion.replace(/^v/,'') === op.panel.version)
  const panel = matching ? base : {
    updateId:op.panelUpdateId || op.operationId,phase:op.continuation ? 'succeeded' as const : 'checking' as const,
    progress:0,fromVersion:currentVersion,toVersion:op.panel.version,originalImage:'',originalDigest:'',
    selectedImage:op.panel.imageId,selectedDigest:op.panel.imageId,errorCode:'',error:'',result:'',
    logs:[],startedAt:op.createdAt,updatedAt:op.createdAt,finishedAt:null,
  }
  const phase = op.phase === 'succeeded' ? 'succeeded'
    : op.phase === 'scope_changed' ? 'awaiting_confirmation'
    : op.phase === 'manual_action' ? 'manual_action'
    : op.phase === 'failed_safe' || op.phase === 'panel_failed_rolled_back' ? 'failed_safe'
    : op.phase === 'waiting_panel' && !op.continuation ? 'waiting_panel' : 'checking_runtime'
  return { ...panel, operationId:op.operationId, fullStack:{
    phase,progress:phase === 'succeeded' ? 100 : 0,runtimeRequired:true,
    result:fullStackPhaseLabel(op.phase),errorCode:op.errorCode,error:op.error,
    instances:op.groups.flatMap(group => group.members.map(instanceId => ({
      instanceId,phase:group.phase,progress:group.phase === 'succeeded' ? 100 : 0,errorCode:group.errorCode,
    }))),
  }}
}
