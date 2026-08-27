import { stateLabel } from '../../core/helpers'
import type { SaveInfo } from '../../types'
import { InviteCodeCard } from './InviteCodeCard'
import { LanDirectConnectCard } from './LanDirectConnectCard'
import type { StardewDashboardData } from './stardew-routes'
import { shouldShowPlayerLimitAction } from './server-runtime-settings-state'

type ServerSummaryCardProps = {
  instanceState: StardewDashboardData['instanceState']
  dashboardData: StardewDashboardData
  className?: string
  canEditPlayerLimit: boolean
  onEditPlayerLimit?: () => void
}

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

export function ServerSummaryCard({
  instanceState,
  dashboardData,
  className,
  canEditPlayerLimit,
  onEditPlayerLimit,
}: ServerSummaryCardProps) {
  const state = instanceState?.state ?? null
  const isRunning = state === 'running'
  const isStarting = state === 'starting'
  const playersData = dashboardData.players
  const activeSaveName = dashboardData.saves?.activeSaveName ?? null
  const activeSave = dashboardData.saves?.saves.find(
    (save) => save.isActive || save.name === activeSaveName,
  ) ?? null
  const onlineCountText = playersData?.onlineCount != null ? String(playersData.onlineCount) : '—'
  const maxPlayersText = playersData?.maxPlayers != null ? String(playersData.maxPlayers) : '—'
  const playerSourceText = playersData?.source === 'smapi_control'
    ? 'SMAPI 控制文件'
    : playersData?.source === 'junimo_info'
      ? 'Junimo info'
      : '—'
  const stateLabelText = state
    ? stateLabel(state)
    : dashboardData.loading
      ? '读取中…'
      : '未知'
  const dotClass = isRunning
    ? 'sd-dot sd-dot-green sd-dot-pulse'
    : state === 'stopped' || state === 'error'
      ? 'sd-dot sd-dot-red'
      : isStarting
        ? 'sd-dot sd-dot-yellow sd-dot-pulse'
        : 'sd-dot sd-dot-gray'
  const farmNameText = activeSave
    ? activeSave.farmName
      ? activeSave.farmName
      : activeSave.name
    : activeSaveName ?? '—'
  const hostFarmerText = activeSave?.farmerName ?? '—'
  const gameDateText = activeSave?.gameYear ? saveDate(activeSave) : '—'
  const summaryItems = [
    {
      icon: '/assets/stardew/ui/icons/icon_top_summary_players.png',
      iconClass: '',
      label: '在线玩家',
      value: `${onlineCountText} / ${maxPlayersText}`,
      sub: `最大玩家数：${maxPlayersText}`,
      editPlayerLimit: shouldShowPlayerLimitAction(canEditPlayerLimit, Boolean(onEditPlayerLimit)),
    },
    {
      icon: '/assets/stardew/ui/icons/icon_top_summary_save.png',
      iconClass: '',
      label: '当前存档',
      value: farmNameText,
      sub: activeSave?.name ?? activeSaveName ?? playerSourceText,
      editPlayerLimit: false,
    },
    {
      icon: '/assets/stardew/ui/topbar/icon_topbar_user_avatar_image2_v2.png',
      iconClass: 'sd-server-summary-farmer-icon',
      label: '主机农民',
      value: hostFarmerText,
      sub: activeSave ? `农场主：${gameDateText}` : '当前存档',
      editPlayerLimit: false,
    },
    {
      icon: '/assets/stardew/ui/icons/icon_top_summary_time.png',
      iconClass: '',
      label: '游戏日期',
      value: gameDateText,
      sub: activeSave?.name ?? '星露谷时间',
      editPlayerLimit: false,
    },
  ]

  return (
    <div className={['sd-server-summary', className].filter(Boolean).join(' ')}>
      <div className="sd-server-summary-title">服务器摘要</div>

      <div className="sd-server-summary-grid">
        <div className="sd-server-summary-status">
          <span className="sd-server-summary-label">服务器状态</span>
          <strong className={`sd-server-summary-state sd-server-summary-state-${state ?? 'unknown'}`}>
            <span className={dotClass} aria-hidden="true" />
            {stateLabelText}
          </strong>
          <span className="sd-server-summary-sub">{isRunning ? '正常' : isStarting ? '启动中' : playerSourceText}</span>
        </div>

        {summaryItems.map((item) => (
          <div
            className={`sd-server-summary-item${item.editPlayerLimit ? ' sd-server-summary-item--editable' : ''}`}
            key={item.label}
          >
            <span className="sd-server-summary-label">
              <img className={item.iconClass || undefined} src={item.icon} alt="" />
              {item.label}
            </span>
            <strong className="sd-server-summary-value">{item.value}</strong>
            <div className="sd-server-summary-subrow">
              <span className="sd-server-summary-sub">{item.sub}</span>
              {item.editPlayerLimit ? (
                <button
                  type="button"
                  className="sd-server-summary-limit-btn"
                  onClick={onEditPlayerLimit}
                  aria-label="修改联机人数上限"
                  title="修改联机人数上限"
                >
                  修改上限
                </button>
              ) : null}
            </div>
          </div>
        ))}
      </div>

      <div className="sd-server-summary-invite sd-server-summary-connections">
        <LanDirectConnectCard dashboardData={dashboardData} />
        {instanceState?.steamInviteEnabled === true ? (
          <InviteCodeCard
            instanceState={instanceState}
            dashboardData={dashboardData}
            canManageSteamInvite={canEditPlayerLimit}
          />
        ) : null}
      </div>
    </div>
  )
}
