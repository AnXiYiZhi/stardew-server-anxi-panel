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

const shellCss = readFileSync(new URL('../src/games/stardew/StardewPanel.css', import.meta.url), 'utf8')
const panelSource = readFileSync(new URL('../src/games/stardew/StardewPanel.tsx', import.meta.url), 'utf8')
const appCss = readFileSync(new URL('../src/App.css', import.meta.url), 'utf8')
const compactShellCss = readFileSync(new URL('../src/games/stardew/StardewMobileShell.css', import.meta.url), 'utf8')
const updateDialogCss = readFileSync(new URL('../src/games/stardew/UpdateDetailsDialog.css', import.meta.url), 'utf8')
const panelUpdateCss = readFileSync(new URL('../src/games/stardew/PanelUpdateProvider.css', import.meta.url), 'utf8')
const mobilePlayersCss = readFileSync(new URL('../src/games/stardew/mobile/MobilePlayersPage.css', import.meta.url), 'utf8')
const newGameCreatorCss = readFileSync(new URL('../src/games/stardew/NewGameCreator.css', import.meta.url), 'utf8')
const savesPageCss = readFileSync(new URL('../src/games/stardew/pages/SavesPage.css', import.meta.url), 'utf8')
const installPageCss = readFileSync(new URL('../src/games/stardew/pages/InstallPage.css', import.meta.url), 'utf8')
const modsPageSource = readFileSync(new URL('../src/games/stardew/pages/ModsPage.tsx', import.meta.url), 'utf8')
const modalPortalSource = readFileSync(new URL('../src/core/ModalPortal.tsx', import.meta.url), 'utf8')
const playersPageCss = readFileSync(new URL('../src/games/stardew/pages/PlayersPage.css', import.meta.url), 'utf8')
const playersPageSource = readFileSync(new URL('../src/games/stardew/pages/PlayersPage.tsx', import.meta.url), 'utf8')
const savesSectionSource = readFileSync(new URL('../src/games/stardew/SavesSection.tsx', import.meta.url), 'utf8')
const settingsPageSource = readFileSync(new URL('../src/games/stardew/pages/SettingsPage.tsx', import.meta.url), 'utf8')
const jobsLogsPageSource = readFileSync(new URL('../src/games/stardew/pages/JobsLogsPage.tsx', import.meta.url), 'utf8')
const mobileControlPageSource = readFileSync(new URL('../src/games/stardew/mobile/MobileControlPage.tsx', import.meta.url), 'utf8')
const mobileSavesPageSource = readFileSync(new URL('../src/games/stardew/mobile/MobileSavesPage.tsx', import.meta.url), 'utf8')
const qaSource = readFileSync(new URL('../src/qa-layout-main.tsx', import.meta.url), 'utf8')
const releaseWorkflow = readFileSync(new URL('../../.github/workflows/release.yml', import.meta.url), 'utf8')
const releaseCandidateWorkflow = readFileSync(new URL('../../.github/workflows/release-candidate.yml', import.meta.url), 'utf8')
const releaseAfterCandidateWorkflow = readFileSync(new URL('../../.github/workflows/release-after-candidate.yml', import.meta.url), 'utf8')
const releaseGates = readFileSync(new URL('../../scripts/run-release-gates.sh', import.meta.url), 'utf8')
const releaseCandidateScript = readFileSync(new URL('../../scripts/release-candidate.sh', import.meta.url), 'utf8')
const releaseUpgradeE2E = readFileSync(new URL('../../scripts/tests/test_release_candidate_upgrade.sh', import.meta.url), 'utf8')
const compatibilityWorkflow = readFileSync(new URL('../../.github/workflows/compatibility-matrix.yml', import.meta.url), 'utf8')

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
assert.match(jobsLogsPageSource, /CONTROL_COMMAND_PAGE_SIZE\s*=\s*3/)
assert.match(jobsLogsPageSource, /controlCommands\.slice\(/)
assert.match(jobsLogsPageSource, /aria-label="最近控制命令分页"/)
assert.match(qaSource, /cmd_qa_joja_07/)
assert.match(modsPageSource, /typeof ResizeObserver === 'undefined'/)
assert.match(qaSource, /SURFACE === 'app'/)
assert.match(qaSource, /\/\\\/control-commands\$\//)
assert.match(releaseGates, /npm run test:responsive-layout/)
assert.match(compatibilityWorkflow, /npm run test:responsive-layout/)
assert.match(releaseGates, /npm run test:new-game-idempotency/)
assert.match(compatibilityWorkflow, /npm run test:new-game-idempotency/)
assert.match(releaseGates, /npm run test:nexus-extension-idempotency/)
assert.match(compatibilityWorkflow, /npm run test:nexus-extension-idempotency/)
assert.match(releaseGates, /npm run test:cabin-strategy-options/)
assert.match(compatibilityWorkflow, /npm run test:cabin-strategy-options/)
assert.match(releaseGates, /npm run test:save-backup-details/)
assert.match(compatibilityWorkflow, /npm run test:save-backup-details/)
assert.match(releaseCandidateScript, /assert_frontend_contract_from_container/)
assert.match(releaseCandidateScript, /FarmhouseStack（兼容已有配置）/)
assert.match(releaseCandidateScript, /game-day rollback hover details/)
assert.match(releaseUpgradeE2E, /assert_upgraded_frontend_contract/)
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
