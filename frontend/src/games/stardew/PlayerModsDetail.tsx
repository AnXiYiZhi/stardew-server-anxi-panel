import { useEffect, useMemo, useState } from 'react'
import { getPlayerModDetails } from '../../api'
import { errorMessage, formatDate } from '../../core/helpers'
import type { PlayerModComparisonItem, PlayerModDetailsResult, StardewPlayerInfo } from '../../types'
import {
  groupPlayerModItems,
  hasCjbRisk,
  hasPlayerCjbRisk,
  isCjbMod,
  PLAYER_CJB_BANNER_TITLE,
  PLAYER_CJB_DETECTED_LABEL,
  resolvePlayerModViewState,
  type PlayerModViewKind,
} from './player-mod-details'
import './PlayerModsDetail.css'

const GROUP_INITIAL_LIMIT = 60

type PlayerModsDetailProps = {
  playerId: string
  instanceId: string
  player?: StardewPlayerInfo | null
  onBack?: () => void
}

type ModGroupProps = {
  id: string
  title: string
  note: string
  tone: 'match' | 'missing' | 'mismatch' | 'extra'
  items: PlayerModComparisonItem[]
}

function versionText(value: string | undefined): string {
  return value?.trim() || '—'
}

function syncKindText(item: PlayerModComparisonItem): string | null {
  if (item.syncKind === 'server_only') return '服务器专用'
  if (item.syncKind === 'client_required') return '玩家需同步'
  if (item.syncKind === 'unknown') return '分类待确认'
  return null
}

function resultText(item: PlayerModComparisonItem): string {
  if (item.result === 'match') return '版本匹配'
  if (item.result === 'missing_on_client') return '玩家缺少 Mod'
  if (item.result === 'version_mismatch') return '版本不同'
  return '玩家额外安装'
}

function ModRow({ item }: { item: PlayerModComparisonItem }) {
  const syncKind = syncKindText(item)
  const cjb = isCjbMod(item)
  return (
    <li className={`sd-pmods-item sd-pmods-item--${item.result}${cjb ? ' sd-pmods-item--cjb' : ''}`}>
      <div className="sd-pmods-item-heading">
        <div className="sd-pmods-item-name-block">
          <strong title={item.name || item.uniqueId}>{item.name || item.uniqueId}</strong>
          <code title={item.uniqueId}>{item.uniqueId}</code>
        </div>
        <div className="sd-pmods-item-badges">
          <span className={`sd-pmods-result-badge sd-pmods-result-badge--${item.result}`}>
            {resultText(item)}
          </span>
          {syncKind ? <span className="sd-pmods-sync-badge">{syncKind}</span> : null}
          {cjb ? <span className="sd-pmods-cjb-badge">{PLAYER_CJB_DETECTED_LABEL}</span> : null}
        </div>
      </div>
      <dl className="sd-pmods-versions">
        <div>
          <dt>服务器版本</dt>
          <dd title={item.serverVersion}>{versionText(item.serverVersion)}</dd>
        </div>
        <div>
          <dt>客户端版本</dt>
          <dd title={item.clientVersion}>
            {item.result === 'missing_on_client' ? '未上报' : versionText(item.clientVersion)}
          </dd>
        </div>
      </dl>
    </li>
  )
}

function ModGroup({ id, title, note, tone, items }: ModGroupProps) {
  const [expanded, setExpanded] = useState(false)
  if (items.length === 0) return null
  const visibleItems = expanded ? items : items.slice(0, GROUP_INITIAL_LIMIT)
  const hiddenCount = items.length - visibleItems.length

  return (
    <section className={`sd-pmods-group sd-pmods-group--${tone}`} aria-labelledby={id}>
      <header className="sd-pmods-group-heading">
        <div>
          <h3 id={id}>{title}</h3>
          <p>{note}</p>
        </div>
        <span className="sd-pmods-group-count">{items.length}</span>
      </header>
      <ul className="sd-pmods-list">
        {visibleItems.map((item) => (
          <ModRow key={`${item.result}:${item.uniqueId.toLocaleLowerCase('en-US')}`} item={item} />
        ))}
      </ul>
      {hiddenCount > 0 ? (
        <button className="sd-pmods-more" type="button" onClick={() => setExpanded(true)}>
          再显示 {hiddenCount} 项
        </button>
      ) : null}
    </section>
  )
}

function noticeTitle(kind: PlayerModViewKind): string {
  if (kind === 'request_error') return 'Mod 详情读取失败'
  if (kind === 'pending') return '等待客户端上报'
  if (kind === 'unavailable') return '客户端清单不可用'
  if (kind === 'stale') return '这是过期的上报记录'
  if (kind === 'comparison_unavailable') return '服务器比较基准不可用'
  return ''
}

export function PlayerModsDetail({ playerId, instanceId, player, onBack }: PlayerModsDetailProps) {
  const [details, setDetails] = useState<PlayerModDetailsResult | null>(null)
  const [loading, setLoading] = useState(Boolean(playerId))
  const [requestError, setRequestError] = useState<string | null>(playerId ? null : '链接缺少 playerId，无法读取玩家 Mod。')
  const [retryNonce, setRetryNonce] = useState(0)

  useEffect(() => {
    if (!playerId) return
    const controller = new AbortController()
    setLoading(true)
    setRequestError(null)
    setDetails(null)
    getPlayerModDetails(playerId, instanceId, controller.signal)
      .then(setDetails)
      .catch((error: unknown) => {
        if (controller.signal.aborted) return
        setRequestError(errorMessage(error))
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false)
      })
    return () => controller.abort()
  }, [instanceId, playerId, retryNonce])

  const viewState = resolvePlayerModViewState(details, requestError)
  const groups = useMemo(() => groupPlayerModItems(details?.comparison.items ?? []), [details])
  const playerName = player?.name?.trim() || '玩家 Mod 详情'
  const inferredOnline = player?.status === 'online'
  const statusText = player ? (inferredOnline ? '在线' : '离线') : details?.contextStatus === 'stale' ? '离线' : '状态待同步'
  const reportedAt = loading ? '读取中…' : details?.reportedAt ? formatDate(details.reportedAt) : '尚未上报'
  const cjbRisk = hasCjbRisk(details) || hasPlayerCjbRisk(player)

  return (
    <article className="sd-pmods" aria-busy={loading}>
      {onBack ? (
        <button className="sd-pmods-back" type="button" onClick={onBack}>
          <span aria-hidden="true">←</span>
          返回玩家列表
        </button>
      ) : null}

      <section className="sd-pmods-identity" aria-labelledby="sd-pmods-player-name">
        <div className="sd-pmods-avatar" aria-hidden="true">
          {(playerName === '玩家 Mod 详情' ? '?' : playerName.slice(0, 1)).toUpperCase()}
        </div>
        <div className="sd-pmods-identity-copy">
          <p className="sd-pmods-kicker">客户端清单核对簿</p>
          <div className="sd-pmods-name-row">
            <h2 id="sd-pmods-player-name" title={playerName}>{playerName}</h2>
            <span className={`sd-pmods-online sd-pmods-online--${inferredOnline ? 'online' : 'offline'}`}>
              <span aria-hidden="true" />{statusText}
            </span>
          </div>
          <code className="sd-pmods-player-id" title={playerId}>联机 ID · {playerId || '缺失'}</code>
        </div>
        <dl className="sd-pmods-meta">
          <div>
            <dt>上报时间</dt>
            <dd>{reportedAt}</dd>
          </div>
          <div>
            <dt>游戏版本</dt>
            <dd title={details?.gameVersion}>{loading ? '读取中…' : versionText(details?.gameVersion)}</dd>
          </div>
          <div>
            <dt>SMAPI 版本</dt>
            <dd title={details?.apiVersion}>
              {loading ? '读取中…' : details?.hasSmapi === false ? '未检测到 SMAPI' : versionText(details?.apiVersion)}
            </dd>
          </div>
        </dl>
      </section>

      {cjbRisk ? (
        <section className="sd-pmods-risk" role="alert" aria-label="检测到 CJB 作弊工具">
          <strong>{PLAYER_CJB_BANNER_TITLE}</strong>
        </section>
      ) : null}

      {loading ? (
        <section className="sd-pmods-loading" aria-live="polite">
          <span className="sd-pmods-loading-mark" aria-hidden="true" />
          <div><strong>正在读取 Mod 清单</strong><p>正在获取客户端上报与服务器实际加载清单的比较结果…</p></div>
        </section>
      ) : viewState.kind !== 'available' ? (
        <section className={`sd-pmods-notice sd-pmods-notice--${viewState.kind}`} role={viewState.kind === 'request_error' ? 'alert' : 'status'}>
          <div>
            <strong>{noticeTitle(viewState.kind)}</strong>
            <p>{viewState.message}</p>
          </div>
          {viewState.kind === 'request_error' && playerId ? (
            <button className="sd-pmods-retry" type="button" onClick={() => setRetryNonce((value) => value + 1)}>
              重新读取
            </button>
          ) : null}
        </section>
      ) : null}

      {!loading && viewState.showComparison ? (
        <>
          <dl className="sd-pmods-tally" aria-label="Mod 比较统计">
            <div className="sd-pmods-tally--extra"><dt>玩家额外安装</dt><dd>{groups.clientOnly.length}</dd></div>
            <div className="sd-pmods-tally--missing"><dt>玩家缺少 Mod</dt><dd>{groups.missingOnClient.length}</dd></div>
            <div className="sd-pmods-tally--mismatch"><dt>版本不同</dt><dd>{groups.versionMismatch.length}</dd></div>
            <div className="sd-pmods-tally--match"><dt>匹配</dt><dd>{groups.match.length}</dd></div>
          </dl>

          {groups.match.length + groups.missingOnClient.length + groups.versionMismatch.length + groups.clientOnly.length === 0 ? (
            <section className="sd-pmods-empty">
              <strong>没有可展示的比较条目</strong>
              <p>服务器已完成比较，但这份上报里没有匹配项或差异项。</p>
            </section>
          ) : (
            <div className="sd-pmods-groups">
              <ModGroup id="sd-pmods-extra" title="玩家额外安装" note="玩家上报了服务器进程未加载的 Mod。" tone="extra" items={groups.clientOnly} />
              <ModGroup id="sd-pmods-missing" title="玩家缺少 Mod" note="仅包含服务器已启用且要求玩家同步的 Mod。" tone="missing" items={groups.missingOnClient} />
              <ModGroup id="sd-pmods-mismatch" title="版本不同" note="服务器与客户端均有此 Mod，但版本不同。" tone="mismatch" items={groups.versionMismatch} />
              <ModGroup id="sd-pmods-match" title="匹配" note="服务器与玩家上报了相同版本。" tone="match" items={groups.match} />
            </div>
          )}
        </>
      ) : null}
    </article>
  )
}
