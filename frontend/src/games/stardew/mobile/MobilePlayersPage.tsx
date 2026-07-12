import { useEffect, useState } from 'react'
import { approvePlayerAuth, banPlayer, getInstancePasswordStatus, kickPlayer, warpPlayerHome } from '../../../api'
import { errorMessage, formatDate } from '../../../core/helpers'
import type { InstancePasswordStatus, StardewPlayerInfo } from '../../../types'
import type { StardewPageProps } from '../stardew-routes'
import { formatStardewLocation } from '../location-format'
import { submitAndWaitForPlayerCommand, type PlayerCommandFeedback } from '../player-command-results'
import './MobilePlayersPage.css'

type MobilePlayersPageProps = Pick<StardewPageProps, 'user' | 'instanceState' | 'dashboardData'>

type PlayerTarget = { uniqueMultiplayerId: string; name: string }

function isWaitingPlayerStatus(status?: string): boolean {
  return status === 'waiting' || status === 'pending' || status === 'joining'
}

function playerStatusText(status?: string): string {
  if (status === 'online') return '在线'
  if (!status) return '未知'
  if (isWaitingPlayerStatus(status)) return '等待'
  return '离线'
}

function playerStatusTagClass(status?: string): string {
  if (status === 'online') return 'sd-tag sd-tag-green sd-mplay-status-tag'
  if (isWaitingPlayerStatus(status)) return 'sd-tag sd-tag-gold sd-mplay-status-tag'
  return 'sd-tag sd-mplay-status-tag'
}

function playerActivityText(player: StardewPlayerInfo): string {
  if (player.status === 'online') return player.onlineFor ? `在线 ${player.onlineFor}` : '在线中'
  if (player.lastSeen) return `最近活动：${formatDate(player.lastSeen)}`
  return '—'
}

function playerLocationText(player: StardewPlayerInfo): string {
  return formatStardewLocation(player)
}

export function MobilePlayersPage({ user, instanceState, dashboardData }: MobilePlayersPageProps) {
  const isAdmin = user.role === 'admin'
  const state = instanceState?.state ?? null
  const isRunning = state === 'running'

  const [kickConfirmTarget, setKickConfirmTarget] = useState<PlayerTarget | null>(null)
  const [warpHomeConfirmTarget, setWarpHomeConfirmTarget] = useState<PlayerTarget | null>(null)
  const [warpHomeBusyId, setWarpHomeBusyId] = useState<string | null>(null)
  const [warpHomeError, setWarpHomeError] = useState<string | null>(null)
  const [warpHomeMessage, setWarpHomeMessage] = useState<string | null>(null)
  const [warpHomeConfirmed, setWarpHomeConfirmed] = useState(false)
  const [kickBusyId, setKickBusyId] = useState<string | null>(null)
  const [kickError, setKickError] = useState<string | null>(null)
  const [kickMessage, setKickMessage] = useState<string | null>(null)
  const [kickConfirmed, setKickConfirmed] = useState(false)
  const [approveConfirmTarget, setApproveConfirmTarget] = useState<PlayerTarget | null>(null)
  const [approveBusyId, setApproveBusyId] = useState<string | null>(null)
  const [approveError, setApproveError] = useState<string | null>(null)
  const [approveMessage, setApproveMessage] = useState<string | null>(null)
  const [approveConfirmed, setApproveConfirmed] = useState(false)
  const [passwordStatus, setPasswordStatus] = useState<InstancePasswordStatus | null>(null)

  const [banConfirmTarget, setBanConfirmTarget] = useState<PlayerTarget | null>(null)
  const [banBusyId, setBanBusyId] = useState<string | null>(null)
  const [banError, setBanError] = useState<string | null>(null)
  const [banMessage, setBanMessage] = useState<string | null>(null)
  const [banConfirmed, setBanConfirmed] = useState(false)

  const playersData = dashboardData.players
  const playerRows = playersData?.players ?? []
  const playersLoading = dashboardData.playersLoading
  const playersError = dashboardData.playersError

  function handleRefresh() {
    dashboardData.refreshPlayers()
  }

  function applyFeedback(
    feedback: PlayerCommandFeedback,
    setMessage: (value: string | null) => void,
    setError: (value: string | null) => void,
    setConfirmed: (value: boolean) => void,
  ) {
    if (feedback.kind === 'failed') {
      setMessage(null)
      setError(feedback.message)
      setConfirmed(false)
    } else {
      setError(null)
      setMessage(feedback.message)
      setConfirmed(feedback.kind === 'succeeded')
    }
  }

  useEffect(() => {
    if (!isRunning) {
      setPasswordStatus(null)
      return
    }
    let cancelled = false
    getInstancePasswordStatus()
      .then((result) => { if (!cancelled) setPasswordStatus(result) })
      .catch(() => { if (!cancelled) setPasswordStatus(null) })
    return () => { cancelled = true }
  }, [isRunning])

  async function handleConfirmKick() {
    const target = kickConfirmTarget
    if (!target) return
    setKickBusyId(target.uniqueMultiplayerId)
    setKickError(null)
    setKickMessage(null)
    try {
      const feedback = await submitAndWaitForPlayerCommand(
        () => kickPlayer(target.uniqueMultiplayerId, target.name),
        'kick',
        target.name,
        (next) => applyFeedback(next, setKickMessage, setKickError, setKickConfirmed),
      )
      if (feedback.kind === 'succeeded') await dashboardData.refreshPlayers()
    } catch (e) {
      setKickError(errorMessage(e))
    } finally {
      setKickBusyId(null)
      setKickConfirmTarget(null)
    }
  }

  async function handleConfirmWarpHome() {
    const target = warpHomeConfirmTarget
    if (!target) return
    setWarpHomeBusyId(target.uniqueMultiplayerId)
    setWarpHomeError(null)
    setWarpHomeMessage(null)
    try {
      const feedback = await submitAndWaitForPlayerCommand(
        () => warpPlayerHome(target.uniqueMultiplayerId, target.name),
        'warp-home',
        target.name,
        (next) => applyFeedback(next, setWarpHomeMessage, setWarpHomeError, setWarpHomeConfirmed),
      )
      if (feedback.kind === 'succeeded') await dashboardData.refreshPlayers()
    } catch (e) {
      setWarpHomeError(errorMessage(e))
    } finally {
      setWarpHomeBusyId(null)
      setWarpHomeConfirmTarget(null)
    }
  }

  async function handleConfirmApprove() {
    const target = approveConfirmTarget
    if (!target) return
    setApproveBusyId(target.uniqueMultiplayerId)
    setApproveError(null)
    setApproveMessage(null)
    try {
      const feedback = await submitAndWaitForPlayerCommand(
        () => approvePlayerAuth(target.uniqueMultiplayerId),
        'approve-auth',
        target.name,
        (next) => applyFeedback(next, setApproveMessage, setApproveError, setApproveConfirmed),
      )
      if (feedback.kind === 'succeeded') await dashboardData.refreshPlayers()
    } catch (e) {
      setApproveError(errorMessage(e))
    } finally {
      setApproveBusyId(null)
      setApproveConfirmTarget(null)
    }
  }

  async function handleConfirmBan() {
    const target = banConfirmTarget
    if (!target) return
    setBanBusyId(target.uniqueMultiplayerId)
    setBanError(null)
    setBanMessage(null)
    try {
      const feedback = await submitAndWaitForPlayerCommand(
        () => banPlayer(target.name, target.uniqueMultiplayerId),
        'ban',
        target.name,
        (next) => applyFeedback(next, setBanMessage, setBanError, setBanConfirmed),
      )
      if (feedback.kind === 'succeeded') await dashboardData.refreshPlayers()
    } catch (e) {
      setBanError(errorMessage(e))
    } finally {
      setBanBusyId(null)
      setBanConfirmTarget(null)
    }
  }

  const isPlayerActionBusy = (playerId: string) =>
    warpHomeBusyId === playerId || kickBusyId === playerId || approveBusyId === playerId || banBusyId === playerId

  return (
    <div className="sd-mplay-wrap">
      <section className="sd-panel sd-mplay-card">
        <div className="sd-mplay-header-row">
          <div className="sd-mplay-header-title">
            <img src="/assets/stardew/ui/icons/icon_nav_players_avatar_image2.png" alt="" />
            在线玩家
          </div>
          <button
            type="button"
            className="sd-btn-tan sd-mplay-refresh-btn"
            onClick={handleRefresh}
            disabled={playersLoading}
          >
            {playersLoading ? '刷新中…' : '刷新'}
          </button>
        </div>

        {playersError ? (
          <div className="sd-notice sd-notice--error sd-mplay-notice">读取玩家数据失败：{playersError}</div>
        ) : null}

        {playersLoading && !playersData ? (
          <div className="sd-mplay-empty">
            <div className="sd-mplay-empty-title">正在读取玩家列表</div>
          </div>
        ) : playerRows.length === 0 ? (
          <div className="sd-mplay-empty">
            <div className="sd-mplay-empty-title">暂无在线玩家</div>
            <div className="sd-mplay-empty-desc">服务器运行并有玩家进入后，会在这里显示玩家状态。</div>
          </div>
        ) : (
          <div className="sd-mplay-player-list">
            {playerRows.map((player) => (
              <div className="sd-mplay-player-card" key={player.uniqueMultiplayerId || player.name}>
                <div className="sd-mplay-player-top">
                  <span className="sd-mplay-player-name">{player.name}</span>
                  <span className={playerStatusTagClass(player.status)}>{playerStatusText(player.status)}</span>
                </div>
                <div className="sd-mplay-player-meta">
                  {player.isHost ? <span className="sd-tag sd-mplay-meta-tag">主机</span> : null}
                  {player.role ? <span className="sd-tag sd-mplay-meta-tag">{player.role}</span> : null}
                  <span className="sd-mplay-player-activity">{playerActivityText(player)}</span>
                </div>
                <div className="sd-mplay-player-bottom">
                  <span className="sd-mplay-player-location" title={playerLocationText(player)}>
                    {playerLocationText(player)}
                  </span>
                  <div className="sd-mplay-player-actions">
                    <button
                      type="button"
                      className="sd-btn-green sd-mplay-player-action-btn"
                      disabled={
                        !isAdmin ||
                        !isRunning ||
                        player.status !== 'online' ||
                        player.isHost ||
                        !player.uniqueMultiplayerId ||
                        isPlayerActionBusy(player.uniqueMultiplayerId)
                      }
                      title={
                        !isAdmin
                          ? '仅管理员可用'
                          : player.isHost
                            ? '主机没有可传送的小屋'
                            : player.status !== 'online'
                              ? '玩家不在线'
                              : !player.uniqueMultiplayerId
                                ? '缺少玩家联机 ID，暂不支持传送回家'
                                : '传送玩家回家'
                      }
                      onClick={() =>
                        setWarpHomeConfirmTarget({ uniqueMultiplayerId: player.uniqueMultiplayerId || '', name: player.name })
                      }
                    >
                      {warpHomeBusyId === player.uniqueMultiplayerId ? '处理中…' : '回家'}
                    </button>
                    <button
                      type="button"
                      className="sd-btn-delete sd-mplay-player-action-btn"
                      disabled={
                        !isAdmin ||
                        !isRunning ||
                        player.status !== 'online' ||
                        player.isHost ||
                        !player.uniqueMultiplayerId ||
                        isPlayerActionBusy(player.uniqueMultiplayerId)
                      }
                      title={
                        !isAdmin
                          ? '仅管理员可用'
                          : player.isHost
                            ? '无法踢出主机玩家'
                            : player.status !== 'online'
                              ? '玩家不在线'
                              : !player.uniqueMultiplayerId
                                ? '缺少玩家联机 ID，暂不支持踢出'
                                : '踢出玩家'
                      }
                      onClick={() =>
                        setKickConfirmTarget({ uniqueMultiplayerId: player.uniqueMultiplayerId || '', name: player.name })
                      }
                    >
                      {kickBusyId === player.uniqueMultiplayerId ? '处理中…' : '踢出'}
                    </button>
                    <button
                      type="button"
                      className="sd-btn-delete sd-mplay-player-action-btn"
                      disabled={!isAdmin || !isRunning || player.isHost || !player.uniqueMultiplayerId || isPlayerActionBusy(player.uniqueMultiplayerId)}
                      title={
                        !isAdmin
                          ? '仅管理员可用'
                          : player.isHost
                            ? '无法封禁主机玩家'
                            : !player.uniqueMultiplayerId
                              ? '缺少玩家联机 ID，暂不支持封禁'
                              : '封禁玩家'
                      }
                      onClick={() =>
                        setBanConfirmTarget({ uniqueMultiplayerId: player.uniqueMultiplayerId || '', name: player.name })
                      }
                    >
                      {banBusyId === player.uniqueMultiplayerId ? '处理中…' : '封禁'}
                    </button>
                    {passwordStatus?.enabled && player.isAuthenticated === false ? (
                      <button
                        type="button"
                        className="sd-btn-green sd-mplay-player-action-btn"
                        disabled={
                          !isAdmin ||
                          !isRunning ||
                          player.isHost ||
                          !player.uniqueMultiplayerId ||
                          !passwordStatus.passwordBridgeAvailable ||
                          isPlayerActionBusy(player.uniqueMultiplayerId)
                        }
                        title={!passwordStatus.passwordBridgeAvailable ? '密码认证反射桥不可用' : '批准该玩家认证'}
                        onClick={() => setApproveConfirmTarget({ uniqueMultiplayerId: player.uniqueMultiplayerId || '', name: player.name })}
                      >
                        {approveBusyId === player.uniqueMultiplayerId ? '处理中…' : '批准认证'}
                      </button>
                    ) : null}
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}

        {kickMessage ? <div className={`sd-notice ${kickConfirmed ? 'sd-notice--ok' : ''} sd-mplay-notice`}>{kickMessage}</div> : null}
        {kickError ? <div className="sd-notice sd-notice--error sd-mplay-notice">{kickError}</div> : null}
        {warpHomeMessage ? <div className={`sd-notice ${warpHomeConfirmed ? 'sd-notice--ok' : ''} sd-mplay-notice`}>{warpHomeMessage}</div> : null}
        {warpHomeError ? <div className="sd-notice sd-notice--error sd-mplay-notice">{warpHomeError}</div> : null}
        {approveMessage ? <div className={`sd-notice ${approveConfirmed ? 'sd-notice--ok' : ''} sd-mplay-notice`}>{approveMessage}</div> : null}
        {approveError ? <div className="sd-notice sd-notice--error sd-mplay-notice">{approveError}</div> : null}
        {banMessage ? <div className={`sd-notice ${banConfirmed ? 'sd-notice--ok' : ''} sd-mplay-notice`}>{banMessage}</div> : null}
        {banError ? <div className="sd-notice sd-notice--error sd-mplay-notice">{banError}</div> : null}
      </section>

      {warpHomeConfirmTarget ? (
        <div className="sd-mplay-confirm-overlay" role="dialog" aria-modal="true">
          <div className="sd-panel sd-mplay-confirm-dialog">
            <h3>确认传送回家</h3>
            <p>将玩家 {warpHomeConfirmTarget.name} 传送回自己的小屋？该操作适合玩家卡在地图或建筑边缘时救援。</p>
            <div className="sd-mplay-confirm-actions">
              <button
                type="button"
                className="sd-btn-tan sd-mplay-confirm-btn"
                onClick={() => setWarpHomeConfirmTarget(null)}
                disabled={warpHomeBusyId !== null}
              >
                取消
              </button>
              <button
                type="button"
                className="sd-btn-green sd-mplay-confirm-btn"
                onClick={() => void handleConfirmWarpHome()}
                disabled={warpHomeBusyId !== null}
              >
                {warpHomeBusyId !== null ? '传送中…' : '确认传送'}
              </button>
            </div>
          </div>
        </div>
      ) : null}

      {kickConfirmTarget ? (
        <div className="sd-mplay-confirm-overlay" role="dialog" aria-modal="true">
          <div className="sd-panel sd-mplay-confirm-dialog">
            <h3>确认踢出玩家</h3>
            <p>将玩家 {kickConfirmTarget.name} 踢出服务器？该操作会立即断开该玩家的连接，玩家可以重新加入。</p>
            <div className="sd-mplay-confirm-actions">
              <button
                type="button"
                className="sd-btn-tan sd-mplay-confirm-btn"
                onClick={() => setKickConfirmTarget(null)}
                disabled={kickBusyId !== null}
              >
                取消
              </button>
              <button
                type="button"
                className="sd-btn-delete sd-mplay-confirm-btn"
                onClick={() => void handleConfirmKick()}
                disabled={kickBusyId !== null}
              >
                {kickBusyId !== null ? '踢出中…' : '确认踢出'}
              </button>
            </div>
          </div>
        </div>
      ) : null}

      {approveConfirmTarget ? (
        <div className="sd-mplay-confirm-overlay" role="dialog" aria-modal="true">
          <div className="sd-panel sd-mplay-confirm-dialog">
            <h3>确认批准认证</h3>
            <p>批准玩家 {approveConfirmTarget.name} 的密码认证？该操作会让玩家进入正式农场。</p>
            <div className="sd-mplay-confirm-actions">
              <button
                type="button"
                className="sd-btn-tan sd-mplay-confirm-btn"
                onClick={() => setApproveConfirmTarget(null)}
                disabled={approveBusyId !== null}
              >
                取消
              </button>
              <button
                type="button"
                className="sd-btn-green sd-mplay-confirm-btn"
                onClick={() => void handleConfirmApprove()}
                disabled={approveBusyId !== null}
              >
                {approveBusyId !== null ? '处理中…' : '确认批准'}
              </button>
            </div>
          </div>
        </div>
      ) : null}

      {banConfirmTarget ? (
        <div className="sd-mplay-confirm-overlay" role="dialog" aria-modal="true">
          <div className="sd-panel sd-mplay-confirm-dialog">
            <h3>确认封禁玩家</h3>
            <p>
              封禁玩家 {banConfirmTarget.name}？控制模组会优先按联机 ID 精确封禁；封禁记录在服务器容器重启后会丢失，需要重新操作。
            </p>
            <div className="sd-mplay-confirm-actions">
              <button
                type="button"
                className="sd-btn-tan sd-mplay-confirm-btn"
                onClick={() => setBanConfirmTarget(null)}
                disabled={banBusyId !== null}
              >
                取消
              </button>
              <button
                type="button"
                className="sd-btn-delete sd-mplay-confirm-btn"
                onClick={() => void handleConfirmBan()}
                disabled={banBusyId !== null}
              >
                {banBusyId !== null ? '封禁中…' : '确认封禁'}
              </button>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  )
}
