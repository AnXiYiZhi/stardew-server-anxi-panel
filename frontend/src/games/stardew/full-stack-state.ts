import type { FullStackUpgradeStatus, PanelUpdateApplyStatus } from '../../api'

export const fullStackTerminal = (phase: string) => ['succeeded','failed_safe','manual_action','preflight_failed','panel_failed_rolled_back'].includes(phase)
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
  if (!op?.authorizedAt) return base
  const matching = base && (base.operationId === op.operationId || op.continuation && base.toVersion.replace(/^v/,'') === op.panel.version)
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
