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

for (const icon of ['seed', 'steam', 'download']) {
  const png = readFileSync(new URL(`../public/assets/stardew/ui/install/icon_install_step_${icon}_image2_regen.png`, import.meta.url))
  assert.equal(png.subarray(0, 8).toString('hex'), '89504e470d0a1a0a', `${icon} icon must be PNG`)
  assert.equal(png.readUInt32BE(16), 72, `${icon} icon width`)
  assert.equal(png.readUInt32BE(20), 72, `${icon} icon height`)
  assert.equal(png[25], 6, `${icon} icon must keep RGBA transparency`)
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
const apiSource = readTextFile(new URL('../src/api.ts', import.meta.url))
const dashboardDataSource = readTextFile(new URL('../src/games/stardew/useStardewDashboardData.ts', import.meta.url))
const overviewPageSource = readTextFile(new URL('../src/games/stardew/pages/OverviewPage.tsx', import.meta.url))
const serverSummarySource = readTextFile(new URL('../src/games/stardew/ServerSummaryCard.tsx', import.meta.url))
const inviteCardSource = readTextFile(new URL('../src/games/stardew/InviteCodeCard.tsx', import.meta.url))
const steamInviteStateSource = readTextFile(new URL('../src/games/stardew/steam-invite-state.ts', import.meta.url))
const lanDirectConnectSource = readTextFile(new URL('../src/games/stardew/LanDirectConnectCard.tsx', import.meta.url))
const mobileHomePageSource = readTextFile(new URL('../src/games/stardew/mobile/MobileHomePage.tsx', import.meta.url))
const appSource = readTextFile(new URL('../src/App.tsx', import.meta.url))
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
const releaseMatrixValidator = readTextFile(new URL('../../scripts/validate-release-matrix.sh', import.meta.url))
const releaseMatrixTests = readTextFile(new URL('../../scripts/tests/test_release_matrix.sh', import.meta.url))
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
assert.match(installPageCss, /\.sd-install-column-title-download::before\s*{[^}]*icon_install_step_download_image2_regen\.png/s)
assert.match(installPageCss, /@media\s*\(prefers-reduced-motion:\s*reduce\)[\s\S]*\.sd-install-activity-dot\s*{[^}]*animation:\s*none/s)
assert.match(installPageSource, /aria-label="SMAPI 安装包下载进度"/)
assert.match(installPageSource, /'下载与安装'/)
assert.match(installPageSource, /icon_install_step_seed_image2_regen\.png/)
assert.match(installPageSource, /icon_install_step_steam_image2_regen\.png/)
assert.match(installPageSource, /icon_install_step_download_image2_regen\.png/)
assert.doesNotMatch(installPageSource, /icon_install_step_(?:seed|steam|download)_clean\.svg/)
assert.match(installPageCss, /icon_install_step_steam_image2_regen\.png/)
assert.match(installPageCss, /icon_install_step_download_image2_regen\.png/)
assert.match(installPageCss, /\.sd-install-page \.sd-install-step\s*\{[^}]*isolation:\s*isolate[^}]*overflow:\s*visible/s)
assert.match(installPageCss, /\.sd-install-steps::before\s*\{[^}]*z-index:\s*0/s)
assert.match(installPageCss, /\.sd-install-step-art\s*\{[^}]*z-index:\s*2[^}]*object-fit:\s*contain[^}]*filter:\s*none/s)
assert.doesNotMatch(installPageCss, /\.sd-install-step-art\s*\{[^}]*drop-shadow/s)
assert.match(installPageCss, /\.sd-install-auth-placeholder\s*\{[^}]*isolation:\s*isolate[^}]*overflow:\s*visible/s)
assert.match(installPageCss, /\.sd-install-auth-orb\s*\{[^}]*z-index:\s*1[^}]*object-fit:\s*contain[^}]*filter:\s*none/s)
assert.doesNotMatch(installPageCss, /\.sd-install-auth-orb\s*\{[^}]*drop-shadow/s)
assert.match(installPageCss, /\.sd-install-log-order-note\s*\{[^}]*margin-left:\s*auto[^}]*white-space:\s*nowrap/s)
assert.match(
  installPageCss,
  /@container sd-main-scroll \(max-width: 760px\)[\s\S]*\.sd-install-log-order-note\s*\{[^}]*flex-basis:\s*100%[^}]*white-space:\s*normal/s,
)
assert.match(installPageSource, /最新日志在最上方（倒序显示）/)
assert.match(apiSource, /export function getLatestJobLogs/)
assert.match(apiSource, /params\.set\('latest', 'true'\)/)
assert.match(installPageSource, /getLatestJobLogs\(installJobId, 1000\)/)
assert.match(installPageSource, /const jobRes = await getJob\(installJobId\)\s+const logsRes = await getLatestJobLogs\(installJobId, 1000\)/)
assert.match(dashboardDataSource, /getLatestJobLogs\(jobId, 200\)/)
assert.match(dashboardDataSource, /if \(!steamInviteEnabledRef\.current\) \{[\s\S]*?setInviteCodeStatus\('disabled'\)[\s\S]*?return/)
assert.doesNotMatch(dashboardDataSource, /Promise\.allSettled\(\[[\s\S]{0,500}refreshInviteCode\(\)/)
assert.match(dashboardDataSource, /shouldPollSteamInvite\([\s\S]*?inviteCodeStatus/)
assert.match(dashboardDataSource, /STEAM_INVITE_POLL_INTERVAL_MS = 5_000/)
assert.match(dashboardDataSource, /STEAM_INVITE_POLL_MAX_ATTEMPTS = 125/)
assert.match(dashboardDataSource, /const shouldResetRuntimeGeneration = shouldRestartSteamInvitePolling\([\s\S]*?previousRuntimeState[\s\S]*?s\.state[\s\S]*?invitePollLastStatusRef\.current/)
assert.match(dashboardDataSource, /invitePollBudgetExhaustedRef\.current = true[\s\S]*?setInvitePollRequested\(false\)/)
assert.match(dashboardDataSource, /runtimeMayStillBeWarming[\s\S]*?const failureStatus = runtimeMayStillBeWarming \? 'generating' : 'auth_unavailable'[\s\S]*?setInviteCodeStatus\(failureStatus\)/)
assert.match(dashboardDataSource, /const shouldContinuePolling = shouldPollSteamInvite\([\s\S]*?setInvitePollRequested\(shouldContinuePolling\)/)
assert.match(dashboardDataSource, /setInvitePollRequested\(runtimeMayStillBeWarming\)/)
assert.match(dashboardDataSource, /if \(invitePollLastStatusRef\.current === 'auth_unavailable'\) return/)
assert.match(dashboardDataSource, /const inviteRequestGeneration = \+\+inviteRequestGenerationRef\.current/)
assert.match(dashboardDataSource, /isCurrentSteamInviteRequest\([\s\S]*?inviteRequestGenerationRef\.current[\s\S]*?steamInviteEnabledRef\.current/)
assert.match(dashboardDataSource, /instanceStateRequestGenerationRef\.current[\s\S]*?stateRequestGeneration !== instanceStateRequestGenerationRef\.current/)
assert.match(dashboardDataSource, /const inviteProjectionGeneration = \+\+inviteProjectionGenerationRef\.current/)
assert.match(dashboardDataSource, /const canProjectInvite = isCurrentSteamInviteProjection\([\s\S]*?preserveSteamInviteProjection\([\s\S]*?if \(!canProjectInvite\) return/)
assert.match(dashboardDataSource, /if \(shouldResetRuntimeGeneration\) \{[\s\S]*?invitePollAttemptsRef\.current = 0[\s\S]*?invitePollBudgetExhaustedRef\.current = false[\s\S]*?!canProjectInvite && shouldResumeSteamInviteAfterRuntimeReset\([\s\S]*?setInviteCodeStatus\('generating'\)[\s\S]*?setInvitePollRequested\(true\)/)
assert.match(dashboardDataSource, /!isCurrentSteamInviteProjection\([\s\S]*?inviteProjectionGenerationRef\.current[\s\S]*?\)\) return/)
assert.match(dashboardDataSource, /steamInviteEnabledRef\.current = false[\s\S]*?steamInviteEnabled: false, inviteCode: ''[\s\S]*?setInviteCodeLoading\(false\)/)
assert.match(dashboardDataSource, /const clearInviteCode[\s\S]*?inviteProjectionGenerationRef\.current \+= 1[\s\S]*?const requestInviteCodeRefresh[\s\S]*?inviteProjectionGenerationRef\.current \+= 1/)
assert.match(dashboardDataSource, /const shouldStartPollingWithoutCurrentCode = shouldStartPolling && !inviteCodeRef\.current/)
assert.match(dashboardDataSource, /delayedJobRefreshRef\.current = window\.setTimeout\([\s\S]*?if \(!dashboardMountedRef\.current\) return/)
assert.match(dashboardDataSource, /dashboardMountedRef\.current = false[\s\S]*?clearTimeout\(delayedJobRefreshRef\.current\)/)
assert.match(dashboardDataSource, /await refreshInstanceState\(\)[\s\S]*?steamInviteEnabledRef\.current[\s\S]*?await refreshInviteCode\(\)/)
assert.match(dashboardDataSource, /const pollInviteCode = async \(\) => \{[\s\S]*?await refreshInstanceState\(\)[\s\S]*?if \(cancelled \|\| !steamInviteEnabledRef\.current\) return[\s\S]*?await refreshInviteCode\(\)/)
assert.match(dashboardDataSource, /const stateExposesInviteCode = s\.state === 'running'/)
assert.match(dashboardDataSource, /if \(!stateExposesInviteCode\) \{[\s\S]*?updateInviteCode\(null\)[\s\S]*?else if \(!recordedInviteCode\)/)
assert.match(overviewPageSource, /<LanDirectConnectCard dashboardData=\{dashboardData\} \/>[\s\S]*?instanceState\?\.steamInviteEnabled === true[\s\S]*?<InviteCodeCard/)
assert.match(serverSummarySource, /<LanDirectConnectCard dashboardData=\{dashboardData\} \/>[\s\S]*?instanceState\?\.steamInviteEnabled === true[\s\S]*?<InviteCodeCard/)
assert.match(mobileHomePageSource, /steamInviteEnabled \? \([\s\S]*?Steam 邀请码/)
assert.match(mobileHomePageSource, /局域网直连/)
assert.match(mobileHomePageSource, /const steamInviteEnabled = steamInviteIsEnabled\(instanceState\)/)
assert.match(mobileHomePageSource, /invite\.retryAuthorization && isAdmin \? \([\s\S]*?重新授权/)
assert.match(mobileHomePageSource, /invite\.retryAuthorization && !isAdmin[\s\S]*?授权异常，请联系管理员/)
assert.match(mobileHomePageSource, /\{inviteText\}/)
assert.match(mobileHomePageSource, /useSteamAuthLogin\(\{[\s\S]*?routeToPath\(route, options\)[\s\S]*?onUseDesktop\(\)/)
assert.match(mobileHomePageSource, /steamAuth\.requiresStop[\s\S]*?请先停止服务器，再重新完成 Steam 邀请码授权/)
assert.match(mobileHomePageSource, /const \[pendingStartupSawActiveJob, setPendingStartupSawActiveJob\] = useState\(false\)/)
assert.match(mobileHomePageSource, /pendingStartupAction && hasActiveLifecycleJob[\s\S]*?setPendingStartupSawActiveJob\(true\)/)
assert.match(mobileHomePageSource, /shouldClearPendingStartupAction\(\{[\s\S]*?sawActiveLifecycleJob: pendingStartupSawActiveJob[\s\S]*?setPendingStartupAction\(null\)[\s\S]*?setPendingStartupSawActiveJob\(false\)/)
assert.match(inviteCardSource, /if \(!enabled\) return null/)
assert.match(inviteCardSource, /授权尚未就绪|重新授权/)
assert.match(inviteCardSource, /steamInvitePresentation\([\s\S]*?instanceState\?\.state/)
assert.match(mobileHomePageSource, /steamInvitePresentation\([\s\S]*?instanceState\?\.state/)
assert.match(inviteCardSource, /steamInvitePresentation\([\s\S]*?dashboardData\.inviteCodeRefreshing/)
assert.match(mobileHomePageSource, /steamInvitePresentation\([\s\S]*?dashboardData\.inviteCodeRefreshing/)
assert.match(overviewPageSource, /confirmAction === 'stop'[\s\S]*?instanceState\?\.steamInviteEnabled === true[\s\S]*?Steam 邀请码将失效[\s\S]*?停止服务器将断开所有玩家连接。/)
assert.match(serverControlPageSource, /confirmAction === 'stop'[\s\S]*?instanceState\?\.steamInviteEnabled === true[\s\S]*?Steam 邀请码将立即失效[\s\S]*?停止服务器将断开所有在线玩家的连接。此操作不可撤销/)
assert.match(mobileHomePageSource, /steamInviteEnabled[\s\S]*?Steam 邀请码将失效[\s\S]*?停止服务器将断开所有玩家连接。/)
assert.match(steamInviteStateSource, /if \(status === 'auth_unavailable'\)[\s\S]*?Auth 异常（直连仍可用）/)
assert.match(steamInviteStateSource, /polling && error && status === 'generating'[\s\S]*?runtimeState === 'starting'[\s\S]*?runtimeState === 'running'/)
assert.doesNotMatch(steamInviteStateSource, /state\.state === 'starting' && status === 'auth_unavailable'/)
assert.match(steamInviteStateSource, /text: '等待中…'/)
assert.match(lanDirectConnectSource, /局域网直连/)
assert.match(apiSource, /export let defaultInstanceId = normalizeInstanceId\(null\)/)
assert.match(apiSource, /export function setDefaultInstanceId/)
assert.match(appSource, /setDefaultInstanceId\(status\.defaultInstanceId\)[\s\S]*?if \(!status\.initialized\)/)
assert.match(jobsLogsPageSource, /getLatestJobLogs\(selectedJobId\)/)
assert.match(jobsLogsPageSource, /const jobRes = await getJob\(selectedJobId\)\s+const logsRes = await getLatestJobLogs\(selectedJobId\)/)
assert.match(jobsLogsPageSource, /setLogsTruncated\(logsRes\.hasEarlier\)/)
assert.match(jobsLogsPageSource, /当前显示最新 1000 行日志，更早的日志未加载。/)
assert.doesNotMatch(jobsLogsPageSource, /当前仅显示最近加载的 1000 行日志/)
assert.match(newGameCreatorCss, /@container\s+ngc-modal\s*\(max-width:\s*1100px\)[\s\S]{0,260}grid-template-columns:minmax\(205px,\.9fr\) minmax\(360px,1\.75fr\) minmax\(172px,\.78fr\)/)
assert.match(newGameCreatorCss, /@container\s+ngc-modal\s*\(max-width:\s*780px\)[\s\S]{0,220}grid-template-columns:minmax\(205px,\.82fr\) minmax\(0,1\.7fr\)/)
assert.match(newGameCreatorCss, /@container\s+ngc-modal\s*\(max-width:\s*780px\)[\s\S]{0,360}grid-column:1\s*\/\s*-1/)
assert.match(newGameCreatorCss, /@container\s+ngc-modal\s*\(max-width:\s*560px\)[\s\S]{0,180}grid-template-columns:minmax\(0,1fr\)/)
assert.match(newGameCreatorCss, /@container\s+ngc-modal\s*\(max-width:\s*480px\)/)
assert.match(newGameCreatorCss, /@container\s+ngc-modal\s*\(max-width:\s*360px\)/)
assert.doesNotMatch(newGameCreatorCss, /@container\s+ngc-modal\s*\(max-width:\s*1100px\)[\s\S]{0,260}grid-template-columns:\s*1fr/)
assert.doesNotMatch(newGameCreatorCss, /transform:\s*scale\(/)
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
assert.ok(releaseCandidateWorkflow.includes('oldest_version:'))
assert.ok(releaseCandidateWorkflow.includes('Validate affected oldest supported Web upgrade'))
assert.ok(releaseCandidateWorkflow.includes('candidate-oldest.json'))
assert.ok(releaseCandidateWorkflow.includes('[manual-release-candidate]'))
assert.ok(releaseCandidateWorkflow.includes("github.event_name != 'push'"))
assert.ok(releaseCandidateWorkflow.includes('scripts/validate-release-matrix.sh'))
assert.ok(releaseCandidateWorkflow.includes('--require-oldest-for-zero-patch'))
assert.ok(releaseAfterCandidateWorkflow.includes('workflow_run:'))
assert.ok(releaseAfterCandidateWorkflow.includes('gh workflow run release.yml'))
assert.ok(releaseAfterCandidateWorkflow.includes("workflow_run.event != 'push'"))
assert.ok(releaseAfterCandidateWorkflow.includes('workflow_run.head_commit.message'))
assert.ok(releaseAfterCandidateWorkflow.includes('.previousVersion // empty'))
assert.ok(releaseAfterCandidateWorkflow.includes('.oldestTestedVersion // empty'))
assert.ok(releaseAfterCandidateWorkflow.includes('--require-oldest-for-zero-patch'))
assert.ok(releaseWorkflow.includes('workflow_dispatch:'))
assert.ok(releaseWorkflow.includes('.previousVersion // empty'))
assert.ok(releaseWorkflow.includes('.oldestTestedVersion // empty'))
assert.ok(releaseWorkflow.includes('--require-oldest-for-zero-patch'))
assert.match(releaseWorkflow, /skopeo copy --all --preserve-digests/)
assert.doesNotMatch(releaseWorkflow, /docker build/)
for (const workflowSource of [releaseCandidateWorkflow, releaseAfterCandidateWorkflow]) {
  assert.ok(workflowSource.includes('if [[ "$previous" != "$latest_release" ]]; then'))
}
assert.ok(releaseWorkflow.includes('group: release-panel-promotion'))
assert.ok(releaseWorkflow.includes('echo "previous=$previous"'))
assert.ok(releaseWorkflow.includes('if [[ "$previous" != "$latest_release" && "$version" != "$latest_release" ]]; then'))
assert.ok(releaseAfterCandidateWorkflow.includes('tag_candidate_run_id'))
assert.ok(releaseAfterCandidateWorkflow.includes('bound to a different or malformed immutable candidate'))
assert.ok(releaseAfterCandidateWorkflow.includes('git tag -a "$tag" "$commit" -m "$tag validated candidate" -m "Candidate workflow: $CANDIDATE_RUN_ID"'))
assert.ok(releaseAfterCandidateWorkflow.includes('Digest: $digest'))
assert.ok(releaseWorkflow.includes('echo "tag_candidate_run_id=$tag_candidate_run_id"'))
assert.ok(releaseWorkflow.includes('echo "tag_candidate_digest=$tag_candidate_digest"'))
assert.ok(releaseWorkflow.includes('actions/runs/$TAG_CANDIDATE_RUN_ID/artifacts'))
assert.ok(releaseWorkflow.includes('.conclusion == "success" and .head_sha == $sha and .head_branch == "main" and .path == ".github/workflows/release-candidate.yml"'))
assert.ok(releaseWorkflow.includes('[.artifacts[] | select(.name == $name and (.expired | not))] | length'))
assert.ok(releaseWorkflow.includes('if [[ "$artifact_count" != 1 ]]; then'))
assert.ok(releaseWorkflow.includes('"$digest" != "$TAG_CANDIDATE_DIGEST"'))
assert.doesNotMatch(releaseWorkflow, /actions\/workflows\/release-candidate\.yml\/runs/)
assert.ok(releaseWorkflow.includes('git fetch --no-tags origin main:refs/remotes/origin/main'))
const identityMainFetch = releaseWorkflow.indexOf('git fetch --no-tags origin main:refs/remotes/origin/main')
const identityMainRead = releaseWorkflow.indexOf('git rev-parse origin/main')
assert.ok(identityMainFetch >= 0 && identityMainRead > identityMainFetch)
assert.ok(releaseWorkflow.includes('origin/main changed after exact version promotion'))
assert.ok(releaseWorkflow.includes('latest GitHub Release changed after proof validation'))
const latestPromotionStep = releaseWorkflow.indexOf('- name: Promote and verify latest tags')
const latestMainRecheck = releaseWorkflow.indexOf('git fetch --no-tags origin main:refs/remotes/origin/main', latestPromotionStep)
const latestReleaseRecheck = releaseWorkflow.indexOf(`latest_release="$(gh api "repos/$GITHUB_REPOSITORY/releases/latest" --jq '.tag_name')"`, latestPromotionStep)
const firstLatestCopy = releaseWorkflow.indexOf('docker://$DOCKERHUB_IMAGE:latest', latestPromotionStep)
assert.ok(latestPromotionStep >= 0
  && latestMainRecheck > latestPromotionStep
  && latestReleaseRecheck > latestMainRecheck
  && firstLatestCopy > latestReleaseRecheck)
assert.ok(releaseMatrixValidator.includes('oldest version must be strictly older than previous version'))
assert.ok(releaseMatrixValidator.includes('an affected oldest version is required for an explicit zero-patch release'))
assert.match(releaseMatrixValidator, /0\.6\.0\)[\s\S]*?"\$previous_version" != "0\.5\.13"[\s\S]*?"\$oldest_version" != "0\.3\.2"/)
assert.ok(releaseMatrixTests.includes('--version 0.6.0 --previous-version 0.5.13 --oldest-version 0.3.2 --require-oldest-for-zero-patch'))
assert.ok(releaseGates.includes('bash scripts/tests/test_release_matrix.sh'))

console.log('responsive layout tests passed')
