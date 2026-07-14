import assert from 'node:assert/strict'
import { shouldShowSMAPIUpdate, smapiPhaseActive, smapiPhaseLabel, smapiStatusLabel } from '../src/games/stardew/smapi-update-status.ts'
import type { SMAPIUpdateInfo } from '../src/types.ts'

assert.equal(smapiStatusLabel('incompatible_game'), '游戏或 Steamworks SDK 前置不兼容')
assert.equal(smapiPhaseLabel('rolling_back'), '正在切回旧 volume')
assert.equal(smapiPhaseActive('installing'), true)
assert.equal(smapiPhaseActive('failed_rolled_back'), false)
const info = { available: true, supported: true, status: 'update_available' } as SMAPIUpdateInfo
assert.equal(shouldShowSMAPIUpdate(info), true)
assert.equal(shouldShowSMAPIUpdate({ ...info, supported: false }), false)
assert.equal(shouldShowSMAPIUpdate({ ...info, status: 'custom_or_unknown' }), false)
