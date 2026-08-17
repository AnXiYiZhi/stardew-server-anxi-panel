import assert from 'node:assert/strict'
import type { ModInfo } from '../src/types.ts'
import {
  filterAndSortInstalledMods,
  modMatchesInstalledQuery,
  nexusInstalledState,
  sortInstalledMods,
} from '../src/games/stardew/mod-list-utils.ts'

function mod(overrides: Partial<ModInfo>): ModInfo {
  return {
    id: overrides.folderName ?? 'Mod',
    folderName: 'Mod',
    enabled: true,
    syncKind: 'unknown',
    ...overrides,
  }
}

const older = mod({
  id: 'older',
  folderName: 'ContentPatcher',
  uniqueId: 'Pathoschild.ContentPatcher',
  name: 'Content Patcher',
  author: 'Pathoschild',
  nexusModId: 1915,
  installedAt: '2026-07-29T08:00:00Z',
})
const newer = mod({
  id: 'newer',
  folderName: 'StardewValleyExpanded',
  uniqueId: 'FlashShifter.StardewValleyExpandedCP',
  name: 'Stardew Valley Expanded',
  packageName: 'SVE bundle',
  originNexusModId: 3753,
  installedAt: '2026-07-30T08:00:00Z',
})
const legacy = mod({ id: 'legacy', folderName: 'Legacy', name: 'A Legacy Mod' })

assert.equal(modMatchesInstalledQuery(older, 'pathoschild.content'), true)
assert.equal(modMatchesInstalledQuery(older, 'pathoschildcontent'), true)
assert.equal(modMatchesInstalledQuery(older, '1915'), true)
assert.equal(modMatchesInstalledQuery(newer, '3753'), true)
assert.equal(modMatchesInstalledQuery(newer, 'flash expanded'), true)
assert.equal(modMatchesInstalledQuery(newer, 'missing'), false)
assert.equal(modMatchesInstalledQuery(newer, '-'), false, 'punctuation-only queries must not match every mod')

const source = [older, legacy, newer]
assert.deepEqual(sortInstalledMods(source).map((item) => item.id), ['newer', 'older', 'legacy'])
assert.deepEqual(source.map((item) => item.id), ['older', 'legacy', 'newer'], 'sorting must not mutate API data')
assert.deepEqual(sortInstalledMods(source, 'name-asc').map((item) => item.id), ['legacy', 'older', 'newer'])
assert.deepEqual(sortInstalledMods(source, 'name-desc').map((item) => item.id), ['newer', 'older', 'legacy'])
assert.deepEqual(filterAndSortInstalledMods(source, 'pathoschild 1915').map((item) => item.id), ['older'])

assert.deepEqual(nexusInstalledState(source, 1915), {
  installed: true,
  installedEnabled: true,
  installedFolderName: 'ContentPatcher',
  installedVersion: undefined,
})
assert.deepEqual(nexusInstalledState([], 1915), {
  installed: false,
  installedEnabled: undefined,
  installedFolderName: undefined,
  installedVersion: undefined,
}, 'refreshing after deletion must explicitly clear stale Nexus installed metadata')

const nexusPackage = mod({
  id: 'package-child',
  folderName: 'PackageChild',
  originSource: 'nexus',
  originNexusModId: 3753,
  version: '1.2.3',
})
assert.equal(nexusInstalledState([nexusPackage], 3753).installed, true)

const builtIn = mod({ id: '__smapi_runtime', folderName: 'SMAPI', name: 'SMAPI', builtIn: true })
assert.equal(sortInstalledMods([newer, builtIn])[0].id, '__smapi_runtime')

console.log('mod list search/sort tests passed')
