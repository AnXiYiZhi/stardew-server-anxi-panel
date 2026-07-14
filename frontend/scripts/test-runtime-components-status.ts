import assert from 'node:assert/strict'
import { runtimeComponentsStatusLabel, shouldShowRuntimeComponentsUpdate } from '../src/games/stardew/runtime-components-status.ts'
import type { RuntimeComponentsInfo } from '../src/types.ts'

assert.equal(runtimeComponentsStatusLabel('game_missing'), '游戏文件缺失')
assert.equal(runtimeComponentsStatusLabel('sdk_missing'), '联机运行库缺失')
assert.equal(runtimeComponentsStatusLabel('manifest_invalid'), '版本清单损坏')
assert.equal(runtimeComponentsStatusLabel('custom_or_unknown'), '自定义或未知状态')

const info = { status: 'update_available', recommended: { tested: true } } as RuntimeComponentsInfo
assert.equal(shouldShowRuntimeComponentsUpdate(info), true)
assert.equal(shouldShowRuntimeComponentsUpdate({ ...info, status: 'up_to_date' }), false)
assert.equal(shouldShowRuntimeComponentsUpdate({ ...info, recommended: { ...info.recommended, tested: false } }), false)
console.log('runtime components status tests passed')
