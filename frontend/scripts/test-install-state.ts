import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { calcSteamDownloadTaskProgress, extractSMAPIArchiveProgress } from '../src/games/stardew/install-helpers.ts'
import { canonicalInstallJobs, logsDescribeActiveInstall, reconcileJobSnapshots } from '../src/games/stardew/install-state.ts'
import { classifyInstallationState } from '../src/games/stardew/installation-state.ts'
import type { InstallationDiagnostic, InstanceState, Job, JobLog, JobStatus } from '../src/types.ts'

const installPageSource = readFileSync(
  new URL('../src/games/stardew/pages/InstallPage.tsx', import.meta.url),
  'utf8',
).replace(/\r\n?/g, '\n')

assert.match(installPageSource, /const \[forceReauth, setForceReauth\] = useState\(false\)/)
assert.match(installPageSource, /onClick=\{\(\) => \{ setForceReauth\(true\); setShowForm\(true\); setInstallError\(''\) \}\}/)
assert.match(installPageSource, /forceReauth\s*\? \{ steamUsername, steamPassword, vncPassword, imageTag, forceReauth: true \}/)
assert.match(installPageSource, /installation\.canOpenInstallForm \|\| forceReauth/)
assert.match(installPageSource, /!canDirectRetry \|\| forceReauth/)
assert.match(installPageSource, /确认更换账号并重新认证/)

function job(id: string, status: JobStatus, updatedAt: string, createdAt = '2026-08-11T00:00:00.000Z'): Job {
  return {
    id,
    type: 'stardew_install',
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
    ...overrides,
  }
}

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
    state: instanceState('steam_auth_failed', 'credentials_required', {
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
    name: 'known installer error offers a retry without a first-install prompt',
    state: instanceState('error', 'steam_auth_failed'),
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
