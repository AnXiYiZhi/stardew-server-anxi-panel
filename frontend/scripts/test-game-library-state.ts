import assert from 'node:assert/strict'
import { parseAppRoute, stardewInstallPath } from '../src/app-routes.ts'
import { setDefaultInstanceId } from '../src/api.ts'
import {
  canCreateWorld,
  gameCardNavigationIndex,
  gameLibraryBackgroundForDate,
  gameLibraryBackgroundForHour,
  gameLibraryBackgroundPreference,
  gameLibraryManualBackgroundPreference,
  gameLibraryNextBackgroundBoundary,
  initialStardewCatalogItem,
  stardewCatalogItems,
  stardewCreateInstanceRequest,
  stardewGameCardAriaLabel,
  stardewGameDestination,
  stardewInstallCardState,
  stardewInstanceDestination,
  stardewJoinAddress,
  stardewJoinAddressValue,
  stardewRequiresSave,
  stardewShouldLoadConnection,
  stardewWorldLifecycleControl,
  type StardewCatalogItem,
} from '../src/games/game-library-state.ts'
import { parseRoute, routeToPath } from '../src/games/stardew/stardew-routes.ts'
import type { Instance, InstanceState } from '../src/types.ts'

function instance(id: string, state: string, driverId = 'stardew_junimo'): Instance {
  return {
    id,
    driverId,
    driverName: 'Stardew Valley (Junimo)',
    name: id === 'stardew' ? '主世界' : `世界 ${id}`,
    state,
    stateMessage: null,
    driverPhase: 'ready',
    createdAt: '2026-08-31T00:00:00Z',
    updatedAt: '2026-08-31T01:00:00Z',
  }
}

function item(
  value: Instance,
  state: string,
  driverPhase = 'ready',
  stateMessage: string | null = null,
  hasActiveInstallJob = false,
): StardewCatalogItem {
  return {
    instance: value,
    state: {
      instanceId: value.id,
      driverId: value.driverId,
      name: value.name,
      state,
      stateMessage,
      driverPhase,
      updatedAt: value.updatedAt,
    } as InstanceState,
    stateLoading: false,
    stateError: null,
    connection: { ip: '203.0.113.24', checkedAt: '2026-08-31T01:00:00Z', cached: false, gamePort: 24642, protocol: 'udp' },
    connectionLoading: false,
    connectionError: null,
    hasActiveInstallJob,
    activeLifecycleJob: null,
  }
}

assert.deepEqual(parseAppRoute('/games', '', 'stardew'), { kind: 'games' })
assert.equal(gameLibraryBackgroundPreference(null), 'day')
assert.equal(gameLibraryBackgroundPreference('day'), 'day')
assert.equal(gameLibraryBackgroundPreference('night'), 'night')
assert.equal(gameLibraryBackgroundPreference('unknown'), 'day')
assert.equal(gameLibraryBackgroundForHour(5), 'night')
assert.equal(gameLibraryBackgroundForHour(6), 'day')
assert.equal(gameLibraryBackgroundForHour(17), 'day')
assert.equal(gameLibraryBackgroundForHour(18), 'night')
const beforeDawn = new Date(2026, 8, 4, 5, 59, 30)
const beforeDusk = new Date(2026, 8, 4, 17, 59, 30)
const afterDusk = new Date(2026, 8, 4, 18, 30, 0)
assert.equal(gameLibraryBackgroundForDate(beforeDawn), 'night')
assert.equal(gameLibraryNextBackgroundBoundary(beforeDawn), new Date(2026, 8, 4, 6, 0, 0).getTime())
assert.equal(gameLibraryNextBackgroundBoundary(beforeDusk), new Date(2026, 8, 4, 18, 0, 0).getTime())
assert.equal(gameLibraryNextBackgroundBoundary(afterDusk), new Date(2026, 8, 5, 6, 0, 0).getTime())
assert.deepEqual(gameLibraryManualBackgroundPreference('day', '2000', 1000), { background: 'day', expiresAt: 2000 })
assert.equal(gameLibraryManualBackgroundPreference('night', '1000', 1000), null)
assert.equal(gameLibraryManualBackgroundPreference('unknown', '2000', 1000), null)
assert.equal(gameLibraryManualBackgroundPreference('night', 'invalid', 1000), null)
assert.deepEqual(parseAppRoute('/games/stardew', '', 'stardew'), { kind: 'stardew-worlds' })
assert.deepEqual(parseAppRoute('/games/stardew/new', '', 'stardew'), { kind: 'stardew-new-world' })
assert.deepEqual(parseAppRoute('/games/stardew/install', '?instance=farm-02', 'stardew'), {
  kind: 'stardew-install',
  instanceId: 'stardew',
})
assert.deepEqual(parseAppRoute('/instances/farm-02/install', '', 'stardew'), {
  kind: 'stardew-install',
  instanceId: 'farm-02',
})
assert.deepEqual(parseAppRoute('/instances/farm-02/mods', '', 'stardew'), {
  kind: 'stardew-instance',
  instanceId: 'farm-02',
  route: 'mods',
})
assert.deepEqual(parseAppRoute('/instances/farm-02', '', 'stardew'), {
  kind: 'stardew-instance',
  instanceId: 'farm-02',
  route: 'overview',
})
assert.equal(stardewInstallPath(), '/games/stardew/install')
const secondWorldAuthPath = routeToPath('install', { installJobId: 'auth_second_world' }, 'farm-02')
assert.equal(secondWorldAuthPath, '/instances/farm-02/install?jobId=auth_second_world')
assert.deepEqual(parseAppRoute(secondWorldAuthPath.split('?')[0], '?jobId=auth_second_world', 'stardew'), {
  kind: 'stardew-install', instanceId: 'farm-02',
})

const defaultInstance = instance('stardew', 'uninitialized')
const secondInstance = instance('farm-02', 'running')
assert.deepEqual(stardewCatalogItems([
  defaultInstance,
  instance('minecraft', 'running', 'minecraft'),
  secondInstance,
]).map((value) => value.id), ['stardew', 'farm-02'])

assert.deepEqual(stardewGameDestination([], 'stardew'), { kind: 'unavailable' })
assert.deepEqual(stardewGameDestination([
  item(defaultInstance, 'uninitialized'),
], 'stardew'), { kind: 'install', instanceId: 'stardew' })
assert.deepEqual(stardewGameDestination([
  item(defaultInstance, 'uninitialized'),
  item(secondInstance, 'running'),
], 'stardew'), { kind: 'worlds' })
assert.equal(stardewInstanceDestination(item(defaultInstance, 'uninitialized')), 'install')
assert.equal(stardewInstanceDestination(item(secondInstance, 'running')), 'panel')
const saveRequiredInstance = instance('farm-03', 'save_required')
const saveRequiredItem = item(saveRequiredInstance, 'save_required', 'instance_ready', '请创建或导入存档')
assert.equal(stardewRequiresSave(saveRequiredItem), true)
assert.equal(stardewInstanceDestination(saveRequiredItem), 'saves')
assert.equal(stardewShouldLoadConnection(saveRequiredItem), false)
assert.equal(stardewJoinAddress(saveRequiredItem, 'panel.example.com'), '创建或上传存档后提供')
const initialSaveRequired = initialStardewCatalogItem(saveRequiredInstance)
assert.equal(initialSaveRequired.stateLoading, true)
assert.equal(initialSaveRequired.connectionLoading, false)
assert.equal(
  stardewInstanceDestination(item(instance('repair', 'error'), 'error', 'install_verification_failed', '运行文件不完整')),
  'install',
)
assert.deepEqual(stardewInstallCardState([], 'stardew', true, null), {
  kind: 'loading', label: '正在读取安装状态', actionLabel: '读取中…', actionDisabled: true, progress: null, tone: 'working',
})
const notInstalledCard = stardewInstallCardState([item(defaultInstance, 'uninitialized')], 'stardew', false, null)
assert.deepEqual(notInstalledCard, {
  kind: 'not_installed', label: '未安装', actionLabel: '开始安装', actionDisabled: false, progress: 0, tone: 'idle',
})
assert.equal(stardewGameCardAriaLabel(notInstalledCard), '星露谷物语，未安装，点击开始安装')
const installingCard = stardewInstallCardState([item(defaultInstance, 'uninitialized', 'installing', null, true)], 'stardew', false, null)
assert.deepEqual(installingCard, {
  kind: 'installing', label: '安装中 · 实时进度可查看', actionLabel: '查看安装进度', actionDisabled: false, progress: null, tone: 'working',
})
assert.equal(stardewGameCardAriaLabel(installingCard), '星露谷物语，安装中，点击查看真实安装进度')
const reinstallingItem = item(secondInstance, 'running', 'installing', null, true)
assert.equal(stardewInstallCardState([reinstallingItem], 'stardew', false, null).kind, 'installing')
assert.deepEqual(stardewGameDestination([reinstallingItem], 'stardew'), { kind: 'install', instanceId: 'farm-02' })
const installedCard = stardewInstallCardState([item(defaultInstance, 'running'), item(secondInstance, 'stopped')], 'stardew', false, null)
assert.deepEqual(installedCard, {
  kind: 'installed', label: '已安装 · 2 个世界', actionLabel: '选择世界', actionDisabled: false, progress: 100, tone: 'ready',
})
assert.equal(stardewGameCardAriaLabel(installedCard), '星露谷物语，已安装 · 2 个世界，点击选择世界')

const failedInstallItem = item(instance('failed-install', 'error'), 'error', 'steamcmd_failed', 'SteamCMD 登录失败')
const failedCard = stardewInstallCardState([failedInstallItem], 'stardew', false, null)
assert.deepEqual(failedCard, {
  kind: 'install_failed', label: '安装失败', actionLabel: '处理安装问题', actionDisabled: false, progress: null, tone: 'error',
})
assert.equal(stardewGameDestination([failedInstallItem], 'stardew').kind, 'install')

const repairInstallItem = item(instance('repair-install', 'error'), 'error', 'install_verification_failed', '运行文件不完整')
const repairCard = stardewInstallCardState([repairInstallItem], 'stardew', false, null)
assert.deepEqual(repairCard, {
  kind: 'repair_required', label: '需要修复', actionLabel: '修复安装', actionDisabled: false, progress: null, tone: 'error',
})
assert.equal(stardewGameCardAriaLabel(repairCard), '星露谷物语，需要修复，点击进入安装修复流程')

assert.equal(gameCardNavigationIndex(0, [true, true, false], 'next'), 1)
assert.equal(gameCardNavigationIndex(1, [true, true, false], 'next'), 1)
assert.equal(gameCardNavigationIndex(1, [true, true, false], 'previous'), 0)
assert.equal(gameCardNavigationIndex(1, [true, false, true], 'last'), 2)
assert.equal(gameCardNavigationIndex(2, [true, false, true], 'first'), 0)
assert.equal(gameCardNavigationIndex(0, [false, false], 'next'), 0)
for (const panelURL of ['http://121.40.29.22:8090', 'https://panel.example.com', 'http://192.168.1.20:3000']) {
  const panelAccessHost = new URL(panelURL).hostname
  const expected = `${panelAccessHost}:24642`
  assert.equal(stardewJoinAddress(item(secondInstance, 'running'), panelAccessHost), expected)
  assert.equal(stardewJoinAddressValue(item(secondInstance, 'running'), panelAccessHost), expected)
}
const ipv6Item = item(secondInstance, 'running')
ipv6Item.connection = { ...ipv6Item.connection!, gamePort: 24643 }
for (const panelAccessHost of [new URL('http://[2001:db8::1]:8090').hostname, '2001:db8::1']) {
  assert.equal(stardewJoinAddress(ipv6Item, panelAccessHost), '[2001:db8::1]:24643')
  assert.equal(stardewJoinAddressValue(ipv6Item, panelAccessHost), '[2001:db8::1]:24643')
}
assert.equal(stardewJoinAddressValue(ipv6Item, ''), null)
for (const gamePort of [0, -1, 65536, 24642.5]) {
  const invalidPortItem = item(secondInstance, 'running')
  invalidPortItem.connection = { ...invalidPortItem.connection!, gamePort }
  assert.equal(stardewJoinAddressValue(invalidPortItem, 'panel.example.com'), null)
}
const missingConnection = item(secondInstance, 'running')
missingConnection.connection = null
missingConnection.connectionError = 'network unavailable'
assert.equal(stardewJoinAddress(missingConnection, 'panel.example.com'), '暂时无法读取')
assert.equal(stardewJoinAddressValue(missingConnection, 'panel.example.com'), null)
const loadingConnection = item(secondInstance, 'running')
loadingConnection.connection = null
loadingConnection.connectionLoading = true
assert.equal(stardewJoinAddress(loadingConnection, 'panel.example.com'), '正在读取加入地址…')
assert.equal(stardewJoinAddressValue(loadingConnection, 'panel.example.com'), null)
assert.equal(stardewJoinAddress(item(defaultInstance, 'uninitialized'), 'panel.example.com'), '安装完成后提供')
assert.equal(stardewJoinAddressValue(item(defaultInstance, 'uninitialized'), 'panel.example.com'), null)

const runningWorld = item(secondInstance, 'running')
assert.deepEqual(stardewWorldLifecycleControl(runningWorld, null, 'admin'), {
  action: 'stop', label: '停止', disabled: false, busy: false, tone: 'stop',
})
assert.deepEqual(stardewWorldLifecycleControl(runningWorld, null, 'user'), {
  action: 'stop', label: '停止', disabled: true, busy: false, tone: 'stop',
})
assert.deepEqual(stardewWorldLifecycleControl(runningWorld, 'stop', 'admin'), {
  action: null, label: '停止中…', disabled: true, busy: true, tone: 'busy',
})
const stoppedWorld = item(instance('farm-04', 'stopped'), 'stopped')
assert.deepEqual(stardewWorldLifecycleControl(stoppedWorld, null, 'admin'), {
  action: 'start', label: '启动', disabled: false, busy: false, tone: 'start',
})
assert.deepEqual(stardewWorldLifecycleControl(stoppedWorld, 'start', 'admin'), {
  action: null, label: '启动中…', disabled: true, busy: true, tone: 'busy',
})
const startingWorld = item(instance('farm-05', 'starting'), 'starting')
assert.deepEqual(stardewWorldLifecycleControl(startingWorld, null, 'admin'), {
  action: null, label: '启动中…', disabled: true, busy: true, tone: 'busy',
})
const lifecycleOwnedStartingWorld = item(instance('farm-07', 'stopped'), 'stopped')
lifecycleOwnedStartingWorld.activeLifecycleJob = {
  id: 'job-start', type: 'stardew_lifecycle', status: 'running', targetType: 'instance', targetId: 'farm-07',
  createdBy: 1, createdAt: secondInstance.createdAt, startedAt: secondInstance.updatedAt, finishedAt: null,
  errorMessage: null, updatedAt: secondInstance.updatedAt,
}
assert.deepEqual(stardewWorldLifecycleControl(lifecycleOwnedStartingWorld, null, 'admin'), {
  action: null, label: '启动中…', disabled: true, busy: true, tone: 'busy',
})
const queuedStopWorld = item(instance('farm-08', 'running'), 'running', 'running')
queuedStopWorld.state = { ...queuedStopWorld.state!, uiStatus: 'ready' }
queuedStopWorld.activeLifecycleJob = {
  id: 'job-stop-queued', type: 'stardew_lifecycle', operation: 'stop', status: 'queued', targetType: 'instance', targetId: 'farm-08',
  createdBy: 1, createdAt: secondInstance.createdAt, startedAt: null, finishedAt: null,
  errorMessage: null, updatedAt: secondInstance.updatedAt,
}
assert.deepEqual(stardewWorldLifecycleControl(queuedStopWorld, null, 'admin'), {
  action: null, label: '停止中…', disabled: true, busy: true, tone: 'busy',
})
const legacyStopWorld = item(instance('farm-09', 'running'), 'running', 'running')
legacyStopWorld.state = { ...legacyStopWorld.state!, uiStatus: 'ready' }
legacyStopWorld.activeLifecycleJob = {
  id: 'job-stop-legacy', type: 'stardew_lifecycle', status: 'running', targetType: 'instance', targetId: 'farm-09',
  createdBy: null, createdAt: secondInstance.createdAt, startedAt: secondInstance.updatedAt, finishedAt: null,
  errorMessage: null, updatedAt: secondInstance.updatedAt,
}
assert.deepEqual(stardewWorldLifecycleControl(legacyStopWorld, null, 'admin'), {
  action: null, label: '停止中…', disabled: true, busy: true, tone: 'busy',
})
const stoppingWorld = item(instance('farm-06', 'running'), 'running', 'stopping')
stoppingWorld.activeLifecycleJob = {
  id: 'job-stop', type: 'stardew_lifecycle', status: 'running', targetType: 'instance', targetId: 'farm-06',
  createdBy: 1, createdAt: secondInstance.createdAt, startedAt: secondInstance.updatedAt, finishedAt: null,
  errorMessage: null, updatedAt: secondInstance.updatedAt,
}
assert.deepEqual(stardewWorldLifecycleControl(stoppingWorld, null, 'admin'), {
  action: null, label: '停止中…', disabled: true, busy: true, tone: 'busy',
})
assert.deepEqual(stardewWorldLifecycleControl(saveRequiredItem, null, 'admin'), {
  action: null, label: '需要存档', disabled: true, busy: false, tone: 'disabled',
})
assert.deepEqual(stardewWorldLifecycleControl(item(defaultInstance, 'uninitialized'), null, 'admin'), {
  action: null, label: '暂不可用', disabled: true, busy: false, tone: 'disabled',
})

assert.equal(canCreateWorld('admin'), true)
assert.equal(canCreateWorld('user'), false)
assert.deepEqual(stardewCreateInstanceRequest('  第二个世界  '), {
  name: '第二个世界',
  gameId: 'stardew',
})

setDefaultInstanceId('farm-02')
assert.equal(routeToPath('overview'), '/instances/farm-02/overview')
assert.equal(routeToPath('player-mods', { playerId: '123' }), '/instances/farm-02/player-mods?playerId=123')
assert.equal(routeToPath('jobs', undefined, 'stardew'), '/instances/stardew/jobs')
assert.equal(parseRoute('/instances/farm-02/player-mods'), 'player-mods')

setDefaultInstanceId('stardew')
console.log('game library state tests passed')
