import { useState } from 'react'
import { stateLabel, formatDate } from '../../../core/helpers'
import type { StardewPageProps } from '../stardew-routes'

type PlayerLocationLike = {
  location?: string
  locationName?: string
  locationDisplayName?: string
  tileX?: number
  tileY?: number
}

const LOCATION_ZH: Record<string, string> = {
  AbandonedJojaMart: '废弃 Joja 超市',
  AdventureGuild: '探险家公会',
  AnimalShop: '玛妮的牧场',
  ArchaeologyHouse: '星露谷博物馆和图书馆',
  Backwoods: '边远森林',
  Backwoods_GraveSite: '边远森林墓地',
  Backwoods_Staircase: '边远森林楼梯',
  Barn: '畜棚',
  Barn2: '大畜棚',
  Barn3: '高级畜棚',
  BathHouse_Entry: '温泉',
  BathHouse_MensLocker: '温泉男更衣室',
  BathHouse_Pool: '温泉浴池',
  BathHouse_WomensLocker: '温泉女更衣室',
  Beach: '鹈鹕镇海滩',
  BeachNightMarket: '夜市海滩',
  'Beach-Jellies': '月光水母节海滩',
  'Beach-Jellies2': '月光水母节海滩',
  'Beach-Luau': '夏威夷宴会海滩',
  'Beach-Luau2': '夏威夷宴会海滩',
  'Beach-NightMarket': '夜市海滩',
  Beach_SquidFest: '鱿鱼节海滩',
  Blacksmith: '铁匠铺',
  BoatTunnel: '威利船舱通道',
  BugLand: '突变虫穴',
  BusStop: '巴士站',
  Caldera: '火山顶',
  CaptainRoom: '船长室',
  Cellar: '地窖',
  Club: '赌场',
  CommunityCenter: '社区中心',
  CommunityCenter_Joja: 'Joja 社区开发仓库',
  CommunityCenter_Refurbished: '翻新后的社区中心',
  CommunityCenter_Ruins: '废弃社区中心',
  Coop: '鸡舍',
  Coop2: '大鸡舍',
  Coop3: '高级鸡舍',
  Darkroom: '暗室',
  Default: '默认地点',
  Desert: '卡利科沙漠',
  DesertFestival: '沙漠节',
  'Desert-Festival': '沙漠节',
  ElliottHouse: '艾利欧特小屋',
  ElliottSea: '艾利欧特海上场景',
  EmilyDreamscape: '艾米丽梦境',
  Farm: '农场',
  Farm_Beach: '海滩农场',
  Farm_Combat: '荒野农场',
  Farm_Fishing: '河边农场',
  Farm_Foraging: '森林农场',
  Farm_Forest: '森林农场',
  Farm_FourCorners: '四角农场',
  Farm_Greenhouse_Dirt: '温室',
  Farm_Greenhouse_Dirt_FourCorners: '温室',
  Farm_Hilltop: '山顶农场',
  Farm_Island: '姜岛农场',
  Farm_MeadowlandsFarm: '草原农场',
  Farm_Mining: '山顶农场',
  Farm_Ranching: '草原农场',
  Farm_Riverland: '河边农场',
  Farm_Standard: '标准农场',
  Farm_Wilderness: '荒野农场',
  FarmCave: '农场山洞',
  FarmHouse: '农舍',
  FarmHouse1: '农舍',
  FarmHouse1_marriage: '农舍',
  FarmHouse2: '农舍',
  FarmHouse2_marriage: '农舍',
  fishingGame: '钓鱼小游戏',
  FishingGame: '钓鱼小游戏',
  FishShop: '鱼店',
  Forest: '煤矿森林',
  Forest_FishingDerby: '钓鱼大赛森林',
  Forest_RaccoonHouse: '浣熊树屋',
  Forest_RaccoonStump: '浣熊树桩',
  'Forest-FlowerFestival': '花舞节森林',
  'Forest-FlowerFestival2': '花舞节森林',
  'Forest-IceFestival': '冰雪节森林',
  'Forest-IceFestival2': '冰雪节森林',
  'Forest-SewerClean': '煤矿森林',
  Greenhouse: '温室',
  HaleyHouse: '艾米丽和海莉的家',
  HarveyBalloon: '哈维热气球场景',
  HarveyRoom: '哈维的房间',
  Hospital: '哈维的诊所',
  Island_CaptainRoom: '姜岛船长室',
  Island_E: '姜岛东部',
  Island_FarmCave: '姜岛农场山洞',
  Island_FieldOffice: '岛屿办事处',
  Island_House_Bin: '姜岛农舍信箱',
  Island_House_Cave: '姜岛农舍洞穴',
  Island_House_Restored: '姜岛农舍',
  Island_Hut: '姜岛小屋',
  Island_N: '姜岛北部',
  Island_N_Trader: '姜岛商人',
  Island_Resort: '姜岛度假村',
  Island_S: '姜岛南部',
  Island_SE: '姜岛东南部',
  Island_Secret: '姜岛密室',
  Island_Shrine: '姜岛神龛',
  Island_W: '姜岛西部',
  Island_W_Obelisk: '姜岛西部方尖碑',
  IslandEast: '姜岛东部',
  IslandFarmCave: '姜岛农场山洞',
  IslandFarmHouse: '姜岛农舍',
  IslandFieldOffice: '岛屿办事处',
  IslandHut: '姜岛小屋',
  IslandNorth: '姜岛北部',
  IslandNorthCave1: '姜岛北部洞穴',
  IslandShrine: '姜岛神龛',
  IslandSouth: '姜岛南部',
  IslandSouthEast: '姜岛东南部',
  IslandSouthEastCave: '姜岛东南洞穴',
  IslandSouthEastCave_pirates: '海盗湾',
  IslandWest: '姜岛西部',
  IslandWestCave1: '姜岛西部洞穴',
  JojaMart: 'Joja 超市',
  JoshHouse: '乔治、艾芙琳和亚历克斯的家',
  LeahHouse: '莉亚的农舍',
  LeoTreeHouse: '雷欧的树屋',
  LewisBasement: '刘易斯地下室',
  ManorHouse: '镇长的庄园',
  MarnieBarn: '玛妮的畜棚',
  MaruBasement: '玛鲁地下室',
  MasteryCave: '精通山洞',
  MermaidHouse: '美人鱼屋',
  Mine: '矿井',
  Mines: '矿井',
  Mountain: '山',
  MovieTheater: '电影院',
  MovieTheaterScreen: '电影院放映厅',
  QiNutRoom: '齐先生核桃房',
  Railroad: '铁路',
  RefurbishedSaloonRoom: '翻新酒吧房间',
  Saloon: '星之果实酒吧',
  SamHouse: '乔迪、肯特和山姆的家',
  SamShow: '山姆演出场景',
  SandyHouse: '桑迪的绿洲',
  ScienceHouse: '木匠的商店',
  SebastianMountain: '塞巴斯蒂安山顶场景',
  SebastianRide: '塞巴斯蒂安骑行场景',
  SebastianRoom: '塞巴斯蒂安的房间',
  SeedShop: '皮埃尔的杂货店',
  Sewer: '下水道',
  Shed: '小屋',
  Shed2: '大屋',
  SkullCave: '骷髅洞穴',
  SkullCaveAltar: '骷髅洞穴祭坛',
  SlimeHutch: '史莱姆屋',
  Stadium: '体育场',
  Submarine: '潜水艇',
  Summit: '山顶',
  Sunroom: '日光室',
  TargetGame: '靶场小游戏',
  Temp: '临时地点',
  Tent: '帐篷',
  Town: '鹈鹕镇',
  'Town-Christmas': '冬日星盛宴小镇',
  'Town-Christmas2': '冬日星盛宴小镇',
  'Town-EggFestival': '复活节小镇',
  'Town-EggFestival2': '复活节小镇',
  'Town-Fair': '星露谷展览会小镇',
  'Town-Fair2': '星露谷展览会小镇',
  'Town-Halloween': '万灵节小镇',
  'Town-Halloween2': '万灵节小镇',
  'Town-Theater': '电影院小镇',
  'Town-TheaterCC': '电影院小镇',
  'Town-TheaterCC-Halloween2': '万灵节电影院小镇',
  Trailer: '潘姆和潘妮之家',
  Trailer_Big: '潘姆和潘妮之家',
  Trailer_big: '潘姆和潘妮之家',
  Tunnel: '隧道',
  UndergroundMine: '矿井',
  Volcano: '火山',
  VolcanoDungeon: '火山地牢',
  WitchHut: '女巫小屋',
  WitchSwamp: '女巫沼泽',
  WitchWarpCave: '女巫传送洞穴',
  WizardHouse: '法师塔',
  WizardHouseBasement: '法师塔地下室',
  Woods: '秘密树林',
}

function normalizeLocationKey(value?: string): string {
  return (value ?? '').trim()
}

function generatedLocationLabel(key: string): string | null {
  if (/^Barn\d*$/i.test(key)) return '畜棚'
  if (/^Cabin\d*$/i.test(key)) return '小屋'
  if (/^Cellar\d*$/i.test(key)) return '地窖'
  if (/^Coop\d*$/i.test(key)) return '鸡舍'
  if (/^FarmHouse\d*$/i.test(key)) return '农舍'
  if (/^Shed\d*$/i.test(key)) return '小屋'
  if (/^VolcanoDungeon\d*$/i.test(key)) return '火山地牢'
  if (/^UndergroundMine\d*$/i.test(key)) return '矿井'
  return null
}

function translateLocationName(player: PlayerLocationLike): string {
  const candidates = [player.locationName, player.location, player.locationDisplayName]
  for (const raw of candidates) {
    const key = normalizeLocationKey(raw)
    if (!key) continue
    const mapped = LOCATION_ZH[key] ?? generatedLocationLabel(key)
    if (mapped) return mapped
  }
  return player.locationDisplayName || player.locationName || player.location || '—'
}

function originalLocationName(player: PlayerLocationLike): string {
  return player.locationDisplayName || player.locationName || player.location || '—'
}

export function PlayersPage({ user, instanceState, dashboardData }: StardewPageProps) {
  const [copied, setCopied] = useState(false)
  const [copyError, setCopyError] = useState(false)

  const isAdmin = user.role === 'admin'
  const state = instanceState?.state ?? null
  const isRunning = state === 'running'
  const isStarting = state === 'starting'
  const playersData = dashboardData.players
  const playerRows = playersData?.players ?? []
  const recentEvents = playersData?.recentEvents ?? []
  const serverInfo = playersData?.rawInfo ?? null
  const serverInfoError = dashboardData.playersError
  const serverInfoLoading = dashboardData.playersLoading
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
    : state === 'error'
      ? 'sd-dot sd-dot-red'
      : isStarting
        ? 'sd-dot sd-dot-yellow sd-dot-pulse'
        : 'sd-dot sd-dot-gray'

  const activeSaveName = dashboardData.saves?.activeSaveName ?? null
  const activeSave = dashboardData.saves?.saves.find(
    (s) => s.isActive || s.name === activeSaveName,
  ) ?? null

  function handleCopyInvite() {
    const code = dashboardData.inviteCode
    if (!code) return
    setCopyError(false)
    navigator.clipboard.writeText(code).then(
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

  const SEASON_ZH: Record<string, string> = {
    spring: '春',
    summer: '夏',
    fall: '秋',
    winter: '冬',
  }
  function saveDate(save: NonNullable<typeof activeSave>): string {
    if (!save.gameYear) return '—'
    const season = SEASON_ZH[save.gameSeason?.toLowerCase() ?? ''] ?? save.gameSeason ?? '?'
    return `第 ${save.gameYear} 年${season}季第 ${save.gameDay ?? '?'} 天`
  }

  function playerRole(player: (typeof playerRows)[number]): string {
    if (player.isHost || player.role === 'host') return '主机'
    if (player.role === 'player') return '玩家'
    return player.role || '—'
  }

  function playerStatusText(player: (typeof playerRows)[number]): string {
    return player.status === 'online' ? '在线' : '离线'
  }

  function playerStatusDot(player: (typeof playerRows)[number]): string {
    return player.status === 'online' ? 'sd-dot sd-dot-green' : 'sd-dot sd-dot-gray'
  }

  function formatGold(value?: number): string {
    if (typeof value !== 'number' || !Number.isFinite(value)) return '—'
    return `${Math.round(value).toLocaleString('zh-CN')}g`
  }

  function farmIncome(player: (typeof playerRows)[number]): number | undefined {
    return player.farmIncome ?? player.totalMoneyEarned
  }

  function personalIncome(player: (typeof playerRows)[number]): number | undefined {
    if (typeof player.personalIncome === 'number') return player.personalIncome
    if (player.walletMode === 'separate') return player.totalMoneyEarned
    return undefined
  }

  function walletModeLabel(mode?: string): string {
    if (mode === 'shared') return '共享'
    if (mode === 'separate') return '分开'
    return '—'
  }

  function formatPlayerLocation(player: (typeof playerRows)[number]): string {
    const name = translateLocationName(player)
    if (typeof player.tileX === 'number' && typeof player.tileY === 'number') {
      return `${name} (${player.tileX}, ${player.tileY})`
    }
    return name
  }

  function eventTypeText(type?: string): string {
    if (type === 'joined') return '加入'
    if (type === 'left') return '离开'
    return '记录'
  }

  function eventClassName(type?: string): string {
    if (type === 'joined' || type === 'seen') return 'sd-player-event sd-player-event-online'
    if (type === 'left') return 'sd-player-event sd-player-event-offline'
    return 'sd-player-event'
  }

  function eventLocation(event: PlayerLocationLike): string {
    const translated = translateLocationName(event)
    return translated === '—' ? '' : translated
  }

  return (
    <div className="sd-page">
      <div className="sd-page-header">
        <img
          className="sd-page-icon"
          src="/assets/stardew/ui/icons/icon_nav_players.png"
          alt=""
        />
        <div>
          <h2 className="sd-page-title">玩家管理</h2>
          <p className="sd-page-desc">
            查看玩家名册、在线/离线状态、持有现金、农场收入、个人收入、邀请码和 Junimo 服务器信息；踢出 / 白名单等管理功能待后端接入。
          </p>
        </div>
      </div>

      <div className="sd-srv-section">
        <div className="sd-srv-section-title">玩家概览</div>

        <div className="sd-players-overview-grid">
          <div className="sd-players-stat">
            <span className="sd-players-stat-label">服务器状态</span>
            <span className="sd-players-stat-value">
              <span className={dotClass} aria-hidden="true" />
              {stateLabelText}
            </span>
          </div>

          <div className="sd-players-stat">
            <span className="sd-players-stat-label">在线人数</span>
            <span className="sd-players-stat-value">
              {onlineCountText}
              {isRunning && playersData?.onlineCount == null && (
                <span className="sd-srv-badge-pending">未识别</span>
              )}
            </span>
          </div>

          <div className="sd-players-stat">
            <span className="sd-players-stat-label">最大人数</span>
            <span className="sd-players-stat-value">
              {maxPlayersText}
              {isRunning && playersData?.maxPlayers == null && (
                <span className="sd-srv-badge-pending">未识别</span>
              )}
            </span>
          </div>

          <div className="sd-players-stat">
            <span className="sd-players-stat-label">数据来源</span>
            <span className="sd-players-stat-value">{playerSourceText}</span>
          </div>

          {playersData?.saveId && (
            <div className="sd-players-stat">
              <span className="sd-players-stat-label">控制存档</span>
              <span className="sd-players-stat-value">{playersData.saveId}</span>
            </div>
          )}

          <div className="sd-players-stat">
            <span className="sd-players-stat-label">当前农场</span>
            <span className="sd-players-stat-value">
              {activeSave
                ? activeSave.farmName
                  ? activeSave.farmName
                  : activeSave.name
                : activeSaveName ?? '—'}
            </span>
          </div>

          {activeSave?.farmerName && (
            <div className="sd-players-stat">
              <span className="sd-players-stat-label">主机农民</span>
              <span className="sd-players-stat-value">{activeSave.farmerName}</span>
            </div>
          )}

          {activeSave?.gameYear && (
            <div className="sd-players-stat">
              <span className="sd-players-stat-label">游戏日期</span>
              <span className="sd-players-stat-value">{saveDate(activeSave)}</span>
            </div>
          )}
        </div>

        <div className="sd-players-invite-row">
          <span className="sd-players-invite-label">邀请码</span>
          {isRunning ? (
            dashboardData.inviteCode ? (
              <span className="sd-players-invite-code">{dashboardData.inviteCode}</span>
            ) : dashboardData.inviteCodeError ? (
              <span className="sd-players-invite-error">获取失败</span>
            ) : (
              <span className="sd-players-invite-loading">获取中…</span>
            )
          ) : (
            <span className="sd-players-invite-empty">服务器未运行</span>
          )}
          {isRunning && dashboardData.inviteCode && (
            <button
              className="sd-btn-tan sd-btn-xs"
              onClick={handleCopyInvite}
              disabled={!dashboardData.inviteCode}
              title="复制邀请码"
            >
              {copied ? '已复制 ✓' : '复制'}
            </button>
          )}
          {isRunning && (
            <button
              className="sd-btn-tan sd-btn-xs"
              onClick={() => { void dashboardData.refreshInviteCode() }}
              title="刷新邀请码"
            >
              刷新
            </button>
          )}
        </div>
        {copyError && (
          <div className="sd-srv-hint" style={{ color: '#b94040', marginTop: 4 }}>
            复制失败，请手动选取邀请码文字。
          </div>
        )}
      </div>

      <div className="sd-srv-section">
        <div className="sd-srv-section-title">
          服务器信息
          {isRunning && (
            <button
              className="sd-btn-tan sd-btn-xs"
              style={{ marginLeft: 8 }}
              onClick={() => { void dashboardData.refreshPlayers() }}
              disabled={serverInfoLoading}
            >
              {serverInfoLoading ? '获取中…' : '刷新'}
            </button>
          )}
        </div>

        {!isRunning && !isStarting && playerRows.length === 0 ? (
          <div className="sd-srv-empty">服务器未运行，暂无服务器信息。</div>
        ) : serverInfoLoading && !serverInfo ? (
          <div className="sd-srv-empty">正在获取服务器信息…</div>
        ) : serverInfoError ? (
          <div className="sd-players-info-error">
            获取服务器信息失败：{serverInfoError}
          </div>
        ) : serverInfo ? (
          <pre className="sd-players-info-terminal">{serverInfo}</pre>
        ) : (
          <div className="sd-srv-empty">
            {isRunning
              ? '服务器已运行，正在通过 Junimo info 获取服务器信息。'
              : '服务器启动中，请稍候…'}
          </div>
        )}

        <div className="sd-srv-hint" style={{ marginTop: 6 }}>
          {playersData?.source === 'smapi_control' ? (
            <span>↑ 玩家列表优先来自 StardewAnxiPanel.Control 写出的结构化控制文件；Junimo info 仅作为回退。</span>
          ) : (
            <>
              <span>↑ 上方内容来自后端调用 JunimoServer </span>
              <code style={{ fontSize: 9 }}>info</code>
              <span> 后的原始输出；玩家数量和姓名由后端保守解析。</span>
            </>
          )}
        </div>
        {playersData?.message && (
          <div className="sd-srv-hint" style={{ marginTop: 2 }}>
            {playersData.message}
          </div>
        )}
      </div>

      <div className="sd-srv-section">
        <div className="sd-srv-section-title">
          玩家列表
          {isRunning && playersData?.parseStatus === 'exact' ? (
            <span className="sd-players-badge-live">已接入</span>
          ) : isRunning ? (
            <span className="sd-srv-badge-pending">部分识别</span>
          ) : null}
        </div>

        {!isRunning && !isStarting && playerRows.length === 0 ? (
          <div className="sd-players-empty">
            <img
              className="sd-players-empty-icon"
              src="/assets/stardew/ui/icons/icon_top_summary_players.png"
              alt=""
            />
            <div className="sd-players-empty-title">暂无玩家名册</div>
            <div className="sd-players-empty-desc">服务器运行并有玩家进入后，会在这里保留玩家并显示在线/离线状态。</div>
          </div>
        ) : serverInfoLoading && !playersData ? (
          <div className="sd-players-empty">
            <img
              className="sd-players-empty-icon"
              src="/assets/stardew/ui/icons/icon_top_summary_players.png"
              alt=""
            />
            <div className="sd-players-empty-title">正在读取玩家列表</div>
            <div className="sd-players-empty-desc">正在读取 Mod 写出的 players.json，并合并已记录玩家名册。</div>
          </div>
        ) : playerRows.length === 0 && playersData?.onlineCount === 0 ? (
          <div className="sd-players-empty">
            <img
              className="sd-players-empty-icon"
              src="/assets/stardew/ui/icons/icon_top_summary_players.png"
              alt=""
            />
            <div className="sd-players-empty-title">暂无在线玩家</div>
            <div className="sd-players-empty-desc">服务器正在运行，但当前快照里没有玩家在线。</div>
          </div>
        ) : playerRows.length === 0 ? (
          <div className="sd-players-empty">
            <img
              className="sd-players-empty-icon"
              src="/assets/stardew/ui/icons/icon_top_summary_players.png"
              alt=""
            />
            <div className="sd-players-empty-title">暂未识别玩家姓名</div>
            <div className="sd-players-empty-desc">
              当前数据源未包含明确玩家名；有 players.json 时会优先展示 SMAPI 控制文件中的玩家列表。
            </div>
          </div>
        ) : null}

        {serverInfoError && (
          <div className="sd-players-info-error">在线玩家读取失败：{serverInfoError}</div>
        )}

        <div className="sd-players-table-placeholder">
          <div className="sd-players-table-header">
            <span>玩家名</span>
            <span>角色</span>
            <span>联机 ID</span>
            <span>位置</span>
            <span>现金</span>
            <span>农场收入</span>
            <span>个人收入</span>
            <span>钱包</span>
            <span>状态</span>
          </div>
          {playerRows.length > 0 ? (
            playerRows.map((player) => (
              <div className="sd-players-table-row" key={player.uniqueMultiplayerId || player.name}>
                <span>{player.name}</span>
                <span>{playerRole(player)}</span>
                <span>{player.uniqueMultiplayerId || '—'}</span>
                <span title={originalLocationName(player)}>{formatPlayerLocation(player)}</span>
                <span>{formatGold(player.money)}</span>
                <span>{formatGold(farmIncome(player))}</span>
                <span>{formatGold(personalIncome(player))}</span>
                <span>{walletModeLabel(player.walletMode)}</span>
                <span>
                  <span className={playerStatusDot(player)} aria-hidden="true" />
                  {playerStatusText(player)}
                </span>
              </div>
            ))
          ) : (
            <div className="sd-players-table-empty-row">
              {isRunning ? '暂无可展示的玩家姓名' : '暂无已记录玩家'}
            </div>
          )}
        </div>
        {playerRows.some((player) => player.walletMode === 'shared') && (
          <div className="sd-srv-hint" style={{ marginTop: 6 }}>
            当前存档使用共享钱包时，现金显示的是团队共享资金，不代表该玩家独立私有余额。
          </div>
        )}
        {playerRows.length > 0 && (
          <div className="sd-srv-hint" style={{ marginTop: 4 }}>
            收入列固定显示农场累计收入和玩家个人累计收入，不随钱包模式切换含义。
          </div>
        )}
      </div>

      <div className="sd-srv-section">
        <div className="sd-srv-section-title">
          玩家活动 / 最近事件
          {recentEvents.length > 0 && (
            <span className="sd-players-badge-live">已接入</span>
          )}
        </div>

        {recentEvents.length > 0 ? (
          <div className="sd-player-events-list">
            {recentEvents.map((event) => {
              const location = eventLocation(event)
              return (
                <div className={eventClassName(event.type)} key={event.id}>
                  <span className="sd-player-event-dot" aria-hidden="true" />
                  <div className="sd-player-event-main">
                    <div className="sd-player-event-title">
                      <span className="sd-player-event-name">{event.playerName}</span>
                      <span className="sd-player-event-type">{eventTypeText(event.type)}</span>
                      {event.isHost && <span className="sd-player-event-host">主机</span>}
                    </div>
                    <div className="sd-player-event-desc">
                      {event.message}
                      {location ? ` 位置：${location}` : ''}
                    </div>
                  </div>
                  <time className="sd-player-event-time" dateTime={event.at}>
                    {formatDate(event.at)}
                  </time>
                </div>
              )
            })}
          </div>
        ) : (
          <div className="sd-players-empty sd-players-empty-small">
            <div className="sd-players-empty-title">暂无最近事件</div>
            <div className="sd-players-empty-desc">
              玩家首次记录、加入和离开会在下一次玩家快照刷新后显示。
            </div>
          </div>
        )}
      </div>

      <div className="sd-srv-section">
        <div className="sd-srv-section-title">
          管理操作
          {!isAdmin && (
            <span className="sd-srv-badge-pending" style={{ background: 'rgba(180,80,0,0.12)', color: '#7a3c00' }}>
              仅管理员
            </span>
          )}
        </div>

        {!isAdmin && (
          <div className="sd-srv-hint" style={{ marginBottom: 8 }}>
            管理操作仅管理员可用。
          </div>
        )}

        <div className="sd-players-actions-grid">
          <div className="sd-players-action-item">
            <button
              className="sd-btn-tan"
              disabled
              title={!isAdmin ? '仅管理员可用' : '踢出玩家 API 待接入'}
            >
              踢出玩家
            </button>
            <span className="sd-srv-badge-pending">待接入</span>
          </div>

          <div className="sd-players-action-item">
            <button
              className="sd-btn-tan"
              disabled
              title={!isAdmin ? '仅管理员可用' : '封禁 API 待接入'}
            >
              封禁玩家
            </button>
            <span className="sd-srv-badge-pending">待接入</span>
          </div>

          <div className="sd-players-action-item">
            <button
              className="sd-btn-tan"
              disabled
              title={!isAdmin ? '仅管理员可用' : '白名单 API 待接入'}
            >
              白名单管理
            </button>
            <span className="sd-srv-badge-pending">待接入</span>
          </div>

          <div className="sd-players-action-item">
            <button
              className="sd-btn-tan"
              disabled
              title={!isAdmin ? '仅管理员可用' : '权限设置 API 待接入'}
            >
              权限设置
            </button>
            <span className="sd-srv-badge-pending">待接入</span>
          </div>
        </div>

        <div className="sd-srv-hint" style={{ marginTop: 8 }}>
          踢出、封禁、白名单和权限设置仍需要 JunimoServer 提供对应 API 或可控命令后才能启用。
        </div>
      </div>

      {dashboardData.versionInfo?.version && (
        <div className="sd-srv-hint">
          面板版本：{dashboardData.versionInfo.version}
          {dashboardData.versionInfo.buildDate
            ? `  ·  构建：${formatDate(dashboardData.versionInfo.buildDate)}`
            : ''}
        </div>
      )}
    </div>
  )
}
