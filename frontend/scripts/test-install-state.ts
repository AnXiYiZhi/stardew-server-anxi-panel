import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { normalizeInstanceId } from '../src/instance-id.ts'
import { calcSteamDownloadTaskProgress, extractPullProgress, extractSMAPIArchiveProgress, installFailureDisplayMessage } from '../src/games/stardew/install-helpers.ts'
import { canonicalInstallJobs, canonicalInstallPageJobs, installJobForDisplay, latestInstallLogsFirst, logsDescribeActiveInstall, reconcileJobSnapshots } from '../src/games/stardew/install-state.ts'
import { gameInstallProgressPresentation, gameInstallStepProgressLabel } from '../src/games/stardew/install-progress-presentation.ts'
import { classifyInstallationState } from '../src/games/stardew/installation-state.ts'
import { routeToPath } from '../src/games/stardew/stardew-routes.ts'
import {
  isCurrentSteamInviteProjection,
  isCurrentSteamInviteRequest,
  preserveSteamInviteProjection,
  shouldPollSteamInvite,
  shouldResumeSteamInviteAfterRuntimeReset,
  shouldRestartSteamInvitePolling,
  steamInvitePollBudgetExhausted,
  steamInvitePresentation,
} from '../src/games/stardew/steam-invite-state.ts'
import type { InstallationDiagnostic, InstanceState, Job, JobLog, JobStatus } from '../src/types.ts'

const installPageSource = readFileSync(
  new URL('../src/games/stardew/pages/InstallPage.tsx', import.meta.url),
  'utf8',
).replace(/\r\n?/g, '\n')
const gameInstallRailSource = readFileSync(
  new URL('../src/games/GameInstallRail.tsx', import.meta.url),
  'utf8',
).replace(/\r\n?/g, '\n')
const diagnosticsPageSource = readFileSync(
  new URL('../src/games/stardew/pages/DiagnosticsPage.tsx', import.meta.url),
  'utf8',
).replace(/\r\n?/g, '\n')
const steamAuthHookSource = readFileSync(
  new URL('../src/games/stardew/useSteamAuthLogin.ts', import.meta.url),
  'utf8',
).replace(/\r\n?/g, '\n')
const apiSource = readFileSync(
  new URL('../src/api.ts', import.meta.url),
  'utf8',
).replace(/\r\n?/g, '\n')

assert.match(installPageSource, /const \[editingSteamCredentials, setEditingSteamCredentials\] = useState\(false\)/)
assert.match(installPageSource, /onClick=\{\(\) => \{ setEditingSteamCredentials\(true\); setShowForm\(true\); setInstallError\(''\) \}\}/)
assert.match(installPageSource, /\{isInstalled \? \(\s*<button[\s\S]*?setEditingSteamCredentials\(true\)[\s\S]*?修改 Steam 账号密码\s*<\/button>\s*\) : null\}/)
assert.match(installPageSource, /await updateSteamCredentials\(\{ steamUsername, steamPassword \}\)/)
assert.match(installPageSource, /await dashboardData\.refreshInstanceState\(\)/)
assert.match(installPageSource, /onSubmit=\{editingSteamCredentials \? handleSteamCredentialsSubmit : handleInstallSubmit\}/)
assert.match(installPageSource, /!editingSteamCredentials && !optionsLoading/)
assert.match(installPageSource, /\{!editingSteamCredentials \? \(\s*<div className="sd-install-field">\s*<label className="sd-install-field-label">VNC 密码<\/label>/)
assert.match(apiSource, /\/steam-credentials`, \{\s*method: 'PUT',\s*body,/)
assert.doesNotMatch(apiSource, /forceReauth/)
assert.doesNotMatch(installPageSource, /forceReauth|setForceReauth/)
assert.doesNotMatch(installPageSource, /from 'qrcode'|打开扫码窗口|>扫码登录</)
assert.match(installPageSource, /const automaticChoice = effectivePhase === 'auth_method_required'/)
assert.match(installPageSource, /handleAuthMethodSelect\('1'\)/)
assert.doesNotMatch(installPageSource, /选择 Steam 登录方式|选择 Steam Guard 验证方式|SteamCMD 需要重新授权<\/div>/)
assert.match(gameInstallRailSource, /sourceInstallation\.reason === 'required_files_missing'/)
assert.match(gameInstallRailSource, /sourceState\?\.driverPhase === 'install_verification_failed'/)
assert.match(gameInstallRailSource, /liveState\?\.installationDiagnostic\?\.requiredFiles === 'missing'/)
assert.match(installPageSource, /启用 Steam 邀请码（需要再次登录授权）/)
assert.match(installPageSource, /\{isAdmin \? \(\s*<>\s*<button[\s\S]*?启用 Steam 邀请码/)
assert.doesNotMatch(installPageSource, /只会检查 Auth 镜像并启动一次性登录容器；session 保存后容器立即停止/)
assert.match(installPageSource, /const steamInviteAuthorizationCleanupPending = instanceState\?\.steamInviteEnabled === true/)
assert.match(installPageSource, /const steamInviteAuthorizationReady = instanceState\?\.steamInviteEnabled === true/)
assert.match(installPageSource, /Steam 邀请码已启用/)
assert.match(installPageSource, /Steam 邀请码授权收尾中…/)
assert.match(installPageSource, /disabled=\{steamInviteAuthorizationReady \|\| steamInviteAuthorizationCleanupPending \|\| steamAuth\.busy/)
assert.doesNotMatch(installPageSource, /现有 SteamCMD 授权缓存和 SteamAuth session 保持不变/)
assert.doesNotMatch(installPageSource, /只有缓存或 session 被实际判定失效时，相关流程才会使用新凭据重新登录/)
assert.doesNotMatch(installPageSource, /原 SteamAuth session 会失效/)
assert.doesNotMatch(steamAuthHookSource, /session|只启动一次性 SteamAuth 授权容器/)
assert.match(installPageSource, /修改 Steam 账号密码/)
assert.match(installPageSource, /确认修改 Steam 账号密码/)
assert.doesNotMatch(installPageSource, /SteamCMD 与 Steam 邀请码授权共用这里保存的 Steam 账号密码/)
assert.doesNotMatch(installPageSource, /Steam 邀请码授权失败，可点击上方按钮重试/)
assert.doesNotMatch(installPageSource, /Steam 邀请码已按需启用，正在等待登录授权/)
assert.doesNotMatch(installPageSource, /Steam 邀请码二维码授权失败，可点击/)
assert.doesNotMatch(installPageSource, /更换 SteamCMD/)
assert.doesNotMatch(installPageSource, /steam-auth 国内网络波动导致下载失败/)
assert.doesNotMatch(installPageSource, /SteamCMD 兜底/)
assert.match(installPageSource, /latestInstallLogsFirst\(displayableLogs\)/)
assert.match(installPageSource, /最新日志在最上方（倒序显示）/)
assert.doesNotMatch(installPageSource, /scrollTo\(\{ top: 0/)
assert.doesNotMatch(installPageSource, /scrollHeight/)
assert.match(steamAuthHookSource, /const response = await steamAuthLogin\(\)/)
assert.match(steamAuthHookSource, /onStarted\?\.\(response\.jobId\)/)
assert.match(steamAuthHookSource, /onNavigate\('install', \{ installJobId: response\.jobId \}\)/)
assert.match(installPageSource, /onStarted: \(jobId\) => \{\s*setInstallJobId\(jobId\)\s*setInstallJob\(null\)\s*setLogs\(\[\]\)/)
assert.match(installPageSource, /if \(requestedInstallJobId\) return/)
assert.match(installPageSource, /const latestSteamTaskJob = installJobId \? installPageJobs\.selected : installJobForDisplay\(installPageJobs\)/)
assert.match(installPageSource, /const selectedSteamTaskLogs = latestSteamTaskJob\?\.id === installJobId \? logs : \[\]/)
assert.match(installPageSource, /const selectedTaskFailurePhase = latestSteamTaskJob\?\.status === 'failed'/)
assert.match(installPageSource, /installationWorkflowComplete && !hasActiveSteamAuthJob && !selectedTaskIsSteamAuth/)
assert.match(
  installPageSource,
  /const res = await installInstance\(body\)\s*onNavigate\('install', \{ installJobId: res\.jobId \}\)\s*setInstallJobId\(res\.jobId\)/,
)
assert.match(
  installPageSource,
  /if \(jobId\) \{\s*onNavigate\('install', \{ installJobId: jobId \}\)\s*if \(jobId !== installJobId\)/,
)
assert.equal(routeToPath('install', { installJobId: 'job_auth_new' }), '/instances/stardew/install?jobId=job_auth_new')
assert.equal(
  installFailureDisplayMessage('credentials_required', 'credentials_required', '', undefined, null, []),
  'Steam 账号或密码错误（SteamCMD 登录失败），请修改后再试。',
)
assert.match(diagnosticsPageSource, /const steamInviteEnabled = instanceState\?\.steamInviteEnabled === true/)
assert.match(diagnosticsPageSource, /steamInviteEnabled \? 'Junimo 运行组件版本对' : 'JunimoServer 运行组件版本'/)
assert.match(diagnosticsPageSource, /JunimoServer 版本完全匹配；可选 Auth 未启用/)
assert.match(diagnosticsPageSource, /未物化或验收可选 Auth/)

assert.equal(normalizeInstanceId('preview-instance_01'), 'preview-instance_01')
for (const invalidInstanceId of ['', '   ', '../escape', 'has/slash', '含中文', 'x'.repeat(65), null]) {
  assert.equal(normalizeInstanceId(invalidInstanceId), 'stardew')
}

function job(id: string, status: JobStatus, updatedAt: string, createdAt = '2026-08-11T00:00:00.000Z', type = 'stardew_install'): Job {
  return {
    id,
    type,
    status,
    targetType: 'instance',
    targetId: 'stardew',
    createdBy: 1,
    createdAt,
    startedAt: status === 'queued' ? null : createdAt,
    finishedAt: ['succeeded', 'failed', 'canceled'].includes(status) ? updatedAt : null,
    errorMessage: null,
    updatedAt,
  }
}

const staleDetail = job('job_install', 'running', '2026-08-11T00:00:05.000Z')
const terminalDashboard = job('job_install', 'succeeded', '2026-08-11T00:00:06.000Z')
assert.equal(reconcileJobSnapshots(staleDetail, terminalDashboard).status, 'succeeded')
assert.equal(reconcileJobSnapshots(terminalDashboard, staleDetail).status, 'succeeded')

const completed = canonicalInstallJobs([terminalDashboard], staleDetail)
assert.equal(completed.active, null)
assert.equal(completed.selected?.status, 'succeeded')
assert.equal(logsDescribeActiveInstall(completed.active, 'job_install'), false)

const active = job('job_new', 'running', '2026-08-11T00:01:05.000Z', '2026-08-11T00:01:00.000Z')
const withActive = canonicalInstallJobs([active, terminalDashboard], staleDetail)
assert.equal(withActive.active?.id, 'job_new')
assert.equal(withActive.latest?.id, 'job_new')
assert.equal(logsDescribeActiveInstall(withActive.active, 'job_install'), false)
assert.equal(logsDescribeActiveInstall(withActive.active, 'job_new'), true)

const terminalDetail = job('job_new', 'failed', '2026-08-11T00:01:06.000Z', '2026-08-11T00:01:00.000Z')
const terminalWinsOverDashboard = canonicalInstallJobs([active], terminalDetail)
assert.equal(terminalWinsOverDashboard.active, null)
assert.equal(terminalWinsOverDashboard.selected?.status, 'failed')

const activeSteamAuth = job(
  'job_steam_auth',
  'running',
  '2026-08-11T00:02:05.000Z',
  '2026-08-11T00:02:00.000Z',
  'stardew_steam_auth',
)
assert.equal(canonicalInstallJobs([activeSteamAuth], null).active, null)
assert.equal(canonicalInstallPageJobs([activeSteamAuth], null).active?.id, 'job_steam_auth')
assert.equal(logsDescribeActiveInstall(canonicalInstallPageJobs([activeSteamAuth], null).active, 'job_steam_auth'), true)

const competingActiveInstall = job(
  'job_competing_install',
  'running',
  '2026-08-11T00:03:05.000Z',
  '2026-08-11T00:03:00.000Z',
)
const explicitlySelectedAuthFailure = job(
  'job_selected_auth_failure',
  'failed',
  '2026-08-11T00:02:55.000Z',
  '2026-08-11T00:02:50.000Z',
  'stardew_steam_auth',
)
const explicitlySelected = canonicalInstallPageJobs(
  [competingActiveInstall, explicitlySelectedAuthFailure],
  null,
  explicitlySelectedAuthFailure.id,
)
assert.equal(explicitlySelected.active?.id, competingActiveInstall.id)
assert.equal(explicitlySelected.selected?.id, explicitlySelectedAuthFailure.id)
assert.equal(installJobForDisplay(explicitlySelected)?.id, explicitlySelectedAuthFailure.id)

const disabledInviteState = instanceState('running', 'running', { steamInviteEnabled: false })
assert.equal(shouldPollSteamInvite(disabledInviteState, true, null, 'generating'), false)
assert.equal(shouldPollSteamInvite(
  instanceState('running', 'running', { steamInviteEnabled: true }),
  true,
  null,
  'generating',
), true)
assert.equal(shouldPollSteamInvite(
  instanceState('stopped', 'stopped', { steamInviteEnabled: true }),
  true,
  null,
  'generating',
), false)
assert.equal(shouldPollSteamInvite(
  instanceState('starting', 'starting', { steamInviteEnabled: true }),
  true,
  null,
  'auth_unavailable',
), false)
assert.equal(shouldPollSteamInvite(
  instanceState('running', 'running', { steamInviteEnabled: true, steamInviteAuthState: 'ready' }),
  true,
  null,
  'auth_unavailable',
), false)
assert.equal(shouldPollSteamInvite(
  instanceState('running', 'running', { steamInviteEnabled: true, steamInviteAuthState: 'pending' }),
  true,
  null,
  'auth_unavailable',
), false)
assert.equal(steamInvitePresentation(false, 'disabled', null, null).text, '')
assert.deepEqual(
  steamInvitePresentation(true, 'waiting_authorization', null, null),
  { text: '等待 Steam 授权', copyable: false, retryAuthorization: true, tone: 'muted' },
)
assert.deepEqual(
  steamInvitePresentation(true, 'authorization_failed', null, null, 'failed'),
  { text: '授权失败，可重试', copyable: false, retryAuthorization: true, tone: 'error' },
)
assert.deepEqual(
  steamInvitePresentation(true, 'auth_unavailable', null, 'holder cleanup failed', 'cleanup_pending'),
  { text: '等待中…', copyable: false, retryAuthorization: false, tone: 'loading' },
)
assert.equal(steamInvitePresentation(true, 'server_stopped', null, null).text, '服务器未运行')
assert.equal(steamInvitePresentation(true, 'generating', null, null).text, '等待中…')
assert.equal(steamInvitePresentation(true, 'ready', 'ANXI-CODE', null).copyable, true)
assert.equal(steamInvitePresentation(true, 'auth_unavailable', null, null).text, 'Auth 异常（直连仍可用）')
assert.deepEqual(
  steamInvitePresentation(true, 'auth_unavailable', null, 'container starting', 'ready', 'starting', true),
  { text: 'Auth 异常（直连仍可用）', copyable: false, retryAuthorization: true, tone: 'error' },
)
assert.deepEqual(
  steamInvitePresentation(true, 'auth_unavailable', null, 'container starting', 'ready', 'running', true),
  { text: 'Auth 异常（直连仍可用）', copyable: false, retryAuthorization: true, tone: 'error' },
)
assert.deepEqual(
  steamInvitePresentation(true, 'generating', null, 'temporary network error', 'ready', 'running', true),
  { text: '等待中…', copyable: false, retryAuthorization: false, tone: 'loading' },
)
assert.equal(
  steamInvitePresentation(true, 'auth_unavailable', null, 'runtime failure', 'ready', 'running').text,
  'Auth 异常（直连仍可用）',
)
assert.equal(
  steamInvitePresentation(true, 'auth_unavailable', null, 'probe failed', 'pending', 'running', true).text,
  'Auth 异常（直连仍可用）',
)
assert.equal(shouldRestartSteamInvitePolling(null, 'running', null), true)
assert.equal(shouldRestartSteamInvitePolling('stopped', 'starting', 'auth_unavailable'), true)
assert.equal(shouldRestartSteamInvitePolling('starting', 'running', 'generating'), true)
assert.equal(shouldRestartSteamInvitePolling('starting', 'running', 'auth_unavailable'), false)
assert.equal(shouldRestartSteamInvitePolling('running', 'running', 'generating'), false)
assert.equal(isCurrentSteamInviteRequest(4, 4, true), true)
assert.equal(isCurrentSteamInviteRequest(3, 4, true), false)
assert.equal(isCurrentSteamInviteRequest(4, 4, false), false)
assert.equal(steamInvitePollBudgetExhausted(124, 125), false)
assert.equal(steamInvitePollBudgetExhausted(125, 125), true)

function instanceState(
  state: string,
  driverPhase = state,
  overrides: Partial<InstanceState> = {},
): InstanceState {
  return {
    instanceId: 'stardew',
    driverId: 'stardew_junimo',
    name: 'Stardew',
    state,
    stateMessage: null,
    driverPhase,
    updatedAt: '2026-08-13T00:00:00.000Z',
    steamInviteEnabled: false,
    ...overrides,
  }
}

function deferred<T>(): { promise: Promise<T>; resolve: (value: T) => void } {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((accept) => {
    resolve = accept
  })
  return { promise, resolve }
}

{
  let projectionGeneration = 0
  let projectedState = instanceState('stopped', 'stopped', {
    steamInviteEnabled: true,
    steamInviteAuthState: 'ready',
  })
  let projectedStatus: 'generating' | 'ready' = 'generating'
  let pollRequested = true
  const oldStateResponse = deferred<InstanceState>()
  const oldStateProjectionGeneration = ++projectionGeneration
  const oldStateCommit = oldStateResponse.promise.then((nextState) => {
    projectedState = isCurrentSteamInviteProjection(oldStateProjectionGeneration, projectionGeneration)
      ? nextState
      : preserveSteamInviteProjection(nextState, projectedState)
  })

  const newInviteProjectionGeneration = ++projectionGeneration
  assert.equal(isCurrentSteamInviteProjection(newInviteProjectionGeneration, projectionGeneration), true)
  projectionGeneration += 1
  projectedState = { ...projectedState, steamInviteEnabled: true, inviteCode: 'READY-CODE' }
  projectedStatus = 'ready'
  pollRequested = false
  oldStateResponse.resolve(instanceState('starting', 'starting', {
    steamInviteEnabled: true,
    steamInviteAuthState: 'ready',
    inviteCode: '',
  }))
  await oldStateCommit

  assert.equal(projectedState.state, 'starting')
  assert.equal(projectedState.inviteCode, 'READY-CODE')
  assert.equal(projectedState.steamInviteEnabled, true)
  assert.equal(projectedStatus, 'ready')
  assert.equal(pollRequested, false)
}

{
  let projectionGeneration = 0
  let inviteRequestGeneration = 0
  let projectedState = instanceState('starting', 'starting', {
    steamInviteEnabled: true,
    steamInviteAuthState: 'ready',
  })
  let projectedStatus: 'generating' | 'disabled' = 'generating'
  let pollRequested = true
  const oldInviteResponse = deferred<{ steamInviteEnabled: boolean; inviteCode: string }>()
  const oldInviteRequestGeneration = ++inviteRequestGeneration
  const oldInviteProjectionGeneration = ++projectionGeneration
  const oldInviteCommit = oldInviteResponse.promise.then((response) => {
    if (!isCurrentSteamInviteRequest(
      oldInviteRequestGeneration,
      inviteRequestGeneration,
      projectedState.steamInviteEnabled,
    ) || !isCurrentSteamInviteProjection(oldInviteProjectionGeneration, projectionGeneration)) return
    projectedState = { ...projectedState, steamInviteEnabled: response.steamInviteEnabled, inviteCode: response.inviteCode }
  })

  const disabledStateProjectionGeneration = ++projectionGeneration
  assert.equal(isCurrentSteamInviteProjection(disabledStateProjectionGeneration, projectionGeneration), true)
  inviteRequestGeneration += 1
  projectedState = { ...projectedState, state: 'stopped', steamInviteEnabled: false, inviteCode: '' }
  projectedStatus = 'disabled'
  pollRequested = false
  oldInviteResponse.resolve({ steamInviteEnabled: true, inviteCode: 'STALE-CODE' })
  await oldInviteCommit

  assert.equal(projectedState.state, 'stopped')
  assert.equal(projectedState.steamInviteEnabled, false)
  assert.equal(projectedState.inviteCode, '')
  assert.equal(projectedStatus, 'disabled')
  assert.equal(pollRequested, false)
  assert.equal(shouldPollSteamInvite(projectedState, true, null, 'generating'), false)
}

for (const stateResponseFirst of [true, false]) {
  let projectionGeneration = 0
  let runtimeState = 'starting'
  let attempts = 125
  let budgetExhausted = false
  let lastRequestStatus: 'generating' | 'auth_unavailable' = 'generating'
  let displayedStatus: 'generating' | 'auth_unavailable' = 'generating'
  let pollRequested = true
  const stateResponse = deferred<'running'>()
  const inviteResponse = deferred<'generating'>()
  const stateProjectionGeneration = ++projectionGeneration
  const stateCommit = stateResponse.promise.then((nextRuntimeState) => {
    const resetRuntimeGeneration = shouldRestartSteamInvitePolling(
      runtimeState,
      nextRuntimeState,
      lastRequestStatus,
    )
    runtimeState = nextRuntimeState
    if (!resetRuntimeGeneration) return
    attempts = 0
    budgetExhausted = false
    if (!isCurrentSteamInviteProjection(stateProjectionGeneration, projectionGeneration)
      && shouldResumeSteamInviteAfterRuntimeReset(true, null, lastRequestStatus)) {
      displayedStatus = 'generating'
      pollRequested = true
    }
  })
  const inviteProjectionGeneration = ++projectionGeneration
  const inviteCommit = inviteResponse.promise.then((status) => {
    if (!isCurrentSteamInviteProjection(inviteProjectionGeneration, projectionGeneration)) return
    projectionGeneration += 1
    lastRequestStatus = status
    if (attempts >= 125) {
      budgetExhausted = true
      displayedStatus = 'auth_unavailable'
      pollRequested = false
    }
  })

  if (stateResponseFirst) {
    stateResponse.resolve('running')
    await stateCommit
    inviteResponse.resolve('generating')
    await inviteCommit
  } else {
    inviteResponse.resolve('generating')
    await inviteCommit
    stateResponse.resolve('running')
    await stateCommit
  }

  assert.equal(runtimeState, 'running')
  assert.equal(attempts, 0)
  assert.equal(budgetExhausted, false)
  assert.equal(displayedStatus, 'generating')
  assert.equal(pollRequested, true)
}

assert.equal(shouldResumeSteamInviteAfterRuntimeReset(true, null, 'auth_unavailable'), false)
assert.equal(shouldResumeSteamInviteAfterRuntimeReset(true, 'READY-CODE', 'generating'), false)
assert.equal(shouldResumeSteamInviteAfterRuntimeReset(false, null, 'generating'), false)

function installationDiagnostic(
  overrides: Partial<InstallationDiagnostic> = {},
): InstallationDiagnostic {
  return {
    status: 'installed',
    requiredFiles: 'ok',
    compose: 'ready',
    image: 'available',
    serverContainer: 'stopped',
    recommendedAction: 'retry_start',
    checkedAt: '2026-08-13T00:00:00.000Z',
    ...overrides,
    control: {
      static: 'match',
      runtime: 'not_observed',
      expectedVersion: '1.0.0',
      ...overrides.control,
    },
  }
}

const classificationCases = [
  {
    name: 'legacy installed lifecycle state',
    state: instanceState('stopped', 'container_stopped'),
    active: false,
    expected: { kind: 'installed', action: 'none', isInstalled: true, showMissingInstallPrompt: false, canOpenInstallForm: true },
  },
  {
    name: 'Steam invite authorization failure does not become a base install failure',
    state: instanceState('stopped', 'steam_auth_failed', {
      steamInviteEnabled: true,
      steamInviteAuthState: 'failed',
      installationDiagnostic: installationDiagnostic(),
    }),
    active: false,
    expected: { kind: 'installed', action: 'none', isInstalled: true, showMissingInstallPrompt: false, canOpenInstallForm: true },
  },
  {
    name: 'active Steam invite authorization keeps an installed base state',
    state: instanceState('game_installed', 'steam_auth_running', {
      steamInviteEnabled: true,
      steamInviteAuthState: 'authorizing',
    }),
    active: false,
    expected: { kind: 'installed', action: 'none', isInstalled: true, showMissingInstallPrompt: false, canOpenInstallForm: true },
  },
  {
    name: 'legacy Steam invite failure without diagnostic stays diagnostic-only',
    state: instanceState('steam_auth_failed', 'steam_invite_auth_failed', {
      steamInviteEnabled: true,
      steamInviteAuthState: 'failed',
    }),
    active: false,
    expected: { kind: 'unknown', action: 'diagnose', isInstalled: false, showMissingInstallPrompt: false, canOpenInstallForm: false },
  },
  {
    name: 'fresh instance is the only legacy missing-install prompt',
    state: instanceState('admin_created'),
    active: false,
    expected: { kind: 'not_installed', action: 'install', isInstalled: false, showMissingInstallPrompt: true, canOpenInstallForm: true },
  },
  {
    name: 'active install suppresses the missing-install prompt',
    state: instanceState('admin_created'),
    active: true,
    expected: { kind: 'installing', action: 'none', isInstalled: false, showMissingInstallPrompt: false, canOpenInstallForm: false },
  },
  {
    name: 'scaffold and pulled image are not proof of a completed install',
    state: instanceState('junimo_scaffolded', 'junimo_scaffolded', {
      installationDiagnostic: installationDiagnostic({
        status: 'incomplete',
        requiredFiles: 'unknown',
        image: 'missing',
        control: { static: 'match', runtime: 'not_observed', expectedVersion: '1.0.0' },
        recommendedAction: 'repair_install',
      }),
    }),
    active: false,
    expected: { kind: 'not_installed', action: 'install', isInstalled: false, showMissingInstallPrompt: true, canOpenInstallForm: true },
  },
  {
    name: 'credential failure asks for install recovery instead of repair',
    state: instanceState('credentials_required', 'credentials_required', {
      installationDiagnostic: installationDiagnostic({
        status: 'incomplete',
        requiredFiles: 'unknown',
        image: 'missing',
        recommendedAction: 'repair_install',
      }),
    }),
    active: false,
    expected: { kind: 'install_failed', action: 'install', isInstalled: false, showMissingInstallPrompt: false, canOpenInstallForm: true },
  },
  {
    name: 'SteamCMD invalid password overrides partial-install repair evidence',
    state: instanceState('credentials_required', 'credentials_required', {
      installationDiagnostic: installationDiagnostic({
        status: 'incomplete',
        requiredFiles: 'missing',
        recommendedAction: 'repair_install',
      }),
    }),
    active: false,
    expected: { kind: 'install_failed', action: 'install', isInstalled: false, showMissingInstallPrompt: false, canOpenInstallForm: true },
  },
  {
    name: 'generic lifecycle error is not treated as uninstalled',
    state: instanceState('error', 'startup_control_mod_timeout'),
    active: false,
    expected: { kind: 'runtime_error', action: 'diagnose', isInstalled: false, showMissingInstallPrompt: false, canOpenInstallForm: false },
  },
  {
    name: 'legacy verifier confirms missing files',
    state: instanceState('error', 'install_verification_failed', { stateMessage: '游戏运行文件不完整，请重新安装或修复。' }),
    active: false,
    expected: { kind: 'repair_required', action: 'repair_install', isInstalled: false, showMissingInstallPrompt: false, canOpenInstallForm: true },
  },
  {
    name: 'legacy verifier transport failure remains diagnostic-only',
    state: instanceState('error', 'install_verification_failed', { stateMessage: '验证游戏运行文件失败，请检查任务日志后重试。' }),
    active: false,
    expected: { kind: 'runtime_error', action: 'diagnose', isInstalled: false, showMissingInstallPrompt: false, canOpenInstallForm: false },
  },
  {
    name: 'known SteamCMD installer error offers a retry without a first-install prompt',
    state: instanceState('error', 'steamcmd_failed'),
    active: false,
    expected: { kind: 'install_failed', action: 'install', isInstalled: false, showMissingInstallPrompt: false, canOpenInstallForm: true },
  },
  {
    name: 'installed diagnostic plus runtime error offers start retry',
    state: instanceState('error', 'startup_failed', {
      installationDiagnostic: installationDiagnostic({ recommendedAction: 'retry_start' }),
    }),
    active: false,
    expected: { kind: 'runtime_error', action: 'retry_start', isInstalled: true, showMissingInstallPrompt: false, canOpenInstallForm: false },
  },
  {
    name: 'explicit Control mismatch is diagnostic-only and never reinstall',
    state: instanceState('stopped', 'container_stopped', {
      installationDiagnostic: installationDiagnostic({
        control: { static: 'match', runtime: 'mismatch', observedVersion: '0.9.0', expectedVersion: '1.0.0' },
        recommendedAction: 'repair_install',
      }),
    }),
    active: false,
    expected: { kind: 'runtime_error', action: 'diagnose', isInstalled: true, showMissingInstallPrompt: false, canOpenInstallForm: false },
  },
  {
    name: 'diagnostic confirms required files missing',
    state: instanceState('error', 'install_verification_failed', {
      installationDiagnostic: installationDiagnostic({ status: 'incomplete', requiredFiles: 'missing', recommendedAction: 'repair_install' }),
    }),
    active: false,
    expected: { kind: 'repair_required', action: 'repair_install', isInstalled: false, showMissingInstallPrompt: false, canOpenInstallForm: true },
  },
  {
    name: 'running container plus incomplete evidence fails closed',
    state: instanceState('error', 'install_verification_failed', {
      installationDiagnostic: installationDiagnostic({
        status: 'incomplete',
        requiredFiles: 'missing',
        serverContainer: 'running',
        recommendedAction: 'repair_install',
      }),
    }),
    active: false,
    expected: { kind: 'runtime_error', action: 'diagnose', isInstalled: false, showMissingInstallPrompt: false, canOpenInstallForm: false },
  },
  {
    name: 'diagnostic confirms not installed',
    state: instanceState('error', 'not_installed', {
      installationDiagnostic: installationDiagnostic({
        status: 'not_installed',
        requiredFiles: 'unknown',
        compose: 'missing',
        image: 'missing',
        control: { static: 'missing', runtime: 'not_observed', expectedVersion: '1.0.0' },
        recommendedAction: 'install',
      }),
    }),
    active: false,
    expected: { kind: 'not_installed', action: 'install', isInstalled: false, showMissingInstallPrompt: true, canOpenInstallForm: true },
  },
  {
    name: 'unavailable diagnostics never become an install action',
    state: instanceState('error', 'docker_unavailable', {
      installationDiagnostic: installationDiagnostic({
        status: 'unknown',
        requiredFiles: 'unknown',
        compose: 'unavailable',
        image: 'unavailable',
        control: { static: 'unknown', runtime: 'unknown', expectedVersion: '1.0.0' },
        recommendedAction: 'diagnose',
      }),
    }),
    active: false,
    expected: { kind: 'unknown', action: 'diagnose', isInstalled: false, showMissingInstallPrompt: false, canOpenInstallForm: false },
  },
  {
    name: 'contradictory not-installed diagnostic fails closed',
    state: instanceState('error', 'diagnostic_conflict', {
      installationDiagnostic: installationDiagnostic({ status: 'not_installed', requiredFiles: 'ok', recommendedAction: 'install' }),
    }),
    active: false,
    expected: { kind: 'unknown', action: 'diagnose', isInstalled: false, showMissingInstallPrompt: false, canOpenInstallForm: false },
  },
] as const

for (const testCase of classificationCases) {
  const actual = classifyInstallationState(testCase.state, testCase.active)
  for (const [key, value] of Object.entries(testCase.expected)) {
    assert.equal(actual[key as keyof typeof actual], value, `${testCase.name}: ${key}`)
  }
}

function jobLog(sequence: number, message: string): JobLog {
  return {
    id: sequence,
    jobId: 'job_install',
    sequence,
    level: 'info',
    message,
    createdAt: '2026-08-18T00:00:00.000Z',
  }
}

const installLogSelectionInput = Array.from(
  { length: 60 },
  (_, index) => jobLog(((index * 17) % 60) + 1, `line-${index + 1}`),
)
const installLogSelectionInputOrder = installLogSelectionInput.map((entry) => entry.sequence)
const latestInstallLogs = latestInstallLogsFirst(installLogSelectionInput)
assert.equal(latestInstallLogs.length, 50)
assert.deepEqual(latestInstallLogs.map((entry) => entry.sequence), Array.from({ length: 50 }, (_, index) => 60 - index))
assert.deepEqual(
  installLogSelectionInput.map((entry) => entry.sequence),
  installLogSelectionInputOrder,
  'latest-first presentation must not mutate the chronological SSE state',
)
assert.deepEqual(latestInstallLogsFirst(installLogSelectionInput, 0), [])

const smapiDownload = extractSMAPIArchiveProgress([
  jobLog(1, '[smapi:download:progress:0:1000:1:2:false]'),
  jobLog(2, '[smapi:download:progress:500:1000:1:2:false]'),
], 'stardew_install')
assert.deepEqual(smapiDownload, {
  downloadedBytes: 500,
  totalBytes: 1000,
  percent: 50,
  candidate: 1,
  candidateCount: 2,
  cached: false,
})
const smapiDownloadingTask = calcSteamDownloadTaskProgress('smapi_installing', null, null, null, smapiDownload)
assert.equal(smapiDownloadingTask?.percent, 90)
assert.equal(smapiDownloadingTask?.done, 2)
assert.match(smapiDownloadingTask?.label ?? '', /50%.*1\/2/)

const smapiCache = extractSMAPIArchiveProgress([
  jobLog(3, '[smapi:download:progress:1000:1000:1:2:true]'),
], 'stardew_install')
assert.equal(smapiCache?.cached, true)
assert.equal(smapiCache?.percent, 100)
const smapiCachedTask = calcSteamDownloadTaskProgress('smapi_installing', null, null, null, smapiCache)
assert.equal(smapiCachedTask?.percent, 100)
assert.equal(smapiCachedTask?.done, 3)
assert.match(smapiCachedTask?.label ?? '', /缓存校验通过/)

assert.equal(extractSMAPIArchiveProgress([
  jobLog(4, '[smapi:download:progress:1001:1000:1:1:false]'),
], 'stardew_install'), null)
assert.equal(extractSMAPIArchiveProgress([
  jobLog(5, '[smapi:download:progress:500:1000:1:1:false]'),
], 'stardew_start'), null)

assert.deepEqual(extractPullProgress([
  jobLog(6, '[pull:progress:1:4]'),
  jobLog(7, '[pull:progress:3:4]'),
]), { done: 3, total: 4, percent: 75 })
assert.equal(extractPullProgress([jobLog(8, '[pull:progress:5:4]')]), null)

const idleInlineInstall = gameInstallProgressPresentation(instanceState('uninitialized'), null, [])
assert.equal(idleInlineInstall.mode, 'idle')
assert.equal(idleInlineInstall.percent, null)

const activeInlineInstallJob = job('job_inline_install', 'running', '2026-08-18T00:00:00.000Z')
const authorizationInlineInstall = gameInstallProgressPresentation(
  instanceState('installing', 'steam_guard_required'),
  activeInlineInstallJob,
  [],
)
assert.equal(authorizationInlineInstall.mode, 'active')
assert.equal(authorizationInlineInstall.stepIndex, 2)
assert.equal(authorizationInlineInstall.percent, null, 'indeterminate phases must not invent a percentage')

const downloadingInlineInstall = gameInstallProgressPresentation(
  instanceState('installing', 'game_downloading'),
  activeInlineInstallJob,
  [
    jobLog(9, '[steam] Downloading app 413150'),
    jobLog(10, '[steam] Progress: 42/100 files - 4.2 GB/10 GB (42%)'),
  ],
)
assert.equal(downloadingInlineInstall.mode, 'active')
assert.equal(downloadingInlineInstall.stepIndex, 3)
assert.equal(downloadingInlineInstall.percent, 42)
assert.equal(downloadingInlineInstall.approximate, false)
assert.equal(downloadingInlineInstall.overallPercent, 50, '42% of the game download is not 42% of the full installation')
assert.equal(idleInlineInstall.overallPercent, 0)
assert.equal(authorizationInlineInstall.overallPercent, 20, 'waiting for Guard stays at a fixed overall milestone')

const overallFor = (phase: string, logs: JobLog[] = []) => gameInstallProgressPresentation(
  instanceState('installing', phase), activeInlineInstallJob, logs,
).overallPercent
assert.equal(overallFor('pull_running', [jobLog(11, '[pull:progress:1:1]')]), 15)
assert.equal(overallFor('steamcmd_image_pulling', [jobLog(12, '[pull:progress:1:1]')]), 20)
assert.equal(overallFor('steamcmd_downloading', [
  jobLog(13, '[steamcmd] [100%] Downloading update (100 of 100 KB)...'),
]), 20, 'SteamCMD self-update must not be counted as game download completion')
assert.equal(overallFor('steamcmd_downloading'), 20, 'pre-login phase stays at the authorization milestone')
assert.equal(overallFor('steamcmd_guard_mobile_required'), 20)
assert.equal(overallFor('steamcmd_downloading', [
  jobLog(14, "[steamcmd] Success! App '413150' fully installed."),
]), 85, 'game completion reserves progress for SDK and SMAPI')
assert.equal(overallFor('steamcmd_downloading', [
  jobLog(14, "[steamcmd] Success! App '413150' fully installed."),
  jobLog(15, "[steamcmd] Success! App '1007' fully installed."),
]), 90)
assert.equal(overallFor('smapi_installing', [jobLog(16, '[smapi:download:progress:100:100:1:1:true]')]), 99,
  'even a complete SMAPI archive must leave room for installation verification')
assert.equal(overallFor('preparing'), 0, 'a new job resets overall progress')
assert.equal(overallFor('game_installed'), 99, 'an active job must not show overall completion')

const completeInlineInstall = gameInstallProgressPresentation(instanceState('game_installed'), null, [])
assert.equal(completeInlineInstall.mode, 'complete')
assert.equal(completeInlineInstall.percent, 100)
assert.equal(completeInlineInstall.overallPercent, 100)

const missingRuntimeInlineInstall = gameInstallProgressPresentation(instanceState('stopped'), null, [], true)
assert.equal(missingRuntimeInlineInstall.mode, 'failed')
assert.equal(missingRuntimeInlineInstall.stepIndex, 3)
assert.equal(missingRuntimeInlineInstall.steps[3], 'error')
assert.equal(missingRuntimeInlineInstall.detail, '游戏运行文件不完整，请重新安装或修复。')
assert.equal(missingRuntimeInlineInstall.overallPercent, 0, 'a missing runtime must not inherit a stale 100%')

const failedInlineJob = { ...job('job_inline_failed', 'failed', '2026-08-18T00:01:00.000Z'), errorMessage: '下载校验失败' }
const failedInlineInstall = gameInstallProgressPresentation(
  instanceState('error', 'download_failed'),
  failedInlineJob,
  [],
)
assert.equal(failedInlineInstall.mode, 'failed')
assert.equal(failedInlineInstall.steps[3], 'error')
assert.equal(failedInlineInstall.detail, '下载校验失败')
const badPasswordJob = { ...failedInlineJob, errorMessage: 'SteamCMD install exited with code 5' }
const badPasswordLog = { ...jobLog(21, '[steamcmd] Logging in user [REDACTED]...ERROR (Invalid Password)'), jobId: badPasswordJob.id }
assert.equal(gameInstallProgressPresentation(instanceState('error', 'credentials_required'), badPasswordJob, [badPasswordLog]).detail,
  'Steam 账号或密码错误，请修改后重试。')
assert.equal(gameInstallProgressPresentation(instanceState('error', 'steamcmd_failed'), badPasswordJob, [{ ...badPasswordLog, jobId: 'older-job' }]).detail,
  'SteamCMD install exited with code 5', 'another job must not supply the password diagnosis')
assert.equal(gameInstallProgressPresentation({ ...instanceState('error', 'credentials_required'), stateMessage: 'Steam 验证码不正确，请重新验证。' }, badPasswordJob, []).detail,
  'Steam 验证码不正确，请重新验证。', 'preserve the credential diagnosis instead of generic exit code')
assert.equal(gameInstallStepProgressLabel(failedInlineInstall), '请重试')
assert.equal(gameInstallStepProgressLabel(authorizationInlineInstall), '进行中')
assert.equal(gameInstallStepProgressLabel(completeInlineInstall), '100%')
assert.equal(gameInstallStepProgressLabel(idleInlineInstall), '等待安装')
const timedOutInstall = gameInstallProgressPresentation(
  instanceState('installing', 'steamcmd_guard_mobile_required'),
  { ...failedInlineJob, errorMessage: 'SteamCMD install authorization confirmation timed out' },
  [],
)
assert.equal(timedOutInstall.mode, 'failed', 'terminal job must override stale authorization state')
assert.equal(timedOutInstall.detail, 'Steam 登录授权确认超时，请重新安装并及时完成 Steam 验证。')
assert.equal(gameInstallStepProgressLabel(timedOutInstall), '请重试')
assert.equal(timedOutInstall.steps[2], 'error')
assert.equal(gameInstallStepProgressLabel({ ...failedInlineInstall, percent: 42 }), '请重试')
assert.equal(gameInstallProgressPresentation(
  instanceState('game_installed'), { ...failedInlineJob, status: 'canceled' }, [],
).mode, 'failed', 'canceled jobs must not be mistaken for completion')

// Invite authorization keeps the installed game state throughout Guard. Only
// the authorization job can prove completion, including after a page reload.
for (const state of ['game_installed', 'stopped', 'ready_to_start']) {
  for (const phase of ['steam_guard_required', 'steam_guard_mobile_required', 'auth_method_required']) {
    const authJob = { ...job('auth_second_world', 'running', '2026-09-05T00:00:00Z'), type: 'stardew_steam_auth', targetId: 'farm-02' }
    const active = gameInstallProgressPresentation(instanceState(state, phase), authJob, [])
    assert.equal(active.mode, 'active', `${state}/${phase} must keep authorization visible`)
    assert.equal(active.percent, null)
    assert.notEqual(active.overallPercent, 100)
    assert.equal(gameInstallProgressPresentation(instanceState(state, phase), { ...authJob, status: 'succeeded' }, []).mode, 'complete')
    for (const status of ['failed', 'canceled'] as const) {
      assert.equal(gameInstallProgressPresentation(instanceState(state, phase), { ...authJob, status }, []).mode, 'failed')
    }
  }
}
