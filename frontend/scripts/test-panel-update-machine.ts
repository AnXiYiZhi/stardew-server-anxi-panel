import assert from 'node:assert/strict'
import type { PanelUpdateApplyStatus, PanelUpdateDryRunStatus, PanelUpdateStatus } from '../src/api.ts'
import {
  canStartPanelUpdate,
  isPanelUpdateActive,
  isPanelUpdateTerminal,
  panelUpdatePhaseLabel,
  panelUpdateSurface,
  reconnectDelay,
} from '../src/games/stardew/panel-update-machine.ts'

const update: PanelUpdateStatus = {
  currentVersion: '0.1.14', currentCommit: '', currentBuildDate: '', latestVersion: 'v0.1.15',
  updateAvailable: true, releaseUrl: 'https://example.test/release', publishedAt: null, checkedAt: null,
  checkStatus: 'ok', checkError: '',
}

const apply = (phase: PanelUpdateApplyStatus['phase'], progress = 0): PanelUpdateApplyStatus => ({
  updateId: 'update-1', phase, progress, fromVersion: '0.1.14', toVersion: '0.1.15',
  originalImage: '', originalDigest: '', selectedImage: '', selectedDigest: '', errorCode: '', error: '', result: '', logs: [],
  startedAt: '', updatedAt: '', finishedAt: null,
})

const dryRun: PanelUpdateDryRunStatus = {
  id: 'dry-1', phase: 'succeeded', targetVersion: '0.1.15', targetImage: '',
  capability: { supported: true, reason: '', code: 'supported', composeProject: '', composeService: '', composeFile: '', installDir: '', currentContainer: '', currentImage: '', dataMount: '', dockerAvailable: true, composeAvailable: true },
  logs: [], startedAt: '', updatedAt: '', finishedAt: '', errorCode: '', error: '',
}

assert.equal(panelUpdatePhaseLabel('backing_up'), '正在备份')
assert.equal(panelUpdatePhaseLabel('pulling'), '正在拉取镜像')
assert.equal(panelUpdatePhaseLabel('recreating'), '正在重建面板')
assert.equal(panelUpdatePhaseLabel('waiting_health'), '正在等待新版本健康')
assert.equal(panelUpdatePhaseLabel('rolling_back'), '正在回滚')

const pulling = panelUpdateSurface(update, apply('pulling', 65), { version: '0.1.14' })
assert.equal(pulling.topbarText, '正在全栈升级 65%')
assert.equal(pulling.overviewText, '正在拉取镜像')
assert.equal(pulling.tone, 'working')

const rollingBack = panelUpdateSurface(update, apply('rolling_back', 88), null)
assert.equal(rollingBack.topbarText, '升级失败，正在恢复')
assert.equal(rollingBack.tone, 'rollback')

const restored = panelUpdateSurface(update, apply('failed_rolled_back', 100), null)
assert.equal(restored.overviewText, '升级失败，已恢复')
assert.equal(restored.tone, 'restored')

const succeeded = panelUpdateSurface(update, apply('succeeded', 100), null)
assert.equal(succeeded.topbarText, 'v0.1.15')
assert.equal(succeeded.overviewText, '✓ 最新')

// 上一次升级成功记录只能解释它自己的目标，不能覆盖后续新版本。
const nextUpdate: PanelUpdateStatus = {
  ...update,
  currentVersion: '0.1.15',
  latestVersion: 'v0.1.16',
}
const nextDryRun: PanelUpdateDryRunStatus = {
  ...dryRun,
  id: 'dry-2',
  targetVersion: '0.1.16',
}
const availableAfterSuccess = panelUpdateSurface(nextUpdate, apply('succeeded', 100), { version: '0.1.15' })
assert.equal(availableAfterSuccess.currentVersion, '0.1.15')
assert.equal(availableAfterSuccess.targetVersion, 'v0.1.16')
assert.equal(availableAfterSuccess.topbarText, '发现新版本 v0.1.16')
assert.equal(availableAfterSuccess.overviewText, '发现新版本 v0.1.16')
assert.equal(panelUpdateSurface(nextUpdate, apply('pulling', 65), null).targetVersion, '0.1.15')
assert.equal(panelUpdateSurface(nextUpdate, apply('rollback_failed', 100), null).targetVersion, '0.1.15')

// 当前进程已经高于历史成功目标时，历史记录只能作为详情，不能把版本倒写回去。
const currentAfterExternalUpgrade: PanelUpdateStatus = {
  ...nextUpdate,
  currentVersion: '0.1.16',
  latestVersion: 'v0.1.16',
  updateAvailable: false,
}
const currentAfterHistoryLoads = panelUpdateSurface(currentAfterExternalUpgrade, apply('succeeded', 100), { version: '0.1.16' })
assert.equal(currentAfterHistoryLoads.currentVersion, '0.1.16')
assert.equal(currentAfterHistoryLoads.targetVersion, 'v0.1.16')
assert.equal(currentAfterHistoryLoads.topbarText, 'v0.1.16')
assert.equal(currentAfterHistoryLoads.overviewText, '✓ 最新')

// 顶栏和总览由同一个 selector 读取同一份状态，不能各自推导出不同阶段。
const shared = panelUpdateSurface(update, apply('recreating', 65), null)
assert.equal(shared.topbarText, '正在全栈升级 65%')
assert.equal(shared.overviewText, '正在重建面板')

const available = panelUpdateSurface(update, null, null)
assert.equal(available.topbarText, '发现新版本 v0.1.15')
assert.equal(available.overviewText, '发现新版本 v0.1.15')

assert.equal(canStartPanelUpdate({ id: 1, username: 'admin', role: 'admin' }, update, dryRun, null), true)
assert.equal(canStartPanelUpdate({ id: 2, username: 'player', role: 'user' }, update, dryRun, null), false)
assert.equal(canStartPanelUpdate({ id: 1, username: 'admin', role: 'admin' }, update, dryRun, apply('pulling')), false)
assert.equal(canStartPanelUpdate({ id: 1, username: 'admin', role: 'admin' }, update, dryRun, apply('succeeded')), false)
assert.equal(canStartPanelUpdate({ id: 1, username: 'admin', role: 'admin' }, nextUpdate, dryRun, apply('succeeded')), false)
assert.equal(canStartPanelUpdate({ id: 1, username: 'admin', role: 'admin' }, nextUpdate, nextDryRun, apply('succeeded')), true)
assert.equal(canStartPanelUpdate({ id: 1, username: 'admin', role: 'admin' }, nextUpdate, nextDryRun, apply('rollback_failed')), false)
assert.equal(isPanelUpdateActive(apply('waiting_health')), true)
assert.equal(isPanelUpdateTerminal(apply('failed_rolled_back')), true)
const runtimeActive = { ...apply('succeeded', 100), fullStack: { phase: 'updating_runtime', progress: 72, runtimeRequired: true } }
assert.equal(isPanelUpdateActive(runtimeActive), true)
assert.equal(isPanelUpdateTerminal(runtimeActive), false)
assert.equal(panelUpdateSurface(update, runtimeActive, null).topbarText, '正在全栈升级 72%')
const runtimeSafeFailure = { ...apply('succeeded', 100), fullStack: { phase: 'failed_safe', progress: 100, runtimeRequired: true } }
assert.equal(isPanelUpdateActive(runtimeSafeFailure), false)
assert.equal(isPanelUpdateTerminal(runtimeSafeFailure), true)
assert.deepEqual([0, 1, 2, 6, 20].map(reconnectDelay), [800, 1200, 2000, 10000, 10000])

console.log('panel update state machine tests passed')
