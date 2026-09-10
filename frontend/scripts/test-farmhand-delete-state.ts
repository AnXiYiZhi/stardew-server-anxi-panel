import assert from 'node:assert/strict'
import type { FarmhandDeleteIntent } from '../src/types.ts'
import {
  farmhandDeleteConfirmationReady,
  farmhandDeleteExpiryText,
  farmhandDeleteIntentActive,
  farmhandDeleteIntentCancelable,
  farmhandDeleteIntentNeedsRecovery,
  farmhandDeleteStatusLabel,
} from '../src/games/stardew/farmhand-delete-state.ts'

const waiting: FarmhandDeleteIntent = {
  operationId: '0123456789abcdef0123456789abcdef',
  mode: 'wait',
  status: 'waiting',
  uniqueMultiplayerId: '42',
  expectedName: 'Leah',
  expectedSaveId: 'Farm_1',
  expiresAt: '2026-09-10T12:00:00.000Z',
}

assert.equal(farmhandDeleteIntentActive(waiting), true)
assert.equal(farmhandDeleteIntentCancelable(waiting), true)
assert.equal(farmhandDeleteIntentNeedsRecovery(waiting), false)
assert.equal(farmhandDeleteExpiryText(waiting, Date.parse('2026-09-10T10:01:00.000Z')), '剩余约 2 小时')
assert.equal(farmhandDeleteExpiryText(waiting, Date.parse('2026-09-10T11:31:00.000Z')), '剩余约 29 分钟')

for (const status of ['launching', 'countdown', 'active', 'recovery_required'] as const) {
  assert.equal(farmhandDeleteIntentActive({ ...waiting, status }), true, status)
}
for (const status of ['completed', 'canceled', 'expired', 'failed'] as const) {
  assert.equal(farmhandDeleteIntentActive({ ...waiting, status }), false, status)
}

assert.equal(farmhandDeleteIntentNeedsRecovery({ ...waiting, status: 'recovery_required' }), true)
assert.equal(farmhandDeleteIntentCancelable({ ...waiting, status: 'countdown' }), true)
assert.equal(farmhandDeleteStatusLabel('countdown'), '维护倒计时中')
assert.equal(farmhandDeleteStatusLabel('recovery_required'), '需要恢复处理')
assert.equal(farmhandDeleteStatusLabel('canceled'), '操作已取消')
assert.equal(farmhandDeleteConfirmationReady('wait', false, '', 'Leah'), true)
assert.equal(farmhandDeleteConfirmationReady('maintenance_now', false, 'Leah', 'Leah'), false)
assert.equal(farmhandDeleteConfirmationReady('maintenance_now', true, 'Wrong', 'Leah'), false)
assert.equal(farmhandDeleteConfirmationReady('maintenance_now', true, ' Leah ', 'Leah'), true)

console.log('farmhand deletion state tests passed')
