import { Fragment } from 'react'
import { stateLabel, formatDate } from '../../../core/helpers'
import { ServerSummaryCard } from '../ServerSummaryCard'
import type { StardewPageProps } from '../stardew-routes'
import { useStardewLifecycleActions } from '../useStardewLifecycleActions'
import { useServerQuickBackup } from '../useServerQuickBackup'
import { useServerRestartSchedule } from '../useServerRestartSchedule'
import { useServerVNCSettings } from '../useServerVNCSettings'
import { useServerPassword } from '../useServerPassword'
import { useServerRuntimeSettings } from '../useServerRuntimeSettings'
import { useServerFestival } from '../useServerFestival'
import { useServerJoja } from '../useServerJoja'
import { useServerConsole } from '../useServerConsole'
import { useServerBroadcast } from '../useServerBroadcast'
import { useServerSaveNow } from '../useServerSaveNow'
import { useGameLanguage } from '../useGameLanguage'
import { STARDEW_GAME_LANGUAGES } from '../game-languages'

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

export function ServerControlPage({ user, instanceState, dashboardData, onNavigate }: StardewPageProps) {
  // ── 状态推导 ──────────────────────────────────────────────────────────────
  const activeSaveName = dashboardData.saves?.activeSaveName ?? ''
  const isAdmin = user.role === 'admin'
  const {
    state,
    isRunning,
    isStarting,
    isStopped,
    startupInProgress,
    waitingForStop,
    actionBusy,
    actionError,
    showSaveRequiredPrompt,
    confirmAction,
    canStart,
    canStop,
    canRestart,
    handleStart,
    requestConfirm,
    cancelConfirm,
    confirmPendingAction,
  } = useStardewLifecycleActions({ instanceState, dashboardData, isAdmin })
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
  const { quickBackupBusy, quickBackupMessage, quickBackupError, handleQuickBackup } = useServerQuickBackup({
    activeSaveName,
    isAdmin,
  })

  const {
    scheduleOpen,
    scheduleDraft,
    setScheduleDraft,
    scheduleLoading,
    scheduleSaving,
    scheduleError,
    scheduleSaved,
    openRestartSchedule,
    closeRestartSchedule,
    handleSaveRestartSchedule,
    toggleScheduleWarning,
  } = useServerRestartSchedule({ isAdmin, refreshJobs: dashboardData.refreshJobs })

  const {
    vncPort,
    vncPortLoading,
    vncDisplayBusy,
    vncRenderingEnabled,
    vncRenderingStatusLoading,
    vncMessage,
    vncError,
    vncDisplayFPS,
    buildVNCControlURL,
    handleToggleVNCDisplay,
    handleOpenVNCControl,
  } = useServerVNCSettings({ isAdmin, isRunning })

  const {
    passwordOpen,
    passwordDraft,
    passwordVisible,
    passwordLoading,
    passwordSaving,
    passwordError,
    passwordMessage,
    passwordStatus,
    passwordStatusLoading,
    passwordStatusError,
    openPasswordSettings,
    closePasswordSettings,
    togglePasswordVisible,
    updatePasswordDraft,
    loadPasswordStatus,
    handleSaveServerPassword,
  } = useServerPassword({ isAdmin })

  const {
    runtimeSettingsOpen,
    runtimeSettingsDraft,
    setRuntimeSettingsDraft,
    runtimeSettingsLoading,
    runtimeSettingsSaving,
    runtimeSettingsError,
    runtimeSettingsMessage,
    setRuntimeSettingsMessage,
    openRuntimeSettings,
    closeRuntimeSettings,
    handleSaveRuntimeSettings,
  } = useServerRuntimeSettings({ isAdmin })

  const {
    gameLanguageOpen,
    setGameLanguageOpen,
    gameLanguageCode,
    setGameLanguageCode,
    gameLanguageLoading,
    gameLanguageSaving,
    gameLanguageError,
    gameLanguageMessage,
    setGameLanguageMessage,
    openGameLanguage,
    saveGameLanguage,
  } = useGameLanguage({ isAdmin, isRunning })

  const { festivalBusy, festivalMessage, festivalError, handleTriggerFestivalEvent } = useServerFestival({
    isAdmin,
    isRunning,
  })
  const { saveNowBusy, saveNowMessage, saveNowError, handleSaveNow } = useServerSaveNow({ isAdmin, isRunning })

  const {
    jojaOpen,
    jojaConfirmInput,
    jojaBusy,
    jojaMessage,
    jojaError,
    jojaConfirmText,
    openJojaConfirm,
    closeJojaConfirm,
    updateJojaConfirmInput,
    fillJojaConfirmText,
    handleEnableJoja,
  } = useServerJoja({ isAdmin, isRunning })

  const {
    commands,
    commandsLoading,
    commandsError,
    selectedCommand,
    commandBusy,
    commandResult,
    commandError,
    loadCommands,
    selectCommand,
    handleRunCommand,
  } = useServerConsole({ isRunning })

  const { sayMessage, setSayMessage, sayBusy, sayResult, sayError, sayConfirmed, handleSay } = useServerBroadcast()

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
                onClick={() => requestConfirm('stop')}
                title={!isAdmin ? '仅管理员可停止服务器' : !isRunning ? '服务器未运行' : '停止服务器（需确认）'}
              >
                <img src="/assets/stardew/ui/icons/icon_button_stop.png" alt="" className="sd-btn-img" />
                停止
              </button>

              <button
                key="restart"
                className="sd-btn-restart"
                disabled={!canRestart}
                onClick={() => requestConfirm('restart')}
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
              <div className={sayConfirmed ? 'sd-srv-result' : 'sd-srv-hint'} style={{ marginTop: 5 }}>{sayResult}</div>
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
                      onChange={(e) => selectCommand(e.target.value)}
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
            key="save-now"
            className="sd-btn-green sd-btn--lg"
            disabled={!isAdmin || !isRunning || saveNowBusy}
            title={
              !isAdmin
                ? '仅管理员可执行此操作'
                : !isRunning
                  ? '服务器运行后才能请求游戏内保存'
                  : '设置游戏内保存请求，并等待 GameLoop.Saved 确认；这不是创建 ZIP 备份'
            }
            onClick={() => void handleSaveNow()}
          >
            <img className="sd-server-quick-icon" src={SERVER_PAGE_ICONS.backup} alt="" />
            <span className="sd-server-quick-copy">
              <strong>{saveNowBusy ? '等待保存…' : '请求游戏内保存'}</strong>
              <span>以 Saved 事件确认完成</span>
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
            key="game-language-settings"
            className="sd-btn-tan sd-btn--lg"
            disabled={!isAdmin}
            title={isAdmin ? '设置服务器游戏与 Mod 消息使用的语言' : '仅管理员可设置服务器游戏语言'}
            onClick={() => void openGameLanguage()}
          >
            <img className="sd-server-quick-icon" src={SERVER_PAGE_ICONS.settings} alt="" />
            <span className="sd-server-quick-copy">
              <strong>服务器游戏语言</strong>
              <span>默认简体中文 / 支持 12 种语言</span>
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
        {saveNowMessage ? (
          <div className={saveNowError ? 'sd-ov-error' : 'sd-srv-result'} style={{ marginTop: 6 }}>
            {saveNowMessage}
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
          “手动备份”只打包当前已经落盘的存档；“请求游戏内保存”只在收到同一 commandId 关联的 Saved 事件后才显示完成，两者不是同一操作。VNC 控制需要先打开显示渲染。完整备份与恢复请前往
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
              <button className="sd-btn-tan" onClick={cancelConfirm}>
                取消
              </button>
              <button
                className={confirmAction === 'stop' ? 'sd-btn-delete' : 'sd-btn-green'}
                onClick={confirmPendingAction}
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
              <button className="sd-btn-tan" onClick={closeRestartSchedule} disabled={scheduleSaving}>
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
                      onChange={(e) => updatePasswordDraft(e.target.value)}
                      disabled={passwordSaving}
                    />
                    <button
                      type="button"
                      className="sd-btn-tan"
                      onClick={togglePasswordVisible}
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
                  <button className="sd-btn-tan" onClick={closePasswordSettings} disabled={passwordSaving}>
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
                  <button className="sd-btn-tan" onClick={closeRuntimeSettings} disabled={runtimeSettingsSaving}>
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

      {gameLanguageOpen ? (
        <div key="game-language" className="sd-confirm-overlay" role="dialog" aria-modal="true" aria-labelledby="game-language-title">
          <div className="sd-confirm-dialog">
            <h3 id="game-language-title">服务器游戏语言</h3>
            {gameLanguageLoading ? (
              <p>正在读取当前语言...</p>
            ) : (
              <>
                <label className="sd-schedule-field">
                  <span>游戏语言</span>
                  <select
                    className="sd-input"
                    value={gameLanguageCode}
                    disabled={gameLanguageSaving}
                    onChange={(event) => {
                      setGameLanguageCode(event.target.value)
                      setGameLanguageMessage(null)
                    }}
                  >
                    {STARDEW_GAME_LANGUAGES.map((language) => (
                      <option key={language.code} value={language.code}>{language.label}</option>
                    ))}
                  </select>
                </label>
                <div className="sd-confirm-warning">
                  决定服务器生成的 Mod 消息、系统文本和聊天通知语言，不影响面板界面语言。修改后需要重新启动服务器才能生效。
                </div>
                {gameLanguageError ? <div className="sd-ov-error">{gameLanguageError}</div> : null}
                {gameLanguageMessage ? <div className="sd-srv-result">{gameLanguageMessage}</div> : null}
                <div className="sd-confirm-actions">
                  <button className="sd-btn-tan" onClick={() => setGameLanguageOpen(false)} disabled={gameLanguageSaving}>关闭</button>
                  <button className="sd-btn-green" onClick={() => void saveGameLanguage(false)} disabled={gameLanguageSaving}>
                    {gameLanguageSaving ? '保存中…' : '保存'}
                  </button>
                  {isRunning ? (
                    <button className="sd-btn-restart" onClick={() => void saveGameLanguage(true)} disabled={gameLanguageSaving}>
                      {gameLanguageSaving ? '处理中…' : '保存并重启'}
                    </button>
                  ) : null}
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
                请输入 <code>{jojaConfirmText}</code> 以确认
              </span>
              <div style={{ display: 'flex', gap: 6 }}>
                <input
                  className="sd-input"
                  type="text"
                  value={jojaConfirmInput}
                  placeholder={jojaConfirmText}
                  onChange={(e) => updateJojaConfirmInput(e.target.value)}
                  disabled={jojaBusy}
                />
                <button
                  type="button"
                  className="sd-btn-tan"
                  onClick={fillJojaConfirmText}
                  disabled={jojaBusy}
                >
                  填入
                </button>
              </div>
            </label>

            {jojaError ? <div className="sd-ov-error">{jojaMessage}</div> : null}
            {!jojaError && jojaMessage ? <div className="sd-srv-result">{jojaMessage}</div> : null}

            <div className="sd-confirm-actions">
              <button className="sd-btn-tan" onClick={closeJojaConfirm} disabled={jojaBusy}>
                取消
              </button>
              <button
                className="sd-btn-delete"
                onClick={() => void handleEnableJoja()}
                disabled={jojaBusy || jojaConfirmInput !== jojaConfirmText}
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
import './ServerControlPage.css'
