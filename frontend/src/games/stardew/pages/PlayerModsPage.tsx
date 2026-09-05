import type { StardewPageProps } from '../stardew-routes'
import { PlayerModsDetail } from '../PlayerModsDetail'

type PlayerModsPageProps = StardewPageProps & {
  playerId: string
}

export function PlayerModsPage({ playerId, instanceId, dashboardData, onNavigate }: PlayerModsPageProps) {
  const player = dashboardData.players?.players.find(
    (entry) => entry.uniqueMultiplayerId === playerId,
  )

  return (
    <div className="sd-page sd-player-mods-page">
      <PlayerModsDetail
        playerId={playerId}
        instanceId={instanceId}
        player={player}
        onBack={() => onNavigate('players')}
      />
    </div>
  )
}
