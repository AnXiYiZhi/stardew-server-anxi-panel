import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

const surfaces = [
  ['desktop', new URL('../src/games/stardew/pages/ServerControlPage.tsx', import.meta.url)],
  ['mobile', new URL('../src/games/stardew/mobile/MobileControlPage.tsx', import.meta.url)],
] as const

for (const [surface, path] of surfaces) {
  const source = readFileSync(path, 'utf8')
  assert.match(source, /<ServerRuntimeSettingsDialog/, `${surface} must use the shared runtime settings dialog`)
}

const dialog = readFileSync(new URL('../src/games/stardew/ServerRuntimeSettingsDialog.tsx', import.meta.url), 'utf8')
assert.match(dialog, /<option value="CabinStack">/, 'shared dialog must keep CabinStack selectable')
assert.match(dialog, /<option value="None">/, 'shared dialog must keep None selectable')
assert.match(
  dialog,
  /<option value="FarmhouseStack" hidden>FarmhouseStack（兼容已有配置）<\/option>/,
  'shared dialog must hide FarmhouseStack while preserving the legacy controlled value',
)
assert.doesNotMatch(dialog, /<option value="FarmhouseStack">/, 'shared dialog must not expose FarmhouseStack')

console.log('cabin strategy option regression checks passed')
