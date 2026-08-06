import assert from 'node:assert/strict'
import {
  PLAYER_MOD_UNAVAILABLE_COPY,
  PLAYER_CJB_BANNER_TITLE,
  PLAYER_CJB_DETECTED_LABEL,
  groupPlayerModItems,
  hasCjbRisk,
  hasPlayerCjbRisk,
  isIgnoredPlayerModComparison,
  isCjbMod,
  playerModActionLabel,
  resolvePlayerModViewState,
} from '../src/games/stardew/player-mod-details.ts'
import { parseRoute, routeToPath } from '../src/games/stardew/stardew-routes.ts'
import type { PlayerModDetailsResult } from '../src/types.ts'

function details(overrides: Partial<PlayerModDetailsResult> = {}): PlayerModDetailsResult {
  return {
    instanceId: 'stardew',
    uniqueMultiplayerId: '123456789',
    hasSmapi: true,
    gameVersion: '1.6.15',
    apiVersion: '4.1.10',
    mods: [],
    contextStatus: 'reported',
    reportedAt: '2026-08-06T08:00:00Z',
    serverContext: null,
    comparison: {
      status: 'available',
      items: [],
      summary: { match: 0, missingOnClient: 0, clientOnly: 0, versionMismatch: 0 },
    },
    riskFlags: [],
    ...overrides,
  }
}

const pending = resolvePlayerModViewState(details({ contextStatus: 'pending', mods: null }), null)
assert.equal(pending.kind, 'pending')
assert.equal(pending.showComparison, false)

const unavailable = resolvePlayerModViewState(details({ contextStatus: 'unavailable', mods: null }), null)
assert.equal(unavailable.kind, 'unavailable')
assert.equal(unavailable.message, PLAYER_MOD_UNAVAILABLE_COPY)
assert.equal(unavailable.showComparison, false)
for (const forbidden of ['0 个 Mod', '完全一致', '安全']) {
  assert.equal(unavailable.message.includes(forbidden), false, `unavailable copy must not claim ${forbidden}`)
}

for (const platform of ['PC 原版', 'Android 官方客户端', 'iOS 官方客户端']) {
  const platformState = resolvePlayerModViewState(
    details({ hasSmapi: false, gameVersion: '', apiVersion: '', mods: null, contextStatus: 'unavailable' }),
    null,
  )
  assert.equal(platformState.kind, 'unavailable', `${platform} must remain a stable unavailable state`)
  assert.equal(platformState.showComparison, false)
}

const stale = resolvePlayerModViewState(
  details({
    contextStatus: 'stale',
    mods: null,
    comparison: {
      status: 'unavailable',
      unavailableReason: 'stale',
      items: [],
      summary: { match: 0, missingOnClient: 0, clientOnly: 0, versionMismatch: 0 },
    },
  }),
  null,
)
assert.equal(stale.kind, 'stale')
assert.equal(stale.showComparison, false)

const requestError = resolvePlayerModViewState(null, '网络连接中断')
assert.equal(requestError.kind, 'request_error')
assert.equal(requestError.message, '网络连接中断')

const groups = groupPlayerModItems([
  { result: 'match', uniqueId: 'Example.Mod', name: 'Example', riskFlags: [] },
  { result: 'version_mismatch', uniqueId: 'example.mod', name: 'Example duplicate', riskFlags: [] },
  {
    result: 'missing_on_client',
    uniqueId: 'Server.Only',
    name: 'Server only',
    syncKind: 'server_only',
    riskFlags: [],
  },
  { result: 'client_only', uniqueId: 'Client.Extra', name: 'Client extra', riskFlags: [] },
  { result: 'version_mismatch', uniqueId: 'Pathoschild.SMAPI', name: 'SMAPI', riskFlags: [] },
  { result: 'match', uniqueId: 'JunimoHost.Server', name: 'JunimoServer', riskFlags: [] },
  { result: 'client_only', uniqueId: 'ANXIYIZHI.STARDEWANXIPANEL.CONTROL', name: 'Panel Control', riskFlags: [] },
])
assert.equal(groups.match.length, 0)
assert.equal(groups.versionMismatch.length, 1)
assert.equal(groups.missingOnClient.length, 0)
assert.equal(groups.clientOnly.length, 1)
assert.equal(isIgnoredPlayerModComparison({ uniqueId: 'pathoschild.SMAPI' }), true)
assert.equal(isIgnoredPlayerModComparison({ uniqueId: 'JUNIMOHOST.SERVER' }), true)
assert.equal(isIgnoredPlayerModComparison({ uniqueId: 'AnXiYiZhi.StardewAnxiPanel.Control' }), true)
assert.equal(isIgnoredPlayerModComparison({ uniqueId: 'Example.Normal' }), false)

const comparisonMatrix = groupPlayerModItems([
  { result: 'match', uniqueId: 'Example.Match', name: 'Match', serverVersion: '1.0.0', clientVersion: '1.0', riskFlags: [] },
  { result: 'missing_on_client', uniqueId: 'Example.Required', name: 'Required', syncKind: 'client_required', riskFlags: [] },
  { result: 'version_mismatch', uniqueId: 'Example.Version', name: 'Version', serverVersion: '2.0.0', clientVersion: '1.0.0', riskFlags: [] },
  { result: 'client_only', uniqueId: 'Example.ClientOnly', name: 'Client only', clientVersion: '3.0.0', riskFlags: [] },
  { result: 'match', uniqueId: 'Example.ServerOnly', name: 'Server only', syncKind: 'server_only', riskFlags: [] },
])
assert.deepEqual(
  {
    match: comparisonMatrix.match.length,
    missing: comparisonMatrix.missingOnClient.length,
    mismatch: comparisonMatrix.versionMismatch.length,
    extra: comparisonMatrix.clientOnly.length,
  },
  { match: 2, missing: 1, mismatch: 1, extra: 1 },
)
assert.equal(comparisonMatrix.missingOnClient.some((item) => item.syncKind === 'server_only'), false)

const longVersion = '2026.08.06-build+'.repeat(20)
const boundedDuplicate = groupPlayerModItems([
  { result: 'match', uniqueId: ' Example.Long ', name: 'L'.repeat(256), clientVersion: longVersion, riskFlags: [] },
  { result: 'version_mismatch', uniqueId: 'example.long', name: 'duplicate', clientVersion: longVersion, riskFlags: [] },
])
assert.equal(boundedDuplicate.versionMismatch.length, 1)
assert.equal(boundedDuplicate.match.length, 0)
assert.equal(boundedDuplicate.versionMismatch[0]?.clientVersion, longVersion)

assert.equal(isCjbMod({ uniqueId: 'cjbOK.CHEATSMENU' }), true)
assert.equal(isCjbMod({ uniqueId: 'CJBok.ItemSpawner' }), true)
assert.equal(isCjbMod({ uniqueId: 'NotCJB.CheatsMenu' }), false)
assert.equal(isCjbMod({ uniqueId: 'ModifiedAuthor.CheatsMenu' }), false)
assert.equal(PLAYER_CJB_DETECTED_LABEL, '检测到 CJB 作弊')
assert.equal(PLAYER_CJB_BANNER_TITLE, '检测到该玩家使用了 CJB 作弊工具')
assert.equal(hasPlayerCjbRisk({ modRiskFlags: [' CJB '] }), true)
assert.equal(hasPlayerCjbRisk({ modRiskFlags: ['other'] }), false)
assert.equal(hasPlayerCjbRisk({}), false)
assert.equal(playerModActionLabel({ modRiskFlags: ['cjb'] }), PLAYER_CJB_DETECTED_LABEL)
assert.equal(playerModActionLabel({ modRiskFlags: [] }), '查看上报 Mod')
assert.equal(
  hasCjbRisk(
    details({
      comparison: {
        status: 'available',
        summary: { match: 0, missingOnClient: 0, clientOnly: 1, versionMismatch: 0 },
        items: [{ result: 'client_only', uniqueId: 'CJBoK.ItemSpawner', name: 'CJB Item Spawner', riskFlags: ['cjb'] }],
      },
    }),
  ),
  true,
)
assert.equal(
  hasCjbRisk(
    details({
      comparison: {
        status: 'available',
        summary: { match: 0, missingOnClient: 0, clientOnly: 1, versionMismatch: 0 },
        items: [{ result: 'client_only', uniqueId: 'CJBok.CheatsMenu', name: 'CJB Cheats Menu', riskFlags: ['cjb'] }],
      },
    }),
  ),
  true,
)

assert.equal(routeToPath('player-mods', { playerId: '-123456789' }), '/instances/stardew/player-mods?playerId=-123456789')
assert.equal(parseRoute('/instances/stardew/player-mods'), 'player-mods')

console.log('player Mod detail state tests passed')
