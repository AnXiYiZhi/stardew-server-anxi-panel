// 临时 QA harness：mock fetch + 真实 StardewPanel 全壳，用于布局对比截图。用完删除。
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './App.css'
import './games/stardew/stardew-theme.css'
import { StardewPanel } from './games/stardew/StardewPanel'
import { StardewMobileShell } from './games/stardew/StardewMobileShell'
import { PanelUpdateProvider } from './games/stardew/PanelUpdateProvider'
import type { CurrentUser } from './types'

const params = new URLSearchParams(location.search)
const STATE = params.get('state') || 'running'
const SHELL = params.get('shell') || 'desktop'
const UPDATE = params.get('update') || 'latest'
const APPLY = params.get('apply') || ''
const JUNIMO_WORKFLOW = params.get('junimoWorkflow') || ''
const JUNIMO_CONFIG = params.get('junimoConfig') || ''
const ROLE = params.get('role') === 'user' ? 'user' : 'admin'
if (JUNIMO_WORKFLOW === 'race-retry') window.confirm = () => true

const now = new Date('2025-05-21T14:28:36+08:00')
const iso = (mins: number) => new Date(now.getTime() - mins * 60000).toISOString()

const players = [
  { name: 'AnxiPlayer', role: 'host', isHost: true, locationDisplayName: '农场 · 春季 12 日', tileX: 64, tileY: 11, uniqueMultiplayerId: '7f9a2b10', status: 'online', ping: 36, farmMoney: 128640, personalMoney: 28230, onlineSeconds: 8000 },
  { name: '小鸡快跑', locationDisplayName: '温室', tileX: 12, tileY: 30, uniqueMultiplayerId: '3c2e9d55', status: 'online', ping: 48, farmMoney: 52310, personalMoney: 41780, onlineSeconds: 6420 },
  { name: '星露谷旅人', locationDisplayName: '矿洞湖', uniqueMultiplayerId: 'a1b7c3f8', status: 'online', ping: 62, farmMoney: 34820, personalMoney: 29150, onlineSeconds: 3480 },
  { name: 'WinterBreeze', locationDisplayName: '等待加入…', uniqueMultiplayerId: 'd4e5f6a1', status: 'waiting', ping: null },
  { name: 'PendingGuest', locationDisplayName: '登录中…', uniqueMultiplayerId: 'f2a8c410', status: 'online', isAuthenticated: false, ping: 50 },
]

const recentPlayerEvents = [
  { id: 'evt-1', type: 'joined', playerName: '小鸡快跑', uniqueMultiplayerId: '3c2e9d55', locationDisplayName: '温室', at: iso(6), message: '小鸡快跑 加入了服务器。' },
  { id: 'evt-2', type: 'seen', playerName: 'AnxiPlayer', uniqueMultiplayerId: '7f9a2b10', isHost: true, locationDisplayName: '农场', at: iso(18), message: '首次记录玩家 AnxiPlayer 在线。' },
  { id: 'evt-3', type: 'joined', playerName: '星露谷旅人', uniqueMultiplayerId: 'a1b7c3f8', locationDisplayName: '矿洞湖', at: iso(44), message: '星露谷旅人 加入了服务器。' },
  { id: 'evt-4', type: 'left', playerName: 'WinterBreeze', uniqueMultiplayerId: 'd4e5f6a1', locationDisplayName: '巴士站', at: iso(1440), message: 'WinterBreeze 离开了服务器。' },
  { id: 'evt-5', type: 'joined', playerName: 'JunimoGuest', uniqueMultiplayerId: 'e7f8a9b0', locationDisplayName: '鹈鹕镇', at: iso(2880), message: 'JunimoGuest 加入了服务器。' },
  { id: 'evt-6', type: 'left', playerName: 'JunimoGuest', uniqueMultiplayerId: 'e7f8a9b0', locationDisplayName: '鹈鹕镇', at: iso(2940), message: 'JunimoGuest 离开了服务器。' },
]

const jobs = [
  { id: 'job_01JH8A3K7M2QZ8S1', type: 'save_auto', displayName: '存档自动保存', status: 'running', targetType: 'instance', targetId: 'stardew', createdBy: 1, createdAt: iso(3), startedAt: iso(3), finishedAt: null, errorMessage: null, updatedAt: iso(0) },
  { id: 'job_01JH7YB3VQ1J9R4C', type: 'mod_update_check', displayName: '模组更新检查', status: 'succeeded', targetType: 'instance', targetId: 'stardew', createdBy: 1, createdAt: iso(16), startedAt: iso(16), finishedAt: iso(15), errorMessage: null, updatedAt: iso(15) },
  { id: 'job_01JH6W3M9Z2D8K1E', type: 'player_backup', displayName: '玩家数据备份', status: 'succeeded', targetType: 'instance', targetId: 'stardew', createdBy: 1, createdAt: iso(30), startedAt: iso(30), finishedAt: iso(29), errorMessage: null, updatedAt: iso(29) },
  { id: 'job_01JH5T3N6B9F2P7Q', type: 'log_cleanup', displayName: '日志清理', status: 'succeeded', targetType: 'instance', targetId: 'stardew', createdBy: 1, createdAt: iso(56), startedAt: iso(56), finishedAt: iso(55), errorMessage: null, updatedAt: iso(55) },
  { id: 'job_01JH4R3V8D1M6J2P', type: 'mod_remote_install', displayName: '模组安装：UI Info Suite 2', status: 'failed', targetType: 'instance', targetId: 'stardew', createdBy: 1, createdAt: iso(126), startedAt: iso(126), finishedAt: iso(125), errorMessage: '下载失败', updatedAt: iso(125) },
  { id: 'job_01JH3P3Q7K2S9M4T', type: 'server_restart', displayName: '服务器重启', status: 'succeeded', targetType: 'instance', targetId: 'stardew', createdBy: 1, createdAt: iso(160), startedAt: iso(160), finishedAt: iso(159), errorMessage: null, updatedAt: iso(159) },
  { id: 'job_01JH2L3B6F9Q1N8D', type: 'save_repair', displayName: '存档修复', status: 'failed', targetType: 'instance', targetId: 'stardew', createdBy: 1, createdAt: iso(247), startedAt: iso(247), finishedAt: iso(246), errorMessage: '存档损坏', updatedAt: iso(246) },
]

const saves = {
  activeSaveName: 'AnxiFarm',
  saves: [
    { name: 'AnxiFarm', farmerName: 'AnxiPlayer', farmName: 'AnxiFarm', gameYear: 1, gameSeason: 'spring', gameDay: 12, farmType: '标准农场', fileSizeBytes: 24.6 * 1048576, modifiedAt: iso(0), isActive: true },
    { name: 'GreenFarm', farmerName: '', farmName: 'GreenFarm', gameYear: 1, gameSeason: 'spring', gameDay: 10, farmType: '标准农场', fileSizeBytes: 21.3 * 1048576, modifiedAt: iso(1440) },
    { name: 'SunnyDay', farmName: 'SunnyDay', gameYear: 1, gameSeason: 'summer', gameDay: 5, farmType: '河边农场', fileSizeBytes: 18.7 * 1048576, modifiedAt: iso(2880) },
    { name: 'MoonLight', farmName: 'MoonLight', gameYear: 1, gameSeason: 'fall', gameDay: 8, farmType: '森林农场', fileSizeBytes: 22.1 * 1048576, modifiedAt: iso(4320) },
  ],
}

const mods = {
  restartRequired: false,
  mods: Array.from({ length: 37 }, (_, i) => ({
    id: `mod_${i}`, uniqueId: `Author.Mod${i}`, name: `示例模组 ${i + 1}`, version: '1.0.0', author: 'Pathoschild',
    folderName: `Mod${i}`, enabled: i % 9 !== 0, canToggle: true, syncKind: 'client_optional', builtIn: i < 1,
  })),
}

const health = {
  status: 'ok',
  checks: [
    { name: 'Docker 服务', status: 'ok', message: 'Docker 服务正在运行' },
    { name: 'Docker Compose', status: 'ok', message: '版本 2.24.6' },
    { name: '数据目录', status: 'ok', message: '/data/stardew | 可用 215.8 GB' },
    { name: '实例目录', status: 'ok', message: '/data/stardew/instances/AnxiFarm' },
    { name: 'Compose 文件', status: 'ok', message: 'docker-compose.yml 存在' },
    { name: '启动存档', status: 'ok', message: 'AnxiFarm (GreenFarm_春季第12天)' },
  ],
}

const metrics = {
  instanceId: 'stardew', service: 'stardew',
  sample: { timestamp: now.toISOString(), cpuPercent: 18, memoryPercent: 42, memoryUsedBytes: 3.4 * 1073741824, memoryLimitBytes: 8 * 1073741824, diskPercent: 31, diskUsedBytes: 42.6 * 1073741824, diskTotalBytes: 128 * 1073741824, containerRunning: true },
}

const users = {
  users: [
    { id: 1, username: '管理员', role: 'admin', isSuperAdmin: true, isActive: true, createdAt: iso(9000), updatedAt: iso(3), lastLoginAt: iso(3) },
    { id: 2, username: 'junimo', role: 'admin', isSuperAdmin: false, isActive: true, createdAt: iso(9000), updatedAt: iso(300), lastLoginAt: iso(300) },
    { id: 3, username: 'player_one', role: 'user', isSuperAdmin: false, isActive: true, createdAt: iso(9000), updatedAt: iso(1000), lastLoginAt: iso(1000) },
    { id: 4, username: 'farmer_cat', role: 'user', isSuperAdmin: false, isActive: false, createdAt: iso(9000), updatedAt: iso(4000), lastLoginAt: iso(4000) },
    { id: 5, username: 'test_user', role: 'user', isSuperAdmin: false, isActive: true, createdAt: iso(9000), updatedAt: iso(2000), lastLoginAt: iso(2000) },
  ],
}

const audit = {
  total: 126, limit: 50, offset: 0,
  logs: Array.from({ length: 7 }, (_, i) => ({ id: 200 - i, actorUserId: 1, actorName: i % 2 ? 'junimo' : '管理员', action: ['登录面板', '更新用户角色', '安装模组', '修改服务器配置', '备份存档', '登录面板', '清理日志'][i], targetType: 'system', targetId: '—', metadataJson: '{}', ipAddress: '127.0.0.1', userAgent: '', createdAt: iso(i * 30) })),
}

const commands = { commands: [ { name: 'help', description: '显示帮助' }, { name: 'save', description: '保存存档' } ] }
const backups = {
  backups: Array.from({ length: 5 }, (_, i) => {
    const gameDay = 12 - i
    return {
      name: `auto_AnxiFarm_${String(gameDay).padStart(6, '0')}.zip`,
      saveName: 'AnxiFarm',
      kind: 'auto',
      size: (24.6 - i * 0.3) * 1048576,
      createdAt: iso((i + 1) * 1440),
      farmName: 'AnxiFarm',
      farmerName: 'AnxiPlayer',
      gameYear: 1,
      gameSeason: 'spring',
      gameDay,
      gameDayOrdinal: gameDay,
    }
  }),
}
const backupPolicy = { policy: { gameSaveBackups: true, retainGameDays: 5 } }
const restartSchedule = { schedule: { instanceId: 'stardew', enabled: false, shutdownTime: '04:00', startupTime: '04:10', timezone: 'Asia/Shanghai', warningMinutes: [10, 5, 1], backupBeforeShutdown: true, skipIfPlayersOnline: true } }
const passwordStatus = { enabled: true, authenticatedCount: 3, pendingCount: 1, timeoutSeconds: 60, maxAttempts: 3, passwordBridgeAvailable: true }
const nexusSettings = { configured: true, hasApiKey: true, extensionConnected: true }
const vncConfig = { vncPort: '24643' }
const rendering = { fps: 30 }
const serverPassword = { serverPassword: '' }
const serverRuntimeSettings = { cabinStrategy: 'CabinStack', existingCabinBehavior: 'KeepExisting', networkBroadcastPeriod: 1 }
const panelUpdate = {
  currentVersion: '0.1.14', currentCommit: '3f7a9c2', currentBuildDate: '2026-07-13T12:00:00Z',
  latestVersion: UPDATE === 'available' ? 'v0.1.15' : 'v0.1.14',
  updateAvailable: UPDATE === 'available',
  releaseUrl: 'https://github.com/anxiyizhi/stardew-server-anxi-panel/releases/tag/v0.1.15',
  publishedAt: '2026-07-12T08:00:00Z', checkedAt: '2026-07-13T12:00:00Z',
  checkStatus: UPDATE === 'error' ? 'error' : 'ok', checkError: UPDATE === 'error' ? '访问 GitHub Release 失败' : '',
}
const applyStatus = APPLY ? {
  updateId: 'qa-panel-update', phase: APPLY === 'offline' || APPLY === 'reconnect-success' ? 'recreating' : APPLY, progress: APPLY === 'backing_up' ? 15 : APPLY === 'pulling' ? 35 : APPLY === 'recreating' || APPLY === 'offline' || APPLY === 'reconnect-success' ? 65 : APPLY === 'waiting_health' ? 82 : APPLY === 'rolling_back' ? 88 : 100,
  fromVersion: '0.1.14', toVersion: '0.1.15', originalImage: '', originalDigest: '', selectedImage: '', selectedDigest: '', errorCode: APPLY === 'failed_rolled_back' ? 'health_check_failed' : '', error: APPLY === 'failed_rolled_back' ? '新版本未通过健康检查' : '', result: APPLY === 'succeeded' ? '面板升级并验收成功' : APPLY === 'failed_rolled_back' ? '已自动恢复并验收旧面板' : '', logs: [], startedAt: iso(5), updatedAt: iso(0), finishedAt: APPLY === 'succeeded' || APPLY === 'failed_rolled_back' ? iso(0) : null,
} : null
const dryRunStatus = {
  id: 'qa-dry-run', phase: 'succeeded', targetVersion: '0.1.15', targetImage: 'ghcr.io/anxiyizhi/stardew-server-anxi-panel:0.1.15',
  capability: { supported: true, reason: '标准 Compose 部署可安全升级', code: 'supported', composeProject: 'anxi-panel', composeFile: '', installDir: '', currentContainer: 'anxi-panel', currentImage: 'ghcr.io/anxiyizhi/stardew-server-anxi-panel:0.1.14', dataMount: '', dockerAvailable: true, composeAvailable: true },
  logs: [], startedAt: iso(3), updatedAt: iso(0), finishedAt: iso(0), errorCode: '', error: '',
}
const junimoUpdate = {
  available: JUNIMO_CONFIG !== 'repairable', supported: JUNIMO_CONFIG !== 'repairable', repairable: JUNIMO_CONFIG === 'repairable',
  status: JUNIMO_CONFIG === 'repairable' ? 'invalid_config' : 'update_available',
  code: JUNIMO_CONFIG === 'repairable' ? 'invalid_config/image_candidates' : 'update_available',
  reason: JUNIMO_CONFIG === 'repairable' ? '实例运行组件候选镜像配置无效或 tag 不一致。' : '',
  repairCode: JUNIMO_CONFIG === 'repairable' ? 'repairable/legacy_candidates' : undefined,
  repairReason: JUNIMO_CONFIG === 'repairable' ? '检测到可信旧版候选列表；可先私有备份原配置，再规范化为当前版本的可信候选并继续升级。' : undefined,
  current: {
    server: { image: 'dockerproxy.net/sdvd/server:1.5.0-preview.121', tag: '1.5.0-preview.121' },
    steamAuth: { image: 'docker.1ms.run/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2', tag: '1.5.0-anxi.2' },
  },
  recommended: {
    status: 'recommended', tested: true, stackVersion: 'junimo-1.5.0-preview.125_auth-1.5.0-anxi.2', channel: 'preview', minimumPanelVersion: '0.3.2', runtimeUpdatePolicy: 'required',
    server: { image: 'sdvd/server:1.5.0-preview.125', images: ['sdvd/server:1.5.0-preview.125'], tag: '1.5.0-preview.125' },
    steamAuth: { image: 'anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2', images: ['anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2'], tag: '1.5.0-anxi.2' },
  },
  releaseNotes: ['preview.121 可继续使用；本次升级由管理员自愿执行。'],
}
const runtimeComponents = {
  status: 'up_to_date', reason: '游戏版本与联机运行库均匹配推荐组合。',
  current: {
    game: { appId: '413150', buildId: '16826371', stateFlags: '4', manifestPath: 'steamapps/appmanifest_413150.acf', installDir: 'Stardew Valley' },
    sdk: { appId: '1007', buildId: '20939719', stateFlags: '4', manifestPath: '.steam-sdk/steamapps/appmanifest_1007.acf', installDir: 'Steamworks SDK Redist' },
  },
  recommended: {
    status: 'recommended', tested: true, stackVersion: 'junimo-1.5.0-preview.125_auth-1.5.0-anxi.2_game-16826371_sdk-20939719_smapi-4.5.2', channel: 'preview', minimumPanelVersion: '0.3.2', runtimeUpdatePolicy: 'required',
    game: { buildId: '16826371', manifestVersion: 'stardew-1.6.15-public', notes: [] },
    sdk: { buildId: '20939719', manifestVersion: 'steamworks-sdk-redist-public', notes: [] },
  },
}
const smapiUpdate = {
  available: false, supported: true, status: 'up_to_date', reason: 'SMAPI 已匹配推荐版本。',
  current: { present: true, valid: true, version: '4.5.2', versionSource: 'StardewModdingAPI.dll' },
  recommended: { version: '4.5.2', sha256: 'qa-sha256', archiveBytes: 41943040, compatibility: { gameBuildId: '16826371', sdkBuildId: '20939719', junimoVersion: '1.5.0-preview.125', steamAuthVersion: '1.5.0-anxi.2', controlVersion: '0.1.0', commandResultVersion: 1 } },
}
const idleWorkflow = { phase: 'idle', progress: 0, target: {}, selected: {}, checks: [], warnings: [], logs: [] }
const idleJunimoWorkflow = { ...idleWorkflow, target: { server: {}, steamAuth: {} }, selected: { server: {}, steamAuth: {} } }
const junimoDryRunWorkflow = JUNIMO_WORKFLOW === 'race-retry' ? {
  ...idleJunimoWorkflow, dryRunId: 'qa-old-dry-run', phase: 'succeeded', progress: 100,
  startedAt: '2026-07-14T15:20:00Z', finishedAt: '2026-07-14T15:20:02Z',
} : JUNIMO_WORKFLOW === 'pulling' ? {
  ...idleJunimoWorkflow, dryRunId: 'qa-junimo-dry-run', phase: 'pulling_server', progress: 61,
  download: { component: 'server', image: 'dockerproxy.net/sdvd/server:1.5.0-preview.125', doneLayers: 5, totalLayers: 8, percent: 62 },
} : JUNIMO_WORKFLOW === 'rollback-failed' ? { ...idleJunimoWorkflow, phase: 'succeeded', progress: 100 } : idleJunimoWorkflow
const junimoApplyWorkflow = JUNIMO_WORKFLOW === 'race-retry' ? {
  ...idleJunimoWorkflow, applyId: 'qa-old-apply', phase: 'failed_rolled_back', progress: 100,
  causeCode: 'junimo_contract_not_ready', causeError: '旧任务未通过验收，已恢复原版本。',
  startedAt: '2026-07-14T15:10:00Z', finishedAt: '2026-07-14T15:15:00Z',
} : JUNIMO_WORKFLOW === 'rollback-failed' ? {
  ...idleJunimoWorkflow, applyId: 'qa-junimo-apply', phase: 'rollback_failed', progress: 100,
  causeCode: 'junimo_health_not_ready', causeError: '新版 Junimo 健康检查未在时限内就绪。',
  rollbackCode: 'rollback_verify_server_failed', rollbackError: '升级前的 Junimo server 未能在验收时限内恢复就绪。',
  manualAction: '保留恢复材料并核对当前旧服务状态。',
} : idleJunimoWorkflow

function jsonRes(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
}

const qaJunimoEvents: string[] = []
type QAJunimoWorkflow = Record<string, unknown> & {
  phase: string
  progress: number
  dryRunId?: string
  applyId?: string
  jobId?: string
}
let qaRaceDryRun: QAJunimoWorkflow = junimoDryRunWorkflow
let qaRaceApply: QAJunimoWorkflow = junimoApplyWorkflow
let qaRaceDryRunGets = 0
let qaRaceApplyGets = 0

function recordQAJunimoEvent(event: string) {
  qaJunimoEvents.push(event)
  let output = document.getElementById('qa-junimo-events')
  if (!output) {
    output = document.createElement('output')
    output.id = 'qa-junimo-events'
    output.hidden = true
    document.body.appendChild(output)
  }
  output.textContent = qaJunimoEvents.join(',')
}

const routes: Array<[RegExp, unknown]> = [
  [/\/junimo-update\/dry-run$/, junimoDryRunWorkflow],
  [/\/junimo-update\/apply$/, junimoApplyWorkflow],
  [/\/junimo-update$/, junimoUpdate],
  [/\/runtime-components\/preflight$/, idleWorkflow],
  [/\/runtime-components$/, runtimeComponents],
  [/\/smapi-update\/dry-run$/, idleWorkflow],
  [/\/smapi-update\/apply$/, idleWorkflow],
  [/\/smapi-update$/, smapiUpdate],
  [/\/api\/system\/update\/apply$/, applyStatus],
  [/\/api\/system\/update\/dry-run$/, dryRunStatus],
  [/\/api\/system\/update(?:\/check)?$/, panelUpdate],
  [/\/api\/version$/, { version: APPLY === 'succeeded' ? '0.1.15' : '0.1.14', commit: '3f7a9c2', buildDate: '2025-05-21 14:28:36' }],
  [/\/state$/, { instanceId: 'stardew', driverId: 'stardew_junimo', name: 'AnxiFarm', state: STATE, stateMessage: null, driverPhase: STATE, updatedAt: iso(2) }],
  [/\/metrics$/, metrics],
  [/\/players$/, { instanceId: 'stardew', state: STATE, source: 'junimo', onlineCount: 3, maxPlayers: 12, players, parseStatus: 'exact', updatedAt: iso(0), recentEvents: recentPlayerEvents, rawInfo: JSON.stringify({ server: 'AnxiFarm', uptime: '2天 4小时 12分', version: '1.6.15 (Stardew Valley)', players_online: 3, max_players: 8, junimo_note: '此信息为 Junimo 协议原始输出，用于调试与集成。', timestamp: '2025-05-21T14:28:36+08:00' }, null, 2) }],
  [/\/jobs$/, { jobs }],
  [/\/jobs\/[^/]+\/logs/, { logs: Array.from({ length: 16 }, (_, i) => ({ jobId: 'x', sequence: 2042 + i, level: i === 4 ? 'WARN' : 'INFO', message: `复制存档文件… (${i + 1}/16)`, timestamp: iso(0) })) }],
  [/\/jobs\/[^/]+$/, { job: jobs[0] }],
  [/\/saves\/backups\/policy$/, backupPolicy],
  [/\/saves\/backups$/, backups],
  [/\/saves\/preflight$/, { canCreate: true, canUpload: true, warnings: [] }],
  [/\/saves$/, saves],
  [/\/mods$/, mods],
  [/\/mods\/nexus\/install$/, { jobId: 'job_mobile_nexus_install' }],
  [/\/mods\/nexus\/extension\/download$/, {}],
  [/\/health\/diagnostics$/, health],
  [/\/invite-code$/, { inviteCode: STATE === 'running' ? 'ANXI-FARM-2024' : '' }],
  [/\/restart-schedule$/, restartSchedule],
  [/\/config\/vnc-port$/, vncConfig],
  [/\/config\/server-password$/, serverPassword],
  [/\/config\/server-runtime-settings$/, serverRuntimeSettings],
  [/\/rendering$/, rendering],
  [/\/mods\/nexus\/search/, { query: 'ui', page: 1, pageSize: 20, total: 1248, hasMore: true, results: [
    { modId: 2400, name: 'SMAPI - Stardew Modding API', summary: 'Stardew Modding API 的实现，所有模组的必要依赖。', author: 'Pathoschild', version: '4.0.8', updatedAt: '2024-05-16', endorsementCount: 3800, downloadCount: 7200000, nexusUrl: 'https://x', installed: false, installedEnabled: false, requiredMods: [] },
    { modId: 1915, name: 'Content Patcher', summary: '通过内容包修改游戏数据、图像、地图等，无需解压原始文件。', author: 'Pathoschild', version: '2.3.3', updatedAt: '2024-04-28', endorsementCount: 2600, downloadCount: 6100000, nexusUrl: 'https://x', installed: false, installedEnabled: false, requiredMods: [{ modId: 2400, name: 'SMAPI', nexusUrl: 'https://x', installed: true, installedEnabled: true }] },
    { modId: 1150, name: 'UI Info Suite 2', summary: '在游戏 UI 中显示更多有用信息和工具提示。', author: 'Annosz', version: '2.2.3', updatedAt: '2024-03-20', endorsementCount: 1400, downloadCount: 2600000, nexusUrl: 'https://x', installed: false, installedEnabled: false, requiredMods: [{ modId: 1915, name: 'Content Patcher', nexusUrl: 'https://x', installed: false, installedEnabled: false }] },
    { modId: 541, name: 'Lookup Anything', summary: '在游戏中检查物品、NPC、地名等的详细信息和 ID。', author: 'Pathoschild', version: '1.40.5', updatedAt: '2024-04-12', endorsementCount: 2000, downloadCount: 1400000, nexusUrl: 'https://x', installed: false, installedEnabled: false, requiredMods: [] },
  ] }],
  [/\/password-status$/, passwordStatus],
  [/\/settings\/nexus$/, nexusSettings],
  [/\/api\/users$/, users],
  [/\/api\/audit-logs/, audit],
  [/\/commands$/, commands],
  [/\/install-options$/, { imageTagOptions: [{ tag: '1.6.15', label: 'v1.6.15 (Stable)', recommended: true, isLatest: true }, { tag: '1.6.14', label: 'v1.6.14', recommended: false }] }],
  [/\/auth\/me$/, { user: { id: 1, username: '管理员', role: 'admin', isSuperAdmin: true } }],
]

const realFetch = window.fetch.bind(window)
let applyFetchCount = 0
window.fetch = async (input: RequestInfo | URL, init?: RequestInit) => {
  const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
  const method = (init?.method ?? (input instanceof Request ? input.method : 'GET')).toUpperCase()
  const path = url.split('?')[0]
  if (JUNIMO_WORKFLOW === 'race-retry' && path.endsWith('/junimo-update/dry-run')) {
    if (method === 'POST') {
      recordQAJunimoEvent('dry-run:POST')
      qaRaceDryRun = {
        ...idleJunimoWorkflow, dryRunId: 'qa-new-dry-run', jobId: 'qa-new-dry-run-job', phase: 'checking', progress: 5,
        startedAt: '2026-07-14T15:47:28Z', updatedAt: '2026-07-14T15:47:28Z',
      }
      await new Promise((resolve) => window.setTimeout(resolve, 250))
      return jsonRes(qaRaceDryRun)
    }
    recordQAJunimoEvent('dry-run:GET')
    qaRaceDryRunGets += 1
    if (qaRaceDryRunGets >= 1 && qaRaceDryRun.dryRunId === 'qa-new-dry-run') {
      qaRaceDryRun = { ...qaRaceDryRun, phase: 'succeeded', progress: 100, updatedAt: '2026-07-14T15:47:30Z', finishedAt: '2026-07-14T15:47:30Z' }
    }
    return jsonRes(qaRaceDryRun)
  }
  if (JUNIMO_WORKFLOW === 'race-retry' && path.endsWith('/junimo-update/apply')) {
    if (method === 'POST') {
      if (qaRaceDryRun.dryRunId !== 'qa-new-dry-run' || qaRaceDryRun.phase !== 'succeeded') {
        recordQAJunimoEvent('apply:POST-rejected')
        return jsonRes({ code: 'runtime_update_busy', message: '新预检尚未完成。' }, 409)
      }
      recordQAJunimoEvent('apply:POST')
      qaRaceApply = {
        ...idleJunimoWorkflow, applyId: 'qa-new-apply', jobId: 'qa-new-apply-job', phase: 'checking', progress: 5,
        startedAt: '2026-07-14T15:47:31Z', updatedAt: '2026-07-14T15:47:31Z',
      }
      return jsonRes(qaRaceApply)
    }
    recordQAJunimoEvent('apply:GET')
    qaRaceApplyGets += 1
    if (qaRaceApplyGets >= 1 && qaRaceApply.applyId === 'qa-new-apply') {
      qaRaceApply = { ...qaRaceApply, phase: 'succeeded', progress: 100, updatedAt: '2026-07-14T15:47:33Z', finishedAt: '2026-07-14T15:47:33Z' }
    }
    return jsonRes(qaRaceApply)
  }
  if (APPLY === 'offline' && url.includes('/api/system/update/apply')) {
    applyFetchCount += 1
    if (applyFetchCount > 2) throw new TypeError('mock panel offline')
  }
  if (APPLY === 'offline' && (url.endsWith('/health') || url.includes('/api/version')) && applyFetchCount > 2) {
    throw new TypeError('mock panel offline')
  }
  if (APPLY === 'reconnect-success') {
    if (url.includes('/api/system/update/apply')) {
      applyFetchCount += 1
      if (applyFetchCount === 3 || applyFetchCount === 4) throw new TypeError('mock expected restart')
      if (applyFetchCount > 4) return jsonRes({ ...applyStatus, phase: 'succeeded', progress: 100, result: '面板升级并验收成功', finishedAt: iso(0) })
    }
    if (url.endsWith('/health')) return jsonRes({ status: 'ok' })
    if (url.includes('/api/version') && applyFetchCount > 4) return jsonRes({ version: '0.1.15', commit: 'new-build', buildDate: now.toISOString() })
  }
  if (url.includes('/api/')) {
    for (const [re, body] of routes) {
      if (re.test(url.split('?')[0]) || re.test(url)) return jsonRes(body)
    }
    return jsonRes({}, 200)
  }
  return realFetch(input as RequestInfo, init)
}

class NoopES extends EventTarget {
  close() {}
  onerror: unknown = null
}
;(window as unknown as { EventSource: unknown }).EventSource = NoopES

const mockUser: CurrentUser = { id: 1, username: ROLE === 'admin' ? '管理员' : '普通玩家', role: ROLE, isSuperAdmin: ROLE === 'admin' }

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <PanelUpdateProvider user={mockUser}>
      {SHELL === 'mobile' ? <StardewMobileShell user={mockUser} /> : <StardewPanel user={mockUser} onLogout={() => {}} />}
    </PanelUpdateProvider>
  </StrictMode>,
)
