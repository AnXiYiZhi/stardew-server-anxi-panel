import { useEffect, useState } from 'react'
import {
  approvePlayerAuth,
  getInstancePasswordStatus,
  restartInstance,
  startInstance,
  stopInstance,
} from '../../../api'
import type { InstancePasswordStatus, SaveInfo } from '../../../types'
import { errorMessage, stateLabel } from '../../../core/helpers'
import type { StardewDashboardData, StardewPageProps } from '../stardew-routes'
import { panelUpdateSurface, } from '../panel-update-machine'
import './MobileHomePage.css'

type MobileHomePageProps = Pick<StardewPageProps, 'user' | 'instanceState' | 'dashboardData'>

type ApproveTarget = { uniqueMultiplayerId: string; name: string }

type ConfirmState =
  | { kind: 'stop' }
  | { kind: 'restart' }
  | { kind: 'approve'; target: ApproveTarget }

// navigator.clipboard 需要安全上下文（HTTPS 或 localhost）。面板常见通过局域网/公网 IP
// 走 HTTP 访问，此时 navigator.clipboard 为 undefined，直接调用会同步抛错；回退到
// execCommand 方案保证复制按钮在这些场景下仍可用。与 InviteCodeCard.tsx 的实现保持一致。
async function copyText(text: string): Promise<boolean> {
  if (typeof navigator !== 'undefined' && navigator.clipboard && window.isSecureContext) {
    try {
      await navigator.clipboard.writeText(text)
      return true
    } catch {
      // fall through to legacy fallback below
    }
  }
  try {
    const textarea = document.createElement('textarea')
    textarea.value = text
    textarea.style.position = 'fixed'
    textarea.style.opacity = '0'
    document.body.appendChild(textarea)
    textarea.focus()
    textarea.select()
    const ok = document.execCommand('copy')
    document.body.removeChild(textarea)
    return ok
  } catch {
    return false
  }
}

function serverStatusText(state: string | null, loading: boolean): string {
  if (!state) return loading ? '读取中' : '未知'
  if (state === 'stopping') return '停止中'
  return stateLabel(state)
}

function serverStatusDotClass(state: string | null, loading: boolean): string {
  if (state === 'running') return 'sd-dot sd-dot-green sd-dot-pulse'
  if (state === 'starting' || state === 'stopping' || (loading && !state)) return 'sd-dot sd-dot-yellow sd-dot-pulse'
  if (state === 'stopped' || state === 'error') return 'sd-dot sd-dot-red'
  return 'sd-dot sd-dot-gray'
}

function inviteInfo(dashboardData: StardewDashboardData, instanceState: MobileHomePageProps['instanceState']): { text: string; copyable: boolean } {
  if (dashboardData.inviteCode) return { text: dashboardData.inviteCode, copyable: true }
  const state = instanceState?.state ?? null
  const canRefreshInvite = state === 'running' || state === 'starting'
  const needAuthLogin = instanceState?.steamAuthLoggedIn !== true
  if (needAuthLogin) return { text: '需登录 Steam 授权', copyable: false }
  if (canRefreshInvite) return { text: dashboardData.inviteCodeError ? '获取失败' : '获取中…', copyable: false }
  return { text: '服务器未运行', copyable: false }
}

function hostInfo(dashboardData: StardewDashboardData): { text: string; copyable: boolean } {
  if (dashboardData.publicIP?.ip) return { text: dashboardData.publicIP.ip, copyable: true }
  if (dashboardData.publicIPRefreshing) return { text: '检测中…', copyable: false }
  if (dashboardData.publicIPError) return { text: '检测失败', copyable: false }
  return { text: '未检测', copyable: false }
}

// 和 ServerSummaryCard.tsx 的 SEASON_ZH/saveDate 同构，展示游戏内日期而不是面板/SMAPI 版本号。
const SEASON_ZH: Record<string, string> = {
  spring: '春',
  summer: '夏',
  fall: '秋',
  winter: '冬',
}

function saveDate(save: SaveInfo): string {
  if (!save.gameYear) return '—'
  const season = SEASON_ZH[save.gameSeason?.toLowerCase() ?? ''] ?? save.gameSeason ?? '?'
  return `第 ${save.gameYear} 年${season}季${save.gameDay ?? '?'} 日`
}

export function MobileHomePage({ user, instanceState, dashboardData }: MobileHomePageProps) {
  const isAdmin = user.role === 'admin'
  const state = instanceState?.state ?? null
  const isRunning = state === 'running'

  const [actionBusy, setActionBusy] = useState(false)
  const [actionError, setActionError] = useState<string | null>(null)
  const [pendingStartupAction, setPendingStartupAction] = useState<'start' | 'restart' | null>(null)
  const [pendingStopAction, setPendingStopAction] = useState(false)
  const [confirm, setConfirm] = useState<ConfirmState | null>(null)

  const [inviteCopied, setInviteCopied] = useState(false)
  const [hostCopied, setHostCopied] = useState(false)
  const [copyFailed, setCopyFailed] = useState(false)

  const [passwordStatus, setPasswordStatus] = useState<InstancePasswordStatus | null>(null)
  const [approveBusy, setApproveBusy] = useState(false)
  const [approveError, setApproveError] = useState<string | null>(null)
  const [approveMessage, setApproveMessage] = useState<string | null>(null)

  const hasActiveLifecycleJob = dashboardData.jobs.some(
    (j) => j.type === 'stardew_lifecycle' && (j.status === 'running' || j.status === 'queued'),
  )
  const activeLifecycleIsStopping = hasActiveLifecycleJob && instanceState?.driverPhase === 'stopping'
  const hostOnline = (dashboardData.players?.players ?? []).some(
    (player) => player.isHost && player.status === 'online',
  )
  const waitingForStop = state === 'stopping' || pendingStopAction || activeLifecycleIsStopping
  const waitingForStartup = !waitingForStop && !hostOnline && (
    state === 'starting' ||
    Boolean(pendingStartupAction) ||
    (hasActiveLifecycleJob && !activeLifecycleIsStopping && state !== 'running')
  )
  const canStart = state === 'ready_to_start' || state === 'stopped' || state === 'game_installed'

  useEffect(() => {
    if (!hasActiveLifecycleJob && state === 'running') setPendingStartupAction(null)
  }, [hasActiveLifecycleJob, state])

  useEffect(() => {
    if (state === 'stopped' || state === 'ready_to_start' || state === 'game_installed' || state === 'save_required' || state === 'error') {
      setPendingStopAction(false)
    }
  }, [state])

  useEffect(() => {
    if (!isRunning) return
    let cancelled = false
    getInstancePasswordStatus()
      .then((res) => {
        if (!cancelled) setPasswordStatus(res)
      })
      .catch(() => {
        if (!cancelled) setPasswordStatus(null)
      })
    return () => {
      cancelled = true
    }
  }, [isRunning])

  async function handleStart() {
    setActionBusy(true)
    setPendingStartupAction('start')
    setPendingStopAction(false)
    setActionError(null)
    try {
      await startInstance()
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

  async function handleConfirmApprove(target: ApproveTarget) {
    setApproveBusy(true)
    setApproveError(null)
    setApproveMessage(null)
    try {
      const res = await approvePlayerAuth(target.uniqueMultiplayerId)
      setApproveMessage(res.output?.trim() || `已提交批准 ${target.name} 认证的指令。`)
      await dashboardData.refreshPlayers()
    } catch (e) {
      setApproveError(errorMessage(e))
    } finally {
      setApproveBusy(false)
      setConfirm(null)
    }
  }

  function handleCopyInvite() {
    const code = dashboardData.inviteCode
    if (!code) return
    setCopyFailed(false)
    void copyText(code).then((ok) => {
      if (ok) {
        setInviteCopied(true)
        setTimeout(() => setInviteCopied(false), 2000)
      } else {
        setCopyFailed(true)
        setTimeout(() => setCopyFailed(false), 3000)
      }
    })
  }

  function handleCopyHost() {
    const host = dashboardData.publicIP?.ip
    if (!host) return
    setCopyFailed(false)
    void copyText(host).then((ok) => {
      if (ok) {
        setHostCopied(true)
        setTimeout(() => setHostCopied(false), 2000)
      } else {
        setCopyFailed(true)
        setTimeout(() => setCopyFailed(false), 3000)
      }
    })
  }

  const activeSaveNameRaw = dashboardData.saves?.activeSaveName ?? null
  const activeSave = dashboardData.saves?.saves.find(
    (save) => save.isActive || save.name === activeSaveNameRaw,
  ) ?? null
  const activeSaveName = activeSaveNameRaw?.trim() || '暂无存档'
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
  const gameDateText = activeSave?.gameYear ? saveDate(activeSave) : '—'
  const statusText = serverStatusText(state, dashboardData.loading)
  const statusDotClass = serverStatusDotClass(state, dashboardData.loading)

  const invite = inviteInfo(dashboardData, instanceState)
  const host = hostInfo(dashboardData)

  const pendingAuthPlayers = (dashboardData.players?.players ?? []).filter(
    (player) => player.status === 'online' && player.isAuthenticated === false,
  )
  const updateSurface = panelUpdateSurface(dashboardData.updateStatus, dashboardData.updateApply, dashboardData.versionInfo)
  const currentPanelVersion = updateSurface.currentVersion || '—'

  // 和桌面 OverviewPage.tsx 的 renderLifecycleButtons() 同一套状态分支：未运行时只显示
  // 启动按钮，运行中才切换成停止+重启；不像早期版本那样把三个按钮同时常驻靠 disabled 区分。
  function renderLifecycleButtons() {
    if (!state) return null

    if (state === 'save_required') {
      return (
        <button type="button" className="sd-btn-start sd-mhome-lifecycle-btn" disabled title="请先创建或上传存档后再启动">
          <img src="/assets/stardew/ui/icons/icon_button_play.png" alt="" className="sd-btn-img" />
          启动
        </button>
      )
    }

    if (waitingForStop) {
      return (
        <button type="button" className="sd-btn-stop sd-mhome-lifecycle-btn sd-btn-loading" disabled>
          <span className="sd-btn-spinner" aria-hidden="true" />
          停止中…
        </button>
      )
    }

    if (waitingForStartup) {
      return (
        <button type="button" className="sd-btn-start sd-mhome-lifecycle-btn sd-btn-loading" disabled>
          <span className="sd-btn-spinner" aria-hidden="true" />
          启动中…
        </button>
      )
    }

    if (canStart) {
      return (
        <button
          type="button"
          className="sd-btn-start sd-mhome-lifecycle-btn"
          onClick={() => void handleStart()}
          disabled={actionBusy || !isAdmin}
          title={isAdmin ? undefined : '仅管理员可启动服务器'}
        >
          <img src="/assets/stardew/ui/icons/icon_button_play.png" alt="" className="sd-btn-img" />
          启动
        </button>
      )
    }

    if (state === 'running') {
      return (
        <>
          <button
            type="button"
            className="sd-btn-stop sd-mhome-lifecycle-btn"
            onClick={() => setConfirm({ kind: 'stop' })}
            disabled={actionBusy || !isAdmin}
            title={isAdmin ? undefined : '仅管理员可停止服务器'}
          >
            <img src="/assets/stardew/ui/icons/icon_button_stop.png" alt="" className="sd-btn-img" />
            停止
          </button>
          <button
            type="button"
            className="sd-btn-restart sd-mhome-lifecycle-btn"
            onClick={() => setConfirm({ kind: 'restart' })}
            disabled={actionBusy || !isAdmin}
            title={isAdmin ? undefined : '仅管理员可重启服务器'}
          >
            <img src="/assets/stardew/ui/icons/icon_button_restart.png" alt="" className="sd-btn-img" />
            重启
          </button>
        </>
      )
    }

    if (state === 'error') {
      return <div className="sd-notice sd-notice--error sd-mhome-notice">服务器异常，请到电脑端查看诊断信息。</div>
    }

    return null
  }

  return (
    <div className="sd-mhome-wrap">
      <section className="sd-panel sd-mhome-card">
        <div className="sd-mhome-card-title">
          <img src="/assets/stardew/ui/icons/icon_top_summary_save.png" alt="" />
          服务器状态
        </div>
        <div className="sd-mhome-status-grid">
          <div className="sd-mhome-status-item">
            <span className="sd-mhome-status-label">存档</span>
            <span className="sd-mhome-status-value">{activeSaveName}</span>
          </div>
          <div className="sd-mhome-status-item">
            <span className="sd-mhome-status-label">状态</span>
            <span className="sd-mhome-status-value">
              <span className={statusDotClass} aria-hidden="true" />
              {statusText}
            </span>
          </div>
          <div className="sd-mhome-status-item">
            <span className="sd-mhome-status-label">在线玩家</span>
            <span className="sd-mhome-status-value">{playerSummary}</span>
          </div>
          <div className="sd-mhome-status-item">
            <span className="sd-mhome-status-label">游戏日期</span>
            <span className="sd-mhome-status-value">{gameDateText}</span>
          </div>
          <button type="button" className="sd-mhome-status-item sd-mhome-status-item--version" onClick={dashboardData.openUpdateDialog}>
            <span className="sd-mhome-status-label">面板版本</span>
            <span className="sd-mhome-status-value">{currentPanelVersion}</span>
          </button>
          <button
            type="button"
            className={`sd-mhome-status-item sd-mhome-status-item--version sd-mhome-status-item--${updateSurface.tone}`}
            onClick={dashboardData.openUpdateDialog}
          >
            <span className="sd-mhome-status-label">更新状态</span>
            <span className="sd-mhome-status-value">{updateSurface.overviewText}</span>
          </button>
        </div>
      </section>

      <section className="sd-panel sd-mhome-card">
        <div className="sd-mhome-card-title">
          <img src="/assets/stardew/ui/icons/icon_nav_server_rack_image2.png" alt="" />
          邀请信息
        </div>
        <div className="sd-mhome-invite-row">
          <span className="sd-mhome-invite-label">邀请码</span>
          <div className={`sd-mhome-invite-box${invite.copyable ? '' : ' sd-mhome-invite-box--muted'}`}>
            {invite.text}
          </div>
          <button
            type="button"
            className="sd-btn-tan sd-mhome-copy-btn"
            onClick={handleCopyInvite}
            disabled={!invite.copyable}
            title={invite.copyable ? '复制邀请码' : '暂无可复制的邀请码'}
          >
            {inviteCopied ? '已复制' : '复制'}
          </button>
        </div>
        <div className="sd-mhome-invite-row">
          <span className="sd-mhome-invite-label">局域网邀请</span>
          <div className={`sd-mhome-invite-box${host.copyable ? '' : ' sd-mhome-invite-box--muted'}`}>
            {host.text}
          </div>
          <button
            type="button"
            className="sd-btn-green sd-mhome-copy-btn"
            onClick={handleCopyHost}
            disabled={!host.copyable}
            title={host.copyable ? '复制当前面板访问地址' : '暂无可复制的地址'}
          >
            {hostCopied ? '已复制' : '复制'}
          </button>
        </div>
        {copyFailed ? (
          <div className="sd-notice sd-notice--error sd-mhome-notice">复制失败，请手动选取文字。</div>
        ) : null}
      </section>

      <section className="sd-panel sd-mhome-card">
        <div className="sd-mhome-card-title">
          <img src="/assets/stardew/ui/icons/icon_nav_players_avatar_image2.png" alt="" />
          待认证玩家
          {pendingAuthPlayers.length > 0 ? (
            <span className="sd-tag sd-tag-gold sd-mhome-title-tag">待批准 {pendingAuthPlayers.length}</span>
          ) : null}
        </div>
        {!isRunning ? (
          <div className="sd-notice sd-notice--info sd-mhome-notice">服务器运行后才会显示待认证玩家。</div>
        ) : passwordStatus === null ? (
          <div className="sd-notice sd-notice--info sd-mhome-notice">识别中…</div>
        ) : !passwordStatus.enabled ? (
          <div className="sd-notice sd-notice--info sd-mhome-notice">未开启密码认证，玩家无需批准即可进入。</div>
        ) : passwordStatus.passwordBridgeAvailable === false ? (
          <div
            className="sd-notice sd-notice--warn sd-mhome-notice"
            title={passwordStatus.passwordBridgeDetail || undefined}
          >
            控制模组暂不支持批准认证，需要玩家自行输入密码。
          </div>
        ) : pendingAuthPlayers.length === 0 ? (
          <div className="sd-notice sd-notice--ok sd-mhome-notice">暂无待认证玩家。</div>
        ) : (
          <div className="sd-mhome-pending-list">
            {pendingAuthPlayers.map((player) => (
              <div className="sd-mhome-pending-row" key={player.uniqueMultiplayerId || player.name}>
                <span className="sd-mhome-pending-name">{player.name}</span>
                <button
                  type="button"
                  className="sd-btn-green sd-mhome-approve-btn"
                  onClick={() =>
                    setConfirm({
                      kind: 'approve',
                      target: { uniqueMultiplayerId: player.uniqueMultiplayerId ?? '', name: player.name },
                    })
                  }
                  disabled={!isAdmin || !player.uniqueMultiplayerId || approveBusy}
                  title={
                    !isAdmin
                      ? '仅管理员可批准玩家认证'
                      : !player.uniqueMultiplayerId
                        ? '缺少玩家标识，无法批准'
                        : undefined
                  }
                >
                  批准
                </button>
              </div>
            ))}
          </div>
        )}
        {approveMessage ? <div className="sd-notice sd-notice--ok sd-mhome-notice">{approveMessage}</div> : null}
        {approveError ? <div className="sd-notice sd-notice--error sd-mhome-notice">{approveError}</div> : null}
      </section>

      <section className="sd-panel sd-mhome-card">
        <div className="sd-mhome-card-title">
          <img src="/assets/stardew/ui/icons/icon_button_play.png" alt="" />
          快捷控制
        </div>
        <div className="sd-mhome-lifecycle-list">
          {renderLifecycleButtons()}
        </div>
        {actionError ? <div className="sd-notice sd-notice--error sd-mhome-notice">{actionError}</div> : null}
      </section>

      {confirm ? (
        <div className="sd-mhome-confirm-overlay">
          <div className="sd-panel sd-mhome-confirm-dialog">
            {confirm.kind === 'stop' ? (
              <>
                <h3>确认停止服务器</h3>
                <p>停止服务器将断开所有玩家连接，邀请码将失效。</p>
                <div className="sd-mhome-confirm-actions">
                  <button type="button" className="sd-btn-tan sd-mhome-confirm-btn" onClick={() => setConfirm(null)}>
                    取消
                  </button>
                  <button
                    type="button"
                    className="sd-btn-delete sd-mhome-confirm-btn"
                    onClick={() => {
                      setConfirm(null)
                      void handleStop()
                    }}
                  >
                    确认停止
                  </button>
                </div>
              </>
            ) : confirm.kind === 'restart' ? (
              <>
                <h3>确认重启服务器</h3>
                <p>重启服务器将短暂断开所有玩家连接，请确认操作。</p>
                <div className="sd-mhome-confirm-actions">
                  <button type="button" className="sd-btn-tan sd-mhome-confirm-btn" onClick={() => setConfirm(null)}>
                    取消
                  </button>
                  <button
                    type="button"
                    className="sd-btn-green sd-mhome-confirm-btn"
                    onClick={() => {
                      setConfirm(null)
                      void handleRestart()
                    }}
                  >
                    确认重启
                  </button>
                </div>
              </>
            ) : (
              <>
                <h3>确认批准玩家认证</h3>
                <p>
                  批准玩家 {confirm.target.name} 的密码认证？该操作会立即让玩家进入正式农场，等同于服务器替其正确输入了一次密码。
                </p>
                <div className="sd-mhome-confirm-actions">
                  <button
                    type="button"
                    className="sd-btn-tan sd-mhome-confirm-btn"
                    onClick={() => setConfirm(null)}
                    disabled={approveBusy}
                  >
                    取消
                  </button>
                  <button
                    type="button"
                    className="sd-btn-green sd-mhome-confirm-btn"
                    onClick={() => void handleConfirmApprove(confirm.target)}
                    disabled={approveBusy}
                  >
                    {approveBusy ? '批准中…' : '确认批准'}
                  </button>
                </div>
              </>
            )}
          </div>
        </div>
      ) : null}
    </div>
  )
}
