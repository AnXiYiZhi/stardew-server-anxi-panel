import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const savesSectionPath = fileURLToPath(new URL('../src/games/stardew/SavesSection.tsx', import.meta.url))
const source = readFileSync(savesSectionPath, 'utf8')

const gameDaySectionStart = source.indexOf('aria-label="游戏日回档"')
const otherSectionStart = source.indexOf('aria-label="其他备份"', gameDaySectionStart)

assert.notEqual(gameDaySectionStart, -1, 'SavesSection must render the game-day rollback table')
assert.notEqual(otherSectionStart, -1, 'SavesSection must render the other-backups table')

const gameDaySection = source.slice(gameDaySectionStart, otherSectionStart)
const otherSection = source.slice(otherSectionStart)

assert.match(
  gameDaySection,
  /title=\{backupDetailsTitle\(backup\)\}/,
  'game-day rollback rows must expose the shared backup details on hover',
)
assert.match(
  otherSection,
  /title=\{backupDetailsTitle\(backup\)\}/,
  'other backup rows must keep using the shared backup details on hover',
)
assert.match(source, /backup\.farmerName \? `农民：\$\{backup\.farmerName\}`/)
assert.match(source, /backup\.farmType \? `地图：\$\{farmTypeLabel\[backup\.farmType\]/)

console.log('save backup hover detail regression checks passed')
