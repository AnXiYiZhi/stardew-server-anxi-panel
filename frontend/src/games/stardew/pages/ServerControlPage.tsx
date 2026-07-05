import { Fragment, useState, useEffect, useCallback } from 'react'
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
} from '../../../api'
import { errorMessage, stateLabel, formatDate } from '../../../core/helpers'
import { ServerSummaryCard } from '../ServerSummaryCard'
import type { StardewPageProps } from '../stardew-routes'
import type { ConsoleCommandDef, RestartSchedule } from '../../../types'

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
} as const

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
  const waitingForInvite =
    isStarting ||
    Boolean(pendingStartupAction) ||
    (dashboardData.inviteCodeRefreshing && !dashboardData.inviteCode)
  const waitingForStop = isStopping || pendingStopAction
  const noSavesDetected = Boolean(dashboardData.saves && dashboardData.saves.saves.length === 0)
  const showSaveRequiredPrompt =
    (state === 'save_required' || saveRequiredDetected || noSavesDetected) &&
    !isRunning &&
    !isStarting
  const canStart = isStopped && !actionBusy && !waitingForInvite && !waitingForStop
  const canStop = isRunning && !actionBusy && !waitingForInvite && !waitingForStop && Boolean(dashboardData.inviteCode)
  const canRestart = isRunning && !actionBusy && !waitingForInvite && !waitingForStop && Boolean(dashboardData.inviteCode)
  const stateLabelText = state
    ? stateLabel(state)
    : dashboardData.loading
      ? '读取中…'
      : '未知'
  const lifecycleDotClass = isRunning
    ? 'sd-dot sd-dot-green sd-dot-pulse'
    : state === 'stopped' || state === 'error'
      ? 'sd-dot sd-dot-red'
      : isStarting || waitingForInvite || waitingForStop
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
    if (dashboardData.inviteCode) {
      setPendingStartupAction(null)
    }
  }, [dashboardData.inviteCode])

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
              className={`sd-btn-start${waitingForInvite ? ' sd-btn-loading' : ''}`}
              disabled={waitingForInvite || !canStart}
              onClick={() => void handleStart()}
              title={waitingForInvite ? '服务器启动中，等待邀请码生成' : isRunning ? '服务器已运行' : '启动服务器'}
            >
              {waitingForInvite ? (
                <span className="sd-btn-spinner" aria-hidden="true" />
              ) : (
                <img src="/assets/stardew/ui/icons/icon_button_play.png" alt="" className="sd-btn-img" />
              )}
              {waitingForInvite || (actionBusy && canStart) ? '启动中…' : '启动'}
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
          ) : !waitingForInvite ? (
            <Fragment key="running-actions">
              <button
                key="stop"
                className="sd-btn-stop"
                disabled={!canStop}
                onClick={() => setConfirmAction('stop')}
                title={!isRunning ? '服务器未运行' : '停止服务器（需确认）'}
              >
                <img src="/assets/stardew/ui/icons/icon_button_stop.png" alt="" className="sd-btn-img" />
                停止
              </button>

              <button
                key="restart"
                className="sd-btn-restart"
                disabled={!canRestart}
                onClick={() => setConfirmAction('restart')}
                title={!isRunning ? '服务器未运行' : '重启服务器（需确认）'}
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

        {waitingForInvite ? (
          <div className="sd-srv-hint" style={{ marginTop: 4 }}>
            <span className="sd-dot sd-dot-yellow sd-dot-pulse" aria-hidden="true" />
            &nbsp;服务器正在启动，请等待邀请码生成后再操作。
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
            <div className="sd-srv-hint" style={{ marginTop: 4 }}>
              通过 SMAPI say 命令发送全服公告。注：该命令当前版本可能返回"命令不支持"。
            </div>
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
            key="server-settings"
            className="sd-btn-tan sd-btn--lg"
            disabled
            title="待接入：端口/可见性/密码配置"
          >
            <img className="sd-server-quick-icon" src={SERVER_PAGE_ICONS.settings} alt="" />
            <span className="sd-server-quick-copy">
              <strong>服务器设置</strong>
              <span>待接入</span>
            </span>
            <span className="sd-srv-badge-pending">待接入</span>
          </button>
        </div>
        {quickBackupMessage ? (
          <div className={quickBackupError ? 'sd-ov-error' : 'sd-srv-result'} style={{ marginTop: 6 }}>
            {quickBackupMessage}
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
    </div>
  )
}
