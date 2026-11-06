import assert from 'node:assert/strict'
import type { FullStackUpgradeStatus, PanelUpdateApplyStatus, PanelUpdateDryRunStatus, PanelUpdateStatus } from '../src/api.ts'
import { currentGameUpgradeComplete, projectFullStackApply, selectCurrentFullStackUpgrade, selectHistoricalFullStackRecovery } from '../src/games/stardew/full-stack-state.ts'
import {
  canStartPanelUpdate,
  hasGameUpgradeFlow,
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

assert.equal(hasGameUpgradeFlow(null), false)
assert.equal(hasGameUpgradeFlow({ version: '0.7.2' }), false)
assert.equal(hasGameUpgradeFlow({ version: '0.7.3', runtimeUpgrade: { requiresScopeConfirmation: false, serverVersion: '1.5.0-preview.125', authVersion: '1.5.0-anxi.2' } }), false)
assert.equal(hasGameUpgradeFlow({ version: '0.8.0', runtimeUpgrade: { requiresScopeConfirmation: true, serverVersion: '1.5.0-preview.140', authVersion: '1.5.0-anxi.3' } }), true)
assert.equal(hasGameUpgradeFlow({ version: '0.7.3' }, { authorizedAt: '2026-10-25T00:00:00Z' }), true)
const knownBridge = { version: '0.7.3', runtimeUpgrade: { requiresScopeConfirmation: false, targetPanelVersion: '0.8.0', serverVersion: '1.5.0-preview.140', authVersion: '1.5.0-anxi.3' } }
assert.equal(hasGameUpgradeFlow(knownBridge), false)
assert.equal(hasGameUpgradeFlow(knownBridge, null, { ...update, latestVersion: 'v0.7.4' }), false)
assert.equal(hasGameUpgradeFlow(knownBridge, null, { ...update, latestVersion: 'v0.8.0' }), true)
assert.equal(hasGameUpgradeFlow(knownBridge, null, { ...update, latestVersion: 'v0.8.1' }), false)
assert.equal(hasGameUpgradeFlow(knownBridge, null, { ...update, latestVersion: 'v0.8.2' }), false)
assert.equal(hasGameUpgradeFlow({ ...knownBridge, version: '0.8.0', runtimeUpgrade: { ...knownBridge.runtimeUpgrade, requiresScopeConfirmation: true } }, { authorizedAt: '2026-10-25T00:00:00Z', phase: 'succeeded' }, { ...update, latestVersion: 'v0.8.1' }), false)
assert.equal(hasGameUpgradeFlow({ ...knownBridge, version: '0.8.1', runtimeUpgrade: { ...knownBridge.runtimeUpgrade, requiresScopeConfirmation: true } }), true)
for (const version of ['0.8.1', '0.8.2']) {
  const loadedRelease = { ...knownBridge, version, runtimeUpgrade: { ...knownBridge.runtimeUpgrade, targetPanelVersion: version, requiresScopeConfirmation: true } }
  assert.equal(hasGameUpgradeFlow(loadedRelease), true, 'the loaded target release owns its runtime scope confirmation')
  assert.equal(hasGameUpgradeFlow(loadedRelease, null, { ...update, latestVersion: 'v0.8.3' }), false, 'future Panel updates precede that release\'s own runtime confirmation')
}
assert.equal(hasGameUpgradeFlow(knownBridge, { authorizedAt: '2026-10-25T00:00:00Z' }, { ...update, latestVersion: 'v0.8.1' }), true)

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
assert.equal(panelUpdatePhaseLabel('verifying_auth'), '正在验证认证服务健康（不等待 Steam 登录）')
assert.equal(panelUpdatePhaseLabel('verifying_runtime'), '正在验证 SMAPI 实际加载版本')
assert.equal(panelUpdatePhaseLabel('manual_action','legacy_full_stack_confirmation_required'),'等待游戏迁移确认')
assert.equal(panelUpdatePhaseLabel('manual_action'),'运行栈需要人工恢复')

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
const authVerificationActive = { ...apply('succeeded', 100), fullStack: { phase: 'verifying_auth', progress: 68, runtimeRequired: true } }
assert.equal(isPanelUpdateActive(authVerificationActive), true)
assert.equal(isPanelUpdateTerminal(authVerificationActive), false)
assert.equal(panelUpdateSurface(update, authVerificationActive, null).overviewText, '正在验证认证服务健康（不等待 Steam 登录）')
const runtimeSafeFailure = { ...apply('succeeded', 100), fullStack: { phase: 'failed_safe', progress: 100, runtimeRequired: true } }
assert.equal(isPanelUpdateActive(runtimeSafeFailure), false)
assert.equal(isPanelUpdateTerminal(runtimeSafeFailure), true)
assert.deepEqual([0, 1, 2, 6, 20].map(reconnectDelay), [800, 1200, 2000, 10000, 10000])

// The actual080 -> 081 failure: an older authorized080 scope must not replace
// the newer Panel apply's explicit request for runtime-scope confirmation.
const current081: PanelUpdateApplyStatus = {
  ...apply('succeeded', 100), updateId:'panel-081', fromVersion:'0.8.0', toVersion:'0.8.1',
  startedAt:'2026-11-05T00:00:00Z',
  fullStack:{phase:'manual_action',progress:100,runtimeRequired:true,errorCode:'legacy_full_stack_confirmation_required'},
}
const history080: FullStackUpgradeStatus = {
  operationId:'scope-080',planSha256:'1'.repeat(64),phase:'succeeded',actorId:41,
  panel:{version:'0.8.0',image:'panel:080',imageId:'sha256:'+'2'.repeat(64),revision:'3'.repeat(40),buildDate:'2026-10-28T00:00:00Z'},
  runtimeVersion:'1.5.0-preview.140',runtimeDigest:'sha256:'+'4'.repeat(64),
  controlVersion:'0.4.0',controlSha256:'5'.repeat(64),gameBuildId:'game',sdkBuildId:'sdk',smapiIdentity:'patched SMAPI',
  createdAt:'2026-10-28T00:00:00Z',expiresAt:'2026-10-29T00:00:00Z',authorizedAt:'2026-10-28T00:01:00Z',
  panelUpdateId:'panel-080',continuation:false,groups:[],
}
const historyCopy = structuredClone(history080)
assert.equal(selectCurrentFullStackUpgrade(current081,history080,'0.8.1'),null)
assert.equal(projectFullStackApply(current081,history080,'0.8.1'),current081)
const manualSurface = panelUpdateSurface({...update,currentVersion:'0.8.1',latestVersion:'v0.8.1',updateAvailable:false},projectFullStackApply(current081,history080,'0.8.1'),{version:'0.8.1'})
assert.equal(manualSurface.topbarText,'等待游戏迁移确认')
assert.equal(manualSurface.overviewText,'等待游戏迁移确认')
assert.equal(manualSurface.targetVersion,'0.8.1')
assert.deepEqual(history080,historyCopy,'raw history must remain intact')
for (const phase of ['failed_safe','manual_action','preflight_failed','panel_failed_rolled_back']) {
  assert.equal(selectCurrentFullStackUpgrade(current081,{...history080,phase},'0.8.1'),null)
}
const currentScope: FullStackUpgradeStatus = {
  ...history080,operationId:'scope-081',panel:{...history080.panel,version:'0.8.1'},
  panelUpdateId:undefined,continuation:true,createdAt:'2026-11-05T00:02:00Z',authorizedAt:'2026-11-05T00:03:00Z',
}
assert.equal(selectCurrentFullStackUpgrade(current081,currentScope,'v0.8.1'),currentScope)
assert.equal(projectFullStackApply(current081,currentScope,'0.8.1')?.fullStack?.phase,'succeeded')
assert.equal(selectCurrentFullStackUpgrade(current081,{...currentScope,createdAt:history080.createdAt},'0.8.1'),null,'same-version terminal history cannot replace a later unrelated Panel apply')
const explicitlyLinked = {...currentScope,createdAt:history080.createdAt,panelUpdateId:current081.updateId}
assert.equal(selectCurrentFullStackUpgrade(current081,explicitlyLinked,'0.8.1'),explicitlyLinked,'explicit persisted identity takes precedence over timestamps')
const panelIdOnlyLinked = {...explicitlyLinked,continuation:false}
const projectedLinked = projectFullStackApply(current081,panelIdOnlyLinked,'0.8.1')
assert.equal(projectedLinked?.updateId,current081.updateId,'panelUpdateId-only association retains the accepted Panel update')
assert.equal(projectedLinked?.fromVersion,'0.8.0')
assert.equal(projectedLinked?.toVersion,'0.8.1')
const operationLinked = {...current081,operationId:currentScope.operationId}
assert.equal(selectCurrentFullStackUpgrade(operationLinked,currentScope,'0.8.1'),currentScope)
for (const phase of ['preparing','awaiting_confirmation','waiting_panel','migrating','recovering','scope_changed']) {
  const pending = {...currentScope,phase,createdAt:history080.createdAt,authorizedAt:phase==='preparing'||phase==='awaiting_confirmation'?undefined:currentScope.authorizedAt}
  assert.equal(selectCurrentFullStackUpgrade(current081,pending,'0.8.1'),pending,'current nonterminal scope is not discarded by timestamp alignment')
}
for (const createdAt of ['', 'unknown']) {
  const unaligned = {...currentScope,createdAt}
  assert.equal(selectCurrentFullStackUpgrade(current081,unaligned,'0.8.1'),unaligned,'missing legacy timestamps do not invent a stale ordering')
}
const futurePanelScope = {...currentScope,panel:{...currentScope.panel,version:'0.8.2'},phase:'waiting_panel',continuation:false}
assert.equal(selectCurrentFullStackUpgrade(current081,futurePanelScope,'0.8.1'),futurePanelScope,'a newly authorized future Panel transaction stays observable')
const currentPrepared = {...currentScope,phase:'awaiting_confirmation',authorizedAt:undefined}
assert.equal(projectFullStackApply(current081,currentPrepared,'0.8.1'),current081,'preflight alone does not manufacture Panel success')
assert.equal(selectCurrentFullStackUpgrade(null,currentScope,'0.8.1'),currentScope)
assert.equal(selectCurrentFullStackUpgrade(null,history080,'0.8.1'),null)
const recoveryHistory = {...history080,phase:'manual_action'}
assert.equal(selectHistoricalFullStackRecovery(recoveryHistory,null),recoveryHistory,'an old manual transaction remains available for its original recovery')
assert.equal(selectHistoricalFullStackRecovery(recoveryHistory,recoveryHistory),null,'a current recovery is not duplicated as history')
assert.equal(selectHistoricalFullStackRecovery(history080,null),null)
assert.equal(selectHistoricalFullStackRecovery({...recoveryHistory,authorizedAt:undefined},null),null)

const currentNoop: PanelUpdateApplyStatus = {...current081,selectedDigest:'sha256:'+'2'.repeat(64),fullStack:{phase:'not_needed',progress:100,runtimeRequired:false,currentRuntimeVerified:{updateId:current081.updateId,panel:currentScope.panel,scopeSha256:'a'.repeat(64),localSha256:'b'.repeat(64),verifiedAt:'2026-11-05T00:05:00Z'}}}
assert.equal(currentGameUpgradeComplete(currentNoop,history080,'0.8.1'),true,'unrelated old success cannot require a new migration after the current no-op')
assert.equal(currentGameUpgradeComplete(currentNoop,currentPrepared,'0.8.1'),false,'new unapproved preflight overrides the previous completed no-op')
assert.equal(currentGameUpgradeComplete(currentNoop,{...currentPrepared,phase:'preparing'},'0.8.1'),false)
assert.equal(currentGameUpgradeComplete(currentNoop,null,'0.8.1',recoveryHistory),false,'old manual recovery remains actionable')
assert.equal(currentGameUpgradeComplete(currentNoop,null,'0.8.2'),false)
assert.equal(currentGameUpgradeComplete({...currentNoop,updateId:'another-apply'},null,'0.8.1'),false)
assert.equal(currentGameUpgradeComplete({...currentNoop,selectedDigest:'sha256:'+'9'.repeat(64)},null,'0.8.1'),false)
assert.equal(currentGameUpgradeComplete(current081,currentScope,'0.8.1'),true)
assert.equal(currentGameUpgradeComplete(currentNoop,{...currentScope,phase:'manual_action'},'0.8.1'),false)
const cancelledScope = {...currentPrepared,phase:'failed_safe',errorCode:'administrator_cancelled'}
assert.equal(currentGameUpgradeComplete(currentNoop,cancelledScope,'0.8.1'),true,'a cancelled preparation does not supersede an existing completion')
const checkedScope = {...currentPrepared,phase:'succeeded',groups:[]}
assert.equal(currentGameUpgradeComplete(current081,checkedScope,'0.8.1'),true)
assert.equal(projectFullStackApply(current081,checkedScope,'0.8.1')?.fullStack?.phase,'not_needed')
assert.equal(currentGameUpgradeComplete(current081,checkedScope,'0.8.2'),false,'completion is scoped to the installed Panel')

console.log('panel update state machine tests passed')
