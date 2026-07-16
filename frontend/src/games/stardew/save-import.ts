import type { InstanceState, Job, JobLog, SaveImportHostHandling } from '../../types'

export const SAVE_IMPORT_JOB_TYPE = 'stardew_import_save_and_start'

export type SaveImportMode = SaveImportHostHandling['mode']

export type SaveImportDecisionDraft = {
  mode: SaveImportMode | null
  platformId: string
  takeoverAcknowledged: boolean
}

export type SaveImportErrorTone = 'error' | 'warning'

export type SaveImportErrorPresentation = {
  message: string
  tone: SaveImportErrorTone
  retryBlocked: boolean
}

const saveImportErrorMap: Record<string, SaveImportErrorPresentation> = {
  host_decision_required: { message: '请选择原主机角色的处理方式。', tone: 'error', retryBlocked: false },
  platform_id_invalid: { message: '请输入有效的 Steam64 或 GOG 十进制平台 ID。', tone: 'error', retryBlocked: false },
  save_exists: { message: '同名存档已经存在。现有存档未被覆盖，请先处理重名存档。', tone: 'error', retryBlocked: false },
  junimo_import_unsupported: { message: '当前 Junimo 运行版本不支持安全导入，请先完成运行组件升级。', tone: 'error', retryBlocked: false },
  save_import_busy: { message: '已有存档导入任务正在运行或等待恢复，请先查看当前任务。', tone: 'error', retryBlocked: true },
  import_command_failed: { message: 'Junimo 未执行导入命令，系统不会自动重试。请查看任务详情后再决定下一步。', tone: 'error', retryBlocked: true },
  import_result_unconfirmed: { message: '导入结果暂时无法确认。请保持服务器现状并查看任务详情，不要重复提交。', tone: 'warning', retryBlocked: true },
  import_recovery_required: { message: '导入需要人工恢复。请勿再次点击导入或重启服务器，并保留现有恢复材料。', tone: 'error', retryBlocked: true },
  import_activation_timeout: { message: '目标存档未能在期限内完成加载。请查看任务详情；系统不会重新执行导入。', tone: 'error', retryBlocked: true },
  save_in_progress: { message: '服务器正在保存、运行或接管该上传事务，请等待当前操作结束。', tone: 'error', retryBlocked: true },
}

export function validateSaveImportDecision(draft: SaveImportDecisionDraft): {
  valid: boolean
  hostHandling?: SaveImportHostHandling
  code?: 'host_decision_required' | 'platform_id_invalid'
} {
  if (draft.mode === 'swap_to_player') {
    const platformId = draft.platformId.trim()
    if (!platformId || !/^\d+$/.test(platformId)) {
      return { valid: false, code: 'platform_id_invalid' }
    }
    return { valid: true, hostHandling: { mode: 'swap_to_player', platformId } }
  }
  if (draft.mode === 'virtual_host_takeover') {
    if (!draft.takeoverAcknowledged) return { valid: false, code: 'host_decision_required' }
    return { valid: true, hostHandling: { mode: 'virtual_host_takeover', acknowledged: true } }
  }
  return { valid: false, code: 'host_decision_required' }
}

export function saveImportErrorPresentation(error: unknown): SaveImportErrorPresentation {
  const code = typeof error === 'object' && error !== null && 'code' in error && typeof error.code === 'string' ? error.code : ''
  if (code && saveImportErrorMap[code]) return saveImportErrorMap[code]
  return {
    message: '导入请求未完成，请查看任务列表确认状态后再操作。',
    tone: 'error',
    retryBlocked: true,
  }
}

export function allSaveImportErrorCodes(): string[] {
  return Object.keys(saveImportErrorMap)
}

export function findLatestSaveImportJob(jobs: Job[]): Job | null {
  return jobs.find((job) => job.type === SAVE_IMPORT_JOB_TYPE) ?? null
}

export function findActiveSaveImportJob(jobs: Job[]): Job | null {
  return jobs.find((job) => job.type === SAVE_IMPORT_JOB_TYPE && (job.status === 'queued' || job.status === 'running')) ?? null
}

export function saveImportRuntimeUnsupported(state: InstanceState | null | undefined): boolean {
  const diagnostic = state?.runtimeDiagnostic
  if (!diagnostic) return false
  const serverVersion = diagnostic.serverVersion?.trim() ?? ''
  return (serverVersion !== '' && !serverVersion.endsWith('.125')) || diagnostic.junimoVersionMatches === false
}

export function saveImportSubmissionDisabled(input: {
  draft: SaveImportDecisionDraft
  uploadBusy: boolean
  generalBusy?: boolean
  runtimeUnsupported: boolean
  activeImport: boolean
}): boolean {
  return input.uploadBusy || Boolean(input.generalBusy) || input.runtimeUnsupported || input.activeImport || !validateSaveImportDecision(input.draft).valid
}

export type SaveImportJobStage = {
  label: string
  tone: 'active' | 'success' | 'warning' | 'error'
}

export function saveImportJobStage(job: Job, logs: JobLog[]): SaveImportJobStage {
  if (job.status === 'succeeded') return { label: '导入完成', tone: 'success' }
  if (job.status === 'failed' || job.status === 'canceled') {
    return { label: '导入未完成，请查看任务详情且不要盲目重试', tone: 'error' }
  }
  const messages = logs.map((entry) => entry.message)
  const includes = (text: string) => messages.some((message) => message.includes(text))
  if (includes('Durable save-now submitted') || includes('Resuming durable save observation')) return { label: '正在保存迁移结果', tone: 'active' }
  if (includes('GameLoop.Saved') && !includes('transaction completed')) return { label: '正在验证', tone: 'active' }
  if (includes('Target runtime saveId and composite activation evidence verified')) return { label: '正在迁移住宅和家庭', tone: 'active' }
  if (includes('Phase A confirmed') || includes('Phase A composite disk evidence confirmed')) return { label: '正在加载目标存档', tone: 'active' }
  if (includes('pre-submit evidence') || includes('runtime ready')) return { label: '正在转换原主机角色', tone: 'active' }
  if (includes('Starting save_import_maintenance') || includes('Staging and preimport backup completed')) return { label: '正在启动 Junimo 导入环境', tone: 'active' }
  if (includes('Import transaction journal created')) return { label: '正在创建导入前备份', tone: 'active' }
  return { label: '正在暂存存档', tone: 'active' }
}
