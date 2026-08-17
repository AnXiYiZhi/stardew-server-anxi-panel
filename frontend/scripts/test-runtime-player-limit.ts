import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import {
  maxPlayersValidationError,
  runtimeSettingsEffectText,
  runtimeSettingsOnlineWarning,
  shouldShowPlayerLimitAction,
} from '../src/games/stardew/server-runtime-settings-state.ts'

const read = (path: string) => readFileSync(new URL(path, import.meta.url), 'utf8').replace(/\r\n?/g, '\n')

assert.equal(maxPlayersValidationError(1), null)
assert.equal(maxPlayersValidationError(100), null)
assert.notEqual(maxPlayersValidationError(0), null)
assert.notEqual(maxPlayersValidationError(101), null)
assert.notEqual(maxPlayersValidationError(1.5), null)
assert.equal(runtimeSettingsOnlineWarning(3, 4), '目标上限低于当前在线 4 人；仍可仅保存，重启后才会生效。')
assert.equal(runtimeSettingsOnlineWarning(4, 4), null)
assert.equal(runtimeSettingsOnlineWarning(1, null), null)
assert.equal(shouldShowPlayerLimitAction(true, true), true)
assert.equal(shouldShowPlayerLimitAction(false, true), false)
assert.equal(shouldShowPlayerLimitAction(true, false), false)
assert.equal(runtimeSettingsEffectText({ isRunning: true, currentMaxPlayers: 10, configuredMaxPlayers: 16 }), '当前生效上限 10 人；重启后配置 16 人，存在待重启变更。')
assert.equal(runtimeSettingsEffectText({ isRunning: true, currentMaxPlayers: 10, configuredMaxPlayers: 10 }), '当前生效上限与重启后配置均为 10 人。')
assert.equal(runtimeSettingsEffectText({ isRunning: false, currentMaxPlayers: 10, configuredMaxPlayers: 16 }), '服务器已停止；配置上限 16 人将在下次启动时生效。')

const desktop = read('../src/games/stardew/pages/ServerControlPage.tsx')
const mobile = read('../src/games/stardew/mobile/MobileControlPage.tsx')
const summary = read('../src/games/stardew/ServerSummaryCard.tsx')
const dialog = read('../src/games/stardew/ServerRuntimeSettingsDialog.tsx')
const dialogCss = read('../src/games/stardew/ServerRuntimeSettingsDialog.css')
const shellCss = read('../src/games/stardew/StardewPanel.css')
const hook = read('../src/games/stardew/useServerRuntimeSettings.ts')

for (const [name, source] of [['desktop', desktop], ['mobile', mobile]] as const) {
  assert.match(source, /useServerRuntimeSettings\(\{/, `${name} must use the shared runtime settings state flow`)
  assert.match(source, /<ServerRuntimeSettingsDialog/, `${name} must use the shared runtime settings form`)
  assert.match(source, /currentMaxPlayers=\{dashboardData\.players\?\.maxPlayers \?\? null\}/)
  assert.match(source, /人数上限 \/ 小屋策略 \/ 广播频率/)
}

assert.match(desktop, /canEditPlayerLimit=\{isAdmin\}/)
assert.match(desktop, /onEditPlayerLimit=\{\(\) => \{ void openRuntimeSettings\(\) \}\}/)
assert.match(summary, /canEditPlayerLimit: boolean/)
assert.match(summary, /onEditPlayerLimit\?: \(\) => void/)
assert.match(summary, /shouldShowPlayerLimitAction\(canEditPlayerLimit, Boolean\(onEditPlayerLimit\)\)/)
assert.match(summary, /修改上限/)
assert.match(shellCss, /\.sd-server-summary-limit-btn\s*\{[^}]*min-height:\s*44px/s)

assert.match(dialog, /type="number"/)
assert.match(dialog, /min=\{1\}/)
assert.match(dialog, /max=\{100\}/)
assert.match(dialog, /step=\{1\}/)
assert.match(dialog, /范围 1~100，包含主机位。降低上限不会删除已有角色或小屋。/)
assert.match(dialog, /<fieldset className="sd-runtime-advanced">/)
assert.match(dialog, /runtimeSettingsOnlineWarning\(draft\.maxPlayers, onlineCount\)/)
assert.match(dialog, /仅保存配置，不会直接重启服务器/)
assert.match(dialog, /saving \|\| Boolean\(validationError\)/)
assert.doesNotMatch(dialog, /disabled=\{[^}]*onlineWarning/)
assert.match(dialogCss, /\.sd-runtime-dialog-btn\s*\{[^}]*min-height:\s*44px/s)
assert.match(dialogCss, /@media \(max-width: 420px\)/)

assert.match(hook, /updateInstanceServerRuntimeSettings\(runtimeSettingsDraft\)/)
assert.match(hook, /await refreshPlayers\(\)/)
assert.match(hook, /isRunning[\s\S]*?下次启动时生效/)

console.log('runtime player limit regression checks passed')
