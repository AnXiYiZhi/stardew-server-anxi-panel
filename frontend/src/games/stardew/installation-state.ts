import type { InstallationDiagnostic, InstanceState } from '../../types'

export type InstallationClassificationKind =
  | 'installed'
  | 'installing'
  | 'not_installed'
  | 'install_failed'
  | 'repair_required'
  | 'runtime_error'
  | 'unknown'

export type InstallationClassificationAction =
  | 'none'
  | InstallationDiagnostic['recommendedAction']

export type InstallationClassificationReason =
  | 'installed_state'
  | 'active_install_job'
  | 'not_installed'
  | 'install_failed'
  | 'required_files_missing'
  | 'deployment_incomplete'
  | 'control_mismatch'
  | 'runtime_error'
  | 'diagnostic_unknown'
  | 'state_unknown'

export type InstallationClassification = {
  kind: InstallationClassificationKind
  action: InstallationClassificationAction
  reason: InstallationClassificationReason
  isInstalled: boolean
  showMissingInstallPrompt: boolean
  canOpenInstallForm: boolean
}

const INSTALLED_STATES = new Set([
  'game_installed',
  'save_required',
  'ready_to_start',
  'starting',
  'running',
  'stopping',
  'stopped',
])

const PRE_INSTALL_STATES = new Set(['uninitialized', 'admin_created', 'junimo_scaffolded'])

const INCOMPLETE_INSTALL_STATES = new Set([
  'credentials_required',
  'steam_auth_running',
  'steam_auth_failed',
  'steam_auth_done',
])

const INSTALL_FAILURE_PHASES = new Set([
  'pull_failed',
  'install_timeout',
  'credentials_required',
  'steam_auth_failed',
  'qr_auth_failed',
  'steam_auth_console_failed',
  'steam_auth_connection_failed',
  'install_interrupted',
  'download_failed',
  'post_auth_failed',
  'smapi_install_failed',
  'smapi_bundled_sync_failed',
  'steamcmd_failed',
  'steamcmd_image_pull_failed',
])

function result(
  kind: InstallationClassificationKind,
  action: InstallationClassificationAction,
  reason: InstallationClassificationReason,
  isInstalled = false,
): InstallationClassification {
  return {
    kind,
    action,
    reason,
    isInstalled,
    showMissingInstallPrompt: kind === 'not_installed',
    canOpenInstallForm: kind === 'installed'
      || kind === 'not_installed'
      || kind === 'install_failed'
      || kind === 'repair_required',
  }
}

function classifyDiagnostic(
  state: InstanceState,
  diagnostic: InstallationDiagnostic,
): InstallationClassification {
  if (diagnostic.status === 'not_installed') {
    // A contradictory positive file check is not safe evidence for either an
    // install or a repair action. Keep the UI diagnostic-only until refreshed.
    if (diagnostic.requiredFiles === 'ok' || diagnostic.serverContainer === 'running') {
      return result('unknown', 'diagnose', 'diagnostic_unknown')
    }
    return result('not_installed', 'install', 'not_installed')
  }

  // Lifecycle state distinguishes a never-started/scaffold-only install from
  // a damaged installation. A pulled image or generated Compose file alone is
  // not evidence that this instance has credentials or game data.
  if (diagnostic.status !== 'installed' && PRE_INSTALL_STATES.has(state.state)) {
    return result('not_installed', 'install', 'not_installed')
  }

  if (diagnostic.status !== 'installed' && INCOMPLETE_INSTALL_STATES.has(state.state)) {
    return result('install_failed', 'install', 'install_failed')
  }

  if (diagnostic.serverContainer === 'running' && diagnostic.status === 'incomplete') {
    return result('runtime_error', 'diagnose', 'diagnostic_unknown')
  }

  if (diagnostic.requiredFiles === 'missing') {
    return result('repair_required', 'repair_install', 'required_files_missing')
  }

  if (diagnostic.compose === 'missing'
    || diagnostic.compose === 'invalid'
    || diagnostic.image === 'missing') {
    return result('repair_required', 'repair_install', 'deployment_incomplete')
  }

  if (diagnostic.compose === 'unavailable' || diagnostic.image === 'unavailable') {
    return result(
      'unknown',
      'diagnose',
      'diagnostic_unknown',
      diagnostic.status === 'installed' && diagnostic.requiredFiles === 'ok',
    )
  }

  if (diagnostic.control.static === 'mismatch'
    || diagnostic.control.static === 'missing'
    || diagnostic.control.runtime === 'mismatch'
    || diagnostic.control.runtime === 'invalid') {
    return result('runtime_error', 'diagnose', 'control_mismatch', diagnostic.status === 'installed')
  }

  if (diagnostic.status === 'incomplete') {
    return result('repair_required', 'repair_install', 'deployment_incomplete')
  }

  if (diagnostic.status === 'installed') {
    if (state.state === 'error' || state.uiStatus === 'failed') {
      const action = diagnostic.recommendedAction === 'retry_start' ? 'retry_start' : 'diagnose'
      return result('runtime_error', action, 'runtime_error', true)
    }
    return result('installed', 'none', 'installed_state', true)
  }

  return result('unknown', 'diagnose', 'diagnostic_unknown')
}

export function classifyInstallationState(
  state: InstanceState | null | undefined,
  hasActiveInstallJob = false,
): InstallationClassification {
  if (!state) return result('unknown', 'diagnose', 'state_unknown')

  if (hasActiveInstallJob) {
    const isInstalled = state.installationDiagnostic?.status === 'installed'
      || INSTALLED_STATES.has(state.state)
    return result('installing', 'none', 'active_install_job', isInstalled)
  }

  if (state.installationDiagnostic) {
    return classifyDiagnostic(state, state.installationDiagnostic)
  }

  if (INSTALLED_STATES.has(state.state)) {
    return result('installed', 'none', 'installed_state', true)
  }

  if (PRE_INSTALL_STATES.has(state.state)) {
    return result('not_installed', 'install', 'not_installed')
  }

  if (INCOMPLETE_INSTALL_STATES.has(state.state)) {
    return result('install_failed', 'install', 'install_failed')
  }

  if (state.state === 'error') {
    if (state.driverPhase === 'install_verification_failed') {
      // Legacy API fallback. New backends must provide installationDiagnostic;
      // older versions only distinguish a transient verifier error from a
      // confirmed missing-file result in the persisted message.
      if (state.stateMessage?.includes('运行文件不完整')) {
        return result('repair_required', 'repair_install', 'required_files_missing')
      }
      return result('runtime_error', 'diagnose', 'runtime_error')
    }
    if (INSTALL_FAILURE_PHASES.has(state.driverPhase)) {
      return result('install_failed', 'install', 'install_failed')
    }
    return result('runtime_error', 'diagnose', 'runtime_error')
  }

  return result('unknown', 'diagnose', 'state_unknown')
}
