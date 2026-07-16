import assert from 'node:assert/strict'
import { ApiError, uploadSaveCommitAndStart } from '../src/api.ts'
import type { Job, JobLog, SaveImportHostHandling } from '../src/types.ts'
import {
  allSaveImportErrorCodes,
  findActiveSaveImportJob,
  saveImportErrorPresentation,
  saveImportJobStage,
  saveImportRuntimeUnsupported,
  saveImportSubmissionDisabled,
  validateSaveImportDecision,
} from '../src/games/stardew/save-import.ts'

const hugeID = '18446744073709551615'
const swap = validateSaveImportDecision({ mode: 'swap_to_player', platformId: ` ${hugeID} `, takeoverAcknowledged: false })
assert.equal(swap.valid, true)
assert.deepEqual(swap.hostHandling, { mode: 'swap_to_player', platformId: hugeID })
assert.equal(typeof swap.hostHandling?.mode === 'string' && (swap.hostHandling as { platformId?: unknown }).platformId, hugeID)
assert.equal(validateSaveImportDecision({ mode: 'swap_to_player', platformId: '', takeoverAcknowledged: false }).code, 'platform_id_invalid')
assert.equal(validateSaveImportDecision({ mode: 'swap_to_player', platformId: '7e16', takeoverAcknowledged: false }).code, 'platform_id_invalid')
assert.equal(validateSaveImportDecision({ mode: null, platformId: '', takeoverAcknowledged: false }).code, 'host_decision_required')
assert.equal(validateSaveImportDecision({ mode: 'virtual_host_takeover', platformId: '', takeoverAcknowledged: false }).valid, false)
assert.deepEqual(
  validateSaveImportDecision({ mode: 'virtual_host_takeover', platformId: '', takeoverAcknowledged: true }).hostHandling,
  { mode: 'virtual_host_takeover', acknowledged: true },
)
assert.equal(saveImportRuntimeUnsupported({ runtimeDiagnostic: { serverVersion: '1.5.0-preview.125', junimoVersionMatches: true } } as never), false)
assert.equal(saveImportRuntimeUnsupported({ runtimeDiagnostic: { serverVersion: '.125', junimoVersionMatches: true } } as never), false)
assert.equal(saveImportRuntimeUnsupported({ runtimeDiagnostic: { serverVersion: '1.5.0-preview.124', junimoVersionMatches: false } } as never), true)

const capturedBodies: unknown[] = []
const originalFetch = globalThis.fetch
globalThis.fetch = async (_input, init) => {
  capturedBodies.push(JSON.parse(String(init?.body)))
  return new Response(JSON.stringify({ jobId: 'job-1', operationId: 'op-1', saveName: 'Farm_1' }), {
    status: 202,
    headers: { 'Content-Type': 'application/json' },
  })
}
const desktopHandling: SaveImportHostHandling = { mode: 'swap_to_player', platformId: hugeID }
const mobileHandling: SaveImportHostHandling = { mode: 'virtual_host_takeover', acknowledged: true }
await uploadSaveCommitAndStart('desktop-token', desktopHandling)
await uploadSaveCommitAndStart('mobile-token', mobileHandling)
globalThis.fetch = originalFetch
assert.deepEqual(capturedBodies[0], { token: 'desktop-token', hostHandling: desktopHandling })
assert.deepEqual(capturedBodies[1], { token: 'mobile-token', hostHandling: mobileHandling })
assert.equal(JSON.stringify(capturedBodies).includes('cancel'), false)

function job(status: Job['status'] = 'running'): Job {
  return {
    id: 'import-job', type: 'stardew_import_save_and_start', status, targetType: 'instance', targetId: 'stardew',
    createdBy: 1, createdAt: '2026-07-16T00:00:00Z', startedAt: '2026-07-16T00:00:01Z', finishedAt: null,
    errorMessage: null, updatedAt: '2026-07-16T00:00:02Z',
  }
}

function logs(...messages: string[]): JobLog[] {
  return messages.map((message, index) => ({ id: index + 1, jobId: 'import-job', level: 'info', message, createdAt: '2026-07-16T00:00:00Z', sequence: index + 1 }))
}

assert.equal(saveImportJobStage(job(), []).label, '正在暂存存档')
assert.equal(saveImportJobStage(job(), logs('Import transaction journal created.')).label, '正在创建导入前备份')
assert.equal(saveImportJobStage(job(), logs('Staging and preimport backup completed; starting save_import_maintenance runtime.')).label, '正在启动 Junimo 导入环境')
assert.equal(saveImportJobStage(job(), logs('save_import_maintenance runtime ready; composite evidence baseline captured. No import command was sent.')).label, '正在转换原主机角色')
assert.equal(saveImportJobStage(job(), logs('Phase A confirmed; waiting for the target runtime saveId and finalizer composite evidence.')).label, '正在加载目标存档')
assert.equal(saveImportJobStage(job(), logs('Target runtime saveId and composite activation evidence verified; no import command was re-sent.')).label, '正在迁移住宅和家庭')
assert.equal(saveImportJobStage(job(), logs('Durable save-now submitted through Control; waiting for the same commandId GameLoop.Saved result.')).label, '正在保存迁移结果')
assert.equal(saveImportJobStage(job(), logs('GameLoop.Saved received; checking disk stability.')).label, '正在验证')
assert.equal(saveImportJobStage(job('succeeded'), []).label, '导入完成')
assert.notEqual(saveImportJobStage(job('failed'), []).tone, 'success')

assert.equal(findActiveSaveImportJob([job('running')])?.id, 'import-job')
assert.equal(findActiveSaveImportJob([job('succeeded')]), null)
assert.equal(saveImportSubmissionDisabled({
  draft: { mode: null, platformId: '', takeoverAcknowledged: false }, uploadBusy: false, runtimeUnsupported: false, activeImport: false,
}), true)
assert.equal(saveImportSubmissionDisabled({
  draft: { mode: 'swap_to_player', platformId: hugeID, takeoverAcknowledged: false }, uploadBusy: false, runtimeUnsupported: false, activeImport: false,
}), false)
assert.equal(saveImportSubmissionDisabled({
  draft: { mode: 'swap_to_player', platformId: hugeID, takeoverAcknowledged: false }, uploadBusy: false, runtimeUnsupported: false, activeImport: true,
}), true)

assert.deepEqual(allSaveImportErrorCodes().sort(), [
  'host_decision_required', 'platform_id_invalid', 'save_exists', 'junimo_import_unsupported', 'save_import_busy',
  'import_command_failed', 'import_result_unconfirmed', 'import_recovery_required', 'import_activation_timeout', 'save_in_progress',
].sort())
assert.equal(saveImportErrorPresentation(new ApiError(409, 'import_result_unconfirmed', 'upstream text')).tone, 'warning')
assert.equal(saveImportErrorPresentation(new ApiError(409, 'import_result_unconfirmed', 'upstream text')).message.includes('upstream text'), false)
assert.equal(saveImportErrorPresentation(new ApiError(409, 'import_recovery_required', 'retry')).retryBlocked, true)

console.log('save import frontend tests passed')
