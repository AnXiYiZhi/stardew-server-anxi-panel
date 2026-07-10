import { useState } from 'react'
import { banPlayer, kickPlayer } from '../../../api'
import { errorMessage, formatDate } from '../../../core/helpers'
import type { StardewPlayerInfo } from '../../../types'
import type { StardewPageProps } from '../stardew-routes'
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
  const name = player.locationDisplayName || player.locationName || player.location
  if (!name) return '—'
  if (typeof player.tileX === 'number' && typeof player.tileY === 'number') {
    return `${name} (${player.tileX}, ${player.tileY})`
  }
  return name
}

export function MobilePlayersPage({ user, instanceState, dashboardData }: MobilePlayersPageProps) {
  const isAdmin = user.role === 'admin'
  const state = instanceState?.state ?? null
  const isRunning = state === 'running'

  const [kickConfirmTarget, setKickConfirmTarget] = useState<PlayerTarget | null>(null)
  const [kickBusyId, setKickBusyId] = useState<string | null>(null)
  const [kickError, setKickError] = useState<string | null>(null)
  const [kickMessage, setKickMessage] = useState<string | null>(null)

  const [banConfirmTarget, setBanConfirmTarget] = useState<PlayerTarget | null>(null)
  const [banBusyId, setBanBusyId] = useState<string | null>(null)
  const [banError, setBanError] = useState<string | null>(null)
  const [banMessage, setBanMessage] = useState<string | null>(null)

  const playersData = dashboardData.players
  const playerRows = playersData?.players ?? []
  const playersLoading = dashboardData.playersLoading
  const playersError = dashboardData.playersError

  function handleRefresh() {
    dashboardData.refreshPlayers()
  }

  async function handleConfirmKick() {
    const target = kickConfirmTarget
    if (!target) return
    setKickBusyId(target.uniqueMultiplayerId)
    setKickError(null)
    setKickMessage(null)
    try {
      const res = await kickPlayer(target.uniqueMultiplayerId, target.name)
      setKickMessage(res.output?.trim() || `已提交踢出 ${target.name} 的指令。`)
      await dashboardData.refreshPlayers()
    } catch (e) {
      setKickError(errorMessage(e))
    } finally {
      setKickBusyId(null)
      setKickConfirmTarget(null)
    }
  }

  async function handleConfirmBan() {
    const target = banConfirmTarget
    if (!target) return
    setBanBusyId(target.uniqueMultiplayerId)
    setBanError(null)
    setBanMessage(null)
    try {
      const res = await banPlayer(target.name, target.uniqueMultiplayerId)
      setBanMessage(res.output?.trim() || `已提交封禁 ${target.name} 的指令。`)
      await dashboardData.refreshPlayers()
    } catch (e) {
      setBanError(errorMessage(e))
    } finally {
      setBanBusyId(null)
      setBanConfirmTarget(null)
    }
  }

  const rosterActionBusy = kickBusyId !== null || banBusyId !== null

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
                      className="sd-btn-delete sd-mplay-player-action-btn"
                      disabled={
                        !isAdmin ||
                        !isRunning ||
                        player.status !== 'online' ||
                        player.isHost ||
                        !player.uniqueMultiplayerId ||
                        rosterActionBusy
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
                      disabled={!isAdmin || !isRunning || player.isHost || !player.uniqueMultiplayerId || rosterActionBusy}
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
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}

        {kickMessage ? <div className="sd-notice sd-notice--ok sd-mplay-notice">{kickMessage}</div> : null}
        {kickError ? <div className="sd-notice sd-notice--error sd-mplay-notice">{kickError}</div> : null}
        {banMessage ? <div className="sd-notice sd-notice--ok sd-mplay-notice">{banMessage}</div> : null}
        {banError ? <div className="sd-notice sd-notice--error sd-mplay-notice">{banError}</div> : null}
      </section>

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

      {banConfirmTarget ? (
        <div className="sd-mplay-confirm-overlay" role="dialog" aria-modal="true">
          <div className="sd-panel sd-mplay-confirm-dialog">
            <h3>确认封禁玩家</h3>
            <p>
              封禁玩家 {banConfirmTarget.name}？该玩家会被立即断开且暂时无法重新加入服务器；如果之后重启了服务器容器，这条封禁可能会失效，需要重新操作。
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
