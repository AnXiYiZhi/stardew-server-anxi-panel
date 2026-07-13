import assert from 'node:assert/strict'
import type { PanelUpdateStatus } from '../src/api.ts'
import { updateDisplayKind, updateSummaryText, withVersionPrefix, withoutVersionPrefix } from '../src/games/stardew/update-status.ts'

function status(overrides: Partial<PanelUpdateStatus>): PanelUpdateStatus {
  return {
    currentVersion: '0.1.14', currentCommit: '', currentBuildDate: '', latestVersion: '',
    updateAvailable: false, releaseUrl: '', publishedAt: null, checkedAt: null,
    checkStatus: 'pending', checkError: '', ...overrides,
  }
}

assert.equal(withVersionPrefix('0.1.14'), 'v0.1.14')
assert.equal(withVersionPrefix('v0.1.14'), 'v0.1.14')
assert.equal(withoutVersionPrefix('V0.1.15'), '0.1.15')

const available = status({ latestVersion: 'v0.1.15', updateAvailable: true, checkStatus: 'ok' })
assert.equal(updateDisplayKind(available), 'available')
assert.equal(updateSummaryText(available), '发现新版本 v0.1.15')

assert.equal(updateDisplayKind(status({ checkStatus: 'ok' })), 'latest')
assert.equal(updateSummaryText(status({ checkStatus: 'ok' })), '✓ 最新')
assert.equal(updateDisplayKind(status({ checkStatus: 'error', checkError: 'network' })), 'error')
assert.equal(updateSummaryText(status({ checkStatus: 'error' })), '检查失败')
assert.equal(updateSummaryText(status({ checkStatus: 'unavailable' })), '开发版本')

// A failed refresh must still surface a previously known available release.
const staleAvailable = status({ latestVersion: 'v0.1.15', updateAvailable: true, checkStatus: 'error' })
assert.equal(updateDisplayKind(staleAvailable), 'available')

console.log('update status display tests passed')
