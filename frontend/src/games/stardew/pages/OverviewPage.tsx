import { useEffect, useState } from 'react'
import { ApiError, startInstance, stopInstance, restartInstance } from '../../../api'
import { errorMessage, stateLabel, formatDate } from '../../../core/helpers'
import type { StardewPageProps } from '../stardew-routes'

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
  const [copied, setCopied] = useState(false)
  const [copyError, setCopyError] = useState(false)

  const state = instanceState?.state ?? null
  const stateLabelText = state
    ? stateLabel(state)
    : dashboardData.loading
      ? '读取中…'
      : '未知'

  const activeSave = dashboardData.saves?.activeSaveName ?? null
  const saveCount = dashboardData.saves?.saves.length ?? 0
  const noSavesDetected = Boolean(dashboardData.saves && dashboardData.saves.saves.length === 0)
  const showSaveRequiredPrompt =
    (state === 'save_required' || saveRequiredDetected || noSavesDetected) &&
    state !== 'running' &&
    state !== 'starting'
  const modCount = dashboardData.mods?.mods.length ?? 0
  const modRestartRequired = dashboardData.mods?.restartRequired ?? false

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

  async function handleStart() {
    setActionBusy(true)
    setActionError(null)
    try {
      await startInstance()
      setSaveRequiredDetected(false)
      dashboardData.refreshInstanceState()
      dashboardData.refreshJobs()
      dashboardData.refreshInviteCode()
    } catch (e) {
      const saveBlocker = saveStartBlocker(e)
      if (saveBlocker) {
        setSaveRequiredDetected(saveBlocker === 'new')
        setActionError(saveBlocker === 'new' ? null : errorMessage(e))
        dashboardData.refreshInstanceState()
        dashboardData.refreshSaves()
        return
      }
      setActionError(errorMessage(e))
    } finally {
      setActionBusy(false)
    }
  }

  async function handleStop() {
    setActionBusy(true)
    setActionError(null)
    try {
      await stopInstance()
      dashboardData.refreshInstanceState()
      dashboardData.refreshInviteCode()
    } catch (e) {
      setActionError(errorMessage(e))
    } finally {
      setActionBusy(false)
    }
  }

  async function handleRestart() {
    setActionBusy(true)
    setActionError(null)
    try {
      await restartInstance()
      dashboardData.refreshInstanceState()
    } catch (e) {
      setActionError(errorMessage(e))
    } finally {
      setActionBusy(false)
    }
  }

  function handleCopy() {
    if (!dashboardData.inviteCode) return
    setCopyError(false)
    navigator.clipboard.writeText(dashboardData.inviteCode).then(
      () => {
        setCopied(true)
        setTimeout(() => setCopied(false), 2000)
      },
      () => {
        setCopyError(true)
        setTimeout(() => setCopyError(false), 3000)
      },
    )
  }

  function renderLifecycleButtons() {
    if (!state) return null

    if (state === 'game_installed') {
      return (
        <button className="sd-btn-green" onClick={() => onNavigate('install')} disabled={actionBusy}>
          前往安装配置
        </button>
      )
    }

    if (state === 'save_required') {
      return (
        <button className="sd-btn-start" disabled>
          <img
            src="/assets/stardew/ui/icons/icon_button_play.png"
            alt=""
            className="sd-btn-img"
            style={{ width: 12, height: 13 }}
          />
          启动
        </button>
      )
    }

    if (state === 'ready_to_start' || state === 'stopped') {
      return (
        <button className="sd-btn-start" onClick={() => void handleStart()} disabled={actionBusy}>
          <img
            src="/assets/stardew/ui/icons/icon_button_play.png"
            alt=""
            className="sd-btn-img"
            style={{ width: 12, height: 13 }}
          />
          {actionBusy ? '启动中…' : '启动'}
        </button>
      )
    }

    if (state === 'starting') {
      return (
        <button className="sd-btn-start" disabled>
          <span className="sd-state-badge-dot" aria-hidden="true" />
          启动中…
        </button>
      )
    }

    if (state === 'running') {
      return (
        <>
          <button className="sd-btn-stop" onClick={() => setConfirmAction('stop')} disabled={actionBusy}>
            <img
              src="/assets/stardew/ui/icons/icon_button_stop.png"
              alt=""
              className="sd-btn-img"
              style={{ width: 11, height: 11 }}
            />
            停止
          </button>
          <button
            className="sd-btn-restart"
            onClick={() => setConfirmAction('restart')}
            disabled={actionBusy}
          >
            <img
              src="/assets/stardew/ui/icons/icon_button_restart.png"
              alt=""
              className="sd-btn-img"
              style={{ width: 12, height: 12 }}
            />
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

  const showCtrlDivider =
    state === 'ready_to_start' ||
    state === 'stopped' ||
    state === 'starting' ||
    state === 'running'

  return (
    <div className="sd-ov-wrap">
      {/* 顶部农场横幅 */}
      <div className="sd-ov-banner">
        <div className="sd-ov-banner-bg" />
        <div className="sd-ov-banner-overlay" />
        <div className="sd-ov-banner-stats">
          {activeSave ? (
            <div className="sd-bstat">
              <img src="/assets/stardew/ui/icons/icon_top_summary_save.png" alt="" />
              <span className="sd-bstat-l">存档：</span>
              <span className="sd-bstat-v">{activeSave}</span>
            </div>
          ) : null}
          <div className="sd-bstat">
            <img src="/assets/stardew/ui/icons/icon_top_summary_players.png" alt="" />
            <span className="sd-bstat-l">玩家：</span>
            <span className="sd-bstat-v">—</span>
          </div>
          {dashboardData.versionInfo ? (
            <div className="sd-bstat">
              <img src="/assets/stardew/ui/icons/icon_top_summary_version.png" alt="" />
              <span className="sd-bstat-l">版本：</span>
              <span className="sd-bstat-v">{dashboardData.versionInfo.version}</span>
            </div>
          ) : null}
          {instanceState?.updatedAt ? (
            <div className="sd-bstat">
              <img src="/assets/stardew/ui/icons/icon_top_summary_time.png" alt="" />
              <span className="sd-bstat-l">更新：</span>
              <span className="sd-bstat-v">{formatDate(instanceState.updatedAt)}</span>
            </div>
          ) : null}
        </div>
        <div className="sd-ov-banner-right">
          <span className={`sd-state-badge sd-state-badge-${state ?? 'unknown'}`}>
            {(state === 'running' || state === 'starting') ? (
              <span className="sd-state-badge-dot" aria-hidden="true" />
            ) : null}
            {stateLabelText}
          </span>
        </div>
      </div>

      {/* 服务器控制 */}
      <div className="sd-ov-section">
        <div className="sd-ov-title">服务器控制</div>
        <div className="sd-ctrl-row">
          {renderLifecycleButtons()}
          {showSaveRequiredPrompt ? (
            <div className="sd-start-save-required">
              <span>当前没有存档，请点击此按钮去创建/上传存档。</span>
              <button className="sd-btn-green" onClick={() => onNavigate('saves')} disabled={actionBusy}>
                创建/上传存档
              </button>
            </div>
          ) : null}
          {showCtrlDivider && dashboardData.inviteCode ? (
            <div className="sd-ctrl-div">│</div>
          ) : null}
          <div className="sd-invite-wrap">
            {dashboardData.inviteCode ? (
              <>
                <div className="sd-invite-box">
                  <div className="sd-invite-code">{dashboardData.inviteCode}</div>
                  <button className="sd-btn-copy" onClick={handleCopy}>
                    {copied ? '✓' : '复制'}
                  </button>
                </div>
                {copyError ? (
                  <span className="sd-bstat-l" style={{ color: '#c02020', fontSize: 9.5, marginLeft: 4 }}>
                    复制失败，请手动选取
                  </span>
                ) : null}
              </>
            ) : dashboardData.loading ? (
              <span className="sd-bstat-l">读取邀请码中…</span>
            ) : dashboardData.inviteCodeError ? (
              <span className="sd-bstat-l" style={{ fontStyle: 'italic' }}>
                服务器未运行，邀请码不可用
              </span>
            ) : (
              <span className="sd-bstat-l" style={{ fontStyle: 'italic' }}>暂无邀请码</span>
            )}
          </div>
        </div>
        {actionError ? <div className="sd-ov-error">{actionError}</div> : null}
      </div>

      {/* 主体双栏 */}
      <div className="sd-ov-body">
        {/* 左栏：指标 + 玩家 */}
        <div className="sd-ov-left">
          <div className="sd-ov-title" style={{ padding: '5px 8px 0' }}>服务器状态</div>
          <div className="sd-metric-grid">
            {/* 存档 */}
            <div className={`sd-mc${dashboardData.savesError ? ' sd-mc--error' : !activeSave ? ' sd-mc--warn' : ''}`}>
              <div className="sd-mc-name">存档</div>
              <div className="sd-mc-val">{saveCount}</div>
              <div className="sd-mc-sub">
                {activeSave
                  ? activeSave
                  : dashboardData.savesError
                    ? '读取失败'
                    : '暂无激活存档'}
              </div>
            </div>

            {/* 模组 */}
            <div className={`sd-mc${dashboardData.modsError ? ' sd-mc--error' : modRestartRequired ? ' sd-mc--warn' : ''}`}>
              <div className="sd-mc-name">
                模组
                {modRestartRequired ? (
                  <span
                    style={{ color: '#d08010', fontWeight: 700, marginLeft: 4, fontSize: 9 }}
                  >
                    ⚠需重启
                  </span>
                ) : null}
              </div>
              <div className="sd-mc-val">{modCount}</div>
              <div className="sd-mc-sub">
                {dashboardData.modsError ? '读取失败' : modRestartRequired ? '有模组变更待应用' : '状态正常'}
              </div>
            </div>

            {/* 系统健康 */}
            <div className={`sd-mc${healthStatus === 'ok' ? ' sd-mc--ok' : healthStatus === 'warning' ? ' sd-mc--warn' : healthStatus === 'error' ? ' sd-mc--error' : ''}`}>
              <div className="sd-mc-name">系统健康</div>
              <div className="sd-mc-val" style={{ color: healthStatus === 'ok' ? '#4a9e30' : healthStatus === 'warning' ? '#d08010' : healthStatus === 'error' ? '#c02020' : '#2c1a0a' }}>
                {healthStatus === 'ok' ? '✓ 正常' : healthStatus === 'warning' ? `${warnCount}警告` : healthStatus === 'error' ? `${errorCount}错误` : '—'}
              </div>
              <div className="sd-mc-sub">
                {healthStatus === 'ok'
                  ? `${okCount}项全部通过`
                  : healthStatus === 'warning'
                    ? `${errorCount === 0 ? '' : `${errorCount}错误 · `}${okCount}正常`
                    : healthStatus === 'error'
                      ? `${warnCount}警告 · ${okCount}正常`
                      : dashboardData.healthError
                        ? '健康检查失败'
                        : '检查中…'}
              </div>
            </div>

            {/* 运行任务 */}
            <div className={`sd-mc${hasFailedJob ? ' sd-mc--error' : ''}`}>
              <div className="sd-mc-name">运行任务</div>
              <div
                className="sd-mc-val"
                style={{ color: hasFailedJob ? '#c02020' : '#2c1a0a' }}
              >
                {activeJobCount}
              </div>
              <div className="sd-mc-sub">
                {hasFailedJob ? '最近有失败任务' : activeJobCount > 0 ? '进行中' : '无活跃任务'}
              </div>
            </div>
          </div>

          <div className="sd-player-hd">
            在线玩家
          </div>
          <div
            style={{
              padding: '4px 8px',
              color: '#8a7060',
              fontSize: 10,
              fontStyle: 'italic',
              flex: 1,
            }}
          >
            玩家列表 API 待接入，当前无法获取在线人数。
            <button
              className="sd-btn-tan"
              style={{ marginLeft: 8, verticalAlign: 'middle' }}
              onClick={() => onNavigate('players')}
            >
              玩家管理
            </button>
          </div>
        </div>

        {/* 右栏：事件 + 模组摘要 */}
        <div className="sd-ov-right">
          <div className="sd-ev-title">任务与事件</div>
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
                    {j.type}
                    {j.status === 'failed' && j.errorMessage ? (
                      <span style={{ color: '#c02020', fontSize: 9, marginLeft: 4 }}>
                        {j.errorMessage}
                      </span>
                    ) : null}
                  </div>
                </div>
              ))
            ) : dashboardData.loading ? (
              <div style={{ padding: '4px 8px', color: '#8a7060', fontSize: 10, fontStyle: 'italic' }}>
                读取中…
              </div>
            ) : (
              <div style={{ padding: '4px 8px', color: '#8a7060', fontSize: 10, fontStyle: 'italic' }}>
                暂无事件记录
              </div>
            )}
          </div>
          <button className="sd-all-logs-btn" onClick={() => onNavigate('jobs')}>
            查看全部任务 →
          </button>

          <div className="sd-pack-section">
            <div className="sd-pack-title">模组状态</div>
            <div className="sd-pack-row">
              <span className="sd-pack-name">已安装模组</span>
              <span style={{ color: '#4a9e30', fontSize: 9.5 }}>
                {dashboardData.mods ? `${modCount} 个` : '—'}
              </span>
            </div>
            {modRestartRequired ? (
              <div className="sd-pack-row">
                <span className="sd-pack-name">模组变更</span>
                <span style={{ color: '#d08010', fontWeight: 700, fontSize: 9.5 }}>需要重启</span>
              </div>
            ) : null}
            <div className="sd-pack-row" style={{ borderBottom: 'none', paddingTop: 5 }}>
              <button className="sd-btn-tan" onClick={() => onNavigate('mods')}>
                管理模组
              </button>
              <button
                className="sd-btn-tan"
                style={{ marginLeft: 4 }}
                onClick={() => onNavigate('diagnostics')}
              >
                诊断
              </button>
            </div>
          </div>
        </div>
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
                className={confirmAction === 'stop' ? 'sd-btn-stop' : 'sd-btn-restart'}
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
