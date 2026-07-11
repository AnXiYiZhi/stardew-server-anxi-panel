import { Fragment, useState, useEffect, useCallback, useRef } from 'react'
import {
  ApiError,
  startInstance,
  stopInstance,
  restartInstance,
  createSaveBackup,
  getInstanceVNCConfig,
  getInstanceRenderingFPS,
  getRestartSchedule,
  setInstanceRenderingFPS,
  updateRestartSchedule,
  getCommands,
  runCommand,
  sendSay,
  getInstanceServerPassword,
  updateInstanceServerPassword,
  getInstancePasswordStatus,
  getInstanceServerRuntimeSettings,
  updateInstanceServerRuntimeSettings,
  triggerFestivalEvent,
  enableJojaRoute,
} from '../../../api'
import { errorMessage, stateLabel, formatDate } from '../../../core/helpers'
import { ServerSummaryCard } from '../ServerSummaryCard'
import type { StardewPageProps } from '../stardew-routes'
import type { ConsoleCommandDef, RestartSchedule, InstancePasswordStatus, ServerRuntimeSettings } from '../../../types'

const defaultRuntimeSettings: ServerRuntimeSettings = {
  cabinStrategy: 'CabinStack',
  existingCabinBehavior: 'KeepExisting',
  networkBroadcastPeriod: 1,
}

const defaultRestartSchedule: RestartSchedule = {
  instanceId: 'stardew',
  enabled: false,
  shutdownTime: '04:00',
  startupTime: '04:20',
  timezone: 'Asia/Shanghai',
  warningMinutes: [10, 5, 1],
  backupBeforeShutdown: true,
  skipIfPlayersOnline: false,
}

const vncDisplayFPS = 15
const SERVER_PAGE_ICONS = {
  title: '/assets/stardew/ui/icons/icon_nav_server_rack_image2.png',
  command: '/assets/stardew/ui/icons/icon_nav_diagnostics_monitor_image2.png',
  backup: '/assets/stardew/ui/icons/icon_nav_saves_chest_image2.png',
  schedule: '/assets/stardew/ui/icons/icon_right_rail_in_progress_clock_image2.png',
  display: '/assets/stardew/ui/icons/icon_nav_diagnostics_monitor_image2.png',
  vnc: '/assets/stardew/ui/icons/icon_dropdown_arrow_gold_image2.png',
  settings: '/assets/stardew/ui/icons/icon_nav_settings_gear_image2.png',
  festival: '/assets/stardew/ui/icons/icon_nav_tasks_scroll_image2.png',
  joja: '/assets/stardew/ui/icons/icon_players_action_permission_image2.png',
} as const

const JOJA_CONFIRM_TEXT = 'IRREVERSIBLY_ENABLE_JOJA_RUN'
// 主机玩家迟迟不上线时的兜底超时：避免像 2026-07-06 那次一样因为玩家快照
// 闪烁/不可用导致按钮永久卡在"启动中…"。实测大存档场景下，容器状态到
// `running`、SMAPI 内部 status.json 到 `save-loaded`、主机真正出现在
// players.json 在线列表里，前后可以相差好几分钟，超时阈值必须留足这个余量，
// 否则会在主机还没真正上线时就提前放行，等于白做了这层确认。
const HOST_ONLINE_WAIT_TIMEOUT_MS = 10 * 60_000

function buildVNCControlURL(port: string) {
  const host = window.location.hostname.includes(':')
    ? `[${window.location.hostname}]`
    : window.location.hostname
  return `http://${host}:${port}/`
}

function saveStartBlocker(error: unknown): 'new' | 'saves' | null {
  if (!(error instanceof ApiError)) return null
  if (error.code === 'save_required') return 'new'
  if (error.code === 'active_save_required' || error.code === 'active_save_missing') return 'saves'
  return null
}

export function ServerControlPage({ user, instanceState, dashboardData, onNavigate }: StardewPageProps) {
  // ── 生命周期操作状态 ──────────────────────────────────────────────────────
  const [actionBusy, setActionBusy] = useState(false)
  const [actionError, setActionError] = useState<string | null>(null)
  const [saveRequiredDetected, setSaveRequiredDetected] = useState(false)
  const [confirmAction, setConfirmAction] = useState<'stop' | 'restart' | null>(null)
  const [pendingStartupAction, setPendingStartupAction] = useState<'start' | 'restart' | null>(null)
  const [pendingStopAction, setPendingStopAction] = useState(false)
  const [quickBackupBusy, setQuickBackupBusy] = useState(false)
  const [quickBackupMessage, setQuickBackupMessage] = useState<string | null>(null)
  const [quickBackupError, setQuickBackupError] = useState(false)
  const [scheduleOpen, setScheduleOpen] = useState(false)
  const [scheduleDraft, setScheduleDraft] = useState<RestartSchedule>(defaultRestartSchedule)
  const [scheduleLoading, setScheduleLoading] = useState(false)
  const [scheduleSaving, setScheduleSaving] = useState(false)
  const [scheduleError, setScheduleError] = useState<string | null>(null)
  const [scheduleSaved, setScheduleSaved] = useState<string | null>(null)
  const [vncPort, setVNCPort] = useState('')
  const [vncPortLoading, setVNCPortLoading] = useState(false)
  const [vncDisplayBusy, setVNCDisplayBusy] = useState(false)
  const [vncRenderingEnabled, setVNCRenderingEnabled] = useState(false)
  const [vncRenderingStatusLoading, setVNCRenderingStatusLoading] = useState(false)
  const [vncMessage, setVNCMessage] = useState<string | null>(null)
  const [vncError, setVNCError] = useState<string | null>(null)

  // ── 服务器密码设置 ────────────────────────────────────────────────────────
  const [passwordOpen, setPasswordOpen] = useState(false)
  const [passwordDraft, setPasswordDraft] = useState('')
  const [passwordVisible, setPasswordVisible] = useState(false)
  const [passwordLoading, setPasswordLoading] = useState(false)
  const [passwordSaving, setPasswordSaving] = useState(false)
  const [passwordError, setPasswordError] = useState<string | null>(null)
  const [passwordMessage, setPasswordMessage] = useState<string | null>(null)
  const [passwordStatus, setPasswordStatus] = useState<InstancePasswordStatus | null>(null)
  const [passwordStatusLoading, setPasswordStatusLoading] = useState(false)
  const [passwordStatusError, setPasswordStatusError] = useState<string | null>(null)

  // ── 小屋与联机高级设置 ────────────────────────────────────────────────────
  const [runtimeSettingsOpen, setRuntimeSettingsOpen] = useState(false)
  const [runtimeSettingsDraft, setRuntimeSettingsDraft] = useState<ServerRuntimeSettings>(defaultRuntimeSettings)
  const [runtimeSettingsLoading, setRuntimeSettingsLoading] = useState(false)
  const [runtimeSettingsSaving, setRuntimeSettingsSaving] = useState(false)
  const [runtimeSettingsError, setRuntimeSettingsError] = useState<string | null>(null)
  const [runtimeSettingsMessage, setRuntimeSettingsMessage] = useState<string | null>(null)

  // ── 触发节日活动 ──────────────────────────────────────────────────────────
  const [festivalBusy, setFestivalBusy] = useState(false)
  const [festivalMessage, setFestivalMessage] = useState<string | null>(null)
  const [festivalError, setFestivalError] = useState(false)

  // ── 永久启用 Joja 路线 ────────────────────────────────────────────────────
  const [jojaOpen, setJojaOpen] = useState(false)
  const [jojaConfirmInput, setJojaConfirmInput] = useState('')
  const [jojaBusy, setJojaBusy] = useState(false)
  const [jojaMessage, setJojaMessage] = useState<string | null>(null)
  const [jojaError, setJojaError] = useState(false)

  // ── 控制台命令 ────────────────────────────────────────────────────────────
  const [commands, setCommands] = useState<ConsoleCommandDef[]>([])
  const [commandsLoading, setCommandsLoading] = useState(false)
  const [commandsError, setCommandsError] = useState<string | null>(null)
  const [selectedCommand, setSelectedCommand] = useState('')
  const [commandBusy, setCommandBusy] = useState(false)
  const [commandResult, setCommandResult] = useState<string | null>(null)
  const [commandError, setCommandError] = useState<string | null>(null)

  // ── 全服喊话 ──────────────────────────────────────────────────────────────
  const [sayMessage, setSayMessage] = useState('')
  const [sayBusy, setSayBusy] = useState(false)
  const [sayResult, setSayResult] = useState<string | null>(null)
  const [sayError, setSayError] = useState<string | null>(null)

  // ── 状态推导 ──────────────────────────────────────────────────────────────
  const state = instanceState?.state ?? null
  const isRunning = state === 'running'
  const isStarting = state === 'starting'
  const isStopping = state === 'stopping'
  const isStopped = state === 'stopped' || state === 'ready_to_start' || state === 'game_installed'
  const activeSaveName = dashboardData.saves?.activeSaveName ?? ''
  const isAdmin = user.role === 'admin'
  const hasActiveLifecycleJob = dashboardData.jobs.some(
    (j) => j.type === 'stardew_lifecycle' && (j.status === 'running' || j.status === 'queued'),
  )
  const activeLifecycleIsStopping = hasActiveLifecycleJob && instanceState?.driverPhase === 'stopping'
  const hostOnline = (dashboardData.players?.players ?? []).some(
    (p) => p.isHost && p.status === 'online',
  )
  // state 已经是 running（无论是本次点击启动，还是打开/刷新页面时服务器早已在
  // 运行）但主机玩家还没出现在在线列表里，继续按"启动中"展示，不受
  // pendingStartupAction 这个仅在当次点击时才有值的本地状态影响。为避免像
  // 2026-07-06 那次一样因为玩家快照闪烁/不可用导致按钮永久转圈，超过
  // HOST_ONLINE_WAIT_TIMEOUT_MS 后无论主机是否上线都会放行。
  const hostWaitStartedAtRef = useRef<number | null>(null)
  const [hostConfirmTimedOut, setHostConfirmTimedOut] = useState(false)
  const awaitingHostConfirmation = isRunning && !hostOnline && !hostConfirmTimedOut
  const startupInProgress =
    isStarting ||
    Boolean(pendingStartupAction) ||
    (hasActiveLifecycleJob && !activeLifecycleIsStopping && !isRunning) ||
    awaitingHostConfirmation
  const waitingForStop = isStopping || pendingStopAction || activeLifecycleIsStopping
  const noSavesDetected = Boolean(dashboardData.saves && dashboardData.saves.saves.length === 0)
  const showSaveRequiredPrompt =
    (state === 'save_required' || saveRequiredDetected || noSavesDetected) &&
    !isRunning &&
    !isStarting
  const canStart = isAdmin && isStopped && !actionBusy && !startupInProgress && !waitingForStop
  const canStop = isAdmin && isRunning && !actionBusy && !waitingForStop
  const canRestart = isAdmin && isRunning && !actionBusy && !waitingForStop
  const stateLabelText = state
    ? stateLabel(state)
    : dashboardData.loading
      ? '读取中…'
      : '未知'
  const lifecycleDotClass = isRunning
    ? 'sd-dot sd-dot-green sd-dot-pulse'
    : state === 'stopped' || state === 'error'
      ? 'sd-dot sd-dot-red'
      : isStarting || startupInProgress || waitingForStop
        ? 'sd-dot sd-dot-yellow sd-dot-pulse'
        : 'sd-dot sd-dot-gray'
  // ── 命令列表：服务器运行时加载一次 ───────────────────────────────────────
  const loadCommands = useCallback(async () => {
    if (!isRunning) {
      setCommands([])
      setCommandsError(null)
      return
    }
    setCommandsLoading(true)
    setCommandsError(null)
    try {
      const res = await getCommands()
      setCommands(res.commands)
      if (res.commands.length > 0 && !selectedCommand) {
        setSelectedCommand(res.commands[0].id || res.commands[0].name)
      }
    } catch (e) {
      setCommandsError(errorMessage(e))
    } finally {
      setCommandsLoading(false)
    }
  }, [isRunning, selectedCommand])

  useEffect(() => {
    void loadCommands()
  }, [loadCommands])

  useEffect(() => {
    if (state && state !== 'save_required') {
      setSaveRequiredDetected(false)
    }
  }, [state])

  useEffect(() => {
    // Startup display follows the backend lifecycle job/state.
    if (!hasActiveLifecycleJob && isRunning) {
      setPendingStartupAction(null)
    }
  }, [hasActiveLifecycleJob, isRunning])

  useEffect(() => {
    if (isRunning && !hostOnline) {
      if (hostWaitStartedAtRef.current === null) {
        hostWaitStartedAtRef.current = Date.now()
        setHostConfirmTimedOut(false)
      } else if (Date.now() - hostWaitStartedAtRef.current >= HOST_ONLINE_WAIT_TIMEOUT_MS) {
        setHostConfirmTimedOut(true)
      }
    } else {
      hostWaitStartedAtRef.current = null
      setHostConfirmTimedOut(false)
    }
  }, [isRunning, hostOnline, dashboardData.players?.updatedAt])

  useEffect(() => {
    if (state === 'stopped' || state === 'ready_to_start' || state === 'game_installed' || state === 'save_required' || state === 'error') {
      setPendingStopAction(false)
    }
  }, [state])

  useEffect(() => {
    if (!isRunning) {
      setVNCRenderingEnabled(false)
      setVNCRenderingStatusLoading(false)
    }
  }, [isRunning])

  useEffect(() => {
    if (!isAdmin || !isRunning) return
    let canceled = false
    setVNCRenderingStatusLoading(true)
    getInstanceRenderingFPS()
      .then((res) => {
        if (canceled) return
        setVNCRenderingEnabled(res.fps > 0)
      })
      .catch((e) => {
        if (canceled) return
        setVNCError(`读取 VNC 显示状态失败：${errorMessage(e)}`)
      })
      .finally(() => {
        if (!canceled) setVNCRenderingStatusLoading(false)
      })
    return () => {
      canceled = true
    }
  }, [isAdmin, isRunning])

  useEffect(() => {
    if (!isAdmin) {
      setVNCPort('')
      return
    }
    let canceled = false
    setVNCPortLoading(true)
    getInstanceVNCConfig()
      .then((res) => {
        if (canceled) return
        setVNCPort(res.vncPort)
      })
      .catch((e) => {
        if (canceled) return
        setVNCError(`读取 VNC 端口失败：${errorMessage(e)}`)
      })
      .finally(() => {
        if (!canceled) setVNCPortLoading(false)
      })
    return () => {
      canceled = true
    }
  }, [isAdmin])

  // ── 生命周期操作 ──────────────────────────────────────────────────────────
  async function handleStart() {
    setActionBusy(true)
    setPendingStartupAction('start')
    setPendingStopAction(false)
    setActionError(null)
    try {
      await startInstance()
      dashboardData.requestInviteCodeRefresh()
      setSaveRequiredDetected(false)
      dashboardData.refreshInstanceState()
      dashboardData.refreshJobs()
    } catch (e) {
      const saveBlocker = saveStartBlocker(e)
      if (saveBlocker) {
        setSaveRequiredDetected(saveBlocker === 'new')
        setActionError(saveBlocker === 'new' ? null : errorMessage(e))
        dashboardData.refreshInstanceState()
        dashboardData.refreshSaves()
        setPendingStartupAction(null)
        return
      }
      setActionError(errorMessage(e))
      setPendingStartupAction(null)
    } finally {
      setActionBusy(false)
    }
  }

  async function handleStop() {
    setActionBusy(true)
    setPendingStopAction(true)
    setPendingStartupAction(null)
    setActionError(null)
    dashboardData.clearInviteCode()
    try {
      await stopInstance()
      dashboardData.refreshInstanceState()
      dashboardData.refreshJobs()
    } catch (e) {
      setActionError(errorMessage(e))
      setPendingStopAction(false)
    } finally {
      setActionBusy(false)
    }
  }

  async function handleRestart() {
    setActionBusy(true)
    setPendingStartupAction('restart')
    setActionError(null)
    try {
      await restartInstance()
      dashboardData.requestInviteCodeRefresh()
      dashboardData.refreshInstanceState()
      dashboardData.refreshJobs()
    } catch (e) {
      setActionError(errorMessage(e))
      setPendingStartupAction(null)
    } finally {
      setActionBusy(false)
    }
  }

  async function handleQuickBackup() {
    if (!activeSaveName || !isAdmin) return
    setQuickBackupBusy(true)
    setQuickBackupMessage(null)
    setQuickBackupError(false)
    try {
      const result = await createSaveBackup(activeSaveName)
      setQuickBackupMessage(`已为 ${activeSaveName} 创建手动备份：${result.backupName}`)
    } catch (e) {
      setQuickBackupError(true)
      setQuickBackupMessage(errorMessage(e))
    } finally {
      setQuickBackupBusy(false)
    }
  }

  async function handleTriggerFestivalEvent() {
    if (!isAdmin || !isRunning) return
    setFestivalBusy(true)
    setFestivalMessage(null)
    setFestivalError(false)
    try {
      const result = await triggerFestivalEvent()
      setFestivalMessage(result.output?.trim() || '触发节日活动指令已提交。')
    } catch (e) {
      setFestivalError(true)
      setFestivalMessage(errorMessage(e))
    } finally {
      setFestivalBusy(false)
    }
  }

  function openJojaConfirm() {
    if (!isAdmin || !isRunning) return
    setJojaConfirmInput('')
    setJojaMessage(null)
    setJojaError(false)
    setJojaOpen(true)
  }

  async function handleEnableJoja() {
    if (jojaConfirmInput !== JOJA_CONFIRM_TEXT) return
    setJojaBusy(true)
    setJojaMessage(null)
    setJojaError(false)
    try {
      const result = await enableJojaRoute(jojaConfirmInput)
      setJojaMessage(result.output?.trim() || 'Joja 路线已永久启用。')
    } catch (e) {
      setJojaError(true)
      setJojaMessage(errorMessage(e))
    } finally {
      setJojaBusy(false)
    }
  }

  async function handleToggleVNCDisplay() {
    if (!isAdmin || !isRunning) return
    const nextEnabled = !vncRenderingEnabled
    const nextFPS = nextEnabled ? vncDisplayFPS : 0
    setVNCDisplayBusy(true)
    setVNCMessage(null)
    setVNCError(null)
    try {
      const result = await setInstanceRenderingFPS(nextFPS)
      setVNCRenderingEnabled(nextEnabled)
      setVNCMessage(
        nextEnabled
          ? `VNC 显示已打开（${result.fps} FPS），现在可以跳转到 VNC 控制。`
          : 'VNC 显示已关闭。'
      )
    } catch (e) {
      setVNCError(errorMessage(e))
    } finally {
      setVNCDisplayBusy(false)
    }
  }

  function handleOpenVNCControl() {
    if (!isAdmin || !isRunning || !vncPort) return
    setVNCError(null)
    const opened = window.open(buildVNCControlURL(vncPort), '_blank')
    if (!opened) {
      setVNCError('浏览器拦截了 VNC 控制窗口，请允许弹出窗口后重试。')
      return
    }
    opened.opener = null
    setVNCMessage(`已打开 VNC 控制页面（端口 ${vncPort}）。`)
  }

  // ── 邀请码复制 ────────────────────────────────────────────────────────────
  async function openRestartSchedule() {
    if (!isAdmin) return
    setScheduleOpen(true)
    setScheduleLoading(true)
    setScheduleSaving(false)
    setScheduleError(null)
    setScheduleSaved(null)
    try {
      const result = await getRestartSchedule()
      setScheduleDraft(result.schedule)
    } catch (e) {
      setScheduleError(errorMessage(e))
      setScheduleDraft(defaultRestartSchedule)
    } finally {
      setScheduleLoading(false)
    }
  }

  async function handleSaveRestartSchedule() {
    setScheduleSaving(true)
    setScheduleError(null)
    setScheduleSaved(null)
    try {
      const result = await updateRestartSchedule(scheduleDraft)
      setScheduleDraft(result.schedule)
      setScheduleSaved('计划重启已保存。')
      dashboardData.refreshJobs()
    } catch (e) {
      setScheduleError(errorMessage(e))
    } finally {
      setScheduleSaving(false)
    }
  }

  async function openPasswordSettings() {
    if (!isAdmin) return
    setPasswordOpen(true)
    setPasswordVisible(false)
    setPasswordLoading(true)
    setPasswordSaving(false)
    setPasswordError(null)
    setPasswordMessage(null)
    try {
      const res = await getInstanceServerPassword()
      setPasswordDraft(res.serverPassword)
    } catch (e) {
      setPasswordError(errorMessage(e))
      setPasswordDraft('')
    } finally {
      setPasswordLoading(false)
    }
    void loadPasswordStatus()
  }

  async function loadPasswordStatus() {
    setPasswordStatusLoading(true)
    setPasswordStatusError(null)
    try {
      const res = await getInstancePasswordStatus()
      setPasswordStatus(res)
    } catch (e) {
      setPasswordStatus(null)
      setPasswordStatusError(errorMessage(e))
    } finally {
      setPasswordStatusLoading(false)
    }
  }

  async function handleSaveServerPassword() {
    if (passwordDraft.length > 128) {
      setPasswordError('服务器密码不能超过 128 个字符')
      setPasswordMessage(null)
      return
    }
    setPasswordSaving(true)
    setPasswordError(null)
    setPasswordMessage(null)
    try {
      const res = await updateInstanceServerPassword(passwordDraft)
      setPasswordDraft(res.serverPassword)
      setPasswordMessage('密码已保存，需要重启服务器容器后才会生效。')
    } catch (e) {
      setPasswordError(errorMessage(e))
    } finally {
      setPasswordSaving(false)
    }
  }

  async function openRuntimeSettings() {
    if (!isAdmin) return
    setRuntimeSettingsOpen(true)
    setRuntimeSettingsLoading(true)
    setRuntimeSettingsSaving(false)
    setRuntimeSettingsError(null)
    setRuntimeSettingsMessage(null)
    try {
      const res = await getInstanceServerRuntimeSettings()
      setRuntimeSettingsDraft(res)
    } catch (e) {
      setRuntimeSettingsError(errorMessage(e))
      setRuntimeSettingsDraft(defaultRuntimeSettings)
    } finally {
      setRuntimeSettingsLoading(false)
    }
  }

  async function handleSaveRuntimeSettings() {
    setRuntimeSettingsSaving(true)
    setRuntimeSettingsError(null)
    setRuntimeSettingsMessage(null)
    try {
      const res = await updateInstanceServerRuntimeSettings(runtimeSettingsDraft)
      setRuntimeSettingsDraft(res)
      setRuntimeSettingsMessage('设置已保存，需要重启服务器容器后才会生效。')
    } catch (e) {
      setRuntimeSettingsError(errorMessage(e))
    } finally {
      setRuntimeSettingsSaving(false)
    }
  }

  function toggleScheduleWarning(minute: number) {
    setScheduleDraft((draft) => {
      const exists = draft.warningMinutes.includes(minute)
      const next = exists
        ? draft.warningMinutes.filter((value) => value !== minute)
        : [...draft.warningMinutes, minute]
      next.sort((a, b) => b - a)
      return { ...draft, warningMinutes: next }
    })
  }

  // ── 执行控制台命令 ────────────────────────────────────────────────────────
  async function handleRunCommand() {
    if (!selectedCommand) return
    setCommandBusy(true)
    setCommandResult(null)
    setCommandError(null)
    try {
      const res = await runCommand(selectedCommand)
      setCommandResult(res.output?.trim() || '命令已执行（无输出）')
    } catch (e) {
      setCommandError(errorMessage(e))
    } finally {
      setCommandBusy(false)
    }
  }

  // ── 全服喊话 ──────────────────────────────────────────────────────────────
  async function handleSay() {
    if (!sayMessage.trim()) return
    setSayBusy(true)
    setSayResult(null)
    setSayError(null)
    try {
      const res = await sendSay(sayMessage.trim())
      setSayResult(res.output?.trim() || '消息已发送')
      setSayMessage('')
    } catch (e) {
      setSayError(errorMessage(e))
    } finally {
      setSayBusy(false)
    }
  }

  const selectedCommandDef = commands.find((c) => c.id === selectedCommand)
  const terminalLines = commandResult
    ? commandResult
    : commandError
      ? `命令执行失败：${commandError}`
      : commandsError
        ? `命令列表加载失败：${commandsError}`
        : isRunning
          ? '等待命令输出...\n选择左侧命令并点击执行，结果会显示在这里。'
          : '服务器未运行。\n启动服务器后可执行 allowlist 控制台命令。'

  return (
    <div className="sd-page sd-server-page">
      {/* ── 页面标题 ───────────────────────────────────────────────────────── */}
      <div key="page-header" className="sd-page-header">
        <img
          className="sd-page-icon"
          src={SERVER_PAGE_ICONS.title}
          alt=""
        />
        <div>
          <h2 className="sd-page-title">服务器控制</h2>
        </div>
      </div>

      {/* ── 服务器摘要卡片 ─────────────────────────────────────────────────── */}
      <ServerSummaryCard
        key="summary"
        instanceState={instanceState}
        dashboardData={dashboardData}
        className="sd-server-summary-card"
      />

      {/* ── 生命周期控制 ───────────────────────────────────────────────────── */}
      <div key="lifecycle" className="sd-srv-section sd-server-lifecycle">
        <div className="sd-srv-section-title">
          生命周期控制
          <span className="sd-server-title-sprout" aria-hidden="true">⌘</span>
        </div>
        <div className="sd-ctrl-row">
          {!waitingForStop ? (
            <button
              key="start"
              className={`sd-btn-start${startupInProgress ? ' sd-btn-loading' : ''}`}
              disabled={startupInProgress || !canStart}
              onClick={() => void handleStart()}
              title={
                !isAdmin
                  ? '仅管理员可启动服务器'
                  : startupInProgress
                    ? '服务器启动中，正在加载存档'
                    : isRunning
                      ? '服务器已运行'
                      : '启动服务器'
              }
            >
              {startupInProgress ? (
                <span className="sd-btn-spinner" aria-hidden="true" />
              ) : (
                <img src="/assets/stardew/ui/icons/icon_button_play.png" alt="" className="sd-btn-img" />
              )}
              {startupInProgress || (actionBusy && canStart) ? '启动中…' : '启动'}
            </button>
          ) : null}

          {showSaveRequiredPrompt ? (
            <div key="save-required" className="sd-start-save-required">
              <span>当前没有存档，请点击此按钮去创建/上传存档。</span>
              <button className="sd-btn-green" onClick={() => onNavigate('saves')} disabled={actionBusy}>
                创建/上传存档
              </button>
            </div>
          ) : null}

          {waitingForStop ? (
            <button key="stopping" className="sd-btn-stop sd-btn-loading" disabled>
              <span className="sd-btn-spinner" aria-hidden="true" />
              停止中…
            </button>
          ) : !startupInProgress ? (
            <Fragment key="running-actions">
              <button
                key="stop"
                className="sd-btn-stop"
                disabled={!canStop}
                onClick={() => setConfirmAction('stop')}
                title={!isAdmin ? '仅管理员可停止服务器' : !isRunning ? '服务器未运行' : '停止服务器（需确认）'}
              >
                <img src="/assets/stardew/ui/icons/icon_button_stop.png" alt="" className="sd-btn-img" />
                停止
              </button>

              <button
                key="restart"
                className="sd-btn-restart"
                disabled={!canRestart}
                onClick={() => setConfirmAction('restart')}
                title={!isAdmin ? '仅管理员可重启服务器' : !isRunning ? '服务器未运行' : '重启服务器（需确认）'}
              >
                <img src="/assets/stardew/ui/icons/icon_button_restart.png" alt="" className="sd-btn-img" />
                重启
              </button>
            </Fragment>
          ) : null}

          {actionBusy ? (
            <span key="busy-hint" className="sd-srv-hint" style={{ marginLeft: 6 }}>
              <span className="sd-dot sd-dot-yellow sd-dot-pulse" aria-hidden="true" />
              操作进行中，请稍候…
            </span>
          ) : null}
        </div>

        {actionError ? (
          <div className="sd-ov-error" style={{ marginTop: 6 }}>{actionError}</div>
        ) : null}

        <div className="sd-server-lifecycle-status">
          状态
          <span className={lifecycleDotClass} aria-hidden="true" />
          <span className={`sd-server-lifecycle-status-val sd-server-lifecycle-status-val-${state ?? 'unknown'}`}>
            {stateLabelText}
          </span>
        </div>

        {startupInProgress ? (
          <div className="sd-srv-hint" style={{ marginTop: 4 }}>
            <span className="sd-dot sd-dot-yellow sd-dot-pulse" aria-hidden="true" />
            &nbsp;服务器正在启动，等待主机玩家上线后再操作。
          </div>
        ) : null}

        {waitingForStop ? (
          <div className="sd-srv-hint" style={{ marginTop: 4 }}>
            <span className="sd-dot sd-dot-yellow sd-dot-pulse" aria-hidden="true" />
            &nbsp;服务器正在停止，请等待完全停止后再启动。
          </div>
        ) : null}

        {state && !isRunning && !isStopped && !isStarting && !showSaveRequiredPrompt ? (
          <div className="sd-srv-hint" style={{ marginTop: 4 }}>
            当前状态（{stateLabelText}）下无法直接启动服务器，请先完成安装或选择存档。
          </div>
        ) : null}
      </div>

      {/* ── 全服喊话 ───────────────────────────────────────────────────────── */}
      <div key="broadcast" className="sd-srv-section sd-server-broadcast">
        <div className="sd-srv-section-title">
          全服消息
          <span className="sd-server-title-sprout" aria-hidden="true">⌘</span>
        </div>
        {isRunning ? (
          <>
            <div className="sd-server-message-row">
              <input
                className="sd-input"
                type="text"
                placeholder="向所有在线玩家发送消息…"
                value={sayMessage}
                onChange={(e) => setSayMessage(e.target.value)}
                disabled={sayBusy}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') void handleSay()
                }}
              />
              <span className="sd-server-message-count">{sayMessage.length}/120</span>
              <button
                className="sd-btn-green"
                onClick={() => void handleSay()}
                disabled={sayBusy || !sayMessage.trim()}
              >
                {sayBusy ? '发送中…' : '发送'}
              </button>
            </div>
            {sayResult ? (
              <div className="sd-srv-result" style={{ marginTop: 5 }}>{sayResult}</div>
            ) : null}
            {sayError ? (
              <div className="sd-ov-error" style={{ marginTop: 4 }}>{sayError}</div>
            ) : null}
          </>
        ) : (
          <div className="sd-srv-empty">服务器运行时可向在线玩家发送全服消息。</div>
        )}
      </div>

      {/* ── 控制台命令 ─────────────────────────────────────────────────────── */}
      <div key="command" className="sd-srv-section sd-server-command">
        <div className="sd-srv-section-title">
          <img className="sd-server-section-icon" src={SERVER_PAGE_ICONS.command} alt="" />
          控制台命令
          <span className="sd-server-title-sprout" aria-hidden="true">⌘</span>
        </div>
        <div className="sd-server-command-body">
          <div className="sd-server-command-controls">
            {isRunning ? (
              commandsError ? (
                <div className="sd-srv-empty" style={{ color: '#c02020' }}>
                  加载命令列表失败：{commandsError}
                  <button
                    className="sd-btn-tan sd-btn--sm"
                    style={{ marginLeft: 8 }}
                    onClick={() => void loadCommands()}
                  >
                    重试
                  </button>
                </div>
              ) : commandsLoading ? (
                <div className="sd-srv-empty">正在加载可用命令列表…</div>
              ) : commands.length > 0 ? (
                <>
                  <div className="sd-server-command-row">
                    <select
                      className="sd-input"
                      value={selectedCommand}
                      onChange={(e) => {
                        setSelectedCommand(e.target.value)
                        setCommandResult(null)
                        setCommandError(null)
                      }}
                      disabled={commandBusy}
                    >
                      {commands.map((cmd) => {
                        const commandId = cmd.id || cmd.name
                        return (
                        <option key={commandId} value={commandId}>
                          {cmd.name}{cmd.adminOnly ? ' (仅管理员)' : ''}
                        </option>
                        )
                      })}
                    </select>
                    <button
                      className="sd-btn-green"
                      onClick={() => void handleRunCommand()}
                      disabled={commandBusy || !selectedCommand}
                    >
                      {commandBusy ? '执行中…' : '执行'}
                    </button>
                  </div>
                  {selectedCommandDef?.description ? (
                    <div className="sd-srv-hint" style={{ marginTop: 4 }}>
                      {selectedCommandDef.description}
                    </div>
                  ) : null}
                </>
              ) : (
                <div className="sd-srv-empty">服务器未返回可用命令，可能尚未完全就绪。</div>
              )
            ) : (
              <div className="sd-srv-empty">服务器运行时可执行 SMAPI 控制台命令（allowlist 限制）。</div>
            )}
          </div>
          <div className="sd-server-terminal" aria-live="polite">
            <div className="sd-server-terminal-head">
              <span>实时输出</span>
              <span className="sd-server-terminal-live">
                <span className="sd-dot sd-dot-green" aria-hidden="true" />
                实时输出
              </span>
            </div>
            <pre>{terminalLines}</pre>
          </div>
        </div>
      </div>

      {/* ── 快捷操作 ─────────────────────────────────────────────────────── */}
      <div key="quick" className="sd-srv-section sd-server-quick">
        <div className="sd-srv-section-title">
          快捷操作
          <span className="sd-server-title-sprout" aria-hidden="true">⌘</span>
        </div>
        <div className="sd-server-quick-grid">
          <button
            key="manual-backup"
            className="sd-btn-green sd-btn--lg"
            disabled={quickBackupBusy || !isAdmin || !activeSaveName}
            title={
              !isAdmin
                ? '仅管理员可执行此操作'
                : !activeSaveName
                  ? '当前没有激活存档，无法创建备份'
                  : `为当前激活存档 ${activeSaveName} 备份已保存到磁盘的进度；不会强制保存游戏内实时进度`
            }
            onClick={() => void handleQuickBackup()}
          >
            <img className="sd-server-quick-icon" src={SERVER_PAGE_ICONS.backup} alt="" />
            <span className="sd-server-quick-copy">
              <strong>{quickBackupBusy ? '备份中…' : '手动备份'}</strong>
              <span>备份当前存档</span>
            </span>
          </button>
          <button
            key="restart-schedule"
            className="sd-btn-tan sd-btn--lg"
            disabled={!isAdmin}
            title={isAdmin ? '设置每天几点关闭、几点开启服务器' : '仅管理员可设置计划重启'}
            onClick={() => void openRestartSchedule()}
          >
            <img className="sd-server-quick-icon" src={SERVER_PAGE_ICONS.schedule} alt="" />
            <span className="sd-server-quick-copy">
              <strong>计划重启</strong>
              <span>设置定时重启</span>
            </span>
          </button>
          <button
            key="toggle-vnc-display"
            className={`${vncRenderingEnabled ? 'sd-btn-tan' : 'sd-btn-green'} sd-btn--lg`}
            disabled={!isAdmin || !isRunning || vncDisplayBusy || vncRenderingStatusLoading}
            title={
              !isAdmin
                ? '仅管理员可控制 VNC 显示'
                : !isRunning
                  ? '服务器运行后才能控制 VNC 显示'
                  : vncRenderingStatusLoading
                    ? '正在读取 VNC 显示状态'
                  : vncRenderingEnabled
                    ? '关闭 Junimo 服务端画面渲染'
                    : `通过 Junimo API 开启服务端画面渲染（${vncDisplayFPS} FPS）`
            }
            onClick={() => void handleToggleVNCDisplay()}
          >
            <img className="sd-server-quick-icon" src={SERVER_PAGE_ICONS.display} alt="" />
            <span className="sd-server-quick-copy">
              <strong>
                {vncDisplayBusy
                  ? vncRenderingEnabled
                    ? '关闭中…'
                    : '打开中…'
                  : vncRenderingStatusLoading
                    ? '读取VNC状态…'
                    : vncRenderingEnabled
                      ? '关闭VNC显示'
                      : '打开VNC显示'}
              </strong>
              <span>远程桌面显示</span>
            </span>
            {vncRenderingEnabled ? <span className="sd-server-quick-status">已启用</span> : null}
          </button>
          {vncRenderingEnabled ? (
            <button
              key="open-vnc-control"
              className="sd-btn-tan sd-btn--lg"
              disabled={!isAdmin || !isRunning || vncPortLoading || !vncPort}
              title={
                !isAdmin
                  ? '仅管理员可进入 VNC 控制'
                  : !isRunning
                    ? '服务器运行后才能进入 VNC 控制'
                    : vncPortLoading
                      ? '正在读取 VNC 端口'
                      : vncPort
                        ? `打开 ${buildVNCControlURL(vncPort)}`
                        : '未读取到 VNC 端口'
              }
              onClick={handleOpenVNCControl}
            >
              <img className="sd-server-quick-icon" src={SERVER_PAGE_ICONS.vnc} alt="" />
              <span className="sd-server-quick-copy">
                <strong>{vncPortLoading ? '读取端口…' : '跳转VNC控制'}</strong>
                <span>打开浏览器 VNC 控制台</span>
              </span>
            </button>
          ) : null}
          <button
            key="server-password-settings"
            className="sd-btn-tan sd-btn--lg"
            disabled={!isAdmin}
            title={isAdmin ? '设置玩家加入服务器所需的密码' : '仅管理员可设置服务器密码'}
            onClick={() => void openPasswordSettings()}
          >
            <img className="sd-server-quick-icon" src={SERVER_PAGE_ICONS.settings} alt="" />
            <span className="sd-server-quick-copy">
              <strong>服务器密码设置</strong>
              <span>配置玩家加入密码</span>
            </span>
          </button>
          <button
            key="server-runtime-settings"
            className="sd-btn-tan sd-btn--lg"
            disabled={!isAdmin}
            title={isAdmin ? '配置小屋策略与联机广播频率' : '仅管理员可配置小屋与联机高级设置'}
            onClick={() => void openRuntimeSettings()}
          >
            <img className="sd-server-quick-icon" src={SERVER_PAGE_ICONS.settings} alt="" />
            <span className="sd-server-quick-copy">
              <strong>小屋与联机高级设置</strong>
              <span>小屋策略 / 广播频率</span>
            </span>
          </button>
          <button
            key="trigger-festival-event"
            className="sd-btn-tan sd-btn--lg"
            disabled={!isAdmin || !isRunning || festivalBusy}
            title={
              !isAdmin
                ? '仅管理员可执行此操作'
                : !isRunning
                  ? '服务器运行后才能触发节日活动'
                  : '模拟游戏内 !event 指令，强制开始当天节日的主活动（若当天没有节日则不会生效）'
            }
            onClick={() => void handleTriggerFestivalEvent()}
          >
            <img className="sd-server-quick-icon" src={SERVER_PAGE_ICONS.festival} alt="" />
            <span className="sd-server-quick-copy">
              <strong>{festivalBusy ? '触发中…' : '触发节日活动'}</strong>
              <span>卡住时强制开始</span>
            </span>
          </button>
          <button
            key="enable-joja-route"
            className="sd-btn-delete sd-btn--lg"
            disabled={!isAdmin || !isRunning}
            title={
              !isAdmin
                ? '仅管理员可执行此操作'
                : !isRunning
                  ? '服务器运行后才能启用 Joja 路线'
                  : '永久启用 Joja 路线并禁用标准社区中心，此操作不可撤销'
            }
            onClick={openJojaConfirm}
          >
            <img className="sd-server-quick-icon" src={SERVER_PAGE_ICONS.joja} alt="" />
            <span className="sd-server-quick-copy">
              <strong>永久启用 Joja 路线</strong>
              <span>不可撤销，请谨慎操作</span>
            </span>
          </button>
        </div>
        {quickBackupMessage ? (
          <div className={quickBackupError ? 'sd-ov-error' : 'sd-srv-result'} style={{ marginTop: 6 }}>
            {quickBackupMessage}
          </div>
        ) : null}
        {festivalMessage ? (
          <div className={festivalError ? 'sd-ov-error' : 'sd-srv-result'} style={{ marginTop: 6 }}>
            {festivalMessage}
          </div>
        ) : null}
        {vncMessage ? (
          <div className="sd-srv-result" style={{ marginTop: 6 }}>
            {vncMessage}
          </div>
        ) : null}
        {vncError ? (
          <div className="sd-ov-error" style={{ marginTop: 6 }}>
            {vncError}
          </div>
        ) : null}
        <div className="sd-srv-hint" style={{ marginTop: 6 }}>
          备份只会打包当前已经落盘的激活存档，运行中也可用，但不会强制保存尚未写盘的游戏进度。VNC 控制需要先打开显示渲染。完整备份与恢复请前往
          <button
            className="sd-inline-nav"
            style={{ marginLeft: 2 }}
            onClick={() => onNavigate('saves')}
          >
            存档页
          </button>。
        </div>
      </div>

      {/* ── 危险操作确认弹框 ───────────────────────────────────────────────── */}
      {confirmAction ? (
        <div key="confirm" className="sd-confirm-overlay">
          <div className="sd-confirm-dialog">
            <h3>{confirmAction === 'stop' ? '确认停止服务器' : '确认重启服务器'}</h3>
            <p>
              {confirmAction === 'stop'
                ? '停止服务器将断开所有在线玩家的连接，邀请码将立即失效。此操作不可撤销，确认继续？'
                : '重启服务器将短暂中断所有玩家的连接。重启完成后服务器会自动恢复，确认继续？'}
            </p>
            <div className="sd-confirm-actions">
              <button className="sd-btn-tan" onClick={() => setConfirmAction(null)}>
                取消
              </button>
              <button
                className={confirmAction === 'stop' ? 'sd-btn-delete' : 'sd-btn-green'}
                onClick={() => {
                  const action = confirmAction
                  setConfirmAction(null)
                  void (action === 'stop' ? handleStop() : handleRestart())
                }}
              >
                确认{confirmAction === 'stop' ? '停止' : '重启'}
              </button>
            </div>
          </div>
        </div>
      ) : null}

      {scheduleOpen ? (
        <div key="schedule" className="sd-confirm-overlay" role="dialog" aria-modal="true">
          <div className="sd-confirm-dialog sd-confirm-dialog-wide">
            <h3>计划重启</h3>
            {scheduleLoading ? (
              <p>正在读取计划重启配置...</p>
            ) : (
              <>
                <div className="sd-schedule-grid">
                  <label className="sd-schedule-check">
                    <input
                      type="checkbox"
                      checked={scheduleDraft.enabled}
                      onChange={(e) => setScheduleDraft({ ...scheduleDraft, enabled: e.target.checked })}
                    />
                    启用每日计划维护
                  </label>

                  <label className="sd-schedule-field">
                    <span>关闭时间</span>
                    <input
                      className="sd-input"
                      type="time"
                      value={scheduleDraft.shutdownTime}
                      onChange={(e) => setScheduleDraft({ ...scheduleDraft, shutdownTime: e.target.value })}
                    />
                  </label>

                  <label className="sd-schedule-field">
                    <span>开启时间</span>
                    <input
                      className="sd-input"
                      type="time"
                      value={scheduleDraft.startupTime}
                      onChange={(e) => setScheduleDraft({ ...scheduleDraft, startupTime: e.target.value })}
                    />
                  </label>

                  <label className="sd-schedule-field">
                    <span>时区</span>
                    <input
                      className="sd-input"
                      value={scheduleDraft.timezone}
                      onChange={(e) => setScheduleDraft({ ...scheduleDraft, timezone: e.target.value })}
                    />
                  </label>

                  <div className="sd-schedule-field sd-schedule-field-wide">
                    <span>关服前提醒</span>
                    <div className="sd-schedule-options">
                      {[10, 5, 1].map((minute) => (
                        <label key={minute} className="sd-schedule-check">
                          <input
                            type="checkbox"
                            checked={scheduleDraft.warningMinutes.includes(minute)}
                            onChange={() => toggleScheduleWarning(minute)}
                          />
                          {minute} 分钟
                        </label>
                      ))}
                    </div>
                  </div>

                  <label className="sd-schedule-check sd-schedule-field-wide">
                    <input
                      type="checkbox"
                      checked={scheduleDraft.backupBeforeShutdown}
                      onChange={(e) => setScheduleDraft({ ...scheduleDraft, backupBeforeShutdown: e.target.checked })}
                    />
                    关闭前备份当前已保存进度
                  </label>

                  <label className="sd-schedule-check sd-schedule-field-wide">
                    <input
                      type="checkbox"
                      checked={scheduleDraft.skipIfPlayersOnline}
                      onChange={(e) => setScheduleDraft({ ...scheduleDraft, skipIfPlayersOnline: e.target.checked })}
                    />
                    如果仍有玩家在线则跳过本次关闭
                  </label>
                </div>

                <div className="sd-confirm-warning">
                  关闭时间到达后，面板会先按配置发送提醒、备份当前已经落盘的存档，再提交停止任务；开启时间到达后会按当前激活存档提交启动任务。
                </div>

                <div className="sd-schedule-summary">
                  <div>下次关闭：{scheduleDraft.nextShutdownAt ? formatDate(scheduleDraft.nextShutdownAt) : '未启用'}</div>
                  <div>下次开启：{scheduleDraft.nextStartupAt ? formatDate(scheduleDraft.nextStartupAt) : '未启用'}</div>
                  <div>上次状态：{scheduleDraft.lastStatus ?? '暂无记录'}</div>
                  {scheduleDraft.lastMessage ? <div>说明：{scheduleDraft.lastMessage}</div> : null}
                </div>
              </>
            )}

            {scheduleError ? <div className="sd-ov-error">{scheduleError}</div> : null}
            {scheduleSaved ? <div className="sd-srv-result">{scheduleSaved}</div> : null}

            <div className="sd-confirm-actions">
              <button className="sd-btn-tan" onClick={() => setScheduleOpen(false)} disabled={scheduleSaving}>
                取消
              </button>
              <button
                className="sd-btn-green"
                onClick={() => void handleSaveRestartSchedule()}
                disabled={scheduleLoading || scheduleSaving}
              >
                {scheduleSaving ? '保存中…' : '保存'}
              </button>
            </div>
          </div>
        </div>
      ) : null}

      {passwordOpen ? (
        <div key="password" className="sd-confirm-overlay" role="dialog" aria-modal="true">
          <div className="sd-confirm-dialog">
            <h3>服务器密码设置</h3>

            {passwordLoading ? (
              <p>正在读取当前密码配置...</p>
            ) : (
              <>
                <label className="sd-schedule-field">
                  <span>加入密码</span>
                  <div style={{ display: 'flex', gap: 6 }}>
                    <input
                      className="sd-input"
                      type={passwordVisible ? 'text' : 'password'}
                      value={passwordDraft}
                      placeholder="留空表示不设置密码"
                      maxLength={128}
                      onChange={(e) => {
                        setPasswordDraft(e.target.value)
                        setPasswordMessage(null)
                      }}
                      disabled={passwordSaving}
                    />
                    <button
                      type="button"
                      className="sd-btn-tan"
                      onClick={() => setPasswordVisible((v) => !v)}
                    >
                      {passwordVisible ? '隐藏' : '显示'}
                    </button>
                  </div>
                </label>

                <div className="sd-confirm-warning">
                  该密码仅在服务器容器启动时生效（JunimoServer 不支持运行时热改）。保存后需要重启服务器容器才会真正生效；玩家加入时需要在游戏内输入 <code>!login 密码</code>。
                </div>

                {passwordError ? <div className="sd-ov-error">{passwordError}</div> : null}
                {passwordMessage ? <div className="sd-srv-result">{passwordMessage}</div> : null}

                <div className="sd-confirm-actions">
                  <button className="sd-btn-tan" onClick={() => setPasswordOpen(false)} disabled={passwordSaving}>
                    关闭
                  </button>
                  <button
                    className="sd-btn-green"
                    onClick={() => void handleSaveServerPassword()}
                    disabled={passwordSaving}
                  >
                    {passwordSaving ? '保存中…' : '保存'}
                  </button>
                </div>

                <div className="sd-schedule-summary" style={{ marginTop: 12 }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                    <strong>密码保护状态（来自 JunimoServer）</strong>
                    <button
                      type="button"
                      className="sd-btn-tan"
                      onClick={() => void loadPasswordStatus()}
                      disabled={passwordStatusLoading || !isRunning}
                    >
                      {passwordStatusLoading ? '读取中…' : '刷新'}
                    </button>
                  </div>
                  {!isRunning ? (
                    <div>服务器未运行，无法读取密码保护状态。</div>
                  ) : passwordStatusError ? (
                    <div className="sd-ov-error">{passwordStatusError}</div>
                  ) : passwordStatus ? (
                    <>
                      <div>是否启用：{passwordStatus.enabled ? '已启用' : '未启用'}</div>
                      <div>已认证玩家：{passwordStatus.authenticatedCount}　待认证玩家：{passwordStatus.pendingCount}</div>
                      <div>认证超时：{passwordStatus.timeoutSeconds} 秒　最大失败次数：{passwordStatus.maxAttempts}</div>
                    </>
                  ) : (
                    <div>暂无数据。</div>
                  )}
                </div>
              </>
            )}
          </div>
        </div>
      ) : null}

      {runtimeSettingsOpen ? (
        <div key="runtime-settings" className="sd-confirm-overlay" role="dialog" aria-modal="true">
          <div className="sd-confirm-dialog">
            <h3>小屋与联机高级设置</h3>

            {runtimeSettingsLoading ? (
              <p>正在读取当前配置...</p>
            ) : (
              <>
                <label className="sd-schedule-field">
                  <span>小屋策略（CabinStrategy）</span>
                  <select
                    className="sd-input"
                    value={runtimeSettingsDraft.cabinStrategy}
                    disabled={runtimeSettingsSaving}
                    onChange={(e) => {
                      setRuntimeSettingsDraft((draft) => ({ ...draft, cabinStrategy: e.target.value }))
                      setRuntimeSettingsMessage(null)
                    }}
                  >
                    <option value="CabinStack">CabinStack（隐藏小屋堆叠，最适合大多数服务器）</option>
                    <option value="FarmhouseStack">FarmhouseStack（隐藏小屋，从主农舍共用入口出）</option>
                    <option value="None">None（原版行为，小屋放置在真实农场位置）</option>
                  </select>
                </label>

                <label className="sd-schedule-field">
                  <span>已有小屋处理方式（ExistingCabinBehavior）</span>
                  <select
                    className="sd-input"
                    value={runtimeSettingsDraft.existingCabinBehavior}
                    disabled={runtimeSettingsSaving}
                    onChange={(e) => {
                      setRuntimeSettingsDraft((draft) => ({ ...draft, existingCabinBehavior: e.target.value }))
                      setRuntimeSettingsMessage(null)
                    }}
                  >
                    <option value="KeepExisting">KeepExisting（保留已有小屋位置）</option>
                    <option value="MoveToStack">MoveToStack（把已有小屋迁移到策略指定位置）</option>
                  </select>
                </label>

                <label className="sd-schedule-field">
                  <span>网络广播频率（NetworkBroadcastPeriod，单位：刻）</span>
                  <select
                    className="sd-input"
                    value={runtimeSettingsDraft.networkBroadcastPeriod}
                    disabled={runtimeSettingsSaving}
                    onChange={(e) => {
                      setRuntimeSettingsDraft((draft) => ({ ...draft, networkBroadcastPeriod: Number(e.target.value) }))
                      setRuntimeSettingsMessage(null)
                    }}
                  >
                    <option value={1}>1（每刻广播，最实时）</option>
                    <option value={2}>2</option>
                    <option value={3}>3（原版频率）</option>
                  </select>
                </label>

                <div className="sd-confirm-warning">
                  这些设置写入 server-settings.json，JunimoServer 只在容器启动时读取。保存后需要重启服务器容器才会生效，对已有存档同样适用。
                </div>

                {runtimeSettingsError ? <div className="sd-ov-error">{runtimeSettingsError}</div> : null}
                {runtimeSettingsMessage ? <div className="sd-srv-result">{runtimeSettingsMessage}</div> : null}

                <div className="sd-confirm-actions">
                  <button className="sd-btn-tan" onClick={() => setRuntimeSettingsOpen(false)} disabled={runtimeSettingsSaving}>
                    关闭
                  </button>
                  <button
                    className="sd-btn-green"
                    onClick={() => void handleSaveRuntimeSettings()}
                    disabled={runtimeSettingsSaving}
                  >
                    {runtimeSettingsSaving ? '保存中…' : '保存'}
                  </button>
                </div>
              </>
            )}
          </div>
        </div>
      ) : null}

      {jojaOpen ? (
        <div key="joja" className="sd-confirm-overlay" role="dialog" aria-modal="true">
          <div className="sd-confirm-dialog">
            <h3>永久启用 Joja 路线</h3>

            <div className="sd-confirm-warning">
              此操作会模拟游戏内 <code>!joja IRREVERSIBLY_ENABLE_JOJA_RUN</code> 指令，永久禁用标准社区中心路线，改为 Joja 路线。<strong>此操作不可撤销</strong>，对本存档的剩余游玩时间永久生效。请仅在你确实需要切换路线时使用。
            </div>

            <label className="sd-schedule-field">
              <span>
                请输入 <code>{JOJA_CONFIRM_TEXT}</code> 以确认
              </span>
              <div style={{ display: 'flex', gap: 6 }}>
                <input
                  className="sd-input"
                  type="text"
                  value={jojaConfirmInput}
                  placeholder={JOJA_CONFIRM_TEXT}
                  onChange={(e) => {
                    setJojaConfirmInput(e.target.value)
                    setJojaMessage(null)
                  }}
                  disabled={jojaBusy}
                />
                <button
                  type="button"
                  className="sd-btn-tan"
                  onClick={() => {
                    setJojaConfirmInput(JOJA_CONFIRM_TEXT)
                    setJojaMessage(null)
                  }}
                  disabled={jojaBusy}
                >
                  填入
                </button>
              </div>
            </label>

            {jojaError ? <div className="sd-ov-error">{jojaMessage}</div> : null}
            {!jojaError && jojaMessage ? <div className="sd-srv-result">{jojaMessage}</div> : null}

            <div className="sd-confirm-actions">
              <button className="sd-btn-tan" onClick={() => setJojaOpen(false)} disabled={jojaBusy}>
                取消
              </button>
              <button
                className="sd-btn-delete"
                onClick={() => void handleEnableJoja()}
                disabled={jojaBusy || jojaConfirmInput !== JOJA_CONFIRM_TEXT}
              >
                {jojaBusy ? '提交中…' : '确认永久启用'}
              </button>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  )
}
