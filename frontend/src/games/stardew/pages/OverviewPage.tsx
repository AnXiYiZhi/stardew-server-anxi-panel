import { useEffect, useState } from 'react'
import { ApiError, startInstance, stopInstance, restartInstance } from '../../../api'
import { errorMessage, stateLabel, formatDate, jobDisplayName } from '../../../core/helpers'
import { InviteCodeCard } from '../InviteCodeCard'
import { modIsSystemRuntime } from '../mod-visibility'
import type { StardewPageProps } from '../stardew-routes'

const OVERVIEW_ICONS = {
  server: '/assets/stardew/ui/icons/icon_nav_server_rack_image2.png',
  saves: '/assets/stardew/ui/icons/icon_nav_saves_chest_image2.png',
  mods: '/assets/stardew/ui/icons/icon_nav_mods_crystal_image2.png',
  health: '/assets/stardew/ui/icons/icon_right_rail_health_heart_image2.png',
  tasks: '/assets/stardew/ui/icons/icon_nav_tasks_scroll_image2.png',
  players: '/assets/stardew/ui/icons/icon_nav_players_avatar_image2.png',
} as const

function saveStartBlocker(error: unknown): 'new' | 'saves' | null {
  if (!(error instanceof ApiError)) return null
  if (error.code === 'save_required') return 'new'
  if (error.code === 'active_save_required' || error.code === 'active_save_missing') return 'saves'
  return null
}

export function OverviewPage({ instanceState, onNavigate, dashboardData }: StardewPageProps) {
  const [actionBusy, setActionBusy] = useState(false)
  const [actionError, setActionError] = useState<string | null>(null)
  const [saveRequiredDetected, setSaveRequiredDetected] = useState(false)
  const [confirmAction, setConfirmAction] = useState<'stop' | 'restart' | null>(null)
  const [pendingStartupAction, setPendingStartupAction] = useState<'start' | 'restart' | null>(null)
  const [pendingStopAction, setPendingStopAction] = useState(false)

  const state = instanceState?.state ?? null

  const activeSave = dashboardData.saves?.activeSaveName ?? null
  const saveCount = dashboardData.saves?.saves.length ?? 0
  const noSavesDetected = Boolean(dashboardData.saves && dashboardData.saves.saves.length === 0)
  const showSaveRequiredPrompt =
    (state === 'save_required' || saveRequiredDetected || noSavesDetected) &&
    state !== 'running' &&
    state !== 'starting'
  const visibleMods = dashboardData.mods?.mods.filter((m) => !modIsSystemRuntime(m)) ?? []
  const modCount = visibleMods.length
  const enabledModCount = visibleMods.filter((m) => m.enabled).length
  const disabledModCount = modCount - enabledModCount
  const modRestartRequired = dashboardData.mods?.restartRequired ?? false
  const onlineCount = dashboardData.players?.onlineCount
  const maxPlayers = dashboardData.players?.maxPlayers
  const playerSummary =
    onlineCount != null
      ? maxPlayers != null
        ? `${onlineCount}/${maxPlayers}`
        : String(onlineCount)
      : state === 'running'
        ? '识别中'
        : '—'
  const onlinePlayers = dashboardData.players?.players.filter((player) => player.status === 'online') ?? []

  const healthChecks = dashboardData.health?.checks ?? []
  const healthStatus = dashboardData.health?.status
  const errorCount = healthChecks.filter((c) => c.status === 'error').length
  const warnCount = healthChecks.filter((c) => c.status === 'warning').length
  const okCount = healthChecks.filter((c) => c.status === 'ok').length

  const recentJobs = dashboardData.jobs.slice(0, 5)
  const activeJobCount = dashboardData.jobs.filter(
    (j) => j.status === 'running' || j.status === 'queued',
  ).length
  const hasFailedJob = dashboardData.jobs.some((j) => j.status === 'failed')

  useEffect(() => {
    if (state && state !== 'save_required') {
      setSaveRequiredDetected(false)
    }
  }, [state])

  useEffect(() => {
    // Startup is complete once the server is running (invite code is optional/background).
    if (state === 'running' || dashboardData.inviteCode) {
      setPendingStartupAction(null)
    }
  }, [state, dashboardData.inviteCode])

  useEffect(() => {
    if (state === 'stopped' || state === 'ready_to_start' || state === 'game_installed' || state === 'save_required' || state === 'error') {
      setPendingStopAction(false)
    }
  }, [state])

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
    } catch (e) {
      setActionError(errorMessage(e))
      setPendingStartupAction(null)
    } finally {
      setActionBusy(false)
    }
  }

  function renderLifecycleButtons() {
    if (!state) return null
    // "Starting" ends when the server is running, not when an invite code arrives
    // (invite codes are optional/background and may never appear).
    const waitingForInvite =
      state === 'starting' ||
      Boolean(pendingStartupAction)
    const waitingForStop = state === 'stopping' || pendingStopAction

    if (state === 'save_required') {
      return (
        <button className="sd-btn-start" disabled>
          <img src="/assets/stardew/ui/icons/icon_button_play.png" alt="" className="sd-btn-img" />
          启动
        </button>
      )
    }

    if (waitingForInvite) {
      return (
        <button className="sd-btn-start sd-btn-loading" disabled>
          <span className="sd-btn-spinner" aria-hidden="true" />
          启动中…
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

    if (state === 'ready_to_start' || state === 'stopped' || state === 'game_installed') {
      return (
        <button className="sd-btn-start" onClick={() => void handleStart()} disabled={actionBusy}>
          <img src="/assets/stardew/ui/icons/icon_button_play.png" alt="" className="sd-btn-img" />
          {actionBusy ? '启动中…' : '启动'}
        </button>
      )
    }

    if (state === 'running') {
      return (
        <>
          <button className="sd-btn-stop" onClick={() => setConfirmAction('stop')} disabled={actionBusy}>
            <img src="/assets/stardew/ui/icons/icon_button_stop.png" alt="" className="sd-btn-img" />
            停止
          </button>
          <button
            className="sd-btn-restart"
            onClick={() => setConfirmAction('restart')}
            disabled={actionBusy}
          >
            <img src="/assets/stardew/ui/icons/icon_button_restart.png" alt="" className="sd-btn-img" />
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
    <div className="sd-ov-wrap">
      {/* 顶部农场横幅：场景图 + 底部信息条 */}
      <div className="sd-ov-banner">
        <div className="sd-ov-banner-scene">
          <div className="sd-ov-banner-bg" />
          <div className="sd-ov-banner-overlay" />
        </div>
        <div className="sd-ov-banner-statbar">
          <div className="sd-bstat">
            <img src="/assets/stardew/ui/icons/icon_top_summary_save.png" alt="" />
            <div className="sd-bstat-tx">
              <span className="sd-bstat-l">存档</span>
              <span className="sd-bstat-v">{saveCount}</span>
            </div>
          </div>
          <div className="sd-bstat">
            <img src="/assets/stardew/ui/icons/icon_top_summary_players.png" alt="" />
            <div className="sd-bstat-tx">
              <span className="sd-bstat-l">玩家</span>
              <span className="sd-bstat-v">{playerSummary}</span>
            </div>
          </div>
          <div className="sd-bstat">
            <img src="/assets/stardew/ui/icons/icon_top_summary_version.png" alt="" />
            <div className="sd-bstat-tx">
              <span className="sd-bstat-l">版本</span>
              <span className="sd-bstat-v">{dashboardData.versionInfo?.version ?? '—'}</span>
            </div>
          </div>
          <div className="sd-bstat sd-bstat--latest">
            <span className="sd-ov-latest">最新</span>
          </div>
        </div>
      </div>

      {/* 服务器控制 */}
      <div className="sd-ov-section">
        <div className="sd-ov-title">
          <img src={OVERVIEW_ICONS.server} alt="" />
          服务器控制
        </div>
        <div className="sd-ctrl-row">
          <div className="sd-lifecycle-actions">
            <div className="sd-lifecycle-btns">
              {renderLifecycleButtons()}
            </div>
            <div className="sd-lifecycle-status">
              状态
              <span
                className={
                  state === 'running' || state === 'starting'
                    ? 'sd-dot sd-dot-green'
                    : state === 'stopped' || state === 'error'
                      ? 'sd-dot sd-dot-red'
                      : 'sd-dot sd-dot-gray'
                }
                aria-hidden="true"
              />
              <span className={`sd-lifecycle-status-val sd-lifecycle-status-val-${state ?? 'unknown'}`}>
                {state ? stateLabel(state) : '未知'}
              </span>
            </div>
            {showSaveRequiredPrompt ? (
              <div className="sd-start-save-required">
                <span>当前没有存档，请点击此按钮去创建/上传存档。</span>
                <button className="sd-btn-green" onClick={() => onNavigate('saves')} disabled={actionBusy}>
                  创建/上传存档
                </button>
              </div>
            ) : null}
          </div>
          <InviteCodeCard
            instanceState={instanceState}
            dashboardData={dashboardData}
            className="sd-overview-invite-card"
            onNavigate={onNavigate}
          />
        </div>
        {actionError ? <div className="sd-ov-error">{actionError}</div> : null}
      </div>

      <div className="sd-metric-grid sd-ov-metric-strip" aria-label="服务器摘要">
        <div className={`sd-mc${dashboardData.savesError ? ' sd-mc--error' : !activeSave ? ' sd-mc--warn' : ''}`}>
          <div className="sd-mc-name">
            <img src={OVERVIEW_ICONS.saves} alt="" />
            存档
          </div>
          <div className="sd-mc-val">{saveCount}</div>
          <div className="sd-mc-unit">个存档</div>
          <div className="sd-mc-sub">
            {activeSave
              ? `当前: ${activeSave}`
              : dashboardData.savesError
                ? '读取失败'
                : '暂无激活存档'}
          </div>
          <span className={`sd-mc-pill${activeSave ? ' sd-mc-pill--ok' : ' sd-mc-pill--warn'}`}>
            {activeSave ? '正常' : '待选择'}
          </span>
        </div>

        <div className={`sd-mc${dashboardData.modsError ? ' sd-mc--error' : modRestartRequired ? ' sd-mc--warn' : ''}`}>
          <div className="sd-mc-name">
            <img src={OVERVIEW_ICONS.mods} alt="" />
            模组
          </div>
          <div className="sd-mc-val">{modCount}</div>
          <div className="sd-mc-unit">个模组</div>
          <div className="sd-mc-sub">
            {dashboardData.modsError ? '读取失败' : modRestartRequired ? '有模组变更待应用' : `已启用 ${enabledModCount} 个`}
          </div>
          <span className={`sd-mc-pill${dashboardData.modsError ? ' sd-mc-pill--error' : modRestartRequired ? ' sd-mc-pill--warn' : ' sd-mc-pill--ok'}`}>
            {dashboardData.modsError ? '异常' : modRestartRequired ? '待应用' : '健康'}
          </span>
        </div>

        <div className={`sd-mc${healthStatus === 'ok' ? ' sd-mc--ok' : healthStatus === 'warning' ? ' sd-mc--warn' : healthStatus === 'error' ? ' sd-mc--error' : ''}`}>
          <div className="sd-mc-name">
            <img src={OVERVIEW_ICONS.health} alt="" />
            系统健康
          </div>
          <div className="sd-mc-val" style={{ color: healthStatus === 'ok' ? '#4a9e30' : healthStatus === 'warning' ? '#d08010' : healthStatus === 'error' ? '#c02020' : '#2c1a0a' }}>
            {healthStatus === 'ok' ? '100%' : healthStatus === 'warning' ? `${warnCount}警告` : healthStatus === 'error' ? `${errorCount}错误` : '—'}
          </div>
          <div className="sd-mc-unit">健康评分</div>
          <div className="sd-mc-sub">
            {healthStatus === 'ok'
              ? `${okCount}项全部通过`
              : healthStatus === 'warning'
                ? `${errorCount === 0 ? '' : `${errorCount}错误 · `}${okCount}正常`
                : healthStatus === 'error'
                  ? `${warnCount}警告 · ${okCount}正常`
                  : dashboardData.healthError
                    ? '健康检查失败'
                    : '进入诊断页后检查'}
          </div>
          <span className={`sd-mc-pill${healthStatus === 'error' ? ' sd-mc-pill--error' : healthStatus === 'warning' ? ' sd-mc-pill--warn' : healthStatus === 'ok' ? ' sd-mc-pill--ok' : ''}`}>
            {healthStatus === 'error' ? '异常' : healthStatus === 'warning' ? '警告' : healthStatus === 'ok' ? '优秀' : '未检查'}
          </span>
        </div>

        <div className={`sd-mc${hasFailedJob ? ' sd-mc--error' : ''}`}>
          <div className="sd-mc-name">
            <img src={OVERVIEW_ICONS.tasks} alt="" />
            运行任务
          </div>
          <div className="sd-mc-val" style={{ color: hasFailedJob ? '#c02020' : '#2c1a0a' }}>
            {activeJobCount}
          </div>
          <div className="sd-mc-unit">个任务</div>
          <div className="sd-mc-sub">
            {hasFailedJob ? '最近有失败任务' : activeJobCount > 0 ? '进行中' : '无异常'}
          </div>
          <span className={`sd-mc-pill${hasFailedJob ? ' sd-mc-pill--error' : ' sd-mc-pill--ok'}`}>
            {hasFailedJob ? '异常' : '正常'}
          </span>
        </div>
      </div>

      <div className="sd-ov-summary-grid">
        <section className="sd-ov-card">
          <div className="sd-player-hd">
            <img src={OVERVIEW_ICONS.players} alt="" />
            在线玩家
            {onlineCount != null && maxPlayers != null ? <span>{onlineCount}/{maxPlayers}</span> : null}
          </div>
          <div className="sd-ov-player-body">
            {onlinePlayers.length > 0 ? (
              <div className="sd-ov-player-list">
                {onlinePlayers.slice(0, 4).map((player) => (
                  <div className="sd-ov-player-row" key={player.uniqueMultiplayerId || player.name}>
                    <span className="sd-ov-player-avatar" aria-hidden="true">{player.name.slice(0, 1)}</span>
                    <span className="sd-ov-player-main">
                      <span className="sd-ov-player-name">{player.name}</span>
                      <span className="sd-ov-player-meta">
                        {player.locationDisplayName || player.locationName || player.location || (player.isHost ? '农场主' : '在线')}
                      </span>
                    </span>
                    <span className="sd-dot sd-dot-green" aria-hidden="true" />
                  </div>
                ))}
              </div>
            ) : dashboardData.playersError ? (
              <span>在线玩家读取失败。</span>
            ) : onlineCount === 0 ? (
              <span>暂无在线玩家。</span>
            ) : state === 'running' ? (
              <span>已接入在线人数，玩家姓名等待控制文件或 Junimo info 输出。</span>
            ) : (
              <span>服务器运行后显示在线玩家。</span>
            )}
          </div>
          <button className="sd-all-logs-btn" onClick={() => onNavigate('players')}>
            查看全部玩家 →
          </button>
        </section>

        <section className="sd-ov-card">
          <div className="sd-ev-title">
            <img src={OVERVIEW_ICONS.tasks} alt="" />
            近期事件
          </div>
          <div className="sd-ev-list">
            {recentJobs.length > 0 ? (
              recentJobs.map((j) => (
                <div key={j.id} className="sd-ev-item">
                  <div className="sd-ev-time">
                    {j.createdAt ? formatDate(j.createdAt).slice(5) : '—'}
                  </div>
                  <div className="sd-ev-text">
                    <span
                      className={
                        j.status === 'succeeded'
                          ? 'sd-dot sd-dot-green'
                          : j.status === 'failed'
                            ? 'sd-dot sd-dot-red'
                            : j.status === 'running'
                              ? 'sd-dot sd-dot-green sd-dot-pulse'
                              : j.status === 'queued'
                                ? 'sd-dot sd-dot-yellow'
                                : 'sd-dot sd-dot-gray'
                      }
                      aria-hidden="true"
                      style={{ marginRight: 4 }}
                    />
                    <span title={jobDisplayName(j)}>{jobDisplayName(j)}</span>
                    {j.status === 'failed' && j.errorMessage ? (
                      <span style={{ color: '#c02020', fontSize: 9, marginLeft: 4 }}>
                        {j.errorMessage}
                      </span>
                    ) : null}
                  </div>
                </div>
              ))
            ) : dashboardData.loading ? (
              <div className="sd-ov-empty">读取中…</div>
            ) : (
              <div className="sd-ov-empty">暂无事件记录</div>
            )}
          </div>
          <button className="sd-all-logs-btn" onClick={() => onNavigate('jobs')}>
            查看全部事件 →
          </button>
        </section>

        <section className="sd-ov-card">
          <div className="sd-pack-title">
            <img src={OVERVIEW_ICONS.mods} alt="" />
            模组状态
            <button className="sd-pack-more" onClick={() => onNavigate('mods')}>查看更多 →</button>
          </div>
          <div className="sd-pack-section">
            <div className="sd-pack-row">
              <span className="sd-pack-name">
                <span className="sd-dot sd-dot-green" aria-hidden="true" />已启用
              </span>
              <span className="sd-pack-count">{dashboardData.mods ? enabledModCount : '—'}</span>
            </div>
            <div className="sd-pack-row">
              <span className="sd-pack-name">
                <span className="sd-dot sd-dot-yellow" aria-hidden="true" />已禁用
              </span>
              <span className="sd-pack-count">{dashboardData.mods ? disabledModCount : '—'}</span>
            </div>
            <div className="sd-pack-row">
              <span className="sd-pack-name">
                <span className="sd-dot sd-dot-yellow" aria-hidden="true" />可更新
              </span>
              <span className="sd-pack-count">{dashboardData.mods ? 0 : '—'}</span>
            </div>
            <div className="sd-pack-row">
              <span className="sd-pack-name">
                <span className="sd-dot sd-dot-red" aria-hidden="true" />异常
              </span>
              <span className="sd-pack-count">{dashboardData.modsError ? 1 : 0}</span>
            </div>
          </div>
          <button className="sd-pack-manage sd-btn-tan" onClick={() => onNavigate('mods')}>
            管理模组
          </button>
        </section>
      </div>

      {/* 危险操作确认弹框 */}
      {confirmAction ? (
        <div className="sd-confirm-overlay">
          <div className="sd-confirm-dialog">
            <h3>{confirmAction === 'stop' ? '确认停止服务器' : '确认重启服务器'}</h3>
            <p>
              {confirmAction === 'stop'
                ? '停止服务器将断开所有玩家连接，邀请码将失效。'
                : '重启服务器将短暂断开所有玩家连接，请确认操作。'}
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
    </div>
  )
}
