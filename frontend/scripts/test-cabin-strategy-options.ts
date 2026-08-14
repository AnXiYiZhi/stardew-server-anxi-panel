import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

const sources = [
  ['desktop', new URL('../src/games/stardew/pages/ServerControlPage.tsx', import.meta.url)],
  ['mobile', new URL('../src/games/stardew/mobile/MobileControlPage.tsx', import.meta.url)],
] as const

for (const [surface, path] of sources) {
  const source = readFileSync(path, 'utf8')
  assert.match(source, /<option value="CabinStack">/, `${surface} must keep CabinStack selectable`)
  assert.match(source, /<option value="None">/, `${surface} must keep None selectable`)
  assert.match(
    source,
    /<option value="FarmhouseStack" hidden>FarmhouseStack（兼容已有配置）<\/option>/,
    `${surface} must hide FarmhouseStack while preserving the legacy controlled value`,
  )
  assert.doesNotMatch(
    source,
    /<option value="FarmhouseStack">/,
    `${surface} must not expose FarmhouseStack as a selectable option`,
  )
}

console.log('cabin strategy option regression checks passed')
