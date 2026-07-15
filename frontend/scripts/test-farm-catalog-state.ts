import assert from 'node:assert/strict'
import {
  catalogResponseState,
  farmCatalogIconSource,
  farmComponentsToEnable,
  farmDependencyStatusText,
  modFarmPlaceholder,
  startFarmCatalogLoad,
  canSelectModFarm,
  isSafeManualFarmTypeId,
  moddedCompatibilityShortcut,
} from '../src/games/stardew/farm-catalog-state.ts'
import { builtinFarms, isBuiltinFarmType } from '../src/games/stardew/new-game-farms.ts'
import type { FarmTypeCatalogItem, FarmTypeCatalogResponse } from '../src/types.ts'

function modFarm(overrides: Partial<FarmTypeCatalogItem> = {}): FarmTypeCatalogItem {
  return {
    id: 'FrontierFarm',
    label: '边境农场',
    description: '一大片土地',
    kind: 'modded',
    providerModId: 'FlashShifter.FrontierFarm',
    providerName: 'Frontier Farm',
    providerVersion: '1.15.11',
    enabled: true,
    confidence: 'explicit',
    conditions: [],
    conflict: false,
    dependenciesReady: null,
    selectable: false,
    requiresRuntimeValidation: true,
    iconUrl: '/api/instances/stardew/saves/farm-types/FrontierFarm/icon',
    warnings: [],
    ...overrides,
  }
}

assert.equal(builtinFarms.length, 8)
assert.deepEqual(builtinFarms.map((farm) => farm.id), ['standard', 'riverland', 'forest', 'hilltop', 'wilderness', 'fourcorners', 'beach', 'meadowlands'])
assert.equal(isBuiltinFarmType('standard'), true)
assert.equal(isBuiltinFarmType('FrontierFarm'), false)

const frontier = modFarm()
const disabled = modFarm({ id: 'DisabledFarm', label: '禁用农场', enabled: false, iconUrl: undefined })
const conflict = modFarm({ id: 'SharedFarm', label: '冲突农场', conflict: true, iconUrl: undefined })
const response: FarmTypeCatalogResponse = {
  farmTypes: [
    { ...frontier, id: 'standard', label: '标准农场', kind: 'builtin', selectable: true, requiresRuntimeValidation: false },
    frontier,
    disabled,
    conflict,
  ],
  catalogWarnings: ['partial metadata'],
  moddedCreationEnabled: false,
}
const state = catalogResponseState(response)
assert.deepEqual(state.modded.map((farm) => farm.id), ['FrontierFarm', 'DisabledFarm', 'SharedFarm'])
assert.equal(state.modded[1].enabled, false)
assert.equal(state.modded[2].conflict, true)
assert.equal(state.modded.every((farm) => farm.selectable === false), true)
assert.deepEqual(catalogResponseState({ farmTypes: response.farmTypes.slice(0, 1), catalogWarnings: [], moddedCreationEnabled: false }).modded, [])
const legacyBackendResponse = { farmTypes: response.farmTypes, catalogWarnings: [] } as unknown as FarmTypeCatalogResponse
assert.equal(catalogResponseState(legacyBackendResponse).moddedCreationEnabled, false)

assert.equal(farmCatalogIconSource(disabled, new Set()), modFarmPlaceholder)
assert.equal(farmCatalogIconSource(frontier, new Set()), frontier.iconUrl)
assert.equal(farmCatalogIconSource(frontier, new Set([frontier.iconUrl!])), modFarmPlaceholder)

const dependencyComponents = [
  { key: 'unique:Frontier', uniqueId: 'Frontier', folderName: 'Frontier', name: 'Frontier Farm', enabled: true, provider: true },
  { key: 'unique:ContentPatcher', uniqueId: 'ContentPatcher', folderName: 'Content Patcher', name: 'Content Patcher', enabled: false, provider: false },
]
const dependencyBase = {
  farmTypeId: 'FrontierFarm',
  providerModKey: 'unique:Frontier',
  requiredModKeys: ['unique:ContentPatcher'],
  optionalDependencyKeys: [],
  enabledModKeys: ['unique:Frontier'],
  disabledRequiredModKeys: ['unique:ContentPatcher'],
  missingRequiredModKeys: [],
  conflictingProviderModKeys: [],
  components: dependencyComponents,
  warnings: [],
  readiness: 'needs_enable' as const,
  dependenciesReady: false,
}
const needsEnable = modFarm({ modSelection: dependencyBase, dependenciesReady: false })
assert.equal(farmDependencyStatusText(needsEnable), '有 1 个依赖尚未启用')
assert.deepEqual(farmComponentsToEnable(needsEnable).map((component) => component.uniqueId), ['ContentPatcher'])
const missing = modFarm({ modSelection: { ...dependencyBase, readiness: 'missing_required', missingRequiredModKeys: ['FlashShifter.SVECode'] } })
assert.equal(farmDependencyStatusText(missing), '缺少：FlashShifter.SVECode')
const ready = modFarm({ dependenciesReady: true, modSelection: { ...dependencyBase, readiness: 'ready', disabledRequiredModKeys: [], enabledModKeys: ['unique:Frontier', 'unique:ContentPatcher'], dependenciesReady: true } })
assert.equal(farmDependencyStatusText(ready), '依赖完整')
assert.equal(farmDependencyStatusText(conflict), 'FarmType ID 存在冲突')
const selectableFrontier = { ...ready, selectable: true }
assert.equal(canSelectModFarm(selectableFrontier, false), false)
assert.equal(canSelectModFarm(selectableFrontier, true), true)
assert.equal(canSelectModFarm({ ...selectableFrontier, enabled: false }, true), false)
assert.equal(canSelectModFarm({ ...selectableFrontier, conflict: true }, true), false)
assert.equal(canSelectModFarm(missing, true), false)
assert.equal(moddedCompatibilityShortcut([selectableFrontier], true)?.id, 'FrontierFarm')
assert.equal(moddedCompatibilityShortcut([selectableFrontier, { ...selectableFrontier, id: 'SecondFarm' }], true), null)
assert.equal(isSafeManualFarmTypeId('FrontierFarm'), true)
assert.equal(isSafeManualFarmTypeId('Bad\nFarm'), false)

let errorState = null
startFarmCatalogLoad('stardew', (next) => { errorState = next }, async () => { throw new Error('API 500') })
await new Promise((resolve) => setTimeout(resolve, 0))
assert.equal(errorState?.error, 'API 500')
assert.deepEqual(errorState?.modded, [])

let resolveRequest: ((value: FarmTypeCatalogResponse) => void) | undefined
let capturedSignal: AbortSignal | undefined
let updatesAfterUnmount = 0
const cleanup = startFarmCatalogLoad(
  'stardew',
  () => { updatesAfterUnmount++ },
  async (_instanceId, signal) => {
    capturedSignal = signal
    return new Promise<FarmTypeCatalogResponse>((resolve) => { resolveRequest = resolve })
  },
)
cleanup()
resolveRequest?.({ farmTypes: [frontier], catalogWarnings: [], moddedCreationEnabled: false })
await new Promise((resolve) => setTimeout(resolve, 0))
assert.equal(capturedSignal?.aborted, true)
assert.equal(updatesAfterUnmount, 0)

console.log('farm catalog frontend state tests passed')
