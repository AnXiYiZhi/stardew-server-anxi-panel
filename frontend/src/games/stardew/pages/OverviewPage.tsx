import { LifecycleIcon } from '../LifecycleIcon'
import { useEffect, useState } from 'react'
import './OverviewPage.css'
import { approvePlayerAuth, getInstancePasswordStatus, getJunimoUpdate, getRuntimeComponents, peekInstanceRead } from '../../../api'
import type { InstancePasswordStatus, JunimoUpdateInfo, RuntimeComponentsInfo, StardewPlayerInfo } from '../../../types'
import { errorMessage, stateLabel, formatDate } from '../../../core/helpers'
import { jobErrorSummary, jobEventLabel, shortEventTime } from '../../../core/job-presentation'
import { ModalPortal } from '../../../core/ModalPortal'
import { InviteCodeCard } from '../InviteCodeCard'
import { LanDirectConnectCard } from '../LanDirectConnectCard'
import { ServerRuntimeSettingsDialog } from '../ServerRuntimeSettingsDialog'
import { modIsSystemRuntime } from '../mod-visibility'
import { panelUpdateSurface } from '../panel-update-machine'
import type { StardewPageProps } from '../stardew-routes'
import { useStardewLifecycleActions } from '../useStardewLifecycleActions'
import { useServerRuntimeSettings } from '../useServerRuntimeSettings'
import { formatStardewLocation } from '../location-format'
import { shouldShowRuntimeComponentsUpdate } from '../runtime-components-status'
import { shouldShowSMAPIUpdate } from '../smapi-update-status'
import { submitAndWaitForPlayerCommand, type PlayerCommandFeedback } from '../player-command-results'

const OVERVIEW_ICONS = {
  server: '/assets/stardew/ui/icons/icon_nav_server_rack_image2.png',
  saves: '/assets/stardew/ui/icons/icon_nav_saves_chest_image2.png',
  mods: '/assets/stardew/ui/icons/icon_nav_mods_crystal_image2.png',
  health: '/assets/stardew/ui/icons/icon_right_rail_health_heart_image2.optimized.webp',
  tasks: '/assets/stardew/ui/icons/icon_nav_tasks_scroll_image2.png',
  players: '/assets/stardew/ui/icons/icon_nav_players_avatar_image2.png',
} as const

function isPendingApproval(player: StardewPlayerInfo) {
  return !player.isHost && player.status === 'online' && player.isAuthenticated === false
}

export function OverviewPage({ user, instanceId, instanceState, onNavigate, dashboardData }: StardewPageProps) {
  const isAdmin = user.role === 'admin'
  const [passwordStatus, setPasswordStatus] = useState<InstancePasswordStatus | null>(null)
  const [approveTarget, setApproveTarget] = useState<StardewPlayerInfo | null>(null)
  const [approveBusyId, setApproveBusyId] = useState<string | null>(null)
  const [approveFeedback, setApproveFeedback] = useState<PlayerCommandFeedback | null>(null)
  const [junimoUpdate, setJunimoUpdate] = useState<JunimoUpdateInfo | null>(() => isAdmin ? peekInstanceRead<JunimoUpdateInfo>('junimo-update', instanceId) ?? null : null)
  const [runtimeComponents, setRuntimeComponents] = useState<RuntimeComponentsInfo | null>(() => isAdmin ? peekInstanceRead<RuntimeComponentsInfo>('runtime-components', instanceId) ?? null : null)
  const {
    state,
    isRunning,
    actionBusy,
    actionError,
    showSaveRequiredPrompt,
    confirmAction,
    startupInProgress,
    waitingForStop,
    handleStart,
    handleRestart,
    requestConfirm,
    cancelConfirm,
    confirmPendingAction,
  } = useStardewLifecycleActions({ instanceState, dashboardData, isAdmin })
  const {
    runtimeSettingsOpen,
    runtimeSettingsDraft,
    setRuntimeSettingsDraft,
    runtimeSettingsLoading,
    runtimeSettingsSaving,
    runtimeSettingsSavingAction,
    runtimeSettingsError,
    runtimeSettingsMessage,
    clearRuntimeSettingsFeedback,
    openRuntimeSettings,
    closeRuntimeSettings,
    handleSaveRuntimeSettings,
  } = useServerRuntimeSettings({
    isAdmin,
    isRunning,
    refreshPlayers: dashboardData.refreshPlayers,
    restartServer: handleRestart,
  })

  const activeSave = dashboardData.saves?.activeSaveName ?? null
  const activeFarm = dashboardData.saves?.saves.find((save) => save.name === activeSave || save.isActive)?.farmName || activeSave || '未选择存档'
  const saveCount = dashboardData.saves?.saves.length ?? 0
  const visibleMods = dashboardData.mods?.mods.filter((m) => !modIsSystemRuntime(m)) ?? []
  const modCount = visibleMods.length
  const enabledModCount = visibleMods.filter((m) => m.enabled).length
  const disabledModCount = modCount - enabledModCount
  const onlineCount = dashboardData.players?.onlineCount
  const showPlayerEmptyState = !dashboardData.playersError && onlineCount === 0
  const maxPlayers = dashboardData.players?.maxPlayers
  const playerSummary =
    onlineCount != null
      ? maxPlayers != null
        ? `${onlineCount} / ${maxPlayers}`
        : String(onlineCount)
      : state === 'running'
        ? '识别中'
        : '—'
  const onlinePlayers = (dashboardData.players?.players.filter((player) => player.status === 'online') ?? [])
    .sort((a, b) => Number(Boolean(b.isHost)) - Number(Boolean(a.isHost)) || Number(isPendingApproval(b)) - Number(isPendingApproval(a)))
  const priorityPlayerCount = onlinePlayers.filter((player) => player.isHost || isPendingApproval(player)).length
  const hasPendingApproval = onlinePlayers.some(isPendingApproval)
  const canApprove = isAdmin && isRunning && passwordStatus?.enabled === true && passwordStatus.passwordBridgeAvailable === true
  const approveTargetAvailable = onlinePlayers.some((player) => player.uniqueMultiplayerId === approveTarget?.uniqueMultiplayerId && isPendingApproval(player))
  const updateSurface = panelUpdateSurface(dashboardData.updateStatus, dashboardData.updateApply, dashboardData.versionInfo)
  const currentPanelVersion = updateSurface.currentVersion || '—'

  useEffect(() => {
    setPasswordStatus(null)
    if (!isRunning || !hasPendingApproval || !isAdmin) return
    let alive = true
    getInstancePasswordStatus(instanceId).then((result) => {
      if (alive) setPasswordStatus(result)
    }).catch(() => {
      if (alive) setPasswordStatus(null)
    })
    return () => { alive = false }
  }, [instanceId, isRunning, hasPendingApproval, isAdmin])

  async function handleApprove() {
    const target = approveTarget
    if (!target?.uniqueMultiplayerId || !canApprove || !approveTargetAvailable || approveBusyId) return
    setApproveBusyId(target.uniqueMultiplayerId)
    setApproveFeedback({ kind: 'processing', message: '处理中…' })
    try {
      const feedback = await submitAndWaitForPlayerCommand(
        () => approvePlayerAuth(target.uniqueMultiplayerId!, instanceId),
        'approve-auth',
        target.name,
        setApproveFeedback,
      )
      if (feedback.kind === 'succeeded') await dashboardData.refreshPlayers()
    } catch (error) {
      setApproveFeedback({ kind: 'failed', message: errorMessage(error) })
    } finally {
      setApproveBusyId(null)
      setApproveTarget(null)
    }
  }

  useEffect(() => {
    if (!isAdmin) {
      setJunimoUpdate(null)
      setRuntimeComponents(null)
      return
    }
    let alive = true
    const controller = new AbortController()
    getJunimoUpdate(instanceId, controller.signal).then((result) => {
      if (alive) setJunimoUpdate(result)
    }).catch(() => {
      if (alive) setJunimoUpdate(null)
    })
    getRuntimeComponents(instanceId, controller.signal).then((result) => { if (alive) setRuntimeComponents(result) }).catch(() => { if (alive) setRuntimeComponents(null) })
    return () => { alive = false; controller.abort() }
  }, [instanceId, isAdmin])

  const healthChecks = dashboardData.health?.checks ?? []
  const healthStatus = dashboardData.health?.status
  const errorCount = healthChecks.filter((c) => c.status === 'error').length
  const warnCount = healthChecks.filter((c) => c.status === 'warning').length
  const healthLabel = healthStatus === 'error' ? `${errorCount} 项异常` : healthStatus === 'warning' ? `${warnCount} 项警告` : healthStatus === 'ok' ? '正常' : dashboardData.healthError ? '检查失败' : '待检查'

  const recentJobs = dashboardData.jobs.slice(0, 5)

  function renderLifecycleButtons() {
    if (!state) return null
    // Startup succeeds when the host player is online (save loaded/playable), not
    // merely when the container is running, and not when an invite code arrives
    // (invite codes are optional/background and may never appear).
    if (state === 'save_required') {
      return (
        <button className="sd-btn-start" disabled>
          <LifecycleIcon action="start" />
          启动
        </button>
      )
    }

    if (waitingForStop) {
      return (
        <button className="sd-btn-stop sd-btn-loading" disabled>
          <span className="sd-btn-spinner" aria-hidden="true" />
          停止中…
        </button>
      )
    }

    if (startupInProgress) {
      return (
        <button className="sd-btn-start sd-btn-loading" disabled>
          <span className="sd-btn-spinner" aria-hidden="true" />
          启动中…
        </button>
      )
    }

    if (state === 'ready_to_start' || state === 'stopped' || state === 'game_installed') {
      return (
        <button
          className="sd-btn-start"
          onClick={() => void handleStart()}
          disabled={actionBusy || !isAdmin}
          title={isAdmin ? undefined : '仅管理员可启动服务器'}
        >
          <LifecycleIcon action="start" />
          {actionBusy ? '启动中…' : '启动'}
        </button>
      )
    }

    if (state === 'running') {
      return (
        <>
          <button
            className="sd-btn-stop"
            onClick={() => requestConfirm('stop')}
            disabled={actionBusy || !isAdmin}
            title={isAdmin ? undefined : '仅管理员可停止服务器'}
          >
            <LifecycleIcon action="stop" />
            停止
          </button>
          <button
            className="sd-btn-restart"
            onClick={() => requestConfirm('restart')}
            disabled={actionBusy || !isAdmin}
            title={isAdmin ? undefined : '仅管理员可重启服务器'}
          >
            <LifecycleIcon action="restart" />
            重启
          </button>
        </>
      )
    }

    if (state === 'error') {
      return (
        <button className="sd-btn-tan" onClick={() => onNavigate('diagnostics')} disabled={actionBusy}>
          查看诊断
        </button>
      )
    }

    return null
  }

  return (
    <div className="sd-overview">
      <header className="sd-overview-banner">
        <div className="sd-overview-scene" role="img" aria-label="星露谷农场风景" />
        <div className="sd-overview-identity">
          <div className="sd-overview-world">
            <h1 title={instanceState?.name || '星露谷物语'}>{instanceState?.name || '星露谷物语'}</h1>
            <span className={`sd-overview-state${isRunning ? ' is-running' : ''}`}>
              <i aria-hidden="true" />{state ? stateLabel(state) : '状态未知'}
            </span>
          </div>
          <span className="sd-overview-farm" title={activeFarm}>当前农场 · {activeFarm}</span>
          <button className="sd-overview-saves" onClick={() => onNavigate('saves')} title="查看存档" aria-label={dashboardData.savesError ? '存档读取失败，打开存档管理' : `存档 ${dashboardData.saves ? saveCount : '读取中'}，打开存档管理`}>
            <img src={OVERVIEW_ICONS.saves} alt="" />
            <span>存档</span><strong>{dashboardData.savesError ? '读取失败' : dashboardData.saves ? saveCount : '—'}</strong>
          </button>
          <div className="sd-overview-checks">
            <button className="sd-overview-health" onClick={() => onNavigate('diagnostics')}>
              <img src={OVERVIEW_ICONS.health} alt="" />健康检查
              <span className={`sd-overview-health-state is-${healthStatus || 'unknown'}`}>{healthLabel}</span>
            </button>
            <button className={`sd-overview-version is-${updateSurface.tone}`} onClick={dashboardData.openUpdateDialog}
              title={currentPanelVersion === 'dev' ? '开发版本' : `面板版本：${currentPanelVersion}`}>
              <img src="/assets/stardew/ui/icons/icon_top_summary_version.png" alt="" />
              {updateSurface.overviewText}
            </button>
          </div>
        </div>
      </header>

      {junimoUpdate?.available ? (
        <section className="sd-overview-notice" aria-label="游戏运行组件更新提示">
          <div><strong>游戏运行组件可更新</strong>
            <p>{junimoUpdate.recommended.runtimeUpdatePolicy === 'required' ? '此版本为当前面板必需版本，系统会自动完成校验、下载、安装和验收。' : '已有可选更新，可进入版本维护完成升级。'}</p>
          </div>
          <button className="sd-btn-tan" onClick={() => onNavigate('diagnostics')}>进入版本维护</button>
        </section>
      ) : null}
      {shouldShowRuntimeComponentsUpdate(runtimeComponents) ? (
        <section className="sd-overview-notice" aria-label="游戏运行文件更新提示">
          <div><strong>游戏运行文件可更新</strong><p>游戏版本或联机运行库与推荐版本不一致，可先查看更新预检结果。</p></div>
          <button className="sd-btn-tan" onClick={() => onNavigate('diagnostics')}>查看详情</button>
        </section>
      ) : null}
      {shouldShowSMAPIUpdate(runtimeComponents?.smapi) ? (
        <section className="sd-overview-notice" aria-label="模组运行环境更新提示">
          <div><strong>游戏模组运行环境可更新</strong><p>模组运行环境与推荐版本不一致，更新后玩家可能需要重新获取完整同步包。</p></div>
          <button className="sd-btn-tan" onClick={() => onNavigate('diagnostics')}>查看详情</button>
        </section>
      ) : null}

      <section className="sd-overview-control" aria-label="服务器控制">
        <div className="sd-overview-lifecycle">
          <h2><img src={OVERVIEW_ICONS.server} alt="" />服务器控制</h2>
          <div className="sd-overview-actions">{renderLifecycleButtons()}</div>
          {showSaveRequiredPrompt ? (
            <div className="sd-overview-feedback">
              <p>请先创建或上传存档，再启动服务器。</p>
              <button className="sd-btn-green" onClick={() => onNavigate('saves')} disabled={actionBusy}>创建/上传存档</button>
            </div>
          ) : null}
        </div>
        <div className="sd-overview-connections">
          <LanDirectConnectCard dashboardData={dashboardData} />
          {instanceState?.steamInviteEnabled === true ? (
            <InviteCodeCard instanceState={instanceState} dashboardData={dashboardData} canManageSteamInvite={isAdmin} onNavigate={onNavigate} />
          ) : null}
        </div>
        {actionError ? <p className="sd-overview-error" role="alert">{jobErrorSummary(actionError)}</p> : null}
      </section>

      <div className="sd-overview-bottom">
        <section className="sd-overview-panel sd-overview-players">
          <header className="sd-overview-panel-head">
            <h2><img src={OVERVIEW_ICONS.players} alt="" />在线玩家</h2>
            <span className="sd-overview-player-count">{playerSummary}</span>
            {isAdmin ? <button className="sd-overview-detail" onClick={() => { void openRuntimeSettings() }} aria-label="修改联机人数上限">修改上限</button> : null}
          </header>
          <div className={`sd-overview-player-list${onlinePlayers.length ? '' : ' is-empty'}`}>
            {onlinePlayers.length ? onlinePlayers.slice(0, Math.max(4, priorityPlayerCount)).map((player) => (
              <div className="sd-overview-player" key={player.uniqueMultiplayerId || player.name}>
                <span className="sd-overview-avatar" aria-hidden="true">{player.name.slice(0, 1)}</span>
                <span className="sd-overview-player-info"><strong>{player.name}</strong>
                  <span>{isPendingApproval(player) ? '待批准' : formatStardewLocation(player, { fallback: player.isHost ? '农场主' : '在线' })}</span>
                </span>
                {isPendingApproval(player) && isAdmin ? (
                  <button
                    type="button"
                    className="sd-btn-green sd-overview-approve"
                    disabled={!canApprove || !player.uniqueMultiplayerId || approveBusyId !== null}
                    title={!isRunning ? '服务器运行后可批准' : !passwordStatus ? '暂未获取认证状态，请稍后重试或前往玩家页刷新' : !passwordStatus.enabled ? '服务器未开启密码认证' : !passwordStatus.passwordBridgeAvailable ? '密码认证反射桥不可用' : !player.uniqueMultiplayerId ? '缺少玩家联机 ID' : '批准该玩家认证'}
                    aria-label={`批准 ${player.name}`}
                    onClick={() => { setApproveFeedback(null); setApproveTarget(player) }}
                  >{approveBusyId === player.uniqueMultiplayerId ? '批准中…' : '批准'}</button>
                ) : null}
                <i className={`sd-overview-online-dot${isPendingApproval(player) ? ' is-pending' : ''}`} aria-label={isPendingApproval(player) ? '待批准' : '在线'} />
              </div>
            )) : showPlayerEmptyState ? (
              <div className="sd-overview-player-empty" role="status">
                <img src="/assets/stardew/ui/sprites/overview_empty_junimo.webp" width="120" height="80" alt="" />
                <strong>暂无在线玩家</strong>
                <p>{isRunning ? '玩家加入农场后，将显示在这里。' : '服务器启动后，玩家可加入农场。'}</p>
              </div>
            ) : <p className="sd-overview-empty" role="status">{dashboardData.playersError ? '在线玩家读取失败，请稍后重试。' : isRunning ? '正在读取玩家信息…' : '服务器运行后显示在线玩家。'}</p>}
          </div>
          {approveFeedback ? <p className={approveFeedback.kind === 'failed' ? 'sd-overview-error' : 'sd-overview-approval-feedback'} role={approveFeedback.kind === 'failed' ? 'alert' : 'status'}>{approveFeedback.message}</p> : null}
          <button className="sd-overview-link sd-overview-player-more" onClick={() => onNavigate('players')}>查看全部玩家 →</button>
        </section>
        <section className="sd-overview-events sd-overview-panel">
          <header className="sd-overview-panel-head">
            <h2><img src={OVERVIEW_ICONS.tasks} alt="" />近期事件</h2>
            <button className="sd-overview-link" onClick={() => onNavigate('jobs')}>查看全部 →</button>
          </header>
          <div className="sd-overview-event-list">
            {recentJobs.length ? recentJobs.map((job) => (
              <div key={job.id} className={`sd-overview-event is-${job.status}${job.status === 'succeeded' && (job.type === 'stardew_stop' || (job.type === 'stardew_lifecycle' && job.operation === 'stop')) ? ' is-stopped' : ''}`}>
                <i className="sd-overview-event-dot" aria-hidden="true" />
                <span className="sd-overview-event-name" title={jobEventLabel(job)}>{jobEventLabel(job)}</span>
                <time dateTime={job.createdAt} title={formatDate(job.createdAt)}>{shortEventTime(job.createdAt)}</time>
              </div>
            )) : <p className="sd-overview-empty">{dashboardData.loading ? '正在读取事件…' : '暂无事件记录'}</p>}
          </div>
        </section>
        <section className="sd-overview-panel sd-overview-mods">
          <header className="sd-overview-panel-head"><h2><img src={OVERVIEW_ICONS.mods} alt="" />模组状态</h2></header>
          <div className="sd-overview-mod-grid">
            <div><strong className="is-ok">{dashboardData.mods ? enabledModCount : '—'}</strong><span>已启用</span></div>
            <div><strong>{dashboardData.mods ? disabledModCount : '—'}</strong><span>已禁用</span></div>
            <div><strong>—</strong><span>更新待检查</span></div>
            <div><strong className={dashboardData.modsError ? 'is-error' : dashboardData.mods ? 'is-ok' : undefined}>{dashboardData.modsError ? '失败' : dashboardData.mods ? '正常' : '—'}</strong><span>读取状态</span></div>
          </div>
          <button className="sd-btn-tan sd-overview-manage" onClick={() => onNavigate('mods')}>管理模组</button>
        </section>
      </div>

      {approveTarget ? (
        <ModalPortal
          className="sd-confirm-overlay"
          ariaLabelledBy="overview-approve-title"
          onEscape={approveBusyId ? undefined : () => setApproveTarget(null)}
        >
          <div className="sd-confirm-dialog">
            <h3 id="overview-approve-title">确认批准认证</h3>
            <p>批准玩家 {approveTarget.name} 的密码认证？该操作会立即让玩家进入正式农场，等同于服务器替其正确输入了一次密码。</p>
            <div className="sd-confirm-actions">
              <button className="sd-btn-tan" onClick={() => setApproveTarget(null)} disabled={approveBusyId !== null}>取消</button>
              <button className="sd-btn-green" onClick={() => { void handleApprove() }} disabled={!canApprove || !approveTargetAvailable || approveBusyId !== null}>{approveBusyId ? '批准中…' : '确认批准'}</button>
            </div>
          </div>
        </ModalPortal>
      ) : null}

      {runtimeSettingsOpen ? (
        <ServerRuntimeSettingsDialog
          draft={runtimeSettingsDraft}
          setDraft={setRuntimeSettingsDraft}
          loading={runtimeSettingsLoading}
          saving={runtimeSettingsSaving}
          savingAction={runtimeSettingsSavingAction}
          error={runtimeSettingsError}
          message={runtimeSettingsMessage}
          isRunning={isRunning}
          currentMaxPlayers={dashboardData.players?.maxPlayers ?? null}
          onlineCount={dashboardData.players?.onlineCount ?? null}
          onClearFeedback={clearRuntimeSettingsFeedback}
          onClose={closeRuntimeSettings}
          onSave={() => { void handleSaveRuntimeSettings() }}
          onSaveAndRestart={() => { void handleSaveRuntimeSettings(true) }}
        />
      ) : null}

      {/* 危险操作确认弹框 */}
      {confirmAction ? (
        <ModalPortal
          className="sd-confirm-overlay"
          role="alertdialog"
          ariaLabelledBy="overview-lifecycle-confirm-title"
          onEscape={cancelConfirm}
        >
          <div className="sd-confirm-dialog">
            <h3 id="overview-lifecycle-confirm-title">{confirmAction === 'stop' ? '确认停止服务器' : '确认重启服务器'}</h3>
            <p>
              {confirmAction === 'stop'
                ? instanceState?.steamInviteEnabled === true
                  ? '停止服务器将断开所有玩家连接，Steam 邀请码将失效。'
                  : '停止服务器将断开所有玩家连接。'
                : '重启服务器将短暂断开所有玩家连接，请确认操作。'}
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
        </ModalPortal>
      ) : null}
    </div>
  )
}
