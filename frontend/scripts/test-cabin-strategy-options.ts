import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import {
  DEFAULT_SERVER_RUNTIME_SETTINGS,
  normalizeServerRuntimeSettings,
} from '../src/games/stardew/server-runtime-settings-state.ts'

const surfaces = [
  ['desktop', new URL('../src/games/stardew/pages/ServerControlPage.tsx', import.meta.url)],
  ['mobile', new URL('../src/games/stardew/mobile/MobileControlPage.tsx', import.meta.url)],
] as const

for (const [surface, path] of surfaces) {
  const source = readFileSync(path, 'utf8')
  assert.match(source, /<ServerRuntimeSettingsDialog/, `${surface} must use the shared runtime settings dialog`)
}

const dialog = readFileSync(new URL('../src/games/stardew/ServerRuntimeSettingsDialog.tsx', import.meta.url), 'utf8')
const newGameCreator = readFileSync(new URL('../src/games/stardew/NewGameCreator.tsx', import.meta.url), 'utf8')
const qaLayout = readFileSync(new URL('../src/qa-layout-main.tsx', import.meta.url), 'utf8')

assert.match(newGameCreator, /cabinMode: 'vanilla'/, 'new games must default to original cabin behavior')
assert.match(
  newGameCreator,
  /cfg\.cabinMode === 'recommended' \? '堆叠' : '原版'/,
  'the compatibility wire value recommended must be presented as 堆叠',
)
assert.doesNotMatch(
  newGameCreator,
  /cfg\.cabinMode === 'vanilla' \? '原版' : '推荐'/,
  'the new-game cabin control must not present stacking as 推荐',
)

assert.equal(DEFAULT_SERVER_RUNTIME_SETTINGS.cabinStrategy, 'None')
assert.equal(normalizeServerRuntimeSettings(null).cabinStrategy, 'None')
assert.equal(normalizeServerRuntimeSettings({ cabinStrategy: '' }).cabinStrategy, 'None')
assert.equal(normalizeServerRuntimeSettings({ cabinStrategy: 'CabinStack' }).cabinStrategy, 'CabinStack')
assert.equal(normalizeServerRuntimeSettings({ cabinStrategy: 'FarmhouseStack' }).cabinStrategy, 'FarmhouseStack')
assert.match(qaLayout, /cabinStrategy: 'None'/, 'QA runtime settings must mirror the fresh original default')

assert.match(dialog, /<option value="CabinStack">/, 'shared dialog must keep CabinStack selectable')
assert.match(dialog, /<option value="None">/, 'shared dialog must keep None selectable')
assert.match(
  dialog,
  /<option value="None">[\s\S]*?<option value="CabinStack">/,
  'the original option should be presented before the opt-in stacking mode',
)
assert.doesNotMatch(dialog, /最适合大多数服务器/, 'the stacking option must no longer be presented as recommended')
assert.match(
  dialog,
  /<option value="FarmhouseStack" hidden>FarmhouseStack（兼容已有配置）<\/option>/,
  'shared dialog must hide FarmhouseStack while preserving the legacy controlled value',
)
assert.doesNotMatch(dialog, /<option value="FarmhouseStack">/, 'shared dialog must not expose FarmhouseStack')

console.log('cabin strategy option regression checks passed')
