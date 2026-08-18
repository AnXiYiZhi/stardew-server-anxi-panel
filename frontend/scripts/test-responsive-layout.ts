import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import {
  COMPACT_PHONE_MAX_WIDTH,
  COMPACT_SHELL_MEDIA_QUERY,
  COMPACT_TOUCH_MAX_WIDTH,
  SHELL_DESIGN_HEIGHT,
  SHELL_DESIGN_WIDTH,
  SHELL_MIN_UI_SCALE,
  calculateShellViewport,
  expandedMainWidthForShell,
  shouldAutoCollapseOpsRail,
  shouldForceCompactShell,
  shouldUseCompactShell,
} from '../src/games/stardew/responsive-layout.ts'
import { playerAuthPasswordError } from '../src/games/stardew/player-auth-password.ts'

function readTextFile(url: URL): string {
  return readFileSync(url, 'utf8').replace(/\r\n?/g, '\n')
}

function closeTo(actual: number, expected: number, message: string, tolerance = 1e-7): void {
  assert.ok(
    Math.abs(actual - expected) <= tolerance,
    `${message}: expected ${expected}, received ${actual}`,
  )
}

function assertValidLayout(width: number, height: number): void {
  const layout = calculateShellViewport(width, height)
  assert.ok(Number.isFinite(layout.scale) && layout.scale >= SHELL_MIN_UI_SCALE)
  assert.ok(Number.isFinite(layout.layoutWidth) && layout.layoutWidth > 0)
  assert.ok(Number.isFinite(layout.layoutHeight) && layout.layoutHeight > 0)
  closeTo(layout.layoutWidth * layout.scale, width, `${width}×${height} visual width`)
  closeTo(layout.layoutHeight * layout.scale, height, `${width}×${height} visual height`)

  if (layout.scale > SHELL_MIN_UI_SCALE + Number.EPSILON) {
    assert.ok(
      Math.abs(layout.layoutWidth - SHELL_DESIGN_WIDTH) <= 1e-7 ||
        Math.abs(layout.layoutHeight - SHELL_DESIGN_HEIGHT) <= 1e-7,
      `${width}×${height} must remain anchored to one design edge above the minimum scale`,
    )
  }
}

assert.equal(SHELL_DESIGN_WIDTH, 1536)
assert.equal(SHELL_DESIGN_HEIGHT, 1024)
assert.equal(SHELL_MIN_UI_SCALE, 0.72)
assert.equal(COMPACT_PHONE_MAX_WIDTH, 768)
assert.equal(COMPACT_TOUCH_MAX_WIDTH, 1366)
assert.match(COMPACT_SHELL_MEDIA_QUERY, /max-width: 768px/)
assert.match(COMPACT_SHELL_MEDIA_QUERY, /hover: none/)
assert.match(COMPACT_SHELL_MEDIA_QUERY, /pointer: coarse/)
assert.match(COMPACT_SHELL_MEDIA_QUERY, /max-width: 1366px/)
assert.equal(shouldForceCompactShell('?shell=mobile'), true)
assert.equal(shouldForceCompactShell('?playerId=123&shell=mobile'), true)
assert.equal(shouldForceCompactShell('?shell=desktop'), false)
assert.equal(shouldForceCompactShell('?shell=MOBILE'), false)
assert.equal(shouldForceCompactShell(''), false)

for (const [width, hoverNone, pointerCoarse, expected] of [
  [767, false, false, true],
  [768, false, false, true],
  [769, false, false, false],
  [769, true, true, true],
  [1366, true, true, true],
  [1366, false, true, false],
  [1366, true, false, false],
  [1367, true, true, false],
  [Number.NaN, true, true, false],
] as const) {
  assert.equal(
    shouldUseCompactShell(width, hoverNone, pointerCoarse),
    expected,
    `compact-shell capability tuple ${String(width)}/${hoverNone}/${pointerCoarse}`,
  )
}

const sweepHeights = [240, 320, 360, 390, 480, 568, 600, 720, 768, 844, 900, 1024, 1080, 1180, 1440, 2160]
for (let width = 280; width <= 3840; width += 1) {
  for (const height of sweepHeights) assertValidLayout(width, height)
}

for (const [width, height] of [
  [280, 653],
  [319, 568],
  [320, 568],
  [640, 480],
  [641, 480],
  [720, 600],
  [721, 600],
  [768, 1024],
  [769, 1024],
  [1105, 737],
  [1106, 738],
  [1366, 500],
  [1367, 500],
  [1920, 500],
  [2560, 720],
  [3840, 2160],
  [5120, 1440],
  [7680, 4320],
] as const) {
  assertValidLayout(width, height)
}

closeTo(calculateShellViewport(1105, 900).scale, 0.72, 'minimum scale below width threshold')
assert.ok(calculateShellViewport(1106, 738).scale > 0.72)
closeTo(calculateShellViewport(1366, 768).scale, 0.75, 'height-limited desktop scale')
closeTo(calculateShellViewport(1920, 1080).scale, 1080 / 1024, 'large desktop scale')

for (const [width, height] of [
  [Number.NaN, 0],
  [Number.POSITIVE_INFINITY, 844],
  [390, Number.NEGATIVE_INFINITY],
  [-1, -1],
] as const) {
  const layout = calculateShellViewport(width, height)
  assert.ok(Number.isFinite(layout.scale) && layout.scale >= SHELL_MIN_UI_SCALE)
  assert.ok(Number.isFinite(layout.layoutWidth) && layout.layoutWidth > 0)
  assert.ok(Number.isFinite(layout.layoutHeight) && layout.layoutHeight > 0)
}

assert.equal(shouldAutoCollapseOpsRail(720, 600, false), false)
assert.ok(Number.isFinite(expandedMainWidthForShell(721, 600)))
const freshExpandWidth = Array.from({ length: 1600 }, (_, index) => 721 + index).find(
  (width) => !shouldAutoCollapseOpsRail(width, 768, false),
)
const collapsedExpandWidth = Array.from({ length: 1600 }, (_, index) => 721 + index).find(
  (width) => !shouldAutoCollapseOpsRail(width, 768, true),
)
assert.ok(freshExpandWidth != null && collapsedExpandWidth != null)
assert.ok(collapsedExpandWidth > freshExpandWidth, 'ops rail must retain a real hysteresis band')

const shellCss = readTextFile(new URL('../src/games/stardew/StardewPanel.css', import.meta.url))
const panelSource = readTextFile(new URL('../src/games/stardew/StardewPanel.tsx', import.meta.url))
const appCss = readTextFile(new URL('../src/App.css', import.meta.url))
const compactShellCss = readTextFile(new URL('../src/games/stardew/StardewMobileShell.css', import.meta.url))
const compactShellSource = readTextFile(new URL('../src/games/stardew/StardewMobileShell.tsx', import.meta.url))
const updateDialogCss = readTextFile(new URL('../src/games/stardew/UpdateDetailsDialog.css', import.meta.url))
const panelUpdateCss = readTextFile(new URL('../src/games/stardew/PanelUpdateProvider.css', import.meta.url))
const mobilePlayersCss = readTextFile(new URL('../src/games/stardew/mobile/MobilePlayersPage.css', import.meta.url))
const newGameCreatorCss = readTextFile(new URL('../src/games/stardew/NewGameCreator.css', import.meta.url))
const savesPageCss = readTextFile(new URL('../src/games/stardew/pages/SavesPage.css', import.meta.url))
const installPageCss = readTextFile(new URL('../src/games/stardew/pages/InstallPage.css', import.meta.url))
const installPageSource = readTextFile(new URL('../src/games/stardew/pages/InstallPage.tsx', import.meta.url))
const modsPageCss = readTextFile(new URL('../src/games/stardew/pages/ModsPage.css', import.meta.url))
const modsPageSource = readTextFile(new URL('../src/games/stardew/pages/ModsPage.tsx', import.meta.url))
const diagnosticsPageCss = readTextFile(new URL('../src/games/stardew/pages/DiagnosticsPage.css', import.meta.url))
const diagnosticsPageSource = readTextFile(new URL('../src/games/stardew/pages/DiagnosticsPage.tsx', import.meta.url))
const modalPortalSource = readTextFile(new URL('../src/core/ModalPortal.tsx', import.meta.url))
const playersPageCss = readTextFile(new URL('../src/games/stardew/pages/PlayersPage.css', import.meta.url))
const playersPageSource = readTextFile(new URL('../src/games/stardew/pages/PlayersPage.tsx', import.meta.url))
const savesSectionSource = readTextFile(new URL('../src/games/stardew/SavesSection.tsx', import.meta.url))
const settingsPageSource = readTextFile(new URL('../src/games/stardew/pages/SettingsPage.tsx', import.meta.url))
const jobsLogsPageSource = readTextFile(new URL('../src/games/stardew/pages/JobsLogsPage.tsx', import.meta.url))
const mobileControlPageSource = readTextFile(new URL('../src/games/stardew/mobile/MobileControlPage.tsx', import.meta.url))
const serverControlPageSource = readTextFile(new URL('../src/games/stardew/pages/ServerControlPage.tsx', import.meta.url))
const playerAuthDialogSource = readTextFile(new URL('../src/games/stardew/PlayerAuthSettingsDialog.tsx', import.meta.url))
const playerAuthDialogCss = readTextFile(new URL('../src/games/stardew/PlayerAuthSettingsDialog.css', import.meta.url))
const lifecycleActionsSource = readTextFile(new URL('../src/games/stardew/useStardewLifecycleActions.ts', import.meta.url))
const mobileSavesPageSource = readTextFile(new URL('../src/games/stardew/mobile/MobileSavesPage.tsx', import.meta.url))
const stardewRoutesSource = readTextFile(new URL('../src/games/stardew/stardew-routes.ts', import.meta.url))
const qaSource = readTextFile(new URL('../src/qa-layout-main.tsx', import.meta.url))
const releaseWorkflow = readTextFile(new URL('../../.github/workflows/release.yml', import.meta.url))
const releaseCandidateWorkflow = readTextFile(new URL('../../.github/workflows/release-candidate.yml', import.meta.url))
const releaseAfterCandidateWorkflow = readTextFile(new URL('../../.github/workflows/release-after-candidate.yml', import.meta.url))
const releaseGates = readTextFile(new URL('../../scripts/run-release-gates.sh', import.meta.url))
const releaseCandidateScript = readTextFile(new URL('../../scripts/release-candidate.sh', import.meta.url))
const releaseUpgradeE2E = readTextFile(new URL('../../scripts/tests/test_release_candidate_upgrade.sh', import.meta.url))
const compatibilityWorkflow = readTextFile(new URL('../../.github/workflows/compatibility-matrix.yml', import.meta.url))

assert.doesNotMatch(shellCss, /100(?:vw|vh|dvh)\s*\/\s*var\(--sd-design/)
assert.doesNotMatch(shellCss, /--sd-ui-fluid-scale/)
assert.doesNotMatch(shellCss, /\/\s*var\(--sd-ui-scale\)/)
assert.match(shellCss, /width:\s*var\(--sd-shell-layout-width\)/)
assert.match(shellCss, /height:\s*var\(--sd-shell-layout-height\)/)
assert.match(shellCss, /transform:\s*scale\(var\(--sd-ui-scale\)\)/)
assert.match(shellCss, /touch-action:\s*pan-x pan-y pinch-zoom/)
assert.match(shellCss, /\.sd-opsrail-stack\s*{[^}]*overflow-y:\s*auto/s)
assert.doesNotMatch(appCss, /:has\(\.sd-shell\)/)
assert.match(appCss, /body\s*{[^}]*min-width:\s*280px/s)
assert.match(appCss, /#root\.sd-desktop-shell-mounted/)
assert.match(appCss, /#root\.sd-mobile-shell-mounted/)
assert.match(appCss, /max-aspect-ratio:\s*8\s*\/\s*5/)
assert.match(appCss, /min-aspect-ratio:\s*5\s*\/\s*2/)
assert.match(shellCss, /\.sd-shell-viewport\s*{[^}]*overflow:\s*hidden/s)
assert.match(shellCss, /\.sd-shell-viewport\s*{[^}]*overflow:\s*clip/s)
assert.match(panelSource, /lockViewportOrigin/)
assert.match(compactShellCss, /padding-left:\s*env\(safe-area-inset-left/)
assert.match(compactShellCss, /padding-right:\s*env\(safe-area-inset-right/)
assert.match(compactShellCss, /touch-action:\s*pan-x pan-y pinch-zoom/)
assert.match(compactShellCss, /font-size:\s*16px/)
assert.match(compactShellCss, /\.sd-mshell-update\s*{[^}]*min-height:\s*44px/s)
assert.doesNotMatch(compactShellCss, /--sd-mobile-main-frame-edge-thickness\)\s*\/\s*2/)
assert.match(updateDialogCss, /\.sd-update-close\s*{[^}]*width:\s*44px;[^}]*height:\s*44px/s)
assert.match(updateDialogCss, /\.sd-update-actions a,\s*\.sd-update-actions button\s*{[^}]*min-height:\s*44px/s)
assert.match(updateDialogCss, /overscroll-behavior:\s*contain/)
assert.match(panelUpdateCss, /@media\s*\(max-width:\s*560px\)[\s\S]*?safe-area-inset-left/)
assert.match(mobilePlayersCss, /\.sd-mplay-player-actions\s*{[^}]*max-width:\s*100%/s)
assert.match(mobilePlayersCss, /\.sd-mplay-confirm-dialog\s*{[^}]*max-height:\s*calc\(100dvh - 32px\)/s)
assert.match(shellCss, /\.sd-confirm-dialog\s*{[^}]*box-sizing:\s*border-box/s)
assert.match(shellCss, /\.sd-confirm-dialog\s*{[^}]*max-height:\s*min\(90vh,\s*100%\)/s)
assert.match(savesPageCss, /\.sd-saves-modal-card\s*{[^}]*box-sizing:\s*border-box/s)
assert.match(savesPageCss, /\.sd-saves-modal-card\s*{[^}]*max-height:\s*min\(90vh,\s*100%\)/s)
assert.match(savesPageCss, /\.sd-saves-modal-card-wide\s*{[^}]*container-name:\s*ngc-modal[^}]*container-type:\s*inline-size/s)
assert.match(installPageCss, /\.sd-install-qr-card\s*{[^}]*box-sizing:\s*border-box/s)
assert.match(installPageCss, /\.sd-install-qr-card\s*{[^}]*max-height:\s*min\(92vh,\s*100%\)/s)
assert.match(installPageCss, /\.sd-install-column-title-download::before\s*{[^}]*icon_install_step_download_image2\.png/s)
assert.match(installPageCss, /@media\s*\(prefers-reduced-motion:\s*reduce\)[\s\S]*\.sd-install-activity-dot\s*{[^}]*animation:\s*none/s)
assert.match(installPageSource, /aria-label="SMAPI 安装包下载进度"/)
assert.match(installPageSource, /'下载与环境'/)
assert.match(newGameCreatorCss, /@container\s+ngc-modal\s*\(max-width:\s*1100px\)/)
assert.match(newGameCreatorCss, /@container\s+ngc-modal\s*\(max-width:\s*480px\)/)
assert.doesNotMatch(newGameCreatorCss, /@container\s+sd-main-scroll/)
assert.match(playersPageSource, /sd-players-pending-table-row/)
assert.match(playersPageCss, /\.sd-players-pending-table\s*{[^}]*overflow-x:\s*hidden/s)
assert.match(playersPageCss, /\.sd-players-pending-table \.sd-players-pending-table-row\s*{[^}]*grid-template-columns:[^}]*min-width:\s*0/s)
assert.match(modalPortalSource, /createPortal/)
assert.match(modalPortalSource, /document\.body/)
assert.match(modalPortalSource, /event\.key !== 'Tab'/)
assert.match(modalPortalSource, /event\.key === 'Escape'/)
assert.match(modalPortalSource, /returnTarget\.focus\(\)/)
assert.match(modalPortalSource, /element\.inert = true/)
assert.match(modalPortalSource, /setAttribute\('aria-hidden', 'true'\)/)
for (const modalConsumer of [
  panelSource,
  savesSectionSource,
  settingsPageSource,
  jobsLogsPageSource,
  playersPageSource,
  modsPageSource,
  mobileControlPageSource,
  mobileSavesPageSource,
]) {
  assert.match(modalConsumer, /<ModalPortal/)
}
assert.match(savesSectionSource, /role="alertdialog"/)
assert.match(mobileControlPageSource, />\s*填入\s*</)
assert.match(serverControlPageSource, /<PlayerAuthSettingsDialog/)
assert.match(mobileControlPageSource, /<PlayerAuthSettingsDialog/)
assert.match(playerAuthDialogSource, /mode:\s*'none'/)
assert.match(playerAuthDialogSource, /mode:\s*'global'/)
assert.match(playerAuthDialogSource, /mode:\s*'role'/)
assert.match(playerAuthDialogSource, /getInstancePlayerAuthConfig/)
assert.match(playerAuthDialogSource, /updateInstancePlayerAuthConfig/)
assert.doesNotMatch(playerAuthDialogSource, /restartInstance/)
assert.match(playerAuthDialogSource, /onRestart:\s*\(\) => Promise<void>/)
assert.match(playerAuthDialogSource, /!restartSettingChanged\s*&&\s*!roleCredentialsChanged/)
assert.match(playerAuthDialogSource, /revisionChanged\s*&&\s*next\.restartRequired\s*&&\s*isRunning/)
assert.match(playerAuthDialogSource, /await onRestart\(\)[\s\S]*?onClose\(\)/)
assert.match(serverControlPageSource, /onRestart={handleRestart}/)
assert.match(mobileControlPageSource, /onRestart={onPlayerAuthRestart}/)
assert.match(compactShellSource, /restartInProgress[\s\S]*?'正在重启'/)
assert.match(compactShellSource, /onPlayerAuthRestart={lifecycleActions\.handleRestart}/)
assert.match(lifecycleActionsSource, /restartInProgress\s*=\s*pendingStartupAction\s*===\s*'restart'/)
assert.match(lifecycleActionsSource, /pendingStartupSawActiveJob/)
assert.match(lifecycleActionsSource, /refreshInstanceState\(\)[\s\S]*?refreshJobs\(\)/)
assert.match(modsPageSource, /const latest = await loadMods\(\)[\s\S]*?syncNexusResultsFromInstalledMods\(latest\.mods\)/)
assert.match(diagnosticsPageSource, /Promise\.allSettled\([\s\S]*?getHealthDiagnostics\(\)[\s\S]*?getComposePs\(\)[\s\S]*?healthResult\.status === 'fulfilled'[\s\S]*?composeResult\.status === 'fulfilled'/)
assert.match(jobsLogsPageSource, /selectedStillExists[\s\S]*?setSelectedJob\(null\)[\s\S]*?setLogs\(\[\]\)/)
assert.match(mobileSavesPageSource, /setBackups\(Array\.isArray\(result\.backups\) \? result\.backups : \[\]\)/)
assert.match(stardewRoutesSource, /refreshMods: \(\) => Promise<void>/)
assert.match(stardewRoutesSource, /refreshPublicIP: \(force\?: boolean\) => Promise<void>/)
assert.match(playerAuthDialogSource, /role="alertdialog"/)
assert.match(playerAuthDialogSource, /timeoutSeconds/)
assert.match(playerAuthDialogSource, /maxAttempts/)
assert.match(playerAuthDialogSource, /htmlFor={`\$\{dialogTitleId\}-timeout`}/)
assert.match(playerAuthDialogSource, /htmlFor={`\$\{dialogTitleId\}-attempts`}/)
assert.match(playerAuthDialogSource, /mode !== 'none' \? <fieldset className="sd-player-auth-policy-fields"/)
assert.match(playerAuthDialogSource, /credentialStatus/)
assert.match(playerAuthDialogSource, /rolePasswordRemovals/)
assert.match(playerAuthDialogSource, /第一次 !login/)
assert.doesNotMatch(playerAuthDialogSource, /启用前必须/)
for (const password of ['x', 'one internal space', '不间断\u00a0空格', '🔐'.repeat(128)]) {
  assert.equal(playerAuthPasswordError(password, '角色密码'), null, `valid role password rejected: ${password}`)
}
for (const password of ['', ' leading', 'trailing ', 'two  spaces', 'line\nbreak', '🔐'.repeat(129)]) {
  assert.notEqual(playerAuthPasswordError(password, '角色密码'), null, `invalid role password accepted: ${password}`)
}
assert.match(playerAuthDialogCss, /\.sd-player-auth-modes\s*{[^}]*grid-template-columns:\s*repeat\(3,/s)
assert.match(playerAuthDialogCss, /@media\s*\(max-width:\s*620px\)[\s\S]*?\.sd-player-auth-modes\s*{[^}]*grid-template-columns:\s*1fr/s)
assert.match(playerAuthDialogCss, /\.sd-player-auth-role-list\s*{[^}]*overflow-y:\s*auto/s)
assert.match(playerAuthDialogCss, /\.sd-player-auth-clear\.sd-mctrl-dialog-btn\s*{[^}]*min-height:\s*44px/s)
assert.match(playerAuthDialogCss, /\.sd-player-auth-policy-fields\s*{[^}]*grid-template-columns:\s*repeat\(2,/s)
assert.match(playerAuthDialogCss, /@media\s*\(max-width:\s*620px\)[\s\S]*?\.sd-player-auth-policy-fields\s*{[^}]*grid-template-columns:\s*1fr/s)
assert.match(jobsLogsPageSource, /CONTROL_COMMAND_PAGE_SIZE\s*=\s*3/)
assert.match(jobsLogsPageSource, /controlCommands\.slice\(/)
assert.match(jobsLogsPageSource, /aria-label="最近控制命令分页"/)
assert.match(qaSource, /cmd_qa_joja_07/)
assert.match(modsPageSource, /typeof ResizeObserver === 'undefined'/)
assert.match(diagnosticsPageSource, /sd-diag-header-actions[\s\S]*?导出诊断包[\s\S]*?重新检查[\s\S]*?<\/div>/)
assert.match(diagnosticsPageSource, /sd-diag-maintenance-tools">\s*<p>[\s\S]*?<\/p>\s*<\/div>/)
assert.match(diagnosticsPageCss, /@media\s*\(max-width:\s*760px\)[\s\S]*?\.sd-diag-page \.sd-diag-header-actions\s*{[^}]*grid-template-columns:\s*repeat\(2,/s)
assert.match(diagnosticsPageCss, /@media\s*\(max-width:\s*460px\)[\s\S]*?\.sd-diag-page \.sd-diag-header-actions\s*{[^}]*grid-template-columns:\s*1fr/s)
assert.match(
  modsPageCss,
  /\.sd-mods-nexus-card \.sd-mods-card-actions > \.sd-btn-delete\s*{[^}]*background-color:\s*#b94432;[^}]*button_server_stop_red_blank\.png[^}]*color:\s*#fff6dc;/s,
)
assert.match(qaSource, /SURFACE === 'app'/)
assert.match(qaSource, /\/\\\/control-commands\$\//)
assert.match(qaSource, /\/\\\/config\\\/player-auth\$\//)
assert.match(qaSource, /timeoutSeconds:\s*120/)
assert.match(qaSource, /maxAttempts:\s*5/)
assert.match(releaseGates, /npm run test:responsive-layout/)
assert.match(compatibilityWorkflow, /npm run test:responsive-layout/)
assert.match(releaseGates, /npm run test:new-game-idempotency/)
assert.match(compatibilityWorkflow, /npm run test:new-game-idempotency/)
assert.match(releaseGates, /npm run test:nexus-extension-idempotency/)
assert.match(compatibilityWorkflow, /npm run test:nexus-extension-idempotency/)
assert.match(releaseGates, /npm run test:cabin-strategy-options/)
assert.match(compatibilityWorkflow, /npm run test:cabin-strategy-options/)
assert.match(releaseGates, /npm run test:runtime-player-limit/)
assert.match(compatibilityWorkflow, /npm run test:runtime-player-limit/)
assert.match(releaseGates, /npm run test:save-backup-details/)
assert.match(compatibilityWorkflow, /npm run test:save-backup-details/)
assert.match(releaseCandidateScript, /assert_frontend_contract_from_container/)
assert.match(releaseCandidateScript, /ServerRuntimeSettingsDialog/)
assert.match(releaseCandidateScript, /FarmhouseStack（兼容已有配置）/)
assert.match(releaseCandidateScript, /game-day rollback hover details/)
assert.match(releaseUpgradeE2E, /assert_upgraded_frontend_contract/)
assert.match(releaseUpgradeE2E, /ServerRuntimeSettingsDialog/)
assert.match(releaseUpgradeE2E, /FarmhouseStack（兼容已有配置）/)
assert.match(releaseUpgradeE2E, /game-day rollback hover details/)
assert.ok(releaseCandidateWorkflow.includes('scripts/run-release-gates.sh'))
assert.ok(releaseCandidateWorkflow.includes('push:\n    branches: [main]'))
assert.ok(releaseAfterCandidateWorkflow.includes('workflow_run:'))
assert.ok(releaseAfterCandidateWorkflow.includes('gh workflow run release.yml'))
assert.ok(releaseWorkflow.includes('workflow_dispatch:'))
assert.match(releaseWorkflow, /skopeo copy --all --preserve-digests/)
assert.doesNotMatch(releaseWorkflow, /docker build/)

console.log('responsive layout tests passed')
